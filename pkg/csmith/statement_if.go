// Upstream: StatementIf.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIf mirrors StatementIf::make_random.
// StatementIf.cpp:57–111 — shared pre-branch env; visit_facts merge (cpp:162–202).
// cg is *CGContext (C++ CGContext&) so effect_stm clear and branch merges stick.
func MakeRandomIf(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) *Stmt {
	// StatementIf.cpp always has RNG + CGContext sticky; no invent if shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (before EffectStm clear; no invent soft re-pick)
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
	// StatementIf.cpp:58 — DEPTH_GUARD_BY_TYPE_RETURN(dtStatementIf, nullptr)
	if DepthGuardByType(opts, DtStatementIf) == BadDepth {
		return nil
	}
	// StatementIf.cpp:62–69 — func_1 hacking snapshot before condition
	var func1PreFacts []*FactPointTo
	var func1PreEffect Effect
	func1Hack := cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop()
	if func1Hack && cg.FM != nil {
		// incomplete GlobalFacts fail closed (no invent cleaned pre-facts snapshot)
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
		func1PreFacts = CloneFactSlice(cg.FM.GlobalFacts)
		if cg.EffectAccum != nil {
			if !EffectComplete(*cg.EffectAccum) {
				SetError(ErrGeneric)
				return nil
			}
			func1PreEffect = cg.EffectAccum.Clone()
		}
	}
	// StatementIf.cpp:69 — clear per-statement effect before condition
	cg.EffectStm = EmptyEffect()
	// StatementIf.cpp:70–72 — Expression::make_random(int, nullptr, false, !const_as_condition)
	// no soft TermVariable/TermConstant retries (ERROR_GUARD on null)
	noConst := !opts.ConstAsCondition
	test := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, false, noConst, MaxTermTypes, cg.ExprDepth)
	// StatementIf.cpp:72 — ERROR_GUARD(nullptr)
	if test == nil || HasError() {
		return nil
	}
	// StatementIf.cpp:74–91 — re-analyze uncertain calls in func_1
	if func1Hack && cg.FM != nil && HasUncertainCallRecursiveExpr(test) {
		// makeup_new_var_facts(pre_facts, global); reset accum; visit(pre_facts)
		// incomplete current GlobalFacts fail closed sticky (makeup would nil snapshot)
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
		if !MakeupNewVarFacts(&func1PreFacts, cg.FM.GlobalFacts) {
			// incomplete makeup / snapshot — fail closed sticky
			SetError(ErrGeneric)
			return nil
		}
		if cg.EffectAccum != nil {
			*cg.EffectAccum = func1PreEffect.Clone()
		}
		cg.FM.GlobalFacts = CloneFactSlice(func1PreFacts)
		if !VisitFactsExpression(test, cg, opts) {
			// StatementIf.cpp:84–88 — assert(ok) sticky; no invent soft re-pick past visit fail
			if cg.EffectAccum != nil {
				*cg.EffectAccum = func1PreEffect.Clone()
			}
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		// StatementIf.cpp:89 — global_facts = pre_facts (already in FM via visit)
	}
	// Snapshot pre-branch effect; each arm runs from the same pre-state (StatementIf.cpp:96–99).
	// Condition effects come from Expression::make_random visit_facts (not a second visit).
	pre := EmptyEffect()
	if cg.EffectAccum != nil {
		pre = *cg.EffectAccum
	}
	// StatementIf.cpp:92 — effect_stm after condition (for set_accumulated_effect_after_block)
	condEff := cg.EffectStm.Clone()

	thenEff := pre
	thenCG := *cg
	thenCG.EffectAccum = &thenEff
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &thenCG, false)
	// StatementIf.cpp:94 ERROR_GUARD_AND_DEL1 after if_true
	// live if_true Block* required (no invent if with nil Then shell)
	if HasError() || thenB == nil {
		return nil
	}

	// StatementIf.cpp:97–98 — else starts from map_facts_in[if_true]
	// C++ map[] always assigns (missing → empty); no invent pre-branch GlobalFacts fallback
	// Incomplete then-in / StmID fails closed sticky (no invent else gen past holes)
	if cg.FM != nil {
		if thenB.StmID <= 0 {
			cg.FM.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return nil
		}
		in := cg.FM.GetMapFactsIn(thenB.StmID)
		if !FactsComplete(in) {
			cg.FM.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return nil
		}
		cg.FM.GlobalFacts = CloneFactSlice(in)
	}

	elseEff := pre
	elseCG := *cg
	elseCG.EffectAccum = &elseEff
	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &elseCG, false)
	// StatementIf.cpp:99 ERROR_GUARD_AND_DEL2 after if_false
	// live if_false Block* required
	if HasError() || elseB == nil {
		return nil
	}
	// StatementIf.cpp:101–107 — construct StatementIf; do not merge branch facts here
	// (combine_branch_facts runs in post_creation_analysis / visit_facts)

	st := &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB, StmID: AllocStmID()}
	// StatementIf.cpp:105–106 — set_accumulated_effect_after_block(eff, each branch)
	// Incomplete cond/arm effects fail closed (no invent if stmt with Incomplete map_stm)
	if !EffectComplete(condEff) || !EffectComplete(thenCG.EffectStm) || !EffectComplete(elseCG.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	SetAccumulatedEffectAfterBlock(st, thenCG.EffectStm, cg, condEff)
	SetAccumulatedEffectAfterBlock(st, elseCG.EffectStm, cg, condEff)
	if cg.FM != nil && !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
		SetError(ErrGeneric)
		return nil
	}

	// branch accumulators still observed on parent (generation-time effect merge)
	// Incomplete arm accum fails closed (no invent pure MergeEffects past holes)
	if cg.EffectAccum != nil {
		merged := MergeEffects(thenEff, elseEff)
		if !EffectComplete(merged) {
			SetError(ErrGeneric)
			return nil
		}
		*cg.EffectAccum = merged
	}
	return st
}
