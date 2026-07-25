package csmith

import "testing"

// TestCreateArrayVariablePointerPrimaryNullFact — VariableSelector.cpp:1364 + Fact.cpp:94–106.
// Constant::make_random(pointer) is always "0"; AddNewVarFact must record null/may-null.
func TestCreateArrayVariablePointerPrimaryNullFact(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	opts := Defaults()
	probs := NewProbabilities(opts)
	r := NewRng(42)
	elem := PointerTo(GetIntType())
	init := MakeRandom(elem, opts, probs, r)
	if init == nil || init.Value != "0" || init.Type == nil || !init.Type.IsPointerLike() {
		t.Fatalf("pointer MakeRandom: %+v", init)
	}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, probs, nil, nil, nil, "l_233", elem, init, q)
	if av == nil || HasErrorSess(testAmbientSession) {
		t.Fatalf("create err=%v", HasErrorSess(testAmbientSession))
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.AddNewVarFact(&av.Variable)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(HasErrorSess(testAmbientSession))
	}
	got := FindRelatedPointTo(fm.GlobalFacts, &av.Variable)
	if got == nil || !got.IsNull() {
		t.Fatalf("primary pointer-0 must yield null/may-null, got %+v", got)
	}
	ClearErrorSess(testAmbientSession)
}

// TestPostLoopRestoresEntryMayNullNotOut — StatementFor.cpp:355–357.
// post_loop installs map_facts_in[body], not map_facts_out. Entry may-null
// (pointer-array init / self-back) must not be replaced by mid-gen definitive-only out.
func TestPostLoopRestoresEntryMayNullNotOut(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "func_54", ReturnType: GetIntType()}
	ptType := PointerTo(GetSimpleType(EShort))
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", GetSimpleType(EShort), false, false)
	arr := &ArrayVariable{
		Variable: Variable{Name: "l_233", Type: ptType, IsArray: true},
		Sizes:    []int{10},
	}
	arr.AsArray = arr
	entryMay := MakeFactPointToSet(&arr.Variable, []*Variable{g, NullPtr})
	outDef := MakeFactPointTo(&arr.Variable, g)
	body := &Block{StmID: 25, Func: f, Looping: true}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.SetMapFactsIn(25, []*FactPointTo{entryMay})
	fm.SetMapFactsOut(25, []*FactPointTo{outDef})
	fm.GlobalFacts = []*FactPointTo{outDef}
	forSt := &Stmt{Kind: StmtFor, StmID: 24, Then: body}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	postLoopAnalysis(fm, forSt, body, []*FactPointTo{outDef}, nil, EmptyEffect(), &cg)
	got := FindRelatedPointTo(fm.GlobalFacts, &arr.Variable)
	if got == nil || !got.IsNull() {
		t.Fatalf("post_loop must install map_in may-null (not map_out definitive): %+v", got)
	}
	ClearErrorSess(testAmbientSession)
}

// TestFindFixedPointAfterResetKeepsEntryMayNull — Block.cpp:703 facts_copy + 719 reset.
// After reset_stm_fact_maps, re-enter find_fixed_point with the pre-loop facts_copy
// (may-null from entry / prior self-back). map_facts_in must be reinstalled with it.
func TestFindFixedPointAfterResetKeepsEntryMayNull(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "f", ReturnType: GetIntType()}
	ptType := PointerTo(GetIntType())
	g := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntType(), false, false)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", ptType, false, false)
	entry := []*FactPointTo{MakeFactPointToSet(p, []*Variable{g, NullPtr})}
	x := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), false, false)
	asg := Stmt{
		Kind: StmtAssign, StmID: 2,
		LhsVar: x, Lhs: &Lhs{Var: x, Type: GetIntType()},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()},
		AssignOp: AssignSimple,
	}
	b := &Block{StmID: 1, Func: f, Looping: true, Stmts: []Stmt{asg}}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.SetMapFactsIn(1, entry)
	fm.MapVisited = map[int]bool{1: true}
	fm.ResetBlockFactMaps(b)
	factsCopy := CloneFactSlice(entry)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_, _, _, ok := FindFixedPointBlock(b, factsCopy, &cg, Defaults(), true)
	if !ok {
		t.Fatalf("FP after reset must succeed err=%v", HasErrorSess(testAmbientSession))
	}
	got := FindRelatedPointTo(fm.GetMapFactsIn(1), p)
	if got == nil || !got.IsNull() {
		t.Fatalf("map_in after FP must keep entry may-null: %+v", got)
	}
	ClearErrorSess(testAmbientSession)
}
