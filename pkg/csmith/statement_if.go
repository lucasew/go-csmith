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
