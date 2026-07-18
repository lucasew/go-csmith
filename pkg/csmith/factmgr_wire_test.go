package csmith

import "testing"

func TestMakeFirstCreatesFactMgr(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	m := NewFactMgrMap()
	list := FunctionList{}
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
}

func TestAbstractFactForVarInitPointerArrayAlts(t *testing.T) {
	// array of pointers with alt init "0" → null
	parent := &ArrayVariable{
		Variable: Variable{
			Name: "g_ap", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2},
			Init: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		},
		Sizes:      []int{2},
		InitValues: []string{"0"},
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
