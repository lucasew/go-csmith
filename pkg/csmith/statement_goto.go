// Upstream: StatementGoto.cpp (make_random) with back/forward edge selection.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// goodGotoTarget reports statements that may receive a label (not jump/return-ish).
// StatementGoto.cpp:99–109 — disallow break/continue/goto/return as targets.
func goodGotoTarget(st Stmt) bool {
	switch st.Kind {
	case StmtReturn, StmtBreak, StmtContinue, StmtGoto:
		return false
	default:
		return true
	}
}

// HasInitSkippedVars mirrors StatementGoto::has_init_skipped_vars.
// StatementGoto.cpp:281–306 — jump into/out would skip locals of intermediate blocks.
func HasInitSkippedVars(src *Block, destParent *Block) bool {
	if destParent == nil {
		return false
	}
	// walk dest ancestors until src; any local not visible in src → skipped
	for b := destParent; b != nil && b != src; b = b.Parent {
		for _, v := range b.LocalVars {
			if v == nil {
				continue
			}
			if src == nil || !v.IsVisibleLocal(src) {
				return true
			}
		}
	}
	return false
}

// FindGoodJumpBlock mirrors StatementGoto::find_good_jump_block.
// StatementGoto.cpp:309–354 — pick a block suitable as jump source/dest.
// asDest true: block is jump destination; false: block is jump source (contains goto).
// Mutates blocks slice by removing bad candidates (caller should pass a copy).
func FindGoodJumpBlock(r *Rng, blocks []*Block, curr *Block, asDest bool) *Block {
	if r == nil || len(blocks) == 0 {
		return nil
	}
	// StatementGoto.cpp:314–320
	if curr != nil {
		if curr.InArrayLoop && !asDest {
			return nil
		}
		if len(curr.Stmts) == 0 && !asDest {
			return nil
		}
		if asDest {
			if last := curr.GetLastStm(); last != nil && last.MustReturn() {
				return nil
			}
		}
	}
	// work on a mutable copy
	blks := append([]*Block(nil), blocks...)
	for len(blks) > 0 {
		idx := int(r.RndUpto(uint32(len(blks))))
		b := blks[idx]
		if b == nil {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		// disallow array-loop dest
		if b.InArrayLoop && asDest {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		if len(b.Stmts) == 0 {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		// sole return cannot be jump source
		if !asDest && len(b.Stmts) == 1 && b.Stmts[0].Kind == StmtReturn {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		if b == curr {
			return b
		}
		// skipped locals between blocks
		if asDest {
			// jump from curr into b
			if curr != nil && len(b.Stmts) > 0 && HasInitSkippedVars(curr, b) {
				blks = append(blks[:idx], blks[idx+1:]...)
				continue
			}
		} else {
			// jump from b into curr
			if curr != nil && len(curr.Stmts) > 0 && HasInitSkippedVars(b, curr) {
				blks = append(blks[:idx], blks[idx+1:]...)
				continue
			}
		}
		return b
	}
	return nil
}

// MakeRandomGoto mirrors StatementGoto::make_random.
// StatementGoto.cpp:61+ — find_good_jump_block; choose_visible_read_var; cfg_edge.
func MakeRandomGoto(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	blk *Block,
) Stmt {
	_ = opts
	_ = probs
	if r == nil || cg.CurrentFunc == nil {
		return Stmt{Kind: StmtGoto}
	}

	// clear per-stmt effect (StatementGoto.cpp:112)
	cg.ClearEffectStm()

	// 40% prefer back-edge (StatementGoto.cpp:76)
	wantBack := r.RndFlipcoin(40)
	blocks := append([]*Block(nil), cg.CurrentFunc.Blocks...)
	// include current if not yet in Blocks list
	if blk != nil {
		found := false
		for _, b := range blocks {
			if b == blk {
				found = true
				break
			}
		}
		if !found {
			blocks = append(blocks, blk)
		}
	}

	var okBlk *Block
	backEdge := false
	if wantBack {
		// as_dest=true: pick destination for backward jump
		okBlk = FindGoodJumpBlock(r, blocks, blk, true)
		if okBlk != nil {
			backEdge = true
		}
	}
	if okBlk == nil {
		backEdge = false
		blocks = append([]*Block(nil), cg.CurrentFunc.Blocks...)
		if blk != nil {
			found := false
			for _, b := range blocks {
				if b == blk {
					found = true
					break
				}
			}
			if !found {
				blocks = append(blocks, blk)
			}
		}
		// forward: dest block for forward jump; as_dest true for dest selection
		// When back failed, pick dest for forward (as_dest true)
		okBlk = FindGoodJumpBlock(r, blocks, blk, true)
	}
	if okBlk == nil {
		// fallback: forward label-after-goto in current block (legacy path)
		return makeForwardGotoOnly(r, opts, vs, tables, cg, blk)
	}

	// pick a good statement in okBlk
	var okStms []int
	for i, s := range okBlk.Stmts {
		if !goodGotoTarget(s) || s.MustReturn() {
			continue
		}
		// skip "current" last stmt identity loosely: same block last
		if blk == okBlk && i == len(okBlk.Stmts)-1 {
			continue
		}
		okStms = append(okStms, i)
	}
	if len(okStms) == 0 {
		return makeForwardGotoOnly(r, opts, vs, tables, cg, blk)
	}
	ti := okStms[r.RndUpto(uint32(len(okStms)))]
	tgt := &okBlk.Stmts[ti]

	// condition: prefer already-read visible var
	var cond *Expression
	if cg.EffectAccum != nil {
		var uf []*FactUnion
		if cg.FM != nil {
			uf = cg.FM.UnionFacts
		}
		if v := ChooseVisibleReadVar(r, blk, cg.EffectAccum.ReadVars(), GetIntType(), uf); v != nil {
			cg.NoteRead(v)
			cond = &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
		}
	}
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
		if cond == nil {
			cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}

	label := "lbl_1"
	if vs != nil {
		label = vs.Sym.Next("lbl_")
	}
	if tgt.SourceLabel == "" {
		tgt.SourceLabel = label
	} else {
		label = tgt.SourceLabel
	}
	if tgt.StmID == 0 {
		tgt.StmID = AllocStmID()
	}

	st := Stmt{Kind: StmtGoto, Expr: cond, Label: label, StmID: AllocStmID()}
	if backEdge {
		st.GotoBack = true
		// StatementGoto.cpp:139 — create_cfg_edge(sg, other_stm, false, true)
		if cg.FM != nil {
			cg.FM.CreateCFGEdgeTo(st.StmID, okBlk, tgt.StmID, false, true)
		}
		// StatementGoto.cpp:149 — Bookkeeper::backward_jump_cnt++
		RecordBackwardJump()
	} else {
		st.GotoForward = true
		// StatementGoto.cpp:203 — create_cfg_edge(sg, stm, false, false)
		if cg.FM != nil {
			cg.FM.CreateCFGEdgeTo(st.StmID, okBlk, tgt.StmID, false, false)
		}
		// StatementGoto.cpp:211 — Bookkeeper::forward_jump_cnt++
		RecordForwardJump()
	}
	return st
}

// makeForwardGotoOnly is the fall-back: label placed after goto in current block.
func makeForwardGotoOnly(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	blk *Block,
) Stmt {
	cond := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
	}
	label := "lbl_1"
	if vs != nil {
		label = vs.Sym.Next("lbl_")
	}
	st := Stmt{Kind: StmtGoto, Expr: cond, Label: label, GotoForward: true, StmID: AllocStmID()}
	if cg.FM != nil && blk != nil {
		cg.FM.CreateCFGEdgeTo(st.StmID, blk, 0, false, false)
	}
	// StatementGoto.cpp:211 — forward_jump_cnt
	RecordForwardJump()
	return st
}
