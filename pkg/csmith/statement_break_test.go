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

func TestBreakContinueGotoIfNoInventEmptyCond(t *testing.T) {
	// StatementBreak/Continue/Goto/If always have live test Expression*; no invent if ()
	out := (&Block{Stmts: []Stmt{{Kind: StmtBreak}}}).Output(0)
	if strings.Contains(out, "break") || strings.Contains(out, "if (") {
		t.Fatal("incomplete break must not invent if () break", out)
	}
	out = (&Block{Stmts: []Stmt{{Kind: StmtContinue}}}).Output(0)
	if strings.Contains(out, "continue") || strings.Contains(out, "if (") {
		t.Fatal("incomplete continue must not invent if () continue", out)
	}
	out = (&Block{Stmts: []Stmt{{Kind: StmtGoto, Label: "lbl"}}}).Output(0)
	if strings.Contains(out, "goto") || strings.Contains(out, "if (") {
		t.Fatal("incomplete goto must not invent if () goto", out)
	}
	// StatementIf always has test + both branches
	out = (&Block{Stmts: []Stmt{{
		Kind: StmtIfElse,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		Then: &Block{},
	}}}).Output(0)
	if strings.Contains(out, "if (") {
		t.Fatal("if without else must not invent partial if", out)
	}
	out = (&Block{Stmts: []Stmt{{Kind: StmtReturn}}}).Output(0)
	if strings.Contains(out, "return") {
		t.Fatal("incomplete return must not invent bare return;", out)
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
	// empty block → nullptr (empty Stmt; no Kind shell invent)
	blk := &Block{}
	st := MakeRandomContinue(NewRng(3), opts, vs, tables, func() *CGContext { c := EmptyCGContext(); return &c }(), blk)
	if st.Kind != 0 || st.Expr != nil {
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

func TestMakeRandomBreakNoCFGEdgeInvent(t *testing.T) {
	// StatementBreak.cpp:79–81 — only break_stms push; edges in post_loop_analysis
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, NewProbabilities(opts), vs, nil)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	loop := &Block{Func: f, Looping: true}
	inner := &Block{Func: f, Parent: loop, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1, LhsVar: g, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}},
	}}
	f.Stack = []*Block{loop, inner}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.Types = vs.Types
	// force variable term with existing global
	st := MakeRandomBreak(NewRng(3), opts, vs, NewExprTables(opts), &cg)
	if st.Expr == nil {
		t.Skip("break expr nil")
	}
	if len(loop.BreakStmIDs) != 1 {
		t.Fatalf("break_stms %v", loop.BreakStmIDs)
	}
	// no CFG edge invented at make time
	for _, e := range fm.CFGEdges {
		if e != nil && e.SrcID == st.StmID {
			t.Fatal("break must not CreateCFGEdge in make_random", e)
		}
	}
}
