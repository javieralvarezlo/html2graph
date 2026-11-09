# Makefile de html2graph.
#
# Objetivos:
#   make build     compila todos los paquetes y genera bin/munger
#   make test      comprueba el formato (sin tocar nada), go vet y los tests
#   make fmtcheck  falla si algún fichero no está formateado con gofmt
#   make vet       solo go vet
#   make fmt       formatea con gofmt los ficheros que lo necesiten
#   make ejemplos  regenera las salidas de ejemplos/ok   (fase 7)
#   make dist      binarios para Windows, Linux y macOS  (fase 8)
#
# Las recetas usan solo órdenes de Go y funciones de make, para que funcionen
# igual con GNU make en Linux, macOS y Windows.

GO ?= go

.PHONY: all build test vet fmt fmtcheck ejemplos dist

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

ejemplos:
	@echo "ejemplos: pendiente (fase 7)"

dist:
	@echo "dist: pendiente (fase 8)"