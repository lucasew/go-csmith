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
func MakeRandom(typ *Type, opts Options, probs *Probabilities, r *Rng) *Constant {
	// Type* always live at make_random (Expression choose_random / callers);
	// sticky no invent Constant{Type:nil, Value:"0"} success shell past hole
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	// Constant.cpp:312 — assert(st != eVoid) before simple emit sticky
	// (no invent soft re-pick past void as empty success / "/* void */")
	if typ.IsSimple() && typ.Simple() == EVoid {
		SetError(ErrGeneric)
		return nil
	}
	v := generateRandomConstant(typ, opts, probs, r)
	// Constant.cpp:425 — ERROR_GUARD(nullptr)
	if HasError() {
		return nil
	}
	// assert / nil-probs / field fail paths yield ""; sticky no invent Constant{"", …}
	// success: simple non-void non-empty, pointer "0", aggregate at least "{}"
	if v == "" {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}
	return &Constant{Type: typ, Value: v}
}

// MakeRandomUpto mirrors Constant::make_random_upto — rnd_upto(limit) as eUInt decimal.
// Constant.cpp:429–433 — always has RNG; sticky no soft invent NewRng(0).
func MakeRandomUpto(limit uint32, r *Rng) *Constant {
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	n := r.RndUpto(limit)
	// Constant.cpp:432 — ERROR_GUARD(nullptr)
	if HasError() {
		return nil
	}
	return &Constant{Type: GetSimpleType(EUInt), Value: strconv.FormatUint(uint64(n), 10)}
}

// MakeInt mirrors Constant::make_int.
func MakeInt(v int) *Constant {
	return &Constant{Type: GetSimpleType(EInt), Value: strconv.Itoa(v)}
}

// Equals mirrors Constant::equals(int).
// Constant.cpp:509–510.
// Incomplete Constant shell sticky false (no invent not-equal fold / soft re-pick).
func (c *Constant) Equals(num int) bool {
	// Constant always live with non-empty Value for fold; sticky incomplete
	// (no invent not-equal soft-skip past empty-value shell / soft re-pick)
	if c == nil || c.Value == "" {
		SetError(ErrGeneric)
		return false
	}
	n, err := strconv.Atoi(c.Value)
	if err != nil {
		// try 0x hex
		if len(c.Value) > 2 && (c.Value[0:2] == "0x" || c.Value[0:2] == "0X") {
			u, e2 := strconv.ParseUint(c.Value[2:], 16, 64)
			return e2 == nil && int(u) == num
		}
		// non-numeric complete value → not equal (not incomplete IR)
		return false
	}
	return n == num
}

// NotEqualsZero mirrors Constant/Expression not_equals(0).
// Incomplete Constant sticky (Equals sets sticky false for nil).
func (c *Constant) NotEqualsZero() bool {
	return !c.Equals(0)
}

// NotEquals mirrors Constant::not_equals(int).
// Constant.cpp:505–507.
// Incomplete Constant sticky false (no invent equals fold / soft re-pick).
func (c *Constant) NotEquals(num int) bool {
	// nil / empty Value → Equals stickies false; !false invents "not equals"
	// — fail closed false sticky before complement
	if c == nil || c.Value == "" {
		SetError(ErrGeneric)
		return false
	}
	return !c.Equals(num)
}

// LessThan mirrors Constant::less_than(int).
// Constant.cpp:501–503 — str2int(value) < num.
// Incomplete Constant sticky false (no invent compare fold / soft re-pick).
func (c *Constant) LessThan(num int) bool {
	// Constant always live with non-empty Value for fold; sticky incomplete
	// (no invent not-less soft-skip past empty-value shell / soft re-pick)
	if c == nil || c.Value == "" {
		SetError(ErrGeneric)
		return false
	}
	n, err := strconv.Atoi(c.Value)
	if err != nil {
		if len(c.Value) > 2 && (c.Value[0:2] == "0x" || c.Value[0:2] == "0X") {
			u, e2 := strconv.ParseUint(c.Value[2:], 16, 64)
			if e2 != nil {
				return false
			}
			return int(u) < num
		}
		// non-numeric complete value → not less (not incomplete IR)
		return false
	}
	return n < num
}

// GetField mirrors Constant::get_field.
// Constant.cpp:513–522 — split union/struct brace init by "{}," pick field fid.
// Constant always live with non-empty Value; sticky empty (no invent empty field
// soft-skip past empty-value shell). Negative fid is complete empty (not incomplete).
func (c *Constant) GetField(fid int) string {
	if c == nil || c.Value == "" {
		SetError(ErrGeneric)
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
func generateRandomConstant(typ *Type, opts Options, probs *Probabilities, r *Rng) string {
	// Constant.cpp:302–314 — type dispatch before pure_rnd (no invent require RNG first)
	// Type* always live at GenerateRandomConstant in generation; sticky no invent "0"
	// success string that MakeRandom would wrap as Type-nil Constant shell
	if typ == nil {
		SetError(ErrGeneric)
		return ""
	}
	if typ.IsStruct() {
		// Probabilities singleton always live in C++; sticky no invent NewProbabilities(opts)
		if r == nil || probs == nil {
			SetError(ErrGeneric)
			return ""
		}
		if c := MakeStructConstant(r, opts, probs, typ); c != nil {
			return c.Value
		}
		// incomplete struct const IR sticky (MakeStruct may already SetError)
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	if typ.IsUnion() {
		if r == nil || probs == nil {
			SetError(ErrGeneric)
			return ""
		}
		if c := MakeUnionConstant(r, opts, probs, typ); c != nil {
			return c.Value
		}
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	// Constant.cpp:308–310 — pointer constant is always "0" (no RNG draw)
	if typ.PtrType() != nil {
		return "0"
	}
	// Constant.cpp:411 — assert(0) for types other than simple/pointer/struct/union sticky
	if !typ.IsSimple() {
		SetError(ErrGeneric)
		return ""
	}
	// Constant.cpp:312 — assert(st != eVoid); no soft invent comment literal sticky
	if typ.Simple() == EVoid {
		SetError(ErrGeneric)
		return ""
	}
	// simple non-void needs process RNG sticky (no invent NewRng for nil)
	if r == nil {
		SetError(ErrGeneric)
		return ""
	}
	st := typ.Simple()

	var v string
	// pure_rnd_flipcoin(50) — in random mode == rnd_flipcoin(50)
	if r.RndFlipcoin(50) {
		// small integer / small float path (Constant.cpp:318–361)
		var num int
		if r.RndFlipcoin(50) {
			num = int(r.RndUpto(3)) - 1
		} else {
			num = int(r.RndUpto(20)) - 10
		}
		// Constant.cpp:346–348 — eFloat → GenerateSmallRandomFloatHexConstant(num)
		if st == EFloat {
			v = generateSmallRandomFloatHexConstant(num, r)
		} else {
			v = formatSmallConstant(st, num, opts)
		}
	} else {
		// hex / wide path by simple type
		switch st {
		case EChar, EUChar:
			v = generateRandomCharConstant(opts, r)
		case EInt, EUInt:
			v = generateRandomIntConstant(opts, r)
		case EShort, EUShort:
			v = generateRandomShortConstant(opts, r)
		case ELong, EULong:
			v = generateRandomLongConstant(opts, r)
		case ELongLong, EULongLong:
			v = generateRandomLongLongConstant(opts, r)
		case EInt128, EUInt128:
			v = generateRandomInt128Constant(opts, r)
		case EFloat:
			v = generateRandomFloatHexConstant(r)
		default:
			// Constant.cpp:407 — assert(0 && "Unsupported type!") sticky; no soft invent int
			SetError(ErrGeneric)
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
// Constant.cpp:207–223 — ±0x{num}.{RandomHexDigits(1)}p±1 via pure_rnd_flipcoin(50).
func generateSmallRandomFloatHexConstant(num int, r *Rng) string {
	if r == nil {
		// C++ always has RNG; sticky no invent fixed float literal
		SetError(ErrGeneric)
		return ""
	}
	sign := ""
	abs := num
	if abs < 0 {
		sign = "-"
		abs = -abs
	}
	pm := "+1"
	// pure_rnd_flipcoin(50) — random mode == rnd_flipcoin(50)
	if !r.RndFlipcoin(50) {
		pm = "-1"
	}
	// Constant.cpp:215 — num << "." << RandomHexDigits(1)
	return sign + "0x" + strconv.Itoa(abs) + "." + r.RandomHexDigits(1) + "p" + pm
}

func formatSmallConstant(st ESimpleType, num int, opts Options) string {
	// Constant.cpp:329–361 cast + L/UL suffix (non-float only; float uses
	// generateSmallRandomFloatHexConstant with live RNG)
	if st == EFloat {
		// broken call path — float must not invent digit/sign from num alone
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
func HexToBinary(val string) string {
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
			SetError(ErrGeneric)
			return ""
		}
		b.WriteString(toBin[idx*4 : idx*4+4])
	}
	return b.String()
}

// binaryConstProb mirrors BinaryConstProb() (Probabilities singleton; default 3).
func binaryConstProb() uint32 {
	if p := ProcessProbabilities(); p != nil {
		if v := p.Single(PBinaryConstProb); v >= 0 {
			return uint32(v)
		}
	}
	return 3
}

// maybeBinaryConstant is binary_constant && rnd_flipcoin(BinaryConstProb).
// On hit returns "0b"+HexToBinary(...)+suffix; ok=true means binary branch taken
// (including empty fail-closed). No invent hex fallback when binary was selected.
// When BinaryConstant is on, RNG always live; sticky (no invent soft skip without draw).
// BinaryConstant off is complete no-op.
func maybeBinaryConstant(opts Options, r *Rng, nHex int, suffix string) (string, bool) {
	if !opts.BinaryConstant {
		return "", false
	}
	if r == nil {
		SetError(ErrGeneric)
		return "", false
	}
	if !r.RndFlipcoin(binaryConstProb()) {
		return "", false
	}
	hex := r.RandomHexDigits(nHex)
	bin := HexToBinary(hex)
	if bin == "" {
		return "", true
	}
	return "0b" + bin + suffix, true
}

// generateRandomCharConstant — Constant.cpp:100–108
func generateRandomCharConstant(opts Options, r *Rng) string {
	if s, ok := maybeBinaryConstant(opts, r, 2, ""); ok {
		return s
	}
	hex := r.RandomHexDigits(2)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomIntConstant(opts Options, r *Rng) string {
	// Constant.cpp:112–122
	if s, ok := maybeBinaryConstant(opts, r, 8, ""); ok {
		return s
	}
	hex := r.RandomHexDigits(8)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomShortConstant(opts Options, r *Rng) string {
	// Constant.cpp:136–146
	if s, ok := maybeBinaryConstant(opts, r, 4, ""); ok {
		return s
	}
	hex := r.RandomHexDigits(4)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongConstant(opts Options, r *Rng) string {
	// Constant.cpp:150–160 — !longlong → no L; else L
	if s, ok := maybeBinaryConstant(opts, r, 8, ""); ok {
		return s
	}
	hex := r.RandomHexDigits(8)
	if !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongLongConstant(opts Options, r *Rng) string {
	// Constant.cpp:164–171 — binary path includes "LL"
	if s, ok := maybeBinaryConstant(opts, r, 16, "LL"); ok {
		return s
	}
	hex := r.RandomHexDigits(16)
	return "0x" + hex + "LL"
}

func generateRandomInt128Constant(opts Options, r *Rng) string {
	// Constant.cpp:126–133
	if s, ok := maybeBinaryConstant(opts, r, 16, ""); ok {
		return s
	}
	hex := r.RandomHexDigits(16)
	return "0x" + hex
}

// generateRandomFloatHexConstant mirrors GenerateRandomFloatHexConstant.
// Constant.cpp:187–199 — pure_rnd_upto(100); hex; pure_rnd_flipcoin(50) for +/− exp.
func generateRandomFloatHexConstant(r *Rng) string {
	if r == nil {
		// C++ always has RNG; sticky no invent fixed literal
		SetError(ErrGeneric)
		return ""
	}
	exp := int(r.RndUpto(100))
	sign := "+"
	// pure_rnd_flipcoin(50) — random mode == rnd_flipcoin(50)
	if !r.RndFlipcoin(50) {
		sign = "-"
	}
	return fmt.Sprintf("0x%s.%sp%s%d", r.RandomHexDigits(1), r.RandomHexDigits(6), sign, exp)
}
