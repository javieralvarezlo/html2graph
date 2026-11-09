package htmltok

import (
	"os"
	"strings"
	"testing"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// tokenize lee src entero y devuelve cada token con el formato de
// token.Token.String, hasta EOF o hasta el primer error.
func tokenize(src string) ([]string, error) {
	tk := New(src)
	var out []string
	for {
		tok, err := tk.Next()
		if err != nil {
			return out, err
		}
		out = append(out, tok.String())
		if tok.Kind == token.EOF {
			return out, nil
		}
	}
}

// compare informa de las diferencias entre dos listas de tokens, línea a
// línea, para que sea fácil ver cuál falla.
func compare(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("\nobtenido:\n  %s\nesperado:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestEnunciado comprueba la secuencia completa de tokens del HTML del
// enunciado (ejemplos/ok/enunciado.html), con sus posiciones.
func TestEnunciado(t *testing.T) {
	src, err := os.ReadFile("../../ejemplos/ok/enunciado.html")
	if err != nil {
		t.Fatal(err)
	}

	got, err := tokenize(string(src))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	compare(t, got, []string{
		`1:1 StartTag "html"`,
		`1:7 Text "\n "`,
		`2:2 StartTag "head"`,
		`2:8 StartTag "title"`,
		`2:15 Text "Hoja de enlaces"`,
		`2:30 EndTag "title"`,
		`2:38 EndTag "head"`,
		`2:45 Text "\n "`,
		`3:2 StartTag "body"`,
		`3:8 Text "\n  "`,
		`4:3 StartTag "h1"`,
		`4:7 Text "Ejemplo2"`,
		`4:15 EndTag "h1"`,
		`4:20 Text "\n  "`,
		`5:3 StartTag "p"`,
		`5:6 Text "Este texto tiene un\n  "`,
		`6:3 StartTag "a" [href="http://www.issi.uned.es"]`,
		`6:37 Text " enlace"`,
		`6:44 EndTag "a"`,
		`6:48 Text "\n     y un texto con "`,
		`7:21 StartTag "em"`,
		`7:25 Text "énfasis"`,
		`7:32 EndTag "em"`,
		`7:37 EndTag "p"`,
		`7:41 Text "\n\n  "`,
		`9:3 StartTag "p"`,
		`9:6 Text "Este texto NO tiene otro enlace\n     y un texto con "`,
		`10:21 StartTag "b"`,
		`10:24 Text "negrita "`,
		`10:32 EndTag "b"`,
		`10:36 EndTag "p"`,
		`10:40 Text "\n  "`,
		`11:3 EndTag "body"`,
		`11:10 Text "\n"`,
		`12:1 EndTag "html"`,
		`12:8 Text "\n"`,
		`13:1 EOF`,
	})
}

func TestTokensCorrectos(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"vacío", "", []string{`1:1 EOF`}},
		{"solo texto", "hola", []string{`1:1 Text "hola"`, `1:5 EOF`}},
		{
			"atributos con y sin comillas y sin valor",
			`<input type="text" name='n' value=v disabled>`,
			[]string{`1:1 StartTag "input" [type="text" name="n" value="v" disabled=""]`, `1:46 EOF`},
		},
		{
			"nombres en minúsculas",
			`<DIV ID="X"></DIV>`,
			[]string{`1:1 StartTag "div" [id="X"]`, `1:13 EndTag "div"`, `1:19 EOF`},
		},
		{
			"espacios alrededor de =",
			`<a href = "x" >`,
			[]string{`1:1 StartTag "a" [href="x"]`, `1:16 EOF`},
		},
		{
			"comillas simples con comillas dobles dentro",
			`<a t='di "hola"'>`,
			[]string{`1:1 StartTag "a" [t="di \"hola\""]`, `1:18 EOF`},
		},
		{
			"autocerradas",
			`<br/><br />`,
			[]string{`1:1 SelfClosing "br"`, `1:6 SelfClosing "br"`, `1:12 EOF`},
		},
		{
			"autocerrada con atributos",
			`<img src="a.png" alt=""/>`,
			[]string{`1:1 SelfClosing "img" [src="a.png" alt=""]`, `1:26 EOF`},
		},
		{
			"atributo sin valor antes de /",
			`<input disabled/>`,
			[]string{`1:1 SelfClosing "input" [disabled=""]`, `1:18 EOF`},
		},
		{
			"cierre con espacio antes de >",
			`</p >`,
			[]string{`1:1 EndTag "p"`, `1:6 EOF`},
		},
		{
			"nombres con guion, subrayado, dos puntos y punto",
			`<my-tag data-x_y:z.w="1">`,
			[]string{`1:1 StartTag "my-tag" [data-x_y:z.w="1"]`, `1:26 EOF`},
		},
		{
			"letras no inglesas en el nombre",
			`<año>`,
			[]string{`1:1 StartTag "año"`, `1:6 EOF`},
		},
		{
			"comentario con etiquetas dentro",
			`<!-- hola <b> -->x`,
			[]string{`1:1 Comment " hola <b> "`, `1:18 Text "x"`, `1:19 EOF`},
		},
		{
			"doctype en mayúsculas",
			`<!DOCTYPE html>`,
			[]string{`1:1 Doctype "html"`, `1:16 EOF`},
		},
		{
			"doctype en minúsculas",
			`<!doctype html>`,
			[]string{`1:1 Doctype "html"`, `1:16 EOF`},
		},
		{
			"entidades en el texto",
			`&amp;&lt;&gt;&quot;&apos;&nbsp;&#233;&#xE9;&#XE9;`,
			[]string{`1:1 Text "&<>\"'\u00a0ééé"`, `1:50 EOF`},
		},
		{
			"entidades desconocidas o mal escritas se dejan literales",
			`&foo; &amp &#xZZ; &#0; & x`,
			[]string{`1:1 Text "&foo; &amp &#xZZ; &#0; & x"`, `1:27 EOF`},
		},
		{
			"entidades en un valor de atributo",
			`<a title="a&amp;b &lt;c&gt;">`,
			[]string{`1:1 StartTag "a" [title="a&b <c>"]`, `1:30 EOF`},
		},
		{
			"atributo sin valor justo antes de >",
			`<a x>`,
			[]string{`1:1 StartTag "a" [x=""]`, `1:6 EOF`},
		},
		{
			"espacio entre nombre de atributo y =",
			`<a x =1>`,
			[]string{`1:1 StartTag "a" [x="1"]`, `1:9 EOF`},
		},
		{
			"valor sin comillas con entidad, seguido de otro atributo",
			`<a x=a&amp;b y>`,
			[]string{`1:1 StartTag "a" [x="a&b" y=""]`, `1:16 EOF`},
		},
		{
			"varios espacios alrededor de =",
			"<a x  =\n  1>",
			[]string{`1:1 StartTag "a" [x="1"]`, `2:5 EOF`},
		},
		{
			"valor sin comillas seguido de /> (el / es parte del valor, como en HTML5)",
			`<x a=b/>`,
			[]string{`1:1 StartTag "x" [a="b/"]`, `1:9 EOF`},
		},
		{
			"valor entre comillas con salto de línea",
			"<a t=\"x\ny\">",
			[]string{`1:1 StartTag "a" [t="x\ny"]`, `2:4 EOF`},
		},
		{
			"entidades numéricas con ceros a la izquierda",
			`&#x0001F600;&#0000000065;`,
			[]string{`1:1 Text "😀A"`, `1:26 EOF`},
		},
		{
			"entidades numéricas no admitidas se dejan literales",
			`&#+65;&#x+41;&#1;&#x7F;&#xFFFE;&#xFDD0;&#;&#x;&abcdefghijklmnop;`,
			[]string{`1:1 Text "&#+65;&#x+41;&#1;&#x7F;&#xFFFE;&#xFDD0;&#;&#x;&abcdefghijklmnop;"`, `1:65 EOF`},
		},
		{
			"& al final del fichero",
			`a &amp`,
			[]string{`1:1 Text "a &amp"`, `1:7 EOF`},
		},
		{
			"doctype con > entre comillas",
			`<!DOCTYPE html SYSTEM "a>b">`,
			[]string{`1:1 Doctype "html SYSTEM \"a>b\""`, `1:29 EOF`},
		},
		{
			"comentario con -- dentro",
			`<!-- a -- b --->`,
			[]string{`1:1 Comment " a -- b -"`, `1:17 EOF`},
		},
		{
			"<!--> no cierra el comentario (limitación documentada)",
			`<!-->x-->`,
			[]string{`1:1 Comment ">x"`, `1:10 EOF`},
		},
		{
			"saltos de línea de Windows",
			"<p>\r\n<b>",
			[]string{`1:1 StartTag "p"`, `1:4 Text "\n"`, `2:1 StartTag "b"`, `2:4 EOF`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.src)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			compare(t, got, tt.want)
		})
	}
}

// TestFinDeFicheroEnCadaEstado comprueba que llegar al final de la entrada
// en mitad de una etiqueta es un error en cualquier estado de la máquina.
func TestFinDeFicheroEnCadaEstado(t *testing.T) {
	tests := []struct {
		estado string
		src    string
	}{
		{"AperturaCierre", `</`},
		{"NombreEtiqueta", `<p`},
		{"AntesAtributo", `<p `},
		{"NombreAtributo", `<p a`},
		{"DespuesNombreAtributo", `<p a `},
		{"AntesValor", `<p a=`},
		{"ValorSinComillas", `<p a=b`},
		{"DespuesValor", `<p a="b"`},
		{"AutoCierre", `<p/`},
	}
	want := `1:1: etiqueta sin cerrar: falta '>' antes del final del fichero`
	for _, tt := range tests {
		t.Run(tt.estado, func(t *testing.T) {
			_, err := tokenize(tt.src)
			if err == nil || err.Error() != want {
				t.Errorf("error = %v, se esperaba %q", err, want)
			}
		})
	}
}

func TestErroresLexicos(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    []string // tokens correctos antes del error
		wantErr string
	}{
		{
			"comilla doble sin cerrar",
			"<a href=\"x>\nhola",
			nil,
			`1:9: comilla sin cerrar en el valor del atributo 'href'`,
		},
		{
			"comilla simple sin cerrar",
			`<a href='x>`,
			nil,
			`1:9: comilla sin cerrar en el valor del atributo 'href'`,
		},
		{
			"etiqueta sin > al final",
			`<p`,
			nil,
			`1:1: etiqueta sin cerrar: falta '>' antes del final del fichero`,
		},
		{
			"etiqueta con atributos sin > al final",
			`<p class="x"`,
			nil,
			`1:1: etiqueta sin cerrar: falta '>' antes del final del fichero`,
		},
		{
			"< al final del texto",
			`hola <`,
			[]string{`1:1 Text "hola "`},
			`1:6: '<' al final del fichero (si es texto, debe escribirse &lt;)`,
		},
		{
			"< como único contenido",
			`<`,
			nil,
			`1:1: '<' al final del fichero (si es texto, debe escribirse &lt;)`,
		},
		{
			"comentario sin cerrar",
			`<!-- hola`,
			nil,
			`1:1: comentario sin cerrar: falta '-->'`,
		},
		{
			"doctype sin cerrar",
			`<!DOCTYPE html`,
			nil,
			`1:1: declaración <!DOCTYPE> sin cerrar: falta '>'`,
		},
		{
			"declaración no reconocida",
			`<![CDATA[x]]>`,
			nil,
			`1:1: declaración no reconocida: tras '<!' solo se admiten comentarios <!-- --> y <!DOCTYPE>`,
		},
		{
			"< suelto en el texto",
			`si a < b`,
			[]string{`1:1 Text "si a "`},
			`1:7: carácter ' ' (espacio) inválido tras '<' (si es texto, debe escribirse &lt;)`,
		},
		{
			"dígito tras <",
			`<1>`,
			nil,
			`1:2: carácter '1' inválido tras '<' (si es texto, debe escribirse &lt;)`,
		},
		{
			"carácter inválido en el nombre de la etiqueta",
			`<p$>`,
			nil,
			`1:3: carácter '$' inválido en el nombre de la etiqueta`,
		},
		{
			"espacio tras </",
			`</ p>`,
			nil,
			`1:3: carácter ' ' (espacio) inválido tras '</'`,
		},
		{
			"cierre con atributos",
			`</p class="x">`,
			nil,
			`1:5: una etiqueta de cierre no puede tener atributos`,
		},
		{
			"cierre autocerrado",
			`</br/>`,
			nil,
			`1:5: una etiqueta de cierre no puede ser autocerrada`,
		},
		{
			"atributo repetido aunque cambien las mayúsculas",
			`<a href="x" HREF="y">`,
			nil,
			`1:13: atributo 'href' repetido`,
		},
		{
			"falta el valor tras =",
			`<a href=>`,
			nil,
			`1:9: falta el valor del atributo 'href' tras '='`,
		},
		{
			"falta espacio entre atributos",
			`<a x="1"y="2">`,
			nil,
			`1:9: carácter 'y' inválido tras el valor de un atributo: falta un espacio de separación`,
		},
		{
			"comilla dentro de un valor sin comillas",
			`<a x=a"b>`,
			nil,
			`1:7: carácter '"' inválido en un valor de atributo sin comillas`,
		},
		{
			"= sin nombre de atributo",
			`<p =x>`,
			nil,
			`1:4: carácter '=' inválido en la etiqueta: se esperaba un atributo, '>' o '/>'`,
		},
		{
			"/ sin > a continuación",
			`<br/x>`,
			nil,
			`1:5: carácter 'x' inválido tras '/': se esperaba '>'`,
		},
		{
			"carácter inválido en el nombre de un atributo",
			`<a b$=1>`,
			nil,
			`1:5: carácter '$' inválido en el nombre del atributo`,
		},
		{
			"comilla simple en el nombre de un atributo",
			`<a b'>`,
			nil,
			`1:5: carácter "'" (comilla simple) inválido en el nombre del atributo`,
		},
		{
			"carácter invisible en una etiqueta",
			"<a\u200bb>",
			nil,
			`1:3: carácter U+200B inválido en el nombre de la etiqueta`,
		},
		{
			"carácter de control en el fichero",
			"<p>\n  <a title=\"x\vy\">",
			nil,
			`2:14: carácter de control U+000B no admitido: no puede representarse en las salidas XML; elimínelo del fichero`,
		},
		{
			"no-carácter en el fichero",
			"<p>\uFDD0</p>",
			nil,
			`1:4: no-carácter Unicode U+FDD0 no admitido: no puede representarse en las salidas XML; elimínelo del fichero`,
		},
		{
			"espacio de no separación en una etiqueta",
			"<a\u00a0b>",
			nil,
			`1:3: carácter U+00A0 inválido en el nombre de la etiqueta`,
		},
		{
			"acento combinante suelto",
			"<e\u0301>",
			nil,
			`1:3: carácter U+0301 inválido en el nombre de la etiqueta`,
		},
		{
			"símbolo no ASCII",
			`<p€>`,
			nil,
			`1:3: carácter '€' (U+20AC) inválido en el nombre de la etiqueta`,
		},
		{
			"= en una etiqueta de cierre",
			`</a =>`,
			nil,
			`1:5: carácter '=' inválido en la etiqueta de cierre: se esperaba '>'`,
		},
		{
			"doctype sin espacio",
			`<!DOCTYPEhtml>`,
			nil,
			`1:10: carácter 'h' inválido tras '<!DOCTYPE': se esperaba un espacio`,
		},
		{
			"palabra que empieza por doctype",
			`<!doctypes>`,
			nil,
			`1:10: carácter 's' inválido tras '<!DOCTYPE': se esperaba un espacio`,
		},
		{
			"doctype con comilla sin cerrar",
			`<!DOCTYPE html "a>`,
			nil,
			`1:1: declaración <!DOCTYPE> sin cerrar: falta '>'`,
		},
		{
			"fichero que no es UTF-8",
			"<p>\xe9nfasis</p>",
			nil,
			`1:4: el fichero no está codificado en UTF-8 (byte 0xE9 inválido); conviértalo a UTF-8`,
		},
		{
			"error en una línea posterior",
			"<p>\n  <b \"x\">",
			[]string{`1:1 StartTag "p"`, `1:4 Text "\n  "`},
			`2:6: carácter '"' inválido en la etiqueta: se esperaba un atributo, '>' o '/>'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.src)
			if err == nil {
				t.Fatalf("se esperaba el error %q y no hubo error; tokens: %v", tt.wantErr, got)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q\nse esperaba  %q", err.Error(), tt.wantErr)
			}
			if _, ok := err.(*token.PosError); !ok {
				t.Errorf("el error es %T, se esperaba *token.PosError", err)
			}
			compare(t, got, tt.want)
		})
	}
}

// TestRepeticionTrasFinal comprueba que, tras EOF o tras un error, Next
// devuelve siempre lo mismo.
func TestRepeticionTrasFinal(t *testing.T) {
	tk := New("x")
	tk.Next()
	for i := 0; i < 3; i++ {
		tok, err := tk.Next()
		if err != nil || tok.Kind != token.EOF {
			t.Fatalf("llamada %d tras EOF: %v, %v", i, tok, err)
		}
	}

	tk = New("<p")
	_, first := tk.Next()
	for i := 0; i < 3; i++ {
		_, err := tk.Next()
		if err == nil || err.Error() != first.Error() {
			t.Fatalf("llamada %d tras error: %v, se esperaba %v", i, err, first)
		}
	}
}

func TestDecodeEntity(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		wantOk bool
	}{
		{"amp", "&", true},
		{"lt", "<", true},
		{"gt", ">", true},
		{"quot", `"`, true},
		{"apos", "'", true},
		{"nbsp", "\u00a0", true},
		{"#65", "A", true},
		{"#x41", "A", true},
		{"#X41", "A", true},
		{"#x1F600", "😀", true},
		{"AMP", "", false},
		{"copy", "", false},
		{"#", "", false},
		{"#x", "", false},
		{"#0", "", false},
		{"#xD800", "", false},
		{"#x110000", "", false},
		{"#12a", "", false},
		{"#+65", "", false},
		{"#x+41", "", false},
		{"#-1", "", false},
		{"# 65", "", false},
		{"#9", "\t", true},
		{"#1", "", false},
		{"#13", "\r", true},
		{"#xC", "", false},
		{"#xB", "", false},
		{"#x7F", "", false},
		{"#x85", "", false},
		{"#xFFFE", "", false},
		{"#x1FFFF", "", false},
		{"#xFDD0", "", false},
		{"#x0001F600", "😀", true},
		{"#0000000065", "A", true},
		{"#99999999999999999999", "", false},
	}
	for _, tt := range tests {
		got, ok := decodeEntity(tt.name)
		if got != tt.want || ok != tt.wantOk {
			t.Errorf("decodeEntity(%q) = %q, %v; se esperaba %q, %v", tt.name, got, ok, tt.want, tt.wantOk)
		}
	}
}

// TestStatesCompleto comprueba que todos los estados tienen función: un
// estado sin ella provocaría un pánico al despacharlo.
func TestStatesCompleto(t *testing.T) {
	for st := stText; st <= stDoctype; st++ {
		if states[st] == nil {
			t.Errorf("el estado %d no tiene función", st)
		}
	}
}

// FuzzTokenizer prueba el tokenizer con entradas aleatorias. Con «go test»
// solo ejecuta las semillas; con «go test -fuzz=FuzzTokenizer» genera
// entradas nuevas. Comprueba que, para cualquier entrada:
//   - el tokenizer termina (como mucho un token por carácter, más EOF);
//   - las posiciones de los tokens nunca retroceden;
//   - todo error es un *token.PosError con posición válida.
func FuzzTokenizer(f *testing.F) {
	seeds := []string{
		"", "<", "</", "<p>", "a < b", "&amp;", "&#x41;", "<!-- x -->",
		"<!DOCTYPE html>", `<a href="x" b='y' c=z d>`, "<br/>", "<p\n>\r\n",
		"\xe9", "&#0000065;", `<!DOCTYPE a "b>c">`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		tk := New(src)
		lastLine, lastCol := 1, 1
		for i := 0; i <= len(src)+1; i++ {
			tok, err := tk.Next()
			if err != nil {
				pe, ok := err.(*token.PosError)
				if !ok || pe.Line < 1 || pe.Col < 1 {
					t.Fatalf("error sin posición válida: %#v", err)
				}
				return
			}
			if tok.Line < lastLine || (tok.Line == lastLine && tok.Col < lastCol) {
				t.Fatalf("la posición retrocede: %d:%d tras %d:%d", tok.Line, tok.Col, lastLine, lastCol)
			}
			lastLine, lastCol = tok.Line, tok.Col
			if tok.Kind == token.EOF {
				return
			}
		}
		t.Fatalf("el tokenizer no termina con %q", src)
	})
}
