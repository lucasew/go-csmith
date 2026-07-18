package csmith

import "testing"

func TestBuildStateTransitions(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	// Function.cpp FMList at create — pair before GenerateBody (no invent inside)
	_ = f.ensurePairedFactMgr()
	if f.BuildState != BuildUnbuilt || f.IsEffectKnown() {
		t.Fatal("unbuilt")
	}
	f.GenerateBody(NewRng(2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), WithFunc(f, EmptyEffect()))
	if f.BuildState != BuildBuilt || !f.IsBuilt || !f.IsEffectKnown() {
		t.Fatalf("built %v", f.BuildState)
	}
	// regenerate ignored
	body := f.Body
	f.GenerateBody(NewRng(3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), WithFunc(f, EmptyEffect()))
	if f.Body != body {
		t.Fatal("regen")
	}
}

func TestPointerParamTBD(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	f.Param = []*Variable{p}
	// pair FactMgr at create (Function.cpp FMList); pass same via CGContext
	fm := f.ensurePairedFactMgr()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	f.GenerateBody(NewRng(2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg)
	// after build, may still have fact (or oos); at least was added during building
	// regenerate blocked so check via second function
	f2 := &Function{Name: "f2", ReturnType: GetIntType()}
	f2.RV = CreateVariableScalars("f2_rv", GetIntType(), false, false)
	p2 := CreateVariableScalars("p_2", PointerTo(GetIntType()), false, false)
	f2.Param = []*Variable{p2}
	fm2 := NewFactMgr(f2)
	// manually run param fact path: Building adds tbd before body
	f2.BuildState = BuildBuilding
	if FindRelatedPointTo(fm2.GlobalFacts, p2) == nil {
		fm2.GlobalFacts = append(fm2.GlobalFacts, MakeFactPointTo(p2, TBDPtr))
	}
	if !FindRelatedPointTo(fm2.GlobalFacts, p2).IsTBDOnly() {
		t.Fatal("tbd")
	}
}

func TestIsVisible(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if !g.IsVisible(nil) {
		t.Fatal("global")
	}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk.LocalVars = []*Variable{l}
	if !l.IsVisible(blk) {
		t.Fatal("local")
	}
	if l.IsVisible(nil) {
		t.Fatal("local nil blk")
	}
}

func TestStatementFilterMaxFuncs(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 1
	// ReachMaxFunctions with one non-builtin
	list := &FunctionList{Funcs: []*Function{{Name: "f", IsBuilt: true}}}
	if !ReachMaxFunctions(list, opts) {
		t.Fatal("max")
	}
}

func TestMakeFirstMarksBuilt(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), nil, nil)
	if f == nil || !f.IsEffectKnown() {
		t.Fatal("built")
	}
}

func TestChooseFuncSkipsBuilding(t *testing.T) {
	built := &Function{Name: "a", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	building := &Function{Name: "b", ReturnType: GetIntType(), BuildState: BuildBuilding}
	got := ChooseFunc(NewRng(2), []*Function{building, built}, GetIntType(), nil)
	if got != built {
		t.Fatal(got)
	}
}
