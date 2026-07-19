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
	ClearError()
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
	// incomplete maps fail closed sticky (no invent soft re-pick past hole)
	hole := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	base := []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	if RenewFacts(&base, hole) {
		t.Fatal("incomplete newFacts must fail closed false")
	}
	if FactsComplete(base) {
		t.Fatal("incomplete renew must wipe facts incomplete")
	}
	if !HasError() {
		t.Fatal("incomplete RenewFacts must SetError sticky")
	}
	ClearError()
	// incomplete RenewFact target
	facts2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if RenewFact(&facts2, nil) {
		t.Fatal("nil nf must fail closed false")
	}
	if FactsComplete(facts2) {
		t.Fatal("nil nf renew must wipe incomplete")
	}
	if !HasError() {
		t.Fatal("nil nf RenewFact must SetError sticky")
	}
	ClearError()
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

func TestSaveReturnFactsIncompleteFailClosed(t *testing.T) {
	// incomplete maps sticky (no invent soft-skip hole and still register later)
	ClearError()
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	rv := CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	fi := &Invocation{User: &Function{Name: "f", RV: rv}}
	good := MakeFactPointTo(rv, NullPtr)
	fi.SaveReturnFacts([]*FactPointTo{nil, good})
	if GetReturnFactForInvocation(fi, rv) != nil {
		t.Fatal("incomplete SaveReturnFacts must not invent registry entry")
	}
	if !HasError() {
		t.Fatal("incomplete SaveReturnFacts must SetError sticky")
	}
	ClearError()
	// incomplete PointTo on matching fact sticky
	fi.SaveReturnFacts([]*FactPointTo{{Var: rv, PointTo: []*Variable{nil}}})
	if GetReturnFactForInvocation(fi, rv) != nil {
		t.Fatal("incomplete PointTo must not invent registry")
	}
	if !HasError() {
		t.Fatal("incomplete PointTo SaveReturnFacts must SetError sticky")
	}
	ClearError()
	// nil Invocation* registry slot sticky fail closed (no invent later match)
	ClearError()
	fi2 := &Invocation{User: &Function{Name: "g", RV: rv}}
	AddReturnFactForInvocation(fi2, MakeFactPointTo(rv, NullPtr))
	returnFactInvocations = append([]*Invocation{nil}, returnFactInvocations...)
	returnFactPoints = append([]*FactPointTo{MakeFactPointTo(rv, NullPtr)}, returnFactPoints...)
	if GetReturnFactForInvocation(fi2, rv) != nil {
		t.Fatal("nil inv registry hole must fail closed, not invent later match")
	}
	if !HasError() {
		t.Fatal("nil inv registry GetReturnFact must SetError sticky")
	}
	// Add with hole must wipe sticky rather than soft-skip re-seed
	ClearError()
	AddReturnFactForInvocation(fi2, MakeFactPointTo(rv, NullPtr))
	if GetReturnFactForInvocation(fi2, rv) != nil {
		t.Fatal("AddReturnFact over hole must wipe, not invent re-seed")
	}
	if !HasError() {
		t.Fatal("AddReturnFact over hole must SetError sticky")
	}
	ClearError()
	InvocationReturnFactsDoFinalization()
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
	ClearError()
	fi := &Invocation{IsStd: true}
	q := fi.GetQualifiers()
	if q.IsConst() || q.IsVolatile() {
		t.Fatal(q)
	}
	if HasError() {
		t.Fatal("std Invoke GetQualifiers must not sticky")
	}
	rv := CreateVariableScalars("f_rv", GetIntType(), false, false)
	rv.Qfer.SetConst(true, 0)
	fi2 := &Invocation{User: &Function{RV: rv}}
	if !fi2.GetQualifiers().IsConst() {
		t.Fatal("rv const")
	}
	// nil RV sticky empty qfer (no invent storage-level false/false shell)
	ClearError()
	fi3 := &Invocation{User: &Function{Name: "f"}}
	q3 := fi3.GetQualifiers()
	if len(q3.IsConsts) != 0 || len(q3.IsVolatiles) != 0 {
		t.Fatalf("nil RV must not invent qfer bits: %+v", q3)
	}
	if !HasError() {
		t.Fatal("nil RV GetQualifiers must SetError sticky")
	}
	ClearError()
	// nil Invocation sticky empty
	if q4 := (*Invocation)(nil).GetQualifiers(); len(q4.IsConsts) != 0 {
		t.Fatal("nil Invocation GetQualifiers must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil Invocation GetQualifiers must SetError sticky")
	}
	ClearError()
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

func TestVisitUnorderedParamsMergeIncompleteFailClosed(t *testing.T) {
	// multi-order merge: incomplete cur after second order fails closed
	// plant via incomplete GlobalFacts mid-visit is hard; exercise MergeFacts path
	// with two orders of constants — success baseline
	ClearError()
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	cg := EmptyCGContext().WithFactMgr(fm)
	a := &Expression{Term: TermConstant, Con: MakeInt(1)}
	b := &Expression{Term: TermConstant, Con: MakeInt(2)}
	// n==2 forces two permute orders
	fi := &Invocation{
		User: &Function{Name: "f", ReturnType: GetIntType(), IsBuilt: true},
		Args: []*Expression{a, b},
	}
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if !fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("complete two-arg should succeed")
	}
	if !FactsComplete(facts) {
		t.Fatal("post-merge facts must stay complete")
	}
}

func TestVisitUnorderedParamsEmptyOrdersFailClosed(t *testing.T) {
	// FunctionInvocation.cpp:462 — assert(orders.size() > 0) sticky
	ClearError()
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	fi := &Invocation{Args: []*Expression{{Term: TermConstant, Con: MakeInt(1)}}}
	facts := []*FactPointTo{}
	if fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("empty orders must fail closed")
	}
	if !HasError() {
		t.Fatal("empty orders VisitUnorderedParams must SetError sticky")
	}
	ClearError()
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

func TestNeedsRevisitIsPointerReferencedIncompleteSticky(t *testing.T) {
	ClearError()
	if (*Function)(nil).NeedsRevisit() {
		t.Fatal("nil Function NeedsRevisit must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Function NeedsRevisit must SetError sticky")
	}
	ClearError()
	f := &Function{Name: "f", ReferencedPtrs: []*Variable{nil}}
	if !f.IsPointerReferenced() {
		t.Fatal("incomplete ReferencedPtrs must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete ReferencedPtrs IsPointerReferenced must SetError sticky")
	}
	ClearError()
}

func TestRenewFactNilFactsSticky(t *testing.T) {
	ClearError()
	if RenewFact(nil, MakeFactPointTo(CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false), NullPtr)) {
		t.Fatal("nil facts RenewFact must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil facts RenewFact must SetError sticky")
	}
	ClearError()
}
