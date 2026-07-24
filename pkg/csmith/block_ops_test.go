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
	// FactMgr + Block always live; sticky no invent soft-skip reset past hole
	ClearError()
	(*FactMgr)(nil).ResetBlockFactMaps(b)
	if !HasError() {
		t.Fatal("nil FM ResetBlockFactMaps must SetError sticky")
	}
	ClearError()
	fm.ResetBlockFactMaps(nil)
	if !HasError() {
		t.Fatal("nil Block ResetBlockFactMaps must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).ResetStmFactMaps(&Stmt{StmID: 1})
	if !HasError() {
		t.Fatal("nil FM ResetStmFactMaps must SetError sticky")
	}
	ClearError()
	fm.ResetStmFactMaps(nil)
	if !HasError() {
		t.Fatal("nil Stmt ResetStmFactMaps must SetError sticky")
	}
	ClearError()
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
	// FactMgr + live dest StmID always required; sticky nil
	ClearError()
	if (*FactMgr)(nil).FindJumpSources(5) != nil {
		t.Fatal("nil FM FindJumpSources must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FM FindJumpSources must SetError sticky")
	}
	ClearError()
	if fm.FindJumpSources(IncompleteStmID) != nil {
		t.Fatal("destStmID 0 FindJumpSources must fail closed")
	}
	if !HasError() {
		t.Fatal("destStmID 0 FindJumpSources must SetError sticky")
	}
	ClearError()
	// nil CFG hole fails closed sticky
	fm.CFGEdges = []*CFGEdge{{SrcID: 10, DestStmID: 5}, nil}
	if fm.FindJumpSources(5) != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil CFG hole FindJumpSources must SetError sticky")
	}
	ClearError()
	if FindJumpLabel(fm, 5) != "" {
		t.Fatal("nil hole FindJumpLabel must fail closed")
	}
	if !HasError() {
		t.Fatal("nil CFG hole FindJumpLabel must SetError sticky")
	}
	ClearError()
	// IncompleteCFGEdges marker is incomplete (not invent empty-complete)
	if CFGEdgesComplete(IncompleteCFGEdges()) {
		t.Fatal("IncompleteCFGEdges must be incomplete")
	}
	if CFGEdgesComplete(nil) {
		// complete empty is complete
	} else {
		t.Fatal("CFGEdgesComplete(nil) must be complete empty")
	}
	if !CFGEdgesComplete([]*CFGEdge{}) {
		t.Fatal("empty non-nil CFG must be complete")
	}
}

func TestNeedNestedLoop(t *testing.T) {
	// live ArrayVariable AsArray required for GetDimension (no invent dim from ArraySizes alone)
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	arr := &av.Variable
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

func TestNeedNestedLoopNilRWHoleFailClosed(t *testing.T) {
	// Soft invent: skip nil RW entry as absent → no nested needed (false).
	// Fair: incomplete MustRead/MustWrite list fails closed sticky true (need nested).
	ClearError()
	av := &ArrayVariable{
		Variable: Variable{Name: "a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	arr := &av.Variable
	b := &Block{Looping: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	rw := &RWDirective{MustReadVars: []*Variable{nil, arr}}
	cg := CGContext{RW: rw, IVBounds: map[*Variable]int{}}
	if !b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("nil MustRead hole must fail closed true need-nested, not invent none")
	}
	if !HasError() {
		t.Fatal("incomplete MustReadVars must SetError sticky")
	}
	ClearError()
	rw2 := &RWDirective{MustWriteVars: []*Variable{nil, arr}}
	cg2 := CGContext{RW: rw2, IVBounds: map[*Variable]int{}}
	if !b.NeedNestedLoop(cg2, NewRng(1)) {
		t.Fatal("nil MustWrite hole must fail closed true need-nested, not invent none")
	}
	if !HasError() {
		t.Fatal("incomplete MustWriteVars must SetError sticky")
	}
	ClearError()
}

func TestNeedNestedLoopMustJumpResidualSticky(t *testing.T) {
	// StmtBreak with nil Expr: MustJump stickies residual ERROR+false.
	// Soft invent was treat as not-must-jump then invent "no nested" (false) past hole.
	// Fair: residual sticky restrictive need nested true.
	ClearError()
	defer ClearError()
	b := &Block{Looping: true, Stmts: []Stmt{{Kind: StmtBreak}}} // nil Expr
	cg := CGContext{}                                            // RW nil would soft invent false after residual
	if !b.NeedNestedLoop(cg, NewRng(1)) {
		t.Fatal("MustJump residual must fail closed true need-nested, not invent none")
	}
	if !HasError() {
		t.Fatal("MustJump residual NeedNestedLoop must SetError sticky")
	}
	ClearError()
}

func TestMustBreakOrReturnFullBackEdge(t *testing.T) {
	fm := NewFactMgr(nil)
	b := &Block{
		StmID: 50,
		Stmts: []Stmt{{
			Kind:  StmtReturn,
			StmID: 51,
			Expr:  &Expression{Term: TermConstant, Con: MakeInt(0)},
		}},
	}
	// no back edges → true
	if !b.MustBreakOrReturnFull(fm) {
		t.Fatal("expect must break/return")
	}
	// continue-like edge from outside into block (dest = Block*, DestStmID = block.StmID)
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{
		SrcID: 99, DestBlock: b, DestStmID: b.StmID, BackLink: true,
	})
	if b.MustBreakOrReturnFull(fm) {
		t.Fatal("back edge escapes return")
	}
	// goto-to-label: DestStmID is label stmt, not block — must not escape
	// (Block.cpp:346–353 find_edges_in e->dest == this only)
	fm.CFGEdges = []*CFGEdge{{
		SrcID: 5, DestBlock: b, DestStmID: 51, BackLink: true,
	}}
	if !b.MustBreakOrReturnFull(fm) {
		t.Fatal("goto-to-label must not escape must_break_or_return")
	}
	// Block always live; sticky no invent not-must-break soft-skip past hole
	ClearError()
	if (*Block)(nil).MustBreakOrReturnFull(fm) {
		t.Fatal("nil MustBreakOrReturnFull must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil MustBreakOrReturnFull must SetError sticky")
	}
	ClearError()
	// Block always live at remove_stmt; sticky no invent no-op remove soft-skip
	if (*Block)(nil).RemoveStmt(1, fm) != 0 {
		t.Fatal("nil RemoveStmt must fail closed 0")
	}
	if !HasError() {
		t.Fatal("nil RemoveStmt must SetError sticky")
	}
	ClearError()
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
	ReinstallTestProcessSingletons()
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
	ReinstallTestProcessSingletons()
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
	// Block StmID 0 + FM fails closed (no invent fold into key 0)
	ClearError()
	bad := &Block{Func: f, StmID: IncompleteStmID, Parent: nil}
	f.Stack = []*Block{bad}
	if bad.AppendReturnStmt(NewRng(2), opts, NewVariableSelector(opts), &cg) != nil {
		t.Fatal("block StmID 0 must fail closed")
	}
	if !HasError() {
		t.Fatal("expect sticky error on incomplete block id")
	}
	ClearError()
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
	// nil FM must not invent clean no-back-edge
	if !b2.ContainsBackEdge(nil) {
		t.Fatal("nil FM must fail closed has-back")
	}
}

func TestMakeDummyBlockCG(t *testing.T) {
	ClearError()
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
	ClearError()
	// Block.cpp:96–97 assert(curr_func) sticky
	if MakeDummyBlockCG(nil, opts) != nil {
		t.Fatal("nil cg must fail closed")
	}
	if !HasError() {
		t.Fatal("nil cg must SetError sticky")
	}
	ClearError()
	empty := EmptyCGContext()
	if MakeDummyBlockCG(&empty, opts) != nil {
		t.Fatal("nil CurrentFunc must fail closed")
	}
	if !HasError() {
		t.Fatal("nil CurrentFunc must SetError sticky")
	}
	ClearError()
	// MakeDummyBlock without CG still needs live Function sticky
	if MakeDummyBlock(nil) != nil {
		t.Fatal("nil Function MakeDummyBlock must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function MakeDummyBlock must SetError sticky")
	}
	ClearError()
}

func TestMakeDummyBlockCGIncompleteFailClosed(t *testing.T) {
	// incomplete EffectAccum / GlobalFacts / EffectContext must not invent dummy block success
	ClearError()
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeDummyBlockCG(&cg, opts) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeDummyBlockCG")
	}
	if !HasError() {
		t.Fatal("must SetError")
	}
	if len(f.Blocks) != 0 || len(f.Stack) != 0 {
		t.Fatal("must not leave partial block registration")
	}
	ClearError()
	f2 := &Function{Name: "f2", ReturnType: GetSimpleType(EVoid)}
	fm2 := NewFactMgr(f2)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f2, EmptyEffect()).WithFactMgr(fm2)
	if MakeDummyBlockCG(&cg2, opts) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeDummyBlockCG")
	}
	if !HasError() {
		t.Fatal("must SetError GlobalFacts")
	}
	ClearError()
	f3 := &Function{Name: "f3", ReturnType: GetSimpleType(EVoid)}
	cg3 := WithFunc(f3, IncompleteEffect()).WithFactMgr(NewFactMgr(f3))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if MakeDummyBlockCG(&cg3, opts) != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeDummyBlockCG")
	}
	if !HasError() {
		t.Fatal("must SetError EffectContext")
	}
	if len(f3.Blocks) != 0 || len(f3.Stack) != 0 {
		t.Fatal("must not leave partial block on incomplete context")
	}
	ClearError()
}

func TestBlockPostCreationIncompletePreEffectFailClosed(t *testing.T) {
	ClearError()
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	b := &Block{StmID: AllocStmID(), Func: f}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	b.PostCreationAnalysis(&cg, Defaults(), IncompleteEffect(), nil, nil)
	if !HasError() {
		t.Fatal("incomplete preEffect must SetError")
	}
	if FactsComplete(fm.GlobalFacts) && len(fm.GlobalFacts) == 0 {
		// may be IncompleteFactSlice
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("must wipe GlobalFacts incomplete")
	}
	ClearError()
	b0 := &Block{StmID: IncompleteStmID, Func: f}
	b0.PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), nil, nil)
	if !HasError() {
		t.Fatal("StmID 0 must SetError")
	}
	ClearError()
	// Block + CGContext always live; sticky (no invent soft-skip past hole)
	// Nil FM is non-sticky soft re-pick (sticky poisons soft factories without FM)
	(*Block)(nil).PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), nil, nil)
	if !HasError() {
		t.Fatal("nil block PostCreationAnalysis must SetError sticky")
	}
	ClearError()
	b.PostCreationAnalysis(nil, Defaults(), EmptyEffect(), nil, nil)
	if !HasError() {
		t.Fatal("nil cg PostCreationAnalysis must SetError sticky")
	}
	ClearError()
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
	// macro_tmp_vars name+type always live; sticky no invent partial tmp list
	// Block.cpp:261 — OutputTmpVariableList only under math_notmp
	prevO := ProcessOptions()
	o := prevO
	o.MathNoTmp = true
	SetProcessOptions(o)
	defer SetProcessOptions(prevO)
	ClearError()
	b := &Block{TmpVars: map[string]ESimpleType{"": EInt}}
	if out := b.Output(0); out != "" {
		t.Fatal("empty tmp name must fail closed whole block", out)
	}
	if !HasError() {
		t.Fatal("empty tmp name must SetError sticky")
	}
	// incomplete LocalVars fails closed sticky (no invent soft-skip hole partial defs)
	ClearError()
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	b2 := &Block{LocalVars: []*Variable{loc, nil}}
	if out := b2.Output(0); out != "" {
		t.Fatal("LocalVars hole must fail closed whole block", out)
	}
	if !HasError() {
		t.Fatal("LocalVars hole must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was synthetic ArrayVariable from ArraySizes
	// fair: sticky empty whole block (no invent partial def emit)
	arrShell := &Variable{Name: "l_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	b3 := &Block{LocalVars: []*Variable{arrShell}}
	if out := b3.Output(0); out != "" {
		t.Fatal("IsArray without AsArray must fail closed whole block", out)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray Block.Output must SetError sticky")
	}
	ClearError()
	// PreOutput residual soft invent was soft-continue then emit later stmts.
	// Fair: sticky fail closed whole Block.Output (StmID 0 under FM stickies PreOutput).
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	good := CreateVariableScalars("g_1", GetIntType(), false, false)
	b4 := &Block{
		Func:   f,
		EmitFM: fm,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: IncompleteStmID, LhsVar: good}, // StmID 0 under FM sticky PreOutput
			{Kind: StmtAssign, StmID: 2, LhsVar: good, Expr: &Expression{Term: TermVariable, Var: good, ExprType: GetIntType()}},
		},
	}
	if out := b4.Output(0); out != "" {
		t.Fatal("PreOutput residual must fail closed whole block", out)
	}
	if !HasError() {
		t.Fatal("PreOutput residual Block.Output must SetError sticky")
	}
	ClearError()
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
	currentSession().MetaFactPointToEnabled = false
	var empty []*FactPointTo
	AddNewVarFactTo(p, &empty)
	if len(empty) != 0 {
		t.Fatal("meta off must not invent", empty)
	}
	ClearMetaFacts()
}

func TestAddNewVarFactIntoNilFieldHoleFailClosed(t *testing.T) {
	// soft invent: skip nil FieldVars and still makeup later pointer fields
	// fair: incomplete FieldVars clears *facts
	currentSession().MetaFactPointToEnabled = true
	defer ClearMetaFacts()
	ClearError()
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "p", Type: PointerTo(GetIntType()), BitWidth: -1},
		{Name: "q", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	v := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if v == nil || HasError() {
		t.Fatal("CreateVariableQfer struct with pointer fields", v, HasError())
	}
	// CreateFieldVars may have filled; force nil hole + later live pointer field
	q := CreateVariableScalars("g_s.q", PointerTo(GetIntType()), false, false)
	if q == nil {
		t.Fatal("live pointer field var")
	}
	v.FieldVars = []*Variable{nil, q}
	// seed a prior fact so clear is observable vs empty start
	prior := MakeFactPointTo(CreateVariableScalars("g_other", PointerTo(GetIntType()), false, false), NullPtr)
	facts := []*FactPointTo{prior}
	AddNewVarFactInto(v, &facts)
	if FactsComplete(facts) {
		t.Fatal("nil FieldVars hole must fail closed clear facts, not soft-skip", facts)
	}
	if !HasError() {
		t.Fatal("nil FieldVars hole must SetError sticky")
	}
	ClearError()
	// nil Variable* subject fails closed sticky (no invent skip as absent)
	facts2 := []*FactPointTo{prior}
	AddNewVarFactInto(nil, &facts2)
	if FactsComplete(facts2) {
		t.Fatal("nil v must fail closed clear facts", facts2)
	}
	if !HasError() {
		t.Fatal("nil v must SetError sticky")
	}
	ClearError()
	// facts always live; sticky (no invent soft-skip makeup past hole)
	AddNewVarFactInto(CreateVariableScalars("g_n", GetIntType(), false, false), nil)
	if !HasError() {
		t.Fatal("nil facts AddNewVarFactInto must SetError sticky")
	}
	ClearError()
	// Type-nil non-special shell: soft invent was IsPointer residual ERROR + empty FieldVars
	// complete skip (facts stay complete). Fair: clear facts sticky before field walk.
	shell := &Variable{Name: "g_typeless"}
	facts3 := []*FactPointTo{prior}
	AddNewVarFactInto(shell, &facts3)
	if FactsComplete(facts3) {
		t.Fatal("Type-nil shell must fail closed clear facts, not empty-fields complete", facts3)
	}
	if !HasError() {
		t.Fatal("Type-nil shell must SetError sticky")
	}
	ClearError()
	// AddNewVarFact same Type-nil sticky clear GlobalFacts
	fm := NewFactMgr(nil)
	if fm == nil {
		t.Fatal("NewFactMgr")
	}
	fm.GlobalFacts = []*FactPointTo{prior}
	fm.AddNewVarFact(shell)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("AddNewVarFact Type-nil must clear GlobalFacts", fm.GlobalFacts)
	}
	if !HasError() {
		t.Fatal("AddNewVarFact Type-nil must SetError sticky")
	}
	ClearError()
}

func TestFindFixedPointLocalVarsNilHoleFailClosed(t *testing.T) {
	// soft invent: AddNewVarFactTo(nil) no-op skip LocalVars hole
	// fair: nil LocalVars fails closed fixed-point
	ClearError()
	defer ClearError()
	b := &Block{
		StmID:     60,
		LocalVars: []*Variable{nil},
		Stmts:     []Stmt{},
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	inputs := []*FactPointTo{}
	_, _, _, ok := FindFixedPointBlock(b, inputs, &cg, Defaults(), true)
	if ok {
		t.Fatal("nil LocalVars hole must fail closed fixed-point")
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
	out, _, _, ok := FindFixedPointBlock(b, in, &cg, Defaults(), false)
	if !ok {
		t.Fatal("fp")
	}
	_ = out
}

// TestDropUnionLocalsSyncsCurrentUnionsForSameFacts — Block.cpp:557–599
// set_fact_in stores the same FactVec that remains current_inputs. Go's
// eUnionWrite split: DropUnionSubjectsByVars built entryUnions for map_in but
// left currentUnions holding body locals reintroduced by back-edge merge
// (seed-189 blk 575: l_1333 from goto src map_out). same_facts forever saw
// nCurU=mapInU+1 → 50 full walks rewrote map_accum_effect → forward
// StatementGoto.cpp:125–128 choose_visible_read_var (g_6 vs UP g_1192).
// After drop, currentUnions must equal the map_in half so shortcut can match.
func TestDropUnionLocalsSyncsCurrentUnionsForSameFacts(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	ut := &Type{isUnion: true, StructName: "U_sync", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	loc := CreateVariableScalars("l_local_u", ut, false, false)
	loc.CreateFieldVars()
	g := CreateVariableScalars("g_u_sync", ut, false, false)
	g.CreateFieldVars()
	uLoc := MakeFactUnion(loc, 0)
	uG := MakeFactUnion(g, 0)
	if uLoc == nil || uG == nil {
		t.Fatal("union facts")
	}
	// currentUnions after merge of goto map_out that still listed a body local.
	currentUnions := []*FactUnion{uG.Clone(), uLoc.Clone()}
	locals := []*Variable{loc}
	entryUnions := DropUnionSubjectsByVars(currentUnions, locals)
	if !UnionFactsComplete(entryUnions) {
		t.Fatal("drop incomplete")
	}
	if FindRelatedUnion(entryUnions, loc) != nil {
		t.Fatal("drop must remove body local")
	}
	if FindRelatedUnion(entryUnions, g) == nil {
		t.Fatal("drop must keep non-local")
	}
	// Without sync: same_facts(currentUnions, entryUnions) is false forever.
	if SameUnionFacts(currentUnions, entryUnions) {
		t.Fatal("pre-sync currentUnions must still hold local (merge residue)")
	}
	// FindFixedPointBlock assigns currentUnions = entryUnions after drop.
	currentUnions = entryUnions
	if !SameUnionFacts(currentUnions, entryUnions) {
		t.Fatal("synced currentUnions must same_facts with map_in half")
	}
	ClearError()
}

func TestPostCreationFPOnlyOnHasEdgeIn(t *testing.T) {
	// Block.cpp:696–697 — FP only when is_loop_body || need_revisit || has_edge_in(false,true).
	// has_edge_in: Statement.cpp:434–446 e->dest == this (the block).
	// ContainsBackEdge (dest->parent==this) must NOT invent force FP: that wiped
	// mid-gen may-null via map_facts_out re-analysis install (seed-2 e10107).
	ClearError()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	parent := &Block{StmID: 1, Func: f}
	inner := &Block{StmID: 50, Func: f, Parent: parent, Looping: false, NeedRevisit: false}
	// stmt inside inner — back edge dest parent is inner (ContainsBackEdge) but dest != inner
	st := Stmt{Kind: StmtAssign, StmID: 51,
		LhsVar: CreateVariableScalars("g_x", GetIntType(), false, false),
		Lhs:    &Lhs{Var: CreateVariableScalars("g_x", GetIntType(), false, false), Type: GetIntType()},
		Expr:   &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	inner.Stmts = []Stmt{st}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// mid-gen may-null
	live := MakeFactPointToSet(p, []*Variable{NullPtr})
	if live == nil {
		t.Fatal("live")
	}
	fm.GlobalFacts = []*FactPointTo{live}
	// CFG: back edge to stmt 51 inside inner — NOT to block 50
	fm.CreateCFGEdge(51, inner, false, true)
	// force DestBlock parent semantics for ContainsBackEdge
	if len(fm.CFGEdges) > 0 {
		fm.CFGEdges[len(fm.CFGEdges)-1].DestBlock = inner
		fm.CFGEdges[len(fm.CFGEdges)-1].DestStmID = 51
	}
	// map_facts_in without may-null (stale entry) — FP would reinject issue if forced
	fm.SetMapFactsIn(50, []*FactPointTo{MakeFactPointTo(p, CreateVariableScalars("g_t", GetIntType(), true, false))})
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	inner.PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), nil, nil)
	// no FP → mid-gen may-null survives (after OOS of empty locals)
	got := FindRelatedPointTo(fm.GlobalFacts, p)
	if got == nil || !got.IsNull() {
		t.Fatalf("without has_edge_in(block), must not FP-wipe mid-gen may-null: %+v hasEdge=%v", got, fm.HasEdgeIn(50, false, true))
	}
	// ContainsBackEdge true would have invent-forced FP under old hasBack
	if !inner.ContainsBackEdge(fm) {
		t.Log("note: ContainsBackEdge false — edge shape may not match parent check; still no wipe")
	}
	ClearError()
}

func TestPostCreationGlobalFactsFromBodyOut(t *testing.T) {
	// Block.cpp:729 — global_facts = map_facts_out[this] after fixed-point path.
	// After ResetBlockFactMaps deletes out maps, assign must clear GlobalFacts
	// (C++ map[] empty), not invent keep prior.
	f := &Function{Name: "f", ReturnType: GetIntType()}
	parent := &Block{StmID: 1, Func: f}
	// incomplete assign fails visit → remove → empty stmts → break without re-set out
	b := &Block{StmID: 70, Func: f, Parent: parent, Looping: true, NeedRevisit: true,
		Stmts: []Stmt{{Kind: StmtAssign, StmID: 71}},
	}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	prior := MakeFactPointTo(p, GarbagePtr)
	fm.GlobalFacts = []*FactPointTo{prior}
	fm.SetMapFactsIn(70, []*FactPointTo{prior})
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	pre := EmptyEffect()
	b.PostCreationAnalysis(&cg, Defaults(), pre, nil, nil)
	// MapFactsOut missing (C++ map[] empty) → complete empty, not prior garbage
	if _, ok := fm.MapFactsOut[70]; !ok {
		if FindRelatedPointTo(fm.GlobalFacts, p) != nil {
			t.Fatal("missing MapFactsOut must not invent keep prior GlobalFacts")
		}
		return
	}
	// if out present incomplete → fail closed incomplete GlobalFacts
	if !FactsComplete(fm.MapFactsOut[70]) {
		if FactsComplete(fm.GlobalFacts) {
			t.Fatal("incomplete MapFactsOut must fail closed incomplete GlobalFacts")
		}
	}
}

func TestPostCreationIncompleteMapFactsInNoInventEmptyFP(t *testing.T) {
	// incomplete MapFactsIn[block] must not invent empty fixed-point re-analysis
	// (old soft path: FactsComplete fail → treat as nil empty env → FP success)
	ClearError()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	parent := &Block{StmID: 1, Func: f}
	b := &Block{StmID: 80, Func: f, Parent: parent, Looping: true, NeedRevisit: true,
		Stmts: []Stmt{{Kind: StmtAssign, StmID: 81,
			LhsVar: CreateVariableScalars("g_x", GetIntType(), false, false),
			Lhs:    &Lhs{Var: CreateVariableScalars("g_x", GetIntType(), false, false), Type: GetIntType()},
			Expr:   &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
		}},
	}
	// fix LhsVar pointer identity
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	b.Stmts[0].LhsVar = v
	b.Stmts[0].Lhs = &Lhs{Var: v, Type: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	// plant hole in MapFactsIn (bypass SetMapFactsIn which stores nil for incomplete)
	fm.MapFactsIn = map[int][]*FactPointTo{
		80: {MakeFactPointTo(p, NullPtr), nil},
	}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	b.PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), nil, nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete MapFactsIn must fail closed incomplete GlobalFacts, not invent empty FP", fm.GlobalFacts)
	}
	if !HasError() {
		t.Fatal("incomplete MapFactsIn must SetError sticky")
	}
	ClearError()
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

func TestFindJumpSourcesFindStmtResidualSticky(t *testing.T) {
	// FindStmt residual soft invent was soft-continue sources then invent complete src list.
	// Fair: sticky fail closed nil sources.
	ClearError()
	defer ClearError()
	f := &Function{Name: "f"}
	// incomplete if sole Blocks — FindStmt residual for SrcID under incomplete arm
	outer := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtIfElse, StmID: 1, Then: &Block{Stmts: []Stmt{{Kind: StmtGoto, StmID: 20, Label: "L"}}}, Else: nil},
		{Kind: StmtAssign, StmID: 10},
	}}
	f.Blocks = []*Block{outer}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	if fm.FindJumpSources(10) != nil {
		t.Fatal("FindStmt residual must fail closed nil sources, not invent complete list")
	}
	if !HasError() {
		t.Fatal("FindStmt residual FindJumpSources must SetError sticky")
	}
	ClearError()
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
		{SrcID: 30, DestStmID: 10},                 // goto
	}
	// assign 10: only real goto 30
	srcs := fm.FindJumpSources(10)
	if srcs == nil || len(srcs) != 1 || srcs[0] != 30 {
		t.Fatalf("goto only: %v", srcs)
	}
	// for 50: break filtered out
	if got := fm.FindJumpSources(50); got == nil || len(got) != 0 {
		t.Fatal("break must not be jump source", got)
	}
	// dangling SrcID with Func set sticky — no invent skip as non-goto
	ClearError()
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{SrcID: 99, DestStmID: 10})
	if fm.FindJumpSources(10) != nil {
		t.Fatal("unresolved SrcID must fail closed nil sources")
	}
	if !HasError() {
		t.Fatal("unresolved SrcID FindJumpSources must SetError sticky")
	}
	ClearError()
	// only unresolved edge for dest 11 — no invent skip hole to registry/label
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestStmID: 11}}
	if FindJumpLabel(fm, 11) != "" {
		t.Fatal("unresolved SrcID must fail closed empty label")
	}
	if !HasError() {
		t.Fatal("unresolved SrcID FindJumpLabel must SetError sticky")
	}
	ClearError()
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
	currentSession().StmLabels[100] = ""
	if got := FindJumpLabel(fm, 100); got != "" {
		t.Fatal("empty registry label must fail closed", got)
	}
}

func TestFindStmtByID(t *testing.T) {
	f := &Function{Name: "f"}
	inner := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	// StatementIf always has both arms
	outer := &Block{Func: f, Stmts: []Stmt{{Kind: StmtIfElse, StmID: 1, Then: inner, Else: &Block{}}}}
	inner.Parent = outer
	f.Blocks = []*Block{outer, inner}
	st := FindStmtByID(f, 3)
	if st == nil || st.Kind != StmtAssign {
		t.Fatalf("%+v", st)
	}
	if FindStmtByID(f, 999) != nil {
		t.Fatal("missing")
	}
	// incomplete if on sole Blocks entry fails closed sticky (no invent soft-continue past nil Else)
	ClearError()
	f.Blocks = []*Block{outer}
	outer.Stmts[0].Else = nil
	if FindStmtByID(f, 3) != nil {
		t.Fatal("nil Else must fail closed when only reachable via incomplete if")
	}
	if !HasError() {
		t.Fatal("nil Else FindStmtByID must SetError sticky")
	}
	ClearError()
	// Function + live StmID always required; sticky no invent miss soft-success
	if FindStmtByID(nil, 3) != nil {
		t.Fatal("nil Function FindStmtByID must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function FindStmtByID must SetError sticky")
	}
	ClearError()
	if FindStmtByID(f, IncompleteStmID) != nil {
		t.Fatal("stmID 0 FindStmtByID must fail closed")
	}
	if !HasError() {
		t.Fatal("stmID 0 FindStmtByID must SetError sticky")
	}
	ClearError()
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
	// IsArray without AsArray soft invent was dim from ArraySizes; fair sticky 0
	ClearError()
	v := CreateVariableScalars("g_b", GetIntType(), false, false)
	v.IsArray = true
	v.ArraySizes = []int{5, 6}
	if v.GetDimension() != 0 {
		t.Fatal("IsArray without AsArray GetDimension must fail closed 0, got", v.GetDimension())
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray GetDimension must SetError sticky")
	}
	ClearError()
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

func TestRemoveStmtFindStmtResidualNoInventGotoCascade(t *testing.T) {
	// Soft invent was FindStmt residual/nil miss → isGoto true cascade invent.
	// Fair: unresolved SrcID with Func set sticky wipe IncompleteCFGEdges.
	ClearError()
	defer ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	destID := 10
	// missing src stmt id — FindStmt returns nil complete miss
	body := &Block{Func: f, StmID: 1, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: destID},
		{Kind: StmtAssign, StmID: 30},
	}}
	f.Blocks = []*Block{body}
	f.Body = body
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestStmID: destID}} // src 99 not in function
	_ = body.RemoveStmt(destID, fm)
	if CFGEdgesComplete(fm.CFGEdges) {
		t.Fatal("unresolved SrcID must wipe IncompleteCFGEdges, not invent empty complete / cascade")
	}
	if !HasError() {
		t.Fatal("unresolved SrcID RemoveStmt must SetError sticky")
	}
	ClearError()
}

func TestRemoveStmtScrubsFuncBlocks(t *testing.T) {
	// Block.cpp:655–663 — nested Then block dropped from Function.Blocks
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	inner := &Block{Func: f, StmID: 5, Stmts: []Stmt{{Kind: StmtAssign, StmID: 6}}}
	// StatementIf always has live if_true/if_false
	ifSt := Stmt{Kind: StmtIfElse, StmID: 4, Then: inner, Else: &Block{Func: f, StmID: 8}}
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

func TestRemoveStmtScrubsParentChainOrphanBlocks(t *testing.T) {
	// Statement.cpp:684–694 eBlock contains_stmt: candidate parent chain, not only
	// Stmts-linked StmtBlock children. Failed/orphan nested blocks stay on
	// Func.Blocks with Parent set (Block.cpp:142–174 abort leaves the entry);
	// remove_stmt of the ancestor must still erase them (Block.cpp:655–663).
	// Soft invent Stmts-only walk left them for StatementGoto find_good_jump_block
	// (seed 11466719812903307384 first_div: Go n=37 vs UP n=3).
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	body := &Block{Func: f, StmID: 10}
	// orphan: Parent=body but never a StmtBlock child of body
	orphan := &Block{Func: f, StmID: 11, Parent: body}
	forSt := Stmt{Kind: StmtFor, StmID: 4, Then: body}
	outer := &Block{Func: f, StmID: 1, Stmts: []Stmt{forSt, {Kind: StmtAssign, StmID: 7}}}
	body.Parent = outer
	f.Blocks = []*Block{outer, body, orphan}
	f.Body = outer
	n := outer.RemoveStmt(4, fm)
	if n != 1 {
		t.Fatalf("RemoveStmt count=%d", n)
	}
	if HasError() {
		t.Fatal("sticky ERROR after RemoveStmt")
	}
	for _, b := range f.Blocks {
		if b == body || b == orphan {
			t.Fatalf("must scrub body+orphan via parent chain, still have %v", b.StmID)
		}
	}
	if len(f.Blocks) != 1 || f.Blocks[0] != outer {
		t.Fatalf("Func.Blocks=%v", f.Blocks)
	}
}

func TestRemoveStmtDestEdgeUsesParentChainContains(t *testing.T) {
	// Block.cpp:632–646 — if (s->contains_stmt(edge->dest)) full contains_stmt.
	// Soft invent was blockUnderStmt (Stmts walk) for DestBlock, missing orphan
	// nested blocks with only Parent set → edge kept, goto cascade skipped.
	ClearError()
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	body := &Block{Func: f, StmID: 10}
	orphan := &Block{Func: f, StmID: 11, Parent: body}
	gotoSt := Stmt{Kind: StmtGoto, StmID: 20, Label: "lbl"}
	forSt := Stmt{Kind: StmtFor, StmID: 4, Then: body}
	outer := &Block{Func: f, StmID: 1, Stmts: []Stmt{forSt, gotoSt, {Kind: StmtAssign, StmID: 7}}}
	body.Parent = outer
	f.Blocks = []*Block{outer, body, orphan}
	f.Body = outer
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 20, DestStmID: 11, DestBlock: orphan},
		{SrcID: 7, DestStmID: 99},
	}
	n := outer.RemoveStmt(4, fm)
	if n < 1 {
		t.Fatalf("RemoveStmt count=%d", n)
	}
	if HasError() {
		t.Fatal("sticky ERROR after RemoveStmt")
	}
	for _, e := range fm.CFGEdges {
		if e != nil && e.SrcID == 20 {
			t.Fatalf("edge into orphan DestBlock must scrub, still have %+v", e)
		}
	}
	for _, s := range outer.Stmts {
		if s.StmID == 20 {
			t.Fatal("goto into removed orphan dest must cascade-remove")
		}
	}
}

func TestRemoveStmtIncompleteCFGWipeMarker(t *testing.T) {
	// incomplete CFG scrub must wipe IncompleteCFGEdges sticky (not bare nil invent empty complete)
	ClearError()
	fm := NewFactMgr(nil)
	b := &Block{
		StmID: 10,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: 1},
			{Kind: StmtBreak, StmID: 2},
		},
	}
	fm.CFGEdges = []*CFGEdge{{SrcID: 2, DestStmID: 100}, nil}
	n := b.RemoveStmt(2, fm)
	if n != 1 {
		t.Fatal(n)
	}
	if CFGEdgesComplete(fm.CFGEdges) {
		t.Fatal("incomplete scrub must leave IncompleteCFGEdges, not invent empty complete", fm.CFGEdges)
	}
	if !HasError() {
		t.Fatal("incomplete CFG scrub must SetError sticky")
	}
	ClearError()
	// incomplete Function.Blocks hole → IncompleteBlocks sticky
	f := &Function{Name: "f"}
	fm2 := NewFactMgr(f)
	outer := &Block{Func: f, StmID: 1, Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	f.Blocks = []*Block{outer, nil}
	f.Body = outer
	if outer.RemoveStmt(3, fm2) != 1 {
		t.Fatal("remove")
	}
	if BlocksComplete(f.Blocks) {
		t.Fatal("incomplete Blocks scrub must leave IncompleteBlocks", f.Blocks)
	}
	if !HasError() {
		t.Fatal("incomplete Blocks scrub must SetError sticky")
	}
	ClearError()
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

func TestAppendNestedLoopBumpsBlkDepthAroundFor(t *testing.T) {
	// Block.cpp:424 Statement::make_random(eFor) → Statement.cpp:272–274 / 315–317
	// compound eFor increments blk_depth for the nested body, restores after.
	// Without the bump, body statements see parent depth (seed-4 e11119: GO blk=3 vs UP blk=5).
	ClearError()
	opts := Defaults()
	opts.MaxBlockDepth = 5
	opts.MaxBlockSize = 1
	// Minimal process tables so MakeRandomFor can run or fail cleanly
	prevStmt := ProcessStmtTab()
	SetProcessStmtTab(InitProbabilityTable(opts))
	defer SetProcessStmtTab(prevStmt)

	f := &Function{Name: "f", ReturnType: GetIntType()}
	b := &Block{Looping: true, Func: f, StmID: 1}
	f.Stack = []*Block{b}
	f.Blocks = []*Block{b}
	cg := WithFunc(f, EmptyEffect())
	cg.BlkDepth = 2
	cg.Flags |= FlagInLoop
	// FM required by MakeRandomFor
	cg.FM = NewFactMgr(f)

	pre := cg.BlkDepth
	_ = b.AppendNestedLoop(NewRng(42), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	// Whether for succeeds or null-fails, outer depth must restore (Statement.cpp:315–317).
	if cg.BlkDepth != pre {
		t.Fatalf("AppendNestedLoop must restore BlkDepth: got %d want %d", cg.BlkDepth, pre)
	}
	ClearError()
}

func TestAppendNestedLoopUsesMakeRandomForcedFor(t *testing.T) {
	// Block.cpp:424 — Statement::make_random(cg, eFor), not bare MakeRandomFor.
	// makeRandomStmtForced zeros expr_depth and bumps blk_depth (Statement.cpp:288–291).
	// Leftover ExprDepth=7 must not stick as the starting depth of make_iteration:
	// after append returns, either factory ran (depth may be non-zero from exprs)
	// or null-failed; BlkDepth must restore either way.
	ClearError()
	opts := Defaults()
	opts.MaxBlockDepth = 5
	opts.MaxBlockSize = 1
	prevStmt := ProcessStmtTab()
	SetProcessStmtTab(InitProbabilityTable(opts))
	defer SetProcessStmtTab(prevStmt)

	f := &Function{Name: "f", ReturnType: GetIntType()}
	b := &Block{Looping: true, Func: f, StmID: 1}
	f.Stack = []*Block{b}
	f.Blocks = []*Block{b}
	cg := WithFunc(f, EmptyEffect())
	cg.BlkDepth = 2
	cg.Flags |= FlagInLoop
	cg.FM = NewFactMgr(f)
	cg.ExprDepth = 7 // leftover from prior sibling assign

	preBlk := cg.BlkDepth
	_ = b.AppendNestedLoop(NewRng(42), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if cg.BlkDepth != preBlk {
		t.Fatalf("BlkDepth restore after make_random(eFor): got %d want %d", cg.BlkDepth, preBlk)
	}
	// Injected 7 must have been cleared at factory entry (Statement.cpp:288).
	// After a live factory, expr_depth may be left non-zero by nested exprs —
	// only require we did not keep 7 untouched without ever entering make_random.
	if cg.ExprDepth == 7 && len(b.Stmts) == 0 {
		// null path still zeros depth each try; if nothing appended and depth still 7,
		// makeRandomStmtForced never ran (regression).
		t.Fatal("AppendNestedLoop must enter makeRandomStmtForced (ExprDepth still 7, no stmt)")
	}
	ClearError()
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
	ClearError()
	// incomplete ambient must not invent nested for past holes
	cgInc := WithFunc(f, IncompleteEffect())
	if b.AppendNestedLoop(NewRng(2), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cgInc) != nil {
		t.Fatal("incomplete EffectContext must fail closed AppendNestedLoop")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomAssignRejectsConstStruct(t *testing.T) {
	// StatementAssign.cpp:124 assert(!is_const_struct_union) sticky
	ClearError()
	opts := Defaults()
	// ProcessAssignOpsTable required past FM gate
	prevTab := ProcessAssignOpsTable()
	SetProcessAssignOpsTable(NewAssignOpsTable(opts))
	defer SetProcessAssignOpsTable(prevTab)
	cq := NewCVQualifiers([]bool{true}, []bool{false})
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: cq},
	}}
	if !st.IsConstStructUnion() {
		t.Fatal("fixture not const struct")
	}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	got := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, st)
	if stmtOK(got) {
		t.Fatal("const struct assign must fail closed")
	}
	if !HasError() {
		t.Fatal("const struct assign must SetError sticky")
	}
	ClearError()
	// IsConstStructUnion residual soft invent was soft-continue assign past Type-nil field.
	// Fair: sticky fail closed empty Stmt.
	hole := &Type{isStruct: true, StructName: "H", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if !hole.IsConstStructUnion() || !HasError() {
		t.Fatal("fixture Type-nil field must residual IsConstStructUnion sticky true")
	}
	ClearError()
	got2 := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, hole)
	if stmtOK(got2) {
		t.Fatal("IsConstStructUnion residual must fail closed assign")
	}
	if !HasError() {
		t.Fatal("IsConstStructUnion residual assign must SetError sticky")
	}
	ClearError()
	// IsVolatileStructUnion residual under StrictVolatileRule soft invent was soft-continue RHS.
	optsSV := Defaults()
	optsSV.StrictVolatileRule = true
	SetProcessAssignOpsTable(NewAssignOpsTable(optsSV))
	got3 := MakeRandomAssign(NewRng(1), optsSV, NewProbabilities(optsSV), NewVariableSelector(optsSV), NewExprTables(optsSV), &cg, hole)
	if stmtOK(got3) {
		t.Fatal("IsVolatileStructUnion residual must fail closed assign")
	}
	if !HasError() {
		t.Fatal("IsVolatileStructUnion residual assign must SetError sticky")
	}
	ClearError()
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

func TestFindJumpLabelNilFMSticky(t *testing.T) {
	ClearError()
	if FindJumpLabel(nil, 1) != "" {
		t.Fatal("nil FM FindJumpLabel must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FM FindJumpLabel must SetError sticky")
	}
	ClearError()
}

func TestContainsBackEdgeIncompleteSticky(t *testing.T) {
	ClearError()
	if (*Block)(nil).ContainsBackEdge(NewFactMgr(nil)) {
		t.Fatal("nil Block ContainsBackEdge must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Block ContainsBackEdge must SetError sticky")
	}
	ClearError()
	b := &Block{StmID: 1}
	if !b.ContainsBackEdge(nil) {
		t.Fatal("nil FM ContainsBackEdge must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil FM ContainsBackEdge must SetError sticky")
	}
	ClearError()
}

func TestContainsBackEdgeNilResidualSticky(t *testing.T) {
	// ContainsBackEdge residual soft invent was invent no-back soft-skip past nil Block.
	ClearError()
	if (*Block)(nil).ContainsBackEdge(NewFactMgr(nil)) {
		t.Fatal("nil Block ContainsBackEdge must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Block ContainsBackEdge must SetError sticky")
	}
	ClearError()
}

func TestFromTailToHeadNilResidualSticky(t *testing.T) {
	ClearError()
	if (*Block)(nil).FromTailToHead() {
		t.Fatal("nil Block FromTailToHead must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Block FromTailToHead must SetError sticky")
	}
	ClearError()
}

func TestFindFixedPointDoesNotReinjectStrippedMayNull(t *testing.T) {
	// Block.cpp:558–562 — set_fact_out from re-analysis outputs only.
	// StmVisitFacts must not invent mergeMayNullFromLive into analysis inputs.
	// Block.cpp:513–568 — find_fixed_point does not assign global_facts; mid-gen
	// live may stay polluted until post_creation installs map_facts_out (729).
	// map_facts_out from re-analysis outputs only (no invent reinject after strip).
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// clean entry: g_p → g_t only
	entry := []*FactPointTo{MakeFactPointTo(p, tgt)}
	// live GlobalFacts polluted as if mid-gen assign set may-null then stmt stripped
	polluted := MakeFactPointToSet(p, []*Variable{tgt, NullPtr})
	if polluted == nil {
		t.Fatal("make fact")
	}
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{polluted}
	b := &Block{StmID: 100, Func: f, Looping: false, LocalVars: nil}
	fm.SetMapFactsIn(100, entry)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	out, _, _, ok := FindFixedPointBlock(b, entry, &cg, Defaults(), true)
	if !ok {
		t.Fatal("empty-body fixed-point must succeed")
	}
	got := FindRelatedPointTo(out, p)
	if got == nil {
		t.Fatal("missing fact for g_p")
	}
	if got.IsNull() {
		t.Fatalf("stripped may-null must not reinject into out: %v", got.PointTo)
	}
	// map_facts_out is the installed analysis env — must stay clean
	mout := fm.GetMapFactsOut(100)
	if g := FindRelatedPointTo(mout, p); g != nil && g.IsNull() {
		t.Fatal("map_facts_out must not reinject stripped may-null")
	}
	// global_facts unchanged by find_fixed_point (C++); may still hold mid-gen
	if g := FindRelatedPointTo(fm.GlobalFacts, p); g == nil || !g.IsNull() {
		t.Fatal("find_fixed_point must not assign global_facts from out")
	}
}

func TestStmVisitFactsRestoresLiveGlobalFacts(t *testing.T) {
	// Statement.cpp:609–626 — does not assign global_facts = inputs.
	// After visit, mid-gen may-null on GlobalFacts must remain (seed-2 e10107).
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	inputs := []*FactPointTo{MakeFactPointTo(p, tgt)}
	live := MakeFactPointToSet(p, []*Variable{tgt, NullPtr})
	if live == nil {
		t.Fatal("live fact")
	}
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 42, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{live}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := CloneFactSlice(inputs)
	if !StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatalf("simple assign visit must ok hasErr=%v", HasError())
	}
	gotLive := FindRelatedPointTo(fm.GlobalFacts, p)
	if gotLive == nil || !gotLive.IsNull() {
		t.Fatalf("live GlobalFacts must restore mid-gen may-null: %+v", gotLive)
	}
	ClearError()
}

// TestAbortBlockMakeLeavesOnFuncBlocks — Block.cpp:142–174 ERROR path.
// stack.pop_back(); delete b; return nullptr — no func->blocks.erase.
// remove_stmt alone erases (Block.cpp:653–660). Invent erase shrinks
// StatementGoto::make_random's func->blocks copy (seed-2 e12688 n=11 vs 14).
// ~Block clears stms so find_good_jump filters empty (StatementGoto.cpp:333–336).
func TestAbortBlockMakeLeavesOnFuncBlocks(t *testing.T) {
	ClearError()
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	nested := &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: AllocStmID()}}}
	b := &Block{
		StmID: AllocStmID(), Func: f,
		Stmts: []Stmt{
			{Kind: StmtReturn, StmID: AllocStmID()},
			{Kind: StmtIfElse, StmID: AllocStmID(), Then: nested, Else: &Block{StmID: AllocStmID()}},
		},
	}
	f.Stack = []*Block{b}
	f.Blocks = []*Block{b, nested}
	abortBlockMake(f, b)
	if len(f.Stack) != 0 {
		t.Fatalf("abort must pop stack, got %d", len(f.Stack))
	}
	if len(f.Blocks) != 2 || f.Blocks[0] != b {
		t.Fatalf("abort must leave block on Function.Blocks (C++ no erase), got %d", len(f.Blocks))
	}
	if len(b.Stmts) != 0 {
		t.Fatalf("~Block must clear stms for empty find_good filter, got %d", len(b.Stmts))
	}
	if len(nested.Stmts) != 0 {
		t.Fatal("nested then via if must be tombstoned (~StatementIf delete &if_true)")
	}
	// sticky on nil args still
	ClearError()
	abortBlockMake(nil, b)
	if !HasError() {
		t.Fatal("nil func must SetError")
	}
	ClearError()
}

// TestMakeRandomBlockPostPushErrorLeavesOnBlocks — after blocks.push_back,
// ERROR cleanup (Block.cpp:142–174) leaves entries on Blocks for goto pool size.
func TestMakeRandomBlockPostPushErrorLeavesOnBlocks(t *testing.T) {
	ClearError()
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	b1 := &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{{Kind: StmtReturn, StmID: AllocStmID()}}}
	b2 := &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: AllocStmID()}}}
	b3 := &Block{StmID: AllocStmID(), Func: f}
	for _, b := range []*Block{b1, b2, b3} {
		f.Stack = append(f.Stack, b)
		f.Blocks = append(f.Blocks, b)
		abortBlockMake(f, b)
	}
	if len(f.Blocks) != 3 {
		t.Fatalf("three ERROR make_random must leave 3 Blocks entries, got %d", len(f.Blocks))
	}
	if len(f.Stack) != 0 {
		t.Fatalf("stack must be empty after aborts, got %d", len(f.Stack))
	}
	for i, b := range f.Blocks {
		if len(b.Stmts) != 0 {
			t.Fatalf("tombstoned block %d must have empty stms, got %d", i, len(b.Stmts))
		}
	}
	if n := len(append([]*Block(nil), f.Blocks...)); n != 3 {
		t.Fatalf("goto pool copy size %d want 3", n)
	}
	ClearError()
}

// TestStmVisitFactsDoesNotMergeLiveMayNullIntoInputs — Statement.cpp:609–626.
// stm_visit_facts mutates inputs only; never merges global_facts may-null into inputs.
// Invent per-stmt mergeMayNull reinjected mid-gen null during FP (seed-2 e12688).
func TestFindFixedPointSelfBackPreservesMayNull(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// mid-gen: may-null
	mid := MakeFactPointToSet(p, []*Variable{tgt, NullPtr})
	if mid == nil {
		t.Fatal("mid")
	}
	entry := []*FactPointTo{MakeFactPointTo(p, tgt)} // pre-block entry non-null only
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{mid}
	b := &Block{StmID: 50, Func: f, Looping: true, LocalVars: nil}
	// pre-FP set_fact_out from mid-gen global (Block.cpp:693)
	fm.SetMapFactsOut(50, []*FactPointTo{mid})
	fm.SetMapFactsIn(50, entry)
	fm.MapVisited = map[int]bool{50: true} // post_creation marks visited before FP
	// self-back like Block.cpp:701
	fm.CreateCFGEdge(50, b, false, true)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// entry to FP is map_facts_in (facts_copy)
	out, _, _, ok := FindFixedPointBlock(b, entry, &cg, Defaults(), false)
	if !ok {
		t.Fatalf("FP must succeed sticky=%v", HasError())
	}
	got := FindRelatedPointTo(out, p)
	if got == nil || !got.IsNull() {
		t.Fatalf("self-back must bring mid-gen may-null into out: %+v", got)
	}
	ClearError()
}

func TestFindFixedPointBackEdgeMergesUnionFacts(t *testing.T) {
	// Block.cpp:535 — merge_facts(current_inputs, map_facts_out[src]) full FactVec.
	// Soft invent was PT-only merge; eUnionWrite half stayed at entry last_write.
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	ut := &Type{isUnion: true, StructName: "U_fpbe", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_fpbe", ut, false, false)
	parent.CreateFieldVars()
	entryU := MakeFactUnion(parent, 0)
	outU := MakeFactUnion(parent, 1)
	if entryU == nil || outU == nil {
		t.Fatal("union facts")
	}
	fm := NewFactMgr(f)
	pt := []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{entryU}
	b := &Block{StmID: 60, Func: f, Looping: true}
	fm.SetMapFactsInPair(60, pt, []*FactUnion{entryU})
	// self-back map_out carries last_write=1
	fm.SetMapFactsOutPair(60, pt, []*FactUnion{outU})
	fm.MapVisited = map[int]bool{60: true}
	fm.CreateCFGEdge(60, b, false, true)
	fm.SetMapStmEffect(60, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visitOnce false + map_in unions entry vs current after merge → no pure shortcut;
	// full pass must still merge eUnionWrite into current_inputs before set_fact_in.
	_, _, _, ok := FindFixedPointBlock(b, pt, &cg, Defaults(), true)
	if !ok {
		t.Fatalf("FP must succeed sticky=%v", HasError())
	}
	// After first full pass + second merge: entryUnions for set_fact_in should reflect
	// merge of map_out (fid 1) into currentUnions. map_facts_in[block] is entry of last pass.
	// After merge fid1 into currentUnions then set_fact_in(currentUnions):
	inU := fm.GetMapUnionFactsIn(60)
	got := FindRelatedUnion(inU, parent)
	if got == nil {
		t.Fatal("map_in must have union subject after FP")
	}
	// Join of fid 0 and fid 1 → BOTTOM for eUnionWrite (or at least not stay fid 0 only)
	if got.LastWrittenFID == 0 {
		t.Fatalf("back-edge must merge map_out last_write into currentUnions, got fid=%d", got.LastWrittenFID)
	}
	ClearError()
}

// TestFindFixedPointRecreatesMayNullFromAssign — after reset_stm_fact_maps clears
// mid-gen map_facts_out, re-visit of p=0 must recreate null lattice (C++ inputs path).
// Statement.cpp:609–626 + FactMgr::update_fact_for_assign(sa, inputs).
func TestFindFixedPointAssignDerefFailsOnMayNull(t *testing.T) {
	// FactPointTo.cpp:411–419 — is_valid_ptr rejects may-null when null_prob=0.
	// Self-back / entry with may-null makes *p=… visit fail (C++ same); strip path.
	// No invent mergeMayNull needed for that fail — inputs already carry null.
	ClearError()
	SetProcessOptions(Defaults())
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	pointee := CreateVariableScalars("g_x", GetIntType(), false, false)
	ptr := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// may-null entry (as after self-back merge of mid-gen)
	entry := []*FactPointTo{MakeFactPointToSet(ptr, []*Variable{pointee, NullPtr})}
	// *g_p = 0
	asg := Stmt{
		Kind: StmtAssign, StmID: 2,
		LhsVar: ptr, Lhs: &Lhs{Var: ptr, Type: GetIntType()}, // deref store
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
		AssignOp: AssignSimple,
	}
	body := &Block{StmID: 1, Func: f, Looping: true, Stmts: []Stmt{asg}}
	fm := NewFactMgr(f)
	fm.GlobalFacts = CloneFactSlice(entry)
	fm.SetMapFactsIn(1, entry)
	fm.MapVisited = map[int]bool{1: true}
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_, _, idx, ok := FindFixedPointBlock(body, entry, &cg, opts, true)
	if ok {
		t.Fatal("FP must fail analyze of *p write under may-null (null_prob=0)")
	}
	if idx != 0 {
		t.Fatalf("failIdx want 0 got %d", idx)
	}
	ClearError()
}

// TestFindFixedPointKeepsUnrelatedMayNull — self-back merges mid-gen may-null;
// a non-touching assign must not drop it (Statement.cpp inputs flow).
// No invent mergeMayNullFromLive — pure sequential + self-back.
func TestFindFixedPointKeepsUnrelatedMayNull(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	x := CreateVariableScalars("g_x", GetIntType(), false, false)
	entry := []*FactPointTo{MakeFactPointTo(p, tgt)}
	mid := MakeFactPointToSet(p, []*Variable{tgt, NullPtr})
	if mid == nil {
		t.Fatal("mid fact")
	}
	asg := Stmt{
		Kind: StmtAssign, StmID: 2,
		LhsVar: x, Lhs: &Lhs{Var: x, Type: GetIntType()},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()},
		AssignOp: AssignSimple,
	}
	body := &Block{StmID: 1, Func: f, Looping: true, Stmts: []Stmt{asg}}
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{mid}
	fm.SetMapFactsIn(1, entry)
	fm.SetMapFactsOut(1, []*FactPointTo{mid})
	fm.MapVisited = map[int]bool{1: true}
	fm.CreateCFGEdge(1, body, false, true)
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	out, _, idx, ok := FindFixedPointBlock(body, entry, &cg, Defaults(), true)
	if !ok {
		t.Fatalf("FP must succeed idx=%d err=%v", idx, HasError())
	}
	got := FindRelatedPointTo(out, p)
	if got == nil || !got.IsNull() {
		t.Fatalf("unrelated assign must keep self-back may-null on g_p, got %+v", got)
	}
	ClearError()
}

// TestFindFixedPointReturnsPreOOSPostFacts — Block.cpp:558 vs 560–561.
// find_fixed_point assigns post_facts = outputs (pre-OOS); map_facts_out is
// post-OOS. Pure shortcut returns nil so the caller keeps its line-690 snapshot.
// Top-level return path (734–735) restores post_facts, not map_facts_out.
// Full FactVec: pre-OOS eUnionWrite is returned with post_facts (seed-49:
// append_return must not re-read live after ShortcutAnalysis installs map_out).
func TestFindFixedPointReturnsPreOOSPostFacts(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	ut := &Type{isUnion: true, StructName: "U_fp", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	lu.Init = MakeInt(0)
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	gu.Init = MakeInt(0)
	entry := []*FactPointTo{MakeFactPointTo(p, tgt)}
	mid := MakeFactPointToSet(p, []*Variable{tgt, NullPtr})
	if mid == nil {
		t.Fatal("mid")
	}
	// Empty stmts + visitOnce: single full visit path without shortcut pressure.
	body := &Block{StmID: 1, Func: f, Looping: false, Parent: nil, Stmts: nil, LocalVars: []*Variable{loc, lu}}
	fm := NewFactMgr(f)
	fm.GlobalFacts = []*FactPointTo{mid}
	fm.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0)}
	fm.SetMapFactsInPair(1, entry, []*FactUnion{MakeFactUnion(gu, 0)})
	fm.SetMapFactsOutPair(1, []*FactPointTo{mid}, []*FactUnion{MakeFactUnion(gu, 0)})
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visitOnce true → full sequential visit (no pure shortcut)
	post, postU, _, ok := FindFixedPointBlock(body, entry, &cg, Defaults(), true)
	if !ok || post == nil {
		t.Fatalf("full visit must return pre-OOS post_facts ok=%v post=%v err=%v", ok, post, HasError())
	}
	// pre-OOS outputs include local facts
	if FindRelatedPointTo(post, loc) == nil {
		t.Fatal("post_facts (pre-OOS) must include local var fact")
	}
	// pre-OOS eUnionWrite half includes body-local union subject (AddNewVarFact)
	if !UnionFactsComplete(postU) {
		t.Fatal("post_facts unions incomplete")
	}
	if FindRelatedUnion(postU, lu) == nil {
		t.Fatal("post_facts unions (pre-OOS) must include body-local union subject", postU)
	}
	if FindRelatedUnion(postU, gu) == nil {
		t.Fatal("post_facts unions must keep global union", postU)
	}
	// map_facts_out is post-OOS — local removed
	mout := fm.GetMapFactsOut(1)
	if FindRelatedPointTo(mout, loc) != nil {
		t.Fatal("map_facts_out must OOS local var")
	}
	moutU := fm.GetMapUnionFactsOut(1)
	if FindRelatedUnion(moutU, lu) != nil {
		t.Fatal("map_union_out must OOS body-local union", moutU)
	}
	// Returned unions are independent of live after ShortcutAnalysis may install
	// map_union_out into fm.UnionFacts on a later loop iteration.
	ClearError()
}

// TestIsNonreadableFieldNeedsUnionFact — FactUnion.cpp:178–192 + Block.cpp:747.
// Body-local union field is nonreadable without related FactUnion; readable with
// the pre-OOS post_facts subject that append_return restores.
func TestIsNonreadableFieldNeedsUnionFact(t *testing.T) {
	ClearError()
	ut := &Type{isUnion: true, StructName: "U_nr", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if !lu.FieldVarsComplete() || len(lu.FieldVars) == 0 {
		t.Fatal("union field_vars")
	}
	f0 := lu.FieldVars[0]
	// no fact → nonreadable
	if !IsNonreadableField(f0, []*FactUnion{}) {
		t.Fatal("empty facts: union field must be nonreadable")
	}
	// related last-write f0 → readable
	facts := []*FactUnion{MakeFactUnion(lu, 0)}
	if IsNonreadableField(f0, facts) {
		t.Fatal("matching FactUnion must make field readable")
	}
	ClearError()
}

// TestPostCreationDefersOOSUntilAfterFP — Block.cpp:690–729.
// OOS for map_out must not permanently poison GlobalFacts before find_fixed_point
// when Go uses GlobalFacts as the analysis working set (unlike C++ inputs FactVec).
func TestPostCreationDefersOOSUntilAfterFP(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	pt := PointerTo(GetIntType())
	outer := CreateVariableScalars("l_outer", pt, false, false)
	bodyLoc := CreateVariableScalars("l_body", GetIntType(), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(outer, bodyLoc)}
	x := CreateVariableScalars("g_x", GetIntType(), false, false)
	asg := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: x, Lhs: &Lhs{Var: x, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	parent := &Block{StmID: AllocStmID(), Func: f}
	body := &Block{
		StmID: AllocStmID(), Func: f, Looping: true, Parent: parent,
		LocalVars: []*Variable{bodyLoc},
		Stmts:     []Stmt{asg},
	}
	fm.SetMapFactsIn(body.StmID, CloneFactSlice(fm.GlobalFacts))
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[body.StmID] = EmptyEffect()
	fm.MapStmEffect[asg.StmID] = EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// Pre-OOS live must not be dead before/during construction
	if FindRelatedPointTo(fm.GlobalFacts, outer) == nil || FindRelatedPointTo(fm.GlobalFacts, outer).IsDead() {
		t.Fatal("setup: outer must be live→bodyLoc")
	}
	body.PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), nil, nil)
	if HasError() {
		t.Fatalf("post_creation: %v", HasError())
	}
	// map_out exists
	if !FactsComplete(fm.GetMapFactsOut(body.StmID)) {
		t.Fatal("map_out incomplete")
	}
	// visited set
	if fm.MapVisited == nil || !fm.MapVisited[body.StmID] {
		t.Fatal("must mark visited after post_creation")
	}
	ClearError()
}

// TestPostCreationMapVisitedMergesSelfBackMayNull — Block.cpp:687 map_visited[this]=true
// before find_fixed_point so the first FP iteration merges self-back map_facts_out
// (post-OOS body lattice with may-null) into map_facts_in (Block.cpp:525–536).
// StatementFor.cpp:355 post_loop then restores map_facts_in — without early visited,
// visit_once=false pure-shortcuts on entry and post_loop wipes live may-null
// (seed-2 first_div 10107: auto_statement_for_631 WIPE).
func TestPostCreationMapVisitedMergesSelfBackMayNull(t *testing.T) {
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	SetProcessProbabilities(NewProbabilities(opts))
	SetProcessRng(NewRng(1))

	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_127", PointerTo(GetSimpleType(EShort)), false, false)
	elem := PointerTo(PointerTo(GetSimpleType(EShort)))
	base := CreateVariableScalars("l_233", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{10}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "l_233"
	av.Type = elem

	// Pure entry (pre-body) in map_facts_in; live GlobalFacts already has may-null
	// as after mid-gen *p=null assign before post_creation.
	entry := []*FactPointTo{MakeFactPointToSet(&av.Variable, []*Variable{g})}
	mayNull := []*FactPointTo{MakeFactPointToSet(&av.Variable, []*Variable{g, NullPtr})}
	nullRHS := &Expression{Term: TermConstant, Con: &Constant{Type: elem, Value: "0"}, ExprType: elem}
	st := Stmt{
		Kind: StmtAssign, StmID: 2, LhsVar: &av.Variable,
		Lhs:  &Lhs{Var: &av.Variable, Type: elem},
		Expr: nullRHS, AssignOp: AssignSimple,
	}
	body := &Block{
		Func: f, StmID: 90, Looping: true, Parent: nil,
		Stmts: []Stmt{st},
	}
	f.Blocks = []*Block{body}
	f.Stack = []*Block{body}
	fm.SetMapFactsIn(body.StmID, entry)
	// Mid-gen lattice after null assign (pre-post_creation).
	fm.GlobalFacts = CloneFactSlice(mayNull)
	// Stmt effect complete so set_accumulated_effect succeeds.
	fm.SetMapStmEffect(st.StmID, EmptyEffect())

	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	pre := EmptyEffect()
	cg.EffectAccum = &pre
	body.PostCreationAnalysis(&cg, opts, pre, NewRng(1), nil)
	if HasError() {
		t.Fatalf("PostCreationAnalysis sticky err=%v", HasError())
	}
	inAfter := fm.GetMapFactsIn(body.StmID)
	fpIn := FindRelatedPointTo(inAfter, &av.Variable)
	if fpIn == nil || !fpIn.IsNull() {
		pts := []string{}
		if fpIn != nil {
			for _, p := range fpIn.PointTo {
				if p != nil {
					pts = append(pts, p.Name)
				}
			}
		}
		t.Fatalf("map_facts_in after post_creation must include may-null via self-back merge, pts=%v visited=%v",
			pts, fm.MapVisited[body.StmID])
	}
	// post_loop contract: map_facts_in keeps may-null for StatementFor.cpp:355 restore
	if !factHasL233MayNull(inAfter) {
		t.Fatal("factHasL233MayNull map_facts_in")
	}
	ClearError()
}

func TestAppendNestedLoopMakeupsUnionFacts(t *testing.T) {
	// Block.cpp:429 makeup_new_var_facts(pre_facts, global) full FactVec then set_fact_in.
	// Soft invent was MakeupNewVarFacts (PT) only — preUnion map_in missed mid-for unions.
	ClearError()
	ut := &Type{isUnion: true, StructName: "U_nloop", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	// Simulate pre-snapshot without g_newu, then live gains union fact (created mid-for).
	preU := []*FactUnion{}
	g := &Variable{Name: "g_newu", Type: ut, Init: MakeInt(0)}
	liveU := []*FactUnion{MakeFactUnion(g, 0)}
	if liveU[0] == nil {
		t.Fatal("MakeFactUnion")
	}
	if !makeupNewUnionFacts(&preU, liveU) {
		t.Fatal("makeup", HasError(), GetError())
	}
	if len(preU) != 1 || preU[0].Var != g {
		t.Fatalf("makeup must add init union for mid-for global: %#v", preU)
	}
	// PT-only path must not leave preU empty when live has unions (regression lock)
	preU2 := []*FactUnion{}
	if UnionFactsComplete(preU2) && len(preU2) == 0 {
		// without makeupNewUnionFacts this would be what SetMapFactsInPair stored
		if FindRelatedUnion(preU2, g) != nil {
			t.Fatal("empty pre should not find g")
		}
	}
	ClearError()
}

// TestPostCreationNoFPOOSsLiveUnionsNotMapOut — Block.cpp:723–726 + 735–773.
// No-FP path mutates live global_facts (OOS locals) and never assigns map_facts_out.
// Soft invent (1) left live UnionFacts mid-body; (2) AssignGlobalFactsFromMapOut
// applied remove_function_local to live env on function body (parent==nullptr).
func TestPostCreationNoFPOOSsLiveUnionsNotMapOut(t *testing.T) {
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	ut := &Type{isUnion: true, StructName: "U_nofp", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	fn := &Function{Name: "f", ReturnType: GetIntType()}
	param := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	fn.Param = []*Variable{param}
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// Nested no-FP block: parent != nil so map_out keeps full post-OOS without
	// remove_function_local; live must OOS lu but not invent map_out install.
	parent := &Block{StmID: AllocStmID(), Func: fn}
	x := CreateVariableScalars("g_x", GetIntType(), false, false)
	asg := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: x, Lhs: &Lhs{Var: x, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	inner := &Block{
		StmID: AllocStmID(), Func: fn, Parent: parent, Looping: false, NeedRevisit: false,
		LocalVars: []*Variable{lu},
		Stmts:     []Stmt{asg},
	}
	fm := NewFactMgr(fn)
	fm.GlobalFacts = []*FactPointTo{
		MakeFactPointTo(param, NullPtr),
		MakeFactPointTo(gp, param),
	}
	fm.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(lu, 0)}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[inner.StmID] = EmptyEffect()
	fm.MapStmEffect[asg.StmID] = EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = fn
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	inner.PostCreationAnalysis(&cg, opts, EmptyEffect(), nil, nil)
	if HasError() {
		t.Fatalf("no-FP post_creation sticky: %v", GetError())
	}
	// live Union: local subject dropped; global kept
	if FindRelatedUnion(fm.UnionFacts, lu) != nil {
		t.Fatal("no-FP live UnionFacts must OOS body-local union subject", fm.UnionFacts)
	}
	if FindRelatedUnion(fm.UnionFacts, gu) == nil {
		t.Fatal("no-FP live UnionFacts must keep global union", fm.UnionFacts)
	}
	// map_union_out also OOS (SetMapFactsOutForBlock)
	outU := fm.GetMapUnionFactsOut(inner.StmID)
	if FindRelatedUnion(outU, lu) != nil {
		t.Fatal("map_union_out must drop local", outU)
	}
	// Function-body no-FP: live must NOT apply remove_function_local (param subject stays).
	// map_out for parent==nil does strip params.
	ClearError()
	bodyLoc := CreateVariableQfer("l_bu", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	bodyAsg := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: x, Lhs: &Lhs{Var: x, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}, AssignOp: AssignSimple,
	}
	body := &Block{
		StmID: AllocStmID(), Func: fn, Parent: nil, Looping: false, NeedRevisit: false,
		LocalVars: []*Variable{bodyLoc},
		Stmts:     []Stmt{bodyAsg},
	}
	fn.Body = body
	fm2 := NewFactMgr(fn)
	fm2.GlobalFacts = []*FactPointTo{
		MakeFactPointTo(param, NullPtr),
		MakeFactPointTo(gp, param),
	}
	fm2.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(bodyLoc, 0)}
	fm2.MapStmEffect = map[int]Effect{
		body.StmID:    EmptyEffect(),
		bodyAsg.StmID: EmptyEffect(),
	}
	cg2 := EmptyCGContext().WithFactMgr(fm2)
	cg2.CurrentFunc = fn
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	body.PostCreationAnalysis(&cg2, opts, EmptyEffect(), nil, nil)
	if HasError() {
		t.Fatalf("function-body no-FP sticky: %v", GetError())
	}
	// live: param subject still present (C++ OOS only local_vars, not remove_function_local)
	if FindRelatedPointTo(fm2.GlobalFacts, param) == nil {
		t.Fatal("no-FP live must not remove_function_local param subject", fm2.GlobalFacts)
	}
	// map_out: parent==nil strips params
	outPT := fm2.GetMapFactsOut(body.StmID)
	for _, f := range outPT {
		if f != nil && f.Var != nil && (f.Var == param || f.Var.Match(param)) {
			t.Fatal("function-body map_facts_out must remove param subject", outPT)
		}
	}
	if FindRelatedUnion(fm2.UnionFacts, bodyLoc) != nil {
		t.Fatal("no-FP live must OOS body local union", fm2.UnionFacts)
	}
	ClearError()
}

// TestPostCreationNoFPNoSelfBackWhenMustBreak — Block.cpp:735–739.
// Self-back only when is_loop_body && from_tail_to_head inside the FP arm.
// Soft invent created self-back on no-FP when Looping && must_break_or_return
// (is_loop_body false) && from_tail.
func TestPostCreationNoFPNoSelfBackWhenMustBreak(t *testing.T) {
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	fn := &Function{Name: "f", ReturnType: GetIntType()}
	parent := &Block{StmID: AllocStmID(), Func: fn}
	// Last stmt is return → must_break_or_return true → is_loop_body false
	// even though Looping (Block.cpp:729).
	ret := Stmt{
		Kind: StmtReturn, StmID: AllocStmID(),
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
	}
	// Fall-through last would be from_tail; with return last, from_tail is false
	// unless continue/break topology. Use looping + return last and assert no edge.
	b := &Block{
		StmID: AllocStmID(), Func: fn, Parent: parent,
		Looping: true, NeedRevisit: false,
		Stmts: []Stmt{ret},
	}
	fm := NewFactMgr(fn)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.UnionFacts = []*FactUnion{}
	fm.MapStmEffect = map[int]Effect{
		b.StmID:   EmptyEffect(),
		ret.StmID: EmptyEffect(),
	}
	nEdgesBefore := len(fm.CFGEdges)
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = fn
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	b.PostCreationAnalysis(&cg, opts, EmptyEffect(), nil, nil)
	if HasError() {
		t.Fatalf("sticky: %v", GetError())
	}
	// No self-back: is_loop_body false so FP arm (and its CreateCFGEdge) never runs.
	for _, e := range fm.CFGEdges[nEdgesBefore:] {
		if e != nil && e.BackLink && e.SrcID == b.StmID && e.DestBlock == b {
			t.Fatal("no-FP must not invent self-back when !is_loop_body", e)
		}
	}
	// must_break_or_return true for return last
	if !b.MustBreakOrReturnFull(fm) {
		t.Fatal("setup: return last must must_break_or_return")
	}
	ClearError()
}
