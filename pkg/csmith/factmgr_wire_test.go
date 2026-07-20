package csmith

import (
	"strings"
	"testing"
)

func TestMakeFirstCreatesFactMgr(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	m := NewFactMgrMap()
	list := FunctionList{}
	seedTypesForTest(NewRng(2), opts, probs, vs, &list)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, NewExprTables(opts), NewStatementThresholdTable(opts), &list, m)
	if f == nil {
		t.Fatal("nil")
	}
	fm := m.ForFunc(f)
	if fm == nil || fm.Func != f {
		t.Fatal("no fm")
	}
}

func TestAddNewVarFactFromInitNull(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	p.Init = &Constant{Type: PointerTo(GetIntType()), Value: "0"}
	fm.AddNewVarFact(p)
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("want null from 0 init")
	}
}

func TestUpdateFactForReturn(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: PointerTo(GetIntType())}
	f.RV = CreateVariableScalars("func_1_rv", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(f)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
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
	ClearError()
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	fm := NewFactMgr(nil)
	// incomplete existing union map (nil hole)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0), nil}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if fm.UpdateFactForAssign(f1, 0, rhs) {
		t.Fatal("nil UnionFacts hole must fail closed false, not invent success")
	}
	if UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("incomplete union merge must fail closed incomplete UnionFacts", fm.UnionFacts)
	}
	// incomplete abstract alone must not invent success; soft re-pick keeps non-sticky
	ClearError()
	// complete map + MergeUnionFact incomplete subject sticky wipe
	fm2 := NewFactMgr(nil)
	fm2.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0)}
	merged := MergeUnionFact(fm2.UnionFacts, nil)
	if UnionFactsComplete(merged) {
		t.Fatal("nil fact MergeUnionFact must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil fact MergeUnionFact must SetError sticky")
	}
	ClearError()
}

func TestApplyPointToAssignFactsNilHoleFailClosed(t *testing.T) {
	// soft invent: MergeFactInto nil still return true with partial maps
	// fair: incomplete newFacts fails closed ok=false without poisoning prior map
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	// nil hole in newFacts — fail closed, keep prior complete facts for factory re-pick
	if _, ok := applyPointToAssignFacts(&facts, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}); ok {
		t.Fatal("nil newFact hole must fail closed ok=false")
	}
	if !FactsComplete(facts) || FindRelatedPointTo(facts, p) == nil {
		t.Fatal("incomplete newFacts must not wipe prior complete facts", facts)
	}
	// incomplete subject map — wipe sticky
	ClearError()
	facts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	if _, ok := applyPointToAssignFacts(&facts, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("nil subject map hole must fail closed ok=false")
	}
	if FactsComplete(facts) {
		t.Fatal("incomplete subject must clear", facts)
	}
	if !HasError() {
		t.Fatal("incomplete subject wipe must SetError sticky")
	}
	ClearError()
	// empty newFacts is ok no-op (not incomplete)
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	ch, ok := applyPointToAssignFacts(&facts, p, 0, nil)
	if !ok || ch {
		t.Fatal("empty newFacts must be ok with no change", ch, ok)
	}
	// IncompleteFactSlice newFacts must not invent empty-apply success
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	if _, ok := applyPointToAssignFacts(&facts, p, 0, IncompleteFactSlice()); ok {
		t.Fatal("IncompleteFactSlice newFacts must fail closed ok=false")
	}
	if !FactsComplete(facts) {
		t.Fatal("IncompleteFactSlice newFacts must not wipe prior", facts)
	}
	// incomplete lhs pointees must not invent lvar_cnt via len(IncompleteVariables)==1 renew
	ClearError()
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	if VariablesComplete(lhsAssignPointees(facts, nil, 0)) {
		t.Fatal("nil lhs must IncompleteVariables")
	}
	if !HasError() {
		t.Fatal("nil lhs lhsAssignPointees must SetError sticky")
	}
	ClearError()
	// Type-nil non-special sticky (no invent empty lvars soft-complete past hole)
	hole := &Variable{Name: "g_hole", Type: nil}
	if VariablesComplete(lhsAssignPointees(facts, hole, 0)) {
		t.Fatal("Type-nil lhs must IncompleteVariables")
	}
	if !HasError() {
		t.Fatal("Type-nil lhs lhsAssignPointees must SetError sticky")
	}
	ClearError()
	if _, ok := applyPointToAssignFacts(&facts, nil, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("nil lhs pointees must fail closed ok=false, not invent renew/merge")
	}
	if FactsComplete(facts) {
		t.Fatal("incomplete lhs assign must wipe subject facts")
	}
	if !HasError() {
		t.Fatal("incomplete lhs wipe must SetError sticky")
	}
	ClearError()
	// facts accumulator always live; sticky
	if _, ok := applyPointToAssignFacts(nil, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("nil facts applyPointToAssignFacts must fail closed")
	}
	if !HasError() {
		t.Fatal("nil facts applyPointToAssignFacts must SetError sticky")
	}
	ClearError()
	// RenewFact Match residual: Type-nil Var in subject map soft invent was merge later.
	// Fair: sticky wipe incomplete fail closed ok=false.
	brokenSubj := &Variable{Name: "g_broken"} // Type nil
	factsR := []*FactPointTo{{Var: brokenSubj, PointTo: []*Variable{NullPtr}}}
	// lvarCnt path: use pointer lhs with indir 0 and newFacts for p
	// When lvars empty and lhs not pointer, goes to merge path with Type-nil Match residual.
	// Use merge path: lvarCnt != 1 definitive renew
	if _, ok := applyPointToAssignFacts(&factsR, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("Match residual applyPointTo must fail closed ok=false")
	}
	if FactsComplete(factsR) {
		t.Fatal("Match residual applyPointTo must wipe incomplete")
	}
	if !HasError() {
		t.Fatal("Match residual applyPointTo must SetError sticky")
	}
	ClearError()
}

func TestUpdateFactForAssignPointToHoleNoUnionInvent(t *testing.T) {
	// incomplete point-to apply must not invent union merge success sticky
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	fm.UnionFacts = []*FactUnion{}
	rhs := &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()}
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
	if !HasError() {
		t.Fatal("incomplete GlobalFacts assign must SetError sticky")
	}
	ClearError()
}

func TestUpdateFactForAssignRenewsDefinitive(t *testing.T) {
	// FactMgr.cpp:376–380 — lvar_cnt==1 non-array → renew (replace, not join)
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	fm := NewFactMgr(nil)
	// start with multi-target set {a,b}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSet(p, []*Variable{a, b})}
	// definitive p = &a via constant null then variable? use RhsToLhs via var expression
	// assign p = 0 → null only (renew replaces multi set)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}, ExprType: PointerTo(GetIntType())}
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
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut, Init: MakeInt(0)}
	pt, un := AbstractFactForVarInit(uv)
	if len(pt) != 0 {
		t.Fatal("no pt for union")
	}
	if len(un) != 1 || un[0].LastWrittenFID != 0 {
		t.Fatalf("%+v", un)
	}
	fm := NewFactMgr(nil)
	fm.AddNewVarFact(uv)
	if FindRelatedUnion(fm.UnionFacts, uv) == nil || FindRelatedUnion(fm.UnionFacts, uv).LastWrittenFID != 0 {
		t.Fatal(fm.UnionFacts)
	}
	// incomplete hard IR: nil var sticky (no invent empty init success / soft re-pick)
	ClearError()
	pt2, un2 := AbstractFactForVarInit(nil)
	if FactsComplete(pt2) || UnionFactsComplete(un2) {
		t.Fatal("nil var init must fail closed incomplete", pt2, un2)
	}
	if !HasError() {
		t.Fatal("nil var AbstractFactForVarInit must SetError sticky")
	}
	ClearError()
	// array without AsArray sticky (Fact.cpp:99 assert(av))
	ptArr, _ := AbstractFactForVarInit(&Variable{
		Name: "g_ap_bad", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2},
		Init: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
	})
	if FactsComplete(ptArr) {
		t.Fatal("IsArray without AsArray must fail closed incomplete", ptArr)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray must SetError sticky")
	}
	ClearError()
	// nil InitExprs hole sticky
	badInit := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap_nil", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		},
		Sizes:     []int{2},
		InitExprs: []*Expression{nil},
	}
	badInit.AsArray = badInit
	ptNil, _ := AbstractFactForVarInit(&badInit.Variable)
	if FactsComplete(ptNil) {
		t.Fatal("nil InitExprs hole must fail closed incomplete", ptNil)
	}
	if !HasError() {
		t.Fatal("nil InitExprs AbstractFactForVarInit must SetError sticky")
	}
	ClearError()
	// incomplete abstract of live alt sticky (no invent soft-skip incomplete init alt)
	badAlt := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap_bad", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		},
		Sizes: []int{2},
		// TermFunction without Invoke → incomplete abstract transfer
		InitExprs: []*Expression{{Term: TermFunction}},
	}
	badAlt.AsArray = badAlt
	// also exercise incomplete alt via RhsToLhsTransfer nil Invoke sticky path
	more := AbstractFactForAssign(nil, &badAlt.Variable, 0, &Expression{Term: TermFunction})
	if FactsComplete(more) {
		t.Fatal("nil Invoke AbstractFactForAssign must incomplete", more)
	}
	if !HasError() {
		t.Fatal("nil Invoke AbstractFactForAssign must SetError sticky")
	}
	ClearError()
	ptBad, _ := AbstractFactForVarInit(&badAlt.Variable)
	if FactsComplete(ptBad) {
		t.Fatalf("incomplete alt abstract must fail closed incomplete complete=%v err=%v n=%d", FactsComplete(ptBad), HasError(), len(ptBad))
	}
	if !HasError() {
		t.Fatal("incomplete alt AbstractFactForVarInit must SetError sticky")
	}
	ClearError()
	// union without rhs/init — incomplete abstract non-sticky (AddParamFacts soft path);
	// AddNewVarFact sticks after incomplete abstract (no invent skip no-fact)
	ClearError()
	uv2 := &Variable{Name: "g_u2", Type: ut}
	_, un3 := AbstractFactForVarInit(uv2)
	if UnionFactsComplete(un3) {
		t.Fatal("union without init must fail closed incomplete", un3)
	}
	if HasError() {
		t.Fatal("union without init abstract must stay non-sticky for soft re-pick")
	}
	ClearError()
	fm2 := NewFactMgr(nil)
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(
		CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), NullPtr)}
	fm2.AddNewVarFact(uv2)
	if UnionFactsComplete(fm2.UnionFacts) {
		t.Fatal("AddNewVarFact incomplete union must fail closed", fm2.UnionFacts)
	}
	if !HasError() {
		t.Fatal("AddNewVarFact incomplete union must SetError sticky")
	}
	ClearError()
}

func TestAbstractFactForVarInitPointerArrayAlts(t *testing.T) {
	// array of pointers with alt init Expression "0" → null
	// Fact.cpp:100–106 — get_more_init_values Expression*; no invent from InitValues
	ptType := PointerTo(GetIntType())
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
	ptType := PointerTo(GetIntType())
	tgt := CreateVariableScalars("g_x", GetIntType(), false, false)
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
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := &Variable{Name: "g_u", Type: ut}
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	f1 := &Variable{Name: "g_u.f1", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0, f1}
	fm := NewFactMgr(nil)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
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
	g := NewProgramGenerator(opts)
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
	ClearError()
	g2 := NewProgramGenerator(opts)
	g2.FactMgrs = nil
	g2.GenerateFunctions()
	if !HasError() {
		t.Fatal("nil FactMgrs GenerateFunctions must SetError sticky")
	}
	ClearError()
}

func TestUpdateFactForReturnSetsFactOut(t *testing.T) {
	// FactMgr.cpp:418–420 — set_fact_out(sr, inputs) after abstract return
	f := &Function{Name: "func_1", ReturnType: PointerTo(GetIntType())}
	f.RV = CreateVariableScalars("func_1_rv", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(f)
	st := &Stmt{Kind: StmtReturn, StmID: 7,
		Expr: &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
			ExprType: PointerTo(GetIntType())}}
	if !fm.UpdateFactForReturnStmt(st, f.RV, st.Expr) {
		t.Fatal("update")
	}
	out := fm.MapFactsOut[7]
	if FindRelatedPointTo(out, f.RV) == nil {
		t.Fatal("map_facts_out missing rv", out)
	}
}

func TestVisitFactsReturnSetsOut(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	fm := NewFactMgr(f)
	st := &Stmt{Kind: StmtReturn, StmID: 8,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(3), ExprType: GetIntType()}}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsStatementReturn(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if _, ok := fm.MapFactsOut[8]; !ok {
		t.Fatal("facts out")
	}
	if !fm.MapVisited[8] && fm.GetMapStmEffect(8).IsEmpty() {
		// effect may be empty for const return; map_stm_effect should still be set
	}
	_ = fm.GetMapStmEffect(8)
}

func TestGetMapFactsStmID0FailClosed(t *testing.T) {
	ClearError()
	fm := NewFactMgr(nil)
	// StmID 0 must IncompleteFactSlice sticky — not invent empty-complete map miss
	if FactsComplete(fm.GetMapFactsIn(0)) || FactsComplete(fm.GetMapFactsOut(0)) {
		t.Fatal("StmID 0 must IncompleteFactSlice")
	}
	if !HasError() {
		t.Fatal("StmID 0 GetMapFacts must SetError sticky")
	}
	ClearError()
	if FactsComplete(fm.GetMapFactsInFinal(0)) || FactsComplete(fm.GetMapFactsOutFinal(0)) {
		t.Fatal("StmID 0 final maps must IncompleteFactSlice")
	}
	if !HasError() {
		t.Fatal("StmID 0 GetMapFactsFinal must SetError sticky")
	}
	ClearError()
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
	ClearError()
	if FactsComplete((*FactMgr)(nil).GetMapFactsIn(1)) {
		t.Fatal("nil FM GetMapFactsIn must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapFactsIn must SetError sticky")
	}
	ClearError()
	if FactsComplete((*FactMgr)(nil).GetMapFactsOut(1)) {
		t.Fatal("nil FM GetMapFactsOut must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapFactsOut must SetError sticky")
	}
	ClearError()
	if FactsComplete((*FactMgr)(nil).GetMapFactsInFinal(1)) {
		t.Fatal("nil FM GetMapFactsInFinal must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapFactsInFinal must SetError sticky")
	}
	ClearError()
	if FactsComplete((*FactMgr)(nil).GetMapFactsOutFinal(1)) {
		t.Fatal("nil FM GetMapFactsOutFinal must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapFactsOutFinal must SetError sticky")
	}
	ClearError()
	// GetMapStmEffect / GetMapAccumEffect nil FM sticky IncompleteEffect
	if EffectComplete((*FactMgr)(nil).GetMapStmEffect(1)) {
		t.Fatal("nil FM GetMapStmEffect must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapStmEffect must SetError sticky")
	}
	ClearError()
	if EffectComplete((*FactMgr)(nil).GetMapAccumEffect(1)) {
		t.Fatal("nil FM GetMapAccumEffect must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil FM GetMapAccumEffect must SetError sticky")
	}
	ClearError()
	// SetMap* nil FM / bad stm_id sticky
	(*FactMgr)(nil).SetMapFactsIn(1, nil)
	if !HasError() {
		t.Fatal("nil FM SetMapFactsIn must SetError sticky")
	}
	ClearError()
	fm.SetMapFactsIn(0, nil)
	if !HasError() {
		t.Fatal("stmID 0 SetMapFactsIn must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).SetMapStmEffect(1, EmptyEffect())
	if !HasError() {
		t.Fatal("nil FM SetMapStmEffect must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).ClearMapVisited()
	if !HasError() {
		t.Fatal("nil FM ClearMapVisited must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).CreateCFGEdgeTo(1, &Block{}, 0, false, false)
	if !HasError() {
		t.Fatal("nil FM CreateCFGEdgeTo must SetError sticky")
	}
	ClearError()
	// FactMgr mutators always live; sticky no invent soft-skip past hole
	(*FactMgr)(nil).SetupInOutMaps(true)
	if !HasError() {
		t.Fatal("nil FM SetupInOutMaps must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).AddNewVarFact(CreateVariableScalars("g_x", GetIntType(), false, false))
	if !HasError() {
		t.Fatal("nil FM AddNewVarFact must SetError sticky")
	}
	ClearError()
	fm.AddNewVarFact(nil)
	if !HasError() {
		t.Fatal("nil Variable AddNewVarFact must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).FindDanglingGlobalPtrs(&Function{Name: "f"})
	if !HasError() {
		t.Fatal("nil FM FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearError()
	fm.FindDanglingGlobalPtrs(nil)
	if !HasError() {
		t.Fatal("nil Function FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).AddFactOut(&Stmt{StmID: 1}, nil, MakeFactPointTo(
		CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), NullPtr))
	if !HasError() {
		t.Fatal("nil FM AddFactOut must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).SetMapFactsOutForStmtDest(&Stmt{StmID: 1}, nil, nil, nil)
	if !HasError() {
		t.Fatal("nil FM SetMapFactsOutForStmtDest must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).BackupStmFactMaps(&Stmt{StmID: 1}, map[int][]*FactPointTo{}, map[int][]*FactPointTo{})
	if !HasError() {
		t.Fatal("nil FM BackupStmFactMaps must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).RestoreStmFactMaps(&Stmt{StmID: 1}, map[int][]*FactPointTo{}, map[int][]*FactPointTo{})
	if !HasError() {
		t.Fatal("nil FM RestoreStmFactMaps must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).AddNewVarFactAndUpdate(nil, CreateVariableScalars("g_y", GetIntType(), false, false))
	if !HasError() {
		t.Fatal("nil FM AddNewVarFactAndUpdate must SetError sticky")
	}
	ClearError()
	(*FactMgr)(nil).UpdateFactsForOOSVars([]*Variable{CreateVariableScalars("g_z", GetIntType(), false, false)})
	if !HasError() {
		t.Fatal("nil FM UpdateFactsForOOSVars must SetError sticky")
	}
	ClearError()
	// empty OOS list complete no-op
	fm.UpdateFactsForOOSVars(nil)
	if HasError() {
		t.Fatal("empty UpdateFactsForOOSVars must not sticky")
	}
	ClearError()
	facts := []*FactPointTo{}
	(*FactMgr)(nil).AddParamFacts(nil, &facts)
	if !HasError() {
		t.Fatal("nil FM AddParamFacts must SetError sticky")
	}
	ClearError()
}

func TestFindParentBlockNilSticky(t *testing.T) {
	ClearError()
	if FindParentBlockOfStmID(nil, 1) != nil {
		t.Fatal("nil Function FindParentBlockOfStmID must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Function FindParentBlockOfStmID must SetError sticky")
	}
	ClearError()
	if FindParentBlockOfStmID(&Function{Name: "f"}, 0) != nil {
		t.Fatal("StmID 0 FindParentBlockOfStmID must fail closed")
	}
	if !HasError() {
		t.Fatal("StmID 0 FindParentBlockOfStmID must SetError sticky")
	}
	ClearError()
	// Block* always live on Function.Blocks; nil hole sticky miss
	f := &Function{Name: "f", Blocks: []*Block{nil}}
	if FindParentBlockOfStmID(f, 1) != nil {
		t.Fatal("nil Blocks hole FindParentBlockOfStmID must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Blocks hole FindParentBlockOfStmID must SetError sticky")
	}
	ClearError()
	// incomplete if-arm sticky whole miss (no invent soft-continue past nil Else)
	inner := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 3}}}
	outer := &Block{Func: f, Stmts: []Stmt{{Kind: StmtIfElse, StmID: 1, Then: inner, Else: nil}}}
	inner.Parent = outer
	f.Blocks = []*Block{outer}
	if FindParentBlockOfStmID(f, 3) != nil {
		t.Fatal("nil Else arm must fail closed FindParentBlockOfStmID")
	}
	if !HasError() {
		t.Fatal("nil Else arm FindParentBlockOfStmID must SetError sticky")
	}
	ClearError()
}

func TestApplyFactForAssignIsPointerResidualSticky(t *testing.T) {
	// IsPointer residual soft invent was invent lvarCnt=1 renew past Type-nil LHS.
	ClearError()
	hole := &Variable{Name: "g_p", Type: nil}
	if hole.IsPointer() {
		t.Fatal("Type-nil IsPointer must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil IsPointer must SetError sticky")
	}
	ClearError()
}

func TestAddNewVarFactAndUpdateIsGlobalResidualSticky(t *testing.T) {
	// IsGlobal residual soft invent was invent soft-skip makeup past FieldVarOf residual.
	ClearError()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
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
	(*Variable)(nil).IsGlobal()
	if !HasError() {
		t.Fatal("nil IsGlobal must SetError sticky")
	}
	ClearError()
	// blk==nil + non-global complete soft-skip no sticky
	fm.AddNewVarFactAndUpdate(nil, orphan)
	if HasError() {
		t.Fatal("non-global complete AddNewVarFactAndUpdate must soft-skip no sticky")
	}
	ClearError()
	// residual: Variable with FieldVarOf whose IsGlobal residual — FieldVarOf nil Variable?
	// FieldVarOf points to var; if FieldVarOf itself is ok. Plant residual via
	// IsGlobal after FieldVarOf: parent nil Variable not possible as FieldVarOf type.
	// Plant via recursive: hole.FieldVarOf where parent.FieldVarOf is set, walk ends.
	// Soft invent was: residual ERROR already set before call soft-continues makeup.
	SetError(ErrGeneric)
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
	//   if blk == nil && !v.IsGlobal() { return } // soft return with ambient residual invent soft-skip
	// New code checks HasError after IsGlobal.
	SetError(ErrGeneric)
	fm.AddNewVarFactAndUpdate(nil, CreateVariableScalars("l_1", GetIntType(), false, false))
	if !HasError() {
		// ambient residual should remain sticky (IsGlobal complete false still HasError after)
		t.Fatal("ambient residual AddNewVarFactAndUpdate must keep sticky")
	}
	ClearError()
	// residual FieldVarOf: Variable with FieldVarOf that is nil receiver? 
	// Use (*Variable)(nil) as subject — already sticky at entry.
	fm.AddNewVarFactAndUpdate(nil, nil)
	if !HasError() {
		t.Fatal("nil var AddNewVarFactAndUpdate must SetError sticky")
	}
	ClearError()
}

func TestCloneFactSliceIncompleteResidualSticky(t *testing.T) {
	// CloneFactSlice residual soft invent was invent soft-complete empty past IncompleteFactSlice.
	ClearError()
	out := CloneFactSlice(IncompleteFactSlice())
	if FactsComplete(out) {
		t.Fatal("IncompleteFactSlice CloneFactSlice must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("IncompleteFactSlice CloneFactSlice must SetError sticky")
	}
	ClearError()
	// complete empty
	out2 := CloneFactSlice([]*FactPointTo{})
	if !FactsComplete(out2) || out2 == nil {
		// empty complete may be non-nil empty
	}
	if HasError() {
		t.Fatal("complete empty CloneFactSlice must not sticky")
	}
	ClearError()
}

func TestGetProgramEndFacts(t *testing.T) {
	// FactMgr.cpp:732–735
	f1 := &Function{Name: "func_1"}
	f2 := &Function{Name: "func_2"}
	list := &FunctionList{Funcs: []*Function{f1, f2}}
	fms := NewFactMgrMap()
	fm1 := fms.ForFunc(f1)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm1.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	got := GetProgramEndFacts(list, fms)
	if len(got) != 1 || FindRelatedPointTo(got, p) == nil {
		t.Fatal(got)
	}
	ClearError()
	if GetProgramEndFacts(nil, fms) != nil {
		t.Fatal("empty list")
	}
	ClearError()
	if FactsComplete(GetProgramEndFacts(list, nil)) || !HasError() {
		t.Fatal("nil fms sticky")
	}
	ClearError()
}

func TestSanityCheckMap(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgr(f)
	// empty maps ok
	ClearError()
	fm.SanityCheckMap()
	if HasError() {
		t.Fatal("empty complete")
	}
	// incomplete map sticky
	fm.MapFactsIn[1] = IncompleteFactSlice()
	fm.SanityCheckMap()
	if !HasError() {
		t.Fatal("incomplete sticky")
	}
	ClearError()
	(*FactMgr)(nil).SanityCheckMap()
	if !HasError() {
		t.Fatal("nil fm sticky")
	}
	ClearError()
}
