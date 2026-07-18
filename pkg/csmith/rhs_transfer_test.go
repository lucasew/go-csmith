package csmith

import "testing"

func TestConstantEquals(t *testing.T) {
	if !MakeInt(0).Equals(0) {
		t.Fatal("0")
	}
	if MakeInt(3).Equals(0) {
		t.Fatal("3")
	}
}

func TestRhsToLhsTransferNullConst(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNull() {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferAddrOf(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// ExpressionVariable with ExprType = pointer means &g_t if var is int?
	// IndirectLevel = var.level - exprType.level; int(0) - ptr(1) = -1 → address-of
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	if rhs.IndirectLevel() != -1 {
		t.Fatalf("indir %d", rhs.IndirectLevel())
	}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts[0])
	}
}

func TestRhsToLhsTransferCopy(t *testing.T) {
	p1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	in := []*FactPointTo{MakeFactPointTo(p2, tgt)}
	rhs := &Expression{Term: TermVariable, Var: p2, ExprType: PointerTo(GetIntType())}
	// p2 level 1, expr 1 → indirect 0; merge uses fact of p2
	facts := RhsToLhsTransfer(in, []*Variable{p1}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestUpdateFactForAssign(t *testing.T) {
	fm := NewFactMgr(nil)
	// Variable.cpp:395 — pointer Constant::make_random is "0" → null on AddNewVarFact
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFact(p)
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("init null")
	}
	// assign to non-null target expression
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	rhs := &Expression{Term: TermVariable, Var: a, ExprType: PointerTo(GetIntType())}
	// take address form for pointee
	rhs = &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	if !fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("update")
	}
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("null after assign")
	}
}

func TestAbstractFactNonPointerLHS(t *testing.T) {
	v := CreateVariableScalars("g_i", GetIntType(), false, false)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if AbstractFactForAssign(nil, v, 0, rhs) != nil {
		t.Fatal("non-ptr")
	}
}

func TestRhsToLhsTransferCommaPeel(t *testing.T) {
	// FactPointTo.cpp:259–261 — comma uses RHS of comma
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(1)},
		CommaRHS: &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}, ExprType: PointerTo(GetIntType())},
		ExprType: PointerTo(GetIntType()),
	}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNull() {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferAssignPeel(t *testing.T) {
	// FactPointTo.cpp:256–258 — embedded assign peels to assign RHS
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	assign := &Stmt{
		Kind: StmtAssign, LhsVar: q, Lhs: &Lhs{Var: q, Type: PointerTo(GetIntType())},
		Expr:     &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}, ExprType: PointerTo(GetIntType())},
		AssignOp: AssignSimple,
	}
	rhs := &Expression{Term: TermAssignment, Assign: assign, ExprType: PointerTo(GetIntType())}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNull() {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferFunctionReturn(t *testing.T) {
	// FactPointTo.cpp:247–253 — RV return fact copied to LHS
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	fn := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	fn.RV = CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	fi := &Invocation{User: fn}
	AddReturnFactForInvocation(fi, MakeFactPointTo(fn.RV, tgt))
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerTo(GetIntType())}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || len(facts[0].PointTo) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferUnionAggregateFields(t *testing.T) {
	// FactPointTo.cpp:172 + 210–224 — only pointer/union pass early type gate;
	// union RHS maps pointer fields pairwise (struct RHS is garbage early).
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	pf := &Variable{Name: "g_u.f0", Type: PointerTo(GetIntType()), FieldVarOf: uv}
	uv.FieldVars = []*Variable{pf}
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	lhsP := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	in := []*FactPointTo{MakeFactPointTo(pf, tgt)}
	rhs := &Expression{Term: TermVariable, Var: uv, ExprType: ut}
	facts := RhsToLhsTransfer(in, []*Variable{lhsP}, rhs)
	if len(facts) != 1 || len(facts[0].PointTo) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferStructIsGarbage(t *testing.T) {
	// FactPointTo.cpp:172–178 — struct type fails pointer/union gate → garbage
	st := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	sv := &Variable{Name: "g_s", Type: st}
	lhsP := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermVariable, Var: sv, ExprType: st}
	facts := RhsToLhsTransfer(nil, []*Variable{lhsP}, rhs)
	if len(facts) != 1 || !facts[0].IsDead() {
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
			{Name: "x", Type: GetIntType(), BitWidth: -1},
			{Name: "p0", Type: PointerTo(GetIntType()), BitWidth: -1},
			{Name: "p1", Type: PointerTo(GetIntType()), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		uv.CreateFieldVars()
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
		if f.IsPointer() {
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
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}
	facts := AbstractFactForAssign(nil, xField, 0, rhs)
	got0 := FindRelatedPointTo(facts, p0)
	got1 := FindRelatedPointTo(facts, p1)
	if got0 == nil || got1 == nil {
		t.Fatalf("union walk should yield ptr field facts: %+v", facts)
	}
}
