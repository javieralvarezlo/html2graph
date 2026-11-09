package token

import (
	"errors"
	"fmt"
	"testing"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		k    Kind
		want string
	}{
		{EOF, "EOF"},
		{StartTag, "StartTag"},
		{EndTag, "EndTag"},
		{SelfClosing, "SelfClosing"},
		{Text, "Text"},
		{Comment, "Comment"},
		{Doctype, "Doctype"},
		{LBrace, "LBrace"},
		{RBrace, "RBrace"},
		{LBracket, "LBracket"},
		{RBracket, "RBracket"},
		{Colon, "Colon"},
		{Comma, "Comma"},
		{String, "String"},
		{Number, "Number"},
		{True, "True"},
		{False, "False"},
		{Null, "Null"},
		{Kind(-1), "Kind(-1)"},
		{Kind(99), "Kind(99)"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, se esperaba %q", int(tt.k), got, tt.want)
		}
	}
}

// TestKindNamesCompleto comprueba que ninguna clase se ha quedado sin nombre
// en la tabla (por ejemplo, al añadir una constante nueva).
func TestKindNamesCompleto(t *testing.T) {
	for k := EOF; k <= Null; k++ {
		if kindNames[k] == "" {
			t.Errorf("la clase %d no tiene nombre", int(k))
		}
	}
	if len(kindNames) != int(Null)+1 {
		t.Errorf("kindNames tiene %d entradas, se esperaban %d", len(kindNames), int(Null)+1)
	}
}

func TestTokenString(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want string
	}{
		{"eof", Token{Kind: EOF, Line: 3, Col: 1}, "3:1 EOF"},
		{"apertura sin atributos", Token{Kind: StartTag, Value: "p", Line: 1, Col: 1}, `1:1 StartTag "p"`},
		{
			"apertura con atributos",
			Token{Kind: StartTag, Value: "a", Attrs: []Attr{{"href", "http://www.issi.uned.es"}, {"title", `di "hola"`}}, Line: 5, Col: 3},
			`5:3 StartTag "a" [href="http://www.issi.uned.es" title="di \"hola\""]`,
		},
		{"texto", Token{Kind: Text, Value: "énfasis", Line: 7, Col: 34}, `7:34 Text "énfasis"`},
		{"puntuación JSON", Token{Kind: LBrace, Line: 1, Col: 1}, "1:1 LBrace"},
		{"número", Token{Kind: Number, Value: "-1.5e3", Line: 2, Col: 9}, `2:9 Number "-1.5e3"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.String(); got != tt.want {
				t.Errorf("String() = %s, se esperaba %s", got, tt.want)
			}
		})
	}
}

func TestPosError(t *testing.T) {
	err := Errorf(12, 7, "se esperaba </%s> y se encontró </%s>", "b", "p")
	if got, want := err.Error(), "12:7: se esperaba </b> y se encontró </p>"; got != want {
		t.Errorf("Error() = %q, se esperaba %q", got, want)
	}

	// Un PosError envuelto debe poder recuperarse con errors.As para que la
	// CLI pueda distinguir los errores de la entrada de los demás.
	var wrapped error = fmt.Errorf("leyendo entrada: %w", err)
	var pe *PosError
	if !errors.As(wrapped, &pe) {
		t.Fatal("errors.As no encuentra el *PosError envuelto")
	}
	if pe.Line != 12 || pe.Col != 7 {
		t.Errorf("posición = %d:%d, se esperaba 12:7", pe.Line, pe.Col)
	}
}
