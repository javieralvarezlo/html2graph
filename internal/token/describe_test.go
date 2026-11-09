package token

import "testing"

func TestDescribe(t *testing.T) {
	tests := []struct {
		c    rune
		want string
	}{
		{' ', "' ' (espacio)"},
		{'\t', "tabulador"},
		{'\n', "salto de línea"},
		{'\r', "retorno de carro"},
		{'\f', "salto de página"},
		{'\'', `"'" (comilla simple)`},
		{'$', "'$'"},
		{'x', "'x'"},
		{'€', "'€' (U+20AC)"},
		{'\u00a0', "U+00A0"},
		{'\u0301', "U+0301"},
		{'\u200B', "U+200B"},
		{'\v', "U+000B"},
	}
	for _, tt := range tests {
		if got := Describe(tt.c); got != tt.want {
			t.Errorf("Describe(%U) = %q, se esperaba %q", tt.c, got, tt.want)
		}
	}
}

func TestDescribeDisallowed(t *testing.T) {
	tests := []struct {
		c    rune
		want string
	}{
		{0, "carácter de control U+0000"},
		{'\b', "carácter de control U+0008"},
		{0x85, "carácter de control U+0085"},
		{0xFDD0, "no-carácter Unicode U+FDD0"},
		{0x1FFFE, "no-carácter Unicode U+1FFFE"},
	}
	for _, tt := range tests {
		if got := DescribeDisallowed(tt.c); got != tt.want {
			t.Errorf("DescribeDisallowed(%U) = %q, se esperaba %q", tt.c, got, tt.want)
		}
	}
}
