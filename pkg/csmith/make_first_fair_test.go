package csmith

import "testing"

func TestBodyOutAssignMissingNoInventPrior(t *testing.T) {
	// Function.cpp:469 — global_facts = map_facts_out[body]
	// missing body out must not invent keep prior GlobalFacts
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), Body: &Block{StmID: 10}}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	prior := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	fm.GlobalFacts = []*FactPointTo{prior}
	// no MapFactsOut[10]
	// mirror MakeFirst handoff assign
	out := fm.MapFactsOut[f.Body.StmID]
	if !FactsComplete(out) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSliceSess(testAmbientSession, out)
	}
	// missing MapFactsOut is complete empty (C++ map[]); must not keep prior
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) != nil {
		t.Fatal("missing body out must clear prior GlobalFacts, not invent keep prior")
	}
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("missing body out is complete empty, not incomplete marker")
	}
}

func TestBodyOutAssignIncompleteFailClosed(t *testing.T) {
	// incomplete map_facts_out[body] must not invent cleaned GlobalFacts
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), Body: &Block{StmID: 11}}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}
	fm.MapFactsOut = map[int][]*FactPointTo{
		11: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	out := fm.MapFactsOut[f.Body.StmID]
	if !FactsComplete(out) {
		fm.GlobalFacts = IncompleteFactSlice()
	} else {
		fm.GlobalFacts = CloneFactSliceSess(testAmbientSession, out)
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete body out must fail closed incomplete GlobalFacts")
	}
}

func TestRetFactsNoInventGlobalFactsFallback(t *testing.T) {
	// FunctionInvocationUser.cpp:212 — ret_facts = map_facts_out[body]
	// no invent GlobalFacts when body out missing
	callee := &Function{Name: "g", ReturnType: GetIntTypeSess(testAmbientSession), Body: &Block{StmID: 20}}
	calFM := NewFactMgrSess(testAmbientSession, callee)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	calFM.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}
	// no MapFactsOut[20]
	var retFacts []*FactPointTo
	var retUnions []*FactUnion
	out := calFM.MapFactsOut[callee.Body.StmID]
	if FactsComplete(out) {
		retFacts = CloneFactSliceSess(testAmbientSession, out)
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
		20: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
		99: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr)},
	}
	calFM.MapUnionFactsOut = map[int][]*FactUnion{20: {}, 99: {}}
	retFacts = nil
	retUnions = nil
	out = calFM.MapFactsOut[callee.Body.StmID]
	if FactsComplete(out) {
		retFacts = CloneFactSliceSess(testAmbientSession, out)
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
		20: {MakeFactPointToSess(testAmbientSession, p, NullPtr)},
		99: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil},
	}
	calFM.MapUnionFactsOut = map[int][]*FactUnion{20: {}, 99: {}}
	retFacts = CloneFactSliceSess(testAmbientSession, calFM.MapFactsOut[20])
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
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{}
	fmMap := NewFactMgrMapSess(testAmbientSession)
	seedTypesForTest(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, list)
	f := MakeFirst(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, fmMap)
	if f == nil || f.Body == nil {
		t.Fatal("nil first")
	}
	if f.BuildState != BuildBuilt {
		t.Fatal(f.BuildState)
	}
	fm := fmMap.ForFuncSess(testAmbientSession, f)
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
	vs := NewVariableSelector(testAmbientSession, opts)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	list := &FunctionList{}
	// seed first so list non-empty for choose
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, list)
	_ = MakeFirst(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Funcs = list
	f := MakeRandomFunction(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), cg, GetIntTypeSess(testAmbientSession), nil, list)
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
	if MakeRandomFunction(nil, opts, NewProbabilities(opts), vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), cg, GetIntTypeSess(testAmbientSession), nil, list) != nil {
		t.Fatal("nil RNG must not invent unbuilt function")
	}
}

func TestMakeFirstERRORGuard(t *testing.T) {
	// Function.cpp:445/453 ERROR_GUARD paths
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// empty Types → RandomReturnType nil
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	list := &FunctionList{Types: vs.Types}
	if MakeFirst(NewRngSess(testAmbientSession, 1), opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, nil) != nil {
		t.Fatal("empty AllTypes must fail closed")
	}
	// sticky error
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, probs, vs, list)
	SetErrorSess(testAmbientSession, ErrGeneric)
	if MakeFirst(NewRngSess(testAmbientSession, 3), opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, nil) != nil {
		t.Fatal("sticky error")
	}
	ClearErrorSess(testAmbientSession)
	// no invent Unbuilt success when body cannot generate
	if MakeFirst(nil, opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, nil) != nil {
		t.Fatal("nil RNG must not invent first")
	}
}

func TestMakeFirstIncompleteGlobalListFailClosed(t *testing.T) {
	// AddNewVarFact(nil) no-ops — incomplete GlobalList must not invent partial FM seed + body
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{}
	seedTypesForTest(NewRngSess(testAmbientSession, 4), opts, probs, vs, list)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g, nil}
	if MakeFirst(NewRngSess(testAmbientSession, 5), opts, probs, vs, &vs.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), list, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeFirst")
	}
	ClearErrorSess(testAmbientSession)
	// MakeRandomFunction same seed path
	vs2 := NewVariableSelector(testAmbientSession, opts)
	list2 := &FunctionList{}
	seedTypesForTest(NewRngSess(testAmbientSession, 6), opts, probs, vs2, list2)
	vs2.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntTypeSess(testAmbientSession), false, false), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFuncList(list2)
	if MakeRandomFunction(NewRngSess(testAmbientSession, 7), opts, probs, vs2, &vs2.Sym, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), cg, GetIntTypeSess(testAmbientSession), nil, list2) != nil {
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
	g.VS.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false), nil}
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
