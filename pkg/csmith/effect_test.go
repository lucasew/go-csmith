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
	ClearError()
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	// restrictive true on residual GetCollective
	if !EmptyEffect().SiblingUnionFieldIsRead(shell) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsRead must fail closed true")
	}
	if !HasError() {
		t.Fatal("GetCollective residual SiblingUnionFieldIsRead must SetError sticky")
	}
	ClearError()
	if !EmptyEffect().SiblingUnionFieldIsWritten(shell) {
		t.Fatal("GetCollective residual SiblingUnionFieldIsWritten must fail closed true")
	}
	if !HasError() {
		t.Fatal("GetCollective residual SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearError()
}
