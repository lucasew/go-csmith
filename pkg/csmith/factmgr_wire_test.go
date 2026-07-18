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
