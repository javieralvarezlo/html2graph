package model

import "testing"

func TestNodeKindString(t *testing.T) {
	tests := []struct {
		k    NodeKind
		want string
	}{
		{Element, "element"},
		{Text, "text"},
		{Attribute, "attribute"},
		{JSONObject, "object"},
		{JSONArray, "array"},
		{JSONKey, "key"},
		{JSONValue, "value"},
		{NodeKind(-1), "NodeKind(-1)"},
		{NodeKind(42), "NodeKind(42)"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("NodeKind(%d).String() = %q, se esperaba %q", int(tt.k), got, tt.want)
		}
	}
}

func TestNodeKindNamesCompleto(t *testing.T) {
	for k := Element; k <= JSONValue; k++ {
		if nodeKindNames[k] == "" {
			t.Errorf("la clase %d no tiene nombre", int(k))
		}
	}
	if len(nodeKindNames) != int(JSONValue)+1 {
		t.Errorf("nodeKindNames tiene %d entradas, se esperaban %d", len(nodeKindNames), int(JSONValue)+1)
	}
}

func TestNewNode(t *testing.T) {
	n := NewNode(Element, "html")
	if n.Kind != Element || n.Label != "html" || n.ID != 0 {
		t.Errorf("NewNode = %+v", n)
	}
	if len(n.Children) != 0 || len(n.Attrs) != 0 {
		t.Errorf("un nodo nuevo no debe tener hijos ni atributos: %+v", n)
	}
}

// TestAppendChild construye a mano el comienzo del árbol del enunciado y
// comprueba que los hijos conservan el orden de inserción.
func TestAppendChild(t *testing.T) {
	html := NewNode(Element, "html")
	head := html.AppendChild(NewNode(Element, "head"))
	body := html.AppendChild(NewNode(Element, "body"))
	title := head.AppendChild(NewNode(Element, "title"))
	title.AppendChild(NewNode(Text, "Hoja de enlaces"))
	a := body.AppendChild(NewNode(Element, "a"))
	a.Attrs = []Attr{{Name: "href", Value: "http://www.issi.uned.es"}}

	if got := len(html.Children); got != 2 {
		t.Fatalf("html tiene %d hijos, se esperaban 2", got)
	}
	if html.Children[0] != head || html.Children[1] != body {
		t.Error("los hijos de html no están en orden de inserción")
	}
	if got := title.Children[0].Label; got != "Hoja de enlaces" {
		t.Errorf("texto de title = %q", got)
	}
	if got := body.Children[0].Attrs[0]; got.Name != "href" || got.Value != "http://www.issi.uned.es" {
		t.Errorf("atributo de a = %+v", got)
	}
}
