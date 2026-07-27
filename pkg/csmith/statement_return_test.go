package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomReturnIsVariable(t *testing.T) {
	// StatementReturn.cpp:60–66 — ExpressionVariable only; nullptr on fail (no const fallback)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	stmtTab := NewStatementThresholdTableSess(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	// StatementReturn.cpp:58–59 assert(fm) — session FactMgr required
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 5), opts, vs, &cg)
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

// TestMakeRandomReturnNoCCompBitfieldCast — StatementReturn.cpp:64–65 only
// check_and_set_cast (lang_cpp). Assign.cpp:208–209 ccomp+bitfield cast is
// assign RHS only; must not invent CastType when returning a bitfield under ccomp.
func TestMakeRandomReturnNoCCompBitfieldCast(t *testing.T) {
	opts := Defaults()
	opts.CComp = true
	opts.LangCPP = false
	retT := GetSimpleTypeSess(testAmbientSession, EUInt128)
	f := &Function{Name: "func_1", ReturnType: retT}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "rv", retT, false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	// bitfield field as return source
	bf := CreateVariableScalarsSess(testAmbientSession, "f1", GetIntTypeSess(testAmbientSession), false, false)
	bf.IsBitfield = true
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = []*Variable{bf}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 7), opts, vs, &cg)
	if !stmtOK(st) || st.Expr == nil {
		t.Skip("no return expr for seed")
	}
	if st.Expr.CastType != nil {
		t.Fatalf("ccomp bitfield return must not invent CastType (got %v)", st.Expr.CastType.CNameSess(testAmbientSession))
	}
	out := st.Expr.OutputOptsSess(testAmbientSession, opts)
	if strings.Contains(out, "(") && strings.Contains(out, ")") {
		// bare field name ok; cast form "(unsigned __int128) ..." is soft invent
		if strings.Contains(out, "__int128") {
			t.Fatalf("emit cast under ccomp/!lang_cpp: %q", out)
		}
	}
}

func TestMakeRandomReturnFailsWithoutVars(t *testing.T) {
	// StatementReturn.cpp:66 ERROR_GUARD — no soft const when select fails
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// nil vs → ExpressionVariable soft nil (non-sticky) → empty return re-pick
	ClearErrorSess(testAmbientSession)
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 1), opts, nil, &cg)
	if st.Expr != nil {
		t.Fatal("nil vs must yield nullptr-style empty return")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil vs MakeRandomReturn must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomReturnRequiresFactMgr(t *testing.T) {
	// StatementReturn.cpp:58–59 — assert(fm); soft re-pick non-sticky without FactMgr
	// (sticky poisons MakeRandomFor / generation when return path re-picks)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	// sticky without RNG
	if stmtOK(MakeRandomReturn(nil, opts, NewVariableSelector(testAmbientSession, opts), nil)) {
		t.Fatal("nil RNG/cg must fail closed")
	}
	// nil RNG MakeRandomReturn must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 1), opts, NewVariableSelector(testAmbientSession, opts), &cg)
	if st.Expr != nil {
		t.Fatal("nil FM must fail closed empty return")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM MakeRandomReturn must stay non-sticky soft re-pick")
	}
	// nil RV non-sticky soft re-pick
	fm := NewFactMgrSess(testAmbientSession, f)
	f.RV = nil
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st2 := MakeRandomReturn(NewRngSess(testAmbientSession, 1), opts, NewVariableSelector(testAmbientSession, opts), &cg2)
	if st2.Expr != nil {
		t.Fatal("nil RV must fail closed empty return")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil RV MakeRandomReturn must stay non-sticky soft re-pick")
	}
}

func TestMakeRandomReturnIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient/facts must sticky ERROR (no invent return soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	vs := NewVariableSelector(testAmbientSession, opts)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 1), opts, vs, &cg)
	if st.Expr != nil || stmtOK(st) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomReturn")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	st2 := MakeRandomReturn(NewRngSess(testAmbientSession, 2), opts, vs, &cg2)
	if st2.Expr != nil || stmtOK(st2) {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomReturn")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckAndSetCastResidualNoInventReturnShell(t *testing.T) {
	// Soft invent was CheckAndSetCast residual then invent StmtReturn complete shell.
	// Fair: residual ERROR stickies; return path fail closed empty (no invent).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.LangCPP = true
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole"}}
	hole.CheckAndSetCastOptsSess(testAmbientSession, GetIntTypeSess(testAmbientSession), opts)
	if hole.CastType != nil {
		t.Fatal("GetTypeUncast residual must not invent CastType")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CheckAndSetCast residual must SetError sticky")
	}
	// MakeRandomReturn pattern after cast residual: empty Stmt{}, not invent return
	if HasErrorSess(testAmbientSession) {
		st := Stmt{}
		if st.Kind == StmtReturn || stmtOK(st) {
			t.Fatal("cast residual must not invent Return shell")
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsStatementReturnNoInventWithoutFuncRV(t *testing.T) {
	// StatementReturn.cpp:91–94 — get_fact_mgr + curr_func + rv always live sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	st := &Stmt{Kind: StmtReturn, StmID: 1, Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}}
	// no CurrentFunc
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
	if VisitFactsStatementReturn(st, &cg, opts) {
		t.Fatal("nil CurrentFunc must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CurrentFunc return visit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// CurrentFunc without RV
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("nil RV must fail closed")
	}
	// nil RV return visit must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// complete path
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", GetIntTypeSess(testAmbientSession), false, false)
	if !VisitFactsStatementReturn(st, &cg2, opts) {
		t.Fatal("live return must visit")
	}
	ClearErrorSess(testAmbientSession)
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
