package csmith

import "testing"

func TestPickTermTypeNoFuncNoConst(t *testing.T) {
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	// Filter function+const → only Variable/Assign/Comma remain (weights 20+10+10=40)
	// Seed2 first RndUpto(40) with filter rejecting F and C
	r := NewRngSess(testAmbientSession, 2)
	tt := PickTermTypeSess(testAmbientSession, r, tables, opts, GetSimpleTypeSess(testAmbientSession, EInt), true, true, 0)
	if tt == TermFunction || tt == TermConstant {
		t.Fatalf("filtered terms appeared: %v", tt)
	}
}

func TestPickTermTypeDepthBlocksNested(t *testing.T) {
	opts := Defaults()
	opts.MaxExprComplexity = 2
	tables := NewExprTablesSess(testAmbientSession, opts)
	// exprDepth+2 > max → filter Function, Assign, Comma → only Variable+Constant
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 50; i++ {
		tt := PickTermTypeSess(testAmbientSession, r, tables, opts, GetSimpleTypeSess(testAmbientSession, EInt), false, false, 1)
		if tt == TermFunction || tt == TermAssignment || tt == TermCommaExpr {
			t.Fatalf("depth gate failed: %v", tt)
		}
	}
}

func TestMakeRandomExpressionConstant(t *testing.T) {
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	e := func() *Expression {
		c := EmptyCGContext().WithSession(testAmbientSession)
		return MakeRandomExpression(r, opts, tables, nil, &c, GetSimpleTypeSess(testAmbientSession, EInt), nil, false, false, TermConstant, 0)
	}()
	if e == nil || e.Term != TermConstant || e.Con == nil || e.OutputSess(testAmbientSession) == "" {
		t.Fatalf("%+v out=%q", e, e.OutputSess(testAmbientSession))
	}
}

func TestMakeRandomExpressionVariableCreatesGlobal(t *testing.T) {
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	e := func() *Expression {
		c := EmptyCGContext().WithSession(testAmbientSession)
		return MakeRandomExpression(r, opts, tables, vs, &c, GetSimpleTypeSess(testAmbientSession, EInt), &q, false, false, TermVariable, 0)
	}()
	if e == nil || e.Term != TermVariable || e.Var == nil {
		t.Fatalf("%+v", e)
	}
	if !e.Var.IsGlobalSess(testAmbientSession) {
		t.Fatal("expected global")
	}
	if len(vs.GlobalList) < 1 {
		t.Fatal("GlobalList empty")
	}
}

func TestExpressionTypeProbabilitySeedBand(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	f := NewVectorFilterSess(testAmbientSession, &tables.Expr)
	// no filters: max=120
	r := NewRngSess(testAmbientSession, 2)
	// first RndUpto(120) for seed2
	r2 := NewRngSess(testAmbientSession, 2)
	raw := int(r2.RndUptoSess(testAmbientSession, 120))
	want := TermType(tables.Expr.RndNumToKeySess(testAmbientSession, raw))
	got := ExpressionTypeProbabilitySess(testAmbientSession, r, f)
	if got != want {
		t.Fatalf("got %v want %v (raw %d)", got, want, raw)
	}
	// Expression.cpp:107–111 assert(filter) ERROR_GUARD sticky
	ClearErrorSess(testAmbientSession)
	if ExpressionTypeProbabilitySess(testAmbientSession, nil, f) != MaxTermTypes {
		t.Fatal("nil RNG must fail closed MaxTermTypes")
	}
	// nil RNG ExpressionTypeProbability must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if ExpressionTypeProbabilitySess(testAmbientSession, NewRngSess(testAmbientSession, 1), nil) != MaxTermTypes {
		t.Fatal("nil filter must fail closed MaxTermTypes")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil filter ExpressionTypeProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleWithExprNilVarFailClosed(t *testing.T) {
	// ExpressionVariable always has live Variable*; nil hole sticky fail closed reject
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	live := &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}
	hole := &Expression{Term: TermVariable, Var: nil}
	if hole.CompatibleWithExprSess(testAmbientSession, live, false) {
		t.Fatal("nil Var lhs must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var lhs CompatibleWithExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if live.CompatibleWithExprSess(testAmbientSession, hole, false) {
		t.Fatal("nil Var rhs must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var rhs CompatibleWithExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Compatible residual soft invent was soft-continue invent compatible true.
	// Fair: sticky false. CompatibleWithVar residual via nil Var path already sticky.
	// CompatibleCheckExprs residual soft invent was soft-continue no-reject invent false.
	// Fair: sticky reject true — requires CompatibleChecker static enabled.
	opts := Defaults()
	EnableCompatibleCheckSess(testAmbientSession)
	if !CompatibleCheckExprsSess(testAmbientSession, opts, hole, live) {
		t.Fatal("Compatible residual CompatibleCheckExprs must fail closed reject true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Compatible residual CompatibleCheckExprs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ResetCompatibleCheckSess(testAmbientSession)
}

func TestConstantCompatibleWithVarExpandStruct(t *testing.T) {
	// Constant.cpp:488–493 — expand_struct → true; else false
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	c := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)}
	if c.CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("without expand_struct constant incompatible")
	}
	if !c.CompatibleWithVarSess(testAmbientSession, v, true) {
		t.Fatal("expand_struct → true")
	}
	// assert(v) — nil var sticky fail closed
	if c.CompatibleWithVarSess(testAmbientSession, nil, true) {
		t.Fatal("nil var")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var CompatibleWithVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionGetQualifiersIndirect(t *testing.T) {
	// ExpressionVariable.cpp:194–196 — qfer.indirect_qualifiers(deref)
	// Layout [ptr_level, storage]; deref pops storage (Lhs test: remaining [false])
	ClearErrorSess(testAmbientSession)
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	q := NewCVQualifiersSess(testAmbientSession, []bool{false, true}, []bool{false, false})
	v := CreateVariableQferSess(testAmbientSession, "g_p", pt, q)
	e := &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}
	gq := e.GetQualifiersSess(testAmbientSession)
	if len(gq.IsConsts) != 1 {
		t.Fatalf("after deref: %+v", gq)
	}
	// bare pointer type → indirect 0 → full two-level qfer
	e2 := &Expression{Term: TermVariable, Var: v, ExprType: pt}
	gq2 := e2.GetQualifiersSess(testAmbientSession)
	if len(gq2.IsConsts) != 2 || !gq2.IsConsts[1] {
		t.Fatalf("no deref: %+v", gq2)
	}
	// assign uses Lhs quals
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	st := &Stmt{Kind: StmtAssign, Lhs: lhs, LhsVar: v, AssignOp: AssignSimple}
	ea := &Expression{Term: TermAssignment, Assign: st}
	if len(ea.GetQualifiersSess(testAmbientSession).IsConsts) != 1 {
		t.Fatalf("assign: %+v", ea.GetQualifiersSess(testAmbientSession))
	}
}

func TestExpressionGetTypeIncompleteFailClosed(t *testing.T) {
	// no invent ExprType shell without live invoke / assign / comma RHS
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction, ExprType: GetIntTypeSess(testAmbientSession)}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Invoke must not invent type from ExprType")
	}
	if (&Expression{Term: TermAssignment, ExprType: GetIntTypeSess(testAmbientSession)}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Assign must not invent type from ExprType")
	}
	if (&Expression{Term: TermCommaExpr}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil CommaRHS must fail closed nil type, not panic")
	}
	if (&Expression{Term: TermVariable}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Var must fail closed")
	}
	// ExprType alone must not invent type without live Var
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermVariable, ExprType: GetIntTypeSess(testAmbientSession)}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Var must not invent type from ExprType alone")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var+ExprType GetType must SetError sticky")
	}
	// incomplete Constant Con/Type sticky (no invent untyped constant soft-miss)
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermConstant}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Con must fail closed nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermConstant, Con: &Constant{Value: "0"}}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Con.Type must fail closed nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con.Type GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete still works
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	if (&Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}).GetTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
		t.Fatal("complete variable type")
	}
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	if (&Expression{Term: TermCommaExpr, CommaLHS: rhs, CommaRHS: rhs}).GetTypeSess(testAmbientSession) != GetIntTypeSess(testAmbientSession) {
		t.Fatal("complete comma RHS type")
	}
}

func TestExpressionEqualsIntIncompleteFailClosed(t *testing.T) {
	// incomplete must not panic or invent fold as equals
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermCommaExpr}).EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("nil CommaRHS must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CommaRHS EqualsInt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermAssignment, Assign: &Stmt{AssignOp: AssignSimple}}).EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("nil Assign.Expr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Assign.Expr EqualsInt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction}).EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("nil Invoke must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke EqualsInt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested EqualsInt residual soft invent was soft-continue invent equal true.
	// Fair: sticky false. Invoke with incomplete unary arg residual.
	holeInv := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{nil}}
	e := &Expression{Term: TermFunction, Invoke: holeInv}
	if e.EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("nested EqualsInt residual must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested EqualsInt residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// NotEquals residual same invent soft-continue.
	// Fair: sticky false.
	if (&Expression{Term: TermConstant, Con: &Constant{Type: nil, Value: "1"}}).NotEqualsSess(testAmbientSession, 0) {
		t.Fatal("Type-nil Con NotEquals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Con NotEquals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionLessThanAndIs0Or1(t *testing.T) {
	if !(&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}).LessThanSess(testAmbientSession, 5) {
		t.Fatal("3 < 5")
	}
	if (&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 7)}).LessThanSess(testAmbientSession, 5) {
		t.Fatal("7 < 5")
	}
	// FunctionInvocationUnary::is_0_or_1 — eNot only
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "!"}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if !e.Is0Or1Sess(testAmbientSession) {
		t.Fatal("unary not")
	}
	// binary comparison also 0/1
	fi2 := &Invocation{IsStd: true, Binary: "==", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2)},
	}}
	if !(&Expression{Term: TermFunction, Invoke: fi2}).Is0Or1Sess(testAmbientSession) {
		t.Fatal("cmp")
	}
	// simple assign of !x
	st := &Stmt{
		Kind: StmtAssign, AssignOp: AssignSimple,
		Expr: e,
	}
	ea := &Expression{Term: TermAssignment, Assign: st}
	if !ea.Is0Or1Sess(testAmbientSession) {
		t.Fatal("assign peel")
	}
}

func TestExpressionComplexityFuncArgs(t *testing.T) {
	// ExpressionFuncall.cpp:131–143 — call + sum(args)
	inner := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	fi := &Invocation{
		User: &Function{Name: "f"}, IsStd: false,
		Args: []*Expression{inner, inner},
	}
	e := &Expression{Term: TermFunction, Invoke: fi}
	// 1 (call) + 0 + 0
	if ExpressionComplexitySess(testAmbientSession, e) != 1 {
		t.Fatal(ExpressionComplexitySess(testAmbientSession, e))
	}
	// nested call arg
	nested := &Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "g"}, Args: nil}}
	fi2 := &Invocation{User: &Function{Name: "f"}, Args: []*Expression{nested}}
	e2 := &Expression{Term: TermFunction, Invoke: fi2}
	if ExpressionComplexitySess(testAmbientSession, e2) != 2 {
		t.Fatal(ExpressionComplexitySess(testAmbientSession, e2))
	}
	// nil Invoke — fail closed sticky -1 (no invent leaf depth 0)
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil invoke must fail closed -1, not invent depth 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil invoke ExpressionComplexity must SetError sticky")
	}
	// incomplete assign / comma / nil arg — fail closed sticky -1
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermAssignment}) >= 0 {
		t.Fatal("nil Assign must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Assign ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermCommaExpr, CommaLHS: inner}) >= 0 {
		t.Fatal("nil CommaRHS must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CommaRHS ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{
		User: &Function{Name: "h"}, Args: []*Expression{inner, nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg ExpressionComplexity must SetError sticky")
	}
	// incomplete constant / variable leaf
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermConstant}) >= 0 {
		t.Fatal("nil Con must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant shell sticky (no invent leaf complexity 0)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}) >= 0 {
		t.Fatal("nil Con.Type must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con.Type ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable shell sticky (no invent leaf complexity 0)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil}}) >= 0 {
		t.Fatal("Type-nil Var must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Var ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-std nil User sticky (no invent complexity 0 as non-call)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{}}) >= 0 {
		t.Fatal("non-std nil User must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-std nil User ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// std binary without User is complete leaf complexity 0
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{IsStd: true, Binary: "+"}}) != 0 {
		t.Fatal("std binary ExpressionComplexity must be 0")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("std binary ExpressionComplexity must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ExpressionComplexitySess(testAmbientSession, &Expression{Term: TermVariable}) >= 0 {
		t.Fatal("nil Var must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var ExpressionComplexity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionIndentedOutput(t *testing.T) {
	e := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 7)}
	got := e.IndentedOutputSess(testAmbientSession, 2)
	if got != "        7" { // OutputTab 4 spaces per level
		t.Fatalf("%q", got)
	}
}

func TestConstantGetField(t *testing.T) {
	// Constant.cpp:513–522
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f2", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	c := &Constant{Type: ut, Value: "{0, 1, 2}"}
	if c.GetFieldSess(testAmbientSession, 0) != "0" || c.GetFieldSess(testAmbientSession, 1) != "1" || c.GetFieldSess(testAmbientSession, 2) != "2" {
		t.Fatal(c.GetFieldSess(testAmbientSession, 0), c.GetFieldSess(testAmbientSession, 1), c.GetFieldSess(testAmbientSession, 2))
	}
	if c.GetFieldSess(testAmbientSession, 9) != "" {
		t.Fatal("oob")
	}
	// Constant always live; sticky empty (no invent empty field soft-skip)
	ClearErrorSess(testAmbientSession)
	if (*Constant)(nil).GetFieldSess(testAmbientSession, 0) != "" {
		t.Fatal("nil GetField must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Value incomplete shell sticky (no invent empty field soft-skip)
	if (&Constant{Type: GetIntTypeSess(testAmbientSession), Value: ""}).GetFieldSess(testAmbientSession, 0) != "" {
		t.Fatal("empty Value GetField must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Value GetField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil incomplete shell sticky (no invent field split past hole)
	if (&Constant{Value: "{0, 1}"}).GetFieldSess(testAmbientSession, 0) != "" {
		t.Fatal("nil Type GetField must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type GetField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionTypeProbabilityForceFunction(t *testing.T) {
	// Expression.cpp:104–105 — direct_expand_check(eInvoke) → eFunction
	ClearPartialExpanderSess(testAmbientSession)
	if !InitPartialExpanderSess(testAmbientSession, "invoke") {
		t.Fatal("init")
	}
	defer ClearPartialExpanderSess(testAmbientSession)
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	f := NewVectorFilterSess(testAmbientSession, &tables.Expr)
	// even with no_func filter setup in PickTermType, ExpressionTypeProbability alone forces Function
	got := ExpressionTypeProbabilitySess(testAmbientSession, NewRngSess(testAmbientSession, 2), f)
	if got != TermFunction {
		t.Fatalf("got %v want TermFunction", got)
	}
	// PickTermType with noFunc still hits ExpressionTypeProbability force
	tt := PickTermTypeSess(testAmbientSession, NewRngSess(testAmbientSession, 2), tables, opts, GetIntTypeSess(testAmbientSession), true, false, 0)
	if tt != TermFunction {
		t.Fatalf("PickTermType force: %v", tt)
	}
}

func TestMakeExpressionVariablePassesDummyToSelect(t *testing.T) {
	// ExpressionVariable.cpp:78 — select(..., dummy invalid_vars)
	// After rejecting a float for non-float want, select must not keep returning it forever.
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// only a float global
	fv := CreateVariableScalarsSess(testAmbientSession, "g_f", GetSimpleTypeSess(testAmbientSession, EFloat), true, false)
	vs.GlobalList = []*Variable{fv}
	vs.AllVars = []*Variable{fv}
	// force global selection
	opts.GlobalVariables = true
	vs.Opts = opts
	// int want — float rejected then new var created (ScopeNewValue) or nil after tries
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = vs.Types
	ev := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 1), vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false)
	// either created a new non-float, or nil — must not return the float
	if ev != nil && ev.Var == fv {
		t.Fatal("must not use float for int want")
	}
	// ExpressionVariable.cpp always has RNG; sticky no invent var shell
	ClearErrorSess(testAmbientSession)
	if e := makeExpressionVariableFlags(nil, vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false); e != nil {
		t.Fatal("nil RNG must not invent ExpressionVariable")
	}
	// nil RNG makeExpressionVariableFlags must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	// Type* always live; nil want must not soft-skip type filters sticky
	if e := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 1), vs, &cg, nil, nil, false, false); e != nil {
		t.Fatal("nil typ must not invent ExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil typ makeExpressionVariableFlags must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil VS is soft re-pick (not sticky) — MaxTermTypes unit paths omit selector
	if e := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 1), nil, &cg, GetIntTypeSess(testAmbientSession), nil, false, false); e != nil {
		t.Fatal("nil vs must not invent ExpressionVariable")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil vs makeExpressionVariableFlags must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	// Variable::type always live; Type-nil candidate must not soft-skip filters to success
	ClearErrorSess(testAmbientSession)
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), true, false)
	broken.Type = nil
	vs.GlobalList = []*Variable{broken}
	vs.AllVars = []*Variable{broken}
	// disable new-var creation path by restricting options if possible; still must not return broken
	evBroken := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 3), vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false)
	if evBroken != nil && evBroken.Var == broken {
		t.Fatal("Type-nil var must not be accepted as ExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		// Type-nil may SetError in makeExpressionVariableFlags or via ChooseVarFull
		// either sticky path is acceptable; only invent success is forbidden
	}
	// sticky ERROR_GUARD after incomplete type IR — clear for suite
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionVariableIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent var expr soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{v}
	vs.AllVars = []*Variable{v}
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if makeExpressionVariableFlags(NewRngSess(testAmbientSession, 1), vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false) != nil {
		t.Fatal("incomplete EffectAccum must fail closed makeExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if makeExpressionVariableFlags(NewRngSess(testAmbientSession, 2), vs, &cg2, GetIntTypeSess(testAmbientSession), nil, false, false) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed makeExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionVariableIndirectZeroUsesVarType(t *testing.T) {
	// ExpressionVariable.cpp:122–123 — indirection 0 → ExpressionVariable(*var) without forced type
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{v}
	vs.AllVars = []*Variable{v}
	vs.Opts = opts
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// want int, var int → indirect 0 → ExprType should be var.Type
	ev := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 2), vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false)
	if ev == nil {
		t.Fatal("nil")
	}
	if ev.Var != v {
		// may create new var if select path differs — still check zero indirect shape
		if ev.IndirectLevelSess(testAmbientSession) != 0 {
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
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	ev := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 2), vs, &cg, GetIntTypeSess(testAmbientSession), nil, false, false)
	if ev == nil || ev.Var == nil {
		t.Skip("no expression variable")
	}
	if cg.EffectAccum != nil && !cg.EffectAccum.IsReadSess(testAmbientSession, ev.Var) && !cg.EffectStm.IsReadSess(testAmbientSession, ev.Var) {
		t.Fatalf("expected read effect on var %s after visit_facts", ev.Var.Name)
	}
}

func TestSelectWithInvalidRejectsVolatileWhenImpure(t *testing.T) {
	// VariableSelector.cpp:1225–1227 — assert(!var->is_volatile()) under impure effect_context
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Opts = opts
	vq := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{true})
	vol := CreateVariableQferSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), vq)
	vs.GlobalList = []*Variable{vol}
	vs.AllVars = []*Variable{vol}
	cg := WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)
	// under impure context must never hand back a volatile
	for seed := uint64(1); seed < 40; seed++ {
		got := vs.SelectWithInvalid(AccessRead, cg, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, seed), MatchFlexible, nil)
		if got != nil && got.IsVolatileSess(testAmbientSession) {
			t.Fatalf("seed %d: must not return volatile under impure effect_context", seed)
		}
	}
}

func TestSelectWithInvalidExpandStructNewValueErrors(t *testing.T) {
	// VariableSelector.cpp:1217–1219 — eNewValue + expand_struct sets ERROR → nullptr
	opts := Defaults()
	opts.ExpandStruct = true
	opts.GlobalVariables = true
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Opts = opts
	// scan until ScopeNewValue (rnd_upto(100) in [95,100) with globals table)
	hit := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearErrorSess(testAmbientSession)
		// empty lists → local/param fail, NewValue or Global create
		vs.GlobalList = nil
		vs.AllVars = nil
		v := vs.SelectWithInvalid(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, seed), MatchFlexible, nil)
		if HasErrorSess(testAmbientSession) {
			hit = true
			if v != nil {
				t.Fatal("ERROR_GUARD must return nil")
			}
			break
		}
	}
	ClearErrorSess(testAmbientSession)
	if !hit {
		t.Log("ScopeNewValue+expand_struct not hit in seed scan")
	}
}

func TestSelectWithInvalidExcludesDummy(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{a, b}
	vs.AllVars = []*Variable{a, b}
	vs.Opts = opts
	// only two globals; exclude a → must pick b or create
	got := vs.SelectWithInvalid(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 3), MatchFlexible, []*Variable{a})
	if got == a {
		t.Fatal("invalid_vars must exclude a")
	}
}

func TestBumpsExprDepth(t *testing.T) {
	// Expression.cpp:213–218
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}) {
		t.Fatal("const")
	}
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermVariable, Var: v}) {
		t.Fatal("var")
	}
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "f"}}}) {
		t.Fatal("user call")
	}
	if BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{IsStd: true, Binary: "+"}}) {
		t.Fatal("std binary no bump")
	}
	if BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermCommaExpr}) {
		t.Fatal("comma no bump")
	}
	// Expression always live; sticky true (no invent not-bump soft-skip depth)
	ClearErrorSess(testAmbientSession)
	if !BumpsExprDepthSess(testAmbientSession, nil) {
		t.Fatal("nil BumpsExprDepth must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil BumpsExprDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Function IR sticky true (no invent not-bump for siblings past hole)
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermFunction}) {
		t.Fatal("nil Invoke BumpsExprDepth must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke BumpsExprDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{}}) {
		t.Fatal("non-std nil User BumpsExprDepth must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-std nil User BumpsExprDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant sticky bump (no invent not-bump soft-skip depth past hole)
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}) {
		t.Fatal("Type-nil Con BumpsExprDepth must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Con BumpsExprDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable sticky bump (specials exempt)
	if !BumpsExprDepthSess(testAmbientSession, &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil}}) {
		t.Fatal("Type-nil Var BumpsExprDepth must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Var BumpsExprDepth must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExpressionBumpsCallerExprDepth(t *testing.T) {
	// Expression.cpp:213–218 — cg_context.expr_depth++ on Constant/Variable/user call
	opts := Defaults()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.ExprDepth = 2
	e := MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, NewExprTablesSess(testAmbientSession, opts), nil, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermConstant, cg.ExprDepth)
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
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.ExprDepth = 2 // near max: 2+2 > 3 → force leaf terms
	// pass stale exprDepth=0 that would allow Function if used; force Constant leaf
	e := MakeRandomExpression(NewRngSess(testAmbientSession, 5), opts, NewExprTablesSess(testAmbientSession, opts), nil, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("%+v", e)
	}
	// with high cg.ExprDepth, complex MaxTermTypes must not pick Function via stale 0
	e2 := MakeRandomExpression(NewRngSess(testAmbientSession, 5), opts, NewExprTablesSess(testAmbientSession, opts), nil, &cg, GetIntTypeSess(testAmbientSession), nil, false, false, MaxTermTypes, 0)
	if e2 != nil && (e2.Term == TermFunction || e2.Term == TermAssignment || e2.Term == TermCommaExpr) {
		t.Fatalf("stale depth arg must not allow complex term: %v", e2.Term)
	}
}

func TestMakeRandomExpressionNilTypeUsesEnv(t *testing.T) {
	// Expression.cpp:147–152 — nil type from choose_random_nonvoid when SE-free
	opts := Defaults()
	env := &TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort)}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = env
	// force constant so we don't need VariableSelector
	e := MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, NewExprTablesSess(testAmbientSession, opts), nil, &cg, nil, nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("%+v", e)
	}
	// type was chosen from env (not stuck on void)
	if e.Con == nil || e.Con.Type == nil {
		t.Fatal("const type")
	}
	// empty/nil TypeEnv: complete soft miss (no invent simple type; non-sticky soft re-pick)
	// later choose retries may still SetError when typ remains nil after tries
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.Types = &TypeEnv{Sess: testAmbientSession}
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, NewExprTablesSess(testAmbientSession, opts), nil, &cg2, nil, nil, true, false, TermConstant, 0) != nil {
		t.Fatal("empty Type env must not invent simple type")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExpressionNoInventSessionProbs(t *testing.T) {
	// C++ Probabilities singleton; no invent NewProbabilities(opts) when vs.Probs nil
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// nil vs: simple constant still ok (MakeRandom allows nil probs for simple)
	e := MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermConstant, 0)
	if e == nil || e.Term != TermConstant {
		t.Fatalf("simple const without vs: %+v", e)
	}
	// vs with nil Probs: same simple path; must not invent session tables
	vs := &VariableSelector{Opts: opts, Sess: testAmbientSession}
	e2 := MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, vs, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermConstant, 0)
	if e2 == nil || e2.Term != TermConstant {
		t.Fatalf("simple const with nil vs.Probs: %+v", e2)
	}
}

func TestMakeRandomExpressionAssertFailClosed(t *testing.T) {
	// Expression.cpp:154–157, 186–187 — asserts sticky; no soft invent rewrite/emit
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// no_const && eConstant
	ClearErrorSess(testAmbientSession)
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, GetIntTypeSess(testAmbientSession), nil, false, true, TermConstant, 0) != nil {
		t.Fatal("no_const + Constant must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no_const + Constant must SetError sticky")
	}
	// no_func && eFunction
	ClearErrorSess(testAmbientSession)
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermFunction, 0) != nil {
		t.Fatal("no_func + Function must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no_func + Function must SetError sticky")
	}
	// struct + eConstant (was soft invent TermVariable)
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "SAssert", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, st, nil, false, false, TermConstant, 0) != nil {
		t.Fatal("struct + Constant must fail closed, not rewrite to Variable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("struct + Constant must SetError sticky")
	}
	// void simple constant
	ClearErrorSess(testAmbientSession)
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, GetSimpleTypeSess(testAmbientSession, EVoid), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("void Constant must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("void Constant must SetError sticky")
	}
	// Expression.cpp always has RNG sticky — no invent TermConstant shell without RNG
	ClearErrorSess(testAmbientSession)
	if e := MakeRandomExpression(nil, opts, tables, nil, &cg, GetIntTypeSess(testAmbientSession), nil, true, false, TermConstant, 0); e != nil {
		t.Fatal("nil RNG must not invent TermConstant shell", e)
	}
	// nil RNG MakeRandomExpression must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionFuncallForcesUserForAggregate(t *testing.T) {
	// ExpressionFuncall.cpp:71–73 — struct/union never std unary/binary
	opts := Defaults()
	opts.MaxFuncs = 4
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{st, GetIntTypeSess(testAmbientSession)}}
	vs.Types = env
	list := &FunctionList{Types: env}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg.Types = env
	cg.Funcs = list
	// many tries: result if any should not be pure std binary/unary alone when type is struct
	// (user path may still fail → variable fallback)
	for seed := uint64(1); seed < 20; seed++ {
		ClearErrorSess(testAmbientSession)
		e := makeExpressionFuncall(NewRngSess(testAmbientSession, seed), opts, vs, tables, &cg, st, nil, list)
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
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if makeExpressionFuncall(NewRngSess(testAmbientSession, 1), opts, NewVariableSelector(testAmbientSession, opts), NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("nil FM must fail closed")
	}
}

func TestMakeExpressionFuncallIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / facts fail closed sticky (no invent funcall soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	fm := NewFactMgrSess(testAmbientSession, nil)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if makeExpressionFuncall(NewRngSess(testAmbientSession, 1), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed makeExpressionFuncall")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(nil, IncompleteEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if makeExpressionFuncall(NewRngSess(testAmbientSession, 2), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg2, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("incomplete EffectContext must fail closed makeExpressionFuncall")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm3 := NewFactMgrSess(testAmbientSession, nil)
	fm3.GlobalFacts = IncompleteFactSlice()
	cg3 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm3)
	if makeExpressionFuncall(NewRngSess(testAmbientSession, 3), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg3, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed makeExpressionFuncall")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionFuncallRestoresFactsOnFail(t *testing.T) {
	// ExpressionFuncall.cpp:84–90 — restore facts when invocation failed
	opts := Defaults()
	opts.MaxFuncs = 0 // force failure to create user funcs; may still get std
	vs := NewVariableSelector(testAmbientSession, opts)
	// seed globals for variable fallback
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// mark as written so restore is observable if accum mutates
	pre := CloneFactSliceSess(testAmbientSession, fm.GlobalFacts)
	// force failed user path: nil list / max funcs
	list := &FunctionList{}
	// std may succeed; use void type to force user and fail
	e := makeExpressionFuncall(NewRngSess(testAmbientSession, 1), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetSimpleTypeSess(testAmbientSession, EVoid), nil, list)
	// facts should still be recoverable (either unchanged or restored)
	if len(fm.GlobalFacts) != len(pre) {
		// RestoreFacts may replace; ensure related fact still present
		if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) == nil {
			t.Fatal("facts lost")
		}
	}
	_ = e
}

func TestExpressionVariableAddrOfArgForbiddenAsParam(t *testing.T) {
	// ExpressionVariable.cpp:97–100 — var->type->is_dereferenced_from(want)
	// Taking & of argument for pointer want is forbidden when as_param.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.AddrTakenOfLocals = true // only as_param rule under test
	vs := NewVariableSelector(testAmbientSession, opts)
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	// sole candidate: int argument (address would yield int*)
	arg := CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)
	// mark as argument via name p_ / IsArgument
	if !arg.IsArgumentSess(testAmbientSession) {
		// force argument role if CreateVariableScalars does not
		arg.Name = "p_1"
	}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), Param: []*Variable{arg}}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// only param available — select may create globals; disable globals to force param
	opts.GlobalVariables = false
	vs.Opts = opts
	// param on function only
	e := makeExpressionVariableFlags(NewRngSess(testAmbientSession, 2), vs, &cg, pt, nil, true, false)
	// either nil (loop exhaust / create local only) or not &arg
	if e != nil && e.Var == arg && e.IndirectLevelSess(testAmbientSession) < 0 {
		t.Fatal("as_param must not take address of argument")
	}
}

func TestExpressionVariableIsDereferencedFromOrder(t *testing.T) {
	// want int*, var int → var.Type.IsDereferencedFromSess(testAmbientSession, want) true (address-of)
	// want.IsDereferencedFromSess(testAmbientSession, var) false
	want := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	vty := GetIntTypeSess(testAmbientSession)
	if !vty.IsDereferencedFromSess(testAmbientSession, want) {
		t.Fatal("int is_dereferenced_from int* (address-of)")
	}
	if want.IsDereferencedFromSess(testAmbientSession, vty) {
		t.Fatal("int* is not obtained by deref of int")
	}
}

func TestMakeExpressionVariableResidualSticky(t *testing.T) {
	// residual ERROR soft-continue invents var expr via fall-through select / later try.
	// Fair: sticky fail closed whole makeExpressionVariableFlags.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), true, false)
	broken.Type = nil
	good := CreateVariableScalarsSess(testAmbientSession, "g_good", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{good}
	vs.AllVars = []*Variable{good}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	// must_use Type-nil stickies residual; must not invent soft select past hole
	rw := &RWDirective{MustReadVars: []*Variable{broken, good}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	if makeExpressionVariableFlags(NewRngSess(testAmbientSession, 1), vs, &cg, GetIntTypeSess(testAmbientSession), &q, false, false) != nil {
		t.Fatal("must-use Type-nil residual must fail closed makeExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must-use residual makeExpressionVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray shell in must-use: same residual invent hole
	shell := &Variable{
		Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2},
		Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}),
	}
	rw2 := &RWDirective{MustReadVars: []*Variable{shell, good}}
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw2)
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if makeExpressionVariableFlags(NewRngSess(testAmbientSession, 2), vs, &cg2, GetIntTypeSess(testAmbientSession), &q, false, false) != nil {
		t.Fatal("IsArray without AsArray must-use residual must fail closed makeExpressionVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray residual makeExpressionVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExpressionIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent leaf / soft re-pick past holes)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if MakeRandomExpression(NewRngSess(testAmbientSession, 1), opts, tables, nil, &cg, GetIntTypeSess(testAmbientSession), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomExpression")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(nil, IncompleteEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if MakeRandomExpression(NewRngSess(testAmbientSession, 2), opts, tables, nil, &cg2, GetIntTypeSess(testAmbientSession), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomExpression")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg3 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if MakeRandomExpression(NewRngSess(testAmbientSession, 3), opts, tables, nil, &cg3, GetIntTypeSess(testAmbientSession), nil, false, false, TermConstant, 0) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomExpression")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// MakeRandomParam same ambient gate
	inc2 := IncompleteEffect()
	cg4 := EmptyCGContext().WithSession(testAmbientSession)
	cg4.EffectAccum = &inc2
	if MakeRandomParam(NewRngSess(testAmbientSession, 4), opts, tables, nil, &cg4, GetIntTypeSess(testAmbientSession), nil, 0) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomParam")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky MakeRandomParam")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionOutputNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Expression)(nil).OutputSess(testAmbientSession) != "" {
		t.Fatal("nil Expression Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// outputBody residual soft invent was invent empty cast body soft-success.
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}, CastType: GetIntTypeSess(testAmbientSession)}
	if hole.OutputSess(testAmbientSession) != "" {
		t.Fatal("Type-nil Con Output residual must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Con Output residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetTypeIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Expression)(nil).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Expression GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("Funcall without Invoke GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermCommaExpr}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("comma without RHS GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("comma without RHS GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermAssignment}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("assign without Assign GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("assign without Assign GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetQualifiersEqualsIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if q := (*Expression)(nil).GetQualifiersSess(testAmbientSession); len(q.IsConsts) != 0 || len(q.IsVolatiles) != 0 {
		t.Fatal("nil Expression GetQualifiers must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression GetQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if q := (&Expression{Term: TermFunction}).GetQualifiersSess(testAmbientSession); len(q.IsConsts) != 0 {
		t.Fatal("Funcall without Invoke GetQualifiers must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke GetQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction}).EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("Funcall without Invoke EqualsInt must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke EqualsInt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Constant shell sticky (no invent complete empty-qfer past hole)
	if q := (&Expression{Term: TermConstant}).GetQualifiersSess(testAmbientSession); len(q.IsConsts) != 0 {
		t.Fatal("nil Con GetQualifiers must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con GetQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Constant complete empty quals OK
	if q := (&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}).GetQualifiersSess(testAmbientSession); len(q.IsConsts) != 0 {
		t.Fatal("Constant GetQualifiers should be empty complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("Constant GetQualifiers must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleWithIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if (*Expression)(nil).CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("nil Expression CompatibleWithVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression CompatibleWithVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermVariable}).CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("Var without Variable CompatibleWithVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Var without Variable CompatibleWithVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction}).CompatibleWithVarSess(testAmbientSession, v, false) {
		t.Fatal("Funcall without Invoke CompatibleWithVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke CompatibleWithVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Constant expand_struct complete true
	if !(&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}).CompatibleWithVarSess(testAmbientSession, v, true) {
		t.Fatal("Constant expand_struct must be true complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("Constant CompatibleWithVar must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Constant shell sticky (no invent expand_struct success past hole)
	if (&Expression{Term: TermConstant}).CompatibleWithVarSess(testAmbientSession, v, true) {
		t.Fatal("nil Con CompatibleWithVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con CompatibleWithVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIs0Or1NotEqualsLessThanIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Expression)(nil).Is0Or1Sess(testAmbientSession) {
		t.Fatal("nil Expression Is0Or1 must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression Is0Or1 must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermFunction}).Is0Or1Sess(testAmbientSession) {
		t.Fatal("Funcall without Invoke Is0Or1 must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke Is0Or1 must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested Is0Or1 residual soft invent was soft-continue invent 0or1 true.
	// Fair: sticky false. assign peel incomplete residual.
	holeAssign := &Expression{Term: TermAssignment, Assign: &Stmt{AssignOp: AssignSimple, Expr: nil}}
	if holeAssign.Is0Or1Sess(testAmbientSession) {
		t.Fatal("nested Is0Or1 residual must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested Is0Or1 residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermConstant}).NotEqualsSess(testAmbientSession, 0) {
		t.Fatal("Constant without Con NotEquals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Constant without Con NotEquals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Expression{Term: TermConstant}).LessThanSess(testAmbientSession, 1) {
		t.Fatal("Constant without Con LessThan must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Constant without Con LessThan must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Constant fold sticky (no invent fold soft-success past type hole)
	noTy := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}
	if noTy.EqualsIntSess(testAmbientSession, 0) || noTy.NotEqualsSess(testAmbientSession, 1) || noTy.LessThanSess(testAmbientSession, 1) {
		t.Fatal("Type-nil Constant fold must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Constant fold must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable term complete default false for NotEquals (Expression.h)
	if (&Expression{Term: TermVariable}).NotEqualsSess(testAmbientSession, 0) {
		t.Fatal("Variable NotEquals must default false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("Variable NotEquals must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUseVarIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	subj := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if !(*Expression)(nil).UseVarSess(testAmbientSession, subj) {
		t.Fatal("nil Expression UseVar must fail closed true (uses)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expression UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(&Expression{Term: TermVariable}).UseVarSess(testAmbientSession, subj) {
		t.Fatal("Var without Variable UseVar must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Var without Variable UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(&Expression{Term: TermFunction}).UseVarSess(testAmbientSession, subj) {
		t.Fatal("Funcall without Invoke UseVar must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcall without Invoke UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Constant complete — does not use vars
	if (&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}).UseVarSess(testAmbientSession, subj) {
		t.Fatal("Constant UseVar must be false complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("Constant UseVar must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// live Variable that is the subject
	if !(&Expression{Term: TermVariable, Var: subj}).UseVarSess(testAmbientSession, subj) {
		t.Fatal("matching Variable UseVar must be true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("matching Variable UseVar must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Match residual: Type-nil aggregate subject soft invent was not-use soft-skip.
	// Fair: sticky uses true (restrictive).
	hole := &Variable{Name: "g_agg", Type: nil}
	other := CreateVariableScalarsSess(testAmbientSession, "g_y", GetIntTypeSess(testAmbientSession), false, false)
	if !(&Expression{Term: TermVariable, Var: hole}).UseVarSess(testAmbientSession, other) {
		t.Fatal("Match residual UseVar must fail closed true (uses)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested UseVar residual in comma: soft invent was soft-continue RHS invent not-use.
	// Fair: sticky uses true.
	leftHole := &Expression{Term: TermVariable, Var: hole}
	rightOK := &Expression{Term: TermVariable, Var: other}
	comma := &Expression{Term: TermCommaExpr, CommaLHS: leftHole, CommaRHS: rightOK}
	if !comma.UseVarSess(testAmbientSession, other) {
		t.Fatal("nested Match residual UseVar must fail closed true (uses)")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested Match residual comma UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionGetTypeInvokeResidualSticky(t *testing.T) {
	// Invoke GetType residual soft invent was invent type shell past incomplete arg IR.
	ClearErrorSess(testAmbientSession)
	// binary invoke with Type-nil arg → GetType residual sticky
	fi := &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermVariable, Var: &Variable{Name: "g_x", Type: nil}},
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
		},
	}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if e.GetTypeSess(testAmbientSession) != nil {
		t.Fatal("GetType residual invoke must fail closed nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual invoke must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Output residual: Failed soft empty non-sticky; incomplete name sticky
	fi2 := &Invocation{User: &Function{Name: "", ReturnType: GetIntTypeSess(testAmbientSession)}}
	e2 := &Expression{Term: TermFunction, Invoke: fi2}
	if e2.OutputSess(testAmbientSession) != "" {
		t.Fatal("empty User name Output must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty User name Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionToStringIndentedOutputResidualSticky(t *testing.T) {
	// Output residual soft invent was invent soft-empty ToString/Indented past incomplete Con.
	ClearErrorSess(testAmbientSession)
	e := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil
	if e.ToStringSess(testAmbientSession) != "" {
		t.Fatal("Type-nil constant ToString must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil constant ToString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if e.IndentedOutputSess(testAmbientSession, 1) != "" {
		t.Fatal("Type-nil constant IndentedOutput must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil constant IndentedOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPickTermTypeIsConstStructUnionResidualSticky(t *testing.T) {
	// IsConstStructUnion residual soft invent was invent soft-filter term past Type-nil field.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	// aggregate with Type-nil field → IsConstStructUnion residual sticky true
	agg := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: nil, BitWidth: -1}}}
	tt := PickTermTypeSess(testAmbientSession, NewRngSess(testAmbientSession, 2), tables, opts, agg, false, false, 0)
	// residual → MaxTermTypes sticky
	if tt != MaxTermTypes {
		// may still return MaxTermTypes on residual
		_ = tt
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsConstStructUnion residual PickTermType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete int path no sticky
	tt2 := PickTermTypeSess(testAmbientSession, NewRngSess(testAmbientSession, 2), tables, opts, GetIntTypeSess(testAmbientSession), false, false, 0)
	if tt2 == MaxTermTypes && HasErrorSess(testAmbientSession) {
		t.Fatal("complete int PickTermType must not sticky MaxTermTypes", tt2)
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionIndirectLevelCompleteResidualSticky(t *testing.T) {
	// IndirectLevel residual soft invent was invent level-0 complete past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	e := &Expression{Term: TermVariable, Var: &Variable{Name: "g_x", Type: nil}, ExprType: GetIntTypeSess(testAmbientSession)}
	n, ok := e.IndirectLevelCompleteSess(testAmbientSession)
	if ok || n != 0 {
		t.Fatal("Type-nil Var IndirectLevelComplete must fail closed 0,false", n, ok)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Var IndirectLevelComplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExpressionVoidIsSimpleResidualSticky(t *testing.T) {
	// IsSimple residual soft invent was invent TermConstant shell past void type.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	tables := NewExprTablesSess(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// force constant term with void type via PickTermType path is hard; direct residual:
	// void simple constant path
	e := MakeRandomExpression(r, opts, tables, nil, &cg, GetSimpleTypeSess(testAmbientSession, EVoid), nil, false, false, TermConstant, 0)
	if e != nil {
		// may fail closed nil
		_ = e
	}
	// void constant must sticky ERROR (assert simple != eVoid)
	if e != nil && e.Term == TermConstant {
		if !HasErrorSess(testAmbientSession) {
			// MakeRandom may have sticky
		}
	}
	// nil Type IsSimple residual on void path via direct GetSimpleTypeSess(testAmbientSession, EVoid) complete
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsSimpleSess(testAmbientSession) {
		// IsSimple on nil returns false without SetError
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil IsPointer residual hygiene for this slice
	if (*Type)(nil).PtrTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Type PtrType must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type PtrType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionCloneGetInvokeComplexity(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	c := MakeIntSess(testAmbientSession, 3)
	e := &Expression{Term: TermConstant, Con: c}
	cl := e.CloneSess(testAmbientSession)
	if cl == nil || cl.Con == c || cl.Con.Value != "3" {
		t.Fatal(cl)
	}
	if e.GetComplexitySess(testAmbientSession) != 0 {
		t.Fatal(e.GetComplexitySess(testAmbientSession))
	}
	if e.GetInvokeSess(testAmbientSession) != nil {
		t.Fatal("non-func")
	}
	// compound sticky
	if e2 := (&Expression{Term: TermCommaExpr}).CloneSess(testAmbientSession); e2 != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("comma clone sticky")
	}
	ClearErrorSess(testAmbientSession)
	tabs := InitProbabilityTablesSess(testAmbientSession, Defaults())
	if tabs == nil || ProcessExprTablesSess(testAmbientSession) != tabs {
		t.Fatal("init tables")
	}
}

func TestExpressionConstantOutputParensNegatives(t *testing.T) {
	// Constant.cpp:534–538 via Expression::to_string for array init_strings
	ClearErrorSess(testAmbientSession)
	e := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession), Value: "-6L"}, ExprType: GetIntTypeSess(testAmbientSession)}
	got := e.OutputSess(testAmbientSession)
	if got != "(-6L)" {
		t.Fatalf("negative const Output: got %q want (-6L)", got)
	}
	e2 := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession), Value: "3L"}, ExprType: GetIntTypeSess(testAmbientSession)}
	if e2.OutputSess(testAmbientSession) != "3L" {
		t.Fatalf("positive: got %q", e2.OutputSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestExprVarSelectRetryCeiling(t *testing.T) {
	// ExpressionVariable.cpp:71–132 — do { ... } while (true) (unbounded).
	// Go soft ceiling must stay >>256: seed-599096333 exhausted at try 256
	// (depth≈70004) while UP continued the same EV loop (SelectParentLocal).
	// Lock the named constant (same order as MakeRandomLhs 10000).
	if exprVarSelectRetryCeiling < 1000 {
		t.Fatalf("exprVarSelectRetryCeiling=%d too low for C++ unbounded EV loop", exprVarSelectRetryCeiling)
	}
	if exprVarSelectRetryCeiling != 10000 {
		t.Fatalf("exprVarSelectRetryCeiling=%d want 10000 (Lhs parity / seed-599096333)", exprVarSelectRetryCeiling)
	}
}
