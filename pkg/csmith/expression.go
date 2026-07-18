// Upstream: Expression.h / Expression.cpp
// (eTermType, InitExprProbabilityTable, ExpressionTypeProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// TermType mirrors eTermType.
type TermType int

const (
	TermConstant TermType = iota
	TermVariable
	TermFunction
	TermAssignment
	TermCommaExpr
	TermLhs // not in MAX_TERM_TYPES range for pick
)

// MaxTermTypes mirrors MAX_TERM_TYPES (eCommaExpr+1) — used as “unspecified”.
const MaxTermTypes TermType = TermCommaExpr + 1

// Expression is a minimal Expression base (term + optional constant/var).
type Expression struct {
	Term TermType
	// Con is set for TermConstant.
	Con *Constant
	// Var is set for TermVariable (the selected variable).
	Var *Variable
}

// ExprTables holds expr/param DistributionTables (Expression::exprTable_/paramTable_).
type ExprTables struct {
	Expr  DistributionTable
	Param DistributionTable
}

// NewExprTables mirrors Expression::InitProbabilityTables for given options.
// Expression.cpp:68–96.
func NewExprTables(opts Options) *ExprTables {
	t := &ExprTables{}
	// InitExprProbabilityTable
	t.Expr.AddEntry(int(TermFunction), 70)
	t.Expr.AddEntry(int(TermVariable), 20)
	t.Expr.AddEntry(int(TermConstant), 10)
	if opts.EmbeddedAssigns {
		t.Expr.AddEntry(int(TermAssignment), 10)
	}
	if opts.CommaOperators {
		t.Expr.AddEntry(int(TermCommaExpr), 10)
	}
	// InitParamProbabilityTable
	t.Param.AddEntry(int(TermFunction), 40)
	t.Param.AddEntry(int(TermVariable), 40)
	t.Param.AddEntry(int(TermConstant), 0) // constant params disallowed (weight 0 — not added if we skip; upstream adds 0)
	// weight 0 still add_entry in upstream but max doesn't grow... actually add_entry always adds
	// Replicate: add with 0 does max+=0
	if opts.EmbeddedAssigns {
		t.Param.AddEntry(int(TermAssignment), 10)
	}
	if opts.CommaOperators {
		t.Param.AddEntry(int(TermCommaExpr), 10)
	}
	return t
}

// ExpressionTypeProbability mirrors ExpressionTypeProbability.
// Expression.cpp:103–112 — rnd_upto(filter.max, filter); lookup → eTermType.
// PartialExpander force-function skipped (not ported).
func ExpressionTypeProbability(r *Rng, filter *VectorFilter) TermType {
	if r == nil || filter == nil {
		return TermVariable
	}
	i := r.RndUptoFilter(uint32(filter.MaxProb()), filter)
	return TermType(filter.Lookup(int(i)))
}

// PickTermType builds default filters for Expression::make_random when tt==MAX.
// Expression.cpp:160–179 (subset: no_func, no_const, depth).
func PickTermType(r *Rng, tables *ExprTables, opts Options, typ *Type, noFunc, noConst bool, exprDepth int) TermType {
	if tables == nil {
		tables = NewExprTables(opts)
	}
	f := NewVectorFilter(&tables.Expr)
	if noFunc {
		f.Add(int(TermFunction))
	}
	if noConst {
		f.Add(int(TermConstant))
	}
	// depth gate: Expression.cpp:177–178
	if exprDepth+2 > opts.MaxExprComplexity {
		f.Add(int(TermFunction)).Add(int(TermAssignment)).Add(int(TermCommaExpr))
	}
	_ = typ // struct/union filters deferred
	return ExpressionTypeProbability(r, f)
}

// MakeRandomExpression mirrors Expression::make_random for Constant and Variable only.
// Function/Assign/Comma return nil term with false ok until those ports land.
// Expression.cpp:141–204.
func MakeRandomExpression(
	r *Rng,
	opts Options,
	tables *ExprTables,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	noFunc, noConst bool,
	tt TermType,
	exprDepth int,
) *Expression {
	if typ == nil {
		// choose_random_nonvoid deferred — require type for now
		typ = GetSimpleType(EInt)
	}
	if tt == MaxTermTypes {
		tt = PickTermType(r, tables, opts, typ, noFunc, noConst, exprDepth)
	}
	switch tt {
	case TermConstant:
		// Expression.cpp:185–188
		return &Expression{Term: TermConstant, Con: MakeRandom(typ, opts, r)}
	case TermVariable:
		// ExpressionVariable::make_random simplified — SelectGlobal flexible
		return makeExpressionVariable(r, vs, cg, typ, qfer)
	default:
		// Not yet: Function, Assignment, Comma
		return nil
	}
}

// makeExpressionVariable — ExpressionVariable.cpp:56+ simplified:
// VariableSelector::select → SelectGlobal with MatchFlexible (globals only).
func makeExpressionVariable(r *Rng, vs *VariableSelector, cg CGContext, typ *Type, qfer *CVQualifiers) *Expression {
	if vs == nil {
		return nil
	}
	// Prefer choose among existing globals with flexible match; else create.
	var ok []*Variable
	for _, v := range vs.GlobalList {
		if v != nil && v.Type != nil && typ != nil && typ.Match(v.Type, MatchFlexible) {
			ok = append(ok, v)
		}
	}
	var v *Variable
	if len(ok) > 0 {
		v = ChooseOKVar(r, ok)
	} else {
		v = vs.GenerateNewGlobal(AccessRead, cg, typ, qfer, r)
	}
	if v == nil {
		return nil
	}
	return &Expression{Term: TermVariable, Var: v}
}

// Output is a minimal C fragment for tests (not full Expression::Output).
func (e *Expression) Output() string {
	if e == nil {
		return ""
	}
	switch e.Term {
	case TermConstant:
		if e.Con != nil {
			return e.Con.Value
		}
	case TermVariable:
		if e.Var != nil {
			return e.Var.Name
		}
	}
	return "/*expr*/"
}
