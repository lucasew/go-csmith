package csmith

import "testing"

func TestArrayBoundBareNameIgnoresPrefixName(t *testing.T) {
	// ArrayVariable.cpp:573–590 — bounds use bare name, not get_actual_name.
	opts := Defaults()
	opts.PrefixName = true
	s := NewSession(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_10", Type: GetIntTypeSess(s), IsArray: true, ArraySizes: []int{7}},
		Sizes:    []int{7},
	}
	av.AsArray = av
	if got := av.OutputLowerBoundSess(s); got != "g_10[0]" {
		t.Fatalf("lower bound: %q", got)
	}
	if got := av.OutputUpperBoundArraySess(s); got != "g_10[6]" {
		t.Fatalf("upper bound: %q", got)
	}
	// subject Output still empty under prefix_name NDEBUG count-prefix
	if got := av.GetActualNameSess(s, true); got != "" && got != "g_10" {
		// either empty (NDEBUG prefix) or full — not the bound path
		t.Logf("GetActualName under prefix=%q", got)
	}
}
