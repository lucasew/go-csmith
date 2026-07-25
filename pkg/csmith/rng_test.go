package csmith

import "testing"

// Reference LCG values for srand48/lrand48 (glibc) — independent of residual-era code.
// AbsRndNumGenerator::seedrand + genrand.
// Computed: X0=(seed<<16)+0x330E; Xi+1=a*Xi+c mod 2^48; out=Xi>>17.

func TestGenrandSeed2Sequence(t *testing.T) {
	// First 8 lrand48 outputs after srand48(2).
	want := []uint32{
		1959434203,
		341627945,
		1231072447,
		1721222818,
		1189008653,
		467581419,
		1466698862,
		1314478799,
	}
	r := NewRngSess(testAmbientSession, 2)
	for i, w := range want {
		got := r.Genrand()
		if got != w {
			t.Fatalf("Genrand seed=2 i=%d: got %d want %d", i, got, w)
		}
	}
}

func TestGenrandSeed0And1(t *testing.T) {
	cases := map[uint64][]uint32{
		0: {366850414, 1610402240, 206956554, 1869309841},
		1: {89400484, 976015093, 1792756325, 721524505},
	}
	for seed, want := range cases {
		r := NewRngSess(testAmbientSession, seed)
		for i, w := range want {
			got := r.Genrand()
			if got != w {
				t.Fatalf("Genrand seed=%d i=%d: got %d want %d", seed, i, got, w)
			}
		}
	}
}

func TestRndUptoUsesModulo(t *testing.T) {
	// DefaultRndNumGenerator::rnd_upto: v = genrand() % n; rand_depth++.
	ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	// first genrand % 10 == 1959434203 % 10 == 3
	if got := r.RndUpto(10); got != 3 {
		t.Fatalf("RndUpto(10) first: got %d want 3", got)
	}
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth after one RndUpto: got %d want 1", r.RandDepth())
	}
	// second 341627945 % 10 == 5
	if got := r.RndUpto(10); got != 5 {
		t.Fatalf("RndUpto(10) second: got %d want 5", got)
	}
	// n==0 undefined non-sticky fail closed (soft re-pick empty domain)
	ClearErrorSess(testAmbientSession)
	if got := r.RndUpto(0); got != 0 {
		t.Fatalf("RndUpto(0) got %d want 0", got)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("RndUpto(0) must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRndUptoFilterRetries(t *testing.T) {
	// Reject first candidate (3); should re-genrand until accepted.
	// seed2: first raw%10=3 reject, second raw%10=5 accept.
	r := NewRngSess(testAmbientSession, 2)
	got := r.RndUptoFilter(10, RejectEQ(3))
	if got != 5 {
		t.Fatalf("RndUptoFilter reject 3: got %d want 5", got)
	}
	// One logical rnd_upto step: depth ends at local+1 (=1), not 1+tries.
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth after filtered upto: got %d want 1", r.RandDepth())
	}
}

func TestRndUptoFilterResidualSticky(t *testing.T) {
	// Filter residual ERROR soft invent was soft-retry forever (hang) then invent pick.
	// Fair: sticky fail closed return 0 without infinite soft-retry.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 1)
	residualReject := filterFunc(func(v uint32) bool {
		SetErrorSess(testAmbientSession, ErrGeneric)
		return true
	})
	got := r.RndUptoFilter(10, residualReject)
	if got != 0 {
		t.Fatalf("residual filter must fail closed 0, got %d", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("residual filter RndUptoFilter must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRndFlipcoin(t *testing.T) {
	// seed2 first genrand%100 = 1959434203%100 = 3 < 50 → true
	r := NewRngSess(testAmbientSession, 2)
	if !r.RndFlipcoin(50) {
		t.Fatal("RndFlipcoin(50) first seed2: want true")
	}
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth: got %d want 1", r.RandDepth())
	}
}

func TestRndFlipcoinFilterForce(t *testing.T) {
	// Filter rejects 0 → return true without genrand (depth still increments once).
	r := NewRngSess(testAmbientSession, 2)
	if !r.RndFlipcoinFilter(50, RejectEQ(0)) {
		t.Fatal("filter reject 0: want true without draw")
	}
	// State must be unchanged (no Genrand).
	r2 := NewRngSess(testAmbientSession, 2)
	want := r2.Genrand()
	// r never called Genrand; next Genrand should match first of fresh seed2
	if got := r.Genrand(); got != want {
		t.Fatalf("after force-true flipcoin, Genrand desynced: got %d want %d", got, want)
	}
}

func TestRandomHexDigits(t *testing.T) {
	// DefaultRndNumGenerator::RandomHexDigits: each digit genrand()%16, depth++.
	// AbsRndNumGenerator.cpp:50 — hex1 uppercase ABCDEF
	r := NewRngSess(testAmbientSession, 2)
	// 1959434203%16 = 11 → 'B'
	hex := r.RandomHexDigits(1)
	if hex != "B" {
		t.Fatalf("RandomHexDigits(1) seed2: got %q want B", hex)
	}
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth after one hex digit: got %d want 1", r.RandDepth())
	}
}

func TestRandomDigits(t *testing.T) {
	r := NewRngSess(testAmbientSession, 2)
	// 1959434203%10 = 3
	d := r.RandomDigits(1)
	if d != "3" {
		t.Fatalf("RandomDigits(1) seed2: got %q want 3", d)
	}
	ClearErrorSess(testAmbientSession)
	if (*Rng)(nil).RandomDigits(4) != "" {
		t.Fatal("nil RNG RandomDigits must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG RandomDigits must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRngNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Rng)(nil).Genrand() != 0 {
		t.Fatal("nil Genrand must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Genrand must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Rng)(nil).RndUpto(5) != 0 {
		t.Fatal("nil RndUpto must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RndUpto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Rng)(nil).RndFlipcoin(50) {
		t.Fatal("nil RndFlipcoin must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RndFlipcoin must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// --- AbsRndNumGenerator / DefaultRndNumGenerator component contracts ---
// CHECKLIST: AbsRndNumGenerator.cpp::*, DefaultRndNumGenerator.cpp::*

func TestAbsRndNumAlphabetTables(t *testing.T) {
	// AbsRndNumGenerator.cpp:50–52, get_hex1 / get_dec1
	if HexAlphabet != "0123456789ABCDEF" {
		t.Fatalf("get_hex1: got %q", HexAlphabet)
	}
	if DecAlphabet != "0123456789" {
		t.Fatalf("get_dec1: got %q", DecAlphabet)
	}
}

func TestAbsRndNumGeneratorCount(t *testing.T) {
	// AbsRndNumGenerator::count → MAX_RNDNUM_GENERATOR = rDFS + 1
	if RngKindCount != 2 {
		t.Fatalf("count: got %d want 2", RngKindCount)
	}
	if RngKindDefault != 0 || RngKindDFS != 1 {
		t.Fatalf("RNDNUM_GENERATOR enum order: default=%d dfs=%d", RngKindDefault, RngKindDFS)
	}
}

func TestDefaultRndKind(t *testing.T) {
	// DefaultRndNumGenerator::kind
	r := NewRngSess(testAmbientSession, 2)
	if r.Kind() != RngKindDefault {
		t.Fatalf("kind: got %d want Default", r.Kind())
	}
}

func TestDefaultGetPrefixedNameIdentity(t *testing.T) {
	// DefaultRndNumGenerator.cpp:105–107 — returns name unchanged
	if got := GetPrefixedNameDefault("g_42"); got != "g_42" {
		t.Fatalf("get_prefixed_name: got %q", got)
	}
	// random.cpp get_prefixed_name with prefix_name off
	if got := GetPrefixedName("g_42", false); got != "g_42" {
		t.Fatalf("prefix off: got %q", got)
	}
	// prefix on + default RNG still identity
	if got := GetPrefixedName("g_42", true); got != "g_42" {
		t.Fatalf("prefix on default: got %q", got)
	}
}

func TestDefaultTraceDepthAndSequenceEmpty(t *testing.T) {
	// DefaultRndNumGenerator::trace_depth starts empty; get_sequence empty when Sequence add_number is no-op
	r := NewRngSess(testAmbientSession, 2)
	if r.TraceDepth() != "" {
		t.Fatalf("trace_depth initial: %q", r.TraceDepth())
	}
	if r.GetSequence() != "" {
		t.Fatalf("get_sequence default: %q", r.GetSequence())
	}
}

func TestSetRandDepth(t *testing.T) {
	// DefaultRndNumGenerator::set_rand_depth
	r := NewRngSess(testAmbientSession, 2)
	r.SetRandDepth(42)
	if r.RandDepth() != 42 {
		t.Fatalf("set_rand_depth: got %d want 42", r.RandDepth())
	}
	_ = r.RndUpto(3)
	if r.RandDepth() != 43 {
		t.Fatalf("after rnd_upto from 42: got %d want 43", r.RandDepth())
	}
}

func TestRandomHexDigitsMulti(t *testing.T) {
	// DefaultRndNumGenerator::RandomHexDigits — per-digit genrand%16, depth++ each
	// seed2: raw0=1959434203%16=11→B, raw1=341627945%16=9→9
	r := NewRngSess(testAmbientSession, 2)
	got := r.RandomHexDigits(2)
	if got != "B9" {
		t.Fatalf("RandomHexDigits(2) seed2: got %q want B9", got)
	}
	if r.RandDepth() != 2 {
		t.Fatalf("rand_depth after 2 hex: got %d want 2", r.RandDepth())
	}
	// zero / negative → empty, no draw
	r2 := NewRngSess(testAmbientSession, 2)
	if r2.RandomHexDigits(0) != "" || r2.RandomHexDigits(-1) != "" {
		t.Fatal("RandomHexDigits(<=0) must be empty")
	}
	if r2.RandDepth() != 0 {
		t.Fatal("RandomHexDigits(<=0) must not burn depth")
	}
}

func TestRandomDigitsMulti(t *testing.T) {
	// seed2: 1959434203%10=3, 341627945%10=5 → "35"
	r := NewRngSess(testAmbientSession, 2)
	got := r.RandomDigits(2)
	if got != "35" {
		t.Fatalf("RandomDigits(2) seed2: got %q want 35", got)
	}
	if r.RandDepth() != 2 {
		t.Fatalf("rand_depth after 2 digits: got %d want 2", r.RandDepth())
	}
}

func TestRndFlipcoinFilterForceFalse(t *testing.T) {
	// DefaultRndNumGenerator::rnd_flipcoin: filter(1) → return false without genrand
	r := NewRngSess(testAmbientSession, 2)
	if r.RndFlipcoinFilter(50, RejectEQ(1)) {
		t.Fatal("filter reject 1: want false without draw")
	}
	r2 := NewRngSess(testAmbientSession, 2)
	want := r2.Genrand()
	if got := r.Genrand(); got != want {
		t.Fatalf("after force-false flipcoin, Genrand desynced: got %d want %d", got, want)
	}
}

func TestRndFlipcoinP0And100(t *testing.T) {
	// p=0 → always false; p=100 → always true (genrand still burned)
	r := NewRngSess(testAmbientSession, 2)
	if r.RndFlipcoin(0) {
		t.Fatal("RndFlipcoin(0) want false")
	}
	r = NewRngSess(testAmbientSession, 2)
	if !r.RndFlipcoin(100) {
		t.Fatal("RndFlipcoin(100) want true")
	}
	// clamp p>100 to 100 (C++ asserts p<=100; non-assert builds use clamp safety)
	r = NewRngSess(testAmbientSession, 2)
	if !r.RndFlipcoin(150) {
		t.Fatal("RndFlipcoin(150) clamped to 100 want true")
	}
}

func TestSeedrandIndependence(t *testing.T) {
	// AbsRndNumGenerator::seedrand — each NewRngSess(testAmbientSession, seed) reseeds independently (srand48)
	a := NewRngSess(testAmbientSession, 7)
	b := NewRngSess(testAmbientSession, 7)
	for i := 0; i < 5; i++ {
		if a.Genrand() != b.Genrand() {
			t.Fatalf("same seed diverged at i=%d", i)
		}
	}
	c := NewRngSess(testAmbientSession, 8)
	a = NewRngSess(testAmbientSession, 7)
	if a.Genrand() == c.Genrand() {
		// extremely unlikely equal; if equal still ok for this weak check — compare sequences
	}
	// stronger: full first value differs for seed 7 vs 8
	a = NewRngSess(testAmbientSession, 7)
	c = NewRngSess(testAmbientSession, 8)
	if a.Genrand() == c.Genrand() {
		t.Fatal("seed 7 and 8 should not share first genrand")
	}
}

func TestProcessRndWrappers(t *testing.T) {
	// random.cpp process wrappers vs direct Rng methods (random mode).
	prev := ProcessRngSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	SetProcessRngSess(testAmbientSession, r)
	defer SetProcessRngSess(testAmbientSession, prev)

	r2 := NewRngSess(testAmbientSession, 2)
	// pure_rnd_upto(n==0) short-circuits without draw
	if PureRndUpto(0, nil) != 0 {
		t.Fatal("pure_rnd_upto(0) → 0")
	}
	// ProcessRndUpto matches Rng.RndUpto on same seed stream
	got := ProcessRndUptoSess(testAmbientSession, 10, nil)
	want := r2.RndUpto(10)
	if got != want {
		t.Fatalf("ProcessRndUpto: got %d want %d", got, want)
	}
	gotF := ProcessRndFlipcoinSess(testAmbientSession, 50, nil)
	wantF := r2.RndFlipcoin(50)
	if gotF != wantF {
		t.Fatalf("ProcessRndFlipcoin: got %v want %v", gotF, wantF)
	}
	gotH := ProcessRandomHexDigitsSess(testAmbientSession, 4)
	wantH := r2.RandomHexDigits(4)
	if gotH != wantH {
		t.Fatalf("hex: got %q want %q", gotH, wantH)
	}
	gotD := PureRandomDigits(3)
	wantD := r2.RandomDigits(3)
	if gotD != wantD {
		t.Fatalf("digits: got %q want %q", gotD, wantD)
	}
	if ProcessTraceDepthSess(testAmbientSession) != "" || ProcessGetSequenceSess(testAmbientSession) != "" {
		t.Fatal("default mode trace/sequence empty")
	}
}

func TestProcessRndNilSticky(t *testing.T) {
	prev := ProcessRngSess(testAmbientSession)
	SetProcessRngSess(testAmbientSession, nil)
	defer SetProcessRngSess(testAmbientSession, prev)
	ClearErrorSess(testAmbientSession)
	if ProcessRndUptoSess(testAmbientSession, 5, nil) != 0 || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil process Rng RndUpto sticky 0")
	}
	ClearErrorSess(testAmbientSession)
	if ProcessRndFlipcoinSess(testAmbientSession, 50, nil) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil process Rng flipcoin sticky false")
	}
	ClearErrorSess(testAmbientSession)
	if ProcessRandomHexDigitsSess(testAmbientSession, 2) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil process hex sticky empty")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomNumberCreateInstance(t *testing.T) {
	// RandomNumber.cpp:63–74 CreateInstance default generator
	prevR := ProcessRngSess(testAmbientSession)
	RandomNumberDoFinalizationSess(testAmbientSession)
	defer func() {
		RandomNumberDoFinalizationSess(testAmbientSession)
		SetProcessRngSess(testAmbientSession, prevR)
	}()

	ClearErrorSess(testAmbientSession)
	CreateRandomNumberInstanceSess(testAmbientSession, RngKindDefault, 2)
	rn := GetRandomNumberSess(testAmbientSession)
	if rn == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("CreateInstance default must succeed")
	}
	if GetRndNumGeneratorSess(testAmbientSession) == nil {
		t.Fatal("GetRndNumGenerator")
	}
	r2 := NewRngSess(testAmbientSession, 2)
	got := rn.RndUpto(10, nil)
	want := r2.RndUpto(10)
	if got != want {
		t.Fatalf("rnd_upto via RandomNumber: got %d want %d", got, want)
	}
	// Switch default→default identity
	old := SwitchRndNumGeneratorSess(testAmbientSession, RngKindDefault)
	if old != RngKindDefault {
		t.Fatalf("switch same kind old=%v", old)
	}
	// DFS requires MaxExhaustiveDepth > 0 on process options
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 4
	SetProcessOptionsSess(testAmbientSession, o)
	old = SwitchRndNumGeneratorSess(testAmbientSession, RngKindDFS)
	if old != RngKindDefault {
		t.Fatalf("switch to DFS old=%v", old)
	}
	if HasErrorSess(testAmbientSession) || GetRndNumGeneratorSess(testAmbientSession) == nil || GetRndNumGeneratorSess(testAmbientSession).Kind() != RngKindDFS {
		t.Fatal("DFS switch", HasErrorSess(testAmbientSession), GetRndNumGeneratorSess(testAmbientSession))
	}
	// restore default for later tests via finalization path
	ClearErrorSess(testAmbientSession)
}

func TestRandomNumberDoFinalization(t *testing.T) {
	prevR := ProcessRngSess(testAmbientSession)
	defer SetProcessRngSess(testAmbientSession, prevR)
	CreateRandomNumberInstanceSess(testAmbientSession, RngKindDefault, 1)
	RandomNumberDoFinalizationSess(testAmbientSession)
	ClearErrorSess(testAmbientSession)
	if GetRandomNumberSess(testAmbientSession) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("after doFinalization GetInstance sticky nil")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRndNumGeneratorDefault(t *testing.T) {
	// AbsRndNumGenerator::make_rndnum_generator default
	ClearErrorSess(testAmbientSession)
	r := MakeRndNumGeneratorSess(testAmbientSession, RngKindDefault, 7)
	if r == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("default generator")
	}
	if r.Kind() != RngKindDefault {
		t.Fatal(r.Kind())
	}
	// DFS without positive max depth sticky
	ClearErrorSess(testAmbientSession)
	prevO := ProcessOptionsSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults()) // MaxExhaustiveDepth -1
	if MakeRndNumGeneratorSess(testAmbientSession, RngKindDFS, 1) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("DFS max_depth<=0 sticky")
	}
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 3
	SetProcessOptionsSess(testAmbientSession, o)
	// clear prior singleton if any
	clearDFSImpl()
	dr := MakeRndNumGeneratorSess(testAmbientSession, RngKindDFS, 1)
	if dr == nil || HasErrorSess(testAmbientSession) || dr.Kind() != RngKindDFS {
		t.Fatal("DFS generator", HasErrorSess(testAmbientSession))
	}
	// singleton
	if MakeRndNumGeneratorSess(testAmbientSession, RngKindDFS, 9) != dr {
		t.Fatal("DFS singleton")
	}
	clearDFSImpl()
	SetProcessOptionsSess(testAmbientSession, prevO)
	ClearErrorSess(testAmbientSession)
}
