package parser

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/javieralvarezlo/html2graph/internal/jsontok"
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/token"
)

// parseJSON analiza src con el tokenizer JSON real.
func parseJSON(src string) (*model.Node, error) {
	return ParseJSON(jsontok.New(src))
}

func TestParseJSONCorrecto(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"ejemplo del comentario",
			`{"nombre": "Ana", "edad": 30, "tags": ["x", null]}`,
			lines(`{}`, `  nombre`, `    "Ana"`, `  edad`, `    30`, `  tags`, `    []`, `      "x"`, `      null`),
		},
		{"número como raíz", `42`, lines(`42`)},
		{"cadena como raíz", ` "hola" `, lines(`"hola"`)},
		{"true como raíz", `true`, lines(`true`)},
		{"false como raíz", "\nfalse\n", lines(`false`)},
		{"null como raíz", `null`, lines(`null`)},
		{"objeto vacío", `{}`, lines(`{}`)},
		{"array vacío", `[ ]`, lines(`[]`)},
		{
			"el orden de las claves se conserva",
			`{"z": 1, "a": 2, "m": 3}`,
			lines(`{}`, `  z`, `    1`, `  a`, `    2`, `  m`, `    3`),
		},
		{
			"cadenas y literales del mismo texto se distinguen",
			`["30", 30, "null", null, "true", true, ""]`,
			lines(`[]`, `  "30"`, `  30`, `  "null"`, `  null`, `  "true"`, `  true`, `  ""`),
		},
		{
			"los números conservan su literal",
			`[1.0, 1e3, -0, 0.10, 123456789012345678901234567890]`,
			lines(`[]`, `  1.0`, `  1e3`, `  -0`, `  0.10`, `  123456789012345678901234567890`),
		},
		{
			"escapes resueltos en claves y valores",
			`{"a\nb": "di \"hola\"", "\u00f1": "\ud83d\ude00\t"}`,
			lines(`{}`, `  a\nb`, `    "di "hola""`, `  ñ`, "    \"😀\t\""),
		},
		{
			"clave vacía",
			`{"": 1}`,
			lines(`{}`, `  `, `    1`),
		},
		{
			"la misma clave en objetos distintos",
			`{"a": {"a": 1}, "b": [{"a": 2}, {"a": 3}]}`,
			lines(`{}`, `  a`, `    {}`, `      a`, `        1`, `  b`, `    []`, `      {}`, `        a`, `          2`, `      {}`, `        a`, `          3`),
		},
		{
			"arrays anidados y vacíos",
			`[[], [[]], [{}]]`,
			lines(`[]`, `  []`, `  []`, `    []`, `  []`, `    {}`),
		},
		{
			"claves cuyo valor es un contenedor",
			`{"o": {}, "a": []}`,
			lines(`{}`, `  o`, `    {}`, `  a`, `    []`),
		},
		{
			"espacios, saltos de Windows y BOM",
			"\uFEFF{\r\n\t\"a\" :\r\n\t[ 1 ,\t2 ]\r\n}\r\n",
			lines(`{}`, `  a`, `    []`, `      1`, `      2`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parseJSON(tt.src)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got := model.Dump(root); got != tt.want {
				t.Errorf("\nobtenido:\n%s\nesperado:\n%s", got, tt.want)
			}
			if msg := checkJSONTree(root); msg != "" {
				t.Error(msg)
			}
		})
	}
}

func TestParseJSONErrores(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"documento vacío", ``, `1:1: documento vacío: no hay ningún valor JSON`},
		{"solo espacios", "  \n\t", `2:2: documento vacío: no hay ningún valor JSON`},
		{"llave sin cerrar", `{"a": 1`, `1:1: '{' sin cerrar: falta '}' antes del final del fichero`},
		{"corchete sin cerrar", "[1, 2,\n 3", `1:1: '[' sin cerrar: falta ']' antes del final del fichero`},
		{"se señala el más interno sin cerrar", "{\"a\": [1, {\"b\": 2}", `1:7: '[' sin cerrar: falta ']' antes del final del fichero`},
		{"sin cerrar tras una clave", `{"a"`, `1:1: '{' sin cerrar: falta '}' antes del final del fichero`},
		{"sin cerrar tras dos puntos", `{"a":`, `1:1: '{' sin cerrar: falta '}' antes del final del fichero`},
		{"sin cerrar tras una coma", `[1,`, `1:1: '[' sin cerrar: falta ']' antes del final del fichero`},
		{"cierre que no corresponde", `[1}`, `1:3: se esperaba ']' para cerrar el '[' abierto en 1:1 y se encontró '}'`},
		{"cierre de array en un objeto", "{\"a\": 1\n]", `2:1: se esperaba '}' para cerrar el '{' abierto en 1:1 y se encontró ']'`},
		{"llave cerrada con corchete", `{]`, `1:2: se esperaba una clave entre comillas dobles o '}' y se encontró ']'`},
		{"corchete cerrado con llave", `[}`, `1:2: se esperaba un valor o ']' y se encontró '}'`},
		{"coma sobrante en un array", `[1, 2,]`, `1:6: coma sobrante: tras ',' se esperaba otro elemento y se encontró ']'`},
		{"coma sobrante en un objeto", "{\"a\": 1,\n}", `1:8: coma sobrante: tras ',' se esperaba otra clave y se encontró '}'`},
		{"coma que falta en un array", `[1 2]`, `1:4: falta una coma: se esperaba ',' o ']' y se encontró el número 2`},
		{"coma que falta entre objetos", `[{} {}]`, `1:5: falta una coma: se esperaba ',' o ']' y se encontró '{'`},
		{"coma que falta en un objeto", "{\"a\": 1\n \"b\": 2}", `2:2: falta una coma: se esperaba ',' o '}' y se encontró una cadena`},
		{"dos puntos en un array", `[1: 2]`, `1:3: se esperaba ',' o ']' y se encontró ':'`},
		{"dos puntos de más", `{"a": 1: 2}`, `1:8: se esperaba ',' o '}' y se encontró ':'`},
		{"clave que no es cadena", `{1: 2}`, `1:2: se esperaba una clave entre comillas dobles o '}' y se encontró el número 1`},
		{"clave true", `{true: 2}`, `1:2: se esperaba una clave entre comillas dobles o '}' y se encontró true`},
		{"clave que no es cadena tras coma", `{"a": 1, 2: 3}`, `1:10: se esperaba una clave entre comillas dobles tras ',' y se encontró el número 2`},
		{"objeto como clave", `{{}: 1}`, `1:2: se esperaba una clave entre comillas dobles o '}' y se encontró '{'`},
		{"falta dos puntos", `{"a" 1}`, `1:6: se esperaba ':' tras la clave "a" y se encontró el número 1`},
		{"coma en lugar de dos puntos", `{"a", "b"}`, `1:5: se esperaba ':' tras la clave "a" y se encontró ','`},
		{"valor que falta en un objeto", `{"a":}`, `1:6: falta el valor de la clave "a": se esperaba un valor y se encontró '}'`},
		{"valor que falta antes de una coma", `{"a": , "b": 1}`, `1:7: falta el valor de la clave "a": se esperaba un valor y se encontró ','`},
		{"valor que falta al principio de un array", `[,1]`, `1:2: se esperaba un valor o ']' y se encontró ','`},
		{"dos comas seguidas", `[1,,2]`, `1:4: se esperaba un valor tras ',' y se encontró ','`},
		{"cierre como raíz", `]`, `1:1: se esperaba un valor y se encontró ']'`},
		{"coma como raíz", `,`, `1:1: se esperaba un valor y se encontró ','`},
		{"contenido tras el valor raíz", "{\"a\": 1}\n{\"b\": 2}", `2:1: contenido tras el valor raíz: se esperaba el final del fichero y se encontró '{'`},
		{"cierre de más", `[1]]`, `1:4: contenido tras el valor raíz: se esperaba el final del fichero y se encontró ']'`},
		{"dos valores simples", `1 2`, `1:3: contenido tras el valor raíz: se esperaba el final del fichero y se encontró el número 2`},
		{"clave repetida", "{\"a\": 1,\n \"b\": 2,\n \"a\": 3}", `3:2: clave "a" repetida (ya aparece en 1:2)`},
		{"clave repetida escrita con escapes", `{"ñ": 1, "\u00f1": 2}`, `1:10: clave "ñ" repetida (ya aparece en 1:2)`},
		{"clave repetida en un objeto anidado", `[{"x": {"k": 1, "k": 2}}]`, `1:17: clave "k" repetida (ya aparece en 1:9)`},

		// Los errores del tokenizer se devuelven tal cual.
		{"clave sin comillas", `{a: 1}`, `1:2: texto 'a' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"comilla sin cerrar", "{\"a\": \"x}\n", `1:7: cadena sin cerrar: falta '"' antes del final de la línea (un salto de línea dentro de una cadena se escribe \n)`},
		{"número mal escrito", `[01]`, `1:3: número no válido: no se admiten ceros a la izquierda`},
		{"escape no admitido", `["\u0001"]`, `1:3: el escape \u0001 produce el carácter de control U+0001, no admitido: no puede representarse en las salidas XML`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := parseJSON(tt.src)
			if err == nil {
				t.Fatalf("se esperaba el error %q y se obtuvo el árbol:\n%s", tt.want, model.Dump(root))
			}
			if err.Error() != tt.want {
				t.Errorf("error:\nobtenido: %s\nesperado: %s", err, tt.want)
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

// TestParseJSONErrorDelTokenizer comprueba que el error del tokenizer se
// devuelve tal cual, sin envolverlo, aunque no sea un *token.PosError.
func TestParseJSONErrorDelTokenizer(t *testing.T) {
	want := errors.New("fallo del tokenizer")
	tk := &fakeTokenizer{
		toks: []token.Token{{Kind: token.LBracket, Line: 1, Col: 1}},
		err:  want,
	}
	if _, err := ParseJSON(tk); err != want {
		t.Errorf("error = %v, se esperaba %v", err, want)
	}
}

// TestParseJSONTokenInesperado comprueba que un token que no es de JSON
// (por ejemplo, de un tokenizer HTML) produce un error con posición.
func TestParseJSONTokenInesperado(t *testing.T) {
	tk := &fakeTokenizer{toks: []token.Token{
		{Kind: token.LBracket, Line: 1, Col: 1},
		{Kind: token.StartTag, Value: "p", Line: 1, Col: 2},
	}}
	_, err := ParseJSON(tk)
	if err == nil || err.Error() != "1:2: token StartTag inesperado en un documento JSON" {
		t.Errorf("error = %v", err)
	}
}

// TestParseJSONIDs comprueba los IDs concretos de un árbol pequeño.
func TestParseJSONIDs(t *testing.T) {
	root, err := parseJSON(`{"a": [1, {"b": null}], "c": "x"}`)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	var walk func(n *model.Node)
	walk = func(n *model.Node) {
		got = append(got, strconv.Itoa(n.ID)+":"+n.Label)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)

	want := []string{`0:{}`, `1:a`, `2:[]`, `3:1`, `4:{}`, `5:b`, `6:null`, `7:c`, `8:"x"`}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("preorden = %v, se esperaba %v", got, want)
	}
}

// TestParseJSONEjemplo compara el árbol de ejemplos/ok/anidado.json con el
// que se obtiene de encoding/json y comprueba sus nodos.
func TestParseJSONEjemplo(t *testing.T) {
	src, err := os.ReadFile("../../ejemplos/ok/anidado.json")
	if err != nil {
		t.Fatal(err)
	}

	root, err := parseJSON(string(src))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want, err := stdDump(string(src))
	if err != nil {
		t.Fatalf("encoding/json: %v", err)
	}
	if got := model.Dump(root); got != want {
		t.Errorf("\nobtenido:\n%s\nesperado (encoding/json):\n%s", got, want)
	}
	if msg := checkJSONTree(root); msg != "" {
		t.Error(msg)
	}
}

// TestParseJSONComoEncodingJSON compara, para varios documentos válidos, el
// árbol con el que se construye a partir de encoding/json.
func TestParseJSONComoEncodingJSON(t *testing.T) {
	docs := []string{
		`{"nombre": "Ana", "edad": 30, "tags": ["x", null]}`,
		`[1, -2.5e+3, true, false, null, "\u00e9\ud83d\ude00\n\"\\\/"]`,
		`{"z": {"y": {"x": [[], {}, [[{"w": 0}]]]}}, "a": 1E-7}`,
		`"solo una cadena"`,
		`-0.0`,
		`{"a\tb": "c\r\nd", "": ""}`,
	}
	for _, src := range docs {
		root, err := parseJSON(src)
		if err != nil {
			t.Errorf("%s: error inesperado: %v", src, err)
			continue
		}
		want, err := stdDump(src)
		if err != nil {
			t.Fatalf("%s: encoding/json: %v", src, err)
		}
		if got := model.Dump(root); got != want {
			t.Errorf("%s\nobtenido:\n%s\nesperado (encoding/json):\n%s", src, got, want)
		}
	}
}

// TestParseJSONProfundo comprueba que un array anidado 100 000 veces no
// agota la pila del parser (que no es recursivo) ni la del aplanado.
func TestParseJSONProfundo(t *testing.T) {
	const depth = 100000
	root, err := parseJSON(strings.Repeat("[", depth) + strings.Repeat("]", depth))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	g := model.Flatten(root)
	if len(g.Nodes) != depth || len(g.Edges) != depth-1 {
		t.Errorf("%d nodos y %d aristas, se esperaban %d y %d", len(g.Nodes), len(g.Edges), depth, depth-1)
	}

	// Sin cerrar, el error señala el corchete más interno.
	_, err = parseJSON(strings.Repeat("[", depth))
	if want := "1:100000: '[' sin cerrar: falta ']' antes del final del fichero"; err == nil || err.Error() != want {
		t.Errorf("error = %v, se esperaba %q", err, want)
	}
}

// checkJSONTree comprueba que los IDs están en preorden y que la forma del
// árbol es la del modelo JSON: cada clave tiene exactamente un hijo, los
// valores simples son hojas y los hijos de un objeto son claves. Devuelve
// un mensaje con el primer fallo, o "" si no hay ninguno.
func checkJSONTree(root *model.Node) string {
	if msg := checkPreorder(root); msg != "" {
		return msg
	}

	var walk func(n *model.Node) string
	walk = func(n *model.Node) string {
		switch {
		case n.Kind == model.JSONKey && len(n.Children) != 1:
			return "la clave " + n.Label + " no tiene exactamente un hijo"
		case n.Kind == model.JSONValue && len(n.Children) != 0:
			return "el valor " + n.Label + " tiene hijos"
		}
		for _, c := range n.Children {
			if n.Kind == model.JSONObject && c.Kind != model.JSONKey {
				return "un hijo del objeto no es una clave: " + c.Label
			}
			if msg := walk(c); msg != "" {
				return msg
			}
		}
		return ""
	}

	return walk(root)
}

// stdDump construye con encoding/json el Dump que debería dar ParseJSON
// para src, o devuelve un error si src no es JSON válido. Usa la lectura
// por tokens de encoding/json para conservar el orden de las claves.
func stdDump(src string) (string, error) {
	src = strings.TrimPrefix(src, "\uFEFF")
	if !json.Valid([]byte(src)) {
		return "", errors.New("JSON no válido")
	}

	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var b strings.Builder
	put := func(depth int, label string) {
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString(strings.ReplaceAll(label, "\n", `\n`))
		b.WriteString("\n")
	}

	var walk func(depth int) error
	walk = func(depth int) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case json.Delim:
			if v == '{' {
				put(depth, "{}")
				for dec.More() {
					key, err := dec.Token()
					if err != nil {
						return err
					}
					put(depth+1, key.(string))
					if err := walk(depth + 2); err != nil {
						return err
					}
				}
			} else {
				put(depth, "[]")
				for dec.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			}
			_, err = dec.Token()
			return err
		case string:
			put(depth, `"`+v+`"`)
		case json.Number:
			put(depth, v.String())
		case bool:
			put(depth, strconv.FormatBool(v))
		case nil:
			put(depth, "null")
		}
		return nil
	}

	err := walk(0)

	return b.String(), err
}

// FuzzParseJSON prueba el parser con entradas aleatorias. Con «go test»
// solo ejecuta las semillas; con «go test -fuzz=FuzzParseJSON» genera
// entradas nuevas. Comprueba que, para cualquier entrada:
//   - no hay pánico;
//   - todo error es un *token.PosError con posición válida;
//   - si no hay error, los IDs son 0..n-1 en preorden, el árbol tiene la
//     forma del modelo JSON y coincide con el que se obtiene de
//     encoding/json (que, por tanto, también acepta la entrada).
func FuzzParseJSON(f *testing.F) {
	seeds := []string{
		"", "{}", "[]", `{"a": [1, {"b": null}], "c": "x"}`, `[1}`, `[1,]`, `{"a":1,}`,
		`[1 2]`, `{a:1}`, `{1:2}`, `{"a" 1}`, `{"a":}`, `[,1]`, `[1] 2`, `{"a":1,"a":2}`,
		`"\ud83d\ude00"`, `-1.5e-3`, "\uFEFF[\r\n]", `[[[[[]]]]]`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		root, err := parseJSON(src)
		if err != nil {
			pe, ok := err.(*token.PosError)
			if !ok || pe.Line < 1 || pe.Col < 1 {
				t.Fatalf("error sin posición válida: %#v", err)
			}
			return
		}

		if msg := checkJSONTree(root); msg != "" {
			t.Fatal(msg)
		}
		want, err := stdDump(src)
		if err != nil {
			t.Fatalf("ParseJSON acepta %q, pero encoding/json no: %v", src, err)
		}
		if got := model.Dump(root); got != want {
			t.Fatalf("árbol distinto del de encoding/json para %q:\nobtenido:\n%s\nesperado:\n%s", src, got, want)
		}
	})
}
