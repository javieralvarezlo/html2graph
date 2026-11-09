package htmltok

import (
	"strings"
	"unicode"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// Funciones de estado. Cada una recibe el carácter actual c sin consumir
// (ok es falso al final de la entrada) y, según el caso, lo consume con
// t.r.Next(), lo guarda en el token en construcción (t.b), cambia de estado
// (t.st) o termina devolviendo next(), emit(token) o fail(error).
//
// En todos los estados salvo Texto, llegar al final de la entrada es un
// error: algo ha quedado sin cerrar.

// text lee el contenido entre etiquetas (estado Texto). Es el estado inicial
// de cada token.
func (t *Tokenizer) text(c rune, ok bool) (token.Token, bool, error) {
	switch {
	// Fin de la entrada: se emite el texto pendiente; si no hay, EOF.
	case !ok && t.b.value.Len() > 0:
		return emit(t.b.token(token.Text))
	case !ok:
		return emit(t.b.token(token.EOF))

	// Un '<' termina el texto. No se consume: la siguiente llamada a Next
	// empezará por él.
	case c == '<' && t.b.value.Len() > 0:
		return emit(t.b.token(token.Text))

	// Sin texto previo, el '<' abre una etiqueta.
	case c == '<':
		t.r.Next()
		t.st = stTagOpen

	// Las entidades se sustituyen por su carácter.
	case c == '&':
		t.b.value.WriteString(readEntity(t.r))

	// Cualquier otro carácter forma parte del texto.
	default:
		t.r.Next()
		t.b.value.WriteRune(c)
	}

	return next()
}

// tagOpen decide qué abre el '<' recién leído (estado AperturaEtiqueta).
func (t *Tokenizer) tagOpen(c rune, ok bool) (token.Token, bool, error) {
	switch {
	// Un '<' al final del fichero no abre nada: es el mismo caso que un '<'
	// suelto en mitad del texto.
	case !ok:
		return fail(t.b.err("'<' al final del fichero (si es texto, debe escribirse &lt;)"))

	// Una letra empieza el nombre de una etiqueta de apertura.
	case unicode.IsLetter(c):
		t.b.kind = token.StartTag
		t.st = stTagName

	// "</" abre una etiqueta de cierre.
	case c == '/':
		t.r.Next()
		t.st = stEndTagOpen

	// "<!--" abre un comentario y "<!DOCTYPE" una declaración; cualquier
	// otra cosa tras "<!" no se admite.
	case t.r.HasPrefix("!--"):
		t.r.Skip(3)
		t.st = stComment
	case t.r.HasPrefixFold("!DOCTYPE"):
		return t.doctypeOpen()
	case c == '!':
		return fail(t.b.err("declaración no reconocida: tras '<!' solo se admiten comentarios <!-- --> y <!DOCTYPE>"))

	// Cualquier otro carácter: el '<' no abre nada. No se trata como texto,
	// porque la entrada no se repara.
	default:
		return fail(errInvalid(t.r, c, "tras '<' (si es texto, debe escribirse &lt;)"))
	}

	return next()
}

// endTagOpen exige que tras "</" venga el nombre de la etiqueta (estado
// AperturaCierre).
func (t *Tokenizer) endTagOpen(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case unicode.IsLetter(c):
		t.b.kind = token.EndTag
		t.st = stTagName
	default:
		return fail(errInvalid(t.r, c, "tras '</'"))
	}

	return next()
}

// tagName lee el nombre de una etiqueta (estado NombreEtiqueta).
func (t *Tokenizer) tagName(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())

	// Los nombres se guardan en minúsculas.
	case isNameChar(c):
		t.r.Next()
		t.b.value.WriteRune(unicode.ToLower(c))

	// Un espacio termina el nombre; pueden venir atributos.
	case token.IsSpace(c):
		t.r.Next()
		t.st = stBeforeAttr

	// '>' termina la etiqueta.
	case c == '>':
		t.r.Next()
		return emit(t.b.token(t.b.kind))

	// '/' empieza el final "/>" de una etiqueta autocerrada.
	case c == '/':
		return t.slash()

	default:
		return fail(errInvalid(t.r, c, "en el nombre de la etiqueta"))
	}

	return next()
}

// beforeAttr salta los espacios entre el nombre de la etiqueta y sus
// atributos, o entre un atributo y otro (estado AntesAtributo).
func (t *Tokenizer) beforeAttr(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case token.IsSpace(c):
		t.r.Next()

	// Empieza un atributo. Se anota su posición por si resulta repetido.
	// Las etiquetas de cierre no pueden tener atributos.
	case unicode.IsLetter(c):
		if t.b.kind == token.EndTag {
			return fail(errAt(t.r, "una etiqueta de cierre no puede tener atributos"))
		}
		t.b.attrLine, t.b.attrCol = t.r.Pos()
		t.st = stAttrName

	case c == '>':
		t.r.Next()
		return emit(t.b.token(t.b.kind))
	case c == '/':
		return t.slash()

	default:
		if t.b.kind == token.EndTag {
			return fail(errInvalid(t.r, c, "en la etiqueta de cierre: se esperaba '>'"))
		}
		return fail(errInvalid(t.r, c, "en la etiqueta: se esperaba un atributo, '>' o '/>'"))
	}

	return next()
}

// slash trata el '/' que empieza el final "/>" de una etiqueta autocerrada.
// Es común a los estados NombreEtiqueta y AntesAtributo.
func (t *Tokenizer) slash() (token.Token, bool, error) {
	if t.b.kind == token.EndTag {
		return fail(errAt(t.r, "una etiqueta de cierre no puede ser autocerrada"))
	}

	t.r.Next()
	t.st = stSelfClosing

	return next()
}

// attrName lee el nombre de un atributo (estado NombreAtributo).
func (t *Tokenizer) attrName(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())

	// Los nombres se guardan en minúsculas.
	case isNameChar(c):
		t.r.Next()
		t.b.attrName.WriteRune(unicode.ToLower(c))

	// '=' introduce el valor.
	case c == '=':
		t.r.Next()
		t.st = stBeforeValue

	// Tras un espacio aún puede venir '=' (por ejemplo, "x = 1").
	case token.IsSpace(c):
		t.r.Next()
		t.st = stAfterAttrName

	// '>' o '/' terminan un atributo sin valor (<input disabled>). No se
	// consumen: los trata AntesAtributo.
	case c == '>' || c == '/':
		return t.closeAttr(stBeforeAttr)

	default:
		return fail(errInvalid(t.r, c, "en el nombre del atributo"))
	}

	return next()
}

// afterAttrName decide, tras el espacio que sigue al nombre de un atributo,
// si este tiene valor (estado DespuesNombreAtributo).
func (t *Tokenizer) afterAttrName(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case token.IsSpace(c):
		t.r.Next()
	case c == '=':
		t.r.Next()
		t.st = stBeforeValue

	// Cualquier otra cosa: el atributo no tenía valor. El carácter se
	// procesa en AntesAtributo.
	default:
		return t.closeAttr(stBeforeAttr)
	}

	return next()
}

// beforeValue decide, tras el '=', qué tipo de valor viene (estado
// AntesValor).
func (t *Tokenizer) beforeValue(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case token.IsSpace(c):
		t.r.Next()

	// Una comilla abre un valor entrecomillado. Se anota su posición para
	// el error de comilla sin cerrar.
	case c == '"' || c == '\'':
		t.b.quote = c
		t.b.quoteLine, t.b.quoteCol = t.r.Pos()
		t.r.Next()
		t.st = stValueQuoted

	// '=' seguido de '>' deja el atributo sin valor.
	case c == '>':
		return fail(errAt(t.r, "falta el valor del atributo '%s' tras '='", t.b.attrName.String()))

	// Cualquier otro carácter empieza un valor sin comillas; lo lee
	// ValorSinComillas.
	default:
		t.st = stValueUnquoted
	}

	return next()
}

// valueQuoted lee un valor entre comillas dobles o simples (estado
// ValorEntreComillas). La comilla que lo cerrará es la que lo abrió,
// guardada en t.b.quote.
func (t *Tokenizer) valueQuoted(c rune, ok bool) (token.Token, bool, error) {
	switch {
	// El error se sitúa en la comilla que abrió el valor.
	case !ok:
		return fail(token.Errorf(t.b.quoteLine, t.b.quoteCol,
			"comilla sin cerrar en el valor del atributo '%s'", t.b.attrName.String()))

	// La comilla de cierre termina el atributo.
	case c == t.b.quote:
		t.r.Next()
		return t.closeAttr(stAfterValue)

	case c == '&':
		t.b.attrValue.WriteString(readEntity(t.r))

	// Dentro de las comillas vale cualquier carácter, incluida la otra
	// comilla y los saltos de línea.
	default:
		t.r.Next()
		t.b.attrValue.WriteRune(c)
	}

	return next()
}

// valueUnquoted lee un valor sin comillas (estado ValorSinComillas).
func (t *Tokenizer) valueUnquoted(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())

	// Un espacio o '>' terminan el valor. El '>' no se consume: lo trata
	// AntesAtributo.
	case token.IsSpace(c):
		t.r.Next()
		return t.closeAttr(stBeforeAttr)
	case c == '>':
		return t.closeAttr(stBeforeAttr)

	case c == '&':
		t.b.attrValue.WriteString(readEntity(t.r))

	// Estos caracteres son ambiguos sin comillas y no se admiten.
	case strings.ContainsRune("\"'<=`", c):
		return fail(errInvalid(t.r, c, "en un valor de atributo sin comillas"))

	default:
		t.r.Next()
		t.b.attrValue.WriteRune(c)
	}

	return next()
}

// afterValue exige un separador tras la comilla de cierre de un valor
// (estado DespuesValor).
func (t *Tokenizer) afterValue(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case token.IsSpace(c):
		t.r.Next()
		t.st = stBeforeAttr

	// '>' y '/' son separadores válidos; no se consumen.
	case c == '>' || c == '/':
		t.st = stBeforeAttr

	// a="1"b="2": falta el espacio entre atributos.
	default:
		return fail(errInvalid(t.r, c, "tras el valor de un atributo: falta un espacio de separación"))
	}

	return next()
}

// closeAttr cierra el atributo en curso y pasa al estado siguiente, o
// termina con error si el atributo está repetido.
func (t *Tokenizer) closeAttr(following state) (token.Token, bool, error) {
	err := t.b.closeAttr()
	if err != nil {
		return fail(err)
	}

	t.st = following

	return next()
}

// selfClosing exige el '>' que sigue a '/' (estado AutoCierre).
func (t *Tokenizer) selfClosing(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.errUnclosedTag())
	case c == '>':
		t.r.Next()
		return emit(t.b.token(token.SelfClosing))
	default:
		return fail(errInvalid(t.r, c, "tras '/': se esperaba '>'"))
	}
}

// comment lee el contenido de un comentario hasta "-->" (estado
// Comentario). Dentro vale cualquier carácter, incluido '<'.
func (t *Tokenizer) comment(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.err("comentario sin cerrar: falta '-->'"))
	case t.r.HasPrefix("-->"):
		t.r.Skip(3)
		return emit(t.b.token(token.Comment))
	default:
		t.r.Next()
		t.b.value.WriteRune(c)
	}

	return next()
}

// doctypeOpen consume "!DOCTYPE" (en mayúsculas o minúsculas) y exige un
// espacio detrás, de modo que <!DOCTYPEhtml> o <!doctypes> no se aceptan.
func (t *Tokenizer) doctypeOpen() (token.Token, bool, error) {
	t.r.Skip(8)

	c, ok := t.r.Peek()
	switch {
	case !ok:
		return fail(t.b.err("declaración <!DOCTYPE> sin cerrar: falta '>'"))
	case !token.IsSpace(c):
		return fail(errInvalid(t.r, c, "tras '<!DOCTYPE': se esperaba un espacio"))
	}

	t.st = stDoctype

	return next()
}

// doctype lee el contenido de una declaración DOCTYPE hasta '>' (estado
// Doctype). El valor del token es ese contenido sin espacios en los
// extremos (normalmente "html"). Un '>' entre comillas, como en
// <!DOCTYPE html SYSTEM "a>b">, no cierra la declaración.
func (t *Tokenizer) doctype(c rune, ok bool) (token.Token, bool, error) {
	switch {
	case !ok:
		return fail(t.b.err("declaración <!DOCTYPE> sin cerrar: falta '>'"))

	// Dentro de comillas todo es contenido; la misma comilla las cierra.
	case t.b.quote != 0:
		if c == t.b.quote {
			t.b.quote = 0
		}

	case c == '"' || c == '\'':
		t.b.quote = c

	case c == '>':
		t.r.Next()
		tok := t.b.token(token.Doctype)
		tok.Value = strings.TrimSpace(tok.Value)
		return emit(tok)
	}

	t.r.Next()
	t.b.value.WriteRune(c)

	return next()
}
