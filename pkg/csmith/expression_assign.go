// Upstream: ExpressionAssign.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeExpressionAssign mirrors ExpressionAssign::make_random.
// ExpressionAssign.cpp:49–65 — StatementAssign::make_random; update_fact_for_assign; wrap.
// cg is *CGContext (C++ CGContext&) so assign RHS/LHS visit_facts and fact updates stick.
func MakeExpressionAssign(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
) *Expression {
	// ExpressionAssign.cpp always has RNG + CGContext; no invent assign expr without them
	if r == nil || cg == nil {
		return nil
	}
	// ExpressionAssign.cpp:56–57 / 61–62 — get_fact_mgr always live; no invent without FM
	if cg.FM == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent assign expr / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// ExpressionAssign.cpp:49+ — type from Expression::make_random (may be non-null);
	// StatementAssign::make_random SelectLType when type nullptr — pass through nil, no invent.
	// ExpressionAssign.cpp:52–55 — WRITE qfer when nil (random_qualifiers WRITE, no_volatile)
	if qfer == nil {
		q := RandomQualifiersDefaultProbs(typ, AccessWrite, *cg, true, opts, probs, r)
		qfer = &q
	}
	// ExpressionAssign.cpp:56 / 61 — StatementAssign::make_random(cg, type, qfer)
	// forces match_exact_qualifiers while selecting LHS
	st := MakeRandomAssignQfer(r, opts, probs, vs, tables, cg, typ, qfer)
	// StatementAssign nullptr / ERROR_GUARD → no soft invent empty ExpressionAssign
	if HasError() || !stmtOK(st) {
		return nil
	}
	// ExpressionAssign.cpp:57–58 / 61–62 — FactMgr::update_fact_for_assign(sa, global_facts)
	// uses get_rhs(); MakeRandomAssignQfer already updates; re-apply matches C++ double call
	if st.LhsVar != nil {
		indir := 0
		if st.Lhs != nil {
			indir = st.Lhs.IndirectLevel()
		}
		_ = cg.FM.UpdateFactForAssign(st.LhsVar, indir, st.GetAssignRhs())
		// incomplete assign must not invent ExpressionAssign shell with wiped facts
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
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
