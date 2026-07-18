// Upstream: StatementIf.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIf mirrors StatementIf::make_random without FactMgr re-analysis.
// StatementIf.cpp:57–111 — test on get_int_type(), then/else Block::make_random.
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
		// last resort constant if const allowed
		if !noConst {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermConstant, cg.ExprDepth)
		} else {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	return &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB}
}
