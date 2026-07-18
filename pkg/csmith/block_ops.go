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
	check := func(v *Variable) bool {
		if v == nil || !v.IsArray {
			return false
		}
		dimen := len(v.ArraySizes)
		if v.AsArray != nil && len(v.AsArray.Sizes) > dimen {
			dimen = len(v.AsArray.Sizes)
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
		if check(v) {
			return true
		}
	}
	for _, v := range cg.RW.MustWriteVars {
		if check(v) {
			return true
		}
	}
	return false
}

// RemoveStmt mirrors Block::remove_stmt by StmID.
// Block.cpp:586+ — remove from stms; drop CFG edges involving it; scrub break list.
// Returns number of statements removed from this block.
func (b *Block) RemoveStmt(stmID int, fm *FactMgr) int {
	if b == nil || stmID <= 0 {
		return 0
	}
	// collect IDs inside removed tree
	var removed *Stmt
	idx := -1
	for i := range b.Stmts {
		if b.Stmts[i].StmID == stmID || stmtTreeContainsID(&b.Stmts[i], stmID) {
			// only remove top-level match by exact id, or containing tree if root matches
			if b.Stmts[i].StmID == stmID {
				removed = &b.Stmts[i]
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		// search nested blocks
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
	// scrub break_stms on enclosing loop
	loop := b
	for loop != nil && !loop.Looping {
		loop = loop.Parent
	}
	if loop != nil {
		nb := loop.BreakStmIDs[:0]
		for _, id := range loop.BreakStmIDs {
			if !ids[id] {
				nb = append(nb, id)
			}
		}
		loop.BreakStmIDs = nb
	}
	// remove CFG edges with src or dest in removed set
	if fm != nil {
		ne := fm.CFGEdges[:0]
		for _, e := range fm.CFGEdges {
			if e == nil {
				continue
			}
			if ids[e.SrcID] || ids[e.DestStmID] {
				continue
			}
			ne = append(ne, e)
		}
		fm.CFGEdges = ne
		// clear fact maps for removed ids
		for id := range ids {
			delete(fm.MapFactsIn, id)
			delete(fm.MapFactsOut, id)
			delete(fm.MapStmEffect, id)
			delete(fm.MapAccumEffect, id)
			delete(fm.MapVisited, id)
		}
	}
	// erase from stms
	b.Stmts = append(b.Stmts[:idx], b.Stmts[idx+1:]...)
	return 1
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
func (fm *FactMgr) FindJumpSources(destStmID int) []int {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	var srcs []int
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:501 — e->src->eType == eGoto
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			if src == nil || src.Kind != StmtGoto {
				continue
			}
		}
		srcs = append(srcs, e.SrcID)
	}
	return srcs
}

// FindJumpLabel mirrors Statement::find_jump_label.
// Statement.cpp:473–487 — label of first goto that jumps to destStmID.
func FindJumpLabel(fm *FactMgr, destStmID int) string {
	if fm == nil || destStmID <= 0 {
		return ""
	}
	for _, e := range fm.CFGEdges {
		if e == nil || e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			if src == nil || src.Kind != StmtGoto {
				continue
			}
			if src.Label != "" {
				return src.Label
			}
		}
	}
	// stm_labels registry when edge/func incomplete
	if lab, ok := stmLabels[destStmID]; ok {
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
	cg CGContext,
) *Stmt {
	if b == nil {
		return nil
	}
	var preFacts []*FactPointTo
	if cg.FM != nil {
		preFacts = CloneFactSlice(cg.FM.GlobalFacts)
	}
	cg.ClearEffectStm()
	st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg)
	if st == nil {
		return nil
	}
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}
	b.Stmts = append(b.Stmts, *st)
	if cg.FM != nil {
		MakeupNewVarFacts(&preFacts, cg.FM.GlobalFacts)
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
	if b == nil || cg == nil {
		return nil
	}
	fm := cg.FM
	var preFacts []*FactPointTo
	if fm != nil {
		preFacts = CloneFactSlice(fm.GlobalFacts)
	}
	cg.ClearEffectStm()
	ret := MakeRandomReturn(r, opts, vs, *cg)
	if ret.StmID == 0 {
		ret.StmID = AllocStmID()
	}
	b.Stmts = append(b.Stmts, ret)
	st := &b.Stmts[len(b.Stmts)-1]
	if fm != nil {
		MakeupNewVarFacts(&preFacts, fm.GlobalFacts)
		// visit_facts on return
		ok := VisitFactsStatementReturn(st, cg, opts)
		if !ok {
			// still keep statement; upstream asserts visited
			_ = ok
		}
		if st.StmID > 0 {
			fm.SetMapFactsIn(st.StmID, preFacts)
			fm.SetMapFactsOut(st.StmID, fm.GlobalFacts)
			if fm.MapAccumEffect == nil {
				fm.MapAccumEffect = make(map[int]Effect)
			}
			fm.MapAccumEffect[st.StmID] = cg.AccumEffect()
			if fm.MapVisited == nil {
				fm.MapVisited = make(map[int]bool)
			}
			fm.MapVisited[st.StmID] = true
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
func (b *Block) ContainsBackEdge(fm *FactMgr) bool {
	if b == nil || fm == nil {
		return false
	}
	for _, e := range fm.CFGEdges {
		if e == nil || !e.BackLink {
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
// Block.cpp:95–110 — empty block, fact_in, post_creation_analysis.
func MakeDummyBlockCG(cg *CGContext, opts Options) *Block {
	if cg == nil || cg.CurrentFunc == nil {
		return nil
	}
	f := cg.CurrentFunc
	parent := cg.CurrentBlock()
	b := &Block{
		Parent:      parent,
		Func:        f,
		blockSize:   0,
		StmID:       AllocStmID(),
		InArrayLoop: len(cg.IVBounds) > 0,
	}
	f.Blocks = append(f.Blocks, b)
	f.Stack = append(f.Stack, b)
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		preEffect = cg.EffectAccum.Clone()
	}
	if cg.FM != nil {
		cg.FM.SetMapFactsIn(b.StmID, cg.FM.GlobalFacts)
	}
	b.PostCreationAnalysis(cg, opts, preEffect, nil, nil)
	if len(f.Stack) > 0 {
		f.Stack = f.Stack[:len(f.Stack)-1]
	}
	return b
}

// AddNewVarFactTo mirrors FactMgr::add_new_var_fact into a fact vector.
// FactMgr.cpp:118–131 subset for pointer init into outputs (find_fixed_point).
func AddNewVarFactTo(v *Variable, facts *[]*FactPointTo) {
	if v == nil || facts == nil {
		return
	}
	if !v.IsPointer() {
		for _, f := range v.FieldVars {
			AddNewVarFactTo(f, facts)
		}
		return
	}
	if FindRelatedPointTo(*facts, v) != nil {
		return
	}
	if v.Init != nil {
		rhs := &Expression{Term: TermConstant, Con: v.Init}
		newFacts := AbstractFactForAssign(nil, v, 0, rhs)
		for _, f := range newFacts {
			*facts = MergeFactInto(*facts, f)
		}
		if len(newFacts) > 0 {
			return
		}
	}
	*facts = append(*facts, NewFactPointTo(v))
}

// ShortcutAnalysisBlock mirrors Statement::shortcut_analysis for a Block.
// Statement.cpp:545–567 via Block as Statement.
func ShortcutAnalysisBlock(b *Block, facts *[]*FactPointTo, cg *CGContext) int {
	if b == nil || facts == nil || cg == nil || cg.FM == nil || b.StmID == 0 {
		return ShortcutNone
	}
	fm := cg.FM
	in, ok := fm.MapFactsIn[b.StmID]
	if !ok {
		return ShortcutNone
	}
	if !SameFacts(*facts, in) {
		return ShortcutNone
	}
	// block is not is_ctrl_stmt; skip unfixed goto under block for light path
	eff := fm.GetMapStmEffect(b.StmID)
	if cg.InConflict(eff) {
		return ShortcutConflict
	}
	if out, ok := fm.MapFactsOut[b.StmID]; ok {
		*facts = CloneFactSlice(out)
	}
	cg.AddEffect(eff, false)
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[b.StmID] = cg.AccumEffect()
	return ShortcutOK
}
