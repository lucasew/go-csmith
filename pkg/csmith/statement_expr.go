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
	if r == nil || cg == nil {
		return Stmt{Kind: StmtInvoke}
	}
	// StatementExpr.cpp:58–59 — snapshot for rollback
	var preEffect Effect
	if cg.EffectAccum != nil {
		preEffect = cg.EffectAccum.Clone()
	}
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
	}
	list := cg.Funcs
	// is_std_func=false (StatementExpr.cpp:60)
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, nil, nil, false)
	if fi == nil || fi.Failed {
		// StatementExpr.cpp:62–66 — reset_effect_accum + restore_facts
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect
		}
		if cg.FM != nil {
			cg.FM.RestoreFacts(factsCopy)
		}
		// Statement::make_random retries on null; empty invoke for retry
		return Stmt{Kind: StmtInvoke}
	}
	return Stmt{
		Kind: StmtInvoke,
		Expr: &Expression{Term: TermFunction, Invoke: fi},
	}
}
