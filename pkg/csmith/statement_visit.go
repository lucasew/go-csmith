// Upstream: StatementIf/StatementFor/Block visit_facts analysis paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStmt dispatches Statement::visit_facts by kind.
// Statement subclasses — assign/if/for/block/return/jump/expr.
// Incomplete IR fails closed sticky (no soft invent true / soft re-pick past holes).
func VisitFactsStmt(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	switch st.Kind {
	case StmtAssign:
		return VisitFactsStatementAssign(st, cg, opts)
	case StmtIfElse:
		return VisitFactsStatementIf(st, cg, opts)
	case StmtFor:
		return VisitFactsStatementFor(st, cg, opts)
	case StmtArrayOp:
		return VisitFactsStatementArrayOp(st, cg, opts)
	case StmtReturn:
		return VisitFactsStatementReturn(st, cg, opts)
	case StmtBreak, StmtContinue:
		return VisitFactsStatementJump(st, cg, opts)
	case StmtGoto:
		return VisitFactsStatementGoto(st, cg, opts)
	case StmtLabel:
		// Statement::stm_id always live; StmID 0 + FM fails closed sticky
		// Incomplete EffectStm sticky (no invent visit true with incomplete map)
		if cg.FM != nil {
			if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
		return true
	case StmtInvoke:
		return VisitFactsStatementExpr(st, cg, opts)
	case StmtBlock:
		if st.Then == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		return VisitFactsBlock(st.Then, cg, opts)
	default:
		// unknown kind hard IR sticky
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
}

// VisitFactsStatementJump mirrors StatementBreak/Continue::visit_facts.
// StatementBreak.cpp:126–134 / StatementContinue.cpp:125–133 — test then effect_stm.
func VisitFactsStatementJump(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// C++ always has live test Expression*; nil Expr sticky hard IR
	if st.Expr == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM fails closed sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsStatementGoto mirrors StatementGoto::visit_facts.
// StatementGoto.cpp:364–402 — test; check_write skipped vars; subset re-analysis of dest.
func VisitFactsStatementGoto(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementGoto.cpp:366–368 — test.visit_facts always; nil Expr sticky
	if st.Expr == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// check write on skipped vars (re-init at dest)
	// StatementGoto.cpp — vars[i] always live; nil hole sticky via CheckWriteVar
	facts := cg.pointToFacts()
	// incomplete working facts sticky (no invent write-check past holes / soft re-pick)
	if !FactsComplete(facts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	for _, v := range st.InitSkippedVars {
		if v == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if !cg.CheckWriteVar(v, facts) {
			return false
		}
	}
	fm := cg.FM
	if fm != nil {
		// Statement::stm_id always live; StmID 0 sticky
		if StmIDUnset(st.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		// StatementGoto.cpp:390–398 — force dest re-analysis when current outs
		// are proper subset of previous outs and neither visited this pass.
		destID := st.GotoDestStmID
		// StatementGoto.cpp:390–398 — fair sid allows dest 0; clear full FactVec
		// (ePointTo + eUnionWrite). Soft invent was destID>0 + PT-only delete.
		if !StmIDUnset(destID) {
			visitedThis := fm.MapVisited != nil && fm.MapVisited[st.StmID]
			visitedDest := fm.MapVisited != nil && fm.MapVisited[destID]
			prevOut := fm.GetMapFactsOut(st.StmID)
			prevOutU := fm.GetMapUnionFactsOut(st.StmID)
			cur := facts
			if !FactsComplete(prevOut) || !UnionFactsComplete(prevOutU) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			if !UnionFactsComplete(fm.UnionFacts) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			curU := fm.UnionFacts
			if !visitedThis && !visitedDest &&
				!SameFactVecSess(cgSess(cg), cur, curU, prevOut, prevOutU) &&
				SubsetFactVecSess(cgSess(cg), cur, curU, prevOut, prevOutU) {
				if sessHasError(cgSess(cg)) {
					return false
				}
				delete(fm.MapFactsIn, destID)
				delete(fm.MapFactsOut, destID)
				delete(fm.MapUnionFactsIn, destID)
				delete(fm.MapUnionFactsOut, destID)
			}
			if sessHasError(cgSess(cg)) {
				return false
			}
		}
		// Incomplete EffectStm sticky (no invent visit true with incomplete map)
		if !EffectComplete(cg.EffectStm) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		fm.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsStatementExpr mirrors StatementExpr::visit_facts.
// StatementExpr.cpp:104–110 — expr.visit_facts; store effect_stm.
func VisitFactsStatementExpr(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementExpr.cpp:106 — expr always live; nil Expr sticky
	if st.Expr == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsBlock mirrors Block::visit_facts via find_fixed_point.
// Block.cpp:466–479 — pre_effect snapshot; find_fixed_point; on fail reset
// effect_accum; on success inputs=map_facts_out and map_visited[this]=true.
func VisitFactsBlock(b *Block, cg *CGContext, opts Options) bool {
	if b == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// Block.cpp:472 — Effect pre_effect = cg_context.get_accum_effect();
	var preEffect Effect
	havePre := false
	if cg.EffectAccum != nil {
		if !EffectComplete(*cg.EffectAccum) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		preEffect = cg.EffectAccum.Clone()
		if sessHasError(cgSess(cg)) {
			return false
		}
		havePre = true
	}
	var inputs []*FactPointTo
	if cg.FM != nil {
		// incomplete GlobalFacts sticky (no invent fixed-point / soft re-pick past holes)
		if !FactsComplete(cg.FM.GlobalFacts) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		inputs = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-fixed-point past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		// Block.cpp:471–480 — visit_facts does NOT clear map_visited[this] before
		// find_fixed_point. Preserving map_visited enables shortcut_analysis on
		// re-entry (same_facts vs map_facts_in) and fair back-edge merge when already
		// visited. Inventing delete(map_visited) forced full re-analysis of every
		// nested call on each outer FP pass (seed-2 func_49 visited 2..6 via func_33
		// then BUILD_REV fail at 7 / first_div e37241).
	}
	// Block.cpp:473–476 — find_fixed_point; fail → reset_effect_accum(pre_effect)
	out, _, _, ok := FindFixedPointBlock(b, inputs, cg, opts, false)
	if !ok {
		// Block.cpp:474–475 — cg_context.reset_effect_accum(pre_effect)
		if havePre && cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		return false
	}
	if cg.FM != nil && !StmIDUnset(b.StmID) {
		// Block.cpp:477 — inputs = fm->map_facts_out[this] (full FactVec).
		// Soft invent was SetGlobalFacts(PT-only): UnionFacts stayed at last-stmt
		// pre-OOS lattice → IsNonreadableField / same_facts skew on outer re-visit
		// (seed-7 nested loop FP over-strip of back-edge gotos).
		mout := cg.FM.GetMapFactsOut(b.StmID)
		if FactsComplete(mout) {
			cg.FM.AssignGlobalFactsFromMapOut(b.StmID)
			if sessHasError(cgSess(cg)) {
				return false
			}
		} else if FactsComplete(out) {
			cl := CloneFactSlice(out)
			if sessHasError(cgSess(cg)) || !FactsComplete(cl) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(cl, "auto_statement_visit_224")
		}
		// Block.cpp:478 — fm->map_visited[this] = true (always on success)
		if cg.FM.MapVisited == nil {
			cg.FM.MapVisited = make(map[int]bool)
		}
		cg.FM.MapVisited[b.StmID] = true
	}
	return true
}

// VisitFactsStatementIf mirrors StatementIf::visit_facts.
// StatementIf.cpp:162–198 — condition; both branches from post-cond env;
// set_accumulated_effect_after_block(cond_eff, each arm); merge by must_return.
// When both arms must_return, outputs restore pre-condition inputs (inputs_copy).
func VisitFactsStatementIf(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtIfElse {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementIf.cpp:164 — inputs_copy before condition (full FactVec)
	var inputsCopy []*FactPointTo
	var inputsCopyU []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		inputsCopy = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if visit past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		inputsCopyU = CloneUnionFactSliceDeepSess(cgSess(cg), cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(inputsCopyU) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
	// StatementIf.cpp:165–168 — evaluate condition first; nil test sticky
	if st.Expr == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// StatementIf.cpp:169 — effect_stm after condition
	condEff := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-if arms past EffectStm Clone residual
	if sessHasError(cgSess(cg)) {
		return false
	}
	// post-condition env shared as entry to both arms (full FactVec)
	var postCond []*FactPointTo
	var postCondU []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		postCond = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if arms past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		postCondU = CloneUnionFactSliceDeepSess(cgSess(cg), cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(postCondU) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
	// StatementIf.cpp:170–177 — both arms always live Blocks sticky
	if st.Then == nil || st.Else == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementIf.cpp:170–177 — both arms use the SAME cg_context (shared
	// effect_accum). Soft invent forked thenAccum/elseAccum so StmVisitFacts
	// rewrote map_accum_effect[nested] with arm-local history only (seed-42:
	// generation map_accum nRead=37 → post-visit overwrite nRead=7 →
	// choose_visible_read_var ok 1 vs UP 13).
	// StatementIf.cpp:170–173 — true branch from post-cond facts
	if !VisitFactsBlock(st.Then, cg, opts) {
		return false
	}
	var thenFacts []*FactPointTo
	var thenUnions []*FactUnion
	if cg.FM != nil {
		// incomplete then-arm facts sticky (no invent soft re-pick past hole as visit success)
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-then facts past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		thenUnions = CloneUnionFactSliceDeepSess(cgSess(cg), cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(thenUnions) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		// StatementIf.cpp:174 — false starts from same post-cond inputs (not after true)
		// Full FactVec: generation uses AssignGlobalFactsFromMapIn (PT+union).
		// Soft invent was SetGlobalFacts(PT-only) → else kept then-exit last-writes
		// (seed-7 nested loop FP over-strip of back-edge gotos).
		// effect_accum is NOT reset (C++ continues growing through false arm).
		cg.FM.SetGlobalFacts(CloneFactSlice(postCond), "auto_statement_visit_317")
		// residual ERROR sticky — no invent soft-else start past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		clU := CloneUnionFactSliceDeepSess(cgSess(cg), postCondU)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(clU) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		if clU == nil {
			cg.FM.UnionFacts = []*FactUnion{}
		} else {
			cg.FM.UnionFacts = clU
		}
	}

	// StatementIf.cpp:174–177 — false branch (same cg_context / effect_accum)
	if !VisitFactsBlock(st.Else, cg, opts) {
		return false
	}
	var elseFacts []*FactPointTo
	var elseUnions []*FactUnion
	if cg.FM != nil {
		// incomplete else-arm facts sticky (no invent soft re-pick past hole as visit success)
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		elseFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-else facts past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		elseUnions = CloneUnionFactSliceDeepSess(cgSess(cg), cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(elseUnions) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		// Arm map_facts_out is post-OOS of that arm's LocalVars only (Block.cpp:560–561).
		// Soft invent left foreign locals from the other arm in an arm's exit env when
		// entry was polluted (else-local l_2590 surviving on then after else OOS'd it).
		// C++ arm outs are map_facts_out[arm] post-OOS; other-arm locals are not in
		// scope on that path. Drop sibling-arm LocalVars from each arm exit before
		// must_return merge so if-output cannot re-inject out-of-scope eUnionWrite
		// subjects (seed-58: pure-shortcut same_facts size skew on for 1169 wiped
		// nested make_iteration IV reads from feffect).
		if len(st.Else.LocalVars) > 0 {
			thenFacts = DropFactSubjectsByVarsSess(cgSess(cg), thenFacts, st.Else.LocalVars)
			thenUnions = DropUnionSubjectsByVarsSess(cgSess(cg), thenUnions, st.Else.LocalVars)
			if !FactsComplete(thenFacts) || !UnionFactsComplete(thenUnions) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
		}
		if len(st.Then.LocalVars) > 0 {
			elseFacts = DropFactSubjectsByVarsSess(cgSess(cg), elseFacts, st.Then.LocalVars)
			elseUnions = DropUnionSubjectsByVarsSess(cgSess(cg), elseUnions, st.Then.LocalVars)
			if !FactsComplete(elseFacts) || !UnionFactsComplete(elseUnions) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
		}
	}

	// StatementIf.cpp:178–180 — set_accumulated_effect_after_block(eff, &if_true/false)
	// eff is mutated across both calls: cond + then + else map_stm_effect[block]
	// Block stm_id always live; StmID 0 fails closed (no invent EffectStm soft fallback)
	// Incomplete arm effects sticky (no invent SetMapStmEffect incomplete then true)
	if cg.FM != nil {
		if StmIDUnset(st.Then.StmID) || StmIDUnset(st.Else.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if StmIDUnset(st.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if !EffectComplete(condEff) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		thenE := cg.FM.GetMapStmEffect(st.Then.StmID)
		elseE := cg.FM.GetMapStmEffect(st.Else.StmID)
		if !EffectComplete(thenE) || !EffectComplete(elseE) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		acc := condEff.AddEffect(thenE)
		// residual ERROR sticky — no invent soft-continue else merge past then AddEffect residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		if !EffectComplete(acc) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		acc = acc.AddEffect(elseE)
		// residual ERROR sticky — no invent soft-continue set-map past else AddEffect residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		if !EffectComplete(acc) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		cg.FM.SetMapStmEffect(st.StmID, acc)
		// residual ERROR sticky — no invent soft-continue visit past SetMapStmEffect residual
		if sessHasError(cgSess(cg)) {
			return false
		}
	}

	// StatementIf.cpp:185–196 — must_return pruning (full FactVec)
	if cg.FM != nil {
		trueMust := st.Then.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Then MustReturn residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		falseMust := st.Else.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Else MustReturn residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		installU := func(u []*FactUnion) bool {
			if !UnionFactsComplete(u) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cl := CloneUnionFactSliceDeepSess(cgSess(cg), u)
			if sessHasError(cgSess(cg)) || !UnionFactsComplete(cl) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			if cl == nil {
				cg.FM.UnionFacts = []*FactUnion{}
			} else {
				cg.FM.UnionFacts = cl
			}
			return true
		}
		switch {
		case trueMust && falseMust:
			if !FactsComplete(inputsCopy) || !UnionFactsComplete(inputsCopyU) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(inputsCopy), "auto_statement_visit_419")
			if sessHasError(cgSess(cg)) || !installU(inputsCopyU) {
				return false
			}
		case trueMust:
			if !FactsComplete(elseFacts) || !UnionFactsComplete(elseUnions) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(elseFacts, "auto_statement_visit_427")
			if sessHasError(cgSess(cg)) || !installU(elseUnions) {
				return false
			}
		case falseMust:
			if !FactsComplete(thenFacts) || !UnionFactsComplete(thenUnions) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_435")
			if sessHasError(cgSess(cg)) || !installU(thenUnions) {
				return false
			}
		default:
			if !FactsComplete(thenFacts) || !FactsComplete(elseFacts) ||
				!UnionFactsComplete(thenUnions) || !UnionFactsComplete(elseUnions) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_443")
			_ = MergeFactsSess(cgSess(cg), &cg.FM.GlobalFacts, elseFacts)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			if !installU(thenUnions) {
				return false
			}
			for _, uf := range elseUnions {
				merged := MergeUnionFactSess(cgSess(cg), cg.FM.UnionFacts, uf)
				if !UnionFactsComplete(merged) {
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					return false
				}
				cg.FM.UnionFacts = merged
			}
		}
	}
	// StatementIf.cpp:170–177 — effect_accum already grew through both arms on
	// the shared cg_context; do not MergeEffects of forked arm snapshots.
	return true
}

// VisitFactsStatementFor mirrors StatementFor::visit_facts.
// StatementFor.cpp:427–472 — init; IV bound; body; map_facts_in[body] or
// post-init restore on must_return; merge post_dest break edges; accum effect.
// Hard IR incomplete sticky (nil Loop/IV/body/init, StmID 0, incomplete maps/effect);
// visit/body policy fails stay non-sticky false.
func VisitFactsStatementFor(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementFor.cpp:430+ — always has init StatementAssign, body Block, IV sticky
	if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementFor.cpp:430–432 — init StatementAssign always live sticky
	if st.Loop.InitStmt == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !VisitFactsStatementAssign(st.Loop.InitStmt, cg, opts) {
		return false
	}
	// StatementFor.cpp:433–434 — facts_copy / effect_stm after init (full FactVec)
	var factsCopy []*FactPointTo
	var factsCopyU []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-for visit past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		factsCopyU = CloneUnionFactSliceDeepSess(cgSess(cg), cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(factsCopyU) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
	eff := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-for visit past EffectStm Clone residual
	if sessHasError(cgSess(cg)) {
		return false
	}

	iv := st.Loop.IV
	// StatementFor.cpp:440 — assert(iv->type->eType == eSimple) sticky
	if iv.Type == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	if !iv.Type.IsSimple() {
		// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual
		if sessHasError(cgSess(cg)) {
			return false
		}
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual true
	if sessHasError(cgSess(cg)) {
		return false
	}
	// StatementFor.cpp:441 — assert(iv_bounds.find(iv) == end); hard sticky re-bind
	if cg.IVBounds != nil {
		if _, ok := cg.IVBounds[iv]; ok {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
	}
	// StatementFor.cpp:443 — arbitrary bound 0
	cg.AddIVBound(iv, 0)
	// StatementFor.cpp:445–449 — body.visit_facts(inputs, cg_context) on the
	// SAME CGContext (only iv_bounds mutated). Soft invent was CloneSubcontext +
	// Flags|=IN_LOOP (C++ visit does not; generation uses loop-body ctor).
	if !VisitFactsBlock(st.Then, cg, opts) {
		// StatementFor.cpp:446–448 — erase IV on body fail
		cg.RemoveIVBound(iv)
		return false
	}
	// Cleanup IV on every exit after body success (StatementFor.cpp:470).
	defer cg.RemoveIVBound(iv)
	if cg.FM != nil {
		// StatementFor.cpp:452–458 — body Block always has stm_id sticky
		if StmIDUnset(st.Then.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if st.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if sessHasError(cgSess(cg)) {
				return false
			}
			// control reaches end of for with pre-loop (post-init) env — full FactVec
			if !UnionFactsComplete(factsCopyU) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(factsCopy), "auto_statement_visit_559")
			if sessHasError(cgSess(cg)) {
				return false
			}
			clU := CloneUnionFactSliceDeepSess(cgSess(cg), factsCopyU)
			if sessHasError(cgSess(cg)) || !UnionFactsComplete(clU) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			if clU == nil {
				cg.FM.UnionFacts = []*FactUnion{}
			} else {
				cg.FM.UnionFacts = clU
			}
		} else {
			// residual ERROR sticky — no invent soft-continue map_facts_in path past MustReturn residual false
			if sessHasError(cgSess(cg)) {
				return false
			}
			// map_facts_in[&body] — fixed-point entry full FactVec
			cg.FM.AssignGlobalFactsFromMapIn(st.Then.StmID)
			if sessHasError(cgSess(cg)) {
				return false
			}
			// Drop body LocalVars from restored entry (see post_loop / DropFactSubjectsByVars).
			if len(st.Then.LocalVars) > 0 {
				cg.FM.GlobalFacts = DropFactSubjectsByVarsSess(cgSess(cg), cg.FM.GlobalFacts, st.Then.LocalVars)
				cg.FM.UnionFacts = DropUnionSubjectsByVarsSess(cgSess(cg), cg.FM.UnionFacts, st.Then.LocalVars)
				if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					return false
				}
			}
		}
		// StatementFor.cpp:460–466 / post_loop_analysis:361–367 —
		// find_edges_in(true, false) on this for stmt (break edges dest = for-stmt)
		// nil FindEdgesIn sticky incomplete CFG; tryMergeJumpFacts already sticky
		if StmIDUnset(st.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			// FactMgr.cpp:569–588 — merge_jump_facts is full FactVec (ePointTo + eUnionWrite).
			// Soft invent was PT-only tryMergeJumpFacts; break arms' eUnionWrite never joined
			// (post_loop already merges both; visit path must match StatementFor.cpp:465).
			if _, mok := tryMergeJumpFactsSess(cgSess(cg), &cg.FM.GlobalFacts, out); !mok {
				return false
			}
			outU := cg.FM.GetMapUnionFactsOut(e.SrcID)
			if !mergeJumpUnionFactsSess(cgSess(cg), &cg.FM.UnionFacts, outU) {
				return false
			}
		}
	}
	// StatementFor.cpp:468 — set_accumulated_effect_after_block(eff, &body, …)
	// Incomplete body/pre effect sticky (no invent visit true with IncompleteEffect map)
	bodyEff := EmptyEffect()
	if cg.FM != nil {
		if StmIDUnset(st.Then.StmID) || StmIDUnset(st.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		bodyEff = cg.FM.GetMapStmEffect(st.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(eff) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	} else if !EffectComplete(eff) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	SetAccumulatedEffectAfterBlock(st, bodyEff, cg, eff)
	if cg.FM != nil && !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	return true
}

// VisitFactsStatementArrayOp mirrors StatementArrayOp::visit_facts.
// StatementArrayOp.cpp:268–318 — write each IV; body or init_value assign path.
// Hard IR incomplete sticky (nil Loop/IV/body, StmID 0, incomplete maps/effect);
// CheckWrite/visit policy fails stay non-sticky false.
func VisitFactsStatementArrayOp(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// collect IVs from nested ArrayOp Loop chain
	// StatementArrayOp always has live ctrl_vars[i]; incomplete Loop/IV sticky
	var ivs []*Variable
	for cur := st; cur != nil && cur.Kind == StmtArrayOp; {
		if cur.Loop == nil || cur.Loop.IV == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		ivs = append(ivs, cur.Loop.IV)
		// nested: Then may hold next ArrayOp as first stmt
		if cur.Then == nil || len(cur.Then.Stmts) == 0 {
			break
		}
		next := &cur.Then.Stmts[0]
		if next.Kind != StmtArrayOp {
			break
		}
		cur = next
	}
	// StatementArrayOp.cpp:270–275 — empty ctrl chain incomplete sticky
	if len(ivs) == 0 {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementArrayOp.cpp:270–275 — check_write_var each ctrl var
	// IV nil sticky; CheckWrite policy fail non-sticky
	facts := cg.pointToFacts()
	for _, iv := range ivs {
		if iv == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if !cg.CheckWriteVar(iv, facts) {
			return false
		}
		facts = cg.pointToFacts()
	}

	// find innermost body assign (array init) or nested block body
	inner := findArrayOpInnermost(st)
	// StatementArrayOp.cpp:276–317 — body OR init_value path; neither is incomplete sticky
	if inner == nil || inner.Then == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}

	// body path: nested fors around a Block of statements (array loop)
	// init path: Then is a block whose first stmt is assign with ArrayAccess
	if isArrayInitBody(inner.Then) {
		// StatementArrayOp.cpp:299–316 — init_value + lhs visit + update_fact_for_assign
		// StatementAssign always has live Lhs + Expression*
		asg := &inner.Then.Stmts[0]
		if asg.Expr == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if !VisitFactsExpression(asg.Expr, cg, opts) {
			return false
		}
		if asg.LhsVar == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		lhs := &Lhs{Var: asg.LhsVar, Type: asg.LhsVar.Type}
		if !cg.VisitFactsLhs(lhs, opts) {
			return false
		}
		if cg.FM != nil {
			// Statement::stm_id always live; StmID 0 sticky
			if StmIDUnset(st.StmID) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			_ = cg.FM.UpdateFactForAssign(asg.LhsVar, 0, asg.Expr)
			// incomplete assign sticky (no invent visit success / soft re-pick)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			// Incomplete EffectStm sticky
			if !EffectComplete(cg.EffectStm) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			effCl := cg.EffectStm.Clone()
			// residual ERROR sticky — no invent soft-map effect past IncompleteEffect Clone residual
			if sessHasError(cgSess(cg)) {
				return false
			}
			cg.FM.SetMapStmEffect(st.StmID, effCl)
		}
		return true
	}

	// body path — StatementArrayOp.cpp:277–297 (same shape as StatementFor visit)
	// incomplete GlobalFacts sticky (no invent cleaned pre-body snapshot)
	ptFacts := cg.pointToFacts()
	if !FactsComplete(ptFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	preFacts := CloneFactSlice(ptFacts)
	// residual ERROR sticky — no invent soft-arrayop visit past CloneFactSlice residual
	if sessHasError(cgSess(cg)) {
		return false
	}
	preStm := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-arrayop visit past EffectStm Clone residual
	if sessHasError(cgSess(cg)) {
		return false
	}
	// StatementArrayOp.cpp:284–287 — body->visit_facts(inputs, cg_context) on the
	// SAME context (IVs already written via check_write_var above). Soft invent was
	// CloneSubcontext + IN_LOOP.
	if !VisitFactsBlock(inner.Then, cg, opts) {
		return false
	}
	if cg.FM != nil {
		// StatementArrayOp.cpp:285–291 — body Block always has stm_id sticky
		if StmIDUnset(inner.Then.StmID) || StmIDUnset(st.StmID) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		if inner.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if sessHasError(cgSess(cg)) {
				return false
			}
			cg.FM.SetGlobalFacts(preFacts, "auto_statement_visit_770")
		} else {
			// residual ERROR sticky — no invent soft-continue map_facts_in path past MustReturn residual false
			if sessHasError(cgSess(cg)) {
				return false
			}
			// map_facts_in[&body] — C++ map[] always; missing → empty
			in := cg.FM.GetMapFactsIn(inner.Then.StmID)
			if !FactsComplete(in) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(in), "auto_statement_visit_784")
		}
		// StatementArrayOp.cpp:292–297 — find_edges_in(true, false) on this stmt
		// nil FindEdgesIn sticky incomplete CFG
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			if _, mok := tryMergeJumpFactsSess(cgSess(cg), &cg.FM.GlobalFacts, out); !mok {
				return false
			}
		}
		// StatementArrayOp.cpp:298–299 — set_accumulated_effect_after_block
		// Incomplete body/pre sticky
		bodyEff := cg.FM.GetMapStmEffect(inner.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(preStm) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		SetAccumulatedEffectAfterBlock(st, bodyEff, cg, preStm)
		if !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
	return true
}

func findArrayOpInnermost(st *Stmt) *Stmt {
	// Statement always live; sticky incomplete no invent nil soft-skip
	if st == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	cur := st
	for cur.Then != nil && len(cur.Then.Stmts) == 1 && cur.Then.Stmts[0].Kind == StmtArrayOp {
		cur = &cur.Then.Stmts[0]
	}
	return cur
}

func isArrayInitBody(b *Block) bool {
	// Block always live for array-init body check; sticky incomplete no invent false soft-skip
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(b.Stmts) != 1 {
		return false
	}
	s := &b.Stmts[0]
	return s.Kind == StmtAssign && s.ArrayAccess != ""
}
