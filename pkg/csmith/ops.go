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
	return op.BinaryOpCSess(testAmbientSession)
}

// BinaryOpCSess is BinaryOpC with explicit session residual sticky.
func (op BinaryOp) BinaryOpCSess(s *Session) string {
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
		// invalid op sticky; no soft invent "+"
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// CmpOpC returns the C operator string for a comparison (for-loop tests).
func (op BinaryOp) CmpOpC() string {
	return op.CmpOpCSess(testAmbientSession)
}

// CmpOpCSess is CmpOpC with explicit session residual sticky.
func (op BinaryOp) CmpOpCSess(s *Session) string {
	switch op {
	case BinCmpLt, BinCmpLe, BinCmpGt, BinCmpGe, BinCmpEq, BinCmpNe:
		return op.BinaryOpCSess(s)
	default:
		// invalid cmp sticky; no soft invent "<"
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// BinaryOpsFilter mirrors BINARY_OPS_PROB_FILTER via process Probabilities.
// No invent NewProbabilities(opts) one-off table when process unset — reject-all.
func BinaryOpsFilter(opts Options) Filter {
	return BinaryOpsFilterSess(testAmbientSession, opts)
}

// BinaryOpsFilterSess is BinaryOpsFilter using session Probabilities.
func BinaryOpsFilterSess(s *Session, opts Options) Filter {
	return BinaryOpsFilterProbs(opts, sessProbs(s))
}

// BinaryOpsFilterProbs uses an explicit Probabilities bag (session path).
func BinaryOpsFilterProbs(opts Options, probs *Probabilities) Filter {
	_ = opts
	if probs != nil {
		// Prefer bag-bound ProbabilityFilter (implements Filter interface).
		if f, ok := probs.probFilters[PBinaryOpsProb]; ok && f != nil {
			return f
		}
		return filterFunc(func(v uint32) bool {
			if v < 0 || int(v) >= len(probs.binaryOpWeight) {
				return true
			}
			return probs.binaryOpWeight[v] == 0
		})
	}
	// C++ GetInstance always live; fail closed reject every op
	return filterFunc(func(v uint32) bool { return true })
}

// PickBinaryOp mirrors rnd_upto(MAX_BINARY_OP, BINARY_OPS_PROB_FILTER()).
// FunctionInvocation.cpp:179–183 — filter from Probabilities pBinaryOpsProb.
func PickBinaryOp(r *Rng, opts Options) BinaryOp {
	return PickBinaryOpSess(testAmbientSession, r, opts)
}

// PickBinaryOpSess is PickBinaryOp using session Probabilities.
func PickBinaryOpSess(s *Session, r *Rng, opts Options) BinaryOp {
	return PickBinaryOpProbsSess(s, r, opts, sessProbs(s))
}

// PickBinaryOpProbs is PickBinaryOp with an explicit Probabilities bag.
func PickBinaryOpProbs(r *Rng, opts Options, probs *Probabilities) BinaryOp {
	return PickBinaryOpProbsSess(testAmbientSession, r, opts, probs)
}

// PickBinaryOpProbsSess is PickBinaryOpProbs with explicit session residual sticky.
func PickBinaryOpProbsSess(s *Session, r *Rng, opts Options, probs *Probabilities) BinaryOp {
	// FunctionInvocation.cpp:179–183 — always rnd_upto; sticky no invent eAdd without draw
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return BinaryOp(MaxBinaryOp)
	}
	// BINARY_OPS_PROB_FILTER uses session Probabilities group (no invent opts-only
	// filter when session singleton is live).
	if probs == nil {
		// library path without NewProgramGenerator — fail closed MAX (non-sticky:
		// sticky poisons unit paths that omit process Probabilities singleton)
		_ = opts
		return BinaryOp(MaxBinaryOp)
	}
	return BinaryOp(r.RndUptoFilterSess(s, uint32(MaxBinaryOp), probs.BinaryOpsFilterSess(s)))
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
	return op.UnaryOpCSess(testAmbientSession)
}

// UnaryOpCSess is UnaryOpC with explicit session residual sticky.
func (op UnaryOp) UnaryOpCSess(s *Session) string {
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
		// invalid unary sticky; no soft invent "-"
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// UnaryOpsFilter mirrors UNARY_OPS_PROB_FILTER via process Probabilities.
// No invent NewProbabilities(opts) when process unset — reject-all.
func UnaryOpsFilter(opts Options) Filter {
	return UnaryOpsFilterSess(testAmbientSession, opts)
}

// UnaryOpsFilterSess is UnaryOpsFilter using session Probabilities.
func UnaryOpsFilterSess(s *Session, opts Options) Filter {
	return UnaryOpsFilterProbs(opts, sessProbs(s))
}

// UnaryOpsFilterProbs uses an explicit Probabilities bag (session path).
func UnaryOpsFilterProbs(opts Options, probs *Probabilities) Filter {
	_ = opts
	if probs != nil {
		if f, ok := probs.probFilters[PUnaryOpsProb]; ok && f != nil {
			return f
		}
		return filterFunc(func(v uint32) bool {
			if int(v) < 0 || int(v) >= len(probs.unaryOpWeight) {
				return true
			}
			return probs.unaryOpWeight[v] == 0
		})
	}
	return filterFunc(func(v uint32) bool { return true })
}

// PickUnaryOp mirrors rnd_upto(MAX_UNARY_OP, UNARY_OPS_PROB_FILTER()).
// FunctionInvocation.cpp:146–148 — filter from Probabilities pUnaryOpsProb.
func PickUnaryOp(r *Rng, opts Options) UnaryOp {
	return PickUnaryOpSess(testAmbientSession, r, opts)
}

// PickUnaryOpSess is PickUnaryOp using session Probabilities.
func PickUnaryOpSess(s *Session, r *Rng, opts Options) UnaryOp {
	return PickUnaryOpProbsSess(s, r, opts, sessProbs(s))
}

// PickUnaryOpProbs is PickUnaryOp with an explicit Probabilities bag.
func PickUnaryOpProbs(r *Rng, opts Options, probs *Probabilities) UnaryOp {
	return PickUnaryOpProbsSess(testAmbientSession, r, opts, probs)
}

// PickUnaryOpProbsSess is PickUnaryOpProbs with explicit session residual sticky.
func PickUnaryOpProbsSess(s *Session, r *Rng, opts Options, probs *Probabilities) UnaryOp {
	// FunctionInvocation.cpp:146–148 — always rnd_upto; sticky no invent eMinus without draw
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return UnaryOp(MaxUnaryOp)
	}
	if probs == nil {
		// library path without session probs — fail closed MAX (non-sticky soft
		// re-pick for unit paths without process Probabilities)
		_ = opts
		return UnaryOp(MaxUnaryOp)
	}
	return UnaryOp(r.RndUptoFilterSess(s, uint32(MaxUnaryOp), probs.UnaryOpsFilterSess(s)))
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
// StatementAssign.cpp:515–537 — lhs always live; rhs live when op needs it.
// sticky no invent " = x" / "++" / "g = " empty-side shells
func (op AssignOp) AssignOpC(name, rhs string) string {
	return op.AssignOpCSess(testAmbientSession, name, rhs)
}

// AssignOpCSess is AssignOpC with explicit session residual sticky.
func (op AssignOp) AssignOpCSess(s *Session, name, rhs string) string {
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !op.NeedNoRHS() && rhs == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
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
		// invalid assign op sticky; no soft invent simple assign
		sessNoteError(s, ErrGeneric)
		return ""
	}
}
