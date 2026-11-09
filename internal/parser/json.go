package parser

// El parser JSON construye el árbol de un documento JSON con esta
// correspondencia: un objeto es un nodo JSONObject con etiqueta {}; cada una
// de sus claves es un hijo JSONKey con el nombre de la clave (sin comillas),
// cuyo único hijo es el valor; un array es un nodo JSONArray con etiqueta []
// cuyos hijos son sus elementos; y un valor simple es una hoja JSONValue. La
// etiqueta de una cadena es su contenido entre comillas dobles, para que la
// cadena "30" no se confunda con el número 30 ni "null" con null; la de un
// número, true, false o null es el literal. Por ejemplo,
//
//	{"nombre": "Ana", "edad": 30, "tags": ["x", null]}
//
// da el árbol
//
//	{}
//	├── nombre
//	│   └── "Ana"
//	├── edad
//	│   └── 30
//	└── tags
//	    └── []
//	        ├── "x"
//	        └── null

import (
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/token"
)

// expect es lo que el parser JSON espera recibir a continuación.
type expect int

const (
	expectValue      expect = iota // el valor raíz o el valor tras ':'
	expectFirstElem                // tras '[': un elemento o ']'
	expectElem                     // tras ',' en un array: un elemento
	expectFirstKey                 // tras '{': una clave o '}'
	expectKey                      // tras ',' en un objeto: una clave
	expectColon                    // tras una clave: ':'
	expectCommaOrEnd               // tras un valor en un objeto o array: ',' o el cierre
	expectEOF                      // tras el valor raíz: el final del fichero
)

// tokenNames nombran cada clase de token JSON en los mensajes de error.
var tokenNames = map[token.Kind]string{
	token.LBrace:   "'{'",
	token.RBrace:   "'}'",
	token.LBracket: "'['",
	token.RBracket: "']'",
	token.Colon:    "':'",
	token.Comma:    "','",
	token.String:   "una cadena",
	token.Number:   "el número",
	token.True:     "true",
	token.False:    "false",
	token.Null:     "null",
}

// openContainer es un objeto o array abierto en la pila, con la posición de
// su llave o corchete de apertura para los mensajes de error.
type openContainer struct {
	node      *model.Node
	line, col int

	// Solo en los objetos: la clave que espera su valor y las claves ya
	// vistas, con su token, para detectar las repetidas.
	key  *model.Node
	keys map[string]token.Token
}

// jsonParser guarda el estado del parser JSON mientras consume tokens.
type jsonParser struct {
	// Objetos y arrays abiertos; la cima es el último.
	stack []*openContainer

	// Valor raíz, nil hasta que aparece.
	root *model.Node

	// Qué se espera a continuación y posición de la última coma, para el
	// error de coma sobrante.
	want                expect
	commaLine, commaCol int

	// ID que recibirá el próximo nodo creado.
	nextID int
}

// ParseJSON lee todos los tokens de t y devuelve el árbol del documento
// JSON. Los errores del tokenizer se devuelven tal cual; los sintácticos son
// *token.PosError.
//
// Cualquier valor JSON es un documento válido, también un valor simple (42
// da un árbol de un solo nodo). Las claves conservan el orden de la entrada,
// y una clave repetida en el mismo objeto es un error. Los IDs de los nodos
// son consecutivos desde 0 en orden de creación, que coincide con el
// preorden del árbol.
func ParseJSON(t token.Tokenizer) (*model.Node, error) {
	p := &jsonParser{}
	for {
		tok, err := t.Next()
		if err != nil {
			return nil, err
		}

		// Un tokenizer JSON no produce otras clases de token.
		_, isJSON := tokenNames[tok.Kind]
		switch {
		case tok.Kind == token.EOF:
			return p.end(tok)
		case !isJSON:
			err = token.Errorf(tok.Line, tok.Col, "token %s inesperado en un documento JSON", tok.Kind)
		case p.want == expectValue, p.want == expectFirstElem, p.want == expectElem:
			err = p.value(tok)
		case p.want == expectFirstKey, p.want == expectKey:
			err = p.key(tok)
		case p.want == expectColon:
			err = p.colon(tok)
		case p.want == expectCommaOrEnd:
			err = p.commaOrEnd(tok)
		default:
			err = p.errorf(tok, "contenido tras el valor raíz: se esperaba el final del fichero")
		}

		if err != nil {
			return nil, err
		}
	}
}

// newNode crea un nodo con el siguiente ID libre.
func (p *jsonParser) newNode(kind model.NodeKind, label string) *model.Node {
	n := model.NewNode(kind, label)
	n.ID = p.nextID
	p.nextID++

	return n
}

// top devuelve el objeto o array de la cima de la pila.
func (p *jsonParser) top() *openContainer {
	return p.stack[len(p.stack)-1]
}

// value trata el token que empieza un valor: un objeto o array se abre y se
// apila; un valor simple es una hoja. En los dos casos el nodo se cuelga de
// donde corresponde (véase attach).
func (p *jsonParser) value(tok token.Token) error {
	switch tok.Kind {
	case token.LBrace:
		n := p.attach(p.newNode(model.JSONObject, "{}"))
		p.stack = append(p.stack, &openContainer{node: n, line: tok.Line, col: tok.Col, keys: map[string]token.Token{}})
		p.want = expectFirstKey
		return nil
	case token.LBracket:
		n := p.attach(p.newNode(model.JSONArray, "[]"))
		p.stack = append(p.stack, &openContainer{node: n, line: tok.Line, col: tok.Col})
		p.want = expectFirstElem
		return nil
	case token.String:
		p.attach(p.newNode(model.JSONValue, `"`+tok.Value+`"`))
		p.afterValue()
		return nil
	case token.Number, token.True, token.False, token.Null:
		p.attach(p.newNode(model.JSONValue, tok.Value))
		p.afterValue()
		return nil
	}

	// No es un valor. Si es el ']' de un array vacío, lo cierra; si no, el
	// mensaje depende de dónde faltaba el valor.
	switch {
	case p.want == expectFirstElem && tok.Kind == token.RBracket:
		return p.close(tok)
	case p.want == expectFirstElem:
		return p.errorf(tok, "se esperaba un valor o ']'")
	case p.want == expectElem && tok.Kind == token.RBracket:
		return token.Errorf(p.commaLine, p.commaCol, "coma sobrante: tras ',' se esperaba otro elemento y se encontró ']'")
	case p.want == expectElem:
		return p.errorf(tok, "se esperaba un valor tras ','")
	case len(p.stack) > 0:
		return p.errorf(tok, "falta el valor de la clave %q: se esperaba un valor", p.top().key.Label)
	}

	return p.errorf(tok, "se esperaba un valor")
}

// attach cuelga el nodo de un valor de donde corresponde y lo devuelve: de
// la cima si es un array, de la clave pendiente si es un objeto, o como raíz
// si la pila está vacía.
func (p *jsonParser) attach(n *model.Node) *model.Node {
	if len(p.stack) == 0 {
		p.root = n
		return n
	}

	top := p.top()
	if top.node.Kind == model.JSONArray {
		return top.node.AppendChild(n)
	}

	return top.key.AppendChild(n)
}

// afterValue fija lo que se espera tras un valor completo: el final del
// fichero si era la raíz, o una coma o el cierre si está dentro de un objeto
// o array.
func (p *jsonParser) afterValue() {
	if len(p.stack) == 0 {
		p.want = expectEOF
	} else {
		p.want = expectCommaOrEnd
	}
}

// key trata el token que debería ser una clave del objeto de la cima: crea
// el nodo de la clave, que esperará su valor.
func (p *jsonParser) key(tok token.Token) error {
	top := p.top()
	switch {
	case tok.Kind == token.String:
		// Una clave repetida haría ambiguo el documento: se señala la
		// segunda, como un atributo HTML repetido.
		first, repeated := top.keys[tok.Value]
		if repeated {
			return token.Errorf(tok.Line, tok.Col, "clave %q repetida (ya aparece en %d:%d)", tok.Value, first.Line, first.Col)
		}
		top.keys[tok.Value] = tok
		top.key = top.node.AppendChild(p.newNode(model.JSONKey, tok.Value))
		p.want = expectColon
		return nil

	case p.want == expectFirstKey && tok.Kind == token.RBrace:
		return p.close(tok)
	case p.want == expectFirstKey:
		return p.errorf(tok, "se esperaba una clave entre comillas dobles o '}'")
	case tok.Kind == token.RBrace:
		return token.Errorf(p.commaLine, p.commaCol, "coma sobrante: tras ',' se esperaba otra clave y se encontró '}'")
	}

	return p.errorf(tok, "se esperaba una clave entre comillas dobles tras ','")
}

// colon exige los dos puntos que separan una clave de su valor.
func (p *jsonParser) colon(tok token.Token) error {
	if tok.Kind != token.Colon {
		return p.errorf(tok, "se esperaba ':' tras la clave %q", p.top().key.Label)
	}

	p.want = expectValue

	return nil
}

// commaOrEnd trata lo que sigue a un valor dentro de un objeto o array: una
// coma, que da paso al siguiente elemento o clave, o el cierre.
func (p *jsonParser) commaOrEnd(tok token.Token) error {
	isArray := p.top().node.Kind == model.JSONArray
	closer := "'}'"
	if isArray {
		closer = "']'"
	}

	switch tok.Kind {
	case token.Comma:
		p.commaLine, p.commaCol = tok.Line, tok.Col
		p.want = expectKey
		if isArray {
			p.want = expectElem
		}
		return nil
	case token.RBrace, token.RBracket:
		return p.close(tok)

	// Si llega otro valor, lo más probable es que falte la coma.
	case token.Colon:
		return p.errorf(tok, "se esperaba ',' o %s", closer)
	}

	return p.errorf(tok, "falta una coma: se esperaba ',' o %s", closer)
}

// close trata una llave o corchete de cierre: si cierra el objeto o array de
// la cima, lo desapila; si es del otro tipo, es un error.
func (p *jsonParser) close(tok token.Token) error {
	top := p.top()
	opener, closer := "{", token.RBrace
	if top.node.Kind == model.JSONArray {
		opener, closer = "[", token.RBracket
	}
	if tok.Kind != closer {
		return p.errorf(tok, "se esperaba %s para cerrar el '%s' abierto en %d:%d", tokenNames[closer], opener, top.line, top.col)
	}

	p.stack = p.stack[:len(p.stack)-1]
	p.afterValue()

	return nil
}

// end trata el final de la entrada: no puede quedar nada abierto y tiene que
// haber habido un valor.
func (p *jsonParser) end(tok token.Token) (*model.Node, error) {
	// Si quedan objetos o arrays abiertos, se señala el más interno: es el
	// primero que habría que cerrar.
	if len(p.stack) > 0 {
		top := p.top()
		opener, closer := "{", "}"
		if top.node.Kind == model.JSONArray {
			opener, closer = "[", "]"
		}
		return nil, token.Errorf(top.line, top.col, "'%s' sin cerrar: falta '%s' antes del final del fichero", opener, closer)
	}

	if p.root == nil {
		return nil, token.Errorf(tok.Line, tok.Col, "documento vacío: no hay ningún valor JSON")
	}

	return p.root, nil
}

// errorf devuelve un error en la posición de tok con el mensaje formateado,
// seguido de «y se encontró» y el token encontrado.
func (p *jsonParser) errorf(tok token.Token, format string, args ...any) error {
	found := tokenNames[tok.Kind]
	if tok.Kind == token.Number {
		found += " " + tok.Value
	}

	return token.Errorf(tok.Line, tok.Col, format+" y se encontró %s", append(args, found)...)
}
