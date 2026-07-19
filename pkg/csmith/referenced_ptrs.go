// Upstream: Expression/Statement get_referenced_ptrs; Function::compute_summary.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectReferencedPtrsExpression mirrors Expression::get_referenced_ptrs.
// ExpressionVariable.cpp:230–235 — pointer vars; comma/assign recurse; invoke args + callee.
// Incomplete IR fails closed: *ptrs → IncompleteVariables (not bare nil —
// VariablesComplete(nil)/len==0 invents empty-complete ptr list success).
func CollectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) {
	if ptrs == nil {
		return
	}
	if !collectReferencedPtrsExpression(e, ptrs) {
		*ptrs = IncompleteVariables()
	}
}

// collectReferencedPtrsExpression returns false on incomplete IR.
// On failure *ptrs is IncompleteVariables (caller may also overwrite).
func collectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) bool {
	if e == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		return false
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		if e.Var == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		if e.Var.IsPointer() {
			*ptrs = appendUniqueVar(*ptrs, e.Var)
		}
		return true
	case TermCommaExpr:
		// both sides always live
		if e.CommaLHS == nil || e.CommaRHS == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		if !collectReferencedPtrsExpression(e.CommaLHS, ptrs) {
			return false
		}
		return collectReferencedPtrsExpression(e.CommaRHS, ptrs)
	case TermAssignment:
		if e.Assign == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		return collectReferencedPtrsStmt(e.Assign, ptrs)
	case TermFunction:
		// ExpressionFuncall.cpp:165–177 — param_value then eFuncCall → assert(fiu) + callee
		if e.Invoke == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		for _, a := range e.Invoke.Args {
			// Expression* args always live after ERROR_GUARD
			if a == nil {
				*ptrs = IncompleteVariables()
				return false
			}
			if !collectReferencedPtrsExpression(a, ptrs) {
				return false
			}
		}
		// ExpressionFuncall.cpp:172–177 — only user FuncCall walks callee refs
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			// incomplete callee ReferencedPtrs fails closed (no invent skip hole)
			if !VariablesComplete(e.Invoke.User.ReferencedPtrs) {
				*ptrs = IncompleteVariables()
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
		// unknown term — incomplete IR
		*ptrs = IncompleteVariables()
		return false
	}
}

// CollectReferencedPtrsStmt mirrors Statement::get_referenced_ptrs.
// Statement.cpp:331–345 — exprs + nested blocks.
// Incomplete IR → IncompleteVariables (not bare nil invent empty-complete).
func CollectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) {
	if ptrs == nil {
		return
	}
	if !collectReferencedPtrsStmt(st, ptrs) {
		*ptrs = IncompleteVariables()
	}
}

func collectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) bool {
	if st == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		return false
	}
	// Statement.cpp:331–345 — get_exprs + get_blocks; get_exprs always live for
	// assign/invoke/return/if/break/continue/goto and for-test.
	// Kind-gated for (no invent Loop-on-wrong-kind as for get_exprs).
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		// Incomplete Loop without TestExpr fails closed (no invent skip for-test
		// ptrs / soft-claim IV alone as the only for-related pointer).
		if st.Loop == nil || st.Loop.TestExpr == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		if !collectReferencedPtrsExpression(st.Loop.TestExpr, ptrs) {
			return false
		}
	} else {
		switch st.Kind {
		case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
			// C++ get_exprs always yields live Expression* for these kinds
			// incomplete nil Expr fails closed (no invent partial ptr list as success)
			if st.Expr == nil {
				*ptrs = IncompleteVariables()
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
	if st.LhsVar != nil && st.LhsVar.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.LhsVar)
	}
	if st.Lhs != nil {
		// Lhs always has live Var in C++; incomplete Lhs.Var fails closed
		if st.Lhs.Var == nil {
			*ptrs = IncompleteVariables()
			return false
		}
		if st.Lhs.Var.IsPointer() {
			*ptrs = appendUniqueVar(*ptrs, st.Lhs.Var)
		}
	}
	// get_blocks only — no invent ptrs via stray Then on assign
	blks := GetBlocksStmt(st)
	for _, b := range blks {
		if b == nil {
			// incomplete arm — fail closed (no invent partial ptr list past hole)
			*ptrs = IncompleteVariables()
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
// Incomplete IR → IncompleteVariables (not bare nil invent empty-complete).
func CollectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) {
	if ptrs == nil {
		return
	}
	if !collectReferencedPtrsBlock(b, ptrs) {
		*ptrs = IncompleteVariables()
	}
}

func collectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) bool {
	if b == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = IncompleteVariables()
		}
		return false
	}
	for i := range b.Stmts {
		if !collectReferencedPtrsStmt(&b.Stmts[i], ptrs) {
			return false
		}
	}
	return true
}

func appendUniqueVar(s []*Variable, v *Variable) []*Variable {
	if v == nil {
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
// Incomplete IR fails closed as true (no invent "no union field read").
func ReadUnionFieldExpr(e *Expression) bool {
	if e == nil {
		return true
	}
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			return true
		}
		return e.Var.IsInsideUnionField()
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return true
		}
		return ReadUnionFieldExpr(e.CommaLHS) || ReadUnionFieldExpr(e.CommaRHS)
	case TermAssignment:
		if e.Assign == nil {
			return true
		}
		return ReadUnionFieldStmt(e.Assign)
	case TermFunction:
		if e.Invoke == nil {
			return true
		}
		for _, a := range e.Invoke.Args {
			if a == nil {
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
// Incomplete for-test / call-collect / block holes fail closed true
// (no invent "no union field read").
func ReadUnionFieldStmt(st *Stmt) bool {
	if st == nil {
		return true
	}
	// get_exprs: for → Loop.TestExpr; assign/etc require live Expr
	// Kind-gated for (no invent Loop-on-wrong-kind)
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		if st.Loop == nil || st.Loop.TestExpr == nil {
			return true
		}
		if ReadUnionFieldExpr(st.Loop.TestExpr) {
			return true
		}
	} else {
		switch st.Kind {
		case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
			// C++ get_exprs always live; nil Expr fails closed true
			if st.Expr == nil {
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
	if st.LhsVar != nil && st.LhsVar.IsInsideUnionField() {
		return true
	}
	if st.Lhs != nil {
		// Lhs always has live Var; incomplete fails closed
		if st.Lhs.Var == nil {
			return true
		}
		if st.Lhs.Var.IsInsideUnionField() {
			return true
		}
	}
	// Statement.cpp:671–676 — get_called_funcs; callee->union_field_read
	var calls []*Invocation
	if !collectCalledInvocationsStmt(st, &calls) {
		return true
	}
	for _, inv := range calls {
		if inv == nil {
			return true
		}
		if inv.User != nil && inv.User.UnionFieldRead {
			return true
		}
	}
	// get_blocks → nested stmts (Then/Else for if/for body)
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			return true
		}
		if ReadUnionFieldBlock(b) {
			return true
		}
	}
	return false
}

// ReadUnionFieldBlock walks statements for union field access.
func ReadUnionFieldBlock(b *Block) bool {
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
// Incomplete body IR fails closed UnionFieldRead + IncompleteVariables ReferencedPtrs
// (no invent clean empty summary / IsPointerReferenced false via bare nil).
func (f *Function) ComputeSummary(bodyEffect Effect) {
	if f == nil {
		return
	}
	f.ReferencedPtrs = nil
	f.UnionFieldRead = false
	if f.Body != nil {
		var ptrs []*Variable
		if !collectReferencedPtrsBlock(f.Body, &ptrs) {
			// incomplete referenced-ptrs walk — fail closed needs-revisit path
			// IncompleteVariables so IsPointerReferenced cannot invent false via len(nil)==0
			f.ReferencedPtrs = IncompleteVariables()
			f.UnionFieldRead = true
		} else {
			f.ReferencedPtrs = ptrs
		}
		if ReadUnionFieldBlock(f.Body) {
			f.UnionFieldRead = true
		}
	}
	// feffect.add_external_effect(body effect)
	f.FEffect = f.FEffect.AddExternalEffect(bodyEffect)
}
