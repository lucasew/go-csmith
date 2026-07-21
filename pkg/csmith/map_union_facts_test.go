package csmith

import "testing"

// FactMgr.cpp set_fact_in / StatementFor.cpp:355 — map_facts_in stores full FactVec
// (ePointTo + eUnionWrite). post_loop assigns global_facts = map_facts_in[&body]
// for both partitions so IsNonreadableField sees body-entry last-writes.
func TestMapFactsInPairsUnionWrite(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || !ut.IsUnion() {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if uv == nil || len(uv.FieldVars) < 1 {
		t.Skip("fields")
	}
	entryU := MakeFactUnion(uv, 0)
	if entryU == nil {
		t.Fatal("MakeFactUnion entry", HasError())
	}
	bodyU := MakeFactUnion(uv, FactUnionBottom)
	if bodyU == nil {
		t.Fatal("MakeFactUnion body", HasError())
	}
	p := CreateVariableScalars("g_p", GetIntType(), true, false)
	pt := MakeFactPointTo(p, NullPtr)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{pt}
	fm.UnionFacts = []*FactUnion{entryU}
	// block entry set_fact_in (pairs live UnionFacts)
	fm.SetMapFactsIn(10, fm.GlobalFacts)
	// mutate live lattice as body would
	fm.UnionFacts = []*FactUnion{bodyU}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	// post_loop: global_facts = map_facts_in[&body]
	fm.AssignGlobalFactsFromMapIn(10)
	if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("assign from map_in incomplete", fm.GlobalFacts, fm.UnionFacts, HasError())
	}
	got := FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.LastWrittenFID != 0 {
		t.Fatalf("want entry last-write 0 restored, got %+v", fm.UnionFacts)
	}
	gotPT := FindRelatedPointTo(fm.GlobalFacts, p)
	if gotPT == nil || !gotPT.IsNull() {
		t.Fatalf("want entry PT null restored, got %+v", fm.GlobalFacts)
	}
}

// StatementFor.cpp:356–359 must_return restores full pre_facts FactVec.
func TestRestoreFactsPairRewindsUnion(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(7), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if uv == nil {
		t.Fatal("uv")
	}
	preU := MakeFactUnion(uv, 0)
	liveU := MakeFactUnion(uv, FactUnionBottom)
	p := CreateVariableScalars("g_p", GetIntType(), true, false)
	prePT := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	fm.UnionFacts = []*FactUnion{liveU}
	fm.RestoreFactsPair(prePT, []*FactUnion{preU})
	got := FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.LastWrittenFID != 0 {
		t.Fatalf("restore pair want fid 0, got %+v", fm.UnionFacts)
	}
	gotPT := FindRelatedPointTo(fm.GlobalFacts, p)
	if gotPT == nil || !gotPT.IsNull() {
		t.Fatalf("restore pair want null PT, got %+v", fm.GlobalFacts)
	}
}

// FactMgr.cpp:580–582 merge_jump_facts missing eUnionWrite → BOTTOM.
func TestMergeJumpUnionFactsMissingIsBottom(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(9), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	live := []*FactUnion{MakeFactUnion(uv, 0)}
	if !mergeJumpUnionFacts(&live, []*FactUnion{}) {
		t.Fatal("merge failed", HasError())
	}
	got := FindRelatedUnion(live, uv)
	if got == nil || got.LastWrittenFID != FactUnionBottom {
		t.Fatalf("want BOTTOM after jump-missing, got %+v", live)
	}
}

// SetMapFactsOut pairs live UnionFacts (FactMgr.cpp set_fact_out full FactVec).
func TestSetMapFactsOutPairsUnionWrite(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(13), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	fu := MakeFactUnion(uv, 0)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{fu}
	fm.SetMapFactsOut(7, fm.GlobalFacts)
	got := fm.GetMapUnionFactsOut(7)
	if FindRelatedUnion(got, uv) == nil || FindRelatedUnion(got, uv).LastWrittenFID != 0 {
		t.Fatalf("map_out must pair live UnionFacts, got %+v", got)
	}
}
