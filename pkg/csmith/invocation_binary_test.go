package csmith

import "testing"

func TestBinaryOpFromString(t *testing.T) {
	op, ok := BinaryOpFromString("&&")
	if !ok || op != BinAnd {
		t.Fatal(op, ok)
	}
	if GetBinopString(BinAdd) != "+" {
		t.Fatal(GetBinopString(BinAdd))
	}
}

func TestInvocationIs0Or1(t *testing.T) {
	fi := &Invocation{IsStd: true, Binary: ">", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(0)},
	}}
	if !fi.Is0Or1() {
		t.Fatal("cmp")
	}
	fi.Binary = "+"
	if fi.Is0Or1() {
		t.Fatal("add")
	}
}

func TestInvocationEqualsIntFold(t *testing.T) {
	zero := &Expression{Term: TermConstant, Con: MakeInt(0)}
	one := &Expression{Term: TermConstant, Con: MakeInt(1)}
	// 0 * x
	fi := &Invocation{IsStd: true, Binary: "*", Args: []*Expression{zero, one}}
	if !fi.EqualsInt(0) {
		t.Fatal("0*")
	}
	// x * 0
	fi = &Invocation{IsStd: true, Binary: "*", Args: []*Expression{one, zero}}
	if !fi.EqualsInt(0) {
		t.Fatal("*0")
	}
	// same expr subtract
	v := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_1", GetIntType(), true, false)}
	fi = &Invocation{IsStd: true, Binary: "-", Args: []*Expression{v, v}}
	if !fi.EqualsInt(0) {
		t.Fatal("a-a")
	}
	// x % 1
	fi = &Invocation{IsStd: true, Binary: "%", Args: []*Expression{one, one}}
	if !fi.EqualsInt(0) {
		t.Fatal("%1")
	}
	// Expression.EqualsInt through funcall
	e := &Expression{Term: TermFunction, Invoke: &Invocation{IsStd: true, Binary: "*", Args: []*Expression{zero, one}}}
	if !e.EqualsInt(0) {
		t.Fatal("expr fold")
	}
}

func TestVisitFactsBinaryOrderedMerges(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	// start with a
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a)}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	// && with constants — both visit ok, merge keeps a
	fi := &Invocation{IsStd: true, Binary: "&&", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(0)},
	}}
	if !VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		t.Fatal("facts kept")
	}
	// through VisitFactsInvocation
	fi2 := &Invocation{IsStd: true, Binary: "||", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(0)},
		{Term: TermConstant, Con: MakeInt(1)},
	}}
	if !VisitFactsInvocation(fi2, &cg, Defaults()) {
		t.Fatal("dispatch")
	}
	_ = b
}

func TestSafeOpsBinaryMatches(t *testing.T) {
	if !SafeOpsBinary("+") || SafeOpsBinary("&&") {
		t.Fatal("safe ops set")
	}
}
