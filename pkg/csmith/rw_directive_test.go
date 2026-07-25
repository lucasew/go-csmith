package csmith

import (
	"strings"
	"testing"
)

func TestIsNonReadableWritable(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	rw := &RWDirective{
		NoReadVars:  []*Variable{a},
		NoWriteVars: []*Variable{b},
	}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(rw)
	if !cg.IsNonReadable(a) || cg.IsNonReadable(b) {
		t.Fatal("read")
	}
	if !cg.IsNonWritable(b) || cg.IsNonWritable(a) {
		t.Fatal("write")
	}
}

func TestIsNonReadableMatchResidualSticky(t *testing.T) {
	// Type-nil ban entry: Match residual soft invent was soft-continue then invent readable later.
	// Fair: sticky nonreadable true.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	hole := &Variable{Name: "g_hole"} // Type nil
	good := CreateVariableScalarsSess(testAmbientSession, "g_ok", GetIntType(), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoReadVars: []*Variable{hole, good}})
	if !cg.IsNonReadable(good) {
		t.Fatal("Match residual must fail closed nonreadable, not invent later readable skip")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual IsNonReadable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIVBoundNonWritable(t *testing.T) {
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.AddIVBound(iv, 10)
	if !cg.IsNonWritable(iv) {
		t.Fatal("iv")
	}
	if IsEligibleVar(iv, 0, AccessWrite, cg) {
		t.Fatal("eligible write iv")
	}
	if !IsEligibleVar(iv, 0, AccessRead, cg) {
		t.Fatal("read iv ok")
	}
	cg.RemoveIVBound(iv)
	if cg.IsNonWritable(iv) {
		t.Fatal("removed")
	}
}

func TestIsEligibleNonReadable(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoReadVars: []*Variable{v}})
	if IsEligibleVar(v, 0, AccessRead, cg) {
		t.Fatal("no read")
	}
}

func TestStepHashEmittedInBlock(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	opts.MaxBlockSize = 2
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "func_1_rv", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	// reset sid for stable-ish ids
	currentSession().NextStmID = 0
	b := MakeRandomBlock(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	if !b.EmitStepHash {
		t.Fatal("StepHash+ComputeHash must set EmitStepHash")
	}
	out := b.Output(1)
	if !strings.Contains(out, "step_hash(") {
		t.Fatal(out)
	}
	// StepHash without ComputeHash — no invent step_hash calls
	opts2 := Defaults()
	opts2.StepHashByStmt = true
	opts2.ComputeHash = false
	b2 := MakeRandomBlock(NewRngSess(testAmbientSession, 3), opts2, NewProbabilities(opts2), vs, NewExprTables(opts2), NewStatementThresholdTable(opts2), &cg, false)
	if b2 != nil && b2.EmitStepHash {
		t.Fatal("must not invent EmitStepHash without ComputeHash")
	}
}

func TestMakeRandomForIVBoundDuringBody(t *testing.T) {
	// for body: IV is nonwritable via iv_bounds
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	iv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 2))
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.AddIVBound(iv, 5)
	if IsEligibleVar(iv, 0, AccessWrite, cg) {
		t.Fatal("iv write blocked")
	}
}
