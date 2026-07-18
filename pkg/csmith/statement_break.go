// Upstream: StatementBreak.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomBreak mirrors StatementBreak::make_random.
// StatementBreak.cpp:59–82 — closest looping block; test expr; break_stms push.
func MakeRandomBreak(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
) Stmt {
	st := Stmt{Kind: StmtBreak}
	if r == nil || cg == nil {
		return st
	}
	// find closest looping parent (StatementBreak.cpp:71–75)
	loop := ClosestLoopingBlock(cg.CurrentBlock())
	// StatementBreak.cpp:76 — clear effect_stm before condition
	cg.EffectStm = EmptyEffect()
	// StatementBreak.cpp:77–79 — make_random(int, 0, true, true, eVariable); ERROR_GUARD
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if expr == nil || HasError() {
		// StatementBreak.cpp:79 — ERROR_GUARD(nullptr)
		return Stmt{Kind: StmtBreak}
	}
	st.Expr = expr
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}
	// Block::break_stms.push_back (StatementBreak.cpp:81)
	if loop != nil {
		loop.BreakStmIDs = append(loop.BreakStmIDs, st.StmID)
		// break exits to after loop — post_dest true, back_link false (common shape)
		if cg.FM != nil {
			cg.FM.CreateCFGEdge(st.StmID, loop, true, false)
		}
	}
	return st
}
