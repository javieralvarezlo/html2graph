// Package htmltok implementa el tokenizer de HTML: convierte el texto de un
// documento HTML en una secuencia de tokens (etiquetas de apertura, de
// cierre y autocerradas, textos, comentarios y declaraciones DOCTYPE).
//
// En la arquitectura del code munger (Herrington), es el primer bloque del
// camino HTML: lee la entrada con un token.Reader y entrega token.Token al
// parser a través de la interfaz token.Tokenizer. Solo se ocupa de la forma
// de cada token; que las etiquetas casen entre sí lo comprueba el parser.
//
// Es una máquina de estados explícita. Cada estado es una función (fichero
// states.go) que mira el carácter actual y decide qué hacer con él y a qué
// estado pasar. Los errores léxicos se devuelven como *token.PosError, y la
// entrada nunca se repara: un documento mal formado produce un error.
//
// La máquina es una versión reducida del tokenizador de HTML5 (WHATWG, HTML
// Living Standard, sección 13.2.5), sin sus reglas de recuperación de
// errores. Limitación: <script> y <style> no tienen tratamiento especial, así
// que su contenido no puede contener '<'.
package htmltok

import (
	"github.com/javieralvarezlo/html2graph/internal/token"
)

// state es un estado de la máquina. Junto a cada constante, su nombre en la
// memoria.
type state int

const (
	stText          state = iota // Texto
	stTagOpen                    // AperturaEtiqueta
	stEndTagOpen                 // AperturaCierre
	stTagName                    // NombreEtiqueta
	stBeforeAttr                 // AntesAtributo
	stAttrName                   // NombreAtributo
	stAfterAttrName              // DespuesNombreAtributo
	stBeforeValue                // AntesValor
	stValueQuoted                // ValorEntreComillas
	stValueUnquoted              // ValorSinComillas
	stAfterValue                 // DespuesValor
	stSelfClosing                // AutoCierre
	stComment                    // Comentario
	stDoctype                    // Doctype
)

// stateFunc procesa el carácter actual c en un estado. ok es false si se ha
// llegado al final de la entrada. Devuelve done true cuando el token
// está terminado (tok) o cuando hay un error (err); si no, la máquina sigue
// con el carácter siguiente en el estado que haya fijado la función.
type stateFunc func(t *Tokenizer, c rune, ok bool) (tok token.Token, done bool, err error)

// states asocia cada estado con la función que lo implementa.
var states = map[state]stateFunc{
	stText:          (*Tokenizer).text,
	stTagOpen:       (*Tokenizer).tagOpen,
	stEndTagOpen:    (*Tokenizer).endTagOpen,
	stTagName:       (*Tokenizer).tagName,
	stBeforeAttr:    (*Tokenizer).beforeAttr,
	stAttrName:      (*Tokenizer).attrName,
	stAfterAttrName: (*Tokenizer).afterAttrName,
	stBeforeValue:   (*Tokenizer).beforeValue,
	stValueQuoted:   (*Tokenizer).valueQuoted,
	stValueUnquoted: (*Tokenizer).valueUnquoted,
	stAfterValue:    (*Tokenizer).afterValue,
	stSelfClosing:   (*Tokenizer).selfClosing,
	stComment:       (*Tokenizer).comment,
	stDoctype:       (*Tokenizer).doctype,
}

// Tokenizer es el tokenizer de HTML. Implementa token.Tokenizer.
type Tokenizer struct {
	r *token.Reader

	// Estado de la máquina y token en construcción durante una llamada a
	// Next.
	st state
	b  *builder

	// done indica que ya se devolvió EOF o un error; en ese caso Next
	// repite lastTok y lastErr.
	done    bool
	lastTok token.Token
	lastErr error
}

// Comprobación en compilación de que Tokenizer cumple la interfaz común.
var _ token.Tokenizer = (*Tokenizer)(nil)

// New crea un tokenizer que leerá el documento HTML src.
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

// scan ejecuta la máquina de estados hasta emitir un token o encontrar un
// error. Cada token empieza en el estado Texto, en la posición actual, y se
// llama a la función del estado en curso con cada carácter hasta que una de
// ellas termina el token o da un error.
func (t *Tokenizer) scan() (token.Token, error) {
	t.b = &builder{}
	t.b.line, t.b.col = t.r.Pos()
	t.st = stText

	for {
		c, ok := t.r.Peek()

		tok, done, err := states[t.st](t, c, ok)
		if done {
			return tok, err
		}
	}
}

// Resultados de una función de estado: seguir leyendo, emitir el token o
// terminar con un error.

func next() (token.Token, bool, error) {
	return token.Token{}, false, nil
}

func emit(tok token.Token) (token.Token, bool, error) {
	return tok, true, nil
}

func fail(err error) (token.Token, bool, error) {
	return token.Token{}, true, err
}
