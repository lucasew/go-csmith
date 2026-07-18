package csmith

import "testing"

func TestMakeFirstSetupInOutMaps(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 2
	opts.MaxBlockDepth = 2
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	fmMap := NewFactMgrMap()
	seedTypesForTest(NewRng(5), opts, NewProbabilities(opts), vs, list)
	f := MakeFirst(NewRng(5), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, fmMap)
	if f == nil || f.Body == nil {
		t.Fatal("nil first")
	}
	if f.BuildState != BuildBuilt {
		t.Fatal(f.BuildState)
	}
	fm := fmMap.ForFunc(f)
	if fm == nil {
		t.Fatal("no fm")
	}
	// setup_in_out_maps should have populated maps when body has stm ids
	if f.Body.StmID > 0 {
		// maps may be empty of facts but should not panic
		_ = fm.MapFactsOut[f.Body.StmID]
	}
	if !f.Body.EmitDepthProtect {
		t.Fatal("body depth_protect")
	}
}

func TestMakeRandomFunction(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	opts.MaxFuncs = 5
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	list := &FunctionList{}
	// seed first so list non-empty for choose
	seedTypesForTest(NewRng(2), opts, NewProbabilities(opts), vs, list)
	_ = MakeFirst(NewRng(2), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil)
	cg := EmptyCGContext()
	cg.Funcs = list
	f := MakeRandomFunction(NewRng(3), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), cg, GetIntType(), nil, list)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Body == nil || f.BuildState != BuildBuilt {
		t.Fatal("not built")
	}
	if len(f.Param) == 0 {
		// param list probability can yield 0 params when max is 0... max is usually >0 so i<=max at least 1
		// actually max=0 → i=0 only → 1 param; max can be 0 with ParamListProbability
		t.Log("params", len(f.Param))
	}
}
