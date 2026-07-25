// Upstream: Type.h / Type.cpp (eSimpleType, get_simple_type, choose_random_nonvoid_simple,
// match / is_convertable / is_signed / SizeInBytes / get_indirect_level).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

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
	// StructName is C tag, e.g. "S0" or "U1" (from Type::sid — shared struct/union sequence).
	StructName string
	// SID mirrors Type::sid for aggregates (shared sequence; not used for simples).
	SID int
	// Fields are struct/union members (simple or nested struct).
	Fields []StructField
	// Packed mirrors Type::packed_ for structs.
	Packed bool
	// Used mirrors Type::used.
	Used bool
	// Printed mirrors Type::printed — set when OutputStructUnion has emitted this type.
	Printed bool
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
		sessNoteError(nil, ErrGeneric)
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

// IntSubfield is one flattened simple leaf from Type::get_int_subfield_names.
// Type.cpp:1615–1636 — prefix ".f0.f1"; types parallel names.
type IntSubfield struct {
	Name string // e.g. ".f0.f0"
	Type *Type  // always simple
}

// GetIntSubfieldNames mirrors Type::get_int_subfield_names.
// Type.cpp:1615–1636 — recurse aggregates; skip 0-length bitfields; exclude
// union field indices (j after padding skip) when listed in excluded.
// Incomplete field Type sticky empty (no invent partial name list past hole).
func (t *Type) GetIntSubfieldNames(prefix string, excluded []int) []IntSubfield {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if t.IsSimple() {
		// residual ERROR sticky — no invent soft-leaf past IsSimple residual
		if sessHasError(nil) {
			return nil
		}
		return []IntSubfield{{Name: prefix, Type: t}}
	}
	// residual ERROR sticky — no invent soft-continue past IsSimple residual false
	if sessHasError(nil) {
		return nil
	}
	if !t.IsAggregate() {
		// residual ERROR sticky — no invent soft-empty past IsAggregate residual
		if sessHasError(nil) {
			return nil
		}
		return nil
	}
	// residual ERROR sticky — no invent soft-empty past IsAggregate residual true path
	if sessHasError(nil) {
		return nil
	}
	var out []IntSubfield
	j := 0
	for i := range t.Fields {
		f := &t.Fields[i]
		// Type.cpp:1623 — is_unamed_padding (0-length bitfield)
		if f.BitWidth == 0 {
			continue
		}
		// Type.cpp:1625–1629 — excluded by field index j (post-padding)
		skip := false
		for _, ex := range excluded {
			if ex == j {
				skip = true
				break
			}
		}
		if skip {
			j++
			continue
		}
		if f.Type == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		// Type.cpp:1631–1634 — prefix+".f"+j; recurse with empty excluded
		subPrefix := prefix + ".f" + itoa(j)
		j++
		nested := f.Type.GetIntSubfieldNames(subPrefix, nil)
		if sessHasError(nil) {
			return nil
		}
		out = append(out, nested...)
	}
	return out
}

// IsConstStructUnion mirrors Type::is_const_struct_union.
// Type.cpp:437–451 — any field const or nested const aggregate.
// Type* always live on Fields; nil hole sticky fail closed as const (no invent non-const).
func (t *Type) IsConstStructUnion() bool {
	// non-aggregate / nil Type is complete false (not const-struct)
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			// incomplete field Type sticky const (restrictive — no invent non-const)
			sessNoteError(nil, ErrGeneric)
			return true
		}
		if f.Type.IsConstStructUnion() {
			// residual ERROR sticky — no invent const-true past nested IsConstStructUnion hole
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual false
		if sessHasError(nil) {
			return true
		}
		if f.Qfer.IsConst() {
			// residual ERROR sticky — no invent const-true past field IsConst residual
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past IsConst residual false
		if sessHasError(nil) {
			return true
		}
	}
	return false
}

// IsVolatileStructUnion mirrors Type::is_volatile_struct_union.
// Type.cpp:454+.
// Type* always live on Fields; nil hole sticky fail closed as volatile (no invent non-vol).
func (t *Type) IsVolatileStructUnion() bool {
	// non-aggregate / nil Type is complete false (not vol-struct)
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			// incomplete field Type sticky volatile (restrictive — no invent non-vol)
			sessNoteError(nil, ErrGeneric)
			return true
		}
		if f.Type.IsVolatileStructUnion() {
			// residual ERROR sticky — no invent vol-true past nested IsVolatileStructUnion hole
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual false
		if sessHasError(nil) {
			return true
		}
		if f.Qfer.IsVolatile() {
			// residual ERROR sticky — no invent vol-true past field IsVolatile residual
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past IsVolatile residual false
		if sessHasError(nil) {
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
// Type* always live on Fields; nil hole sticky incompleteStructDepth
// (no invent depth 0 that soft-skips nested-struct caps / soft re-pick).
func (t *Type) StructDepth() int {
	if t == nil || !t.IsStruct() {
		return 0
	}
	depth := 1
	maxField := 0
	for _, f := range t.Fields {
		if f.Type == nil {
			// incomplete field Type sticky deep (restrictive nesting filter)
			sessNoteError(nil, ErrGeneric)
			return incompleteStructDepth
		}
		d := f.Type.StructDepth()
		// residual ERROR sticky — no invent soft-continue later fields past nested StructDepth hole
		if sessHasError(nil) {
			return incompleteStructDepth
		}
		if d > maxField {
			maxField = d
		}
	}
	return depth + maxField
}

// Simple returns the eSimpleType (only meaningful if IsSimple).
// Type* always live at Simple sites; sticky EVoid (no invent void soft-skip past hole).
func (t *Type) Simple() ESimpleType {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
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
// Type* always live; sticky false (no invent has-int soft success past hole).
// Type* always live on Fields; nil field hole fails closed as false (no invent has-int).
func (t *Type) HasIntField() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if t.IsInt() {
		// residual ERROR sticky — no invent has-int true past IsInt residual hole
		if sessHasError(nil) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue field scan past IsInt residual false
	if sessHasError(nil) {
		return false
	}
	for _, f := range t.Fields {
		if f.Type == nil {
			// incomplete field Type sticky not-has-int (no invent int field past hole)
			sessNoteError(nil, ErrGeneric)
			return false
		}
		if f.Type.HasIntField() {
			// residual ERROR sticky — no invent has-int true past nested HasIntField hole
			if sessHasError(nil) {
				return false
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual false
		if sessHasError(nil) {
			return false
		}
	}
	return false
}

// ContainPointerField mirrors Type::contain_pointer_field.
// Type.cpp:1664–1674 — ePointer, or any aggregate field that does.
// Type* always live; nil Type sticky true (no invent pointer-free soft-skip).
// Type* always live on Fields; nil field hole sticky true (no invent pointer-free).
func (t *Type) ContainPointerField() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if t.ptrTo != nil {
		return true
	}
	if t.IsAggregate() {
		// residual ERROR sticky — no invent soft-continue scan past IsAggregate residual
		if sessHasError(nil) {
			return true
		}
		for _, f := range t.Fields {
			if f.Type == nil {
				// incomplete field Type sticky has-pointer (restrictive)
				sessNoteError(nil, ErrGeneric)
				return true
			}
			if f.Type.ContainPointerField() {
				// residual ERROR sticky — no invent has-pointer true past nested hole
				if sessHasError(nil) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later fields past nested residual false
			if sessHasError(nil) {
				return true
			}
		}
	} else if sessHasError(nil) {
		// residual ERROR sticky — no invent pointer-free soft-skip past IsAggregate residual false
		return true
	}
	return false
}

// IsFloat mirrors Type::is_float.
// Type* always live; sticky false (no invent not-float soft-skip past hole).
func (t *Type) IsFloat() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !t.IsSimple() {
		// residual ERROR sticky — no invent not-float soft-skip past IsSimple residual
		if sessHasError(nil) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue float check past IsSimple residual true
	if sessHasError(nil) {
		return false
	}
	return t.simple == EFloat
}

// IsSignedChar mirrors Type::is_signed_char.
// Type.h:265–268 — eSimple && eChar (signed char).
// Type* always live; sticky false (no invent not-char soft-skip past hole).
func (t *Type) IsSignedChar() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !t.IsSimple() {
		// residual ERROR sticky — no invent not-char soft-skip past IsSimple residual
		if sessHasError(nil) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue char check past IsSimple residual true
	if sessHasError(nil) {
		return false
	}
	return t.simple == EChar
}

// IsFullBitfieldsStruct mirrors Type::is_full_bitfields_struct.
// Type.cpp:1316–1324 — every field is a bitfield (BitWidth >= 0).
// Type* always live; sticky true (no invent not-full-bitfields soft-skip past hole).
func (t *Type) IsFullBitfieldsStruct() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if !t.IsStruct() || len(t.Fields) == 0 {
		// residual ERROR sticky — no invent not-full soft-skip past IsStruct residual
		if sessHasError(nil) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue full check past IsStruct residual true
	if sessHasError(nil) {
		return true
	}
	for _, f := range t.Fields {
		if f.BitWidth < 0 {
			return false
		}
	}
	return true
}

// IsSigned mirrors Type::is_signed (Type.cpp:1326–1347).
// Type* always live; sticky true (no invent unsigned soft-skip past hole).
func (t *Type) IsSigned() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if !t.IsSimple() {
		// residual ERROR sticky — no invent unsigned soft-skip past IsSimple residual
		if sessHasError(nil) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue signed check past IsSimple residual true
	if sessHasError(nil) {
		return true
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
// Type* always live at to_unsigned; sticky nil (no invent soft-skip past hole).
func (t *Type) ToUnsigned() *Type {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !t.IsSimple() {
		// residual ERROR sticky — no invent soft-nil unsigned past IsSimple residual
		if sessHasError(nil) {
			return nil
		}
		return nil
	}
	// residual ERROR sticky — no invent soft-continue unsigned past IsSimple residual true
	if sessHasError(nil) {
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

// SetPlatformSizes mirrors host/platform.info integer and pointer sizes for SizeInBytes.
func SetPlatformSizes(intSize, ptrSize int) {
	SetPlatformSizesSess(nil, intSize, ptrSize)
}

// SetPlatformSizesSess sets platform sizes on an explicit session bag.
func SetPlatformSizesSess(s *Session, intSize, ptrSize int) {
	s = sessOrAmbient(s)
	if intSize > 0 {
		s.PlatformIntSize = intSize
	}
	if ptrSize > 0 {
		s.PlatformPtrSize = ptrSize
	}
}

// SizeInBytes mirrors Type::SizeInBytes for simple/pointer types.
// Type.cpp:1497–1572 — simple sizes are fixed (not host LP64 invent):
// eInt/eUInt/eLong/eULong = 4, eLongLong/eULongLong = 8, eInt128/eUInt128 = 16.
// Pointer: Type.cpp:1568–1572 calls CGOptions::pointer_size() then falls through
// to return 0 — fair port returns 0 (is_convertable compares pointee sizes via this).
// Type* always live at SizeInBytes sites; sticky 0 (no invent zero-size soft-skip).
func (t *Type) SizeInBytes() int {
	return t.SizeInBytesSess(nil)
}

// SizeInBytesSess is SizeInBytes with PlatformPtrSize read on bag s.
func (t *Type) SizeInBytesSess(s *Session) int {
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	if t.ptrTo != nil {
		// Type.cpp:1568–1572 — ePointer: pointer_size() side-effect then return 0
		_ = sessOrAmbient(s).PlatformPtrSize
		return 0
	}
	// Type.cpp:1502–1531 — fixed simple sizes (independent of host LP64)
	switch t.simple {
	case EVoid:
		return 0
	case EChar, EUChar:
		return 1
	case EShort, EUShort:
		return 2
	case EInt, EUInt:
		return 4
	case ELong, EULong:
		// Type.cpp:1511–1512 / 1521–1522 — always 4 (not invent LP64 long==8)
		return 4
	case EFloat:
		return 4
	case ELongLong, EULongLong:
		return 8
	case EInt128, EUInt128:
		// Type.cpp:1527–1530 — 16 bytes
		return 16
	default:
		// unknown simple — assert path sticky; no soft invent platform int size
		sessNoteError(s, ErrGeneric)
		return 0
	}
}

// CName mirrors Type::Output for simple/pointer/struct/union spellings.
// Type.cpp:1678–1706 — stdint-style intN_t/uintN_t from SizeInBytes*8.
func (t *Type) CName() string {
	// Type* always live at Output sites in C++; sticky no invent "void" for nil
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if t.ptrTo != nil {
		inner := t.ptrTo.CName()
		if inner == "" {
			// incomplete pointee sticky — no invent bare "*"
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		return inner + "*"
	}
	if t.isStruct {
		// Type.cpp: eStruct → "struct S" + sid; sticky no invent bare "struct"
		if t.StructName == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		return "struct " + t.StructName
	}
	if t.isUnion {
		// Type.cpp: eUnion → "union U" + sid; sticky no invent bare "union"
		if t.StructName == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		return "union " + t.StructName
	}
	// eSimple
	switch t.simple {
	case EVoid:
		return "void"
	case EFloat:
		return "float"
	case EInt128:
		// Type.cpp:1685–1686 — __int{SizeInBytes*8}
		sz := t.SizeInBytes()
		if sessHasError(nil) || sz <= 0 {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		return fmt.Sprintf("__int%d", sz*8)
	case EUInt128:
		sz := t.SizeInBytes()
		if sessHasError(nil) || sz <= 0 {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		return fmt.Sprintf("unsigned __int%d", sz*8)
	default:
		// Type.cpp:1689–1693 — (is_signed?"int":"uint") + bits + "_t"
		if t.simple == 0 && !t.IsSimple() {
			// unknown/non-simple fallthrough sticky
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// only integer simples reach here
		switch t.simple {
		case EChar, EShort, EInt, ELong, ELongLong, EUChar, EUShort, EUInt, EULong, EULongLong:
			// ok
		default:
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		sz := t.SizeInBytes()
		if sessHasError(nil) || sz <= 0 {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		signed := t.IsSigned()
		if sessHasError(nil) {
			return ""
		}
		bits := sz * 8
		if signed {
			return fmt.Sprintf("int%d_t", bits)
		}
		return fmt.Sprintf("uint%d_t", bits)
	}
}

// PtrType returns Type::ptr_type (pointee), or nil if not a pointer.
// Type* always live at ptr_type; sticky nil (no invent not-pointer soft-skip past hole).
func (t *Type) PtrType() *Type {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	return t.ptrTo
}

// IndirectLevel mirrors Type::get_indirect_level.
// Type* always live at get_indirect_level; sticky 0 (no invent level-0 non-deref
// soft-skip past missing Type shell / soft re-pick).
func (t *Type) IndirectLevel() int {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	n := 0
	for cur := t; cur != nil && cur.ptrTo != nil; cur = cur.ptrTo {
		n++
	}
	return n
}

// currentSession().PointerCache keys by pointee pointer for stable eExact match on pointers.
// Mirrors Type::derived_types pointer entries (Type.cpp find_pointer_type).
// TypeDoFinalization mirrors Type::doFinalization for session derived types.
// Type.cpp:1962–1971 — clears derived_types (Go: PointerCache).
// simpleTypes stay: permanent eSimple cache (C++ simple_types[] is not reallocated each run).
func TypeDoFinalization() {
	TypeDoFinalizationSess(nil)
}

// TypeDoFinalizationSess clears PointerCache on an explicit session bag.
func TypeDoFinalizationSess(s *Session) {
	sessOrAmbient(s).PointerCache = map[*Type]*Type{}
}

// PointerTo builds/caches a pointer type (find_pointer_type-ish for one level).
// Type.cpp:423+ find_pointer_type — pointee Type* always live; sticky no invent int* for nil.
func PointerTo(pointee *Type) *Type {
	return PointerToSess(nil, pointee)
}

// PointerToSess is PointerTo on an explicit session bag.
func PointerToSess(s *Session, pointee *Type) *Type {
	if pointee == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	s = sessOrAmbient(s)
	if p, ok := s.PointerCache[pointee]; ok {
		return p
	}
	p := &Type{ptrTo: pointee}
	s.PointerCache[pointee] = p
	return p
}

// Match mirrors Type::match (Type.cpp:1475–1488).
// Type* always live on both sides; sticky false (no invent equal/not-match soft-skip past hole).
func (t *Type) Match(other *Type, mt MatchType) bool {
	return t.MatchOpts(other, mt, ProcessOptions())
}

// MatchOpts is Match with explicit session Options (MatchConvert / is_convertable).
func (t *Type) MatchOpts(other *Type, mt MatchType, opts Options) bool {
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	switch mt {
	case MatchExact:
		return t == other
	case MatchConvert:
		// Type::is_convertable reads CGOptions::strict_float / lang_cpp
		ok := t.IsConvertableOpts(other, opts)
		// residual ERROR sticky — no invent match true past IsConvertableOpts residual hole
		if sessHasError(nil) {
			return false
		}
		return ok
	case MatchDereference:
		ok := t.IsDereferencedFrom(other)
		// residual ERROR sticky — no invent match true past IsDereferencedFrom residual hole
		if sessHasError(nil) {
			return false
		}
		return ok
	case MatchDerefExact:
		if other == t {
			return true
		}
		ok := t.IsDereferencedFrom(other)
		// residual ERROR sticky — no invent match true past IsDereferencedFrom residual hole
		if sessHasError(nil) {
			return false
		}
		return ok
	case MatchFlexible:
		ok := t.IsDerivable(other)
		// residual ERROR sticky — no invent match true past IsDerivable residual hole
		if sessHasError(nil) {
			return false
		}
		return ok
	default:
		return false
	}
}

// IsPromotable mirrors Type::is_promotable.
// Type.cpp:1387–1416 — integer rank promotion among simples.
// Type* always live; sticky false (no invent not-promotable soft-skip past hole).
func (t *Type) IsPromotable(other *Type) bool {
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !t.IsSimple() || !other.IsSimple() {
		// residual ERROR sticky — no invent not-promotable soft-skip past IsSimple residual
		if sessHasError(nil) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue promote past IsSimple residual true
	if sessHasError(nil) {
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
// Uses session CGOptions; no soft invent Defaults().
func (t *Type) IsConvertable(other *Type) bool {
	return t.IsConvertableOpts(other, ProcessOptions())
}

// IsConvertableOpts applies CGOptions::strict_float / lang_cpp pointer rules.
// Incomplete Type* sticky false (no invent not-convertable soft-skip past holes).
func (t *Type) IsConvertableOpts(other *Type, opts Options) bool {
	// both Type* always live; sticky incomplete no invent not-convertable soft-skip
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if t == other {
		return true
	}
	if t.IsSimple() && other.IsSimple() {
		// residual ERROR sticky — no invent soft-continue convert past IsSimple residual
		if sessHasError(nil) {
			return false
		}
		// forbidden conversion from float to int (Type.cpp:1428–1429)
		of := other.IsFloat()
		// residual ERROR sticky — no invent soft-continue convert past IsFloat residual
		if sessHasError(nil) {
			return false
		}
		tf := t.IsFloat()
		if sessHasError(nil) {
			return false
		}
		if of && !tf {
			return false
		}
		if (t.simple != EVoid && other.simple != EVoid) || t.simple == other.simple {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue pointer convert past IsSimple residual false
	if sessHasError(nil) {
		return false
	}
	if t.ptrTo != nil && other.ptrTo != nil {
		if t.ptrTo == other.ptrTo {
			return true
		}
		ts := t.ptrTo.IsSimple()
		// residual ERROR sticky — no invent soft-convert past subject pointee IsSimple residual
		if sessHasError(nil) {
			return false
		}
		os := other.ptrTo.IsSimple()
		// residual ERROR sticky — no invent soft-convert past other pointee IsSimple residual
		if sessHasError(nil) {
			return false
		}
		if ts && os {
			if t.ptrTo.simple == other.ptrTo.simple {
				return true
			}
			// Type.cpp:1439–1449
			if opts.StrictFloat {
				tf := t.ptrTo.IsFloat()
				// residual ERROR sticky — no invent soft-continue convert past IsFloat residual
				if sessHasError(nil) {
					return false
				}
				of := other.ptrTo.IsFloat()
				if sessHasError(nil) {
					return false
				}
				if (tf && !of) || (!tf && of) {
					return false
				}
			}
			if opts.LangCPP {
				return false
			}
			sb1 := t.ptrTo.SizeInBytes()
			// residual ERROR sticky — no invent size-equal past SizeInBytes residual
			if sessHasError(nil) {
				return false
			}
			sb2 := other.ptrTo.SizeInBytes()
			if sessHasError(nil) {
				return false
			}
			return sb1 == sb2
		}
	}
	return false
}

// IsDereferencedFrom mirrors Type::is_dereferenced_from.
// Type* always live; sticky false (no invent not-deref soft-skip past hole).
func (t *Type) IsDereferencedFrom(other *Type) bool {
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if other.ptrTo == nil {
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
// Type* always live; sticky false (no invent not-derivable / equal-nil soft-skip past hole).
func (t *Type) IsDerivable(other *Type) bool {
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if t == other {
		return true
	}
	if t.IsConvertable(other) {
		// residual ERROR sticky — no invent derivable true past IsConvertable residual hole
		if sessHasError(nil) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue derivable past IsConvertable residual false
	if sessHasError(nil) {
		return false
	}
	if t.IsDereferencedFrom(other) {
		// residual ERROR sticky — no invent derivable true past IsDereferencedFrom residual hole
		if sessHasError(nil) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue derivable past IsDereferencedFrom residual false
	if sessHasError(nil) {
		return false
	}
	return t.ptrTo == other
}

// IsEquivalent mirrors Type::is_equivalent — same size and signedness for simples.
// Type.cpp:1455–1464.
// Type* always live; sticky false (no invent equal-nil soft-skip past hole).
func (t *Type) IsEquivalent(other *Type) bool {
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if t == other {
		return true
	}
	if t.IsSimple() && other.IsSimple() {
		// residual ERROR sticky — no invent soft-equal past IsSimple residual hole
		if sessHasError(nil) {
			return false
		}
		s1 := t.IsSigned()
		// residual ERROR sticky — no invent soft-equal past IsSigned residual hole
		if sessHasError(nil) {
			return false
		}
		s2 := other.IsSigned()
		if sessHasError(nil) {
			return false
		}
		n1 := t.SizeInBytes()
		// residual ERROR sticky — no invent soft-equal past SizeInBytes residual hole
		if sessHasError(nil) {
			return false
		}
		n2 := other.SizeInBytes()
		if sessHasError(nil) {
			return false
		}
		return s1 == s2 && n1 == n2
	}
	// residual ERROR sticky — no invent soft-not-equal past IsSimple residual false
	if sessHasError(nil) {
		return false
	}
	return false
}

// BaseType walks pointers to the ultimate pointee (Type::get_base_type).
// Type* always live at get_base_type; sticky nil (no invent missing base soft-skip
// past hole / soft re-pick).
func (t *Type) BaseType() *Type {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	for t.ptrTo != nil {
		t = t.ptrTo
	}
	return t
}

// NeedsCast mirrors Type::needs_cast.
// Type.cpp:1470–1473 — this is pointer and base_type not equivalent to other's base.
// `this` is the expression's type; `other` is the desired cast target type.
// Incomplete Type* sticky false for nil shell; incomplete base sticky needs-cast
// (no invent no-cast / soft re-pick past holes).
func (t *Type) NeedsCast(other *Type) bool {
	// both Type* always live; sticky incomplete no invent no-cast soft-skip
	if t == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if t.PtrType() == nil {
		// residual ERROR sticky — no invent no-cast soft-skip past PtrType residual hole
		if sessHasError(nil) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue cast past PtrType residual true path
	if sessHasError(nil) {
		return true
	}
	tb, ob := t.BaseType(), other.BaseType()
	// residual ERROR sticky — no invent soft-continue no-cast past BaseType residual
	if sessHasError(nil) {
		return true
	}
	if tb == nil || ob == nil {
		// incomplete base type sticky needs cast (restrictive)
		sessNoteError(nil, ErrGeneric)
		return true
	}
	eq := tb.IsEquivalent(ob)
	// residual ERROR sticky — no invent soft-continue no-cast past IsEquivalent residual
	if sessHasError(nil) {
		return true
	}
	return !eq
}

// HasBitfields mirrors Type::has_bitfields.
// Type.cpp:1290–1301.
// Type* always live; nil Type sticky true (no invent bitfield-free soft-skip).
// Type* always live on Fields; nil field hole sticky true (no invent bitfield-free).
func (t *Type) HasBitfields() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	for _, f := range t.Fields {
		if f.BitWidth >= 0 {
			return true
		}
		if f.Type == nil {
			// incomplete field Type sticky has-bitfields (restrictive)
			sessNoteError(nil, ErrGeneric)
			return true
		}
		isStruct := f.Type.IsStruct()
		// residual ERROR sticky — no invent soft-continue later fields past IsStruct residual
		if sessHasError(nil) {
			return true
		}
		if isStruct {
			if f.Type.HasBitfields() {
				// residual ERROR sticky — no invent has-bitfields true past nested hole
				if sessHasError(nil) {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later fields past nested residual false
			if sessHasError(nil) {
				return true
			}
		}
	}
	return false
}

// IsBitfieldIndex mirrors Type::is_bitfield(index).
// Type.cpp:1286–1288 — assert(index < bitfields_length_.size()); BitWidth >= 0.
func (t *Type) IsBitfieldIndex(index int) bool {
	// Type* always live; sticky false (no invent non-bitfield soft-skip past hole)
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// OOB is assert path sticky — fail closed false (not invent non-bitfield / soft skip)
	if index < 0 || index >= len(t.Fields) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return t.Fields[index].BitWidth >= 0
}

// IsUnamedPadding mirrors Type::is_unamed_padding.
// Type.cpp:1278–1283 — assert(index < sz); bitfields_length_[index] == 0.
func (t *Type) IsUnamedPadding(index int) bool {
	// Type* always live; sticky false (no invent not-padding soft success past hole)
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// assert OOB sticky — no invent "not padding" soft success past hole
	if index < 0 || index >= len(t.Fields) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// only bitfield slots can be zero-width padding
	return t.Fields[index].BitWidth == 0
}

// HasPadding mirrors Type::has_padding.
// Type.cpp:1305–1314 — unpacked struct, bitfield member, or nested padding.
// Type* always live; sticky true (no invent padding-free soft-skip past hole).
// Type* always live on Fields; nil field hole sticky true (no invent padding-free
// soft-skip nested HasPadding past incomplete field Type).
func (t *Type) HasPadding() bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if t.IsStruct() && !t.Packed {
		return true
	}
	for i, f := range t.Fields {
		if t.IsBitfieldIndex(i) {
			// residual ERROR sticky — no invent has-padding true past IsBitfieldIndex hole
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past IsBitfieldIndex residual
		if sessHasError(nil) {
			return true
		}
		if f.Type == nil {
			// incomplete field Type sticky has-padding (restrictive)
			sessNoteError(nil, ErrGeneric)
			return true
		}
		if f.Type.HasPadding() {
			// residual ERROR sticky — no invent has-padding true past nested hole
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual false
		if sessHasError(nil) {
			return true
		}
	}
	return false
}

// ChooseRandomNonvoidSimple mirrors Type::choose_random_nonvoid_simple.
// Type.cpp:618–635 — rnd_upto(MAX_SIMPLE_TYPES, SIMPLE_TYPES_PROB_FILTER); no soft invent eInt.
// Type.cpp:1246 — assert(ty != eVoid) on choose_random_simple; filter zeros void weight.
func ChooseRandomNonvoidSimple(r *Rng, probs *Probabilities) ESimpleType {
	// C++ always has RNG + probs; sticky no soft invent EInt when missing
	if r == nil || probs == nil {
		sessNoteError(nil, ErrGeneric)
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
// Type* always live; sticky true (no invent overflow-free soft-skip past hole).
func (t *Type) SignedOverflowPossible(intSize int) bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if !t.IsSimple() {
		// residual ERROR sticky — no invent overflow-free soft-skip past IsSimple residual
		if sessHasError(nil) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue past IsSimple residual true
	if sessHasError(nil) {
		return true
	}
	if !t.IsSigned() {
		// residual ERROR sticky — no invent overflow-free soft-skip past IsSigned residual
		if sessHasError(nil) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue past IsSigned residual true
	if sessHasError(nil) {
		return true
	}
	// CGOptions::int_size always positive; no invent platform size when arg is 0
	if intSize < 1 {
		return false
	}
	sz := t.SizeInBytes()
	// residual ERROR sticky — no invent overflow-free soft-skip past SizeInBytes residual
	if sessHasError(nil) {
		return true
	}
	return sz >= intSize
}

// GetTypeFromString mirrors Type::get_type_from_string.
// Type.cpp:370–402 — builtin name → simple type; default assert(0) sticky.
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
		// Type.cpp:401 assert(0); sticky — no soft invent GetIntType for unknown names
		sessNoteError(nil, ErrGeneric)
		return nil
	}
}

// TypeNameString is the reverse of get_type_from_string for simple types.
// Type* always live at name emit; sticky empty (no invent empty type-name past hole).
// Struct/union sid always live (parity with CName); empty StructName sticky empty
// (no invent bare empty type-name token / soft re-pick past incomplete aggregate IR).
func (t *Type) TypeNameString() string {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if t.ptrTo != nil {
		return "Pointer"
	}
	isSt := t.IsStruct()
	// residual ERROR sticky — no invent soft-name past IsStruct residual
	if sessHasError(nil) {
		return ""
	}
	isUn := t.IsUnion()
	// residual ERROR sticky — no invent soft-name past IsUnion residual
	if sessHasError(nil) {
		return ""
	}
	if isSt || isUn {
		// Type.cpp struct/union tag always live at name emit; sticky no invent ""
		if t.StructName == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
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
		// unknown simple — assert path sticky; no soft invent empty type-name token
		sessNoteError(nil, ErrGeneric)
		return ""
	}
}

// PrintfDirective mirrors Type::printf_directive.
// Type.cpp:1932–1957.
// Type* always live at printf emit; sticky empty (no invent empty directive past hole).
func (t *Type) PrintfDirective() string {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if t.ptrTo != nil {
		return "0x%0x"
	}
	if t.IsAggregate() {
		// residual ERROR sticky — no invent soft-printf past IsAggregate residual true
		if sessHasError(nil) {
			return ""
		}
		// Type.cpp:1945–1951 — fields[i]->printf_directive always live Type*
		// sticky no invent empty holes / partial "{%d, }" for nil field types
		var b strings.Builder
		b.WriteString("{")
		for i, f := range t.Fields {
			if f.Type == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			part := f.Type.PrintfDirective()
			// residual ERROR sticky — no invent soft-continue later fields past nested residual
			if sessHasError(nil) {
				return ""
			}
			if part == "" {
				sessNoteError(nil, ErrGeneric)
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
	// residual ERROR sticky — no invent soft-continue past IsAggregate residual false
	if sessHasError(nil) {
		return ""
	}
	if t.IsSimple() {
		// residual ERROR sticky — no invent soft-printf past IsSimple residual
		if sessHasError(nil) {
			return ""
		}
		sz := t.SizeInBytes()
		// residual ERROR sticky — no invent format past SizeInBytes residual hole
		if sessHasError(nil) {
			return ""
		}
		if sz >= 8 {
			if t.IsSigned() {
				// residual ERROR sticky — no invent %lld past IsSigned residual hole
				if sessHasError(nil) {
					return ""
				}
				return "%lld"
			}
			// residual ERROR sticky — no invent %llu past IsSigned residual false path
			if sessHasError(nil) {
				return ""
			}
			return "%llu"
		}
		if t.IsSigned() {
			// residual ERROR sticky — no invent %d past IsSigned residual hole
			if sessHasError(nil) {
				return ""
			}
			return "%d"
		}
		// residual ERROR sticky — no invent %u past IsSigned residual false path
		if sessHasError(nil) {
			return ""
		}
		return "%u"
	}
	// Type.cpp switch covers simple/pointer/struct/union only; other shape sticky
	// empty (no invent empty directive soft-success past broken type shell)
	sessNoteError(nil, ErrGeneric)
	return ""
}

// SizeofString mirrors Type::get_type_sizeof_string.
// Type.cpp:1708–1714 — Output on live Type*; sticky no invent sizeof()/sizeof(void).
func (t *Type) SizeofString() string {
	// Type* always live; sticky no invent sizeof(void)/sizeof()
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	cn := t.CName()
	if cn == "" {
		// incomplete CName IR sticky (CName may already SetError)
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	return "sizeof(" + cn + ")"
}

// HasAggregateField mirrors Type::has_aggregate_field.
// Type.cpp:1057–1064.
// Type* always live on Fields; nil hole sticky true (no invent none / soft re-pick
// past incomplete field Type that would soft-skip ccomp packing bans).
func HasAggregateField(fields []StructField) bool {
	for _, f := range fields {
		if f.Type == nil {
			// incomplete field Type sticky has-aggregate (restrictive)
			sessNoteError(nil, ErrGeneric)
			return true
		}
		if f.Type.IsAggregate() {
			// residual ERROR sticky — no invent soft has-aggregate past IsAggregate residual true
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue past IsAggregate residual false
		if sessHasError(nil) {
			return true
		}
	}
	return false
}

// HasLongLongField mirrors Type::has_longlong_field.
// Type.cpp:1066–1073.
// Type* always live on Fields; nil hole sticky true (no invent none / soft re-pick).
func HasLongLongField(fields []StructField) bool {
	return HasLongLongFieldSess(nil, fields)
}

func HasLongLongFieldSess(s *Session, fields []StructField) bool {
	for _, f := range fields {
		if f.Type == nil {
			// incomplete field Type sticky has-longlong (restrictive)
			sessNoteError(s, ErrGeneric)
			return true
		}
		if f.Type.IsSimple() {
			// residual ERROR sticky — no invent soft has-longlong past IsSimple residual
			if sessHasError(s) {
				return true
			}
			if f.Type.simple == ELongLong || f.Type.simple == EULongLong {
				return true
			}
		} else if sessHasError(s) {
			return true
		}
		if f.Type.IsAggregate() {
			// residual ERROR sticky — no invent soft has-longlong past IsAggregate residual
			if sessHasError(s) {
				return true
			}
			if HasLongLongFieldSess(s, f.Type.Fields) {
				return true
			}
			// residual ERROR sticky — no invent soft-continue past recursive residual
			if sessHasError(s) {
				return true
			}
		} else if sessHasError(s) {
			return true
		}
	}
	return false
}

// IsUnnamedPadding mirrors Type::is_unamed_padding.
// Type.cpp:1280+ — zero-width bitfield without a name used as padding.
// Our StructField always has names; treat BitWidth==0 as unnamed padding candidate.
// Type* always live; sticky false (no invent not-padding soft success past hole).}

func (t *Type) IsUnnamedPadding(index int) bool {
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// assert OOB sticky — no invent "not padding" soft success past hole
	if index < 0 || index >= len(t.Fields) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	f := t.Fields[index]
	// zero-length bitfield is padding
	return f.BitWidth == 0
}

// IfStructWillHaveAssignOps mirrors Type::if_struct_will_have_assign_ops.
// Type.cpp:505–510 — C++ only; flipcoin RegularVolatileProb.
// Non-C++ is complete false. RNG always live under C++; sticky false (no invent
// no-assign-ops soft-skip past hole). Nil probs → 0% (no invent default 50).
func IfStructWillHaveAssignOps(r *Rng, opts Options, probs *Probabilities) bool {
	if !opts.LangCPP {
		return false
	}
	if r == nil {
		sessNoteError(nil, ErrGeneric)
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
