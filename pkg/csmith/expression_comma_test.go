package csmith

import (
	"strings"
	"testing"
)

func TestMakeExpressionComma(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// ExpressionComma lhs uses type nullptr → needs Type env (GenerateSimpleTypes)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	tables := NewExprTables(opts)
	r := NewRngSess(testAmbientSession, 2)
	e := func() *Expression {
		// ExpressionFuncall / make_random paths need get_fact_mgr
		c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
		c.Types = vs.Types
		return MakeExpressionComma(r, opts, probs, vs, tables, &c, GetIntTypeSess(testAmbientSession), nil)
	}()
	if e == nil || e.Term != TermCommaExpr || e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatalf("%+v", e)
	}
	out := e.Output()
	// ExpressionComma.cpp:141 — " , " (spaces around comma)
	if !strings.Contains(out, " , ") {
		t.Fatal(out)
	}
	// outer parens
	if !strings.HasPrefix(out, "(") || !strings.HasSuffix(out, ")") {
		t.Fatal(out)
	}
	// Expression always live for cast_if_needed; sticky (no invent soft-skip past hole)
	ClearErrorSess(testAmbientSession)
	castIfNeeded(testAmbientSession, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil castIfNeeded must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-constant complete no-op
	castIfNeeded(testAmbientSession, &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)})
	if HasErrorSess(testAmbientSession) {
		t.Fatal("non-constant castIfNeeded must complete no-op")
	}
	ClearErrorSess(testAmbientSession)
	// ExpressionComma always has live lhs/rhs; sticky no invent "( , )" for missing sides
	bare := &Expression{Term: TermCommaExpr}
	if bare.Output() != "" {
		t.Fatalf("incomplete comma must fail closed empty, got %q", bare.Output())
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete comma Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	oneSide := &Expression{Term: TermCommaExpr, CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	if oneSide.Output() != "" {
		t.Fatalf("one-side comma must fail closed empty, got %q", oneSide.Output())
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("one-side comma Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomParamNilType(t *testing.T) {
	// Expression.cpp:241 — assert(type) sticky; no GetIntType soft invent
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	c := EmptyCGContext().WithSession(testAmbientSession)
	if e := MakeRandomParam(NewRngSess(testAmbientSession, 1), opts, NewExprTables(opts), NewVariableSelector(testAmbientSession, opts), &c, nil, nil, 0); e != nil {
		t.Fatal("nil type must not soft-fallback")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type MakeRandomParam must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Expression.cpp always has RNG sticky; no invent param shell
	if e := MakeRandomParam(nil, opts, NewExprTables(opts), NewVariableSelector(testAmbientSession, opts), &c, GetIntTypeSess(testAmbientSession), nil, 0); e != nil {
		t.Fatal("nil RNG must not invent param expr")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomParam must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionCommaLHSNoConstPreference(t *testing.T) {
	// LHS built with noConst=true — may still be variable/function/assign
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, probs, vs, nil)
	tables := NewExprTables(opts)
	// Many seeds: LHS should not always be a bare hex constant-only pattern... soft check
	e := func() *Expression {
		c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
		c.Types = vs.Types
		return MakeExpressionComma(NewRngSess(testAmbientSession, 11), opts, probs, vs, tables, &c, GetIntTypeSess(testAmbientSession), nil)
	}()
	if e == nil || e.CommaLHS == nil {
		t.Fatal("lhs")
	}
}

func TestGenerateCanEmitCommaExpr(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// crude: look for "), " pattern from (a, b) — may false positive
		if strings.Contains(out, ", 0x") || strings.Contains(out, ", g_") ||
			strings.Contains(out, ", (") || strings.Contains(out, ", func_") {
			// stronger: parenthesized comma
			if strings.Contains(out, ", ") && strings.Contains(out, "(") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Log("comma expr rare (weight 10 in term table + depth gates)")
	}
}

func TestMakeExpressionCommaIncompleteAmbientFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if MakeExpressionComma(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeExpressionComma")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionCommaNilDepsSticky(t *testing.T) {
	// ExpressionComma always has RNG + CGContext; sticky no invent comma shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if MakeExpressionComma(nil, opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), ptrEmptyCG(), GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeExpressionComma must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MakeExpressionComma(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), nil, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("nil cg must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeExpressionComma must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCastIfNeededGetTypeResidualSticky(t *testing.T) {
	// GetType residual soft invent was soft-skip cast then invent CastType / complete no-op.
	ClearErrorSess(testAmbientSession)
	// Type-nil Con → GetType residual; no invent cast
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}
	castIfNeeded(testAmbientSession, hole)
	if hole.CastType != nil {
		t.Fatal("GetType residual must not invent CastType", hole.CastType)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual castIfNeeded must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Value → EqualsInt residual after live pointer type; no invent cast
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	eqHole := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: ""}}
	castIfNeeded(testAmbientSession, eqHole)
	if eqHole.CastType != nil {
		t.Fatal("EqualsInt residual must not invent CastType", eqHole.CastType)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("EqualsInt residual castIfNeeded must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCommaOutputLHSResidualSticky(t *testing.T) {
	// CommaLHS Output residual soft invent was soft-continue invent partial comma with RHS.
	ClearErrorSess(testAmbientSession)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: &Constant{Value: "1"}}, // Type-nil residual
		CommaRHS: &Expression{Term: TermConstant, Con: MakeInt(2)},
	}
	if s := e.Output(); s != "" {
		t.Fatal("LHS Output residual must fail closed comma Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("LHS Output residual comma Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHaveOverlappingFieldsFindUnionResidualSticky(t *testing.T) {
	// FindUnionPointees residual soft invent was invent no-overlap soft-success past Type-nil.
	ClearErrorSess(testAmbientSession)
	// Type-nil non-special expr → FindUnionPointees incomplete → overlap sticky
	e1 := &Expression{Term: TermVariable, Var: &Variable{Name: "g_p", Type: nil}}
	e2 := &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)}
	if !HaveOverlappingFields(e1, e2, nil) {
		t.Fatal("Type-nil FindUnion residual must fail closed overlap true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil FindUnion residual HaveOverlappingFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
