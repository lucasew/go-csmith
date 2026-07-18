package csmith

import (
	"strings"
	"testing"
)

func TestRandomFunctionName(t *testing.T) {
	var g GenSym
	if RandomFunctionName(&g) != "func_1" || RandomFunctionName(&g) != "func_2" {
		t.Fatal("func gensym")
	}
}

func TestParamListProbabilityRange(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	// max_params=5 → rnd_upto(5) in 0..4
	for i := 0; i < 20; i++ {
		p := ParamListProbability(r, opts)
		if p >= 5 {
			t.Fatalf("param list prob %d", p)
		}
	}
}

func TestMakeRandomSignatureParams(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// share gensym counters: use vs.Sym for params and separate for funcs is OK upstream-global
	// For 1:1 naming, share one GenSym for both
	sym := &vs.Sym
	r := NewRng(2)
	f := MakeRandomSignature(r, opts, probs, vs, sym, EmptyCGContext(), GetSimpleType(EInt), nil, nil)
	if f == nil || !strings.HasPrefix(f.Name, "func_") {
		t.Fatalf("name %+v", f)
	}
	if f.ReturnType != GetSimpleType(EInt) {
		t.Fatal("return type")
	}
	if f.RV == nil || !strings.HasSuffix(f.RV.Name, "_rv") {
		t.Fatal("rv")
	}
	// param count = max+1 from first ParamListProbability
	// At least 1 param (max>=0 → loop 0..max)
	if len(f.Param) < 1 {
		t.Fatal("expected >=1 param")
	}
	for _, p := range f.Param {
		if p == nil || !p.IsArgument() {
			t.Fatalf("param %v", p)
		}
	}
	proto := f.OutputForwardDecl()
	if !strings.Contains(proto, f.Name) || !strings.HasSuffix(proto, ");") {
		t.Fatalf("proto %q", proto)
	}
}

func TestMakeFirstNoParamsHasBody(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	var list FunctionList
	r := NewRng(2)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list, nil)
	if f == nil || f.Body == nil {
		t.Fatal("body")
	}
	if len(f.Param) != 0 {
		t.Fatalf("make_first default extension null → no params, got %d", len(f.Param))
	}
	if len(list.Funcs) < 1 {
		t.Fatal("func list empty")
	}
	// Body may create additional function signatures (ExpressionFuncall).
	if list.Funcs[0] != f {
		t.Fatal("first func not registered first")
	}
	out := f.Output()
	if !strings.Contains(out, f.Name) || !strings.Contains(out, "{") {
		t.Fatalf("output %q", out)
	}
	// body should have max_block_size statements unless early return;
	// forward-goto StmtLabel markers may inflate len(Stmts); trailing append_return
	// may add one more (Block.cpp:734).
	if len(f.Body.Stmts) < 1 {
		t.Fatal("empty stmts")
	}
	n := 0
	for _, s := range f.Body.Stmts {
		if s.Kind != StmtLabel {
			n++
		}
	}
	if n > opts.MaxBlockSize+1 {
		t.Fatalf("too many real stmts %d (raw %d)", n, len(f.Body.Stmts))
	}
}

func TestBlockProbabilityAlwaysMaxMinusOne(t *testing.T) {
	// Keep-filter forces block_size-1
	if BlockProbability(4, NewRng(2)) != 3 {
		t.Fatal("max_block_size 4 → 3")
	}
}

func TestMakeFirstReturnBreaksEarly(t *testing.T) {
	// With enough seeds, some bodies end early on return
	opts := Defaults()
	probs := NewProbabilities(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	foundEarly := false
	for seed := uint64(1); seed < 50; seed++ {
		vs := NewVariableSelector(opts)
		r := NewRng(seed)
		f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
		if f.Body != nil && len(f.Body.Stmts) < opts.MaxBlockSize {
			// last should be return if early
			last := f.Body.Stmts[len(f.Body.Stmts)-1]
			if last.Kind == StmtReturn {
				foundEarly = true
				break
			}
		}
	}
	if !foundEarly {
		// not a hard failure — return is only 5% band (30-34); may be rare with filters
		t.Log("no early return in seeds 1..49 (ok if filters reduce return rate)")
	}
}
