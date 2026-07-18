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
	r := NewRng(2)
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
		r := NewRng(seed)
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
	r := NewRng(2)
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
}

func TestRndUptoFilterRetries(t *testing.T) {
	// Reject first candidate (3); should re-genrand until accepted.
	// seed2: first raw%10=3 reject, second raw%10=5 accept.
	r := NewRng(2)
	got := r.RndUptoFilter(10, RejectEQ(3))
	if got != 5 {
		t.Fatalf("RndUptoFilter reject 3: got %d want 5", got)
	}
	// One logical rnd_upto step: depth ends at local+1 (=1), not 1+tries.
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth after filtered upto: got %d want 1", r.RandDepth())
	}
}

func TestRndFlipcoin(t *testing.T) {
	// seed2 first genrand%100 = 1959434203%100 = 3 < 50 → true
	r := NewRng(2)
	if !r.RndFlipcoin(50) {
		t.Fatal("RndFlipcoin(50) first seed2: want true")
	}
	if r.RandDepth() != 1 {
		t.Fatalf("rand_depth: got %d want 1", r.RandDepth())
	}
}

func TestRndFlipcoinFilterForce(t *testing.T) {
	// Filter rejects 0 → return true without genrand (depth still increments once).
	r := NewRng(2)
	if !r.RndFlipcoinFilter(50, RejectEQ(0)) {
		t.Fatal("filter reject 0: want true without draw")
	}
	// State must be unchanged (no Genrand).
	r2 := NewRng(2)
	want := r2.Genrand()
	// r never called Genrand; next Genrand should match first of fresh seed2
	if got := r.Genrand(); got != want {
		t.Fatalf("after force-true flipcoin, Genrand desynced: got %d want %d", got, want)
	}
}

func TestRandomHexDigits(t *testing.T) {
	// DefaultRndNumGenerator::RandomHexDigits: each digit genrand()%16, depth++.
	// AbsRndNumGenerator.cpp:50 — hex1 uppercase ABCDEF
	r := NewRng(2)
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
	r := NewRng(2)
	// 1959434203%10 = 3
	d := r.RandomDigits(1)
	if d != "3" {
		t.Fatalf("RandomDigits(1) seed2: got %q want 3", d)
	}
}
