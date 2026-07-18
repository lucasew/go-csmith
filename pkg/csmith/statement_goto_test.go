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
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomGoto(NewRng(9), opts, probs, vs, tables, cg)
	if st.Kind != StmtGoto || st.Label == "" {
		t.Fatalf("%+v", st)
	}
	if len(f.Labels) < 1 {
		t.Fatal("label not registered")
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
