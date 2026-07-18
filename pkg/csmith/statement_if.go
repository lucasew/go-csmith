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
	if cg == nil {
		return nil
	}
	// StatementIf.cpp:62–69 — func_1 hacking snapshot before condition
	var func1PreFacts []*FactPointTo
	var func1PreEffect Effect
	func1Hack := cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop()
	if func1Hack && cg.FM != nil {
		func1PreFacts = CloneFactSlice(cg.FM.GlobalFacts)
		if cg.EffectAccum != nil {
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
		MakeupNewVarFacts(&func1PreFacts, cg.FM.GlobalFacts)
		if cg.EffectAccum != nil {
			*cg.EffectAccum = func1PreEffect.Clone()
		}
		cg.FM.GlobalFacts = CloneFactSlice(func1PreFacts)
		if !VisitFactsExpression(test, cg, opts) {
			// StatementIf.cpp:84–88 — assert(ok); treat as make_random failure
			if cg.EffectAccum != nil {
				*cg.EffectAccum = func1PreEffect.Clone()
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
	// Snapshot pre-branch facts for else arm (map_facts_in[if_true] after true block)
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}

	thenEff := pre
	thenCG := *cg
	thenCG.EffectAccum = &thenEff
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &thenCG, false)
	var thenFacts []*FactPointTo
	if cg.FM != nil {
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// StatementIf.cpp:98 — else branch starts from map_facts_in[if_true]
		// (entry facts of true block == pre-branch after condition)
		if thenB != nil && thenB.StmID > 0 {
			if in, ok := cg.FM.MapFactsIn[thenB.StmID]; ok {
				cg.FM.GlobalFacts = CloneFactSlice(in)
			} else {
				cg.FM.GlobalFacts = CloneFactSlice(preFacts)
			}
		} else {
			cg.FM.GlobalFacts = CloneFactSlice(preFacts)
		}
	}

	elseEff := pre
	elseCG := *cg
	elseCG.EffectAccum = &elseEff
	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &elseCG, false)
	var elseFacts []*FactPointTo
	if cg.FM != nil {
		elseFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// StatementIf.cpp:185–200 — merge branch outputs into GlobalFacts
		trueMust := thenB != nil && thenB.MustReturn()
		falseMust := elseB != nil && elseB.MustReturn()
		switch {
		case trueMust && falseMust:
			cg.FM.GlobalFacts = CloneFactSlice(preFacts)
		case trueMust:
			cg.FM.GlobalFacts = elseFacts
		case falseMust:
			cg.FM.GlobalFacts = thenFacts
		default:
			cg.FM.GlobalFacts = thenFacts
			MergeFacts(&cg.FM.GlobalFacts, elseFacts)
		}
	}

	st := &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB, StmID: AllocStmID()}
	// StatementIf.cpp:105–106 — set_accumulated_effect_after_block(eff, branch)
	SetAccumulatedEffectAfterBlock(st, thenCG.EffectStm, cg, condEff)
	SetAccumulatedEffectAfterBlock(st, elseCG.EffectStm, cg, condEff)

	// Merge branch effects into parent accum
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(thenEff, elseEff)
	}
	return st
}
