// Upstream: StatementReturn.cpp (make_random, Output, visit_facts).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStatementReturn mirrors StatementReturn::visit_facts.
// StatementReturn.cpp:76–97 — no_return_dead_ptr filter; var.visit_facts; update return.
func VisitFactsStatementReturn(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtReturn {
		return false
	}
	// StatementReturn always has ExpressionVariable; nil expr is incomplete IR
	if st.Expr == nil {
		return false
	}
	// no_return_dead_ptr: reject returning local-pointing ptrs
	if opts.NoReturnDeadPointer && st.Expr.Term == TermVariable && st.Expr.Var != nil {
		v := st.Expr.Var
		ind := st.Expr.IndirectLevel()
		facts := cg.pointToFacts()
		if IsPointingToLocals(v, cg.CurrentBlock(), ind, facts) {
			return false
		}
	}
	if !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// FactMgr::update_fact_for_return — StatementReturn.cpp:91–94
	if cg.FM != nil && cg.CurrentFunc != nil && cg.CurrentFunc.RV != nil {
		cg.FM.UpdateFactForReturnStmt(st, cg.CurrentFunc.RV, st.Expr)
	}
	// StatementReturn.cpp:93–94 — map_stm_effect[this] = effect_stm
	if cg.FM != nil && st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
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
	st := Stmt{Kind: StmtReturn}
	if r == nil || cg == nil || cg.CurrentFunc == nil {
		return st
	}
	// StatementReturn.cpp:55 — DEPTH_GUARD_BY_TYPE_RETURN(dtStatementReturn, nullptr)
	if DepthGuardByType(opts, DtStatementReturn) == BadDepth {
		return st
	}
	// StatementReturn.cpp:56–59 — assert(curr_func); assert(fm)
	// library: no FactMgr invent; still allow expr build (visit_facts later needs FM)
	// StatementReturn.cpp:56–62 — curr_func->return_type; no invent
	ret := cg.CurrentFunc.ReturnType
	if ret == nil {
		return st
	}
	// StatementReturn.cpp:61–62 — &curr_func->rv->qfer (assert rv present in C++)
	var qfer *CVQualifiers
	if cg.CurrentFunc.RV != nil {
		q := cg.CurrentFunc.RV.Qfer
		qfer = &q
	}
	// ExpressionVariable::make_random(cg, return_type, &rv->qfer, false, true) — as_return
	ev := makeExpressionVariableFlags(r, vs, cg, ret, qfer, false, true)
	// StatementReturn.cpp:66 ERROR_GUARD after make_random + cast setup
	if ev == nil || HasError() {
		return st
	}
	// typecast if needed (StatementReturn.cpp:64 — check_and_set_cast; lang_cpp only)
	ev.CheckAndSetCastOpts(ret, opts)
	// ccomp + bitfield return cast (StatementAssign.cpp similar path)
	if opts.CComp && ev.Var != nil && ev.Var.IsBitfield {
		ev.CastType = ret
	}
	st.Expr = ev
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}
	// StatementReturn.cpp make_random does not visit_facts — stm_visit / append_return does
	return st
}
