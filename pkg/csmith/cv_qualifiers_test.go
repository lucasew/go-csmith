package csmith

import "testing"

func TestNewCVQualifiersMismatchSticky(t *testing.T) {
	// C++ assert equal vector sizes; mismatch no invent truncated paired qfer
	ClearError()
	q := NewCVQualifiers([]bool{true, false}, []bool{false})
	if len(q.IsConsts) != 0 || len(q.IsVolatiles) != 0 {
		t.Fatalf("mismatch must fail closed empty, got consts=%v vols=%v", q.IsConsts, q.IsVolatiles)
	}
	if !HasError() {
		t.Fatal("mismatch NewCVQualifiers must SetError sticky")
	}
	ClearError()
	// unpaired shells sticky on Match / StricterThan
	unpaired := CVQualifiers{IsConsts: []bool{true, false}, IsVolatiles: []bool{false}}
	other := NewCVQualifiers([]bool{false}, []bool{false})
	if unpaired.Match(other, false) {
		t.Fatal("unpaired Match must fail closed false")
	}
	if !HasError() {
		t.Fatal("unpaired Match must SetError sticky")
	}
	ClearError()
	if unpaired.StricterThan(other) {
		t.Fatal("unpaired StricterThan must fail closed false")
	}
	if !HasError() {
		t.Fatal("unpaired StricterThan must SetError sticky")
	}
	ClearError()
}

func TestRandomStricterAndLooserConsts(t *testing.T) {
	// CVQualifiers.cpp:375–457 — exact match returns identity
	opts := Defaults()
	opts.MatchExactQualifiers = true
	q := NewCVQualifiers([]bool{true, false}, []bool{false, true})
	if got := q.RandomStricterConsts(NewRng(2), opts, NewProbabilities(opts)); !boolsEqual(got, q.IsConsts) {
		t.Fatal("exact stricter const", got)
	}
	if got := q.RandomLooserVolatiles(NewRng(2), opts, NewProbabilities(opts)); !boolsEqual(got, q.IsVolatiles) {
		t.Fatal("exact looser vol", got)
	}
	// non-exact: already-const level stays const under stricter
	opts.MatchExactQualifiers = false
	q2 := NewCVQualifiers([]bool{true}, []bool{false})
	got := q2.RandomStricterConsts(NewRng(2), opts, NewProbabilities(opts))
	if len(got) != 1 || !got[0] {
		t.Fatal("already const stays", got)
	}
	// looser: non-const stays false
	q3 := NewCVQualifiers([]bool{false}, []bool{false})
	got3 := q3.RandomLooserConsts(NewRng(2), opts, NewProbabilities(opts))
	if len(got3) != 1 || got3[0] {
		t.Fatal("non-const stays false", got3)
	}
	// CVQualifiers.cpp:390+ DEPTH_GUARD on flipcoin paths — random mode always GOOD
	q4 := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	if len(q4.RandomStricterConsts(NewRng(3), opts, NewProbabilities(opts))) != 2 {
		t.Fatal("stricter depth guard path")
	}
	added := q4.RandomAddQualifiers(NewRng(3), opts, NewProbabilities(opts), false)
	if len(added.IsConsts) != 3 {
		t.Fatalf("add level %d", len(added.IsConsts))
	}
	// nil RNG sticky — no invent fixed non-const non-vol pointer level
	ClearError()
	if got := q4.RandomAddQualifiers(nil, opts, NewProbabilities(opts), false); len(got.IsConsts) != len(q4.IsConsts) {
		t.Fatalf("nil RNG must not invent grow, got %d", len(got.IsConsts))
	}
	if !HasError() {
		t.Fatal("nil RNG RandomAddQualifiers must SetError sticky")
	}
	ClearError()
}

func TestRandomQualifiersFromNoInventWithoutRNG(t *testing.T) {
	// CVQualifiers.cpp always has process RNG sticky; no invent fixed looser/stricter shells
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	base := NewCVQualifiers([]bool{false}, []bool{false})
	base.AcceptStricter = true
	if out := base.RandomQualifiersFrom(false, AccessRead, EmptyCGContext(), opts, probs, nil); len(out.IsConsts) != 0 || len(out.IsVolatiles) != 0 {
		t.Fatalf("nil RNG From must fail closed empty, got %+v", out)
	}
	if !HasError() {
		t.Fatal("nil RNG RandomQualifiersFrom must SetError sticky")
	}
	ClearError()
	if out := base.RandomLooseQualifiers(false, AccessRead, EmptyCGContext(), opts, probs, nil); len(out.IsConsts) != 0 || len(out.IsVolatiles) != 0 {
		t.Fatalf("nil RNG Loose must fail closed empty, got %+v", out)
	}
	if !HasError() {
		t.Fatal("nil RNG RandomLooseQualifiers must SetError sticky")
	}
	ClearError()
	// wildcard still works without RNG (short-circuit before RNG)
	w := CVQualifiers{Wildcard: true}
	if out := w.RandomQualifiersFrom(true, AccessRead, EmptyCGContext(), opts, probs, nil); !out.Wildcard {
		t.Fatal("wildcard")
	}
	if HasError() {
		t.Fatal("wildcard RandomQualifiersFrom must not sticky on nil RNG")
	}
}

func TestRandomStricterLooserNilRNGAndProbs(t *testing.T) {
	// C++ always has RNG sticky; nil r must not invent identity bits as successful draw
	ClearError()
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if q.RandomStricterConsts(nil, opts, NewProbabilities(opts)) != nil {
		t.Fatal("nil RNG stricter const must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomStricterConsts must SetError sticky")
	}
	ClearError()
	if q.RandomStricterVolatiles(nil, opts, NewProbabilities(opts)) != nil {
		t.Fatal("nil RNG stricter vol must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomStricterVolatiles must SetError sticky")
	}
	ClearError()
	if q.RandomLooserConsts(nil, opts, NewProbabilities(opts)) != nil {
		t.Fatal("nil RNG looser const must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomLooserConsts must SetError sticky")
	}
	ClearError()
	if q.RandomLooserVolatiles(nil, opts, NewProbabilities(opts)) != nil {
		t.Fatal("nil RNG looser vol must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomLooserVolatiles must SetError sticky")
	}
	ClearError()
	// nil probs → 0% (no invent default 50); drawable non-const stays false under stricter
	got := q.RandomStricterConsts(NewRng(1), opts, nil)
	if len(got) != 1 || got[0] {
		t.Fatalf("nil probs stricter must not invent 50%% true, got %v", got)
	}
	// exact match still identity without RNG (no sticky — short-circuit before RNG)
	opts.MatchExactQualifiers = true
	q2 := NewCVQualifiers([]bool{true, false}, []bool{false, true})
	if got := q2.RandomStricterConsts(nil, opts, nil); !boolsEqual(got, q2.IsConsts) {
		t.Fatal("exact match still identity", got)
	}
	if HasError() {
		t.Fatal("exact match must not SetError on nil RNG")
	}
}

func TestRandomQualifiersFromNoVolatile(t *testing.T) {
	// CVQualifiers.cpp:194–225 — no_volatile clears all vols
	opts := Defaults()
	probs := NewProbabilities(opts)
	base := NewCVQualifiers([]bool{true, false}, []bool{true, true})
	base.AcceptStricter = true
	out := base.RandomQualifiersFrom(true, AccessRead, EmptyCGContext(), opts, probs, NewRng(2))
	for _, v := range out.IsVolatiles {
		if v {
			t.Fatal("no_volatile", out)
		}
	}
	if len(out.IsConsts) != 2 {
		t.Fatal(out)
	}
	// WRITE clears storage const
	outW := base.RandomQualifiersFrom(true, AccessWrite, EmptyCGContext(), opts, probs, NewRng(2))
	if len(outW.IsConsts) > 0 && outW.IsConsts[len(outW.IsConsts)-1] {
		t.Fatal("write no storage const", outW)
	}
}

func TestRandomLooseQualifiers(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	base := NewCVQualifiers([]bool{true}, []bool{true})
	out := base.RandomLooseQualifiers(true, AccessRead, EmptyCGContext(), opts, probs, NewRng(3))
	if len(out.IsVolatiles) != 1 || out.IsVolatiles[0] {
		t.Fatal(out)
	}
}

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
	// packed nested field Type hole sticky has-padding (no invent padding-free soft-skip)
	ClearError()
	st4 := &Type{isStruct: true, Packed: true, Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if !st4.HasPadding() {
		t.Fatal("nil field Type must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil field Type HasPadding must SetError sticky")
	}
	ClearError()
	// nested HasPadding residual soft invent was soft-continue later fields padding-free.
	// Fair: sticky has-padding true.
	innerHole := &Type{isStruct: true, Packed: true, Fields: []StructField{{Name: "x", Type: nil, BitWidth: -1}}}
	outer := &Type{isStruct: true, Packed: true, Fields: []StructField{
		{Name: "nest", Type: innerHole, BitWidth: -1},
		{Name: "ok", Type: GetIntType(), BitWidth: -1},
	}}
	if !outer.HasPadding() {
		t.Fatal("nested residual HasPadding must fail closed true")
	}
	if !HasError() {
		t.Fatal("nested residual HasPadding must SetError sticky")
	}
	ClearError()
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

func TestRandomQualifiersIncompleteAmbientSticky(t *testing.T) {
	// incomplete EffectContext fails closed sticky (no invent non-vol qfer past holes)
	opts := Defaults()
	ty := GetSimpleType(EInt)
	cg := WithEffectContext(IncompleteEffect())
	ClearError()
	q := RandomQualifiersForType(ty, AccessRead, cg, false, 0, 100, opts, NewRng(2))
	if len(q.IsConsts) != 0 || len(q.IsVolatiles) != 0 {
		t.Fatalf("incomplete ambient must fail closed empty qfer %+v", q)
	}
	if !HasError() {
		t.Fatal("incomplete ambient must SetError sticky")
	}
	ClearError()
	base := NewCVQualifiers([]bool{false}, []bool{false})
	q2 := base.RandomQualifiersFrom(false, AccessRead, cg, opts, NewProbabilities(opts), NewRng(3))
	if len(q2.IsConsts) != 0 || len(q2.IsVolatiles) != 0 {
		t.Fatalf("RandomQualifiersFrom incomplete ambient must empty %+v", q2)
	}
	if !HasError() {
		t.Fatal("RandomQualifiersFrom incomplete ambient must SetError sticky")
	}
	ClearError()
	// Type + RNG always live; sticky empty (no invent soft-skip qfer past hole)
	q3 := RandomQualifiersForType(nil, AccessRead, EmptyCGContext(), false, 0, 100, opts, NewRng(1))
	if len(q3.IsConsts) != 0 || len(q3.IsVolatiles) != 0 {
		t.Fatalf("nil type must fail closed empty qfer %+v", q3)
	}
	if !HasError() {
		t.Fatal("nil type RandomQualifiersForType must SetError sticky")
	}
	ClearError()
	q4 := RandomQualifiersForType(ty, AccessRead, EmptyCGContext(), false, 0, 100, opts, nil)
	if len(q4.IsConsts) != 0 || len(q4.IsVolatiles) != 0 {
		t.Fatalf("nil rng must fail closed empty qfer %+v", q4)
	}
	if !HasError() {
		t.Fatal("nil rng RandomQualifiersForType must SetError sticky")
	}
	ClearError()
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
	ClearError()
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
	// CVQualifiers.cpp:569/578 — assert OOB; fail closed as const/vol (non-sticky soft filter)
	if !q.IsConstAfterDeref(5) {
		t.Fatal("OOB deref must fail closed as const")
	}
	if !q.IsVolatileAfterDeref(5) {
		t.Fatal("OOB deref must fail closed as volatile")
	}
	// empty qfer remains non-const (zero-value)
	if (CVQualifiers{}).IsConstAfterDeref(0) {
		t.Fatal("empty qfer not const")
	}
	// Variable.IsConstAfterDeref residual: Type-nil after non-const qfer soft invent was non-const WRITE OK.
	// Fair: sticky const true (restrictive).
	ClearError()
	v := &Variable{Name: "g_p", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if !v.IsConstAfterDeref(0) {
		t.Fatal("Type-nil IsConstAfterDeref must fail closed const true")
	}
	if !HasError() {
		t.Fatal("Type-nil IsConstAfterDeref must SetError sticky")
	}
	ClearError()
	// IsVolatileAfterDeref residual same
	if !v.IsVolatileAfterDeref(0) {
		t.Fatal("Type-nil IsVolatileAfterDeref must fail closed volatile true")
	}
	if !HasError() {
		t.Fatal("Type-nil IsVolatileAfterDeref must SetError sticky")
	}
	ClearError()
}
