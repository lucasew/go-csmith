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
	return ReturnFloatTypeBinarySess(nil, opts, rv, op1, op2, bop)
}

func ReturnFloatTypeBinarySess(s *Session, opts Options, rv, op1, op2 *Type, bop BinaryOp) bool {
	if !opts.EnableFloat {
		return false
	}
	if rv != nil && rv.IsFloatSess(s) {
		// residual ERROR sticky — no invent float-true past rv IsFloat residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past rv IsFloat residual false
	if sessHasError(s) {
		return false
	}
	if op1 != nil && op1.IsFloatSess(s) {
		// residual ERROR sticky — no invent float-true past op1 IsFloat residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past op1 IsFloat residual false
	if sessHasError(s) {
		return false
	}
	if op2 != nil && op2.IsFloatSess(s) {
		// residual ERROR sticky — no invent float-true past op2 IsFloat residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past op2 IsFloat residual false
	if sessHasError(s) {
		return false
	}
	if !BinaryOpWorksForFloat(bop) {
		return false
	}
	return false
}

// ReturnFloatTypeUnary mirrors SafeOpFlags::return_float_type for unary.
// SafeOpFlags.cpp:126–136.}

func ReturnFloatTypeUnary(opts Options, rv, op1 *Type, uop UnaryOp) bool {
	return ReturnFloatTypeUnarySess(nil, opts, rv, op1, uop)
}

func ReturnFloatTypeUnarySess(s *Session, opts Options, rv, op1 *Type, uop UnaryOp) bool {
	if !opts.EnableFloat {
		return false
	}
	if rv != nil && rv.IsFloatSess(s) {
		// residual ERROR sticky — no invent float-true past rv IsFloat residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past rv IsFloat residual false
	if sessHasError(s) {
		return false
	}
	if op1 != nil && op1.IsFloatSess(s) {
		// residual ERROR sticky — no invent float-true past op1 IsFloat residual hole
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past op1 IsFloat residual false
	if sessHasError(s) {
		return false
	}
	if !UnaryOpWorksForFloat(uop) {
		return false
	}
	return false
}

// Clone mirrors SafeOpFlags::clone.
// SafeOpFlags.cpp:217.
// SafeOpFlags* always live at clone; sticky nil (no invent soft-skip past hole).}

func (f *SafeOpFlags) Clone() *SafeOpFlags {
	return f.CloneSess(nil)
}

func (f *SafeOpFlags) CloneSess(s *Session) *SafeOpFlags {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	cp := *f
	return &cp
}

// currentSession().WrapperNames is SafeOpFlags::wrapper_names for to_id.

// SafeOpFlagsToID mirrors SafeOpFlags::to_id.
// SafeOpFlags.cpp:343–352 — assign stable id to wrapper fname (1-based).
func SafeOpFlagsToID(fname string) int {
	return SafeOpFlagsToIDSess(nil, fname)
}

// SafeOpFlagsToIDSess is SafeOpFlagsToID on an explicit session bag.
// Nil s uses quarantined unit-test ambient bag (testAmbientSession).
func SafeOpFlagsToIDSess(s *Session, fname string) int {
	s = sessOrAmbient(s)
	for i, n := range s.WrapperNames {
		if n == fname {
			return i + 1
		}
	}
	s.WrapperNames = append(s.WrapperNames, fname)
	return len(s.WrapperNames)
}

// ClearSafeOpWrapperNames resets to_id registry (finalization/tests).
func ClearSafeOpWrapperNames() {
	ClearSafeOpWrapperNamesSess(nil)
}

// ClearSafeOpWrapperNamesSess clears wrapper names on an explicit session bag.
func ClearSafeOpWrapperNamesSess(s *Session) {
	sessOrAmbient(s).WrapperNames = nil
}

// WrapperNamesCount mirrors SafeOpFlags::wrapper_names.size().
func WrapperNamesCount() int { return WrapperNamesCountSess(nil) }

// WrapperNamesCountSess returns wrapper count on an explicit session bag.
func WrapperNamesCountSess(s *Session) int { return len(sessOrAmbient(s).WrapperNames) }

// OutputWrapperH mirrors DefaultProgramGenerator identify_wrappers wrapper.h body.
// DefaultProgramGenerator.cpp:73–77 — #define N_WRAP <count>.
func OutputWrapperH() string {
	return OutputWrapperHSess(nil)
}

// OutputWrapperHSess is OutputWrapperH reading wrapper names on bag s.
func OutputWrapperHSess(s *Session) string {
	return "#define N_WRAP " + itoa(WrapperNamesCountSess(s)) + "\n"
}

// SafeOpKind mirrors SafeOpKind for make_random_binary / make_random_unary.
type SafeOpKind int

const (
	// SafeOpUnary is sOpUnary.
	SafeOpUnary SafeOpKind = iota
	// SafeOpBinary is sOpBinary.
	SafeOpBinary
	// SafeOpAssign is sOpAssign (compound assign).
	SafeOpAssign
)

// MaxSafeOpKind is MAX_SAFE_OP_KIND (one past last kind).
const MaxSafeOpKind = int(SafeOpAssign) + 1

// MakeDummyFlags mirrors SafeOpFlags::make_dummy_flags.
// SafeOpFlags.cpp:61–63 — (false, false, false, sInt8).
func MakeDummyFlags() *SafeOpFlags {
	return &SafeOpFlags{
		Op1Signed: false,
		Op2Signed: false,
		IsFunc:    false,
		Size:      SafeInt8,
	}
}

// Op1Sign mirrors SafeOpFlags::get_op1_sign.
func (f *SafeOpFlags) Op1Sign() bool {
	return f.Op1SignSess(nil)
}

func (f *SafeOpFlags) Op1SignSess(s *Session) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.Op1Signed
}

// Op2Sign mirrors SafeOpFlags::get_op2_sign.
func (f *SafeOpFlags) Op2Sign() bool {
	return f.Op2SignSess(nil)
}

func (f *SafeOpFlags) Op2SignSess(s *Session) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.Op2Signed
}

// OpSize mirrors SafeOpFlags::get_op_size.
func (f *SafeOpFlags) OpSize() SafeOpSize {
	return f.OpSizeSess(nil)
}

func (f *SafeOpFlags) OpSizeSess(s *Session) SafeOpSize {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return SafeInt8
	}
	return f.Size
}

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
	return MakeRandomUnarySess(nil, r, opts, probs, rvType, op1Type, uop)
}

func MakeRandomUnarySess(s *Session, r *Rng, opts Options, probs *Probabilities, rvType, op1Type *Type, uop UnaryOp) *SafeOpFlags {
	// SafeOpFlags.cpp:139–167 — always uses rnd_* sticky; no soft invent fixed flags
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	f := &SafeOpFlags{IsFunc: true}
	rvFloat := ReturnFloatTypeUnarySess(s, opts, rvType, op1Type, uop)
	// residual ERROR sticky — no invent soft-flags past ReturnFloatTypeUnary residual
	if sessHasError(s) {
		return nil
	}
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
		sz, ok := pickSafeOpSizeSess(s, r, probs)
		// residual ERROR sticky — no invent soft-flags past pickSafeOpSize residual
		if sessHasError(s) {
			return nil
		}
		if !ok {
			return nil
		}
		f.Size = sz
	}
	return f
}

// MakeRandomBinaryKind mirrors SafeOpFlags::make_random_binary full signature.
// SafeOpFlags.cpp:169–215.}

func MakeRandomBinaryKind(
	r *Rng,
	opts Options,
	probs *Probabilities,
	rvType, op1Type, op2Type *Type,
	opKind SafeOpKind,
	bop BinaryOp,
) *SafeOpFlags {
	return MakeRandomBinaryKindSess(nil, r, opts, probs, rvType, op1Type, op2Type, opKind, bop)
}

func MakeRandomBinaryKindSess(s *Session, 
	r *Rng,
	opts Options,
	probs *Probabilities,
	rvType, op1Type, op2Type *Type,
	opKind SafeOpKind,
	bop BinaryOp,
) *SafeOpFlags {
	// SafeOpFlags.cpp:174–175 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtSafeOpFlags, op_kind, nullptr)
	if DepthGuardByTypeFlagSess(s, opts, DtSafeOpFlags, int(opKind)) == BadDepth {
		return nil
	}
	// SafeOpFlags.cpp:176+ — always uses rnd_* sticky; no soft invent fixed flags
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	f := &SafeOpFlags{IsFunc: true} // ISSUE upstream: always true
	rvFloat := ReturnFloatTypeBinarySess(s, opts, rvType, op1Type, op2Type, bop)
	// residual ERROR sticky — no invent soft-flags past ReturnFloatTypeBinary residual
	if sessHasError(s) {
		return nil
	}

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
		sz, ok := pickSafeOpSizeSess(s, r, probs)
		// residual ERROR sticky — no invent soft-flags past pickSafeOpSize residual
		if sessHasError(s) {
			return nil
		}
		if !ok {
			return nil
		}
		f.Size = sz
	}
	return f
}

// pickSafeOpSize mirrors rnd_upto(MAX_SAFE_OP_SIZE-1, SAFE_OPS_SIZE_PROB_FILTER).
// SafeOpFlags.cpp:164 / 212 — filter from Probabilities pSafeOpsSizeProb.
// No invent opts-only weight table when probs missing.}

func pickSafeOpSize(r *Rng, probs *Probabilities) (SafeOpSize, bool) {
	return pickSafeOpSizeSess(nil, r, probs)
}

func pickSafeOpSizeSess(s *Session, r *Rng, probs *Probabilities) (SafeOpSize, bool) {
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0, false
	}
	if probs == nil {
		probs = sessProbs(s)
	}
	if probs == nil {
		// Probabilities singleton always live in C++; sticky fail closed
		sessNoteError(s, ErrGeneric)
		return 0, false
	}
	v := r.RndUptoFilterSess(s, uint32(MaxSafeOpSizeNonFloat), probs.SafeOpsSizeFilter())
	if sessHasError(s) {
		return 0, false
	}
	sz := SafeOpSize(v)
	if int(sz) < 0 || int(sz) >= MaxSafeOpSizeNonFloat || probs.SafeOpsSizeWeight(int(sz)) == 0 {
		return 0, false
	}
	return sz, true
}

// OutputFuncOrMacro mirrors SafeOpFlags::OutputFuncOrMacro.
// SafeOpFlags.cpp:245–247 — "func_" or "macro_".}

func (f *SafeOpFlags) OutputFuncOrMacro() string {
	return f.OutputFuncOrMacroSess(nil)
}

func (f *SafeOpFlags) OutputFuncOrMacroSess(s *Session) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if f.IsFunc {
		return "func_"
	}
	return "macro_"
}

// OutputSign mirrors SafeOpFlags::OutputSign.
// SafeOpFlags.cpp:249–251 — "_s" or "_u".
func (f *SafeOpFlags) OutputSign(signed bool) string {
	return f.OutputSignSess(nil, signed)
}

func (f *SafeOpFlags) OutputSignSess(s *Session, signed bool) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if signed {
		return "_s"
	}
	return "_u"
}

// OutputOp1 mirrors SafeOpFlags::OutputOp1 → OutputSign(op1_).
// SafeOpFlags.cpp:253.
func (f *SafeOpFlags) OutputOp1() string {
	return f.OutputOp1Sess(nil)
}

func (f *SafeOpFlags) OutputOp1Sess(s *Session) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return f.OutputSignSess(s, f.Op1Signed)
}

// OutputOp2 mirrors SafeOpFlags::OutputOp2 → OutputSign(op2_).
// SafeOpFlags.cpp:255.
func (f *SafeOpFlags) OutputOp2() string {
	return f.OutputOp2Sess(nil)
}

func (f *SafeOpFlags) OutputOp2Sess(s *Session) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return f.OutputSignSess(s, f.Op2Signed)
}

// OutputSize mirrors SafeOpFlags::OutputSize.
// SafeOpFlags.cpp:219–242 — optional leading "u" from !op1_, then type token.
func (f *SafeOpFlags) OutputSize() string {
	return f.OutputSizeSess(nil)
}

func (f *SafeOpFlags) OutputSizeSess(s *Session) string {
	return f.SizeTokenSess(s)
}

// SizeToken mirrors OutputSize (optional leading 'u' for unsigned op1).
// SafeOpFlags.cpp:219–242 — assert invalid size; method is const on live flags.
func (f *SafeOpFlags) SizeToken() string {
	return f.SizeTokenSess(nil)
}

func (f *SafeOpFlags) SizeTokenSess(s *Session) string {
	if f == nil {
		// sticky no soft invent int32_t for nil flags
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// SafeOpFlags.cpp:236–238 — float has no u prefix path
	if f.Size == SafeFloat {
		return "float"
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
	default:
		// SafeOpFlags.cpp:239 — assert(!"invalid size!"); sticky no soft invent int32_t
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return b.String()
}

// FlagsToType mirrors SafeOpFlags::flags_to_type.
// SafeOpFlags.cpp:65–98.
func FlagsToType(signed bool, size SafeOpSize) *Type {
	return FlagsToTypeSess(nil, signed, size)
}

func FlagsToTypeSess(s *Session, signed bool, size SafeOpSize) *Type {
	// SafeOpFlags.cpp:65–98 — default: assert(0); return eInt is dead after assert
	if signed {
		switch size {
		case SafeInt8:
			return GetSimpleTypeSess(s, EChar)
		case SafeInt16:
			return GetSimpleTypeSess(s, EShort)
		case SafeInt32:
			return GetSimpleTypeSess(s, EInt)
		case SafeInt64:
			return GetSimpleTypeSess(s, ELongLong)
		case SafeFloat:
			return GetSimpleTypeSess(s, EFloat)
		default:
			// assert(0) path sticky — no soft invent GetIntType for unknown size
			sessNoteError(s, ErrGeneric)
			return nil
		}
	}
	switch size {
	case SafeInt8:
		return GetSimpleTypeSess(s, EUChar)
	case SafeInt16:
		return GetSimpleTypeSess(s, EUShort)
	case SafeInt32:
		return GetSimpleTypeSess(s, EUInt)
	case SafeInt64:
		return GetSimpleTypeSess(s, EULongLong)
	default:
		// assert(0) path sticky — no soft invent EUInt for unknown size
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// LHSType mirrors SafeOpFlags::get_lhs_type.
// SafeOpFlags.cpp:98–102 — flags_to_type(op1_, op_size_); nil flags → nil (no invent).
func (f *SafeOpFlags) LHSType() *Type {
	return f.LHSTypeSess(nil)
}

func (f *SafeOpFlags) LHSTypeSess(s *Session) *Type {
	// SafeOpFlags methods are const on live flags; sticky no invent type for nil
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return FlagsToTypeSess(s, f.Op1Signed, f.Size)
}

// RHSType mirrors SafeOpFlags::get_rhs_type.
// SafeOpFlags.cpp:104–108 — flags_to_type(op2_, op_size_); nil flags → nil.
func (f *SafeOpFlags) RHSType() *Type {
	return f.RHSTypeSess(nil)
}

func (f *SafeOpFlags) RHSTypeSess(s *Session) *Type {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return FlagsToTypeSess(s, f.Op2Signed, f.Size)
}

// BinaryFuncName mirrors SafeOpFlags::to_string(eBinaryOps) for safe arithmetic/shifts.
// SafeOpFlags.cpp:285–320 — float uses safe_*_func_float_f_f.
func (f *SafeOpFlags) BinaryFuncName(op string) string {
	return f.BinaryFuncNameSess(nil, op)
}

func (f *SafeOpFlags) BinaryFuncNameSess(s *Session, op string) string {
	// live flags required; sticky no invent safe_* name for nil
	if f == nil {
		sessNoteError(s, ErrGeneric)
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
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// SafeOpFlags.cpp:314–319 — OutputFuncOrMacro + OutputSize + OutputOp1 + Op1/Op2
	sz := f.OutputSizeSess(s)
	if sz == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// safe_add_func_int32_t_s_s  /  safe_lshift_func_int32_t_s_u
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(f.OutputFuncOrMacroSess(s))
	b.WriteString(sz)
	b.WriteString(f.OutputOp1Sess(s))
	// shifts use Op2 sign; other ops repeat Op1 (SafeOpFlags.cpp:318)
	if shift {
		b.WriteString(f.OutputOp2Sess(s))
	} else {
		b.WriteString(f.OutputOp1Sess(s))
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
	return f.UnaryMinusFuncNameSess(nil)
}

func (f *SafeOpFlags) UnaryMinusFuncNameSess(s *Session) string {
	// live flags required; sticky no invent int32 name for nil
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// SafeOpFlags.cpp:325 — assert(op_size_ != sFloat); non-sticky empty
	// (emit falls through to cast form — soft re-pick / alternate emit path)
	if f.Size == SafeFloat {
		return ""
	}
	// SafeOpFlags.cpp:334–338 — OutputFuncOrMacro + OutputSize + OutputOp1
	sz := f.OutputSizeSess(s)
	if sz == "" {
		// invalid size assert path sticky
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	return "safe_unary_minus_" + f.OutputFuncOrMacroSess(s) + sz + f.OutputOp1Sess(s)
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
