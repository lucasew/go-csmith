package csmith

import "testing"

func TestSizeInBytesUsesPlatform(t *testing.T) {
	// Type.cpp:1497–1531 — simple sizes fixed; ePointer SizeInBytes returns 0
	SetPlatformSizesSess(testAmbientSession, 4, 8)
	if GetIntTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) != 4 {
		t.Fatal("int")
	}
	// Type.cpp:1568–1572 — pointer falls through to 0 (not invent platform width)
	if PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).SizeInBytesSess(testAmbientSession) != 0 {
		t.Fatal("ptr SizeInBytes must be 0 like C++")
	}
	// Type.cpp:1511–1522 — eLong/eULong always 4 (no invent host LP64 long==8)
	if GetSimpleTypeSess(testAmbientSession, ELong).SizeInBytesSess(testAmbientSession) != 4 {
		t.Fatal("long fixed 4")
	}
	if GetSimpleTypeSess(testAmbientSession, EULong).SizeInBytesSess(testAmbientSession) != 4 {
		t.Fatal("ulong fixed 4")
	}
	if GetSimpleTypeSess(testAmbientSession, ELongLong).SizeInBytesSess(testAmbientSession) != 8 {
		t.Fatal("long long 8")
	}
	if GetSimpleTypeSess(testAmbientSession, EInt128).SizeInBytesSess(testAmbientSession) != 16 {
		t.Fatal("int128 16")
	}
	// CName from SizeInBytes*8 — ulong → uint32_t not uint64_t
	if GetSimpleTypeSess(testAmbientSession, EULong).CNameSess(testAmbientSession) != "uint32_t" {
		t.Fatal(GetSimpleTypeSess(testAmbientSession, EULong).CNameSess(testAmbientSession))
	}
	SetPlatformSizesSess(testAmbientSession, 4, 4)
	// pointer still 0; currentSession().PlatformPtrSize only for other uses
	if PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).SizeInBytesSess(testAmbientSession) != 0 {
		t.Fatal("ptr still 0 on ILP32")
	}
	// long still 4 regardless of pointer size
	if GetSimpleTypeSess(testAmbientSession, ELong).SizeInBytesSess(testAmbientSession) != 4 {
		t.Fatal("long still 4 on ILP32 ptr")
	}
	// restore common default
	SetPlatformSizesSess(testAmbientSession, 4, 8)
}

func TestFieldVarsMarkBitfield(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: 5, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	v := CreateVariableQferSess(testAmbientSession, "g_1", st, NewCVQualifiers([]bool{false}, []bool{false}))
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
