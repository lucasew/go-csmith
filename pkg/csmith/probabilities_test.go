package csmith

import "testing"

func TestSingleProbsDefaults(t *testing.T) {
	// Probabilities.cpp:478–585 initialize_single_probs — full default map vs C++.
	p := NewProbabilities(Defaults())
	want := map[ProbName]int{
		PMoreStructUnionProb:            50,
		PBitFieldsCreationProb:          50,
		PBitFieldInNormalStructProb:     10,
		PScalarFieldInFullBitFieldsProb: 10,
		PExhaustiveBitFieldsProb:        10,
		PBitFieldsSignedProb:            50,
		PSafeOpsSignedProb:              50,
		PFuncAttrProb:                   30,
		PTypeAttrProb:                   50,
		PLabelAttrProb:                  30,
		PVarAttrProb:                    30,
		PBinaryConstProb:                3,
		PRegularVolatileProb:            50, // volatiles on
		PRegularConstProb:               10, // consts on
		PStricterConstProb:              50,
		PLooserConstProb:                50,
		PFieldVolatileProb:              30, // vol + vol_struct_union + globals
		PFieldConstProb:                 20, // consts + const_struct_union_fields
		PStdUnaryFuncProb:               5,
		PShiftByNonConstantProb:         50,
		PStructAsLTypeProb:              30,
		PUnionAsLTypeProb:               25,
		PFloatAsLTypeProb:               0, // float off
		PNewArrayVariableProb:           20,
		PPointerAsLTypeProb:             50,
		PSelectDerefPointerProb:         80,
		PAccessOnceVariableProb:         20,
		PInlineFunctionProb:             50, // CGOptions::inline_function_prob
		PBuiltinFunctionProb:            50, // CGOptions::builtin_function_prob
		PArrayOOBProb:                   0,
	}
	for name, w := range want {
		if got := p.Single(name); got != w {
			t.Errorf("single %v: got %d want %d", name, got, w)
		}
	}
}

func TestSingleProbsRespectFlags(t *testing.T) {
	o := Defaults()
	o.Pointers = false
	o.Arrays = false
	o.Volatiles = false
	o.Consts = false
	p := NewProbabilities(o)
	if p.Single(PPointerAsLTypeProb) != 0 || p.Single(PSelectDerefPointerProb) != 0 {
		t.Fatal("pointers off → pointer probs 0")
	}
	if p.Single(PNewArrayVariableProb) != 0 {
		t.Fatal("arrays off → new array prob 0")
	}
	if p.Single(PRegularVolatileProb) != 0 || p.Single(PRegularConstProb) != 0 {
		t.Fatal("vol/const off → regular probs 0")
	}
}

func TestSimpleTypeWeightsDefaults(t *testing.T) {
	// set_default_simple_types_prob under defaults
	p := NewProbabilities(Defaults())
	if p.SimpleTypeWeight(int(EVoid)) != 0 {
		t.Fatal("void weight must be 0")
	}
	enabled := []ESimpleType{EChar, EInt, EShort, ELong, ELongLong, EUChar, EUInt, EUShort, EULong, EULongLong}
	for _, st := range enabled {
		if p.SimpleTypeWeight(int(st)) != 1 {
			t.Errorf("%v weight want 1 got %d", st, p.SimpleTypeWeight(int(st)))
		}
	}
	disabled := []ESimpleType{EFloat, EInt128, EUInt128}
	for _, st := range disabled {
		if p.SimpleTypeWeight(int(st)) != 0 {
			t.Errorf("%v weight want 0 got %d", st, p.SimpleTypeWeight(int(st)))
		}
	}
}

func TestSimpleTypeWeightsNoInt64(t *testing.T) {
	o := Defaults()
	o.LongLong = false
	p := NewProbabilities(o)
	if p.SimpleTypeWeight(int(ELongLong)) != 0 || p.SimpleTypeWeight(int(EULongLong)) != 0 {
		t.Fatal("longlong off → long long weights 0")
	}
}

func TestChooseRandomNonvoidSimpleNeverVoid(t *testing.T) {
	// Type::choose_random_nonvoid_simple with SIMPLE_TYPES_PROB_FILTER
	p := NewProbabilities(Defaults())
	r := NewRng(2)
	seen := map[ESimpleType]int{}
	for i := 0; i < 200; i++ {
		st := ChooseRandomNonvoidSimple(r, p)
		if st == EVoid {
			t.Fatalf("iter %d: void chosen", i)
		}
		if p.SimpleTypeWeight(int(st)) == 0 {
			t.Fatalf("iter %d: zero-weight type %v", i, st)
		}
		seen[st]++
	}
	if len(seen) < 3 {
		t.Fatalf("expected diversity among simple types, got %v", seen)
	}
}

func TestChooseRandomNonvoidSimpleSeed2First(t *testing.T) {
	// Deterministic first draw: rnd_upto(14, filter) after seed 2.
	// First genrand = 1959434203; scan until weight>0.
	p := NewProbabilities(Defaults())
	r := NewRng(2)
	st := ChooseRandomNonvoidSimple(r, p)
	// Manual: try v = raw%14 until not filtered.
	r2 := NewRng(2)
	raw := r2.Genrand()
	v := raw % 14
	for p.SimpleTypeWeight(int(v)) == 0 {
		raw = r2.Genrand()
		v = raw % 14
	}
	if st != ESimpleType(v) {
		t.Fatalf("first choose: got %v want %v", st, ESimpleType(v))
	}
	// C++ always has RNG+probs sticky — no invent EInt when missing
	ClearError()
	if ChooseRandomNonvoidSimple(nil, p) != EVoid {
		t.Fatal("nil RNG must fail closed EVoid")
	}
	if !HasError() {
		t.Fatal("nil RNG ChooseRandomNonvoidSimple must SetError sticky")
	}
	ClearError()
	if ChooseRandomNonvoidSimple(NewRng(1), nil) != EVoid {
		t.Fatal("nil probs must fail closed EVoid")
	}
	if !HasError() {
		t.Fatal("nil probs ChooseRandomNonvoidSimple must SetError sticky")
	}
	ClearError()
}

func TestProbabilitiesNilSticky(t *testing.T) {
	ClearError()
	if (*Probabilities)(nil).Single(PMoreStructUnionProb) != 0 {
		t.Fatal("nil Single must return 0")
	}
	if !HasError() {
		t.Fatal("nil Single must SetError sticky")
	}
	ClearError()
	if (*Probabilities)(nil).BinaryOpWeight(0) != 0 {
		t.Fatal("nil BinaryOpWeight must return 0")
	}
	if !HasError() {
		t.Fatal("nil BinaryOpWeight must SetError sticky")
	}
	ClearError()
	if (*Probabilities)(nil).StatementThresholdTable() != nil {
		t.Fatal("nil StatementThresholdTable must return nil")
	}
	if !HasError() {
		t.Fatal("nil StatementThresholdTable must SetError sticky")
	}
	ClearError()
}

func TestSimpleTypesFilterNilProbsResidualSticky(t *testing.T) {
	// SimpleTypeWeight residual soft invent was invent keep candidate past nil probs.
	ClearError()
	f := (*Probabilities)(nil).SimpleTypesFilter()
	if !f.Filter(0) {
		t.Fatal("nil probs filter must reject (filter true) fail closed")
	}
	if !HasError() {
		t.Fatal("nil probs SimpleTypesFilter must SetError sticky")
	}
	ClearError()
}

func TestProbabilityFilterEqualGroup(t *testing.T) {
	// ProbabilityFilter for pSimpleTypesProb via process singleton.
	prev := ProcessProbabilities()
	p := NewProbabilities(Defaults())
	SetProcessProbabilities(p)
	defer SetProcessProbabilities(prev)

	f := GetProbFilter(PSimpleTypesProb)
	// void weight 0 → reject
	if !f.Filter(uint32(EVoid)) {
		t.Fatal("void must be filtered")
	}
	// eInt weight 1 → accept
	if f.Filter(uint32(EInt)) {
		t.Fatal("eInt must pass filter")
	}
	// BINARY_OPS with muls off
	o := Defaults()
	o.Muls = false
	p2 := NewProbabilities(o)
	SetProcessProbabilities(p2)
	bf := GetProbFilter(PBinaryOpsProb)
	if !bf.Filter(uint32(BinMul)) {
		t.Fatal("mul disabled must filter")
	}
	if bf.Filter(uint32(BinAdd)) {
		t.Fatal("add must pass")
	}
}

// rejectSimple mirrors a VectorFilter-style extra reject on one simple type index.
type rejectSimple uint32

func (r *rejectSimple) Filter(v uint32) bool { return v == uint32(*r) }

func TestRegisterExtraFilter(t *testing.T) {
	// Probabilities.cpp:791–813 register + check_extra_filter
	prev := ProcessProbabilities()
	p := NewProbabilities(Defaults())
	SetProcessProbabilities(p)
	defer SetProcessProbabilities(prev)

	// Reject eInt via extra filter even though weight is 1 (pointer Filter for identity)
	rej := rejectSimple(EInt)
	extra := &rej
	RegisterExtraFilter(PSimpleTypesProb, extra)
	f := GetProbFilter(PSimpleTypesProb)
	if !f.Filter(uint32(EInt)) {
		t.Fatal("extra filter must reject eInt before weight check")
	}
	// eShort still passes (weight 1, extra false)
	if f.Filter(uint32(EShort)) {
		t.Fatal("eShort must still pass")
	}
	UnregisterExtraFilter(PSimpleTypesProb, extra)
	if f.Filter(uint32(EInt)) {
		t.Fatal("after unregister, eInt weight 1 must pass")
	}
}

func TestGetProbFilterMissingSticky(t *testing.T) {
	// No process probs → fail closed
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
	ClearError()
	f := GetProbFilter(PSimpleTypesProb)
	if !f.Filter(0) {
		t.Fatal("missing process probs filter must reject")
	}
	if !HasError() {
		t.Fatal("GetProbFilter without process must SetError sticky")
	}
	ClearError()
}

func TestProbabilityFilterNilReceiver(t *testing.T) {
	ClearError()
	if !(*ProbabilityFilter)(nil).Filter(0) {
		t.Fatal("nil ProbabilityFilter must reject")
	}
	if !HasError() {
		t.Fatal("nil ProbabilityFilter must SetError sticky")
	}
	ClearError()
}
