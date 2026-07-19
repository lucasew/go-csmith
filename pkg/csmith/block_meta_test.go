package csmith

import (
	"strings"
	"testing"
)

func TestFromTailToHead(t *testing.T) {
	b := &Block{Looping: true, Stmts: []Stmt{
		{Kind: StmtAssign},
	}}
	if !b.FromTailToHead() {
		t.Fatal("fall through")
	}
	b.Stmts = []Stmt{{Kind: StmtReturn}}
	if b.FromTailToHead() {
		t.Fatal("return must_jump")
	}
	b.Looping = false
	if b.FromTailToHead() {
		t.Fatal("not looping")
	}
}

func TestGetLastStmStopsAtReturn(t *testing.T) {
	b := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtReturn, StmID: 2},
		{Kind: StmtAssign, StmID: 3},
	}}
	if b.GetLastStm() == nil || b.GetLastStm().StmID != 2 {
		t.Fatal(b.GetLastStm())
	}
}

func TestSetAccumulatedEffect(t *testing.T) {
	fm := NewFactMgr(nil)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	fm.SetMapStmEffect(1, EmptyEffect().WriteVar(v))
	fm.SetMapStmEffect(2, EmptyEffect().ReadVar(v))
	b := &Block{
		StmID: 10,
		Stmts: []Stmt{{StmID: 1}, {StmID: 2}},
	}
	eff := b.SetAccumulatedEffect(fm)
	if !eff.IsWritten(v) || !eff.IsRead(v) {
		t.Fatal("union")
	}
	if !fm.GetMapStmEffect(10).IsWritten(v) {
		t.Fatal("block effect")
	}
	// StmID 0 incomplete — IncompleteEffect (not EmptyEffect invent pure/empty success)
	b2 := &Block{StmID: 11, Stmts: []Stmt{{StmID: 1}, {StmID: 0}}}
	fm.SetMapStmEffect(11, EmptyEffect().WriteVar(v))
	eff2 := b2.SetAccumulatedEffect(fm)
	if EffectComplete(eff2) || eff2.IsEmpty() || eff2.IsPure() {
		t.Fatal("StmID 0 must fail closed IncompleteEffect, not invent empty/pure", eff2)
	}
	if EffectComplete(fm.GetMapStmEffect(11)) || fm.GetMapStmEffect(11).IsWritten(v) {
		t.Fatal("block map must IncompleteEffect, not invent partial write")
	}
	// nil block/fm must IncompleteEffect (not invent EmptyEffect pure)
	if EffectComplete(((*Block)(nil)).SetAccumulatedEffect(fm)) {
		t.Fatal("nil block must IncompleteEffect")
	}
}

func TestRandomParentBlock(t *testing.T) {
	outer := &Block{}
	inner := &Block{Parent: outer}
	seen := map[*Block]bool{}
	r := NewRng(1)
	for i := 0; i < 40; i++ {
		seen[inner.RandomParentBlock(r, true)] = true
	}
	// nil (global), outer, inner
	if len(seen) < 2 {
		t.Fatal(seen)
	}
	// without global
	seen2 := map[*Block]bool{}
	for i := 0; i < 20; i++ {
		p := inner.RandomParentBlock(NewRng(uint64(i+2)), false)
		if p == nil {
			t.Fatal("nil without global")
		}
		seen2[p] = true
	}
}

func TestLabelAttrEmit(t *testing.T) {
	ClearAttrGenerators()
	labelAttrGenerator = &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "hot", Prob: 100},
	}}
	b := &Block{
		EmitLabelAttrs: true,
		LabelAttrRng:   NewRng(1),
		Stmts: []Stmt{{
			Kind: StmtAssign, SourceLabel: "lbl_1",
			LhsVar: CreateVariableScalars("g_1", GetIntType(), false, false),
			AssignOp: AssignSimple,
			Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)},
		}},
	}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_1:") || !strings.Contains(out, "hot") {
		t.Fatal(out)
	}
	ClearAttrGenerators()
}

func TestLoopSelfBackEdgeOnPostCreation(t *testing.T) {
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// make a small looping block
	b := MakeRandomBlock(NewRng(3), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, true)
	if b == nil {
		t.Fatal("nil")
	}
	// if fall-through possible, self back edge exists
	if b.FromTailToHead() {
		found := false
		for _, e := range fm.CFGEdges {
			if e != nil && e.BackLink && e.DestBlock == b {
				found = true
			}
		}
		if !found {
			t.Fatal("missing self back edge", fm.CFGEdges)
		}
	}
}

func TestMustBreakOrReturn(t *testing.T) {
	// Block.cpp:342–357 — last must_return (break alone is not enough)
	b := &Block{Stmts: []Stmt{{
		Kind: StmtBreak,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}}
	if b.MustBreakOrReturn() {
		t.Fatal("break is not must_return")
	}
	b.Stmts = []Stmt{{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
	}}
	if !b.MustBreakOrReturn() {
		t.Fatal("return must_break_or_return")
	}
}

func TestMustReturnBreakStmsAndBackEdge(t *testing.T) {
	// Block.cpp:313–331 — break_stms nonempty → not must_return
	ret := Stmt{Kind: StmtReturn, StmID: 2, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	b := &Block{StmID: 1, Stmts: []Stmt{ret}, BreakStmIDs: []int{9}}
	if b.MustReturn() {
		t.Fatal("break_stms blocks must_return")
	}
	b.BreakStmIDs = nil
	if !b.MustReturn() {
		t.Fatal("return last")
	}
	// continue edge into block escapes
	fm := NewFactMgr(nil)
	b.EmitFM = fm
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestBlock: b, BackLink: true}}
	if b.MustReturn() {
		t.Fatal("back edge escapes")
	}
	// Block StmID 0 + FM fails closed as escape (no invent "no back edge")
	b0 := &Block{StmID: 0, Stmts: []Stmt{ret}, EmitFM: fm}
	if b0.MustReturn() {
		t.Fatal("block StmID 0 must fail closed not must_return")
	}
	// MustJump also requires empty break_stms
	b2 := &Block{Stmts: []Stmt{{
		Kind: StmtBreak, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}, BreakStmIDs: []int{1}}
	if b2.MustJump() {
		t.Fatal("break_stms nonempty")
	}
	b2.BreakStmIDs = nil
	if !b2.MustJump() {
		t.Fatal("true break must_jump")
	}
}

func TestBlockOutputBlockIDComment(t *testing.T) {
	// Block.cpp:250–253 — "{ " + /* block id: N */
	b := &Block{StmID: 42, Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 1,
		LhsVar: CreateVariableScalars("g_1", GetIntType(), false, false),
		Expr:   &Expression{Term: TermConstant, Con: MakeInt(0)},
		AssignOp: AssignSimple,
	}}}
	out := b.Output(0)
	if !strings.Contains(out, "/* block id: 42 */") {
		t.Fatal(out)
	}
	// concise skips comment body of OutputCommentLine when we gate EmitConcise
	b.EmitConcise = true
	out2 := b.Output(0)
	if strings.Contains(out2, "block id:") {
		t.Fatal("concise should skip block id", out2)
	}
}
