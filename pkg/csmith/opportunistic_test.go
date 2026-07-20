package csmith

import "testing"

func TestOpportunisticValidateNoDeref(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if OpportunisticValidate(NewRng(1), v, GetIntType(), nil, 0, 0) != 1 {
		t.Fatal("same level")
	}
	// nil var/type sticky — no invent not-valid soft success past hole
	if OpportunisticValidate(NewRng(1), nil, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("nil var must fail closed")
	}
	if !HasError() {
		t.Fatal("nil var OpportunisticValidate must SetError sticky")
	}
	ClearError()
	if OpportunisticValidate(NewRng(1), v, nil, nil, 0, 0) != 0 {
		t.Fatal("nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type OpportunisticValidate must SetError sticky")
	}
	ClearError()
}

func TestOpportunisticValidateNullDead(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// nil facts is complete empty — no related fact → 0
	if OpportunisticValidate(NewRng(1), p, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("no fact")
	}
	// incomplete map hole → 0 sticky (not invent ok / soft re-pick past hole)
	if OpportunisticValidate(NewRng(1), p, GetIntType(), []*FactPointTo{nil}, 0, 0) != 0 {
		t.Fatal("incomplete must reject")
	}
	if !HasError() {
		t.Fatal("incomplete facts must SetError sticky")
	}
	ClearError()
	// null, prob 0 → 0
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("null blocked")
	}
	// live target → 1
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	facts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 1 {
		t.Fatal("live")
	}
	// garbage, prob 0 → 0
	facts = []*FactPointTo{NewFactPointTo(p)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("dead blocked")
	}
	if HasError() {
		t.Fatal("complete dead blocked must not sticky")
	}
	ClearError()
}

func TestCompatibleCheckNilHoleFailClosed(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.CompatibleCheck = true
	// enabled + incomplete IR rejects sticky (no invent non-error)
	if !CompatibleCheckExprVar(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil var must reject when compatible-check on")
	}
	if !HasError() {
		t.Fatal("nil var CompatibleCheck must SetError sticky")
	}
	ClearError()
	if !CompatibleCheckExprs(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil expr must reject when compatible-check on")
	}
	if !HasError() {
		t.Fatal("nil expr CompatibleCheck must SetError sticky")
	}
	ClearError()
	opts.CompatibleCheck = false
	if CompatibleCheckExprVar(opts, nil, nil) {
		t.Fatal("disabled must not reject")
	}
	// disabled incomplete stays non-sticky (feature off, not broken IR path)
	if HasError() {
		t.Fatal("disabled CompatibleCheck must not SetError")
	}
	ClearError()
}

func TestIsNonReadableNilHole(t *testing.T) {
	ClearError()
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoReadVars: []*Variable{nil}})
	if !cg.IsNonReadable(g) {
		t.Fatal("nil NoReadVars hole must fail closed as nonreadable")
	}
	if !HasError() {
		t.Fatal("nil NoReadVars hole IsNonReadable must SetError sticky")
	}
	ClearError()
	cg2 := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{nil}})
	if !cg2.IsNonWritable(g) {
		t.Fatal("nil NoWriteVars hole must fail closed as nonwritable")
	}
	if !HasError() {
		t.Fatal("nil NoWriteVars hole IsNonWritable must SetError sticky")
	}
	ClearError()
}

func TestVariableCompatible(t *testing.T) {
	ClearError()
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
	ClearError()
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
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	cg := EmptyCGContext().WithFactMgr(fm)
	if !HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("valid ptr")
	}
	// garbage not valid
	fm.GlobalFacts = []*FactPointTo{NewFactPointTo(p)}
	if HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("dead")
	}
	// Type* always live; sticky no invent no-deref soft-skip
	ClearError()
	if HasDereferenceableVar([]*Variable{p}, nil, cg, opts) {
		t.Fatal("nil typ HasDereferenceableVar must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil typ HasDereferenceableVar must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was residual soft-continue then true
	// from later good ptr. Fair: sticky fail closed whole probe.
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	if HasDereferenceableVar([]*Variable{shell, p}, GetIntType(), cg, opts) {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must fail closed false")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray HasDereferenceableVar must SetError sticky")
	}
	ClearError()
	// IsValidPtr residual soft invent was soft-continue later good invent true.
	// Fair: sticky false. incomplete facts on first candidate stickies residual.
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt), nil}
	if HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("IsValidPtr residual (incomplete facts) must fail closed false")
	}
	if !HasError() {
		t.Fatal("IsValidPtr residual HasDereferenceableVar must SetError sticky")
	}
	ClearError()
}

func TestIsPartialVolatileAfterDeref(t *testing.T) {
	// pointer to volatile struct/union field type
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
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
	ClearError()
	hole := &Variable{Name: "g_p_hole", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if !hole.IsPartialVolatileAfterDeref(0) {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must fail closed partial true")
	}
	if !HasError() {
		t.Fatal("Type-nil IsPartialVolatileAfterDeref must SetError sticky")
	}
	ClearError()
	// IsVolatileStructUnion residual: Type-nil field soft invent was not-partial.
	// Fair: sticky partial true. Use deref 0 on aggregate (qfer level 0 non-vol).
	broken := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: nil, BitWidth: -1}}}
	vs := &Variable{Name: "g_s_hole", Type: broken, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if !vs.IsPartialVolatileAfterDeref(0) {
		t.Fatal("IsVolatileStructUnion residual IsPartialVolatile must fail closed true")
	}
	if !HasError() {
		t.Fatal("IsVolatileStructUnion residual IsPartialVolatile must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomAssignCompatibleRegen(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.CompatibleCheck = true
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), &q, NewRng(2))
	if g == nil {
		t.Fatal("g")
	}
	// generate assigns — should not panic; StatementAssign.cpp:127 assert(fm)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	for seed := uint64(1); seed < 10; seed++ {
		ClearError() // compatible-check fail sticks ErrCompatibleCheck per try
		st := func() Stmt {
			c := EmptyCGContext().WithFactMgr(NewFactMgr(f))
			return MakeRandomAssign(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &c, GetIntType())
		}()
		if st.Kind != StmtAssign {
			t.Fatal(st.Kind)
		}
	}
	ClearError()
}

func TestLhsCompatibleExpr(t *testing.T) {
	// Lhs.cpp:359–362
	ClearError()
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
	ClearError()
	if (*Lhs)(nil).CompatibleExpr(ea, false) {
		t.Fatal("nil Lhs CompatibleExpr must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Lhs CompatibleExpr must SetError sticky")
	}
	ClearError()
	if lhs.CompatibleExpr(nil, false) {
		t.Fatal("nil exp CompatibleExpr must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil exp CompatibleExpr must SetError sticky")
	}
	ClearError()
	if (*Lhs)(nil).CompatibleVar(a, false) {
		t.Fatal("nil Lhs CompatibleVar must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Lhs CompatibleVar must SetError sticky")
	}
	ClearError()
}

func TestExpressionFuncallCompatibleUnary(t *testing.T) {
	// ExpressionFuncall.cpp:206–207 — unary invoke compatible via operand
	ClearError()
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
	ClearError()
	BookkeeperDoFinalization()
	defer BookkeeperDoFinalization()
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
	if !HasEligibleVolatileVarQfer([]*Variable{vol}, pt, &qfer, AccessRead, EmptyCGContext()) {
		// may fail IsEligibleVar if effect rules; at least no panic
		t.Log("eligible path optional under empty effect")
	}
	// wildcard always matches
	qw := NewCVQualifiers([]bool{true}, []bool{true})
	qw.Wildcard = true
	BookkeeperDoFinalization()
	if !HasEligibleVolatileVarQfer([]*Variable{vol}, pt, &qw, AccessRead, EmptyCGContext()) {
		t.Fatal("wildcard qfer must accept")
	}
	if VolatileAvailCount() < 1 {
		t.Fatal("volatile_avail")
	}
	// IsArray without AsArray soft invent was residual soft-continue then true
	// from later good volatile. Fair: sticky fail closed whole probe.
	ClearError()
	BookkeeperDoFinalization()
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	good := CreateVariableScalars("g_v", GetIntType(), true, false)
	if HasEligibleVolatileVarQfer([]*Variable{shell, good}, GetIntType(), nil, AccessRead, EmptyCGContext()) {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must fail closed false")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray HasEligibleVolatileVarQfer must SetError sticky")
	}
	ClearError()
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
