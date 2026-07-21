// Upstream: StatementIf/StatementFor/Block visit_facts analysis paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStmt dispatches Statement::visit_facts by kind.
// Statement subclasses — assign/if/for/block/return/jump/expr.
// Incomplete IR fails closed sticky (no soft invent true / soft re-pick past holes).
func VisitFactsStmt(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		SetError(ErrGeneric)
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
				SetError(ErrGeneric)
				return false
			}
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
		return true
	case StmtInvoke:
		return VisitFactsStatementExpr(st, cg, opts)
	case StmtBlock:
		if st.Then == nil {
			SetError(ErrGeneric)
			return false
		}
		return VisitFactsBlock(st.Then, cg, opts)
	default:
		// unknown kind hard IR sticky
		SetError(ErrGeneric)
		return false
	}
}

// VisitFactsStatementJump mirrors StatementBreak/Continue::visit_facts.
// StatementBreak.cpp:126–134 / StatementContinue.cpp:125–133 — test then effect_stm.
func VisitFactsStatementJump(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	// C++ always has live test Expression*; nil Expr sticky hard IR
	if st.Expr == nil {
		SetError(ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM fails closed sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// StatementGoto.cpp:366–368 — test.visit_facts always; nil Expr sticky
	if st.Expr == nil {
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	for _, v := range st.InitSkippedVars {
		if v == nil {
			SetError(ErrGeneric)
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
			SetError(ErrGeneric)
			return false
		}
		// StatementGoto.cpp:390–398 — force dest re-analysis when current outs
		// are proper subset of previous outs and neither visited this pass.
		destID := st.GotoDestStmID
		if destID > 0 {
			visitedThis := fm.MapVisited != nil && fm.MapVisited[st.StmID]
			visitedDest := fm.MapVisited != nil && fm.MapVisited[destID]
			prevOut := fm.GetMapFactsOut(st.StmID)
			cur := facts
			// incomplete prev outs sticky (GetMapFactsOut may already SetError)
			if !FactsComplete(prevOut) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			if !visitedThis && !visitedDest &&
				!SameFacts(cur, prevOut) &&
				SubsetFacts(cur, prevOut) {
				delete(fm.MapFactsIn, destID)
				delete(fm.MapFactsOut, destID)
			}
		}
		// Incomplete EffectStm sticky (no invent visit true with incomplete map)
		if !EffectComplete(cg.EffectStm) {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// StatementExpr.cpp:106 — expr always live; nil Expr sticky
	if st.Expr == nil {
		SetError(ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// Statement::stm_id always live; StmID 0 + FM sticky
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if cg.FM != nil {
		if StmIDUnset(st.StmID) || !EffectComplete(cg.EffectStm) {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// Block.cpp:472 — Effect pre_effect = cg_context.get_accum_effect();
	var preEffect Effect
	havePre := false
	if cg.EffectAccum != nil {
		if !EffectComplete(*cg.EffectAccum) {
			SetError(ErrGeneric)
			return false
		}
		preEffect = cg.EffectAccum.Clone()
		if HasError() {
			return false
		}
		havePre = true
	}
	var inputs []*FactPointTo
	if cg.FM != nil {
		// incomplete GlobalFacts sticky (no invent fixed-point / soft re-pick past holes)
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return false
		}
		inputs = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-fixed-point past CloneFactSlice residual
		if HasError() {
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
	out, _, ok := FindFixedPointBlock(b, inputs, cg, opts, false)
	if !ok {
		// Block.cpp:474–475 — cg_context.reset_effect_accum(pre_effect)
		if havePre && cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		return false
	}
	if cg.FM != nil && !StmIDUnset(b.StmID) {
		// Prefer map_facts_out (C++); fall back to FindFixedPointBlock return
		mout := cg.FM.GetMapFactsOut(b.StmID)
		if FactsComplete(mout) {
			cl := CloneFactSlice(mout)
			if HasError() || !FactsComplete(cl) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(cl, "auto_statement_visit_215")
		} else if FactsComplete(out) {
			cl := CloneFactSlice(out)
			if HasError() || !FactsComplete(cl) {
				if !HasError() {
					SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// StatementIf.cpp:164 — inputs_copy before condition
	var inputsCopy []*FactPointTo
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return false
		}
		inputsCopy = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if visit past CloneFactSlice residual
		if HasError() {
			return false
		}
	}
	// StatementIf.cpp:165–168 — evaluate condition first; nil test sticky
	if st.Expr == nil {
		SetError(ErrGeneric)
		return false
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// StatementIf.cpp:169 — effect_stm after condition
	condEff := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-if arms past EffectStm Clone residual
	if HasError() {
		return false
	}
	// post-condition env shared as entry to both arms
	var postCond []*FactPointTo
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		postCond = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-if arms past CloneFactSlice residual
		if HasError() {
			return false
		}
	}
	preAccum := EmptyEffect()
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-if arms past EffectAccum Clone residual
		if HasError() {
			return false
		}
	}

	// StatementIf.cpp:170–177 — both arms always live Blocks sticky
	if st.Then == nil || st.Else == nil {
		SetError(ErrGeneric)
		return false
	}
	// StatementIf.cpp:170–173 — true branch from post-cond facts
	thenAccum := preAccum
	thenCG := cg.CloneSubcontext()
	thenCG.EffectAccum = &thenAccum
	if !VisitFactsBlock(st.Then, &thenCG, opts) {
		return false
	}
	var thenFacts []*FactPointTo
	if cg.FM != nil {
		// incomplete then-arm facts sticky (no invent soft re-pick past hole as visit success)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-then facts past CloneFactSlice residual
		if HasError() {
			return false
		}
		// StatementIf.cpp:174 — false starts from same post-cond inputs (not after true)
		cg.FM.SetGlobalFacts(CloneFactSlice(postCond), "auto_statement_visit_317")
		// residual ERROR sticky — no invent soft-else start past CloneFactSlice residual
		if HasError() {
			return false
		}
	}

	// StatementIf.cpp:174–177 — false branch
	elseAccum := preAccum
	elseCG := cg.CloneSubcontext()
	elseCG.EffectAccum = &elseAccum
	if !VisitFactsBlock(st.Else, &elseCG, opts) {
		return false
	}
	var elseFacts []*FactPointTo
	if cg.FM != nil {
		// incomplete else-arm facts sticky (no invent soft re-pick past hole as visit success)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		elseFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-else facts past CloneFactSlice residual
		if HasError() {
			return false
		}
	}

	// StatementIf.cpp:178–180 — set_accumulated_effect_after_block(eff, &if_true/false)
	// eff is mutated across both calls: cond + then + else map_stm_effect[block]
	// Block stm_id always live; StmID 0 fails closed (no invent EffectStm soft fallback)
	// Incomplete arm effects sticky (no invent SetMapStmEffect incomplete then true)
	if cg.FM != nil {
		if StmIDUnset(st.Then.StmID) || StmIDUnset(st.Else.StmID) {
			SetError(ErrGeneric)
			return false
		}
		if StmIDUnset(st.StmID) {
			SetError(ErrGeneric)
			return false
		}
		if !EffectComplete(condEff) {
			SetError(ErrGeneric)
			return false
		}
		thenE := cg.FM.GetMapStmEffect(st.Then.StmID)
		elseE := cg.FM.GetMapStmEffect(st.Else.StmID)
		if !EffectComplete(thenE) || !EffectComplete(elseE) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		acc := condEff.AddEffect(thenE)
		// residual ERROR sticky — no invent soft-continue else merge past then AddEffect residual
		if HasError() {
			return false
		}
		if !EffectComplete(acc) {
			SetError(ErrGeneric)
			return false
		}
		acc = acc.AddEffect(elseE)
		// residual ERROR sticky — no invent soft-continue set-map past else AddEffect residual
		if HasError() {
			return false
		}
		if !EffectComplete(acc) {
			SetError(ErrGeneric)
			return false
		}
		cg.FM.SetMapStmEffect(st.StmID, acc)
		// residual ERROR sticky — no invent soft-continue visit past SetMapStmEffect residual
		if HasError() {
			return false
		}
	}

	// StatementIf.cpp:185–196 — must_return pruning
	if cg.FM != nil {
		trueMust := st.Then.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Then MustReturn residual
		if HasError() {
			return false
		}
		falseMust := st.Else.MustReturn()
		// residual ERROR sticky — no invent soft-continue merge path past Else MustReturn residual
		if HasError() {
			return false
		}
		switch {
		case trueMust && falseMust:
			// pre-condition env (inputs_copy), not post-condition
			// incomplete inputs sticky (no invent soft re-pick past hole as visit success)
			if !FactsComplete(inputsCopy) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(inputsCopy), "auto_statement_visit_419")
		case trueMust:
			if !FactsComplete(elseFacts) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(elseFacts, "auto_statement_visit_427")
		case falseMust:
			if !FactsComplete(thenFacts) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_435")
		default:
			if !FactsComplete(thenFacts) || !FactsComplete(elseFacts) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(thenFacts, "auto_statement_visit_443")
			// MergeFacts clears GlobalFacts on incomplete mid-join — fail closed visit sticky
			_ = MergeFacts(&cg.FM.GlobalFacts, elseFacts)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
	}
	// parent accum: both arms observed (generation-time separates; visit matches merge)
	// Incomplete arm accum sticky (MergeEffects IncompleteEffect — no invent pure merge success)
	if cg.EffectAccum != nil {
		merged := MergeEffects(thenAccum, elseAccum)
		if !EffectComplete(merged) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		*cg.EffectAccum = merged
	}
	return true
}

// VisitFactsStatementFor mirrors StatementFor::visit_facts.
// StatementFor.cpp:427–472 — init; IV bound; body; map_facts_in[body] or
// post-init restore on must_return; merge post_dest break edges; accum effect.
// Hard IR incomplete sticky (nil Loop/IV/body/init, StmID 0, incomplete maps/effect);
// visit/body policy fails stay non-sticky false.
func VisitFactsStatementFor(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	// StatementFor.cpp:430+ — always has init StatementAssign, body Block, IV sticky
	if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		SetError(ErrGeneric)
		return false
	}
	// StatementFor.cpp:430–432 — init StatementAssign always live sticky
	if st.Loop.InitStmt == nil {
		SetError(ErrGeneric)
		return false
	}
	if !VisitFactsStatementAssign(st.Loop.InitStmt, cg, opts) {
		return false
	}
	// StatementFor.cpp:433–434 — facts_copy / effect_stm after init
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-for visit past CloneFactSlice residual
		if HasError() {
			return false
		}
	}
	eff := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-for visit past EffectStm Clone residual
	if HasError() {
		return false
	}

	iv := st.Loop.IV
	// StatementFor.cpp:440 — assert(iv->type->eType == eSimple) sticky
	if iv.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	if !iv.Type.IsSimple() {
		// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual
		if HasError() {
			return false
		}
		SetError(ErrGeneric)
		return false
	}
	// residual ERROR sticky — no invent soft-continue for-visit past IsSimple residual true
	if HasError() {
		return false
	}
	// StatementFor.cpp:441 — assert(iv_bounds.find(iv) == end); hard sticky re-bind
	if cg.IVBounds != nil {
		if _, ok := cg.IVBounds[iv]; ok {
			SetError(ErrGeneric)
			return false
		}
	}
	// StatementFor.cpp:443 — arbitrary bound 0
	cg.AddIVBound(iv, 0)
	defer cg.RemoveIVBound(iv)
	// StatementFor.cpp:445–449 — body under IN_LOOP (body uses shared accum)
	bodyCG := cg.CloneSubcontext()
	bodyCG.Flags |= FlagInLoop
	if !VisitFactsBlock(st.Then, &bodyCG, opts) {
		return false
	}
	if cg.FM != nil {
		// StatementFor.cpp:452–458 — body Block always has stm_id sticky
		if StmIDUnset(st.Then.StmID) {
			SetError(ErrGeneric)
			return false
		}
		if st.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if HasError() {
				return false
			}
			// control reaches end of for with pre-loop (post-init) env
			cg.FM.SetGlobalFacts(CloneFactSlice(factsCopy), "auto_statement_visit_559")
		} else {
			// residual ERROR sticky — no invent soft-continue map_facts_in path past MustReturn residual false
			if HasError() {
				return false
			}
			// map_facts_in[&body] — fixed-point entry, not merge(pre,post)
			// C++ map[] always assigns (missing → empty); no invent keep prior
			in := cg.FM.GetMapFactsIn(st.Then.StmID)
			if !FactsComplete(in) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(in), "auto_statement_visit_574")
		}
		// StatementFor.cpp:460–466 / post_loop_analysis:361–367 —
		// find_edges_in(true, false) on this for stmt (break edges dest = for-stmt)
		// nil FindEdgesIn sticky incomplete CFG; tryMergeJumpFacts already sticky
		if StmIDUnset(st.StmID) {
			SetError(ErrGeneric)
			return false
		}
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			// incomplete jump facts sticky via tryMergeJumpFacts
			if _, mok := tryMergeJumpFacts(&cg.FM.GlobalFacts, out); !mok {
				return false
			}
		}
	}
	// StatementFor.cpp:468 — set_accumulated_effect_after_block(eff, &body, …)
	// Incomplete body/pre effect sticky (no invent visit true with IncompleteEffect map)
	bodyEff := EmptyEffect()
	if cg.FM != nil {
		if StmIDUnset(st.Then.StmID) || StmIDUnset(st.StmID) {
			SetError(ErrGeneric)
			return false
		}
		bodyEff = cg.FM.GetMapStmEffect(st.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(eff) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
	} else if !EffectComplete(eff) {
		SetError(ErrGeneric)
		return false
	}
	SetAccumulatedEffectAfterBlock(st, bodyEff, cg, eff)
	if cg.FM != nil && !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
		if !HasError() {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// collect IVs from nested ArrayOp Loop chain
	// StatementArrayOp always has live ctrl_vars[i]; incomplete Loop/IV sticky
	var ivs []*Variable
	for cur := st; cur != nil && cur.Kind == StmtArrayOp; {
		if cur.Loop == nil || cur.Loop.IV == nil {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	// StatementArrayOp.cpp:270–275 — check_write_var each ctrl var
	// IV nil sticky; CheckWrite policy fail non-sticky
	facts := cg.pointToFacts()
	for _, iv := range ivs {
		if iv == nil {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}

	// body path: nested fors around a Block of statements (array loop)
	// init path: Then is a block whose first stmt is assign with ArrayAccess
	if isArrayInitBody(inner.Then) {
		// StatementArrayOp.cpp:299–316 — init_value + lhs visit + update_fact_for_assign
		// StatementAssign always has live Lhs + Expression*
		asg := &inner.Then.Stmts[0]
		if asg.Expr == nil {
			SetError(ErrGeneric)
			return false
		}
		if !VisitFactsExpression(asg.Expr, cg, opts) {
			return false
		}
		if asg.LhsVar == nil {
			SetError(ErrGeneric)
			return false
		}
		lhs := &Lhs{Var: asg.LhsVar, Type: asg.LhsVar.Type}
		if !cg.VisitFactsLhs(lhs, opts) {
			return false
		}
		if cg.FM != nil {
			// Statement::stm_id always live; StmID 0 sticky
			if StmIDUnset(st.StmID) {
				SetError(ErrGeneric)
				return false
			}
			_ = cg.FM.UpdateFactForAssign(asg.LhsVar, 0, asg.Expr)
			// incomplete assign sticky (no invent visit success / soft re-pick)
			if !FactsComplete(cg.FM.GlobalFacts) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			// Incomplete EffectStm sticky
			if !EffectComplete(cg.EffectStm) {
				SetError(ErrGeneric)
				return false
			}
			effCl := cg.EffectStm.Clone()
			// residual ERROR sticky — no invent soft-map effect past IncompleteEffect Clone residual
			if HasError() {
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
		SetError(ErrGeneric)
		return false
	}
	preFacts := CloneFactSlice(ptFacts)
	// residual ERROR sticky — no invent soft-arrayop visit past CloneFactSlice residual
	if HasError() {
		return false
	}
	preStm := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-arrayop visit past EffectStm Clone residual
	if HasError() {
		return false
	}
	bodyCG := cg.CloneSubcontext()
	bodyCG.Flags |= FlagInLoop
	// add all IVs as bounds for body analysis
	for _, iv := range ivs {
		bodyCG.AddIVBound(iv, 0)
		defer bodyCG.RemoveIVBound(iv)
	}
	if !VisitFactsBlock(inner.Then, &bodyCG, opts) {
		return false
	}
	if cg.FM != nil {
		// StatementArrayOp.cpp:285–291 — body Block always has stm_id sticky
		if StmIDUnset(inner.Then.StmID) || StmIDUnset(st.StmID) {
			SetError(ErrGeneric)
			return false
		}
		if inner.Then.MustReturn() {
			// residual ERROR sticky — no invent soft-continue pre-loop path past MustReturn residual true
			if HasError() {
				return false
			}
			cg.FM.SetGlobalFacts(preFacts, "auto_statement_visit_770")
		} else {
			// residual ERROR sticky — no invent soft-continue map_facts_in path past MustReturn residual false
			if HasError() {
				return false
			}
			// map_facts_in[&body] — C++ map[] always; missing → empty
			in := cg.FM.GetMapFactsIn(inner.Then.StmID)
			if !FactsComplete(in) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			cg.FM.SetGlobalFacts(CloneFactSlice(in), "auto_statement_visit_784")
		}
		// StatementArrayOp.cpp:292–297 — find_edges_in(true, false) on this stmt
		// nil FindEdgesIn sticky incomplete CFG
		edges := cg.FM.FindEdgesIn(st.StmID, true, false)
		if edges == nil {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		for _, e := range edges {
			out := cg.FM.GetMapFactsOut(e.SrcID)
			if _, mok := tryMergeJumpFacts(&cg.FM.GlobalFacts, out); !mok {
				return false
			}
		}
		// StatementArrayOp.cpp:298–299 — set_accumulated_effect_after_block
		// Incomplete body/pre sticky
		bodyEff := cg.FM.GetMapStmEffect(inner.Then.StmID)
		if !EffectComplete(bodyEff) || !EffectComplete(preStm) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		SetAccumulatedEffectAfterBlock(st, bodyEff, cg, preStm)
		if !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
	}
	return true
}

func findArrayOpInnermost(st *Stmt) *Stmt {
	// Statement always live; sticky incomplete no invent nil soft-skip
	if st == nil {
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	if len(b.Stmts) != 1 {
		return false
	}
	s := &b.Stmts[0]
	return s.Kind == StmtAssign && s.ArrayAccess != ""
}
