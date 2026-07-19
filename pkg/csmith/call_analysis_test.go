package csmith

import "testing"

func userCall(name string, args ...*Expression) *Expression {
	return &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: name, ReturnType: GetIntType(), IsBuilt: true},
			Args: args,
		},
	}
}

func TestFuncCountAndCollectCalls(t *testing.T) {
	c1 := userCall("func_a")
	c2 := userCall("func_b")
	// binary-like: two user calls as args of outer std op — wait outer needs User for count of std?
	// FuncCount counts user invocations in tree
	outer := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: "func_c", ReturnType: GetIntType(), IsBuilt: true},
			Args: []*Expression{c1, c2},
		},
	}
	if FuncCount(outer) != 3 { // c1, c2, then outer
		t.Fatalf("func count %d", FuncCount(outer))
	}
	var calls []*Invocation
	CollectCalledInvocationsExpr(outer, &calls)
	if len(calls) != 3 {
		t.Fatal(len(calls))
	}
}

func TestFuncCountIncompleteFailClosed(t *testing.T) {
	// nil Invoke / nil arg hole — no invent empty call count
	if FuncCount(&Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil Invoke must FuncCount -1")
	}
	if FuncCount(&Expression{Term: TermFunction, Invoke: &Invocation{
		User: &Function{Name: "h"}, Args: []*Expression{userCall("a"), nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must FuncCount -1")
	}
	var calls []*Invocation
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsExpr(&Expression{Term: TermFunction}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("incomplete collect must fail closed incomplete, not invent empty complete", calls)
	}
}

func TestCollectCalledForTestExpr(t *testing.T) {
	// StatementFor::get_exprs → test; soft invent skip would miss for-test calls
	call := userCall("func_in_test")
	st := &Stmt{Kind: StmtFor, Loop: &LoopControl{TestExpr: call}, Then: &Block{}}
	var calls []*Invocation
	CollectCalledInvocationsStmt(st, &calls)
	if len(calls) != 1 || calls[0].User == nil || calls[0].User.Name != "func_in_test" {
		t.Fatalf("for-test calls: %+v", calls)
	}
	// incomplete for without TestExpr → incomplete marker
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmt(&Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("for without TestExpr must fail closed incomplete, not invent empty", calls)
	}
	if !HasUncertainCallRecursiveStmt(&Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}) {
		t.Fatal("incomplete for must fail closed uncertain")
	}
}

func TestCollectCalledAssignNilExprFailClosed(t *testing.T) {
	// C++ get_exprs always yields live Expression* for assign/invoke
	// soft invent skip nil Expr would invent empty call list as success
	var calls []*Invocation
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmt(&Stmt{Kind: StmtAssign, StmID: 1}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("assign without Expr must fail closed incomplete, not invent empty success", calls)
	}
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmt(&Stmt{Kind: StmtInvoke, StmID: 2}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("invoke without Expr must fail closed incomplete")
	}
	if !HasUncertainCallRecursiveStmt(&Stmt{Kind: StmtAssign, StmID: 3}) {
		t.Fatal("nil Expr assign must fail closed uncertain")
	}
	if !HasUncertainCallRecursiveExpr(nil) {
		t.Fatal("nil expr must fail closed uncertain")
	}
	if !(*Invocation)(nil).HasUncertainCall() {
		t.Fatal("nil invoke must fail closed HasUncertainCall")
	}
}

func TestHasUncertainCall(t *testing.T) {
	// two args each with a call → uncertain
	a := userCall("func_a")
	b := userCall("func_b")
	fi := &Invocation{
		User: &Function{Name: "func_c", ReturnType: GetIntType()},
		Args: []*Expression{a, b},
	}
	if !fi.HasUncertainCall() {
		t.Fatal("uncertain")
	}
	// one call arg only
	fi2 := &Invocation{
		User: &Function{Name: "func_c", ReturnType: GetIntType()},
		Args: []*Expression{a, &Expression{Term: TermConstant, Con: MakeInt(1)}},
	}
	if fi2.HasUncertainCall() {
		t.Fatal("not uncertain")
	}
	if !fi.HasUncertainCallRecursive() {
		t.Fatal("recursive")
	}
}

func TestHasSimpleParams(t *testing.T) {
	fi := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermVariable, Var: CreateVariableScalars("g_1", GetIntType(), true, false)},
	}}
	if !fi.HasSimpleParams() {
		t.Fatal("simple")
	}
	fi.Args[0] = userCall("f")
	if fi.HasSimpleParams() {
		t.Fatal("has call")
	}
	// nil arg hole — no invent simple
	fi.Args = []*Expression{{Term: TermConstant, Con: MakeInt(1)}, nil}
	if fi.HasSimpleParams() {
		t.Fatal("nil arg must fail closed not-simple")
	}
	// nil invoke shell — no invent simple-params success
	if (*Invocation)(nil).HasSimpleParams() {
		t.Fatal("nil invoke must fail closed not-simple")
	}
	// incomplete FuncCount → uncertain
	fiHole := &Invocation{Args: []*Expression{
		userCall("a"),
		{Term: TermFunction}, // nil Invoke
	}}
	if !fiHole.HasUncertainCall() {
		t.Fatal("incomplete arg must fail closed uncertain")
	}
}

func TestGetDirectInvocation(t *testing.T) {
	inv := &Invocation{User: &Function{Name: "f"}}
	st := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermFunction, Invoke: inv}}
	if GetDirectInvocation(st) != inv {
		t.Fatal("assign")
	}
	st2 := &Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: inv}}
	if GetDirectInvocation(st2) != inv {
		t.Fatal("invoke")
	}
	st3 := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	if GetDirectInvocation(st3) != nil {
		t.Fatal("const")
	}
	// incomplete Expr/Invoke fails closed Failed (no invent nil as no-call)
	got := GetDirectInvocation(&Stmt{Kind: StmtInvoke})
	if got == nil || !got.Failed {
		t.Fatal("nil Expr invoke must fail closed Failed shell", got)
	}
	got = GetDirectInvocation(&Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermFunction}})
	if got == nil || !got.Failed {
		t.Fatal("nil Invoke on TermFunction must fail closed Failed")
	}
}

func TestFindContainedLabels(t *testing.T) {
	thenB := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 2, SourceLabel: "lbl_2"},
	}}
	// StatementIf always has both arms; nil Else is incomplete IR
	st := &Stmt{Kind: StmtIfElse, StmID: 1, SourceLabel: "lbl_1", Then: thenB, Else: &Block{}}
	labs := FindContainedLabels(st)
	if len(labs) < 2 {
		t.Fatal(labs)
	}
	// incomplete if — fail closed incomplete (no invent empty/partial labels)
	if LabelsComplete(FindContainedLabels(&Stmt{Kind: StmtIfElse, StmID: 9, SourceLabel: "x", Then: thenB})) {
		t.Fatal("nil Else must fail closed incomplete")
	}
	// FM + StmID 0 — no invent complete child labels while soft-skipping self id
	fm := NewFactMgr(nil)
	if LabelsComplete(FindContainedLabelsFM(&Stmt{Kind: StmtAssign, StmID: 0, SourceLabel: "x"}, fm)) {
		t.Fatal("StmID 0 under FM must fail closed incomplete")
	}
}

func TestCombineBranchFacts(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	pre := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	thenB := &Block{StmID: 10}
	elseB := &Block{StmID: 11}
	fm.SetMapFactsOut(10, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	fm.SetMapFactsOut(11, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	// both return → pre
	st := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtReturn, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtReturn, StmID: 21}}},
	}
	// MustReturn needs return as last
	_ = thenB
	_ = elseB
	CombineBranchFacts(st, pre, fm)
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		// both must return → pre facts
	}
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || !fp.IsNull() {
		t.Fatal("both return use pre", fp)
	}
	// nil hole in branch outs fails closed sticky — no invent partial combine
	// bypass SetMapFactsOut (CloneFactSlice strips holes → nil) to plant a hole
	ClearError()
	fm2 := NewFactMgr(nil)
	fm2.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointTo(p, GarbagePtr), nil},
		11: {MakeFactPointTo(p, NullPtr)},
	}
	st2 := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtAssign, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	CombineBranchFacts(st2, pre, fm2)
	if FactsComplete(fm2.GlobalFacts) {
		t.Fatal("nil branch fact hole must fail closed", fm2.GlobalFacts)
	}
	if !HasError() {
		t.Fatal("nil branch fact hole must SetError sticky")
	}
	ClearError()
	// missing Then/Else arm — no invent empty branch via FactsComplete(nil)
	fm3 := NewFactMgr(nil)
	fm3.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	st3 := &Stmt{Kind: StmtIfElse, Then: &Block{StmID: 10}, Else: nil}
	fm3.SetMapFactsOut(10, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	CombineBranchFacts(st3, pre, fm3)
	if FactsComplete(fm3.GlobalFacts) {
		t.Fatal("nil Else arm must fail closed", fm3.GlobalFacts)
	}
	if !HasError() {
		t.Fatal("nil Else arm must SetError sticky")
	}
	ClearError()
	// arm Block StmID 0 — no invent empty outs via FactsComplete(nil)
	fm4 := NewFactMgr(nil)
	fm4.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	st4 := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 0, Stmts: []Stmt{{Kind: StmtAssign, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	fm4.SetMapFactsOut(11, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	CombineBranchFacts(st4, pre, fm4)
	if FactsComplete(fm4.GlobalFacts) {
		t.Fatal("Then StmID 0 must fail closed", fm4.GlobalFacts)
	}
	if !HasError() {
		t.Fatal("Then StmID 0 must SetError sticky")
	}
	ClearError()
}

func TestPostCreationAssignFacts(t *testing.T) {
	f := &Function{Name: "func_2", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtAssign, StmID: 3,
		LhsVar: p, Lhs: &Lhs{Var: p, Type: PointerTo(GetIntType())},
		Expr:     &Expression{Term: TermVariable, Var: tgt}, // need address? assign ptr = &tgt via expr
		AssignOp: AssignSimple,
	}
	// pointer assign from variable of type int won't abstract well; use Null
	st.Expr = &Expression{Term: TermConstant, Con: MakeInt(0)}
	pre := CloneFactSlice(fm.GlobalFacts)
	PostCreationAnalysis(st, pre, EmptyEffect(), &cg, Defaults())
	if !fm.MapVisited[3] {
		t.Fatal("visited")
	}
	if _, ok := fm.MapFactsIn[3]; !ok {
		t.Fatal("facts in")
	}
}

func TestPostCreationUncertainFunc1(t *testing.T) {
	// Statement.cpp:868–871 — assert(validate) when func_1 uncertain revalidate fails
	ClearError()
	defer ClearError()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// assign with RHS uncertain call (two call args) — incomplete user IR fails visit
	a := userCall("func_a")
	b := userCall("func_b")
	rhs := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: "func_c", ReturnType: GetIntType(), IsBuilt: true},
			Args: []*Expression{a, b},
		},
	}
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 9,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: rhs, AssignOp: AssignSimple,
	}
	if !HasUncertainCallRecursiveStmt(st) {
		t.Fatal("expect uncertain")
	}
	PostCreationAnalysis(st, nil, EmptyEffect(), &cg, Defaults())
	// incomplete call tree fails validate → sticky error (no soft invent continue)
	if !HasError() {
		t.Fatal("validate fail must set sticky error like assert(0)")
	}
}

func TestFindContainedLabelsFM(t *testing.T) {
	f := &Function{Name: "f"}
	thenB := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 2},
		{Kind: StmtGoto, StmID: 3, Label: "lbl_cfg", GotoDestStmID: 2},
	}}
	// StatementIf always has both arms
	st := &Stmt{Kind: StmtIfElse, StmID: 1, Then: thenB, Else: &Block{}}
	f.Blocks = []*Block{thenB}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 3, DestStmID: 2}}
	labs := FindContainedLabelsFM(st, fm)
	found := false
	for _, l := range labs {
		if l == "lbl_cfg" {
			found = true
		}
	}
	if !found {
		t.Fatal(labs)
	}
	// with FM: SourceLabel must not invent label when CFG has no jump to this stm
	stSrc := &Stmt{Kind: StmtAssign, StmID: 99, SourceLabel: "lbl_invent"}
	if labs2 := FindContainedLabelsFM(stSrc, fm); len(labs2) != 0 {
		t.Fatal("SourceLabel must not invent under live FM", labs2)
	}
	// incomplete CFGEdges hole fails whole collect
	fm.CFGEdges = []*CFGEdge{{SrcID: 3, DestStmID: 2}, nil}
	if LabelsComplete(FindContainedLabelsFM(st, fm)) {
		t.Fatal("nil CFG edge hole must fail closed incomplete labels")
	}
}
