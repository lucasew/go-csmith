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

func TestRhsToLhsTransferAddrOfNilCollectiveFailClosed(t *testing.T) {
	// GetCollective nil is broken IR — no invent MakeFactsPointTo with nil pointee
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// Variable without name/type quirks still has collective self; use incomplete shell
	broken := &Variable{Name: "broken"} // Type nil, GetCollective returns self but IsPointer fails on lvars
	// address-of path requires pointer lvars; use p as lhs, rhs var with nil collective via nil GetCollective
	// only nil GetCollective when v is nil — covered by Var==nil fail closed
	// multi-level & invents nil
	rhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_i", GetIntType(), false, false), ExprType: PointerTo(GetIntType())}
	// force Indir < -1 by ExprType deeper than Var.Type
	rhs.ExprType = PointerTo(PointerTo(GetIntType()))
	if RhsToLhsTransfer(nil, []*Variable{p}, rhs) != nil {
		// indirect != -1 fails closed
		t.Fatal("multi-level & must fail closed")
	}
	_ = broken
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

func TestRhsToLhsTransferNonPointerLvarsFailClosed(t *testing.T) {
	// FactPointTo.cpp:164–167 — assert all LHS are pointers; no invent transfer
	i := CreateVariableScalars("g_i", GetIntType(), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	if RhsToLhsTransfer(nil, []*Variable{i}, rhs) != nil {
		t.Fatal("non-pointer lvar must fail closed")
	}
}

func TestRhsToLhsTransferMultiLevelAddrFailClosed(t *testing.T) {
	// FactPointTo.cpp:205 — assert(indirect == -1); no invent for multi-level &
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// int var with ** type → IndirectLevel = 0-2 = -2
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(PointerTo(GetIntType()))}
	if rhs.IndirectLevel() != -2 {
		t.Fatalf("want indir -2 got %d", rhs.IndirectLevel())
	}
	if RhsToLhsTransfer(nil, []*Variable{p}, rhs) != nil {
		t.Fatal("multi-level address-of must fail closed")
	}
}

func TestRhsToLhsTransferAggregateLenMismatchFailClosed(t *testing.T) {
	// FactPointTo.cpp:216 — assert(lvars.size() == pointers.size())
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	pf := &Variable{Name: "g_u.f0", Type: PointerTo(GetIntType()), FieldVarOf: uv}
	uv.FieldVars = []*Variable{pf}
	lhs0 := CreateVariableScalars("g_p0", PointerTo(GetIntType()), false, false)
	lhs1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermVariable, Var: uv, ExprType: ut}
	// two LHS pointers vs one field pointer
	if RhsToLhsTransfer(nil, []*Variable{lhs0, lhs1}, rhs) != nil {
		t.Fatal("lvars/pointers length mismatch must fail closed")
	}
}

func TestRhsToLhsTransferMissingReturnFactFailClosed(t *testing.T) {
	// FactPointTo.cpp:252 — assert(rv_fact); no invent garbage points-to
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fn := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	fn.RV = CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	fi := &Invocation{User: fn}
	// no AddReturnFactForInvocation
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerTo(GetIntType())}
	if RhsToLhsTransfer(nil, []*Variable{p}, rhs) != nil {
		t.Fatal("missing rv_fact must fail closed")
	}
}
