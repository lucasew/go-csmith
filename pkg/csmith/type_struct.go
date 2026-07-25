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
	return MoreTypesProbabilitySess(testAmbientSession, r, probs, typeCount)
}

// MoreTypesProbabilitySess is MoreTypesProbability with explicit session residual sticky.
func MoreTypesProbabilitySess(s *Session, r *Rng, probs *Probabilities, typeCount int) bool {
	if typeCount < 10 {
		return true
	}
	// C++ always has RNG; sticky no invent always-true past threshold when r nil
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.Single(PMoreStructUnionProb)
	}
	return r.RndFlipcoinSess(s, uint32(p))
}

// MakeOneStructField mirrors Type::make_one_struct_field.
// Type.cpp:683–697 + ChooseRandomTypeFilter depth gate (Type.cpp:240–242).
// Nested prior structs allowed when StructDepth < MaxNestedStructLevel.
// On ERROR_RETURN / choose fail returns zero field (Type==nil); callers abort.
func MakeOneStructField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int) StructField {
	// Type.cpp always has RNG + Probabilities sticky; no invent field shell without them
	if r == nil || probs == nil {
		noteErrEnv(env, ErrGeneric)
		return StructField{}
	}
	// Type.cpp:658–666 make_one_struct_field — rnd_upto(AllTypes, filter for_field_var)
	// without marking used (Type.cpp:1186–1190 used only in choose_random).
	var ft *Type
	if env != nil {
		ft = env.chooseRandomForStructField(r, opts, probs)
	}
	// Type.cpp:661 — ERROR_RETURN when AllTypes empty / choose fails; no soft invent simple
	if ft == nil || hasErrEnv(env) {
		return StructField{}
	}
	// Type.cpp:692–694 — FieldConstProb / FieldVolatileProb random_qualifiers
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext().WithSession(sessFromEnv(env)), false, constP, volP, opts, r)
	// Type.cpp:694 ERROR_RETURN after random_qualifiers
	if hasErrEnv(env) {
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
	return MakeOneBitfieldSess(testAmbientSession, r, opts, probs, fieldIdx, prevZero)
}

func MakeOneBitfieldSess(s *Session, r *Rng, opts Options, probs *Probabilities, fieldIdx int, prevZero bool) StructField {
	// Type.cpp:641 — CGOptions::int_size()*8; no soft invent 32 when size is 0
	// empty StructField uses BitWidth -1 (not a bitfield) so callers can detect fail
	fail := StructField{BitWidth: -1}
	maxLen := opts.IntSize * 8
	if maxLen < 1 {
		// broken options IR sticky — empty field (no invent maxLen=32)
		sessNoteError(s, ErrGeneric)
		return fail
	}
	if r == nil || probs == nil {
		sessNoteError(s, ErrGeneric)
		return fail
	}
	sign := r.RndFlipcoin(uint32(probs.Single(PBitFieldsSignedProb)))
	if sessHasError(s) {
		return fail
	}
	var ft *Type
	if sign {
		ft = GetIntTypeSess(s)
	} else {
		ft = GetSimpleTypeSess(s, EUInt)
	}
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext().WithSession(s), false, constP, volP, opts, r)
	if sessHasError(s) {
		return fail
	}
	length := int(r.RndUpto(uint32(maxLen)))
	if sessHasError(s) {
		return fail
	}
	// force non-zero if first field or previous was zero-length
	if length == 0 && prevZero {
		if maxLen <= 2 {
			length = 1
		} else {
			length = int(r.RndUpto(uint32(maxLen-1))) + 1
			if sessHasError(s) {
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
// Type.cpp:1075–1130 — BitFieldsCreationProb chooses full-bitfields vs normal fields.}

func MakeRandomStructType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, tag string) *Type {
	// Type.cpp always has process RNG; sticky no invent struct type without it
	if r == nil {
		noteErrEnv(env, ErrGeneric)
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
		noteErrEnv(env, ErrGeneric)
		return nil
	}
	// Type.cpp:1082 — ERROR_GUARD(nullptr) after field_cnt draw
	if hasErrEnv(env) {
		return nil
	}
	// is_bitfields = bitfields && flipcoin(BitFieldsCreationProb)
	// Type.cpp:1086–1088 — ERROR_GUARD after flip
	fullBitfields := opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldsCreationProb)))
	if hasErrEnv(env) {
		return nil
	}
	fields := make([]StructField, 0, fieldCnt)
	prevZero := true // first field cannot be zero-width
	for i := 0; i < fieldCnt; i++ {
		var f StructField
		if fullBitfields {
			// make_full_bitfields_struct_fields: ScalarFieldInFullBitFieldsProb → normal else bitfield
			if r.RndFlipcoin(uint32(probs.Single(PScalarFieldInFullBitFieldsProb))) {
				if hasErrEnv(env) {
					return nil
				}
				f = MakeOneStructField(r, opts, probs, env, i)
				prevZero = false
			} else {
				if hasErrEnv(env) {
					return nil
				}
				f = MakeOneBitfieldSess(sessFromEnv(env), r, opts, probs, i, prevZero)
				prevZero = f.Type != nil && f.BitWidth == 0
			}
		} else if opts.Bitfields && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
			// make_normal_struct_fields: BitFieldInNormalStructProb → bitfield
			if hasErrEnv(env) {
				return nil
			}
			f = MakeOneBitfieldSess(sessFromEnv(env), r, opts, probs, i, prevZero)
			prevZero = f.Type != nil && f.BitWidth == 0
		} else {
			if hasErrEnv(env) {
				return nil
			}
			f = MakeOneStructField(r, opts, probs, env, i)
			prevZero = false
		}
		// Type.cpp:1090 ERROR_GUARD after make_*_struct_fields; no soft invent nil-type field
		if f.Type == nil || hasErrEnv(env) {
			return nil
		}
		fields = append(fields, f)
	}
	// Type.cpp:1100–1110 — packed_struct; ccomp skips when aggregate/longlong fields
	packed := false
	if opts.PackedStruct {
		if opts.CComp && (HasAggregateFieldSess(sessFromEnv(env), fields) || HasLongLongFieldSess(sessFromEnv(env), fields)) {
			// leave packed false
		} else {
			packed = r.RndFlipcoin(50)
			if hasErrEnv(env) {
				return nil
			}
		}
	}
	hasAssign := IfStructWillHaveAssignOpsSess(sessFromEnv(env), r, opts, probs)
	if hasErrEnv(env) {
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
		noteErrEnv(env, ErrGeneric)
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
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOpsSess(sessFromEnv(env), opts, fields),
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
	return CheckImplicitNontrivialAssignOpsSess(testAmbientSession, opts, fields)
}

// CheckImplicitNontrivialAssignOpsSess is CheckImplicitNontrivialAssignOps with
// explicit session residual sticky.
func CheckImplicitNontrivialAssignOpsSess(s *Session, opts Options, fields []StructField) bool {
	if !opts.LangCPP {
		return false
	}
	for _, f := range fields {
		if f.Type == nil {
			// incomplete field Type sticky has-nontrivial (restrictive)
			sessNoteError(s, ErrGeneric)
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
		// no TypeEnv bag — sticky on unit-test ambient (cannot envSess(nil))
		sessNoteError(testAmbientSession, ErrGeneric)
		return
	}
	// No ambient install — unit tests set TypeEnv.Sess; Generate sets Types.Sess.
	if env.Sess == nil {
		panic("GenerateAllTypesEnv: Sess unset (set TypeEnv.Sess)")
	}
	// Type.cpp:1170–1176 GenerateSimpleTypes — push eChar..eUInt128 always.
	// Float/int64/int128 gates are probability filters + ChooseRandomTypeFilter,
	// not AllTypes population (seed-2 first choose was n=13 vs upstream n=14).
	if len(env.AllTypes) == 0 {
		for st := EChar; int(st) < MaxSimpleTypes; st++ {
			env.AllTypes = append(env.AllTypes, GetSimpleTypeSess(sessFromEnv(env), st))
		}
	}
	// struct/union generation draws RNG + probs; no invent fixed S0 shells without them
	// Tag names come from Type.cpp shared sid sequence (env.AggregateSeq), not per-kind 0-based.
	if opts.Structs && r != nil && probs != nil {
		for MoreTypesProbabilitySess(sessFromEnv(env), r, probs, len(env.AllTypes)) {
			// Type.cpp:1191–1193 — make_random_struct_type; sticky ERROR_RETURN aborts further
			if MakeRandomStructType(r, opts, probs, env, "") == nil || hasErrEnv(env) {
				break
			}
			if len(env.StructTypes) > 20 {
				break
			}
		}
	}
	if opts.Unions && r != nil && probs != nil {
		for MoreTypesProbabilitySess(sessFromEnv(env), r, probs, len(env.AllTypes)) {
			if MakeRandomUnionType(r, opts, probs, env, "") == nil || hasErrEnv(env) {
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
	return t.OutputStructDeclSess(testAmbientSession, nil, nil)
}

// OutputStructDeclSess is OutputStructDecl with Options/sticky from an explicit bag.
func (t *Type) OutputStructDeclSess(s *Session, r *Rng, attrs *AttributeGenerator) string {
	return t.OutputStructDeclWithSess(s, r, attrs, sessOpts(s))
}

// OutputStructDeclOpts optionally emits type attributes (Type.cpp type_attr_generator).
// Type.cpp:1836–1884 — OutputStructUnion field loop with bitfield asserts.
func (t *Type) OutputStructDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	return t.OutputStructDeclSess(testAmbientSession, r, attrs)
}

// OutputStructDeclWith is OutputStructDeclOpts with explicit session Options (ccomp pack).
func (t *Type) OutputStructDeclWith(r *Rng, attrs *AttributeGenerator, opts Options) string {
	return t.OutputStructDeclWithSess(testAmbientSession, r, attrs, opts)
}

func (t *Type) OutputStructDeclWithSess(s *Session, r *Rng, attrs *AttributeGenerator, opts Options) string {
	// Type* always live at struct emit; sticky no invent decl without it
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// non-struct: soft empty (callers use OutputUnionDecl for unions)
	if !t.isStruct {
		return ""
	}
	// Type.cpp always has sid name (S#); sticky no invent "struct  {"
	if t.StructName == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	if t.Packed {
		// Type.cpp:1823–1829 OutputStructUnion — non-ccomp: pack(push) then pack(1).
		// ccomp: only pack(1). Defaults CComp=false (CGOptions::ccomp).
		if !opts.CComp {
			b.WriteString("#pragma pack(push)\n")
		}
		b.WriteString("#pragma pack(1)\n")
	}
	b.WriteString("struct ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.OutputSess(s, r))
		// residual ERROR sticky — no invent soft-continue fields past attr residual
		if sessHasError(s) {
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
				sessNoteError(s, ErrGeneric)
				return ""
			}
			if !f.Type.IsSimpleSess(s) {
				// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual
				if sessHasError(s) {
					return ""
				}
				// Type.cpp:1866 assert(eSimple) sticky; fail closed whole decl
				sessNoteError(s, ErrGeneric)
				return ""
			}
			// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual true
			if sessHasError(s) {
				return ""
			}
			st := f.Type.SimpleSess(s)
			// residual ERROR sticky — no invent soft-continue bitfield past Simple residual
			if sessHasError(s) {
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
				sessNoteError(s, ErrGeneric)
				return ""
			}
			b.WriteString("   ")
			// Type.cpp:1867 — OutputFirstQuals
			if f.Qfer.IsConstSess(s) {
				// residual ERROR sticky — no invent soft-const past IsConst residual hole
				if sessHasError(s) {
					return ""
				}
				b.WriteString("const ")
			} else if sessHasError(s) {
				// residual ERROR sticky — no invent soft-continue field past IsConst residual false
				return ""
			}
			if f.Qfer.IsVolatileSess(s) {
				// residual ERROR sticky — no invent soft-vol past IsVolatile residual hole
				if sessHasError(s) {
					return ""
				}
				b.WriteString("volatile ")
			} else if sessHasError(s) {
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
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// Type.cpp:1879–1880 — output_qualified_type always live; sticky no invent " fN;"
		ty := f.Qfer.OutputQualifiedTypeOptsSess(s, f.Type, opts)
		// residual ERROR sticky — no invent soft-continue field past OutputQualifiedType residual
		if sessHasError(s) {
			return ""
		}
		if ty == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		b.WriteString("   ")
		b.WriteString(ty)
		// Type.cpp:1857–1858 — always " f"<<j++ (not stored field name / raw index).
		// make_one_struct_field labels fN with raw slot i; zero-width bitfields do not
		// advance j, so inventing emit from f.Name desyncs names after padding (seed 118:
		// GO f4/f5 vs UP f3/f4 after `volatile unsigned : 0`).
		b.WriteString(fmt.Sprintf(" f%d", j))
		b.WriteString(";\n")
		j++
	}
	b.WriteString("};")
	// Type.cpp:1877–1887 — after `;`: really_outputln; if packed then pack(pop|()) + ln; always extra blank.
	b.WriteString("\n")
	if t.Packed {
		// Type.cpp:1879–1883 — ccomp → pack(); else pack(pop)
		if opts.CComp {
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
	return GenerateRandomConstantInRangeSess(testAmbientSession, typ, bound, opts, r)
}

func GenerateRandomConstantInRangeSess(s *Session, typ *Type, bound int, opts Options, r *Rng) string {
	// Constant.cpp:222–246 — GenerateRandomConstantInRange
	// Constant.cpp:223 — assert(type->eType == eSimple)
	// Constant.cpp:226–245 — only eInt / eUInt; else assert(0)
	// sticky no invent empty/default constant past broken range IR
	if r == nil || typ == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !typ.IsSimpleSess(s) {
		// residual ERROR sticky — no invent soft-empty range past IsSimple residual
		if sessHasError(s) {
			return ""
		}
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// residual ERROR sticky — no invent soft-continue range past IsSimple residual true
	if sessHasError(s) {
		return ""
	}
	st := typ.SimpleSess(s)
	if st != EInt && st != EUInt {
		// assert(0) for other simples — sticky no soft invent generic decimal
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if bound <= 0 {
		// invalid bitfield width; sticky no invent "0" for broken range
		sessNoteError(s, ErrGeneric)
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
		sessNoteError(s, ErrGeneric)
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
// Constant.cpp:253–284 — skip zero-width bitfields; bitfields use in-range constants.}

func MakeStructConstant(r *Rng, opts Options, probs *Probabilities, st *Type) *Constant {
	return MakeStructConstantSess(testAmbientSession, r, opts, probs, st)
}

func MakeStructConstantSess(s *Session, r *Rng, opts Options, probs *Probabilities, st *Type) *Constant {
	// Constant.cpp:255 — assert(eStruct); always has RNG for field constants sticky
	// no invent "{}" shell without live RNG / fields path
	if r == nil || st == nil || !st.isStruct {
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return nil
		}
		var val string
		if f.BitWidth > 0 {
			// bitfield: GenerateRandomConstantInRange (eInt/eUInt only)
			val = GenerateRandomConstantInRangeSess(s, f.Type, f.BitWidth, opts, r)
			// residual ERROR sticky — no invent soft-field past range residual
			if sessHasError(s) {
				return nil
			}
		} else if f.Type.IsStructSess(s) {
			// residual ERROR sticky — no invent soft-field past IsStruct residual true
			if sessHasError(s) {
				return nil
			}
			if c := MakeStructConstantSess(s, r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
			if sessHasError(s) {
				return nil
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue past IsStruct residual false
			return nil
		} else if f.Type.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-field past IsUnion residual true
			if sessHasError(s) {
				return nil
			}
			if c := MakeUnionConstantSess(s, r, opts, probs, f.Type); c != nil {
				val = c.Value
			}
			if sessHasError(s) {
				return nil
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue past IsUnion residual false
			return nil
		} else {
			// Constant.cpp:271 — GenerateRandomConstant(fields[i]); no soft invent "0"
			if c := MakeRandomSess(s, f.Type, opts, probs, r); c != nil {
				val = c.Value
			}
			if sessHasError(s) {
				return nil
			}
		}
		// Constant.cpp ERROR_GUARD("") on empty field — sticky fail whole struct, no invent hole
		if val == "" {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
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
// prevZero mirrors Type.cpp:640 no_zero_len = fields_length.empty() || back()==0
// (non-bitfield pushes -1, so after a normal field zero-width bitfields are allowed).
// Invent always-true prevZero forced every union bitfield non-zero (seed 33: UP
// `const signed : 0` vs GO `const signed f3 : 2` after normal fields).}

func MakeOneUnionField(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, fieldIdx int, prevZero bool) StructField {
	// Type.cpp always has RNG + Probabilities sticky; no invent field shell without them
	if r == nil || probs == nil {
		noteErrEnv(env, ErrGeneric)
		return StructField{}
	}
	// Type.cpp:677–680 — bitfield when bitfields && !ccomp && flipcoin(BitFieldInNormalStructProb)
	// bitfield path uses traced rnd_flipcoin (not pure_rnd)
	if opts.Bitfields && !opts.CComp && r.RndFlipcoin(uint32(probs.Single(PBitFieldInNormalStructProb))) {
		if hasErrEnv(env) {
			return StructField{}
		}
		// Type.cpp:680 make_one_bitfield(fields, qfers, lens) — no_zero_len from lens
		return MakeOneBitfieldSess(sessFromEnv(env), r, opts, probs, fieldIdx, prevZero)
	}
	if hasErrEnv(env) {
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
				noteErrEnv(env, ErrGeneric)
				return StructField{}
			}
			// Type.cpp:691–692 — contain_pointer_field rejected (pointers + aggregates with ptr fields)
			if t.ContainPointerFieldSess(sessFromEnv(env)) {
				if hasErrEnv(env) {
					return StructField{}
				}
				continue
			}
			isSt := t.IsStructSess(sessFromEnv(env))
			if hasErrEnv(env) {
				return StructField{}
			}
			isUn := t.IsUnionSess(sessFromEnv(env))
			if hasErrEnv(env) {
				return StructField{}
			}
			if !isSt && !isUn {
				// Type.cpp:694–696 — all non-struct/non-union (simples + others) enter pool
				nonStruct = append(nonStruct, t)
				continue
			}
			// Type.cpp:701–702 — no bitfields in union members for now
			if t.HasBitfieldsSess(sessFromEnv(env)) {
				if hasErrEnv(env) {
					return StructField{}
				}
				continue
			}
			// Type.cpp:705–706 — has_implicit_nontrivial_assign_ops()
			if t.HasImplicitNontrivialAssignOps {
				continue
			}
			if t.IsStructSess(sessFromEnv(env)) {
				if hasErrEnv(env) {
					return StructField{}
				}
				structTypes = append(structTypes, t)
			} else if hasErrEnv(env) {
				return StructField{}
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
			if hasErrEnv(env) {
				return StructField{}
			}
			ft = structTypes[r.RndUpto(uint32(len(structTypes)))]
			if hasErrEnv(env) {
				return StructField{}
			}
			break
		}
		if hasErrEnv(env) {
			return StructField{}
		}
		if len(nonStruct) == 0 {
			break
		}
		cand := nonStruct[r.RndUpto(uint32(len(nonStruct)))]
		if hasErrEnv(env) {
			return StructField{}
		}
		// Type.cpp:723–727 — SIMPLE_TYPES_PROB_FILTER reject (weight 0), retry; pool stays full
		if cand.IsSimpleSess(sessFromEnv(env)) {
			if hasErrEnv(env) {
				return StructField{}
			}
			if probs.SimpleTypeWeight(int(cand.SimpleSess(sessFromEnv(env)))) == 0 {
				if hasErrEnv(env) {
					return StructField{}
				}
				continue
			}
			if hasErrEnv(env) {
				return StructField{}
			}
		} else if hasErrEnv(env) {
			return StructField{}
		}
		ft = cand
		break
	}
	// Type.cpp:730 — while (type == nullptr); no soft invent simple when pools empty
	if ft == nil || hasErrEnv(env) {
		return StructField{}
	}
	// Type.cpp:733–735 — FieldConstProb / FieldVolatileProb (traced random_qualifiers)
	constP := uint32(probs.Single(PFieldConstProb))
	volP := uint32(probs.Single(PFieldVolatileProb))
	q := RandomQualifiersForType(ft, AccessRead, EmptyCGContext().WithSession(sessFromEnv(env)), false, constP, volP, opts, r)
	if hasErrEnv(env) {
		return StructField{}
	}
	return StructField{Name: fmt.Sprintf("f%d", fieldIdx), Type: ft, Qfer: q, BitWidth: -1}
}

// MakeRandomUnionType mirrors Type::make_random_union_type.
// Type.cpp:1132–1150.
func MakeRandomUnionType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, tag string) *Type {
	// Type.cpp always has process RNG; sticky no invent union type without it
	if r == nil {
		noteErrEnv(env, ErrGeneric)
		return nil
	}
	// Type.cpp:1133–1135 — max_union_fields as-is; no soft invent maxCnt=1
	maxCnt := opts.MaxUnionFields
	if maxCnt < 0 {
		maxCnt = 0
	}
	fieldCnt := int(r.RndUpto(uint32(maxCnt))) + 1
	// Type.cpp:1136 — ERROR_GUARD after field_cnt
	if hasErrEnv(env) {
		return nil
	}
	if fieldCnt < 1 {
		// sticky no invent zero-field union shell
		noteErrEnv(env, ErrGeneric)
		return nil
	}
	fields := make([]StructField, 0, fieldCnt)
	// Type.cpp:640 — no_zero_len = fields_length.empty() || (fields_length.back() == 0)
	// empty → true; non-bitfield pushes -1 (back()!=0); pad pushes 0.
	prevZero := true
	for i := 0; i < fieldCnt; i++ {
		f := MakeOneUnionField(r, opts, probs, env, i, prevZero)
		// Type.cpp:1140–1141 — make_one_union_field; assert no bitfields on last
		// no soft invent nil-type union field
		if f.Type == nil || hasErrEnv(env) {
			return nil
		}
		if f.Type.HasBitfieldsSess(sessFromEnv(env)) {
			// residual ERROR sticky — no invent soft-skip assert fail past incomplete field Type
			if !hasErrEnv(env) {
				// C++ assert(!fields.back()->has_bitfields()) — complete bitfields still fail closed
				noteErrEnv(env, ErrGeneric)
			}
			return nil
		}
		// Type.cpp:649 fields_length.push_back(length); -1 for non-bitfield
		// next no_zero_len: only true when last length was 0 (pad)
		prevZero = f.BitWidth == 0
		fields = append(fields, f)
	}
	hasAssign := IfUnionWillHaveAssignOpsSess(sessFromEnv(env), r, opts, probs)
	if hasErrEnv(env) {
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
		noteErrEnv(env, ErrGeneric)
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
		HasImplicitNontrivialAssignOps: hasAssign || CheckImplicitNontrivialAssignOpsSess(sessFromEnv(env), opts, fields),
	}
	if env != nil {
		env.UnionTypes = append(env.UnionTypes, ut)
		env.AllTypes = append(env.AllTypes, ut)
	}
	return ut
}

// OutputUnionDecl emits a C union definition.
func (t *Type) OutputUnionDecl() string {
	return t.OutputUnionDeclSess(testAmbientSession, nil, nil)
}

// OutputUnionDeclSess is OutputUnionDecl with Options/sticky from an explicit bag.
func (t *Type) OutputUnionDeclSess(s *Session, r *Rng, attrs *AttributeGenerator) string {
	return t.OutputUnionDeclWithSess(s, r, attrs, sessOpts(s))
}

// OutputUnionDeclOpts optionally emits type attributes.
// Type.cpp:1836+ OutputStructUnion for unions (same field loop).
func (t *Type) OutputUnionDeclOpts(r *Rng, attrs *AttributeGenerator) string {
	return t.OutputUnionDeclSess(testAmbientSession, r, attrs)
}

// OutputUnionDeclWith is OutputUnionDeclOpts with explicit session Options.
func (t *Type) OutputUnionDeclWith(r *Rng, attrs *AttributeGenerator, opts Options) string {
	return t.OutputUnionDeclWithSess(testAmbientSession, r, attrs, opts)
}

func (t *Type) OutputUnionDeclWithSess(s *Session, r *Rng, attrs *AttributeGenerator, opts Options) string {
	// Type* always live at union emit; sticky no invent decl without it
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// non-union: soft empty
	if !t.isUnion {
		return ""
	}
	// Type.cpp always has sid name (U#); sticky no invent "union  {"
	if t.StructName == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString("union ")
	b.WriteString(t.StructName)
	if attrs != nil && r != nil {
		b.WriteString(attrs.OutputSess(s, r))
		// residual ERROR sticky — no invent soft-continue fields past attr residual
		if sessHasError(s) {
			return ""
		}
	}
	b.WriteString(" {\n")
	j := 0
	for _, f := range t.Fields {
		if f.BitWidth >= 0 {
			// unions rarely have bitfields; same assert rules as struct sticky
			if f.Type == nil {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			if !f.Type.IsSimpleSess(s) {
				// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual
				if sessHasError(s) {
					return ""
				}
				sessNoteError(s, ErrGeneric)
				return ""
			}
			// residual ERROR sticky — no invent soft-continue bitfield past IsSimple residual true
			if sessHasError(s) {
				return ""
			}
			st := f.Type.SimpleSess(s)
			// residual ERROR sticky — no invent soft-continue bitfield past Simple residual
			if sessHasError(s) {
				return ""
			}
			var signedKW string
			switch st {
			case EInt:
				signedKW = "signed"
			case EUInt:
				signedKW = "unsigned"
			default:
				sessNoteError(s, ErrGeneric)
				return ""
			}
			b.WriteString("   ")
			if f.Qfer.IsConstSess(s) {
				// residual ERROR sticky — no invent soft-const past IsConst residual hole
				if sessHasError(s) {
					return ""
				}
				b.WriteString("const ")
			} else if sessHasError(s) {
				// residual ERROR sticky — no invent soft-continue field past IsConst residual false
				return ""
			}
			if f.Qfer.IsVolatileSess(s) {
				// residual ERROR sticky — no invent soft-vol past IsVolatile residual hole
				if sessHasError(s) {
					return ""
				}
				b.WriteString("volatile ")
			} else if sessHasError(s) {
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
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// output_qualified_type always live sticky; no invent " fN;" without type
		ty := f.Qfer.OutputQualifiedTypeOptsSess(s, f.Type, opts)
		// residual ERROR sticky — no invent soft-continue field past OutputQualifiedType residual
		if sessHasError(s) {
			return ""
		}
		if ty == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		b.WriteString("   ")
		b.WriteString(ty)
		// Type.cpp:1857–1858 — always " f"<<j++ (not stored Name / raw creation index).
		// Same as OutputStructDeclOpts: skip j on zero-width padding bitfields.
		b.WriteString(fmt.Sprintf(" f%d", j))
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
	return MakeUnionConstantSess(testAmbientSession, r, opts, probs, ut)
}

func MakeUnionConstantSess(s *Session, r *Rng, opts Options, probs *Probabilities, ut *Type) *Constant {
	// Constant.cpp:289–291 — assert union with fields; always has RNG sticky
	// no soft invent MakeIntSess(sessFromEnv(env), 0) / "{}" without live RNG
	if r == nil || ut == nil || !ut.isUnion || len(ut.Fields) == 0 {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	f0 := ut.Fields[0]
	// Type* always live on Fields; Type-nil sticky (no invent soft-empty val then
	// ERROR_GUARD as complete first-field miss / soft re-pick past incomplete Type)
	if f0.Type == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	var val string
	if f0.Type.IsStructSess(s) {
		// residual ERROR sticky — no invent soft-union past IsStruct residual true
		if sessHasError(s) {
			return nil
		}
		if c := MakeStructConstantSess(s, r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
		if sessHasError(s) {
			return nil
		}
	} else if sessHasError(s) {
		return nil
	} else if f0.Type.IsUnionSess(s) {
		// residual ERROR sticky — no invent soft-union past IsUnion residual true
		if sessHasError(s) {
			return nil
		}
		if c := MakeUnionConstantSess(s, r, opts, probs, f0.Type); c != nil {
			val = c.Value
		}
		if sessHasError(s) {
			return nil
		}
	} else if sessHasError(s) {
		return nil
	} else {
		// Constant.cpp:292 — GenerateRandomConstant(fields[0]); no soft invent "0"
		if c := MakeRandomSess(s, f0.Type, opts, probs, r); c != nil {
			val = c.Value
		}
		if sessHasError(s) {
			return nil
		}
	}
	// ERROR_GUARD on empty first field sticky — no invent "{}"
	if val == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return nil
	}
	return &Constant{Type: ut, Value: "{" + val + "}"}
}
