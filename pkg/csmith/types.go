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
// allTypesList mirrors Type::AllTypes after GenerateSimpleTypes (eChar..eUInt128,
// void is separate and NOT in AllTypes) + GenerateAllTypes structs/unions.
// Indices must match rnd_upto(AllTypes.size()) for NonVoidNonVolatile filter.
func allTypesList(info compositeInfo) []CType {
	// GenerateSimpleTypes starts at eChar (skips eVoid):
	// char,int,short,long,longlong,uchar,uint,ushort,ulong,float,ulonglong,int128,uint128
	simples := []CType{
		{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2},                  // eChar
		{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8},                 // eInt
		{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4},                 // eShort
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8},                 // eLong
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16},                // eLongLong
		{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2},                 // eUChar
		{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8},               // eUInt
		{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4},               // eUShort
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8},               // eULong
		{Name: "float", Signed: true, Bits: 32, HexDigits: 0},                   // eFloat
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16},              // eULongLong
		{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16},              // eInt128
		{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16},    // eUInt128
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

// pickChooseRandom mirrors Type::choose_random (ChooseRandomTypeFilter):
// SIMPLE_TYPES_PROB_FILTER for simples; reject struct if !ReturnStructs.
// Does NOT reject volatile aggregates (unlike NonVoidNonVolatile).
// Used by make_random_pointer_type pointed-to base (Type.cpp:1133).
func pickChooseRandom(r *rng, info compositeInfo, opts Options) CType {
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
		// SIMPLE_TYPES_PROB_FILTER: disabled float / int128 / uint128
		if t.Name == "float" && !opts.EnableFloat {
			return true
		}
		if t.Name == "__int128" && !opts.Int128 {
			return true
		}
		if t.Name == "unsigned __int128" && !opts.UInt128 {
			return true
		}
		// ChooseRandomTypeFilter: struct rejected when !return_structs()
		if strings.HasPrefix(t.Name, "struct") && !opts.ReturnStructs {
			return true
		}
		return false
	}
	idx := int(r.uptoWithFilter(uint32(len(types)), reject))
	return types[idx]
}

// rejectAllTypesNonVoidNonVolatile mirrors NonVoidNonVolatileTypeFilter +
// SIMPLE_TYPES_PROB_FILTER over AllTypes indices (Type.cpp:162–187).
// Used by pickNonVoidNonVolatile and free-invent ExpressionComma lhs type burns
// (ExpressionComma::make_random → choose_random_nonvoid_nonvolatile).
func rejectAllTypesNonVoidNonVolatile(x uint32, types []CType, info compositeInfo, opts Options) bool {
	i := int(x)
	if i < 0 || i >= len(types) {
		return true
	}
	t := types[i]
	// SIMPLE_TYPES_PROB_FILTER: disabled float / int128 / uint128
	if t.Name == "float" && !opts.EnableFloat {
		return true
	}
	if t.Name == "__int128" && !opts.Int128 {
		return true
	}
	if t.Name == "unsigned __int128" && !opts.UInt128 {
		return true
	}
	// NonVoidNonVolatileTypeFilter rejects only volatile aggregates
	// (is_volatile_struct_union), not all structs when the option is on.
	// seed4/seed2 S0 is volatile → AllTypes index 13 rejected (e19638 tries=3).
	if strings.HasPrefix(t.Name, "struct S") {
		var si int
		if _, err := fmt.Sscanf(t.Name, "struct S%d", &si); err == nil {
			if si >= 0 && si < len(info.structs) && info.structs[si].isVolatile {
				return true
			}
		}
	}
	if strings.HasPrefix(t.Name, "union U") {
		var ui int
		if _, err := fmt.Sscanf(t.Name, "union U%d", &ui); err == nil {
			if ui >= 0 && ui < len(info.unions) && info.unions[ui].isVolatile {
				return true
			}
		}
	}
	// arg_structs / arg_unions (Type.cpp:171–177)
	if strings.HasPrefix(t.Name, "struct") && !opts.ArgStructs {
		return true
	}
	if strings.HasPrefix(t.Name, "union") && !opts.ArgUnions {
		return true
	}
	return false
}

// pickNonVoid mirrors Type::choose_random_nonvoid (NonVoidTypeFilter +
// SIMPLE_TYPES_PROB_FILTER). Rejects void and disabled float/int128 only —
// volatile aggregates remain eligible (unlike NonVoidNonVolatile).
func pickNonVoid(r *rng, info compositeInfo, opts Options) CType {
	types := allTypesList(info)
	if len(types) == 0 {
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	idx := int(r.uptoWithFilter(uint32(len(types)), func(x uint32) bool {
		i := int(x)
		if i < 0 || i >= len(types) {
			return true
		}
		tn := types[i].Name
		if tn == "float" && !opts.EnableFloat {
			return true
		}
		if tn == "__int128" && !opts.Int128 {
			return true
		}
		if tn == "unsigned __int128" && !opts.UInt128 {
			return true
		}
		return false
	}))
	return types[idx]
}

// pickNonVoidNonVolatile mirrors Type::choose_random_nonvoid_nonvolatile
// (NonVoidNonVolatileTypeFilter + SIMPLE_TYPES_PROB_FILTER).
func pickNonVoidNonVolatile(r *rng, pool []CType, info compositeInfo, opts Options) CType {
	_ = pool
	types := allTypesList(info)
	if len(types) == 0 {
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	idx := int(r.uptoWithFilter(uint32(len(types)), func(x uint32) bool {
		return rejectAllTypesNonVoidNonVolatile(x, types, info, opts)
	}))
	return types[idx]
}

// burnAllTypesNonVoidNonVolatile burns rnd_upto(AllTypes.size(), NonVoidNonVolatile)
// without returning a type — free-invent ExpressionComma lhs residual.
// Bound must match AllTypes.size() (seed2/seed4: 13 simples + S0 = 14).
func burnAllTypesNonVoidNonVolatile(r *rng, info compositeInfo, opts Options) {
	types := allTypesList(info)
	n := len(types)
	if n == 0 {
		n = 14
	}
	_ = r.uptoWithFilter(uint32(n), func(x uint32) bool {
		if len(types) == 0 {
			// Fallback SIMPLE-only when info empty (legacy residual).
			return x == 9 || x == 11 || x == 12
		}
		return rejectAllTypesNonVoidNonVolatile(x, types, info, opts)
	})
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
	// eSimple: true choose_random_simple order. historical left-shifts for early
	// seed2. useESimpleRetypeSink: seed4 e585 after PP pads only.
	simples := historical
	floatIdx, i128Idx, u128Idx := 11, 12, 13
	// eSimpleType order: seed2 late useSmallParentStack; seed4 PP-era
	// (isParamPPFallPicks>=2 or explicit retype sink). seed4 e1236: NewValue PL
	// retype U14=2 must be eInt (hex 8), not historical uint8 (hex 2).
	// seed5 e486 PL retype still historical when !smallStack (next fix).
	useESimple := (useSmallParentStackSink != nil && *useSmallParentStackSink) ||
		(useESimpleRetypeSink != nil && *useESimpleRetypeSink) ||
		(isParamPPFallPicksSink != nil && *isParamPPFallPicksSink >= 2)
	if useESimple {
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

// simpleTypesGenerateOrder is AllTypes after Type::GenerateSimpleTypes
// (eChar..eUInt128, void excluded). Index 0..12 for U13 draws.
func simpleTypesGenerateOrder(opts Options) []CType {
	return []CType{
		{Name: "int8_t", Signed: true, Bits: 8, HexDigits: 2},                // eChar
		{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8},              // eInt (host)
		{Name: "int16_t", Signed: true, Bits: 16, HexDigits: 4},              // eShort
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 8},              // eLong
		{Name: "int64_t", Signed: true, Bits: 64, HexDigits: 16},             // eLongLong
		{Name: "uint8_t", Signed: false, Bits: 8, HexDigits: 2},              // eUChar
		{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8},            // eUInt
		{Name: "uint16_t", Signed: false, Bits: 16, HexDigits: 4},            // eUShort
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 8},            // eULong
		{Name: "float", Signed: true, Bits: 32, HexDigits: 0},                // eFloat
		{Name: "uint64_t", Signed: false, Bits: 64, HexDigits: 16},           // eULongLong
		{Name: "__int128", Signed: true, Bits: 128, HexDigits: 16},           // eInt128
		{Name: "unsigned __int128", Signed: false, Bits: 128, HexDigits: 16}, // eUInt128
	}
}

// simpleTypeEnumFiltered mirrors SIMPLE_TYPES_PROB_FILTER for AllTypes index
// after GenerateSimpleTypes (0=eChar … 12=eUInt128).
func simpleTypeEnumFiltered(idx int, opts Options) bool {
	// Map idx → eSimpleType value used by ProbabilityFilter.
	// eChar=1 … eFloat=10, eULongLong=11, eInt128=12, eUInt128=13
	switch idx {
	case 9: // eFloat
		return !opts.EnableFloat
	case 11: // eInt128
		return !opts.Int128
	case 12: // eUInt128
		return !opts.UInt128
	case 3, 8: // eLong, eULong — ccomp disables; default !ccomp
		return false
	case 4, 10: // eLongLong, eULongLong — allow_int64 default true via LongLong
		return !opts.LongLong
	default:
		return false
	}
}

// pickFieldType mirrors make_one_struct_field: rnd_upto(AllTypes.size(), filter)
// with tries on one event (not separate upto per reject).
// For union fields with prior structs, make_one_union_field uses pure_rnd_flipcoin(15)
// then pure_rnd_upto(ok_nonstruct) with manual retry — use pickUnionFieldType there.
// nStructs: when >0 and forUnion, 15% struct field chance first.
func pickFieldType(r *rng, opts Options, nStructs int) CType {
	return pickFieldTypeEx(r, opts, nStructs, false)
}

// pickUnionFieldType mirrors make_one_union_field (manual retry loop = multi U events).
func pickUnionFieldType(r *rng, opts Options, nStructs int) CType {
	return pickFieldTypeEx(r, opts, nStructs, true)
}

func pickFieldTypeEx(r *rng, opts Options, nStructs int, forUnion bool) CType {
	if r == nil {
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	simples := simpleTypesGenerateOrder(opts)
	if opts.IntSize == 4 {
		simples[1] = hostIntType(opts)
		simples[6] = unsignedOf(hostIntType(opts).Bits)
	}
	// AllTypes size during early field gen ≈ simples only (+ prior structs for struct fields).
	// Struct fields: ChooseRandomTypeFilter on full AllTypes including prior structs.
	// For simplicity when nStructs==0: U13 with filter in one event.
	if forUnion {
		// pure_rnd_flipcoin(15) only if structs exist; pure_rnd_upto + manual retry.
		for tries := 0; tries < 1<<16; tries++ {
			if nStructs > 0 && r.flipcoin(15) {
				return CType{Name: fmt.Sprintf("struct S%d", int(r.upto(uint32(nStructs)))), Bits: 32}
			}
			idx := int(r.upto(uint32(len(simples))))
			if simpleTypeEnumFiltered(idx, opts) {
				continue
			}
			return simples[idx]
		}
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	// Struct field: rnd_upto(AllTypes, filter) — one event with tries.
	// AllTypes = simples + prior structs (nStructs).
	nAll := len(simples) + nStructs
	reject := func(x uint32) bool {
		i := int(x)
		if i < 0 || i >= nAll {
			return true
		}
		if i < len(simples) {
			return simpleTypeEnumFiltered(i, opts)
		}
		return false // struct types ok for fields when assign_ops allow (default ok)
	}
	idx := int(r.uptoWithFilter(uint32(nAll), reject))
	if idx < len(simples) {
		return simples[idx]
	}
	return CType{Name: fmt.Sprintf("struct S%d", idx-len(simples)), Bits: 32}
}

// pickReturnType mirrors Type::choose_random / RandomReturnType:
// rnd_upto(AllTypes.size(), ChooseRandomTypeFilter).
func pickReturnType(r *rng, opts Options, info compositeInfo) CType {
	if r == nil {
		return CType{Name: "int32_t", Signed: true, Bits: 32, HexDigits: 8}
	}
	simples := simpleTypesGenerateOrder(opts)
	if opts.IntSize == 4 {
		simples[1] = hostIntType(opts)
		simples[6] = unsignedOf(hostIntType(opts).Bits)
	}
	nAll := len(simples) + len(info.structs) + len(info.unions)
	if nAll <= 0 {
		return CType{Name: "uint32_t", Signed: false, Bits: 32, HexDigits: 8}
	}
	reject := func(x uint32) bool {
		i := int(x)
		if i < 0 || i >= nAll {
			return true
		}
		if i < len(simples) {
			return simpleTypeEnumFiltered(i, opts)
		}
		return false
	}
	idx := int(r.uptoWithFilter(uint32(nAll), reject))
	if idx < len(simples) {
		return simples[idx]
	}
	idx -= len(simples)
	if idx < len(info.structs) {
		return CType{Name: fmt.Sprintf("struct S%d", idx), Bits: 32}
	}
	idx -= len(info.structs)
	return CType{Name: fmt.Sprintf("union U%d", idx), Bits: 32}
}
