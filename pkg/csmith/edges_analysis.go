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

// mergeJumpUnionFacts is the eUnionWrite half of FactMgr::merge_jump_facts.
// FactMgr.cpp:569–588 — for each non-rv fact, join related jump fact; missing → BOTTOM.
func mergeJumpUnionFacts(facts *[]*FactUnion, jumpFacts []*FactUnion) bool {
	if facts == nil {
		SetError(ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*facts) || !UnionFactsComplete(jumpFacts) {
		*facts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return false
	}
	subjects := append([]*FactUnion(nil), *facts...)
	for _, f := range subjects {
		if f == nil || f.Var == nil {
			*facts = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			return false
		}
		if isReturnVar(f.Var) {
			continue
		}
		jumpF := FindRelatedUnion(jumpFacts, f.Var)
		if HasError() {
			*facts = IncompleteUnionFactSlice()
			return false
		}
		if jumpF == nil {
			// FactMgr.cpp:580–582 — jump over init → BOTTOM for eUnionWrite
			jumpF = MakeFactUnion(f.Var, FactUnionBottom)
			if jumpF == nil || HasError() {
				*facts = IncompleteUnionFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
		merged := MergeUnionFact(*facts, jumpF)
		if !UnionFactsComplete(merged) {
			*facts = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			return false
		}
		*facts = merged
	}
	return true
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
	if StmIDUnset(destStmID) {
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
		// CreateCFGEdge stores DestStmID=dest.StmID (block is Statement).
		// Match DestBlock; id may be 0 for the first block (fair sid).
		if e.DestBlock == dest && e.PostDest == postDest && e.BackLink == backLink {
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
		if StmIDUnset(st.StmID) {
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
				// FactMgr.cpp:569–588 — full FactVec (ePointTo + eUnionWrite)
				out := fm.GetMapFactsOut(e.SrcID)
				if _, ok := tryMergeJumpFacts(facts, out); !ok {
					// tryMergeJumpFacts already SetError sticky
					return false
				}
				outU := fm.GetMapUnionFactsOut(e.SrcID)
				if !mergeJumpUnionFacts(&fm.UnionFacts, outU) {
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
			// Statement.cpp:830–831 — always merge_jump_facts / add_effect (full FactVec)
			out := fm.GetMapFactsOut(e.SrcID)
			if _, ok := tryMergeJumpFacts(facts, out); !ok {
				return false
			}
			outU := fm.GetMapUnionFactsOut(e.SrcID)
			if !mergeJumpUnionFacts(&fm.UnionFacts, outU) {
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
	if cg.FM == nil || StmIDUnset(st.StmID) {
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
// preUnion is the eUnionWrite partition of pre_facts FactVec (Statement.cpp:260).
// opts is the session Options (CGOptions); no soft invent Defaults().
// Statement + CGContext always live; sticky (no invent soft-skip post-creation past hole).
// Nil FM is non-sticky soft re-pick (sticky poisons soft factories without FM).
func PostCreationAnalysis(st *Stmt, preFacts []*FactPointTo, preUnion []*FactUnion, preEffect Effect, cg *CGContext, opts Options) {
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
	if !FactsComplete(preFacts) || !UnionFactsComplete(preUnion) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return
	}
	// incomplete GlobalFacts: makeup/branch combine must not invent past holes
	if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Statement::stm_id always live; StmID 0 fails closed sticky (no invent post_creation
	// success without map_facts_in/out / map_visited)
	if StmIDUnset(st.StmID) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Statement.cpp:847–852 — makeup / combine on full FactVec pre_facts.
	// preUnion is the eUnionWrite partition; must track makeup for set_fact_in.
	workPreUnion := append([]*FactUnion(nil), preUnion...)
	if workPreUnion == nil {
		workPreUnion = []*FactUnion{}
	}
	if st.Kind == StmtIfElse {
		// StatementIf.cpp:208–231 — combine_branch_facts mutates pre_facts + sets global_facts
		CombineBranchFacts(st, &preFacts, &workPreUnion, fm)
		if HasError() {
			return
		}
	} else {
		// MakeupNewVarFacts fails closed sticky (nils preFacts) on holes; pre already complete
		if !MakeupNewVarFacts(&preFacts, fm.GlobalFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return
		}
		if !makeupNewUnionFacts(&workPreUnion, fm.UnionFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
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
			// Statement.cpp:1006–1012 — FactVec outputs = pre_facts (after makeup);
			// reset_effect_accum(pre_effect); validate_and_update_facts.
			// Soft invent used post-gen GlobalFacts as the only revalidate base so
			// visit ran under a richer lattice and failed on seed-561037, leaving
			// gen-time map_stm_effect (first-build AddVisibleEffect effect_accum IV
			// order: g_39 after g_35 vs UP after g_2402 once visit rewrites via
			// feffect). Fair path: validate from pre_facts like C++.
			//
			// C++ FactVec is full (ePointTo + eUnionWrite). Soft invent restored
			// only pre point-to while FM.UnionFacts stayed post-gen: nested
			// func_30 last-write of g_582.f4 made g_582.f0 IsNonreadableField
			// during special revalidate arg visit (seed-10054), aborting visit
			// and leaving gen-time map_stm_effect IV extras (g_39.f4, g_356).
			// Install workPreUnion (makeup pre eUnionWrite) before validate;
			// deep-clone so visit Join/SetBottom does not mutate workPreUnion
			// used later for set_fact_in.
			//
			// When visit fails (NDEBUG continues), C++ installs pre_facts. Go's
			// visit still fails on some seeds where C++ succeeds (seed-363), and
			// pre_facts lacks renew-replaced may-null Fact* that live only on the
			// post-gen lattice. On visit failure keep post-gen GlobalFacts +
			// post-gen UnionFacts; map_stm_effect stays gen-time (visit did not
			// rewrite) matching both. On visit success leave visit outputs +
			// rewritten map + visit-mutated UnionFacts. Never reinstall gen-time
			// map after a successful visit.
			if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				SetError(ErrGeneric)
				return
			}
			if !UnionFactsComplete(workPreUnion) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				SetError(ErrGeneric)
				return
			}
			// Shallow Fact* copies (C++ FactVec assignment) for point-to;
			// deep clone for union work lattice (visit may mutate in place).
			postGenFacts := append([]*FactPointTo(nil), fm.GlobalFacts...)
			postGenUnion := append([]*FactUnion(nil), fm.UnionFacts...)
			outputs := append([]*FactPointTo(nil), preFacts...)
			preUnionWork := CloneUnionFactSliceDeep(workPreUnion)
			if HasError() || !UnionFactsComplete(preUnionWork) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				return
			}
			fm.UnionFacts = preUnionWork
			// Statement.cpp:1007 — reset_effect_accum(pre_effect)
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preEffect.Clone()
				if HasError() {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					return
				}
			}
			// Statement.cpp:1008–1010 — assert(0) if !validate; NDEBUG continues.
			okV := ValidateAndUpdateFacts(st, &outputs, cg, opts, cg.CurrentBlock())
			ClearError()
			if okV {
				if !FactsComplete(outputs) || !UnionFactsComplete(fm.UnionFacts) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					SetError(ErrGeneric)
					return
				}
				fm.SetGlobalFacts(outputs, "auto_edges_analysis_408")
				// map_stm_effect / effect_accum / UnionFacts: post-validate
				// (visit rewrite). Do not reinstall gen-time map or unions.
			} else {
				// Visit failed under NDEBUG: C++ installs pre_facts. Go visit still
				// fails on seeds where C++ succeeds; pre_facts then lacks
				// renew-replaced may-null Fact* that only live on post-gen
				// GlobalFacts (seed-363). Keep post-gen PT + eUnionWrite lattice;
				// map_stm_effect remains gen-time (visit did not rewrite);
				// effect_accum stays at reset pre_effect (C++ on visit fail).
				if !FactsComplete(postGenFacts) || !UnionFactsComplete(postGenUnion) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					SetError(ErrGeneric)
					return
				}
				fm.SetGlobalFacts(postGenFacts, "auto_edges_analysis_408")
				fm.UnionFacts = postGenUnion
			}
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
			// FactUnion.cpp:133 / FactMgr.cpp:370 — abstract uses Lhs::get_type()
			// (desired type after deref), not Variable::type. Soft invent was
			// UpdateFactForAssign(var,…) only: post_creation of (*union*) and
			// visit path diverged (visit already used UpdateFactForAssignWant).
			var lhsWant *Type
			if st.Lhs != nil {
				lhs = st.Lhs.Var
				// incomplete Lhs type IR must not invent indir 0 assign facts
				n, iok := st.Lhs.IndirectLevelComplete()
				if !iok {
					SetError(ErrGeneric)
					return
				}
				indir = n
				lhsWant = st.Lhs.GetType()
				if HasError() {
					return
				}
			}
			if lhs == nil {
				SetError(ErrGeneric)
				return
			}
			// FactMgr.cpp:397–399 — update_fact_for_assign(sa) uses get_rhs() + lhs
			// incomplete assign fails closed sticky — no invent mark visited with wiped facts
			_ = fm.UpdateFactForAssignWant(lhs, indir, lhsWant, st.GetAssignRhs())
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
	// incomplete union after combine/assign must not invent set_fact_in success
	if !UnionFactsComplete(fm.UnionFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	// FactMgr.cpp set_fact_in — full FactVec (point-to pre + eUnionWrite pre, after makeup)
	fm.SetMapFactsInPair(st.StmID, preFacts, workPreUnion)
	fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, cg.CurrentBlock())
	// Incomplete accum fails closed sticky (no invent MapAccumEffect incomplete as recorded success)
	acc := cg.AccumEffect()
	if !EffectComplete(acc) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	fm.SetMapAccumEffect(st.StmID, acc)
	// residual ERROR sticky — no invent post_creation success past SetMapAccum residual
	if HasError() {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[st.StmID] = true
}

// FindFixedPointBlock mirrors Block::find_fixed_point.
// Block.cpp:513–568 — merge back edges, shortcut, locals, analyze stmts, loop.
// failIndex is the statement index that failed analyze_with_edges_in, or -1.
//
// On success, facts/unions are C++ post_facts after the call (full FactVec split):
//   - After at least one full sequential visit: pre-OOS outputs (Block.cpp:558).
//   - Pure shortcut (no full visit this call): nil,nil — C++ leaves the caller's
//     post_facts unchanged (Block.cpp:538–541 return true without assignment).
// map_facts_out is always post-OOS (Block.cpp:560–561).
// Soft invent refreshed postUnion from live after ShortcutAnalysis installed
// map_union_out (post-OOS) → body-local eUnionWrite dropped before append_return
// (seed-49 l_593 pool n=17 vs UP n=18).
func FindFixedPointBlock(b *Block, inputs []*FactPointTo, cg *CGContext, opts Options, visitOnce bool) (facts []*FactPointTo, unions []*FactUnion, failIndex int, ok bool) {
	// Block.cpp:513+ — always live this + cg_context; sticky no soft invent success on nil
	if b == nil || cg == nil {
		SetError(ErrGeneric)
		return inputs, nil, -1, false
	}
	// incomplete input env fails closed sticky (no invent cleaned fixed-point from holes)
	if !FactsComplete(inputs) {
		SetError(ErrGeneric)
		return inputs, nil, -1, false
	}
	fm := cg.FM
	currentInputs := CloneFactSlice(inputs)
	// currentUnions is the eUnionWrite half of C++ current_inputs (FactVec).
	// Only back-edge merge_facts grow it — not sequential stmt analyze (Block.cpp:520–536).
	// Soft invent reused post-analyze live UnionFacts as entry → same_facts / map_in skew.
	var currentUnions []*FactUnion
	if fm != nil {
		if !UnionFactsComplete(fm.UnionFacts) {
			SetError(ErrGeneric)
			return inputs, nil, -1, false
		}
		currentUnions = CloneUnionFactSliceDeep(fm.UnionFacts)
		if !UnionFactsComplete(currentUnions) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return inputs, nil, -1, false
		}
	}
	// last pre-OOS sequential outputs (C++ post_facts assignment at Block.cpp:558)
	var lastPreOOS []*FactPointTo
	var lastPreOOSUnions []*FactUnion
	// Generation-time stack: make_random already has `b` on func.Stack during
	// post_creation. Always-push here double-entered `b` (seed-2 e13830).
	// VisitFactsBlock (off stack) still needs CurrentBlock() for some visit paths.
	// C++ find_fixed_point does not push (Block.cpp:513–568); push only when not top.
	pushed := false
	if cg.CurrentFunc != nil {
		n := len(cg.CurrentFunc.Stack)
		if n == 0 || cg.CurrentFunc.Stack[n-1] != b {
			cg.CurrentFunc.Stack = append(cg.CurrentFunc.Stack, b)
			pushed = true
		}
		if pushed {
			defer func() {
				if f := cg.CurrentFunc; f != nil {
					if n := len(f.Stack); n > 0 && f.Stack[n-1] == b {
						f.Stack = f.Stack[:n-1]
					}
				}
			}()
		}
	}
	cnt := 0
	for {
		// Block.cpp:526–536 — when already visited, merge back-edge outs into inputs
		if fm != nil && !StmIDUnset(b.StmID) && fm.MapVisited != nil && fm.MapVisited[b.StmID] {
			if cnt++; cnt > 7 {
				// Block.cpp:526–530 — assert(0); NDEBUG continues do-while.
				// Returning false+strip invents emptying the body (seed-2 func_54).
				// Fair: keep iterating until shortcut converges (Release UP path).
				// Cap hard to avoid hang: accept current outputs as fixed-point.
				if cnt > 50 {
					// last-resort accept (not in C++; safety only)
					return currentInputs, currentUnions, -1, true
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
				return currentInputs, nil, -1, false
			}
			for _, e := range back {
				// Block.cpp:535 — merge_facts(current_inputs, map_facts_out[src])
				// Full FactVec: ePointTo + eUnionWrite (soft invent was PT-only).
				// C++ map[] always; missing → empty merge; incomplete fails closed
				out := fm.GetMapFactsOut(e.SrcID)
				if !FactsComplete(currentInputs) || !FactsComplete(out) {
					SetError(ErrGeneric)
					return currentInputs, nil, -1, false
				}
				// MergeFacts clears on mid-join failure — fail closed fixed-point
				_ = MergeFacts(&currentInputs, out)
				// residual ERROR sticky — no invent soft-fixed-point past MergeFacts residual
				if HasError() {
					return currentInputs, nil, -1, false
				}
				if !FactsComplete(currentInputs) {
					SetError(ErrGeneric)
					return currentInputs, nil, -1, false
				}
				outU := fm.GetMapUnionFactsOut(e.SrcID)
				if !UnionFactsComplete(currentUnions) || !UnionFactsComplete(outU) {
					SetError(ErrGeneric)
					return currentInputs, nil, -1, false
				}
				for _, uf := range outU {
					if uf == nil {
						SetError(ErrGeneric)
						return currentInputs, nil, -1, false
					}
					// MergeUnionFact appends without clone on new subject; deep-clone
					// so map_facts_out lattice is not aliased into current_inputs.
					cp := uf.Clone()
					if cp == nil || HasError() {
						if !HasError() {
							SetError(ErrGeneric)
						}
						return currentInputs, nil, -1, false
					}
					currentUnions = MergeUnionFact(currentUnions, cp)
					if !UnionFactsComplete(currentUnions) {
						if !HasError() {
							SetError(ErrGeneric)
						}
						return currentInputs, nil, -1, false
					}
				}
			}
		}
		// Install entry eUnionWrite as live before shortcut / sequential (C++ one FactVec).
		if fm != nil {
			if !UnionFactsComplete(currentUnions) {
				SetError(ErrGeneric)
				return currentInputs, nil, -1, false
			}
			liveU := CloneUnionFactSliceDeep(currentUnions)
			if !UnionFactsComplete(liveU) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return currentInputs, nil, -1, false
			}
			fm.UnionFacts = liveU
		}
		// Block.cpp:537–541 — shortcut when inputs match previous
		if !visitOnce && fm != nil {
			// currentInputs kept complete; incomplete after merge fails closed above
			if !FactsComplete(currentInputs) {
				SetError(ErrGeneric)
				return currentInputs, nil, -1, false
			}
			work := CloneFactSlice(currentInputs)
			sc := ShortcutAnalysisBlock(b, &work, cg)
			switch sc {
			case ShortcutOK:
				// Block.cpp:538–541 — shortcut returns true without assigning post_facts.
				// If a full visit already set lastPreOOS, that is the C++ post_facts value.
				// Pure shortcut (no full visit): nil so caller keeps its pre-call post_facts.
				if lastPreOOS != nil {
					// Pair pre-OOS eUnionWrite with lastPreOOS (C++ one FactVec).
					return lastPreOOS, lastPreOOSUnions, -1, true
				}
				return nil, nil, -1, true
			case ShortcutConflict:
				// Block.cpp:541 — `// if (shortcut == 1) return false;` is commented out.
				// Effect conflict at block level falls through to full statement re-analysis
				// (unlike Statement::validate_and_update_facts which fails on shortcut==1).
				// Inventing fail-closed here stripped nested fors during outer FP
				// (seed-2 e10107: body 90 ShortcutConflict → for 124 fail → strip).
			}
		}
		if !FactsComplete(currentInputs) {
			SetError(ErrGeneric)
			return currentInputs, nil, -1, false
		}
		// Entry eUnionWrite for set_fact_in (C++ current_inputs unchanged by analyze).
		// Snapshot from currentUnions — not post-analyze live (which includes locals).
		var entryUnions []*FactUnion
		if fm != nil {
			if !UnionFactsComplete(currentUnions) {
				SetError(ErrGeneric)
				return currentInputs, nil, -1, false
			}
			entryUnions = CloneUnionFactSliceDeep(currentUnions)
			if !UnionFactsComplete(entryUnions) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return currentInputs, nil, -1, false
			}
		}
		outputs := CloneFactSlice(currentInputs)
		// Block.cpp:546–549 — facts for locals (full FactVec: ePointTo + eUnionWrite)
		// Variable* always live on LocalVars; nil hole fails closed (no invent skip)
		workUnions := entryUnions
		if fm != nil {
			workUnions = CloneUnionFactSliceDeep(currentUnions)
			if !UnionFactsComplete(workUnions) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return currentInputs, nil, -1, false
			}
		}
		for _, v := range b.LocalVars {
			if v == nil {
				SetError(ErrGeneric)
				return nil, nil, -1, false
			}
			AddNewVarFactTo(v, &outputs)
			// AddNewVarFactInto may clear on field/abstract holes
			if !FactsComplete(outputs) {
				SetError(ErrGeneric)
				return nil, nil, -1, false
			}
			// FactMgr.cpp:118–131 add_new_var_fact — eUnionWrite half into outputs FactVec
			if fm != nil && MetaFactUnionEnabled() {
				_, unInit := AbstractFactForVarInit(v)
				if HasError() || !UnionFactsComplete(unInit) {
					if !HasError() {
						SetError(ErrGeneric)
					}
					return nil, nil, -1, false
				}
				for _, uf := range unInit {
					if uf == nil || uf.Var == nil {
						SetError(ErrGeneric)
						return nil, nil, -1, false
					}
					if FindRelatedUnion(workUnions, uf.Var) != nil {
						if HasError() {
							return nil, nil, -1, false
						}
						continue
					}
					if HasError() {
						return nil, nil, -1, false
					}
					cp := uf.Clone()
					if cp == nil || HasError() {
						if !HasError() {
							SetError(ErrGeneric)
						}
						return nil, nil, -1, false
					}
					workUnions = append(workUnions, cp)
				}
			}
		}
		// incomplete after local makeup fails closed
		if !FactsComplete(outputs) {
			SetError(ErrGeneric)
			return outputs, nil, -1, false
		}
		// Sequential analyze uses outputs FactVec = PT + live unions (with locals).
		if fm != nil {
			if !UnionFactsComplete(workUnions) {
				SetError(ErrGeneric)
				return outputs, nil, -1, false
			}
			fm.UnionFacts = workUnions
		}
		// Block.cpp:552–557 — analyze each statement
		for i := range b.Stmts {
			if !AnalyzeWithEdgesIn(&b.Stmts[i], &outputs, cg, opts, b) {
				return outputs, nil, i, false
			}
		}
		if fm == nil {
			// no DFA maps — single pass
			return outputs, workUnions, -1, true
		}
		// Block::stm_id always live when FM bound; StmID 0 fails closed
		// (no invent soft single-pass success without map_facts_in/out)
		if StmIDUnset(b.StmID) {
			SetError(ErrGeneric)
			return outputs, nil, -1, false
		}
		// Block.cpp:557 — set_fact_in(this, current_inputs) full FactVec entry env.
		// Strip this block's LocalVars subjects if back-edge merge reintroduced them
		// (see DropFactSubjectsByVars). Entry must not list body locals.
		if len(b.LocalVars) > 0 {
			currentInputs = DropFactSubjectsByVars(currentInputs, b.LocalVars)
			if !FactsComplete(currentInputs) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return currentInputs, nil, -1, false
			}
			entryUnions = DropUnionSubjectsByVars(entryUnions, b.LocalVars)
			if !UnionFactsComplete(entryUnions) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return currentInputs, nil, -1, false
			}
			// Keep currentUnions in sync with map_facts_in eUnionWrite half.
			// Drop mutates entryUnions for set_fact_in but left currentUnions holding
			// body locals reintroduced by back-edge merge (seed-189 blk 575: l_1333
			// from goto src map_out). same_facts then forever saw nCurU=mapInU+1 and
			// never shortcut — multi-pass rewrote map_accum_effect and polluted
			// StatementGoto.cpp:125–128 forward choose_visible_read_var pools
			// (if (g_6) vs UP if (g_1192)). C++ current_inputs is the same FactVec
			// stored via set_fact_in (Block.cpp:557–599).
			currentUnions = entryUnions
		}
		fm.SetMapFactsInPair(b.StmID, currentInputs, entryUnions)
		// Block.cpp:558 — post_facts = outputs (pre-OOS)
		// incomplete outputs after analyze fail closed (no invent cleaned out)
		if !FactsComplete(outputs) {
			SetError(ErrGeneric)
			return outputs, nil, -1, false
		}
		lastPreOOS = CloneFactSlice(outputs)
		if HasError() || !FactsComplete(lastPreOOS) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return outputs, nil, -1, false
		}
		// C++ post_facts = full FactVec pre-OOS (ePointTo + eUnionWrite).
		// Capture eUnionWrite now — live still pre-OOS; later ShortcutAnalysis
		// installs map_union_out (post-OOS) into live before return.
		if !UnionFactsComplete(fm.UnionFacts) {
			SetError(ErrGeneric)
			return outputs, nil, -1, false
		}
		lastPreOOSUnions = CloneUnionFactSliceDeep(fm.UnionFacts)
		if HasError() || !UnionFactsComplete(lastPreOOSUnions) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return outputs, nil, -1, false
		}
		if lastPreOOSUnions == nil {
			lastPreOOSUnions = []*FactUnion{}
		}
		// Block.cpp:560–561 — OOS locals then set_fact_out (post-OOS).
		// C++ mutates the local outputs FactVec only. Soft invent used
		// fm.UpdateFactsForOOSVars on a GlobalFacts temp swap, which also
		// OOS-stripped live UnionFacts permanently. PT-only OOS on outCopy;
		// SetMapFactsOutForBlock clones+OOS live unions for map_union_out.
		outCopy := CloneFactSlice(outputs)
		if len(b.LocalVars) > 0 {
			UpdateFactsForOOSVars(b.LocalVars, &outCopy)
			if !FactsComplete(outCopy) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return outCopy, nil, -1, false
			}
		}
		// Block.cpp:561 — set_fact_out(this, outputs) after OOS only.
		// FactMgr.cpp:268–270 — function body (parent==nullptr) remove_function_local_facts.
		// Do not mergeMayNullFromLive / SetGlobalFacts here (Block.cpp:513–568
		// never assigns global_facts; post_creation installs map_facts_out at 729).
		fm.SetMapFactsOutForBlock(b, outCopy)
		if HasError() {
			return outCopy, nil, -1, false
		}
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[b.StmID] = true
		b.SetAccumulatedEffect(fm)
		facts = lastPreOOS
		visitOnce = false
		// next loop: merge edges + shortcut when inputs stable
	}
}
