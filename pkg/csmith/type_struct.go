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

// MakeOneStructField mirrors Type::make_one_struct_field.
// Type.cpp:683–697 + ChooseRandomTypeFilter depth gate (Type.cpp:240–242).
// Nested prior structs allowed when StructDepth < MaxNestedStructLevel.
func MakeOneStructField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp:687–691 — ChooseRandomTypeFilter(for_field_var=true) over AllTypes
	// (no soft invent 15% nested-struct path then simple-only)
	var ft *Type
	if env != nil {
		ft = env.ChooseRandom(r, opts, probs, true)
	}
	// Type.cpp:687–691 — ERROR_RETURN when AllTypes empty / choose fails; no soft invent simple
	if ft == nil {
		return StructField{Name: fmt.Sprintf("f%d", fieldIdx), BitWidth: -1}
	}
	// Type.cpp:692–694 — FieldConstProb / FieldVolatileProb random_qualifiers
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
	hasAssign := IfStructWillHaveAssignOps(r, opts, probs)
	st := &Type{
		isStruct:     true,
		StructName:   tag,
		Fields:       fields,
		Packed:       packed,
		Used:         true,
		HasAssignOps: hasAssign,
		// Type.cpp:1094–1096 — hasAssignOps || checkImplicitNontrivialAssignOps(fields)
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOps(opts, fields),
	}
	// Type.cpp:126 / Bookkeeper::record_type_with_bitfields
	RecordTypeWithBitfields(st)
	if env != nil {
		env.StructTypes = append(env.StructTypes, st)
		env.AllTypes = append(env.AllTypes, st)
	}
	return st
}

// CheckImplicitNontrivialAssignOps mirrors Type.cpp checkImplicitNontrivialAssignOps.
// Type.cpp:259–269 — true if any field has_implicit_nontrivial_assign_ops (C++ only).
func CheckImplicitNontrivialAssignOps(opts Options, fields []StructField) bool {
	if !opts.LangCPP {
		return false
	}
	for _, f := range fields {
		if f.Type != nil && f.Type.HasImplicitNontrivialAssignOps {
			return true
		}
	}
	return false
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
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			tag := fmt.Sprintf("S%d", len(env.StructTypes))
			MakeRandomStructType(r, opts, probs, env, tag)
			if len(env.StructTypes) > 20 {
				break
			}
		}
	}
	if opts.Unions {
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			tag := fmt.Sprintf("U%d", len(env.UnionTypes))
			MakeRandomUnionType(r, opts, probs, env, tag)
			if len(env.UnionTypes) > 20 {
				break
			}
		}
	}
}

// OutputStructDecl emits a C struct definition.
func (t *Type) OutputStructDecl() string {
	return t.OutputStructDeclOpts(nil, nil)
}

// OutputStructDeclOpts optionally emits type attributes (Type.cpp type_attr_generator).
func (t *Type) OutputStructDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	if t == nil || !t.isStruct {
		return ""
	}
	var b strings.Builder
	if t.Packed {
		b.WriteString("#pragma pack(push, 1)\n")
	}
	b.WriteString("struct ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.Output(r))
	}
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

// GenerateRandomConstantInRange mirrors GenerateRandomConstantInRange for bitfields.
// Constant.cpp:225–250 — small random value within ~2^(bound/2).
func GenerateRandomConstantInRange(typ *Type, bound int, opts Options, r *Rng) string {
	// Constant.cpp:225+ — pure_rnd_*; no soft invent NewRng(0)
	if r == nil {
		return ""
	}
	if bound <= 0 {
		// invalid bitfield width; no soft invent "0" for broken range
		return ""
	}
	// b = 2^(bound/2); clamp
	exp := bound / 2
	if exp < 0 {
		exp = 0
	}
	if exp > 30 {
		exp = 30
	}
	bmax := uint32(1) << uint(exp)
	if bmax == 0 {
		bmax = 1
	}
	num := int(r.RndUpto(bmax))
	signed := typ != nil && typ.IsSimple() && typ.IsSigned()
	if signed && r.RndFlipcoin(50) {
		num = -num
	}
	// format as small constant for field type
	if typ != nil && typ.IsSimple() {
		return formatSmallConstant(typ.Simple(), num, opts)
	}
	if num < 0 {
		return fmt.Sprintf("%d", num)
	}
	return fmt.Sprintf("%d", num)
}

// MakeStructConstant mirrors GenerateRandomStructConstant.
// Constant.cpp:253–284 — skip zero-width bitfields; bitfields use in-range constants.
func MakeStructConstant(r *Rng, opts Options, probs *Probabilities, st *Type) *Constant {
	if st == nil || !st.isStruct {
		// Constant.cpp assert(type); no soft invent MakeInt(0)
		return nil
	}
	var b strings.Builder
	b.WriteString("{")
	first := true
	for _, f := range st.Fields {
		// Type::is_unamed_padding / bound==0 → skip (Constant.cpp:265–267)
		if f.BitWidth == 0 {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		var val string
		if f.BitWidth > 0 {
			// bitfield: GenerateRandomConstantInRange
			val = GenerateRandomConstantInRange(f.Type, f.BitWidth, opts, r)
		} else if f.Type != nil && f.Type.IsStruct() {
			if c := MakeStructConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
		} else if f.Type != nil && f.Type.IsUnion() {
			if c := MakeUnionConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
		} else if f.Type != nil {
			// Constant.cpp:271 — GenerateRandomConstant(fields[i]); no soft invent "0"
			if c := MakeRandom(f.Type, opts, r); c != nil {
				val = c.Value
			}
		}
		// nil field type / failed GenerateRandomConstant — empty (no invent "0")
		b.WriteString(val)
	}
	b.WriteString("}")
	return &Constant{Type: st, Value: b.String()}
}

// MakeOneUnionField mirrors Type::make_one_union_field.
// Type.cpp:699–763 — bitfield optional; else non-pointer / no-bitfield structs;
// no union-in-union; 15% struct fields.
func MakeOneUnionField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp:702–706 — bitfield when bitfields && !ccomp
	if opts.Bitfields && !opts.CComp && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
		return MakeOneBitfield(r, opts, probs, fieldIdx, true)
	}
	// Build ok_nonstruct_types and struct_types (Type.cpp:708–739)
	var nonStruct []*Type
	var structTypes []*Type
	if env != nil {
		for _, t := range env.AllTypes {
			if t == nil {
				continue
			}
			// Type.cpp:716–717 — contain_pointer_field rejected
			if t.ContainPointerField() {
				continue
			}
			if !t.IsStruct() && !t.IsUnion() {
				// skip void / filtered simples
				if t.IsSimple() && t.Simple() == EVoid {
					continue
				}
				if t.IsSimple() && t.IsFloat() && !opts.EnableFloat {
					continue
				}
				nonStruct = append(nonStruct, t)
				continue
			}
			// Type.cpp:726–727 — no bitfields in union members for now
			if t.HasBitfields() {
				continue
			}
			// Type.cpp:730–731 — has_implicit_nontrivial_assign_ops() (not mere has_assign_ops)
			if t.HasImplicitNontrivialAssignOps {
				continue
			}
			if t.IsStruct() {
				structTypes = append(structTypes, t)
			}
			// Type.cpp:736–737 — no union in union
		}
	}
	// Type.cpp does not soft-seed simples when AllTypes empty (ERROR would fail)
	var ft *Type
	// Type.cpp:742–755 — do { 15% struct else nonstruct } while (type == nullptr)
	// C++ loops until type set; cap high (no soft invent simple early)
	for tries := 0; tries < 256; tries++ {
		if len(structTypes) > 0 && r.RndFlipcoin(15) {
			ft = structTypes[r.RndUpto(uint32(len(structTypes)))]
			break
		}
		if len(nonStruct) == 0 {
			break
		}
		cand := nonStruct[r.RndUpto(uint32(len(nonStruct)))]
		// Type.cpp:747–752 — SIMPLE_TYPES_PROB_FILTER reject
		if cand.IsSimple() && probs != nil && probs.SimpleTypeWeight(int(cand.Simple())) == 0 {
			continue
		}
		if cand.IsSimple() && cand.Simple() == EVoid {
			continue
		}
		ft = cand
		break
	}
	// Type.cpp:755 — while (type == nullptr); no soft invent simple when pools empty
	if ft == nil {
		return StructField{Name: fmt.Sprintf("f%d", fieldIdx), BitWidth: -1}
	}
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	return StructField{Name: fmt.Sprintf("f%d", fieldIdx), Type: ft, Qfer: q, BitWidth: -1}
}

// MakeRandomUnionType mirrors Type::make_random_union_type.
// Type.cpp:1132–1150.
func MakeRandomUnionType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, tag string) *Type {
	if r == nil {
		return nil
	}
	maxCnt := opts.MaxUnionFields
	if maxCnt < 1 {
		maxCnt = 1
	}
	fieldCnt := int(r.RndUpto(uint32(maxCnt))) + 1
	fields := make([]StructField, 0, fieldCnt)
	for i := 0; i < fieldCnt; i++ {
		fields = append(fields, MakeOneUnionField(r, opts, probs, env, i))
	}
	hasAssign := IfUnionWillHaveAssignOps(r, opts, probs)
	ut := &Type{
		isUnion:      true,
		StructName:   tag,
		Fields:       fields,
		Used:         true,
		HasAssignOps: hasAssign,
		// Type.cpp:1146–1148 — hasAssignOps || checkImplicitNontrivialAssignOps
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOps(opts, fields),
	}
	// Type.cpp:180 — record_type_with_bitfields for unions
	RecordTypeWithBitfields(ut)
	if env != nil {
		env.UnionTypes = append(env.UnionTypes, ut)
		env.AllTypes = append(env.AllTypes, ut)
	}
	return ut
}

// OutputUnionDecl emits a C union definition.
func (t *Type) OutputUnionDecl() string {
	return t.OutputUnionDeclOpts(nil, nil)
}

// OutputUnionDeclOpts optionally emits type attributes.
func (t *Type) OutputUnionDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	if t == nil || !t.isUnion {
		return ""
	}
	var b strings.Builder
	b.WriteString("union ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.Output(r))
	}
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
			b.WriteString(fmt.Sprintf(" : %d", f.BitWidth))
		}
		b.WriteString(";\n")
	}
	b.WriteString("};\n")
	return b.String()
}

// MakeUnionConstant mirrors GenerateRandomUnionConstant — initialize first field only.
// Constant.cpp:288–294.
func MakeUnionConstant(r *Rng, opts Options, probs *Probabilities, ut *Type) *Constant {
	// Constant.cpp:289–291 — assert union with fields; no soft invent MakeInt(0)
	if ut == nil || !ut.isUnion || len(ut.Fields) == 0 {
		return nil
	}
	f0 := ut.Fields[0]
	var val string
	if f0.Type != nil && f0.Type.IsStruct() {
		if c := MakeStructConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
	} else if f0.Type != nil && f0.Type.IsUnion() {
		if c := MakeUnionConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
	} else if f0.Type != nil {
		// Constant.cpp:292 — GenerateRandomConstant(fields[0]); no soft invent "0"
		if c := MakeRandom(f0.Type, opts, r); c != nil {
			val = c.Value
		}
	}
	return &Constant{Type: ut, Value: "{" + val + "}"}
}
