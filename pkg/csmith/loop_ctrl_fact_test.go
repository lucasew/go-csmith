package csmith

import "testing"

func TestHasIntFieldAndContainPointer(t *testing.T) {
	if !GetIntType().HasIntField() {
		t.Fatal("int")
	}
	if GetIntType().ContainPointerField() {
		t.Fatal("int no ptr")
	}
	pt := PointerTo(GetIntType())
	if !pt.ContainPointerField() {
		t.Fatal("ptr")
	}
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	_ = st.HasIntField() // may or may not
	// nil field Type holes sticky fail closed
	ClearError()
	hole := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil, BitWidth: -1}}}
	if hole.HasIntField() {
		t.Fatal("nil field Type must not invent has-int")
	}
	if !HasError() {
		t.Fatal("nil field Type HasIntField must SetError sticky")
	}
	ClearError()
	if !hole.ContainPointerField() {
		t.Fatal("nil field Type must fail closed as has-pointer")
	}
	if !HasError() {
		t.Fatal("nil field Type ContainPointerField must SetError sticky")
	}
	ClearError()
	if !hole.IsConstStructUnion() {
		t.Fatal("nil field Type must fail closed as const")
	}
	if !HasError() {
		t.Fatal("nil field Type IsConstStructUnion must SetError sticky")
	}
	ClearError()
	if !hole.IsVolatileStructUnion() {
		t.Fatal("nil field Type must fail closed as volatile")
	}
	if !HasError() {
		t.Fatal("nil field Type IsVolatileStructUnion must SetError sticky")
	}
	ClearError()
	if !hole.HasBitfields() {
		t.Fatal("nil field Type must fail closed as has-bitfields")
	}
	if !HasError() {
		t.Fatal("nil field Type HasBitfields must SetError sticky")
	}
	ClearError()
}

func TestIsVisibleLocal(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	outer := &Block{Func: f}
	inner := &Block{Parent: outer, Func: f}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	if l == nil {
		t.Fatal("loc")
	}
	outer.LocalVars = []*Variable{l}
	if !l.IsVisibleLocal(inner) {
		t.Fatal("outer local visible from inner")
	}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if g.IsVisibleLocal(inner) {
		t.Fatal("global not local-visible")
	}
	p := CreateVariableScalars("p_1", GetIntType(), false, false)
	f.Param = []*Variable{p}
	if !p.IsVisibleLocal(inner) {
		t.Fatal("param")
	}
	if HasError() {
		t.Fatal("complete IsVisibleLocal must not sticky")
	}
	// nil Param/Local holes sticky fail closed
	ClearError()
	f.Param = []*Variable{nil, p}
	if p.IsVisibleLocal(inner) {
		t.Fatal("nil Param hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Param hole IsVisibleLocal must SetError sticky")
	}
	ClearError()
	f.Param = []*Variable{p}
	outer.LocalVars = []*Variable{nil, l}
	if l.IsVisibleLocal(inner) {
		t.Fatal("nil LocalVars hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil LocalVars hole IsVisibleLocal must SetError sticky")
	}
	ClearError()
	if f.IsVarOnStack(l, inner) {
		t.Fatal("IsVarOnStack nil local hole must fail closed")
	}
	if !HasError() {
		t.Fatal("IsVarOnStack LocalVars hole must SetError sticky")
	}
	ClearError()
}

func TestIsPointingToLocalsArrayUsesCollective(t *testing.T) {
	// FactPointTo.cpp:506–508 — isArray / is_array_field → get_collective before fact lookup.
	// Itemized array members share the collective's points-to; without the collective
	// step, FindRelatedPointTo misses and as_return allows local-pointing elems
	// (seed-4: return l_897[…] while UP rejects and keeps selecting).
	ClearError()
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{LocalVars: []*Variable{loc}}
	// collective array of int* whose fact points at a local
	elemT := PointerTo(GetIntType())
	collAV := &ArrayVariable{
		Variable: Variable{Name: "l_arr", Type: elemT, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	collAV.AsArray = collAV
	collAV.IsArray = true
	item := collAV.ItemizeConstIndices([]int{1}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	// facts keyed on collective only (C++ style)
	facts := []*FactPointTo{MakeFactPointTo(&collAV.Variable, loc)}
	// direct collective: pointing to local
	if !IsPointingToLocals(&collAV.Variable, blk, 0, facts) {
		t.Fatal("collective array fact must detect local pointee")
	}
	// itemized member must use collective — same answer
	if !IsPointingToLocals(&item.Variable, blk, 0, facts) {
		t.Fatal("itemized array must use collective points-to (FactPointTo.cpp:506–508)")
	}
	// as_return ExpressionVariable must reject itemized local-pointing array
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: elemT, RV: CreateVariableScalars("rv", elemT, false, false)}
	f.Stack = []*Block{blk}
	blk.Func = f
	blk.LocalVars = []*Variable{loc, &collAV.Variable}
	fm := NewFactMgr(f)
	fm.GlobalFacts = facts
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// Force pool: only the array is choosable as int*
	vs.GlobalList = nil
	// Local array available via block
	for seed := uint64(1); seed < 40; seed++ {
		ClearError()
		ev := makeExpressionVariableFlags(NewRng(seed), vs, &cg, elemT, nil, false, true)
		if ev != nil && (ev.Var == &item.Variable || ev.Var == &collAV.Variable ||
			(ev.Var != nil && ev.Var.AsArray != nil && ev.Var.AsArray.Collective == collAV) ||
			(ev.Var != nil && ev.Var.Name == "l_arr")) {
			t.Fatalf("as_return must not accept local-pointing array seed=%d var=%v", seed, ev.Var)
		}
	}
	ClearError()
}

func TestIsPointingToLocalsNilHole(t *testing.T) {
	ClearError()
	// Variable always live; sticky true (no invent not-local soft-skip)
	if !IsPointingToLocals(nil, &Block{}, 0, nil) {
		t.Fatal("nil Variable IsPointingToLocals must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil Variable IsPointingToLocals must SetError sticky")
	}
	ClearError()
	ptr := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// incomplete PointTo sticky as pointing-to-locals
	facts := []*FactPointTo{{Var: ptr, PointTo: []*Variable{nil}}}
	if !IsPointingToLocals(ptr, &Block{}, 0, facts) {
		t.Fatal("nil pointee hole must fail closed as pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("nil pointee IsPointingToLocals must SetError sticky")
	}
	ClearError()
	// incomplete map hole sticky true
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	holeMap := []*FactPointTo{nil, MakeFactPointTo(ptr, loc)}
	if !IsPointingToLocals(ptr, blk, 0, holeMap) {
		t.Fatal("incomplete map must fail closed as pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("incomplete map IsPointingToLocals must SetError sticky")
	}
	ClearError()
	// incomplete stack sticky true
	badBlk := &Block{LocalVars: []*Variable{loc, nil}}
	okFacts := []*FactPointTo{MakeFactPointTo(ptr, loc)}
	if !IsPointingToLocals(ptr, badBlk, 0, okFacts) {
		t.Fatal("incomplete stack must fail closed as pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("incomplete stack IsPointingToLocals must SetError sticky")
	}
	ClearError()
	// Type-nil subject soft invent: IsPointer residual ERROR+false → not-local
	// fair: sticky true (restrictive) before classify
	shell := &Variable{Name: "g_typeless"}
	if !IsPointingToLocals(shell, blk, 0, []*FactPointTo{}) {
		t.Fatal("Type-nil subject must fail closed as pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("Type-nil subject IsPointingToLocals must SetError sticky")
	}
	ClearError()
	// Type-nil non-special pointee: IsPointer residual false skips recurse → invent not-local
	// fair: sticky true before IsPointer gate
	tyNilPointee := &Variable{Name: "l_typeless"}
	factsTy := []*FactPointTo{MakeFactPointTo(ptr, tyNilPointee)}
	if !IsPointingToLocals(ptr, blk, 0, factsTy) {
		t.Fatal("Type-nil pointee must fail closed as pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("Type-nil pointee IsPointingToLocals must SetError sticky")
	}
	ClearError()
	// IsVisibleLocal residual: incomplete LocalVars on parent soft invent was not-local then invent false.
	// Fair: sticky true (restrictive pointing-to-locals).
	// Use indirection -1 path (direct IsVisibleLocal).
	loc2 := CreateVariableScalars("l_2", GetIntType(), false, false)
	loc2.Name = "l_2"
	badParent := &Block{LocalVars: []*Variable{loc2, nil}}
	if !IsPointingToLocals(loc2, badParent, -1, nil) {
		t.Fatal("IsVisibleLocal residual must fail closed true pointing-to-locals")
	}
	if !HasError() {
		t.Fatal("IsVisibleLocal residual IsPointingToLocals must SetError sticky")
	}
	ClearError()
}

func TestIsPointingToLocals(t *testing.T) {
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk.LocalVars = []*Variable{loc}
	ptr := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// fact points to local → true
	facts := []*FactPointTo{MakeFactPointTo(ptr, loc)}
	if !IsPointingToLocals(ptr, blk, 0, facts) {
		t.Fatal("points to local")
	}
	// points to null → false
	facts = []*FactPointTo{MakeFactPointTo(ptr, NullPtr)}
	if IsPointingToLocals(ptr, blk, 0, facts) {
		t.Fatal("null")
	}
}

func TestSelectLoopCtrlVarFiltersUnionPtr(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// plain int global
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	v := vs.SelectLoopCtrlVar(NewRng(2), cg, nil)
	if v == nil {
		t.Fatal("nil")
	}
}

func TestSelectLoopCtrlVarIncompleteAmbientSticky(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect())
	cg.EffectAccum = &inc
	if vs.SelectLoopCtrlVar(NewRng(1), cg, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed SelectLoopCtrlVar")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if vs.SelectLoopCtrlVar(NewRng(2), cg2, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed SelectLoopCtrlVar")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestSelectLoopCtrlVarHasIntFieldResidualSticky(t *testing.T) {
	// Type-nil field: HasIntField stickies residual ERROR+false.
	// Soft invent was soft-continue filter then ChooseVarFull/GenerateNewGlobal later good int.
	// Fair: sticky fail closed whole SelectLoopCtrlVar.
	ClearError()
	defer ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	holeTy := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil, BitWidth: -1}}}
	hole := &Variable{Name: "g_hole", Type: holeTy}
	good := &Variable{Name: "g_1", Type: GetIntType()}
	vs.GlobalList = []*Variable{hole, good}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	if vs.SelectLoopCtrlVar(NewRng(2), cg, nil) != nil {
		t.Fatal("HasIntField residual must fail closed SelectLoopCtrlVar, not invent later good")
	}
	if !HasError() {
		t.Fatal("HasIntField residual SelectLoopCtrlVar must SetError sticky")
	}
	ClearError()
	// Union + int field then Type-nil: HasIntField true, ContainPointerField residual.
	// Soft invent was soft-continue filter then pick later good int.
	// Fair: sticky fail closed whole SelectLoopCtrlVar.
	unionOKThenHole := &Type{isUnion: true, Fields: []StructField{
		{Name: "i", Type: GetIntType(), BitWidth: -1},
		{Name: "x", Type: nil, BitWidth: -1},
	}}
	uvar := &Variable{Name: "g_u", Type: unionOKThenHole}
	vs2 := NewVariableSelector(opts)
	vs2.GlobalList = []*Variable{uvar, good}
	f2 := &Function{Name: "f", ReturnType: GetIntType()}
	blk2 := &Block{Func: f2}
	f2.Stack = []*Block{blk2}
	cg3 := WithFunc(f2, EmptyEffect())
	if vs2.SelectLoopCtrlVar(NewRng(3), cg3, nil) != nil {
		t.Fatal("ContainPointer residual must fail closed SelectLoopCtrlVar, not invent later good")
	}
	if !HasError() {
		t.Fatal("ContainPointer residual SelectLoopCtrlVar must SetError sticky")
	}
	ClearError()
}

func TestAddNewVarFactPointer(t *testing.T) {
	f := &Function{Name: "f"}
	fm := NewFactMgr(f)
	// Variable.cpp:395 — pointer init Constant::make_random → "0" → null fact
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFact(p)
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || !fp.IsNull() {
		t.Fatalf("init null from make_random pointer, got %+v", fp)
	}
	// idempotent
	fm.AddNewVarFact(p)
	if len(fm.GlobalFacts) != 1 {
		t.Fatal("dup")
	}
}

func TestMakeExpressionVariableAsReturnFiltersLocalPtr(t *testing.T) {
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk.LocalVars = []*Variable{loc}
	// local pointer that points to local
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	blk.LocalVars = append(blk.LocalVars, lp)
	fm := NewFactMgr(f)
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointTo(lp, loc))
	// also a global pointer to global target (ok to return)
	gt := CreateVariableScalars("g_t", GetIntType(), false, false)
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	vs.GlobalList = []*Variable{gp, gt}
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointTo(gp, gt))
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// many tries — should never return the local-pointing ptr when filter works
	for seed := uint64(1); seed < 30; seed++ {
		ev := makeExpressionVariableFlags(NewRng(seed), vs, &cg, PointerTo(GetIntType()), nil, false, true)
		if ev != nil && ev.Var == lp {
			t.Fatalf("returned local-pointing ptr seed=%d", seed)
		}
	}
}
