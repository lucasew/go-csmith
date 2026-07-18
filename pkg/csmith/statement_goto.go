// Upstream: StatementGoto.cpp (make_random) — simplified CFG-free variant.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomGoto mirrors StatementGoto::make_random without FactMgr/CFG edges.
// Emits if (cond) goto lbl; with labels collected on Function for end-of-body emit.
// Upstream uses 40% back-edge preference; we always place labels at function end
// (valid forward targets) until full block CFG exists.
func MakeRandomGoto(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
) Stmt {
	_ = opts
	if r == nil || cg.CurrentFunc == nil {
		return Stmt{Kind: StmtGoto}
	}
	// flipcoin(40) would select back-edge; record preference but same emit path for now
	_ = r.RndFlipcoin(40)

	// condition: prefer existing int variable (ExpressionVariable)
	cond := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
	}
	label := "lbl_1"
	if vs != nil {
		label = vs.Sym.Next("lbl_")
	}
	cg.CurrentFunc.Labels = append(cg.CurrentFunc.Labels, label)
	_ = probs
	return Stmt{Kind: StmtGoto, Expr: cond, Label: label}
}
