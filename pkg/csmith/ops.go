// Upstream: Common.h / FunctionInvocation.h eBinaryOps, StatementAssign eAssignOps.
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

// AssignOp mirrors eAssignOps for loop increment.
type AssignOp int

const (
	AssignSimple AssignOp = iota
	AssignAdd
	AssignSub
	AssignPreIncr
	AssignPostIncr
	AssignPreDecr
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

// AssignOpC formats an assignment/increment for a variable name and optional RHS.
func (op AssignOp) AssignOpC(name, rhs string) string {
	switch op {
	case AssignAdd:
		return name + " += " + rhs
	case AssignSub:
		return name + " -= " + rhs
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
