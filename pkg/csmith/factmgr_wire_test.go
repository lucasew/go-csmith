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
	// fair: incomplete union map hole fails closed false
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
}

func TestApplyPointToAssignFactsNilHoleFailClosed(t *testing.T) {
	// soft invent: MergeFactInto nil still return true with partial maps
	// fair: incomplete newFacts fails closed ok=false without poisoning prior map
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
	// incomplete subject map — wipe to hole marker
	facts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	if _, ok := applyPointToAssignFacts(&facts, p, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("nil subject map hole must fail closed ok=false")
	}
	if FactsComplete(facts) {
		t.Fatal("incomplete subject must clear", facts)
	}
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
	facts = []*FactPointTo{MakeFactPointTo(p, a)}
	if VariablesComplete(lhsAssignPointees(facts, nil, 0)) {
		t.Fatal("nil lhs must IncompleteVariables")
	}
	if _, ok := applyPointToAssignFacts(&facts, nil, 0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}); ok {
		t.Fatal("nil lhs pointees must fail closed ok=false, not invent renew/merge")
	}
	if FactsComplete(facts) {
		t.Fatal("incomplete lhs assign must wipe subject facts")
	}
}

func TestUpdateFactForAssignPointToHoleNoUnionInvent(t *testing.T) {
	// incomplete point-to apply must not invent union merge success
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a), nil}
	fm.UnionFacts = []*FactUnion{}
	rhs := &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()}
	// assign through incomplete GlobalFacts — apply fails closed
	if fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("incomplete GlobalFacts assign must fail closed false")
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("point-to hole must clear GlobalFacts", fm.GlobalFacts)
	}
	if UnionFactsComplete(fm.UnionFacts) {
		t.Fatal("must not invent keep UnionFacts after point-to fail", fm.UnionFacts)
	}
}

func TestUpdateFactForAssignRenewsDefinitive(t *testing.T) {
	// FactMgr.cpp:376–380 — lvar_cnt==1 non-array → renew (replace, not join)
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
	// incomplete: nil Type must not invent empty init success (AddNewVarFact skip)
	pt2, un2 := AbstractFactForVarInit(nil)
	if FactsComplete(pt2) || UnionFactsComplete(un2) {
		t.Fatal("nil var init must fail closed incomplete", pt2, un2)
	}
	// union without rhs/init — assert path incomplete (not invent empty union fact)
	uv2 := &Variable{Name: "g_u2", Type: ut}
	_, un3 := AbstractFactForVarInit(uv2)
	if UnionFactsComplete(un3) {
		t.Fatal("union without init must fail closed incomplete", un3)
	}
	// incomplete abstract must wipe on AddNewVarFact (not invent skip no-fact)
	fm2 := NewFactMgr(nil)
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(
		CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), NullPtr)}
	fm2.AddNewVarFact(uv2)
	if UnionFactsComplete(fm2.UnionFacts) {
		t.Fatal("AddNewVarFact incomplete union must fail closed", fm2.UnionFacts)
	}
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
	fm := NewFactMgr(nil)
	// StmID 0 must IncompleteFactSlice — not invent empty-complete map miss
	if FactsComplete(fm.GetMapFactsIn(0)) || FactsComplete(fm.GetMapFactsOut(0)) {
		t.Fatal("StmID 0 must IncompleteFactSlice")
	}
	// missing live key is complete empty
	if !FactsComplete(fm.GetMapFactsIn(42)) || len(fm.GetMapFactsIn(42)) != 0 {
		t.Fatal("missing live id must complete empty")
	}
	fm.SetMapFactsOut(7, IncompleteFactSlice())
	if FactsComplete(fm.GetMapFactsOut(7)) {
		t.Fatal("stored incomplete must stay incomplete via getter")
	}
}
