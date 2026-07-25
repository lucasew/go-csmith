package csmith

import "testing"

func TestAddExternalEffectGlobalsOnly(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	e := EmptyEffect().WriteVar(g).WriteVar(l).ReadVar(g)
	ext := EmptyEffect().AddExternalEffect(e)
	if !ext.IsWritten(g) || ext.IsWritten(l) {
		t.Fatal("globals only")
	}
	if !ext.IsRead(g) {
		t.Fatal("read g")
	}
}

func TestExtendCallChain(t *testing.T) {
	f := &Function{Name: "f"}
	b1 := &Block{Func: f}
	b2 := &Block{Func: f, Parent: b1}
	f.Stack = []*Block{b1, b2}
	from := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	from.CallChain = []*Block{b1}
	var to CGContext
	to.ExtendCallChain(from)
	if len(to.CallChain) != 2 || to.CallChain[1] != b2 {
		// CurrentBlock is b2 (top of stack)
		if len(to.CallChain) < 1 {
			t.Fatal(to.CallChain)
		}
	}
}

// Function.cpp:635 / 677 — extend_call_chain(prev) uses CALLER get_current_block().
// Soft invent set bodyCG.CurrentFunc=callee before generateBodyCore so
// ExtendCallChain(prev) saw empty callee stack and omitted the caller frame.
func TestExtendCallChainFromCallerNotCallee(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	caller := &Function{Name: "caller", ReturnType: GetIntType()}
	callee := &Function{Name: "callee", ReturnType: GetIntType()}
	callerBlk := &Block{Func: caller, StmID: AllocStmID()}
	caller.Stack = []*Block{callerBlk}

	// Fair: prev is still the caller when ExtendCallChain runs (generateBodyCore order).
	prev := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	prev.CallChain = nil
	body := prev
	body.CurrentFunc = callee // only the body context switches
	body.ExtendCallChain(prev)
	if len(body.CallChain) != 1 || body.CallChain[0] != callerBlk {
		t.Fatalf("call_chain must include caller block, got %v", body.CallChain)
	}

	// Unfair pre-switch: prev.CurrentFunc already callee → empty stack → missing frame.
	prevBad := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	prevBad.CurrentFunc = callee
	prevBad.CallChain = nil
	var bodyBad CGContext
	bodyBad.CurrentFunc = callee
	bodyBad.ExtendCallChain(prevBad)
	if len(bodyBad.CallChain) != 0 {
		t.Fatalf("callee-as-prev must not invent a frame, got %v", bodyBad.CallChain)
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildInvocationAndFunction(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxFuncs = 5
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	list := &FunctionList{Types: &TypeEnv{Sess: testAmbientSession}}
	caller := &Function{Name: "caller", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm).WithFuncList(list)
	caller.Stack = []*Block{{Func: caller}}
	fi := BuildInvocationAndFunction(NewRng(4), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntType(), nil)
	if fi == nil || fi.Failed || fi.User == nil {
		t.Fatal("fail")
	}
	if !fi.User.IsEffectKnown() {
		t.Fatal("callee built")
	}
	if len(list.Funcs) < 2 {
		t.Fatal("not registered")
	}
}

func TestBuildUserInvocationMergesFEffect(t *testing.T) {
	// CGContext::add_external_effect merges into accum/stm only (not caller feffect).
	// Function.cpp:657 finalizes caller feffect from map_stm_effect[body].
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	callee.FEffect = EmptyEffect().WriteVar(g)
	caller := &Function{Name: "a", ReturnType: GetIntType()}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_ = BuildUserInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, nil, callee)
	if !eff.IsWritten(g) {
		t.Fatal("external write into accum")
	}
	if caller.FEffect.IsWritten(g) {
		t.Fatal("built-callee call must not invent mid-call into caller FEffect")
	}
}

func TestGenerateStillWorksWithEagerBodies(t *testing.T) {
	opts := Defaults()
	opts.Seed = 9
	opts.MaxFuncs = 3
	opts.MaxBlockSize = 2
	out, err := Generate(opts)
	if err != nil || out == "" {
		t.Fatal(err)
	}
}
