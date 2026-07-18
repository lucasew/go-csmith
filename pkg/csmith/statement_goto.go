// Upstream: StatementGoto.cpp (make_random) with back/forward edge selection.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// stmLabels mirrors StatementGoto::stm_labels — dest statement → shared label.
// StatementGoto.cpp:55, 224–229.
var stmLabels = map[int]string{}

// LabelForGotoDest returns existing or new gensym label for a jump destination.
// StatementGoto.cpp:224–229 — reuse stm_labels[dest] when present.
func LabelForGotoDest(destStmID int, nextLabel func() string) string {
	if destStmID > 0 {
		if lab, ok := stmLabels[destStmID]; ok && lab != "" {
			return lab
		}
	}
	lab := "lbl_1"
	if nextLabel != nil {
		lab = nextLabel()
	}
	if destStmID > 0 {
		stmLabels[destStmID] = lab
	}
	return lab
}

// GotoLabelsDoFinalization mirrors StatementGoto::doFinalization.
// StatementGoto.cpp:404 — stm_labels.clear().
func GotoLabelsDoFinalization() {
	stmLabels = map[int]string{}
}

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

// ContainsStmt reports whether b (or nested Then/Else blocks) holds st.
// Block::contains_stmt light — used for NeedRevisit LCA on back-edge goto.
func (b *Block) ContainsStmt(st *Stmt) bool {
	if b == nil || st == nil {
		return false
	}
	for i := range b.Stmts {
		s := &b.Stmts[i]
		if s == st {
			return true
		}
		if st.StmID > 0 && s.StmID == st.StmID {
			return true
		}
		if s.Then != nil && s.Then.ContainsStmt(st) {
			return true
		}
		if s.Else != nil && s.Else.ContainsStmt(st) {
			return true
		}
	}
	return false
}

// MarkNeedRevisitLCA sets NeedRevisit on the least incomplete ancestor of curr
// that contains dest (StatementGoto.cpp:141–147).
func MarkNeedRevisitLCA(curr *Block, dest *Stmt) {
	for b := curr; b != nil; b = b.Parent {
		if b.ContainsStmt(dest) {
			b.NeedRevisit = true
			return
		}
	}
	if curr != nil {
		curr.NeedRevisit = true
	}
}

// HasInitSkippedVars mirrors StatementGoto::has_init_skipped_vars.
// StatementGoto.cpp:281–306 — jump into/out would skip locals of intermediate blocks.
func HasInitSkippedVars(src *Block, destParent *Block) bool {
	return len(CollectInitSkippedVars(src, destParent)) > 0
}

// CollectInitSkippedVars collects locals whose initialization is skipped by a jump.
// StatementGoto.cpp:281–306 — walk dest parent chain until src.
func CollectInitSkippedVars(src *Block, destParent *Block) []*Variable {
	if destParent == nil {
		return nil
	}
	var skipped []*Variable
	// walk dest ancestors until src
	for b := destParent; b != nil && b != src; b = b.Parent {
		for _, v := range b.LocalVars {
			if v == nil {
				continue
			}
			// b==src handled by loop exit; all locals in intermediate blocks skipped
			// when not visible in src
			if src == nil || !v.IsVisibleLocal(src) {
				skipped = append(skipped, v)
			}
		}
	}
	return skipped
}

// OutputSkippedVarInits mirrors StatementGoto::output_skipped_var_inits.
// StatementGoto.cpp:264–275 — re-init skipped locals at destination label.
func OutputSkippedVarInits(st *Stmt, indent string) string {
	if st == nil || len(st.InitSkippedVars) == 0 {
		return ""
	}
	var b strings.Builder
	for _, v := range st.InitSkippedVars {
		if v == nil {
			continue
		}
		b.WriteString(indent)
		b.WriteString(v.GetActualName(false))
		b.WriteString(" = ")
		if v.Init != nil {
			b.WriteString(v.Init.Value)
		} else {
			b.WriteString("0")
		}
		b.WriteString(";\n")
	}
	return b.String()
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
	if tgt.StmID == 0 {
		tgt.StmID = AllocStmID()
	}

	// condition: prefer already-read visible var (StatementGoto.cpp:117–132)
	// back: curr_blk + effect_accum reads; forward: ok_blk + map_accum_effect[dest]
	var cond *Expression
	var uf []*FactUnion
	if cg.FM != nil {
		uf = cg.FM.UnionFacts
	}
	var readVars []*Variable
	condBlk := blk
	if backEdge {
		// StatementGoto.cpp:119–122
		if cg.EffectAccum != nil {
			readVars = cg.EffectAccum.ReadVars()
		}
	} else {
		// StatementGoto.cpp:125–128 — travel in time to other_stm accum effect
		condBlk = okBlk
		if cg.FM != nil {
			if acc, ok := cg.FM.MapAccumEffect[tgt.StmID]; ok {
				readVars = acc.ReadVars()
			}
			// prefer union facts from dest out when available
			if out, ok := cg.FM.MapFactsOut[tgt.StmID]; ok && len(out) > 0 {
				// map_facts_out used for nonreadable filter via uf only; point-to list not required here
				_ = out
			}
		}
		if len(readVars) == 0 && cg.EffectAccum != nil {
			readVars = cg.EffectAccum.ReadVars()
		}
	}
	if len(readVars) > 0 {
		if v := ChooseVisibleReadVar(r, condBlk, readVars, GetIntType(), uf); v != nil {
			cg.NoteRead(v)
			cond = &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
		}
	}
	// C++ returns nullptr when cond_var missing; we soft-fallback for library generation
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
		if cond == nil {
			cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}

	// StatementGoto.cpp:224–229 — stm_labels[dest]
	nextLab := func() string {
		if vs != nil {
			return vs.Sym.Next("lbl_")
		}
		return "lbl_1"
	}
	label := tgt.SourceLabel
	if label == "" {
		label = LabelForGotoDest(tgt.StmID, nextLab)
		tgt.SourceLabel = label
	} else {
		// keep SourceLabel; ensure registry knows it
		if tgt.StmID > 0 {
			stmLabels[tgt.StmID] = label
		}
	}

	st := Stmt{
		Kind: StmtGoto, Expr: cond, Label: label, StmID: AllocStmID(),
		GotoDestStmID:  tgt.StmID,
		GotoDestParent: okBlk,
	}
	if backEdge {
		st.GotoBack = true
		// jump from curr (blk) into okBlk → collect skipped locals
		st.InitSkippedVars = CollectInitSkippedVars(blk, okBlk)
		// StatementGoto.cpp:139 — create_cfg_edge(sg, other_stm, false, true)
		if cg.FM != nil {
			cg.FM.CreateCFGEdgeTo(st.StmID, okBlk, tgt.StmID, false, true)
		}
		// StatementGoto.cpp:141–147 — LCA need_revisit
		MarkNeedRevisitLCA(blk, tgt)
		// StatementGoto.cpp:149 — Bookkeeper::backward_jump_cnt++
		RecordBackwardJump()
	} else {
		st.GotoForward = true
		// jump from okBlk into curr (blk)
		st.InitSkippedVars = CollectInitSkippedVars(okBlk, blk)
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
