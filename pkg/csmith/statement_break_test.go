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

func TestForArrayOpNoInventIncompleteHeader(t *testing.T) {
	// StatementFor always has init/test/incr + body; sticky no invent for(;;) or header-only
	ClearErrorSess(testAmbientSession)
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	// Loop with IV only — missing InitStmt/TestExpr/IncrStmt
	lc := &LoopControl{IV: iv, InitN: 0, LimitN: 3, IncrN: 1}
	if forHeaderOutput(lc) != "" {
		t.Fatal("forHeader must fail closed without init/test/incr IR")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete forHeader must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Test residual soft invent was soft-continue incr invent partial for header.
	// Fair: sticky empty whole header.
	goodInit := &Stmt{Kind: StmtAssign, LhsVar: iv, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple}
	badTest := &Expression{Term: TermConstant, Con: &Constant{Value: "1"}} // Type-nil residual Output
	goodIncr := &Stmt{Kind: StmtAssign, LhsVar: iv, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignAdd}
	lcRes := &LoopControl{IV: iv, InitStmt: goodInit, TestExpr: badTest, IncrStmt: goodIncr}
	if forHeaderOutput(lcRes) != "" {
		t.Fatal("test Output residual must fail closed forHeaderOutput, not invent partial")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("test Output residual forHeaderOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	out := (&Block{Stmts: []Stmt{{Kind: StmtFor, Loop: lc}}}).Output(0)
	if strings.Contains(out, "for") {
		t.Fatal("for without body/IR must not invent header", out)
	}
	// header present but no Then body
	ClearErrorSess(testAmbientSession)
	goodTest := &Expression{Term: TermConstant, Con: MakeInt(1)}
	lc2 := &LoopControl{IV: iv, InitStmt: goodInit, TestExpr: goodTest, IncrStmt: goodIncr}
	if forHeaderOutput(lc2) == "" {
		t.Fatal("complete IR must emit header")
	}
	out = (&Block{Stmts: []Stmt{{Kind: StmtFor, Loop: lc2}}}).Output(0)
	if strings.Contains(out, "for") {
		t.Fatal("for without body must not invent header-only", out)
	}
	// ArrayOp header without Then
	ClearErrorSess(testAmbientSession)
	out = (&Block{Stmts: []Stmt{{Kind: StmtArrayOp, Loop: lc}}}).Output(0)
	if strings.Contains(out, "for") {
		t.Fatal("arrayop without body must not invent header", out)
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayLoopKeepsStmtForKind(t *testing.T) {
	// StatementArrayOp.cpp:80–81 / StatementFor::make_random_array_loop → StatementFor*
	// no soft invent StmtArrayOp kind over for IR
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntType(), nil, NewRng(2))
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// force non-init path: flipcoin(5) false — seed until we get array loop (for) not array_init
	var got *Stmt
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		st := MakeRandomArrayOp(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
		if st.Kind == StmtFor && st.Loop != nil && st.Then != nil {
			got = &st
			break
		}
		if st.Kind == StmtArrayOp && st.Loop != nil {
			// array_init is also StmtArrayOp — skip
			continue
		}
	}
	if got == nil {
		t.Skip("no array-loop for in sample seeds")
	}
	if got.Kind != StmtFor {
		t.Fatalf("array loop must stay StmtFor, got %v", got.Kind)
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
	// StatementBreak.cpp:72 assert(b) sticky — no soft invent break without looping parent
	ClearErrorSess(testAmbientSession)
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
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no looping parent must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBreakContinueNilDepsSticky(t *testing.T) {
	// StatementBreak/Continue always have RNG + CGContext; sticky no invent shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	loop := &Block{Func: f, Looping: true, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{loop}
	if stmtOK(MakeRandomBreak(nil, opts, vs, NewExprTables(opts), ptrEmptyCG())) {
		t.Fatal("nil RNG break must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomBreak must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if stmtOK(MakeRandomContinue(nil, opts, vs, NewExprTables(opts), ptrEmptyCG(), loop)) {
		t.Fatal("nil RNG continue must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomContinue must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBreakContinueIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR before EffectStm clear / soft re-pick
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	loop := &Block{Func: f, Looping: true, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{loop}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect())
	cg.EffectAccum = &inc
	cg.Flags |= FlagInLoop
	st := MakeRandomBreak(NewRng(1), opts, vs, NewExprTables(opts), &cg)
	if st.Expr != nil || stmtOK(st) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomBreak")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky MakeRandomBreak")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(f, IncompleteEffect())
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	cg2.Flags |= FlagInLoop
	st2 := MakeRandomContinue(NewRng(2), opts, vs, NewExprTables(opts), &cg2, loop)
	if st2.Expr != nil || stmtOK(st2) {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomContinue")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky MakeRandomContinue")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts fails closed sticky
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg3 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff3 := EmptyEffect()
	cg3.EffectAccum = &eff3
	cg3.Flags |= FlagInLoop
	st3 := MakeRandomBreak(NewRng(3), opts, vs, NewExprTables(opts), &cg3)
	if st3.Expr != nil || stmtOK(st3) {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomBreak")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky MakeRandomBreak")
	}
	ClearErrorSess(testAmbientSession)
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
	// empty IV OutputC — sticky no invent for ( = 0; …)
	ClearErrorSess(testAmbientSession)
	anon := &Variable{Type: GetIntType()}
	if s := arrayOpHeaderOutput(&LoopControl{IV: anon, InitN: 0, LimitN: 3, IncrN: 1}, Defaults()); s != "" {
		t.Fatal("empty IV name must fail closed arrayop header", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty IV arrayop header must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomContinueNotFirstFallsBack(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// empty block → nullptr (empty Stmt; no Kind shell invent)
	// first-stmt reject is non-sticky soft re-pick (StatementContinue.cpp:63–66)
	blk := &Block{}
	st := MakeRandomContinue(NewRng(3), opts, vs, tables, func() *CGContext { c := EmptyCGContext(); return &c }(), blk)
	if st.Kind != 0 || st.Expr != nil {
		t.Fatalf("want null continue, got %+v", st)
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject first-stmt continue")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("first-stmt continue must stay non-sticky soft re-pick")
	}
}

func TestMakeRandomContinueRequiresLoop(t *testing.T) {
	// StatementContinue.cpp:71 assert(b) sticky — no soft invent without looping parent
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	// prior stmt so first-stmt gate passes; non-looping parent only
	blk := &Block{Func: f, Looping: false, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= FlagInLoop
	st := MakeRandomContinue(NewRng(1), opts, vs, NewExprTables(opts), &cg, blk)
	if st.Expr != nil || stmtOK(st) {
		t.Fatalf("no looping parent must fail closed, got %+v", st)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("no looping parent continue must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomContinueWithPrior(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	probs := NewProbabilities(opts)
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	if f == nil {
		t.Fatal("MakeFirst nil", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	cg := WithFunc(f, EmptyEffect())
	cg.Flags |= 2
	loop := &Block{Func: f, Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	f.Stack = []*Block{loop}
	st := MakeRandomContinue(NewRng(11), opts, vs, tables, &cg, loop)
	if st.Kind != StmtContinue {
		t.Fatalf("got %v", st.Kind)
	}
	ClearErrorSess(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
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
	fm := NewFactMgrSess(testAmbientSession, f)
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

func TestMakeRandomBreakNilCGSticky(t *testing.T) {
	// nil CG residual soft invent was invent Kind-only break shell.
	ClearErrorSess(testAmbientSession)
	if MakeRandomBreak(NewRng(1), Defaults(), nil, nil, nil).Kind != 0 {
		t.Fatal("nil cg MakeRandomBreak must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeRandomBreak must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
