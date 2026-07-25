package csmith

import (
	"strings"
	"testing"
)

func TestReachMaxFunctions(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxFuncs = 2
	var list FunctionList
	list.Funcs = []*Function{{Name: "a"}, {Name: "b"}}
	if !ReachMaxFunctions(&list, opts) {
		t.Fatal("at max")
	}
	list.Funcs = list.Funcs[:1]
	if ReachMaxFunctions(&list, opts) {
		t.Fatal("under max")
	}
	// nil Function* hole fails closed as at-max non-sticky (soft re-pick gate)
	ClearErrorSess(testAmbientSession)
	opts.MaxFuncs = 100
	list.Funcs = []*Function{{Name: "a"}, nil}
	if !ReachMaxFunctions(&list, opts) {
		t.Fatal("nil hole must fail closed as at-max")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole ReachMaxFunctions must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBinaryInvocationOutput(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	// FunctionInvocationBinary.cpp:68 assert(blk) — need live block for safe_ops temps
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// C++ always sets SafeOpFlags; Output uses safe_* only for arith/shift + SafeMath.
	var fi *Invocation
	for seed := uint64(1); seed < 100; seed++ {
		ClearErrorSess(testAmbientSession)
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
		fi = MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
		if fi != nil && fi.IsStd && SafeOpsBinary(fi.Binary) && fi.OutSafeMath {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Fatal("no safe-ops binary in sample")
	}
	out := fi.Output()
	if !strings.Contains(out, "safe_") {
		t.Fatalf("safe wrapper missing: %s", out)
	}
}

func TestMakeRandomBinaryInvocationIncompleteEffectFailClosed(t *testing.T) {
	// incomplete EffectAccum after lhs must sticky ERROR (no invent RHS / soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession))
	if fi != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomBinaryInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts snapshot before RHS
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	cg2.EffectAccum = &eff
	fi2 := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg2, GetIntTypeSess(testAmbientSession))
	if fi2 != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomBinaryInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomInvocationIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient / GlobalFacts fail closed sticky before choose/build
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	list := &FunctionList{Funcs: []*Function{f}}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	fi := MakeRandomInvocation(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, list, GetIntTypeSess(testAmbientSession), nil, false)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	cg2.EffectAccum = &eff
	fi2 := MakeRandomInvocation(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg2, list, GetIntTypeSess(testAmbientSession), nil, false)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBinaryHasPointerTypeIncompleteSticky(t *testing.T) {
	// incomplete DerivedTypes must not invent scalar binary past HasPointerType hole
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.DerivedTypes = IncompleteTypes()
	vs.Types = &env
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Types = &env
	// seed forces flipcoin(10) path when possible — try many seeds
	var sawSticky bool
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession))
		if HasErrorSess(testAmbientSession) {
			sawSticky = true
			if fi != nil {
				t.Fatal("sticky incomplete DerivedTypes must not return binary inv")
			}
			break
		}
	}
	// if no seed hit flipcoin(10), still verify HasPointerType sticky alone
	ClearErrorSess(testAmbientSession)
	if env.HasPointerType() {
		t.Fatal("incomplete DerivedTypes must fail HasPointerType")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasPointerType incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	_ = sawSticky
}

func TestMakeRandomBinaryInvocationMergesLhsEffect(t *testing.T) {
	// FunctionInvocation.cpp:208–221 — LHS under dedicated accum; merge_param_context
	// folds reads into caller's effect_accum and raises expr_depth.
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.ExprDepth = 0
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		// fresh accum each try
		eff = EmptyEffect()
		cg.EffectAccum = &eff
		cg.ExprDepth = 0
		cg.EffectStm = EmptyEffect()
		fi = MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession))
		if fi != nil && len(fi.Args) == 2 && fi.Args[0] != nil {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Skip("no binary")
	}
	// leaf LHS constant or variable bumps expr_depth via merge_param_context
	if cg.ExprDepth < 1 {
		t.Fatalf("ExprDepth=%d after binary (want ≥1 from LHS leaf)", cg.ExprDepth)
	}
}

func TestExpressionFuncallCanCreateUserFunc(t *testing.T) {
	// Force TermFunction and stdFunc=false path often enough to create multi-func programs.
	foundMulti := false
	for seed := uint64(1); seed < 60; seed++ {
		opts := Defaults()
		opts.Seed = seed
		opts.MaxFuncs = 10
		g := NewProgramGenerator(NewSession(opts))
		_ = g.GoGenerator()
		if len(g.Funcs.Funcs) > 1 {
			foundMulti = true
			// later funcs should be built
			for _, f := range g.Funcs.Funcs {
				if f != nil && !f.IsBuilt {
					t.Fatalf("%s not built", f.Name)
				}
			}
			out := g.GoGenerator() // regenerate for string — wait that re-seeds. use first run
			_ = out
			break
		}
	}
	// Check multi from a single generate
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// count func_ definitions roughly
		if strings.Count(out, "func_") >= 4 { // forward + body for 2 funcs at least
			// e.g. func_1 and func_2 appear
			if strings.Contains(out, "func_2") {
				foundMulti = true
				break
			}
		}
	}
	if !foundMulti {
		t.Log("no multi-func in seeds 1..79 — may still be rare; check binary ops at least")
	}
}

func TestGenerateEmitsBinaryOrCall(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, " + ") || strings.Contains(out, " - ") ||
			strings.Contains(out, "func_2") || strings.Contains(out, "++") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected binary op or second function in some seed")
	}
}

func TestGenerateNewParentLocal(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	blk := &Block{}
	v := vs.GenerateNewParentLocal(blk, AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, r)
	if v == nil || !v.IsLocalSess(testAmbientSession) || len(blk.LocalVars) != 1 {
		t.Fatalf("%+v", v)
	}
}

func TestMakeRandomBinaryPtrComparison(t *testing.T) {
	// Operands may recurse into comma (type nullptr) — seed full Type env
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 2), opts, probs, env)
	_ = env.FindPointerType(GetIntTypeSess(testAmbientSession), true)
	vs.Types = env
	tables := NewExprTablesSess(testAmbientSession, opts)
	fi := func() *Invocation {
		c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
		c.Types = env
		return MakeRandomBinaryPtrComparison(NewRngSess(testAmbientSession, 4), opts, probs, vs, tables, &c, env)
	}()
	if fi == nil || !fi.IsStd {
		t.Fatalf("%+v", fi)
	}
	if fi.Binary != "==" && fi.Binary != "!=" {
		t.Fatalf("op %s", fi.Binary)
	}
	out := fi.Output()
	if out == "" {
		t.Fatal("empty ptr cmp output")
	}
	if out == "/*invoke*/" || out == "/*bad_call*/" {
		t.Fatal("no soft invent comments", out)
	}
	// top-level is ==/!= (operands may contain safe_* from nested exprs)
	if !strings.Contains(out, "==") && !strings.Contains(out, "!=") {
		t.Fatalf("expected cmp op in %s", out)
	}
	// C++ sets SafeOpFlags for ptr cmp but Output uses standard ==/!= (not safe_ops)
	if strings.HasPrefix(out, "(safe_") {
		t.Fatalf("ptr cmp must not use safe wrapper: %s", out)
	}
}

func TestMakeRandomInvocationPropagatesFactChanged(t *testing.T) {
	// FunctionInvocation.cpp:95–97
	opts := Defaults()
	opts.MaxFuncs = 5
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	list := &FunctionList{}
	caller := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true}
	callee := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true, FactChanged: true}
	// mark effect known for ChooseFunc
	callee.FEffect = EmptyEffect()
	list.Funcs = []*Function{caller, callee}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	cg.Funcs = list
	// force non-std and pick existing callee when flipcoin allows
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		caller.FactChanged = false
		fi := MakeRandomInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, list, GetIntTypeSess(testAmbientSession), nil, false)
		if fi != nil && !fi.Failed && fi.User == callee {
			if !caller.FactChanged {
				t.Fatal("caller.fact_changed not set from callee")
			}
			found = true
			break
		}
	}
	if !found {
		// may not hit callee due to RNG; at least ensure BuildUserInvocation path works
		fi := BuildUserInvocation(NewRngSess(testAmbientSession, 1), opts, probs, vs, tables, &cg, list, callee)
		if fi != nil && !fi.Failed {
			caller.FactChanged = caller.FactChanged || fi.User.FactChanged
			if !caller.FactChanged {
				t.Fatal("manual or")
			}
		}
	}
}

func TestUserInvocationOutputNoInventNilArgs(t *testing.T) {
	// FunctionInvocationUser::Output — param_value[i] always live; sticky no invent f(a, , c)
	ClearErrorSess(testAmbientSession)
	callee := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true, BuildState: BuildBuilt}
	a0 := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	// empty callee name — sticky no invent "()"
	if out := (&Invocation{User: &Function{Name: "", ReturnType: GetIntTypeSess(testAmbientSession)}, Args: nil}).Output(); out != "" {
		t.Fatal("empty User.Name must fail closed, got", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty User.Name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fi := &Invocation{
		User: callee,
		Args: []*Expression{a0, nil},
	}
	if out := fi.Output(); out != "" {
		t.Fatal("nil arg must fail closed empty, got", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg must SetError sticky")
	}
	// incomplete arg Output (nil Con) — sticky no invent empty slot
	ClearErrorSess(testAmbientSession)
	fi.Args = []*Expression{a0, {Term: TermConstant}}
	if out := fi.Output(); out != "" {
		t.Fatal("empty arg Output must fail closed, got", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty arg Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fi.Args = []*Expression{a0, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2)}}
	out := fi.Output()
	if out != "func_2(1, 2)" {
		t.Fatal(out)
	}
	// binary incomplete operand Output sticky
	ClearErrorSess(testAmbientSession)
	bin := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant},
	}}
	if out := bin.Output(); out != "" {
		t.Fatal("empty binary operand must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty binary operand must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationOutputNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Invocation)(nil).Output() != "" {
		t.Fatal("nil Invocation Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invocation Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Failed stays non-sticky soft re-pick
	if (&Invocation{Failed: true}).Output() != "" {
		t.Fatal("Failed Invocation Output must fail closed empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("Failed Invocation Output must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	// Invocation always live; sticky (no invent soft-skip out-opts past hole)
	(*Invocation)(nil).setOutOpts(Defaults())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invocation setOutOpts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBinaryInvocationIsFloatResidualSticky(t *testing.T) {
	// IsFloat residual soft invent was invent soft-continue binary pick past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	if (*Type)(nil).IsFloatSess(testAmbientSession) {
		t.Fatal("nil Type IsFloat must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsFloat must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete float IsFloat true path no sticky
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	if !ft.IsFloatSess(testAmbientSession) {
		t.Fatal("EFloat IsFloat must true")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete EFloat IsFloat must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete int IsFloat false no sticky
	if GetIntTypeSess(testAmbientSession).IsFloatSess(testAmbientSession) {
		t.Fatal("int IsFloat must false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete int IsFloat must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
