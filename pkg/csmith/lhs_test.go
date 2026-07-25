package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLhsSelectsOrCreates(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	r := NewRng(3)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	lhs := MakeRandomLhs(r, opts, probs, vs, &cg, GetIntType(), false, false, nil)
	if lhs == nil || lhs.Var == nil {
		t.Fatal("nil lhs")
	}
	if lhs.GetType() == nil {
		t.Fatal("nil type")
	}
	if lhs.Var.Name == "" {
		t.Fatal("empty name")
	}
}

func TestMakeRandomLhsDerefPointer(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	vs.Types = env
	// seed an int* global — qfer depth must be indirect+1 (pointer: 2 levels)
	p := env.FindPointerType(GetIntType(), true)
	q := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	pv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), p, &q, NewRng(1))
	if pv == nil {
		t.Fatal("no ptr global")
	}
	// force high deref probability by trying many seeds
	var got *Lhs
	for seed := uint64(1); seed < 100; seed++ {
		got = MakeRandomLhs(NewRng(seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntType(), false, false, nil)
		if got != nil && got.Var != nil && got.Var.Type != nil && got.Var.Type.IndirectLevel() == 1 && got.IndirectLevel() == 1 {
			break
		}
		got = nil
	}
	if got == nil {
		t.Skip("deref path rare")
	}
	out := got.Output(false)
	if !strings.Contains(out, "*") {
		t.Fatal(out)
	}
}

func TestMakeRandomLhsRejectsNilVarType(t *testing.T) {
	// Variable::type always live; Type-nil candidate must not soft-skip filters
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	broken := CreateVariableScalars("g_broken", GetIntType(), true, false)
	broken.Type = nil
	vs.GlobalList = []*Variable{broken}
	vs.AllVars = []*Variable{broken}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// Type-nil in pool stickies select residual; whole MakeRandomLhs fails closed
	// (no invent soft fall-through create/accept past incomplete type IR)
	for seed := uint64(1); seed < 20; seed++ {
		ClearErrorSess(testAmbientSession)
		lhs := MakeRandomLhs(NewRng(seed), opts, probs, vs, &cg, GetIntType(), false, false, nil)
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
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	// must_use Type-nil stickies SelectMustUseVar residual; must not invent soft select Lhs
	broken := CreateVariableScalars("g_broken", GetIntType(), true, false)
	broken.Type = nil
	good := CreateVariableScalars("g_good", GetIntType(), true, false)
	vs.GlobalList = []*Variable{good}
	vs.AllVars = []*Variable{good}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	rw := &RWDirective{MustWriteVars: []*Variable{broken, good}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if MakeRandomLhs(NewRng(1), opts, probs, vs, &cg, GetIntType(), false, false, nil) != nil {
		t.Fatal("must-use Type-nil residual must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must-use residual MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray shell in must-use: SelectMustUse stickies residual;
	// soft invent was fall through soft select invent Lhs from good global.
	shell := &Variable{
		Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	rw2 := &RWDirective{MustWriteVars: []*Variable{shell, good}}
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithRW(rw2)
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if MakeRandomLhs(NewRng(2), opts, probs, vs, &cg2, GetIntType(), false, false, nil) != nil {
		t.Fatal("IsArray without AsArray must-use residual must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray residual MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputVolLval(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	out := lhs.Output(true)
	if !strings.Contains(out, "VOL_LVAL(g_v") {
		t.Fatal(out)
	}
}

func TestLhsIndirectLevel(t *testing.T) {
	p := PointerTo(GetIntType())
	v := CreateVariableQfer("g_p", p, NewCVQualifiers([]bool{false}, []bool{false}))
	lhs := &Lhs{Var: v, Type: GetIntType()}
	if lhs.IndirectLevel() != 1 {
		t.Fatal(lhs.IndirectLevel())
	}
	if lhs.Output(false) != "(*g_p)" {
		t.Fatal(lhs.Output(false))
	}
	// incomplete Lhs type IR: Complete false sticky (no invent level 0 as complete)
	broken := &Lhs{Var: &Variable{Name: "x"}} // Type nil
	if _, ok := broken.IndirectLevelComplete(); ok {
		t.Fatal("nil Var.Type must fail closed Incomplete")
	}
	ClearErrorSess(testAmbientSession)
	if !broken.IsVolatile() {
		t.Fatal("incomplete Lhs IsVolatile must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Lhs IsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Lhs shell sticky restrictive volatile + empty quals
	if !(*Lhs)(nil).IsVolatile() {
		t.Fatal("nil Lhs IsVolatile must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs IsVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if q := (*Lhs)(nil).GetQualifiers(); len(q.IsConsts) != 0 {
		t.Fatal("nil Lhs GetQualifiers must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// GetCollective residual soft invent was soft-merge invent complete GetLvars list.
	// IsArray without AsArray stickies GetCollective.
	arrShell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	lhsArr := &Lhs{Var: arrShell, Type: GetIntType()}
	if VariablesComplete(lhsArr.GetLvars(nil)) {
		t.Fatal("GetCollective residual GetLvars must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetCollective residual GetLvars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(broken.GetLvars(nil)) {
		t.Fatal("GetLvars incomplete must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetLvars incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete((&Lhs{}).GetReferencedPtrs()) {
		t.Fatal("GetReferencedPtrs incomplete Lhs must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetReferencedPtrs incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Var soft invent: IsPointer residual ERROR+false → complete empty no-ptrs
	// fair: sticky IncompleteVariables before classify
	tyNilLhs := &Lhs{Var: &Variable{Name: "g_typeless"}}
	if VariablesComplete(tyNilLhs.GetReferencedPtrs()) {
		t.Fatal("GetReferencedPtrs Type-nil must fail closed incomplete, not empty-complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetReferencedPtrs Type-nil must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	eBroken := &Expression{Term: TermVariable, Var: &Variable{Name: "y"}}
	if _, ok := eBroken.IndirectLevelComplete(); ok {
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
	r := NewRng(1)
	for i := 0; i < 100; i++ {
		seen[PickUnaryOp(r, opts)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("seen %v", seen)
	}
	opts.UnaryPlusOperator = false
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	r2 := NewRng(2)
	for i := 0; i < 50; i++ {
		if PickUnaryOp(r2, opts) == UnPlus {
			t.Fatal("unary plus disabled")
		}
	}
}

func TestExpressionVariableDerefOutput(t *testing.T) {
	p := PointerTo(GetIntType())
	v := CreateVariableQfer("g_1", p, NewCVQualifiers([]bool{false}, []bool{false}))
	e := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	out := e.Output()
	if out != "(*g_1)" {
		t.Fatalf("%q", out)
	}
}

func TestExpressionVariableAddrOutput(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerTo(GetIntType())}
	// ind = 0 - 1 = -1
	out := e.Output()
	if out != "&g_1" {
		t.Fatalf("%q", out)
	}
}

func TestExpressionVariableMultiLevelAddrFailClosed(t *testing.T) {
	// ExpressionVariable.cpp:211 — assert(indirect_level == -1); no invent single &
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerTo(PointerTo(GetIntType()))}
	// ind = 0 - 2 = -2
	if e.IndirectLevel() != -2 {
		t.Fatalf("indir %d", e.IndirectLevel())
	}
	if out := e.Output(); out != "" {
		t.Fatalf("multi-level & must fail closed, got %q", out)
	}
}

func TestLhsBookkeepingWriteDeref(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	vs.Types = env
	p := env.FindPointerType(GetIntType(), true)
	// pointer type needs two-level qfer for SanityCheck / MakeInitValue
	q := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), p, &q, NewRng(1))
	ClearErrorSess(testAmbientSession) // sticky ERROR_GUARD on failed qfer/create must not poison Lhs make
	// bump deref prob
	probs.single[PSelectDerefPointerProb] = 100
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		_ = MakeRandomLhs(NewRng(seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntType(), false, false, nil)
	}
	if CalcTotal(currentSession().BK.writeDereferenceCnts) == 0 && currentSession().BK.writeVolatileCnt+currentSession().BK.writeNonVolatileCnt == 0 {
		t.Fatal("expected some write bookkeeping")
	}
}

func TestMakeRandomLhsNoSignedOverflow(t *testing.T) {
	// Lhs.cpp:110–113 — no_signed_overflow rejects signed / bitfield for ++/--
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// only a signed int global
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	// force create of unsigned if signed rejected — or nil after tries
	// with only signed available and noSignedOverflow, must not return signed
	foundSigned := false
	for seed := uint64(1); seed < 15; seed++ {
		lhs := MakeRandomLhs(NewRng(seed), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntType(), true, true, nil)
		if lhs == nil {
			continue
		}
		base := lhs.Var.Type.BaseType()
		if base != nil && base.IsSigned() && !lhs.Var.IsBitfield {
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
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectStm = EmptyEffect().WriteVar(g)
	// with only g written in stm, make may create another var or fail
	lhs := MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil)
	if lhs != nil && lhs.Var == g {
		t.Fatal("must not select var already written in effect_stm")
	}
}

func TestMakeRandomLhsUsesProvidedQferWildcard(t *testing.T) {
	// Lhs.cpp:90–93 — wildcard skips Restrict
	opts := Defaults()
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	q.Wildcard = true
	lhs := MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext().WithSession(testAmbientSession); return &c }(), GetIntType(), false, false, &q)
	if lhs == nil {
		t.Fatal("nil")
	}
}

func TestMakeRandomLhsMutatesCallerEffect(t *testing.T) {
	// Lhs::make_random visit_facts must update caller's effect_accum / effect_stm
	// so StatementAssign can merge_param_context(lhs_cg).
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	lhs := MakeRandomLhs(NewRng(2), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil)
	if lhs == nil || lhs.Var == nil {
		t.Skip("no lhs")
	}
	// Scalar WRITE → IsWritten(lhs.Var); *p WRITE → WritePointed records pointees,
	// not the pointer itself (Lhs.cpp:337–346 / CheckWriteVar).
	hasWrite := false
	if cg.EffectAccum != nil {
		hasWrite = len(cg.EffectAccum.WrittenVars()) > 0 || len(cg.EffectAccum.LhsWriteVars()) > 0
	}
	if !hasWrite && len(cg.EffectStm.WrittenVars()) == 0 {
		t.Fatalf("expected write effect after lhs make (var %s indir=%d)", lhs.Var.Name, lhs.IndirectLevel())
	}
}

func TestMakeRandomLhsNilGatesSticky(t *testing.T) {
	// Lhs::make_random always has type + RNG + VS + CG sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	if MakeRandomLhs(nil, opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, nil, false, false, nil) != nil {
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
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	if MakeRandomLhs(NewRng(2), opts, NewProbabilities(opts), vs, &cg2, GetIntType(), false, false, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalList hole fails closed sticky via selectWritable pool scan
	vs2 := NewVariableSelector(opts)
	vs2.GlobalList = []*Variable{g, nil}
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if MakeRandomLhs(NewRng(3), opts, NewProbabilities(opts), vs2, &cg3, GetIntType(), false, false, nil) != nil {
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
	vs := NewVariableSelector(opts)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if selectWritable(NewRng(1), vs, cg, nil, false) != nil {
		t.Fatal("nil typ must fail closed selectWritable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil typ selectWritable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsOutputNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).Output(false) != "" {
		t.Fatal("nil Lhs Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{}).Output(false) != "" {
		t.Fatal("nil Var Lhs Output must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var Lhs Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsGetVarGetTypeNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).GetVar() != nil {
		t.Fatal("nil Lhs GetVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{}).GetVar() != nil {
		t.Fatal("nil Var Lhs GetVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Var Lhs GetVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Lhs)(nil).GetType() != nil {
		t.Fatal("nil Lhs GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (&Lhs{Var: &Variable{Name: "x"}}).GetType() != nil {
		t.Fatal("Lhs without type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Lhs without type GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestLhsAsExpressionIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if LhsAsExpression(nil) != nil {
		t.Fatal("nil Lhs LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if LhsAsExpression(&Lhs{}) != nil {
		t.Fatal("Lhs without Var LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Lhs without Var LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	e := LhsAsExpression(&Lhs{Var: v, Type: GetIntType()})
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
	parent := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType()}, Sizes: []int{2}}
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true},
		Sizes:      []int{2},
		Collective: parent,
		IndexExprs: []*Expression{{Term: TermConstant, Con: &Constant{Value: "0"}}}, // Type-nil residual
	}
	item.AsArray = item
	lhs := &Lhs{Var: &item.Variable, Type: GetIntType()}
	if s := lhs.Output(false); s != "" {
		t.Fatal("OutputAccess residual must fail closed Lhs.Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputAccess residual Lhs.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// OutputLhsC residual same hole
	if s := item.OutputLhsC(); s != "" {
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
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	lhs := &Lhs{Var: v, Type: &Type{isStruct: true}} // unnamed struct → CName residual
	if s := lhs.Output(true); s != "" {
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
	v := CreateVariableScalars("g_ok", GetIntType(), false, false)
	if s := v.OutputLhsC(); s != "g_ok" {
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
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	// Type-nil Lhs + Type-nil Var → GetType residual
	lhs := &Lhs{Var: &Variable{Name: "g_x", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, Type: nil, CompoundAssign: true}
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
	if LhsAsExpression(&Lhs{Var: &Variable{Name: "g_x", Type: nil}, Type: nil}) != nil {
		t.Fatal("Type-nil LhsAsExpression must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil LhsAsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete path
	v := CreateVariableScalars("g_y", GetIntType(), false, false)
	e := LhsAsExpression(&Lhs{Var: v, Type: GetIntType()})
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
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	// incomplete EffectStm → AddEffect residual sticky before index visit
	cg.EffectStm = IncompleteEffect()
	av := &ArrayVariable{Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true}, Sizes: []int{2}}
	av.AsArray = av
	av.IndexExprs = []*Expression{{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}}
	av.Variable.AsArray = av
	l := &Lhs{Var: &av.Variable, Type: GetIntType()}
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
	lhs := &Lhs{Var: CreateVariableScalars("g_a", GetIntType(), false, false)}
	if lhs.CompatibleVar(nil, false) {
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
	l := &Lhs{Var: &Variable{Name: "g_x", Type: nil}, Type: GetIntType()}
	n, ok := l.IndirectLevelComplete()
	if ok || n != 0 {
		t.Fatal("Type-nil Lhs IndirectLevelComplete must fail closed 0,false", n, ok)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Lhs IndirectLevelComplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Lhs
	n2, ok2 := (*Lhs)(nil).IndirectLevelComplete()
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
	v := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	lhs := &Lhs{Var: v, Type: GetIntType()} // deref once: pointer → int
	cl := lhs.Clone()
	if cl == nil || cl.Var != v || cl == lhs {
		t.Fatal(cl)
	}
	if lhs.GetComplexity() != 0 {
		t.Fatal(lhs.GetComplexity())
	}
	ptrs := lhs.GetDereferencedPtrs()
	if len(ptrs) != 1 || ptrs[0].Var != v {
		t.Fatal(ptrs)
	}
	// bare non-deref
	sc := CreateVariableScalars("g_1", GetIntType(), true, false)
	bare := &Lhs{Var: sc, Type: GetIntType()}
	if len(bare.GetDereferencedPtrs()) != 0 {
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
	iv := CreateVariableScalars("g_iv", GetIntType(), false, false)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:    []int{8},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{8}},
		Sizes:      []int{8},
		Collective: parent,
		Indices:    []string{"g_iv"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntType()}},
	}
	item.AsArray = item
	ok := CreateVariableScalars("l_ok", GetIntType(), false, false)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)

	// Precondition: is_eligible on itemized writes g_iv into effect_accum.
	acc := EmptyEffect()
	cgElig := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cgElig.EffectAccum = &acc
	_ = IsEligibleVar(&item.Variable, 0, AccessWrite, cgElig)
	if !acc.IsRead(iv) {
		t.Fatal("precondition: is_eligible itemized must ReadVar index IV on effect_accum")
	}

	// Residual pollution (as after filter-reject keep) must survive a successful Lhs
	// that does not itself re-run is_eligible on the itemized member.
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(opts)
	vs.GlobalList = []*Variable{ok}
	vs.AllVars = []*Variable{ok}
	eff := EmptyEffect().ReadVar(iv)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	lhs := MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil)
	if lhs == nil || HasErrorSess(testAmbientSession) {
		t.Fatalf("expected Lhs from l_ok-only pool err=%v", HasErrorSess(testAmbientSession))
	}
	if !cg.EffectAccum.IsRead(iv) {
		t.Fatal("filter-reject path must not wipe residual is_eligible effect_accum reads")
	}
}
