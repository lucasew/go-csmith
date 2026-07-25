// Upstream: StatementReturn.cpp (make_random, Output, visit_facts).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStatementReturn mirrors StatementReturn::visit_facts.
// StatementReturn.cpp:76–97 — no_return_dead_ptr filter; var.visit_facts; update return.
// Hard IR incomplete sticky (nil expr/FM/rv, StmID 0, incomplete facts/effect);
// policy rejects (pointing-to-locals) stay non-sticky false.
func VisitFactsStatementReturn(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtReturn {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementReturn always has ExpressionVariable; nil expr sticky hard IR
	if st.Expr == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// no_return_dead_ptr: reject returning local-pointing ptrs
	if opts.NoReturnDeadPointer && st.Expr.Term == TermVariable && st.Expr.Var != nil {
		// StatementReturn.cpp:83–84 — const Block *b = cg_context.curr_blk; assert(b)
		// Prefer CurrBlk (set in stm_visit_facts) over stack-top CurrentBlock.
		b := cg.AnalysisBlock()
		if b == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		v := st.Expr.Var
		// incomplete type IR sticky (no invent level-0 skip / soft re-pick)
		ind, iok := st.Expr.IndirectLevelCompleteSess(cgSess(cg))
		if !iok {
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		facts := cg.pointToFacts()
		if IsPointingToLocalsSess(cgSess(cg), v, b, ind, facts) {
			// residual ERROR sticky — no invent policy soft-reject past residual hole
			if sessHasError(cgSess(cg)) {
				return false
			}
			// policy reject — non-sticky
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past residual false path
		if sessHasError(cgSess(cg)) {
			return false
		}
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// residual ERROR sticky — no invent visit success past VisitFactsExpression residual
	if sessHasError(cgSess(cg)) {
		return false
	}
	// FactMgr::update_fact_for_return — StatementReturn.cpp:91–94
	// get_fact_mgr + curr_func + rv always live; sticky without them
	if cg.FM == nil || cg.CurrentFunc == nil || cg.CurrentFunc.RV == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// Statement::stm_id always live; StmID 0 sticky
	if StmIDUnset(st.StmID) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	_ = cg.FM.UpdateFactForReturnStmt(st, cg.CurrentFunc.RV, st.Expr)
	// residual ERROR sticky — no invent visit success past UpdateFact/set_fact_out residual
	// (soft invent was FactsComplete GlobalFacts true while residual ERROR soft-continued)
	if sessHasError(cgSess(cg)) || !FactsComplete(cg.FM.GlobalFacts) {
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// Incomplete EffectStm sticky (no invent visit true with incomplete map)
	if !EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// StatementReturn.cpp:93–94 — map_stm_effect[this] = effect_stm
	cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	return true
}

// MakeRandomReturn mirrors StatementReturn::make_random.
// StatementReturn.cpp:54–72 — ExpressionVariable only (as_return); no fact update here.
// ERROR_GUARD(nullptr) when ExpressionVariable::make_random fails — no const soft-fallback.
func MakeRandomReturn(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	cg *CGContext,
) Stmt {
	// StatementReturn.cpp nullptr sticky — empty Stmt (no invent Kind-only return shell)
	if r == nil || cg == nil || cg.CurrentFunc == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient fails closed sticky (no invent return / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// StatementReturn.cpp:55 — DEPTH_GUARD_BY_TYPE_RETURN(dtStatementReturn, nullptr)
	if DepthGuardByTypeSess(cgSess(cg), opts, DtStatementReturn) == BadDepth {
		return Stmt{}
	}
	// StatementReturn.cpp:56–59 — assert(curr_func); assert(fm)
	// fail closed without FactMgr invent (C++ get_fact_mgr always live)
	// non-sticky: soft re-pick factory (sticky poisons MakeRandomFor / generation)
	if cg.FM == nil {
		return Stmt{}
	}
	// incomplete GlobalFacts fail closed sticky (no invent cleaned return expr under holes)
	if !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// StatementReturn.cpp:56–62 — curr_func->return_type; non-sticky soft re-pick
	ret := cg.CurrentFunc.ReturnType
	if ret == nil {
		return Stmt{}
	}
	// StatementReturn.cpp:61–62 — &curr_func->rv->qfer; non-sticky soft re-pick
	if cg.CurrentFunc.RV == nil {
		return Stmt{}
	}
	q := cg.CurrentFunc.RV.Qfer
	qfer := &q
	// ExpressionVariable::make_random(cg, return_type, &rv->qfer, false, true) — as_return
	ev := makeExpressionVariableFlags(r, vs, cg, ret, qfer, false, true)
	// StatementReturn.cpp:66 ERROR_GUARD after make_random + cast setup
	if ev == nil || sessHasError(cgSess(cg)) {
		return Stmt{}
	}
	// typecast if needed (StatementReturn.cpp:64 — check_and_set_cast; lang_cpp only)
	ev.CheckAndSetCastOpts(ret, opts)
	// residual ERROR sticky — no invent Return stmt past CheckAndSetCast residual hole
	if sessHasError(cgSess(cg)) {
		return Stmt{}
	}
	// ccomp + bitfield return cast (StatementAssign.cpp similar path)
	if opts.CComp && ev.Var != nil && ev.Var.IsBitfield {
		ev.CastType = ret
	}
	// StatementReturn.cpp make_random does not visit_facts — stm_visit / append_return does
	return Stmt{Kind: StmtReturn, Expr: ev, StmID: AllocStmIDSess(cgSess(cg))}
}
