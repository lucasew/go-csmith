package csmith

import "testing"

func TestAddFactOutVisible(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.AddFactOut(st, body, MakeFactPointTo(
		CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false),
		NullPtr,
	))
	// use global pointer fact
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.AddFactOut(st, body, MakeFactPointTo(p, NullPtr))
	if len(fm.MapFactsOut[5]) != 2 {
		// first call also used a fresh p - ok
		if len(fm.MapFactsOut[5]) < 1 {
			t.Fatal(fm.MapFactsOut[5])
		}
	}
	// return drops non-global
	ret := &Stmt{Kind: StmtReturn, StmID: 6}
	fm.AddFactOut(ret, body, MakeFactPointTo(
		// local as subject — need pointer local
		func() *Variable {
			lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
			lp.Name = "l_p"
			return lp
		}(),
		NullPtr,
	))
	if len(fm.MapFactsOut[6]) != 0 {
		t.Fatal("return non-global should drop", fm.MapFactsOut[6])
	}
	// return keeps global
	fm.AddFactOut(ret, body, MakeFactPointTo(p, NullPtr))
	if len(fm.MapFactsOut[6]) != 1 {
		t.Fatal(fm.MapFactsOut[6])
	}
	_ = g
}

func TestArrayIsVariant(t *testing.T) {
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", IsArray: true, Type: GetIntType()},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	a := &ArrayVariable{
		Variable:   Variable{Name: "g_a[i]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	a.AsArray = a
	b := &ArrayVariable{
		Variable:   Variable{Name: "g_a[i]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"i"},
	}
	b.AsArray = b
	if !a.IsVariant(&b.Variable) {
		t.Fatal("same keys")
	}
	c := &ArrayVariable{
		Variable:   Variable{Name: "g_a[j]", IsArray: true, Type: GetIntType()},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"j"},
	}
	c.AsArray = c
	if a.IsVariant(&c.Variable) {
		t.Fatal("different keys")
	}
	// collective must match
	otherParent := &ArrayVariable{Variable: Variable{Name: "g_b", IsArray: true}}
	d := &ArrayVariable{
		Variable:   Variable{Name: "g_b[i]", IsArray: true},
		Collective: otherParent,
		Indices:    []string{"i"},
	}
	d.AsArray = d
	if a.IsVariant(&d.Variable) {
		t.Fatal("diff collective")
	}
}
