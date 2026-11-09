package token

import "fmt"

// PosError es un error asociado a una posición de la entrada. Lo usan los
// tokenizers (errores léxicos) y los parsers (errores sintácticos), de modo
// que todos los mensajes de error del programa tienen el mismo formato y
// siempre indican dónde está el problema.
//
// Campos:
//   - Line, Col: posición del error, desde 1.
//   - Msg: descripción del problema, en español y sin la posición.
type PosError struct {
	Line int
	Col  int
	Msg  string
}

// Error devuelve el mensaje completo: "línea:columna: mensaje".
func (e *PosError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

// Errorf construye un PosError en la posición line:col, con el mensaje
// formateado como en fmt.Sprintf.
func Errorf(line, col int, format string, args ...any) *PosError {
	return &PosError{Line: line, Col: col, Msg: fmt.Sprintf(format, args...)}
}
