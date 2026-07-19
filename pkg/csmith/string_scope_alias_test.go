package csmith

import (
	"strings"
	"testing"
)

func TestSplitString(t *testing.T) {
	got := SplitString("a, b, c", ',')
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatal(got)
	}
	got = SplitString("  x  ;  y ", ';')
	// ignore_spaces only at token start; trailing spaces kept (upstream)
	if len(got) != 2 || !strings.HasPrefix(got[0], "x") || !strings.Contains(got[1], "y") {
		t.Fatal(got)
	}
}

func TestGetSubstring(t *testing.T) {
	s := GetSubstring("(UInt, UChar)", '(', ')')
	if s != "UInt, UChar" {
		t.Fatal(s)
	}
	if GetSubstring("nope", '(', ')') != "" {
		t.Fatal("empty")
	}
}

func TestFindVariableScope(t *testing.T) {
	ClearError()
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if EmptyCGContext().FindVariableScope(g) != ScopeGlobalVar {
		t.Fatal("global")
	}
	f := &Function{Name: "f"}
	p := CreateVariableScalars("p_1", GetIntType(), false, false)
	f.Param = []*Variable{p}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	if cg.FindVariableScope(p) != 0 {
		t.Fatal("param", cg.FindVariableScope(p))
	}
	if cg.FindVariableScope(loc) != 1 {
		t.Fatal("local", cg.FindVariableScope(loc))
	}
	// on caller frame
	callerBlk := &Block{LocalVars: []*Variable{CreateVariableScalars("l_c", GetIntType(), false, false)}}
	cg.CallChain = []*Block{callerBlk}
	lc := callerBlk.LocalVars[0]
	if cg.FindVariableScope(lc) != ScopeInvisible {
		t.Fatal("invisible", cg.FindVariableScope(lc))
	}
	if cg.FindVariableScope(CreateVariableScalars("l_x", GetIntType(), false, false)) != ScopeInactive {
		t.Fatal("inactive")
	}
	if HasError() {
		t.Fatal("complete FindVariableScope paths must not sticky")
	}
	// incomplete Param sticky ScopeInactive
	ClearError()
	f.Param = []*Variable{p, nil}
	if cg.FindVariableScope(loc) != ScopeInactive {
		t.Fatal("Param hole must fail closed ScopeInactive")
	}
	if !HasError() {
		t.Fatal("Param hole FindVariableScope must SetError sticky")
	}
	ClearError()
	f.Param = []*Variable{p}
	// non-global without CurrentFunc sticky (no invent "not found" past missing frame)
	ClearError()
	if EmptyCGContext().FindVariableScope(loc) != ScopeInactive {
		t.Fatal("nil CurrentFunc local must fail closed ScopeInactive")
	}
	if !HasError() {
		t.Fatal("nil CurrentFunc FindVariableScope must SetError sticky")
	}
	ClearError()
	// global without CurrentFunc stays complete ScopeGlobalVar
	if EmptyCGContext().FindVariableScope(g) != ScopeGlobalVar {
		t.Fatal("global without CurrentFunc must stay ScopeGlobalVar")
	}
	if HasError() {
		t.Fatal("global without CurrentFunc must not sticky")
	}
	ClearError()
	// Match residual: Type-nil param soft invent was soft-continue then later scope.
	// Fair: sticky ScopeInactive.
	holeParam := &Variable{Name: "p_hole", Type: nil}
	f.Param = []*Variable{holeParam, p}
	if cg.FindVariableScope(p) != ScopeInactive {
		t.Fatal("Match residual FindVariableScope must fail closed ScopeInactive")
	}
	if !HasError() {
		t.Fatal("Match residual FindVariableScope must SetError sticky")
	}
	ClearError()
	f.Param = []*Variable{p}
}

func TestUpdatePtrAliasesAndAggregate(t *testing.T) {
	ClearPointToAggregates()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a), MakeFactPointTo(p, NullPtr)}
	var ptrs []*Variable
	var aliases [][]*Variable
	if !UpdatePtrAliases(facts[:1], &ptrs, &aliases) || !UpdatePtrAliases(facts[1:], &ptrs, &aliases) {
		t.Fatal("complete facts must succeed")
	}
	if len(ptrs) != 1 || len(aliases[0]) != 2 {
		t.Fatal(ptrs, aliases)
	}
	// nil fact hole fails closed
	// Type-nil non-special must fail closed (no invent soft-skip partial alias)
	broken := CreateVariableScalars("g_broken", GetIntType(), false, false)
	broken.Type = nil
	if UpdatePtrAliases([]*FactPointTo{MakeFactPointTo(broken, p)}, &ptrs, &aliases) {
		t.Fatal("Type-nil subject must fail closed UpdatePtrAliases")
	}
	if UpdatePtrAliases([]*FactPointTo{nil}, &ptrs, &aliases) {
		t.Fatal("nil fact hole must fail closed")
	}
	// Aggregate from FactMgr
	fm := NewFactMgr(nil)
	fm.GlobalFacts = facts[:1]
	fm.SetMapFactsOut(1, facts)
	f := &Function{Name: "f", BuildState: BuildBuilt, IsBuilt: true}
	fms := NewFactMgrMap()
	fms.byFunc = map[*Function]*FactMgr{f: fm}
	AggregateAllPointToSets([]*Function{f}, fms)
	if len(AllPtrs) != 1 {
		t.Fatal(AllPtrs)
	}
	out := OutputStatistics(nil, Defaults())
	if !strings.Contains(out, "total number of pointers: 1") {
		t.Fatal(out)
	}
	// nil Function hole / missing FM fails closed sticky (clears aggregates)
	ClearError()
	AggregateAllPointToSets([]*Function{f, nil}, fms)
	if len(AllPtrs) != 0 {
		t.Fatal("nil hole must clear aggregates", AllPtrs)
	}
	if !HasError() {
		t.Fatal("nil Function hole must SetError sticky")
	}
	ClearError()
	AggregateAllPointToSets([]*Function{f}, nil)
	if len(AllPtrs) != 0 {
		t.Fatal("nil FactMgrMap must clear aggregates", AllPtrs)
	}
	if !HasError() {
		t.Fatal("nil FactMgrMap must SetError sticky")
	}
	ClearError()
	// incomplete map_facts_out / GlobalFacts sticky clear (UpdatePtrAliases SetError)
	fmBad := NewFactMgr(nil)
	fmBad.GlobalFacts = []*FactPointTo{nil}
	fBad := &Function{Name: "f_bad", BuildState: BuildBuilt, IsBuilt: true}
	fmsBad := NewFactMgrMap()
	fmsBad.byFunc = map[*Function]*FactMgr{fBad: fmBad}
	AggregateAllPointToSets([]*Function{fBad}, fmsBad)
	if len(AllPtrs) != 0 {
		t.Fatal("incomplete GlobalFacts must clear aggregates", AllPtrs)
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts AggregateAllPointToSets must SetError sticky")
	}
	ClearError()
	ClearPointToAggregates()
}

func TestInt2Str(t *testing.T) {
	if Int2Str(42) != "42" {
		t.Fatal(Int2Str(42))
	}
}
