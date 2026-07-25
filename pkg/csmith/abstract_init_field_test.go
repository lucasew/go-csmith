package csmith

import "testing"

// TestAbstractFactForVarInitAddressOfItemizedField — Fact.cpp:85–95 + FactPointTo.cpp:202–207.
// Pointer local init &arr[i].field must abstract to a related FactPointTo (not nofact).
// Seed 86: int16_t *l_1226 = &l_1053[5][2][1].f3; re-visit IsValidPtr failed with nofact.
func TestAbstractFactForVarInitAddressOfItemizedField(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Process RNG for CreateFieldVars field Constant::make_random
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 1))

	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f1", Type: GetSimpleType(EULongLong), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f2", Type: GetSimpleType(EChar), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f3", Type: GetSimpleType(EShort), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	av := &ArrayVariable{
		Variable: Variable{Name: "l_1053", Type: st, IsArray: true, ArraySizes: []int{8, 5, 3}},
		Sizes:    []int{8, 5, 3},
	}
	av.AsArray = av
	// ArrayVariable.cpp:161–163 — collective create_field_vars for aggregate element
	av.CreateFieldVarsSess(testAmbientSession)
	if HasErrorSess(testAmbientSession) || len(av.FieldVars) < 4 {
		t.Fatalf("collective fields n=%d err=%v", len(av.FieldVars), GetErrorSess(testAmbientSession))
	}
	item := av.Itemize(NewRngSess(testAmbientSession, 2))
	if item == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("itemize", HasErrorSess(testAmbientSession))
	}
	if len(item.FieldVars) < 4 {
		t.Fatalf("item field vars %d", len(item.FieldVars))
	}
	f3 := item.FieldVars[3]
	// GetCollective of itemized field maps onto collective field
	collF3 := f3.GetCollectiveSess(testAmbientSession)
	if collF3 == nil || HasErrorSess(testAmbientSession) {
		t.Fatalf("GetCollective itemized field err=%v", GetErrorSess(testAmbientSession))
	}
	if collF3 != av.FieldVars[3] {
		t.Fatalf("collective field want %p %s got %p %s",
			av.FieldVars[3], av.FieldVars[3].Name, collF3, collF3.Name)
	}

	ptrType := PointerTo(GetSimpleType(EShort))
	init := &Expression{Term: TermVariable, Var: f3, ExprType: ptrType}
	if n, ok := init.IndirectLevelComplete(); !ok || n != -1 {
		t.Fatalf("want address-of -1 got n=%d ok=%v", n, ok)
	}
	p := CreateVariableScalarsSess(testAmbientSession, "l_1226", ptrType, false, false)
	p.InitExpr = init

	pt, _ := AbstractFactForVarInit(p)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("AbstractFactForVarInit sticky err=%v", GetErrorSess(testAmbientSession))
	}
	if !FactsComplete(pt) || len(pt) != 1 {
		t.Fatalf("want 1 complete fact complete=%v n=%d", FactsComplete(pt), len(pt))
	}
	rel := FindRelatedPointTo(pt, p)
	if rel == nil {
		t.Fatal("related fact missing after abstract")
	}
	if rel.IsNull() {
		t.Fatal("address-of field must not be null")
	}
	// pointee should be collective field (not itemized)
	if len(rel.PointTo) == 0 || rel.PointTo[0] != collF3 {
		t.Fatalf("want point-to collective f3, got %v", rel.PointTo)
	}

	facts := []*FactPointTo{}
	AddNewVarFactInto(p, &facts)
	if HasErrorSess(testAmbientSession) || !FactsComplete(facts) {
		t.Fatalf("AddNewVarFactInto err=%v complete=%v", GetErrorSess(testAmbientSession), FactsComplete(facts))
	}
	if FindRelatedPointTo(facts, p) == nil {
		t.Fatal("makeup nofact")
	}
	if !IsValidPtr(p, facts, 0, 0) {
		t.Fatalf("IsValidPtr after makeup err=%v", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}
