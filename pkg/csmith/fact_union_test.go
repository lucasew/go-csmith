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
	// FactUnion.cpp:163 — make_fact requires union type
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	a := MakeFactUnion(uv, 0)
	b := MakeFactUnion(uv, 1)
	if a == nil || b == nil {
		t.Fatal("make fact")
	}
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
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
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
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	uv := &Variable{Name: "g_u", Type: ut}
	f := MakeFactUnion(uv, 2)
	if f == nil {
		t.Fatal("make")
	}
	s := f.Output()
	if !strings.Contains(s, "g_u") || !strings.Contains(s, "2") {
		t.Fatal(s)
	}
	// sticky no invent " last written field: N" without identifier
	ClearErrorSess(testAmbientSession)
	anon := MakeFactUnion(&Variable{Type: ut}, 0)
	if anon != nil {
		if out := anon.Output(); out != "" {
			t.Fatal("empty union var name must fail closed", out)
		}
		if !HasErrorSess(testAmbientSession) {
			t.Fatal("empty union var name Output must SetError sticky")
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeFactUnionNonUnionFailClosed(t *testing.T) {
	// FactUnion.cpp:163 assert union type sticky — no invent FactUnion on int
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), true, false)
	if MakeFactUnion(v, 0) != nil {
		t.Fatal("non-union must not invent FactUnion")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-union MakeFactUnion must SetError sticky")
	}
	// MakeFactUnions fails closed sticky incomplete on non-union / nil hole
	ClearErrorSess(testAmbientSession)
	if UnionFactsComplete(MakeFactUnions([]*Variable{v}, 0)) {
		t.Fatal("non-union MakeFactUnions must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-union MakeFactUnions must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if UnionFactsComplete(MakeFactUnions([]*Variable{nil}, 0)) {
		t.Fatal("nil hole MakeFactUnions must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole MakeFactUnions must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if len(MakeFactUnions([]*Variable{}, 0)) != 0 {
		t.Fatal("empty vars must yield empty facts")
	}
}

func TestJoinVarFactsUnion(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	u1 := &Variable{Name: "g_u1", Type: ut}
	facts := []*FactUnion{MakeFactUnion(u1, 0), MakeFactUnion(u1, 1)}
	// same var twice with different fids — FindRelated finds first only
	// join list with one var
	j := JoinVarFactsUnion(facts, []*Variable{u1})
	if j == nil || j.LastWrittenFID != 0 {
		t.Fatal(j)
	}
}

func TestJoinVarFactsUnionResidualSticky(t *testing.T) {
	// FindRelated residual soft invent was continue then join later complete var.
	// Fair: sticky fail closed nil whole join.
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1}}}
	u1 := &Variable{Name: "g_u1", Type: ut}
	u2 := &Variable{Name: "g_u2", Type: ut}
	// facts hole then complete fact — soft invent was skip hole invent join u2
	facts := []*FactUnion{nil, MakeFactUnion(u2, 0)}
	if JoinVarFactsUnion(facts, []*Variable{u1, u2}) != nil {
		t.Fatal("FindRelated residual must fail closed JoinVarFactsUnion")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindRelated residual JoinVarFactsUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetLastWrittenTypeUnionOnly(t *testing.T) {
	// FactUnion.cpp:65 assert union; OOB fid fail closed sticky
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut, FieldVars: []*Variable{
		{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession)},
	}}
	f := MakeFactUnion(uv, 0)
	if f.GetLastWrittenType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("field0 type")
	}
	f.LastWrittenFID = 99
	if f.GetLastWrittenType() != nil {
		t.Fatal("OOB fid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OOB fid GetLastWrittenType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactUnion always live; sticky nil (no invent soft-skip past hole)
	if (*FactUnion)(nil).GetLastWrittenType() != nil {
		t.Fatal("nil GetLastWrittenType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetLastWrittenType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&FactUnion{}).GetLastWrittenType() != nil {
		t.Fatal("nil Var GetLastWrittenType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var GetLastWrittenType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferUnionConstant(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
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
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	src := &Variable{Name: "g_src", Type: ut}
	dst := &Variable{Name: "g_dst", Type: ut}
	ufacts := []*FactUnion{MakeFactUnion(src, 1)}
	rhs := &Expression{Term: TermVariable, Var: src, ExprType: ut}
	out := RhsToLhsTransferUnion(ufacts, nil, []*Variable{dst}, rhs)
	if len(out) != 1 || out[0].LastWrittenFID != 1 || out[0].Var != dst {
		t.Fatalf("%+v", out)
	}
	// incomplete union map — non-sticky hole (soft re-pick factories)
	ClearErrorSess(testAmbientSession)
	if UnionFactsComplete(RhsToLhsTransferUnion([]*FactUnion{MakeFactUnion(src, 0), nil}, nil, []*Variable{dst}, rhs)) {
		t.Fatal("incomplete unionFacts must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete unionFacts transfer must stay non-sticky")
	}
	// non-union lvar hard IR sticky
	ClearErrorSess(testAmbientSession)
	i := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), true, false)
	if UnionFactsComplete(RhsToLhsTransferUnion(nil, nil, []*Variable{i}, rhs)) {
		t.Fatal("non-union lvar must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-union lvar RhsToLhsTransferUnion must SetError sticky")
	}
	// nil rhs with targets — incomplete non-sticky (AddParamFacts missing union args)
	ClearErrorSess(testAmbientSession)
	if UnionFactsComplete(RhsToLhsTransferUnion(nil, nil, []*Variable{dst}, nil)) {
		t.Fatal("nil rhs must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil rhs RhsToLhsTransferUnion must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	// empty lvars is complete empty
	if RhsToLhsTransferUnion(nil, nil, nil, rhs) != nil {
		t.Fatal("empty lvars must be complete empty")
	}
}

func TestRhsToLhsTransferUnionCommaNilRHSFailClosed(t *testing.T) {
	// incomplete CommaRHS must not soft-re-pick via bare nil-rhs peel path
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	dst := &Variable{Name: "g_dst", Type: ut}
	rhs := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(1)},
		// CommaRHS nil
	}
	out := RhsToLhsTransferUnion(nil, nil, []*Variable{dst}, rhs)
	if UnionFactsComplete(out) {
		t.Fatal("nil CommaRHS must fail closed incomplete", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CommaRHS RhsToLhsTransferUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferUnionAssignNilExprFailClosed(t *testing.T) {
	// incomplete Assign.Expr must not soft-re-pick via bare nil-rhs peel path
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	dst := &Variable{Name: "g_dst", Type: ut}
	assign := &Stmt{
		Kind:     StmtAssign,
		LhsVar:   dst,
		Lhs:      &Lhs{Var: dst, Type: ut},
		AssignOp: AssignSimple,
		// Expr nil
	}
	rhs := &Expression{Term: TermAssignment, Assign: assign, ExprType: ut}
	out := RhsToLhsTransferUnion(nil, nil, []*Variable{dst}, rhs)
	if UnionFactsComplete(out) {
		t.Fatal("nil Assign.Expr must fail closed incomplete", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Assign.Expr RhsToLhsTransferUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactUnionForAssignField(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, {Name: "g_u.f1", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(3)}
	out, n := AbstractFactUnionForAssign(nil, nil, f0, 0, nil, rhs)
	if n != 1 || len(out) != 1 || out[0].Var != parent || out[0].LastWrittenFID != 0 {
		t.Fatalf("n=%d out=%+v", n, out)
	}
}

func TestAbstractFactUnionForAssignUnionTypedLHS(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	lhs := &Variable{Name: "g_u", Type: ut}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	out, n := AbstractFactUnionForAssign(nil, nil, lhs, 0, nil, rhs)
	if n != 1 || len(out) != 1 || out[0].LastWrittenFID != 0 {
		t.Fatalf("n=%d out=%+v", n, out)
	}
}

func TestAbstractFactUnionPaddingBottom(t *testing.T) {
	// FactUnion.cpp:144–146 — inside union field with type padding → BOTTOM
	// Assigning to a padded struct that is a union field (not the direct union field fid path).
	st := &Type{isStruct: true, Packed: false, Fields: []StructField{
		{Name: "x", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
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
	out, _ := AbstractFactUnionForAssign(nil, nil, nested, 0, nil, rhs)
	if len(out) != 1 || out[0].Var != parent || !out[0].IsBottom() {
		t.Fatalf("%+v", out)
	}
}

func TestAbstractFactUnionTypeNilSticky(t *testing.T) {
	// FactUnion.cpp:129 — lhs->get_type() always live; Type-nil sticky incomplete
	ClearErrorSess(testAmbientSession)
	lhs := &Variable{Name: "g_x", Type: nil}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	out, _ := AbstractFactUnionForAssign(nil, nil, lhs, 0, nil, rhs)
	if UnionFactsComplete(out) {
		t.Fatal("Type-nil LHS must IncompleteUnionFactSlice, not invent non-union complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil LHS AbstractFactUnionForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// special null Type-nil is complete non-union path (by design)
	out2, n := AbstractFactUnionForAssign(nil, nil, NullPtr, 0, nil, rhs)
	if !UnionFactsComplete(out2) || n != 1 {
		t.Fatalf("special Type-nil must complete non-union path n=%d out=%+v", n, out2)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("special Type-nil must not SetError")
	}
	ClearErrorSess(testAmbientSession)
	// IsUnionField residual: Type-nil parent soft invent was soft-continue IsInside path.
	// Fair: sticky IncompleteUnionFactSlice.
	parentHole := &Variable{Name: "g_u", Type: nil}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parentHole}
	parentHole.FieldVars = []*Variable{f0}
	out3, _ := AbstractFactUnionForAssign(nil, nil, f0, 0, nil, rhs)
	if UnionFactsComplete(out3) {
		t.Fatal("IsUnionField residual AbstractFactUnion must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsUnionField residual AbstractFactUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindRelatedUnionNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if FindRelatedUnion(nil, nil) != nil {
		t.Fatal("nil subject FindRelatedUnion must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject FindRelatedUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_u", GetIntTypeSess(testAmbientSession), false, false)
	if FindRelatedUnion([]*FactUnion{nil}, v) != nil {
		t.Fatal("nil fact hole FindRelatedUnion must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole FindRelatedUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFactUnionEqualImplyJoinIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).Equal(&FactUnion{}) {
		t.Fatal("nil Equal must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Equal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).Imply(&FactUnion{}) {
		t.Fatal("nil Imply must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Imply must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).Join(&FactUnion{}) {
		t.Fatal("nil Join must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Join must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFactUnionIsTopBottomCloneIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).IsTop() {
		t.Fatal("nil IsTop must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsTop must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).IsBottom() {
		t.Fatal("nil IsBottom must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsBottom must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).Clone() != nil {
		t.Fatal("nil Clone must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Clone must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &FactUnion{LastWrittenFID: FactUnionTop}
	if !f.IsTop() || f.IsBottom() {
		t.Fatal("TOP lattice")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsTop must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeFactUnionIsUnionResidualSticky(t *testing.T) {
	// IsUnion residual soft invent was invent soft-nil FactUnion past non-union Type.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if MakeFactUnion(v, 0) != nil {
		t.Fatal("non-union MakeFactUnion must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-union MakeFactUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil sticky
	if MakeFactUnion(&Variable{Name: "g_y", Type: nil}, 0) != nil {
		t.Fatal("Type-nil MakeFactUnion must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil MakeFactUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestImplyIsBottomResidualSticky(t *testing.T) {
	// IsBottom residual soft invent was invent soft-imply past nil FactUnion.
	ClearErrorSess(testAmbientSession)
	if (*FactUnion)(nil).Imply(MakeFactUnionTop(CreateVariableScalarsSess(testAmbientSession, "g_u", GetIntTypeSess(testAmbientSession), false, false))) {
		t.Fatal("nil Imply must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Imply must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
