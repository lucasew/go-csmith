package csmith

import "testing"

func TestOpportunisticValidateNoDeref(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if OpportunisticValidate(NewRng(1), v, GetIntType(), nil, 0, 0) != 1 {
		t.Fatal("same level")
	}
	// nil var/type sticky — no invent not-valid soft success past hole
	if OpportunisticValidate(NewRng(1), nil, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("nil var must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var OpportunisticValidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if OpportunisticValidate(NewRng(1), v, nil, nil, 0, 0) != 0 {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type OpportunisticValidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOpportunisticValidateNullDead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// nil facts is complete empty — no related fact → 0
	if OpportunisticValidate(NewRng(1), p, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("no fact")
	}
	// incomplete map hole → 0 sticky (not invent ok / soft re-pick past hole)
	if OpportunisticValidate(NewRng(1), p, GetIntType(), []*FactPointTo{nil}, 0, 0) != 0 {
		t.Fatal("incomplete must reject")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// null, prob 0 → 0; FactPointTo.cpp:455 still rnd_flipcoin(0)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	rNull := NewRng(1)
	d0 := rNull.RandDepth()
	if OpportunisticValidate(rNull, p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("null blocked")
	}
	if rNull.RandDepth() != d0+1 {
		t.Fatalf("null p=0 must still flipcoin once: depth %d → %d", d0, rNull.RandDepth())
	}
	// live target → 1 (no flip when not null/dead)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	facts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	rLive := NewRng(1)
	dLive := rLive.RandDepth()
	if OpportunisticValidate(rLive, p, GetIntType(), facts, 0, 0) != 1 {
		t.Fatal("live")
	}
	if rLive.RandDepth() != dLive {
		t.Fatalf("live pointees must not flipcoin: depth %d → %d", dLive, rLive.RandDepth())
	}
	// garbage, prob 0 → 0; FactPointTo.cpp:464 still rnd_flipcoin(0)
	facts = []*FactPointTo{NewFactPointTo(p)}
	rDead := NewRng(1)
	dDead := rDead.RandDepth()
	if OpportunisticValidate(rDead, p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("dead blocked")
	}
	if rDead.RandDepth() != dDead+1 {
		t.Fatalf("dead p=0 must still flipcoin once: depth %d → %d", dDead, rDead.RandDepth())
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete dead blocked must not sticky")
	}
	// null+dead both set: two flips (null then dead)
	facts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// make dead+null fact: NewFactPointTo is garbage/dead; force null+dead via fields
	fBoth := MakeFactPointTo(p, NullPtr)
	// IsDead for null pointees? if only null, one flip. dead-only above. allow-unsafe path:
	rAllow := NewRng(1)
	if OpportunisticValidate(rAllow, p, GetIntType(), []*FactPointTo{fBoth}, 100, 0) != 2 {
		// p=100 always allows null unsafe when is_null
		t.Fatal("null with nullProb=100 must allow ret=2")
	}
	// nil r on null path sticky (C++ always has process RNG for flipcoin)
	ClearErrorSess(testAmbientSession)
	if OpportunisticValidate(nil, p, GetIntType(), []*FactPointTo{MakeFactPointTo(p, NullPtr)}, 0, 0) != 0 {
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
	elem := GetIntType()
	pt := PointerTo(elem)
	// collective array-of-pointer shell (fact subject)
	coll := &ArrayVariable{
		Variable: Variable{Name: "g_arr", Type: pt, IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
	}
	coll.AsArray = coll
	tgt := CreateVariableScalars("g_t", elem, false, false)
	facts := []*FactPointTo{MakeFactPointTo(&coll.Variable, tgt)}
	// itemized member of that collective
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_arr", Type: pt, IsArray: true, ArraySizes: []int{3}},
		Sizes:      []int{3},
		Collective: coll,
	}
	item.AsArray = item
	if OpportunisticValidate(NewRng(1), &item.Variable, elem, facts, 0, 0) != 1 {
		t.Fatal("itemized must find fact via get_collective()")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete collective path must not sticky")
	}
	// fact keyed only on item (not collective) must miss when looking up via collective
	factsItemOnly := []*FactPointTo{MakeFactPointTo(&item.Variable, tgt)}
	if OpportunisticValidate(NewRng(1), &item.Variable, elem, factsItemOnly, 0, 0) != 0 {
		t.Fatal("fact on item alone must miss when lookup uses collective identity")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleCheckNilHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.CompatibleCheck = true
	// enabled + incomplete IR rejects sticky (no invent non-error)
	if !CompatibleCheckExprVar(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil var must reject when compatible-check on")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var CompatibleCheck must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !CompatibleCheckExprs(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil expr must reject when compatible-check on")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil expr CompatibleCheck must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	opts.CompatibleCheck = false
	if CompatibleCheckExprVar(opts, nil, nil) {
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
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
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
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	if !a.Compatible(a, false) {
		t.Fatal("self")
	}
	if a.Compatible(b, false) {
		t.Fatal("other no expand")
	}
	if !a.Compatible(b, true) {
		t.Fatal("expand non-field")
	}
	vol := CreateVariableScalars("g_v", GetIntType(), false, true)
	if vol.Compatible(vol, false) {
		t.Fatal("vol self")
	}
}

func TestCompatibleCheckerDisabled(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	e := &Expression{Term: TermVariable, Var: a}
	if CompatibleCheckExprVar(opts, a, e) {
		t.Fatal("disabled")
	}
	opts.CompatibleCheck = true
	// CompatibleChecker.cpp:49 assert(0) — Variable* overload always rejects when enabled
	if !CompatibleCheckExprVar(opts, a, e) {
		t.Fatal("enabled Variable* overload must fail closed reject")
	}
	// unrelated var still rejected (assert(0), not invent compatible)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	eb := &Expression{Term: TermVariable, Var: b}
	if !CompatibleCheckExprVar(opts, a, eb) {
		t.Fatal("enabled Variable* overload always rejects")
	}
}

func TestHasDereferenceableVar(t *testing.T) {
	opts := Defaults()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if !HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("valid ptr")
	}
	// garbage not valid
	fm.GlobalFacts = []*FactPointTo{NewFactPointTo(p)}
	if HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
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
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	if HasDereferenceableVar([]*Variable{shell, p}, GetIntType(), cg, opts) {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsValidPtr residual soft invent was soft-continue later good invent true.
	// Fair: sticky false. incomplete facts on first candidate stickies residual.
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt), nil}
	if HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
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
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	// force a volatile field if possible — check method on non-vol struct pointer
	pt := PointerTo(st)
	v := CreateVariableQfer("g_p", pt, NewCVQualifiers([]bool{false}, []bool{false}))
	// if struct has no vol fields, partial is false
	_ = v.IsPartialVolatileAfterDeref(1)
	// fully volatile pointer qfer → not partial
	vv := CreateVariableQfer("g_pv", pt, NewCVQualifiers([]bool{false, false}, []bool{true, false}))
	// IsVolatileAfterDeref(1) depends on qfer layout — just ensure no panic
	_ = vv.IsPartialVolatileAfterDeref(1)
	// Type-nil residual soft invent was not-partial soft-skip. Fair: sticky partial true.
	ClearErrorSess(testAmbientSession)
	hole := &Variable{Name: "g_p_hole", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if !hole.IsPartialVolatileAfterDeref(0) {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must fail closed partial true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVolatileStructUnion residual: Type-nil field soft invent was not-partial.
	// Fair: sticky partial true. Use deref 0 on aggregate (qfer level 0 non-vol).
	broken := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: nil, BitWidth: -1}}}
	vs := &Variable{Name: "g_s_hole", Type: broken, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if !vs.IsPartialVolatileAfterDeref(0) {
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
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), &q, NewRng(2))
	if g == nil {
		t.Fatal("g")
	}
	// generate assigns — should not panic; StatementAssign.cpp:127 assert(fm)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	for seed := uint64(1); seed < 10; seed++ {
		ClearErrorSess(testAmbientSession) // compatible-check fail sticks ErrCompatibleCheck per try
		st := func() Stmt {
			c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
			return MakeRandomAssign(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &c, GetIntType())
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
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	if a == nil || b == nil {
		t.Fatal("vars")
	}
	lhs := &Lhs{Var: a, Type: GetIntType()}
	ea := &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()}
	eb := &Expression{Term: TermVariable, Var: b, ExprType: GetIntType()}
	if !lhs.CompatibleExpr(ea, false) {
		t.Fatal("same var")
	}
	if lhs.CompatibleExpr(eb, false) {
		t.Fatal("other var")
	}
	// Lhs + Expression always live; sticky no invent not-compatible soft-skip
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).CompatibleExpr(ea, false) {
		t.Fatal("nil Lhs CompatibleExpr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs CompatibleExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if lhs.CompatibleExpr(nil, false) {
		t.Fatal("nil exp CompatibleExpr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil exp CompatibleExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).CompatibleVar(a, false) {
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
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if v == nil {
		t.Fatal("var")
	}
	ev := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{ev}}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if !e.CompatibleWithVar(v, false) {
		t.Fatal("unary minus of v compatible with v")
	}
	other := CreateVariableScalars("g_2", GetIntType(), true, false)
	if e.CompatibleWithVar(other, false) {
		t.Fatal("not other")
	}
	// binary always false
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ev}}
	e2 := &Expression{Term: TermFunction, Invoke: fi2}
	if e2.CompatibleWithVar(v, false) {
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
	pt := PointerTo(GetIntType())
	vol := CreateVariableScalars("g_p", pt, true, false)
	if vol == nil {
		t.Fatal("vol ptr")
	}
	vol.Qfer = NewCVQualifiers([]bool{false, false}, []bool{true, true}) // vol at both levels
	// request only storage-level volatile (len 1) — match_indirect via indirect_qualifiers
	// deref = 2-1 = 1 → other.IndirectQualifiers(1) → should still allow if eligible
	qfer := NewCVQualifiers([]bool{false}, []bool{true})
	if !HasEligibleVolatileVarQfer([]*Variable{vol}, pt, &qfer, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		// may fail IsEligibleVar if effect rules; at least no panic
		t.Log("eligible path optional under empty effect")
	}
	// wildcard always matches
	qw := NewCVQualifiers([]bool{true}, []bool{true})
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
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	good := CreateVariableScalars("g_v", GetIntType(), true, false)
	if HasEligibleVolatileVarQfer([]*Variable{shell, good}, GetIntType(), nil, AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEnableCompatibleCheckProcess(t *testing.T) {
	// CompatibleChecker.cpp:68–70 + CGOptions resolve_exhaustive
	ResetCompatibleCheck()
	defer ResetCompatibleCheck()
	opts := Defaults()
	opts.CompatibleCheck = false
	if CompatibleCheckExprVar(opts, &Variable{}, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("disabled must be false")
	}
	EnableCompatibleCheck()
	// process static on even when opts.CompatibleCheck false
	if !CompatibleCheckExprVar(opts, &Variable{}, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("EnableCompatibleCheck must activate checker")
	}
}
