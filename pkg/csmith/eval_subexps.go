// Upstream: Expression get_eval_to_subexps; Lhs have_overlapping_fields.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// GetEvalToSubexps mirrors Expression::get_eval_to_subexps.
// Variable/Constant: self; Comma: rhs only; Assign: lhs; Funcall: self (result).
func GetEvalToSubexps(e *Expression) []*Expression {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermConstant, TermVariable, TermFunction:
		return []*Expression{e}
	case TermCommaExpr:
		// ExpressionComma.cpp:102–105 — only RHS evaluates to the value
		return GetEvalToSubexps(e.CommaRHS)
	case TermAssignment:
		// ExpressionAssign.cpp:107–111 — get_lhs()->get_eval_to_subexps (Lhs pushes self)
		if e.Assign != nil {
			if e.Assign.Lhs != nil {
				return []*Expression{LhsAsExpression(e.Assign.Lhs)}
			}
			if e.Assign.LhsVar != nil {
				ty := e.Assign.LhsVar.Type
				return []*Expression{{
					Term:     TermVariable,
					Var:      e.Assign.LhsVar,
					ExprType: ty,
				}}
			}
		}
		return nil
	default:
		return []*Expression{e}
	}
}

// FindUnionPointees mirrors FactPointTo::find_union_pointees.
// FactPointTo.cpp:807–829 — union fields referred via pointer expression.
func FindUnionPointees(facts []*FactPointTo, e *Expression) []*Variable {
	if e == nil {
		return nil
	}
	var vars []*Variable
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			return nil
		}
		ind := e.IndirectLevel()
		vars = MergePointeesOfPointer(e.Var.GetCollective(), ind, facts)
	default:
		return nil
	}
	var unions []*Variable
	for _, v := range vars {
		if v == nil {
			continue
		}
		u := v.GetContainerUnion()
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
func HaveOverlappingFields(e1, e2 *Expression, facts []*FactPointTo) bool {
	vars1 := FindUnionPointees(facts, e1)
	if len(vars1) == 0 {
		return false
	}
	vars2 := FindUnionPointees(facts, e2)
	for _, v := range vars2 {
		if IsVariableInSet(vars1, v) {
			return true
		}
	}
	return false
}

// LhsAsExpression builds a TermVariable expression for Lhs (for overlap checks).
func LhsAsExpression(lhs *Lhs) *Expression {
	if lhs == nil || lhs.Var == nil {
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
func GetDereferencedPtrs(e *Expression) []*Expression {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermVariable:
		if e.IndirectLevel() > 0 {
			return []*Expression{e}
		}
	case TermCommaExpr:
		return append(GetDereferencedPtrs(e.CommaLHS), GetDereferencedPtrs(e.CommaRHS)...)
	case TermAssignment:
		if e.Assign != nil {
			return GetDereferencedPtrs(e.Assign.Expr)
		}
	case TermFunction:
		if e.Invoke != nil {
			var out []*Expression
			for _, a := range e.Invoke.Args {
				out = append(out, GetDereferencedPtrs(a)...)
			}
			return out
		}
	}
	return nil
}
