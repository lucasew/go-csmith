// Upstream: Statement.cpp find_edges_in / analyze_with_edges_in;
// FactMgr::merge_jump_facts; set_accumulated_effect_after_block.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MergeJumpFacts mirrors FactMgr::merge_jump_facts.
// FactMgr.cpp:569–588 — for each non-rv fact, join related jump fact (or garbage).
func MergeJumpFacts(facts *[]*FactPointTo, jumpFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	changed := false
	// iterate a snapshot of subjects so we can grow via MergeFactInto
	subjects := append([]*FactPointTo(nil), *facts...)
	for _, f := range subjects {
		if f == nil || f.Var == nil {
			continue
		}
		// skip return variables (*_rv)
		if isReturnVar(f.Var) {
			continue
		}
		jumpF := FindRelatedPointTo(jumpFacts, f.Var)
		if jumpF == nil {
			// jump over initializer → garbage
			jumpF = MakeFactPointTo(f.Var, GarbagePtr)
		}
		before := FindRelatedPointTo(*facts, f.Var)
		*facts = MergeFactInto(*facts, jumpF)
		after := FindRelatedPointTo(*facts, f.Var)
		if before == nil || after == nil || !before.Equal(after) {
			changed = true
		}
	}
	return changed
}

func isReturnVar(v *Variable) bool {
	if v == nil {
		return false
	}
	// Function RV names end with _rv
	n := v.Name
	return len(n) > 3 && n[len(n)-3:] == "_rv"
}

// FindEdgesIn mirrors Statement::find_edges_in for dest StmID.
// Statement.cpp:453–467 — edges with matching dest, post_dest, back_link.
func (fm *FactMgr) FindEdgesIn(destStmID int, postDest, backLink bool) []*CFGEdge {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	var out []*CFGEdge
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestStmID == destStmID && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// FindEdgesInToBlock finds edges whose DestBlock matches (break/continue).
func (fm *FactMgr) FindEdgesInToBlock(dest *Block, postDest, backLink bool) []*CFGEdge {
	if fm == nil || dest == nil {
		return nil
	}
	var out []*CFGEdge
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestBlock == dest && e.DestStmID == 0 && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// HasEdgeIn mirrors Statement::has_edge_in.
func (fm *FactMgr) HasEdgeIn(destStmID int, postDest, backLink bool) bool {
	return len(fm.FindEdgesIn(destStmID, postDest, backLink)) > 0
}

// AnalyzeWithEdgesIn mirrors Statement::analyze_with_edges_in.
// Statement.cpp:808–834 — merge visited jump sources then validate_and_update.
func AnalyzeWithEdgesIn(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	if st == nil || facts == nil || cg == nil {
		return true
	}
	fm := cg.FM
	if fm != nil && st.StmID > 0 {
		// back edges only if already visited
		if fm.MapVisited != nil && fm.MapVisited[st.StmID] {
			for _, e := range fm.FindEdgesIn(st.StmID, false, true) {
				if e == nil || fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
					continue
				}
				if out, ok := fm.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(facts, out)
				}
				if acc, ok := fm.MapAccumEffect[e.SrcID]; ok {
					cg.AddEffect(acc, false)
				}
			}
		}
		// always consider forward edges
		for _, e := range fm.FindEdgesIn(st.StmID, false, false) {
			if e == nil || fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
				continue
			}
			if out, ok := fm.MapFactsOut[e.SrcID]; ok {
				MergeJumpFacts(facts, out)
			}
			if acc, ok := fm.MapAccumEffect[e.SrcID]; ok {
				cg.AddEffect(acc, false)
			}
		}
	}
	return ValidateAndUpdateFacts(st, facts, cg, opts, blk)
}

// SetAccumulatedEffectAfterBlock mirrors Statement::set_accumulated_effect_after_block.
// Statement.cpp:515–520 — eff += block effect; store as this statement's effect.
func SetAccumulatedEffectAfterBlock(st *Stmt, blockEffect Effect, cg *CGContext, preStm Effect) {
	if st == nil || cg == nil || cg.FM == nil {
		return
	}
	eff := preStm.AddEffect(blockEffect)
	if st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, eff)
	}
}

// PostCreationAnalysis mirrors Statement::post_creation_analysis light path.
// Statement.cpp:845–870 — makeup new var facts; save simple stmt effect.
func PostCreationAnalysis(st *Stmt, preFacts []*FactPointTo, preEffect Effect, cg *CGContext) {
	if st == nil || cg == nil || cg.FM == nil {
		return
	}
	fm := cg.FM
	if st.Kind == StmtIfElse {
		// combine_branch_facts already done during make_random / visit
	} else {
		MakeupNewVarFacts(&preFacts, fm.GlobalFacts)
		_ = preFacts
	}
	// simple statements: save effect_stm
	if !IsCompound(st.Kind) {
		if st.StmID > 0 {
			fm.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
	}
	_ = preEffect
}

// FindFixedPointBlock is a light Block::find_fixed_point (one or two passes).
// Block.cpp:513+ — sequential analyze_with_edges_in; optional second pass for loops.
// failIndex is the statement index that failed analyze_with_edges_in, or -1.
func FindFixedPointBlock(b *Block, inputs []*FactPointTo, cg *CGContext, opts Options, visitOnce bool) (facts []*FactPointTo, failIndex int, ok bool) {
	if b == nil || cg == nil {
		return inputs, -1, true
	}
	facts = CloneFactSlice(inputs)
	// push block
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.Stack = append(cg.CurrentFunc.Stack, b)
		defer func() {
			if f := cg.CurrentFunc; f != nil && len(f.Stack) > 0 {
				f.Stack = f.Stack[:len(f.Stack)-1]
			}
		}()
	}
	pass := func() (int, bool) {
		cur := CloneFactSlice(facts)
		for i := range b.Stmts {
			if !AnalyzeWithEdgesIn(&b.Stmts[i], &cur, cg, opts, b) {
				return i, false
			}
		}
		facts = cur
		return -1, true
	}
	if idx, passOK := pass(); !passOK {
		return facts, idx, false
	}
	// loop body or back edges: second pass
	needSecond := b.Looping || visitOnce
	if !needSecond && cg.FM != nil {
		for _, e := range cg.FM.CFGEdges {
			if e != nil && e.BackLink && e.DestBlock == b {
				needSecond = true
				break
			}
		}
	}
	if needSecond {
		if idx, passOK := pass(); !passOK {
			return facts, idx, false
		}
	}
	if cg.FM != nil {
		if b.StmID > 0 {
			cg.FM.SetMapFactsIn(b.StmID, inputs)
			cg.FM.SetMapFactsOut(b.StmID, facts)
		}
		cg.FM.GlobalFacts = facts
		b.SetAccumulatedEffect(cg.FM)
	}
	return facts, -1, true
}
