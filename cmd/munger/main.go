// Munger es la línea de órdenes de html2graph: lee un documento HTML o JSON
// bien formado y escribe su árbol como un grafo en el formato pedido.
//
// Es el code munger activo de Herrington completo: une los bloques del
// programa en un pipeline (fichero → tokenizer → parser → transformación →
// aplanado → plantilla → fichero) y no contiene lógica propia más allá de
// leer las opciones, elegir los bloques y comunicar los errores.
//
// Uso:
//
//	munger [-out formato] [-o salida] fichero
//
// Códigos de salida: 0 correcto, 1 error en la entrada, 2 error de uso.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/javieralvarezlo/html2graph/internal/htmltok"
	"github.com/javieralvarezlo/html2graph/internal/jsontok"
	"github.com/javieralvarezlo/html2graph/internal/model"
	"github.com/javieralvarezlo/html2graph/internal/parser"
	"github.com/javieralvarezlo/html2graph/internal/rules"
	"github.com/javieralvarezlo/html2graph/internal/token"
	"github.com/javieralvarezlo/html2graph/internal/transform"
	"github.com/javieralvarezlo/html2graph/internal/writer"
)

// Códigos de salida.
const (
	exitOK    = 0
	exitInput = 1 // error en la entrada (o al escribir la salida)
	exitUsage = 2 // error en la línea de órdenes
)

// help es el texto de munger -h. El %s es la lista de formatos.
const help = `Uso: munger [-out formato] [-o salida] fichero

Lee un documento HTML o JSON bien formado y escribe su árbol como un grafo.
El tipo de entrada se deduce de la extensión del fichero (.html o .htm para
HTML, .json para JSON). Las opciones van antes del fichero.

Opciones:
  -out formato  formato de salida: %s (por defecto, dot)
  -o salida     fichero de salida (por defecto, la salida estándar)
  -h            muestra esta ayuda

Ejemplos:
  munger ejemplos/ok/enunciado.html
  munger -o enunciado.dot ejemplos/ok/enunciado.html
  munger -out gexf -o rico.gexf ejemplos/ok/rico.html
  munger -out graphml -o anidado.graphml ejemplos/ok/anidado.json
  munger ejemplos/ok/enunciado.html | dot -Tpng -o enunciado.png

Códigos de salida: 0 correcto, 1 error en la entrada, 2 error de uso.
`

// options son las opciones de una ejecución, ya comprobadas.
type options struct {
	input     string        // fichero de entrada
	inputType string        // tipo de entrada: "html" o "json"
	output    string        // fichero de salida; vacío para la salida estándar
	writer    writer.Writer // escritor del formato de salida
}

// inputTypes asocia cada extensión de fichero con su tipo de entrada.
var inputTypes = map[string]string{
	".html": "html",
	".htm":  "html",
	".json": "json",
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run ejecuta munger con los argumentos args (sin el nombre del programa),
// escribe el grafo en stdout o en el fichero de -o y los mensajes de error
// en stderr, y devuelve el código de salida.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(stdout, help, strings.Join(writer.Formats(), ", "))
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "munger: %v\n", err)
		fmt.Fprintln(stderr, "Ejecute «munger -h» para ver la ayuda.")
		return exitUsage
	}

	out, err := convert(opts)
	if err == nil {
		err = writeOutput(opts.output, out, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "munger: %v\n", err)
		return exitInput
	}

	return exitOK
}

// parseArgs lee las opciones y el fichero de args y comprueba que la orden
// es correcta. Devuelve flag.ErrHelp si se ha pedido la ayuda, o un error de
// uso con el mensaje para el usuario.
func parseArgs(args []string) (options, error) {
	// El paquete flag no escribe nada (su salida se descarta): la ayuda y
	// los mensajes de error los escribe run, en español.
	flags := flag.NewFlagSet("munger", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("out", "dot", "formato de salida")
	output := flags.String("o", "", "fichero de salida")

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return options{}, err
	}
	if err != nil {
		return options{}, fmt.Errorf("opción incorrecta: %v", err)
	}

	// Tiene que quedar exactamente un argumento: el fichero de entrada.
	switch flags.NArg() {
	case 0:
		return options{}, errors.New("falta el fichero de entrada")
	case 1:
	default:
		return options{}, fmt.Errorf("sobran argumentos: %s", strings.Join(flags.Args()[1:], " "))
	}
	input := flags.Arg(0)

	// El tipo de entrada se deduce de la extensión, sin distinguir
	// mayúsculas. En la fase 7 la opción -in permitirá elegirlo.
	inputType, ok := inputTypes[strings.ToLower(filepath.Ext(input))]
	if !ok {
		return options{}, fmt.Errorf("no se puede deducir el tipo de entrada de «%s»: la extensión debe ser .html, .htm o .json", input)
	}

	// El formato se comprueba antes de leer nada: un formato inexistente es
	// un error de uso aunque el fichero también tenga errores.
	w, err := writer.Lookup(*format)
	if err != nil {
		return options{}, err
	}

	return options{input: input, inputType: inputType, output: *output, writer: w}, nil
}

// convert lee el fichero de entrada y devuelve el grafo ya escrito en el
// formato de salida. Es el pipeline del code munger: tokenizer → parser →
// transformación → aplanado → plantilla. La salida se genera completa en
// memoria para no dejar un fichero a medias si algo falla.
func convert(opts options) ([]byte, error) {
	src, err := readInput(opts.input)
	if err != nil {
		return nil, err
	}

	// Tokenizer y parser según el tipo de entrada.
	var root *model.Node
	switch opts.inputType {
	case "json":
		root, err = parser.ParseJSON(jsontok.New(src))
	default:
		root, err = parser.ParseHTML(htmltok.New(src))
	}
	if err != nil {
		// Los errores léxicos y sintácticos llevan línea y columna; con el
		// nombre del fichero delante quedan como fichero:línea:columna.
		var posErr *token.PosError
		if errors.As(err, &posErr) {
			return nil, fmt.Errorf("%s:%w", opts.input, err)
		}
		return nil, fmt.Errorf("%s: %w", opts.input, err)
	}

	// Transformación del árbol según las reglas. La CLI usa siempre las
	// reglas por defecto, que dejan el árbol igual y nunca fallan.
	root, err = transform.Apply(root, rules.Default{})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", opts.input, err)
	}

	var out bytes.Buffer
	if err := opts.writer.Write(&out, model.Flatten(root)); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// readInput devuelve el contenido del fichero path, o un error que nombra el
// fichero y da la causa.
func readInput(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		// El error de os ya incluye la ruta; se muestra solo la causa.
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return "", fmt.Errorf("no se puede leer «%s»: %v", path, err)
	}

	return string(src), nil
}

// writeOutput escribe out en el fichero path o, si path está vacío, en
// stdout.
func writeOutput(path string, out []byte, stdout io.Writer) error {
	var err error
	if path == "" {
		_, err = stdout.Write(out)
	} else {
		err = os.WriteFile(path, out, 0o644)
	}
	if err != nil {
		return fmt.Errorf("no se puede escribir la salida: %v", err)
	}

	return nil
}
