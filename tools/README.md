# Herramientas de validación externa

Comprobaciones que leen las salidas de html2graph con programas independientes del proyecto. No forman parte del programa ni son necesarias para compilarlo o usarlo: `go.mod` sigue sin dependencias.

| Fichero | Qué hace | Cómo se ejecuta |
|---|---|---|
| `validar.py` | Lee cada salida esperada (`internal/writer/testdata/<caso>/esperado.<formato>`) con Graphviz, los esquemas XSD (con xmllint o lxml) y networkx, y comprueba que nodos, aristas y etiquetas coinciden exactamente con el grafo de origen. Emula además los importadores de Gephi para GDF y CSV y avisa de las «salidas peculiares». | `make validar` o `python3 tools/validar.py` |
| `matriz.sh` | Compila `munger` y prueba por la línea de órdenes todas las combinaciones de entrada (HTML, JSON), formato (los siete) y destino (salida estándar, `-o`, `-all`); compara cada salida con la esperada y la valida con `validar.py`. | `make matriz` o `bash tools/matriz.sh` |
| `requirements.txt` | Bibliotecas de Python que necesita `validar.py`. | `python -m pip install -r tools/requirements.txt` |
| `xsd/` | Esquemas XSD de GraphML 1.0 y GEXF 1.3 que usa `validar.py`. | — |

## Requisitos

- Python 3 con las bibliotecas de `requirements.txt` (networkx, regex y lxml).
- Graphviz (`dot`).
- Opcionalmente `xmllint` (paquete libxml2). Si está en el PATH se usa para los esquemas XSD; si no, se usa `lxml`, que lleva dentro la misma libxml2. En Windows basta con `lxml`.
- Para `matriz.sh`, bash (en Windows, Git Bash).

En Windows el Makefile usa `python` en lugar de `python3`; si el intérprete tiene otro nombre: `make validar PYTHON=py`.

Para `make matriz` en Windows hay que usar el bash de Git for Windows: basta con lanzar `make matriz` desde Git Bash, o, desde `cmd` o PowerShell, donde `bash` a secas suele abrir el de WSL:

```
make matriz BASH="C:/Program Files/Git/bin/bash.exe"
```

Si falta `dot`, o faltan a la vez `xmllint` y `lxml`, `validar.py` lo avisa al empezar y omite lo que los necesita (DOT entero; los esquemas XSD de GraphML y GEXF, que entonces se validan solo con networkx), en lugar de contarlo como fallo. En Windows, Graphviz se instala en `C:\Program Files\Graphviz\bin`, que hay que añadir al `Path` si el instalador no lo hace.

## Instalación paso a paso

1. Instalar Python 3 (<https://www.python.org/downloads/>; en Windows, marcar «Add python.exe to PATH») y comprobarlo con `python --version` (`python3 --version` en Linux y macOS).
2. Instalar las bibliotecas desde la raíz del repositorio:

   ```
   python -m pip install -r tools/requirements.txt
   ```

   Si se prefiere no tocar el Python del sistema, antes puede crearse un entorno virtual con `python -m venv .venv` y activarse (`.venv\Scripts\activate` en Windows, `source .venv/bin/activate` en Linux y macOS).
3. Instalar Graphviz (<https://graphviz.org/download/>; en Windows, `winget install Graphviz.Graphviz`) y comprobar que `dot -V` responde en una terminal nueva.
4. Ejecutar `make validar` y, desde Git Bash en Windows, `make matriz`. Ambos deben terminar con «0 fallos».

## Qué son los avisos

Un aviso no es un fallo: la salida es correcta según la especificación del formato, pero un programa concreto la leería de otra manera. Son las «salidas peculiares» que se documentan en la memoria (apartado 2.3.4) y salen siempre, también con todo instalado. `make validar` los lista uno a uno; `make matriz` solo los cuenta. Hay cuatro clases:

- **GML y networkx.** networkx exige por defecto etiquetas únicas, y en HTML se repiten (`p`, `li`); por eso se valida con `label='id'`, y que falle con el valor por defecto queda como aviso. Además lee la etiqueta `[]` de un array JSON (escrita como `label "[]"`) como una lista vacía y no como una cadena.
- **GDF y Gephi.** El importador GDF de Gephi no deshace los escapes (`\n`, `\'`, `\\`, `\>`) y trata `null` como valor vacío; la etiqueta que vería Gephi difiere de la escrita.
- **CSV y Gephi.** El CSV de aristas usa las etiquetas como identificadores; al repetirse, Gephi fundiría nodos distintos y podrían aparecer ciclos. En algún caso, además, detectaría mal el separador o no crearía un grafo de un solo nodo.
- **Número de avisos.** Con las tres entradas de `ejemplos/ok/` salen 13; con todos los casos de `internal/writer/testdata/`, 45. Si cambia el número sin haber cambiado las salidas, algo ha cambiado en las herramientas externas.

## Procedencia de los esquemas

- `xsd/gexf/`: `gexf.xsd`, `dynamics.xsd` y `viz.xsd` de la versión 1.3, del repositorio oficial `gephi/gexf` (rama `gh-pages`).
- `xsd/graphml/`: `graphml.xsd`, `graphml-structure.xsd` y `xlink.xsd` de GraphML 1.0. El sitio oficial (graphml.graphdrawing.org) no era accesible; se tomaron de una copia en el repositorio de Eclipse QVTd, con el espacio de nombres corregido. Se comprobó que validan los ficheros correctos, rechazan los incorrectos y dan el mismo resultado que el `graphml.xsd` que distribuye JGraphT.
