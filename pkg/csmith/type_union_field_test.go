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
	env := &TypeEnv{
		StructTypes: []*Type{withPtr},
		AllTypes:    []*Type{GetIntType(), withPtr},
	}
	// many seeds: never get pointer-containing struct
	for seed := uint64(1); seed < 60; seed++ {
		ClearError()
		f := MakeOneUnionField(NewRng(seed), opts, probs, env, 0)
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
	ClearError()
	opts := Defaults()
	opts.EnableFloat = false
	opts.Bitfields = false // always non-bitfield path
	probs := NewProbabilities(opts)
	// AllTypes like GenerateSimpleTypes: eChar.. (includes float with weight 0)
	var env TypeEnv
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
		ClearError()
		f := MakeOneUnionField(NewRng(seed), opts, probs, &env, 0)
		if f.Type == nil || HasError() {
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
	ClearError()
}

func TestMakeOneUnionFieldFilterResidualSticky(t *testing.T) {
	// ContainPointerField/HasBitfields Type-nil field residual: soft invent was soft-skip
	// then pick good simple. Fair: sticky fail closed whole MakeOneUnionField.
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	broken := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	env := &TypeEnv{AllTypes: []*Type{broken, GetIntType()}}
	// disable bitfield path so we always hit type-pool filter
	opts.Bitfields = false
	f := MakeOneUnionField(NewRng(1), opts, probs, env, 0)
	if f.Type != nil {
		t.Fatal("ContainPointerField residual must fail closed MakeOneUnionField")
	}
	if !HasError() {
		t.Fatal("ContainPointerField residual MakeOneUnionField must SetError sticky")
	}
	ClearError()
}

func TestMakeOneUnionFieldMayNestPlainStruct(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	plain := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	env := &TypeEnv{
		StructTypes: []*Type{plain},
		AllTypes:    []*Type{GetIntType(), plain},
	}
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		f := MakeOneUnionField(NewRng(seed), opts, probs, env, 0)
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
	ClearError()
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	if loc == nil {
		t.Fatal("create local")
	}
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	other := EmptyEffect().WriteVar(loc)
	cg.AddVisibleEffectAt(other, blk)
	if !cg.EffectAccum.IsWritten(loc) {
		t.Fatal("frame write via callers")
	}
}
