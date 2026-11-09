package rules

import "testing"

func TestDefault(t *testing.T) {
	var r Rules = Default{}
	for _, tag := range []string{"html", "p", "a", "br", "x-desconocida", ""} {
		if got := r.ElementAction(tag); got != Include {
			t.Errorf("ElementAction(%q) = %v, se esperaba Include", tag, got)
		}
	}
	if got := r.AttrMode(); got != AttrsInNode {
		t.Errorf("AttrMode() = %v, se esperaba AttrsInNode", got)
	}
	if !r.IncludeText() {
		t.Error("IncludeText() = false, se esperaba true")
	}
}

// TestValoresCero comprueba que el valor cero de cada tipo es el
// comportamiento por defecto.
func TestValoresCero(t *testing.T) {
	var a Action
	var m AttrMode
	if a != Include || m != AttrsInNode {
		t.Errorf("valores cero: %v y %v; se esperaban Include y AttrsInNode", a, m)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		got, want string
	}{
		{Include.String(), "Include"},
		{Ignore.String(), "Ignore"},
		{Flatten.String(), "Flatten"},
		{Action(-1).String(), "Action(-1)"},
		{Action(3).String(), "Action(3)"},
		{AttrsInNode.String(), "AttrsInNode"},
		{AttrsAsChildren.String(), "AttrsAsChildren"},
		{AttrsNone.String(), "AttrsNone"},
		{AttrMode(3).String(), "AttrMode(3)"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("String() = %q, se esperaba %q", tt.got, tt.want)
		}
	}
}
