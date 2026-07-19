package csmith

import (
	"testing"
)

func TestGetExternalNoReadsWrites(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{
		NoReadVars:  []*Variable{g, loc},
		NoWriteVars: []*Variable{g},
	})
	// without frame, only globals
	nr, nw := cg.GetExternalNoReadsWrites(nil)
	if len(nr) != 1 || nr[0] != g {
		t.Fatal("nr", nr)
	}
	if len(nw) != 1 || nw[0] != g {
		t.Fatal("nw", nw)
	}
	// with frame includes local
	nr, nw = cg.GetExternalNoReadsWrites([]*Variable{loc})
	if len(nr) != 2 {
		t.Fatal("frame nr", nr)
	}
	_ = nw
	// nil RW hole fails closed incomplete (not bare nil invent empty complete)
	cg2 := EmptyCGContext().WithRW(&RWDirective{NoReadVars: []*Variable{nil, g}})
	nr, nw = cg2.GetExternalNoReadsWrites(nil)
	if VariablesComplete(nr) || VariablesComplete(nw) {
		t.Fatal("nil NoReadVars hole must fail closed incomplete", nr, nw)
	}
}

func TestFindMustUseArraysNilHole(t *testing.T) {
	rw := &RWDirective{MustReadVars: []*Variable{nil}}
	if rw.FindMustUseArrays() != nil {
		t.Fatal("nil must-use hole must fail closed")
	}
}

func TestFindRelatedPointToNilHole(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{nil, MakeFactPointTo(p, NullPtr)}
	if FindRelatedPointTo(facts, p) != nil {
		t.Fatal("nil fact hole must fail closed (no invent skip to later)")
	}
	if FindRelatedUnion([]*FactUnion{nil}, p) != nil {
		t.Fatal("nil union fact hole must fail closed")
	}
}

func TestPtrModifiedInRhsNilPointees(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	lhs := &Lhs{Var: p, Type: GetIntType()} // indir via type peel — set IndirectLevel path
	// force multi-level via Type pointer chain
	pt := PointerTo(PointerTo(GetIntType()))
	pp := CreateVariableScalars("g_pp", pt, true, false)
	lhs2 := &Lhs{Var: pp, Type: GetIntType()}
	// IndirectLevel for Lhs
	if lhs2.IndirectLevel() <= 1 {
		// still exercise MergePointees nil path via incomplete facts with hole
		_ = lhs
	}
	cg := EmptyCGContext()
	// incomplete merge via nil in pointees of related fact
	facts := []*FactPointTo{{Var: pp, PointTo: []*Variable{nil}}}
	// for indir > 1 need multi-level pointer
	// Lhs IndirectLevel = var.Type.IndirectLevel - want.IndirectLevel
	// pp is **int, Type GetIntType → indir 2
	if lhs2.IndirectLevel() > 1 {
		if !cg.PtrModifiedInRhs(lhs2, facts) {
			t.Fatal("nil pointee hole must fail closed as modified")
		}
	}
}

func TestGetExternalNoWritesFromIV(t *testing.T) {
	g := CreateVariableScalars("g_i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.AddIVBound(g, 10)
	_, nw := cg.GetExternalNoReadsWrites(nil)
	if len(nw) != 1 || nw[0] != g {
		t.Fatal(nw)
	}
}

func TestBuildCalleeRWDirective(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	rwd := cg.BuildCalleeRWDirective(nil)
	if rwd == nil || len(rwd.NoWriteVars) != 1 {
		t.Fatal(rwd)
	}
}

func TestFindReachableFrameVarsCompleteEmpty(t *testing.T) {
	// complete empty must be non-nil empty (not invent nil==incomplete)
	cg := EmptyCGContext()
	got := cg.FindReachableFrameVars(nil)
	if got == nil {
		t.Fatal("complete empty must be non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatal(got)
	}
	// incomplete fact map fails closed nil
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	if VariablesComplete(cg.FindReachableFrameVars([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil})) {
		t.Fatal("incomplete facts must fail closed incomplete")
	}
}

func TestBuildCalleeRWDirectiveIncompleteFactsFailClosed(t *testing.T) {
	// soft invent: incomplete frame → nil RW (no restrictions)
	// fair: inherit full NoWrite without inventing unrestricted nil; sticky
	ClearError()
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rwd := cg.BuildCalleeRWDirective([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil})
	if rwd == nil {
		t.Fatal("incomplete must not invent nil unrestricted RW")
	}
	if len(rwd.NoWriteVars) != 1 || rwd.NoWriteVars[0] != g {
		t.Fatal(rwd)
	}
	if !HasError() {
		t.Fatal("incomplete frame facts must SetError sticky")
	}
	ClearError()
}

func TestVisitFactsInvocationParams(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermVariable, Var: a, ExprType: GetIntType()},
			{Term: TermVariable, Var: b, ExprType: GetIntType()},
		},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsRead(a) || !eff.IsRead(b) {
		t.Fatal("reads")
	}
}

func TestVisitFactsInvocationConflict(t *testing.T) {
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	callee.FEffect = EmptyEffect().WriteVar(g)
	fi := &Invocation{User: callee}
	// context already wrote g
	cg := WithEffectContext(EmptyEffect().WriteVar(g))
	if VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatal("should conflict")
	}
}

func TestFactMgrMapStmEffect(t *testing.T) {
	fm := NewFactMgr(nil)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	eff := EmptyEffect().WriteVar(v)
	fm.SetMapStmEffect(3, eff)
	got := fm.GetMapStmEffect(3)
	if !got.IsWritten(v) {
		t.Fatal("map")
	}
	if fm.GetMapStmEffect(9).IsWritten(v) {
		t.Fatal("empty")
	}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.SetMapFactsIn(1, facts)
	fm.SetMapFactsOut(1, facts)
	if len(fm.MapFactsIn[1]) != 1 || len(fm.MapFactsOut[1]) != 1 {
		t.Fatal("facts maps")
	}
}

func TestVisitFactsBlockRecordsMaps(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := Stmt{
		Kind: StmtAssign, StmID: 7, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	// Block::stm_id always live when FM bound
	b := &Block{StmID: 5, Stmts: []Stmt{st}}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsBlock(b, &cg, Defaults()) {
		t.Fatal("block")
	}
	if _, ok := fm.MapFactsIn[7]; !ok {
		t.Fatal("facts_in")
	}
	if _, ok := fm.MapFactsOut[7]; !ok {
		t.Fatal("facts_out")
	}
}
