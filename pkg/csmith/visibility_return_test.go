package csmith

import (
	"strings"
	"testing"
)

func TestFunctionIsVarOnStack(t *testing.T) {
	f := &Function{Name: "f"}
	p := CreateVariableScalars("p_1", GetIntType(), false, false)
	f.Param = []*Variable{p}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	if !f.IsVarOnStack(p, blk) || !f.IsVarOnStack(loc, blk) {
		t.Fatal("stack")
	}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if f.IsVarOnStack(g, blk) {
		t.Fatal("global")
	}
	if !f.IsVarVisible(g, blk) {
		t.Fatal("global visible")
	}
}

func TestStackScanCompleteHoleFailClosed(t *testing.T) {
	// soft invent: IsVarOnStack false past LocalVars hole → leave stack pointees live
	// fair: StackScanComplete false; MarkFuncEndOnFacts / RemoveFunctionLocal clear sticky
	ClearError()
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	if f.StackScanComplete(blk) {
		t.Fatal("LocalVars hole must be incomplete stack scan")
	}
	if f.IsVarOnStack(loc, blk) {
		t.Fatal("IsVarOnStack must not invent membership past hole")
	}
	facts := []*FactPointTo{MakeFactPointTo(p, loc)}
	MarkFuncEndOnFacts(&facts, f, blk)
	if FactsComplete(facts) {
		t.Fatal("MarkFuncEndOnFacts must clear on incomplete stack, not invent live pointee", facts)
	}
	if !HasError() {
		t.Fatal("MarkFuncEndOnFacts incomplete stack must SetError sticky")
	}
	ClearError()
	facts2 := []*FactPointTo{MakeFactPointTo(p, loc)}
	out := RemoveFunctionLocalFactsAt(facts2, f, blk)
	if FactsComplete(out) {
		t.Fatal("RemoveFunctionLocalFactsAt must fail closed on incomplete stack", out)
	}
	if !HasError() {
		t.Fatal("RemoveFunctionLocalFactsAt incomplete stack must SetError sticky")
	}
	// Block stack scan
	if blk.StackScanComplete() {
		t.Fatal("Block.StackScanComplete hole")
	}
	ClearError()
}

func TestFunctionIsVarOOS(t *testing.T) {
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	inner := &Block{Func: f, LocalVars: []*Variable{loc}}
	// at function body root with empty parent chain, loc not on stack
	if !f.IsVarOOS(loc, nil) {
		// need loc in Blocks
		f.Blocks = []*Block{inner}
		if !f.IsVarOOS(loc, nil) {
			t.Fatal("oos")
		}
	}
	// visible when parent is inner
	if f.IsVarOOS(loc, inner) {
		t.Fatal("visible not oos")
	}
}

func TestAddBackReturnFacts(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	retFacts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	st := Stmt{Kind: StmtReturn, StmID: 42}
	fm.SetMapFactsOut(42, retFacts)
	body := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		st,
	}}
	var facts []*FactPointTo
	if !AddBackReturnFacts(body, fm, &facts) || len(facts) != 1 || facts[0].Var != p {
		t.Fatal(facts)
	}
	// return StmID 0 fails closed sticky (no invent soft-merge MapFactsOut[0])
	var facts0 []*FactPointTo
	if AddBackReturnFacts(&Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 0}}}, fm, &facts0) || FactsComplete(facts0) {
		t.Fatal("return StmID 0 must fail closed", facts0)
	}
	if !HasError() {
		t.Fatal("return StmID 0 must SetError sticky")
	}
	ClearError()
}

func TestAddBackReturnFactsIncompleteStopsWalk(t *testing.T) {
	// incomplete map_facts_out fails closed sticky — no invent merge of later returns
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	// first return incomplete out
	fm.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointTo(p, NullPtr), nil},
		20: {MakeFactPointTo(q, NullPtr)},
	}
	body := &Block{Stmts: []Stmt{
		{Kind: StmtReturn, StmID: 10},
		{Kind: StmtReturn, StmID: 20},
	}}
	var facts []*FactPointTo
	if AddBackReturnFacts(body, fm, &facts) || FactsComplete(facts) {
		t.Fatal("incomplete out must fail closed nil accumulator, not invent later return", facts)
	}
	if !HasError() {
		t.Fatal("incomplete out must SetError sticky")
	}
	ClearError()
	// nested if Then with incomplete return must stop before Else returns
	fm2 := NewFactMgr(f)
	fm2.MapFactsOut = map[int][]*FactPointTo{
		30: {MakeFactPointTo(p, NullPtr), nil},
		40: {MakeFactPointTo(q, NullPtr)},
	}
	body2 := &Block{Stmts: []Stmt{{
		Kind: StmtIfElse,
		Then: &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 30}}},
		Else: &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 40}}},
	}}}
	var facts2 []*FactPointTo
	if AddBackReturnFacts(body2, fm2, &facts2) || FactsComplete(facts2) {
		t.Fatal("nested incomplete must fail closed without inventing Else return", facts2)
	}
	if !HasError() {
		t.Fatal("nested incomplete must SetError sticky")
	}
	ClearError()
}

func TestUpdateFactsForOOSVarsVisibility(t *testing.T) {
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(p, loc),
		MakeFactPointTo(CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false), NullPtr),
	}
	// second fact subject is l_p local-named — Match with l_1? no
	lp := facts[1].Var
	UpdateFactsForOOSVars([]*Variable{loc, lp}, &facts)
	// fact for lp subject removed; p fact remains but pointee loc marked dead
	if len(facts) != 1 || facts[0].Var != p {
		t.Fatal(facts)
	}
	if !facts[0].IsDead() {
		t.Fatal("should mark dead pointee")
	}
	// nil fact hole fails closed sticky — no invent clean filter past hole
	ClearError()
	hole := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	UpdateFactsForOOSVars([]*Variable{loc}, &hole)
	if FactsComplete(hole) {
		t.Fatal("nil fact hole must fail closed", hole)
	}
	if !HasError() {
		t.Fatal("nil fact hole must SetError sticky")
	}
	ClearError()
	// nil OOS var hole fails closed sticky
	ok := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	UpdateFactsForOOSVars([]*Variable{nil}, &ok)
	if FactsComplete(ok) {
		t.Fatal("nil OOS var hole must fail closed", ok)
	}
	if !HasError() {
		t.Fatal("nil OOS var hole must SetError sticky")
	}
	ClearError()
}

func TestOutputCommentLine(t *testing.T) {
	s := OutputCommentLine("hello", false, false)
	if s != "/* hello */\n" {
		t.Fatal(s)
	}
	if OutputCommentLine("x", true, false) != "\n" {
		t.Fatal("quiet")
	}
	// empty comment — no invent "/*  */"
	if OutputCommentLine("", false, false) != "" {
		t.Fatal("empty comment must fail closed")
	}
}

func TestFunctionOutputSeparator(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	out := f.Output()
	if !strings.Contains(out, "/* ------------------------------------------ */") {
		t.Fatal(out)
	}
}
