package csmith

import "testing"

func TestStructDepthNested(t *testing.T) {
	inner := &Type{isStruct: true, StructName: "S1", Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	outer := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: inner, BitWidth: -1},
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if inner.StructDepthSess(testAmbientSession) != 1 {
		t.Fatalf("inner %d", inner.StructDepthSess(testAmbientSession))
	}
	if outer.StructDepthSess(testAmbientSession) != 2 {
		t.Fatalf("outer %d", outer.StructDepthSess(testAmbientSession))
	}
	if GetIntTypeSess(testAmbientSession).StructDepthSess(testAmbientSession) != 0 {
		t.Fatal("int")
	}
	// nil field Type: fail closed deep (no invent depth 0 past hole)
	ClearErrorSess(testAmbientSession)
	hole := &Type{isStruct: true, StructName: "Shole", Fields: []StructField{
		{Type: nil, BitWidth: -1},
	}}
	if hole.StructDepthSess(testAmbientSession) != incompleteStructDepth {
		t.Fatalf("incomplete depth %d want %d", hole.StructDepthSess(testAmbientSession), incompleteStructDepth)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field Type StructDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested residual: Type-nil deeper field soft invent was soft-continue later siblings.
	// Fair: sticky incompleteStructDepth.
	nestedHole := &Type{isStruct: true, StructName: "Snest", Fields: []StructField{
		{Type: hole, BitWidth: -1},
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if nestedHole.StructDepthSess(testAmbientSession) != incompleteStructDepth {
		t.Fatalf("nested residual depth %d want %d", nestedHole.StructDepthSess(testAmbientSession), incompleteStructDepth)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual StructDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested filter treats incomplete as too deep
	opts := Defaults()
	opts.MaxNestedStructLevel = 3
	if hole.StructDepthSess(testAmbientSession) < opts.MaxNestedStructLevel {
		t.Fatal("incomplete must fail closed over max nested")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomTypeFilterNoReturnUnionsGate(t *testing.T) {
	// Type.cpp:223–244 ChooseRandomTypeFilter — only !return_structs rejects structs;
	// return_unions is not a ChooseRandom gate (unlike NonVoidNonVolatile arg_unions).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ReturnUnions = false
	opts.ReturnStructs = true
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	// only union + void-weight simple so choose must accept union
	u := &Type{isUnion: true, StructName: "U0", Used: false}
	env.AllTypes = []*Type{u}
	env.UnionTypes = []*Type{u}
	r := NewRngSess(testAmbientSession, 2)
	found := false
	for i := 0; i < 20; i++ {
		ty := env.ChooseRandom(r, opts, probs, false)
		if ty != nil && ty.IsUnionSess(testAmbientSession) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ChooseRandom must not invent return_unions filter-out of unions")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOkStructUnionSkipsVolatile(t *testing.T) {
	// suite hygiene: prior sticky tests leave residual ERROR; clear before complete filter
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession}
	okt := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}), BitWidth: -1},
	}}
	volt := &Type{isStruct: true, StructName: "S1", Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{true}), BitWidth: -1},
	}}
	env.StructTypes = []*Type{okt, volt}
	cands := okStructUnionLTypes(env, true, true, false)
	if len(cands) != 1 || cands[0] != okt {
		t.Fatalf("%v", cands)
	}
	// nil hole must IncompleteTypes — not bare nil invent empty-complete keep-typ
	ClearErrorSess(testAmbientSession)
	env.StructTypes = []*Type{okt, nil}
	bad := okStructUnionLTypes(env, true, true, false)
	if typesComplete(bad) {
		t.Fatal("StructTypes hole must IncompleteTypes")
	}
	if chooseRandomStructFromType(env, GetIntTypeSess(testAmbientSession), true, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("incomplete ok pool must fail closed nil, not invent keep original")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ok pool must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVolRValEmit(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, true)
	v.UseVolRVal = true
	out := v.OutputCSess(testAmbientSession, false)
	if out != "VOL_RVAL(g_1, int32_t)" {
		t.Fatal(out)
	}
}
