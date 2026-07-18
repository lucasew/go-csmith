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

// MakeRandomBinary mirrors SafeOpFlags::make_random_binary for integer ops.
// SafeOpFlags.cpp:169–215 (float path omitted unless EnableFloat).
func MakeRandomBinary(r *Rng, opts Options, probs *Probabilities, typ *Type) *SafeOpFlags {
	if r == nil {
		return &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32}
	}
	f := &SafeOpFlags{IsFunc: true}
	// SafeOpsSignedProb — default 50
	sigProb := uint32(50)
	if probs != nil {
		if p := probs.Single(PSafeOpsSignedProb); p >= 0 {
			sigProb = uint32(p)
		}
	}
	f.Op1Signed = r.RndFlipcoin(sigProb)
	f.Op2Signed = r.RndFlipcoin(sigProb)
	// size via equal group filter (int8 only if Int8&&UInt8, int64 if AllowInt64)
	f.Size = pickSafeOpSize(r, opts)
	_ = typ
	return f
}

// pickSafeOpSize mirrors rnd_upto(MAX_SAFE_OP_SIZE-1, SAFE_OPS_SIZE_PROB_FILTER).
func pickSafeOpSize(r *Rng, opts Options) SafeOpSize {
	// weights: Int8 if int8&uint8, Int16, Int32, Int64 if allow_int64
	w := make([]int, MaxSafeOpSizeNonFloat)
	if opts.Int8 && opts.UInt8 {
		w[SafeInt8] = 1
	}
	w[SafeInt16] = 1
	w[SafeInt32] = 1
	if opts.AllowInt64() {
		w[SafeInt64] = 1
	}
	filt := filterFunc(func(v uint32) bool {
		i := int(v)
		return i < 0 || i >= len(w) || w[i] == 0
	})
	v := r.RndUptoFilter(uint32(MaxSafeOpSizeNonFloat), filt)
	return SafeOpSize(v)
}

// SizeToken mirrors OutputSize (without leading 'u' for unsigned).
func (f *SafeOpFlags) SizeToken() string {
	if f == nil {
		return "int32_t"
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
		b.WriteString("int32_t")
	}
	return b.String()
}

// BinaryFuncName mirrors SafeOpFlags::to_string(eBinaryOps) for + - * / %.
// SafeOpFlags.cpp:285–320.
func (f *SafeOpFlags) BinaryFuncName(op string) string {
	if f == nil {
		return ""
	}
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
	case "%":
		prefix = "safe_mod_"
	default:
		return ""
	}
	// safe_add_func_int32_t_s_s
	var b strings.Builder
	b.WriteString(prefix)
	if f.IsFunc {
		b.WriteString("func_")
	} else {
		b.WriteString("macro_")
	}
	b.WriteString(f.SizeToken())
	if f.Op1Signed {
		b.WriteString("_s")
	} else {
		b.WriteString("_u")
	}
	// non-shift: second sign == first
	if f.Op1Signed {
		b.WriteString("_s")
	} else {
		b.WriteString("_u")
	}
	return b.String()
}

// UnaryMinusFuncName mirrors to_string(eMinus).
func (f *SafeOpFlags) UnaryMinusFuncName() string {
	if f == nil {
		return "safe_unary_minus_func_int32_t_s"
	}
	var b strings.Builder
	b.WriteString("safe_unary_minus_")
	if f.IsFunc {
		b.WriteString("func_")
	} else {
		b.WriteString("macro_")
	}
	b.WriteString(f.SizeToken())
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
