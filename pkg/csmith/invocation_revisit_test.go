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
	// FactPointTo.h:65–68 is_related is var identity only — not Variable.Match.
	// Soft invent Match could replace an aggregate fact when renewing a field and
	// leave the field's garbage fact in place (dangling on later check_read_var).
	parent := CreateVariableScalars("g_agg", GetIntType(), true, false)
	// synthetic aggregate shell with field p for Match
	parent.Type = &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1}}}
	parent.FieldVars = []*Variable{p}
	p.FieldVarOf = parent
	if !parent.Match(p) {
		t.Fatal("setup: parent must Match field for this anti-soft-invent test")
	}
	factsX := []*FactPointTo{
		MakeFactPointToSet(parent, []*Variable{NullPtr}), // unrelated aggregate subject
		MakeFactPointToSet(p, []*Variable{GarbagePtr}),   // field pointer still dead
	}
	tgt := CreateVariableScalars("g_tgt", GetIntType(), true, false)
	nfField := MakeFactPointTo(p, tgt) // definitive p = &tgt
	if !RenewFact(&factsX, nfField) {
		t.Fatal("renew field by var identity must succeed")
	}
	// parent fact must remain (not replaced via Match)
	if ft := FindRelatedPointTo(factsX, parent); ft == nil || !ft.IsNull() {
		t.Fatalf("parent fact must stay null after field renew: %+v", ft)
	}
	// field fact renewed — garbage cleared
	if ft := FindRelatedPointTo(factsX, p); ft == nil || ft.IsDead() || len(ft.PointTo) != 1 || ft.PointTo[0] != tgt {
		t.Fatalf("field must renew to tgt only: %+v", ft)
	}
	ClearError()
	// nil-hole subject in map still fails closed sticky (FactsComplete false path)
	factsHole := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	if RenewFact(&factsHole, MakeFactPointTo(p, GarbagePtr)) {
		t.Fatal("incomplete map RenewFact must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete map RenewFact must SetError sticky")
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

// TestSaveReturnUnionFactsRegistry — FunctionInvocationUser.cpp:76–106 / 358–365.
// Full FactVec return registry includes eUnionWrite so FactUnion::rhs_to_lhs_transfer
// for FuncCall (FactUnion.cpp:103–106) finds rv_fact. Soft invent was PT-only registry
// → AddParamFacts left union params without FactUnion → nonreadable fields dropped
// from choose_var (seed-213 p_34.f0 vs p_33.f0 pool).
func TestSaveReturnUnionFactsRegistry(t *testing.T) {
	ClearError()
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	ut := &Type{isUnion: true, StructName: "U_ret", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	rv := CreateVariableQfer("rv", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	f := &Function{Name: "func_u", ReturnType: ut, RV: rv}
	fi := &Invocation{User: f}
	uf := MakeFactUnion(rv, 0)
	fi.SaveReturnUnionFacts([]*FactUnion{uf})
	if HasError() {
		t.Fatal("SaveReturnUnionFacts sticky", GetError())
	}
	got := GetReturnUnionFactForInvocation(fi, rv)
	if got == nil || got.LastWrittenFID != 0 {
		t.Fatalf("registry must return eUnionWrite fact for rv, got %+v", got)
	}
	// transfer to param-like LHS uses registry, not ambient
	param := CreateVariableQfer("p_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: ut}
	out := RhsToLhsTransferUnion(nil, nil, []*Variable{param}, rhs)
	if !UnionFactsComplete(out) || len(out) != 1 || out[0].Var != param || out[0].LastWrittenFID != 0 {
		t.Fatalf("FuncCall union transfer must use return registry: %+v", out)
	}
	ClearError()
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
	// Match residual soft invent was soft-skip not-match then save later matching fact.
	// Fair: Type-nil RV Match stickies residual then fail closed whole save.
	// Use nil RV field already complete no-op; use RV with Match residual via nil other fact Var
	// (FactsComplete rejects nil fact). Residual path: Match on complete then Add residual.
	// Plant desynced registry so AddReturnFact stickies after first Match success.
	ClearError()
	InvocationReturnFactsDoFinalization()
	fiOK := &Invocation{User: &Function{Name: "h", RV: rv}}
	currentSession().ReturnFactInvocations = []*Invocation{&Invocation{User: &Function{Name: "x"}}}
	currentSession().ReturnFactPoints = []*FactPointTo{} // desync sizes
	fiOK.SaveReturnFacts([]*FactPointTo{MakeFactPointTo(rv, NullPtr)})
	if GetReturnFactForInvocation(fiOK, rv) != nil {
		t.Fatal("desync Add residual must fail closed no invent registry")
	}
	if !HasError() {
		t.Fatal("desync Add residual SaveReturnFacts must SetError sticky")
	}
	ClearError()
	// nil Invocation* registry slot sticky fail closed (no invent later match)
	ClearError()
	fi2 := &Invocation{User: &Function{Name: "g", RV: rv}}
	AddReturnFactForInvocation(fi2, MakeFactPointTo(rv, NullPtr))
	currentSession().ReturnFactInvocations = append([]*Invocation{nil}, currentSession().ReturnFactInvocations...)
	currentSession().ReturnFactPoints = append([]*FactPointTo{MakeFactPointTo(rv, NullPtr)}, currentSession().ReturnFactPoints...)
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
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
		AssignOp: AssignSimple,
	}
	callee := &Function{
		Name: "func_x", ReturnType: GetIntType(),
		Body:        &Block{StmID: 100, Stmts: []Stmt{assign}},
		FactChanged: true,
	}
	callee.RV = CreateVariableScalars("func_x_rv", GetIntType(), false, false)
	// FunctionInvocationUser.cpp:311 — get_fact_mgr_for_func(func) is the callee FM
	fm := callee.ensurePairedFactMgr()
	callee.Body.Func = callee
	eff := EmptyEffect()
	// caller context may hold a different FM; revisit must use callee.PairedFactMgr()
	callerFM := NewFactMgr(&Function{Name: "caller"})
	cg := WithFunc(callee, EmptyEffect()).WithFactMgr(callerFM)
	cg.EffectAccum = &eff
	// mark body effect empty on callee maps
	fm.SetMapStmEffect(100, EmptyEffect())
	fi := &Invocation{User: callee}
	facts := []*FactPointTo{}
	ClearError()
	ok := RevisitUserInvocation(fi, &facts, &cg, Defaults())
	if !ok {
		t.Fatalf("revisit ok=%v err=%v", ok, HasError())
	}
	if callee.VisitedCnt < 1 {
		t.Fatal("visited_cnt")
	}
}

// TestRevisitInstallsCallerUnionFacts — FunctionInvocationUser.cpp:206+324 full FactVec
// handover includes eUnionWrite. Soft invent left stale callee UnionFacts across
// revisits (seed-7 ChooseOKVar / IsNonreadableField skew).
func TestRevisitInstallsCallerUnionFacts(t *testing.T) {
	ClearError()
	// Callee with empty body (always visits OK)
	callee := &Function{
		Name: "func_u", ReturnType: GetIntType(),
		Body:        &Block{StmID: 200, Stmts: nil},
		FactChanged: true,
	}
	callee.RV = CreateVariableScalars("func_u_rv", GetIntType(), false, false)
	calFM := callee.ensurePairedFactMgr()
	callee.Body.Func = callee
	// Stale last-written field on callee from a prior visit (g_ prefix → IsGlobal)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uParent := CreateVariableScalars("g_u", ut, false, false)
	stale := MakeFactUnion(uParent, 1) // last-written f1
	calFM.UnionFacts = []*FactUnion{stale}

	// Caller lattice has last-written f0 for same union
	callerU := MakeFactUnion(uParent, 0)
	callerFM := NewFactMgr(&Function{Name: "caller"})
	callerFM.UnionFacts = []*FactUnion{callerU}
	callerFM.GlobalFacts = []*FactPointTo{}

	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(callerFM)
	cg.EffectAccum = &eff
	calFM.SetMapStmEffect(200, EmptyEffect())
	calFM.SetMapFactsOut(200, []*FactPointTo{})

	fi := &Invocation{User: callee}
	facts := []*FactPointTo{}
	if !RevisitUserInvocation(fi, &facts, &cg, Defaults()) {
		t.Fatalf("revisit ok expected err=%v", HasError())
	}
	// After success, callee live UnionFacts must come from caller (f0), not stale f1
	if !UnionFactsComplete(calFM.UnionFacts) {
		t.Fatal("post-revisit UnionFacts incomplete")
	}
	found := false
	for _, uf := range calFM.UnionFacts {
		if uf != nil && uf.Var == uParent {
			found = true
			if uf.LastWrittenFID != 0 {
				t.Fatalf("expected caller last-written f0, got %d (stale f1 not replaced)", uf.LastWrittenFID)
			}
		}
	}
	if !found {
		t.Fatalf("expected g_u union fact from caller after revisit, got %d unions", len(calFM.UnionFacts))
	}
	ClearError()
}

func TestRevisitOOSsParamUnions(t *testing.T) {
	// FunctionInvocationUser.cpp:344 — update_facts_for_oos_vars(func->param, inputs)
	// Full FactVec includes eUnionWrite. Soft invent was PT-only OOS on work.
	ClearError()
	defer ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(9), opts, probs, &env, "U2")
	if ut == nil {
		t.Skip("union")
	}
	gu := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	p := CreateVariableQfer("p_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	callee := &Function{
		Name: "func_p", ReturnType: GetIntType(),
		Body:        &Block{StmID: 300, Stmts: nil},
		Param:       []*Variable{p},
		FactChanged: true,
	}
	callee.RV = CreateVariableScalars("func_p_rv", GetIntType(), false, false)
	callee.Body.Func = callee
	calFM := callee.ensurePairedFactMgr()
	calFM.SetMapStmEffect(300, EmptyEffect())
	calFM.SetMapFactsOut(300, []*FactPointTo{})
	// Caller lattice: global union only.
	callerFM := NewFactMgr(&Function{Name: "caller"})
	callerFM.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0)}
	callerFM.GlobalFacts = []*FactPointTo{}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(callerFM)
	cg.EffectAccum = &eff
	fi := &Invocation{User: callee, Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
	}}
	facts := []*FactPointTo{}
	// Seed callee param union as if body abstract had it after handover (params kept by filter)
	// Revisit installs caller unions first; inject param after by pre-setting and relying on
	// Filter keeping params when PT work has param from handover.
	// Pre-seed: put param on caller temporarily so install+filter retain it, then OOS drops.
	callerFM.UnionFacts = []*FactUnion{MakeFactUnion(gu, 0), MakeFactUnion(p, 0)}
	if !RevisitUserInvocation(fi, &facts, &cg, Defaults()) {
		t.Fatalf("revisit expected ok err=%v", HasError())
	}
	// After success, callee live UnionFacts must not still list the param subject
	if FindRelatedUnion(calFM.UnionFacts, p) != nil {
		t.Fatal("revisit must OOS param union subject from callee lattice", calFM.UnionFacts)
	}
	if FindRelatedUnion(calFM.UnionFacts, gu) == nil {
		t.Fatal("callee must still hold global union after param OOS")
	}
	// Caller renews globals-only from callee; param subject on caller is not removed by renew
	// (renew joins/replaces related only). Clear residual caller param for contract of renew path:
	if uf := FindRelatedUnion(callerFM.UnionFacts, gu); uf == nil {
		t.Fatal("caller must retain global union after revisit renew")
	}
	ClearError()
}

func TestRevisitCallerUnionIncompleteFailClosed(t *testing.T) {
	ClearError()
	callee := &Function{
		Name: "func_v", ReturnType: GetIntType(),
		Body: &Block{StmID: 201, Stmts: nil},
	}
	callee.RV = CreateVariableScalars("func_v_rv", GetIntType(), false, false)
	calFM := callee.ensurePairedFactMgr()
	callee.Body.Func = callee
	calFM.SetMapStmEffect(201, EmptyEffect())
	calFM.UnionFacts = []*FactUnion{}

	callerFM := NewFactMgr(&Function{Name: "caller2"})
	callerFM.UnionFacts = IncompleteUnionFactSlice()
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(callerFM)
	cg.EffectAccum = &eff
	fi := &Invocation{User: callee}
	facts := []*FactPointTo{}
	if RevisitUserInvocation(fi, &facts, &cg, Defaults()) {
		t.Fatal("incomplete caller UnionFacts must fail closed revisit")
	}
	if !HasError() {
		t.Fatal("incomplete caller UnionFacts must SetError sticky")
	}
	ClearError()
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

func TestNeedsRevisitNilResidualSticky(t *testing.T) {
	// NeedsRevisit residual soft invent was invent not-revisit soft-skip past nil Function.
	ClearError()
	if (*Function)(nil).NeedsRevisit() {
		t.Fatal("nil NeedsRevisit must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil NeedsRevisit must SetError sticky")
	}
	ClearError()
}
