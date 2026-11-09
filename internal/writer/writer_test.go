package writer

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/javieralvarezlo/html2graph/internal/htmltok"
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/parser"
)

// update regenera las salidas esperadas en lugar de compararlas:
//
//	go test ./internal/writer -update
var update = flag.Bool("update", false, "regenera los ficheros testdata/<caso>/esperado.<formato>")

func TestEscape(t *testing.T) {
	tests := []struct {
		format string
		escape func(string) string
		in     string
		want   string
	}{
		{"dot", escapeDOT, "Hoja de enlaces", `Hoja de enlaces`},
		{"dot", escapeDOT, "", ``},
		{"dot", escapeDOT, `a href="http://x"`, `a href=\"http://x\"`},
		{"dot", escapeDOT, `C:\dir\n`, `C:\\dir\\n`},
		{"dot", escapeDOT, `\"`, `\\\"`},
		{"dot", escapeDOT, "uno\ndos", `uno\ndos`},
		{"dot", escapeDOT, "uno\r\ndos\rtres", `uno\ndos\ntres`},
		{"dot", escapeDOT, "a\u00a0b énfasis\t", "a\u00a0b énfasis\t"},
		{"dot", escapeDOT, `Librería & café &quot;`, `Librería &amp; café &amp;quot;`},
		{"dot", escapeDOT, `termina en \`, `termina en \\`},
		{"dot", escapeDOT, `edgedef> ]]> --> "a" 'b'`, `edgedef> ]]> --> \"a\" 'b'`},
		{"dot", escapeDOT, "a\x00b\vc\x7f\u0085\uFFFE\uFDD0", "a\uFFFDb\uFFFDc\uFFFD\uFFFD\uFFFD\uFFFD"},
		{"dot", escapeDOT, "a\xffb", "a\uFFFDb"},

		{"gdf", escapeGDF, "Hoja de enlaces", `Hoja de enlaces`},
		{"gdf", escapeGDF, `a href="x", b`, `a href="x", b`},
		{"gdf", escapeGDF, `it's 'a', 'b'`, `it\'s \'a\', \'b\'`},
		{"gdf", escapeGDF, `C:\ y \'`, `C:\\ y \\\'`},
		{"gdf", escapeGDF, "uno\ndos\r\ntres", `uno\ndos\r\ntres`},
		{"gdf", escapeGDF, "a\u00a0b ñ 😀; x", "a\u00a0b ñ 😀; x"},
		{"gdf", escapeGDF, `termina en \`, `termina en \\`},
		{"gdf", escapeGDF, `x,edgedef>y Nodedef>name`, `x,edgedef\>y Nodedef\>name`},
		{"gdf", escapeGDF, `]]> --> "a"`, `]]\> --\> "a"`},
		{"gdf", escapeGDF, "a\x00b\vc\uFFFE", "a\uFFFDb\uFFFDc\uFFFD"},

		{"gml", escapeGML, "Hoja de enlaces", `Hoja de enlaces`},
		{"gml", escapeGML, `a href="x"`, `a href=&quot;x&quot;`},
		{"gml", escapeGML, `&quot; & <b>`, `&amp;quot; &amp; <b>`},
		{"gml", escapeGML, `C:\dir ' [x] #`, `C:\dir ' [x] #`},
		{"gml", escapeGML, "énfasis\u00a0😀", `&#233;nfasis&#160;&#128512;`},
		{"gml", escapeGML, "uno\ndos\r\ntres\tcuatro", `uno&#10;dos&#13;&#10;tres&#9;cuatro`},
		{"gml", escapeGML, "\x7f~", `&#127;~`},
		{"gml", escapeGML, `termina en \`, `termina en \`},
		{"gml", escapeGML, `edgedef> ]]> --> "a" 'b'`, `edgedef> ]]> --> &quot;a&quot; 'b'`},
		{"gml", escapeGML, "a\x00b\vc\uFFFE", `a&#0;b&#11;c&#65534;`},

		{"xml", escapeXML, "Hoja de enlaces", `Hoja de enlaces`},
		{"xml", escapeXML, `<a href="x">'y' & z</a>`, `&lt;a href=&quot;x&quot;&gt;&apos;y&apos; &amp; z&lt;/a&gt;`},
		{"xml", escapeXML, `&amp;`, `&amp;amp;`},
		{"xml", escapeXML, "uno\ndos\r\ntres\tcuatro", `uno&#10;dos&#13;&#10;tres&#9;cuatro`},
		{"xml", escapeXML, "énfasis\u00a0😀 \\", "énfasis\u00a0😀 \\"},
		{"xml", escapeXML, `]]> --> <!-- edgedef>`, `]]&gt; --&gt; &lt;!-- edgedef&gt;`},
		{"xml", escapeXML, "a\x00b\vc\fd\x1b\x7f\u0085\uFFFE\uFFFF\uFDD0\U0001FFFE", "a\uFFFDb\uFFFDc\uFFFDd\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD\uFFFD"},
		{"xml", escapeXML, "a\xffb", "a\uFFFDb"},

		{"json", escapeJSON, "Hoja de enlaces", `Hoja de enlaces`},
		{"json", escapeJSON, `a href="x" C:\dir`, `a href=\"x\" C:\\dir`},
		{"json", escapeJSON, "uno\ndos\r\ntres\tcuatro", `uno\ndos\r\ntres\tcuatro`},
		{"json", escapeJSON, "\x00\x01\b\f\x1f\x7f", `\u0000\u0001\u0008\u000c\u001f` + "\x7f"},
		{"json", escapeJSON, "énfasis\u00a0😀 / <b>", "énfasis\u00a0😀 / <b>"},
		{"json", escapeJSON, `termina en \`, `termina en \\`},
		{"json", escapeJSON, `edgedef> ]]> --> "a" 'b'`, `edgedef> ]]> --> \"a\" 'b'`},
		{"json", escapeJSON, "\v\uFFFE", `\u000b` + "\uFFFE"},

		{"csv", escapeCSV, "Hoja de enlaces", `Hoja de enlaces`},
		{"csv", escapeCSV, `a href="x"; b, c`, `a href=""x""; b, c`},
		{"csv", escapeCSV, `C:\dir\n \`, `C:\\dir\\n \\`},
		{"csv", escapeCSV, "uno\ndos\r\ntres\tcuatro", "uno\\ndos\\r\\ntres\tcuatro"},
		{"csv", escapeCSV, "it's\u00a0ñ 😀", "it's\u00a0ñ 😀"},
		{"csv", escapeCSV, `edgedef> ]]> --> "a" 'b'`, `edgedef> ]]> --> ""a"" 'b'`},
		{"csv", escapeCSV, "a\x00b\vc\uFFFE", "a\uFFFDb\uFFFDc\uFFFD"},
	}
	for _, tt := range tests {
		t.Run(tt.format+"/"+tt.in, func(t *testing.T) {
			if got := tt.escape(tt.in); got != tt.want {
				t.Errorf("escape %s(%q) = %q, se esperaba %q", tt.format, tt.in, got, tt.want)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	w, err := Lookup("dot")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if w.Ext != ".dot" {
		t.Errorf("extensión de dot = %q, se esperaba \".dot\"", w.Ext)
	}

	_, err = Lookup("xxx")
	want := "formato de salida «xxx» inexistente; formatos disponibles: csv, dot, gdf, gexf, gml, graphml, graphson"
	if err == nil || err.Error() != want {
		t.Errorf("Lookup(\"xxx\") da el error %v, se esperaba %q", err, want)
	}
}

func TestFormats(t *testing.T) {
	got := Formats()
	if !slices.IsSorted(got) {
		t.Errorf("Formats() = %v no está ordenado", got)
	}
	if len(got) != len(writers) {
		t.Errorf("Formats() = %v, se esperaban %d formatos", got, len(writers))
	}
}

// TestDOT comprueba la salida DOT de un grafo construido a mano: grafo
// vacío, un solo nodo y etiquetas que hay que escapar.
func TestDOT(t *testing.T) {
	a := model.NewNode(model.Element, "a")
	a.Attrs = []model.Attr{{Name: "title", Value: `di "hola"`}}
	a.AppendChild(model.NewNode(model.Text, "uno\\dos\ntres"))

	tests := []struct {
		name string
		root *model.Node
		want []string
	}{
		{"grafo vacío", nil, []string{
			`digraph html2graph {`,
			`	node [shape=box];`,
			`}`,
		}},
		{"un nodo", model.NewNode(model.Element, "br"), []string{
			`digraph html2graph {`,
			`	node [shape=box];`,
			`	n0 [label="br"];`,
			`}`,
		}},
		{"etiquetas escapadas", a, []string{
			`digraph html2graph {`,
			`	node [shape=box];`,
			`	n0 [label="a title='di \"hola\"'"];`,
			`	n1 [label="uno\\dos\ntres"];`,
			`	n0 -> n1;`,
			`}`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := write(t, "dot", model.Flatten(tt.root))
			if want := strings.Join(tt.want, "\n") + "\n"; got != want {
				t.Errorf("\nobtenido:\n%s\nesperado:\n%s", got, want)
			}
		})
	}
}

// TestDOTLargo comprueba que una etiqueta de 40000 caracteres se parte en
// cadenas de como mucho 8000 bytes unidas con +, sin separar un "\r\n" ni
// una secuencia de escape. Si Graphviz está instalado, comprueba además que
// dot -Tsvg acepta el fichero y que dot -Tjson recupera la etiqueta entera.
func TestDOTLargo(t *testing.T) {
	tests := []struct {
		name  string
		label string
	}{
		{"texto con acentos", strings.Repeat("palabra é ", 4000)},
		{"todo escapes", strings.Repeat(`&"\`, 13334)[:40000]},
		{"retorno de carro en el corte", strings.Repeat("x", dotChunk-1) + "\r\n" + strings.Repeat("y", 38000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeDOT(tt.label)
			for _, part := range strings.Split(got, `" + "`) {
				if len(part) > 8000 {
					t.Errorf("trozo de %d bytes, se esperaban como mucho 8000", len(part))
				}
				if strings.HasSuffix(strings.TrimSuffix(part, `\\`), `\`) {
					t.Errorf("un trozo termina en mitad de un escape: …%q", part[len(part)-10:])
				}
			}
			if strings.Count(got, `\n`) != strings.Count(tt.label, "\n") {
				t.Errorf("el par \\r\\n se ha partido: %d saltos de línea", strings.Count(got, `\n`))
			}
		})
	}

	if _, err := exec.LookPath("dot"); err != nil {
		t.Skip("Graphviz no está instalado")
	}
	label := strings.Repeat("palabra é ", 4000)
	src := write(t, "dot", model.Flatten(model.NewNode(model.Text, label)))
	svg := exec.Command("dot", "-Tsvg")
	svg.Stdin = strings.NewReader(src)
	if out, err := svg.CombinedOutput(); err != nil {
		t.Fatalf("dot -Tsvg: %v\n%.300s", err, out)
	}
	js := exec.Command("dot", "-Tjson")
	js.Stdin = strings.NewReader(src)
	out, err := js.Output()
	if err != nil {
		t.Fatalf("dot -Tjson: %v", err)
	}
	var doc struct {
		Objects []struct{ Label string }
	}
	if err := json.Unmarshal(out, &doc); err != nil || len(doc.Objects) != 1 {
		t.Fatalf("salida de dot -Tjson inesperada: %v", err)
	}
	if doc.Objects[0].Label != label {
		t.Errorf("Graphviz lee una etiqueta de %d bytes, se esperaban %d", len(doc.Objects[0].Label), len(label))
	}
}

// outputCases son los casos de TestSalidas: los dos ejemplos públicos, los
// casos con salidas peculiares de testdata/<caso>/entrada.html y un árbol
// construido a mano
// con saltos de línea, retornos de carro y tabuladores también en los
// textos, donde el parser HTML los colapsa (en los valores de atributo sí
// llegan desde HTML: caso raro-saltos-atributo).
func outputCases(t *testing.T) []struct {
	name string
	g    model.Graph
} {
	t.Helper()
	files := []struct{ name, path string }{
		{"enunciado", "../../ejemplos/ok/enunciado.html"},
		{"rico", "../../ejemplos/ok/rico.html"},
		{"raro-caracteres", "testdata/raro-caracteres/entrada.html"},
		{"raro-homonimos", "testdata/raro-homonimos/entrada.html"},
		{"raro-largo", "testdata/raro-largo/entrada.html"},
		{"raro-saltos-atributo", "testdata/raro-saltos-atributo/entrada.html"},
		{"raro-un-nodo", "testdata/raro-un-nodo/entrada.html"},
	}
	var cases []struct {
		name string
		g    model.Graph
	}
	for _, f := range files {
		src, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		root, err := parser.ParseHTML(htmltok.New(string(src)))
		if err != nil {
			t.Fatalf("%s: error inesperado: %v", f.path, err)
		}
		cases = append(cases, struct {
			name string
			g    model.Graph
		}{f.name, model.Flatten(root)})
	}

	// Saltos de línea, retornos de carro y tabuladores en etiquetas y en
	// valores de atributo, como los que llegarán desde cadenas JSON.
	pre := model.NewNode(model.Element, "pre")
	pre.Attrs = []model.Attr{{Name: "title", Value: "línea 1\nlínea 2"}}
	pre.AppendChild(model.NewNode(model.Text, "uno\ndos"))
	pre.AppendChild(model.NewNode(model.Text, "tres\r\ncuatro"))
	pre.AppendChild(model.NewNode(model.Text, "cinco\rseis"))
	pre.AppendChild(model.NewNode(model.Text, "siete\tocho"))
	cases = append(cases, struct {
		name string
		g    model.Graph
	}{"saltos", model.Flatten(pre)})

	return cases
}

// TestSalidas escribe cada caso de outputCases en cada formato y compara la
// salida con la esperada, testdata/<caso>/esperado.<formato>. Los casos
// leídos de fichero recorren todo el pipeline (tokenizer, parser, aplanado y
// plantilla).
func TestSalidas(t *testing.T) {
	for _, c := range outputCases(t) {
		for _, format := range Formats() {
			t.Run(c.name+"."+format, func(t *testing.T) {
				got := write(t, format, c.g)
				expected := filepath.Join("testdata", c.name, "esperado."+format)
				if *update {
					if err := os.WriteFile(expected, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(expected)
				if err != nil {
					t.Fatalf("%v (ejecute go test ./internal/writer -update para generarlo)", err)
				}
				if got != string(want) {
					t.Errorf("la salida no coincide con %s\nobtenido:\n%s\nesperado:\n%s", expected, got, want)
				}
			})
		}
	}
}

// hostileCases son grafos construidos a mano con etiquetas que ponen a
// prueba el escapado: barra invertida final, «edgedef>», fin de CDATA y de
// comentario, los dos tipos de comillas y, en los dos últimos, caracteres
// que no cumplen token.IsAllowed. Con ellos, sanitize de cada formato
// debe dejar un fichero bien formado aunque cambie la etiqueta.
func hostileCases() []struct {
	name string
	g    model.Graph
} {
	root := model.NewNode(model.Element, "a")
	root.Attrs = []model.Attr{{Name: "title", Value: `di "hola" y 'adiós'`}, {Name: "x", Value: `\`}}
	for _, l := range []string{`termina en \`, `\`, "edgedef>", "x,edgedef>y", "]]>", "-->", "<!--", `"`, `'`, `' "`} {
		root.AppendChild(model.NewNode(model.Text, l))
	}

	control := model.NewNode(model.Element, "p")
	for _, l := range []string{"a\x01b", "\x00", "\x1b[0m", "\v\f", "\x7f\u0085", "\uFFFE\uFFFF\uFDD0"} {
		control.AppendChild(model.NewNode(model.Text, l))
	}

	return []struct {
		name string
		g    model.Graph
	}{
		{"hostil", model.Flatten(root)},
		{"controles", model.Flatten(control)},
	}
}

// TestGraphSON lee con encoding/json la salida GraphSON de cada caso, de los
// casos hostiles y de un grafo vacío, y comprueba que es JSON válido con los
// mismos nodos (identificador, etiqueta y clase) y aristas que el grafo.
// GraphSON escribe los caracteres de control con \u00XX, así que también en
// el caso de controles la etiqueta leída es idéntica.
func TestGraphSON(t *testing.T) {
	type vertex struct {
		ID, Type, Label, Kind string
	}
	type edge struct {
		ID, Type, OutV, InV, Label string
	}
	cases := append(outputCases(t), hostileCases()...)
	cases = append(cases, struct {
		name string
		g    model.Graph
	}{"vacío", model.Graph{}})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var doc struct {
				Mode     string
				Vertices []map[string]string
				Edges    []map[string]string
			}
			out := write(t, "graphson", c.g)
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("la salida no es JSON válido: %v\n%s", err, out)
			}
			if doc.Mode != "NORMAL" {
				t.Errorf("mode = %q, se esperaba NORMAL", doc.Mode)
			}

			var want []vertex
			for _, n := range c.g.Nodes {
				want = append(want, vertex{fmt.Sprintf("n%d", n.ID), "vertex", n.Label, n.Kind.String()})
			}
			var got []vertex
			for _, v := range doc.Vertices {
				got = append(got, vertex{v["_id"], v["_type"], v["label"], v["kind"]})
			}
			if !slices.Equal(got, want) {
				t.Errorf("vértices:\nobtenidos: %q\nesperados: %q", got, want)
			}

			var wantEdges []edge
			for i, e := range c.g.Edges {
				wantEdges = append(wantEdges, edge{fmt.Sprintf("e%d", i), "edge", fmt.Sprintf("n%d", e.From), fmt.Sprintf("n%d", e.To), "child"})
			}
			var gotEdges []edge
			for _, e := range doc.Edges {
				gotEdges = append(gotEdges, edge{e["_id"], e["_type"], e["_outV"], e["_inV"], e["_label"]})
			}
			if !slices.Equal(gotEdges, wantEdges) {
				t.Errorf("aristas:\nobtenidas: %q\nesperadas: %q", gotEdges, wantEdges)
			}
		})
	}
}

// TestXML lee con encoding/xml las salidas GraphML y GEXF de cada caso, de
// los casos hostiles y de un grafo vacío, y comprueba que son XML bien
// formado con las mismas etiquetas, clases y aristas que el grafo. Así se
// comprueba el escapado XML sin herramientas externas. Las etiquetas se
// comparan después de sanitize: en el caso de controles, los caracteres
// que XML no admite se leen como U+FFFD.
func TestXML(t *testing.T) {
	type node struct{ ID, Label, Kind string }
	type edge struct {
		Source string `xml:"source,attr"`
		Target string `xml:"target,attr"`
	}
	var graphml struct {
		Nodes []struct {
			ID   string `xml:"id,attr"`
			Data []struct {
				Key   string `xml:"key,attr"`
				Value string `xml:",chardata"`
			} `xml:"data"`
		} `xml:"graph>node"`
		Edges []edge `xml:"graph>edge"`
	}
	var gexf struct {
		Nodes []struct {
			ID     string `xml:"id,attr"`
			Label  string `xml:"label,attr"`
			Values []struct {
				For   string `xml:"for,attr"`
				Value string `xml:"value,attr"`
			} `xml:"attvalues>attvalue"`
		} `xml:"graph>nodes>node"`
		Edges []edge `xml:"graph>edges>edge"`
	}

	cases := append(outputCases(t), hostileCases()...)
	cases = append(cases, struct {
		name string
		g    model.Graph
	}{"vacío", model.Graph{}})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var want []node
			for _, n := range c.g.Nodes {
				want = append(want, node{fmt.Sprintf("n%d", n.ID), sanitize(n.Label), n.Kind.String()})
			}
			var wantEdges []edge
			for _, e := range c.g.Edges {
				wantEdges = append(wantEdges, edge{fmt.Sprintf("n%d", e.From), fmt.Sprintf("n%d", e.To)})
			}

			graphml.Nodes, graphml.Edges = nil, nil
			if err := xml.Unmarshal([]byte(write(t, "graphml", c.g)), &graphml); err != nil {
				t.Fatalf("GraphML no es XML bien formado: %v", err)
			}
			var got []node
			for _, n := range graphml.Nodes {
				data := map[string]string{}
				for _, d := range n.Data {
					data[d.Key] = d.Value
				}
				got = append(got, node{n.ID, data["label"], data["kind"]})
			}
			if !slices.Equal(got, want) || !slices.Equal(graphml.Edges, wantEdges) {
				t.Errorf("GraphML:\nnodos: %q\naristas: %v\nse esperaba:\nnodos: %q\naristas: %v", got, graphml.Edges, want, wantEdges)
			}

			gexf.Nodes, gexf.Edges = nil, nil
			if err := xml.Unmarshal([]byte(write(t, "gexf", c.g)), &gexf); err != nil {
				t.Fatalf("GEXF no es XML bien formado: %v", err)
			}
			got = nil
			for _, n := range gexf.Nodes {
				kind := ""
				for _, v := range n.Values {
					if v.For == "kind" {
						kind = v.Value
					}
				}
				got = append(got, node{n.ID, n.Label, kind})
			}
			if !slices.Equal(got, want) || !slices.Equal(gexf.Edges, wantEdges) {
				t.Errorf("GEXF:\nnodos: %q\naristas: %v\nse esperaba:\nnodos: %q\naristas: %v", got, gexf.Edges, want, wantEdges)
			}
		})
	}
}

// write escribe g en el formato format y devuelve el texto.
func write(t *testing.T, format string, g model.Graph) string {
	t.Helper()
	w, err := Lookup(format)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := w.Write(&b, g); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	return b.String()
}
