package csmith

import (
	"testing"
)

func TestErrorCodes(t *testing.T) {
	ClearError()
	if HasError() || GetError() != ErrSuccess {
		t.Fatal(GetError())
	}
	SetError(ErrCompatibleCheck)
	if !HasError() || GetError() != ErrCompatibleCheck {
		t.Fatal(GetError())
	}
	ClearError()
}

func TestSafeAssign(t *testing.T) {
	if !SafeAssign(AssignBitAnd) || SafeAssign(AssignAdd) {
		t.Fatal("safe")
	}
}

func TestVisitFactsStatementAssignSimple(t *testing.T) {
	BookkeeperDoFinalization()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(3)}
	st := Stmt{Kind: StmtAssign, LhsVar: v, Lhs: lhs, Expr: rhs, AssignOp: AssignSimple}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !VisitFactsStatementAssign(&st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsWritten(v) {
		t.Fatal("lhs write missing")
	}
}

func TestVisitFactsStatementAssignNoWriteToIV(t *testing.T) {
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	lhs := &Lhs{Var: iv, Type: GetIntType()}
	st := Stmt{
		Kind: StmtAssign, LhsVar: iv, Lhs: lhs,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	cg := EmptyCGContext()
	cg.AddIVBound(iv, 10)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if VisitFactsStatementAssign(&st, &cg, Defaults()) {
		t.Fatal("should reject IV write")
	}
}

func TestMakeRandomAssignDualContext(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	vs.Types = &TypeEnv{}
	// seed a global
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	if g == nil {
		t.Fatal("global")
	}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	tables := NewExprTables(opts)
	st := MakeRandomAssign(NewRng(5), opts, probs, vs, tables, cg, GetIntType())
	if st.Kind != StmtAssign {
		t.Fatal("kind")
	}
	if st.LhsVar == nil && st.Lhs == nil {
		t.Fatal("no lhs", st)
	}
}

func TestVisitFactsExpressionComma(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()},
		CommaRHS: &Expression{Term: TermVariable, Var: b, ExprType: GetIntType()},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !VisitFactsExpression(e, &cg, Defaults()) {
		t.Fatal("comma")
	}
	if !eff.IsRead(a) || !eff.IsRead(b) {
		t.Fatal("reads", eff.IsRead(a), eff.IsRead(b))
	}
}
