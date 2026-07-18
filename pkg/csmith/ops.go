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
		// invalid op; no soft invent "+"
		return ""
	}
}

// CmpOpC returns the C operator string for a comparison (for-loop tests).
func (op BinaryOp) CmpOpC() string {
	switch op {
	case BinCmpLt, BinCmpLe, BinCmpGt, BinCmpGe, BinCmpEq, BinCmpNe:
		return op.BinaryOpC()
	default:
		// invalid cmp; no soft invent "<"
		return ""
	}
}

// BinaryOpsFilter mirrors BINARY_OPS_PROB_FILTER from opts defaults only.
// Prefer Probabilities.BinaryOpsFilter via ProcessProbabilities (session singleton).
// Kept for library tests that build a one-off filter without process probs.
func BinaryOpsFilter(opts Options) Filter {
	return NewProbabilities(opts).BinaryOpsFilter()
}

// PickBinaryOp mirrors rnd_upto(MAX_BINARY_OP, BINARY_OPS_PROB_FILTER()).
// FunctionInvocation.cpp:179–183 — filter from Probabilities pBinaryOpsProb.
func PickBinaryOp(r *Rng, opts Options) BinaryOp {
	// FunctionInvocation.cpp:179–183 — always rnd_upto; no soft invent eAdd
	if r == nil {
		return BinaryOp(MaxBinaryOp)
	}
	// BINARY_OPS_PROB_FILTER uses process Probabilities group (no invent opts-only
	// filter when session singleton is live).
	probs := ProcessProbabilities()
	if probs == nil {
		// library path without NewProgramGenerator — fail closed MAX
		_ = opts
		return BinaryOp(MaxBinaryOp)
	}
	return BinaryOp(r.RndUptoFilter(uint32(MaxBinaryOp), probs.BinaryOpsFilter()))
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

// UnaryOpWorksForFloat mirrors FunctionInvocation::UnaryOpWorksForFloat.
// FunctionInvocation.cpp:646–654 — +, -, ! only.
func UnaryOpWorksForFloat(op UnaryOp) bool {
	switch op {
	case UnPlus, UnMinus, UnNot:
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
		// invalid unary; no soft invent "-"
		return ""
	}
}

// UnaryOpsFilter mirrors UNARY_OPS_PROB_FILTER from opts defaults only.
// Prefer Probabilities.UnaryOpsFilter via ProcessProbabilities.
func UnaryOpsFilter(opts Options) Filter {
	return NewProbabilities(opts).UnaryOpsFilter()
}

// PickUnaryOp mirrors rnd_upto(MAX_UNARY_OP, UNARY_OPS_PROB_FILTER()).
// FunctionInvocation.cpp:146–148 — filter from Probabilities pUnaryOpsProb.
func PickUnaryOp(r *Rng, opts Options) UnaryOp {
	// FunctionInvocation.cpp:146–148 — always rnd_upto; no soft invent eMinus
	if r == nil {
		return UnaryOp(MaxUnaryOp)
	}
	probs := ProcessProbabilities()
	if probs == nil {
		// library path without session probs — fail closed MAX
		_ = opts
		return UnaryOp(MaxUnaryOp)
	}
	return UnaryOp(r.RndUptoFilter(uint32(MaxUnaryOp), probs.UnaryOpsFilter()))
}

// NeedNoRHS mirrors StatementAssign::need_no_rhs.
func (op AssignOp) NeedNoRHS() bool {
	return op == AssignPreIncr || op == AssignPreDecr || op == AssignPostIncr || op == AssignPostDecr
}

// CompoundToBinaryOps mirrors StatementAssign::compound_to_binary_ops.
// StatementAssign.cpp:306+ — maps compound assign to eBinaryOps; MAX when none.
func (op AssignOp) CompoundToBinaryOps() (BinaryOp, bool) {
	switch op {
	case AssignAdd, AssignPreIncr, AssignPostIncr:
		return BinAdd, true
	case AssignSub, AssignPreDecr, AssignPostDecr:
		return BinSub, true
	case AssignMul:
		return BinMul, true
	case AssignDiv:
		return BinDiv, true
	case AssignRem:
		return BinMod, true
	case AssignBitAnd:
		return BinBitAnd, true
	case AssignBitXor:
		return BinBitXor, true
	case AssignBitOr:
		return BinBitOr, true
	case AssignLShift:
		return BinLShift, true
	case AssignRShift:
		return BinRShift, true
	default:
		// MAX when none; no soft invent eAdd with ok=false
		return BinaryOp(MaxBinaryOp), false
	}
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
		// invalid assign op; no soft invent simple assign
		return ""
	}
}
