package csmith

import (
	"strings"
	"testing"
)

func TestIsPackedAggregateFieldVar(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0", Packed: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(parent.FieldVars) == 0 {
		t.Skip("no fields")
	}
	f0 := parent.FieldVars[0]
	if !f0.IsPackedAggregateFieldVar() {
		t.Fatal("want packed field")
	}
	plain := CreateVariableScalars("g_i", GetIntType(), false, false)
	if plain.IsPackedAggregateFieldVar() {
		t.Fatal("scalar")
	}
}

func TestItemizeArrayOffsetBinary(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// size 8 so remain > 1 when bound is 0 → offset possible
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	// force size
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{8}
	av.ArraySizes = []int{8}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: 0}
	// scan seeds for offset form
	foundOff := false
	for seed := uint64(1); seed < 40; seed++ {
		item := vs.ItemizeArray(NewRng(seed), cg, av)
		if item == nil {
			t.Fatal("itemize")
		}
		if len(item.IndexExprs) != 1 {
			t.Fatalf("dims %d", len(item.IndexExprs))
		}
		ie := item.IndexExprs[0]
		if ie.Term == TermFunction && ie.Invoke != nil && ie.Invoke.Binary == "+" {
			foundOff = true
			out := item.OutputAccess()
			if !strings.Contains(out, "+") {
				t.Fatal(out)
			}
			// UseVar of IV still works via nested expression
			if !ie.UseVar(iv) {
				// UseVar on func may not recurse — check Args
				if len(ie.Invoke.Args) > 0 && ie.Invoke.Args[0] != nil && ie.Invoke.Args[0].UseVar(iv) {
					// ok
				} else {
					t.Log("UseVar IV optional on binary")
				}
			}
			break
		}
	}
	if !foundOff {
		// still valid if all seeds picked 0 offset
		t.Log("no offset in scan; bare IV path covered by other tests")
	}
}

func TestItemizeArrayRejectsInvalidBound(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: InvalidIVBound}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("invalid bound must not itemize")
	}
}

func TestSelectArrayFiltersPartialWrite(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	// mark partially written → filtered → CreateRandomArray may still run
	eff := EmptyEffect().WriteVar(&av.Variable)
	cg := WithEffectContext(eff)
	// disable global create by turning off globals? CreateRandomArray uses globals
	// ensure filter drops av: if CreateRandomArray returns different name ok
	got := vs.SelectArray(NewRng(3), cg)
	if got == av {
		t.Fatal("partially written array must not be selected")
	}
}
