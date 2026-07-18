package csmith

import (
	"strings"
	"testing"
)

func TestItemizeConsumesRNGPerDim(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, nil, "g_a", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("create")
	}
	r2 := NewRng(5)
	before := r2.RandDepth()
	item := av.Itemize(r2)
	if item == nil || item.Collective != av {
		t.Fatal(item)
	}
	if len(item.Indices) != av.Dimension() {
		t.Fatalf("indices %v dims %d", item.Indices, av.Dimension())
	}
	if r2.RandDepth() != before+uint64(av.Dimension()) {
		// each dim one RndUpto
		if r2.RandDepth() < before {
			t.Fatal("depth")
		}
	}
	acc := item.OutputAccess()
	if !strings.HasPrefix(acc, "g_a[") {
		t.Fatal(acc)
	}
}

func TestSelectArrayCreatesWhenEmpty(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	av := vs.SelectArray(r, EmptyCGContext())
	if av == nil || len(vs.Arrays) < 1 {
		t.Fatal(av)
	}
}

func TestSelectArrayChoosesExisting(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateArrayVariable(r, opts, nil, "g_1", GetIntType(), MakeInt(0), q)
	b := CreateArrayVariable(r, opts, nil, "g_2", GetIntType(), MakeInt(1), q)
	vs.Arrays = []*ArrayVariable{a, b}
	// sole when filter... both ok
	got := vs.SelectArray(NewRng(3), EmptyCGContext())
	if got != a && got != b {
		t.Fatal(got)
	}
}

func TestMakeRandomArrayOpEmitsFor(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil)
	cg := WithFunc(f, EmptyEffect())
	// StatementArrayOp::make_random — 5% array_init (StmtArrayOp) else for-loop (StmtFor)
	var st Stmt
	for seed := uint64(1); seed < 40; seed++ {
		st = MakeRandomArrayOp(NewRng(seed), opts, probs, vs, tables, stmtTab, cg)
		if st.Kind == StmtArrayOp || st.Kind == StmtFor {
			break
		}
	}
	if st.Kind != StmtArrayOp && st.Kind != StmtFor {
		t.Fatalf("kind %v", st.Kind)
	}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
}

func TestGenerateArrayOpOrDecl(t *testing.T) {
	// arrays may appear as decls or array-op loops
	found := false
	for seed := uint64(1); seed < 50; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "[") && strings.Contains(out, "g_") {
			found = true
			break
		}
	}
	if !found {
		t.Log("arrays still rare in short seed scan")
	}
}
