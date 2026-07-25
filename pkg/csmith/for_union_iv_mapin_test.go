package csmith

import "testing"

// StatementFor.cpp:244–245 + Block.cpp:105 — make_iteration visits init (eUnionWrite
// renew to IV field) before Block::make_random set_fact_in(body, global_facts).
// Soft invent left map_facts_in[body] at pre-init last_write (f0) while live was f1
// after init → post_creation FP merged entry f0 with body/continue f1 → BOTTOM
// (seed-177 g_88 choose ok n=29 vs UP n=30 with g_88.f0).
func TestForUnionFieldIVBodyMapInMatchesInitLastWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalizationSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	r := NewRngSess(testAmbientSession, 177)
	vs := NewVariableSelector(testAmbientSession, opts)
	// union with two simple fields
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	g88 := CreateVariableQferSess(testAmbientSession, "g_88", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	g88.CreateFieldVarsSess(testAmbientSession)
	if len(g88.FieldVars) < 2 {
		t.Fatal("field_vars")
	}
	if g88.FieldVars[0].GetFieldIDSess(testAmbientSession) != 0 || g88.FieldVars[1].GetFieldIDSess(testAmbientSession) != 1 {
		t.Fatalf("GetFieldID f0=%d f1=%d", g88.FieldVars[0].GetFieldIDSess(testAmbientSession), g88.FieldVars[1].GetFieldIDSess(testAmbientSession))
	}
	// pre-init last_write f0 (abstract init of union)
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(g88, 0)}
	fm.GlobalFacts = []*FactPointTo{}
	outer := &Block{StmID: AllocStmID(), Func: f, Looping: false}
	f.Body = outer
	f.Stack = []*Block{outer}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	f.Stack = []*Block{outer}
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// Force IV to be g_88.f1 by planting as only suitable induction candidate?
	// MakeIteration selects IV via VariableSelector — may not pick g_88.f1.
	// Directly exercise: VisitFactsStatementAssign of f1 then MakeRandomBlock.
	iv := g88.FieldVars[1]
	initSt := &Stmt{
		Kind: StmtAssign, LhsVar: iv, Lhs: &Lhs{Var: iv, Type: iv.Type},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
		AssignOp: AssignSimple, StmID: AllocStmID(),
	}
	if !VisitFactsStatementAssign(initSt, &cg, opts) {
		t.Fatal("init visit", GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(fm.UnionFacts, g88)
	if got == nil || got.LastWrittenFID != 1 {
		t.Fatalf("after f1 IV init want last=1, got %#v", got)
	}
	// Block entry must snapshot post-init lattice
	body := MakeRandomBlock(r, opts, probs, vs, nil, nil, &cg, true)
	if body == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("body", GetErrorSess(testAmbientSession))
	}
	inU := fm.GetMapUnionFactsIn(body.StmID)
	gotIn := FindRelatedUnion(inU, g88)
	if gotIn == nil {
		t.Fatal("map_in missing g_88")
	}
	if gotIn.LastWrittenFID != 1 {
		t.Fatalf("map_facts_in[body] must be post-init last=1, got %d (live=%d)",
			gotIn.LastWrittenFID, got.LastWrittenFID)
	}
}
