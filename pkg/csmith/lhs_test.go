package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLhsSelectsOrCreates(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	r := NewRng(3)
	cg := EmptyCGContext()
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
	env := &TypeEnv{}
	vs.Types = env
	// seed an int* global — qfer depth must be indirect+1 (pointer: 2 levels)
	p := env.FindPointerType(GetIntType(), true)
	q := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	pv := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), p, &q, NewRng(1))
	if pv == nil {
		t.Fatal("no ptr global")
	}
	// force high deref probability by trying many seeds
	var got *Lhs
	for seed := uint64(1); seed < 100; seed++ {
		got = MakeRandomLhs(NewRng(seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext(); return &c }(), GetIntType(), false, false, nil)
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
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	broken := CreateVariableScalars("g_broken", GetIntType(), true, false)
	broken.Type = nil
	vs.GlobalList = []*Variable{broken}
	vs.AllVars = []*Variable{broken}
	cg := EmptyCGContext()
	// may create a new var instead of broken; must never accept Type-nil
	for seed := uint64(1); seed < 20; seed++ {
		ClearError()
		lhs := MakeRandomLhs(NewRng(seed), opts, probs, vs, &cg, GetIntType(), false, false, nil)
		if lhs != nil && lhs.Var == broken {
			t.Fatal("Type-nil var must not be accepted as Lhs")
		}
	}
	// sticky after incomplete type IR in select — clear for suite
	ClearError()
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
	ClearError()
	if !broken.IsVolatile() {
		t.Fatal("incomplete Lhs IsVolatile must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete Lhs IsVolatile must SetError sticky")
	}
	ClearError()
	if VariablesComplete(broken.GetLvars(nil)) {
		t.Fatal("GetLvars incomplete must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("GetLvars incomplete must SetError sticky")
	}
	ClearError()
	if VariablesComplete((&Lhs{}).GetReferencedPtrs()) {
		t.Fatal("GetReferencedPtrs incomplete Lhs must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("GetReferencedPtrs incomplete must SetError sticky")
	}
	ClearError()
	eBroken := &Expression{Term: TermVariable, Var: &Variable{Name: "y"}}
	if _, ok := eBroken.IndirectLevelComplete(); ok {
		t.Fatal("expr incomplete type must fail closed")
	}
}

func TestPickUnaryOp(t *testing.T) {
	// FunctionInvocation.cpp:146 — UNARY_OPS_PROB_FILTER from process Probabilities
	opts := Defaults()
	prev := ProcessProbabilities()
	SetProcessProbabilities(NewProbabilities(opts))
	defer SetProcessProbabilities(prev)
	seen := map[UnaryOp]bool{}
	r := NewRng(1)
	for i := 0; i < 100; i++ {
		seen[PickUnaryOp(r, opts)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("seen %v", seen)
	}
	opts.UnaryPlusOperator = false
	SetProcessProbabilities(NewProbabilities(opts))
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
	ClearError()
	BookkeeperDoFinalization()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	vs.Types = env
	p := env.FindPointerType(GetIntType(), true)
	// pointer type needs two-level qfer for SanityCheck / MakeInitValue
	q := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), p, &q, NewRng(1))
	ClearError() // sticky ERROR_GUARD on failed qfer/create must not poison Lhs make
	// bump deref prob
	probs.single[PSelectDerefPointerProb] = 100
	for seed := uint64(1); seed < 80; seed++ {
		ClearError()
		_ = MakeRandomLhs(NewRng(seed), opts, probs, vs, func() *CGContext { c := EmptyCGContext(); return &c }(), GetIntType(), false, false, nil)
	}
	if CalcTotal(writeDereferenceCnts) == 0 && writeVolatileCnt+writeNonVolatileCnt == 0 {
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
		lhs := MakeRandomLhs(NewRng(seed), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext(); return &c }(), GetIntType(), true, true, nil)
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
	cg := EmptyCGContext()
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
	lhs := MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, func() *CGContext { c := EmptyCGContext(); return &c }(), GetIntType(), false, false, &q)
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
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
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
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	if MakeRandomLhs(nil, opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomLhs must SetError sticky")
	}
	ClearError()
	if MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, nil, false, false, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type MakeRandomLhs must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomLhsIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent LHS / soft re-pick past holes)
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomLhs(NewRng(1), opts, NewProbabilities(opts), vs, &cg, GetIntType(), false, false, nil) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomLhs")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm2 := NewFactMgr(f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	if MakeRandomLhs(NewRng(2), opts, NewProbabilities(opts), vs, &cg2, GetIntType(), false, false, nil) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomLhs")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
	// incomplete GlobalList hole fails closed sticky via selectWritable pool scan
	vs2 := NewVariableSelector(opts)
	vs2.GlobalList = []*Variable{g, nil}
	cg3 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if MakeRandomLhs(NewRng(3), opts, NewProbabilities(opts), vs2, &cg3, GetIntType(), false, false, nil) != nil {
		t.Fatal("incomplete GlobalList must fail closed MakeRandomLhs/selectWritable")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalList must SetError sticky")
	}
	ClearError()
}

func TestLhsOutputNilSticky(t *testing.T) {
	ClearError()
	if (*Lhs)(nil).Output(false) != "" {
		t.Fatal("nil Lhs Output must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Lhs Output must SetError sticky")
	}
	ClearError()
	if (&Lhs{}).Output(false) != "" {
		t.Fatal("nil Var Lhs Output must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Var Lhs Output must SetError sticky")
	}
	ClearError()
}
