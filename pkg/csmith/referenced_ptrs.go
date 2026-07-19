// Upstream: Expression/Statement get_referenced_ptrs; Function::compute_summary.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectReferencedPtrsExpression mirrors Expression::get_referenced_ptrs.
// ExpressionVariable.cpp:230–235 — pointer vars; comma/assign recurse; invoke args + callee.
func CollectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) {
	if e == nil || ptrs == nil {
		return
	}
	switch e.Term {
	case TermVariable:
		if e.Var != nil && e.Var.IsPointer() {
			*ptrs = appendUniqueVar(*ptrs, e.Var)
		}
	case TermCommaExpr:
		CollectReferencedPtrsExpression(e.CommaLHS, ptrs)
		CollectReferencedPtrsExpression(e.CommaRHS, ptrs)
	case TermAssignment:
		if e.Assign != nil {
			CollectReferencedPtrsStmt(e.Assign, ptrs)
		}
	case TermFunction:
		// ExpressionFuncall.cpp:165–177 — param_value then eFuncCall → assert(fiu) + callee
		if e.Invoke == nil {
			return
		}
		for _, a := range e.Invoke.Args {
			// Expression* args always live after ERROR_GUARD; nil hole fails closed
			// (clear collected so far — no invent partial arg ptrs)
			if a == nil {
				*ptrs = nil
				return
			}
			CollectReferencedPtrsExpression(a, ptrs)
		}
		// ExpressionFuncall.cpp:172–177 — only user FuncCall walks callee refs
		// assert(fiu); incomplete std-as-user skip (no invent empty follow)
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			for _, p := range e.Invoke.User.ReferencedPtrs {
				// Variable* always live on ReferencedPtrs; nil hole fails closed
				if p == nil {
					*ptrs = nil
					return
				}
				*ptrs = appendUniqueVar(*ptrs, p)
			}
		}
	}
}

// CollectReferencedPtrsStmt mirrors Statement::get_referenced_ptrs.
// Statement.cpp:331–345 — exprs + nested blocks.
func CollectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) {
	if st == nil || ptrs == nil {
		return
	}
	if st.Expr != nil {
		CollectReferencedPtrsExpression(st.Expr, ptrs)
	}
	if st.LhsVar != nil && st.LhsVar.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.LhsVar)
	}
	if st.Lhs != nil && st.Lhs.Var != nil && st.Lhs.Var.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.Lhs.Var)
	}
	if st.Then != nil {
		CollectReferencedPtrsBlock(st.Then, ptrs)
	}
	if st.Else != nil {
		CollectReferencedPtrsBlock(st.Else, ptrs)
	}
	if st.Loop != nil && st.Loop.IV != nil && st.Loop.IV.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.Loop.IV)
	}
}

// CollectReferencedPtrsBlock walks all statements in a block.
func CollectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) {
	if b == nil || ptrs == nil {
		return
	}
	for i := range b.Stmts {
		CollectReferencedPtrsStmt(&b.Stmts[i], ptrs)
	}
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
func ReadUnionFieldExpr(e *Expression) bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermVariable:
		return e.Var != nil && e.Var.IsInsideUnionField()
	case TermCommaExpr:
		return ReadUnionFieldExpr(e.CommaLHS) || ReadUnionFieldExpr(e.CommaRHS)
	case TermAssignment:
		if e.Assign != nil {
			return ReadUnionFieldStmt(e.Assign)
		}
	case TermFunction:
		if e.Invoke != nil {
			for _, a := range e.Invoke.Args {
				if ReadUnionFieldExpr(a) {
					return true
				}
			}
		}
	}
	return false
}

// ReadUnionFieldStmt mirrors Statement::read_union_field for one stmt.
func ReadUnionFieldStmt(st *Stmt) bool {
	if st == nil {
		return false
	}
	if ReadUnionFieldExpr(st.Expr) {
		return true
	}
	if st.LhsVar != nil && st.LhsVar.IsInsideUnionField() {
		return true
	}
	if st.Then != nil && ReadUnionFieldBlock(st.Then) {
		return true
	}
	if st.Else != nil && ReadUnionFieldBlock(st.Else) {
		return true
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
func (f *Function) ComputeSummary(bodyEffect Effect) {
	if f == nil {
		return
	}
	f.ReferencedPtrs = nil
	if f.Body != nil {
		CollectReferencedPtrsBlock(f.Body, &f.ReferencedPtrs)
		f.UnionFieldRead = ReadUnionFieldBlock(f.Body)
	}
	// feffect.add_external_effect(body effect)
	f.FEffect = f.FEffect.AddExternalEffect(bodyEffect)
}
