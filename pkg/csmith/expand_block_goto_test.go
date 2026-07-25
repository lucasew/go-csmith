package csmith

import "testing"

func TestBlockContainsStmIDParentChain(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	outer := &Block{StmID: 1}
	inner := &Block{Parent: outer, StmID: 2}
	dest := Stmt{Kind: StmtAssign, StmID: 10}
	src := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	inner.Stmts = []Stmt{dest}
	// StatementIf always has both arms
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: inner, Else: &Block{}}, src}

	if !BlockContainsStmID(inner, 10) {
		t.Fatal("inner should contain dest")
	}
	if BlockContainsStmID(inner, 20) {
		t.Fatal("inner must not contain outer goto")
	}
	if !BlockContainsStmID(outer, 10) || !BlockContainsStmID(outer, 20) {
		t.Fatal("outer contains both")
	}
	// incomplete if-arm sticky (no invent soft-continue past nil Else then miss Then)
	ClearErrorSess(testAmbientSession)
	outer.Stmts[0].Else = nil
	if BlockContainsStmID(outer, 10) {
		t.Fatal("nil Else must fail closed not-contain (sticky miss whole search)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Else BlockContainsStmID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpandBlockForGotoClimbsParent(t *testing.T) {
	f := &Function{Name: "f"}
	outer := &Block{Func: f, StmID: 1}
	inner := &Block{Func: f, Parent: outer, StmID: 2}
	dest := Stmt{Kind: StmtAssign, StmID: 10}
	src := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	inner.Stmts = []Stmt{dest}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: inner, Else: &Block{}}, src}
	f.Blocks = []*Block{outer}

	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)

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
	if ExpandBlockForGoto(b, EmptyCGContext().WithSession(testAmbientSession)) != b {
		t.Fatal("no-op without FM")
	}
	// Block always live; sticky no invent soft-skip expand past hole
	ClearErrorSess(testAmbientSession)
	if ExpandBlockForGoto(nil, EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("nil ExpandBlockForGoto must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ExpandBlockForGoto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpandBlockForGotoAssertB(t *testing.T) {
	// Live tree: goto in sibling arm so climb from then must reach outer.
	// VariableSelector.cpp:773–779 expand then stop when src is covered.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	outer := &Block{Func: f, StmID: 1}
	thenB := &Block{Func: f, Parent: outer, StmID: 2, Stmts: []Stmt{{Kind: StmtAssign, StmID: 10}}}
	elseB := &Block{Func: f, Parent: outer, StmID: 3, Stmts: []Stmt{{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}}}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: thenB, Else: elseB}}
	f.Blocks = []*Block{outer}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	got := ExpandBlockForGoto(thenB, cg)
	if got != outer {
		t.Fatalf("climb must reach outer for sibling goto: got %#v sticky=%v", got, HasErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpandBlockForGotoNilCFGHole(t *testing.T) {
	// CFGEdge* always live; nil hole must not invent skip as absent edge
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	outer := &Block{Func: f, StmID: 1}
	inner := &Block{Func: f, Parent: outer, StmID: 2}
	inner.Stmts = []Stmt{{Kind: StmtAssign, StmID: 10}}
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: 5, Then: inner, Else: &Block{}}, {Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}}
	f.Blocks = []*Block{outer}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{nil, {SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if ExpandBlockForGoto(inner, cg) != nil {
		t.Fatal("nil CFG hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CFG hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpandBlockForGotoFindStmtResidualSticky(t *testing.T) {
	// FindStmt residual: incomplete if-arm on sole Blocks path stickies ERROR.
	// Soft invent was soft-continue expand past residual then climb later complete edge.
	// Fair: sticky fail closed nil ExpandBlockForGoto.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	outer := &Block{Func: f, StmID: 1}
	inner := &Block{Func: f, Parent: outer, StmID: 2}
	inner.Stmts = []Stmt{{Kind: StmtAssign, StmID: 10}}
	// incomplete if (nil Else) — FindStmt residual when walking Blocks
	outer.Stmts = []Stmt{
		{Kind: StmtIfElse, StmID: 5, Then: inner, Else: nil},
		{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10},
	}
	f.Blocks = []*Block{outer}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if ExpandBlockForGoto(inner, cg) != nil {
		t.Fatal("FindStmt residual must fail closed ExpandBlockForGoto, not invent later climb")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindStmt residual ExpandBlockForGoto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestExpandBlockForGotoMidGenUnlinkedThenArm — seed-62 body parity.
// StatementIf builds then fully before StmtIfElse is linked under parent.
// Forward goto may already sit on parent; dest is first then-stmt. Expand during
// then-arm GenerateNewParentLocal must climb via Statement::parent chain
// (Statement.cpp:689–696), not root GetBlocksStmt tree membership only.
func TestExpandBlockForGotoMidGenUnlinkedThenArm(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	// parent already has a forward goto; then-arm is NOT yet a child of any IfElse
	parent := &Block{Func: f, StmID: 298}
	thenArm := &Block{Func: f, Parent: parent, StmID: 322}
	dest := Stmt{Kind: StmtAssign, StmID: 323, SourceLabel: "lbl_x"}
	thenArm.Stmts = []Stmt{dest}
	// goto lives on parent; thenArm only Parent-linked (mid MakeRandomIf)
	gotoSt := Stmt{Kind: StmtGoto, StmID: 324, Label: "lbl_x", GotoDestStmID: 323, GotoForward: true}
	parent.Stmts = []Stmt{{Kind: StmtFor, StmID: 304, Then: &Block{Func: f, Parent: parent, StmID: 301}}, gotoSt}
	// Func.Blocks lists both; root tree under a fake body does not include thenArm
	body := &Block{Func: f, StmID: 16, Stmts: []Stmt{{Kind: StmtIfElse, StmID: 20, Then: parent, Else: &Block{Func: f, StmID: 21}}}}
	f.Blocks = []*Block{body, parent, thenArm}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 324, DestStmID: 323, DestBlock: thenArm, BackLink: false}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)

	// contains_stmt: dest under thenArm via parent chain (not root tree alone)
	if !BlockContainsStmID(thenArm, 323) {
		t.Fatal("thenArm must contain dest mid-gen")
	}
	if BlockContainsStmID(thenArm, 324) {
		t.Fatal("thenArm must not contain parent goto")
	}
	if !BlockContainsStmID(parent, 324) {
		t.Fatal("parent must contain goto")
	}

	got := ExpandBlockForGoto(thenArm, cg)
	if got != parent {
		t.Fatalf("mid-gen expand must climb thenArm->parent, got %#v sticky=%v", got, HasErrorSess(testAmbientSession))
	}
	// Create local on thenArm must land on parent after expand
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f.Stack = []*Block{parent, thenArm}
	beforeP, beforeT := len(parent.LocalVars), len(thenArm.LocalVars)
	v := vs.GenerateNewParentLocal(thenArm, AccessWrite, cg, GetIntType(), nil, NewRng(5))
	if v == nil {
		t.Fatal("nil var", HasErrorSess(testAmbientSession))
	}
	if len(parent.LocalVars) != beforeP+1 || !IsVariableInSet(parent.LocalVars, v) {
		t.Fatalf("var must land on parent: parent=%d then=%d v=%s",
			len(parent.LocalVars)-beforeP, len(thenArm.LocalVars)-beforeT, v.Name)
	}
	ClearErrorSess(testAmbientSession)
}

// TestExpandBlockForGotoSkipsOrphanGotoEdges — aborted make_random leaves Blocks
// on Func.Blocks with CFGEdges to live dests (C++ delete frees IR; edges gone).
// Expand must not climb-fail on those orphans (seed-2 e15453 GenerateNewParentLocal).
func TestExpandBlockForGotoSkipsOrphanGotoEdges(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	body := &Block{Func: f, StmID: 22}
	// live dest statement in body
	body.Stmts = []Stmt{{Kind: StmtAssign, StmID: 153}}
	// orphan block not linked under body (abort left it on Func.Blocks only)
	orphan := &Block{Func: f, StmID: 99}
	orphan.Stmts = []Stmt{{Kind: StmtGoto, StmID: 210, GotoDestStmID: 153}}
	f.Blocks = []*Block{body, orphan}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 210, DestStmID: 153}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// Without orphan skip, climb from body for src=210 fails (src not in live tree).
	got := ExpandBlockForGoto(body, cg)
	if got != body {
		t.Fatalf("orphan goto edge must be skipped; want body, got %#v sticky=%v", got, HasErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("orphan skip must not SetError sticky")
	}
}

func TestLowerBlockForVarsLocalVarsHoleFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsVariableInSet false → var stays remaining
	// fair: incomplete LocalVars → nil, IncompleteVariables sticky
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "l_a", GetIntType(), false, false)
	a.Name = "l_a"
	inner := &Block{LocalVars: []*Variable{a, nil}}
	blk, rem := LowerBlockForVars([]*Block{inner}, []*Variable{a})
	if blk != nil || VariablesComplete(rem) {
		t.Fatal("incomplete LocalVars must fail closed incomplete remaining", blk, rem)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete LocalVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLowerBlockForVars(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "l_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "l_b", GetIntType(), false, false)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
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
	// nil block hole fails closed sticky
	blk, rem = LowerBlockForVars([]*Block{nil, outer}, []*Variable{a, b})
	if blk != nil || VariablesComplete(rem) {
		t.Fatal("nil block hole must fail closed incomplete remaining", blk, rem)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil block hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	outer.Stmts = []Stmt{{Kind: StmtIfElse, StmID: AllocStmID(), Then: inner, Else: &Block{}}, src}
	f.Blocks = []*Block{outer}
	f.Stack = []*Block{outer, inner}

	fm := NewFactMgrSess(testAmbientSession, f)
	fm.CFGEdges = []*CFGEdge{{SrcID: srcID, DestStmID: dest.StmID}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)

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
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
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

func TestReachMaxFunctionsNilFuncResidualSticky(t *testing.T) {
	// nil Func soft invent was invent room for more past incomplete Funcs hole.
	// Fair: fail closed max-reached true, non-sticky (soft re-pick factories).
	ClearErrorSess(testAmbientSession)
	list := &FunctionList{Funcs: []*Function{nil}}
	opts := Defaults()
	opts.MaxFuncs = 10
	if !ReachMaxFunctions(list, opts) {
		t.Fatal("nil Func hole must fail closed max-reached true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil Func hole ReachMaxFunctions must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete under max
	list2 := &FunctionList{Funcs: []*Function{
		{Name: "func_1", ReturnType: GetIntType()},
	}}
	if ReachMaxFunctions(list2, opts) {
		t.Fatal("one user func under max must not invent max")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete ReachMaxFunctions must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
