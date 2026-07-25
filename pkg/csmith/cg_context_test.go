package csmith

import "testing"

func TestCloneSubcontextDeepCopiesIVBounds(t *testing.T) {
	// CGContext.cpp copy ctor deep-copies iv_bounds map
	ClearErrorSess(testAmbientSession)
	iv1 := CreateVariableScalarsSess(testAmbientSession, "i1", GetIntTypeSess(testAmbientSession), false, false)
	iv2 := CreateVariableScalarsSess(testAmbientSession, "i2", GetIntTypeSess(testAmbientSession), false, false)
	parent := EmptyCGContext().WithSession(testAmbientSession)
	parent.AddIVBound(iv1, 3)
	child := parent.CloneSubcontext()
	child.AddIVBound(iv2, 5)
	if _, ok := parent.IVBounds[iv2]; ok {
		t.Fatal("parent must not see child AddIVBound")
	}
	if parent.IVBounds[iv1] != 3 {
		t.Fatal("parent iv1")
	}
	if child.IVBounds[iv1] != 3 || child.IVBounds[iv2] != 5 {
		t.Fatal(child.IVBounds)
	}
	child.RemoveIVBound(iv1)
	if _, ok := parent.IVBounds[iv1]; !ok {
		t.Fatal("parent must keep iv1 after child Remove")
	}
	// WithFlags also isolates
	parent2 := EmptyCGContext().WithSession(testAmbientSession)
	parent2.AddIVBound(iv1, 1)
	loop := parent2.WithFlags(FlagInLoop)
	loop.AddIVBound(iv2, 2)
	if _, ok := parent2.IVBounds[iv2]; ok {
		t.Fatal("WithFlags body must not share IVBounds map")
	}
	if !loop.InLoop() {
		t.Fatal("FlagInLoop")
	}
	ClearErrorSess(testAmbientSession)
}

// Statement.cpp:612 — stm_visit_facts sets curr_blk = parent before visit_facts.
func TestStmVisitFactsSetsCurrBlk(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	parent := &Block{Func: f, StmID: AllocStmID()}
	st := &Stmt{Kind: StmtReturn, StmID: AllocStmID(), Expr: &Expression{
		Term: TermConstant, Con: MakeInt(0), ExprType: GetIntTypeSess(testAmbientSession),
	}}
	// Return with const may not need RV; still exercise CurrBlk assignment path
	// via ValidateAndUpdateFacts which sets CurrBlk before StmVisitFacts.
	fm := NewFactMgrSess(testAmbientSession, f)
	f.RV = CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false)
	f.ReturnType = GetIntTypeSess(testAmbientSession)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// Valid return expr: variable not pointing to locals
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st.Expr = &Expression{Term: TermVariable, Var: g, ExprType: GetIntTypeSess(testAmbientSession)}
	fm.AddNewVarFact(g)
	facts := CloneFactSlice(fm.GlobalFacts)
	// Parent block must be on stack for other paths; CurrBlk set from blk arg
	f.Stack = []*Block{parent}
	_ = ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), parent)
	if cg.CurrBlk != parent {
		t.Fatalf("CurrBlk must be statement parent after validate, got %v want %v", cg.CurrBlk, parent)
	}
	ClearErrorSess(testAmbientSession)
}

// CGContext.cpp:95–105 — loop-body ctor: expr_depth=0, curr_rhs=nil, empty stm,
// share EffectAccum, IN_LOOP, optional iv bound.
func TestWithLoopBodyMatchesCtor(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	eff := EmptyEffect()
	parent := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	parent.EffectAccum = &eff
	parent.ExprDepth = 9
	parent.CurrRHS = rhs
	parent.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, iv)
	parent.Flags = FlagNoDanglingPtr
	body := parent.WithLoopBody(parent.RW, iv, 4)
	if !body.InLoop() {
		t.Fatal("must set IN_LOOP")
	}
	if body.Flags&FlagNoDanglingPtr == 0 {
		t.Fatal("must keep parent flags")
	}
	if body.ExprDepth != 0 {
		t.Fatalf("expr_depth must be 0, got %d", body.ExprDepth)
	}
	if body.CurrRHS != nil {
		t.Fatal("curr_rhs must be nil")
	}
	if body.EffectAccum != parent.EffectAccum {
		t.Fatal("must share effect_accum pointer")
	}
	if len(body.EffectStm.WrittenVarsSess(testAmbientSession)) != 0 {
		t.Fatal("effect_stm must start empty")
	}
	if b, ok := body.IVBounds[iv]; !ok || b != 4 {
		t.Fatalf("iv_bounds[iv]=4, got %v ok=%v", b, ok)
	}
	if _, ok := parent.IVBounds[iv]; ok {
		t.Fatal("parent iv_bounds must not gain body IV")
	}
	// parent ExprDepth/CurrRHS unchanged
	if parent.ExprDepth != 9 || parent.CurrRHS != rhs {
		t.Fatal("parent must not be mutated")
	}
	ClearErrorSess(testAmbientSession)
}
