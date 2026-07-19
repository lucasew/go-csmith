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
// FactUnion.cpp:162–167 — assert(v==null || union type); default fid 0 when omitted.
func MakeFactUnion(v *Variable, fid int) *FactUnion {
	// FactUnion.cpp:163 — assert(v == nullptr || v->type->eType == eUnion)
	// no soft invent FactUnion on scalar/struct vars
	if v != nil && (v.Type == nil || !v.Type.IsUnion()) {
		return nil
	}
	return &FactUnion{Var: v, LastWrittenFID: fid}
}

// MakeFactUnionTop mirrors make_fact(v) with TOP.
func MakeFactUnionTop(v *Variable) *FactUnion {
	f := MakeFactUnion(v, FactUnionTop)
	if f == nil {
		return nil
	}
	return f
}

// MakeFactUnions mirrors FactUnion::make_facts.
// FactUnion.cpp:169–176 — vars[i] always live; make_fact asserts union type.
// Incomplete IR fails closed IncompleteUnionFactSlice (not bare nil —
// UnionFactsComplete(nil)==true invents empty-complete make_facts success).
func MakeFactUnions(vars []*Variable, fid int) []*FactUnion {
	out := make([]*FactUnion, 0, len(vars))
	for _, v := range vars {
		if v == nil {
			return IncompleteUnionFactSlice()
		}
		f := MakeFactUnion(v, fid)
		// non-union subject is assert path — fail closed whole batch
		if f == nil {
			return IncompleteUnionFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// FindRelatedUnion finds FactUnion for union variable v.
// FactUnion* always live; nil hole fails closed (nil — no invent skip to later match).
func FindRelatedUnion(facts []*FactUnion, v *Variable) *FactUnion {
	if v == nil {
		return nil
	}
	for _, f := range facts {
		if f == nil {
			return nil
		}
		if f.Var == v {
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
	// FactUnion.cpp:65 — assert(var->type && eUnion); fail closed nil if not union
	if f.Var.Type == nil || !f.Var.Type.IsUnion() {
		return nil
	}
	fid := f.LastWrittenFID
	// FactUnion.cpp:68–69 — assert fid in [0, field_vars.size())
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
	// var name always live; no invent " last written field: N" without identifier
	name := f.Var.GetActualName(false)
	if name == "" {
		return ""
	}
	return name + " last written field: " + strconv.Itoa(f.LastWrittenFID)
}

// UnionFactsComplete reports FactUnion* maps have no nil holes.
// Incomplete lists must not invent readable/related matches past a hole.
// Note: UnionFactsComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteUnionFactSlice() so later checks do not invent empty success.
func UnionFactsComplete(facts []*FactUnion) bool {
	for _, f := range facts {
		if f == nil || f.Var == nil {
			return false
		}
	}
	return true
}

// IncompleteUnionFactSlice is the fail-closed incomplete union-fact list marker.
// UnionFactsComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteUnionFactSlice() []*FactUnion {
	return []*FactUnion{nil}
}

// IsFieldReadable mirrors FactUnion::is_field_readable.
// FactUnion.cpp:262–270.
// Incomplete facts fail closed false (no invent readable past UnionFacts hole).
func IsFieldReadable(v *Variable, fid int, facts []*FactUnion) bool {
	if v == nil || v.Type == nil || !v.Type.IsUnion() || fid < 0 {
		return false
	}
	if !UnionFactsComplete(facts) {
		return false
	}
	if v.Type != nil && fid >= len(v.Type.Fields) && fid >= len(v.FieldVars) {
		return false
	}
	tmp := MakeFactUnion(v, fid)
	if tmp == nil {
		return false
	}
	fu := FindRelatedUnion(facts, v)
	return fu != nil && tmp.Imply(fu)
}

// IsNonreadableField mirrors FactUnion::is_nonreadable_field.
// FactUnion.cpp:178–192 — when analysis active (facts non-empty), unread union fields blocked.
// When facts empty, returns false (analysis not engaged).
// Incomplete FactUnion maps fail closed nonreadable (no invent readable while
// FindRelatedUnion returns nil past a hole before a matching parent fact).
func IsNonreadableField(v *Variable, facts []*FactUnion) bool {
	if v == nil || !v.IsInsideUnionField() {
		return false
	}
	if len(facts) == 0 {
		// no FactUnion tracking yet — do not ban all union fields
		return false
	}
	if !UnionFactsComplete(facts) {
		return true
	}
	// walk to the union field variable
	// FactUnion.cpp:181–184 — for (; !is_union_field(); field_var_of); assert(is_union_field)
	uf := v
	for uf != nil && !uf.IsUnionField() {
		uf = uf.FieldVarOf
	}
	// broken IR (no union field in ancestry) — fail closed nonreadable (no invent readable)
	if uf == nil || uf.FieldVarOf == nil {
		return true
	}
	parent := uf.FieldVarOf
	fid := uf.GetFieldID()
	// incomplete parent FieldVars → GetFieldID -1 → MakeFactUnion fails → nonreadable
	tmp := MakeFactUnion(parent, fid)
	if tmp == nil {
		// parent not union type — fail closed nonreadable
		return true
	}
	fu := FindRelatedUnion(facts, parent)
	if fu == nil || !tmp.Imply(fu) {
		// no fact or last write was a different field → nonreadable
		return true
	}
	return false
}

// JoinVarFactsUnion mirrors FactUnion::join_var_facts for a set of union vars.
// FactUnion.cpp:226–245 — merge existing facts for vars into one.
// Variable* always live in vars; nil hole / incomplete union map fails closed
// (nil join — callers of transfer must not invent empty complete from that alone;
// RhsToLhsTransferUnion pre-checks completeness before join).
func JoinVarFactsUnion(facts []*FactUnion, vars []*Variable) *FactUnion {
	// incomplete union map / vars fails closed sticky (no invent soft nil join success path)
	if !UnionFactsComplete(facts) || !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return nil
	}
	var fu *FactUnion
	for _, v := range vars {
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
// FactUnion* always live; nil nf or map hole fails closed IncompleteUnionFactSlice
// (no invent empty-complete via UnionFactsComplete(nil)).
func MergeUnionFactInto(facts []*FactUnion, nf *FactUnion) []*FactUnion {
	if nf == nil {
		return IncompleteUnionFactSlice()
	}
	if !UnionFactsComplete(facts) {
		return IncompleteUnionFactSlice()
	}
	for i, old := range facts {
		if old.Var == nf.Var {
			cp := old.Clone()
			if cp == nil {
				return IncompleteUnionFactSlice()
			}
			cp.Join(nf)
			facts[i] = cp
			return facts
		}
	}
	cl := nf.Clone()
	if cl == nil {
		return IncompleteUnionFactSlice()
	}
	return append(facts, cl)
}

// RhsToLhsTransferUnion mirrors FactUnion::rhs_to_lhs_transfer.
// FactUnion.cpp:74–118 — constant→fid 0; variable→join RHS union facts;
// assign/comma peel to RHS; function return uses registry when available.
// Incomplete IR fails closed IncompleteUnionFactSlice (not bare nil invent
// empty-complete transfer / assign apply success).
func RhsToLhsTransferUnion(
	unionFacts []*FactUnion,
	ptFacts []*FactPointTo,
	lvars []*Variable,
	rhs *Expression,
) []*FactUnion {
	// complete empty transfer targets
	if len(lvars) == 0 {
		return nil
	}
	// FactUnion.cpp:82 — assert(rhs != nullptr)
	if rhs == nil {
		return IncompleteUnionFactSlice()
	}
	// incomplete input maps must not invent transfer past holes
	if !UnionFactsComplete(unionFacts) || !FactsComplete(ptFacts) {
		return IncompleteUnionFactSlice()
	}
	// FactUnion.cpp:80–81 — assert all possible LHS are unions
	for _, v := range lvars {
		if v == nil || v.Type == nil || !v.Type.IsUnion() {
			// fail closed — no soft invent transfer onto non-union
			return IncompleteUnionFactSlice()
		}
	}
	switch rhs.Term {
	case TermConstant:
		return MakeFactUnions(lvars, 0)
	case TermVariable:
		if rhs.Var == nil {
			return IncompleteUnionFactSlice()
		}
		// incomplete type IR must not invent level-0 union transfer
		indirect, iok := rhs.IndirectLevelComplete()
		if !iok {
			return IncompleteUnionFactSlice()
		}
		// FactUnion.cpp:89 — assert(indirect >= 0); no soft invent clamp to 0 for &
		if indirect < 0 {
			return IncompleteUnionFactSlice()
		}
		rvars := MergePointeesOfPointer(rhs.Var.GetCollective(), indirect, ptFacts)
		// incomplete pointees (not complete empty)
		if !VariablesComplete(rvars) {
			return IncompleteUnionFactSlice()
		}
		rhsFact := JoinVarFactsUnion(unionFacts, rvars)
		if rhsFact == nil {
			// sticky incomplete join vs complete: no related RHS fact → empty transfer
			if HasError() {
				return IncompleteUnionFactSlice()
			}
			return nil
		}
		return MakeFactUnions(lvars, rhsFact.LastWrittenFID)
	case TermFunction:
		// FactUnion.cpp:99–109 — return fact for invocation RV (union category).
		// assert(rv_fact) when FIU present — fail closed if missing (no invent fid 0)
		if rhs.Invoke == nil || rhs.Invoke.User == nil || rhs.Invoke.User.RV == nil {
			return IncompleteUnionFactSlice()
		}
		rv := rhs.Invoke.User.RV
		if uf := FindRelatedUnion(unionFacts, rv); uf != nil {
			return MakeFactUnions(lvars, uf.LastWrittenFID)
		}
		// FactUnion.cpp:107 assert(rv_fact) — no soft invent empty transfer success
		return IncompleteUnionFactSlice()
	case TermAssignment:
		if rhs.Assign == nil {
			return IncompleteUnionFactSlice()
		}
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs.CommaRHS)
	default:
		return IncompleteUnionFactSlice()
	}
}

// AbstractFactUnionForAssign mirrors FactUnion::abstract_fact_for_assign.
// FactUnion.cpp:121–154 — union-typed LHS transfers fid; union-field write
// records parent fid; padding/packed-after-bitfield → BOTTOM on container.
// Returns (factsOut, lvarCount). Incomplete → IncompleteUnionFactSlice.
func AbstractFactUnionForAssign(
	unionFacts []*FactUnion,
	ptFacts []*FactPointTo,
	lhs *Variable,
	lhsIndir int,
	rhs *Expression,
) (out []*FactUnion, lvarCnt int) {
	if lhs == nil {
		return IncompleteUnionFactSlice(), 0
	}
	if !UnionFactsComplete(unionFacts) || !FactsComplete(ptFacts) {
		return IncompleteUnionFactSlice(), 0
	}
	lvars := MergePointeesOfPointer(lhs.GetCollective(), lhsIndir, ptFacts)
	// incomplete merge at indir>0; indir 0 yields [lhs]
	if !VariablesComplete(lvars) {
		return IncompleteUnionFactSlice(), 0
	}
	lvarCnt = len(lvars)
	if lhs.Type != nil && lhs.Type.IsUnion() {
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs), lvarCnt
	}
	if rhs == nil {
		// non-union LHS with no RHS: complete empty field facts
		return nil, lvarCnt
	}
	for _, v := range lvars {
		// pointees always live; no invent skip nil holes
		if v == nil {
			return IncompleteUnionFactSlice(), lvarCnt
		}
		var fu *FactUnion
		if v.IsUnionField() {
			// FactUnion.cpp:141–143
			fu = MakeFactUnion(v.FieldVarOf, v.GetFieldID())
			// FieldVarOf non-union → MakeFactUnion nil is broken IR
			if fu == nil && v.FieldVarOf != nil {
				return IncompleteUnionFactSlice(), lvarCnt
			}
		} else if v.IsInsideUnionField() {
			// FactUnion.cpp:144–146 — padding or packed-after-bitfield → BOTTOM
			typ := v.Type
			if (typ != nil && typ.HasPadding()) || v.IsPackedAfterBitfield() {
				cu := v.GetContainerUnion()
				if cu != nil {
					fu = MakeFactUnion(cu, FactUnionBottom)
					if fu == nil {
						return IncompleteUnionFactSlice(), lvarCnt
					}
				}
			}
		}
		if fu != nil {
			out = append(out, fu)
		}
	}
	return out, lvarCnt
}
