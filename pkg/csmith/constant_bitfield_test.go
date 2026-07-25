package csmith

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateRandomConstantInRangePowFloat(t *testing.T) {
	// Constant.cpp:228 — (int)pow(2, bound/2.0); bound=15 → ~181 not 1<<(15/2)=128
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	wantB := int(math.Pow(2, 15.0/2.0))
	if wantB < 180 || wantB > 182 {
		t.Fatalf("pow domain %d want ~181", wantB)
	}
	// Same seed: range const must burn U(wantB)+F like manual draws
	rManual := NewRngSess(testAmbientSession, 1)
	_ = rManual.RndUpto(uint32(wantB))
	_ = rManual.RndFlipcoin(50)
	rGen := NewRngSess(testAmbientSession, 1)
	s := GenerateRandomConstantInRange(GetIntTypeSess(testAmbientSession), 15, opts, rGen)
	if s == "" || HasErrorSess(testAmbientSession) {
		t.Fatal("range const", s, GetErrorSess(testAmbientSession))
	}
	if rGen.RandDepth() != rManual.RandDepth() {
		t.Fatalf("depth want %d (U%d+F) got %d — integer shift would use U128", rManual.RandDepth(), wantB, rGen.RandDepth())
	}
	// Contrast: U128 stream diverges from U181 after first draw with same raw
	r128 := NewRngSess(testAmbientSession, 1)
	_ = r128.RndUpto(128)
	r181 := NewRngSess(testAmbientSession, 1)
	_ = r181.RndUpto(uint32(wantB))
	if r128.Genrand() == 0 && r181.Genrand() == 0 {
		// not a meaningful assert; just ensure wantB != 128
	}
	if wantB == 128 {
		t.Fatal("bound=15 must not collapse to integer half-shift domain 128")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateRandomConstantInRangeBounded(t *testing.T) {
	opts := Defaults()
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 50; i++ {
		s := GenerateRandomConstantInRange(GetIntTypeSess(testAmbientSession), 3, opts, r)
		if s == "" {
			t.Fatal("empty")
		}
	}
}

func TestGenerateRandomConstantInRangeSignPolarity(t *testing.T) {
	// Constant.cpp:230–236 — flip true → positive digit string; false → "-" + digits.
	// Seed 2 first eInt range: U then F; first flip seed2 after U domain is fixed.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	// bound=8 → b=pow(2,4)=16
	r := NewRngSess(testAmbientSession, 2)
	num := int(r.RndUpto(16))
	pos := r.RndFlipcoin(50)
	want := strconv.Itoa(num)
	if !pos {
		want = "-" + want
	}
	r2 := NewRngSess(testAmbientSession, 2)
	got := GenerateRandomConstantInRange(GetIntTypeSess(testAmbientSession), 8, opts, r2)
	if got != want {
		t.Fatalf("sign polarity: got %q want %q (num=%d flipTrue=%v)", got, want, num, pos)
	}
	// no invent L/UL suffix (Constant.cpp oss is bare digits)
	if strings.ContainsAny(got, "LUlu") {
		t.Fatalf("range const must be bare digits, got %q", got)
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateRandomConstantInRangeNilDepsSticky(t *testing.T) {
	// Constant.cpp assert path sticky — no invent empty/default past broken range IR
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if GenerateRandomConstantInRange(GetIntTypeSess(testAmbientSession), 8, opts, nil) != "" {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG GenerateRandomConstantInRange must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GenerateRandomConstantInRange(nil, 8, opts, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type GenerateRandomConstantInRange must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GenerateRandomConstantInRange(GetIntTypeSess(testAmbientSession), 0, opts, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("bound 0 must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("bound 0 GenerateRandomConstantInRange must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if GenerateRandomConstantInRange(GetSimpleTypeSess(testAmbientSession, EChar), 8, opts, NewRngSess(testAmbientSession, 1)) != "" {
		t.Fatal("non int/uint simple must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("char GenerateRandomConstantInRange must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeStructConstantSkipsZeroWidthBitfield(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	st := &Type{
		isStruct:   true,
		StructName: "Sbf",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: 3, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "pad", Type: GetIntTypeSess(testAmbientSession), BitWidth: 0, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	c := MakeStructConstant(NewRngSess(testAmbientSession, 4), opts, probs, st)
	// should have two values, not three (pad skipped)
	// Constant.cpp:266–275 — "{a,b}" with bare "," separators
	inner := strings.TrimPrefix(strings.TrimSuffix(c.Value, "}"), "{")
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		t.Fatalf("want 2 fields, got %q parts=%v", c.Value, parts)
	}
}

func TestSelectGlobalFlexibleMatchesConvert(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// create a short global; SelectGlobal for int with Flexible may match if MatchFlexible allows
	q := NewCVQualifiers([]bool{false}, []bool{false})
	sh := GetSimpleTypeSess(testAmbientSession, EShort)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), sh, &q, NewRngSess(testAmbientSession, 1))
	if g == nil {
		t.Fatal("no global")
	}
	// exact would miss int, flexible may convert short→int depending on Match
	v := vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2))
	// should at least not panic; may create new int global
	if v == nil {
		t.Fatal("nil")
	}
}
