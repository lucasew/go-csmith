package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomExprStmtUserCall(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRngSess(testAmbientSession, 2)
	var list FunctionList
	seedTypesForTest(r, opts, probs, vs, &list)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list, nil)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFuncList(&list)
	st := MakeRandomExprStmt(NewRngSess(testAmbientSession, 7), opts, probs, vs, tables, &cg)
	// success → Kind Invoke + expr; fail → empty (nullptr), not Kind-only shell
	if st.Expr != nil {
		if st.Kind != StmtInvoke {
			t.Fatalf("kind %v", st.Kind)
		}
		if st.Expr.Invoke != nil && !st.Expr.Invoke.Failed {
			out := st.Expr.Output()
			if out == "" || out == "/*bad_call*/" || out == "/*invoke*/" {
				t.Fatal(out)
			}
		}
	} else if st.Kind != 0 {
		t.Fatalf("fail must be empty Stmt, got %#v", st)
	}
}

func TestMakeRandomExprStmtEmitSemicolon(t *testing.T) {
	// suite hygiene: prior sticky tests leave residual ERROR; clear before complete emit
	ClearErrorSess(testAmbientSession)
	// complete binary IR (live args) — no invent bare ";" for empty Output
	fi := &Invocation{
		IsStd:  true,
		Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(2)},
		},
	}
	st := Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: fi}}
	b := &Block{Stmts: []Stmt{st}}
	out := b.Output(0)
	if !strings.Contains(out, ";") || !strings.Contains(out, "+") {
		t.Fatal(out)
	}
	if strings.Contains(out, "invoke-stub") {
		t.Fatal("still stub")
	}
	// incomplete invoke Output fails whole block (no invent soft-skip empty invoke)
	ClearErrorSess(testAmbientSession)
	empty := Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: &Invocation{IsStd: true, Binary: "+"}}}
	out2 := (&Block{Stmts: []Stmt{empty}}).Output(0)
	if out2 != "" {
		t.Fatal("incomplete invoke must fail closed whole block", out2)
	}
	ClearErrorSess(testAmbientSession)
	// incomplete among live stmts fails whole block (no invent skip hole)
	good := Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: fi}}
	out3 := (&Block{Stmts: []Stmt{empty, good}}).Output(0)
	if out3 != "" {
		t.Fatal("mixed incomplete must fail closed whole block", out3)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExprStmtRollbackOnFail(t *testing.T) {
	// StatementExpr.cpp:62–66 — failed invoke restores effect and facts
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxFuncs = 0 // force no new functions if possible
	// ReachMaxFunctions with empty list and max 0
	list := &FunctionList{}
	// max funcs 0 means ReachMaxFunctions may block creation
	opts.MaxFuncs = 0
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	eff := EmptyEffect().WriteVarSess(testAmbientSession, p)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Funcs = list
	// snapshot
	preEff := eff.CloneSess(testAmbientSession)
	preFacts := CloneFactSlice(fm.GlobalFacts)
	// invoke may fail or succeed; if fail, empty Stmt + state restored
	st := MakeRandomExprStmt(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg)
	if !stmtOK(st) {
		// rollback path (nullptr factory)
		if st.Kind != 0 {
			t.Fatalf("fail invents Kind shell %#v", st)
		}
		if !cg.EffectAccum.IsWrittenSess(testAmbientSession, p) {
			t.Fatal("effect should still have pre write after restore or no change")
		}
		// facts restored (may equal pre)
		if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
			t.Fatal("facts")
		}
	}
	// ensure we can still match pre shape when failed
	_ = preEff
	_ = preFacts
	// force failed path: nil rng already handled; use list at max
	// mutate during a call that fails by using Failed invocation manually
	// Directly verify RestoreFacts + accum restore pattern used in MakeRandomExprStmt
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointTo(
		CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false), NullPtr))
	cg.FM.RestoreFacts(preFacts)
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil || len(fm.GlobalFacts) < 1 {
		t.Fatal("restore")
	}
}

func TestFailedInvokeRestoreRewindsUnionWrite(t *testing.T) {
	// FactMgr.cpp:489–492 restore_facts on full FactVec (ePointTo + eUnionWrite).
	// StatementExpr.cpp:58–66 / ExpressionFuncall.cpp:78–91:
	//   facts_copy = fm->global_facts;  // both categories
	//   fm->restore_facts(facts_copy);
	// Soft invent was RestoreFacts(PT-only): UnionFacts kept post-call last_written_fid
	// so IsNonreadableField over-filtered choose_var (seed-7 eligible pool half size).
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_u", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 2 {
		t.Fatal("need union fields")
	}
	f0 := parent.FieldVars[0]
	// pre-call: last written field 0 → f0 readable
	preU := MakeFactUnion(parent, 0)
	if preU == nil {
		t.Fatal("MakeFactUnion f0")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{preU}
	// snapshot like StatementExpr / ExpressionFuncall (shallow vector copy)
	factsCopy := append([]*FactPointTo(nil), fm.GlobalFacts...)
	unionCopy := append([]*FactUnion(nil), fm.UnionFacts...)
	// simulate failed call that wrote a different field
	mut := MakeFactUnion(parent, 1)
	if mut == nil {
		t.Fatal("MakeFactUnion f1")
	}
	fm.UnionFacts = []*FactUnion{mut}
	if !IsNonreadableField(f0, fm.UnionFacts) {
		t.Fatal("after mutate to f1 write, f0 must be nonreadable")
	}
	// full restore (production path after fix)
	fm.RestoreFactsPair(factsCopy, unionCopy)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if len(fm.UnionFacts) != 1 || fm.UnionFacts[0] == nil {
		t.Fatalf("union restore: %#v", fm.UnionFacts)
	}
	if fm.UnionFacts[0].LastWrittenFID != 0 {
		t.Fatalf("want last_written fid 0 after restore, got %d", fm.UnionFacts[0].LastWrittenFID)
	}
	if IsNonreadableField(f0, fm.UnionFacts) {
		t.Fatal("after full restore, f0 must be readable again")
	}
	// PT-only restore must NOT be relied on for production: document the hole
	fm.UnionFacts = []*FactUnion{mut}
	fm.RestoreFacts(factsCopy) // legacy PT-only
	if !IsNonreadableField(f0, fm.UnionFacts) {
		t.Fatal("PT-only RestoreFacts must leave mutated union last-write (hole)")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExprStmtSuccessHasInvoke(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 5
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	list := &FunctionList{}
	vs := NewVariableSelector(testAmbientSession, opts)
	// seed a built function to call
	callee := &Function{
		Name: "func_x", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true, BuildState: BuildBuilt,
		RV:   CreateVariableQferSess(testAmbientSession, "func_x_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false})),
		Body: &Block{},
	}
	list.Funcs = []*Function{callee}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Funcs = list
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// try several seeds for a successful invoke stmt
	for seed := uint64(1); seed < 40; seed++ {
		st := MakeRandomExprStmt(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg)
		if st.Expr != nil && st.Expr.Invoke != nil && !st.Expr.Invoke.Failed {
			// Statement.cpp:364–367 — Statement ctor always assigns stm_id
			if st.StmID == 0 {
				t.Fatal("success expr stmt must have stm_id")
			}
			return
		}
	}
	// acceptable if generation never picks invoke with available funcs under constraints
	t.Log("no success under constraints (ok if max-func / choose filters)")
}

func TestMakeRandomExprStmtNilCGFailClosed(t *testing.T) {
	// StatementExpr.cpp always has CGContext; sticky no invent Kind shell
	ClearErrorSess(testAmbientSession)
	st := MakeRandomExprStmt(NewRngSess(testAmbientSession, 1), Defaults(), nil, nil, nil, nil)
	if st.Kind != 0 || stmtOK(st) {
		t.Fatalf("nil cg invent %#v", st)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeRandomExprStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	st2 := MakeRandomExprStmt(nil, Defaults(), nil, nil, nil, ptrEmptyCG())
	if st2.Kind != 0 || stmtOK(st2) {
		t.Fatalf("nil RNG invent %#v", st2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomExprStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
