// Package token define el vocabulario común que comparten los tokenizers de
// html2graph.
//
// En la arquitectura del sistema desarrollado, el tokenizer es el primer
// bloque: lee el texto de entrada carácter a carácter y lo convierte en una
// secuencia de unidades léxicas (tokens). En html2graph hay dos tokenizers,
// uno para HTML (paquete htmltok) y otro para JSON (paquete jsontok), y ambos
// producen el mismo tipo Token definido aquí. Gracias a ello el resto del
// sistema no necesita conocer los detalles léxicos de cada lenguaje: el parser
// consume tokens a través de la interfaz Tokenizer y construye con ellos el
// árbol del modelo (model.Node).
//
// # Contenido del paquete
//
//   - Kind: enumeración de las clases de token (HTML, JSON, EOF e Invalid).
//   - Attr: par nombre/valor de un atributo HTML.
//   - Token: unidad léxica con su clase, su valor, sus atributos y la
//     posición (línea y columna) de su primer carácter.
//   - Tokenizer: interfaz con una única operación, Next, que devuelve el
//     siguiente token o un error.
//   - PosError: error con posición, de formato "línea:columna: mensaje",
//     usado por tokenizers y parsers para informar de cualquier fallo.
//   - Reader: lector de caracteres con posición, común a los tokenizers.
//   - IsSpace: definición de espacio en blanco común a tokenizers y parsers.
//
// # Convenciones de posición
//
// Las líneas y las columnas se numeran desde 1. La columna cuenta caracteres
// (puntos de código Unicode), no bytes, de modo que una letra acentuada ocupa
// una sola columna.
package token

import "fmt"

// Kind identifica la clase de un token.
//
// Las clases se agrupan así:
//   - HTML: StartTag, EndTag, SelfClosing, Text, Comment, Doctype.
//   - JSON: LBrace, RBrace, LBracket, RBracket, Colon, Comma, String,
//     Number, True, False, Null.
//   - Ambos: EOF, que marca el final de la entrada.
//   - Invalid: valor 0, el de un Token sin inicializar. Ningún tokenizer lo
//     produce; si aparece, es que alguien usó el token de un error.
type Kind int

const (
	Invalid Kind = iota // token sin inicializar (nunca lo emite un tokenizer)
	EOF                 // fin de la entrada

	// HTML
	StartTag    // etiqueta de apertura: <p>, <a href="...">
	EndTag      // etiqueta de cierre: </p>
	SelfClosing // etiqueta autocerrada: <br/>
	Text        // texto entre etiquetas
	Comment     // comentario: <!-- ... -->
	Doctype     // declaración: <!DOCTYPE html>

	// JSON
	LBrace   // {
	RBrace   // }
	LBracket // [
	RBracket // ]
	Colon    // :
	Comma    // ,
	String   // cadena entre comillas dobles (Value ya sin escapes)
	Number   // número con la gramática de JSON (Value es el literal)
	True     // true
	False    // false
	Null     // null
)

// kindNames asocia a cada Kind su nombre legible.
var kindNames = map[Kind]string{
	Invalid:     "Invalid",
	EOF:         "EOF",
	StartTag:    "StartTag",
	EndTag:      "EndTag",
	SelfClosing: "SelfClosing",
	Text:        "Text",
	Comment:     "Comment",
	Doctype:     "Doctype",
	LBrace:      "LBrace",
	RBrace:      "RBrace",
	LBracket:    "LBracket",
	RBracket:    "RBracket",
	Colon:       "Colon",
	Comma:       "Comma",
	String:      "String",
	Number:      "Number",
	True:        "True",
	False:       "False",
	Null:        "Null",
}

// String devuelve el nombre legible de una clase de token.
func (k Kind) String() string {
	kindName, ok := kindNames[k]
	if ok {
		return kindName
	}

	return fmt.Sprintf("Kind(%d)", int(k))
}

// Attr es un atributo HTML: su nombre, en minúsculas, y su valor, con las
// entidades ya resueltas. Un atributo sin valor (<input disabled>) tiene
// Value vacío.
type Attr struct {
	Name  string
	Value string
}

// Token es una unidad léxica de la entrada.
//
// Campos:
//   - Kind: clase del token.
//   - Value: depende de la clase. Nombre de etiqueta (en minúsculas) para
//     StartTag, EndTag y SelfClosing; texto con entidades resueltas para Text;
//     contenido para Comment y Doctype; contenido sin comillas ni escapes
//     para String; literal tal cual para Number, True, False y Null; vacío
//     para los signos de puntuación y para EOF.
//   - Attrs: atributos de StartTag y SelfClosing, en el orden en que
//     aparecen en la entrada. Vacío en el resto de clases.
//   - Line, Col: posición del primer carácter del token, desde 1.
type Token struct {
	Kind  Kind
	Value string
	Attrs []Attr
	Line  int
	Col   int
}

// String devuelve una representación compacta del token, pensada para
// tests y depuración, con el formato
//
//	línea:columna Clase "valor" [nombre="valor" ...]
//
// donde el valor y los atributos se omiten si están vacíos.
func (t Token) String() string {
	s := fmt.Sprintf("%d:%d %s", t.Line, t.Col, t.Kind)
	if t.Value != "" {
		s += fmt.Sprintf(" %q", t.Value)
	}
	if len(t.Attrs) > 0 {
		s += " ["
		for i, a := range t.Attrs {
			if i > 0 {
				s += " "
			}
			s += fmt.Sprintf("%s=%q", a.Name, a.Value)
		}
		s += "]"
	}
	return s
}

// Tokenizer es la interfaz que implementan los tokenizers de HTML y JSON.
//
// Next devuelve el siguiente token de la entrada, o un error (un *PosError)
// si la entrada es léxicamente incorrecta; en ese caso el token devuelto es
// el vacío, de clase Invalid. Al agotarse la entrada devuelve un token EOF
// con la posición siguiente al último carácter. Una vez devuelto EOF o un
// error, las llamadas posteriores repiten ese mismo resultado.
type Tokenizer interface {
	Next() (Token, error)
}
