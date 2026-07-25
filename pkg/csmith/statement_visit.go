// Upstream: StatementIf/StatementFor/Block visit_facts analysis paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStmt dispatches Statement::visit_facts by kind.
// Statement subclasses — assign/if/for/block/return/jump/expr.
// Incomplete IR fails closed sticky (no soft invent true / soft re-pick past holes).
func VisitFactsStmt(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
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
				noteErrCG(cg, ErrGeneric)
				return false
			}
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
		return true
	case StmtInvoke:
		return VisitFactsStatementExpr(st, cg, opts)
	case StmtBlock:
		if st.Then == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		return VisitFactsBlock(st.Then, cg, opts)
	default:
		// unknown kind hard IR sticky
		noteErrCG(cg, ErrGeneric)
		return false
	}
}

// VisitFactsStatementJump mirrors StatementBreak/Continue::visit_facts.
// StatementBreak.cpp:126–134 / StatementContinue.cpp:125–133 — test then effect_stm.
func VisitFactsStatementJump(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// C++ always has live test Expression*; nil Expr sticky hard IR
	if st.Expr == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM fails closed sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementGoto.cpp:366–368 — test.visit_facts always; nil Expr sticky
	if st.Expr == nil {
		noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	for _, v := range st.InitSkippedVars {
		if v == nil {
			noteErrCG(cg, ErrGeneric)
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
			noteErrCG(cg, ErrGeneric)
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
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			if !UnionFactsComplete(fm.UnionFacts) {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			curU := fm.UnionFacts
			if !visitedThis && !visitedDest &&
				!SameFactVecSess(sessFromCG(cg), cur, curU, prevOut, prevOutU) &&
				SubsetFactVecSess(sessFromCG(cg), cur, curU, prevOut, prevOutU) {
				if hasErrCG(cg) {
					return false
				}
				delete(fm.MapFactsIn, destID)
				delete(fm.MapFactsOut, destID)
				delete(fm.MapUnionFactsIn, destID)
				delete(fm.MapUnionFactsOut, destID)
			}
			if hasErrCG(cg) {
				return false
			}
		}
		// Incomplete EffectStm sticky (no invent visit true with incomplete map)
		if !EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementExpr.cpp:106 — expr always live; nil Expr sticky
	if st.Expr == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// Block.cpp:472 — Effect pre_effect = cg_context.get_accum_effect();
	var preEffect Effect
	havePre := false
	if cg.EffectAccum != nil {
		if !EffectComplete(*cg.EffectAccum) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		preEffect = cg.EffectAccum.CloneSess(sessFromCG(cg))
		if hasErrCG(cg) {
			return false
		}
		havePre = true
	}
	var inputs []*FactPointTo
	if cg.FM != nil {
		// incomplete GlobalFacts sticky (no invent fixed-point / soft re-pick past holes)
		if !FactsComplete(cg.FM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		inputs = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-fixed-point past CloneFactSlice residual
		if hasErrCG(cg) {
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
			if hasErrCG(cg) {
				return false
			}
		} else if FactsComplete(out) {
			cl := CloneFactSliceSess(sessFromCG(cg), out)
			if hasErrCG(cg) || !FactsComplete(cl) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementIf.cpp:164 — inputs_copy before condition (full FactVec)
	var inputsCopy []*FactPointTo
	var inputsCopyU []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		inputsCopy = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if visit past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		inputsCopyU = CloneUnionFactSliceDeepSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(inputsCopyU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	}
	// StatementIf.cpp:165–168 — evaluate condition first; nil test sticky
	if st.Expr == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// StatementIf.cpp:169 — effect_stm after condition
	condEff := cg.EffectStm.CloneSess(sessFromCG(cg))
	// residual ERROR sticky — no invent soft-if arms past EffectStm Clone residual
	if hasErrCG(cg) {
		return false
	}
	// post-condition env shared as entry to both arms (full FactVec)
	var postCond []*FactPointTo
	var postCondU []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		postCond = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if arms past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		postCondU = CloneUnionFactSliceDeepSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(postCondU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	}
	// StatementIf.cpp:170–177 — both arms always live Blocks sticky
	if st.Then == nil || st.Else == nil {
		noteErrCG(cg, ErrGeneric)
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
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		thenFacts = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-then facts past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		thenUnions = CloneUnionFactSliceDeepSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(thenUnions) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		// StatementIf.cpp:174 — false starts from same post-cond inputs (not after true)
		// Full FactVec: generation uses AssignGlobalFactsFromMapIn (PT+union).
		// Soft invent was SetGlobalFacts(PT-only) → else kept then-exit last-writes
		// (seed-7 nested loop FP over-strip of back-edge gotos).
		// effect_accum is NOT reset (C++ continues growing through false arm).
		cg.FM.SetGlobalFacts(CloneFactSliceSess(sessFromCG(cg), postCond), "auto_statement_visit_317")
		// residual ERROR sticky — no invent soft-else start past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		clU := CloneUnionFactSliceDeepSess(sessFromCG(cg), postCondU)
		if hasErrCG(cg) || !UnionFactsComplete(clU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
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
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		elseFacts = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-else facts past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		elseUnions = CloneUnionFactSliceDeepSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(elseUnions) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
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
			thenFacts = DropFactSubjectsByVarsSess(sessFromCG(cg), thenFacts, st.Else.LocalVars)
			thenUnions = DropUnionSubjectsByVarsSess(sessFromCG(cg), thenUnions, st.Else.LocalVars)
			if !FactsComplete(thenFacts) || !UnionFactsComplete(thenUnions) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
		}
		if len(st.Then.LocalVars) > 0 {
			elseFacts = DropFactSubjectsByVarsSess(sessFromCG(cg), elseFacts, st.Then.LocalVars)
			elseUnions = DropUnionSubjectsByVarsSess(sessFromCG(cg), elseUnions, st.Then.LocalVars)
			if !FactsComplete(elseFacts) || !UnionFactsComplete(elseUnions) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
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
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if StmIDUnset(st.StmID) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !EffectComplete(condEff) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		thenE := cg.FM.GetMapStmEffect(st.Then.StmID)
		elseE := cg.FM.GetMapStmEffect(st.Else.StmID)
		if !EffectComplete(thenE) || !EffectComplete(elseE) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		acc := condEff.AddEffectSess(sessFromCG(cg), thenE)
		// residual ERROR sticky — no invent soft-continue else merge past then AddEffect residual
		if hasErrCG(cg) {
			return false
		}
		if !EffectComplete(acc) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		acc = acc.AddEffectSess(sessFromCG(cg), elseE)
		// residual ERROR sticky — no invent soft-continue set-map past else AddEffect residual
		if hasErrCG(cg) {
			return false
		}
		if !EffectComplete(acc) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		cg.FM.SetMapStmEffect(st.StmID, acc)
		// residual ERROR sticky — no invent soft-continue visit past SetMapStmEffect residual
		if hasErrCG(cg) {
			return false
		}
	}

	// StatementIf.cpp:185–196 — must_return pruning (full FactVec)
	if cg.FM != nil {
		trueMust := st.Then.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Then MustReturn residual
		if hasErrCG(cg) {
			return false
		}
		falseMust := st.Else.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Else MustReturn residual
		if hasErrCG(cg) {
			return false
		}
		installU := func(u []*FactUnion) bool {
			if !UnionFactsComplete(u) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cl := CloneUnionFactSliceDeepSess(sessFromCG(cg), u)
			if hasErrCG(cg) || !UnionFactsComplete(cl) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
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
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSliceSess(sessFromCG(cg), inputsCopy), "auto_statement_visit_419")
			if hasErrCG(cg) || !installU(inputsCopyU) {
				return false
			}
		case trueMust:
			if !FactsComplete(elseFacts) || !UnionFactsComplete(elseUnions) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(elseFacts, "auto_statement_visit_427")
			if hasErrCG(cg) || !installU(elseUnions) {
				return false
			}
		case falseMust:
			if !FactsComplete(thenFacts) || !UnionFactsComplete(thenUnions) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_435")
			if hasErrCG(cg) || !installU(thenUnions) {
				return false
			}
		default:
			if !FactsComplete(thenFacts) || !FactsComplete(elseFacts) ||
				!UnionFactsComplete(thenUnions) || !UnionFactsComplete(elseUnions) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_443")
			_ = MergeFactsSess(sessFromCG(cg), &cg.FM.GlobalFacts, elseFacts)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			if !installU(thenUnions) {
				return false
			}
			for _, uf := range elseUnions {
				merged := MergeUnionFactSess(sessFromCG(cg), cg.FM.UnionFacts, uf)
				if !UnionFactsComplete(merged) {
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementFor.cpp:430+ — always has init StatementAssign, body Block, IV sticky
	if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementFor.cpp:430–432 — init StatementAssign always live sticky
	if st.Loop.InitStmt == nil {
		noteErrCG(cg, ErrGeneric)
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
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		factsCopy = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-for visit past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		factsCopyU = CloneUnionFactSliceDeepSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(factsCopyU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	}
	eff := cg.EffectStm.CloneSess(sessFromCG(cg))
	// residual ERROR sticky — no invent soft-for visit past EffectStm Clone residual
	if hasErrCG(cg) {
		return false
	}

	iv := st.Loop.IV
	// StatementFor.cpp:440 — assert(iv->type->eType == eSimple) sticky
	if iv.Type == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if !iv.Type.IsSimpleSess(cgSess(cg)) {
		// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual
		if hasErrCG(cg) {
			return false
		}
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual true
	if hasErrCG(cg) {
		return false
	}
	// StatementFor.cpp:441 — assert(iv_bounds.find(iv) == end); hard sticky re-bind
	if cg.IVBounds != nil {
		if _, ok := cg.IVBounds[iv]; ok {
			noteErrCG(cg, ErrGeneric)
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
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if st.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if hasErrCG(cg) {
				return false
			}
			// control reaches end of for with pre-loop (post-init) env — full FactVec
			if !UnionFactsComplete(factsCopyU) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSliceSess(sessFromCG(cg), factsCopy), "auto_statement_visit_559")
			if hasErrCG(cg) {
				return false
			}
			clU := CloneUnionFactSliceDeepSess(sessFromCG(cg), factsCopyU)
			if hasErrCG(cg) || !UnionFactsComplete(clU) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
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
			if hasErrCG(cg) {
				return false
			}
			// map_facts_in[&body] — fixed-point entry full FactVec
			cg.FM.AssignGlobalFactsFromMapIn(st.Then.StmID)
			if hasErrCG(cg) {
				return false
			}
			// Drop body LocalVars from restored entry (see post_loop / DropFactSubjectsByVars).
			if len(st.Then.LocalVars) > 0 {
				cg.FM.GlobalFacts = DropFactSubjectsByVarsSess(sessFromCG(cg), cg.FM.GlobalFacts, st.Then.LocalVars)
				cg.FM.UnionFacts = DropUnionSubjectsByVarsSess(sessFromCG(cg), cg.FM.UnionFacts, st.Then.LocalVars)
				if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
					}
					return false
				}
			}
		}
		// StatementFor.cpp:460–466 / post_loop_analysis:361–367 —
		// find_edges_in(true, false) on this for stmt (break edges dest = for-stmt)
		// nil FindEdgesIn sticky incomplete CFG; tryMergeJumpFacts already sticky
		if StmIDUnset(st.StmID) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			// FactMgr.cpp:569–588 — merge_jump_facts is full FactVec (ePointTo + eUnionWrite).
			// Soft invent was PT-only tryMergeJumpFacts; break arms' eUnionWrite never joined
			// (post_loop already merges both; visit path must match StatementFor.cpp:465).
			if _, mok := tryMergeJumpFactsSess(sessFromCG(cg), &cg.FM.GlobalFacts, out); !mok {
				return false
			}
			outU := cg.FM.GetMapUnionFactsOut(e.SrcID)
			if !mergeJumpUnionFactsSess(sessFromCG(cg), &cg.FM.UnionFacts, outU) {
				return false
			}
		}
	}
	// StatementFor.cpp:468 — set_accumulated_effect_after_block(eff, &body, …)
	// Incomplete body/pre effect sticky (no invent visit true with IncompleteEffect map)
	bodyEff := EmptyEffect()
	if cg.FM != nil {
		if StmIDUnset(st.Then.StmID) || StmIDUnset(st.StmID) {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		bodyEff = cg.FM.GetMapStmEffect(st.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(eff) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	} else if !EffectComplete(eff) {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	SetAccumulatedEffectAfterBlock(st, bodyEff, cg, eff)
	if cg.FM != nil && !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// collect IVs from nested ArrayOp Loop chain
	// StatementArrayOp always has live ctrl_vars[i]; incomplete Loop/IV sticky
	var ivs []*Variable
	for cur := st; cur != nil && cur.Kind == StmtArrayOp; {
		if cur.Loop == nil || cur.Loop.IV == nil {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// StatementArrayOp.cpp:270–275 — check_write_var each ctrl var
	// IV nil sticky; CheckWrite policy fail non-sticky
	facts := cg.pointToFacts()
	for _, iv := range ivs {
		if iv == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !cg.CheckWriteVar(iv, facts) {
			return false
		}
		facts = cg.pointToFacts()
	}

	// find innermost body assign (array init) or nested block body
	inner := findArrayOpInnermostSess(sessFromCG(cg), st)
	// StatementArrayOp.cpp:276–317 — body OR init_value path; neither is incomplete sticky
	if inner == nil || inner.Then == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}

	// body path: nested fors around a Block of statements (array loop)
	// init path: Then is a block whose first stmt is assign with ArrayAccess
	if isArrayInitBodySess(sessFromCG(cg), inner.Then) {
		// StatementArrayOp.cpp:299–316 — init_value + lhs visit + update_fact_for_assign
		// StatementAssign always has live Lhs + Expression*
		asg := &inner.Then.Stmts[0]
		if asg.Expr == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if !VisitFactsExpression(asg.Expr, cg, opts) {
			return false
		}
		if asg.LhsVar == nil {
			noteErrCG(cg, ErrGeneric)
			return false
		}
		lhs := &Lhs{Var: asg.LhsVar, Type: asg.LhsVar.Type}
		if !cg.VisitFactsLhs(lhs, opts) {
			return false
		}
		if cg.FM != nil {
			// Statement::stm_id always live; StmID 0 sticky
			if StmIDUnset(st.StmID) {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			_ = cg.FM.UpdateFactForAssign(asg.LhsVar, 0, asg.Expr)
			// incomplete assign sticky (no invent visit success / soft re-pick)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			// Incomplete EffectStm sticky
			if !EffectComplete(cg.EffectStm) {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			effCl := cg.EffectStm.CloneSess(sessFromCG(cg))
			// residual ERROR sticky — no invent soft-map effect past IncompleteEffect Clone residual
			if hasErrCG(cg) {
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
		noteErrCG(cg, ErrGeneric)
		return false
	}
	preFacts := CloneFactSliceSess(sessFromCG(cg), ptFacts)
	// residual ERROR sticky — no invent soft-arrayop visit past CloneFactSlice residual
	if hasErrCG(cg) {
		return false
	}
	preStm := cg.EffectStm.CloneSess(sessFromCG(cg))
	// residual ERROR sticky — no invent soft-arrayop visit past EffectStm Clone residual
	if hasErrCG(cg) {
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
			noteErrCG(cg, ErrGeneric)
			return false
		}
		if inner.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if hasErrCG(cg) {
				return false
			}
			cg.FM.SetGlobalFacts(preFacts, "auto_statement_visit_770")
		} else {
			// residual ERROR sticky — no invent soft-continue map_facts_in path past MustReturn residual false
			if hasErrCG(cg) {
				return false
			}
			// map_facts_in[&body] — C++ map[] always; missing → empty
			in := cg.FM.GetMapFactsIn(inner.Then.StmID)
			if !FactsComplete(in) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSliceSess(sessFromCG(cg), in), "auto_statement_visit_784")
		}
		// StatementArrayOp.cpp:292–297 — find_edges_in(true, false) on this stmt
		// nil FindEdgesIn sticky incomplete CFG
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			if _, mok := tryMergeJumpFactsSess(sessFromCG(cg), &cg.FM.GlobalFacts, out); !mok {
				return false
			}
		}
		// StatementArrayOp.cpp:298–299 — set_accumulated_effect_after_block
		// Incomplete body/pre sticky
		bodyEff := cg.FM.GetMapStmEffect(inner.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(preStm) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		SetAccumulatedEffectAfterBlock(st, bodyEff, cg, preStm)
		if !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	}
	return true
}

// Non-Sess findArrayOpInnermost deleted — pass run bag or testAmbientSession explicitly.

// findArrayOpInnermostSess is findArrayOpInnermost with explicit session residual sticky.
func findArrayOpInnermostSess(s *Session, st *Stmt) *Stmt {
	// Statement always live; sticky incomplete no invent nil soft-skip
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	cur := st
	for cur.Then != nil && len(cur.Then.Stmts) == 1 && cur.Then.Stmts[0].Kind == StmtArrayOp {
		cur = &cur.Then.Stmts[0]
	}
	return cur
}

// Non-Sess isArrayInitBody deleted — pass run bag or testAmbientSession explicitly.

// isArrayInitBodySess is isArrayInitBody with explicit session residual sticky.
func isArrayInitBodySess(s *Session, b *Block) bool {
	// Block always live for array-init body check; sticky incomplete no invent false soft-skip
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if len(b.Stmts) != 1 {
		return false
	}
	s0 := &b.Stmts[0]
	return s0.Kind == StmtAssign && s0.ArrayAccess != ""
}
