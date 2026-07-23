package csmith

import (
	"strings"
	"testing"
)

func TestItemizedIVAsIndexExpressionOutput(t *testing.T) {
	ClearError()
	defer ClearError()
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_106", Type: GetIntType(), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	parent.AsArray = parent
	item := parent.ItemizeConstIndices([]int{4}, nil)
	if item == nil {
		t.Fatal("itemize", GetError())
	}
	e := &Expression{Term: TermVariable, Var: &item.Variable, ExprType: GetIntType()}
	got := e.Output()
	if got != "g_106[4]" {
		t.Fatalf("ExpressionVariable of itemized IV: got %q want g_106[4] err=%v", got, GetError())
	}
}

// VariableSelector.cpp:1492 — ExpressionVariable(*iv); Indices string must match Output.
func TestItemizeArrayIndicesStringUsesItemizedOutput(t *testing.T) {
	ClearError()
	defer ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	// Fixed seed that may add offset; accept g_106[4] or (g_106[4] + N)
	r := NewRng(1)
	vs := NewVariableSelector(opts)
	ivParent := &ArrayVariable{
		Variable: Variable{Name: "g_106", Type: GetIntType(), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	ivParent.AsArray = ivParent
	ivItem := ivParent.ItemizeConstIndices([]int{4}, vs)
	if ivItem == nil {
		t.Fatal("iv itemize")
	}
	target1 := &ArrayVariable{
		Variable: Variable{Name: "l_91", Type: GetIntType(), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	target1.AsArray = target1
	fm := NewFactMgr(&Function{Name: "f"})
	fm.GlobalFacts = []*FactPointTo{}
	cg1 := EmptyCGContext().WithFactMgr(fm)
	cg1.IVBounds = map[*Variable]int{&ivItem.Variable: 0}
	got1 := vs.ItemizeArray(r, cg1, target1)
	if got1 == nil {
		t.Fatalf("ItemizeArray 1d: %v", GetError())
	}
	out1 := got1.OutputAccess()
	if !strings.Contains(out1, "g_106[4]") {
		t.Fatalf("got %q must contain g_106[4] (not bare g_106); Indices=%v", out1, got1.Indices)
	}
	// Indices string form must also carry itemization (not v.Name only)
	if len(got1.Indices) != 1 || !strings.Contains(got1.Indices[0], "g_106[4]") {
		t.Fatalf("Indices string must use Output not Name: %v", got1.Indices)
	}
}
