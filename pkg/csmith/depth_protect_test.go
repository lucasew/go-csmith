package csmith

import (
	"strings"
	"testing"
)

func TestDepthGuardRandomModeAlwaysGood(t *testing.T) {
	opts := Defaults()
	if DepthGuardByDepth(opts, 99) != GoodDepth {
		t.Fatal("depth")
	}
	if DepthGuardByType(opts, "dtBlock") != GoodDepth {
		t.Fatal("type")
	}
	// wired factories always GOOD in random mode
	for _, dt := range []string{
		DtStatementIf, DtStatementExpr, DtStatementReturn,
		DtFunctionInvocationRandomUnary, DtFunctionInvocationRandomBinary,
		DtFunctionInvocationBinary, DtExpression, DtLhs,
	} {
		if DepthGuardByType(opts, dt) != GoodDepth {
			t.Fatal(dt)
		}
	}
}

func TestMakeRandomUnaryInvocationNilType(t *testing.T) {
	// FunctionInvocation.cpp:144 — assert(type); no GetIntType soft invent
	opts := Defaults()
	c := EmptyCGContext()
	if fi := MakeRandomUnaryInvocation(NewRng(1), opts, NewVariableSelector(opts), NewExprTables(opts), &c, nil); fi != nil {
		t.Fatal("nil type must not soft-fallback")
	}
}

func TestDepthGuardTypeAndSafeOpFlags(t *testing.T) {
	// Type.cpp / SafeOpFlags.cpp DEPTH_GUARD wired; random mode always GOOD
	opts := Defaults()
	if DepthGuardByType(opts, DtRandomTypeFromType) != GoodDepth {
		t.Fatal("dtRandomTypeFromType")
	}
	if DepthGuardByType(opts, DtTypeChooseSimple) != GoodDepth {
		t.Fatal("dtTypeChooseSimple")
	}
	if DepthGuardByTypeFlag(opts, DtSafeOpFlags, int(SafeOpBinary)) != GoodDepth {
		t.Fatal("dtSafeOpFlags")
	}
	probs := NewProbabilities(opts)
	if f := MakeRandomBinaryKind(NewRng(1), opts, probs, GetIntType(), GetIntType(), GetIntType(), SafeOpBinary, BinAdd); f == nil {
		t.Fatal("binary flags")
	}
	if t2 := RandomTypeFromType(NewRng(1), nil, opts, probs, GetIntType(), false, false); t2 == nil {
		t.Fatal("random type from simple")
	}
}

func TestDepthProtectEmit(t *testing.T) {
	opts := Defaults()
	opts.DepthProtect = true
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#define MAX_DEPTH") {
		t.Fatal("header")
	}
	if !strings.Contains(out, "DEPTH++") || !strings.Contains(out, "DEPTH--") {
		t.Fatal("block depth")
	}
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") {
		t.Fatal("func guard")
	}
	if !strings.Contains(out, "else") {
		t.Fatal("else return")
	}
}

func TestMakeReturnConstWhenDepthProtect(t *testing.T) {
	opts := Defaults()
	opts.DepthProtect = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	if f == nil {
		t.Fatal("nil")
	}
	if f.NeedReturnStmt() && f.RetConst == nil {
		t.Fatal("expected ret_c")
	}
	out := f.Output()
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") {
		t.Fatal(out)
	}
}
