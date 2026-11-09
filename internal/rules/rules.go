// Package rules contiene la política de html2graph: qué partes de un
// documento HTML se convierten en nodos del grafo y dónde van los
// atributos.
//
// En el code munger (Herrington) la política se separa del mecanismo. El
// parser construye siempre el árbol completo y el paquete transform lo
// reescribe recorriéndolo (el mecanismo), preguntando en cada nodo a unas
// Rules qué hacer (la política). Este paquete es el único sitio donde se
// toman esas decisiones: contiene solo la interfaz, sus tipos y las reglas
// por defecto, y cambiar la política no obliga a tocar el recorrido.
package rules

import "fmt"

// Action es lo que se hace con un elemento HTML al transformar el árbol.
//
//	Include  el elemento genera un nodo, con sus hijos transformados
//	Ignore   el elemento y todo su subárbol desaparecen
//	Flatten  el elemento desaparece y sus hijos ocupan su lugar en el padre
//
// El valor cero es Include.
type Action int

const (
	Include Action = iota
	Ignore
	Flatten
)

// AttrMode dice dónde van los atributos de los elementos que generan nodo.
//
//	AttrsInNode      en el propio nodo, dentro de su etiqueta (a href="x")
//	AttrsAsChildren  cada uno como un nodo hijo href="x", antes que los demás
//	AttrsNone        en ninguna parte: se descartan
//
// El valor cero es AttrsInNode.
type AttrMode int

const (
	AttrsInNode AttrMode = iota
	AttrsAsChildren
	AttrsNone
)

// actionNames y attrModeNames asocian a cada acción y a cada modo su nombre
// en el código, para los mensajes y los nombres de los tests.
var (
	actionNames = map[Action]string{
		Include: "Include",
		Ignore:  "Ignore",
		Flatten: "Flatten",
	}
	attrModeNames = map[AttrMode]string{
		AttrsInNode:     "AttrsInNode",
		AttrsAsChildren: "AttrsAsChildren",
		AttrsNone:       "AttrsNone",
	}
)

// String devuelve el nombre de la acción (por ejemplo "Flatten"), o
// "Action(n)" si a no es una acción definida.
func (a Action) String() string {
	name, ok := actionNames[a]
	if ok {
		return name
	}

	return fmt.Sprintf("Action(%d)", int(a))
}

// String devuelve el nombre del modo (por ejemplo "AttrsNone"), o
// "AttrMode(n)" si m no es un modo definido.
func (m AttrMode) String() string {
	name, ok := attrModeNames[m]
	if ok {
		return name
	}

	return fmt.Sprintf("AttrMode(%d)", int(m))
}

// Rules es la política de transformación de un árbol HTML. Los nodos JSON
// no la consultan: se copian siempre completos.
type Rules interface {
	// ElementAction dice qué hacer con el elemento de nombre tag (en
	// minúsculas, como lo deja el tokenizer).
	ElementAction(tag string) Action

	// AttrMode dice dónde van los atributos de los elementos incluidos.
	AttrMode() AttrMode

	// IncludeText dice si los textos generan nodo.
	IncludeText() bool
}

// Default son las reglas por defecto, las que usa la línea de órdenes:
// incluyen todos los elementos y textos y dejan los atributos en el nodo,
// como en la figura del enunciado. Con ellas la transformación devuelve una
// copia idéntica del árbol.
type Default struct{}

// ElementAction incluye todos los elementos.
func (Default) ElementAction(string) Action { return Include }

// AttrMode deja los atributos en el nodo de su elemento.
func (Default) AttrMode() AttrMode { return AttrsInNode }

// IncludeText incluye todos los textos.
func (Default) IncludeText() bool { return true }
