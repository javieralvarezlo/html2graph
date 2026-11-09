package writer

import (
	"fmt"
	"strings"

	"github.com/javieralvarezlo/html2graph/internal/token"
)

// Funciones de escapado, una por formato (XML es común a GraphML y GEXF).
// Cada una recibe la etiqueta de presentación tal cual y devuelve el texto
// que puede escribirse en el sitio que le reserva su plantilla (entre
// comillas, en un atributo XML, en un campo CSV...). Todas tratan los
// saltos de línea, el retorno de carro y el tabulador: desde HTML llegan en
// los valores de atributo (escritos tal cual o como &#10;, &#13; y &#9;),
// porque el parser solo colapsa los espacios de los textos, y llegarán
// también desde las cadenas JSON.
//
// El lector de la entrada ya rechaza los caracteres que no cumplen
// token.IsAllowed. Aun así, los formatos que escriben los caracteres tal
// cual (DOT, XML, GDF y CSV) los sustituyen por U+FFFD con sanitize, para
// que una etiqueta construida a mano nunca produzca un fichero mal formado.
// GML y GraphSON no lo necesitan: escriben esos caracteres con un escape
// (&#N; y \u00XX) que el fichero admite y que se lee de nuevo sin pérdida.

// sanitize sustituye por U+FFFD (el carácter de reemplazo) cada carácter de
// s que no cumple token.IsAllowed, y también cada byte que no forma UTF-8
// válido.
func sanitize(s string) string {
	return strings.Map(func(c rune) rune {
		if !token.IsAllowed(c) {
			return '\uFFFD'
		}

		return c
	}, s)
}

// dotReplacer sustituye los caracteres que no pueden ir tal cual dentro de
// una cadena entre comillas de DOT. "\r\n" va antes que "\r" para que el
// par cuente como un solo salto de línea.
var dotReplacer = strings.NewReplacer(`\`, `\\`, `"`, `\"`, `&`, `&amp;`, "\r\n", `\n`, "\r", `\n`, "\n", `\n`)

// dotChunk es el número máximo de caracteres de cada trozo de una etiqueta
// DOT. Graphviz rechaza las cadenas entre comillas de más de 16384 bytes;
// un carácter escapado ocupa como mucho 5 bytes (& se escribe &amp;), así
// que un trozo de 1600 caracteres ocupa como mucho 8000 bytes ya escapado.
const dotChunk = 1600

// escapeDOT prepara s para escribirlo entre comillas dobles en DOT: la barra
// invertida pasa a ser \\, la comilla \" y el salto de línea \n (que
// Graphviz dibuja como un salto de línea centrado). El & se escribe &amp;
// porque Graphviz interpreta las entidades de HTML en todas las etiquetas:
// sin escaparlo, un texto que diga literalmente &quot; se dibujaría como ".
// DOT no puede representar un retorno de carro (su \r significa «salto de
// línea alineado a la derecha»), así que "\r\n" y "\r" se tratan como un
// salto de línea, igual que hace el lector de la entrada. El resto,
// incluidos los caracteres no ASCII y el U+00A0 de &nbsp;, se escribe tal
// cual en UTF-8.
//
// Una etiqueta de más de dotChunk caracteres se parte en trozos que se
// escapan por separado y se unen con `" + "`: DOT concatena las cadenas
// unidas con +, así que la plantilla, que escribe label="...", produce
// label="trozo 1" + "trozo 2" y Graphviz lee la etiqueta completa. Las
// etiquetas normales quedan en un solo trozo y no cambian.
func escapeDOT(s string) string {
	rest := []rune(sanitize(s))
	var b strings.Builder
	for len(rest) > 0 {
		n := min(dotChunk, len(rest))
		// Un "\r\n" no se separa: partido, daría dos saltos de línea.
		if n < len(rest) && rest[n-1] == '\r' && rest[n] == '\n' {
			n++
		}

		if b.Len() > 0 {
			b.WriteString(`" + "`)
		}
		b.WriteString(dotReplacer.Replace(string(rest[:n])))
		rest = rest[n:]
	}

	return b.String()
}

// gdfReplacer sustituye los caracteres que no pueden ir tal cual dentro de
// un valor GDF entre comillas simples.
var gdfReplacer = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `>`, `\>`, "\n", `\n`, "\r", `\r`)

// escapeGDF prepara s para escribirlo entre comillas simples en GDF. La coma
// no se escapa: entre comillas no separa campos. La comilla simple se
// escribe \' (Gephi no termina el valor en una comilla precedida de barra
// invertida), la barra invertida \\ para que la anterior no sea ambigua, y
// el salto de línea y el retorno de carro \n y \r, porque GDF es un formato
// de una línea por nodo. El > se escribe \> porque Gephi toma como cabecera
// de la sección de aristas cualquier línea que contenga «edgedef>», aunque
// sea dentro de una etiqueta; con la barra, ninguna etiqueta contiene esa
// cadena (ni «nodedef>»).
func escapeGDF(s string) string {
	return gdfReplacer.Replace(sanitize(s))
}

// escapeGML prepara s para escribirlo entre comillas dobles en GML. El
// formato es ASCII de 7 bits y las cadenas no pueden contener comillas, de
// modo que se usan entidades de HTML: & es &amp;, " es &quot; y todo
// carácter fuera del ASCII imprimible (los no ASCII, como é o U+00A0, y los
// de control, como el salto de línea) se escribe con su número de carácter
// en decimal, &#233;, &#160; o &#10;.
//
// No usa un strings.Replacer, como los demás formatos, porque la regla es un
// intervalo de caracteres y no una lista cerrada de sustituciones.
func escapeGML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '"':
			b.WriteString("&quot;")
		case r < ' ' || r > '~':
			fmt.Fprintf(&b, "&#%d;", r)
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// xmlReplacer sustituye los caracteres especiales de XML por sus entidades,
// y los tres caracteres de control admitidos en XML por referencias
// numéricas.
var xmlReplacer = strings.NewReplacer(
	`&`, `&amp;`, `<`, `&lt;`, `>`, `&gt;`, `"`, `&quot;`, `'`, `&apos;`,
	"\t", `&#9;`, "\n", `&#10;`, "\r", `&#13;`,
)

// escapeXML prepara s para escribirlo en GraphML y GEXF, tanto en el
// contenido de un elemento como en el valor de un atributo entre comillas:
// &, <, >, " y ' pasan a ser entidades. El tabulador, el salto de línea y el
// retorno de carro se escriben como referencias numéricas porque un
// analizador XML los normaliza (a espacio dentro de un atributo, y \r a \n
// en el contenido) y la etiqueta cambiaría. El resto se escribe en UTF-8.
//
// Los demás caracteres de control y los no-caracteres no pueden aparecer en
// XML 1.0 ni siquiera como referencia: se sustituyen por U+FFFD (sanitize).
func escapeXML(s string) string {
	return xmlReplacer.Replace(sanitize(s))
}

// jsonReplacer sustituye los caracteres que JSON no admite tal cual dentro
// de una cadena: la comilla, la barra invertida y los 32 caracteres de
// control (U+0000 a U+001F). Se construye con newJSONReplacer.
var jsonReplacer = newJSONReplacer()

// newJSONReplacer construye jsonReplacer: los caracteres con escape corto
// de JSON (\" \\ \n \r \t) y, para el resto de los de control, la forma
// \u00XX en hexadecimal.
func newJSONReplacer() *strings.Replacer {
	pairs := []string{`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`}
	for c := rune(0); c < ' '; c++ {
		if c != '\n' && c != '\r' && c != '\t' {
			pairs = append(pairs, string(c), fmt.Sprintf(`\u%04x`, c))
		}
	}

	return strings.NewReplacer(pairs...)
}

// escapeJSON prepara s para escribirlo entre comillas dobles en JSON
// (GraphSON). Los caracteres no ASCII se escriben tal cual en UTF-8, que es
// la codificación de JSON.
func escapeJSON(s string) string {
	return jsonReplacer.Replace(s)
}

// csvReplacer sustituye los caracteres que no pueden ir tal cual dentro de
// un campo CSV entre comillas dobles.
var csvReplacer = strings.NewReplacer(`"`, `""`, `\`, `\\`, "\n", `\n`, "\r", `\r`)

// escapeCSV prepara s para escribirlo entre comillas dobles en el CSV de
// Gephi. El separador (;) no se escapa: entre comillas no separa campos. La
// comilla se duplica, como en RFC 4180. Gephi, además, lee la barra
// invertida como carácter de escape, así que se escribe \\, y el salto de
// línea y el retorno de carro se escriben \n y \r para conservar una línea
// por nodo.
func escapeCSV(s string) string {
	return csvReplacer.Replace(sanitize(s))
}
