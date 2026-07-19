package csmith

import (
	"testing"
)

func TestGenerateBodyWithKnownParamsSetsRW(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	g := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
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
	prev := WithFunc(caller, EmptyEffect()).WithFactMgr(fm)
	prev.EffectAccum = &accum
	prev.RW = &RWDirective{NoWriteVars: []*Variable{g}}

	callee := &Function{
		Name:       "func_2",
		ReturnType: GetIntType(),
		Param:      []*Variable{CreateVariableScalars("p_1", GetIntType(), false, false)},
	}
	callee.RV = CreateVariableQfer("func_2_rv", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	_ = callee.ensurePairedFactMgr()
	// handover facts empty for calFM from signature pairing
	callee.GenerateBodyWithKnownParams(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), prev)
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

func TestGenerateBodyBuiltinDummy(t *testing.T) {
	opts := Defaults()
	f := &Function{
		Name:       "__builtin_clz",
		ReturnType: GetIntType(),
		IsBuiltin:  true,
	}
	// Function.cpp:757–758 FMList at create; GenerateBody uses get_fact_mgr (no invent)
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext())
	if f.Body == nil {
		t.Fatal("dummy body")
	}
	if f.BuildState != BuildBuilt {
		t.Fatal(f.BuildState)
	}
}

func TestGenerateBodyFailsClosedWithoutFactMgr(t *testing.T) {
	// Function.cpp:635 get_fact_mgr_for_func; null → fail closed (no invent NewFactMgr)
	f := &Function{Name: "func_x", ReturnType: GetIntType()}
	f.GenerateBody(NewRng(1), Defaults(), NewProbabilities(Defaults()), NewVariableSelector(Defaults()), NewExprTables(Defaults()), NewStatementThresholdTable(Defaults()), EmptyCGContext())
	if f.Body != nil || f.BuildState == BuildBuilt {
		t.Fatal("must not invent body/FM without paired FactMgr")
	}
}

func TestGenerateBodyNoInventWithoutRNG(t *testing.T) {
	// Function.cpp non-builtin make_random body always has process RNG
	f := &Function{Name: "func_x", ReturnType: GetIntType()}
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(nil, Defaults(), NewProbabilities(Defaults()), NewVariableSelector(Defaults()), NewExprTables(Defaults()), NewStatementThresholdTable(Defaults()), EmptyCGContext())
	if f.Body != nil || f.BuildState != BuildUnbuilt {
		t.Fatalf("nil RNG must not invent body/Built, state=%v body=%v", f.BuildState, f.Body != nil)
	}
}

func TestGenerateBodyIncompleteFailClosedNoBuilt(t *testing.T) {
	// incomplete Param hole / mark_func_end must not invent Built or stuck Building
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "func_w", ReturnType: GetIntType(), Param: []*Variable{nil}}
	_ = f.ensurePairedFactMgr()
	f.GenerateBody(NewRng(2), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext().WithFactMgr(f.PairedFactMgr()))
	if f.BuildState == BuildBuilt || f.IsBuilt {
		t.Fatal("Param nil hole must not invent Built")
	}
	if f.BuildState == BuildBuilding {
		t.Fatal("must not leave stuck Building after fail closed")
	}
	if !HasError() {
		t.Fatal("Param nil hole must SetError")
	}
	ClearError()
	// incomplete GlobalFacts at mark_func_end when Blocks non-empty
	f2 := &Function{Name: "func_v", ReturnType: GetIntType(), IsBuiltin: true}
	fm2 := f2.ensurePairedFactMgr()
	fm2.GlobalFacts = IncompleteFactSlice()
	f2.GenerateBody(NewRng(3), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), EmptyCGContext().WithFactMgr(fm2))
	if len(f2.Blocks) > 0 && (f2.BuildState == BuildBuilt || f2.IsBuilt) {
		t.Fatal("incomplete GlobalFacts at mark_func_end must not invent Built")
	}
	ClearError()
}

func TestGenerateBodyIncompleteAmbientSticky(t *testing.T) {
	// incomplete prev ambient must not invent Building/Built body under hole shells
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "func_amb", ReturnType: GetIntType()}
	_ = f.ensurePairedFactMgr()
	prev := WithFunc(f, IncompleteEffect()).WithFactMgr(f.PairedFactMgr())
	f.GenerateBody(NewRng(4), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), prev)
	if f.BuildState == BuildBuilt || f.IsBuilt || f.BuildState == BuildBuilding {
		t.Fatal("incomplete EffectContext must not invent Built/Building")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomSignaturePairsFactMgr(t *testing.T) {
	// Function.cpp:422 — FMList.push_back at make_random_signature
	opts := Defaults()
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, NewProbabilities(opts), vs, nil)
	f := MakeRandomSignature(NewRng(2), opts, NewProbabilities(opts), vs, &vs.Sym, EmptyCGContext(), GetIntType(), nil, nil)
	if f == nil {
		t.Fatal("sig")
	}
	if f.PairedFactMgr() == nil {
		t.Fatal("signature must pair FactMgr")
	}
}

func TestMakeReturnConst(t *testing.T) {
	ClearError()
	defer ClearError()
	opts := Defaults()
	opts.DepthProtect = true
	probs := NewProbabilities(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.MakeReturnConst(opts, probs, NewRng(1))
	if f.RetConst == nil {
		t.Fatal("want ret const")
	}
	// void — no
	f2 := &Function{Name: "v", ReturnType: GetSimpleType(EVoid)}
	f2.MakeReturnConst(opts, probs, NewRng(1))
	if f2.RetConst != nil {
		t.Fatal("void no const")
	}
	// aggregate ret + nil probs — no invent NewProbabilities(opts); ERROR_RETURN
	st := &Type{isStruct: true, StructName: "SRet", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	f3 := &Function{Name: "s", ReturnType: st}
	f3.MakeReturnConst(opts, nil, NewRng(1))
	if f3.RetConst != nil {
		t.Fatal("nil probs must not invent aggregate ret_c")
	}
	if !HasError() {
		t.Fatal("nil probs aggregate must ERROR_RETURN for GenerateBody")
	}
	ClearError()
	// nil RNG — no invent "0"; sticky error for GenerateBody
	f4 := &Function{Name: "n", ReturnType: GetIntType()}
	f4.MakeReturnConst(opts, probs, nil)
	if f4.RetConst != nil || !HasError() {
		t.Fatal("nil RNG must fail closed with sticky error")
	}
	ClearError()
}

func TestMakeExpressionCommaNilLHSType(t *testing.T) {
	// ExpressionComma lhs type nullptr → choose_random_nonvoid needs Type env
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	e := func() *Expression {
		c := EmptyCGContext().WithFactMgr(NewFactMgr(nil))
		c.Types = vs.Types
		return MakeExpressionComma(NewRng(3), opts, probs, vs, NewExprTables(opts), &c, GetIntType(), nil)
	}()
	if e == nil || e.Term != TermCommaExpr {
		t.Fatal(e)
	}
	if e.CommaLHS == nil || e.CommaRHS == nil {
		t.Fatal("sides")
	}
}
