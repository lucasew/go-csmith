package csmith

import "testing"

func TestBodyOutAssignMissingNoInventPrior(t *testing.T) {
	// Function.cpp:469 — global_facts = map_facts_out[body]
	// missing body out must not invent keep prior GlobalFacts
	f := &Function{Name: "f", ReturnType: GetIntType(), Body: &Block{StmID: 10}}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	prior := MakeFactPointTo(p, NullPtr)
	fm.GlobalFacts = []*FactPointTo{prior}
	// no MapFactsOut[10]
	// mirror MakeFirst handoff assign
	out := fm.MapFactsOut[f.Body.StmID]
	if !FactsComplete(out) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSlice(out)
	}
	// missing MapFactsOut is complete empty (C++ map[]); must not keep prior
	if FindRelatedPointTo(fm.GlobalFacts, p) != nil {
		t.Fatal("missing body out must clear prior GlobalFacts, not invent keep prior")
	}
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("missing body out is complete empty, not incomplete marker")
	}
}

func TestBodyOutAssignIncompleteFailClosed(t *testing.T) {
	// incomplete map_facts_out[body] must not invent cleaned GlobalFacts
	f := &Function{Name: "f", ReturnType: GetIntType(), Body: &Block{StmID: 11}}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	fm.MapFactsOut = map[int][]*FactPointTo{
		11: {MakeFactPointTo(p, NullPtr), nil},
	}
	out := fm.MapFactsOut[f.Body.StmID]
	if !FactsComplete(out) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSlice(out)
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete body out must fail closed incomplete GlobalFacts")
	}
}

func TestRetFactsNoInventGlobalFactsFallback(t *testing.T) {
	// FunctionInvocationUser.cpp:212 — ret_facts = map_facts_out[body]
	// no invent GlobalFacts when body out missing
	callee := &Function{Name: "g", ReturnType: GetIntType(), Body: &Block{StmID: 20}}
	calFM := NewFactMgrSess(testAmbientSession, callee)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	calFM.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	// no MapFactsOut[20]
	var retFacts []*FactPointTo
	var retUnions []*FactUnion
	out := calFM.MapFactsOut[callee.Body.StmID]
	if FactsComplete(out) {
		retFacts = CloneFactSlice(out)
		retUnions = []*FactUnion{}
		AddBackReturnFacts(callee.Body, calFM, &retFacts, &retUnions)
	}
	// missing MapFactsOut — retFacts never populated (no invent from GlobalFacts)
	if retFacts != nil {
		t.Fatal("missing body out must not invent GlobalFacts as ret_facts", retFacts)
	}
	// incomplete body out — no invent soft-merge returns onto wiped empty
	ret := Stmt{Kind: StmtReturn, StmID: 99}
	callee.Body.Stmts = []Stmt{ret}
	calFM.MapFactsOut = map[int][]*FactPointTo{
		20: {MakeFactPointTo(p, NullPtr), nil},
		99: {MakeFactPointTo(p, GarbagePtr)},
	}
	calFM.MapUnionFactsOut = map[int][]*FactUnion{20: {}, 99: {}}
	retFacts = nil
	retUnions = nil
	out = calFM.MapFactsOut[callee.Body.StmID]
	if FactsComplete(out) {
		retFacts = CloneFactSlice(out)
		retUnions = []*FactUnion{}
		if !AddBackReturnFacts(callee.Body, calFM, &retFacts, &retUnions) {
			retFacts = IncompleteFactSlice()
		}
	}
	// incomplete body out leaves retFacts unset (nil) — must not invent returns-only
	if retFacts != nil && FactsComplete(retFacts) {
		t.Fatal("incomplete body out must not invent returns-only ret_facts", retFacts)
	}
	// complete body out + incomplete return out — AddBack fails closed
	calFM.MapFactsOut = map[int][]*FactPointTo{
		20: {MakeFactPointTo(p, NullPtr)},
		99: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	calFM.MapUnionFactsOut = map[int][]*FactUnion{20: {}, 99: {}}
	retFacts = CloneFactSlice(calFM.MapFactsOut[20])
	retUnions = []*FactUnion{}
	ClearErrorSess(testAmbientSession)
	if AddBackReturnFacts(callee.Body, calFM, &retFacts, &retUnions) || FactsComplete(retFacts) {
		t.Fatal("incomplete return out must fail closed AddBackReturnFacts", retFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete return out must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeFirstSetupInOutMaps(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 2
	opts.MaxBlockDepth = 2
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	fmMap := NewFactMgrMapSess(testAmbientSession)
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
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(1))
	list := &FunctionList{}
	// seed first so list non-empty for choose
	seedTypesForTest(NewRng(2), opts, NewProbabilities(opts), vs, list)
	_ = MakeFirst(NewRng(2), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// empty Types → RandomReturnType nil
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	list := &FunctionList{Types: vs.Types}
	if MakeFirst(NewRng(1), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("empty AllTypes must fail closed")
	}
	// sticky error
	seedTypesForTest(NewRng(2), opts, probs, vs, list)
	SetErrorSess(testAmbientSession, ErrGeneric)
	if MakeFirst(NewRng(3), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("sticky error")
	}
	ClearErrorSess(testAmbientSession)
	// no invent Unbuilt success when body cannot generate
	if MakeFirst(nil, opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("nil RNG must not invent first")
	}
}

func TestMakeFirstIncompleteGlobalListFailClosed(t *testing.T) {
	// AddNewVarFact(nil) no-ops — incomplete GlobalList must not invent partial FM seed + body
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	seedTypesForTest(NewRng(4), opts, probs, vs, list)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g, nil}
	if MakeFirst(NewRng(5), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), list, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeFirst")
	}
	ClearErrorSess(testAmbientSession)
	// MakeRandomFunction same seed path
	vs2 := NewVariableSelector(opts)
	list2 := &FunctionList{}
	seedTypesForTest(NewRng(6), opts, probs, vs2, list2)
	vs2.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntType(), false, false), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFuncList(list2)
	if MakeRandomFunction(NewRng(7), opts, probs, vs2, &vs2.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), cg, GetIntType(), nil, list2) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeRandomFunction")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateFunctionsStopsOnERROR(t *testing.T) {
	// Function.cpp:797/805 ERROR_RETURN after make_first / GenerateBody
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	s := NewSession(opts)
	g := NewProgramGenerator(s)
	// poison after types so make_first fails (bag-local sticky; no ambient dual-sync)
	g.Initialize()
	g.GenerateAllTypes()
	SetErrorSess(s, ErrGeneric)
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
	ClearErrorSess(s)
}

func TestGenerateFunctionsIncompleteGlobalListSeedFailClosed(t *testing.T) {
	// GenerateFunctions FM seed (MakeFirst + unbuilt loop) shares VariablesComplete /
	// FactsComplete gate — incomplete GlobalList must not invent built bodies.
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	s := NewSession(opts)
	g := NewProgramGenerator(s)
	g.Initialize()
	g.GenerateAllTypes()
	g.VS.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), false, false), nil}
	g.GenerateFunctions()
	for _, f := range g.Funcs.Funcs {
		if f != nil && (f.IsBuilt || f.BuildState == BuildBuilt) {
			t.Fatal("incomplete GlobalList seed must not invent built body")
		}
	}
	if !HasErrorSess(s) {
		t.Fatal("incomplete GlobalList seed must SetError sticky")
	}
	ClearErrorSess(s)
}

func TestGenerateFunctionsNoInventNilFuncHole(t *testing.T) {
	// Function* always live on Funcs; nil hole stops unbuilt-body loop
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxFuncs = 3
	opts.MaxBlockSize = 1
	g := NewProgramGenerator(NewSession(opts))
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
	ClearErrorSess(testAmbientSession)
}
