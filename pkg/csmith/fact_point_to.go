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
		// FactPointTo.cpp:223–226 — merge_pointees(collective, indirect+1)
		set := MergePointeesOfPointer(rhs.Var.GetCollective(), indirect+1, facts)
		if len(set) == 0 {
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		return MakeFactsPointToSet(lvars, set)
	default:
		// function/assign/comma — conservative garbage
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
}

// AbstractFactForAssign mirrors FactPointTo::abstract_fact_for_assign.
// FactPointTo.cpp:266–295 — merge_pointees of LHS; pointer assign or pointer fields.
func AbstractFactForAssign(factsIn []*FactPointTo, lhs *Variable, lhsIndir int, rhs *Expression) []*FactPointTo {
	if lhs == nil || lhs.Type == nil {
		return nil
	}
	// find all pointed variables on LHS (merge_pointees of collective)
	lvars := MergePointeesOfPointer(lhs.GetCollective(), lhsIndir, factsIn)
	if lhsIndir == 0 && lhs.Type.ptrTo != nil {
		// direct pointer store: LHS is the pointer var itself
		if len(lvars) == 0 {
			lvars = []*Variable{lhs.GetCollective()}
		}
		return RhsToLhsTransfer(factsIn, lvars, rhs)
	}
	// when assigning through *p (indir>0) or to aggregate, transfer to pointer fields
	if len(lvars) == 0 && lhsIndir == 0 {
		// non-pointer scalar LHS
		if !lhs.IsAggregate() {
			return nil
		}
		lvars = []*Variable{lhs}
	}
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			continue
		}
		// walk to union container for union field path (simplified: pointer fields only)
		ptrs := v.FindPointerFields()
		if v.IsPointer() && lhsIndir > 0 {
			// assigning *p = rhs updates pointees as if they were written — garbage if unknown
			// for pointer-typed *p result, use transfer onto pointees that are pointers
			for _, p := range MergePointeesOfPointer(v, 1, factsIn) {
				if p != nil && p.IsPointer() {
					ptrs = append(ptrs, p)
				}
			}
		}
		if len(ptrs) == 0 {
			continue
		}
		out = append(out, RhsToLhsTransfer(factsIn, ptrs, rhs)...)
	}
	return out
}

// Equal reports same var and same points-to set.
func (f *FactPointTo) Equal(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	if len(f.PointTo) != len(other.PointTo) {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		set[p] = true
	}
	for _, p := range other.PointTo {
		if !set[p] {
			return false
		}
	}
	return true
}

// Imply mirrors FactPointTo::imply — other.point_to ⊆ this.point_to.
// FactPointTo.cpp:602–609.
func (f *FactPointTo) Imply(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		set[p] = true
	}
	for _, p := range other.PointTo {
		if !set[p] {
			return false
		}
	}
	return true
}

// Join mirrors FactPointTo::join — union of points-to sets; returns true if changed.
// FactPointTo.cpp:563–578.
func (f *FactPointTo) Join(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		set[p] = true
	}
	changed := false
	for _, p := range other.PointTo {
		if p == nil || set[p] {
			continue
		}
		set[p] = true
		f.PointTo = append(f.PointTo, p)
		changed = true
	}
	return changed
}

// JoinVisits mirrors FactPointTo::join_visits.
// FactPointTo.cpp:584–605 — merge across revisits; ignore TBD-only other;
// clear TBD-only self before absorbing concrete pointees.
func (f *FactPointTo) JoinVisits(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	if other.IsTBDOnly() {
		return false
	}
	if f.IsTBDOnly() {
		f.PointTo = nil
	}
	return f.Join(other)
}

// JoinVisitsInto merges newFacts into facts with join_visits semantics.
// Used when combining results of multiple visits to the same function.
func JoinVisitsInto(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if nf == nil || nf.Var == nil {
			continue
		}
		cur := FindRelatedPointTo(*facts, nf.Var)
		if cur == nil {
			*facts = append(*facts, nf.Clone())
			changed = true
			continue
		}
		// join into clone then replace
		cp := cur.Clone()
		if cp.JoinVisits(nf) {
			// replace in slice
			for i, f := range *facts {
				if f != nil && f.Var == nf.Var {
					(*facts)[i] = cp
					break
				}
			}
			changed = true
		}
	}
	return changed
}

// Clone shallow-copies the fact (new PointTo slice).
func (f *FactPointTo) Clone() *FactPointTo {
	if f == nil {
		return nil
	}
	return MakeFactPointToSet(f.Var, f.PointTo)
}

// MergeFactInto merges new fact with lattice join (Fact::merge_fact).
// Fact.cpp:149–171 — strong replace only when not related; else join.
func MergeFactInto(facts []*FactPointTo, f *FactPointTo) []*FactPointTo {
	if f == nil {
		return facts
	}
	for i, old := range facts {
		if old != nil && old.Var == f.Var {
			if old.Imply(f) {
				// old already covers f
				return facts
			}
			// join: copy f, join old into it
			cp := f.Clone()
			_ = cp.Join(old)
			facts[i] = cp
			return facts
		}
	}
	return append(facts, f.Clone())
}

// MergeFacts mirrors merge_facts — merge each of new into facts.
// Fact.cpp:192–200.
func MergeFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	changed := false
	for _, f := range newFacts {
		before := len(*facts)
		*facts = MergeFactInto(*facts, f)
		// detect change roughly: length or content
		if len(*facts) != before {
			changed = true
			continue
		}
		// check related fact expanded
		if f != nil {
			cur := FindRelatedPointTo(*facts, f.Var)
			if cur != nil && !cur.Equal(f) && !f.Imply(cur) {
				changed = true
			}
		}
	}
	return changed
}

// CloneFactSlice deep-clones a FactPointTo slice.
func CloneFactSlice(facts []*FactPointTo) []*FactPointTo {
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
		if f != nil {
			out = append(out, f.Clone())
		}
	}
	return out
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

// MarkFuncEnd mirrors FactPointTo::mark_func_end.
// FactPointTo.cpp:129–154 — pointees on stack at stm become garbage.
// stParent is the statement's parent block (for is_var_on_stack).
func (f *FactPointTo) MarkFuncEnd(fn *Function, stParent *Block) *FactPointTo {
	if f == nil || fn == nil {
		return nil
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
		v := set[i]
		if v == nil || IsSpecialPtr(v) {
			continue
		}
		if !fn.IsVarOnStack(v, stParent) {
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

// MarkFuncEndOnFacts applies mark_func_end to each point-to fact in-place.
// FactMgr.cpp:196–204.
func MarkFuncEndOnFacts(facts *[]*FactPointTo, fn *Function, stParent *Block) {
	if facts == nil {
		return
	}
	for i, f := range *facts {
		if f == nil {
			continue
		}
		if nf := f.MarkFuncEnd(fn, stParent); nf != nil {
			(*facts)[i] = nf
		}
	}
}

// indexExprUsesVar reports whether a string index expression refers to indexVar.
// Indices are stored as strings (e.g. "i", "(i + 2)"); approximate Expression::use_var.
func indexExprUsesVar(idx string, indexVar *Variable) bool {
	if indexVar == nil || idx == "" {
		return false
	}
	name := indexVar.Name
	if name == "" {
		return false
	}
	if idx == name {
		return true
	}
	// token-ish: whole name appears bounded by non-ident chars
	for i := 0; i+len(name) <= len(idx); i++ {
		if idx[i:i+len(name)] != name {
			continue
		}
		leftOK := i == 0 || !isIdentChar(idx[i-1])
		rightOK := i+len(name) == len(idx) || !isIdentChar(idx[i+len(name)])
		if leftOK && rightOK {
			return true
		}
	}
	return false
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// UpdateWithModifiedIndex mirrors FactPointTo::update_with_modified_index.
// FactPointTo.cpp:712–748 — if pointee is itemized array whose index uses
// indexVar, replace that index with Constant("-1") (any-member).
// Returns this fact if unchanged, or a new fact with rewritten pointees.
func (f *FactPointTo) UpdateWithModifiedIndex(indexVar *Variable) *FactPointTo {
	if f == nil || indexVar == nil {
		return f
	}
	pointees := append([]*Variable(nil), f.PointTo...)
	changed := false
	for j, v := range f.PointTo {
		if v == nil {
			continue
		}
		// walk to root field_var_of (FactPointTo.cpp:718–720)
		root := v
		for root.FieldVarOf != nil {
			root = root.FieldVarOf
		}
		// itemized array: isArray && get_collective() != v (FactPointTo.cpp:722)
		av := root.AsArray
		if av == nil || !root.IsArray || av.Collective == nil {
			continue
		}
		var modified []int
		// prefer IndexExprs UseVar (FactPointTo.cpp:726–730); fall back to string
		if len(av.IndexExprs) > 0 {
			for k, exp := range av.IndexExprs {
				if exp != nil && exp.UseVar(indexVar) {
					modified = append(modified, k)
				}
			}
		} else {
			for k, exp := range av.Indices {
				if indexExprUsesVar(exp, indexVar) {
					modified = append(modified, k)
				}
			}
		}
		if len(modified) == 0 {
			continue
		}
		// ArrayVariable.cpp set_index path: clone itemized member, set -1 on hit dims
		newAV := &ArrayVariable{
			Variable:   av.Variable,
			Sizes:      append([]int(nil), av.Sizes...),
			InitValues: av.InitValues,
			Block:      av.Block,
			Collective: av.Collective,
			Indices:    append([]string(nil), av.Indices...),
			IndexExprs: append([]*Expression(nil), av.IndexExprs...),
		}
		newAV.IsArray = true
		newAV.AsArray = newAV
		for _, k := range modified {
			newAV.SetIndex(k, "-1")
		}
		// FactPointTo.cpp:740 — pointees[j] = new_av (array root, even if field walked)
		pointees[j] = &newAV.Variable
		changed = true
	}
	if !changed {
		return f
	}
	return MakeFactPointToSet(f.Var, pointees)
}

// UpdateFactsWithModifiedIndex mirrors FactPointTo::update_facts_with_modified_index.
// FactPointTo.cpp:751–761 — rewrite each point-to fact when indexVar is modified.
func UpdateFactsWithModifiedIndex(facts *[]*FactPointTo, indexVar *Variable) {
	if facts == nil || indexVar == nil {
		return
	}
	for i, fp := range *facts {
		if fp == nil {
			continue
		}
		newFP := fp.UpdateWithModifiedIndex(indexVar)
		if newFP != fp {
			(*facts)[i] = newFP
		}
	}
}

// MergePointeesOfPointers mirrors FactPointTo::merge_pointees_of_pointers.
// FactPointTo.cpp:680–704 — union of points-to sets for each pointer.
func MergePointeesOfPointers(ptrs []*Variable, facts []*FactPointTo) []*Variable {
	var out []*Variable
	seen := make(map[*Variable]bool)
	for _, p := range ptrs {
		if p == nil || IsSpecialPtr(p) {
			continue
		}
		ft := FindRelatedPointTo(facts, p)
		if ft == nil {
			continue
		}
		for _, pointee := range ft.PointTo {
			if pointee == nil || seen[pointee] {
				continue
			}
			seen[pointee] = true
			out = append(out, pointee)
		}
	}
	return out
}

// MergePointeesOfPointer mirrors FactPointTo::merge_pointees_of_pointer.
// FactPointTo.cpp:669–676 — start from ptr, indirect steps of merge_pointees.
func MergePointeesOfPointer(ptr *Variable, indirect int, facts []*FactPointTo) []*Variable {
	if ptr == nil {
		return nil
	}
	tmp := []*Variable{ptr}
	for indirect > 0 {
		tmp = MergePointeesOfPointers(tmp, facts)
		indirect--
	}
	return tmp
}

// IsPointingToLocals mirrors FactPointTo::is_pointing_to_locals.
// FactPointTo.cpp:487–526.
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
	var pointees []*Variable
	if indirection == 0 {
		ft := FindRelatedPointTo(facts, v)
		if ft == nil {
			return false
		}
		pointees = ft.PointTo
	} else {
		pointees = MergePointeesOfPointer(v, indirection, facts)
	}
	for _, p := range pointees {
		if p == nil || IsSpecialPtr(p) {
			continue
		}
		if p.IsVisibleLocal(b) {
			return true
		}
		// recurse one level of pointees that are pointers
		if p.IsPointer() {
			for j := 0; j < p.Type.IndirectLevel(); j++ {
				nested := MergePointeesOfPointer(p, j+1, facts)
				for _, n := range nested {
					if n != nil && !IsSpecialPtr(n) && n.IsVisibleLocal(b) {
						return true
					}
				}
			}
		}
	}
	return false
}

// Package-level alias aggregates (FactPointTo.cpp:67–68).
var (
	// AllPtrs mirrors FactPointTo::all_ptrs.
	AllPtrs []*Variable
	// AllAliases mirrors FactPointTo::all_aliases (parallel to AllPtrs).
	AllAliases [][]*Variable
)

// ClearPointToAggregates resets all_ptrs / all_aliases (generation start).
func ClearPointToAggregates() {
	AllPtrs = nil
	AllAliases = nil
}

// UpdatePtrAliases mirrors FactPointTo::update_ptr_aliases.
// FactPointTo.cpp:764–790 — merge point-to sets into parallel ptr/alias vectors.
func UpdatePtrAliases(facts []*FactPointTo, ptrs *[]*Variable, aliases *[][]*Variable) {
	if ptrs == nil || aliases == nil {
		return
	}
	for _, f := range facts {
		if f == nil || f.Var == nil || f.Var.Type == nil {
			// skip rv-like without type (upstream: type != 0)
			continue
		}
		pos := -1
		for i, p := range *ptrs {
			if p == f.Var {
				pos = i
				break
			}
		}
		if pos < 0 {
			*ptrs = append(*ptrs, f.Var)
			set := append([]*Variable(nil), f.PointTo...)
			*aliases = append(*aliases, set)
			continue
		}
		// merge alias set
		for _, v := range f.PointTo {
			if !IsVariableInSet((*aliases)[pos], v) {
				(*aliases)[pos] = append((*aliases)[pos], v)
			}
		}
	}
}

// AggregateAllPointToSets mirrors FactPointTo::aggregate_all_pointto_sets.
// FactPointTo.cpp:792–804 — scan each non-builtin func FactMgr map_facts_out.
func AggregateAllPointToSets(funcs []*Function, fms *FactMgrMap) {
	ClearPointToAggregates()
	for _, f := range funcs {
		if f == nil || f.IsBuiltin {
			continue
		}
		var fm *FactMgr
		if fms != nil {
			fm = fms.ForFunc(f)
		}
		if fm == nil {
			continue
		}
		// prefer map_facts_out values; also include GlobalFacts
		seen := map[int]bool{}
		for id, facts := range fm.MapFactsOut {
			_ = id
			UpdatePtrAliases(facts, &AllPtrs, &AllAliases)
			seen[id] = true
		}
		UpdatePtrAliases(fm.GlobalFacts, &AllPtrs, &AllAliases)
	}
}
