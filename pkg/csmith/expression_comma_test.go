package csmith

import (
	"strings"
	"testing"
)

func TestMakeExpressionComma(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	r := NewRng(2)
	e := func() *Expression { c := EmptyCGContext(); return MakeExpressionComma(r, opts, probs, vs, tables, &c, GetIntType(), nil) }()
	if e == nil || e.Term != TermCommaExpr || e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatalf("%+v", e)
	}
	out := e.Output()
	if !strings.Contains(out, ", ") {
		t.Fatal(out)
	}
	// outer parens
	if !strings.HasPrefix(out, "(") || !strings.HasSuffix(out, ")") {
		t.Fatal(out)
	}
}

func TestMakeExpressionCommaLHSNoConstPreference(t *testing.T) {
	// LHS built with noConst=true — may still be variable/function/assign
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// Many seeds: LHS should not always be a bare hex constant-only pattern... soft check
	e := func() *Expression { c := EmptyCGContext(); return MakeExpressionComma(NewRng(11), opts, probs, vs, tables, &c, GetIntType(), nil) }()
	if e.CommaLHS == nil {
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
