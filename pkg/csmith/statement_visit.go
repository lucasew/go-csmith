// Upstream: StatementIf/StatementFor/Block visit_facts analysis paths.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VisitFactsStmt dispatches Statement::visit_facts by kind.
// Statement subclasses — assign/if/for/block/return/jump/expr.
// Incomplete IR fails — no soft invent true (C++ always visits live Statement*).
func VisitFactsStmt(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return false
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
		if st.Then == nil {
			return false
		}
		return VisitFactsBlock(st.Then, cg, opts)
	default:
		// unknown kind; no soft invent success
		return false
	}
}

// VisitFactsStatementJump mirrors StatementBreak/Continue::visit_facts.
// StatementBreak.cpp:126–134 / StatementContinue.cpp:125–133 — test then effect_stm.
func VisitFactsStatementJump(st *Stmt, cg *CGContext, opts Options) bool {
	if st == nil || cg == nil {
		return false
	}
	// C++ always has live test Expression*
	if st.Expr == nil || !VisitFactsExpression(st.Expr, cg, opts) {
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
		return false
	}
	// StatementGoto.cpp:366–368 — test.visit_facts always
	if st.Expr == nil || !VisitFactsExpression(st.Expr, cg, opts) {
		return false
	}
	// check write on skipped vars (re-init at dest)
	// StatementGoto.cpp — vars[i] always live; no invent skip nil holes
	facts := cg.pointToFacts()
	for _, v := range st.InitSkippedVars {
		if v == nil || !cg.CheckWriteVar(v, facts) {
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
		return false
	}
	// StatementExpr.cpp:106 — expr always live
	if st.Expr == nil || !VisitFactsExpression(st.Expr, cg, opts) {
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
		return false
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
	// StatementIf.cpp:165–168 — evaluate condition first (always live test)
	if st.Expr == nil || !VisitFactsExpression(st.Expr, cg, opts) {
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

	// StatementIf.cpp:170–177 — both arms always live Blocks
	if st.Then == nil || st.Else == nil {
		return false
	}
	// StatementIf.cpp:170–173 — true branch from post-cond facts
	thenAccum := preAccum
	thenCG := *cg
	thenCG.EffectAccum = &thenAccum
	if !VisitFactsBlock(st.Then, &thenCG, opts) {
		return false
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
	if !VisitFactsBlock(st.Else, &elseCG, opts) {
		return false
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
	// StatementFor.cpp:430+ — always has init StatementAssign, body Block, IV
	if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		return false
	}
	// StatementFor.cpp:430–432 — init StatementAssign always live; no soft invent
	// reconstruct from InitN when InitStmt missing
	if st.Loop.InitStmt == nil {
		return false
	}
	if !VisitFactsStatementAssign(st.Loop.InitStmt, cg, opts) {
		return false
	}
	// StatementFor.cpp:433–434 — facts_copy / effect_stm after init
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
	}
	eff := cg.EffectStm.Clone()

	iv := st.Loop.IV
	// StatementFor.cpp:440 — assert(iv->type->eType == eSimple)
	if iv.Type == nil || !iv.Type.IsSimple() {
		return false
	}
	// StatementFor.cpp:441 — assert(iv_bounds.find(iv) == end); no soft invent re-bind
	if cg.IVBounds != nil {
		if _, ok := cg.IVBounds[iv]; ok {
			return false
		}
	}
	// StatementFor.cpp:443 — arbitrary bound 0
	cg.AddIVBound(iv, 0)
	defer cg.RemoveIVBound(iv)
	// StatementFor.cpp:445–449 — body under IN_LOOP (body uses shared accum)
	bodyCG := *cg
	bodyCG.Flags |= FlagInLoop
	if !VisitFactsBlock(st.Then, &bodyCG, opts) {
		return false
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
		// StatementFor.cpp:460–466 / post_loop_analysis:361–367 —
		// find_edges_in(true, false) on this for stmt (break edges dest = for-stmt)
		// CFGEdge* always live; no invent skip nil holes
		if st.StmID > 0 {
			for _, e := range cg.FM.FindEdgesIn(st.StmID, true, false) {
				if e == nil {
					return false
				}
				if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(&cg.FM.GlobalFacts, out)
				}
			}
		}
	}
	// StatementFor.cpp:468 — set_accumulated_effect_after_block(eff, &body, …)
	bodyEff := EmptyEffect()
	if cg.FM != nil && st.Then.StmID > 0 {
		bodyEff = cg.FM.GetMapStmEffect(st.Then.StmID)
	}
	SetAccumulatedEffectAfterBlock(st, bodyEff, cg, eff)
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
	// StatementArrayOp.cpp:270–275 — ctrl_vars sized to dimension; empty is incomplete IR
	if len(ivs) == 0 {
		return false
	}
	// StatementArrayOp.cpp:270–275 — check_write_var each ctrl var
	// IV always live; no invent skip nil holes in ctrl chain
	facts := cg.pointToFacts()
	for _, iv := range ivs {
		if iv == nil || !cg.CheckWriteVar(iv, facts) {
			return false
		}
		facts = cg.pointToFacts()
	}

	// find innermost body assign (array init) or nested block body
	inner := findArrayOpInnermost(st)
	// StatementArrayOp.cpp:276–317 — body OR init_value path; neither is incomplete
	if inner == nil || inner.Then == nil {
		return false
	}

	// body path: nested fors around a Block of statements (array loop)
	// init path: Then is a block whose first stmt is assign with ArrayAccess
	if isArrayInitBody(inner.Then) {
		// StatementArrayOp.cpp:299–316 — init_value + lhs visit + update_fact_for_assign
		// StatementAssign always has live Lhs + Expression*
		asg := &inner.Then.Stmts[0]
		if asg.Expr == nil || !VisitFactsExpression(asg.Expr, cg, opts) {
			return false
		}
		if asg.LhsVar == nil {
			return false
		}
		lhs := &Lhs{Var: asg.LhsVar, Type: asg.LhsVar.Type}
		if !cg.VisitFactsLhs(lhs, opts) {
			return false
		}
		if cg.FM != nil {
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
		// CFGEdge* always live; no invent skip nil holes
		if st.StmID > 0 {
			for _, e := range cg.FM.FindEdgesIn(st.StmID, true, false) {
				if e == nil {
					return false
				}
				if out, ok := cg.FM.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(&cg.FM.GlobalFacts, out)
				}
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
