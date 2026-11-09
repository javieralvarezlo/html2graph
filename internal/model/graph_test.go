package model

import (
	"reflect"
	"testing"
)

func TestFlatten(t *testing.T) {
	// Árbol del enunciado reducido, con IDs del árbol desordenados a
	// propósito: Flatten debe numerar de nuevo.
	a := node(Element, "a", node(Text, "enlace"))
	a.Attrs = []Attr{{"href", "http://www.issi.uned.es"}}
	p := node(Element, "p", node(Text, "Este texto tiene un"), a, node(Text, "y un texto con"))
	html := node(Element, "html", node(Element, "head", node(Element, "title")), node(Element, "body", p))
	html.ID, p.ID, a.ID = 7, 7, 3

	tests := []struct {
		name string
		root *Node
		want Graph
	}{
		{"árbol vacío", nil, Graph{}},
		{"un solo nodo", node(Text, "hola"), Graph{Nodes: []GraphNode{{ID: 0, Label: "hola", Kind: Text}}}},
		{
			"preorden, IDs renumerados y aristas padre → hijo",
			html,
			Graph{
				Nodes: []GraphNode{
					{ID: 0, Label: "html", Kind: Element},
					{ID: 1, Label: "head", Kind: Element},
					{ID: 2, Label: "title", Kind: Element},
					{ID: 3, Label: "body", Kind: Element},
					{ID: 4, Label: "p", Kind: Element},
					{ID: 5, Label: "Este texto tiene un", Kind: Text},
					{ID: 6, Label: `a href="http://www.issi.uned.es"`, Kind: Element, Attrs: []Attr{{"href", "http://www.issi.uned.es"}}},
					{ID: 7, Label: "enlace", Kind: Text},
					{ID: 8, Label: "y un texto con", Kind: Text},
				},
				Edges: []GraphEdge{{0, 1}, {1, 2}, {0, 3}, {3, 4}, {4, 5}, {4, 6}, {6, 7}, {4, 8}},
			},
		},
		{
			"árbol JSON construido a mano",
			node(JSONObject, "{}",
				node(JSONKey, "nombre", node(JSONValue, "Ana")),
				node(JSONKey, "tags", node(JSONArray, "[]", node(JSONValue, "x")))),
			Graph{
				Nodes: []GraphNode{
					{ID: 0, Label: "{}", Kind: JSONObject},
					{ID: 1, Label: "nombre", Kind: JSONKey},
					{ID: 2, Label: "Ana", Kind: JSONValue},
					{ID: 3, Label: "tags", Kind: JSONKey},
					{ID: 4, Label: "[]", Kind: JSONArray},
					{ID: 5, Label: "x", Kind: JSONValue},
				},
				Edges: []GraphEdge{{0, 1}, {1, 2}, {0, 3}, {3, 4}, {4, 5}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Flatten(tt.root); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nobtenido: %+v\nesperado: %+v", got, tt.want)
			}
		})
	}
}

func TestAdjacency(t *testing.T) {
	tests := []struct {
		name string
		root *Node
		want [][]int
	}{
		{"grafo vacío", nil, [][]int{}},
		{"un solo nodo", node(Element, "br"), [][]int{nil}},
		{
			"hijos en orden y hojas sin vecinos",
			node(Element, "p", node(Text, "uno"), node(Element, "b", node(Text, "dos")), node(Text, "tres")),
			[][]int{{1, 2, 4}, nil, {3}, nil, nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Flatten(tt.root).Adjacency(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Adjacency() = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

// TestFlattenAttrsCopiados comprueba que los atributos del grafo son una
// copia: cambiar el árbol después de aplanarlo no cambia el grafo.
func TestFlattenAttrsCopiados(t *testing.T) {
	a := node(Element, "a")
	a.Attrs = []Attr{{"href", "x"}}
	g := Flatten(a)
	a.Attrs[0].Value = "y"
	if got := g.Nodes[0].Attrs[0].Value; got != "x" {
		t.Errorf("el atributo del grafo cambió a %q al cambiar el árbol", got)
	}
}

func TestDisplayLabel(t *testing.T) {
	attrs := func(n *Node, a ...Attr) *Node {
		n.Attrs = a
		return n
	}

	tests := []struct {
		name string
		n    *Node
		want string
	}{
		{"elemento sin atributos", node(Element, "p"), `p`},
		{"elemento con un atributo", attrs(node(Element, "a"), Attr{"href", "http://www.issi.uned.es"}), `a href="http://www.issi.uned.es"`},
		{"varios atributos en orden", attrs(node(Element, "img"), Attr{"src", "x.png"}, Attr{"alt", ""}), `img src="x.png" alt=""`},
		{"apóstrofo entre comillas dobles", attrs(node(Element, "a"), Attr{"title", `it's`}), `a title="it's"`},
		{"comillas dobles entre simples", attrs(node(Element, "a"), Attr{"title", `di "hola"`}), `a title='di "hola"'`},
		{"los dos tipos de comillas", attrs(node(Element, "a"), Attr{"title", `di "hola" y 'adiós'`}), `a title="di &quot;hola&quot; y 'adiós'"`},
		{"comillas en varios atributos", attrs(node(Element, "a"), Attr{"a", `"`}, Attr{"b", `'`}, Attr{"c", `'"`}), `a a='"' b="'" c="'&quot;"`},
		{"texto sin comillas", node(Text, "Hoja de enlaces"), `Hoja de enlaces`},
		{"atributo como nodo", node(Attribute, `href="x"`), `href="x"`},
		{"valor JSON", node(JSONValue, "null"), `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.DisplayLabel(); got != tt.want {
				t.Errorf("DisplayLabel() = %q, se esperaba %q", got, tt.want)
			}
		})
	}
}
