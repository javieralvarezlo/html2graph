package token

import (
	"fmt"
	"unicode"
)

// Describe devuelve c tal como se muestra en un mensaje de error. La usan
// los tokenizers de HTML y JSON para nombrar el carácter culpable:
//   - los espacios en blanco, por su nombre ("salto de línea");
//   - la comilla simple, entre comillas dobles, para que se lea bien;
//   - el resto de ASCII visible, entre comillas simples ('$');
//   - cualquier otro carácter, con su código Unicode, para que no se
//     confunda con otro parecido o invisible: un espacio de no separación
//     parece un espacio y un acento combinante no se ve solo.
func Describe(c rune) string {
	names := map[rune]string{
		' ':  "' ' (espacio)",
		'\t': "tabulador",
		'\n': "salto de línea",
		'\r': "retorno de carro",
		'\f': "salto de página",
		'\'': `"'" (comilla simple)`,
	}

	name, ok := names[c]
	switch {
	case ok:
		return name
	case c > ' ' && c < 0x7F:
		return "'" + string(c) + "'"
	case unicode.IsGraphic(c) && !unicode.Is(unicode.Mn, c) && !unicode.IsSpace(c):
		return fmt.Sprintf("'%c' (%U)", c, c)
	}

	return fmt.Sprintf("%U", c)
}

// DescribeDisallowed nombra en un mensaje de error un carácter que no
// cumple IsAllowed: "carácter de control U+000B" o "no-carácter Unicode
// U+FDD0". La usan el lector, para los caracteres escritos tal cual, y el
// tokenizer JSON, para los que producen sus escapes.
func DescribeDisallowed(c rune) string {
	kind := "carácter de control"
	if !unicode.IsControl(c) {
		kind = "no-carácter Unicode"
	}

	return fmt.Sprintf("%s U+%04X", kind, c)
}
