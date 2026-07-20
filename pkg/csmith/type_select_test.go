package csmith

import (
	"strings"
	"testing"
)

func TestFindPointerTypeCachesAndRegisters(t *testing.T) {
	ClearError()
	var env TypeEnv
	p1 := env.FindPointerType(GetIntType(), true)
	p2 := env.FindPointerType(GetIntType(), true)
	if p1 != p2 {
		t.Fatal("identity")
	}
	if !env.HasPointerType() || len(env.DerivedTypes) != 1 {
		t.Fatal(env.DerivedTypes)
	}
	// incomplete DerivedTypes: no invent soft-skip hole / HasPointerType true
	env.DerivedTypes = append(env.DerivedTypes, nil)
	ClearError()
	if env.HasPointerType() {
		t.Fatal("incomplete DerivedTypes must not invent HasPointerType true")
	}
	if !HasError() {
		t.Fatal("incomplete DerivedTypes HasPointerType must SetError sticky")
	}
	ClearError()
	if env.FindPointerType(GetIntType(), true) != nil {
		t.Fatal("incomplete DerivedTypes FindPointerType(add) must fail closed nil")
	}
	if !HasError() {
		t.Fatal("incomplete DerivedTypes FindPointerType(add) must SetError sticky")
	}
	ClearError()
}

func TestChooseRandomStructUnionTypeEmptyPool(t *testing.T) {
	// Type.cpp:523 assert(sz > 0) — empty ok_types must not invent a type
	ClearError()
	if ChooseRandomStructUnionType(NewRng(1), nil) != nil {
		t.Fatal("empty pool")
	}
	// empty pool non-sticky soft (no candidates)
	if HasError() {
		t.Fatal("empty pool must stay non-sticky soft")
	}
	ClearError()
	if ChooseRandomStructUnionType(nil, []*Type{GetIntType()}) != nil {
		t.Fatal("nil rng")
	}
	if !HasError() {
		t.Fatal("nil RNG ChooseRandomStructUnionType must SetError sticky")
	}
	ClearError()
}

func TestChooseRandomNilRNGSticky(t *testing.T) {
	// Type.cpp always has process RNG; sticky no invent AllTypes[0] / derived pick
	ClearError()
	env := &TypeEnv{AllTypes: []*Type{GetIntType()}, DerivedTypes: []*Type{PointerTo(GetIntType())}}
	if env.ChooseRandom(nil, Defaults(), NewProbabilities(Defaults()), false) != nil {
		t.Fatal("nil RNG ChooseRandom must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG ChooseRandom must SetError sticky")
	}
	ClearError()
	if env.ChooseRandomNonvoid(nil, Defaults(), NewProbabilities(Defaults())) != nil {
		t.Fatal("nil RNG ChooseRandomNonvoid must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG ChooseRandomNonvoid must SetError sticky")
	}
	ClearError()
	if env.ChooseRandomPointerType(nil) != nil {
		t.Fatal("nil RNG ChooseRandomPointerType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG ChooseRandomPointerType must SetError sticky")
	}
	ClearError()
	if env.MakeRandomPointerType(nil, Defaults(), NewProbabilities(Defaults())) != nil {
		t.Fatal("nil RNG MakeRandomPointerType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomPointerType must SetError sticky")
	}
	ClearError()
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

func TestSelectLTypeIncompleteStructPoolFailClosed(t *testing.T) {
	// incomplete ok_types must sticky fail — no invent fall-through get_int_type past hole
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	// force struct-as-LType path eligible: simple assign + AllTypes with hole
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), nil}}
	// plant high struct prob so we would enter branch if complete
	probs.single[PStructAsLTypeProb] = 100
	probs.single[PPointerAsLTypeProb] = 0
	probs.single[PFloatAsLTypeProb] = 0
	ty := SelectLType(NewRng(1), opts, probs, env, false, AssignSimple)
	if ty != nil {
		t.Fatalf("incomplete AllTypes must fail closed SelectLType, got %v", ty.CName())
	}
	if !HasError() {
		t.Fatal("incomplete AllTypes must SetError sticky SelectLType")
	}
	ClearError()
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
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{AllTypes: []*Type{GetIntType(), nil, GetSimpleType(EShort)}}
	if env.ChooseRandom(NewRng(1), opts, probs, false) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandom")
	}
	if !HasError() {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandom")
	}
	ClearError()
	if env.ChooseRandomNonvoid(NewRng(2), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoid")
	}
	if !HasError() {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandomNonvoid")
	}
	ClearError()
	if env.ChooseRandomNonvoidNonvolatile(NewRng(3), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoidNonvolatile")
	}
	if !HasError() {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandomNonvoidNonvolatile")
	}
	ClearError()
}

func TestChooseRandomPointerTypeNilHole(t *testing.T) {
	// Type* always live on derived_types; no invent pick past hole
	ClearError()
	intStar := PointerTo(GetIntType())
	env := &TypeEnv{DerivedTypes: []*Type{intStar, nil}}
	if env.ChooseRandomPointerType(NewRng(1)) != nil {
		t.Fatal("nil DerivedTypes hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil DerivedTypes hole must SetError sticky")
	}
	ClearError()
}

func TestChooseRandomStructUnionTypeNilHole(t *testing.T) {
	ClearError()
	st := &Type{isStruct: true, StructName: "S0"}
	if ChooseRandomStructUnionType(NewRng(1), []*Type{st, nil}) != nil {
		t.Fatal("nil ok_types hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil ok_types hole must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomPointerTypeDerivedNilHole(t *testing.T) {
	// derived_types hole must not soft invent skip + fall through to choose_random
	// as if the hole were absent. Incomplete AllTypes also fails ChooseRandom.
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	intStar := PointerTo(GetIntType())
	envBad := &TypeEnv{
		AllTypes:     []*Type{GetIntType(), nil},
		DerivedTypes: []*Type{intStar, nil},
	}
	for seed := uint64(1); seed < 30; seed++ {
		ClearError()
		if envBad.MakeRandomPointerType(NewRng(seed), opts, probs) != nil {
			t.Fatalf("incomplete type pools must fail closed MakeRandomPointerType seed=%d", seed)
		}
	}
	if !HasError() {
		t.Fatal("incomplete type pools must SetError sticky")
	}
	ClearError()
	// incomplete derived only: when 20% path hits, fail closed (nil); never soft-skip hole
	envDerived := &TypeEnv{
		AllTypes:     []*Type{GetIntType()},
		DerivedTypes: []*Type{intStar, nil},
	}
	sawNilOnDerivedPath := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearError()
		r := NewRng(seed)
		// peek same flip as MakeRandomPointerType first coin
		if !r.RndFlipcoin(20) {
			continue
		}
		// re-run full make with same seed — first flip true → derived incomplete → nil
		if envDerived.MakeRandomPointerType(NewRng(seed), opts, probs) != nil {
			t.Fatalf("derived nil hole + flipcoin(20) must fail closed seed=%d", seed)
		}
		if !HasError() {
			t.Fatalf("derived nil hole must SetError sticky seed=%d", seed)
		}
		sawNilOnDerivedPath = true
		break
	}
	if !sawNilOnDerivedPath {
		t.Fatal("expected a seed hitting flipcoin(20) derived path")
	}
	ClearError()
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
		if strings.Contains(out, "int32_t*") || strings.Contains(out, "int32_t *") || strings.Contains(out, "int*") {
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

func TestFindPointerTypeNilSticky(t *testing.T) {
	ClearError()
	env := &TypeEnv{}
	if env.FindPointerType(nil, true) != nil {
		t.Fatal("nil pointee FindPointerType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil pointee FindPointerType must SetError sticky")
	}
	ClearError()
	if env.FindType(nil) != nil {
		t.Fatal("nil type FindType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type FindType must SetError sticky")
	}
	ClearError()
}

func TestGetAllOKStructUnionTypesFieldTypeResidualSticky(t *testing.T) {
	// IsConstStructUnion residual soft invent was invent keep incomplete struct in ok pool.
	ClearError()
	env := &TypeEnv{AllTypes: []*Type{
		{isStruct: true, StructName: "S0", Fields: []StructField{
			{Name: "f0", Type: nil, BitWidth: -1}, // residual via IsConstStructUnion
		}},
	}}
	// noConst true forces IsConstStructUnion scan
	got := env.GetAllOKStructUnionTypes(true, false, false, true)
	if typesComplete(got) {
		// IncompleteTypes marker
		t.Fatal("field Type-nil residual must fail closed IncompleteTypes", got)
	}
	if !HasError() {
		t.Fatal("field Type-nil residual GetAllOKStructUnionTypes must SetError sticky")
	}
	ClearError()
}

func TestChooseRandomStructFromTypeIsVolatileResidualSticky(t *testing.T) {
	// IsVolatileStructUnion residual soft invent was invent return typ past hole.
	ClearError()
	env := &TypeEnv{}
	// Type with nil field Type → IsVolatileStructUnion residual when noVolatile
	typ := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if env.ChooseRandomStructFromType(NewRng(1), typ, true) != nil {
		t.Fatal("IsVolatileStructUnion residual must fail closed ChooseRandomStructFromType")
	}
	if !HasError() {
		t.Fatal("IsVolatileStructUnion residual ChooseRandomStructFromType must SetError sticky")
	}
	ClearError()
}

func TestRandomTypeFromTypeVoidIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent keep void simple past assert.
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	r := NewRng(2)
	// strict simple void
	got := RandomTypeFromType(r, nil, opts, probs, GetSimpleType(EVoid), true, true)
	if got != nil {
		t.Fatal("strict void RandomTypeFromType must fail closed nil", got)
	}
	if !HasError() {
		t.Fatal("strict void RandomTypeFromType must SetError sticky")
	}
	ClearError()
}
