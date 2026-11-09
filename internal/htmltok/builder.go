package htmltok

import (
	"strings"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// builder acumula las partes del token que se está leyendo mientras la
// máquina de estados avanza.
type builder struct {
	// Posición del primer carácter del token.
	line, col int

	// StartTag o EndTag, según cómo empezó la etiqueta.
	kind token.Kind

	// Texto, nombre de etiqueta, comentario o declaración.
	value strings.Builder

	// Atributos ya cerrados, en orden de aparición.
	attrs []token.Attr

	// Atributo que se está leyendo y posición de su nombre.
	attrName, attrValue strings.Builder
	attrLine, attrCol   int

	// Comilla que abrió el valor en curso y su posición.
	quote               rune
	quoteLine, quoteCol int
}

// token construye el token de clase kind con lo acumulado.
func (b *builder) token(kind token.Kind) token.Token {
	return token.Token{
		Kind:  kind,
		Value: b.value.String(),
		Attrs: b.attrs,
		Line:  b.line,
		Col:   b.col,
	}
}

// closeAttr da por terminado el atributo en curso y lo añade a la lista.
// Devuelve un error si la etiqueta ya tenía un atributo con ese nombre.
func (b *builder) closeAttr() error {
	name := b.attrName.String()

	// Un atributo repetido es un error, en la posición del segundo.
	for _, a := range b.attrs {
		if a.Name == name {
			return token.Errorf(b.attrLine, b.attrCol, "atributo '%s' repetido", name)
		}
	}

	b.attrs = append(b.attrs, token.Attr{Name: name, Value: b.attrValue.String()})

	// Se vacían nombre y valor para el atributo siguiente.
	b.attrName.Reset()
	b.attrValue.Reset()

	return nil
}

// err devuelve un error en la posición de inicio del token (el '<').
func (b *builder) err(msg string) error {
	return token.Errorf(b.line, b.col, "%s", msg)
}

// errUnclosedTag devuelve el error de etiqueta que llega al final de la
// entrada sin su '>', en la posición del '<' que la abrió.
func (b *builder) errUnclosedTag() error {
	return b.err("etiqueta sin cerrar: falta '>' antes del final del fichero")
}

// errAt devuelve un error en la posición actual del Reader.
func errAt(r *token.Reader, format string, args ...any) error {
	line, col := r.Pos()
	return token.Errorf(line, col, format, args...)
}

// errInvalid devuelve el error de carácter c inválido en la posición actual;
// where explica dónde apareció.
func errInvalid(r *token.Reader, c rune, where string) error {
	return errAt(r, "carácter %s inválido %s", token.Describe(c), where)
}
