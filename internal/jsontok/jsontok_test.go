package jsontok

import (
	"fmt"
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

func TestTokensCorrectos(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"vacío", "", []string{`1:1 EOF`}},
		{"solo espacios", " \t\n\r\n ", []string{`3:2 EOF`}},
		{
			"puntuación",
			"{}[]:,",
			[]string{`1:1 LBrace`, `1:2 RBrace`, `1:3 LBracket`, `1:4 RBracket`, `1:5 Colon`, `1:6 Comma`, `1:7 EOF`},
		},
		{
			"palabras clave",
			"true false null",
			[]string{`1:1 True "true"`, `1:6 False "false"`, `1:12 Null "null"`, `1:16 EOF`},
		},
		{
			"objeto con todos los tipos de valor",
			`{"a": [1, -2.5e+3, true, false, null, "x"]}`,
			[]string{
				`1:1 LBrace`, `1:2 String "a"`, `1:5 Colon`, `1:7 LBracket`,
				`1:8 Number "1"`, `1:9 Comma`, `1:11 Number "-2.5e+3"`, `1:18 Comma`,
				`1:20 True "true"`, `1:24 Comma`, `1:26 False "false"`, `1:31 Comma`,
				`1:33 Null "null"`, `1:37 Comma`, `1:39 String "x"`, `1:42 RBracket`,
				`1:43 RBrace`, `1:44 EOF`,
			},
		},
		{
			"posiciones en varias líneas, con saltos de Windows y tabuladores",
			"{\r\n\t\"clave\": 1,\r\n\t\"b\":\n\t\t[]\r\n}",
			[]string{
				`1:1 LBrace`, `2:2 String "clave"`, `2:9 Colon`, `2:11 Number "1"`, `2:12 Comma`,
				`3:2 String "b"`, `3:5 Colon`, `4:3 LBracket`, `4:4 RBracket`, `5:1 RBrace`, `5:2 EOF`,
			},
		},
		{
			"tokens pegados, sin espacios",
			`{"a":[1,"b",null]}`,
			[]string{
				`1:1 LBrace`, `1:2 String "a"`, `1:5 Colon`, `1:6 LBracket`, `1:7 Number "1"`, `1:8 Comma`,
				`1:9 String "b"`, `1:12 Comma`, `1:13 Null "null"`, `1:17 RBracket`, `1:18 RBrace`, `1:19 EOF`,
			},
		},
		{
			"las columnas cuentan caracteres, no bytes",
			`"ñandú" 1`,
			[]string{`1:1 String "ñandú"`, `1:9 Number "1"`, `1:10 EOF`},
		},
		{"marca BOM ignorada", "\uFEFF[]", []string{`1:1 LBracket`, `1:2 RBracket`, `1:3 EOF`}},
		{
			"palabras pegadas a la puntuación",
			"[true,false]null",
			[]string{`1:1 LBracket`, `1:2 True "true"`, `1:6 Comma`, `1:7 False "false"`, `1:12 RBracket`, `1:13 Null "null"`, `1:17 EOF`},
		},
		{"un número tras otro valor", `"a"1`, []string{`1:1 String "a"`, `1:4 Number "1"`, `1:5 EOF`}},
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

func TestCadenas(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"vacía", `""`, ""},
		{"acentos y emojis tal cual", `"Ñandú 😀 €"`, "Ñandú 😀 €"},
		{"espacios conservados", `"  dos  espacios  "`, "  dos  espacios  "},
		{"comilla y barra", `"di \"hola\" C:\\dir"`, `di "hola" C:\dir`},
		{"barra inclinada", `"a\/b"`, "a/b"},
		{"saltos y tabulador", `"uno\ndos\r\ntres\tcuatro"`, "uno\ndos\r\ntres\tcuatro"},
		{"\\u minúsculas y mayúsculas", `"\u00e9\u00C9\u20ac"`, "éÉ€"},
		{"\\u de un carácter ASCII", `"\u0041\u0022"`, `A"`},
		{"\\u de espacio de no separación", `"a\u00a0b"`, "a\u00a0b"},
		{"\\u de tabulador, salto de línea y retorno", `"\u0009\u000a\u000D"`, "\t\n\r"},
		{"par sustituto", `"\ud83d\ude00"`, "😀"},
		{"par sustituto en mayúsculas", `"\uD83D\uDE00"`, "😀"},
		{"dos pares seguidos (bandera)", `"\uD83C\uDDEA\uD83C\uDDF8"`, "🇪🇸"},
		{"último carácter de Unicode", `"\udbff\udffd"`, "\U0010FFFD"},
		{"comilla simple tal cual", `"it's"`, "it's"},
		{"llaves y corchetes dentro", `"{[:,]}"`, "{[:,]}"},
		{"U+FFFD escrito tal cual", "\"\uFFFD\"", "\uFFFD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := New(tt.src).Next()
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if tok.Kind != token.String || tok.Value != tt.want {
				t.Errorf("token = %v, se esperaba String %q", tok, tt.want)
			}
		})
	}
}

// TestNumeros comprueba la gramática de los números: los válidos se
// devuelven como literal tal cual, sin convertirlos.
func TestNumeros(t *testing.T) {
	valid := []string{
		"0", "-0", "1", "-1", "10", "1234567890", "0.5", "-0.5", "1.0", "3.14159",
		"1e3", "1E3", "1e+3", "1e-3", "1E-03", "0e0", "-0.0e-0", "2.5E+10",
		"123456789012345678901234567890", "1.000000000000000000000000001",
		"1e999", "-1e-999",
	}
	for _, src := range valid {
		toks, err := tokenize(src)
		if err != nil {
			t.Errorf("%s: error inesperado: %v", src, err)
			continue
		}
		want := []string{`1:1 Number "` + src + `"`, fmt.Sprintf("1:%d EOF", len(src)+1)}
		compare(t, toks, want)
	}
}

func TestErroresLexicos(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    []string // tokens correctos antes del error
		wantErr string
	}{
		// Cadenas.
		{
			"cadena sin cerrar al final del fichero",
			`["abc`,
			[]string{`1:1 LBracket`},
			`1:2: cadena sin cerrar: falta '"' antes del final del fichero`,
		},
		{
			"cadena sin cerrar tras una barra",
			`"abc\`,
			nil,
			`1:1: cadena sin cerrar: falta '"' antes del final del fichero`,
		},
		{
			"cadena sin cerrar al final de la línea",
			"{\"a\": \"abc,\n\"b\": 1}",
			[]string{`1:1 LBrace`, `1:2 String "a"`, `1:5 Colon`},
			`1:7: cadena sin cerrar: falta '"' antes del final de la línea (un salto de línea dentro de una cadena se escribe \n)`,
		},
		{
			"retorno de carro dentro de una cadena",
			"\"a\rb\"",
			nil,
			`1:1: cadena sin cerrar: falta '"' antes del final de la línea (un salto de línea dentro de una cadena se escribe \n)`,
		},
		{
			"tabulador sin escapar",
			"\"a\tb\"",
			nil,
			`1:3: tabulador sin escapar dentro de una cadena: JSON exige escribir los caracteres de control con un escape, como \t`,
		},
		{
			"escape desconocido",
			`"a\x"`,
			nil,
			`1:3: escape no válido: tras '\' viene 'x'; se admiten \" \\ \/ \b \f \n \r \t y \uXXXX`,
		},
		{
			"escape de comilla simple",
			`"\'"`,
			nil,
			`1:2: escape no válido: tras '\' viene "'" (comilla simple); se admiten \" \\ \/ \b \f \n \r \t y \uXXXX`,
		},
		{
			"barra seguida de salto de línea",
			"\"a\\\nb\"",
			nil,
			`1:3: escape no válido: tras '\' viene salto de línea; se admiten \" \\ \/ \b \f \n \r \t y \uXXXX`,
		},
		{
			"\\u con menos de cuatro cifras",
			`"\u12"`,
			nil,
			`1:2: escape \u incompleto: se esperaban cuatro cifras hexadecimales tras \u`,
		},
		{
			"\\u con una cifra que no es hexadecimal",
			`"\u00g0"`,
			nil,
			`1:2: escape \u incompleto: se esperaban cuatro cifras hexadecimales tras \u`,
		},
		{
			"\\u al final del fichero",
			`"\u00`,
			nil,
			`1:2: escape \u incompleto: se esperaban cuatro cifras hexadecimales tras \u`,
		},

		// Caracteres que producen los escapes y que no cumplen IsAllowed.
		{
			"\\b",
			`"a\b"`,
			nil,
			`1:3: el escape \b produce el carácter de control U+0008, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\f",
			`"\f"`,
			nil,
			`1:2: el escape \f produce el carácter de control U+000C, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\u0000",
			`"\u0000"`,
			nil,
			`1:2: el escape \u0000 produce el carácter de control U+0000, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\u0001",
			`"\u0001"`,
			nil,
			`1:2: el escape \u0001 produce el carácter de control U+0001, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\u007F (DEL)",
			`"\u007F"`,
			nil,
			`1:2: el escape \u007F produce el carácter de control U+007F, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\u0085 (control C1)",
			`"\u0085"`,
			nil,
			`1:2: el escape \u0085 produce el carácter de control U+0085, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\uFFFE (no-carácter)",
			`"x\uFFFE"`,
			nil,
			`1:3: el escape \uFFFE produce el no-carácter Unicode U+FFFE, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"\\uFDD0 (no-carácter)",
			`"\ufdd0"`,
			nil,
			`1:2: el escape \ufdd0 produce el no-carácter Unicode U+FDD0, no admitido: no puede representarse en las salidas XML`,
		},
		{
			"par sustituto que da un no-carácter",
			`"\uD83F\uDFFE"`,
			nil,
			`1:2: el escape \uD83F\uDFFE produce el no-carácter Unicode U+1FFFE, no admitido: no puede representarse en las salidas XML`,
		},

		// Pares sustitutos mal formados.
		{
			"mitad alta sola",
			`"\ud83d"`,
			nil,
			`1:2: escape \ud83d: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`,
		},
		{
			"mitad alta seguida de un carácter",
			`"\ud83dx"`,
			nil,
			`1:2: escape \ud83d: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`,
		},
		{
			"mitad alta seguida de otro escape",
			`"\ud83d\n"`,
			nil,
			`1:2: escape \ud83d: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`,
		},
		{
			"dos mitades altas",
			`"\ud83d\ud83d"`,
			nil,
			`1:2: escape \ud83d: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`,
		},
		{
			"mitad alta seguida de un carácter normal escapado",
			`"\uD83D\u0041"`,
			nil,
			`1:2: escape \uD83D: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`,
		},
		{
			"mitad baja incompleta",
			`"\uD83D\uDE0"`,
			nil,
			`1:8: escape \u incompleto: se esperaban cuatro cifras hexadecimales tras \u`,
		},
		{
			"mitad baja sola",
			`"ab\ude00"`,
			nil,
			`1:4: escape \ude00: mitad baja de un par sustituto sin su mitad alta delante`,
		},
		{
			"par invertido",
			`"\ude00\ud83d"`,
			nil,
			`1:2: escape \ude00: mitad baja de un par sustituto sin su mitad alta delante`,
		},

		// Números.
		{"cero a la izquierda", `01`, nil, `1:2: número no válido: no se admiten ceros a la izquierda`},
		{"cero a la izquierda negativo", `[-007]`, []string{`1:1 LBracket`}, `1:4: número no válido: no se admiten ceros a la izquierda`},
		{"punto sin decimales", `1.`, nil, `1:3: número no válido: se esperaba una cifra tras '.'`},
		{"punto seguido de exponente", `1.e5`, nil, `1:3: número no válido: se esperaba una cifra tras '.'`},
		{"punto y coma", `[1.,2]`, []string{`1:1 LBracket`}, `1:4: número no válido: se esperaba una cifra tras '.'`},
		{"empieza por punto", `.5`, nil, `1:1: carácter '.' inválido: un número no puede empezar por '.' (escriba 0.5 en lugar de .5)`},
		{"signo más", `+1`, nil, `1:1: carácter '+' inválido: un número no puede empezar por '+'`},
		{"exponente vacío", `1e`, nil, `1:3: número no válido: se esperaba una cifra en el exponente`},
		{"exponente solo con signo", `1E+`, nil, `1:4: número no válido: se esperaba una cifra en el exponente`},
		{"menos solo", `-`, nil, `1:2: número no válido: se esperaba una cifra tras '-'`},
		{"menos y espacio", `- 1`, nil, `1:2: número no válido: se esperaba una cifra tras '-'`},
		{"menos infinito", `-Infinity`, nil, `1:2: número no válido: se esperaba una cifra tras '-'`},
		{"hexadecimal", `0x1F`, nil, `1:2: número no válido: carácter 'x' inesperado tras 0`},
		{"dos puntos decimales", `1.5.3`, nil, `1:4: número no válido: carácter '.' inesperado tras 1.5`},
		{"dos exponentes", `1e5e3`, nil, `1:4: número no válido: carácter 'e' inesperado tras 1e5`},
		{"letra pegada", `12abc`, nil, `1:3: número no válido: carácter 'a' inesperado tras 12`},
		{"cifras de otro alfabeto", `1٣`, nil, `1:2: número no válido: carácter '٣' (U+0663) inesperado tras 1`},

		// Palabras sin comillas.
		{"NaN", `NaN`, nil, `1:1: texto 'NaN' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"Infinity", `[Infinity]`, []string{`1:1 LBracket`}, `1:2: texto 'Infinity' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"True con mayúscula", `True`, nil, `1:1: 'True' no es un valor JSON: true, false y null se escriben en minúsculas`},
		{"NULL", `NULL`, nil, `1:1: 'NULL' no es un valor JSON: true, false y null se escriben en minúsculas`},
		{"nul incompleta", `nul`, nil, `1:1: texto 'nul' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"nulll con letra de más", `nulll`, nil, `1:1: texto 'nulll' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"truex", `truex`, nil, `1:1: texto 'truex' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{"palabra con cifras y subrayado", `true_1`, nil, `1:1: texto 'true_1' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},
		{
			"clave sin comillas",
			`{nombre: "Ana"}`,
			[]string{`1:1 LBrace`},
			`1:2: texto 'nombre' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`,
		},
		{"palabra con acentos", `[ñandú]`, []string{`1:1 LBracket`}, `1:2: texto 'ñandú' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null`},

		// Otros caracteres inválidos fuera de una cadena.
		{"comillas simples", `{'a': 1}`, []string{`1:1 LBrace`}, `1:2: carácter "'" (comilla simple) inválido: las cadenas y las claves van entre comillas dobles`},
		{"comentario", "[1, // uno\n2]", []string{`1:1 LBracket`, `1:2 Number "1"`, `1:3 Comma`}, `1:5: carácter '/' inválido: JSON no admite comentarios`},
		{"almohadilla", `#`, nil, `1:1: carácter '#' inválido`},
		{"punto y coma", `[1;2]`, []string{`1:1 LBracket`, `1:2 Number "1"`}, `1:3: carácter ';' inválido`},
		{"espacio de no separación", "[1,\u00a02]", []string{`1:1 LBracket`, `1:2 Number "1"`, `1:3 Comma`}, `1:4: carácter U+00A0 inválido`},
		{"comillas tipográficas", "“a”", nil, `1:1: carácter '“' (U+201C) inválido`},
		{"BOM fuera del principio", "[]\uFEFF", []string{`1:1 LBracket`, `1:2 RBracket`}, `1:3: carácter U+FEFF inválido`},

		// Errores del lector común: los mismos mensajes que en HTML.
		{"salto de página", "[1,\f2]", nil, `1:4: carácter de control U+000C no admitido: no puede representarse en las salidas XML; elimínelo del fichero`},
		{"tabulador vertical en una cadena", "\"a\vb\"", nil, `1:3: carácter de control U+000B no admitido: no puede representarse en las salidas XML; elimínelo del fichero`},
		{"no-carácter en una cadena", "\"\uFDD0\"", nil, `1:2: no-carácter Unicode U+FDD0 no admitido: no puede representarse en las salidas XML; elimínelo del fichero`},
		{"UTF-8 inválido", "[\"caf\xe9\"]", nil, `1:6: el fichero no está codificado en UTF-8 (byte 0xE9 inválido); conviértalo a UTF-8`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.src)
			if err == nil {
				t.Fatalf("se esperaba el error %q y no hubo error; tokens: %v", tt.wantErr, got)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error:\nobtenido: %s\nesperado: %s", err, tt.wantErr)
			}
			if _, ok := err.(*token.PosError); !ok {
				t.Errorf("el error es %T, se esperaba *token.PosError", err)
			}
			compare(t, got, tt.want)
		})
	}
}

// TestRepeticionTrasFinal comprueba que, tras EOF o tras un error, Next
// devuelve siempre lo mismo, y que el token de un error es el vacío.
func TestRepeticionTrasFinal(t *testing.T) {
	tk := New("1")
	tk.Next()
	for i := 0; i < 3; i++ {
		tok, err := tk.Next()
		if err != nil || tok.Kind != token.EOF || tok.Line != 1 || tok.Col != 2 {
			t.Fatalf("llamada %d tras EOF: %v, %v", i, tok, err)
		}
	}

	tk = New(`[01]`)
	tk.Next()
	_, first := tk.Next()
	for i := 0; i < 3; i++ {
		tok, err := tk.Next()
		if err == nil || err.Error() != first.Error() {
			t.Fatalf("llamada %d tras error: %v, se esperaba %v", i, err, first)
		}
		if tok.Kind != token.Invalid || tok.Line != 0 || tok.Value != "" {
			t.Fatalf("el token de un error debe ser el vacío: %v", tok)
		}
	}
}

// FuzzJSONTokenizer prueba el tokenizer con entradas aleatorias. Con «go
// test» solo ejecuta las semillas; con «go test -fuzz=FuzzJSONTokenizer»
// genera entradas nuevas. Comprueba que, para cualquier entrada:
//   - el tokenizer termina (como mucho un token por carácter, más EOF);
//   - las posiciones de los tokens nunca retroceden;
//   - todo error es un *token.PosError con posición válida y su token es
//     el vacío;
//   - las cadenas solo contienen caracteres admitidos (token.IsAllowed) y
//     los números solo cifras, signos, punto y exponente.
func FuzzJSONTokenizer(f *testing.F) {
	seeds := []string{
		"", "{", `{"a": [1, -2.5e+3, true, false, null]}`, `"\u00e9\ud83d\ude00"`,
		`"\ud83d"`, `"\b"`, `01`, `1.`, `-`, `1e+`, `nulll`, `True`, "\"a\n", `"\`,
		"\xe9", "\uFEFF[]", `'a'`, `[1 2]`,
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
				if tok.Kind != token.Invalid {
					t.Fatalf("el token de un error no es el vacío: %v", tok)
				}
				return
			}
			if tok.Line < lastLine || (tok.Line == lastLine && tok.Col < lastCol) {
				t.Fatalf("la posición retrocede: %d:%d tras %d:%d", tok.Line, tok.Col, lastLine, lastCol)
			}
			lastLine, lastCol = tok.Line, tok.Col

			switch tok.Kind {
			case token.EOF:
				return
			case token.String:
				for _, c := range tok.Value {
					if !token.IsAllowed(c) {
						t.Fatalf("cadena con un carácter no admitido %U: %q", c, tok.Value)
					}
				}
			case token.Number:
				if tok.Value == "" || strings.Trim(tok.Value, "0123456789-+.eE") != "" {
					t.Fatalf("número mal formado: %q", tok.Value)
				}
			}
		}
		t.Fatalf("el tokenizer no termina con %q", src)
	})
}
