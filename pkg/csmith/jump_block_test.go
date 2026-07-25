package csmith

import (
	"strings"
	"testing"
)

func TestHasInitSkippedVars(t *testing.T) {
	outer := &Block{}
	inner := &Block{Parent: outer}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	inner.LocalVars = []*Variable{loc}
	// jump from outer into inner skips l_1
	if !HasInitSkippedVars(outer, inner) {
		t.Fatal("skip")
	}
	// same block no skip
	if HasInitSkippedVars(inner, inner) {
		t.Fatal("same")
	}
	// src nil sticky has-skipped (no invent none soft-skip past hole)
	ClearErrorSess(testAmbientSession)
	if !HasInitSkippedVars(nil, inner) {
		t.Fatal("nil src HasInitSkippedVars must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil src HasInitSkippedVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// destParent nil complete false
	if HasInitSkippedVars(outer, nil) {
		t.Fatal("nil dest complete false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil dest HasInitSkippedVars must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindGoodJumpBlock(t *testing.T) {
	f := &Function{Name: "f"}
	b1 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	b2 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 2}}}
	f.Blocks = []*Block{b1, b2}
	got := FindGoodJumpBlock(NewRng(2), f.Blocks, b1, true)
	if got == nil {
		t.Fatal("nil")
	}
	// empty blocks rejected
	empty := &Block{Func: f}
	if FindGoodJumpBlock(NewRng(2), []*Block{empty}, b1, true) != nil {
		t.Fatal("empty")
	}
	// array loop dest rejected
	arr := &Block{Func: f, InArrayLoop: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	if FindGoodJumpBlock(NewRng(2), []*Block{arr}, b1, true) != nil {
		t.Fatal("array dest")
	}
}

func TestFindGoodJumpBlockResidualSticky(t *testing.T) {
	// HasInitSkippedVars residual ERROR soft invent was soft-continue then pick later block.
	// Fair: sticky fail closed whole FindGoodJumpBlock.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	src := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	// hole in LocalVars stickies CollectInitSkippedVars residual when climbed as dest parent
	hole := &Block{Func: f, Parent: src, LocalVars: []*Variable{nil}, Stmts: []Stmt{{Kind: StmtAssign, StmID: 2}}}
	good := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	// asDest: HasInitSkippedVars(src, hole) stickies residual; soft invent picks good
	if FindGoodJumpBlock(NewRng(1), []*Block{hole, good}, src, true) != nil {
		t.Fatal("HasInitSkippedVars residual must fail closed FindGoodJumpBlock")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasInitSkippedVars residual FindGoodJumpBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// curr last MustReturn residual soft invent was soft-continue allow dest invent pick.
	// Fair: sticky fail closed whole FindGoodJumpBlock (asDest).
	// last if incomplete residual MustReturn false; without sticky re-pick continues to good.
	curr := &Block{
		Func:  f,
		Stmts: []Stmt{{Kind: StmtIfElse, Then: nil, Else: &Block{}}},
	}
	good2 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 4}}}
	if FindGoodJumpBlock(NewRng(1), []*Block{good2}, curr, true) != nil {
		t.Fatal("MustReturn residual must fail closed FindGoodJumpBlock asDest")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MustReturn residual FindGoodJumpBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCollectInitSkippedVarsIsVisibleResidualSticky(t *testing.T) {
	// IsVisibleLocal residual soft invent was treat not-visible then complete skip list.
	// Fair: sticky IncompleteVariables fail closed.
	// When src is not an ancestor of dest, path uses !IsVisibleLocal(src) for each intermediate.
	ClearErrorSess(testAmbientSession)
	src := &Block{}
	src.LocalVars = []*Variable{nil}                  // incomplete: IsVisibleLocal hits nil hole
	mid := &Block{}                                   // no Parent chain to src → !reachedSrc
	loc := &Variable{Name: "l_x", Type: GetIntType()} // non-global local
	mid.LocalVars = []*Variable{loc}
	got := CollectInitSkippedVars(src, mid)
	if VariablesComplete(got) {
		t.Fatal("IsVisibleLocal residual must fail closed incomplete skip list")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVisibleLocal residual CollectInitSkippedVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputPtrResetsArray(t *testing.T) {
	// OutputMgr.cpp:326–340 — get_last_ctrl_vars + output_init(&zero); no invent "0"
	CtrlVarsDoFinalization()
	opts := Defaults()
	// ArrayVariable.cpp:179 — pointer alt-inits need make_init_value; strict_const
	// uses Constant::make_random so library test can omit VS/CGContext.
	opts.StrictConstArrays = true
	_ = GetNewCtrlVars(opts) // OutputMgr.cpp: get_last_ctrl_vars after array inits
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", PointerTo(GetIntType()), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("av")
	}
	out := OutputPtrResets([]*Variable{&av.Variable}, opts)
	if !strings.Contains(out, "g_a") || !strings.Contains(out, " = 0;") || !strings.Contains(out, "for (i = 0") {
		t.Fatal(out)
	}
	// ArrayVariable.cpp:649 — missing init sticky fail closed (no invent "0" shell)
	ClearErrorSess(testAmbientSession)
	av2 := &ArrayVariable{
		Variable: Variable{Name: "g_b", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av2.AsArray = av2
	if got := outputArrayInitForced(av2, "    ", []string{"i"}, true); got != "" {
		t.Fatalf("nil init must fail closed, got %q", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil init outputArrayInitForced must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// post_incr_operator false → "i = i + 1" (ArrayVariable.cpp:640–645)
	opts.PostIncrOperator = false
	out2 := OutputPtrResets([]*Variable{&av.Variable}, opts)
	if !strings.Contains(out2, "i = i + 1") {
		t.Fatal(out2)
	}
	CtrlVarsDoFinalization()
}

func TestClearEffectStm(t *testing.T) {
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, CreateVariableScalars("g_x", GetIntType(), false, false))
	cg.ClearEffectStm()
	if !cg.EffectStm.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("clear")
	}
}

func TestGotoUsesFindGoodJumpBlock(t *testing.T) {
	// StatementGoto.cpp:117–132 — cond from choose_visible_read_var only
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	// two stmts so dest (last) != other candidate
	b1 := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: AllocStmID()},
		{Kind: StmtAssign, StmID: AllocStmID()},
	}}
	f.Blocks = []*Block{b1}
	f.Stack = []*Block{b1}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg.EffectAccum = &eff
	for i := range b1.Stmts {
		fm.MapAccumEffect[b1.Stmts[i].StmID] = eff
		fm.SetMapFactsIn(b1.Stmts[i].StmID, nil)
		fm.SetMapFactsOut(b1.Stmts[i].StmID, nil)
	}
	var st Stmt
	for seed := uint64(1); seed < 40; seed++ {
		st = MakeRandomGoto(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b1)
		if st.GotoBack || (st.Label == "" && !stmtOK(st)) {
			// success back-edge or fair null after forward insert
			if st.GotoBack {
				break
			}
		}
	}
	if st.Kind != StmtGoto {
		t.Fatal(st.Kind)
	}
}
