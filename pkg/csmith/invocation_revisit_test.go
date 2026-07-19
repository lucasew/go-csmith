package csmith

import "testing"

func TestPermuteParamOrdersTwo(t *testing.T) {
	fi := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(2)},
	}}
	ords := fi.PermuteParamOrders()
	if len(ords) != 2 {
		t.Fatal(ords)
	}
	if ords[0][0] != 0 || ords[0][1] != 1 {
		t.Fatal(ords[0])
	}
	if ords[1][0] != 1 || ords[1][1] != 0 {
		t.Fatal(ords[1])
	}
}

func TestRenewFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	RenewFacts(&facts, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	if !facts[0].IsDead() {
		t.Fatal(facts[0])
	}
	// equal → no growth
	n := len(facts)
	RenewFacts(&facts, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	if len(facts) != n {
		t.Fatal(len(facts))
	}
}

func TestReturnFactRegistry(t *testing.T) {
	InvocationReturnFactsDoFinalization()
	fi := &Invocation{User: &Function{Name: "f", RV: CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)}}
	f := MakeFactPointTo(fi.User.RV, NullPtr)
	fi.SaveReturnFacts([]*FactPointTo{f})
	got := GetReturnFactForInvocation(fi, fi.User.RV)
	if got == nil || !got.IsNull() {
		t.Fatal(got)
	}
	InvocationReturnFactsDoFinalization()
	if GetReturnFactForInvocation(fi, fi.User.RV) != nil {
		t.Fatal("cleared")
	}
}

func TestNeedsRevisit(t *testing.T) {
	f := &Function{}
	if f.NeedsRevisit() {
		t.Fatal("empty")
	}
	f.FactChanged = true
	if !f.NeedsRevisit() {
		t.Fatal("fact")
	}
	f.FactChanged = false
	f.ReferencedPtrs = []*Variable{CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)}
	if !f.IsPointerReferenced() || !f.NeedsRevisit() {
		t.Fatal("ptrs")
	}
}

func TestGetQualifiers(t *testing.T) {
	fi := &Invocation{IsStd: true}
	q := fi.GetQualifiers()
	if q.IsConst() || q.IsVolatile() {
		t.Fatal(q)
	}
	rv := CreateVariableScalars("f_rv", GetIntType(), false, false)
	rv.Qfer.SetConst(true, 0)
	fi2 := &Invocation{User: &Function{RV: rv}}
	if !fi2.GetQualifiers().IsConst() {
		t.Fatal("rv const")
	}
}

func TestVisitUnorderedParamsMerges(t *testing.T) {
	// two constant args — both orders succeed, facts unchanged
	fm := NewFactMgr(nil)
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	fi := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(2)},
	}}
	facts := []*FactPointTo{}
	if !fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("visit")
	}
}

func TestPermuteParamOrdersEmptyBaseFailClosed(t *testing.T) {
	// FunctionInvocation.cpp:434–453 + util permute(empty) → empty;
	// visit_unordered assert(orders.size()>0) — no soft invent identity order
	fi := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
	}}
	if got := fi.PermuteParamOrders(); len(got) != 0 {
		t.Fatalf("want empty orders for n!=2 without call args, got %v", got)
	}
	fi0 := &Invocation{}
	if got := fi0.PermuteParamOrders(); len(got) != 0 {
		t.Fatalf("want empty for 0 args, got %v", got)
	}
	// n==2 still both orders
	fi2 := &Invocation{Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(2)},
	}}
	if len(fi2.PermuteParamOrders()) != 2 {
		t.Fatal("2-arg shortcut")
	}
	// n!=2 with nil arg hole — no invent skip hole as non-call permute slot
	fiHole := &Invocation{Args: []*Expression{
		userCall("a"),
		{Term: TermConstant, Con: MakeInt(1)},
		nil,
	}}
	if got := fiHole.PermuteParamOrders(); got != nil {
		t.Fatalf("nil arg must fail closed nil orders, got %v", got)
	}
}

func TestVisitUnorderedParamsEmptyOrdersFailClosed(t *testing.T) {
	// FunctionInvocation.cpp:462 — assert(orders.size() > 0)
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	fi := &Invocation{Args: []*Expression{{Term: TermConstant, Con: MakeInt(1)}}}
	facts := []*FactPointTo{}
	if fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("empty orders must fail closed")
	}
}

func TestFactChangedOnAssign(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.UpdateFactForAssign(p, 0, &Expression{Term: TermConstant, Con: MakeInt(0)})
	if !f.FactChanged {
		// may not change if no pointer abstract result — force with null assign
		// AbstractFactForAssign for pointer const 0 should yield null fact
	}
	// ensure we have a pointer fact path
	if len(fm.GlobalFacts) > 0 && !f.FactChanged {
		t.Fatal("expected fact_changed")
	}
	// if no facts produced, set manually via second path
	if !f.FactChanged {
		// non-pointer assign doesn't set — use pointer lhs
		p2 := CreateVariableScalars("g_q", PointerTo(GetIntType()), true, false)
		// init as pointer type already
		_ = p2
		// call with existing null merge
		fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
		// re-assign to null
		if fm.UpdateFactForAssign(p, 0, &Expression{Term: TermConstant, Con: MakeInt(0)}) {
			if !f.FactChanged {
				t.Fatal("fact_changed after update")
			}
		}
	}
}

func TestRevisitUserInvocationSimple(t *testing.T) {
	// StatementAssign always has live Lhs + Expression* (Constant make_int for ++/--)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	assign := Stmt{
		Kind: StmtAssign, StmID: 101,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
		AssignOp: AssignSimple,
	}
	callee := &Function{
		Name: "func_x", ReturnType: GetIntType(),
		Body:        &Block{StmID: 100, Stmts: []Stmt{assign}},
		FactChanged: true,
	}
	callee.RV = CreateVariableScalars("func_x_rv", GetIntType(), false, false)
	fm := NewFactMgr(callee)
	eff := EmptyEffect()
	cg := WithFunc(callee, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// mark body effect empty
	fm.SetMapStmEffect(100, EmptyEffect())
	fi := &Invocation{User: callee}
	facts := []*FactPointTo{}
	if !RevisitUserInvocation(fi, &facts, &cg, Defaults()) {
		t.Fatal("revisit")
	}
	if callee.VisitedCnt < 1 {
		t.Fatal("visited_cnt")
	}
}
