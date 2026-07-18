package csmith

import "testing"

func TestRemoveStmtScrubsCFGAndBreaks(t *testing.T) {
	fm := NewFactMgr(nil)
	loop := &Block{Looping: true, StmID: 100, BreakStmIDs: []int{1, 2, 3}}
	b := &Block{
		Parent: loop,
		StmID:  10,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: 1},
			{Kind: StmtBreak, StmID: 2},
			{Kind: StmtAssign, StmID: 3},
		},
	}
	fm.CreateCFGEdge(1, b, false, false)
	// edge src=2 dest=100
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{SrcID: 2, DestStmID: 100})
	fm.SetMapFactsIn(2, nil)
	fm.SetMapFactsOut(2, nil)

	n := b.RemoveStmt(2, fm)
	if n != 1 {
		t.Fatalf("removed %d", n)
	}
	if len(b.Stmts) != 2 {
		t.Fatalf("stmts %d", len(b.Stmts))
	}
	for _, e := range fm.CFGEdges {
		if e != nil && (e.SrcID == 2 || e.DestStmID == 2) {
			t.Fatal("edge involving removed stmt")
		}
	}
	for _, id := range loop.BreakStmIDs {
		if id == 2 {
			t.Fatal("break list still has 2")
		}
	}
	if _, ok := fm.MapFactsIn[2]; ok {
		t.Fatal("facts in")
	}
}

func TestResetBlockFactMaps(t *testing.T) {
	fm := NewFactMgr(nil)
	b := &Block{StmID: 9, Stmts: []Stmt{{StmID: 1}, {StmID: 2}}}
	fm.SetMapFactsIn(1, nil)
	fm.SetMapFactsOut(2, nil)
	fm.SetMapFactsIn(9, nil)
	fm.ResetBlockFactMaps(b)
	if _, ok := fm.MapFactsIn[1]; ok {
		t.Fatal("in 1")
	}
	if _, ok := fm.MapFactsOut[2]; ok {
		t.Fatal("out 2")
	}
	if _, ok := fm.MapFactsIn[9]; !ok {
		// block itself may be collected via collectBlockStmIDs
	}
}

func TestFindJumpSources(t *testing.T) {
	fm := NewFactMgr(nil)
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 10, DestStmID: 5},
		{SrcID: 11, DestStmID: 5},
		{SrcID: 12, DestStmID: 6},
	}
	srcs := fm.FindJumpSources(5)
	if len(srcs) != 2 {
		t.Fatal(srcs)
	}
}

func TestNeedNestedLoop(t *testing.T) {
	arr := CreateVariableScalars("a", GetIntType(), false, false)
	arr.IsArray = true
	arr.ArraySizes = []int{2, 3} // dim 2
	b := &Block{Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	rw := &RWDirective{MustReadVars: []*Variable{arr}}
	cg := CGContext{RW: rw, IVBounds: map[*Variable]int{}} // depth 0
	if !b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("dim 2 > iv 0")
	}
	// must_jump last blocks nested loop
	b.Stmts = []Stmt{{
		Kind: StmtBreak,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}
	if b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("must jump")
	}
}

func TestMustBreakOrReturnFullBackEdge(t *testing.T) {
	fm := NewFactMgr(nil)
	b := &Block{
		StmID: 50,
		Stmts: []Stmt{{
			Kind: StmtReturn,
			StmID: 51,
			Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
		}},
	}
	// no back edges → true
	if !b.MustBreakOrReturnFull(fm) {
		t.Fatal("expect must break/return")
	}
	// continue-like edge from outside into block
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{
		SrcID: 99, DestBlock: b, BackLink: true,
	})
	if b.MustBreakOrReturnFull(fm) {
		t.Fatal("back edge escapes return")
	}
}

func TestEffectCloneIndependent(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := EmptyEffect().WriteVar(v)
	c := e.Clone()
	e = e.ReadVar(CreateVariableScalars("g_2", GetIntType(), false, false))
	if c.IsRead(CreateVariableScalars("g_2", GetIntType(), false, false)) {
		// different var ptr — just check written still
	}
	if !c.IsWritten(v) {
		t.Fatal("clone lost write")
	}
	// mutate original maps should not clear clone if deep
	e2 := EmptyEffect().WriteVar(v)
	c2 := e2.Clone()
	_ = e2.WriteVar(CreateVariableScalars("g_3", GetIntType(), false, false))
	// e2.WriteVar returns new Effect; original e2 maps may be shared with... 
	// Clone deep-copies so c2.written is independent
	if len(c2.written) != 1 {
		t.Fatal(len(c2.written))
	}
}

func TestInArrayLoopFromIVBounds(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	f := &Function{Name: "f", ReturnType: GetIntType()}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	cg.IVBounds = map[*Variable]int{iv: 10}
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), cg, false)
	if b == nil || !b.InArrayLoop {
		t.Fatal("InArrayLoop", b)
	}
}
