package csmith

import "testing"

func TestCreateVariableAndPredicates(t *testing.T) {
	v := CreateVariableScalars("g_1", GetSimpleType(EInt), true, false)
	if v == nil || !v.IsGlobal() || v.IsLocal() || !v.IsConst() || v.IsVolatile() {
		t.Fatalf("global const int: %+v const=%v vol=%v", v, v.IsConst(), v.IsVolatile())
	}
	p := CreateVariableScalars("p_1", GetSimpleType(EShort), false, true)
	if !p.IsArgument() || !p.IsVolatile() {
		t.Fatal("param volatile")
	}
	l := CreateVariableScalars("l_1", GetSimpleType(EChar), false, false)
	if !l.IsLocal() || l.IsGlobal() {
		t.Fatal("local")
	}
}

func TestCreateVariableRejectsVoid(t *testing.T) {
	// Variable.cpp:388/412 — assert(type)/void sticky; empty name soft factory non-sticky
	ClearError()
	if CreateVariableScalars("g_1", GetSimpleType(EVoid), false, false) != nil {
		t.Fatal("void simple must be rejected")
	}
	if !HasError() {
		t.Fatal("void CreateVariableScalars must SetError sticky")
	}
	ClearError()
	if CreateVariableScalars("g_n", nil, false, false) != nil {
		t.Fatal("nil type must be rejected")
	}
	if !HasError() {
		t.Fatal("nil type CreateVariableScalars must SetError sticky")
	}
	ClearError()
	if CreateVariableWithInit("g_n", nil, MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("CreateVariableWithInit nil type")
	}
	if !HasError() {
		t.Fatal("nil type CreateVariableWithInit must SetError sticky")
	}
	ClearError()
	// name always live; empty name soft factory (non-sticky re-pick gate)
	if CreateVariableScalars("", GetIntType(), false, false) != nil {
		t.Fatal("empty name must fail closed CreateVariableScalars")
	}
	if HasError() {
		t.Fatal("empty name CreateVariableScalars must stay non-sticky soft factory")
	}
	if CreateVariableWithInit("", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("empty name must fail closed CreateVariableWithInit")
	}
	if HasError() {
		t.Fatal("empty name CreateVariableWithInit must stay non-sticky soft factory")
	}
}

func TestCreateVariableErrorGuardAfterInit(t *testing.T) {
	// Variable.cpp:397/401 — ERROR_GUARD after Constant::make_random / field vars
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	if CreateVariableScalars("g_e", GetIntType(), false, false) != nil {
		t.Fatal("sticky error must fail CreateVariableScalars")
	}
	ClearError()
	SetError(ErrGeneric)
	if CreateVariableWithInit("g_e2", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("sticky error must fail CreateVariableWithInit after field expand")
	}
}

func TestCreateVariableScalarsUsesProcessProbs(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses Probabilities singleton
	opts := Defaults()
	probs := NewProbabilities(opts)
	prev := ProcessProbabilities()
	SetProcessProbabilities(probs)
	defer SetProcessProbabilities(prev)
	ClearError()
	// simple still works with process table
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if v == nil || v.Init == nil || v.Init.Value == "" {
		t.Fatal("simple init")
	}
	// aggregate needs live process probs (no invent NewProbabilities)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	s := CreateVariableScalars("g_s", st, false, false)
	if s == nil || s.Init == nil {
		t.Fatal("struct init needs process probs")
	}
	if len(s.FieldVars) != 1 || s.FieldVars[0].Init == nil {
		t.Fatal("field init needs process probs")
	}
}

func TestCreateVariableScalarsNilProcessProbsAggregateFailClosed(t *testing.T) {
	// no invent NewProbabilities when process singleton unset — sticky
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
	ClearError()
	st := &Type{isStruct: true, StructName: "SFail", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if CreateVariableScalars("g_s", st, false, false) != nil {
		t.Fatal("nil process probs must not invent aggregate init")
	}
	if !HasError() {
		t.Fatal("nil process probs aggregate must SetError sticky")
	}
	ClearError()
	// simple non-aggregate still works with process RNG (probs optional for simple)
	if CreateVariableScalars("g_i", GetIntType(), false, false) == nil {
		t.Fatal("simple with process RNG")
	}
}

func TestCreateVariableScalarsNilProcessRngFailClosed(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses process RNG; sticky no invent private stream
	prevR := ProcessRng()
	SetProcessRng(nil)
	defer SetProcessRng(prevR)
	ClearError()
	// simple needs pure_rnd — fail closed sticky without invent nextCreateVarRng
	if CreateVariableScalars("g_i", GetIntType(), false, false) != nil {
		t.Fatal("simple must fail closed without ProcessRng")
	}
	if !HasError() {
		t.Fatal("simple without ProcessRng must SetError sticky")
	}
	ClearError()
	// pointer constant is "0" without RNG draws (Constant.cpp:308–310)
	if CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false) == nil {
		t.Fatal("pointer init must not require RNG")
	}
}

func TestNewProgramGeneratorSetsProcessProbs(t *testing.T) {
	opts := Defaults()
	g := NewProgramGenerator(opts)
	if ProcessProbabilities() != g.Probs {
		t.Fatal("process probs must be session table")
	}
	if g.VS.Probs != g.Probs {
		t.Fatal("VS share")
	}
	if ProcessRng() != g.Rng {
		t.Fatal("process RNG must be session DefaultRndNumGenerator")
	}
}

func TestCreateVariableScalarsUsesProcessRng(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses process DefaultRndNumGenerator
	opts := Defaults()
	r := NewRng(42)
	prevR := ProcessRng()
	prevP := ProcessProbabilities()
	SetProcessRng(r)
	SetProcessProbabilities(NewProbabilities(opts))
	defer func() {
		SetProcessRng(prevR)
		SetProcessProbabilities(prevP)
	}()
	ClearError()
	// burn some process draws so depth moves
	before := r.RandDepth()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if v == nil || v.Init == nil {
		t.Fatal("want init")
	}
	if r.RandDepth() <= before {
		t.Fatalf("CreateVariable must burn process RNG (depth %d → %d)", before, r.RandDepth())
	}
}
