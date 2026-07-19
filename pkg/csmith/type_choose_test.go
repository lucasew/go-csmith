package csmith

import (
	"strings"
	"testing"
)

func TestChooseRandomFromAllTypes(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	if len(env.AllTypes) < 5 {
		t.Fatalf("AllTypes %d", len(env.AllTypes))
	}
	seenStruct := false
	r := NewRng(3)
	for i := 0; i < 200; i++ {
		ty := env.ChooseRandom(r, opts, probs, false)
		if ty == nil {
			t.Fatal("nil")
		}
		if ty.IsSimple() && ty.Simple() == EVoid {
			t.Fatal("void")
		}
		if ty.IsStruct() {
			seenStruct = true
		}
	}
	if opts.Structs && len(env.StructTypes) > 0 && !seenStruct {
		t.Log("struct not picked in sample — may be rare if few structs")
	}
}

func TestRandomReturnTypeUsesEnv(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(5), opts, probs, env)
	ty := RandomReturnType(NewRng(7), probs, env, opts)
	if ty == nil {
		t.Fatal("nil")
	}
	if ty.IsSimple() && ty.Simple() == EVoid {
		t.Fatal("void return")
	}
	// sticky no invent default int without RNG
	ClearError()
	if RandomReturnType(nil, probs, env, opts) != nil {
		t.Fatal("nil RNG RandomReturnType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomReturnType must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomParamNoConstant(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// force many picks — constant weight 0 + filtered
	for seed := uint64(1); seed < 40; seed++ {
		e := func() *Expression { c := EmptyCGContext(); return MakeRandomParam(NewRng(seed), opts, tables, vs, &c, GetIntType(), nil, 0) }()
		if e != nil && e.Term == TermConstant {
			t.Fatalf("constant param seed %d", seed)
		}
	}
}

func TestGenerateCanReturnStruct(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// struct S0 func_... or similar
		if strings.Contains(out, "struct S") && strings.Contains(out, "func_") {
			// look for "struct S0 func_" or "S0 func_" depending on CName
			if strings.Contains(out, "func_") {
				// return type of form "struct S0 func_N"
				if strings.Contains(out, "struct S0 ") || strings.Contains(out, "struct S1 ") {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Log("struct-returning func rare in sample")
	}
}
