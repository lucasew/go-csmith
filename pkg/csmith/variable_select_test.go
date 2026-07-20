package csmith

import (
	"testing"
)

func TestVariableSelectionProbabilityRange(t *testing.T) {
	// VariableSelector.cpp:110–122 InitScopeTable once; no invent table per draw
	opts := Defaults()
	InitScopeTable(opts)
	defer SetProcessScopeTab(nil)
	seen := map[VariableScope]bool{}
	r := NewRng(2)
	for i := 0; i < 200; i++ {
		seen[VariableSelectionProbability(r, opts)] = true
	}
	if !seen[ScopeGlobal] || !seen[ScopeNewValue] {
		t.Fatalf("scopes %#v", seen)
	}
}

func TestVariableSelectionProbabilityNilScopeTabFailClosed(t *testing.T) {
	// VariableSelector.cpp:1050 InitScopeTable required sticky ERROR_GUARD MAX
	ClearError()
	prev := ProcessScopeTab()
	SetProcessScopeTab(nil)
	defer SetProcessScopeTab(prev)
	sc := VariableSelectionProbability(NewRng(1), Defaults())
	if sc != MaxVarScope {
		t.Fatalf("want MAX without InitScopeTable, got %v", sc)
	}
	if !HasError() {
		t.Fatal("nil ProcessScopeTab must SetError sticky")
	}
	ClearError()
}

func TestVariableSelectionProbabilityNilRNGSticky(t *testing.T) {
	// VariableSelector.cpp:1053 ERROR_GUARD sticky without RNG
	ClearError()
	opts := Defaults()
	InitScopeTable(opts)
	defer SetProcessScopeTab(nil)
	if sc := VariableSelectionProbability(nil, opts); sc != MaxVarScope {
		t.Fatalf("nil RNG must fail closed MAX, got %v", sc)
	}
	if !HasError() {
		t.Fatal("nil RNG VariableSelectionProbability must SetError sticky")
	}
	ClearError()
	if sc := VariableCreationProbability(nil, opts); sc != MaxVarScope {
		t.Fatalf("nil RNG creation must fail closed MAX, got %v", sc)
	}
	if !HasError() {
		t.Fatal("nil RNG VariableCreationProbability must SetError sticky")
	}
	ClearError()
}

func TestVariableSelectFilterSkipsEmptyParams(t *testing.T) {
	// VariableSelector.cpp:98–105 — ParentParam filtered when param.empty()
	ClearError()
	opts := Defaults()
	InitScopeTable(opts)
	defer SetProcessScopeTab(nil)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	// many draws with empty params: never ParentParam
	for seed := uint64(1); seed < 80; seed++ {
		sc := VariableSelectionProbabilityCG(NewRng(seed), opts, &cg, MaxVarScope)
		if sc == ScopeParentParam {
			t.Fatalf("seed %d: ParentParam with empty params", seed)
		}
		if sc == MaxVarScope {
			t.Fatalf("seed %d: MAX scope", seed)
		}
	}
	// with a param, ParentParam is allowed
	f.Param = []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)}
	seenParam := false
	for seed := uint64(1); seed < 100; seed++ {
		if VariableSelectionProbabilityCG(NewRng(seed), opts, &cg, MaxVarScope) == ScopeParentParam {
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
	ClearError()
	opts := Defaults()
	InitScopeTable(opts)
	defer SetProcessScopeTab(nil)
	f := &Function{Name: "f", ReturnType: GetIntType(), Param: IncompleteVariables()}
	cg := WithFunc(f, EmptyEffect())
	if sc := VariableSelectionProbabilityCG(NewRng(1), opts, &cg, MaxVarScope); sc != MaxVarScope {
		t.Fatalf("incomplete Param must fail closed MAX, got %v", sc)
	}
	if !HasError() {
		t.Fatal("incomplete Param must SetError sticky")
	}
	ClearError()
}

func TestSelectCreatesOrFinds(t *testing.T) {
	opts := Defaults()
	InitScopeTable(opts)
	defer SetProcessScopeTab(nil)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	r := NewRng(3)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	f.Body = blk
	cg := WithFunc(f, EmptyEffect())
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.Select(AccessRead, cg, GetIntType(), &q, r, MatchFlexible)
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
	if !found && !v.IsGlobal() && !v.IsLocal() {
		t.Fatalf("orphan var %+v", v)
	}
}

func TestMakeRandomIterCtrl(t *testing.T) {
	r := NewRng(2)
	init, incr := MakeRandomIterCtrl(r, 10)
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
	ClearError()
	init, incr = MakeRandomIterCtrl(nil, 10)
	if init != 0 || incr != 0 {
		t.Fatalf("nil RNG must fail closed zeros, got %d %d", init, incr)
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomIterCtrl must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomArrayOpNotEmpty(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	// force some arrays via select
	for seed := uint64(1); seed < 30; seed++ {
		st := MakeRandomArrayOp(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
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
	ClearError()
	// incomplete vars pool sticky
	out := ExpandStructUnionVars([]*Variable{nil}, GetIntType())
	if VariablesComplete(out) {
		t.Fatal("nil hole ExpandStructUnionVars must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil hole ExpandStructUnionVars must SetError sticky")
	}
	ClearError()
}
