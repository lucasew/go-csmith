// Upstream: FactPointTo.h / FactPointTo.cpp (points-to facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Special points-to targets (FactPointTo.cpp:61–66).
var (
	// NullPtr mirrors FactPointTo::null_ptr.
	NullPtr = MakeDummyStaticVariable("null")
	// GarbagePtr mirrors FactPointTo::garbage_ptr (dangling / uninit).
	GarbagePtr = MakeDummyStaticVariable("garbage")
	// TBDPtr mirrors FactPointTo::tbd_ptr.
	TBDPtr = MakeDummyStaticVariable("tbd")
)

// FactPointTo mirrors FactPointTo for one pointer variable.
type FactPointTo struct {
	// Var is the pointer whose points-to set this fact describes.
	Var *Variable
	// PointTo are possible pointees (may include NullPtr/GarbagePtr/TBDPtr).
	PointTo []*Variable
}

// NewFactPointTo mirrors FactPointTo(const Variable*) — starts as garbage.
// FactPointTo.cpp:354–359.
func NewFactPointTo(v *Variable) *FactPointTo {
	return &FactPointTo{Var: v, PointTo: []*Variable{GarbagePtr}}
}

// MakeFactPointTo mirrors FactPointTo::make_fact(v, point_to).
func MakeFactPointTo(v *Variable, pointTo *Variable) *FactPointTo {
	return &FactPointTo{Var: v, PointTo: []*Variable{pointTo}}
}

// MakeFactPointToSet mirrors FactPointTo::make_fact(v, set).
func MakeFactPointToSet(v *Variable, set []*Variable) *FactPointTo {
	cp := append([]*Variable(nil), set...)
	return &FactPointTo{Var: v, PointTo: cp}
}

// IsNull mirrors FactPointTo::is_null — any null_ptr in the set.
func (f *FactPointTo) IsNull() bool {
	if f == nil {
		return false
	}
	for _, p := range f.PointTo {
		if p == NullPtr {
			return true
		}
	}
	return false
}

// IsDead mirrors FactPointTo::is_dead — garbage_ptr in the set.
func (f *FactPointTo) IsDead() bool {
	if f == nil {
		return false
	}
	for _, p := range f.PointTo {
		if p == GarbagePtr {
			return true
		}
	}
	return false
}

// IsTBDOnly mirrors FactPointTo::is_tbd_only.
func (f *FactPointTo) IsTBDOnly() bool {
	return f != nil && len(f.PointTo) == 1 && f.PointTo[0] == TBDPtr
}

// IsSpecialPtr mirrors FactPointTo::is_special_ptr.
func IsSpecialPtr(p *Variable) bool {
	return p == NullPtr || p == GarbagePtr || p == TBDPtr
}

// FindRelatedPointTo mirrors find_related_fact for ePointTo (var identity).
func FindRelatedPointTo(facts []*FactPointTo, p *Variable) *FactPointTo {
	if p == nil {
		return nil
	}
	for _, f := range facts {
		if f != nil && f.Var == p {
			return f
		}
	}
	return nil
}

// IsValidPtr mirrors FactPointTo::is_valid_ptr(Variable*, facts).
// FactPointTo.cpp:411–419 — needs related fact; null/dead forbidden when probs are 0.
// opts.NullPointerDerefProb / DeadPointerDerefProb default 0 (upstream CGOptions).
func IsValidPtr(p *Variable, facts []*FactPointTo, nullProb, deadProb int) bool {
	fact := FindRelatedPointTo(facts, p)
	if fact == nil {
		return false
	}
	if nullProb <= 0 && fact.IsNull() {
		return false
	}
	if deadProb <= 0 && fact.IsDead() {
		return false
	}
	return true
}

// IsDanglingPtr mirrors FactPointTo::is_dangling_ptr.
// FactPointTo.cpp:476–482 — related fact is dead (and dead deref not allowed).
func IsDanglingPtr(p *Variable, facts []*FactPointTo, deadProb int) bool {
	fact := FindRelatedPointTo(facts, p)
	if fact == nil {
		return false
	}
	return fact.IsDead() && deadProb == 0
}
