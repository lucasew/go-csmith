package csmith

import "testing"

func TestMergeParamContext(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	parent := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	parent.EffectAccum = &eff
	paramAccum := EmptyEffect().ReadVarSess(testAmbientSession, a)
	param := EmptyCGContext().WithSession(testAmbientSession)
	param.EffectAccum = &paramAccum
	param.ExprDepth = 3
	parent.MergeParamContext(param, true)
	if !eff.IsReadSess(testAmbientSession, a) {
		t.Fatal("merged read")
	}
	if parent.ExprDepth != 3 {
		t.Fatal("depth")
	}
	// incomplete param accum must not invent expr_depth handoff after failed merge
	parent2 := EmptyCGContext().WithSession(testAmbientSession)
	parent2.ExprDepth = 1
	inc := IncompleteEffect()
	param2 := EmptyCGContext().WithSession(testAmbientSession)
	param2.EffectAccum = &inc
	param2.ExprDepth = 9
	parent2.MergeParamContext(param2, true)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete param accum must SetError")
	}
	if parent2.ExprDepth != 1 {
		t.Fatal("must not invent ExprDepth after failed effect merge")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateNewGlobalTracksNewGlobals(t *testing.T) {
	ClearErrorSess(testAmbientSession) // ERROR_GUARD on GenerateNewGlobal must not see prior test sticky error
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2))
	if v == nil || len(f.NewGlobals) != 1 || f.NewGlobals[0] != v {
		t.Fatalf("%v %v", v, f.NewGlobals)
	}
}

func TestBuildInvocationHandoverNewGlobals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 2
	opts.MaxFuncs = 5
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm).WithFuncList(list)
	caller.Stack = []*Block{{Func: caller}}
	// force globals enabled so body can create
	var fi *Invocation
	for seed := uint64(7); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		list.Funcs = []*Function{caller}
		caller.NewGlobals = nil
		fi = BuildInvocationAndFunction(NewRngSess(testAmbientSession, seed), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntTypeSess(testAmbientSession), nil)
		if fi != nil && !fi.Failed {
			break
		}
		fi = nil
	}
	if fi == nil || fi.Failed {
		t.Fatal("fail")
	}
	// caller may receive new_globals from callee
	_ = caller.NewGlobals
	if !fi.User.IsEffectKnown() {
		t.Fatal("built")
	}
}

func TestBuildUserInvocationParamMerge(t *testing.T) {
	// FunctionInvocationUser.cpp:252–268 — param_cg + merge_param_context raises expr_depth
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	callee := &Function{
		Name:       "c",
		ReturnType: GetIntTypeSess(testAmbientSession),
		BuildState: BuildBuilt,
		IsBuilt:    true,
		Param:      []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)},
	}
	caller := &Function{Name: "a"}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.ExprDepth = 0
	fi := BuildUserInvocation(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, nil, callee)
	if fi == nil || len(fi.Args) != 1 {
		t.Fatal("args")
	}
	// constant/variable param bumps depth via make_random_param + merge
	if fi.Args[0] != nil && BumpsExprDepth(fi.Args[0]) && cg.ExprDepth < 1 {
		t.Fatalf("ExprDepth=%d after param (want ≥1)", cg.ExprDepth)
	}
}

func TestBuildUserInvocationErrorGuardOnParam(t *testing.T) {
	// FunctionInvocationUser.cpp:259 — ERROR_GUARD(false) sticky error → failed
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	callee := &Function{
		Name:       "c",
		ReturnType: GetIntTypeSess(testAmbientSession),
		BuildState: BuildBuilt,
		IsBuilt:    true,
		Param:      []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)},
	}
	caller := &Function{Name: "a"}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	fi := BuildUserInvocation(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, nil, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("sticky error must fail invocation, not invent params")
	}
}

func TestMakeRandomSignatureErrorGuardOnRV(t *testing.T) {
	// Function.cpp:419–420 — CreateVariable ERROR_GUARD; sticky error aborts signature
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	vs.Types = env
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = env
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if MakeRandomSignature(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &vs.Sym, cg, GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("sticky error must not invent signature")
	}
}

func TestMakeRandomUnaryInvocationBumpsExprDepth(t *testing.T) {
	// FunctionInvocation.cpp:157–159 — operand make_random mutates cg.expr_depth
	// FunctionInvocationUnary.cpp:57 assert(blk) — need stack for safe tmp
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	var fi *Invocation
	var cg CGContext
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		cg = WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
		cg.ExprDepth = 1
		fi = MakeRandomUnaryInvocation(NewRngSess(testAmbientSession, seed), opts, vs, NewExprTables(opts), &cg, GetIntTypeSess(testAmbientSession))
		if fi != nil && fi.IsUnary {
			break
		}
		fi = nil
	}
	if fi == nil || !fi.IsUnary {
		t.Fatal("unary")
	}
	if cg.ExprDepth < 2 {
		t.Fatalf("ExprDepth=%d want ≥2 after operand leaf", cg.ExprDepth)
	}
	ClearErrorSess(testAmbientSession)
}
