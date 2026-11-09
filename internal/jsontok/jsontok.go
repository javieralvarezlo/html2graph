// Package jsontok implementa el tokenizer de JSON: convierte el texto de un
// documento JSON (RFC 8259) en una secuencia de tokens: llaves, corchetes,
// dos puntos, comas, cadenas, números y las palabras true, false y null.
//
// En la arquitectura del code munger (Herrington), es el primer bloque del
// camino JSON: lee la entrada con el mismo token.Reader que el tokenizer HTML
// (mismas posiciones y misma comprobación de UTF-8 y de caracteres admitidos)
// y entrega token.Token al parser a través de la interfaz token.Tokenizer.
// Solo se ocupa de la forma de cada token; que formen un documento lo
// comprueba el parser.
//
// En JSON el primer carácter de un token decide su clase, así que no hace
// falta una máquina de estados como la de HTML: Next salta los espacios, mira
// ese carácter y llama a la función que lee esa clase de token (cadena,
// número o palabra). Los errores léxicos se devuelven como *token.PosError, y
// la entrada nunca se repara.
package jsontok

import (
	"strings"
	"unicode"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// Tokenizer es el tokenizer de JSON. Implementa token.Tokenizer.
type Tokenizer struct {
	r *token.Reader

	// done indica que ya se devolvió EOF o un error; en ese caso Next
	// repite lastTok y lastErr.
	done    bool
	lastTok token.Token
	lastErr error
}

// Comprobación en compilación de que Tokenizer cumple la interfaz común.
var _ token.Tokenizer = (*Tokenizer)(nil)

// punctuation son los tokens de un solo carácter.
var punctuation = map[rune]token.Kind{
	'{': token.LBrace,
	'}': token.RBrace,
	'[': token.LBracket,
	']': token.RBracket,
	':': token.Colon,
	',': token.Comma,
}

// keywords son las palabras que JSON admite sin comillas.
var keywords = map[string]token.Kind{
	"true":  token.True,
	"false": token.False,
	"null":  token.Null,
}

// hints completan el mensaje de los caracteres inválidos más probables con
// la causa del error.
var hints = map[rune]string{
	'\'': ": las cadenas y las claves van entre comillas dobles",
	'+':  ": un número no puede empezar por '+'",
	'.':  ": un número no puede empezar por '.' (escriba 0.5 en lugar de .5)",
	'/':  ": JSON no admite comentarios",
}

// New crea un tokenizer que leerá el documento JSON src.
func New(src string) *Tokenizer {
	return &Tokenizer{r: token.NewReader(src)}
}

// Next devuelve el siguiente token del documento, o un *token.PosError si
// la entrada es léxicamente incorrecta. Tras devolver EOF o un error, repite
// ese mismo resultado en las llamadas siguientes.
func (t *Tokenizer) Next() (token.Token, error) {
	if t.done {
		return t.lastTok, t.lastErr
	}

	// Un fichero que no es UTF-8 válido no llega a tokenizarse.
	tok, err := token.Token{}, t.r.Err()
	if err == nil {
		tok, err = t.scan()
	}

	// EOF y los errores son definitivos: se recuerdan para repetirlos.
	if err != nil || tok.Kind == token.EOF {
		t.done = true
		t.lastTok, t.lastErr = tok, err
	}

	return tok, err
}

// scan salta los espacios y lee el token que empieza en el carácter
// siguiente, según cuál sea ese carácter.
func (t *Tokenizer) scan() (token.Token, error) {
	// Los espacios entre tokens no pertenecen a ninguno.
	for c, ok := t.r.Peek(); ok && isSpace(c); c, ok = t.r.Peek() {
		t.r.Next()
	}

	line, col := t.r.Pos()
	tok := token.Token{Line: line, Col: col}
	c, ok := t.r.Peek()
	if !ok {
		tok.Kind = token.EOF
		return tok, nil
	}

	kind, isPunct := punctuation[c]
	var err error
	switch {
	case isPunct:
		t.r.Next()
		tok.Kind = kind
	case c == '"':
		tok.Kind = token.String
		tok.Value, err = t.readString()
	case c == '-' || isDigit(c):
		tok.Kind = token.Number
		tok.Value, err = t.readNumber()
	case unicode.IsLetter(c):
		tok.Kind, tok.Value, err = t.readWord()

	// Cualquier otro carácter es inválido fuera de una cadena; si no tiene
	// pista, el mensaje solo lo nombra.
	default:
		hint := hints[c]
		err = errAt(t.r, "carácter %s inválido%s", token.Describe(c), hint)
	}

	// El token que acompaña a un error es el vacío.
	if err != nil {
		return token.Token{}, err
	}

	return tok, nil
}

// readWord lee una palabra sin comillas, que solo puede ser true, false o
// null, escritas en minúsculas.
func (t *Tokenizer) readWord() (token.Kind, string, error) {
	line, col := t.r.Pos()

	// La palabra se lee entera (letras, cifras y '_') para que el mensaje
	// de error la muestre completa: nulll, True, nombre.
	var b strings.Builder
	for c, ok := t.r.Peek(); ok && (unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'); c, ok = t.r.Peek() {
		t.r.Next()
		b.WriteRune(c)
	}
	word := b.String()

	kind, ok := keywords[word]
	if ok {
		return kind, word, nil
	}
	if _, ok := keywords[strings.ToLower(word)]; ok {
		return 0, "", token.Errorf(line, col, "'%s' no es un valor JSON: true, false y null se escriben en minúsculas", word)
	}

	return 0, "", token.Errorf(line, col,
		"texto '%s' sin comillas: las claves y las cadenas van entre comillas dobles; sin comillas solo se admiten números, true, false y null", word)
}

// isSpace indica si c es un espacio en blanco de JSON (RFC 8259): espacio,
// tabulador, salto de línea o retorno de carro. A diferencia de
// token.IsSpace (la definición de HTML), no incluye el salto de página.
func isSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isDigit indica si c es una cifra decimal ASCII. Las cifras de otros
// alfabetos (unicode.IsDigit) no valen en un número JSON.
func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

// errAt devuelve un error en la posición actual del Reader.
func errAt(r *token.Reader, format string, args ...any) error {
	line, col := r.Pos()
	return token.Errorf(line, col, format, args...)
}
