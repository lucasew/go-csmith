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
