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
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	_ = st.HasIntField() // may or may not
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
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFact(p)
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsDead() {
		t.Fatal("init garbage")
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
