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
	b := MakeRandomBlock(NewRng(3), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), cg, true)
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
	// break with constant true test is must_jump
	b := &Block{Stmts: []Stmt{{
		Kind: StmtBreak,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}}
	if !b.MustBreakOrReturn() {
		t.Fatal("break")
	}
}
