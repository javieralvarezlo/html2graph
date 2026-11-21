package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	enunciado = "../../ejemplos/ok/enunciado.html"
	anidado   = "../../ejemplos/ok/anidado.json"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	cruzadas := filepath.Join(dir, "cruzadas.html")
	if err := os.WriteFile(cruzadas, []byte("<p><b></p></b>"), 0o644); err != nil {
		t.Fatal(err)
	}
	control := filepath.Join(dir, "control.html")
	if err := os.WriteFile(control, []byte("<p>\n t\vv</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	noHTML := filepath.Join(dir, "datos.txt")
	pagina := filepath.Join(dir, "pagina.txt")
	if err := os.WriteFile(pagina, []byte("<p>hola</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	jsonEnHTML := filepath.Join(dir, "datos.html")
	if err := os.WriteFile(jsonEnHTML, []byte("[1]"), 0o644); err != nil {
		t.Fatal(err)
	}
	comaSobrante := filepath.Join(dir, "coma.json")
	if err := os.WriteFile(comaSobrante, []byte("{\"a\": 1,\n}"), 0o644); err != nil {
		t.Fatal(err)
	}
	mayusculas := filepath.Join(dir, "DATOS.JSON")
	if err := os.WriteFile(mayusculas, []byte(`[true]`), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		args       []string
		code       int
		stdout     string // prefijo esperado de la salida estándar
		stderr     string // prefijo del mensaje de error esperado ("" si no hay)
		wantOutput bool   // si la salida estándar debe tener algo
	}{
		{
			name: "enunciado en DOT", args: []string{enunciado},
			code: 0, stdout: "digraph html2graph {\n", wantOutput: true,
		},
		{
			name: "formato explícito", args: []string{"-out", "dot", enunciado},
			code: 0, stdout: "digraph html2graph {\n", wantOutput: true,
		},
		{
			name: "otro formato", args: []string{"-out", "gexf", enunciado},
			code: 0, stdout: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<gexf ", wantOutput: true,
		},
		{
			name: "ayuda", args: []string{"-h"},
			code: 0, stdout: "Uso: munger [-in tipo] [-out formato] [-o salida] [-all dir] fichero\n", wantOutput: true,
		},
		{
			name: "formato inexistente", args: []string{"-out", "xxx", enunciado},
			code: 2,
			stderr: "munger: formato de salida «xxx» inexistente; formatos disponibles: csv, dot, gdf, gexf, gml, graphml, graphson\n" +
				"Ejecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "sin argumentos", args: nil,
			code:   2,
			stderr: "munger: falta el fichero de entrada\nEjecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "argumentos de más", args: []string{enunciado, "otro.html"},
			code:   2,
			stderr: "munger: sobran argumentos: otro.html\nEjecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "opción desconocida", args: []string{"-x", enunciado},
			code:   2,
			stderr: "munger: opción incorrecta: flag provided but not defined: -x\nEjecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "extensión desconocida", args: []string{noHTML},
			code: 2,
			stderr: "munger: no se puede deducir el tipo de entrada de «" + noHTML +
				"»: la extensión debe ser .html, .htm o .json; si no, indique el tipo con -in html o -in json\n" +
				"Ejecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "-in html con extensión desconocida", args: []string{"-in", "html", pagina},
			code: 0, stdout: "digraph html2graph {\n\tnode [shape=box];\n\tn0 [label=\"p\"];\n", wantOutput: true,
		},
		{
			name: "-in json explícito", args: []string{"-in", "json", "-out", "csv", anidado},
			code: 0, stdout: "\"{} (0)\";\"biblioteca (1)\";", wantOutput: true,
		},
		{
			name: "-in html explícito", args: []string{"-in", "html", enunciado},
			code: 0, stdout: "digraph html2graph {\n", wantOutput: true,
		},
		{
			name: "-in manda sobre la extensión", args: []string{"-in", "json", "-out", "csv", jsonEnHTML},
			code: 0, stdout: "\"[] (0)\";\"1 (1)\"\n", wantOutput: true,
		},
		{
			name: "sin -in, la extensión decide", args: []string{jsonEnHTML},
			code:   1,
			stderr: "munger: " + jsonEnHTML + ":1:1: texto fuera del elemento raíz",
		},
		{
			name: "-in desconocido", args: []string{"-in", "xml", enunciado},
			code: 2,
			stderr: "munger: tipo de entrada «xml» inexistente; tipos disponibles: html, json\n" +
				"Ejecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "-in vacío", args: []string{"-in", "", enunciado},
			code:   2,
			stderr: "munger: tipo de entrada «» inexistente; tipos disponibles: html, json\n",
		},
		{
			name: "-in en mayúsculas", args: []string{"-in", "HTML", enunciado},
			code:   2,
			stderr: "munger: tipo de entrada «HTML» inexistente; tipos disponibles: html, json\n",
		},
		{
			name: "-all y -o", args: []string{"-all", dir, "-o", "x.dot", enunciado},
			code: 2,
			stderr: "munger: las opciones -all y -o no pueden usarse juntas: -all escribe cada formato en su propio fichero\n" +
				"Ejecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "-all y -out", args: []string{"-out", "gml", "-all", dir, enunciado},
			code:   2,
			stderr: "munger: las opciones -all y -out no pueden usarse juntas: -all escribe todos los formatos\n",
		},
		{
			name: "-all y -out con el valor por defecto", args: []string{"-out", "dot", "-all", dir, enunciado},
			code:   2,
			stderr: "munger: las opciones -all y -out no pueden usarse juntas: -all escribe todos los formatos\n",
		},
		{
			name: "-all sin directorio", args: []string{"-all", "", enunciado},
			code:   2,
			stderr: "munger: la opción -all necesita un directorio\n",
		},
		{
			name: "-all sin fichero de entrada", args: []string{"-all", dir},
			code:   2,
			stderr: "munger: falta el fichero de entrada\n",
		},
		{
			name: "-o igual a la entrada", args: []string{"-out", "graphson", "-o", anidado, anidado},
			code: 2,
			stderr: "munger: la salida «" + anidado + "» sobrescribiría el fichero de entrada; elija otro fichero o directorio de salida\n" +
				"Ejecute «munger -h» para ver la ayuda.\n",
		},
		{
			name: "-o con otra ruta al mismo fichero", args: []string{"-o", "../munger/../../ejemplos/ok/./enunciado.html", enunciado},
			code:   2,
			stderr: "munger: la salida «../munger/../../ejemplos/ok/./enunciado.html» sobrescribiría el fichero de entrada",
		},
		{
			name: "-all sobrescribiría la entrada JSON", args: []string{"-all", "../../ejemplos/ok", anidado},
			code:   2,
			stderr: "munger: la salida «" + filepath.Join("../../ejemplos/ok", "anidado.json") + "» sobrescribiría el fichero de entrada",
		},
		{
			name: "fichero inexistente", args: []string{"no-existe.html"},
			code:   1,
			stderr: "munger: no se puede leer «no-existe.html»: ", // la causa depende del sistema
		},
		{
			name: "error sintáctico con nombre de fichero", args: []string{cruzadas},
			code:   1,
			stderr: "munger: " + cruzadas + ":1:7: se esperaba </b> (abierta en 1:4) y se encontró </p>\n",
		},
		{
			name: "JSON en DOT", args: []string{anidado},
			code: 0, stdout: "digraph html2graph {\n\tnode [shape=box];\n\tn0 [label=\"{}\"];\n\tn1 [label=\"biblioteca\"];\n", wantOutput: true,
		},
		{
			name: "JSON en otro formato", args: []string{"-out", "gml", anidado},
			code: 0, stdout: "graph [\n", wantOutput: true,
		},
		{
			name: "extensión .JSON en mayúsculas", args: []string{"-out", "csv", mayusculas},
			code: 0, stdout: "\"[] (0)\";\"true (1)\"\n", wantOutput: true,
		},
		{
			name: "error sintáctico en un JSON", args: []string{comaSobrante},
			code:   1,
			stderr: "munger: " + comaSobrante + ":1:8: coma sobrante: tras ',' se esperaba otra clave y se encontró '}'\n",
		},
		{
			name: "carácter de control en la entrada", args: []string{"-out", "graphml", control},
			code: 1,
			stderr: "munger: " + control + ":2:3: carácter de control U+000B no admitido: " +
				"no puede representarse en las salidas XML; elimínelo del fichero\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.code {
				t.Errorf("código de salida %d, se esperaba %d (stderr: %q)", code, tt.code, stderr.String())
			}
			if got := stderr.String(); !strings.HasPrefix(got, tt.stderr) || (got == "") != (tt.stderr == "") {
				t.Errorf("stderr:\nobtenido: %q\nesperado: %q", got, tt.stderr)
			}
			if !strings.HasPrefix(stdout.String(), tt.stdout) || (stdout.Len() > 0) != tt.wantOutput {
				t.Errorf("stdout = %q, se esperaba que empezara por %q", stdout.String(), tt.stdout)
			}
		})
	}
}

// TestRunFicheroSalida comprueba -o: con una entrada correcta escribe el
// mismo grafo que en la salida estándar; con una errónea no crea el fichero.
func TestRunFicheroSalida(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if code := run([]string{enunciado}, &stdout, &stderr); code != 0 {
		t.Fatalf("código de salida %d: %s", code, stderr.String())
	}

	ok := filepath.Join(dir, "enunciado.dot")
	var vacio bytes.Buffer
	if code := run([]string{"-o", ok, enunciado}, &vacio, &stderr); code != 0 {
		t.Fatalf("código de salida %d con -o: %s", code, stderr.String())
	}
	if vacio.Len() != 0 {
		t.Errorf("con -o no debe escribirse nada en la salida estándar: %q", vacio.String())
	}
	got, err := os.ReadFile(ok)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != stdout.String() {
		t.Errorf("el fichero de -o no coincide con la salida estándar:\n%s", got)
	}

	mal := filepath.Join(dir, "mal.html")
	if err := os.WriteFile(mal, []byte("<p>sin cerrar"), 0o644); err != nil {
		t.Fatal(err)
	}
	salida := filepath.Join(dir, "mal.dot")
	if code := run([]string{"-o", salida, mal}, &vacio, &stderr); code != 1 {
		t.Errorf("código de salida %d con entrada errónea, se esperaba 1", code)
	}
	if _, err := os.Stat(salida); !os.IsNotExist(err) {
		t.Errorf("con una entrada errónea no debe crearse %s", salida)
	}

	// Un directorio inexistente en -o es un error al escribir: código 1.
	stderr.Reset()
	if code := run([]string{"-o", filepath.Join(dir, "no", "existe.dot"), enunciado}, &vacio, &stderr); code != 1 {
		t.Errorf("código de salida %d al no poder escribir, se esperaba 1", code)
	}
	if !strings.HasPrefix(stderr.String(), "munger: no se puede escribir la salida: ") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// TestRunAll comprueba -all: con una entrada correcta escribe un fichero por
// formato, con el nombre de la entrada y la extensión del formato, y el mismo
// contenido que -out con ese formato; con un error no escribe nada.
func TestRunAll(t *testing.T) {
	dir := t.TempDir()
	mal := filepath.Join(dir, "mal.json")
	if err := os.WriteFile(mal, []byte(`{"a": 1,}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fichero := filepath.Join(dir, "fichero")
	if err := os.WriteFile(fichero, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Un directorio con el nombre de una de las salidas impide escribirla.
	ocupado := filepath.Join(dir, "ocupado")
	if err := os.MkdirAll(filepath.Join(ocupado, "enunciado.gml"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		input  string
		outDir string
		code   int
		stderr string   // prefijo del mensaje de error ("" si no hay)
		files  []string // ficheros que deben quedar en outDir
		noDir  bool     // si outDir no debe llegar a crearse
	}{
		{
			name: "HTML en un directorio que no existe", input: enunciado,
			outDir: filepath.Join(dir, "nuevo", "enunciado"), code: 0,
			files: []string{"enunciado.csv", "enunciado.dot", "enunciado.gdf", "enunciado.gexf",
				"enunciado.gml", "enunciado.graphml", "enunciado.json"},
		},
		{
			name: "JSON en un directorio que ya existe", input: anidado, outDir: dir, code: 0,
			files: []string{"anidado.csv", "anidado.dot", "anidado.gdf", "anidado.gexf",
				"anidado.gml", "anidado.graphml", "anidado.json"},
		},
		{
			name: "entrada errónea", input: mal, outDir: filepath.Join(dir, "mal"), code: 1,
			stderr: "munger: " + mal + ":1:8: coma sobrante", noDir: true,
		},
		{
			name: "entrada inexistente", input: "no-existe.html", outDir: filepath.Join(dir, "no-existe"), code: 1,
			stderr: "munger: no se puede leer «no-existe.html»: ", noDir: true,
		},
		{
			name: "el directorio es un fichero", input: enunciado, outDir: filepath.Join(fichero, "sub"), code: 1,
			stderr: "munger: no se puede crear el directorio «" + filepath.Join(fichero, "sub") + "»: ",
		},
		{
			name: "una salida no se puede escribir", input: enunciado, outDir: ocupado, code: 1,
			stderr: "munger: no se puede escribir la salida: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-all", tt.outDir, tt.input}, &stdout, &stderr)
			if code != tt.code {
				t.Errorf("código de salida %d, se esperaba %d (stderr: %q)", code, tt.code, stderr.String())
			}
			if got := stderr.String(); !strings.HasPrefix(got, tt.stderr) || (got == "") != (tt.stderr == "") {
				t.Errorf("stderr:\nobtenido: %q\nesperado: %q", got, tt.stderr)
			}
			if stdout.Len() > 0 {
				t.Errorf("-all no debe escribir en la salida estándar: %q", stdout.String())
			}
			if _, err := os.Stat(tt.outDir); tt.noDir && !os.IsNotExist(err) {
				t.Errorf("con una entrada errónea no debe crearse %s", tt.outDir)
			}
			for _, name := range tt.files {
				got, err := os.ReadFile(filepath.Join(tt.outDir, name))
				if err != nil {
					t.Errorf("falta la salida: %v", err)
					continue
				}
				// El contenido es el de -out con el formato de esa extensión.
				format := strings.TrimPrefix(filepath.Ext(name), ".")
				if format == "json" {
					format = "graphson"
				}
				var want bytes.Buffer
				if code := run([]string{"-out", format, tt.input}, &want, &stderr); code != 0 {
					t.Fatalf("-out %s: código %d", format, code)
				}
				if !bytes.Equal(got, want.Bytes()) {
					t.Errorf("%s no coincide con la salida de -out %s", name, format)
				}
			}
		})
	}
}

// TestSameFile comprueba cuándo dos rutas nombran el mismo fichero.
func TestSameFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	if err := os.WriteFile(a, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(dir, "b.json")
	if err := os.WriteFile(b, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Un enlace duro tiene otro nombre pero es el mismo fichero.
	enlace := filepath.Join(dir, "enlace.json")
	if err := os.Link(a, enlace); err != nil {
		t.Skipf("no se puede crear un enlace duro: %v", err)
	}

	tests := []struct {
		name string
		x, y string
		want bool
	}{
		{"misma ruta", a, a, true},
		{"misma ruta escrita de otra forma", a, filepath.Join(dir, ".", "sub", "..", "a.json"), true},
		{"ruta relativa y absoluta", enunciado, mustAbs(t, enunciado), true},
		{"enlace duro", a, enlace, true},
		{"ficheros distintos con el mismo contenido", a, b, false},
		{"uno no existe", a, filepath.Join(dir, "c.json"), false},
		{"ninguno existe", filepath.Join(dir, "x"), filepath.Join(dir, "y"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameFile(tt.x, tt.y); got != tt.want {
				t.Errorf("sameFile(%q, %q) = %v, se esperaba %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

// TestCause comprueba que cause quita la ruta de los errores del sistema de
// ficheros y deja los demás igual.
func TestCause(t *testing.T) {
	otro := errors.New("otro error")
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"error con ruta", &fs.PathError{Op: "open", Path: "x.html", Err: fs.ErrNotExist}, fs.ErrNotExist},
		{"otro error", otro, otro},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cause(tt.err); got != tt.want {
				t.Errorf("cause(%v) = %v, se esperaba %v", tt.err, got, tt.want)
			}
		})
	}
}
