package csmith

import "testing"

func TestEagerCreateGlobalStruct(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 2), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	if len(env.StructTypes) == 0 {
		t.Skip("no structs")
	}
	// want int — eager create struct then field match
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.EagerCreateGlobalStruct(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 5), MatchFlexible)
	// may fail if no int fields
	if v != nil && v.Type != nil {
		if !GetIntType().MatchSess(testAmbientSession, v.Type, MatchFlexible) {
			t.Log("matched", v.Type.CName())
		}
	}
}

func TestSelectGlobalExpandStructPath(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 3), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// empty GlobalList → expand or create
	v := vs.SelectGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 7))
	if v == nil {
		t.Fatal("nil")
	}
}

func TestMergeEffectsMergesReads(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	e1 := EmptyEffect().ReadVarSess(testAmbientSession, a)
	e2 := EmptyEffect().ReadVarSess(testAmbientSession, b)
	m := MergeEffectsSess(testAmbientSession, e1, e2)
	if !m.IsReadSess(testAmbientSession, a) || !m.IsReadSess(testAmbientSession, b) {
		t.Fatal("reads")
	}
}

func TestFactMgrForFunc(t *testing.T) {
	f := &Function{Name: "func_1"}
	m := NewFactMgrMapSess(testAmbientSession)
	fm := m.ForFunc(f)
	if fm == nil || fm.Func != f {
		t.Fatal(fm)
	}
	if m.ForFunc(f) != fm {
		t.Fatal("reuse")
	}
	// FactMgrMap + Function always live; sticky nil
	ClearErrorSess(testAmbientSession)
	if (*FactMgrMap)(nil).ForFunc(f) != nil {
		t.Fatal("nil map ForFunc must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil map ForFunc must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if m.ForFunc(nil) != nil {
		t.Fatal("nil Function ForFunc must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function ForFunc must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpandStructUnionVars(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 2), opts, probs, &env, "S0")
	sv := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Fatal("no fields")
	}
	got := ExpandStructUnionVars([]*Variable{sv}, GetIntType())
	// parent removed when want is not the struct type itself
	for _, v := range got {
		if v == sv {
			t.Fatal("parent should expand away")
		}
	}
	if len(got) == 0 {
		t.Fatal("empty after expand")
	}
	// want exact struct type → keep parent
	keep := ExpandStructUnionVars([]*Variable{sv}, st)
	if len(keep) != 1 || keep[0] != sv {
		t.Fatalf("keep aggregate: %+v", keep)
	}
	// nil candidate / field hole fails closed sticky incomplete (not invent empty complete)
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(ExpandStructUnionVars([]*Variable{nil}, GetIntType())) {
		t.Fatal("nil var hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	sv.FieldVars = append(sv.FieldVars, nil)
	if VariablesComplete(ExpandStructUnionVars([]*Variable{sv}, GetIntType())) {
		t.Fatal("nil FieldVars hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FieldVars hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil non-special sticky incomplete (no invent keep shell as complete candidate)
	hole := &Variable{Name: "g_hole", Type: nil}
	if VariablesComplete(ExpandStructUnionVars([]*Variable{hole}, GetIntType())) {
		t.Fatal("Type-nil expand must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil expand must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was IsVirtual residual false soft-continue
	// fair: sticky IncompleteVariables
	arrShell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if VariablesComplete(ExpandStructUnionVars([]*Variable{arrShell}, GetIntType())) {
		t.Fatal("IsArray without AsArray expand must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ExpandStructUnionVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FieldVarOf ancestry IsArray-without-AsArray: IsVirtual residual ERROR+false.
	// Soft invent was soft-continue keep child then expand later good as complete pool.
	// Fair: sticky IncompleteVariables whole expand.
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	child := &Variable{Name: "g_arr.f0", Type: GetIntType(), FieldVarOf: shell}
	good := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	if VariablesComplete(ExpandStructUnionVars([]*Variable{child, good}, GetIntType())) {
		t.Fatal("IsVirtual ancestry residual must fail closed incomplete, not invent later good")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVirtual ancestry residual ExpandStructUnionVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEagerCreateLocalStruct(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 4), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	if len(env.StructTypes) == 0 {
		t.Skip("no structs")
	}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{}
	f.Stack = []*Block{blk}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.EagerCreateLocalStruct(blk, AccessRead, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 9), MatchFlexible)
	if len(blk.LocalVars) == 0 {
		t.Fatal("no local created")
	}
	if v != nil && v.Type != nil && !GetIntType().MatchSess(testAmbientSession, v.Type, MatchFlexible) {
		t.Log("field", v.Name, v.Type.CName())
	}
}

func TestEagerCreateStructIncompleteAmbientSticky(t *testing.T) {
	// Incomplete ambient / invalidVars must not invent soft re-pick create success
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 2), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if vs.EagerCreateGlobalStruct(AccessRead, WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 5), MatchFlexible) != nil {
		t.Fatal("incomplete EffectContext must fail closed EagerCreateGlobalStruct")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.EagerCreateGlobalStruct(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 5), MatchFlexible, IncompleteVariables()) != nil {
		t.Fatal("incomplete invalidVars must fail closed EagerCreateGlobalStruct")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete invalidVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{}
	f.Stack = []*Block{blk}
	if vs.EagerCreateLocalStruct(blk, AccessRead, WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 9), MatchFlexible) != nil {
		t.Fatal("incomplete EffectContext must fail closed EagerCreateLocalStruct")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext local must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if vs.EagerCreateLocalStruct(blk, AccessRead, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntType(), &q, NewRngSess(testAmbientSession, 9), MatchFlexible, IncompleteVariables()) != nil {
		t.Fatal("incomplete invalidVars must fail closed EagerCreateLocalStruct")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete invalidVars local must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectParentLocalExpandStruct(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 6), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{}
	f.Stack = []*Block{blk}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	v := vs.SelectParentLocal(AccessRead, cg, GetIntType(), &q, NewRngSess(testAmbientSession, 11), MatchFlexible)
	if v == nil {
		t.Fatal("nil")
	}
}

func TestSelectParentLocalErrorGuardAndEmptyStack(t *testing.T) {
	// VariableSelector.cpp:991–1003 — empty stack assert; ERROR_GUARD after rnd_upto
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// empty stack → fail closed (no soft invent param/global)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if vs.SelectParentLocal(AccessRead, cg, GetIntType(), &q, NewRngSess(testAmbientSession, 1), MatchFlexible) != nil {
		t.Fatal("empty stack must not invent parent local")
	}
	// sticky error after stack pick → ERROR_GUARD
	f.Stack = []*Block{{}}
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	if vs.SelectParentLocal(AccessRead, cg, GetIntType(), &q, NewRngSess(testAmbientSession, 1), MatchFlexible) != nil {
		t.Fatal("sticky error must fail SelectParentLocal")
	}
}

func TestExpandCheckUnregisteredKindFailClosed(t *testing.T) {
	// PartialExpander.cpp:137 — kinds not in expands_ map fail closed under partial mode
	ClearPartialExpanderSess(testAmbientSession)
	if !InitPartialExpander("for") {
		t.Fatal("init")
	}
	// Goto/Break not in expands_ → ExpandCheck false (filter rejects)
	if ExpandCheck(StmtGoto) || ExpandCheck(StmtBreak) {
		t.Fatal("unregistered kinds must not soft invent allow")
	}
	ClearPartialExpanderSess(testAmbientSession)
}

func TestVariableCreationProbability10(t *testing.T) {
	// flipcoin(10): seed scan for at least one global and mostly local
	opts := Defaults()
	opts.GlobalVariables = true
	var nG, nL int
	for seed := uint64(1); seed <= 200; seed++ {
		if VariableCreationProbability(NewRngSess(testAmbientSession, seed), opts) == ScopeGlobal {
			nG++
		} else {
			nL++
		}
	}
	if nG == 0 || nL == 0 {
		t.Fatalf("expected mix global/local got g=%d l=%d", nG, nL)
	}
	// ~10% global: allow wide band
	if nG > 60 {
		t.Fatalf("too many global %d/200", nG)
	}
}
