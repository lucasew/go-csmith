package csmith

import "testing"

// TestShortcutAnalysisBlockMissingMapInIsEmpty — Statement.cpp:545–567 /
// Block.cpp:772 — C++ map_facts_in[this] default-inserts empty FactVec.
// Soft invent treated missing MapFactsIn as ShortcutNone, forcing full re-visit
// of loop bodies on first post_creation find_fixed_point and rewriting gen
// map_stm_effect (seed-90 nested-call IV reads dropped from caller feffect).
func TestShortcutAnalysisBlockMissingMapInIsEmpty(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	b := &Block{Func: f, StmID: AllocStmIDSess(testAmbientSession), Looping: true, Stmts: []Stmt{}}
	f.Body = b
	fm := NewFactMgrSess(testAmbientSession, f)
	// no MapFactsIn entry for b — C++ empty
	// set map_out + map_stm so later steps can succeed when same_facts holds
	fm.SetMapFactsOut(b.StmID, []*FactPointTo{})
	fm.SetMapStmEffect(b.StmID, EmptyEffect())
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect()
	cg.EffectStm = EmptyEffect()
	facts := []*FactPointTo{}
	sc := ShortcutAnalysisBlock(b, &facts, &cg)
	if sc != ShortcutOK {
		t.Fatalf("missing map_in + empty inputs must ShortcutOK (C++ map[] empty), sc=%d err=%v", sc, GetErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
