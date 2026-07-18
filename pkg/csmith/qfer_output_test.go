package csmith

import (
	"strings"
	"testing"
)

func TestRestrictWrite(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	q.Restrict(AccessWrite, EmptyCGContext())
	if q.IsConst() {
		t.Fatal("const cleared")
	}
	// SE-free keeps volatile
	if !q.IsVolatile() {
		t.Fatal("vol kept se-free")
	}
	q2 := NewCVQualifiers([]bool{false}, []bool{true})
	q2.Restrict(AccessRead, WithEffectContext(WithSideEffects()))
	if q2.IsVolatile() {
		t.Fatal("vol cleared non-se-free")
	}
}

func TestOutputQualifiedTypeSimple(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedType(GetIntType())
	if !strings.Contains(s, "const") || !strings.Contains(s, "volatile") || !strings.Contains(s, "int") {
		t.Fatal(s)
	}
}

func TestVariableOutputDef(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	v.Init = MakeInt(3)
	s := v.OutputDef(true)
	if !strings.Contains(s, "static") || !strings.Contains(s, "const") || !strings.Contains(s, "g_1") || !strings.Contains(s, "3") {
		t.Fatal(s)
	}
}

func TestOutputGlobalsUsesOutputDef(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	g := NewProgramGenerator(opts)
	g.GenerateAllTypes()
	// force a global
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = g.VS.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, g.Rng)
	out := g.OutputGlobals()
	if !strings.Contains(out, "static") || !strings.Contains(out, "g_") {
		t.Fatal(out)
	}
}
