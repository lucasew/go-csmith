package csmith

import (
	"strings"
	"testing"
)

func TestIsEquivalentSameSize(t *testing.T) {
	// Type.cpp SizeInBytes: int 4, long 4 (fixed; not invent LP64 long==8)
	if !GetIntTypeSess(testAmbientSession).IsEquivalentSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("int~int")
	}
	if GetSimpleTypeSess(testAmbientSession, EChar).IsEquivalentSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("char!~int")
	}
}

func TestNeedsCastPointerBases(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	pi := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	pc := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar))
	if !pi.NeedsCastSess(testAmbientSession, pc) && pi.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) != pc.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) {
		// needs cast when bases inequivalent
	}
	if pi.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) != pc.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) {
		if !pi.NeedsCastSess(testAmbientSession, pc) {
			t.Fatal("int* needs cast from char*")
		}
	}
	if pi.NeedsCastSess(testAmbientSession, pi) {
		t.Fatal("same no cast")
	}
	// incomplete Type sticky
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).NeedsCastSess(testAmbientSession, pi) {
		t.Fatal("nil NeedsCast must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NeedsCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionCastOutput(t *testing.T) {
	e := &Expression{
		Term:     TermConstant,
		Con:      MakeIntSess(testAmbientSession, 0),
		CastType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)),
	}
	out := e.Output()
	// Expression.cpp:228–231 — "(type) " with trailing space
	if !strings.Contains(out, "int32_t*") || !strings.HasPrefix(out, "(") {
		t.Fatal(out)
	}
	if !strings.Contains(out, ") ") {
		t.Fatalf("want space after cast: %q", out)
	}
}

func TestHasBitfields(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: 3},
	}}
	if !st.HasBitfieldsSess(testAmbientSession) {
		t.Fatal("bf")
	}
	st2 := &Type{isStruct: true, Fields: []StructField{
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if st2.HasBitfieldsSess(testAmbientSession) {
		t.Fatal("no bf")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete HasBitfields must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested HasBitfields residual: Type-nil deeper field soft invent was soft-continue later siblings.
	// Fair: sticky has-bitfields true.
	innerHole := &Type{isStruct: true, Fields: []StructField{{Type: nil, BitWidth: -1}}}
	outer := &Type{isStruct: true, Fields: []StructField{
		{Type: innerHole, BitWidth: -1},
		{Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if !outer.HasBitfieldsSess(testAmbientSession) {
		t.Fatal("nested residual HasBitfields must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual HasBitfields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested ContainPointerField residual same invent soft-continue pointer-free.
	if !outer.ContainPointerFieldSess(testAmbientSession) {
		t.Fatal("nested residual ContainPointerField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual ContainPointerField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested HasIntField residual soft invent was soft-continue later fields invent has-int.
	// Fair: sticky not-has-int false.
	if outer.HasIntFieldSess(testAmbientSession) {
		t.Fatal("nested residual HasIntField must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual HasIntField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckAndSetCast(t *testing.T) {
	// int* expr desired as char* → needs cast if bases inequivalent
	v := CreateVariableQferSess(testAmbientSession, "g_1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), NewCVQualifiers([]bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	want := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar))
	e.CheckAndSetCast(want)
	if GetIntTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) != GetSimpleTypeSess(testAmbientSession, EChar).SizeInBytesSess(testAmbientSession) {
		if e.CastType == nil {
			t.Fatal("expected cast")
		}
	}
	// Expression + desired Type always live; sticky no invent skip-cast soft-success
	ClearErrorSess(testAmbientSession)
	(*Expression)(nil).CheckAndSetCast(want)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression CheckAndSetCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	e2 := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	e2.CheckAndSetCast(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil desired CheckAndSetCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GetTypeUncast sticky (no invent soft-skip cast past Type-nil shell)
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil}}
	hole.CheckAndSetCast(want)
	if hole.CastType != nil {
		t.Fatal("Type-nil var must not invent cast")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil var CheckAndSetCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// NeedsCast residual soft invent was CastType set then invent complete cast success.
	// Fair: sticky no CastType past NeedsCast residual true path (nil shells).
	src := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if src.NeedsCastSess(testAmbientSession, nil) {
		t.Fatal("NeedsCast nil other must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("NeedsCast nil other must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual true path: nil src base via NeedsCast on nil this
	if (*Type)(nil).NeedsCastSess(testAmbientSession, want) {
		t.Fatal("nil NeedsCast must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NeedsCast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckAndSetCastOptsLangCPP(t *testing.T) {
	// Expression.cpp:222 — only lang_cpp sets cast
	v := CreateVariableQferSess(testAmbientSession, "g_1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), NewCVQualifiers([]bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	want := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar))
	opts := Defaults()
	opts.LangCPP = false
	e.CheckAndSetCastOpts(want, opts)
	if e.CastType != nil {
		t.Fatal("C mode must not set cast via check_and_set_cast")
	}
	opts.LangCPP = true
	e.CheckAndSetCastOpts(want, opts)
	if e.CastType == nil && !PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)).BaseTypeSess(testAmbientSession).IsEquivalentSess(testAmbientSession, want.BaseTypeSess(testAmbientSession)) {
		t.Fatal("lang_cpp should set cast for inequivalent pointer bases")
	}
}

func TestCheckAndSetCastViaInvokeGetType(t *testing.T) {
	// check_and_set_cast uses get_type() for all term kinds (not only var/const)
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "+", Args: []*Expression{arg}}
	e := &Expression{Term: TermFunction, Invoke: fi}
	want := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar))
	e.CheckAndSetCast(want)
	if e.GetTypeUncast() == nil {
		t.Fatal("uncast type")
	}
	// + of pointer-typed constant still pointer type via unary get_type
	if e.CastType == nil && e.GetTypeUncast().NeedsCastSess(testAmbientSession, want) {
		t.Fatal("expected cast from invoke type")
	}
}

func TestNeedsCastOnlySourcePointer(t *testing.T) {
	// Type.cpp:1470 — only `this` must be pointer
	ClearErrorSess(testAmbientSession)
	pi := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	// bases int vs char inequivalent → cast
	if !pi.NeedsCastSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar)) {
		t.Fatal("int* needs_cast(char)")
	}
	// non-pointer source never needs cast
	if GetIntTypeSess(testAmbientSession).NeedsCastSess(testAmbientSession, pi) {
		t.Fatal("int does not needs_cast")
	}
	// same base → no cast
	if pi.NeedsCastSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("int* base int equivalent to int")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete NeedsCast must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsPromotableRanks(t *testing.T) {
	// Type.cpp:1387–1416
	if !GetSimpleTypeSess(testAmbientSession, EChar).IsPromotableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("char→int")
	}
	if GetIntTypeSess(testAmbientSession).IsPromotableSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EChar)) {
		t.Fatal("int not→char")
	}
	if !GetSimpleTypeSess(testAmbientSession, EShort).IsPromotableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("short→int")
	}
	if GetIntTypeSess(testAmbientSession).IsPromotableSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)) {
		t.Fatal("int not→short")
	}
	if !GetSimpleTypeSess(testAmbientSession, EFloat).IsPromotableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("float→int promotable")
	}
}

func TestIsConvertableFloatToIntForbidden(t *testing.T) {
	// is_convertable: conversion FROM float TO int forbidden
	// other.IsFloatSess(testAmbientSession) && !t.IsFloatSess(testAmbientSession) → false when converting int from float?
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
	if GetIntTypeSess(testAmbientSession).IsConvertableSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EFloat)) {
		t.Fatal("code blocks non-float→float?")
	}
	// float → int allowed by code
	if !GetSimpleTypeSess(testAmbientSession, EFloat).IsConvertableSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) {
		t.Fatal("float→int")
	}
}

func TestIsConvertablePtrStrictFloatAndCPP(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	pi := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	pf := PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EFloat))
	// same size usually float=4 int=4 → C allows by size
	opts := Defaults()
	opts.StrictFloat = false
	opts.LangCPP = false
	if !pi.IsConvertableOptsSess(testAmbientSession, pf, opts) && pi.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) == pf.BaseTypeSess(testAmbientSession).SizeInBytesSess(testAmbientSession) {
		t.Fatal("same size ptr C")
	}
	opts.StrictFloat = true
	if pi.IsConvertableOptsSess(testAmbientSession, pf, opts) {
		t.Fatal("strict float blocks int*/float*")
	}
	opts.StrictFloat = false
	opts.LangCPP = true
	if pi.IsConvertableOptsSess(testAmbientSession, pf, opts) {
		t.Fatal("lang_cpp blocks")
	}
	// incomplete Type sticky not-convertable
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsConvertableOptsSess(testAmbientSession, pi, opts) {
		t.Fatal("nil IsConvertableOpts must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsConvertableOpts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
