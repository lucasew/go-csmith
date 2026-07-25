package csmith

import "testing"

// TestIsVarOOSLocalAggregateField — Function.cpp:187–224 / find_variable_in_set.
// C++ find_variable_in_set uses Variable::match: a field of a stack aggregate
// matches the parent in local_vars → is_var_on_stack true (not OOS). Soft invent
// was pointer-identity only (field never in local_vars) then wrong OOS/mark_dead.
// 5b8ae90: IsVarOnStack uses Match like find_variable_in_set.
func TestIsVarOOSLocalAggregateField(t *testing.T) {
	ReinstallTestProcessSingletons()
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f3", Type: GetSimpleTypeSess(testAmbientSession, EShort), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	f := &Function{Name: "func_30", ReturnType: GetIntTypeSess(testAmbientSession)}
	body := &Block{Func: f, StmID: 463}
	f.Blocks = []*Block{body}
	f.Stack = []*Block{body}
	arr := CreateVariableScalarsSess(testAmbientSession, "l_1053", st, false, false)
	if arr == nil {
		t.Fatalf("CreateVariableScalars nil err=%v", GetErrorSess(testAmbientSession))
	}
	arr.CreateFieldVarsSess(testAmbientSession)
	if len(arr.FieldVars) < 2 {
		t.Fatalf("fields %d err=%v", len(arr.FieldVars), GetErrorSess(testAmbientSession))
	}
	body.LocalVars = []*Variable{arr}
	field := arr.FieldVars[1] // f3
	// Parent is on stack at body
	if !f.IsVarOnStackSess(testAmbientSession, arr, body) {
		t.Fatal("array must be on stack")
	}
	// Field of stack aggregate: Match(parent) in local_vars → on-stack
	if !f.IsVarOnStackSess(testAmbientSession, field, body) {
		t.Fatal("field of stack aggregate must be IsVarOnStack via Match")
	}
	// Visible when on-stack
	if !f.IsVarVisibleSess(testAmbientSession, field, body) {
		t.Fatal("on-stack field must be visible")
	}
	// Not OOS while parent remains on stack
	if f.IsVarOOSSess(testAmbientSession, field, body) {
		t.Fatal("field of live stack aggregate must not be IsVarOOS")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	// UpdateFactsForDest must not mark-dead pointers to the field
	ptr := CreateVariableScalarsSess(testAmbientSession, "l_1226", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	factsIn := []*FactPointTo{MakeFactPointToSess(testAmbientSession, ptr, field)}
	factsOut := []*FactPointTo{}
	UpdateFactsForDest(factsIn, &factsOut, f, body)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	got := FindRelatedPointToSess(testAmbientSession, factsOut, ptr)
	if got == nil {
		t.Fatal("pointer fact missing after dest update")
	}
	if got.IsDeadSess(testAmbientSession) {
		t.Fatalf("pointees to in-scope aggregate field must not be garbage: %+v", got.PointTo)
	}
	ClearErrorSess(testAmbientSession)
}
