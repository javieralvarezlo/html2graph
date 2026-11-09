// Package writer contiene los escritores de html2graph: convierten el grafo
// aplanado (model.Graph) en el texto de un formato de salida.
//
// En el code munger (Herrington) es el último bloque, el de las plantillas.
// Cada formato es una plantilla text/template (templates/<formato>.tmpl),
// embebida en el ejecutable, más las funciones de escapado que esa
// plantilla necesita (escape.go). Añadir un formato solo exige su plantilla
// y una entrada en el registro; el resto del programa no cambia.
package writer

import (
	"embed"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/template"

	"github.com/javieralvarezlo/html2graph/internal/model"
)

// Writer es un formato de salida: la extensión de los ficheros de ese
// formato (con el punto, como ".dot") y la plantilla que lo genera.
type Writer struct {
	Ext  string
	tmpl *template.Template
}

//go:embed templates/*.tmpl
var templates embed.FS

// funcs son las funciones que pueden usar las plantillas: una de escapado
// por formato, salvo GraphML y GEXF, que comparten las reglas de XML.
var funcs = template.FuncMap{
	"dot":  escapeDOT,
	"gdf":  escapeGDF,
	"gml":  escapeGML,
	"xml":  escapeXML,
	"json": escapeJSON,
	"csv":  escapeCSV,
}

// writers es el registro de formatos, por nombre. GraphSON usa la
// extensión .json, la de los ficheros GraphSON de TinkerPop.
var writers = map[string]Writer{
	"dot":      {Ext: ".dot", tmpl: parse("dot.tmpl")},
	"gdf":      {Ext: ".gdf", tmpl: parse("gdf.tmpl")},
	"gml":      {Ext: ".gml", tmpl: parse("gml.tmpl")},
	"graphml":  {Ext: ".graphml", tmpl: parse("graphml.tmpl")},
	"graphson": {Ext: ".json", tmpl: parse("graphson.tmpl")},
	"gexf":     {Ext: ".gexf", tmpl: parse("gexf.tmpl")},
	"csv":      {Ext: ".csv", tmpl: parse("csv.tmpl")},
}

// parse carga una plantilla embebida. Las plantillas forman parte del
// programa, así que un error en una de ellas es un fallo de programación y
// detiene el programa al arrancar.
func parse(name string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).ParseFS(templates, "templates/"+name))
}

// Lookup devuelve el escritor del formato de nombre format, o un error que
// enumera los formatos disponibles si no existe.
func Lookup(format string) (Writer, error) {
	w, ok := writers[format]
	if !ok {
		return Writer{}, fmt.Errorf("formato de salida «%s» inexistente; formatos disponibles: %s",
			format, strings.Join(Formats(), ", "))
	}

	return w, nil
}

// Formats devuelve los nombres de los formatos disponibles, en orden
// alfabético.
func Formats() []string {
	names := make([]string, 0, len(writers))
	for name := range writers {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

// Write escribe g en out con la plantilla del formato.
func (w Writer) Write(out io.Writer, g model.Graph) error {
	return w.tmpl.Execute(out, g)
}
