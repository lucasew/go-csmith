package csmith

import (
	"strings"
	"testing"
)

func TestArrayInitRecursiveProcessStaticSeed(t *testing.T) {
	// ArrayVariable.cpp:429 static unsigned seed = 0xABCDEF across OutputDefs
	ClearError()
	ResetArrayInitSeed()
	SetProcessOptions(Defaults())

	// First array: size 1, pool {"A"} → one leaf, seed advances once
	a1 := &ArrayVariable{
		Variable: Variable{Name: "a1", Type: GetIntType(), IsArray: true, ArraySizes: []int{1}},
		Sizes:    []int{1},
	}
	s1 := a1.buildInitRecursive(0, []string{"A"})
	if s1 != "{A}" {
		t.Fatalf("a1: got %q", s1)
	}
	if arrayInitSeed != 0xABCDEF+1 {
		t.Fatalf("seed after 1 leaf: got %#x want %#x", arrayInitSeed, 0xABCDEF+1)
	}

	// Second array: size 2, pool {"X","Y"} — first leaf uses seed 0xABCDEF+1 not reset
	a2 := &ArrayVariable{
		Variable: Variable{Name: "a2", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	s2 := a2.buildInitRecursive(0, []string{"X", "Y"})
	// manual: seed=s0+1, i=0 → idx; seed++; i=1 → idx
	wantSeed := uint32(0xABCDEF + 1)
	idx0 := ((wantSeed*wantSeed + uint32(0+7)*uint32(0+13)) * 52369) % 2
	wantSeed++
	idx1 := ((wantSeed*wantSeed + uint32(1+7)*uint32(1+13)) * 52369) % 2
	pool := []string{"X", "Y"}
	want := "{" + pool[idx0] + "," + pool[idx1] + "}"
	if s2 != want {
		t.Fatalf("a2: got %q want %q (idx %d,%d)", s2, want, idx0, idx1)
	}
	ResetArrayInitSeed()
	ClearError()
}

func TestBuildInitializerStrForceNonUniformUsesStaticSeed(t *testing.T) {
	ClearError()
	ResetArrayInitSeed()
	opts := Defaults()
	opts.ForceNonUniformArrayInit = true
	SetProcessOptions(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 2}},
		Sizes:    []int{2, 2},
	}
	// four leaves with pool of 3
	out := av.buildInitializerStr([]string{"0", "1", "2"})
	if !strings.HasPrefix(out, "{{") || strings.Count(out, "{") != 3 {
		// 1 outer + 2 mid for 2d? actually {{a,b},{c,d}} → braces 1+2+0? {{ }} 3 open
		t.Log(out)
	}
	if strings.Count(out, "0")+strings.Count(out, "1")+strings.Count(out, "2") != 4 {
		// each leaf one char
		t.Fatalf("want 4 leaves: %q", out)
	}
	// second call continues seed
	seedAfter := arrayInitSeed
	_ = av.buildInitializerStr([]string{"0", "1", "2"})
	if arrayInitSeed <= seedAfter {
		t.Fatal("static seed must advance across arrays")
	}
	ResetArrayInitSeed()
	ClearError()
}
