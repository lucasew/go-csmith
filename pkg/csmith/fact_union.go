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
func MakeFactUnionSess(s *Session, v *Variable, fid int) *FactUnion {
	// FactUnion.cpp:163 — assert(v == nullptr || v->type->eType == eUnion) sticky
	// no soft invent FactUnion on scalar/struct vars
	if v != nil {
		if v.Type == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if !v.Type.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-nil FactUnion past IsUnion residual
			if sessHasError(s) {
				return nil
			}
			sessNoteError(s, ErrGeneric)
			return nil
		}
		// residual ERROR sticky — no invent soft-FactUnion past IsUnion residual true
		if sessHasError(s) {
			return nil
		}
	}
	return &FactUnion{Var: v, LastWrittenFID: fid}
}

// MakeFactUnionTop mirrors make_fact(v) with TOP.
// Incomplete MakeFactUnion sticky nil (no invent TOP shell past hole).}

func MakeFactUnionTopSess(s *Session, v *Variable) *FactUnion {
	f := MakeFactUnionSess(s, v, FactUnionTop)
	if f == nil {
		// MakeFactUnion may already sticky non-union/incomplete subject
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return nil
	}
	return f
}

// MakeFactUnions mirrors FactUnion::make_facts.
// FactUnion.cpp:169–176 — vars[i] always live; make_fact asserts union type.
// Incomplete IR fails closed sticky IncompleteUnionFactSlice (not bare nil —
// UnionFactsComplete(nil)==true invents empty-complete make_facts / soft re-pick).
func MakeFactUnionsSess(s *Session, vars []*Variable, fid int) []*FactUnion {
	out := make([]*FactUnion, 0, len(vars))
	for _, v := range vars {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		f := MakeFactUnionSess(s, v, fid)
		// non-union subject is assert path — fail closed sticky whole batch
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// FindRelatedUnion finds FactUnion for union variable v.
// FactUnion* always live; nil hole fails closed (nil — no invent skip to later match).
func FindRelatedUnionSess(s *Session, facts []*FactUnion, v *Variable) *FactUnion {
	// subject always live; sticky no invent miss / soft-skip nil key
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	for _, f := range facts {
		// FactUnion* always live; sticky no invent skip hole to later match
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if f.Var == v {
			return f
		}
	}
	return nil
}

// GetVar mirrors FactUnion::get_var.
func (f *FactUnion) GetVarSess(s *Session) *Variable {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f.Var
}

// SetVar mirrors FactUnion::set_var.
func (f *FactUnion) SetVarSess(s *Session, v *Variable) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.Var = v
}

// GetLastWrittenFID mirrors FactUnion::get_last_written_fid.
func (f *FactUnion) GetLastWrittenFIDSess(s *Session) int {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return FactUnionBottom
	}
	return f.LastWrittenFID
}

// IsRelated mirrors Fact::is_related for Union — same subject var.
func (f *FactUnion) IsRelatedSess(s *Session, other *FactUnion) bool {
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.Var == other.Var
}

// IsTop mirrors FactUnion::is_top.
// Incomplete FactUnion sticky false (no invent TOP / soft re-pick past hole).}

func (f *FactUnion) IsTopSess(s *Session) bool {
	// FactUnion always live; sticky incomplete no invent TOP soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.LastWrittenFID == FactUnionTop
}

// IsBottom mirrors FactUnion::is_bottom.
// Incomplete FactUnion sticky false (no invent BOTTOM / soft re-pick past hole).
func (f *FactUnion) IsBottomSess(s *Session) bool {
	// FactUnion always live; sticky incomplete no invent BOTTOM soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.LastWrittenFID == FactUnionBottom
}

// SetTop mirrors FactUnion::set_top.
// Incomplete FactUnion sticky no-op (no invent soft-set TOP past missing shell).
func (f *FactUnion) SetTopSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.LastWrittenFID = FactUnionTop
}

// SetBottom mirrors FactUnion::set_bottom.
// Incomplete FactUnion sticky no-op (no invent soft-set BOTTOM past missing shell).
func (f *FactUnion) SetBottomSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.LastWrittenFID = FactUnionBottom
}

// Clone mirrors FactUnion::clone.
// Incomplete FactUnion sticky nil (no invent empty clone shell past hole).
func (f *FactUnion) CloneSess(s *Session) *FactUnion {
	// FactUnion always live; sticky incomplete no invent nil clone soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return &FactUnion{Var: f.Var, LastWrittenFID: f.LastWrittenFID}
}

// Equal mirrors FactUnion::equal.
// FactUnion.cpp:195–201.
// Incomplete FactUnion sticky false (no invent not-equal / soft re-pick past holes).
func (f *FactUnion) EqualSess(s *Session, other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent not-equal
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
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
func (f *FactUnion) ImplySess(s *Session, other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent not-imply
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	if f.IsBottomSess(s) {
		// residual ERROR sticky — no invent soft-imply past IsBottom residual true
		if sessHasError(s) {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue past IsBottom residual false
	if sessHasError(s) {
		return false
	}
	if other.IsBottomSess(s) {
		// residual ERROR sticky — no invent soft-imply past other IsBottom residual true
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue past other IsBottom residual false
	if sessHasError(s) {
		return false
	}
	return f.LastWrittenFID == other.LastWrittenFID
}

// Join mirrors FactUnion::join — merge lattice; return true if changed.
// FactUnion.cpp:207–221.
// Incomplete FactUnion sticky false (no invent join no-op / soft re-pick past holes).
func (f *FactUnion) JoinSess(s *Session, other *FactUnion) bool {
	// both FactUnion* always live; sticky incomplete no invent join no-op success
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	if f.ImplySess(s, other) {
		// residual ERROR sticky — no invent join no-op true past Imply hole
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue join past Imply residual false path
	if sessHasError(s) {
		return false
	}
	if other.ImplySess(s, f) {
		// residual ERROR sticky — no invent absorb past other.Imply hole
		if sessHasError(s) {
			return false
		}
		f.LastWrittenFID = other.LastWrittenFID
	} else {
		// residual ERROR sticky — no invent soft-set BOTTOM past other.Imply residual false
		if sessHasError(s) {
			return false
		}
		f.SetBottomSess(s)
	}
	return true
}

// GetLastWrittenType mirrors FactUnion::get_last_written_type.
// FactUnion.cpp:63–71.
// FactUnion + Var always live; sticky nil (no invent soft-skip past hole).
// Top/bottom are complete no-type (not incomplete IR).}

func (f *FactUnion) GetLastWrittenTypeSess(s *Session) *Type {
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	isTop := f.IsTopSess(s)
	// residual ERROR sticky — no invent soft-nil type past IsTop residual
	if sessHasError(s) {
		return nil
	}
	isBot := f.IsBottomSess(s)
	// residual ERROR sticky — no invent soft-nil type past IsBottom residual
	if sessHasError(s) {
		return nil
	}
	if isTop || isBot {
		return nil
	}
	// FactUnion.cpp:65 — assert(var->type && eUnion) sticky; fail closed nil if not union
	if f.Var.Type == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if !f.Var.Type.IsUnionSess(s) {
		// residual ERROR sticky — no invent soft-nil type past IsUnion residual
		if sessHasError(s) {
			return nil
		}
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// residual ERROR sticky — no invent soft-type past IsUnion residual true
	if sessHasError(s) {
		return nil
	}
	fid := f.LastWrittenFID
	// FactUnion.cpp:68–69 — assert fid in [0, field_vars.size()) sticky
	if fid < 0 || fid >= len(f.Var.FieldVars) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	fv := f.Var.FieldVars[fid]
	// field Variable* always live; sticky no invent nil type via hole
	if fv == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return fv.Type
}

// Output mirrors FactUnion::Output.
// FactUnion.cpp:272–275.}

func (f *FactUnion) OutputSess(s *Session) string {
	if f == nil || f.Var == nil {
		if f != nil && f.Var == nil {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// var name always live; sticky no invent " last written field: N" without identifier
	name := f.Var.GetActualNameSess(s, false)
	// residual ERROR sticky — no invent soft-empty union fact past GetActualName residual
	if sessHasError(s) {
		return ""
	}
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return name + " last written field: " + strconv.Itoa(f.LastWrittenFID)
}

// OutputAssertion mirrors FactUnion::OutputAssertion — empty body.
// FactUnion.h:97–98 — override is a no-op (never prints assert lines).
// Callers still emit output_tab before the virtual call (FactMgr.cpp:643–645),
// which leaves orphan indent before the next line (seed-3 `    }` after return).
func (f *FactUnion) OutputAssertionSess(s *Session, stParent *Block, indent string) string {
	_ = s
	_ = stParent
	_ = indent
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return ""
}

// CombineUnionFactsSess mirrors combine_facts for eUnionWrite (Fact.cpp:225–236).
// Fact::join_visits defaults to join for FactUnion (Fact.h:69). Only updates
// subjects already present in facts1; does not append facts2-only subjects.
func CombineUnionFactsSess(s *Session, facts *[]*FactUnion, facts2 []*FactUnion) {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !UnionFactsComplete(*facts) || !UnionFactsComplete(facts2) {
		*facts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, nf := range facts2 {
		for i, old := range *facts {
			if old.Var != nf.Var {
				continue
			}
			cp := old.CloneSess(s)
			if cp == nil || sessHasError(s) {
				*facts = IncompleteUnionFactSlice()
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return
			}
			_ = cp.JoinSess(s, nf)
			if sessHasError(s) {
				*facts = IncompleteUnionFactSlice()
				return
			}
			(*facts)[i] = cp
			break
		}
	}
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
func CloneUnionFactSliceSess(s *Session, facts []*FactUnion) []*FactUnion {
	if facts == nil {
		return nil
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, len(facts))
	copy(out, facts)
	return out
}

// CloneUnionFactSliceDeep copies FactUnion objects (subject pointer shared, lattice value owned).
// Use for pre-visit snapshots where visit may Join/SetBottom in place on live facts;
// shallow CloneUnionFactSlice would alias and observe post-visit last_written_fid.
// Incomplete maps fail closed sticky IncompleteUnionFactSlice.
func CloneUnionFactSliceDeepSess(s *Session, facts []*FactUnion) []*FactUnion {
	if facts == nil {
		return nil
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, len(facts))
	for i, f := range facts {
		cp := f.CloneSess(s)
		if cp == nil || sessHasError(s) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteUnionFactSlice()
		}
		out[i] = cp
	}
	return out
}

// RenewUnionFact mirrors renew_fact for FactUnion (Fact.cpp:178–201).
// Related subject replaced; else append. Incomplete maps fail closed sticky wipe.}

func RenewUnionFactSess(s *Session, facts *[]*FactUnion, nf *FactUnion) bool {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if nf == nil || nf.Var == nil {
		*facts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*facts) {
		*facts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	for i, f := range *facts {
		if f.Var == nf.Var {
			if f.EqualSess(s, nf) {
				if sessHasError(s) {
					*facts = IncompleteUnionFactSlice()
					return false
				}
				return false
			}
			if sessHasError(s) {
				*facts = IncompleteUnionFactSlice()
				return false
			}
			(*facts)[i] = nf
			return true
		}
	}
	// Fact.cpp:185–188 — if not found, push_back(new_fact)
	*facts = append(*facts, nf)
	return true
}

// RenewUnionFacts mirrors renew_facts for FactUnion (Fact.cpp:222–229).
// FunctionInvocationUser.cpp:221 — renew_facts(caller, ret_facts) includes eUnionWrite.}

func RenewUnionFactsSess(s *Session, facts *[]*FactUnion, newFacts []*FactUnion) bool {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*facts) || !UnionFactsComplete(newFacts) {
		*facts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if RenewUnionFactSess(s, facts, nf) {
			if sessHasError(s) {
				*facts = IncompleteUnionFactSlice()
				return false
			}
			changed = true
		} else if sessHasError(s) {
			*facts = IncompleteUnionFactSlice()
			return false
		}
	}
	return changed
}

// GlobalUnionFactsOnly keeps FactUnion subjects that are global.
// FactMgr::remove_function_local_facts drops stack/other-RV; ret_facts for renew
// are body map_facts_out after that filter — global union last-writes remain.}

func GlobalUnionFactsOnlySess(s *Session, facts []*FactUnion) []*FactUnion {
	if facts == nil {
		return nil
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, 0, len(facts))
	for _, f := range facts {
		isG := f.Var.IsGlobalSess(s)
		if sessHasError(s) {
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
func IsFieldReadableSess(s *Session, v *Variable, fid int, facts []*FactUnion) bool {
	// subject always live union; incomplete subject sticky not-readable
	if v == nil || v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !v.Type.IsUnionSess(s) || fid < 0 {
		// residual ERROR sticky — no invent not-readable soft-skip past IsUnion residual
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue readable past IsUnion residual true
	if sessHasError(s) {
		return false
	}
	if !UnionFactsComplete(facts) {
		// incomplete map sticky (no invent readable / soft re-pick past hole)
		sessNoteError(s, ErrGeneric)
		return false
	}
	if fid >= len(v.Type.Fields) && fid >= len(v.FieldVars) {
		return false
	}
	tmp := MakeFactUnionSess(s, v, fid)
	if tmp == nil {
		// MakeFactUnion may already sticky non-union; ensure sticky
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return false
	}
	fu := FindRelatedUnionSess(s, facts, v)
	// residual ERROR sticky — no invent readable/not-readable soft-skip past FindRelated hole
	if sessHasError(s) {
		return false
	}
	if fu == nil {
		return false
	}
	ok := tmp.ImplySess(s, fu)
	// residual ERROR sticky — no invent readable true past Imply hole
	if sessHasError(s) {
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
// FindRelatedUnion returns nil past a hole before a matching parent fact).}

func IsNonreadableFieldSess(s *Session, v *Variable, facts []*FactUnion) bool {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !v.IsInsideUnionFieldSess(s) {
		// residual ERROR sticky — no invent not-nonreadable soft-skip past IsInsideUnionField hole
		if sessHasError(s) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue nonreadable past IsInsideUnion residual true path
	if sessHasError(s) {
		return true
	}
	if !UnionFactsComplete(facts) {
		// incomplete union map sticky nonreadable (no invent readable past hole)
		sessNoteError(s, ErrGeneric)
		return true
	}
	// walk to the union field variable
	// FactUnion.cpp:181–184 — for (; !is_union_field(); field_var_of); assert(is_union_field)
	uf := v
	for uf != nil && !uf.IsUnionFieldSess(s) {
		// residual ERROR sticky — no invent soft-continue walk past IsUnionField residual
		if sessHasError(s) {
			return true
		}
		uf = uf.FieldVarOf
	}
	// residual ERROR sticky — no invent soft-continue past final IsUnionField residual
	if sessHasError(s) {
		return true
	}
	// broken IR (no union field in ancestry) — sticky fail closed nonreadable
	// (no invent readable / soft re-pick past hole)
	if uf == nil || uf.FieldVarOf == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	parent := uf.FieldVarOf
	fid := uf.GetFieldIDSess(s)
	// incomplete parent FieldVars → GetFieldID -1 → MakeFactUnion fails → nonreadable
	tmp := MakeFactUnionSess(s, parent, fid)
	if tmp == nil {
		// parent not union type — sticky fail closed nonreadable
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return true
	}
	fu := FindRelatedUnionSess(s, facts, parent)
	// residual ERROR sticky — no invent soft-continue readable past FindRelatedUnion hole
	if sessHasError(s) {
		return true
	}
	if fu == nil {
		// no fact → nonreadable (complete analysis)
		return true
	}
	ok := tmp.ImplySess(s, fu)
	// residual ERROR sticky — no invent soft-continue readable past Imply hole
	if sessHasError(s) {
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
// RhsToLhsTransferUnion pre-checks completeness before join).}

func JoinVarFactsUnionSess(s *Session, facts []*FactUnion, vars []*Variable) *FactUnion {
	// incomplete union map / vars fails closed sticky (no invent soft nil join success path)
	if !UnionFactsComplete(facts) || !VariablesComplete(vars) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	var fu *FactUnion
	for _, v := range vars {
		exist := FindRelatedUnionSess(s, facts, v)
		// residual ERROR sticky — no invent soft-continue join past FindRelated hole
		if sessHasError(s) {
			return nil
		}
		if exist == nil {
			continue
		}
		if fu == nil {
			fu = exist.CloneSess(s)
			// residual ERROR sticky — no invent soft-continue past Clone hole
			if sessHasError(s) || fu == nil {
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return nil
			}
			continue
		}
		// hack: both must share var identity for join — set to exist's var
		fu.Var = exist.Var
		fu.JoinSess(s, exist)
		// residual ERROR sticky — no invent soft-continue partial join past Join hole
		if sessHasError(s) {
			return nil
		}
	}
	return fu
}

// MergeUnionFactInto merges nf into facts slice (join if related).
// Fact.cpp:149–171 merge_fact — same as MergeUnionFact (imply short-circuit,
// copy=new.clone(); copy.join(old)). Soft invent always-joined old.clone().join(new)
// without imply short-circuit (see call_analysis comment at if-combine).}

func MergeUnionFactIntoSess(s *Session, facts []*FactUnion, nf *FactUnion) []*FactUnion {
	return MergeUnionFactSess(s, facts, nf)
}

// RhsToLhsTransferUnion mirrors FactUnion::rhs_to_lhs_transfer.
// FactUnion.cpp:74–118 — constant→fid 0; variable→join RHS union facts;
// assign/comma peel to RHS; function return uses registry when available.
// Hard IR asserts (nil rhs, non-union lvars, nil Var/Invoke, address-of,
// missing rv_fact) fail closed sticky IncompleteUnionFactSlice. Incomplete
// union/pt maps and MergePointees stay non-sticky for soft re-pick factories.
func RhsToLhsTransferUnionSess(s *Session,
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
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if !v.Type.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-transfer past IsUnion residual
			if sessHasError(s) {
				return IncompleteUnionFactSlice()
			}
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// residual ERROR sticky — no invent soft-continue later lvars past IsUnion residual true
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
	}
	switch rhs.Term {
	case TermConstant:
		return MakeFactUnionsSess(s, lvars, 0)
	case TermVariable:
		if rhs.Var == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// incomplete type IR sticky (no invent level-0 union transfer)
		indirect, iok := rhs.IndirectLevelCompleteSess(s)
		if !iok {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		// FactUnion.cpp:89 assert(indirect >= 0) is NDEBUG-elided in Release.
		// merge_pointees_of_pointer with indirect<=0 returns {collective} (while loop
		// never runs). Fair: allow &expr (indirect==-1) as pointee set {coll}; only
		// multi-level & (indirect < -1) is sticky broken IR.
		if indirect < -1 {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		coll := rhs.Var.GetCollectiveSess(s)
		// residual ERROR sticky — no invent soft-merge past GetCollective residual
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		// indirect<=0 → MergePointeesOfPointer no-ops to {coll}; pass 0 for that path
		mergeLevel := indirect
		if mergeLevel < 0 {
			mergeLevel = 0
		}
		rvars := MergePointeesOfPointerSess(s, coll, mergeLevel, ptFacts)
		// residual ERROR sticky — no invent soft-merge past MergePointees residual
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		// incomplete pointees — non-sticky abstract hole
		if !VariablesComplete(rvars) {
			return IncompleteUnionFactSlice()
		}
		rhsFact := JoinVarFactsUnionSess(s, unionFacts, rvars)
		// residual ERROR sticky — no invent soft-empty transfer past JoinVarFacts residual
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		if rhsFact == nil {
			// complete: no related RHS fact → empty transfer
			return nil
		}
		return MakeFactUnionsSess(s, lvars, rhsFact.LastWrittenFID)
	case TermFunction:
		// FactUnion.cpp:99–109 — get_return_fact_for_invocation(…, eUnionWrite).
		// Soft invent looked up ambient unionFacts by RV subject (missed registry).
		// missing Invoke/User/RV or related union fact during generation —
		// non-sticky hole (soft re-pick; no invent fid 0)
		if rhs.Invoke == nil || rhs.Invoke.User == nil || rhs.Invoke.User.RV == nil {
			return IncompleteUnionFactSlice()
		}
		// only user func calls have return-fact registry (eFuncCall)
		if rhs.Invoke.IsStd {
			return IncompleteUnionFactSlice()
		}
		rv := rhs.Invoke.User.RV
		uf := GetReturnUnionFactForInvocationSess(s, rhs.Invoke, rv)
		// residual ERROR sticky — no invent soft-union transfer past registry residual
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		if uf != nil {
			return MakeFactUnionsSess(s, lvars, uf.LastWrittenFID)
		}
		// FactUnion.cpp:107 assert(rv_fact) path — non-sticky generation hole
		return IncompleteUnionFactSlice()
	case TermAssignment:
		// FactUnion.cpp:110–112 — peel embedded assign RHS
		// incomplete Assign/Expr sticky — no invent empty/non-sticky via nil-rhs peel
		// (generation AddParamFacts missing-arg path is bare rhs=nil, not compound shell)
		if rhs.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if rhs.Assign.Expr == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		return RhsToLhsTransferUnionSess(s, unionFacts, ptFacts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		// FactUnion.cpp:113–115 — peel comma RHS
		// incomplete CommaRHS sticky — no invent empty/non-sticky via nil-rhs peel
		if rhs.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		return RhsToLhsTransferUnionSess(s, unionFacts, ptFacts, lvars, rhs.CommaRHS)
	default:
		// unknown term — non-sticky hole
		return IncompleteUnionFactSlice()
	}
}

// AbstractFactUnionForAssign mirrors FactUnion::abstract_fact_for_assign.
// FactUnion.cpp:121–154 — union-typed LHS transfers fid; union-field write
// records parent fid; padding/packed-after-bitfield → BOTTOM on container.
//
// lhsWant is Lhs::get_type() (FactUnion.cpp:133) — the *desired* type after
// dereference, not Variable::type. Soft invent used lhs.Type (the pointer
// shell for `(*p)=…` where p is union*) so whole-union assigns never took
// rhs_to_lhs_transfer; eUnionWrite stayed BOTTOM while UP renewed from (*p)=.
// nil lhsWant falls back to lhs.Type (bare Variable paths / tests).
//
// Returns (factsOut, lvarCount). Hard IR (nil lhs, nil pointee, MakeFact fail)
// sticky; incomplete maps/MergePointees stay non-sticky hole markers.}

func AbstractFactUnionForAssignSess(s *Session,
	unionFacts []*FactUnion,
	ptFacts []*FactPointTo,
	lhs *Variable,
	lhsIndir int,
	lhsWant *Type,
	rhs *Expression,
) (out []*FactUnion, lvarCnt int) {
	if lhs == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice(), 0
	}
	// incomplete maps — non-sticky abstract hole
	if !UnionFactsComplete(unionFacts) || !FactsComplete(ptFacts) {
		return IncompleteUnionFactSlice(), 0
	}
	coll := lhs.GetCollectiveSess(s)
	// residual ERROR sticky — no invent soft-abstract union past GetCollective residual
	if sessHasError(s) {
		return IncompleteUnionFactSlice(), 0
	}
	lvars := MergePointeesOfPointerSess(s, coll, lhsIndir, ptFacts)
	// residual ERROR sticky — no invent soft-abstract union past MergePointees residual
	if sessHasError(s) {
		return IncompleteUnionFactSlice(), 0
	}
	// incomplete merge at indir>0 — non-sticky; indir 0 yields [lhs]
	if !VariablesComplete(lvars) {
		return IncompleteUnionFactSlice(), 0
	}
	lvarCnt = len(lvars)
	// FactUnion.cpp:133 — lhs->get_type() (Lhs desired type), not var->type.
	// Soft invent used Variable.Type: (*union*) never matched eUnion.
	want := lhsWant
	if want == nil {
		want = lhs.Type
	}
	// FactUnion.cpp:129 — lhs->get_type() always live Type&; Type-nil shell sticky
	// (no invent non-union complete transfer soft-skip past incomplete LHS type)
	// Special null/garbage/tbd have Type nil by design — complete non-union path.
	if want == nil {
		if !IsSpecialPtr(lhs) {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice(), lvarCnt
		}
	} else if want.IsUnionSess(s) {
		// residual ERROR sticky — no invent union transfer past IsUnion residual hole
		if sessHasError(s) {
			return IncompleteUnionFactSlice(), lvarCnt
		}
		out := RhsToLhsTransferUnionSess(s, unionFacts, ptFacts, lvars, rhs)
		// residual ERROR sticky — no invent soft-continue transfer past union transfer residual
		if sessHasError(s) {
			return IncompleteUnionFactSlice(), lvarCnt
		}
		return out, lvarCnt
	} else if sessHasError(s) {
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
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice(), lvarCnt
		}
		var fu *FactUnion
		if v.IsUnionFieldSess(s) {
			// residual ERROR sticky — no invent soft-continue transfer past IsUnionField hole
			if sessHasError(s) {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FactUnion.cpp:141–143
			fid := v.GetFieldIDSess(s)
			// residual ERROR sticky — no invent soft-union fact past GetFieldID residual
			if sessHasError(s) {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			fu = MakeFactUnionSess(s, v.FieldVarOf, fid)
			// residual ERROR sticky — no invent soft-continue past MakeFactUnion residual
			if sessHasError(s) {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FieldVarOf non-union → MakeFactUnion nil is broken IR sticky
			if fu == nil && v.FieldVarOf != nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteUnionFactSlice(), lvarCnt
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue IsInside path past IsUnionField residual false
			return IncompleteUnionFactSlice(), lvarCnt
		} else if v.IsInsideUnionFieldSess(s) {
			// residual ERROR sticky — no invent soft-continue transfer past IsInsideUnionField hole
			if sessHasError(s) {
				return IncompleteUnionFactSlice(), lvarCnt
			}
			// FactUnion.cpp:144–146 — v->type->has_padding() always live Type*
			// Type-nil non-special sticky (no invent skip BOTTOM / soft re-pick)
			typ := v.Type
			if typ == nil {
				if !IsSpecialPtr(v) {
					sessNoteError(s, ErrGeneric)
					return IncompleteUnionFactSlice(), lvarCnt
				}
			} else if typ.HasPaddingSess(s) || v.IsPackedAfterBitfieldSess(s) {
				// residual ERROR sticky — no invent soft-continue padding path past IsPacked residual
				if sessHasError(s) {
					return IncompleteUnionFactSlice(), lvarCnt
				}
				cu := v.GetContainerUnionSess(s)
				// residual ERROR sticky — no invent soft-skip container past GetContainerUnion residual
				if sessHasError(s) {
					return IncompleteUnionFactSlice(), lvarCnt
				}
				if cu != nil {
					fu = MakeFactUnionSess(s, cu, FactUnionBottom)
					// residual ERROR sticky — no invent soft-continue past MakeFactUnion residual
					if sessHasError(s) {
						return IncompleteUnionFactSlice(), lvarCnt
					}
					if fu == nil {
						sessNoteError(s, ErrGeneric)
						return IncompleteUnionFactSlice(), lvarCnt
					}
				}
			} else if sessHasError(s) {
				// residual ERROR sticky — no invent soft-skip no-padding past IsPacked residual false
				return IncompleteUnionFactSlice(), lvarCnt
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-skip not-inside past IsInside residual false
			return IncompleteUnionFactSlice(), lvarCnt
		}
		if fu != nil {
			out = append(out, fu)
		}
	}
	return out, lvarCnt
}
