package csmith

import (
	"strings"
	"testing"
)

func TestIsNonReadableWritable(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	rw := &RWDirective{
		NoReadVars:  []*Variable{a},
		NoWriteVars: []*Variable{b},
	}
	cg := EmptyCGContext().WithRW(rw)
	if !cg.IsNonReadable(a) || cg.IsNonReadable(b) {
		t.Fatal("read")
	}
	if !cg.IsNonWritable(b) || cg.IsNonWritable(a) {
		t.Fatal("write")
	}
}

func TestIVBoundNonWritable(t *testing.T) {
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	cg := EmptyCGContext()
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
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoReadVars: []*Variable{v}})
	if IsEligibleVar(v, 0, AccessRead, cg) {
		t.Fatal("no read")
	}
}

func TestStepHashEmittedInBlock(t *testing.T) {
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.MaxBlockSize = 2
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	// reset sid for stable-ish ids
	nextStmID = 0
	b := MakeRandomBlock(NewRng(3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	out := b.Output(1)
	if !strings.Contains(out, "step_hash(") {
		t.Fatal(out)
	}
}

func TestMakeRandomForIVBoundDuringBody(t *testing.T) {
	// for body: IV is nonwritable via iv_bounds
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	iv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), &q, NewRng(2))
	cg := EmptyCGContext()
	cg.AddIVBound(iv, 5)
	if IsEligibleVar(iv, 0, AccessWrite, cg) {
		t.Fatal("iv write blocked")
	}
}
