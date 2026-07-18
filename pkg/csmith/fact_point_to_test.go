package csmith

import "testing"

func TestFactPointToNullDead(t *testing.T) {
	p := CreateVariableScalars("g_p", GetIntType(), false, false)
	// default NewFactPointTo starts garbage
	f := NewFactPointTo(p)
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
	if !NullPtr.IsVirtual() {
		t.Fatal("virtual")
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
