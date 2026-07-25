package csmith

import "testing"

func userCall(name string, args ...*Expression) *Expression {
	return &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: name, ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true},
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
			User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true},
			Args: []*Expression{c1, c2},
		},
	}
	if FuncCountSess(testAmbientSession, outer) != 3 { // c1, c2, then outer
		t.Fatalf("func count %d", FuncCountSess(testAmbientSession, outer))
	}
	var calls []*Invocation
	CollectCalledInvocationsExprSess(testAmbientSession, outer, &calls)
	if len(calls) != 3 {
		t.Fatal(len(calls))
	}
}

func TestFuncCountIncompleteFailClosed(t *testing.T) {
	// nil Invoke / nil arg hole — no invent empty call count
	if FuncCountSess(testAmbientSession, &Expression{Term: TermFunction}) >= 0 {
		t.Fatal("nil Invoke must FuncCount -1")
	}
	if FuncCountSess(testAmbientSession, &Expression{Term: TermFunction, Invoke: &Invocation{
		User: &Function{Name: "h"}, Args: []*Expression{userCall("a"), nil},
	}}) >= 0 {
		t.Fatal("nil arg hole must FuncCount -1")
	}
	ClearErrorSess(testAmbientSession)
	var calls []*Invocation
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsExprSess(testAmbientSession, &Expression{Term: TermFunction}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("incomplete collect must fail closed incomplete, not invent empty complete", calls)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete collect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// out always live; sticky (no invent soft-skip collect past hole)
	CollectCalledInvocationsExprSess(testAmbientSession, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil out CollectCalledInvocationsExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CollectCalledInvocationsStmtSess(testAmbientSession, &Stmt{Kind: StmtBreak, StmID: 1}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil out CollectCalledInvocationsStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CollectCalledInvocationsBlockSess(testAmbientSession, &Block{}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil out CollectCalledInvocationsBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCollectCalledForTestExpr(t *testing.T) {
	// StatementFor::get_exprs → test; soft invent skip would miss for-test calls
	ClearErrorSess(testAmbientSession)
	call := userCall("func_in_test")
	st := &Stmt{Kind: StmtFor, Loop: &LoopControl{TestExpr: call}, Then: &Block{}}
	var calls []*Invocation
	CollectCalledInvocationsStmtSess(testAmbientSession, st, &calls)
	if len(calls) != 1 || calls[0].User == nil || calls[0].User.Name != "func_in_test" {
		t.Fatalf("for-test calls: %+v", calls)
	}
	// incomplete for without TestExpr → incomplete marker sticky
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmtSess(testAmbientSession, &Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("for without TestExpr must fail closed incomplete, not invent empty", calls)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("for without TestExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Statement.h:185 — StatementFor does not override has_uncertain_call_recursive
	// (base false). Soft invent walked for-test/body as uncertain (unfair special path).
	if HasUncertainCallRecursiveStmtSess(testAmbientSession, &Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}) {
		t.Fatal("StatementFor must be certain (base false), not invent body walk")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("StatementFor HasUncertainCallRecursive must not SetError")
	}
	ClearErrorSess(testAmbientSession)
	// StatementArrayOp.h:65–68 — get_exprs is if(init_value) only, not For test.
	// array_init numeric LoopControl must not fail closed incomplete (seed-2 fair).
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	body := &Block{Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 2,
		LhsVar:      CreateVariableScalarsSess(testAmbientSession, "a", GetIntTypeSess(testAmbientSession), false, false),
		Expr:        &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
		ArrayAccess: "a[i]",
	}}}
	var callsArr []*Invocation
	CollectCalledInvocationsStmtSess(testAmbientSession, &Stmt{
		Kind: StmtArrayOp, StmID: 1,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 4, IncrN: 1},
		Then: body,
	}, &callsArr)
	if !InvocationsComplete(callsArr) {
		t.Fatal("array-init ArrayOp must collect complete (no invent For-test hole)", callsArr)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("array-init ArrayOp must not SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if HasUncertainCallRecursiveStmtSess(testAmbientSession, &Stmt{
		Kind: StmtArrayOp, StmID: 1,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 4, IncrN: 1},
		Then: body,
	}) {
		t.Fatal("array-init without calls must be certain (false)")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("array-init HasUncertainCallRecursive must not SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCollectCalledAssignNilExprFailClosed(t *testing.T) {
	// C++ get_exprs always yields live Expression* for assign/invoke
	// soft invent skip nil Expr would invent empty call list as success sticky
	ClearErrorSess(testAmbientSession)
	var calls []*Invocation
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmtSess(testAmbientSession, &Stmt{Kind: StmtAssign, StmID: 1}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("assign without Expr must fail closed incomplete, not invent empty success", calls)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("assign without Expr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	calls = []*Invocation{{User: &Function{Name: "stale"}}}
	CollectCalledInvocationsStmtSess(testAmbientSession, &Stmt{Kind: StmtInvoke, StmID: 2}, &calls)
	if InvocationsComplete(calls) {
		t.Fatal("invoke without Expr must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invoke without Expr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !HasUncertainCallRecursiveStmtSess(testAmbientSession, &Stmt{Kind: StmtAssign, StmID: 3}) {
		t.Fatal("nil Expr assign must fail closed uncertain")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr assign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !HasUncertainCallRecursiveExprSess(testAmbientSession, nil) {
		t.Fatal("nil expr must fail closed uncertain")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil expr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(*Invocation)(nil).HasUncertainCallSess(testAmbientSession) {
		t.Fatal("nil invoke must fail closed HasUncertainCall")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil invoke HasUncertainCall must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasUncertainCall(t *testing.T) {
	// two args each with a call → uncertain
	a := userCall("func_a")
	b := userCall("func_b")
	fi := &Invocation{
		User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession)},
		Args: []*Expression{a, b},
	}
	if !fi.HasUncertainCallSess(testAmbientSession) {
		t.Fatal("uncertain")
	}
	// one call arg only
	fi2 := &Invocation{
		User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession)},
		Args: []*Expression{a, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}},
	}
	if fi2.HasUncertainCallSess(testAmbientSession) {
		t.Fatal("not uncertain")
	}
	// nested HasUncertainCallRecursive residual: soft invent was soft-continue later args certain.
	// Fair: sticky uncertain true.
	ClearErrorSess(testAmbientSession)
	nestedHole := &Expression{Term: TermFunction, Invoke: nil}
	fiHole := &Invocation{
		User: &Function{Name: "func_h", ReturnType: GetIntTypeSess(testAmbientSession)},
		Args: []*Expression{nestedHole, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}},
	}
	if !fiHole.HasUncertainCallRecursiveSess(testAmbientSession) {
		t.Fatal("nested Invoke-nil residual must fail closed uncertain")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested Invoke-nil residual HasUncertainCallRecursive must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// comma LHS residual soft invent was soft-continue RHS invent certain.
	comma := &Expression{Term: TermCommaExpr, CommaLHS: nestedHole, CommaRHS: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}}
	if !HasUncertainCallRecursiveExprSess(testAmbientSession, comma) {
		t.Fatal("comma nested residual must fail closed uncertain")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("comma nested residual HasUncertainCallRecursiveExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !fi.HasUncertainCallRecursiveSess(testAmbientSession) {
		t.Fatal("recursive")
	}
}

func TestHasSimpleParams(t *testing.T) {
	fi := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)},
	}}
	if !fi.HasSimpleParamsSess(testAmbientSession) {
		t.Fatal("simple")
	}
	fi.Args[0] = userCall("f")
	if fi.HasSimpleParamsSess(testAmbientSession) {
		t.Fatal("has call")
	}
	// nil arg hole — sticky not-simple
	ClearErrorSess(testAmbientSession)
	fi.Args = []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, nil}
	if fi.HasSimpleParamsSess(testAmbientSession) {
		t.Fatal("nil arg must fail closed not-simple")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg HasSimpleParams must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil invoke shell — sticky not-simple
	if (*Invocation)(nil).HasSimpleParamsSess(testAmbientSession) {
		t.Fatal("nil invoke must fail closed not-simple")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil invoke HasSimpleParams must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete FuncCount → uncertain sticky
	fiHole := &Invocation{Args: []*Expression{
		userCall("a"),
		{Term: TermFunction}, // nil Invoke
	}}
	if !fiHole.HasUncertainCallSess(testAmbientSession) {
		t.Fatal("incomplete arg must fail closed uncertain")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete arg HasUncertainCall must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetDirectInvocation(t *testing.T) {
	inv := &Invocation{User: &Function{Name: "f"}}
	st := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermFunction, Invoke: inv}}
	if GetDirectInvocationSess(testAmbientSession, st) != inv {
		t.Fatal("assign")
	}
	st2 := &Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: inv}}
	if GetDirectInvocationSess(testAmbientSession, st2) != inv {
		t.Fatal("invoke")
	}
	st3 := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}}
	if GetDirectInvocationSess(testAmbientSession, st3) != nil {
		t.Fatal("const")
	}
	// incomplete Expr/Invoke fails closed Failed sticky (no invent nil as no-call soft-skip)
	ClearErrorSess(testAmbientSession)
	got := GetDirectInvocationSess(testAmbientSession, &Stmt{Kind: StmtInvoke})
	if got == nil || !got.Failed {
		t.Fatal("nil Expr invoke must fail closed Failed shell", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr invoke must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	got = GetDirectInvocationSess(testAmbientSession, &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermFunction}})
	if got == nil || !got.Failed {
		t.Fatal("nil Invoke on TermFunction must fail closed Failed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	got = GetDirectInvocationSess(testAmbientSession, &Stmt{Kind: StmtAssign, Expr: nil})
	if got == nil || !got.Failed {
		t.Fatal("nil Expr assign must fail closed Failed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr assign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindContainedLabels(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	thenB := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 2, SourceLabel: "lbl_2"},
	}}
	// StatementIf always has both arms; nil Else is incomplete IR
	st := &Stmt{Kind: StmtIfElse, StmID: 1, SourceLabel: "lbl_1", Then: thenB, Else: &Block{}}
	labs := FindContainedLabels(st)
	if len(labs) < 2 {
		t.Fatal(labs)
	}
	// incomplete if — fail closed sticky incomplete (no invent empty/partial labels)
	if LabelsComplete(FindContainedLabels(&Stmt{Kind: StmtIfElse, StmID: 9, SourceLabel: "x", Then: thenB})) {
		t.Fatal("nil Else must fail closed incomplete")
	}
	// nil Else FindContainedLabels must SetError sticky — sticky on owner bag / throwaway, not package ambient dual-fill
	ClearErrorSess(testAmbientSession)
	// FM + StmID 0 — no invent complete child labels while soft-skipping self id
	fm := NewFactMgrSess(testAmbientSession, nil)
	if LabelsComplete(FindContainedLabelsFM(&Stmt{Kind: StmtAssign, StmID: IncompleteStmID, SourceLabel: "x"}, fm)) {
		t.Fatal("StmID 0 under FM must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 FindContainedLabelsFM must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCombineBranchFacts(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	pre := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	preU := []*FactUnion{}
	thenB := &Block{StmID: 10}
	elseB := &Block{StmID: 11}
	fm.SetMapFactsOut(10, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	fm.SetMapFactsOut(11, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	// both return → pre
	st := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtReturn, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtReturn, StmID: 21}}},
	}
	// MustReturn needs return as last
	_ = thenB
	_ = elseB
	// Statement + FactMgr always live; sticky no invent soft-skip combine past hole
	ClearErrorSess(testAmbientSession)
	CombineBranchFacts(nil, &pre, &preU, fm)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stmt CombineBranchFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CombineBranchFacts(st, &pre, &preU, nil)
	// nil FM CombineBranchFacts must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	CombineBranchFacts(st, &pre, &preU, fm)
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) == nil {
		// both must return → pre facts
	}
	fp := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p)
	if fp == nil || !fp.IsNullSess(testAmbientSession) {
		t.Fatal("both return use pre", fp)
	}
	// nil hole in branch outs fails closed sticky — no invent partial combine
	// bypass SetMapFactsOut (CloneFactSlice strips holes → nil) to plant a hole
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil},
		11: {MakeFactPointToSess(testAmbientSession, p, NullPtr)},
	}
	st2 := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtAssign, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	pre2 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	preU2 := []*FactUnion{}
	CombineBranchFacts(st2, &pre2, &preU2, fm2)
	if FactsComplete(fm2.GlobalFacts) {
		t.Fatal("nil branch fact hole must fail closed", fm2.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil branch fact hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// missing Then/Else arm — no invent empty branch via FactsComplete(nil)
	fm3 := NewFactMgrSess(testAmbientSession, nil)
	fm3.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	st3 := &Stmt{Kind: StmtIfElse, Then: &Block{StmID: 10}, Else: nil}
	fm3.SetMapFactsOut(10, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	pre3 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	preU3 := []*FactUnion{}
	CombineBranchFacts(st3, &pre3, &preU3, fm3)
	if FactsComplete(fm3.GlobalFacts) {
		t.Fatal("nil Else arm must fail closed", fm3.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Else arm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// arm Block StmID 0 — no invent empty outs via FactsComplete(nil)
	fm4 := NewFactMgrSess(testAmbientSession, nil)
	fm4.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	st4 := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: IncompleteStmID, Stmts: []Stmt{{Kind: StmtAssign, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	fm4.SetMapFactsOut(11, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	pre4 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	preU4 := []*FactUnion{}
	CombineBranchFacts(st4, &pre4, &preU4, fm4)
	if FactsComplete(fm4.GlobalFacts) {
		t.Fatal("Then StmID 0 must fail closed", fm4.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Then StmID 0 must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// MustReturn residual soft invent was soft-continue branch-merge invent complete GlobalFacts.
	// Then last is If with nil arm → MustReturn residual sticky false.
	fm5 := NewFactMgrSess(testAmbientSession, nil)
	fm5.SetMapFactsOut(10, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	fm5.SetMapFactsOut(11, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	fm5.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	st5 := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{
			StmID: 10,
			Stmts: []Stmt{{Kind: StmtIfElse, Then: nil, Else: &Block{StmID: 30}}},
		},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	pre5 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	preU5 := []*FactUnion{}
	CombineBranchFacts(st5, &pre5, &preU5, fm5)
	if FactsComplete(fm5.GlobalFacts) {
		t.Fatal("MustReturn residual must fail closed incomplete GlobalFacts", fm5.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MustReturn residual CombineBranchFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCombineBranchFactsMergesUnionWrite(t *testing.T) {
	// StatementIf.cpp:228–230 — outputs = then_out; merge_facts(outputs, else_out)
	// Full FactVec includes eUnionWrite. Soft invent left UnionFacts at else-exit only.
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_u", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 2 {
		t.Fatal("need fields")
	}
	f0, f1 := parent.FieldVars[0], parent.FieldVars[1]
	thenU := MakeFactUnionSess(testAmbientSession, parent, 0) // then wrote f0
	elseU := MakeFactUnionSess(testAmbientSession, parent, 1) // else wrote f1
	if thenU == nil || elseU == nil {
		t.Fatal("MakeFactUnion")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	// empty PT maps; union partitions differ by arm
	fm.SetMapFactsOutPair(10, []*FactPointTo{}, []*FactUnion{thenU})
	fm.SetMapFactsOutPair(11, []*FactPointTo{}, []*FactUnion{elseU})
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{elseU} // live = else exit (old bug state)
	st := &Stmt{
		Kind: StmtIfElse,
		Then: &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtAssign, StmID: 20}}},
		Else: &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign, StmID: 21}}},
	}
	pre := []*FactPointTo{}
	preU := []*FactUnion{MakeFactUnionSess(testAmbientSession, parent, FactUnionTop)}
	CombineBranchFacts(st, &pre, &preU, fm)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	// join f0 + f1 → BOTTOM; both fields nonreadable
	got := FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, parent)
	if got == nil {
		t.Fatal("missing union fact after merge")
	}
	if !got.IsBottomSess(testAmbientSession) {
		t.Fatalf("want BOTTOM after f0|f1 branch merge, fid=%d", got.LastWrittenFID)
	}
	if !IsNonreadableFieldSess(testAmbientSession, f0, fm.UnionFacts) || !IsNonreadableFieldSess(testAmbientSession, f1, fm.UnionFacts) {
		t.Fatal("BOTTOM merge: both fields nonreadable")
	}
	ClearErrorSess(testAmbientSession)
}

// DropUnionSubjectsByVars is the sibling-arm strip used by VisitFactsStatementIf
// and CombineBranchFacts (seed-58 else-local must not re-enter if-output).
func TestDropUnionSubjectsByVarsSiblingArmLocal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	ut := &Type{isUnion: true, StructName: "U1", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	elseLocal := CreateVariableScalarsSess(testAmbientSession, "l_else_u", ut, false, false)
	g := CreateVariableScalarsSess(testAmbientSession, "g_u2", ut, false, false)
	if elseLocal == nil || g == nil {
		t.Fatal("vars")
	}
	thenU := MakeFactUnionSess(testAmbientSession, g, 0)
	foreign := MakeFactUnionSess(testAmbientSession, elseLocal, 0)
	if thenU == nil || foreign == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("MakeFactUnion", GetErrorSess(testAmbientSession))
	}
	stripped := DropUnionSubjectsByVarsSess(testAmbientSession, []*FactUnion{thenU, foreign}, []*Variable{elseLocal})
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if FindRelatedUnionSess(testAmbientSession, stripped, elseLocal) != nil {
		t.Fatal("must drop else-local from then-out")
	}
	if FindRelatedUnionSess(testAmbientSession, stripped, g) == nil {
		t.Fatal("must keep global subject")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostCreationAssignFacts(t *testing.T) {
	f := &Function{Name: "func_2", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtAssign, StmID: 3,
		LhsVar: p, Lhs: &Lhs{Var: p, Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		Expr:     &Expression{Term: TermVariable, Var: tgt}, // need address? assign ptr = &tgt via expr
		AssignOp: AssignSimple,
	}
	// pointer assign from variable of type int won't abstract well; use Null
	st.Expr = &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}
	pre := CloneFactSliceSess(testAmbientSession, fm.GlobalFacts)
	PostCreationAnalysis(st, pre, nil, EmptyEffect(), &cg, Defaults())
	if !fm.MapVisited[3] {
		t.Fatal("visited")
	}
	if _, ok := fm.MapFactsIn[3]; !ok {
		t.Fatal("facts in")
	}
}

func TestPostCreationUncertainFunc1(t *testing.T) {
	// Statement.cpp:868–871 — assert(0) when func_1 uncertain revalidate fails;
	// NDEBUG elides assert and still installs outputs + special_handled.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// assign with RHS uncertain call (two call args) — incomplete user IR fails visit
	a := userCall("func_a")
	b := userCall("func_b")
	rhs := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true},
			Args: []*Expression{a, b},
		},
	}
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 9,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: rhs, AssignOp: AssignSimple,
	}
	if !HasUncertainCallRecursiveStmtSess(testAmbientSession, st) {
		t.Fatal("expect uncertain")
	}
	PostCreationAnalysis(st, nil, nil, EmptyEffect(), &cg, Defaults())
	// NDEBUG Release: assert(0) elided — no sticky abort; facts still installed.
	if HasErrorSess(testAmbientSession) {
		t.Fatal("NDEBUG assert(0) path must not sticky-poison generation", GetErrorSess(testAmbientSession))
	}
}

// TestPostCreationUncertainFunc1KeepsGenStmEffect — Statement.cpp:854–875.
// Gen-time map_stm_effect (line 857) must survive special validate for func_1
// uncertain calls. Soft invent let VisitFactsAssign overwrite map_stm_effect
// with re-analysis lattice that under-collected first-build callee reads
// (seed-12 func_1 missing g_208/g_1489/g_1939).
func TestPostCreationUncertainFunc1KeepsGenStmEffect(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	gExtra := CreateVariableScalarsSess(testAmbientSession, "g_extra", GetIntTypeSess(testAmbientSession), true, false)
	gKeep := CreateVariableScalarsSess(testAmbientSession, "g_keep", GetIntTypeSess(testAmbientSession), true, false)
	// Gen-time effect_stm already includes a global read from a nested call.
	genEff := EmptyEffect().ReadVarSess(testAmbientSession, gKeep).ReadVarSess(testAmbientSession, gExtra)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.EffectStm = genEff
	// Uncertain call RHS so special path runs validate (may fail/clear EffectStm).
	a := userCall("func_a")
	b := userCall("func_b")
	rhs := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true, Body: &Block{StmID: AllocStmIDSess(testAmbientSession)}},
			Args: []*Expression{a, b},
		},
	}
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: AllocStmIDSess(testAmbientSession),
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: rhs, AssignOp: AssignSimple,
	}
	if !HasUncertainCallRecursiveStmtSess(testAmbientSession, st) {
		t.Fatal("expect uncertain for special path")
	}
	// Pre-install gen map as PostCreation would after saving EffectStm, then run
	// full post_creation (saves EffectStm then special validate).
	pre := []*FactPointTo{MakeFactPointToSess(testAmbientSession, v, NullPtr)}
	PostCreationAnalysis(st, pre, []*FactUnion{}, EmptyEffect(), &cg, opts)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("post_creation sticky", GetErrorSess(testAmbientSession))
	}
	got := fm.GetMapStmEffect(st.StmID)
	if !got.IsReadSess(testAmbientSession, gExtra) || !got.IsReadSess(testAmbientSession, gKeep) {
		t.Fatalf("gen-time reads must survive special validate: got reads=%v", got.ReadVarsSess(testAmbientSession))
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
	fm := NewFactMgrSess(testAmbientSession, f)
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
	// incomplete CFGEdges hole fails whole collect sticky
	ClearErrorSess(testAmbientSession)
	fm.CFGEdges = []*CFGEdge{{SrcID: 3, DestStmID: 2}, nil}
	if LabelsComplete(FindContainedLabelsFM(st, fm)) {
		t.Fatal("nil CFG edge hole must fail closed incomplete labels")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CFG edge hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetDirectInvocationNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if GetDirectInvocationSess(testAmbientSession, nil) != nil {
		t.Fatal("nil Stmt GetDirectInvocation must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stmt GetDirectInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// Statement.h:185 — StatementIf does not override has_uncertain_call_recursive.
// Soft invent checked condition expr and took Statement.cpp:969 special path for
// if (seed-250: empty pre_facts wipe of g_67 may-null). C++ never specials if.
func TestHasUncertainCallRecursiveIfElseBaseFalse(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Condition with multi-arg call would look "uncertain" if we walked Expr.
	a := userCall("func_a")
	b := userCall("func_b")
	cond := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{
			User: &Function{Name: "func_c", ReturnType: GetIntTypeSess(testAmbientSession), IsBuilt: true},
			Args: []*Expression{a, b},
		},
	}
	st := &Stmt{Kind: StmtIfElse, StmID: 1, Expr: cond, Then: &Block{}, Else: &Block{}}
	if HasUncertainCallRecursiveStmtSess(testAmbientSession, st) {
		t.Fatal("StatementIf must return false (base), not invent condition walk")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("StatementIf must not SetError")
	}
	ClearErrorSess(testAmbientSession)
}
