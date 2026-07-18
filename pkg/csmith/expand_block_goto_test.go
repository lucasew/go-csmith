package csmith

import "testing"

func TestBlockContainsStmIDParentChain(t *testing.T) {
	outer := &Block{StmID: 1}
	inner := &Block{Parent: outer, StmID: 2}
	dest := Stmt{Kind: StmtAssign, StmID: 10}
	src := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	inner.Stmts = []Stmt{dest}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: inner}, src}

	if !BlockContainsStmID(inner, 10) {
		t.Fatal("inner should contain dest")
	}
	if BlockContainsStmID(inner, 20) {
		t.Fatal("inner must not contain outer goto")
	}
	if !BlockContainsStmID(outer, 10) || !BlockContainsStmID(outer, 20) {
		t.Fatal("outer contains both")
	}
}

func TestExpandBlockForGotoClimbsParent(t *testing.T) {
	f := &Function{Name: "f"}
	outer := &Block{Func: f, StmID: 1}
	inner := &Block{Func: f, Parent: outer, StmID: 2}
	dest := Stmt{Kind: StmtAssign, StmID: 10}
	src := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	inner.Stmts = []Stmt{dest}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: inner}, src}
	f.Blocks = []*Block{outer}

	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)

	got := ExpandBlockForGoto(inner, cg)
	if got != outer {
		t.Fatalf("want outer, got %v", got)
	}
	// already covers both → no climb
	got2 := ExpandBlockForGoto(outer, cg)
	if got2 != outer {
		t.Fatal(got2)
	}
}

func TestExpandBlockForGotoNilFM(t *testing.T) {
	b := &Block{}
	if ExpandBlockForGoto(b, EmptyCGContext()) != b {
		t.Fatal("no-op without FM")
	}
}

func TestExpandBlockForGotoAssertB(t *testing.T) {
	// VariableSelector.cpp:778 assert(b) — no soft invent root when climb fails
	f := &Function{Name: "f"}
	// dest in orphan block; src in unrelated block (not ancestor)
	destBlk := &Block{Func: f, StmID: 1, Stmts: []Stmt{{Kind: StmtAssign, StmID: 10}}}
	srcBlk := &Block{Func: f, StmID: 2, Stmts: []Stmt{{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}}}
	f.Blocks = []*Block{destBlk, srcBlk}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// climb from destBlk cannot reach src (no parent link)
	if ExpandBlockForGoto(destBlk, cg) != nil {
		t.Fatal("want nil when assert(b) would fire")
	}
}

func TestLowerBlockForVars(t *testing.T) {
	a := CreateVariableScalars("l_a", GetIntType(), false, false)
	b := CreateVariableScalars("l_b", GetIntType(), false, false)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	inner := &Block{LocalVars: []*Variable{a}}
	outer := &Block{LocalVars: []*Variable{a, b}, Parent: nil}
	// vars {a,b}: inner covers only a → remaining {b}; outer covers rest
	blk, rem := LowerBlockForVars([]*Block{inner, outer}, []*Variable{a, b})
	if blk != outer || len(rem) != 0 {
		t.Fatalf("blk=%v rem=%v", blk, rem)
	}
	// only globals → nil
	blk, rem = LowerBlockForVars([]*Block{inner}, []*Variable{g})
	if blk != nil || len(rem) != 1 {
		t.Fatalf("want nil remaining g, got %v %v", blk, rem)
	}
	// first block already covers all
	blk, rem = LowerBlockForVars([]*Block{outer, inner}, []*Variable{a, b})
	if blk != outer || len(rem) != 0 {
		t.Fatalf("first cover: %v %v", blk, rem)
	}
}

func TestGenerateNewParentLocalExpandGoto(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	outer := &Block{Func: f}
	inner := &Block{Func: f, Parent: outer}
	dest := Stmt{Kind: StmtAssign, StmID: AllocStmID()}
	srcID := AllocStmID()
	src := Stmt{Kind: StmtGoto, StmID: srcID, GotoDestStmID: dest.StmID}
	inner.Stmts = []Stmt{dest}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: AllocStmID(), Then: inner}, src}
	f.Blocks = []*Block{outer}
	f.Stack = []*Block{outer, inner}

	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: srcID, DestStmID: dest.StmID}}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)

	beforeOuter := len(outer.LocalVars)
	beforeInner := len(inner.LocalVars)
	v := vs.GenerateNewParentLocal(inner, AccessWrite, cg, GetIntType(), nil, NewRng(3))
	if v == nil {
		t.Fatal("nil var")
	}
	// variable should land on outer (goto expand), not inner
	if len(outer.LocalVars) != beforeOuter+1 {
		t.Fatalf("outer locals %d want %d (var=%s inner=%d)",
			len(outer.LocalVars), beforeOuter+1, v.Name, len(inner.LocalVars)-beforeInner)
	}
	if !IsVariableInSet(outer.LocalVars, v) {
		t.Fatal("var not on outer")
	}
}

func TestGenerateNewParentLocalVolatileAggGlobal(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// volatile field → IsVolatileStructUnion
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{
				Name:     "f0",
				Type:     GetIntType(),
				BitWidth: -1,
				Qfer:     NewCVQualifiers([]bool{false}, []bool{true}), // volatile
			},
		},
	}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	beforeG := len(vs.GlobalList)
	v := vs.GenerateNewParentLocal(blk, AccessRead, cg, st, nil, NewRng(1))
	if v == nil {
		t.Fatal("nil")
	}
	if len(vs.GlobalList) != beforeG+1 {
		t.Fatalf("want global, got locals=%d globals=%d", len(blk.LocalVars), len(vs.GlobalList))
	}
	if IsVariableInSet(blk.LocalVars, v) {
		t.Fatal("must not be local")
	}
}
