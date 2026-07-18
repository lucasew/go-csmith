// Upstream: StatementReturn.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

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
	ev := makeExpressionVariableFlags(r, vs, cg, ret, qfer, false, true)
	if ev == nil {
		// last resort: constant of return type (not upstream, avoids empty return)
		ev = &Expression{Term: TermConstant, Con: MakeRandom(ret, opts, r)}
	}
	// typecast if needed (StatementReturn.cpp:60 check_and_set_cast)
	ev.CheckAndSetCast(ret)
	// ccomp + bitfield return cast (StatementAssign.cpp similar path)
	if opts.CComp && ev.Var != nil && ev.Var.IsBitfield {
		ev.CastType = ret
	}
	st.Expr = ev
	// FactMgr::update_fact_for_return — assign into rv (Fact.cpp:76–82)
	if cg.FM != nil && cg.CurrentFunc.RV != nil {
		cg.FM.UpdateFactForReturn(cg.CurrentFunc.RV, ev)
	}
	return st
}
