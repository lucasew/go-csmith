package csmith

import "testing"

// FactMgr.cpp set_fact_in / StatementFor.cpp:355 — map_facts_in stores full FactVec
// (ePointTo + eUnionWrite). post_loop assigns global_facts = map_facts_in[&body]
// for both partitions so IsNonreadableField sees body-entry last-writes.
func TestMapFactsInPairsUnionWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
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
		t.Fatal("MakeFactUnion entry", HasErrorSess(testAmbientSession))
	}
	bodyU := MakeFactUnion(uv, FactUnionBottom)
	if bodyU == nil {
		t.Fatal("MakeFactUnion body", HasErrorSess(testAmbientSession))
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
		t.Fatal("assign from map_in incomplete", fm.GlobalFacts, fm.UnionFacts, HasErrorSess(testAmbientSession))
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(9), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	live := []*FactUnion{MakeFactUnion(uv, 0)}
	if !mergeJumpUnionFacts(&live, []*FactUnion{}) {
		t.Fatal("merge failed", HasErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(live, uv)
	if got == nil || got.LastWrittenFID != FactUnionBottom {
		t.Fatalf("want BOTTOM after jump-missing, got %+v", live)
	}
}

// StatementGoto.cpp:167 + FactMgr.cpp:569–588 — forward goto merge_jump_facts is
// full FactVec. Soft invent merged PT-only then never rewound UnionFacts from
// map_facts_out[dest] (seed-104: else-start goto left g_111 last=0 vs UP BOTTOM).
// Contract: dest map_in last=0 + empty goto_out unions → BOTTOM after merge_jump;
// AssignGlobalFactsFromMapOut installs that lattice into live UnionFacts.
func TestForwardGotoMergeJumpUnionBottomAndMapOutInstall(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	ut := &Type{isUnion: true, StructName: "U_goto", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_goto", ut, false, false)
	parent.CreateFieldVars()
	// dest map_facts_in: last=0 (readable f0)
	entryU := MakeFactUnion(parent, 0)
	if entryU == nil {
		t.Fatal("entry")
	}
	// goto_out lacks g_u → merge_jump synthesizes BOTTOM (FactMgr.cpp:580–582)
	stmInU := []*FactUnion{entryU.Clone()}
	if stmInU[0] == nil {
		t.Fatal("clone")
	}
	gotoOutU := []*FactUnion{} // missing subject
	if !mergeJumpUnionFacts(&stmInU, gotoOutU) {
		t.Fatal("merge_jump union", GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(stmInU, parent)
	if got == nil || !got.IsBottom() {
		t.Fatalf("dest last=0 ⊕ missing goto_out must BOTTOM, got %#v", got)
	}
	// set_fact_out pairs then AssignGlobalFactsFromMapOut rewinds live
	fm := NewFactMgr(&Function{Name: "f", ReturnType: GetIntType()})
	// live still last=0 (as before fix)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0)}
	fm.GlobalFacts = []*FactPointTo{}
	destID := 42
	fm.SetMapFactsOutPair(destID, []*FactPointTo{}, stmInU)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	fm.AssignGlobalFactsFromMapOut(destID)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	live := FindRelatedUnion(fm.UnionFacts, parent)
	if live == nil || !live.IsBottom() {
		t.Fatalf("AssignGlobalFactsFromMapOut must install BOTTOM, got %#v", fm.UnionFacts)
	}
	// f0 nonreadable under BOTTOM (ChooseOKVar filter)
	if len(parent.FieldVars) < 1 {
		t.Fatal("fields")
	}
	if !IsNonreadableField(parent.FieldVars[0], fm.UnionFacts) {
		t.Fatal("BOTTOM must make f0 nonreadable for ChooseOKVar")
	}
	ClearErrorSess(testAmbientSession)
}

// FactMgr.cpp:450–482 eUnionWrite half of update_facts_for_dest.
func TestUpdateUnionFactsForDestCopiesNonRVOOSDrop(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U_dest", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	g := CreateVariableScalars("g_u_dest", ut, false, false)
	g.CreateFieldVars()
	fn := &Function{Name: "f", ReturnType: GetIntType()}
	body := &Block{Func: fn}
	fn.Body = body
	// local union is OOS at dest outside its block — use global only
	in := []*FactUnion{MakeFactUnion(g, 0)}
	var out []*FactUnion
	UpdateUnionFactsForDest(in, &out, fn, body)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(out, g)
	if got == nil || got.LastWrittenFID != 0 {
		t.Fatalf("global union must copy to dest out, got %#v", out)
	}
	// nil func fail closed
	var out2 []*FactUnion
	UpdateUnionFactsForDest(in, &out2, nil, body)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil func must sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestSetMapFactsOutGotoDropsOOSUnionWrite — FactMgr.cpp:263–266.
// set_fact_out(goto) runs update_facts_for_dest on the full FactVec so
// eUnionWrite subjects OOS at dest are dropped. Soft invent filtered only
// ePointTo then stored raw live UnionFacts → map_facts_out[goto] kept
// then-arm local union last-write after jump to sibling else (seed
// 10613516242873274820: choose_visible nOk 36 vs UP 35; if (l_1156) vs
// if (l_670.f0) because OOS l_1372.f0 stayed readable).
func TestSetMapFactsOutGotoDropsOOSUnionWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U_goto", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	g := CreateVariableScalars("g_keep", ut, false, false)
	g.CreateFieldVars()
	loc := CreateVariableScalars("l_arm", ut, false, false)
	loc.CreateFieldVars()
	fn := &Function{Name: "f_goto_u", ReturnType: GetIntType()}
	// then-arm holds local; dest is body (sibling path) where local is OOS
	body := &Block{Func: fn, StmID: AllocStmID()}
	thenArm := &Block{Func: fn, Parent: body, StmID: AllocStmID(), LocalVars: []*Variable{loc}}
	fn.Body = body
	fn.Blocks = []*Block{body, thenArm}
	// Live unions include both global and then-local
	fm := NewFactMgr(fn)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(g, 0), MakeFactUnion(loc, 0)}
	fm.GlobalFacts = []*FactPointTo{}
	// Goto in then-arm jumping to a dest in body (local OOS at dest)
	sg := &Stmt{
		Kind: StmtGoto, StmID: AllocStmID(),
		GotoDestStmID: AllocStmID(), GotoDestParent: body,
	}
	// parent of goto is thenArm for stack/OOS
	fm.SetMapFactsOutForStmtDest(sg, []*FactPointTo{}, thenArm, body)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("set_fact_out goto: %v", GetErrorSess(testAmbientSession))
	}
	outU := fm.GetMapUnionFactsOut(sg.StmID)
	if !UnionFactsComplete(outU) {
		t.Fatal("map_union_out must be complete")
	}
	if FindRelatedUnion(outU, loc) != nil {
		t.Fatal("then-arm local eUnionWrite must be OOS-dropped at dest outside arm")
	}
	if FindRelatedUnion(outU, g) == nil {
		t.Fatal("global eUnionWrite must remain at dest")
	}
	// Field of OOS local must be nonreadable without fact
	if len(loc.FieldVars) == 0 {
		t.Fatal("need field")
	}
	if !IsNonreadableField(loc.FieldVars[0], outU) {
		t.Fatal("OOS local field must be nonreadable after goto map_out filter")
	}
	ClearErrorSess(testAmbientSession)
}

// SetMapFactsOut pairs live UnionFacts (FactMgr.cpp set_fact_out full FactVec).
func TestSetMapFactsOutPairsUnionWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
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
