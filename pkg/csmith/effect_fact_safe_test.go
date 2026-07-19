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
}

func TestIsWrittenIncompleteEffectFailClosed(t *testing.T) {
	// IsWritten/IsRead false on IncompleteEffect invents conflict-free / eligible
	v := CreateVariableScalars("g_v", GetIntType(), false, false)
	inc := IncompleteEffect()
	if !inc.IsWritten(v) {
		t.Fatal("incomplete IsWritten must fail closed true")
	}
	if !inc.IsRead(v) {
		t.Fatal("incomplete IsRead must fail closed true")
	}
	if !inc.IsWrittenPartially(v) || !inc.IsReadPartially(v) {
		t.Fatal("incomplete partial membership must fail closed true")
	}
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
	// sibling-union on incomplete effect
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	uv.CreateFieldVars()
	if len(uv.FieldVars) < 1 {
		t.Fatal("union fields")
	}
	if !inc.SiblingUnionFieldIsRead(uv.FieldVars[0]) || !inc.SiblingUnionFieldIsWritten(uv.FieldVars[0]) {
		t.Fatal("incomplete SiblingUnion* must fail closed true")
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
}

func TestEffectIsReadByName(t *testing.T) {
	v := CreateVariableScalars("g_x", GetIntType(), true, false)
	e := EmptyEffect().ReadVar(v).WriteVar(v)
	if !e.IsReadByName("g_x") || !e.IsWrittenByName("g_x") {
		t.Fatal("by name")
	}
	if e.IsReadByName("g_y") {
		t.Fatal("missing")
	}
}

func TestJoinVisits(t *testing.T) {
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
