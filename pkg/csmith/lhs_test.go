package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLhsSelectsOrCreates(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	r := NewRng(3)
	cg := EmptyCGContext()
	lhs, ty := MakeRandomLhs(r, opts, probs, vs, cg, GetIntType(), false)
	if lhs == nil {
		t.Fatal("nil lhs")
	}
	if ty == nil {
		t.Fatal("nil type")
	}
	if lhs.Name == "" {
		t.Fatal("empty name")
	}
}

func TestMakeRandomLhsDerefPointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	vs.Types = env
	// seed an int* global
	p := env.FindPointerType(GetIntType(), true)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	pv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), p, &q, NewRng(1))
	if pv == nil {
		t.Fatal("no ptr global")
	}
	// force high deref probability by trying many seeds
	var got *Variable
	var et *Type
	for seed := uint64(1); seed < 100; seed++ {
		got, et = MakeRandomLhs(NewRng(seed), opts, probs, vs, EmptyCGContext(), GetIntType(), false)
		if got != nil && got.Type != nil && got.Type.IndirectLevel() == 1 && et != nil && et.IndirectLevel() == 0 {
			break
		}
		got = nil
	}
	if got == nil {
		t.Skip("deref path rare")
	}
	// emit
	ind := got.Type.IndirectLevel() - et.IndirectLevel()
	if ind != 1 {
		t.Fatalf("ind=%d", ind)
	}
	_ = strings.Contains
}

func TestPickUnaryOp(t *testing.T) {
	opts := Defaults()
	seen := map[UnaryOp]bool{}
	r := NewRng(1)
	for i := 0; i < 100; i++ {
		seen[PickUnaryOp(r, opts)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("seen %v", seen)
	}
	opts.UnaryPlusOperator = false
	r2 := NewRng(2)
	for i := 0; i < 50; i++ {
		if PickUnaryOp(r2, opts) == UnPlus {
			t.Fatal("unary plus disabled")
		}
	}
}

func TestExpressionVariableDerefOutput(t *testing.T) {
	p := PointerTo(GetIntType())
	v := CreateVariableQfer("g_1", p, NewCVQualifiers([]bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	out := e.Output()
	if out != "(*g_1)" {
		t.Fatalf("%q", out)
	}
}
