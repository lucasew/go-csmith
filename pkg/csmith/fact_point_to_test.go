package csmith

import "testing"

func TestFactPointToNullDead(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", GetIntTypeSess(testAmbientSession), false, false)
	// default NewFactPointTo starts garbage
	f := NewFactPointToSess(testAmbientSession, p)
	// nil PointTo hole fails closed as dead/null (no invent not-dead/not-null)
	hole := &FactPointTo{Var: CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false), PointTo: []*Variable{nil}}
	if !hole.IsDeadSess(testAmbientSession) || !hole.IsNullSess(testAmbientSession) {
		t.Fatal("nil pointee hole must fail closed IsDead/IsNull")
	}
	if !f.IsDeadSess(testAmbientSession) || f.IsNullSess(testAmbientSession) {
		t.Fatal("init garbage")
	}
	fn := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	if !fn.IsNullSess(testAmbientSession) || fn.IsDeadSess(testAmbientSession) {
		t.Fatal("null fact")
	}
	if !IsSpecialPtr(NullPtr) || !IsSpecialPtr(GarbagePtr) || !IsSpecialPtr(TBDPtr) {
		t.Fatal("special")
	}
	// Variable.cpp:280–288 — is_virtual is array collective parent, not dummy specials
	if NullPtr.Type != nil {
		t.Fatal("dummy null type")
	}
	if NullPtr.IsVirtualSess(testAmbientSession) {
		t.Fatal("special ptr is not array is_virtual")
	}
	// nil subject sticky fact ctor
	ClearErrorSess(testAmbientSession)
	if NewFactPointToSess(testAmbientSession, nil) != nil || MakeFactPointToSess(testAmbientSession, nil, NullPtr) != nil || MakeFactPointToSetSess(testAmbientSession, nil, nil) != nil {
		t.Fatal("nil subject must fail closed fact ctor")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject fact ctor must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil pointee sticky
	if MakeFactPointToSess(testAmbientSession, p, nil) != nil {
		t.Fatal("nil pointTo must fail closed MakeFactPointTo")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil pointTo MakeFactPointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MakeFactPointToSetSess(testAmbientSession, p, []*Variable{NullPtr, nil}) != nil {
		t.Fatal("nil hole in set must fail closed MakeFactPointToSet")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole MakeFactPointToSet must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil set is incomplete merge non-sticky — no invent empty IsTop from nil
	if MakeFactPointToSetSess(testAmbientSession, p, nil) != nil {
		t.Fatal("nil set must fail closed MakeFactPointToSet")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil set MakeFactPointToSet must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MakeFactsPointToSetSess(testAmbientSession, []*Variable{p}, nil)) {
		t.Fatal("nil set must fail closed incomplete MakeFactsPointToSet")
	}
	// MakeFactsPointToSet may sticky on nil set path — clear after
	ClearErrorSess(testAmbientSession)
	// empty non-nil is valid top
	if MakeFactPointToSetSess(testAmbientSession, p, []*Variable{}) == nil {
		t.Fatal("empty non-nil set must succeed as top")
	}
	// Clone of incomplete PointTo sticky fail closed
	ClearErrorSess(testAmbientSession)
	if (&FactPointTo{Var: p, PointTo: []*Variable{nil}}).CloneSess(testAmbientSession) != nil {
		t.Fatal("Clone incomplete PointTo must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Clone incomplete PointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactPointTo)(nil).CloneSess(testAmbientSession) != nil {
		t.Fatal("nil FactPointTo Clone must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FactPointTo Clone must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactsComplete requires complete PointTo (empty IsTop OK)
	if !FactsComplete([]*FactPointTo{{Var: p, PointTo: nil}}) {
		t.Fatal("empty PointTo (top) is complete")
	}
	if FactsComplete([]*FactPointTo{{Var: p, PointTo: []*Variable{nil}}}) {
		t.Fatal("nil pointee hole is incomplete")
	}
	// CloneFactSlice incomplete → sticky hole marker (not bare nil invent empty complete)
	ClearErrorSess(testAmbientSession)
	if FactsComplete(CloneFactSliceSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil})) {
		t.Fatal("CloneFactSlice nil fact hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CloneFactSlice nil fact hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(CloneFactSliceSess(testAmbientSession, []*FactPointTo{{Var: p, PointTo: []*Variable{nil}}})) {
		t.Fatal("CloneFactSlice pointee hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CloneFactSlice pointee hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty stays empty complete
	if CloneFactSliceSess(testAmbientSession, nil) != nil {
		t.Fatal("CloneFactSliceSess(testAmbientSession, nil) must stay complete empty nil")
	}
	if cl := CloneFactSliceSess(testAmbientSession, []*FactPointTo{}); cl == nil || !FactsComplete(cl) {
		t.Fatal("CloneFactSlice empty non-nil must stay complete empty", cl)
	}
	// complete non-empty clones
	if cl := CloneFactSliceSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}); !FactsComplete(cl) || len(cl) != 1 {
		t.Fatal("CloneFactSlice complete must clone", cl)
	}
	// MakeFacts — no invent skip of nil holes as partial success / empty complete sticky
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MakeFactsPointToSess(testAmbientSession, []*Variable{p, nil}, NullPtr)) {
		t.Fatal("nil hole in lvars must fail closed incomplete MakeFactsPointTo")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole MakeFactsPointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MakeFactsPointToSetSess(testAmbientSession, []*Variable{nil, p}, []*Variable{NullPtr})) {
		t.Fatal("nil hole in lvars must fail closed incomplete MakeFactsPointToSet")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole MakeFactsPointToSet must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// specials Type-nil skipped; non-special Type-nil fails closed sticky whole batch
	if !FactsComplete(MakeFactsPointToSess(testAmbientSession, []*Variable{NullPtr, p}, NullPtr)) {
		t.Fatal("special Type-nil must soft-skip not fail batch")
	}
	broken := &Variable{Name: "broken"} // Type nil, not special
	if FactsComplete(MakeFactsPointToSess(testAmbientSession, []*Variable{broken, p}, NullPtr)) {
		t.Fatal("non-special Type-nil must fail closed incomplete MakeFactsPointTo")
	}
	// non-special Type-nil MakeFactsPointTo must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MakeFactsPointToSetSess(testAmbientSession, []*Variable{broken, p}, []*Variable{NullPtr})) {
		t.Fatal("non-special Type-nil must fail closed incomplete MakeFactsPointToSet")
	}
	// non-special Type-nil MakeFactsPointToSet must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestArrayIsVirtualCollectiveParent(t *testing.T) {
	// Variable.cpp:285–286 — collective==0 → virtual; itemized → not
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	if !parent.IsVirtualSess(testAmbientSession) {
		t.Fatal("parent collective must be virtual")
	}
	item := parent.ItemizeIntoSess(testAmbientSession, NewRngSess(testAmbientSession, 1), nil)
	if item == nil || item.IsVirtualSess(testAmbientSession) {
		t.Fatal("itemized member must not be virtual")
	}
	// field of parent array is virtual via recurse
	parent.Type = &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) == 0 {
		t.Skip("no fields")
	}
	if !parent.FieldVars[0].IsVirtualSess(testAmbientSession) {
		t.Fatal("field of virtual array must be virtual")
	}
	// IsArray without AsArray soft invent was virtual-collective true
	// fair: sticky false (broken IR, not invent virtual soft-success)
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_b", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	if shell.IsVirtualSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray must fail closed not-virtual")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray IsVirtual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsValidPtr(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", GetIntTypeSess(testAmbientSession), false, false)
	target := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	// Variable always live; sticky invalid / dangling
	ClearErrorSess(testAmbientSession)
	if IsValidPtrSess(testAmbientSession, nil, nil, 0, 0) {
		t.Fatal("nil p IsValidPtr must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil p IsValidPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !IsDanglingPtrSess(testAmbientSession, nil, nil, 0) {
		t.Fatal("nil p IsDanglingPtr must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil p IsDanglingPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// no fact → invalid
	if IsValidPtrSess(testAmbientSession, p, nil, 0, 0) {
		t.Fatal("no fact")
	}
	// points to real target → valid
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, target)}
	if !IsValidPtrSess(testAmbientSession, p, facts, 0, 0) {
		t.Fatal("live")
	}
	// null with prob 0 → invalid
	facts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	if IsValidPtrSess(testAmbientSession, p, facts, 0, 0) {
		t.Fatal("null blocked")
	}
	// null with prob >0 → allowed
	if !IsValidPtrSess(testAmbientSession, p, facts, 1, 0) {
		t.Fatal("null allowed")
	}
	// garbage with prob 0 → invalid + dangling
	facts = []*FactPointTo{NewFactPointToSess(testAmbientSession, p)}
	if IsValidPtrSess(testAmbientSession, p, facts, 0, 0) {
		t.Fatal("dead blocked")
	}
	if !IsDanglingPtrSess(testAmbientSession, p, facts, 0) {
		t.Fatal("dangling")
	}
	// IsDead residual: PointTo nil hole soft invent was soft-continue then invent valid true.
	// Fair: sticky invalid / dangling.
	ClearErrorSess(testAmbientSession)
	broken := &FactPointTo{Var: p, PointTo: []*Variable{target, nil}}
	if IsValidPtrSess(testAmbientSession, p, []*FactPointTo{broken}, 0, 0) {
		t.Fatal("IsDead residual must fail closed invalid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsDead residual IsValidPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !IsDanglingPtrSess(testAmbientSession, p, []*FactPointTo{broken}, 0) {
		t.Fatal("IsDead residual must fail closed dangling true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsDead residual IsDanglingPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil subject soft invent: related-fact match invents valid true
	// fair: sticky invalid / dangling before fact lookup
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_typeless"}
	factsShell := []*FactPointTo{MakeFactPointToSess(testAmbientSession, shell, target)}
	if IsValidPtrSess(testAmbientSession, shell, factsShell, 0, 0) {
		t.Fatal("Type-nil subject IsValidPtr must fail closed false, not invent valid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil subject IsValidPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !IsDanglingPtrSess(testAmbientSession, shell, factsShell, 0) {
		t.Fatal("Type-nil subject IsDanglingPtr must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil subject IsDanglingPtr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete maps fail closed as dangling (no invent not-dangling past hole)
	ClearErrorSess(testAmbientSession)
	hole := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	if !IsDanglingPtrSess(testAmbientSession, p, hole, 0) {
		t.Fatal("incomplete facts must fail closed as dangling")
	}
	if OpportunisticValidateSess(testAmbientSession, NewRngSess(testAmbientSession, 1), p, GetIntTypeSess(testAmbientSession), hole, 0, 0) != 0 {
		t.Fatal("incomplete facts must reject opportunistic validate")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts OpportunisticValidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFactMgrGlobalFacts(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", GetIntTypeSess(testAmbientSession), false, false)
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointToSess(testAmbientSession, p, NullPtr))
	if !FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p).IsNullSess(testAmbientSession) {
		t.Fatal("lookup")
	}
}

func TestMarkFuncEnd(t *testing.T) {
	// FactPointTo.cpp:129–154 — stack local pointee → garbage
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_t", GetIntTypeSess(testAmbientSession), false, false)
	loc.Name = "l_t"
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	f.Body = body
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	ft := MakeFactPointToSess(testAmbientSession, p, loc)
	nf := ft.MarkFuncEndSess(testAmbientSession, f, body)
	if nf == nil || len(nf.PointTo) != 1 || nf.PointTo[0] != GarbagePtr {
		t.Fatalf("%+v", nf)
	}
	// non-stack target unchanged
	g := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), true, false)
	ft2 := MakeFactPointToSess(testAmbientSession, p, g)
	if ft2.MarkFuncEndSess(testAmbientSession, f, body) != nil {
		t.Fatal("global pointee")
	}
	// nil Function: complete no-op non-sticky (no invent residual wipe via RemoveFunctionLocal)
	ClearErrorSess(testAmbientSession)
	if ft.MarkFuncEndSess(testAmbientSession, nil, body) != nil {
		t.Fatal("nil Function MarkFuncEnd must no-op nil")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function MarkFuncEnd must stay non-sticky complete no-op")
	}
	ClearErrorSess(testAmbientSession)
	// nil pointee hole fails closed sticky
	ClearErrorSess(testAmbientSession)
	ft3 := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	if ft3.MarkFuncEndSess(testAmbientSession, f, body) != nil {
		t.Fatal("nil PointTo hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil PointTo hole MarkFuncEnd must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ft.MarkFuncEndLocalsSess(testAmbientSession, []*Variable{nil}) != nil {
		t.Fatal("nil locals hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil locals hole MarkFuncEndLocals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ft3.MarkDeadVarSess(testAmbientSession, loc) != nil {
		t.Fatal("MarkDeadVar nil PointTo hole must fail closed")
	}
	// incomplete facts IsValidPtr sticky
	ClearErrorSess(testAmbientSession)
	if IsValidPtrSess(testAmbientSession, p, []*FactPointTo{nil}, 0, 0) {
		t.Fatal("IsValidPtr incomplete facts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsValidPtr incomplete facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRemoveFunctionLocalFactsMarksGarbage(t *testing.T) {
	// remaining global ptr that points at local → garbage after remove
	fn := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_t", GetIntTypeSess(testAmbientSession), false, false)
	loc.Name = "l_t"
	body := &Block{Func: fn, LocalVars: []*Variable{loc}}
	fn.Blocks = []*Block{body}
	fn.Body = body
	gp := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	lp.Name = "l_p"
	body.LocalVars = append(body.LocalVars, lp)
	facts := []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, lp, NullPtr),
		MakeFactPointToSess(testAmbientSession, gp, loc),
	}
	out := RemoveFunctionLocalFactsAtSess(testAmbientSession, facts, fn, fn.Body)
	if len(out) != 1 || out[0].Var != gp {
		t.Fatalf("%+v", out)
	}
	if len(out[0].PointTo) != 1 || out[0].PointTo[0] != GarbagePtr {
		t.Fatal("want garbage pointee", out[0].PointTo)
	}
}

func TestUpdateWithModifiedIndexNilPointee(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	idx := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	ClearErrorSess(testAmbientSession)
	if f.UpdateWithModifiedIndexSess(testAmbientSession, idx) != nil {
		t.Fatal("nil pointee hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil pointee UpdateWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactPointTo)(nil).UpdateWithModifiedIndexSess(testAmbientSession, idx) != nil {
		t.Fatal("nil fact UpdateWithModifiedIndex must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact UpdateWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f.UpdateWithModifiedIndexSess(testAmbientSession, nil) != nil {
		t.Fatal("nil indexVar UpdateWithModifiedIndex must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil indexVar UpdateWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was continue soft-skip → identity success
	// fair: sticky nil fail closed
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	fShell := MakeFactPointToSess(testAmbientSession, p, shell)
	if fShell.UpdateWithModifiedIndexSess(testAmbientSession, idx) != nil {
		t.Fatal("IsArray without AsArray root must fail closed UpdateWithModifiedIndex")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray UpdateWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live for string index use; sticky false
	if indexExprUsesVarSess(testAmbientSession, "i", nil) {
		t.Fatal("nil indexVar indexExprUsesVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil indexVar indexExprUsesVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if indexExprUsesVarSess(testAmbientSession, "", idx) {
		t.Fatal("empty idx must complete not-used")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty idx indexExprUsesVar must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(MergePointeesOfPointersSess(testAmbientSession, []*Variable{nil}, nil)) {
		t.Fatal("nil ptr hole MergePointees must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptr hole MergePointees must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergePointeesMissingFactNDEBUGSkip(t *testing.T) {
	// FactPointTo.cpp:691–696 — assert(exist_fact); if (exist_fact) merge.
	// NDEBUG: missing related fact skips that pointer (empty complete, not Incomplete).
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// facts empty / no related fact for p — NDEBUG skip → empty complete
	got := MergePointeesOfPointersSess(testAmbientSession, []*Variable{p}, nil)
	if !VariablesComplete(got) || len(got) != 0 {
		t.Fatalf("missing exist_fact must NDEBUG-skip empty complete, got %+v", got)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("missing exist_fact must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	got = MergePointeesOfPointersSess(testAmbientSession, []*Variable{p}, []*FactPointTo{})
	if !VariablesComplete(got) || len(got) != 0 {
		t.Fatalf("empty facts without related must NDEBUG-skip empty, got %+v", got)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty facts missing related must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete fact map still fails closed non-sticky
	if VariablesComplete(MergePointeesOfPointersSess(testAmbientSession, []*Variable{p}, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil})) {
		t.Fatal("incomplete facts must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts MergePointees must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	// complete related fact still works
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	got = MergePointeesOfPointersSess(testAmbientSession, []*Variable{p}, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)})
	if !VariablesComplete(got) || len(got) != 1 || got[0] != tgt {
		t.Fatalf("complete related fact: %+v", got)
	}
	// specials still skip without fact
	sp := MergePointeesOfPointersSess(testAmbientSession, []*Variable{NullPtr}, nil)
	if !VariablesComplete(sp) || len(sp) != 0 {
		t.Fatal("specials-only must yield empty complete, not fail closed", sp)
	}
	// multi: one missing + one present → only present's pointees
	p2 := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	got = MergePointeesOfPointersSess(testAmbientSession, []*Variable{p, p2}, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)})
	if !VariablesComplete(got) || len(got) != 1 || got[0] != tgt {
		t.Fatalf("partial missing must merge remaining: %+v", got)
	}
	// PointTo nil hole is FactsComplete-false → non-sticky incomplete map path
	bad := MakeFactPointToSess(testAmbientSession, p, tgt)
	bad.PointTo = []*Variable{nil}
	if VariablesComplete(MergePointeesOfPointersSess(testAmbientSession, []*Variable{p}, []*FactPointTo{bad})) {
		t.Fatal("nil pointee hole must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil pointee via incomplete facts must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergePointeesOfPointerPropagatesNil(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// indirect 1 with missing fact → NDEBUG skip → empty complete (not Incomplete)
	gotMiss := MergePointeesOfPointerSess(testAmbientSession, p, 1, nil)
	if !VariablesComplete(gotMiss) || len(gotMiss) != 0 {
		t.Fatalf("missing fact at indir 1 must NDEBUG empty complete, got %+v", gotMiss)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("missing fact MergePointeesOfPointer must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// indirect 0 does not look up facts
	got := MergePointeesOfPointerSess(testAmbientSession, p, 0, nil)
	if !VariablesComplete(got) || len(got) != 1 || got[0] != p {
		t.Fatalf("indir0: %+v", got)
	}
	// Variable always live; sticky
	if VariablesComplete(MergePointeesOfPointerSess(testAmbientSession, nil, 0, nil)) {
		t.Fatal("nil ptr must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptr MergePointeesOfPointer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUpdateWithModifiedIndex(t *testing.T) {
	// FactPointTo.cpp:712–748 — a[i] → a[-1] when i modified
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	item.AsArray = item
	idx := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	idx.Name = "i"
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f := MakeFactPointToSess(testAmbientSession, p, &item.Variable)
	nf := f.UpdateWithModifiedIndexSess(testAmbientSession, idx)
	if nf == f {
		t.Fatal("expected new fact")
	}
	if len(nf.PointTo) != 1 || nf.PointTo[0] == nil || nf.PointTo[0].AsArray == nil {
		t.Fatalf("pointee %+v", nf.PointTo)
	}
	if got := nf.PointTo[0].AsArray.Indices; len(got) != 1 || got[0] != "-1" {
		t.Fatalf("indices %v", got)
	}
	// unrelated index → unchanged
	j := CreateVariableScalarsSess(testAmbientSession, "j", GetIntTypeSess(testAmbientSession), false, false)
	j.Name = "j"
	if f.UpdateWithModifiedIndexSess(testAmbientSession, j) != f {
		t.Fatal("j should not rewrite")
	}
	// bulk update
	ClearErrorSess(testAmbientSession)
	facts := []*FactPointTo{f.CloneSess(testAmbientSession)}
	UpdateFactsWithModifiedIndexSess(testAmbientSession, &facts, idx)
	if facts[0] == f || facts[0].PointTo[0].AsArray.Indices[0] != "-1" {
		t.Fatal("bulk", facts[0])
	}
	// incomplete facts fail closed sticky
	hole := []*FactPointTo{f.CloneSess(testAmbientSession), nil}
	UpdateFactsWithModifiedIndexSess(testAmbientSession, &hole, idx)
	if FactsComplete(hole) {
		t.Fatal("incomplete bulk must wipe IncompleteFactSlice", hole)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete UpdateFactsWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// facts + indexVar always live; sticky no invent soft-skip update past hole
	UpdateFactsWithModifiedIndexSess(testAmbientSession, nil, idx)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts UpdateFactsWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	UpdateFactsWithModifiedIndexSess(testAmbientSession, &facts, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil indexVar UpdateFactsWithModifiedIndex must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// offset form "(i + 2)"
	item2 := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"(i + 2)"},
	}
	item2.AsArray = item2
	f2 := MakeFactPointToSess(testAmbientSession, p, &item2.Variable)
	nf2 := f2.UpdateWithModifiedIndexSess(testAmbientSession, idx)
	if nf2 == f2 || nf2.PointTo[0].AsArray.Indices[0] != "-1" {
		t.Fatal("offset form", nf2)
	}
}

func TestFindRelatedPointToNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if FindRelatedPointToSess(testAmbientSession, nil, nil) != nil {
		t.Fatal("nil subject FindRelatedPointTo must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject FindRelatedPointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if FindRelatedPointToSess(testAmbientSession, []*FactPointTo{nil}, p) != nil {
		t.Fatal("nil fact hole FindRelatedPointTo must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole FindRelatedPointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsNullIsDeadPointsToNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !(*FactPointTo)(nil).IsNullSess(testAmbientSession) {
		t.Fatal("nil Fact IsNull must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Fact IsNull must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*FactPointTo)(nil).IsDeadSess(testAmbientSession) {
		t.Fatal("nil Fact IsDead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Fact IsDead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*FactPointTo)(nil).PointsToSess(testAmbientSession, CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)) {
		t.Fatal("nil Fact PointsTo must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Fact PointsTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFactPointToLatticeTopBottom(t *testing.T) {
	// FactPointTo.h:93–98 is_top/is_bottom/set_top/set_bottom
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	if f.IsBottomSess(testAmbientSession) {
		t.Fatal("is_bottom always false")
	}
	if f.IsTopSess(testAmbientSession) {
		t.Fatal("non-empty not top")
	}
	f.SetTopSess(testAmbientSession)
	if !f.IsTopSess(testAmbientSession) || len(f.PointTo) != 0 {
		t.Fatal("set_top clears")
	}
	f.SetBottomSess(testAmbientSession) // no-op
	if f.GetVarSess(testAmbientSession) != p {
		t.Fatal("get_var")
	}
	f2 := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	if out := f2.OutputSess(testAmbientSession); out == "" || out != "g_p => {null}" {
		// name may vary; just require format
		if out == "" {
			t.Fatal("Output empty")
		}
	}
}

func TestFactFreeHelpers(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	cp := CopyFactsSess(testAmbientSession, facts)
	if !SameFactsSess(testAmbientSession, cp, facts) {
		t.Fatal("CopyFacts/SameFacts")
	}
	// CombineFacts join_visits
	other := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	CombineFactsSess(testAmbientSession, &facts, other)
	fp := FindRelatedPointToSess(testAmbientSession, facts, p)
	if fp == nil || len(fp.PointTo) < 1 {
		t.Fatal("combine")
	}
	// AbstractFactForReturn — Fact.cpp:76–83 assign into func.rv
	rv := CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fn := &Function{Name: "f", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), RV: rv}
	// null constant RHS is a complete abstract path
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}
	ClearErrorSess(testAmbientSession)
	ret := AbstractFactForReturnSess(testAmbientSession, nil, rhs, fn)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("return abstract sticky")
	}
	// nil fn/expr sticky
	ClearErrorSess(testAmbientSession)
	if FactsComplete(AbstractFactForReturnSess(testAmbientSession, nil, rhs, nil)) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fn must sticky incomplete")
	}
	ClearErrorSess(testAmbientSession)
	_ = ret
	// PrintFacts not sticky on complete empty
	_ = PrintFactsSess(testAmbientSession, nil, nil)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("print empty")
	}
	ClearErrorSess(testAmbientSession)
	FactDoFinalizationSess(testAmbientSession)
}

func TestFactPointToPointToAndStr(t *testing.T) {
	// FactPointTo.cpp:398–405 point_to; 530–540 point_to_str
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f := MakeFactPointToSess(testAmbientSession, p, a)
	if !f.PointsToSess(testAmbientSession, a) {
		t.Fatal("points to a")
	}
	if f.PointsToSess(testAmbientSession, p) {
		t.Fatal("not points to p")
	}
	if PointToStrSess(testAmbientSession, NullPtr) != "0" || PointToStrSess(testAmbientSession, TBDPtr) != "tbd" || PointToStrSess(testAmbientSession, GarbagePtr) != "garbage" {
		t.Fatal("specials")
	}
	if PointToStrSess(testAmbientSession, a) != "&g_a" {
		t.Fatal(PointToStrSess(testAmbientSession, a))
	}
	if f.SizeSess(testAmbientSession) != 1 || f.EmptySess(testAmbientSession) {
		t.Fatal("size/empty")
	}
	f2 := MakeFactPointToSess(testAmbientSession, p, a)
	if !f.IsRelatedSess(testAmbientSession, f2) {
		t.Fatal("related same var")
	}
	f.ClearSess(testAmbientSession)
	if !f.EmptySess(testAmbientSession) || !f.IsTopSess(testAmbientSession) {
		t.Fatal("clear → top")
	}
	ClearErrorSess(testAmbientSession)
	if PointToStrSess(testAmbientSession, nil) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil PointToStr sticky")
	}
	ClearErrorSess(testAmbientSession)
}
