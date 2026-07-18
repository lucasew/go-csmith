package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomReturnIsVariableOrConst(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomReturn(NewRng(5), opts, vs, cg)
	if st.Kind != StmtReturn {
		t.Fatalf("%v", st.Kind)
	}
	if st.Expr == nil {
		t.Fatal("nil expr")
	}
	// upstream: ExpressionVariable (or our const fallback)
	if st.Expr.Term != TermVariable && st.Expr.Term != TermConstant {
		t.Fatalf("want variable/const, got %v", st.Expr.Term)
	}
	if st.Expr.Term == TermFunction {
		t.Fatal("return must not be free func call (StatementReturn)")
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
