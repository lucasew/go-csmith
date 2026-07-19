package csmith

import "testing"

func TestGetBlkDepthAndInBlock(t *testing.T) {
	outer := &Block{}
	inner := &Block{Parent: outer}
	if GetBlkDepth(inner) != 2 {
		t.Fatal(GetBlkDepth(inner))
	}
	if !StmtInBlock(inner, outer) {
		t.Fatal("inner in outer")
	}
	if StmtInBlock(outer, inner) {
		t.Fatal("outer not in inner")
	}
}

func TestFindTypedStmts(t *testing.T) {
	// Statement.cpp:631–646
	ret := Stmt{Kind: StmtReturn, StmID: 3}
	assign := Stmt{Kind: StmtAssign, StmID: 2}
	gotos := Stmt{Kind: StmtGoto, StmID: 4}
	inner := Stmt{
		Kind: StmtIfElse, StmID: 1,
		Then: &Block{Stmts: []Stmt{assign, ret}},
		Else: &Block{Stmts: []Stmt{gotos}},
	}
	var stms []*Stmt
	n := FindTypedStmts(&inner, &stms, []StatementType{StmtReturn, StmtGoto})
	if n != 2 {
		t.Fatalf("count %d stms=%v", n, stms)
	}
	kinds := map[StatementType]bool{}
	for _, s := range stms {
		kinds[s.Kind] = true
	}
	if !kinds[StmtReturn] || !kinds[StmtGoto] {
		t.Fatal(kinds)
	}
	// block walk
	var stms2 []*Stmt
	FindTypedStmtsInBlock(&Block{Stmts: []Stmt{inner}}, &stms2, []StatementType{StmtAssign})
	if len(stms2) != 1 || stms2[0].Kind != StmtAssign {
		t.Fatal(stms2)
	}
}

func TestIs1stStm(t *testing.T) {
	b := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtAssign, StmID: 2},
	}}
	if !Is1stStm(&b.Stmts[0], b) {
		t.Fatal("first")
	}
	if Is1stStm(&b.Stmts[1], b) {
		t.Fatal("second")
	}
}

func TestFindContainerAndDominate(t *testing.T) {
	body := &Block{StmID: 20}
	ifSt := Stmt{Kind: StmtIfElse, StmID: 10, Then: body}
	outer := &Block{Stmts: []Stmt{ifSt, {Kind: StmtAssign, StmID: 11}}}
	body.Parent = outer
	// fix Then pointer to outer.Stmts[0].Then
	outer.Stmts[0].Then = body
	c := FindContainerStm(body)
	if c == nil || c.StmID != 10 {
		t.Fatal(c)
	}
	// if dominates stmt inside then
	inner := &Stmt{Kind: StmtAssign, StmID: 21}
	body.Stmts = []Stmt{*inner}
	if !Dominate(&outer.Stmts[0], outer, &body.Stmts[0], body) {
		t.Fatal("if dominates then body stmt")
	}
	// earlier dominates later same block
	if !Dominate(&outer.Stmts[0], outer, &outer.Stmts[1], outer) {
		t.Fatal("10 dominates 11")
	}
	if Dominate(&outer.Stmts[1], outer, &outer.Stmts[0], outer) {
		t.Fatal("11 does not dominate 10")
	}
}

func TestDominateIncompleteStmIDNoInvent(t *testing.T) {
	// StmID 0 is incomplete IR; orphans not in parent must not invent dominate via 0<=0
	parent := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	orphanA := &Stmt{Kind: StmtAssign, StmID: 0}
	orphanB := &Stmt{Kind: StmtAssign, StmID: 0}
	if Dominate(orphanA, parent, orphanB, parent) {
		t.Fatal("orphan StmID 0 pair must fail closed not dominate")
	}
	// same-parent members with StmID 0 still order by index
	parent.Stmts = []Stmt{{Kind: StmtAssign, StmID: 0}, {Kind: StmtAssign, StmID: 0}}
	if !Dominate(&parent.Stmts[0], parent, &parent.Stmts[1], parent) {
		t.Fatal("index order must dominate when both in parent")
	}
	if Dominate(&parent.Stmts[1], parent, &parent.Stmts[0], parent) {
		t.Fatal("later must not dominate earlier")
	}
}

func TestIsJumpTargetFromOtherBlocks(t *testing.T) {
	fm := NewFactMgr(nil)
	destParent := &Block{Stmts: []Stmt{{StmID: 5}}}
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestStmID: 5}}
	if !IsJumpTargetFromOtherBlocks(5, destParent, fm, nil) {
		t.Fatal("external goto")
	}
	// sibling source
	destParent.Stmts = append(destParent.Stmts, Stmt{StmID: 99, Kind: StmtGoto})
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestStmID: 5}}
	if IsJumpTargetFromOtherBlocks(5, destParent, fm, nil) {
		t.Fatal("sibling not other block")
	}
	// nil FM — no invent "not a jump target"
	if !IsJumpTargetFromOtherBlocks(5, destParent, nil, nil) {
		t.Fatal("nil FM must fail closed jump-target")
	}
}

func TestIsPtrUsedForTestExpr(t *testing.T) {
	// StatementFor::get_exprs → test; ptr in for-test must count (no invent skip)
	pt := PointerTo(GetIntType())
	pv := CreateVariableScalars("p", pt, false, false)
	test := &Expression{Term: TermVariable, Var: pv, ExprType: pt}
	st := &Stmt{Kind: StmtFor, Loop: &LoopControl{TestExpr: test}, Then: &Block{}}
	if !IsPtrUsed(st) {
		t.Fatal("for-test pointer must be used")
	}
	// incomplete for without TestExpr fails closed as true (IsPtrUsed)
	st2 := &Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}
	if !IsPtrUsed(st2) {
		t.Fatal("incomplete for must fail closed ptr-used true")
	}
}

func TestIsPtrUsed(t *testing.T) {
	p := CreateVariableScalars("p", PointerTo(GetIntType()), false, false)
	st := &Stmt{Kind: StmtAssign, LhsVar: p, Expr: &Expression{Term: TermVariable, Var: p}}
	if !IsPtrUsed(st) {
		t.Fatal("ptr")
	}
	st2 := &Stmt{Kind: StmtAssign, LhsVar: CreateVariableScalars("i", GetIntType(), false, false),
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	if IsPtrUsed(st2) {
		t.Fatal("no ptr")
	}
}

func TestContainsStmtTree(t *testing.T) {
	inner := Stmt{Kind: StmtAssign, StmID: 2}
	thenB := &Block{Stmts: []Stmt{inner}}
	root := &Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB}
	if !ContainsStmtTree(root, root) {
		t.Fatal("self")
	}
	if !ContainsStmtTree(root, &thenB.Stmts[0]) {
		t.Fatal("nested")
	}
}

func TestFindTypedStmtsCompleteStillWorks(t *testing.T) {
	// complete if/then still collects nested typed stmts
	thenB := &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 3}}}
	st := &Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB}
	var stms []*Stmt
	n := FindTypedStmts(st, &stms, []StatementType{StmtReturn})
	if n != 1 || len(stms) != 1 {
		t.Fatal(n, stms)
	}
}
