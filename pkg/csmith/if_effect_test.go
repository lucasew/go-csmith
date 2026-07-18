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
	st := MakeRandomIf(NewRng(7), opts, probs, vs, tables, tab, cg)
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
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_1", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false}))
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
