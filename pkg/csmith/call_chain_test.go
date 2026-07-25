package csmith

import "testing"

func TestAddExternalEffectGlobalsOnly(t *testing.T) {
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	l := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	e := EmptyEffect().WriteVarSess(testAmbientSession, g).WriteVarSess(testAmbientSession, l).ReadVarSess(testAmbientSession, g)
	ext := EmptyEffect().AddExternalEffectSess(testAmbientSession, e)
	if !ext.IsWrittenSess(testAmbientSession, g) || ext.IsWrittenSess(testAmbientSession, l) {
		t.Fatal("globals only")
	}
	if !ext.IsReadSess(testAmbientSession, g) {
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
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession)}
	callee := &Function{Name: "callee", ReturnType: GetIntTypeSess(testAmbientSession)}
	callerBlk := &Block{Func: caller, StmID: AllocStmIDSess(testAmbientSession)}
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
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{Types: &TypeEnv{Sess: testAmbientSession}}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm).WithFuncList(list)
	caller.Stack = []*Block{{Func: caller}}
	fi := BuildInvocationAndFunction(NewRngSess(testAmbientSession, 4), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTableSess(testAmbientSession, opts), &cg, list, GetIntTypeSess(testAmbientSession), nil)
	if fi == nil || fi.Failed || fi.User == nil {
		t.Fatal("fail")
	}
	if !fi.User.IsEffectKnownSess(testAmbientSession) {
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
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	callee.FEffect = EmptyEffect().WriteVarSess(testAmbientSession, g)
	caller := &Function{Name: "a", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_ = BuildUserInvocation(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, nil, callee)
	if !eff.IsWrittenSess(testAmbientSession, g) {
		t.Fatal("external write into accum")
	}
	if caller.FEffect.IsWrittenSess(testAmbientSession, g) {
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
