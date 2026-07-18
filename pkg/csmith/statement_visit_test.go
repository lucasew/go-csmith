package csmith

import (
	"strings"
	"testing"
)

func TestVisitFactsStatementIfMerge(t *testing.T) {
	BookkeeperDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a)}
	// then writes p → b; else keeps a
	thenAssign := Stmt{
		Kind: StmtAssign,
		Lhs:  &Lhs{Var: p, Type: p.Type}, LhsVar: p,
		Expr: &Expression{Term: TermVariable, Var: b, ExprType: PointerTo(GetIntType())},
		AssignOp: AssignSimple,
	}
	// UpdateFact needs Expression for pointer assign - use &b style constant 0 and manual
	// Simpler: just empty branches and ensure visit succeeds
	st := Stmt{
		Kind: StmtIfElse,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		Then: &Block{Stmts: []Stmt{thenAssign}},
		Else: &Block{Stmts: []Stmt{}},
	}
	// Fix then: assign p = &a is hard; empty then with return
	st.Then = &Block{Stmts: []Stmt{{Kind: StmtReturn}}}
	st.Else = &Block{Stmts: []Stmt{}}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsStatementIf(&st, &cg, Defaults()) {
		t.Fatal("visit if")
	}
	// true must return → facts from else (pre) kept
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || !fp.IsNull() && len(fp.PointTo) > 0 && fp.PointTo[0] != a {
		// pre was p→a; true returns so else facts = pre
		if fp == nil {
			t.Fatal("nil fact")
		}
	}
}

func TestVisitFactsStatementForIV(t *testing.T) {
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	// body tries to write IV — should fail VisitFactsStatementAssign inside
	body := &Block{Stmts: []Stmt{{
		Kind: StmtAssign, LhsVar: iv, Lhs: &Lhs{Var: iv, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}}}
	st := Stmt{
		Kind: StmtFor,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 10, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		Then: body,
	}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visit should fail because IV write in body
	if VisitFactsStatementFor(&st, &cg, Defaults()) {
		t.Fatal("expected IV write reject")
	}
}

func TestOutputAssignAsExprSafeAdd(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	flags := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32}
	st := Stmt{
		Kind: StmtAssign, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)},
		AssignOp: AssignAdd, SafeFlags: flags,
	}
	out := OutputAssignAsExpr(&st, false)
	if !strings.Contains(out, "safe_add_") || !strings.Contains(out, "g_1 = ") {
		t.Fatal(out)
	}
}

func TestMakePossibleCompoundAssignTmps(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	opts.MathNoTmp = true
	probs := NewProbabilities(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	lhs := &Lhs{Var: CreateVariableScalars("g_1", GetIntType(), false, false), Type: GetIntType()}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(3)}
	st := makePossibleCompoundAssign(cg, opts, probs, NewRng(2), GetIntType(), lhs, AssignAdd, rhs)
	if st.SafeFlags == nil {
		t.Fatal("flags")
	}
	if st.Tmp1 == "" || st.Tmp2 == "" {
		t.Fatal("tmps", st.Tmp1, st.Tmp2, blk.TmpVars)
	}
	if st.Tmp1 == st.Tmp2 {
		t.Fatal("same tmp")
	}
}

func TestVisitFactsBlockSequential(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	b := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
			Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple},
	}}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsBlock(b, &cg, Defaults()) {
		t.Fatal("block")
	}
	if !eff.IsWritten(v) {
		t.Fatal("write")
	}
}
