// Upstream: Type.h / Type.cpp (eSimpleType, get_simple_type, choose_random_nonvoid_simple,
// match / is_convertable / is_signed / SizeInBytes / get_indirect_level).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

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
func GetSimpleType(st ESimpleType) *Type {
	if st < 0 || int(st) >= MaxSimpleTypes {
		st = EInt
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
func (t *Type) IsConstStructUnion() bool {
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type != nil && f.Type.IsConstStructUnion() {
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
func (t *Type) IsVolatileStructUnion() bool {
	if t == nil || !t.IsAggregate() {
		return false
	}
	for _, f := range t.Fields {
		if f.Type != nil && f.Type.IsVolatileStructUnion() {
			return true
		}
		if f.Qfer.IsVolatile() {
			return true
		}
	}
	return false
}

// StructDepth mirrors Type::get_struct_depth.
// Type.cpp:1261–1275 — 0 if not struct; else 1 + max field depth.
func (t *Type) StructDepth() int {
	if t == nil || !t.IsStruct() {
		return 0
	}
	depth := 1
	maxField := 0
	for _, f := range t.Fields {
		if f.Type == nil {
			continue
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

// IsFloat mirrors Type::is_float.
func (t *Type) IsFloat() bool {
	return t != nil && t.IsSimple() && t.simple == EFloat
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

// SizeInBytes mirrors Type::SizeInBytes for simple types (Type.cpp).
func (t *Type) SizeInBytes() int {
	if t == nil {
		return 0
	}
	if t.ptrTo != nil {
		// Platform pointer size not modeled here; LP64-ish 8 is not asserted yet.
		return 8
	}
	switch t.simple {
	case EVoid:
		return 0
	case EChar, EUChar:
		return 1
	case EShort, EUShort:
		return 2
	case EInt, EUInt, ELong, EULong, EFloat:
		return 4
	case ELongLong, EULongLong, EInt128, EUInt128:
		return 8
	default:
		return 4
	}
}

// CName is a minimal C spelling for simple/pointer types.
func (t *Type) CName() string {
	if t == nil {
		return "void"
	}
	if t.ptrTo != nil {
		return t.ptrTo.CName() + "*"
	}
	if t.isStruct {
		if t.StructName != "" {
			return "struct " + t.StructName
		}
		return "struct"
	}
	if t.isUnion {
		if t.StructName != "" {
			return "union " + t.StructName
		}
		return "union"
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
		return "int"
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
var pointerCache = map[*Type]*Type{}

// PointerTo builds/caches a pointer type (find_pointer_type-ish for one level).
func PointerTo(pointee *Type) *Type {
	if pointee == nil {
		pointee = GetSimpleType(EInt)
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
		return t.IsConvertable(other)
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

// IsConvertable mirrors Type::is_convertable (simple + pointer size rules).
func (t *Type) IsConvertable(other *Type) bool {
	if t == nil || other == nil {
		return false
	}
	if t == other {
		return true
	}
	if t.IsSimple() && other.IsSimple() {
		// forbidden float → int
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
			// lang_cpp / strict_float not applied (defaults: C, !strict_float)
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

// ChooseRandomNonvoidSimple mirrors Type::choose_random_nonvoid_simple.
func ChooseRandomNonvoidSimple(r *Rng, probs *Probabilities) ESimpleType {
	if r == nil || probs == nil {
		return EInt
	}
	v := r.RndUptoFilter(uint32(MaxSimpleTypes), probs.SimpleTypesFilter())
	st := ESimpleType(v)
	if st == EVoid {
		return EInt
	}
	return st
}

// GetIntType mirrors get_int_type() → eInt.
// Type.cpp:408.
func GetIntType() *Type { return GetSimpleType(EInt) }

