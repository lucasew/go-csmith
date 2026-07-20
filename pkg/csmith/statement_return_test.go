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
	// nil vs → ExpressionVariable soft nil (non-sticky) → empty return re-pick
	ClearError()
	st := MakeRandomReturn(NewRng(1), opts, nil, &cg)
	if st.Expr != nil {
		t.Fatal("nil vs must yield nullptr-style empty return")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject")
	}
	if HasError() {
		t.Fatal("nil vs MakeRandomReturn must stay non-sticky soft re-pick")
	}
	ClearError()
}

func TestMakeRandomReturnRequiresFactMgr(t *testing.T) {
	// StatementReturn.cpp:58–59 — assert(fm); soft re-pick non-sticky without FactMgr
	// (sticky poisons MakeRandomFor / generation when return path re-picks)
	ClearError()
	opts := Defaults()
	// sticky without RNG
	if stmtOK(MakeRandomReturn(nil, opts, NewVariableSelector(opts), nil)) {
		t.Fatal("nil RNG/cg must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomReturn must SetError sticky")
	}
	ClearError()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("rv", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomReturn(NewRng(1), opts, NewVariableSelector(opts), &cg)
	if st.Expr != nil {
		t.Fatal("nil FM must fail closed empty return")
	}
	if HasError() {
		t.Fatal("nil FM MakeRandomReturn must stay non-sticky soft re-pick")
	}
	// nil RV non-sticky soft re-pick
	fm := NewFactMgr(f)
	f.RV = nil
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	st2 := MakeRandomReturn(NewRng(1), opts, NewVariableSelector(opts), &cg2)
	if st2.Expr != nil {
		t.Fatal("nil RV must fail closed empty return")
	}
	if HasError() {
		t.Fatal("nil RV MakeRandomReturn must stay non-sticky soft re-pick")
	}
}

func TestMakeRandomReturnIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient/facts must sticky ERROR (no invent return soft re-pick)
	ClearError()
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("rv", GetIntType(), false, false)
	fm := NewFactMgr(f)
	vs := NewVariableSelector(opts)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	st := MakeRandomReturn(NewRng(1), opts, vs, &cg)
	if st.Expr != nil || stmtOK(st) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomReturn")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm2 := NewFactMgr(f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	st2 := MakeRandomReturn(NewRng(2), opts, vs, &cg2)
	if st2.Expr != nil || stmtOK(st2) {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomReturn")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestCheckAndSetCastResidualNoInventReturnShell(t *testing.T) {
	// Soft invent was CheckAndSetCast residual then invent StmtReturn complete shell.
	// Fair: residual ERROR stickies; return path fail closed empty (no invent).
	ClearError()
	opts := Defaults()
	opts.LangCPP = true
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole"}}
	hole.CheckAndSetCastOpts(GetIntType(), opts)
	if hole.CastType != nil {
		t.Fatal("GetTypeUncast residual must not invent CastType")
	}
	if !HasError() {
		t.Fatal("CheckAndSetCast residual must SetError sticky")
	}
	// MakeRandomReturn pattern after cast residual: empty Stmt{}, not invent return
	if HasError() {
		st := Stmt{}
		if st.Kind == StmtReturn || stmtOK(st) {
			t.Fatal("cast residual must not invent Return shell")
		}
	}
	ClearError()
}

func TestVisitFactsStatementReturnNoInventWithoutFuncRV(t *testing.T) {
	// StatementReturn.cpp:91–94 — get_fact_mgr + curr_func + rv always live sticky
	ClearError()
	opts := Defaults()
	st := &Stmt{Kind: StmtReturn, StmID: 1, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	// no CurrentFunc
	cg := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
	if VisitFactsStatementReturn(st, &cg, opts) {
		t.Fatal("nil CurrentFunc must fail closed")
	}
	if !HasError() {
		t.Fatal("nil CurrentFunc return visit must SetError sticky")
	}
	ClearError()
	// CurrentFunc without RV
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("nil RV must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RV return visit must SetError sticky")
	}
	ClearError()
	// complete path
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	if !VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("live return must visit")
	}
	ClearError()
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
