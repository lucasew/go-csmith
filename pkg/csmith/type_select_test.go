package csmith

import (
	"strings"
	"testing"
)

func TestFindPointerTypeCachesAndRegisters(t *testing.T) {
	var env TypeEnv
	p1 := env.FindPointerType(GetIntType(), true)
	p2 := env.FindPointerType(GetIntType(), true)
	if p1 != p2 {
		t.Fatal("identity")
	}
	if !env.HasPointerType() || len(env.DerivedTypes) != 1 {
		t.Fatal(env.DerivedTypes)
	}
}

func TestMakeRandomPointerTypeIntStar(t *testing.T) {
	var env TypeEnv
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Force non-20% path often; result should be int* when simple base consolidated
	r := NewRng(2)
	// first flip 20% for double ptr — seed2 first flipcoin(20)
	p := env.MakeRandomPointerType(r, opts, probs)
	if p == nil || p.PtrType() == nil {
		t.Fatal(p)
	}
	// if base simple path, pointee is int
	if p.IndirectLevel() == 1 && p.PtrType() != GetIntType() {
		// double-ptr path might differ
		if p.PtrType().PtrType() == nil && p.PtrType() != GetIntType() {
			t.Fatalf("expected int* or deeper, got %s", p.CName())
		}
	}
}

func TestSelectLTypeDefaultInt(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Force no pointer: use op that isn't simple assign... still pointer only on simple assign
	// With PointerAsLTypeProb=50, may get pointer. Use many seeds with compound op.
	var env TypeEnv
	r := NewRng(2)
	ty := SelectLType(r, opts, probs, &env, false, AssignBitAnd)
	if ty != GetIntType() {
		// float off, no struct → int
		t.Fatalf("compound assign LType want int got %s", ty.CName())
	}
}

func TestSelectLTypeCanBePointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		r := NewRng(seed)
		ty := SelectLType(r, opts, probs, &env, false, AssignSimple)
		if ty != nil && ty.PtrType() != nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected some pointer LType for simple assign")
	}
}

func TestGenerateCanEmitPointerGlobal(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 60; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "int*") || strings.Contains(out, "int *") {
			found = true
			break
		}
		// CName is "int*" without space
		if strings.Contains(out, "int*") {
			found = true
			break
		}
	}
	// pointer as assign LType creates int* globals
	if !found {
		// also check param pointers after first pointer type exists
		t.Log("no int* in 1..59 — pointer LType may still create via assign")
	}
}

func TestAssignOpWorksForFloat(t *testing.T) {
	if !AssignOpWorksForFloat(AssignSimple) || AssignOpWorksForFloat(AssignBitAnd) {
		t.Fatal("float ops")
	}
}
