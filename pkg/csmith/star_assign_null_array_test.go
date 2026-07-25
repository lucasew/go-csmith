package csmith

import "testing"

// FactPointTo.cpp:266–278 — *p = 0 when p may point to pointer-array l_233:
// merge_pointees(p, 1) yields l_233; rhs null transfers; merge keeps may-null.
// Upstream seed-2: UP_ABS_L233 null=1 lhs=l_236 indir=1 → e10107 lattice.
func TestStarAssignNullMergesIntoPointerArray(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	elem := PointerTo(PointerTo(GetSimpleType(EShort))) // int16_t**
	g := CreateVariableScalars("g_127", PointerTo(GetSimpleType(EShort)), false, false)
	arr := &ArrayVariable{
		Variable: Variable{Name: "l_233", Type: elem, IsArray: true},
		Sizes:    []int{10},
	}
	arr.AsArray = arr
	// l_236: int16_t *** pointing at l_233 collective
	p := CreateVariableScalars("l_236", PointerTo(elem), false, false)
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	fm.GlobalFacts = []*FactPointTo{
		MakeFactPointTo(&arr.Variable, g),
		MakeFactPointTo(p, &arr.Variable),
	}
	// *p = 0
	nullRHS := &Expression{
		Term:     TermConstant,
		Con:      &Constant{Type: elem, Value: "0"},
		ExprType: elem,
	}
	if !fm.UpdateFactForAssign(p, 1, nullRHS) {
		t.Fatalf("update *p=0 failed sticky=%v", HasErrorSess(testAmbientSession))
	}
	got := FindRelatedPointTo(fm.GlobalFacts, &arr.Variable)
	if got == nil {
		t.Fatal("missing l_233 fact after *p=0")
	}
	if !got.IsNull() {
		pts := []string{}
		for _, x := range got.PointTo {
			if x != nil {
				pts = append(pts, x.Name)
			}
		}
		t.Fatalf("*p=0 must merge null into l_233, pts=%v", pts)
	}
	hasG := false
	for _, x := range got.PointTo {
		if x == g {
			hasG = true
		}
	}
	if !hasG {
		t.Fatalf("must keep prior g_127 via merge, pts=%v", got.PointTo)
	}
	ClearErrorSess(testAmbientSession)
}
