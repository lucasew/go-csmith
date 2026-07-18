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

// CompatibleWithVar mirrors Expression::compatible(Variable*).
// ExpressionVariable.cpp:288–291 — var.compatible(v);
// ExpressionFuncall.cpp:206–207 — invoke.compatible(v) (unary operand only);
// default / other terms false.
func (e *Expression) CompatibleWithVar(v *Variable, expandStruct bool) bool {
	if e == nil || v == nil {
		return false
	}
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			return false
		}
		return e.Var.Compatible(v, expandStruct)
	case TermFunction:
		// ExpressionFuncall.cpp:206–207
		return e.Invoke != nil && e.Invoke.CompatibleVar(v, expandStruct)
	case TermLhs:
		if e.Var == nil {
			return false
		}
		return e.Var.Compatible(v, expandStruct)
	default:
		return false
	}
}

// CompatibleWithExpr mirrors Expression::compatible(Expression*).
// ExpressionVariable.cpp:276–282 — exp.compatible(&var);
// ExpressionFuncall.cpp:210–212 — always false for Expression*.
func (e *Expression) CompatibleWithExpr(other *Expression, expandStruct bool) bool {
	if e == nil || other == nil {
		return false
	}
	// Variable / Lhs → other.compatible(this.var)
	if (e.Term == TermVariable || e.Term == TermLhs) && e.Var != nil {
		return other.CompatibleWithVar(e.Var, expandStruct)
	}
	if (other.Term == TermVariable || other.Term == TermLhs) && other.Var != nil {
		return e.CompatibleWithVar(other.Var, expandStruct)
	}
	// Funcall::compatible(Expression*) is false (ExpressionFuncall.cpp:210–212)
	return false
}

// CheckAndSetCast mirrors Expression::check_and_set_cast.
// Expression.cpp:222–226 — lang_cpp or needs_cast; we apply when desired type needs cast.
// CheckAndSetCast mirrors Expression::check_and_set_cast without lang_cpp gate
// (tests / call sites that already decided a cast may be needed).
// Expression.cpp:221–225 — get_type().needs_cast(desired) → cast_type = desired.
func (e *Expression) CheckAndSetCast(desired *Type) {
	e.checkAndSetCastCore(desired)
}

// CheckAndSetCastOpts mirrors Expression::check_and_set_cast fully.
// Expression.cpp:221–225 — only when CGOptions::lang_cpp().
func (e *Expression) CheckAndSetCastOpts(desired *Type, opts Options) {
	if !opts.LangCPP {
		return
	}
	e.checkAndSetCastCore(desired)
}

func (e *Expression) checkAndSetCastCore(desired *Type) {
	if e == nil || desired == nil {
		return
	}
	// Expression.cpp:222 — get_type() before cast is applied
	src := e.GetTypeUncast()
	if src != nil && src.NeedsCast(desired) {
		e.CastType = desired
	}
}

// GetTypeUncast is get_type ignoring cast_type (for check_and_set_cast).
func (e *Expression) GetTypeUncast() *Type {
	if e == nil {
		return nil
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
		if e.Invoke != nil {
			return e.Invoke.GetType()
		}
		if e.ExprType != nil {
			return e.ExprType
		}
	case TermCommaExpr:
		if e.CommaRHS != nil {
			return e.CommaRHS.GetTypeUncast()
		}
	case TermAssignment:
		if e.Assign != nil {
			if e.Assign.Lhs != nil {
				if t := e.Assign.Lhs.GetType(); t != nil {
					return t
				}
			}
			if e.Assign.LhsVar != nil {
				return e.Assign.LhsVar.Type
			}
		}
		if e.ExprType != nil {
			return e.ExprType
		}
	}
	return nil
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

// GetType mirrors Expression::get_type.
// ExpressionFuncall.cpp:122–124 — invoke.get_type();
// ExpressionAssign — LHS type; Constant / Variable as typed.
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
		// ExpressionFuncall.cpp:122–124
		if e.Invoke != nil {
			return e.Invoke.GetType()
		}
		if e.ExprType != nil {
			return e.ExprType
		}
	case TermCommaExpr:
		return e.CommaRHS.GetType()
	case TermAssignment:
		// ExpressionAssign::get_type — LHS type
		if e.Assign != nil {
			if e.Assign.Lhs != nil {
				if t := e.Assign.Lhs.GetType(); t != nil {
					return t
				}
			}
			if e.Assign.LhsVar != nil {
				return e.Assign.LhsVar.Type
			}
		}
		if e.ExprType != nil {
			return e.ExprType
		}
	}
	return nil
}

// GetQualifiers mirrors Expression::get_qualifiers.
// ExpressionVariable.cpp:194–196; ExpressionAssign.cpp:85–86;
// ExpressionFuncall.cpp:187–188; ExpressionComma / Constant default empty.
func (e *Expression) GetQualifiers() CVQualifiers {
	if e == nil {
		return CVQualifiers{}
	}
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			return CVQualifiers{}
		}
		// ExpressionVariable::get_qualifiers — qfer.indirect_qualifiers(indirect)
		return e.Var.Qfer.IndirectQualifiers(e.IndirectLevel())
	case TermAssignment:
		if e.Assign != nil && e.Assign.Lhs != nil {
			return e.Assign.Lhs.GetQualifiers()
		}
		if e.Assign != nil && e.Assign.LhsVar != nil {
			return e.Assign.LhsVar.Qfer
		}
	case TermFunction:
		if e.Invoke != nil {
			return e.Invoke.GetQualifiers()
		}
	case TermCommaExpr:
		// ExpressionComma has no override in header → default empty-ish;
		// value type is RHS — use RHS qualifiers when present.
		if e.CommaRHS != nil {
			return e.CommaRHS.GetQualifiers()
		}
	}
	return CVQualifiers{}
}

// EqualsInt mirrors Expression::equals(int).
// Expression.h: equals default false; Constant; ExpressionFuncall via invoke.
func (e *Expression) EqualsInt(num int) bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermConstant:
		return e.Con != nil && e.Con.Equals(num)
	case TermFunction:
		return e.Invoke != nil && e.Invoke.EqualsInt(num)
	case TermCommaExpr:
		// comma value is RHS
		return e.CommaRHS.EqualsInt(num)
	case TermAssignment:
		// ExpressionAssign::equals — simple assign && expr.equals(num)
		if e.Assign != nil && e.Assign.AssignOp == AssignSimple {
			return e.Assign.Expr.EqualsInt(num)
		}
	}
	return false
}

// NotEquals mirrors Expression::not_equals(int).
// Expression.h:139 — default false; Constant: !equals(num).
func (e *Expression) NotEquals(num int) bool {
	if e == nil || e.Term != TermConstant || e.Con == nil {
		return false
	}
	return e.Con.NotEquals(num)
}

// LessThan mirrors Expression::less_than(int).
// Expression.h default false; Constant.cpp:501–502.
func (e *Expression) LessThan(num int) bool {
	if e == nil || e.Term != TermConstant || e.Con == nil {
		return false
	}
	return e.Con.LessThan(num)
}

// Is0Or1 mirrors Expression::is_0_or_1.
// ExpressionFuncall → invoke; ExpressionComma → rhs; ExpressionAssign → simple+rhs.
func (e *Expression) Is0Or1() bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermFunction:
		return e.Invoke != nil && e.Invoke.Is0Or1()
	case TermCommaExpr:
		return e.CommaRHS != nil && e.CommaRHS.Is0Or1()
	case TermAssignment:
		// ExpressionAssign.cpp:103–104
		if e.Assign != nil && e.Assign.AssignOp == AssignSimple {
			return e.Assign.Expr != nil && e.Assign.Expr.Is0Or1()
		}
	}
	return false
}

// UseVar mirrors Expression::use_var.
// Expression.h:143 default false; Variable/Funcall/Comma/Assign overrides.
func (e *Expression) UseVar(v *Variable) bool {
	if e == nil || v == nil {
		return false
	}
	switch e.Term {
	case TermVariable:
		return e.Var == v || (e.Var != nil && e.Var.Match(v))
	case TermFunction:
		if e.Invoke == nil {
			return false
		}
		for _, a := range e.Invoke.Args {
			if a != nil && a.UseVar(v) {
				return true
			}
		}
		return false
	case TermCommaExpr:
		return e.CommaLHS.UseVar(v) || e.CommaRHS.UseVar(v)
	case TermAssignment:
		if e.Assign == nil {
			return false
		}
		if e.Assign.LhsVar != nil && (e.Assign.LhsVar == v || e.Assign.LhsVar.Match(v)) {
			return true
		}
		if e.Assign.Lhs != nil && e.Assign.Lhs.Var != nil &&
			(e.Assign.Lhs.Var == v || e.Assign.Lhs.Var.Match(v)) {
			return true
		}
		return e.Assign.Expr.UseVar(v)
	case TermLhs:
		// Lhs as expression term if ever used
		return e.Var == v || (e.Var != nil && e.Var.Match(v))
	default:
		return false
	}
}

// ToString mirrors Expression::to_string — Output without stream.
// Expression.cpp:120–124.
func (e *Expression) ToString() string {
	if e == nil {
		return ""
	}
	return e.Output()
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
// Expression.cpp:103–112 — PartialExpander force eFunction when invoke expand;
// else rnd_upto(filter.max, filter); lookup → eTermType.
func ExpressionTypeProbability(r *Rng, filter *VectorFilter) TermType {
	// Expression.cpp:104–105 — PartialExpander::direct_expand_check(eInvoke)
	if DirectExpandCheck(StmtInvoke) {
		return TermFunction
	}
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

// BumpsExprDepth reports whether this expression increments cg_context.expr_depth.
// Expression.cpp:213–218 — Constant, Variable, or user FunctionInvocation.
func BumpsExprDepth(e *Expression) bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermConstant, TermVariable:
		return true
	case TermFunction:
		return e.Invoke != nil && e.Invoke.User != nil && !e.Invoke.IsStd
	default:
		return false
	}
}

// MakeRandomExpression mirrors Expression::make_random (const/var/funcall).
// Expression.cpp:141–219.
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

	probs := NewProbabilities(opts)
	if vs != nil && vs.Probs != nil {
		probs = vs.Probs
	}
	env := cg.Types
	if env == nil && vs != nil {
		env = vs.Types
	}

	// Expression.cpp:147–153 — type==nullptr → choose_random_nonvoid(_nonvolatile)
	// based on effect_context purity; re-roll if struct + Constant want.
	if typ == nil {
		seFree := cg.EffectContext().IsSideEffectFree()
		for tries := 0; tries < 16; tries++ {
			if env != nil && len(env.AllTypes) > 0 {
				if seFree {
					typ = env.ChooseRandomNonvoid(r, opts, probs)
				} else {
					typ = env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
				}
			} else {
				typ = GetSimpleType(ChooseRandomNonvoidSimple(r, probs))
			}
			// Expression.cpp:151–152 — constant structs not as subexpression
			if typ != nil && typ.IsStruct() && tt == TermConstant {
				continue
			}
			if typ != nil {
				break
			}
		}
		if typ == nil {
			typ = GetIntType()
		}
	}
	// Expression.cpp:155 — constant struct not allowed as term type
	if typ.IsStruct() && tt == TermConstant {
		tt = TermVariable
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
	var e *Expression
	switch tt {
	case TermConstant:
		// Expression.cpp:185–188
		e = &Expression{Term: TermConstant, Con: MakeRandom(typ, opts, r)}
	case TermVariable:
		// ExpressionVariable::make_random
		e = makeExpressionVariable(r, vs, cg, typ, qfer)
	case TermFunction:
		// ExpressionFuncall::make_random
		e = makeExpressionFuncall(r, opts, vs, tables, cg, typ, qfer, flist)
	case TermAssignment:
		// ExpressionAssign::make_random
		e = MakeExpressionAssign(r, opts, probs, vs, tables, cg, typ, qfer)
	case TermCommaExpr:
		// ExpressionComma::make_random
		e = MakeExpressionComma(r, opts, probs, vs, tables, cg, typ, qfer)
	default:
		return nil
	}
	// Expression.cpp:213–218 — depth++ for leaves / user calls
	// (callers that share exprDepth across siblings should also use BumpsExprDepth)
	_ = BumpsExprDepth(e)
	return e
}

// makeExpressionVariable — ExpressionVariable.cpp:56+ :
// VariableSelector::select(READ, type, qfer, eFlexible); as_param / as_return filters.
func makeExpressionVariable(r *Rng, vs *VariableSelector, cg CGContext, typ *Type, qfer *CVQualifiers) *Expression {
	return makeExpressionVariableFlags(r, vs, cg, typ, qfer, false, false)
}

// makeExpressionVariableFlags adds as_param / as_return restrictions (ExpressionVariable.cpp:56–142).
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
	// ExpressionVariable.cpp:67–69 — snapshot effects for visit_facts failure restore
	var preAccum, preStm Effect
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
	}
	preStm = cg.EffectStm.Clone()
	// ExpressionVariable.cpp:71–132 — do { select; filters; visit_facts } while (!ev)
	// dummy is invalid_vars passed into select (ExpressionVariable.cpp:78, 131)
	var dummy []*Variable
	for tries := 0; tries < 24; tries++ {
		// ExpressionVariable.cpp:74–76 — select_must_use_var READ first
		v := vs.SelectMustUseVar(r, AccessRead, cg, typ, qfer)
		if v == nil {
			// ExpressionVariable.cpp:77–78 — select(..., dummy, eFlexible)
			v = vs.SelectWithInvalid(AccessRead, cg, typ, qfer, r, MatchFlexible, dummy)
		}
		if v == nil {
			// C++ continues the loop; we give up after tries
			continue
		}
		// already in dummy should be rare when SelectWithInvalid works; keep guard
		skip := false
		for _, d := range dummy {
			if d == v {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if typ != nil && v.Type != nil {
			// ExpressionVariable.cpp:93–94 — no float var for non-float want
			if !typ.IsFloat() && v.Type.IsFloat() {
				dummy = append(dummy, v)
				continue
			}
			// as_param: forbid address-of argument (ExpressionVariable.cpp:97–100)
			if asParam && v.IsArgument() && typ.IsDereferencedFrom(v.Type) {
				dummy = append(dummy, v)
				continue
			}
			// !addr_taken_of_locals: forbid & of local/arg (ExpressionVariable.cpp:101–105)
			if !vs.Opts.AddrTakenOfLocals && (v.IsArgument() || v.IsLocal()) &&
				typ.IsDereferencedFrom(v.Type) {
				dummy = append(dummy, v)
				continue
			}
			// ExpressionVariable.cpp:111–115 — as_return + no_return_dead_ptr
			if asReturn && vs.Opts.NoReturnDeadPointer {
				indirection := v.Type.IndirectLevel() - typ.IndirectLevel()
				var facts []*FactPointTo
				if cg.FM != nil {
					facts = cg.FM.GlobalFacts
				}
				if IsPointingToLocals(v, cg.CurrentBlock(), indirection, facts) {
					dummy = append(dummy, v)
					continue
				}
			}
			// ExpressionVariable.cpp:116–119 — opportunistic_validate
			if cg.FM != nil {
				if OpportunisticValidate(r, v, typ, cg.FM.GlobalFacts, vs.Opts.NullPointerDerefProb, vs.Opts.DeadPointerDerefProb) == 0 {
					dummy = append(dummy, v)
					continue
				}
			}
		}
		// ExpressionVariable.cpp:120–124 — visit_facts with (var, type); on success
		// use ExpressionVariable(*var) when indirection==0 else (*var, type)
		probe := &Expression{Term: TermVariable, Var: v, ExprType: typ}
		if cg.FM != nil {
			cgp := &cg
			if !cgp.VisitFactsExpressionVariable(probe, vs.Opts) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				dummy = append(dummy, v)
				continue
			}
		} else {
			cg.NoteRead(v)
		}
		// ExpressionVariable.cpp:122–123
		ev := probe
		if probe.IndirectLevel() == 0 {
			ev = &Expression{Term: TermVariable, Var: v, ExprType: v.Type}
		}
		// ExpressionVariable.cpp:137–142 — bookkeeping on successful make
		deref := ev.IndirectLevel()
		if deref > 0 {
			IncrCounter(&readDereferenceCnts, deref)
		} else if deref < 0 {
			RecordAddressTaken(v)
		}
		RecordVolatileAccess(v, deref, false)
		return ev
	}
	return nil
}

// Output is a minimal C fragment (Expression::Output + optional cast).
// Expression.cpp:227–232 output_cast — "(type) " prefix when cast_type set.
func (e *Expression) Output() string {
	if e == nil {
		return ""
	}
	body := e.outputBody()
	if e.CastType != nil {
		// Expression.cpp:228–231 — "(" + type + ") " (space after close paren)
		return "(" + e.CastType.CName() + ") " + body
	}
	return body
}

// IndentedOutput mirrors Expression::indented_output.
// Expression.cpp:133–136 — output_tab(indent) + Output.
func (e *Expression) IndentedOutput(indent int) string {
	return OutputTab(indent) + e.Output()
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
		// ExpressionVariable.cpp:202–219 — base is Variable::Output (VOL_RVAL/ACCESS_ONCE).
		return outputExpressionVariable(e.Var, e.ExprType)
	case TermFunction:
		if e.Invoke != nil {
			return e.Invoke.Output()
		}
	case TermAssignment:
		if e.Assign != nil {
			// ExpressionAssign::Output → (assign as expr)
			wrap := e.Assign.LhsVar != nil && e.Assign.LhsVar.UseVolRVal
			as := OutputAssignAsExpr(e.Assign, wrap)
			if as != "" {
				return "(" + as + ")"
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
