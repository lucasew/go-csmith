package csmith

import "testing"

func TestIsTmpVar(t *testing.T) {
	if !(&Variable{Name: "t_1"}).IsTmpVar() {
		t.Fatal("t_")
	}
	if (&Variable{Name: "g_1"}).IsTmpVar() {
		t.Fatal("g_")
	}
}

func TestIsValidVolatile(t *testing.T) {
	// non-const always ok
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if !v.IsValidVolatile() {
		t.Fatal("non-const")
	}
	// const null pointer invalid
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	p.Qfer.SetConst(true, 0)
	p.Init = MakeInt(0)
	if !p.IsConst() {
		t.Fatal("expected const")
	}
	if p.IsValidVolatile() {
		t.Fatal("const null ptr should be invalid volatile")
	}
	// const non-null ok
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), true, false)
	p2.Qfer.SetConst(true, 0)
	p2.Init = MakeInt(1)
	if !p2.IsValidVolatile() {
		t.Fatal("const non-null")
	}
}

func TestIsPackedAfterBitfield(t *testing.T) {
	st := &Type{
		isStruct: true,
		Packed:   true,
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: 3},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	parent := &Variable{Name: "g_s", Type: st}
	f0 := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent, IsBitfield: true}
	f1 := &Variable{Name: "g_s.f1", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	if f0.IsPackedAfterBitfield() {
		t.Fatal("first field not after bitfield")
	}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("f1 after bitfield in packed struct")
	}
	// incomplete FieldVars hole before f1: fail closed packed-after (restrictive)
	parent.FieldVars = []*Variable{f0, nil, f1}
	if !f1.IsPackedAfterBitfield() {
		t.Fatal("FieldVars hole must fail closed as packed-after-bitfield")
	}
}

func TestGetSeqNum(t *testing.T) {
	// Variable.cpp:261–265 — assert '_' present
	v := CreateVariableScalars("g_42", GetIntType(), false, false)
	if v.GetSeqNum() != 42 {
		t.Fatal(v.GetSeqNum())
	}
	if (&Variable{Name: "badname"}).GetSeqNum() != -1 {
		t.Fatal("no underscore fail closed")
	}
}

func TestGetCollectiveArrayField(t *testing.T) {
	// Variable.cpp:583–612 — field of itemized array maps to collective field
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVars()
	if len(parent.FieldVars) == 0 {
		t.Fatal("fields")
	}
	item := parent.ItemizeConstIndices([]int{1}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	item.CreateFieldVars()
	if len(item.FieldVars) == 0 {
		t.Fatal("item fields")
	}
	// itemized field collective should be parent field
	got := item.FieldVars[0].GetCollective()
	if got != parent.FieldVars[0] {
		t.Fatalf("want parent field, got %v", got)
	}
}

func TestIsArrayField(t *testing.T) {
	arr := CreateVariableScalars("g_a", GetIntType(), true, false)
	arr.IsArray = true
	field := &Variable{Name: "g_a[0].f0", Type: GetIntType(), FieldVarOf: arr}
	if !field.IsArrayField() {
		t.Fatal("array field")
	}
}
