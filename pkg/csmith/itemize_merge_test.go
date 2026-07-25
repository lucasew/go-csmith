package csmith

import "testing"

// FactMgr.cpp:376–388 — array LHS assign MERGES (not renew).
// Itemized l_233[i]=&g must keep prior null on the collective subject.
// Seed-2 first_div@10107: after itemize of l_233[10], UP still has null/may-null
// (flipcoin p=0) while GO had pure g_127 — merge vs renew on array is the
// contract that keeps null in the lattice across element assigns.
func TestItemizedArrayAssignMergesNotRenews(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerTo(GetSimpleType(EShort)), false, false)
	elem := PointerTo(PointerTo(GetSimpleType(EShort))) // int16_t**
	coll := &ArrayVariable{
		Variable: Variable{Name: "l_233", Type: elem, IsArray: true},
		Sizes:    []int{10},
	}
	coll.AsArray = coll
	item := &ArrayVariable{
		Variable:   Variable{Name: "l_233", Type: elem, IsArray: true},
		Sizes:      []int{10},
		Collective: coll,
		Indices:    []string{"8"},
	}
	item.AsArray = item
	item.IndexExprs = []*Expression{{Term: TermConstant, Con: MakeInt(8), ExprType: GetIntType()}}

	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	// entry: primary pointer-0 → null only
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(&coll.Variable, NullPtr)}
	// RHS: &g_127 — ExpressionVariable(g) with desired type int16_t** (indir -1)
	rhs := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	if !fm.UpdateFactForAssign(&item.Variable, 0, rhs) {
		t.Fatalf("update err=%v", HasErrorSess(testAmbientSession))
	}
	fp := FindRelatedPointTo(fm.GlobalFacts, &coll.Variable)
	if fp == nil {
		t.Fatal("missing coll fact")
	}
	pts := []string{}
	for _, p := range fp.PointTo {
		if p != nil {
			pts = append(pts, p.Name)
		}
	}
	if !fp.IsNull() {
		t.Fatalf("itemized array assign must MERGE keep null, pts=%v factVarIsArray=%v",
			pts, fp.Var != nil && fp.Var.IsArray)
	}
	hasG := false
	for _, p := range fp.PointTo {
		if p == g || (p != nil && p.Name == "g_127") {
			hasG = true
		}
	}
	if !hasG {
		t.Fatalf("must also include g_127, pts=%v", pts)
	}
	ClearErrorSess(testAmbientSession)
}

// FactPointTo.cpp:448–450 + ExpressionVariable after itemize:
// OpportunisticValidate looks up get_collective(). When the collective fact is
// definitive non-null/non-dead, validate returns 1 without flipcoin. A lattice
// that incorrectly lost null/may-null will skip F p=0 that upstream still draws
// (seed-2 first_div@10107 after itemize size 10).
func TestOpportunisticValidateItemizedUsesCollectiveNullFlip(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	elem := PointerTo(GetIntType())
	coll := &ArrayVariable{
		Variable: Variable{Name: "l_233", Type: elem, IsArray: true},
		Sizes:    []int{10},
	}
	coll.AsArray = coll
	item := &ArrayVariable{
		Variable:   Variable{Name: "l_233", Type: elem, IsArray: true},
		Sizes:      []int{10},
		Collective: coll,
	}
	item.AsArray = item
	// may-null on collective (post_loop / merge lattice)
	facts := []*FactPointTo{MakeFactPointToSet(&coll.Variable, []*Variable{NullPtr, CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntType(), false, false)})}
	r := NewRngSess(testAmbientSession, 1)
	d0 := r.RandDepth()
	// need one more indir than var for validate to check null
	got := OpportunisticValidate(r, &item.Variable, GetIntType(), facts, 0, 0)
	if got != 0 {
		t.Fatalf("may-null + null_prob=0 must reject, got %d", got)
	}
	if r.RandDepth() != d0+1 {
		t.Fatalf("must still flipcoin(null_prob=0): depth %d → %d", d0, r.RandDepth())
	}
	// pure non-null: no flipcoin
	live := []*FactPointTo{MakeFactPointTo(&coll.Variable, CreateVariableScalarsSess(testAmbientSession, "g_u", GetIntType(), false, false))}
	r2 := NewRngSess(testAmbientSession, 1)
	d1 := r2.RandDepth()
	if OpportunisticValidate(r2, &item.Variable, GetIntType(), live, 0, 0) != 1 {
		t.Fatal("pure live must accept without null flip")
	}
	if r2.RandDepth() != d1 {
		t.Fatalf("pure live must not flipcoin: depth %d → %d", d1, r2.RandDepth())
	}
	ClearErrorSess(testAmbientSession)
}
