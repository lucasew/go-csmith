// Upstream: StatementBreak.cpp (make_random, Output).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomBreak mirrors StatementBreak::make_random.
// StatementBreak.cpp:59–82 — Expression::make_random(int, no_func, no_const, eVariable);
// emit as if (test) break;
// Requires IN_LOOP (caller StatementFilter). Closest looping block bookkeeping deferred.
func MakeRandomBreak(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
) Stmt {
	st := Stmt{Kind: StmtBreak}
	if r == nil {
		return st
	}
	// Expression::make_random(..., true, true, eVariable)
	expr := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if expr == nil {
		expr = makeExpressionVariable(r, vs, cg, GetIntType(), nil)
	}
	st.Expr = expr
	return st
}
