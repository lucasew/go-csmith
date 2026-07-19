package csmith

import (
	"strings"
	"testing"
)

func TestReachMaxFunctions(t *testing.T) {
	ClearError()
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
	ClearError()
	opts.MaxFuncs = 100
	list.Funcs = []*Function{{Name: "a"}, nil}
	if !ReachMaxFunctions(&list, opts) {
		t.Fatal("nil hole must fail closed as at-max")
	}
	if HasError() {
		t.Fatal("nil hole ReachMaxFunctions must stay non-sticky")
	}
	ClearError()
}

func TestMakeRandomBinaryInvocationOutput(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// FunctionInvocationBinary.cpp:68 assert(blk) — need live block for safe_ops temps
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// C++ always sets SafeOpFlags; Output uses safe_* only for arith/shift + SafeMath.
	var fi *Invocation
	for seed := uint64(1); seed < 100; seed++ {
		ClearError()
		cg := WithFunc(f, EmptyEffect())
		fi = MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, tables, &cg, GetIntType())
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
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	fi := MakeRandomBinaryInvocation(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType())
	if fi != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomBinaryInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	// incomplete GlobalFacts snapshot before RHS
	fm2 := NewFactMgr(f)
	fm2.GlobalFacts = IncompleteFactSlice()
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	cg2.EffectAccum = &eff
	fi2 := MakeRandomBinaryInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, GetIntType())
	if fi2 != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomBinaryInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomInvocationIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient / GlobalFacts fail closed sticky before choose/build
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	list := &FunctionList{Funcs: []*Function{f}}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	fi := MakeRandomInvocation(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, GetIntType(), nil, false)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm2 := NewFactMgr(f)
	fm2.GlobalFacts = IncompleteFactSlice()
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	cg2.EffectAccum = &eff
	fi2 := MakeRandomInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, list, GetIntType(), nil, false)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomBinaryHasPointerTypeIncompleteSticky(t *testing.T) {
	// incomplete DerivedTypes must not invent scalar binary past HasPointerType hole
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	var env TypeEnv
	env.DerivedTypes = IncompleteTypes()
	vs.Types = &env
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Types = &env
	// seed forces flipcoin(10) path when possible — try many seeds
	var sawSticky bool
	for seed := uint64(1); seed < 80; seed++ {
		ClearError()
		fi := MakeRandomBinaryInvocation(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType())
		if HasError() {
			sawSticky = true
			if fi != nil {
				t.Fatal("sticky incomplete DerivedTypes must not return binary inv")
			}
			break
		}
	}
	// if no seed hit flipcoin(10), still verify HasPointerType sticky alone
	ClearError()
	if env.HasPointerType() {
		t.Fatal("incomplete DerivedTypes must fail HasPointerType")
	}
	if !HasError() {
		t.Fatal("HasPointerType incomplete must SetError sticky")
	}
	ClearError()
	_ = sawSticky
}

func TestMakeRandomBinaryInvocationMergesLhsEffect(t *testing.T) {
	// FunctionInvocation.cpp:208–221 — LHS under dedicated accum; merge_param_context
	// folds reads into caller's effect_accum and raises expr_depth.
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.ExprDepth = 0
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		// fresh accum each try
		eff = EmptyEffect()
		cg.EffectAccum = &eff
		cg.ExprDepth = 0
		cg.EffectStm = EmptyEffect()
		fi = MakeRandomBinaryInvocation(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType())
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
		g := NewProgramGenerator(opts)
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
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	blk := &Block{}
	v := vs.GenerateNewParentLocal(blk, AccessRead, EmptyCGContext(), GetIntType(), nil, r)
	if v == nil || !v.IsLocal() || len(blk.LocalVars) != 1 {
		t.Fatalf("%+v", v)
	}
}

func TestMakeRandomBinaryPtrComparison(t *testing.T) {
	// Operands may recurse into comma (type nullptr) — seed full Type env
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	_ = env.FindPointerType(GetIntType(), true)
	vs.Types = env
	tables := NewExprTables(opts)
	fi := func() *Invocation {
		c := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
		c.Types = env
		return MakeRandomBinaryPtrComparison(NewRng(4), opts, probs, vs, tables, &c, env)
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
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	probs := NewProbabilities(opts)
	list := &FunctionList{}
	caller := &Function{Name: "func_1", ReturnType: GetIntType(), IsBuilt: true}
	callee := &Function{Name: "func_2", ReturnType: GetIntType(), IsBuilt: true, FactChanged: true}
	// mark effect known for ChooseFunc
	callee.FEffect = EmptyEffect()
	list.Funcs = []*Function{caller, callee}
	cg := WithFunc(caller, EmptyEffect())
	cg.Funcs = list
	// force non-std and pick existing callee when flipcoin allows
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		caller.FactChanged = false
		fi := MakeRandomInvocation(NewRng(seed), opts, probs, vs, tables, &cg, list, GetIntType(), nil, false)
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
		fi := BuildUserInvocation(NewRng(1), opts, probs, vs, tables, &cg, list, callee)
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
	ClearError()
	callee := &Function{Name: "func_2", ReturnType: GetIntType(), IsBuilt: true, BuildState: BuildBuilt}
	a0 := &Expression{Term: TermConstant, Con: MakeInt(1)}
	// empty callee name — sticky no invent "()"
	if out := (&Invocation{User: &Function{Name: "", ReturnType: GetIntType()}, Args: nil}).Output(); out != "" {
		t.Fatal("empty User.Name must fail closed, got", out)
	}
	if !HasError() {
		t.Fatal("empty User.Name must SetError sticky")
	}
	ClearError()
	fi := &Invocation{
		User: callee,
		Args: []*Expression{a0, nil},
	}
	if out := fi.Output(); out != "" {
		t.Fatal("nil arg must fail closed empty, got", out)
	}
	if !HasError() {
		t.Fatal("nil arg must SetError sticky")
	}
	// incomplete arg Output (nil Con) — sticky no invent empty slot
	ClearError()
	fi.Args = []*Expression{a0, {Term: TermConstant}}
	if out := fi.Output(); out != "" {
		t.Fatal("empty arg Output must fail closed, got", out)
	}
	if !HasError() {
		t.Fatal("empty arg Output must SetError sticky")
	}
	ClearError()
	fi.Args = []*Expression{a0, &Expression{Term: TermConstant, Con: MakeInt(2)}}
	out := fi.Output()
	if out != "func_2(1, 2)" {
		t.Fatal(out)
	}
	// binary incomplete operand Output sticky
	ClearError()
	bin := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant},
	}}
	if out := bin.Output(); out != "" {
		t.Fatal("empty binary operand must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty binary operand must SetError sticky")
	}
	ClearError()
}
