package csmith

import "testing"

// CGContext.cpp:175–185 — read_var updates both effect_accum and effect_stm.
// After MakeRandomAssign, EffectAccum must include every EffectStm read.
func TestAssignGenAccumIncludesStmReads(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelectorProbs(opts, probs)
	vs.Sess = testAmbientSession
	for i := 0; i < 8; i++ {
		g := CreateVariableScalars("g_"+string(rune('a'+i)), GetIntType(), false, false)
		if g != nil {
			vs.GlobalList = append(vs.GlobalList, g)
			vs.AllVars = append(vs.AllVars, g)
		}
	}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	for _, g := range vs.GlobalList {
		fm.AddNewVarFact(g)
	}
	blk := &Block{Func: f, StmID: AllocStmID()}
	f.Body = blk
	f.Blocks = []*Block{blk}
	f.Stack = []*Block{blk}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	tables := NewExprTables(opts)

	checked := 0
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		cg.EffectStm = EmptyEffect()
		st := MakeRandomAssign(NewRng(seed), opts, probs, vs, tables, &cg, nil)
		if st.Kind != StmtAssign {
			continue
		}
		if !EffectComplete(cg.EffectStm) || cg.EffectAccum == nil || !EffectComplete(*cg.EffectAccum) {
			ClearErrorSess(testAmbientSession)
			continue
		}
		checked++
		for _, v := range cg.EffectStm.ReadVars() {
			if v == nil {
				continue
			}
			if !cg.EffectAccum.IsRead(v) {
				t.Fatalf("seed %d: EffectStm read %s not in EffectAccum (stm=%v accum=%v)",
					seed, v.Name,
					mapAccumNamesOf(cg.EffectStm.ReadVars()),
					mapAccumNamesOf(cg.EffectAccum.ReadVars()))
			}
		}
	}
	if checked < 3 {
		t.Fatalf("too few assigns checked: %d", checked)
	}
	ClearErrorSess(testAmbientSession)
}
