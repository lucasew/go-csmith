package csmith

import (
	"testing"
)

func TestMergeJumpFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	jump := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if !MergeJumpFacts(&facts, jump) {
		t.Fatal("changed")
	}
	fp := FindRelatedPointTo(facts, p)
	// joined set should include both
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal(fp)
	}
}

func TestMergeJumpFactsMissingIsGarbage(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	// jump has no fact for p → garbage join
	if !MergeJumpFacts(&facts, nil) {
		t.Fatal("expect garbage merge")
	}
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || !fp.IsDead() {
		// garbage_ptr is IsDead
		if fp == nil || !IsVariableInSet(fp.PointTo, GarbagePtr) {
			t.Fatal(fp)
		}
	}
}

func TestFindEdgesIn(t *testing.T) {
	fm := NewFactMgr(nil)
	blk := &Block{}
	fm.CreateCFGEdgeTo(10, blk, 20, false, true)
	fm.CreateCFGEdgeTo(11, blk, 20, false, false)
	back := fm.FindEdgesIn(20, false, true)
	if len(back) != 1 || back[0].SrcID != 10 {
		t.Fatal(back)
	}
	fwd := fm.FindEdgesIn(20, false, false)
	if len(fwd) != 1 || fwd[0].SrcID != 11 {
		t.Fatal(fwd)
	}
	if !fm.HasEdgeIn(20, false, true) {
		t.Fatal("has")
	}
}

func TestAnalyzeWithEdgesInMergesJump(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	// prior goto edge from 10 → 20 with facts
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.CreateCFGEdgeTo(10, &Block{}, 20, false, false)
	fm.MapVisited = map[int]bool{10: true}
	fm.MapFactsOut[10] = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// also seed p fact at dest
	facts := []*FactPointTo{MakeFactPointTo(p, CreateVariableScalars("g_a", GetIntType(), false, false))}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("analyze")
	}
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal("merged jump", fp)
	}
}

func TestFindFixedPointBlock(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	b := &Block{Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 1, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(3)}, AssignOp: AssignSimple,
	}}}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	out, _, ok := FindFixedPointBlock(b, nil, &cg, Defaults(), false)
	if !ok {
		t.Fatal("fp")
	}
	_ = out
	if !fm.MapVisited[1] {
		t.Fatal("visited")
	}
}

func TestSetAccumulatedEffectAfterBlock(t *testing.T) {
	st := &Stmt{Kind: StmtIfElse, StmID: 7}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	SetAccumulatedEffectAfterBlock(st, EmptyEffect().WriteVar(v), &cg, EmptyEffect())
	if !fm.GetMapStmEffect(7).IsWritten(v) {
		t.Fatal("effect")
	}
}
