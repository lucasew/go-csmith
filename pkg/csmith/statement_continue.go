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
	cg *CGContext,
	blk *Block,
) Stmt {
	// StatementContinue.cpp:63–66 — don't generate continue as first stmt (prev_stm==0)
	// get_last_stm() empty → return nullptr (stmtOK rejects Expr-less continue)
	if blk != nil && blk.GetLastStm() == nil {
		return Stmt{Kind: StmtContinue}
	}
	st := Stmt{Kind: StmtContinue}
	if r == nil || cg == nil {
		return st
	}
	loop := ClosestLoopingBlock(cg.CurrentBlock())
	// StatementContinue.cpp:72 — clear effect_stm before condition
	cg.EffectStm = EmptyEffect()
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
