#!/usr/bin/env bash
# Comprueba que «todas las combinaciones funcionan» por la línea de órdenes.
#
# Uso (desde la raíz del proyecto, o con make matriz):
#   bash tools/matriz.sh [python]
#
# Compila munger y genera {enunciado, rico (HTML), anidado (JSON)} × 7
# formatos × {salida estándar, -o, -all}, con el tipo de entrada deducido de
# la extensión, con -in explícito y con -in sobre una copia con extensión
# .txt. Comprueba que todas las variantes son idénticas byte a byte a las
# salidas esperadas (internal/writer/testdata/<caso>/esperado.<formato>) y a
# ejemplos/ok/salidas, y valida cada variante con tools/validar.py.
#
# Requisitos: bash (en Windows, el de Git for Windows), Go, y lo que pide
# validar.py. El primer argumento es el intérprete de Python (por defecto,
# python3; en Windows suele ser python). La variable GO permite elegir otro go.
set -u
P=$(cd "$(dirname "$0")/.." && pwd)
GO=${GO:-go}
PYTHON=${1:-python3}
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT

# En Windows el binario tiene que llamarse munger.exe para poder ejecutarlo.
M="$T/munger$("$GO" env GOEXE)"
(cd "$P" && "$GO" build -o "$M" ./cmd/munger) || exit 1
fallos=0
fail() { echo "FALLO: $*"; fallos=$((fallos+1)); }
formats="csv dot gdf gexf gml graphml graphson"
ext() { [ "$1" = graphson ] && echo json || echo "$1"; }

for caso in enunciado:html rico:html anidado:json; do
  c=${caso%%:*}; tipo=${caso##*:}
  in="$P/ejemplos/ok/$c.$tipo"
  cp "$in" "$T/$c.txt"
  for variante in deducido explicito txt; do
    case $variante in
      deducido) pre=(); src="$in";;
      explicito) pre=(-in "$tipo"); src="$in";;
      txt) pre=(-in "$tipo"); src="$T/$c.txt";;
    esac
    for modo in stdout o all; do
      d="$T/$variante-$modo/$c"; mkdir -p "$d"
      if [ $modo = all ]; then
        "$M" "${pre[@]}" -all "$T/all-raw/$variante/$c" "$src" 2>"$T/err" || fail "$variante $modo $c: $(cat "$T/err")"
      fi
      for f in $formats; do
        e=$(ext $f)
        case $modo in
          stdout) "$M" "${pre[@]}" -out $f "$src" >"$d/esperado.$f" 2>"$T/err" || fail "$variante $modo $c $f";;
          o) "$M" "${pre[@]}" -out $f -o "$d/esperado.$f" "$src" 2>"$T/err" || fail "$variante $modo $c $f";;
          all) cp "$T/all-raw/$variante/$c/$c.$e" "$d/esperado.$f" || fail "falta $c.$e";;
        esac
        [ -s "$T/err" ] && fail "$variante $modo $c $f: salida de error no vacía"
        cmp -s "$d/esperado.$f" "$P/internal/writer/testdata/$c/esperado.$f" || fail "$variante $modo $c $f distinto del esperado"
        cmp -s "$d/esperado.$f" "$P/ejemplos/ok/salidas/$c/$c.$e" || fail "$variante $modo $c $f distinto de ejemplos/ok/salidas"
      done
      n=$(ls "$T/all-raw/$variante/$c" 2>/dev/null | wc -l)
      [ $modo = all ] && [ "$n" != 7 ] && fail "-all generó $n ficheros"
    done
  done
done
echo "comparaciones byte a byte: $fallos fallos"

for dir in "$T"/*-stdout "$T"/*-o "$T"/*-all; do
  echo "== validar $(basename "$dir")"
  "$PYTHON" "$P/tools/validar.py" "$dir" | tail -1
  "$PYTHON" "$P/tools/validar.py" "$dir" >/dev/null || fail "validar.py $(basename "$dir")"
done

echo "TOTAL: $fallos fallos"
echo "(los avisos no son fallos: son las salidas peculiares; make validar los lista, véase tools/LEEME.md)"
[ $fallos = 0 ]
