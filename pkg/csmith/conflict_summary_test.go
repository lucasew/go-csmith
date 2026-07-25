package csmith

import (
	"testing"
)

func TestAllowVolatileAndAcceptType(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if !cg.AllowVolatile() {
		t.Fatal("SE-free allows volatile")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete AllowVolatile must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)
	if cg2.AllowVolatile() {
		t.Fatal("SE should block")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete non-SE AllowVolatile must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !cg.AllowConst(AccessRead) || cg.AllowConst(AccessWrite) {
		t.Fatal("const")
	}
	// volatile aggregate rejected when not SE-free
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{true}), BitWidth: -1},
	}}
	// IsVolatileStructUnion may need field volatile
	if cg2.AcceptType(st) && st.IsVolatileStructUnion() {
		t.Fatal("should reject vol struct")
	}
	// nil type / incomplete ambient sticky (no invent accept / soft re-pick)
	ClearErrorSess(testAmbientSession)
	if cg.AcceptType(nil) {
		t.Fatal("nil type must fail closed AcceptType")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type AcceptType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cgi := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	if cgi.AllowVolatile() {
		t.Fatal("incomplete ambient must not AllowVolatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient AllowVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cgi.AcceptType(GetIntType()) {
		t.Fatal("incomplete ambient must not invent AcceptType int")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient AcceptType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVolatileStructUnion residual: Type-nil field soft invent was accept true.
	// Fair: sticky reject under non-SE-free context.
	hole := &Type{isStruct: true, StructName: "Shole", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	if cg2.AcceptType(hole) {
		t.Fatal("IsVolatileStructUnion residual AcceptType must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileStructUnion residual AcceptType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInConflictReadWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	// context already wrote g
	ctx := WithEffectContext(EmptyEffect().WriteVar(g)).WithSession(testAmbientSession)
	// callee reads g
	eff := EmptyEffect().ReadVar(g)
	if !ctx.InConflict(eff) {
		t.Fatal("write then read conflict")
	}
	// empty context ok
	if EmptyCGContext().WithSession(testAmbientSession).InConflict(eff) {
		t.Fatal("empty should not conflict on read alone")
	}
}

func TestInConflictNoWrite(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	eff := EmptyEffect().WriteVar(g)
	if !cg.InConflict(eff) {
		t.Fatal("no_write conflict")
	}
}

func TestInConflictNilHoleFailClosed(t *testing.T) {
	// nil Variable* in effect lists sticky conflict (no invent conflict-free)
	ClearErrorSess(testAmbientSession)
	eff := EmptyEffect()
	eff.written = map[*Variable]bool{nil: true}
	if !EmptyCGContext().WithSession(testAmbientSession).InConflict(eff) {
		t.Fatal("nil write hole must fail closed as conflict")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil write hole InConflict must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	eff2 := EmptyEffect()
	eff2.read = map[*Variable]bool{nil: true}
	if !EmptyCGContext().WithSession(testAmbientSession).InConflict(eff2) {
		t.Fatal("nil read hole must fail closed as conflict")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil read hole InConflict must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestInConflictIncompleteEffectFailClosed(t *testing.T) {
	// IncompleteEffect / incomplete ambient sticky conflict
	ClearErrorSess(testAmbientSession)
	if !EmptyCGContext().WithSession(testAmbientSession).InConflict(IncompleteEffect()) {
		t.Fatal("IncompleteEffect must fail closed as conflict")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect InConflict must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	if !cg.InConflict(EmptyEffect()) {
		t.Fatal("incomplete ambient must fail closed as conflict")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient InConflict must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseFuncContextSkipsConflict(t *testing.T) {
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	bad := &Function{Name: "bad", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	bad.FEffect = EmptyEffect().WriteVar(g)
	good := &Function{Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	// context already wrote g → bad conflicts
	cg := WithEffectContext(EmptyEffect().WriteVar(g)).WithSession(testAmbientSession)
	got := ChooseFuncContext(NewRng(2), []*Function{bad, good}, GetIntType(), nil, &cg, Defaults(), nil)
	if got != good {
		t.Fatalf("got %v", got)
	}
}

func TestComputeSummaryReferencedPtrs(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.Body = &Block{Stmts: []Stmt{
		{Kind: StmtAssign, LhsVar: p, Lhs: &Lhs{Var: p, Type: p.Type},
			Expr: &Expression{Term: TermConstant, Con: &Constant{Type: p.Type, Value: "0"}}, AssignOp: AssignSimple},
	}}
	bodyEff := EmptyEffect().WriteVar(p)
	f.ComputeSummary(bodyEff)
	if len(f.ReferencedPtrs) != 1 || f.ReferencedPtrs[0] != p {
		t.Fatal(f.ReferencedPtrs)
	}
	if !f.FEffect.IsWritten(p) {
		t.Fatal("feffect")
	}
	if f.UnionFieldRead {
		t.Fatal("no union field on plain ptr assign")
	}
}

func TestReadUnionFieldForTestExpr(t *testing.T) {
	// StatementFor get_exprs → test; soft invent skip would miss for-test union field
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	f0 := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: uv}
	uv.FieldVars = []*Variable{f0}
	test := &Expression{Term: TermVariable, Var: f0, ExprType: GetIntType()}
	st := &Stmt{Kind: StmtFor, Loop: &LoopControl{TestExpr: test}, Then: &Block{}}
	if !ReadUnionFieldStmt(st) {
		t.Fatal("for-test union field must count")
	}
	// incomplete for without TestExpr fails closed true
	if !ReadUnionFieldStmt(&Stmt{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}}) {
		t.Fatal("incomplete for must fail closed true")
	}
}

func TestReadUnionFieldCalleeFlag(t *testing.T) {
	// Statement.cpp:671–676 — callee union_field_read
	callee := &Function{Name: "g", UnionFieldRead: true, ReturnType: GetIntType(), IsBuilt: true}
	call := &Expression{Term: TermFunction, Invoke: &Invocation{User: callee}}
	st := &Stmt{Kind: StmtInvoke, Expr: call}
	if !ReadUnionFieldStmt(st) {
		t.Fatal("callee UnionFieldRead must count")
	}
	// assign/invoke without Expr — no invent "no union field read"
	if !ReadUnionFieldStmt(&Stmt{Kind: StmtAssign, StmID: 1}) {
		t.Fatal("nil Expr assign must fail closed true")
	}
	if !ReadUnionFieldExpr(nil) {
		t.Fatal("nil expr must fail closed true")
	}
}

func TestCollectReferencedPtrsAssignNilExprFailClosed(t *testing.T) {
	// C++ get_exprs always live; nil Expr must not invent empty ptr list sticky
	ClearErrorSess(testAmbientSession)
	var ptrs []*Variable
	ptrs = []*Variable{CreateVariableScalars("stale", PointerTo(GetIntType()), false, false)}
	CollectReferencedPtrsStmt(&Stmt{Kind: StmtAssign, StmID: 1}, &ptrs)
	// IncompleteVariables sticky — not bare nil invent empty-complete
	if VariablesComplete(ptrs) {
		t.Fatal("incomplete collect must IncompleteVariables, not empty-complete", ptrs)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("assign without Expr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ptrs = []*Variable{}
	CollectReferencedPtrsStmt(&Stmt{Kind: StmtInvoke, StmID: 2}, &ptrs)
	if VariablesComplete(ptrs) {
		t.Fatal("incomplete invoke must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invoke without Expr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// ptrs always live; sticky (no invent soft-skip collect past hole)
	CollectReferencedPtrsExpression(&Expression{Term: TermConstant, Con: MakeInt(0)}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptrs CollectReferencedPtrsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CollectReferencedPtrsStmt(&Stmt{Kind: StmtBreak, StmID: 3}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptrs CollectReferencedPtrsStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	CollectReferencedPtrsBlock(&Block{}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ptrs CollectReferencedPtrsBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live in collect walks; sticky
	got := appendUniqueVar(nil, nil)
	if got != nil {
		t.Fatal("nil var appendUniqueVar must leave list", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var appendUniqueVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// TermFunction without Invoke sticky incomplete collect
	var ptrs2 []*Variable
	CollectReferencedPtrsExpression(&Expression{Term: TermFunction}, &ptrs2)
	if VariablesComplete(ptrs2) {
		t.Fatal("nil Invoke collect must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Invoke CollectReferencedPtrsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Expr assign sticky
	var ptrs3 []*Variable
	CollectReferencedPtrsStmt(&Stmt{Kind: StmtAssign, StmID: 9}, &ptrs3)
	if VariablesComplete(ptrs3) {
		t.Fatal("nil Expr assign collect must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr CollectReferencedPtrsStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil Variable sticky (no invent complete no-ptrs via IsPointer false)
	var ptrs4 []*Variable
	CollectReferencedPtrsExpression(&Expression{
		Term: TermVariable, Var: &Variable{Name: "g_hole", Type: nil},
	}, &ptrs4)
	if VariablesComplete(ptrs4) {
		t.Fatal("Type-nil Var collect must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil Var CollectReferencedPtrsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil LhsVar sticky
	var ptrs5 []*Variable
	CollectReferencedPtrsStmt(&Stmt{
		Kind: StmtAssign, StmID: 10,
		Expr:   &Expression{Term: TermConstant, Con: MakeInt(0)},
		LhsVar: &Variable{Name: "g_hole", Type: nil},
	}, &ptrs5)
	if VariablesComplete(ptrs5) {
		t.Fatal("Type-nil LhsVar collect must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil LhsVar CollectReferencedPtrsStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// StatementArrayOp.h:65–68 / 69–71 — get_exprs if(init_value); get_blocks if(body).
	// array_init Expression ctor: body=0, init_value=e. Go Then is Output-only.
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(0)}
	ptrArr := CreateVariableScalars("a", PointerTo(GetIntType()), false, false)
	body := &Block{Stmts: []Stmt{{
		Kind: StmtAssign, LhsVar: ptrArr,
		Expr: rhs, ArrayAccess: "a[i]", StmID: 2,
	}}}
	var ptrsArr []*Variable
	// with init_value on Expr (C++ path) — must NOT walk Then Lhs as extra ptr
	CollectReferencedPtrsStmt(&Stmt{
		Kind: StmtArrayOp, StmID: 1,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 4, IncrN: 1},
		Then: body, Expr: rhs,
	}, &ptrsArr)
	if !VariablesComplete(ptrsArr) {
		t.Fatal("array-init ArrayOp must collect without IncompleteVariables", ptrsArr)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("array-init ArrayOp must not SetError sticky")
	}
	for _, p := range ptrsArr {
		if p == ptrArr {
			t.Fatal("array-init get_blocks empty: must not invent Lhs ptr from Output Then")
		}
	}
	ClearErrorSess(testAmbientSession)
	// Expr-only (no Then) still completes
	var ptrsInit []*Variable
	CollectReferencedPtrsStmt(&Stmt{
		Kind: StmtArrayOp, StmID: 3,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 2, IncrN: 1},
		Expr: rhs,
	}, &ptrsInit)
	if !VariablesComplete(ptrsInit) {
		t.Fatal("ArrayOp with init_value Expr must complete", ptrsInit)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("ArrayOp init_value path must not SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestComputeSummaryIncompleteForFailClosed(t *testing.T) {
	// incomplete for in body — no invent clean empty summary (false UnionFieldRead)
	// nor IsPointerReferenced false via bare-nil ReferencedPtrs — sticky
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.Body = &Block{Stmts: []Stmt{
		{Kind: StmtFor, Loop: &LoopControl{}, Then: &Block{}},
	}}
	f.ComputeSummary(EmptyEffect())
	if !f.UnionFieldRead {
		t.Fatal("incomplete for must fail closed UnionFieldRead")
	}
	if VariablesComplete(f.ReferencedPtrs) {
		t.Fatal("incomplete for must IncompleteVariables ReferencedPtrs, not empty-complete")
	}
	if !f.IsPointerReferenced() {
		t.Fatal("incomplete ReferencedPtrs must fail closed IsPointerReferenced true")
	}
	if !f.NeedsRevisit() {
		t.Fatal("incomplete summary must NeedsRevisit")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete referenced-ptrs walk must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Function always live; sticky no invent soft-skip summary past hole
	(*Function)(nil).ComputeSummary(EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function ComputeSummary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsFrameVar(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if !cg.IsFrameVar(loc) {
		t.Fatal("local frame")
	}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if cg.IsFrameVar(g) {
		t.Fatal("global not frame")
	}
	// CGContext.cpp:494 — no curr_blk: complete not-frame (not invent via call_chain only;
	// sticky would poison FindReachableFrameVars empty-frame complete path)
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.CallChain = []*Block{blk}
	if cg2.IsFrameVar(loc) {
		t.Fatal("nil curr_blk must fail closed, not invent via call_chain only")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil curr_blk IsFrameVar must stay non-sticky complete not-frame")
	}
	// incomplete LocalVars sticky not-frame
	ClearErrorSess(testAmbientSession)
	blkHole := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Stack = []*Block{blkHole}
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	if cg3.IsFrameVar(loc) {
		t.Fatal("incomplete stack IsFrameVar must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete stack IsFrameVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Stack = []*Block{blk}
}

func TestDoFinalization(t *testing.T) {
	prevR := ProcessRngSess(testAmbientSession)
	prevP := ProcessProbabilitiesSess(testAmbientSession)
	defer func() {
		SetProcessRngSess(testAmbientSession, prevR)
		SetProcessProbabilitiesSess(testAmbientSession, prevP)
	}()
	IncrCounterSess(testAmbientSession, &currentSession().BK.structDepthCnts, 1)
	currentSession().NextStmID = 5
	SetErrorSess(testAmbientSession, ErrGeneric)
	DoFinalizationSess(testAmbientSession)
	if len(currentSession().BK.structDepthCnts) != 0 || currentSession().NextStmID != 0 || HasErrorSess(testAmbientSession) {
		t.Fatal("not cleared")
	}
}

func TestReadUnionFieldIncompleteSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !ReadUnionFieldExpr(nil) {
		t.Fatal("nil Expr ReadUnionField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr ReadUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !ReadUnionFieldStmt(nil) {
		t.Fatal("nil Stmt ReadUnionField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Stmt ReadUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete constant assign does not read union field
	st := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, LhsVar: CreateVariableScalars("g_x", GetIntType(), false, false), AssignOp: AssignSimple}
	if ReadUnionFieldStmt(st) {
		t.Fatal("scalar assign must not read union field")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete ReadUnionFieldStmt must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsInsideUnionField residual on LhsVar: soft invent was soft-continue no-union-read.
	// Fair: sticky true.
	parentHole := &Variable{Name: "g_u"} // Type nil
	fieldHole := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parentHole}
	stHole := &Stmt{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, LhsVar: fieldHole, AssignOp: AssignSimple}
	if !ReadUnionFieldStmt(stHole) {
		t.Fatal("IsInsideUnionField residual ReadUnionFieldStmt must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsInsideUnionField residual ReadUnionFieldStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// comma LHS residual soft invent was soft-continue RHS invent no-union-read.
	lhsHole := &Expression{Term: TermVariable, Var: fieldHole}
	rhsOK := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_x2", GetIntType(), false, false)}
	comma := &Expression{Term: TermCommaExpr, CommaLHS: lhsHole, CommaRHS: rhsOK}
	if !ReadUnionFieldExpr(comma) {
		t.Fatal("nested IsInside residual ReadUnionFieldExpr must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested IsInside residual ReadUnionFieldExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNeedNestedLoopIsEffectKnownSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Block)(nil).NeedNestedLoop(EmptyCGContext().WithSession(testAmbientSession), NewRng(1)) {
		t.Fatal("nil Block NeedNestedLoop must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Block NeedNestedLoop must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Function)(nil).IsEffectKnown() {
		t.Fatal("nil Function IsEffectKnown must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Function IsEffectKnown must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
