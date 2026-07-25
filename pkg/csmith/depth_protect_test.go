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
	// FunctionInvocation.cpp:144 — assert(type) sticky; no GetIntType soft invent
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	c := EmptyCGContext().WithSession(testAmbientSession)
	if fi := MakeRandomUnaryInvocation(NewRngSess(testAmbientSession, 1), opts, NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &c, nil); fi != nil {
		t.Fatal("nil type must not soft-fallback")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type MakeRandomUnaryInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestDepthGuardTypeAndSafeOpFlags(t *testing.T) {
	// Type.cpp / SafeOpFlags.cpp DEPTH_GUARD wired; random mode always GOOD
	ClearErrorSess(testAmbientSession)
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
	if f := MakeRandomBinaryKind(NewRngSess(testAmbientSession, 1), opts, probs, GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), SafeOpBinary, BinAdd); f == nil {
		t.Fatal("binary flags")
	}
	if t2 := RandomTypeFromType(NewRngSess(testAmbientSession, 1), nil, opts, probs, GetIntTypeSess(testAmbientSession), false, false); t2 == nil {
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
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, probs, vs, nil)
	f := MakeFirst(NewRngSess(testAmbientSession, 2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
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

func TestDepthGuardUnknownTypeFailClosed(t *testing.T) {
	// DepthSpec.cpp:381–382 assert(0); DFS mode → BAD_DEPTH sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.DFSExhaustive = true
	if DepthGuardByType(opts, "dtNoSuchType") != BadDepth {
		t.Fatal("unknown dType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown dType DepthGuard must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MinimalDepth("dtNoSuchType", 0) >= 0 {
		t.Fatal("unknown minimal depth")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown MinimalDepth must SetError sticky")
	}
	// random mode still GOOD for any type name (guard short-circuits)
	ClearErrorSess(testAmbientSession)
	opts.DFSExhaustive = false
	if DepthGuardByType(opts, "dtNoSuchType") != GoodDepth {
		t.Fatal("random mode always GOOD")
	}
	ClearErrorSess(testAmbientSession)
}

func TestKnownDepthTypeUnknownResidualSticky(t *testing.T) {
	// MinimalDepth residual soft invent was invent known-true for unknown dType.
	ClearErrorSess(testAmbientSession)
	if knownDepthType("dtTotallyUnknown") {
		t.Fatal("unknown dType must fail closed not-known")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown dType knownDepthType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !knownDepthType(DtConstant) {
		t.Fatal("DtConstant must be known")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete knownDepthType must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestDefaultDepthProtectNoInventDEPTH(t *testing.T) {
	// Block.cpp:255–267 — DEPTH++/-- only when CGOptions::depth_protect()
	// Function.cpp:648 sets body->set_depth_protect(true) always; must not invent DEPTH emit.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if opts.DepthProtect {
		t.Fatal("default depth_protect must be false")
	}
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, Defaults())
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "DEPTH++") || strings.Contains(out, "DEPTH--") {
		t.Fatal("default options must not invent DEPTH++/--")
	}
	if strings.Contains(out, "MAX_DEPTH") {
		t.Fatal("default must not invent MAX_DEPTH macro")
	}
}
