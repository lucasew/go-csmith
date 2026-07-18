// Upstream: StatementContinue.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomContinue mirrors StatementContinue::make_random.
// StatementContinue.cpp:59–84 —
//
//	reject if block has no prior statement (first stmt);
//	Expression::make_random(int, no_func, no_const, eVariable);
//	emit if (test) continue;
//
// CFG edge to loop head deferred (FactMgr).
func MakeRandomContinue(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	blk *Block,
) Stmt {
	// don't generate continue as the first statement in a block
	if blk != nil && len(blk.Stmts) == 0 {
		// upstream returns null → Statement::make_random retries; we fall back to assign
		return MakeRandomAssign(r, opts, NewProbabilities(opts), vs, tables, cg, nil)
	}
	st := Stmt{Kind: StmtContinue}
	if r == nil {
		return st
	}
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if expr == nil {
		expr = makeExpressionVariable(r, vs, cg, GetIntType(), nil)
	}
	st.Expr = expr
	return st
}
