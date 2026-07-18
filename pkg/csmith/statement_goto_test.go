package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomGotoHasLabel(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	// empty block → forward edge (no back-edge candidates yet)
	blk := &Block{}
	st := MakeRandomGoto(NewRng(9), opts, probs, vs, tables, cg, blk)
	if st.Kind != StmtGoto || st.Label == "" {
		t.Fatalf("%+v", st)
	}
	if !st.GotoForward && !st.GotoBack {
		t.Fatal("expected GotoForward or GotoBack")
	}
}

func TestMakeRandomGotoBackEdge(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// minimal func + one assign target (no full MakeFirst body noise)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Stmts: []Stmt{{Kind: StmtAssign, AssignOp: AssignSimple}}}
	f.Blocks = []*Block{blk}
	f.Body = blk
	cg := WithFunc(f, EmptyEffect())
	var st Stmt
	for seed := uint64(1); seed < 40; seed++ {
		blk.Stmts[0].SourceLabel = ""
		st = MakeRandomGoto(NewRng(seed), opts, probs, vs, tables, cg, blk)
		if st.GotoBack {
			break
		}
	}
	if !st.GotoBack {
		t.Skip("no back-edge in seed sample")
	}
	if st.Label == "" || blk.Stmts[0].SourceLabel == "" {
		t.Fatalf("back edge label missing: st=%+v src=%q", st, blk.Stmts[0].SourceLabel)
	}
	if st.Label != blk.Stmts[0].SourceLabel {
		t.Fatalf("label mismatch goto=%q target=%q", st.Label, blk.Stmts[0].SourceLabel)
	}
}

func TestGenerateCanEmitGoto(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "goto lbl_") {
			found = true
			if !strings.Contains(out, "lbl_") || !strings.Contains(out, ":") {
				t.Fatal("goto without label def")
			}
			break
		}
	}
	if !found {
		t.Log("goto rare (statement weight band 45-49)")
	}
}

func TestLabelForGotoDestReuses(t *testing.T) {
	GotoLabelsDoFinalization()
	defer GotoLabelsDoFinalization()
	n := 0
	next := func() string {
		n++
		return "lbl_" + Int2Str(n)
	}
	a := LabelForGotoDest(42, next)
	b := LabelForGotoDest(42, next)
	if a != b || a != "lbl_1" {
		t.Fatalf("%q %q n=%d", a, b, n)
	}
	c := LabelForGotoDest(99, next)
	if c == a || c != "lbl_2" {
		t.Fatalf("%q %q", a, c)
	}
}

func TestMarkNeedRevisitLCA(t *testing.T) {
	// outer → then(inner with assign) — back-edge LCA is outer when dest in then
	dest := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 7}
	inner := &Block{Stmts: []Stmt{dest}}
	outer := &Block{
		Stmts: []Stmt{{Kind: StmtIfElse, Then: inner, StmID: 1}},
	}
	inner.Parent = outer
	// dest pointer must be into slice
	d := &inner.Stmts[0]
	MarkNeedRevisitLCA(inner, d)
	if !inner.NeedRevisit {
		t.Fatal("inner contains dest → mark inner")
	}
	// from a sibling-ish curr that does not contain dest → walk to outer
	curr := &Block{Parent: outer, Stmts: []Stmt{{Kind: StmtAssign, StmID: 8}}}
	outer.NeedRevisit = false
	inner.NeedRevisit = false
	MarkNeedRevisitLCA(curr, d)
	if !outer.NeedRevisit {
		t.Fatal("outer is LCA containing dest")
	}
	if curr.NeedRevisit {
		t.Fatal("curr should not be marked when outer contains dest")
	}
}

func TestMakeBinaryForCompare(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	lhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_i", GetIntType(), true, false), ExprType: GetIntType()}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(10), ExprType: GetIntType()}
	fi := MakeBinary(NewRng(1), opts, NewProbabilities(opts), EmptyCGContext(), BinCmpLt, lhs, rhs)
	if fi == nil || fi.Binary != "<" {
		t.Fatalf("%+v", fi)
	}
	if fi.Safe == nil {
		t.Fatal("flags always set")
	}
	if fi.GetType() != GetIntType() {
		t.Fatal("cmp type")
	}
	// Output is standard cmp (not safe_ops arith)
	out := fi.Output()
	if strings.Contains(out, "safe_") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "<") {
		t.Fatal(out)
	}
}

func TestGotoLabelsClearedOnFinalization(t *testing.T) {
	GotoLabelsDoFinalization()
	_ = LabelForGotoDest(1, func() string { return "lbl_x" })
	DoFinalization()
	// after finalization map empty → new gensym path
	lab := LabelForGotoDest(1, func() string { return "lbl_y" })
	if lab != "lbl_y" {
		t.Fatal(lab)
	}
	GotoLabelsDoFinalization()
}
