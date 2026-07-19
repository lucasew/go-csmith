// Upstream: Expression/Statement get_referenced_ptrs; Function::compute_summary.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CollectReferencedPtrsExpression mirrors Expression::get_referenced_ptrs.
// ExpressionVariable.cpp:230–235 — pointer vars; comma/assign recurse; invoke args + callee.
// Incomplete IR fails closed: clears *ptrs and returns false (no invent partial lists).
func CollectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) {
	_ = collectReferencedPtrsExpression(e, ptrs)
}

// collectReferencedPtrsExpression returns false on incomplete IR (*ptrs cleared).
func collectReferencedPtrsExpression(e *Expression, ptrs *[]*Variable) bool {
	if e == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = nil
		}
		return false
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		if e.Var == nil {
			*ptrs = nil
			return false
		}
		if e.Var.IsPointer() {
			*ptrs = appendUniqueVar(*ptrs, e.Var)
		}
		return true
	case TermCommaExpr:
		// both sides always live
		if e.CommaLHS == nil || e.CommaRHS == nil {
			*ptrs = nil
			return false
		}
		if !collectReferencedPtrsExpression(e.CommaLHS, ptrs) {
			return false
		}
		return collectReferencedPtrsExpression(e.CommaRHS, ptrs)
	case TermAssignment:
		if e.Assign == nil {
			*ptrs = nil
			return false
		}
		return collectReferencedPtrsStmt(e.Assign, ptrs)
	case TermFunction:
		// ExpressionFuncall.cpp:165–177 — param_value then eFuncCall → assert(fiu) + callee
		if e.Invoke == nil {
			*ptrs = nil
			return false
		}
		for _, a := range e.Invoke.Args {
			// Expression* args always live after ERROR_GUARD
			if a == nil {
				*ptrs = nil
				return false
			}
			if !collectReferencedPtrsExpression(a, ptrs) {
				return false
			}
		}
		// ExpressionFuncall.cpp:172–177 — only user FuncCall walks callee refs
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			for _, p := range e.Invoke.User.ReferencedPtrs {
				// Variable* always live on ReferencedPtrs
				if p == nil {
					*ptrs = nil
					return false
				}
				*ptrs = appendUniqueVar(*ptrs, p)
			}
		}
		return true
	case TermConstant:
		return true
	default:
		// unknown term — incomplete IR
		*ptrs = nil
		return false
	}
}

// CollectReferencedPtrsStmt mirrors Statement::get_referenced_ptrs.
// Statement.cpp:331–345 — exprs + nested blocks.
func CollectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) {
	_ = collectReferencedPtrsStmt(st, ptrs)
}

func collectReferencedPtrsStmt(st *Stmt, ptrs *[]*Variable) bool {
	if st == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = nil
		}
		return false
	}
	if st.Expr != nil {
		if !collectReferencedPtrsExpression(st.Expr, ptrs) {
			return false
		}
	}
	if st.LhsVar != nil && st.LhsVar.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.LhsVar)
	}
	if st.Lhs != nil {
		// Lhs always has live Var in C++; incomplete Lhs.Var fails closed
		if st.Lhs.Var == nil {
			*ptrs = nil
			return false
		}
		if st.Lhs.Var.IsPointer() {
			*ptrs = appendUniqueVar(*ptrs, st.Lhs.Var)
		}
	}
	if st.Then != nil {
		if !collectReferencedPtrsBlock(st.Then, ptrs) {
			return false
		}
	}
	if st.Else != nil {
		if !collectReferencedPtrsBlock(st.Else, ptrs) {
			return false
		}
	}
	if st.Loop != nil && st.Loop.IV != nil && st.Loop.IV.IsPointer() {
		*ptrs = appendUniqueVar(*ptrs, st.Loop.IV)
	}
	return true
}

// CollectReferencedPtrsBlock walks all statements in a block.
func CollectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) {
	_ = collectReferencedPtrsBlock(b, ptrs)
}

func collectReferencedPtrsBlock(b *Block, ptrs *[]*Variable) bool {
	if b == nil || ptrs == nil {
		if ptrs != nil {
			*ptrs = nil
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
		return false
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
