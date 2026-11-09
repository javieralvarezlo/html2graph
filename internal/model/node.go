// Package model define el modelo intermedio de html2graph: el árbol que
// construyen los parsers y el grafo aplanado que consumen las plantillas.
//
// Los parsers de HTML y JSON producen un *Node, la transformación
// reescribe ese árbol y el aplanado lo convierte en un Graph
// (lista de nodos y lista de aristas), que es lo único que reciben las
// plantillas. Así, añadir un tipo de entrada no obliga a tocar las salidas y
// añadir un formato de salida no obliga a tocar las entradas.
//
// Contenido del paquete
//
//   - node.go: el nodo del árbol (Node, NodeKind, Attr). No hay un tipo
//     árbol aparte: un árbol se representa por su nodo raíz, y el resto
//     se alcanza recorriendo recursivamente la lista de hijos.
//   - graph.go: el grafo aplanado (Graph, GraphNode, GraphEdge).
//   - debug.go: Dump, que escribe un árbol como texto con sangría para
//     los tests y la depuración.
package model

import "fmt"

// NodeKind indica qué clase de cosa representa un nodo del árbol.
//
// Todo nodo tiene una clase (Kind) y una etiqueta (Label). La etiqueta es el
// texto que se mostrará en el nodo al dibujar el grafo, la clase dice qué es
// el nodo. Hacen falta las dos porque nodos distintos pueden mostrar el mismo
// texto: el elemento <a> y un texto que diga "a" tienen ambos la etiqueta
// "a", pero uno es Element y el otro Text. Las reglas de transformación y
// los formatos de salida deciden qué hacer con cada nodo según su clase.
//
// Clases y etiqueta de cada una:
//
//	Clase       Qué representa                         Label
//	----------  -------------------------------------  ------------------
//	Element     un elemento HTML                       nombre de etiqueta
//	Text        un texto HTML (espacios ya limpiados)  el texto
//	Attribute   un atributo HTML convertido en nodo    nombre="valor"
//	JSONObject  un objeto JSON                         {}
//	JSONArray   un array JSON                          []
//	JSONKey     una clave de un objeto JSON            nombre de la clave
//	JSONValue   un valor JSON simple (cadena, número,  el valor
//	            true, false o null)
//
// Los nodos Attribute solo aparecen si las reglas piden los atributos como
// hijos, por defecto los atributos se guardan en el campo Attrs del nodo
// Element. Un nodo JSONKey tiene siempre un único hijo: su valor.
//
// Ejemplo HTML. La entrada
//
//	<a href="http://www.issi.uned.es">enlace</a>
//
// produce:
//
//	a        (Element, Attrs = [href="http://www.issi.uned.es"])
//	└── enlace   (Text)
//
// Ejemplo JSON. La entrada
//
//	{"nombre": "Ana", "tags": ["x"]}
//
// produce:
//
//	{}           (JSONObject)
//	├── nombre   (JSONKey)
//	│   └── Ana  (JSONValue)
//	└── tags     (JSONKey)
//	    └── []   (JSONArray)
//	        └── x    (JSONValue)
type NodeKind int

const (
	Element NodeKind = iota
	Text
	Attribute
	JSONObject
	JSONArray
	JSONKey
	JSONValue
)

// nodeKindNames asocia a cada NodeKind su nombre legible. Es un mapa (y no
// un array) para poder preguntar si una clase tiene nombre con la forma
// "valor, ok := mapa[clave]".
var nodeKindNames = map[NodeKind]string{
	Element:    "element",
	Text:       "text",
	Attribute:  "attribute",
	JSONObject: "object",
	JSONArray:  "array",
	JSONKey:    "key",
	JSONValue:  "value",
}

// String devuelve el nombre legible de una clase de nodo, en minúsculas
// (por ejemplo "element"), o "NodeKind(n)" si k no es una clase definida.
func (k NodeKind) String() string {
	kindName, ok := nodeKindNames[k]
	if ok {
		return kindName
	}

	return fmt.Sprintf("NodeKind(%d)", int(k))
}

// Attr es un atributo de un nodo: nombre y valor.
type Attr struct {
	Name  string
	Value string
}

// Node es un nodo del árbol.
//
// Campos:
//   - ID: identificador numérico. Lo asignan el parser, la transformación y
//     el aplanado. Dentro de un mismo árbol es único.
//   - Kind: qué representa el tipo de nodo.
//   - Label: texto principal del nodo (etiqueta, texto, clave o literal).
//   - Attrs: atributos HTML del elemento, en el orden de la entrada. Solo
//     se usa en nodos Element.
//   - Children: hijos en el orden de la entrada.
type Node struct {
	ID       int
	Kind     NodeKind
	Label    string
	Attrs    []Attr
	Children []*Node
}

// NewNode crea un nodo sin hijos ni atributos.
func NewNode(kind NodeKind, label string) *Node {
	return &Node{Kind: kind, Label: label}
}

// AppendChild añade c al final de la lista de hijos de n y devuelve c, para
// poder seguir usándolo (por ejemplo, apilarlo) en la misma línea.
func (n *Node) AppendChild(c *Node) *Node {
	n.Children = append(n.Children, c)
	return c
}
