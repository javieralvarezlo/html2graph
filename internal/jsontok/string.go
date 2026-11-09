package jsontok

import (
	"strconv"
	"strings"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// escapes son los escapes de un carácter (lo que sigue a la '\') y el
// carácter que producen. El escape \u se trata aparte.
var escapes = map[rune]rune{
	'"':  '"',
	'\\': '\\',
	'/':  '/',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
}

// readString lee una cadena desde su comilla de apertura hasta la de cierre
// y devuelve su contenido sin comillas y con los escapes resueltos.
func (t *Tokenizer) readString() (string, error) {
	line, col := t.r.Pos()
	t.r.Next()

	var b strings.Builder
	for {
		c, ok := t.r.Peek()
		_, more := t.r.PeekAt(1)
		switch {
		// El fichero termina dentro de la cadena, quizá justo tras una '\'.
		// Se señala la comilla que la abrió.
		case !ok, c == '\\' && !more:
			return "", token.Errorf(line, col, `cadena sin cerrar: falta '"' antes del final del fichero`)

		// Una cadena JSON no puede ocupar varias líneas, así que el salto de
		// línea indica que falta la comilla de cierre en esta.
		case c == '\n':
			return "", token.Errorf(line, col,
				`cadena sin cerrar: falta '"' antes del final de la línea (un salto de línea dentro de una cadena se escribe \n)`)

		case c == '"':
			t.r.Next()
			return b.String(), nil

		case c == '\\':
			decoded, err := t.readEscape()
			if err != nil {
				return "", err
			}
			b.WriteRune(decoded)

		// El resto de caracteres de control (el lector solo deja pasar el
		// tabulador) tienen que escribirse con un escape.
		case c < 0x20:
			return "", errAt(t.r,
				`%s sin escapar dentro de una cadena: JSON exige escribir los caracteres de control con un escape, como \t`, token.Describe(c))

		default:
			t.r.Next()
			b.WriteRune(c)
		}
	}
}

// readEscape lee el escape que empieza en la '\' actual, que va seguida de
// algún carácter, y devuelve el carácter que produce. Un par sustituto
// (😀) se lee entero y produce un solo carácter (U+1F600).
func (t *Tokenizer) readEscape() (rune, error) {
	line, col := t.r.Pos()
	t.r.Next()
	c, _ := t.r.Next()

	// Escapes de un carácter.
	if c != 'u' {
		decoded, ok := escapes[c]
		if !ok {
			return 0, token.Errorf(line, col,
				`escape no válido: tras '\' viene %s; se admiten \" \\ \/ \b \f \n \r \t y \uXXXX`, token.Describe(c))
		}
		return allowed(decoded, `\`+string(c), line, col)
	}

	// \uXXXX: cuatro cifras hexadecimales con el código de una unidad
	// UTF-16. text es el escape tal como está escrito, para los mensajes.
	code, hex, err := t.readHex(line, col)
	if err != nil {
		return 0, err
	}
	text := `\u` + hex

	switch {
	// Una mitad baja sin una alta delante no forma ningún carácter.
	case code >= 0xDC00 && code <= 0xDFFF:
		return 0, token.Errorf(line, col, "escape %s: mitad baja de un par sustituto sin su mitad alta delante", text)

	// Una mitad alta tiene que ir seguida de otro \u con la mitad baja; las
	// dos juntas dan el carácter.
	case code >= 0xD800 && code <= 0xDBFF:
		lowLine, lowCol := t.r.Pos()
		if !t.r.HasPrefix(`\u`) {
			return 0, errLoneHigh(text, line, col)
		}
		t.r.Skip(2)
		low, lowHex, err := t.readHex(lowLine, lowCol)
		if err != nil {
			return 0, err
		}
		if low < 0xDC00 || low > 0xDFFF {
			return 0, errLoneHigh(text, line, col)
		}
		text += `\u` + lowHex
		code = 0x10000 + (code-0xD800)<<10 + (low - 0xDC00)
	}

	return allowed(code, text, line, col)
}

// readHex lee las cuatro cifras hexadecimales que siguen a \u y devuelve su
// valor y las cifras tal como están escritas. line y col son la posición de
// la '\' del escape, donde se señala el error si faltan cifras.
func (t *Tokenizer) readHex(line, col int) (rune, string, error) {
	var hex strings.Builder
	for i := 0; i < 4; i++ {
		c, ok := t.r.Peek()
		if !ok || !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return 0, "", token.Errorf(line, col, `escape \u incompleto: se esperaban cuatro cifras hexadecimales tras \u`)
		}
		t.r.Next()
		hex.WriteRune(c)
	}

	// Cuatro cifras hexadecimales siempre caben en 16 bits.
	code, _ := strconv.ParseUint(hex.String(), 16, 16)

	return rune(code), hex.String(), nil
}

// allowed devuelve c si cumple token.IsAllowed, los mismos caracteres que el
// lector admite escritos tal cual; si no, un error en la posición del escape
// text que lo produce.
func allowed(c rune, text string, line, col int) (rune, error) {
	if !token.IsAllowed(c) {
		return 0, token.Errorf(line, col,
			"el escape %s produce el %s, no admitido: no puede representarse en las salidas XML", text, token.DescribeDisallowed(c))
	}

	return c, nil
}

// errLoneHigh devuelve el error de una mitad alta de par sustituto (el
// escape text, en line:col) que no va seguida de su mitad baja.
func errLoneHigh(text string, line, col int) error {
	return token.Errorf(line, col,
		`escape %s: mitad alta de un par sustituto sin su mitad baja (se esperaba a continuación un escape \uDC00 a \uDFFF)`, text)
}
