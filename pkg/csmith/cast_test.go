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
