package csmith

import (
	"math"
	"strings"
	"testing"
)

func TestGenerateRandomConstantInRangePowFloat(t *testing.T) {
	// Constant.cpp:228 — (int)pow(2, bound/2.0); bound=15 → ~181 not 1<<(15/2)=128
	ClearError()
	opts := Defaults()
	wantB := int(math.Pow(2, 15.0/2.0))
	if wantB < 180 || wantB > 182 {
		t.Fatalf("pow domain %d want ~181", wantB)
	}
	// Same seed: range const must burn U(wantB)+F like manual draws
	rManual := NewRng(1)
	_ = rManual.RndUpto(uint32(wantB))
	_ = rManual.RndFlipcoin(50)
	rGen := NewRng(1)
	s := GenerateRandomConstantInRange(GetIntType(), 15, opts, rGen)
	if s == "" || HasError() {
		t.Fatal("range const", s, GetError())
	}
	if rGen.RandDepth() != rManual.RandDepth() {
		t.Fatalf("depth want %d (U%d+F) got %d — integer shift would use U128", rManual.RandDepth(), wantB, rGen.RandDepth())
	}
	// Contrast: U128 stream diverges from U181 after first draw with same raw
	r128 := NewRng(1)
	_ = r128.RndUpto(128)
	r181 := NewRng(1)
	_ = r181.RndUpto(uint32(wantB))
	if r128.Genrand() == 0 && r181.Genrand() == 0 {
		// not a meaningful assert; just ensure wantB != 128
	}
	if wantB == 128 {
		t.Fatal("bound=15 must not collapse to integer half-shift domain 128")
	}
	ClearError()
}

func TestGenerateRandomConstantInRangeBounded(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	for i := 0; i < 50; i++ {
		s := GenerateRandomConstantInRange(GetIntType(), 3, opts, r)
		if s == "" {
			t.Fatal("empty")
		}
	}
}

func TestGenerateRandomConstantInRangeNilDepsSticky(t *testing.T) {
	// Constant.cpp assert path sticky — no invent empty/default past broken range IR
	ClearError()
	opts := Defaults()
	if GenerateRandomConstantInRange(GetIntType(), 8, opts, nil) != "" {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG GenerateRandomConstantInRange must SetError sticky")
	}
	ClearError()
	if GenerateRandomConstantInRange(nil, 8, opts, NewRng(1)) != "" {
		t.Fatal("nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type GenerateRandomConstantInRange must SetError sticky")
	}
	ClearError()
	if GenerateRandomConstantInRange(GetIntType(), 0, opts, NewRng(1)) != "" {
		t.Fatal("bound 0 must fail closed")
	}
	if !HasError() {
		t.Fatal("bound 0 GenerateRandomConstantInRange must SetError sticky")
	}
	ClearError()
	if GenerateRandomConstantInRange(GetSimpleType(EChar), 8, opts, NewRng(1)) != "" {
		t.Fatal("non int/uint simple must fail closed")
	}
	if !HasError() {
		t.Fatal("char GenerateRandomConstantInRange must SetError sticky")
	}
	ClearError()
}

func TestMakeStructConstantSkipsZeroWidthBitfield(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	st := &Type{
		isStruct:   true,
		StructName: "Sbf",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: 3, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "pad", Type: GetIntType(), BitWidth: 0, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f1", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	c := MakeStructConstant(NewRng(4), opts, probs, st)
	// should have two values, not three (pad skipped)
	// "{a, b}"
	inner := strings.TrimPrefix(strings.TrimSuffix(c.Value, "}"), "{")
	parts := strings.Split(inner, ", ")
	if len(parts) != 2 {
		t.Fatalf("want 2 fields, got %q parts=%v", c.Value, parts)
	}
}

func TestSelectGlobalFlexibleMatchesConvert(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// create a short global; SelectGlobal for int with Flexible may match if MatchFlexible allows
	q := NewCVQualifiers([]bool{false}, []bool{false})
	sh := GetSimpleType(EShort)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), sh, &q, NewRng(1))
	if g == nil {
		t.Fatal("no global")
	}
	// exact would miss int, flexible may convert short→int depending on Match
	v := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(2))
	// should at least not panic; may create new int global
	if v == nil {
		t.Fatal("nil")
	}
}
