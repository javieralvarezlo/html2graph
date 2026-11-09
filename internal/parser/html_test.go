package parser

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/javieralvarezlo/html2graph/internal/htmltok"
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/token"
)

// lines une las líneas esperadas de un Dump, cada una con su salto de línea.
func lines(l ...string) string {
	return strings.Join(l, "\n") + "\n"
}

// parse analiza src con el tokenizer HTML real.
func parse(src string) (*model.Node, error) {
	return ParseHTML(htmltok.New(src))
}

// checkPreorder comprueba que los IDs del árbol son 0..n-1 en preorden y
// devuelve un mensaje con el primer fallo, o "" si no hay ninguno.
func checkPreorder(root *model.Node) string {
	next := 0
	var walk func(n *model.Node) string
	walk = func(n *model.Node) string {
		if n.ID != next {
			return "el nodo " + n.Label + " tiene un ID fuera del preorden"
		}
		next++
		for _, c := range n.Children {
			if msg := walk(c); msg != "" {
				return msg
			}
		}
		return ""
	}
	return walk(root)
}

// TestEnunciado comprueba que el HTML del enunciado produce el árbol de la
// sección 6 de CLAUDE.md, con el atributo href dentro del nodo a.
func TestEnunciado(t *testing.T) {
	src, err := os.ReadFile("../../ejemplos/ok/enunciado.html")
	if err != nil {
		t.Fatal(err)
	}

	root, err := parse(string(src))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	want := lines(
		`html`,
		`  head`,
		`    title`,
		`      "Hoja de enlaces"`,
		`  body`,
		`    h1`,
		`      "Ejemplo2"`,
		`    p`,
		`      "Este texto tiene un"`,
		`      a href="http://www.issi.uned.es"`,
		`        "enlace"`,
		`      "y un texto con"`,
		`      em`,
		`        "énfasis"`,
		`    p`,
		`      "Este texto NO tiene otro enlace y un texto con"`,
		`      b`,
		`        "negrita"`,
	)
	if got := model.Dump(root); got != want {
		t.Errorf("\nobtenido:\n%s\nesperado:\n%s", got, want)
	}
	if msg := checkPreorder(root); msg != "" {
		t.Error(msg)
	}
}

func TestParseCorrecto(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"elementos vacíos con y sin barra",
			`<p>a<br>b<br/>c<img src="x.png" alt="">d</p>`,
			lines(`p`, `  "a"`, `  br`, `  "b"`, `  br`, `  "c"`, `  img src="x.png" alt=""`, `  "d"`),
		},
		{
			"elemento no vacío autocerrado al estilo XHTML",
			`<div><p/><p />x</div>`,
			lines(`div`, `  p`, `  p`, `  "x"`),
		},
		{
			"elemento vacío como raíz",
			`<br>`,
			lines(`br`),
		},
		{
			"etiqueta autocerrada como raíz",
			`<hr/>`,
			lines(`hr`),
		},
		{
			"comentarios y doctype en cualquier posición",
			"<!DOCTYPE html>\n<!-- a -->\n<html><!-- b --><body>x<!-- c -->y</body><!DOCTYPE x></html>\n<!-- d -->\n",
			lines(`html`, `  body`, `    "x"`, `    "y"`),
		},
		{
			"espacios antes y después de la raíz",
			"\n\n  <p>x</p>  \n\t\n",
			lines(`p`, `  "x"`),
		},
		{
			"texto multilínea colapsado",
			"<p>\n  uno\n\tdos   tres\n\n</p>",
			lines(`p`, `  "uno dos tres"`),
		},
		{
			"&nbsp; no se colapsa ni se recorta",
			"<p>&nbsp;a&nbsp;&nbsp; b \n &nbsp;</p>",
			lines(`p`, "  \"\u00a0a\u00a0\u00a0 b \u00a0\""),
		},
		{
			"un &nbsp; entre espacios se conserva solo",
			"<p> &nbsp; </p>",
			lines(`p`, "  \"\u00a0\""),
		},
		{
			"texto solo de espacios dentro de un elemento",
			"<p> \n\t </p>",
			lines(`p`),
		},
		{
			"comillas y barra invertida en el texto",
			`<p>di &quot;hola&quot; \ x</p>`,
			lines(`p`, `  "di \"hola\" \\ x"`),
		},
		{
			"atributo sin valor",
			`<form><input type="text" disabled></form>`,
			lines(`form`, `  input type="text" disabled=""`),
		},
		{
			"nombres en mayúsculas",
			`<P>x</p>`,
			lines(`p`, `  "x"`),
		},
		{
			"anidamiento profundo",
			`<a><b><c><d>x</d></c></b></a>`,
			lines(`a`, `  b`, `    c`, `      d`, `        "x"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parse(tt.src)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got := model.Dump(root); got != tt.want {
				t.Errorf("\nobtenido:\n%s\nesperado:\n%s", got, tt.want)
			}
			if msg := checkPreorder(root); msg != "" {
				t.Error(msg)
			}
		})
	}
}

func TestParseErrores(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"etiquetas cruzadas",
			`<p><b></p></b>`,
			`1:7: se esperaba </b> (abierta en 1:4) y se encontró </p>`,
		},
		{
			"cierre de un elemento abierto más abajo, en otra línea",
			"<html>\n  <body>\n  <p>hola\n</body></html>",
			`4:1: se esperaba </p> (abierta en 3:3) y se encontró </body>`,
		},
		{
			"etiqueta sin cerrar",
			`<p>hola`,
			`1:1: etiqueta <p> sin cerrar al llegar al final del fichero`,
		},
		{
			"se señala la etiqueta sin cerrar más interna",
			"<html><body>\n<p>hola</p>",
			`1:7: etiqueta <body> sin cerrar al llegar al final del fichero`,
		},
		{
			"cierre sin apertura con la pila vacía",
			`</p>`,
			`1:1: cierre </p> sin apertura`,
		},
		{
			"cierre de una etiqueta que no está abierta",
			`<p></i></p>`,
			`1:4: cierre </i> sin apertura`,
		},
		{
			"cierre repetido tras la raíz",
			`<p></p></p>`,
			`1:8: cierre </p> sin apertura`,
		},
		{
			"dos raíces",
			"<p>a</p>\n<div>b</div>",
			`2:1: más de un elemento raíz: <div> aparece después de cerrar <p>`,
		},
		{
			"segunda raíz autocerrada",
			`<p></p><br/>`,
			`1:8: más de un elemento raíz: <br> aparece después de cerrar <p>`,
		},
		{
			"dos elementos vacíos seguidos",
			`<br><br>`,
			`1:5: más de un elemento raíz: <br> aparece después de cerrar <br>`,
		},
		{
			"documento vacío",
			``,
			`1:1: documento vacío: no hay ningún elemento`,
		},
		{
			"documento con solo espacios, comentarios y doctype",
			"  \n<!-- c -->\n<!DOCTYPE html>\n",
			`4:1: documento vacío: no hay ningún elemento`,
		},
		{
			"cierre de un elemento vacío",
			`<p><br></br></p>`,
			`1:8: el elemento vacío <br> no admite etiqueta de cierre`,
		},
		{
			"cierre de un elemento vacío sin nada abierto",
			`</br>`,
			`1:1: el elemento vacío <br> no admite etiqueta de cierre`,
		},
		{
			"texto antes de la raíz",
			`hola<p></p>`,
			`1:1: texto fuera del elemento raíz`,
		},
		{
			"texto después de la raíz, tras espacios",
			"<p></p>\n\n  fin",
			`3:3: texto fuera del elemento raíz`,
		},
		{
			"&nbsp; fuera de la raíz es contenido",
			` &nbsp;<p></p>`,
			`1:2: texto fuera del elemento raíz`,
		},
		{
			"error léxico propagado",
			`<p><a href="x></p>`,
			`1:12: comilla sin cerrar en el valor del atributo 'href'`,
		},
		{
			"el primer error detiene el análisis",
			`<p></b><`,
			`1:4: cierre </b> sin apertura`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parse(tt.src)
			if err == nil {
				t.Fatalf("se esperaba el error %q y no hubo error; árbol:\n%s", tt.want, model.Dump(root))
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q\nse esperaba  %q", err.Error(), tt.want)
			}
			if _, ok := err.(*token.PosError); !ok {
				t.Errorf("el error es %T, se esperaba *token.PosError", err)
			}
			if root != nil {
				t.Error("con error no debe devolverse árbol")
			}
		})
	}
}

// fakeTokenizer devuelve una lista fija de tokens y después el error err.
type fakeTokenizer struct {
	toks []token.Token
	err  error
}

func (f *fakeTokenizer) Next() (token.Token, error) {
	if len(f.toks) == 0 {
		return token.Token{}, f.err
	}
	tok := f.toks[0]
	f.toks = f.toks[1:]
	return tok, nil
}

// TestErrorDelTokenizer comprueba que el error del tokenizer se devuelve tal
// cual, sin envolverlo, aunque no sea un *token.PosError.
func TestErrorDelTokenizer(t *testing.T) {
	want := errors.New("fallo del tokenizer")
	tk := &fakeTokenizer{
		toks: []token.Token{{Kind: token.StartTag, Value: "p", Line: 1, Col: 1}},
		err:  want,
	}
	if _, err := ParseHTML(tk); err != want {
		t.Errorf("error = %v, se esperaba %v", err, want)
	}
}

// TestTokenInesperado comprueba que un token que no es de HTML (por ejemplo,
// de un tokenizer JSON) produce un error con posición.
func TestTokenInesperado(t *testing.T) {
	tk := &fakeTokenizer{toks: []token.Token{
		{Kind: token.StartTag, Value: "p", Line: 1, Col: 1},
		{Kind: token.LBrace, Line: 1, Col: 4},
	}}
	_, err := ParseHTML(tk)
	if err == nil || err.Error() != "1:4: token LBrace inesperado en un documento HTML" {
		t.Errorf("error = %v", err)
	}
}

// TestIDsPreorden comprueba los IDs concretos de un árbol pequeño: los
// textos descartados no consumen ID.
func TestIDsPreorden(t *testing.T) {
	root, err := parse("<a>\n  <b>x</b>\n  <c/>\n  <d><e>y</e></d>\n</a>")
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	var walk func(n *model.Node)
	walk = func(n *model.Node) {
		got = append(got, n.Label)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)

	want := []string{"a", "b", "x", "c", "d", "e", "y"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("preorden = %v, se esperaba %v", got, want)
	}
	if msg := checkPreorder(root); msg != "" {
		t.Error(msg)
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{" \t\n\r\f", ""},
		{"hola", "hola"},
		{"  hola  ", "hola"},
		{"a \n\t b", "a b"},
		{"\u00a0", "\u00a0"},
		{" \u00a0 ", "\u00a0"},
		{"a\u00a0\u00a0b", "a\u00a0\u00a0b"},
		{"a\u2003b", "a\u2003b"},
	}
	for _, tt := range tests {
		if got := cleanText(tt.in); got != tt.want {
			t.Errorf("cleanText(%q) = %q, se esperaba %q", tt.in, got, tt.want)
		}
	}
}

// FuzzParseHTML prueba el parser con entradas aleatorias. Con «go test» solo
// ejecuta las semillas; con «go test -fuzz=FuzzParseHTML» genera entradas
// nuevas. Comprueba que, para cualquier entrada:
//   - no hay pánico;
//   - todo error es un *token.PosError con posición válida;
//   - si no hay error, la raíz es un elemento, los IDs son 0..n-1 en
//     preorden y los textos están limpios.
func FuzzParseHTML(f *testing.F) {
	seeds := []string{
		"", "<p>", "</p>", "<p></p>", "<p><b></p></b>", "<br></br>", "<p/>",
		"<p></p><p></p>", "x<p></p>", "<!DOCTYPE html><!-- c --><html> a &nbsp; b </html>",
		"<a href=x><img src='y'>t</a>", "<p>\n uno \n dos </p>\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		root, err := parse(src)
		if err != nil {
			pe, ok := err.(*token.PosError)
			if !ok || pe.Line < 1 || pe.Col < 1 {
				t.Fatalf("error sin posición válida: %#v", err)
			}
			return
		}

		if root == nil || root.Kind != model.Element {
			t.Fatalf("la raíz no es un elemento: %+v", root)
		}
		if msg := checkPreorder(root); msg != "" {
			t.Fatal(msg)
		}

		var walk func(n *model.Node)
		walk = func(n *model.Node) {
			if n.Kind == model.Text && (n.Label == "" || n.Label != cleanText(n.Label)) {
				t.Fatalf("texto sin limpiar: %q", n.Label)
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(root)
	})
}
