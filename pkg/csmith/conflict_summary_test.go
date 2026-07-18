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
