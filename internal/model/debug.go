package model

import "strings"

// Dump devuelve el árbol de root como texto, un nodo por línea, con dos
// espacios de sangría por nivel de profundidad y un salto de línea al final.
// Sirve para los tests y para depurar: no es un formato de salida.
//
// Cada nodo se escribe según su clase:
//
//	Element     nombre y, si tiene, sus atributos:  a href="http://x"
//	Text        el texto entre comillas dobles:      "Hoja de enlaces"
//	el resto    su etiqueta tal cual:                {}  []  nombre  "Ana"
//
// Dentro de las comillas (textos y valores de atributo) se escriben \", \\
// y \n en lugar de la comilla, la barra invertida y el salto de línea, y
// fuera de ellas los saltos de línea también se escriben \n, de modo que
// cada nodo ocupa siempre una sola línea. Un árbol vacío (nil) da "".
//
// Por ejemplo, <p>Hola <b>mundo</b></p> da:
//
//	p
//	  "Hola"
//	  b
//	    "mundo"
func Dump(root *Node) string {
	if root == nil {
		return ""
	}

	var b strings.Builder
	dump(&b, root, 0)

	return b.String()
}

// quoted escapa un texto que va entre comillas; unquoted, una etiqueta que
// va sin ellas.
var (
	quoted   = strings.NewReplacer(`"`, `\"`, `\`, `\\`, "\n", `\n`)
	unquoted = strings.NewReplacer("\n", `\n`)
)

// dump escribe en b el nodo n con la sangría de su profundidad y, debajo,
// sus hijos en orden, un nivel más adentro.
func dump(b *strings.Builder, n *Node, depth int) {
	b.WriteString(strings.Repeat("  ", depth))

	switch n.Kind {
	case Element:
		b.WriteString(n.Label)
		for _, a := range n.Attrs {
			b.WriteString(" ")
			b.WriteString(a.Name)
			b.WriteString(`="`)
			b.WriteString(quoted.Replace(a.Value))
			b.WriteString(`"`)
		}
	case Text:
		b.WriteString(`"`)
		b.WriteString(quoted.Replace(n.Label))
		b.WriteString(`"`)
	default:
		b.WriteString(unquoted.Replace(n.Label))
	}
	b.WriteByte('\n')

	for _, c := range n.Children {
		dump(b, c, depth+1)
	}
}
