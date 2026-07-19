// Upstream: Expression get_eval_to_subexps; Lhs have_overlapping_fields.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// GetEvalToSubexps mirrors Expression::get_eval_to_subexps.
// Variable/Constant: self; Comma: rhs only; Assign: lhs; Funcall: self (result).
// Incomplete IR fails closed as nil (no invent empty eval list / skip overlap).
// Complete expressions always yield ≥1 subexp.
func GetEvalToSubexps(e *Expression) []*Expression {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Value
		if e.Con == nil || e.Con.Value == "" {
			return nil
		}
		return []*Expression{e}
	case TermVariable, TermLhs:
		// ExpressionVariable / Lhs always have live Variable*
		if e.Var == nil {
			return nil
		}
		return []*Expression{e}
	case TermFunction:
		// ExpressionFuncall always live invoke (eval is the call itself)
		if e.Invoke == nil {
			return nil
		}
		return []*Expression{e}
	case TermCommaExpr:
		// ExpressionComma.cpp:102–105 — only RHS evaluates to the value
		if e.CommaRHS == nil {
			return nil
		}
		return GetEvalToSubexps(e.CommaRHS)
	case TermAssignment:
		// ExpressionAssign.cpp:107–111 — get_lhs()->get_eval_to_subexps (Lhs pushes self)
		if e.Assign == nil {
			return nil
		}
		if e.Assign.Lhs != nil {
			// Lhs always live Var
			if e.Assign.Lhs.Var == nil {
				return nil
			}
			sub := LhsAsExpression(e.Assign.Lhs)
			if sub == nil {
				return nil
			}
			return []*Expression{sub}
		}
		if e.Assign.LhsVar != nil {
			ty := e.Assign.LhsVar.Type
			return []*Expression{{
				Term:     TermVariable,
				Var:      e.Assign.LhsVar,
				ExprType: ty,
			}}
		}
		// assign without lhs — incomplete IR
		return nil
	default:
		// unknown term — incomplete IR (no invent self-eval shell)
		return nil
	}
}

// FindUnionPointees mirrors FactPointTo::find_union_pointees.
// FactPointTo.cpp:807–829 — union fields referred via pointer expression.
// Pointee Variable* always live; nil hole fails closed (nil out, no invent partial).
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
			return nil
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
// Incomplete fact maps / pointees / exprs fail closed as overlap
// (no invent conflict-free past holes).
func HaveOverlappingFields(e1, e2 *Expression, facts []*FactPointTo) bool {
	if facts != nil && !FactsComplete(facts) {
		return true
	}
	// incomplete expression shells fail closed as overlap
	if e1 == nil || e2 == nil {
		return true
	}
	if (e1.Term == TermVariable || e1.Term == TermLhs) && e1.Var == nil {
		return true
	}
	if (e2.Term == TermVariable || e2.Term == TermLhs) && e2.Var == nil {
		return true
	}
	vars1 := FindUnionPointees(facts, e1)
	if len(vars1) == 0 {
		return false
	}
	vars2 := FindUnionPointees(facts, e2)
	for _, v := range vars2 {
		if v == nil {
			return true
		}
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
// Incomplete IR fails closed as nil (no invent partial deref list past holes).
// Complete no-deref cases return empty non-nil slice so callers can distinguish.
func GetDereferencedPtrs(e *Expression) []*Expression {
	out, ok := collectDereferencedPtrs(e)
	if !ok {
		return nil
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
		// Variable* always live
		if e.Var == nil || e.Var.Type == nil {
			return nil, false
		}
		if e.IndirectLevel() > 0 {
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
