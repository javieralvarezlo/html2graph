package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update regenera las salidas de ejemplos/ok en lugar de compararlas. Es lo
// que hace make ejemplos:
//
//	go test ./cmd/munger -run TestEjemplos -update
var update = flag.Bool("update", false, "regenera las salidas de ejemplos/ok/salidas")

// TestEjemplosOK genera con munger -all los siete formatos de cada entrada
// de ejemplos/ok y los compara con los de ejemplos/ok/salidas/<nombre>/; con
// -update, los escribe allí.
func TestEjemplosOK(t *testing.T) {
	// Desde la raíz del proyecto, como lo ejecutaría el usuario.
	t.Chdir("../..")

	entries, err := os.ReadDir("ejemplos/ok")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		t.Run(name, func(t *testing.T) {
			want := "ejemplos/ok/salidas/" + name
			dir := filepath.Join(t.TempDir(), name)
			if *update {
				// Se borra el directorio para que no queden salidas de
				// formatos que ya no existan.
				if err := os.RemoveAll(want); err != nil {
					t.Fatal(err)
				}
				dir = want
			}

			var stdout, stderr bytes.Buffer
			if code := run([]string{"-all", dir, "ejemplos/ok/" + e.Name()}, &stdout, &stderr); code != exitOK {
				t.Fatalf("código de salida %d: %s", code, stderr.String())
			}
			if stdout.Len() > 0 || stderr.Len() > 0 {
				t.Errorf("-all no debe escribir nada en pantalla: %q %q", stdout.String(), stderr.String())
			}
			if !*update {
				compareDirs(t, dir, want)
			}
		})
	}
}

// compareDirs comprueba que los directorios got y want contienen los mismos
// ficheros con el mismo contenido.
func compareDirs(t *testing.T, got, want string) {
	t.Helper()
	gotFiles := readDir(t, got)
	wantFiles := readDir(t, want)
	for name, content := range gotFiles {
		w, ok := wantFiles[name]
		switch {
		case !ok:
			t.Errorf("%s/%s no existe (ejecute make ejemplos)", want, name)
		case !bytes.Equal(content, w):
			t.Errorf("%s/%s no coincide con la salida de munger (ejecute make ejemplos)", want, name)
		}
	}
	for name := range wantFiles {
		if _, ok := gotFiles[name]; !ok {
			t.Errorf("%s/%s sobra: munger no lo genera", want, name)
		}
	}
}

// readDir devuelve el contenido de cada fichero del directorio dir, por
// nombre.
func readDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%v (ejecute make ejemplos)", err)
	}
	files := map[string][]byte{}
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = content
	}

	return files
}

// TestEjemplosError ejecuta munger con cada entrada de ejemplos/error y
// comprueba que escribe exactamente el mensaje de su .txt, nada en la
// salida estándar, y el código de salida que corresponde al mensaje: 2 si
// termina remitiendo a la ayuda (error de uso), 1 si no. Las opciones de un
// caso, si las hay, están en <nombre>.args. Nunca regenera los .txt.
func TestEjemplosError(t *testing.T) {
	t.Chdir("../..")

	const dir = "ejemplos/error/"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Ficheros de cada caso, por nombre sin extensión: el .txt, el .args
	// si lo hay y la entrada, que es cualquier otro.
	cases := map[string][]string{}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		cases[name] = append(cases[name], e.Name())
	}
	if len(cases) == 0 {
		t.Fatal("no hay casos en ejemplos/error")
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			var inputs []string
			for _, f := range files {
				if ext := filepath.Ext(f); ext != ".txt" && ext != ".args" {
					inputs = append(inputs, f)
				}
			}
			if len(inputs) != 1 {
				t.Fatalf("el caso debe tener exactamente un fichero de entrada y tiene %v", inputs)
			}

			var args []string
			if b, err := os.ReadFile(dir + name + ".args"); err == nil {
				args = strings.Fields(string(b))
			}
			// La ruta se escribe con / en todos los sistemas, para que el
			// mensaje sea el mismo en Windows.
			args = append(args, dir+inputs[0])

			txt := dir + name + ".txt"
			want, err := os.ReadFile(txt)
			if err != nil {
				t.Fatalf("%v: falta el mensaje esperado", err)
			}
			wantCode := exitInput
			if strings.HasSuffix(string(want), helpHint+"\n") {
				wantCode = exitUsage
			}

			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			if stderr.String() != string(want) {
				t.Errorf("munger %s\nmensaje obtenido:\n%s\nmensaje esperado (%s):\n%s",
					strings.Join(args, " "), stderr.String(), txt, want)
			}
			if code != wantCode {
				t.Errorf("código de salida %d, se esperaba %d", code, wantCode)
			}
			if stdout.Len() > 0 {
				t.Errorf("con un error no debe escribirse nada en la salida estándar: %q", stdout.String())
			}
		})
	}
}
