// Package transform reescribe el árbol del modelo según unas reglas
// (rules.Rules) antes de aplanarlo.
//
// En el code munger (Herrington) es el bloque que va entre el parser y el
// aplanado, y es el mecanismo de la separación entre política y mecanismo:
// recorre el árbol y construye uno nuevo, pero no decide nada por su
// cuenta; en cada elemento y en cada texto HTML pregunta a las reglas qué
// hacer. Los nodos JSON se copian siempre completos, sin consultarlas.
package transform

import (
	"errors"
	"fmt"
	"slices"

	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/rules"
)

// ErrMultipleRoots es el error de Apply cuando la raíz se aplana y quedan
// varios nodos sin padre: el resultado no sería un árbol.
var ErrMultipleRoots = errors.New("el árbol transformado tendría varias raíces")

// transformer guarda el estado de una transformación mientras recorre el
// árbol.
type transformer struct {
	rules rules.Rules

	// ID que recibirá el próximo nodo creado.
	nextID int
}

// Apply devuelve un árbol nuevo, resultado de aplicar las reglas r al árbol
// de root. El original no se modifica ni comparte con el nuevo nodos, hijos
// ni atributos. Los IDs del árbol nuevo son consecutivos desde 0 en
// preorden.
//
// La raíz sigue las mismas reglas que cualquier otro nodo. Si desaparece
// (Ignore, o un texto con los textos desactivados), el resultado es el
// árbol vacío, nil, sin error. Si se aplana (Flatten), ocupan su lugar sus
// hijos transformados: si queda uno, es la nueva raíz; si no queda
// ninguno, el árbol vacío; y si quedan varios, el resultado no sería un
// árbol y Apply devuelve ErrMultipleRoots en lugar de inventar una raíz.
func Apply(root *model.Node, r rules.Rules) (*model.Node, error) {
	if root == nil {
		return nil, nil
	}

	t := &transformer{rules: r}
	nodes := t.transform(root)
	switch len(nodes) {
	case 0:
		return nil, nil
	case 1:
		return nodes[0], nil
	}

	return nil, fmt.Errorf("%w: Flatten sobre la raíz <%s> deja %d nodos sin padre", ErrMultipleRoots, root.Label, len(nodes))
}

// transform devuelve los nodos que ocupan el lugar de n en el árbol nuevo:
// ninguno si n desaparece, uno si se conserva y, si se aplana, los que
// ocupan el lugar de sus hijos.
func (t *transformer) transform(n *model.Node) []*model.Node {
	switch n.Kind {
	case model.Element:
		return t.element(n)
	case model.Text:
		if !t.rules.IncludeText() {
			return nil
		}
		return []*model.Node{t.newNode(model.Text, n.Label)}
	}

	// El resto son nodos JSON y, si el árbol ya se había transformado,
	// nodos Attribute: se copian con su subárbol, sin consultar las reglas.
	return []*model.Node{t.clone(n)}
}

// element transforma un elemento HTML según la acción que le asignan las
// reglas. Una acción o un modo de atributos desconocidos son un fallo de
// programación de las reglas y detienen el programa.
func (t *transformer) element(n *model.Node) []*model.Node {
	switch action := t.rules.ElementAction(n.Label); action {
	case rules.Include:
	case rules.Ignore:
		// Los hijos no se visitan: desaparecen con el elemento.
		return nil
	case rules.Flatten:
		// El elemento no genera nodo y sus atributos desaparecen con él; sus
		// hijos, ya transformados, ocupan su lugar en el mismo orden.
		var nodes []*model.Node
		for _, c := range n.Children {
			nodes = append(nodes, t.transform(c)...)
		}
		return nodes
	default:
		panic(fmt.Sprintf("transform: acción %v desconocida para <%s>", action, n.Label))
	}

	// El nodo se crea antes que sus hijos para que su ID quede en preorden.
	e := t.newNode(model.Element, n.Label)
	switch mode := t.rules.AttrMode(); mode {
	case rules.AttrsInNode:
		e.Attrs = slices.Clone(n.Attrs)
	case rules.AttrsAsChildren:
		// Un nodo hijo por atributo, delante de los demás hijos, con la
		// misma forma que tendría dentro de la etiqueta del elemento.
		for _, a := range n.Attrs {
			e.AppendChild(t.newNode(model.Attribute, a.String()))
		}
	case rules.AttrsNone:
	default:
		panic(fmt.Sprintf("transform: modo de atributos %v desconocido", mode))
	}

	for _, c := range n.Children {
		e.Children = append(e.Children, t.transform(c)...)
	}

	return []*model.Node{e}
}

// clone copia n y todo su subárbol, con IDs nuevos en preorden.
func (t *transformer) clone(n *model.Node) *model.Node {
	c := t.newNode(n.Kind, n.Label)
	c.Attrs = slices.Clone(n.Attrs)
	for _, child := range n.Children {
		c.AppendChild(t.clone(child))
	}

	return c
}

// newNode crea un nodo sin hijos con el siguiente ID libre.
func (t *transformer) newNode(kind model.NodeKind, label string) *model.Node {
	n := model.NewNode(kind, label)
	n.ID = t.nextID
	t.nextID++

	return n
}
