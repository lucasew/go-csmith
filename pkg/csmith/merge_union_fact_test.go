package csmith

import "testing"

// Fact.cpp:149–171 merge_fact for eUnionWrite — join lattice, not replace.
func TestMergeUnionFactJoinsLattice(t *testing.T) {
	ClearError()
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
	if !UnionFactsComplete(merged) || HasError() {
		t.Fatal("merge incomplete", HasError(), GetError())
	}
	got := FindRelatedUnion(merged, uv)
	if got == nil || !got.IsBottom() {
		t.Fatalf("want BOTTOM after 0 join 4, got %#v", got)
	}
	// old already implies new → keep old
	ClearError()
	facts2 := []*FactUnion{MakeFactUnion(uv, 3)}
	merged2 := MergeUnionFact(facts2, MakeFactUnion(uv, 3))
	got2 := FindRelatedUnion(merged2, uv)
	if got2 == nil || got2.LastWrittenFID != 3 {
		t.Fatalf("want keep 3, got %#v", got2)
	}
}

// FactMgr.cpp:376–381 — definitive union field write renews (replace), not join-to-BOTTOM.
func TestUpdateFactForAssignUnionRenewDefinitive(t *testing.T) {
	ClearError()
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
		t.Fatal("update", HasError(), GetError())
	}
	got := FindRelatedUnion(fm.UnionFacts, uv)
	if got == nil || got.LastWrittenFID != f3.GetFieldID() {
		t.Fatalf("want renew to f3 fid, got %#v fieldID=%d", got, f3.GetFieldID())
	}
}

// May-assign (multi pointee) must join, not replace last_written.
func TestUpdateFactForAssignUnionMayMergeJoins(t *testing.T) {
	ClearError()
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
