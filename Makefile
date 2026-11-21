# Makefile de html2graph.
#
# Objetivos:
#   make build     compila todos los paquetes y genera bin/munger
#   make test      comprueba el formato (sin tocar nada), go vet y los tests
#   make fmtcheck  falla si algún fichero no está formateado con gofmt
#   make vet       solo go vet
#   make fmt       formatea con gofmt los ficheros que lo necesiten
#   make ejemplos  regenera las salidas de ejemplos/ok y comprueba los
#                  mensajes de ejemplos/error
#   make validar   valida las salidas esperadas con herramientas externas
#                  (Python con networkx, Graphviz, lxml o xmllint; véase tools/LEEME.md)
#   make matriz    prueba todas las combinaciones de la CLI (necesita bash)
#   make dist      binarios para Windows, Linux y macOS  (fase 8)
#
# Las recetas usan solo órdenes de Go y funciones de make, para que funcionen
# igual con GNU make en Linux, macOS y Windows.

GO ?= go
BASH ?= bash

# En Windows el intérprete de Python se llama python (python3 suele ser un
# acceso directo a la Microsoft Store); en Linux y macOS, python3. La
# variable OS vale Windows_NT en Windows, también desde Git Bash.
ifeq ($(OS),Windows_NT)
PYTHON ?= python
else
PYTHON ?= python3
endif

.PHONY: all build test vet fmt fmtcheck ejemplos validar matriz dist

all: test build

# El binario se llama bin/munger en Linux y macOS y bin/munger.exe en
# Windows: GOEXE es la extensión de los ejecutables del sistema.
build:
	$(GO) build ./...
	$(GO) build -o bin/munger$(shell $(GO) env GOEXE) ./cmd/munger

test: fmtcheck vet
	$(GO) test ./...

vet:
	$(GO) vet ./...

# gofmt -w reescribe los ficheros mal formateados y -l muestra cuáles ha
# cambiado; si todos estaban bien formateados, no muestra nada.
fmt:
	gofmt -l -w .

# fmtcheck no modifica nada: gofmt -l lista los ficheros mal formateados y,
# si la lista no está vacía, make se detiene con un error que los nombra.
# Para corregirlos, make fmt.
fmtcheck:
	$(if $(strip $(shell gofmt -l .)),$(error Ficheros sin formatear (ejecute make fmt): $(shell gofmt -l .)),@echo gofmt: OK)

# make ejemplos es una prueba de Go (cmd/munger/ejemplos_test.go) que llama
# a la misma función que la orden munger: con -update escribe con -all los
# siete formatos de cada entrada de ejemplos/ok en ejemplos/ok/salidas/<nombre>/
# y comprueba que cada entrada de ejemplos/error produce el mensaje de su .txt.
# make test hace las mismas comprobaciones sin regenerar nada.
ejemplos:
	$(GO) test ./cmd/munger -run TestEjemplos -update

# validar y matriz usan herramientas externas que no son necesarias para
# compilar ni usar munger; son comprobaciones adicionales (tools/LEEME.md).
validar:
	$(PYTHON) tools/validar.py

# En Windows, bash a secas suele ser el de WSL: hay que indicar el de Git for
# Windows (o lanzar make desde Git Bash), p. ej.
# make matriz BASH="C:/Program Files/Git/bin/bash.exe"
matriz:
	"$(BASH)" tools/matriz.sh $(PYTHON)

dist:
	@echo "dist: pendiente (fase 8)"