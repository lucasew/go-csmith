package csmith

import "testing"

func TestChooseOKVarItemizeFailClosed(t *testing.T) {
	// VariableSelector.cpp:332–337 — collective array must itemize(); no return bare
	ClearErrorSess(testAmbientSession)
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), Defaults(), NewProbabilities(Defaults()), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("create")
	}
	// single collective with nil r → sticky fail closed (itemize needs RNG)
	if ChooseOKVar(nil, []*Variable{&av.Variable}) != nil {
		t.Fatal("itemize needs RNG; no soft return collective")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG itemize ChooseOKVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// multi-cand without RNG sticky — no invent vars[0]
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	if ChooseOKVar(nil, []*Variable{a, b}) != nil {
		t.Fatal("nil RNG multi ChooseOKVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG multi ChooseOKVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	got := ChooseOKVar(NewRngSess(testAmbientSession, 3), []*Variable{&av.Variable})
	if got == nil || got.AsArray == nil || got.AsArray.Collective != av {
		t.Fatalf("want itemized member, got %+v", got)
	}
}

func TestChooseOKVarSoleAndUpto(t *testing.T) {
	// VariableSelector::choose_ok_var
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_1", GetSimpleTypeSess(testAmbientSession, EInt), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_2", GetSimpleTypeSess(testAmbientSession, EInt), false, false)
	c := CreateVariableScalarsSess(testAmbientSession, "g_3", GetSimpleTypeSess(testAmbientSession, EInt), false, false)

	if ChooseOKVar(NewRngSess(testAmbientSession, 2), nil) != nil {
		t.Fatal("empty")
	}
	// nil hole fails closed sticky — no invent skip as absent candidate
	if ChooseOKVar(NewRngSess(testAmbientSession, 2), []*Variable{a, nil, b}) != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ChooseOKVar(NewRngSess(testAmbientSession, 2), []*Variable{a}) != a {
		t.Fatal("sole")
	}
	// seed2 first RndUpto(3) = 1959434203 % 3
	r := NewRngSess(testAmbientSession, 2)
	wantIdx := int(r.RndUptoSess(testAmbientSession, 3))
	r2 := NewRngSess(testAmbientSession, 2)
	got := ChooseOKVar(r2, []*Variable{a, b, c})
	want := []*Variable{a, b, c}[wantIdx]
	if got != want {
		t.Fatalf("choose_ok_var: got %v want %v (idx %d)", got.Name, want.Name, wantIdx)
	}
}

func TestChooseOKVarMatchTypeNilSticky(t *testing.T) {
	// Type-nil candidate: soft invent was soft-skip as absent then pick later good.
	// Fair: sticky fail closed whole ChooseOKVarMatch.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	good := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	hole := &Variable{Name: "g_hole", Type: nil}
	if ChooseOKVarMatch(NewRngSess(testAmbientSession, 1), []*Variable{hole, good}, GetIntTypeSess(testAmbientSession), MatchFlexible, false) != nil {
		t.Fatal("Type-nil candidate must fail closed ChooseOKVarMatch, not invent later good")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil candidate ChooseOKVarMatch must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateNewGlobalNamesAndList(t *testing.T) {
	// GenerateNewGlobal: gensym g_1, push GlobalList, random_qualifiers draws.
	ResetDefaultGensym()
	opts := Defaults()
	// force scalar path — NewArrayVariableProb can flip to array (collective+member on list)
	opts.Arrays = false
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	tInt := GetSimpleTypeSess(testAmbientSession, EInt)
	// Fixed qfer — no RNG for quals
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), tInt, &q, r)
	if v == nil || v.Name != "g_1" {
		t.Fatalf("name: %+v", v)
	}
	if len(vs.GlobalList) != 1 || vs.GlobalList[0] != v {
		t.Fatal("GlobalList")
	}
	if !vs.VarCreated || vs.TmpCount != 1 {
		t.Fatal("flags")
	}
	v2 := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), tInt, &q, r)
	if v2.Name != "g_2" {
		t.Fatalf("second name %s", v2.Name)
	}
}

func TestGenerateNewGlobalIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / GlobalFacts fail closed sticky before create
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if vs.GenerateNewGlobal(AccessRead, cg, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("incomplete EffectAccum must fail closed GenerateNewGlobal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	if len(vs.GlobalList) != 0 {
		t.Fatal("must not invent GlobalList registration past ambient hole")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	if vs.GenerateNewGlobal(AccessRead, cg2, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed GenerateNewGlobal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg3 := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if vs.GenerateNewGlobal(AccessRead, cg3, GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 3)) != nil {
		t.Fatal("incomplete EffectContext must fail closed GenerateNewGlobal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMaxGlobalsFailClosed(t *testing.T) {
	// Go MaxGlobals library cap — no invent unbounded GlobalList past limit
	ResetDefaultGensym()
	opts := Defaults()
	opts.Arrays = false
	opts.MaxGlobals = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 2))
	if v == nil {
		t.Fatal("first global")
	}
	if vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 3)) != nil {
		t.Fatal("at MaxGlobals must fail closed")
	}
	if vs.GenerateNewNonArrayGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 4)) != nil {
		t.Fatal("NonArray at MaxGlobals must fail closed")
	}
	if len(vs.GlobalList) != 1 {
		t.Fatal(len(vs.GlobalList))
	}
	// CreateRandomArray global path also respects MaxGlobals
	opts2 := Defaults()
	opts2.MaxGlobals = 1
	opts2.GlobalVariables = true
	vs2 := NewVariableSelector(testAmbientSession, opts2)
	vs2.GlobalList = []*Variable{v}
	vs2.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntTypeSess(testAmbientSession)}}
	// force asGlobal by only allowing global path — stack empty + GlobalVariables
	// CreateRandomArray: asGlobal = GlobalVariables && flipcoin(25); may pick local
	// empty stack + GlobalVariables false for local fail; with GlobalVariables and at max
	for seed := uint64(1); seed < 40; seed++ {
		if av := vs2.CreateRandomArray(NewRngSess(testAmbientSession, seed), EmptyCGContext().WithSession(testAmbientSession)); av != nil && av.IsGlobalSess(testAmbientSession) {
			t.Fatal("CreateRandomArray global at MaxGlobals must fail closed", av.Name)
		}
	}
}

func TestDefaultsMaxGlobalsUnlimited(t *testing.T) {
	// Defaults MaxGlobals=0: fair with VariableSelector.cpp (no GlobalList cap).
	// Cap 80 used to soft-nil GenerateNewGlobal mid seed-2 (first_div e25774:
	// UP createAndInitialize F20 NewArrayVariableProb vs Go Select U100).
	ResetDefaultGensym()
	opts := Defaults()
	if opts.MaxGlobals != 0 {
		t.Fatalf("Defaults.MaxGlobals want 0 (unlimited), got %d", opts.MaxGlobals)
	}
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	opts.Arrays = false
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Past the old unfair default of 80 — still create (upstream has no cap).
	for i := 0; i < 85; i++ {
		v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, uint64(100+i)))
		if v == nil || HasErrorSess(testAmbientSession) {
			t.Fatalf("global %d: unlimited Defaults must create (got nil err=%v list=%d)", i, GetErrorSess(testAmbientSession), len(vs.GlobalList))
		}
	}
	if len(vs.GlobalList) != 85 {
		t.Fatal(len(vs.GlobalList))
	}
}

func TestCreateArrayVariableEmptyNameFailClosed(t *testing.T) {
	// name always live from gensym; no invent empty-name array shell
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, nil, nil, "", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q) != nil {
		t.Fatal("empty name must fail closed")
	}
}

func TestCreateArrayVariableIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / GlobalFacts fail closed sticky when CG is live
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	ClearErrorSess(testAmbientSession)
	if CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, &cg, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q) != nil {
		t.Fatal("incomplete EffectAccum must fail closed CreateArrayVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	cg2.EffectAccum = &eff
	if CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, &cg2, nil, "g_b", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed CreateArrayVariable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectGlobalEmptyCreates(t *testing.T) {
	// SelectGlobal empty → GenerateNewGlobal
	ResetDefaultGensym()
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), &q, r)
	if v == nil || v.Name != "g_1" || len(vs.GlobalList) != 1 {
		t.Fatalf("create on empty: %+v list=%d", v, len(vs.GlobalList))
	}
}

func TestSelectGlobalChoosesExisting(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQferSess(testAmbientSession, "g_1", GetSimpleTypeSess(testAmbientSession, EInt), q)
	// non-convertible pointer won't match int under Flexible
	b := CreateVariableQferSess(testAmbientSession, "g_2", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt)), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRngSess(testAmbientSession, 2)
	got := vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), &q, r)
	// eFlexible: int matches a; *int is not convertible to int without deref path
	// is_derivable: ptr_type==int for *int? this is int*, ptr_type is int, is_derivable(int)
	// from *int: this==t false; is_convertable(*int) false; is_dereferenced_from(*int) true (int from *int)
	// Wait — match is want.MatchSess(testAmbientSession, var.Type): int.MatchSess(testAmbientSession, *int, Flexible) = int.is_derivable(*int)
	// is_derivable(*int): this==t no; is_convertable no; is_dereferenced_from(*int) yes (int is deref of *int)
	// So Flexible actually matches *int as source for int! That's eDereference-like via is_derivable.
	// Upstream may then emit *g_2 via ExpressionVariable. Our SelectGlobal returns the var.
	if got != a && got != b {
		t.Fatalf("should pick existing, got %v", got)
	}
	// only matching exact non-pointer when we use two ints
	c := CreateVariableQferSess(testAmbientSession, "g_3", GetSimpleTypeSess(testAmbientSession, EInt), q)
	vs.GlobalList = []*Variable{a, c}
	got = vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), &q, r)
	if got != a && got != c {
		t.Fatalf("want one of int globals, got %v", got)
	}
}

func TestSelectGlobalMultiMatchUpto(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQferSess(testAmbientSession, "g_1", GetSimpleTypeSess(testAmbientSession, EInt), q)
	b := CreateVariableQferSess(testAmbientSession, "g_2", GetSimpleTypeSess(testAmbientSession, EInt), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRngSess(testAmbientSession, 2)
	// First upto(2) = 1959434203 % 2
	rProbe := NewRngSess(testAmbientSession, 2)
	idx := int(rProbe.RndUptoSess(testAmbientSession, 2))
	got := vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), &q, r)
	want := []*Variable{a, b}[idx]
	if got != want {
		t.Fatalf("got %s want %s", got.Name, want.Name)
	}
}

func TestGenerateNewGlobalRandomQferConsumesRNG(t *testing.T) {
	// nil qfer → random_qualifiers (2 flips) + Constant::make_random (more draws).
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRngSess(testAmbientSession, 2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), nil, r)
	if v == nil || v.Init == nil || v.Init.Value == "" {
		t.Fatalf("init missing: %+v", v)
	}
	if r.RandDepthSess(testAmbientSession) < 2 {
		t.Fatalf("expected qfer+const RNG, depth=%d", r.RandDepthSess(testAmbientSession))
	}
}

func TestGenerateNewGlobalFixedQferHasInit(t *testing.T) {
	// Scalar create_and_initialize path (not array itemize): make_init_value applied.
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	r := NewRngSess(testAmbientSession, 2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EInt), &q, r)
	if v == nil || (v.Init == nil && v.InitExpr == nil) {
		t.Fatal("MakeInitValue init")
	}
	// pointer qfer must be depth 2 (indirect_level+1); make_init asserts sanity_check
	qPtr := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// pointer: make_init_value → Constant "0" (20%) or &visible Expression
	vp := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EInt)), &qPtr, r)
	if vp == nil || (vp.Init == nil && vp.InitExpr == nil) {
		t.Fatalf("pointer init: Init=%+v InitExpr=%+v", vp.Init, vp.InitExpr)
	}
	if vp.Init != nil && vp.Init.Value != "0" {
		t.Fatalf("pointer constant init want 0 got %+v", vp.Init)
	}
}

func TestCreateAndInitializeStrictConstMakeRandomFailClosed(t *testing.T) {
	// VariableSelector.cpp:526–527 — Constant::make_random then create_array;
	// no invent array with nil init when make_random fails.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.StrictConstArrays = true
	opts.Arrays = true
	probs := NewProbabilities(opts)
	probs.single[PNewArrayVariableProb] = 100
	vs := NewVariableSelectorProbs(opts, probs)
	vs.Sess = testAmbientSession
	// void → MakeRandom nil on array path → fail closed (no invent array shell)
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EVoid), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_x", NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("void must fail closed")
	}
	ClearErrorSess(testAmbientSession)
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_y", nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG createAndInitialize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}), nil, "", NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("empty name must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name createAndInitialize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateRandomArrayIncompleteAmbientSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if vs.CreateRandomArray(NewRngSess(testAmbientSession, 1), cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed CreateRandomArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if vs.CreateRandomArray(NewRngSess(testAmbientSession, 2), cg2) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed CreateRandomArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateRandomArrayMakeRandomFailClosed(t *testing.T) {
	// VariableSelector.cpp:1364 — Constant::make_random; no invent CreateArray with nil init
	opts := Defaults()
	opts.GlobalVariables = true
	probs := NewProbabilities(opts)
	// only non-simple type in AllTypes so choose picks struct; MakeRandom needs probs
	st := &Type{isStruct: true, StructName: "SOnly", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	vs := NewVariableSelectorProbs(opts, probs)
	vs.Sess = testAmbientSession
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{st}, StructTypes: []*Type{st}}
	vs.Probs = nil // MakeRandomSess(testAmbientSession, struct) fails closed
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	for seed := uint64(1); seed < 30; seed++ {
		ClearErrorSess(testAmbientSession)
		if av := vs.CreateRandomArray(NewRngSess(testAmbientSession, seed), cg); av != nil {
			t.Fatalf("seed %d: must not invent array when make_random fails", seed)
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeInitValueSelectLoopCtrlNilDepsSticky(t *testing.T) {
	// MakeInitValue / SelectLoopCtrl / SelectParentLocal always have VS+RNG sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if vs.MakeInitValue(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, nil, nil) != nil {
		t.Fatal("nil RNG MakeInitValue must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeInitValue must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.SelectLoopCtrlVar(nil, EmptyCGContext().WithSession(testAmbientSession), nil) != nil {
		t.Fatal("nil RNG SelectLoopCtrlVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG SelectLoopCtrlVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// no CurrentFunc is soft re-pick (not sticky) — EmptyCGContext select scopes
	if vs.SelectParentLocalInv(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1), MatchFlexible, nil) != nil {
		t.Fatal("nil CurrentFunc SelectParentLocalInv must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil CurrentFunc SelectParentLocalInv must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	if vs.SelectParentLocalInv(AccessRead, WithFunc(&Function{Name: "f"}, EmptyEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil, MatchFlexible, nil) != nil {
		t.Fatal("nil RNG SelectParentLocalInv must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG SelectParentLocalInv must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.EagerCreateGlobalStruct(AccessRead, EmptyCGContext().WithSession(testAmbientSession), nil, nil, NewRngSess(testAmbientSession, 1), MatchFlexible) != nil {
		t.Fatal("nil type EagerCreateGlobalStruct must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type EagerCreateGlobalStruct must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateSelectArrayNilDepsSticky(t *testing.T) {
	// VariableSelector always has VS + RNG; sticky no invent select/create array shells
	ClearErrorSess(testAmbientSession)
	if (*VariableSelector)(nil).SelectArray(NewRngSess(testAmbientSession, 1), EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("nil VS SelectArray must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil VS SelectArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(testAmbientSession, Defaults())
	if vs.SelectArray(nil, EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("nil RNG SelectArray must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG SelectArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.CreateRandomArray(nil, EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("nil RNG CreateRandomArray must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG CreateRandomArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateNewParentLocalNilDepsSticky(t *testing.T) {
	// parent-local always has VS + block + type + RNG; sticky no invent shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	blk := &Block{}
	if vs.GenerateNewParentLocal(nil, AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil block must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil block GenerateNewParentLocal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.GenerateNewParentLocal(blk, AccessRead, EmptyCGContext().WithSession(testAmbientSession), nil, nil, NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type GenerateNewParentLocal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.GenerateNewParentLocal(blk, AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG GenerateNewParentLocal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateAndInitializeMakeInitValueFailClosed(t *testing.T) {
	// VariableSelector.cpp:531–533 — make_init_value then new_variable;
	// make_init_value always Expression* or ERROR_GUARD — no invent uninit shell.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Arrays = false // force scalar path
	vs := NewVariableSelector(testAmbientSession, opts)
	// nil qfer sanity: MakeInitValue requires non-nil qf
	// createAndInitialize always builds qfer; force fail via void type
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EVoid), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_v", NewRngSess(testAmbientSession, 1)) != nil {
		t.Fatal("void must fail closed")
	}
	// nil RNG sticky
	ClearErrorSess(testAmbientSession)
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_n", nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG createAndInitialize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// bad qfer (empty levels on pointer) — MakeInitValue fail closed sticky
	ptr := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	badQ := CVQualifiers{} // empty not sanity_check for pointer
	if vs.createAndInitialize(AccessRead, EmptyCGContext().WithSession(testAmbientSession), ptr, badQ, nil, "g_p", NewRngSess(testAmbientSession, 3)) != nil {
		t.Fatal("bad qfer must fail closed without invent uninit var")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectParentLocalInvIncompleteStackFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	// Stack with nil hole
	f.Stack = []*Block{nil}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if vs.SelectParentLocalInv(AccessRead, cg, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1), MatchFlexible, nil) != nil {
		t.Fatal("nil Stack hole must fail closed SelectParentLocalInv")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stack hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete LocalVars
	blk := &Block{Func: f, LocalVars: []*Variable{nil}}
	f.Stack = []*Block{blk}
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if vs.SelectParentLocalInv(AccessRead, cg2, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 2), MatchFlexible, nil) != nil {
		t.Fatal("incomplete LocalVars must fail closed SelectParentLocalInv")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete LocalVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete invalid_vars on SelectWithInvalid
	if vs.SelectWithInvalid(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 3), MatchFlexible, []*Variable{nil}) != nil {
		t.Fatal("incomplete invalid_vars must fail closed SelectWithInvalid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete invalid_vars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete ambient on SelectWithInvalid
	inc := IncompleteEffect()
	cg3 := EmptyCGContext().WithSession(testAmbientSession)
	cg3.EffectAccum = &inc
	if vs.SelectWithInvalid(AccessRead, cg3, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 4), MatchFlexible, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed SelectWithInvalid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky SelectWithInvalid")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg4 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if vs.SelectWithInvalid(AccessRead, cg4, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 5), MatchFlexible, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed SelectWithInvalid")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky SelectWithInvalid")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete ambient on SelectParentLocalInv
	f3 := &Function{Name: "f3", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk3 := &Block{Func: f3}
	f3.Stack = []*Block{blk3}
	inc2 := IncompleteEffect()
	cg5 := WithFunc(f3, EmptyEffect()).WithSession(testAmbientSession)
	cg5.EffectAccum = &inc2
	if vs.SelectParentLocalInv(AccessRead, cg5, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 6), MatchFlexible, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed SelectParentLocalInv")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky SelectParentLocalInv")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Param on SelectParentParamInv
	f4 := &Function{Name: "f4", ReturnType: GetIntTypeSess(testAmbientSession), Param: []*Variable{nil}}
	cg6 := WithFunc(f4, EmptyEffect()).WithSession(testAmbientSession)
	if vs.SelectParentParamInv(AccessRead, cg6, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 7), MatchFlexible, nil) != nil {
		t.Fatal("incomplete Param must fail closed SelectParentParamInv")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Param must SetError sticky SelectParentParamInv")
	}
	ClearErrorSess(testAmbientSession)
	// Function always live for parent-param select; sticky
	if vs.SelectParentParamInv(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 7), MatchFlexible, nil) != nil {
		t.Fatal("nil CurrentFunc must fail closed SelectParentParamInv")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CurrentFunc SelectParentParamInv must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalList on SelectGlobalMT
	vs2 := NewVariableSelector(testAmbientSession, opts)
	vs2.GlobalList = []*Variable{nil}
	if vs2.SelectGlobalMT(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 8), MatchFlexible, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed SelectGlobalMT")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalList must SetError sticky SelectGlobalMT")
	}
	ClearErrorSess(testAmbientSession)
}
