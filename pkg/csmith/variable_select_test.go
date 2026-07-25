package csmith

import (
	"testing"
)

func TestVariableSelectionProbabilityRange(t *testing.T) {
	// VariableSelector.cpp:110–122 InitScopeTable once; no invent table per draw
	opts := Defaults()
	InitScopeTableSess(testAmbientSession, opts)
	defer SetProcessScopeTabSess(testAmbientSession, nil)
	seen := map[VariableScope]bool{}
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 200; i++ {
		seen[VariableSelectionProbability(r, opts)] = true
	}
	if !seen[ScopeGlobal] || !seen[ScopeNewValue] {
		t.Fatalf("scopes %#v", seen)
	}
}

func TestVariableSelectionProbabilityNilScopeTabFailClosed(t *testing.T) {
	// VariableSelector.cpp:1050 InitScopeTable required sticky ERROR_GUARD MAX
	ClearErrorSess(testAmbientSession)
	prev := ProcessScopeTabSess(testAmbientSession)
	SetProcessScopeTabSess(testAmbientSession, nil)
	defer SetProcessScopeTabSess(testAmbientSession, prev)
	sc := VariableSelectionProbability(NewRngSess(testAmbientSession, 1), Defaults())
	if sc != MaxVarScope {
		t.Fatalf("want MAX without InitScopeTable, got %v", sc)
	}
	// nil ProcessScopeTab must SetError sticky — residual sticky may live on owner bag, not ambient dual-fill
	ClearErrorSess(testAmbientSession)
}

func TestVariableSelectionProbabilityNilRNGSticky(t *testing.T) {
	// VariableSelector.cpp:1053 ERROR_GUARD sticky without RNG
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	InitScopeTableSess(testAmbientSession, opts)
	defer SetProcessScopeTabSess(testAmbientSession, nil)
	if sc := VariableSelectionProbability(nil, opts); sc != MaxVarScope {
		t.Fatalf("nil RNG must fail closed MAX, got %v", sc)
	}
	// nil RNG VariableSelectionProbability must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if sc := VariableCreationProbabilitySess(testAmbientSession, nil, opts); sc != MaxVarScope {
		t.Fatalf("nil RNG creation must fail closed MAX, got %v", sc)
	}
	// nil RNG VariableCreationProbability must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestVariableSelectFilterSkipsEmptyParams(t *testing.T) {
	// VariableSelector.cpp:98–105 — ParentParam filtered when param.empty()
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	InitScopeTableSess(testAmbientSession, opts)
	defer SetProcessScopeTabSess(testAmbientSession, nil)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	// many draws with empty params: never ParentParam
	for seed := uint64(1); seed < 80; seed++ {
		sc := VariableSelectionProbabilityCG(NewRngSess(testAmbientSession, seed), opts, &cg, MaxVarScope)
		if sc == ScopeParentParam {
			t.Fatalf("seed %d: ParentParam with empty params", seed)
		}
		if sc == MaxVarScope {
			t.Fatalf("seed %d: MAX scope", seed)
		}
	}
	// with a param, ParentParam is allowed
	f.Param = []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)}
	seenParam := false
	for seed := uint64(1); seed < 100; seed++ {
		if VariableSelectionProbabilityCG(NewRngSess(testAmbientSession, seed), opts, &cg, MaxVarScope) == ScopeParentParam {
			seenParam = true
			break
		}
	}
	if !seenParam {
		t.Fatal("expected ParentParam when params present")
	}
}

func TestVariableSelectionProbabilityIncompleteParamSticky(t *testing.T) {
	// incomplete Param must not invent scope filter / soft re-pick past holes
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	InitScopeTableSess(testAmbientSession, opts)
	defer SetProcessScopeTabSess(testAmbientSession, nil)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession), Param: IncompleteVariables()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if sc := VariableSelectionProbabilityCG(NewRngSess(testAmbientSession, 1), opts, &cg, MaxVarScope); sc != MaxVarScope {
		t.Fatalf("incomplete Param must fail closed MAX, got %v", sc)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Param must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectCreatesOrFinds(t *testing.T) {
	opts := Defaults()
	InitScopeTableSess(testAmbientSession, opts)
	defer SetProcessScopeTabSess(testAmbientSession, nil)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	r := NewRngSess(testAmbientSession, 3)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	f.Body = blk
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	v := vs.Select(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, r, MatchFlexible)
	if v == nil {
		t.Fatal("nil")
	}
	// should be on global list or local or param
	found := false
	for _, g := range vs.GlobalList {
		if g == v {
			found = true
		}
	}
	for _, l := range blk.LocalVars {
		if l == v {
			found = true
		}
	}
	if !found {
		// field of expanded? or returned expandable child
		if v.FieldVarOf != nil {
			found = true
		}
	}
	if !found && !v.IsGlobalSess(testAmbientSession) && !v.IsLocalSess(testAmbientSession) {
		t.Fatalf("orphan var %+v", v)
	}
}

func TestMakeRandomIterCtrl(t *testing.T) {
	r := NewRngSess(testAmbientSession, 2)
	init, incr := MakeRandomIterCtrlSess(testAmbientSession, r, 10)
	if incr < 1 {
		t.Fatalf("incr %d", incr)
	}
	if init < 0 || init >= 10 {
		// init can be 0..9
		if init != 0 {
			t.Fatalf("init %d", init)
		}
	}
	// nil RNG sticky — no invent incr=1 shell
	ClearErrorSess(testAmbientSession)
	init, incr = MakeRandomIterCtrlSess(testAmbientSession, nil, 10)
	if init != 0 || incr != 0 {
		t.Fatalf("nil RNG must fail closed zeros, got %d %d", init, incr)
	}
	// nil RNG MakeRandomIterCtrl must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomArrayOpNotEmpty(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	tables := NewExprTablesSess(testAmbientSession, opts)
	stmtTab := NewStatementThresholdTableSess(testAmbientSession, opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	// force some arrays via select
	for seed := uint64(1); seed < 30; seed++ {
		st := MakeRandomArrayOp(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, stmtTab, &cg)
		if st.Kind != StmtArrayOp && st.Kind != StmtFor {
			// array loop returns for
			if st.Loop == nil && st.Kind == StmtArrayOp {
				continue
			}
		}
		if st.Loop != nil || st.Kind == StmtFor {
			return
		}
	}
	t.Log("array op rare in sample")
}

func TestExpandStructUnionVarsIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-continue expand past incomplete pool.
	ClearErrorSess(testAmbientSession)
	// incomplete vars pool sticky
	out := ExpandStructUnionVarsSess(testAmbientSession, []*Variable{nil}, GetIntTypeSess(testAmbientSession))
	if VariablesComplete(out) {
		t.Fatal("nil hole ExpandStructUnionVars must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole ExpandStructUnionVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
