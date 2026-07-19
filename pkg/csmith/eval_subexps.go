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
// Incomplete facts/pointees/expr fail closed IncompleteVariables (not bare nil —
// VariablesComplete(nil)/len(nil)==0 invent empty-complete "no union" success).
// Complete empty (no union pointees) returns non-nil empty.
func FindUnionPointees(facts []*FactPointTo, e *Expression) []*Variable {
	if e == nil {
		return IncompleteVariables()
	}
	// incomplete fact map fails closed before merge_pointees
	if facts != nil && !FactsComplete(facts) {
		return IncompleteVariables()
	}
	var vars []*Variable
	switch e.Term {
	case TermVariable, TermLhs:
		if e.Var == nil {
			return IncompleteVariables()
		}
		// incomplete type IR must not invent level-0 merge as empty unions
		ind, iok := e.IndirectLevelComplete()
		if !iok {
			return IncompleteVariables()
		}
		vars = MergePointeesOfPointer(e.Var.GetCollective(), ind, facts)
		// incomplete merge; empty non-nil = no pointees
		if !VariablesComplete(vars) {
			return IncompleteVariables()
		}
	default:
		// non-pointer expr: complete empty union set
		return []*Variable{}
	}
	unions := make([]*Variable, 0)
	for _, v := range vars {
		if v == nil {
			return IncompleteVariables()
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
// (no invent conflict-free past holes / incomplete FindUnionPointees as empty).
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
	// incomplete → overlap; complete empty → no union pointees on e1 → no overlap
	if !VariablesComplete(vars1) {
		return true
	}
	if len(vars1) == 0 {
		return false
	}
	vars2 := FindUnionPointees(facts, e2)
	if !VariablesComplete(vars2) {
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
