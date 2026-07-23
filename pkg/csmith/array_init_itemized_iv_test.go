package csmith

import "testing"

// StatementArrayOp.cpp:245–250 + ArrayVariable.cpp:708–709 —
// output_with_indices uses cvs[i]->Output (virtual ArrayVariable::Output for
// itemized IVs). Soft invent built access with IV.Name only → bare collective
// name in array-init body while for-header used OutputC (seed-48:
// l_91[p_74][g_105][g_106[4]] vs …[g_106]).
func TestArrayInitAccessUsesItemizedIVOutput(t *testing.T) {
	ClearError()
	defer ClearError()
	CtrlVarsDoFinalization()
	opts := Defaults()
	SetProcessOptions(opts)
	// itemized array IV like choose_ok_var after SelectLoopCtrlVar
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_106", Type: GetIntType(), IsArray: true, ArraySizes: []int{5}},
		Sizes:    []int{5},
	}
	parent.AsArray = parent
	iv := parent.ItemizeConstIndices([]int{4}, nil)
	if iv == nil {
		t.Fatal("itemize")
	}
	// Mirror make_random_array_init access build: must use OutputC not Name
	if iv.Name != "g_106" {
		t.Fatalf("Name field is bare %q", iv.Name)
	}
	if out := iv.OutputC(); out != "g_106[4]" {
		t.Fatalf("OutputC got %q want g_106[4]", out)
	}
	// Soft invent: Name only loses indices
	bad := "l_91[" + iv.Name + "]"
	if bad != "l_91[g_106]" {
		t.Fatal(bad)
	}
	// Fair: OutputC
	good := "l_91[" + iv.OutputC() + "]"
	if good != "l_91[g_106[4]]" {
		t.Fatalf("got %q", good)
	}
}
