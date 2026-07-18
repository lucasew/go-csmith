// Upstream: StatementReturn.cpp (make_random, Output, visit_facts).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStatementReturn mirrors StatementReturn::visit_facts.
// StatementReturn.cpp:76–97 — no_return_dead_ptr filter; var.visit_facts; update return.
func VisitFactsStatementReturn(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtReturn {
		return false
	}
	if st.Expr == nil {
		return true
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
// StatementReturn.cpp:54–72 — ExpressionVariable only; visit_facts updates return facts.
func MakeRandomReturn(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	cg CGContext,
) Stmt {
	st := Stmt{Kind: StmtReturn}
	if r == nil || cg.CurrentFunc == nil {
		return st
	}
	ret := cg.CurrentFunc.ReturnType
	if ret == nil {
		ret = GetIntType()
	}
	// rv->qfer for return dummy when present
	var qfer *CVQualifiers
	if cg.CurrentFunc.RV != nil {
		q := cg.CurrentFunc.RV.Qfer
		qfer = &q
	}
	// ExpressionVariable::make_random(cg, return_type, &rv->qfer, false, true) — as_return
	ev := makeExpressionVariableFlags(r, vs, &cg, ret, qfer, false, true)
	if ev == nil {
		// last resort: constant of return type (not upstream, avoids empty return)
		ev = &Expression{Term: TermConstant, Con: MakeRandom(ret, opts, r)}
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
	// StatementReturn.cpp make_random does not update facts — visit_facts does
	// (StatementReturn.cpp:76–97). Optional eager visit when FM present for
	// generation-time analysis consistency with other stmt factories.
	if cg.FM != nil {
		cgp := &cg
		_ = VisitFactsStatementReturn(&st, cgp, opts)
	}
	return st
}
