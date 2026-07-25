// Upstream: Statement.h must_return / must_jump; StatementReturn/If/Block overrides.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustReturn mirrors Statement::must_return (default false; Return true; If both branches).
// Statement.h:187 — virtual default false (StatementFor / StatementArrayOp do NOT override).
// StatementReturn.h:60; StatementIf.cpp:200–201; Block.cpp:313–316.
// Incomplete get_blocks (nil if-arm) sticky false — C++ always has live Block refs;
// no invent "must_return" by soft-skipping a missing arm.
//
// For/ArrayOp: body may contain return, but Statement::must_return stays false, so
// Block::make_random continues siblings after a for/array-op (seed-2 e13830: Go used
// to stop the parent block when the for body returned → stack n=4 vs upstream n=5).
func (st Stmt) MustReturn() bool {
	return st.MustReturnSess(testAmbientSession)
}

func (st Stmt) MustReturnSess(s *Session) bool {
	switch st.Kind {
	case StmtReturn:
		return true
	case StmtIfElse:
		// StatementIf — if_true.must_return() && if_false.must_return()
		blks := GetBlocksStmtSess(s, &st)
		if len(blks) != 2 || blks[0] == nil || blks[1] == nil {
			// incomplete arms sticky not-must-return (no invent soft-skip missing arm)
			sessNoteError(s, ErrGeneric)
			return false
		}
		if !blks[0].MustReturnSess(s) {
			// residual ERROR sticky — no invent soft-continue false-arm past Then residual false
			if sessHasError(s) {
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent soft-continue false-arm past Then residual true
		if sessHasError(s) {
			return false
		}
		ok := blks[1].MustReturnSess(s)
		// residual ERROR sticky — no invent must-return true past Else residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	default:
		// Statement.h:187 — includes StmtFor / StmtArrayOp (no override in C++)
		return false
	}
}

// MustJump mirrors Statement::must_jump — transfer of control always taken.
// Break/Continue/Goto: true only when test.not_equals(0); variable tests → false.
// StatementIf: both branches must_jump; Block: last stm.
// Incomplete get_blocks sticky false (no invent not-must-jump soft re-pick past holes).}

func (st Stmt) MustJump() bool {
	return st.MustJumpSess(testAmbientSession)
}

func (st Stmt) MustJumpSess(s *Session) bool {
	if st.MustReturnSess(s) {
		// residual ERROR sticky — no invent must-jump true past MustReturn residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue jump kinds past MustReturn residual false
	if sessHasError(s) {
		return false
	}
	switch st.Kind {
	case StmtBreak, StmtContinue, StmtGoto:
		// Expression::not_equals(0) — Constant only; other terms false
		// incomplete test Expr sticky not-must-jump (no invent always-jump)
		if st.Expr == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := st.Expr.NotEqualsSess(s, 0)
		// residual ERROR sticky — no invent must-jump true past NotEquals residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case StmtIfElse:
		blks := GetBlocksStmtSess(s, &st)
		if len(blks) != 2 || blks[0] == nil || blks[1] == nil {
			// StatementIf always both arms; incomplete sticky not-must-jump
			sessNoteError(s, ErrGeneric)
			return false
		}
		if !blks[0].MustJumpSess(s) {
			// residual ERROR sticky — no invent soft-continue false-arm past Then residual false
			if sessHasError(s) {
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent soft-continue false-arm past Then residual true
		if sessHasError(s) {
			return false
		}
		ok := blks[1].MustJumpSess(s)
		// residual ERROR sticky — no invent must-jump true past Else residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	default:
		// Statement.h:189 — includes StmtFor / StmtArrayOp (no override in C++)
		return false
	}
}

// MustReturn mirrors Block::must_return.
// Block.cpp:313–331 — last must_return, no break_stms, no escape via back edges.
// Uses b.EmitFM for CFG when set; prefer MustReturnWithFM during DFA.
// Block always live; sticky false (no invent not-must-return soft-skip past hole).

func (b *Block) MustReturn() bool {
	return b.MustReturnSess(testAmbientSession)
}

// MustReturnSess is MustReturn with explicit session residual sticky.
func (b *Block) MustReturnSess(s *Session) bool {
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return b.MustReturnWithFMSess(s, b.EmitFM)
}

// MustReturnWithFM is must_return with an explicit FactMgr for back-edge checks.
// Block always live; sticky false (no invent not-must-return soft-skip past hole).
// Non-Sess MustReturnWithFM deleted — pass run bag or testAmbientSession explicitly.

// MustReturnWithFMSess is MustReturnWithFM with explicit session residual sticky.
func (b *Block) MustReturnWithFMSess(s *Session, fm *FactMgr) bool {
	if b == nil {
		if s == nil {
			s = sessFromFM(fm)
		}
		sessNoteError(s, ErrGeneric)
		return false
	}
	if s == nil {
		s = sessFromFM(fm)
	}
	if len(b.Stmts) == 0 {
		return false
	}
	// StatementBreak.cpp push → break_stms; any break means can leave without return
	if len(b.BreakStmIDs) > 0 {
		return false
	}
	last := b.GetLastStmSess(s)
	if last == nil || !last.MustReturnSess(s) {
		// residual ERROR sticky — no invent not-must-return soft-skip past MustReturn residual
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue escape check past MustReturn residual true
	if sessHasError(s) {
		return false
	}
	// Block.cpp:318–326 — back edges into block (continue) can skip end return
	esc := b.hasEscapeBackEdge(fm)
	// residual ERROR sticky — no invent must-return true past escape CFG residual hole
	if sessHasError(s) {
		return false
	}
	return !esc
}

// MustJump mirrors Block::must_jump.
// Block.cpp:336–341 — last must_jump and break_stms empty.
// Block always live; sticky false (no invent not-must-jump soft-skip past hole).
func (b *Block) MustJump() bool {
	return b.MustJumpSess(testAmbientSession)
}

// MustJumpSess is MustJump with explicit session residual sticky.
func (b *Block) MustJumpSess(s *Session) bool {
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if len(b.Stmts) == 0 {
		return false
	}
	if len(b.BreakStmIDs) > 0 {
		return false
	}
	last := b.GetLastStmSess(s)
	if last == nil {
		return false
	}
	ok := last.MustJumpSess(s)
	// residual ERROR sticky — no invent must-jump true past last MustJump residual hole
	if sessHasError(s) {
		return false
	}
	return ok
}

// hasEscapeBackEdge reports a back_link edge into b whose src is not the block itself.
// Block.cpp:318–326 / 346–353 — Statement::find_edges_in(this, false, true) then
// any edge with src != this. C++ matches e->dest == this (Block* as Statement*):
// continue / self-back use dest=Block*; goto uses dest=labeled Statement* and must
// not count (seed-79: DestBlock bookkeeping on goto falsely escaped → double return).
// Incomplete CFG sticky possible escape — no invent "no edge" soft re-pick.
// Nil FactMgr: no edges known (C++ find_edges_in empty) → no escape (not invent escape).
func (b *Block) hasEscapeBackEdge(fm *FactMgr) bool {
	if b == nil || fm == nil {
		return false
	}
	// Block::stm_id always live for CFG-indexed edges; StmID 0 is valid (fair sid).
	// IncompleteStmID sticky possible escape (no invent "no back edge").
	if StmIDUnset(b.StmID) {
		noteErrFM(fm, ErrGeneric)
		return true
	}
	// Statement.cpp:453–467 — e->dest == this only (DestStmID == block.StmID).
	// Do not use FindEdgesInToBlock: goto CreateCFGEdgeTo stores DestBlock=parent
	// for bookkeeping while DestStmID is the label stmt (StatementGoto.cpp:139).
	back := fm.FindEdgesIn(b.StmID, false, true)
	// incomplete CFG sticky possible escape
	if back == nil {
		if !hasErrFM(fm) {
			noteErrFM(fm, ErrGeneric)
		}
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
// Incomplete Function/ReturnType sticky true (no invent "no return needed" past holes).
// Non-Sess NeedReturnStmt deleted — pass run bag or testAmbientSession explicitly.

// NeedReturnStmtSess is NeedReturnStmt with explicit session residual sticky.
func (f *Function) NeedReturnStmtSess(s *Session) bool {
	// Function always live; sticky incomplete need-return (restrictive)
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.ReturnType == nil {
		// incomplete return type sticky need return (no invent void soft-skip)
		sessNoteError(s, ErrGeneric)
		return true
	}
	simple := f.ReturnType.IsSimpleSess(s)
	// residual ERROR sticky — no invent soft need-return past IsSimple residual
	if sessHasError(s) {
		return true
	}
	return !(simple && f.ReturnType.SimpleSess(s) == EVoid)
}
