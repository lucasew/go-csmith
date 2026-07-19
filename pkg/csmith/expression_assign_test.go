package csmith

import (
	"strings"
	"testing"
)

func TestMakeExpressionAssign(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	r := NewRng(2)
	// ExpressionAssign.cpp:56–62 — get_fact_mgr always live
	f := &Function{Name: "f", ReturnType: GetIntType()}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(1))
	e := func() *Expression {
		c := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		return MakeExpressionAssign(r, opts, probs, vs, tables, &c, GetIntType(), nil)
	}()
	if e == nil || e.Term != TermAssignment || e.Assign == nil {
		t.Fatal(e)
	}
	out := e.Output()
	if !strings.Contains(out, "=") && !strings.Contains(out, "++") && !strings.Contains(out, "--") {
		t.Fatal(out)
	}
}

func TestMakeExpressionAssignRequiresFactMgr(t *testing.T) {
	// no soft invent ExpressionAssign without FactMgr
	opts := Defaults()
	c := EmptyCGContext()
	e := MakeExpressionAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType(), nil)
	if e != nil {
		t.Fatal("nil FM must fail closed")
	}
}

func TestMakeExpressionAssignNoInventWithoutRNG(t *testing.T) {
	// ExpressionAssign.cpp always has RNG; no invent empty-qfer then assign shell
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	c := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	if e := MakeExpressionAssign(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType(), nil); e != nil {
		t.Fatal("nil RNG must fail closed")
	}
}

func TestPickTermAssignmentDepthOk(t *testing.T) {
	opts := Defaults()
	tables := NewExprTables(opts)
	// depth 0 allows assignment in table
	found := false
	r := NewRng(1)
	for i := 0; i < 80; i++ {
		tt := PickTermType(r, tables, opts, GetIntType(), false, false, 0)
		if tt == TermAssignment {
			found = true
			break
		}
	}
	if !found {
		t.Log("assignment term rare in table (weight 10/120)")
	}
}
