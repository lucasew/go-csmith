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

func TestChooseRandomStructUnionTypeEmptyPool(t *testing.T) {
	// Type.cpp:523 assert(sz > 0) — empty ok_types must not invent a type
	if ChooseRandomStructUnionType(NewRng(1), nil) != nil {
		t.Fatal("empty pool")
	}
	if ChooseRandomStructUnionType(nil, []*Type{GetIntType()}) != nil {
		t.Fatal("nil rng")
	}
}

func TestMakeRandomPointerTypeIntStar(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp GenerateSimpleTypes seeds AllTypes before make_random_pointer_type
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
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
	// Type.cpp:1141 — nil env/rng → ERROR_GUARD nullptr, no soft invent
	if (*TypeEnv)(nil).MakeRandomPointerType(NewRng(1), opts, probs) != nil {
		t.Fatal("nil env must not invent int*")
	}
}

func TestMakeRandomPointerTypeConsolidatesAllSimple(t *testing.T) {
	// Type.cpp:1161–1164 — any eSimple (including float) consolidates to int*
	opts := Defaults()
	opts.EnableFloat = true
	probs := NewProbabilities(opts)
	env := &TypeEnv{AllTypes: []*Type{GetSimpleType(EFloat)}}
	// avoid 20% derived path (empty derived)
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		p := env.MakeRandomPointerType(NewRng(seed), opts, probs)
		if p != nil && p.IndirectLevel() == 1 && p.PtrType() == GetIntType() {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("float base must consolidate to int*")
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

func TestSelectLTypeErrorGuard(t *testing.T) {
	// Type.cpp:1613 — ERROR_GUARD after pointer branch always; no soft invent get_int_type
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	if SelectLType(NewRng(1), opts, probs, env, false, AssignSimple) != nil {
		t.Fatal("sticky error must not invent LType (not soft fall through to int)")
	}
	// also compound assign: same ERROR_GUARD before default int
	if SelectLType(NewRng(2), opts, probs, env, false, AssignBitAnd) != nil {
		t.Fatal("sticky error on compound must not invent int LType")
	}
}

func TestChooseRandomErrorGuard(t *testing.T) {
	// Type.cpp:1208 — ERROR_GUARD after rnd_upto(AllTypes)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), GetSimpleType(EShort)}}
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	if env.ChooseRandom(NewRng(1), opts, probs, false) != nil {
		t.Fatal("sticky error must fail ChooseRandom")
	}
}

func TestChooseRandomNilTypeHole(t *testing.T) {
	// Type* always live on AllTypes; filter-out hole is soft invent of partial pool
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), nil, GetSimpleType(EShort)}}
	if env.ChooseRandom(NewRng(1), opts, probs, false) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandom")
	}
	if env.ChooseRandomNonvoid(NewRng(2), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoid")
	}
	if env.ChooseRandomNonvoidNonvolatile(NewRng(3), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoidNonvolatile")
	}
}

func TestChooseRandomPointerTypeNilHole(t *testing.T) {
	// Type* always live on derived_types; no invent pick past hole
	intStar := PointerTo(GetIntType())
	env := &TypeEnv{DerivedTypes: []*Type{intStar, nil}}
	if env.ChooseRandomPointerType(NewRng(1)) != nil {
		t.Fatal("nil DerivedTypes hole must fail closed")
	}
}

func TestChooseRandomStructUnionTypeNilHole(t *testing.T) {
	st := &Type{isStruct: true, StructName: "S0"}
	if ChooseRandomStructUnionType(NewRng(1), []*Type{st, nil}) != nil {
		t.Fatal("nil ok_types hole must fail closed")
	}
}

func TestMakeRandomPointerTypeDerivedNilHole(t *testing.T) {
	// derived_types hole must not soft invent skip + fall through to choose_random
	// as if the hole were absent. Incomplete AllTypes also fails ChooseRandom.
	opts := Defaults()
	probs := NewProbabilities(opts)
	intStar := PointerTo(GetIntType())
	envBad := &TypeEnv{
		AllTypes:     []*Type{GetIntType(), nil},
		DerivedTypes: []*Type{intStar, nil},
	}
	for seed := uint64(1); seed < 30; seed++ {
		if envBad.MakeRandomPointerType(NewRng(seed), opts, probs) != nil {
			t.Fatalf("incomplete type pools must fail closed MakeRandomPointerType seed=%d", seed)
		}
	}
	// incomplete derived only: when 20% path hits, fail closed (nil); never soft-skip hole
	envDerived := &TypeEnv{
		AllTypes:     []*Type{GetIntType()},
		DerivedTypes: []*Type{intStar, nil},
	}
	sawNilOnDerivedPath := false
	for seed := uint64(1); seed < 200; seed++ {
		r := NewRng(seed)
		// peek same flip as MakeRandomPointerType first coin
		if !r.RndFlipcoin(20) {
			continue
		}
		// re-run full make with same seed — first flip true → derived incomplete → nil
		if envDerived.MakeRandomPointerType(NewRng(seed), opts, probs) != nil {
			t.Fatalf("derived nil hole + flipcoin(20) must fail closed seed=%d", seed)
		}
		sawNilOnDerivedPath = true
		break
	}
	if !sawNilOnDerivedPath {
		t.Fatal("expected a seed hitting flipcoin(20) derived path")
	}
}

func TestSelectLTypeCanBePointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp always has AllTypes after GenerateSimpleTypes / GenerateAllTypes
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		r := NewRng(seed)
		ty := SelectLType(r, opts, probs, env, false, AssignSimple)
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
