// Package parser contiene los parsers de html2graph: reciben los tokens de
// un tokenizer y construyen con ellos el árbol del modelo (*model.Node).
//
// En el code munger (Herrington) es el segundo bloque: comprueba que los
// tokens forman un documento bien estructurado y entrega el árbol a la
// transformación. Depende solo de la interfaz token.Tokenizer, no de un
// tokenizer concreto. Los errores sintácticos se devuelven como
// *token.PosError y la entrada nunca se repara.
package parser

import (
	"strings"

	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/token"
)

// voidElements son los elementos vacíos de HTML: no tienen contenido ni
// etiqueta de cierre, así que su apertura no se apila.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// openElement es un elemento abierto en la pila, con la posición de su
// etiqueta de apertura para los mensajes de error.
type openElement struct {
	node      *model.Node
	line, col int
}

// htmlParser guarda el estado del parser HTML mientras consume tokens.
type htmlParser struct {
	// Elementos abiertos; la cima es el último.
	stack []openElement

	// Elemento raíz, nil hasta que aparece el primer elemento.
	root *model.Node

	// ID que recibirá el próximo nodo creado.
	nextID int
}

// ParseHTML lee todos los tokens de t y devuelve el árbol del documento
// HTML. Los errores del tokenizer se devuelven tal cual; los sintácticos son
// *token.PosError.
//
// Los comentarios y las declaraciones DOCTYPE no generan nodo, los textos se
// limpian de espacios y los formados solo por espacios se descartan. Los IDs
// de los nodos son consecutivos desde 0 en orden de creación, que coincide
// con el preorden del árbol.
func ParseHTML(t token.Tokenizer) (*model.Node, error) {
	p := &htmlParser{}
	for {
		tok, err := t.Next()
		if err != nil {
			return nil, err
		}

		switch tok.Kind {
		case token.StartTag, token.SelfClosing:
			err = p.element(tok)
		case token.EndTag:
			err = p.endTag(tok)
		case token.Text:
			err = p.text(tok)
		case token.EOF:
			return p.end(tok)

		// Comentarios y DOCTYPE no forman parte del árbol: se ignoran en
		// cualquier posición, también fuera del elemento raíz.
		case token.Comment, token.Doctype:

		// Un tokenizer HTML no produce otras clases de token.
		default:
			err = token.Errorf(tok.Line, tok.Col, "token %s inesperado en un documento HTML", tok.Kind)
		}

		if err != nil {
			return nil, err
		}
	}
}

// newNode crea un nodo con el siguiente ID libre.
func (p *htmlParser) newNode(kind model.NodeKind, label string) *model.Node {
	n := model.NewNode(kind, label)
	n.ID = p.nextID
	p.nextID++

	return n
}

// element trata una etiqueta de apertura o autocerrada: crea el elemento, lo
// cuelga de la cima de la pila (o lo hace raíz) y lo apila si tendrá
// contenido.
func (p *htmlParser) element(tok token.Token) error {
	// Con la pila vacía, el elemento solo puede ser la raíz: si ya la hubo,
	// el documento tiene dos.
	if len(p.stack) == 0 && p.root != nil {
		return token.Errorf(tok.Line, tok.Col,
			"más de un elemento raíz: <%s> aparece después de cerrar <%s>", tok.Value, p.root.Label)
	}

	n := p.newNode(model.Element, tok.Value)
	for _, a := range tok.Attrs {
		n.Attrs = append(n.Attrs, model.Attr{Name: a.Name, Value: a.Value})
	}

	if len(p.stack) == 0 {
		p.root = n
	} else {
		p.stack[len(p.stack)-1].node.AppendChild(n)
	}

	// Una etiqueta autocerrada (<br/>, y también <p/> al estilo XHTML) y un
	// elemento vacío (<br>) no tienen contenido: no se apilan.
	if tok.Kind == token.SelfClosing || voidElements[tok.Value] {
		return nil
	}
	p.stack = append(p.stack, openElement{n, tok.Line, tok.Col})

	return nil
}

// endTag trata una etiqueta de cierre: si cierra el elemento de la cima, lo
// desapila; si no, es un error.
func (p *htmlParser) endTag(tok token.Token) error {
	// Los elementos vacíos nunca se apilan, así que su cierre no puede casar.
	if voidElements[tok.Value] {
		return token.Errorf(tok.Line, tok.Col, "el elemento vacío <%s> no admite etiqueta de cierre", tok.Value)
	}

	// Caso normal: cierra la cima.
	top := len(p.stack) - 1
	if top >= 0 && p.stack[top].node.Label == tok.Value {
		p.stack = p.stack[:top]
		return nil
	}

	// Si el elemento está abierto más abajo, las etiquetas se cruzan
	// (<p><b></p>): se informa de cuál debía cerrarse primero. Si no está
	// abierto en ninguna parte, el cierre no tiene apertura.
	for i := top - 1; i >= 0; i-- {
		if p.stack[i].node.Label == tok.Value {
			want := p.stack[top]
			return token.Errorf(tok.Line, tok.Col, "se esperaba </%s> (abierta en %d:%d) y se encontró </%s>",
				want.node.Label, want.line, want.col, tok.Value)
		}
	}

	return token.Errorf(tok.Line, tok.Col, "cierre </%s> sin apertura", tok.Value)
}

// text trata un texto: lo limpia de espacios y, si queda algo, lo cuelga de
// la cima como nodo hoja.
func (p *htmlParser) text(tok token.Token) error {
	s := cleanText(tok.Value)

	// Los textos formados solo por espacios (sangrías, saltos de línea entre
	// etiquetas) no generan nodo, dentro o fuera de la raíz.
	if s == "" {
		return nil
	}

	// Fuera de la raíz solo se admiten espacios. El error señala el primer
	// carácter que no lo es, no el principio del token.
	if len(p.stack) == 0 {
		line, col := firstNonSpace(tok)
		return token.Errorf(line, col, "texto fuera del elemento raíz")
	}

	p.stack[len(p.stack)-1].node.AppendChild(p.newNode(model.Text, s))

	return nil
}

// end trata el final de la entrada: todos los elementos tienen que estar
// cerrados y tiene que haber habido al menos uno.
func (p *htmlParser) end(tok token.Token) (*model.Node, error) {
	// Si quedan elementos abiertos, se señala el más interno: es el primero
	// que habría que cerrar.
	if len(p.stack) > 0 {
		e := p.stack[len(p.stack)-1]
		return nil, token.Errorf(e.line, e.col, "etiqueta <%s> sin cerrar al llegar al final del fichero", e.node.Label)
	}

	if p.root == nil {
		return nil, token.Errorf(tok.Line, tok.Col, "documento vacío: no hay ningún elemento")
	}

	return p.root, nil
}

// cleanText colapsa cada secuencia de espacios en blanco de s en un solo
// espacio y quita los del principio y el final. Usa token.IsSpace, de modo
// que el U+00A0 de &nbsp; se conserva siempre.
func cleanText(s string) string {
	var b strings.Builder
	pending := false
	for _, c := range s {
		// Un espacio no se escribe todavía: solo se sabe si hace falta
		// cuando llega el siguiente carácter que no lo es. Así los espacios
		// iniciales (sin nada escrito antes) y los finales (sin nada detrás)
		// desaparecen.
		if token.IsSpace(c) {
			pending = b.Len() > 0
			continue
		}

		if pending {
			b.WriteByte(' ')
			pending = false
		}
		b.WriteRune(c)
	}

	return b.String()
}

// firstNonSpace devuelve la posición del primer carácter del texto de tok
// que no es un espacio, contando desde la posición del token.
func firstNonSpace(tok token.Token) (line, col int) {
	line, col = tok.Line, tok.Col
	for _, c := range tok.Value {
		switch {
		case !token.IsSpace(c):
			return line, col
		case c == '\n':
			line, col = line+1, 1
		default:
			col++
		}
	}

	return line, col
}
