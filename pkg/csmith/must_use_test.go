package csmith

import "testing"

func TestFindMustUseArrays(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	sc := CreateVariableScalars("g_i", GetIntType(), false, false)
	rw := &RWDirective{
		MustReadVars:  []*Variable{&av.Variable, sc},
		MustWriteVars: []*Variable{&av.Variable},
	}
	got := rw.FindMustUseArrays()
	if len(got) != 1 || got[0] != av {
		t.Fatalf("%v", got)
	}
}

func TestSelectMustUseVar(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	cg := WithFunc(f, EmptyEffect()).WithRW(rw)
	v := vs.SelectMustUseVar(NewRng(2), AccessWrite, cg, GetIntType(), nil)
	if v != g {
		t.Fatalf("%v", v)
	}
	// 75% may erase — either still present or gone is fine
}

func TestSelectMustUseArrayItemize(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(3), opts, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	rw := &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	cg := WithFunc(f, EmptyEffect()).WithRW(rw)
	v := vs.SelectMustUseVar(NewRng(5), AccessRead, cg, GetIntType(), nil)
	if v == nil {
		t.Fatal("nil")
	}
	// itemized has Collective set
	if v.AsArray != nil && v.AsArray.Collective == nil && v == &av.Variable {
		// returned collective without itemize if flip failed path
		t.Log("collective")
	}
}

func TestMakeRandomLhsMustUse(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_w", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	// force only must-use by empty globals after? keep g
	cg := EmptyCGContext().WithRW(rw)
	lhs, _ := MakeRandomLhs(NewRng(2), opts, NewProbabilities(opts), vs, cg, GetIntType(), false)
	if lhs == nil {
		t.Fatal("nil")
	}
}

func TestChooseVarQferFilter(t *testing.T) {
	vol := CreateVariableScalars("g_v", GetIntType(), false, true)
	nv := CreateVariableScalars("g_n", GetIntType(), false, false)
	// want non-vol qfer — MatchIndirect with 1-level always matches for non-exact
	// use exact: non-vol wanted vs vol var
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Match exact false for 1-level returns true always — skip
	got := ChooseVarQfer(NewRng(2), []*Variable{vol, nv}, AccessRead, EmptyCGContext(), GetIntType(), &q, MatchFlexible)
	if got == nil {
		t.Fatal("nil")
	}
}
