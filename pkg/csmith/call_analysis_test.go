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
}

func TestFindContainedLabels(t *testing.T) {
	thenB := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 2, SourceLabel: "lbl_2"},
	}}
	st := &Stmt{Kind: StmtIfElse, StmID: 1, SourceLabel: "lbl_1", Then: thenB}
	labs := FindContainedLabels(st)
	if len(labs) < 2 {
		t.Fatal(labs)
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
	PostCreationAnalysis(st, pre, EmptyEffect(), &cg)
	if !fm.MapVisited[3] {
		t.Fatal("visited")
	}
	if _, ok := fm.MapFactsIn[3]; !ok {
		t.Fatal("facts in")
	}
}

func TestPostCreationUncertainFunc1(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// assign with RHS uncertain call (two call args)
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
	PostCreationAnalysis(st, nil, EmptyEffect(), &cg)
	if !fm.MapVisited[9] {
		t.Fatal("visited")
	}
}
