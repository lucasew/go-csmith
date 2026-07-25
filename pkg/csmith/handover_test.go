package csmith

import (
	"testing"
)

func TestIsRV(t *testing.T) {
	v := CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	if !v.IsRV() {
		t.Fatal("rv")
	}
	if CreateVariableScalars("g_1", GetIntType(), false, false).IsRV() {
		t.Fatal("not")
	}
}

func TestCallerToCalleeHandoverKeepsGlobals(t *testing.T) {
	callee := &Function{Name: "c", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p}
	fm := NewFactMgrSess(testAmbientSession, callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	// g points to loc — loc should be kept transitively
	facts := []*FactPointTo{
		MakeFactPointTo(g, loc),
		MakeFactPointTo(CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false), NullPtr),
	}
	// subject l_p is local name — not kept unless pointed
	fm.CallerToCalleeHandover(nil, &facts)
	// keep g (global); drop l_p unless pointed by keep
	if FindRelatedPointTo(facts, g) == nil {
		t.Fatal("lost global")
	}
	// loc subject not in facts initially as subject; g points to loc so if there was a fact for loc as subject...
	// only subjects kept: g and p (after param facts)
	// FactMgr.cpp:108–114 — nil arg → abstract nullptr rhs → garbage for pointer param
	// (no invent NewFactPointTo outside abstract path)
	if FindRelatedPointTo(facts, p) == nil {
		t.Fatal("nil-arg param must get abstract garbage fact", facts)
	}
	if !FindRelatedPointTo(facts, p).IsDead() {
		t.Fatal("nil arg → garbage, not invent other pointee")
	}
}

func TestCallerToCalleeHandoverTransitive(t *testing.T) {
	callee := &Function{Name: "c"}
	fm := NewFactMgrSess(testAmbientSession, callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// caller local that g points to
	loc := CreateVariableScalars("l_tgt", GetIntType(), false, false)
	// fact about loc as subject (e.g. field) — use pointer fact with subject loc that is pointed
	// better: subject is another pointer that lives on stack
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(g, lp),   // global points to stack ptr
		MakeFactPointTo(lp, loc), // stack ptr facts
	}
	fm.CallerToCalleeHandover(nil, &facts)
	// g kept; lp kept because g points to it; loc not a subject of pointer fact kept unless...
	if FindRelatedPointTo(facts, g) == nil || FindRelatedPointTo(facts, lp) == nil {
		t.Fatal("transitive", facts)
	}
}

func TestCallerToCalleeHandoverNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm.CallerToCalleeHandover(nil, &facts)
	if FactsComplete(facts) {
		t.Fatal("nil fact hole must fail closed", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactMgr + Func + inputs always live; sticky no invent soft-skip handover past hole
	(*FactMgr)(nil).CallerToCalleeHandover(nil, &facts)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM CallerToCalleeHandover must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm.CallerToCalleeHandover(nil, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil inputs CallerToCalleeHandover must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).RemoveRVFacts(&facts)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM RemoveRVFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm.RemoveRVFacts(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts RemoveRVFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCallerToCalleeUnionFactsHandover(t *testing.T) {
	// FunctionInvocationUser.cpp:206 — global_facts = caller includes eUnionWrite;
	// FactMgr.cpp:324–353 — partition keeps globals/params, drops pure stack subjects.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	callee := &Function{Name: "c", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, callee)

	// global union — must survive handover filter
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 1 {
		t.Skip("union type")
	}
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if gu == nil {
		t.Fatal("global union var")
	}
	// local union — dropped unless pointed-to by kept PT
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if lu == nil {
		t.Fatal("local union var")
	}
	fm.UnionFacts = []*FactUnion{
		MakeFactUnion(gu, 0),
		MakeFactUnion(lu, 0),
	}
	// empty keepPT (no transitive) — only globals/params
	fm.FilterUnionFactsForHandover([]*FactPointTo{})
	if !UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("complete handover filter must stay complete", fm.UnionFacts)
	}
	if FindRelatedUnion(fm.UnionFacts, gu) == nil {
		t.Fatal("global union FactUnion must survive handover")
	}
	if FindRelatedUnion(fm.UnionFacts, lu) != nil {
		t.Fatal("stack-only union FactUnion must drop on handover", fm.UnionFacts)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete FilterUnionFactsForHandover must not sticky")
	}

	// transitive: global pointer to local union keeps local FactUnion
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, callee)
	gp := CreateVariableScalars("g_p", PointerTo(ut), true, false)
	fm2.UnionFacts = []*FactUnion{MakeFactUnion(lu, 1)}
	keepPT := []*FactPointTo{MakeFactPointTo(gp, lu)}
	fm2.FilterUnionFactsForHandover(keepPT)
	if FindRelatedUnion(fm2.UnionFacts, lu) == nil {
		t.Fatal("pointee local union FactUnion must survive transitive keep", fm2.UnionFacts)
	}

	// Clone + renew round-trip (FunctionInvocationUser.cpp:206 + 221)
	ClearErrorSess(testAmbientSession)
	callerUF := []*FactUnion{MakeFactUnion(gu, 0)}
	cloned := CloneUnionFactSlice(callerUF)
	if len(cloned) != 1 || FindRelatedUnion(cloned, gu) == nil {
		t.Fatal("CloneUnionFactSlice", cloned)
	}
	// callee wrote field 1 on global
	retUF := []*FactUnion{MakeFactUnion(gu, 1)}
	if !RenewUnionFacts(&callerUF, retUF) {
		t.Fatal("RenewUnionFacts should change")
	}
	if FindRelatedUnion(callerUF, gu).LastWrittenFID != 1 {
		t.Fatal("renew last-write", callerUF)
	}
	// GlobalUnionFactsOnly drops locals
	mixed := []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(lu, 0)}
	onlyG := GlobalUnionFactsOnly(mixed)
	if FindRelatedUnion(onlyG, gu) == nil || FindRelatedUnion(onlyG, lu) != nil {
		t.Fatal("GlobalUnionFactsOnly", onlyG)
	}
}

func TestUpdateUnionFactsForOOSVars(t *testing.T) {
	// FactMgr.cpp:143–156 — OOS erase by subject match (FactUnion category too).
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("union")
	}
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	facts := []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(lu, 0)}
	UpdateUnionFactsForOOSVars([]*Variable{lu}, &facts)
	if FindRelatedUnion(facts, gu) == nil || FindRelatedUnion(facts, lu) != nil {
		t.Fatal("OOS must drop local keep global", facts)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete OOS must not sticky")
	}
	// FM path also drops UnionFacts
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(lu, 0)}
	fm.UpdateFactsForOOSVars([]*Variable{lu})
	if FindRelatedUnion(fm.UnionFacts, lu) != nil {
		t.Fatal("FM OOS must drop UnionFacts for OOS var", fm.UnionFacts)
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetMapFactsOutForBlockOOSsUnionLocals(t *testing.T) {
	// FactMgr.cpp:141–156 + Block.cpp:690–693 — set_fact_out after OOS is full FactVec.
	// Soft invent stored post-OOS point-to with live UnionFacts still holding body locals
	// → map_union_out too large → same_facts false → extra full re-visits / over-strip.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(7), opts, probs, &env, "U1")
	if ut == nil {
		t.Skip("union")
	}
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	lu := CreateVariableQfer("l_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	body := &Block{StmID: 10, Func: f, LocalVars: []*Variable{lu}, Parent: &Block{StmID: 1, Func: f}}
	f.Body = body
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(lu, 0)}
	// Nested block: OOS locals only (parent != nil skips remove_function_local)
	outPT := CloneFactSlice(fm.GlobalFacts)
	UpdateFactsForOOSVars(body.LocalVars, &outPT)
	fm.SetMapFactsOutForBlock(body, outPT)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("SetMapFactsOutForBlock sticky", HasErrorSess(testAmbientSession))
	}
	// live UnionFacts must remain pre-OOS (post_creation keeps live during FP)
	if FindRelatedUnion(fm.UnionFacts, lu) == nil {
		t.Fatal("live UnionFacts must not be mutated by SetMapFactsOutForBlock")
	}
	gotU := fm.GetMapUnionFactsOut(body.StmID)
	if FindRelatedUnion(gotU, lu) != nil {
		t.Fatal("map_union_out must drop body-local union subject", gotU)
	}
	if FindRelatedUnion(gotU, gu) == nil {
		t.Fatal("map_union_out must keep global union", gotU)
	}
	ClearErrorSess(testAmbientSession)
}

func TestCallerToCalleeHandoverParamHoleFailClosed(t *testing.T) {
	// soft invent: Param hole → IsVariableInSet false → drop param from keep
	// fair: VariablesComplete Param fails closed nil inputs sticky
	ClearErrorSess(testAmbientSession)
	callee := &Function{Name: "c", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p, nil}
	fm := NewFactMgrSess(testAmbientSession, callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(g, NullPtr), MakeFactPointTo(p, NullPtr)}
	fm.CallerToCalleeHandover(nil, &facts)
	if FactsComplete(facts) {
		t.Fatal("incomplete Param must fail closed nil inputs, not invent drop param", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Param must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariablesCompleteAndIsVariableInSet(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	if !VariablesComplete([]*Variable{a, b}) || VariablesComplete([]*Variable{a, nil, b}) {
		t.Fatal("VariablesComplete")
	}
	if !IsVariableInSet([]*Variable{a, b}, a) {
		t.Fatal("complete membership")
	}
	// incomplete: membership false (no invent skip hole to later match)
	if IsVariableInSet([]*Variable{nil, a}, a) {
		t.Fatal("incomplete set must not invent membership past hole")
	}
}

func TestRemoveRVFactsNilHole(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	facts := []*FactPointTo{nil}
	fm.RemoveRVFacts(&facts)
	if FactsComplete(facts) {
		t.Fatal("nil fact hole must fail closed", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRemoveRVFacts(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", RV: CreateVariableScalars("f_rv", GetIntType(), false, false)}
	fm := NewFactMgrSess(testAmbientSession, f)
	other := CreateVariableScalars("other_rv", GetIntType(), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), NullPtr),
		{Var: other, PointTo: []*Variable{NullPtr}},
		{Var: f.RV, PointTo: []*Variable{NullPtr}},
	}
	// only pointer facts with Type - MakeFactPointTo needs pointer type
	// use raw for rv
	fm.RemoveRVFacts(&facts)
	// other_rv dropped; f_rv kept; g_p kept
	for _, fact := range facts {
		if fact.Var == other {
			t.Fatal("other rv kept")
		}
	}
}

func TestRemoveRVFactsMatchResidualSticky(t *testing.T) {
	// Type-nil own RV: Match stickies residual ERROR+false.
	// Soft invent was soft-continue drop then keep later non-RV complete filter.
	// Fair: sticky wipe IncompleteFactSlice whole RemoveRVFacts.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", RV: &Variable{Name: "f_rv"}} // Type nil
	fm := NewFactMgrSess(testAmbientSession, f)
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	other := CreateVariableScalars("other_rv", GetIntType(), false, false)
	facts := []*FactPointTo{
		{Var: other, PointTo: []*Variable{NullPtr}}, // IsRV; Match residual vs Type-nil f.RV
		MakeFactPointTo(gp, NullPtr),
	}
	fm.RemoveRVFacts(&facts)
	if FactsComplete(facts) {
		t.Fatal("Match residual must IncompleteFactSlice, not invent later non-RV keep")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual RemoveRVFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputTab(t *testing.T) {
	if OutputTab(0) != "" || OutputTab(2) != "        " {
		t.Fatal(OutputTab(2))
	}
}

func TestAddParamFacts(t *testing.T) {
	// FactMgr.cpp:108–114 — update_fact_for_assign; null const → null fact
	// (not invent NewFactPointTo garbage when abstract succeeds)
	callee := &Function{Name: "c"}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p}
	fm := NewFactMgrSess(testAmbientSession, callee)
	facts := []*FactPointTo{}
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}, ExprType: PointerTo(GetIntType())}
	fm.AddParamFacts([]*Expression{arg}, &facts)
	got := FindRelatedPointTo(facts, p)
	if got == nil {
		t.Fatal("param fact", facts)
	}
	if got.IsDead() {
		t.Fatal("null arg must not invent garbage")
	}
	if !got.IsNull() {
		t.Fatalf("want null, got %+v", got.PointTo)
	}
	// missing arg → nullptr rhs → garbage via abstract
	facts2 := []*FactPointTo{}
	fm.AddParamFacts(nil, &facts2)
	got2 := FindRelatedPointTo(facts2, p)
	if got2 == nil || !got2.IsDead() {
		t.Fatal("nil args → garbage via abstract", facts2)
	}
}
