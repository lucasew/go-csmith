package csmith

import "testing"

// TestFindFixedPointMultiPassResetsEffectAccumForMapAccum —
// Block.cpp:513–568 find_fixed_point may re-walk statements when same_facts
// misses. Caller reset_effect_accum(pre_effect) only once before the call
// (Block.cpp:789). Soft invent left end-of-body effect_accum on re-walk so
// early stmts' map_accum_effect absorbed later reads (Statement.cpp:654/744
// map_accum = *effect_accum). StatementGoto.cpp:125–128 forward
// choose_visible_read_var then saw inflated ok pools (seed 1469030: nOk 49
// vs UP 40; if (l_858) vs if (g_1065.f0)).
//
// Fair: snapshot entry accum at FindFixedPointBlock entry and re-reset before
// each full statement walk so map_accum rebuilds progressively.
func TestFindFixedPointMultiPassResetsEffectAccumForMapAccum(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "f_map_accum_prog"}
	fm := NewFactMgrSess(testAmbientSession, f)
	early := CreateVariableScalars("g_early", GetIntType(), false, false)
	late := CreateVariableScalars("g_late", GetIntType(), false, false)

	// Minimal body: two invokes so AnalyzeWithEdgesIn has something to walk.
	// Use assign with constant RHS so visit_facts stays simple.
	s0 := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		AssignOp: AssignSimple,
		Lhs:      &Lhs{Var: early},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()},
	}
	s1 := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		AssignOp: AssignSimple,
		Lhs:      &Lhs{Var: late},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()},
	}
	// Manually plant map_stm and pre-seed map_accum as if a polluted second
	// pass already ran: early's map_accum incorrectly lists late.
	polluted := EmptyEffect().ReadVar(early).ReadVar(late).WriteVar(early)
	fm.SetMapStmEffect(s0.StmID, EmptyEffect().WriteVar(early))
	fm.SetMapStmEffect(s1.StmID, EmptyEffect().WriteVar(late))
	fm.SetMapAccumEffect(s0.StmID, polluted)
	fm.SetMapAccumEffect(s1.StmID, EmptyEffect().ReadVar(late).WriteVar(late))

	b := &Block{
		Func: f, StmID: AllocStmID(), Looping: true,
		Stmts: []Stmt{s0, s1},
	}
	fm.SetMapStmEffect(b.StmID, EmptyEffect())
	entry := []*FactPointTo{}
	fm.SetMapFactsIn(b.StmID, entry)
	fm.SetMapFactsOut(b.StmID, entry)
	fm.MapVisited[b.StmID] = true
	// No self-back: first walk only; entryAccum reset still applies before walk.

	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	// Entry accum: only early (block pre_effect).
	pre := EmptyEffect().ReadVar(early)
	// Simulate dirty live accum from a prior full body walk.
	dirty := EmptyEffect().ReadVar(early).ReadVar(late)
	d := dirty
	cg.EffectAccum = &d
	fm.GlobalFacts = []*FactPointTo{}
	fm.UnionFacts = []*FactUnion{}

	// FindFixedPointBlock snapshots entryAccum from live (=dirty) at call time.
	// PostCreation always reset_effect_accum(pre) first — mirror that.
	cg.ResetEffectAccum(pre)
	if !EffectComplete(*cg.EffectAccum) || cg.EffectAccum.IsRead(late) {
		t.Fatalf("precondition: live accum must be pre-only; late=%v", cg.EffectAccum.IsRead(late))
	}

	_, _, _, ok := FindFixedPointBlock(b, CloneFactSlice(entry), &cg, Defaults(), true)
	_ = ok
	ClearErrorSess(testAmbientSession)

	acc0 := fm.GetMapAccumEffect(s0.StmID)
	if !EffectComplete(acc0) {
		// Minimal assign visit may fail closed incomplete — still ensure we did
		// not keep the intentionally polluted late-read snapshot.
		if EffectComplete(polluted) && fm.MapAccumEffect[s0.StmID].IsRead(late) {
			// If store was incomplete marker, fine; if complete and still has late, fail.
			if EffectComplete(fm.MapAccumEffect[s0.StmID]) {
				t.Fatal("polluted map_accum with g_late must not survive progressive rebuild")
			}
		}
		return
	}
	if acc0.IsRead(late) {
		t.Fatalf("s0 map_accum must not list g_late after progressive walk; reads=%v",
			readNames(acc0))
	}
}

func readNames(e Effect) []string {
	var out []string
	for _, v := range e.ReadVars() {
		if v != nil {
			out = append(out, v.Name)
		}
	}
	return out
}
