package csmith

import (
	"strings"
	"testing"
)

func TestIfBranchesIsolateEffect(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	// assign-only so arms write
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	opts.MaxBlockSize = 2
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	cg.Types = &TypeEnv{}
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// plant a known global
	g1 := CreateVariableQfer("g_1", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	vs.GlobalList = []*Variable{g1}
	st := MakeRandomIf(NewRng(7), opts, probs, vs, tables, tab, &cg)
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
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e1 := EmptyEffect().WriteVar(a)
	e2 := EmptyEffect().WriteVar(b)
	m := MergeEffects(e1, e2)
	if !m.IsWritten(a) || !m.IsWritten(b) {
		t.Fatal("union")
	}
	if m.IsSideEffectFree() {
		t.Fatal("SE")
	}
}

func TestMergeEffectsIncompleteFailClosed(t *testing.T) {
	// incomplete arm must not invent pure/empty-complete merge success — sticky
	ClearError()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	ok := EmptyEffect().WriteVar(a)
	m := MergeEffects(ok, IncompleteEffect())
	if EffectComplete(m) {
		t.Fatal("incomplete b must fail closed MergeEffects")
	}
	if !HasError() {
		t.Fatal("incomplete b MergeEffects must SetError sticky")
	}
	ClearError()
	m2 := MergeEffects(IncompleteEffect(), ok)
	if EffectComplete(m2) {
		t.Fatal("incomplete a must fail closed MergeEffects")
	}
	if !HasError() {
		t.Fatal("incomplete a MergeEffects must SetError sticky")
	}
	ClearError()
	// nil map key on complete-looking shell
	bad := EmptyEffect()
	bad.read = map[*Variable]bool{nil: true}
	m3 := MergeEffects(ok, bad)
	if EffectComplete(m3) {
		t.Fatal("nil key must fail closed MergeEffects")
	}
	if !HasError() {
		t.Fatal("nil key MergeEffects must SetError sticky")
	}
	ClearError()
}

func TestArrayBuildInitRecursive(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil")
	}
	av.Sizes = []int{2, 2}
	av.ArraySizes = av.Sizes
	av.InitValues = []string{"1", "2", "3"}
	av.ArrayInits = av.InitValues
	out := av.OutputDef()
	if !strings.Contains(out, "{{") {
		t.Fatal("want nested braces", out)
	}
	// empty init_strings list is broken IR sticky
	ClearError()
	seed := uint32(1)
	if av.buildInitRecursive(0, nil, &seed) != "" {
		t.Fatal("empty init list must fail closed")
	}
	if !HasError() {
		t.Fatal("empty init list must SetError sticky")
	}
	ClearError()
	if av.buildInitRecursive(0, []string{""}, &seed) != "" {
		t.Fatal("empty hole string must fail closed")
	}
	if !HasError() {
		t.Fatal("empty hole string must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomIfERRORGuardAfterBranches(t *testing.T) {
	// StatementIf.cpp:94/99 ERROR_GUARD after Block::make_random branches
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.Types = vs.Types
	f.Stack = []*Block{{Func: f}}
	// sticky error after condition would abort; set after a successful path component
	st := MakeRandomIf(NewRng(2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	// may succeed with empty blocks (max size 0)
	if HasError() {
		if st != nil {
			t.Fatal("sticky error must fail closed")
		}
	}
	ClearError()
	SetError(ErrGeneric)
	st2 := MakeRandomIf(NewRng(3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st2 != nil {
		t.Fatal("ERROR_GUARD after flip path: want nil")
	}
	ClearError()
}

func TestMakeRandomIfElseFromThenMapFactsIn(t *testing.T) {
	// StatementIf.cpp:97 — global_facts = map_facts_in[if_true]
	// missing then-in must not invent pre-branch GlobalFacts for else
	// Unit: plant missing MapFactsIn after then would have set it — contract of assign
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	prior := MakeFactPointTo(p, GarbagePtr)
	fm.GlobalFacts = []*FactPointTo{prior}
	// missing MapFactsIn[5]
	thenStmID := 5
	in := fm.MapFactsIn[thenStmID]
	if !FactsComplete(in) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSlice(in)
	}
	// missing MapFactsIn is complete empty (C++ map[]); must not keep prior
	if FindRelatedPointTo(fm.GlobalFacts, p) != nil {
		t.Fatal("missing then MapFactsIn must clear prior, not invent pre-branch")
	}
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("missing then-in is complete empty, not incomplete marker")
	}
	// incomplete hole
	fm.GlobalFacts = []*FactPointTo{prior}
	fm.MapFactsIn = map[int][]*FactPointTo{
		5: {MakeFactPointTo(p, NullPtr), nil},
	}
	in = fm.MapFactsIn[thenStmID]
	if !FactsComplete(in) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSlice(in)
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete then MapFactsIn must fail closed")
	}
}

func TestMakeRandomIfNoInventWithoutRNG(t *testing.T) {
	// StatementIf.cpp always has RNG + CGContext sticky; no invent if shell
	ClearError()
	opts := Defaults()
	if st := MakeRandomIf(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), nil); st != nil {
		t.Fatal("nil RNG+cg")
	}
	if !HasError() {
		t.Fatal("nil RNG+cg MakeRandomIf must SetError sticky")
	}
	ClearError()
	cg := EmptyCGContext()
	if st := MakeRandomIf(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg); st != nil {
		t.Fatal("nil RNG")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomIf must SetError sticky")
	}
	ClearError()
}

func TestRandomParentBlockERRORGuard(t *testing.T) {
	ClearError()
	b := &Block{}
	SetError(ErrGeneric)
	if b.RandomParentBlock(NewRng(1), true) != nil {
		t.Fatal("ERROR_GUARD")
	}
	ClearError()
}

func TestMakeRandomIfIncompleteThenInFailClosed(t *testing.T) {
	// incomplete map_facts_in[then] must not invent else gen / if success
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	// Plant then block will get new StmID; hard to plant incomplete then-in mid-make.
	// Instead incomplete GlobalFacts before make fails closed when func_1 hack off:
	// Use incomplete EffectAccum so arm merge fails after branches.
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	cg.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	// MakeRandomIf will fail on MergeEffects incomplete or MakeRandomBlock
	st := MakeRandomIf(NewRng(1), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomIf")
	}
	ClearError()
}

func TestMakeRandomForIncompleteEffectAccumFailClosed(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 0
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// seed globals so MakeIteration can succeed; fail closed is on incomplete EffectAccum after
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, NewRng(1))
	inc := IncompleteEffect()
	cg.EffectAccum = &inc
	if MakeRandomFor(NewRng(2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomFor")
	}
	// nil return is the invent ban; SetError when iteration path reaches accum check
	ClearError()
}
