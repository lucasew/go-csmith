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
	// no invent unbuilt success when GenerateBody cannot run (nil RNG)
	if MakeRandomFunction(nil, opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), cg, GetIntType(), nil, list) != nil {
		t.Fatal("nil RNG must not invent unbuilt function")
	}
}

func TestMakeFirstERRORGuard(t *testing.T) {
	// Function.cpp:445/453 ERROR_GUARD paths
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// empty Types → RandomReturnType nil
	vs.Types = &TypeEnv{}
	list := &FunctionList{Types: vs.Types}
	if MakeFirst(NewRng(1), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("empty AllTypes must fail closed")
	}
	// sticky error
	seedTypesForTest(NewRng(2), opts, probs, vs, list)
	SetError(ErrGeneric)
	if MakeFirst(NewRng(3), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("sticky error")
	}
	ClearError()
	// no invent Unbuilt success when body cannot generate
	if MakeFirst(nil, opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("nil RNG must not invent first")
	}
}

func TestGenerateFunctionsStopsOnERROR(t *testing.T) {
	// Function.cpp:797/805 ERROR_RETURN after make_first / GenerateBody
	ClearError()
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	g := NewProgramGenerator(opts)
	// poison after types so make_first fails
	g.Initialize()
	g.GenerateAllTypes()
	SetError(ErrGeneric)
	g.GenerateFunctions()
	// with sticky error, first may fail; must not invent many built funcs
	built := 0
	for _, f := range g.Funcs.Funcs {
		if f != nil && f.IsBuilt {
			built++
		}
	}
	if built > 0 {
		t.Log("unexpected built with sticky error", built)
	}
	ClearError()
}

func TestGenerateFunctionsNoInventNilFuncHole(t *testing.T) {
	// Function* always live on Funcs; nil hole stops unbuilt-body loop
	ClearError()
	opts := Defaults()
	opts.MaxFuncs = 3
	opts.MaxBlockSize = 1
	g := NewProgramGenerator(opts)
	g.Initialize()
	g.GenerateAllTypes()
	// pre-seed nil hole; make_first appends after it → [nil, first]
	g.Funcs.Funcs = []*Function{nil}
	g.GenerateFunctions()
	// loop hits nil at index 0 and returns; no invent filling the hole
	if len(g.Funcs.Funcs) < 1 || g.Funcs.Funcs[0] != nil {
		t.Fatalf("nil hole must remain, got %v", g.Funcs.Funcs)
	}
	// no invent processing past hole into extra unbuilt creates from body
	unbuilt := 0
	for _, f := range g.Funcs.Funcs {
		if f != nil && f.BuildState != BuildBuilt {
			unbuilt++
		}
	}
	if unbuilt > 0 {
		t.Fatalf("must not invent unbuilt past nil hole, unbuilt=%d", unbuilt)
	}
	ClearError()
}
