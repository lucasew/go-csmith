// Upstream: Expression.h / Expression.cpp
// (eTermType, InitExprProbabilityTable, ExpressionTypeProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

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
	// ExprType is the desired type for TermVariable (ExpressionVariable::type).
	// Indir = Var.Type.IndirectLevel - ExprType.IndirectLevel (*… or &).
	ExprType *Type
	// Invoke is set for TermFunction.
	Invoke *Invocation
	// Assign is set for TermAssignment (embedded StatementAssign).
	Assign *Stmt
	// CommaLHS / CommaRHS for TermCommaExpr.
	CommaLHS *Expression
	CommaRHS *Expression
	// CastType is Expression::cast_type (optional C cast prefix).
	CastType *Type
}

// CompatibleWithVar mirrors ExpressionVariable::compatible(Variable*).
// ExpressionVariable.cpp:288–291 — var.compatible(v); non-variable terms false.
func (e *Expression) CompatibleWithVar(v *Variable, expandStruct bool) bool {
	if e == nil || v == nil {
		return false
	}
	if e.Term != TermVariable || e.Var == nil {
		return false
	}
	return e.Var.Compatible(v, expandStruct)
}

// CompatibleWithExpr mirrors ExpressionVariable::compatible(Expression*).
// ExpressionVariable.cpp:276–282 — exp.compatible(&var).
func (e *Expression) CompatibleWithExpr(other *Expression, expandStruct bool) bool {
	if e == nil || other == nil {
		return false
	}
	if e.Term == TermVariable && e.Var != nil {
		return other.CompatibleWithVar(e.Var, expandStruct)
	}
	if other.Term == TermVariable && other.Var != nil {
		return e.CompatibleWithVar(other.Var, expandStruct)
	}
	return false
}

// CheckAndSetCast mirrors Expression::check_and_set_cast.
// Expression.cpp:222–226 — lang_cpp or needs_cast; we apply when desired type needs cast.
func (e *Expression) CheckAndSetCast(desired *Type) {
	if e == nil || desired == nil {
		return
	}
	// Source type from var / const / invoke approximate
	var src *Type
	switch e.Term {
	case TermVariable:
		if e.ExprType != nil {
			src = e.ExprType
		} else if e.Var != nil {
			src = e.Var.Type
		}
	case TermConstant:
		if e.Con != nil {
			src = e.Con.Type
		}
	default:
		return
	}
	if src != nil && desired.NeedsCast(src) {
		e.CastType = desired
	}
}

// IndirectLevel mirrors ExpressionVariable::get_indirect_level.
func (e *Expression) IndirectLevel() int {
	if e == nil || e.Var == nil || e.Var.Type == nil {
		return 0
	}
	want := e.ExprType
	if want == nil {
		want = e.Var.Type
	}
	return e.Var.Type.IndirectLevel() - want.IndirectLevel()
}

// GetType mirrors Expression::get_type approximate.
func (e *Expression) GetType() *Type {
	if e == nil {
		return nil
	}
	if e.CastType != nil {
		return e.CastType
	}
	switch e.Term {
	case TermConstant:
		if e.Con != nil {
			return e.Con.Type
		}
	case TermVariable:
		if e.ExprType != nil {
			return e.ExprType
		}
		if e.Var != nil {
			return e.Var.Type
		}
	case TermFunction:
		if e.Invoke != nil && e.Invoke.User != nil {
			return e.Invoke.User.ReturnType
		}
	case TermCommaExpr:
		return e.CommaRHS.GetType()
	case TermAssignment:
		if e.Assign != nil && e.Assign.LhsVar != nil {
			return e.Assign.LhsVar.Type
		}
	}
	return nil
}

// EqualsInt mirrors Expression::equals(int) for constants.
func (e *Expression) EqualsInt(num int) bool {
	if e == nil || e.Term != TermConstant || e.Con == nil {
		return false
	}
	return e.Con.Equals(num)
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
	// Expression.cpp:166–175 — struct/union cannot be constant subexpr; func gated by return flags
	if typ != nil && (typ.IsStruct() || typ.IsUnion()) {
		f.Add(int(TermConstant))
		if typ.IsStruct() && !opts.ReturnStructs {
			f.Add(int(TermFunction))
		}
		if typ.IsUnion() && !opts.ReturnUnions {
			f.Add(int(TermFunction))
		}
		if typ.IsConstStructUnion() || typ.IsVolatileStructUnion() {
			f.Add(int(TermAssignment))
		}
	}
	// depth gate: Expression.cpp:177–178
	if exprDepth+2 > opts.MaxExprComplexity {
		f.Add(int(TermFunction)).Add(int(TermAssignment)).Add(int(TermCommaExpr))
	}
	return ExpressionTypeProbability(r, f)
}

// PickParamTermType mirrors Expression::make_random_param term selection.
// Expression.cpp:244–260 — paramTable + always filter Constant.
func PickParamTermType(r *Rng, tables *ExprTables, opts Options, typ *Type, exprDepth int) TermType {
	if tables == nil {
		tables = NewExprTables(opts)
	}
	f := NewVectorFilter(&tables.Param)
	// don't call functions with constant parameters
	f.Add(int(TermConstant))
	if typ != nil {
		if typ.IsStruct() && !opts.ReturnStructs {
			f.Add(int(TermFunction))
		}
		if typ.IsUnion() && !opts.ReturnUnions {
			f.Add(int(TermFunction))
		}
		if typ.IsConstStructUnion() {
			f.Add(int(TermAssignment))
		}
	}
	if exprDepth+2 > opts.MaxExprComplexity {
		f.Add(int(TermFunction)).Add(int(TermAssignment)).Add(int(TermCommaExpr))
	}
	return ExpressionTypeProbability(r, f)
}

// MakeRandomParam mirrors Expression::make_random_param.
// Expression.cpp:238–296 — param probability table; constants filtered out.
func MakeRandomParam(
	r *Rng,
	opts Options,
	tables *ExprTables,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	exprDepth int,
	list ...*FunctionList,
) *Expression {
	if typ == nil {
		typ = GetIntType()
	}
	tt := PickParamTermType(r, tables, opts, typ, exprDepth)
	// hard depth cap
	if exprDepth+2 > opts.MaxExprComplexity {
		if tt == TermFunction || tt == TermAssignment || tt == TermCommaExpr {
			tt = TermVariable
		}
	}
	// ExpressionVariable::make_random(..., as_param=true)
	if tt == TermVariable {
		return makeExpressionVariableFlags(r, vs, cg, typ, qfer, true, false)
	}
	// no_func=false, no_const=true for other terms
	return MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, false, true, tt, exprDepth, list...)
}

// MakeRandomExpression mirrors Expression::make_random (const/var/funcall).
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
	list ...*FunctionList,
) *Expression {
	var flist *FunctionList
	if len(list) > 0 {
		flist = list[0]
	}
	if flist == nil {
		flist = cg.Funcs
	}

	if typ == nil {
		// Expression.cpp:147–152 choose_random_nonvoid when type==nullptr
		st := ChooseRandomNonvoidSimple(r, NewProbabilities(opts))
		typ = GetSimpleType(st)
	}
	if tt == MaxTermTypes {
		tt = PickTermType(r, tables, opts, typ, noFunc, noConst, exprDepth)
	}
	// Hard depth cap: never nest Function/Assign/Comma when near max_expr_depth
	// (mirrors Expression.cpp:177–178 filter; prevents unbounded recursion).
	if exprDepth+2 > opts.MaxExprComplexity {
		if tt == TermFunction || tt == TermAssignment || tt == TermCommaExpr {
			if noConst {
				tt = TermVariable
			} else {
				// prefer constant leaf
				tt = TermConstant
			}
		}
	}
	switch tt {
	case TermConstant:
		// Expression.cpp:185–188
		return &Expression{Term: TermConstant, Con: MakeRandom(typ, opts, r)}
	case TermVariable:
		// ExpressionVariable::make_random simplified — SelectGlobal flexible
		return makeExpressionVariable(r, vs, cg, typ, qfer)
	case TermFunction:
		// ExpressionFuncall::make_random
		return makeExpressionFuncall(r, opts, vs, tables, cg, typ, qfer, flist)
	case TermAssignment:
		// ExpressionAssign::make_random
		return MakeExpressionAssign(r, opts, NewProbabilities(opts), vs, tables, cg, typ, qfer)
	case TermCommaExpr:
		// ExpressionComma::make_random
		return MakeExpressionComma(r, opts, NewProbabilities(opts), vs, tables, cg, typ, qfer)
	default:
		return nil
	}
}

// makeExpressionVariable — ExpressionVariable.cpp:56+ :
// VariableSelector::select(READ, type, qfer, eFlexible); as_param / as_return filters.
func makeExpressionVariable(r *Rng, vs *VariableSelector, cg CGContext, typ *Type, qfer *CVQualifiers) *Expression {
	return makeExpressionVariableFlags(r, vs, cg, typ, qfer, false, false)
}

// makeExpressionVariableFlags adds as_param / as_return restrictions (ExpressionVariable.cpp:99–117).
func makeExpressionVariableFlags(
	r *Rng,
	vs *VariableSelector,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	asParam, asReturn bool,
) *Expression {
	if vs == nil {
		return nil
	}
	// try several selects if filtered
	for tries := 0; tries < 8; tries++ {
		v := vs.Select(AccessRead, cg, typ, qfer, r, MatchFlexible)
		if v == nil {
			return nil
		}
		if typ != nil && v.Type != nil {
			// as_param: forbid address-of argument (is_dereferenced_from desired type)
			if asParam && v.IsArgument() && typ.PtrType() != nil && typ.Match(v.Type, MatchDereference) {
				continue
			}
			// !addr_taken_of_locals: forbid & of local/arg when desired is pointer-to-var
			if !vs.Opts.AddrTakenOfLocals && (v.IsArgument() || v.IsLocal()) {
				if typ.IndirectLevel() < v.Type.IndirectLevel() {
					if v.Type.IndirectLevel()-typ.IndirectLevel() < 0 {
						continue
					}
				}
			}
			// ExpressionVariable.cpp:111–115 — as_return + no_return_dead_ptr
			if asReturn && vs.Opts.NoReturnDeadPointer && v.IsPointer() {
				indirection := v.Type.IndirectLevel() - typ.IndirectLevel()
				var facts []*FactPointTo
				if cg.FM != nil {
					facts = cg.FM.GlobalFacts
				}
				if IsPointingToLocals(v, cg.CurrentBlock(), indirection, facts) {
					continue
				}
			}
			// ExpressionVariable.cpp:118–119 — opportunistic_validate when FactMgr present
			if cg.FM != nil && v.Type.IndirectLevel() > typ.IndirectLevel() {
				if OpportunisticValidate(r, v, typ, cg.FM.GlobalFacts, vs.Opts.NullPointerDerefProb, vs.Opts.DeadPointerDerefProb) == 0 {
					continue
				}
			}
		}
		// Effect::read_var for selected variable
		cg.NoteRead(v)
		return &Expression{Term: TermVariable, Var: v, ExprType: typ}
	}
	return nil
}

// Output is a minimal C fragment (Expression::Output + optional cast).
func (e *Expression) Output() string {
	if e == nil {
		return ""
	}
	body := e.outputBody()
	if e.CastType != nil {
		return "(" + e.CastType.CName() + ")" + body
	}
	return body
}

func (e *Expression) outputBody() string {
	if e == nil {
		return ""
	}
	switch e.Term {
	case TermConstant:
		if e.Con != nil {
			return e.Con.Value
		}
	case TermVariable:
		if e.Var == nil {
			return ""
		}
		// ExpressionVariable::Output — *…var or &var from indirect level.
		ind := e.IndirectLevel()
		if ind > 0 {
			// deref uses actual name inside * (Variable::Output style)
			stars := strings.Repeat("*", ind)
			return "(" + stars + e.Var.Name + ")"
		}
		if ind < 0 {
			// address-of: mark addr taken; &name not ACCESS_ONCE
			e.Var.IsAddrTaken = true
			return "&" + e.Var.Name
		}
		return e.Var.OutputC()
	case TermFunction:
		if e.Invoke != nil {
			return e.Invoke.Output()
		}
	case TermAssignment:
		if e.Assign != nil {
			// ExpressionAssign::Output → (assign as expr)
			lhs := ""
			if e.Assign.ArrayAccess != "" {
				lhs = e.Assign.ArrayAccess
			} else if e.Assign.LhsVar != nil {
				lhs = e.Assign.LhsVar.OutputLhsC()
			}
			rhs := "0"
			if e.Assign.Expr != nil {
				rhs = e.Assign.Expr.Output()
			}
			if lhs != "" {
				if e.Assign.AssignOp.NeedNoRHS() {
					return "(" + e.Assign.AssignOp.AssignOpC(lhs, "") + ")"
				}
				return "(" + e.Assign.AssignOp.AssignOpC(lhs, rhs) + ")"
			}
		}
	case TermCommaExpr:
		l, r := "0", "0"
		if e.CommaLHS != nil {
			l = e.CommaLHS.Output()
		}
		if e.CommaRHS != nil {
			r = e.CommaRHS.Output()
		}
		return "(" + l + ", " + r + ")"
	}
	return "/*expr*/"
}

// makeExpressionFuncall mirrors ExpressionFuncall::make_random.
// ExpressionFuncall.cpp:66–102.
func makeExpressionFuncall(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	list *FunctionList,
) *Expression {
	if r == nil {
		return nil
	}
	probs := NewProbabilities(opts)
	stdFunc := ExpressionFunctionProbability(r, list, opts)
	// non-simple/void forces !std
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, typ, qfer, stdFunc)
	if fi == nil || fi.Failed {
		// replace with variable
		return makeExpressionVariable(r, vs, cg, typ, qfer)
	}
	return &Expression{Term: TermFunction, Invoke: fi}
}
