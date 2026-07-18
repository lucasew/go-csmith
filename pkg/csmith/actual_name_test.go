package csmith

import (
	"strings"
	"testing"
)

func TestGetActualNameAndPrefix(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if g.GetActualName(false) != "g_1" {
		t.Fatal(g.GetActualName(false))
	}
	// default generator: prefix returns name unchanged
	if GetPrefixedName("g_1", true) != "g_1" {
		t.Fatal("prefix")
	}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	if l.GetActualName(true) != "l_1" {
		t.Fatal("local")
	}
}

func TestOutputDefVolatileComment(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	s := v.OutputDefOpts(true, false)
	if !strings.Contains(s, "VOLATILE GLOBAL g_v") {
		t.Fatal(s)
	}
	// local volatile — no comment
	l := CreateVariableScalars("l_v", GetIntType(), false, true)
	l.Init = MakeInt(1)
	s2 := l.OutputDefOpts(false, false)
	if strings.Contains(s2, "VOLATILE GLOBAL") {
		t.Fatal(s2)
	}
}

func TestOutputAddrOf(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	v.UseVolRVal = true
	// even with wrap, AddrOf uses bare name
	if v.OutputAddrOf(false) != "&g_1" {
		t.Fatal(v.OutputAddrOf(false))
	}
}

func TestNewProgramGeneratorSharesSessionProbs(t *testing.T) {
	// C++ Probabilities singleton — generator and VS must share one table
	opts := Defaults()
	g := NewProgramGenerator(opts)
	if g.Probs == nil || g.VS == nil || g.VS.Probs == nil {
		t.Fatal("missing probs")
	}
	if g.Probs != g.VS.Probs {
		t.Fatal("VS must share session Probs, not invent a second table")
	}
}

func TestOutputGlobalsVolatileComment(t *testing.T) {
	opts := Defaults()
	g := NewProgramGenerator(opts)
	// Force a known scalar volatile global through OutputGlobals path.
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.Init = MakeInt(0)
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if !strings.Contains(out, "VOLATILE GLOBAL g_v") {
		t.Fatal(out)
	}
}

func TestOutputGlobalsVolatileArrayComment(t *testing.T) {
	opts := Defaults()
	g := NewProgramGenerator(opts)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			Qfer: NewCVQualifiers([]bool{false}, []bool{true}),
			Init: MakeInt(0),
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	g.VS.GlobalList = []*Variable{&av.Variable}
	g.VS.Arrays = []*ArrayVariable{av}
	out := g.OutputGlobals()
	if !strings.Contains(out, "VOLATILE GLOBAL g_a") {
		t.Fatal(out)
	}
}
