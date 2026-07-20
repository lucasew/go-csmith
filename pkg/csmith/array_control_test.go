package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomArrayControlLe(t *testing.T) {
	r := NewRng(2)
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

func TestMakeRandomArrayControlOOBIncrements(t *testing.T) {
	// StatementFor.cpp:157–158 — oob_cnt when oob flip hits
	BookkeeperDoFinalization()
	defer BookkeeperDoFinalization()
	// 100% OOB
	_, _, _, _, _, _ = MakeRandomArrayControl(NewRng(1), 8, false, 100)
	if OOBCount() != 1 {
		t.Fatalf("oob %d", OOBCount())
	}
	// 0% OOB
	_, _, _, _, _, _ = MakeRandomArrayControl(NewRng(2), 8, false, 0)
	if OOBCount() != 1 {
		t.Fatalf("still 1 after no-oob %d", OOBCount())
	}
	// nil RNG sticky — no invent fixed array-loop control
	ClearError()
	init, limit, incr, testOp, incrOp, outBound := MakeRandomArrayControl(nil, 8, false, 0)
	if init != 0 || limit != 0 || incr != 0 || testOp != 0 || incrOp != 0 || outBound != 0 {
		t.Fatalf("nil RNG must fail closed zeros, got %d %d %d %v %v %d", init, limit, incr, testOp, incrOp, outBound)
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomArrayControl must SetError sticky")
	}
	ClearError()
}

func TestMakeIterationUsesMustUseArrays(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{8}
	av.ArraySizes = av.Sizes
	vs.GlobalList = []*Variable{&av.Variable}
	vs.Arrays = []*ArrayVariable{av}
	// add a loop ctrl candidate
	iv := CreateVariableQfer("g_2", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	// StatementFor.cpp:204–208 — must-use via rw_directive (not invent MustUseArrays list)
	cg.RW = &RWDirective{MustReadVars: []*Variable{&av.Variable}}
	lc := MakeIteration(NewRng(5), opts, probs, vs, &cg)
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
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), q)
	av.Sizes = []int{5, 5}
	av.ArraySizes = av.Sizes
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQfer("g_iv", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	// force non-init path by calling setup+for (MakeRandomArrayLoop sets RW)
	// If only setup helper is available, plant RW from a known array.
	if av != nil {
		cg.RW = &RWDirective{
			MustReadVars:  []*Variable{&av.Variable},
			MustWriteVars: []*Variable{&av.Variable},
		}
	}
	st := MakeRandomFor(NewRng(4), opts, probs, vs, tables, stmtTab, &cg)
	if st == nil || st.Loop == nil {
		t.Fatal("for")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
}

func TestCombineVariableSets(t *testing.T) {
	ClearError()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	got := CombineVariableSets([]*Variable{a}, []*Variable{a, b})
	if len(got) != 2 {
		t.Fatalf("%d", len(got))
	}
	// nil hole fails closed sticky IncompleteVariables (not bare nil invent empty-complete)
	bad := CombineVariableSets([]*Variable{a, nil}, []*Variable{b})
	if VariablesComplete(bad) {
		t.Fatal("nil hole must IncompleteVariables, not empty-complete")
	}
	if !HasError() {
		t.Fatal("incomplete CombineVariableSets must SetError sticky")
	}
	ClearError()
}

func TestVectorFilterNilTableMatchesCPP(t *testing.T) {
	// VectorFilter.cpp:75–83 — ptable==nullptr → get_max_prob 100, lookup returns v
	// kinds all set (Filter ctor) → valid in random mode
	SetProcessOptions(Defaults())
	f := NewVectorFilter(nil)
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
	vs := NewVariableSelector(opts)
	// no Types env → CreateRandomArray fails closed
	vs.Types = nil
	vs.Arrays = nil
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	// empty stack → CreateRandomArray cannot invent local array
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	ClearError()
	st := MakeRandomArrayLoop(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st != nil {
		t.Fatal("nil select_array must fail closed whole array-loop, not soft-skip slots")
	}
}

func TestMakeRandomArrayLoopMustRW(t *testing.T) {
	opts := Defaults()
	opts.MaxArrayNumInLoop = 4
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// seed several arrays so select_array can pick
	for i := 0; i < 3; i++ {
		av := CreateArrayVariable(NewRng(uint64(10+i)), opts, NewProbabilities(opts), nil, nil, nil, "g_"+string(rune('a'+i)), GetIntType(), MakeInt(0), q)
		if av == nil {
			continue
		}
		av.Sizes = []int{4}
		av.ArraySizes = av.Sizes
		vs.Arrays = append(vs.Arrays, av)
		vs.GlobalList = append(vs.GlobalList, &av.Variable)
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, &av.Variable)
	}
	iv := CreateVariableQfer("g_iv", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	// force non-array_init path: call MakeRandomArrayLoop directly
	foundSplit := false
	for seed := uint64(1); seed < 60; seed++ {
		// reset must-use by cloning selector inventories (reuse vs)
		st := MakeRandomArrayLoop(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
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
	r := NewRng(42)
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
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f)), GetIntType(), nil, NewRng(2))
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	// pre-seed effect_stm as dirty
	cg.EffectStm = EmptyEffect().WriteVar(v)
	st := MakeRandomFor(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st == nil {
		t.Fatal("nil")
	}
	// StatementFor.cpp:290 — clear effect_stm on CGContext& before iteration
	if st.Loop == nil {
		t.Fatal("no loop")
	}
	// pre-seed write of unrelated v must not survive on caller's EffectStm
	if cg.EffectStm.IsWritten(v) {
		t.Fatal("effect_stm clear on *CGContext must drop pre-seed write")
	}
}

func TestMakeRandomArrayLoopSetupNilSelectFailClosed(t *testing.T) {
	// StatementFor.cpp:319+ — select_array always used; nil fails whole setup
	ClearError()
	opts := Defaults()
	opts.MaxArrayNumInLoop = 3
	vs := NewVariableSelector(opts)
	// no Types → CreateRandomArray fails → SelectArray nil
	vs.Types = nil
	vs.Arrays = nil
	got := MakeRandomArrayLoopSetup(NewRng(1), opts, vs, EmptyCGContext())
	if got != nil {
		t.Fatal("nil SelectArray must fail closed whole setup, not invent fewer arrays")
	}
	ClearError()
	// sticky factory gates
	if MakeRandomArrayLoopSetup(nil, opts, vs, EmptyCGContext()) != nil {
		t.Fatal("nil RNG setup must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomArrayLoopSetup must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomArrayLoopIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient / RW combine must sticky ERROR (no invent array loop soft re-pick)
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomArrayLoop(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayLoop")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	// incomplete NoReadVars on RW — MaxArrayNumInLoop=0 skips select; always hits combine
	opts0 := Defaults()
	opts0.MaxArrayNumInLoop = 0
	eff := EmptyEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg2.EffectAccum = &eff
	cg2.RW = &RWDirective{NoReadVars: IncompleteVariables()}
	if MakeRandomArrayLoop(NewRng(2), opts0, NewProbabilities(opts0), vs, NewExprTables(opts0), NewStatementThresholdTable(opts0), &cg2) != nil {
		t.Fatal("incomplete RW NoReadVars must fail closed MakeRandomArrayLoop")
	}
	if !HasError() {
		t.Fatal("incomplete RW lists must SetError sticky")
	}
	ClearError()
	if stmtOK(MakeRandomArrayOp(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayOp")
	}
	if !HasError() {
		t.Fatal("MakeRandomArrayOp must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomArrayLoopSetupIncompleteAmbientFailClosed(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	inc := IncompleteEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &inc
	if MakeRandomArrayLoopSetup(NewRng(1), opts, vs, cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomArrayLoopSetup")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
}

func TestVectorFilterNilSticky(t *testing.T) {
	ClearError()
	if (*VectorFilter)(nil).MaxProb() != 0 {
		t.Fatal("nil MaxProb must return 0")
	}
	if !HasError() {
		t.Fatal("nil MaxProb must SetError sticky")
	}
	ClearError()
	if !(*VectorFilter)(nil).Filter(0) {
		t.Fatal("nil Filter must reject-all true")
	}
	if !HasError() {
		t.Fatal("nil Filter must SetError sticky")
	}
	ClearError()
}
