package csmith

import (
	"strings"
	"testing"
)

func TestFunctionIsVarOnStack(t *testing.T) {
	ClearError()
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
	if HasError() {
		t.Fatal("complete visibility paths must not sticky")
	}
	ClearError()
	// Function.cpp:194 — find_variable_in_set uses Variable::match (not ==).
	// Aggregate local match(field) via has_field_var → field is on-stack so
	// mark_func_end / eReturn set_fact_out garbage field pointees (seed-30).
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	agg := CreateVariableScalars("l_s", st, false, false)
	agg.CreateFieldVars()
	if len(agg.FieldVars) == 0 {
		t.Fatal("need field")
	}
	fld := agg.FieldVars[0]
	blk2 := &Block{Func: f, LocalVars: []*Variable{agg}}
	if !f.IsVarOnStack(agg, blk2) {
		t.Fatal("aggregate local must be on stack")
	}
	if !f.IsVarOnStack(fld, blk2) {
		t.Fatal("field of stack aggregate must IsVarOnStack (C++ find_variable_in_set+match)")
	}
	if HasError() {
		t.Fatal("field check must not sticky")
	}
	ClearError()
	// Type-nil param soft invent was soft-continue then invent on-stack later good.
	// Fair: sticky fail closed not-on-stack (StackScanComplete / Param hole).
	f.Param = []*Variable{&Variable{Name: "p_hole"}, p}
	if f.IsVarOnStack(p, blk) {
		t.Fatal("Param hole must fail closed not-on-stack, not invent later param match")
	}
	if !HasError() {
		t.Fatal("Param hole Function.IsVarOnStack must SetError sticky")
	}
	ClearError()
	f.Param = []*Variable{p}
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
	if !HasError() {
		t.Fatal("IsVarOnStack incomplete stack must SetError sticky")
	}
	ClearError()
	facts := []*FactPointTo{MakeFactPointTo(p, loc)}
	MarkFuncEndOnFacts(&facts, f, blk)
	if FactsComplete(facts) {
		t.Fatal("MarkFuncEndOnFacts must clear on incomplete stack, not invent live pointee", facts)
	}
	if !HasError() {
		t.Fatal("MarkFuncEndOnFacts incomplete stack must SetError sticky")
	}
	ClearError()
	// facts slice always live; sticky no invent soft-skip mark past hole
	MarkFuncEndOnFacts(nil, f, blk)
	if !HasError() {
		t.Fatal("nil facts MarkFuncEndOnFacts must SetError sticky")
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
	ClearError()
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
	if HasError() {
		t.Fatal("complete IsVarOOS paths must not sticky")
	}
	ClearError()
}

func TestAddBackReturnFacts(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	retFacts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	st := Stmt{Kind: StmtReturn, StmID: 42}
	fm.SetMapFactsOutPair(42, retFacts, []*FactUnion{})
	body := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		st,
	}}
	var facts []*FactPointTo
	var unions []*FactUnion
	if !AddBackReturnFacts(body, fm, &facts, &unions) || len(facts) != 1 || facts[0].Var != p {
		t.Fatal(facts)
	}
	// return StmID unset fails closed sticky (no invent soft-merge MapFactsOut[0])
	var facts0 []*FactPointTo
	var unions0 []*FactUnion
	if AddBackReturnFacts(&Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: IncompleteStmID}}}, fm, &facts0, &unions0) || FactsComplete(facts0) {
		t.Fatal("return StmID 0 must fail closed", facts0)
	}
	if !HasError() {
		t.Fatal("return StmID 0 must SetError sticky")
	}
	ClearError()
}

// Statement.cpp:528 — merge_facts full FactVec joins eUnionWrite from return outs.
// Body exit last=0 + early-return last=1 → BOTTOM (seed-123 g_135 lattice).
func TestAddBackReturnFactsMergesUnionWrite(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	uv.CreateFieldVars()
	// body-out style entry: last field 0; return wrote field 1
	bodyU := []*FactUnion{MakeFactUnion(uv, 0)}
	retU := []*FactUnion{MakeFactUnion(uv, 1)}
	fm.SetMapFactsOutPair(7, []*FactPointTo{}, retU)
	body := &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 7}}}
	facts := []*FactPointTo{}
	unions := CloneUnionFactSliceDeep(bodyU)
	if !AddBackReturnFacts(body, fm, &facts, &unions) {
		t.Fatal("add_back must succeed")
	}
	got := FindRelatedUnion(unions, uv)
	if got == nil || !got.IsBottom() {
		t.Fatalf("0 join 1 must BOTTOM, got %v", got)
	}
	if HasError() {
		t.Fatal("complete path must not sticky")
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
	fm.MapUnionFactsOut = map[int][]*FactUnion{
		10: {},
		20: {},
	}
	body := &Block{Stmts: []Stmt{
		{Kind: StmtReturn, StmID: 10},
		{Kind: StmtReturn, StmID: 20},
	}}
	var facts []*FactPointTo
	var unions []*FactUnion
	if AddBackReturnFacts(body, fm, &facts, &unions) || FactsComplete(facts) {
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
	fm2.MapUnionFactsOut = map[int][]*FactUnion{
		30: {},
		40: {},
	}
	body2 := &Block{Stmts: []Stmt{{
		Kind: StmtIfElse,
		Then: &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 30}}},
		Else: &Block{Stmts: []Stmt{{Kind: StmtReturn, StmID: 40}}},
	}}}
	var facts2 []*FactPointTo
	var unions2 []*FactUnion
	if AddBackReturnFacts(body2, fm2, &facts2, &unions2) || FactsComplete(facts2) {
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
	// facts always live; sticky (no invent soft-skip OOS cleanup past hole)
	// empty vars is complete no-op
	UpdateFactsForOOSVars(nil, nil)
	if !HasError() {
		t.Fatal("nil facts UpdateFactsForOOSVars must SetError sticky")
	}
	ClearError()
	var empty []*FactPointTo
	UpdateFactsForOOSVars(nil, &empty)
	if HasError() {
		t.Fatal("empty vars UpdateFactsForOOSVars must not sticky")
	}
	ClearError()
	// Match residual: Type-nil OOS var soft invent was soft-continue keep later fact.
	// Fair: sticky IncompleteFactSlice.
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), false, false)
	facts2 := []*FactPointTo{MakeFactPointTo(p2, NullPtr)}
	holeOOS := &Variable{Name: "l_hole", Type: nil, FieldVars: nil}
	// FieldVarsComplete for Type-nil with empty FieldVars is true (no nil holes)
	UpdateFactsForOOSVars([]*Variable{holeOOS}, &facts2)
	if FactsComplete(facts2) {
		t.Fatal("Match residual UpdateFactsForOOSVars must fail closed incomplete", facts2)
	}
	if !HasError() {
		t.Fatal("Match residual UpdateFactsForOOSVars must SetError sticky")
	}
	ClearError()
}

func TestOutputCommentLine(t *testing.T) {
	ClearError()
	s := OutputCommentLine("hello", false, false)
	if s != "/* hello */\n" {
		t.Fatal(s)
	}
	if OutputCommentLine("x", true, false) != "\n" {
		t.Fatal("quiet")
	}
	// empty comment sticky — no invent "/*  */"
	ClearError()
	if OutputCommentLine("", false, false) != "" {
		t.Fatal("empty comment must fail closed")
	}
	if !HasError() {
		t.Fatal("empty comment must SetError sticky")
	}
	ClearError()
}

func TestFunctionOutputSeparator(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	out := f.Output()
	if !strings.Contains(out, "/* ------------------------------------------ */") {
		t.Fatal(out)
	}
}
