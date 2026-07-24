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
	// get_last_stm() empty → return nullptr (empty Stmt, not Kind shell)
	if blk != nil && blk.GetLastStm() == nil {
		return Stmt{}
	}
	// StatementContinue always has RNG + CGContext; sticky no invent continue shell without them
	if r == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient fails closed sticky (before EffectStm clear; no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	loop := ClosestLoopingBlock(cg.CurrentBlock())
	// StatementContinue.cpp:71 — assert(b) sticky; no soft invent continue without looping block
	if loop == nil {
		sessNoteError(nil, ErrGeneric)
		return Stmt{}
	}
	// StatementContinue.cpp:72 — clear effect_stm before condition
	cg.EffectStm = EmptyEffect()
	// StatementContinue.cpp:73–75 — make_random(int, 0, true, true, eVariable); ERROR_GUARD
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	// residual ERROR sticky — no invent soft-return continue past condition make residual
	if expr == nil || sessHasError(nil) {
		return Stmt{}
	}
	st := Stmt{Kind: StmtContinue, Expr: expr, StmID: AllocStmID()}
	// FactMgr::create_cfg_edge(sc, b, false, true) — StatementContinue.cpp:83
	if cg.FM != nil {
		cg.FM.CreateCFGEdge(st.StmID, loop, false, true)
		// residual ERROR sticky — no invent soft-return continue past CreateCFGEdge residual
		if sessHasError(nil) {
			return Stmt{}
		}
	}
	return st
}
