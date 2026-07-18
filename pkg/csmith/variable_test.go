package csmith

import "testing"

func TestCreateVariableAndPredicates(t *testing.T) {
	v := CreateVariableScalars("g_1", GetSimpleType(EInt), true, false)
	if v == nil || !v.IsGlobal() || v.IsLocal() || !v.IsConst() || v.IsVolatile() {
		t.Fatalf("global const int: %+v const=%v vol=%v", v, v.IsConst(), v.IsVolatile())
	}
	p := CreateVariableScalars("p_1", GetSimpleType(EShort), false, true)
	if !p.IsArgument() || !p.IsVolatile() {
		t.Fatal("param volatile")
	}
	l := CreateVariableScalars("l_1", GetSimpleType(EChar), false, false)
	if !l.IsLocal() || l.IsGlobal() {
		t.Fatal("local")
	}
}

func TestCreateVariableRejectsVoid(t *testing.T) {
	if CreateVariableScalars("g_1", GetSimpleType(EVoid), false, false) != nil {
		t.Fatal("void simple must be rejected")
	}
	// Variable.cpp:388/412 — assert(type); no soft invent
	if CreateVariableScalars("g_n", nil, false, false) != nil {
		t.Fatal("nil type must be rejected")
	}
	if CreateVariableWithInit("g_n", nil, MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("CreateVariableWithInit nil type")
	}
}

func TestCreateVariableErrorGuardAfterInit(t *testing.T) {
	// Variable.cpp:397/401 — ERROR_GUARD after Constant::make_random / field vars
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	if CreateVariableScalars("g_e", GetIntType(), false, false) != nil {
		t.Fatal("sticky error must fail CreateVariableScalars")
	}
	ClearError()
	SetError(ErrGeneric)
	if CreateVariableWithInit("g_e2", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("sticky error must fail CreateVariableWithInit after field expand")
	}
}
