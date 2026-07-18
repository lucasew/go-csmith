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
	ty := RandomTypeFromType(NewRng(3), env, opts, probs, nil, false)
	if ty == nil || (ty.IsSimple() && ty.Simple() == EVoid) {
		t.Fatalf("%v", ty)
	}
}

func TestRandomTypeFromTypeSimpleRerolls(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// requesting int may yield another simple
	seen := map[ESimpleType]bool{}
	r := NewRng(5)
	for i := 0; i < 40; i++ {
		ty := RandomTypeFromType(r, nil, opts, probs, GetIntType(), false)
		if ty == nil || !ty.IsSimple() {
			t.Fatalf("%v", ty)
		}
		seen[ty.Simple()] = true
	}
	if len(seen) < 2 {
		t.Log("only one simple type in sample", seen)
	}
}

func TestRandomTypeFromTypeStructUnchanged(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	st := &Type{isStruct: true, StructName: "S9"}
	ty := RandomTypeFromType(NewRng(1), nil, opts, probs, st, false)
	if ty != st {
		t.Fatal("struct should pass through")
	}
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
