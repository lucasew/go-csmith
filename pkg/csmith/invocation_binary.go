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
// FunctionInvocationBinary.cpp:184–187 — op_flags size is sFloat.
func (fi *Invocation) IsReturnTypeFloat() bool {
	if fi == nil || fi.Safe == nil {
		return false
	}
	return fi.Safe.Size == SafeFloat
}

// GetType mirrors FunctionInvocation{Unary,Binary,User}::get_type.
// FunctionInvocationUnary.cpp:114–131; FunctionInvocationBinary.cpp:192–241;
// FunctionInvocationUser.cpp:380 — return type.
func (fi *Invocation) GetType() *Type {
	// C++ FunctionInvocation always non-null; incomplete IR → nil (no invent int)
	if fi == nil {
		return nil
	}
	if fi.User != nil {
		// FunctionInvocationUser.cpp:380 — return func->return_type
		return fi.User.ReturnType
	}
	if !fi.IsStd {
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
	// eNot → int; ePlus/eMinus/eBitNot → operand type
	if fi.Unary == "!" {
		return GetIntType()
	}
	if len(fi.Args) >= 1 && fi.Args[0] != nil {
		if t := fi.Args[0].GetType(); t != nil {
			return t
		}
	}
	// C++ uses param_value[0]->get_type(); missing operand → incomplete IR
	return nil
}

// getTypeBinary mirrors FunctionInvocationBinary::get_type.
// FunctionInvocationBinary.cpp:192–241.
func (fi *Invocation) getTypeBinary() *Type {
	// FunctionInvocationBinary.cpp:193–194
	if fi.IsReturnTypeFloat() {
		return GetSimpleType(EFloat)
	}
	op, ok := BinaryOpFromString(fi.Binary)
	// FunctionInvocationBinary.cpp:196–199 — assert invalid operator; no soft invent eInt
	if !ok {
		return nil
	}
	switch op {
	case BinAdd, BinSub, BinMul, BinDiv, BinMod, BinBitXor, BinBitAnd, BinBitOr:
		// FunctionInvocationBinary.cpp:208–224 — both signed → int else uint
		ls, rs := true, true
		if len(fi.Args) >= 1 && fi.Args[0] != nil {
			if t := fi.Args[0].GetType(); t != nil {
				ls = t.IsSigned()
			}
		}
		if len(fi.Args) >= 2 && fi.Args[1] != nil {
			if t := fi.Args[1].GetType(); t != nil {
				rs = t.IsSigned()
			}
		}
		if ls && rs {
			return GetSimpleType(EInt)
		}
		return GetSimpleType(EUInt)
	case BinCmpGt, BinCmpLt, BinCmpGe, BinCmpLe, BinCmpEq, BinCmpNe, BinAnd, BinOr:
		return GetIntType()
	case BinLShift, BinRShift:
		// FunctionInvocationBinary.cpp:229–238 — follow left signedness
		if len(fi.Args) >= 1 && fi.Args[0] != nil {
			if t := fi.Args[0].GetType(); t != nil && !t.IsSigned() {
				return GetSimpleType(EUInt)
			}
		}
		return GetSimpleType(EInt)
	default:
		// FunctionInvocationBinary.cpp:240–241 — assert(0); no soft invent eInt
		return nil
	}
}

// SafeInvocation mirrors FunctionInvocation::safe_invocation.
// Unary: eMinus false (FunctionInvocationUnary.cpp:185–187);
// Binary: always false; User: always true.
func (fi *Invocation) SafeInvocation() bool {
	if fi == nil {
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
func (fi *Invocation) CompatibleVar(v *Variable, expandStruct bool) bool {
	if fi == nil || v == nil || !fi.IsStd || !fi.IsUnary {
		return false
	}
	if len(fi.Args) == 0 || fi.Args[0] == nil {
		return false
	}
	return fi.Args[0].CompatibleWithVar(v, expandStruct)
}

// Is0Or1 mirrors FunctionInvocationBinary::is_0_or_1 and Unary eNot.
// FunctionInvocationBinary.cpp:179–181 — comparison ops yield 0/1.
// FunctionInvocationUnary.h:67 — eNot only.
func (fi *Invocation) Is0Or1() bool {
	if fi == nil || !fi.IsStd {
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
func (fi *Invocation) EqualsInt(num int) bool {
	if fi == nil || !fi.IsStd {
		return false
	}
	// FunctionInvocationUnary.cpp:144–156
	if fi.IsUnary {
		if len(fi.Args) < 1 || fi.Args[0] == nil {
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
	if len(fi.Args) < 2 {
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
func VisitFactsBinaryOrdered(fi *Invocation, cg *CGContext, opts Options) bool {
	// incomplete IR — fail closed (no soft invent visit success)
	if fi == nil || cg == nil || len(fi.Args) < 2 {
		return false
	}
	// left
	if fi.Args[0] != nil && !VisitFactsExpression(fi.Args[0], cg, opts) {
		return false
	}
	// snapshot after left
	var afterLeft []*FactPointTo
	if cg.FM != nil {
		afterLeft = CloneFactSlice(cg.FM.GlobalFacts)
	}
	// right may or may not evaluate
	if fi.Args[1] != nil && !VisitFactsExpression(fi.Args[1], cg, opts) {
		return false
	}
	// merge post-right with post-left
	if cg.FM != nil {
		MergeFacts(&cg.FM.GlobalFacts, afterLeft)
	}
	return true
}
