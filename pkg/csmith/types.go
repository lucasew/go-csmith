// Upstream: Type.h / Type.cpp (eSimpleType, get_simple_type, choose_random_nonvoid_simple,
// match / is_convertable / is_signed / SizeInBytes / get_indirect_level).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// ESimpleType mirrors Type.h enum class eSimpleType (declaration order = integer value).
type ESimpleType int

const (
	EVoid ESimpleType = iota
	EChar
	EInt
	EShort
	ELong
	ELongLong
	EUChar
	EUInt
	EUShort
	EULong
	EFloat
	// eDouble omitted in upstream
	EULongLong
	EInt128
	EUInt128
)

// MaxSimpleTypes is Type.h MAX_SIMPLE_TYPES (eUInt128 + 1).
const MaxSimpleTypes = int(EUInt128) + 1

// MatchType mirrors Type.h enum class eMatchType.
type MatchType int

const (
	MatchExact MatchType = iota
	MatchConvert
	MatchDereference
	MatchDerefExact
	MatchFlexible
)

// Type is a minimal Type.cpp stand-in for simple + pointer types.
type Type struct {
	simple ESimpleType
	// ptrTo non-nil ⇒ pointer type (Type::ptr_type).
	ptrTo *Type
	// Struct/union aggregate.
	isStruct bool
	isUnion  bool
	// StructName is C tag, e.g. "S0" or "U0".
	StructName string
	// Fields are struct/union members (simple or nested struct).
	Fields []StructField
	// Packed mirrors Type::packed_ for structs.
	Packed bool
	// Used mirrors Type::used.
	Used bool
	// HasAssignOps mirrors Type::has_assign_ops_ (C++ struct/union assign ops).
	HasAssignOps bool
	// HasImplicitNontrivialAssignOps mirrors Type::has_implicit_nontrivial_assign_ops_
	// (true when this type or a nested field needs nontrivial assign ops).
	HasImplicitNontrivialAssignOps bool
}

// StructField is one struct member.
type StructField struct {
	Name string
	Type *Type
	Qfer CVQualifiers
	// BitWidth is bitfield width; -1 means not a bitfield (Type.cpp fields_length).
	BitWidth int
}

// simpleTypes mirrors Type::simple_types[MAX_SIMPLE_TYPES] cache.
var simpleTypes [MaxSimpleTypes]*Type

func init() {
	for i := 0; i < MaxSimpleTypes; i++ {
		simpleTypes[i] = &Type{simple: ESimpleType(i)}
	}
}

// GetSimpleType mirrors Type::get_simple_type — canonical cached values.
// Type.cpp get_simple_type indexes simple_types[MAX_SIMPLE_TYPES]; OOB is assert
// path — fail closed nil (no invent eInt for invalid eSimpleType).
func GetSimpleType(st ESimpleType) *Type {
	if st < 0 || int(st) >= MaxSimpleTypes {
		return nil
	}
	return simpleTypes[st]
}

// IsSimple reports eType == eSimple.
func (t *Type) IsSimple() bool {
	return t != nil && t.ptrTo == nil && !t.isStruct && !t.isUnion
}

// IsStruct reports eType == eStruct.
func (t *Type) IsStruct() bool {
	return t != nil && t.isStruct
}

// IsUnion reports eType == eUnion.
func (t *Type) IsUnion() bool {
	return t != nil && t.isUnion
}

// IsAggregate mirrors Type::is_aggregate (struct/union).
func (t *Type) IsAggregate() bool {
	return t != nil && (t.isStruct || t.isUnion)
}

// IsConstStructUnion mirrors Type::is_const_struct_union.
// Type.cpp:437–451 — any field const or nested const aggregate.
// Type* always live on Fields; nil hole fails closed as const (no invent non-const).
func (t *Type) IsConstStructUnion() bool {
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			return true
		}
		if f.Type.IsConstStructUnion() {
			return true
		}
		if f.Qfer.IsConst() {
			return true
		}
	}
	return false
}

// IsVolatileStructUnion mirrors Type::is_volatile_struct_union.
// Type.cpp:454+.
// Type* always live on Fields; nil hole fails closed as volatile (no invent non-vol).
func (t *Type) IsVolatileStructUnion() bool {
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			return true
		}
		if f.Type.IsVolatileStructUnion() {
			return true
		}
		if f.Qfer.IsVolatile() {
			return true
		}
	}
	return false
}

// incompleteStructDepth is returned when Fields contain a nil Type hole.
// Large enough that MaxNestedStructLevel filters reject the type (no invent
// depth-0 allowing deeper nesting), but bounded so bookkeeper counters stay small.
const incompleteStructDepth = 256

// StructDepth mirrors Type::get_struct_depth.
// Type.cpp:1261–1275 — 0 if not struct; else 1 + max field depth.
// Type* always live on Fields; nil hole fails closed as incompleteStructDepth
// (no invent depth 0 that soft-skips nested-struct caps).
func (t *Type) StructDepth() int {
	if t == nil || !t.IsStruct() {
		return 0
	}
	depth := 1
	maxField := 0
	for _, f := range t.Fields {
		if f.Type == nil {
			return incompleteStructDepth
		}
		if d := f.Type.StructDepth(); d > maxField {
			maxField = d
		}
	}
	return depth + maxField
}

// Simple returns the eSimpleType (only meaningful if IsSimple).
func (t *Type) Simple() ESimpleType {
	if t == nil {
		return EVoid
	}
	return t.simple
}

// IsNonVoidSimple is true for non-void simple types.
func (t *Type) IsNonVoidSimple() bool {
	return t != nil && t.IsSimple() && t.simple != EVoid
}

// IsInt mirrors Type::is_int — non-void simple (integer-ish including float in upstream).
// Type.h:291–293.
func (t *Type) IsInt() bool {
	return t.IsNonVoidSimple()
}

// IsPointerLike is true for ePointer types (ptrTo set).
func (t *Type) IsPointerLike() bool {
	return t != nil && t.ptrTo != nil
}

// HasIntField mirrors Type::has_int_field.
// Type.cpp:471–480 — self is int or any field has_int_field.
// HasIntField mirrors Type::has_int_field.
// Type* always live on Fields; nil hole fails closed as false (no invent has-int).
func (t *Type) HasIntField() bool {
	if t == nil {
		return false
	}
	if t.IsInt() {
		return true
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			return false
		}
		if f.Type.HasIntField() {
			return true
		}
	}
	return false
}

// ContainPointerField mirrors Type::contain_pointer_field.
// Type.cpp:1664–1674 — ePointer, or any aggregate field that does.
// Type* always live on Fields; nil hole fails closed as true (no invent pointer-free).
func (t *Type) ContainPointerField() bool {
	if t == nil {
		return false
	}
	if t.ptrTo != nil {
		return true
	}
	if t.IsAggregate() {
		for _, f := range t.Fields {
			if f.Type == nil {
				return true
			}
			if f.Type.ContainPointerField() {
				return true
			}
		}
	}
	return false
}

// IsFloat mirrors Type::is_float.
func (t *Type) IsFloat() bool {
	return t != nil && t.IsSimple() && t.simple == EFloat
}

// IsSignedChar mirrors Type::is_signed_char.
// Type.h:265–268 — eSimple && eChar (signed char).
func (t *Type) IsSignedChar() bool {
	return t != nil && t.IsSimple() && t.simple == EChar
}

// IsFullBitfieldsStruct mirrors Type::is_full_bitfields_struct.
// Type.cpp:1316–1324 — every field is a bitfield (BitWidth >= 0).
func (t *Type) IsFullBitfieldsStruct() bool {
	if t == nil || !t.IsStruct() || len(t.Fields) == 0 {
		return false
	}
	for _, f := range t.Fields {
		if f.BitWidth < 0 {
			return false
		}
	}
	return true
}

// IsSigned mirrors Type::is_signed (Type.cpp:1326–1347).
func (t *Type) IsSigned() bool {
	if t == nil || !t.IsSimple() {
		return false
	}
	switch t.simple {
	case EUChar, EUInt, EUShort, EULong, EULongLong, EUInt128:
		return false
	default:
		// signed integers, float, void → true in upstream default path
		return true
	}
}

// ToUnsigned mirrors Type::to_unsigned.
// Type.cpp:1349–1377 — map signed simple types to unsigned counterparts.
func (t *Type) ToUnsigned() *Type {
	if t == nil || !t.IsSimple() {
		return nil
	}
	switch t.simple {
	case EUChar, EUInt, EUShort, EULong, EULongLong, EUInt128:
		return t
	case EChar:
		return GetSimpleType(EUChar)
	case EInt:
		return GetSimpleType(EUInt)
	case EShort:
		return GetSimpleType(EUShort)
	case ELong:
		return GetSimpleType(EULong)
	case ELongLong:
		return GetSimpleType(EULongLong)
	case EInt128:
		// Type.cpp:1369–1372 — int128 stays as itself (and uint128)
		return GetSimpleType(EInt128)
	default:
		return nil
	}
}

// platform sizes from Options / platform.info (set by Generate via SetPlatformSizes).
var (
	platformIntSize = 4
	platformPtrSize = 8
)

// SetPlatformSizes mirrors host/platform.info integer and pointer sizes for SizeInBytes.
func SetPlatformSizes(intSize, ptrSize int) {
	if intSize > 0 {
		platformIntSize = intSize
	}
	if ptrSize > 0 {
		platformPtrSize = ptrSize
	}
}

// SizeInBytes mirrors Type::SizeInBytes for simple/pointer types (Type.cpp).
// Integer/pointer sizes come from platform (CGOptions / platform.info).
func (t *Type) SizeInBytes() int {
	if t == nil {
		return 0
	}
	if t.ptrTo != nil {
		return platformPtrSize
	}
	switch t.simple {
	case EVoid:
		return 0
	case EChar, EUChar:
		return 1
	case EShort, EUShort:
		return 2
	case EInt, EUInt:
		return platformIntSize
	case ELong, EULong:
		// LP64: long == pointer; ILP32: long == int
		if platformPtrSize > platformIntSize {
			return platformPtrSize
		}
		return platformIntSize
	case EFloat:
		return 4
	case ELongLong, EULongLong, EInt128, EUInt128:
		return 8
	default:
		// unknown simple — assert path sticky; no soft invent platform int size
		SetError(ErrGeneric)
		return 0
	}
}

// CName is a minimal C spelling for simple/pointer types.
func (t *Type) CName() string {
	// Type* always live at Output sites in C++; no soft invent "void" for nil
	if t == nil {
		return ""
	}
	if t.ptrTo != nil {
		inner := t.ptrTo.CName()
		if inner == "" {
			// incomplete pointee sticky — no invent bare "*"
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		return inner + "*"
	}
	if t.isStruct {
		// Type.cpp: eStruct → "struct S" + sid; sticky no invent bare "struct"
		if t.StructName == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "struct " + t.StructName
	}
	if t.isUnion {
		// Type.cpp: eUnion → "union U" + sid; sticky no invent bare "union"
		if t.StructName == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "union " + t.StructName
	}
	switch t.simple {
	case EVoid:
		return "void"
	case EChar:
		return "char"
	case EInt:
		return "int"
	case EShort:
		return "short"
	case ELong:
		return "long"
	case ELongLong:
		return "long long"
	case EUChar:
		return "unsigned char"
	case EUInt:
		return "unsigned int"
	case EUShort:
		return "unsigned short"
	case EULong:
		return "unsigned long"
	case EFloat:
		return "float"
	case EULongLong:
		return "unsigned long long"
	case EInt128:
		return "__int128"
	case EUInt128:
		return "unsigned __int128"
	default:
		// unknown simple — assert path sticky; no soft invent "int"
		SetError(ErrGeneric)
		return ""
	}
}

// PtrType returns Type::ptr_type (pointee), or nil if not a pointer.
func (t *Type) PtrType() *Type {
	if t == nil {
		return nil
	}
	return t.ptrTo
}

// IndirectLevel mirrors Type::get_indirect_level.
func (t *Type) IndirectLevel() int {
	n := 0
	for cur := t; cur != nil && cur.ptrTo != nil; cur = cur.ptrTo {
		n++
	}
	return n
}

// pointerCache keys by pointee pointer for stable eExact match on pointers.
// Mirrors Type::derived_types pointer entries (Type.cpp find_pointer_type).
var pointerCache = map[*Type]*Type{}

// TypeDoFinalization mirrors Type::doFinalization for process-wide derived types.
// Type.cpp:1962–1971 — clears derived_types (Go: pointerCache).
// simpleTypes stay: permanent eSimple cache (C++ simple_types[] is not reallocated each run).
// Allocated struct/union *Type values are GC'd with their ProgramGenerator session.
func TypeDoFinalization() {
	pointerCache = map[*Type]*Type{}
}

// PointerTo builds/caches a pointer type (find_pointer_type-ish for one level).
// Type.cpp:423+ find_pointer_type — pointee Type* always live; no invent int* for nil.
func PointerTo(pointee *Type) *Type {
	if pointee == nil {
		return nil
	}
	if p, ok := pointerCache[pointee]; ok {
		return p
	}
	p := &Type{ptrTo: pointee}
	pointerCache[pointee] = p
	return p
}

// Match mirrors Type::match (Type.cpp:1475–1488).
func (t *Type) Match(other *Type, mt MatchType) bool {
	if t == nil || other == nil {
		return t == other
	}
	switch mt {
	case MatchExact:
		return t == other
	case MatchConvert:
		// Type::is_convertable reads CGOptions::strict_float / lang_cpp
		return t.IsConvertableOpts(other, ProcessOptions())
	case MatchDereference:
		return t.IsDereferencedFrom(other)
	case MatchDerefExact:
		return other == t || t.IsDereferencedFrom(other)
	case MatchFlexible:
		return t.IsDerivable(other)
	default:
		return false
	}
}

// IsPromotable mirrors Type::is_promotable.
// Type.cpp:1387–1416 — integer rank promotion among simples.
func (t *Type) IsPromotable(other *Type) bool {
	if t == nil || other == nil || !t.IsSimple() || !other.IsSimple() {
		return false
	}
	switch t.simple {
	case EChar, EUChar:
		return other.simple != EVoid
	case EShort, EUShort:
		return other.simple != EVoid && other.simple != EChar && other.simple != EUChar
	case EInt, EUInt:
		return other.simple != EVoid && other.simple != EChar && other.simple != EUChar &&
			other.simple != EShort && other.simple != EUShort
	case ELong, EULong:
		return other.simple == ELong || other.simple == EULong ||
			other.simple == ELongLong || other.simple == EULongLong
	case ELongLong, EULongLong:
		return other.simple == ELongLong || other.simple == EULongLong
	case EFloat:
		return other.simple != EVoid
	default:
		return false
	}
}

// IsConvertable mirrors Type::is_convertable (simple + pointer size rules).
// Type.cpp:1423–1455 — float→int forbidden; pointer same size unless strict_float/lang_cpp.
// Uses process CGOptions; no soft invent Defaults().
func (t *Type) IsConvertable(other *Type) bool {
	return t.IsConvertableOpts(other, ProcessOptions())
}

// IsConvertableOpts applies CGOptions::strict_float / lang_cpp pointer rules.
func (t *Type) IsConvertableOpts(other *Type, opts Options) bool {
	if t == nil || other == nil {
		return false
	}
	if t == other {
		return true
	}
	if t.IsSimple() && other.IsSimple() {
		// forbidden conversion from float to int (Type.cpp:1428–1429)
		if other.IsFloat() && !t.IsFloat() {
			return false
		}
		if (t.simple != EVoid && other.simple != EVoid) || t.simple == other.simple {
			return true
		}
		return false
	}
	if t.ptrTo != nil && other.ptrTo != nil {
		if t.ptrTo == other.ptrTo {
			return true
		}
		if t.ptrTo.IsSimple() && other.ptrTo.IsSimple() {
			if t.ptrTo.simple == other.ptrTo.simple {
				return true
			}
			// Type.cpp:1439–1449
			if opts.StrictFloat &&
				((t.ptrTo.IsFloat() && !other.ptrTo.IsFloat()) ||
					(!t.ptrTo.IsFloat() && other.ptrTo.IsFloat())) {
				return false
			}
			if opts.LangCPP {
				return false
			}
			return t.ptrTo.SizeInBytes() == other.ptrTo.SizeInBytes()
		}
	}
	return false
}

// IsDereferencedFrom mirrors Type::is_dereferenced_from.
func (t *Type) IsDereferencedFrom(other *Type) bool {
	if other == nil || other.ptrTo == nil {
		return false
	}
	for pt := other.ptrTo; pt != nil; pt = pt.ptrTo {
		if pt == t {
			return true
		}
	}
	return false
}

// IsDerivable mirrors Type::is_derivable.
func (t *Type) IsDerivable(other *Type) bool {
	if t == other {
		return true
	}
	return t.IsConvertable(other) || t.IsDereferencedFrom(other) || t.ptrTo == other
}

// IsEquivalent mirrors Type::is_equivalent — same size and signedness for simples.
// Type.cpp:1455–1464.
func (t *Type) IsEquivalent(other *Type) bool {
	if t == nil || other == nil {
		return t == other
	}
	if t == other {
		return true
	}
	if t.IsSimple() && other.IsSimple() {
		return t.IsSigned() == other.IsSigned() && t.SizeInBytes() == other.SizeInBytes()
	}
	return false
}

// BaseType walks pointers to the ultimate pointee (Type::get_base_type).
func (t *Type) BaseType() *Type {
	for t != nil && t.ptrTo != nil {
		t = t.ptrTo
	}
	return t
}

// NeedsCast mirrors Type::needs_cast.
// Type.cpp:1470–1473 — this is pointer and base_type not equivalent to other's base.
// `this` is the expression's type; `other` is the desired cast target type.
func (t *Type) NeedsCast(other *Type) bool {
	if t == nil || other == nil {
		return false
	}
	if t.PtrType() == nil {
		return false
	}
	tb, ob := t.BaseType(), other.BaseType()
	if tb == nil || ob == nil {
		return true
	}
	return !tb.IsEquivalent(ob)
}

// HasBitfields mirrors Type::has_bitfields.
// Type.cpp:1290–1301.
// HasBitfields mirrors Type::has_bitfields.
// Type* always live on Fields; nil hole fails closed as true (no invent bitfield-free).
func (t *Type) HasBitfields() bool {
	if t == nil {
		return false
	}
	for _, f := range t.Fields {
		if f.BitWidth >= 0 {
			return true
		}
		if f.Type == nil {
			return true
		}
		if f.Type.IsStruct() && f.Type.HasBitfields() {
			return true
		}
	}
	return false
}

// IsBitfieldIndex mirrors Type::is_bitfield(index).
// Type.cpp:1286–1288 — assert(index < bitfields_length_.size()); BitWidth >= 0.
func (t *Type) IsBitfieldIndex(index int) bool {
	// OOB is assert path — fail closed false (not invent non-bitfield)
	if t == nil || index < 0 || index >= len(t.Fields) {
		return false
	}
	return t.Fields[index].BitWidth >= 0
}

// IsUnamedPadding mirrors Type::is_unamed_padding.
// Type.cpp:1278–1283 — assert(index < sz); bitfields_length_[index] == 0.
func (t *Type) IsUnamedPadding(index int) bool {
	if t == nil || index < 0 || index >= len(t.Fields) {
		return false
	}
	// only bitfield slots can be zero-width padding
	return t.Fields[index].BitWidth == 0
}

// HasPadding mirrors Type::has_padding.
// Type.cpp:1305–1314 — unpacked struct, bitfield member, or nested padding.
func (t *Type) HasPadding() bool {
	if t == nil {
		return false
	}
	if t.IsStruct() && !t.Packed {
		return true
	}
	for i, f := range t.Fields {
		if t.IsBitfieldIndex(i) {
			return true
		}
		if f.Type != nil && f.Type.HasPadding() {
			return true
		}
	}
	return false
}

// ChooseRandomNonvoidSimple mirrors Type::choose_random_nonvoid_simple.
// Type.cpp:618–635 — rnd_upto(MAX_SIMPLE_TYPES, SIMPLE_TYPES_PROB_FILTER); no soft invent eInt.
// Type.cpp:1246 — assert(ty != eVoid) on choose_random_simple; filter zeros void weight.
func ChooseRandomNonvoidSimple(r *Rng, probs *Probabilities) ESimpleType {
	// C++ always has RNG + probs; no soft invent EInt when missing
	if r == nil || probs == nil {
		return EVoid
	}
	v := r.RndUptoFilter(uint32(MaxSimpleTypes), probs.SimpleTypesFilter())
	return ESimpleType(v)
}

// GetIntType mirrors get_int_type() → eInt.
// Type.cpp:408.
func GetIntType() *Type { return GetSimpleType(EInt) }

// SignedOverflowPossible mirrors Type::signed_overflow_possible.
// Type.cpp:482–484 — signed simple with size >= int_size.
func (t *Type) SignedOverflowPossible(intSize int) bool {
	if t == nil || !t.IsSimple() || !t.IsSigned() {
		return false
	}
	// CGOptions::int_size always positive; no invent platform size when arg is 0
	if intSize < 1 {
		return false
	}
	return t.SizeInBytes() >= intSize
}

// GetTypeFromString mirrors Type::get_type_from_string.
// Type.cpp:370–402 — builtin name → simple type.
func GetTypeFromString(typeString string) *Type {
	switch typeString {
	case "Void":
		return GetSimpleType(EVoid)
	case "Char":
		return GetSimpleType(EChar)
	case "UChar":
		return GetSimpleType(EUChar)
	case "Short":
		return GetSimpleType(EShort)
	case "UShort":
		return GetSimpleType(EUShort)
	case "Int":
		return GetSimpleType(EInt)
	case "UInt":
		return GetSimpleType(EUInt)
	case "Long":
		return GetSimpleType(ELong)
	case "ULong":
		return GetSimpleType(EULong)
	case "Longlong", "LongLong":
		return GetSimpleType(ELongLong)
	case "ULonglong", "ULongLong":
		return GetSimpleType(EULongLong)
	case "Float":
		return GetSimpleType(EFloat)
	case "Int128":
		return GetSimpleType(EInt128)
	case "UInt128":
		return GetSimpleType(EUInt128)
	default:
		return nil
	}
}

// TypeNameString is the reverse of get_type_from_string for simple types.
func (t *Type) TypeNameString() string {
	if t == nil {
		return ""
	}
	if t.ptrTo != nil {
		return "Pointer"
	}
	if t.IsStruct() {
		return t.StructName
	}
	if t.IsUnion() {
		return t.StructName
	}
	switch t.simple {
	case EVoid:
		return "Void"
	case EChar:
		return "Char"
	case EUChar:
		return "UChar"
	case EShort:
		return "Short"
	case EUShort:
		return "UShort"
	case EInt:
		return "Int"
	case EUInt:
		return "UInt"
	case ELong:
		return "Long"
	case EULong:
		return "ULong"
	case ELongLong:
		return "Longlong"
	case EULongLong:
		return "ULonglong"
	case EFloat:
		return "Float"
	case EInt128:
		return "Int128"
	case EUInt128:
		return "UInt128"
	default:
		return ""
	}
}

// PrintfDirective mirrors Type::printf_directive.
// Type.cpp:1932–1957.
func (t *Type) PrintfDirective() string {
	if t == nil {
		return ""
	}
	if t.ptrTo != nil {
		return "0x%0x"
	}
	if t.IsAggregate() {
		// Type.cpp:1945–1951 — fields[i]->printf_directive always live Type*
		// no invent empty holes for nil field types
		var b strings.Builder
		b.WriteString("{")
		for i, f := range t.Fields {
			if f.Type == nil {
				return ""
			}
			part := f.Type.PrintfDirective()
			if part == "" {
				return ""
			}
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(part)
		}
		b.WriteString("}")
		return b.String()
	}
	if t.IsSimple() {
		if t.SizeInBytes() >= 8 {
			if t.IsSigned() {
				return "%lld"
			}
			return "%llu"
		}
		if t.IsSigned() {
			return "%d"
		}
		return "%u"
	}
	return ""
}

// SizeofString mirrors Type::get_type_sizeof_string.
// Type.cpp:1708–1714 — Output on live Type*; no invent sizeof(void)/sizeof().
func (t *Type) SizeofString() string {
	if t == nil {
		return ""
	}
	cn := t.CName()
	if cn == "" {
		return ""
	}
	return "sizeof(" + cn + ")"
}

// HasAggregateField mirrors Type::has_aggregate_field.
// Type.cpp:1057–1064.
func HasAggregateField(fields []StructField) bool {
	for _, f := range fields {
		if f.Type != nil && f.Type.IsAggregate() {
			return true
		}
	}
	return false
}

// HasLongLongField mirrors Type::has_longlong_field.
// Type.cpp:1066–1073.
// Type* always live on Fields; nil hole fails closed as true (no invent none).
func HasLongLongField(fields []StructField) bool {
	for _, f := range fields {
		if f.Type == nil {
			return true
		}
		if f.Type.IsSimple() && (f.Type.simple == ELongLong || f.Type.simple == EULongLong) {
			return true
		}
		if f.Type.IsAggregate() && HasLongLongField(f.Type.Fields) {
			return true
		}
	}
	return false
}

// IsUnnamedPadding mirrors Type::is_unamed_padding.
// Type.cpp:1280+ — zero-width bitfield without a name used as padding.
// Our StructField always has names; treat BitWidth==0 as unnamed padding candidate.
func (t *Type) IsUnnamedPadding(index int) bool {
	if t == nil || index < 0 || index >= len(t.Fields) {
		return false
	}
	f := t.Fields[index]
	// zero-length bitfield is padding
	return f.BitWidth == 0
}

// IfStructWillHaveAssignOps mirrors Type::if_struct_will_have_assign_ops.
// Type.cpp:505–510 — C++ only; flipcoin RegularVolatileProb.
func IfStructWillHaveAssignOps(r *Rng, opts Options, probs *Probabilities) bool {
	if !opts.LangCPP {
		return false
	}
	if r == nil {
		return false
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.Single(PRegularVolatileProb)
	}
	return r.RndFlipcoin(uint32(p))
}

// IfUnionWillHaveAssignOps mirrors Type::if_union_will_have_assign_ops.
// Type.cpp:514–519.
func IfUnionWillHaveAssignOps(r *Rng, opts Options, probs *Probabilities) bool {
	return IfStructWillHaveAssignOps(r, opts, probs)
}
