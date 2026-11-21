#!/usr/bin/env python3
"""Validación externa de las salidas de html2graph.

Uso (desde la raíz del proyecto, o con make validar):
    python3 tools/validar.py [DIR_CASOS] [DIR_XSD]

DIR_CASOS   directorio con una carpeta por caso y, en cada una, las salidas
            esperadas esperado.<formato> (por defecto
            internal/writer/testdata del proyecto).
DIR_XSD     directorio con graphml/graphml.xsd y gexf/gexf.xsd
            (por defecto tools/xsd, junto a este script).

Requisitos (ninguno es dependencia del programa; solo se usan para validar):
Python 3 con networkx y regex (pip install networkx regex), Graphviz (dot)
y, para los esquemas XSD, xmllint (libxml2) o, si no está, el módulo
lxml de Python (pip install lxml), que usa la misma libxml2.

Para cada caso toma como referencia el GraphSON (cuya igualdad exacta con
el model.Graph comprueba el test Go TestGraphSON con encoding/json) y
comprueba, con herramientas independientes del proyecto:

  dot      dot -Tsvg sin errores ni avisos; dot -Tjson0: nº de nodos y
           aristas y etiquetas (deshaciendo el escString de Graphviz y las
           entidades que Graphviz interpreta).
  gml      networkx.read_gml(label="id"): grafo dirigido, nodos, aristas,
           label y kind idénticos. Informa si read_gml con label="label"
           (el valor por defecto) falla por etiquetas repetidas.
  graphml  XSD de GraphML 1.0 (xmllint o lxml) y networkx.read_graphml.
  gexf     XSD de GEXF 1.3 (xmllint o lxml) y networkx.read_gexf.
  graphson JSON válido con claves mode, vertices, edges en ese orden.
  gdf      analizador estricto de la gramática elegida (comillas simples
           con escapes \\ \\' \\n \\r); además emula el importador de Gephi
           (su expresión regular) e informa de las etiquetas que Gephi
           leería distintas (solo aviso).
  csv      analizador al estilo de Apache Commons CSV con la configuración
           de Gephi (delimitador ;, comillas ", escape \\); comprueba las
           aristas y que «etiqueta (ID)» se corresponde con cada nodo; emula
           también la importación de Gephi (solo aviso).

Termina con código 0 si todo es correcto y 1 si algo falla. Los avisos de
Gephi no son fallos: documentan las «salidas peculiares». Si falta dot, o
faltan a la vez xmllint y lxml, las comprobaciones que los necesitan se
omiten con un aviso en lugar de contar como fallos.
"""

import glob
import html
import json
import os
import re
import shutil
import subprocess
import sys

import networkx as nx
import regex  # admite look-behind de anchura variable, como Java

AQUI = os.path.dirname(os.path.abspath(__file__))
RAIZ = os.path.dirname(AQUI)
DIR = sys.argv[1] if len(sys.argv) > 1 else os.path.join(RAIZ, "internal", "writer", "testdata")
XSD = sys.argv[2] if len(sys.argv) > 2 else os.path.join(AQUI, "xsd")

fallos = []
avisos = []

# Programas externos opcionales: si falta alguno, se omite lo que lo necesita.
# Su salida se lee siempre como UTF-8 (en Windows, Python usaría por defecto
# la codificación del sistema, cp1252, y estropearía los acentos).
DOT = shutil.which("dot")
XMLLINT = shutil.which("xmllint")

# Sin xmllint (lo normal en Windows) los XSD se validan con lxml, si está.
try:
    from lxml import etree
except ImportError:
    etree = None
VALIDADOR_XSD = "xmllint" if XMLLINT else ("lxml" if etree else None)


def falla(caso, fmt, msg):
    fallos.append(f"{caso}.{fmt}: {msg}")


def aviso(caso, fmt, msg):
    avisos.append(f"{caso}.{fmt}: {msg}")


def referencia(caso):
    with open(os.path.join(DIR, caso, "esperado.graphson"), encoding="utf-8") as f:
        texto = f.read()
    doc = json.loads(texto)
    if list(doc.keys()) != ["mode", "vertices", "edges"]:
        falla(caso, "graphson", f"claves {list(doc.keys())}")
    if doc["mode"] != "NORMAL":
        falla(caso, "graphson", "mode distinto de NORMAL")
    nodos = []
    for i, v in enumerate(doc["vertices"]):
        if v["_id"] != f"n{i}" or v["_type"] != "vertex":
            falla(caso, "graphson", f"vértice {i} mal formado: {v}")
        nodos.append((v["label"], v["kind"]))
    aristas = []
    for i, e in enumerate(doc["edges"]):
        if e["_id"] != f"e{i}" or e["_type"] != "edge" or e["_label"] != "child":
            falla(caso, "graphson", f"arista {i} mal formada: {e}")
        aristas.append((int(e["_outV"][1:]), int(e["_inV"][1:])))
    return nodos, aristas


def ruta(caso, fmt):
    return os.path.join(DIR, caso, f"esperado.{fmt}")


def comparar(caso, fmt, nodos, aristas, got_nodos, got_aristas):
    if len(got_nodos) != len(nodos):
        falla(caso, fmt, f"{len(got_nodos)} nodos, se esperaban {len(nodos)}")
    for i, (a, b) in enumerate(zip(got_nodos, nodos)):
        if a != b:
            falla(caso, fmt, f"nodo {i}: {a!r} != {b!r}")
    if sorted(got_aristas) != sorted(aristas):
        falla(caso, fmt, f"aristas distintas: {sorted(got_aristas)} != {sorted(aristas)}")


# ---------------------------------------------------------------- DOT

def deshacer_escstring(s):
    # Graphviz conserva \\ y \n en la etiqueta (escString) e interpreta las
    # entidades HTML al dibujar.
    s = re.sub(r"\\(.)", lambda m: "\n" if m.group(1) in "nlr" else m.group(1), s)
    return html.unescape(s)


def validar_dot(caso, nodos, aristas):
    p = ruta(caso, "dot")
    r = subprocess.run([DOT, "-Tsvg", p], capture_output=True, text=True, encoding="utf-8", errors="replace")
    if r.returncode != 0 or r.stderr.strip():
        falla(caso, "dot", f"dot -Tsvg: {r.returncode} {r.stderr.strip()}")
        return
    r = subprocess.run([DOT, "-Tjson0", p], capture_output=True, text=True, encoding="utf-8", errors="replace")
    d = json.loads(r.stdout)
    objs = d.get("objects", [])
    got = [(deshacer_escstring(o["label"]), None) for o in objs]
    esperado = [(l.replace("\r\n", "\n").replace("\r", "\n"), None) for l, _ in nodos]
    if [o["name"] for o in objs] != [f"n{i}" for i in range(len(objs))]:
        falla(caso, "dot", "nombres de nodo inesperados")
    comparar(caso, "dot", esperado, aristas, got,
             [(e["tail"], e["head"]) for e in d.get("edges", [])])
    if not d.get("directed"):
        falla(caso, "dot", "el grafo no es dirigido")


# ---------------------------------------------------------------- GML

def validar_gml(caso, nodos, aristas):
    p = ruta(caso, "gml")
    with open(p, "rb") as f:
        datos = f.read()
    try:
        datos.decode("ascii")
    except UnicodeDecodeError:
        falla(caso, "gml", "el fichero no es ASCII")
    G = nx.read_gml(p, label="id")
    if not G.is_directed():
        falla(caso, "gml", "el grafo no es dirigido")
    got = []
    for n in sorted(G.nodes):
        label = G.nodes[n]["label"]
        # networkx convierte las cadenas "[]" y "()" en una lista o tupla
        # vacías (su propia convención para escribirlas).
        if label == [] or label == ():
            aviso(caso, "gml", f"networkx lee la etiqueta del nodo {n} como {label!r} en lugar de la cadena {str(label) if label == [] else '()'!r}")
            label = "[]" if label == [] else "()"
        got.append((label, G.nodes[n]["kind"]))
    comparar(caso, "gml", nodos, aristas, got, list(G.edges()))
    try:
        nx.read_gml(p)
    except (nx.NetworkXError, TypeError) as e:
        aviso(caso, "gml", f"networkx.read_gml con label='label' (por defecto) falla: {type(e).__name__}: {e}")


# ---------------------------------------------------------------- XML

def validar_xsd(caso, fmt, xsd):
    if VALIDADOR_XSD == "lxml":
        esquema = etree.XMLSchema(etree.parse(xsd))
        if not esquema.validate(etree.parse(ruta(caso, fmt))):
            falla(caso, fmt, f"lxml: {esquema.error_log}")
        return
    if not XMLLINT:
        return
    r = subprocess.run([XMLLINT, "--noout", "--schema", xsd, ruta(caso, fmt)],
                       capture_output=True, text=True, encoding="utf-8", errors="replace")
    if r.returncode != 0:
        falla(caso, fmt, f"xmllint: {r.stderr.strip()}")


def validar_graphml(caso, nodos, aristas):
    validar_xsd(caso, "graphml", os.path.join(XSD, "graphml", "graphml.xsd"))
    G = nx.read_graphml(ruta(caso, "graphml"))
    if not G.is_directed():
        falla(caso, "graphml", "el grafo no es dirigido")
    got = [(G.nodes[f"n{i}"]["label"], G.nodes[f"n{i}"]["kind"]) for i in range(G.number_of_nodes())]
    comparar(caso, "graphml", nodos, aristas, got,
             [(int(a[1:]), int(b[1:])) for a, b in G.edges()])


def validar_gexf(caso, nodos, aristas):
    validar_xsd(caso, "gexf", os.path.join(XSD, "gexf", "gexf.xsd"))
    G = nx.read_gexf(ruta(caso, "gexf"))
    if not G.is_directed():
        falla(caso, "gexf", "el grafo no es dirigido")
    got = [(G.nodes[f"n{i}"]["label"], G.nodes[f"n{i}"]["kind"]) for i in range(G.number_of_nodes())]
    comparar(caso, "gexf", nodos, aristas, got,
             [(int(a[1:]), int(b[1:])) for a, b in G.edges()])


# ---------------------------------------------------------------- GDF

GDF_VALOR = re.compile(r"'((?:[^'\\]|\\.)*)'")
GEPHI_GDF = regex.compile(
    "(?<=(?:,|^)\")(.*?)(?=(?<=(?:[^\\\\]))\",|\"$)|(?<=(?:,|^)')(.*?)(?=(?<=(?:[^\\\\]))',|'$)|(?<=(?:,|^))(?=[^'\"])(.*?)(?=(?:,|$))|(?<=,)($)")


def gephi_gdf_campos(linea):
    # Reproduce ImporterGDF de Gephi: la expresión regular y el trim/null.
    campos = []
    for m in GEPHI_GDF.finditer(linea):
        if m.start() != m.end():
            dato = linea[m.start():m.end()].strip()
            campos.append(None if dato == "" or dato.lower() == "null" else dato)
        else:
            campos.append(None)
    return campos


def validar_gdf(caso, nodos, aristas):
    with open(ruta(caso, "gdf"), encoding="utf-8") as f:
        lineas = f.read().split("\n")
    if lineas[-1] != "":
        falla(caso, "gdf", "no termina en salto de línea")
    lineas = lineas[:-1]
    if lineas[0] != "nodedef>name VARCHAR,label VARCHAR,kind VARCHAR":
        falla(caso, "gdf", f"cabecera de nodos: {lineas[0]!r}")
    i = lineas.index("edgedef>node1 VARCHAR,node2 VARCHAR,directed BOOLEAN")
    got, gephi = [], []
    for k, l in enumerate(lineas[1:i]):
        m = re.fullmatch(r"n(\d+),'((?:[^'\\\n\r]|\\.)*)',(\w+)", l)
        if not m or int(m.group(1)) != k:
            falla(caso, "gdf", f"línea de nodo mal formada: {l!r}")
            continue
        etiqueta = re.sub(r"\\(.)", lambda x: {"n": "\n", "r": "\r"}.get(x.group(1), x.group(1)), m.group(2))
        got.append((etiqueta, m.group(3)))
        campos = gephi_gdf_campos(l)
        gephi.append(campos[1] if len(campos) > 1 else None)
    got_aristas = []
    for l in lineas[i + 1:]:
        m = re.fullmatch(r"n(\d+),n(\d+),true", l)
        if not m:
            falla(caso, "gdf", f"línea de arista mal formada: {l!r}")
            continue
        got_aristas.append((int(m.group(1)), int(m.group(2))))
    comparar(caso, "gdf", nodos, aristas, got, got_aristas)
    # Emulación de Gephi (solo avisos).
    for k, (l, _) in enumerate(nodos):
        if k < len(gephi) and gephi[k] != l:
            aviso(caso, "gdf", f"Gephi leería la etiqueta del nodo n{k} como {gephi[k]!r} en lugar de {l!r}")
    for k, l in enumerate(lineas[1:i]):
        if "edgedef>" in l:
            aviso(caso, "gdf", f"Gephi tomaría la línea del nodo n{k} como cabecera de aristas (contiene «edgedef>»)")


# ---------------------------------------------------------------- CSV

def commons_csv(linea, delim=";"):
    # Analizador de una línea al estilo de Apache Commons CSV con
    # quote '"', escape '\\' y trim, como lo configura Gephi.
    campos, i, n = [], 0, len(linea)
    while True:
        buf, citado = [], False
        while i < n and linea[i] == " ":
            i += 1
        if i < n and linea[i] == '"':
            citado = True
            i += 1
            while True:
                if i >= n:
                    raise ValueError("comilla sin cerrar")
                c = linea[i]
                if c == "\\":
                    sig = linea[i + 1]
                    mapa = {"n": "\n", "r": "\r", "t": "\t", "b": "\b", "f": "\f"}
                    if sig in mapa:
                        buf.append(mapa[sig])
                    elif sig in (delim, "\\", '"'):
                        buf.append(sig)
                    else:
                        buf.append(c + sig)
                    i += 2
                elif c == '"':
                    if i + 1 < n and linea[i + 1] == '"':
                        buf.append('"')
                        i += 2
                    else:
                        i += 1
                        break
                else:
                    buf.append(c)
                    i += 1
            while i < n and linea[i] == " ":
                i += 1
        else:
            while i < n and linea[i] != delim:
                buf.append(linea[i])
                i += 1
        campos.append(("".join(buf), citado))
        if i >= n:
            return campos
        if linea[i] != delim:
            raise ValueError(f"basura tras el campo en la columna {i}")
        i += 1


def validar_csv(caso, nodos, aristas):
    with open(ruta(caso, "csv"), encoding="utf-8") as f:
        texto = f.read()
    lineas = texto.split("\n")
    if lineas[-1] != "":
        falla(caso, "csv", "no termina en salto de línea")
    lineas = lineas[:-1]
    if len(lineas) != len(nodos):
        falla(caso, "csv", f"{len(lineas)} líneas para {len(nodos)} nodos")
    ident = {}
    got_aristas = []
    for k, l in enumerate(lineas):
        try:
            campos = commons_csv(l)
        except ValueError as e:
            falla(caso, "csv", f"línea {k + 1}: {e}")
            continue
        if not all(c for _, c in campos):
            falla(caso, "csv", f"línea {k + 1}: campo sin comillas")
        valores = [v for v, _ in campos]
        m = re.fullmatch(r"(.*) \((\d+)\)", valores[0], re.S)
        if not m or int(m.group(2)) != k:
            falla(caso, "csv", f"línea {k + 1}: identificador {valores[0]!r}")
            continue
        ident[valores[0]] = k
        if m.group(1) != nodos[k][0]:
            falla(caso, "csv", f"nodo {k}: {m.group(1)!r} != {nodos[k][0]!r}")
        for v in valores[1:]:
            got_aristas.append((k, v))
    resueltas = []
    for a, v in got_aristas:
        if v not in ident:
            falla(caso, "csv", f"vecino {v!r} sin línea propia")
        else:
            resueltas.append((a, ident[v]))
    comparar(caso, "csv", nodos, aristas, nodos, resueltas)
    if len(set(ident)) != len(nodos):
        falla(caso, "csv", "identificadores repetidos")
    # Emulación de Gephi: detecta el separador contando, en la primera línea
    # y fuera de comillas (' o "), comas, puntos y coma, tabuladores y
    # espacios; en caso de empate gana la coma.
    if lineas:
        cuenta = {",": 0, ";": 0, "\t": 0, " ": 0}
        dentro = False
        for c in lineas[0].strip().replace(" , ", ",").replace(" ; ", ";"):
            if c in "\"'":
                dentro = not dentro
            if not dentro and c in cuenta:
                cuenta[c] += 1
        sep = max(cuenta, key=lambda c: (cuenta[c], ",;\t ".index(c) * -1))
        if sep != ";" and len(lineas[0].split('";"')) > 1:
            aviso(caso, "csv", f"Gephi detectaría {sep!r} como separador en lugar de ';'")
    # Emulación de Gephi: solo crea los nodos que aparecen en alguna arista.
    con_arista = {n for a, b in aristas for n in (a, b)}
    perdidos = [k for k in range(len(nodos)) if k not in con_arista]
    if perdidos:
        aviso(caso, "csv", f"Gephi no crearía los nodos {perdidos}: una línea con un solo campo no genera nada")
    # Qué pasaría si se usaran las etiquetas sin ID (fusión de homónimos).
    etiquetas = [l for l, _ in nodos]
    distintas = len(set(etiquetas))
    if distintas < len(etiquetas):
        G = nx.DiGraph((etiquetas[a], etiquetas[b]) for a, b in aristas)
        G.add_nodes_from(etiquetas)
        ciclos = list(nx.simple_cycles(G))
        aviso(caso, "csv", f"con las etiquetas como identificador, {len(etiquetas)} nodos quedarían en {distintas}"
              + (f" y aparecerían ciclos {ciclos}" if ciclos else "")
              + ("" if nx.is_arborescence(G) else "; el resultado ya no es un árbol"))


def main():
    casos = sorted(os.path.basename(os.path.dirname(p))
                   for p in glob.glob(os.path.join(DIR, "*", "esperado.graphson")))
    if not casos:
        print(f"no hay ficheros <caso>/esperado.graphson en {DIR}")
        return 1
    if not DOT:
        print("AVISO: no se encuentra dot (Graphviz) en el PATH: se omite la validación de DOT")
    if not VALIDADOR_XSD:
        print("AVISO: no se encuentra xmllint en el PATH ni el módulo lxml: GraphML y GEXF"
              " se validan solo con networkx, sin los esquemas XSD")
    for caso in casos:
        nodos, aristas = referencia(caso)
        for fmt, f in [("dot", validar_dot), ("gml", validar_gml), ("graphml", validar_graphml),
                       ("gexf", validar_gexf), ("gdf", validar_gdf), ("csv", validar_csv)]:
            if fmt == "dot" and not DOT:
                print(f"{caso:18} {fmt:8} OMITIDO  (falta dot)")
                continue
            antes = len(fallos)
            try:
                f(caso, nodos, aristas)
            except Exception as e:  # cualquier excepción del lector es un fallo
                falla(caso, fmt, f"{type(e).__name__}: {e}")
            estado = "OK" if len(fallos) == antes else "FALLO"
            print(f"{caso:18} {fmt:8} {estado}  ({len(nodos)} nodos, {len(aristas)} aristas)")
        print(f"{caso:18} graphson OK  (referencia)")
    print()
    for a in avisos:
        print("AVISO", a)
    for f in fallos:
        print("FALLO", f)
    omitidas = [n for n, ruta_prog in [("dot", DOT), ("xmllint o lxml", VALIDADOR_XSD)] if not ruta_prog]
    extra = f"; omitido por falta de {' y '.join(omitidas)}" if omitidas else ""
    print(f"\n{len(casos)} casos, {len(fallos)} fallos, {len(avisos)} avisos{extra}")
    return 1 if fallos else 0


if __name__ == "__main__":
    sys.exit(main())
