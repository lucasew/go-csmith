package csmith

import (
	"fmt"
	"strings"
)

// CType is a lightweight C scalar type descriptor used by the current generator.
type CType struct {
	Name      string
	Signed    bool
	Bits      int
	HexDigits int // upstream RandomHexDigits count for Constant::make_random
}

func hostIntType(opts Options) CType {
	switch opts.IntSize {
	case 1:
		return CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2}
	case 2:
		return CType{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4}
	case 8:
		return CType{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8}
	default:
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
}

func unsignedOf(bits int) CType {
	switch bits {
	case 8:
		return CType{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2}
	case 16:
		return CType{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4}
	case 64:
		return CType{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8}
	default:
		return CType{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8}
	}
}

func typePool(opts Options) []CType {
	// Mirrors Type::GenerateSimpleTypes order:
	// eChar, eSChar, eUChar, eShort, eUShort, eInt, eUInt,
	// eLong, eULong, eLongLong, eULongLong, eInt128, eUInt128.
	// Keep entries even when aliases collapse to same C type to preserve
	// upstream RNG selection cardinality.
	pool := make([]CType, 0, 13)
	pool = append(pool, CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2})   // char
	pool = append(pool, CType{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2})   // signed char
	pool = append(pool, CType{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2}) // unsigned char
	pool = append(pool, CType{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4})
	pool = append(pool, CType{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4})
	pool = append(pool, hostIntType(opts))
	pool = append(pool, unsignedOf(hostIntType(opts).Bits))
	pool = append(pool, CType{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8})   // eLong
	pool = append(pool, CType{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8}) // eULong
	if opts.LongLong {
		pool = append(pool, CType{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16})   // eLongLong
		pool = append(pool, CType{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16}) // eULongLong
	}
	pool = append(pool, CType{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16})
	pool = append(pool, CType{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16})
	return pool
}

func pickType(r *rng, pool []CType) CType {
	return pool[int(r.upto(uint32(len(pool))))]
}

// allTypesList mirrors Type::AllTypes after GenerateSimpleTypes + aggregates:
// eChar..eUInt128 (13 entries, always includes float slot) then structs then unions.
func allTypesList(info compositeInfo) []CType {
	simples := []CType{
		{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2},
		{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8},
		{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16},
		{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2},
		{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8},
		{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8},
		{Name: "float", Signed: true, Bits: 32, HexDigits: 0},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16},
		{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16},
		{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16},
	}
	out := append([]CType{}, simples...)
	for i := range info.structs {
		out = append(out, CType{Name: fmt.Sprintf("struct S%d", i), Bits: 32})
	}
	for i := range info.unions {
		out = append(out, CType{Name: fmt.Sprintf("union U%d", i), Bits: 32})
	}
	return out
}

// pickNonVoidNonVolatile mirrors Type::choose_random_nonvoid_nonvolatile.
func pickNonVoidNonVolatile(r *rng, pool []CType, info compositeInfo, opts Options) CType {
	_ = pool
	types := allTypesList(info)
	if len(types) == 0 {
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	reject := func(x uint32) bool {
		i := int(x)
		if i < 0 || i >= len(types) {
			return true
		}
		t := types[i]
		if t.Name == "float" && !opts.EnableFloat {
			return true
		}
		// NonVoidNonVolatileTypeFilter rejects volatile aggregates.
		if strings.HasPrefix(t.Name, "struct ") || strings.HasPrefix(t.Name, "union ") {
			if opts.VolStructUnionFields {
				return true
			}
		}
		return false
	}
	idx := int(r.uptoWithFilter(uint32(len(types)), reject))
	return types[idx]
}

// pickSimpleNonVoid mirrors Type::choose_random_nonvoid_simple /
// choose_random_simple: rnd_upto(MAX_SIMPLE_TYPES=14, SIMPLE_TYPES_PROB_FILTER).
// Early: historical interleaved layout (seed2 e351 inventory).
// Late useSmallParentStack: true eSimpleType order so float@10 is filtered
// when !EnableFloat (seed2 e1470 U14 tries=2).
func pickSimpleNonVoid(r *rng, opts Options) CType {
	historical := []CType{
		{Name: "void", Bits: 0},
		{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2},
		{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2},
		{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4},
		{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4},
		{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8},
		{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16},
		{Name: "float", Signed: true, Bits: 32, HexDigits: 0},
		{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16},
		{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16},
	}
	// Type.h eSimpleType: void,char,int,short,long,longlong,uchar,uint,ushort,ulong,float,ulonglong,int128,uint128
	eSimple := []CType{
		{Name: "void", Bits: 0},
		{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2},
		{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8},
		{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8},
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16},
		{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2},
		{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8},
		{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8},
		{Name: "float", Signed: true, Bits: 32, HexDigits: 0},
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16},
		{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16},
		{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16},
	}
	simples := historical
	floatIdx, i128Idx, u128Idx := 11, 12, 13
	if useSmallParentStackSink != nil && *useSmallParentStackSink {
		simples = eSimple
		floatIdx, i128Idx, u128Idx = 10, 12, 13
	}
	reject := func(x uint32) bool {
		i := int(x)
		if i < 0 || i >= len(simples) {
			return true
		}
		if i == 0 {
			return true
		}
		if i == floatIdx {
			return !opts.EnableFloat
		}
		if i == i128Idx {
			return !opts.Int128
		}
		if i == u128Idx {
			return !opts.UInt128
		}
		return false
	}
	idx := int(r.uptoWithFilter(uint32(len(simples)), reject))
	return simples[idx]
}

// hexDigitsForConstant mirrors Constant.cpp GenerateRandom*Constant widths:
// char=2, short=4, int/long=8, longlong/int128=16.
func hexDigitsForConstant(t CType) int {
	if t.HexDigits > 0 {
		return t.HexDigits
	}
	switch {
	case t.Bits <= 8:
		return 2
	case t.Bits <= 16:
		return 4
	case t.Bits <= 32:
		return 8
	case t.Bits <= 64:
		// Ambiguous long vs longlong when HexDigits unset; prefer longlong
		// width only when the name suggests it, else classic long (8).
		if strings.Contains(t.Name, "int128") {
			return 16
		}
		return 8
	default:
		return 16
	}
}

func castLiteral(t CType, expr string) string {
	return fmt.Sprintf("((%s)(%s))", t.Name, expr)
}
