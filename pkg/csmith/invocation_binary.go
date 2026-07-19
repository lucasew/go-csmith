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
func GetBinopString(op BinaryOp) string {
	return op.BinaryOpC()
}

// IsReturnTypeFloat mirrors FunctionInvocationBinary::is_return_type_float.
// FunctionInvocationBinary.cpp:184–187 — assert(op_flags); size is sFloat.
// Missing Safe is complete non-float (not sticky) — Safe is optional on std binary
// shells; invent would be treating nil as float, not non-float.
func (fi *Invocation) IsReturnTypeFloat() bool {
	// nil inv sticky incomplete; missing Safe alone → complete false (not float)
	if fi == nil {
		SetError(ErrGeneric)
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
	// C++ FunctionInvocation always non-null; sticky nil → no invent int type shell
	if fi == nil {
		SetError(ErrGeneric)
		return nil
	}
	if fi.User != nil {
		// FunctionInvocationUser.cpp:380 — return func->return_type
		// nil ReturnType incomplete sticky
		if fi.User.ReturnType == nil {
			SetError(ErrGeneric)
			return nil
		}
		return fi.User.ReturnType
	}
	if !fi.IsStd {
		// incomplete non-user non-std shell sticky
		SetError(ErrGeneric)
		return nil
	}
	if fi.IsUnary {
		return fi.getTypeUnary()
	}
	return fi.getTypeBinary()
}

// getTypeUnary mirrors FunctionInvocationUnary::get_type.
// FunctionInvocationUnary.cpp:114–131.
func (fi *Invocation) getTypeUnary() *Type {
	// FunctionInvocationUnary.cpp:116–129 — switch on known ops only
	switch fi.Unary {
	case "!":
		return GetIntType()
	case "+", "-", "~":
		// C++ param_value[0]->get_type(); missing operand → incomplete IR sticky
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			SetError(ErrGeneric)
			return nil
		}
		return fi.Args[0].GetType()
	default:
		// FunctionInvocationUnary.cpp:117 assert invalid operator sticky; no invent eInt
		SetError(ErrGeneric)
		return nil
	}
}

// getTypeBinary mirrors FunctionInvocationBinary::get_type.
// FunctionInvocationBinary.cpp:192–241.
func (fi *Invocation) getTypeBinary() *Type {
	// FunctionInvocationBinary.cpp:193–194
	if fi.IsReturnTypeFloat() {
		return GetSimpleType(EFloat)
	}
	op, ok := BinaryOpFromString(fi.Binary)
	// FunctionInvocationBinary.cpp:196–199 — assert invalid operator sticky; no soft invent eInt
	if !ok {
		SetError(ErrGeneric)
		return nil
	}
	switch op {
	case BinAdd, BinSub, BinMul, BinDiv, BinMod, BinBitXor, BinBitAnd, BinBitOr:
		// FunctionInvocationBinary.cpp:208–224 — param_value[0/1]->get_type always live
		// missing operands → incomplete IR sticky (no invent signed=true → eInt)
		if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
			SetError(ErrGeneric)
			return nil
		}
		lt, rt := fi.Args[0].GetType(), fi.Args[1].GetType()
		if lt == nil || rt == nil {
			SetError(ErrGeneric)
			return nil
		}
		if lt.IsSigned() && rt.IsSigned() {
			return GetSimpleType(EInt)
		}
		return GetSimpleType(EUInt)
	case BinCmpGt, BinCmpLt, BinCmpGe, BinCmpLe, BinCmpEq, BinCmpNe, BinAnd, BinOr:
		return GetIntType()
	case BinLShift, BinRShift:
		// FunctionInvocationBinary.cpp:229–238 — param_value[0]->get_type always sticky
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			SetError(ErrGeneric)
			return nil
		}
		lt := fi.Args[0].GetType()
		if lt == nil {
			SetError(ErrGeneric)
			return nil
		}
		if lt.IsSigned() {
			return GetSimpleType(EInt)
		}
		return GetSimpleType(EUInt)
	default:
		// FunctionInvocationBinary.cpp:240–241 — assert(0) sticky; no soft invent eInt
		SetError(ErrGeneric)
		return nil
	}
}

// SafeInvocation mirrors FunctionInvocation::safe_invocation.
// Unary: eMinus false (FunctionInvocationUnary.cpp:185–187);
// Binary: always false; User: always true.
// Incomplete Invocation sticky false (no invent unsafe soft-skip / soft re-pick).
func (fi *Invocation) SafeInvocation() bool {
	// Invocation always live; sticky incomplete no invent not-safe soft-skip
	if fi == nil {
		SetError(ErrGeneric)
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
// Incomplete unary operand sticky false (no invent soft-skip / soft re-pick past hole).
func (fi *Invocation) CompatibleVar(v *Variable, expandStruct bool) bool {
	// Invocation always live; non-unary-std complete false (C++ binary/user)
	if fi == nil {
		SetError(ErrGeneric)
		return false
	}
	if v == nil {
		// FunctionInvocationUnary.cpp assert path via operand.compatible(v)
		SetError(ErrGeneric)
		return false
	}
	if !fi.IsStd || !fi.IsUnary {
		return false
	}
	if len(fi.Args) == 0 || fi.Args[0] == nil {
		// incomplete unary operand sticky — no invent not-compatible soft-skip
		SetError(ErrGeneric)
		return false
	}
	return fi.Args[0].CompatibleWithVar(v, expandStruct)
}

// Is0Or1 mirrors FunctionInvocationBinary::is_0_or_1 and Unary eNot.
// FunctionInvocationBinary.cpp:179–181 — comparison ops yield 0/1.
// FunctionInvocationUnary.h:67 — eNot only.
// Incomplete Invocation sticky false (no invent not-0or1 / soft re-pick past hole).
func (fi *Invocation) Is0Or1() bool {
	// Invocation always live for fold; sticky incomplete no invent not-0or1
	if fi == nil {
		SetError(ErrGeneric)
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
// Incomplete param IR sticky false (no invent not-equal fold / soft re-pick past holes).
func (fi *Invocation) EqualsInt(num int) bool {
	// Invocation always live for fold; sticky incomplete no invent not-equal
	if fi == nil {
		SetError(ErrGeneric)
		return false
	}
	if !fi.IsStd {
		return false
	}
	// FunctionInvocationUnary.cpp:145 — assert(!param_value.empty())
	// FunctionInvocationBinary.cpp:155 — assert(param_value.size() == 2)
	if fi.IsUnary {
		if len(fi.Args) < 1 || fi.Args[0] == nil {
			SetError(ErrGeneric)
			return false
		}
		a0 := fi.Args[0]
		if num == 0 && fi.Unary == "!" && a0.NotEquals(0) {
			return true
		}
		if num == 1 && fi.Unary == "!" && a0.EqualsInt(0) {
			return true
		}
		if fi.Unary == "-" && a0.EqualsInt(-num) {
			return true
		}
		return false
	}
	if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
		// incomplete binary operands sticky — no invent not-equal fold
		SetError(ErrGeneric)
		return false
	}
	a0, a1 := fi.Args[0], fi.Args[1]
	op, ok := BinaryOpFromString(fi.Binary)
	if !ok {
		return false
	}
	if num == 0 {
		// 0 * x, 0 / x, 0 % x, 0 << x, 0 >> x, 0 && x, 0 & x
		if a0 != nil && a0.EqualsInt(0) {
			switch op {
			case BinMul, BinDiv, BinMod, BinLShift, BinRShift, BinAnd, BinBitAnd:
				return true
			}
		}
		// x * 0, x && 0, x & 0
		if a1 != nil && a1.EqualsInt(0) {
			switch op {
			case BinMul, BinAnd, BinBitAnd:
				return true
			}
		}
		// a - a, a > a, a < a, a != a (same expression pointer)
		if a0 != nil && a1 != nil && a0 == a1 {
			switch op {
			case BinSub, BinCmpGt, BinCmpLt, BinCmpNe:
				return true
			}
		}
		// x % 1, x % -1
		if a1 != nil && (a1.EqualsInt(1) || a1.EqualsInt(-1)) && op == BinMod {
			return true
		}
	}
	return false
}

// VisitFactsBinaryOrdered mirrors FunctionInvocationBinary::visit_facts for &&/||.
// FunctionInvocationBinary.cpp:487–508 — evaluate left; merge right with post-left.
// Hard IR incomplete sticky (nil fi/args, incomplete maps); visit policy fails non-sticky.
func VisitFactsBinaryOrdered(fi *Invocation, cg *CGContext, opts Options) bool {
	// incomplete IR sticky (no soft invent visit success / soft re-pick)
	// param_value[0/1] always live Expression* after ERROR_GUARD
	if fi == nil || cg == nil || len(fi.Args) < 2 {
		SetError(ErrGeneric)
		return false
	}
	if fi.Args[0] == nil || fi.Args[1] == nil {
		SetError(ErrGeneric)
		return false
	}
	// left
	if !VisitFactsExpression(fi.Args[0], cg, opts) {
		return false
	}
	// snapshot after left — incomplete GlobalFacts sticky
	var afterLeft []*FactPointTo
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		afterLeft = CloneFactSlice(cg.FM.GlobalFacts)
	}
	// right may or may not evaluate (short-circuit still visits for facts merge)
	if !VisitFactsExpression(fi.Args[1], cg, opts) {
		return false
	}
	// merge post-right with post-left
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !FactsComplete(afterLeft) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		// MergeFacts sticky on incomplete mid-join
		_ = MergeFacts(&cg.FM.GlobalFacts, afterLeft)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
	}
	return true
}
