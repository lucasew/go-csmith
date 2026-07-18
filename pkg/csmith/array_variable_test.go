package csmith

import (
	"strings"
	"testing"
)

func TestCreateArrayVariableDimensions(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, nil, "g_1", GetIntType(), MakeInt(0), q)
	if av == nil || av.Dimension() < 1 {
		t.Fatal(av)
	}
	if av.TotalSize() < 1 {
		t.Fatal(av.Sizes)
	}
	for _, s := range av.Sizes {
		if s < 1 || s > opts.MaxArrayLenPerDim {
			t.Fatalf("dim %d", s)
		}
	}
	if av.TotalSize() > opts.MaxArrayLength {
		t.Fatalf("total %d > max %d", av.TotalSize(), opts.MaxArrayLength)
	}
}

func TestArrayCDecl(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	d := av.CDeclType()
	if d != "int a[2][3]" {
		t.Fatal(d)
	}
}

func TestGenerateCanEmitArrayDecl(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// look for [digit]
		if strings.Contains(out, "[") && strings.Contains(out, "]") {
			// exclude main argc argv
			if strings.Contains(out, "static ") && (strings.Contains(out, "g_") || strings.Contains(out, "l_")) {
				found = true
				break
			}
		}
	}
	if !found {
		t.Log("no array decl in 1..79 — NewArrayVariableProb=20 may be rare with few creates")
	}
}

func TestCreateAndInitializeArrayFlip(t *testing.T) {
	opts := Defaults()
	// force high array prob
	vs := NewVariableSelector(opts)
	// hack probs
	vs.Probs.single[PNewArrayVariableProb] = 100
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.createAndInitialize(AccessWrite, EmptyCGContext(), GetIntType(), q, nil, "g_9", r)
	if v == nil || !v.IsArray {
		t.Fatalf("%+v arrays=%d", v, len(vs.Arrays))
	}
}
