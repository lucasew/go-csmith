// Upstream: SafeOpFlags.h / SafeOpFlags.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// SafeOpSize mirrors SafeOpSize (float last; binary size pick excludes float).
type SafeOpSize int

const (
	SafeInt8 SafeOpSize = iota
	SafeInt16
	SafeInt32
	SafeInt64
	SafeFloat
)

// MaxSafeOpSizeNonFloat is MAX_SAFE_OP_SIZE - 1 for integer size draws.
const MaxSafeOpSizeNonFloat = int(SafeFloat) // 4 values: 0..3

// SafeOpFlags mirrors SafeOpFlags for binary/unary safe math naming.
type SafeOpFlags struct {
	Op1Signed bool
	Op2Signed bool
	IsFunc    bool // ISSUE in upstream: always true
	Size      SafeOpSize
}

// ReturnFloatTypeBinary mirrors SafeOpFlags::return_float_type for binary.
// SafeOpFlags.cpp:113–124.
func ReturnFloatTypeBinary(opts Options, rv, op1, op2 *Type, bop BinaryOp) bool {
	if !opts.EnableFloat {
		return false
	}
	if rv != nil && rv.IsFloat() {
		return true
	}
	if (op1 != nil && op1.IsFloat()) || (op2 != nil && op2.IsFloat()) {
		return true
	}
	if !BinaryOpWorksForFloat(bop) {
		return false
	}
	return false
}

// ReturnFloatTypeUnary mirrors SafeOpFlags::return_float_type for unary.
// SafeOpFlags.cpp:126–136.
func ReturnFloatTypeUnary(opts Options, rv, op1 *Type, uop UnaryOp) bool {
	if !opts.EnableFloat {
		return false
	}
	if rv != nil && rv.IsFloat() {
		return true
	}
	if op1 != nil && op1.IsFloat() {
		return true
	}
	if !UnaryOpWorksForFloat(uop) {
		return false
	}
	return false
}

// Clone mirrors SafeOpFlags::clone.
// SafeOpFlags.cpp:217.
func (f *SafeOpFlags) Clone() *SafeOpFlags {
	if f == nil {
		return nil
	}
	cp := *f
	return &cp
}

// wrapperNames is SafeOpFlags::wrapper_names for to_id.
var wrapperNames []string

// SafeOpFlagsToID mirrors SafeOpFlags::to_id.
// SafeOpFlags.cpp:343–352 — assign stable id to wrapper fname (1-based).
func SafeOpFlagsToID(fname string) int {
	for i, n := range wrapperNames {
		if n == fname {
			return i + 1
		}
	}
	wrapperNames = append(wrapperNames, fname)
	return len(wrapperNames)
}

// ClearSafeOpWrapperNames resets to_id registry (finalization/tests).
func ClearSafeOpWrapperNames() {
	wrapperNames = nil
}

// WrapperNamesCount mirrors SafeOpFlags::wrapper_names.size().
func WrapperNamesCount() int { return len(wrapperNames) }

// OutputWrapperH mirrors DefaultProgramGenerator identify_wrappers wrapper.h body.
// DefaultProgramGenerator.cpp:73–77 — #define N_WRAP <count>.
func OutputWrapperH() string {
	return "#define N_WRAP " + itoa(WrapperNamesCount()) + "\n"
}

// SafeOpKind mirrors SafeOpKind for make_random_binary.
type SafeOpKind int

const (
	// SafeOpBinary is sOpBinary.
	SafeOpBinary SafeOpKind = iota
	// SafeOpAssign is sOpAssign (compound assign).
	SafeOpAssign
)

// SafeMathWrapperAllowed mirrors CGOptions::safe_math_wrapper(id).
// CGOptions.cpp:597–602 — empty list means all allowed.
func SafeMathWrapperAllowed(opts Options, id int) bool {
	if opts.SafeMathWrappers == "" {
		return true
	}
	ids := SplitIntString(opts.SafeMathWrappers, ",")
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// MakeRandomBinary mirrors SafeOpFlags::make_random_binary for binary ops.
// SafeOpFlags.cpp:169–215 — float path when EnableFloat and return_float_type.
func MakeRandomBinary(r *Rng, opts Options, probs *Probabilities, typ *Type) *SafeOpFlags {
	return MakeRandomBinaryKind(r, opts, probs, typ, typ, typ, SafeOpBinary, BinAdd)
}

// MakeRandomUnary mirrors SafeOpFlags::make_random_unary.
// SafeOpFlags.cpp:139–167 — float always signed + SafeFloat; else signed coin + int size.
func MakeRandomUnary(r *Rng, opts Options, probs *Probabilities, rvType, op1Type *Type, uop UnaryOp) *SafeOpFlags {
	// SafeOpFlags.cpp:139–167 — always uses rnd_flipcoin / rnd_upto; no soft invent fixed flags
	if r == nil {
		return nil
	}
	f := &SafeOpFlags{IsFunc: true}
	rvFloat := ReturnFloatTypeUnary(opts, rvType, op1Type, uop)
	// C++ Probabilities singleton always live; nil probs → 0% (no invent default 50)
	sigProb := uint32(0)
	if probs != nil {
		if p := probs.Single(PSafeOpsSignedProb); p >= 0 {
			sigProb = uint32(p)
		}
	}
	// SafeOpFlags.cpp:146–153
	if rvFloat {
		f.Op1Signed = true
	} else {
		f.Op1Signed = r.RndFlipcoin(sigProb)
	}
	f.Op2Signed = f.Op1Signed
	if rvFloat {
		f.Size = SafeFloat
	} else {
		sz, ok := pickSafeOpSize(r, probs)
		if !ok {
			return nil
		}
		f.Size = sz
	}
	return f
}

// MakeRandomBinaryKind mirrors SafeOpFlags::make_random_binary full signature.
// SafeOpFlags.cpp:169–215.
func MakeRandomBinaryKind(
	r *Rng,
	opts Options,
	probs *Probabilities,
	rvType, op1Type, op2Type *Type,
	opKind SafeOpKind,
	bop BinaryOp,
) *SafeOpFlags {
	// SafeOpFlags.cpp:174–175 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtSafeOpFlags, op_kind, nullptr)
	if DepthGuardByTypeFlag(opts, DtSafeOpFlags, int(opKind)) == BadDepth {
		return nil
	}
	// SafeOpFlags.cpp:176+ — always uses rnd_*; no soft invent fixed flags
	if r == nil {
		return nil
	}
	f := &SafeOpFlags{IsFunc: true} // ISSUE upstream: always true
	rvFloat := ReturnFloatTypeBinary(opts, rvType, op1Type, op2Type, bop)

	// SafeOpFlags.cpp:181–190 — float always signed
	// C++ Probabilities singleton always live; nil probs → 0% (no invent default 50)
	sigProb := uint32(0)
	if probs != nil {
		if p := probs.Single(PSafeOpsSignedProb); p >= 0 {
			sigProb = uint32(p)
		}
	}
	if rvFloat {
		f.Op1Signed = true
	} else {
		f.Op1Signed = r.RndFlipcoin(sigProb)
	}

	// SafeOpFlags.cpp:193–201 — op2 for binary; assign copies op1
	if opKind == SafeOpBinary {
		if rvFloat {
			f.Op2Signed = true
		} else {
			f.Op2Signed = r.RndFlipcoin(sigProb)
		}
	} else {
		f.Op2Signed = f.Op1Signed
	}

	// SafeOpFlags.cpp:207–213 — float size vs integer size pick
	if rvFloat {
		f.Size = SafeFloat
	} else {
		sz, ok := pickSafeOpSize(r, probs)
		if !ok {
			return nil
		}
		f.Size = sz
	}
	return f
}

// pickSafeOpSize mirrors rnd_upto(MAX_SAFE_OP_SIZE-1, SAFE_OPS_SIZE_PROB_FILTER).
// SafeOpFlags.cpp:164 / 212 — filter from Probabilities pSafeOpsSizeProb.
// No invent opts-only weight table when probs missing.
func pickSafeOpSize(r *Rng, probs *Probabilities) (SafeOpSize, bool) {
	if r == nil {
		return 0, false
	}
	if probs == nil {
		probs = ProcessProbabilities()
	}
	if probs == nil {
		// Probabilities singleton always live in C++; fail closed
		return 0, false
	}
	v := r.RndUptoFilter(uint32(MaxSafeOpSizeNonFloat), probs.SafeOpsSizeFilter())
	if HasError() {
		return 0, false
	}
	sz := SafeOpSize(v)
	if int(sz) < 0 || int(sz) >= MaxSafeOpSizeNonFloat || probs.SafeOpsSizeWeight(int(sz)) == 0 {
		return 0, false
	}
	return sz, true
}

// SizeToken mirrors OutputSize (without leading 'u' for unsigned).
// SafeOpFlags.cpp:219–242 — assert invalid size; method is const on live flags.
func (f *SafeOpFlags) SizeToken() string {
	if f == nil {
		// no soft invent int32_t for nil flags
		return ""
	}
	var b strings.Builder
	if !f.Op1Signed {
		b.WriteString("u")
	}
	switch f.Size {
	case SafeInt8:
		b.WriteString("int8_t")
	case SafeInt16:
		b.WriteString("int16_t")
	case SafeInt32:
		b.WriteString("int32_t")
	case SafeInt64:
		b.WriteString("int64_t")
	case SafeFloat:
		return "float"
	default:
		// SafeOpFlags.cpp:239 — assert(!"invalid size!"); sticky no soft invent int32_t
		SetError(ErrGeneric)
		return ""
	}
	return b.String()
}

// FlagsToType mirrors SafeOpFlags::flags_to_type.
// SafeOpFlags.cpp:65–98.
func FlagsToType(signed bool, size SafeOpSize) *Type {
	// SafeOpFlags.cpp:65–98 — default: assert(0); return eInt is dead after assert
	if signed {
		switch size {
		case SafeInt8:
			return GetSimpleType(EChar)
		case SafeInt16:
			return GetSimpleType(EShort)
		case SafeInt32:
			return GetSimpleType(EInt)
		case SafeInt64:
			return GetSimpleType(ELongLong)
		case SafeFloat:
			return GetSimpleType(EFloat)
		default:
			// assert(0) path sticky — no soft invent GetIntType for unknown size
			SetError(ErrGeneric)
			return nil
		}
	}
	switch size {
	case SafeInt8:
		return GetSimpleType(EUChar)
	case SafeInt16:
		return GetSimpleType(EUShort)
	case SafeInt32:
		return GetSimpleType(EUInt)
	case SafeInt64:
		return GetSimpleType(EULongLong)
	default:
		// assert(0) path sticky — no soft invent EUInt for unknown size
		SetError(ErrGeneric)
		return nil
	}
}

// LHSType mirrors SafeOpFlags::get_lhs_type.
// SafeOpFlags.cpp:98–102 — flags_to_type(op1_, op_size_); nil flags → nil (no invent).
func (f *SafeOpFlags) LHSType() *Type {
	if f == nil {
		return nil
	}
	return FlagsToType(f.Op1Signed, f.Size)
}

// RHSType mirrors SafeOpFlags::get_rhs_type.
// SafeOpFlags.cpp:104–108 — flags_to_type(op2_, op_size_); nil flags → nil.
func (f *SafeOpFlags) RHSType() *Type {
	if f == nil {
		return nil
	}
	return FlagsToType(f.Op2Signed, f.Size)
}

// BinaryFuncName mirrors SafeOpFlags::to_string(eBinaryOps) for safe arithmetic/shifts.
// SafeOpFlags.cpp:285–320 — float uses safe_*_func_float_f_f.
func (f *SafeOpFlags) BinaryFuncName(op string) string {
	if f == nil {
		return ""
	}
	// SafeOpFlags.cpp:286–287 — float size short-circuit
	if f.Size == SafeFloat {
		return safeFloatFuncString(op)
	}
	var prefix string
	shift := false
	switch op {
	case "+":
		prefix = "safe_add_"
	case "-":
		prefix = "safe_sub_"
	case "*":
		prefix = "safe_mul_"
	case "/":
		prefix = "safe_div_"
	case "%":
		prefix = "safe_mod_"
	case "<<":
		prefix = "safe_lshift_"
		shift = true
	case ">>":
		prefix = "safe_rshift_"
		shift = true
	default:
		// invalid binary op sticky (no invent empty wrapper name)
		SetError(ErrGeneric)
		return ""
	}
	// SafeOpFlags.cpp:239 assert invalid size; sticky no invent safe_add_func__s_s
	sz := f.SizeToken()
	if sz == "" {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	// safe_add_func_int32_t_s_s  /  safe_lshift_func_int32_t_s_u
	var b strings.Builder
	b.WriteString(prefix)
	if f.IsFunc {
		b.WriteString("func_")
	} else {
		b.WriteString("macro_")
	}
	b.WriteString(sz)
	if f.Op1Signed {
		b.WriteString("_s")
	} else {
		b.WriteString("_u")
	}
	// shifts use Op2 sign; other ops repeat Op1 (SafeOpFlags.cpp:318)
	if shift {
		if f.Op2Signed {
			b.WriteString("_s")
		} else {
			b.WriteString("_u")
		}
	} else if f.Op1Signed {
		b.WriteString("_s")
	} else {
		b.WriteString("_u")
	}
	return b.String()
}

// safeFloatFuncString mirrors SafeOpFlags::safe_float_func_string.
// SafeOpFlags.cpp:261–283 — safe_{add,sub,mul,div}_func_float_f_f.
func safeFloatFuncString(op string) string {
	var prefix string
	switch op {
	case "+":
		prefix = "safe_add_"
	case "-":
		prefix = "safe_sub_"
	case "*":
		prefix = "safe_mul_"
	case "/":
		prefix = "safe_div_"
	default:
		// float has no mod/shift wrappers — non-sticky empty (not broken IR invent)
		return ""
	}
	return prefix + "func_float_f_f"
}

// UnaryMinusFuncName mirrors to_string(eMinus).
// SafeOpFlags.cpp:323–341 — no float unary safe function.
func (f *SafeOpFlags) UnaryMinusFuncName() string {
	// live flags required; no soft invent int32 name for nil
	if f == nil {
		return ""
	}
	// SafeOpFlags.cpp:325 — assert(op_size_ != sFloat); non-sticky empty
	// (emit falls through to cast form — soft re-pick / alternate emit path)
	if f.Size == SafeFloat {
		return ""
	}
	sz := f.SizeToken()
	if sz == "" {
		// invalid size assert path sticky
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	var b strings.Builder
	b.WriteString("safe_unary_minus_")
	if f.IsFunc {
		b.WriteString("func_")
	} else {
		b.WriteString("macro_")
	}
	b.WriteString(sz)
	if f.Op1Signed {
		b.WriteString("_s")
	} else {
		b.WriteString("_u")
	}
	return b.String()
}

// SafeOpsBinary reports whether op uses safe wrappers under avoid_signed_overflow.
// FunctionInvocationBinary::safe_ops — add/sub/mul/mod/div/shift.
func SafeOpsBinary(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%", "<<", ">>":
		return true
	default:
		return false
	}
}
