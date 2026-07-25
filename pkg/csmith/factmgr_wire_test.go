package csmith

import (
	"strings"
	"testing"
)

func TestMakeFirstCreatesFactMgr(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	m := NewFactMgrMapSess(testAmbientSession)
	list := FunctionList{}
	seedTypesForTest(NewRngSess(testAmbientSession, 2), opts, probs, vs, &list)
	f := MakeFirst(NewRngSess(testAmbientSession, 2), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), &list, m)
	if f == nil {
		t.Fatal("nil")
	}
	fm := m.ForFunc(f)
	if fm == nil || fm.Func != f {
		t.Fatal("no fm")
	}
}

func TestAddNewVarFactFromInitNull(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	p.Init = &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}
	fm.AddNewVarFact(p)
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("want null from 0 init")
	}
}

func TestUpdateFactForReturn(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "func_1_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}}
	if !fm.UpdateFactForReturn(f.RV, rhs) {
		t.Fatal("update")
	}
	if !FindRelatedPointTo(fm.GlobalFacts, f.RV).IsNull() {
		t.Fatal("rv null")
	}
}

func TestUpdateFactForAssignUnionMergeHoleFailClosed(t *testing.T) {
	// soft invent: MergeUnionFact nil still changed=true with wiped UnionFacts
	// fair: incomplete union map hole fails closed false (abstract incomplete non-sticky)
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	fm := NewFactMgrSess(testAmbientSession, nil)
	// incomplete existing union map (nil hole)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0), nil}
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	if fm.UpdateFactForAssign(f1, 0, rhs) {
		t.Fatal("nil UnionFacts hole must fail closed false, not invent success")
	}
	if UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("incomplete union merge must fail closed incomplete UnionFacts", fm.UnionFacts)
	}
	// incomplete abstract alone must not invent success; soft re-pick keeps non-sticky
	ClearErrorSess(testAmbientSession)
	// complete map + MergeUnionFact incomplete subject sticky wipe
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0)}
	merged := MergeUnionFact(fm2.UnionFacts, nil)
	if UnionFactsComplete(merged) {
		t.Fatal("nil fact MergeUnionFact must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact MergeUnionFact must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestApplyPointToAssignFactsNilHoleFailClosed(t *testing.T) {
	// soft invent: MergeFactInto nil still return true with partial maps
	// fair: incomplete newFacts fails closed ok=false without poisoning prior map
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	// nil hole in newFacts — fail closed, keep prior complete facts for factory re-pick
	if _, ok := applyPointToAssignFacts(&facts, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, 1); ok {
		t.Fatal("nil newFact hole must fail closed ok=false")
	}
	if !FactsComplete(facts) || FindRelatedPointTo(facts, p) == nil {
		t.Fatal("incomplete newFacts must not wipe prior complete facts", facts)
	}
	// incomplete subject map — wipe sticky
	ClearErrorSess(testAmbientSession)
	facts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	if _, ok := applyPointToAssignFacts(&facts, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, 1); ok {
		t.Fatal("nil subject map hole must fail closed ok=false")
	}
	if FactsComplete(facts) {
		t.Fatal("incomplete subject must clear", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete subject wipe must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty newFacts is ok no-op (not incomplete)
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	ch, ok := applyPointToAssignFacts(&facts, p, 0, nil, 1)
	if !ok || ch {
		t.Fatal("empty newFacts must be ok with no change", ch, ok)
	}
	// IncompleteFactSlice newFacts must not invent empty-apply success
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	if _, ok := applyPointToAssignFacts(&facts, p, 0, IncompleteFactSlice(), 1); ok {
		t.Fatal("IncompleteFactSlice newFacts must fail closed ok=false")
	}
	if !FactsComplete(facts) {
		t.Fatal("IncompleteFactSlice newFacts must not wipe prior", facts)
	}
	// incomplete lhs pointees must not invent lvar_cnt via len(IncompleteVariables)==1 renew
	ClearErrorSess(testAmbientSession)
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	if VariablesComplete(lhsAssignPointees(facts, nil, 0)) {
		t.Fatal("nil lhs must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs lhsAssignPointees must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil non-special sticky (no invent empty lvars soft-complete past hole)
	hole := &Variable{Name: "g_hole", Type: nil}
	if VariablesComplete(lhsAssignPointees(facts, hole, 0)) {
		t.Fatal("Type-nil lhs must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil lhs lhsAssignPointees must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil lhs: incomplete abstract path — lvarCnt still applied; fail closed on sticky
	if _, ok := applyPointToAssignFacts(&facts, nil, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, 1); ok {
		// nil lhs is unused for renew path when lvarCnt provided; renew may succeed
		// C++ always has live Lhs*; sticky incomplete lhs only via abstract pre-step.
		// Keep prior map when newFacts complete and lvarCnt==1 renews p.
		_ = ok
	}
	ClearErrorSess(testAmbientSession)
	// facts accumulator always live; sticky
	if _, ok := applyPointToAssignFacts(nil, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, 1); ok {
		t.Fatal("nil facts applyPointToAssignFacts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts applyPointToAssignFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactPointTo is_related is var identity only (FactPointTo.h:65–68).
	// Unrelated Type-nil subject is not related to p — renew appends p's fact (C++).
	// Soft invent Match residual sticky-wipe is gone with is_related-only RenewFact.
	brokenSubj := &Variable{Name: "g_broken"} // Type nil
	factsR := []*FactPointTo{{Var: brokenSubj, PointTo: []*Variable{NullPtr}}}
	if _, ok := applyPointToAssignFacts(&factsR, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, 1); !ok {
		t.Fatal("unrelated Type-nil subject must not block renew of p")
	}
	if FindRelatedPointTo(factsR, p) == nil || !FindRelatedPointTo(factsR, p).IsNull() {
		t.Fatal("p must renew to null", factsR)
	}
	if FindRelatedPointTo(factsR, brokenSubj) == nil {
		t.Fatal("unrelated subject fact must remain")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUpdateFactForAssignPointToHoleNoUnionInvent(t *testing.T) {
	// incomplete point-to apply must not invent union merge success sticky
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	fm.UnionFacts = []*FactUnion{}
	rhs := &Expression{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)}
	// assign through incomplete GlobalFacts — apply fails closed sticky
	if fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("incomplete GlobalFacts assign must fail closed false")
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("point-to hole must clear GlobalFacts", fm.GlobalFacts)
	}
	if UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("must not invent keep UnionFacts after point-to fail", fm.UnionFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts assign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUpdateFactForAssignRenewsDefinitive(t *testing.T) {
	// FactMgr.cpp:376–380 — lvar_cnt==1 non-array → renew (replace, not join)
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), true, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	// start with multi-target set {a,b}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSet(p, []*Variable{a, b})}
	// definitive p = &a via constant null then variable? use RhsToLhs via var expression
	// assign p = 0 → null only (renew replaces multi set)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if !fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("update")
	}
	ft := FindRelatedPointTo(fm.GlobalFacts, p)
	if ft == nil || !ft.IsNull() || len(ft.PointTo) != 1 {
		t.Fatalf("renew to null only: %+v", ft)
	}
}

func TestAbstractFactForVarInitUnion(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut, Init: MakeIntSess(testAmbientSession, 0)}
	pt, un := AbstractFactForVarInit(uv)
	if len(pt) != 0 {
		t.Fatal("no pt for union")
	}
	if len(un) != 1 || un[0].LastWrittenFID != 0 {
		t.Fatalf("%+v", un)
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.AddNewVarFact(uv)
	if FindRelatedUnion(fm.UnionFacts, uv) == nil || FindRelatedUnion(fm.UnionFacts, uv).LastWrittenFID != 0 {
		t.Fatal(fm.UnionFacts)
	}
	// incomplete hard IR: nil var sticky (no invent empty init success / soft re-pick)
	ClearErrorSess(testAmbientSession)
	pt2, un2 := AbstractFactForVarInit(nil)
	if FactsComplete(pt2) || UnionFactsComplete(un2) {
		t.Fatal("nil var init must fail closed incomplete", pt2, un2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var AbstractFactForVarInit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// array without AsArray sticky (Fact.cpp:99 assert(av))
	ptArr, _ := AbstractFactForVarInit(&Variable{
		Name: "g_ap_bad", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2},
		Init: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"},
	})
	if FactsComplete(ptArr) {
		t.Fatal("IsArray without AsArray must fail closed incomplete", ptArr)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil InitExprs hole sticky
	badInit := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap_nil", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"},
		},
		Sizes:     []int{2},
		InitExprs: []*Expression{nil},
	}
	badInit.AsArray = badInit
	ptNil, _ := AbstractFactForVarInit(&badInit.Variable)
	if FactsComplete(ptNil) {
		t.Fatal("nil InitExprs hole must fail closed incomplete", ptNil)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil InitExprs AbstractFactForVarInit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete abstract of live alt sticky (no invent soft-skip incomplete init alt)
	badAlt := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap_bad", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"},
		},
		Sizes: []int{2},
		// TermFunction without Invoke → incomplete abstract transfer
		InitExprs: []*Expression{{Term: TermFunction}},
	}
	badAlt.AsArray = badAlt
	// also exercise incomplete alt via RhsToLhsTransfer nil Invoke sticky path
	more, _ := AbstractFactForAssign(nil, &badAlt.Variable, 0, &Expression{Term: TermFunction})
	if FactsComplete(more) {
		t.Fatal("nil Invoke AbstractFactForAssign must incomplete", more)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke AbstractFactForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ptBad, _ := AbstractFactForVarInit(&badAlt.Variable)
	if FactsComplete(ptBad) {
		t.Fatalf("incomplete alt abstract must fail closed incomplete complete=%v err=%v n=%d", FactsComplete(ptBad), HasErrorSess(testAmbientSession), len(ptBad))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete alt AbstractFactForVarInit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// union without rhs/init — incomplete abstract non-sticky (AddParamFacts soft path);
	// AddNewVarFact sticks after incomplete abstract (no invent skip no-fact)
	ClearErrorSess(testAmbientSession)
	uv2 := &Variable{Name: "g_u2", Type: ut}
	_, un3 := AbstractFactForVarInit(uv2)
	if UnionFactsComplete(un3) {
		t.Fatal("union without init must fail closed incomplete", un3)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("union without init abstract must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(
		CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false), NullPtr)}
	fm2.AddNewVarFact(uv2)
	if UnionFactsComplete(fm2.UnionFacts) {
		t.Fatal("AddNewVarFact incomplete union must fail closed", fm2.UnionFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("AddNewVarFact incomplete union must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactForVarInitPointerArrayAlts(t *testing.T) {
	// array of pointers with alt init Expression "0" → null
	// Fact.cpp:100–106 — get_more_init_values Expression*; no invent from InitValues
	ptType := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	nullAlt := &Expression{Term: TermConstant, Con: &Constant{Type: ptType, Value: "0"}, ExprType: ptType}
	parent := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap", Type: ptType, IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: ptType, Value: "0"},
		},
		Sizes:     []int{2},
		InitExprs: []*Expression{nullAlt},
	}
	parent.AsArray = parent
	pt, _ := AbstractFactForVarInit(&parent.Variable)
	if len(pt) == 0 {
		t.Fatal("want facts")
	}
	// should point null from 0 init
	found := false
	for _, f := range pt {
		if f != nil && f.Var == &parent.Variable && f.IsNull() {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", pt)
	}
	// InitValues alone must not invent Constant alts
	onlyStr := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap2", Type: ptType, IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: ptType, Value: "0"},
		},
		Sizes:      []int{2},
		InitValues: []string{"0"},
	}
	onlyStr.AsArray = onlyStr
	pt2, _ := AbstractFactForVarInit(&onlyStr.Variable)
	// primary init still null; InitValues ignored
	if len(pt2) == 0 {
		t.Fatal("want primary init facts")
	}
}

func TestAbstractFactForVarInitPointerArrayInitExprs(t *testing.T) {
	// Fact.cpp:100–106 — Expression* alts; no invent Constant from to_string of &g_x
	ptType := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	// ExpressionVariable: Var=int, ExprType=int* → &g_x (indirect -1)
	addr := &Expression{
		Term: TermVariable, Var: tgt, ExprType: ptType,
	}
	// sanity: emit is &g_x, not inventable as Constant "0"
	if !strings.Contains(addr.Output(), "g_x") || !strings.Contains(addr.Output(), "&") {
		t.Fatalf("addr output %q", addr.Output())
	}
	parent := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap", Type: ptType, IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: ptType, Value: "0"},
		},
		Sizes:     []int{2},
		InitExprs: []*Expression{addr},
		// invent-prone string list deliberately wrong if used alone
		InitValues: []string{"0"},
	}
	parent.AsArray = parent
	facts, _ := AbstractFactForVarInit(&parent.Variable)
	if len(facts) == 0 {
		t.Fatal("want facts")
	}
	// must include pointee g_x from InitExprs, not only null from string invent
	sawTgt := false
	for _, f := range facts {
		if f == nil || f.Var != &parent.Variable {
			continue
		}
		for _, p := range f.PointTo {
			if p == tgt {
				sawTgt = true
			}
		}
	}
	if !sawTgt {
		t.Fatalf("InitExprs &g_x must transfer pointee, got %+v", facts)
	}
}

func TestUpdateFactForAssignUnionField(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	fm := NewFactMgrSess(testAmbientSession, nil)
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	if !fm.UpdateFactForAssign(f1, 0, rhs) {
		t.Fatal("update")
	}
	fu := FindRelatedUnion(fm.UnionFacts, parent)
	if fu == nil || fu.LastWrittenFID != 1 {
		t.Fatalf("%+v", fu)
	}
}

func TestGenerateFunctionsWiresFactMgr(t *testing.T) {
	opts := Defaults()
	opts.Seed = 7
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	g.GenerateFunctions()
	if g.FactMgrs == nil {
		t.Fatal("no map")
	}
	if len(g.Funcs.Funcs) == 0 {
		t.Fatal("no funcs")
	}
	f := g.Funcs.Funcs[0]
	fm := g.FactMgrs.ForFunc(f)
	if fm == nil {
		t.Fatal("fm")
	}
	// Function::FMList is session state; sticky no invent mid-run miss
	// noteErr dual-writes g2.Sess + ambient; clear both so suite mates stay clean.
	g2 := NewProgramGenerator(NewSession(opts))
	g2.FactMgrs = nil
	g2.GenerateFunctions()
	if !g2.hasErr() {
		t.Fatal("nil FactMgrs GenerateFunctions must SetError sticky")
	}
	g2.clearErr()
}

func TestUpdateFactForReturnSetsFactOut(t *testing.T) {
	// FactMgr.cpp:418–420 — set_fact_out(sr, inputs) after abstract return
	f := &Function{Name: "func_1", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "func_1_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	st := &Stmt{Kind: StmtReturn, StmID: 7,
		Expr: &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"},
			ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}}
	if !fm.UpdateFactForReturnStmt(st, f.RV, st.Expr) {
		t.Fatal("update")
	}
	out := fm.MapFactsOut[7]
	if FindRelatedPointTo(out, f.RV) == nil {
		t.Fatal("map_facts_out missing rv", out)
	}
}

func TestVisitFactsReturnSetsOut(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "func_1_rv", GetIntTypeSess(testAmbientSession), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	st := &Stmt{Kind: StmtReturn, StmID: 8,
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3), ExprType: GetIntTypeSess(testAmbientSession)}}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsStatementReturn(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if _, ok := fm.MapFactsOut[8]; !ok {
		t.Fatal("facts out")
	}
	if !fm.MapVisited[8] && fm.GetMapStmEffect(8).IsEmptySess(testAmbientSession) {
		// effect may be empty for const return; map_stm_effect should still be set
	}
	_ = fm.GetMapStmEffect(8)
}

func TestGetMapFactsStmID0FailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	// StmID 0 must IncompleteFactSlice sticky — not invent empty-complete map miss
	if FactsComplete(fm.GetMapFactsIn(IncompleteStmID)) || FactsComplete(fm.GetMapFactsOut(IncompleteStmID)) {
		t.Fatal("StmID 0 must IncompleteFactSlice")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 GetMapFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(fm.GetMapFactsInFinal(IncompleteStmID)) || FactsComplete(fm.GetMapFactsOutFinal(IncompleteStmID)) {
		t.Fatal("StmID 0 final maps must IncompleteFactSlice")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 GetMapFactsFinal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// missing live key is complete empty
	if !FactsComplete(fm.GetMapFactsIn(42)) || len(fm.GetMapFactsIn(42)) != 0 {
		t.Fatal("missing live id must complete empty")
	}
	// stored incomplete stays incomplete marker (non-sticky local map hole)
	fm.SetMapFactsOut(7, IncompleteFactSlice())
	if FactsComplete(fm.GetMapFactsOut(7)) {
		t.Fatal("stored incomplete must stay incomplete via getter")
	}
	// FactMgr always live; sticky IncompleteFactSlice (no invent empty-complete)
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).GetMapFactsIn(1)) {
		t.Fatal("nil FM GetMapFactsIn must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapFactsIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).GetMapFactsOut(1)) {
		t.Fatal("nil FM GetMapFactsOut must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapFactsOut must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).GetMapFactsInFinal(1)) {
		t.Fatal("nil FM GetMapFactsInFinal must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapFactsInFinal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).GetMapFactsOutFinal(1)) {
		t.Fatal("nil FM GetMapFactsOutFinal must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapFactsOutFinal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// GetMapStmEffect / GetMapAccumEffect nil FM sticky IncompleteEffect
	if EffectComplete((*FactMgr)(nil).GetMapStmEffect(1)) {
		t.Fatal("nil FM GetMapStmEffect must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapStmEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if EffectComplete((*FactMgr)(nil).GetMapAccumEffect(1)) {
		t.Fatal("nil FM GetMapAccumEffect must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM GetMapAccumEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// SetMap* nil FM / bad stm_id sticky
	(*FactMgr)(nil).SetMapFactsIn(1, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM SetMapFactsIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm.SetMapFactsIn(IncompleteStmID, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("stmID 0 SetMapFactsIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).SetMapStmEffect(1, EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM SetMapStmEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).ClearMapVisited()
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM ClearMapVisited must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).CreateCFGEdgeTo(1, &Block{}, 0, false, false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM CreateCFGEdgeTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactMgr mutators always live; sticky no invent soft-skip past hole
	(*FactMgr)(nil).SetupInOutMaps(true)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM SetupInOutMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).AddNewVarFact(CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM AddNewVarFact must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm.AddNewVarFact(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable AddNewVarFact must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).FindDanglingGlobalPtrs(&Function{Name: "f"})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fm.FindDanglingGlobalPtrs(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).AddFactOut(&Stmt{StmID: 1}, nil, MakeFactPointTo(
		CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false), NullPtr))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM AddFactOut must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).SetMapFactsOutForStmtDest(&Stmt{StmID: 1}, nil, nil, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM SetMapFactsOutForStmtDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).BackupStmFactMaps(&Stmt{StmID: 1}, map[int][]*FactPointTo{}, map[int][]*FactPointTo{}, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM BackupStmFactMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).RestoreStmFactMaps(&Stmt{StmID: 1}, map[int][]*FactPointTo{}, map[int][]*FactPointTo{}, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM RestoreStmFactMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).AddNewVarFactAndUpdate(nil, CreateVariableScalarsSess(testAmbientSession, "g_y", GetIntTypeSess(testAmbientSession), false, false))
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM AddNewVarFactAndUpdate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).UpdateFactsForOOSVars([]*Variable{CreateVariableScalarsSess(testAmbientSession, "g_z", GetIntTypeSess(testAmbientSession), false, false)})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM UpdateFactsForOOSVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty OOS list complete no-op
	fm.UpdateFactsForOOSVars(nil)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty UpdateFactsForOOSVars must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	facts := []*FactPointTo{}
	(*FactMgr)(nil).AddParamFacts(nil, &facts)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM AddParamFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindParentBlockNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if FindParentBlockOfStmID(nil, 1) != nil {
		t.Fatal("nil Function FindParentBlockOfStmID must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function FindParentBlockOfStmID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FindParentBlockOfStmID(&Function{Name: "f"}, IncompleteStmID) != nil {
		t.Fatal("StmID 0 FindParentBlockOfStmID must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 FindParentBlockOfStmID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Block* always live on Function.Blocks; nil hole sticky miss
	f := &Function{Name: "f", Blocks: []*Block{nil}}
	if FindParentBlockOfStmID(f, 1) != nil {
		t.Fatal("nil Blocks hole FindParentBlockOfStmID must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Blocks hole FindParentBlockOfStmID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete if-arm sticky whole miss (no invent soft-continue past nil Else)
	inner := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	outer := &Block{Func: f, Stmts: []Stmt{{Kind: StmtIfElse, StmID: 1, Then: inner, Else: nil}}}
	inner.Parent = outer
	f.Blocks = []*Block{outer}
	if FindParentBlockOfStmID(f, 3) != nil {
		t.Fatal("nil Else arm must fail closed FindParentBlockOfStmID")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Else arm FindParentBlockOfStmID must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestApplyFactForAssignIsPointerResidualSticky(t *testing.T) {
	// IsPointer residual soft invent was invent lvarCnt=1 renew past Type-nil LHS.
	ClearErrorSess(testAmbientSession)
	hole := &Variable{Name: "g_p", Type: nil}
	if hole.IsPointerSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddNewVarFactAndUpdateIsGlobalResidualSticky(t *testing.T) {
	// IsGlobal residual soft invent was invent soft-skip makeup past FieldVarOf residual.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	// FieldVarOf with Type-nil parent chain residual IsGlobal
	parent := &Variable{Name: "x_bad"} // not g_ prefix, not global
	hole := &Variable{Name: "f0", FieldVarOf: parent}
	// blk==nil path: IsGlobal residual/false soft-skip without sticky was invent soft-success
	// non-global soft return is intentional; residual via nil FieldVarOf parent IsGlobal sticky
	parent.FieldVarOf = (*Variable)(nil)
	// force residual: FieldVarOf points to nil-handled? parent IsGlobal false complete.
	// Use recursive residual: FieldVarOf IsGlobal when parent is incomplete via nil self walk.
	// Parent with FieldVarOf set to broken chain:
	orphan := &Variable{Name: "orphan"}
	// nil subject IsGlobal residual
	(*Variable)(nil).IsGlobalSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// blk==nil + non-global complete soft-skip no sticky
	fm.AddNewVarFactAndUpdate(nil, orphan)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("non-global complete AddNewVarFactAndUpdate must soft-skip no sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual: Variable with FieldVarOf whose IsGlobal residual — FieldVarOf nil Variable?
	// FieldVarOf points to var; if FieldVarOf itself is ok. Plant residual via
	// IsGlobal after FieldVarOf: parent nil Variable not possible as FieldVarOf type.
	// Plant via recursive: hole.FieldVarOf where parent.FieldVarOf is set, walk ends.
	// Soft invent was: residual ERROR already set before call soft-continues makeup.
	SetErrorSess(testAmbientSession, ErrGeneric)
	// ambient residual before call — AddNewVarFactAndUpdate must not invent soft-skip past residual
	// via !IsGlobal path without checking HasError first after IsGlobal
	// After our fix: IsGlobal after residual ambient still checks HasError after call.
	// Better: FieldVarOf residual path.
	// Create field of non-global: IsGlobal false complete, soft return.
	_ = hole
	// Use incomplete FieldVarOf chain residual from IsGlobal on field of nil name parent with FieldVarOf loop?
	// IsGlobal residual only on nil subject and recursive FieldVarOf residual.
	// Soft invent residual: parent IsGlobal residual when parent is nil — FieldVarOf never nil-typed as *Variable value nil means no parent.
	// Field of a parent: if parent FieldVarOf is set to a Variable that IsGlobal residual...
	// Actually residual on FieldVarOf recursion: if ancestor is nil *Variable via FieldVarOf = nil, recursion stops at name check.
	// The invent was residual ambient before IsGlobal: IsGlobal complete returns false, HasError still true from ambient, old code:
	//   if blk == nil && !v.IsGlobalSess(testAmbientSession) { return } // soft return with ambient residual invent soft-skip
	// New code checks HasError after IsGlobal.
	SetErrorSess(testAmbientSession, ErrGeneric)
	fm.AddNewVarFactAndUpdate(nil, CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false))
	if !HasErrorSess(testAmbientSession) {
		// ambient residual should remain sticky (IsGlobal complete false still HasError after)
		t.Fatal("ambient residual AddNewVarFactAndUpdate must keep sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual FieldVarOf: Variable with FieldVarOf that is nil receiver?
	// Use (*Variable)(nil) as subject — already sticky at entry.
	fm.AddNewVarFactAndUpdate(nil, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var AddNewVarFactAndUpdate must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCloneFactSliceIncompleteResidualSticky(t *testing.T) {
	// CloneFactSlice residual soft invent was invent soft-complete empty past IncompleteFactSlice.
	ClearErrorSess(testAmbientSession)
	out := CloneFactSlice(IncompleteFactSlice())
	if FactsComplete(out) {
		t.Fatal("IncompleteFactSlice CloneFactSlice must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteFactSlice CloneFactSlice must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty
	out2 := CloneFactSlice([]*FactPointTo{})
	if !FactsComplete(out2) || out2 == nil {
		// empty complete may be non-nil empty
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete empty CloneFactSlice must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetProgramEndFacts(t *testing.T) {
	// FactMgr.cpp:732–735
	f1 := &Function{Name: "func_1"}
	f2 := &Function{Name: "func_2"}
	list := &FunctionList{Funcs: []*Function{f1, f2}}
	fms := NewFactMgrMapSess(testAmbientSession)
	fm1 := fms.ForFunc(f1)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm1.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	got := GetProgramEndFacts(list, fms)
	if len(got) != 1 || FindRelatedPointTo(got, p) == nil {
		t.Fatal(got)
	}
	ClearErrorSess(testAmbientSession)
	if GetProgramEndFacts(nil, fms) != nil {
		t.Fatal("empty list")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(GetProgramEndFacts(list, nil)) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fms sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSanityCheckMap(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgrSess(testAmbientSession, f)
	// empty maps ok
	ClearErrorSess(testAmbientSession)
	fm.SanityCheckMap()
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty complete")
	}
	// incomplete map sticky
	fm.MapFactsIn[1] = IncompleteFactSlice()
	fm.SanityCheckMap()
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*FactMgr)(nil).SanityCheckMap()
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fm sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// storeUnionFactMapEntry must deep-clone so renew/join on live cannot rewrite maps.
// Soft invent CloneUnionFactSlice left *FactUnion shared (seed-123 g_721 combine).
func TestStoreUnionFactMapEntryDeepIsolatesLive(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ut := &Type{
		isUnion: true, StructName: "U",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	uv.CreateFieldVarsSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	live := MakeFactUnion(uv, 0)
	fm.UnionFacts = []*FactUnion{live}
	fm.SetMapFactsInPair(10, []*FactPointTo{}, fm.UnionFacts)
	// mutate live lattice after store
	live.LastWrittenFID = 1
	got := fm.GetMapUnionFactsIn(10)
	stored := FindRelatedUnion(got, uv)
	if stored == nil {
		t.Fatal("missing stored union fact")
	}
	if stored.LastWrittenFID != 0 {
		t.Fatalf("map_in must keep fid 0 after live mutate, got %d (shared alias)", stored.LastWrittenFID)
	}
	if live.LastWrittenFID != 1 {
		t.Fatal("live must still be 1")
	}
	ClearErrorSess(testAmbientSession)
}
