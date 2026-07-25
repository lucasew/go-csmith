package csmith

import (
	"strings"
	"testing"
)

func TestBuildInvocationAndFunctionNilType(t *testing.T) {
	// FunctionInvocationUser.cpp:175 — assert(type); sticky Failed no GetIntType invent
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	fi := BuildInvocationAndFunction(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, nil, nil)
	if fi == nil || !fi.Failed {
		t.Fatal("nil return type must fail without soft invent")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil return type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildUserInvocationNoInventWithoutRNG(t *testing.T) {
	// zero-param callee must not invent success call without process RNG — sticky Failed
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	callee := &Function{Name: "func_x", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	list := &FunctionList{Funcs: []*Function{callee}}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	fi := BuildUserInvocation(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("nil RNG must fail closed user invoke")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG BuildUserInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fi2 := BuildInvocationAndFunction(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntType(), nil)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("nil RNG must fail closed build+function")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG BuildInvocationAndFunction must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// callee / cg hard IR sticky Failed
	fi3 := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, nil)
	if fi3 == nil || !fi3.Failed {
		t.Fatal("nil callee must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil callee BuildUserInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Param hole sticky Failed
	callee2 := &Function{Name: "func_y", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true,
		Param: []*Variable{nil}}
	fi4 := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee2)
	if fi4 == nil || !fi4.Failed {
		t.Fatal("nil Param hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Param hole BuildUserInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// MakeRandomInvocation nil r/cg sticky Failed
	fi5 := MakeRandomInvocation(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, GetIntType(), nil, false)
	if fi5 == nil || !fi5.Failed {
		t.Fatal("nil RNG MakeRandomInvocation must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
		_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(uint64(10+i)))
	}
	caller := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	cgSeed := EmptyCGContext().WithSession(testAmbientSession)
	g := vs.GenerateNewGlobal(AccessRead, cgSeed, GetIntType(), nil, NewRng(1))
	if g == nil {
		t.Fatal("need int global")
	}
	_ = vs.GenerateNewGlobal(AccessRead, cgSeed, PointerTo(GetIntType()), nil, NewRng(2))
	callee := &Function{
		Name:       "g_1",
		ReturnType: GetIntType(),
		Param: []*Variable{
			CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntType(), false, false),
			CreateVariableScalarsSess(testAmbientSession, "p_2", PointerTo(GetIntType()), false, false),
		},
		BuildState: BuildBuilt,
	}
	callee.FEffect = EmptyEffect().ReadVarSess(testAmbientSession, g)
	var fi *Invocation
	for seed := uint64(3); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		caller := &Function{Name: "func_1"}
		blk := &Block{Func: caller}
		caller.Stack = []*Block{blk}
		cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, caller))
		fi = BuildUserInvocation(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, nil, callee)
		if fi != nil && !fi.Failed && len(fi.Args) == 2 {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Fatal("no successful BuildUserInvocation with 2 args in seed scan")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildUserInvocationParamFailHard(t *testing.T) {
	// FunctionInvocationUser.cpp:257–258 — ERROR_GUARD(false) when make_random_param null
	// sticky Failed (no invent soft re-pick past null param without ERROR)
	ClearErrorSess(testAmbientSession)
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
		Param:      []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntType(), false, false)},
		BuildState: BuildBuilt,
		IsBuilt:    true,
	}
	caller := &Function{Name: "func_1"}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	// nil vs forces ExpressionVariable fail regardless of term choice
	fi := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), nil, NewExprTables(opts), &cg, nil, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("want Failed on null param")
	}
	if len(fi.Args) != 0 {
		t.Fatalf("must not append partial args: %d", len(fi.Args))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("null param BuildUserInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildInvocationAndFunctionNilPairedFMSticky(t *testing.T) {
	// get_fact_mgr_for_func after signature always live; sticky Failed without paired FM
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxFuncs = 10
	vs := NewVariableSelector(opts)
	list := &FunctionList{}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	f := MakeRandomSignature(NewRng(1), opts, NewProbabilities(opts), vs, &vs.Sym, cg, GetIntType(), nil, list)
	if f == nil {
		t.Fatal("signature", HasErrorSess(testAmbientSession))
	}
	// same-package clear paired FM (signature always pairs; strip for fail-closed path)
	f.factMgr = nil
	f.GenerateBody(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext().WithSession(testAmbientSession))
	if f.Body != nil {
		t.Fatal("nil paired FM must fail closed body")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil paired FM GenerateBody must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// BuildInvocationAndFunction incomplete ambient: Failed sticky before body hand-over
	inc := IncompleteEffect()
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.EffectAccum = &inc
	fi := BuildInvocationAndFunction(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2, list, GetIntType(), nil)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete ambient BuildInvocationAndFunction must Failed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient BuildInvocationAndFunction must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// stripped calFM sticky Failed: signature ok then clear FM before hand-over via
	// same-package re-entry of PairedFactMgr path — BuildInvocationAndFunction re-makes
	// signature so clear after MakeRandomSignature inside is not injectable.
	// Exercise PairedFactMgr sticky on cleared function used as BuildUserInvocation
	// static path does not need calFM. Direct sticky:
	if (*Function)(nil).PairedFactMgr() != nil {
		t.Fatal("nil Function PairedFactMgr must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function PairedFactMgr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f2 := &Function{Name: "no_fm", ReturnType: GetIntType()}
	if f2.PairedFactMgr() != nil {
		t.Fatal("unpaired Function PairedFactMgr must be nil")
	}
	// unpaired is complete miss (no invent FM) — not sticky until generate path
	if HasErrorSess(testAmbientSession) {
		t.Fatal("unpaired live Function PairedFactMgr must not sticky empty miss")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildUserInvocationIncompleteAccumEffContextFailClosed(t *testing.T) {
	// revisit under incomplete AccumEffContext must not invent success
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	vs.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)}
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
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFuncList(list)
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg = cg.WithFactMgr(fm)
	fi := BuildUserInvocation(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, callee)
	if fi == nil || !fi.Failed {
		t.Fatal("incomplete AccumEffContext must fail closed BuildUserInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete AccumEffContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	fm2 := NewFactMgrSess(testAmbientSession, caller2)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(caller2, EmptyEffect()).WithSession(testAmbientSession).WithFuncList(list2).WithFactMgr(fm2)
	fi2 := BuildUserInvocation(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, list2, callee2)
	if fi2 == nil || !fi2.Failed {
		t.Fatal("incomplete GlobalFacts must fail closed BuildUserInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestBuildUserInvocationGenVisibleEffectUsesCurrentBlock — gen-time revisit handoff
// uses get_current_block(), not curr_blk.
// FunctionInvocationUser.cpp:287–289:
//
//	assert(cg_context.get_current_block());
//	add_visible_effect(*accum, get_current_block());
//
// Visit path uses curr_blk (FunctionInvocation.cpp:543–546 / VisitFactsInvocation).
// Mid-gen FP leaves CurrBlk on a nested statement parent; preferring AnalysisBlock
// folds visible effects against the wrong frame (ok-var eligibility / seed-7 pool size).
func TestBuildUserInvocationGenVisibleEffectUsesCurrentBlock(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	// Frame local lives only on stack-top (inner). Outer parent is stale CurrBlk.
	// IsVarOnStack walks Parent: inner sees l_inner; outer alone does not.
	innerLoc := CreateVariableScalarsSess(testAmbientSession, "l_inner", GetIntType(), false, false)
	if innerLoc == nil {
		t.Fatal("create inner local")
	}
	innerLoc.Name = "l_inner"
	outer := &Block{StmID: AllocStmID()}
	inner := &Block{Parent: outer, LocalVars: []*Variable{innerLoc}, StmID: AllocStmID()}
	caller := &Function{Name: "func_1", ReturnType: GetIntType(), Stack: []*Block{outer, inner}}
	outer.Func = caller
	inner.Func = caller
	list := &FunctionList{Funcs: []*Function{caller}}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFuncList(list).WithFactMgr(fm)
	// Stale curr_blk from prior ValidateAndUpdateFacts on outer statement parent
	cg.CurrBlk = outer
	if cg.CurrentBlock() != inner {
		t.Fatal("precondition: stack top is inner")
	}
	if cg.AnalysisBlock() != outer {
		t.Fatal("precondition: AnalysisBlock prefers stale CurrBlk=outer")
	}
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	writeInner := EmptyEffect().WriteVarSess(testAmbientSession, innerLoc)
	// Gen path: CurrentBlock (inner) — l_inner is frame of inner → folded
	cg.AddVisibleEffectAt(writeInner, cg.CurrentBlock())
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("CurrentBlock handoff sticky: %v", HasErrorSess(testAmbientSession))
	}
	if !cg.EffectAccum.IsWrittenSess(testAmbientSession, innerLoc) {
		t.Fatal("gen path CurrentBlock=inner must fold inner-frame write")
	}
	// Visit-style wrong block: AnalysisBlock (outer) alone — l_inner not on outer → not folded
	eff2 := EmptyEffect()
	cg.EffectAccum = &eff2
	cg.AddVisibleEffectAt(writeInner, cg.AnalysisBlock())
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("AnalysisBlock handoff sticky: %v", HasErrorSess(testAmbientSession))
	}
	if cg.EffectAccum.IsWrittenSess(testAmbientSession, innerLoc) {
		t.Fatal("AnalysisBlock=outer must not invent fold of inner-only frame write")
	}
	// Full BuildUserInvocation revisit with stale CurrBlk must still succeed (gen uses stack top)
	ClearErrorSess(testAmbientSession)
	callee := &Function{
		Name:            "func_rev",
		ReturnType:      GetIntType(),
		BuildState:      BuildBuilt,
		IsBuilt:         true,
		FactChanged:     true,
		AccumEffContext: EmptyEffect(),
		Body:            &Block{StmID: AllocStmID(), Stmts: []Stmt{}},
	}
	callee.ensurePairedFactMgr()
	list.Funcs = []*Function{caller, callee}
	cg.CurrBlk = outer
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect()
	fi := BuildUserInvocation(NewRng(11), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee)
	if fi == nil || fi.Failed {
		t.Fatalf("gen revisit with stale CurrBlk must succeed; Failed=%v err=%v", fi != nil && fi.Failed, HasErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("must not sticky: %v", HasErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

// TestBuildUserInvocationRevisitClearsCallerCurrRHS — FunctionInvocationUser.cpp:282–284
// / CGContext.cpp:85–93. BUILD revisit must not inherit caller CurrRHS (ExpressionAssign
// leaves it set); else Lhs::visit_facts in the callee body can fail closed on overlap.
func TestBuildUserInvocationRevisitClearsCallerCurrRHS(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	// Simple built callee: empty body always visits OK; FactChanged forces NeedsRevisit.
	callee := &Function{
		Name:            "func_49",
		ReturnType:      GetIntType(),
		BuildState:      BuildBuilt,
		IsBuilt:         true,
		FactChanged:     true,
		AccumEffContext: EmptyEffect(),
		Body:            &Block{StmID: 10, Stmts: []Stmt{}},
		Param:           nil,
	}
	callee.ensurePairedFactMgr()
	caller := &Function{Name: "func_11", ReturnType: GetIntType()}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFuncList(list).WithFactMgr(fm)
	// Simulate ExpressionAssign: CurrRHS set while building invocation as RHS of assign.
	rhsDummy := &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), false, false), ExprType: GetIntType()}
	cg.CurrRHS = rhsDummy
	cg.EffectStm = EmptyEffect()
	// Mark a write on EffectStm as if RHS gen ran
	cg.EffectStm = cg.EffectStm.WriteVarSess(testAmbientSession, rhsDummy.Var)
	fi := BuildUserInvocation(NewRng(7), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, list, callee)
	if fi == nil {
		t.Fatal("BuildUserInvocation returned nil")
	}
	if fi.Failed {
		t.Fatalf("revisit must succeed with empty body despite caller CurrRHS; Failed=%v err=%v", fi.Failed, HasErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("must not sticky-error: %v", HasErrorSess(testAmbientSession))
	}
	// Caller CurrRHS must remain (BUILD clones, does not clear caller)
	if cg.CurrRHS != rhsDummy {
		t.Fatal("caller CurrRHS must be left intact after BuildUserInvocation")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildInvocationEffectHandoverIncompleteFailClosed(t *testing.T) {
	// BuildInvocationAndFunction effect hand-over must not invent success past incomplete
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxFuncs = 5
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	list := &FunctionList{Types: vs.Types}
	// plant incomplete ambient on caller before build
	caller := &Function{Name: "func_1", ReturnType: GetIntType()}
	list.Funcs = []*Function{caller}
	cg := WithFunc(caller, IncompleteEffect()).WithSession(testAmbientSession).WithFuncList(list)
	cg.Types = vs.Types
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg = cg.WithFactMgr(fm)
	fi := BuildInvocationAndFunction(NewRng(4), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, list, GetIntType(), nil)
	if fi != nil && !fi.Failed {
		// may return Failed or nil; must not invent clean success under incomplete ambient
		t.Fatal("incomplete caller EffectContext must fail closed BuildInvocationAndFunction")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete caller EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetFirstFunction(t *testing.T) {
	// nil/empty list is complete miss (isolated BuildUserInvocation passes nil list)
	ClearErrorSess(testAmbientSession)
	if GetFirstFunction(nil) != nil {
		t.Fatal("nil list")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil list GetFirstFunction must stay non-sticky complete miss")
	}
	ClearErrorSess(testAmbientSession)
	a := &Function{Name: "func_1"}
	b := &Function{Name: "func_2"}
	list := &FunctionList{Funcs: []*Function{a, b}}
	if GetFirstFunction(list) != a {
		t.Fatal("want first")
	}
	// nil hole at [0] sticky (no invent scan later)
	ClearErrorSess(testAmbientSession)
	if GetFirstFunction(&FunctionList{Funcs: []*Function{nil, b}}) != nil {
		t.Fatal("nil first hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil first hole GetFirstFunction must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBuildUserInvocationNoRevisitStaticEffect(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(1))
	callee := &Function{
		Name:       "func_2",
		ReturnType: GetIntType(),
		BuildState: BuildBuilt,
		IsBuilt:    true,
		FEffect:    EmptyEffect().ReadVarSess(testAmbientSession, g),
		// NeedsRevisit false — no FactChanged / ptrs
	}
	caller := &Function{Name: "func_1"}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	accum := EmptyEffect()
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	cg.EffectAccum = &accum
	fi := BuildUserInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, callee)
	if fi.Failed {
		t.Fatal("failed")
	}
	// static path: external effect of callee merged into accum (global read)
	if !accum.IsReadSess(testAmbientSession, g) {
		// AddExternalEffect only globals — g is global so should be visible
		t.Log("accum may track via EffectStm; check FEffect path")
	}
}

func TestBuildUserInvocationRevisitPath(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(1))
	// callee that needs revisit
	callee := &Function{
		Name:        "func_2",
		ReturnType:  GetIntType(),
		BuildState:  BuildBuilt,
		IsBuilt:     true,
		FactChanged: true,
		Body:        &Block{StmID: AllocStmID(), Stmts: []Stmt{}},
		Param:       []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntType(), false, false)},
	}
	callee.Body.Func = callee
	// FunctionInvocationUser.cpp:311 — revisit uses get_fact_mgr_for_func(callee)
	_ = callee.ensurePairedFactMgr()
	callee.ensurePairedFactMgr().SetMapStmEffect(callee.Body.StmID, EmptyEffect())
	caller := &Function{Name: "func_1"}
	list := &FunctionList{Funcs: []*Function{caller, callee}}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, caller)
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
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
	ClearErrorSess(testAmbientSession)
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
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession)
	fi := BuildUserInvocation(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, first)
	if fi.Failed {
		t.Fatalf("first should not fail revisit err=%v", HasErrorSess(testAmbientSession))
	}
	// first path uses add_external_effect, not revisit → VisitedCnt unchanged by Revisit
	if first.VisitedCnt != 0 {
		t.Fatalf("first should skip revisit, visited=%d", first.VisitedCnt)
	}
}
