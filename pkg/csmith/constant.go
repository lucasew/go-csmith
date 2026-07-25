// Upstream: Constant.h / Constant.cpp (make_random, GenerateRandomConstant, hex helpers).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strconv"
	"strings"
)

// Constant mirrors Constant : Expression for literal values (value string only for now).
type Constant struct {
	Type  *Type
	Value string
}

// MakeRandom mirrors Constant::make_random.
// Constant.cpp:423–426 → GenerateRandomConstant; ERROR_GUARD(nullptr).
// probs is the session Probabilities (C++ singleton); nil allowed for simple/pointer
// only — aggregate constants need live probs (no invent NewProbabilities(opts)).

func MakeRandomSess(s *Session, typ *Type, opts Options, probs *Probabilities, r *Rng) *Constant {
	// Type* always live at make_random (Expression choose_random / callers);
	// sticky no invent Constant{Type:nil, Value:"0"} success shell past hole
	if typ == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Constant.cpp:312 — assert(st != eVoid) before simple emit sticky
	// (no invent soft re-pick past void as empty success / "/* void */")
	if typ.IsSimpleSess(s) {
		// residual ERROR sticky — no invent soft-continue void check past IsSimple residual
		if sessHasError(s) {
			return nil
		}
		st := typ.SimpleSess(s)
		// residual ERROR sticky — no invent soft-continue void check past Simple residual
		if sessHasError(s) {
			return nil
		}
		if st == EVoid {
			sessNoteError(s, ErrGeneric)
			return nil
		}
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-continue const past IsSimple residual false
		return nil
	}
	v := generateRandomConstantSess(s, typ, opts, probs, r)
	// Constant.cpp:425 — ERROR_GUARD(nullptr)
	if sessHasError(s) {
		return nil
	}
	// assert / nil-probs / field fail paths yield ""; sticky no invent Constant{"", …}
	// success: simple non-void non-empty, pointer "0", aggregate at least "{}"
	if v == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return nil
	}
	return &Constant{Type: typ, Value: v}
}

// MakeRandomUpto mirrors Constant::make_random_upto — rnd_upto(limit) as eUInt decimal.
// Constant.cpp:429–433 — always has RNG; sticky no soft invent NewRng(0).}

func MakeRandomUptoSess(s *Session, limit uint32, r *Rng) *Constant {
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	n := r.RndUptoSess(s, limit)
	// Constant.cpp:432 — ERROR_GUARD(nullptr)
	if sessHasError(s) {
		return nil
	}
	return &Constant{Type: GetSimpleTypeSess(s, EUInt), Value: strconv.FormatUint(uint64(n), 10)}
}

// MakeInt mirrors Constant::make_int.
// Constant.cpp:449–481 — optional mark_mutable_const wraps "(v)".

// MakeIntSess is MakeInt with Options from an explicit session bag.
func MakeIntSess(s *Session, v int) *Constant {
	return MakeIntOptsSess(s, v, sessOpts(s))
}

// MakeIntOpts is make_int with explicit Options (library tests).

// MakeIntOptsSess is MakeIntOpts with sticky errors on bag s.
func MakeIntOptsSess(s *Session, v int, opts Options) *Constant {
	str := strconv.Itoa(v)
	// Constant.cpp:475–478 — mark_mutable_const → "(v)"
	if opts.MarkMutableConst {
		str = "(" + str + ")"
	}
	return &Constant{Type: GetSimpleTypeSess(s, EInt), Value: str}
}

// MakeRandomNonzero mirrors Constant::make_random_nonzero.
// Constant.cpp:436–446 — regenerate until str2int(value) != 0.
// Incomplete type/rng sticky nil; bounded retries fail closed (no invent hang).

func MakeRandomNonzeroSess(s *Session, typ *Type, opts Options, probs *Probabilities, r *Rng) *Constant {
	if typ == nil || r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Cap retries — C++ loops unbounded; library fail closed if stuck on zero.
	for tries := 0; tries < 64; tries++ {
		c := MakeRandomSess(s, typ, opts, probs, r)
		if c == nil || sessHasError(s) {
			return nil
		}
		// Constant.cpp:439 — StringUtils::str2int(v) == 0 → retry
		if c.NotEqualsZeroSess(s) {
			return c
		}
		// residual ERROR sticky — no invent soft-retry past NotEqualsZero residual
		if sessHasError(s) {
			return nil
		}
	}
	sessNoteError(s, ErrGeneric)
	return nil
}

// Clone mirrors Constant::clone → new Constant(*this).
// Constant.cpp:82.
// Incomplete Constant sticky nil (no invent zero Constant shell).}

func (c *Constant) CloneSess(s *Session) *Constant {
	if c == nil || c.Type == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return &Constant{Type: c.Type, Value: c.Value}
}

// GetType mirrors Constant::get_type.
// Constant.cpp:527 — return *type.

func (c *Constant) GetTypeSess(s *Session) *Type {
	if c == nil || c.Type == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return c.Type
}

// GetValue mirrors Constant value string accessor.

func (c *Constant) GetValueSess(s *Session) string {
	if c == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return c.Value
}

// GetComplexity mirrors Constant::get_complexity — always 1.
// Constant.h:88.

func (c *Constant) GetComplexitySess(s *Session) int {
	if c == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return 1
}

// GetReferencedPtrs mirrors Expression::get_referenced_ptrs on Constant — none.
// Constant has no pointers.

func (c *Constant) GetReferencedPtrsSess(s *Session) []*Variable {
	if c == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return nil
}

// CompatibleWithVar mirrors Constant::compatible(Variable*).
// Constant.cpp:488–493 — expand_struct → true; else false (not field soft-match).

func (c *Constant) CompatibleWithVarSess(s *Session, v *Variable, expandStruct bool) bool {
	if c == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// C++ assert(v)
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if expandStruct {
		return true
	}
	return false
}

// CompatibleWithExpr mirrors Constant::compatible(Expression*).
// Constant.cpp:496–498 — always false (assert exp non-null).

func (c *Constant) CompatibleWithExprSess(s *Session, exp *Expression) bool {
	if c == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if exp == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return false
}

// Output mirrors Constant::Output.
// Constant.cpp:532–553 — paren negatives; pointer-0 → (void*)0 / nullptr; else cast+value.
// Incomplete Constant sticky "" (no invent bare "0" for Type-nil shell).

// OutputSess is Output with Options/sticky from an explicit session bag.
func (c *Constant) OutputSess(s *Session) string {
	return c.OutputOptsSess(s, sessOpts(s))
}

// OutputOpts is Output with explicit session Options (lang_cpp nullptr).

// OutputOptsSess is OutputOpts with sticky errors on bag s.
func (c *Constant) OutputOptsSess(s *Session, opts Options) string {
	if c == nil || c.Type == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// empty value sticky (no invent bare token)
	if c.Value == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// negative numbers in parentheses
	if c.Value[0] == '-' {
		return "(" + c.Value + ")"
	}
	// pointer zero
	if c.Type.PtrTypeSess(s) != nil {
		// residual ERROR sticky — no invent soft-null past PtrType residual
		if sessHasError(s) {
			return ""
		}
		if c.EqualsSess(s, 0) {
			if opts.LangCPP {
				return "nullptr"
			}
			return "(void*)" + c.Value
		}
	} else if sessHasError(s) {
		return ""
	}
	return c.Value
}

// Equals mirrors Constant::equals(int).
// Constant.cpp:509–510 — StringUtils::str2int(value) == num.
// Incomplete Constant shell sticky false (no invent not-equal fold / soft re-pick).
//
// Small-path constants use "0L"/"0UL" suffixes (Constant.cpp:357–361). C++ str2int
// stream-extracts and stops at the suffix; Atoi would miss equals(0) and skip the
// div/mod binary re-pick (seed-2 e15477 U18 vs U120).

func (c *Constant) EqualsSess(s *Session, num int) bool {
	// Constant always live with Type* + non-empty Value for fold; sticky incomplete
	// (no invent not-equal / fold success past Type-nil or empty-value shell)
	if c == nil || c.Type == nil || c.Value == "" {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return Str2Int(c.Value) == num
}

// NotEqualsZero mirrors Constant/Expression not_equals(0).
// Incomplete Constant sticky (Equals sets sticky false for nil).
func (c *Constant) NotEqualsZeroSess(s *Session) bool {
	return !c.EqualsSess(s, 0)
}

// NotEquals mirrors Constant::not_equals(int).
// Constant.cpp:505–507.
// Incomplete Constant sticky false (no invent equals fold / soft re-pick).

func (c *Constant) NotEqualsSess(s *Session, num int) bool {
	// nil / Type-nil / empty Value → Equals stickies false; !false invents "not equals"
	// — fail closed false sticky before complement
	if c == nil || c.Type == nil || c.Value == "" {
		sessNoteError(s, ErrGeneric)
		return false
	}
	eq := c.EqualsSess(s, num)
	// residual ERROR sticky — no invent not-equal true past Equals residual hole
	if sessHasError(s) {
		return false
	}
	return !eq
}

// LessThan mirrors Constant::less_than(int).
// Constant.cpp:501–503 — str2int(value) < num.
// Incomplete Constant sticky false (no invent compare fold / soft re-pick).

func (c *Constant) LessThanSess(s *Session, num int) bool {
	// Constant always live with Type* + non-empty Value for fold; sticky incomplete
	// (no invent not-less soft-skip past Type-nil or empty-value shell)
	if c == nil || c.Type == nil || c.Value == "" {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return Str2Int(c.Value) < num
}

// GetField mirrors Constant::get_field.
// Constant.cpp:513–522 — split union/struct brace init by "{}," pick field fid.
// Constant always live with non-empty Value; sticky empty (no invent empty field
// soft-skip past empty-value shell). Negative fid is complete empty (not incomplete).

func (c *Constant) GetFieldSess(s *Session, fid int) string {
	// Type* + Value always live for aggregate field split; sticky empty
	// (no invent empty field soft-skip past Type-nil / empty-value shell)
	if c == nil || c.Type == nil || c.Value == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if fid < 0 {
		return ""
	}
	// StringUtils::split_string(value, fields, "{},")
	fields := splitConstFields(c.Value)
	if fid < len(fields) {
		return fields[fid]
	}
	return ""
}

// splitConstFields mirrors StringUtils::split_string with separators { } ,
func splitConstFields(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		t := strings.TrimSpace(cur.String())
		cur.Reset()
		if t != "" {
			out = append(out, t)
		}
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{', '}', ',':
			flush()
		default:
			cur.WriteByte(s[i])
		}
	}
	flush()
	return out
}

// generateRandomConstant mirrors GenerateRandomConstant (simple + pointer only).
// Constant.cpp:296–...

func generateRandomConstantSess(s *Session, typ *Type, opts Options, probs *Probabilities, r *Rng) string {
	// Constant.cpp:302–314 — type dispatch before pure_rnd (no invent require RNG first)
	// Type* always live at GenerateRandomConstant in generation; sticky no invent "0"
	// success string that MakeRandom would wrap as Type-nil Constant shell
	if typ == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if typ.IsStructSess(s) {
		// residual ERROR sticky — no invent soft-continue const past IsStruct residual hole
		if sessHasError(s) {
			return ""
		}
		// Probabilities singleton always live in C++; sticky no invent NewProbabilities(opts)
		if r == nil || probs == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if c := MakeStructConstantSess(s, r, opts, probs, typ); c != nil {
			// residual ERROR sticky — no invent soft-empty value past MakeStruct residual
			if sessHasError(s) {
				return ""
			}
			return c.Value
		}
		// incomplete struct const IR sticky (MakeStruct may already SetError)
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// residual ERROR sticky — no invent soft-continue union path past IsStruct residual false
	if sessHasError(s) {
		return ""
	}
	if typ.IsUnionSess(s) {
		// residual ERROR sticky — no invent soft-continue const past IsUnion residual hole
		if sessHasError(s) {
			return ""
		}
		if r == nil || probs == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if c := MakeUnionConstantSess(s, r, opts, probs, typ); c != nil {
			// residual ERROR sticky — no invent soft-empty value past MakeUnion residual
			if sessHasError(s) {
				return ""
			}
			return c.Value
		}
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// residual ERROR sticky — no invent soft-continue pointer path past IsUnion residual false
	if sessHasError(s) {
		return ""
	}
	// Constant.cpp:308–310 — pointer constant is always "0" (no RNG draw)
	if typ.PtrTypeSess(s) != nil {
		// residual ERROR sticky — no invent "0" past PtrType residual hole
		if sessHasError(s) {
			return ""
		}
		return "0"
	}
	// residual ERROR sticky — no invent soft-continue simple path past PtrType residual nil
	if sessHasError(s) {
		return ""
	}
	// Constant.cpp:411 — assert(0) for types other than simple/pointer/struct/union sticky
	if !typ.IsSimpleSess(s) {
		// residual ERROR sticky — no invent soft-continue past IsSimple residual hole
		if sessHasError(s) {
			return ""
		}
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// residual ERROR sticky — no invent soft-continue simple past IsSimple residual true
	if sessHasError(s) {
		return ""
	}
	// Constant.cpp:312 — assert(st != eVoid); no soft invent comment literal sticky
	if typ.SimpleSess(s) == EVoid {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// simple non-void needs process RNG sticky (no invent NewRng for nil)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	st := typ.SimpleSess(s)

	var v string
	// pure_rnd_flipcoin(50) — in random mode == rnd_flipcoin(50)
	if r.RndFlipcoinSess(s, 50) {
		// small integer / small float path (Constant.cpp:318–361)
		var num int
		if r.RndFlipcoinSess(s, 50) {
			num = int(r.RndUptoSess(s, 3)) - 1
		} else {
			num = int(r.RndUptoSess(s, 20)) - 10
		}
		// Constant.cpp:346–348 — eFloat → GenerateSmallRandomFloatHexConstant(num)
		if st == EFloat {
			v = generateSmallRandomFloatHexConstantSess(s, num, r)
		} else {
			v = formatSmallConstantSess(s, st, num, opts)
		}
	} else {
		// hex / wide path by simple type
		switch st {
		case EChar, EUChar:
			v = generateRandomCharConstant(s, opts, r)
		case EInt, EUInt:
			v = generateRandomIntConstant(s, opts, r)
		case EShort, EUShort:
			v = generateRandomShortConstant(s, opts, r)
		case ELong, EULong:
			v = generateRandomLongConstant(s, opts, r)
		case ELongLong, EULongLong:
			v = generateRandomLongLongConstant(s, opts, r)
		case EInt128, EUInt128:
			v = generateRandomInt128Constant(s, opts, r)
		case EFloat:
			v = generateRandomFloatHexConstantSess(s, r)
		default:
			// Constant.cpp:407 — assert(0 && "Unsupported type!") sticky; no soft invent int
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	// Constant.cpp:413–415 — simple + mark_mutable_const → "(" + v + ")"
	// no soft invent ignore of MarkMutableConst (silent option no-op)
	if v != "" && opts.MarkMutableConst {
		return "(" + v + ")"
	}
	return v
}

// generateSmallRandomFloatHexConstant mirrors GenerateSmallRandomFloatHexConstant.
// Constant.cpp:207–223 — RandomHexDigits(1) then pure_rnd_flipcoin(50) for p±1.

func generateSmallRandomFloatHexConstantSess(s *Session, num int, r *Rng) string {
	if r == nil {
		// C++ always has RNG; sticky no invent fixed float literal
		sessNoteError(s, ErrGeneric)
		return ""
	}
	sign := ""
	abs := num
	if abs < 0 {
		sign = "-"
		abs = -abs
	}
	// Constant.cpp:212–220 — digits first, then flipcoin for ±1 (RNG order matters).
	frac := r.RandomHexDigitsSess(s, 1)
	pm := "+1"
	// pure_rnd_flipcoin(50) — random mode == rnd_flipcoin(50)
	if !r.RndFlipcoinSess(s, 50) {
		pm = "-1"
	}
	return sign + "0x" + strconv.Itoa(abs) + "." + frac + "p" + pm
}

func formatSmallConstantSess(s *Session, st ESimpleType, num int, opts Options) string {
	// Constant.cpp:329–361 cast + L/UL suffix (non-float only; float uses
	// generateSmallRandomFloatHexConstant with live RNG)
	if st == EFloat {
		// broken call path sticky — float must not invent digit/sign from num alone
		// or soft-empty success past wrong dispatch (use generateSmallRandomFloatHexConstant)
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var body string
	switch st {
	case EUChar:
		body = strconv.FormatUint(uint64(uint8(int8(num))), 10)
	case EUShort:
		body = strconv.FormatUint(uint64(uint16(int16(num))), 10)
	case EUInt:
		body = strconv.FormatUint(uint64(uint32(int32(num))), 10)
	case EULong:
		// (unsigned long)num — use uint64 cast of int64(num) for two's-complement wrap
		body = strconv.FormatUint(uint64(int64(num)), 10)
	case EULongLong, EUInt128:
		body = strconv.FormatUint(uint64(int64(num)), 10)
	default:
		body = strconv.Itoa(num)
	}
	signed := true
	switch st {
	case EUChar, EUInt, EUShort, EULong, EULongLong, EUInt128:
		signed = false
	}
	if opts.CComp || !opts.LongLong {
		if signed {
			return body
		}
		return body + "U"
	}
	if signed {
		return body + "L"
	}
	return body + "UL"
}

// HexToBinary mirrors Constant.cpp HexToBinary.
// Constant.cpp:85–97 — each hex digit → 4-bit nibble string.

func HexToBinarySess(s *Session, val string) string {
	const toBin = "0000000100100011010001010110011110001001101010111100110111101111"
	var b strings.Builder
	b.Grow(len(val) * 4)
	for i := 0; i < len(val); i++ {
		c := val[i]
		var idx int
		switch {
		case c >= '0' && c <= '9':
			idx = int(c - '0')
		case c >= 'A' && c <= 'F':
			idx = int(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			idx = int(c - 'a' + 10)
		default:
			// broken hex IR — sticky fail closed empty (C++ OOB ToBinary)
			sessNoteError(s, ErrGeneric)
			return ""
		}
		b.WriteString(toBin[idx*4 : idx*4+4])
	}
	return b.String()
}

// binaryConstProb mirrors BinaryConstProb() (Probabilities singleton; default 3).

// binaryConstProbSess is binaryConstProb on an explicit session bag.
func binaryConstProbSess(s *Session) uint32 {
	if p := sessProbs(s); p != nil {
		if v := p.SingleSess(s, PBinaryConstProb); v >= 0 {
			return uint32(v)
		}
	}
	return 3
}

// maybeBinaryConstant is binary_constant && rnd_flipcoin(BinaryConstProb).
// On hit returns "0b"+HexToBinarySess(s, ...)+suffix; ok=true means binary branch taken
// (including empty fail-closed). No invent hex fallback when binary was selected.
// When BinaryConstant is on, RNG always live; sticky (no invent soft skip without draw).
// BinaryConstant off is complete no-op.

func maybeBinaryConstantSess(s *Session, opts Options, r *Rng, nHex int, suffix string) (string, bool) {
	if !opts.BinaryConstant {
		return "", false
	}
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return "", false
	}
	if !r.RndFlipcoinSess(s, binaryConstProbSess(s)) {
		return "", false
	}
	hex := r.RandomHexDigitsSess(s, nHex)
	bin := HexToBinarySess(s, hex)
	if bin == "" {
		return "", true
	}
	return "0b" + bin + suffix, true
}

// generateRandomCharConstant — Constant.cpp:100–108
func generateRandomCharConstant(s *Session, opts Options, r *Rng) string {
	if s, ok := maybeBinaryConstantSess(s, opts, r, 2, ""); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 2)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomIntConstant(s *Session, opts Options, r *Rng) string {
	// Constant.cpp:112–122
	if s, ok := maybeBinaryConstantSess(s, opts, r, 8, ""); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 8)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomShortConstant(s *Session, opts Options, r *Rng) string {
	// Constant.cpp:136–146
	if s, ok := maybeBinaryConstantSess(s, opts, r, 4, ""); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 4)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongConstant(s *Session, opts Options, r *Rng) string {
	// Constant.cpp:150–160 — !longlong → no L; else L
	if s, ok := maybeBinaryConstantSess(s, opts, r, 8, ""); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 8)
	if !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongLongConstant(s *Session, opts Options, r *Rng) string {
	// Constant.cpp:164–171 — binary path includes "LL"
	if s, ok := maybeBinaryConstantSess(s, opts, r, 16, "LL"); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 16)
	return "0x" + hex + "LL"
}

func generateRandomInt128Constant(s *Session, opts Options, r *Rng) string {
	// Constant.cpp:126–133
	if s, ok := maybeBinaryConstantSess(s, opts, r, 16, ""); ok {
		return s
	}
	hex := r.RandomHexDigitsSess(s, 16)
	return "0x" + hex
}

// generateRandomFloatHexConstant mirrors GenerateRandomFloatHexConstant.
// Constant.cpp:187–199 — pure_rnd_upto(100); RandomHexDigits(1)+RandomHexDigits(6);
// then pure_rnd_flipcoin(50) for +/− exp (RNG order matches C++ stream).

func generateRandomFloatHexConstantSess(s *Session, r *Rng) string {
	if r == nil {
		// C++ always has RNG; sticky no invent fixed literal
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Constant.cpp:188–198 — exp, then hex digits, then sign flipcoin.
	exp := int(r.RndUptoSess(s, 100))
	hi := r.RandomHexDigitsSess(s, 1)
	lo := r.RandomHexDigitsSess(s, 6)
	sign := "+"
	// pure_rnd_flipcoin(50) — random mode == rnd_flipcoin(50)
	if !r.RndFlipcoinSess(s, 50) {
		sign = "-"
	}
	return fmt.Sprintf("0x%s.%sp%s%d", hi, lo, sign, exp)
}
