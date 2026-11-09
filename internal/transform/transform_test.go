package transform

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/javieralvarezlo/html2graph/internal/htmltok"
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/parser"
	"github.com/javieralvarezlo/html2graph/internal/rules"
	"github.com/javieralvarezlo/html2graph/internal/writer"
)

// testRules son reglas de prueba: una acción por etiqueta (Include para las
// que no aparecen), un modo de atributos y una bandera para quitar los
// textos.
type testRules struct {
	actions map[string]rules.Action
	attrs   rules.AttrMode
	noText  bool
}

func (r testRules) ElementAction(tag string) rules.Action {
	action, ok := r.actions[tag]
	if ok {
		return action
	}

	return rules.Include
}

func (r testRules) AttrMode() rules.AttrMode { return r.attrs }

func (r testRules) IncludeText() bool { return !r.noText }

// actions abrevia el mapa de acciones de testRules en las tablas.
type actions = map[string]rules.Action

// lines une las líneas esperadas de un Dump, cada una con su salto de línea;
// sin líneas, el Dump del árbol vacío.
func lines(l ...string) string {
	if len(l) == 0 {
		return ""
	}

	return strings.Join(l, "\n") + "\n"
}

// parse analiza un documento HTML con el tokenizer y el parser reales.
func parse(t *testing.T, src string) *model.Node {
	t.Helper()
	root, err := parser.ParseHTML(htmltok.New(src))
	if err != nil {
		t.Fatalf("error inesperado al analizar %q: %v", src, err)
	}

	return root
}

// checkPreorder comprueba que los IDs del árbol son 0..n-1 en preorden.
func checkPreorder(t *testing.T, root *model.Node) {
	t.Helper()
	next := 0
	var walk func(n *model.Node)
	walk = func(n *model.Node) {
		if n.ID != next {
			t.Errorf("el nodo %q tiene el ID %d, se esperaba %d", n.Label, n.ID, next)
		}
		next++
		for _, c := range n.Children {
			walk(c)
		}
	}
	if root != nil {
		walk(root)
	}
}

// enunciadoP es el primer párrafo del enunciado, con un atributo más para
// probar las comillas.
const enunciadoP = `<p>Este texto tiene un <a href="http://www.issi.uned.es" title='di "hola"'> enlace</a> y un texto con <em>énfasis</em></p>`

func TestApply(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		rules rules.Rules
		want  string
	}{
		{"Default no cambia nada", enunciadoP, rules.Default{}, lines(
			`p`,
			`  "Este texto tiene un"`,
			`  a href="http://www.issi.uned.es" title="di \"hola\""`,
			`    "enlace"`,
			`  "y un texto con"`,
			`  em`,
			`    "énfasis"`,
		)},
		{"Include explícito", `<p><b>x</b></p>`, testRules{actions: actions{"p": rules.Include, "b": rules.Include}}, lines(
			`p`,
			`  b`,
			`    "x"`,
		)},
		{"Ignore quita el elemento y su subárbol", enunciadoP, testRules{actions: actions{"a": rules.Ignore}}, lines(
			`p`,
			`  "Este texto tiene un"`,
			`  "y un texto con"`,
			`  em`,
			`    "énfasis"`,
		)},
		{"Flatten cuelga los hijos del padre en su lugar", `<p>uno <b>dos <i>tres</i> cuatro</b> cinco</p>`, testRules{actions: actions{"b": rules.Flatten}}, lines(
			`p`,
			`  "uno"`,
			`  "dos"`,
			`  i`,
			`    "tres"`,
			`  "cuatro"`,
			`  "cinco"`,
		)},
		{"Flatten anidado", `<section>a<div><div><p>b</p></div>c<p>d</p></div>e</section>`, testRules{actions: actions{"div": rules.Flatten}}, lines(
			`section`,
			`  "a"`,
			`  p`,
			`    "b"`,
			`  "c"`,
			`  p`,
			`    "d"`,
			`  "e"`,
		)},
		{"Flatten de un elemento cuyos hijos se ignoran", `<ul><li><b>x</b><i>y</i></li><li>z</li></ul>`, testRules{actions: actions{"li": rules.Flatten, "b": rules.Ignore, "i": rules.Ignore}}, lines(
			`ul`,
			`  "z"`,
		)},
		{"Flatten descarta los atributos del elemento", `<p><a href="x">y</a></p>`, testRules{actions: actions{"a": rules.Flatten}, attrs: rules.AttrsAsChildren}, lines(
			`p`,
			`  "y"`,
		)},
		{"AttrsInNode", `<p><img src="x.png" alt=""/></p>`, testRules{attrs: rules.AttrsInNode}, lines(
			`p`,
			`  img src="x.png" alt=""`,
		)},
		{"AttrsAsChildren con las comillas de DisplayLabel", enunciadoP, testRules{attrs: rules.AttrsAsChildren}, lines(
			`p`,
			`  "Este texto tiene un"`,
			`  a`,
			`    href="http://www.issi.uned.es"`,
			`    title='di "hola"'`,
			`    "enlace"`,
			`  "y un texto con"`,
			`  em`,
			`    "énfasis"`,
		)},
		{"AttrsAsChildren con las dos comillas y elemento vacío", `<p><img alt="it's" title='"a" y &apos;b&apos;'/><br></p>`, testRules{attrs: rules.AttrsAsChildren}, lines(
			`p`,
			`  img`,
			`    alt="it's"`,
			`    title="&quot;a&quot; y 'b'"`,
			`  br`,
		)},
		{"AttrsNone", enunciadoP, testRules{attrs: rules.AttrsNone}, lines(
			`p`,
			`  "Este texto tiene un"`,
			`  a`,
			`    "enlace"`,
			`  "y un texto con"`,
			`  em`,
			`    "énfasis"`,
		)},
		{"textos desactivados", enunciadoP, testRules{noText: true}, lines(
			`p`,
			`  a href="http://www.issi.uned.es" title="di \"hola\""`,
			`  em`,
		)},
		{"textos desactivados y atributos como hijos", enunciadoP, testRules{noText: true, attrs: rules.AttrsAsChildren}, lines(
			`p`,
			`  a`,
			`    href="http://www.issi.uned.es"`,
			`    title='di "hola"'`,
			`  em`,
		)},
		{"Ignore sobre la raíz da el árbol vacío", enunciadoP, testRules{actions: actions{"p": rules.Ignore}}, lines()},
		{"Flatten sobre la raíz con un hijo", `<html><body><p>x</p></body></html>`, testRules{actions: actions{"html": rules.Flatten}}, lines(
			`body`,
			`  p`,
			`    "x"`,
		)},
		{"Flatten sobre la raíz con un texto", `<p>solo texto</p>`, testRules{actions: actions{"p": rules.Flatten}}, lines(
			`"solo texto"`,
		)},
		{"Flatten sobre la raíz con los textos desactivados", `<p>solo texto</p>`, testRules{actions: actions{"p": rules.Flatten}, noText: true}, lines()},
		{"Flatten sobre la raíz sin hijos", `<p><br></p>`, testRules{actions: actions{"p": rules.Flatten, "br": rules.Ignore}}, lines()},
		{"Flatten sobre la raíz cuyos hijos se aplanan", `<html><body><main><p>x</p></main></body></html>`, testRules{actions: actions{"html": rules.Flatten, "body": rules.Flatten}}, lines(
			`main`,
			`  p`,
			`    "x"`,
		)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(parse(t, tt.src), tt.rules)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if d := model.Dump(got); d != tt.want {
				t.Errorf("\nobtenido:\n%s\nesperado:\n%s", d, tt.want)
			}
			checkPreorder(t, got)
		})
	}
}

// TestApplyVariasRaices comprueba que aplanar una raíz que deja varios
// nodos sin padre es un error y no un árbol con una raíz inventada.
func TestApplyVariasRaices(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		rules rules.Rules
	}{
		{"dos elementos", `<body><h1>x</h1><p>y</p></body>`, testRules{actions: actions{"body": rules.Flatten}}},
		{"elemento y texto", `<p>uno <b>dos</b></p>`, testRules{actions: actions{"p": rules.Flatten}}},
		{"elementos con atributos como hijos", `<div><a href="x"></a><br></div>`, testRules{actions: actions{"div": rules.Flatten}, attrs: rules.AttrsAsChildren}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Apply(parse(t, tt.src), tt.rules)
			if !errors.Is(err, ErrMultipleRoots) {
				t.Fatalf("error = %v, se esperaba ErrMultipleRoots", err)
			}
			if got != nil {
				t.Errorf("con error se esperaba un árbol vacío, no:\n%s", model.Dump(got))
			}
		})
	}

	_, err := Apply(parse(t, `<body><h1>x</h1><p>y</p></body>`), testRules{actions: actions{"body": rules.Flatten}})
	want := "el árbol transformado tendría varias raíces: Flatten sobre la raíz <body> deja 2 nodos sin padre"
	if err == nil || err.Error() != want {
		t.Errorf("mensaje de error = %v, se esperaba %q", err, want)
	}
}

func TestApplyArbolVacio(t *testing.T) {
	got, err := Apply(nil, rules.Default{})
	if got != nil || err != nil {
		t.Errorf("Apply(nil) = %v, %v; se esperaba nil, nil", got, err)
	}
}

// TestApplyArbolVacioEscritores comprueba que el árbol vacío que deja un
// Ignore sobre la raíz se escribe sin error en todos los formatos.
func TestApplyArbolVacioEscritores(t *testing.T) {
	root, err := Apply(parse(t, `<html><body>x</body></html>`), testRules{actions: actions{"html": rules.Ignore}})
	if root != nil || err != nil {
		t.Fatalf("Apply = %v, %v; se esperaba el árbol vacío", root, err)
	}
	for _, format := range writer.Formats() {
		w, err := writer.Lookup(format)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := w.Write(&b, model.Flatten(root)); err != nil {
			t.Errorf("%s: error inesperado con el grafo vacío: %v", format, err)
		}
	}
}

// TestApplyNoModificaOriginal comprueba que Apply no cambia el árbol de
// entrada y que el árbol nuevo no comparte con él nodos, listas de hijos
// ni atributos: después de modificar a fondo el resultado, el original
// sigue igual.
func TestApplyNoModificaOriginal(t *testing.T) {
	reglas := []rules.Rules{
		rules.Default{},
		testRules{attrs: rules.AttrsAsChildren},
		testRules{actions: actions{"em": rules.Flatten, "h1": rules.Ignore}, attrs: rules.AttrsNone, noText: true},
	}
	for _, r := range reglas {
		t.Run(r.AttrMode().String(), func(t *testing.T) {
			orig := parse(t, `<body a="1"><h1 b="2">x</h1><p c="3">y <em d="4">z</em></p></body>`)
			before := model.Dump(orig)
			ids := map[*model.Node]bool{}
			var collect func(n *model.Node)
			collect = func(n *model.Node) {
				ids[n] = true
				for _, c := range n.Children {
					collect(c)
				}
			}
			collect(orig)

			got, err := Apply(orig, r)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if after := model.Dump(orig); after != before {
				t.Fatalf("Apply ha modificado el original:\nantes:\n%s\ndespués:\n%s", before, after)
			}

			var mutate func(n *model.Node)
			mutate = func(n *model.Node) {
				_, shared := ids[n]
				if shared {
					t.Errorf("el nodo %q es el mismo puntero en los dos árboles", n.Label)
				}
				n.Label += "!"
				for i := range n.Attrs {
					n.Attrs[i].Value += "!"
				}
				for i := range n.Children {
					mutate(n.Children[i])
					n.Children[i] = model.NewNode(model.Text, "otro")
				}
				n.Children = append(n.Children, model.NewNode(model.Text, "más"))
			}
			mutate(got)
			if after := model.Dump(orig); after != before {
				t.Errorf("modificar el resultado ha cambiado el original:\nantes:\n%s\ndespués:\n%s", before, after)
			}
		})
	}
}

// TestApplyJSON comprueba que los nodos JSON se copian completos con
// cualquier regla, aunque una clave se llame como un elemento ignorado, y
// que sus IDs se renumeran en preorden.
func TestApplyJSON(t *testing.T) {
	node := func(kind model.NodeKind, label string, children ...*model.Node) *model.Node {
		n := model.NewNode(kind, label)
		n.ID = 99
		n.Children = children
		return n
	}
	root := node(model.JSONObject, "{}",
		node(model.JSONKey, "p", node(model.JSONValue, "texto")),
		node(model.JSONKey, "html", node(model.JSONArray, "[]",
			node(model.JSONValue, "1"),
			node(model.JSONObject, "{}", node(model.JSONKey, "a", node(model.JSONValue, "null"))))),
		node(model.JSONKey, "vacío", node(model.JSONArray, "[]")))
	want := lines(
		`{}`,
		`  p`,
		`    texto`,
		`  html`,
		`    []`,
		`      1`,
		`      {}`,
		`        a`,
		`          null`,
		`  vacío`,
		`    []`,
	)

	reglas := []rules.Rules{
		rules.Default{},
		testRules{actions: actions{"p": rules.Ignore, "html": rules.Flatten, "a": rules.Ignore}, attrs: rules.AttrsNone, noText: true},
		testRules{attrs: rules.AttrsAsChildren},
	}
	for _, r := range reglas {
		got, err := Apply(root, r)
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if d := model.Dump(got); d != want {
			t.Errorf("con %+v:\nobtenido:\n%s\nesperado:\n%s", r, d, want)
		}
		checkPreorder(t, got)
		if !reflect.DeepEqual(model.Flatten(got), model.Flatten(root)) {
			t.Errorf("con %+v el grafo cambia", r)
		}
	}
}

// TestApplyReglasDesconocidas comprueba que una acción o un modo de
// atributos fuera de los definidos detienen el programa con un mensaje
// que los nombra.
func TestApplyReglasDesconocidas(t *testing.T) {
	tests := []struct {
		name  string
		rules rules.Rules
		want  string
	}{
		{"acción", testRules{actions: actions{"b": rules.Action(7)}}, "transform: acción Action(7) desconocida para <b>"},
		{"modo de atributos", testRules{attrs: rules.AttrMode(7)}, "transform: modo de atributos AttrMode(7) desconocido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != tt.want {
					t.Errorf("pánico = %v, se esperaba %q", got, tt.want)
				}
			}()
			Apply(parse(t, `<p><b>x</b></p>`), tt.rules)
		})
	}
}

// TestDefaultNoCambiaSalidas comprueba que, para cada documento HTML de
// las salidas esperadas de los escritores, las reglas por defecto devuelven
// un árbol idéntico (mismo Dump, mismos IDs) que da el mismo grafo. Así,
// añadir la transformación al pipeline no cambia ninguna salida.
func TestDefaultNoCambiaSalidas(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../../ejemplos/ok/*.html", "../writer/testdata/*/entrada.html"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 7 {
		t.Fatalf("solo se han encontrado %d documentos: %v", len(files), files)
	}

	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			orig := parse(t, string(src))
			got, err := Apply(orig, rules.Default{})
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if model.Dump(got) != model.Dump(orig) {
				t.Errorf("el árbol cambia:\nobtenido:\n%s\nesperado:\n%s", model.Dump(got), model.Dump(orig))
			}
			checkPreorder(t, got)
			if !reflect.DeepEqual(model.Flatten(got), model.Flatten(orig)) {
				t.Error("el grafo aplanado cambia")
			}
		})
	}
}
