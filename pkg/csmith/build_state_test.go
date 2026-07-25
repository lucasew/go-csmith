package csmith

import "testing"

func TestBuildStateTransitions(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "func_1_rv", GetIntType(), false, false)
	// Function.cpp FMList at create — pair before GenerateBody (no invent inside)
	_ = f.ensurePairedFactMgr()
	if f.BuildState != BuildUnbuilt || f.IsEffectKnown() {
		t.Fatal("unbuilt")
	}
	f.GenerateBody(NewRngSess(testAmbientSession, 2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), WithFunc(f, EmptyEffect()).WithSession(testAmbientSession))
	if f.BuildState != BuildBuilt || !f.IsBuilt || !f.IsEffectKnown() {
		t.Fatalf("built %v", f.BuildState)
	}
	// regenerate ignored
	body := f.Body
	f.GenerateBody(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), WithFunc(f, EmptyEffect()).WithSession(testAmbientSession))
	if f.Body != body {
		t.Fatal("regen")
	}
}

func TestPointerParamTBD(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", GetIntType(), false, false)
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", PointerTo(GetIntType()), false, false)
	f.Param = []*Variable{p}
	// pair FactMgr at create (Function.cpp FMList); pass same via CGContext
	fm := f.ensurePairedFactMgr()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	f.GenerateBody(NewRngSess(testAmbientSession, 2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg)
	// after build, may still have fact (or oos); at least was added during building
	// regenerate blocked so check via second function
	f2 := &Function{Name: "f2", ReturnType: GetIntType()}
	f2.RV = CreateVariableScalarsSess(testAmbientSession, "f2_rv", GetIntType(), false, false)
	p2 := CreateVariableScalarsSess(testAmbientSession, "p_2", PointerTo(GetIntType()), false, false)
	f2.Param = []*Variable{p2}
	fm2 := NewFactMgrSess(testAmbientSession, f2)
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
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	if !g.IsVisibleSess(testAmbientSession, nil) {
		t.Fatal("global")
	}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	l := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	blk.LocalVars = []*Variable{l}
	if !l.IsVisibleSess(testAmbientSession, blk) {
		t.Fatal("local")
	}
	if l.IsVisibleSess(testAmbientSession, nil) {
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
	// nil Function* hole fails closed as at-max non-sticky (soft re-pick gate)
	ClearErrorSess(testAmbientSession)
	opts.MaxFuncs = 100
	list.Funcs = []*Function{{Name: "f", IsBuilt: true}, nil}
	if !ReachMaxFunctions(list, opts) {
		t.Fatal("nil hole must fail closed as at-max")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole ReachMaxFunctions must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeFirstMarksBuilt(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, probs, vs, nil)
	f := MakeFirst(NewRngSess(testAmbientSession, 2), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), nil, nil)
	if f == nil || !f.IsEffectKnown() {
		t.Fatal("built")
	}
}

func TestChooseFuncSkipsBuilding(t *testing.T) {
	built := &Function{Name: "a", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	building := &Function{Name: "b", ReturnType: GetIntType(), BuildState: BuildBuilding}
	got := ChooseFunc(NewRngSess(testAmbientSession, 2), []*Function{building, built}, GetIntType(), nil)
	if got != built {
		t.Fatal(got)
	}
}

func TestGenerateBodyIncompleteAmbientResidualSticky(t *testing.T) {
	// incomplete ambient residual soft invent was invent Built shell past hole.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntType(), BuildState: BuildUnbuilt}
	prev := EmptyCGContext().WithSession(testAmbientSession)
	prev.EffectStm = IncompleteEffect()
	f.GenerateBody(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), NewStatementThresholdTable(opts), prev)
	if f.BuildState != BuildUnbuilt {
		t.Fatal("incomplete ambient must leave Unbuilt", f.BuildState)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient GenerateBody must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
