package jsontok

import (
	"strings"
	"unicode"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// readNumber lee un número con la gramática de JSON,
//
//	-? (0 | [1-9][0-9]*) (.[0-9]+)? ([eE][+-]?[0-9]+)?
//
// y devuelve el literal tal cual, sin convertirlo, para no perder precisión
// ni cambiar su forma (1.0, 1e3 y 1000 son literales distintos). Los errores
// señalan el carácter donde se esperaba otra cosa.
func (t *Tokenizer) readNumber() (string, error) {
	var b strings.Builder

	// Signo menos opcional; el más no se admite.
	t.accept(&b, "-")

	// Parte entera: un 0 solo o una cifra del 1 al 9 seguida de más cifras.
	c, _ := t.r.Peek()
	switch {
	case c == '0':
		t.accept(&b, "0")
		if c, _ := t.r.Peek(); isDigit(c) {
			return "", errAt(t.r, "número no válido: no se admiten ceros a la izquierda")
		}
	case isDigit(c):
		t.digits(&b)
	default:
		return "", errAt(t.r, "número no válido: se esperaba una cifra tras '-'")
	}

	// Parte decimal opcional: un punto y al menos una cifra.
	if t.accept(&b, ".") && t.digits(&b) == 0 {
		return "", errAt(t.r, "número no válido: se esperaba una cifra tras '.'")
	}

	// Exponente opcional: e o E, signo opcional y al menos una cifra.
	if t.accept(&b, "eE") {
		t.accept(&b, "+-")
		if t.digits(&b) == 0 {
			return "", errAt(t.r, "número no válido: se esperaba una cifra en el exponente")
		}
	}

	// El número termina aquí. Una letra, una cifra de otro alfabeto o un
	// punto pegados a él (0x1F, 1.5.3, 1e5e3) no pueden empezar otro token
	// válido: son parte de un número mal escrito.
	if c, ok := t.r.Peek(); ok && (unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.') {
		return "", errAt(t.r, "número no válido: carácter %s inesperado tras %s", token.Describe(c), b.String())
	}

	return b.String(), nil
}

// accept consume el carácter actual y lo añade a b si es uno de chars, e
// indica si lo ha hecho.
func (t *Tokenizer) accept(b *strings.Builder, chars string) bool {
	c, ok := t.r.Peek()
	if !ok || !strings.ContainsRune(chars, c) {
		return false
	}

	t.r.Next()
	b.WriteRune(c)

	return true
}

// digits consume las cifras decimales que siguen, las añade a b y devuelve
// cuántas había.
func (t *Tokenizer) digits(b *strings.Builder) int {
	n := 0
	for t.accept(b, "0123456789") {
		n++
	}

	return n
}
