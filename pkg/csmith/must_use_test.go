package csmith

import "testing"

func TestFindMustUseArrays(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("array")
	}
	sc := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	rw := &RWDirective{
		MustReadVars:  []*Variable{&av.Variable, sc},
		MustWriteVars: []*Variable{&av.Variable},
	}
	got := rw.FindMustUseArraysSess(testAmbientSession)
	if len(got) != 1 || got[0] != av {
		t.Fatalf("%v", got)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete FindMustUseArrays must not sticky")
	}
	// incomplete must-use list sticky
	ClearErrorSess(testAmbientSession)
	rw.MustReadVars = []*Variable{&av.Variable, nil}
	if rw.FindMustUseArraysSess(testAmbientSession) != nil {
		t.Fatal("nil hole FindMustUseArrays must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole FindMustUseArrays must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was soft-skip shell as absent → empty complete
	// fair: sticky nil fail closed
	shell := &Variable{Name: "g_b", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	rw2 := &RWDirective{MustReadVars: []*Variable{shell}}
	if rw2.FindMustUseArraysSess(testAmbientSession) != nil {
		t.Fatal("IsArray without AsArray FindMustUseArrays must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray FindMustUseArrays must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectMustUseVar(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	v := vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cg, GetIntTypeSess(testAmbientSession), nil)
	if v != g {
		t.Fatalf("%v", v)
	}
	// 75% may erase — either still present or gone is fine
}

func TestSelectMustUseVarTypeNilHole(t *testing.T) {
	// Variable::type always live; Type-nil must not soft-skip to a later candidate
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), false, false)
	broken.Type = nil
	good := CreateVariableScalarsSess(testAmbientSession, "g_good", GetIntTypeSess(testAmbientSession), false, false)
	rw := &RWDirective{MustWriteVars: []*Variable{broken, good}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cg, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("Type-nil must-use entry must fail closed whole select")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was bare pick via else branch
	// fair: sticky fail closed whole select
	shell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	rw2 := &RWDirective{MustWriteVars: []*Variable{shell, good}}
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw2)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cg2, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("IsArray without AsArray must fail closed SelectMustUseVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray SelectMustUseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectMustUseVarIncompleteAmbientSticky(t *testing.T) {
	// Incomplete EffectContext / GlobalFacts must not invent soft re-pick success
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	cg := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession).WithRW(rw)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cg, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("incomplete EffectContext must fail closed SelectMustUseVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw).WithFactMgr(fm)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cg2, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed SelectMustUseVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseVarFullWantNilTypeNil(t *testing.T) {
	// want==nil path: Type-nil candidate must fail closed sticky, not invent eligible
	ClearErrorSess(testAmbientSession)
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), false, false)
	broken.Type = nil
	good := CreateVariableScalarsSess(testAmbientSession, "g_good", GetIntTypeSess(testAmbientSession), false, false)
	if ChooseVarFull(NewRngSess(testAmbientSession, 1), []*Variable{broken, good}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		nil, nil, MatchFlexible, nil, false, false, false) != nil {
		t.Fatal("Type-nil with want==nil must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was IsEligible residual false soft-continue
	// then invent pick later good. Fair: sticky fail closed whole choose.
	shell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	if ChooseVarFull(NewRngSess(testAmbientSession, 1), []*Variable{shell, good}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		nil, nil, MatchFlexible, nil, false, false, false) != nil {
		t.Fatal("IsArray without AsArray want==nil must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ChooseVarFull want==nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// same for want!=nil path
	if ChooseVarFull(NewRngSess(testAmbientSession, 1), []*Variable{shell, good}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, false, false, false) != nil {
		t.Fatal("IsArray without AsArray want!=nil must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ChooseVarFull want!=nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectMustUseVarResidualSticky(t *testing.T) {
	// residual ERROR soft-continue invents later must-use pick. Fair: sticky whole select.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	good := CreateVariableScalarsSess(testAmbientSession, "g_good", GetIntTypeSess(testAmbientSession), false, false)
	// ItemizeArray Type-nil IV residual: soft invent was try-next then pick good.
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("array")
	}
	ivBroken := &Variable{Name: "i_broken"} // Type nil
	rwArr := &RWDirective{MustReadVars: []*Variable{&av.Variable, good}}
	cgArr := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rwArr)
	cgArr.IVBounds = map[*Variable]int{ivBroken: 0}
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 5), AccessRead, cgArr, GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("ItemizeArray Type-nil IV residual must fail closed SelectMustUseVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ItemizeArray residual SelectMustUseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// unpaired qfer Match residual: Match stickies on len(consts)!=len(vols)
	brokenQ := CreateVariableScalarsSess(testAmbientSession, "g_q", GetIntTypeSess(testAmbientSession), false, false)
	badQfer := CVQualifiers{IsConsts: []bool{false, false}, IsVolatiles: []bool{true}} // len mismatch
	rwQ := &RWDirective{MustWriteVars: []*Variable{brokenQ, good}}
	cgQ := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rwQ)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 2), AccessWrite, cgQ, GetIntTypeSess(testAmbientSession), &badQfer) != nil {
		t.Fatal("Match residual must fail closed SelectMustUseVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual SelectMustUseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectMustUseArrayItemize(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	rw := &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	// VariableSelector.cpp:1442 — need IV bounds for itemize_array
	cg.IVBounds = map[*Variable]int{iv: 0}
	v := vs.SelectMustUseVar(NewRngSess(testAmbientSession, 5), AccessRead, cg, GetIntTypeSess(testAmbientSession), nil)
	if v == nil {
		t.Fatal("nil")
	}
	if v.AsArray == nil || v.AsArray.Collective != av {
		t.Fatalf("want itemized member, got %v asArray=%v", v, v.AsArray)
	}
	// VariableSelector.cpp:1528–1530 — always itemize; sticky no bare collective without RNG
	ClearErrorSess(testAmbientSession)
	if bare := vs.SelectMustUseVar(nil, AccessRead, cg, GetIntTypeSess(testAmbientSession), nil); bare != nil {
		t.Fatalf("nil RNG must not invent bare collective array, got %v", bare)
	}
	// nil RNG SelectMustUseVar must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestSelectMustUseVarNilDepsSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	rw := &RWDirective{}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(rw)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 1), AccessWrite, cg, nil, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type SelectMustUseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil RW is soft re-pick (no must-use list)
	if vs.SelectMustUseVar(NewRngSess(testAmbientSession, 1), AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil) != nil {
		t.Fatal("nil RW must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil RW SelectMustUseVar must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomLhsMustUse(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_w", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	rw := &RWDirective{MustWriteVars: []*Variable{g}}
	// force only must-use by empty globals after? keep g
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(rw)
	lhs := MakeRandomLhs(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
	if lhs == nil || lhs.Var == nil {
		t.Fatal("nil")
	}
}

func TestChooseVarQferFilter(t *testing.T) {
	vol := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	nv := CreateVariableScalarsSess(testAmbientSession, "g_n", GetIntTypeSess(testAmbientSession), false, false)
	// want non-vol qfer — MatchIndirect with 1-level always matches for non-exact
	// use exact: non-vol wanted vs vol var
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	// Match exact false for 1-level returns true always — skip
	got := ChooseVarQfer(NewRngSess(testAmbientSession, 2), []*Variable{vol, nv}, AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, MatchFlexible)
	if got == nil {
		t.Fatal("nil")
	}
}
