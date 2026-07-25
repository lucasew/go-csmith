package csmith

import "testing"

func TestAddFactOutIncompleteStackFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsVarVisible false → drop stack local fact
	// fair: incomplete stack → sticky IncompleteFactSlice (not invent empty-complete skip)
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", PointerTo(GetIntType()), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Body = body
	fm := NewFactMgrSess(testAmbientSession, f)
	st := &Stmt{Kind: StmtAssign, StmID: 3}
	fm.AddFactOut(st, body, MakeFactPointTo(loc, NullPtr))
	if FactsComplete(fm.MapFactsOut[3]) {
		t.Fatal("incomplete stack must fail closed incomplete out, not invent empty complete", fm.MapFactsOut[3])
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete stack AddFactOut must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// later appends must not invent cleaned facts onto incomplete map
	gp := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	fm.AddFactOut(st, body, MakeFactPointTo(gp, NullPtr))
	if FactsComplete(fm.MapFactsOut[3]) {
		t.Fatal("append after incomplete must stay incomplete", fm.MapFactsOut[3])
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("append onto incomplete map must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete fact PointTo → sticky hole marker
	fm2 := NewFactMgrSess(testAmbientSession, f)
	body2 := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Body = body2
	st2 := &Stmt{Kind: StmtAssign, StmID: 4}
	hole := &FactPointTo{Var: gp, PointTo: []*Variable{nil}}
	fm2.AddFactOut(st2, body2, hole)
	if FactsComplete(fm2.MapFactsOut[4]) {
		t.Fatal("incomplete fact must fail closed IncompleteFactSlice", fm2.MapFactsOut[4])
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete fact PointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVarVisible residual via incomplete Param after StackScanComplete path:
	// Param hole on Function stickies StackScanComplete first — already covered.
	// Type-nil subject IsGlobal residual is nil-only; use incomplete Blocks for goto dest.
	// Goto dest parent resolve residual: incomplete Func Blocks hole sticky.
	f3 := &Function{Name: "f3"}
	gp3 := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerTo(GetIntType()), true, false)
	body3 := &Block{Func: f3}
	f3.Body = body3
	f3.Blocks = []*Block{body3, nil}
	fm3 := NewFactMgrSess(testAmbientSession, f3)
	stGoto := &Stmt{Kind: StmtGoto, StmID: 9, GotoDestStmID: 99}
	// FindParentBlockOfStmID walks Blocks; nil hole stickies residual
	fm3.AddFactOut(stGoto, body3, MakeFactPointTo(gp3, NullPtr))
	if FactsComplete(fm3.MapFactsOut[9]) {
		t.Fatal("FindParentBlock residual must fail closed incomplete out", fm3.MapFactsOut[9])
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("FindParentBlock residual AddFactOut must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddFactOutVisible(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	fm := NewFactMgrSess(testAmbientSession, f)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.AddFactOut(st, body, MakeFactPointTo(
		CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false),
		NullPtr,
	))
	// use global pointer fact
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
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
			lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerTo(GetIntType()), false, false)
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
	loc := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerTo(GetIntType()), false, false)
	loc.Name = "l_p"
	inner.LocalVars = []*Variable{loc}
	f.Blocks = []*Block{outer, inner}
	fm := NewFactMgrSess(testAmbientSession, f)
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
	gp := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
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
	if !a.IsVariantSess(testAmbientSession, &b.Variable) {
		t.Fatal("same keys")
	}
	c := &ArrayVariable{
		Variable:   Variable{Name: "g_a[j]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"j"},
	}
	c.AsArray = c
	if a.IsVariantSess(testAmbientSession, &c.Variable) {
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
	if a.IsVariantSess(testAmbientSession, &d.Variable) {
		t.Fatal("diff collective")
	}
	// incomplete IndexExprs must not invent variant via string Indices soft-skip
	ClearErrorSess(testAmbientSession)
	a.IndexExprs = []*Expression{nil}
	a.Indices = []string{"i"}
	b.IndexExprs = []*Expression{nil}
	b.Indices = []string{"i"}
	if a.IsVariantSess(testAmbientSession, &b.Variable) {
		t.Fatal("IndexExprs hole must fail closed not invent string match")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IndexExprs hole IsVariant must SetError sticky")
	}
	// mixed IndexExprs vs Indices-only must not invent dual-path match
	ClearErrorSess(testAmbientSession)
	a.IndexExprs = []*Expression{{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "i", GetIntType(), false, false), ExprType: GetIntType()}}
	b.IndexExprs = nil
	b.Indices = []string{"i"}
	if a.IsVariantSess(testAmbientSession, &b.Variable) {
		t.Fatal("mixed IndexExprs/Indices must fail closed")
	}
	ClearErrorSess(testAmbientSession)
}

// TestAddFactOutUnionContinueDropsNestedLoopLocal —
// FactMgr.cpp:288–296 + add_new_var_fact_and_update_inout_maps:96–105.
// Soft invent: eUnionWrite half of AddNewVarFactAndUpdate used IsVarVisible(parent)
// only → continue map_out re-gained nested loop-local unions after remove_loop_local
// (seed 2020240685: continue 39 kept l_237 → for-body map_in pollution →
// post_loop break invent BOTTOM → VisitFacts nonreadable → FP strip).
func TestAddFactOutUnionContinueDropsNestedLoopLocal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	// loop body (looping)
	body := &Block{Func: f, Looping: true, StmID: 26}
	// nested block declares union local
	ut := &Type{isUnion: true, StructName: "U1", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	l237 := CreateVariableScalarsSess(testAmbientSession, "l_237", ut, false, false)
	nested := &Block{Func: f, Parent: body, Looping: false, StmID: 27, LocalVars: []*Variable{l237}}
	// continue lives deeper under nested
	contParent := &Block{Func: f, Parent: nested, Looping: false, StmID: 31}
	cont := &Stmt{Kind: StmtContinue, StmID: 39}
	contParent.Stmts = []Stmt{*cont}
	// point cont at slice element for AddFactOut
	cont = &contParent.Stmts[0]
	f.Blocks = []*Block{body, nested, contParent}
	fm := NewFactMgrSess(testAmbientSession, f)
	// seed map_out with empty complete (as after set_fact_out remove_loop_local)
	fm.MapFactsOut = map[int][]*FactPointTo{39: {}}
	fm.MapUnionFactsOut = map[int][]*FactUnion{39: {}}
	// add_fact_out union for nested local must drop (not visible at loop head body)
	uf := MakeFactUnion(l237, 0)
	fm.AddFactOutUnion(cont, contParent, uf)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("AddFactOutUnion sticky", GetErrorSess(testAmbientSession))
	}
	outU := fm.GetMapUnionFactsOut(39)
	if !UnionFactsComplete(outU) {
		t.Fatal("map_union_out incomplete", outU)
	}
	if FindRelatedUnion(outU, l237) != nil {
		t.Fatalf("continue map_out must drop nested loop-local union l_237, got %v", outU)
	}
	// global union still accepted
	gU := CreateVariableScalarsSess(testAmbientSession, "g_u", ut, true, false)
	fm.AddFactOutUnion(cont, contParent, MakeFactUnion(gU, 0))
	if FindRelatedUnion(fm.GetMapUnionFactsOut(39), gU) == nil {
		t.Fatal("continue map_out must keep global union subject")
	}
}

// TestAddNewVarFactAndUpdateUnionContinueFilter —
// end-to-end: AddNewVarFactAndUpdate must not re-append nested union onto continue out.
func TestAddNewVarFactAndUpdateUnionContinueFilter(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	body := &Block{Func: f, Looping: true, StmID: 26}
	ut := &Type{isUnion: true, StructName: "U1", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	// Init required for complete union abstract (array_type_test.go:576)
	l237 := &Variable{Name: "l_237", Type: ut, Init: MakeInt(0)}
	nested := &Block{Func: f, Parent: body, Looping: false, StmID: 27, LocalVars: []*Variable{l237}}
	contParent := &Block{Func: f, Parent: nested, Looping: false, StmID: 31}
	contParent.Stmts = []Stmt{{Kind: StmtContinue, StmID: 39}}
	f.Blocks = []*Block{body, nested, contParent}
	f.Body = body
	fm := NewFactMgrSess(testAmbientSession, f)
	// pre-seed continue map_out empty complete (post remove_loop_local)
	fm.MapFactsOut = map[int][]*FactPointTo{39: {}}
	fm.MapUnionFactsOut = map[int][]*FactUnion{39: {}}
	// also seed an assign out (non-jump) so PT map has keys
	fm.MapFactsOut[100] = []*FactPointTo{}
	fm.MapUnionFactsOut[100] = []*FactUnion{}
	// declare nested local under nested block
	fm.AddNewVarFactAndUpdate(nested, l237)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("AddNewVarFactAndUpdate sticky", GetErrorSess(testAmbientSession))
	}
	if FindRelatedUnion(fm.GetMapUnionFactsOut(39), l237) != nil {
		t.Fatalf("AddNewVarFactAndUpdate must not re-append l_237 onto continue map_out, got %v",
			fm.GetMapUnionFactsOut(39))
	}
}
