// Upstream: Block.cpp remove_stmt, need_nested_loop, must_break_or_return details.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustBreakOrReturnFull mirrors Block::must_break_or_return.
// Block.cpp:342–357 — last must_return unless continue-like back edge from outside.
// Note: unlike must_return, does not require break_stms empty.
// Block always live; sticky false (no invent not-must-break soft-skip past hole).
func (b *Block) MustBreakOrReturnFull(fm *FactMgr) bool {
	s := sessFromFM(fm)
	if s == nil {
		s = testAmbientSession
	}
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if len(b.Stmts) == 0 {
		return false
	}
	last := b.GetLastStmSess(s)
	if last == nil || !last.MustReturnSess(s) {
		// residual ERROR sticky — no invent not-must-return soft-skip past MustReturn hole
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent must-return true past MustReturn hole
	if sessHasError(s) {
		return false
	}
	// Block.cpp:345–353 — same back-edge escape check as must_return
	esc := b.hasEscapeBackEdge(fm)
	// residual ERROR sticky — no invent no-escape soft-skip past CFG hole
	if sessHasError(s) {
		// incomplete CFG sticky restrictive must-return false (escape uncertain)
		return false
	}
	return !esc
}

// NeedNestedLoop mirrors Block::need_nested_loop.
// Block.cpp:390–420 — looping block needs deeper IV for must-use array dimensions.
// Incomplete Block/Rng sticky false (no invent nested-loop soft-skip past holes).
func (b *Block) NeedNestedLoop(cg CGContext, r *Rng) bool {
	// Block + Rng always live; sticky incomplete no invent not-nested soft-skip
	if b == nil || r == nil {
		noteErrCG(&cg, ErrGeneric)
		return false
	}
	if !b.Looping {
		return false
	}
	s := b.GetLastStmSess(sessFromCG(&cg))
	if s != nil && s.MustJumpSess(sessFromCG(&cg)) {
		return false
	}
	// residual ERROR sticky — no invent not-must-jump soft-skip past MustJump hole
	// incomplete last jump sticky restrictive need nested (no invent "no nested" past hole)
	if hasErrCG(&cg) {
		return true
	}
	if cg.RW == nil {
		return false
	}
	// incomplete must-use lists fail closed sticky true (no invent "no nested loop"
	// by soft-skipping holes as absent must-use / soft re-pick past incomplete RW)
	if !VariablesComplete(cg.RW.MustReadVars) || !VariablesComplete(cg.RW.MustWriteVars) {
		noteErrCG(&cg, ErrGeneric)
		return true
	}
	ivDepth := 0
	if cg.IVBounds != nil {
		ivDepth = len(cg.IVBounds)
	}
	// Block.cpp:399–414 — must_read/write vars use get_dimension()
	// pre-validated VariablesComplete
	check := func(v *Variable) bool {
		dimen := v.GetDimensionSess(sessFromCG(&cg))
		// incomplete array IR (IsArray without AsArray) stickies via GetDimension
		// sticky need nested (restrictive — no invent "no nested" past broken dim)
		if hasErrCG(&cg) {
			return true
		}
		if dimen == 0 {
			return false
		}
		if dimen > ivDepth {
			return true
		}
		if dimen == ivDepth && r.RndFlipcoinSess(sessFromCG(&cg), 10) {
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
// Block.cpp:586–652 — scrub break_stms; drop CFG edges from control stmts inside s;
// drop edges into s and recursively remove goto sources; drop contained blocks
// from Function.Blocks; erase s from stms.
// Returns number of statements removed from this block (including cascaded gotos).
// Block always live; sticky 0 (no invent no-op remove soft-skip past hole).
func (b *Block) RemoveStmt(stmID int, fm *FactMgr) int {
	if b == nil {
		noteErrFM(fm, ErrGeneric)
		return 0
	}
	if StmIDUnset(stmID) {
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
		// search nested get_blocks only (kind-gated; no invent Then on assign)
		for i := range b.Stmts {
			blks := GetBlocksStmtSess(sessFromFM(fm), &b.Stmts[i])
			// pre-validate complete arms — nil hole fails closed sticky (no invent
			// soft-skip missing arm then search sibling arm as complete tree)
			for _, blk := range blks {
				if blk == nil {
					noteErrFM(fm, ErrGeneric)
					return 0
				}
			}
			for _, blk := range blks {
				if n := blk.RemoveStmt(stmID, fm); n > 0 {
					return n
				}
			}
		}
		return 0
	}
	ids := map[int]bool{}
	// get_blocks tree of stm+nested block ids; incomplete → partial known only
	// (no invent ids via stray Then/Else on non-compound kinds)
	_ = collectTreeStmAndBlockIDsSess(sessFromFM(fm), removed, ids)

	// Statement.cpp find_typed_stmts: continue/break/goto inside s
	cfgIDs := map[int]bool{}
	if !collectTypedStmIDsSess(sessFromFM(fm), removed, []StatementType{StmtBreak, StmtContinue, StmtGoto}, cfgIDs) {
		// incomplete IR under removed — fail closed sticky wipe CFG with hole marker
		// (not bare nil — CFGEdgesComplete(nil) invents empty-complete edge set)
		if fm != nil {
			fm.CFGEdges = IncompleteCFGEdges()
			noteErrFM(fm, ErrGeneric)
		}
		cfgIDs = map[int]bool{}
	}

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
		// CFGEdge* always live; nil hole fails closed IncompleteCFGEdges
		// (no invent keep-hole partial scrub / empty-complete via bare nil)
		if !CFGEdgesComplete(fm.CFGEdges) {
			fm.CFGEdges = IncompleteCFGEdges()
			noteErrFM(fm, ErrGeneric)
			gotoSrcIDs = nil
		} else {
			ne := make([]*CFGEdge, 0, len(fm.CFGEdges))
			for _, e := range fm.CFGEdges {
				if cfgIDs[e.SrcID] {
					continue
				}
				ne = append(ne, e)
			}
			fm.CFGEdges = ne

			// Block.cpp:632–652 — remove edges with dest inside s; cascade-delete gotos
			// C++: if (s->contains_stmt(edge->dest)) — full Statement::contains_stmt
			// (Statement.cpp:684–705). For Block* dest, eBlock uses parent chain, not
			// Stmts-only walk. Soft invent was blockUnderStmtSess(s, get_blocks+Stmts) which
			// missed orphan DestBlock → kept CFG edges / skipped goto cascade.
			ne = make([]*CFGEdge, 0, len(fm.CFGEdges))
			scrubIncomplete := false
			for _, e := range fm.CFGEdges {
				destIn := !StmIDUnset(e.DestStmID) && ids[e.DestStmID]
				if !destIn && e.DestBlock != nil {
					// Block* dest: parent-chain contains_stmt (not Stmts walk)
					destIn = stmtContainsBlockSess(sessFromFM(fm), removed, e.DestBlock)
					// residual ERROR sticky — no invent dest-in scan past contains residual
					if hasErrFM(fm) {
						scrubIncomplete = true
						break
					}
				}
				if !destIn && !StmIDUnset(e.DestStmID) && fm.Func != nil &&
					BlocksComplete(fm.Func.Blocks) {
					// Dest may be a Block on Func.Blocks (Block is-a Statement) whose
					// StmID was never in the Stmts tree (orphan after failed arm).
					for _, blk := range fm.Func.Blocks {
						if blk != nil && blk.StmID == e.DestStmID {
							destIn = stmtContainsBlockSess(sessFromFM(fm), removed, blk)
							if hasErrFM(fm) {
								scrubIncomplete = true
							}
							break
						}
					}
					if scrubIncomplete {
						break
					}
				}
				if destIn {
					// Block.cpp:641–646 — if src is goto, remove_stmt(src)
					// Soft invent was FindStmt residual/nil miss → isGoto true cascade invent.
					// Fair: sticky fail closed wipe CFG (no invent goto cascade past hole).
					if e.SrcID > 0 && !ids[e.SrcID] {
						if fm.Func != nil {
							src := FindStmtByIDSess(sessFromFM(fm), fm.Func, e.SrcID)
							if hasErrFM(fm) {
								scrubIncomplete = true
								break
							}
							if src == nil {
								// unresolved with Func = incomplete IR sticky
								noteErrFM(fm, ErrGeneric)
								scrubIncomplete = true
								break
							}
							if src.Kind == StmtGoto {
								gotoSrcIDs = append(gotoSrcIDs, e.SrcID)
							}
						} else {
							// no Func to classify — cascade as C++ assert-less default
							gotoSrcIDs = append(gotoSrcIDs, e.SrcID)
						}
					}
					continue
				}
				ne = append(ne, e)
			}
			if scrubIncomplete {
				fm.CFGEdges = IncompleteCFGEdges()
				gotoSrcIDs = nil
				if !hasErrFM(fm) {
					noteErrFM(fm, ErrGeneric)
				}
			} else {
				fm.CFGEdges = ne
			}
		}
	}

	// Block.cpp:655–663 — delete blocks inside s from Function.Blocks
	// via s->contains_stmt(b) (Statement.cpp:684–705). Block is-a Statement in
	// C++; eBlock uses parent-chain of the candidate (not Stmts walk). Soft invent
	// was get_blocks+Stmts-only (blockUnderStmt) which missed failed/orphan
	// nested blocks that still have Parent set → Func.Blocks inflated for
	// StatementGoto::find_good_jump_block (seed 114667… first_div n=37 vs UP n=3).
	f := b.Func
	if f == nil && fm != nil {
		f = fm.Func
	}
	if f != nil {
		// Block* always live on Function.Blocks; nil hole fails closed sticky
		// IncompleteBlocks (not bare nil invent empty-complete Blocks list)
		if !BlocksComplete(f.Blocks) {
			f.Blocks = IncompleteBlocks()
			noteErrFM(fm, ErrGeneric)
		} else {
			nb := make([]*Block, 0, len(f.Blocks))
			for _, blk := range f.Blocks {
				if stmtContainsBlockSess(sessFromFM(fm), removed, blk) {
					// residual ERROR sticky — no invent keep-block past contains residual
					if hasErrFM(fm) {
						f.Blocks = IncompleteBlocks()
						break
					}
					continue
				}
				// residual ERROR sticky — no invent keep-block past contains residual false
				if hasErrFM(fm) {
					f.Blocks = IncompleteBlocks()
					break
				}
				nb = append(nb, blk)
			}
			if BlocksComplete(f.Blocks) {
				f.Blocks = nb
			}
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
			if p := FindParentBlockOfStmIDSess(sessFromFM(fm), fm.Func, gid); p != nil {
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
// Uses kind-gated get_blocks; sticky false on incomplete Block* hole
// (no invent partial typed list past missing if-arm as complete).
func collectTypedStmIDsSess(s *Session, st *Stmt, kinds []StatementType, ids map[int]bool) bool {
	// Statement + id set always live; sticky incomplete no invent empty typed list
	if st == nil || ids == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	var stms []*Stmt
	if FindTypedStmtsSess(s, st, &stms, kinds) < 0 {
		// FindTypedStmts may already sticky IncompleteStmtsSlice
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return false
	}
	for _, stt := range stms {
		if stt != nil && !StmIDUnset(stt.StmID) {
			ids[stt.StmID] = true
		}
	}
	return true
}

// blockUnderStmt reports whether blk is under st via get_blocks+Stmts walk.
// Prefer stmtContainsBlock for remove_stmt Func.Blocks scrub (C++ contains_stmt).
// Incomplete get_blocks hole sticky true (no invent "not under" while soft-skipping
// a nil if-arm — scrub aggressively / treat as contained for remove_stmt).
func blockUnderStmtSess(s *Session, st *Stmt, blk *Block) bool {
	// Statement + Block always live; sticky incomplete no invent not-under soft-skip
	if st == nil || blk == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, b := range GetBlocksStmtSess(s, st) {
		if b == nil {
			// incomplete arm sticky contained (restrictive scrub)
			sessNoteError(s, ErrGeneric)
			return true
		}
		if b == blk {
			return true
		}
		for i := range b.Stmts {
			if blockUnderStmtSess(s, &b.Stmts[i], blk) {
				// residual ERROR sticky — no invent soft-continue under-scan past hole
				return true
			}
			// residual ERROR sticky — no invent soft-skip not-under past recursive hole
			if sessHasError(s) {
				return true
			}
		}
	}
	return false
}

// stmtContainsBlock mirrors Statement::contains_stmt when the argument is a Block*
// (C++ Block is-a Statement). Statement.cpp:684–705 + Block.cpp:655–663 remove_stmt.
//
//	if (this == s) return true;
//	if (eType == eBlock) {
//	  for (tmp = s->parent; tmp; tmp = tmp->parent)
//	    if (tmp == this) return true;
//	  return false;
//	}
//	get_blocks → each child Block::contains_stmt(s)  // parent-chain only
//
// Soft invent was Stmts-only walk (blockUnderStmt): missed orphan/failed nested
// blocks that still have Parent pointing into the removed tree but were never
// linked as StmtBlock children — left them on Func.Blocks for goto pool.
func stmtContainsBlockSess(s *Session, st *Stmt, blk *Block) bool {
	// Statement + Block always live; sticky incomplete no invent not-contain soft-skip
	if st == nil || blk == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// C++ this == s: StmtBlock statement object is not the Block*; Then is.
	if st.Kind == StmtBlock {
		// Statement.cpp:689–694 — eBlock: identity or candidate parent chain
		if st.Then == nil {
			// incomplete nested block sticky (no invent not-contain past hole)
			sessNoteError(s, ErrGeneric)
			return false
		}
		return blockIsSelfOrAncestorSess(s, st.Then, blk)
	}
	// Non-eBlock: get_blocks then each as eBlock contains_stmt
	blks := GetBlocksStmtSess(s, st)
	// pre-validate complete get_blocks (if always both arms; for body always live)
	for _, b := range blks {
		if b == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
	}
	for _, b := range blks {
		if blockIsSelfOrAncestorSess(s, b, blk) {
			return true
		}
	}
	return false
}

// blockIsSelfOrAncestor mirrors Block-as-eBlock Statement::contains_stmt(candidate).
// Statement.cpp:684–694 — this==s or candidate's parent chain includes this.
func blockIsSelfOrAncestorSess(s *Session, ancestor, blk *Block) bool {
	if ancestor == nil || blk == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if ancestor == blk {
		return true
	}
	for tmp := blk.Parent; tmp != nil; tmp = tmp.Parent {
		if tmp == ancestor {
			return true
		}
	}
	return false
}

// stmtTreeContainsID reports whether id appears under st via get_blocks.
// Incomplete arm sticky false (no invent membership past holes).
func stmtTreeContainsIDSess(s *Session, st *Stmt, id int) bool {
	// Statement always live; sticky incomplete no invent not-contain soft-skip
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if id <= 0 {
		return false
	}
	if st.StmID == id {
		return true
	}
	blks := GetBlocksStmtSess(s, st)
	for _, b := range blks {
		if b == nil {
			// incomplete arm sticky not-contain
			sessNoteError(s, ErrGeneric)
			return false
		}
	}
	for _, b := range blks {
		if b.StmID == id {
			return true
		}
		for i := range b.Stmts {
			if stmtTreeContainsIDSess(s, &b.Stmts[i], id) {
				return true
			}
		}
	}
	return false
}

// ResetStmFactMaps mirrors FactMgr::reset_stm_fact_maps for a statement tree.
// FactMgr.cpp:553–567 — walk get_blocks only (no invent via stray Then on assign).
// Clears both ePointTo and eUnionWrite partitions (full FactVec empty).
// FactMgr + Statement always live; sticky (no invent soft-skip reset past hole).
func (fm *FactMgr) ResetStmFactMaps(st *Stmt) {
	if fm == nil || st == nil {
		noteErrFM(fm, ErrGeneric)
		return
	}
	ids := map[int]bool{}
	_ = collectTreeStmAndBlockIDsSess(sessFromFM(fm), st, ids)
	for id := range ids {
		delete(fm.MapFactsIn, id)
		delete(fm.MapFactsOut, id)
		delete(fm.MapUnionFactsIn, id)
		delete(fm.MapUnionFactsOut, id)
	}
}

// ResetBlockFactMaps mirrors FactMgr::reset_stm_fact_maps(Block*).
// FactMgr.cpp:553–567 — clear in/out for all statements under block (full FactVec).
// FactMgr + Block always live; sticky (no invent soft-skip reset past hole).
func (fm *FactMgr) ResetBlockFactMaps(b *Block) {
	if fm == nil || b == nil {
		noteErrFM(fm, ErrGeneric)
		return
	}
	ids := map[int]bool{}
	_ = collectBlockStmIDsSess(sessFromFM(fm), b, ids)
	for id := range ids {
		delete(fm.MapFactsIn, id)
		delete(fm.MapFactsOut, id)
		delete(fm.MapUnionFactsIn, id)
		delete(fm.MapUnionFactsOut, id)
	}
}

// collectTreeStmAndBlockIDs records st's StmID and nested get_blocks Block/Stmt ids.
// Sticky false on incomplete Block* hole (no invent partial tree as complete).
func collectTreeStmAndBlockIDsSess(s *Session, st *Stmt, ids map[int]bool) bool {
	// Statement + id set always live; sticky incomplete no invent empty tree
	if st == nil || ids == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !StmIDUnset(st.StmID) {
		ids[st.StmID] = true
	}
	for _, b := range GetBlocksStmtSess(s, st) {
		if b == nil {
			// incomplete arm sticky fail closed
			sessNoteError(s, ErrGeneric)
			return false
		}
		if !collectBlockStmIDsSess(s, b, ids) {
			return false
		}
	}
	return true
}

// collectBlockStmIDs records b.StmID and nested get_blocks trees.
// Sticky false on incomplete hole.
func collectBlockStmIDsSess(s *Session, b *Block, ids map[int]bool) bool {
	// Block + id set always live; sticky incomplete no invent empty block tree
	if b == nil || ids == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !StmIDUnset(b.StmID) {
		ids[b.StmID] = true
	}
	for i := range b.Stmts {
		if !collectTreeStmAndBlockIDsSess(s, &b.Stmts[i], ids) {
			return false
		}
	}
	return true
}

// FindJumpSources mirrors Statement::find_jump_sources.
// Statement.cpp:492–506 — CFG edges with dest=stm and src eType==eGoto.
// Returns source StmIDs of gotos targeting destStmID.
// When fm.Func is set, non-goto sources (e.g. break→for) are excluded.
// FactMgr + live dest StmID always required; sticky nil (no invent empty soft-success).
// Incomplete CFG fails closed sticky nil (no invent soft re-pick empty sources past holes).
// Complete empty → non-nil [].
func (fm *FactMgr) FindJumpSources(destStmID int) []int {
	if fm == nil || StmIDUnset(destStmID) {
		noteErrFM(fm, ErrGeneric)
		return nil
	}
	// incomplete CFG fails closed sticky nil (distinct from complete empty non-nil [])
	if !CFGEdgesComplete(fm.CFGEdges) {
		noteErrFM(fm, ErrGeneric)
		return nil
	}
	srcs := make([]int, 0)
	for _, e := range fm.CFGEdges {
		if e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:501 — e->src->eType == eGoto (src always live Statement*)
		// unresolved SrcID with Func set = incomplete IR sticky (no invent skip as non-goto)
		if fm.Func != nil {
			src := FindStmtByIDSess(sessFromFM(fm), fm.Func, e.SrcID)
			// residual ERROR sticky — no invent soft-continue sources past FindStmt hole
			if hasErrFM(fm) {
				return nil
			}
			if src == nil {
				noteErrFM(fm, ErrGeneric)
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
// Incomplete CFG fails closed sticky (empty label — no invent scan/registry past hole).
func FindJumpLabel(fm *FactMgr, destStmID int) string {
	// FactMgr always live for CFG label lookup; sticky no invent empty label without it
	if fm == nil {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	// StmID ≤0 incomplete dest sticky (no invent label for key 0 / soft re-pick)
	if StmIDUnset(destStmID) {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	// incomplete CFG fails closed sticky (no invent label from partial scan or registry)
	if !CFGEdgesComplete(fm.CFGEdges) {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	for _, e := range fm.CFGEdges {
		if e.DestStmID != destStmID || e.SrcID <= 0 {
			continue
		}
		if fm.Func != nil {
			src := FindStmtByIDSess(sessFromFM(fm), fm.Func, e.SrcID)
			// residual ERROR sticky — no invent soft-continue label scan past FindStmt hole
			if hasErrFM(fm) {
				return ""
			}
			// unresolved src = incomplete function tree sticky; no invent skip hole to registry
			if src == nil {
				noteErrFM(fm, ErrGeneric)
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
	if lab := lookupStmLabelSess(testAmbientSession, destStmID); lab != "" {
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
	// sticky no invent nested for shell without live block/context/RNG
	if b == nil || r == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky before EffectStm clear (no invent nested for)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// Block.cpp:422–427 — outer pre_facts snapshot, clear effect_stm, then
	// Statement::make_random(cg, eFor). make_random does its own pre_facts for
	// post_creation; outer pre_facts is for makeup_new_var_facts / set_fact_in.
	// Must go through makeRandomStmtForced (not MakeRandomFor alone) so
	// expr_depth=0, compound blk_depth bump, shallow post_creation pre_facts,
	// and null→re-pick any kind match Statement.cpp:314–316 (seed 2020240685
	// NeedNested append path; seed-4 blk_depth).
	var preFacts []*FactPointTo
	var preUnion []*FactUnion
	if cg.FM != nil {
		// incomplete GlobalFacts fail closed sticky (no invent cleaned pre-for snapshot)
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Block.cpp:426 — FactVec pre_facts = fm->global_facts (shallow)
		preFacts = append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
		preUnion = append([]*FactUnion(nil), cg.FM.UnionFacts...)
	}
	cg.ClearEffectStm()
	// Statement.cpp make_random(eFor): full factory + post_creation + null re-pick
	made := makeRandomStmtForced(r, opts, probs, vs, tables, stmtTab, cg, b, StmtFor)
	if hasErrCG(cg) || !stmtOK(made) {
		return nil
	}
	st := &made
	if StmIDUnset(st.StmID) {
		st.StmID = AllocStmIDSess(sessFromCG(cg))
	}
	b.Stmts = append(b.Stmts, *st)
	if cg.FM != nil {
		// Block::stm_id always live when FM bound (no invent fold into key 0)
		if StmIDUnset(b.StmID) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Block.cpp:429 — makeup_new_var_facts(pre_facts, global_facts) full FactVec
		// Soft invent was PT-only MakeupNewVarFacts then SetMapFactsInPair with
		// preUnion without makeupNewUnionFacts — undid PostCreationAnalysis union
		// makeup and left map_facts_in missing mid-for union subjects.
		if !MakeupNewVarFactsSess(sessFromCG(cg), &preFacts, cg.FM.GlobalFacts) ||
			!makeupNewUnionFactsSess(sessFromCG(cg), &preUnion, cg.FM.UnionFacts) ||
			!FactsComplete(preFacts) || !FactsComplete(cg.FM.GlobalFacts) ||
			!UnionFactsComplete(preUnion) || !UnionFactsComplete(cg.FM.UnionFacts) {
			// incomplete makeup must not invent SetMapFactsIn from cleared preFacts
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// map_facts_in pre-make full FactVec after makeup; map_facts_out pairs live
		cg.FM.SetMapFactsInPair(st.StmID, preFacts, preUnion)
		cg.FM.SetMapFactsOut(st.StmID, cg.FM.GlobalFacts)
		// Incomplete accum/stm effects fail closed (no invent MapAccumEffect/map fold success)
		acc := cg.AccumEffect()
		if !EffectComplete(acc) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		cg.FM.SetMapAccumEffect(st.StmID, acc)
		// residual ERROR sticky — no invent soft-append past SetMapAccum residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		if cg.FM.MapVisited == nil {
			cg.FM.MapVisited = make(map[int]bool)
		}
		cg.FM.MapVisited[st.StmID] = true
		// fold for effect into block
		be := cg.FM.GetMapStmEffect(b.StmID)
		stE := cg.FM.GetMapStmEffect(st.StmID)
		if !EffectComplete(be) || !EffectComplete(stE) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		merged := be.AddEffectSess(sessFromCG(cg), stE)
		// residual ERROR sticky — no invent soft-complete for fold past AddEffect residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		if !EffectComplete(merged) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		cg.FM.SetMapStmEffect(b.StmID, merged)
		cg.FM.SetMapAccumEffect(b.StmID, acc)
		// residual ERROR sticky — no invent soft-append past block SetMapAccum residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
	}
	return st
}

// AppendReturnStmt mirrors Block::append_return_stmt.
// Block.cpp:374–391 — make return, visit_facts, record fact/effect maps.
func (b *Block) AppendReturnStmt(r *Rng, opts Options, vs *VariableSelector, cg *CGContext) *Stmt {
	// Block.cpp:378+ — Statement::make_random(eReturn) always has RNG
	// no invent return shell without live block/context/RNG
	// non-sticky: MakeDummyBlock PostCreation passes nil RNG (soft fail append)
	if b == nil || r == nil || cg == nil {
		return nil
	}
	// incomplete ambient fails closed sticky before EffectStm clear (no invent soft re-pick)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	fm := cg.FM
	var preFacts []*FactPointTo
	var preUnion []*FactUnion
	if fm != nil {
		// incomplete GlobalFacts fail closed sticky (no invent cleaned pre-return snapshot)
		if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		preFacts = CloneFactSliceSess(sessFromCG(cg), fm.GlobalFacts)
		preUnion = CloneUnionFactSliceSess(sessFromCG(cg), fm.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(preUnion) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return nil
		}
	}
	cg.ClearEffectStm()
	// Block.cpp:378–380 — Statement::make_random(eReturn); ERROR_GUARD(nullptr)
	ret := MakeRandomReturn(r, opts, vs, cg)
	if !stmtOK(ret) {
		return nil
	}
	if StmIDUnset(ret.StmID) {
		ret.StmID = AllocStmIDSess(sessFromCG(cg))
	}
	b.Stmts = append(b.Stmts, ret)
	st := &b.Stmts[len(b.Stmts)-1]
	if fm != nil {
		// Block::stm_id always live when FM bound (no invent fold into key 0)
		if StmIDUnset(b.StmID) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Block.cpp:383 — makeup_new_var_facts(pre_facts, global_facts) full FactVec
		// Soft invent was PT-only MakeupNewVarFacts before set_fact_in.
		if !MakeupNewVarFactsSess(sessFromCG(cg), &preFacts, fm.GlobalFacts) ||
			!makeupNewUnionFactsSess(sessFromCG(cg), &preUnion, fm.UnionFacts) ||
			!FactsComplete(preFacts) || !FactsComplete(fm.GlobalFacts) ||
			!UnionFactsComplete(preUnion) || !UnionFactsComplete(fm.UnionFacts) {
			// incomplete makeup must not invent SetMapFactsIn from cleared preFacts
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Block.cpp:383–384 — sr->visit_facts; assert(visited)
		// no soft invent success / silent drop when visit fails
		if !VisitFactsStatementReturn(st, cg, opts) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			// C++ assert(visited) — sticky error for ERROR_GUARD callers
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Block.cpp:386–389 — set_fact_in; set_fact_out; accum; visited
		// set_fact_in full FactVec: pre point-to + pre eUnionWrite after makeup
		fm.SetMapFactsInPair(st.StmID, preFacts, preUnion)
		// set_fact_out filters function-locals for return (FactMgr.cpp:270–272)
		fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, b)
		// Incomplete accum/stm effects fail closed (no invent MapAccumEffect/map fold success)
		acc := cg.AccumEffect()
		if !EffectComplete(acc) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		fm.SetMapAccumEffect(st.StmID, acc)
		// residual ERROR sticky — no invent soft-return past SetMapAccum residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[st.StmID] = true
		// Block.cpp:391–392 — map_accum_effect[block]; map_stm_effect[block] += return
		be := fm.GetMapStmEffect(b.StmID)
		stE := fm.GetMapStmEffect(st.StmID)
		if !EffectComplete(be) || !EffectComplete(stE) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		merged := be.AddEffectSess(sessFromCG(cg), stE)
		// residual ERROR sticky — no invent soft-complete return fold past AddEffect residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		if !EffectComplete(merged) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		fm.SetMapStmEffect(b.StmID, merged)
		fm.SetMapAccumEffect(b.StmID, acc)
		// residual ERROR sticky — no invent soft-return past block SetMapAccum residual
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
		// FactMgr.cpp:268–270 — set_fact_out(Block*) with parent==nullptr filter
		fm.SetMapFactsOutForBlock(b, fm.GlobalFacts)
		if hasErrCG(cg) {
			b.Stmts = b.Stmts[:len(b.Stmts)-1]
			return nil
		}
	}
	return st
}

// ContainsBackEdge mirrors Block::contains_back_edge.
// Block.cpp:485–496 — CFG back_link whose dest->parent == this.
// CFGEdge* always live; nil hole sticky has-back (no invent none / soft re-pick).
// Nil FactMgr sticky has-back (no invent clean CFG without edges list).
func (b *Block) ContainsBackEdge(fm *FactMgr) bool {
	// Block always live; sticky incomplete no invent no-back-edge soft-skip
	if b == nil {
		noteErrFM(fm, ErrGeneric)
		return false
	}
	if fm == nil {
		// missing CFG sticky has-back (restrictive)
		noteErrFM(fm, ErrGeneric)
		return true
	}
	for _, e := range fm.CFGEdges {
		if e == nil {
			// incomplete CFGEdges sticky has-back
			noteErrFM(fm, ErrGeneric)
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

// blockHasStmtID reports whether b directly contains stm_id (no nest).
// Block always live; sticky false (no invent not-found soft-skip past hole).
func blockHasStmtIDSess(s *Session, b *Block, id int) bool {
	if b == nil {
		sessNoteError(s, ErrGeneric)
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
// Incomplete EffectAccum / GlobalFacts / post-creation fails closed nil
// (no invent dummy block success past hole shells).
func MakeDummyBlockCG(cg *CGContext, opts Options) *Block {
	// Block.cpp:96–97 — assert(curr_func) sticky
	if cg == nil || cg.CurrentFunc == nil {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky before stack push (no invent dummy past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	f := cg.CurrentFunc
	parent := cg.CurrentBlock()
	// Block.cpp:99 — Block(get_current_block(), 0)
	b := &Block{
		Parent:    parent,
		Func:      f,
		blockSize: 0,
		StmID:     AllocStmIDSess(sessFromCG(cg)),
		// Block.cpp:101 — in_array_loop from iv_bounds
		InArrayLoop: len(cg.IVBounds) > 0,
		EmitFM:      cg.FM,
	}
	// Block.cpp:102–103 — blocks + stack push
	f.Blocks = append(f.Blocks, b)
	f.Stack = append(f.Stack, b)
	// Block.cpp:95–110 — make_dummy never erases func->blocks (only stack pop at end).
	// Fail paths still leave the entry — same as make_random ERROR (Block.cpp:142–174).
	popStack := func() {
		if len(f.Stack) > 0 && f.Stack[len(f.Stack)-1] == b {
			f.Stack = f.Stack[:len(f.Stack)-1]
		}
	}
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		// pre-validated EffectComplete(*EffectAccum)
		preEffect = cg.EffectAccum.CloneSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-snapshot past IncompleteEffect Clone residual
		if hasErrCG(cg) {
			popStack()
			return nil
		}
	}
	if !EffectComplete(preEffect) {
		popStack()
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// Block.cpp:105 — set_fact_in(b, global_facts)
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			popStack()
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		cg.FM.SetMapFactsIn(b.StmID, cg.FM.GlobalFacts)
	}
	// Block.cpp:107 — post_creation_analysis
	b.PostCreationAnalysis(cg, opts, preEffect, nil, nil)
	// Block.cpp:108 — stack pop only (b stays on func->blocks)
	popStack()
	// incomplete post-creation must not invent dummy block success
	// Block.cpp:95–110 still returns b after post_creation; ERROR is caller-visible.
	// Keep b on Function.Blocks (no invent erase). Nil only when ERROR/incomplete.
	if hasErrCG(cg) || (cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return nil
	}
	return b
}

// AddNewVarFactTo mirrors FactMgr::add_new_var_fact into a fact vector.
// FactMgr.cpp:118–131 / Block.cpp:546–549 — abstract_fact_for_var_init only.
// No invent NewFactPointTo garbage when init abstract fails or InitExpr is the RHS.
// AddNewVarFactToSess is AddNewVarFactTo with explicit session residual sticky.
func AddNewVarFactToSess(s *Session, v *Variable, facts *[]*FactPointTo) {
	// same path as MakeupNewVarFacts / AddNewVarFactInto
	AddNewVarFactIntoSess(s, v, facts)
}

// ShortcutAnalysisBlock mirrors Statement::shortcut_analysis for a Block.
// Statement.cpp:545–567 — same_facts && !is_ctrl_stmt && !contains_unfixed_goto.
// Incomplete map_facts_out fails closed (ShortcutNone) — no invent reuse past holes.
// Missing map_facts_in/out keys: C++ std::map::operator[] default-inserts empty
// FactVec (Block.cpp post_creation find_fixed_point uses map_facts_in[this] before
// any set_fact_in). Soft invent treated missing as ShortcutNone → forced full
// re-visit of loop bodies, rewriting gen map_stm_effect and dropping nested-call
// make_iteration IV reads from caller feffect (seed-90: func_57 missing g_188 /
// g_262.f2). Fair: missing key = empty complete lattice (same as GetMapFactsIn).
// Block + facts + CGContext always live; sticky ShortcutNone
// (no invent soft-skip shortcut past hole).
// Nil FM / StmID 0 is non-sticky ShortcutNone (intentional reuse miss / soft re-pick).
func ShortcutAnalysisBlock(b *Block, facts *[]*FactPointTo, cg *CGContext) int {
	if b == nil || facts == nil || cg == nil {
		noteErrCG(cg, ErrGeneric)
		return ShortcutNone
	}
	if cg.FM == nil || StmIDUnset(b.StmID) {
		return ShortcutNone
	}
	fm := cg.FM
	// C++ map_facts_in[this] — missing key is empty FactVec, not "no shortcut".
	in := fm.GetMapFactsIn(b.StmID)
	// Fact* always live in maps; incomplete in/inputs fail closed
	if !FactsComplete(*facts) || !FactsComplete(in) {
		return ShortcutNone
	}
	// Statement.cpp:551 — same_facts full FactVec (ePointTo + eUnionWrite).
	// Live fm.UnionFacts paired with *facts by FindFixedPointBlock currentUnions.
	if !UnionFactsComplete(fm.UnionFacts) {
		return ShortcutNone
	}
	inU := fm.GetMapUnionFactsIn(b.StmID)
	if !UnionFactsComplete(inU) {
		return ShortcutNone
	}
	if !SameFactVecSess(sessFromCG(cg), *facts, fm.UnionFacts, in, inU) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past same_facts residual
		if hasErrCG(cg) {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past same_facts true path
	if hasErrCG(cg) {
		return ShortcutNone
	}
	// Statement.cpp:552 — !contains_unfixed_goto()
	if ContainsUnfixedGotoBlockSess(sessFromCG(cg), b, fm) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual true
		if hasErrCG(cg) {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual false
	if hasErrCG(cg) {
		return ShortcutNone
	}
	// block is not is_ctrl_stmt
	// Incomplete map_stm_effect / accum fails closed before AddEffect
	// (no invent ShortcutOK with poison; no sticky SetError on intentional none)
	eff := fm.GetMapStmEffect(b.StmID)
	if !EffectComplete(eff) {
		return ShortcutNone
	}
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		return ShortcutNone
	}
	if !EffectComplete(cg.EffectStm) {
		return ShortcutNone
	}
	if cg.InConflict(eff) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past InConflict residual true
		if hasErrCG(cg) {
			return ShortcutConflict
		}
		return ShortcutConflict
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past InConflict residual false
	if hasErrCG(cg) {
		return ShortcutConflict
	}
	// Statement.cpp:559 — inputs = map_facts_out[this]; C++ map[] empty if missing.
	// Full FactVec (ePointTo + eUnionWrite); soft invent was PT-only assign.
	out := fm.GetMapFactsOut(b.StmID)
	if !FactsComplete(out) {
		return ShortcutNone
	}
	outU := fm.GetMapUnionFactsOut(b.StmID)
	if !UnionFactsComplete(outU) {
		return ShortcutNone
	}
	*facts = CloneFactSliceSess(sessFromCG(cg), out)
	clU := CloneUnionFactSliceDeepSess(sessFromCG(cg), outU)
	if !UnionFactsComplete(clU) {
		return ShortcutNone
	}
	fm.UnionFacts = clU
	cg.AddEffect(eff, false)
	// residual ERROR sticky — no invent soft-continue ShortcutOK past AddEffect residual
	if hasErrCG(cg) {
		return ShortcutNone
	}
	if !EffectComplete(cg.EffectStm) {
		return ShortcutNone
	}
	acc := cg.AccumEffect()
	if !EffectComplete(acc) {
		return ShortcutNone
	}
	fm.SetMapAccumEffect(b.StmID, acc)
	// residual ERROR sticky — no invent ShortcutOK past SetMapAccum residual
	if hasErrCG(cg) {
		return ShortcutNone
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[b.StmID] = true
	return ShortcutOK
}
