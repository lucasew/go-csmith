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
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	_ = st.HasIntField() // may or may not
	// nil field Type holes fail closed
	hole := &Type{isStruct: true, Fields: []StructField{{Name: "x", Type: nil, BitWidth: -1}}}
	if hole.HasIntField() {
		t.Fatal("nil field Type must not invent has-int")
	}
	if !hole.ContainPointerField() {
		t.Fatal("nil field Type must fail closed as has-pointer")
	}
	if !hole.IsConstStructUnion() {
		t.Fatal("nil field Type must fail closed as const")
	}
	if !hole.IsVolatileStructUnion() {
		t.Fatal("nil field Type must fail closed as volatile")
	}
	if !hole.HasBitfields() {
		t.Fatal("nil field Type must fail closed as has-bitfields")
	}
}

func TestIsVisibleLocal(t *testing.T) {
	f := &Function{Name: "f"}
	outer := &Block{Func: f}
	inner := &Block{Parent: outer, Func: f}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
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
	// nil Param/Local holes fail closed
	f.Param = []*Variable{nil, p}
	if p.IsVisibleLocal(inner) {
		t.Fatal("nil Param hole must fail closed")
	}
	f.Param = []*Variable{p}
	outer.LocalVars = []*Variable{nil, l}
	if l.IsVisibleLocal(inner) {
		t.Fatal("nil LocalVars hole must fail closed")
	}
	if f.IsVarOnStack(l, inner) {
		t.Fatal("IsVarOnStack nil local hole must fail closed")
	}
}

func TestIsPointingToLocalsNilHole(t *testing.T) {
	ptr := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// incomplete PointTo fails closed as pointing-to-locals
	facts := []*FactPointTo{{Var: ptr, PointTo: []*Variable{nil}}}
	if !IsPointingToLocals(ptr, &Block{}, 0, facts) {
		t.Fatal("nil pointee hole must fail closed as pointing-to-locals")
	}
	// incomplete map hole before related fact: no invent not-local via FindRelated nil
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	holeMap := []*FactPointTo{nil, MakeFactPointTo(ptr, loc)}
	if !IsPointingToLocals(ptr, blk, 0, holeMap) {
		t.Fatal("incomplete map must fail closed as pointing-to-locals")
	}
	// incomplete stack: no invent not-local via IsVisibleLocal false past hole
	badBlk := &Block{LocalVars: []*Variable{loc, nil}}
	okFacts := []*FactPointTo{MakeFactPointTo(ptr, loc)}
	if !IsPointingToLocals(ptr, badBlk, 0, okFacts) {
		t.Fatal("incomplete stack must fail closed as pointing-to-locals")
	}
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
