// Upstream: ExpressionAssign.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeExpressionAssign mirrors ExpressionAssign::make_random.
// ExpressionAssign.cpp:49–65 — StatementAssign::make_random; update_fact_for_assign; wrap.
func MakeExpressionAssign(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
) *Expression {
	if typ == nil {
		typ = GetIntType()
	}
	// ExpressionAssign.cpp:52–55 — WRITE qfer when nil (random_qualifiers WRITE, no_volatile)
	if qfer == nil {
		q := RandomQualifiersDefaultProbs(typ, AccessWrite, cg, true, opts, probs, r)
		qfer = &q
	}
	_ = qfer // MakeRandomAssign derives LHS qfer from RHS; WRITE constraint via AccessWrite on Lhs
	st := MakeRandomAssign(r, opts, probs, vs, tables, cg, typ)
	// ExpressionAssign.cpp:57–58 / 61–62 — FactMgr::update_fact_for_assign(sa, global_facts)
	// (MakeRandomAssign already updates; re-apply ensures expr-assign path matches C++)
	if cg.FM != nil && st.LhsVar != nil {
		indir := 0
		if st.Lhs != nil {
			indir = st.Lhs.IndirectLevel()
		}
		cg.FM.UpdateFactForAssign(st.LhsVar, indir, st.Expr)
	}
	// ExpressionAssign value type is LHS type (ExpressionAssign.h:get_type)
	exprType := typ
	if st.Lhs != nil {
		if t := st.Lhs.GetType(); t != nil {
			exprType = t
		}
	} else if st.LhsVar != nil && st.LhsVar.Type != nil {
		exprType = st.LhsVar.Type
	}
	return &Expression{
		Term:     TermAssignment,
		Assign:   &st,
		Var:      st.LhsVar,
		ExprType: exprType,
	}
}
