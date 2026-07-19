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

// ExpressionsComplete reports every Expression* is live (no nil holes).
// Note: ExpressionsComplete(nil)==true (complete empty). Fail-closed incomplete
// IndexExprs / arg lists must not invent visit success past a nil slot.
func ExpressionsComplete(exprs []*Expression) bool {
	for _, e := range exprs {
		if e == nil {
			return false
		}
	}
	return true
}

// IncompleteExpressions is the fail-closed incomplete Expression* list marker.
func IncompleteExpressions() []*Expression {
	return []*Expression{nil}
}

// CompatibleWithVar mirrors Expression::compatible(Variable*).
// ExpressionVariable.cpp:288–291 — var.compatible(v);
// ExpressionFuncall.cpp:206–207 — invoke.compatible(v) (unary operand only);
// default / other terms false.
func (e *Expression) CompatibleWithVar(v *Variable, expandStruct bool) bool {
	// Constant.cpp:489 / ExpressionVariable.cpp:289 — assert(v)
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
	case TermConstant:
		// Constant.cpp:488–493 — assert(v); expand_struct → true; else false
		// no soft invent field-var special cases beyond expand_struct
		return expandStruct
	default:
		return false
	}
}

// CompatibleWithExpr mirrors Expression::compatible(Expression*).
// ExpressionVariable.cpp:276–282 — exp.compatible(&var);
// ExpressionFuncall.cpp:210–212 — always false for Expression*.
// Incomplete Variable/Lhs shells fail closed false (no invent soft-skip nil Var).
func (e *Expression) CompatibleWithExpr(other *Expression, expandStruct bool) bool {
	// ExpressionVariable.cpp:277 — assert(exp); nil is broken IR
	if e == nil || other == nil {
		return false
	}
	// incomplete ExpressionVariable / Lhs — assert(var) path fails closed reject
	if (e.Term == TermVariable || e.Term == TermLhs) && e.Var == nil {
		return false
	}
	if (other.Term == TermVariable || other.Term == TermLhs) && other.Var == nil {
		return false
	}
	// Variable / Lhs → other.compatible(this.var)
	// ExpressionVariable.cpp:289 — assert(v) on Variable* overload via CompatibleWithVar
	if e.Term == TermVariable || e.Term == TermLhs {
		return other.CompatibleWithVar(e.Var, expandStruct)
	}
	if other.Term == TermVariable || other.Term == TermLhs {
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
// Incomplete IR fails closed nil (no invent ExprType shell without live invoke/assign/rhs).
func (e *Expression) GetTypeUncast() *Type {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermConstant:
		if e.Con != nil {
			return e.Con.Type
		}
		return nil
	case TermVariable, TermLhs:
		// ExpressionVariable always has live type; ExprType preferred then Var.Type
		if e.ExprType != nil {
			return e.ExprType
		}
		if e.Var != nil {
			return e.Var.Type
		}
		return nil
	case TermFunction:
		// ExpressionFuncall.cpp:122–124 — invoke.get_type(); no invent ExprType without invoke
		if e.Invoke == nil {
			return nil
		}
		return e.Invoke.GetType()
	case TermCommaExpr:
		// value type is RHS; incomplete without CommaRHS fails closed
		if e.CommaRHS == nil {
			return nil
		}
		return e.CommaRHS.GetTypeUncast()
	case TermAssignment:
		// ExpressionAssign::get_type — LHS type only; no invent ExprType without Assign
		if e.Assign == nil {
			return nil
		}
		if e.Assign.Lhs != nil {
			if t := e.Assign.Lhs.GetType(); t != nil {
				return t
			}
			// incomplete Lhs without type
			if e.Assign.Lhs.Var == nil {
				return nil
			}
		}
		if e.Assign.LhsVar != nil {
			return e.Assign.LhsVar.Type
		}
		return nil
	}
	return nil
}

// IndirectLevel mirrors ExpressionVariable::get_indirect_level.
// Incomplete expr IR returns 0 for the bit; callers that must not invent bare
// level-0 / non-address-of use IndirectLevelComplete.
func (e *Expression) IndirectLevel() int {
	n, ok := e.IndirectLevelComplete()
	if !ok {
		return 0
	}
	return n
}

// IndirectLevelComplete is get_indirect_level with ok=false on incomplete expr IR
// (no invent treat broken Variable/type as level 0 for transfer/visit/overlap).
func (e *Expression) IndirectLevelComplete() (n int, ok bool) {
	if e == nil || e.Var == nil || e.Var.Type == nil {
		return 0, false
	}
	want := e.ExprType
	if want == nil {
		want = e.Var.Type
	}
	if want == nil {
		return 0, false
	}
	return e.Var.Type.IndirectLevel() - want.IndirectLevel(), true
}

// GetType mirrors Expression::get_type.
// ExpressionFuncall.cpp:122–124 — invoke.get_type();
// ExpressionAssign — LHS type; Constant / Variable as typed.
// Incomplete IR fails closed nil (no invent type shell / panic on nil CommaRHS).
func (e *Expression) GetType() *Type {
	if e == nil {
		return nil
	}
	if e.CastType != nil {
		return e.CastType
	}
	return e.GetTypeUncast()
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
		// incomplete type IR must not invent storage-level quals via level 0
		n, ok := e.IndirectLevelComplete()
		if !ok {
			return CVQualifiers{}
		}
		return e.Var.Qfer.IndirectQualifiers(n)
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
// Incomplete IR fails closed false (no invent fold / panic on nil CommaRHS/Assign.Expr).
func (e *Expression) EqualsInt(num int) bool {
	if e == nil {
		return false
	}
	switch e.Term {
	case TermConstant:
		return e.Con != nil && e.Con.Equals(num)
	case TermFunction:
		// ExpressionFuncall always has live invoke for fold
		if e.Invoke == nil {
			return false
		}
		return e.Invoke.EqualsInt(num)
	case TermCommaExpr:
		// comma value is RHS; incomplete without RHS fails closed
		if e.CommaRHS == nil {
			return false
		}
		return e.CommaRHS.EqualsInt(num)
	case TermAssignment:
		// ExpressionAssign::equals — simple assign && expr.equals(num)
		if e.Assign == nil || e.Assign.AssignOp != AssignSimple {
			return false
		}
		if e.Assign.Expr == nil {
			return false
		}
		return e.Assign.Expr.EqualsInt(num)
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
// Incomplete IR fails closed as true (uses v) — no invent conflict-free non-use.
func (e *Expression) UseVar(v *Variable) bool {
	if e == nil || v == nil {
		return false
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		if e.Var == nil {
			return true
		}
		return e.Var == v || e.Var.Match(v)
	case TermFunction:
		// ExpressionFuncall always has live invoke + args after ERROR_GUARD
		if e.Invoke == nil {
			return true
		}
		for _, a := range e.Invoke.Args {
			if a == nil {
				return true
			}
			if a.UseVar(v) {
				return true
			}
		}
		return false
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return true
		}
		return e.CommaLHS.UseVar(v) || e.CommaRHS.UseVar(v)
	case TermAssignment:
		if e.Assign == nil {
			return true
		}
		if e.Assign.LhsVar != nil && (e.Assign.LhsVar == v || e.Assign.LhsVar.Match(v)) {
			return true
		}
		if e.Assign.Lhs != nil {
			if e.Assign.Lhs.Var == nil {
				return true
			}
			if e.Assign.Lhs.Var == v || e.Assign.Lhs.Var.Match(v) {
				return true
			}
		}
		if e.Assign.Expr == nil {
			return true
		}
		return e.Assign.Expr.UseVar(v)
	case TermLhs:
		// Lhs as expression term if ever used
		if e.Var == nil {
			return true
		}
		return e.Var == v || e.Var.Match(v)
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
	// Expression.cpp:107–111 — assert(filter); ERROR_GUARD(MAX_TERM_TYPES); no soft invent eVariable
	if r == nil || filter == nil {
		return MaxTermTypes
	}
	i := r.RndUptoFilter(uint32(filter.MaxProb()), filter)
	return TermType(filter.Lookup(int(i)))
}

// PickTermType builds default filters for Expression::make_random when tt==MAX.
// Expression.cpp:160–179 (subset: no_func, no_const, depth).
func PickTermType(r *Rng, tables *ExprTables, opts Options, typ *Type, noFunc, noConst bool, exprDepth int) TermType {
	// Expression::InitProbabilityTables always live; process tables if arg nil
	if tables == nil {
		tables = ProcessExprTables()
	}
	if tables == nil {
		return MaxTermTypes
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
	// Expression::InitProbabilityTables always live; process tables if arg nil
	if tables == nil {
		tables = ProcessExprTables()
	}
	if tables == nil {
		return MaxTermTypes
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
// cg is *CGContext so visit_facts effects persist (C++ CGContext&).
func MakeRandomParam(
	r *Rng,
	opts Options,
	tables *ExprTables,
	vs *VariableSelector,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
	exprDepth int,
	list ...*FunctionList,
) *Expression {
	// Expression.cpp always has RNG; no invent param expression without it
	if r == nil {
		return nil
	}
	// incomplete ambient fails closed sticky when live cg (no invent param past holes)
	if cg != nil {
		if !EffectComplete(cg.EffectContext()) ||
			(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
			!EffectComplete(cg.EffectStm) {
			SetError(ErrGeneric)
			return nil
		}
		if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
	}
	// Expression.cpp:241–242 — assert(type); DEPTH_GUARD after type known
	if typ == nil {
		return nil
	}
	// Expression.cpp:242–243 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtExpressionRandomParam, tt, …)
	// term type not chosen yet when MAX; guard uses flag 0 until Pick (C++ uses tt arg)
	if DepthGuardByTypeFlag(opts, DtExpressionRandomParam, int(MaxTermTypes)) == BadDepth {
		return nil
	}
	// Expression.cpp:258 — use cg_context.expr_depth (exprDepth param kept for API)
	depth := exprDepth
	if cg != nil {
		depth = cg.ExprDepth
	}
	tt := PickParamTermType(r, tables, opts, typ, depth)
	// Expression.cpp:264 — ERROR_GUARD(nullptr) after ExpressionTypeProbability
	if HasError() {
		return nil
	}
	// Expression.cpp:258–260 — depth filtered only in PickParamTermType; no soft invent leaf
	if tt == MaxTermTypes {
		return nil
	}
	// ExpressionVariable::make_random(..., as_param=true)
	if tt == TermVariable {
		// Expression.cpp:291–294 — depth++ for variable (make_random_param path)
		e := makeExpressionVariableFlags(r, vs, cg, typ, qfer, true, false)
		// Expression.cpp:293 — ERROR_GUARD before return
		if HasError() {
			return nil
		}
		if e != nil && cg != nil && BumpsExprDepth(e) {
			cg.ExprDepth++
		}
		return e
	}
	// no_func=false, no_const=true for other terms (MakeRandomExpression bumps depth)
	return MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, false, true, tt, depth, list...)
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
// cg is *CGContext so ExpressionVariable::visit_facts mutates the caller's
// context (C++ CGContext&); by-value dropped EffectStm / merge_param updates.
func MakeRandomExpression(
	r *Rng,
	opts Options,
	tables *ExprTables,
	vs *VariableSelector,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
	noFunc, noConst bool,
	tt TermType,
	exprDepth int,
	list ...*FunctionList,
) *Expression {
	// Expression.cpp always has RNG + live CGContext; no invent leaf shells without them
	if r == nil || cg == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent leaf / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// Expression.cpp:144–145 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtExpression, tt, nullptr)
	if DepthGuardByTypeFlag(opts, DtExpression, int(tt)) == BadDepth {
		return nil
	}
	// Expression::InitProbabilityTables — process tables when caller omits session tables
	if tables == nil {
		tables = ProcessExprTables()
	}
	if tables == nil {
		// no soft invent NewExprTables mid expression
		return nil
	}
	var flist *FunctionList
	if len(list) > 0 {
		flist = list[0]
	}
	if flist == nil {
		flist = cg.Funcs
	}

	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	var probs *Probabilities
	if vs != nil {
		probs = vs.Probs
	}
	env := cg.Types
	if env == nil && vs != nil {
		env = vs.Types
	}

	// Expression.cpp:147–153 — type==nullptr → choose_random_nonvoid(_nonvolatile)
	// based on effect_context purity; re-roll if struct + Constant want.
	// C++ always has AllTypes from GenerateSimpleTypes; no GetSimpleType invent.
	if typ == nil {
		if env == nil || len(env.AllTypes) == 0 {
			return nil
		}
		seFree := cg.EffectContext().IsSideEffectFree()
		for tries := 0; tries < 256; tries++ {
			if seFree {
				typ = env.ChooseRandomNonvoid(r, opts, probs)
			} else {
				typ = env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
			}
			// Expression.cpp: ERROR_GUARD paths inside choose_random*
			if HasError() {
				return nil
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
			return nil
		}
	}
	// Expression.cpp:154–157 — asserts on illegal term/type; fail closed (no soft invent rewrite)
	// no_func && eFunction / no_const && eConstant / struct && eConstant
	if noFunc && tt == TermFunction {
		return nil
	}
	if noConst && tt == TermConstant {
		return nil
	}
	if typ != nil && typ.IsStruct() && tt == TermConstant {
		// was soft invent TermVariable — C++ assert, not rewrite
		return nil
	}
	// Expression.cpp:176–178 / 213 — always cg_context.expr_depth (not a separate local)
	_ = exprDepth
	depth := cg.ExprDepth
	if tt == MaxTermTypes {
		tt = PickTermType(r, tables, opts, typ, noFunc, noConst, depth)
	}
	// Expression.cpp:182 — ERROR_GUARD(nullptr) after term pick
	if HasError() {
		return nil
	}
	// Expression.cpp:177–178 — depth only via filter in PickTermType; ERROR_GUARD if MAX
	// no soft invent TermVariable/Constant leaf when depth high
	if tt == MaxTermTypes {
		return nil
	}
	var e *Expression
	switch tt {
	case TermConstant:
		// Expression.cpp:185–188 — assert simple != eVoid
		if typ != nil && typ.IsSimple() && typ.Simple() == EVoid {
			return nil
		}
		// Expression.cpp:188 — Constant::make_random; ERROR_GUARD after
		// no invent TermConstant shell with nil Con when make_random fails
		con := MakeRandom(typ, opts, probs, r)
		if con == nil || HasError() {
			return nil
		}
		e = &Expression{Term: TermConstant, Con: con}
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
	// Expression.cpp:213–218 — depth++ for Constant, Variable, or user FuncCall
	// so siblings (comma/binary/params) see raised expr_depth via same CGContext&.
	// Expression.cpp:217 — ERROR_GUARD(nullptr) before return
	if HasError() {
		return nil
	}
	if e == nil {
		// C++ would crash on e->term_type; factories return null only with error
		return nil
	}
	if BumpsExprDepth(e) {
		cg.ExprDepth++
	}
	return e
}

// makeExpressionVariable — ExpressionVariable.cpp:56+ :
// VariableSelector::select(READ, type, qfer, eFlexible); as_param / as_return filters.
// cg is *CGContext (C++ CGContext&) so visit_facts writes stick for merge_param_context.
func makeExpressionVariable(r *Rng, vs *VariableSelector, cg *CGContext, typ *Type, qfer *CVQualifiers) *Expression {
	return makeExpressionVariableFlags(r, vs, cg, typ, qfer, false, false)
}

// makeExpressionVariableFlags adds as_param / as_return restrictions (ExpressionVariable.cpp:56–142).
func makeExpressionVariableFlags(
	r *Rng,
	vs *VariableSelector,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
	asParam, asReturn bool,
) *Expression {
	// ExpressionVariable.cpp always has RNG + live selector/context + Type*
	// no invent var shell without them; nil typ must not soft-skip type filters
	if r == nil || vs == nil || cg == nil || typ == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent var expr / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// ExpressionVariable.cpp:61 — DEPTH_GUARD_BY_TYPE_RETURN(dtExpressionVariable, nullptr)
	if DepthGuardByType(vs.Opts, DtExpressionVariable) == BadDepth {
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
	// C++ loops until success or ERROR_GUARD; cap high to avoid soft invent nil early
	var dummy []*Variable
	for tries := 0; tries < 256; tries++ {
		// ExpressionVariable.cpp:74–76 — select_must_use_var READ first
		v := vs.SelectMustUseVar(r, AccessRead, *cg, typ, qfer)
		if v == nil {
			// ExpressionVariable.cpp:77–78 — select(..., dummy, eFlexible)
			v = vs.SelectWithInvalid(AccessRead, *cg, typ, qfer, r, MatchFlexible, dummy)
		}
		if v == nil {
			// ERROR_GUARD from select; sticky error aborts like C++
			if HasError() {
				break
			}
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
		// Variable::type always live; incomplete type IR fails closed sticky (no invent
		// soft re-pick past hole candidate as if absent)
		if v.Type == nil {
			SetError(ErrGeneric)
			break
		}
		// ExpressionVariable.cpp:93–94 — no float var for non-float want
		if !typ.IsFloat() && v.Type.IsFloat() {
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:97–100 — as_param forbid address-of argument
		// C++: var->type->is_dereferenced_from(type)  (want = type, take &)
		if asParam && v.IsArgument() && v.Type.IsDereferencedFrom(typ) {
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:101–105 — !addr_taken_of_locals: forbid & local/arg
		if !vs.Opts.AddrTakenOfLocals && (v.IsArgument() || v.IsLocal()) &&
			v.Type.IsDereferencedFrom(typ) {
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:111–115 — as_return + no_return_dead_ptr
		if asReturn && vs.Opts.NoReturnDeadPointer {
			indirection := v.Type.IndirectLevel() - typ.IndirectLevel()
			var facts []*FactPointTo
			if cg.FM != nil {
				// incomplete GlobalFacts fail closed sticky (no invent soft-skip local-ptr filter)
				if !FactsComplete(cg.FM.GlobalFacts) {
					SetError(ErrGeneric)
					break
				}
				facts = cg.FM.GlobalFacts
			}
			if IsPointingToLocals(v, cg.CurrentBlock(), indirection, facts) {
				dummy = append(dummy, v)
				continue
			}
		}
		// ExpressionVariable.cpp:116–119 — opportunistic_validate (always; empty facts if no FM)
		var facts []*FactPointTo
		if cg.FM != nil {
			if !FactsComplete(cg.FM.GlobalFacts) {
				SetError(ErrGeneric)
				break
			}
			facts = cg.FM.GlobalFacts
		}
		if OpportunisticValidate(r, v, typ, facts, vs.Opts.NullPointerDerefProb, vs.Opts.DeadPointerDerefProb) == 0 {
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:80 ERROR_GUARD after select (sticky)
		if HasError() {
			break
		}
		// ExpressionVariable.cpp:120–124 — visit_facts with (var, type); on success
		// use ExpressionVariable(*var) when indirection==0 else (*var, type)
		// C++ always has FactMgr; visit_facts records reads even with empty fact vec
		probe := &Expression{Term: TermVariable, Var: v, ExprType: typ}
		if !cg.VisitFactsExpressionVariable(probe, vs.Opts) {
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:122–123
		// VisitFactsExpressionVariable already required complete probe type
		ev := probe
		if ind0, iok := probe.IndirectLevelComplete(); iok && ind0 == 0 {
			ev = &Expression{Term: TermVariable, Var: v, ExprType: v.Type}
		}
		// ExpressionVariable.cpp:137–142 — bookkeeping on successful make
		deref, _ := ev.IndirectLevelComplete()
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
		// Expression.cpp:228–231 — cast_type->Output + body; both always live
		// no invent "() body" / "(type) " empty body
		cn := e.CastType.CName()
		if cn == "" || body == "" {
			return ""
		}
		return "(" + cn + ") " + body
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
		// Constant::Output always live value; no invent empty token for incomplete Constant
		if e.Con != nil && e.Con.Value != "" {
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
		// ExpressionComma.cpp:137–144 — "(" + lhs + " , " + rhs + ")"
		// C++ always has live lhs/rhs Output; no invent "( , )" / "(x , )" / "( , y)"
		if e.CommaLHS == nil || e.CommaRHS == nil {
			return ""
		}
		lhs := e.CommaLHS.Output()
		rhs := e.CommaRHS.Output()
		if lhs == "" || rhs == "" {
			// incomplete side IR — fail closed whole comma
			return ""
		}
		return "(" + lhs + " , " + rhs + ")"
	}
	// Expression.cpp:195–200 default: no emit invent; incomplete IR → empty
	return ""
}

// makeExpressionFuncall mirrors ExpressionFuncall::make_random.
// ExpressionFuncall.cpp:66–102.
// cg is *CGContext so failed-invoke EffectStm restore and var fallback visit_facts persist.
func makeExpressionFuncall(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
	qfer *CVQualifiers,
	list *FunctionList,
) *Expression {
	if r == nil || cg == nil {
		return nil
	}
	// ExpressionFuncall.cpp:75 — get_fact_mgr always live; fail closed without invent
	if cg.FM == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent funcall / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	// ExpressionFuncall.cpp:66+ — no DEPTH_GUARD here (guard is on Expression::make_random)
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	var probs *Probabilities
	if vs != nil {
		probs = vs.Probs
	}
	stdFunc := ExpressionFunctionProbability(r, list, opts)
	// ExpressionFuncall.cpp:71–73 — unary/binary only for non-void simple types
	if typ != nil && (!typ.IsSimple() || typ.Simple() == EVoid) {
		stdFunc = false
	}
	// ExpressionFuncall.cpp:75–78 — snapshot effects and facts for failed invocation
	var preAccum Effect
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
	}
	preStm := cg.EffectStm.Clone()
	// incomplete GlobalFacts fail closed sticky (no invent cleaned snapshot for failed call restore)
	if !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	factsCopy := CloneFactSlice(cg.FM.GlobalFacts)
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, typ, qfer, stdFunc)
	// ExpressionFuncall.cpp:82 — ERROR_GUARD(nullptr) before fi->failed
	if HasError() {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFacts(factsCopy)
		return nil
	}
	// FunctionInvocation.cpp:119 assert(fi != 0); nil without Failed is incomplete
	if fi == nil {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFacts(factsCopy)
		return nil
	}
	if fi.Failed {
		// ExpressionFuncall.cpp:84–91 — restore env; replace with simple var
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFacts(factsCopy)
		return makeExpressionVariable(r, vs, cg, typ, qfer)
	}
	return &Expression{Term: TermFunction, Invoke: fi}
}
