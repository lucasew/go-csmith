package csmith

import (
	"strings"
	"testing"
)

func TestItemizedIVAsIndexExpressionOutput(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_106", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	parent.AsArray = parent
	item := parent.ItemizeConstIndices([]int{4}, NewVariableSelector(testAmbientSession, Defaults()))
	if item == nil {
		t.Fatal("itemize", GetErrorSess(testAmbientSession))
	}
	e := &Expression{Term: TermVariable, Var: &item.Variable, ExprType: GetIntTypeSess(testAmbientSession)}
	got := e.OutputSess(testAmbientSession)
	if got != "g_106[4]" {
		t.Fatalf("ExpressionVariable of itemized IV: got %q want g_106[4] err=%v", got, GetErrorSess(testAmbientSession))
	}
}

// VariableSelector.cpp:1492 — ExpressionVariable(*iv); Indices string must match Output.
func TestItemizeArrayIndicesStringUsesItemizedOutput(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	// Fixed seed that may add offset; accept g_106[4] or (g_106[4] + N)
	r := NewRngSess(testAmbientSession, 1)
	vs := NewVariableSelector(testAmbientSession, opts)
	ivParent := &ArrayVariable{
		Variable: Variable{Name: "g_106", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	ivParent.AsArray = ivParent
	ivItem := ivParent.ItemizeConstIndices([]int{4}, vs)
	if ivItem == nil {
		t.Fatal("iv itemize")
	}
	target1 := &ArrayVariable{
		Variable: Variable{Name: "l_91", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	target1.AsArray = target1
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	fm.GlobalFacts = []*FactPointTo{}
	cg1 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg1.IVBounds = map[*Variable]int{&ivItem.Variable: 0}
	got1 := vs.ItemizeArray(r, cg1, target1)
	if got1 == nil {
		t.Fatalf("ItemizeArray 1d: %v", GetErrorSess(testAmbientSession))
	}
	out1 := got1.OutputAccessSess(testAmbientSession)
	if !strings.Contains(out1, "g_106[4]") {
		t.Fatalf("got %q must contain g_106[4] (not bare g_106); Indices=%v IndexExprs=%d",
			out1, got1.Indices, len(got1.IndexExprs))
	}
	// C++ itemize_array stores Expression* only (VariableSelector.cpp:1463–1470).
	// Emit uses IndexExprs→Output; Indices is a non-emit cache (may be Name-based).
	if len(got1.IndexExprs) != 1 || got1.IndexExprs[0] == nil {
		t.Fatalf("IndexExprs must hold live Expression*: %v", got1.IndexExprs)
	}
}
