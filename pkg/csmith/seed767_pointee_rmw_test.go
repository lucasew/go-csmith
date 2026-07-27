package csmith

import "testing"

func TestCompoundPointedRMWRecordsPointeeRead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	s := testAmbientSession
	intT := GetIntTypeSess(s)
	ptrT := PointerToSess(s, intT)
	pointee := CreateVariableScalarsSess(s, "g_pointee", intT, false, false)
	ptr := CreateVariableScalarsSess(s, "g_p", ptrT, false, false)

	fm := NewFactMgrSess(s, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(s, ptr, pointee)}

	cg := EmptyCGContext().WithSession(s)
	cg.FM = fm
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect()

	lhs := &Lhs{Var: ptr, Type: intT, CompoundAssign: true}
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("VisitFactsLhs compound *p failed sticky=", HasErrorSess(s))
	}
	if !eff.IsWrittenSess(s, pointee) {
		t.Fatal("want write pointee")
	}
	if !eff.IsReadSess(s, pointee) {
		t.Fatalf("want read pointee for compound RMW")
	}
}

// Volatile pointee (seed767 g_716.f1 is field of VOLATILE GLOBAL).
func TestCompoundPointedRMWVolatilePointeeRead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	s := testAmbientSession
	intT := GetIntTypeSess(s)
	ptrT := PointerToSess(s, intT)
	pointee := CreateVariableScalarsSess(s, "g_vol", intT, false, true) // volatile
	ptr := CreateVariableScalarsSess(s, "g_p", ptrT, false, false)

	fm := NewFactMgrSess(s, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(s, ptr, pointee)}

	cg := EmptyCGContext().WithSession(s)
	cg.FM = fm
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect()

	lhs := &Lhs{Var: ptr, Type: intT, CompoundAssign: true}
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("VisitFactsLhs compound *p volatile failed sticky=", HasErrorSess(s))
	}
	if !eff.IsWrittenSess(s, pointee) {
		t.Fatal("want write volatile pointee")
	}
	if !eff.IsReadSess(s, pointee) {
		var names []string
		for _, v := range eff.ReadVarsSess(s) {
			if v != nil {
				names = append(names, v.Name)
			}
		}
		t.Fatalf("want read volatile pointee; reads=%v", names)
	}
}
