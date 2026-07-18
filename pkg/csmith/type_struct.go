// Upstream: Type.cpp make_random_struct_type / make_normal_struct_fields / GenerateAllTypes.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// MoreTypesProbability mirrors Type.cpp MoreTypesProbability.
// Always true while AllTypes-like count < 10; else 50% MoreStructUnionProb.
func MoreTypesProbability(r *Rng, probs *Probabilities, typeCount int) bool {
	if typeCount < 10 {
		return true
	}
	p := 50
	if probs != nil {
		p = probs.Single(PMoreStructUnionProb)
	}
	return r.RndFlipcoin(uint32(p))
}

// MakeOneStructField mirrors Type::make_one_struct_field for simple fields only
// (no nested struct selection until fields can reference prior structs).
// Type.cpp:683–697 — pick from AllTypes with filter; we pick nonvoid simple.
func MakeOneStructField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Prefer simple types; occasionally nested prior struct if env has any
	var ft *Type
	if env != nil && len(env.StructTypes) > 0 && r.RndFlipcoin(15) {
		ft = env.StructTypes[r.RndUpto(uint32(len(env.StructTypes)))]
	} else {
		st := ChooseRandomNonvoidSimple(r, probs)
		ft = GetSimpleType(st)
	}
	// FieldConstProb / FieldVolatileProb random_qualifiers
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	return StructField{
		Name:     fmt.Sprintf("f%d", fieldIdx),
		Type:     ft,
		Qfer:     q,
		BitWidth: -1,
	}
}


// MakeOneBitfield mirrors Type::make_one_bitfield.
// Type.cpp:638–668 — signed flip, int/uint type, field qfer, length rnd_upto(int_size*8).
func MakeOneBitfield(r *Rng, opts Options, probs *Probabilities, fieldIdx int, prevZero bool) StructField {
	maxLen := opts.IntSize * 8
	if maxLen < 1 {
		maxLen = 32
	}
	sign := r.RndFlipcoin(uint32(probs.Single(PBitFieldsSignedProb)))
	var ft *Type
	if sign {
		ft = GetIntType()
	} else {
		ft = GetSimpleType(EUInt)
	}
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	length := int(r.RndUpto(uint32(maxLen)))
	// force non-zero if first field or previous was zero-length
	if length == 0 && prevZero {
		if maxLen <= 2 {
			length = 1
		} else {
			length = int(r.RndUpto(uint32(maxLen-1))) + 1
		}
	}
	return StructField{
		Name:     fmt.Sprintf("f%d", fieldIdx),
		Type:     ft,
		Qfer:     q,
		BitWidth: length,
	}
}

// MakeRandomStructType mirrors Type::make_random_struct_type.
// Type.cpp:1075–1130 — BitFieldsCreationProb chooses full-bitfields vs normal fields.
func MakeRandomStructType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, tag string) *Type {
	if r == nil {
		return nil
	}
	maxCnt := opts.MaxStructFields
	if maxCnt < 1 {
		maxCnt = 1
	}
	fieldCnt := maxCnt
	if !opts.FixedStructFields {
		fieldCnt = int(r.RndUpto(uint32(maxCnt))) + 1
	}
	// is_bitfields = bitfields && flipcoin(BitFieldsCreationProb)
	fullBitfields := opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldsCreationProb)))
	fields := make([]StructField, 0, fieldCnt)
	prevZero := true // first field cannot be zero-width
	for i := 0; i < fieldCnt; i++ {
		if fullBitfields {
			// make_full_bitfields_struct_fields: ScalarFieldInFullBitFieldsProb → normal else bitfield
			if r.RndFlipcoin(uint32(probs.Single(PScalarFieldInFullBitFieldsProb))) {
				fields = append(fields, MakeOneStructField(r, opts, probs, env, i))
				prevZero = false
			} else {
				f := MakeOneBitfield(r, opts, probs, i, prevZero)
				prevZero = f.BitWidth == 0
				fields = append(fields, f)
			}
			continue
		}
		// make_normal_struct_fields: BitFieldInNormalStructProb → bitfield
		if opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
			f := MakeOneBitfield(r, opts, probs, i, prevZero)
			prevZero = f.BitWidth == 0
			fields = append(fields, f)
			continue
		}
		fields = append(fields, MakeOneStructField(r, opts, probs, env, i))
		prevZero = false
	}
	packed := false
	if opts.PackedStruct {
		packed = r.RndFlipcoin(50)
	}
	st := &Type{
		isStruct:   true,
		StructName: tag,
		Fields:     fields,
		Packed:     packed,
		Used:       true,
	}
	if env != nil {
		env.StructTypes = append(env.StructTypes, st)
		env.AllTypes = append(env.AllTypes, st)
	}
	return st
}

// GenerateAllTypesEnv mirrors GenerateAllTypes for random mode with structs.
// Type.cpp:1179–1202.
func GenerateAllTypesEnv(r *Rng, opts Options, probs *Probabilities, env *TypeEnv) {
	if env == nil {
		return
	}
	// GenerateSimpleTypes already cached; seed AllTypes with non-void simples
	if len(env.AllTypes) == 0 {
		for st := EChar; int(st) < MaxSimpleTypes; st++ {
			if st == EVoid {
				continue
			}
			if st == EFloat && !opts.EnableFloat {
				continue
			}
			if (st == ELongLong || st == EULongLong) && !opts.AllowInt64() {
				continue
			}
			env.AllTypes = append(env.AllTypes, GetSimpleType(st))
		}
	}
	if opts.Structs {
		// count types for MoreTypesProbability — use len(AllTypes)
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			tag := fmt.Sprintf("S%d", len(env.StructTypes))
			MakeRandomStructType(r, opts, probs, env, tag)
			// stop runaway
			if len(env.StructTypes) > 20 {
				break
			}
		}
	}
	// unions deferred
}

// OutputStructDecl emits a C struct definition.
func (t *Type) OutputStructDecl() string {
	if t == nil || !t.isStruct {
		return ""
	}
	var b strings.Builder
	if t.Packed {
		b.WriteString("#pragma pack(push, 1)\n")
	}
	b.WriteString("struct ")
	b.WriteString(t.StructName)
	b.WriteString(" {\n")
	for _, f := range t.Fields {
		b.WriteString("   ")
		if f.Qfer.IsConst() {
			b.WriteString(" const")
		}
		if f.Qfer.IsVolatile() {
			b.WriteString(" volatile")
		}
		b.WriteString(" ")
		if f.Type != nil {
			b.WriteString(f.Type.CName())
		} else {
			b.WriteString("int")
		}
		b.WriteString(" ")
		b.WriteString(f.Name)
		if f.BitWidth >= 0 {
			// bitfield: "int f0 : 3;" (width 0 allowed as padding after first)
			b.WriteString(fmt.Sprintf(" : %d", f.BitWidth))
		}
		b.WriteString(";\n")
	}
	b.WriteString("};")
	if t.Packed {
		b.WriteString("\n#pragma pack(pop)")
	}
	b.WriteString("\n")
	return b.String()
}

// MakeStructConstant mirrors GenerateRandomStructConstant simplified.
func MakeStructConstant(r *Rng, opts Options, probs *Probabilities, st *Type) *Constant {
	if st == nil || !st.isStruct {
		return MakeInt(0)
	}
	var b strings.Builder
	b.WriteString("{")
	for i, f := range st.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		if f.Type != nil && f.Type.IsStruct() {
			c := MakeStructConstant(r, opts, probs, f.Type)
			b.WriteString(c.Value)
		} else if f.Type != nil {
			c := MakeRandom(f.Type, opts, r)
			if c != nil {
				b.WriteString(c.Value)
			} else {
				b.WriteString("0")
			}
		} else {
			b.WriteString("0")
		}
	}
	b.WriteString("}")
	return &Constant{Type: st, Value: b.String()}
}
