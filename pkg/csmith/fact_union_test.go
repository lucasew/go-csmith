package csmith

import (
	"strings"
	"testing"
)

func TestFactUnionLatticeConstants(t *testing.T) {
	// match upstream TOP=-2 BOTTOM=-1
	if FactUnionTop != -2 || FactUnionBottom != -1 {
		t.Fatalf("top=%d bottom=%d", FactUnionTop, FactUnionBottom)
	}
}

func TestFactUnionJoinAndImply(t *testing.T) {
	uv := &Variable{Name: "g_u"}
	a := MakeFactUnion(uv, 0)
	b := MakeFactUnion(uv, 1)
	if a.Imply(b) {
		t.Fatal("0 does not imply 1")
	}
	if !a.Imply(a) {
		t.Fatal("equal")
	}
	bot := MakeFactUnion(uv, FactUnionBottom)
	if !bot.Imply(a) {
		t.Fatal("bottom implies all")
	}
	if a.Imply(bot) {
		t.Fatal("concrete does not imply bottom")
	}
	// join different → bottom
	cp := a.Clone()
	if !cp.Join(b) {
		t.Fatal("join should change")
	}
	if !cp.IsBottom() {
		t.Fatal(cp.LastWrittenFID)
	}
	// join with equal → no change
	c2 := MakeFactUnion(uv, 0)
	if c2.Join(MakeFactUnion(uv, 0)) {
		t.Fatal("no change")
	}
}

func TestIsFieldReadable(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if !IsFieldReadable(uv, 0, facts) {
		t.Fatal("f0")
	}
	if IsFieldReadable(uv, 1, facts) {
		t.Fatal("f1")
	}
}

func TestFactUnionOutput(t *testing.T) {
	uv := CreateVariableScalars("g_u", GetIntType(), true, false)
	f := MakeFactUnion(uv, 2)
	s := f.Output()
	if !strings.Contains(s, "g_u") || !strings.Contains(s, "2") {
		t.Fatal(s)
	}
}

func TestJoinVarFactsUnion(t *testing.T) {
	u1 := &Variable{Name: "g_u1"}
	facts := []*FactUnion{MakeFactUnion(u1, 0), MakeFactUnion(u1, 1)}
	// same var twice with different fids — FindRelated finds first only
	// join list with one var
	j := JoinVarFactsUnion(facts, []*Variable{u1})
	if j == nil || j.LastWrittenFID != 0 {
		t.Fatal(j)
	}
}

func TestRhsToLhsTransferUnionConstant(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	lhs := &Variable{Name: "g_u", Type: ut}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	out := RhsToLhsTransferUnion(nil, nil, []*Variable{lhs}, rhs)
	if len(out) != 1 || out[0].LastWrittenFID != 0 || out[0].Var != lhs {
		t.Fatalf("%+v", out)
	}
}

func TestRhsToLhsTransferUnionVariable(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	src := &Variable{Name: "g_src", Type: ut}
	dst := &Variable{Name: "g_dst", Type: ut}
	ufacts := []*FactUnion{MakeFactUnion(src, 1)}
	rhs := &Expression{Term: TermVariable, Var: src, ExprType: ut}
	out := RhsToLhsTransferUnion(ufacts, nil, []*Variable{dst}, rhs)
	if len(out) != 1 || out[0].LastWrittenFID != 1 || out[0].Var != dst {
		t.Fatalf("%+v", out)
	}
}

func TestAbstractFactUnionForAssignField(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, {Name: "g_u.f1", Type: GetIntType(), FieldVarOf: parent}}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(3)}
	out, n := AbstractFactUnionForAssign(nil, nil, f0, 0, rhs)
	if n != 1 || len(out) != 1 || out[0].Var != parent || out[0].LastWrittenFID != 0 {
		t.Fatalf("n=%d out=%+v", n, out)
	}
}

func TestAbstractFactUnionForAssignUnionTypedLHS(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	lhs := &Variable{Name: "g_u", Type: ut}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	out, n := AbstractFactUnionForAssign(nil, nil, lhs, 0, rhs)
	if n != 1 || len(out) != 1 || out[0].LastWrittenFID != 0 {
		t.Fatalf("n=%d out=%+v", n, out)
	}
}

func TestAbstractFactUnionPaddingBottom(t *testing.T) {
	// FactUnion.cpp:144–146 — inside union field with type padding → BOTTOM
	// Assigning to a padded struct that is a union field (not the direct union field fid path).
	st := &Type{isStruct: true, Packed: false, Fields: []StructField{
		{Name: "x", Type: GetIntType(), BitWidth: -1},
	}}
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: st, BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	ufield := &Variable{Name: "g_u.f0", Type: st, FieldVarOf: parent}
	// nested struct field of the union field: is_inside_union_field, type has padding via walk?
	// C++ checks v->type->has_padding() on the LHS var itself — use padded struct type on nested root.
	// Direct union field uses IsUnionField path (fid), so use a child of ufield with padded type.
	nested := &Variable{Name: "g_u.f0.sub", Type: st, FieldVarOf: ufield}
	parent.FieldVars = []*Variable{ufield}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	out, _ := AbstractFactUnionForAssign(nil, nil, nested, 0, rhs)
	if len(out) != 1 || out[0].Var != parent || !out[0].IsBottom() {
		t.Fatalf("%+v", out)
	}
}
