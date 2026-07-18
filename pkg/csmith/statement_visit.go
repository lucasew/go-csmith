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
	case StmtFor:
		return VisitFactsStatementFor(st, cg, opts)
	case StmtArrayOp:
		return VisitFactsStatementArrayOp(st, cg, opts)
	case StmtReturn:
		return VisitFactsStatementReturn(st, cg, opts)
	case StmtBreak, StmtContinue:
		return VisitFactsStatementJump(st, cg, opts)
	case StmtGoto:
		return VisitFactsStatementGoto(st, cg, opts)
	case StmtLabel:
		if cg.FM != nil && st.StmID > 0 {
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
		return true
	case StmtInvoke:
		return VisitFactsStatementExpr(st, cg, opts)
	case StmtBlock:
		if st.Then != nil {
			return VisitFactsBlock(st.Then, cg, opts)
		}
		return true
	default:
		return true
	}
}

// VisitFactsStatementJump mirrors StatementBreak/Continue::visit_facts.
// StatementBreak.cpp:126–134 / StatementContinue.cpp:125–133 — test then effect_stm.
func VisitFactsStatementJump(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return true
	}
	if st.Expr != nil && !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	if cg.FM != nil && st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsStatementGoto mirrors StatementGoto::visit_facts.
// StatementGoto.cpp:364–402 — test; check_write skipped vars; subset re-analysis of dest.
func VisitFactsStatementGoto(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return true
	}
	if st.Expr != nil && !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// check write on skipped vars (re-init at dest)
	facts := cg.pointToFacts()
	for _, v := range st.InitSkippedVars {
		if v != nil && !cg.CheckWriteVar(v, facts) {
			return false
		}
	}
	fm := cg.FM
	if fm != nil && st.StmID > 0 {
		// StatementGoto.cpp:390–398 — force dest re-analysis when current outs
		// are proper subset of previous outs and neither visited this pass.
		destID := st.GotoDestStmID
		if destID > 0 {
			visitedThis := fm.MapVisited != nil && fm.MapVisited[st.StmID]
			visitedDest := fm.MapVisited != nil && fm.MapVisited[destID]
			prevOut := fm.MapFactsOut[st.StmID]
			cur := facts
			if !visitedThis && !visitedDest &&
				!SameFacts(cur, prevOut) &&
				SubsetFacts(cur, prevOut) {
				delete(fm.MapFactsIn, destID)
				delete(fm.MapFactsOut, destID)
			}
		}
		fm.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsStatementExpr mirrors StatementExpr::visit_facts.
// StatementExpr.cpp:104–110 — expr.visit_facts; store effect_stm.
func VisitFactsStatementExpr(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return true
	}
	if st.Expr != nil && !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	if cg.FM != nil && st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
	}
	return true
}

// VisitFactsBlock mirrors Block::visit_facts via find_fixed_point.
// Block.cpp:466–479.
func VisitFactsBlock(b *Block, cg *CGContext, opts Options) bool {
	if b == nil || cg == nil {
		return true
	}
	var inputs []*FactPointTo
	if cg.FM != nil {
		inputs = CloneFactSlice(cg.FM.GlobalFacts)
	}
	_, _, ok := FindFixedPointBlock(b, inputs, cg, opts, false)
	return ok
}

// VisitFactsStatementIf mirrors StatementIf::visit_facts.
// StatementIf.cpp:162–198 — condition; both branches from post-cond env;
// set_accumulated_effect_after_block(cond_eff, each arm); merge by must_return.
// When both arms must_return, outputs restore pre-condition inputs (inputs_copy).
func VisitFactsStatementIf(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil || st.Kind != StmtIfElse {
		return false
	}
	// StatementIf.cpp:164 — inputs_copy before condition
	var inputsCopy []*FactPointTo
	if cg.FM != nil {
		inputsCopy = CloneFactSlice(cg.FM.GlobalFacts)
	}
	// StatementIf.cpp:165–168 — evaluate condition first
	if st.Expr != nil && !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// StatementIf.cpp:169 — effect_stm after condition
	condEff := cg.EffectStm.Clone()
	// post-condition env shared as entry to both arms
	var postCond []*FactPointTo
	if cg.FM != nil {
		postCond = CloneFactSlice(cg.FM.GlobalFacts)
	}
	preAccum := EmptyEffect()
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
	}

	// StatementIf.cpp:170–173 — true branch from post-cond facts
	thenAccum := preAccum
	thenCG := *cg
	thenCG.EffectAccum = &thenAccum
	if st.Then != nil {
		if !VisitFactsBlock(st.Then, &thenCG, opts) {
			return false
		}
	}
	var thenFacts []*FactPointTo
	if cg.FM != nil {
		thenFacts = CloneFactSlice(cg.FM.GlobalFacts)
		// StatementIf.cpp:174 — false starts from same post-cond inputs (not after true)
		cg.FM.GlobalFacts = CloneFactSlice(postCond)
	}

	// StatementIf.cpp:174–177 — false branch
	elseAccum := preAccum
	elseCG := *cg
	elseCG.EffectAccum = &elseAccum
	if st.Else != nil {
		if !VisitFactsBlock(st.Else, &elseCG, opts) {
			return false
		}
	}
	var elseFacts []*FactPointTo
	if cg.FM != nil {
		elseFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}

	// StatementIf.cpp:178–180 — set_accumulated_effect_after_block(eff, &if_true/false)
	// eff is mutated across both calls: cond + then + else map_stm_effect[block]
	if cg.FM != nil {
		acc := condEff
		if st.Then != nil && st.Then.StmID > 0 {
			acc = acc.AddEffect(cg.FM.GetMapStmEffect(st.Then.StmID))
		} else {
			acc = acc.AddEffect(thenCG.EffectStm)
		}
		if st.Else != nil && st.Else.StmID > 0 {
			acc = acc.AddEffect(cg.FM.GetMapStmEffect(st.Else.StmID))
		} else {
			acc = acc.AddEffect(elseCG.EffectStm)
		}
		if st.StmID > 0 {
			cg.FM.SetMapStmEffect(st.StmID, acc)
		}
	}

	// StatementIf.cpp:185–196 — must_return pruning
	if cg.FM != nil {
		trueMust := st.Then != nil && st.Then.MustReturn()
		falseMust := st.Else != nil && st.Else.MustReturn()
		switch {
		case trueMust && falseMust:
			// pre-condition env (inputs_copy), not post-condition
			cg.FM.GlobalFacts = CloneFactSlice(inputsCopy)
		case trueMust:
			cg.FM.GlobalFacts = elseFacts
		case falseMust:
			cg.FM.GlobalFacts = thenFacts
		default:
			cg.FM.GlobalFacts = thenFacts
			MergeFacts(&cg.FM.GlobalFacts, elseFacts)
		}
	}
	// parent accum: both arms observed (generation-time separates; visit matches merge)
	if cg.EffectAccum != nil {
		*cg.EffectAccum = MergeEffects(thenAccum, elseAccum)
	}
	return true
}

// VisitFactsStatementFor mirrors StatementFor::visit_facts.
// StatementFor.cpp:427–472 — init; IV bound; body; map_facts_in[body] or
// post-init restore on must_return; merge post_dest break edges; accum effect.
func VisitFactsStatementFor(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return false
	}
	// StatementFor.cpp:430–432 — walk initializing statement
	if st.Loop != nil {
		if st.Loop.InitStmt != nil {
			if !VisitFactsStatementAssign(st.Loop.InitStmt, cg, opts) {
				return false
			}
		} else if st.Loop.IV != nil {
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
	}
	// StatementFor.cpp:433–434 — facts_copy / effect_stm after init
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
	}
	eff := cg.EffectStm.Clone()

	var iv *Variable
	if st.Loop != nil {
		iv = st.Loop.IV
	}
	if iv != nil {
		// StatementFor.cpp:437–443 — scalar IV; not outer loop IV (assert upstream)
		if cg.IVBounds != nil {
			if _, ok := cg.IVBounds[iv]; ok {
				// outer IV — upstream asserts; still track for body reject
			}
		}
		cg.AddIVBound(iv, 0)
		defer cg.RemoveIVBound(iv)
	}
	// StatementFor.cpp:445–449 — body under IN_LOOP (body uses shared accum)
	bodyCG := *cg
	bodyCG.Flags |= FlagInLoop
	if st.Then != nil {
		if !VisitFactsBlock(st.Then, &bodyCG, opts) {
			return false
		}
	}
	if cg.FM != nil {
		// StatementFor.cpp:452–458
		if st.Then != nil && st.Then.MustReturn() {
			// control reaches end of for with pre-loop (post-init) env
			cg.FM.GlobalFacts = CloneFactSlice(factsCopy)
		} else if st.Then != nil && st.Then.StmID > 0 {
			// map_facts_in[&body] — fixed-point entry, not merge(pre,post)
			if in, ok := cg.FM.MapFactsIn[st.Then.StmID]; ok {
				cg.FM.GlobalFacts = CloneFactSlice(in)
			}
		}
		// StatementFor.cpp:460–466 — find_edges_in(true, false) on this for stmt
		if st.StmID > 0 {
			for _, e := range cg.FM.FindEdgesIn(st.StmID, true, false) {
				if e == nil {
					continue
				}
				if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(&cg.FM.GlobalFacts, out)
				}
			}
		}
		// also breaks targeting body block (CreateCFGEdge to looping block)
		if st.Then != nil {
			for _, e := range cg.FM.FindEdgesInToBlock(st.Then, true, false) {
				if e == nil {
					continue
				}
				if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(&cg.FM.GlobalFacts, out)
				}
			}
		}
	}
	// StatementFor.cpp:468 — set_accumulated_effect_after_block(eff, &body, …)
	if st.Then != nil {
		bodyEff := EmptyEffect()
		if cg.FM != nil && st.Then.StmID > 0 {
			bodyEff = cg.FM.GetMapStmEffect(st.Then.StmID)
		}
		SetAccumulatedEffectAfterBlock(st, bodyEff, cg, eff)
	}
	return true
}

// VisitFactsStatementArrayOp mirrors StatementArrayOp::visit_facts.
// StatementArrayOp.cpp:268–318 — write each IV; body or init_value assign path.
func VisitFactsStatementArrayOp(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return false
	}
	// collect IVs from nested ArrayOp Loop chain
	var ivs []*Variable
	for cur := st; cur != nil && cur.Kind == StmtArrayOp; {
		if cur.Loop != nil && cur.Loop.IV != nil {
			ivs = append(ivs, cur.Loop.IV)
		}
		// nested: Then may hold next ArrayOp as first stmt
		if cur.Then == nil || len(cur.Then.Stmts) == 0 {
			break
		}
		next := &cur.Then.Stmts[0]
		if next.Kind != StmtArrayOp {
			break
		}
		cur = next
	}
	// StatementArrayOp.cpp:270–275 — check_write_var each ctrl var
	facts := cg.pointToFacts()
	for _, iv := range ivs {
		if !cg.CheckWriteVar(iv, facts) {
			return false
		}
		facts = cg.pointToFacts()
	}

	// find innermost body assign (array init) or nested block body
	inner := findArrayOpInnermost(st)
	if inner == nil {
		return true
	}

	// body path: nested fors around a Block of statements (array loop)
	// init path: Then is a block whose first stmt is assign with ArrayAccess
	if inner.Then != nil && isArrayInitBody(inner.Then) {
		// StatementArrayOp.cpp:299–316 — init_value + lhs visit + update_fact_for_assign
		asg := &inner.Then.Stmts[0]
		if asg.Expr != nil && !VisitFactsExpression(asg.Expr, cg, opts) {
			return false
		}
		lhs := &Lhs{Var: asg.LhsVar, Type: nil}
		if asg.LhsVar != nil {
			lhs.Type = asg.LhsVar.Type
		}
		if lhs.Var != nil && !cg.VisitFactsLhs(lhs, opts) {
			return false
		}
		if cg.FM != nil && asg.LhsVar != nil {
			cg.FM.UpdateFactForAssign(asg.LhsVar, 0, asg.Expr)
			if st.StmID > 0 {
				cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm.Clone())
			}
		}
		return true
	}

	// body path — StatementArrayOp.cpp:277–297 (same shape as StatementFor visit)
	preFacts := CloneFactSlice(cg.pointToFacts())
	preStm := cg.EffectStm.Clone()
	bodyCG := *cg
	bodyCG.Flags |= FlagInLoop
	// add all IVs as bounds for body analysis
	for _, iv := range ivs {
		bodyCG.AddIVBound(iv, 0)
		defer bodyCG.RemoveIVBound(iv)
	}
	if inner.Then != nil {
		if !VisitFactsBlock(inner.Then, &bodyCG, opts) {
			return false
		}
		if cg.FM != nil {
			// StatementArrayOp.cpp:285–291 — must_return → pre-body; else map_facts_in[body]
			if inner.Then.MustReturn() {
				cg.FM.GlobalFacts = preFacts
			} else if in, ok := cg.FM.MapFactsIn[inner.Then.StmID]; ok {
				cg.FM.GlobalFacts = CloneFactSlice(in)
			}
			// StatementArrayOp.cpp:292–297 — find_edges_in(true, false) on this stmt
			if st.StmID > 0 {
				for _, e := range cg.FM.FindEdgesIn(st.StmID, true, false) {
					if e == nil {
						continue
					}
					if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
						MergeJumpFacts(&cg.FM.GlobalFacts, out)
					}
				}
			}
			// breaks targeting body block (looping CreateCFGEdge dest)
			for _, e := range cg.FM.FindEdgesInToBlock(inner.Then, true, false) {
				if e == nil {
					continue
				}
				if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(&cg.FM.GlobalFacts, out)
				}
			}
			// StatementArrayOp.cpp:298–299 — set_accumulated_effect_after_block
			bodyEff := EmptyEffect()
			if inner.Then.StmID > 0 {
				bodyEff = cg.FM.GetMapStmEffect(inner.Then.StmID)
			}
			if bodyEff.IsEmpty() {
				bodyEff = bodyCG.EffectStm
			}
			SetAccumulatedEffectAfterBlock(st, bodyEff, cg, preStm)
		}
	}
	return true
}

func findArrayOpInnermost(st *Stmt) *Stmt {
	if st == nil {
		return nil
	}
	cur := st
	for cur.Then != nil && len(cur.Then.Stmts) == 1 && cur.Then.Stmts[0].Kind == StmtArrayOp {
		cur = &cur.Then.Stmts[0]
	}
	return cur
}

func isArrayInitBody(b *Block) bool {
	if b == nil || len(b.Stmts) != 1 {
		return false
	}
	s := &b.Stmts[0]
	return s.Kind == StmtAssign && s.ArrayAccess != ""
}
