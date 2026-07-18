// Upstream: FactUnion.h / FactUnion.cpp (last-written union field lattice).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strconv"

// FactUnion lattice sentinels (FactUnion.h TOP/BOTTOM).
// FactUnion.cpp:48–49 — TOP = -2, BOTTOM = -1.
const (
	// FactUnionTop — no write known (lattice top).
	FactUnionTop = -2
	// FactUnionBottom — unknown / conflicting writes (lattice bottom).
	FactUnionBottom = -1
)

// FactUnion mirrors FactUnion — last written field id of a union variable.
type FactUnion struct {
	// Var is the union aggregate variable.
	Var *Variable
	// LastWrittenFID is field index last written (or Top/Bottom).
	LastWrittenFID int
}

// MakeFactUnion mirrors FactUnion::make_fact(v, fid).
// FactUnion.cpp:162–167 — default fid 0 when omitted in C++.
func MakeFactUnion(v *Variable, fid int) *FactUnion {
	return &FactUnion{Var: v, LastWrittenFID: fid}
}

// MakeFactUnionTop mirrors make_fact(v) with TOP.
func MakeFactUnionTop(v *Variable) *FactUnion {
	return &FactUnion{Var: v, LastWrittenFID: FactUnionTop}
}

// MakeFactUnions mirrors FactUnion::make_facts.
// FactUnion.cpp:169–176.
func MakeFactUnions(vars []*Variable, fid int) []*FactUnion {
	out := make([]*FactUnion, 0, len(vars))
	for _, v := range vars {
		out = append(out, MakeFactUnion(v, fid))
	}
	return out
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

// IsTop mirrors FactUnion::is_top.
func (f *FactUnion) IsTop() bool {
	return f != nil && f.LastWrittenFID == FactUnionTop
}

// IsBottom mirrors FactUnion::is_bottom.
func (f *FactUnion) IsBottom() bool {
	return f != nil && f.LastWrittenFID == FactUnionBottom
}

// SetTop mirrors FactUnion::set_top.
func (f *FactUnion) SetTop() {
	if f != nil {
		f.LastWrittenFID = FactUnionTop
	}
}

// SetBottom mirrors FactUnion::set_bottom.
func (f *FactUnion) SetBottom() {
	if f != nil {
		f.LastWrittenFID = FactUnionBottom
	}
}

// Clone mirrors FactUnion::clone.
func (f *FactUnion) Clone() *FactUnion {
	if f == nil {
		return nil
	}
	return &FactUnion{Var: f.Var, LastWrittenFID: f.LastWrittenFID}
}

// Equal mirrors FactUnion::equal.
// FactUnion.cpp:195–201.
func (f *FactUnion) Equal(other *FactUnion) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// Imply mirrors FactUnion::imply.
// FactUnion.cpp:249–259 — bottom implies all; equal fid implies; else false.
func (f *FactUnion) Imply(other *FactUnion) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	if f.IsBottom() {
		return true
	}
	if other.IsBottom() {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// Join mirrors FactUnion::join — merge lattice; return true if changed.
// FactUnion.cpp:207–221.
func (f *FactUnion) Join(other *FactUnion) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	if f.Imply(other) {
		return false
	}
	if other.Imply(f) {
		f.LastWrittenFID = other.LastWrittenFID
	} else {
		f.SetBottom()
	}
	return true
}

// GetLastWrittenType mirrors FactUnion::get_last_written_type.
// FactUnion.cpp:63–71.
func (f *FactUnion) GetLastWrittenType() *Type {
	if f == nil || f.Var == nil || f.IsTop() || f.IsBottom() {
		return nil
	}
	fid := f.LastWrittenFID
	if fid < 0 || fid >= len(f.Var.FieldVars) {
		return nil
	}
	fv := f.Var.FieldVars[fid]
	if fv == nil {
		return nil
	}
	return fv.Type
}

// Output mirrors FactUnion::Output.
// FactUnion.cpp:272–275.
func (f *FactUnion) Output() string {
	if f == nil || f.Var == nil {
		return ""
	}
	return f.Var.GetActualName(false) + " last written field: " + strconv.Itoa(f.LastWrittenFID)
}

// IsFieldReadable mirrors FactUnion::is_field_readable.
// FactUnion.cpp:262–270.
func IsFieldReadable(v *Variable, fid int, facts []*FactUnion) bool {
	if v == nil || v.Type == nil || !v.Type.IsUnion() || fid < 0 {
		return false
	}
	if v.Type != nil && fid >= len(v.Type.Fields) && fid >= len(v.FieldVars) {
		return false
	}
	tmp := MakeFactUnion(v, fid)
	fu := FindRelatedUnion(facts, v)
	return fu != nil && tmp.Imply(fu)
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

// JoinVarFactsUnion mirrors FactUnion::join_var_facts for a set of union vars.
// FactUnion.cpp:226–245 — merge existing facts for vars into one.
func JoinVarFactsUnion(facts []*FactUnion, vars []*Variable) *FactUnion {
	var fu *FactUnion
	for _, v := range vars {
		if v == nil {
			continue
		}
		exist := FindRelatedUnion(facts, v)
		if exist == nil {
			continue
		}
		if fu == nil {
			fu = exist.Clone()
			continue
		}
		// hack: both must share var identity for join — set to exist's var
		fu.Var = exist.Var
		fu.Join(exist)
	}
	return fu
}

// MergeUnionFactInto merges nf into facts slice (join if related).
func MergeUnionFactInto(facts []*FactUnion, nf *FactUnion) []*FactUnion {
	if nf == nil {
		return facts
	}
	for i, old := range facts {
		if old != nil && old.Var == nf.Var {
			cp := old.Clone()
			cp.Join(nf)
			facts[i] = cp
			return facts
		}
	}
	return append(facts, nf.Clone())
}
