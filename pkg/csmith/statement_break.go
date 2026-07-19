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
	// StatementBreak.cpp nullptr — empty Stmt (no invent Kind-only shell)
	if r == nil || cg == nil {
		return Stmt{}
	}
	// incomplete ambient fails closed sticky (before EffectStm clear; no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return Stmt{}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return Stmt{}
	}
	// find closest looping parent (StatementBreak.cpp:71–75)
	loop := ClosestLoopingBlock(cg.CurrentBlock())
	// StatementBreak.cpp:72 — assert(b) sticky; no soft invent break without looping block
	if loop == nil {
		SetError(ErrGeneric)
		return Stmt{}
	}
	// StatementBreak.cpp:76 — clear effect_stm before condition
	cg.EffectStm = EmptyEffect()
	// StatementBreak.cpp:77–79 — make_random(int, 0, true, true, eVariable); ERROR_GUARD
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if expr == nil || HasError() {
		// StatementBreak.cpp:79 — ERROR_GUARD(nullptr)
		return Stmt{}
	}
	st := Stmt{Kind: StmtBreak, Expr: expr, StmID: AllocStmID()}
	// StatementBreak.cpp:81 — b->break_stms.push_back only
	// CFG edges created in StatementFor::post_loop_analysis (for-stmt dest), not here
	loop.BreakStmIDs = append(loop.BreakStmIDs, st.StmID)
	return st
}
