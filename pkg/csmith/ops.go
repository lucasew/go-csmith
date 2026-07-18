// Upstream: Common.h / FunctionInvocation.h eBinaryOps, StatementAssign.h eAssignOps.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// BinaryOp mirrors eBinaryOps (FunctionInvocation.h order).
type BinaryOp int

const (
	BinAdd BinaryOp = iota
	BinSub
	BinMul
	BinDiv
	BinMod
	BinCmpGt
	BinCmpLt
	BinCmpGe
	BinCmpLe
	BinCmpEq
	BinCmpNe
	BinAnd
	BinOr
	BinBitXor
	BinBitAnd
	BinBitOr
	BinRShift
	BinLShift
)

// MaxBinaryOp mirrors MAX_BINARY_OP.
const MaxBinaryOp = int(BinLShift) + 1

// AssignOp mirrors eAssignOps (StatementAssign.h order).
type AssignOp int

const (
	AssignSimple AssignOp = iota
	AssignMul
	AssignDiv
	AssignRem
	AssignAdd
	AssignSub
	AssignLShift
	AssignRShift
	AssignBitAnd
	AssignBitXor
	AssignBitOr
	AssignPreIncr
	AssignPreDecr
	AssignPostIncr
	AssignPostDecr
)

// BinaryOpC returns the C operator token for eBinaryOps.
// FunctionInvocationBinary::Output / GetOpString subset.
func (op BinaryOp) BinaryOpC() string {
	switch op {
	case BinAdd:
		return "+"
	case BinSub:
		return "-"
	case BinMul:
		return "*"
	case BinDiv:
		return "/"
	case BinMod:
		return "%"
	case BinCmpGt:
		return ">"
	case BinCmpLt:
		return "<"
	case BinCmpGe:
		return ">="
	case BinCmpLe:
		return "<="
	case BinCmpEq:
		return "=="
	case BinCmpNe:
		return "!="
	case BinAnd:
		return "&&"
	case BinOr:
		return "||"
	case BinBitXor:
		return "^"
	case BinBitAnd:
		return "&"
	case BinBitOr:
		return "|"
	case BinRShift:
		return ">>"
	case BinLShift:
		return "<<"
	default:
		return "+"
	}
}

// CmpOpC returns the C operator string for a comparison (for-loop tests).
func (op BinaryOp) CmpOpC() string {
	switch op {
	case BinCmpLt, BinCmpLe, BinCmpGt, BinCmpGe, BinCmpEq, BinCmpNe:
		return op.BinaryOpC()
	default:
		return "<"
	}
}

// BinaryOpsFilter mirrors BINARY_OPS_PROB_FILTER — reject zero-weight ops.
// Probabilities.cpp set_default_binary_ops_prob (equal group, weight 0/1).
func BinaryOpsFilter(opts Options) Filter {
	w := make([]int, MaxBinaryOp)
	for i := range w {
		w[i] = 1
	}
	if !opts.Muls {
		w[BinMul] = 0
	}
	if !opts.Divs {
		w[BinDiv] = 0
	}
	return filterFunc(func(v uint32) bool {
		i := int(v)
		return i < 0 || i >= len(w) || w[i] == 0
	})
}

// PickBinaryOp mirrors rnd_upto(MAX_BINARY_OP, BINARY_OPS_PROB_FILTER()).
// FunctionInvocation.cpp:179–183.
func PickBinaryOp(r *Rng, opts Options) BinaryOp {
	if r == nil {
		return BinAdd
	}
	return BinaryOp(r.RndUptoFilter(uint32(MaxBinaryOp), BinaryOpsFilter(opts)))
}

// IsOrderedBinary mirrors FunctionInvocation::IsOrderedStandardFunc (&& / ||).
func IsOrderedBinary(op BinaryOp) bool {
	return op == BinAnd || op == BinOr
}

// BinaryOpWorksForFloat mirrors FunctionInvocation::BinaryOpWorksForFloat.
func BinaryOpWorksForFloat(op BinaryOp) bool {
	switch op {
	case BinAdd, BinSub, BinMul, BinDiv,
		BinCmpGt, BinCmpLt, BinCmpGe, BinCmpLe, BinCmpEq, BinCmpNe:
		return true
	default:
		return false
	}
}

// UnaryOp mirrors eUnaryOps (FunctionInvocation.h order).
type UnaryOp int

const (
	UnPlus UnaryOp = iota
	UnMinus
	UnNot
	UnBitNot
)

// MaxUnaryOp mirrors MAX_UNARY_OP.
const MaxUnaryOp = int(UnBitNot) + 1

// UnaryOpC returns the C token for eUnaryOps.
func (op UnaryOp) UnaryOpC() string {
	switch op {
	case UnPlus:
		return "+"
	case UnMinus:
		return "-"
	case UnNot:
		return "!"
	case UnBitNot:
		return "~"
	default:
		return "-"
	}
}

// UnaryOpsFilter mirrors UNARY_OPS_PROB_FILTER — reject weight-0 ops.
// Probabilities.cpp set_default_unary_ops_prob.
func UnaryOpsFilter(opts Options) Filter {
	w := make([]int, MaxUnaryOp)
	if opts.UnaryPlusOperator {
		w[UnPlus] = 1
	}
	w[UnMinus] = 1
	w[UnNot] = 1
	w[UnBitNot] = 1
	return filterFunc(func(v uint32) bool {
		i := int(v)
		return i < 0 || i >= len(w) || w[i] == 0
	})
}

// PickUnaryOp mirrors rnd_upto(MAX_UNARY_OP, UNARY_OPS_PROB_FILTER()).
func PickUnaryOp(r *Rng, opts Options) UnaryOp {
	if r == nil {
		return UnMinus
	}
	return UnaryOp(r.RndUptoFilter(uint32(MaxUnaryOp), UnaryOpsFilter(opts)))
}

// NeedNoRHS mirrors StatementAssign::need_no_rhs.
func (op AssignOp) NeedNoRHS() bool {
	return op == AssignPreIncr || op == AssignPreDecr || op == AssignPostIncr || op == AssignPostDecr
}

// AssignOpC formats an assignment for a variable name and optional RHS.
func (op AssignOp) AssignOpC(name, rhs string) string {
	switch op {
	case AssignSimple:
		return name + " = " + rhs
	case AssignMul:
		return name + " *= " + rhs
	case AssignDiv:
		return name + " /= " + rhs
	case AssignRem:
		return name + " %= " + rhs
	case AssignAdd:
		return name + " += " + rhs
	case AssignSub:
		return name + " -= " + rhs
	case AssignLShift:
		return name + " <<= " + rhs
	case AssignRShift:
		return name + " >>= " + rhs
	case AssignBitAnd:
		return name + " &= " + rhs
	case AssignBitXor:
		return name + " ^= " + rhs
	case AssignBitOr:
		return name + " |= " + rhs
	case AssignPreIncr:
		return "++" + name
	case AssignPostIncr:
		return name + "++"
	case AssignPreDecr:
		return "--" + name
	case AssignPostDecr:
		return name + "--"
	default:
		return name + " = " + rhs
	}
}
