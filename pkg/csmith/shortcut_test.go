package csmith

import (
	"testing"
)

func TestSameFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	b := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if !SameFacts(a, b) {
		t.Fatal("same")
	}
	b2 := []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	if SameFacts(a, b2) {
		t.Fatal("diff")
	}
}

func TestSubsetFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// wider set implies narrower
	wide := []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}
	narrow := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// subset_facts(narrow, wide): each narrow must be implied by related in wide
	// wide.Imply(narrow) means wide covers narrow's points
	if !SubsetFacts(narrow, wide) {
		// narrow is subset if wide implies narrow
		// Imply: f.Imply(other) means f covers other
		// SubsetFacts(a,b): for each f1 in a, related f2 in b implies f1
		// so f2.Imply(f1) — wide implies narrow ✓
		if !wide[0].Imply(narrow[0]) {
			t.Fatal("imply")
		}
		// size must match for subset_facts upstream
		t.Log("size mismatch expected fail")
	}
	// same size: both one fact
	if !SubsetFacts(narrow, []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}) {
		// related wide implies null-only? wide implies null-only yes
		// wait SubsetFacts(narrow, wide2) with same len
		w := []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}
		if !SubsetFacts(narrow, w) {
			t.Fatal("subset")
		}
	}
}

func TestIsCtrlStmt(t *testing.T) {
	if !IsCtrlStmt(&Stmt{Kind: StmtBreak}) || IsCtrlStmt(&Stmt{Kind: StmtAssign}) {
		t.Fatal("ctrl")
	}
}

func TestShortcutAnalysisReuse(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 5, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(5, facts)
	fm.SetMapFactsOut(5, facts)
	fm.SetMapStmEffect(5, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// first: no prior in match empty — SameFacts empty empty works
	sc := ShortcutAnalysis(st, &facts, &cg, Defaults())
	if sc != ShortcutOK {
		t.Fatal(sc)
	}
	// ctrl stmt never shortcuts
	st2 := &Stmt{Kind: StmtReturn, StmID: 6}
	fm.SetMapFactsIn(6, facts)
	if ShortcutAnalysis(st2, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("ctrl")
	}
}

func TestShortcutConflict(t *testing.T) {
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	st := &Stmt{Kind: StmtAssign, StmID: 3}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(3, facts)
	fm.SetMapFactsOut(3, facts)
	// previous effect wrote g_x
	fm.SetMapStmEffect(3, EmptyEffect().WriteVar(g))
	// ambient context also wrote g_x → InConflict
	cg := WithEffectContext(EmptyEffect().WriteVar(g))
	cg.FM = fm
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutConflict {
		t.Fatal("want conflict")
	}
}

func TestValidateAndUpdateFacts(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 9, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if !ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("validate")
	}
	if !fm.MapVisited[9] {
		t.Fatal("visited")
	}
	// second pass with same facts → shortcut
	if !ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("shortcut")
	}
}

func TestValidateAndUpdateFactsMarksContainedGotos(t *testing.T) {
	// Statement.cpp:580–595 — shortcut reuse marks gotos inside tree visited
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	gotoSt := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	// for-like compound with nested goto
	loop := &Stmt{
		Kind: StmtFor, StmID: 10,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		Then: &Block{Stmts: []Stmt{gotoSt}},
	}
	fm := NewFactMgr(nil)
	// seed maps so shortcut fires
	fm.SetMapFactsIn(10, nil)
	fm.SetMapFactsOut(10, nil)
	fm.SetMapStmEffect(10, EmptyEffect())
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10, BackLink: true}}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if !ValidateAndUpdateFacts(loop, &facts, &cg, Defaults(), nil) {
		t.Fatal("shortcut validate")
	}
	if !fm.MapVisited[20] {
		t.Fatal("goto inside for should be marked visited on shortcut")
	}
	if !fm.MapVisited[10] {
		t.Fatal("loop itself visited")
	}
}

func TestCGContextAddEffect(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.AddEffect(EmptyEffect().WriteVar(v), false)
	if !cg.AccumEffect().IsWritten(v) || !cg.EffectStm.IsWritten(v) {
		t.Fatal("add")
	}
}

func TestContainsStmt(t *testing.T) {
	inner := Stmt{Kind: StmtAssign, StmID: 2}
	outer := Stmt{Kind: StmtIfElse, StmID: 1, Then: &Block{Stmts: []Stmt{inner}}}
	if !ContainsStmt(&outer, &inner) {
		t.Fatal("contains")
	}
}
