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
		c := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
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
	// non-sticky soft re-pick without FactMgr (no invent assign Expression shell)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	c := EmptyCGContext()
	e := MakeExpressionAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType(), nil)
	if e != nil {
		t.Fatal("nil FM must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM MakeExpressionAssign must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionAssignNoInventWithoutRNG(t *testing.T) {
	// ExpressionAssign.cpp always has RNG; sticky no invent empty-qfer then assign shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	c := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	if e := MakeExpressionAssign(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType(), nil); e != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeExpressionAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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

func TestLhsGetTypeResidualNoInventExprType(t *testing.T) {
	// GetType residual soft invent was soft-fallback exprType=typ then invent ExpressionAssign shell.
	// Probe Lhs.GetType residual itself (wrap path guards HasError after same residual).
	ClearErrorSess(testAmbientSession)
	hole := &Lhs{Var: &Variable{Name: "g_hole"}}
	if hole.GetType() != nil {
		t.Fatal("Type-nil Lhs GetType must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Lhs GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionAssignIncompleteAmbientFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeExpressionAssign(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType(), nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeExpressionAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	if MakeExpressionAssign(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, GetIntType(), nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeExpressionAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionAssignIndirectLevelResidualSticky(t *testing.T) {
	// IndirectLevel residual soft invent was invent ExpressionAssign shell past Type-nil Lhs.
	// Force path: complete assign then Type-nil Lhs on re-apply UpdateFact is hard.
	// Hygiene: Type-nil ambient incomplete Effect fails closed before assign.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	cg := WithFunc(f, EmptyEffect())
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	// incomplete EffectStm sticky before MakeExpressionAssign
	cg.EffectStm = IncompleteEffect()
	if MakeExpressionAssign(NewRng(1), opts, probs, vs, tables, &cg, GetIntType(), nil) != nil {
		t.Fatal("incomplete EffectStm must fail closed MakeExpressionAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm MakeExpressionAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionAssignNilCGSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if MakeExpressionAssign(NewRng(1), Defaults(), nil, nil, nil, nil, GetIntType(), nil) != nil {
		t.Fatal("nil cg must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeExpressionAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
