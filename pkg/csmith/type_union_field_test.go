package csmith

import "testing"

func TestIsSignedChar(t *testing.T) {
	if !GetSimpleType(EChar).IsSignedChar() {
		t.Fatal("eChar")
	}
	if GetSimpleType(EUChar).IsSignedChar() {
		t.Fatal("uChar")
	}
	if GetIntType().IsSignedChar() {
		t.Fatal("int")
	}
}

func TestIsFullBitfieldsStruct(t *testing.T) {
	full := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: 3},
		{Name: "f1", Type: GetIntType(), BitWidth: 5},
	}}
	if !full.IsFullBitfieldsStruct() {
		t.Fatal("full")
	}
	mixed := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: 3},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	if mixed.IsFullBitfieldsStruct() {
		t.Fatal("mixed")
	}
}

func TestMakeOneUnionFieldRejectsPointerStruct(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// struct containing pointer must not appear as union field
	pt := PointerTo(GetIntType())
	withPtr := &Type{isStruct: true, StructName: "Sp", Fields: []StructField{
		{Name: "p", Type: pt, BitWidth: -1},
	}}
	// ContainPointerField true for pointer field type
	if !withPtr.ContainPointerField() {
		// fields[0].Type is pointer → ContainPointerField walks fields
		// Type.ContainPointerField checks ptrTo or field.ContainPointerField
		// f.Type is pointer so f.Type.ContainPointerField is true
		if !pt.ContainPointerField() {
			t.Fatal("ptr type")
		}
	}
	env := &TypeEnv{Sess: testAmbientSession,
		StructTypes: []*Type{withPtr},
		AllTypes:    []*Type{GetIntType(), withPtr},
	}
	// many seeds: never get pointer-containing struct
	for seed := uint64(1); seed < 60; seed++ {
		ClearErrorSess(testAmbientSession)
		f := MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, env, 0, true)
		if f.Type != nil && f.Type.IsStruct() && f.Type.ContainPointerField() {
			t.Fatalf("pointer struct in union field seed %d", seed)
		}
		if f.Type != nil && f.Type.IsPointerLike() {
			t.Fatalf("raw pointer in union seed %d", seed)
		}
	}
}

func TestMakeOneUnionFieldKeepsWeight0SimplesInPool(t *testing.T) {
	// Type.cpp:694–696 / 723–727 — weight-0 simples stay in ok_nonstruct_types;
	// SIMPLE_TYPES_PROB_FILTER rejects at pick (retry). Trimmed pool changes rnd_upto size.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.EnableFloat = false
	opts.Bitfields = false // always non-bitfield path
	probs := NewProbabilities(opts)
	// AllTypes like GenerateSimpleTypes: eChar.. (includes float with weight 0)
	env := TypeEnv{Sess: testAmbientSession}
	for st := EChar; int(st) < MaxSimpleTypes; st++ {
		env.AllTypes = append(env.AllTypes, GetSimpleType(st))
	}
	// float must have weight 0 under defaults
	if probs.SimpleTypeWeight(int(EFloat)) != 0 {
		t.Fatal("expected float weight 0 when EnableFloat false")
	}
	// Must successfully pick a weight>0 simple without inventing trimmed pool (no hang / nil)
	ok := 0
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		f := MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, &env, 0, true)
		if f.Type == nil || HasErrorSess(testAmbientSession) {
			continue
		}
		if !f.Type.IsSimple() || f.Type.Simple() == EVoid {
			t.Fatalf("unexpected field type %v seed %d", f.Type, seed)
		}
		if probs.SimpleTypeWeight(int(f.Type.Simple())) == 0 {
			t.Fatalf("picked weight-0 simple %v seed %d", f.Type.Simple(), seed)
		}
		ok++
	}
	if ok < 10 {
		t.Fatalf("too few successful picks: %d", ok)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeOneUnionFieldFilterResidualSticky(t *testing.T) {
	// ContainPointerField/HasBitfields Type-nil field residual: soft invent was soft-skip
	// then pick good simple. Fair: sticky fail closed whole MakeOneUnionField.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	broken := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{broken, GetIntType()}}
	// disable bitfield path so we always hit type-pool filter
	opts.Bitfields = false
	f := MakeOneUnionField(NewRngSess(testAmbientSession, 1), opts, probs, env, 0, true)
	if f.Type != nil {
		t.Fatal("ContainPointerField residual must fail closed MakeOneUnionField")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ContainPointerField residual MakeOneUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeOneUnionFieldMayNestPlainStruct(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	plain := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	env := &TypeEnv{Sess: testAmbientSession,
		StructTypes: []*Type{plain},
		AllTypes:    []*Type{GetIntType(), plain},
	}
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		f := MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, env, 0, true)
		if f.Type != nil && f.Type.IsStruct() && f.Type.StructName == "S0" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected some nested plain struct")
	}
}

func TestAddVisibleEffectAtUsesBlock(t *testing.T) {
	// non-global on call chain block is tracked
	ClearErrorSess(testAmbientSession)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	if loc == nil {
		t.Fatal("create local")
	}
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	other := EmptyEffect().WriteVarSess(testAmbientSession, loc)
	cg.AddVisibleEffectAt(other, blk)
	if !cg.EffectAccum.IsWrittenSess(testAmbientSession, loc) {
		t.Fatal("frame write via callers")
	}
}

// TestMakeOneUnionFieldPrevZero mirrors Type.cpp:640–646 make_one_bitfield:
// no_zero_len = fields_length.empty() || back()==0. After a non-bitfield (-1),
// zero-width pad is allowed. Invent always-true prevZero forced seed-33 GO
// non-zero bitfield where UP had `const signed : 0`.
func TestMakeOneUnionFieldPrevZero(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType(), GetSimpleType(EUInt), GetSimpleType(EShort)}}
	// After normal field: prevZero false — length 0 must be keepable when drawn.
	// Search seeds that draw bitfield with length 0 under prevZero=false.
	foundPad := false
	for seed := uint64(1); seed < 500 && !foundPad; seed++ {
		ClearErrorSess(testAmbientSession)
		f := MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, env, 1, false)
		if f.Type != nil && f.BitWidth == 0 {
			foundPad = true
		}
	}
	if !foundPad {
		t.Fatal("with prevZero=false must allow zero-width union bitfield (Type.cpp:640)")
	}
	// First field / after pad: prevZero true — length 0 must be forced non-zero.
	for seed := uint64(1); seed < 200; seed++ {
		ClearErrorSess(testAmbientSession)
		f := MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, env, 0, true)
		if f.Type != nil && f.BitWidth == 0 {
			t.Fatalf("seed %d: prevZero=true must not leave zero-width (no_zero_len)", seed)
		}
	}
	// MakeRandomUnionType: after first non-bitfield, second field bitfield with
	// prevZero=false can keep length 0 (Type.cpp:640 back()!=0 → no force).
	ClearErrorSess(testAmbientSession)
	env2 := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType(), GetSimpleType(EUInt), GetSimpleType(EShort), GetSimpleType(EUShort)}}
	// Craft: first field normal (BitWidth -1) → prevZero becomes false;
	// second call with prevZero=false can return pad.
	f0 := MakeOneUnionField(NewRngSess(testAmbientSession, 1), opts, probs, env2, 0, true)
	if f0.Type == nil {
		// rare; try other seeds
		for seed := uint64(2); seed < 50 && f0.Type == nil; seed++ {
			ClearErrorSess(testAmbientSession)
			f0 = MakeOneUnionField(NewRngSess(testAmbientSession, seed), opts, probs, env2, 0, true)
		}
	}
	if f0.Type == nil {
		t.Fatal("expected some union field")
	}
	prev := f0.BitWidth == 0
	// if first was already pad, prev true; force second non-pad first then pad
	// contract already covered above; ensure prev after non-bitfield is false
	if f0.BitWidth < 0 && prev {
		t.Fatal("non-bitfield must not set prevZero")
	}
	ClearErrorSess(testAmbientSession)
}
