// Upstream: Statement.h must_return / must_jump; StatementReturn/If/Block overrides.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustReturn mirrors Statement::must_return (default false; Return true; If both branches).
// Statement.h:187; StatementReturn.h:60; StatementIf.cpp:200–201; Block.cpp:313–316.
// Incomplete get_blocks (nil if-arm / for body) fails closed false — C++ always has
// live Block refs; no invent "must_return" by soft-skipping a missing arm.
func (st Stmt) MustReturn() bool {
	switch st.Kind {
	case StmtReturn:
		return true
	case StmtIfElse:
		// StatementIf — if_true.must_return() && if_false.must_return()
		blks := GetBlocksStmt(&st)
		if len(blks) != 2 || blks[0] == nil || blks[1] == nil {
			return false
		}
		return blks[0].MustReturn() && blks[1].MustReturn()
	case StmtFor, StmtArrayOp:
		// for only if body always returns (StatementFor.cpp visit path)
		blks := GetBlocksStmt(&st)
		if len(blks) == 0 || blks[0] == nil {
			return false
		}
		return blks[0].MustReturn()
	default:
		return false
	}
}

// MustJump mirrors Statement::must_jump — transfer of control always taken.
// Break/Continue/Goto: true only when test.not_equals(0); variable tests → false.
// StatementIf: both branches must_jump; Block: last stm.
// Incomplete get_blocks fails closed false (same as MustReturn).
func (st Stmt) MustJump() bool {
	if st.MustReturn() {
		return true
	}
	switch st.Kind {
	case StmtBreak, StmtContinue, StmtGoto:
		// Expression::not_equals(0) — Constant only; other terms false
		return st.Expr != nil && st.Expr.NotEquals(0)
	case StmtIfElse:
		blks := GetBlocksStmt(&st)
		if len(blks) != 2 || blks[0] == nil || blks[1] == nil {
			return false
		}
		return blks[0].MustJump() && blks[1].MustJump()
	case StmtFor, StmtArrayOp:
		blks := GetBlocksStmt(&st)
		if len(blks) == 0 || blks[0] == nil {
			return false
		}
		return blks[0].MustJump()
	default:
		return false
	}
}

// MustReturn mirrors Block::must_return.
// Block.cpp:313–331 — last must_return, no break_stms, no escape via back edges.
// Uses b.EmitFM for CFG when set; prefer MustReturnWithFM during DFA.
func (b *Block) MustReturn() bool {
	return b.MustReturnWithFM(b.EmitFM)
}

// MustReturnWithFM is must_return with an explicit FactMgr for back-edge checks.
func (b *Block) MustReturnWithFM(fm *FactMgr) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	// StatementBreak.cpp push → break_stms; any break means can leave without return
	if len(b.BreakStmIDs) > 0 {
		return false
	}
	last := b.GetLastStm()
	if last == nil || !last.MustReturn() {
		return false
	}
	// Block.cpp:318–326 — back edges into block (continue) can skip end return
	return !b.hasEscapeBackEdge(fm)
}

// MustJump mirrors Block::must_jump.
// Block.cpp:336–341 — last must_jump and break_stms empty.
func (b *Block) MustJump() bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	if len(b.BreakStmIDs) > 0 {
		return false
	}
	last := b.GetLastStm()
	return last != nil && last.MustJump()
}

// hasEscapeBackEdge reports a back_link edge into b whose src is not the block itself.
// Block.cpp:318–326 / 346–353 — continue into loop body can bypass end return.
// Incomplete CFG (nil hole) fails closed as possible escape — no invent "no edge".
// Nil FactMgr: no edges known (C++ find_edges_in empty) → no escape (not invent escape).
func (b *Block) hasEscapeBackEdge(fm *FactMgr) bool {
	if b == nil || fm == nil {
		return false
	}
	// Block::stm_id always live for CFG-indexed edges; StmID 0 fails closed
	// as possible escape (no invent "no back edge" soft-skipping FindEdgesIn)
	if b.StmID <= 0 {
		return true
	}
	// edges targeting the block as DestBlock
	toBlk := fm.FindEdgesInToBlock(b, false, true)
	if toBlk == nil {
		return true
	}
	for _, e := range toBlk {
		if e.SrcID != b.StmID {
			return true
		}
	}
	// edges targeting block stm_id
	back := fm.FindEdgesIn(b.StmID, false, true)
	if back == nil {
		return true
	}
	for _, e := range back {
		if e.SrcID != b.StmID {
			return true
		}
	}
	return false
}

// NeedReturnStmt mirrors Function::need_return_stmt.
// Function.cpp:618–619 — return_type always live; void simple → false.
// Nil ReturnType fails closed true (no invent "no return needed" for incomplete IR).
func (f *Function) NeedReturnStmt() bool {
	if f == nil {
		return false
	}
	if f.ReturnType == nil {
		return true
	}
	return !(f.ReturnType.IsSimple() && f.ReturnType.Simple() == EVoid)
}
