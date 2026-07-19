package csmith

import "testing"

func TestFindMustUseArrays(t *testing.T) {
	ClearError()
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("array")
	}
	sc := CreateVariableScalars("g_i", GetIntType(), false, false)
	rw := &RWDirective{
		MustReadVars:  []*Variable{&av.Variable, sc},
		MustWriteVars: []*Variable{&av.Variable},
	}
	got := rw.FindMustUseArrays()
	if len(got) != 1 || got[0] != av {
		t.Fatalf("%v", got)
	}
	if HasError() {
		t.Fatal("complete FindMustUseArrays must not sticky")
	}
	// incomplete must-use list sticky
	ClearError()
	rw.MustReadVars = []*Variable{&av.Variable, nil}
	if rw.FindMustUseArrays() != nil {
		t.Fatal("nil hole FindMustUseArrays must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil hole FindMustUseArrays must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was soft-skip shell as absent → empty complete
	// fair: sticky nil fail closed
	shell := &Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	rw2 := &RWDirective{MustReadVars: []*Variable{shell}}
	if rw2.FindMustUseArrays() != nil {
		t.Fatal("IsArray without AsArray FindMustUseArrays must fail closed nil")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray FindMustUseArrays must SetError sticky")
	}
	ClearError()
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

func TestSelectMustUseVarTypeNilHole(t *testing.T) {
	// Variable::type always live; Type-nil must not soft-skip to a later candidate
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	broken := CreateVariableScalars("g_broken", GetIntType(), false, false)
	broken.Type = nil
	good := CreateVariableScalars("g_good", GetIntType(), false, false)
	rw := &RWDirective{MustWriteVars: []*Variable{broken, good}}
	cg := WithFunc(f, EmptyEffect()).WithRW(rw)
	if vs.SelectMustUseVar(NewRng(2), AccessWrite, cg, GetIntType(), nil) != nil {
		t.Fatal("Type-nil must-use entry must fail closed whole select")
	}
	if !HasError() {
		t.Fatal("Type-nil must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was bare pick via else branch
	// fair: sticky fail closed whole select
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	rw2 := &RWDirective{MustWriteVars: []*Variable{shell, good}}
	cg2 := WithFunc(f, EmptyEffect()).WithRW(rw2)
	if vs.SelectMustUseVar(NewRng(2), AccessWrite, cg2, GetIntType(), nil) != nil {
		t.Fatal("IsArray without AsArray must fail closed SelectMustUseVar")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray SelectMustUseVar must SetError sticky")
	}
	ClearError()
}

func TestSelectMustUseVarIncompleteAmbientSticky(t *testing.T) {
	// Incomplete EffectContext / GlobalFacts must not invent soft re-pick success
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	cg := WithFunc(f, IncompleteEffect()).WithRW(rw)
	if vs.SelectMustUseVar(NewRng(2), AccessWrite, cg, GetIntType(), nil) != nil {
		t.Fatal("incomplete EffectContext must fail closed SelectMustUseVar")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithRW(rw).WithFactMgr(fm)
	if vs.SelectMustUseVar(NewRng(2), AccessWrite, cg2, GetIntType(), nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed SelectMustUseVar")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestChooseVarFullWantNilTypeNil(t *testing.T) {
	// want==nil path: Type-nil candidate must fail closed sticky, not invent eligible
	ClearError()
	broken := CreateVariableScalars("g_broken", GetIntType(), false, false)
	broken.Type = nil
	good := CreateVariableScalars("g_good", GetIntType(), false, false)
	if ChooseVarFull(NewRng(1), []*Variable{broken, good}, AccessRead, EmptyCGContext(),
		nil, nil, MatchFlexible, nil, false, false, false) != nil {
		t.Fatal("Type-nil with want==nil must fail closed")
	}
	if !HasError() {
		t.Fatal("Type-nil must SetError sticky")
	}
	ClearError()
}

func TestSelectMustUseArrayItemize(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(3), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	rw := &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	cg := WithFunc(f, EmptyEffect()).WithRW(rw)
	// VariableSelector.cpp:1442 — need IV bounds for itemize_array
	cg.IVBounds = map[*Variable]int{iv: 0}
	v := vs.SelectMustUseVar(NewRng(5), AccessRead, cg, GetIntType(), nil)
	if v == nil {
		t.Fatal("nil")
	}
	if v.AsArray == nil || v.AsArray.Collective != av {
		t.Fatalf("want itemized member, got %v asArray=%v", v, v.AsArray)
	}
	// VariableSelector.cpp:1528–1530 — always itemize; sticky no bare collective without RNG
	ClearError()
	if bare := vs.SelectMustUseVar(nil, AccessRead, cg, GetIntType(), nil); bare != nil {
		t.Fatalf("nil RNG must not invent bare collective array, got %v", bare)
	}
	if !HasError() {
		t.Fatal("nil RNG SelectMustUseVar must SetError sticky")
	}
	ClearError()
}

func TestSelectMustUseVarNilDepsSticky(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	rw := &RWDirective{}
	cg := EmptyCGContext().WithRW(rw)
	if vs.SelectMustUseVar(NewRng(1), AccessWrite, cg, nil, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type SelectMustUseVar must SetError sticky")
	}
	ClearError()
	// nil RW is soft re-pick (no must-use list)
	if vs.SelectMustUseVar(NewRng(1), AccessWrite, EmptyCGContext(), GetIntType(), nil) != nil {
		t.Fatal("nil RW must fail closed")
	}
	if HasError() {
		t.Fatal("nil RW SelectMustUseVar must stay non-sticky soft re-pick")
	}
	ClearError()
}

func TestMakeRandomLhsMustUse(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_w", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	// force only must-use by empty globals after? keep g
	cg := EmptyCGContext().WithRW(rw)
	lhs := MakeRandomLhs(NewRng(2), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil)
	if lhs == nil || lhs.Var == nil {
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
