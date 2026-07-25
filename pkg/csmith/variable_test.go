package csmith

import "testing"

func TestCreateVariableAndPredicates(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetSimpleTypeSess(testAmbientSession, EInt), true, false)
	if v == nil || !v.IsGlobalSess(testAmbientSession) || v.IsLocalSess(testAmbientSession) || !v.IsConstSess(testAmbientSession) || v.IsVolatileSess(testAmbientSession) {
		t.Fatalf("global const int: %+v const=%v vol=%v", v, v.IsConstSess(testAmbientSession), v.IsVolatileSess(testAmbientSession))
	}
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", GetSimpleTypeSess(testAmbientSession, EShort), false, true)
	if !p.IsArgumentSess(testAmbientSession) || !p.IsVolatileSess(testAmbientSession) {
		t.Fatal("param volatile")
	}
	l := CreateVariableScalarsSess(testAmbientSession, "l_1", GetSimpleTypeSess(testAmbientSession, EChar), false, false)
	if !l.IsLocalSess(testAmbientSession) || l.IsGlobalSess(testAmbientSession) {
		t.Fatal("local")
	}
}

func TestCreateVariableRejectsVoid(t *testing.T) {
	// Variable.cpp:388/412 — assert(type)/void sticky; empty name soft factory non-sticky
	ClearErrorSess(testAmbientSession)
	if CreateVariableScalarsSess(testAmbientSession, "g_1", GetSimpleTypeSess(testAmbientSession, EVoid), false, false) != nil {
		t.Fatal("void simple must be rejected")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void CreateVariableScalars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CreateVariableScalarsSess(testAmbientSession, "g_n", nil, false, false) != nil {
		t.Fatal("nil type must be rejected")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type CreateVariableScalars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if CreateVariableWithInitSess(testAmbientSession, "g_n", nil, MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("CreateVariableWithInit nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type CreateVariableWithInit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// name always live; empty name soft factory (non-sticky re-pick gate)
	if CreateVariableScalarsSess(testAmbientSession, "", GetIntTypeSess(testAmbientSession), false, false) != nil {
		t.Fatal("empty name must fail closed CreateVariableScalars")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty name CreateVariableScalars must stay non-sticky soft factory")
	}
	if CreateVariableWithInitSess(testAmbientSession, "", GetIntTypeSess(testAmbientSession), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("empty name must fail closed CreateVariableWithInit")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty name CreateVariableWithInit must stay non-sticky soft factory")
	}
}

func TestCreateVariableErrorGuardAfterInit(t *testing.T) {
	// Variable.cpp:397/401 — ERROR_GUARD after Constant::make_random / field vars
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if CreateVariableScalarsSess(testAmbientSession, "g_e", GetIntTypeSess(testAmbientSession), false, false) != nil {
		t.Fatal("sticky error must fail CreateVariableScalars")
	}
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	if CreateVariableWithInitSess(testAmbientSession, "g_e2", GetIntTypeSess(testAmbientSession), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("sticky error must fail CreateVariableWithInit after field expand")
	}
}

func TestCreateVariableScalarsUsesProcessProbs(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses Probabilities singleton
	opts := Defaults()
	probs := NewProbabilities(opts)
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, probs)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	ClearErrorSess(testAmbientSession)
	// simple still works with process table
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if v == nil || v.Init == nil || v.Init.Value == "" {
		t.Fatal("simple init")
	}
	// aggregate needs live process probs (no invent NewProbabilities)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	s := CreateVariableScalarsSess(testAmbientSession, "g_s", st, false, false)
	if s == nil || s.Init == nil {
		t.Fatal("struct init needs process probs")
	}
	if len(s.FieldVars) != 1 || s.FieldVars[0].Init == nil {
		t.Fatal("field init needs process probs")
	}
}

func TestCreateVariableScalarsNilProcessProbsAggregateFailClosed(t *testing.T) {
	// no invent NewProbabilities when process singleton unset — sticky
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "SFail", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if CreateVariableScalarsSess(testAmbientSession, "g_s", st, false, false) != nil {
		t.Fatal("nil process probs must not invent aggregate init")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil process probs aggregate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// simple non-aggregate still works with process RNG (probs optional for simple)
	if CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false) == nil {
		t.Fatal("simple with process RNG")
	}
}

func TestCreateVariableScalarsNilProcessRngFailClosed(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses process RNG; sticky no invent private stream
	prevR := ProcessRngSess(testAmbientSession)
	SetProcessRngSess(testAmbientSession, nil)
	defer SetProcessRngSess(testAmbientSession, prevR)
	ClearErrorSess(testAmbientSession)
	// simple needs pure_rnd — fail closed sticky without invent nextCreateVarRng
	if CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false) != nil {
		t.Fatal("simple must fail closed without ProcessRng")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("simple without ProcessRng must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// pointer constant is "0" without RNG draws (Constant.cpp:308–310)
	if CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false) == nil {
		t.Fatal("pointer init must not require RNG")
	}
}

func TestNewProgramGeneratorSetsProcessProbs(t *testing.T) {
	opts := Defaults()
	s := NewSession(opts)
	g := NewProgramGenerator(s)
	// Tables live on the session bag (ambient Process* only while activated).
	if s.Probs != g.Probs {
		t.Fatal("session probs must be generator table")
	}
	if g.VS.Probs != g.Probs {
		t.Fatal("VS share")
	}
	if s.Rng != g.Rng {
		t.Fatal("session RNG must be generator DefaultRndNumGenerator")
	}
	if g.Sess != s || g.VS.Sess != s {
		t.Fatal("Sess wiring")
	}
}

func TestCreateVariableScalarsUsesProcessRng(t *testing.T) {
	// Variable.cpp:395 — Constant::make_random uses process DefaultRndNumGenerator
	opts := Defaults()
	r := NewRngSess(testAmbientSession, 42)
	prevR := ProcessRngSess(testAmbientSession)
	prevP := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessRngSess(testAmbientSession, r)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	defer func() {
		SetProcessRngSess(testAmbientSession, prevR)
		SetProcessProbabilitiesSess(testAmbientSession, prevP)
	}()
	ClearErrorSess(testAmbientSession)
	// burn some process draws so depth moves
	before := r.RandDepth()
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if v == nil || v.Init == nil {
		t.Fatal("want init")
	}
	if r.RandDepth() <= before {
		t.Fatalf("CreateVariable must burn process RNG (depth %d → %d)", before, r.RandDepth())
	}
}

func TestVariableCompatibleMatchIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if (*Variable)(nil).CompatibleSess(testAmbientSession, v, false) {
		t.Fatal("nil Variable Compatible must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable Compatible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if v.CompatibleSess(testAmbientSession, nil, false) {
		t.Fatal("nil other Compatible must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other Compatible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !v.CompatibleSess(testAmbientSession, v, false) {
		t.Fatal("self Compatible must be true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("self Compatible must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).MatchSess(testAmbientSession, v) {
		t.Fatal("nil Variable Match must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable Match must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !v.MatchSess(testAmbientSession, v) {
		t.Fatal("self Match must be true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("self Match must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// special Type-nil complete not-match (unless identity)
	if NullPtr.MatchSess(testAmbientSession, v) {
		t.Fatal("special Match other must stay complete false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("special Match must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !NullPtr.MatchSess(testAmbientSession, NullPtr) {
		t.Fatal("special self Match must be true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("special self Match must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-special Type-nil sticky (no invent not-match soft-skip)
	broken := &Variable{Name: "broken"}
	if broken.MatchSess(testAmbientSession, v) {
		t.Fatal("Type-nil Match must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Match must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if v.MatchSess(testAmbientSession, broken) {
		t.Fatal("Match Type-nil other must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match Type-nil other must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// HasFieldVar residual soft invent was Match true/false past FieldVars hole.
	// Fair: sticky not-match.
	agg := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	parent := &Variable{Name: "g_s", Type: agg, FieldVars: []*Variable{nil}}
	field := &Variable{Name: "g_s.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	if parent.MatchSess(testAmbientSession, field) {
		t.Fatal("HasFieldVar residual Match must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasFieldVar residual Match must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateFieldVarsNilSticky(t *testing.T) {
	// Variable always live; sticky no invent soft-skip create past missing shell
	ClearErrorSess(testAmbientSession)
	(*Variable)(nil).CreateFieldVarsSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CreateFieldVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasFieldVarLooseMatchIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if (*Variable)(nil).HasFieldVarSess(testAmbientSession, v) {
		t.Fatal("nil HasFieldVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HasFieldVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).LooseMatchSess(testAmbientSession, v) {
		t.Fatal("nil LooseMatch must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil LooseMatch must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariableKindPredicatesNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsGlobalSess(testAmbientSession) {
		t.Fatal("nil IsGlobal must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsConstSess(testAmbientSession) || (*Variable)(nil).IsVolatileSess(testAmbientSession) {
		t.Fatal("nil IsConst/IsVolatile must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsConst/IsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsVisibleSess(testAmbientSession, nil) {
		t.Fatal("nil IsVisible must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsVisible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).OutputLhsCOptsSess(testAmbientSession, false) != "" {
		t.Fatal("nil OutputLhsC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil OutputLhsC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).HashOutputSess(testAmbientSession) != "" {
		t.Fatal("nil HashOutput must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// special Type-nil complete not-pointer / not-aggregate
	if NullPtr.IsPointerSess(testAmbientSession) || GarbagePtr.IsAggregateSess(testAmbientSession) {
		t.Fatal("special Type-nil must stay complete not-pointer/not-aggregate")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("special Type-nil IsPointer/IsAggregate must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-special Type-nil incomplete shell sticky (no invent not-pointer soft-skip)
	broken := &Variable{Name: "broken"}
	if broken.IsPointerSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if broken.IsAggregateSess(testAmbientSession) {
		t.Fatal("Type-nil IsAggregate must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsAggregate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete predicates
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if !p.IsPointerSess(testAmbientSession) {
		t.Fatal("pointer IsPointer must be true")
	}
	if p.IsAggregateSess(testAmbientSession) {
		t.Fatal("pointer IsAggregate must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsPointer/IsAggregate must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// field with Type-nil parent sticky (no invent not-union-field soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", FieldVarOf: parent, Type: GetIntTypeSess(testAmbientSession)}
	if field.IsUnionFieldSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsUnionField must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-field complete false
	if p.IsUnionFieldSess(testAmbientSession) {
		t.Fatal("non-field IsUnionField must be false complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("non-field IsUnionField must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFieldVarsCompleteNilIncomplete(t *testing.T) {
	// nil Variable is incomplete shell — not invent empty-complete fields
	if (*Variable)(nil).FieldVarsCompleteSess(testAmbientSession) {
		t.Fatal("nil FieldVarsComplete must be incomplete false")
	}
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if !v.FieldVarsCompleteSess(testAmbientSession) {
		t.Fatal("scalar FieldVarsComplete empty complete")
	}
}

func TestIsValidVolatileInitExprResidualSticky(t *testing.T) {
	// NotEquals residual soft invent was invent valid-true past incomplete InitExpr shell.
	ClearErrorSess(testAmbientSession)
	// const volatile pointer with Type-nil InitExpr residual NotEquals
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	v := CreateVariableQferSess(testAmbientSession, "g_p", pt, NewCVQualifiers([]bool{true, false}, []bool{true, false}))
	if v == nil {
		t.Fatal("create")
	}
	// ensure const+vol at storage level
	v.Qfer = NewCVQualifiers([]bool{true}, []bool{true})
	v.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil residual
	v.Init = nil
	if v.IsValidVolatileSess(testAmbientSession) {
		t.Fatal("InitExpr NotEquals residual must fail closed invalid volatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("InitExpr NotEquals residual IsValidVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsValidVolatileNonConstResidualHygiene(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	if !v.IsValidVolatileSess(testAmbientSession) {
		t.Fatal("non-const volatile must be valid")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsValidVolatile must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleIsVolatileResidualSticky(t *testing.T) {
	// IsVolatile residual soft invent was invent soft-compat past nil subject already sticky.
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	if a.CompatibleSess(testAmbientSession, nil, false) {
		t.Fatal("nil other Compatible must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other Compatible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete path no sticky
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	if a.CompatibleSess(testAmbientSession, b, false) {
		// different vars not expand → false complete
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete Compatible must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !a.CompatibleSess(testAmbientSession, a, false) {
		t.Fatal("same var Compatible must true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("same var Compatible must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateFieldVarsIsVolatileResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-skip create past non-aggregate.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	v.CreateFieldVarsSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-aggregate CreateFieldVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsPointerPtrTypeResidualSticky(t *testing.T) {
	// PtrType residual soft invent was invent not-pointer soft-skip past Type-nil.
	ClearErrorSess(testAmbientSession)
	if (&Variable{Name: "g_x", Type: nil}).IsPointerSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete pointer
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if !p.IsPointerSess(testAmbientSession) {
		t.Fatal("pointer IsPointer must true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete pointer IsPointer must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateVariableScalarsVoidIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent void scalar create past void type shell.
	ClearErrorSess(testAmbientSession)
	if CreateVariableScalarsSess(testAmbientSession, "g_v", GetSimpleTypeSess(testAmbientSession, EVoid), false, false) != nil {
		t.Fatal("void CreateVariableScalars must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void CreateVariableScalars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil sticky
	if CreateVariableScalarsSess(testAmbientSession, "g_x", nil, false, false) != nil {
		t.Fatal("Type-nil CreateVariableScalars must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil CreateVariableScalars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateVariableWithInitVoidIsSimpleResidualSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if CreateVariableWithInitSess(testAmbientSession, "g_v", GetSimpleTypeSess(testAmbientSession, EVoid), nil, NewCVQualifiers([]bool{false}, []bool{false})) != nil {
		t.Fatal("void CreateVariableWithInit must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void CreateVariableWithInit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsVolatileIncludesVolatileStructUnion(t *testing.T) {
	// Variable.cpp:519 — is_volatile() → is_volatile_after_deref(0) which ORs
	// type->is_volatile_struct_union(). Qfer-only IsVolatile missed S0-style
	// aggregates (volatile bitfields, non-vol storage) and left side_effect_free.
	ClearErrorSess(testAmbientSession)
	volQ := NewCVQualifiers([]bool{false}, []bool{true}) // non-const, volatile field
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetSimpleTypeSess(testAmbientSession, EUInt), Qfer: volQ, BitWidth: 15},
		},
	}
	if !st.IsVolatileStructUnionSess(testAmbientSession) {
		t.Fatal("struct with volatile field must IsVolatileStructUnion")
	}
	// Storage qfer non-vol
	v := CreateVariableScalarsSess(testAmbientSession, "g_44", st, true, false)
	if v == nil {
		t.Fatal("create")
	}
	// Force type (CreateVariableScalars may wrap)
	v.Type = st
	v.Qfer = NewCVQualifiers([]bool{false}, []bool{false})
	if !v.IsVolatileSess(testAmbientSession) {
		t.Fatal("IsVolatile must true for volatile-field struct (qfer non-vol)")
	}
	if v.IsVolatileSess(testAmbientSession) != v.IsVolatileAfterDerefSess(testAmbientSession, 0) {
		t.Fatal("IsVolatile must equal IsVolatileAfterDeref(0)")
	}
	// plain int remains non-vol
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), true, false)
	if iv.IsVolatileSess(testAmbientSession) {
		t.Fatal("plain int non-vol")
	}
	// ReadVar of vol struct must clear SE-free
	e := EmptyEffect().ReadVarSess(testAmbientSession, v)
	if e.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("ReadVar volatile struct must clear side_effect_free")
	}
}

func TestCreateFieldVarsStorageVolOnly(t *testing.T) {
	// Variable.cpp:344–358 — create_field_vars ORs qfer.is_volatile() (storage),
	// not Variable::is_volatile() (includes is_volatile_struct_union).
	ClearErrorSess(testAmbientSession)
	volField := NewCVQualifiers([]bool{false}, []bool{true})
	plainField := NewCVQualifiers([]bool{false}, []bool{false})
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetSimpleTypeSess(testAmbientSession, EUInt), Qfer: volField, BitWidth: 15},
			{Name: "f1", Type: GetSimpleTypeSess(testAmbientSession, EInt), Qfer: plainField, BitWidth: -1},
		},
	}
	// non-vol storage qfer; type is vol-struct via f0
	v := CreateVariableScalarsSess(testAmbientSession, "g_s", st, false, false)
	if v == nil {
		t.Fatal("create")
	}
	v.Type = st
	v.Qfer = NewCVQualifiers([]bool{false}, []bool{false})
	v.FieldVars = nil
	ClearErrorSess(testAmbientSession)
	v.CreateFieldVarsSess(testAmbientSession)
	if HasErrorSess(testAmbientSession) || len(v.FieldVars) < 2 {
		t.Fatalf("fields err=%v n=%d", HasErrorSess(testAmbientSession), len(v.FieldVars))
	}
	// parent IsVolatile true (vol struct fields)
	if !v.IsVolatileSess(testAmbientSession) {
		t.Fatal("parent must IsVolatile via struct fields")
	}
	// f1 must stay non-vol (only storage OR field qfer)
	f1 := v.FieldVars[1]
	if f1.Qfer.IsVolatileSess(testAmbientSession) {
		t.Fatal("plain field must not inherit parent IsVolatile() type-OR")
	}
	if f1.IsVolatileSess(testAmbientSession) {
		t.Fatal("plain field Variable.IsVolatile must false")
	}
	// f0 stays vol from field qfer
	if !v.FieldVars[0].IsVolatileSess(testAmbientSession) {
		t.Fatal("vol field must remain volatile")
	}
}
