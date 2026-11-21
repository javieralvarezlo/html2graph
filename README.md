# html2graph

Transformador de documentos HTML o JSON bien formados en grafos. Lee el fichero, construye su árbol (elementos y textos en HTML; objetos, claves, arrays y valores en JSON) y lo escribe como grafo dirigido padre → hijo en uno de siete formatos: Graphviz DOT, GDF, GML (Graph Modelling Language), GraphML, GraphSON, GEXF y CSV.

Es la práctica de la asignatura Generación Automática de Código del Máster en Investigación en Ingeniería de Software (UNED): un *code munger* en la terminología de Herrington (*Code Generation in Action*), escrito en Go solo con la biblioteca estándar. El tokenizer y el parser de cada lenguaje de entrada, el modelo, las reglas de transformación y las plantillas de salida están implementados a mano; `go.mod` no tiene dependencias.

```
Fichero ─> Tokenizer ─> Parser ─> Árbol ─> Transformación ─> Aplanado ─> Plantilla ─> Grafo
           (HTML|JSON)  (HTML|JSON)        (reglas)          nodos+aristas  (formato)
```

## Uso rápido

Requisitos: Go 1.27 o posterior y, opcionalmente, GNU make.

```
make build                                    # genera bin/munger (bin/munger.exe en Windows)
bin/munger ejemplos/ok/enunciado.html         # grafo en DOT por la salida estándar
bin/munger -out graphml -o grafo.graphml ejemplos/ok/enunciado.html
bin/munger -all salidas ejemplos/ok/anidado.json   # los siete formatos en salidas/
bin/munger -h                                 # ayuda con todas las opciones
```

Sin make: `go build -o bin/munger ./cmd/munger`.

| Orden | Qué hace |
|---|---|
| `make test` | Formato (`gofmt`), `go vet` y todas las pruebas |
| `make ejemplos` | Regenera las salidas de `ejemplos/ok/` y comprueba los mensajes de `ejemplos/error/` |
| `make dist` | Binarios para Windows, Linux y macOS (x86-64 y ARM64) en `dist/` |
| `make validar`, `make matriz` | Validación externa opcional con Graphviz, networkx y los esquemas XSD (véase [tools/README.md](tools/README.md)) |

## Documentación

- [Manual de instalación y uso](memoria/04-manual.md): opciones, ejemplos, mensajes de error, cómo abrir cada formato.
- [Validación externa](tools/README.md): requisitos de Python (`tools/requirements.txt`), instalación paso a paso y qué significan los avisos.
- [Memoria](memoria/): introducción, alcance, especificación detallada de cada bloque, casos de prueba, ficheros, conclusiones, uso de IA y referencias.

## Estructura

| Directorio | Contenido |
|---|---|
| `cmd/munger/` | La línea de órdenes |
| `internal/token/` | Token común, lector UTF-8 con posiciones y error con línea y columna |
| `internal/htmltok/`, `internal/jsontok/` | Tokenizers de HTML (máquina de estados) y JSON |
| `internal/parser/` | Parsers de HTML y JSON, con pila, que producen el árbol |
| `internal/model/` | Árbol, aplanado a grafo y escritura del árbol para pruebas |
| `internal/rules/`, `internal/transform/` | Qué genera nodo (política) y cómo se reescribe el árbol (mecanismo) |
| `internal/writer/` | Una plantilla `text/template` por formato y sus funciones de escapado |
| `ejemplos/` | Entradas correctas con sus siete salidas y entradas erróneas con su mensaje |
| `tools/` | Validación externa opcional |
| `memoria/` | La memoria del trabajo |
