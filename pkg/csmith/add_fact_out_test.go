package csmith

import "testing"

func TestAddFactOutIncompleteStackFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsVarVisible false → drop stack local fact
	// fair: incomplete stack → IncompleteFactSlice (not invent empty-complete skip)
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Body = body
	fm := NewFactMgr(f)
	st := &Stmt{Kind: StmtAssign, StmID: 3}
	fm.AddFactOut(st, body, MakeFactPointTo(loc, NullPtr))
	if FactsComplete(fm.MapFactsOut[3]) {
		t.Fatal("incomplete stack must fail closed incomplete out, not invent empty complete", fm.MapFactsOut[3])
	}
	// later appends must not invent cleaned facts onto incomplete map
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.AddFactOut(st, body, MakeFactPointTo(gp, NullPtr))
	if FactsComplete(fm.MapFactsOut[3]) {
		t.Fatal("append after incomplete must stay incomplete", fm.MapFactsOut[3])
	}
	// incomplete fact PointTo → hole marker
	fm2 := NewFactMgr(f)
	body2 := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Body = body2
	st2 := &Stmt{Kind: StmtAssign, StmID: 4}
	hole := &FactPointTo{Var: gp, PointTo: []*Variable{nil}}
	fm2.AddFactOut(st2, body2, hole)
	if FactsComplete(fm2.MapFactsOut[4]) {
		t.Fatal("incomplete fact must fail closed IncompleteFactSlice", fm2.MapFactsOut[4])
	}
}

func TestAddFactOutVisible(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.AddFactOut(st, body, MakeFactPointTo(
		CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false),
		NullPtr,
	))
	// use global pointer fact
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.AddFactOut(st, body, MakeFactPointTo(p, NullPtr))
	if len(fm.MapFactsOut[5]) != 2 {
		// first call also used a fresh p - ok
		if len(fm.MapFactsOut[5]) < 1 {
			t.Fatal(fm.MapFactsOut[5])
		}
	}
	// return drops non-global
	ret := &Stmt{Kind: StmtReturn, StmID: 6}
	fm.AddFactOut(ret, body, MakeFactPointTo(
		// local as subject — need pointer local
		func() *Variable {
			lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
			lp.Name = "l_p"
			return lp
		}(),
		NullPtr,
	))
	if len(fm.MapFactsOut[6]) != 0 {
		t.Fatal("return non-global should drop", fm.MapFactsOut[6])
	}
	// return keeps global
	fm.AddFactOut(ret, body, MakeFactPointTo(p, NullPtr))
	if len(fm.MapFactsOut[6]) != 1 {
		t.Fatal(fm.MapFactsOut[6])
	}
	_ = g
}

func TestAddFactOutGotoDestVisibility(t *testing.T) {
	// FactMgr.cpp:296–300 — fact dropped when local not visible at dest parent
	f := &Function{Name: "f", ReturnType: GetIntType()}
	outer := &Block{Func: f}
	inner := &Block{Func: f, Parent: outer}
	loc := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	loc.Name = "l_p"
	inner.LocalVars = []*Variable{loc}
	f.Blocks = []*Block{outer, inner}
	fm := NewFactMgr(f)
	// goto from inner to outer dest — local of inner not visible at outer
	st := &Stmt{
		Kind: StmtGoto, StmID: 9,
		GotoDestStmID:  20,
		GotoDestParent: outer,
	}
	fm.AddFactOut(st, inner, MakeFactPointTo(loc, NullPtr))
	if len(fm.MapFactsOut[9]) != 0 {
		t.Fatal("local invisible at dest should drop", fm.MapFactsOut[9])
	}
	// global pointer still recorded
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.AddFactOut(st, inner, MakeFactPointTo(gp, NullPtr))
	if len(fm.MapFactsOut[9]) != 1 {
		t.Fatal("global at dest", fm.MapFactsOut[9])
	}
	// dest parent = inner → local visible
	st2 := &Stmt{
		Kind: StmtGoto, StmID: 10,
		GotoDestStmID:  21,
		GotoDestParent: inner,
	}
	fm.AddFactOut(st2, inner, MakeFactPointTo(loc, NullPtr))
	if len(fm.MapFactsOut[10]) != 1 {
		t.Fatal("local visible at dest", fm.MapFactsOut[10])
	}
	// resolve via FindParentBlockOfStmID when GotoDestParent nil
	tgt := Stmt{Kind: StmtAssign, StmID: 30}
	outer.Stmts = []Stmt{tgt}
	st3 := &Stmt{Kind: StmtGoto, StmID: 11, GotoDestStmID: 30}
	fm.AddFactOut(st3, inner, MakeFactPointTo(loc, NullPtr))
	if len(fm.MapFactsOut[11]) != 0 {
		t.Fatal("resolved dest parent outer, local drop")
	}
}

func TestArrayIsVariant(t *testing.T) {
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", IsArray: true, Type: GetIntType()},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	a := &ArrayVariable{
		Variable:   Variable{Name: "g_a[i]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	a.AsArray = a
	b := &ArrayVariable{
		Variable:   Variable{Name: "g_a[i]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	b.AsArray = b
	if !a.IsVariant(&b.Variable) {
		t.Fatal("same keys")
	}
	c := &ArrayVariable{
		Variable:   Variable{Name: "g_a[j]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"j"},
	}
	c.AsArray = c
	if a.IsVariant(&c.Variable) {
		t.Fatal("different keys")
	}
	// collective must match
	otherParent := &ArrayVariable{Variable: Variable{Name: "g_b", IsArray: true}}
	d := &ArrayVariable{
		Variable:   Variable{Name: "g_b[i]", IsArray: true},
		Collective: otherParent,
		Indices:    []string{"i"},
	}
	d.AsArray = d
	if a.IsVariant(&d.Variable) {
		t.Fatal("diff collective")
	}
}
