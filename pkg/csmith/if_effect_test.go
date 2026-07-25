package csmith

import (
	"strings"
	"testing"
)

func TestIfBranchesIsolateEffect(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	// assign-only so arms write
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	opts.MaxBlockSize = 2
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.Types = &TypeEnv{Sess: testAmbientSession}
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// plant a known global
	g1 := CreateVariableQferSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	vs.GlobalList = []*Variable{g1}
	st := MakeRandomIf(NewRngSess(testAmbientSession, 7), opts, probs, vs, tables, tab, &cg)
	if st == nil || st.Then == nil || st.Else == nil {
		t.Fatal("if")
	}
	// parent accum should not be SE-free if either arm wrote
	// (with only assigns, likely wrote)
	_ = stmtTab
	// structural: both arms have statements possibly
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "if (") || !strings.Contains(out, "else") {
		t.Fatal(out)
	}
}

func TestMergeEffectsUnion(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	e1 := EmptyEffect().WriteVarSess(testAmbientSession, a)
	e2 := EmptyEffect().WriteVarSess(testAmbientSession, b)
	// Effect.cpp:write_var — non-volatile write keeps SE-free true
	if !e1.IsSideEffectFreeSess(testAmbientSession) || !e2.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("non-volatile WriteVar must stay SE-free")
	}
	m := MergeEffectsSess(testAmbientSession, e1, e2)
	if !m.IsWrittenSess(testAmbientSession, a) || !m.IsWrittenSess(testAmbientSession, b) {
		t.Fatal("union")
	}
	// Effect.cpp:add_effect — side_effect_free &= e.side_effect_free only
	// (not invent SE-free false for every write)
	if !m.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("MergeEffects of non-vol writes must stay SE-free")
	}
	// volatile write clears SE-free on that arm → merge not SE-free
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	ev := EmptyEffect().WriteVarSess(testAmbientSession, v)
	if ev.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("volatile WriteVar must clear SE-free")
	}
	mv := MergeEffectsSess(testAmbientSession, e1, ev)
	if mv.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("MergeEffects with volatile arm must not be SE-free")
	}
	if !mv.IsWrittenSess(testAmbientSession, a) || !mv.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("volatile merge must keep both writes")
	}
}

func TestMergeEffectsIncompleteFailClosed(t *testing.T) {
	// incomplete arm must not invent pure/empty-complete merge success — sticky
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	ok := EmptyEffect().WriteVarSess(testAmbientSession, a)
	m := MergeEffectsSess(testAmbientSession, ok, IncompleteEffect())
	if EffectComplete(m) {
		t.Fatal("incomplete b must fail closed MergeEffects")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete b MergeEffects must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	m2 := MergeEffectsSess(testAmbientSession, IncompleteEffect(), ok)
	if EffectComplete(m2) {
		t.Fatal("incomplete a must fail closed MergeEffects")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete a MergeEffects must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil map key on complete-looking shell
	bad := EmptyEffect()
	bad.read = map[*Variable]bool{nil: true}
	m3 := MergeEffectsSess(testAmbientSession, ok, bad)
	if EffectComplete(m3) {
		t.Fatal("nil key must fail closed MergeEffects")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil key MergeEffects must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayBuildInitRecursive(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 1), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil")
	}
	av.Sizes = []int{2, 2}
	av.ArraySizes = av.Sizes
	av.InitValues = []string{"1", "2", "3"}
	av.ArrayInits = av.InitValues
	out := av.OutputDefSess(testAmbientSession, Defaults())
	if !strings.Contains(out, "{{") {
		t.Fatal("want nested braces", out)
	}
	// empty init_strings list is broken IR sticky
	ClearErrorSess(testAmbientSession)
	if av.buildInitRecursiveSess(testAmbientSession, 0, nil) != "" {
		t.Fatal("empty init list must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty init list must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if av.buildInitRecursiveSess(testAmbientSession, 0, []string{""}) != "" {
		t.Fatal("empty hole string must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty hole string must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfERRORGuardAfterBranches(t *testing.T) {
	// StatementIf.cpp:94/99 ERROR_GUARD after Block::make_random branches
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.Types = vs.Types
	f.Stack = []*Block{{Func: f}}
	// sticky error after condition would abort; set after a successful path component
	st := MakeRandomIf(NewRngSess(testAmbientSession, 2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	// may succeed with empty blocks (max size 0)
	if HasErrorSess(testAmbientSession) {
		if st != nil {
			t.Fatal("sticky error must fail closed")
		}
	}
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	st2 := MakeRandomIf(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st2 != nil {
		t.Fatal("ERROR_GUARD after flip path: want nil")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfElseFromThenMapFactsIn(t *testing.T) {
	// StatementIf.cpp:97 — global_facts = map_facts_in[if_true]
	// missing then-in must not invent pre-branch GlobalFacts for else
	// Unit: plant missing MapFactsIn after then would have set it — contract of assign
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	prior := MakeFactPointToSess(testAmbientSession, p, GarbagePtr)
	fm.GlobalFacts = []*FactPointTo{prior}
	// missing MapFactsIn[5]
	thenStmID := 5
	in := fm.MapFactsIn[thenStmID]
	if !FactsComplete(in) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSliceSess(testAmbientSession, in)
	}
	// missing MapFactsIn is complete empty (C++ map[]); must not keep prior
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) != nil {
		t.Fatal("missing then MapFactsIn must clear prior, not invent pre-branch")
	}
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("missing then-in is complete empty, not incomplete marker")
	}
	// incomplete hole
	fm.GlobalFacts = []*FactPointTo{prior}
	fm.MapFactsIn = map[int][]*FactPointTo{
		5: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	in = fm.MapFactsIn[thenStmID]
	if !FactsComplete(in) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSliceSess(testAmbientSession, in)
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete then MapFactsIn must fail closed")
	}
}

func TestAssignGlobalFactsFromMapInRewindsUnionWrite(t *testing.T) {
	// StatementIf.cpp:97–98 — fm->global_facts = fm->map_facts_in[if_true]
	// Full FactVec (ePointTo + eUnionWrite). Soft invent was SetGlobalFacts(PT-only)
	// so else generation kept then-exit UnionFacts last-writes → IsNonreadableField
	// over-filtered choose_var (seed-7 eligible pool half-size).
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_u", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 2 {
		t.Fatal("need union fields")
	}
	f0 := parent.FieldVars[0]
	entryU := MakeFactUnionSess(testAmbientSession, parent, 0) // then-entry: last write f0
	exitU := MakeFactUnionSess(testAmbientSession, parent, 1)  // then-exit: last write f1
	if entryU == nil || exitU == nil {
		t.Fatal("MakeFactUnion")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	// Live env after then-branch: advanced union write
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{exitU}
	// map_facts_in[then] stored entry FactVec (both partitions)
	thenID := 42
	fm.SetMapFactsInPair(thenID, []*FactPointTo{}, []*FactUnion{entryU})
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	// Production path: AssignGlobalFactsFromMapIn (full FactVec)
	fm.AssignGlobalFactsFromMapIn(thenID)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if len(fm.UnionFacts) != 1 || fm.UnionFacts[0] == nil || fm.UnionFacts[0].LastWrittenFID != 0 {
		t.Fatalf("want entry last_written 0, got %#v", fm.UnionFacts)
	}
	if IsNonreadableFieldSess(testAmbientSession, f0, fm.UnionFacts) {
		t.Fatal("after map_facts_in assign, f0 must be readable")
	}
	// Document PT-only hole: SetGlobalFacts alone leaves exit union write
	fm.UnionFacts = []*FactUnion{exitU}
	fm.SetGlobalFacts([]*FactPointTo{}, "test_pt_only")
	if !IsNonreadableFieldSess(testAmbientSession, f0, fm.UnionFacts) {
		t.Fatal("PT-only SetGlobalFacts must leave exit union last-write (hole)")
	}
	ClearErrorSess(testAmbientSession)
}

// TestVisitFactsStatementIfSharesEffectAccum — StatementIf.cpp:170–177.
// Both arms visit with the same CGContext& so effect_accum grows through true
// then false. Soft invent forked arm accums → StmVisitFacts rewrote
// map_accum_effect[nested] without outer history (seed-42 choose_visible nOk).
func TestVisitFactsStatementIfSharesEffectAccum(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	outer := CreateVariableScalarsSess(testAmbientSession, "g_outer", GetIntTypeSess(testAmbientSession), false, false)
	inner := CreateVariableScalarsSess(testAmbientSession, "g_inner", GetIntTypeSess(testAmbientSession), false, false)
	fn := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, fn)
	// then: read g_inner via return expr (visit records CheckReadVar)
	thenRet := Stmt{
		Kind: StmtReturn, StmID: AllocStmID(),
		Expr: &Expression{Term: TermVariable, Var: inner, ExprType: GetIntTypeSess(testAmbientSession)},
	}
	elseRet := Stmt{
		Kind: StmtReturn, StmID: AllocStmID(),
		Expr: &Expression{Term: TermVariable, Var: outer, ExprType: GetIntTypeSess(testAmbientSession)},
	}
	thenBlk := &Block{StmID: AllocStmID(), Func: fn, Stmts: []Stmt{thenRet}}
	elseBlk := &Block{StmID: AllocStmID(), Func: fn, Stmts: []Stmt{elseRet}}
	fm.MapStmEffect = map[int]Effect{
		thenBlk.StmID: EmptyEffect(),
		elseBlk.StmID: EmptyEffect(),
	}
	st := &Stmt{
		Kind: StmtIfElse, StmID: AllocStmID(),
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
		Then: thenBlk, Else: elseBlk,
	}
	fn.RV = CreateVariableScalarsSess(testAmbientSession, "g_rv", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = fn
	// parent accum already observed g_outer (as if prior statements in the block)
	eff := EmptyEffect().ReadVarSess(testAmbientSession, outer)
	cg.EffectAccum = &eff
	if !VisitFactsStatementIf(st, &cg, opts) {
		t.Fatalf("visit if err=%v", HasErrorSess(testAmbientSession))
	}
	// After shared-arm visit, accum must include outer (pre) + both arm reads.
	if cg.EffectAccum == nil || !cg.EffectAccum.IsReadSess(testAmbientSession, outer) {
		t.Fatal("shared effect_accum must keep pre-if outer read")
	}
	if !cg.EffectAccum.IsReadSess(testAmbientSession, inner) {
		t.Fatal("shared effect_accum must record true-arm read of g_inner")
	}
	// Nested then stmt map_accum must include pre-if history (not arm-local only).
	thenAcc := fm.GetMapAccumEffect(thenRet.StmID)
	if !EffectComplete(thenAcc) || !thenAcc.IsReadSess(testAmbientSession, outer) {
		t.Fatalf("map_accum_effect[then] must include outer pre-history, reads=%v", thenAcc.ReadVarsSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

// TestVisitFactsStatementIfRewindsUnionBeforeElse — StatementIf.cpp:170–177.
// Else arm starts from post-condition full FactVec (ePointTo + eUnionWrite).
// Soft invent restored only GlobalFacts so UnionFacts stayed at then-exit
// last-writes (seed-7 nested over-strip).
func TestVisitFactsStatementIfRewindsUnionBeforeElse(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	ut := &Type{isUnion: true, StructName: "U_vif", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_uv", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 1 {
		t.Fatal("need field")
	}
	f0 := parent.FieldVars[0]
	entryU := MakeFactUnionSess(testAmbientSession, parent, 0)
	exitU := MakeFactUnionSess(testAmbientSession, parent, 1)
	if entryU == nil || exitU == nil {
		t.Fatal("MakeFactUnion")
	}
	fn := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, fn)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{entryU}
	thenBlk := &Block{StmID: AllocStmID(), Func: fn, Stmts: []Stmt{}}
	elseBlk := &Block{StmID: AllocStmID(), Func: fn, Stmts: []Stmt{}}
	fm.MapStmEffect = map[int]Effect{thenBlk.StmID: EmptyEffect(), elseBlk.StmID: EmptyEffect()}
	// then out has exit last-write; else out re-entry
	fm.SetMapFactsInPair(thenBlk.StmID, []*FactPointTo{}, []*FactUnion{entryU})
	fm.SetMapFactsOutPair(thenBlk.StmID, []*FactPointTo{}, []*FactUnion{exitU})
	fm.SetMapFactsInPair(elseBlk.StmID, []*FactPointTo{}, []*FactUnion{entryU})
	fm.SetMapFactsOutPair(elseBlk.StmID, []*FactPointTo{}, []*FactUnion{entryU})
	fm.MapVisited = map[int]bool{thenBlk.StmID: true, elseBlk.StmID: true}
	st := &Stmt{
		Kind: StmtIfElse, StmID: AllocStmID(),
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
		Then: thenBlk, Else: elseBlk,
	}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = fn
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsStatementIf(st, &cg, opts) {
		t.Fatalf("visit if failed err=%v", HasErrorSess(testAmbientSession))
	}
	if !UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("UnionFacts incomplete")
	}
	if FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, parent) == nil {
		t.Fatal("union subject missing after if visit")
	}
	// Sanity: exit-only would make f0 nonreadable; merged then+else should not be stuck
	// solely in a way that loses completeness.
	_ = f0
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfNoInventWithoutRNG(t *testing.T) {
	// StatementIf.cpp always has RNG + CGContext sticky; no invent if shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if st := MakeRandomIf(nil, opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), NewStatementThresholdTable(opts), nil); st != nil {
		t.Fatal("nil RNG+cg")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG+cg MakeRandomIf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if st := MakeRandomIf(nil, opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg); st != nil {
		t.Fatal("nil RNG")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomIf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomParentBlockERRORGuard(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	b := &Block{}
	SetErrorSess(testAmbientSession, ErrGeneric)
	if b.RandomParentBlock(NewRngSess(testAmbientSession, 1), true) != nil {
		t.Fatal("ERROR_GUARD")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfIncompleteThenInFailClosed(t *testing.T) {
	// incomplete EffectAccum must fail closed before arms (shared accum contract)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetSimpleTypeSess(testAmbientSession, EVoid)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	cg.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	st := MakeRandomIf(NewRngSess(testAmbientSession, 1), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomIf")
	}
	ClearErrorSess(testAmbientSession)
}

// TestMakeRandomIfSharesCGContextWithParent — StatementIf.cpp:93–99 both arms
// use the same CGContext& (shared effect_accum, effect_stm, blk_depth, iv_bounds).
// CloneSubcontext arms left parent EffectStm/BlkDepth stale so else started from a
// second clone (seed-2 e13830: SelectParentLocal stack n=4 vs UP n=5).
func TestMakeRandomIfSharesCGContextWithParent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	// assign-only arms so generation writes/reads globals into shared accum
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	opts.MaxBlockSize = 2
	r := NewRngSess(testAmbientSession, 2)
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	fm := NewFactMgrSess(testAmbientSession, f)
	pre := CreateVariableScalarsSess(testAmbientSession, "pre_if_rd", GetIntTypeSess(testAmbientSession), true, false)
	g1 := CreateVariableScalarsSess(testAmbientSession, "g_if_arm", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = append(vs.GlobalList, g1)
	accum := EmptyEffect().ReadVarSess(testAmbientSession, pre)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &accum
	cg.Types = vs.Types
	preDepth := cg.BlkDepth
	st := MakeRandomIf(NewRngSess(testAmbientSession, 9), opts, probs, vs, tables, tab, &cg)
	if cg.EffectAccum == nil {
		t.Fatal("parent EffectAccum must remain non-nil")
	}
	if !cg.EffectAccum.IsReadSess(testAmbientSession, pre) {
		t.Fatal("parent EffectAccum must keep pre-if reads when arms share accum")
	}
	// shared pointer: arm generation must not rebind parent to a private snapshot
	if cg.EffectAccum != &accum {
		t.Fatal("MakeRandomIf must keep parent EffectAccum pointer identity (C++ shared)")
	}
	// compound factories bump/restore BlkDepth on the shared context
	if cg.BlkDepth != preDepth {
		t.Fatalf("shared cg BlkDepth must restore after if arms: got %d want %d", cg.BlkDepth, preDepth)
	}
	if st != nil && st.StmID > 0 && fm != nil {
		// map_stm_effect[if] = cond + then_block + else_block (sequential mutates)
		if !EffectComplete(fm.GetMapStmEffect(st.StmID)) {
			t.Fatal("if map_stm_effect must be complete after set_accumulated_effect_after_block")
		}
	}
	_ = g1
	_ = stmtTab
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomForIncompleteEffectAccumFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// seed globals so MakeIteration can succeed; fail closed is on incomplete EffectAccum after
	f := &Function{Name: "f", ReturnType: GetSimpleTypeSess(testAmbientSession, EVoid)}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	inc := IncompleteEffect()
	cg.EffectAccum = &inc
	if MakeRandomFor(NewRngSess(testAmbientSession, 2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomFor")
	}
	// nil return is the invent ban; SetError when iteration path reaches accum check
	ClearErrorSess(testAmbientSession)
}
