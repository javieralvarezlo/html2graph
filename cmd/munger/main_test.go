package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const enunciado = "../../ejemplos/ok/enunciado.html"

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
			code: 0, stdout: "Uso: munger [-out formato] [-o salida] fichero\n", wantOutput: true,
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
				"»: la extensión debe ser .html o .htm\nEjecute «munger -h» para ver la ayuda.\n",
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
