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

func TestVariableCompatibleMatchIncompleteSticky(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	if (*Variable)(nil).Compatible(v, false) {
		t.Fatal("nil Variable Compatible must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Variable Compatible must SetError sticky")
	}
	ClearError()
	if v.Compatible(nil, false) {
		t.Fatal("nil other Compatible must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil other Compatible must SetError sticky")
	}
	ClearError()
	if !v.Compatible(v, false) {
		t.Fatal("self Compatible must be true")
	}
	if HasError() {
		t.Fatal("self Compatible must not sticky")
	}
	ClearError()
	if (*Variable)(nil).Match(v) {
		t.Fatal("nil Variable Match must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Variable Match must SetError sticky")
	}
	ClearError()
	if !v.Match(v) {
		t.Fatal("self Match must be true")
	}
	if HasError() {
		t.Fatal("self Match must not sticky")
	}
	ClearError()
	// special Type-nil complete not-match (unless identity)
	if NullPtr.Match(v) {
		t.Fatal("special Match other must stay complete false")
	}
	if HasError() {
		t.Fatal("special Match must not sticky")
	}
	ClearError()
	if !NullPtr.Match(NullPtr) {
		t.Fatal("special self Match must be true")
	}
	if HasError() {
		t.Fatal("special self Match must not sticky")
	}
	ClearError()
	// non-special Type-nil sticky (no invent not-match soft-skip)
	broken := &Variable{Name: "broken"}
	if broken.Match(v) {
		t.Fatal("Type-nil Match must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil Match must SetError sticky")
	}
	ClearError()
	if v.Match(broken) {
		t.Fatal("Match Type-nil other must fail closed false")
	}
	if !HasError() {
		t.Fatal("Match Type-nil other must SetError sticky")
	}
	ClearError()
	// HasFieldVar residual soft invent was Match true/false past FieldVars hole.
	// Fair: sticky not-match.
	agg := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	parent := &Variable{Name: "g_s", Type: agg, FieldVars: []*Variable{nil}}
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	if parent.Match(field) {
		t.Fatal("HasFieldVar residual Match must fail closed false")
	}
	if !HasError() {
		t.Fatal("HasFieldVar residual Match must SetError sticky")
	}
	ClearError()
}

func TestCreateFieldVarsNilSticky(t *testing.T) {
	// Variable always live; sticky no invent soft-skip create past missing shell
	ClearError()
	(*Variable)(nil).CreateFieldVars()
	if !HasError() {
		t.Fatal("nil CreateFieldVars must SetError sticky")
	}
	ClearError()
}

func TestHasFieldVarLooseMatchIncompleteSticky(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	if (*Variable)(nil).HasFieldVar(v) {
		t.Fatal("nil HasFieldVar must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil HasFieldVar must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).LooseMatch(v) {
		t.Fatal("nil LooseMatch must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil LooseMatch must SetError sticky")
	}
	ClearError()
}

func TestVariableKindPredicatesNilSticky(t *testing.T) {
	ClearError()
	if (*Variable)(nil).IsGlobal() {
		t.Fatal("nil IsGlobal must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).IsConst() || (*Variable)(nil).IsVolatile() {
		t.Fatal("nil IsConst/IsVolatile must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsConst/IsVolatile must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).IsVisible(nil) {
		t.Fatal("nil IsVisible must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsVisible must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).OutputLhsC() != "" {
		t.Fatal("nil OutputLhsC must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputLhsC must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).HashOutput() != "" {
		t.Fatal("nil HashOutput must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil HashOutput must SetError sticky")
	}
	ClearError()
	// special Type-nil complete not-pointer / not-aggregate
	if NullPtr.IsPointer() || GarbagePtr.IsAggregate() {
		t.Fatal("special Type-nil must stay complete not-pointer/not-aggregate")
	}
	if HasError() {
		t.Fatal("special Type-nil IsPointer/IsAggregate must not sticky")
	}
	ClearError()
	// non-special Type-nil incomplete shell sticky (no invent not-pointer soft-skip)
	broken := &Variable{Name: "broken"}
	if broken.IsPointer() {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearError()
	if broken.IsAggregate() {
		t.Fatal("Type-nil IsAggregate must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil IsAggregate must SetError sticky")
	}
	ClearError()
	// complete predicates
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	if !p.IsPointer() {
		t.Fatal("pointer IsPointer must be true")
	}
	if p.IsAggregate() {
		t.Fatal("pointer IsAggregate must be false")
	}
	if HasError() {
		t.Fatal("complete IsPointer/IsAggregate must not sticky")
	}
	ClearError()
	// field with Type-nil parent sticky (no invent not-union-field soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", FieldVarOf: parent, Type: GetIntType()}
	if field.IsUnionField() {
		t.Fatal("Type-nil parent IsUnionField must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil parent IsUnionField must SetError sticky")
	}
	ClearError()
	// non-field complete false
	if p.IsUnionField() {
		t.Fatal("non-field IsUnionField must be false complete")
	}
	if HasError() {
		t.Fatal("non-field IsUnionField must not sticky")
	}
	ClearError()
}

func TestFieldVarsCompleteNilIncomplete(t *testing.T) {
	// nil Variable is incomplete shell — not invent empty-complete fields
	if (*Variable)(nil).FieldVarsComplete() {
		t.Fatal("nil FieldVarsComplete must be incomplete false")
	}
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	if !v.FieldVarsComplete() {
		t.Fatal("scalar FieldVarsComplete empty complete")
	}
}

func TestIsValidVolatileInitExprResidualSticky(t *testing.T) {
	// NotEquals residual soft invent was invent valid-true past incomplete InitExpr shell.
	ClearError()
	// const volatile pointer with Type-nil InitExpr residual NotEquals
	pt := PointerTo(GetIntType())
	v := CreateVariableQfer("g_p", pt, NewCVQualifiers([]bool{true, false}, []bool{true, false}))
	if v == nil {
		t.Fatal("create")
	}
	// ensure const+vol at storage level
	v.Qfer = NewCVQualifiers([]bool{true}, []bool{true})
	v.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil residual
	v.Init = nil
	if v.IsValidVolatile() {
		t.Fatal("InitExpr NotEquals residual must fail closed invalid volatile")
	}
	if !HasError() {
		t.Fatal("InitExpr NotEquals residual IsValidVolatile must SetError sticky")
	}
	ClearError()
}

func TestIsValidVolatileNonConstResidualHygiene(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	if !v.IsValidVolatile() {
		t.Fatal("non-const volatile must be valid")
	}
	if HasError() {
		t.Fatal("complete IsValidVolatile must not sticky")
	}
	ClearError()
}

func TestCompatibleIsVolatileResidualSticky(t *testing.T) {
	// IsVolatile residual soft invent was invent soft-compat past nil subject already sticky.
	ClearError()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	if a.Compatible(nil, false) {
		t.Fatal("nil other Compatible must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil other Compatible must SetError sticky")
	}
	ClearError()
	// complete path no sticky
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	if a.Compatible(b, false) {
		// different vars not expand → false complete
	}
	if HasError() {
		t.Fatal("complete Compatible must not sticky")
	}
	ClearError()
	if !a.Compatible(a, false) {
		t.Fatal("same var Compatible must true")
	}
	if HasError() {
		t.Fatal("same var Compatible must not sticky")
	}
	ClearError()
}

func TestCreateFieldVarsIsVolatileResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-skip create past non-aggregate.
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	v.CreateFieldVars()
	if !HasError() {
		t.Fatal("non-aggregate CreateFieldVars must SetError sticky")
	}
	ClearError()
}

func TestIsPointerPtrTypeResidualSticky(t *testing.T) {
	// PtrType residual soft invent was invent not-pointer soft-skip past Type-nil.
	ClearError()
	if (&Variable{Name: "g_x", Type: nil}).IsPointer() {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearError()
	// complete pointer
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	if !p.IsPointer() {
		t.Fatal("pointer IsPointer must true")
	}
	if HasError() {
		t.Fatal("complete pointer IsPointer must not sticky")
	}
	ClearError()
}
