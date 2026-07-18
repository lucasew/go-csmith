// Upstream: Constant.h / Constant.cpp (make_random, GenerateRandomConstant, hex helpers).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strconv"
)

// Constant mirrors Constant : Expression for literal values (value string only for now).
type Constant struct {
	Type  *Type
	Value string
}

// MakeRandom mirrors Constant::make_random.
// Constant.cpp:423–426 → GenerateRandomConstant.
func MakeRandom(typ *Type, opts Options, r *Rng) *Constant {
	v := generateRandomConstant(typ, opts, r)
	return &Constant{Type: typ, Value: v}
}

// MakeRandomUpto mirrors Constant::make_random_upto — rnd_upto(limit) as eUInt decimal.
func MakeRandomUpto(limit uint32, r *Rng) *Constant {
	if r == nil {
		r = NewRng(0)
	}
	n := r.RndUpto(limit)
	return &Constant{Type: GetSimpleType(EUInt), Value: strconv.FormatUint(uint64(n), 10)}
}

// MakeInt mirrors Constant::make_int.
func MakeInt(v int) *Constant {
	return &Constant{Type: GetSimpleType(EInt), Value: strconv.Itoa(v)}
}

// generateRandomConstant mirrors GenerateRandomConstant (simple + pointer only).
// Constant.cpp:296–...
func generateRandomConstant(typ *Type, opts Options, r *Rng) string {
	if r == nil {
		r = NewRng(0)
	}
	if typ == nil {
		return "0"
	}
	// Pointer → "0" (no RNG). Constant.cpp:308–310
	if typ.PtrType() != nil {
		return "0"
	}
	if !typ.IsSimple() || typ.Simple() == EVoid {
		return "0"
	}
	st := typ.Simple()

	// pure_rnd_flipcoin(50) — in random mode == rnd_flipcoin(50)
	if r.RndFlipcoin(50) {
		// small integer path
		var num int
		if r.RndFlipcoin(50) {
			num = int(r.RndUpto(3)) - 1
		} else {
			num = int(r.RndUpto(20)) - 10
		}
		return formatSmallConstant(st, num, opts)
	}
	// hex / wide path by simple type
	switch st {
	case EChar, EUChar:
		return generateRandomCharConstant(opts, r)
	case EInt, EUInt:
		return generateRandomIntConstant(opts, r)
	case EShort, EUShort:
		return generateRandomShortConstant(opts, r)
	case ELong, EULong:
		return generateRandomLongConstant(opts, r)
	case ELongLong, EULongLong:
		return generateRandomLongLongConstant(opts, r)
	case EInt128, EUInt128:
		return generateRandomInt128Constant(opts, r)
	case EFloat:
		return generateRandomFloatHexConstant(r)
	default:
		return generateRandomIntConstant(opts, r)
	}
}

func formatSmallConstant(st ESimpleType, num int, opts Options) string {
	// Constant.cpp:329–361 cast + L/UL suffix
	if st == EFloat {
		// GenerateSmallRandomFloatHexConstant deferred
		return "0x0.0p+0"
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

// generateRandomCharConstant — Constant.cpp:100–108
func generateRandomCharConstant(opts Options, r *Rng) string {
	if opts.BinaryConstant && r.RndFlipcoin(uint32(3)) { // BinaryConstProb default 3
		// binary path uses HexToBinary(RandomHexDigits(2)) — port hex only for now if binary off (defaults)
	}
	hex := r.RandomHexDigits(2)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomIntConstant(opts Options, r *Rng) string {
	hex := r.RandomHexDigits(8)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomShortConstant(opts Options, r *Rng) string {
	hex := r.RandomHexDigits(4)
	if opts.CComp || !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongConstant(opts Options, r *Rng) string {
	hex := r.RandomHexDigits(8)
	if !opts.LongLong {
		return "0x" + hex
	}
	return "0x" + hex + "L"
}

func generateRandomLongLongConstant(opts Options, r *Rng) string {
	hex := r.RandomHexDigits(16)
	return "0x" + hex + "LL"
}

func generateRandomInt128Constant(opts Options, r *Rng) string {
	_ = opts
	hex := r.RandomHexDigits(16)
	return "0x" + hex
}

// generateRandomFloatHexConstant — Constant.cpp:187+ simplified
func generateRandomFloatHexConstant(r *Rng) string {
	// pure_rnd_upto(100) for exp; RandomHexDigits(1)+"."+RandomHexDigits(6)+"p"+/-
	exp := int(r.RndUpto(100))
	// sign of exp: not fully ported; use + for positive half
	return fmt.Sprintf("0x%s.%sp+%d", r.RandomHexDigits(1), r.RandomHexDigits(6), exp)
}
