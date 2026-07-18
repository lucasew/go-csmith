package csmith

import "testing"

func TestSizeInBytesUsesPlatform(t *testing.T) {
	SetPlatformSizes(4, 8)
	if GetIntType().SizeInBytes() != 4 {
		t.Fatal("int")
	}
	if PointerTo(GetIntType()).SizeInBytes() != 8 {
		t.Fatal("ptr")
	}
	if GetSimpleType(ELong).SizeInBytes() != 8 {
		t.Fatal("long LP64")
	}
	SetPlatformSizes(4, 4)
	if GetSimpleType(ELong).SizeInBytes() != 4 {
		t.Fatal("long ILP32")
	}
	// restore common default
	SetPlatformSizes(4, 8)
}

func TestFieldVarsMarkBitfield(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: GetIntType(), BitWidth: 5, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	v := CreateVariableQfer("g_1", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(v.FieldVars) != 2 {
		t.Fatalf("fields %d", len(v.FieldVars))
	}
	if !v.FieldVars[0].IsBitfield {
		t.Fatal("bf")
	}
	if v.FieldVars[1].IsBitfield {
		t.Fatal("not bf")
	}
}

func TestCompoundToBinaryOps(t *testing.T) {
	b, ok := AssignAdd.CompoundToBinaryOps()
	if !ok || b != BinAdd {
		t.Fatal(b, ok)
	}
	_, ok = AssignSimple.CompoundToBinaryOps()
	if ok {
		t.Fatal("simple")
	}
	b, ok = AssignBitAnd.CompoundToBinaryOps()
	if !ok || b != BinBitAnd {
		t.Fatal(b)
	}
}
