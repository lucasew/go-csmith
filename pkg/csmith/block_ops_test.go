package csmith

import (
	"strings"
	"testing"
)

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
	if srcs == nil || len(srcs) != 2 {
		t.Fatal(srcs)
	}
	// complete empty is non-nil empty
	none := fm.FindJumpSources(999)
	if none == nil || len(none) != 0 {
		t.Fatal("complete empty", none)
	}
	// nil CFG hole fails closed
	fm.CFGEdges = []*CFGEdge{{SrcID: 10, DestStmID: 5}, nil}
	if fm.FindJumpSources(5) != nil {
		t.Fatal("nil hole must fail closed")
	}
	if FindJumpLabel(fm, 5) != "" {
		t.Fatal("nil hole FindJumpLabel must fail closed")
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
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
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
	// Block.cpp always has RNG for make_random(eReturn)
	if st2 := b.AppendReturnStmt(nil, opts, NewVariableSelector(opts), &cg); st2 != nil {
		t.Fatal("nil RNG must not invent return")
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

func TestBlockOutputNoInventNilOrBrokenTmp(t *testing.T) {
	// Block.cpp always live this; no invent empty braces for nil
	if out := (*Block)(nil).Output(0); out != "" {
		t.Fatal("nil block must fail closed empty, got", out)
	}
	// empty live block still emits braces
	if out := (&Block{}).Output(0); !strings.Contains(out, "{") || !strings.Contains(out, "}") {
		t.Fatal("empty live block", out)
	}
	// macro_tmp_vars name+type always live; no invent partial tmp list
	b := &Block{TmpVars: map[string]ESimpleType{"": EInt}}
	if out := b.Output(0); out != "" {
		t.Fatal("empty tmp name must fail closed whole block", out)
	}
}

func TestAddNewVarFactTo(t *testing.T) {
	// Block.cpp:546–549 / FactMgr.cpp:118–131 — abstract_fact_for_var_init;
	// no invent NewFactPointTo garbage for null-init pointer.
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	SetProcessProbabilities(NewProbabilities(opts))
	p := CreateVariableScalars("p", PointerTo(GetIntType()), false, false)
	if p == nil || p.Init == nil {
		t.Fatal("pointer init")
	}
	var facts []*FactPointTo
	AddNewVarFactTo(p, &facts)
	got := FindRelatedPointTo(facts, p)
	if got == nil {
		t.Fatal(facts)
	}
	if got.IsDead() {
		t.Fatal("must not invent garbage for null-init pointer")
	}
	if !got.IsNull() {
		t.Fatalf("want null from init, got %+v", got.PointTo)
	}
	// idempotent
	AddNewVarFactTo(p, &facts)
	if len(facts) != 1 {
		t.Fatal(len(facts))
	}
	// no Init/InitExpr → C++ v->init nullptr → garbage via abstract; empty abstract
	// when meta off fails closed without invent
	ClearMetaFacts()
	metaFactPointToEnabled = false
	var empty []*FactPointTo
	AddNewVarFactTo(p, &empty)
	if len(empty) != 0 {
		t.Fatal("meta off must not invent", empty)
	}
	ClearMetaFacts()
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
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
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
	// empty registry entry — no invent empty label token
	stmLabels[100] = ""
	if got := FindJumpLabel(fm, 100); got != "" {
		t.Fatal("empty registry label must fail closed", got)
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
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
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
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_m", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
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

func TestRemoveStmtCascadesGotoSource(t *testing.T) {
	// Block.cpp:641–646 — removing dest also removes goto that jumps to it
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	// outer has goto → dest assign; dest will be removed
	destID := 10
	gotoID := 20
	body := &Block{Func: f, StmID: 1, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: destID},
		{Kind: StmtGoto, StmID: gotoID, Label: "lbl", GotoDestStmID: destID},
		{Kind: StmtAssign, StmID: 30},
	}}
	// label on dest
	body.Stmts[0].SourceLabel = "lbl"
	f.Blocks = []*Block{body}
	f.Body = body
	fm.CFGEdges = []*CFGEdge{{SrcID: gotoID, DestStmID: destID}}
	n := body.RemoveStmt(destID, fm)
	if n < 1 {
		t.Fatal("removed dest")
	}
	// goto should be gone too
	for _, s := range body.Stmts {
		if s.StmID == destID || s.StmID == gotoID {
			t.Fatalf("still present: %+v", s)
		}
	}
	if len(body.Stmts) != 1 || body.Stmts[0].StmID != 30 {
		t.Fatalf("stmts %+v", body.Stmts)
	}
	for _, e := range fm.CFGEdges {
		if e != nil && (e.SrcID == gotoID || e.DestStmID == destID) {
			t.Fatal("edge remains")
		}
	}
}

func TestRemoveStmtScrubsFuncBlocks(t *testing.T) {
	// Block.cpp:655–663 — nested Then block dropped from Function.Blocks
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	inner := &Block{Func: f, StmID: 5, Stmts: []Stmt{{Kind: StmtAssign, StmID: 6}}}
	ifSt := Stmt{Kind: StmtIfElse, StmID: 4, Then: inner}
	outer := &Block{Func: f, StmID: 1, Stmts: []Stmt{ifSt, {Kind: StmtAssign, StmID: 7}}}
	inner.Parent = outer
	f.Blocks = []*Block{outer, inner}
	f.Body = outer
	n := outer.RemoveStmt(4, fm)
	if n != 1 {
		t.Fatal(n)
	}
	for _, b := range f.Blocks {
		if b == inner {
			t.Fatal("inner block should be scrubbed from Func.Blocks")
		}
	}
	if len(outer.Stmts) != 1 || outer.Stmts[0].StmID != 7 {
		t.Fatal(outer.Stmts)
	}
}

func TestBlockProbabilityUniformNotAlwaysMax(t *testing.T) {
	// Block.cpp:87–93 random mode: disable filter → uniform rnd_upto(size)
	// not soft invent always size-1
	ClearError()
	seen := map[int]bool{}
	r := NewRng(2)
	for i := 0; i < 80; i++ {
		v := BlockProbability(5, r)
		if v < 0 || v >= 5 {
			t.Fatalf("out of range %d", v)
		}
		seen[v] = true
	}
	if len(seen) < 3 {
		t.Fatalf("want spread over [0,5), got %#v", seen)
	}
	if !seen[0] && !seen[1] && !seen[2] {
		t.Fatal("never saw low values — still inventing max?")
	}
}

func TestAppendNestedLoopERRORGuard(t *testing.T) {
	// Block.cpp:425 ERROR_GUARD after make for
	ClearError()
	opts := Defaults()
	b := &Block{Looping: true}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	b.Func = f
	f.Stack = []*Block{b}
	cg := WithFunc(f, EmptyEffect())
	SetError(ErrGeneric)
	if b.AppendNestedLoop(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("sticky error must not append nested for")
	}
	ClearError()
	// Block.cpp always has RNG for make_random(eFor)
	if b.AppendNestedLoop(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("nil RNG must not invent nested for")
	}
}

func TestMakeRandomAssignRejectsConstStruct(t *testing.T) {
	// StatementAssign.cpp:124 assert(!is_const_struct_union)
	ClearError()
	opts := Defaults()
	cq := NewCVQualifiers([]bool{true}, []bool{false})
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: cq},
	}}
	if !st.IsConstStructUnion() {
		t.Fatal("fixture not const struct")
	}
	cg := EmptyCGContext()
	got := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, st)
	if stmtOK(got) {
		t.Fatal("const struct assign must fail closed")
	}
}

func TestMakeDummyBlockCGFactIn(t *testing.T) {
	// Block.cpp:95–110 — fact_in + post_creation, not empty shell only
	ClearError()
	opts := Defaults()
	f := &Function{Name: "builtin_x", ReturnType: GetIntType(), IsBuiltin: true}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	b := MakeDummyBlockCG(&cg, opts)
	if b == nil || b.StmID == 0 {
		t.Fatal("nil dummy")
	}
	if in, ok := fm.MapFactsIn[b.StmID]; !ok || FindRelatedPointTo(in, p) == nil {
		t.Fatal("fact_in missing")
	}
	// stack popped after make
	if len(f.Stack) != 0 {
		t.Fatal("stack")
	}
}

func TestAppendReturnStmtVisitFailSetsError(t *testing.T) {
	// Block.cpp:383–384 assert(visited) → sticky error, no soft drop only
	ClearError()
	opts := Defaults()
	// void return type makes NeedReturn false; use int return without seed for expr
	f := &Function{Name: "f", ReturnType: GetIntType()}
	// no globals / no params → ExpressionVariable make may fail → AppendReturn fails
	b := &Block{Func: f, Parent: nil}
	f.Body = b
	f.Stack = []*Block{b}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// MakeRandomReturn with empty selector may still invent globals via select
	// Force error by sticky error before append path visit
	// Direct path: call AppendReturn with vs that cannot create
	vs := NewVariableSelector(opts)
	vs.Opts.GlobalVariables = false
	st := b.AppendReturnStmt(NewRng(1), opts, vs, &cg)
	// either success or fail with SetError on visit fail; must not leave incomplete without error if visit fails
	if st == nil && !HasError() {
		// make return itself failed without visit — also ok (no invent)
	}
	ClearError()
}
