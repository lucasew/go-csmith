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
		// ExpressionAssign — value is LHS
		if e.Assign != nil && e.Assign.LhsVar != nil {
			return []*Expression{{
				Term:     TermVariable,
				Var:      e.Assign.LhsVar,
				ExprType: e.Assign.LhsVar.Type,
			}}
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
