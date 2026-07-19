package csmith

import "testing"

func TestFactPointToNullDead(t *testing.T) {
	p := CreateVariableScalars("g_p", GetIntType(), false, false)
	// default NewFactPointTo starts garbage
	f := NewFactPointTo(p)
	// nil PointTo hole fails closed as dead/null (no invent not-dead/not-null)
	hole := &FactPointTo{Var: CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false), PointTo: []*Variable{nil}}
	if !hole.IsDead() || !hole.IsNull() {
		t.Fatal("nil pointee hole must fail closed IsDead/IsNull")
	}
	if !f.IsDead() || f.IsNull() {
		t.Fatal("init garbage")
	}
	fn := MakeFactPointTo(p, NullPtr)
	if !fn.IsNull() || fn.IsDead() {
		t.Fatal("null fact")
	}
	if !IsSpecialPtr(NullPtr) || !IsSpecialPtr(GarbagePtr) || !IsSpecialPtr(TBDPtr) {
		t.Fatal("special")
	}
	// Variable.cpp:280–288 — is_virtual is array collective parent, not dummy specials
	if NullPtr.Type != nil {
		t.Fatal("dummy null type")
	}
	if NullPtr.IsVirtual() {
		t.Fatal("special ptr is not array is_virtual")
	}
	// no invent FactPointTo shell without live subject Variable*
	if NewFactPointTo(nil) != nil || MakeFactPointTo(nil, NullPtr) != nil || MakeFactPointToSet(nil, nil) != nil {
		t.Fatal("nil subject must fail closed fact ctor")
	}
	// no invent fact with nil pointee / incomplete PointTo set
	if MakeFactPointTo(p, nil) != nil {
		t.Fatal("nil pointTo must fail closed MakeFactPointTo")
	}
	if MakeFactPointToSet(p, []*Variable{NullPtr, nil}) != nil {
		t.Fatal("nil hole in set must fail closed MakeFactPointToSet")
	}
	// nil set is incomplete merge — no invent empty IsTop from nil
	if MakeFactPointToSet(p, nil) != nil {
		t.Fatal("nil set must fail closed MakeFactPointToSet")
	}
	if FactsComplete(MakeFactsPointToSet([]*Variable{p}, nil)) {
		t.Fatal("nil set must fail closed incomplete MakeFactsPointToSet")
	}
	// empty non-nil is valid top
	if MakeFactPointToSet(p, []*Variable{}) == nil {
		t.Fatal("empty non-nil set must succeed as top")
	}
	// Clone of incomplete PointTo fails closed
	if (&FactPointTo{Var: p, PointTo: []*Variable{nil}}).Clone() != nil {
		t.Fatal("Clone incomplete PointTo must fail closed")
	}
	// FactsComplete requires complete PointTo (empty IsTop OK)
	if !FactsComplete([]*FactPointTo{{Var: p, PointTo: nil}}) {
		t.Fatal("empty PointTo (top) is complete")
	}
	if FactsComplete([]*FactPointTo{{Var: p, PointTo: []*Variable{nil}}}) {
		t.Fatal("nil pointee hole is incomplete")
	}
	// CloneFactSlice incomplete → hole marker (not bare nil invent empty complete)
	if FactsComplete(CloneFactSlice([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil})) {
		t.Fatal("CloneFactSlice nil fact hole must fail closed incomplete")
	}
	if FactsComplete(CloneFactSlice([]*FactPointTo{{Var: p, PointTo: []*Variable{nil}}})) {
		t.Fatal("CloneFactSlice pointee hole must fail closed incomplete")
	}
	// complete empty stays empty complete
	if CloneFactSlice(nil) != nil {
		t.Fatal("CloneFactSlice(nil) must stay complete empty nil")
	}
	if cl := CloneFactSlice([]*FactPointTo{}); cl == nil || !FactsComplete(cl) {
		t.Fatal("CloneFactSlice empty non-nil must stay complete empty", cl)
	}
	// complete non-empty clones
	if cl := CloneFactSlice([]*FactPointTo{MakeFactPointTo(p, NullPtr)}); !FactsComplete(cl) || len(cl) != 1 {
		t.Fatal("CloneFactSlice complete must clone", cl)
	}
	// MakeFacts — no invent skip of nil holes as partial success / empty complete
	if FactsComplete(MakeFactsPointTo([]*Variable{p, nil}, NullPtr)) {
		t.Fatal("nil hole in lvars must fail closed incomplete MakeFactsPointTo")
	}
	if FactsComplete(MakeFactsPointToSet([]*Variable{nil, p}, []*Variable{NullPtr})) {
		t.Fatal("nil hole in lvars must fail closed incomplete MakeFactsPointToSet")
	}
	// specials Type-nil skipped; non-special Type-nil fails closed whole batch
	if !FactsComplete(MakeFactsPointTo([]*Variable{NullPtr, p}, NullPtr)) {
		t.Fatal("special Type-nil must soft-skip not fail batch")
	}
	broken := &Variable{Name: "broken"} // Type nil, not special
	if FactsComplete(MakeFactsPointTo([]*Variable{broken, p}, NullPtr)) {
		t.Fatal("non-special Type-nil must fail closed incomplete MakeFactsPointTo")
	}
	if FactsComplete(MakeFactsPointToSet([]*Variable{broken, p}, []*Variable{NullPtr})) {
		t.Fatal("non-special Type-nil must fail closed incomplete MakeFactsPointToSet")
	}
}

func TestArrayIsVirtualCollectiveParent(t *testing.T) {
	// Variable.cpp:285–286 — collective==0 → virtual; itemized → not
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	if !parent.IsVirtual() {
		t.Fatal("parent collective must be virtual")
	}
	item := parent.Itemize(NewRng(1))
	if item == nil || item.IsVirtual() {
		t.Fatal("itemized member must not be virtual")
	}
	// field of parent array is virtual via recurse
	parent.Type = &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	parent.CreateFieldVars()
	if len(parent.FieldVars) == 0 {
		t.Skip("no fields")
	}
	if !parent.FieldVars[0].IsVirtual() {
		t.Fatal("field of virtual array must be virtual")
	}
}

func TestIsValidPtr(t *testing.T) {
	p := CreateVariableScalars("g_p", GetIntType(), false, false)
	target := CreateVariableScalars("g_t", GetIntType(), false, false)
	// no fact → invalid
	if IsValidPtr(p, nil, 0, 0) {
		t.Fatal("no fact")
	}
	// points to real target → valid
	facts := []*FactPointTo{MakeFactPointTo(p, target)}
	if !IsValidPtr(p, facts, 0, 0) {
		t.Fatal("live")
	}
	// null with prob 0 → invalid
	facts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if IsValidPtr(p, facts, 0, 0) {
		t.Fatal("null blocked")
	}
	// null with prob >0 → allowed
	if !IsValidPtr(p, facts, 1, 0) {
		t.Fatal("null allowed")
	}
	// garbage with prob 0 → invalid + dangling
	facts = []*FactPointTo{NewFactPointTo(p)}
	if IsValidPtr(p, facts, 0, 0) {
		t.Fatal("dead blocked")
	}
	if !IsDanglingPtr(p, facts, 0) {
		t.Fatal("dangling")
	}
	// incomplete maps fail closed as dangling (no invent not-dangling past hole)
	hole := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	if !IsDanglingPtr(p, hole, 0) {
		t.Fatal("incomplete facts must fail closed as dangling")
	}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), hole, 0, 0) != 0 {
		t.Fatal("incomplete facts must reject opportunistic validate")
	}
}

func TestFactMgrGlobalFacts(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", GetIntType(), false, false)
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointTo(p, NullPtr))
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("lookup")
	}
}

func TestMarkFuncEnd(t *testing.T) {
	// FactPointTo.cpp:129–154 — stack local pointee → garbage
	f := &Function{Name: "f", ReturnType: GetIntType()}
	loc := CreateVariableScalars("l_t", GetIntType(), false, false)
	loc.Name = "l_t"
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	f.Body = body
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	ft := MakeFactPointTo(p, loc)
	nf := ft.MarkFuncEnd(f, body)
	if nf == nil || len(nf.PointTo) != 1 || nf.PointTo[0] != GarbagePtr {
		t.Fatalf("%+v", nf)
	}
	// non-stack target unchanged
	g := CreateVariableScalars("g_t", GetIntType(), true, false)
	ft2 := MakeFactPointTo(p, g)
	if ft2.MarkFuncEnd(f, body) != nil {
		t.Fatal("global pointee")
	}
	// nil pointee hole fails closed
	ft3 := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	if ft3.MarkFuncEnd(f, body) != nil {
		t.Fatal("nil PointTo hole must fail closed")
	}
	if ft.MarkFuncEndLocals([]*Variable{nil}) != nil {
		t.Fatal("nil locals hole must fail closed")
	}
	if ft3.MarkDeadVar(loc) != nil {
		t.Fatal("MarkDeadVar nil PointTo hole must fail closed")
	}
	// incomplete facts IsValidPtr fails closed
	if IsValidPtr(p, []*FactPointTo{nil}, 0, 0) {
		t.Fatal("IsValidPtr incomplete facts must fail closed")
	}
}

func TestRemoveFunctionLocalFactsMarksGarbage(t *testing.T) {
	// remaining global ptr that points at local → garbage after remove
	fn := &Function{Name: "f", ReturnType: GetIntType()}
	loc := CreateVariableScalars("l_t", GetIntType(), false, false)
	loc.Name = "l_t"
	body := &Block{Func: fn, LocalVars: []*Variable{loc}}
	fn.Blocks = []*Block{body}
	fn.Body = body
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	lp.Name = "l_p"
	body.LocalVars = append(body.LocalVars, lp)
	facts := []*FactPointTo{
		MakeFactPointTo(lp, NullPtr),
		MakeFactPointTo(gp, loc),
	}
	out := RemoveFunctionLocalFacts(facts, fn)
	if len(out) != 1 || out[0].Var != gp {
		t.Fatalf("%+v", out)
	}
	if len(out[0].PointTo) != 1 || out[0].PointTo[0] != GarbagePtr {
		t.Fatal("want garbage pointee", out[0].PointTo)
	}
}

func TestUpdateWithModifiedIndexNilPointee(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	idx := CreateVariableScalars("i", GetIntType(), false, false)
	if f.UpdateWithModifiedIndex(idx) != nil {
		t.Fatal("nil pointee hole must fail closed")
	}
	if VariablesComplete(MergePointeesOfPointers([]*Variable{nil}, nil)) {
		t.Fatal("nil ptr hole MergePointees must fail closed incomplete")
	}
}

func TestMergePointeesMissingFactFailClosed(t *testing.T) {
	// FactPointTo.cpp:694 assert(exist_fact) — missing related fact fails closed
	// (no invent soft-skip partial pointees / empty complete)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// facts empty / no related fact for p
	if VariablesComplete(MergePointeesOfPointers([]*Variable{p}, nil)) {
		t.Fatal("missing exist_fact must fail closed incomplete, not invent empty skip")
	}
	if VariablesComplete(MergePointeesOfPointers([]*Variable{p}, []*FactPointTo{})) {
		t.Fatal("empty facts without related must fail closed incomplete")
	}
	// incomplete fact map
	if VariablesComplete(MergePointeesOfPointers([]*Variable{p}, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil})) {
		t.Fatal("incomplete facts must fail closed incomplete")
	}
	// complete related fact still works
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	got := MergePointeesOfPointers([]*Variable{p}, []*FactPointTo{MakeFactPointTo(p, tgt)})
	if !VariablesComplete(got) || len(got) != 1 || got[0] != tgt {
		t.Fatalf("complete related fact: %+v", got)
	}
	// specials still skip without fact
	sp := MergePointeesOfPointers([]*Variable{NullPtr}, nil)
	if !VariablesComplete(sp) || len(sp) != 0 {
		t.Fatal("specials-only must yield empty complete, not fail closed", sp)
	}
}

func TestMergePointeesOfPointerPropagatesNil(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// indirect 1 with missing fact → incomplete (not invent empty step)
	if VariablesComplete(MergePointeesOfPointer(p, 1, nil)) {
		t.Fatal("missing fact at indir 1 must propagate incomplete")
	}
	// indirect 0 does not look up facts
	got := MergePointeesOfPointer(p, 0, nil)
	if !VariablesComplete(got) || len(got) != 1 || got[0] != p {
		t.Fatalf("indir0: %+v", got)
	}
}

func TestUpdateWithModifiedIndex(t *testing.T) {
	// FactPointTo.cpp:712–748 — a[i] → a[-1] when i modified
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	item.AsArray = item
	idx := CreateVariableScalars("i", GetIntType(), false, false)
	idx.Name = "i"
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := MakeFactPointTo(p, &item.Variable)
	nf := f.UpdateWithModifiedIndex(idx)
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
	j := CreateVariableScalars("j", GetIntType(), false, false)
	j.Name = "j"
	if f.UpdateWithModifiedIndex(j) != f {
		t.Fatal("j should not rewrite")
	}
	// bulk update
	facts := []*FactPointTo{f.Clone()}
	UpdateFactsWithModifiedIndex(&facts, idx)
	if facts[0] == f || facts[0].PointTo[0].AsArray.Indices[0] != "-1" {
		t.Fatal("bulk", facts[0])
	}
	// offset form "(i + 2)"
	item2 := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"(i + 2)"},
	}
	item2.AsArray = item2
	f2 := MakeFactPointTo(p, &item2.Variable)
	nf2 := f2.UpdateWithModifiedIndex(idx)
	if nf2 == f2 || nf2.PointTo[0].AsArray.Indices[0] != "-1" {
		t.Fatal("offset form", nf2)
	}
}
