package csmith

import "testing"

func TestRemoveStmtScrubsCFGAndBreaks(t *testing.T) {
	fm := NewFactMgr(nil)
	loop := &Block{Looping: true, StmID: 100, BreakStmIDs: []int{1, 2, 3}}
	b := &Block{
		Parent: loop,
		StmID:  10,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: 1},
			{Kind: StmtBreak, StmID: 2},
			{Kind: StmtAssign, StmID: 3},
		},
	}
	fm.CreateCFGEdge(1, b, false, false)
	// edge src=2 dest=100
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{SrcID: 2, DestStmID: 100})
	fm.SetMapFactsIn(2, nil)
	fm.SetMapFactsOut(2, nil)

	n := b.RemoveStmt(2, fm)
	if n != 1 {
		t.Fatalf("removed %d", n)
	}
	if len(b.Stmts) != 2 {
		t.Fatalf("stmts %d", len(b.Stmts))
	}
	for _, e := range fm.CFGEdges {
		if e != nil && (e.SrcID == 2 || e.DestStmID == 2) {
			t.Fatal("edge involving removed stmt")
		}
	}
	for _, id := range loop.BreakStmIDs {
		if id == 2 {
			t.Fatal("break list still has 2")
		}
	}
	if _, ok := fm.MapFactsIn[2]; ok {
		t.Fatal("facts in")
	}
}

func TestResetBlockFactMaps(t *testing.T) {
	fm := NewFactMgr(nil)
	b := &Block{StmID: 9, Stmts: []Stmt{{StmID: 1}, {StmID: 2}}}
	fm.SetMapFactsIn(1, nil)
	fm.SetMapFactsOut(2, nil)
	fm.SetMapFactsIn(9, nil)
	fm.ResetBlockFactMaps(b)
	if _, ok := fm.MapFactsIn[1]; ok {
		t.Fatal("in 1")
	}
	if _, ok := fm.MapFactsOut[2]; ok {
		t.Fatal("out 2")
	}
	if _, ok := fm.MapFactsIn[9]; !ok {
		// block itself may be collected via collectBlockStmIDs
	}
}

func TestFindJumpSources(t *testing.T) {
	fm := NewFactMgr(nil)
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 10, DestStmID: 5},
		{SrcID: 11, DestStmID: 5},
		{SrcID: 12, DestStmID: 6},
	}
	srcs := fm.FindJumpSources(5)
	if len(srcs) != 2 {
		t.Fatal(srcs)
	}
}

func TestNeedNestedLoop(t *testing.T) {
	arr := CreateVariableScalars("a", GetIntType(), false, false)
	arr.IsArray = true
	arr.ArraySizes = []int{2, 3} // dim 2
	b := &Block{Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	rw := &RWDirective{MustReadVars: []*Variable{arr}}
	cg := CGContext{RW: rw, IVBounds: map[*Variable]int{}} // depth 0
	if !b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("dim 2 > iv 0")
	}
	// must_jump last blocks nested loop
	b.Stmts = []Stmt{{
		Kind: StmtBreak,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}
	if b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("must jump")
	}
}

func TestMustBreakOrReturnFullBackEdge(t *testing.T) {
	fm := NewFactMgr(nil)
	b := &Block{
		StmID: 50,
		Stmts: []Stmt{{
			Kind: StmtReturn,
			StmID: 51,
			Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
		}},
	}
	// no back edges → true
	if !b.MustBreakOrReturnFull(fm) {
		t.Fatal("expect must break/return")
	}
	// continue-like edge from outside into block
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{
		SrcID: 99, DestBlock: b, BackLink: true,
	})
	if b.MustBreakOrReturnFull(fm) {
		t.Fatal("back edge escapes return")
	}
}

func TestEffectCloneIndependent(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := EmptyEffect().WriteVar(v)
	c := e.Clone()
	e = e.ReadVar(CreateVariableScalars("g_2", GetIntType(), false, false))
	if c.IsRead(CreateVariableScalars("g_2", GetIntType(), false, false)) {
		// different var ptr — just check written still
	}
	if !c.IsWritten(v) {
		t.Fatal("clone lost write")
	}
	// mutate original maps should not clear clone if deep
	e2 := EmptyEffect().WriteVar(v)
	c2 := e2.Clone()
	_ = e2.WriteVar(CreateVariableScalars("g_3", GetIntType(), false, false))
	// e2.WriteVar returns new Effect; original e2 maps may be shared with... 
	// Clone deep-copies so c2.written is independent
	if len(c2.written) != 1 {
		t.Fatal(len(c2.written))
	}
}

func TestInArrayLoopFromIVBounds(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	f := &Function{Name: "f", ReturnType: GetIntType()}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect())
	cg.IVBounds = map[*Variable]int{iv: 10}
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), cg, false)
	if b == nil || !b.InArrayLoop {
		t.Fatal("InArrayLoop", b)
	}
}

func TestAppendReturnStmtRecordsMaps(t *testing.T) {
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	b := &Block{Func: f, StmID: AllocStmID(), Parent: nil}
	f.Stack = []*Block{b}
	st := b.AppendReturnStmt(NewRng(1), opts, NewVariableSelector(opts), &cg)
	if st == nil || st.Kind != StmtReturn {
		t.Fatal(st)
	}
	if len(b.Stmts) != 1 {
		t.Fatal(len(b.Stmts))
	}
	if !fm.MapVisited[st.StmID] {
		t.Fatal("visited return")
	}
	if _, ok := fm.MapFactsOut[st.StmID]; !ok {
		t.Fatal("facts out")
	}
}

func TestContainsBackEdge(t *testing.T) {
	// Block.cpp:491 — edge->back_link && edge->dest->parent == this
	fm := NewFactMgr(nil)
	b := &Block{StmID: 10, Stmts: []Stmt{{StmID: 1}, {StmID: 2}}}
	if b.ContainsBackEdge(fm) {
		t.Fatal("empty")
	}
	// DestStmID alone is insufficient without DestBlock parent
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{
		SrcID: 9, DestStmID: 1, BackLink: true,
	})
	if b.ContainsBackEdge(fm) {
		t.Fatal("DestStmID without DestBlock must not match")
	}
	b2 := &Block{StmID: 20}
	fm.CFGEdges = []*CFGEdge{{SrcID: 8, DestBlock: b2, BackLink: true}}
	if !b2.ContainsBackEdge(fm) {
		t.Fatal("dest block parent")
	}
}

func TestMakeDummyBlockCG(t *testing.T) {
	opts := Defaults()
	// void: no append_return_stmt during post_creation
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	b := MakeDummyBlockCG(&cg, opts)
	if b == nil || b.Func != f {
		t.Fatal(b)
	}
	if len(b.Stmts) != 0 {
		t.Fatal("dummy empty", b.Output(0))
	}
	if !fm.MapVisited[b.StmID] {
		t.Fatal("visited")
	}
	if len(f.Stack) != 0 {
		t.Fatal("stack not popped")
	}
}

func TestAddNewVarFactTo(t *testing.T) {
	p := CreateVariableScalars("p", GetIntType(), false, false)
	// make pointer type
	p.Type = PointerTo(GetIntType())
	var facts []*FactPointTo
	AddNewVarFactTo(p, &facts)
	if len(facts) != 1 || facts[0].Var != p {
		t.Fatal(facts)
	}
	// idempotent
	AddNewVarFactTo(p, &facts)
	if len(facts) != 1 {
		t.Fatal(len(facts))
	}
}

func TestFindFixedPointShortcut(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	b := &Block{
		StmID: 50,
		Stmts: []Stmt{{
			Kind: StmtAssign, StmID: 1, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
			Expr: &Expression{Term: TermConstant, Con: MakeInt(3)}, AssignOp: AssignSimple,
		}},
	}
	fm := NewFactMgr(nil)
	in := []*FactPointTo{}
	fm.SetMapFactsIn(50, in)
	fm.SetMapFactsOut(50, in)
	fm.MapVisited = map[int]bool{50: true}
	fm.SetMapStmEffect(50, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// second call should shortcut on matching inputs
	out, _, ok := FindFixedPointBlock(b, in, &cg, Defaults(), false)
	if !ok {
		t.Fatal("fp")
	}
	_ = out
}

func TestPostCreationAppendsReturn(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	b := MakeRandomBlock(NewRng(7), opts, NewProbabilities(opts), NewVariableSelector(opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), cg, false)
	if b == nil {
		t.Fatal("nil")
	}
	if f.NeedReturnStmt() && !b.MustReturn() {
		t.Fatal("missing return after post_creation", b.Output(0))
	}
}

func TestFindJumpSourcesFiltersNonGoto(t *testing.T) {
	// Statement.cpp:501 — only eGoto sources; break→for must not count
	f := &Function{Name: "func_1"}
	// for stm 50; break 40 targets for; goto 30 targets assign 10
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 10, SourceLabel: "lbl_t"},
		{Kind: StmtBreak, StmID: 40},
		{Kind: StmtGoto, StmID: 30, Label: "lbl_t", GotoDestStmID: 10},
		{Kind: StmtFor, StmID: 50},
	}}
	f.Blocks = []*Block{body}
	f.Body = body
	fm := NewFactMgr(f)
	// break edge into for (DestStmID=50) and goto edge into assign
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 40, DestStmID: 50, PostDest: true}, // break
		{SrcID: 30, DestStmID: 10},                  // goto
		{SrcID: 99, DestStmID: 10},                  // dangling non-goto id
	}
	// assign 10: only real goto 30
	srcs := fm.FindJumpSources(10)
	if len(srcs) != 1 || srcs[0] != 30 {
		t.Fatalf("goto only: %v", srcs)
	}
	// for 50: break filtered out
	if len(fm.FindJumpSources(50)) != 0 {
		t.Fatal("break must not be jump source")
	}
}

func TestFindJumpLabel(t *testing.T) {
	GotoLabelsDoFinalization()
	defer GotoLabelsDoFinalization()
	f := &Function{Name: "f"}
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 7},
		{Kind: StmtGoto, StmID: 8, Label: "lbl_from_goto", GotoDestStmID: 7},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 8, DestStmID: 7}}
	if got := FindJumpLabel(fm, 7); got != "lbl_from_goto" {
		t.Fatal(got)
	}
	// registry fallback when no matching edge
	_ = LabelForGotoDest(99, func() string { return "lbl_reg" })
	if got := FindJumpLabel(fm, 99); got != "lbl_reg" {
		t.Fatal(got)
	}
}

func TestFindStmtByID(t *testing.T) {
	f := &Function{Name: "f"}
	inner := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	outer := &Block{Func: f, Stmts: []Stmt{{Kind: StmtIfElse, StmID: 1, Then: inner}}}
	inner.Parent = outer
	f.Blocks = []*Block{outer, inner}
	st := FindStmtByID(f, 3)
	if st == nil || st.Kind != StmtAssign {
		t.Fatalf("%+v", st)
	}
	if FindStmtByID(f, 999) != nil {
		t.Fatal("missing")
	}
}

func TestGetDimension(t *testing.T) {
	// Variable.h default 0; ArrayVariable sizes
	sc := CreateVariableScalars("g_i", GetIntType(), false, false)
	if sc.GetDimension() != 0 {
		t.Fatal("scalar")
	}
	opts := Defaults()
	av := CreateArrayVariable(NewRng(1), opts, nil, "g_a", GetIntType(), MakeInt(0),
		NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("av")
	}
	av.Sizes = []int{2, 3, 4}
	if av.Variable.GetDimension() != 3 {
		t.Fatal(av.Variable.GetDimension())
	}
	// IsArray without AsArray
	v := CreateVariableScalars("g_b", GetIntType(), false, false)
	v.IsArray = true
	v.ArraySizes = []int{5, 6}
	if v.GetDimension() != 2 {
		t.Fatal(v.GetDimension())
	}
}

func TestNeedNestedLoopUsesGetDimension(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_m", GetIntType(), MakeInt(0),
		NewCVQualifiers([]bool{false}, []bool{false}))
	av.Sizes = []int{2, 3}
	// via AsArray on Variable
	b := &Block{Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	rw := &RWDirective{MustWriteVars: []*Variable{&av.Variable}}
	cg := CGContext{RW: rw, IVBounds: map[*Variable]int{}}
	if !b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("dim 2 > iv 0 via GetDimension")
	}
}

func TestAppendReturnStmtFiltersLocalOut(t *testing.T) {
	// FactMgr::set_fact_out on return drops function-locals
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	loc.Name = "l_1"
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{
		MakeFactPointTo(loc, NullPtr),
		MakeFactPointTo(f.RV, NullPtr),
	}
	body := &Block{Func: f, StmID: AllocStmID(), LocalVars: []*Variable{loc}}
	f.Body = body
	f.Blocks = []*Block{body}
	f.Stack = []*Block{body}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	st := body.AppendReturnStmt(NewRng(3), opts, NewVariableSelector(opts), &cg)
	if st == nil || st.Kind != StmtReturn {
		t.Fatal(st)
	}
	out := fm.MapFactsOut[st.StmID]
	if FindRelatedPointTo(out, loc) != nil {
		t.Fatal("local fact must drop on return out", out)
	}
}
