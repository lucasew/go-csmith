// Upstream: StatementIf/StatementFor/Block visit_facts analysis paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStmt dispatches Statement::visit_facts by kind.
// Statement subclasses — assign/if/for/block/return/jump/expr.
func VisitFactsStmt(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return true
	}
	switch st.Kind {
	case StmtAssign:
		return VisitFactsStatementAssign(st, cg, opts)
	case StmtIfElse:
		return VisitFactsStatementIf(st, cg, opts)
	case StmtFor, StmtArrayOp:
		return VisitFactsStatementFor(st, cg, opts)
	case StmtReturn:
		return VisitFactsStatementReturn(st, cg, opts)
	case StmtBreak, StmtContinue, StmtGoto, StmtLabel:
		// control transfer: condition expr if present
		if st.Expr != nil {
			return VisitFactsExpression(st.Expr, cg, opts)
		}
		return true
	case StmtInvoke:
		if st.Expr != nil {
			return VisitFactsExpression(st.Expr, cg, opts)
		}
		return true
	case StmtBlock:
		if st.Then != nil {
			return VisitFactsBlock(st.Then, cg, opts)
		}
		return true
	default:
		return true
	}
}

// VisitFactsBlock mirrors Block::visit_facts simplified (no full fixed-point).
// Block.cpp:466–479 — sequential statement walk; updates facts in place via FM.
func VisitFactsBlock(b *Block, cg *CGContext, opts Options) bool {
	if b == nil || cg == nil {
		return true
	}
	// push block on stack if function present
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.Stack = append(cg.CurrentFunc.Stack, b)
		defer func() {
			if f := cg.CurrentFunc; f != nil && len(f.Stack) > 0 {
				f.Stack = f.Stack[:len(f.Stack)-1]
			}
		}()
	}
	for i := range b.Stmts {
		st := &b.Stmts[i]
		// record facts_in before visit
		if cg.FM != nil && st.StmID > 0 {
			cg.FM.SetMapFactsIn(st.StmID, cg.FM.GlobalFacts)
		}
		// clear effect_stm for each statement (Statement::pre_output path)
		cg.ClearEffectStm()
		if !VisitFactsStmt(st, cg, opts) {
			return false
		}
		if cg.FM != nil && st.StmID > 0 {
			cg.FM.SetMapFactsOutForStmt(st, cg.FM.GlobalFacts, b)
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
	}
	return true
}

// VisitFactsStatementIf mirrors StatementIf::visit_facts.
// StatementIf.cpp:162–198 — test then true/false branches; merge by must_return.
func VisitFactsStatementIf(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtIfElse {
		return false
	}
	// evaluate condition first
	if st.Expr != nil && !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}
	preEff := EmptyEffect()
	if cg.EffectAccum != nil {
		preEff = *cg.EffectAccum
	}

	// true branch
	thenEff := preEff
	thenCG := *cg
	thenCG.EffectAccum = &thenEff
	if st.Then != nil {
		if !VisitFactsBlock(st.Then, &thenCG, opts) {
			return false
		}
	}
	var thenFacts []*FactPointTo
	if cg.FM != nil {
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		cg.FM.GlobalFacts = CloneFactSlice(preFacts)
	}

	// false branch
	elseEff := preEff
	elseCG := *cg
	elseCG.EffectAccum = &elseEff
	if st.Else != nil {
		if !VisitFactsBlock(st.Else, &elseCG, opts) {
			return false
		}
	}
	var elseFacts []*FactPointTo
	if cg.FM != nil {
		elseFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}

	// merge facts by must_return
	if cg.FM != nil {
		trueMust := st.Then != nil && st.Then.MustReturn()
		falseMust := st.Else != nil && st.Else.MustReturn()
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
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(thenEff, elseEff)
	}
	return true
}

// VisitFactsStatementFor mirrors StatementFor::visit_facts core.
// StatementFor.cpp:427–472 — init, IV bound, body, restore pre if must_return.
func VisitFactsStatementFor(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return false
	}
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}
	// init as assignment of IV if present
	if st.Loop != nil && st.Loop.IV != nil {
		initLhs := &Lhs{Var: st.Loop.IV, Type: st.Loop.IV.Type}
		initSt := Stmt{
			Kind: StmtAssign, LhsVar: st.Loop.IV, Lhs: initLhs,
			Expr: &Expression{Term: TermConstant, Con: MakeInt(st.Loop.InitN)},
			AssignOp: AssignSimple,
		}
		if !VisitFactsStatementAssign(&initSt, cg, opts) {
			return false
		}
	}
	var iv *Variable
	if st.Loop != nil {
		iv = st.Loop.IV
	}
	if iv != nil {
		// StatementFor.cpp:441–443 — IV must not be written in body
		if cg.IVBounds != nil {
			if _, ok := cg.IVBounds[iv]; ok {
				// already outer IV — upstream asserts
			}
		}
		cg.AddIVBound(iv, 0)
		defer cg.RemoveIVBound(iv)
	}
	bodyCG := *cg
	bodyCG.Flags |= FlagInLoop
	if st.Then != nil {
		if !VisitFactsBlock(st.Then, &bodyCG, opts) {
			return false
		}
	}
	// body must_return → restore pre-loop facts
	if cg.FM != nil {
		if st.Then != nil && st.Then.MustReturn() {
			cg.FM.GlobalFacts = CloneFactSlice(preFacts)
		} else {
			// 0+ iterations: merge pre with post-body
			post := CloneFactSlice(cg.FM.GlobalFacts)
			cg.FM.GlobalFacts = CloneFactSlice(preFacts)
			MergeFacts(&cg.FM.GlobalFacts, post)
		}
	}
	return true
}
