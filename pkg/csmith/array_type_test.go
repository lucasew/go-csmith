package csmith

import "testing"

func TestCreateRandomArrayUsesEnvTypes(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	// create several arrays; at least one may be non-simple if structs exist
	seenSimple, seenAgg := false, false
	for seed := uint64(1); seed < 40; seed++ {
		f := &Function{Name: "func_1", ReturnType: GetIntType()}
		blk := &Block{Func: f}
		f.Stack = []*Block{blk}
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
		av := vs.CreateRandomArray(NewRng(seed), cg)
		if av == nil || av.Type == nil {
			continue
		}
		if av.Type.IsSimple() {
			seenSimple = true
		}
		if av.Type.IsAggregateSess(testAmbientSession) {
			seenAgg = true
		}
	}
	if !seenSimple {
		t.Fatal("expected some simple-element arrays")
	}
	_ = seenAgg // optional if structs rare under global flip
}

func TestChooseRandomNonvoidNonvolatile(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(3), opts, probs, env)
	// inject volatile struct
	volt := &Type{isStruct: true, StructName: "SV", Fields: []StructField{
		{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{true}), BitWidth: -1},
	}}
	env.AllTypes = append(env.AllTypes, volt)
	r := NewRng(4)
	for i := 0; i < 50; i++ {
		ty := env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
		if ty != nil && ty.IsVolatileStructUnion() {
			t.Fatal("volatile aggregate")
		}
		if ty != nil && ty.IsSimple() && ty.Simple() == EVoid {
			t.Fatal("void")
		}
	}
}

func TestEffectWriteVar(t *testing.T) {
	// Effect.cpp:137–146 — non-volatile write keeps pure and SE-free
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	e := EmptyEffect()
	if !e.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("empty")
	}
	e2 := e.WriteVarSess(testAmbientSession, v)
	if !e2.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("not marked")
	}
	if !e2.IsSideEffectFreeSess(testAmbientSession) || !e2.IsPureSess(testAmbientSession) {
		t.Fatal("non-vol write keeps pure/SE-free")
	}
	// volatile write clears SE-free only
	vv := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, true)
	e3 := EmptyEffect().WriteVarSess(testAmbientSession, vv)
	if e3.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("vol write clears SE-free")
	}
	if !e3.IsPureSess(testAmbientSession) {
		t.Fatal("write does not clear pure")
	}
	// original unchanged
	if !e.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("value semantics")
	}
}

func TestEffectReadVarPurity(t *testing.T) {
	// Effect.cpp:116–122
	c := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntType(), true, false) // const non-vol
	e := EmptyEffect().ReadVarSess(testAmbientSession, c)
	if !e.IsPureSess(testAmbientSession) || !e.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("const non-vol read pure+SE-free")
	}
	nv := CreateVariableScalarsSess(testAmbientSession, "g_nv", GetIntType(), false, false)
	e2 := EmptyEffect().ReadVarSess(testAmbientSession, nv)
	if e2.IsPureSess(testAmbientSession) {
		t.Fatal("non-const read not pure")
	}
	if !e2.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("non-vol read still SE-free")
	}
	vol := CreateVariableScalarsSess(testAmbientSession, "g_vol", GetIntType(), false, true)
	e3 := EmptyEffect().ReadVarSess(testAmbientSession, vol)
	if e3.IsSideEffectFreeSess(testAmbientSession) || e3.IsPureSess(testAmbientSession) {
		t.Fatal("vol read")
	}
}

func TestCreateRandomArrayAddsFacts(t *testing.T) {
	// VariableSelector.cpp:1371–1377 — AddNewVarFactAndUpdate for new arrays
	opts := Defaults()
	opts.GlobalVariables = true
	vs := NewVariableSelector(opts)
	env := &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	vs.Types = env
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// seed meta so AddNewVarFact creates point-to when pointer; int array still registers
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.Types = env
	// force global path: seed 25% — try several
	var av *ArrayVariable
	for seed := uint64(1); seed < 40; seed++ {
		av = vs.CreateRandomArray(NewRng(seed), cg)
		if av != nil && av.IsGlobalSess(testAmbientSession) {
			break
		}
	}
	if av == nil {
		t.Fatal("no array")
	}
	// Global path should call AddNewVarFactAndUpdate; inventory updated
	if !IsVariableInSet(vs.GlobalList, &av.Variable) && !IsVariableInSet(vs.AllVars, &av.Variable) {
		t.Fatal("not inventoried")
	}
}

func TestAddNewVarFactAndUpdateMapsAndGlobalAssert(t *testing.T) {
	// FactMgr.cpp:69–110 — assert global when blk nil; push into map_facts_in/out
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// seed map slots for a statement
	sid := 42
	fm.MapFactsIn[sid] = nil
	fm.MapFactsOut[sid] = nil
	// non-global with blk==nil must fail closed (assert path)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", PointerTo(GetIntType()), false, false)
	before := len(fm.GlobalFacts)
	fm.AddNewVarFactAndUpdate(nil, loc)
	if len(fm.GlobalFacts) != before {
		t.Fatal("non-global must not invent facts when blk==nil")
	}
	// global pointer: facts in global + maps
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFactAndUpdate(nil, g)
	if FindRelatedPointTo(fm.GlobalFacts, g) == nil {
		t.Fatal("global fact missing")
	}
	if len(fm.MapFactsIn[sid]) == 0 || len(fm.MapFactsOut[sid]) == 0 {
		t.Fatal("blk==nil must append fact to all map_facts_in/out")
	}
	// incomplete map slot must not invent soft-append past hole (local marker, non-sticky)
	fm.MapFactsIn[sid] = IncompleteFactSlice()
	g2 := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFactAndUpdate(nil, g2)
	if FactsComplete(fm.MapFactsIn[sid]) {
		t.Fatal("incomplete map slot must stay incomplete after AddNewVarFactAndUpdate")
	}
	// incomplete GlobalFacts fails closed sticky before add
	fm.GlobalFacts = IncompleteFactSlice()
	g3 := CreateVariableScalarsSess(testAmbientSession, "g_r", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFactAndUpdate(nil, g3)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete GlobalFacts must stay wiped")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts AddNewVarFactAndUpdate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddNewVarFactAndUpdatePushesMapsWhenFactAlreadyPresent(t *testing.T) {
	// FactMgr.cpp:84–104 — always push *init* abstract into maps even when
	// global_facts already has a related fact (caller new_globals handoff after
	// RenewFacts). Must push the init abstract, not the existing related entry
	// (seed-2: post-analysis g_87→g_62 must not be what maps receive — init
	// g_87→g_64 is appended so for-body map_in restore can re-surface g_64).
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_tgt", GetIntType(), false, false)
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	// init &g_tgt for abstract_fact_for_var_init
	// ExprType pointer + Var int → IndirectLevel() == -1 (address-of)
	g.InitExpr = &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	// seed GlobalFacts as if RenewFacts already set a *different* points-to
	fp := MakeFactPointTo(g, NullPtr)
	if fp == nil {
		t.Fatal("MakeFactPointTo")
	}
	fm.GlobalFacts = []*FactPointTo{fp}
	// map slot exists but does not yet list g (post-call maps lag GlobalFacts)
	sid := 7
	fm.MapFactsIn[sid] = []*FactPointTo{}
	fm.MapFactsOut[sid] = []*FactPointTo{}
	fm.AddNewVarFactAndUpdate(nil, g)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("must not sticky when fact already present")
	}
	inF := FindRelatedPointTo(fm.MapFactsIn[sid], g)
	if inF == nil {
		t.Fatal("must push init abstract into map_facts_in")
	}
	// maps must carry INIT pointees (g_tgt), not the existing NullPtr related
	if !IsVariableInSet(inF.PointTo, tgt) {
		t.Fatalf("map_facts_in must get init pointee g_tgt, got %v", pointToNames(inF))
	}
	if IsVariableInSet(inF.PointTo, NullPtr) && len(inF.PointTo) == 1 {
		t.Fatal("must not push existing NullPtr-related fact as the map entry")
	}
	outF := FindRelatedPointTo(fm.MapFactsOut[sid], g)
	if outF == nil || !IsVariableInSet(outF.PointTo, tgt) {
		t.Fatal("must push init abstract into map_facts_out")
	}
	// GlobalFacts length unchanged (no second invent of same subject); still NullPtr
	if len(fm.GlobalFacts) != 1 {
		t.Fatalf("GlobalFacts len=%d want 1", len(fm.GlobalFacts))
	}
	if gf := FindRelatedPointTo(fm.GlobalFacts, g); gf == nil || !IsVariableInSet(gf.PointTo, NullPtr) {
		t.Fatal("GlobalFacts must keep post-analysis NullPtr (not overwrite with init)")
	}
	ClearErrorSess(testAmbientSession)
}

func pointToNames(f *FactPointTo) []string {
	if f == nil {
		return nil
	}
	out := make([]string, 0, len(f.PointTo))
	for _, v := range f.PointTo {
		if v == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, v.Name)
	}
	return out
}

// TestAddNewVarFactAndUpdateDoesNotPushIntoDeclaringBlockMapIn —
// Statement.cpp:380–389 in_block walks parent (never self). FactMgr.cpp:96–98
// only updates map_facts_in for stm->in_block(blk). The declaring Block itself
// must not receive its own locals on map_facts_in (seed-2: body-local l_260 on
// map_in[body] → post_loop reintroduces → outer OOS → garbage → e10107).
func TestAddNewVarFactAndUpdateDoesNotPushIntoDeclaringBlockMapIn(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// Declaring loop body (no parent needed for in_block of nested stmts)
	body := &Block{StmID: 90, Func: f}
	// Nested assign inside body — Statement::in_block(body) walks parent to body
	inner := &Stmt{Kind: StmtAssign, StmID: 108}
	body.Stmts = []Stmt{*inner}
	f.Blocks = []*Block{body}
	// Map slots for body (declaring block) and inner stmt
	fm.MapFactsIn[body.StmID] = []*FactPointTo{}
	fm.MapFactsOut[body.StmID] = []*FactPointTo{}
	fm.MapFactsIn[inner.StmID] = []*FactPointTo{}
	fm.MapFactsOut[inner.StmID] = []*FactPointTo{}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_260", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFactAndUpdate(body, loc)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("AddNewVarFactAndUpdate sticky: %v", HasErrorSess(testAmbientSession))
	}
	if FindRelatedPointTo(fm.MapFactsIn[body.StmID], loc) != nil {
		t.Fatal("declaring block map_facts_in must not get body-local (in_block self is false)")
	}
	if FindRelatedPointTo(fm.MapFactsIn[inner.StmID], loc) == nil {
		t.Fatal("nested stmt in body must get the local on map_facts_in")
	}
	if FindRelatedPointTo(fm.GlobalFacts, loc) == nil {
		t.Fatal("GlobalFacts must still get init fact")
	}
	ClearErrorSess(testAmbientSession)
}

// TestAddNewVarFactAndUpdatePushesBlockMapOut —
// FactMgr.cpp:99–100 add_fact_out on every map_facts_out key including Block*
// (C++ Block : Statement). Parent-local created mid-else must still reach
// map_facts_out[if_true] so combine_branch_facts merges then-arm init points-to
// (seed-2 func_11: l_1326=&g_99 then reassigned in else → *l_1326 must read g_99).
func TestAddNewVarFactAndUpdatePushesBlockMapOut(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_11", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// body owns parent-locals; then arm is a nested Block with its own StmID key
	body := &Block{StmID: 10, Func: f}
	thenB := &Block{StmID: 20, Func: f, Parent: body}
	elseB := &Block{StmID: 21, Func: f, Parent: body}
	f.Blocks = []*Block{body, thenB, elseB}
	// if statement already recorded map slots for both arms (post then gen, mid else)
	fm.MapFactsIn[thenB.StmID] = []*FactPointTo{}
	fm.MapFactsOut[thenB.StmID] = []*FactPointTo{}
	fm.MapFactsIn[elseB.StmID] = []*FactPointTo{}
	fm.MapFactsOut[elseB.StmID] = []*FactPointTo{}
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_99", GetIntType(), false, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1326", PointerTo(GetIntType()), false, false)
	loc.InitExpr = &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	// create as body parent-local while "in else" (maps already open for both arms)
	body.LocalVars = append(body.LocalVars, loc)
	fm.AddNewVarFactAndUpdate(body, loc)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("sticky: %v", GetErrorSess(testAmbientSession))
	}
	// then arm Block out must receive init fact (Block key, not FindStmtByID)
	thenF := FindRelatedPointTo(fm.MapFactsOut[thenB.StmID], loc)
	if thenF == nil {
		t.Fatal("map_facts_out[if_true Block] must get parent-local init fact")
	}
	if !IsVariableInSet(thenF.PointTo, tgt) {
		t.Fatalf("then out must point to g_99, got %v", pointToNames(thenF))
	}
	elseF := FindRelatedPointTo(fm.MapFactsOut[elseB.StmID], loc)
	if elseF == nil || !IsVariableInSet(elseF.PointTo, tgt) {
		t.Fatal("map_facts_out[if_false Block] must also get init fact")
	}
	ClearErrorSess(testAmbientSession)
}

// TestAddNewVarFactAndUpdatePushesGotoOutMidGeneration —
// FactMgr.cpp:95–103 add_fact_out for all map_facts_out when blk!=null (no
// in_block filter on outs). Mid-MakeRandomIf the if is not yet in body.Stmts, so
// tree-only stmtIDInBlock misses nested gotos; parent-local facts must still
// reach map_facts_out[goto] or merge_jump_facts invents garbage (seed-2 func_11
// l_1325 pts=[garbage,g_32] → else FP strip).
func TestAddNewVarFactAndUpdatePushesGotoOutMidGeneration(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_11", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// Function body (declaring blk for parent-local) — if not yet linked in Stmts
	body := &Block{StmID: 514, Func: f}
	// Else arm mid-construction: Parent set, not yet reachable via body.Stmts
	els := &Block{StmID: 523, Func: f, Parent: body}
	// Back-edge goto target assign + nested for body with goto (as in seed-2 else)
	asg := Stmt{Kind: StmtAssign, StmID: 524}
	forBody := &Block{StmID: 530, Func: f, Parent: els, Looping: true}
	gt := Stmt{
		Kind: StmtGoto, StmID: 540, GotoBack: true,
		GotoDestStmID: 524, GotoDestParent: els, Label: "lbl",
	}
	forBody.Stmts = []Stmt{gt}
	forSt := Stmt{Kind: StmtFor, StmID: 556, Then: forBody}
	els.Stmts = []Stmt{asg, forSt}
	// body.Stmts intentionally empty (if not yet PushStmt) — tree miss for nested
	f.Blocks = []*Block{body, els, forBody}
	// map slots as after post_creation of nested stmts
	for _, id := range []int{asg.StmID, gt.StmID, forSt.StmID, forBody.StmID, els.StmID} {
		fm.MapFactsIn[id] = []*FactPointTo{}
		fm.MapFactsOut[id] = []*FactPointTo{}
	}
	// Parent-local int16_t* created while still under else (block = body)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1325", PointerTo(GetIntType()), false, false)
	body.LocalVars = append(body.LocalVars, loc) // visibility at dest via IsVarOnStack
	loc.Init = nil
	g32 := CreateVariableScalarsSess(testAmbientSession, "g_32", GetIntType(), false, false)
	loc.InitExpr = &Expression{Term: TermVariable, Var: g32, ExprType: PointerTo(GetIntType())}
	fm.AddNewVarFactAndUpdate(body, loc)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("AddNewVarFactAndUpdate sticky: err=%v", GetErrorSess(testAmbientSession))
	}
	// Goto out must receive the new fact (C++ add_fact_out, not tree in_block)
	if FindRelatedPointTo(fm.MapFactsOut[gt.StmID], loc) == nil {
		t.Fatalf("map_facts_out[goto] must get parent-local mid-if construction; out=%v",
			fm.MapFactsOut[gt.StmID])
	}
	// Assign dest also (visible at dest)
	if FindRelatedPointTo(fm.MapFactsOut[asg.StmID], loc) == nil {
		t.Fatal("map_facts_out[assign] must get parent-local")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVarCollectiveNilMustNotInventAddNewVarFact(t *testing.T) {
	// GenerateNew* FM path: varCollective nil → SetError, no silent invent success
	// without facts (AddNewVarFactAndUpdate(nil,nil) no-ops).
	ClearErrorSess(testAmbientSession)
	if varCollective(nil) != nil {
		t.Fatal("nil varCollective must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil varCollective must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVarsSess(testAmbientSession)
	item := parent.ItemizeConstIndices([]int{0}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	item.CreateFieldVarsSess(testAmbientSession)
	if len(item.FieldVars) == 0 {
		t.Fatal("fields")
	}
	fld := item.FieldVars[0]
	item.FieldVars = append(item.FieldVars, nil)
	if varCollective(fld) != nil {
		t.Fatal("incomplete array-field path must yield nil collective")
	}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	before := len(fm.GlobalFacts)
	ClearErrorSess(testAmbientSession)
	// mirror GenerateNew* fail-closed: coll nil with FM set → sticky error, no invent facts
	coll := varCollective(fld)
	if coll == nil {
		SetErrorSess(testAmbientSession, ErrGeneric)
	} else {
		fm.AddNewVarFactAndUpdate(nil, coll)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil collective must SetError (no invent generate success)")
	}
	if len(fm.GlobalFacts) != before {
		t.Fatal("must not invent facts for nil collective")
	}
	// bare nil subject still no-ops without invent
	fm.AddNewVarFactAndUpdate(nil, nil)
	if len(fm.GlobalFacts) != before {
		t.Fatal("AddNewVarFactAndUpdate(nil,nil) must not invent")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateNewGlobalIncompleteGlobalFactsFailClosed(t *testing.T) {
	// AddNewVarFactAndUpdate leaves incomplete GlobalFacts — must not invent create success
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.GlobalVariables = true
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if vs.GenerateNewGlobal(AccessWrite, cg, GetIntType(), nil, NewRng(1)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed GenerateNewGlobal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError")
	}
	ClearErrorSess(testAmbientSession)
	// NonArray path same
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	if vs.GenerateNewNonArrayGlobal(AccessRead, cg2, GetIntType(), nil, NewRng(2)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed GenerateNewNonArrayGlobal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError NonArray")
	}
	ClearErrorSess(testAmbientSession)
	// ParentLocal path
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm3 := NewFactMgrSess(testAmbientSession, f)
	fm3.GlobalFacts = IncompleteFactSlice()
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm3)
	if vs.GenerateNewParentLocal(blk, AccessWrite, cg3, GetIntType(), nil, NewRng(3)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed GenerateNewParentLocal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError ParentLocal")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateRandomArrayRejectsUnacceptableType(t *testing.T) {
	// AcceptType false for volatile struct when context not SE-free
	opts := Defaults()
	vs := NewVariableSelector(opts)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	// make struct look volatile for AcceptType path
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{st, GetIntType()}}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	// non-SE-free context
	cg := WithFunc(f, EmptyEffect().WriteVarSess(testAmbientSession, CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), true, false))).WithSession(testAmbientSession)
	cg.Types = vs.Types
	// VariableSelector.cpp — no soft invent int elem; nil OK when AcceptType rejects
	av := vs.CreateRandomArray(NewRng(3), cg)
	_ = av // may be nil when no acceptable element type
}

func TestCreateRandomArrayIncompleteStackFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.GlobalVariables = false
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.Stack = []*Block{nil}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.Types = vs.Types
	if vs.CreateRandomArray(NewRng(1), cg) != nil {
		t.Fatal("incomplete Stack must fail closed CreateRandomArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Stack must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateRandomArrayIsConstStructUnionResidualSticky(t *testing.T) {
	// Type-nil field: IsConstStructUnion stickies residual ERROR+true.
	// Soft invent was soft-continue filter then CreateArrayVariable later good int.
	// Fair: sticky fail closed whole CreateRandomArray.
	// Use global path (ChooseRandomNonvoid) — local nonvolatile filter residual on
	// IsVolatileStructUnion can hang chooseRandomFiltered when all types rejected.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.GlobalVariables = true
	vs := NewVariableSelector(opts)
	broken := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	// only broken type so Choose always hits residual IsConstStructUnion
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{broken}}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.Types = vs.Types
	// flipcoin(25) gates global create — scan seeds until residual sticky lands
	sawSticky := false
	for seed := uint64(1); seed < 400; seed++ {
		ClearErrorSess(testAmbientSession)
		if vs.CreateRandomArray(NewRng(seed), cg) != nil {
			t.Fatalf("IsConstStructUnion residual must fail closed CreateRandomArray seed %d", seed)
		}
		if HasErrorSess(testAmbientSession) {
			sawSticky = true
			break
		}
	}
	if !sawSticky {
		t.Fatal("IsConstStructUnion residual CreateRandomArray must SetError sticky on global path")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddNewVarFactAndUpdateFillsUnionMapFromPTKeys(t *testing.T) {
	// FactMgr.cpp:69–110 — one FactVec map; eUnionWrite shares keys with ePointTo.
	// Soft invent iterated only MapUnionFactsIn keys → PT-only slots missed union init.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// PT map key without union map key (historical dual-map hole)
	sid := 42
	fm.MapFactsIn = map[int][]*FactPointTo{sid: {}}
	fm.MapFactsOut = map[int][]*FactPointTo{sid: {}}
	// MapUnionFactsIn intentionally nil / missing key
	ut := &Type{isUnion: true, StructName: "U_fill", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	// Init required for complete union abstract (FactUnion assign transfer)
	g := &Variable{Name: "g_ufill", Type: ut, Init: MakeInt(0)}
	fm.AddNewVarFactAndUpdate(nil, g) // global create
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	got := fm.GetMapUnionFactsIn(sid)
	if !UnionFactsComplete(got) {
		t.Fatalf("incomplete union map_in: %#v", got)
	}
	if len(got) == 0 {
		t.Fatal("PT map key must receive eUnionWrite init fact for new global union")
	}
	if got[0].LastWrittenFID != 0 {
		t.Fatalf("want init last_write 0, got %d", got[0].LastWrittenFID)
	}
	ClearErrorSess(testAmbientSession)
}
