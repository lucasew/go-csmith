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

// OpportunisticValidate mirrors FactPointTo::opportunistic_validate.
// FactPointTo.cpp:442–472 — 0 reject, 1 ok, 2 allowed unsafe deref via flipcoin.
// r may be nil when both probs are 0 (deterministic reject on null/dead).
func OpportunisticValidate(r *Rng, v *Variable, typ *Type, facts []*FactPointTo, nullProb, deadProb int) int {
	if v == nil || v.Type == nil || typ == nil {
		return 0
	}
	// no extra indirection needed
	if v.Type.IndirectLevel() <= typ.IndirectLevel() {
		return 1
	}
	fp := FindRelatedPointTo(facts, v)
	if fp == nil {
		return 0
	}
	ret := 0
	if fp.IsNull() {
		if nullProb > 0 && r != nil && r.RndFlipcoin(uint32(nullProb)) {
			ret = 2
		} else {
			return 0
		}
	} else {
		ret = 1
	}
	if fp.IsDead() {
		if deadProb > 0 && r != nil && r.RndFlipcoin(uint32(deadProb)) {
			ret = 2
		} else {
			return 0
		}
	}
	return ret
}

// MakeFactsPointTo mirrors FactPointTo::make_facts(vars, point_to).
// FactPointTo.cpp:340–348.
func MakeFactsPointTo(lvars []*Variable, pointTo *Variable) []*FactPointTo {
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil || v.Type == nil {
			continue
		}
		out = append(out, MakeFactPointTo(v, pointTo))
	}
	return out
}

// MakeFactsPointToSet mirrors FactPointTo::make_facts(vars, set).
func MakeFactsPointToSet(lvars []*Variable, set []*Variable) []*FactPointTo {
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil || v.Type == nil {
			continue
		}
		out = append(out, MakeFactPointToSet(v, set))
	}
	return out
}

// RhsToLhsTransfer mirrors FactPointTo::rhs_to_lhs_transfer for common cases.
// FactPointTo.cpp:158–227 subset — const/null/garbage, &var, pointer copy.
func RhsToLhsTransfer(facts []*FactPointTo, lvars []*Variable, rhs *Expression) []*FactPointTo {
	if len(lvars) == 0 {
		return nil
	}
	if rhs == nil {
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
	rt := rhs.GetType()
	// non-pointer, non-union RHS
	if rt == nil || (!rt.IsPointerLike() && !rt.IsUnion()) {
		// equals(0) and size >= 8 → null else garbage
		if rhs.EqualsInt(0) && rt != nil && rt.SizeInBytes() >= 8 {
			return MakeFactsPointTo(lvars, NullPtr)
		}
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
	switch rhs.Term {
	case TermConstant:
		if rt.ptrTo != nil {
			if rhs.EqualsInt(0) {
				return MakeFactsPointTo(lvars, NullPtr)
			}
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		return MakeFactsPointTo(lvars, GarbagePtr)
	case TermVariable:
		if rhs.Var == nil {
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		indirect := rhs.IndirectLevel()
		if indirect < 0 {
			// taking address: point to the var itself (collective)
			return MakeFactsPointTo(lvars, rhs.Var.GetCollective())
		}
		// copy pointees of RHS pointer (indirect+1 merge simplified to fact set)
		src := FindRelatedPointTo(facts, rhs.Var.GetCollective())
		if src == nil {
			// unknown → garbage
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		return MakeFactsPointToSet(lvars, src.PointTo)
	default:
		// function/assign/comma — conservative garbage
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
}

// AbstractFactForAssign mirrors FactPointTo::abstract_fact_for_assign (pointer LHS).
// FactPointTo.cpp:266–277 — direct pointer assign only (union fields deferred).
func AbstractFactForAssign(factsIn []*FactPointTo, lhs *Variable, lhsIndir int, rhs *Expression) []*FactPointTo {
	if lhs == nil || lhs.Type == nil {
		return nil
	}
	// only when LHS expression type is pointer (indir 0 on a pointer var)
	// Lhs with *p (indir>0) updates pointees — deferred; handle indir==0 pointer store
	if lhsIndir != 0 {
		return nil
	}
	if lhs.Type.ptrTo == nil {
		return nil
	}
	lvars := []*Variable{lhs.GetCollective()}
	return RhsToLhsTransfer(factsIn, lvars, rhs)
}

// MergeFactInto replaces or appends a related FactPointTo in facts (merge_fact subset).
func MergeFactInto(facts []*FactPointTo, f *FactPointTo) []*FactPointTo {
	if f == nil {
		return facts
	}
	for i, old := range facts {
		if old != nil && old.Var == f.Var {
			// replace (strong update)
			facts[i] = f
			return facts
		}
	}
	return append(facts, f)
}

// MarkDeadVar mirrors FactPointTo::mark_dead_var.
// FactPointTo.cpp:106–123 — replace/remove pointee v with garbage_ptr.
func (f *FactPointTo) MarkDeadVar(v *Variable) *FactPointTo {
	if f == nil || v == nil {
		return nil
	}
	set := append([]*Variable(nil), f.PointTo...)
	pos := -1
	for i, p := range set {
		if p == v || (p != nil && v.HasFieldVar(p)) || (p != nil && p.FieldVarOf != nil && isAncestorField(p, v)) {
			pos = i
			break
		}
	}
	if pos < 0 {
		return nil
	}
	hasGarbage := false
	for _, p := range set {
		if p == GarbagePtr {
			hasGarbage = true
			break
		}
	}
	if hasGarbage {
		set = append(set[:pos], set[pos+1:]...)
	} else {
		set[pos] = GarbagePtr
	}
	return MakeFactPointToSet(f.Var, set)
}

// isAncestorField reports whether field is under root via FieldVarOf chain.
func isAncestorField(field, root *Variable) bool {
	for field != nil {
		if field == root {
			return true
		}
		field = field.FieldVarOf
	}
	return false
}

// MarkFuncEndLocals marks any pointee in locals as dead (mark_func_end subset).
// FactPointTo.cpp:130–152 without Statement — locals list is the out-of-scope set.
func (f *FactPointTo) MarkFuncEndLocals(locals []*Variable) *FactPointTo {
	if f == nil || len(locals) == 0 {
		return nil
	}
	localSet := make(map[*Variable]bool, len(locals))
	for _, l := range locals {
		if l != nil {
			localSet[l] = true
			for _, fv := range l.CollectExpandable() {
				localSet[fv] = true
			}
		}
	}
	set := append([]*Variable(nil), f.PointTo...)
	hasGarbage := false
	for _, p := range set {
		if p == GarbagePtr {
			hasGarbage = true
			break
		}
	}
	changed := false
	for i := 0; i < len(set); i++ {
		p := set[i]
		if p == nil || IsSpecialPtr(p) || !localSet[p] {
			continue
		}
		if hasGarbage {
			set = append(set[:i], set[i+1:]...)
			i--
		} else {
			set[i] = GarbagePtr
			hasGarbage = true
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return MakeFactPointToSet(f.Var, set)
}

// IsPointingToLocals mirrors FactPointTo::is_pointing_to_locals (indirection≥0 subset).
// FactPointTo.cpp:487–526 — indirection -1 → IsVisibleLocal; 0 → fact pointees local.
func IsPointingToLocals(v *Variable, b *Block, indirection int, facts []*FactPointTo) bool {
	if v == nil {
		return false
	}
	if indirection == -1 {
		return v.IsVisibleLocal(b)
	}
	if !v.IsPointer() {
		return false
	}
	// indirection==0: look at points-to set; higher levels deferred (merge_pointees)
	if indirection == 0 {
		ft := FindRelatedPointTo(facts, v)
		if ft == nil {
			return false
		}
		for _, p := range ft.PointTo {
			if p == nil || IsSpecialPtr(p) {
				continue
			}
			if p.IsVisibleLocal(b) {
				return true
			}
		}
		return false
	}
	// multi-level: treat like indirection 0 for now (no merge_pointees yet)
	ft := FindRelatedPointTo(facts, v)
	if ft == nil {
		return false
	}
	for _, p := range ft.PointTo {
		if p != nil && !IsSpecialPtr(p) && p.IsVisibleLocal(b) {
			return true
		}
	}
	return false
}
