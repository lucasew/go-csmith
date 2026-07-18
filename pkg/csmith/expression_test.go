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
}

func TestMakeExpressionVariableIndirectZeroUsesVarType(t *testing.T) {
	// ExpressionVariable.cpp:122–123 — indirection 0 → ExpressionVariable(*var) without forced type
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
	cg := EmptyCGContext()
	cg.Types = env
	cg.Funcs = list
	// many tries: result if any should not be pure std binary/unary alone when type is struct
	// (user path may still fail → variable fallback)
	for seed := uint64(1); seed < 20; seed++ {
		e := makeExpressionFuncall(NewRng(seed), opts, vs, tables, &cg, st, nil, list)
		if e == nil {
			continue
		}
		if e.Term == TermFunction && e.Invoke != nil && e.Invoke.IsStd {
			t.Fatalf("struct type must not use std op: %s", e.Invoke.Binary+e.Invoke.Unary)
		}
	}
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
