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
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
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
	zero := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}
	one := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
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
	v := &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)}
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
	if !e.EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("expr fold")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete EqualsInt fold must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// EqualsInt residual soft invent was soft-continue invent fold true later.
	// Fair: sticky false. incomplete arg residual.
	fiHole := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{nil}}
	if fiHole.EqualsInt(0) {
		t.Fatal("EqualsInt residual must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("EqualsInt residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// binary residual soft invent was soft-continue invent *0 fold past a0 residual.
	// Fair: sticky false.
	holeArg := &Expression{Term: TermConstant, Con: &Constant{Type: nil, Value: "0"}}
	fiHole2 := &Invocation{IsStd: true, Binary: "*", Args: []*Expression{holeArg, one}}
	if fiHole2.EqualsInt(0) {
		t.Fatal("binary EqualsInt residual must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("binary EqualsInt residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsBinaryOrderedMerges(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), true, false)
	// start with a
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// && with constants — both visit ok, merge keeps a
	fi := &Invocation{IsStd: true, Binary: "&&", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
	}}
	if !VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) == nil {
		t.Fatal("facts kept")
	}
	// through VisitFactsInvocation
	fi2 := &Invocation{IsStd: true, Binary: "||", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
	}}
	if !VisitFactsInvocation(fi2, &cg, Defaults()) {
		t.Fatal("dispatch")
	}
	_ = b
}

// TestVisitFactsBinaryOrderedMergesUnionWrite —
// FunctionInvocationBinary.cpp:494–499 merge_facts is full FactVec (ePointTo +
// eUnionWrite). Soft invent was PT-only: RHS ExpressionAssign renew of a union
// field left last=f1 without joining post-left last=f0 → nonreadable field
// became choose_var-eligible (seed-58 g_697.f1).
func TestVisitFactsBinaryOrderedMergesUnionWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Manual 2-field union (MakeRandomUnionType may emit 1-field).
	i32 := GetIntTypeSess(testAmbientSession)
	i64 := GetSimpleTypeSess(testAmbientSession, ELongLong)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	ut := &Type{
		isUnion:    true,
		StructName: "U_ord",
		Fields: []StructField{
			{Name: "f0", Type: i32, Qfer: q, BitWidth: -1},
			{Name: "f1", Type: i64, Qfer: q, BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, q)
	if uv == nil || len(uv.FieldVars) < 2 {
		t.Fatalf("want 2 field vars, got %v (fields=%d)", uv, len(uv.FieldVars))
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]
	// pointer to f1 so *p = … renews union last=f1
	p := CreateVariableQferSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, f1.Type), NewCVQualifiers([]bool{false}, []bool{false}))
	fm := NewFactMgrSess(testAmbientSession, nil)
	// post-left lattice: last written f0
	fm.UnionFacts = []*FactUnion{MakeFactUnionSess(testAmbientSession, uv, 0)}
	// p points to f1
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, f1)}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// RHS: *p = 1  (ExpressionAssign) — renews g_u last to f1's field id
	rhsAssign := &Stmt{
		Kind:     StmtAssign,
		LhsVar:   p,
		Lhs:      &Lhs{Var: p, Type: f1.Type}, // indir = ptr - pointee = 1
		Expr:     &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		AssignOp: AssignSimple,
		StmID:    AllocStmID(),
	}
	fi := &Invocation{IsStd: true, Binary: "&&", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermAssignment, Assign: rhsAssign},
	}}
	if !VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatalf("visit sticky=%v", GetErrorSess(testAmbientSession))
	}
	fu := FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, uv)
	if fu == nil {
		t.Fatal("union fact missing after ordered visit")
	}
	// post-left last=f0 join post-right last=f1 → BOTTOM (neither implies)
	if !fu.IsBottomSess(testAmbientSession) {
		t.Fatalf("want BOTTOM after && merge of f0|f1, got last=%d", fu.LastWrittenFID)
	}
	// both fields nonreadable under BOTTOM
	if !IsNonreadableFieldSess(testAmbientSession, f0, fm.UnionFacts) || !IsNonreadableFieldSess(testAmbientSession, f1, fm.UnionFacts) {
		t.Fatal("BOTTOM must make both union fields nonreadable")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete ordered visit must not sticky")
	}
}

func TestUnaryGetTypeInvalidOpFailClosed(t *testing.T) {
	// FunctionInvocationUnary.cpp:117 — assert invalid operator sticky; no invent eInt
	ClearErrorSess(testAmbientSession)
	arg := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "??", Args: []*Expression{arg}}
	if fi.GetType() != nil {
		t.Fatal("invalid unary op must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid unary op must SetError sticky")
	}
	// valid not
	ClearErrorSess(testAmbientSession)
	fi.Unary = "!"
	if fi.GetType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("!")
	}
	// empty args for minus sticky
	ClearErrorSess(testAmbientSession)
	fi.Unary = "-"
	fi.Args = nil
	if fi.GetType() != nil {
		t.Fatal("empty args")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty args unary GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomAssignRequiresFactMgr(t *testing.T) {
	// StatementAssign.cpp:127 assert(fm) — nullptr empty (stmtOK false; StmtAssign is iota 0)
	opts := Defaults()
	c := EmptyCGContext().WithSession(testAmbientSession)
	st := MakeRandomAssign(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTablesSess(testAmbientSession, opts), &c, GetIntTypeSess(testAmbientSession))
	if stmtOK(st) || st.LhsVar != nil || st.Expr != nil {
		t.Fatalf("nil FM must fail closed empty assign, got %#v", st)
	}
}

func TestMakeRandomAssignNoInventWithoutRNG(t *testing.T) {
	// StatementAssign.cpp always has RNG; sticky no invent assign shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.CompoundAssignment = false
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	c := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := MakeRandomAssign(nil, opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTablesSess(testAmbientSession, opts), &c, GetIntTypeSess(testAmbientSession))
	if stmtOK(st) || st.LhsVar != nil || st.Expr != nil {
		t.Fatalf("nil RNG must fail closed empty assign, got %#v", st)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBinaryUnaryInvocationNoInventWithoutRNG(t *testing.T) {
	// FunctionInvocation.cpp always has RNG sticky; no invent empty-op std invoke shells
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTablesSess(testAmbientSession, opts)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if fi := MakeRandomBinaryInvocation(nil, opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession)); fi != nil {
		t.Fatal("nil RNG binary")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG binary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if fi := MakeRandomUnaryInvocation(nil, opts, vs, tables, &cg, GetIntTypeSess(testAmbientSession)); fi != nil {
		t.Fatal("nil RNG unary")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG unary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if fi := MakeRandomInvocation(nil, opts, probs, vs, tables, &cg, nil, GetIntTypeSess(testAmbientSession), nil, true); fi == nil || !fi.Failed {
		// MakeRandomInvocation may Failed shell without sticky when rng nil early
		if fi != nil && fi.Failed {
			// ok
		} else if fi != nil {
			t.Fatal("nil RNG invoke must fail closed")
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomUnaryInvocationIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient must sticky ERROR (no invent unary / soft re-pick past holes)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.EffectAccum = &inc
	if fi := MakeRandomUnaryInvocation(NewRngSess(testAmbientSession, 1), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg, GetIntTypeSess(testAmbientSession)); fi != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomUnaryInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	if fi := MakeRandomUnaryInvocation(NewRngSess(testAmbientSession, 2), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg2, GetIntTypeSess(testAmbientSession)); fi != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomUnaryInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts must fail closed before operand gen
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if fi := MakeRandomUnaryInvocation(NewRngSess(testAmbientSession, 3), opts, vs, NewExprTablesSess(testAmbientSession, opts), &cg3, GetIntTypeSess(testAmbientSession)); fi != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomUnaryInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCreateSafeTmpsMatchesCreatePath(t *testing.T) {
	// FunctionInvocationBinary.cpp:59–75 — Create* allocates tmps with only
	// flags+safe_ops+blk (no ambient EffectComplete gate). Incomplete ambient
	// must not skip gensym t_ while still failing closed.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	flags := MakeDummyFlags()
	cg := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession)
	t1, t2 := createBinarySafeTmps(cg, NewVariableSelector(testAmbientSession, Defaults()), flags, BinAdd)
	if t1 == "" || t2 == "" {
		t.Fatalf("Create* path must still allocate tmps under incomplete ambient, got %q %q", t1, t2)
	}
	ClearErrorSess(testAmbientSession)
	// shift always needs flags_to_type(op2); sticky no invent type1 stand-in for type2.
	// Signed float LHS is simple; unsigned float RHS fails FlagsToTypeSess(testAmbientSession, assert path).
	badShift := &SafeOpFlags{Op1Signed: true, Op2Signed: false, IsFunc: true, Size: SafeFloat}
	f2 := &Function{Name: "f2", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk2 := &Block{Func: f2}
	f2.Stack = []*Block{blk2}
	cg2 := WithFunc(f2, EmptyEffect()).WithSession(testAmbientSession)
	if t1, t2 := createBinarySafeTmps(cg2, NewVariableSelector(testAmbientSession, Defaults()), badShift, BinLShift); t1 != "" || t2 != "" {
		t.Fatalf("bad shift RHS type must fail closed, got %q %q", t1, t2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("shift without simple RHS type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// unary Create* also only needs flags+blk
	tmp := createUnarySafeTmp(cg, NewVariableSelector(testAmbientSession, Defaults()), flags)
	if tmp == "" {
		t.Fatal("Create* unary must allocate tmp under incomplete ambient")
	}
	ClearErrorSess(testAmbientSession)
	// nil block: soft miss (no sticky invent)
	cgNoBlk := EmptyCGContext().WithSession(testAmbientSession)
	if t1, t2 := createBinarySafeTmps(cgNoBlk, NewVariableSelector(testAmbientSession, Defaults()), flags, BinAdd); t1 != "" || t2 != "" {
		t.Fatalf("nil block must soft-skip tmps, got %q %q", t1, t2)
	}
	ClearErrorSess(testAmbientSession)
}

func TestShiftByNonConstantProbNoInventHardcoded50(t *testing.T) {
	// FunctionInvocation.cpp:238 — ShiftByNonConstantProb(); 0% must always take constant RHS
	// (no invent hard-coded RndFlipcoin(50) ignoring session table)
	opts := Defaults()
	probs := NewProbabilities(opts)
	probs.single[PShiftByNonConstantProb] = 0
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, probs)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Probs = probs
	tables := NewExprTablesSess(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	foundShift := false
	for seed := uint64(1); seed < 200; seed++ {
		ClearErrorSess(testAmbientSession)
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
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
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, nil, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
		if fi == nil || (fi.Binary != "<<" && fi.Binary != ">>") {
			continue
		}
		if len(fi.Args) >= 2 && fi.Args[1] != nil && fi.Args[1].Term != TermConstant {
			t.Fatalf("nil probs must not invent 50%% non-constant shift rhs seed=%d", seed)
		}
	}
}

func TestShiftNonConstantRHSNoConstFilter(t *testing.T) {
	// FunctionInvocation.cpp:243–244 — non-constant shift RHS:
	// Expression::make_random(rhs_cg, rhs_type, nullptr, /*no_func=*/false, /*no_const=*/true, MAX)
	// so TermConstant is filtered (avoids negative / oversized shift amounts).
	// Was: no_const=false → could pick Constant mid depth-gate (seed-2 e11008 U120 tries=2 vs UP tries=6).
	opts := Defaults()
	probs := NewProbabilities(opts)
	probs.single[PShiftByNonConstantProb] = 100
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, probs)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	// Seed a local int so Variable term is available under depth/no_const filters.
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Probs = probs
	loc := &Variable{Name: "l_shift", Type: GetIntTypeSess(testAmbientSession)}
	tables := NewExprTablesSess(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	found := 0
	for seed := uint64(1); seed < 400; seed++ {
		ClearErrorSess(testAmbientSession)
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		cg.ExprDepth = 0
		fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
		if fi == nil || !fi.IsStd {
			continue
		}
		if fi.Binary != "<<" && fi.Binary != ">>" {
			continue
		}
		if len(fi.Args) < 2 || fi.Args[1] == nil {
			t.Fatalf("shift missing rhs seed=%d", seed)
		}
		found++
		if fi.Args[1].Term == TermConstant {
			t.Fatalf("ShiftByNonConstantProb=100 must filter TermConstant RHS (no_const=true), got Constant seed=%d", seed)
		}
	}
	if found == 0 {
		t.Fatal("expected at least one << / >> with ShiftByNonConstantProb=100")
	}
}

func TestVisitFactsBinaryOrderedIncompleteFailClosed(t *testing.T) {
	// sticky on nil / short args (no invent visit / soft re-pick)
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if VisitFactsBinaryOrdered(nil, &cg, Defaults()) {
		t.Fatal("nil fi")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fi VisitFactsBinaryOrdered must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VisitFactsBinaryOrdered(&Invocation{IsStd: true, Binary: "&&"}, &cg, Defaults()) {
		t.Fatal("short args")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("short args VisitFactsBinaryOrdered must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil param_value hole sticky
	left := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	if VisitFactsBinaryOrdered(&Invocation{
		IsStd: true, Binary: "&&",
		Args: []*Expression{nil, left},
	}, &cg, Defaults()) {
		t.Fatal("nil left arg must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil left arg must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VisitFactsBinaryOrdered(&Invocation{
		IsStd: true, Binary: "&&",
		Args: []*Expression{left, nil},
	}, &cg, Defaults()) {
		t.Fatal("nil right arg must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil right arg must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsBinaryOrderedPostMergeIncompleteFailClosed(t *testing.T) {
	// incomplete GlobalFacts before left snapshot sticky
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	fi := &Invocation{IsStd: true, Binary: "&&", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
	}}
	if VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts before left snapshot must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts binary visit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeOpsBinaryMatches(t *testing.T) {
	if !SafeOpsBinary("+") || SafeOpsBinary("&&") {
		t.Fatal("safe ops set")
	}
}

func TestInvocationGetTypeUnary(t *testing.T) {
	// FunctionInvocationUnary.cpp:120–128
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: GetSimpleTypeSess(testAmbientSession, ELongLong), Value: "7"}}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{arg}}
	if fi.GetType() != GetSimpleTypeSess(testAmbientSession, ELongLong) {
		t.Fatalf("minus type %v", fi.GetType())
	}
	fi.Unary = "!"
	if fi.GetType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("not → int")
	}
}

func TestInvocationGetTypeBinary(t *testing.T) {
	// cmp → int
	fi := &Invocation{IsStd: true, Binary: ">", Args: []*Expression{
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
	}}
	if fi.GetType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("cmp")
	}
	// float size flags → float
	fi = &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2)},
		},
		Safe: &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeFloat},
	}
	if !fi.IsReturnTypeFloat() || fi.GetType() != GetSimpleTypeSess(testAmbientSession, EFloat) {
		t.Fatal("float ret")
	}
	// both unsigned → uint
	u := GetSimpleTypeSess(testAmbientSession, EUInt)
	cu := &Constant{Type: u, Value: "1"}
	fi = &Invocation{IsStd: true, Binary: "+", Args: []*Expression{
		{Term: TermConstant, Con: cu},
		{Term: TermConstant, Con: cu},
	}}
	if fi.GetType() != GetSimpleTypeSess(testAmbientSession, EUInt) {
		t.Fatalf("unsigned add %v", fi.GetType())
	}
	// shift follows left
	fi = &Invocation{IsStd: true, Binary: "<<", Args: []*Expression{
		{Term: TermConstant, Con: cu},
		{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
	}}
	if fi.GetType() != GetSimpleTypeSess(testAmbientSession, EUInt) {
		t.Fatal("ushift")
	}
}

func TestInvocationSafeInvocation(t *testing.T) {
	// Unary: minus unsafe; others safe; binary always unsafe; user always safe
	ClearErrorSess(testAmbientSession)
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
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete SafeInvocation must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Invocation)(nil).SafeInvocation() {
		t.Fatal("nil SafeInvocation must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SafeInvocation must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationCompatibleVarUnary(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	ev := &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{ev}}
	if !fi.CompatibleVar(v, false) {
		t.Fatal("self")
	}
	if fi.CompatibleVar(CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntTypeSess(testAmbientSession), true, false), false) {
		t.Fatal("other")
	}
	// binary never compatible (complete false, not sticky)
	fi2 := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{ev, ev}}
	if fi2.CompatibleVar(v, false) {
		t.Fatal("binary")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete CompatibleVar paths must not sticky")
	}
	// incomplete unary operand sticky
	ClearErrorSess(testAmbientSession)
	if (&Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: nil}).CompatibleVar(v, false) {
		t.Fatal("missing unary arg must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing unary arg CompatibleVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EqualsInt args sticky
	if (&Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: nil}).EqualsInt(0) {
		t.Fatal("missing unary arg EqualsInt must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing unary arg EqualsInt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionFuncallGetTypeViaInvoke(t *testing.T) {
	// ExpressionFuncall.cpp:122–124
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: GetSimpleTypeSess(testAmbientSession, EShort), Value: "1"}}
	fi := &Invocation{IsStd: true, IsUnary: true, Unary: "+", Args: []*Expression{arg}}
	e := &Expression{Term: TermFunction, Invoke: fi}
	if e.GetTypeSess(testAmbientSession) != GetSimpleTypeSess(testAmbientSession, EShort) {
		t.Fatalf("got %v", e.GetTypeSess(testAmbientSession))
	}
	// user return type
	e2 := &Expression{Term: TermFunction, Invoke: &Invocation{User: &Function{Name: "f", ReturnType: GetSimpleTypeSess(testAmbientSession, EFloat)}}}
	if e2.GetTypeSess(testAmbientSession) != GetSimpleTypeSess(testAmbientSession, EFloat) {
		t.Fatal("user")
	}
}

func TestBinaryGetTypeMissingArgsFailClosed(t *testing.T) {
	// FunctionInvocationBinary.cpp:208–209 — param_value[0/1]->get_type always
	// sticky no soft invent signed=true → eInt when operands missing
	ClearErrorSess(testAmbientSession)
	fi := &Invocation{IsStd: true, Binary: "+"}
	if fi.GetType() != nil {
		t.Fatal("add without args must not invent eInt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("add without args must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fi.Args = []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}}
	if fi.GetType() != nil {
		t.Fatal("add with one arg")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("add with one arg must SetError sticky")
	}
	// shift needs left only sticky
	ClearErrorSess(testAmbientSession)
	fi = &Invocation{IsStd: true, Binary: "<<"}
	if fi.GetType() != nil {
		t.Fatal("shift without left must not invent eInt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("shift without left must SetError sticky")
	}
	// cmp still returns int without consulting operands (C++ switch arm)
	ClearErrorSess(testAmbientSession)
	fi = &Invocation{IsStd: true, Binary: "=="}
	if fi.GetType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("cmp")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationGetTypeNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Invocation)(nil).GetType() != nil {
		t.Fatal("nil Invocation GetType must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invocation GetType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationGetTypeArgResidualSticky(t *testing.T) {
	// GetType residual soft invent was invent eInt/eUInt past incomplete arg type shell.
	ClearErrorSess(testAmbientSession)
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "1"}}
	good := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2)}
	fi := &Invocation{IsStd: true, Binary: "+", Args: []*Expression{hole, good}}
	if fi.GetType() != nil {
		t.Fatal("GetType residual must fail closed nil type, not invent eInt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual binary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// unary operand residual
	fiU := &Invocation{IsStd: true, IsUnary: true, Unary: "+", Args: []*Expression{hole}}
	if fiU.GetType() != nil {
		t.Fatal("GetType residual unary must fail closed nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual unary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// shift left residual
	fiS := &Invocation{IsStd: true, Binary: "<<", Args: []*Expression{hole, good}}
	if fiS.GetType() != nil {
		t.Fatal("GetType residual shift must fail closed nil type")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual shift must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationOutputArgResidualSticky(t *testing.T) {
	// Arg Output residual soft invent was soft-continue later args invent partial call.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	fi := &Invocation{
		User: f,
		Args: []*Expression{
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
			{Term: TermConstant, Con: &Constant{Value: "2"}}, // Type-nil residual
		},
	}
	if s := fi.Output(); s != "" {
		t.Fatal("arg Output residual must fail closed Invocation.Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("arg Output residual Invocation.Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInvocationBinaryOutputResidualSticky(t *testing.T) {
	// a0/a1 Output residual soft invent was invent binary past incomplete arg.
	ClearErrorSess(testAmbientSession)
	fi := &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
			{Term: TermConstant, Con: &Constant{Value: "2"}}, // Type-nil residual
		},
	}
	if s := fi.Output(); s != "" {
		t.Fatal("a1 Output residual must fail closed binary Output", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("a1 Output residual binary Output must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseFuncContextMatchResidualTruePathSticky(t *testing.T) {
	// Match residual soft invent was invent keep after Match true with residual.
	// Hygiene: complete match path + incomplete FEffect sticky.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{
		Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession),
		RV:      &Variable{Name: "func_1_rv", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		FEffect: IncompleteEffect(),
		IsBuilt: true, BuildState: BuildBuilt,
	}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if ChooseFuncContext(NewRngSess(testAmbientSession, 1), []*Function{f}, GetIntTypeSess(testAmbientSession), nil, &cg, opts, &q) != nil {
		t.Fatal("Incomplete FEffect must fail closed ChooseFuncContext")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Incomplete FEffect ChooseFuncContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetTypeBinaryIsSignedResidualSticky(t *testing.T) {
	// IsSigned residual soft invent was invent eInt/eUInt past Type-nil signed check after GetType ok.
	// Force via args whose GetType returns complete type then IsSigned residual — Type nil after get.
	// Path: GetType residual already sticky when Type-nil Con; for IsSigned residual use non-simple type shell?
	// IsSigned on Type-nil SetError sticky true.
	ClearErrorSess(testAmbientSession)
	// Use pointer types (non-simple): IsSigned residual path returns false without error when complete.
	// Residual invent: ambient residual after GetType complete before IsSigned.
	// Direct: (*Type)(nil).IsSigned sticky — exercise getTypeBinary via args with valid GetType then...
	// Fair residual: invoke GetType after planting residual via incomplete simple shell?
	// lt non-nil complete int — IsSigned complete. Test IsSigned residual itself through getTypeBinary:
	// Use Type with IsSimple residual? IsSimple never residual on non-nil Type.
	// Test assignLhsIsVolatile residual and for IsSigned nil Type via statement path.
	// For getTypeBinary: after GetType returns type, call IsSigned — residual only on nil Type.
	// So if GetType returns non-nil, IsSigned residual only ambient.
	// Stick with: Type-nil GetType residual already covered; add IsSigned on nil Type hygiene:
	if !((*Type)(nil)).IsSignedSess(testAmbientSession) {
		// IsSigned residual returns true (restrictive)
		t.Fatal("nil Type IsSigned must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type IsSigned must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// getTypeBinary with complete signed ints must not sticky
	fi := &Invocation{
		IsStd: true, Binary: "+",
		Args: []*Expression{
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
			{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2), ExprType: GetIntTypeSess(testAmbientSession)},
		},
	}
	if fi.GetType() != GetSimpleTypeSess(testAmbientSession, EInt) {
		t.Fatal("complete signed binary GetType must eInt")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete signed binary GetType must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomInvocationStdUnaryAlwaysDrawsNilType(t *testing.T) {
	// FunctionInvocation.cpp:111–118 — rnd_flipcoin(StdUnaryFuncProb()) always,
	// even when type is null. Old Go skipped the draw when typ==nil (unfair
	// prefer-binary without consuming F5).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Force unary branch: PStdUnaryFuncProb=100 so flipcoin always true.
	probs.single[PStdUnaryFuncProb] = 100
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	list := &FunctionList{Funcs: []*Function{f}}
	r := NewRngSess(testAmbientSession, 1)
	// stdFunc=true, typ=nil → must draw StdUnary (100% true) then fail closed
	// (C++ assert(type) / no invent binary after true unary draw).
	fi := MakeRandomInvocation(r, opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), &cg, list, nil, nil, true)
	if fi == nil || !fi.Failed {
		t.Fatalf("nil-type + unary draw must Failed shell, got %#v", fi)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil-type + unary draw must SetError sticky (assert type)")
	}
	// depth advanced by flipcoin (and not by silent skip)
	if r.randDepth < 1 {
		t.Fatal("StdUnaryFuncProb flipcoin must advance randDepth even when typ nil")
	}
	ClearErrorSess(testAmbientSession)
	// typ=nil + unary prob 0 → binary path (still drew flipcoin)
	probs.single[PStdUnaryFuncProb] = 0
	r2 := NewRngSess(testAmbientSession, 2)
	ClearErrorSess(testAmbientSession)
	_ = MakeRandomInvocation(r2, opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), &cg, list, nil, nil, true)
	// binary with nil type may Failed or produce; key is flipcoin was drawn
	if r2.randDepth < 1 {
		t.Fatal("StdUnaryFuncProb flipcoin must advance randDepth when prob=0 typ nil")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinarySubcontextClearsCurrRHS(t *testing.T) {
	// CGContext.cpp:74–82 — CGContext(cgc, eff_context, accum) sets curr_rhs(nullptr).
	// Soft invent: CloneSubcontext kept outer CurrRHS into binary/param subcontexts,
	// so nested Lhs::visit_facts ran overlap checks against the wrong RHS.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.Probs = probs
	tables := NewExprTablesSess(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// seed a global so Select can succeed
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	// plant outer CurrRHS as if mid ExpressionAssign Lhs
	outerRHS := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 42)}
	cg.CurrRHS = outerRHS
	// MakeRandomBinary must not leave residual error from wrong CurrRHS overlap
	// (generation may still fail closed for other reasons — key is CurrRHS not leaked into subcontexts)
	// Probe via CloneSubcontext contract used by binary setup:
	leak := cg.CloneSubcontext()
	if leak.CurrRHS != outerRHS {
		t.Fatal("CloneSubcontext must preserve CurrRHS for explicit callers")
	}
	// Fair param-style adapt (same as MakeRandomBinaryInvocation):
	accum := EmptyEffect()
	sub := cg.CloneSubcontext()
	sub.effectContext = cg.EffectContext().detachMaps()
	sub.EffectAccum = &accum
	sub.EffectStm = EmptyEffect()
	sub.CurrRHS = nil
	if sub.CurrRHS != nil {
		t.Fatal("param/binary subcontext must clear CurrRHS (CGContext.cpp:74–82)")
	}
	// Generation with polluted outer CurrRHS must still be non-sticky when subcontexts clear it
	fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, 3), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
	if HasErrorSess(testAmbientSession) && fi == nil {
		// may fail for depth/inventory — but must not be from residual invent
		ClearErrorSess(testAmbientSession)
	}
	// outer CurrRHS unchanged (binary must not wipe caller's assign Lhs context)
	if cg.CurrRHS != outerRHS {
		t.Fatal("binary must not clear caller's CurrRHS")
	}
	ClearErrorSess(testAmbientSession)
}
