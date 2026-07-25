package csmith

import "testing"

func TestEmptyEffectSEFree(t *testing.T) {
	// Effect::Effect default pure + side_effect_free
	e := EmptyEffect()
	if !e.IsPure() || !e.IsSideEffectFree() {
		t.Fatal("empty effect must be pure and SE-free")
	}
	if WithSideEffects().IsSideEffectFree() {
		t.Fatal("WithSideEffects must not be SE-free")
	}
}

func TestSiblingUnionFieldGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent no-sibling soft-skip past array shell.
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	// restrictive true on residual GetCollective
	if !EmptyEffect().SiblingUnionFieldIsRead(shell) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsRead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !EmptyEffect().SiblingUnionFieldIsWritten(shell) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsWritten must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestWriteVarSetResidualSticky(t *testing.T) {
	// WriteVar residual soft invent was invent soft-continue later writes past nil hole mid set.
	ClearErrorSess(testAmbientSession)
	// incomplete vars list sticky IncompleteEffect
	out := EmptyEffect().WriteVarSet([]*Variable{nil})
	if EffectComplete(out) {
		t.Fatal("nil var WriteVarSet must fail closed IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var WriteVarSet must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete base
	out2 := IncompleteEffect().WriteVarSet([]*Variable{CreateVariableScalars("g_x", GetIntType(), false, false)})
	if EffectComplete(out2) {
		t.Fatal("incomplete base WriteVarSet must IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete base WriteVarSet must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsSideEffectFreeIncompleteResidualSticky(t *testing.T) {
	// IsSideEffectFree residual soft invent was invent SE-free soft-skip past IncompleteEffect.
	ClearErrorSess(testAmbientSession)
	if IncompleteEffect().IsSideEffectFree() {
		t.Fatal("IncompleteEffect IsSideEffectFree must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsSideEffectFree must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete pure is SE-free
	if !EmptyEffect().IsSideEffectFree() {
		t.Fatal("EmptyEffect must be SE-free")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete EmptyEffect IsSideEffectFree must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIncompleteEffectCloneResidualSticky(t *testing.T) {
	// IncompleteEffect Clone residual soft invent was invent soft-complete snapshot past hole.
	ClearErrorSess(testAmbientSession)
	cp := IncompleteEffect().Clone()
	if EffectComplete(cp) {
		t.Fatal("IncompleteEffect Clone must stay IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect Clone must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete clone no sticky
	e := EmptyEffect()
	cp2 := e.Clone()
	if !EffectComplete(cp2) {
		t.Fatal("EmptyEffect Clone must complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete Clone must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAccessEnumAndEmptyEffect(t *testing.T) {
	// Effect.h Access { READ, WRITE }; get_empty_effect
	if AccessRead != 0 || AccessWrite != 1 {
		t.Fatalf("Access enum R=%d W=%d", AccessRead, AccessWrite)
	}
	e := EmptyEffect()
	if !e.IsPure() || !e.IsSideEffectFree() || !e.IsEmpty() {
		t.Fatal("empty_effect pure SE-free empty")
	}
}

func TestNonEmptyIntersectionMatch(t *testing.T) {
	// Effect.cpp:56–69 — Variable::match (identity / aggregate field)
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	if NonEmptyIntersection([]*Variable{a}, []*Variable{b}) {
		t.Fatal("distinct scalars no intersect")
	}
	if !NonEmptyIntersection([]*Variable{a}, []*Variable{a}) {
		t.Fatal("identity intersect")
	}
	// write-write race via has_race_with
	e1 := EmptyEffect().WriteVar(a)
	e2 := EmptyEffect().WriteVar(a)
	if !e1.HasRaceWith(e2) {
		t.Fatal("write-write race")
	}
	e3 := EmptyEffect().ReadVar(b)
	if e1.HasRaceWith(e3) {
		t.Fatal("disjoint no race")
	}
	ClearErrorSess(testAmbientSession)
}
