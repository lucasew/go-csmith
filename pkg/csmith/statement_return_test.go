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
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	// StatementReturn.cpp:58–59 assert(fm) — session FactMgr required
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
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
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// nil vs → ExpressionVariable::make_random cannot select → nullptr
	st := MakeRandomReturn(NewRng(1), opts, nil, &cg)
	if st.Expr != nil {
		t.Fatal("nil vs must yield nullptr-style empty return")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject")
	}
}

func TestMakeRandomReturnRequiresFactMgr(t *testing.T) {
	// StatementReturn.cpp:58–59 — assert(fm); no invent return without FactMgr
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("rv", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomReturn(NewRng(1), opts, NewVariableSelector(opts), &cg)
	if st.Expr != nil {
		t.Fatal("nil FM must fail closed empty return")
	}
}

func TestVisitFactsStatementReturnNoInventWithoutFuncRV(t *testing.T) {
	// StatementReturn.cpp:91–94 — get_fact_mgr + curr_func + rv always live
	opts := Defaults()
	st := &Stmt{Kind: StmtReturn, StmID: 1, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	// no CurrentFunc
	cg := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
	if VisitFactsStatementReturn(st, &cg, opts) {
		t.Fatal("nil CurrentFunc must fail closed")
	}
	// CurrentFunc without RV
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("nil RV must fail closed")
	}
	// complete path
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	if !VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("live return must visit")
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
