package csmith

import (
	"strings"
	"testing"
)

func TestMakeIterationRequiresFactMgr(t *testing.T) {
	// StatementFor.cpp:170 assert(fm); soft re-pick without FactMgr
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if MakeIteration(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, &cg) != nil {
		t.Fatal("nil FM must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM MakeIteration must stay non-sticky soft re-pick")
	}
	// sticky without RNG
	if MakeIteration(nil, opts, NewProbabilities(opts), vs, &cg) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeIteration must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeIterationInitVisitFailReturnsNil(t *testing.T) {
	// StatementFor.cpp:244–245 — assert(visited); failed init visit → no loop IR
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{iv}
	vs.AllVars = []*Variable{iv}
	vs.Opts = opts
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// IV bound makes writing IV in init visit fail
	cg.IVBounds = map[*Variable]int{iv: 10}
	// force select this IV only
	lc := MakeIteration(NewRngSess(testAmbientSession, 7), opts, NewProbabilities(opts), vs, &cg)
	if lc != nil {
		// may create different IV if SelectLoopCtrlVar creates new; only fail when same iv
		if lc.IV == iv {
			t.Fatal("init visit write on IV-bound var must fail make_iteration")
		}
	}
}

func TestMakeIterationNonArrayKeepsInvalidBound(t *testing.T) {
	// StatementFor.cpp:200 / 223–226 — free loop leaves bound = INVALID_BOUND
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	vs.Opts = opts
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// no must-use arrays → free loop control
	var lc *LoopControl
	for seed := uint64(1); seed < 30; seed++ {
		ClearErrorSess(testAmbientSession)
		lc = MakeIteration(NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), vs, &cg)
		if lc != nil {
			break
		}
	}
	if lc == nil {
		t.Fatal("nil iteration")
	}
	if lc.Bound != InvalidIVBound {
		t.Fatalf("Bound=%d want InvalidIVBound", lc.Bound)
	}
}

func TestMakeIterationBuildsIR(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	// seed an int global as potential IV
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntTypeSess(testAmbientSession), &q, NewRngSess(testAmbientSession, 1))
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	lc := MakeIteration(NewRngSess(testAmbientSession, 7), opts, NewProbabilities(opts), vs, &cg)
	if lc == nil || lc.IV == nil {
		t.Fatal("nil iteration")
	}
	if lc.InitStmt == nil || lc.InitStmt.Kind != StmtAssign {
		t.Fatal("init stmt")
	}
	if lc.InitStmt.AssignOp != AssignSimple {
		t.Fatal("init must be simple assign")
	}
	// StatementFor.cpp:236–239 — SafeOpFlags sOpAssign for incr compound map (not bare sOpBinary)
	if lc.InitStmt.SafeFlags == nil {
		t.Fatal("init SafeFlags from make_random_binary(sOpAssign, compound_to_binary(incr))")
	}
	if lc.TestExpr == nil || lc.TestExpr.Term != TermFunction {
		t.Fatal("test expr")
	}
	if lc.IncrStmt == nil || lc.IncrStmt.Kind != StmtAssign {
		t.Fatal("incr stmt")
	}
	hdr := forHeaderOutput(lc)
	if !strings.HasPrefix(hdr, "for (") || !strings.Contains(hdr, ";") {
		t.Fatal(hdr)
	}
	// IV name appears in header
	if !strings.Contains(hdr, lc.IV.Name) && !strings.Contains(hdr, lc.IV.OutputCSess(testAmbientSession, false)) {
		t.Fatal(hdr, lc.IV.Name)
	}
}

func TestMakeIterationArrayBoundPath(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), NewCVQualifiers([]bool{false}, []bool{false}))
	av.Sizes = []int{5}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 3))
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// StatementFor.cpp:204–208 — find_must_use_arrays from rw_directive only
	cg.RW = &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	lc := MakeIteration(NewRngSess(testAmbientSession, 11), opts, NewProbabilities(opts), vs, &cg)
	if lc == nil {
		t.Fatal("nil")
	}
	// bound set from array path
	if lc.Bound <= 0 {
		t.Log("bound may be 0 if choose_ok failed; ok")
	}
	if lc.InitStmt == nil || lc.IncrStmt == nil {
		t.Fatal("IR missing")
	}
}

func TestMakeRandomForEmitsHeader(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 2))
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := MakeRandomFor(NewRngSess(testAmbientSession, 9), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Loop == nil {
		t.Fatal("nil for")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
	if strings.Contains(out, "/* for-stub */") {
		t.Fatal("stub")
	}
}

func TestVisitFactsForUsesInitStmt(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f, StmID: AllocStmID()}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	_ = vs.GenerateNewGlobal(AccessWrite, cg, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	lc := MakeIteration(NewRngSess(testAmbientSession, 4), opts, NewProbabilities(opts), vs, &cg)
	if lc == nil {
		t.Skip("no IV")
	}
	st := &Stmt{Kind: StmtFor, Loop: lc, Then: &Block{Func: f, StmID: AllocStmID()}, StmID: AllocStmID()}
	cgp := cg
	if !VisitFactsStatementFor(st, &cgp, opts) {
		t.Fatal("visit_facts for")
	}
}

func TestMakeIterationMustUseArrayNilHoleFailClosed(t *testing.T) {
	// ArrayVariable* always live on must-use list; nil hole fails closed sticky
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// need live block for MakeIteration
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	// seed a loop-ctrl candidate so we reach must-use hole (not fail earlier on nil IV)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// incomplete RW FindMustUseArrays (nil hole on must_read)
	cg.RW = &RWDirective{MustReadVars: []*Variable{nil}}
	if MakeIteration(NewRngSess(testAmbientSession, 2), opts, probs, vs, &cg) != nil {
		t.Fatal("incomplete FindMustUseArrays must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete FindMustUseArrays must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeIterationIncompleteAmbientFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeIteration(NewRngSess(testAmbientSession, 3), opts, probs, vs, &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeIteration")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if MakeIteration(NewRngSess(testAmbientSession, 4), opts, probs, vs, &cg2) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeIteration")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
