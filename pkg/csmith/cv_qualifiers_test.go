package csmith

import "testing"

func TestIsVolatileOKOnOneLevelCPP(t *testing.T) {
	// CVQualifiers.cpp:269–293
	opts := Defaults()
	opts.LangCPP = true
	// non-aggregate always OK
	if !isVolatileOKOnOneLevel(opts, GetIntType()) {
		t.Fatal("simple")
	}
	// struct without assign ops → false
	st := &Type{isStruct: true, HasAssignOps: false}
	if isVolatileOKOnOneLevel(opts, st) {
		t.Fatal("no assign ops")
	}
	st.HasAssignOps = true
	if !isVolatileOKOnOneLevel(opts, st) {
		t.Fatal("struct with assign ops")
	}
	// union with nested struct → false
	nested := &Type{isStruct: true, HasAssignOps: true}
	ut := &Type{isUnion: true, HasAssignOps: true, Fields: []StructField{
		{Name: "f0", Type: nested, BitWidth: -1},
	}}
	if isVolatileOKOnOneLevel(opts, ut) {
		t.Fatal("union+struct")
	}
	// union of simples OK
	ut2 := &Type{isUnion: true, HasAssignOps: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if !isVolatileOKOnOneLevel(opts, ut2) {
		t.Fatal("union simples")
	}
	// C mode always OK even without assign ops
	optsC := Defaults()
	optsC.LangCPP = false
	if !isVolatileOKOnOneLevel(optsC, st) {
		t.Fatal("C mode")
	}
}

func TestHasPadding(t *testing.T) {
	// unpacked struct always pads
	st := &Type{isStruct: true, Packed: false}
	if !st.HasPadding() {
		t.Fatal("unpacked")
	}
	// packed without bitfields → no
	st2 := &Type{isStruct: true, Packed: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if st2.HasPadding() {
		t.Fatal("packed no bitfield")
	}
	// packed with bitfield → yes
	st3 := &Type{isStruct: true, Packed: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: 3},
	}}
	if !st3.HasPadding() {
		t.Fatal("bitfield")
	}
}

func TestRandomQualifiersSimpleNoVolatile(t *testing.T) {
	// CVQualifiers::random_qualifiers(t) — READ, empty ctx, no_volatile.
	// Only const flipcoin(RegularConstProb=10); vol forced false.
	opts := Defaults()
	probs := NewProbabilities(opts)
	ty := GetSimpleType(EInt)
	r := NewRng(2)
	q := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	if len(q.IsConsts) != 1 || len(q.IsVolatiles) != 1 {
		t.Fatalf("simple depth: consts=%v vols=%v", q.IsConsts, q.IsVolatiles)
	}
	if q.IsVolatiles[0] {
		t.Fatal("no_volatile must clear storage volatile")
	}
	// First flip is const only (vol skipped because no_volatile path still
	// evaluates volatile_ok then sets false after — actually no_volatile clears after.
	// For no_volatile: still runs flipcoins for vol then clears all.
	// CVQualifiers.cpp:337–348: still flipcoins vol then zero all if no_volatile.
}

func TestRandomQualifiersSimpleWriteNoConst(t *testing.T) {
	// WRITE → const_ok false → storage const false (no const flipcoin).
	opts := Defaults()
	probs := NewProbabilities(opts)
	ty := GetSimpleType(EInt)
	r := NewRng(2)
	// no_volatile true so only possible draws: for WRITE, only vol flip if SE-free then cleared.
	// WRITE: const_ok=false → no const flip. vol flip then cleared.
	q := RandomQualifiersDefaultProbs(ty, AccessWrite, EmptyCGContext(), true, opts, probs, r)
	if q.IsConsts[0] {
		t.Fatal("WRITE must not set storage const")
	}
}

func TestRandomQualifiersSideEffectBlocksVolatile(t *testing.T) {
	// effect_context not SE-free → volatile_ok false for storage → no vol flip for storage.
	opts := Defaults()
	ty := GetSimpleType(EInt)
	cg := WithEffectContext(WithSideEffects())
	// Use volatile_prob=100 const=0 so if vol were OK it would always be true.
	r := NewRng(2)
	q := RandomQualifiersForType(ty, AccessRead, cg, false, 0, 100, opts, r)
	if q.IsVolatiles[0] {
		t.Fatal("non-SE-free context must block storage volatile")
	}
	if q.IsConsts[0] {
		t.Fatal("const_prob 0 → no const")
	}
}

func TestRandomQualifiersSEFreeVolatileAlways(t *testing.T) {
	opts := Defaults()
	ty := GetSimpleType(EInt)
	r := NewRng(2)
	q := RandomQualifiersForType(ty, AccessRead, EmptyCGContext(), false, 0, 100, opts, r)
	if !q.IsVolatiles[0] {
		t.Fatal("SE-free + vol_prob 100 → storage volatile")
	}
}

func TestRandomQualifiersDisallowConstVolatile(t *testing.T) {
	opts := Defaults()
	opts.AllowConstVolatile = false
	ty := GetSimpleType(EInt)
	// Force both flips true with prob 100; const must be cleared when both set.
	r := NewRng(2)
	q := RandomQualifiersForType(ty, AccessRead, EmptyCGContext(), false, 100, 100, opts, r)
	if q.IsVolatiles[0] && q.IsConsts[0] {
		t.Fatal("allow_const_volatile false → cannot be both")
	}
	if !q.IsVolatiles[0] {
		t.Fatal("vol_prob 100 SE-free → volatile")
	}
	if q.IsConsts[0] {
		t.Fatal("const cleared when both would be true")
	}
}

func TestRandomQualifiersPointerLevels(t *testing.T) {
	// int* → two qualifier levels: [pointee, pointer-var]
	opts := Defaults()
	ty := PointerTo(GetSimpleType(EInt))
	if ty.IndirectLevel() != 1 {
		t.Fatalf("indirect level: %d", ty.IndirectLevel())
	}
	r := NewRng(2)
	// no_volatile, const_prob 0 → all false; still correct depths
	q := RandomQualifiersForType(ty, AccessRead, EmptyCGContext(), true, 0, 0, opts, r)
	if len(q.IsConsts) != 2 || len(q.IsVolatiles) != 2 {
		t.Fatalf("int* depth want 2, got consts=%v vols=%v", q.IsConsts, q.IsVolatiles)
	}
}

func TestRandomQualifiersPointerDrawOrderSeed2(t *testing.T) {
	// CVQualifiers.cpp order for int*:
	//   pointee: flip vol, flip const; then storage: flip vol, flip const (if allowed).
	// no_volatile=false, const=50, vol=50, SE-free READ.
	opts := Defaults()
	ty := PointerTo(GetSimpleType(EInt))
	r := NewRng(2)
	q := RandomQualifiersForType(ty, AccessRead, EmptyCGContext(), false, 50, 50, opts, r)

	// Independent replay of flip order
	r2 := NewRng(2)
	// pointee vol, const
	pv := r2.RndFlipcoin(50)
	pc := r2.RndFlipcoin(50)
	if pv && pc && !opts.AllowConstVolatile {
		pc = false
	}
	// storage vol, const
	sv := r2.RndFlipcoin(50)
	sc := r2.RndFlipcoin(50)
	if sv && sc && !opts.AllowConstVolatile {
		sc = false
	}
	wantC := []bool{pc, sc}
	wantV := []bool{pv, sv}
	if len(q.IsConsts) != 2 || q.IsConsts[0] != wantC[0] || q.IsConsts[1] != wantC[1] {
		t.Fatalf("consts got %v want %v", q.IsConsts, wantC)
	}
	if len(q.IsVolatiles) != 2 || q.IsVolatiles[0] != wantV[0] || q.IsVolatiles[1] != wantV[1] {
		t.Fatalf("vols got %v want %v", q.IsVolatiles, wantV)
	}
}

func TestMakeScalarVolatilesClearsInner(t *testing.T) {
	opts := Defaults()
	opts.VolatilePointers = false
	vols := []bool{true, true, true}
	MakeScalarVolatiles(opts, vols)
	if !vols[0] || vols[1] || vols[2] {
		t.Fatalf("only index 0 may stay: %v", vols)
	}
}

func TestIsConstAfterDeref(t *testing.T) {
	// is_consts = [outerPtr, storage]; deref 0 → storage (last); deref 1 → outerPtr
	q := NewCVQualifiers([]bool{true, false}, []bool{false, true})
	if q.IsConstAfterDeref(0) {
		t.Fatal("deref0 should be storage false")
	}
	if !q.IsConstAfterDeref(1) {
		t.Fatal("deref1 should be outer true")
	}
	if !q.IsVolatileAfterDeref(0) {
		t.Fatal("deref0 vol storage true")
	}
}
