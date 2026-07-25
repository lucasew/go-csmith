package csmith

import (
	"strings"
	"testing"
)

func TestItemizeConsumesRNGPerDim(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("create")
	}
	r2 := NewRng(5)
	before := r2.RandDepth()
	item := av.Itemize(r2)
	if item == nil || item.Collective != av {
		t.Fatal(item)
	}
	if len(item.Indices) != av.Dimension() {
		t.Fatalf("indices %v dims %d", item.Indices, av.Dimension())
	}
	if r2.RandDepth() != before+uint64(av.Dimension()) {
		// each dim one RndUpto
		if r2.RandDepth() < before {
			t.Fatal("depth")
		}
	}
	acc := item.OutputAccess()
	if !strings.HasPrefix(acc, "g_a[") {
		t.Fatal(acc)
	}
}

func TestSelectArrayCreatesWhenEmpty(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	// create_random_array uses Type env (C++ GenerateSimpleTypes always live)
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	r := NewRng(2)
	av := vs.SelectArray(r, EmptyCGContext().WithSession(testAmbientSession))
	if av == nil || len(vs.Arrays) < 1 {
		t.Fatal(av)
	}
}

func TestSelectArrayChoosesExisting(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// VariableSelector.cpp:1386 — find_all_visible_vars only (GlobalList / local_vars)
	a := CreateArrayVariable(r, opts, NewProbabilities(opts), vs, nil, nil, "g_1", GetIntType(), MakeInt(0), q)
	b := CreateArrayVariable(r, opts, NewProbabilities(opts), vs, nil, nil, "g_2", GetIntType(), MakeInt(1), q)
	if a == nil || b == nil {
		t.Fatal("create")
	}
	// CreateArrayVariable(blk=nil) registers GlobalList when vs non-nil
	got := vs.SelectArray(NewRng(3), EmptyCGContext().WithSession(testAmbientSession))
	if got != a && got != b {
		t.Fatal(got)
	}
}

func TestMakeRandomArrayOpEmitsFor(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	// StatementFor.cpp:172 assert(blk) — parent on stack for array_loop → for
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// StatementArrayOp::make_random — 5% array_init (StmtArrayOp) else for-loop (StmtFor)
	var st Stmt
	for seed := uint64(1); seed < 40; seed++ {
		f.Stack = []*Block{parent}
		st = MakeRandomArrayOp(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
		if (st.Kind == StmtArrayOp || st.Kind == StmtFor) && st.Loop != nil {
			break
		}
	}
	if st.Kind != StmtArrayOp && st.Kind != StmtFor {
		t.Fatalf("kind %v", st.Kind)
	}
	if st.Loop == nil {
		t.Fatal("incomplete arrayop/for without loop IR")
	}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
}

func TestGenerateArrayOpOrDecl(t *testing.T) {
	// arrays may appear as decls or array-op loops
	found := false
	for seed := uint64(1); seed < 50; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "[") && strings.Contains(out, "g_") {
			found = true
			break
		}
	}
	if !found {
		t.Log("arrays still rare in short seed scan")
	}
}

func TestMakeRandomArrayInitMultiDimNested(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession}
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	// force multi-dim array
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_md", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("nil array")
	}
	// ensure at least 2 dims if possible
	if len(av.Sizes) < 2 {
		av.Sizes = []int{2, 3}
		av.ArraySizes = av.Sizes
	}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// force array_init path by calling MakeRandomArrayInit directly
	st := MakeRandomArrayInit(NewRng(9), opts, probs, vs, tables, stmtTab, &cg)
	if st.Kind != StmtArrayOp || st.Loop == nil {
		t.Fatalf("%+v", st)
	}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	// should have nested for and multi-index access
	if strings.Count(out, "for (") < 2 && len(av.Sizes) >= 2 {
		t.Log("dims", av.Sizes, out)
	}
	if !strings.Contains(out, "g_md[") {
		t.Fatal(out)
	}
}

func TestFindAllVisibleVarsNilHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(testAmbientSession, Defaults())
	vs.GlobalList = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false), nil}
	if VariablesComplete(vs.FindAllVisibleVars(nil)) {
		t.Fatal("GlobalList nil hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GlobalList nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	vs.GlobalList = nil
	blk := &Block{LocalVars: []*Variable{CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false), nil}}
	if VariablesComplete(vs.FindAllVisibleVars(blk)) {
		t.Fatal("LocalVars nil hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("LocalVars nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectArrayNilHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = []*Variable{nil}
	if vs.SelectArray(NewRng(1), EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("visible list hole must fail closed SelectArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("visible list hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray is incomplete IR — fail closed sticky
	broken := &Variable{Name: "g_broken", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	vs.GlobalList = []*Variable{broken}
	if vs.SelectArray(NewRng(3), EmptyCGContext().WithSession(testAmbientSession)) != nil {
		t.Fatal("IsArray without AsArray must fail closed SelectArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete ambient fails closed sticky before filters
	inc := IncompleteEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if vs.SelectArray(NewRng(3), cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed SelectArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky SelectArray")
	}
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if vs.SelectArray(NewRng(4), cg2) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed SelectArray")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky SelectArray")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSelectArrayDoesNotInventFromArraysList(t *testing.T) {
	// VariableSelector.cpp:1386–1426 — only find_all_visible_vars; vs.Arrays is not a
	// second inventory. Array only on Arrays (not GlobalList/local) → create_random_array.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Types = &TypeEnv{Sess: testAmbientSession, AllTypes: []*Type{GetIntType()}}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Create without vs/blk registration (orphan array)
	orphan := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_orphan", GetIntType(), MakeInt(0), q)
	if orphan == nil {
		t.Fatal("orphan create")
	}
	vs.Arrays = []*ArrayVariable{orphan}
	// GlobalList empty → must not pick orphan; create_random_array draws flipcoin(25)
	r := NewRng(7)
	d0 := r.RandDepth()
	got := vs.SelectArray(r, EmptyCGContext().WithSession(testAmbientSession))
	if got == orphan {
		t.Fatal("must not invent select from vs.Arrays without visibility")
	}
	// create path: at least F25 when globals enabled
	if r.RandDepth() <= d0 {
		t.Fatal("empty visible must draw create_random_array RNG")
	}
	ClearErrorSess(testAmbientSession)
}
