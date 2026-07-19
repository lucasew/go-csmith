package csmith

import (
	"testing"
)

func TestAllowVolatileAndAcceptType(t *testing.T) {
	cg := EmptyCGContext()
	if !cg.AllowVolatile() {
		t.Fatal("SE-free allows volatile")
	}
	cg2 := WithEffectContext(WithSideEffects())
	if cg2.AllowVolatile() {
		t.Fatal("SE should block")
	}
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
}

func TestInConflictReadWrite(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	// context already wrote g
	ctx := WithEffectContext(EmptyEffect().WriteVar(g))
	// callee reads g
	eff := EmptyEffect().ReadVar(g)
	if !ctx.InConflict(eff) {
		t.Fatal("write then read conflict")
	}
	// empty context ok
	if EmptyCGContext().InConflict(eff) {
		t.Fatal("empty should not conflict on read alone")
	}
}

func TestInConflictNoWrite(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	eff := EmptyEffect().WriteVar(g)
	if !cg.InConflict(eff) {
		t.Fatal("no_write conflict")
	}
}

func TestInConflictNilHoleFailClosed(t *testing.T) {
	// nil Variable* in effect lists must not invent conflict-free
	eff := EmptyEffect()
	eff.written = map[*Variable]bool{nil: true}
	if !EmptyCGContext().InConflict(eff) {
		t.Fatal("nil write hole must fail closed as conflict")
	}
	eff2 := EmptyEffect()
	eff2.read = map[*Variable]bool{nil: true}
	if !EmptyCGContext().InConflict(eff2) {
		t.Fatal("nil read hole must fail closed as conflict")
	}
}

func TestInConflictIncompleteEffectFailClosed(t *testing.T) {
	// IncompleteEffect / incomplete ambient must not invent conflict-free
	if !EmptyCGContext().InConflict(IncompleteEffect()) {
		t.Fatal("IncompleteEffect must fail closed as conflict")
	}
	cg := WithEffectContext(IncompleteEffect())
	if !cg.InConflict(EmptyEffect()) {
		t.Fatal("incomplete ambient must fail closed as conflict")
	}
}

func TestChooseFuncContextSkipsConflict(t *testing.T) {
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	bad := &Function{Name: "bad", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	bad.FEffect = EmptyEffect().WriteVar(g)
	good := &Function{Name: "good", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	// context already wrote g → bad conflicts
	cg := WithEffectContext(EmptyEffect().WriteVar(g))
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
	// C++ get_exprs always live; nil Expr must not invent empty ptr list as success
	var ptrs []*Variable
	ptrs = []*Variable{CreateVariableScalars("stale", PointerTo(GetIntType()), false, false)}
	if collectReferencedPtrsStmt(&Stmt{Kind: StmtAssign, StmID: 1}, &ptrs) {
		t.Fatal("assign without Expr must fail closed")
	}
	// IncompleteVariables marker — not bare nil invent empty-complete
	if VariablesComplete(ptrs) {
		t.Fatal("incomplete collect must IncompleteVariables, not empty-complete", ptrs)
	}
	ptrs = []*Variable{}
	if collectReferencedPtrsStmt(&Stmt{Kind: StmtInvoke, StmID: 2}, &ptrs) {
		t.Fatal("invoke without Expr must fail closed")
	}
	if VariablesComplete(ptrs) {
		t.Fatal("incomplete invoke must IncompleteVariables")
	}
}

func TestComputeSummaryIncompleteForFailClosed(t *testing.T) {
	// incomplete for in body — no invent clean empty summary (false UnionFieldRead)
	// nor IsPointerReferenced false via bare-nil ReferencedPtrs
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
}

func TestIsFrameVar(t *testing.T) {
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	if !cg.IsFrameVar(loc) {
		t.Fatal("local frame")
	}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if cg.IsFrameVar(g) {
		t.Fatal("global not frame")
	}
	// CGContext.cpp:494 — assert(b); no invent frame without curr_blk
	cg2 := EmptyCGContext()
	cg2.CallChain = []*Block{blk}
	if cg2.IsFrameVar(loc) {
		t.Fatal("nil curr_blk must fail closed, not invent via call_chain only")
	}
}

func TestDoFinalization(t *testing.T) {
	IncrCounter(&structDepthCnts, 1)
	nextStmID = 5
	SetError(ErrGeneric)
	DoFinalization()
	if len(structDepthCnts) != 0 || nextStmID != 0 || HasError() {
		t.Fatal("not cleared")
	}
}
