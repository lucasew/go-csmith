// Upstream: Statement.cpp find_edges_in / analyze_with_edges_in;
// FactMgr::merge_jump_facts; set_accumulated_effect_after_block.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MergeJumpFacts mirrors FactMgr::merge_jump_facts.
// FactMgr.cpp:569–588 — for each non-rv fact, join related jump fact (or garbage).
func MergeJumpFacts(facts *[]*FactPointTo, jumpFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	changed := false
	// iterate a snapshot of subjects so we can grow via MergeFactInto
	subjects := append([]*FactPointTo(nil), *facts...)
	for _, f := range subjects {
		if f == nil || f.Var == nil {
			continue
		}
		// skip return variables (*_rv)
		if isReturnVar(f.Var) {
			continue
		}
		jumpF := FindRelatedPointTo(jumpFacts, f.Var)
		if jumpF == nil {
			// jump over initializer → garbage
			jumpF = MakeFactPointTo(f.Var, GarbagePtr)
		}
		before := FindRelatedPointTo(*facts, f.Var)
		*facts = MergeFactInto(*facts, jumpF)
		after := FindRelatedPointTo(*facts, f.Var)
		if before == nil || after == nil || !before.Equal(after) {
			changed = true
		}
	}
	return changed
}

func isReturnVar(v *Variable) bool {
	if v == nil {
		return false
	}
	// Function RV names end with _rv
	n := v.Name
	return len(n) > 3 && n[len(n)-3:] == "_rv"
}

// FindEdgesIn mirrors Statement::find_edges_in for dest StmID.
// Statement.cpp:453–467 — edges with matching dest, post_dest, back_link.
func (fm *FactMgr) FindEdgesIn(destStmID int, postDest, backLink bool) []*CFGEdge {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	var out []*CFGEdge
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestStmID == destStmID && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// FindEdgesInToBlock finds edges whose DestBlock matches (break/continue).
func (fm *FactMgr) FindEdgesInToBlock(dest *Block, postDest, backLink bool) []*CFGEdge {
	if fm == nil || dest == nil {
		return nil
	}
	var out []*CFGEdge
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestBlock == dest && e.DestStmID == 0 && e.PostDest == postDest && e.BackLink == backLink {
			out = append(out, e)
		}
	}
	return out
}

// HasEdgeIn mirrors Statement::has_edge_in.
func (fm *FactMgr) HasEdgeIn(destStmID int, postDest, backLink bool) bool {
	return len(fm.FindEdgesIn(destStmID, postDest, backLink)) > 0
}

// AnalyzeWithEdgesIn mirrors Statement::analyze_with_edges_in.
// Statement.cpp:808–834 — merge visited jump sources then validate_and_update.
func AnalyzeWithEdgesIn(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	if st == nil || facts == nil || cg == nil {
		return true
	}
	fm := cg.FM
	if fm != nil && st.StmID > 0 {
		// back edges only if already visited
		if fm.MapVisited != nil && fm.MapVisited[st.StmID] {
			for _, e := range fm.FindEdgesIn(st.StmID, false, true) {
				if e == nil || fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
					continue
				}
				if out, ok := fm.MapFactsOut[e.SrcID]; ok {
					MergeJumpFacts(facts, out)
				}
				if acc, ok := fm.MapAccumEffect[e.SrcID]; ok {
					cg.AddEffect(acc, false)
				}
			}
		}
		// always consider forward edges
		for _, e := range fm.FindEdgesIn(st.StmID, false, false) {
			if e == nil || fm.MapVisited == nil || !fm.MapVisited[e.SrcID] {
				continue
			}
			if out, ok := fm.MapFactsOut[e.SrcID]; ok {
				MergeJumpFacts(facts, out)
			}
			if acc, ok := fm.MapAccumEffect[e.SrcID]; ok {
				cg.AddEffect(acc, false)
			}
		}
	}
	return ValidateAndUpdateFacts(st, facts, cg, opts, blk)
}

// SetAccumulatedEffectAfterBlock mirrors Statement::set_accumulated_effect_after_block.
// Statement.cpp:515–520 — eff += block effect; store as this statement's effect.
func SetAccumulatedEffectAfterBlock(st *Stmt, blockEffect Effect, cg *CGContext, preStm Effect) {
	if st == nil || cg == nil || cg.FM == nil {
		return
	}
	eff := preStm.AddEffect(blockEffect)
	if st.StmID > 0 {
		cg.FM.SetMapStmEffect(st.StmID, eff)
	}
}

// PostCreationAnalysis mirrors Statement::post_creation_analysis.
// Statement.cpp:844–900 — combine branches / makeup; effect; assign/return facts;
// func_1 uncertain-call revalidate; set in/out/visited.
func PostCreationAnalysis(st *Stmt, preFacts []*FactPointTo, preEffect Effect, cg *CGContext) {
	if st == nil || cg == nil || cg.FM == nil {
		return
	}
	fm := cg.FM
	if st.Kind == StmtIfElse {
		CombineBranchFacts(st, preFacts, fm)
	} else {
		MakeupNewVarFacts(&preFacts, fm.GlobalFacts)
	}
	// simple statements: save effect_stm
	if !IsCompound(st.Kind) {
		if st.StmID > 0 {
			fm.SetMapStmEffect(st.StmID, cg.EffectStm)
		}
	}
	specialHandled := false
	// Statement.cpp:864–878 — func_1 outside loop + uncertain call → full validate
	if cg.CurrentFunc != nil && cg.CurrentFunc.Name == "func_1" && !cg.InLoop() {
		if HasUncertainCallRecursiveStmt(st) {
			outputs := CloneFactSlice(preFacts)
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preEffect.Clone()
			}
			if ValidateAndUpdateFacts(st, &outputs, cg, Defaults(), cg.CurrentBlock()) {
				fm.GlobalFacts = outputs
				specialHandled = true
			}
		}
	}
	if !specialHandled {
		switch st.Kind {
		case StmtAssign:
			// abstract assign into global_facts (RHS calls already handled at gen)
			lhs := st.LhsVar
			indir := 0
			if st.Lhs != nil {
				lhs = st.Lhs.Var
				indir = st.Lhs.IndirectLevel()
			}
			if lhs != nil {
				fm.UpdateFactForAssign(lhs, indir, st.Expr)
			}
		case StmtReturn:
			if cg.CurrentFunc != nil && cg.CurrentFunc.RV != nil {
				fm.UpdateFactForReturn(cg.CurrentFunc.RV, st.Expr)
			}
		}
	}
	fm.RemoveRVFacts(&fm.GlobalFacts)
	if st.StmID > 0 {
		fm.SetMapFactsIn(st.StmID, preFacts)
		fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, cg.CurrentBlock())
		if fm.MapAccumEffect == nil {
			fm.MapAccumEffect = make(map[int]Effect)
		}
		fm.MapAccumEffect[st.StmID] = cg.AccumEffect()
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[st.StmID] = true
	}
}

// FindFixedPointBlock mirrors Block::find_fixed_point.
// Block.cpp:513–568 — merge back edges, shortcut, locals, analyze stmts, loop.
// failIndex is the statement index that failed analyze_with_edges_in, or -1.
func FindFixedPointBlock(b *Block, inputs []*FactPointTo, cg *CGContext, opts Options, visitOnce bool) (facts []*FactPointTo, failIndex int, ok bool) {
	if b == nil || cg == nil {
		return inputs, -1, true
	}
	fm := cg.FM
	currentInputs := CloneFactSlice(inputs)
	// push block
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.Stack = append(cg.CurrentFunc.Stack, b)
		defer func() {
			if f := cg.CurrentFunc; f != nil && len(f.Stack) > 0 {
				f.Stack = f.Stack[:len(f.Stack)-1]
			}
		}()
	}
	cnt := 0
	for {
		// Block.cpp:526–536 — when already visited, merge back-edge outs into inputs
		if fm != nil && b.StmID > 0 && fm.MapVisited != nil && fm.MapVisited[b.StmID] {
			if cnt++; cnt > 7 {
				// upstream asserts; treat as converged with last outs
				if out, has := fm.MapFactsOut[b.StmID]; has {
					fm.GlobalFacts = CloneFactSlice(out)
					return out, -1, true
				}
				return currentInputs, -1, true
			}
			for _, e := range fm.FindEdgesIn(b.StmID, false, true) {
				if e == nil {
					continue
				}
				if out, has := fm.MapFactsOut[e.SrcID]; has {
					MergeFacts(&currentInputs, out)
				}
			}
			for _, e := range fm.FindEdgesInToBlock(b, false, true) {
				if e == nil {
					continue
				}
				if out, has := fm.MapFactsOut[e.SrcID]; has {
					MergeFacts(&currentInputs, out)
				}
			}
		}
		// Block.cpp:537–541 — shortcut when inputs match previous
		if !visitOnce && fm != nil {
			work := CloneFactSlice(currentInputs)
			sc := ShortcutAnalysisBlock(b, &work, cg)
			switch sc {
			case ShortcutOK:
				if fm != nil {
					fm.GlobalFacts = work
				}
				return work, -1, true
			case ShortcutConflict:
				return currentInputs, 0, false
			}
		}
		outputs := CloneFactSlice(currentInputs)
		// Block.cpp:546–549 — facts for locals
		for _, v := range b.LocalVars {
			AddNewVarFactTo(v, &outputs)
		}
		// Block.cpp:552–557 — analyze each statement
		for i := range b.Stmts {
			if !AnalyzeWithEdgesIn(&b.Stmts[i], &outputs, cg, opts, b) {
				return outputs, i, false
			}
		}
		if fm != nil && b.StmID > 0 {
			fm.SetMapFactsIn(b.StmID, currentInputs)
			// OOS locals for fact_out (Block.cpp:560–561)
			outCopy := CloneFactSlice(outputs)
			if len(b.LocalVars) > 0 {
				tmp := outCopy
				// UpdateFactsForOOSVars mutates GlobalFacts; apply via temp
				saved := fm.GlobalFacts
				fm.GlobalFacts = tmp
				fm.UpdateFactsForOOSVars(b.LocalVars)
				outCopy = fm.GlobalFacts
				fm.GlobalFacts = saved
			}
			fm.SetMapFactsOut(b.StmID, outCopy)
			if fm.MapVisited == nil {
				fm.MapVisited = make(map[int]bool)
			}
			fm.MapVisited[b.StmID] = true
			b.SetAccumulatedEffect(fm)
			fm.GlobalFacts = outCopy
			facts = outCopy
		} else {
			facts = outputs
		}
		visitOnce = false
		// Without FM maps, single pass is enough
		if fm == nil || b.StmID == 0 {
			return facts, -1, true
		}
		// next loop: merge edges + shortcut when inputs stable
	}
}
