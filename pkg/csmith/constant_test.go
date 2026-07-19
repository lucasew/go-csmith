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
	// Constant.cpp:312 — assert(st != eVoid) sticky; no invent "/* void */" / soft success
	ClearError()
	c := MakeRandom(GetSimpleType(EVoid), Defaults(), nil, NewRng(1))
	if c != nil {
		t.Fatalf("void constant must fail closed, got %+v", c)
	}
	if !HasError() {
		t.Fatal("void MakeRandom must SetError sticky")
	}
	ClearError()
	// Type* always live; sticky no invent Constant{Type:nil, Value:"0"} shell
	if MakeRandom(nil, Defaults(), nil, NewRng(1)) != nil {
		t.Fatal("nil type MakeRandom must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type MakeRandom must SetError sticky")
	}
	ClearError()
	// simple non-void needs RNG sticky (no invent NewRng)
	if MakeRandom(GetIntType(), Defaults(), nil, nil) != nil {
		t.Fatal("nil RNG simple MakeRandom must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG simple MakeRandom must SetError sticky")
	}
	ClearError()
	// Constant.cpp:411 unsupported kind sticky
	if MakeRandom(&Type{}, Defaults(), nil, NewRng(1)) != nil {
		t.Fatal("non-simple/non-ptr MakeRandom must fail closed")
	}
	if !HasError() {
		t.Fatal("unsupported type MakeRandom must SetError sticky")
	}
	ClearError()
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
	ClearError()
	r := NewRng(2)
	// first RndUpto(10) = 3 for seed2
	c := MakeRandomUpto(10, r)
	if c.Value != "3" || c.Type != GetSimpleType(EUInt) {
		t.Fatalf("%+v", c)
	}
	// Constant.cpp always has RNG; sticky no invent NewRng(0)
	if MakeRandomUpto(10, nil) != nil {
		t.Fatal("nil RNG MakeRandomUpto must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomUpto must SetError sticky")
	}
	ClearError()
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

func TestHexToBinary(t *testing.T) {
	// Constant.cpp:85–97
	ClearError()
	if HexToBinary("0") != "0000" || HexToBinary("f") != "1111" || HexToBinary("A") != "1010" {
		t.Fatalf("nibbles: 0=%q f=%q A=%q", HexToBinary("0"), HexToBinary("f"), HexToBinary("A"))
	}
	if HexToBinary("0f") != "00001111" {
		t.Fatalf("0f=%q", HexToBinary("0f"))
	}
	ClearError()
	if HexToBinary("g") != "" {
		t.Fatal("invalid hex must fail closed")
	}
	if !HasError() {
		t.Fatal("invalid hex must SetError sticky")
	}
	// empty string has no broken digit — complete empty is ok non-sticky
	ClearError()
	if HexToBinary("") != "" {
		t.Fatal("empty hex must be empty")
	}
	ClearError()
}

func TestBinaryConstantPath(t *testing.T) {
	// Constant.cpp:102–103 — binary_constant && flipcoin → 0b…; no invent hex when selected
	opts := Defaults()
	opts.BinaryConstant = true
	// force BinaryConstProb 100% via process probs
	probs := NewProbabilities(opts)
	probs.single[PBinaryConstProb] = 100
	prev := ProcessProbabilities()
	SetProcessProbabilities(probs)
	defer SetProcessProbabilities(prev)
	s := generateRandomIntConstant(opts, NewRng(1))
	if !strings.HasPrefix(s, "0b") {
		t.Fatalf("want binary int, got %q", s)
	}
	// only 0/1 after 0b
	for _, c := range s[2:] {
		if c != '0' && c != '1' {
			t.Fatalf("non-binary digit in %q", s)
		}
	}
	// long long binary includes LL
	sll := generateRandomLongLongConstant(opts, NewRng(1))
	if !strings.HasPrefix(sll, "0b") || !strings.HasSuffix(sll, "LL") {
		t.Fatalf("ll binary %q", sll)
	}
	// BinaryConstant off → never invent 0b
	opts.BinaryConstant = false
	for seed := uint64(1); seed < 20; seed++ {
		s = generateRandomIntConstant(opts, NewRng(seed))
		if strings.HasPrefix(s, "0b") {
			t.Fatalf("binary off invent %q", s)
		}
	}
	ClearError()
	// BinaryConstant on + nil RNG sticky (no invent soft skip without draw)
	opts.BinaryConstant = true
	if _, ok := maybeBinaryConstant(opts, nil, 2, ""); ok {
		t.Fatal("nil RNG maybeBinaryConstant must not claim binary branch")
	}
	if !HasError() {
		t.Fatal("nil RNG BinaryConstant maybeBinaryConstant must SetError sticky")
	}
	ClearError()
	// BinaryConstant off complete no-op
	opts.BinaryConstant = false
	if _, ok := maybeBinaryConstant(opts, nil, 2, ""); ok {
		t.Fatal("BinaryConstant off must complete no-op")
	}
	if HasError() {
		t.Fatal("BinaryConstant off must not sticky")
	}
	ClearError()
}

func TestMarkMutableConstWrapsSimple(t *testing.T) {
	// Constant.cpp:413–415 — simple + mark_mutable_const → "(" + v + ")"
	// no soft invent ignore of MarkMutableConst
	opts := Defaults()
	opts.MarkMutableConst = true
	// force deterministic simple path via MakeRandom int
	r := NewRng(2)
	c := MakeRandom(GetSimpleType(EInt), opts, nil, r)
	if c == nil || c.Value == "" {
		t.Fatal("nil const")
	}
	if !strings.HasPrefix(c.Value, "(") || !strings.HasSuffix(c.Value, ")") {
		t.Fatalf("want paren wrap, got %q", c.Value)
	}
	// pointer is not simple wrap path in C++ (type ePointer, not eSimple)
	c2 := MakeRandom(PointerTo(GetIntType()), opts, nil, NewRng(1))
	if c2 == nil || c2.Value != "0" {
		t.Fatalf("pointer must stay 0, got %+v", c2)
	}
	// off → no invent wrap
	opts.MarkMutableConst = false
	c3 := MakeRandom(GetSimpleType(EInt), opts, nil, NewRng(2))
	if c3 == nil || strings.HasPrefix(c3.Value, "(") {
		t.Fatalf("off must not wrap, got %q", c3.Value)
	}
	// bitfield InRange also wraps
	opts.MarkMutableConst = true
	s := GenerateRandomConstantInRange(GetIntType(), 8, opts, NewRng(3))
	if s == "" || !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		t.Fatalf("InRange wrap got %q", s)
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
	ClearError()
	if generateSmallRandomFloatHexConstant(0, nil) != "" {
		t.Fatal("nil rng fail closed")
	}
	if !HasError() {
		t.Fatal("nil rng generateSmallRandomFloatHexConstant must SetError sticky")
	}
	ClearError()
	// formatSmallConstant must not invent float without RNG; sticky broken dispatch
	ClearError()
	if formatSmallConstant(EFloat, 1, Defaults()) != "" {
		t.Fatal("formatSmallConstant float invent")
	}
	if !HasError() {
		t.Fatal("formatSmallConstant float must SetError sticky")
	}
	ClearError()
}

func TestRandomHexDigitsNilRNGSticky(t *testing.T) {
	// AbsRndNumGenerator always has live RNG sticky
	ClearError()
	if NewRng(1).RandomHexDigits(0) != "" {
		t.Fatal("num<=0 returns empty non-sticky")
	}
	if HasError() {
		t.Fatal("num<=0 must not SetError")
	}
	if (*Rng)(nil).RandomHexDigits(4) != "" {
		t.Fatal("nil RNG must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil RNG RandomHexDigits must SetError sticky")
	}
	ClearError()
}
