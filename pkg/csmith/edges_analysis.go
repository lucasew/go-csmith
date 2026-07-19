// Upstream: Statement.cpp find_edges_in / analyze_with_edges_in;
// FactMgr::merge_jump_facts; set_accumulated_effect_after_block.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MergeJumpFacts mirrors FactMgr::merge_jump_facts.
// FactMgr.cpp:569–588 — for each non-rv fact, join related jump fact (or garbage).
// Fact* always live in maps; nil subject/jump holes fail closed (*facts nil —
// no invent skip holes or leave partial mid-join). Returns whether any fact changed.
// Incomplete / mid-join failure clears *facts and returns false (same as no-change);
// callers that need to distinguish use tryMergeJumpFacts.
func MergeJumpFacts(facts *[]*FactPointTo, jumpFacts []*FactPointTo) bool {
	changed, ok := tryMergeJumpFacts(facts, jumpFacts)
	return ok && changed
}

// tryMergeJumpFacts merges jump outs into facts; fails closed on incomplete maps.
// Distinguishes incomplete (ok=false, *facts nil) from complete no-change
// (ok=true, changed=false). Mid-join MergeFactInto nil clears *facts (no invent
// leave partial join as ok success).
func tryMergeJumpFacts(facts *[]*FactPointTo, jumpFacts []*FactPointTo) (changed, ok bool) {
	if facts == nil {
		return false, false
	}
	// pre-validate: incomplete maps must not soft-join past holes
	if !FactsComplete(*facts) || !FactsComplete(jumpFacts) {
		*facts = nil
		return false, false
	}
	// iterate a snapshot of subjects so we can grow via MergeFactInto
	subjects := append([]*FactPointTo(nil), *facts...)
	for _, f := range subjects {
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
		merged := MergeFactInto(*facts, jumpF)
		if merged == nil {
			// mid-join incomplete — clear partial, no invent keep half-merged map
			*facts = nil
			return false, false
		}
		*facts = merged
		after := FindRelatedPointTo(*facts, f.Var)
		if before == nil || after == nil || !before.Equal(after) {
			changed = true
		}
	}
	return changed, true
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
// CFGEdge* always live; nil hole in CFGEdges → nil (fail closed).
// Complete scan with no matches returns empty non-nil slice.
func (fm *FactMgr) FindEdgesIn(destStmID int, postDest, backLink bool) []*CFGEdge {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	out := make([]*CFGEdge, 0)
	for _, e := range fm.CFGEdges {
		// CFGEdge* always live; no invent skip nil holes as absent edges
		if e == nil {
			return nil
		}
		if e.DestStmID == destStmID && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// FindEdgesInToBlock finds edges whose DestBlock matches (break/continue).
// CFGEdge* always live; nil hole in CFGEdges → nil (fail closed).
// Complete scan with no matches returns empty non-nil slice.
func (fm *FactMgr) FindEdgesInToBlock(dest *Block, postDest, backLink bool) []*CFGEdge {
	if fm == nil || dest == nil {
		return nil
	}
	out := make([]*CFGEdge, 0)
	for _, e := range fm.CFGEdges {
		// CFGEdge* always live; no invent skip nil holes as absent edges
		if e == nil {
			return nil
		}
		if e.DestBlock == dest && e.DestStmID == 0 && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// HasEdgeIn mirrors Statement::has_edge_in.
// Statement.cpp:434–446 — assert(fm); scan cfg_edges.
// Incomplete CFG (FindEdgesIn nil) fails closed true — no invent "no edge"
// from len(nil)==0 soft-skip past hole.
func (fm *FactMgr) HasEdgeIn(destStmID int, postDest, backLink bool) bool {
	if fm == nil {
		// assert(fm) path — fail closed has-edge (conservative revisit)
		return true
	}
	edges := fm.FindEdgesIn(destStmID, postDest, backLink)
	if edges == nil {
		return true
	}
	return len(edges) > 0
}

// AnalyzeWithEdgesIn mirrors Statement::analyze_with_edges_in.
// Statement.cpp:808–834 — merge visited jump sources then validate_and_update.
func AnalyzeWithEdgesIn(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	// Statement.cpp:808+ — always live Statement* + inputs + cg_context
	// no soft invent true on incomplete call
	if st == nil || facts == nil || cg == nil {
		return false
	}
	fm := cg.FM
	if fm != nil {
		// Statement::stm_id always live; StmID 0 fails closed (no invent
		// soft-skip edge merge then validate as complete analysis)
		if st.StmID <= 0 {
			return false
		}
		// back edges only if already visited
		if fm.MapVisited != nil && fm.MapVisited[st.StmID] {
			back := fm.FindEdgesIn(st.StmID, false, true)
			// nil = incomplete CFG (hole); empty non-nil = no matching edges
			if back == nil {
				return false
			}
			for _, e := range back {
				// unvisited src is intentional filter (not soft invent of edge)
				if fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
					continue
				}
				// Statement.cpp:819–820 — always merge_jump_facts / add_effect
				// C++ map[] missing → empty; no invent skip merge when out absent
				// Incomplete out fails closed (no invent partial jump merge)
				out := fm.MapFactsOut[e.SrcID]
				if _, ok := tryMergeJumpFacts(facts, out); !ok {
					return false
				}
				// map_accum_effect[src] — missing → zero Effect (same as skip Add)
				if fm.MapAccumEffect != nil {
					cg.AddEffect(fm.MapAccumEffect[e.SrcID], false)
				}
			}
		}
		// always consider forward edges
		fwd := fm.FindEdgesIn(st.StmID, false, false)
		if fwd == nil {
			return false
		}
		for _, e := range fwd {
			if fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
				continue
			}
			// Statement.cpp:830–831 — always merge_jump_facts / add_effect
			out := fm.MapFactsOut[e.SrcID]
			if _, ok := tryMergeJumpFacts(facts, out); !ok {
				return false
			}
			if fm.MapAccumEffect != nil {
				cg.AddEffect(fm.MapAccumEffect[e.SrcID], false)
			}
		}
	}
	return ValidateAndUpdateFacts(st, facts, cg, opts, blk)
}

// SetAccumulatedEffectAfterBlock mirrors Statement::set_accumulated_effect_after_block.
// Statement.cpp:515–520 — eff += block effect; store as this statement's effect.
// Statement::stm_id always live; StmID 0 is incomplete (no invent soft no-op success
// that leaves map_stm_effect unset while callers treat effect as recorded).
func SetAccumulatedEffectAfterBlock(st *Stmt, blockEffect Effect, cg *CGContext, preStm Effect) {
	if st == nil || cg == nil || cg.FM == nil || st.StmID <= 0 {
		return
	}
	eff := preStm.AddEffect(blockEffect)
	cg.FM.SetMapStmEffect(st.StmID, eff)
}

// PostCreationAnalysis mirrors Statement::post_creation_analysis.
// Statement.cpp:844–900 — combine branches / makeup; effect; assign/return facts;
// func_1 uncertain-call revalidate; set in/out/visited.
// opts is the session Options (CGOptions); no soft invent Defaults().
func PostCreationAnalysis(st *Stmt, preFacts []*FactPointTo, preEffect Effect, cg *CGContext, opts Options) {
	if st == nil || cg == nil || cg.FM == nil {
		return
	}
	fm := cg.FM
	// incomplete pre-facts: fail closed empty env (no invent cleaned post-creation)
	// No sticky ERROR — incomplete pre can appear after fail-closed subpaths; wiping
	// GlobalFacts signals incomplete without aborting whole generation.
	if !FactsComplete(preFacts) {
		fm.GlobalFacts = nil
		return
	}
	// incomplete GlobalFacts: makeup/branch combine must not invent past holes
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = nil
		return
	}
	// Statement::stm_id always live; StmID 0 fails closed (no invent post_creation
	// success without map_facts_in/out / map_visited)
	if st.StmID <= 0 {
		fm.GlobalFacts = nil
		return
	}
	if st.Kind == StmtIfElse {
		CombineBranchFacts(st, preFacts, fm)
	} else {
		// MakeupNewVarFacts fails closed (nils preFacts) on holes; pre already complete
		if !MakeupNewVarFacts(&preFacts, fm.GlobalFacts) {
			fm.GlobalFacts = nil
			return
		}
	}
	// simple statements: save effect_stm
	if !IsCompound(st.Kind) {
		fm.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	specialHandled := false
	// Statement.cpp:864–878 — func_1 outside loop + uncertain call → full validate
	if cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop() {
		if HasUncertainCallRecursiveStmt(st) {
			// preFacts complete above; still re-check after makeup
			if !FactsComplete(preFacts) {
				fm.GlobalFacts = nil
				return
			}
			outputs := CloneFactSlice(preFacts)
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preEffect.Clone()
			}
			// Statement.cpp:868–871 — assert(validate); no soft invent skip special path
			if !ValidateAndUpdateFacts(st, &outputs, cg, opts, cg.CurrentBlock()) {
				SetError(ErrGeneric)
				return
			}
			fm.GlobalFacts = outputs
			specialHandled = true
		}
	}
	if !specialHandled {
		switch st.Kind {
		case StmtAssign:
			// abstract assign into global_facts (RHS calls already handled at gen)
			// StatementAssign always has live Lhs; incomplete IR — no invent skip update
			lhs := st.LhsVar
			indir := 0
			if st.Lhs != nil {
				lhs = st.Lhs.Var
				// incomplete Lhs type IR must not invent indir 0 assign facts
				n, iok := st.Lhs.IndirectLevelComplete()
				if !iok {
					SetError(ErrGeneric)
					return
				}
				indir = n
			}
			if lhs == nil {
				SetError(ErrGeneric)
				return
			}
			// FactMgr.cpp:397–399 — update_fact_for_assign(sa) uses get_rhs()
			// incomplete assign fails closed — no invent mark visited with wiped facts
			_ = fm.UpdateFactForAssign(lhs, indir, st.GetAssignRhs())
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = nil
				return
			}
		case StmtReturn:
			// curr_func + rv + return expr always live for return after make
			// incomplete → ERROR (no invent garbage transfer from nil Expression*)
			if cg.CurrentFunc == nil || cg.CurrentFunc.RV == nil || st.Expr == nil {
				SetError(ErrGeneric)
				return
			}
			_ = fm.UpdateFactForReturnStmt(st, cg.CurrentFunc.RV, st.Expr)
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = nil
				return
			}
		}
	}
	fm.RemoveRVFacts(&fm.GlobalFacts)
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = nil
		return
	}
	fm.SetMapFactsIn(st.StmID, preFacts)
	fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, cg.CurrentBlock())
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[st.StmID] = cg.AccumEffect()
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[st.StmID] = true
}

// FindFixedPointBlock mirrors Block::find_fixed_point.
// Block.cpp:513–568 — merge back edges, shortcut, locals, analyze stmts, loop.
// failIndex is the statement index that failed analyze_with_edges_in, or -1.
func FindFixedPointBlock(b *Block, inputs []*FactPointTo, cg *CGContext, opts Options, visitOnce bool) (facts []*FactPointTo, failIndex int, ok bool) {
	// Block.cpp:513+ — always live this + cg_context; no soft invent success on nil
	if b == nil || cg == nil {
		return inputs, -1, false
	}
	// incomplete input env fails closed (no invent cleaned fixed-point from holes)
	if !FactsComplete(inputs) {
		return inputs, -1, false
	}
	fm := cg.FM
	currentInputs := CloneFactSlice(inputs)
	// push block
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.Stack = append(cg.CurrentFunc.Stack, b)
		defer func() {
			if f := cg.CurrentFunc; f != nil && len(f.Stack) > 0 {
				f.Stack = f.Stack[:len(f.Stack)-1]
			}
		}()
	}
	cnt := 0
	for {
		// Block.cpp:526–536 — when already visited, merge back-edge outs into inputs
		if fm != nil && b.StmID > 0 && fm.MapVisited != nil && fm.MapVisited[b.StmID] {
			if cnt++; cnt > 7 {
				// Block.cpp:526–530 — assert(0) when too many iterations; sticky error
				// no soft invent success / silent false without ERROR for callers
				SetError(ErrGeneric)
				return currentInputs, -1, false
			}
			back := fm.FindEdgesIn(b.StmID, false, true)
			// nil = incomplete CFG; no invent skip holes as absent back-edges
			if back == nil {
				SetError(ErrGeneric)
				return currentInputs, -1, false
			}
			for _, e := range back {
				// Block.cpp:535 — merge_facts(current_inputs, map_facts_out[src])
				// C++ map[] always; missing → empty merge; incomplete fails closed
				out := fm.MapFactsOut[e.SrcID]
				if !FactsComplete(currentInputs) || !FactsComplete(out) {
					SetError(ErrGeneric)
					return currentInputs, -1, false
				}
				// MergeFacts clears on mid-join failure — fail closed fixed-point
				_ = MergeFacts(&currentInputs, out)
				if !FactsComplete(currentInputs) {
					SetError(ErrGeneric)
					return currentInputs, -1, false
				}
			}
			toBlk := fm.FindEdgesInToBlock(b, false, true)
			if toBlk == nil {
				SetError(ErrGeneric)
				return currentInputs, -1, false
			}
			for _, e := range toBlk {
				// same map_facts_out[src] always-merge for edges into block
				out := fm.MapFactsOut[e.SrcID]
				if !FactsComplete(currentInputs) || !FactsComplete(out) {
					SetError(ErrGeneric)
					return currentInputs, -1, false
				}
				_ = MergeFacts(&currentInputs, out)
				if !FactsComplete(currentInputs) {
					SetError(ErrGeneric)
					return currentInputs, -1, false
				}
			}
		}
		// Block.cpp:537–541 — shortcut when inputs match previous
		if !visitOnce && fm != nil {
			// currentInputs kept complete; incomplete after merge fails closed above
			if !FactsComplete(currentInputs) {
				SetError(ErrGeneric)
				return currentInputs, -1, false
			}
			work := CloneFactSlice(currentInputs)
			sc := ShortcutAnalysisBlock(b, &work, cg)
			switch sc {
			case ShortcutOK:
				if fm != nil {
					fm.GlobalFacts = work
				}
				return work, -1, true
			case ShortcutConflict:
				return currentInputs, 0, false
			}
		}
		if !FactsComplete(currentInputs) {
			SetError(ErrGeneric)
			return currentInputs, -1, false
		}
		outputs := CloneFactSlice(currentInputs)
		// Block.cpp:546–549 — facts for locals
		// Variable* always live on LocalVars; nil hole fails closed (no invent skip)
		for _, v := range b.LocalVars {
			if v == nil {
				SetError(ErrGeneric)
				return nil, -1, false
			}
			AddNewVarFactTo(v, &outputs)
			// AddNewVarFactInto may clear on field/abstract holes
			if !FactsComplete(outputs) {
				SetError(ErrGeneric)
				return nil, -1, false
			}
		}
		// incomplete after local makeup fails closed
		if !FactsComplete(outputs) {
			SetError(ErrGeneric)
			return outputs, -1, false
		}
		// Block.cpp:552–557 — analyze each statement
		for i := range b.Stmts {
			if !AnalyzeWithEdgesIn(&b.Stmts[i], &outputs, cg, opts, b) {
				return outputs, i, false
			}
		}
		if fm == nil {
			// no DFA maps — single pass
			return outputs, -1, true
		}
		// Block::stm_id always live when FM bound; StmID 0 fails closed
		// (no invent soft single-pass success without map_facts_in/out)
		if b.StmID <= 0 {
			SetError(ErrGeneric)
			return outputs, -1, false
		}
		fm.SetMapFactsIn(b.StmID, currentInputs)
		// OOS locals for fact_out (Block.cpp:560–561)
		// incomplete outputs after analyze fail closed (no invent cleaned out)
		if !FactsComplete(outputs) {
			SetError(ErrGeneric)
			return outputs, -1, false
		}
		outCopy := CloneFactSlice(outputs)
		if len(b.LocalVars) > 0 {
			tmp := outCopy
			// UpdateFactsForOOSVars mutates GlobalFacts; apply via temp
			saved := fm.GlobalFacts
			fm.GlobalFacts = tmp
			fm.UpdateFactsForOOSVars(b.LocalVars)
			outCopy = fm.GlobalFacts
			fm.GlobalFacts = saved
			// OOS may nil on incomplete; fail closed
			if !FactsComplete(outCopy) {
				SetError(ErrGeneric)
				return outCopy, -1, false
			}
		}
		fm.SetMapFactsOut(b.StmID, outCopy)
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[b.StmID] = true
		b.SetAccumulatedEffect(fm)
		fm.GlobalFacts = outCopy
		facts = outCopy
		visitOnce = false
		// next loop: merge edges + shortcut when inputs stable
	}
}
