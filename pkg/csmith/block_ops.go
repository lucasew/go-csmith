// Upstream: Block.cpp remove_stmt, need_nested_loop, must_break_or_return details.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustBreakOrReturnFull mirrors Block::must_break_or_return.
// Block.cpp:342–357 — last must_return unless continue-like back edge from outside.
// Note: unlike must_return, does not require break_stms empty.
func (b *Block) MustBreakOrReturnFull(fm *FactMgr) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	last := b.GetLastStm()
	if last == nil || !last.MustReturn() {
		return false
	}
	// Block.cpp:345–353 — same back-edge escape check as must_return
	return !b.hasEscapeBackEdge(fm)
}

// NeedNestedLoop mirrors Block::need_nested_loop.
// Block.cpp:390–420 — looping block needs deeper IV for must-use array dimensions.
func (b *Block) NeedNestedLoop(cg CGContext, r *Rng) bool {
	if b == nil || !b.Looping || r == nil {
		return false
	}
	s := b.GetLastStm()
	if s != nil && s.MustJump() {
		return false
	}
	if cg.RW == nil {
		return false
	}
	ivDepth := 0
	if cg.IVBounds != nil {
		ivDepth = len(cg.IVBounds)
	}
	// Block.cpp:399–414 — must_read/write vars use get_dimension()
	// Variable* always live on RW lists; nil hole fails closed true
	// (no invent "no nested loop" by soft-skipping holes as absent must-use).
	check := func(v *Variable) bool {
		dimen := v.GetDimension()
		if dimen == 0 {
			return false
		}
		if dimen > ivDepth {
			return true
		}
		if dimen == ivDepth && r.RndFlipcoin(10) {
			return true
		}
		return false
	}
	for _, v := range cg.RW.MustReadVars {
		if v == nil {
			return true
		}
		if check(v) {
			return true
		}
	}
	for _, v := range cg.RW.MustWriteVars {
		if v == nil {
			return true
		}
		if check(v) {
			return true
		}
	}
	return false
}

// RemoveStmt mirrors Block::remove_stmt by StmID.
// Block.cpp:586–652 — scrub break_stms; drop CFG edges from control stmts inside s;
// drop edges into s and recursively remove goto sources; drop contained blocks
// from Function.Blocks; erase s from stms.
// Returns number of statements removed from this block (including cascaded gotos).
func (b *Block) RemoveStmt(stmID int, fm *FactMgr) int {
	if b == nil || stmID <= 0 {
		return 0
	}
	var removed *Stmt
	idx := -1
	for i := range b.Stmts {
		if b.Stmts[i].StmID == stmID {
			removed = &b.Stmts[i]
			idx = i
			break
		}
	}
	if idx < 0 {
		// search nested Then/Else blocks
		for i := range b.Stmts {
			if b.Stmts[i].Then != nil {
				if n := b.Stmts[i].Then.RemoveStmt(stmID, fm); n > 0 {
					return n
				}
			}
			if b.Stmts[i].Else != nil {
				if n := b.Stmts[i].Else.RemoveStmt(stmID, fm); n > 0 {
					return n
				}
			}
		}
		return 0
	}
	ids := map[int]bool{}
	collectStmIDs(removed, ids)
	// also collect nested block stm ids for Func.Blocks scrub
	if removed.Then != nil {
		collectBlockStmIDs(removed.Then, ids)
	}
	if removed.Else != nil {
		collectBlockStmIDs(removed.Else, ids)
	}

	// Statement.cpp find_typed_stmts: continue/break/goto inside s
	cfgIDs := map[int]bool{}
	collectTypedStmIDs(removed, []StatementType{StmtBreak, StmtContinue, StmtGoto}, cfgIDs)

	// Block.cpp:602–616 — scrub break_stms on enclosing loop
	loop := b
	for loop != nil && !loop.Looping {
		loop = loop.Parent
	}
	if loop != nil && len(cfgIDs) > 0 {
		nb := loop.BreakStmIDs[:0]
		for _, id := range loop.BreakStmIDs {
			if !cfgIDs[id] {
				nb = append(nb, id)
			}
		}
		loop.BreakStmIDs = nb
	}

	var gotoSrcIDs []int
	if fm != nil {
		// Block.cpp:617–629 — remove edges with control stmt inside s as src
		// CFGEdge* always live; nil hole fails closed (nil whole CFG — no invent
		// keep-hole partial scrub that soft-skips past incomplete edges)
		ne := make([]*CFGEdge, 0, len(fm.CFGEdges))
		for _, e := range fm.CFGEdges {
			if e == nil {
				fm.CFGEdges = nil
				gotoSrcIDs = nil
				ne = nil
				break
			}
			if cfgIDs[e.SrcID] {
				continue
			}
			ne = append(ne, e)
		}
		if ne != nil {
			fm.CFGEdges = ne
		}

		// Block.cpp:632–652 — remove edges with dest inside s; cascade-delete gotos
		if fm.CFGEdges != nil {
			ne = make([]*CFGEdge, 0, len(fm.CFGEdges))
			for _, e := range fm.CFGEdges {
				if e == nil {
					fm.CFGEdges = nil
					gotoSrcIDs = nil
					ne = nil
					break
				}
				destIn := e.DestStmID > 0 && ids[e.DestStmID]
				if !destIn && e.DestBlock != nil {
					// dest block nested under removed stmt
					destIn = blockUnderStmt(removed, e.DestBlock)
				}
				if destIn {
					// Block.cpp:641–646 — if src is goto, remove_stmt(src)
					if e.SrcID > 0 && !ids[e.SrcID] {
						isGoto := true
						if fm.Func != nil {
							if src := FindStmtByID(fm.Func, e.SrcID); src != nil {
								isGoto = src.Kind == StmtGoto
							}
						}
						if isGoto {
							gotoSrcIDs = append(gotoSrcIDs, e.SrcID)
						}
					}
					continue
				}
				ne = append(ne, e)
			}
			if ne != nil {
				fm.CFGEdges = ne
			}
		}
	}

	// Block.cpp:655–663 — delete blocks inside s from Function.Blocks
	f := b.Func
	if f == nil && fm != nil {
		f = fm.Func
	}
	if f != nil {
		// Block* always live on Function.Blocks; nil hole fails closed
		// (nil whole list — no invent keep-hole partial scrub)
		nb := make([]*Block, 0, len(f.Blocks))
		incomplete := false
		for _, blk := range f.Blocks {
			if blk == nil {
				incomplete = true
				break
			}
			if blockUnderStmt(removed, blk) {
				continue
			}
			nb = append(nb, blk)
		}
		if incomplete {
			f.Blocks = nil
		} else {
			f.Blocks = nb
		}
	}

	// clear fact maps for removed tree
	if fm != nil {
		for id := range ids {
			delete(fm.MapFactsIn, id)
			delete(fm.MapFactsOut, id)
			delete(fm.MapStmEffect, id)
			delete(fm.MapAccumEffect, id)
			delete(fm.MapVisited, id)
		}
	}

	// Block.cpp:664–671 — erase s itself
	b.Stmts = append(b.Stmts[:idx], b.Stmts[idx+1:]...)
	cnt := 1

	// cascade-remove goto sources (may live in this or other blocks)
	seenGoto := map[int]bool{}
	for _, gid := range gotoSrcIDs {
		if gid <= 0 || seenGoto[gid] || ids[gid] {
			continue
		}
		seenGoto[gid] = true
		parent := b
		if fm != nil && fm.Func != nil {
			if p := FindParentBlockOfStmID(fm.Func, gid); p != nil {
				parent = p
			}
		}
		n := parent.RemoveStmt(gid, fm)
		if parent == b {
			cnt += n
		}
	}
	return cnt
}

// collectTypedStmIDs collects stm_ids of given kinds under st (find_typed_stmts light).
func collectTypedStmIDs(st *Stmt, kinds []StatementType, ids map[int]bool) {
	if st == nil || ids == nil {
		return
	}
	for _, k := range kinds {
		if st.Kind == k && st.StmID > 0 {
			ids[st.StmID] = true
			break
		}
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			collectTypedStmIDs(&st.Then.Stmts[i], kinds, ids)
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			collectTypedStmIDs(&st.Else.Stmts[i], kinds, ids)
		}
	}
}

// blockUnderStmt reports whether blk is the Then/Else of st or nested under them.
func blockUnderStmt(st *Stmt, blk *Block) bool {
	if st == nil || blk == nil {
		return false
	}
	if st.Then == blk || st.Else == blk {
		return true
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			if blockUnderStmt(&st.Then.Stmts[i], blk) {
				return true
			}
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			if blockUnderStmt(&st.Else.Stmts[i], blk) {
				return true
			}
		}
	}
	return false
}

func stmtTreeContainsID(st *Stmt, id int) bool {
	if st == nil {
		return false
	}
	if st.StmID == id {
		return true
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			if stmtTreeContainsID(&st.Then.Stmts[i], id) {
				return true
			}
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			if stmtTreeContainsID(&st.Else.Stmts[i], id) {
				return true
			}
		}
	}
	return false
}

// ResetStmFactMaps mirrors FactMgr::reset_stm_fact_maps for a statement tree.
// FactMgr.cpp:553–567.
func (fm *FactMgr) ResetStmFactMaps(st *Stmt) {
	if fm == nil || st == nil {
		return
	}
	ids := map[int]bool{}
	collectStmIDs(st, ids)
	if st.Then != nil {
		collectBlockStmIDs(st.Then, ids)
	}
	if st.Else != nil {
		collectBlockStmIDs(st.Else, ids)
	}
	for id := range ids {
		delete(fm.MapFactsIn, id)
		delete(fm.MapFactsOut, id)
	}
}

// ResetBlockFactMaps mirrors FactMgr::reset_stm_fact_maps(Block*).
// FactMgr.cpp:553–567 — clear in/out for all statements under block.
func (fm *FactMgr) ResetBlockFactMaps(b *Block) {
	if fm == nil || b == nil {
		return
	}
	ids := map[int]bool{}
	collectBlockStmIDs(b, ids)
	for id := range ids {
		delete(fm.MapFactsIn, id)
		delete(fm.MapFactsOut, id)
	}
}

func collectBlockStmIDs(b *Block, ids map[int]bool) {
	if b == nil {
		return
	}
	if b.StmID > 0 {
		ids[b.StmID] = true
	}
	for i := range b.Stmts {
		collectStmIDs(&b.Stmts[i], ids)
		if b.Stmts[i].Then != nil {
			collectBlockStmIDs(b.Stmts[i].Then, ids)
		}
		if b.Stmts[i].Else != nil {
			collectBlockStmIDs(b.Stmts[i].Else, ids)
		}
	}
}

// FindJumpSources mirrors Statement::find_jump_sources.
// Statement.cpp:492–506 — CFG edges with dest=stm and src eType==eGoto.
// Returns source StmIDs of gotos targeting destStmID.
// When fm.Func is set, non-goto sources (e.g. break→for) are excluded.
// CFGEdge* always live; nil hole → nil (fail closed). Complete empty → non-nil [].
func (fm *FactMgr) FindJumpSources(destStmID int) []int {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	srcs := make([]int, 0)
	for _, e := range fm.CFGEdges {
		// CFGEdge* always live; no invent skip nil holes as absent gotos
		if e == nil {
			return nil
		}
		if e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:501 — e->src->eType == eGoto (src always live Statement*)
		// unresolved SrcID with Func set = incomplete IR (no invent skip as non-goto)
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			if src == nil {
				return nil
			}
			if src.Kind != StmtGoto {
				continue
			}
		}
		srcs = append(srcs, e.SrcID)
	}
	return srcs
}

// FindJumpLabel mirrors Statement::find_jump_label.
// Statement.cpp:473–487 — label of first goto that jumps to destStmID.
// CFGEdge* always live; nil hole fails closed (empty label, no invent scan past hole).
func FindJumpLabel(fm *FactMgr, destStmID int) string {
	if fm == nil || destStmID <= 0 {
		return ""
	}
	for _, e := range fm.CFGEdges {
		if e == nil {
			// incomplete CFG — no invent label from partial scan or registry alone
			return ""
		}
		if e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			// unresolved src = incomplete function tree; no invent skip hole to registry
			if src == nil {
				return ""
			}
			if src.Kind != StmtGoto {
				continue
			}
			if src.Label != "" {
				return src.Label
			}
		}
	}
	// stm_labels registry when edge/func incomplete
	// no invent empty label token from registry
	if lab, ok := stmLabels[destStmID]; ok && lab != "" {
		return lab
	}
	return ""
}

// AppendNestedLoop mirrors Block::append_nested_loop.
// Block.cpp:422–435 — append a for statement for extra array dimensions.
func (b *Block) AppendNestedLoop(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
) *Stmt {
	// Block.cpp:422+ — Statement::make_random(eFor) always has RNG
	// no invent nested for shell without live block/context/RNG
	if b == nil || r == nil || cg == nil {
		return nil
	}
	var preFacts []*FactPointTo
	if cg.FM != nil {
		// incomplete GlobalFacts fail closed (no invent cleaned pre-for snapshot)
		if !FactsComplete(cg.FM.GlobalFacts) {
			return nil
		}
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}
	cg.ClearEffectStm()
	// Block.cpp:424 — Statement::make_random(eFor); ERROR_GUARD(nullptr)
	st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg)
	if st == nil || HasError() || !stmtOK(*st) {
		return nil
	}
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}
	b.Stmts = append(b.Stmts, *st)
	if cg.FM != nil {
		if !MakeupNewVarFacts(&preFacts, cg.FM.GlobalFacts) ||
			!FactsComplete(preFacts) || !FactsComplete(cg.FM.GlobalFacts) {
			// incomplete makeup must not invent SetMapFactsIn from cleared preFacts
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		if st.StmID > 0 {
			cg.FM.SetMapFactsIn(st.StmID, preFacts)
			cg.FM.SetMapFactsOut(st.StmID, cg.FM.GlobalFacts)
			if cg.FM.MapAccumEffect == nil {
				cg.FM.MapAccumEffect = make(map[int]Effect)
			}
			cg.FM.MapAccumEffect[st.StmID] = cg.AccumEffect()
			if cg.FM.MapVisited == nil {
				cg.FM.MapVisited = make(map[int]bool)
			}
			cg.FM.MapVisited[st.StmID] = true
			// fold for effect into block
			be := cg.FM.GetMapStmEffect(b.StmID)
			cg.FM.SetMapStmEffect(b.StmID, be.AddEffect(cg.FM.GetMapStmEffect(st.StmID)))
			cg.FM.MapAccumEffect[b.StmID] = cg.AccumEffect()
		}
	}
	return st
}

// AppendReturnStmt mirrors Block::append_return_stmt.
// Block.cpp:374–391 — make return, visit_facts, record fact/effect maps.
func (b *Block) AppendReturnStmt(r *Rng, opts Options, vs *VariableSelector, cg *CGContext) *Stmt {
	// Block.cpp:378+ — Statement::make_random(eReturn) always has RNG
	// no invent return shell without live block/context/RNG
	if b == nil || r == nil || cg == nil {
		return nil
	}
	fm := cg.FM
	var preFacts []*FactPointTo
	if fm != nil {
		// incomplete GlobalFacts fail closed (no invent cleaned pre-return snapshot)
		if !FactsComplete(fm.GlobalFacts) {
			return nil
		}
		preFacts = CloneFactSlice(fm.GlobalFacts)
	}
	cg.ClearEffectStm()
	// Block.cpp:378–380 — Statement::make_random(eReturn); ERROR_GUARD(nullptr)
	ret := MakeRandomReturn(r, opts, vs, cg)
	if !stmtOK(ret) {
		return nil
	}
	if ret.StmID == 0 {
		ret.StmID = AllocStmID()
	}
	b.Stmts = append(b.Stmts, ret)
	st := &b.Stmts[len(b.Stmts)-1]
	if fm != nil {
		if !MakeupNewVarFacts(&preFacts, fm.GlobalFacts) ||
			!FactsComplete(preFacts) || !FactsComplete(fm.GlobalFacts) {
			// incomplete makeup must not invent SetMapFactsIn from cleared preFacts
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		// Block.cpp:383–384 — sr->visit_facts; assert(visited)
		// no soft invent success / silent drop when visit fails
		if !VisitFactsStatementReturn(st, cg, opts) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			// C++ assert(visited) — sticky error for ERROR_GUARD callers
			SetError(ErrGeneric)
			return nil
		}
		if st.StmID > 0 {
			// Block.cpp:386–389 — set_fact_in; set_fact_out; accum; visited
			fm.SetMapFactsIn(st.StmID, preFacts)
			// set_fact_out filters function-locals for return (FactMgr.cpp:270–272)
			fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, b)
			if fm.MapAccumEffect == nil {
				fm.MapAccumEffect = make(map[int]Effect)
			}
			fm.MapAccumEffect[st.StmID] = cg.AccumEffect()
			if fm.MapVisited == nil {
				fm.MapVisited = make(map[int]bool)
			}
			fm.MapVisited[st.StmID] = true
			// Block.cpp:391–392 — map_accum_effect[block]; map_stm_effect[block] += return
			be := fm.GetMapStmEffect(b.StmID)
			fm.SetMapStmEffect(b.StmID, be.AddEffect(fm.GetMapStmEffect(st.StmID)))
			fm.MapAccumEffect[b.StmID] = cg.AccumEffect()
			if b.StmID > 0 {
				fm.SetMapFactsOut(b.StmID, fm.GlobalFacts)
			}
		}
	}
	return st
}

// ContainsBackEdge mirrors Block::contains_back_edge.
// Block.cpp:485–496 — CFG back_link whose dest->parent == this.
// CFGEdge* always live; nil hole fails closed as has-back (no invent none).
// Nil FactMgr fails closed has-back (no invent clean CFG without edges list).
func (b *Block) ContainsBackEdge(fm *FactMgr) bool {
	if b == nil {
		return false
	}
	if fm == nil {
		return true
	}
	for _, e := range fm.CFGEdges {
		if e == nil {
			return true
		}
		if !e.BackLink {
			continue
		}
		// Block.cpp:491 — edge->dest->parent == this
		if e.DestBlock == b {
			return true
		}
	}
	return false
}

func blockHasStmtID(b *Block, id int) bool {
	if b == nil {
		return false
	}
	for i := range b.Stmts {
		if b.Stmts[i].StmID == id {
			return true
		}
	}
	return false
}

// MakeDummyBlockCG mirrors Block::make_dummy_block with CGContext.
// Block.cpp:95–110 — empty block, fact_in, post_creation_analysis, stack pop.
func MakeDummyBlockCG(cg *CGContext, opts Options) *Block {
	// Block.cpp:96–97 — assert(curr_func)
	if cg == nil || cg.CurrentFunc == nil {
		return nil
	}
	f := cg.CurrentFunc
	parent := cg.CurrentBlock()
	// Block.cpp:99 — Block(get_current_block(), 0)
	b := &Block{
		Parent:      parent,
		Func:        f,
		blockSize:   0,
		StmID:       AllocStmID(),
		// Block.cpp:101 — in_array_loop from iv_bounds
		InArrayLoop: len(cg.IVBounds) > 0,
		EmitFM:      cg.FM,
	}
	// Block.cpp:102–103 — blocks + stack push
	f.Blocks = append(f.Blocks, b)
	f.Stack = append(f.Stack, b)
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		preEffect = cg.EffectAccum.Clone()
	}
	// Block.cpp:105 — set_fact_in(b, global_facts)
	if cg.FM != nil {
		cg.FM.SetMapFactsIn(b.StmID, cg.FM.GlobalFacts)
	}
	// Block.cpp:107 — post_creation_analysis
	b.PostCreationAnalysis(cg, opts, preEffect, nil, nil)
	// Block.cpp:108 — stack pop
	if len(f.Stack) > 0 {
		f.Stack = f.Stack[:len(f.Stack)-1]
	}
	return b
}

// AddNewVarFactTo mirrors FactMgr::add_new_var_fact into a fact vector.
// FactMgr.cpp:118–131 / Block.cpp:546–549 — abstract_fact_for_var_init only.
// No invent NewFactPointTo garbage when init abstract fails or InitExpr is the RHS.
func AddNewVarFactTo(v *Variable, facts *[]*FactPointTo) {
	// same path as MakeupNewVarFacts / AddNewVarFactInto
	AddNewVarFactInto(v, facts)
}

// ShortcutAnalysisBlock mirrors Statement::shortcut_analysis for a Block.
// Statement.cpp:545–567 — same_facts && !is_ctrl_stmt && !contains_unfixed_goto.
// Incomplete or missing map_facts_out fails closed (ShortcutNone) — no invent
// reuse success while leaving inputs unchanged or cloning past nil holes.
func ShortcutAnalysisBlock(b *Block, facts *[]*FactPointTo, cg *CGContext) int {
	if b == nil || facts == nil || cg == nil || cg.FM == nil || b.StmID == 0 {
		return ShortcutNone
	}
	fm := cg.FM
	in, ok := fm.MapFactsIn[b.StmID]
	if !ok {
		return ShortcutNone
	}
	// Fact* always live in maps; incomplete in/inputs fail closed
	if !FactsComplete(*facts) || !FactsComplete(in) {
		return ShortcutNone
	}
	if !SameFacts(*facts, in) {
		return ShortcutNone
	}
	// Statement.cpp:552 — !contains_unfixed_goto()
	if ContainsUnfixedGotoBlock(b, fm) {
		return ShortcutNone
	}
	// block is not is_ctrl_stmt
	eff := fm.GetMapStmEffect(b.StmID)
	if cg.InConflict(eff) {
		return ShortcutConflict
	}
	// Statement.cpp:559 — inputs = map_facts_out[this]; out must be present and complete
	out, ok := fm.MapFactsOut[b.StmID]
	if !ok || !FactsComplete(out) {
		return ShortcutNone
	}
	*facts = CloneFactSlice(out)
	cg.AddEffect(eff, false)
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[b.StmID] = cg.AccumEffect()
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[b.StmID] = true
	return ShortcutOK
}
