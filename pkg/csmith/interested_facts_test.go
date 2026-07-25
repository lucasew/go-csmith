package csmith

import "testing"

func TestAddInterestedFactsGates(t *testing.T) {
	defer ClearMetaFactsSess(testAmbientSession)
	// only point-to
	AddInterestedFacts(FactCategoryPointTo)
	if !MetaFactPointToEnabledSess(testAmbientSession) || MetaFactUnionEnabledSess(testAmbientSession) {
		t.Fatal("point only")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFact(p)
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		t.Fatal("want pt fact")
	}
	// union fact should not be created when disabled
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	fm.AddNewVarFact(uv)
	if FindRelatedUnion(fm.UnionFacts, uv) != nil {
		t.Fatal("union should be skipped")
	}

	// only union
	AddInterestedFacts(FactCategoryUnionWrite)
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.AddNewVarFact(p)
	if FindRelatedPointTo(fm2.GlobalFacts, p) != nil {
		t.Fatal("pt disabled")
	}
	// FactUnion.cpp:82 assert(rhs); Constant init → fid 0 (no invent TOP on nil init)
	uv.Init = MakeInt(0)
	fm2.AddNewVarFact(uv)
	if FindRelatedUnion(fm2.UnionFacts, uv) == nil {
		t.Fatal("want union fact")
	}

	// default both
	ClearMetaFactsSess(testAmbientSession)
	if !MetaFactPointToEnabledSess(testAmbientSession) || !MetaFactUnionEnabledSess(testAmbientSession) {
		t.Fatal("defaults")
	}
}

func TestGenerateFunctionsCallsAddInterested(t *testing.T) {
	defer ClearMetaFactsSess(testAmbientSession)
	// start with both off
	AddInterestedFacts(0)
	if MetaFactPointToEnabledSess(testAmbientSession) {
		t.Fatal("should be off")
	}
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	opts.InterestedFacts = DefaultInterestedFacts
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	g.GenerateFunctions()
	// GenerateFunctions writes meta flags on g.Sess (not ambient Process* bag).
	if !MetaFactPointToEnabledSess(g.Sess) || !MetaFactUnionEnabledSess(g.Sess) {
		t.Fatal("GenerateFunctions should re-enable default interests")
	}
	if len(g.Funcs.Funcs) < 1 {
		t.Fatal("no funcs")
	}
}
