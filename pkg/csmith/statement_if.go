// Upstream: StatementIf.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIf mirrors StatementIf::make_random without FactMgr re-analysis.
// StatementIf.cpp:57–111 — test on get_int_type(), then/else Block::make_random
// with shared pre-branch env (else does not see then-only accum).
func MakeRandomIf(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
) *Stmt {
	// Expression::make_random(..., get_int_type(), no_const = !const_as_condition)
	noConst := !opts.ConstAsCondition
	test := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, false, noConst, MaxTermTypes, cg.ExprDepth)
	if test == nil {
		test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, noConst, TermVariable, cg.ExprDepth)
	}
	if test == nil {
		if !noConst {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermConstant, cg.ExprDepth)
		} else {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}
	// Snapshot pre-branch effect; each arm runs from the same pre-state (StatementIf.cpp:96–99).
	pre := EmptyEffect()
	if cg.EffectAccum != nil {
		pre = *cg.EffectAccum
	}
	thenEff := pre
	thenCG := cg
	thenCG.EffectAccum = &thenEff
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, thenCG, false)

	elseEff := pre
	elseCG := cg
	elseCG.EffectAccum = &elseEff
	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, elseCG, false)

	// Merge branch effects into parent accum
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(thenEff, elseEff)
	}
	return &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB}
}
