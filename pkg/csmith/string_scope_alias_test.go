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
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if EmptyCGContext().WithSession(testAmbientSession).FindVariableScope(g) != ScopeGlobalVar {
		t.Fatal("global")
	}
	f := &Function{Name: "f"}
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)
	f.Param = []*Variable{p}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if cg.FindVariableScope(p) != 0 {
		t.Fatal("param", cg.FindVariableScope(p))
	}
	if cg.FindVariableScope(loc) != 1 {
		t.Fatal("local", cg.FindVariableScope(loc))
	}
	// on caller frame
	callerBlk := &Block{LocalVars: []*Variable{CreateVariableScalarsSess(testAmbientSession, "l_c", GetIntTypeSess(testAmbientSession), false, false)}}
	cg.CallChain = []*Block{callerBlk}
	lc := callerBlk.LocalVars[0]
	if cg.FindVariableScope(lc) != ScopeInvisible {
		t.Fatal("invisible", cg.FindVariableScope(lc))
	}
	if cg.FindVariableScope(CreateVariableScalarsSess(testAmbientSession, "l_x", GetIntTypeSess(testAmbientSession), false, false)) != ScopeInactive {
		t.Fatal("inactive")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete FindVariableScope paths must not sticky")
	}
	// incomplete Param sticky ScopeInactive
	ClearErrorSess(testAmbientSession)
	f.Param = []*Variable{p, nil}
	if cg.FindVariableScope(loc) != ScopeInactive {
		t.Fatal("Param hole must fail closed ScopeInactive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Param hole FindVariableScope must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Param = []*Variable{p}
	// non-global without CurrentFunc sticky (no invent "not found" past missing frame)
	ClearErrorSess(testAmbientSession)
	if EmptyCGContext().WithSession(testAmbientSession).FindVariableScope(loc) != ScopeInactive {
		t.Fatal("nil CurrentFunc local must fail closed ScopeInactive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CurrentFunc FindVariableScope must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// global without CurrentFunc stays complete ScopeGlobalVar
	if EmptyCGContext().WithSession(testAmbientSession).FindVariableScope(g) != ScopeGlobalVar {
		t.Fatal("global without CurrentFunc must stay ScopeGlobalVar")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("global without CurrentFunc must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Match residual: Type-nil param soft invent was soft-continue then later scope.
	// Fair: sticky ScopeInactive.
	holeParam := &Variable{Name: "p_hole", Type: nil}
	f.Param = []*Variable{holeParam, p}
	if cg.FindVariableScope(p) != ScopeInactive {
		t.Fatal("Match residual FindVariableScope must fail closed ScopeInactive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual FindVariableScope must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Param = []*Variable{p}
}

func TestUpdatePtrAliasesAndAggregate(t *testing.T) {
	ClearPointToAggregatesSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
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
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), false, false)
	broken.Type = nil
	if UpdatePtrAliases([]*FactPointTo{MakeFactPointTo(broken, p)}, &ptrs, &aliases) {
		t.Fatal("Type-nil subject must fail closed UpdatePtrAliases")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil subject UpdatePtrAliases must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if UpdatePtrAliases([]*FactPointTo{nil}, &ptrs, &aliases) {
		t.Fatal("nil fact hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole UpdatePtrAliases must SetError sticky")
	}
	// residual hygiene — Aggregate / OutputStatistics are complete-path emit
	ClearErrorSess(testAmbientSession)
	// Aggregate from FactMgr
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = facts[:1]
	fm.SetMapFactsOut(1, facts)
	f := &Function{Name: "f", BuildState: BuildBuilt, IsBuilt: true}
	fms := NewFactMgrMapSess(testAmbientSession)
	fms.byFunc = map[*Function]*FactMgr{f: fm}
	AggregateAllPointToSets([]*Function{f}, fms)
	if len(currentSession().AllPtrs) != 1 {
		t.Fatal(currentSession().AllPtrs)
	}
	out := OutputStatisticsSess(testAmbientSession, nil, Defaults())
	if !strings.Contains(out, "total number of pointers: 1") {
		t.Fatal(out)
	}
	// nil Function hole / missing FM fails closed sticky (clears aggregates)
	ClearErrorSess(testAmbientSession)
	AggregateAllPointToSets([]*Function{f, nil}, fms)
	if len(currentSession().AllPtrs) != 0 {
		t.Fatal("nil hole must clear aggregates", currentSession().AllPtrs)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	AggregateAllPointToSets([]*Function{f}, nil)
	if len(currentSession().AllPtrs) != 0 {
		t.Fatal("nil FactMgrMap must clear aggregates", currentSession().AllPtrs)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FactMgrMap must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete map_facts_out / GlobalFacts sticky clear (UpdatePtrAliases SetError)
	fmBad := NewFactMgrSess(testAmbientSession, nil)
	fmBad.GlobalFacts = []*FactPointTo{nil}
	fBad := &Function{Name: "f_bad", BuildState: BuildBuilt, IsBuilt: true}
	fmsBad := NewFactMgrMapSess(testAmbientSession)
	fmsBad.byFunc = map[*Function]*FactMgr{fBad: fmBad}
	AggregateAllPointToSets([]*Function{fBad}, fmsBad)
	if len(currentSession().AllPtrs) != 0 {
		t.Fatal("incomplete GlobalFacts must clear aggregates", currentSession().AllPtrs)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts AggregateAllPointToSets must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ClearPointToAggregatesSess(testAmbientSession)
}

func TestInt2Str(t *testing.T) {
	if Int2Str(42) != "42" {
		t.Fatal(Int2Str(42))
	}
}
