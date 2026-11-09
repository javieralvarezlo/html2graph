package token

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reader recorre un texto carácter a carácter llevando la cuenta de la línea
// y la columna del carácter actual. Lo comparten los tokenizers de HTML y
// JSON para que ambos calculen las posiciones de la misma forma.
//
// Las posiciones empiezan en 1 y se cuentan en puntos de código Unicode, no
// en bytes: una letra acentuada ocupa una columna. Un tabulador también
// cuenta como una sola columna, y un carácter formado por varios puntos de
// código (una letra seguida de un acento combinante, un emoji compuesto)
// cuenta como varios, así que la columna puede no coincidir con la que
// muestra un editor.
type Reader struct {
	src  []rune
	pos  int
	line int
	col  int
	err  error
}

// NewReader prepara un Reader situado en el primer carácter de src. Quita la
// marca de orden de bytes (U+FEFF) inicial y convierte "\r\n" y "\r" en
// "\n", para que los ficheros de Windows y de Unix cuenten las líneas igual.
//
// Si src no es UTF-8 válido, o contiene un carácter que no cumple
// IsAllowed, el Reader queda vacío y Err devuelve un *PosError con la
// posición del primer byte o carácter incorrecto: no se intenta adivinar la
// codificación ni sustituir o eliminar lo que sobra.
func NewReader(src string) *Reader {
	src = strings.TrimPrefix(src, "\uFEFF")
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")

	r := &Reader{line: 1, col: 1}
	if !utf8.ValidString(src) {
		r.err = invalidUTF8(src)
		return r
	}
	if err := disallowedChar(src); err != nil {
		r.err = err
		return r
	}

	r.src = []rune(src)

	return r
}

// invalidUTF8 devuelve el error que señala el primer byte de src que no
// forma un carácter UTF-8 válido.
func invalidUTF8(src string) error {
	line, col := 1, 1
	for i, c := range src {
		// Al recorrer un string, Go devuelve utf8.RuneError con tamaño 1
		// para cada byte que no forma un carácter válido.
		if c == utf8.RuneError {
			_, size := utf8.DecodeRuneInString(src[i:])
			if size == 1 {
				return Errorf(line, col,
					"el fichero no está codificado en UTF-8 (byte 0x%02X inválido); conviértalo a UTF-8", src[i])
			}
		}

		if c == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}

	return nil
}

// disallowedChar devuelve el error que señala el primer carácter de src que
// no cumple IsAllowed, o nil si no hay ninguno. src ya es UTF-8 válido.
func disallowedChar(src string) error {
	line, col := 1, 1
	for _, c := range src {
		if !IsAllowed(c) {
			kind := "carácter de control"
			if !unicode.IsControl(c) {
				kind = "no-carácter Unicode"
			}
			return Errorf(line, col,
				"%s U+%04X no admitido: no puede representarse en las salidas XML; elimínelo del fichero", kind, c)
		}

		if c == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}

	return nil
}

// Err devuelve el error de codificación o de carácter no admitido detectado
// al crear el Reader, o nil.
func (r *Reader) Err() error {
	return r.err
}

// Peek devuelve el carácter actual sin avanzar. ok es falso si se ha llegado
// al final del texto.
func (r *Reader) Peek() (c rune, ok bool) {
	return r.PeekAt(0)
}

// PeekAt devuelve, sin avanzar, el carácter que está n posiciones por
// delante del actual. ok es falso si esa posición queda fuera del texto.
func (r *Reader) PeekAt(n int) (c rune, ok bool) {
	i := r.pos + n
	if i < 0 || i >= len(r.src) {
		return 0, false
	}

	return r.src[i], true
}

// Next devuelve el carácter actual y avanza al siguiente, actualizando la
// línea y la columna. ok es falso, sin moverse, al final del texto.
func (r *Reader) Next() (c rune, ok bool) {
	c, ok = r.Peek()
	if !ok {
		return 0, false
	}

	r.pos++
	if c == '\n' {
		r.line++
		r.col = 1
	} else {
		r.col++
	}

	return c, true
}

// Skip avanza n caracteres (o hasta el final del texto, si quedan menos).
func (r *Reader) Skip(n int) {
	for i := 0; i < n; i++ {
		if _, ok := r.Next(); !ok {
			return
		}
	}
}

// HasPrefix indica si el texto que queda por leer empieza por s, sin avanzar.
func (r *Reader) HasPrefix(s string) bool {
	return r.hasPrefix(s, false)
}

// HasPrefixFold es como HasPrefix pero sin distinguir mayúsculas de
// minúsculas.
func (r *Reader) HasPrefixFold(s string) bool {
	return r.hasPrefix(s, true)
}

// hasPrefix compara carácter a carácter el texto que queda por leer con s,
// ignorando mayúsculas si fold es verdadero.
func (r *Reader) hasPrefix(s string, fold bool) bool {
	i := r.pos
	for _, c := range s {
		if i >= len(r.src) {
			return false
		}

		a := r.src[i]
		if fold {
			a, c = unicode.ToLower(a), unicode.ToLower(c)
		}
		if a != c {
			return false
		}

		i++
	}

	return true
}

// Pos devuelve la línea y la columna del carácter actual. Al final del texto
// es la posición siguiente al último carácter.
func (r *Reader) Pos() (line, col int) {
	return r.line, r.col
}

// IsSpace indica si c es un espacio en blanco: espacio, tabulador, salto de
// línea, retorno de carro o salto de página. Es la única definición de
// espacio del proyecto: la usan el tokenizer HTML para separar atributos y
// el parser para limpiar los textos.
//
// El salto de página (U+000C) se incluye por coherencia con la definición de
// espacio de HTML, pero nunca llega a usarse: XML 1.0 no lo admite, así que
// IsAllowed lo rechaza y el lector no deja pasar ningún fichero que lo
// contenga.
//
// A propósito no incluye U+00A0 (el espacio de no separación de &nbsp;) ni
// otros espacios Unicode, de modo que un &nbsp; nunca se colapsa ni se
// recorta al limpiar un texto.
func IsSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// IsAllowed indica si c puede aparecer en un documento de entrada. Es la
// única definición del proyecto de los caracteres admitidos: todos ellos
// deben poder escribirse en las salidas XML (GraphML y GEXF), así que se
// siguen las reglas de XML 1.0 y, además, se excluyen los caracteres que
// XML desaconseja (los de control C1 y los no-caracteres). Se rechazan los códigos
// que no son caracteres Unicode (0, las mitades de par sustituto y los
// mayores que U+10FFFF), los caracteres de control salvo el tabulador, el
// salto de línea y el retorno de carro (también DEL y los de control C1,
// U+007F a U+009F) y los no-caracteres (U+FDD0 a U+FDEF y los que acaban en
// FFFE o FFFF).
//
// La usan el lector, para rechazar los caracteres escritos tal cual en el
// fichero, los tokenizers, para los que producen sus entidades o escapes, y
// los escritores, como defensa ante etiquetas construidas a mano.
func IsAllowed(c rune) bool {
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
