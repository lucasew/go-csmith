package csmith

import "testing"

// StatementFor.cpp:193–194 — write_var(var); read_var(var) before init.visit_facts.
// Generation post_loop pre_effect → map_stm_effect[for] → function feffect reads.
// seed-46: after FP re-analysis, Go lost g_952.f8 from func_44 reads while UP kept it
// (statement shortcut same_facts false on Go → visit_facts wiped gen header reads).
func TestMakeIterationEffectStmReadsAndWritesIV(t *testing.T) {
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	probs := NewProbabilities(opts)
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
		{Name: "f2", Type: GetIntType(), BitWidth: -1},
		{Name: "f3", Type: GetIntType(), BitWidth: -1},
		{Name: "f4", Type: GetIntType(), BitWidth: -1},
		{Name: "f5", Type: GetIntType(), BitWidth: -1},
		{Name: "f6", Type: GetIntType(), BitWidth: -1},
		{Name: "f7", Type: GetIntType(), BitWidth: -1},
		{Name: "f8", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_s", st, false, false)
	parent.CreateFieldVars()
	if len(parent.FieldVars) < 9 {
		t.Fatal("need f8")
	}
	iv := parent.FieldVars[8]
	fn := &Function{Name: "func_44", ReturnType: GetIntType()}
	blk := &Block{Func: fn}
	fn.Body = blk
	fn.Blocks = []*Block{blk}
	fm := NewFactMgr(fn)
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{}
	vs := NewVariableSelector(opts)
	vs.GlobalList = append(vs.GlobalList, parent)
	for _, f := range parent.FieldVars {
		vs.AllVars = append(vs.AllVars, f)
	}
	vs.AllVars = append(vs.AllVars, parent)
	cg := WithFunc(fn, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect()
	cg.EffectStm = EmptyEffect()
	fn.Stack = []*Block{blk}

	var lc *LoopControl
	for seed := uint64(1); seed < 500; seed++ {
		ClearError()
		cg.EffectStm = EmptyEffect()
		*cg.EffectAccum = EmptyEffect()
		lc = MakeIteration(NewRng(seed), opts, probs, vs, &cg)
		if lc == nil || HasError() || lc.IV == nil {
			continue
		}
		if lc.IV == iv || lc.IV.FieldVarOf == parent {
			break
		}
		lc = nil
	}
	if lc == nil || lc.IV == nil {
		t.Skip("could not select struct-field IV")
	}
	if !cg.EffectStm.IsWritten(lc.IV) || !cg.EffectStm.IsRead(lc.IV) {
		t.Fatalf("make_iteration must write+read IV, IsW=%v IsR=%v",
			cg.EffectStm.IsWritten(lc.IV), cg.EffectStm.IsRead(lc.IV))
	}
	body := &Block{Func: fn, StmID: AllocStmID()}
	fm.SetMapStmEffect(body.StmID, EmptyEffect())
	fm.SetMapFactsInPair(body.StmID, []*FactPointTo{}, []*FactUnion{})
	forSt := &Stmt{Kind: StmtFor, Loop: lc, Then: body, StmID: AllocStmID()}
	pre := cg.EffectStm.Clone()
	postLoopAnalysis(fm, forSt, body, []*FactPointTo{}, []*FactUnion{}, pre, &cg)
	got := fm.GetMapStmEffect(forSt.StmID)
	if !got.IsRead(lc.IV) {
		t.Fatal("post_loop map_stm_effect[for] must retain IV read from pre_effect")
	}
	ClearError()
}
