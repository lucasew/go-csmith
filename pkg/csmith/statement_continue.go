// Upstream: StatementContinue.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomContinue mirrors StatementContinue::make_random.
// StatementContinue.cpp:59–84 — first-stmt reject; closest loop; cfg_edge back_link.
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
	loop := ClosestLoopingBlock(cg.CurrentBlock())
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if expr == nil {
		expr = makeExpressionVariable(r, vs, cg, GetIntType(), nil)
	}
	st.Expr = expr
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}
	// FactMgr::create_cfg_edge(sc, b, false, true) — StatementContinue.cpp:83
	if loop != nil && cg.FM != nil {
		cg.FM.CreateCFGEdge(st.StmID, loop, false, true)
	}
	return st
}
