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
}
