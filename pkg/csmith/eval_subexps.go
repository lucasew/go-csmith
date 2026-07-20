// Upstream: Expression get_eval_to_subexps; Lhs have_overlapping_fields.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// GetEvalToSubexps mirrors Expression::get_eval_to_subexps.
// Variable/Constant: self; Comma: rhs only; Assign: lhs; Funcall: self (result).
// Incomplete IR fails closed sticky IncompleteExpressions (not bare nil —
// ExpressionsComplete(nil)/len==0 invents empty-complete eval list / soft re-pick skip overlap).
// Complete expressions always yield ≥1 subexp.
func GetEvalToSubexps(e *Expression) []*Expression {
	if e == nil {
		SetError(ErrGeneric)
		return IncompleteExpressions()
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Type* + Value; Type-nil sticky
		// (no invent self-eval complete list past incomplete Constant shell)
		if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		return []*Expression{e}
	case TermVariable, TermLhs:
		// ExpressionVariable / Lhs always have live Variable*
		if e.Var == nil {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		// Type* always live on Variable for eval; Type-nil sticky
		// Special null/garbage/tbd have Type nil by design — complete self-eval
		if e.Var.Type == nil && !IsSpecialPtr(e.Var) {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		return []*Expression{e}
	case TermFunction:
		// ExpressionFuncall always live invoke (eval is the call itself)
		if e.Invoke == nil {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		return []*Expression{e}
	case TermCommaExpr:
		// ExpressionComma.cpp:102–105 — only RHS evaluates to the value
		if e.CommaRHS == nil {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		sub := GetEvalToSubexps(e.CommaRHS)
		// residual ERROR sticky — no invent self-eval complete list past RHS residual
		if HasError() {
			return IncompleteExpressions()
		}
		return sub
	case TermAssignment:
		// ExpressionAssign.cpp:107–111 — get_lhs()->get_eval_to_subexps (Lhs pushes self)
		if e.Assign == nil {
			SetError(ErrGeneric)
			return IncompleteExpressions()
		}
		if e.Assign.Lhs != nil {
			// Lhs always live Var
			if e.Assign.Lhs.Var == nil {
				SetError(ErrGeneric)
				return IncompleteExpressions()
			}
			// Type* always live for eval; Type-nil sticky (specials exempt)
			if e.Assign.Lhs.Var.Type == nil && !IsSpecialPtr(e.Assign.Lhs.Var) {
				SetError(ErrGeneric)
				return IncompleteExpressions()
			}
			sub := LhsAsExpression(e.Assign.Lhs)
			if sub == nil {
				SetError(ErrGeneric)
				return IncompleteExpressions()
			}
			return []*Expression{sub}
		}
		if e.Assign.LhsVar != nil {
			// Type* always live; Type-nil sticky (no invent untyped LHS eval shell)
			// Special null/garbage/tbd have Type nil by design — complete path
			if e.Assign.LhsVar.Type == nil && !IsSpecialPtr(e.Assign.LhsVar) {
				SetError(ErrGeneric)
				return IncompleteExpressions()
			}
			ty := e.Assign.LhsVar.Type
			return []*Expression{{
				Term:     TermVariable,
				Var:      e.Assign.LhsVar,
				ExprType: ty,
			}}
		}
		// assign without lhs — incomplete IR sticky
		SetError(ErrGeneric)
		return IncompleteExpressions()
	default:
		// unknown term — incomplete IR sticky (no invent self-eval shell)
		SetError(ErrGeneric)
		return IncompleteExpressions()
	}
}

// FindUnionPointees mirrors FactPointTo::find_union_pointees.
// FactPointTo.cpp:807–829 — union fields referred via pointer expression.
// Incomplete facts/pointees/expr fail closed sticky IncompleteVariables (not bare nil —
// VariablesComplete(nil)/len(nil)==0 invent empty-complete "no union" success / soft re-pick).
// Complete empty (no union pointees) returns non-nil empty.
func FindUnionPointees(facts []*FactPointTo, e *Expression) []*Variable {
	if e == nil {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	// incomplete fact map fails closed sticky before merge_pointees
	if facts != nil && !FactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	var vars []*Variable
	switch e.Term {
	case TermVariable, TermLhs:
		if e.Var == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		// incomplete type IR must not invent level-0 merge as empty unions
		ind, iok := e.IndirectLevelComplete()
		if !iok {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		coll := e.Var.GetCollective()
		// residual ERROR sticky — no invent soft-merge past GetCollective residual hole
		if HasError() {
			return IncompleteVariables()
		}
		vars = MergePointeesOfPointer(coll, ind, facts)
		// residual ERROR sticky — no invent soft-merge past MergePointees residual hole
		if HasError() {
			return IncompleteVariables()
		}
		// incomplete merge; empty non-nil = no pointees
		if !VariablesComplete(vars) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
	default:
		// non-pointer expr: complete empty union set
		return []*Variable{}
	}
	unions := make([]*Variable, 0)
	for _, v := range vars {
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		u := v.GetContainerUnion()
		// residual ERROR sticky — no invent soft-continue later pointees past GetContainerUnion residual
		if HasError() {
			return IncompleteVariables()
		}
		// only care referenced union fields, not the union itself
		if u != nil && v != u {
			if !IsVariableInSet(unions, u) {
				unions = append(unions, u)
			}
		}
	}
	return unions
}

// HaveOverlappingFields mirrors have_overlapping_fields.
// Lhs.cpp:287–298 — shared union pointee between e1 and e2.
// Incomplete fact maps / pointees / exprs fail closed sticky as overlap
// (no invent conflict-free / soft re-pick past holes).
func HaveOverlappingFields(e1, e2 *Expression, facts []*FactPointTo) bool {
	if facts != nil && !FactsComplete(facts) {
		SetError(ErrGeneric)
		return true
	}
	// incomplete expression shells sticky as overlap
	if e1 == nil || e2 == nil {
		SetError(ErrGeneric)
		return true
	}
	if (e1.Term == TermVariable || e1.Term == TermLhs) && e1.Var == nil {
		SetError(ErrGeneric)
		return true
	}
	if (e2.Term == TermVariable || e2.Term == TermLhs) && e2.Var == nil {
		SetError(ErrGeneric)
		return true
	}
	vars1 := FindUnionPointees(facts, e1)
	// residual ERROR sticky — no invent soft-continue overlap past FindUnion residual
	if HasError() {
		return true
	}
	// incomplete → sticky overlap; complete empty → no union pointees on e1
	if !VariablesComplete(vars1) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	if len(vars1) == 0 {
		return false
	}
	vars2 := FindUnionPointees(facts, e2)
	// residual ERROR sticky — no invent soft-continue overlap past e2 FindUnion residual
	if HasError() {
		return true
	}
	if !VariablesComplete(vars2) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	for _, v := range vars2 {
		if IsVariableInSet(vars1, v) {
			return true
		}
	}
	return false
}

// LhsAsExpression builds a TermVariable expression for Lhs (for overlap checks).
// Incomplete Lhs shell sticky nil (no invent soft-skip / empty expression past hole).
func LhsAsExpression(lhs *Lhs) *Expression {
	// Lhs always live with Variable*; sticky incomplete no invent nil soft-skip
	if lhs == nil || lhs.Var == nil {
		SetError(ErrGeneric)
		return nil
	}
	ty := lhs.Type
	if ty == nil {
		ty = lhs.Var.Type
	}
	return &Expression{Term: TermVariable, Var: lhs.Var, ExprType: ty}
}

// GetDereferencedPtrs mirrors ExpressionVariable::get_dereferenced_ptrs.
// ExpressionVariable.cpp:221–227 — self if indirect_level > 0.
// Incomplete IR fails closed sticky IncompleteExpressions (not bare nil invent
// empty-complete deref list / soft re-pick past holes).
// Complete no-deref cases return empty non-nil slice so callers can distinguish.
func GetDereferencedPtrs(e *Expression) []*Expression {
	out, ok := collectDereferencedPtrs(e)
	if !ok {
		SetError(ErrGeneric)
		return IncompleteExpressions()
	}
	return out
}

func collectDereferencedPtrs(e *Expression) (out []*Expression, ok bool) {
	if e == nil {
		return nil, false
	}
	switch e.Term {
	case TermConstant:
		return []*Expression{}, true
	case TermVariable, TermLhs:
		// Variable* always live; incomplete type IR fails closed
		ind, iok := e.IndirectLevelComplete()
		if !iok {
			return nil, false
		}
		if ind > 0 {
			return []*Expression{e}, true
		}
		return []*Expression{}, true
	case TermCommaExpr:
		// ExpressionComma.cpp:95–99 — both sides always live
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return nil, false
		}
		left, ok1 := collectDereferencedPtrs(e.CommaLHS)
		if !ok1 {
			return nil, false
		}
		right, ok2 := collectDereferencedPtrs(e.CommaRHS)
		if !ok2 {
			return nil, false
		}
		return append(left, right...), true
	case TermAssignment:
		// ExpressionAssign — assign->get_dereferenced_ptrs → expr
		if e.Assign == nil {
			return nil, false
		}
		if e.Assign.Expr == nil {
			return nil, false
		}
		return collectDereferencedPtrs(e.Assign.Expr)
	case TermFunction:
		// ExpressionFuncall.cpp:149–159 — param_value[i] always live
		if e.Invoke == nil {
			return nil, false
		}
		out = []*Expression{}
		for _, a := range e.Invoke.Args {
			if a == nil {
				return nil, false
			}
			part, okp := collectDereferencedPtrs(a)
			if !okp {
				return nil, false
			}
			out = append(out, part...)
		}
		return out, true
	default:
		// unknown term — incomplete IR
		return nil, false
	}
}
