package csmith

import "testing"

func TestEffectHasGlobalAndUnionRead(t *testing.T) {
	ClearError()
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	if g == nil {
		t.Fatal("global")
	}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	e := EmptyEffect().ReadVar(loc)
	if e.HasGlobalEffect() {
		t.Fatal("local only")
	}
	e = e.ReadVar(g)
	if !e.HasGlobalEffect() {
		t.Fatal("global")
	}
	// union field
	ut := &Type{isUnion: true, Fields: []StructField{{Name: "f0", Type: GetIntType()}}}
	uv := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: uv}
	e2 := EmptyEffect().ReadVar(f0)
	if !e2.UnionFieldIsRead() {
		t.Fatal("union field read")
	}
}

func TestEffectUpdatePurity(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	e := EmptyEffect().WriteVar(g)
	// WriteVar already sets pure false typically — force pure then update
	e.pure = true
	e.UpdatePurity()
	if e.IsPure() {
		t.Fatal("not pure after global")
	}
	// Effect always live; sticky no invent soft-skip purity update past hole
	ClearError()
	(*Effect)(nil).UpdatePurity()
	if !HasError() {
		t.Fatal("nil UpdatePurity must SetError sticky")
	}
	ClearError()
}

func TestEffectConsolidate(t *testing.T) {
	parent := CreateVariableScalars("g_s", GetIntType(), true, false)
	// make parent aggregate-ish with field
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect().ReadVar(parent).ReadVar(field)
	e.Consolidate()
	// field entry removed from map (IsRead may still true via parent walk)
	if e.read[field] {
		t.Fatal("field read dropped when parent read")
	}
	if !e.IsRead(parent) {
		t.Fatal("parent kept")
	}
	e2 := EmptyEffect().WriteVar(parent).WriteVar(field)
	e2.Consolidate()
	if e2.written[field] {
		t.Fatal("field write entry dropped")
	}
	if !e2.IsWritten(parent) {
		t.Fatal("parent write kept")
	}
}

func TestEffectIsReadTypeNilParentSticky(t *testing.T) {
	// Type-nil parent sticky read true (restrictive — no invent not-read soft-skip)
	ClearError()
	parent := &Variable{Name: "g_s"} // Type nil
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect()
	if !e.IsRead(field) {
		t.Fatal("Type-nil parent IsRead must fail closed true restrictive")
	}
	if !HasError() {
		t.Fatal("Type-nil parent IsRead must SetError sticky")
	}
	ClearError()
	// complete non-field not-read
	v := CreateVariableScalars("g_i", GetIntType(), false, false)
	if e.IsRead(v) {
		t.Fatal("unrelated var must be not-read complete")
	}
	if HasError() {
		t.Fatal("complete not-read must not sticky")
	}
	ClearError()
}

func TestEffectSiblingTypeNilContainerSticky(t *testing.T) {
	// Type-nil container GetContainerUnion stickies; Sibling must not invent no-sibling false
	ClearError()
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect()
	if !e.SiblingUnionFieldIsRead(field) {
		t.Fatal("Type-nil container SiblingUnionFieldIsRead must fail closed true restrictive")
	}
	if !HasError() {
		t.Fatal("Type-nil container SiblingUnionFieldIsRead must SetError sticky")
	}
	ClearError()
	if !e.SiblingUnionFieldIsWritten(field) {
		t.Fatal("Type-nil container SiblingUnionFieldIsWritten must fail closed true restrictive")
	}
	if !HasError() {
		t.Fatal("Type-nil container SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearError()
}

func TestEffectConsolidateNilKeyFailClosed(t *testing.T) {
	// soft invent: delete some fields then hit nil key mid-map under random order
	// fair: incomplete sticky → IncompleteEffect (not invent partial consolidate / leave base complete)
	ClearError()
	parent := CreateVariableScalars("g_s", GetIntType(), true, false)
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e := EmptyEffect().ReadVar(parent).ReadVar(field)
	e.read[nil] = true
	e.Consolidate()
	if EffectComplete(e) {
		t.Fatal("incomplete effect map must fail closed IncompleteEffect", e)
	}
	if e.IsEmpty() || e.IsPure() {
		t.Fatal("IncompleteEffect must not invent empty/pure", e)
	}
	if !HasError() {
		t.Fatal("incomplete Consolidate must SetError sticky")
	}
	ClearError()
}

func TestWriteReadVarIncompleteBaseFailClosed(t *testing.T) {
	// WriteVar/ReadVar on IncompleteEffect must not invent map growth as complete Effect sticky
	// (membership on incomplete is fail-closed true separately)
	ClearError()
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	w := IncompleteEffect().WriteVar(v)
	if EffectComplete(w) {
		t.Fatal("WriteVar incomplete base must stay IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("WriteVar incomplete base must SetError sticky")
	}
	ClearError()
	r := IncompleteEffect().ReadVar(v)
	if EffectComplete(r) {
		t.Fatal("ReadVar incomplete base must stay IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("ReadVar incomplete base must SetError sticky")
	}
	ClearError()
	if EffectComplete(IncompleteEffect().AccessDerefVolatile(v, 1, true)) {
		t.Fatal("AccessDerefVolatile incomplete base must stay incomplete")
	}
	if !HasError() {
		t.Fatal("AccessDerefVolatile incomplete base must SetError sticky")
	}
	ClearError()
	// Clear incomplete base stays IncompleteEffect sticky (no invent wipe to empty pure)
	inc := IncompleteEffect()
	inc.Clear()
	if EffectComplete(inc) {
		t.Fatal("Clear incomplete base must stay IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("Clear incomplete base must SetError sticky")
	}
	ClearError()
	// Effect* always live at Clear/Consolidate; sticky no invent soft-skip past hole
	(*Effect)(nil).Clear()
	if !HasError() {
		t.Fatal("nil Effect Clear must SetError sticky")
	}
	ClearError()
	(*Effect)(nil).Consolidate()
	if !HasError() {
		t.Fatal("nil Effect Consolidate must SetError sticky")
	}
	ClearError()
}

func TestIsWrittenIncompleteEffectFailClosed(t *testing.T) {
	// IsWritten/IsRead false on IncompleteEffect invents conflict-free / eligible
	ClearError()
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	inc := IncompleteEffect()
	if !inc.IsWritten(v) {
		t.Fatal("incomplete IsWritten must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete IsWritten must SetError sticky")
	}
	ClearError()
	if !inc.IsRead(v) {
		t.Fatal("incomplete IsRead must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete IsRead must SetError sticky")
	}
	ClearError()
	if !inc.IsWrittenPartially(v) || !inc.IsReadPartially(v) {
		t.Fatal("incomplete partial membership must fail closed true")
	}
	ClearError()
	// aggregate field membership
	st := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	parent := &Variable{Name: "g_s", Type: st}
	parent.CreateFieldVars()
	if len(parent.FieldVars) == 0 {
		t.Fatal("fields")
	}
	if !inc.FieldIsWritten(parent) || !inc.FieldIsRead(parent) {
		t.Fatal("incomplete FieldIs* must fail closed true")
	}
	ClearError()
	// sibling-union on incomplete effect
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	uv.CreateFieldVars()
	if len(uv.FieldVars) < 1 || uv.FieldVars[0] == nil {
		t.Fatal("union fields")
	}
	if !inc.SiblingUnionFieldIsRead(uv.FieldVars[0]) || !inc.SiblingUnionFieldIsWritten(uv.FieldVars[0]) {
		t.Fatal("incomplete SiblingUnion* must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete SiblingUnion* must SetError sticky")
	}
	// nil FieldVars hole sticky fail closed true
	ClearError()
	e := EmptyEffect()
	hole := &Variable{Name: "g_h", Type: st, FieldVars: []*Variable{nil}}
	if !e.FieldIsRead(hole) || !HasError() {
		t.Fatal("nil FieldVars hole FieldIsRead must fail closed sticky true")
	}
	ClearError()
	if !e.FieldIsWritten(hole) || !HasError() {
		t.Fatal("nil FieldVars hole FieldIsWritten must fail closed sticky true")
	}
	ClearError()
	// Variable always live; sticky true (no invent no-field-* soft-skip)
	if !e.FieldIsRead(nil) || !HasError() {
		t.Fatal("nil Variable FieldIsRead must fail closed sticky true")
	}
	ClearError()
	if !e.FieldIsWritten(nil) || !HasError() {
		t.Fatal("nil Variable FieldIsWritten must fail closed sticky true")
	}
	ClearError()
}

func TestEffectIsReadByName(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), true, false)
	e := EmptyEffect().ReadVar(v).WriteVar(v)
	if !e.IsReadByName("g_x") || !e.IsWrittenByName("g_x") {
		t.Fatal("by name")
	}
	if e.IsReadByName("g_y") {
		t.Fatal("missing")
	}
	// incomplete effect sticky by-name membership
	ClearError()
	inc := IncompleteEffect()
	if !inc.IsReadByName("g_x") || !inc.IsWrittenByName("g_x") {
		t.Fatal("incomplete by-name must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete by-name must SetError sticky")
	}
	ClearError()
	// empty name sticky true (no invent not-read / not-written soft-skip past hole)
	if !e.IsReadByName("") || !e.IsWrittenByName("") {
		t.Fatal("empty name by-name must fail closed true")
	}
	if !HasError() {
		t.Fatal("empty name by-name must SetError sticky")
	}
	ClearError()
}

func TestJoinVisits(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
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
	// JoinVisitsInto
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	JoinVisitsInto(&facts, []*FactPointTo{MakeFactPointTo(p, b)})
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || !IsVariableInSet(fp.PointTo, b) {
		t.Fatal(fp)
	}
	// incomplete maps fail closed sticky IncompleteFactSlice (not invent no-change complete)
	ClearError()
	factsHole := []*FactPointTo{MakeFactPointTo(p, a), nil}
	if JoinVisitsInto(&factsHole, []*FactPointTo{MakeFactPointTo(p, b)}) {
		t.Fatal("incomplete subject must fail closed false")
	}
	if FactsComplete(factsHole) {
		t.Fatal("incomplete subject must wipe IncompleteFactSlice", factsHole)
	}
	if !HasError() {
		t.Fatal("incomplete subject JoinVisitsInto must SetError sticky")
	}
	ClearError()
	facts2 := []*FactPointTo{MakeFactPointTo(p, a)}
	if JoinVisitsInto(&facts2, []*FactPointTo{MakeFactPointTo(p, b), nil}) {
		t.Fatal("incomplete newFacts must fail closed false")
	}
	if FactsComplete(facts2) {
		t.Fatal("incomplete newFacts must wipe IncompleteFactSlice", facts2)
	}
	if !HasError() {
		t.Fatal("incomplete newFacts JoinVisitsInto must SetError sticky")
	}
	ClearError()
	// facts always live; sticky (no invent soft-skip join-visits past hole)
	if JoinVisitsInto(nil, []*FactPointTo{MakeFactPointTo(p, b)}) {
		t.Fatal("nil facts JoinVisitsInto must fail closed")
	}
	if !HasError() {
		t.Fatal("nil facts JoinVisitsInto must SetError sticky")
	}
	ClearError()
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
	ClearError()
	inc := IncompleteEffect()
	if inc.IsPure() {
		t.Fatal("IncompleteEffect IsPure must fail closed false")
	}
	if !HasError() {
		t.Fatal("IncompleteEffect IsPure must SetError sticky")
	}
	ClearError()
	if inc.IsSideEffectFree() {
		t.Fatal("IncompleteEffect IsSideEffectFree must fail closed false")
	}
	if !HasError() {
		t.Fatal("IncompleteEffect IsSideEffectFree must SetError sticky")
	}
	ClearError()
	if inc.IsEmpty() {
		t.Fatal("IncompleteEffect IsEmpty must fail closed false")
	}
	if !HasError() {
		t.Fatal("IncompleteEffect IsEmpty must SetError sticky")
	}
	ClearError()
	if !EmptyEffect().IsPure() || !EmptyEffect().IsSideEffectFree() || !EmptyEffect().IsEmpty() {
		t.Fatal("EmptyEffect pure SE-free empty")
	}
	if HasError() {
		t.Fatal("EmptyEffect predicates must not sticky")
	}
	ClearError()
}

func TestIsTBDOnlyIncompleteSticky(t *testing.T) {
	ClearError()
	if (*FactPointTo)(nil).IsTBDOnly() {
		t.Fatal("nil Fact IsTBDOnly must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Fact IsTBDOnly must SetError sticky")
	}
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	if f.IsTBDOnly() {
		t.Fatal("PointTo hole IsTBDOnly must fail closed false")
	}
	if !HasError() {
		t.Fatal("PointTo hole IsTBDOnly must SetError sticky")
	}
	ClearError()
}

func TestReadWriteVarNilSticky(t *testing.T) {
	ClearError()
	if EffectComplete(EmptyEffect().WriteVar(nil)) {
		t.Fatal("WriteVar nil must fail closed IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("WriteVar nil must SetError sticky")
	}
	ClearError()
	if EffectComplete(EmptyEffect().ReadVar(nil)) {
		t.Fatal("ReadVar nil must fail closed IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("ReadVar nil must SetError sticky")
	}
	ClearError()
}
