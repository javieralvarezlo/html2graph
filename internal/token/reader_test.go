package token

import (
	"fmt"
	"testing"
)

// leerTodo consume el Reader y devuelve cada carácter con su posición, en el
// formato "línea:columna:carácter".
func leerTodo(r *Reader) []string {
	var out []string
	for {
		line, col := r.Pos()
		c, ok := r.Next()
		if !ok {
			return append(out, fmt.Sprintf("%d:%d:EOF", line, col))
		}
		out = append(out, fmt.Sprintf("%d:%d:%q", line, col, c))
	}
}

func TestReaderPosiciones(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"vacío", "", []string{"1:1:EOF"}},
		{"una línea", "ab", []string{"1:1:'a'", "1:2:'b'", "1:3:EOF"}},
		{"salto unix", "a\nb", []string{"1:1:'a'", "1:2:'\\n'", "2:1:'b'", "2:2:EOF"}},
		{"salto windows", "a\r\nb", []string{"1:1:'a'", "1:2:'\\n'", "2:1:'b'", "2:2:EOF"}},
		{"retorno suelto", "a\rb", []string{"1:1:'a'", "1:2:'\\n'", "2:1:'b'", "2:2:EOF"}},
		{"acentos ocupan una columna", "éñ", []string{"1:1:'é'", "1:2:'ñ'", "1:3:EOF"}},
		{"tabulador ocupa una columna", "\tx", []string{"1:1:'\\t'", "1:2:'x'", "1:3:EOF"}},
		{"marca BOM ignorada", "\uFEFFa", []string{"1:1:'a'", "1:2:EOF"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leerTodo(NewReader(tt.src))
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("\nobtenido: %v\nesperado: %v", got, tt.want)
			}
		})
	}
}

func TestReaderPeek(t *testing.T) {
	r := NewReader("ab")
	if c, ok := r.Peek(); !ok || c != 'a' {
		t.Errorf("Peek() = %q, %v", c, ok)
	}
	if c, ok := r.PeekAt(1); !ok || c != 'b' {
		t.Errorf("PeekAt(1) = %q, %v", c, ok)
	}
	if _, ok := r.PeekAt(2); ok {
		t.Error("PeekAt(2) debería estar fuera del texto")
	}
	if line, col := r.Pos(); line != 1 || col != 1 {
		t.Errorf("Peek no debe avanzar: posición %d:%d", line, col)
	}

	r.Skip(5)
	if _, ok := r.Peek(); ok {
		t.Error("tras Skip(5) debería estar al final")
	}
	if _, ok := r.Next(); ok {
		t.Error("Next al final debería devolver ok falso")
	}
	if line, col := r.Pos(); line != 1 || col != 3 {
		t.Errorf("posición final = %d:%d, se esperaba 1:3", line, col)
	}
}

func TestReaderHasPrefix(t *testing.T) {
	r := NewReader("<!DOCTYPE html>")
	r.Next()
	tests := []struct {
		prefix   string
		fold     bool
		expected bool
	}{
		{"!DOCTYPE", false, true},
		{"!doctype", false, false},
		{"!doctype", true, true},
		{"!--", false, false},
		{"!DOCTYPE html> y más", false, false},
		{"", false, true},
	}
	for _, tt := range tests {
		got := r.HasPrefix(tt.prefix)
		if tt.fold {
			got = r.HasPrefixFold(tt.prefix)
		}
		if got != tt.expected {
			t.Errorf("HasPrefix(%q, fold=%v) = %v, se esperaba %v", tt.prefix, tt.fold, got, tt.expected)
		}
	}
	if line, col := r.Pos(); line != 1 || col != 2 {
		t.Errorf("HasPrefix no debe avanzar: posición %d:%d", line, col)
	}
}

func TestReaderUTF8Invalido(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"Latin-1 al principio", "\xe9", "1:1: el fichero no está codificado en UTF-8 (byte 0xE9 inválido); conviértalo a UTF-8"},
		{"tras acentos válidos", "éñ\xe9", "1:3: el fichero no está codificado en UTF-8 (byte 0xE9 inválido); conviértalo a UTF-8"},
		{"en otra línea", "a\r\nbc\xff", "2:3: el fichero no está codificado en UTF-8 (byte 0xFF inválido); conviértalo a UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader(tt.src)
			if r.Err() == nil || r.Err().Error() != tt.want {
				t.Fatalf("Err() = %v, se esperaba %q", r.Err(), tt.want)
			}
			if _, ok := r.Peek(); ok {
				t.Error("un Reader con error debe estar vacío")
			}
		})
	}

	// El carácter de sustitución U+FFFD escrito de verdad en el fichero sí
	// es UTF-8 válido.
	if err := NewReader("a\uFFFDb").Err(); err != nil {
		t.Errorf("U+FFFD válido rechazado: %v", err)
	}
}

func TestReaderCaracterNoAdmitido(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"tabulador vertical", "<p>t\vv</p>", "1:5: carácter de control U+000B no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"carácter nulo", "a\x00", "1:2: carácter de control U+0000 no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"salto de página", "a\nb\fc", "2:2: carácter de control U+000C no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"control C1 tras un salto de Windows", "a\r\néx\u0085", "2:3: carácter de control U+0085 no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"no-carácter FFFE", "ab\uFFFE", "1:3: no-carácter Unicode U+FFFE no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"no-carácter FDD0", "\n\n\uFDD0", "3:1: no-carácter Unicode U+FDD0 no admitido: no puede representarse en las salidas XML; elimínelo del fichero"},
		{"UTF-8 inválido antes que el control", "\v\xe9", "1:2: el fichero no está codificado en UTF-8 (byte 0xE9 inválido); conviértalo a UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader(tt.src)
			if r.Err() == nil || r.Err().Error() != tt.want {
				t.Fatalf("Err() = %v, se esperaba %q", r.Err(), tt.want)
			}
			if _, ok := r.Peek(); ok {
				t.Error("un Reader con error debe estar vacío")
			}
		})
	}

	// Tabulador, saltos de línea, retorno de carro y U+00A0 se admiten.
	if err := NewReader("a\tb\nc\r\nd\re\u00a0f").Err(); err != nil {
		t.Errorf("carácter admitido rechazado: %v", err)
	}
}

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		c    rune
		want bool
	}{
		{'a', true},
		{'\t', true},
		{'\n', true},
		{'\r', true},
		{'\u00a0', true},
		{'\uFFFD', true},
		{0x1F600, true},
		{0, false},
		{'\v', false},
		{'\f', false},
		{0x7F, false},
		{0x85, false},
		{0x9F, false},
		{0xD800, false},
		{0xFDD0, false},
		{0xFDEF, false},
		{0xFFFE, false},
		{0x1FFFF, false},
		{0x110000, false},
		{-1, false},
	}
	for _, tt := range tests {
		if got := IsAllowed(tt.c); got != tt.want {
			t.Errorf("IsAllowed(%U) = %v, se esperaba %v", tt.c, got, tt.want)
		}
	}
}

func TestIsSpace(t *testing.T) {
	for _, c := range " \t\n\r\f" {
		if !IsSpace(c) {
			t.Errorf("IsSpace(%q) = false", c)
		}
	}
	for _, c := range "a\u00a0\u2003\v_" {
		if IsSpace(c) {
			t.Errorf("IsSpace(%q) = true", c)
		}
	}
}
