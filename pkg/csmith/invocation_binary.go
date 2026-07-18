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

// EqualsInt mirrors FunctionInvocationBinary::equals for constant folding.
// FunctionInvocationBinary.cpp:154–177.
func (fi *Invocation) EqualsInt(num int) bool {
	if fi == nil || !fi.IsStd || fi.IsUnary || len(fi.Args) < 2 {
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
	if fi == nil || cg == nil || len(fi.Args) < 2 {
		return true
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
