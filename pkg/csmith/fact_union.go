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
	if v != nil {
		if v.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		if !v.Type.IsUnion() {
			// residual ERROR sticky — no invent soft-nil FactUnion past IsUnion residual
			if HasError() {
				return nil
			}
			SetError(ErrGeneric)
			return nil
		}
		// residual ERROR sticky — no invent soft-FactUnion past IsUnion residual true
		if HasError() {
			return nil
		}
	}
	return &FactUnion{Var: v, LastWrittenFID: fid}
}

// MakeFactUnionTop mirrors make_fact(v) with TOP.
// Incomplete MakeFactUnion sticky nil (no invent TOP shell past hole).
func MakeFactUnionTop(v *Variable) *FactUnion {
	f := MakeFactUnion(v, FactUnionTop)
	if f == nil {
		// MakeFactUnion may already sticky non-union/incomplete subject
		if !HasError() {
			SetError(ErrGeneric)
		}
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

// GetVar mirrors FactUnion::get_var.
func (f *FactUnion) GetVar() *Variable {
	if f == nil {
		SetError(ErrGeneric)
		return nil
	}
	return f.Var
}

// SetVar mirrors FactUnion::set_var.
func (f *FactUnion) SetVar(v *Variable) {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	f.Var = v
}

// GetLastWrittenFID mirrors FactUnion::get_last_written_fid.
func (f *FactUnion) GetLastWrittenFID() int {
	if f == nil {
		SetError(ErrGeneric)
		return FactUnionBottom
	}
	return f.LastWrittenFID
}

// IsRelated mirrors Fact::is_related for Union — same subject var.
func (f *FactUnion) IsRelated(other *FactUnion) bool {
	if f == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	return f.Var == other.Var
}

// IsTop mirrors FactUnion::is_top.
// Incomplete FactUnion sticky false (no invent TOP / soft re-pick past hole).
func (f *FactUnion) IsTop() bool {
	// FactUnion always live; sticky incomplete no invent TOP soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	return f.LastWrittenFID == FactUnionTop
}

// IsBottom mirrors FactUnion::is_bottom.
// Incomplete FactUnion sticky false (no invent BOTTOM / soft re-pick past hole).
func (f *FactUnion) IsBottom() bool {
	// FactUnion always live; sticky incomplete no invent BOTTOM soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	return f.LastWrittenFID == FactUnionBottom
}

// SetTop mirrors FactUnion::set_top.
// Incomplete FactUnion sticky no-op (no invent soft-set TOP past missing shell).
func (f *FactUnion) SetTop() {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	f.LastWrittenFID = FactUnionTop
}

// SetBottom mirrors FactUnion::set_bottom.
// Incomplete FactUnion sticky no-op (no invent soft-set BOTTOM past missing shell).
func (f *FactUnion) SetBottom() {
	if f == nil {
		SetError(ErrGeneric)
		return
	}
	f.LastWrittenFID = FactUnionBottom
}

// Clone mirrors FactUnion::clone.
// Incomplete FactUnion sticky nil (no invent empty clone shell past hole).
func (f *FactUnion) Clone() *FactUnion {
	// FactUnion always live; sticky incomplete no invent nil clone soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return nil
	}
	return &FactUnion{Var: f.Var, LastWrittenFID: f.LastWrittenFID}
}

// Equal mirrors FactUnion::equal.
// FactUnion.cpp:195–201.
// Incomplete FactUnion sticky false (no invent not-equal / soft re-pick past holes).
func (f *FactUnion) Equal(other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent not-equal
	if f == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// Imply mirrors FactUnion::imply.
// FactUnion.cpp:249–259 — bottom implies all; equal fid implies; else false.
// Incomplete FactUnion sticky false (no invent not-imply / soft re-pick past holes).
func (f *FactUnion) Imply(other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent not-imply
	if f == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	if f.IsBottom() {
		// residual ERROR sticky — no invent soft-imply past IsBottom residual true
		if HasError() {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past IsBottom residual false
	if HasError() {
		return false
	}
	if other.IsBottom() {
		// residual ERROR sticky — no invent soft-imply past other IsBottom residual true
		if HasError() {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue past other IsBottom residual false
	if HasError() {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// Join mirrors FactUnion::join — merge lattice; return true if changed.
// FactUnion.cpp:207–221.
// Incomplete FactUnion sticky false (no invent join no-op / soft re-pick past holes).
func (f *FactUnion) Join(other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent join no-op success
	if f == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	if f.Imply(other) {
		// residual ERROR sticky — no invent join no-op true past Imply hole
		if HasError() {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue join past Imply residual false path
	if HasError() {
		return false
	}
	if other.Imply(f) {
		// residual ERROR sticky — no invent absorb past other.Imply hole
		if HasError() {
			return false
		}
		f.LastWrittenFID = other.LastWrittenFID
	} else {
		// residual ERROR sticky — no invent soft-set BOTTOM past other.Imply residual false
		if HasError() {
			return false
		}
		f.SetBottom()
	}
	return true
}

// GetLastWrittenType mirrors FactUnion::get_last_written_type.
// FactUnion.cpp:63–71.
// FactUnion + Var always live; sticky nil (no invent soft-skip past hole).
// Top/bottom are complete no-type (not incomplete IR).
func (f *FactUnion) GetLastWrittenType() *Type {
	if f == nil || f.Var == nil {
		SetError(ErrGeneric)
		return nil
	}
	isTop := f.IsTop()
	// residual ERROR sticky — no invent soft-nil type past IsTop residual
	if HasError() {
		return nil
	}
	isBot := f.IsBottom()
	// residual ERROR sticky — no invent soft-nil type past IsBottom residual
	if HasError() {
		return nil
	}
	if isTop || isBot {
		return nil
	}
	// FactUnion.cpp:65 — assert(var->type && eUnion) sticky; fail closed nil if not union
	if f.Var.Type == nil {
		SetError(ErrGeneric)
		return nil
	}
	if !f.Var.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-nil type past IsUnion residual
		if HasError() {
			return nil
		}
		SetError(ErrGeneric)
		return nil
	}
	// residual ERROR sticky — no invent soft-type past IsUnion residual true
	if HasError() {
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
	// residual ERROR sticky — no invent soft-empty union fact past GetActualName residual
	if HasError() {
		return ""
	}
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

// CloneUnionFactSlice shallow-copies a complete FactUnion* vector (C++ Fact* vector copy).
// FunctionInvocationUser.cpp:206 — global_facts = caller_fm->global_facts includes eUnionWrite.
// Incomplete maps fail closed sticky IncompleteUnionFactSlice (no invent cleaned partial).
func CloneUnionFactSlice(facts []*FactUnion) []*FactUnion {
	if facts == nil {
		return nil
	}
	if !UnionFactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, len(facts))
	copy(out, facts)
	return out
}

// RenewUnionFact mirrors renew_fact for FactUnion (Fact.cpp:178–201).
// Related subject replaced; else append. Incomplete maps fail closed sticky wipe.
func RenewUnionFact(facts *[]*FactUnion, nf *FactUnion) bool {
	if facts == nil {
		SetError(ErrGeneric)
		return false
	}
	if nf == nil || nf.Var == nil {
		*facts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*facts) {
		*facts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return false
	}
	for i, f := range *facts {
		if f.Var == nf.Var {
			if f.Equal(nf) {
				if HasError() {
					*facts = IncompleteUnionFactSlice()
					return false
				}
				return false
			}
			if HasError() {
				*facts = IncompleteUnionFactSlice()
				return false
			}
			(*facts)[i] = nf
			return true
		}
	}
	*facts = append(*facts, nf)
	return true
}

// RenewUnionFacts mirrors renew_facts for FactUnion (Fact.cpp:222–229).
// FunctionInvocationUser.cpp:221 — renew_facts(caller, ret_facts) includes eUnionWrite.
func RenewUnionFacts(facts *[]*FactUnion, newFacts []*FactUnion) bool {
	if facts == nil {
		SetError(ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*facts) || !UnionFactsComplete(newFacts) {
		*facts = IncompleteUnionFactSlice()
		SetError(ErrGeneric)
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if RenewUnionFact(facts, nf) {
			if HasError() {
				*facts = IncompleteUnionFactSlice()
				return false
			}
			changed = true
		} else if HasError() {
			*facts = IncompleteUnionFactSlice()
			return false
		}
	}
	return changed
}

// GlobalUnionFactsOnly keeps FactUnion subjects that are global.
// FactMgr::remove_function_local_facts drops stack/other-RV; ret_facts for renew
// are body map_facts_out after that filter — global union last-writes remain.
func GlobalUnionFactsOnly(facts []*FactUnion) []*FactUnion {
	if facts == nil {
		return nil
	}
	if !UnionFactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, 0, len(facts))
	for _, f := range facts {
		isG := f.Var.IsGlobal()
		if HasError() {
			return IncompleteUnionFactSlice()
		}
		if isG {
			out = append(out, f)
		}
	}
	return out
}

// IsFieldReadable mirrors FactUnion::is_field_readable.
// FactUnion.cpp:262–270.
// Incomplete facts fail closed sticky false (no invent readable past UnionFacts hole).
func IsFieldReadable(v *Variable, fid int, facts []*FactUnion) bool {
	// subject always live union; incomplete subject sticky not-readable
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	if !v.Type.IsUnion() || fid < 0 {
		// residual ERROR sticky — no invent not-readable soft-skip past IsUnion residual
		if HasError() {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue readable past IsUnion residual true
	if HasError() {
		return false
	}
	if !UnionFactsComplete(facts) {
		// incomplete map sticky (no invent readable / soft re-pick past hole)
		SetError(ErrGeneric)
		return false
	}
	if fid >= len(v.Type.Fields) && fid >= len(v.FieldVars) {
		return false
	}
	tmp := MakeFactUnion(v, fid)
	if tmp == nil {
		// MakeFactUnion may already sticky non-union; ensure sticky
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	fu := FindRelatedUnion(facts, v)
	// residual ERROR sticky — no invent readable/not-readable soft-skip past FindRelated hole
	if HasError() {
		return false
	}
	if fu == nil {
		return false
	}
	ok := tmp.Imply(fu)
	// residual ERROR sticky — no invent readable true past Imply hole
	if HasError() {
		return false
	}
	return ok
}

// IsNonreadableField mirrors FactUnion::is_nonreadable_field.
// FactUnion.cpp:178–192 — inside union field: nonreadable if no related FactUnion
// or last-write does not imply this field (fu == nullptr || !tmp.imply(*fu)).
// Empty complete facts ⇒ no related fact ⇒ nonreadable (same as C++ find_related null).
// Variable always live; sticky nonreadable (no invent readable soft-skip past hole).
// Incomplete FactUnion maps fail closed nonreadable (no invent readable while
// FindRelatedUnion returns nil past a hole before a matching parent fact).
func IsNonreadableField(v *Variable, facts []*FactUnion) bool {
	if v == nil {
		SetError(ErrGeneric)
		return true
	}
	if !v.IsInsideUnionField() {
		// residual ERROR sticky — no invent not-nonreadable soft-skip past IsInsideUnionField hole
		if HasError() {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue nonreadable past IsInsideUnion residual true path
	if HasError() {
		return true
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
		// residual ERROR sticky — no invent soft-continue walk past IsUnionField residual
		if HasError() {
			return true
		}
		uf = uf.FieldVarOf
	}
	// residual ERROR sticky — no invent soft-continue past final IsUnionField residual
	if HasError() {
		return true
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
	// residual ERROR sticky — no invent soft-continue readable past FindRelatedUnion hole
	if HasError() {
		return true
	}
	if fu == nil {
		// no fact → nonreadable (complete analysis)
		return true
	}
	ok := tmp.Imply(fu)
	// residual ERROR sticky — no invent soft-continue readable past Imply hole
	if HasError() {
		return true
	}
	if !ok {
		// last write was a different field → nonreadable
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
		// residual ERROR sticky — no invent soft-continue join past FindRelated hole
		if HasError() {
			return nil
		}
		if exist == nil {
			continue
		}
		if fu == nil {
			fu = exist.Clone()
			// residual ERROR sticky — no invent soft-continue past Clone hole
			if HasError() || fu == nil {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return nil
			}
			continue
		}
		// hack: both must share var identity for join — set to exist's var
		fu.Var = exist.Var
		fu.Join(exist)
		// residual ERROR sticky — no invent soft-continue partial join past Join hole
		if HasError() {
			return nil
		}
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
			// residual ERROR sticky — no invent soft-merge past Clone residual
			if HasError() {
				return IncompleteUnionFactSlice()
			}
			if cp == nil {
				SetError(ErrGeneric)
				return IncompleteUnionFactSlice()
			}
			cp.Join(nf)
			// residual ERROR sticky — no invent soft-merge past Join residual
			if HasError() {
				return IncompleteUnionFactSlice()
			}
			facts[i] = cp
			return facts
		}
	}
	cl := nf.Clone()
	// residual ERROR sticky — no invent soft-append past Clone residual
	if HasError() {
		return IncompleteUnionFactSlice()
	}
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
		if v == nil || v.Type == nil {
			// hard IR sticky — no soft invent transfer onto non-union
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if !v.Type.IsUnion() {
			// residual ERROR sticky — no invent soft-transfer past IsUnion residual
			if HasError() {
				return IncompleteUnionFactSlice()
			}
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// residual ERROR sticky — no invent soft-continue later lvars past IsUnion residual true
		if HasError() {
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
		// FactUnion.cpp:89 assert(indirect >= 0) is NDEBUG-elided in Release.
		// merge_pointees_of_pointer with indirect<=0 returns {collective} (while loop
		// never runs). Fair: allow &expr (indirect==-1) as pointee set {coll}; only
		// multi-level & (indirect < -1) is sticky broken IR.
		if indirect < -1 {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		coll := rhs.Var.GetCollective()
		// residual ERROR sticky — no invent soft-merge past GetCollective residual
		if HasError() {
			return IncompleteUnionFactSlice()
		}
		// indirect<=0 → MergePointeesOfPointer no-ops to {coll}; pass 0 for that path
		mergeLevel := indirect
		if mergeLevel < 0 {
			mergeLevel = 0
		}
		rvars := MergePointeesOfPointer(coll, mergeLevel, ptFacts)
		// residual ERROR sticky — no invent soft-merge past MergePointees residual
		if HasError() {
			return IncompleteUnionFactSlice()
		}
		// incomplete pointees — non-sticky abstract hole
		if !VariablesComplete(rvars) {
			return IncompleteUnionFactSlice()
		}
		rhsFact := JoinVarFactsUnion(unionFacts, rvars)
		// residual ERROR sticky — no invent soft-empty transfer past JoinVarFacts residual
		if HasError() {
			return IncompleteUnionFactSlice()
		}
		if rhsFact == nil {
			// complete: no related RHS fact → empty transfer
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
		uf := FindRelatedUnion(unionFacts, rv)
		// residual ERROR sticky — no invent soft-union transfer past FindRelatedUnion residual
		if HasError() {
			return IncompleteUnionFactSlice()
		}
		if uf != nil {
			return MakeFactUnions(lvars, uf.LastWrittenFID)
		}
		// FactUnion.cpp:107 assert(rv_fact) path — non-sticky generation hole
		return IncompleteUnionFactSlice()
	case TermAssignment:
		// FactUnion.cpp:110–112 — peel embedded assign RHS
		// incomplete Assign/Expr sticky — no invent empty/non-sticky via nil-rhs peel
		// (generation AddParamFacts missing-arg path is bare rhs=nil, not compound shell)
		if rhs.Assign == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if rhs.Assign.Expr == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		return RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		// FactUnion.cpp:113–115 — peel comma RHS
		// incomplete CommaRHS sticky — no invent empty/non-sticky via nil-rhs peel
		if rhs.CommaRHS == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
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
	coll := lhs.GetCollective()
	// residual ERROR sticky — no invent soft-abstract union past GetCollective residual
	if HasError() {
		return IncompleteUnionFactSlice(), 0
	}
	lvars := MergePointeesOfPointer(coll, lhsIndir, ptFacts)
	// residual ERROR sticky — no invent soft-abstract union past MergePointees residual
	if HasError() {
		return IncompleteUnionFactSlice(), 0
	}
	// incomplete merge at indir>0 — non-sticky; indir 0 yields [lhs]
	if !VariablesComplete(lvars) {
		return IncompleteUnionFactSlice(), 0
	}
	lvarCnt = len(lvars)
	// FactUnion.cpp:129 — lhs->get_type() always live Type&; Type-nil shell sticky
	// (no invent non-union complete transfer soft-skip past incomplete LHS type)
	// Special null/garbage/tbd have Type nil by design — complete non-union path.
	if lhs.Type == nil {
		if !IsSpecialPtr(lhs) {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice(), lvarCnt
		}
	} else if lhs.Type.IsUnion() {
		// residual ERROR sticky — no invent union transfer past IsUnion residual hole
		if HasError() {
			return IncompleteUnionFactSlice(), lvarCnt
		}
		out := RhsToLhsTransferUnion(unionFacts, ptFacts, lvars, rhs)
		// residual ERROR sticky — no invent soft-continue transfer past union transfer residual
		if HasError() {
			return IncompleteUnionFactSlice(), lvarCnt
		}
		return out, lvarCnt
	} else if HasError() {
		// residual ERROR sticky — no invent non-union transfer past IsUnion residual false
		return IncompleteUnionFactSlice(), lvarCnt
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
			// residual ERROR sticky — no invent soft-continue transfer past IsUnionField hole
			if HasError() {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FactUnion.cpp:141–143
			fid := v.GetFieldID()
			// residual ERROR sticky — no invent soft-union fact past GetFieldID residual
			if HasError() {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			fu = MakeFactUnion(v.FieldVarOf, fid)
			// residual ERROR sticky — no invent soft-continue past MakeFactUnion residual
			if HasError() {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FieldVarOf non-union → MakeFactUnion nil is broken IR sticky
			if fu == nil && v.FieldVarOf != nil {
				SetError(ErrGeneric)
				return IncompleteUnionFactSlice(), lvarCnt
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-continue IsInside path past IsUnionField residual false
			return IncompleteUnionFactSlice(), lvarCnt
		} else if v.IsInsideUnionField() {
			// residual ERROR sticky — no invent soft-continue transfer past IsInsideUnionField hole
			if HasError() {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FactUnion.cpp:144–146 — v->type->has_padding() always live Type*
			// Type-nil non-special sticky (no invent skip BOTTOM / soft re-pick)
			typ := v.Type
			if typ == nil {
				if !IsSpecialPtr(v) {
					SetError(ErrGeneric)
					return IncompleteUnionFactSlice(), lvarCnt
				}
			} else if typ.HasPadding() || v.IsPackedAfterBitfield() {
				// residual ERROR sticky — no invent soft-continue padding path past IsPacked residual
				if HasError() {
					return IncompleteUnionFactSlice(), lvarCnt
				}
				cu := v.GetContainerUnion()
				// residual ERROR sticky — no invent soft-skip container past GetContainerUnion residual
				if HasError() {
					return IncompleteUnionFactSlice(), lvarCnt
				}
				if cu != nil {
					fu = MakeFactUnion(cu, FactUnionBottom)
					// residual ERROR sticky — no invent soft-continue past MakeFactUnion residual
					if HasError() {
						return IncompleteUnionFactSlice(), lvarCnt
					}
					if fu == nil {
						SetError(ErrGeneric)
						return IncompleteUnionFactSlice(), lvarCnt
					}
				}
			} else if HasError() {
				// residual ERROR sticky — no invent soft-skip no-padding past IsPacked residual false
				return IncompleteUnionFactSlice(), lvarCnt
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-skip not-inside past IsInside residual false
			return IncompleteUnionFactSlice(), lvarCnt
		}
		if fu != nil {
			out = append(out, fu)
		}
	}
	return out, lvarCnt
}
