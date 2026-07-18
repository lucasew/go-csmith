// Upstream: Block.cpp remove_stmt, need_nested_loop, must_break_or_return details.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustBreakOrReturnFull mirrors Block::must_break_or_return.
// Block.cpp:342–357 — last must_return unless continue-like back edge from outside.
func (b *Block) MustBreakOrReturnFull(fm *FactMgr) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	last := b.GetLastStm()
	if last == nil || !last.MustReturn() {
		return false
	}
	if fm == nil || b.StmID == 0 {
		return true
	}
	// if back edge into block from a statement that is not the block itself, can escape return
	edges := fm.FindEdgesInToBlock(b, false, true)
	// also edges with DestStmID pointing at first stmt?
	for _, e := range edges {
		if e != nil && e.SrcID != b.StmID {
			return false
		}
	}
	// DestStmID == first statement of block
	if len(b.Stmts) > 0 && b.Stmts[0].StmID > 0 {
		for _, e := range fm.FindEdgesIn(b.Stmts[0].StmID, false, true) {
			if e != nil && e.SrcID != b.StmID {
				return false
			}
		}
	}
	return true
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
// Statement.cpp:492–506 — CFG edges with dest=stm and src kind goto.
// Returns source StmIDs of gotos targeting destStmID.
func (fm *FactMgr) FindJumpSources(destStmID int) []int {
	if fm == nil || destStmID <= 0 {
		return nil
	}
	var srcs []int
	for _, e := range fm.CFGEdges {
		if e == nil {
			continue
		}
		if e.DestStmID == destStmID && e.SrcID > 0 {
			srcs = append(srcs, e.SrcID)
		}
	}
	return srcs
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
	}
	return st
}
