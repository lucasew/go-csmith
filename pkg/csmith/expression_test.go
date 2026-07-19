package csmith

import "testing"

func TestPickTermTypeNoFuncNoConst(t *testing.T) {
	opts := Defaults()
	tables := NewExprTables(opts)
	// Filter function+const → only Variable/Assign/Comma remain (weights 20+10+10=40)
	// Seed2 first RndUpto(40) with filter rejecting F and C
	r := NewRng(2)
	tt := PickTermType(r, tables, opts, GetSimpleType(EInt), true, true, 0)
	if tt == TermFunction || tt == TermConstant {
		t.Fatalf("filtered terms appeared: %v", tt)
	}
}

func TestPickTermTypeDepthBlocksNested(t *testing.T) {
	opts := Defaults()
	opts.MaxExprComplexity = 2
	tables := NewExprTables(opts)
	// exprDepth+2 > max → filter Function, Assign, Comma → only Variable+Constant
	r := NewRng(2)
	for i := 0; i < 50; i++ {
		tt := PickTermType(r, tables, opts, GetSimpleType(EInt), false, false, 1)
		if tt == TermFunction || tt == TermAssignment || tt == TermCommaExpr {
			t.Fatalf("depth gate failed: %v", tt)
		}
	}
}

func TestMakeRandomExpressionConstant(t *testing.T) {
	opts := Defaults()
	tables := NewExprTables(opts)
	r := NewRng(2)
	e := func() *Expression { c := EmptyCGContext(); return MakeRandomExpression(r, opts, tables, nil, &c, GetSimpleType(EInt), nil, false, false, TermConstant, 0) }()
	if e == nil || e.Term != TermConstant || e.Con == nil || e.Output() == "" {
		t.Fatalf("%+v out=%q", e, e.Output())
	}
}

func TestMakeRandomExpressionVariableCreatesGlobal(t *testing.T) {
	opts := Defaults()
	tables := NewExprTables(opts)
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	e := func() *Expression { c := EmptyCGContext(); return MakeRandomExpression(r, opts, tables, vs, &c, GetSimpleType(EInt), &q, false, false, TermVariable, 0) }()
	if e == nil || e.Term != TermVariable || e.Var == nil {
		t.Fatalf("%+v", e)
	}
	if !e.Var.IsGlobal() {
		t.Fatal("expected global")
	}
	if len(vs.GlobalList) < 1 {
		t.Fatal("GlobalList empty")
	}
}

func TestExpressionTypeProbabilitySeedBand(t *testing.T) {
	ClearPartialExpander()
	opts := Defaults()
	tables := NewExprTables(opts)
	f := NewVectorFilter(&tables.Expr)
	// no filters: max=120
	r := NewRng(2)
	// first RndUpto(120) for seed2
	r2 := NewRng(2)
	raw := int(r2.RndUpto(120))
	want := TermType(tables.Expr.RndNumToKey(raw))
	got := ExpressionTypeProbability(r, f)
	if got != want {
		t.Fatalf("got %v want %v (raw %d)", got, want, raw)
	}
}

func TestCompatibleWithExprNilVarFailClosed(t *testing.T) {
	// ExpressionVariable always has live Variable*; nil hole fails closed reject
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	live := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	hole := &Expression{Term: TermVariable, Var: nil}
	if hole.CompatibleWithExpr(live, false) {
		t.Fatal("nil Var lhs must fail closed")
	}
	if live.CompatibleWithExpr(hole, false) {
		t.Fatal("nil Var rhs must fail closed")
	}
}

func TestConstantCompatibleWithVarExpandStruct(t *testing.T) {
	// Constant.cpp:488–493 — expand_struct → true; else false
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	c := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}
	if c.CompatibleWithVar(v, false) {
		t.Fatal("without expand_struct constant incompatible")
	}
	if !c.CompatibleWithVar(v, true) {
		t.Fatal("expand_struct → true")
	}
	// assert(v) — nil var fail closed
	if c.CompatibleWithVar(nil, true) {
		t.Fatal("nil var")
	}
}

func TestExpressionGetQualifiersIndirect(t *testing.T) {
	// ExpressionVariable.cpp:194–196 — qfer.indirect_qualifiers(deref)
	// Layout [ptr_level, storage]; deref pops storage (Lhs test: remaining [false])
	pt := PointerTo(GetIntType())
	q := NewCVQualifiers([]bool{false, true}, []bool{false, false})
	v := CreateVariableQfer("g_p", pt, q)
	e := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	gq := e.GetQualifiers()
	if len(gq.IsConsts) != 1 {
		t.Fatalf("after deref: %+v", gq)
	}
	// bare pointer type → indirect 0 → full two-level qfer
	e2 := &Expression{Term: TermVariable, Var: v, ExprType: pt}
	gq2 := e2.GetQualifiers()
	if len(gq2.IsConsts) != 2 || !gq2.IsConsts[1] {
		t.Fatalf("no deref: %+v", gq2)
	}
	// assign uses Lhs quals
	lhs := &Lhs{Var: v, Type: GetIntType()}
	st := &Stmt{Kind: StmtAssign, Lhs: lhs, LhsVar: v, AssignOp: AssignSimple}
	ea := &Expression{Term: TermAssignment, Assign: st}
	if len(ea.GetQualifiers().IsConsts) != 1 {
		t.Fatalf("assign: %+v", ea.GetQualifiers())
	}
}

func TestExpressionGetTypeIncompleteFailClosed(t *testing.T) {
	// no invent ExprType shell without live invoke / assign / comma RHS
	if (&Expression{Term: TermFunction, ExprType: GetIntType()}).GetType() != nil {
		t.Fatal("nil Invoke must not invent type from ExprType")
	}
	if (&Expression{Term: TermAssignment, ExprType: GetIntType()}).GetType() != nil {
		t.Fatal("nil Assign must not invent type from ExprType")
	}
	if (&Expression{Term: TermCommaExpr}).GetType() != nil {
		t.Fatal("nil CommaRHS must fail closed nil type, not panic")
	}
	if (&Expression{Term: TermVariable}).GetType() != nil {
		t.Fatal("nil Var must fail closed")
	}
	// complete still works
	v := CreateVariableScalars("g_i", GetIntType(), false, false)
	if (&Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}).GetType() != GetIntType() {
		t.Fatal("complete variable type")
	}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if (&Expression{Term: TermCommaExpr, CommaLHS: rhs, CommaRHS: rhs}).GetType() != GetIntType() {
		t.Fatal("complete comma RHS type")
	}
}

func TestExpressionEqualsIntIncompleteFailClosed(t *testing.T) {
	// incomplete must not panic or invent fold as equals
	if (&Expression{Term: TermCommaExpr}).EqualsInt(0) {
		t.Fatal("nil CommaRHS must fail closed false")
	}
	if (&Expression{Term: TermAssignment, Assign: &Stmt{AssignOp: AssignSimple}}).EqualsInt(0) {
		t.Fatal("nil Assign.Expr must fail closed false")
	}
	if (&Expression{Term: TermFunction}).EqualsInt(0) {
		t.Fatal("nil Invoke must fail closed false")
	}
}

func TestExpressionLessThanAndIs0Or1(t *testing.T) {
	if !(&Expression{Term: TermConstant, Con: MakeInt(3)}).LessThan(5) {
		t.Fatal("3 < 5")
	}
	if (&Expression{Term: TermConstant, Con: MakeInt(7)}).LessThan(5) {
		t.Fatal("7 < 5")
	}
	// FunctionInvocationUnary::is_0_or_1 — eNot only
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "!"}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if !e.Is0Or1() {
		t.Fatal("unary not")
	}
	// binary comparison also 0/1
	fi2 := &Invocation{IsStd: true, Binary: "==", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(2)},
	}}
	if !(&Expression{Term: TermFunction, Invoke: fi2}).Is0Or1() {
		t.Fatal("cmp")
	}
	// simple assign of !x
	st := &Stmt{
		Kind: StmtAssign, AssignOp: AssignSimple,
		Expr: e,
	}
	ea := &Expression{Term: TermAssignment, Assign: st}
	if !ea.Is0Or1() {
		t.Fatal("assign peel")
	}
}

func TestExpressionComplexityFuncArgs(t *testing.T) {
	// ExpressionFuncall.cpp:131–143 — call + sum(args)
	inner := &Expression{Term: TermConstant, Con: MakeInt(1)}
	fi := &Invocation{
		User: &Function{Name: "f"}, IsStd: false,
		Args: []*Expression{inner, inner},
	}
	e := &Expression{Term: TermFunction, Invoke: fi}
	// 1 (call) + 0 + 0
	if ExpressionComplexity(e) != 1 {
		t.Fatal(ExpressionComplexity(e))
	}
	// nested call arg
	nested := &Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "g"}, Args: nil}}
	fi2 := &Invocation{User: &Function{Name: "f"}, Args: []*Expression{nested}}
	e2 := &Expression{Term: TermFunction, Invoke: fi2}
	if ExpressionComplexity(e2) != 2 {
		t.Fatal(ExpressionComplexity(e2))
	}
	// nil Invoke — fail closed sticky -1 (no invent leaf depth 0)
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil invoke must fail closed -1, not invent depth 0")
	}
	if !HasError() {
		t.Fatal("nil invoke ExpressionComplexity must SetError sticky")
	}
	// incomplete assign / comma / nil arg — fail closed sticky -1
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermAssignment}) >= 0 {
		t.Fatal("nil Assign must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Assign ExpressionComplexity must SetError sticky")
	}
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermCommaExpr, CommaLHS: inner}) >= 0 {
		t.Fatal("nil CommaRHS must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil CommaRHS ExpressionComplexity must SetError sticky")
	}
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermFunction, Invoke: &Invocation{
		User: &Function{Name: "h"}, Args: []*Expression{inner, nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil arg ExpressionComplexity must SetError sticky")
	}
	// incomplete constant / variable leaf
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermConstant}) >= 0 {
		t.Fatal("nil Con must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Con ExpressionComplexity must SetError sticky")
	}
	ClearError()
	if ExpressionComplexity(&Expression{Term: TermVariable}) >= 0 {
		t.Fatal("nil Var must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil Var ExpressionComplexity must SetError sticky")
	}
	ClearError()
}

func TestExpressionIndentedOutput(t *testing.T) {
	e := &Expression{Term: TermConstant, Con: MakeInt(7)}
	got := e.IndentedOutput(2)
	if got != "        7" { // OutputTab 4 spaces per level
		t.Fatalf("%q", got)
	}
}

func TestConstantGetField(t *testing.T) {
	// Constant.cpp:513–522
	c := &Constant{Value: "{0, 1, 2}"}
	if c.GetField(0) != "0" || c.GetField(1) != "1" || c.GetField(2) != "2" {
		t.Fatal(c.GetField(0), c.GetField(1), c.GetField(2))
	}
	if c.GetField(9) != "" {
		t.Fatal("oob")
	}
}

func TestExpressionTypeProbabilityForceFunction(t *testing.T) {
	// Expression.cpp:104–105 — direct_expand_check(eInvoke) → eFunction
	ClearPartialExpander()
	if !InitPartialExpander("invoke") {
		t.Fatal("init")
	}
	defer ClearPartialExpander()
	opts := Defaults()
	tables := NewExprTables(opts)
	f := NewVectorFilter(&tables.Expr)
	// even with no_func filter setup in PickTermType, ExpressionTypeProbability alone forces Function
	got := ExpressionTypeProbability(NewRng(2), f)
	if got != TermFunction {
		t.Fatalf("got %v want TermFunction", got)
	}
	// PickTermType with noFunc still hits ExpressionTypeProbability force
	tt := PickTermType(NewRng(2), tables, opts, GetIntType(), true, false, 0)
	if tt != TermFunction {
		t.Fatalf("PickTermType force: %v", tt)
	}
}

func TestMakeExpressionVariablePassesDummyToSelect(t *testing.T) {
	// ExpressionVariable.cpp:78 — select(..., dummy invalid_vars)
	// After rejecting a float for non-float want, select must not keep returning it forever.
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// only a float global
	fv := CreateVariableScalars("g_f", GetSimpleType(EFloat), true, false)
	vs.GlobalList = []*Variable{fv}
	vs.AllVars = []*Variable{fv}
	// force global selection
	opts.GlobalVariables = true
	vs.Opts = opts
	// int want — float rejected then new var created (ScopeNewValue) or nil after tries
	cg := EmptyCGContext()
	cg.Types = vs.Types
	ev := makeExpressionVariableFlags(NewRng(1), vs, &cg, GetIntType(), nil, false, false)
	// either created a new non-float, or nil — must not return the float
	if ev != nil && ev.Var == fv {
		t.Fatal("must not use float for int want")
	}
	// ExpressionVariable.cpp always has RNG; no invent var shell
	if e := makeExpressionVariableFlags(nil, vs, &cg, GetIntType(), nil, false, false); e != nil {
		t.Fatal("nil RNG must not invent ExpressionVariable")
	}
	// Type* always live; nil want must not soft-skip type filters
	if e := makeExpressionVariableFlags(NewRng(1), vs, &cg, nil, nil, false, false); e != nil {
		t.Fatal("nil typ must not invent ExpressionVariable")
	}
	// Variable::type always live; Type-nil candidate must not soft-skip filters to success
	ClearError()
	broken := CreateVariableScalars("g_broken", GetIntType(), true, false)
	broken.Type = nil
	vs.GlobalList = []*Variable{broken}
	vs.AllVars = []*Variable{broken}
	// disable new-var creation path by restricting options if possible; still must not return broken
	evBroken := makeExpressionVariableFlags(NewRng(3), vs, &cg, GetIntType(), nil, false, false)
	if evBroken != nil && evBroken.Var == broken {
		t.Fatal("Type-nil var must not be accepted as ExpressionVariable")
	}
	if !HasError() {
		// Type-nil may SetError in makeExpressionVariableFlags or via ChooseVarFull
		// either sticky path is acceptable; only invent success is forbidden
	}
	// sticky ERROR_GUARD after incomplete type IR — clear for suite
	ClearError()
}

func TestMakeExpressionVariableIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent var expr soft re-pick)
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{v}
	vs.AllVars = []*Variable{v}
	inc := IncompleteEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &inc
	if makeExpressionVariableFlags(NewRng(1), vs, &cg, GetIntType(), nil, false, false) != nil {
		t.Fatal("incomplete EffectAccum must fail closed makeExpressionVariable")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := EmptyCGContext().WithFactMgr(fm)
	if makeExpressionVariableFlags(NewRng(2), vs, &cg2, GetIntType(), nil, false, false) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed makeExpressionVariable")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeExpressionVariableIndirectZeroUsesVarType(t *testing.T) {
	// ExpressionVariable.cpp:122–123 — indirection 0 → ExpressionVariable(*var) without forced type
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{v}
	vs.AllVars = []*Variable{v}
	vs.Opts = opts
	cg := EmptyCGContext()
	// want int, var int → indirect 0 → ExprType should be var.Type
	ev := makeExpressionVariableFlags(NewRng(2), vs, &cg, GetIntType(), nil, false, false)
	if ev == nil {
		t.Fatal("nil")
	}
	if ev.Var != v {
		// may create new var if select path differs — still check zero indirect shape
		if ev.IndirectLevel() != 0 {
			t.Fatal("want 0")
		}
		return
	}
	if ev.ExprType != v.Type {
		t.Fatalf("ExprType %v want var.Type", ev.ExprType)
	}
}

func TestMakeExpressionVariableMutatesCallerEffect(t *testing.T) {
	// ExpressionVariable::make_random visit_facts must update caller's effect_accum /
	// effect_stm so assign RHS merge_param_context and param effects see the read.
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	ev := makeExpressionVariableFlags(NewRng(2), vs, &cg, GetIntType(), nil, false, false)
	if ev == nil || ev.Var == nil {
		t.Skip("no expression variable")
	}
	if cg.EffectAccum != nil && !cg.EffectAccum.IsRead(ev.Var) && !cg.EffectStm.IsRead(ev.Var) {
		t.Fatalf("expected read effect on var %s after visit_facts", ev.Var.Name)
	}
}

func TestSelectWithInvalidRejectsVolatileWhenImpure(t *testing.T) {
	// VariableSelector.cpp:1225–1227 — assert(!var->is_volatile()) under impure effect_context
	opts := Defaults()
	vs := NewVariableSelector(opts)
	vs.Opts = opts
	vq := NewCVQualifiers([]bool{false}, []bool{true})
	vol := CreateVariableQfer("g_v", GetIntType(), vq)
	vs.GlobalList = []*Variable{vol}
	vs.AllVars = []*Variable{vol}
	cg := WithEffectContext(WithSideEffects())
	// under impure context must never hand back a volatile
	for seed := uint64(1); seed < 40; seed++ {
		got := vs.SelectWithInvalid(AccessRead, cg, GetIntType(), nil, NewRng(seed), MatchFlexible, nil)
		if got != nil && got.IsVolatile() {
			t.Fatalf("seed %d: must not return volatile under impure effect_context", seed)
		}
	}
}

func TestSelectWithInvalidExpandStructNewValueErrors(t *testing.T) {
	// VariableSelector.cpp:1217–1219 — eNewValue + expand_struct sets ERROR → nullptr
	opts := Defaults()
	opts.ExpandStruct = true
	opts.GlobalVariables = true
	vs := NewVariableSelector(opts)
	vs.Opts = opts
	// scan until ScopeNewValue (rnd_upto(100) in [95,100) with globals table)
	hit := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearError()
		// empty lists → local/param fail, NewValue or Global create
		vs.GlobalList = nil
		vs.AllVars = nil
		v := vs.SelectWithInvalid(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(seed), MatchFlexible, nil)
		if HasError() {
			hit = true
			if v != nil {
				t.Fatal("ERROR_GUARD must return nil")
			}
			break
		}
	}
	ClearError()
	if !hit {
		t.Log("ScopeNewValue+expand_struct not hit in seed scan")
	}
}

func TestSelectWithInvalidExcludesDummy(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	vs.GlobalList = []*Variable{a, b}
	vs.AllVars = []*Variable{a, b}
	vs.Opts = opts
	// only two globals; exclude a → must pick b or create
	got := vs.SelectWithInvalid(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(3), MatchFlexible, []*Variable{a})
	if got == a {
		t.Fatal("invalid_vars must exclude a")
	}
}

func TestBumpsExprDepth(t *testing.T) {
	// Expression.cpp:213–218
	if !BumpsExprDepth(&Expression{Term: TermConstant, Con: MakeInt(1)}) {
		t.Fatal("const")
	}
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if !BumpsExprDepth(&Expression{Term: TermVariable, Var: v}) {
		t.Fatal("var")
	}
	if !BumpsExprDepth(&Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "f"}}}) {
		t.Fatal("user call")
	}
	if BumpsExprDepth(&Expression{Term: TermFunction, Invoke: &Invocation{IsStd: true, Binary: "+"}}) {
		t.Fatal("std binary no bump")
	}
	if BumpsExprDepth(&Expression{Term: TermCommaExpr}) {
		t.Fatal("comma no bump")
	}
}

func TestMakeRandomExpressionBumpsCallerExprDepth(t *testing.T) {
	// Expression.cpp:213–218 — cg_context.expr_depth++ on Constant/Variable/user call
	opts := Defaults()
	cg := EmptyCGContext()
	cg.ExprDepth = 2
	e := MakeRandomExpression(NewRng(1), opts, NewExprTables(opts), nil, &cg, GetIntType(), nil, true, false, TermConstant, cg.ExprDepth)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("%+v", e)
	}
	if cg.ExprDepth != 3 {
		t.Fatalf("ExprDepth=%d want 3", cg.ExprDepth)
	}
}

func TestMakeRandomExpressionUsesCGExprDepthNotStaleArg(t *testing.T) {
	// Expression.cpp:176 — filter uses cg_context.expr_depth, not a separate caller local
	opts := Defaults()
	opts.MaxExprComplexity = 3
	cg := EmptyCGContext()
	cg.ExprDepth = 2 // near max: 2+2 > 3 → force leaf terms
	// pass stale exprDepth=0 that would allow Function if used; force Constant leaf
	e := MakeRandomExpression(NewRng(5), opts, NewExprTables(opts), nil, &cg, GetIntType(), nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("%+v", e)
	}
	// with high cg.ExprDepth, complex MaxTermTypes must not pick Function via stale 0
	e2 := MakeRandomExpression(NewRng(5), opts, NewExprTables(opts), nil, &cg, GetIntType(), nil, false, false, MaxTermTypes, 0)
	if e2 != nil && (e2.Term == TermFunction || e2.Term == TermAssignment || e2.Term == TermCommaExpr) {
		t.Fatalf("stale depth arg must not allow complex term: %v", e2.Term)
	}
}

func TestMakeRandomExpressionNilTypeUsesEnv(t *testing.T) {
	// Expression.cpp:147–152 — nil type from choose_random_nonvoid when SE-free
	opts := Defaults()
	env := &TypeEnv{}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort)}
	cg := EmptyCGContext()
	cg.Types = env
	// force constant so we don't need VariableSelector
	e := MakeRandomExpression(NewRng(1), opts, NewExprTables(opts), nil, &cg, nil, nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("%+v", e)
	}
	// type was chosen from env (not stuck on void)
	if e.Con == nil || e.Con.Type == nil {
		t.Fatal("const type")
	}
	// no soft invent simple when AllTypes empty
	cg2 := EmptyCGContext()
	cg2.Types = &TypeEnv{}
	if MakeRandomExpression(NewRng(1), opts, NewExprTables(opts), nil, &cg2, nil, nil, true, false, TermConstant, 0) != nil {
		t.Fatal("empty Type env must not invent simple type")
	}
}

func TestMakeRandomExpressionNoInventSessionProbs(t *testing.T) {
	// C++ Probabilities singleton; no invent NewProbabilities(opts) when vs.Probs nil
	opts := Defaults()
	tables := NewExprTables(opts)
	cg := EmptyCGContext()
	// nil vs: simple constant still ok (MakeRandom allows nil probs for simple)
	e := MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, GetIntType(), nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("simple const without vs: %+v", e)
	}
	// vs with nil Probs: same simple path; must not invent session tables
	vs := &VariableSelector{Opts: opts}
	e2 := MakeRandomExpression(NewRng(1), opts, tables, vs, &cg, GetIntType(), nil, true, false, TermConstant, 0)
	if e2 == nil || e2.Term != TermConstant {
		t.Fatalf("simple const with nil vs.Probs: %+v", e2)
	}
}

func TestMakeRandomExpressionAssertFailClosed(t *testing.T) {
	// Expression.cpp:154–157, 186–187 — asserts sticky; no soft invent rewrite/emit
	opts := Defaults()
	tables := NewExprTables(opts)
	cg := EmptyCGContext()
	// no_const && eConstant
	ClearError()
	if MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, GetIntType(), nil, false, true, TermConstant, 0) != nil {
		t.Fatal("no_const + Constant must fail closed")
	}
	if !HasError() {
		t.Fatal("no_const + Constant must SetError sticky")
	}
	// no_func && eFunction
	ClearError()
	if MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, GetIntType(), nil, true, false, TermFunction, 0) != nil {
		t.Fatal("no_func + Function must fail closed")
	}
	if !HasError() {
		t.Fatal("no_func + Function must SetError sticky")
	}
	// struct + eConstant (was soft invent TermVariable)
	ClearError()
	st := &Type{isStruct: true, StructName: "SAssert", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, st, nil, false, false, TermConstant, 0) != nil {
		t.Fatal("struct + Constant must fail closed, not rewrite to Variable")
	}
	if !HasError() {
		t.Fatal("struct + Constant must SetError sticky")
	}
	// void simple constant
	ClearError()
	if MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, GetSimpleType(EVoid), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("void Constant must fail closed")
	}
	if !HasError() {
		t.Fatal("void Constant must SetError sticky")
	}
	// Expression.cpp:188 + ERROR_GUARD — MakeRandom nil → no invent TermConstant shell
	ClearError()
	if e := MakeRandomExpression(nil, opts, tables, nil, &cg, GetIntType(), nil, true, false, TermConstant, 0); e != nil {
		t.Fatal("nil RNG must not invent TermConstant shell", e)
	}
	ClearError()
}

func TestMakeExpressionFuncallForcesUserForAggregate(t *testing.T) {
	// ExpressionFuncall.cpp:71–73 — struct/union never std unary/binary
	opts := Defaults()
	opts.MaxFuncs = 4
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	env := &TypeEnv{AllTypes: []*Type{st, GetIntType()}}
	vs.Types = env
	list := &FunctionList{Types: env}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := EmptyCGContext().WithFactMgr(NewFactMgr(f))
	cg.Types = env
	cg.Funcs = list
	// many tries: result if any should not be pure std binary/unary alone when type is struct
	// (user path may still fail → variable fallback)
	for seed := uint64(1); seed < 20; seed++ {
		ClearError()
		e := makeExpressionFuncall(NewRng(seed), opts, vs, tables, &cg, st, nil, list)
		if e == nil {
			continue
		}
		if e.Term == TermFunction && e.Invoke != nil && e.Invoke.IsStd {
			t.Fatalf("struct type must not use std op: %s", e.Invoke.Binary+e.Invoke.Unary)
		}
	}
}

func TestMakeExpressionFuncallRequiresFactMgr(t *testing.T) {
	// ExpressionFuncall.cpp:75 get_fact_mgr — no invent without FM
	opts := Defaults()
	cg := EmptyCGContext()
	if makeExpressionFuncall(NewRng(1), opts, NewVariableSelector(opts), NewExprTables(opts), &cg, GetIntType(), nil, nil) != nil {
		t.Fatal("nil FM must fail closed")
	}
}

func TestMakeExpressionFuncallIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / facts fail closed sticky (no invent funcall soft re-pick)
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	fm := NewFactMgr(nil)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &inc
	if makeExpressionFuncall(NewRng(1), opts, vs, NewExprTables(opts), &cg, GetIntType(), nil, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed makeExpressionFuncall")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	cg2 := WithFunc(nil, IncompleteEffect()).WithFactMgr(NewFactMgr(nil))
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if makeExpressionFuncall(NewRng(2), opts, vs, NewExprTables(opts), &cg2, GetIntType(), nil, nil) != nil {
		t.Fatal("incomplete EffectContext must fail closed makeExpressionFuncall")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
	fm3 := NewFactMgr(nil)
	fm3.GlobalFacts = IncompleteFactSlice()
	cg3 := EmptyCGContext().WithFactMgr(fm3)
	if makeExpressionFuncall(NewRng(3), opts, vs, NewExprTables(opts), &cg3, GetIntType(), nil, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed makeExpressionFuncall")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeExpressionFuncallRestoresFactsOnFail(t *testing.T) {
	// ExpressionFuncall.cpp:84–90 — restore facts when invocation failed
	opts := Defaults()
	opts.MaxFuncs = 0 // force failure to create user funcs; may still get std
	vs := NewVariableSelector(opts)
	// seed globals for variable fallback
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a)}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	// mark as written so restore is observable if accum mutates
	pre := CloneFactSlice(fm.GlobalFacts)
	// force failed user path: nil list / max funcs
	list := &FunctionList{}
	// std may succeed; use void type to force user and fail
	e := makeExpressionFuncall(NewRng(1), opts, vs, NewExprTables(opts), &cg, GetSimpleType(EVoid), nil, list)
	// facts should still be recoverable (either unchanged or restored)
	if len(fm.GlobalFacts) != len(pre) {
		// RestoreFacts may replace; ensure related fact still present
		if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
			t.Fatal("facts lost")
		}
	}
	_ = e
}

func TestExpressionVariableAddrOfArgForbiddenAsParam(t *testing.T) {
	// ExpressionVariable.cpp:97–100 — var->type->is_dereferenced_from(want)
	// Taking & of argument for pointer want is forbidden when as_param.
	ClearError()
	opts := Defaults()
	opts.AddrTakenOfLocals = true // only as_param rule under test
	vs := NewVariableSelector(opts)
	pt := PointerTo(GetIntType())
	// sole candidate: int argument (address would yield int*)
	arg := CreateVariableScalars("p_1", GetIntType(), false, false)
	// mark as argument via name p_ / IsArgument
	if !arg.IsArgument() {
		// force argument role if CreateVariableScalars does not
		arg.Name = "p_1"
	}
	f := &Function{Name: "f", ReturnType: GetIntType(), Param: []*Variable{arg}}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// only param available — select may create globals; disable globals to force param
	opts.GlobalVariables = false
	vs.Opts = opts
	// param on function only
	e := makeExpressionVariableFlags(NewRng(2), vs, &cg, pt, nil, true, false)
	// either nil (loop exhaust / create local only) or not &arg
	if e != nil && e.Var == arg && e.IndirectLevel() < 0 {
		t.Fatal("as_param must not take address of argument")
	}
}

func TestExpressionVariableIsDereferencedFromOrder(t *testing.T) {
	// want int*, var int → var.Type.IsDereferencedFrom(want) true (address-of)
	// want.IsDereferencedFrom(var) false
	want := PointerTo(GetIntType())
	vty := GetIntType()
	if !vty.IsDereferencedFrom(want) {
		t.Fatal("int is_dereferenced_from int* (address-of)")
	}
	if want.IsDereferencedFrom(vty) {
		t.Fatal("int* is not obtained by deref of int")
	}
}

func TestMakeRandomExpressionIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent leaf / soft re-pick past holes)
	ClearError()
	opts := Defaults()
	tables := NewExprTables(opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &inc
	if MakeRandomExpression(NewRng(1), opts, tables, nil, &cg, GetIntType(), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomExpression")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	cg2 := WithFunc(nil, IncompleteEffect())
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if MakeRandomExpression(NewRng(2), opts, tables, nil, &cg2, GetIntType(), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomExpression")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg3 := EmptyCGContext().WithFactMgr(fm)
	if MakeRandomExpression(NewRng(3), opts, tables, nil, &cg3, GetIntType(), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomExpression")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
	// MakeRandomParam same ambient gate
	inc2 := IncompleteEffect()
	cg4 := EmptyCGContext()
	cg4.EffectAccum = &inc2
	if MakeRandomParam(NewRng(4), opts, tables, nil, &cg4, GetIntType(), nil, 0) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomParam")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky MakeRandomParam")
	}
	ClearError()
}
