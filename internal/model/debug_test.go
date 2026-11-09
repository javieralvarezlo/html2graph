package model

import (
	"strings"
	"testing"
)

// node construye un nodo con hijos, para escribir árboles de prueba en una
// sola expresión.
func node(kind NodeKind, label string, children ...*Node) *Node {
	n := NewNode(kind, label)
	n.Children = children
	return n
}

func TestDump(t *testing.T) {
	a := node(Element, "a", node(Text, "enlace"))
	a.Attrs = []Attr{{"href", "http://www.issi.uned.es"}, {"title", "di \"hola\"\ny \\ adiós"}}

	tests := []struct {
		name string
		root *Node
		want []string
	}{
		{"árbol vacío", nil, nil},
		{"un solo elemento", node(Element, "br"), []string{`br`}},
		{
			"sangría por niveles",
			node(Element, "html",
				node(Element, "head", node(Element, "title", node(Text, "Hoja de enlaces"))),
				node(Element, "body")),
			[]string{`html`, `  head`, `    title`, `      "Hoja de enlaces"`, `  body`},
		},
		{
			"atributos con comillas, barra y salto de línea",
			a,
			[]string{`a href="http://www.issi.uned.es" title="di \"hola\"\ny \\ adiós"`, `  "enlace"`},
		},
		{
			"texto con comillas, barra, salto de línea y &nbsp;",
			node(Element, "p", node(Text, "a \"b\" \\ c\nd\u00a0e")),
			[]string{`p`, "  \"a \\\"b\\\" \\\\ c\\nd\u00a0e\""},
		},
		{
			"atributos como nodos hijos",
			node(Element, "a", node(Attribute, `href="x"`), node(Text, "enlace")),
			[]string{`a`, `  href="x"`, `  "enlace"`},
		},
		{
			"árbol JSON",
			node(JSONObject, "{}",
				node(JSONKey, "nombre", node(JSONValue, `"Ana"`)),
				node(JSONKey, "tags", node(JSONArray, "[]", node(JSONValue, `"x"`), node(JSONValue, "1"))),
				node(JSONKey, "vacío", node(JSONObject, "{}")),
				node(JSONKey, "nulo", node(JSONValue, "null"))),
			[]string{
				`{}`,
				`  nombre`,
				`    "Ana"`,
				`  tags`,
				`    []`,
				`      "x"`,
				`      1`,
				`  vacío`,
				`    {}`,
				`  nulo`,
				`    null`,
			},
		},
		{
			"salto de línea en una etiqueta sin comillas",
			node(JSONKey, "a\nb", node(JSONValue, "c\nd")),
			[]string{`a\nb`, `  c\nd`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := ""
			if tt.want != nil {
				want = strings.Join(tt.want, "\n") + "\n"
			}
			if got := Dump(tt.root); got != want {
				t.Errorf("\nobtenido:\n%s\nesperado:\n%s", got, want)
			}
		})
	}
}
