// Upstream: StatementExpr.cpp (make_random, Output, visit_facts).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomExprStmt mirrors StatementExpr::make_random.
// StatementExpr.cpp:54–68 — FunctionInvocation::make_random(false, …, 0, 0);
// on failed invoke restore pre effect_accum and FactMgr global_facts.
func MakeRandomExprStmt(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
) Stmt {
	// StatementExpr.cpp always has RNG + CGContext; sticky no invent Kind-only shell
	if r == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient fails closed sticky (no invent expr stmt / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	// StatementExpr.cpp:53 — DEPTH_GUARD_BY_TYPE_RETURN(dtStatementExpr, nullptr)
	if DepthGuardByTypeSess(cgSess(cg), opts, DtStatementExpr) == BadDepth {
		return Stmt{}
	}
	// StatementExpr.cpp:58–59 — snapshot for rollback
	// incomplete facts fail closed sticky (no invent expr stmt under hole shells)
	var preEffect Effect
	if cg.EffectAccum != nil {
		// pre-validated EffectComplete
		preEffect = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-expr stmt past Effect Clone residual
		if sessHasError(nil) {
			return Stmt{}
		}
	}
	// StatementExpr.cpp:58 — vector<const Fact *> facts_copy = fm->global_facts
	// Full FactVec: ePointTo + eUnionWrite. Soft invent was PT-only restore leaving
	// UnionFacts at post-failed-call last-writes → IsNonreadableField over-filters
	// choose_var (seed-7 ok pool 26 vs upstream 56).
	var factsCopy []*FactPointTo
	var unionCopy []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			sessNoteError(nil, ErrGeneric)
			return Stmt{}
		}
		// shallow Fact* vector snapshot (C++ facts_copy = global_facts)
		factsCopy = append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
		unionCopy = append([]*FactUnion(nil), cg.FM.UnionFacts...)
	}
	list := cg.Funcs
	// is_std_func=false (StatementExpr.cpp:60)
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, nil, nil, false)
	// StatementExpr.cpp:61 ERROR_GUARD(nullptr)
	if sessHasError(nil) {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		if cg.FM != nil {
			cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		}
		return Stmt{}
	}
	if fi == nil || fi.Failed {
		// StatementExpr.cpp:62–66 — reset_effect_accum + restore_facts; return 0
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		if cg.FM != nil {
			cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		}
		// residual ERROR sticky — no invent soft re-pick past Failed residual (restore may residual)
		if sessHasError(nil) {
			return Stmt{}
		}
		// Statement::make_random retries on null
		return Stmt{}
	}
	// residual ERROR sticky — no invent invoke stmt past MakeRandomInvocation residual success path
	if sessHasError(nil) {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		if cg.FM != nil {
			cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		}
		return Stmt{}
	}
	// Statement base ctor always assigns stm_id (Statement.cpp:364–367)
	return Stmt{
		Kind:  StmtInvoke,
		Expr:  &Expression{Term: TermFunction, Invoke: fi},
		StmID: AllocStmIDSess(cgSess(cg)),
	}
}
