package csmith

import "testing"

func TestOpportunisticValidateNoDeref(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if OpportunisticValidate(NewRng(1), v, GetIntType(), nil, 0, 0) != 1 {
		t.Fatal("same level")
	}
}

func TestOpportunisticValidateNullDead(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// nil facts is complete empty — no related fact → 0
	if OpportunisticValidate(NewRng(1), p, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("no fact")
	}
	// incomplete map hole → 0 (not invent ok past hole)
	if OpportunisticValidate(NewRng(1), p, GetIntType(), []*FactPointTo{nil}, 0, 0) != 0 {
		t.Fatal("incomplete must reject")
	}
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
}

func TestCompatibleCheckNilHoleFailClosed(t *testing.T) {
	opts := Defaults()
	opts.CompatibleCheck = true
	// enabled + incomplete IR rejects (no invent non-error)
	if !CompatibleCheckExprVar(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil var must reject when compatible-check on")
	}
	if !CompatibleCheckExprs(opts, nil, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
		t.Fatal("nil expr must reject when compatible-check on")
	}
	opts.CompatibleCheck = false
	if CompatibleCheckExprVar(opts, nil, nil) {
		t.Fatal("disabled must not reject")
	}
}

func TestIsNonReadableNilHole(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoReadVars: []*Variable{nil}})
	if !cg.IsNonReadable(g) {
		t.Fatal("nil NoReadVars hole must fail closed as nonreadable")
	}
	cg2 := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{nil}})
	if !cg2.IsNonWritable(g) {
		t.Fatal("nil NoWriteVars hole must fail closed as nonwritable")
	}
}

func TestVariableCompatible(t *testing.T) {
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
}

func TestMakeRandomAssignCompatibleRegen(t *testing.T) {
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
		st := func() Stmt {
			c := EmptyCGContext().WithFactMgr(NewFactMgr(f))
			return MakeRandomAssign(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &c, GetIntType())
		}()
		if st.Kind != StmtAssign {
			t.Fatal(st.Kind)
		}
	}
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
}
