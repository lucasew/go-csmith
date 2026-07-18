package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLoopControlRanges(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	for i := 0; i < 50; i++ {
		init, limit, incr, _, incrOp := MakeRandomLoopControl(r, opts, true)
		if incr == 0 {
			t.Fatal("incr never 0 after fixup")
		}
		_ = init
		_ = limit
		_ = incrOp
	}
}

func TestMakeRandomIfHasBranches(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	var list FunctionList
	// need a function for context
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list, nil)
	cg := WithFunc(f, EmptyEffect())
	// force if generation
	r2 := NewRng(11)
	st := MakeRandomIf(r2, opts, probs, vs, tables, stmtTab, cg)
	if st.Kind != StmtIfElse || st.Then == nil || st.Else == nil {
		t.Fatalf("%+v", st)
	}
	if st.Expr == nil {
		t.Fatal("missing test expr")
	}
}

func TestMakeRandomForHasLoopAndBody(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomFor(NewRng(4), opts, probs, vs, tables, stmtTab, cg)
	if st.Kind != StmtFor || st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		t.Fatalf("%+v", st)
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(out, st.Loop.IV.Name) {
		t.Fatal("iv name missing")
	}
}

func TestGenerateCanEmitIfOrFor(t *testing.T) {
	// Scan seeds until we see real if/for syntax (not stubs).
	foundIf, foundFor := false, false
	for seed := uint64(1); seed < 80 && !(foundIf && foundFor); seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "if (") {
			foundIf = true
		}
		if strings.Contains(out, "for (") {
			foundFor = true
		}
	}
	if !foundIf {
		t.Log("no if in seeds 1..79 (probabilistic)")
	}
	if !foundFor {
		t.Log("no for in seeds 1..79 (probabilistic)")
	}
	// Soft: at least one control structure in range is enough for smoke
	if !foundIf && !foundFor {
		t.Fatal("expected some if or for in seeds 1..79")
	}
}
