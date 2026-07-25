package csmith

import "testing"

// StatementFor.cpp:194–299 / 369 — make_iteration write_var+read_var(IV) is in
// pre_effect; set_accumulated_effect_after_block stores it on map_stm_effect[for].
// VisitFactsStatementFor only re-walks init (write) and can drop the gen IV read
// from map_stm; FEffect still sees gen IV reads via effect_accum when need_revisit
// FP re-injects gen map_accum along back edges (Statement.cpp:817–820).
func TestMakeRandomForMapStmHasIVRead(t *testing.T) {
	ClearError()
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	g := CreateVariableScalars("g_iv", GetIntType(), false, false)
	fn := &Function{Name: "func_x", ReturnType: GetIntType()}
	blk := &Block{Func: fn, StmID: AllocStmID()}
	fn.Body = blk
	fn.Blocks = []*Block{blk}
	fn.Stack = []*Block{blk}
	fm := NewFactMgr(fn)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{}
	vs := NewVariableSelector(opts)
	vs.GlobalList = append(vs.GlobalList, g)
	vs.AllVars = append(vs.AllVars, g)
	cg := WithFunc(fn, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect()
	cg.EffectStm = EmptyEffect()
	tables := NewExprTables(opts)
	stmtTab := buildStatementThresholdTable(opts)

	var forSt *Stmt
	for seed := uint64(1); seed < 200; seed++ {
		ClearError()
		*cg.EffectAccum = EmptyEffect()
		cg.EffectStm = EmptyEffect()
		fm.GlobalFacts = []*FactPointTo{}
		forSt = MakeRandomFor(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
		if forSt == nil || HasError() || forSt.Loop == nil || forSt.Loop.IV == nil {
			continue
		}
		break
	}
	if forSt == nil || forSt.Loop == nil || forSt.Loop.IV == nil {
		t.Skip("no for")
	}
	iv := forSt.Loop.IV
	ms := fm.GetMapStmEffect(forSt.StmID)
	if !ms.IsWritten(iv) {
		t.Fatal("map_stm must write IV (init assign)")
	}
	if !ms.IsRead(iv) {
		t.Fatal("map_stm must read IV (make_iteration read_var) — StatementFor.cpp:194")
	}
	if cg.EffectAccum == nil || !cg.EffectAccum.IsRead(iv) {
		t.Fatal("effect_accum must read IV after make_iteration")
	}
	ClearError()
}
