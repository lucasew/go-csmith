package csmith

import (
	"testing"
)

func TestGenerateBodyWithKnownParamsSetsRW(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	// caller context with a no-write on global
	caller := &Function{Name: "func_1"}
	cblk := &Block{Func: caller}
	caller.Stack = []*Block{cblk}
	// Function.cpp:422 FMList at create — paired FactMgr, not invent at GenerateBody
	fm := caller.ensurePairedFactMgr()
	if g != nil {
		fm.AddNewVarFact(g)
	}
	accum := EmptyEffect()
	prev := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	prev.EffectAccum = &accum
	prev.RW = &RWDirective{NoWriteVars: []*Variable{g}}

	callee := &Function{
		Name:       "func_2",
		ReturnType: GetIntTypeSess(testAmbientSession),
		Param:      []*Variable{CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)},
	}
	callee.RV = CreateVariableQferSess(testAmbientSession, "func_2_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee.ensurePairedFactMgr()
	// handover facts empty for calFM from signature pairing
	callee.GenerateBodyWithKnownParams(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	if callee.BuildState != BuildBuilt {
		t.Fatal(callee.BuildState)
	}
	if callee.Body == nil {
		t.Fatal("no body")
	}
	// set_depth_protect(true) on body
	if !callee.Body.EmitDepthProtect {
		t.Fatal("want EmitDepthProtect on body")
	}
}

func TestGenerateBodyResetsBlkDepth(t *testing.T) {
	// Function.cpp:633 — CGContext(this, effect_context, &accum) sets blk_depth(0)
	// even when caller context is nested (blk_depth>0).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 5
	vs := NewVariableSelector(testAmbientSession, opts)
	caller := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = caller.ensurePairedFactMgr()
	prev := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(caller.PairedFactMgr())
	prev.BlkDepth = 4 // nested in caller
	prev.ExprDepth = 7
	prev.Flags = FlagInLoop
	callee := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession)}
	callee.RV = CreateVariableQferSess(testAmbientSession, "func_2_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee.ensurePairedFactMgr()
	// Capture depth at body generation via a thin check: body must be buildable
	// at MaxBlockDepth without inventing max-depth filter (would fail with depth 4 inherit).
	callee.GenerateBody(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if callee.BuildState != BuildBuilt || callee.Body == nil {
		t.Fatal(callee.BuildState, callee.Body)
	}
	// Caller depth must be unchanged (body uses fresh 0)
	if prev.BlkDepth != 4 || prev.ExprDepth != 7 {
		t.Fatalf("caller depth mutated: blk=%d expr=%d", prev.BlkDepth, prev.ExprDepth)
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateBodyClearsIVBounds(t *testing.T) {
	// Function.cpp:633 — new CGContext has empty iv_bounds, not caller's loops
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	iv := CreateVariableScalarsSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), false, false)
	caller := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = caller.ensurePairedFactMgr()
	prev := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(caller.PairedFactMgr())
	prev.AddIVBound(iv, 0) // leftover loop IV as if caller nested
	if len(prev.IVBounds) != 1 {
		t.Fatal(prev.IVBounds)
	}
	callee := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession)}
	callee.RV = CreateVariableQferSess(testAmbientSession, "func_2_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee.ensurePairedFactMgr()
	callee.GenerateBody(NewRngSess(testAmbientSession, 4), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	// caller map still has IV; callee generation must not keep sharing that map
	if _, ok := prev.IVBounds[iv]; !ok {
		t.Fatal("caller IVBounds must remain")
	}
	if callee.BuildState != BuildBuilt {
		t.Fatal(callee.BuildState)
	}
	// After body gen, prev still has its IV (not cleared by callee)
	if len(prev.IVBounds) != 1 {
		t.Fatalf("caller IVBounds mutated: %v", prev.IVBounds)
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateBodyDropsCallerMustUseArrays(t *testing.T) {
	// Function.cpp:66–69 CGContext ctor rw_directive(nullptr).
	// Function.cpp:679–685 known-params RW: empty must_reads/writes + external no-*.
	// Soft invent was cg := prev keeping prev.RW.Must* so callee make_iteration
	// took array_control (StatementFor.cpp:204–225) while upstream loop_control.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, nil, nil, "g_16", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	if av == nil {
		t.Fatal("array")
	}
	caller := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	cblk := &Block{Func: caller}
	caller.Stack = []*Block{cblk}
	fm := caller.ensurePairedFactMgr()
	fm.AddNewVarFact(&av.Variable)
	accum := EmptyEffect()
	prev := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	prev.EffectAccum = &accum
	// Caller is mid array-loop: must-use g_16 (and optional no-write).
	prev.RW = &RWDirective{
		MustReadVars: []*Variable{&av.Variable},
		NoWriteVars:  []*Variable{&av.Variable},
	}
	// BuildCallee keeps no-write for reachable frame globals; must stay empty.
	rwd := prev.BuildCalleeRWDirective(fm.GlobalFacts)
	if rwd == nil {
		t.Fatal("expect non-nil external RW from no-write on global array")
	}
	if got := rwd.FindMustUseArrays(); len(got) != 0 {
		t.Fatalf("BuildCalleeRW must not copy must-use arrays, got %d", len(got))
	}
	if len(rwd.NoWriteVars) == 0 {
		t.Fatal("want external no-write preserved")
	}

	// known-params path must install external no-* only (empty must).
	callee := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession)}
	callee.RV = CreateVariableQferSess(testAmbientSession, "func_2_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee.ensurePairedFactMgr()
	callee.GenerateBodyWithKnownParams(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if callee.BuildState != BuildBuilt || callee.Body == nil {
		t.Fatal(callee.BuildState, callee.Body)
	}
	// Caller must-use list must remain (not cleared/stolen by body setup).
	if len(prev.RW.MustReadVars) != 1 || prev.RW.MustReadVars[0] != &av.Variable {
		t.Fatalf("caller must-read mutated: %v", prev.RW.MustReadVars)
	}

	// GenerateBody (!knownParams): ctor leaves RW nil — never inherit must.
	callee2 := &Function{Name: "func_3", ReturnType: GetIntTypeSess(testAmbientSession)}
	callee2.RV = CreateVariableQferSess(testAmbientSession, "func_3_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee2.ensurePairedFactMgr()
	callee2.GenerateBody(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if callee2.BuildState != BuildBuilt {
		t.Fatal(callee2.BuildState)
	}
	if len(prev.RW.MustReadVars) != 1 {
		t.Fatal("caller must-read must remain after GenerateBody")
	}

	// MakeIteration under empty-must RW takes free loop_control (not Itemize on g_16).
	// StatementFor.cpp:204–225 — only rw_directive->find_must_use_arrays.
	blk := &Block{Func: callee, LocalVars: []*Variable{}}
	callee.Stack = []*Block{blk}
	cg := WithFunc(callee, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(callee.PairedFactMgr())
	cg.RW = rwd // external no-write only (post–BuildCallee)
	// IV pool needs a non-array int
	iv := CreateVariableScalarsSess(testAmbientSession, "g_77", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = append(vs.GlobalList, iv)
	ClearErrorSess(testAmbientSession)
	lc := MakeIteration(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, &cg)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if lc == nil {
		t.Fatal("MakeIteration nil")
	}
	// array path sets Bound from shortest dim; free loop leaves InvalidIVBound / 0 convention
	// StatementFor.cpp:200 bound=INVALID_BOUND then only array path rewrites it.
	if lc.Bound != 0 && lc.Bound != InvalidIVBound {
		// If must leaked, bound would be array size path (e.g. 7 for g_16[7]).
		// Free control keeps InvalidIVBound (0 after assign in Go loop-control branch).
		t.Fatalf("MakeIteration with empty-must RW must not take array bound, bound=%d", lc.Bound)
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateBodyBuiltinDummy(t *testing.T) {
	opts := Defaults()
	f := &Function{
		Name:       "__builtin_clz",
		ReturnType: GetIntTypeSess(testAmbientSession),
		IsBuiltin:  true,
	}
	// Function.cpp:757–758 FMList at create; GenerateBody uses get_fact_mgr (no invent)
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), EmptyCGContext().WithSession(testAmbientSession))
	if f.Body == nil {
		t.Fatal("dummy body")
	}
	if f.BuildState != BuildBuilt {
		t.Fatal(f.BuildState)
	}
}

func TestGenerateBodyFailsClosedWithoutFactMgr(t *testing.T) {
	// Function.cpp:635 get_fact_mgr_for_func; null → fail closed (no invent NewFactMgr)
	f := &Function{Name: "func_x", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.GenerateBody(NewRngSess(testAmbientSession, 1), Defaults(), NewProbabilities(Defaults()), NewVariableSelector(testAmbientSession, Defaults()), NewExprTablesSess(testAmbientSession, Defaults()), NewStatementThresholdTable(Defaults()), EmptyCGContext().WithSession(testAmbientSession))
	if f.Body != nil || f.BuildState == BuildBuilt {
		t.Fatal("must not invent body/FM without paired FactMgr")
	}
}

func TestGenerateBodyNoInventWithoutRNG(t *testing.T) {
	// Function.cpp non-builtin make_random body always has process RNG
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_x", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(nil, Defaults(), NewProbabilities(Defaults()), NewVariableSelector(testAmbientSession, Defaults()), NewExprTablesSess(testAmbientSession, Defaults()), NewStatementThresholdTable(Defaults()), EmptyCGContext().WithSession(testAmbientSession))
	if f.Body != nil || f.BuildState != BuildUnbuilt {
		t.Fatalf("nil RNG must not invent body/Built, state=%v body=%v", f.BuildState, f.Body != nil)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG GenerateBody must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Function always live; sticky (no invent soft-skip body gen past hole)
	(*Function)(nil).GenerateBody(NewRngSess(testAmbientSession, 1), Defaults(), NewProbabilities(Defaults()), NewVariableSelector(testAmbientSession, Defaults()), NewExprTablesSess(testAmbientSession, Defaults()), NewStatementThresholdTable(Defaults()), EmptyCGContext().WithSession(testAmbientSession))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function GenerateBody must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateBodyIncompleteFailClosedNoBuilt(t *testing.T) {
	// incomplete Param hole / mark_func_end must not invent Built or stuck Building
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "func_w", ReturnType: GetIntTypeSess(testAmbientSession), Param: []*Variable{nil}}
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(NewRngSess(testAmbientSession, 2), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(f.PairedFactMgr()))
	if f.BuildState == BuildBuilt || f.IsBuilt {
		t.Fatal("Param nil hole must not invent Built")
	}
	if f.BuildState == BuildBuilding {
		t.Fatal("must not leave stuck Building after fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Param nil hole must SetError")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil param soft invent: IsPointer residual ERROR+false skip TBD seed
	// then soft-continue later params / partial makeup. Fair: sticky abort first.
	fTy := &Function{
		Name:       "func_ty",
		ReturnType: GetIntTypeSess(testAmbientSession),
		Param: []*Variable{
			{Name: "p_typeless"}, // Type nil non-special
			CreateVariableScalarsSess(testAmbientSession, "p_ok", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false),
		},
	}
	fmTy := fTy.ensurePairedFactMgr()
	fTy.GenerateBody(NewRngSess(testAmbientSession, 5), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fmTy))
	if fTy.BuildState == BuildBuilt || fTy.IsBuilt {
		t.Fatal("Type-nil param must not invent Built")
	}
	if fTy.BuildState == BuildBuilding {
		t.Fatal("Type-nil param must not leave stuck Building")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil param GenerateBody must SetError sticky")
	}
	// no invent partial TBD seed for later pointer param past Type-nil shell
	if FindRelatedPointToSess(testAmbientSession, fmTy.GlobalFacts, fTy.Param[1]) != nil {
		t.Fatal("Type-nil param must not soft-seed later pointer TBD past hole")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts at mark_func_end when Blocks non-empty
	f2 := &Function{Name: "func_v", ReturnType: GetIntTypeSess(testAmbientSession), IsBuiltin: true}
	fm2 := f2.ensurePairedFactMgr()
	fm2.GlobalFacts = IncompleteFactSlice()
	f2.GenerateBody(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm2))
	if len(f2.Blocks) > 0 && (f2.BuildState == BuildBuilt || f2.IsBuilt) {
		t.Fatal("incomplete GlobalFacts at mark_func_end must not invent Built")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateBodyIncompleteAmbientSticky(t *testing.T) {
	// incomplete prev ambient must not invent Building/Built body under hole shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "func_amb", ReturnType: GetIntTypeSess(testAmbientSession)}
	_ = f.ensurePairedFactMgr()
	prev := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession).WithFactMgr(f.PairedFactMgr())
	f.GenerateBody(NewRngSess(testAmbientSession, 4), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), prev)
	if f.BuildState == BuildBuilt || f.IsBuilt || f.BuildState == BuildBuilding {
		t.Fatal("incomplete EffectContext must not invent Built/Building")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomSignaturePairsFactMgr(t *testing.T) {
	// Function.cpp:422 — FMList.push_back at make_random_signature
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, nil)
	f := MakeRandomSignature(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &vs.Sym, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil)
	if f == nil {
		t.Fatal("sig")
	}
	if f.PairedFactMgr() == nil {
		t.Fatal("signature must pair FactMgr")
	}
}

func TestMakeReturnConst(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.DepthProtect = true
	probs := NewProbabilities(opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.MakeReturnConst(opts, probs, NewRngSess(testAmbientSession, 1))
	if f.RetConst == nil {
		t.Fatal("want ret const")
	}
	// void — no
	f2 := &Function{Name: "v", ReturnType: GetSimpleTypeSess(testAmbientSession, EVoid)}
	f2.MakeReturnConst(opts, probs, NewRngSess(testAmbientSession, 1))
	if f2.RetConst != nil {
		t.Fatal("void no const")
	}
	// aggregate ret + nil probs — no invent NewProbabilities(opts); ERROR_RETURN
	st := &Type{isStruct: true, StructName: "SRet", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	f3 := &Function{Name: "s", ReturnType: st}
	f3.MakeReturnConst(opts, nil, NewRngSess(testAmbientSession, 1))
	if f3.RetConst != nil {
		t.Fatal("nil probs must not invent aggregate ret_c")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil probs aggregate must ERROR_RETURN for GenerateBody")
	}
	ClearErrorSess(testAmbientSession)
	// nil RNG — no invent "0"; sticky error for GenerateBody
	f4 := &Function{Name: "n", ReturnType: GetIntTypeSess(testAmbientSession)}
	f4.MakeReturnConst(opts, probs, nil)
	if f4.RetConst != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG must fail closed with sticky error")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionCommaNilLHSType(t *testing.T) {
	// ExpressionComma lhs type nullptr → choose_random_nonvoid needs Type env
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	e := func() *Expression {
		c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
		c.Types = vs.Types
		return MakeExpressionComma(NewRngSess(testAmbientSession, 3), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), &c, GetIntTypeSess(testAmbientSession), nil)
	}()
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("sides")
	}
}
