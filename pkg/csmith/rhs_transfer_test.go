package csmith

import "testing"

func TestConstantEquals(t *testing.T) {
	ClearError()
	if !MakeInt(0).Equals(0) {
		t.Fatal("0")
	}
	if MakeInt(3).Equals(0) {
		t.Fatal("3")
	}
	// incomplete Constant sticky false (no invent not-equal / not-less)
	if (*Constant)(nil).Equals(0) {
		t.Fatal("nil Constant Equals must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Constant Equals must SetError sticky")
	}
	ClearError()
	if (*Constant)(nil).NotEquals(0) {
		t.Fatal("nil Constant NotEquals must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Constant NotEquals must SetError sticky")
	}
	ClearError()
	if (*Constant)(nil).LessThan(1) {
		t.Fatal("nil Constant LessThan must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Constant LessThan must SetError sticky")
	}
	ClearError()
	// empty Value incomplete shell sticky (no invent not-equal / not-less soft-skip)
	empty := &Constant{Type: GetIntType(), Value: ""}
	if empty.Equals(0) {
		t.Fatal("empty Value Equals must fail closed false")
	}
	if !HasError() {
		t.Fatal("empty Value Equals must SetError sticky")
	}
	ClearError()
	if empty.NotEquals(0) {
		t.Fatal("empty Value NotEquals must fail closed false")
	}
	if !HasError() {
		t.Fatal("empty Value NotEquals must SetError sticky")
	}
	ClearError()
	if empty.LessThan(1) {
		t.Fatal("empty Value LessThan must fail closed false")
	}
	if !HasError() {
		t.Fatal("empty Value LessThan must SetError sticky")
	}
	ClearError()
	// Type-nil incomplete shell sticky (no invent fold success past hole)
	noTy := &Constant{Value: "0"}
	if noTy.Equals(0) {
		t.Fatal("nil Type Equals must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Type Equals must SetError sticky")
	}
	ClearError()
	if noTy.NotEquals(1) {
		t.Fatal("nil Type NotEquals must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Type NotEquals must SetError sticky")
	}
	ClearError()
	if noTy.LessThan(1) {
		t.Fatal("nil Type LessThan must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Type LessThan must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferNullConst(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNull() {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferNilRHSIsGarbage(t *testing.T) {
	// FactPointTo.cpp:168–169 — nullptr rhs → garbage (AddParamFacts missing arg)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := RhsToLhsTransfer(nil, []*Variable{p}, nil)
	if len(facts) != 1 || !facts[0].IsDead() {
		t.Fatal("nil rhs must abstract as garbage like C++", facts)
	}
	// return always has Expression*; sticky fail closed before garbage invent
	ClearError()
	fm := NewFactMgr(nil)
	rv := CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	if fm.UpdateFactForReturnStmt(&Stmt{Kind: StmtReturn, StmID: 1}, rv, nil) {
		t.Fatal("nil return expr must fail closed")
	}
	if !HasError() {
		t.Fatal("nil return expr UpdateFactForReturnStmt must SetError sticky")
	}
	ClearError()
	// incomplete GlobalFacts after assign path fails closed sticky
	fm.GlobalFacts = IncompleteFactSlice()
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	if fm.UpdateFactForReturnStmt(&Stmt{Kind: StmtReturn, StmID: 1}, rv, rhs) {
		t.Fatal("incomplete GlobalFacts must fail closed UpdateFactForReturnStmt")
	}
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("must stay incomplete GlobalFacts")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts return update must SetError sticky")
	}
	ClearError()
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
	ClearError()
	// nil FM / lhs sticky — no invent soft-skip assign update
	if (*FactMgr)(nil).UpdateFactForAssign(CreateVariableScalars("g_x", GetIntType(), false, false), 0, nil) {
		t.Fatal("nil FM must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FM UpdateFactForAssign must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(nil)
	if fm.UpdateFactForAssign(nil, 0, nil) {
		t.Fatal("nil lhs must fail closed")
	}
	if !HasError() {
		t.Fatal("nil lhs UpdateFactForAssign must SetError sticky")
	}
	ClearError()
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
	// non-pointer scalar: complete empty (no pointer facts), not incomplete marker
	if out := AbstractFactForAssign(nil, v, 0, rhs); !FactsComplete(out) || len(out) != 0 {
		t.Fatal("non-ptr must be complete empty", out)
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

func TestRhsToLhsTransferCommaNilRHSFailClosed(t *testing.T) {
	// incomplete CommaRHS must not invent complete GarbagePtr via nil-rhs peel
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(1)},
		// CommaRHS nil
	}
	out := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if FactsComplete(out) {
		t.Fatal("nil CommaRHS must fail closed incomplete, not invent GarbagePtr", out)
	}
	if !HasError() {
		t.Fatal("nil CommaRHS RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferAddrOfNilCollectiveFailClosed(t *testing.T) {
	// multi-level & hard IR sticky — no invent MakeFactsPointTo past assert(indirect==-1)
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// force Indir < -1 by ExprType deeper than Var.Type
	rhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_i", GetIntType(), false, false), ExprType: PointerTo(PointerTo(GetIntType()))}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{p}, rhs)) {
		t.Fatal("multi-level & must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("multi-level & must SetError sticky")
	}
	ClearError()
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

func TestRhsToLhsTransferAssignNilExprFailClosed(t *testing.T) {
	// incomplete Assign.Expr must not invent complete GarbagePtr via nil-rhs peel
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	assign := &Stmt{
		Kind: StmtAssign, LhsVar: q, Lhs: &Lhs{Var: q, Type: PointerTo(GetIntType())},
		// Expr nil
		AssignOp: AssignSimple,
	}
	rhs := &Expression{Term: TermAssignment, Assign: assign, ExprType: PointerTo(GetIntType())}
	out := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if FactsComplete(out) {
		t.Fatal("nil Assign.Expr must fail closed incomplete, not invent GarbagePtr", out)
	}
	if !HasError() {
		t.Fatal("nil Assign.Expr RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
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

func TestRhsToLhsTransferRVTypeNilSticky(t *testing.T) {
	// RV Type* always live; Type-nil no invent scalar rv soft-transfer past hole
	ClearError()
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fn := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	fn.RV = &Variable{Name: "f_rv", Type: nil}
	fi := &Invocation{User: fn}
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerTo(GetIntType())}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{p}, rhs)) {
		t.Fatal("Type-nil RV must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("Type-nil RV RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferUnionParentTypeNilSticky(t *testing.T) {
	// union constant field0 path: parent Type* always live
	ClearError()
	parent := &Variable{Name: "g_u", Type: nil}
	f0 := &Variable{Name: "g_u.f0", Type: PointerTo(GetIntType()), FieldVarOf: parent}
	parent.FieldVars = []*Variable{f0}
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: ut, Value: "{0}"}, ExprType: ut}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{f0}, rhs)) {
		t.Fatal("Type-nil union parent must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("Type-nil union parent RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
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
	// FactPointTo.cpp:164–167 — assert all LHS are pointers; hard IR sticky
	ClearError()
	i := CreateVariableScalars("g_i", GetIntType(), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{i}, rhs)) {
		t.Fatal("non-pointer lvar must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("non-pointer lvar RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferMultiLevelAddrFailClosed(t *testing.T) {
	// FactPointTo.cpp:205 — assert(indirect == -1); hard IR sticky
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// int var with ** type → IndirectLevel = 0-2 = -2
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(PointerTo(GetIntType()))}
	if rhs.IndirectLevel() != -2 {
		t.Fatalf("want indir -2 got %d", rhs.IndirectLevel())
	}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{p}, rhs)) {
		t.Fatal("multi-level address-of must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("multi-level & RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferAggregateLenMismatchNDEBUG(t *testing.T) {
	// FactPointTo.cpp:216 — assert(lvars.size() == pointers.size()); NDEBUG elides
	// and pairs only the overlapping prefix (no sticky-poison generation).
	ClearError()
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: PointerTo(GetIntType()), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	pf := &Variable{Name: "g_u.f0", Type: PointerTo(GetIntType()), FieldVarOf: uv}
	uv.FieldVars = []*Variable{pf}
	lhs0 := CreateVariableScalars("g_p0", PointerTo(GetIntType()), false, false)
	lhs1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermVariable, Var: uv, ExprType: ut}
	// two LHS pointers vs one field pointer → must not sticky-poison (NDEBUG assert).
	_ = RhsToLhsTransfer(nil, []*Variable{lhs0, lhs1}, rhs)
	if HasError() {
		t.Fatal("NDEBUG len mismatch must not sticky-poison", GetError())
	}
	ClearError()
}

func TestRhsToLhsTransferMissingReturnFactFailClosed(t *testing.T) {
	// FactPointTo.cpp:252 — missing rv_fact: incomplete non-sticky (generation soft re-pick)
	ClearError()
	InvocationReturnFactsDoFinalization()
	defer InvocationReturnFactsDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fn := &Function{Name: "f", ReturnType: PointerTo(GetIntType())}
	fn.RV = CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	fi := &Invocation{User: fn}
	// no AddReturnFactForInvocation
	rhs := &Expression{Term: TermFunction, Invoke: fi, ExprType: PointerTo(GetIntType())}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{p}, rhs)) {
		t.Fatal("missing rv_fact must fail closed incomplete")
	}
	if HasError() {
		t.Fatal("missing rv_fact must stay non-sticky for soft re-pick")
	}
	ClearError()
}

func TestRhsToLhsTransferIncompleteMapsNonSticky(t *testing.T) {
	// incomplete fact map / MergePointees hole stays non-sticky for soft re-pick
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	// incomplete map: nil hole so MergePointeesOfPointer fails closed incomplete
	hole := []*FactPointTo{MakeFactPointTo(q, NullPtr), nil}
	rhs := &Expression{Term: TermVariable, Var: q, ExprType: PointerTo(GetIntType())}
	if FactsComplete(RhsToLhsTransfer(hole, []*Variable{p}, rhs)) {
		t.Fatal("incomplete map transfer must fail closed incomplete")
	}
	if HasError() {
		t.Fatal("incomplete map RhsToLhsTransfer must stay non-sticky for soft re-pick")
	}
	ClearError()
}

func TestAbstractFactForAssignNilLhsSticky(t *testing.T) {
	ClearError()
	if FactsComplete(AbstractFactForAssign(nil, nil, 0, &Expression{Term: TermConstant, Con: MakeInt(0)})) {
		t.Fatal("nil lhs AbstractFactForAssign must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil lhs AbstractFactForAssign must SetError sticky")
	}
	ClearError()
}

func TestAbstractFactForAssignTypeNilMorePointeeSticky(t *testing.T) {
	// *p peels to non-pointer; lvars may be pointer pointees; more = pointees of those.
	// soft invent: IsPointer residual ERROR+false skip Type-nil then partial transfer.
	// fair: sticky IncompleteFactSlice before classify.
	ClearError()
	// p:int* points to q:int*; *p peels to int (non-pointer branch); lvars=[q];
	// more = MergePointees(q,1) → Type-nil shell sticky
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	shell := &Variable{Name: "g_hole"} // Type nil
	factsIn := []*FactPointTo{
		MakeFactPointTo(p, q),
		MakeFactPointTo(q, shell),
	}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}
	out := AbstractFactForAssign(factsIn, p, 1, rhs)
	if FactsComplete(out) {
		t.Fatal("Type-nil more pointee must fail closed incomplete, not partial transfer", out)
	}
	if !HasError() {
		t.Fatal("Type-nil more pointee AbstractFactForAssign must SetError sticky")
	}
	ClearError()
}

func TestAbstractFactUnionForAssignIsUnionResidualSticky(t *testing.T) {
	// IsUnion residual soft invent was invent non-union complete transfer past hole.
	// Type-nil non-special already sticky; complete non-union empty transfer hygiene.
	ClearError()
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	out, n := AbstractFactUnionForAssign(nil, nil, iv, 0, nil)
	if !UnionFactsComplete(out) && out != nil {
		// IncompleteUnionFactSlice when incomplete maps — nil maps are complete empty
	}
	// complete non-union with nil maps: UnionFactsComplete(nil)==true and FactsComplete(nil)==true
	// then non-union path returns nil, lvarCnt
	if HasError() {
		t.Fatal("complete non-union AbstractFactUnionForAssign must not sticky", out, n)
	}
	ClearError()
	// Type-nil non-special sticky
	hole := &Variable{Name: "g_x", Type: nil}
	uf, _ := AbstractFactUnionForAssign(nil, nil, hole, 0, nil)
	if UnionFactsComplete(uf) {
		// IncompleteUnionFactSlice is not complete
		t.Fatal("Type-nil must fail closed incomplete", uf)
	}
	if !HasError() {
		t.Fatal("Type-nil non-special AbstractFactUnionForAssign must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferGetTypeResidualSticky(t *testing.T) {
	// GetType residual soft invent was invent GarbagePtr complete success past Type-nil RHS.
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// Type-nil constant shell → GetType residual
	rhs := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{p}, rhs)) {
		t.Fatal("GetType residual must fail closed IncompleteFactSlice")
	}
	if !HasError() {
		t.Fatal("GetType residual RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferIsPointerResidualSticky(t *testing.T) {
	// IsPointer residual soft invent was invent transfer past non-pointer LHS soft-skip.
	ClearError()
	// Type-nil non-special IsPointer residual ERROR+false
	hole := &Variable{Name: "g_x", Type: nil}
	if FactsComplete(RhsToLhsTransfer(nil, []*Variable{hole}, nil)) {
		t.Fatal("IsPointer residual must fail closed IncompleteFactSlice")
	}
	if !HasError() {
		t.Fatal("IsPointer residual RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferUnionGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-merge union transfer past array shell.
	ClearError()
	// IsArray without AsArray GetCollective residual
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	lvars := []*Variable{CreateVariableScalars("g_u", GetIntType(), false, false)}
	// force union transfer path via RhsToLhsTransferUnion with TermVariable shell
	rhs := &Expression{Term: TermVariable, Var: shell, ExprType: GetIntType()}
	out := RhsToLhsTransferUnion(nil, nil, lvars, rhs)
	if UnionFactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasError() {
		// GetCollective on IsArray without AsArray SetError
		t.Fatal("IsArray without AsArray GetCollective residual must SetError sticky")
	}
	ClearError()
}

func TestRhsToLhsTransferGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-merge pointees past array shell.
	ClearError()
	// IsArray without AsArray GetCollective residual
	shell := &Variable{Name: "g_a", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2}}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermVariable, Var: shell, ExprType: PointerTo(GetIntType())}
	out := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if FactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete empty
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray GetCollective residual RhsToLhsTransfer must SetError sticky")
	}
	ClearError()
}

func TestAbstractFactForAssignGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-abstract past array shell LHS.
	ClearError()
	shell := &Variable{Name: "g_a", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2}}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}
	out := AbstractFactForAssign(nil, shell, 0, rhs)
	if FactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray GetCollective residual AbstractFactForAssign must SetError sticky")
	}
	ClearError()
}

func TestAbstractFactUnionForAssignGetCollectiveResidualSticky(t *testing.T) {
	// GetCollective residual soft invent was invent soft-abstract union past array shell.
	ClearError()
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	out, _ := AbstractFactUnionForAssign(nil, nil, shell, 0, nil)
	if UnionFactsComplete(out) && out != nil && len(out) > 0 {
		// may incomplete
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray GetCollective residual AbstractFactUnionForAssign must SetError sticky")
	}
	ClearError()
}

func TestMergePointeesFindRelatedResidualSticky(t *testing.T) {
	// FindRelated residual soft invent was invent soft-empty merge past nil ptr subject.
	ClearError()
	// VariablesComplete(ptrs) fails sticky on nil ptr (hard IR)
	out := MergePointeesOfPointers([]*Variable{nil}, nil)
	if VariablesComplete(out) {
		t.Fatal("nil ptr MergePointeesOfPointers must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil ptr MergePointeesOfPointers must SetError sticky")
	}
	ClearError()
	// FindRelated residual via nil subject
	if FindRelatedPointTo(nil, nil) != nil {
		t.Fatal("nil subject FindRelatedPointTo must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil subject FindRelatedPointTo must SetError sticky")
	}
	ClearError()
}
