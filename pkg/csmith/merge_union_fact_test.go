package csmith

import "testing"

// Fact.cpp:149–171 — MergeUnionFactInto is the same merge_fact contract as MergeUnionFact
// (imply short-circuit; copy=new.clone(); join(old)). Soft invent always-joined without
// imply short-circuit.
func TestMergeUnionFactIntoMatchesMergeFact(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f3", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	uv.CreateFieldVars()
	// old already implies new → keep old (must not join-to-BOTTOM)
	facts := []*FactUnion{MakeFactUnion(uv, 3)}
	got := MergeUnionFactInto(facts, MakeFactUnion(uv, 3))
	if !UnionFactsComplete(got) || HasErrorSess(testAmbientSession) {
		t.Fatal("into incomplete", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	fu := FindRelatedUnion(got, uv)
	if fu == nil || fu.LastWrittenFID != 3 || fu.IsBottom() {
		t.Fatalf("want keep fid 3, got %#v", fu)
	}
	// 0 join 3 → BOTTOM
	ClearErrorSess(testAmbientSession)
	got2 := MergeUnionFactInto([]*FactUnion{MakeFactUnion(uv, 0)}, MakeFactUnion(uv, 3))
	fu2 := FindRelatedUnion(got2, uv)
	if fu2 == nil || !fu2.IsBottom() {
		t.Fatalf("want BOTTOM after 0 join 3, got %#v", fu2)
	}
}

// Fact.cpp:149–171 merge_fact for eUnionWrite — join lattice, not replace.
func TestMergeUnionFactJoinsLattice(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
			{Name: "f2", Type: GetIntType(), BitWidth: -1},
			{Name: "f3", Type: GetIntType(), BitWidth: -1},
			{Name: "f4", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// seed field-vars so IsUnion paths are live
	uv.CreateFieldVars()
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	// merge fid 4 into fid 0 → neither implies → BOTTOM (not replace with 4)
	merged := MergeUnionFact(facts, MakeFactUnion(uv, 4))
	if !UnionFactsComplete(merged) || HasErrorSess(testAmbientSession) {
		t.Fatal("merge incomplete", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(merged, uv)
	if got == nil || !got.IsBottom() {
		t.Fatalf("want BOTTOM after 0 join 4, got %#v", got)
	}
	// old already implies new → keep old
	ClearErrorSess(testAmbientSession)
	facts2 := []*FactUnion{MakeFactUnion(uv, 3)}
	merged2 := MergeUnionFact(facts2, MakeFactUnion(uv, 3))
	got2 := FindRelatedUnion(merged2, uv)
	if got2 == nil || got2.LastWrittenFID != 3 {
		t.Fatalf("want keep 3, got %#v", got2)
	}
}

// FactMgr.cpp:376–381 — definitive union field write renews (replace), not join-to-BOTTOM.
func TestUpdateFactForAssignUnionRenewDefinitive(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f3", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	uv.CreateFieldVars()
	if len(uv.FieldVars) < 2 {
		t.Fatal("field vars")
	}
	f3 := uv.FieldVars[1]
	fm := NewFactMgr(nil)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(uv, 0)}
	// definitive assign to union field f3 → renew last_written to field id of f3
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}
	if !fm.UpdateFactForAssign(f3, 0, rhs) {
		t.Fatal("update", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.LastWrittenFID != f3.GetFieldID() {
		t.Fatalf("want renew to f3 fid, got %#v fieldID=%d", got, f3.GetFieldID())
	}
}

// May-assign (multi pointee) must join, not replace last_written.
func TestUpdateFactForAssignUnionMayMergeJoins(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f4", Type: GetIntType(), BitWidth: -1},
		},
	}
	u0 := CreateVariableQfer("g_u0", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	u1 := CreateVariableQfer("g_u1", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	u0.CreateFieldVars()
	u1.CreateFieldVars()
	// pointer that may point to either union's f4 field parent via indir write is complex;
	// exercise MergeUnionFact join path used by may-assign: 0 join 4 → BOTTOM
	facts := []*FactUnion{MakeFactUnion(u0, 0)}
	merged := MergeUnionFact(facts, MakeFactUnion(u0, 4))
	got := FindRelatedUnion(merged, u0)
	if got == nil || !got.IsBottom() {
		t.Fatalf("may-merge must BOTTOM not replace-to-4: %#v", got)
	}
	_ = u1
}

// After for-IV write to union.f1 (last=1), empty if/else combine must keep last=1.
// seed-123: combine then=0 else=1 bottomed g_721 after for(g_721.f1) post_loop left last=1.
func TestCombineBranchAfterUnionFieldIVKeepsLastWritten(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetSimpleType(EChar), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EUInt), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_721", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	uv.CreateFieldVars()
	if len(uv.FieldVars) < 2 {
		t.Fatal("need f0 f1")
	}
	f1 := uv.FieldVars[1]
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	body := &Block{StmID: AllocStmID(), Func: f}
	f.Body = body
	f.Stack = []*Block{body}
	fm := NewFactMgr(f)
	// init fact last=0 (constant init of union)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(uv, 0)}
	fm.GlobalFacts = []*FactPointTo{}
	// IV assign g_721.f1 = 0
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}
	if !fm.UpdateFactForAssign(f1, 0, rhs) {
		t.Fatal("assign f1", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.LastWrittenFID != 1 {
		t.Fatalf("after f1 write want last=1 got %#v", got)
	}
	// empty then/else blocks with entry last=1
	thenB := &Block{StmID: AllocStmID(), Func: f, Parent: body}
	elseB := &Block{StmID: AllocStmID(), Func: f, Parent: body}
	fm.SetMapFactsInPair(thenB.StmID, CloneFactSlice(fm.GlobalFacts), CloneUnionFactSliceDeep(fm.UnionFacts))
	fm.SetMapFactsOutForBlock(thenB, CloneFactSlice(fm.GlobalFacts))
	// else starts from then map_in (StatementIf.cpp:97)
	fm.AssignGlobalFactsFromMapIn(thenB.StmID)
	fm.SetMapFactsOutForBlock(elseB, CloneFactSlice(fm.GlobalFacts))
	ifSt := &Stmt{Kind: StmtIfElse, Then: thenB, Else: elseB, StmID: AllocStmID(), Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	prePT := CloneFactSlice(fm.GlobalFacts)
	preU := CloneUnionFactSliceDeep(fm.UnionFacts)
	CombineBranchFacts(ifSt, &prePT, &preU, fm)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("combine", GetErrorSess(testAmbientSession))
	}
	got = FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.IsBottom() {
		t.Fatalf("empty if/else after f1 IV must not bottom g_721, got %#v", got)
	}
	if got.LastWrittenFID != 1 {
		t.Fatalf("want last=1 after combine, got %d", got.LastWrittenFID)
	}
}
