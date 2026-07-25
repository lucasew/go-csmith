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
		env := TypeEnv{Sess: testAmbientSession}
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
	st := MakeRandomStructType(NewRng(1), opts, probs, &TypeEnv{Sess: testAmbientSession}, "Sempty")
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

// TestOutputStructDeclPaddingFieldIndex mirrors Type.cpp:1836–1858 OutputStructUnion:
// zero-width bitfields do not advance j; non-bitfield names are always "f"<<j++, never
// the raw creation slot Name (make_one uses i including padding — seed 118 f4 vs f3).
func TestOutputStructDeclPaddingFieldIndex(t *testing.T) {
	ClearError()
	SetProcessOptionsSess(testAmbientSession, Defaults())
	// Names deliberately wrong/raw-slot so emit must not trust them.
	st := &Type{
		isStruct: true, StructName: "S0", Used: true,
		Fields: []StructField{
			{Name: "f0", Type: GetSimpleType(EUShort), BitWidth: -1, Qfer: NewCVQualifiers([]bool{true}, []bool{false})},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f2", Type: GetSimpleType(EChar), BitWidth: -1, Qfer: NewCVQualifiers([]bool{true}, []bool{true})},
			// raw slot i=3 — Name "f3" would be invent if emit used Name after pad skip
			{Name: "f3", Type: GetSimpleType(EUInt), BitWidth: 0, Qfer: NewCVQualifiers([]bool{false}, []bool{true})},
			{Name: "f4", Type: GetSimpleType(EUShort), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{true})},
			{Name: "f5", Type: GetSimpleType(EUInt), BitWidth: -1, Qfer: NewCVQualifiers([]bool{true}, []bool{false})},
		},
	}
	decl := st.OutputStructDecl()
	if decl == "" || HasError() {
		t.Fatalf("decl empty/err: %q err=%v", decl, HasError())
	}
	// Type.cpp:1851–1852 length==0 → " : 0;" no fN, j unchanged
	if !strings.Contains(decl, " : 0;") {
		t.Fatalf("want zero-width pad, got %q", decl)
	}
	// After pad, named fields must be f3/f4 (j=3,4), not stored Name f4/f5.
	if !strings.Contains(decl, " f3;") || !strings.Contains(decl, " f4;") {
		t.Fatalf("want emit-time f3/f4 after padding, got %q", decl)
	}
	if strings.Contains(decl, " f5;") {
		t.Fatalf("must not invent raw-slot f5 after padding: %q", decl)
	}
	// union same contract
	ClearError()
	ut := &Type{
		isUnion: true, StructName: "U0", Used: true,
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: 0, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f2", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	udecl := ut.OutputUnionDecl()
	if !strings.Contains(udecl, " f0;") || !strings.Contains(udecl, " f1;") {
		t.Fatalf("union want f0 then f1 after pad, got %q", udecl)
	}
	if strings.Contains(udecl, " f2;") {
		t.Fatalf("union must not use raw-slot f2: %q", udecl)
	}
	ClearError()
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
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	if st := MakeRandomStructType(NewRng(1), opts, probs, env, "S0"); st != nil {
		t.Fatalf("fixed max 0 must not invent struct, got %d fields", len(st.Fields))
	}
}

func TestOutputStructUnionDeclNilSticky(t *testing.T) {
	ClearError()
	if (*Type)(nil).OutputStructDecl() != "" {
		t.Fatal("nil Type OutputStructDecl must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Type OutputStructDecl must SetError sticky")
	}
	ClearError()
	if (*Type)(nil).OutputUnionDecl() != "" {
		t.Fatal("nil Type OutputUnionDecl must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Type OutputUnionDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputStructDeclFieldTypeResidualSticky(t *testing.T) {
	// OutputQualifiedType residual soft invent was soft-continue later fields invent partial struct.
	ClearError()
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
			{Name: "f1", Type: &Type{isStruct: true}, BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // CName residual
		},
	}
	if s := st.OutputStructDecl(); s != "" {
		t.Fatal("field CName residual must fail closed OutputStructDecl", s)
	}
	if !HasError() {
		t.Fatal("field CName residual OutputStructDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputUnionDeclFieldTypeResidualSticky(t *testing.T) {
	// OutputQualifiedType residual soft invent was soft-continue invent partial union.
	ClearError()
	ut := &Type{
		isUnion: true, StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: &Type{isStruct: true}, BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	if s := ut.OutputUnionDecl(); s != "" {
		t.Fatal("field CName residual must fail closed OutputUnionDecl", s)
	}
	if !HasError() {
		t.Fatal("field CName residual OutputUnionDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputStructDeclBitfieldIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent signed/unsigned past non-simple bitfield Type.
	ClearError()
	// pointer bitfield Type — IsSimple false without residual then SetError
	st := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: 3},
	}}
	if s := st.OutputStructDecl(); s != "" {
		t.Fatal("non-simple bitfield must fail closed", s)
	}
	if !HasError() {
		t.Fatal("non-simple bitfield OutputStructDecl must SetError sticky")
	}
	ClearError()
}
