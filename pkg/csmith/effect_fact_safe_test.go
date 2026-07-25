package csmith

import "testing"

func TestEffectHasGlobalAndUnionRead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	if g == nil {
		t.Fatal("global")
	}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	e := EmptyEffect().ReadVarSess(testAmbientSession, loc)
	if e.HasGlobalEffectSess(testAmbientSession) {
		t.Fatal("local only")
	}
	e = e.ReadVarSess(testAmbientSession, g)
	if !e.HasGlobalEffectSess(testAmbientSession) {
		t.Fatal("global")
	}
	// union field
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntType()}}}
	uv := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: uv}
	e2 := EmptyEffect().ReadVarSess(testAmbientSession, f0)
	if !e2.UnionFieldIsReadSess(testAmbientSession) {
		t.Fatal("union field read")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete UnionFieldIsRead must not sticky")
	}
	// IsInsideUnionField residual: Type-nil parent soft invent was soft-continue no-union-read.
	// Fair: sticky union-read true.
	ClearErrorSess(testAmbientSession)
	parentHole := &Variable{Name: "g_u2"} // Type nil
	fieldHole := &Variable{Name: "g_u2.f0", Type: GetIntType(), FieldVarOf: parentHole}
	e3 := EmptyEffect().ReadVarSess(testAmbientSession, fieldHole)
	if !e3.UnionFieldIsReadSess(testAmbientSession) {
		t.Fatal("IsInsideUnionField residual UnionFieldIsRead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsInsideUnionField residual UnionFieldIsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsReadPartially residual via Type-nil parent field walk.
	if !EmptyEffect().IsReadPartiallySess(testAmbientSession, fieldHole) {
		t.Fatal("IsRead residual IsReadPartially must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsRead residual IsReadPartially must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !EmptyEffect().IsWrittenPartiallySess(testAmbientSession, fieldHole) {
		t.Fatal("IsWritten residual IsWrittenPartially must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsWritten residual IsWrittenPartially must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectUpdatePurity(t *testing.T) {
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	e := EmptyEffect().WriteVarSess(testAmbientSession, g)
	// WriteVar already sets pure false typically — force pure then update
	e.pure = true
	e.UpdatePuritySess(testAmbientSession)
	if e.IsPureSess(testAmbientSession) {
		t.Fatal("not pure after global")
	}
	// Effect always live; sticky no invent soft-skip purity update past hole
	ClearErrorSess(testAmbientSession)
	(*Effect)(nil).UpdatePuritySess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil UpdatePurity must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectConsolidate(t *testing.T) {
	parent := CreateVariableScalarsSess(testAmbientSession, "g_s", GetIntType(), true, false)
	// make parent aggregate-ish with field
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect().ReadVarSess(testAmbientSession, parent).ReadVarSess(testAmbientSession, field)
	e.ConsolidateSess(testAmbientSession)
	// field entry removed from map (IsRead may still true via parent walk)
	if e.read[field] {
		t.Fatal("field read dropped when parent read")
	}
	if !e.IsReadSess(testAmbientSession, parent) {
		t.Fatal("parent kept")
	}
	e2 := EmptyEffect().WriteVarSess(testAmbientSession, parent).WriteVarSess(testAmbientSession, field)
	e2.ConsolidateSess(testAmbientSession)
	if e2.written[field] {
		t.Fatal("field write entry dropped")
	}
	if !e2.IsWrittenSess(testAmbientSession, parent) {
		t.Fatal("parent write kept")
	}
}

// TestReadVarNoRepushWhenStructParentRead mirrors Effect.cpp:116–122 + is_read
// (276–287): after a struct parent is in read_vars, reading a field must not
// re-push the field (is_read true via parent). Otherwise expand_struct_union_vars
// in choose_visible_read_var duplicates field entries and inflates ok-list size.
func TestReadVarNoRepushWhenStructParentRead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_s", Type: st}
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 2 {
		t.Fatal("need struct fields")
	}
	f0, f1 := parent.FieldVars[0], parent.FieldVars[1]

	// Effect.cpp:117 — if (!is_read(v)) push; parent then field → field skipped
	e := EmptyEffect().ReadVarSess(testAmbientSession, parent).ReadVarSess(testAmbientSession, f0).ReadVarSess(testAmbientSession, f1)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete ReadVar must not sticky")
	}
	reads := e.ReadVarsSess(testAmbientSession)
	if len(reads) != 1 || reads[0] != parent {
		t.Fatalf("only parent in read set after field reads: %v", namesOf(reads))
	}
	if !e.IsReadSess(testAmbientSession, f0) || !e.IsReadSess(testAmbientSession, f1) {
		t.Fatal("fields must still IsRead via parent struct walk")
	}

	// field-first then parent: both present (is_read parent is exact only)
	e2 := EmptyEffect().ReadVarSess(testAmbientSession, f0).ReadVarSess(testAmbientSession, parent)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("field-then-parent ReadVar must not sticky")
	}
	r2 := e2.ReadVarsSess(testAmbientSession)
	if len(r2) != 2 {
		t.Fatalf("field then parent should keep both: %v", namesOf(r2))
	}
	// expand of that set: parent expands to fields → f0 appears twice
	exp := ExpandStructUnionVars(append([]*Variable(nil), r2...), GetIntType())
	if !VariablesComplete(exp) {
		t.Fatal("expand must complete")
	}
	// parent-only set expands without dups
	expParent := ExpandStructUnionVars([]*Variable{parent}, GetIntType())
	if len(exp) <= len(expParent) {
		// field-first path can have dups; parent-only path is the fair post-ReadVar set
		t.Fatalf("sanity: field+parent expand len=%d parent-only=%d", len(exp), len(expParent))
	}

	// AddEffect also skips is_read covered fields (Effect.cpp:169–172)
	base := EmptyEffect().ReadVarSess(testAmbientSession, parent)
	other := EmptyEffect().ReadVarSess(testAmbientSession, f0)
	merged := base.AddEffectSess(testAmbientSession, other)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("AddEffect must not sticky")
	}
	if len(merged.ReadVarsSess(testAmbientSession)) != 1 || merged.ReadVarsSess(testAmbientSession)[0] != parent {
		t.Fatalf("AddEffect must not re-push field covered by parent: %v", namesOf(merged.ReadVarsSess(testAmbientSession)))
	}

	// WriteVar: is_written walks any parent (Effect.cpp:137–140 + 333–345)
	ew := EmptyEffect().WriteVarSess(testAmbientSession, parent).WriteVarSess(testAmbientSession, f0)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVar must not sticky")
	}
	if len(ew.WrittenVarsSess(testAmbientSession)) != 1 || ew.WrittenVarsSess(testAmbientSession)[0] != parent {
		t.Fatalf("only parent in write set after field write: %v", namesOf(ew.WrittenVarsSess(testAmbientSession)))
	}
	ClearErrorSess(testAmbientSession)
}

func namesOf(vars []*Variable) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		if v == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, v.Name)
	}
	return out
}

func TestEffectIsReadTypeNilParentSticky(t *testing.T) {
	// Type-nil parent sticky read true (restrictive — no invent not-read soft-skip)
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_s"} // Type nil
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect()
	if !e.IsReadSess(testAmbientSession, field) {
		t.Fatal("Type-nil parent IsRead must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil parent sticky written true (mirrors IsRead; no invent not-written)
	if !e.IsWrittenSess(testAmbientSession, field) {
		t.Fatal("Type-nil parent IsWritten must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsWritten must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete non-field not-read
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)
	if e.IsReadSess(testAmbientSession, v) {
		t.Fatal("unrelated var must be not-read complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete not-read must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if e.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("unrelated var must be not-written complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete not-written must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectSiblingTypeNilContainerSticky(t *testing.T) {
	// Type-nil container GetContainerUnion stickies; Sibling must not invent no-sibling false
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect()
	if !e.SiblingUnionFieldIsReadSess(testAmbientSession, field) {
		t.Fatal("Type-nil container SiblingUnionFieldIsRead must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil container SiblingUnionFieldIsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !e.SiblingUnionFieldIsWrittenSess(testAmbientSession, field) {
		t.Fatal("Type-nil container SiblingUnionFieldIsWritten must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil container SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectConsolidateTypeNilParentSticky(t *testing.T) {
	// Type-nil parent shell during consolidate: soft invent was leave-base complete.
	// Fair: sticky IncompleteEffect fail closed.
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_s"} // Type nil
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect().ReadVarSess(testAmbientSession, field)
	e.ConsolidateSess(testAmbientSession)
	if EffectComplete(e) {
		t.Fatal("Type-nil parent Consolidate must fail closed IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent Consolidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectConsolidateNilKeyFailClosed(t *testing.T) {
	// soft invent: delete some fields then hit nil key mid-map under random order
	// fair: incomplete sticky → IncompleteEffect (not invent partial consolidate / leave base complete)
	ClearErrorSess(testAmbientSession)
	parent := CreateVariableScalarsSess(testAmbientSession, "g_s", GetIntType(), true, false)
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect().ReadVarSess(testAmbientSession, parent).ReadVarSess(testAmbientSession, field)
	e.read[nil] = true
	e.ConsolidateSess(testAmbientSession)
	if EffectComplete(e) {
		t.Fatal("incomplete effect map must fail closed IncompleteEffect", e)
	}
	if e.IsEmptySess(testAmbientSession) || e.IsPureSess(testAmbientSession) {
		t.Fatal("IncompleteEffect must not invent empty/pure", e)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Consolidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestWriteReadVarIncompleteBaseFailClosed(t *testing.T) {
	// WriteVar/ReadVar on IncompleteEffect must not invent map growth as complete Effect sticky
	// (membership on incomplete is fail-closed true separately)
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, false)
	w := IncompleteEffect().WriteVarSess(testAmbientSession, v)
	if EffectComplete(w) {
		t.Fatal("WriteVar incomplete base must stay IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVar incomplete base must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	r := IncompleteEffect().ReadVarSess(testAmbientSession, v)
	if EffectComplete(r) {
		t.Fatal("ReadVar incomplete base must stay IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ReadVar incomplete base must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if EffectComplete(IncompleteEffect().AccessDerefVolatileSess(testAmbientSession, v, 1, true)) {
		t.Fatal("AccessDerefVolatile incomplete base must stay incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("AccessDerefVolatile incomplete base must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Clear incomplete base stays IncompleteEffect sticky (no invent wipe to empty pure)
	inc := IncompleteEffect()
	inc.ClearSess(testAmbientSession)
	if EffectComplete(inc) {
		t.Fatal("Clear incomplete base must stay IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Clear incomplete base must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Effect* always live at Clear/Consolidate; sticky no invent soft-skip past hole
	(*Effect)(nil).ClearSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Effect Clear must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*Effect)(nil).ConsolidateSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Effect Consolidate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsWrittenIncompleteEffectFailClosed(t *testing.T) {
	// IsWritten/IsRead false on IncompleteEffect invents conflict-free / eligible
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, false)
	inc := IncompleteEffect()
	if !inc.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("incomplete IsWritten must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete IsWritten must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !inc.IsReadSess(testAmbientSession, v) {
		t.Fatal("incomplete IsRead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete IsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !inc.IsWrittenPartiallySess(testAmbientSession, v) || !inc.IsReadPartiallySess(testAmbientSession, v) {
		t.Fatal("incomplete partial membership must fail closed true")
	}
	ClearErrorSess(testAmbientSession)
	// aggregate field membership
	st := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	parent := &Variable{Name: "g_s", Type: st}
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) == 0 {
		t.Fatal("fields")
	}
	if !inc.FieldIsWrittenSess(testAmbientSession, parent) || !inc.FieldIsReadSess(testAmbientSession, parent) {
		t.Fatal("incomplete FieldIs* must fail closed true")
	}
	ClearErrorSess(testAmbientSession)
	// sibling-union on incomplete effect
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	uv.CreateFieldVarsSess(testAmbientSession)
	if len(uv.FieldVars) < 1 || uv.FieldVars[0] == nil {
		t.Fatal("union fields")
	}
	if !inc.SiblingUnionFieldIsReadSess(testAmbientSession, uv.FieldVars[0]) || !inc.SiblingUnionFieldIsWrittenSess(testAmbientSession, uv.FieldVars[0]) {
		t.Fatal("incomplete SiblingUnion* must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete SiblingUnion* must SetError sticky")
	}
	// nil FieldVars hole sticky fail closed true
	ClearErrorSess(testAmbientSession)
	e := EmptyEffect()
	hole := &Variable{Name: "g_h", Type: st, FieldVars: []*Variable{nil}}
	if !e.FieldIsReadSess(testAmbientSession, hole) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FieldVars hole FieldIsRead must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
	if !e.FieldIsWrittenSess(testAmbientSession, hole) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FieldVars hole FieldIsWritten must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live; sticky true (no invent no-field-* soft-skip)
	if !e.FieldIsReadSess(testAmbientSession, nil) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable FieldIsRead must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
	if !e.FieldIsWrittenSess(testAmbientSession, nil) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable FieldIsWritten must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil subject soft invent: IsAggregate residual ERROR+false → no field conflict
	// fair: sticky true (restrictive) before classify
	shell := &Variable{Name: "g_typeless"}
	if !e.FieldIsReadSess(testAmbientSession, shell) || !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil FieldIsRead must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
	if !e.FieldIsWrittenSess(testAmbientSession, shell) || !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil FieldIsWritten must fail closed sticky true")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectIsReadByName(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), true, false)
	e := EmptyEffect().ReadVarSess(testAmbientSession, v).WriteVarSess(testAmbientSession, v)
	if !e.IsReadByNameSess(testAmbientSession, "g_x") || !e.IsWrittenByNameSess(testAmbientSession, "g_x") {
		t.Fatal("by name")
	}
	if e.IsReadByNameSess(testAmbientSession, "g_y") {
		t.Fatal("missing")
	}
	// incomplete effect sticky by-name membership
	ClearErrorSess(testAmbientSession)
	inc := IncompleteEffect()
	if !inc.IsReadByNameSess(testAmbientSession, "g_x") || !inc.IsWrittenByNameSess(testAmbientSession, "g_x") {
		t.Fatal("incomplete by-name must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete by-name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty name sticky true (no invent not-read / not-written soft-skip past hole)
	if !e.IsReadByNameSess(testAmbientSession, "") || !e.IsWrittenByNameSess(testAmbientSession, "") {
		t.Fatal("empty name by-name must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name by-name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestJoinVisits(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), true, false)
	// TBD-only base
	f := MakeFactPointTo(p, TBDPtr)
	if !f.IsTBDOnly() {
		t.Fatal("tbd")
	}
	other := MakeFactPointTo(p, a)
	if !f.JoinVisits(other) {
		t.Fatal("join")
	}
	if f.IsTBDOnly() || !IsVariableInSet(f.PointTo, a) {
		t.Fatal(f.PointTo)
	}
	// ignore TBD other
	f2 := MakeFactPointTo(p, a)
	if f2.JoinVisits(MakeFactPointTo(p, TBDPtr)) {
		t.Fatal("tbd other ignored")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete JoinVisits TBD-other ignore must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsTBDOnly residual soft invent was soft-continue join invent change.
	// Fair: sticky no-change false. PointTo nil hole IsTBDOnly residual.
	fHole := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	if fHole.JoinVisits(MakeFactPointTo(p, a)) {
		t.Fatal("IsTBDOnly residual JoinVisits must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsTBDOnly residual JoinVisits must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// JoinVisitsInto
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	JoinVisitsInto(&facts, []*FactPointTo{MakeFactPointTo(p, b)})
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || !IsVariableInSet(fp.PointTo, b) {
		t.Fatal(fp)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete JoinVisitsInto must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete maps fail closed sticky IncompleteFactSlice (not invent no-change complete)
	ClearErrorSess(testAmbientSession)
	factsHole := []*FactPointTo{MakeFactPointTo(p, a), nil}
	if JoinVisitsInto(&factsHole, []*FactPointTo{MakeFactPointTo(p, b)}) {
		t.Fatal("incomplete subject must fail closed false")
	}
	if FactsComplete(factsHole) {
		t.Fatal("incomplete subject must wipe IncompleteFactSlice", factsHole)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete subject JoinVisitsInto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	facts2 := []*FactPointTo{MakeFactPointTo(p, a)}
	if JoinVisitsInto(&facts2, []*FactPointTo{MakeFactPointTo(p, b), nil}) {
		t.Fatal("incomplete newFacts must fail closed false")
	}
	if FactsComplete(facts2) {
		t.Fatal("incomplete newFacts must wipe IncompleteFactSlice", facts2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete newFacts JoinVisitsInto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// facts always live; sticky (no invent soft-skip join-visits past hole)
	if JoinVisitsInto(nil, []*FactPointTo{MakeFactPointTo(p, b)}) {
		t.Fatal("nil facts JoinVisitsInto must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts JoinVisitsInto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeOpFlagsDummyAndFloat(t *testing.T) {
	d := MakeDummyFlags()
	if d == nil || d.Size != SafeInt8 || d.Op1Signed || d.IsFunc {
		t.Fatal(d)
	}
	c := d.Clone()
	if c == d || *c != *d {
		t.Fatal("clone")
	}
	opts := Defaults()
	opts.EnableFloat = false
	if ReturnFloatTypeBinary(opts, GetSimpleType(EFloat), nil, nil, BinAdd) {
		t.Fatal("float off")
	}
	opts.EnableFloat = true
	if !ReturnFloatTypeBinary(opts, GetSimpleType(EFloat), nil, nil, BinAdd) {
		t.Fatal("rv float")
	}
	if !ReturnFloatTypeUnary(opts, nil, GetSimpleType(EFloat), UnMinus) {
		t.Fatal("unary float op")
	}
	if UnaryOpWorksForFloat(UnBitNot) {
		t.Fatal("~ not for float")
	}
}

func TestIsPureIsEmptyIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	inc := IncompleteEffect()
	if inc.IsPureSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsPure must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsPure must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if inc.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsSideEffectFree must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsSideEffectFree must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if inc.IsEmptySess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsEmpty must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect IsEmpty must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !EmptyEffect().IsPureSess(testAmbientSession) || !EmptyEffect().IsSideEffectFreeSess(testAmbientSession) || !EmptyEffect().IsEmptySess(testAmbientSession) {
		t.Fatal("EmptyEffect pure SE-free empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("EmptyEffect predicates must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsTBDOnlyIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*FactPointTo)(nil).IsTBDOnly() {
		t.Fatal("nil Fact IsTBDOnly must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Fact IsTBDOnly must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	f := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	if f.IsTBDOnly() {
		t.Fatal("PointTo hole IsTBDOnly must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("PointTo hole IsTBDOnly must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReadWriteVarNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if EffectComplete(EmptyEffect().WriteVarSess(testAmbientSession, nil)) {
		t.Fatal("WriteVar nil must fail closed IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVar nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if EffectComplete(EmptyEffect().ReadVarSess(testAmbientSession, nil)) {
		t.Fatal("ReadVar nil must fail closed IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ReadVar nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAccessDerefVolatileResidualSticky(t *testing.T) {
	// IsVolatileAfterDeref residual soft invent was soft-continue peel invent complete SE-free.
	// Qfer depth > deref so OOB non-sticky true is skipped; Type-nil peels sticky.
	ClearErrorSess(testAmbientSession)
	hole := &Variable{
		Name: "g_p", Type: nil,
		Qfer: NewCVQualifiers([]bool{false, false}, []bool{false, false}),
	}
	out := EmptyEffect().AccessDerefVolatileSess(testAmbientSession, hole, 1, true)
	if EffectComplete(out) {
		t.Fatal("IsVolatileAfterDeref residual must fail closed IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileAfterDeref residual AccessDerefVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete non-vol peel must stay SE-free complete
	ok := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)
	e := EmptyEffect().AccessDerefVolatileSess(testAmbientSession, ok, 0, true)
	if !EffectComplete(e) {
		t.Fatal("complete AccessDerefVolatile level0 must stay complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete AccessDerefVolatile must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsReadIsStructResidualSticky(t *testing.T) {
	// IsStruct residual soft invent was invent not-read soft-skip past Type-nil parent.
	// Type-nil parent already sticky read true before IsStruct.
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_s", Type: nil}
	child := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect()
	if !e.IsReadSess(testAmbientSession, child) {
		t.Fatal("Type-nil parent IsRead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFieldIsReadIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent no-field-read past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	hole := &Variable{Name: "g_x", Type: nil}
	if !EmptyEffect().FieldIsReadSess(testAmbientSession, hole) {
		t.Fatal("Type-nil FieldIsRead must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil FieldIsRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete non-aggregate
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)
	if EmptyEffect().FieldIsReadSess(testAmbientSession, v) {
		t.Fatal("scalar FieldIsRead must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete FieldIsRead must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
