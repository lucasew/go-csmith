// Upstream: StatementExpr.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomExprStmt mirrors StatementExpr::make_random.
// StatementExpr.cpp:54–68 — FunctionInvocation::make_random(false, …, 0, 0)
// (prefer user call / create; not std unary/binary).
// FactMgr / effect rollback omitted; on failure returns empty invoke for retry.
func MakeRandomExprStmt(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
) Stmt {
	if r == nil {
		return Stmt{Kind: StmtInvoke}
	}
	// type nullptr → flexible match; we pass nil so MakeRandomInvocation uses GetIntType
	// after non-simple check — choose_func with nil ret accepts any return type.
	list := cg.Funcs
	// is_std_func=false (StatementExpr.cpp:60)
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, nil, nil, false)
	if fi == nil || fi.Failed {
		// Statement::make_random retries on null; fall back to no-op marker
		return Stmt{Kind: StmtInvoke}
	}
	return Stmt{
		Kind: StmtInvoke,
		Expr: &Expression{Term: TermFunction, Invoke: fi},
	}
}
