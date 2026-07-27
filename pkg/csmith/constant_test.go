package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomPointerIsZero(t *testing.T) {
	// Constant.cpp: pointer → "0", no RNG
	r := NewRngSess(testAmbientSession, 2)
	c := MakeRandomSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt)), Defaults(), nil, r)
	if c.Value != "0" {
		t.Fatalf("pointer const: %q", c.Value)
	}
	if r.RandDepthSess(testAmbientSession) != 0 {
		t.Fatalf("pointer must not consume RNG, depth=%d", r.RandDepthSess(testAmbientSession))
	}
}

func TestMakeRandomVoidFailClosed(t *testing.T) {
	// Constant.cpp:312 — assert(st != eVoid) sticky; no invent "/* void */" / soft success
	ClearErrorSess(testAmbientSession)
	c := MakeRandomSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EVoid), Defaults(), nil, NewRngSess(testAmbientSession, 1))
	if c != nil {
		t.Fatalf("void constant must fail closed, got %+v", c)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void MakeRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type* always live; sticky no invent Constant{Type:nil, Value:"0"} shell
	if MakeRandomSess(testAmbientSession, nil, Defaults(), nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil type MakeRandom must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type MakeRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// simple non-void needs RNG sticky (no invent NewRng)
	if MakeRandomSess(testAmbientSession, GetIntTypeSess(testAmbientSession), Defaults(), nil, nil) != nil {
		t.Fatal("nil RNG simple MakeRandom must fail closed")
	}
	// nil RNG simple MakeRandom must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// Constant.cpp:411 unsupported kind sticky
	if MakeRandomSess(testAmbientSession, &Type{}, Defaults(), nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("non-simple/non-ptr MakeRandom must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unsupported type MakeRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIntHexPathSeed2(t *testing.T) {
	// Defaults BinaryConstant=false, LongLong=true.
	// pure_rnd_flipcoin(50) false → hex path GenerateRandomIntConstant → 0x + 8 hex + L
	// Force hex path by probing: if first flip is true we get small path instead.
	opts := Defaults()
	// Find a seed where first flipcoin(50) is false for eInt hex path.
	for seed := uint64(0); seed < 200; seed++ {
		r := NewRngSess(testAmbientSession, seed)
		if r.RndFlipcoinSess(testAmbientSession, 50) {
			continue // small path
		}
		// hex path: RandomHexDigits(8) then + L
		hex := r.RandomHexDigitsSess(testAmbientSession, 8)
		want := "0x" + hex + "L"
		r2 := NewRngSess(testAmbientSession, seed)
		c := MakeRandomSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt), opts, nil, r2)
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
		r := NewRngSess(testAmbientSession, seed)
		if !r.RndFlipcoinSess(testAmbientSession, 50) {
			continue // need small path
		}
		// second flip + upto
		var num int
		if r.RndFlipcoinSess(testAmbientSession, 50) {
			num = int(r.RndUptoSess(testAmbientSession, 3)) - 1
		} else {
			num = int(r.RndUptoSess(testAmbientSession, 20)) - 10
		}
		want := formatSmallConstantSess(testAmbientSession, EInt, num, opts)
		r2 := NewRngSess(testAmbientSession, seed)
		c := MakeRandomSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt), opts, nil, r2)
		if c.Value != want {
			t.Fatalf("seed %d: got %q want %q", seed, c.Value, want)
		}
		return
	}
	t.Fatal("no seed with small path")
}

func TestMakeRandomUpto(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	// first RndUpto(10) = 3 for seed2
	c := MakeRandomUptoSess(testAmbientSession, 10, r)
	if c.Value != "3" || c.Type != GetSimpleTypeSess(testAmbientSession, EUInt) {
		t.Fatalf("%+v", c)
	}
	// Constant.cpp always has RNG; sticky no invent NewRngSess(testAmbientSession, 0)
	if MakeRandomUptoSess(testAmbientSession, 10, nil) != nil {
		t.Fatal("nil RNG MakeRandomUpto must fail closed")
	}
	// nil RNG MakeRandomUpto must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestMakeInt(t *testing.T) {
	c := MakeIntSess(testAmbientSession, 42)
	if c.Value != "42" || c.Type != GetSimpleTypeSess(testAmbientSession, EInt) {
		t.Fatalf("%+v", c)
	}
}

func TestGenerateRandomFloatHexConstantSignFlip(t *testing.T) {
	// Constant.cpp:192–196 — pure_rnd_flipcoin(50) chooses + or − exp; no invent always +
	sawPlus, sawMinus := false, false
	for seed := uint64(1); seed < 40; seed++ {
		s := generateRandomFloatHexConstantSess(testAmbientSession, NewRngSess(testAmbientSession, seed))
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
	if generateRandomFloatHexConstantSess(testAmbientSession, nil) != "" {
		t.Fatal("nil rng must fail closed")
	}
}

func TestHexToBinary(t *testing.T) {
	// Constant.cpp:85–97
	ClearErrorSess(testAmbientSession)
	if HexToBinarySess(testAmbientSession, "0") != "0000" || HexToBinarySess(testAmbientSession, "f") != "1111" || HexToBinarySess(testAmbientSession, "A") != "1010" {
		t.Fatalf("nibbles: 0=%q f=%q A=%q", HexToBinarySess(testAmbientSession, "0"), HexToBinarySess(testAmbientSession, "f"), HexToBinarySess(testAmbientSession, "A"))
	}
	if HexToBinarySess(testAmbientSession, "0f") != "00001111" {
		t.Fatalf("0f=%q", HexToBinarySess(testAmbientSession, "0f"))
	}
	ClearErrorSess(testAmbientSession)
	if HexToBinarySess(testAmbientSession, "g") != "" {
		t.Fatal("invalid hex must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid hex must SetError sticky")
	}
	// empty string has no broken digit — complete empty is ok non-sticky
	ClearErrorSess(testAmbientSession)
	if HexToBinarySess(testAmbientSession, "") != "" {
		t.Fatal("empty hex must be empty")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinaryConstantPath(t *testing.T) {
	// Constant.cpp:102–103 — binary_constant && flipcoin → 0b…; no invent hex when selected
	opts := Defaults()
	opts.BinaryConstant = true
	// force BinaryConstProb 100% via process probs
	probs := NewProbabilities(opts)
	probs.single[PBinaryConstProb] = 100
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, probs)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	s := generateRandomIntConstant(testAmbientSession, opts, NewRngSess(testAmbientSession, 1))
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
	sll := generateRandomLongLongConstant(testAmbientSession, opts, NewRngSess(testAmbientSession, 1))
	if !strings.HasPrefix(sll, "0b") || !strings.HasSuffix(sll, "LL") {
		t.Fatalf("ll binary %q", sll)
	}
	// BinaryConstant off → never invent 0b
	opts.BinaryConstant = false
	for seed := uint64(1); seed < 20; seed++ {
		s = generateRandomIntConstant(testAmbientSession, opts, NewRngSess(testAmbientSession, seed))
		if strings.HasPrefix(s, "0b") {
			t.Fatalf("binary off invent %q", s)
		}
	}
	ClearErrorSess(testAmbientSession)
	// BinaryConstant on + nil RNG sticky (no invent soft skip without draw)
	opts.BinaryConstant = true
	if _, ok := maybeBinaryConstantSess(testAmbientSession, opts, nil, 2, ""); ok {
		t.Fatal("nil RNG maybeBinaryConstant must not claim binary branch")
	}
	// nil RNG BinaryConstant maybeBinaryConstant must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// BinaryConstant off complete no-op
	opts.BinaryConstant = false
	if _, ok := maybeBinaryConstantSess(testAmbientSession, opts, nil, 2, ""); ok {
		t.Fatal("BinaryConstant off must complete no-op")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("BinaryConstant off must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMarkMutableConstWrapsSimple(t *testing.T) {
	// Constant.cpp:413–415 — simple + mark_mutable_const → "(" + v + ")"
	// no soft invent ignore of MarkMutableConst
	opts := Defaults()
	opts.MarkMutableConst = true
	// force deterministic simple path via MakeRandom int
	r := NewRngSess(testAmbientSession, 2)
	c := MakeRandomSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt), opts, nil, r)
	if c == nil || c.Value == "" {
		t.Fatal("nil const")
	}
	if !strings.HasPrefix(c.Value, "(") || !strings.HasSuffix(c.Value, ")") {
		t.Fatalf("want paren wrap, got %q", c.Value)
	}
	// pointer is not simple wrap path in C++ (type ePointer, not eSimple)
	c2 := MakeRandomSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), opts, nil, NewRngSess(testAmbientSession, 1))
	if c2 == nil || c2.Value != "0" {
		t.Fatalf("pointer must stay 0, got %+v", c2)
	}
	// off → no invent wrap
	opts.MarkMutableConst = false
	c3 := MakeRandomSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt), opts, nil, NewRngSess(testAmbientSession, 2))
	if c3 == nil || strings.HasPrefix(c3.Value, "(") {
		t.Fatalf("off must not wrap, got %q", c3.Value)
	}
	// bitfield InRange also wraps
	opts.MarkMutableConst = true
	s := GenerateRandomConstantInRangeSess(testAmbientSession, GetIntTypeSess(testAmbientSession), 8, opts, NewRngSess(testAmbientSession, 3))
	if s == "" || !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		t.Fatalf("InRange wrap got %q", s)
	}
}

func TestGenerateSmallRandomFloatHexConstant(t *testing.T) {
	// Constant.cpp:207–223 — RandomHexDigits(1) + flipcoin ±1; no invent from num%
	// positive num
	s := generateSmallRandomFloatHexConstantSess(testAmbientSession, 2, NewRngSess(testAmbientSession, 1))
	if !strings.HasPrefix(s, "0x2.") || !strings.Contains(s, "p") {
		t.Fatalf("got %q", s)
	}
	// negative num → -0x…
	s = generateSmallRandomFloatHexConstantSess(testAmbientSession, -3, NewRngSess(testAmbientSession, 2))
	if !strings.HasPrefix(s, "-0x3.") {
		t.Fatalf("neg got %q", s)
	}
	// both p+1 and p-1 across seeds
	sawP, sawM := false, false
	for seed := uint64(1); seed < 30; seed++ {
		s = generateSmallRandomFloatHexConstantSess(testAmbientSession, 1, NewRngSess(testAmbientSession, seed))
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
	ClearErrorSess(testAmbientSession)
	if generateSmallRandomFloatHexConstantSess(testAmbientSession, 0, nil) != "" {
		t.Fatal("nil rng fail closed")
	}
	// nil rng generateSmallRandomFloatHexConstant must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// formatSmallConstant must not invent float without RNG; sticky broken dispatch
	ClearErrorSess(testAmbientSession)
	if formatSmallConstantSess(testAmbientSession, EFloat, 1, Defaults()) != "" {
		t.Fatal("formatSmallConstant float invent")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("formatSmallConstant float must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestFormatSmallConstantUInt128DefaultSigned mirrors Constant.cpp:329–361 —
// eUInt128 is not in the unsigned cast switch; default `oss << num` then "U"/"UL".
func TestFormatSmallConstantUInt128DefaultSigned(t *testing.T) {
	opts := Defaults()
	// longlong on → UL suffix
	got := formatSmallConstantSess(testAmbientSession, EUInt128, -4, opts)
	if got != "-4UL" {
		t.Fatalf("longlong: got %q want -4UL", got)
	}
	// ccomp or !longlong → U suffix (Constant.cpp:357–359)
	opts.CComp = true
	got = formatSmallConstantSess(testAmbientSession, EUInt128, -4, opts)
	if got != "-4U" {
		t.Fatalf("ccomp: got %q want -4U", got)
	}
	opts = Defaults()
	opts.LongLong = false
	got = formatSmallConstantSess(testAmbientSession, EUInt128, -4, opts)
	if got != "-4U" {
		t.Fatalf("!longlong: got %q want -4U", got)
	}
	// Output wraps leading '-' (Constant.cpp:535–539)
	c := &Constant{Type: GetSimpleTypeSess(testAmbientSession, EUInt128), Value: "-4U"}
	out := c.OutputOptsSess(testAmbientSession, Defaults())
	if out != "(-4U)" {
		t.Fatalf("Output: got %q want (-4U)", out)
	}
}

func TestRandomHexDigitsNilRNGSticky(t *testing.T) {
	// AbsRndNumGenerator always has live RNG sticky
	ClearErrorSess(testAmbientSession)
	if NewRngSess(testAmbientSession, 1).RandomHexDigitsSess(testAmbientSession, 0) != "" {
		t.Fatal("num<=0 returns empty non-sticky")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("num<=0 must not SetError")
	}
	if (*Rng)(nil).RandomHexDigitsSess(testAmbientSession, 4) != "" {
		t.Fatal("nil RNG must fail closed empty")
	}
	// nil RNG RandomHexDigits must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomConstantVoidResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent Constant void shell past eVoid.
	ClearErrorSess(testAmbientSession)
	vt := GetSimpleTypeSess(testAmbientSession, EVoid)
	if MakeRandomSess(testAmbientSession, vt, Defaults(), nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("void MakeRandom must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void MakeRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Type sticky
	if MakeRandomSess(testAmbientSession, nil, Defaults(), nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil Type MakeRandom must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type MakeRandom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomNonzero(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	opts := Defaults()
	probs := NewProbabilities(opts)
	c := MakeRandomNonzeroSess(testAmbientSession, GetIntTypeSess(testAmbientSession), opts, probs, r)
	if c == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("nonzero", HasErrorSess(testAmbientSession))
	}
	if c.EqualsSess(testAmbientSession, 0) {
		t.Fatal("must nonzero", c.Value)
	}
	if MakeRandomNonzeroSess(testAmbientSession, nil, opts, probs, r) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestConstantCloneOutputCompatible(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	c := MakeIntSess(testAmbientSession, -3)
	cl := c.CloneSess(testAmbientSession)
	if cl == nil || cl.Value != "-3" || cl == c {
		t.Fatal(cl)
	}
	if c.OutputSess(testAmbientSession) != "(-3)" {
		t.Fatal(c.OutputSess(testAmbientSession))
	}
	z := &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}
	if z.OutputSess(testAmbientSession) != "(void*)0" {
		t.Fatal(z.OutputSess(testAmbientSession))
	}
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	if !c.CompatibleWithVarSess(testAmbientSession, v, true) || c.CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("expand_struct gate")
	}
	if c.CompatibleWithExprSess(testAmbientSession, &Expression{Term: TermConstant, Con: c}) {
		t.Fatal("expr always false")
	}
	if c.GetComplexitySess(testAmbientSession) != 1 || c.GetTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
		t.Fatal("accessors")
	}
	if c.GetReferencedPtrsSess(testAmbientSession) != nil {
		t.Fatal("no ptrs")
	}
	// MakeInt with mark_mutable_const
	o := Defaults()
	o.MarkMutableConst = true
	if MakeIntOptsSess(testAmbientSession, 5, o).Value != "(5)" {
		t.Fatal(MakeIntOptsSess(testAmbientSession, 5, o).Value)
	}
	ClearErrorSess(testAmbientSession)
}

func TestBlockDepthProtectAndFind(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	b := &Block{StmID: 7, blockSize: 4}
	if b.BlockSizeSess(testAmbientSession) != 4 {
		t.Fatal(b.BlockSizeSess(testAmbientSession))
	}
	if b.GetDepthProtectSess(testAmbientSession) {
		t.Fatal("default false")
	}
	if !b.SetDepthProtectSess(testAmbientSession, true) || !b.GetDepthProtectSess(testAmbientSession) {
		t.Fatal("set")
	}
	b.PushStmtSess(testAmbientSession, Stmt{Kind: StmtReturn, StmID: 1, Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}})
	if len(b.Stmts) != 1 {
		t.Fatal("push")
	}
	f := &Function{Name: "f", Blocks: []*Block{b}}
	if FindBlockByIDSess(testAmbientSession, []*Function{f}, 7) != b {
		t.Fatal("find")
	}
	if FindBlockByIDSess(testAmbientSession, []*Function{f}, 99) != nil {
		t.Fatal("miss")
	}
	if FindBlockByIDSess(testAmbientSession, []*Function{f}, 0) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("id 0 sticky")
	}
	ClearErrorSess(testAmbientSession)
}
