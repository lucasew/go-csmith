// Upstream: ExpressionComma.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeExpressionComma mirrors ExpressionComma::make_random.
// ExpressionComma.cpp:55–66 —
//
//	lhs = make_random(ctx, nullptr, nullptr, false, true)  // no const
//	rhs = make_random(ctx, type, qfer, false, false)
//
// cg is *CGContext (C++ CGContext&) so subexpr visit_facts persist.
func MakeExpressionComma(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
) *Expression {
	// ExpressionComma always has RNG + CGContext; sticky no invent comma shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent comma / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// ExpressionComma.cpp:58–61 — same CGContext&; make_random bumps expr_depth for siblings
	// ExpressionComma.cpp:58–59 — lhs type nullptr, no_func=false, no_const=true
	lhs := MakeRandomExpression(r, opts, tables, vs, cg, nil, nil, false, true, MaxTermTypes, cg.ExprDepth)
	// ExpressionComma.cpp:60–61 — rhs type/qfer, no_func=false, no_const=false
	rhs := MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, false, false, MaxTermTypes, cg.ExprDepth)
	// no soft TermVariable/TermConstant retries (C++ uses results directly)
	if lhs == nil || rhs == nil || HasError() {
		return nil
	}
	// ExpressionComma.cpp:62–64 — cast_if_needed when lang_cpp (optional for C null ptrs)
	if opts.LangCPP {
		castIfNeeded(rhs)
		// residual ERROR sticky — no invent complete comma past cast/GetType residual hole
		if HasError() {
			return nil
		}
	}
	return &Expression{
		Term:     TermCommaExpr,
		CommaLHS: lhs,
		CommaRHS: rhs,
		ExprType: typ,
	}
}

// castIfNeeded mirrors ExpressionComma.cpp cast_if_needed.
// ExpressionComma.cpp:48–53 — nullptr constant of pointer type → cast_type.
// Expression always live when cast path runs; sticky (no invent soft-skip cast past hole).
// Non-constant / empty Con is complete no-op (nothing to cast).
func castIfNeeded(exp *Expression) {
	if exp == nil {
		SetError(ErrGeneric)
		return
	}
	if exp.Term != TermConstant || exp.Con == nil {
		return
	}
	ty := exp.GetType()
	// residual ERROR sticky — no invent soft-skip cast past GetType residual hole
	if HasError() {
		return
	}
	if ty != nil && ty.IsPointerLike() && exp.EqualsInt(0) {
		// residual ERROR sticky — no invent cast-true past EqualsInt residual hole
		if HasError() {
			return
		}
		exp.CastType = ty
		return
	}
	// residual ERROR sticky — no invent soft-continue cast no-op past EqualsInt residual false
	if HasError() {
		return
	}
}
