// Upstream: Expression/Statement get_referenced_ptrs; Function::compute_summary.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectReferencedPtrsExpression mirrors Expression::get_referenced_ptrs.
// ExpressionVariable.cpp:230–235 — pointer vars; comma/assign recurse; invoke args + callee.
// Incomplete IR fails closed sticky: *ptrs → IncompleteVariables (not bare nil —
// VariablesComplete(nil)/len==0 invents empty-complete ptr list / soft re-pick past hole).
// ptrs always live; sticky (no invent soft-skip collect past hole).
func CollectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) {
	if ptrs == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectReferencedPtrsExpression(e, ptrs) {
		*ptrs = IncompleteVariables()
		SetError(ErrGeneric)
	}
}

// collectReferencedPtrsExpression returns false on incomplete IR.
// On failure *ptrs is IncompleteVariables (caller may also overwrite).
func collectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) bool {
	if e == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		// Expression always live for get_referenced_ptrs; sticky incomplete
		SetError(ErrGeneric)
		return false
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		// Type* always live; Type-nil non-special sticky incomplete
		// (no invent complete no-ptrs soft-success past type hole via IsPointer false)
		if e.Var == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if e.Var.Type == nil && !IsSpecialPtr(e.Var) {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if e.Var.IsPointer() {
			// residual ERROR sticky — no invent complete no-ptrs past IsPointer hole
			if HasError() {
				*ptrs = IncompleteVariables()
				return false
			}
			*ptrs = appendUniqueVar(*ptrs, e.Var)
		} else if HasError() {
			// residual ERROR sticky — no invent complete no-ptrs soft-skip past IsPointer hole
			*ptrs = IncompleteVariables()
			return false
		}
		return true
	case TermCommaExpr:
		// both sides always live
		if e.CommaLHS == nil || e.CommaRHS == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if !collectReferencedPtrsExpression(e.CommaLHS, ptrs) {
			return false
		}
		return collectReferencedPtrsExpression(e.CommaRHS, ptrs)
	case TermAssignment:
		if e.Assign == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		return collectReferencedPtrsStmt(e.Assign, ptrs)
	case TermFunction:
		// ExpressionFuncall.cpp:165–177 — param_value then eFuncCall → assert(fiu) + callee
		// Invoke always live for TermFunction; sticky incomplete (public Collect also sticks)
		if e.Invoke == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		for _, a := range e.Invoke.Args {
			// Expression* args always live after ERROR_GUARD
			if a == nil {
				*ptrs = IncompleteVariables()
				SetError(ErrGeneric)
				return false
			}
			if !collectReferencedPtrsExpression(a, ptrs) {
				return false
			}
		}
		// ExpressionFuncall.cpp:172–177 — only user FuncCall walks callee refs
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			// incomplete callee ReferencedPtrs sticky (no invent skip hole as empty refs)
			if !VariablesComplete(e.Invoke.User.ReferencedPtrs) {
				*ptrs = IncompleteVariables()
				SetError(ErrGeneric)
				return false
			}
			for _, p := range e.Invoke.User.ReferencedPtrs {
				*ptrs = appendUniqueVar(*ptrs, p)
			}
		}
		return true
	case TermConstant:
		return true
	default:
		// unknown term — incomplete IR sticky
		*ptrs = IncompleteVariables()
		SetError(ErrGeneric)
		return false
	}
}

// CollectReferencedPtrsStmt mirrors Statement::get_referenced_ptrs.
// Statement.cpp:331–345 — exprs + nested blocks.
// Incomplete IR → sticky IncompleteVariables (not bare nil invent empty-complete).
// ptrs always live; sticky (no invent soft-skip collect past hole).
func CollectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) {
	if ptrs == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectReferencedPtrsStmt(st, ptrs) {
		*ptrs = IncompleteVariables()
		SetError(ErrGeneric)
	}
}

func collectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) bool {
	if st == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		// Statement always live for get_referenced_ptrs; sticky incomplete
		SetError(ErrGeneric)
		return false
	}
	// Statement.cpp:331–345 — get_exprs + get_blocks; get_exprs always live for
	// assign/invoke/return/if/break/continue/goto and for-test.
	// Kind-gated for (no invent Loop-on-wrong-kind as for get_exprs).
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		// Incomplete Loop without TestExpr sticky (no invent skip for-test ptrs)
		if st.Loop == nil || st.Loop.TestExpr == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if !collectReferencedPtrsExpression(st.Loop.TestExpr, ptrs) {
			return false
		}
	} else {
		switch st.Kind {
		case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
			// C++ get_exprs always yields live Expression* for these kinds
			// incomplete nil Expr sticky (no invent partial ptr list as success)
			if st.Expr == nil {
				*ptrs = IncompleteVariables()
				SetError(ErrGeneric)
				return false
			}
			if !collectReferencedPtrsExpression(st.Expr, ptrs) {
				return false
			}
		default:
			if st.Expr != nil {
				if !collectReferencedPtrsExpression(st.Expr, ptrs) {
					return false
				}
			}
		}
	}
	if st.LhsVar != nil {
		// Type* always live; Type-nil non-special sticky (no invent complete no-ptrs)
		if st.LhsVar.Type == nil && !IsSpecialPtr(st.LhsVar) {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if st.LhsVar.IsPointer() {
			if HasError() {
				*ptrs = IncompleteVariables()
				return false
			}
			*ptrs = appendUniqueVar(*ptrs, st.LhsVar)
		} else if HasError() {
			*ptrs = IncompleteVariables()
			return false
		}
	}
	if st.Lhs != nil {
		// Lhs always has live Var in C++; incomplete Lhs.Var sticky
		if st.Lhs.Var == nil {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		// Type-nil non-special sticky (no invent complete no-ptrs via IsPointer false)
		if st.Lhs.Var.Type == nil && !IsSpecialPtr(st.Lhs.Var) {
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
		if st.Lhs.Var.IsPointer() {
			if HasError() {
				*ptrs = IncompleteVariables()
				return false
			}
			*ptrs = appendUniqueVar(*ptrs, st.Lhs.Var)
		} else if HasError() {
			*ptrs = IncompleteVariables()
			return false
		}
	}
	// get_blocks only — no invent ptrs via stray Then on assign
	blks := GetBlocksStmt(st)
	for _, b := range blks {
		if b == nil {
			// incomplete arm sticky (no invent partial ptr list past hole)
			*ptrs = IncompleteVariables()
			SetError(ErrGeneric)
			return false
		}
	}
	for _, b := range blks {
		if !collectReferencedPtrsBlock(b, ptrs) {
			return false
		}
	}
	return true
}

// CollectReferencedPtrsBlock walks all statements in a block.
// Incomplete IR → sticky IncompleteVariables (not bare nil invent empty-complete).
// CollectReferencedPtrsBlock walks all statements in a block for referenced pointers.
// ptrs always live; sticky (no invent soft-skip collect past hole).
func CollectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) {
	if ptrs == nil {
		SetError(ErrGeneric)
		return
	}
	if !collectReferencedPtrsBlock(b, ptrs) {
		*ptrs = IncompleteVariables()
		SetError(ErrGeneric)
	}
}

func collectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) bool {
	if b == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		// Block always live for get_referenced_ptrs walk; sticky incomplete
		SetError(ErrGeneric)
		return false
	}
	for i := range b.Stmts {
		if !collectReferencedPtrsStmt(&b.Stmts[i], ptrs) {
			return false
		}
	}
	return true
}

// appendUniqueVar appends v if not already present.
// Variable always live in collect walks; sticky leave list unchanged
// (no invent soft-skip nil hole as absent — callers fail closed IncompleteVariables).
func appendUniqueVar(s []*Variable, v *Variable) []*Variable {
	if v == nil {
		SetError(ErrGeneric)
		return s
	}
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// ReadUnionFieldExpr reports whether expression reads a union field.
// Statement.cpp:665+ subset via IsInsideUnionField.
// Incomplete IR sticky true (no invent "no union field read" / soft re-pick).
func ReadUnionFieldExpr(e *Expression) bool {
	// Expression always live; sticky incomplete as reads-union (restrictive)
	if e == nil {
		SetError(ErrGeneric)
		return true
	}
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			SetError(ErrGeneric)
			return true
		}
		ok := e.Var.IsInsideUnionField()
		// residual ERROR sticky — no invent no-union-read soft-skip past IsInsideUnionField hole
		if HasError() {
			return true
		}
		return ok
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			SetError(ErrGeneric)
			return true
		}
		if ReadUnionFieldExpr(e.CommaLHS) {
			// residual ERROR sticky — no invent union-read true past LHS hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue RHS past LHS residual
		if HasError() {
			return true
		}
		if ReadUnionFieldExpr(e.CommaRHS) {
			if HasError() {
				return true
			}
			return true
		}
		if HasError() {
			return true
		}
		return false
	case TermAssignment:
		if e.Assign == nil {
			SetError(ErrGeneric)
			return true
		}
		return ReadUnionFieldStmt(e.Assign)
	case TermFunction:
		if e.Invoke == nil {
			SetError(ErrGeneric)
			return true
		}
		for _, a := range e.Invoke.Args {
			if a == nil {
				SetError(ErrGeneric)
				return true
			}
			if ReadUnionFieldExpr(a) {
				return true
			}
		}
	}
	return false
}

// ReadUnionFieldStmt mirrors Statement::read_union_field for one stmt.
// Statement.cpp:665–678 — map_stm_effect union_field_is_read + callees'
// union_field_read. Go subset: IR walk of get_exprs/get_blocks + callee flags.
// Incomplete for-test / call-collect / block holes sticky true
// (no invent "no union field read" / soft re-pick past holes).
func ReadUnionFieldStmt(st *Stmt) bool {
	// Statement always live; sticky incomplete as reads-union (restrictive)
	if st == nil {
		SetError(ErrGeneric)
		return true
	}
	// get_exprs: for → Loop.TestExpr; assign/etc require live Expr
	// Kind-gated for (no invent Loop-on-wrong-kind)
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		if st.Loop == nil || st.Loop.TestExpr == nil {
			SetError(ErrGeneric)
			return true
		}
		if ReadUnionFieldExpr(st.Loop.TestExpr) {
			return true
		}
	} else {
		switch st.Kind {
		case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
			// C++ get_exprs always live; nil Expr sticky fail closed true
			if st.Expr == nil {
				SetError(ErrGeneric)
				return true
			}
			if ReadUnionFieldExpr(st.Expr) {
				return true
			}
		default:
			if st.Expr != nil && ReadUnionFieldExpr(st.Expr) {
				return true
			}
		}
	}
	if st.LhsVar != nil {
		if st.LhsVar.IsInsideUnionField() {
			// residual ERROR sticky — no invent union-read true past IsInsideUnionField hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-union-read past IsInside residual
		if HasError() {
			return true
		}
	}
	if st.Lhs != nil {
		// Lhs always has live Var; incomplete sticky fail closed
		if st.Lhs.Var == nil {
			SetError(ErrGeneric)
			return true
		}
		if st.Lhs.Var.IsInsideUnionField() {
			// residual ERROR sticky — no invent union-read true past IsInsideUnionField hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-union-read past IsInside residual
		if HasError() {
			return true
		}
	}
	// Statement.cpp:671–676 — get_called_funcs; callee->union_field_read
	var calls []*Invocation
	if !collectCalledInvocationsStmt(st, &calls) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	for _, inv := range calls {
		if inv == nil {
			SetError(ErrGeneric)
			return true
		}
		if inv.User != nil && inv.User.UnionFieldRead {
			return true
		}
	}
	// get_blocks → nested stmts (Then/Else for if/for body)
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			SetError(ErrGeneric)
			return true
		}
		if ReadUnionFieldBlock(b) {
			return true
		}
	}
	return false
}

// ReadUnionFieldBlock walks statements for union field access.
// Incomplete Block sticky false for nil shell is complete empty walk; live block only.
func ReadUnionFieldBlock(b *Block) bool {
	// nil block is complete empty (no stmts) — not incomplete IR
	if b == nil {
		return false
	}
	for i := range b.Stmts {
		if ReadUnionFieldStmt(&b.Stmts[i]) {
			return true
		}
	}
	return false
}

// ComputeSummary mirrors Function::compute_summary.
// Function.cpp:773–784 — referenced_ptrs + feffect external + union_field_read.
// bodyEffect is the accumulated effect of the function body (map_stm_effect[body]).
// Incomplete body IR fails closed sticky UnionFieldRead + IncompleteVariables
// ReferencedPtrs (no invent clean empty summary / soft re-pick past hole walk).
// Function always live; sticky (no invent soft-skip summary past hole).
func (f *Function) ComputeSummary(bodyEffect Effect) {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	f.ReferencedPtrs = nil
	f.UnionFieldRead = false
	if f.Body != nil {
		var ptrs []*Variable
		if !collectReferencedPtrsBlock(f.Body, &ptrs) {
			// incomplete referenced-ptrs walk — fail closed sticky needs-revisit path
			// IncompleteVariables so IsPointerReferenced cannot invent false via len(nil)==0
			f.ReferencedPtrs = IncompleteVariables()
			f.UnionFieldRead = true
			SetError(ErrGeneric)
		} else {
			f.ReferencedPtrs = ptrs
		}
		if ReadUnionFieldBlock(f.Body) {
			f.UnionFieldRead = true
		}
	}
	// feffect.add_external_effect(body effect)
	// Incomplete merge fails closed sticky (no invent silent Incomplete FEffect)
	if !EffectComplete(bodyEffect) || !EffectComplete(f.FEffect) {
		SetError(ErrGeneric)
		return
	}
	f.FEffect = f.FEffect.AddExternalEffect(bodyEffect)
	if !EffectComplete(f.FEffect) {
		SetError(ErrGeneric)
	}
}
