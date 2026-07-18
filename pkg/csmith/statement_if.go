// Upstream: StatementIf.cpp (make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MakeRandomIf mirrors StatementIf::make_random.
// StatementIf.cpp:57–111 — shared pre-branch env; visit_facts merge (cpp:162–202).
func MakeRandomIf(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
) *Stmt {
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
	// Expression::make_random(..., get_int_type(), no_const = !const_as_condition)
	noConst := !opts.ConstAsCondition
	test := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, false, noConst, MaxTermTypes, cg.ExprDepth)
	if test == nil {
		test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, noConst, TermVariable, cg.ExprDepth)
	}
	if test == nil {
		if !noConst {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermConstant, cg.ExprDepth)
		} else {
			test = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}
	// StatementIf.cpp:74–91 — re-analyze uncertain calls in func_1
	if func1Hack && cg.FM != nil && test != nil && HasUncertainCallRecursiveExpr(test) {
		MakeupNewVarFacts(&func1PreFacts, cg.FM.GlobalFacts)
		if cg.EffectAccum != nil {
			*cg.EffectAccum = func1PreEffect.Clone()
		}
		preWork := CloneFactSlice(func1PreFacts)
		cg.FM.GlobalFacts = preWork
		if VisitFactsExpression(test, &cg, opts) {
			// ok — keep facts from re-visit
		} else {
			cg.FM.GlobalFacts = CloneFactSlice(func1PreFacts)
		}
	}
	// Snapshot pre-branch effect; each arm runs from the same pre-state (StatementIf.cpp:96–99).
	pre := EmptyEffect()
	if cg.EffectAccum != nil {
		pre = *cg.EffectAccum
	}
	// Snapshot pre-branch facts for else arm (StatementIf.cpp:96–99 map_facts_in)
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}

	// StatementIf.cpp:80 — visit_facts on condition before branches (when FM set)
	// (skipped when already re-analyzed under func_1 uncertain path)
	if cg.FM != nil && test != nil && !(func1Hack && HasUncertainCallRecursiveExpr(test)) {
		cgp := &cg
		if !VisitFactsExpression(test, cgp, opts) {
			// soft-fail: keep test, continue generation
		}
	}

	thenEff := pre
	thenCG := cg
	thenCG.EffectAccum = &thenEff
	thenB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, thenCG, false)
	var thenFacts []*FactPointTo
	if cg.FM != nil {
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// restore pre-branch facts for else
		cg.FM.GlobalFacts = CloneFactSlice(preFacts)
	}

	elseEff := pre
	elseCG := cg
	elseCG.EffectAccum = &elseEff
	elseB := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, elseCG, false)
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

	// Merge branch effects into parent accum
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(thenEff, elseEff)
	}
	return &Stmt{Kind: StmtIfElse, Expr: test, Then: thenB, Else: elseB}
}
