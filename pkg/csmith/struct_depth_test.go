package csmith

import "testing"

func TestStructDepthNested(t *testing.T) {
	inner := &Type{isStruct: true, StructName: "S1", Fields: []StructField{
		{Type: GetIntType(), BitWidth: -1},
	}}
	outer := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: inner, BitWidth: -1},
		{Type: GetIntType(), BitWidth: -1},
	}}
	if inner.StructDepth() != 1 {
		t.Fatalf("inner %d", inner.StructDepth())
	}
	if outer.StructDepth() != 2 {
		t.Fatalf("outer %d", outer.StructDepth())
	}
	if GetIntType().StructDepth() != 0 {
		t.Fatal("int")
	}
	// nil field Type: fail closed deep (no invent depth 0 past hole)
	ClearError()
	hole := &Type{isStruct: true, StructName: "Shole", Fields: []StructField{
		{Type: nil, BitWidth: -1},
	}}
	if hole.StructDepth() != incompleteStructDepth {
		t.Fatalf("incomplete depth %d want %d", hole.StructDepth(), incompleteStructDepth)
	}
	if !HasError() {
		t.Fatal("nil field Type StructDepth must SetError sticky")
	}
	ClearError()
	// nested residual: Type-nil deeper field soft invent was soft-continue later siblings.
	// Fair: sticky incompleteStructDepth.
	nestedHole := &Type{isStruct: true, StructName: "Snest", Fields: []StructField{
		{Type: hole, BitWidth: -1},
		{Type: GetIntType(), BitWidth: -1},
	}}
	if nestedHole.StructDepth() != incompleteStructDepth {
		t.Fatalf("nested residual depth %d want %d", nestedHole.StructDepth(), incompleteStructDepth)
	}
	if !HasError() {
		t.Fatal("nested residual StructDepth must SetError sticky")
	}
	ClearError()
	// nested filter treats incomplete as too deep
	opts := Defaults()
	opts.MaxNestedStructLevel = 3
	if hole.StructDepth() < opts.MaxNestedStructLevel {
		t.Fatal("incomplete must fail closed over max nested")
	}
	ClearError()
}

func TestChooseRandomFiltersReturnUnions(t *testing.T) {
	opts := Defaults()
	opts.ReturnUnions = false
	opts.ReturnStructs = true
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	// seed types manually
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort)}
	u := &Type{isUnion: true, StructName: "U0"}
	env.AllTypes = append(env.AllTypes, u)
	env.UnionTypes = []*Type{u}
	r := NewRng(2)
	for i := 0; i < 40; i++ {
		ty := env.ChooseRandom(r, opts, probs, false)
		if ty != nil && ty.IsUnion() {
			t.Fatal("union returned when ReturnUnions false")
		}
	}
}

func TestOkStructUnionSkipsVolatile(t *testing.T) {
	// suite hygiene: prior sticky tests leave residual ERROR; clear before complete filter
	ClearError()
	env := &TypeEnv{}
	okt := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false}), BitWidth: -1},
	}}
	volt := &Type{isStruct: true, StructName: "S1", Fields: []StructField{
		{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{true}), BitWidth: -1},
	}}
	env.StructTypes = []*Type{okt, volt}
	cands := okStructUnionLTypes(env, true, true, false)
	if len(cands) != 1 || cands[0] != okt {
		t.Fatalf("%v", cands)
	}
	// nil hole must IncompleteTypes — not bare nil invent empty-complete keep-typ
	ClearError()
	env.StructTypes = []*Type{okt, nil}
	bad := okStructUnionLTypes(env, true, true, false)
	if typesComplete(bad) {
		t.Fatal("StructTypes hole must IncompleteTypes")
	}
	if chooseRandomStructFromType(env, GetIntType(), true, NewRng(1)) != nil {
		t.Fatal("incomplete ok pool must fail closed nil, not invent keep original")
	}
	if !HasError() {
		t.Fatal("incomplete ok pool must SetError sticky")
	}
	ClearError()
}

func TestVolRValEmit(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, true)
	v.UseVolRVal = true
	out := v.OutputC()
	if out != "VOL_RVAL(g_1, int32_t)" {
		t.Fatal(out)
	}
}
