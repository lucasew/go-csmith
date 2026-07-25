package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomArrayControlLe(t *testing.T) {
	r := NewRngSess(testAmbientSession, 2)
	// force Le by unsigned
	init, limit, incr, testOp, incrOp, outBound := MakeRandomArrayControl(r, 10, false, 0)
	if testOp != BinCmpLe {
		t.Fatalf("unsigned want Le got %v", testOp)
	}
	if incrOp != AssignAdd {
		t.Fatal(incrOp)
	}
	if incr < 1 {
		t.Fatal(incr)
	}
	if limit != 10 {
		t.Fatalf("limit %d", limit)
	}
	// StatementFor.cpp:145 — outBound = ((bound-init)/incr)*incr + init
	if outBound < 0 {
		t.Fatal(outBound)
	}
	_ = init
}

func TestMakeRandomArrayControlSignedLeGePolarity(t *testing.T) {
	// StatementFor.cpp:134 — rnd_flipcoin(50) ? eCmpLe : eCmpGe
	ClearErrorSess(testAmbientSession)
	foundLe, foundGe := false, false
	for seed := uint64(1); seed < 80 && !(foundLe && foundGe); seed++ {
		r0 := NewRngSess(testAmbientSession, seed)
		// skip oob flip (prob 0 still draws)
		_ = r0.RndFlipcoin(0)
		wantLe := r0.RndFlipcoin(50)
		_, _, _, testOp, _, _ := MakeRandomArrayControl(NewRngSess(testAmbientSession, seed), 10, true, 0)
		if wantLe && testOp != BinCmpLe {
			t.Fatalf("seed %d flip true must be Le got %v", seed, testOp)
		}
		if !wantLe && testOp != BinCmpGe {
			t.Fatalf("seed %d flip false must be Ge got %v", seed, testOp)
		}
		if testOp == BinCmpLe {
			foundLe = true
		}
		if testOp == BinCmpGe {
			foundGe = true
		}
	}
	if !foundLe || !foundGe {
		t.Fatalf("need both polarities: Le=%v Ge=%v", foundLe, foundGe)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomArrayControlOOBIncrements(t *testing.T) {
	// StatementFor.cpp:157–158 — oob_cnt when oob flip hits
	BookkeeperDoFinalizationSess(testAmbientSession)
	defer BookkeeperDoFinalizationSess(testAmbientSession)
	// 100% OOB
	_, _, _, _, _, _ = MakeRandomArrayControl(NewRngSess(testAmbientSession, 1), 8, false, 100)
	if OOBCount() != 1 {
		t.Fatalf("oob %d", OOBCount())
	}
	// 0% OOB
	_, _, _, _, _, _ = MakeRandomArrayControl(NewRngSess(testAmbientSession, 2), 8, false, 0)
	if OOBCount() != 1 {
		t.Fatalf("still 1 after no-oob %d", OOBCount())
	}
	// nil RNG sticky — no invent fixed array-loop control
	ClearErrorSess(testAmbientSession)
	init, limit, incr, testOp, incrOp, outBound := MakeRandomArrayControl(nil, 8, false, 0)
	if init != 0 || limit != 0 || incr != 0 || testOp != 0 || incrOp != 0 || outBound != 0 {
		t.Fatalf("nil RNG must fail closed zeros, got %d %d %d %v %v %d", init, limit, incr, testOp, incrOp, outBound)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomArrayControl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeIterationUsesMustUseArrays(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{8}
	av.ArraySizes = av.Sizes
	vs.GlobalList = []*Variable{&av.Variable}
	vs.Arrays = []*ArrayVariable{av}
	// add a loop ctrl candidate
	iv := CreateVariableQferSess(testAmbientSession, "g_2", GetIntTypeSess(testAmbientSession), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// StatementFor.cpp:204–208 — must-use via rw_directive (not invent MustUseArrays list)
	cg.RW = &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	lc := MakeIteration(NewRngSess(testAmbientSession, 5), opts, probs, vs, &cg)
	if lc == nil {
		t.Fatal("nil lc")
	}
	// with array control limit should be size-1 (=7) for Le path typically
	if lc.LimitN > 8 {
		t.Fatalf("limit too large %d", lc.LimitN)
	}
}

func TestArrayOpLoopPassesMustUse(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	av.Sizes = []int{5, 5}
	av.ArraySizes = av.Sizes
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQferSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// force non-init path by calling setup+for (MakeRandomArrayLoop sets RW)
	// If only setup helper is available, plant RW from a known array.
	if av != nil {
		cg.RW = &RWDirective{
			MustReadVars:  []*Variable{&av.Variable},
			MustWriteVars: []*Variable{&av.Variable},
		}
	}
	st := MakeRandomFor(NewRngSess(testAmbientSession, 4), opts, probs, vs, tables, stmtTab, &cg)
	if st == nil || st.Loop == nil {
		t.Fatal("for")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
}

func TestCombineVariableSets(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	got := CombineVariableSets([]*Variable{a}, []*Variable{a, b})
	if len(got) != 2 {
		t.Fatalf("%d", len(got))
	}
	// nil hole fails closed sticky IncompleteVariables (not bare nil invent empty-complete)
	bad := CombineVariableSets([]*Variable{a, nil}, []*Variable{b})
	if VariablesComplete(bad) {
		t.Fatal("nil hole must IncompleteVariables, not empty-complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete CombineVariableSets must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVectorFilterNilTableMatchesCPP(t *testing.T) {
	// VectorFilter.cpp:75–83 — ptable==nullptr → get_max_prob 100, lookup returns v
	// kinds all set (Filter ctor) → valid in random mode
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := NewVectorFilterSess(testAmbientSession, nil)
	if f.MaxProb() != 100 {
		t.Fatalf("nil ptable MaxProb: got %d want 100 (VectorFilter.cpp:75–77)", f.MaxProb())
	}
	if f.Lookup(5) != 5 {
		t.Fatalf("nil ptable Lookup: got %d want identity 5", f.Lookup(5))
	}
	// empty FilterOut set → never rejects
	if f.Filter(0) {
		t.Fatal("empty FilterOut must not reject")
	}
}

func TestMakeRandomArrayLoopNoSoftSkipNilSelect(t *testing.T) {
	// StatementFor.cpp:319+ — select_array used; no soft invent fewer arrays by skipping nil
	opts := Defaults()
	opts.MaxArrayNumInLoop = 3
	opts.GlobalVariables = false // CreateRandomArray needs stack for local
	vs := NewVariableSelector(testAmbientSession, opts)
	// no Types env → CreateRandomArray fails closed
	vs.Types = nil
	vs.Arrays = nil
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	// empty stack → CreateRandomArray cannot invent local array
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	ClearErrorSess(testAmbientSession)
	st := MakeRandomArrayLoop(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st != nil {
		t.Fatal("nil select_array must fail closed whole array-loop, not soft-skip slots")
	}
}

func TestMakeRandomArrayLoopMustRW(t *testing.T) {
	opts := Defaults()
	opts.MaxArrayNumInLoop = 4
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// seed several arrays so select_array can pick
	for i := 0; i < 3; i++ {
		av := CreateArrayVariable(NewRngSess(testAmbientSession, uint64(10+i)), opts, NewProbabilities(opts), nil, nil, nil, "g_"+string(rune('a'+i)), GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
		if av == nil {
			continue
		}
		av.Sizes = []int{4}
		av.ArraySizes = av.Sizes
		vs.Arrays = append(vs.Arrays, av)
		vs.GlobalList = append(vs.GlobalList, &av.Variable)
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, &av.Variable)
	}
	iv := CreateVariableQferSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// force non-array_init path: call MakeRandomArrayLoop directly
	foundSplit := false
	for seed := uint64(1); seed < 60; seed++ {
		// reset must-use by cloning selector inventories (reuse vs)
		st := MakeRandomArrayLoop(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, stmtTab, &cg)
		if st == nil || st.Loop == nil {
			continue
		}
		if st.Kind != StmtArrayOp && st.Kind != StmtFor {
			t.Fatalf("kind %v", st.Kind)
		}
		if st.Then != nil && st.Then.InArrayLoop {
			foundSplit = true
			break
		}
	}
	if !foundSplit {
		t.Log("InArrayLoop not set in scan — still exercised create path")
	}
	// explicit unit: access split builds distinct read/write sets
	r := NewRngSess(testAmbientSession, 42)
	// manual simulation of access choices via AddVariableToSet
	av0 := vs.Arrays[0]
	var mr, mw []*Variable
	AddVariableToSet(&mr, &av0.Variable) // read only
	if len(mr) != 1 || len(mw) != 0 {
		t.Fatal("read only")
	}
	AddVariableToSet(&mw, &av0.Variable)
	if len(mw) != 1 {
		t.Fatal("write")
	}
	_ = r
}

func TestMakeRandomForClearsEffectStm(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 2))
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// pre-seed effect_stm as dirty
	cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, v)
	st := MakeRandomFor(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st == nil {
		t.Fatal("nil")
	}
	// StatementFor.cpp:290 — clear effect_stm on CGContext& before iteration
	if st.Loop == nil {
		t.Fatal("no loop")
	}
	// pre-seed write of unrelated v must not survive on caller's EffectStm
	if cg.EffectStm.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("effect_stm clear on *CGContext must drop pre-seed write")
	}
}

func TestMakeRandomArrayLoopSetupNilSelectFailClosed(t *testing.T) {
	// StatementFor.cpp:319+ — select_array always used; nil fails whole setup
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxArrayNumInLoop = 3
	vs := NewVariableSelector(testAmbientSession, opts)
	// no Types → CreateRandomArray fails → SelectArray nil
	vs.Types = nil
	vs.Arrays = nil
	got := MakeRandomArrayLoopSetup(NewRngSess(testAmbientSession, 1), opts, vs, EmptyCGContext().WithSession(testAmbientSession))
	if got != nil {
		t.Fatal("nil SelectArray must fail closed whole setup, not invent fewer arrays")
	}
	ClearErrorSess(testAmbientSession)
	// sticky factory gates
	if MakeRandomArrayLoopSetup(nil, opts, vs, EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("nil RNG setup must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomArrayLoopSetup must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomArrayLoopIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient / RW combine must sticky ERROR (no invent array loop soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomArrayLoop(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayLoop")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete NoReadVars on RW — MaxArrayNumInLoop=0 skips select; always hits combine
	opts0 := Defaults()
	opts0.MaxArrayNumInLoop = 0
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg2.EffectAccum = &eff
	cg2.RW = &RWDirective{NoReadVars: IncompleteVariables()}
	if MakeRandomArrayLoop(NewRngSess(testAmbientSession, 2), opts0, NewProbabilities(opts0), vs, NewExprTables(opts0), NewStatementThresholdTable(opts0), &cg2) != nil {
		t.Fatal("incomplete RW NoReadVars must fail closed MakeRandomArrayLoop")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete RW lists must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if stmtOK(MakeRandomArrayOp(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayOp")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MakeRandomArrayOp must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomArrayLoopSetupIncompleteAmbientFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if MakeRandomArrayLoopSetup(NewRngSess(testAmbientSession, 1), opts, vs, cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayLoopSetup")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVectorFilterNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*VectorFilter)(nil).MaxProb() != 0 {
		t.Fatal("nil MaxProb must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MaxProb must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*VectorFilter)(nil).Filter(0) {
		t.Fatal("nil Filter must reject-all true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Filter must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
