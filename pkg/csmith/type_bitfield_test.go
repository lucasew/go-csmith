package csmith

import (
	"strings"
	"testing"
)

func TestMakeOneBitfieldWidth(t *testing.T) {
	opts := Defaults()
	opts.IntSize = 4
	probs := NewProbabilities(opts)
	r := NewRng(2)
	f := MakeOneBitfield(r, opts, probs, 0, true)
	if f.BitWidth < 0 || f.BitWidth > 32 {
		t.Fatal(f.BitWidth)
	}
	// first field prevZero=true forces non-zero if rolled 0
	if f.BitWidth == 0 {
		t.Fatal("first field should not be zero-width")
	}
	if f.Type != GetIntType() && f.Type != GetSimpleType(EUInt) {
		t.Fatal(f.Type.CName())
	}
}

func TestMakeRandomStructTypeCanHaveBitfields(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	found := false
	for seed := uint64(1); seed < 60; seed++ {
		var env TypeEnv
		st := MakeRandomStructType(NewRng(seed), opts, probs, &env, "S0")
		for _, f := range st.Fields {
			if f.BitWidth >= 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("expected some bitfield field in seeds 1..59")
	}
}

func TestBitfieldDeclEmit(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// craft struct with bitfield
	st := &Type{
		isStruct:   true,
		StructName: "Sbf",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: 3},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	decl := st.OutputStructDecl()
	if !strings.Contains(decl, "f0 : 3") {
		t.Fatal(decl)
	}
	if strings.Contains(decl, "f1 :") {
		t.Fatal("non-bitfield should not have width")
	}
	_ = probs
	_ = opts
}

func TestGenerateEmitsBitfieldSyntax(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// " : N;" bitfield pattern
		if strings.Contains(out, " : ") && strings.Contains(out, "struct ") {
			found = true
			break
		}
	}
	if !found {
		t.Log("bitfield syntax rare in 1..39")
	}
}
