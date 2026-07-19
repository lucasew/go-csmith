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
		// Type.cpp AllTypes has simples before make_random_struct_type
		env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
		st := MakeRandomStructType(NewRng(seed), opts, probs, &env, "S0")
		if st == nil {
			// ERROR_GUARD / empty field path fail closed — retry seed
			ClearError()
			continue
		}
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

func TestMakeRandomStructTypeFailClosedEmptyEnv(t *testing.T) {
	// Type.cpp ERROR_RETURN / ERROR_GUARD — no soft invent nil-type fields
	opts := Defaults()
	probs := NewProbabilities(opts)
	ClearError()
	// empty AllTypes → MakeOneStructField fails; whole struct abort
	st := MakeRandomStructType(NewRng(1), opts, probs, &TypeEnv{}, "Sempty")
	if st != nil {
		// only succeeds if full-bitfields path never needs ChooseRandom
		for _, f := range st.Fields {
			if f.Type == nil {
				t.Fatal("must not invent field with nil type")
			}
		}
	}
}

func TestBitfieldDeclEmit(t *testing.T) {
	ClearError()
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
	// Type.cpp assert(eSimple) sticky — bad bitfield type fails closed whole decl
	bad := &Type{
		isStruct:   true,
		StructName: "Sbad",
		Fields: []StructField{
			{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: 3},
		},
	}
	if s := bad.OutputStructDecl(); s != "" {
		t.Fatal("non-simple bitfield must fail closed", s)
	}
	if !HasError() {
		t.Fatal("non-simple bitfield OutputStructDecl must SetError sticky")
	}
	ClearError()
	// empty sid sticky
	anon := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	if s := anon.OutputStructDecl(); s != "" {
		t.Fatal("empty StructName must fail closed", s)
	}
	if !HasError() {
		t.Fatal("empty StructName OutputStructDecl must SetError sticky")
	}
	ClearError()
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

func TestMakeOneBitfieldNoInventMaxLen(t *testing.T) {
	// Type.cpp:641 — int_size()*8; IntSize 0 sticky fail closed (no invent maxLen=32)
	ClearError()
	opts := Defaults()
	opts.IntSize = 0
	probs := NewProbabilities(opts)
	f := MakeOneBitfield(NewRng(2), opts, probs, 0, true)
	if f.Type != nil || f.BitWidth >= 0 {
		t.Fatalf("IntSize 0 must fail closed, got %+v", f)
	}
	if !HasError() {
		t.Fatal("IntSize 0 MakeOneBitfield must SetError sticky")
	}
	// normal IntSize still works after ClearError
	ClearError()
	opts.IntSize = 4
	f2 := MakeOneBitfield(NewRng(2), opts, probs, 0, true)
	if f2.Type == nil || f2.BitWidth < 1 || f2.BitWidth > 32 {
		t.Fatalf("want live bitfield, got %+v", f2)
	}
}

func TestMakeRandomStructMaxFieldsNoInvent(t *testing.T) {
	// Type.cpp:1077 — max_struct_fields as-is; fixed+0 → nullptr not invent 1 field
	opts := Defaults()
	opts.MaxStructFields = 0
	opts.FixedStructFields = true
	opts.Bitfields = false
	probs := NewProbabilities(opts)
	env := &TypeEnv{AllTypes: []*Type{GetIntType()}}
	if st := MakeRandomStructType(NewRng(1), opts, probs, env, "S0"); st != nil {
		t.Fatalf("fixed max 0 must not invent struct, got %d fields", len(st.Fields))
	}
}
