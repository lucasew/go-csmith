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
	if cg.FM != nil && (!FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts)) {
		SetError(ErrGeneric)
		return nil
	}
	// StatementIf.cpp:58 — DEPTH_GUARD_BY_TYPE_RETURN(dtStatementIf, nullptr)
	if DepthGuardByType(opts, DtStatementIf) == BadDepth {
		return nil
	}
	// StatementIf.cpp:62–69 — func_1 hacking snapshot before condition
	// C++: FactVec pre_facts = fm->global_facts (shallow Fact* vector copy).
	// Soft invent was CloneFactSlice (deep), which freezes the pre-condition
	// lattice so mid-condition Join on shared Fact* (what C++ pre_facts observes)
	// is lost on StatementIf.cpp:80–89 restore + re-visit. Soft invent was also
	// PT-only snapshot so re-analyze left UnionFacts at post-condition last-writes.
	var func1PreFacts []*FactPointTo
	var func1PreUnion []*FactUnion
	var func1PreEffect Effect
	func1Hack := cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop()
	if func1Hack && cg.FM != nil {
		// incomplete GlobalFacts/UnionFacts fail closed (no invent cleaned pre-facts snapshot)
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			SetError(ErrGeneric)
			return nil
		}
		// StatementIf.cpp:69 — shallow Fact* vector (same as restoreFactsPT).
		func1PreFacts = append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
		// CloneUnionFactSlice is already a shallow FactUnion* copy (FactVec partition).
		func1PreUnion = CloneUnionFactSlice(cg.FM.UnionFacts)
		if HasError() || !UnionFactsComplete(func1PreUnion) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		if cg.EffectAccum != nil {
			if !EffectComplete(*cg.EffectAccum) {
				SetError(ErrGeneric)
				return nil
			}
			func1PreEffect = cg.EffectAccum.Clone()
			// residual ERROR sticky — no invent soft-if past Effect Clone residual
			if HasError() {
				return nil
			}
		}
	}
	// StatementIf.cpp:69 — clear per-statement effect before condition
	cg.EffectStm = EmptyEffect()
	// StatementIf.cpp:70–72 — Expression::make_random(int, nullptr, false, !const_as_condition)
	// no soft TermVariable/TermConstant retries (ERROR_GUARD on null)
	noConst := !opts.ConstAsCondition
	test := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, false, noConst, MaxTermTypes, cg.ExprDepth)
	// StatementIf.cpp:72 — ERROR_GUARD(nullptr)
	// residual ERROR sticky — no invent soft-continue if arms past condition make residual
	if test == nil || HasError() {
		return nil
	}
	// StatementIf.cpp:74–91 — re-analyze uncertain calls in func_1
	hasUnc := func1Hack && cg.FM != nil && HasUncertainCallRecursiveExpr(test)
	// residual ERROR sticky — no invent soft-continue if arms past HasUncertain residual
	if HasError() {
		return nil
	}
	if hasUnc {
		// StatementIf.cpp:79–89 — makeup_new_var_facts(pre_facts, global);
		// reset accum; visit_facts(pre_facts); global_facts = pre_facts.
		// incomplete current GlobalFacts/UnionFacts fail closed sticky
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			SetError(ErrGeneric)
			return nil
		}
		// FactMgr.cpp:489–492 restore_facts on full FactVec (makeup + assign both cats)
		// Install pre-condition FactVec; VisitFactsExpression then mutates FM in place
		// (C++ visit mutates pre_facts then assigns back — same end state).
		cg.FM.RestoreFactsPair(func1PreFacts, func1PreUnion)
		if HasError() {
			return nil
		}
		if cg.EffectAccum != nil {
			*cg.EffectAccum = func1PreEffect.Clone()
			// residual ERROR sticky — no invent soft-restore past Effect Clone residual
			if HasError() {
				return nil
			}
		}
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
		// residual ERROR sticky — no invent if arms past condition visit residual true path
		if HasError() {
			if cg.EffectAccum != nil {
				*cg.EffectAccum = func1PreEffect.Clone()
			}
			return nil
		}
		// StatementIf.cpp:89 — global_facts = pre_facts (already in FM via visit on restored env)
	}
	// StatementIf.cpp:92 — effect_stm after condition (for set_accumulated_effect_after_block)
	condEff := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-if arms past EffectStm Clone residual
	if HasError() {
		return nil
	}

	// StatementIf.cpp:93–99 — both arms use the same CGContext& (not a copy).
	// C++ Block::make_random(cg_context) for true then false: shared effect_accum,
	// effect_stm, expr_depth, blk_depth, iv_bounds, curr_blk, etc.
	// CloneSubcontext here was unfair: EffectAccum pointer was shared but EffectStm /
	// BlkDepth / ExprDepth forked; after then, parent still held pre-then EffectStm and
	// else started from a second clone of the stale parent (seed-2 e13830: SelectParentLocal
	// stack n=4 vs UP n=5 — nesting bookkeeping desync after if arms).
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		SetError(ErrGeneric)
		return nil
	}
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	// StatementIf.cpp:94 ERROR_GUARD_AND_DEL1 after if_true (deletes expr only;
	// if_true already ~Block via make_random ERROR path when nil).
	// live if_true Block* required sticky (no invent if with nil Then shell)
	if thenB == nil {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}
	if HasError() {
		// residual after successful then: drop live if_true (no StatementIf yet)
		tombstoneBlock(thenB)
		return nil
	}

	// StatementIf.cpp:97–98 — else starts from map_facts_in[if_true]
	//   fm->global_facts = fm->map_facts_in[if_true];  // full FactVec
	// Soft invent was SetGlobalFacts(PT-only): UnionFacts stayed at then-branch
	// last-writes so IsNonreadableField over-filtered choose_var in else / later
	// (seed-7 eligible pool half-size vs upstream).
	// Incomplete then-in / StmID fails closed sticky (no invent else gen past holes)
	// EffectAccum / EffectStm / BlkDepth continue on the same cg (not reset between arms).
	if cg.FM != nil {
		if StmIDUnset(thenB.StmID) {
			cg.FM.GlobalFacts = IncompleteFactSlice()
			cg.FM.UnionFacts = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			tombstoneBlock(thenB)
			return nil
		}
		cg.FM.AssignGlobalFactsFromMapIn(thenB.StmID)
		if HasError() {
			tombstoneBlock(thenB)
			return nil
		}
	}

	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	// StatementIf.cpp:99 ERROR_GUARD_AND_DEL2 — delete expr + if_true
	// (if_false already ~Block on make_random ERROR when nil).
	// live if_false Block* required sticky (no invent if with nil Else shell)
	if elseB == nil {
		if !HasError() {
			SetError(ErrGeneric)
		}
		tombstoneBlock(thenB)
		return nil
	}
	if HasError() {
		tombstoneBlock(thenB)
		tombstoneBlock(elseB)
		return nil
	}
	// StatementIf.cpp:101–107 — construct StatementIf; do not merge branch facts here
	// (combine_branch_facts runs in post_creation_analysis / visit_facts)

	st := &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB, StmID: AllocStmID()}
	// StatementIf.cpp:105–106 / Statement.cpp:515–520 —
	// set_accumulated_effect_after_block(eff, block): eff += map_stm_effect[block];
	// map_stm_effect[this] = eff. C++ mutates the same Effect& across both calls so
	// final map = cond + then_block + else_block (not cond+else overwriting then).
	// Do not use arm EffectStm (last statement only); use map_stm_effect of each Block.
	if cg.FM != nil {
		thenMap := cg.FM.GetMapStmEffect(thenB.StmID)
		elseMap := cg.FM.GetMapStmEffect(elseB.StmID)
		if !EffectComplete(condEff) || !EffectComplete(thenMap) || !EffectComplete(elseMap) {
			cg.FM.SetMapStmEffect(st.StmID, IncompleteEffect())
			SetError(ErrGeneric)
			return nil
		}
		// first call: cond + then
		SetAccumulatedEffectAfterBlock(st, thenMap, cg, condEff)
		if HasError() {
			return nil
		}
		// second call: (cond+then) + else — base is stored map from first call
		mid := cg.FM.GetMapStmEffect(st.StmID)
		if !EffectComplete(mid) {
			SetError(ErrGeneric)
			return nil
		}
		SetAccumulatedEffectAfterBlock(st, elseMap, cg, mid)
		if HasError() || !EffectComplete(cg.FM.GetMapStmEffect(st.StmID)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
	}
	// effect_accum already holds true+false generation reads (shared pointer)
	return st
}
