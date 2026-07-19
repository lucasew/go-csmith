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
// FactPointTo.cpp:354–359 — Variable* always live at construction sites.
// no invent FactPointTo{nil, garbage} shell for nil subject
func NewFactPointTo(v *Variable) *FactPointTo {
	if v == nil {
		return nil
	}
	return &FactPointTo{Var: v, PointTo: []*Variable{GarbagePtr}}
}

// MakeFactPointTo mirrors FactPointTo::make_fact(v, point_to).
// no invent fact without live subject Variable* / live pointee (use NullPtr etc.)
func MakeFactPointTo(v *Variable, pointTo *Variable) *FactPointTo {
	if v == nil || pointTo == nil {
		return nil
	}
	return &FactPointTo{Var: v, PointTo: []*Variable{pointTo}}
}

// MakeFactPointToSet mirrors FactPointTo::make_fact(v, set).
// no invent fact without live subject; nil set or nil pointee hole fails closed
// (nil set is incomplete merge_pointees — no invent empty IsTop PointTo from nil).
// Valid empty sets use non-nil empty slice []*Variable{}.
func MakeFactPointToSet(v *Variable, set []*Variable) *FactPointTo {
	if v == nil || set == nil {
		return nil
	}
	for _, p := range set {
		if p == nil {
			return nil
		}
	}
	cp := append([]*Variable(nil), set...)
	return &FactPointTo{Var: v, PointTo: cp}
}

// IsNull mirrors FactPointTo::is_null — any null_ptr in the set.
// Incomplete PointTo (nil hole) fails closed true — no invent not-null past holes.
func (f *FactPointTo) IsNull() bool {
	if f == nil {
		return false
	}
	for _, p := range f.PointTo {
		if p == nil {
			return true
		}
		if p == NullPtr {
			return true
		}
	}
	return false
}

// IsDead mirrors FactPointTo::is_dead — garbage_ptr in the set.
// Incomplete PointTo (nil hole) fails closed true — no invent not-dead past holes.
func (f *FactPointTo) IsDead() bool {
	if f == nil {
		return false
	}
	for _, p := range f.PointTo {
		if p == nil {
			return true
		}
		if p == GarbagePtr {
			return true
		}
	}
	return false
}

// IsTBDOnly mirrors FactPointTo::is_tbd_only.
// Incomplete PointTo (nil hole) fails closed false — not a pure TBD-only set.
func (f *FactPointTo) IsTBDOnly() bool {
	if f == nil || len(f.PointTo) != 1 {
		return false
	}
	if f.PointTo[0] == nil {
		return false
	}
	return f.PointTo[0] == TBDPtr
}

// IsSpecialPtr mirrors FactPointTo::is_special_ptr.
func IsSpecialPtr(p *Variable) bool {
	return p == NullPtr || p == GarbagePtr || p == TBDPtr
}

// FindRelatedPointTo mirrors find_related_fact for ePointTo (var identity).
// Fact* always live; nil hole fails closed (nil — no invent skip to later match).
func FindRelatedPointTo(facts []*FactPointTo, p *Variable) *FactPointTo {
	if p == nil {
		return nil
	}
	for _, f := range facts {
		if f == nil {
			return nil
		}
		if f.Var == p {
			return f
		}
	}
	return nil
}

// IsValidPtr mirrors FactPointTo::is_valid_ptr(Variable*, facts).
// FactPointTo.cpp:411–419 — needs related fact; null/dead forbidden when probs are 0.
// opts.NullPointerDerefProb / DeadPointerDerefProb default 0 (upstream CGOptions).
// Incomplete fact maps fail closed as invalid (no invent valid via skip holes).
func IsValidPtr(p *Variable, facts []*FactPointTo, nullProb, deadProb int) bool {
	if !FactsComplete(facts) {
		return false
	}
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
// Incomplete fact maps fail closed as dangling (true — no invent not-dangling
// when FindRelatedPointTo returns nil past a hole before the related fact).
func IsDanglingPtr(p *Variable, facts []*FactPointTo, deadProb int) bool {
	if !FactsComplete(facts) {
		return true
	}
	fact := FindRelatedPointTo(facts, p)
	if fact == nil {
		return false
	}
	return fact.IsDead() && deadProb == 0
}

// OpportunisticValidate mirrors FactPointTo::opportunistic_validate.
// FactPointTo.cpp:442–472 — 0 reject, 1 ok, 2 allowed unsafe deref via flipcoin.
// r may be nil when both probs are 0 (deterministic reject on null/dead).
// Incomplete fact maps fail closed as reject 0 (no invent ok via hole skip).
func OpportunisticValidate(r *Rng, v *Variable, typ *Type, facts []*FactPointTo, nullProb, deadProb int) int {
	if v == nil || v.Type == nil || typ == nil {
		return 0
	}
	if !FactsComplete(facts) {
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
// FactPointTo.cpp:340–348 — vars[i] always live; skip only type==null specials.
// no invent skip of nil Variable* holes as partial success; non-special Type-nil
// is incomplete IR (fail closed whole batch — IncompleteFactSlice, not bare nil;
// FactsComplete(nil)==true invents empty-complete make_facts success).
func MakeFactsPointTo(lvars []*Variable, pointTo *Variable) []*FactPointTo {
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			return IncompleteFactSlice()
		}
		// type null: specials (null/garbage/tbd) skipped; other broken IR fails closed
		if v.Type == nil {
			if IsSpecialPtr(v) {
				continue
			}
			return IncompleteFactSlice()
		}
		f := MakeFactPointTo(v, pointTo)
		if f == nil {
			return IncompleteFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// MakeFactsPointToSet mirrors FactPointTo::make_facts(vars, set).
// same live-vars rules as MakeFactsPointTo; nil set fails closed IncompleteFactSlice
// (no invent empty complete — FactsComplete(nil)==true).
func MakeFactsPointToSet(lvars []*Variable, set []*Variable) []*FactPointTo {
	if set == nil {
		return IncompleteFactSlice()
	}
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			return IncompleteFactSlice()
		}
		if v.Type == nil {
			if IsSpecialPtr(v) {
				continue
			}
			return IncompleteFactSlice()
		}
		f := MakeFactPointToSet(v, set)
		if f == nil {
			return IncompleteFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// RhsToLhsTransfer mirrors FactPointTo::rhs_to_lhs_transfer.
// FactPointTo.cpp:158–263 — const/null/garbage, &var, pointer copy,
// aggregate field transfer, function return facts, assign/comma peel.
func RhsToLhsTransfer(facts []*FactPointTo, lvars []*Variable, rhs *Expression) []*FactPointTo {
	if len(lvars) == 0 {
		// complete empty transfer targets (not incomplete)
		return nil
	}
	// FactPointTo.cpp:164–167 — assert all possible LHS are pointers
	for _, v := range lvars {
		if v == nil || !v.IsPointer() {
			// fail closed IncompleteFactSlice — no invent empty transfer success
			return IncompleteFactSlice()
		}
	}
	// FactPointTo.cpp:168–169 — rhs==nullptr → garbage (param missing / abstract nullptr)
	// fair C++ path for AddParamFacts missing args — not invent; mirrors nullptr value
	if rhs == nil {
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
	rt := rhs.GetType()
	// non-pointer, non-union RHS (FactPointTo.cpp:172–178)
	if rt == nil || (!rt.IsPointerLike() && !rt.IsUnion()) {
		// equals(0) and size >= 8 → null else garbage
		if rhs.EqualsInt(0) && rt != nil && rt.SizeInBytes() >= 8 {
			return MakeFactsPointTo(lvars, NullPtr)
		}
		return MakeFactsPointTo(lvars, GarbagePtr)
	}
	switch rhs.Term {
	case TermConstant:
		if rt.IsPointerLike() {
			if rhs.EqualsInt(0) {
				return MakeFactsPointTo(lvars, NullPtr)
			}
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		// FactPointTo.cpp:186–193 — union constant field0 "0" → null on field0 pointers
		if rt.IsUnion() {
			lv0 := lvars[0]
			if lv0 != nil && lv0.FieldVarOf != nil && lv0.FieldVarOf.Type != nil &&
				lv0.FieldVarOf.Type.IsUnion() && lv0.GetFieldID() == 0 {
				// Constant::get_field(0) == "0"
				if rhs.Con != nil && rhs.Con.GetField(0) == "0" {
					return MakeFactsPointTo(lvars, NullPtr)
				}
				if rhs.EqualsInt(0) {
					return MakeFactsPointTo(lvars, NullPtr)
				}
			}
			return MakeFactsPointTo(lvars, GarbagePtr)
		}
		// FactPointTo.cpp:195–196 — assert(0); no soft invent garbage for other constants
		return IncompleteFactSlice()
	case TermVariable:
		// C++ always has ExpressionVariable; nil var is broken IR
		if rhs.Var == nil {
			return IncompleteFactSlice()
		}
		// incomplete type IR must not invent level-0 transfer / false address-of
		indirect, iok := rhs.IndirectLevelComplete()
		if !iok {
			return IncompleteFactSlice()
		}
		if indirect < 0 {
			// FactPointTo.cpp:202–207 — taking address; multi-level & not allowed
			// assert(indirect == -1); no soft invent for indirect < -1
			if indirect != -1 {
				return IncompleteFactSlice()
			}
			// GetCollective always live for address-of; nil is broken IR
			coll := rhs.Var.GetCollective()
			if coll == nil {
				return IncompleteFactSlice()
			}
			return MakeFactsPointTo(lvars, coll)
		}
		// FactPointTo.cpp:210–224 — aggregate RHS: map pointer fields pairwise
		if rt.IsAggregate() {
			vars := MergePointeesOfPointer(rhs.Var.GetCollective(), indirect, facts)
			// nil = incomplete pointees
			if vars == nil {
				return IncompleteFactSlice()
			}
			var ret []*FactPointTo
			for _, vv := range vars {
				// Variable* always live from merge_pointees; nil hole fails closed
				if vv == nil {
					return IncompleteFactSlice()
				}
				ptrs := vv.FindPointerFields()
				// FindPointerFields nil = incomplete FieldVars
				if ptrs == nil {
					return IncompleteFactSlice()
				}
				// FactPointTo.cpp:216 — assert(lvars.size() == pointers.size())
				if len(lvars) != len(ptrs) {
					// fail closed — no soft invent min-length pairwise transfer
					return IncompleteFactSlice()
				}
				for j := 0; j < len(lvars); j++ {
					set := MergePointeesOfPointer(ptrs[j], 1, facts)
					// C++ make_fact with set as-is (may be empty); no invent garbage
					// set nil only when incomplete — fail closed whole transfer
					if set == nil {
						return IncompleteFactSlice()
					}
					fp := MakeFactPointToSet(lvars[j], set)
					if fp == nil {
						return IncompleteFactSlice()
					}
					ret = append(ret, fp)
				}
			}
			return ret
		}
		// FactPointTo.cpp:225–228 — merge_pointees(collective, indirect+1)
		// empty set is valid (no soft invent GarbagePtr); nil set = incomplete
		set := MergePointeesOfPointer(rhs.Var.GetCollective(), indirect+1, facts)
		if set == nil {
			return IncompleteFactSlice()
		}
		return MakeFactsPointToSet(lvars, set)
	case TermFunction:
		// FactPointTo.cpp:230–231 — assert(fi); no soft invent empty on missing invoke
		if rhs.Invoke == nil {
			return IncompleteFactSlice()
		}
		fi := rhs.Invoke
		// TODO: support pointer arithmetics (upstream); only FuncCall transfers
		if fi.User == nil {
			return IncompleteFactSlice()
		}
		fn := fi.User
		if fn.RV != nil && fn.RV.Type != nil && fn.RV.Type.IsAggregate() {
			ptrs := fn.RV.FindPointerFields()
			// pairwise like aggregate path; length mismatch is broken IR
			if len(lvars) != len(ptrs) {
				return IncompleteFactSlice()
			}
			var ret []*FactPointTo
			for i := 0; i < len(lvars); i++ {
				rvFact := GetReturnFactForInvocation(fi, ptrs[i])
				// missing return fact → fail closed (no invent GarbagePtr)
				if rvFact == nil {
					return IncompleteFactSlice()
				}
				// PointTo may be empty top; nil slice incomplete; holes fail MakeFactPointToSet
				set := rvFact.PointTo
				if set == nil {
					set = []*Variable{}
				}
				fp := MakeFactPointToSet(lvars[i], set)
				if fp == nil {
					return IncompleteFactSlice()
				}
				ret = append(ret, fp)
			}
			return ret
		}
		if fn.RV == nil {
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:250–252 — assert(rv_fact)
		rvFact := GetReturnFactForInvocation(fi, fn.RV)
		if rvFact == nil {
			return IncompleteFactSlice()
		}
		set := rvFact.PointTo
		if set == nil {
			set = []*Variable{}
		}
		return MakeFactsPointToSet(lvars, set)
	case TermAssignment:
		// FactPointTo.cpp:256–258 — peel embedded assign RHS
		if rhs.Assign == nil {
			return IncompleteFactSlice()
		}
		return RhsToLhsTransfer(facts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		// FactPointTo.cpp:259–261 — peel comma RHS
		return RhsToLhsTransfer(facts, lvars, rhs.CommaRHS)
	default:
		return IncompleteFactSlice()
	}
}

// AbstractFactForAssign mirrors FactPointTo::abstract_fact_for_assign.
// FactPointTo.cpp:266–295 — merge_pointees of LHS; pointer assign or pointer fields.
// lhsIndir peels Lhs::get_type() (var type after deref) for the pointer-typed branch.
func AbstractFactForAssign(factsIn []*FactPointTo, lhs *Variable, lhsIndir int, rhs *Expression) []*FactPointTo {
	if lhs == nil || lhs.Type == nil {
		// incomplete LHS IR — hole marker (not bare nil invent empty abstract success)
		return IncompleteFactSlice()
	}
	// find all pointed variables on LHS (merge_pointees of collective)
	lvars := MergePointeesOfPointer(lhs.GetCollective(), lhsIndir, factsIn)
	// FactPointTo.cpp:275–278 — if (lhs->get_type().eType == ePointer)
	// Lhs type is var type after get_indirect_level peels; mirror by peeling ptrTo.
	lhsTy := lhs.Type
	for i := 0; i < lhsIndir && lhsTy != nil; i++ {
		lhsTy = lhsTy.PtrType()
	}
	if lhsTy != nil && lhsTy.PtrType() != nil {
		// pointer-valued store (possibly *p when p is multi-level pointer)
		// FactPointTo.cpp:277 — transfer lvars as merged; incomplete is hole marker
		return RhsToLhsTransfer(factsIn, lvars, rhs)
	}
	// when assigning through *p (indir>0) or to aggregate, transfer to pointer fields
	// FactPointTo.cpp:280–293 — merge_pointees already yields collective at indir 0
	// FactPointTo.cpp:280–293 — union field assign: walk to container union, then
	// find_pointer_fields on that union (all pointer fields share storage).
	// Variable* always live in lvars; nil hole fails closed (no invent skip).
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:283–288 — is_inside_union_field → walk to eUnion container
		u := v
		if u.IsInsideUnionField() {
			if cu := u.GetContainerUnion(); cu != nil {
				u = cu
			} else {
				// walk FieldVarOf until Type is union
				for cur := u; cur != nil; cur = cur.FieldVarOf {
					if cur.Type != nil && cur.Type.IsUnion() {
						u = cur
						break
					}
				}
			}
			// FactPointTo.cpp:288 — assert(v && v->type->eType == eUnion)
			if u == nil || u.Type == nil || !u.Type.IsUnion() {
				// fail closed — no soft invent fields from non-union container
				return IncompleteFactSlice()
			}
		}
		// FactPointTo.cpp:289–292 — find_pointer_fields; rhs_to_lhs_transfer
		ptrs := u.FindPointerFields()
		if v.IsPointer() && lhsIndir > 0 {
			// assigning *p = rhs: also update pointer pointees
			for _, p := range MergePointeesOfPointer(v, 1, factsIn) {
				if p != nil && p.IsPointer() {
					ptrs = append(ptrs, p)
				}
			}
		}
		if len(ptrs) == 0 {
			continue
		}
		part := RhsToLhsTransfer(factsIn, ptrs, rhs)
		// incomplete transfer must not invent partial field abstract
		if !FactsComplete(part) {
			return IncompleteFactSlice()
		}
		out = append(out, part...)
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
	// Variable* always live in PointTo; nil hole fails closed as unequal
	// (no invent soft-skip hole as absent / equal past incomplete sets)
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		if p == nil {
			return false
		}
		set[p] = true
	}
	for _, p := range other.PointTo {
		if p == nil {
			return false
		}
		if !set[p] {
			return false
		}
	}
	return true
}

// Imply mirrors FactPointTo::imply — other.point_to ⊆ this.point_to.
// FactPointTo.cpp:602–609.
// Incomplete PointTo (nil hole) fails closed as not-imply — no invent cover past holes.
func (f *FactPointTo) Imply(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		if p == nil {
			return false
		}
		set[p] = true
	}
	for _, p := range other.PointTo {
		if p == nil {
			return false
		}
		if !set[p] {
			return false
		}
	}
	return true
}

// Join mirrors FactPointTo::join — union of points-to sets; returns true if changed.
// FactPointTo.cpp:563–578.
// Incomplete PointTo (nil hole) fails closed false without partial absorb
// (no invent soft-skip hole and still join later pointees).
func (f *FactPointTo) Join(other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		if p == nil {
			return false
		}
		set[p] = true
	}
	// pre-scan other for holes before mutating self
	for _, p := range other.PointTo {
		if p == nil {
			return false
		}
	}
	changed := false
	for _, p := range other.PointTo {
		if set[p] {
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
// Incomplete maps fail closed: *facts = IncompleteFactSlice(), false
// (no invent no-change success via FactsComplete(nil) or soft-append nil Clone).
func JoinVisitsInto(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		*facts = IncompleteFactSlice()
		return false
	}
	changed := false
	for _, nf := range newFacts {
		cur := FindRelatedPointTo(*facts, nf.Var)
		if cur == nil {
			cl := nf.Clone()
			if cl == nil {
				*facts = IncompleteFactSlice()
				return false
			}
			*facts = append(*facts, cl)
			changed = true
			continue
		}
		// join into clone then replace
		cp := cur.Clone()
		if cp == nil {
			*facts = IncompleteFactSlice()
			return false
		}
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
// Incomplete PointTo (nil hole) fails closed nil — no invent clone of broken set.
// Empty top (nil PointTo) clones as empty non-nil set.
func (f *FactPointTo) Clone() *FactPointTo {
	if f == nil {
		return nil
	}
	set := f.PointTo
	if set == nil {
		set = []*Variable{}
	}
	return MakeFactPointToSet(f.Var, set)
}

// FactsComplete reports whether every Fact* is live with complete PointTo sets.
// Incomplete maps/pointees must not soft-join or soft-filter past holes.
// Empty PointTo (IsTop) is complete; nil pointee slots are not.
// Note: FactsComplete(nil)==true (complete empty). Fail-closed incomplete wipes
// must use IncompleteFactSlice() so later checks do not invent empty success.
func FactsComplete(facts []*FactPointTo) bool {
	for _, f := range facts {
		if f == nil || f.Var == nil {
			return false
		}
		for _, p := range f.PointTo {
			if p == nil {
				return false
			}
		}
	}
	return true
}

// IncompleteFactSlice is the fail-closed incomplete fact-list marker.
// FactsComplete returns false. Distinct from complete empty (nil or non-nil {}).
// Use for GlobalFacts / map entry wipes so FactsComplete(nil) cannot invent success.
func IncompleteFactSlice() []*FactPointTo {
	return []*FactPointTo{nil}
}

// MergeFactInto merges new fact with lattice join (Fact::merge_fact).
// Fact.cpp:149–171 — strong replace only when not related; else join.
// Fact* always live at call sites; nil f or incomplete map fails closed
// IncompleteFactSlice (no invent no-op / empty-complete via FactsComplete(nil)).
func MergeFactInto(facts []*FactPointTo, f *FactPointTo) []*FactPointTo {
	// no invent treat nil fact as empty merge that preserves facts
	if f == nil {
		return IncompleteFactSlice()
	}
	// incomplete map must not invent join success when match appears before a hole
	if !FactsComplete(facts) {
		return IncompleteFactSlice()
	}
	for i, old := range facts {
		if old.Var == f.Var {
			if old.Imply(f) {
				// old already covers f
				return facts
			}
			// join: copy f, join old into it
			cp := f.Clone()
			if cp == nil {
				// incomplete PointTo on f — fail closed
				return IncompleteFactSlice()
			}
			_ = cp.Join(old)
			facts[i] = cp
			return facts
		}
	}
	cl := f.Clone()
	if cl == nil {
		return IncompleteFactSlice()
	}
	return append(facts, cl)
}

// MergeFacts mirrors merge_facts — merge each of new into facts.
// Fact.cpp:192–200.
// Returns whether any fact changed. Incomplete maps fail closed: *facts set nil,
// returns false (no invent skip partial join / keep broken partial).
func MergeFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		*facts = IncompleteFactSlice()
		return false
	}
	changed := false
	for _, f := range newFacts {
		before := FindRelatedPointTo(*facts, f.Var)
		merged := MergeFactInto(*facts, f)
		// MergeFactInto incomplete = hole marker (should not happen after pre-validate)
		if !FactsComplete(merged) {
			*facts = IncompleteFactSlice()
			return false
		}
		*facts = merged
		after := FindRelatedPointTo(*facts, f.Var)
		if before == nil || after == nil || !before.Equal(after) {
			changed = true
		}
	}
	return changed
}

// CloneFactSlice deep-clones a FactPointTo slice.
// Complete empty: nil in → nil out; non-nil {} → non-nil {}.
// Incomplete maps fail closed IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-complete clone success).
func CloneFactSlice(facts []*FactPointTo) []*FactPointTo {
	if facts == nil {
		return nil
	}
	if !FactsComplete(facts) {
		return IncompleteFactSlice()
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
		// Fact* always live after FactsComplete; Clone nil = incomplete PointTo
		cl := f.Clone()
		if cl == nil {
			return IncompleteFactSlice()
		}
		out = append(out, cl)
	}
	return out
}

// MarkDeadVar mirrors FactPointTo::mark_dead_var.
// FactPointTo.cpp:106–123 — replace/remove pointee v with garbage_ptr.
// Variable* always live in PointTo; nil hole fails closed (nil — no invent skip).
// Incomplete FieldVars on v fails closed (nil — no invent leave stack field
// pointees live because HasFieldVar returned false past a hole).
func (f *FactPointTo) MarkDeadVar(v *Variable) *FactPointTo {
	if f == nil || v == nil {
		return nil
	}
	if !v.FieldVarsComplete() {
		return nil
	}
	set := append([]*Variable(nil), f.PointTo...)
	pos := -1
	for i, p := range set {
		if p == nil {
			return nil
		}
		if p == v || v.HasFieldVar(p) || (p.FieldVarOf != nil && isAncestorField(p, v)) {
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
// MarkFuncEndLocals marks pointees that are locals as garbage/dead at function end.
// Variable* always live in locals/PointTo; nil hole fails closed (nil fact).
func (f *FactPointTo) MarkFuncEndLocals(locals []*Variable) *FactPointTo {
	if f == nil || len(locals) == 0 {
		return nil
	}
	localSet := make(map[*Variable]bool, len(locals))
	for _, l := range locals {
		if l == nil {
			return nil
		}
		localSet[l] = true
		exp := l.CollectExpandable()
		if !VariablesComplete(exp) {
			return nil
		}
		for _, fv := range exp {
			localSet[fv] = true
		}
	}
	set := append([]*Variable(nil), f.PointTo...)
	hasGarbage := false
	for _, p := range set {
		if p == nil {
			return nil
		}
		if p == GarbagePtr {
			hasGarbage = true
			break
		}
	}
	changed := false
	for i := 0; i < len(set); i++ {
		p := set[i]
		if IsSpecialPtr(p) || !localSet[p] {
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
// MarkFuncEnd marks stack pointees as garbage at function end.
// Variable* always live in PointTo; nil hole fails closed (nil fact).
// Incomplete Param/LocalVars stack lists fail closed (nil — no invent leave
// stack pointees live because IsVarOnStack returned false past a hole).
func (f *FactPointTo) MarkFuncEnd(fn *Function, stParent *Block) *FactPointTo {
	if f == nil || fn == nil {
		return nil
	}
	if !fn.StackScanComplete(stParent) {
		return nil
	}
	set := append([]*Variable(nil), f.PointTo...)
	hasGarbage := false
	for _, p := range set {
		if p == nil {
			return nil
		}
		if p == GarbagePtr {
			hasGarbage = true
			break
		}
	}
	changed := false
	for i := 0; i < len(set); i++ {
		v := set[i]
		if IsSpecialPtr(v) {
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
// Fact* always live; incomplete facts or stack lists fail closed (nil facts —
// no invent partial mark / leave stack pointees live past Param/LocalVars holes).
func MarkFuncEndOnFacts(facts *[]*FactPointTo, fn *Function, stParent *Block) {
	if facts == nil {
		return
	}
	if !FactsComplete(*facts) {
		*facts = IncompleteFactSlice()
		return
	}
	if fn != nil && !fn.StackScanComplete(stParent) {
		*facts = IncompleteFactSlice()
		return
	}
	for i, f := range *facts {
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
		// Variable* always live in PointTo; nil hole fails closed (no invent skip)
		if v == nil {
			return nil
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
		// Expression* always live in IndexExprs; nil hole fails closed
		if len(av.IndexExprs) > 0 {
			for k, exp := range av.IndexExprs {
				if exp == nil {
					return nil
				}
				if exp.UseVar(indexVar) {
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
			InitExprs:  append([]*Expression(nil), av.InitExprs...),
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
// Fact* always live; nil hole or failed rewrite fails closed (facts nil).
func UpdateFactsWithModifiedIndex(facts *[]*FactPointTo, indexVar *Variable) {
	if facts == nil || indexVar == nil {
		return
	}
	if !FactsComplete(*facts) {
		*facts = IncompleteFactSlice()
		return
	}
	for i, fp := range *facts {
		newFP := fp.UpdateWithModifiedIndex(indexVar)
		// UpdateWithModifiedIndex nil = incomplete pointees
		if newFP == nil {
			*facts = IncompleteFactSlice()
			return
		}
		if newFP != fp {
			(*facts)[i] = newFP
		}
	}
}

// MergePointeesOfPointers mirrors FactPointTo::merge_pointees_of_pointers.
// FactPointTo.cpp:680–704 — union of points-to sets for each pointer.
// FactPointTo.cpp:694 — assert(exist_fact): missing related fact fails closed
// (nil out — no invent soft-skip partial pointees mid-create or otherwise).
// Fact* always live; incomplete fact map or nil ptr/pointee holes fail closed.
func MergePointeesOfPointers(ptrs []*Variable, facts []*FactPointTo) []*Variable {
	// incomplete fact map fails closed (FindRelated would nil on first hole)
	if !FactsComplete(facts) {
		return nil
	}
	out := make([]*Variable, 0)
	seen := make(map[*Variable]bool)
	for _, p := range ptrs {
		if p == nil {
			return nil
		}
		if IsSpecialPtr(p) {
			continue
		}
		ft := FindRelatedPointTo(facts, p)
		// FactPointTo.cpp:694 assert(exist_fact) — fail closed, no invent skip
		if ft == nil {
			return nil
		}
		for _, pointee := range ft.PointTo {
			if pointee == nil {
				return nil
			}
			if seen[pointee] {
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
// Intermediate nil from MergePointeesOfPointers propagates (fail closed).
func MergePointeesOfPointer(ptr *Variable, indirect int, facts []*FactPointTo) []*Variable {
	if ptr == nil {
		return nil
	}
	tmp := []*Variable{ptr}
	for indirect > 0 {
		tmp = MergePointeesOfPointers(tmp, facts)
		// nil = incomplete merge (missing fact / holes) — stop, do not invent empty
		if tmp == nil {
			return nil
		}
		indirect--
	}
	return tmp
}

// IsPointingToLocals mirrors FactPointTo::is_pointing_to_locals.
// FactPointTo.cpp:487–526.
// Variable* always live in pointees; incomplete fact maps / stack scans / nil
// holes fail closed as true (no invent "not pointing to locals" past holes or
// FindRelatedPointTo nil-before-match on incomplete maps).
func IsPointingToLocals(v *Variable, b *Block, indirection int, facts []*FactPointTo) bool {
	if v == nil {
		return false
	}
	// incomplete LocalVars/Param: membership short-circuit invents not-local
	if b != nil && !b.StackScanComplete() {
		return true
	}
	if indirection == -1 {
		return v.IsVisibleLocal(b)
	}
	if !v.IsPointer() {
		return false
	}
	// incomplete fact maps: FindRelatedPointTo fails closed nil before related
	if !FactsComplete(facts) {
		return true
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
		// nil = incomplete merge
		if pointees == nil {
			return true
		}
	}
	for _, p := range pointees {
		if p == nil {
			return true
		}
		if IsSpecialPtr(p) {
			continue
		}
		if p.IsVisibleLocal(b) {
			return true
		}
		// recurse one level of pointees that are pointers
		if p.IsPointer() {
			if p.Type == nil {
				return true
			}
			for j := 0; j < p.Type.IndirectLevel(); j++ {
				nested := MergePointeesOfPointer(p, j+1, facts)
				if nested == nil {
					return true
				}
				for _, n := range nested {
					if n == nil {
						return true
					}
					if !IsSpecialPtr(n) && n.IsVisibleLocal(b) {
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
// UpdatePtrAliases merges facts into parallel ptrs/aliases tables.
// Fact* always live; nil hole fails closed (false — no invent skip partial alias).
// Returns false on incomplete maps; true when scan completed.
func UpdatePtrAliases(facts []*FactPointTo, ptrs *[]*Variable, aliases *[][]*Variable) bool {
	if ptrs == nil || aliases == nil {
		return false
	}
	for _, f := range facts {
		// Fact* always live; no invent skip nil holes as absent aliases
		if f == nil {
			return false
		}
		if f.Var == nil {
			return false
		}
		// FactPointTo.cpp: type != 0 — specials may have Type-nil; other Type-nil
		// is incomplete IR (fail closed — no invent soft-skip partial alias update)
		if f.Var.Type == nil {
			if IsSpecialPtr(f.Var) {
				continue
			}
			return false
		}
		// PointTo Variable* always live
		for _, v := range f.PointTo {
			if v == nil {
				return false
			}
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
	return true
}

// AggregateAllPointToSets mirrors FactPointTo::aggregate_all_pointto_sets.
// FactPointTo.cpp:792–804 — scan each non-builtin func FactMgr map_facts_out.
// FactPointTo.cpp:803 — assert(all_ptrs.size() == all_aliases.size()); kept by UpdatePtrAliases.
// Incomplete fact maps fail closed (clear aggregates — no invent partial AllPtrs).
func AggregateAllPointToSets(funcs []*Function, fms *FactMgrMap) {
	ClearPointToAggregates()
	for _, f := range funcs {
		// Function* always live on Funcs; no invent skip nil holes mid aggregate
		if f == nil {
			ClearPointToAggregates()
			return
		}
		if f.IsBuiltin {
			continue
		}
		// FactMgr always paired for user funcs after make_random_signature / make_first
		// no invent skip missing FM (partial aggregate)
		if fms == nil {
			ClearPointToAggregates()
			return
		}
		fm := fms.ForFunc(f)
		if fm == nil {
			ClearPointToAggregates()
			return
		}
		// prefer map_facts_out values; also include GlobalFacts
		for _, facts := range fm.MapFactsOut {
			if !UpdatePtrAliases(facts, &AllPtrs, &AllAliases) {
				ClearPointToAggregates()
				return
			}
		}
		if !UpdatePtrAliases(fm.GlobalFacts, &AllPtrs, &AllAliases) {
			ClearPointToAggregates()
			return
		}
	}
	// FactPointTo.cpp:803 — sizes must stay paired (no soft invent desync)
	if len(AllPtrs) != len(AllAliases) {
		ClearPointToAggregates()
	}
}
