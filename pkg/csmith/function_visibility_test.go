package csmith

import "testing"


// TestIsVarOOSLocalAggregateField — Function.cpp:214–224.
// find_variable_in_set(local_vars) is pointer identity only. A field of a
// stack aggregate is not itself in local_vars → not OOS (even when Match would
// hit the parent). Seed 86: IsVarOOS(l_1053.f3) invent-true via Match then
// mark_dead on l_1226=&l_1053.f3.
func TestIsVarOOSLocalAggregateField(t *testing.T) {
	ClearError()
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f3", Type: GetSimpleType(EShort), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	f := &Function{Name: "func_30", ReturnType: GetIntType()}
	body := &Block{Func: f, StmID: 463}
	f.Blocks = []*Block{body}
	f.Stack = []*Block{body}
	arr := CreateVariableScalars("l_1053", st, false, false)
	arr.CreateFieldVars()
	if len(arr.FieldVars) < 2 {
		t.Fatalf("fields %d err=%v", len(arr.FieldVars), GetError())
	}
	body.LocalVars = []*Variable{arr}
	field := arr.FieldVars[1] // f3
	// Parent is on stack at body
	if !f.IsVarOnStack(arr, body) {
		t.Fatal("array must be on stack")
	}
	// Field is not on stack by identity (Function.cpp:194 find_variable_in_set)
	if f.IsVarOnStack(field, body) {
		t.Fatal("field must not be on-stack by identity")
	}
	// Field is not visible (not global, not on-stack identity)
	if f.IsVarVisible(field, body) {
		t.Fatal("field not visible by identity stack walk")
	}
	// Field must NOT be OOS: C++ only finds pointer identity in local_vars
	if f.IsVarOOS(field, body) {
		t.Fatal("field of stack aggregate must not be IsVarOOS (no Match invent)")
	}
	if HasError() {
		t.Fatal(GetError())
	}
	// UpdateFactsForDest must not mark-dead pointers to the field
	ptr := CreateVariableScalars("l_1226", PointerTo(GetSimpleType(EShort)), false, false)
	factsIn := []*FactPointTo{MakeFactPointTo(ptr, field)}
	factsOut := []*FactPointTo{}
	UpdateFactsForDest(factsIn, &factsOut, f, body)
	if HasError() {
		t.Fatal(GetError())
	}
	got := FindRelatedPointTo(factsOut, ptr)
	if got == nil {
		t.Fatal("pointer fact missing after dest update")
	}
	if got.IsDead() {
		t.Fatalf("pointees to in-scope aggregate field must not be garbage: %+v", got.PointTo)
	}
	ClearError()
}
