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
