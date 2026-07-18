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
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := Stmt{
		Kind: StmtAssign, StmID: 7, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	b := &Block{Stmts: []Stmt{st}}
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
