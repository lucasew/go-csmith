// Upstream: Common.h / FunctionInvocation.h eBinaryOps, StatementAssign.h eAssignOps.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// BinaryOp mirrors eBinaryOps (comparison subset used by for-loop tests).
type BinaryOp int

const (
	BinCmpLt BinaryOp = iota
	BinCmpLe
	BinCmpGt
	BinCmpGe
	BinCmpEq
	BinCmpNe
)

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

// CmpOpC returns the C operator string for a comparison.
func (op BinaryOp) CmpOpC() string {
	switch op {
	case BinCmpLt:
		return "<"
	case BinCmpLe:
		return "<="
	case BinCmpGt:
		return ">"
	case BinCmpGe:
		return ">="
	case BinCmpEq:
		return "=="
	case BinCmpNe:
		return "!="
	default:
		return "<"
	}
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
