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
	return e.CompatibleWithVarSess(nil, v, expandStruct)
}

func (e *Expression) CompatibleWithVarSess(s *Session, v *Variable, expandStruct bool) bool {
	// Constant.cpp:489 / ExpressionVariable.cpp:289 — assert(v)
	// Expression always live; sticky incomplete — fail closed false
	// (no invent soft-skip compatibility / soft re-pick past hole)
	if e == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*; incomplete sticky reject
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Var.Compatible(v, expandStruct)
		// residual ERROR sticky — no invent compatible true past Compatible residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermFunction:
		// ExpressionFuncall.cpp:206–207 — invoke always live
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Invoke.CompatibleVar(v, expandStruct)
		// residual ERROR sticky — no invent compatible true past CompatibleVar residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermLhs:
		// Lhs shell always has live Variable*
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Var.Compatible(v, expandStruct)
		// residual ERROR sticky — no invent compatible true past Compatible residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermConstant:
		// Constant.cpp:488–493 — assert(v); expand_struct → true; else false
		// incomplete Constant shell sticky (no invent expand_struct success past hole)
		if e.Con == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		return expandStruct
	default:
		return false
	}
}

// CompatibleWithExpr mirrors Expression::compatible(Expression*).
// ExpressionVariable.cpp:276–282 — exp.compatible(&var);
// ExpressionFuncall.cpp:210–212 — always false for Expression*.
// Incomplete Variable/Lhs shells fail closed sticky false (no invent soft-skip nil Var).}

func (e *Expression) CompatibleWithExpr(other *Expression, expandStruct bool) bool {
	return e.CompatibleWithExprSess(nil, other, expandStruct)
}

func (e *Expression) CompatibleWithExprSess(s *Session, other *Expression, expandStruct bool) bool {
	// ExpressionVariable.cpp:277 — assert(exp); nil is broken IR sticky
	if e == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// incomplete ExpressionVariable / Lhs — assert(var) path sticky reject
	if (e.Term == TermVariable || e.Term == TermLhs) && e.Var == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if (other.Term == TermVariable || other.Term == TermLhs) && other.Var == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// Variable / Lhs → other.compatible(this.var)
	// ExpressionVariable.cpp:289 — assert(v) on Variable* overload via CompatibleWithVar
	if e.Term == TermVariable || e.Term == TermLhs {
		ok := other.CompatibleWithVarSess(s, e.Var, expandStruct)
		// residual ERROR sticky — no invent compatible true past CompatibleWithVar residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	}
	if other.Term == TermVariable || other.Term == TermLhs {
		ok := e.CompatibleWithVarSess(s, other.Var, expandStruct)
		// residual ERROR sticky — no invent compatible true past CompatibleWithVar residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	}
	// Funcall::compatible(Expression*) is false (ExpressionFuncall.cpp:210–212)
	return false
}

// GetComplexity mirrors Expression::get_complexity via ExpressionComplexity.
// Incomplete IR sticky -1 (no invent leaf 0 past holes).}

func (e *Expression) GetComplexity() int {
	return ExpressionComplexity(e)
}

// GetInvoke mirrors Expression::get_invoke — non-nil only for TermFunction.
func (e *Expression) GetInvoke() *Invocation {
	return e.GetInvokeSess(nil)
}

// GetInvokeSess is GetInvoke with explicit session residual sticky.
func (e *Expression) GetInvokeSess(s *Session) *Invocation {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if e.Term != TermFunction {
		return nil
	}
	if e.Invoke == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return e.Invoke
}

// Clone mirrors Expression::clone for Constant / Variable / Lhs leaves.
// Compound terms fail closed sticky nil (no invent shallow shell without deep copy).
func (e *Expression) Clone() *Expression {
	return e.CloneSess(nil)
}

func (e *Expression) CloneSess(s *Session) *Expression {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	switch e.Term {
	case TermConstant:
		if e.Con == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		c := e.Con.CloneSess(s)
		if c == nil {
			return nil
		}
		return &Expression{Term: TermConstant, Con: c, CastType: e.CastType}
	case TermVariable, TermLhs:
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		return &Expression{Term: e.Term, Var: e.Var, ExprType: e.ExprType, CastType: e.CastType}
	default:
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// CheckAndSetCast mirrors Expression::check_and_set_cast.
// Expression.cpp:222–226 — lang_cpp or needs_cast; we apply when desired type needs cast.
// CheckAndSetCast mirrors Expression::check_and_set_cast without lang_cpp gate
// (tests / call sites that already decided a cast may be needed).
// Expression.cpp:221–225 — get_type().needs_cast(desired) → cast_type = desired.}

func (e *Expression) CheckAndSetCast(desired *Type) {
	e.CheckAndSetCastSess(nil, desired)
}

// CheckAndSetCastSess is CheckAndSetCast with explicit session residual sticky.
func (e *Expression) CheckAndSetCastSess(s *Session, desired *Type) {
	e.checkAndSetCastCoreSess(s, desired)
}

// CheckAndSetCastOpts mirrors Expression::check_and_set_cast fully.
// Expression.cpp:221–225 — only when CGOptions::lang_cpp().
func (e *Expression) CheckAndSetCastOpts(desired *Type, opts Options) {
	e.CheckAndSetCastOptsSess(nil, desired, opts)
}

// CheckAndSetCastOptsSess is CheckAndSetCastOpts with explicit session residual sticky.
func (e *Expression) CheckAndSetCastOptsSess(s *Session, desired *Type, opts Options) {
	if !opts.LangCPP {
		return
	}
	e.checkAndSetCastCoreSess(s, desired)
}

func (e *Expression) checkAndSetCastCore(desired *Type) {
	e.checkAndSetCastCoreSess(nil, desired)
}

func (e *Expression) checkAndSetCastCoreSess(s *Session, desired *Type) {
	// Expression + desired Type always live at check_and_set_cast; sticky no invent
	// skip-cast soft-success past hole.
	if e == nil || desired == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// Expression.cpp:222 — get_type() before cast is applied; incomplete type sticky
	// (no invent soft-skip cast decision past Type-nil shell as no-cast success)
	src := e.GetTypeUncastSess(s)
	if src == nil {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return
	}
	// residual ERROR sticky — no invent soft-continue NeedsCast past GetTypeUncast residual
	if sessHasError(s) {
		return
	}
	if src.NeedsCast(desired) {
		// residual ERROR sticky — no invent CastType past NeedsCast residual hole
		if sessHasError(s) {
			return
		}
		e.CastType = desired
		return
	}
	// residual ERROR sticky — no invent soft-continue no-cast past NeedsCast residual false
	if sessHasError(s) {
		return
	}
}

// GetTypeUncast is get_type ignoring cast_type (for check_and_set_cast).
// Incomplete IR fails closed nil (no invent ExprType shell without live invoke/assign/rhs).
func (e *Expression) GetTypeUncast() *Type {
	return e.GetTypeUncastSess(nil)
}

func (e *Expression) GetTypeUncastSess(s *Session) *Type {
	// Expression always live for get_type; sticky no invent type shell without it
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Type*; incomplete Con/Type sticky (no invent
		// untyped constant soft-miss / soft re-pick past hole)
		if e.Con == nil || e.Con.Type == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		return e.Con.Type
	case TermVariable, TermLhs:
		// ExpressionVariable always has live Variable*; incomplete sticky
		// (no invent type shell from ExprType alone without live Var)
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		// ExprType preferred (desired type after indirection) then Var.Type
		if e.ExprType != nil {
			return e.ExprType
		}
		if e.Var.Type != nil {
			return e.Var.Type
		}
		// incomplete Variable type IR sticky — no invent untyped variable
		sessNoteError(s, ErrGeneric)
		return nil
	case TermFunction:
		// ExpressionFuncall.cpp:122–124 — invoke.get_type(); sticky no invent without invoke
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ty := e.Invoke.GetTypeSess(s)
		// residual ERROR sticky — no invent invoke type past GetType residual hole
		if sessHasError(s) {
			return nil
		}
		return ty
	case TermCommaExpr:
		// value type is RHS; sticky incomplete without CommaRHS
		if e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ty := e.CommaRHS.GetTypeUncastSess(s)
		// residual ERROR sticky — no invent comma type past RHS GetType residual hole
		if sessHasError(s) {
			return nil
		}
		return ty
	case TermAssignment:
		// ExpressionAssign::get_type — LHS type only; sticky no invent without Assign
		if e.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if e.Assign.Lhs != nil {
			t := e.Assign.Lhs.GetTypeSess(s)
			// residual ERROR sticky — no invent assign type past Lhs GetType residual hole
			if sessHasError(s) {
				return nil
			}
			if t != nil {
				return t
			}
			// Lhs.GetType sticky incomplete already; re-assert sticky
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if e.Assign.LhsVar != nil && e.Assign.LhsVar.Type != nil {
			return e.Assign.LhsVar.Type
		}
		// incomplete assign LHS sticky
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// unknown term sticky incomplete
	sessNoteError(s, ErrGeneric)
	return nil
}

// IndirectLevel mirrors ExpressionVariable::get_indirect_level.
// Incomplete expr IR returns 0 for the bit; callers that must not invent bare
// level-0 / non-address-of use IndirectLevelComplete.}

func (e *Expression) IndirectLevel() int {
	n, ok := e.IndirectLevelComplete()
	if !ok {
		return 0
	}
	return n
}

// IndirectLevelComplete is get_indirect_level with ok=false on incomplete expr IR
// (no invent treat broken Variable/type as level 0 for transfer/visit/overlap).
// Incomplete shell sticky (callers that only use IndirectLevel still surface ERROR).
func (e *Expression) IndirectLevelComplete() (n int, ok bool) {
	return e.IndirectLevelCompleteSess(nil)
}

func (e *Expression) IndirectLevelCompleteSess(s *Session) (n int, ok bool) {
	// Expression Variable/Lhs always live with type; sticky incomplete no invent level 0
	if e == nil || e.Var == nil || e.Var.Type == nil {
		sessNoteError(s, ErrGeneric)
		return 0, false
	}
	want := e.ExprType
	if want == nil {
		want = e.Var.Type
	}
	if want == nil {
		sessNoteError(s, ErrGeneric)
		return 0, false
	}
	lv := e.Var.Type.IndirectLevelSess(s)
	// residual ERROR sticky — no invent level-0 past subject IndirectLevel residual
	if sessHasError(s) {
		return 0, false
	}
	lw := want.IndirectLevelSess(s)
	// residual ERROR sticky — no invent level-0 past desired IndirectLevel residual
	if sessHasError(s) {
		return 0, false
	}
	return lv - lw, true
}

// GetType mirrors Expression::get_type.
// ExpressionFuncall.cpp:122–124 — invoke.get_type();
// ExpressionAssign — LHS type; Constant / Variable as typed.
// Incomplete IR fails closed nil (no invent type shell / panic on nil CommaRHS).}

func (e *Expression) GetType() *Type {
	return e.GetTypeSess(nil)
}

func (e *Expression) GetTypeSess(s *Session) *Type {
	// Expression always live for get_type; sticky no invent type shell without it
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if e.CastType != nil {
		return e.CastType
	}
	ty := e.GetTypeUncastSess(s)
	// residual ERROR sticky — no invent type past GetTypeUncast residual hole
	if sessHasError(s) {
		return nil
	}
	return ty
}

// GetQualifiers mirrors Expression::get_qualifiers.
// ExpressionVariable.cpp:194–196; ExpressionAssign.cpp:85–86;
// ExpressionFuncall.cpp:187–188; ExpressionComma / Constant default empty.}

func (e *Expression) GetQualifiers() CVQualifiers {
	return e.GetQualifiersSess(nil)
}

func (e *Expression) GetQualifiersSess(s *Session) CVQualifiers {
	// Expression always live; sticky no invent empty quals shell without it
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return CVQualifiers{}
	}
	switch e.Term {
	case TermVariable:
		if e.Var == nil {
			// incomplete Variable IR sticky — no invent empty storage quals
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		// ExpressionVariable::get_qualifiers — qfer.indirect_qualifiers(indirect)
		// incomplete type IR must not invent storage-level quals via level 0
		n, ok := e.IndirectLevelCompleteSess(s)
		if !ok {
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		q := e.Var.Qfer.IndirectQualifiers(n)
		// residual ERROR sticky — no invent soft-quals past IndirectQualifiers residual
		if sessHasError(s) {
			return CVQualifiers{}
		}
		return q
	case TermAssignment:
		if e.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		if e.Assign.Lhs != nil {
			q := e.Assign.Lhs.GetQualifiersSess(s)
			// residual ERROR sticky — no invent assign quals past Lhs GetQualifiers residual
			if sessHasError(s) {
				return CVQualifiers{}
			}
			return q
		}
		if e.Assign.LhsVar != nil {
			return e.Assign.LhsVar.Qfer
		}
		// incomplete assign LHS sticky
		sessNoteError(s, ErrGeneric)
		return CVQualifiers{}
	case TermFunction:
		if e.Invoke == nil {
			// incomplete Funcall IR sticky — no invent empty invoke quals
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		q := e.Invoke.GetQualifiersSess(s)
		// residual ERROR sticky — no invent invoke quals past nested residual hole
		if sessHasError(s) {
			return CVQualifiers{}
		}
		return q
	case TermCommaExpr:
		// ExpressionComma value is RHS — sticky incomplete without RHS
		if e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		q := e.CommaRHS.GetQualifiers()
		// residual ERROR sticky — no invent comma quals past RHS residual hole
		if sessHasError(s) {
			return CVQualifiers{}
		}
		return q
	case TermConstant:
		// Constant default empty quals when Con live; incomplete shell sticky
		// (no invent complete empty-qfer success past missing Constant IR)
		if e.Con == nil {
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		return CVQualifiers{}
	}
	// unknown term sticky incomplete
	sessNoteError(s, ErrGeneric)
	return CVQualifiers{}
}

// EqualsInt mirrors Expression::equals(int).
// Expression.h: equals default false; Constant; ExpressionFuncall via invoke.
// Incomplete IR fails closed false (no invent fold / panic on nil CommaRHS/Assign.Expr).}

func (e *Expression) EqualsInt(num int) bool {
	return e.EqualsIntSess(nil, num)
}

func (e *Expression) EqualsIntSess(s *Session, num int) bool {
	// Expression always live for fold; sticky no invent "not equal" past missing shell
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Type* + Value; incomplete shell sticky
		// (no invent fold soft-success past Type-nil / empty-value via Con path alone)
		if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Con.EqualsSess(s, num)
		// residual ERROR sticky — no invent equal-true past Con.Equals residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermFunction:
		// ExpressionFuncall always has live invoke for fold
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Invoke.EqualsIntSess(s, num)
		// residual ERROR sticky — no invent equal-true past nested EqualsInt residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermCommaExpr:
		// comma value is RHS; sticky incomplete without RHS
		if e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.CommaRHS.EqualsIntSess(s, num)
		// residual ERROR sticky — no invent equal-true past RHS EqualsInt residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermAssignment:
		// ExpressionAssign::equals — simple assign && expr.equals(num)
		if e.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		if e.Assign.AssignOp != AssignSimple {
			return false
		}
		if e.Assign.Expr == nil {
			// incomplete assign RHS sticky — no invent not-equal fold
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Assign.Expr.EqualsIntSess(s, num)
		// residual ERROR sticky — no invent equal-true past assign RHS residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	}
	return false
}

// NotEquals mirrors Expression::not_equals(int).
// Expression.h:139 — default false; Constant: !equals(num).
// Incomplete Expression / Constant shell sticky false (no invent fold past holes).}

func (e *Expression) NotEquals(num int) bool {
	return e.NotEqualsSess(nil, num)
}

func (e *Expression) NotEqualsSess(s *Session, num int) bool {
	// Expression always live for fold; sticky no invent "equals" past missing shell
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.Term != TermConstant {
		// Expression.h default false for non-Constant
		return false
	}
	// Constant Type* + Value always live; incomplete sticky (no invent fold past hole)
	if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
		sessNoteError(s, ErrGeneric)
		return false
	}
	ok := e.Con.NotEquals(num)
	// residual ERROR sticky — no invent not-equal true past Con.NotEquals residual hole
	if sessHasError(s) {
		return false
	}
	return ok
}

// LessThan mirrors Expression::less_than(int).
// Expression.h default false; Constant.cpp:501–502.
// Incomplete Expression / Constant shell sticky false (no invent fold past holes).}

func (e *Expression) LessThan(num int) bool {
	return e.LessThanSess(nil, num)
}

func (e *Expression) LessThanSess(s *Session, num int) bool {
	// Expression always live for fold; sticky no invent compare past missing shell
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.Term != TermConstant {
		// Expression.h default false for non-Constant
		return false
	}
	// Constant Type* + Value always live; incomplete sticky (no invent fold past hole)
	if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
		sessNoteError(s, ErrGeneric)
		return false
	}
	ok := e.Con.LessThan(num)
	// residual ERROR sticky — no invent less-true past Con.LessThan residual hole
	if sessHasError(s) {
		return false
	}
	return ok
}

// Is0Or1 mirrors Expression::is_0_or_1.
// ExpressionFuncall → invoke; ExpressionComma → rhs; ExpressionAssign → simple+rhs.
// Incomplete IR sticky false (no invent 0/1 fold / soft re-pick past holes).}

func (e *Expression) Is0Or1() bool {
	return e.Is0Or1Sess(nil)
}

func (e *Expression) Is0Or1Sess(s *Session) bool {
	// Expression always live for fold; sticky no invent "not 0or1" past missing shell
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	switch e.Term {
	case TermFunction:
		// ExpressionFuncall always has live invoke for fold
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Invoke.Is0Or1Sess(s)
		// residual ERROR sticky — no invent 0or1 true past nested Is0Or1 residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermCommaExpr:
		// comma value is RHS; sticky incomplete without RHS
		if e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.CommaRHS.Is0Or1Sess(s)
		// residual ERROR sticky — no invent 0or1 true past RHS Is0Or1 residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	case TermAssignment:
		// ExpressionAssign.cpp:103–104
		if e.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		if e.Assign.AssignOp != AssignSimple {
			return false
		}
		if e.Assign.Expr == nil {
			// incomplete assign RHS sticky — no invent not-0or1 fold
			sessNoteError(s, ErrGeneric)
			return false
		}
		ok := e.Assign.Expr.Is0Or1Sess(s)
		// residual ERROR sticky — no invent 0or1 true past assign RHS residual hole
		if sessHasError(s) {
			return false
		}
		return ok
	}
	// Expression.h default false (incl. Constant — no override)
	return false
}

// UseVar mirrors Expression::use_var.
// Expression.h:143 default false; Variable/Funcall/Comma/Assign overrides.
// Incomplete IR fails closed sticky true (uses v) — no invent conflict-free non-use
// / soft re-pick past holes.}

func (e *Expression) UseVar(v *Variable) bool {
	return e.UseVarSess(nil, v)
}

func (e *Expression) UseVarSess(s *Session, v *Variable) bool {
	// Expression + subject always live; sticky incomplete — fail closed as uses
	// (no invent conflict-free non-use / soft re-pick past hole)
	if e == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Var == v {
			return true
		}
		matched := e.Var.Match(v)
		// residual ERROR sticky — no invent not-use soft-skip past Match hole (restrictive uses)
		if sessHasError(s) {
			return true
		}
		return matched
	case TermFunction:
		// ExpressionFuncall always has live invoke + args after ERROR_GUARD
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		for _, a := range e.Invoke.Args {
			if a == nil {
				// incomplete arg IR sticky — no invent skip hole as non-use
				sessNoteError(s, ErrGeneric)
				return true
			}
			if a.UseVar(v) {
				// residual ERROR sticky — no invent use-true past nested UseVar hole
				if sessHasError(s) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later args past nested UseVar residual
			if sessHasError(s) {
				return true
			}
		}
		return false
	case TermCommaExpr:
		if e.CommaLHS == nil || e.CommaRHS == nil {
			// incomplete comma sides sticky — no invent non-use past hole
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.CommaLHS.UseVar(v) {
			// residual ERROR sticky — no invent use-true past LHS UseVar hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue RHS past LHS UseVar residual
		if sessHasError(s) {
			return true
		}
		if e.CommaRHS.UseVar(v) {
			if sessHasError(s) {
				return true
			}
			return true
		}
		if sessHasError(s) {
			return true
		}
		return false
	case TermAssignment:
		if e.Assign == nil {
			// incomplete Assign shell sticky — no invent non-use
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Assign.LhsVar != nil {
			if e.Assign.LhsVar == v {
				return true
			}
			if e.Assign.LhsVar.Match(v) {
				// residual ERROR sticky — no invent use-true past Match hole
				if sessHasError(s) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue RHS past LhsVar Match residual
			if sessHasError(s) {
				return true
			}
		}
		if e.Assign.Lhs != nil {
			if e.Assign.Lhs.Var == nil {
				// incomplete Lhs.Var sticky
				sessNoteError(s, ErrGeneric)
				return true
			}
			if e.Assign.Lhs.Var == v {
				return true
			}
			if e.Assign.Lhs.Var.Match(v) {
				// residual ERROR sticky — no invent use-true past Match hole
				if sessHasError(s) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue RHS past Lhs Match residual
			if sessHasError(s) {
				return true
			}
		} else if e.Assign.LhsVar == nil {
			// C++ ExpressionAssign always has live lhs; neither side → incomplete
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Assign.Expr == nil {
			// incomplete assign RHS sticky — no invent non-use past hole
			sessNoteError(s, ErrGeneric)
			return true
		}
		ok := e.Assign.Expr.UseVar(v)
		// residual ERROR sticky — no invent not-use soft-skip past RHS UseVar hole
		if sessHasError(s) {
			return true
		}
		return ok
	case TermLhs:
		// Lhs as expression term if ever used
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Var == v {
			return true
		}
		matched := e.Var.Match(v)
		// residual ERROR sticky — no invent not-use soft-skip past Match hole
		if sessHasError(s) {
			return true
		}
		return matched
	case TermConstant:
		// constants do not use variables — complete default false
		return false
	default:
		// unknown term sticky incomplete (no invent default non-use)
		sessNoteError(s, ErrGeneric)
		return true
	}
}

// ToString mirrors Expression::to_string — Output without stream.
// Expression.cpp:120–124.
// Expression always live; sticky empty via Output (no invent soft-skip past hole).}

func (e *Expression) ToString() string {
	return e.ToStringSess(nil)
}

// ToStringSess is ToString with explicit session residual sticky.
func (e *Expression) ToStringSess(s *Session) string {
	out := e.OutputOptsSess(s, sessOpts(s))
	// residual ERROR sticky — no invent soft-empty string past Output residual
	if sessHasError(s) {
		return ""
	}
	return out
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
	// Expression.cpp — constant params weight 0; add_entry still records key (max += 0).
	t.Param.AddEntry(int(TermConstant), 0)
	if opts.EmbeddedAssigns {
		t.Param.AddEntry(int(TermAssignment), 10)
	}
	if opts.CommaOperators {
		t.Param.AddEntry(int(TermCommaExpr), 10)
	}
	return t
}

// InitProbabilityTables mirrors Expression::InitProbabilityTables static setup.
// Expression.cpp:93–96 — installs process session expr/param tables from opts.
func InitProbabilityTables(opts Options) *ExprTables {
	return InitProbabilityTablesSess(nil, opts)
}

// InitProbabilityTablesSess installs ExprTables on an explicit session bag.
func InitProbabilityTablesSess(s *Session, opts Options) *ExprTables {
	t := NewExprTables(opts)
	SetProcessExprTablesSess(s, t)
	return t
}

// InitExprProbabilityTable rebuilds only the expr DistributionTable half.
// Expression.cpp InitExprProbabilityTable path via NewExprTables.
func InitExprProbabilityTable(opts Options) DistributionTable {
	return NewExprTables(opts).Expr
}

// InitParamProbabilityTable rebuilds only the param DistributionTable half.
func InitParamProbabilityTable(opts Options) DistributionTable {
	return NewExprTables(opts).Param
}

// ExpressionTypeProbability mirrors ExpressionTypeProbability.
// Expression.cpp:103–112 — PartialExpander force eFunction when invoke expand;
// else rnd_upto(filter.max, filter); lookup → eTermType.
func ExpressionTypeProbability(r *Rng, filter *VectorFilter) TermType {
	return ExpressionTypeProbabilitySess(nil, r, filter)
}

// ExpressionTypeProbabilitySess is ExpressionTypeProbability with explicit session residual sticky.
func ExpressionTypeProbabilitySess(s *Session, r *Rng, filter *VectorFilter) TermType {
	// Expression.cpp:104–105 — PartialExpander::direct_expand_check(eInvoke)
	if DirectExpandCheckSess(s, StmtInvoke) {
		return TermFunction
	}
	// Expression.cpp:107–111 — assert(filter); ERROR_GUARD(MAX_TERM_TYPES) sticky; no soft invent eVariable
	if r == nil || filter == nil {
		sessNoteError(s, ErrGeneric)
		return MaxTermTypes
	}
	i := r.RndUptoFilterSess(s, uint32(filter.MaxProb()), filter)
	return TermType(filter.Lookup(int(i)))
}

// PickTermType builds default filters for Expression::make_random when tt==MAX.
// Expression.cpp:160–179 (subset: no_func, no_const, depth).
func PickTermType(r *Rng, tables *ExprTables, opts Options, typ *Type, noFunc, noConst bool, exprDepth int) TermType {
	return PickTermTypeSess(nil, r, tables, opts, typ, noFunc, noConst, exprDepth)
}

func PickTermTypeSess(s *Session, r *Rng, tables *ExprTables, opts Options, typ *Type, noFunc, noConst bool, exprDepth int) TermType {
	// Expression::InitProbabilityTables always live; ambient tables if arg nil
	if tables == nil {
		tables = sessExprTables(s)
	}
	// tables always live after InitProbabilityTables; sticky MaxTermTypes
	// (no invent soft term pick without session tables)
	if tables == nil {
		sessNoteError(s, ErrGeneric)
		return MaxTermTypes
	}
	f := NewVectorFilterSess(s, &tables.Expr)
	if noFunc {
		f.Add(int(TermFunction))
	}
	if noConst {
		f.Add(int(TermConstant))
	}
	// Expression.cpp:166–175 — struct/union cannot be constant subexpr; func gated by return flags
	if typ != nil {
		isSt := typ.IsStructSess(s)
		// residual ERROR sticky — no invent soft-filter term past IsStruct residual
		if sessHasError(s) {
			return MaxTermTypes
		}
		isUn := typ.IsUnionSess(s)
		// residual ERROR sticky — no invent soft-filter term past IsUnion residual
		if sessHasError(s) {
			return MaxTermTypes
		}
		if isSt || isUn {
			f.Add(int(TermConstant))
			if isSt && !opts.ReturnStructs {
				f.Add(int(TermFunction))
			}
			if isUn && !opts.ReturnUnions {
				f.Add(int(TermFunction))
			}
			isCSU := typ.IsConstStructUnionSess(s)
			// residual ERROR sticky — no invent soft-filter term past IsConstStructUnion residual
			if sessHasError(s) {
				return MaxTermTypes
			}
			isVSU := typ.IsVolatileStructUnionSess(s)
			// residual ERROR sticky — no invent soft-filter term past IsVolatileStructUnion residual
			if sessHasError(s) {
				return MaxTermTypes
			}
			if isCSU || isVSU {
				f.Add(int(TermAssignment))
			}
		}
	}
	// depth gate: Expression.cpp:177–178
	if exprDepth+2 > opts.MaxExprComplexity {
		f.Add(int(TermFunction)).Add(int(TermAssignment)).Add(int(TermCommaExpr))
	}
	return ExpressionTypeProbabilitySess(s, r, f)
}

// PickParamTermType mirrors Expression::make_random_param term selection.
// Expression.cpp:244–260 — paramTable + always filter Constant.}

func PickParamTermType(r *Rng, tables *ExprTables, opts Options, typ *Type, exprDepth int) TermType {
	return PickParamTermTypeSess(nil, r, tables, opts, typ, exprDepth)
}

func PickParamTermTypeSess(s *Session, r *Rng, tables *ExprTables, opts Options, typ *Type, exprDepth int) TermType {
	// Expression::InitProbabilityTables always live; ambient tables if arg nil
	if tables == nil {
		tables = sessExprTables(s)
	}
	// tables always live after InitProbabilityTables; sticky MaxTermTypes
	// (no invent soft param term pick without session tables)
	if tables == nil {
		sessNoteError(s, ErrGeneric)
		return MaxTermTypes
	}
	f := NewVectorFilterSess(s, &tables.Param)
	// don't call functions with constant parameters
	f.Add(int(TermConstant))
	if typ != nil {
		if typ.IsStructSess(s) && !opts.ReturnStructs {
			f.Add(int(TermFunction))
		}
		if typ.IsUnionSess(s) && !opts.ReturnUnions {
			f.Add(int(TermFunction))
		}
		isCSU := typ.IsConstStructUnionSess(s)
		// residual ERROR sticky — no invent soft-filter param term past IsConstStructUnion residual
		if sessHasError(s) {
			return MaxTermTypes
		}
		if isCSU {
			f.Add(int(TermAssignment))
		}
	}
	if exprDepth+2 > opts.MaxExprComplexity {
		f.Add(int(TermFunction)).Add(int(TermAssignment)).Add(int(TermCommaExpr))
	}
	return ExpressionTypeProbabilitySess(s, r, f)
}

// MakeRandomParam mirrors Expression::make_random_param.
// Expression.cpp:238–296 — param probability table; constants filtered out.
// cg is *CGContext so visit_facts effects persist (C++ CGContext&).}

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
	// Expression.cpp always has RNG sticky; no invent param expression without it
	if r == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky when live cg (no invent param past holes)
	if cg != nil {
		if !EffectComplete(cg.EffectContext()) ||
			(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
			!EffectComplete(cg.EffectStm) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return nil
		}
		if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return nil
		}
	}
	// Expression.cpp:241–242 — assert(type) sticky; DEPTH_GUARD after type known
	if typ == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// Expression.cpp:242–243 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtExpressionRandomParam, tt, …)
	// term type not chosen yet when MAX; guard uses flag 0 until Pick (C++ uses tt arg)
	if DepthGuardByTypeFlagSess(cgSess(cg), opts, DtExpressionRandomParam, int(MaxTermTypes)) == BadDepth {
		return nil
	}
	// Expression.cpp:258 — use cg_context.expr_depth (exprDepth param kept for API)
	depth := exprDepth
	if cg != nil {
		depth = cg.ExprDepth
	}
	tt := PickParamTermTypeSess(cgSess(cg), r, tables, opts, typ, depth)
	// Expression.cpp:264 — ERROR_GUARD(nullptr) after ExpressionTypeProbability
	if sessHasError(cgSess(cg)) {
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
		if sessHasError(cgSess(cg)) {
			return nil
		}
		if e != nil && cg != nil && BumpsExprDepthSess(cgSess(cg), e) {
			cg.ExprDepth++
		}
		return e
	}
	// no_func=false, no_const=true for other terms (MakeRandomExpression bumps depth)
	return MakeRandomExpression(r, opts, tables, vs, cg, typ, qfer, false, true, tt, depth, list...)
}

// BumpsExprDepth reports whether this expression increments cg_context.expr_depth.
// Expression.cpp:213–218 — Constant, Variable, or user FunctionInvocation.
// Expression always live; sticky true (no invent not-bump soft-skip depth past hole).
// Incomplete Function IR (nil Invoke / non-std without User) sticky true — C++ would
// crash on get_invoke()/get_func(); no invent not-bump past holes for siblings.
func BumpsExprDepth(e *Expression) bool {
	return BumpsExprDepthSess(nil, e)
}

func BumpsExprDepthSess(s *Session, e *Expression) bool {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Type*+Value; incomplete sticky bump (restrictive)
		// (no invent not-bump soft-skip depth past Type-nil / empty-value shell)
		if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
			sessNoteError(s, ErrGeneric)
			return true
		}
		return true
	case TermVariable:
		// Variable* always live; Type-nil non-special sticky bump (restrictive)
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Var.Type == nil && !IsSpecialPtr(e.Var) {
			sessNoteError(s, ErrGeneric)
			return true
		}
		return true
	case TermFunction:
		// ExpressionFuncall always has live invoke before get_func/is_std_func
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.Invoke.IsStd {
			return false
		}
		// user-defined path: Function* always live; incomplete sticky bump
		if e.Invoke.User == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		return true
	default:
		return false
	}
}

// MakeRandomExpression mirrors Expression::make_random (const/var/funcall).
// Expression.cpp:141–219.
// cg is *CGContext so ExpressionVariable::visit_facts mutates the caller's
// context (C++ CGContext&); by-value dropped EffectStm / merge_param updates.}

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
	// Expression.cpp always has RNG + live CGContext; sticky no invent leaf shells without them
	if r == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent leaf / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// Expression.cpp:144–145 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtExpression, tt, nullptr)
	if DepthGuardByTypeFlagSess(cgSess(cg), opts, DtExpression, int(tt)) == BadDepth {
		return nil
	}
	// Expression::InitProbabilityTables — session tables when caller omits them
	if tables == nil {
		tables = sessExprTables(cgSess(cg))
	}
	if tables == nil {
		// sticky no invent NewExprTables mid expression
		sessNoteError(cgSess(cg), ErrGeneric)
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
	// C++ always has AllTypes from GenerateSimpleTypes; empty env soft nil
	// (no invent simple type; non-sticky soft re-pick — later tries may sticky if typ stays nil)
	if typ == nil {
		if env == nil || len(env.AllTypes) == 0 {
			return nil
		}
		seFree := cg.EffectContext().IsSideEffectFreeSess(cgSess(cg))
		// residual ERROR sticky — no invent soft-choose type past IsSideEffectFree residual
		if sessHasError(cgSess(cg)) {
			return nil
		}
		for tries := 0; tries < 256; tries++ {
			if seFree {
				typ = env.ChooseRandomNonvoid(r, opts, probs)
			} else {
				typ = env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
			}
			// Expression.cpp: ERROR_GUARD paths inside choose_random*
			if sessHasError(cgSess(cg)) {
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
		// choose_random_nonvoid ERROR_GUARD sticky — no invent expression without type
		if typ == nil {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return nil
		}
	}
	// Expression.cpp:154–157 — asserts on illegal term/type sticky (no soft invent rewrite)
	// no_func && eFunction / no_const && eConstant / struct && eConstant
	if noFunc && tt == TermFunction {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	if noConst && tt == TermConstant {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	if typ != nil && typ.IsStruct() && tt == TermConstant {
		// was soft invent TermVariable — C++ assert sticky, not rewrite
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// Expression.cpp:176–178 / 213 — always cg_context.expr_depth (not a separate local)
	_ = exprDepth
	depth := cg.ExprDepth
	if tt == MaxTermTypes {
		tt = PickTermTypeSess(cgSess(cg), r, tables, opts, typ, noFunc, noConst, depth)
	}
	// Expression.cpp:182 — ERROR_GUARD(nullptr) after term pick
	if sessHasError(cgSess(cg)) {
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
		// Expression.cpp:185–188 — assert simple != eVoid sticky
		if typ != nil {
			simple := typ.IsSimple()
			// residual ERROR sticky — no invent soft-const past IsSimple residual
			if sessHasError(cgSess(cg)) {
				return nil
			}
			if simple && typ.Simple() == EVoid {
				sessNoteError(cgSess(cg), ErrGeneric)
				return nil
			}
		}
		// Expression.cpp:188 — Constant::make_random; ERROR_GUARD after
		// no invent TermConstant shell with nil Con when make_random fails
		con := MakeRandomSess(cgSess(cg), typ, opts, probs, r)
		if con == nil || sessHasError(cgSess(cg)) {
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
	if sessHasError(cgSess(cg)) {
		return nil
	}
	if e == nil {
		// C++ would crash on e->term_type; factories return null only with error
		return nil
	}
	if BumpsExprDepthSess(cgSess(cg), e) {
		cg.ExprDepth++
	}
	return e
}

// exprVarSelectRetryCeiling is the Go soft bound for ExpressionVariable.cpp:71–132
// do { select; filters; visit_facts } while (true). C++ is unbounded; 256 was too
// low (seed-599096333: exhausted mid-EV → Statement::make_random re-pick while UP
// continued SelectParentLocal). Same order as MakeRandomLhs (Lhs.cpp:70–145 /
// seed-2 e4412 SelectDeref path).
const exprVarSelectRetryCeiling = 10000

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
	// ExpressionVariable.cpp always has RNG + live context + Type*
	// sticky no invent var shell without them; nil typ must not soft-skip type filters
	if r == nil || cg == nil || typ == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// nil VS: non-sticky soft re-pick (unit MaxTermTypes / return factory soft nil;
	// sticky poisons leaf term pick when caller omits selector)
	if vs == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent var expr / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// ExpressionVariable.cpp:61 — DEPTH_GUARD_BY_TYPE_RETURN(dtExpressionVariable, nullptr)
	if DepthGuardByTypeSess(vsSess(vs), vs.Opts, DtExpressionVariable) == BadDepth {
		return nil
	}
	// ExpressionVariable.cpp:67–69 — snapshot effects for visit_facts failure restore
	var preAccum, preStm Effect
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-expr past EffectAccum Clone residual
		if sessHasError(cgSess(cg)) {
			return nil
		}
	}
	preStm = cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-expr past EffectStm Clone residual
	if sessHasError(cgSess(cg)) {
		return nil
	}
	// ExpressionVariable.cpp:71–132 — do { select; filters; visit_facts } while (true).
	// dummy is invalid_vars passed into select (ExpressionVariable.cpp:78, 131).
	// C++ is unbounded; Go uses exprVarSelectRetryCeiling (see const).
	var dummy []*Variable
	for tries := 0; tries < exprVarSelectRetryCeiling; tries++ {
		// ExpressionVariable.cpp:74–76 — select_must_use_var READ first
		v := vs.SelectMustUseVar(r, AccessRead, *cg, typ, qfer)
		// residual ERROR sticky — no invent fall through soft select past must-use hole
		if sessHasError(cgSess(cg)) {
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			return nil
		}
		if v == nil {
			// ExpressionVariable.cpp:77–78 — select(..., dummy, eFlexible)
			v = vs.SelectWithInvalid(AccessRead, *cg, typ, qfer, r, MatchFlexible, dummy)
			// residual ERROR sticky — no invent soft-continue / create past select hole
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
		}
		if v == nil {
			// complete soft miss (no residual): re-pick
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
			sessNoteError(cgSess(cg), ErrGeneric)
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			return nil
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent READ var expr past broken array shell)
		if v.IsArray && v.AsArray == nil {
			sessNoteError(cgSess(cg), ErrGeneric)
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			return nil
		}
		// ExpressionVariable.cpp:93–94 — no float var for non-float want
		// C++ continue without dummy.push_back (only validate/visit fail push dummy)
		if !typ.IsFloatSess(cgSess(cg)) && v.Type.IsFloatSess(cgSess(cg)) {
			// residual ERROR sticky — no invent soft-continue then pick later past IsFloat hole
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue non-float filter past IsFloat residual false path
		if sessHasError(cgSess(cg)) {
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			return nil
		}
		// ExpressionVariable.cpp:97–100 — as_param forbid address-of argument
		// C++: var->type->is_dereferenced_from(type)  (want = type, take &)
		// continue without dummy (ExpressionVariable.cpp:97–100)
		if asParam {
			isArg := v.IsArgument()
			// residual ERROR sticky — no invent soft-continue past IsArgument residual
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			if isArg && v.Type.IsDereferencedFrom(typ) {
				// residual ERROR sticky — no invent soft-continue past IsDereferencedFrom hole
				if sessHasError(cgSess(cg)) {
					if cg.EffectAccum != nil {
						*cg.EffectAccum = preAccum
					}
					cg.EffectStm = preStm
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-continue past IsDereferencedFrom residual false
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
		}
		// ExpressionVariable.cpp:101–105 — !addr_taken_of_locals: forbid & local/arg
		// continue without dummy
		if !vs.Opts.AddrTakenOfLocals {
			isArg := v.IsArgument()
			// residual ERROR sticky — no invent soft-continue past IsArgument residual
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			isLoc := v.IsLocal()
			// residual ERROR sticky — no invent soft-continue past IsLocal residual
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			if (isArg || isLoc) && v.Type.IsDereferencedFrom(typ) {
				// residual ERROR sticky — no invent soft-continue past IsDereferencedFrom hole
				if sessHasError(cgSess(cg)) {
					if cg.EffectAccum != nil {
						*cg.EffectAccum = preAccum
					}
					cg.EffectStm = preStm
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-continue past IsDereferencedFrom residual false
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
		}
		// ExpressionVariable.cpp:111–115 — as_return + no_return_dead_ptr
		// continue without dummy
		if asReturn && vs.Opts.NoReturnDeadPointer {
			if v.Type == nil || typ == nil {
				sessNoteError(cgSess(cg), ErrGeneric)
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			lv := v.Type.IndirectLevelSess(cgSess(cg))
			// residual ERROR sticky — no invent soft-filter past subject IndirectLevel residual
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			lw := typ.IndirectLevelSess(cgSess(cg))
			// residual ERROR sticky — no invent soft-filter past desired IndirectLevel residual
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			indirection := lv - lw
			var facts []*FactPointTo
			if cg.FM != nil {
				// incomplete GlobalFacts fail closed sticky (no invent soft-skip local-ptr filter)
				if !FactsComplete(cg.FM.GlobalFacts) {
					sessNoteError(cgSess(cg), ErrGeneric)
					if cg.EffectAccum != nil {
						*cg.EffectAccum = preAccum
					}
					cg.EffectStm = preStm
					return nil
				}
				facts = cg.FM.GlobalFacts
			}
			if IsPointingToLocalsSess(cgSess(cg), v, cg.CurrentBlock(), indirection, facts) {
				// residual ERROR sticky — no invent soft-continue past local-ptr hole
				if sessHasError(cgSess(cg)) {
					if cg.EffectAccum != nil {
						*cg.EffectAccum = preAccum
					}
					cg.EffectStm = preStm
					return nil
				}
				continue
			}
		}
		// ExpressionVariable.cpp:116–119 — opportunistic_validate (always; empty facts if no FM)
		var facts []*FactPointTo
		if cg.FM != nil {
			if !FactsComplete(cg.FM.GlobalFacts) {
				sessNoteError(cgSess(cg), ErrGeneric)
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			facts = cg.FM.GlobalFacts
		}
		if OpportunisticValidateSess(cgSess(cg), r, v, typ, facts, vs.Opts.NullPointerDerefProb, vs.Opts.DeadPointerDerefProb) == 0 {
			// residual ERROR sticky — no invent soft-continue past validate hole
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
			dummy = append(dummy, v)
			continue
		}
		// ExpressionVariable.cpp:80 ERROR_GUARD after select (sticky)
		if sessHasError(cgSess(cg)) {
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preAccum
			}
			cg.EffectStm = preStm
			return nil
		}
		// ExpressionVariable.cpp:120–124 — visit_facts with (var, type); on success
		// use ExpressionVariable(*var) when indirection==0 else (*var, type)
		// C++ always has FactMgr; visit_facts records reads even with empty fact vec
		probe := &Expression{Term: TermVariable, Var: v, ExprType: typ}
		if !cg.VisitFactsExpressionVariable(probe, vs.Opts) {
			// residual ERROR sticky — no invent soft-continue past visit_facts hard IR hole
			if sessHasError(cgSess(cg)) {
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preAccum
				}
				cg.EffectStm = preStm
				return nil
			}
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
		if ind0, iok := probe.IndirectLevelCompleteSess(cgSess(cg)); iok && ind0 == 0 {
			ev = &Expression{Term: TermVariable, Var: v, ExprType: v.Type}
		}
		// ExpressionVariable.cpp:137–142 — bookkeeping on successful make
		deref, _ := ev.IndirectLevelCompleteSess(cgSess(cg))
		if deref > 0 {
			bk := sessBK(cgSess(cg))
			IncrCounterSess(cgSess(cg), &bk.readDereferenceCnts, deref)
		} else if deref < 0 {
			RecordAddressTakenSess(cgSess(cg), v)
		}
		RecordVolatileAccessSess(cgSess(cg), v, deref, false)
		return ev
	}
	return nil
}

// Output is a minimal C fragment (Expression::Output + optional cast).
// Expression.cpp:227–232 output_cast — "(type) " prefix when cast_type set.
// Ambient ProcessOptions bridge; emit paths prefer OutputSess / OutputOptsSess.
func (e *Expression) Output() string {
	return e.OutputSess(nil)
}

// OutputSess is Output with Options/sticky from an explicit session bag.
func (e *Expression) OutputSess(s *Session) string {
	return e.OutputOptsSess(s, sessOpts(s))
}

// OutputOpts is Output with explicit session Options (const emit / access_once / lang_cpp).
func (e *Expression) OutputOpts(opts Options) string {
	return e.OutputOptsSess(nil, opts)
}

// OutputOptsSess is OutputOpts with sticky errors on bag s.
func (e *Expression) OutputOptsSess(s *Session, opts Options) string {
	// Expression* always live at Output; sticky no invent empty token without it
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	body := e.outputBodyOptsSess(s, opts)
	// residual ERROR sticky — no invent soft-empty body past outputBody residual hole
	if sessHasError(s) {
		return ""
	}
	if e.CastType != nil {
		// Expression.cpp:228–231 — cast_type->Output + body; both always live
		// sticky no invent "() body" / "(type) " empty body
		cn := e.CastType.CName()
		// residual ERROR sticky — no invent cast emit past CName residual hole
		if sessHasError(s) {
			return ""
		}
		if cn == "" || body == "" {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return ""
		}
		return "(" + cn + ") " + body
	}
	return body
}

// IndentedOutput mirrors Expression::indented_output.
// Expression.cpp:133–136 — output_tab(indent) + Output.
func (e *Expression) IndentedOutput(indent int) string {
	return e.IndentedOutputSess(nil, indent)
}

// IndentedOutputSess is IndentedOutput with Options/sticky from an explicit session bag.
func (e *Expression) IndentedOutputSess(s *Session, indent int) string {
	return e.IndentedOutputOptsSess(s, indent, sessOpts(s))
}

// IndentedOutputOpts is IndentedOutput with explicit session Options.
func (e *Expression) IndentedOutputOpts(indent int, opts Options) string {
	return e.IndentedOutputOptsSess(nil, indent, opts)
}

// IndentedOutputOptsSess is IndentedOutputOpts with sticky errors on bag s.
func (e *Expression) IndentedOutputOptsSess(s *Session, indent int, opts Options) string {
	out := e.OutputOptsSess(s, opts)
	// residual ERROR sticky — no invent soft-indent emit past Output residual
	if sessHasError(s) {
		return ""
	}
	return OutputTab(indent) + out
}

func (e *Expression) outputBody() string {
	return e.outputBodySess(nil)
}

func (e *Expression) outputBodySess(s *Session) string {
	return e.outputBodyOptsSess(s, sessOpts(s))
}

func (e *Expression) outputBodyOpts(opts Options) string {
	return e.outputBodyOptsSess(nil, opts)
}

func (e *Expression) outputBodyOptsSess(s *Session, opts Options) string {
	if e == nil {
		return ""
	}
	switch e.Term {
	case TermConstant:
		// Expression::to_string → Output → Constant::Output (Constant.cpp:532–551).
		// Must use Con.OutputOpts so negatives become "(-6L)" for array init_strings
		// (ArrayVariable.cpp:490–492 init_values[i]->to_string()), not bare Value.
		if e.Con == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		return e.Con.OutputOptsSess(s, opts)
	case TermVariable:
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// ExpressionVariable::Output — *…var or &var from indirect level.
		// ExpressionVariable.cpp:202–219 — base is Variable::Output (VOL_RVAL/ACCESS_ONCE).
		return outputExpressionVariableOptsSess(s, e.Var, e.ExprType, opts)
	case TermFunction:
		if e.Invoke != nil {
			out := e.Invoke.OutputOptsSess(s, opts)
			// residual ERROR sticky — no invent soft-empty call past Invoke Output residual
			if sessHasError(s) {
				return ""
			}
			return out
		}
		sessNoteError(s, ErrGeneric)
		return ""
	case TermAssignment:
		if e.Assign != nil {
			// ExpressionAssign::Output → (assign as expr)
			wrap := e.Assign.LhsVar != nil && e.Assign.LhsVar.UseVolRVal
			as := OutputAssignAsExprOptsSess(s, e.Assign, wrap, opts)
			// residual ERROR sticky — no invent soft-empty assign expr past Output residual
			if sessHasError(s) {
				return ""
			}
			if as != "" {
				return "(" + as + ")"
			}
			sessNoteError(s, ErrGeneric)
			return ""
		}
		sessNoteError(s, ErrGeneric)
		return ""
	case TermCommaExpr:
		// ExpressionComma.cpp:137–144 — "(" + lhs + " , " + rhs + ")"
		// C++ always has live lhs/rhs Output; sticky no invent "( , )" / "(x , )" / "( , y)"
		if e.CommaLHS == nil || e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		lhs := e.CommaLHS.OutputOptsSess(s, opts)
		// residual ERROR sticky — no invent soft-continue RHS past LHS Output residual
		if sessHasError(s) {
			return ""
		}
		rhs := e.CommaRHS.OutputOptsSess(s, opts)
		// residual ERROR sticky — no invent soft-empty comma past RHS Output residual
		if sessHasError(s) {
			return ""
		}
		if lhs == "" || rhs == "" {
			// incomplete side IR — sticky fail closed whole comma
			sessNoteError(s, ErrGeneric)
			return ""
		}
		return "(" + lhs + " , " + rhs + ")"
	}
	// Expression.cpp:195–200 default: no emit invent; incomplete IR sticky empty
	sessNoteError(s, ErrGeneric)
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
	// ExpressionFuncall always has RNG + CGContext; sticky no invent funcall shell without them
	if r == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// ExpressionFuncall.cpp:75 — get_fact_mgr always live; non-sticky soft re-pick without FM
	// (sticky poisons term re-pick when FactMgr not yet wired)
	if cg.FM == nil {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent funcall / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
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
	if typ != nil {
		simple := typ.IsSimple()
		// residual ERROR sticky — no invent soft-std path past IsSimple residual
		if sessHasError(cgSess(cg)) {
			return nil
		}
		if !simple || typ.Simple() == EVoid {
			stdFunc = false
		}
	}
	// ExpressionFuncall.cpp:75–78 — snapshot effects and facts for failed invocation
	var preAccum Effect
	if cg.EffectAccum != nil {
		preAccum = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-funcall past EffectAccum Clone residual
		if sessHasError(cgSess(cg)) {
			return nil
		}
	}
	preStm := cg.EffectStm.Clone()
	// residual ERROR sticky — no invent soft-funcall past EffectStm Clone residual
	if sessHasError(cgSess(cg)) {
		return nil
	}
	// incomplete GlobalFacts/UnionFacts fail closed sticky (no invent cleaned snapshot)
	if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// ExpressionFuncall.cpp:78 — vector<const Fact *> facts_copy = fm->global_facts
	// Full FactVec (ePointTo + eUnionWrite). Shallow Fact* vector copy — Fact objects
	// shared until merge replaces slots. Deep CloneFactSlice freezes pre-merge lattice
	// and can drop mid-gen may-null updates C++ keeps via shared Fact*.
	// Soft invent was RestoreFacts(PT-only): UnionFacts stayed at post-failed-call
	// last-writes → IsNonreadableField over-filtered choose_var (seed-7).
	factsCopy := append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
	unionCopy := append([]*FactUnion(nil), cg.FM.UnionFacts...)
	fi := MakeRandomInvocation(r, opts, probs, vs, tables, cg, list, typ, qfer, stdFunc)
	// ExpressionFuncall.cpp:82 — ERROR_GUARD(nullptr) before fi->failed
	if sessHasError(cgSess(cg)) {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		return nil
	}
	// FunctionInvocation.cpp:119 assert(fi != 0); nil without Failed is incomplete
	if fi == nil {
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		return nil
	}
	if fi.Failed {
		// ExpressionFuncall.cpp:84–91 — restore env; replace with simple var
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preAccum
		}
		cg.EffectStm = preStm
		cg.FM.RestoreFactsPair(factsCopy, unionCopy)
		return makeExpressionVariable(r, vs, cg, typ, qfer)
	}
	return &Expression{Term: TermFunction, Invoke: fi}
}
