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
	if HasError() {
		t.Fatal("complete EqualsInt fold must not sticky")
	}
	ClearError()
	// EqualsInt residual soft invent was soft-continue invent fold true later.
	// Fair: sticky false. incomplete arg residual.
	fiHole := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{nil}}
	if fiHole.EqualsInt(0) {
		t.Fatal("EqualsInt residual must fail closed false")
	}
	if !HasError() {
		t.Fatal("EqualsInt residual must SetError sticky")
	}
	ClearError()
	// binary residual soft invent was soft-continue invent *0 fold past a0 residual.
	// Fair: sticky false.
	holeArg := &Expression{Term: TermConstant, Con: &Constant{Type: nil, Value: "0"}}
	fiHole2 := &Invocation{IsStd: true, Binary: "*", Args: []*Expression{holeArg, one}}
	if fiHole2.EqualsInt(0) {
		t.Fatal("binary EqualsInt residual must fail closed false")
	}
	if !HasError() {
		t.Fatal("binary EqualsInt residual must SetError sticky")
	}
	ClearError()
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
	// FunctionInvocationUnary.cpp:117 — assert invalid operator sticky; no invent eInt
	ClearError()
	arg := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "??", Args: []*Expression{arg}}
	if fi.GetType() != nil {
		t.Fatal("invalid unary op must fail closed")
	}
	if !HasError() {
		t.Fatal("invalid unary op must SetError sticky")
	}
	// valid not
	ClearError()
	fi.Unary = "!"
	if fi.GetType() != GetIntType() {
		t.Fatal("!")
	}
	// empty args for minus sticky
	ClearError()
	fi.Unary = "-"
	fi.Args = nil
	if fi.GetType() != nil {
		t.Fatal("empty args")
	}
	if !HasError() {
		t.Fatal("empty args unary GetType must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomAssignRequiresFactMgr(t *testing.T) {
	// StatementAssign.cpp:127 assert(fm) — nullptr empty (stmtOK false; StmtAssign is iota 0)
	opts := Defaults()
	c := EmptyCGContext()
	st := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType())
	if stmtOK(st) || st.LhsVar != nil || st.Expr != nil {
		t.Fatalf("nil FM must fail closed empty assign, got %#v", st)
	}
}

func TestMakeRandomAssignNoInventWithoutRNG(t *testing.T) {
	// StatementAssign.cpp always has RNG; sticky no invent assign shell
	ClearError()
	opts := Defaults()
	opts.CompoundAssignment = false
	f := &Function{Name: "f", ReturnType: GetIntType()}
	c := EmptyCGContext().WithFactMgr(NewFactMgr(f))
	st := MakeRandomAssign(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &c, GetIntType())
	if stmtOK(st) || st.LhsVar != nil || st.Expr != nil {
		t.Fatalf("nil RNG must fail closed empty assign, got %#v", st)
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomAssign must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomBinaryUnaryInvocationNoInventWithoutRNG(t *testing.T) {
	// FunctionInvocation.cpp always has RNG sticky; no invent empty-op std invoke shells
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	cg := EmptyCGContext()
	if fi := MakeRandomBinaryInvocation(nil, opts, probs, vs, tables, &cg, GetIntType()); fi != nil {
		t.Fatal("nil RNG binary")
	}
	if !HasError() {
		t.Fatal("nil RNG binary must SetError sticky")
	}
	ClearError()
	if fi := MakeRandomUnaryInvocation(nil, opts, vs, tables, &cg, GetIntType()); fi != nil {
		t.Fatal("nil RNG unary")
	}
	if !HasError() {
		t.Fatal("nil RNG unary must SetError sticky")
	}
	ClearError()
	if fi := MakeRandomInvocation(nil, opts, probs, vs, tables, &cg, nil, GetIntType(), nil, true); fi == nil || !fi.Failed {
		// MakeRandomInvocation may Failed shell without sticky when rng nil early
		if fi != nil && fi.Failed {
			// ok
		} else if fi != nil {
			t.Fatal("nil RNG invoke must fail closed")
		}
	}
	ClearError()
}

func TestMakeRandomUnaryInvocationIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent unary / soft re-pick past holes)
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect())
	cg.EffectAccum = &inc
	if fi := MakeRandomUnaryInvocation(NewRng(1), opts, vs, NewExprTables(opts), &cg, GetIntType()); fi != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomUnaryInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	cg2 := WithFunc(f, IncompleteEffect())
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if fi := MakeRandomUnaryInvocation(NewRng(2), opts, vs, NewExprTables(opts), &cg2, GetIntType()); fi != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomUnaryInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
	// incomplete GlobalFacts must fail closed before operand gen
	fm := NewFactMgr(f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg3 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if fi := MakeRandomUnaryInvocation(NewRng(3), opts, vs, NewExprTables(opts), &cg3, GetIntType()); fi != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomUnaryInvocation")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
}

func TestCreateSafeTmpsIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient must not invent tmp shells
	ClearError()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	flags := MakeDummyFlags()
	cg := WithFunc(f, IncompleteEffect())
	if t1, t2 := createBinarySafeTmps(cg, NewVariableSelector(Defaults()), flags, BinAdd); t1 != "" || t2 != "" {
		t.Fatalf("incomplete EffectContext must fail closed createBinarySafeTmps, got %q %q", t1, t2)
	}
	if !HasError() {
		t.Fatal("createBinarySafeTmps incomplete ambient must SetError sticky")
	}
	ClearError()
	// shift always needs flags_to_type(op2); sticky no invent type1 stand-in for type2.
	// Signed float LHS is simple; unsigned float RHS fails FlagsToType (assert path).
	badShift := &SafeOpFlags{Op1Signed: true, Op2Signed: false, IsFunc: true, Size: SafeFloat}
	f2 := &Function{Name: "f2", ReturnType: GetIntType()}
	blk2 := &Block{Func: f2}
	f2.Stack = []*Block{blk2}
	cg2 := WithFunc(f2, EmptyEffect())
	if t1, t2 := createBinarySafeTmps(cg2, NewVariableSelector(Defaults()), badShift, BinLShift); t1 != "" || t2 != "" {
		t.Fatalf("bad shift RHS type must fail closed, got %q %q", t1, t2)
	}
	if !HasError() {
		t.Fatal("shift without simple RHS type must SetError sticky")
	}
	ClearError()
	if tmp := createUnarySafeTmp(cg, NewVariableSelector(Defaults()), flags); tmp != "" {
		t.Fatalf("incomplete EffectContext must fail closed createUnarySafeTmp, got %q", tmp)
	}
	if !HasError() {
		t.Fatal("createUnarySafeTmp incomplete ambient must SetError sticky")
	}
	ClearError()
}

func TestShiftByNonConstantProbNoInventHardcoded50(t *testing.T) {
	// FunctionInvocation.cpp:238 — ShiftByNonConstantProb(); 0% must always take constant RHS
	// (no invent hard-coded RndFlipcoin(50) ignoring session table)
	opts := Defaults()
	probs := NewProbabilities(opts)
	probs.single[PShiftByNonConstantProb] = 0
	prev := ProcessProbabilities()
	SetProcessProbabilities(probs)
	defer SetProcessProbabilities(prev)
	vs := NewVariableSelector(opts)
	vs.Probs = probs
	tables := NewExprTables(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	foundShift := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearError()
		cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		fi := MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, tables, &cg, GetIntType())
		if fi == nil || !fi.IsStd {
			continue
		}
		if fi.Binary != "<<" && fi.Binary != ">>" {
			continue
		}
		foundShift = true
		if len(fi.Args) < 2 || fi.Args[1] == nil {
			t.Fatalf("shift missing rhs seed=%d", seed)
		}
		if fi.Args[1].Term != TermConstant {
			t.Fatalf("ShiftByNonConstantProb=0 must invent constant rhs, got term=%v seed=%d",
				fi.Args[1].Term, seed)
		}
	}
	if !foundShift {
		t.Log("no shift op in seeds 1..199 — still covered by nil-probs 0% unit path via Single")
	}
	// nil probs + nil process → 0% non-constant (no invent 50)
	SetProcessProbabilities(nil)
	for seed := uint64(1); seed < 80; seed++ {
		ClearError()
		cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		fi := MakeRandomBinaryInvocation(NewRng(seed), opts, nil, vs, tables, &cg, GetIntType())
		if fi == nil || (fi.Binary != "<<" && fi.Binary != ">>") {
			continue
		}
		if len(fi.Args) >= 2 && fi.Args[1] != nil && fi.Args[1].Term != TermConstant {
			t.Fatalf("nil probs must not invent 50%% non-constant shift rhs seed=%d", seed)
		}
	}
}

func TestVisitFactsBinaryOrderedIncompleteFailClosed(t *testing.T) {
	// sticky on nil / short args (no invent visit / soft re-pick)
	ClearError()
	cg := EmptyCGContext()
	if VisitFactsBinaryOrdered(nil, &cg, Defaults()) {
		t.Fatal("nil fi")
	}
	if !HasError() {
		t.Fatal("nil fi VisitFactsBinaryOrdered must SetError sticky")
	}
	ClearError()
	if VisitFactsBinaryOrdered(&Invocation{IsStd: true, Binary: "&&"}, &cg, Defaults()) {
		t.Fatal("short args")
	}
	if !HasError() {
		t.Fatal("short args VisitFactsBinaryOrdered must SetError sticky")
	}
	ClearError()
	// nil param_value hole sticky
	left := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if VisitFactsBinaryOrdered(&Invocation{
		IsStd: true, Binary: "&&",
		Args: []*Expression{nil, left},
	}, &cg, Defaults()) {
		t.Fatal("nil left arg must fail closed")
	}
	if !HasError() {
		t.Fatal("nil left arg must SetError sticky")
	}
	ClearError()
	if VisitFactsBinaryOrdered(&Invocation{
		IsStd: true, Binary: "&&",
		Args: []*Expression{left, nil},
	}, &cg, Defaults()) {
		t.Fatal("nil right arg must fail closed")
	}
	if !HasError() {
		t.Fatal("nil right arg must SetError sticky")
	}
	ClearError()
}

func TestVisitFactsBinaryOrderedPostMergeIncompleteFailClosed(t *testing.T) {
	// incomplete GlobalFacts before left snapshot sticky
	ClearError()
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fi := &Invocation{IsStd: true, Binary: "&&", Args: []*Expression{
		{Term: TermConstant, Con: MakeInt(1)},
		{Term: TermConstant, Con: MakeInt(0)},
	}}
	if VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts before left snapshot must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts binary visit must SetError sticky")
	}
	ClearError()
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
	ClearError()
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
	if HasError() {
		t.Fatal("complete SafeInvocation must not sticky")
	}
	ClearError()
	if (*Invocation)(nil).SafeInvocation() {
		t.Fatal("nil SafeInvocation must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil SafeInvocation must SetError sticky")
	}
	ClearError()
}

func TestInvocationCompatibleVarUnary(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	ev := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{ev}}
	if !fi.CompatibleVar(v, false) {
		t.Fatal("self")
	}
	if fi.CompatibleVar(CreateVariableScalars("g_2", GetIntType(), true, false), false) {
		t.Fatal("other")
	}
	// binary never compatible (complete false, not sticky)
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ev}}
	if fi2.CompatibleVar(v, false) {
		t.Fatal("binary")
	}
	if HasError() {
		t.Fatal("complete CompatibleVar paths must not sticky")
	}
	// incomplete unary operand sticky
	ClearError()
	if (&Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: nil}).CompatibleVar(v, false) {
		t.Fatal("missing unary arg must fail closed")
	}
	if !HasError() {
		t.Fatal("missing unary arg CompatibleVar must SetError sticky")
	}
	ClearError()
	// incomplete EqualsInt args sticky
	if (&Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: nil}).EqualsInt(0) {
		t.Fatal("missing unary arg EqualsInt must fail closed false")
	}
	if !HasError() {
		t.Fatal("missing unary arg EqualsInt must SetError sticky")
	}
	ClearError()
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
	// sticky no soft invent signed=true → eInt when operands missing
	ClearError()
	fi := &Invocation{IsStd: true, Binary: "+"}
	if fi.GetType() != nil {
		t.Fatal("add without args must not invent eInt")
	}
	if !HasError() {
		t.Fatal("add without args must SetError sticky")
	}
	ClearError()
	fi.Args = []*Expression{{Term: TermConstant, Con: MakeInt(1)}}
	if fi.GetType() != nil {
		t.Fatal("add with one arg")
	}
	if !HasError() {
		t.Fatal("add with one arg must SetError sticky")
	}
	// shift needs left only sticky
	ClearError()
	fi = &Invocation{IsStd: true, Binary: "<<"}
	if fi.GetType() != nil {
		t.Fatal("shift without left must not invent eInt")
	}
	if !HasError() {
		t.Fatal("shift without left must SetError sticky")
	}
	// cmp still returns int without consulting operands (C++ switch arm)
	ClearError()
	fi = &Invocation{IsStd: true, Binary: "=="}
	if fi.GetType() != GetIntType() {
		t.Fatal("cmp")
	}
	ClearError()
}

func TestInvocationGetTypeNilSticky(t *testing.T) {
	ClearError()
	if (*Invocation)(nil).GetType() != nil {
		t.Fatal("nil Invocation GetType must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Invocation GetType must SetError sticky")
	}
	ClearError()
}

func TestInvocationGetTypeArgResidualSticky(t *testing.T) {
	// GetType residual soft invent was invent eInt/eUInt past incomplete arg type shell.
	ClearError()
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "1"}}
	good := &Expression{Term: TermConstant, Con: MakeInt(2)}
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{hole, good}}
	if fi.GetType() != nil {
		t.Fatal("GetType residual must fail closed nil type, not invent eInt")
	}
	if !HasError() {
		t.Fatal("GetType residual binary must SetError sticky")
	}
	ClearError()
	// unary operand residual
	fiU := &Invocation{IsStd: true, IsUnary: true, Unary: "+", Args: []*Expression{hole}}
	if fiU.GetType() != nil {
		t.Fatal("GetType residual unary must fail closed nil type")
	}
	if !HasError() {
		t.Fatal("GetType residual unary must SetError sticky")
	}
	ClearError()
	// shift left residual
	fiS := &Invocation{IsStd: true, Binary: "<<", Args: []*Expression{hole, good}}
	if fiS.GetType() != nil {
		t.Fatal("GetType residual shift must fail closed nil type")
	}
	if !HasError() {
		t.Fatal("GetType residual shift must SetError sticky")
	}
	ClearError()
}

func TestInvocationOutputArgResidualSticky(t *testing.T) {
	// Arg Output residual soft invent was soft-continue later args invent partial call.
	ClearError()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fi := &Invocation{
		User: f,
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: &Constant{Value: "2"}}, // Type-nil residual
		},
	}
	if s := fi.Output(); s != "" {
		t.Fatal("arg Output residual must fail closed Invocation.Output", s)
	}
	if !HasError() {
		t.Fatal("arg Output residual Invocation.Output must SetError sticky")
	}
	ClearError()
}

func TestInvocationBinaryOutputResidualSticky(t *testing.T) {
	// a0/a1 Output residual soft invent was invent binary past incomplete arg.
	ClearError()
	fi := &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: &Constant{Value: "2"}}, // Type-nil residual
		},
	}
	if s := fi.Output(); s != "" {
		t.Fatal("a1 Output residual must fail closed binary Output", s)
	}
	if !HasError() {
		t.Fatal("a1 Output residual binary Output must SetError sticky")
	}
	ClearError()
}
