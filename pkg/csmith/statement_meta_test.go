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

func TestGetBlocksStmtKindGated(t *testing.T) {
	// StatementIf always exposes both arms — nil arm is incomplete hole
	ifBlks := GetBlocksStmt(&Stmt{Kind: StmtIfElse, Then: &Block{StmID: 1}})
	if len(ifBlks) != 2 || ifBlks[0] == nil || ifBlks[1] != nil {
		t.Fatalf("if arms: %+v", ifBlks)
	}
	// missing Else fails typed walk sticky (no invent soft-skip absent arm / empty complete)
	ClearError()
	var stms []*Stmt
	if FindTypedStmts(&Stmt{Kind: StmtIfElse, Then: &Block{Stmts: []Stmt{{Kind: StmtReturn}}}}, &stms, []StatementType{StmtReturn}) >= 0 {
		t.Fatal("nil Else arm must fail closed typed walk")
	}
	if StmtsComplete(stms) {
		t.Fatal("nil Else arm must leave IncompleteStmtsSlice, not invent empty complete", stms)
	}
	if !HasError() {
		t.Fatal("nil Else FindTypedStmts must SetError sticky")
	}
	ClearError()
	// for always pushes body slot
	forBlks := GetBlocksStmt(&Stmt{Kind: StmtFor})
	if len(forBlks) != 1 || forBlks[0] != nil {
		t.Fatalf("for body slot: %+v", forBlks)
	}
	// assign has empty get_blocks even if Then is wrongly set
	if blks := GetBlocksStmt(&Stmt{Kind: StmtAssign, Then: &Block{}}); len(blks) != 0 {
		t.Fatal("assign must not invent get_blocks from stray Then", blks)
	}
}

func TestIs1stStm(t *testing.T) {
	ClearError()
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
	if HasError() {
		t.Fatal("complete Is1stStm must not sticky")
	}
	ClearError()
	if Is1stStm(nil, b) {
		t.Fatal("nil stmt Is1stStm must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil stmt Is1stStm must SetError sticky")
	}
	ClearError()
}

func TestFindContainerAndDominate(t *testing.T) {
	body := &Block{StmID: 20}
	// StatementIf always has both arms
	ifSt := Stmt{Kind: StmtIfElse, StmID: 10, Then: body, Else: &Block{}}
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
	// stray Then on assign is not get_blocks — no invent container/dominate
	stray := &Block{StmID: 99, Parent: outer}
	outer.Stmts[1].Then = stray
	if FindContainerStm(stray) != nil {
		t.Fatal("assign Then must not invent container")
	}
	if Dominate(&outer.Stmts[1], outer, &Stmt{Kind: StmtAssign, StmID: 100}, stray) {
		t.Fatal("assign must not invent dominate via stray Then")
	}
	// Block always live; sticky nil (no invent soft-skip missing container past hole)
	ClearError()
	if FindContainerStm(nil) != nil {
		t.Fatal("nil FindContainerStm must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FindContainerStm must SetError sticky")
	}
	ClearError()
	// root complete nil
	if FindContainerStm(&Block{StmID: 1}) != nil {
		t.Fatal("root FindContainerStm must complete nil")
	}
	if HasError() {
		t.Fatal("root FindContainerStm must not sticky")
	}
	ClearError()
	// incomplete if-arm sticky (no invent soft-continue past nil Else)
	body2 := &Block{StmID: 30}
	outer2 := &Block{Stmts: []Stmt{{Kind: StmtIfElse, StmID: 31, Then: body2, Else: nil}}}
	body2.Parent = outer2
	outer2.Stmts[0].Then = body2
	if FindContainerStm(body2) != nil {
		t.Fatal("nil Else FindContainerStm must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Else FindContainerStm must SetError sticky")
	}
	ClearError()
}

func TestDominateIncompleteStmIDNoInvent(t *testing.T) {
	ClearError()
	// StmID 0 is incomplete IR; orphans not in parent must not invent dominate via 0<=0
	parent := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	orphanA := &Stmt{Kind: StmtAssign, StmID: 0}
	orphanB := &Stmt{Kind: StmtAssign, StmID: 0}
	if Dominate(orphanA, parent, orphanB, parent) {
		t.Fatal("orphan StmID 0 pair must fail closed not dominate")
	}
	if !HasError() {
		t.Fatal("orphan StmID 0 Dominate must SetError sticky")
	}
	ClearError()
	// same-parent members with StmID 0 still order by index
	parent.Stmts = []Stmt{{Kind: StmtAssign, StmID: 0}, {Kind: StmtAssign, StmID: 0}}
	if !Dominate(&parent.Stmts[0], parent, &parent.Stmts[1], parent) {
		t.Fatal("index order must dominate when both in parent")
	}
	if Dominate(&parent.Stmts[1], parent, &parent.Stmts[0], parent) {
		t.Fatal("later must not dominate earlier")
	}
	ClearError()
}

func TestIsJumpTargetFromOtherBlocks(t *testing.T) {
	ClearError()
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
	// nil FM sticky — no invent "not a jump target"
	ClearError()
	if !IsJumpTargetFromOtherBlocks(5, destParent, nil, nil) {
		t.Fatal("nil FM must fail closed jump-target")
	}
	if !HasError() {
		t.Fatal("nil FM IsJumpTarget must SetError sticky")
	}
	// StmID 0 fails closed sticky as jump-target (no invent not-target)
	ClearError()
	if !IsJumpTargetFromOtherBlocks(0, destParent, fm, nil) {
		t.Fatal("StmID 0 must fail closed jump-target")
	}
	if !HasError() {
		t.Fatal("StmID 0 IsJumpTarget must SetError sticky")
	}
	ClearError()
	// Dominate nil sticky
	if Dominate(nil, nil, &Stmt{StmID: 1}, nil) {
		t.Fatal("nil Dominate a must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Dominate a must SetError sticky")
	}
	ClearError()
	// GetBlocksStmt nil sticky IncompleteBlocks
	if BlocksComplete(GetBlocksStmt(nil)) {
		t.Fatal("nil Stmt GetBlocksStmt must fail closed IncompleteBlocks")
	}
	if !HasError() {
		t.Fatal("nil Stmt GetBlocksStmt must SetError sticky")
	}
	ClearError()
}

func TestIsPtrUsedForTestExpr(t *testing.T) {
	// StatementFor::get_exprs → test; ptr in for-test must count (no invent skip)
	ClearError()
	pt := PointerTo(GetIntType())
	pv := CreateVariableScalars("p", pt, false, false)
	test := &Expression{Term: TermVariable, Var: pv, ExprType: pt}
	st := &Stmt{Kind: StmtFor, Loop: &LoopControl{TestExpr: test}, Then: &Block{}}
	if !IsPtrUsed(st) {
		t.Fatal("for-test pointer must be used")
	}
	// incomplete for without TestExpr fails closed sticky as true (IsPtrUsed)
	ClearError()
	st2 := &Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}
	if !IsPtrUsed(st2) {
		t.Fatal("incomplete for must fail closed ptr-used true")
	}
	if !HasError() {
		t.Fatal("incomplete for IsPtrUsed must SetError sticky")
	}
	ClearError()
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
	ClearError()
	inner := Stmt{Kind: StmtAssign, StmID: 2}
	thenB := &Block{Stmts: []Stmt{inner}}
	elseB := &Block{}
	root := &Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB, Else: elseB}
	if !ContainsStmtTree(root, root) {
		t.Fatal("self")
	}
	if !ContainsStmtTree(root, &thenB.Stmts[0]) {
		t.Fatal("nested")
	}
	// assign with stray Then must not invent contains via non-get_blocks Then
	stray := &Stmt{Kind: StmtAssign, StmID: 5, Then: thenB}
	if ContainsStmtTree(stray, &thenB.Stmts[0]) {
		t.Fatal("assign must not invent contains via stray Then")
	}
	// nil if arm sticky fail closed false
	if ContainsStmtTree(&Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB}, &thenB.Stmts[0]) {
		t.Fatal("nil Else must fail closed (no invent membership)")
	}
	if !HasError() {
		t.Fatal("nil Else ContainsStmtTree must SetError sticky")
	}
	ClearError()
	// blockHasStmtIDDeep: Block + live StmID always required sticky
	if blockHasStmtIDDeep(nil, 1) {
		t.Fatal("nil block blockHasStmtIDDeep must fail closed")
	}
	if !HasError() {
		t.Fatal("nil block blockHasStmtIDDeep must SetError sticky")
	}
	ClearError()
	if blockHasStmtIDDeep(thenB, 0) {
		t.Fatal("StmID 0 blockHasStmtIDDeep must fail closed")
	}
	if !HasError() {
		t.Fatal("StmID 0 blockHasStmtIDDeep must SetError sticky")
	}
	ClearError()
	// incomplete nested arm sticky
	bad := &Block{Stmts: []Stmt{{Kind: StmtIfElse, StmID: 9, Then: &Block{StmID: 10}, Else: nil}}}
	if blockHasStmtIDDeep(bad, 10) {
		t.Fatal("incomplete if arm must fail closed not invent nest match")
	}
	if !HasError() {
		t.Fatal("incomplete if arm blockHasStmtIDDeep must SetError sticky")
	}
	ClearError()
}

func TestFindTypedStmtsCompleteStillWorks(t *testing.T) {
	// complete if/then+else still collects nested typed stmts
	thenB := &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 3}}}
	elseB := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 4}}}
	st := &Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB, Else: elseB}
	var stms []*Stmt
	n := FindTypedStmts(st, &stms, []StatementType{StmtReturn})
	if n != 1 || len(stms) != 1 {
		t.Fatal(n, stms)
	}
}

func TestMustReturnIncompleteSticky(t *testing.T) {
	ClearError()
	// if with nil Then sticky not-must-return
	st := Stmt{Kind: StmtIfElse, Then: nil, Else: &Block{}}
	if st.MustReturn() {
		t.Fatal("nil Then MustReturn must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Then MustReturn must SetError sticky")
	}
	ClearError()
}

func TestNeedReturnStmtIncompleteSticky(t *testing.T) {
	ClearError()
	if (*Function)(nil).NeedReturnStmt() {
		t.Fatal("nil Function NeedReturnStmt must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Function NeedReturnStmt must SetError sticky")
	}
	ClearError()
	if !(&Function{Name: "f"}).NeedReturnStmt() {
		t.Fatal("nil ReturnType must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil ReturnType NeedReturnStmt must SetError sticky")
	}
	ClearError()
	if (&Function{Name: "f", ReturnType: GetSimpleType(EVoid)}).NeedReturnStmt() {
		t.Fatal("void must not need return")
	}
	if HasError() {
		t.Fatal("void NeedReturnStmt must not sticky")
	}
	ClearError()
}
