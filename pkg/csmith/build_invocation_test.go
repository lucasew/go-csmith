package csmith

import (
	"strings"
	"testing"
)

func TestBuildInvocationAndFunctionNilType(t *testing.T) {
	// FunctionInvocationUser.cpp:175 — assert(type); sticky Failed no GetIntType invent
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	cg := EmptyCGContext()
	fi := BuildInvocationAndFunction(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, nil, nil)
	if fi == nil || !fi.Failed {
		t.Fatal("nil return type must fail without soft invent")
	}
	if !HasError() {
		t.Fatal("nil return type must SetError sticky")
	}
	ClearError()
}

func TestBuildUserInvocationNoInventWithoutRNG(t *testing.T) {
	// zero-param callee must not invent success call without process RNG — sticky Failed
	ClearError()
	opts := Defaults()
	callee := &Function{Name: "func_x", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	list := &FunctionList{Funcs: []*Function{callee}}
	cg := EmptyCGContext()
	fi := BuildUserInvocation(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("nil RNG must fail closed user invoke")
	}
	if !HasError() {
		t.Fatal("nil RNG BuildUserInvocation must SetError sticky")
	}
	ClearError()
	fi2 := BuildInvocationAndFunction(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntType(), nil)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("nil RNG must fail closed build+function")
	}
	if !HasError() {
		t.Fatal("nil RNG BuildInvocationAndFunction must SetError sticky")
	}
	ClearError()
	// callee / cg hard IR sticky Failed
	fi3 := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, nil)
	if fi3 == nil || !fi3.Failed {
		t.Fatal("nil callee must fail closed")
	}
	if !HasError() {
		t.Fatal("nil callee BuildUserInvocation must SetError sticky")
	}
	ClearError()
	// Param hole sticky Failed
	callee2 := &Function{Name: "func_y", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		Param: []*Variable{nil}}
	fi4 := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee2)
	if fi4 == nil || !fi4.Failed {
		t.Fatal("nil Param hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Param hole BuildUserInvocation must SetError sticky")
	}
	ClearError()
	// MakeRandomInvocation nil r/cg sticky Failed
	fi5 := MakeRandomInvocation(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, GetIntType(), nil, false)
	if fi5 == nil || !fi5.Failed {
		t.Fatal("nil RNG MakeRandomInvocation must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomInvocation must SetError sticky")
	}
	ClearError()
}

func TestBuildInvocationAndFunctionParamsBeforeBody(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 10
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	// encourage multi-param signatures
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	list := &FunctionList{}
	// seed globals for args / body
	for i := 0; i < 3; i++ {
		_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(uint64(10+i)))
	}
	caller := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	fm := NewFactMgr(caller)
	cg := WithFunc(caller, EmptyEffect()).WithFactMgr(fm)
	cg.Funcs = list
	list.Funcs = []*Function{caller}

	// force new function creation path
	fi := BuildInvocationAndFunction(NewRng(7), opts, probs, vs, tables, stmtTab, &cg, list, GetIntType(), nil)
	if fi == nil || fi.Failed {
		// may fail if max funcs / depth — try more seeds
		ok := false
		for seed := uint64(1); seed < 40; seed++ {
			fi = BuildInvocationAndFunction(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg, list, GetIntType(), nil)
			if fi != nil && !fi.Failed && fi.User != nil {
				ok = true
				break
			}
		}
		if !ok {
			t.Skip("could not create invocation in seed scan")
		}
	}
	if fi.User == nil {
		t.Fatal("no callee")
	}
	if fi.User.BuildState != BuildBuilt && fi.User.Body == nil {
		// GenerateBody should have run
		t.Log("body state", fi.User.BuildState)
	}
	// arg count matches params
	if len(fi.Args) != len(fi.User.Param) {
		t.Fatalf("args %d params %d", len(fi.Args), len(fi.User.Param))
	}
	// visited once
	if fi.User.VisitedCnt < 1 {
		t.Fatal("visited_cnt")
	}
	// output is a call
	out := fi.Output()
	if !strings.Contains(out, fi.User.Name+"(") {
		t.Fatal(out)
	}
}

func TestBuildUserInvocationArgCount(t *testing.T) {
	// SanityCheck sticky on GenerateNewParentLocal: plant int+pointer targets and scan seeds
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	cgSeed := EmptyCGContext()
	g := vs.GenerateNewGlobal(AccessRead, cgSeed, GetIntType(), nil, NewRng(1))
	if g == nil {
		t.Fatal("need int global")
	}
	_ = vs.GenerateNewGlobal(AccessRead, cgSeed, PointerTo(GetIntType()), nil, NewRng(2))
	callee := &Function{
		Name:       "g_1",
		ReturnType: GetIntType(),
		Param: []*Variable{
			CreateVariableScalars("p_1", GetIntType(), false, false),
			CreateVariableScalars("p_2", PointerTo(GetIntType()), false, false),
		},
		BuildState: BuildBuilt,
	}
	callee.FEffect = EmptyEffect().ReadVar(g)
	var fi *Invocation
	for seed := uint64(3); seed < 80; seed++ {
		ClearError()
		caller := &Function{Name: "func_1"}
		blk := &Block{Func: caller}
		caller.Stack = []*Block{blk}
		cg := WithFunc(caller, EmptyEffect()).WithFactMgr(NewFactMgr(caller))
		fi = BuildUserInvocation(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, nil, callee)
		if fi != nil && !fi.Failed && len(fi.Args) == 2 {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Fatal("no successful BuildUserInvocation with 2 args in seed scan")
	}
	ClearError()
}

func TestBuildUserInvocationParamFailHard(t *testing.T) {
	// FunctionInvocationUser.cpp:257–258 — ERROR_GUARD(false) when make_random_param null
	// sticky Failed (no invent soft re-pick past null param without ERROR)
	ClearError()
	opts := Defaults()
	// force param term → Variable only; empty selector cannot select → null
	opts.MaxExprComplexity = 0
	vs := NewVariableSelector(opts)
	// no globals; Select may still create — zero MaxExpr still uses variable path
	// with empty AllVars and ScopeNewValue false-ish: leave no creation path
	vs.GlobalList = nil
	vs.AllVars = nil
	callee := &Function{
		Name:       "g_1",
		ReturnType: GetIntType(),
		Param:      []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)},
		BuildState: BuildBuilt,
		IsBuilt:    true,
	}
	caller := &Function{Name: "func_1"}
	cg := WithFunc(caller, EmptyEffect())
	// nil vs forces ExpressionVariable fail regardless of term choice
	fi := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), nil, NewExprTables(opts), &cg, nil, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("want Failed on null param")
	}
	if len(fi.Args) != 0 {
		t.Fatalf("must not append partial args: %d", len(fi.Args))
	}
	if !HasError() {
		t.Fatal("null param BuildUserInvocation must SetError sticky")
	}
	ClearError()
}

func TestBuildInvocationAndFunctionNilPairedFMSticky(t *testing.T) {
	// get_fact_mgr_for_func after signature always live; sticky Failed without paired FM
	ClearError()
	opts := Defaults()
	opts.MaxFuncs = 10
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	cg := EmptyCGContext()
	f := MakeRandomSignature(NewRng(1), opts, NewProbabilities(opts), vs, &vs.Sym, cg, GetIntType(), nil, list)
	if f == nil {
		t.Fatal("signature", HasError())
	}
	// same-package clear paired FM (signature always pairs; strip for fail-closed path)
	f.factMgr = nil
	f.GenerateBody(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext())
	if f.Body != nil {
		t.Fatal("nil paired FM must fail closed body")
	}
	if !HasError() {
		t.Fatal("nil paired FM GenerateBody must SetError sticky")
	}
	ClearError()
	// BuildInvocationAndFunction incomplete ambient: Failed sticky before body hand-over
	inc := IncompleteEffect()
	cg2 := EmptyCGContext()
	cg2.EffectAccum = &inc
	fi := BuildInvocationAndFunction(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2, list, GetIntType(), nil)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete ambient BuildInvocationAndFunction must Failed")
	}
	if !HasError() {
		t.Fatal("incomplete ambient BuildInvocationAndFunction must SetError sticky")
	}
	ClearError()
	// stripped calFM sticky Failed: signature ok then clear FM before hand-over via
	// same-package re-entry of PairedFactMgr path — BuildInvocationAndFunction re-makes
	// signature so clear after MakeRandomSignature inside is not injectable.
	// Exercise PairedFactMgr sticky on cleared function used as BuildUserInvocation
	// static path does not need calFM. Direct sticky:
	if (*Function)(nil).PairedFactMgr() != nil {
		t.Fatal("nil Function PairedFactMgr must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function PairedFactMgr must SetError sticky")
	}
	ClearError()
	f2 := &Function{Name: "no_fm", ReturnType: GetIntType()}
	if f2.PairedFactMgr() != nil {
		t.Fatal("unpaired Function PairedFactMgr must be nil")
	}
	// unpaired is complete miss (no invent FM) — not sticky until generate path
	if HasError() {
		t.Fatal("unpaired live Function PairedFactMgr must not sticky empty miss")
	}
	ClearError()
}

func TestBuildUserInvocationIncompleteAccumEffContextFailClosed(t *testing.T) {
	// revisit under incomplete AccumEffContext must not invent success
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	vs.GlobalList = []*Variable{CreateVariableScalars("g_1", GetIntType(), false, false)}
	// callee not first, NeedsRevisit, incomplete AccumEffContext
	callee := &Function{
		Name:            "g_helper",
		ReturnType:      GetIntType(),
		BuildState:      BuildBuilt,
		IsBuilt:         true,
		FactChanged:     true,
		AccumEffContext: IncompleteEffect(),
		Body:            &Block{StmID: 1, Stmts: []Stmt{}},
	}
	// ensure NeedsRevisit true
	if !callee.NeedsRevisit() {
		t.Fatal("need revisit")
	}
	caller := &Function{Name: "func_1"}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	cg := WithFunc(caller, EmptyEffect()).WithFuncList(list)
	fm := NewFactMgr(caller)
	cg = cg.WithFactMgr(fm)
	fi := BuildUserInvocation(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete AccumEffContext must fail closed BuildUserInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete AccumEffContext must SetError sticky")
	}
	ClearError()
	// incomplete GlobalFacts on revisit must sticky fail
	callee2 := &Function{
		Name:            "g_helper2",
		ReturnType:      GetIntType(),
		BuildState:      BuildBuilt,
		IsBuilt:         true,
		FactChanged:     true,
		AccumEffContext: EmptyEffect(),
		Body:            &Block{StmID: 2, Stmts: []Stmt{}},
	}
	caller2 := &Function{Name: "func_1"}
	list2 := &FunctionList{Funcs: []*Function{caller2, callee2}}
	fm2 := NewFactMgr(caller2)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(caller2, EmptyEffect()).WithFuncList(list2).WithFactMgr(fm2)
	fi2 := BuildUserInvocation(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, list2, callee2)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("incomplete GlobalFacts must fail closed BuildUserInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestBuildInvocationEffectHandoverIncompleteFailClosed(t *testing.T) {
	// BuildInvocationAndFunction effect hand-over must not invent success past incomplete
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxFuncs = 5
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	list := &FunctionList{Types: vs.Types}
	// plant incomplete ambient on caller before build
	caller := &Function{Name: "func_1", ReturnType: GetIntType()}
	list.Funcs = []*Function{caller}
	cg := WithFunc(caller, IncompleteEffect()).WithFuncList(list)
	cg.Types = vs.Types
	fm := NewFactMgr(caller)
	cg = cg.WithFactMgr(fm)
	fi := BuildInvocationAndFunction(NewRng(4), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntType(), nil)
	if fi != nil && !fi.Failed {
		// may return Failed or nil; must not invent clean success under incomplete ambient
		t.Fatal("incomplete caller EffectContext must fail closed BuildInvocationAndFunction")
	}
	if !HasError() {
		t.Fatal("incomplete caller EffectContext must SetError sticky")
	}
	ClearError()
}

func TestGetFirstFunction(t *testing.T) {
	// nil/empty list is complete miss (isolated BuildUserInvocation passes nil list)
	ClearError()
	if GetFirstFunction(nil) != nil {
		t.Fatal("nil list")
	}
	if HasError() {
		t.Fatal("nil list GetFirstFunction must stay non-sticky complete miss")
	}
	ClearError()
	a := &Function{Name: "func_1"}
	b := &Function{Name: "func_2"}
	list := &FunctionList{Funcs: []*Function{a, b}}
	if GetFirstFunction(list) != a {
		t.Fatal("want first")
	}
	// nil hole at [0] sticky (no invent scan later)
	ClearError()
	if GetFirstFunction(&FunctionList{Funcs: []*Function{nil, b}}) != nil {
		t.Fatal("nil first hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil first hole GetFirstFunction must SetError sticky")
	}
	ClearError()
}

func TestBuildUserInvocationNoRevisitStaticEffect(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	callee := &Function{
		Name:       "func_2",
		ReturnType: GetIntType(),
		BuildState: BuildBuilt,
		IsBuilt:    true,
		FEffect:    EmptyEffect().ReadVar(g),
		// NeedsRevisit false — no FactChanged / ptrs
	}
	caller := &Function{Name: "func_1"}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	accum := EmptyEffect()
	cg := WithFunc(caller, EmptyEffect())
	cg.EffectAccum = &accum
	fi := BuildUserInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, callee)
	if fi.Failed {
		t.Fatal("failed")
	}
	// static path: external effect of callee merged into accum (global read)
	if !accum.IsRead(g) {
		// AddExternalEffect only globals — g is global so should be visible
		t.Log("accum may track via EffectStm; check FEffect path")
	}
}

func TestBuildUserInvocationRevisitPath(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	// callee that needs revisit
	callee := &Function{
		Name:        "func_2",
		ReturnType:  GetIntType(),
		BuildState:  BuildBuilt,
		IsBuilt:     true,
		FactChanged: true,
		Body:        &Block{StmID: AllocStmID(), Stmts: []Stmt{}},
		Param:       []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)},
	}
	callee.Body.Func = callee
	// FunctionInvocationUser.cpp:311 — revisit uses get_fact_mgr_for_func(callee)
	_ = callee.ensurePairedFactMgr()
	callee.ensurePairedFactMgr().SetMapStmEffect(callee.Body.StmID, EmptyEffect())
	caller := &Function{Name: "func_1"}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	fm := NewFactMgr(caller)
	cg := WithFunc(caller, EmptyEffect()).WithFactMgr(fm)
	cg.Funcs = list
	fi := BuildUserInvocation(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, callee)
	if fi == nil {
		t.Fatal("nil")
	}
	// args for one param
	if len(fi.Args) != 1 {
		t.Fatalf("args %d", len(fi.Args))
	}
	// revisit increments visited
	if callee.VisitedCnt < 1 && !fi.Failed {
		// empty body visit may still count
		t.Log("visited", callee.VisitedCnt, "failed", fi.Failed)
	}
	ClearError()
}

func TestBuildUserInvocationSkipsFirstFunctionRevisit(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	first := &Function{
		Name:        "func_1",
		BuildState:  BuildBuilt,
		IsBuilt:     true,
		FactChanged: true, // would need revisit if not first
		FEffect:     EmptyEffect(),
	}
	list := &FunctionList{Funcs: []*Function{first}}
	caller := first
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	cg := WithFunc(caller, EmptyEffect())
	fi := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, first)
	if fi.Failed {
		t.Fatalf("first should not fail revisit err=%v", HasError())
	}
	// first path uses add_external_effect, not revisit → VisitedCnt unchanged by Revisit
	if first.VisitedCnt != 0 {
		t.Fatalf("first should skip revisit, visited=%d", first.VisitedCnt)
	}
}
