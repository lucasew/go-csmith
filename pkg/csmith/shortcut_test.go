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
	// nil hole fails closed — no invent same-as-skip
	hole := []*FactPointTo{nil}
	if SameFacts(hole, hole) {
		t.Fatal("nil hole must not be same")
	}
	// incomplete PointTo fails closed
	ptHole := []*FactPointTo{{Var: p, PointTo: []*Variable{nil}}}
	if SameFacts(ptHole, ptHole) {
		t.Fatal("nil pointee must not invent SameFacts")
	}
	if FindFact(ptHole, MakeFactPointTo(p, NullPtr)) >= 0 {
		t.Fatal("FindFact incomplete map must fail closed -1")
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
	// nil fact hole fails closed — no invent skip as subset
	hole := []*FactPointTo{nil}
	if SubsetFacts(hole, hole) {
		t.Fatal("nil hole must not be subset")
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
	ClearError()
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

func TestMarkContainedGotosVisitedCFGHoleNoPartial(t *testing.T) {
	// soft invent: mark first goto then stop on nil CFG edge
	// fair: pre-scan holes — mark none when CFG incomplete
	gotoSt := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	root := &Stmt{Kind: StmtFor, StmID: 10, Then: &Block{Stmts: []Stmt{gotoSt}}}
	fm := NewFactMgr(nil)
	fm.MapVisited = map[int]bool{}
	// hole after a valid edge that would mark goto 20
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 20, DestStmID: 10, BackLink: true},
		nil,
		{SrcID: 30, DestStmID: 10},
	}
	MarkContainedGotosVisited(root, fm)
	if fm.MapVisited[20] {
		t.Fatal("incomplete CFG must not invent partial goto visited mark")
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

func TestStmVisitFactsRemoveRVAndAlwaysVisited(t *testing.T) {
	// Statement.cpp:609–626 — remove_rv_facts; map_visited even when visit fails
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	f.RV.Name = "func_1_rv"
	otherRV := CreateVariableScalars("func_2_rv", GetIntType(), false, false)
	otherRV.Name = "func_2_rv"
	// mark as RVs via naming convention used by IsRV
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 42, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(f)
	// inject foreign RV into working facts
	facts := []*FactPointTo{
		MakeFactPointTo(otherRV, GarbagePtr),
		MakeFactPointTo(f.RV, v),
	}
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatal("visit assign should ok")
	}
	if !fm.MapVisited[42] {
		t.Fatal("always visited")
	}
	// foreign RV dropped; own RV kept
	for _, pt := range facts {
		if pt != nil && pt.Var != nil && pt.Var.Name == "func_2_rv" {
			t.Fatal("foreign rv must be removed")
		}
	}
	// validate sets fact_in from pre-visit copy
	pre := []*FactPointTo{MakeFactPointTo(v, GarbagePtr)}
	st2 := &Stmt{
		Kind: StmtAssign, StmID: 43, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}, AssignOp: AssignSimple,
	}
	work := CloneFactSlice(pre)
	if !ValidateAndUpdateFacts(st2, &work, &cg, Defaults(), nil) {
		t.Fatal("validate")
	}
	in := fm.MapFactsIn[43]
	if len(in) != 1 || in[0] == nil || in[0].Var != v {
		t.Fatalf("fact_in should be pre-visit copy: %+v", in)
	}
}

func TestStmVisitFactsMarksVisitedOnFail(t *testing.T) {
	// visit fail (write IV) still marks visited per C++ stm_visit_facts
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 77, LhsVar: iv, Lhs: &Lhs{Var: iv, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.IVBounds = map[*Variable]int{iv: 10}
	facts := []*FactPointTo{}
	FailedStm = nil
	ok := StmVisitFacts(st, &facts, &cg, Defaults())
	if ok {
		t.Fatal("expected visit fail writing IV")
	}
	if !fm.MapVisited[77] {
		t.Fatal("visited must be set even on fail")
	}
	// Statement.cpp:615–617 — failed_stm records non-compound failure
	if FailedStm != st {
		t.Fatal("FailedStm must be the failing assign")
	}
}

func TestContainsUnfixedGotoImply(t *testing.T) {
	// Statement.cpp:797–800 — dest fact not imply jump_src → unfixed
	f := &Function{Name: "f"}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	// dest in: p→{a}; jump src out: p→{a,b} — dest does not imply src (narrower dest
	// imply wider src? Imply is other ⊆ this, so dest.Imply(src) means src ⊆ dest.
	// C++: !f->imply(*jump_src_f) with f = dest in, jump_src = src out.
	// If dest is {a} and src is {a,b}, {a}.Imply({a,b}) is false (b not in dest).
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 10, SourceLabel: "lbl"},
		{Kind: StmtGoto, StmID: 20, Label: "lbl", GotoDestStmID: 10},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	fm.MapVisited = map[int]bool{20: true}
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointToSet(p, []*Variable{a, b})})
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, a)})
	root := &Stmt{Kind: StmtBlock, Then: body, StmID: 1}
	if !ContainsUnfixedGoto(root, fm) {
		t.Fatal("expect unfixed when dest does not imply jump src")
	}
	// equal sets → fixed
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointTo(p, a)})
	if ContainsUnfixedGoto(root, fm) {
		t.Fatal("equal should be fixed")
	}
}

func TestShortcutAnalysisBlockUnfixedGoto(t *testing.T) {
	// block shortcut must not fire with unfixed internal goto
	f := &Function{Name: "f"}
	body := &Block{Func: f, StmID: 50, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 51},
		{Kind: StmtGoto, StmID: 52, Label: "elsewhere", GotoDestStmID: 99},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 52, DestStmID: 99}} // dest outside
	// unvisited goto
	fm.MapVisited = map[int]bool{}
	fm.SetMapFactsIn(50, nil)
	fm.SetMapFactsOut(50, nil)
	fm.SetMapStmEffect(50, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("unfixed goto must block shortcut")
	}
}

func TestStmVisitFactsIncompleteInputFailClosed(t *testing.T) {
	// Fact* always live; incomplete working set must not invent visit success
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 88, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{MakeFactPointTo(v, GarbagePtr), nil}
	if StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatal("incomplete inputs must fail closed")
	}
	if fm.MapVisited[88] {
		t.Fatal("must not mark visited when inputs incomplete before visit")
	}
}

func TestValidateAndUpdateFactsIncompleteInputFailClosed(t *testing.T) {
	// incomplete pre-visit inputs must not invent set_fact_in from cleaned clone
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 90, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{MakeFactPointTo(v, GarbagePtr), nil}
	if ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete inputs must fail closed")
	}
	if _, ok := fm.MapFactsIn[90]; ok {
		t.Fatal("must not invent MapFactsIn from incomplete inputs")
	}
}

func TestShortcutAnalysisMissingOutFailClosed(t *testing.T) {
	// Statement.cpp:559 — inputs = map_facts_out[this]
	// missing out must not invent ShortcutOK while leaving inputs unchanged
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 7, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(7, facts)
	// no MapFactsOut[7]
	fm.SetMapStmEffect(7, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("missing MapFactsOut must fail closed ShortcutNone")
	}
	if fm.MapVisited[7] {
		t.Fatal("must not mark visited on incomplete shortcut")
	}
}

func TestShortcutAnalysisIncompleteOutFailClosed(t *testing.T) {
	// nil fact hole in MapFactsOut — no invent clone-to-nil while ShortcutOK
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 8, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.SetMapFactsIn(8, in)
	// plant hole bypassing SetMapFactsOut (CloneFactSlice strips holes)
	fm.MapFactsOut = map[int][]*FactPointTo{
		8: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	fm.SetMapStmEffect(8, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := CloneFactSlice(in)
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("incomplete MapFactsOut must fail closed ShortcutNone")
	}
	// inputs must not be replaced with invent-cleaned clone
	if !SameFacts(facts, in) {
		t.Fatal("facts must stay pre-shortcut inputs on fail closed")
	}
}

func TestShortcutAnalysisBlockMissingOutFailClosed(t *testing.T) {
	body := &Block{StmID: 60, Stmts: []Stmt{{Kind: StmtAssign, StmID: 61}}}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(60, facts)
	// no MapFactsOut[60]
	fm.SetMapStmEffect(60, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("block missing MapFactsOut must fail closed")
	}
}

func TestShortcutAnalysisBlockIncompleteOutFailClosed(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	body := &Block{StmID: 70, Stmts: []Stmt{{Kind: StmtAssign, StmID: 71}}}
	fm := NewFactMgr(nil)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.SetMapFactsIn(70, in)
	fm.MapFactsOut = map[int][]*FactPointTo{
		70: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	fm.SetMapStmEffect(70, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := CloneFactSlice(in)
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("block incomplete MapFactsOut must fail closed")
	}
}
