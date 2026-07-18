// Upstream: Statement.h must_return / must_jump; StatementReturn/If/Block overrides.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// MustReturn mirrors Statement::must_return (default false; Return true; If both branches).
// Statement.h:187; StatementReturn.h:60; StatementIf.cpp:200–201; Block.cpp:313–316.
func (st Stmt) MustReturn() bool {
	switch st.Kind {
	case StmtReturn:
		return true
	case StmtIfElse:
		t := st.Then != nil && st.Then.MustReturn()
		e := st.Else != nil && st.Else.MustReturn()
		return t && e
	case StmtFor, StmtArrayOp:
		// for only if body always returns (StatementFor.cpp visit path)
		return st.Then != nil && st.Then.MustReturn()
	default:
		return false
	}
}

// MustJump mirrors Statement::must_jump — transfer of control always taken.
// Break/Continue/Goto: true only when test.not_equals(0); variable tests → false.
// StatementIf: both branches must_jump; Block: last stm.
func (st Stmt) MustJump() bool {
	if st.MustReturn() {
		return true
	}
	switch st.Kind {
	case StmtBreak, StmtContinue, StmtGoto:
		// Expression::not_equals(0) — Constant only; other terms false
		return st.Expr != nil && st.Expr.NotEquals(0)
	case StmtIfElse:
		t := st.Then != nil && st.Then.MustJump()
		e := st.Else != nil && st.Else.MustJump()
		return t && e
	case StmtFor, StmtArrayOp:
		return st.Then != nil && st.Then.MustJump()
	default:
		return false
	}
}

// MustReturn mirrors Block::must_return — last statement must return.
// Block.cpp:313–316.
func (b *Block) MustReturn() bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	// skip trailing StmtLabel markers
	for i := len(b.Stmts) - 1; i >= 0; i-- {
		if b.Stmts[i].Kind == StmtLabel {
			continue
		}
		return b.Stmts[i].MustReturn()
	}
	return false
}

// MustJump mirrors Block::must_jump.
// Block.cpp:334–337.
func (b *Block) MustJump() bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	for i := len(b.Stmts) - 1; i >= 0; i-- {
		if b.Stmts[i].Kind == StmtLabel {
			continue
		}
		return b.Stmts[i].MustJump()
	}
	return false
}

// NeedReturnStmt mirrors Function::need_return_stmt.
// Function.cpp:618–619.
func (f *Function) NeedReturnStmt() bool {
	if f == nil || f.ReturnType == nil {
		return false
	}
	return !(f.ReturnType.IsSimple() && f.ReturnType.Simple() == EVoid)
}
