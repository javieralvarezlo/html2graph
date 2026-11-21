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
//	munger [-in tipo] [-out formato] [-o salida] [-all dir] fichero
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
	"maps"
	"os"
	"path/filepath"
	"slices"
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

// helpHint es la línea que sigue a todo mensaje de error de uso.
const helpHint = "Ejecute «munger -h» para ver la ayuda."

// help es el texto de munger -h. Los %s son la lista de tipos de entrada y
// la de formatos de salida.
const help = `Uso: munger [-in tipo] [-out formato] [-o salida] [-all dir] fichero

Lee un documento HTML o JSON bien formado y escribe su árbol como un grafo.
Las opciones van antes del fichero.

Opciones:
  -in tipo      tipo de entrada: %s. Si no se indica, se deduce de la
                extensión del fichero (.html o .htm para HTML, .json para
                JSON); si se indica, manda sobre la extensión
  -out formato  formato de salida (por defecto, dot):
                %s
  -o salida     fichero de salida (por defecto, la salida estándar)
  -all dir      escribe todos los formatos en el directorio dir, que se crea
                si no existe: un fichero por formato con el nombre del de
                entrada y la extensión del formato (GraphSON, .json); no se
                combina con -out ni con -o
  -h            muestra esta ayuda

Ejemplos:
  munger ejemplos/ok/enunciado.html
  munger -o enunciado.dot ejemplos/ok/enunciado.html
  munger -out gexf -o rico.gexf ejemplos/ok/rico.html
  munger -out graphml -o anidado.graphml ejemplos/ok/anidado.json
  munger -in html -out gml -o pagina.gml pagina.txt
  munger -all salidas ejemplos/ok/anidado.json
  munger ejemplos/ok/enunciado.html | dot -Tpng -o enunciado.png

Códigos de salida: 0 correcto, 1 error en la entrada, 2 error de uso.
`

// parseFunc analiza el texto de un documento y devuelve su árbol: reúne el
// tokenizer y el parser de un tipo de entrada.
type parseFunc func(src string) (*model.Node, error)

// parsers es el registro de tipos de entrada, por nombre. Añadir un tipo
// exige su tokenizer, su parser y una línea aquí (y en inputTypes, si tiene
// una extensión propia).
var parsers = map[string]parseFunc{
	"html": func(src string) (*model.Node, error) { return parser.ParseHTML(htmltok.New(src)) },
	"json": func(src string) (*model.Node, error) { return parser.ParseJSON(jsontok.New(src)) },
}

// inputTypes asocia cada extensión de fichero con su tipo de entrada.
var inputTypes = map[string]string{
	".html": "html",
	".htm":  "html",
	".json": "json",
}

// target es un destino de la salida: un fichero y el formato que se escribe
// en él.
type target struct {
	path   string        // fichero de salida; vacío para la salida estándar
	writer writer.Writer // escritor del formato
}

// options son las opciones de una ejecución, ya comprobadas.
type options struct {
	input   string    // fichero de entrada
	parse   parseFunc // tokenizer y parser del tipo de entrada
	allDir  string    // directorio de -all; vacío si no se ha usado
	targets []target  // una salida por formato: una sola salvo con -all
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run ejecuta munger con los argumentos args (sin el nombre del programa),
// escribe el grafo en stdout o en los ficheros de salida y los mensajes de
// error en stderr, y devuelve el código de salida.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(stdout, help, strings.Join(inputTypeNames(), ", "), strings.Join(writer.Formats(), ", "))
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "munger: %v\n", err)
		fmt.Fprintln(stderr, helpHint)
		return exitUsage
	}

	// Primero se generan todas las salidas y solo después se escribe la
	// primera: una entrada errónea no crea ningún fichero ni el directorio.
	outs, err := convert(opts)
	if err == nil {
		err = writeOutputs(opts, outs, stdout)
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
	in := flags.String("in", "", "tipo de entrada")
	format := flags.String("out", "dot", "formato de salida")
	output := flags.String("o", "", "fichero de salida")
	allDir := flags.String("all", "", "directorio para todos los formatos")

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return options{}, err
	}
	if err != nil {
		return options{}, fmt.Errorf("opción incorrecta: %v", err)
	}

	// Las opciones que aparecen en la orden, aunque tengan el valor por
	// defecto: -all no se combina con -out ni con -o, ni siquiera con
	// «-out dot».
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["all"] {
		switch {
		case *allDir == "":
			return options{}, errors.New("la opción -all necesita un directorio")
		case set["o"]:
			return options{}, errors.New("las opciones -all y -o no pueden usarse juntas: -all escribe cada formato en su propio fichero")
		case set["out"]:
			return options{}, errors.New("las opciones -all y -out no pueden usarse juntas: -all escribe todos los formatos")
		}
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

	inputType := *in
	if !set["in"] {
		inputType, err = inputTypeOf(input)
		if err != nil {
			return options{}, err
		}
	}
	parse, ok := parsers[inputType]
	if !ok {
		return options{}, fmt.Errorf("tipo de entrada «%s» inexistente; tipos disponibles: %s",
			inputType, strings.Join(inputTypeNames(), ", "))
	}

	// Los formatos y los ficheros de salida se comprueban antes de leer
	// nada: un formato inexistente es un error de uso aunque el fichero
	// también tenga errores.
	targets, err := outputTargets(input, *format, *output, *allDir)
	if err != nil {
		return options{}, err
	}

	return options{input: input, parse: parse, allDir: *allDir, targets: targets}, nil
}

// inputTypeOf deduce el tipo de entrada de la extensión del fichero, sin
// distinguir mayúsculas.
func inputTypeOf(input string) (string, error) {
	inputType, ok := inputTypes[strings.ToLower(filepath.Ext(input))]
	if !ok {
		return "", fmt.Errorf("no se puede deducir el tipo de entrada de «%s»: la extensión debe ser .html, .htm o .json; "+
			"si no, indique el tipo con -in html o -in json", input)
	}

	return inputType, nil
}

// inputTypeNames devuelve los nombres de los tipos de entrada, en orden
// alfabético.
func inputTypeNames() []string {
	return slices.Sorted(maps.Keys(parsers))
}

// outputTargets devuelve los destinos de la salida: el formato format en el
// fichero output (vacío para la salida estándar) o, si allDir no está
// vacío, todos los formatos en allDir, cada uno en un fichero con el nombre
// de input y la extensión del formato. Es un error de uso un formato
// inexistente o una salida que sobrescribiría el fichero de entrada.
func outputTargets(input, format, output, allDir string) ([]target, error) {
	formats := []string{format}
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	if allDir != "" {
		formats = writer.Formats()
	}

	var targets []target
	for _, name := range formats {
		w, err := writer.Lookup(name)
		if err != nil {
			return nil, err
		}
		path := output
		if allDir != "" {
			path = filepath.Join(allDir, base+w.Ext)
		}
		// Con una entrada .json, la salida GraphSON de -all se llama igual
		// que la entrada; si el directorio es el de la entrada, la
		// sustituiría por el grafo.
		if path != "" && sameFile(path, input) {
			return nil, fmt.Errorf("la salida «%s» sobrescribiría el fichero de entrada; elija otro fichero o directorio de salida", path)
		}
		targets = append(targets, target{path: path, writer: w})
	}

	return targets, nil
}

// sameFile dice si las rutas a y b nombran el mismo fichero: si coinciden
// una vez completas y limpias, o si existen las dos y el sistema operativo
// dice que son el mismo (enlaces, o mayúsculas distintas en los sistemas de
// ficheros que no las distinguen, como los de Windows y macOS).
func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil && absA == absB {
		return true
	}
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)

	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

// convert lee el fichero de entrada y devuelve el grafo ya escrito en el
// formato de cada destino, en el mismo orden. Es el pipeline del code
// munger: tokenizer → parser → transformación → aplanado → plantilla. La
// entrada se analiza una sola vez aunque haya varios destinos, y todas las
// salidas se generan completas en memoria para no dejar ficheros a medias
// si algo falla.
func convert(opts options) ([][]byte, error) {
	src, err := readInput(opts.input)
	if err != nil {
		return nil, err
	}

	root, err := opts.parse(src)
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

	g := model.Flatten(root)
	outs := make([][]byte, len(opts.targets))
	for i, t := range opts.targets {
		var out bytes.Buffer
		if err := t.writer.Write(&out, g); err != nil {
			return nil, err
		}
		outs[i] = out.Bytes()
	}

	return outs, nil
}

// readInput devuelve el contenido del fichero path, o un error que nombra el
// fichero y da la causa.
func readInput(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("no se puede leer «%s»: %v", path, cause(err))
	}

	return string(src), nil
}

// writeOutputs escribe cada salida outs[i] en el destino opts.targets[i].
// Con -all crea antes el directorio, si no existe.
func writeOutputs(opts options, outs [][]byte, stdout io.Writer) error {
	if opts.allDir != "" {
		if err := os.MkdirAll(opts.allDir, 0o755); err != nil {
			return fmt.Errorf("no se puede crear el directorio «%s»: %v", opts.allDir, cause(err))
		}
	}
	for i, t := range opts.targets {
		if err := writeOutput(t.path, outs[i], stdout); err != nil {
			return err
		}
	}

	return nil
}

// writeOutput escribe out en el fichero path o, si path está vacío, en
// stdout. Si falla, el mensaje nombra el fichero, como los de lectura.
func writeOutput(path string, out []byte, stdout io.Writer) error {
	if path == "" {
		if _, err := stdout.Write(out); err != nil {
			return fmt.Errorf("no se puede escribir la salida: %v", err)
		}
		return nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("no se puede escribir «%s»: %v", path, cause(err))
	}

	return nil
}

// cause devuelve la causa de un error del sistema de ficheros sin la ruta,
// que los mensajes de munger ya muestran a su manera.
func cause(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}

	return err
}
