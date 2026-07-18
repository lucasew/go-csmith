package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomPointerIsZero(t *testing.T) {
	// Constant.cpp: pointer → "0", no RNG
	r := NewRng(2)
	c := MakeRandom(PointerTo(GetSimpleType(EInt)), Defaults(), nil, r)
	if c.Value != "0" {
		t.Fatalf("pointer const: %q", c.Value)
	}
	if r.RandDepth() != 0 {
		t.Fatalf("pointer must not consume RNG, depth=%d", r.RandDepth())
	}
}

func TestMakeRandomVoidFailClosed(t *testing.T) {
	// Constant.cpp:312 — assert(st != eVoid); dead switch "/* void */" not a soft invent path
	c := MakeRandom(GetSimpleType(EVoid), Defaults(), nil, NewRng(1))
	if c != nil {
		t.Fatalf("void constant must fail closed, got %+v", c)
	}
}

func TestMakeRandomIntHexPathSeed2(t *testing.T) {
	// Defaults BinaryConstant=false, LongLong=true.
	// pure_rnd_flipcoin(50) false → hex path GenerateRandomIntConstant → 0x + 8 hex + L
	// Force hex path by probing: if first flip is true we get small path instead.
	opts := Defaults()
	// Find a seed where first flipcoin(50) is false for eInt hex path.
	for seed := uint64(0); seed < 200; seed++ {
		r := NewRng(seed)
		if r.RndFlipcoin(50) {
			continue // small path
		}
		// hex path: RandomHexDigits(8) then + L
		hex := r.RandomHexDigits(8)
		want := "0x" + hex + "L"
		r2 := NewRng(seed)
		c := MakeRandom(GetSimpleType(EInt), opts, nil, r2)
		if c.Value != want {
			t.Fatalf("seed %d: got %q want %q", seed, c.Value, want)
		}
		return
	}
	t.Fatal("no seed with hex path in 0..199")
}

func TestMakeRandomIntSmallPathSeed2(t *testing.T) {
	opts := Defaults()
	for seed := uint64(0); seed < 200; seed++ {
		r := NewRng(seed)
		if !r.RndFlipcoin(50) {
			continue // need small path
		}
		// second flip + upto
		var num int
		if r.RndFlipcoin(50) {
			num = int(r.RndUpto(3)) - 1
		} else {
			num = int(r.RndUpto(20)) - 10
		}
		want := formatSmallConstant(EInt, num, opts)
		r2 := NewRng(seed)
		c := MakeRandom(GetSimpleType(EInt), opts, nil, r2)
		if c.Value != want {
			t.Fatalf("seed %d: got %q want %q", seed, c.Value, want)
		}
		return
	}
	t.Fatal("no seed with small path")
}

func TestMakeRandomUpto(t *testing.T) {
	r := NewRng(2)
	// first RndUpto(10) = 3 for seed2
	c := MakeRandomUpto(10, r)
	if c.Value != "3" || c.Type != GetSimpleType(EUInt) {
		t.Fatalf("%+v", c)
	}
}

func TestMakeInt(t *testing.T) {
	c := MakeInt(42)
	if c.Value != "42" || c.Type != GetSimpleType(EInt) {
		t.Fatalf("%+v", c)
	}
}

func TestGenerateRandomFloatHexConstantSignFlip(t *testing.T) {
	// Constant.cpp:192–196 — pure_rnd_flipcoin(50) chooses + or − exp; no invent always +
	sawPlus, sawMinus := false, false
	for seed := uint64(1); seed < 40; seed++ {
		s := generateRandomFloatHexConstant(NewRng(seed))
		if s == "" {
			t.Fatal("nil rng invent empty")
		}
		if strings.Contains(s, "p+") {
			sawPlus = true
		}
		if strings.Contains(s, "p-") {
			sawMinus = true
		}
	}
	if !sawPlus || !sawMinus {
		t.Fatalf("need both exp signs, plus=%v minus=%v", sawPlus, sawMinus)
	}
	if generateRandomFloatHexConstant(nil) != "" {
		t.Fatal("nil rng must fail closed")
	}
}

func TestGenerateSmallRandomFloatHexConstant(t *testing.T) {
	// Constant.cpp:207–223 — RandomHexDigits(1) + flipcoin ±1; no invent from num%
	// positive num
	s := generateSmallRandomFloatHexConstant(2, NewRng(1))
	if !strings.HasPrefix(s, "0x2.") || !strings.Contains(s, "p") {
		t.Fatalf("got %q", s)
	}
	// negative num → -0x…
	s = generateSmallRandomFloatHexConstant(-3, NewRng(2))
	if !strings.HasPrefix(s, "-0x3.") {
		t.Fatalf("neg got %q", s)
	}
	// both p+1 and p-1 across seeds
	sawP, sawM := false, false
	for seed := uint64(1); seed < 30; seed++ {
		s = generateSmallRandomFloatHexConstant(1, NewRng(seed))
		if strings.HasSuffix(s, "p+1") {
			sawP = true
		}
		if strings.HasSuffix(s, "p-1") {
			sawM = true
		}
	}
	if !sawP || !sawM {
		t.Fatalf("need both p±1, +1=%v -1=%v", sawP, sawM)
	}
	if generateSmallRandomFloatHexConstant(0, nil) != "" {
		t.Fatal("nil rng fail closed")
	}
	// formatSmallConstant must not invent float without RNG
	if formatSmallConstant(EFloat, 1, Defaults()) != "" {
		t.Fatal("formatSmallConstant float invent")
	}
}
