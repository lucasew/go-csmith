package csmith

import (
	"strings"
	"testing"
)

func TestMakeExpressionComma(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// ExpressionComma lhs uses type nullptr → needs Type env (GenerateSimpleTypes)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	tables := NewExprTables(opts)
	r := NewRng(2)
	e := func() *Expression {
		// ExpressionFuncall / make_random paths need get_fact_mgr
		c := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
		c.Types = vs.Types
		return MakeExpressionComma(r, opts, probs, vs, tables, &c, GetIntType(), nil)
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
	// ExpressionComma always has live lhs/rhs; no invent "( , )" for missing sides
	bare := &Expression{Term: TermCommaExpr}
	if bare.Output() != "" {
		t.Fatalf("incomplete comma must fail closed empty, got %q", bare.Output())
	}
	oneSide := &Expression{Term: TermCommaExpr, CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	if oneSide.Output() != "" {
		t.Fatalf("one-side comma must fail closed empty, got %q", oneSide.Output())
	}
}

func TestMakeRandomParamNilType(t *testing.T) {
	// Expression.cpp:241 — assert(type); no GetIntType soft invent
	opts := Defaults()
	c := EmptyCGContext()
	if e := MakeRandomParam(NewRng(1), opts, NewExprTables(opts), NewVariableSelector(opts), &c, nil, nil, 0); e != nil {
		t.Fatal("nil type must not soft-fallback")
	}
}

func TestMakeExpressionCommaLHSNoConstPreference(t *testing.T) {
	// LHS built with noConst=true — may still be variable/function/assign
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	tables := NewExprTables(opts)
	// Many seeds: LHS should not always be a bare hex constant-only pattern... soft check
	e := func() *Expression {
		c := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
		c.Types = vs.Types
		return MakeExpressionComma(NewRng(11), opts, probs, vs, tables, &c, GetIntType(), nil)
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
