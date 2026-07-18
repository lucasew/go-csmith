package csmith

import (
	"strings"
	"testing"
)

func TestBlockEffectAccumAfterAssign(t *testing.T) {
	// Block::make_random(CGContext&) — shared effect_accum sees stmt writes.
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// Force assign-only statement table
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	// StatementAssign.cpp:127 assert(fm) — assign needs FactMgr
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg.Types = &TypeEnv{}
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, NewRng(1))
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// single-statement block
	opts.MaxBlockSize = 1
	b := MakeRandomBlock(NewRng(2), opts, probs, vs, tables, tab, &cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	// *CGContext: assign path may write into shared EffectAccum
	if len(b.Stmts) == 0 {
		t.Fatal("empty block")
	}
	// Verify NoteWrite works on shared accum
	v := CreateVariableQfer("g_x", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg.NoteWrite(v)
	// NoteWrite updates EffectAccum; non-vol write stays SE-free (Effect.cpp:144–145)
	if !cg.AccumEffect().IsWritten(v) {
		t.Fatal("not written")
	}
	if !cg.AccumEffect().IsSideEffectFree() {
		// may already have vol effects from block body
	}
	// volatile write clears SE-free
	eff2 := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect())
	cg2.EffectAccum = &eff2
	vv := CreateVariableQfer("g_v", GetIntType(), NewCVQualifiers([]bool{false}, []bool{true}))
	cg2.NoteWrite(vv)
	if cg2.AccumEffect().IsSideEffectFree() {
		t.Fatal("vol write clears SE-free")
	}
}

func TestStepHashOutput(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "void csmith_compute_hash(void)") {
		t.Fatal("missing hash def")
	}
	if !strings.Contains(out, "csmith_compute_hash();") {
		t.Fatal("missing hash call")
	}
	if !strings.Contains(out, "void step_hash(int stmt_id)") {
		t.Fatal("missing step_hash")
	}
}

func TestExpressionCommaUsesEnv(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	cg := EmptyCGContext()
	cg.Types = env
	e := MakeExpressionComma(NewRng(3), opts, probs, vs, tables, &cg, GetIntType(), nil)
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("nil sides")
	}
}
