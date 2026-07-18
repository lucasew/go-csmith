package csmith

import (
	"strings"
	"testing"
)

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
