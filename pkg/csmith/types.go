// Upstream: Type.h / Type.cpp (eSimpleType, get_simple_type, choose_random_nonvoid_simple).
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

// Type is a minimal Type.cpp stand-in for simple types only.
// Pointers/structs/unions land with later ports.
type Type struct {
	simple ESimpleType
	// ptrTo non-nil ⇒ pointer type (Type::ptr_type); not fully ported yet.
	ptrTo *Type
}

// GetSimpleType mirrors Type::get_simple_type — canonical simple Type values.
// Upstream caches Type::simple_types[MAX_SIMPLE_TYPES]; we allocate immutable values.
func GetSimpleType(st ESimpleType) *Type {
	if st < 0 || int(st) >= MaxSimpleTypes {
		st = EInt
	}
	return &Type{simple: st}
}

// IsSimple reports eType == eSimple.
func (t *Type) IsSimple() bool {
	return t != nil && t.ptrTo == nil
}

// Simple returns the eSimpleType (only meaningful if IsSimple).
func (t *Type) Simple() ESimpleType {
	if t == nil {
		return EVoid
	}
	return t.simple
}

// IsNonVoidSimple mirrors Type::is_nonvoid_simple-ish checks used after choose.
func (t *Type) IsNonVoidSimple() bool {
	return t != nil && t.IsSimple() && t.simple != EVoid
}

// CName is a minimal C spelling for simple types (for tests / later emit).
// Not a full Type::to_string port.
func (t *Type) CName() string {
	if t == nil {
		return "void"
	}
	if t.ptrTo != nil {
		return t.ptrTo.CName() + "*"
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

// ChooseRandomNonvoidSimple mirrors Type::choose_random_nonvoid_simple:
//
//	(eSimpleType)rnd_upto(MAX_SIMPLE_TYPES, SIMPLE_TYPES_PROB_FILTER());
//
// Filter rejects zero-weight types from set_default_simple_types_prob.
func ChooseRandomNonvoidSimple(r *Rng, probs *Probabilities) ESimpleType {
	if r == nil || probs == nil {
		return EInt
	}
	// Type.cpp:618–635
	v := r.RndUptoFilter(uint32(MaxSimpleTypes), probs.SimpleTypesFilter())
	st := ESimpleType(v)
	if st == EVoid {
		// Filter should exclude void (weight 0); belt-and-suspenders.
		return EInt
	}
	return st
}
