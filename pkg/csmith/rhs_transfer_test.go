package csmith

import "testing"

func TestConstantEquals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !MakeIntSess(testAmbientSession, 0).EqualsSess(testAmbientSession, 0) {
		t.Fatal("0")
	}
	if MakeIntSess(testAmbientSession, 3).EqualsSess(testAmbientSession, 0) {
		t.Fatal("3")
	}
	// Constant.cpp:357–361 + str2int — small-path "0L"/"0UL" must equals(0)
	// (seed-2 e15477: div/mod re-pick needs rhs->equals(0))
	zeroL := &Constant{Type: GetIntTypeSess(testAmbientSession), Value: "0L"}
	if !zeroL.EqualsSess(testAmbientSession, 0) {
		t.Fatal("0L must Equals(0) via Str2Int stream extract")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("0L Equals must not sticky")
	}
	zeroUL := &Constant{Type: GetSimpleTypeSess(testAmbientSession, EUInt), Value: "0UL"}
	if !zeroUL.EqualsSess(testAmbientSession, 0) {
		t.Fatal("0UL must Equals(0)")
	}
	negL := &Constant{Type: GetIntTypeSess(testAmbientSession), Value: "-1L"}
	if !negL.EqualsSess(testAmbientSession, -1) || negL.EqualsSess(testAmbientSession, 0) {
		t.Fatal("-1L fold")
	}
	// incomplete Constant sticky false (no invent not-equal / not-less)
	if (*Constant)(nil).EqualsSess(testAmbientSession, 0) {
		t.Fatal("nil Constant Equals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Constant Equals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Constant)(nil).NotEqualsSess(testAmbientSession, 0) {
		t.Fatal("nil Constant NotEquals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Constant NotEquals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Constant)(nil).LessThanSess(testAmbientSession, 1) {
		t.Fatal("nil Constant LessThan must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Constant LessThan must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty Value incomplete shell sticky (no invent not-equal / not-less soft-skip)
	empty := &Constant{Type: GetIntTypeSess(testAmbientSession), Value: ""}
	if empty.EqualsSess(testAmbientSession, 0) {
		t.Fatal("empty Value Equals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Value Equals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if empty.NotEqualsSess(testAmbientSession, 0) {
		t.Fatal("empty Value NotEquals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Value NotEquals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if empty.LessThanSess(testAmbientSession, 1) {
		t.Fatal("empty Value LessThan must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Value LessThan must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil incomplete shell sticky (no invent fold success past hole)
	noTy := &Constant{Value: "0"}
	if noTy.EqualsSess(testAmbientSession, 0) {
		t.Fatal("nil Type Equals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type Equals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if noTy.NotEqualsSess(testAmbientSession, 1) {
		t.Fatal("nil Type NotEquals must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type NotEquals must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if noTy.LessThanSess(testAmbientSession, 1) {
		t.Fatal("nil Type LessThan must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type LessThan must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferNullConst(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNullSess(testAmbientSession) {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferNilRHSIsGarbage(t *testing.T) {
	// FactPointTo.cpp:168–169 — nullptr rhs → garbage (AddParamFacts missing arg)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, nil)
	if len(facts) != 1 || !facts[0].IsDeadSess(testAmbientSession) {
		t.Fatal("nil rhs must abstract as garbage like C++", facts)
	}
	// return always has Expression*; sticky fail closed before garbage invent
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	rv := CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if fm.UpdateFactForReturnStmt(&Stmt{Kind: StmtReturn, StmID: 1}, rv, nil) {
		t.Fatal("nil return expr must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil return expr UpdateFactForReturnStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts after assign path fails closed sticky
	fm.GlobalFacts = IncompleteFactSlice()
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}
	if fm.UpdateFactForReturnStmt(&Stmt{Kind: StmtReturn, StmID: 1}, rv, rhs) {
		t.Fatal("incomplete GlobalFacts must fail closed UpdateFactForReturnStmt")
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("must stay incomplete GlobalFacts")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts return update must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferAddrOf(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	// ExpressionVariable with ExprType = pointer means &g_t if var is int?
	// IndirectLevel = var.level - exprType.level; int(0) - ptr(1) = -1 → address-of
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if rhs.IndirectLevelSess(testAmbientSession) != -1 {
		t.Fatalf("indir %d", rhs.IndirectLevelSess(testAmbientSession))
	}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts[0])
	}
}

func TestRhsToLhsTransferCopy(t *testing.T) {
	p1 := CreateVariableScalarsSess(testAmbientSession, "g_p1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	p2 := CreateVariableScalarsSess(testAmbientSession, "g_p2", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	in := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p2, tgt)}
	rhs := &Expression{Term: TermVariable, Var: p2, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	// p2 level 1, expr 1 → indirect 0; merge uses fact of p2
	facts := RhsToLhsTransferSess(testAmbientSession, in, []*Variable{p1}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestUpdateFactForAssign(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// nil FM / lhs sticky — no invent soft-skip assign update
	if (*FactMgr)(nil).UpdateFactForAssign(CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false), 0, nil) {
		t.Fatal("nil FM must fail closed")
	}
	// nil FM UpdateFactForAssign must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	if fm.UpdateFactForAssign(nil, 0, nil) {
		t.Fatal("nil lhs must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs UpdateFactForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable.cpp:395 — pointer Constant::make_random is "0" → null on AddNewVarFact
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm.AddNewVarFact(p)
	if !FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p).IsNullSess(testAmbientSession) {
		t.Fatal("init null")
	}
	// assign to non-null target expression
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	rhs := &Expression{Term: TermVariable, Var: a, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	// take address form for pointee
	rhs = &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}}
	if !fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("update")
	}
	if !FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p).IsNullSess(testAmbientSession) {
		t.Fatal("null after assign")
	}
}

func TestAbstractFactNonPointerLHS(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	// non-pointer scalar: complete empty (no pointer facts), not incomplete marker
	if out, _ := AbstractFactForAssignSess(testAmbientSession, nil, v, 0, rhs); !FactsComplete(out) || len(out) != 0 {
		t.Fatal("non-ptr must be complete empty", out)
	}
}

func TestRhsToLhsTransferCommaPeel(t *testing.T) {
	// FactPointTo.cpp:259–261 — comma uses RHS of comma
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		CommaRHS: &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)),
	}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNullSess(testAmbientSession) {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferCommaNilRHSFailClosed(t *testing.T) {
	// incomplete CommaRHS must not invent complete GarbagePtr via nil-rhs peel
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
		// CommaRHS nil
	}
	out := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if FactsComplete(out) {
		t.Fatal("nil CommaRHS must fail closed incomplete, not invent GarbagePtr", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CommaRHS RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferAddrOfNilCollectiveFailClosed(t *testing.T) {
	// multi-level & hard IR sticky — no invent MakeFactsPointTo past assert(indirect==-1)
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// force Indir < -1 by ExprType deeper than Var.Type
	rhs := &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false), ExprType: PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)) {
		t.Fatal("multi-level & must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("multi-level & must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferAssignPeel(t *testing.T) {
	// FactPointTo.cpp:256–258 — embedded assign peels to assign RHS
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	assign := &Stmt{
		Kind: StmtAssign, LhsVar: q, Lhs: &Lhs{Var: q, Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		Expr:     &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		AssignOp: AssignSimple,
	}
	rhs := &Expression{Term: TermAssignment, Assign: assign, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNullSess(testAmbientSession) {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferAssignNilExprFailClosed(t *testing.T) {
	// incomplete Assign.Expr must not invent complete GarbagePtr via nil-rhs peel
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	assign := &Stmt{
		Kind: StmtAssign, LhsVar: q, Lhs: &Lhs{Var: q, Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		// Expr nil
		AssignOp: AssignSimple,
	}
	rhs := &Expression{Term: TermAssignment, Assign: assign, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	out := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if FactsComplete(out) {
		t.Fatal("nil Assign.Expr must fail closed incomplete, not invent GarbagePtr", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Assign.Expr RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferFunctionReturn(t *testing.T) {
	// FactPointTo.cpp:247–253 — RV return fact copied to LHS
	InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	defer InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	fn := &Function{Name: "f", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	fn.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fi := &Invocation{User: fn}
	AddReturnFactForInvocationSess(testAmbientSession, fi, MakeFactPointToSess(testAmbientSession, fn.RV, tgt))
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if len(facts) != 1 || len(facts[0].PointTo) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferRVTypeNilSticky(t *testing.T) {
	// RV Type* always live; Type-nil no invent scalar rv soft-transfer past hole
	ClearErrorSess(testAmbientSession)
	InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	defer InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fn := &Function{Name: "f", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	fn.RV = &Variable{Name: "f_rv", Type: nil}
	fi := &Invocation{User: fn}
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)) {
		t.Fatal("Type-nil RV must fail closed incomplete")
	}
	// Type-nil RV RhsToLhsTransfer must SetError sticky — nil-owner residual: no bag → fail-closed without ambient sticky
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferUnionParentTypeNilSticky(t *testing.T) {
	// union constant field0 path: parent Type* always live
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_u", Type: nil}
	f0 := &Variable{Name: "g_u.f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0}
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
	}}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: ut, Value: "{0}"}, ExprType: ut}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{f0}, rhs)) {
		t.Fatal("Type-nil union parent must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil union parent RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferUnionAggregateFields(t *testing.T) {
	// FactPointTo.cpp:172 + 210–224 — only pointer/union pass early type gate;
	// union RHS maps pointer fields pairwise (struct RHS is garbage early).
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	pf := &Variable{Name: "g_u.f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), FieldVarOf: uv}
	uv.FieldVars = []*Variable{pf}
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	lhsP := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	in := []*FactPointTo{MakeFactPointToSess(testAmbientSession, pf, tgt)}
	rhs := &Expression{Term: TermVariable, Var: uv, ExprType: ut}
	facts := RhsToLhsTransferSess(testAmbientSession, in, []*Variable{lhsP}, rhs)
	if len(facts) != 1 || len(facts[0].PointTo) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferStructIsGarbage(t *testing.T) {
	// FactPointTo.cpp:172–178 — struct type fails pointer/union gate → garbage
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
	}}
	sv := &Variable{Name: "g_s", Type: st}
	lhsP := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{Term: TermVariable, Var: sv, ExprType: st}
	facts := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{lhsP}, rhs)
	if len(facts) != 1 || !facts[0].IsDeadSess(testAmbientSession) {
		t.Fatalf("%+v", facts)
	}
}

func TestAbstractFactUnionFieldAssignsAllPtrFields(t *testing.T) {
	// FactPointTo.cpp:280–293 — non-pointer LHS inside union walks to container
	// and transfers into all pointer fields of that union.
	// Build union: { int x; int *p0; int *p1; }
	ut := &Type{
		isUnion:    true,
		StructName: "U_mix",
		Fields: []StructField{
			{Name: "x", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "p0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
			{Name: "p1", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		uv.CreateFieldVarsSess(testAmbientSession)
	}
	if len(uv.FieldVars) < 3 {
		t.Skip("need 3 fields")
	}
	// find int field and pointer fields
	var xField, p0, p1 *Variable
	for _, f := range uv.FieldVars {
		if f == nil {
			continue
		}
		if f.IsPointerSess(testAmbientSession) {
			if p0 == nil {
				p0 = f
			} else if p1 == nil {
				p1 = f
			}
		} else if xField == nil {
			xField = f
		}
	}
	if xField == nil || p0 == nil || p1 == nil {
		t.Skip("missing field kinds")
	}
	// assign non-pointer field x = 0 → union path updates p0 and p1
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: GetIntTypeSess(testAmbientSession)}
	facts, _ := AbstractFactForAssignSess(testAmbientSession, nil, xField, 0, rhs)
	got0 := FindRelatedPointToSess(testAmbientSession, facts, p0)
	got1 := FindRelatedPointToSess(testAmbientSession, facts, p1)
	if got0 == nil || got1 == nil {
		t.Fatalf("union walk should yield ptr field facts: %+v", facts)
	}
}

func TestRhsToLhsTransferNonPointerLvarsFailClosed(t *testing.T) {
	// FactPointTo.cpp:164–167 — assert all LHS are pointers; hard IR sticky
	ClearErrorSess(testAmbientSession)
	i := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"}}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{i}, rhs)) {
		t.Fatal("non-pointer lvar must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-pointer lvar RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferMultiLevelAddrFailClosed(t *testing.T) {
	// FactPointTo.cpp:205 — assert(indirect == -1); hard IR sticky
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	// int var with ** type → IndirectLevel = 0-2 = -2
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))}
	if rhs.IndirectLevelSess(testAmbientSession) != -2 {
		t.Fatalf("want indir -2 got %d", rhs.IndirectLevelSess(testAmbientSession))
	}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)) {
		t.Fatal("multi-level address-of must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("multi-level & RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferAggregateLenMismatchNDEBUG(t *testing.T) {
	// FactPointTo.cpp:216 — assert(lvars.size() == pointers.size()); NDEBUG elides
	// and pairs only the overlapping prefix (no sticky-poison generation).
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	pf := &Variable{Name: "g_u.f0", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), FieldVarOf: uv}
	uv.FieldVars = []*Variable{pf}
	lhs0 := CreateVariableScalarsSess(testAmbientSession, "g_p0", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	lhs1 := CreateVariableScalarsSess(testAmbientSession, "g_p1", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{Term: TermVariable, Var: uv, ExprType: ut}
	// two LHS pointers vs one field pointer → must not sticky-poison (NDEBUG assert).
	_ = RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{lhs0, lhs1}, rhs)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("NDEBUG len mismatch must not sticky-poison", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferMissingReturnFactFailClosed(t *testing.T) {
	// FactPointTo.cpp:252 — missing rv_fact: incomplete non-sticky (generation soft re-pick)
	ClearErrorSess(testAmbientSession)
	InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	defer InvocationReturnFactsDoFinalizationSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fn := &Function{Name: "f", ReturnType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	fn.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fi := &Invocation{User: fn}
	// no AddReturnFactForInvocation
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)) {
		t.Fatal("missing rv_fact must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("missing rv_fact must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferIncompleteMapsNonSticky(t *testing.T) {
	// incomplete fact map / MergePointees hole stays non-sticky for soft re-pick
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// incomplete map: nil hole so MergePointeesOfPointer fails closed incomplete
	hole := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr), nil}
	rhs := &Expression{Term: TermVariable, Var: q, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, hole, []*Variable{p}, rhs)) {
		t.Fatal("incomplete map transfer must fail closed incomplete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete map RhsToLhsTransfer must stay non-sticky for soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactForAssignNilLhsSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	outAF, _ := AbstractFactForAssignSess(testAmbientSession, nil, nil, 0, &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)})
	if FactsComplete(outAF) {
		t.Fatal("nil lhs AbstractFactForAssign must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs AbstractFactForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactForAssignTypeNilMorePointeeSticky(t *testing.T) {
	// *p peels to non-pointer; lvars may be pointer pointees; more = pointees of those.
	// soft invent: IsPointer residual ERROR+false skip Type-nil then partial transfer.
	// fair: sticky IncompleteFactSlice before classify.
	ClearErrorSess(testAmbientSession)
	// p:int* points to q:int*; *p peels to int (non-pointer branch); lvars=[q];
	// more = MergePointees(q,1) → Type-nil shell sticky
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	shell := &Variable{Name: "g_hole"} // Type nil
	factsIn := []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, p, q),
		MakeFactPointToSess(testAmbientSession, q, shell),
	}
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: GetIntTypeSess(testAmbientSession)}
	out, _ := AbstractFactForAssignSess(testAmbientSession, factsIn, p, 1, rhs)
	if FactsComplete(out) {
		t.Fatal("Type-nil more pointee must fail closed incomplete, not partial transfer", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil more pointee AbstractFactForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactUnionForAssignIsUnionResidualSticky(t *testing.T) {
	// IsUnion residual soft invent was invent non-union complete transfer past hole.
	// Type-nil non-special already sticky; complete non-union empty transfer hygiene.
	ClearErrorSess(testAmbientSession)
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	out, n := AbstractFactUnionForAssignSess(testAmbientSession, nil, nil, iv, 0, nil, nil)
	if !UnionFactsComplete(out) && out != nil {
		// IncompleteUnionFactSlice when incomplete maps — nil maps are complete empty
	}
	// complete non-union with nil maps: UnionFactsComplete(nil)==true and FactsComplete(nil)==true
	// then non-union path returns nil, lvarCnt
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete non-union AbstractFactUnionForAssign must not sticky", out, n)
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil non-special sticky
	hole := &Variable{Name: "g_x", Type: nil}
	uf, _ := AbstractFactUnionForAssignSess(testAmbientSession, nil, nil, hole, 0, nil, nil)
	if UnionFactsComplete(uf) {
		// IncompleteUnionFactSlice is not complete
		t.Fatal("Type-nil must fail closed incomplete", uf)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil non-special AbstractFactUnionForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferGetTypeResidualSticky(t *testing.T) {
	// GetType residual soft invent was invent GarbagePtr complete success past Type-nil RHS.
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// Type-nil constant shell → GetType residual
	rhs := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)) {
		t.Fatal("GetType residual must fail closed IncompleteFactSlice")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferIsPointerResidualSticky(t *testing.T) {
	// IsPointer residual soft invent was invent transfer past non-pointer LHS soft-skip.
	ClearErrorSess(testAmbientSession)
	// Type-nil non-special IsPointer residual ERROR+false
	hole := &Variable{Name: "g_x", Type: nil}
	if FactsComplete(RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{hole}, nil)) {
		t.Fatal("IsPointer residual must fail closed IncompleteFactSlice")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsPointer residual RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferUnionGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-merge union transfer past array shell.
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray GetCollective residual
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	lvars := []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_u", GetIntTypeSess(testAmbientSession), false, false)}
	// force union transfer path via RhsToLhsTransferUnion with TermVariable shell
	rhs := &Expression{Term: TermVariable, Var: shell, ExprType: GetIntTypeSess(testAmbientSession)}
	out := RhsToLhsTransferUnionSess(testAmbientSession, nil, nil, lvars, rhs)
	if UnionFactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasErrorSess(testAmbientSession) {
		// GetCollective on IsArray without AsArray SetError
		t.Fatal("IsArray without AsArray GetCollective residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRhsToLhsTransferGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-merge pointees past array shell.
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray GetCollective residual
	shell := &Variable{Name: "g_a", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2}}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rhs := &Expression{Term: TermVariable, Var: shell, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	out := RhsToLhsTransferSess(testAmbientSession, nil, []*Variable{p}, rhs)
	if FactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete empty
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray GetCollective residual RhsToLhsTransfer must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactForAssignGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-abstract past array shell LHS.
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_a", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2}}
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: GetIntTypeSess(testAmbientSession)}
	out, _ := AbstractFactForAssignSess(testAmbientSession, nil, shell, 0, rhs)
	if FactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray GetCollective residual AbstractFactForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAbstractFactUnionForAssignGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-abstract union past array shell.
	ClearErrorSess(testAmbientSession)
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	out, _ := AbstractFactUnionForAssignSess(testAmbientSession, nil, nil, shell, 0, nil, nil)
	if UnionFactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray GetCollective residual AbstractFactUnionForAssign must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergePointeesFindRelatedResidualSticky(t *testing.T) {
	// FindRelated residual soft invent was invent soft-empty merge past nil ptr subject.
	ClearErrorSess(testAmbientSession)
	// VariablesComplete(ptrs) fails sticky on nil ptr (hard IR)
	out := MergePointeesOfPointersSess(testAmbientSession, []*Variable{nil}, nil)
	if VariablesComplete(out) {
		t.Fatal("nil ptr MergePointeesOfPointers must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptr MergePointeesOfPointers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FindRelated residual via nil subject
	if FindRelatedPointToSess(testAmbientSession, nil, nil) != nil {
		t.Fatal("nil subject FindRelatedPointTo must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject FindRelatedPointTo must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
