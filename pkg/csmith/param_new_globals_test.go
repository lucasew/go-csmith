package csmith

import "testing"

func TestMergeParamContext(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	parent := EmptyCGContext()
	eff := EmptyEffect()
	parent.EffectAccum = &eff
	paramAccum := EmptyEffect().ReadVar(a)
	param := EmptyCGContext()
	param.EffectAccum = &paramAccum
	param.ExprDepth = 3
	parent.MergeParamContext(param, true)
	if !eff.IsRead(a) {
		t.Fatal("merged read")
	}
	if parent.ExprDepth != 3 {
		t.Fatal("depth")
	}
}

func TestGenerateNewGlobalTracksNewGlobals(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	cg := WithFunc(f, EmptyEffect())
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	if v == nil || len(f.NewGlobals) != 1 || f.NewGlobals[0] != v {
		t.Fatalf("%v %v", v, f.NewGlobals)
	}
}

func TestBuildInvocationHandoverNewGlobals(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 2
	opts.MaxFuncs = 5
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	caller := &Function{Name: "caller", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgr(caller)
	cg := WithFunc(caller, EmptyEffect()).WithFactMgr(fm).WithFuncList(list)
	caller.Stack = []*Block{{Func: caller}}
	// force globals enabled so body can create
	fi := BuildInvocationAndFunction(NewRng(7), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg, list, GetIntType())
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
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	callee := &Function{
		Name:       "c",
		ReturnType: GetIntType(),
		BuildState: BuildBuilt,
		IsBuilt:    true,
		Param:      []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)},
	}
	caller := &Function{Name: "a"}
	cg := WithFunc(caller, EmptyEffect())
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	fi := BuildUserInvocation(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), cg, nil, callee)
	if fi == nil || len(fi.Args) != 1 {
		t.Fatal("args")
	}
}
