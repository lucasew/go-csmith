package csmith

import "testing"

func TestMergePointeesOfPointer(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
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
	p1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	in := []*FactPointTo{MakeFactPointTo(p2, tgt)}
	// p2 as pointer expr: level 1 - 1 = 0, merge(indirect+1)=1
	rhs := &Expression{Term: TermVariable, Var: p2, ExprType: PointerTo(GetIntType())}
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
}

func TestBreakContinueCFGEdges(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	loop := &Block{Func: f, Looping: true}
	inner := &Block{Func: f, Parent: loop, Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	f.Stack = []*Block{loop, inner}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// need globals for break test expr
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	br := MakeRandomBreak(NewRng(3), opts, vs, NewExprTables(opts), cg)
	if br.Kind != StmtBreak || br.StmID == 0 {
		t.Fatal("break")
	}
	if len(loop.BreakStmIDs) != 1 {
		// closest looping from inner is inner itself (Looping true)
		if len(inner.BreakStmIDs) != 1 {
			t.Fatalf("break_stms loop=%v inner=%v", loop.BreakStmIDs, inner.BreakStmIDs)
		}
	}
	if len(fm.CFGEdges) < 1 {
		t.Fatal("no edge")
	}
	// continue
	fm2 := NewFactMgr(f)
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	inner.Stmts = []Stmt{{Kind: StmtAssign}}
	f.Stack = []*Block{loop, inner}
	_ = vs.GenerateNewGlobal(AccessRead, cg2, GetIntType(), &q, NewRng(4))
	cont := MakeRandomContinue(NewRng(5), opts, vs, NewExprTables(opts), cg2, inner)
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
	old := []*FactPointTo{}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	newF := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	MakeupNewVarFacts(&old, newF)
	if FindRelatedPointTo(old, p) == nil {
		t.Fatal("added")
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
}

func TestIsPointingToLocalsMultiLevel(t *testing.T) {
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk.LocalVars = []*Variable{loc}
	// p2 → p1 → loc
	p1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	p2 := CreateVariableScalars("g_p2", PointerTo(PointerTo(GetIntType())), false, false)
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
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	empty := &Block{Func: f, Looping: true}
	f.Stack = []*Block{empty}
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomContinue(NewRng(1), opts, vs, NewExprTables(opts), cg, empty)
	if st.Expr != nil {
		t.Fatal("first-stmt continue must not produce expr")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject null continue")
	}
	// non-empty block accepts continue
	empty.Stmts = []Stmt{{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 1}}
	st2 := MakeRandomContinue(NewRng(2), opts, vs, NewExprTables(opts), cg, empty)
	if st2.Kind != StmtContinue || st2.Expr == nil {
		t.Fatalf("%+v", st2)
	}
}
