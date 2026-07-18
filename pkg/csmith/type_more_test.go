package csmith

import (
	"strings"
	"testing"
)

func TestGetTypeFromString(t *testing.T) {
	if GetTypeFromString("Int") != GetIntType() {
		t.Fatal("Int")
	}
	if GetTypeFromString("Void").Simple() != EVoid {
		t.Fatal("Void")
	}
	if GetTypeFromString("ULonglong").Simple() != EULongLong {
		t.Fatal("ULonglong")
	}
	if GetTypeFromString("Nope") != nil {
		t.Fatal("unknown")
	}
	if GetIntType().TypeNameString() != "Int" {
		t.Fatal(GetIntType().TypeNameString())
	}
}

func TestSignedOverflowPossible(t *testing.T) {
	SetPlatformSizes(4, 8)
	defer SetPlatformSizes(4, 8)
	// char size 1 < int 4
	if GetSimpleType(EChar).SignedOverflowPossible(4) {
		t.Fatal("char")
	}
	if !GetIntType().SignedOverflowPossible(4) {
		t.Fatal("int")
	}
	if GetSimpleType(EUInt).SignedOverflowPossible(4) {
		t.Fatal("unsigned")
	}
}

func TestPrintfDirective(t *testing.T) {
	// pin platform so int is 4-byte (Generate may leave host int size)
	SetPlatformSizes(4, 8)
	defer SetPlatformSizes(4, 8)
	if GetIntType().PrintfDirective() != "%d" {
		t.Fatal(GetIntType().PrintfDirective())
	}
	if GetSimpleType(EUInt).PrintfDirective() != "%u" {
		t.Fatal("uint")
	}
	if GetSimpleType(ELongLong).PrintfDirective() != "%lld" {
		t.Fatal("ll")
	}
	if PointerTo(GetIntType()).PrintfDirective() != "0x%0x" {
		t.Fatal("ptr")
	}
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
		{Name: "f1", Type: GetSimpleType(EUInt)},
	}}
	pd := st.PrintfDirective()
	if !strings.Contains(pd, "%d") || !strings.Contains(pd, "%u") {
		t.Fatal(pd)
	}
}

func TestSizeofString(t *testing.T) {
	if GetIntType().SizeofString() != "sizeof(int)" {
		t.Fatal(GetIntType().SizeofString())
	}
}

func TestHasAggregateAndLongLongField(t *testing.T) {
	fields := []StructField{
		{Name: "f0", Type: GetIntType()},
		{Name: "f1", Type: GetSimpleType(ELongLong)},
	}
	if HasAggregateField(fields) {
		t.Fatal("no aggregate")
	}
	if !HasLongLongField(fields) {
		t.Fatal("ll")
	}
	fields = append(fields, StructField{Name: "f2", Type: &Type{isStruct: true}})
	if !HasAggregateField(fields) {
		t.Fatal("agg")
	}
}

func TestIsUnnamedPadding(t *testing.T) {
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: 0},
		{Name: "f1", Type: GetIntType(), BitWidth: 3},
	}}
	if !st.IsUnnamedPadding(0) {
		t.Fatal("pad")
	}
	if st.IsUnnamedPadding(1) {
		t.Fatal("named bitfield")
	}
}

func TestGetAllOKStructUnionTypes(t *testing.T) {
	env := &TypeEnv{}
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
	}}
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType()},
	}}
	env.AllTypes = []*Type{GetIntType(), st, ut}
	structs := env.GetAllOKStructUnionTypes(false, false, true, true)
	if len(structs) != 1 || structs[0] != st {
		t.Fatal(structs)
	}
	unions := env.GetAllOKStructUnionTypes(false, false, true, false)
	if len(unions) != 1 || unions[0] != ut {
		t.Fatal(unions)
	}
	if env.FindType(st) != st {
		t.Fatal("find")
	}
	if env.FindType(GetSimpleType(EChar)) != nil {
		t.Fatal("not in AllTypes")
	}
}

func TestChooseRandomStructFromType(t *testing.T) {
	env := &TypeEnv{}
	st := &Type{isStruct: true, StructName: "S0"}
	env.AllTypes = []*Type{st}
	if env.ChooseRandomStructFromType(NewRng(1), st, false) != st {
		t.Fatal("same")
	}
	got := ChooseRandomStructUnionType(NewRng(2), []*Type{st})
	if got != st {
		t.Fatal(got)
	}
}

func TestIfStructAssignOps(t *testing.T) {
	opts := Defaults()
	if IfStructWillHaveAssignOps(NewRng(1), opts, NewProbabilities(opts)) {
		t.Fatal("C mode false")
	}
	opts.LangCPP = true
	// may or may not flip — just exercise
	_ = IfUnionWillHaveAssignOps(NewRng(3), opts, NewProbabilities(opts))
}
