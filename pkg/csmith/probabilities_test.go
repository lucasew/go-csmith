package csmith

import "testing"

func TestSingleProbsDefaults(t *testing.T) {
	// Probabilities::initialize_single_probs with default CGOptions flags.
	p := NewProbabilities(Defaults())
	want := map[ProbName]int{
		PMoreStructUnionProb:    50,
		PBitFieldsCreationProb:  50,
		PRegularVolatileProb:    50, // volatiles on
		PRegularConstProb:       10, // consts on
		PSelectDerefPointerProb: 80, // pointers on
		PNewArrayVariableProb:   20, // arrays on
		PFloatAsLTypeProb:       0,  // float off
		PInlineFunctionProb:     50,
		PBuiltinFunctionProb:    50,
		PPointerAsLTypeProb:     50,
		PFieldVolatileProb:      30,
		PFieldConstProb:         20,
		PStdUnaryFuncProb:       5,
		PBinaryConstProb:        3,
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
}
