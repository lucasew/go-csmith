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
}

func TestIsArrayField(t *testing.T) {
	arr := CreateVariableScalars("g_a", GetIntType(), true, false)
	arr.IsArray = true
	field := &Variable{Name: "g_a[0].f0", Type: GetIntType(), FieldVarOf: arr}
	if !field.IsArrayField() {
		t.Fatal("array field")
	}
}
