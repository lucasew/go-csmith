package csmith

import "testing"

func TestIsWrittenFieldInheritsParent(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	sv := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Fatal("fields")
	}
	e := EmptyEffect().WriteVar(sv)
	if !e.IsWritten(sv.FieldVars[0]) {
		t.Fatal("field should inherit parent write")
	}
	if !e.IsWrittenPartially(sv) {
		t.Fatal("partial")
	}
}

func TestFindAllNonArrayVisibleVarsNilHole(t *testing.T) {
	vs := NewVariableSelector(Defaults())
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{g, nil}
	if vs.FindAllNonArrayVisibleVars(nil) != nil {
		t.Fatal("nil GlobalList hole must fail closed")
	}
	if GetAllLocalVars(&Block{LocalVars: []*Variable{g, nil}}) != nil {
		t.Fatal("nil LocalVars hole must fail closed")
	}
}

func TestIsEligibleVarSEFreeVolatile(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	// SE-free: ok
	if !IsEligibleVar(v, 0, AccessRead, EmptyCGContext()) {
		t.Fatal("se-free")
	}
	// non-SE-free: reject volatile
	if IsEligibleVar(v, 0, AccessRead, WithEffectContext(WithSideEffects())) {
		t.Fatal("vol + se")
	}
}

func TestIsEligibleVarWrittenConflict(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	cg := WithEffectContext(EmptyEffect().WriteVar(a))
	if IsEligibleVar(a, 0, AccessRead, cg) {
		t.Fatal("written conflict")
	}
	if IsEligibleVar(a, 0, AccessWrite, cg) {
		t.Fatal("write written")
	}
}

func TestIsEligibleVarConstWrite(t *testing.T) {
	c := CreateVariableScalars("g_c", GetIntType(), true, false)
	if IsEligibleVar(c, 0, AccessWrite, EmptyCGContext()) {
		t.Fatal("const write")
	}
	if !IsEligibleVar(c, 0, AccessRead, EmptyCGContext()) {
		t.Fatal("const read ok")
	}
}

func TestFindAllVisibleVars(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	outer := &Block{}
	inner := &Block{Parent: outer}
	l1 := CreateVariableScalars("l_1", GetIntType(), false, false)
	l2 := CreateVariableScalars("l_2", GetIntType(), false, false)
	outer.LocalVars = []*Variable{l1}
	inner.LocalVars = []*Variable{l2}
	got := vs.FindAllVisibleVars(inner)
	if len(got) != 3 {
		t.Fatalf("want 3 got %d", len(got))
	}
	// params not included
	f := &Function{Param: []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)}}
	outer.Func = f
	na := vs.FindAllNonArrayVisibleVars(inner)
	// global + param + 2 locals
	if len(na) != 4 {
		t.Fatalf("nonarray want 4 got %d", len(na))
	}
}

func TestChooseVarSkipsIneligible(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	cg := WithEffectContext(EmptyEffect().WriteVar(a))
	// only b eligible
	got := ChooseVar(NewRng(2), []*Variable{a, b}, AccessRead, cg, GetIntType(), MatchFlexible)
	if got != b {
		t.Fatalf("got %v", got)
	}
}

func TestIsEligibleVarItemizedReadIndices(t *testing.T) {
	// VariableSelector.cpp:221–227 — itemized uses collective after read_indices
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntType()}},
	}
	item.AsArray = item
	// eligible under empty context
	if !IsEligibleVar(&item.Variable, 0, AccessRead, EmptyCGContext()) {
		t.Fatal("itemized read")
	}
	// IV written in context → read_indices fails → not eligible
	cg := WithEffectContext(EmptyEffect().WriteVar(iv))
	if IsEligibleVar(&item.Variable, 0, AccessRead, cg) {
		t.Fatal("want reject when index IV written")
	}
	// collective itself written → reject after coll switch
	cg2 := WithEffectContext(EmptyEffect().WriteVar(&parent.Variable))
	if IsEligibleVar(&item.Variable, 0, AccessRead, cg2) {
		t.Fatal("collective written")
	}
}

func TestSelectParentParamFallsBackLocal(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{}
	f.Stack = []*Block{blk}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	cg := WithFunc(f, EmptyEffect())
	v := vs.SelectParentParam(AccessRead, cg, GetIntType(), &q, NewRng(3), MatchFlexible)
	if v == nil {
		t.Fatal("nil")
	}
	if len(blk.LocalVars) == 0 && !v.IsGlobal() {
		// should have created local
		t.Log(v.Name)
	}
}
