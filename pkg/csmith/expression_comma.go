// Upstream: ExpressionComma.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeExpressionComma mirrors ExpressionComma::make_random.
// ExpressionComma.cpp:55–66 —
//   lhs = make_random(ctx, nullptr, nullptr, false, true)  // no const
//   rhs = make_random(ctx, type, qfer, false, false)
func MakeExpressionComma(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
) *Expression {
	if r == nil {
		return nil
	}
	// LHS: ExpressionComma.cpp:58 — make_random(nullptr type) → choose_random_nonvoid
	lhsType := GetIntType()
	if cg.Types != nil {
		lhsType = cg.Types.ChooseRandomNonvoid(r, opts, probs)
	} else if probs != nil {
		st := ChooseRandomNonvoidSimple(r, probs)
		lhsType = GetSimpleType(st)
	}
	if lhsType == nil {
		lhsType = GetIntType()
	}
	d := cg.ExprDepth + 1
	// noFunc false, noConst true for lhs (ExpressionComma.cpp:58–59)
	lhs := MakeRandomExpression(r, opts, tables, vs, cg, lhsType, nil, false, true, MaxTermTypes, d)
	if lhs == nil {
		lhs = MakeRandomExpression(r, opts, tables, vs, cg, lhsType, nil, true, true, TermVariable, d)
	}
	if typ == nil {
		typ = GetIntType()
	}
	rhs := MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, false, false, MaxTermTypes, d)
	if rhs == nil {
		rhs = MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, true, false, TermConstant, d)
	}
	// lang_cpp cast_if_needed skipped (defaults lang_cpp false)
	return &Expression{
		Term:     TermCommaExpr,
		CommaLHS: lhs,
		CommaRHS: rhs,
	}
}
