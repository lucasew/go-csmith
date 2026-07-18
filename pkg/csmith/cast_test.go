package csmith

import (
	"strings"
	"testing"
)

func TestIsEquivalentSameSize(t *testing.T) {
	// int and long may same size on LP64 both 8? our SizeInBytes: int 4 long 4/8
	if !GetIntType().IsEquivalent(GetIntType()) {
		t.Fatal("int~int")
	}
	if GetSimpleType(EChar).IsEquivalent(GetIntType()) {
		t.Fatal("char!~int")
	}
}

func TestNeedsCastPointerBases(t *testing.T) {
	pi := PointerTo(GetIntType())
	pc := PointerTo(GetSimpleType(EChar))
	if !pi.NeedsCast(pc) && pi.BaseType().SizeInBytes() != pc.BaseType().SizeInBytes() {
		// needs cast when bases inequivalent
	}
	if pi.BaseType().SizeInBytes() != pc.BaseType().SizeInBytes() {
		if !pi.NeedsCast(pc) {
			t.Fatal("int* needs cast from char*")
		}
	}
	if pi.NeedsCast(pi) {
		t.Fatal("same no cast")
	}
}

func TestExpressionCastOutput(t *testing.T) {
	e := &Expression{
		Term:     TermConstant,
		Con:      MakeInt(0),
		CastType: PointerTo(GetIntType()),
	}
	out := e.Output()
	if !strings.Contains(out, "int*") || !strings.HasPrefix(out, "(") {
		t.Fatal(out)
	}
}

func TestHasBitfields(t *testing.T) {
	st := &Type{isStruct: true, Fields: []StructField{
		{Type: GetIntType(), BitWidth: 3},
	}}
	if !st.HasBitfields() {
		t.Fatal("bf")
	}
	st2 := &Type{isStruct: true, Fields: []StructField{
		{Type: GetIntType(), BitWidth: -1},
	}}
	if st2.HasBitfields() {
		t.Fatal("no bf")
	}
}

func TestCheckAndSetCast(t *testing.T) {
	// int* expr desired as char* → needs cast if sizes differ
	v := CreateVariableQfer("g_1", PointerTo(GetIntType()), NewCVQualifiers([]bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerTo(GetIntType())}
	want := PointerTo(GetSimpleType(EChar))
	e.CheckAndSetCast(want)
	if GetIntType().SizeInBytes() != GetSimpleType(EChar).SizeInBytes() {
		if e.CastType == nil {
			t.Fatal("expected cast")
		}
	}
}

func TestIsPromotableRanks(t *testing.T) {
	// Type.cpp:1387–1416
	if !GetSimpleType(EChar).IsPromotable(GetIntType()) {
		t.Fatal("char→int")
	}
	if GetIntType().IsPromotable(GetSimpleType(EChar)) {
		t.Fatal("int not→char")
	}
	if !GetSimpleType(EShort).IsPromotable(GetIntType()) {
		t.Fatal("short→int")
	}
	if GetIntType().IsPromotable(GetSimpleType(EShort)) {
		t.Fatal("int not→short")
	}
	if !GetSimpleType(EFloat).IsPromotable(GetIntType()) {
		t.Fatal("float→int promotable")
	}
}

func TestIsConvertableFloatToIntForbidden(t *testing.T) {
	// is_convertable: conversion FROM float TO int forbidden
	// other.IsFloat() && !t.IsFloat() → false when converting int from float? 
	// C++: if (t->is_float() && !is_float()) return false
	// so target t is float and this is not float → false when converting non-float to float?
	// Wait: `this->is_convertable(t)` means this converts TO t.
	// if t is float and this is not float → false... that forbids int→float?
	// Re-read: "forbidden conversion from float to int"
	// if (t->is_float() && !is_float()) — t is the parameter (target type)
	// So if target is float and source is not float → return false. That forbids int→float.
	// Comment says float to int... Parameter naming: is_convertable(const Type *t) 
	// "this" is source, t is destination. Comment: forbiden conversion from float to int
	// if (t->is_float() && !is_float()) — if destination float and source not float → false
	// That would block int→float. For float→int: t is int (not float), this is float: condition false, fall through to void check which allows.
	// So int→float is blocked? That seems inverted from the comment.
	// Comment: "forbiden conversion from float to int"
	// if (t->is_float() && !is_float()) — if TARGET is float and SOURCE is not → return false
	// That blocks converting TO float FROM non-float. Comment might mean the reverse of code, or they mean assignment of float into int context differently.
	// Follow code literally:
	if GetIntType().IsConvertable(GetSimpleType(EFloat)) {
		t.Fatal("code blocks non-float→float?")
	}
	// float → int allowed by code
	if !GetSimpleType(EFloat).IsConvertable(GetIntType()) {
		t.Fatal("float→int")
	}
}

func TestIsConvertablePtrStrictFloatAndCPP(t *testing.T) {
	pi := PointerTo(GetIntType())
	pf := PointerTo(GetSimpleType(EFloat))
	// same size usually float=4 int=4 → C allows by size
	opts := Defaults()
	opts.StrictFloat = false
	opts.LangCPP = false
	if !pi.IsConvertableOpts(pf, opts) && pi.BaseType().SizeInBytes() == pf.BaseType().SizeInBytes() {
		t.Fatal("same size ptr C")
	}
	opts.StrictFloat = true
	if pi.IsConvertableOpts(pf, opts) {
		t.Fatal("strict float blocks int*/float*")
	}
	opts.StrictFloat = false
	opts.LangCPP = true
	if pi.IsConvertableOpts(pf, opts) {
		t.Fatal("lang_cpp blocks")
	}
}
