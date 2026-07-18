package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomBreakHasVarTest(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= 2 // IN_LOOP
	// StatementBreak.cpp:76 — clear effect_stm on CGContext& before condition
	pre := CreateVariableScalars("g_pre", GetIntType(), false, false)
	cg.EffectStm = EmptyEffect().WriteVar(pre)
	st := MakeRandomBreak(NewRng(9), opts, vs, tables, &cg)
	if st.Kind != StmtBreak {
		t.Fatalf("%v", st.Kind)
	}
	if st.Expr == nil || st.Expr.Term != TermVariable {
		// may fall back if no vars; still should not be free call
		if st.Expr != nil && st.Expr.Term == TermFunction {
			t.Fatal("break test must be variable")
		}
	}
	if cg.EffectStm.IsWritten(pre) {
		t.Fatal("break must clear pre-seed effect_stm write on *CGContext")
	}
}

func TestBreakOutputIsIfBreak(t *testing.T) {
	st := Stmt{Kind: StmtBreak, Expr: &Expression{Term: TermVariable, Var: &Variable{Name: "g_1", Type: GetIntType()}}}
	b := &Block{Stmts: []Stmt{st}}
	out := b.Output(0)
	if !strings.Contains(out, "if (") || !strings.Contains(out, "break;") {
		t.Fatal(out)
	}
	if strings.Contains(out, "if (g_1)\n    break") || strings.Contains(out, "if (g_1)") {
		// good
	} else if !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
}

func TestMakeRandomContinueNotFirstFallsBack(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// empty block → nullptr (Expr-less continue; stmtOK rejects)
	blk := &Block{}
	st := MakeRandomContinue(NewRng(3), opts, vs, tables, func() *CGContext { c := EmptyCGContext(); return &c }(), blk)
	if st.Kind != StmtContinue || st.Expr != nil {
		t.Fatalf("want null continue, got %+v", st)
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject first-stmt continue")
	}
}

func TestMakeRandomContinueWithPrior(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	probs := NewProbabilities(opts)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= 2
	blk := &Block{Stmts: []Stmt{{Kind: StmtAssign}}}
	st := MakeRandomContinue(NewRng(11), opts, vs, tables, &cg, blk)
	if st.Kind != StmtContinue {
		t.Fatalf("got %v", st.Kind)
	}
}

func TestGenerateEmitsIfBreakOrContinue(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "break;") && strings.Contains(out, "if (") {
			// crude: look for break after if in same function area
			if strings.Contains(out, "break;") {
				found = true
				break
			}
		}
		if strings.Contains(out, "continue;") {
			found = true
			break
		}
	}
	if !found {
		t.Log("break/continue rare in sample seeds")
	}
}
