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
	// C++ always has RNG; sticky no invent always-true past threshold when r nil
	if r == nil {
		SetError(ErrGeneric)
		return false
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.Single(PMoreStructUnionProb)
	}
	return r.RndFlipcoin(uint32(p))
}

// MakeOneStructField mirrors Type::make_one_struct_field.
// Type.cpp:683–697 + ChooseRandomTypeFilter depth gate (Type.cpp:240–242).
// Nested prior structs allowed when StructDepth < MaxNestedStructLevel.
// On ERROR_RETURN / choose fail returns zero field (Type==nil); callers abort.
func MakeOneStructField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp always has RNG + Probabilities sticky; no invent field shell without them
	if r == nil || probs == nil {
		SetError(ErrGeneric)
		return StructField{}
	}
	// Type.cpp:687–691 — ChooseRandomTypeFilter(for_field_var=true) over AllTypes
	// (no soft invent 15% nested-struct path then simple-only)
	var ft *Type
	if env != nil {
		ft = env.ChooseRandom(r, opts, probs, true)
	}
	// Type.cpp:687–691 — ERROR_RETURN when AllTypes empty / choose fails; no soft invent simple
	if ft == nil || HasError() {
		return StructField{}
	}
	// Type.cpp:692–694 — FieldConstProb / FieldVolatileProb random_qualifiers
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	// Type.cpp:694 ERROR_RETURN after random_qualifiers
	if HasError() {
		return StructField{}
	}
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
	// Type.cpp:641 — CGOptions::int_size()*8; no soft invent 32 when size is 0
	// empty StructField uses BitWidth -1 (not a bitfield) so callers can detect fail
	fail := StructField{BitWidth: -1}
	maxLen := opts.IntSize * 8
	if maxLen < 1 {
		// broken options IR sticky — empty field (no invent maxLen=32)
		SetError(ErrGeneric)
		return fail
	}
	if r == nil || probs == nil {
		SetError(ErrGeneric)
		return fail
	}
	sign := r.RndFlipcoin(uint32(probs.Single(PBitFieldsSignedProb)))
	if HasError() {
		return fail
	}
	var ft *Type
	if sign {
		ft = GetIntType()
	} else {
		ft = GetSimpleType(EUInt)
	}
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	if HasError() {
		return fail
	}
	length := int(r.RndUpto(uint32(maxLen)))
	if HasError() {
		return fail
	}
	// force non-zero if first field or previous was zero-length
	if length == 0 && prevZero {
		if maxLen <= 2 {
			length = 1
		} else {
			length = int(r.RndUpto(uint32(maxLen-1))) + 1
			if HasError() {
				return fail
			}
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
	// Type.cpp always has process RNG; sticky no invent struct type without it
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// Type.cpp:1077–1081 — max_struct_fields as-is; no soft invent maxCnt=1
	maxCnt := opts.MaxStructFields
	if maxCnt < 0 {
		maxCnt = 0
	}
	fieldCnt := maxCnt
	if !opts.FixedStructFields {
		// rnd_upto(max_cnt)+1; max 0 → RndUpto(0)+1 = 1 (matches C++ when max is 0)
		fieldCnt = int(r.RndUpto(uint32(maxCnt))) + 1
	}
	if fieldCnt < 1 {
		// fixed + max 0 → empty type IR; sticky no invent zero-field struct shell
		SetError(ErrGeneric)
		return nil
	}
	// Type.cpp:1082 — ERROR_GUARD(nullptr) after field_cnt draw
	if HasError() {
		return nil
	}
	// is_bitfields = bitfields && flipcoin(BitFieldsCreationProb)
	// Type.cpp:1086–1088 — ERROR_GUARD after flip
	fullBitfields := opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldsCreationProb)))
	if HasError() {
		return nil
	}
	fields := make([]StructField, 0, fieldCnt)
	prevZero := true // first field cannot be zero-width
	for i := 0; i < fieldCnt; i++ {
		var f StructField
		if fullBitfields {
			// make_full_bitfields_struct_fields: ScalarFieldInFullBitFieldsProb → normal else bitfield
			if r.RndFlipcoin(uint32(probs.Single(PScalarFieldInFullBitFieldsProb))) {
				if HasError() {
					return nil
				}
				f = MakeOneStructField(r, opts, probs, env, i)
				prevZero = false
			} else {
				if HasError() {
					return nil
				}
				f = MakeOneBitfield(r, opts, probs, i, prevZero)
				prevZero = f.Type != nil && f.BitWidth == 0
			}
		} else if opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
			// make_normal_struct_fields: BitFieldInNormalStructProb → bitfield
			if HasError() {
				return nil
			}
			f = MakeOneBitfield(r, opts, probs, i, prevZero)
			prevZero = f.Type != nil && f.BitWidth == 0
		} else {
			if HasError() {
				return nil
			}
			f = MakeOneStructField(r, opts, probs, env, i)
			prevZero = false
		}
		// Type.cpp:1090 ERROR_GUARD after make_*_struct_fields; no soft invent nil-type field
		if f.Type == nil || HasError() {
			return nil
		}
		fields = append(fields, f)
	}
	// Type.cpp:1100–1110 — packed_struct; ccomp skips when aggregate/longlong fields
	packed := false
	if opts.PackedStruct {
		if opts.CComp && (HasAggregateField(fields) || HasLongLongField(fields)) {
			// leave packed false
		} else {
			packed = r.RndFlipcoin(50)
			if HasError() {
				return nil
			}
		}
	}
	hasAssign := IfStructWillHaveAssignOps(r, opts, probs)
	if HasError() {
		return nil
	}
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
// Type* always live on Fields; nil hole sticky true (no invent no-nontrivial /
// soft re-pick past incomplete field Type that would skip C++ assign-op bans).
func CheckImplicitNontrivialAssignOps(opts Options, fields []StructField) bool {
	if !opts.LangCPP {
		return false
	}
	for _, f := range fields {
		if f.Type == nil {
			// incomplete field Type sticky has-nontrivial (restrictive)
			SetError(ErrGeneric)
			return true
		}
		if f.Type.HasImplicitNontrivialAssignOps {
			return true
		}
	}
	return false
}

// GenerateAllTypesEnv mirrors GenerateAllTypes for random mode with structs.
// Type.cpp:1179–1202.
// TypeEnv always live; sticky (no invent soft-skip type gen past hole).
func GenerateAllTypesEnv(r *Rng, opts Options, probs *Probabilities, env *TypeEnv) {
	if env == nil {
		SetError(ErrGeneric)
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
	// struct/union generation draws RNG + probs; no invent fixed S0 shells without them
	if opts.Structs && r != nil && probs != nil {
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			// Type.cpp:1191–1193 — make_random_struct_type; sticky ERROR_RETURN aborts further
			tag := fmt.Sprintf("S%d", len(env.StructTypes))
			if MakeRandomStructType(r, opts, probs, env, tag) == nil || HasError() {
				break
			}
			if len(env.StructTypes) > 20 {
				break
			}
		}
	}
	if opts.Unions && r != nil && probs != nil {
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			tag := fmt.Sprintf("U%d", len(env.UnionTypes))
			if MakeRandomUnionType(r, opts, probs, env, tag) == nil || HasError() {
				break
			}
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
// Type.cpp:1836–1884 — OutputStructUnion field loop with bitfield asserts.
func (t *Type) OutputStructDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	// Type* always live at struct emit; sticky no invent decl without it
	if t == nil {
		SetError(ErrGeneric)
		return ""
	}
	// non-struct: soft empty (callers use OutputUnionDecl for unions)
	if !t.isStruct {
		return ""
	}
	// Type.cpp always has sid name (S#); sticky no invent "struct  {"
	if t.StructName == "" {
		SetError(ErrGeneric)
		return ""
	}
	var b strings.Builder
	if t.Packed {
		// Type.cpp:1849–1854 — pack pragmas
		b.WriteString("#pragma pack(push, 1)\n")
	}
	b.WriteString("struct ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.Output(r))
		// residual ERROR sticky — no invent soft-continue fields past attr residual
		if HasError() {
			return ""
		}
	}
	b.WriteString(" {\n")
	j := 0
	for i, f := range t.Fields {
		// Type.cpp:1866+ — bitfield fields: signed/unsigned only, not invent "int"
		if f.BitWidth >= 0 {
			if f.Type == nil || !f.Type.IsSimple() {
				// Type.cpp:1866 assert(eSimple) sticky; fail closed whole decl
				SetError(ErrGeneric)
				return ""
			}
			st := f.Type.Simple()
			// Type.cpp:1868–1873 — eInt → signed; eUInt → unsigned; else assert(0) sticky
			var signedKW string
			switch st {
			case EInt:
				signedKW = "signed"
			case EUInt:
				signedKW = "unsigned"
			default:
				SetError(ErrGeneric)
				return ""
			}
			b.WriteString("   ")
			// Type.cpp:1867 — OutputFirstQuals
			if f.Qfer.IsConst() {
				// residual ERROR sticky — no invent soft-const past IsConst residual hole
				if HasError() {
					return ""
				}
				b.WriteString("const ")
			} else if HasError() {
				// residual ERROR sticky — no invent soft-continue field past IsConst residual false
				return ""
			}
			if f.Qfer.IsVolatile() {
				// residual ERROR sticky — no invent soft-vol past IsVolatile residual hole
				if HasError() {
					return ""
				}
				b.WriteString("volatile ")
			} else if HasError() {
				// residual ERROR sticky — no invent soft-continue field past IsVolatile residual false
				return ""
			}
			b.WriteString(signedKW)
			// Type.cpp:1875 assert(length >= 0); 0 → padding " : 0;"
			if f.BitWidth == 0 {
				b.WriteString(" : ")
				b.WriteString(fmt.Sprintf("%d", f.BitWidth))
			} else {
				b.WriteString(fmt.Sprintf(" f%d : %d", j, f.BitWidth))
				j++
			}
			b.WriteString(";\n")
			_ = i
			continue
		}
		// non-bitfield: qualified type + fN
		if f.Type == nil {
			// Type.cpp always has field type sticky; no soft invent "int"
			SetError(ErrGeneric)
			return ""
		}
		// Type.cpp:1879–1880 — output_qualified_type always live; sticky no invent " fN;"
		ty := f.Qfer.OutputQualifiedType(f.Type)
		// residual ERROR sticky — no invent soft-continue field past OutputQualifiedType residual
		if HasError() {
			return ""
		}
		if ty == "" {
			SetError(ErrGeneric)
			return ""
		}
		b.WriteString("   ")
		b.WriteString(ty)
		// Type.cpp uses f0,f1…; field Name may already be fN from make_one
		name := f.Name
		if name == "" {
			name = fmt.Sprintf("f%d", j)
		}
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString(";\n")
		j++
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
	// Constant.cpp:222–246 — GenerateRandomConstantInRange
	// Constant.cpp:223 — assert(type->eType == eSimple)
	// Constant.cpp:226–245 — only eInt / eUInt; else assert(0)
	// sticky no invent empty/default constant past broken range IR
	if r == nil || typ == nil || !typ.IsSimple() {
		SetError(ErrGeneric)
		return ""
	}
	st := typ.Simple()
	if st != EInt && st != EUInt {
		// assert(0) for other simples — sticky no soft invent generic decimal
		SetError(ErrGeneric)
		return ""
	}
	if bound <= 0 {
		// invalid bitfield width; sticky no invent "0" for broken range
		SetError(ErrGeneric)
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
	// Constant.cpp: pure_rnd_upto(b); no invent bmax=1 when shift underflows (exp clamped)
	if bmax == 0 {
		SetError(ErrGeneric)
		return ""
	}
	num := int(r.RndUpto(bmax))
	// Constant.cpp:226–235 — eInt may negate; eUInt stays non-negative
	if st == EInt && r.RndFlipcoin(50) {
		num = -num
	}
	v := formatSmallConstant(st, num, opts)
	// Constant.cpp:246 — mark_mutable_const → "(" + oss + ")"; no invent ignore option
	if v != "" && opts.MarkMutableConst {
		return "(" + v + ")"
	}
	return v
}

// MakeStructConstant mirrors GenerateRandomStructConstant.
// Constant.cpp:253–284 — skip zero-width bitfields; bitfields use in-range constants.
func MakeStructConstant(r *Rng, opts Options, probs *Probabilities, st *Type) *Constant {
	// Constant.cpp:255 — assert(eStruct); always has RNG for field constants sticky
	// no invent "{}" shell without live RNG / fields path
	if r == nil || st == nil || !st.isStruct {
		SetError(ErrGeneric)
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
		// Type* always live on Fields; Type-nil sticky (no invent soft-empty val then
		// ERROR_GUARD as complete field miss / soft re-pick past incomplete field Type)
		if f.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		var val string
		if f.BitWidth > 0 {
			// bitfield: GenerateRandomConstantInRange (eInt/eUInt only)
			val = GenerateRandomConstantInRange(f.Type, f.BitWidth, opts, r)
		} else if f.Type.IsStruct() {
			if c := MakeStructConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
		} else if f.Type.IsUnion() {
			if c := MakeUnionConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
		} else {
			// Constant.cpp:271 — GenerateRandomConstant(fields[i]); no soft invent "0"
			if c := MakeRandom(f.Type, opts, probs, r); c != nil {
				val = c.Value
			}
		}
		// Constant.cpp ERROR_GUARD("") on empty field — sticky fail whole struct, no invent hole
		if val == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(val)
	}
	b.WriteString("}")
	return &Constant{Type: st, Value: b.String()}
}

// MakeOneUnionField mirrors Type::make_one_union_field.
// Type.cpp:699–763 — bitfield optional; else non-pointer / no-bitfield structs;
// no union-in-union; 15% struct fields.
func MakeOneUnionField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp always has RNG + Probabilities sticky; no invent field shell without them
	if r == nil || probs == nil {
		SetError(ErrGeneric)
		return StructField{}
	}
	// Type.cpp:702–706 — bitfield when bitfields && !ccomp
	if opts.Bitfields && !opts.CComp && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
		return MakeOneBitfield(r, opts, probs, fieldIdx, true)
	}
	// Build ok_nonstruct_types and struct_types (Type.cpp:708–739)
	var nonStruct []*Type
	var structTypes []*Type
	if env != nil {
		for _, t := range env.AllTypes {
			// Type* always live on AllTypes; nil hole fails closed sticky (empty field shell)
			if t == nil {
				SetError(ErrGeneric)
				return StructField{}
			}
			// Type.cpp:716–717 — contain_pointer_field rejected
			if t.ContainPointerField() {
				// residual ERROR sticky — no invent soft-skip then pick later past field-Type hole
				if HasError() {
					return StructField{}
				}
				continue
			}
			if !t.IsStruct() && !t.IsUnion() {
				// skip void / filtered simples
				if t.IsSimple() && t.Simple() == EVoid {
					continue
				}
				if t.IsSimple() && t.IsFloat() && !opts.EnableFloat {
					// residual ERROR sticky — no invent soft-continue then pick later past IsFloat hole
					if HasError() {
						return StructField{}
					}
					continue
				}
				// residual ERROR sticky — no invent soft-continue non-float past IsFloat residual false path
				if HasError() {
					return StructField{}
				}
				nonStruct = append(nonStruct, t)
				continue
			}
			// Type.cpp:726–727 — no bitfields in union members for now
			if t.HasBitfields() {
				// residual ERROR sticky — no invent soft-skip then pick later past bitfield hole
				if HasError() {
					return StructField{}
				}
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
	if ft == nil || HasError() {
		return StructField{}
	}
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext(), false, constP, volP, opts, r)
	if HasError() {
		return StructField{}
	}
	return StructField{Name: fmt.Sprintf("f%d", fieldIdx), Type: ft, Qfer: q, BitWidth: -1}
}

// MakeRandomUnionType mirrors Type::make_random_union_type.
// Type.cpp:1132–1150.
func MakeRandomUnionType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, tag string) *Type {
	// Type.cpp always has process RNG; sticky no invent union type without it
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// Type.cpp:1133–1135 — max_union_fields as-is; no soft invent maxCnt=1
	maxCnt := opts.MaxUnionFields
	if maxCnt < 0 {
		maxCnt = 0
	}
	fieldCnt := int(r.RndUpto(uint32(maxCnt))) + 1
	// Type.cpp:1136 — ERROR_GUARD after field_cnt
	if HasError() {
		return nil
	}
	if fieldCnt < 1 {
		// sticky no invent zero-field union shell
		SetError(ErrGeneric)
		return nil
	}
	fields := make([]StructField, 0, fieldCnt)
	for i := 0; i < fieldCnt; i++ {
		f := MakeOneUnionField(r, opts, probs, env, i)
		// Type.cpp:1140–1141 — make_one_union_field; assert no bitfields on last
		// no soft invent nil-type union field
		if f.Type == nil || HasError() {
			return nil
		}
		if f.Type.HasBitfields() {
			// residual ERROR sticky — no invent soft-skip assert fail past incomplete field Type
			if !HasError() {
				// C++ assert(!fields.back()->has_bitfields()) — complete bitfields still fail closed
				SetError(ErrGeneric)
			}
			return nil
		}
		fields = append(fields, f)
	}
	hasAssign := IfUnionWillHaveAssignOps(r, opts, probs)
	if HasError() {
		return nil
	}
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
// Type.cpp:1836+ OutputStructUnion for unions (same field loop).
func (t *Type) OutputUnionDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	// Type* always live at union emit; sticky no invent decl without it
	if t == nil {
		SetError(ErrGeneric)
		return ""
	}
	// non-union: soft empty
	if !t.isUnion {
		return ""
	}
	// Type.cpp always has sid name (U#); sticky no invent "union  {"
	if t.StructName == "" {
		SetError(ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString("union ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.Output(r))
		// residual ERROR sticky — no invent soft-continue fields past attr residual
		if HasError() {
			return ""
		}
	}
	b.WriteString(" {\n")
	j := 0
	for _, f := range t.Fields {
		if f.BitWidth >= 0 {
			// unions rarely have bitfields; same assert rules as struct sticky
			if f.Type == nil || !f.Type.IsSimple() {
				SetError(ErrGeneric)
				return ""
			}
			st := f.Type.Simple()
			var signedKW string
			switch st {
			case EInt:
				signedKW = "signed"
			case EUInt:
				signedKW = "unsigned"
			default:
				SetError(ErrGeneric)
				return ""
			}
			b.WriteString("   ")
			if f.Qfer.IsConst() {
				// residual ERROR sticky — no invent soft-const past IsConst residual hole
				if HasError() {
					return ""
				}
				b.WriteString("const ")
			} else if HasError() {
				// residual ERROR sticky — no invent soft-continue field past IsConst residual false
				return ""
			}
			if f.Qfer.IsVolatile() {
				// residual ERROR sticky — no invent soft-vol past IsVolatile residual hole
				if HasError() {
					return ""
				}
				b.WriteString("volatile ")
			} else if HasError() {
				// residual ERROR sticky — no invent soft-continue field past IsVolatile residual false
				return ""
			}
			b.WriteString(signedKW)
			if f.BitWidth == 0 {
				b.WriteString(fmt.Sprintf(" : %d;\n", f.BitWidth))
			} else {
				b.WriteString(fmt.Sprintf(" f%d : %d;\n", j, f.BitWidth))
				j++
			}
			continue
		}
		if f.Type == nil {
			SetError(ErrGeneric)
			return ""
		}
		// output_qualified_type always live sticky; no invent " fN;" without type
		ty := f.Qfer.OutputQualifiedType(f.Type)
		// residual ERROR sticky — no invent soft-continue field past OutputQualifiedType residual
		if HasError() {
			return ""
		}
		if ty == "" {
			SetError(ErrGeneric)
			return ""
		}
		b.WriteString("   ")
		b.WriteString(ty)
		name := f.Name
		if name == "" {
			name = fmt.Sprintf("f%d", j)
		}
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString(";\n")
		j++
	}
	b.WriteString("};\n")
	return b.String()
}

// MakeUnionConstant mirrors GenerateRandomUnionConstant — initialize first field only.
// Constant.cpp:288–294.
func MakeUnionConstant(r *Rng, opts Options, probs *Probabilities, ut *Type) *Constant {
	// Constant.cpp:289–291 — assert union with fields; always has RNG sticky
	// no soft invent MakeInt(0) / "{}" without live RNG
	if r == nil || ut == nil || !ut.isUnion || len(ut.Fields) == 0 {
		SetError(ErrGeneric)
		return nil
	}
	f0 := ut.Fields[0]
	// Type* always live on Fields; Type-nil sticky (no invent soft-empty val then
	// ERROR_GUARD as complete first-field miss / soft re-pick past incomplete Type)
	if f0.Type == nil {
		SetError(ErrGeneric)
		return nil
	}
	var val string
	if f0.Type.IsStruct() {
		if c := MakeStructConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
	} else if f0.Type.IsUnion() {
		if c := MakeUnionConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
	} else {
		// Constant.cpp:292 — GenerateRandomConstant(fields[0]); no soft invent "0"
		if c := MakeRandom(f0.Type, opts, probs, r); c != nil {
			val = c.Value
		}
	}
	// ERROR_GUARD on empty first field sticky — no invent "{}"
	if val == "" {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}
	return &Constant{Type: ut, Value: "{" + val + "}"}
}
