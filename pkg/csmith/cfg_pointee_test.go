package csmith

import "testing"

func TestMergePointeesOfPointer(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, tgt)}
	// indirect 0 → just p itself? while(indirect-- > 0) so 0 means no steps → [p]
	got0 := MergePointeesOfPointer(p, 0, facts)
	if len(got0) != 1 || got0[0] != p {
		t.Fatalf("indir0 %+v", got0)
	}
	// indirect 1 → pointees of p
	got1 := MergePointeesOfPointer(p, 1, facts)
	if len(got1) != 1 || got1[0] != tgt {
		t.Fatalf("indir1 %+v", got1)
	}
}

func TestRhsTransferUsesMergePointees(t *testing.T) {
	p1 := CreateVariableScalarsSess(testAmbientSession, "g_p1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	p2 := CreateVariableScalarsSess(testAmbientSession, "g_p2", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	in := []*FactPointTo{MakeFactPointTo(p2, tgt)}
	// p2 as pointer expr: level 1 - 1 = 0, merge(indirect+1)=1
	rhs := &Expression{Term: TermVariable, Var: p2, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	facts := RhsToLhsTransfer(in, []*Variable{p1}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestClosestLoopingBlock(t *testing.T) {
	outer := &Block{Looping: true}
	inner := &Block{Parent: outer, Looping: false}
	if ClosestLoopingBlock(inner) != outer {
		t.Fatal("walk")
	}
	if ClosestLoopingBlock(outer) != outer {
		t.Fatal("self")
	}
	if ClosestLoopingBlock(&Block{}) != nil {
		t.Fatal("none")
	}
	// Block always live; sticky nil (no invent no-loop soft-skip past hole)
	ClearErrorSess(testAmbientSession)
	if ClosestLoopingBlock(nil) != nil {
		t.Fatal("nil ClosestLoopingBlock must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ClosestLoopingBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBreakContinueCFGEdges(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	loop := &Block{Func: f, Looping: true}
	inner := &Block{Func: f, Parent: loop, Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	f.Stack = []*Block{loop, inner}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// need globals for break test expr
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2))
	br := MakeRandomBreak(NewRngSess(testAmbientSession, 3), opts, vs, NewExprTables(opts), &cg)
	if br.Kind != StmtBreak || br.StmID == 0 {
		t.Fatal("break")
	}
	if len(loop.BreakStmIDs) != 1 {
		// closest looping from inner is inner itself (Looping true)
		if len(inner.BreakStmIDs) != 1 {
			t.Fatalf("break_stms loop=%v inner=%v", loop.BreakStmIDs, inner.BreakStmIDs)
		}
	}
	// StatementBreak.cpp — no create_cfg_edge at make; post_loop_analysis owns edges
	for _, e := range fm.CFGEdges {
		if e != nil && e.SrcID == br.StmID {
			t.Fatal("break make must not invent CFG edge")
		}
	}
	// continue still creates edge at make (StatementContinue.cpp:83)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	inner.Stmts = []Stmt{{Kind: StmtAssign}}
	f.Stack = []*Block{loop, inner}
	_ = vs.GenerateNewGlobal(AccessRead, cg2, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 4))
	cont := MakeRandomContinue(NewRngSess(testAmbientSession, 5), opts, vs, NewExprTables(opts), &cg2, inner)
	if cont.Kind != StmtContinue {
		t.Fatal("cont")
	}
	foundBack := false
	for _, e := range fm2.CFGEdges {
		if e.BackLink && e.SrcID == cont.StmID {
			foundBack = true
		}
	}
	if !foundBack {
		t.Fatalf("back edge %+v", fm2.CFGEdges)
	}
}

func TestMakeupNewVarFacts(t *testing.T) {
	// FactMgr.cpp:504 — add_new_var_fact → abstract_fact_for_var_init; no invent garbage
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	old := []*FactPointTo{}
	// pointer with const null init (CreateVariableScalars → Constant "0")
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if p == nil || p.Init == nil {
		t.Fatal("pointer init")
	}
	// seed new_facts with a related fact entry so makeup sees the var
	newF := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	MakeupNewVarFacts(&old, newF)
	got := FindRelatedPointTo(old, p)
	if got == nil {
		t.Fatal("added")
	}
	// must use init abstract (null), not invent NewFactPointTo garbage default
	if got.IsDead() {
		t.Fatal("makeup must not invent garbage for null-init pointer")
	}
	if !got.IsNull() {
		t.Fatalf("want null from init, got %+v", got.PointTo)
	}
	// idempotent
	MakeupNewVarFacts(&old, newF)
	n := 0
	for _, f := range old {
		if f != nil && f.Var == p {
			n++
		}
	}
	if n != 1 {
		t.Fatal("dup")
	}
	// address-of init preserved
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	q := CreateVariableQferSess(testAmbientSession, "g_q", pt, NewCVQualifiers([]bool{false}, []bool{false}))
	q.InitExpr = &Expression{Term: TermVariable, Var: tgt, ExprType: pt}
	old2 := []*FactPointTo{}
	MakeupNewVarFacts(&old2, []*FactPointTo{MakeFactPointTo(q, GarbagePtr)})
	fq := FindRelatedPointTo(old2, q)
	if fq == nil || fq.IsDead() || len(fq.PointTo) != 1 || fq.PointTo[0] != tgt {
		t.Fatalf("want &g_t fact, got %+v", fq)
	}
	// nil hole fails closed sticky — no invent skip past hole to makeup later vars
	r := CreateVariableScalarsSess(testAmbientSession, "g_r", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	old3 := []*FactPointTo{}
	MakeupNewVarFacts(&old3, []*FactPointTo{nil, MakeFactPointTo(r, NullPtr)})
	if FindRelatedPointTo(old3, r) != nil {
		t.Fatal("makeup must not invent past nil hole")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("makeup nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsPointingToLocalsMultiLevel(t *testing.T) {
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	blk.LocalVars = []*Variable{loc}
	// p2 → p1 → loc
	p1 := CreateVariableScalarsSess(testAmbientSession, "g_p1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	p2 := CreateVariableScalarsSess(testAmbientSession, "g_p2", PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(p1, loc),
		MakeFactPointTo(p2, p1),
	}
	if !IsPointingToLocals(p2, blk, 2, facts) {
		t.Fatal("2-level")
	}
}

func TestMakeRandomContinueRejectsFirstStmt(t *testing.T) {
	// StatementContinue.cpp:63–66 — first stmt → nullptr
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	empty := &Block{Func: f, Looping: true}
	f.Stack = []*Block{empty}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	st := MakeRandomContinue(NewRngSess(testAmbientSession, 1), opts, vs, NewExprTables(opts), &cg, empty)
	if st.Expr != nil {
		t.Fatal("first-stmt continue must not produce expr")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject null continue")
	}
	// non-empty block accepts continue
	empty.Stmts = []Stmt{{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 1}}
	st2 := MakeRandomContinue(NewRngSess(testAmbientSession, 2), opts, vs, NewExprTables(opts), &cg, empty)
	if st2.Kind != StmtContinue || st2.Expr == nil {
		t.Fatalf("%+v", st2)
	}
}
