package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLhsSelectsOrCreates(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	r := NewRngSess(testAmbientSession, 3)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	lhs := MakeRandomLhs(r, opts, probs, vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
	if lhs == nil || lhs.Var == nil {
		t.Fatal("nil lhs")
	}
	if lhs.GetTypeSess(testAmbientSession) == nil {
		t.Fatal("nil type")
	}
	if lhs.Var.Name == "" {
		t.Fatal("empty name")
	}
}

func TestMakeRandomLhsDerefPointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	vs.Types = env
	// seed an int* global — qfer depth must be indirect+1 (pointer: 2 levels)
	p := env.FindPointerType(GetIntTypeSess(testAmbientSession), true)
	q := NewCVQualifiersSess(testAmbientSession, []bool{false, false}, []bool{false, false})
	pv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), p, &q, NewRngSess(testAmbientSession, 1))
	if pv == nil {
		t.Fatal("no ptr global")
	}
	// force high deref probability by trying many seeds
	var got *Lhs
	for seed := uint64(1); seed < 100; seed++ {
		got = MakeRandomLhs(NewRngSess(testAmbientSession, seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntTypeSess(testAmbientSession), false, false, nil)
		if got != nil && got.Var != nil && got.Var.Type != nil && got.Var.Type.IndirectLevelSess(testAmbientSession) == 1 && got.IndirectLevelSess(testAmbientSession) == 1 {
			break
		}
		got = nil
	}
	if got == nil {
		t.Skip("deref path rare")
	}
	out := got.OutputSess(testAmbientSession, false)
	if !strings.Contains(out, "*") {
		t.Fatal(out)
	}
}

func TestMakeRandomLhsRejectsNilVarType(t *testing.T) {
	// Variable::type always live; Type-nil candidate must not soft-skip filters
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), true, false)
	broken.Type = nil
	vs.GlobalList = []*Variable{broken}
	vs.AllVars = []*Variable{broken}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// Type-nil in pool stickies select residual; whole MakeRandomLhs fails closed
	// (no invent soft fall-through create/accept past incomplete type IR)
	for seed := uint64(1); seed < 20; seed++ {
		ClearErrorSess(testAmbientSession)
		lhs := MakeRandomLhs(NewRngSess(testAmbientSession, seed), opts, probs, vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
		if lhs != nil && lhs.Var == broken {
			t.Fatal("Type-nil var must not be accepted as Lhs")
		}
	}
	// sticky after incomplete type IR in select — clear for suite
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomLhsResidualSticky(t *testing.T) {
	// residual ERROR soft-continue invents Lhs via fall-through select / later try.
	// Fair: sticky fail closed whole MakeRandomLhs.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	// must_use Type-nil stickies SelectMustUseVar residual; must not invent soft select Lhs
	broken := CreateVariableScalarsSess(testAmbientSession, "g_broken", GetIntTypeSess(testAmbientSession), true, false)
	broken.Type = nil
	good := CreateVariableScalarsSess(testAmbientSession, "g_good", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{good}
	vs.AllVars = []*Variable{good}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	rw := &RWDirective{MustWriteVars: []*Variable{broken, good}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, probs, vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("must-use Type-nil residual must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must-use residual MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray shell in must-use: SelectMustUse stickies residual;
	// soft invent was fall through soft select invent Lhs from good global.
	shell := &Variable{
		Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2},
		Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}),
	}
	rw2 := &RWDirective{MustWriteVars: []*Variable{shell, good}}
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw2)
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if MakeRandomLhs(NewRngSess(testAmbientSession, 2), opts, probs, vs, &cg2, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("IsArray without AsArray must-use residual must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray residual MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputVolLval(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	out := lhs.OutputSess(testAmbientSession, true)
	if !strings.Contains(out, "VOL_LVAL(g_v") {
		t.Fatal(out)
	}
}

func TestLhsIndirectLevel(t *testing.T) {
	p := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	v := CreateVariableQferSess(testAmbientSession, "g_p", p, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	if lhs.IndirectLevelSess(testAmbientSession) != 1 {
		t.Fatal(lhs.IndirectLevelSess(testAmbientSession))
	}
	if lhs.OutputSess(testAmbientSession, false) != "(*g_p)" {
		t.Fatal(lhs.OutputSess(testAmbientSession, false))
	}
	// incomplete Lhs type IR: Complete false sticky (no invent level 0 as complete)
	broken := &Lhs{Var: &Variable{Name: "x"}} // Type nil
	if _, ok := broken.IndirectLevelCompleteSess(testAmbientSession); ok {
		t.Fatal("nil Var.Type must fail closed Incomplete")
	}
	ClearErrorSess(testAmbientSession)
	if !broken.IsVolatileSess(testAmbientSession) {
		t.Fatal("incomplete Lhs IsVolatile must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Lhs IsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Lhs shell sticky restrictive volatile + empty quals
	if !(*Lhs)(nil).IsVolatileSess(testAmbientSession) {
		t.Fatal("nil Lhs IsVolatile must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs IsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if q := (*Lhs)(nil).GetQualifiersSess(testAmbientSession); len(q.IsConsts) != 0 {
		t.Fatal("nil Lhs GetQualifiers must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// GetCollective residual soft invent was soft-merge invent complete GetLvars list.
	// IsArray without AsArray stickies GetCollective.
	arrShell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	lhsArr := &Lhs{Var: arrShell, Type: GetIntTypeSess(testAmbientSession)}
	if VariablesComplete(lhsArr.GetLvarsSess(testAmbientSession, nil)) {
		t.Fatal("GetCollective residual GetLvars must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetCollective residual GetLvars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(broken.GetLvarsSess(testAmbientSession, nil)) {
		t.Fatal("GetLvars incomplete must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetLvars incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete((&Lhs{}).GetReferencedPtrsSess(testAmbientSession)) {
		t.Fatal("GetReferencedPtrs incomplete Lhs must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetReferencedPtrs incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Var soft invent: IsPointer residual ERROR+false → complete empty no-ptrs
	// fair: sticky IncompleteVariables before classify
	tyNilLhs := &Lhs{Var: &Variable{Name: "g_typeless"}}
	if VariablesComplete(tyNilLhs.GetReferencedPtrsSess(testAmbientSession)) {
		t.Fatal("GetReferencedPtrs Type-nil must fail closed incomplete, not empty-complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetReferencedPtrs Type-nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	eBroken := &Expression{Term: TermVariable, Var: &Variable{Name: "y"}}
	if _, ok := eBroken.IndirectLevelCompleteSess(testAmbientSession); ok {
		t.Fatal("expr incomplete type must fail closed")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPickUnaryOp(t *testing.T) {
	// FunctionInvocation.cpp:146 — UNARY_OPS_PROB_FILTER from process Probabilities
	opts := Defaults()
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	seen := map[UnaryOp]bool{}
	r := NewRngSess(testAmbientSession, 1)
	for i := 0; i < 100; i++ {
		seen[PickUnaryOpSess(testAmbientSession, r, opts)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("seen %v", seen)
	}
	opts.UnaryPlusOperator = false
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	r2 := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 50; i++ {
		if PickUnaryOpSess(testAmbientSession, r2, opts) == UnPlus {
			t.Fatal("unary plus disabled")
		}
	}
}

func TestExpressionVariableDerefOutput(t *testing.T) {
	p := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	v := CreateVariableQferSess(testAmbientSession, "g_1", p, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}
	out := e.OutputSess(testAmbientSession)
	if out != "(*g_1)" {
		t.Fatalf("%q", out)
	}
}

func TestExpressionVariableAddrOutput(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	// ind = 0 - 1 = -1
	out := e.OutputSess(testAmbientSession)
	if out != "&g_1" {
		t.Fatalf("%q", out)
	}
}

func TestExpressionVariableMultiLevelAddrFailClosed(t *testing.T) {
	// ExpressionVariable.cpp:211 — assert(indirect_level == -1); no invent single &
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))}
	// ind = 0 - 2 = -2
	if e.IndirectLevelSess(testAmbientSession) != -2 {
		t.Fatalf("indir %d", e.IndirectLevelSess(testAmbientSession))
	}
	if out := e.OutputSess(testAmbientSession); out != "" {
		t.Fatalf("multi-level & must fail closed, got %q", out)
	}
}

func TestLhsBookkeepingWriteDeref(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalizationSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	vs.Types = env
	p := env.FindPointerType(GetIntTypeSess(testAmbientSession), true)
	// pointer type needs two-level qfer for SanityCheck / MakeInitValue
	q := NewCVQualifiersSess(testAmbientSession, []bool{false, false}, []bool{false, false})
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), p, &q, NewRngSess(testAmbientSession, 1))
	ClearErrorSess(testAmbientSession) // sticky ERROR_GUARD on failed qfer/create must not poison Lhs make
	// bump deref prob
	probs.single[PSelectDerefPointerProb] = 100
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		_ = MakeRandomLhs(NewRngSess(testAmbientSession, seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntTypeSess(testAmbientSession), false, false, nil)
	}
	if CalcTotal(currentSession().BK.writeDereferenceCnts) == 0 && currentSession().BK.writeVolatileCnt+currentSession().BK.writeNonVolatileCnt == 0 {
		t.Fatal("expected some write bookkeeping")
	}
}

func TestMakeRandomLhsNoSignedOverflow(t *testing.T) {
	// Lhs.cpp:110–113 — no_signed_overflow rejects signed / bitfield for ++/--
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// only a signed int global
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	// force create of unsigned if signed rejected — or nil after tries
	// with only signed available and noSignedOverflow, must not return signed
	foundSigned := false
	for seed := uint64(1); seed < 15; seed++ {
		lhs := MakeRandomLhs(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntTypeSess(testAmbientSession), true, true, nil)
		if lhs == nil {
			continue
		}
		base := lhs.Var.Type.BaseTypeSess(testAmbientSession)
		if base != nil && base.IsSignedSess(testAmbientSession) && !lhs.Var.IsBitfield {
			// new variables might be created unsigned; if same g_1 then fail
			if lhs.Var == g {
				foundSigned = true
				break
			}
		}
	}
	if foundSigned {
		t.Fatal("no_signed_overflow must reject signed g_1 for need_no_rhs ops")
	}
}

func TestMakeRandomLhsRejectsWrittenInEffectStm(t *testing.T) {
	// Lhs.cpp:105 — !effect_stm.is_written(var)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, g)
	// with only g written in stm, make may create another var or fail
	lhs := MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
	if lhs != nil && lhs.Var == g {
		t.Fatal("must not select var already written in effect_stm")
	}
}

func TestMakeRandomLhsUsesProvidedQferWildcard(t *testing.T) {
	// Lhs.cpp:90–93 — wildcard skips Restrict
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	q.Wildcard = true
	lhs := MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntTypeSess(testAmbientSession), false, false, &q)
	if lhs == nil {
		t.Fatal("nil")
	}
}

func TestMakeRandomLhsMutatesCallerEffect(t *testing.T) {
	// Lhs::make_random visit_facts must update caller's effect_accum / effect_stm
	// so StatementAssign can merge_param_context(lhs_cg).
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	lhs := MakeRandomLhs(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
	if lhs == nil || lhs.Var == nil {
		t.Skip("no lhs")
	}
	// Scalar WRITE → IsWritten(lhs.Var); *p WRITE → WritePointed records pointees,
	// not the pointer itself (Lhs.cpp:337–346 / CheckWriteVar).
	hasWrite := false
	if cg.EffectAccum != nil {
		hasWrite = len(cg.EffectAccum.WrittenVarsSess(testAmbientSession)) > 0 || len(cg.EffectAccum.LhsWriteVarsSess(testAmbientSession)) > 0
	}
	if !hasWrite && len(cg.EffectStm.WrittenVarsSess(testAmbientSession)) == 0 {
		t.Fatalf("expected write effect after lhs make (var %s indir=%d)", lhs.Var.Name, lhs.IndirectLevelSess(testAmbientSession))
	}
}

func TestMakeRandomLhsNilGatesSticky(t *testing.T) {
	// Lhs::make_random always has type + RNG + VS + CG sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	if MakeRandomLhs(nil, opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	// nil RNG MakeRandomLhs must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	if MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &cg, nil, false, false, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomLhsIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent LHS / soft re-pick past holes)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	if MakeRandomLhs(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &cg2, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalList hole fails closed sticky via selectWritable pool scan
	vs2 := NewVariableSelector(testAmbientSession, opts)
	vs2.GlobalList = []*Variable{g, nil}
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if MakeRandomLhs(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs2, &cg3, GetIntTypeSess(testAmbientSession), false, false, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeRandomLhs/selectWritable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalList must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectWritableNilTypSticky(t *testing.T) {
	// nil want typ soft invent was soft-skip Match as not-match then empty pool soft-miss.
	// Fair: sticky fail closed whole selectWritable.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if selectWritable(NewRngSess(testAmbientSession, 1), vs, cg, nil, false) != nil {
		t.Fatal("nil typ must fail closed selectWritable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil typ selectWritable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).OutputSess(testAmbientSession, false) != "" {
		t.Fatal("nil Lhs Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{}).OutputSess(testAmbientSession, false) != "" {
		t.Fatal("nil Var Lhs Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var Lhs Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsGetVarGetTypeNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).GetVarSess(testAmbientSession) != nil {
		t.Fatal("nil Lhs GetVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{}).GetVarSess(testAmbientSession) != nil {
		t.Fatal("nil Var Lhs GetVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var Lhs GetVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("nil Lhs GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{Var: &Variable{Name: "x"}}).GetTypeSess(testAmbientSession) != nil {
		t.Fatal("Lhs without type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Lhs without type GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsAsExpressionIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if LhsAsExpressionSess(testAmbientSession, nil) != nil {
		t.Fatal("nil Lhs LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if LhsAsExpressionSess(testAmbientSession, &Lhs{}) != nil {
		t.Fatal("Lhs without Var LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Lhs without Var LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	e := LhsAsExpressionSess(testAmbientSession, &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)})
	if e == nil || e.Var != v {
		t.Fatal("complete LhsAsExpression must return TermVariable")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete LhsAsExpression must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputAccessResidualSticky(t *testing.T) {
	// OutputAccess residual soft invent was soft-empty invent bare/partial LHS.
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession)}, Sizes: []int{2}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true},
		Sizes:      []int{2},
		Collective: parent,
		IndexExprs: []*Expression{{Term: TermConstant, Con: &Constant{Value: "0"}}}, // Type-nil residual
	}
	item.AsArray = item
	lhs := &Lhs{Var: &item.Variable, Type: GetIntTypeSess(testAmbientSession)}
	if s := lhs.OutputSess(testAmbientSession, false); s != "" {
		t.Fatal("OutputAccess residual must fail closed Lhs.Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputAccess residual Lhs.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// OutputLhsC residual same hole
	if s := item.OutputLhsCOptsSess(testAmbientSession, false); s != "" {
		t.Fatal("OutputAccess residual must fail closed OutputLhsC", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputAccess residual OutputLhsC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was soft-wrap invent VOL_LVAL(name, invented type).
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	lhs := &Lhs{Var: v, Type: &Type{isStruct: true}} // unnamed struct → CName residual
	if s := lhs.OutputSess(testAmbientSession, true); s != "" {
		t.Fatal("CName residual must fail closed Lhs.Output VOL_LVAL wrap", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CName residual Lhs.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputLhsCAccessResidualSticky(t *testing.T) {
	// covered by TestLhsOutputAccessResidualSticky OutputLhsC arm; keep Complete path hygiene.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_ok", GetIntTypeSess(testAmbientSession), false, false)
	if s := v.OutputLhsCOptsSess(testAmbientSession, false); s != "g_ok" {
		t.Fatal("complete OutputLhsC", s)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete OutputLhsC must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsLhsGetTypeResidualSticky(t *testing.T) {
	// GetType residual soft invent was invent compound-read visit success past Type-nil Lhs.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession), Body: &Block{}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	// Type-nil Lhs + Type-nil Var → GetType residual
	lhs := &Lhs{Var: &Variable{Name: "g_x", Type: nil, Qfer: NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})}, Type: nil, CompoundAssign: true}
	if cg.VisitFactsLhs(lhs, opts) {
		t.Fatal("GetType residual must fail closed VisitFactsLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsAsExpressionTypeNilResidualSticky(t *testing.T) {
	// Type-nil Lhs soft invent was invent TermVariable shell past incomplete type IR.
	ClearErrorSess(testAmbientSession)
	if LhsAsExpressionSess(testAmbientSession, &Lhs{Var: &Variable{Name: "g_x", Type: nil}, Type: nil}) != nil {
		t.Fatal("Type-nil LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete path
	v := CreateVariableScalarsSess(testAmbientSession, "g_y", GetIntTypeSess(testAmbientSession), false, false)
	e := LhsAsExpressionSess(testAmbientSession, &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)})
	if e == nil || e.Var != v {
		t.Fatal("complete LhsAsExpression")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete LhsAsExpression must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitIndicesAddEffectResidualSticky(t *testing.T) {
	// AddEffect residual soft invent was invent index visit under incomplete ambient merge.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession), Body: &Block{}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	// incomplete EffectStm → AddEffect residual sticky before index visit
	cg.EffectStm = IncompleteEffect()
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true}, Sizes: []int{2}}
	av.AsArray = av
	av.IndexExprs = []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: GetIntTypeSess(testAmbientSession)}}
	av.Variable.AsArray = av
	l := &Lhs{Var: &av.Variable, Type: GetIntTypeSess(testAmbientSession)}
	if l.VisitIndices(&cg, opts) {
		t.Fatal("incomplete EffectStm AddEffect residual must fail closed VisitIndices")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm AddEffect residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCompatibleVarResidualSticky(t *testing.T) {
	// Compatible residual soft invent was invent soft-compat past nil other.
	ClearErrorSess(testAmbientSession)
	lhs := &Lhs{Var: CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)}
	if lhs.CompatibleVarSess(testAmbientSession, nil, false) {
		t.Fatal("nil other CompatibleVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil other CompatibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsIndirectLevelCompleteResidualSticky(t *testing.T) {
	// IndirectLevel residual soft invent was invent level-0 complete past Type-nil shell.
	ClearErrorSess(testAmbientSession)
	l := &Lhs{Var: &Variable{Name: "g_x", Type: nil}, Type: GetIntTypeSess(testAmbientSession)}
	n, ok := l.IndirectLevelCompleteSess(testAmbientSession)
	if ok || n != 0 {
		t.Fatal("Type-nil Lhs IndirectLevelComplete must fail closed 0,false", n, ok)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Lhs IndirectLevelComplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Lhs
	n2, ok2 := (*Lhs)(nil).IndirectLevelCompleteSess(testAmbientSession)
	if ok2 || n2 != 0 {
		t.Fatal("nil Lhs must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs IndirectLevelComplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsCloneDereferencedComplexity(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)} // deref once: pointer → int
	cl := lhs.CloneSess(testAmbientSession)
	if cl == nil || cl.Var != v || cl == lhs {
		t.Fatal(cl)
	}
	if lhs.GetComplexitySess(testAmbientSession) != 0 {
		t.Fatal(lhs.GetComplexitySess(testAmbientSession))
	}
	ptrs := lhs.GetDereferencedPtrsSess(testAmbientSession)
	if len(ptrs) != 1 || ptrs[0].Var != v {
		t.Fatal(ptrs)
	}
	// bare non-deref
	sc := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	bare := &Lhs{Var: sc, Type: GetIntTypeSess(testAmbientSession)}
	if len(bare.GetDereferencedPtrsSess(testAmbientSession)) != 0 {
		t.Fatal("no deref")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomLhsFilterRejectKeepsIsEligiblePollution(t *testing.T) {
	// Lhs.cpp:103–122,139 — valid=false: dummy only (no reset_effect_*).
	// VariableSelector.cpp:221–227 — is_eligible itemized read_indices pollutes
	// shared effect_accum via cg_tmp. Soft invent restore() on filter reject wiped
	// that pollution so map_stm_effect missed outer-loop IV reads (seed-46 g_952.f8).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	iv := CreateVariableScalarsSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), false, false)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{8}},
		Sizes:    []int{8},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{8}},
		Sizes:      []int{8},
		Collective: parent,
		Indices:    []string{"g_iv"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	ok := CreateVariableScalarsSess(testAmbientSession, "l_ok", GetIntTypeSess(testAmbientSession), false, false)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)

	// Precondition: is_eligible on itemized writes g_iv into effect_accum.
	acc := EmptyEffect()
	cgElig := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cgElig.EffectAccum = &acc
	_ = IsEligibleVar(&item.Variable, 0, AccessWrite, cgElig)
	if !acc.IsReadSess(testAmbientSession, iv) {
		t.Fatal("precondition: is_eligible itemized must ReadVar index IV on effect_accum")
	}

	// Residual pollution (as after filter-reject keep) must survive a successful Lhs
	// that does not itself re-run is_eligible on the itemized member.
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = []*Variable{ok}
	vs.AllVars = []*Variable{ok}
	eff := EmptyEffect().ReadVarSess(testAmbientSession, iv)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	lhs := MakeRandomLhs(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &cg, GetIntTypeSess(testAmbientSession), false, false, nil)
	if lhs == nil || HasErrorSess(testAmbientSession) {
		t.Fatalf("expected Lhs from l_ok-only pool err=%v", HasErrorSess(testAmbientSession))
	}
	if !cg.EffectAccum.IsReadSess(testAmbientSession, iv) {
		t.Fatal("filter-reject path must not wipe residual is_eligible effect_accum reads")
	}
}
