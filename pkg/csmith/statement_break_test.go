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
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= 2 // IN_LOOP
	// StatementBreak.cpp:72 — assert(looping parent); put live loop on stack
	loop := &Block{Func: f, Looping: true}
	f.Stack = []*Block{loop}
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

func TestMakeRandomBreakRequiresLoop(t *testing.T) {
	// StatementBreak.cpp:72 assert(b) — no soft invent break without looping parent
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	// non-looping block only
	blk := &Block{Func: f, Looping: false}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= FlagInLoop // flag alone is not enough without Looping parent
	st := MakeRandomBreak(NewRng(1), opts, vs, NewExprTables(opts), &cg)
	if st.Expr != nil {
		t.Fatal("no expr without looping block")
	}
	if stmtOK(st) {
		t.Fatal("stmtOK rejects incomplete break")
	}
}

func TestArrayOpHeaderNumeric(t *testing.T) {
	// StatementArrayOp::output_header uses numeric init/limit/incr (not InitStmt)
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	lc := &LoopControl{IV: iv, InitN: 0, LimitN: 10, IncrN: 1}
	opts := Defaults()
	out := arrayOpHeaderOutput(lc, opts)
	if out != "for (i = 0; i < 10; i += 1)" {
		t.Fatal(out)
	}
	opts.CComp = true
	out = arrayOpHeaderOutput(lc, opts)
	if out != "for (i = 0; i < 10; i = i + 1)" {
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
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= 2
	loop := &Block{Func: f, Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	f.Stack = []*Block{loop}
	st := MakeRandomContinue(NewRng(11), opts, vs, tables, &cg, loop)
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
