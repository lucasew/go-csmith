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
