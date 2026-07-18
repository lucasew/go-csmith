// Upstream: ExpressionAssign.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeExpressionAssign mirrors ExpressionAssign::make_random.
// ExpressionAssign.cpp:49–65 — StatementAssign::make_random then wrap.
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
	// WRITE qfer if nil — random_qualifiers WRITE no_volatile true
	if qfer == nil {
		q := RandomQualifiersDefaultProbs(typ, AccessWrite, cg, true, opts, probs, r)
		qfer = &q
	}
	st := MakeRandomAssign(r, opts, probs, vs, tables, cg, typ)
	// Force non-const lhs already; attach qfer is implicit
	_ = qfer
	return &Expression{
		Term:     TermAssignment,
		Assign:   &st,
		// value of assignment expression is the LHS name (C semantics simplified)
		Var: st.LhsVar,
	}
}
