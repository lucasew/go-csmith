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
	// FactUnion.cpp:163 — assert(v == nullptr || v->type->eType == eUnion) sticky
	// no soft invent FactUnion on scalar/struct vars
	if v != nil && (v.Type == nil || !v.Type.IsUnion()) {
		SetError(ErrGeneric)
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
// Incomplete IR fails closed sticky IncompleteUnionFactSlice (not bare nil —
// UnionFactsComplete(nil)==true invents empty-complete make_facts / soft re-pick).
func MakeFactUnions(vars []*Variable, fid int) []*FactUnion {
	out := make([]*FactUnion, 0, len(vars))
	for _, v := range vars {
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		f := MakeFactUnion(v, fid)
		// non-union subject is assert path — fail closed sticky whole batch
		if f == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// FindRelatedUnion finds FactUnion for union variable v.
// FactUnion* always live; nil hole fails closed (nil — no invent skip to later match).
func FindRelatedUnion(facts []*FactUnion, v *Variable) *FactUnion {
	// subject always live; sticky no invent miss / soft-skip nil key
	if v == nil {
		SetError(ErrGeneric)
		return nil
	}
	for _, f := range facts {
		// FactUnion* always live; sticky no invent skip hole to later match
		if f == nil {
			SetError(ErrGeneric)
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
	// FactUnion.cpp:65 — assert(var->type && eUnion) sticky; fail closed nil if not union
	if f.Var.Type == nil || !f.Var.Type.IsUnion() {
		SetError(ErrGeneric)
		return nil
	}
	fid := f.LastWrittenFID
	// FactUnion.cpp:68–69 — assert fid in [0, field_vars.size()) sticky
	if fid < 0 || fid >= len(f.Var.FieldVars) {
		SetError(ErrGeneric)
		return nil
	}
	fv := f.Var.FieldVars[fid]
	// field Variable* always live; sticky no invent nil type via hole
	if fv == nil {
		SetError(ErrGeneric)
		return nil
	}
	return fv.Type
}

// Output mirrors FactUnion::Output.
// FactUnion.cpp:272–275.
func (f *FactUnion) Output() string {
	if f == nil || f.Var == nil {
		if f != nil && f.Var == nil {
			SetError(ErrGeneric)
		}
		return ""
	}
	// var name always live; sticky no invent " last written field: N" without identifier
	name := f.Var.GetActualName(false)
	if name == "" {
		SetError(ErrGeneric)
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
		// incomplete union map sticky nonreadable (no invent readable past hole)
		SetError(ErrGeneric)
		return true
	}
	// walk to the union field variable
	// FactUnion.cpp:181–184 — for (; !is_union_field(); field_var_of); assert(is_union_field)
	uf := v
	for uf != nil && !uf.IsUnionField() {
		uf = uf.FieldVarOf
	}
	// broken IR (no union field in ancestry) — sticky fail closed nonreadable
	// (no invent readable / soft re-pick past hole)
	if uf == nil || uf.FieldVarOf == nil {
		SetError(ErrGeneric)
		return true
	}
	parent := uf.FieldVarOf
	fid := uf.GetFieldID()
	// incomplete parent FieldVars → GetFieldID -1 → MakeFactUnion fails → nonreadable
	tmp := MakeFactUnion(parent, fid)
	if tmp == nil {
		// parent not union type — sticky fail closed nonreadable
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	fu := FindRelatedUnion(facts, parent)
	if fu == nil || !tmp.Imply(fu) {
		// no fact or last write was a different field → nonreadable (complete analysis)
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
// FactUnion* always live; nil nf or map hole fails closed sticky IncompleteUnionFactSlice
// (no invent empty-complete via UnionFactsComplete(nil) / soft re-pick past wipe).
func MergeUnionFactInto(facts []*FactUnion, nf *FactUnion) []*FactUnion {
	if nf == nil {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if !UnionFactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	for i, old := range facts {
		if old.Var == nf.Var {
			cp := old.Clone()
			if cp == nil {
				SetError(ErrGeneric)
				return IncompleteUnionFactSlice()
			}
			cp.Join(nf)
			facts[i] = cp
			return facts
		}
	}
	cl := nf.Clone()
	if cl == nil {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	return append(facts, cl)
}

// RhsToLhsTransferUnion mirrors FactUnion::rhs_to_lhs_transfer.
// FactUnion.cpp:74–118 — constant→fid 0; variable→join RHS union facts;
// assign/comma peel to RHS; function return uses registry when available.
// Hard IR asserts (nil rhs, non-union lvars, nil Var/Invoke, address-of,
// missing rv_fact) fail closed sticky IncompleteUnionFactSlice. Incomplete
// union/pt maps and MergePointees stay non-sticky for soft re-pick factories.
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
	// FactUnion.cpp:82 — assert(rhs != nullptr) incomplete hole
	// Generation AddParamFacts abstracts missing union args with rhs=nil —
	// non-sticky IncompleteUnion (soft re-pick factories; no invent empty transfer)
	if rhs == nil {
		return IncompleteUnionFactSlice()
	}
	// incomplete input maps — non-sticky hole marker (soft re-pick factories)
	if !UnionFactsComplete(unionFacts) || !FactsComplete(ptFacts) {
		return IncompleteUnionFactSlice()
	}
	// FactUnion.cpp:80–81 — assert all possible LHS are unions
	for _, v := range lvars {
		if v == nil || v.Type == nil || !v.Type.IsUnion() {
			// hard IR sticky — no soft invent transfer onto non-union
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
	}
	switch rhs.Term {
	case TermConstant:
		return MakeFactUnions(lvars, 0)
	case TermVariable:
		if rhs.Var == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// incomplete type IR sticky (no invent level-0 union transfer)
		indirect, iok := rhs.IndirectLevelComplete()
		if !iok {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// FactUnion.cpp:89 — assert(indirect >= 0); hard sticky (no invent clamp for &)
		if indirect < 0 {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		rvars := MergePointeesOfPointer(rhs.Var.GetCollective(), indirect, ptFacts)
		// incomplete pointees — non-sticky abstract hole
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
		// missing Invoke/User/RV or related union fact during generation —
		// non-sticky hole (soft re-pick; no invent fid 0)
		if rhs.Invoke == nil || rhs.Invoke.User == nil || rhs.Invoke.User.RV == nil {
			return IncompleteUnionFactSlice()
		}
		rv := rhs.Invoke.User.RV
		if uf := FindRelatedUnion(unionFacts, rv); uf != nil {
			return MakeFactUnions(lvars, uf.LastWrittenFID)
		}
		// FactUnion.cpp:107 assert(rv_fact) path — non-sticky generation hole
		return IncompleteUnionFactSlice()
	case TermAssignment:
		if rhs.Assign == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs.CommaRHS)
	default:
		// unknown term — non-sticky hole
		return IncompleteUnionFactSlice()
	}
}

// AbstractFactUnionForAssign mirrors FactUnion::abstract_fact_for_assign.
// FactUnion.cpp:121–154 — union-typed LHS transfers fid; union-field write
// records parent fid; padding/packed-after-bitfield → BOTTOM on container.
// Returns (factsOut, lvarCount). Hard IR (nil lhs, nil pointee, MakeFact fail)
// sticky; incomplete maps/MergePointees stay non-sticky hole markers.
func AbstractFactUnionForAssign(
	unionFacts []*FactUnion,
	ptFacts []*FactPointTo,
	lhs *Variable,
	lhsIndir int,
	rhs *Expression,
) (out []*FactUnion, lvarCnt int) {
	if lhs == nil {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice(), 0
	}
	// incomplete maps — non-sticky abstract hole
	if !UnionFactsComplete(unionFacts) || !FactsComplete(ptFacts) {
		return IncompleteUnionFactSlice(), 0
	}
	lvars := MergePointeesOfPointer(lhs.GetCollective(), lhsIndir, ptFacts)
	// incomplete merge at indir>0 — non-sticky; indir 0 yields [lhs]
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
		// pointees always live; nil hole sticky (no invent skip / soft re-pick)
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice(), lvarCnt
		}
		var fu *FactUnion
		if v.IsUnionField() {
			// FactUnion.cpp:141–143
			fu = MakeFactUnion(v.FieldVarOf, v.GetFieldID())
			// FieldVarOf non-union → MakeFactUnion nil is broken IR sticky
			if fu == nil && v.FieldVarOf != nil {
				SetError(ErrGeneric)
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
						SetError(ErrGeneric)
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
