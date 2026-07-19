package csmith

import (
	"strings"
	"testing"
)

func TestRandomTypeFromTypeNil(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	ty := RandomTypeFromType(NewRng(3), env, opts, probs, nil, false, false)
	if ty == nil || (ty.IsSimple() && ty.Simple() == EVoid) {
		t.Fatalf("%v", ty)
	}
}

func TestRandomTypeFromTypeSimpleRerolls(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// requesting int may yield another simple when strict_simple_type=false
	seen := map[ESimpleType]bool{}
	r := NewRng(5)
	for i := 0; i < 40; i++ {
		ty := RandomTypeFromType(r, nil, opts, probs, GetIntType(), false, false)
		if ty == nil || !ty.IsSimple() {
			t.Fatalf("%v", ty)
		}
		seen[ty.Simple()] = true
	}
	if len(seen) < 2 {
		t.Log("only one simple type in sample", seen)
	}
}

func TestRandomTypeFromTypeStrictSimpleKeeps(t *testing.T) {
	// Type.cpp:599 — strict_simple_type skips choose_random_simple re-roll
	opts := Defaults()
	probs := NewProbabilities(opts)
	want := GetIntType()
	for seed := uint64(1); seed < 20; seed++ {
		ty := RandomTypeFromType(NewRng(seed), nil, opts, probs, want, false, true)
		if ty != want {
			t.Fatalf("strict simple seed %d: got %v want int", seed, ty)
		}
	}
}

func TestRandomTypeFromTypeStructUnchanged(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	st := &Type{isStruct: true, StructName: "S9"}
	ty := RandomTypeFromType(NewRng(1), nil, opts, probs, st, false, false)
	if ty != st {
		t.Fatal("struct should pass through")
	}
	// nil type / simple re-roll need RNG sticky; no invent pick/keep-simple shells
	ClearError()
	if RandomTypeFromType(nil, &TypeEnv{AllTypes: []*Type{GetIntType()}}, opts, probs, nil, false, false) != nil {
		t.Fatal("nil RNG + nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG + nil type must SetError sticky")
	}
	ClearError()
	if RandomTypeFromType(nil, nil, opts, probs, GetIntType(), false, false) != nil {
		t.Fatal("nil RNG + simple re-roll must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG + simple re-roll must SetError sticky")
	}
	ClearError()
	// non-simple keep path does not need RNG
	if RandomTypeFromType(nil, nil, opts, probs, st, false, false) != st {
		t.Fatal("struct keep without RNG")
	}
	ClearError()
	// TypeEnv + AllTypes always live after GenerateAllTypes; sticky nil
	if RandomTypeFromType(NewRng(1), nil, opts, probs, nil, false, false) != nil {
		t.Fatal("nil env + nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil env RandomTypeFromType must SetError sticky")
	}
	ClearError()
	if RandomTypeFromType(NewRng(1), &TypeEnv{}, opts, probs, nil, false, false) != nil {
		t.Fatal("empty AllTypes + nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("empty AllTypes RandomTypeFromType must SetError sticky")
	}
	ClearError()
}

func TestGenerateMainHasReturn0(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "return 0;") {
		t.Fatal("main missing return 0")
	}
	// first func invocation present
	if !strings.Contains(out, "func_1(") {
		t.Fatal("missing first func call")
	}
}
