// Upstream: Type.cpp make_random_struct_type / make_normal_struct_fields / GenerateAllTypes.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"math"
	"strconv"
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
	// Type.cpp:658–666 make_one_struct_field — rnd_upto(AllTypes, filter for_field_var)
	// without marking used (Type.cpp:1186–1190 used only in choose_random).
	var ft *Type
	if env != nil {
		ft = env.chooseRandomForStructField(r, opts, probs)
	}
	// Type.cpp:661 — ERROR_RETURN when AllTypes empty / choose fails; no soft invent simple
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
	// Type.cpp:1088–1091 make_random_struct_type — does not set used or record_type_with_bitfields.
	// used + Bookkeeper::record_type_with_bitfields only when first chosen (choose_random*, filters).
	// Type.cpp:298–302 — sid = sequence++ shared with unions; Output "struct S"<<sid.
	sid := 0
	name := tag
	if env != nil {
		sid = env.AggregateSeq
		env.AggregateSeq++
		name = fmt.Sprintf("S%d", sid)
	} else if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	st := &Type{
		isStruct:     true,
		StructName:   name,
		SID:          sid,
		Fields:       fields,
		Packed:       packed,
		Used:         false,
		HasAssignOps: hasAssign,
		// Type.cpp:1094–1096 — hasAssignOps || checkImplicitNontrivialAssignOps(fields)
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOps(opts, fields),
	}
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
	// Type.cpp:1170–1176 GenerateSimpleTypes — push eChar..eUInt128 always.
	// Float/int64/int128 gates are probability filters + ChooseRandomTypeFilter,
	// not AllTypes population (seed-2 first choose was n=13 vs upstream n=14).
	if len(env.AllTypes) == 0 {
		for st := EChar; int(st) < MaxSimpleTypes; st++ {
			env.AllTypes = append(env.AllTypes, GetSimpleType(st))
		}
	}
	// struct/union generation draws RNG + probs; no invent fixed S0 shells without them
	// Tag names come from Type.cpp shared sid sequence (env.AggregateSeq), not per-kind 0-based.
	if opts.Structs && r != nil && probs != nil {
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			// Type.cpp:1191–1193 — make_random_struct_type; sticky ERROR_RETURN aborts further
			if MakeRandomStructType(r, opts, probs, env, "") == nil || HasError() {
				break
			}
			if len(env.StructTypes) > 20 {
				break
			}
		}
	}
	if opts.Unions && r != nil && probs != nil {
		for MoreTypesProbability(r, probs, len(env.AllTypes)) {
			if MakeRandomUnionType(r, opts, probs, env, "") == nil || HasError() {
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
		// Type.cpp:1823–1829 OutputStructUnion — non-ccomp: pack(push) then pack(1).
		// ccomp: only pack(1). Defaults CComp=false (CGOptions::ccomp).
		if !ProcessOptions().CComp {
			b.WriteString("#pragma pack(push)\n")
		}
		b.WriteString("#pragma pack(1)\n")
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
			if f.Type == nil {
				// Type.cpp:1866 assert(eSimple) sticky; fail closed whole decl
				SetError(ErrGeneric)
				return ""
			}
			if !f.Type.IsSimple() {
				// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual
				if HasError() {
					return ""
				}
				// Type.cpp:1866 assert(eSimple) sticky; fail closed whole decl
				SetError(ErrGeneric)
				return ""
			}
			// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual true
			if HasError() {
				return ""
			}
			st := f.Type.Simple()
			// residual ERROR sticky — no invent soft-continue bitfield past Simple residual
			if HasError() {
				return ""
			}
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
	// Type.cpp:1877–1887 — after `;`: really_outputln; if packed then pack(pop|()) + ln; always extra blank.
	b.WriteString("\n")
	if t.Packed {
		// Type.cpp:1879–1883 — ccomp → pack(); else pack(pop)
		if ProcessOptions().CComp {
			b.WriteString("#pragma pack()\n")
		} else {
			b.WriteString("#pragma pack(pop)\n")
		}
	}
	// Type.cpp:1887 really_outputln after printed=true — blank line after each aggregate decl
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
	if r == nil || typ == nil {
		SetError(ErrGeneric)
		return ""
	}
	if !typ.IsSimple() {
		// residual ERROR sticky — no invent soft-empty range past IsSimple residual
		if HasError() {
			return ""
		}
		SetError(ErrGeneric)
		return ""
	}
	// residual ERROR sticky — no invent soft-continue range past IsSimple residual true
	if HasError() {
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
	// Constant.cpp:228 / 238 — int b = (int)pow(2, (double)bound / 2);
	// Floating exponent: bound=15 → 2^7.5 ≈ 181, not integer 1<<(15/2)=128
	// (seed-2 e424 was U181 vs U128).
	b := int(math.Pow(2, float64(bound)/2.0))
	// Constant.cpp:239–240 — eUInt: if (b < 0) b = INT_MAX
	if st == EUInt && b < 0 {
		b = int(^uint32(0) >> 1) // platform INT_MAX-ish for 32-bit cast
	}
	if b < 1 {
		// pure_rnd_upto domain; no invent b=1 soft-success past broken pow
		SetError(ErrGeneric)
		return ""
	}
	// pure_rnd_upto(b); C++ unsigned int domain (random mode == RndUpto)
	num := int(r.RndUpto(uint32(b)))
	// Constant.cpp:230–236 — eInt:
	//   flag = pure_rnd_flipcoin(50); if (flag) oss << num; else oss << "-" << num;
	// true → positive; false → negative (not the inverse).
	// Constant.cpp:241–243 — eUInt: non-negative only (no flip)
	var oss string
	if st == EInt {
		if r.RndFlipcoin(50) {
			oss = strconv.Itoa(num)
		} else {
			oss = "-" + strconv.Itoa(num)
		}
	} else {
		oss = strconv.Itoa(num)
	}
	// Constant.cpp:246 — no L/UL suffix here (only mark_mutable_const paren wrap)
	if opts.MarkMutableConst {
		return "(" + oss + ")"
	}
	return oss
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
			// residual ERROR sticky — no invent soft-field past range residual
			if HasError() {
				return nil
			}
		} else if f.Type.IsStruct() {
			// residual ERROR sticky — no invent soft-field past IsStruct residual true
			if HasError() {
				return nil
			}
			if c := MakeStructConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
			if HasError() {
				return nil
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-continue past IsStruct residual false
			return nil
		} else if f.Type.IsUnion() {
			// residual ERROR sticky — no invent soft-field past IsUnion residual true
			if HasError() {
				return nil
			}
			if c := MakeUnionConstant(r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
			if HasError() {
				return nil
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-continue past IsUnion residual false
			return nil
		} else {
			// Constant.cpp:271 — GenerateRandomConstant(fields[i]); no soft invent "0"
			if c := MakeRandom(f.Type, opts, probs, r); c != nil {
				val = c.Value
			}
			if HasError() {
				return nil
			}
		}
		// Constant.cpp ERROR_GUARD("") on empty field — sticky fail whole struct, no invent hole
		if val == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		// Constant.cpp:266–268 / 273–275 — value += "," (no invent ", " spaces)
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString(val)
	}
	b.WriteString("}")
	return &Constant{Type: st, Value: b.String()}
}

// MakeOneUnionField mirrors Type::make_one_union_field.
// Type.cpp:674–738 — bitfield optional; else non-pointer / no-bitfield structs;
// no union-in-union; pure_rnd 15% struct fields; SIMPLE_TYPES_PROB_FILTER at pick only.
func MakeOneUnionField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp always has RNG + Probabilities sticky; no invent field shell without them
	if r == nil || probs == nil {
		SetError(ErrGeneric)
		return StructField{}
	}
	// Type.cpp:677–680 — bitfield when bitfields && !ccomp && flipcoin(BitFieldInNormalStructProb)
	// bitfield path uses traced rnd_flipcoin (not pure_rnd)
	if opts.Bitfields && !opts.CComp && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
		if HasError() {
			return StructField{}
		}
		return MakeOneBitfield(r, opts, probs, fieldIdx, true)
	}
	if HasError() {
		return StructField{}
	}
	// Build ok_nonstruct_types and struct_types (Type.cpp:682–713)
	// C++ keeps weight-0 simples (float/void/…) in the nonstruct pool; filter only at pick
	// so pure_rnd_upto index space matches. Do not invent a trimmed pool.
	var nonStruct []*Type
	var structTypes []*Type
	if env != nil {
		for _, t := range env.AllTypes {
			// Type* always live on AllTypes; nil hole fails closed sticky (empty field shell)
			if t == nil {
				SetError(ErrGeneric)
				return StructField{}
			}
			// Type.cpp:691–692 — contain_pointer_field rejected (pointers + aggregates with ptr fields)
			if t.ContainPointerField() {
				if HasError() {
					return StructField{}
				}
				continue
			}
			isSt := t.IsStruct()
			if HasError() {
				return StructField{}
			}
			isUn := t.IsUnion()
			if HasError() {
				return StructField{}
			}
			if !isSt && !isUn {
				// Type.cpp:694–696 — all non-struct/non-union (simples + others) enter pool
				nonStruct = append(nonStruct, t)
				continue
			}
			// Type.cpp:701–702 — no bitfields in union members for now
			if t.HasBitfields() {
				if HasError() {
					return StructField{}
				}
				continue
			}
			// Type.cpp:705–706 — has_implicit_nontrivial_assign_ops()
			if t.HasImplicitNontrivialAssignOps {
				continue
			}
			if t.IsStruct() {
				structTypes = append(structTypes, t)
			}
			// Type.cpp:710–712 — no union in union
		}
	}
	// Type.cpp:714–730 — do {
	//   pure_rnd_flipcoin(15) ? struct_types[pure_rnd_upto] : nonstruct + SIMPLE filter
	// } while (type == nullptr)
	// random.cpp: pure_rnd_* == rnd_* when CGOptions::is_random() (defaults). Use session r
	// (same stream as ProcessRng during Generate; unit tests pass local Rng).
	var ft *Type
	for tries := 0; tries < 256; tries++ {
		if len(structTypes) > 0 && r.RndFlipcoin(15) {
			if HasError() {
				return StructField{}
			}
			ft = structTypes[r.RndUpto(uint32(len(structTypes)))]
			if HasError() {
				return StructField{}
			}
			break
		}
		if HasError() {
			return StructField{}
		}
		if len(nonStruct) == 0 {
			break
		}
		cand := nonStruct[r.RndUpto(uint32(len(nonStruct)))]
		if HasError() {
			return StructField{}
		}
		// Type.cpp:723–727 — SIMPLE_TYPES_PROB_FILTER reject (weight 0), retry; pool stays full
		if cand.IsSimple() {
			if HasError() {
				return StructField{}
			}
			if probs.SimpleTypeWeight(int(cand.Simple())) == 0 {
				if HasError() {
					return StructField{}
				}
				continue
			}
			if HasError() {
				return StructField{}
			}
		} else if HasError() {
			return StructField{}
		}
		ft = cand
		break
	}
	// Type.cpp:730 — while (type == nullptr); no soft invent simple when pools empty
	if ft == nil || HasError() {
		return StructField{}
	}
	// Type.cpp:733–735 — FieldConstProb / FieldVolatileProb (traced random_qualifiers)
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
	// Type.cpp:1110–1112 make_random_union_type — does not set used or record bitfields.
	// used + Bookkeeper only when first chosen (choose_random*, NonVoid* filters).
	// Type.cpp:298–302 — sid shared with structs; Output "union U"<<sid (not per-union 0-based).
	sid := 0
	name := tag
	if env != nil {
		sid = env.AggregateSeq
		env.AggregateSeq++
		name = fmt.Sprintf("U%d", sid)
	} else if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	ut := &Type{
		isUnion:      true,
		StructName:   name,
		SID:          sid,
		Fields:       fields,
		Used:         false,
		HasAssignOps: hasAssign,
		// Type.cpp:1146–1148 — hasAssignOps || checkImplicitNontrivialAssignOps
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOps(opts, fields),
	}
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
			if f.Type == nil {
				SetError(ErrGeneric)
				return ""
			}
			if !f.Type.IsSimple() {
				// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual
				if HasError() {
					return ""
				}
				SetError(ErrGeneric)
				return ""
			}
			// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual true
			if HasError() {
				return ""
			}
			st := f.Type.Simple()
			// residual ERROR sticky — no invent soft-continue bitfield past Simple residual
			if HasError() {
				return ""
			}
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
	// Type.cpp:1871–1887 — `};` + really_outputln; unions are not packed in make_random_union;
	// then really_outputln after printed (blank line after decl).
	b.WriteString("};\n\n")
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
		// residual ERROR sticky — no invent soft-union past IsStruct residual true
		if HasError() {
			return nil
		}
		if c := MakeStructConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
		if HasError() {
			return nil
		}
	} else if HasError() {
		return nil
	} else if f0.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-union past IsUnion residual true
		if HasError() {
			return nil
		}
		if c := MakeUnionConstant(r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
		if HasError() {
			return nil
		}
	} else if HasError() {
		return nil
	} else {
		// Constant.cpp:292 — GenerateRandomConstant(fields[0]); no soft invent "0"
		if c := MakeRandom(f0.Type, opts, probs, r); c != nil {
			val = c.Value
		}
		if HasError() {
			return nil
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
