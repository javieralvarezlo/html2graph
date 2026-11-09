# Makefile de html2graph.
#
# Objetivos:
#   make build     compila todos los paquetes
#   make test      formatea (gofmt), pasa go vet y ejecuta los tests
#   make vet       solo go vet
#   make fmt       formatea con gofmt los ficheros que lo necesiten
#   make ejemplos  regenera las salidas de ejemplos/ok   (fase 7)
#   make dist      binarios para Windows, Linux y macOS  (fase 8)
#
# Las recetas usan solo órdenes de Go y funciones de make, para que funcionen
# igual con GNU make en Linux, macOS y Windows.

GO ?= go

.PHONY: all build test vet fmt ejemplos dist

all: test build

build:
	$(GO) build ./...

test: fmt vet
	$(GO) test ./...

vet:
	$(GO) vet ./...

# gofmt -w reescribe los ficheros mal formateados y -l muestra cuáles ha
# cambiado; si todos estaban bien formateados, no muestra nada.
fmt:
	gofmt -l -w .

ejemplos:
	@echo "ejemplos: pendiente (fase 7)"

dist:
	@echo "dist: pendiente (fase 8)"