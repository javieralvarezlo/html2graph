package htmltok

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// isNameChar indica si c puede formar parte de un nombre de etiqueta o de
// atributo. El primer carácter, además, tiene que ser una letra.
func isNameChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || strings.ContainsRune("-_:.", c)
}

// entities son las entidades con nombre que se reconocen.
var entities = map[string]string{
	"amp":  "&",
	"lt":   "<",
	"gt":   ">",
	"quot": "\"",
	"apos": "'",
	"nbsp": "\u00a0",
}

// readEntity lee la entidad que empieza en el '&' actual y devuelve su
// carácter. Si no es una entidad válida, consume solo el '&' y lo devuelve,
// de modo que la entidad queda como texto literal.
func readEntity(r *token.Reader) string {
	// El nombre son las letras, dígitos y '#' que siguen al '&', sin
	// consumirlos todavía.
	var name strings.Builder
	n := 1
	for {
		c, ok := r.PeekAt(n)
		if !ok || !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == '#') {
			break
		}
		name.WriteRune(c)
		n++
	}

	// Detrás tiene que haber un ';' y el nombre tiene que ser una entidad.
	semicolon, _ := r.PeekAt(n)
	decoded, ok := decodeEntity(name.String())
	if semicolon != ';' || !ok {
		r.Next()
		return "&"
	}

	r.Skip(n + 1)

	return decoded
}

// decodeEntity devuelve el carácter de la entidad name (sin '&' ni ';'):
// una con nombre ("amp") o una numérica, decimal ("#233") o hexadecimal
// ("#xE9"). ok es falso si no es una entidad válida.
func decodeEntity(name string) (decoded string, ok bool) {
	decoded, ok = entities[name]
	if ok {
		return decoded, true
	}

	if !strings.HasPrefix(name, "#") {
		return "", false
	}

	digits, base := name[1:], 10
	if strings.HasPrefix(digits, "x") || strings.HasPrefix(digits, "X") {
		digits, base = digits[1:], 16
	}
	if digits == "" {
		return "", false
	}

	// Se calcula el código cifra a cifra; solo se admiten cifras de la base
	// (ni signos ni espacios). Los ceros a la izquierda no cuentan, y en
	// cuanto el número se sale de Unicode se deja de acumular.
	code := 0
	for _, c := range digits {
		d := digitValue(c)
		if d < 0 || d >= base {
			return "", false
		}
		if code <= unicode.MaxRune {
			code = code*base + d
		}
	}

	if !allowedChar(rune(code)) {
		return "", false
	}

	return string(rune(code)), true
}

// digitValue devuelve el valor de la cifra hexadecimal c (0-15), o -1 si c
// no es una cifra.
func digitValue(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}

	return -1
}

// allowedChar indica si una entidad numérica puede producir el carácter c.
// Se rechazan los códigos que no son caracteres Unicode, los caracteres de
// control (salvo tabulador, salto de línea y retorno de carro) y los
// no-caracteres (U+FDD0 a U+FDEF y los que acaban en FFFE o FFFF), porque
// no pueden aparecer en un fichero XML y romperían las salidas GraphML y
// GEXF.
func allowedChar(c rune) bool {
	switch {
	case c <= 0 || !utf8.ValidRune(c):
		return false
	case unicode.IsControl(c):
		return c == '\t' || c == '\n' || c == '\r'
	case c >= 0xFDD0 && c <= 0xFDEF:
		return false
	case c&0xFFFE == 0xFFFE:
		return false
	}

	return true
}
