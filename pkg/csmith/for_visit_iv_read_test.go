package csmith

import "testing"

// Gen map_stm includes make_iteration read_var(IV). VisitFacts re-walks init+body
// only (StatementFor.cpp:430–468) and overwrites map_stm — pure gen IV *read*
// must not survive when body does not re-introduce it.
func TestVisitFactsForDropsGenOnlyIVRead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), false, false)
	fn := &Function{Name: "func_x", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: fn, StmID: AllocStmIDSess(testAmbientSession)}
	fn.Body = blk
	fn.Blocks = []*Block{blk}
	fn.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, fn)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{}
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = append(vs.GlobalList, g)
	vs.AllVars = append(vs.AllVars, g)
	cg := WithFunc(fn, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	acc := EmptyEffect()
	cg.EffectAccum = &acc
	cg.EffectStm = EmptyEffect()
	tables := NewExprTablesSess(testAmbientSession, opts)
	stmtTab := buildStatementThresholdTable(testAmbientSession, opts)

	var forSt *Stmt
	for seed := uint64(1); seed < 300; seed++ {
		ClearErrorSess(testAmbientSession)
		acc = EmptyEffect()
		cg.EffectAccum = &acc
		cg.EffectStm = EmptyEffect()
		fm.GlobalFacts = []*FactPointTo{}
		fm.MapStmEffect = map[int]Effect{}
		fm.MapVisited = map[int]bool{}
		forSt = MakeRandomFor(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, stmtTab, &cg)
		if forSt != nil && !HasErrorSess(testAmbientSession) && forSt.Loop != nil && forSt.Loop.IV != nil && forSt.Then != nil {
			if fm.GetMapStmEffect(forSt.StmID).IsReadSess(testAmbientSession, forSt.Loop.IV) {
				break
			}
		}
		forSt = nil
	}
	if forSt == nil {
		t.Fatal("no for with gen IV read")
	}
	iv := forSt.Loop.IV
	// Strip body statements so re-visit cannot re-read IV from body IR
	forSt.Then.Stmts = nil
	fm.SetMapStmEffect(forSt.Then.StmID, EmptyEffect())

	genMS := fm.GetMapStmEffect(forSt.StmID)
	if !genMS.IsReadSess(testAmbientSession, iv) {
		t.Fatal("gen map_stm must read IV")
	}

	ClearErrorSess(testAmbientSession)
	cg.EffectStm = EmptyEffect()
	acc2 := EmptyEffect()
	cg.EffectAccum = &acc2
	fm.GlobalFacts = []*FactPointTo{}
	if !VisitFactsStatementFor(forSt, &cg, opts) {
		t.Fatalf("VisitFactsStatementFor: %v sticky=%v", GetErrorSess(testAmbientSession), HasErrorSess(testAmbientSession))
	}
	visitMS := fm.GetMapStmEffect(forSt.StmID)
	t.Logf("gen read=%v write=%v → visit read=%v write=%v",
		genMS.IsReadSess(testAmbientSession, iv), genMS.IsWrittenSess(testAmbientSession, iv),
		visitMS.IsReadSess(testAmbientSession, iv), visitMS.IsWrittenSess(testAmbientSession, iv))
	if visitMS.IsReadSess(testAmbientSession, iv) {
		// init assign is g_iv = const — should not read IV; empty body — no body read
		t.Fatal("VisitFacts with empty body must drop gen-only IV read from map_stm")
	}
	if !visitMS.IsWrittenSess(testAmbientSession, iv) {
		t.Fatal("visit must write IV via init")
	}
	ClearErrorSess(testAmbientSession)
}
