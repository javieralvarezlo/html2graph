package model

import (
	"slices"
	"strings"
)

// Graph es el árbol aplanado: la forma en que las plantillas de salida ven
// el modelo. Un árbol con N nodos da un grafo con N nodos y N-1 aristas
// (una por cada relación padre → hijo).
//
// Campos:
//   - Nodes: nodos en preorden, con IDs consecutivos desde 0.
//   - Edges: aristas padre → hijo, en el orden en que se recorren.
type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

// GraphNode es un nodo del grafo aplanado.
//
// Campos:
//   - ID: identificador del nodo en el grafo (posición en preorden).
//   - Label: etiqueta de presentación, el texto que se mostrará en la
//     salida (véase Node.DisplayLabel).
//   - Kind: clase del nodo de origen en el árbol.
//   - Attrs: atributos HTML del nodo de origen, por si un formato quiere
//     exportarlos como datos además de mostrarlos en la etiqueta.
type GraphNode struct {
	ID    int
	Label string
	Kind  NodeKind
	Attrs []Attr
}

// GraphEdge es una arista dirigida del grafo aplanado: From es el ID del
// padre y To el ID del hijo.
type GraphEdge struct {
	From int
	To   int
}

// Flatten convierte el árbol de root en un grafo: recorre el árbol en
// preorden, da a cada nodo el ID de su posición en el recorrido (desde 0) y
// añade una arista de cada padre a cada uno de sus hijos. Los IDs del árbol
// no se usan: el grafo se numera siempre de nuevo. Un árbol vacío (nil) da
// un grafo sin nodos ni aristas.
//
// Las aristas quedan en el orden en que se visitan los hijos, de modo que la
// arista i llega al nodo i+1.
func Flatten(root *Node) Graph {
	var g Graph
	if root == nil {
		return g
	}

	// parent es el ID del padre en el grafo, o -1 para la raíz, que no tiene
	// arista de entrada. El ID de un nodo es el número de nodos ya visitados.
	var walk func(n *Node, parent int)
	walk = func(n *Node, parent int) {
		id := len(g.Nodes)
		g.Nodes = append(g.Nodes, GraphNode{
			ID:    id,
			Label: n.DisplayLabel(),
			Kind:  n.Kind,
			Attrs: slices.Clone(n.Attrs),
		})
		if parent >= 0 {
			g.Edges = append(g.Edges, GraphEdge{From: parent, To: id})
		}

		for _, c := range n.Children {
			walk(c, id)
		}
	}
	walk(root, -1)

	return g
}

// Adjacency devuelve la lista de adyacencia del grafo: el elemento i es la
// lista de los IDs de los hijos del nodo i, en el orden de las aristas (el
// de la entrada), y está vacío si el nodo es una hoja. La usan los formatos
// que escriben cada nodo seguido de sus vecinos, como CSV.
func (g Graph) Adjacency() [][]int {
	adj := make([][]int, len(g.Nodes))
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	return adj
}

// DisplayLabel devuelve la etiqueta de presentación del nodo, la que
// muestran todos los formatos de salida. Para un Element es el nombre seguido
// de ` nombre="valor"` por cada atributo, como en la figura del enunciado
// (a href="http://www.issi.uned.es"); para el resto de clases es Label tal
// cual, y en particular los textos van sin comillas.
//
// Las comillas de cada valor se eligen como lo haría quien escribe HTML,
// para que la etiqueta no sea ambigua: dobles si el valor no contiene
// comillas dobles (title="it's"); simples si contiene dobles pero no
// simples (title='di "hola"'); y si contiene las dos, dobles, con las
// dobles del valor escritas &quot; (title="di &quot;hola&quot; y 'adiós'").
// No se escapa nada más: cada formato escapa la etiqueta según sus propias
// reglas.
func (n *Node) DisplayLabel() string {
	if n.Kind != Element || len(n.Attrs) == 0 {
		return n.Label
	}

	var b strings.Builder
	b.WriteString(n.Label)
	for _, a := range n.Attrs {
		quote, value := `"`, a.Value
		switch {
		case !strings.Contains(value, `"`):
			// Comillas dobles y el valor tal cual, el caso habitual.
		case !strings.Contains(value, "'"):
			quote = "'"
		default:
			value = strings.ReplaceAll(value, `"`, "&quot;")
		}

		b.WriteString(" ")
		b.WriteString(a.Name)
		b.WriteString("=")
		b.WriteString(quote)
		b.WriteString(value)
		b.WriteString(quote)
	}

	return b.String()
}
