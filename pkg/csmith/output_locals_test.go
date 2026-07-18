package csmith

import (
	"strings"
	"testing"
)

func TestLocalOutputDef(t *testing.T) {
	b := &Block{}
	lv := CreateVariableScalars("l_1", GetIntType(), true, false)
	lv.Init = MakeInt(2)
	b.LocalVars = []*Variable{lv}
	out := b.Output(0)
	if !strings.Contains(out, "const") || !strings.Contains(out, "l_1") || !strings.Contains(out, "2") {
		t.Fatal(out)
	}
}

func TestFunctionParamQualified(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), true, false)
	p := CreateVariableScalars("p_1", GetIntType(), true, true)
	f.Param = []*Variable{p}
	decl := f.OutputForwardDecl()
	if !strings.Contains(decl, "const") || !strings.Contains(decl, "p_1") {
		t.Fatal(decl)
	}
	if !strings.Contains(decl, "volatile") {
		t.Fatal("param vol", decl)
	}
	// return type from RV includes const
	if !strings.HasPrefix(strings.TrimSpace(decl), "const") {
		t.Fatal("return", decl)
	}
}
