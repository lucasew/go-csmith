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

func TestUnaryGetTypeInvalidOpFailClosed(t *testing.T) {
	// FunctionInvocationUnary.cpp:117 — assert invalid operator; no invent eInt
	arg := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "??", Args: []*Expression{arg}}
	if fi.GetType() != nil {
		t.Fatal("invalid unary op must fail closed")
	}
	// valid not
	fi.Unary = "!"
	if fi.GetType() != GetIntType() {
		t.Fatal("!")
	}
	// empty args for minus
	fi.Unary = "-"
	fi.Args = nil
	if fi.GetType() != nil {
		t.Fatal("empty args")
	}
}

func TestMakeRandomAssignRequiresFactMgr(t *testing.T) {
	// StatementAssign.cpp:127 assert(fm)
	opts := Defaults()
	c := EmptyCGContext()
	st := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType())
	if st.LhsVar != nil || st.Expr != nil {
		t.Fatal("nil FM must fail closed empty assign")
	}
}

func TestVisitFactsBinaryOrderedIncompleteFailClosed(t *testing.T) {
	// no soft invent visit success on nil / short args
	cg := EmptyCGContext()
	if VisitFactsBinaryOrdered(nil, &cg, Defaults()) {
		t.Fatal("nil fi")
	}
	if VisitFactsBinaryOrdered(&Invocation{IsStd: true, Binary: "&&"}, &cg, Defaults()) {
		t.Fatal("short args")
	}
}

func TestSafeOpsBinaryMatches(t *testing.T) {
	if !SafeOpsBinary("+") || SafeOpsBinary("&&") {
		t.Fatal("safe ops set")
	}
}

func TestInvocationGetTypeUnary(t *testing.T) {
	// FunctionInvocationUnary.cpp:120–128
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: GetSimpleType(ELongLong), Value: "7"}}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{arg}}
	if fi.GetType() != GetSimpleType(ELongLong) {
		t.Fatalf("minus type %v", fi.GetType())
	}
	fi.Unary = "!"
	if fi.GetType() != GetIntType() {
		t.Fatal("not → int")
	}
}

func TestInvocationGetTypeBinary(t *testing.T) {
	// cmp → int
	fi := &Invocation{IsStd: true, Binary: ">", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(0)},
	}}
	if fi.GetType() != GetIntType() {
		t.Fatal("cmp")
	}
	// float size flags → float
	fi = &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(2)},
		},
		Safe: &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeFloat},
	}
	if !fi.IsReturnTypeFloat() || fi.GetType() != GetSimpleType(EFloat) {
		t.Fatal("float ret")
	}
	// both unsigned → uint
	u := GetSimpleType(EUInt)
	cu := &Constant{Type: u, Value: "1"}
	fi = &Invocation{IsStd: true, Binary: "+", Args: []*Expression{
		{Term: TermConstant, Con: cu},
		{Term: TermConstant, Con: cu},
	}}
	if fi.GetType() != GetSimpleType(EUInt) {
		t.Fatalf("unsigned add %v", fi.GetType())
	}
	// shift follows left
	fi = &Invocation{IsStd: true, Binary: "<<", Args: []*Expression{
		{Term: TermConstant, Con: cu},
		{Term: TermConstant, Con: MakeInt(1)},
	}}
	if fi.GetType() != GetSimpleType(EUInt) {
		t.Fatal("ushift")
	}
}

func TestInvocationSafeInvocation(t *testing.T) {
	// Unary: minus unsafe; others safe; binary always unsafe; user always safe
	if (&Invocation{IsStd: true, IsUnary: true, Unary: "-"}).SafeInvocation() {
		t.Fatal("minus")
	}
	if !(&Invocation{IsStd: true, IsUnary: true, Unary: "!"}).SafeInvocation() {
		t.Fatal("not")
	}
	if (&Invocation{IsStd: true, Binary: "+"}).SafeInvocation() {
		t.Fatal("binary")
	}
	if !(&Invocation{User: &Function{Name: "f"}}).SafeInvocation() {
		t.Fatal("user")
	}
}

func TestInvocationCompatibleVarUnary(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	ev := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{ev}}
	if !fi.CompatibleVar(v, false) {
		t.Fatal("self")
	}
	if fi.CompatibleVar(CreateVariableScalars("g_2", GetIntType(), true, false), false) {
		t.Fatal("other")
	}
	// binary never compatible
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ev}}
	if fi2.CompatibleVar(v, false) {
		t.Fatal("binary")
	}
}

func TestExpressionFuncallGetTypeViaInvoke(t *testing.T) {
	// ExpressionFuncall.cpp:122–124
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: GetSimpleType(EShort), Value: "1"}}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "+", Args: []*Expression{arg}}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if e.GetType() != GetSimpleType(EShort) {
		t.Fatalf("got %v", e.GetType())
	}
	// user return type
	e2 := &Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "f", ReturnType: GetSimpleType(EFloat)}}}
	if e2.GetType() != GetSimpleType(EFloat) {
		t.Fatal("user")
	}
}

func TestBinaryGetTypeMissingArgsFailClosed(t *testing.T) {
	// FunctionInvocationBinary.cpp:208–209 — param_value[0/1]->get_type always
	// no soft invent signed=true → eInt when operands missing
	fi := &Invocation{IsStd: true, Binary: "+"}
	if fi.GetType() != nil {
		t.Fatal("add without args must not invent eInt")
	}
	fi.Args = []*Expression{{Term: TermConstant, Con: MakeInt(1)}}
	if fi.GetType() != nil {
		t.Fatal("add with one arg")
	}
	// shift needs left only
	fi = &Invocation{IsStd: true, Binary: "<<"}
	if fi.GetType() != nil {
		t.Fatal("shift without left must not invent eInt")
	}
	// cmp still returns int without consulting operands (C++ switch arm)
	fi = &Invocation{IsStd: true, Binary: "=="}
	if fi.GetType() != GetIntType() {
		t.Fatal("cmp")
	}
}
