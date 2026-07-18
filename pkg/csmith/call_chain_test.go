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
	from := WithFunc(f, EmptyEffect())
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

func TestBuildInvocationAndFunction(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxFuncs = 5
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	list := &FunctionList{Types: &TypeEnv{}}
	caller := &Function{Name: "caller", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgr(caller)
	cg := WithFunc(caller, EmptyEffect()).WithFactMgr(fm).WithFuncList(list)
	caller.Stack = []*Block{{Func: caller}}
	fi := BuildInvocationAndFunction(NewRng(4), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), cg, list, GetIntType())
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
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	callee.FEffect = EmptyEffect().WriteVar(g)
	caller := &Function{Name: "a", ReturnType: GetIntType()}
	cg := WithFunc(caller, EmptyEffect())
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_ = BuildUserInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), cg, nil, callee)
	if !eff.IsWritten(g) {
		t.Fatal("external write")
	}
	if !caller.FEffect.IsWritten(g) {
		t.Fatal("feffect")
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
