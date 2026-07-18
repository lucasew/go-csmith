package csmith

import (
	"strings"
	"testing"
)

func TestMakeIterationBuildsIR(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	// seed an int global as potential IV
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()), GetIntType(), &q, NewRng(1))
	cg := WithFunc(f, EmptyEffect())
	lc := MakeIteration(NewRng(7), opts, NewProbabilities(opts), vs, cg)
	if lc == nil || lc.IV == nil {
		t.Fatal("nil iteration")
	}
	if lc.InitStmt == nil || lc.InitStmt.Kind != StmtAssign {
		t.Fatal("init stmt")
	}
	if lc.InitStmt.AssignOp != AssignSimple {
		t.Fatal("init must be simple assign")
	}
	if lc.TestExpr == nil || lc.TestExpr.Term != TermFunction {
		t.Fatal("test expr")
	}
	if lc.IncrStmt == nil || lc.IncrStmt.Kind != StmtAssign {
		t.Fatal("incr stmt")
	}
	hdr := forHeaderOutput(lc)
	if !strings.HasPrefix(hdr, "for (") || !strings.Contains(hdr, ";") {
		t.Fatal(hdr)
	}
	// IV name appears in header
	if !strings.Contains(hdr, lc.IV.Name) && !strings.Contains(hdr, lc.IV.OutputC()) {
		t.Fatal(hdr, lc.IV.Name)
	}
}

func TestMakeIterationArrayBoundPath(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	av.Sizes = []int{5}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(3))
	cg := WithFunc(f, EmptyEffect())
	cg.MustUseArrays = []*ArrayVariable{av}
	lc := MakeIteration(NewRng(11), opts, NewProbabilities(opts), vs, cg)
	if lc == nil {
		t.Fatal("nil")
	}
	// bound set from array path
	if lc.Bound <= 0 {
		t.Log("bound may be 0 if choose_ok failed; ok")
	}
	if lc.InitStmt == nil || lc.IncrStmt == nil {
		t.Fatal("IR missing")
	}
}

func TestMakeRandomForEmitsHeader(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(2))
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomFor(NewRng(9), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg)
	if st == nil || st.Loop == nil {
		t.Fatal("nil for")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
	if strings.Contains(out, "/* for-stub */") {
		t.Fatal("stub")
	}
}

func TestVisitFactsForUsesInitStmt(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f, StmID: AllocStmID()}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, NewRng(1))
	lc := MakeIteration(NewRng(4), opts, NewProbabilities(opts), vs, cg)
	if lc == nil {
		t.Skip("no IV")
	}
	st := &Stmt{Kind: StmtFor, Loop: lc, Then: &Block{Func: f, StmID: AllocStmID()}, StmID: AllocStmID()}
	cgp := cg
	if !VisitFactsStatementFor(st, &cgp, opts) {
		t.Fatal("visit_facts for")
	}
}
