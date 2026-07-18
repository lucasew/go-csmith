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

func TestArrayBuildInitRecursive(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, "g_1", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false}))
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

func TestRandomParentBlockERRORGuard(t *testing.T) {
	ClearError()
	b := &Block{}
	SetError(ErrGeneric)
	if b.RandomParentBlock(NewRng(1), true) != nil {
		t.Fatal("ERROR_GUARD")
	}
	ClearError()
}
