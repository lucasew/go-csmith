// Upstream: FunctionInvocationBinary.cpp (safe_ops, equals, is_0_or_1, visit_facts).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// BinaryOpFromString maps C token to BinaryOp when possible.
func BinaryOpFromString(s string) (BinaryOp, bool) {
	switch s {
	case "+":
		return BinAdd, true
	case "-":
		return BinSub, true
	case "*":
		return BinMul, true
	case "/":
		return BinDiv, true
	case "%":
		return BinMod, true
	case "&":
		return BinBitAnd, true
	case "|":
		return BinBitOr, true
	case "^":
		return BinBitXor, true
	case "<<":
		return BinLShift, true
	case ">>":
		return BinRShift, true
	case "&&":
		return BinAnd, true
	case "||":
		return BinOr, true
	case ">":
		return BinCmpGt, true
	case "<":
		return BinCmpLt, true
	case ">=":
		return BinCmpGe, true
	case "<=":
		return BinCmpLe, true
	case "==":
		return BinCmpEq, true
	case "!=":
		return BinCmpNe, true
	default:
		return 0, false
	}
}

// GetBinopString mirrors FunctionInvocationBinary::get_binop_string.
// FunctionInvocationBinary.cpp:313+ — same as BinaryOpC for known ops.
func GetBinopStringSess(s *Session, op BinaryOp) string {
	return op.BinaryOpCSess(s)
}

// IsReturnTypeFloat mirrors FunctionInvocationBinary::is_return_type_float.
// FunctionInvocationBinary.cpp:184–187 — assert(op_flags); size is sFloat.
// Missing Safe is complete non-float (not sticky) — Safe is optional on std binary
// shells; invent would be treating nil as float, not non-float.
func (fi *Invocation) IsReturnTypeFloat() bool {
	return fi.IsReturnTypeFloatSess(testAmbientSession)
}

// IsReturnTypeFloatSess is IsReturnTypeFloat with explicit session residual sticky.
func (fi *Invocation) IsReturnTypeFloatSess(s *Session) bool {
	// nil inv sticky incomplete; missing Safe alone → complete false (not float)
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if fi.Safe == nil {
		return false
	}
	return fi.Safe.Size == SafeFloat
}

// GetType mirrors FunctionInvocation{Unary,Binary,User}::get_type.
// FunctionInvocationUnary.cpp:114–131; FunctionInvocationBinary.cpp:192–241;
// FunctionInvocationUser.cpp:380 — return type.
func (fi *Invocation) GetType() *Type {
	return fi.GetTypeSess(testAmbientSession)
}

func (fi *Invocation) GetTypeSess(s *Session) *Type {
	// C++ FunctionInvocation always non-null; sticky nil → no invent int type shell
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if fi.User != nil {
		// FunctionInvocationUser.cpp:380 — return func->return_type
		// nil ReturnType incomplete sticky
		if fi.User.ReturnType == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		return fi.User.ReturnType
	}
	if !fi.IsStd {
		// incomplete non-user non-std shell sticky
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if fi.IsUnary {
		return fi.getTypeUnarySess(s)
	}
	return fi.getTypeBinarySess(s)
}

// getTypeUnary mirrors FunctionInvocationUnary::get_type.
// FunctionInvocationUnary.cpp:114–131.}

func (fi *Invocation) getTypeUnary() *Type {
	return fi.getTypeUnarySess(testAmbientSession)
}

func (fi *Invocation) getTypeUnarySess(s *Session) *Type {
	// FunctionInvocationUnary.cpp:116–129 — switch on known ops only
	switch fi.Unary {
	case "!":
		return GetIntTypeSess(s)
	case "+", "-", "~":
		// C++ param_value[0]->get_type(); missing operand → incomplete IR sticky
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ty := fi.Args[0].GetTypeSess(s)
		// residual ERROR sticky — no invent unary type past GetType residual hole
		if sessHasError(s) {
			return nil
		}
		return ty
	default:
		// FunctionInvocationUnary.cpp:117 assert invalid operator sticky; no invent eInt
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// getTypeBinary mirrors FunctionInvocationBinary::get_type.
// FunctionInvocationBinary.cpp:192–241.}

func (fi *Invocation) getTypeBinary() *Type {
	return fi.getTypeBinarySess(testAmbientSession)
}

func (fi *Invocation) getTypeBinarySess(s *Session) *Type {
	// FunctionInvocationBinary.cpp:193–194
	if fi.IsReturnTypeFloatSess(s) {
		return GetSimpleTypeSess(s, EFloat)
	}
	op, ok := BinaryOpFromString(fi.Binary)
	// FunctionInvocationBinary.cpp:196–199 — assert invalid operator sticky; no soft invent eInt
	if !ok {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	switch op {
	case BinAdd, BinSub, BinMul, BinDiv, BinMod, BinBitXor, BinBitAnd, BinBitOr:
		// FunctionInvocationBinary.cpp:208–224 — param_value[0/1]->get_type always live
		// missing operands → incomplete IR sticky (no invent signed=true → eInt)
		if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		lt, rt := fi.Args[0].GetTypeSess(s), fi.Args[1].GetTypeSess(s)
		// residual ERROR sticky — no invent eInt/eUInt past GetType residual hole
		if sessHasError(s) {
			return nil
		}
		if lt == nil || rt == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ls, rs := lt.IsSignedSess(s), rt.IsSignedSess(s)
		// residual ERROR sticky — no invent eInt/eUInt past IsSigned residual hole
		if sessHasError(s) {
			return nil
		}
		if ls && rs {
			return GetSimpleTypeSess(s, EInt)
		}
		return GetSimpleTypeSess(s, EUInt)
	case BinCmpGt, BinCmpLt, BinCmpGe, BinCmpLe, BinCmpEq, BinCmpNe, BinAnd, BinOr:
		return GetIntTypeSess(s)
	case BinLShift, BinRShift:
		// FunctionInvocationBinary.cpp:229–238 — param_value[0]->get_type always sticky
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		lt := fi.Args[0].GetTypeSess(s)
		// residual ERROR sticky — no invent eInt/eUInt past GetType residual hole
		if sessHasError(s) {
			return nil
		}
		if lt == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ls := lt.IsSignedSess(s)
		// residual ERROR sticky — no invent eInt/eUInt past IsSigned residual hole
		if sessHasError(s) {
			return nil
		}
		if ls {
			return GetSimpleTypeSess(s, EInt)
		}
		return GetSimpleTypeSess(s, EUInt)
	default:
		// FunctionInvocationBinary.cpp:240–241 — assert(0) sticky; no soft invent eInt
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// SafeInvocation mirrors FunctionInvocation::safe_invocation.
// Unary: eMinus false (FunctionInvocationUnary.cpp:185–187);
// Binary: always false; User: always true.
// Incomplete Invocation sticky false (no invent unsafe soft-skip / soft re-pick).}

func (fi *Invocation) SafeInvocation() bool {
	return fi.SafeInvocationSess(testAmbientSession)
}

func (fi *Invocation) SafeInvocationSess(s *Session) bool {
	// Invocation always live; sticky incomplete no invent not-safe soft-skip
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if fi.User != nil {
		return true
	}
	if fi.IsStd && fi.IsUnary {
		return fi.Unary != "-"
	}
	// binary std: false
	return false
}

// CompatibleVar mirrors FunctionInvocationUnary::compatible.
// FunctionInvocationUnary.cpp:137–141 — operand[0].compatible(v); binary/user false.
// Incomplete unary operand sticky false (no invent soft-skip / soft re-pick past hole).}

func (fi *Invocation) CompatibleVar(v *Variable, expandStruct bool) bool {
	return fi.CompatibleVarSess(testAmbientSession, v, expandStruct)
}

func (fi *Invocation) CompatibleVarSess(s *Session, v *Variable, expandStruct bool) bool {
	// Invocation always live; non-unary-std complete false (C++ binary/user)
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if v == nil {
		// FunctionInvocationUnary.cpp assert path via operand.compatible(v)
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !fi.IsStd || !fi.IsUnary {
		return false
	}
	if len(fi.Args) == 0 || fi.Args[0] == nil {
		// incomplete unary operand sticky — no invent not-compatible soft-skip
		sessNoteError(s, ErrGeneric)
		return false
	}
	ok := fi.Args[0].CompatibleWithVarSess(s, v, expandStruct)
	// residual ERROR sticky — no invent compatible true past CompatibleWithVar residual hole
	if sessHasError(s) {
		return false
	}
	return ok
}

// Is0Or1 mirrors FunctionInvocationBinary::is_0_or_1 and Unary eNot.
// FunctionInvocationBinary.cpp:179–181 — comparison ops yield 0/1.
// FunctionInvocationUnary.h:67 — eNot only.
// Incomplete Invocation sticky false (no invent not-0or1 / soft re-pick past hole).}

func (fi *Invocation) Is0Or1() bool {
	return fi.Is0Or1Sess(testAmbientSession)
}

func (fi *Invocation) Is0Or1Sess(s *Session) bool {
	// Invocation always live for fold; sticky incomplete no invent not-0or1
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !fi.IsStd {
		return false
	}
	if fi.IsUnary {
		// UnaryOps eNot → "!"
		return fi.Unary == "!"
	}
	op, ok := BinaryOpFromString(fi.Binary)
	if !ok {
		return false
	}
	switch op {
	case BinCmpGt, BinCmpLt, BinCmpGe, BinCmpLe, BinCmpEq, BinCmpNe:
		return true
	default:
		return false
	}
}

// EqualsInt mirrors FunctionInvocationBinary::equals / FunctionInvocationUnary::equals.
// FunctionInvocationBinary.cpp:154–177; FunctionInvocationUnary.cpp:144–156.
// Incomplete param IR sticky false (no invent not-equal fold / soft re-pick past holes).}

func (fi *Invocation) EqualsInt(num int) bool {
	return fi.EqualsIntSess(testAmbientSession, num)
}

func (fi *Invocation) EqualsIntSess(s *Session, num int) bool {
	// Invocation always live for fold; sticky incomplete no invent not-equal
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !fi.IsStd {
		return false
	}
	// FunctionInvocationUnary.cpp:145 — assert(!param_value.empty())
	// FunctionInvocationBinary.cpp:155 — assert(param_value.size() == 2)
	if fi.IsUnary {
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		a0 := fi.Args[0]
		if num == 0 && fi.Unary == "!" {
			if a0.NotEqualsSess(s, 0) {
				// residual ERROR sticky — no invent equal-true past NotEquals residual hole
				if sessHasError(s) {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue fold past NotEquals residual false
			if sessHasError(s) {
				return false
			}
		}
		if num == 1 && fi.Unary == "!" {
			if a0.EqualsIntSess(s, 0) {
				// residual ERROR sticky — no invent equal-true past EqualsInt residual hole
				if sessHasError(s) {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue fold past EqualsInt residual false
			if sessHasError(s) {
				return false
			}
		}
		if fi.Unary == "-" {
			if a0.EqualsIntSess(s, -num) {
				// residual ERROR sticky — no invent equal-true past EqualsInt residual hole
				if sessHasError(s) {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue fold past EqualsInt residual false
			if sessHasError(s) {
				return false
			}
		}
		return false
	}
	if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
		// incomplete binary operands sticky — no invent not-equal fold
		sessNoteError(s, ErrGeneric)
		return false
	}
	a0, a1 := fi.Args[0], fi.Args[1]
	op, ok := BinaryOpFromString(fi.Binary)
	if !ok {
		return false
	}
	if num == 0 {
		// 0 * x, 0 / x, 0 % x, 0 << x, 0 >> x, 0 && x, 0 & x
		if a0 != nil && a0.EqualsIntSess(s, 0) {
			// residual ERROR sticky — no invent equal-true past EqualsInt residual hole
			if sessHasError(s) {
				return false
			}
			switch op {
			case BinMul, BinDiv, BinMod, BinLShift, BinRShift, BinAnd, BinBitAnd:
				return true
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue fold past a0 EqualsInt residual false
			return false
		}
		// x * 0, x && 0, x & 0
		if a1 != nil && a1.EqualsIntSess(s, 0) {
			// residual ERROR sticky — no invent equal-true past EqualsInt residual hole
			if sessHasError(s) {
				return false
			}
			switch op {
			case BinMul, BinAnd, BinBitAnd:
				return true
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue fold past a1 EqualsInt residual false
			return false
		}
		// a - a, a > a, a < a, a != a (same expression pointer)
		if a0 != nil && a1 != nil && a0 == a1 {
			switch op {
			case BinSub, BinCmpGt, BinCmpLt, BinCmpNe:
				return true
			}
		}
		// x % 1, x % -1
		if a1 != nil {
			eq1 := a1.EqualsIntSess(s, 1)
			// residual ERROR sticky — no invent soft-continue fold past EqualsInt residual
			if sessHasError(s) {
				return false
			}
			eqM1 := a1.EqualsIntSess(s, -1)
			if sessHasError(s) {
				return false
			}
			if (eq1 || eqM1) && op == BinMod {
				return true
			}
		}
	}
	return false
}

// VisitFactsBinaryOrdered mirrors FunctionInvocationBinary::visit_facts for &&/||.
// FunctionInvocationBinary.cpp:487–508 — evaluate left; snapshot post-left;
// evaluate right; merge_facts(post-right, post-left).
// C++ FactVec is full (ePointTo + eUnionWrite). Soft invent was PT-only snapshot/
// merge so RHS ExpressionAssign renew of a union field (e.g. (*p)= via p=&u.f1)
// stayed last=f1 without joining the post-left last=f0 → choose_var ok pool
// gained a nonreadable field (seed-58: n=48 with g_697.f1 vs UP n=47).
// Hard IR incomplete sticky (nil fi/args, incomplete maps); visit policy fails non-sticky.}

func VisitFactsBinaryOrdered(fi *Invocation, cg *CGContext, opts Options) bool {
	// incomplete IR sticky (no soft invent visit success / soft re-pick)
	// param_value[0/1] always live Expression* after ERROR_GUARD
	if fi == nil || cg == nil || len(fi.Args) < 2 {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	if fi.Args[0] == nil || fi.Args[1] == nil {
		noteErrCG(cg, ErrGeneric)
		return false
	}
	// left
	if !VisitFactsExpression(fi.Args[0], cg, opts) {
		return false
	}
	// residual ERROR sticky — no invent soft-continue right/merge past left visit residual
	if hasErrCG(cg) {
		return false
	}
	// FunctionInvocationBinary.cpp:494 — inputs_copy = inputs (full FactVec)
	// incomplete GlobalFacts/UnionFacts sticky
	var afterLeftPT []*FactPointTo
	var afterLeftUnion []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		afterLeftPT = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-continue past CloneFactSlice residual
		if hasErrCG(cg) {
			return false
		}
		// Shallow clone of union pointers: RenewUnionFact replaces live slice
		// entries so post-left still holds the pre-RHS FactUnion* (same as C++
		// Fact* vector copy). Deep clone would freeze post-left at post-left
		// values but is not required for merge_facts semantics.
		afterLeftUnion = CloneUnionFactSliceSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(afterLeftUnion) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
	}
	// right may or may not evaluate (short-circuit still visits for facts merge)
	if !VisitFactsExpression(fi.Args[1], cg, opts) {
		return false
	}
	// residual ERROR sticky — no invent soft-continue merge past right visit residual
	if hasErrCG(cg) {
		return false
	}
	// FunctionInvocationBinary.cpp:499 — merge_facts(inputs, inputs_copy)
	// full FactVec: ePointTo + eUnionWrite
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !FactsComplete(afterLeftPT) ||
			!UnionFactsComplete(cg.FM.UnionFacts) || !UnionFactsComplete(afterLeftUnion) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		_ = MergeFactsSess(sessFromCG(cg), &cg.FM.GlobalFacts, afterLeftPT)
		// residual ERROR sticky — no invent visit success past MergeFacts residual hole
		if hasErrCG(cg) {
			return false
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return false
		}
		// eUnionWrite half of merge_facts (mirror make_random ordered path)
		for _, f := range afterLeftUnion {
			if f == nil {
				noteErrCG(cg, ErrGeneric)
				return false
			}
			cg.FM.UnionFacts = MergeUnionFactIntoSess(sessFromCG(cg), cg.FM.UnionFacts, f)
			if hasErrCG(cg) || !UnionFactsComplete(cg.FM.UnionFacts) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return false
			}
		}
	}
	return true
}
