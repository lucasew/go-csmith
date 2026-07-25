package csmith

import "testing"

func TestIsWrittenFieldInheritsParent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EUInt)}
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 2), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	sv := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Fatal("fields")
	}
	e := EmptyEffect().WriteVarSess(testAmbientSession, sv)
	if !e.IsWrittenSess(testAmbientSession, sv.FieldVars[0]) {
		t.Fatal("field should inherit parent write")
	}
	if !e.IsWrittenPartiallySess(testAmbientSession, sv) {
		t.Fatal("partial")
	}
}

func TestFindAllNonArrayVisibleVarsNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(testAmbientSession, Defaults())
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{g, nil}
	if VariablesComplete(vs.FindAllNonArrayVisibleVars(nil)) {
		t.Fatal("nil GlobalList hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GlobalList hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(GetAllLocalVarsSess(testAmbientSession, &Block{LocalVars: []*Variable{g, nil}})) {
		t.Fatal("nil LocalVars hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil LocalVars hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was soft-skip as array-filtered → complete pool
	// fair: sticky IncompleteVariables
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	vs.GlobalList = []*Variable{g, shell}
	if VariablesComplete(vs.FindAllNonArrayVisibleVars(nil)) {
		t.Fatal("IsArray without AsArray must fail closed incomplete non-array pool")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray FindAllNonArrayVisibleVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// same for LocalVars
	if VariablesComplete(vs.FindAllNonArrayVisibleVars(&Block{LocalVars: []*Variable{shell}})) {
		t.Fatal("IsArray without AsArray LocalVars must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray local FindAllNonArrayVisibleVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsEligibleVarSEFreeVolatile(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	// SE-free: ok
	if !IsEligibleVar(v, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("se-free")
	}
	// non-SE-free: reject volatile
	if IsEligibleVar(v, 0, AccessRead, WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)) {
		t.Fatal("vol + se")
	}
}

func TestIsEligibleVarWrittenConflict(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	cg := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, a)).WithSession(testAmbientSession)
	if IsEligibleVar(a, 0, AccessRead, cg) {
		t.Fatal("written conflict")
	}
	if IsEligibleVar(a, 0, AccessWrite, cg) {
		t.Fatal("write written")
	}
}

func TestIsEligibleVarConstWrite(t *testing.T) {
	c := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	if IsEligibleVar(c, 0, AccessWrite, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("const write")
	}
	if !IsEligibleVar(c, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("const read ok")
	}
}

func TestFindAllVisibleVars(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	outer := &Block{}
	inner := &Block{Parent: outer}
	l1 := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	l2 := CreateVariableScalarsSess(testAmbientSession, "l_2", GetIntTypeSess(testAmbientSession), false, false)
	outer.LocalVars = []*Variable{l1}
	inner.LocalVars = []*Variable{l2}
	got := vs.FindAllVisibleVars(inner)
	if len(got) != 3 {
		t.Fatalf("want 3 got %d", len(got))
	}
	// params not included
	f := &Function{Param: []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)}}
	outer.Func = f
	na := vs.FindAllNonArrayVisibleVars(inner)
	// global + param + 2 locals
	if len(na) != 4 {
		t.Fatalf("nonarray want 4 got %d", len(na))
	}
}

func TestChooseVarSkipsIneligible(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	cg := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, a)).WithSession(testAmbientSession)
	// only b eligible
	got := ChooseVar(NewRngSess(testAmbientSession, 2), []*Variable{a, b}, AccessRead, cg, GetIntTypeSess(testAmbientSession), MatchFlexible)
	if got != b {
		t.Fatalf("got %v", got)
	}
}

func TestIsEligibleVarItemizedReadIndices(t *testing.T) {
	// VariableSelector.cpp:221–227 — itemized uses collective after read_indices
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	// eligible under empty context
	if !IsEligibleVar(&item.Variable, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("itemized read")
	}
	// IV written in context → read_indices fails → not eligible
	cg := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, iv)).WithSession(testAmbientSession)
	if IsEligibleVar(&item.Variable, 0, AccessRead, cg) {
		t.Fatal("want reject when index IV written")
	}
	// collective itself written → reject after coll switch
	cg2 := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, &parent.Variable)).WithSession(testAmbientSession)
	if IsEligibleVar(&item.Variable, 0, AccessRead, cg2) {
		t.Fatal("collective written")
	}
}

func TestIsEligibleVarIncompleteCollectiveFailClosed(t *testing.T) {
	// incomplete FieldVars → GetCollective nil must not invent eligible / panic
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVarsSess(testAmbientSession)
	item := parent.ItemizeConstIndices([]int{0}, NewVariableSelector(testAmbientSession, Defaults()))
	if item == nil {
		t.Fatal("itemize")
	}
	item.CreateFieldVarsSess(testAmbientSession)
	if len(item.FieldVars) == 0 {
		t.Fatal("fields")
	}
	fld := item.FieldVars[0]
	item.FieldVars = append(item.FieldVars, nil)
	if IsEligibleVar(fld, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("incomplete collective must fail closed not eligible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete collective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if fld.LooseMatchSess(testAmbientSession, fld) {
		t.Fatal("incomplete LooseMatch must fail closed false")
	}
}

func TestIsEligibleVarIncompleteEffectSticky(t *testing.T) {
	// Incomplete EffectContext must sticky (no invent soft-skip as absent re-pick)
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if IsEligibleVar(v, 0, AccessRead, WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)) {
		t.Fatal("incomplete EffectContext must fail closed not eligible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsEligibleVarTypeNilDerefResidualSticky(t *testing.T) {
	// Type-nil subject / Type-nil parent field residual: soft invent was eligible true past hole.
	// Fair: sticky fail closed not eligible.
	ClearErrorSess(testAmbientSession)
	broken := &Variable{Name: "g_p"} // Type nil
	if IsEligibleVar(broken, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("Type-nil subject must fail closed not eligible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil subject IsEligibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil parent field: IsNonreadableField residual under FM
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	f := &Function{Name: "f"}
	fm := NewFactMgrSess(testAmbientSession, f)
	// non-empty union facts so IsNonreadableField walks IsInsideUnionField (stickies residual)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	parent.Type = ut
	// Force Type-nil mid-walk: parent of field is parent with Type then clear after fact
	// IsInsideUnionField on Type-nil parent stickies residual true
	parent.Type = nil
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	// plant a related FactUnion so IsNonreadableField walks Imply / ancestry
	// (empty complete facts already nonreadable without residual walk)
	uv := &Variable{Name: "g_u2", Type: ut}
	fm.UnionFacts = []*FactUnion{{Var: uv, LastWrittenFID: 0}}
	if IsEligibleVar(field, 0, AccessRead, cg) {
		t.Fatal("Type-nil parent IsInsideUnionField residual must fail closed not eligible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent residual IsEligibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray GetCollective sticky not eligible
	shell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	if IsEligibleVar(shell, 0, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("IsArray without AsArray must fail closed not eligible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray IsEligibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectParentParamFallsBackLocal(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{}
	f.Stack = []*Block{blk}
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	v := vs.SelectParentParam(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 3), MatchFlexible)
	if v == nil {
		t.Fatal("nil")
	}
	if len(blk.LocalVars) == 0 && !v.IsGlobalSess(testAmbientSession) {
		// should have created local
		t.Log(v.Name)
	}
}

func TestChooseOKVarChooseVarFullIncompleteFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if ChooseOKVarSess(testAmbientSession, NewRngSess(testAmbientSession, 1), []*Variable{nil}) != nil {
		t.Fatal("incomplete list must fail closed ChooseOKVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ChooseOKVar incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if ChooseVarFull(NewRngSess(testAmbientSession, 2), []*Variable{g, nil}, AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, MatchExact, nil, false, false, false) != nil {
		t.Fatal("incomplete vars must fail closed ChooseVarFull")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ChooseVarFull incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseVarFullIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / GlobalFacts must not invent choose / soft re-pick past holes
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vars := []*Variable{g}
	if ChooseVarFull(NewRngSess(testAmbientSession, 1), vars, AccessRead, WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, MatchExact, nil, false, false, false) != nil {
		t.Fatal("incomplete EffectContext must fail closed ChooseVarFull")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if ChooseVarFull(NewRngSess(testAmbientSession, 2), vars, AccessRead, cg, GetIntTypeSess(testAmbientSession), nil, MatchExact, nil, false, false, false) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed ChooseVarFull")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	inc := IncompleteEffect()
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.EffectStm = inc
	if ChooseVarFull(NewRngSess(testAmbientSession, 3), vars, AccessRead, cg2, GetIntTypeSess(testAmbientSession), nil, MatchExact, nil, false, false, false) != nil {
		t.Fatal("incomplete EffectStm must fail closed ChooseVarFull")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeInitValueSanityCheckSticky(t *testing.T) {
	// assert(qf.sanity_check(t)) — incomplete/mismatched qfer fails closed sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// nil qfer
	if vs.MakeInitValue(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil qfer must fail closed MakeInitValue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil qfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// VariableSelector.cpp:838–839 assert simple != void sticky
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	if vs.MakeInitValue(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EVoid), &q, nil, NewRngSess(testAmbientSession, 2)) != nil {
		t.Fatal("void type must fail closed MakeInitValue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeInitValueIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / GlobalFacts fail closed sticky before const/pointer pick
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if vs.MakeInitValue(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeInitValue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if vs.MakeInitValue(AccessRead, cg2, GetIntTypeSess(testAmbientSession), &q, nil, NewRngSess(testAmbientSession, 2)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeInitValue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg3 := WithFunc(nil, IncompleteEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if vs.MakeInitValue(AccessRead, cg3, GetIntTypeSess(testAmbientSession), &q, nil, NewRngSess(testAmbientSession, 3)) != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeInitValue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
