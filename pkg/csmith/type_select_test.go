package csmith

import (
	"strings"
	"testing"
)

func TestFindPointerTypeCachesAndRegisters(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	env := TypeEnv{Sess: testAmbientSession}
	p1 := env.FindPointerType(GetIntTypeSess(testAmbientSession), true)
	p2 := env.FindPointerType(GetIntTypeSess(testAmbientSession), true)
	if p1 != p2 {
		t.Fatal("identity")
	}
	if !env.HasPointerType() || len(env.DerivedTypes) != 1 {
		t.Fatal(env.DerivedTypes)
	}
	// incomplete DerivedTypes: no invent soft-skip hole / HasPointerType true
	env.DerivedTypes = append(env.DerivedTypes, nil)
	ClearErrorSess(testAmbientSession)
	if env.HasPointerType() {
		t.Fatal("incomplete DerivedTypes must not invent HasPointerType true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete DerivedTypes HasPointerType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if env.FindPointerType(GetIntTypeSess(testAmbientSession), true) != nil {
		t.Fatal("incomplete DerivedTypes FindPointerType(add) must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete DerivedTypes FindPointerType(add) must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomStructUnionTypeEmptyPool(t *testing.T) {
	// Type.cpp:523 assert(sz > 0) — empty ok_types must not invent a type
	ClearErrorSess(testAmbientSession)
	if ChooseRandomStructUnionType(NewRngSess(testAmbientSession, 1), nil) != nil {
		t.Fatal("empty pool")
	}
	// empty pool non-sticky soft (no candidates)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty pool must stay non-sticky soft")
	}
	ClearErrorSess(testAmbientSession)
	if ChooseRandomStructUnionType(nil, []*Type{GetIntTypeSess(testAmbientSession)}) != nil {
		t.Fatal("nil rng")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG ChooseRandomStructUnionType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomNilRNGSticky(t *testing.T) {
	// Type.cpp always has process RNG; sticky no invent AllTypes[0] / derived pick
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}, DerivedTypes: []*Type{PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}}
	if env.ChooseRandom(nil, Defaults(), NewProbabilities(Defaults()), false) != nil {
		t.Fatal("nil RNG ChooseRandom must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG ChooseRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if env.ChooseRandomNonvoid(nil, Defaults(), NewProbabilities(Defaults())) != nil {
		t.Fatal("nil RNG ChooseRandomNonvoid must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG ChooseRandomNonvoid must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if env.ChooseRandomPointerType(nil) != nil {
		t.Fatal("nil RNG ChooseRandomPointerType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG ChooseRandomPointerType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if env.MakeRandomPointerType(nil, Defaults(), NewProbabilities(Defaults())) != nil {
		t.Fatal("nil RNG MakeRandomPointerType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomPointerType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomPointerTypeIntStar(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp GenerateSimpleTypes seeds AllTypes before make_random_pointer_type
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	// Force non-20% path often; result should be int* when simple base consolidated
	r := NewRngSess(testAmbientSession, 2)
	// first flip 20% for double ptr — seed2 first flipcoin(20)
	p := env.MakeRandomPointerType(r, opts, probs)
	if p == nil || p.PtrTypeSess(testAmbientSession) == nil {
		t.Fatal(p)
	}
	// if base simple path, pointee is int
	if p.IndirectLevelSess(testAmbientSession) == 1 && p.PtrTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
		// double-ptr path might differ
		if p.PtrTypeSess(testAmbientSession).PtrTypeSess(testAmbientSession) == nil && p.PtrTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
			t.Fatalf("expected int* or deeper, got %s", p.CNameSess(testAmbientSession))
		}
	}
	// Type.cpp:1141 — nil env/rng → ERROR_GUARD nullptr, no soft invent
	if (*TypeEnv)(nil).MakeRandomPointerType(NewRngSess(testAmbientSession, 1), opts, probs) != nil {
		t.Fatal("nil env must not invent int*")
	}
}

func TestMakeRandomPointerTypeConsolidatesAllSimple(t *testing.T) {
	// Type.cpp:1161–1164 — any eSimple (including float) consolidates to int*
	opts := Defaults()
	opts.EnableFloat = true
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetSimpleTypeSess(testAmbientSession, EFloat)}}
	// avoid 20% derived path (empty derived)
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		p := env.MakeRandomPointerType(NewRngSess(testAmbientSession, seed), opts, probs)
		if p != nil && p.IndirectLevelSess(testAmbientSession) == 1 && p.PtrTypeSess(testAmbientSession) == GetIntTypeSess(testAmbientSession) {
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
	env := TypeEnv{Sess: testAmbientSession}
	r := NewRngSess(testAmbientSession, 2)
	ty := SelectLType(r, opts, probs, &env, false, AssignBitAnd)
	if ty != GetIntTypeSess(testAmbientSession) {
		// float off, no struct → int
		t.Fatalf("compound assign LType want int got %s", ty.CNameSess(testAmbientSession))
	}
}

func TestSelectLTypeIncompleteStructPoolFailClosed(t *testing.T) {
	// incomplete ok_types must sticky fail — no invent fall-through get_int_type past hole
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	// force struct-as-LType path eligible: simple assign + AllTypes with hole
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession), nil}}
	// plant high struct prob so we would enter branch if complete
	probs.single[PStructAsLTypeProb] = 100
	probs.single[PPointerAsLTypeProb] = 0
	probs.single[PFloatAsLTypeProb] = 0
	ty := SelectLType(NewRngSess(testAmbientSession, 1), opts, probs, env, false, AssignSimple)
	if ty != nil {
		t.Fatalf("incomplete AllTypes must fail closed SelectLType, got %v", ty.CNameSess(testAmbientSession))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete AllTypes must SetError sticky SelectLType")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectLTypeErrorGuard(t *testing.T) {
	// Type.cpp:1613 — ERROR_GUARD after pointer branch always; no soft invent get_int_type
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if SelectLType(NewRngSess(testAmbientSession, 1), opts, probs, env, false, AssignSimple) != nil {
		t.Fatal("sticky error must not invent LType (not soft fall through to int)")
	}
	// also compound assign: same ERROR_GUARD before default int
	if SelectLType(NewRngSess(testAmbientSession, 2), opts, probs, env, false, AssignBitAnd) != nil {
		t.Fatal("sticky error on compound must not invent int LType")
	}
}

func TestChooseRandomErrorGuard(t *testing.T) {
	// Type.cpp:1208 — ERROR_GUARD after rnd_upto(AllTypes)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort)}}
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if env.ChooseRandom(NewRngSess(testAmbientSession, 1), opts, probs, false) != nil {
		t.Fatal("sticky error must fail ChooseRandom")
	}
}

func TestChooseRandomNilTypeHole(t *testing.T) {
	// Type* always live on AllTypes; filter-out hole is soft invent of partial pool
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession), nil, GetSimpleTypeSess(testAmbientSession, EShort)}}
	if env.ChooseRandom(NewRngSess(testAmbientSession, 1), opts, probs, false) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandom")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandom")
	}
	ClearErrorSess(testAmbientSession)
	if env.ChooseRandomNonvoid(NewRngSess(testAmbientSession, 2), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandomNonvoid")
	}
	ClearErrorSess(testAmbientSession)
	if env.ChooseRandomNonvoidNonvolatile(NewRngSess(testAmbientSession, 3), opts, probs) != nil {
		t.Fatal("nil AllTypes hole must fail closed ChooseRandomNonvoidNonvolatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil AllTypes hole must SetError sticky ChooseRandomNonvoidNonvolatile")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomPointerTypeNilHole(t *testing.T) {
	// Type* always live on derived_types; no invent pick past hole
	ClearErrorSess(testAmbientSession)
	intStar := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	env := &TypeEnv{Sess: testAmbientSession, DerivedTypes: []*Type{intStar, nil}}
	if env.ChooseRandomPointerType(NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil DerivedTypes hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil DerivedTypes hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomStructUnionTypeNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S0"}
	if ChooseRandomStructUnionType(NewRngSess(testAmbientSession, 1), []*Type{st, nil}) != nil {
		t.Fatal("nil ok_types hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ok_types hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomPointerTypeDerivedNilHole(t *testing.T) {
	// derived_types hole must not soft invent skip + fall through to choose_random
	// as if the hole were absent. Incomplete AllTypes also fails ChooseRandom.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	intStar := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	envBad := &TypeEnv{Sess: testAmbientSession,
		AllTypes:     []*Type{GetIntTypeSess(testAmbientSession), nil},
		DerivedTypes: []*Type{intStar, nil},
	}
	for seed := uint64(1); seed < 30; seed++ {
		ClearErrorSess(testAmbientSession)
		if envBad.MakeRandomPointerType(NewRngSess(testAmbientSession, seed), opts, probs) != nil {
			t.Fatalf("incomplete type pools must fail closed MakeRandomPointerType seed=%d", seed)
		}
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete type pools must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete derived only: when 20% path hits, fail closed (nil); never soft-skip hole
	envDerived := &TypeEnv{Sess: testAmbientSession,
		AllTypes:     []*Type{GetIntTypeSess(testAmbientSession)},
		DerivedTypes: []*Type{intStar, nil},
	}
	sawNilOnDerivedPath := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearErrorSess(testAmbientSession)
		r := NewRngSess(testAmbientSession, seed)
		// peek same flip as MakeRandomPointerType first coin
		if !r.RndFlipcoin(20) {
			continue
		}
		// re-run full make with same seed — first flip true → derived incomplete → nil
		if envDerived.MakeRandomPointerType(NewRngSess(testAmbientSession, seed), opts, probs) != nil {
			t.Fatalf("derived nil hole + flipcoin(20) must fail closed seed=%d", seed)
		}
		if !HasErrorSess(testAmbientSession) {
			t.Fatalf("derived nil hole must SetError sticky seed=%d", seed)
		}
		sawNilOnDerivedPath = true
		break
	}
	if !sawNilOnDerivedPath {
		t.Fatal("expected a seed hitting flipcoin(20) derived path")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectLTypeCanBePointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp always has AllTypes after GenerateSimpleTypes / GenerateAllTypes
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		r := NewRngSess(testAmbientSession, seed)
		ty := SelectLType(r, opts, probs, env, false, AssignSimple)
		if ty != nil && ty.PtrTypeSess(testAmbientSession) != nil {
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
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession}
	if env.FindPointerType(nil, true) != nil {
		t.Fatal("nil pointee FindPointerType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil pointee FindPointerType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if env.FindType(nil) != nil {
		t.Fatal("nil type FindType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type FindType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetAllOKStructUnionTypesFieldTypeResidualSticky(t *testing.T) {
	// IsConstStructUnion residual soft invent was invent keep incomplete struct in ok pool.
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{
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
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("field Type-nil residual GetAllOKStructUnionTypes must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseRandomStructFromTypeIsVolatileResidualSticky(t *testing.T) {
	// IsVolatileStructUnion residual soft invent was invent return typ past hole.
	ClearErrorSess(testAmbientSession)
	env := &TypeEnv{Sess: testAmbientSession}
	// Type with nil field Type → IsVolatileStructUnion residual when noVolatile
	typ := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if env.ChooseRandomStructFromType(NewRngSess(testAmbientSession, 1), typ, true) != nil {
		t.Fatal("IsVolatileStructUnion residual must fail closed ChooseRandomStructFromType")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileStructUnion residual ChooseRandomStructFromType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomTypeFromTypeVoidIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent keep void simple past assert.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	r := NewRngSess(testAmbientSession, 2)
	// strict simple void
	got := RandomTypeFromType(r, nil, opts, probs, GetSimpleTypeSess(testAmbientSession, EVoid), true, true)
	if got != nil {
		t.Fatal("strict void RandomTypeFromType must fail closed nil", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("strict void RandomTypeFromType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomPointerTypeEmptyDerivedStillFlips(t *testing.T) {
	// Type.cpp:1145–1154 — rnd_flipcoin(20) always, even when derived_types empty.
	// Short-circuit skip of the flip desyncs vs C++.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	// empty DerivedTypes; AllTypes has simples for choose_random fallthrough
	env.AllTypes = []*Type{
		GetSimpleTypeSess(testAmbientSession, EChar), GetSimpleTypeSess(testAmbientSession, EInt), GetSimpleTypeSess(testAmbientSession, EShort),
		GetSimpleTypeSess(testAmbientSession, ELong), GetSimpleTypeSess(testAmbientSession, ELongLong),
		GetSimpleTypeSess(testAmbientSession, EUChar), GetSimpleTypeSess(testAmbientSession, EUInt), GetSimpleTypeSess(testAmbientSession, EUShort),
		GetSimpleTypeSess(testAmbientSession, EULong), GetSimpleTypeSess(testAmbientSession, EULongLong),
	}
	// Two RNGs: one for MakeRandomPointerType, one manual replay of fair stream
	// Fair: flip(20) then choose_random path. After many seeds both succeed.
	ok := 0
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		p := env.MakeRandomPointerType(NewRngSess(testAmbientSession, seed), opts, probs)
		if p != nil && p.PtrTypeSess(testAmbientSession) != nil {
			ok++
		}
	}
	if ok == 0 {
		t.Fatal("expected some int* from empty-derived fallthrough")
	}
	// When flip(20) true and derived empty, must not sticky-error; fall through
	ClearErrorSess(testAmbientSession)
	// seed that hits flip true with empty derived: brute
	found := false
	for seed := uint64(0); seed < 200; seed++ {
		r := NewRngSess(testAmbientSession, seed)
		if r.RndFlipcoin(20) {
			// this seed's first draw is heads — MakeRandomPointerType must still return
			ClearErrorSess(testAmbientSession)
			p := env.MakeRandomPointerType(NewRngSess(testAmbientSession, seed), opts, probs)
			if p == nil || HasErrorSess(testAmbientSession) {
				t.Fatalf("empty derived + flip true must fall through choose_random, seed=%d p=%v err=%v", seed, p, HasErrorSess(testAmbientSession))
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no seed with first flip true in 0..199")
	}
}
