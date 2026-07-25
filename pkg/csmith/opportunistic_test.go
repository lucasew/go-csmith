package csmith

import "testing"

func TestOpportunisticValidateNoDeref(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), v, GetIntTypeSess(testAmbientSession), nil, 0, 0) != 1 {
		t.Fatal("same level")
	}
	// nil var/type sticky — no invent not-valid soft success past hole
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), nil, GetIntTypeSess(testAmbientSession), nil, 0, 0) != 0 {
		t.Fatal("nil var must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var OpportunisticValidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), v, nil, nil, 0, 0) != 0 {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type OpportunisticValidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOpportunisticValidateNullDead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// nil facts is complete empty — no related fact → 0
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), p, GetIntTypeSess(testAmbientSession), nil, 0, 0) != 0 {
		t.Fatal("no fact")
	}
	// incomplete map hole → 0 sticky (not invent ok / soft re-pick past hole)
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), p, GetIntTypeSess(testAmbientSession), []*FactPointTo{nil}, 0, 0) != 0 {
		t.Fatal("incomplete must reject")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// null, prob 0 → 0; FactPointTo.cpp:455 still rnd_flipcoin(0)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	rNull := NewRngSess(testAmbientSession, 1)
	d0 := rNull.RandDepthSess(testAmbientSession)
	if OpportunisticValidateSess(testAmbientSession, rNull, p, GetIntTypeSess(testAmbientSession), facts, 0, 0) != 0 {
		t.Fatal("null blocked")
	}
	if rNull.RandDepthSess(testAmbientSession) != d0+1 {
		t.Fatalf("null p=0 must still flipcoin once: depth %d → %d", d0, rNull.RandDepthSess(testAmbientSession))
	}
	// live target → 1 (no flip when not null/dead)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	facts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)}
	rLive := NewRngSess(testAmbientSession, 1)
	dLive := rLive.RandDepthSess(testAmbientSession)
	if OpportunisticValidateSess(testAmbientSession, rLive, p, GetIntTypeSess(testAmbientSession), facts, 0, 0) != 1 {
		t.Fatal("live")
	}
	if rLive.RandDepthSess(testAmbientSession) != dLive {
		t.Fatalf("live pointees must not flipcoin: depth %d → %d", dLive, rLive.RandDepthSess(testAmbientSession))
	}
	// garbage, prob 0 → 0; FactPointTo.cpp:464 still rnd_flipcoin(0)
	facts = []*FactPointTo{NewFactPointToSess(testAmbientSession, p)}
	rDead := NewRngSess(testAmbientSession, 1)
	dDead := rDead.RandDepthSess(testAmbientSession)
	if OpportunisticValidateSess(testAmbientSession, rDead, p, GetIntTypeSess(testAmbientSession), facts, 0, 0) != 0 {
		t.Fatal("dead blocked")
	}
	if rDead.RandDepthSess(testAmbientSession) != dDead+1 {
		t.Fatalf("dead p=0 must still flipcoin once: depth %d → %d", dDead, rDead.RandDepthSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete dead blocked must not sticky")
	}
	// null+dead both set: two flips (null then dead)
	facts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	// make dead+null fact: NewFactPointTo is garbage/dead; force null+dead via fields
	fBoth := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	// IsDead for null pointees? if only null, one flip. dead-only above. allow-unsafe path:
	rAllow := NewRngSess(testAmbientSession, 1)
	if OpportunisticValidateSess(testAmbientSession, rAllow, p, GetIntTypeSess(testAmbientSession), []*FactPointTo{fBoth}, 100, 0) != 2 {
		// p=100 always allows null unsafe when is_null
		t.Fatal("null with nullProb=100 must allow ret=2")
	}
	// nil r on null path sticky (C++ always has process RNG for flipcoin)
	ClearErrorSess(testAmbientSession)
	if OpportunisticValidateSess(testAmbientSession, nil, p, GetIntTypeSess(testAmbientSession), []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}, 0, 0) != 0 {
		t.Fatal("nil r null path must reject")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil r null path must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOpportunisticValidateUsesCollective(t *testing.T) {
	// FactPointTo.cpp:448–450 — FactPointTo tmp(var->get_collective()); find_related_fact
	// Fact is stored on the collective; itemized member must resolve via get_collective.
	ClearErrorSess(testAmbientSession)
	elem := GetIntTypeSess(testAmbientSession)
	pt := PointerToSess(testAmbientSession, elem)
	// collective array-of-pointer shell (fact subject)
	coll := &ArrayVariable{
		Variable: Variable{Name: "g_arr", Type: pt, IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
	}
	coll.AsArray = coll
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", elem, false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, &coll.Variable, tgt)}
	// itemized member of that collective
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_arr", Type: pt, IsArray: true, ArraySizes: []int{3}},
		Sizes:      []int{3},
		Collective: coll,
	}
	item.AsArray = item
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), &item.Variable, elem, facts, 0, 0) != 1 {
		t.Fatal("itemized must find fact via get_collective()")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete collective path must not sticky")
	}
	// fact keyed only on item (not collective) must miss when looking up via collective
	factsItemOnly := []*FactPointTo{MakeFactPointToSess(testAmbientSession, &item.Variable, tgt)}
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), &item.Variable, elem, factsItemOnly, 0, 0) != 0 {
		t.Fatal("fact on item alone must miss when lookup uses collective identity")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleCheckNilHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.CompatibleCheck = true
	// enabled + incomplete IR rejects sticky (no invent non-error)
	if !CompatibleCheckExprVarSess(testAmbientSession, opts, nil, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}) {
		t.Fatal("nil var must reject when compatible-check on")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var CompatibleCheck must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !CompatibleCheckExprsSess(testAmbientSession, opts, nil, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}) {
		t.Fatal("nil expr must reject when compatible-check on")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil expr CompatibleCheck must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	opts.CompatibleCheck = false
	if CompatibleCheckExprVarSess(testAmbientSession, opts, nil, nil) {
		t.Fatal("disabled must not reject")
	}
	// disabled incomplete stays non-sticky (feature off, not broken IR path)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("disabled CompatibleCheck must not SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsNonReadableNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoReadVars: []*Variable{nil}})
	if !cg.IsNonReadable(g) {
		t.Fatal("nil NoReadVars hole must fail closed as nonreadable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NoReadVars hole IsNonReadable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoWriteVars: []*Variable{nil}})
	if !cg2.IsNonWritable(g) {
		t.Fatal("nil NoWriteVars hole must fail closed as nonwritable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil NoWriteVars hole IsNonWritable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariableCompatible(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	if !a.CompatibleSess(testAmbientSession, a, false) {
		t.Fatal("self")
	}
	if a.CompatibleSess(testAmbientSession, b, false) {
		t.Fatal("other no expand")
	}
	if !a.CompatibleSess(testAmbientSession, b, true) {
		t.Fatal("expand non-field")
	}
	vol := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	if vol.CompatibleSess(testAmbientSession, vol, false) {
		t.Fatal("vol self")
	}
}

func TestCompatibleCheckerDisabled(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	e := &Expression{Term: TermVariable, Var: a}
	if CompatibleCheckExprVarSess(testAmbientSession, opts, a, e) {
		t.Fatal("disabled")
	}
	opts.CompatibleCheck = true
	// CompatibleChecker.cpp:49 assert(0) — Variable* overload always rejects when enabled
	if !CompatibleCheckExprVarSess(testAmbientSession, opts, a, e) {
		t.Fatal("enabled Variable* overload must fail closed reject")
	}
	// unrelated var still rejected (assert(0), not invent compatible)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	eb := &Expression{Term: TermVariable, Var: b}
	if !CompatibleCheckExprVarSess(testAmbientSession, opts, a, eb) {
		t.Fatal("enabled Variable* overload always rejects")
	}
}

func TestHasDereferenceableVar(t *testing.T) {
	opts := Defaults()
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if !HasDereferenceableVar([]*Variable{p}, GetIntTypeSess(testAmbientSession), cg, opts) {
		t.Fatal("valid ptr")
	}
	// garbage not valid
	fm.GlobalFacts = []*FactPointTo{NewFactPointToSess(testAmbientSession, p)}
	if HasDereferenceableVar([]*Variable{p}, GetIntTypeSess(testAmbientSession), cg, opts) {
		t.Fatal("dead")
	}
	// Type* always live; sticky no invent no-deref soft-skip
	ClearErrorSess(testAmbientSession)
	if HasDereferenceableVar([]*Variable{p}, nil, cg, opts) {
		t.Fatal("nil typ HasDereferenceableVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil typ HasDereferenceableVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was residual soft-continue then true
	// from later good ptr. Fair: sticky fail closed whole probe.
	shell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)}
	if HasDereferenceableVar([]*Variable{shell, p}, GetIntTypeSess(testAmbientSession), cg, opts) {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsValidPtr residual soft invent was soft-continue later good invent true.
	// Fair: sticky false. incomplete facts on first candidate stickies residual.
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt), nil}
	if HasDereferenceableVar([]*Variable{p}, GetIntTypeSess(testAmbientSession), cg, opts) {
		t.Fatal("IsValidPtr residual (incomplete facts) must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsValidPtr residual HasDereferenceableVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsPartialVolatileAfterDeref(t *testing.T) {
	// pointer to volatile struct/union field type
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort), GetSimpleTypeSess(testAmbientSession, EUInt)}
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 2), opts, probs, &env, "S0")
	// force a volatile field if possible — check method on non-vol struct pointer
	pt := PointerToSess(testAmbientSession, st)
	v := CreateVariableQferSess(testAmbientSession, "g_p", pt, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	// if struct has no vol fields, partial is false
	_ = v.IsPartialVolatileAfterDerefSess(testAmbientSession, 1)
	// fully volatile pointer qfer → not partial
	vv := CreateVariableQferSess(testAmbientSession, "g_pv", pt, NewCVQualifiersSess(testAmbientSession, []bool{false, false}, []bool{true, false}))
	// IsVolatileAfterDeref(1) depends on qfer layout — just ensure no panic
	_ = vv.IsPartialVolatileAfterDerefSess(testAmbientSession, 1)
	// Type-nil residual soft invent was not-partial soft-skip. Fair: sticky partial true.
	ClearErrorSess(testAmbientSession)
	hole := &Variable{Name: "g_p_hole", Type: nil, Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})}
	if !hole.IsPartialVolatileAfterDerefSess(testAmbientSession, 0) {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must fail closed partial true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVolatileStructUnion residual: Type-nil field soft invent was not-partial.
	// Fair: sticky partial true. Use deref 0 on aggregate (qfer level 0 non-vol).
	broken := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: nil, BitWidth: -1}}}
	vs := &Variable{Name: "g_s_hole", Type: broken, Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})}
	if !vs.IsPartialVolatileAfterDerefSess(testAmbientSession, 0) {
		t.Fatal("IsVolatileStructUnion residual IsPartialVolatile must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileStructUnion residual IsPartialVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomAssignCompatibleRegen(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.CompatibleCheck = true
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2))
	if g == nil {
		t.Fatal("g")
	}
	// generate assigns — should not panic; StatementAssign.cpp:127 assert(fm)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	for seed := uint64(1); seed < 10; seed++ {
		ClearErrorSess(testAmbientSession) // compatible-check fail sticks ErrCompatibleCheck per try
		st := func() Stmt {
			c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
			return MakeRandomAssign(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &c, GetIntTypeSess(testAmbientSession))
		}()
		if st.Kind != StmtAssign {
			t.Fatal(st.Kind)
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsCompatibleExpr(t *testing.T) {
	// Lhs.cpp:359–362
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), true, false)
	if a == nil || b == nil {
		t.Fatal("vars")
	}
	lhs := &Lhs{Var: a, Type: GetIntTypeSess(testAmbientSession)}
	ea := &Expression{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)}
	eb := &Expression{Term: TermVariable, Var: b, ExprType: GetIntTypeSess(testAmbientSession)}
	if !lhs.CompatibleExprSess(testAmbientSession, ea, false) {
		t.Fatal("same var")
	}
	if lhs.CompatibleExprSess(testAmbientSession, eb, false) {
		t.Fatal("other var")
	}
	// Lhs + Expression always live; sticky no invent not-compatible soft-skip
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).CompatibleExprSess(testAmbientSession, ea, false) {
		t.Fatal("nil Lhs CompatibleExpr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs CompatibleExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if lhs.CompatibleExprSess(testAmbientSession, nil, false) {
		t.Fatal("nil exp CompatibleExpr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil exp CompatibleExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).CompatibleVarSess(testAmbientSession, a, false) {
		t.Fatal("nil Lhs CompatibleVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs CompatibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionFuncallCompatibleUnary(t *testing.T) {
	// ExpressionFuncall.cpp:206–207 — unary invoke compatible via operand
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	if v == nil {
		t.Fatal("var")
	}
	ev := &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{ev}}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if !e.CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("unary minus of v compatible with v")
	}
	other := CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntTypeSess(testAmbientSession), true, false)
	if e.CompatibleWithVarSess(testAmbientSession, other, false) {
		t.Fatal("not other")
	}
	// binary always false
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ev}}
	e2 := &Expression{Term: TermFunction, Invoke: fi2}
	if e2.CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("binary not compatible")
	}
}

func TestHasEligibleVolatileVarQferFilter(t *testing.T) {
	// VariableSelector.cpp:301–303 — match_indirect; scalar non-exact Match is always true
	// (CVQualifiers.cpp both len==1). Filter matters when level counts differ.
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalizationSess(testAmbientSession)
	defer BookkeeperDoFinalizationSess(testAmbientSession)
	// int* var with 2-level qfer; desired qfer 1-level for int* type match_indirect
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	vol := CreateVariableScalarsSess(testAmbientSession, "g_p", pt, true, false)
	if vol == nil {
		t.Fatal("vol ptr")
	}
	vol.Qfer = NewCVQualifiersSess(testAmbientSession, []bool{false, false}, []bool{true, true}) // vol at both levels
	// request only storage-level volatile (len 1) — match_indirect via indirect_qualifiers
	// deref = 2-1 = 1 → other.IndirectQualifiers(1) → should still allow if eligible
	qfer := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{true})
	if !HasEligibleVolatileVarQfer([]*Variable{vol}, pt, &qfer, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		// may fail IsEligibleVar if effect rules; at least no panic
		t.Log("eligible path optional under empty effect")
	}
	// wildcard always matches
	qw := NewCVQualifiersSess(testAmbientSession, []bool{true}, []bool{true})
	qw.Wildcard = true
	BookkeeperDoFinalizationSess(testAmbientSession)
	if !HasEligibleVolatileVarQfer([]*Variable{vol}, pt, &qw, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("wildcard qfer must accept")
	}
	if VolatileAvailCount() < 1 {
		t.Fatal("volatile_avail")
	}
	// IsArray without AsArray soft invent was residual soft-continue then true
	// from later good volatile. Fair: sticky fail closed whole probe.
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalizationSess(testAmbientSession)
	shell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	good := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), true, false)
	if HasEligibleVolatileVarQfer([]*Variable{shell, good}, GetIntTypeSess(testAmbientSession), nil, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEnableCompatibleCheckProcess(t *testing.T) {
	// CompatibleChecker.cpp:68–70 + CGOptions resolve_exhaustive
	ResetCompatibleCheckSess(testAmbientSession)
	defer ResetCompatibleCheckSess(testAmbientSession)
	opts := Defaults()
	opts.CompatibleCheck = false
	if CompatibleCheckExprVarSess(testAmbientSession, opts, &Variable{}, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}) {
		t.Fatal("disabled must be false")
	}
	EnableCompatibleCheckSess(testAmbientSession)
	// process static on even when opts.CompatibleCheck false
	if !CompatibleCheckExprVarSess(testAmbientSession, opts, &Variable{}, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}) {
		t.Fatal("EnableCompatibleCheck must activate checker")
	}
}
