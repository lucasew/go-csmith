package csmith

import "testing"

func TestChooseOKVarItemizeFailClosed(t *testing.T) {
	// VariableSelector.cpp:332–337 — collective array must itemize(); no return bare
	ClearError()
	av := CreateArrayVariable(NewRng(2), Defaults(), NewProbabilities(Defaults()), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("create")
	}
	// r nil with multi-cand would fail earlier; single collective with nil r → fail closed
	if ChooseOKVar(nil, []*Variable{&av.Variable}) != nil {
		t.Fatal("itemize needs RNG; no soft return collective")
	}
	got := ChooseOKVar(NewRng(3), []*Variable{&av.Variable})
	if got == nil || got.AsArray == nil || got.AsArray.Collective != av {
		t.Fatalf("want itemized member, got %+v", got)
	}
}

func TestChooseOKVarSoleAndUpto(t *testing.T) {
	// VariableSelector::choose_ok_var
	ClearError()
	a := CreateVariableScalars("g_1", GetSimpleType(EInt), false, false)
	b := CreateVariableScalars("g_2", GetSimpleType(EInt), false, false)
	c := CreateVariableScalars("g_3", GetSimpleType(EInt), false, false)

	if ChooseOKVar(NewRng(2), nil) != nil {
		t.Fatal("empty")
	}
	// nil hole fails closed sticky — no invent skip as absent candidate
	if ChooseOKVar(NewRng(2), []*Variable{a, nil, b}) != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearError()
	if ChooseOKVar(NewRng(2), []*Variable{a}) != a {
		t.Fatal("sole")
	}
	// seed2 first RndUpto(3) = 1959434203 % 3
	r := NewRng(2)
	wantIdx := int(r.RndUpto(3))
	r2 := NewRng(2)
	got := ChooseOKVar(r2, []*Variable{a, b, c})
	want := []*Variable{a, b, c}[wantIdx]
	if got != want {
		t.Fatalf("choose_ok_var: got %v want %v (idx %d)", got.Name, want.Name, wantIdx)
	}
}

func TestGenerateNewGlobalNamesAndList(t *testing.T) {
	// GenerateNewGlobal: gensym g_1, push GlobalList, random_qualifiers draws.
	ResetDefaultGensym()
	opts := Defaults()
	// force scalar path — NewArrayVariableProb can flip to array (collective+member on list)
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	tInt := GetSimpleType(EInt)
	// Fixed qfer — no RNG for quals
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), tInt, &q, r)
	if v == nil || v.Name != "g_1" {
		t.Fatalf("name: %+v", v)
	}
	if len(vs.GlobalList) != 1 || vs.GlobalList[0] != v {
		t.Fatal("GlobalList")
	}
	if !vs.VarCreated || vs.TmpCount != 1 {
		t.Fatal("flags")
	}
	v2 := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), tInt, &q, r)
	if v2.Name != "g_2" {
		t.Fatalf("second name %s", v2.Name)
	}
}

func TestMaxGlobalsFailClosed(t *testing.T) {
	// Go MaxGlobals library cap — no invent unbounded GlobalList past limit
	ResetDefaultGensym()
	opts := Defaults()
	opts.Arrays = false
	opts.MaxGlobals = 1
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(2))
	if v == nil {
		t.Fatal("first global")
	}
	if vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(3)) != nil {
		t.Fatal("at MaxGlobals must fail closed")
	}
	if vs.GenerateNewNonArrayGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(4)) != nil {
		t.Fatal("NonArray at MaxGlobals must fail closed")
	}
	if len(vs.GlobalList) != 1 {
		t.Fatal(len(vs.GlobalList))
	}
	// CreateRandomArray global path also respects MaxGlobals
	opts2 := Defaults()
	opts2.MaxGlobals = 1
	opts2.GlobalVariables = true
	vs2 := NewVariableSelector(opts2)
	vs2.GlobalList = []*Variable{v}
	vs2.Types = &TypeEnv{AllTypes: []*Type{GetIntType()}}
	// force asGlobal by only allowing global path — stack empty + GlobalVariables
	// CreateRandomArray: asGlobal = GlobalVariables && flipcoin(25); may pick local
	// empty stack + GlobalVariables false for local fail; with GlobalVariables and at max
	for seed := uint64(1); seed < 40; seed++ {
		if av := vs2.CreateRandomArray(NewRng(seed), EmptyCGContext()); av != nil && av.IsGlobal() {
			t.Fatal("CreateRandomArray global at MaxGlobals must fail closed", av.Name)
		}
	}
}

func TestCreateArrayVariableEmptyNameFailClosed(t *testing.T) {
	// name always live from gensym; no invent empty-name array shell
	opts := Defaults()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "", GetIntType(), MakeInt(0), q) != nil {
		t.Fatal("empty name must fail closed")
	}
}

func TestSelectGlobalEmptyCreates(t *testing.T) {
	// SelectGlobal empty → GenerateNewGlobal
	ResetDefaultGensym()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if v == nil || v.Name != "g_1" || len(vs.GlobalList) != 1 {
		t.Fatalf("create on empty: %+v list=%d", v, len(vs.GlobalList))
	}
}

func TestSelectGlobalChoosesExisting(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQfer("g_1", GetSimpleType(EInt), q)
	// non-convertible pointer won't match int under Flexible
	b := CreateVariableQfer("g_2", PointerTo(GetSimpleType(EInt)), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRng(2)
	got := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	// eFlexible: int matches a; *int is not convertible to int without deref path
	// is_derivable: ptr_type==int for *int? this is int*, ptr_type is int, is_derivable(int)
	// from *int: this==t false; is_convertable(*int) false; is_dereferenced_from(*int) true (int from *int)
	// Wait — match is want.Match(var.Type): int.Match(*int, Flexible) = int.is_derivable(*int)
	// is_derivable(*int): this==t no; is_convertable no; is_dereferenced_from(*int) yes (int is deref of *int)
	// So Flexible actually matches *int as source for int! That's eDereference-like via is_derivable.
	// Upstream may then emit *g_2 via ExpressionVariable. Our SelectGlobal returns the var.
	if got != a && got != b {
		t.Fatalf("should pick existing, got %v", got)
	}
	// only matching exact non-pointer when we use two ints
	c := CreateVariableQfer("g_3", GetSimpleType(EInt), q)
	vs.GlobalList = []*Variable{a, c}
	got = vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if got != a && got != c {
		t.Fatalf("want one of int globals, got %v", got)
	}
}

func TestSelectGlobalMultiMatchUpto(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQfer("g_1", GetSimpleType(EInt), q)
	b := CreateVariableQfer("g_2", GetSimpleType(EInt), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRng(2)
	// First upto(2) = 1959434203 % 2
	rProbe := NewRng(2)
	idx := int(rProbe.RndUpto(2))
	got := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	want := []*Variable{a, b}[idx]
	if got != want {
		t.Fatalf("got %s want %s", got.Name, want.Name)
	}
}

func TestGenerateNewGlobalRandomQferConsumesRNG(t *testing.T) {
	// nil qfer → random_qualifiers (2 flips) + Constant::make_random (more draws).
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), nil, r)
	if v == nil || v.Init == nil || v.Init.Value == "" {
		t.Fatalf("init missing: %+v", v)
	}
	if r.RandDepth() < 2 {
		t.Fatalf("expected qfer+const RNG, depth=%d", r.RandDepth())
	}
}

func TestGenerateNewGlobalFixedQferHasInit(t *testing.T) {
	// Scalar create_and_initialize path (not array itemize): make_init_value applied.
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	r := NewRng(2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if v == nil || (v.Init == nil && v.InitExpr == nil) {
		t.Fatal("MakeInitValue init")
	}
	// pointer qfer must be depth 2 (indirect_level+1); make_init asserts sanity_check
	qPtr := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// pointer: make_init_value → Constant "0" (20%) or &visible Expression
	vp := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), PointerTo(GetSimpleType(EInt)), &qPtr, r)
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
	opts := Defaults()
	opts.StrictConstArrays = true
	opts.Arrays = true
	probs := NewProbabilities(opts)
	probs.single[PNewArrayVariableProb] = 100
	vs := NewVariableSelectorProbs(opts, probs)
	// void → MakeRandom nil on array path → fail closed (no invent array shell)
	if vs.createAndInitialize(AccessRead, EmptyCGContext(), GetSimpleType(EVoid), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_x", NewRng(1)) != nil {
		t.Fatal("void must fail closed")
	}
	if vs.createAndInitialize(AccessRead, EmptyCGContext(), GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_y", nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
}

func TestCreateRandomArrayMakeRandomFailClosed(t *testing.T) {
	// VariableSelector.cpp:1364 — Constant::make_random; no invent CreateArray with nil init
	opts := Defaults()
	opts.GlobalVariables = true
	probs := NewProbabilities(opts)
	// only non-simple type in AllTypes so choose picks struct; MakeRandom needs probs
	st := &Type{isStruct: true, StructName: "SOnly", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	vs := NewVariableSelectorProbs(opts, probs)
	vs.Types = &TypeEnv{AllTypes: []*Type{st}, StructTypes: []*Type{st}}
	vs.Probs = nil // MakeRandom(struct) fails closed
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	for seed := uint64(1); seed < 30; seed++ {
		ClearError()
		if av := vs.CreateRandomArray(NewRng(seed), cg); av != nil {
			t.Fatalf("seed %d: must not invent array when make_random fails", seed)
		}
	}
}

func TestCreateAndInitializeMakeInitValueFailClosed(t *testing.T) {
	// VariableSelector.cpp:531–533 — make_init_value then new_variable;
	// make_init_value always Expression* or ERROR_GUARD — no invent uninit shell.
	opts := Defaults()
	opts.Arrays = false // force scalar path
	vs := NewVariableSelector(opts)
	// nil qfer sanity: MakeInitValue requires non-nil qf
	// createAndInitialize always builds qfer; force fail via void type
	if vs.createAndInitialize(AccessRead, EmptyCGContext(), GetSimpleType(EVoid), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_v", NewRng(1)) != nil {
		t.Fatal("void must fail closed")
	}
	// nil RNG
	if vs.createAndInitialize(AccessRead, EmptyCGContext(), GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}), nil, "g_n", nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	// bad qfer (empty levels on pointer) — MakeInitValue fail closed
	ptr := PointerTo(GetIntType())
	badQ := CVQualifiers{} // empty not sanity_check for pointer
	if vs.createAndInitialize(AccessRead, EmptyCGContext(), ptr, badQ, nil, "g_p", NewRng(3)) != nil {
		t.Fatal("bad qfer must fail closed without invent uninit var")
	}
}
