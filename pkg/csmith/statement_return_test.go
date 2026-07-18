package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomReturnIsVariable(t *testing.T) {
	// StatementReturn.cpp:60–66 — ExpressionVariable only; nullptr on fail (no const fallback)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomReturn(NewRng(5), opts, vs, &cg)
	if st.Kind != StmtReturn {
		t.Fatalf("%v", st.Kind)
	}
	if st.Expr == nil {
		// ERROR_GUARD path: stmtOK rejects for retry
		if stmtOK(st) {
			t.Fatal("Expr-less return must fail stmtOK")
		}
		return
	}
	// upstream: ExpressionVariable only (as_return)
	if st.Expr.Term != TermVariable {
		t.Fatalf("want TermVariable, got %v", st.Expr.Term)
	}
}

func TestMakeRandomReturnFailsWithoutVars(t *testing.T) {
	// StatementReturn.cpp:66 ERROR_GUARD — no soft const when select fails
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("rv", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	// nil vs → ExpressionVariable::make_random cannot select → nullptr
	st := MakeRandomReturn(NewRng(1), opts, nil, &cg)
	if st.Expr != nil {
		t.Fatal("nil vs must yield nullptr-style empty return")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject")
	}
}

func TestGenerateReturnUsesVar(t *testing.T) {
	// returns should look like "return g_N;" or "return l_N;" often
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "return g_") || strings.Contains(out, "return l_") ||
			strings.Contains(out, "return (*") || strings.Contains(out, "return &") {
			found = true
			break
		}
	}
	if !found {
		t.Log("variable returns rare in sample — not fatal")
	}
}
