// Upstream: FactUnion.h / FactUnion.cpp (last-written union field; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// FactUnion lattice sentinels (FactUnion.h).
const (
	// FactUnionTop — no write known.
	FactUnionTop = -1
	// FactUnionBottom — unknown / conflicting writes.
	FactUnionBottom = -2
)

// FactUnion mirrors FactUnion — last written field id of a union variable.
type FactUnion struct {
	// Var is the union aggregate variable.
	Var *Variable
	// LastWrittenFID is field index last written (or Top/Bottom).
	LastWrittenFID int
}

// MakeFactUnion mirrors FactUnion::make_fact.
func MakeFactUnion(v *Variable, fid int) *FactUnion {
	return &FactUnion{Var: v, LastWrittenFID: fid}
}

// FindRelatedUnion finds FactUnion for union variable v.
func FindRelatedUnion(facts []*FactUnion, v *Variable) *FactUnion {
	if v == nil {
		return nil
	}
	for _, f := range facts {
		if f != nil && f.Var == v {
			return f
		}
	}
	return nil
}

// Imply mirrors FactUnion::imply.
// FactUnion.cpp:249–259 — bottom implies all; equal fid implies; else false.
func (f *FactUnion) Imply(other *FactUnion) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	if f.LastWrittenFID == FactUnionBottom {
		return true
	}
	if other.LastWrittenFID == FactUnionBottom {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// IsNonreadableField mirrors FactUnion::is_nonreadable_field.
// FactUnion.cpp:178–192 — when analysis active (facts non-empty), unread union fields blocked.
// When facts empty, returns false (analysis not engaged).
func IsNonreadableField(v *Variable, facts []*FactUnion) bool {
	if v == nil || !v.IsInsideUnionField() {
		return false
	}
	if len(facts) == 0 {
		// no FactUnion tracking yet — do not ban all union fields
		return false
	}
	// walk to the union field variable
	uf := v
	for uf != nil && !uf.IsUnionField() {
		uf = uf.FieldVarOf
	}
	if uf == nil || uf.FieldVarOf == nil {
		return false
	}
	parent := uf.FieldVarOf
	fid := uf.GetFieldID()
	tmp := MakeFactUnion(parent, fid)
	fu := FindRelatedUnion(facts, parent)
	if fu == nil || !tmp.Imply(fu) {
		// no fact or last write was a different field → nonreadable
		return true
	}
	return false
}
