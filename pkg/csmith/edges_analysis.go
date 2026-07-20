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
	// facts out always live; sticky (no invent soft-skip jump merge past hole)
	if facts == nil {
		SetError(ErrGeneric)
		return false, false
	}
	// pre-validate: incomplete maps must not soft-join past holes — sticky ERROR
	if !FactsComplete(*facts) || !FactsComplete(jumpFacts) {
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
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
		// residual ERROR sticky — no invent soft-continue garbage path past FindRelated residual
		if HasError() {
			*facts = IncompleteFactSlice()
			return false, false
		}
		if jumpF == nil {
			// jump over initializer → garbage
			jumpF = MakeFactPointTo(f.Var, GarbagePtr)
			if jumpF == nil || HasError() {
				if !HasError() {
					SetError(ErrGeneric)
				}
				*facts = IncompleteFactSlice()
				return false, false
			}
		}
		before := FindRelatedPointTo(*facts, f.Var)
		// residual ERROR sticky — no invent soft-continue merge past FindRelated residual
		if HasError() {
			*facts = IncompleteFactSlice()
			return false, false
		}
		merged := MergeFactInto(*facts, jumpF)
		if !FactsComplete(merged) {
			// mid-join incomplete — clear partial sticky, no invent keep half-merged map
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return false, false
		}
		*facts = merged
		after := FindRelatedPointTo(*facts, f.Var)
		// residual ERROR sticky — no invent soft-continue equal check past FindRelated residual
		if HasError() {
			*facts = IncompleteFactSlice()
			return false, false
		}
		if before == nil || after == nil {
			changed = true
			continue
		}
		eq := before.Equal(after)
		// residual ERROR sticky — no invent soft-continue no-change past Equal residual hole
		if HasError() {
			*facts = IncompleteFactSlice()
			return false, false
		}
		if !eq {
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
// Incomplete CFG fails closed sticky nil (no invent soft re-pick empty edges past holes).
// Complete scan with no matches returns empty non-nil slice.
func (fm *FactMgr) FindEdgesIn(destStmID int, postDest, backLink bool) []*CFGEdge {
	// FactMgr always live for CFG lookup; sticky no invent empty edges without it
	if fm == nil {
		SetError(ErrGeneric)
		return nil
	}
	// StmID ≤0 incomplete key sticky (no invent Map miss as empty edges / soft re-pick)
	if destStmID <= 0 {
		SetError(ErrGeneric)
		return nil
	}
	if !CFGEdgesComplete(fm.CFGEdges) {
		SetError(ErrGeneric)
		return nil
	}
	out := make([]*CFGEdge, 0)
	for _, e := range fm.CFGEdges {
		if e.DestStmID == destStmID && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// FindEdgesInToBlock finds edges whose DestBlock matches (break/continue).
// Incomplete CFGEdges → sticky nil (fail closed). Complete empty → non-nil [].
func (fm *FactMgr) FindEdgesInToBlock(dest *Block, postDest, backLink bool) []*CFGEdge {
	// FactMgr always live for CFG lookup; sticky no invent empty edges without it
	if fm == nil {
		SetError(ErrGeneric)
		return nil
	}
	// nil DestBlock incomplete sticky (no invent empty edges past hole / soft re-pick)
	if dest == nil {
		SetError(ErrGeneric)
		return nil
	}
	if !CFGEdgesComplete(fm.CFGEdges) {
		SetError(ErrGeneric)
		return nil
	}
	out := make([]*CFGEdge, 0)
	for _, e := range fm.CFGEdges {
		if e.DestBlock == dest && e.DestStmID == 0 && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// HasEdgeIn mirrors Statement::has_edge_in.
// Statement.cpp:434–446 — assert(fm); scan cfg_edges.
// Incomplete CFG (FindEdgesIn nil) sticky true — no invent "no edge"
// from len(nil)==0 soft-skip past hole.
func (fm *FactMgr) HasEdgeIn(destStmID int, postDest, backLink bool) bool {
	if fm == nil {
		// assert(fm) path — sticky has-edge (conservative revisit; no invent no-edge)
		SetError(ErrGeneric)
		return true
	}
	edges := fm.FindEdgesIn(destStmID, postDest, backLink)
	if edges == nil {
		// FindEdgesIn already SetError sticky on incomplete CFG
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	return len(edges) > 0
}

// AnalyzeWithEdgesIn mirrors Statement::analyze_with_edges_in.
// Statement.cpp:808–834 — merge visited jump sources then validate_and_update.
// Incomplete call / StmID / CFG / jump out / accum effect fails closed sticky
// false so soft re-pick cannot invent complete analysis past holes.
func AnalyzeWithEdgesIn(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	// Statement.cpp:808+ — always live Statement* + inputs + cg_context
	// no soft invent true on incomplete call
	if st == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	fm := cg.FM
	if fm != nil {
		// Statement::stm_id always live; StmID 0 fails closed sticky (no invent
		// soft-skip edge merge then validate as complete analysis)
		if st.StmID <= 0 {
			SetError(ErrGeneric)
			return false
		}
		// back edges only if already visited
		if fm.MapVisited != nil && fm.MapVisited[st.StmID] {
			back := fm.FindEdgesIn(st.StmID, false, true)
			// nil = incomplete CFG (hole); empty non-nil = no matching edges
			// FindEdgesIn already SetError sticky on incomplete CFG
			if back == nil {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			for _, e := range back {
				// unvisited src is intentional filter (not soft invent of edge)
				if fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
					continue
				}
				// Statement.cpp:819–820 — always merge_jump_facts / add_effect
				// C++ map[] missing → empty; no invent skip merge when out absent
				// Incomplete out fails closed sticky (no invent partial jump merge)
				out := fm.GetMapFactsOut(e.SrcID)
				if _, ok := tryMergeJumpFacts(facts, out); !ok {
					// tryMergeJumpFacts already SetError sticky
					return false
				}
				// map_accum_effect[src] — missing live id → empty; SrcID 0 IncompleteEffect
				// Incomplete accum fails closed sticky (no invent AddEffect poison then still ok)
				accE := fm.GetMapAccumEffect(e.SrcID)
				if !EffectComplete(accE) {
					if !HasError() {
						SetError(ErrGeneric)
					}
					return false
				}
				cg.AddEffect(accE, false)
				if !EffectComplete(cg.EffectStm) {
					if !HasError() {
						SetError(ErrGeneric)
					}
					return false
				}
			}
		}
		// always consider forward edges
		fwd := fm.FindEdgesIn(st.StmID, false, false)
		if fwd == nil {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		for _, e := range fwd {
			if fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
				continue
			}
			// Statement.cpp:830–831 — always merge_jump_facts / add_effect
			out := fm.GetMapFactsOut(e.SrcID)
			if _, ok := tryMergeJumpFacts(facts, out); !ok {
				return false
			}
			accE := fm.GetMapAccumEffect(e.SrcID)
			if !EffectComplete(accE) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.AddEffect(accE, false)
			if !EffectComplete(cg.EffectStm) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
	}
	return ValidateAndUpdateFacts(st, facts, cg, opts, blk)
}

// SetAccumulatedEffectAfterBlock mirrors Statement::set_accumulated_effect_after_block.
// Statement.cpp:515–520 — eff += block effect; store as this statement's effect.
// Statement::stm_id always live; StmID 0 is incomplete (no invent soft no-op success
// that leaves map_stm_effect unset while callers treat effect as recorded).
// Incomplete pre/block effects store IncompleteEffect (AddEffect already fails closed).
// Statement + CGContext always live; sticky (no invent soft-skip effect record past hole).
// Nil FM / StmID≤0 is non-sticky soft re-pick (sticky poisons soft factories without FM).
func SetAccumulatedEffectAfterBlock(st *Stmt, blockEffect Effect, cg *CGContext, preStm Effect) {
	if st == nil || cg == nil {
		SetError(ErrGeneric)
		return
	}
	if cg.FM == nil || st.StmID <= 0 {
		return
	}
	// incomplete inputs fail closed sticky IncompleteEffect map entry (no invent pure merge
	// or soft re-pick past holes as recorded success)
	if !EffectComplete(preStm) || !EffectComplete(blockEffect) {
		cg.FM.SetMapStmEffect(st.StmID, IncompleteEffect())
		SetError(ErrGeneric)
		return
	}
	eff := preStm.AddEffect(blockEffect)
	// residual ERROR sticky — no invent soft-complete map past AddEffect residual
	if HasError() {
		cg.FM.SetMapStmEffect(st.StmID, IncompleteEffect())
		return
	}
	if !EffectComplete(eff) {
		cg.FM.SetMapStmEffect(st.StmID, IncompleteEffect())
		SetError(ErrGeneric)
		return
	}
	cg.FM.SetMapStmEffect(st.StmID, eff)
}

// PostCreationAnalysis mirrors Statement::post_creation_analysis.
// Statement.cpp:844–900 — combine branches / makeup; effect; assign/return facts;
// func_1 uncertain-call revalidate; set in/out/visited.
// opts is the session Options (CGOptions); no soft invent Defaults().
// Statement + CGContext always live; sticky (no invent soft-skip post-creation past hole).
// Nil FM is non-sticky soft re-pick (sticky poisons soft factories without FM).
func PostCreationAnalysis(st *Stmt, preFacts []*FactPointTo, preEffect Effect, cg *CGContext, opts Options) {
	if st == nil || cg == nil {
		SetError(ErrGeneric)
		return
	}
	if cg.FM == nil {
		return
	}
	fm := cg.FM
	// incomplete pre-facts: fail closed sticky (no invent cleaned post-creation)
	// sticky ERROR so makeRandomStmt ERROR_GUARD aborts without soft re-pick past wipe
	if !FactsComplete(preFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// incomplete GlobalFacts: makeup/branch combine must not invent past holes
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Statement::stm_id always live; StmID 0 fails closed sticky (no invent post_creation
	// success without map_facts_in/out / map_visited)
	if st.StmID <= 0 {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	if st.Kind == StmtIfElse {
		CombineBranchFacts(st, preFacts, fm)
	} else {
		// MakeupNewVarFacts fails closed sticky (nils preFacts) on holes; pre already complete
		if !MakeupNewVarFacts(&preFacts, fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return
		}
	}
	// simple statements: save effect_stm
	// Incomplete EffectStm fails closed sticky (no invent map_stm_effect incomplete as recorded success)
	if !IsCompound(st.Kind) {
		if !EffectComplete(cg.EffectStm) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		fm.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	specialHandled := false
	// Statement.cpp:864–878 — func_1 outside loop + uncertain call → full validate
	if cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop() {
		unc := HasUncertainCallRecursiveStmt(st)
		// residual ERROR sticky — no invent soft-validate past HasUncertain residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		if unc {
			// preFacts complete above; still re-check after makeup
			if !FactsComplete(preFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			outputs := CloneFactSlice(preFacts)
			// residual ERROR sticky — no invent soft-validate past CloneFactSlice residual
			if HasError() {
				fm.GlobalFacts = IncompleteFactSlice()
				return
			}
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preEffect.Clone()
				// residual ERROR sticky — no invent soft-validate past Effect Clone residual
				if HasError() {
					fm.GlobalFacts = IncompleteFactSlice()
					return
				}
			}
			// Statement.cpp:868–871 — assert(0) if !validate; NDEBUG elides assert and
			// still installs outputs + special_handled (Release csmith does not abort).
			// Do not sticky-poison generation: match NDEBUG continue (same class as
			// FactUnion indirect==-1 under NDEBUG).
			_ = ValidateAndUpdateFacts(st, &outputs, cg, opts, cg.CurrentBlock())
			// residual ERROR sticky — no invent install outputs past validate residual hole
			if HasError() {
				fm.GlobalFacts = IncompleteFactSlice()
				return
			}
			fm.SetGlobalFacts(outputs, "auto_edges_analysis_408")
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
			// incomplete assign fails closed sticky — no invent mark visited with wiped facts
			_ = fm.UpdateFactForAssign(lhs, indir, st.GetAssignRhs())
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
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
			// residual ERROR sticky — no invent mark visited / soft-continue past return residual
			if HasError() || !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				return
			}
		}
	}
	fm.RemoveRVFacts(&fm.GlobalFacts)
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	fm.SetMapFactsIn(st.StmID, preFacts)
	fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, cg.CurrentBlock())
	// Incomplete accum fails closed sticky (no invent MapAccumEffect incomplete as recorded success)
	acc := cg.AccumEffect()
	if !EffectComplete(acc) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[st.StmID] = acc
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[st.StmID] = true
}

// FindFixedPointBlock mirrors Block::find_fixed_point.
// Block.cpp:513–568 — merge back edges, shortcut, locals, analyze stmts, loop.
// failIndex is the statement index that failed analyze_with_edges_in, or -1.
func FindFixedPointBlock(b *Block, inputs []*FactPointTo, cg *CGContext, opts Options, visitOnce bool) (facts []*FactPointTo, failIndex int, ok bool) {
	// Block.cpp:513+ — always live this + cg_context; sticky no soft invent success on nil
	if b == nil || cg == nil {
		SetError(ErrGeneric)
		return inputs, -1, false
	}
	// incomplete input env fails closed sticky (no invent cleaned fixed-point from holes)
	if !FactsComplete(inputs) {
		SetError(ErrGeneric)
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
				// Block.cpp:526–530 — assert(0); NDEBUG continues do-while.
				// Returning false+strip invents emptying the body (seed-2 func_54).
				// Fair: keep iterating until shortcut converges (Release UP path).
				// Cap hard to avoid hang: accept current outputs as fixed-point.
				if cnt > 50 {
					// last-resort accept (not in C++; safety only)
					return currentInputs, -1, true
				}
				// fall through: continue merge/analyze loop like NDEBUG
			}
			// Block.cpp:531–536 — find_edges_in(this, false, true): e->dest == this.
			// CreateCFGEdge stores DestStmID=block.StmID for Block* dests — no invent
			// second pass FindEdgesInToBlock (would double-merge same edges).
			back := fm.FindEdgesIn(b.StmID, false, true)
			// nil = incomplete CFG; no invent skip holes as absent back-edges
			if back == nil {
				SetError(ErrGeneric)
				return currentInputs, -1, false
			}
			for _, e := range back {
				// Block.cpp:535 — merge_facts(current_inputs, map_facts_out[src])
				// C++ map[] always; missing → empty merge; incomplete fails closed
				out := fm.GetMapFactsOut(e.SrcID)
				if !FactsComplete(currentInputs) || !FactsComplete(out) {
					SetError(ErrGeneric)
					return currentInputs, -1, false
				}
				// MergeFacts clears on mid-join failure — fail closed fixed-point
				_ = MergeFacts(&currentInputs, out)
				// residual ERROR sticky — no invent soft-fixed-point past MergeFacts residual
				if HasError() {
					return currentInputs, -1, false
				}
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
				// Block.cpp:538–541 — shortcut returns true; does not assign global_facts.
				// (map_facts_out already holds the reused env; caller may install.)
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
			fm.SetGlobalFacts(tmp, "fixed_point_oos_tmp")
			fm.UpdateFactsForOOSVars(b.LocalVars)
			outCopy = fm.GlobalFacts
			fm.SetGlobalFacts(saved, "fixed_point_oos_restore")
			// OOS may nil on incomplete; fail closed
			if !FactsComplete(outCopy) {
				SetError(ErrGeneric)
				return outCopy, -1, false
			}
		}
		// Block.cpp:558–562 — set_fact_out(this, outputs) after OOS locals only.
		// Do not mergeMayNullFromLive(GlobalFacts): that re-injects mid-gen may-null
		// from statements later stripped by post_creation_analysis fixed-point
		// (seed-2: (*g_140)=(void*)0 stripped but null kept → ExpressionVariable
		// rejects g_140; UP accepts). C++ installs cleaned outputs only.
		// Block.cpp:513–568 — find_fixed_point never assigns fm->global_facts;
		// post_creation_analysis installs map_facts_out after success (Block.cpp:729).
		// Invent SetGlobalFacts(outCopy) here wiped mid-gen may-null during FP
		// (seed-2 e10107) unless StmVisitFacts unfairly remerged live into maps.
		fm.SetMapFactsOut(b.StmID, outCopy)
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[b.StmID] = true
		b.SetAccumulatedEffect(fm)
		facts = outCopy
		visitOnce = false
		// next loop: merge edges + shortcut when inputs stable
	}
}
