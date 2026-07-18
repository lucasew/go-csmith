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
	e := MakeRandomExpression(r, opts, tables, nil, EmptyCGContext(), GetSimpleType(EInt), nil, false, false, TermConstant, 0)
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
	e := MakeRandomExpression(r, opts, tables, vs, EmptyCGContext(), GetSimpleType(EInt), &q, false, false, TermVariable, 0)
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
