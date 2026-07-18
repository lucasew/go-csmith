package csmith

import (
	"strings"
	"testing"
)

func TestBlockEffectAccumAfterAssign(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	// Force assign-only statement table
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	cg.Types = &TypeEnv{}
	// single-statement block
	opts.MaxBlockSize = 1
	b := MakeRandomBlock(NewRng(2), opts, probs, vs, tables, tab, cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	// EffectAccum on cg was a copy - the block creates its own
	// Verify NoteWrite works
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect())
	cg2.EffectAccum = &eff
	v := CreateVariableQfer("g_x", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	cg2.NoteWrite(v)
	if cg2.EffectContext().IsSideEffectFree() {
		t.Fatal("should not be SE-free after write")
	}
	if !cg2.EffectContext().IsWritten(v) {
		t.Fatal("not written")
	}
	_ = stmtTab
	_ = b
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
	e := MakeExpressionComma(NewRng(3), opts, probs, vs, tables, cg, GetIntType(), nil)
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("nil sides")
	}
}
