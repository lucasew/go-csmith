// Upstream: FactPointTo.h / FactPointTo.cpp (points-to facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

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
// nil subject sticky (no invent FactPointTo{nil, garbage} shell / soft re-pick).
func NewFactPointTo(v *Variable) *FactPointTo {
	return NewFactPointToSess(nil, v)
}

func NewFactPointToSess(s *Session, v *Variable) *FactPointTo {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return &FactPointTo{Var: v, PointTo: []*Variable{GarbagePtr}}
}

// MakeFactPointTo mirrors FactPointTo::make_fact(v, point_to).
// nil subject/pointee sticky (no invent fact without live Variable* / soft re-pick).
func MakeFactPointTo(v *Variable, pointTo *Variable) *FactPointTo {
	return MakeFactPointToSess(nil, v, pointTo)
}

func MakeFactPointToSess(s *Session, v *Variable, pointTo *Variable) *FactPointTo {
	if v == nil || pointTo == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return &FactPointTo{Var: v, PointTo: []*Variable{pointTo}}
}

// MakeFactPointToSet mirrors FactPointTo::make_fact(v, set).
// nil subject sticky; nil set non-sticky incomplete merge_pointees (soft re-pick —
// no invent empty IsTop PointTo from nil); nil pointee hole sticky.
// Valid empty sets use non-nil empty slice []*Variable{}.
func MakeFactPointToSet(v *Variable, set []*Variable) *FactPointTo {
	return MakeFactPointToSetSess(nil, v, set)
}

func MakeFactPointToSetSess(s *Session, v *Variable, set []*Variable) *FactPointTo {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// nil set = incomplete merge_pointees path — non-sticky hole for soft re-pick
	if set == nil {
		return nil
	}
	for _, p := range set {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
	}
	cp := append([]*Variable(nil), set...)
	return &FactPointTo{Var: v, PointTo: cp}
}

// IsNull mirrors FactPointTo::is_null — any null_ptr in the set.
// Incomplete PointTo (nil hole) fails closed true — no invent not-null past holes.
func (f *FactPointTo) IsNull() bool {
	return f.IsNullSess(nil)
}

func (f *FactPointTo) IsNullSess(s *Session) bool {
	// Fact always live; sticky incomplete fails closed true (no invent not-null)
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
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
	return f.IsDeadSess(nil)
}

func (f *FactPointTo) IsDeadSess(s *Session) bool {
	// Fact always live; sticky incomplete fails closed true (no invent not-dead)
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if p == GarbagePtr {
			return true
		}
	}
	return false
}

// IsTBDOnly mirrors FactPointTo::is_tbd_only.
// Incomplete Fact/PointTo sticky false (no invent TBD-only / soft re-pick past holes).
func (f *FactPointTo) IsTBDOnly() bool {
	return f.IsTBDOnlySess(nil)
}

func (f *FactPointTo) IsTBDOnlySess(s *Session) bool {
	// Fact always live; sticky incomplete no invent TBD-only soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if len(f.PointTo) != 1 {
		return false
	}
	if f.PointTo[0] == nil {
		// incomplete PointTo hole sticky not-TBD-only
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.PointTo[0] == TBDPtr
}

// IsSpecialPtr mirrors FactPointTo::is_special_ptr.
func IsSpecialPtr(p *Variable) bool {
	return p == NullPtr || p == GarbagePtr || p == TBDPtr
}

// PointToStr mirrors FactPointTo::point_to_str.
// FactPointTo.cpp:530–540 — specials as 0/tbd/garbage; else "&name".
func PointToStr(v *Variable) string {
	return PointToStrSess(nil, v)
}

func PointToStrSess(s *Session, v *Variable) string {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	switch v {
	case NullPtr:
		return "0"
	case TBDPtr:
		return "tbd"
	case GarbagePtr:
		return "garbage"
	default:
		return "&" + v.Name
	}
}

// Size mirrors FactPointTo::size — number of pointees.
// FactPointTo.cpp:155.
func (f *FactPointTo) Size() int {
	return f.SizeSess(nil)
}

func (f *FactPointTo) SizeSess(s *Session) int {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	return len(f.PointTo)
}

// GetPointToVars mirrors FactPointTo::get_point_to_vars.
// FactPointTo.h:69 — returns pointee slice (may be nil when empty).
func (f *FactPointTo) GetPointToVars() []*Variable {
	return f.GetPointToVarsSess(nil)
}

func (f *FactPointTo) GetPointToVarsSess(s *Session) []*Variable {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f.PointTo
}

// Clear mirrors FactPointTo::clear / empty point-to set (set_top lattice).
func (f *FactPointTo) Clear() {
	f.ClearSess(nil)
}

// ClearSess is Clear with explicit session residual sticky.
func (f *FactPointTo) ClearSess(s *Session) {
	f.SetTopSess(s)
}

// Empty mirrors FactPointTo::empty — no pointees (same as is_top).
func (f *FactPointTo) Empty() bool {
	return f.EmptySess(nil)
}

// EmptySess is Empty with explicit session residual sticky.
func (f *FactPointTo) EmptySess(s *Session) bool {
	return f.IsTopSess(s)
}

// IsRelated mirrors Fact::is_related for PointTo — same category + same subject var.
// Fact.h:81–83.
func (f *FactPointTo) IsRelated(other *FactPointTo) bool {
	return f.IsRelatedSess(nil, other)
}

func (f *FactPointTo) IsRelatedSess(s *Session, other *FactPointTo) bool {
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.Var == other.Var
}

// FindRelatedPointTo mirrors find_related_fact for ePointTo (var identity).
// Fact* always live; nil hole fails closed (nil — no invent skip to later match).}

func FindRelatedPointTo(facts []*FactPointTo, p *Variable) *FactPointTo {
	return FindRelatedPointToSess(nil, facts, p)
}

func FindRelatedPointToSess(s *Session, facts []*FactPointTo, p *Variable) *FactPointTo {
	// subject always live; sticky no invent miss / soft-skip nil key
	if p == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	for _, f := range facts {
		// Fact* always live; sticky no invent skip hole to later match
		if f == nil {
			sessNoteError(s, ErrGeneric)
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
// Variable always live; sticky invalid (no invent valid soft-skip past hole).
// Type* always live for non-special subjects; Type-nil sticky invalid (no invent
// valid true via related-fact match past Type-nil pointer shell).
// Incomplete fact maps fail closed sticky invalid (no invent valid / soft re-pick past holes).
// Missing related fact / null/dead policy rejects stay non-sticky false.
// Ambient InUserInvocationRevisit bridge; generation prefers IsValidPtrSess.
func IsValidPtr(p *Variable, facts []*FactPointTo, nullProb, deadProb int) bool {
	return IsValidPtrSess(nil, p, facts, nullProb, deadProb)
}

// IsValidPtrSess is IsValidPtr with InUserInvocationRevisit from bag s.
func IsValidPtrSess(s *Session, p *Variable, facts []*FactPointTo, nullProb, deadProb int) bool {
	if p == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// Type* always live for non-special subjects; Type-nil sticky invalid
	// (no invent valid via fact lookup past Type-nil pointer shell)
	if !IsSpecialPtr(p) && p.Type == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// FactPointTo.cpp:415–426 — exact var* match. Array facts live on get_collective()
	// (add_new_var_fact_and_update_inout_maps(..., get_collective())). ExpressionVariable
	// may hold itemized (Collective!=nil). During FunctionInvocationUser::revisit only,
	// fall back to collective so IsValidPtr matches opportunistic_validate's lookup
	// without unlocking itemized during gen ExpressionVariable select (stream-stable).
	// seed-10054: nested revisit miss itemized while collective has live fact.
	fact := FindRelatedPointTo(facts, p)
	// residual ERROR sticky — no invent valid false/true soft-skip past FindRelated hole
	if sessHasError(s) {
		return false
	}
	if fact == nil && sessOrAmbient(s).InUserInvocationRevisit && p.AsArray != nil && p.AsArray.Collective != nil {
		fact = FindRelatedPointTo(facts, &p.AsArray.Collective.Variable)
		if sessHasError(s) {
			return false
		}
	}
	if fact == nil {
		// missing subject is complete invalid
		return false
	}
	if nullProb <= 0 && fact.IsNull() {
		// residual ERROR sticky — no invent valid past IsNull residual
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue valid past IsNull residual false path
	if sessHasError(s) {
		return false
	}
	if deadProb <= 0 && fact.IsDead() {
		// residual ERROR sticky — no invent valid past IsDead residual
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent valid true past IsDead residual false path
	if sessHasError(s) {
		return false
	}
	return true
}

// IsDanglingPtr mirrors FactPointTo::is_dangling_ptr.
// FactPointTo.cpp:476–482 — related fact is dead (and dead deref not allowed).
// Variable always live; sticky dangling true (no invent not-dangling soft-skip past hole).
// Type* always live for non-special subjects; Type-nil sticky dangling true (restrictive —
// no invent not-dangling / soft re-pick past Type-nil pointer shell).
// Incomplete fact maps fail closed sticky as dangling (true — no invent not-dangling
// / soft re-pick past holes when FindRelated would skip holes).
func IsDanglingPtr(p *Variable, facts []*FactPointTo, deadProb int) bool {
	return IsDanglingPtrSess(nil, p, facts, deadProb)
}

func IsDanglingPtrSess(s *Session, p *Variable, facts []*FactPointTo, deadProb int) bool {
	if p == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// Type* always live for non-special subjects; Type-nil sticky dangling
	// (restrictive — no invent not-dangling past Type-nil shell)
	if !IsSpecialPtr(p) && p.Type == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return true
	}
	fact := FindRelatedPointTo(facts, p)
	// residual ERROR sticky — no invent not-dangling soft-skip past FindRelated hole
	if sessHasError(s) {
		return true
	}
	if fact == nil {
		return false
	}
	dead := fact.IsDead()
	// residual ERROR sticky — no invent not-dangling soft-skip past IsDead hole
	if sessHasError(s) {
		return true
	}
	return dead && deadProb == 0
}

// OpportunisticValidate mirrors FactPointTo::opportunistic_validate.
// FactPointTo.cpp:442–472 — 0 reject, 1 ok, 2 allowed unsafe deref via flipcoin.
// FactPointTo.cpp:455 / 464 — always rnd_flipcoin(prob) when is_null / is_dead,
// including prob==0 (still consumes RNG + traces F p=0). Do not skip the draw.
// Incomplete fact maps fail closed as reject 0 (no invent ok via hole skip).}

func OpportunisticValidate(r *Rng, v *Variable, typ *Type, facts []*FactPointTo, nullProb, deadProb int) int {
	return OpportunisticValidateSess(nil, r, v, typ, facts, nullProb, deadProb)
}

func OpportunisticValidateSess(s *Session, r *Rng, v *Variable, typ *Type, facts []*FactPointTo, nullProb, deadProb int) int {
	// live Variable* + Type* required; sticky no invent "not valid" soft success past hole
	if v == nil || v.Type == nil || typ == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	// incomplete facts fail closed sticky (no invent soft re-pick as "not valid ptr")
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	// no extra indirection needed
	if v.Type.IndirectLevelSess(s) <= typ.IndirectLevelSess(s) {
		return 1
	}
	// residual ERROR sticky — no invent soft-validate past IndirectLevel residual
	if sessHasError(s) {
		return 0
	}
	// FactPointTo.cpp:448–450 — FactPointTo tmp(var->get_collective()); find_related_fact
	coll := varCollectiveSess(s, v)
	if coll == nil {
		// residual ERROR sticky — no invent soft-miss fact past get_collective hole
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return 0
	}
	fp := FindRelatedPointTo(facts, coll)
	// residual ERROR sticky — no invent soft-continue validate past FindRelated hole
	if sessHasError(s) {
		return 0
	}
	if fp == nil {
		return 0
	}
	ret := 0
	if fp.IsNull() {
		// residual ERROR sticky — no invent ok past IsNull residual hole
		if sessHasError(s) {
			return 0
		}
		// FactPointTo.cpp:455 — rnd_flipcoin(null_pointer_dereference_prob()) always
		// (p=0 still draws). Process RNG always live; sticky no invent reject without draw.
		if r == nil {
			sessNoteError(s, ErrGeneric)
			return 0
		}
		p := nullProb
		if p < 0 {
			p = 0
		}
		if r.RndFlipcoin(uint32(p)) {
			ret = 2
		} else {
			return 0
		}
	} else {
		// residual ERROR sticky — no invent soft-continue not-null past IsNull residual false
		if sessHasError(s) {
			return 0
		}
		ret = 1
	}
	if fp.IsDead() {
		// residual ERROR sticky — no invent ok past IsDead residual hole
		if sessHasError(s) {
			return 0
		}
		// FactPointTo.cpp:464 — rnd_flipcoin(dead_pointer_dereference_prob()) always
		if r == nil {
			sessNoteError(s, ErrGeneric)
			return 0
		}
		p := deadProb
		if p < 0 {
			p = 0
		}
		if r.RndFlipcoin(uint32(p)) {
			ret = 2
		} else {
			return 0
		}
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent not-dead soft-skip past IsDead residual false
		return 0
	}
	return ret
}

// MakeFactsPointTo mirrors FactPointTo::make_facts(vars, point_to).
// FactPointTo.cpp:340–348 — vars[i] always live; skip only type==null specials.
// no invent skip of nil Variable* holes as partial success; non-special Type-nil
// is incomplete IR (fail closed sticky whole batch — IncompleteFactSlice, not bare nil;
// FactsComplete(nil)==true invents empty-complete make_facts / soft re-pick).}

func MakeFactsPointTo(lvars []*Variable, pointTo *Variable) []*FactPointTo {
	return MakeFactsPointToSess(nil, lvars, pointTo)
}

func MakeFactsPointToSess(s *Session, lvars []*Variable, pointTo *Variable) []*FactPointTo {
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		// type null: specials (null/garbage/tbd) skipped; other broken IR fails closed sticky
		if v.Type == nil {
			if IsSpecialPtr(v) {
				continue
			}
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		f := MakeFactPointTo(v, pointTo)
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// MakeFactsPointToSet mirrors FactPointTo::make_facts(vars, set).
// same live-vars rules as MakeFactsPointTo; nil set fails closed sticky IncompleteFactSlice
// (no invent empty complete — FactsComplete(nil)==true / soft re-pick past hole).}

func MakeFactsPointToSet(lvars []*Variable, set []*Variable) []*FactPointTo {
	return MakeFactsPointToSetSess(nil, lvars, set)
}

func MakeFactsPointToSetSess(s *Session, lvars []*Variable, set []*Variable) []*FactPointTo {
	if set == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	var out []*FactPointTo
	for _, v := range lvars {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		if v.Type == nil {
			if IsSpecialPtr(v) {
				continue
			}
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		f := MakeFactPointToSet(v, set)
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		out = append(out, f)
	}
	return out
}

// RhsToLhsTransfer mirrors FactPointTo::rhs_to_lhs_transfer.
// FactPointTo.cpp:158–263 — const/null/garbage, &var, pointer copy,
// aggregate field transfer, function return facts, assign/comma peel.
// Hard IR asserts (non-pointer lvars, nil Var/Invoke/Assign, multi-level &,
// len mismatch, missing rv_fact) fail closed sticky IncompleteFactSlice so soft
// re-pick cannot invent empty transfer success. Incomplete MergePointees /
// FindPointerFields / fact-map holes stay non-sticky hole markers.}

func RhsToLhsTransfer(facts []*FactPointTo, lvars []*Variable, rhs *Expression) []*FactPointTo {
	return RhsToLhsTransferSess(nil, facts, lvars, rhs)
}

func RhsToLhsTransferSess(s *Session, facts []*FactPointTo, lvars []*Variable, rhs *Expression) []*FactPointTo {
	if len(lvars) == 0 {
		// complete empty transfer targets (not incomplete)
		return nil
	}
	// FactPointTo.cpp:164–167 — assert all possible LHS are pointers
	for _, v := range lvars {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		if !v.IsPointerSess(s) {
			// residual ERROR sticky — no invent soft-skip transfer past IsPointer residual hole
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			// hard IR sticky — no invent empty transfer / soft re-pick past assert
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		// residual ERROR sticky — no invent soft-continue later LHS past IsPointer residual true
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
	}
	// FactPointTo.cpp:168–169 — rhs==nullptr → garbage (param missing / abstract nullptr)
	// fair C++ path for AddParamFacts missing args — not invent; mirrors nullptr value
	if rhs == nil {
		return MakeFactsPointToSess(s, lvars, GarbagePtr)
	}
	// Compound terms first — TermFunction without Invoke has nil GetType and must
	// not invent complete GarbagePtr via the scalar non-pointer branch (soft invent).
	switch rhs.Term {
	case TermFunction, TermAssignment, TermCommaExpr:
		// structured transfer below (after rt fetch for pointer-like checks in Constant/Variable)
	default:
		rt0 := rhs.GetTypeSess(s)
		// residual ERROR sticky — no invent soft-continue transfer past GetType residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		// incomplete type sticky Incomplete (no invent GarbagePtr complete success)
		if rt0 == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		// non-pointer, non-union RHS (FactPointTo.cpp:172–178)
		isPtrLike := rt0.IsPointerLikeSess(s)
		// residual ERROR sticky — no invent soft-continue past IsPointerLike residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		isUn := rt0.IsUnionSess(s)
		// residual ERROR sticky — no invent soft-continue past IsUnion residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		if !isPtrLike && !isUn {
			// equals(0) and size >= 8 → null else garbage
			eq0 := rhs.EqualsIntSess(s, 0)
			// residual ERROR sticky — no invent null/garbage past EqualsInt residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			sz := rt0.SizeInBytesSess(s)
			// residual ERROR sticky — no invent null/garbage past SizeInBytes residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			if eq0 && sz >= 8 {
				return MakeFactsPointToSess(s, lvars, NullPtr)
			}
			return MakeFactsPointToSess(s, lvars, GarbagePtr)
		}
	}
	rt := rhs.GetTypeSess(s)
	// residual ERROR sticky — no invent soft-continue transfer past GetType residual
	if sessHasError(s) {
		return IncompleteFactSlice()
	}
	switch rhs.Term {
	case TermConstant:
		if rt == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		if rt.IsPointerLikeSess(s) {
			// residual ERROR sticky — no invent soft-continue past IsPointerLike residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			eq0 := rhs.EqualsIntSess(s, 0)
			// residual ERROR sticky — no invent null/garbage past EqualsInt residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			if eq0 {
				return MakeFactsPointToSess(s, lvars, NullPtr)
			}
			return MakeFactsPointToSess(s, lvars, GarbagePtr)
		}
		// residual ERROR sticky — no invent soft-continue past IsPointerLike residual false
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:186–193 — union constant field0 "0" → null on field0 pointers
		if rt.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-continue past IsUnion residual hole
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			lv0 := lvars[0]
			if lv0 != nil && lv0.FieldVarOf != nil {
				// parent Type* always live; Type-nil sticky (no invent GarbagePtr
				// complete soft-skip past incomplete union-parent shell)
				if lv0.FieldVarOf.Type == nil {
					if !IsSpecialPtr(lv0.FieldVarOf) {
						sessNoteError(s, ErrGeneric)
						return IncompleteFactSlice()
					}
				} else if lv0.FieldVarOf.Type.IsUnionSess(s) && lv0.GetFieldIDSess(s) == 0 {
					// residual ERROR sticky — no invent soft-continue past IsUnion/GetFieldID residual
					if sessHasError(s) {
						return IncompleteFactSlice()
					}
					// Constant::get_field(0) == "0"
					if rhs.Con != nil && rhs.Con.GetFieldSess(s, 0) == "0" {
						// residual ERROR sticky — no invent null past GetField residual
						if sessHasError(s) {
							return IncompleteFactSlice()
						}
						return MakeFactsPointToSess(s, lvars, NullPtr)
					}
					// residual ERROR sticky — no invent soft-continue past GetField residual false
					if sessHasError(s) {
						return IncompleteFactSlice()
					}
					eq0 := rhs.EqualsIntSess(s, 0)
					// residual ERROR sticky — no invent null past EqualsInt residual
					if sessHasError(s) {
						return IncompleteFactSlice()
					}
					if eq0 {
						return MakeFactsPointToSess(s, lvars, NullPtr)
					}
				} else if sessHasError(s) {
					// residual ERROR sticky — no invent soft-continue past IsUnion residual false
					return IncompleteFactSlice()
				}
			}
			return MakeFactsPointToSess(s, lvars, GarbagePtr)
		}
		// residual ERROR sticky — no invent soft-continue past IsUnion residual false
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:195–196 — assert(0); hard IR sticky (no soft invent garbage)
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	case TermVariable:
		// C++ always has ExpressionVariable; nil var is broken IR sticky
		if rhs.Var == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		// incomplete type IR sticky (no invent level-0 transfer / false address-of)
		indirect, iok := rhs.IndirectLevelCompleteSess(s)
		if !iok {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		if indirect < 0 {
			// FactPointTo.cpp:202–207 — taking address; multi-level & not allowed
			// assert(indirect == -1); hard IR sticky (no soft invent for indirect < -1)
			if indirect != -1 {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice()
			}
			// GetCollective always live for address-of; nil is broken IR sticky
			coll := rhs.Var.GetCollectiveSess(s)
			// residual ERROR sticky — no invent soft-address past GetCollective residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			if coll == nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice()
			}
			return MakeFactsPointToSess(s, lvars, coll)
		}
		// FactPointTo.cpp:210–224 — aggregate RHS: map pointer fields pairwise
		if rt.IsAggregateSess(s) {
			// incomplete collective / pointees — non-sticky abstract hole marker
			coll := rhs.Var.GetCollectiveSess(s)
			// residual ERROR sticky — no invent soft-merge aggregate past GetCollective residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			vars := MergePointeesOfPointerSess(s, coll, indirect, facts)
			// residual ERROR sticky — no invent soft-merge past MergePointees residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			if !VariablesComplete(vars) {
				return IncompleteFactSlice()
			}
			var ret []*FactPointTo
			for _, vv := range vars {
				ptrs := vv.FindPointerFieldsSess(s)
				// residual ERROR sticky — no invent soft-merge past FindPointerFields residual
				if sessHasError(s) {
					return IncompleteFactSlice()
				}
				// incomplete FieldVars — hard IR sticky (no invent skip field hole)
				if !VariablesComplete(ptrs) {
					sessNoteError(s, ErrGeneric)
					return IncompleteFactSlice()
				}
				// FactPointTo.cpp:216 — assert(lvars.size() == pointers.size())
				// NDEBUG elides assert; pairwise transfer only the overlapping prefix
				// (avoid UB / sticky-poison whole generation when field counts diverge).
				n := len(lvars)
				if len(ptrs) < n {
					n = len(ptrs)
				}
				for j := 0; j < n; j++ {
					set := MergePointeesOfPointerSess(s, ptrs[j], 1, facts)
					// incomplete set — non-sticky abstract hole (fact-map soft re-pick)
					if !VariablesComplete(set) {
						return IncompleteFactSlice()
					}
					fp := MakeFactPointToSet(lvars[j], set)
					if fp == nil {
						// broken make fact IR sticky
						sessNoteError(s, ErrGeneric)
						return IncompleteFactSlice()
					}
					ret = append(ret, fp)
				}
			}
			return ret
		}
		// FactPointTo.cpp:225–228 — merge_pointees(collective, indirect+1)
		// empty set is valid (no soft invent GarbagePtr); incomplete set non-sticky
		coll := rhs.Var.GetCollectiveSess(s)
		// residual ERROR sticky — no invent soft-merge pointees past GetCollective residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		set := MergePointeesOfPointerSess(s, coll, indirect+1, facts)
		// residual ERROR sticky — no invent soft-merge past MergePointees residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		if !VariablesComplete(set) {
			return IncompleteFactSlice()
		}
		return MakeFactsPointToSetSess(s, lvars, set)
	case TermFunction:
		// FactPointTo.cpp:230–231 — assert(fi); hard IR sticky when Invoke nil
		if rhs.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		fi := rhs.Invoke
		// TODO: support pointer arithmetics (upstream); only FuncCall transfers
		// non-user invoke (stdlib / incomplete) — non-sticky hole for soft re-pick
		if fi.User == nil {
			return IncompleteFactSlice()
		}
		fn := fi.User
		if fn.RV == nil {
			// incomplete RV shell — non-sticky hole (soft re-pick factories)
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:236 — fiu->get_type() / rv always live Type*; Type-nil sticky
		// (no invent scalar rv_fact soft-transfer past incomplete aggregate RV type)
		// Special null/garbage/tbd have Type nil by design — complete non-aggregate path.
		if fn.RV.Type == nil {
			if !IsSpecialPtr(fn.RV) {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice()
			}
		} else if fn.RV.Type.IsAggregateSess(s) {
			// residual ERROR sticky — no invent soft-transfer past IsAggregate residual true
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			ptrs := fn.RV.FindPointerFieldsSess(s)
			// residual ERROR sticky — no invent soft-transfer past FindPointerFields residual
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			if !VariablesComplete(ptrs) {
				// FieldVars hole sticky
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice()
			}
			// pairwise like aggregate path; length mismatch is broken IR sticky
			if len(lvars) != len(ptrs) {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice()
			}
			var ret []*FactPointTo
			for i := 0; i < len(lvars); i++ {
				rvFact := GetReturnFactForInvocation(fi, ptrs[i])
				// missing return fact during generation — non-sticky hole (soft re-pick)
				if rvFact == nil {
					return IncompleteFactSlice()
				}
				// PointTo may be empty top; nil slice → complete empty for set copy
				set := rvFact.PointTo
				if set == nil {
					set = []*Variable{}
				}
				fp := MakeFactPointToSet(lvars[i], set)
				if fp == nil {
					sessNoteError(s, ErrGeneric)
					return IncompleteFactSlice()
				}
				ret = append(ret, fp)
			}
			return ret
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-scalar rv transfer past IsAggregate residual false
			return IncompleteFactSlice()
		}
		// FactPointTo.cpp:250–252 — missing rv_fact during generation is non-sticky
		// hole (soft re-pick AddParamFacts / call factories; no invent GarbagePtr)
		rvFact := GetReturnFactForInvocation(fi, fn.RV)
		if rvFact == nil {
			return IncompleteFactSlice()
		}
		set := rvFact.PointTo
		if set == nil {
			set = []*Variable{}
		}
		return MakeFactsPointToSetSess(s, lvars, set)
	case TermAssignment:
		// FactPointTo.cpp:256–258 — peel embedded assign RHS
		// incomplete Assign/Expr sticky — no invent GarbagePtr via nil-rhs peel soft path
		if rhs.Assign == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		if rhs.Assign.Expr == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		return RhsToLhsTransferSess(s, facts, lvars, rhs.Assign.Expr)
	case TermCommaExpr:
		// FactPointTo.cpp:259–261 — peel comma RHS
		// incomplete CommaRHS sticky — no invent GarbagePtr via nil-rhs peel soft path
		if rhs.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		return RhsToLhsTransferSess(s, facts, lvars, rhs.CommaRHS)
	default:
		// unknown term — non-sticky hole (soft re-pick factories)
		return IncompleteFactSlice()
	}
}

// AbstractFactForAssign mirrors FactPointTo::abstract_fact_for_assign.
// FactPointTo.cpp:266–295 — merge_pointees of LHS; pointer assign or pointer fields.
// Returns (facts_out, lvar_cnt) where lvar_cnt is lvars.size() BEFORE make_facts
// filters type-null specials (FactPointTo.cpp:277–278 / 294 return lvars.size()).
// FactMgr.cpp:376–388 uses that count for renew (cnt==1) vs merge (may-point-to).
// lhsIndir peels Lhs::get_type() (var type after deref) for the pointer-typed branch.
// Hard IR (nil lhs/Type, broken union container, FieldVars holes) sticky; incomplete
// MergePointees / abstract transfer results stay non-sticky hole markers.}

func AbstractFactForAssign(factsIn []*FactPointTo, lhs *Variable, lhsIndir int, rhs *Expression) ([]*FactPointTo, int) {
	return AbstractFactForAssignSess(nil, factsIn, lhs, lhsIndir, rhs)
}

func AbstractFactForAssignSess(s *Session, factsIn []*FactPointTo, lhs *Variable, lhsIndir int, rhs *Expression) ([]*FactPointTo, int) {
	if lhs == nil || lhs.Type == nil {
		// incomplete LHS IR sticky (no invent empty abstract / soft re-pick past hole)
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice(), 0
	}
	// find all pointed variables on LHS (merge_pointees of collective)
	// incomplete pointees — non-sticky abstract hole (fact-map soft re-pick)
	coll := lhs.GetCollectiveSess(s)
	// residual ERROR sticky — no invent soft-abstract past GetCollective residual
	if sessHasError(s) {
		return IncompleteFactSlice(), 0
	}
	lvars := MergePointeesOfPointerSess(s, coll, lhsIndir, factsIn)
	// residual ERROR sticky — no invent soft-abstract past MergePointees residual
	if sessHasError(s) {
		return IncompleteFactSlice(), 0
	}
	if !VariablesComplete(lvars) {
		return IncompleteFactSlice(), 0
	}
	lvarCnt := len(lvars)
	// FactPointTo.cpp:275–278 — if (lhs->get_type().eType == ePointer)
	// Lhs type is var type after get_indirect_level peels; mirror by peeling ptrTo.
	lhsTy := lhs.Type
	for i := 0; i < lhsIndir && lhsTy != nil; i++ {
		lhsTy = lhsTy.PtrType()
		// residual ERROR sticky — no invent soft-peel past PtrType residual
		if sessHasError(s) {
			return IncompleteFactSlice(), 0
		}
	}
	if lhsTy != nil {
		pt := lhsTy.PtrType()
		// residual ERROR sticky — no invent soft-store path past PtrType residual
		if sessHasError(s) {
			return IncompleteFactSlice(), 0
		}
		if pt != nil {
			// pointer-valued store (possibly *p when p is multi-level pointer)
			// FactPointTo.cpp:277–278 — transfer; return lvars.size() for renew/merge
			return RhsToLhsTransferSess(s, factsIn, lvars, rhs), lvarCnt
		}
	}
	// when assigning through *p (indir>0) or to aggregate, transfer to pointer fields
	// FactPointTo.cpp:280–293 — merge_pointees already yields collective at indir 0
	// FactPointTo.cpp:280–293 — union field assign: walk to container union, then
	// find_pointer_fields on that union (all pointer fields share storage).
	var out []*FactPointTo
	for _, v := range lvars {
		// FactPointTo.cpp:283–288 — is_inside_union_field → walk to eUnion container
		u := v
		if u.IsInsideUnionFieldSess(s) {
			// residual ERROR sticky — no invent soft-continue transfer past IsInsideUnionField hole
			if sessHasError(s) {
				return IncompleteFactSlice(), 0
			}
			if cu := u.GetContainerUnionSess(s); cu != nil {
				// residual ERROR sticky — no invent container true past GetContainerUnion hole
				if sessHasError(s) {
					return IncompleteFactSlice(), 0
				}
				u = cu
			} else {
				// residual ERROR sticky — no invent soft-continue walk past GetContainerUnion residual
				if sessHasError(s) {
					return IncompleteFactSlice(), 0
				}
				// walk FieldVarOf until Type is union
				// Type* always live on ancestry; Type-nil non-special sticky
				// (no invent soft-skip incomplete parent past walk to assert)
				for cur := u; cur != nil; cur = cur.FieldVarOf {
					if cur.Type == nil {
						if !IsSpecialPtr(cur) {
							sessNoteError(s, ErrGeneric)
							return IncompleteFactSlice(), 0
						}
						continue
					}
					if cur.Type.IsUnionSess(s) {
						// residual ERROR sticky — no invent soft-container past IsUnion residual true
						if sessHasError(s) {
							return IncompleteFactSlice(), 0
						}
						u = cur
						break
					}
					// residual ERROR sticky — no invent soft-continue walk past IsUnion residual false
					if sessHasError(s) {
						return IncompleteFactSlice(), 0
					}
				}
			}
			// FactPointTo.cpp:288 — assert(v && v->type->eType == eUnion) hard sticky
			if u == nil || u.Type == nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice(), 0
			}
			if !u.Type.IsUnionSess(s) {
				// residual ERROR sticky — no invent soft-assert past IsUnion residual
				if sessHasError(s) {
					return IncompleteFactSlice(), 0
				}
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice(), 0
			}
			// residual ERROR sticky — no invent soft-continue past IsUnion residual true
			if sessHasError(s) {
				return IncompleteFactSlice(), 0
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-skip not-inside past IsInside residual false
			return IncompleteFactSlice(), 0
		}
		// FactPointTo.cpp:289–292 — find_pointer_fields; rhs_to_lhs_transfer
		ptrs := u.FindPointerFieldsSess(s)
		// residual ERROR sticky — no invent soft-transfer past FindPointerFields residual
		if sessHasError(s) {
			return IncompleteFactSlice(), 0
		}
		if !VariablesComplete(ptrs) {
			// FieldVars hole sticky
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice(), 0
		}
		isPtr := v.IsPointerSess(s)
		// residual ERROR sticky — no invent soft-skip *p path past IsPointer residual
		if sessHasError(s) {
			return IncompleteFactSlice(), 0
		}
		if isPtr && lhsIndir > 0 {
			// assigning *p = rhs: also update pointer pointees
			// incomplete more — non-sticky abstract (fact-map soft re-pick)
			more := MergePointeesOfPointerSess(s, v, 1, factsIn)
			// residual ERROR sticky — no invent soft-abstract past MergePointees residual
			if sessHasError(s) {
				return IncompleteFactSlice(), 0
			}
			if !VariablesComplete(more) {
				return IncompleteFactSlice(), 0
			}
			for _, p := range more {
				// PointTo Variable* always live after VariablesComplete
				// Type* always live for non-special pointees; Type-nil sticky
				// (IsPointer residual ERROR+false soft-skips then invents partial
				// transfer past Type-nil shell without pairing that pointee)
				if p.Type == nil && !IsSpecialPtr(p) {
					sessNoteError(s, ErrGeneric)
					return IncompleteFactSlice(), 0
				}
				if p.IsPointerSess(s) {
					// residual ERROR sticky — no invent soft-skip then partial transfer past hole
					if sessHasError(s) {
						return IncompleteFactSlice(), 0
					}
					ptrs = append(ptrs, p)
				} else if sessHasError(s) {
					return IncompleteFactSlice(), 0
				}
			}
		}
		if len(ptrs) == 0 {
			continue
		}
		part := RhsToLhsTransferSess(s, factsIn, ptrs, rhs)
		// incomplete transfer must not invent partial field abstract
		// (sticky only if RhsToLhsTransfer hard path already SetError)
		if !FactsComplete(part) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteFactSlice(), 0
		}
		// residual ERROR sticky — no invent soft-append partial transfer past hard IR hole
		if sessHasError(s) {
			return IncompleteFactSlice(), 0
		}
		out = append(out, part...)
	}
	// FactPointTo.cpp:294 — return lvars.size() (may-point-to count includes specials)
	return out, lvarCnt
}

// Equal reports same var and same points-to set.
// Incomplete PointTo nil hole fails closed sticky as unequal (no invent equal / soft re-pick).}

func (f *FactPointTo) Equal(other *FactPointTo) bool {
	return f.EqualSess(nil, other)
}

func (f *FactPointTo) EqualSess(s *Session, other *FactPointTo) bool {
	// both Fact* always live; sticky incomplete no invent not-equal soft-skip
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	// Variable* always live in PointTo; scan holes before len (len may differ with holes)
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
	}
	for _, p := range other.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
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
// Incomplete PointTo (nil hole) fails closed sticky as not-imply (no invent cover / soft re-pick).
func (f *FactPointTo) Imply(other *FactPointTo) bool {
	return f.ImplySess(nil, other)
}

func (f *FactPointTo) ImplySess(s *Session, other *FactPointTo) bool {
	// both Fact* always live; sticky incomplete no invent not-imply soft-skip
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		set[p] = true
	}
	for _, p := range other.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
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
// Incomplete PointTo (nil hole) fails closed sticky false without partial absorb
// (no invent soft-skip hole and still join later pointees / soft re-pick).
func (f *FactPointTo) Join(other *FactPointTo) bool {
	return f.JoinSess(nil, other)
}

func (f *FactPointTo) JoinSess(s *Session, other *FactPointTo) bool {
	// both Fact* always live; sticky incomplete no invent join no-op soft-skip
	if f == nil || other == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.Var != other.Var {
		return false
	}
	set := make(map[*Variable]bool, len(f.PointTo))
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		set[p] = true
	}
	// pre-scan other for holes before mutating self sticky
	for _, p := range other.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
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
// Nil shells sticky; Join already sticky on PointTo holes.
func (f *FactPointTo) JoinVisits(other *FactPointTo) bool {
	return f.JoinVisitsSess(nil, other)
}

func (f *FactPointTo) JoinVisitsSess(s *Session, other *FactPointTo) bool {
	if f == nil || other == nil || f.Var != other.Var {
		if f == nil || other == nil {
			sessNoteError(s, ErrGeneric)
		}
		return false
	}
	if other.IsTBDOnlySess(s) {
		// residual ERROR sticky — no invent ignore-TBD soft-skip past IsTBDOnly hole
		if sessHasError(s) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue join past other IsTBDOnly residual false
	if sessHasError(s) {
		return false
	}
	if f.IsTBDOnlySess(s) {
		// residual ERROR sticky — no invent clear-TBD past IsTBDOnly hole
		if sessHasError(s) {
			return false
		}
		f.PointTo = nil
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-continue join past self IsTBDOnly residual false
		return false
	}
	ok := f.JoinSess(s, other)
	// residual ERROR sticky — no invent join-change soft-skip past Join hole
	if sessHasError(s) {
		return false
	}
	return ok
}

// JoinVisitsInto merges newFacts into facts with join_visits semantics.
// Used when combining results of multiple visits to the same function.
// Incomplete maps fail closed sticky: *facts = IncompleteFactSlice(), false
// (no invent no-change success via FactsComplete(nil) or soft re-pick past wipe).
// facts always live; sticky (no invent soft-skip join-visits past hole).}

func JoinVisitsInto(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	return JoinVisitsIntoSess(nil, facts, newFacts)
}

func JoinVisitsIntoSess(s *Session, facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	changed := false
	for _, nf := range newFacts {
		cur := FindRelatedPointTo(*facts, nf.Var)
		// residual ERROR sticky — no invent soft-continue later merges past FindRelated hole
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		if cur == nil {
			cl := nf.Clone()
			// residual ERROR sticky — no invent soft-append past Clone residual
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false
			}
			if cl == nil {
				*facts = IncompleteFactSlice()
				sessNoteError(s, ErrGeneric)
				return false
			}
			*facts = append(*facts, cl)
			changed = true
			continue
		}
		// join into clone then replace
		cp := cur.Clone()
		// residual ERROR sticky — no invent soft-join past Clone residual
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		if cp == nil {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
			return false
		}
		if cp.JoinVisits(nf) {
			// residual ERROR sticky — no invent soft-replace past JoinVisits residual
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false
			}
			// replace in slice
			for i, f := range *facts {
				if f != nil && f.Var == nf.Var {
					(*facts)[i] = cp
					break
				}
			}
			changed = true
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue no-change past JoinVisits residual false
			*facts = IncompleteFactSlice()
			return false
		}
	}
	return changed
}

// Clone shallow-copies the fact (new PointTo slice).
// Fact always live; sticky nil (no invent soft-skip clone past hole).
// Incomplete PointTo (nil hole) fails closed sticky nil — no invent clone of broken set
// / soft re-pick past holes. Empty top (nil PointTo) clones as empty non-nil set.}

func (f *FactPointTo) Clone() *FactPointTo {
	return f.CloneSess(nil)
}

func (f *FactPointTo) CloneSess(s *Session) *FactPointTo {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	set := f.PointTo
	if set == nil {
		set = []*Variable{}
	}
	// MakeFactPointToSet sticky on nil pointee holes / nil Var
	return MakeFactPointToSetSess(s, f.Var, set)
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
// Fact* always live at call sites; nil f sticky IncompleteFactSlice
// (no invent no-op / empty-complete via FactsComplete(nil)).
// Incomplete map is non-sticky IncompleteFactSlice (soft re-pick; MergeFacts sticks).
// Clone fail sticky IncompleteFactSlice (hard incomplete PointTo).
func MergeFactInto(facts []*FactPointTo, f *FactPointTo) []*FactPointTo {
	return MergeFactIntoSess(nil, facts, f)
}

func MergeFactIntoSess(s *Session, facts []*FactPointTo, f *FactPointTo) []*FactPointTo {
	// Fact* always live; sticky (no invent soft-skip nil fact as empty merge)
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	// incomplete map must not invent join success when match appears before a hole
	// non-sticky marker — soft re-pick factories; MergeFacts sticks at accumulator
	if !FactsComplete(facts) {
		return IncompleteFactSlice()
	}
	for i, old := range facts {
		if old.Var == f.Var {
			if old.Imply(f) {
				// residual ERROR sticky — no invent soft-skip join past Imply hole
				if sessHasError(s) {
					return IncompleteFactSlice()
				}
				// old already covers f
				return facts
			}
			// residual ERROR sticky — no invent soft-continue join past Imply residual false path
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			// join: copy f, join old into it
			cp := f.Clone()
			if cp == nil {
				// incomplete PointTo on f — fail closed sticky
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return IncompleteFactSlice()
			}
			_ = cp.Join(old)
			// residual ERROR sticky — no invent soft-skip Join residual then keep partial merge
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			facts[i] = cp
			return facts
		}
	}
	cl := f.Clone()
	if cl == nil {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	return append(facts, cl)
}

// MergeFacts mirrors merge_facts — merge each of new into facts.
// Fact.cpp:192–200.
// Returns whether any fact changed. Incomplete maps fail closed: *facts set nil,
// returns false (no invent skip partial join / keep broken partial).}

func MergeFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	return MergeFactsSess(nil, facts, newFacts)
}

func MergeFactsSess(s *Session, facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	// Fact merge always has live accumulator; sticky no invent soft-skip join
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		// incomplete maps wiped sticky (no invent soft re-pick past wiped join)
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	changed := false
	for _, f := range newFacts {
		before := FindRelatedPointTo(*facts, f.Var)
		// residual ERROR sticky — no invent soft-continue merge later past FindRelated hole
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		merged := MergeFactIntoSess(s, *facts, f)
		// MergeFactInto incomplete / residual = hole marker
		if sessHasError(s) || !FactsComplete(merged) {
			*facts = IncompleteFactSlice()
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return false
		}
		*facts = merged
		after := FindRelatedPointTo(*facts, f.Var)
		// residual ERROR sticky — no invent soft-continue Equal/changed past FindRelated hole
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		if before == nil || after == nil {
			changed = true
			continue
		}
		eq := before.Equal(after)
		// residual ERROR sticky — no invent soft-continue changed/not-changed past Equal hole
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		if !eq {
			changed = true
		}
	}
	return changed
}

// CloneFactSlice deep-clones a FactPointTo slice.
// Complete empty: nil in → nil out; non-nil {} → non-nil {}.
// Incomplete maps fail closed sticky IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-complete clone / soft re-pick past hole).}

func CloneFactSlice(facts []*FactPointTo) []*FactPointTo {
	return CloneFactSliceSess(nil, facts)
}

func CloneFactSliceSess(s *Session, facts []*FactPointTo) []*FactPointTo {
	if facts == nil {
		return nil
	}
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
		// Fact* always live after FactsComplete; Clone nil = incomplete PointTo sticky
		cl := f.CloneSess(s)
		// residual ERROR sticky — no invent soft-clone past Clone residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		if cl == nil {
			sessNoteError(s, ErrGeneric)
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
	return f.MarkDeadVarSess(nil, v)
}

func (f *FactPointTo) MarkDeadVarSess(s *Session, v *Variable) *FactPointTo {
	// Fact + subject always live; sticky no invent leave pointees live past hole
	if f == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if !v.FieldVarsComplete() {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	set := append([]*Variable(nil), f.PointTo...)
	pos := -1
	for i, p := range set {
		// PointTo hole sticky — no invent skip dead-mark past incomplete set
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if p == v {
			pos = i
			break
		}
		if v.HasFieldVar(p) {
			// residual ERROR sticky — no invent dead-pos true past HasFieldVar hole
			if sessHasError(s) {
				return nil
			}
			pos = i
			break
		}
		// residual ERROR sticky — no invent soft-continue later pointees past HasField residual
		if sessHasError(s) {
			return nil
		}
		if p.FieldVarOf != nil && isAncestorField(p, v) {
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

// isAncestorField reports whether field is under root via FieldVarOf chain.}

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
	return f.MarkFuncEndLocalsSess(nil, locals)
}

func (f *FactPointTo) MarkFuncEndLocalsSess(s *Session, locals []*Variable) *FactPointTo {
	// Fact always live; sticky no invent leave stack pointees live without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// empty locals: soft no-op (nothing to mark dead)
	if len(locals) == 0 {
		return nil
	}
	localSet := make(map[*Variable]bool, len(locals))
	for _, l := range locals {
		// incomplete locals fails closed sticky (no invent "no change" past hole)
		if l == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		localSet[l] = true
		exp := l.CollectExpandable()
		// residual ERROR sticky — no invent soft-mark past CollectExpandable residual
		if sessHasError(s) {
			return nil
		}
		if !VariablesComplete(exp) {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		for _, fv := range exp {
			localSet[fv] = true
		}
	}
	set := append([]*Variable(nil), f.PointTo...)
	hasGarbage := false
	for _, p := range set {
		// incomplete PointTo fails closed sticky (no invent soft-skip as no change)
		if p == nil {
			sessNoteError(s, ErrGeneric)
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
// stack pointees live because IsVarOnStack returned false past a hole).}

func (f *FactPointTo) MarkFuncEnd(fn *Function, stParent *Block) *FactPointTo {
	return f.MarkFuncEndSess(nil, fn, stParent)
}

func (f *FactPointTo) MarkFuncEndSess(s *Session, fn *Function, stParent *Block) *FactPointTo {
	// Fact always live; sticky no invent leave stack pointees live past hole
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// no function scope: complete no-op (nothing to mark dead) — non-sticky
	// sticky residual here soft invents incomplete RemoveFunctionLocal past nil Func
	// while GlobalFacts stay complete and visit invents success past residual ERROR
	if fn == nil {
		return nil
	}
	// incomplete stack lists fail closed sticky (no invent leave stack pointees live)
	if !fn.StackScanComplete(stParent) {
		// residual ERROR sticky — no invent soft-mark dead past StackScan residual
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return nil
	}
	set := append([]*Variable(nil), f.PointTo...)
	hasGarbage := false
	for _, p := range set {
		if p == nil {
			sessNoteError(s, ErrGeneric)
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
		onStack := fn.IsVarOnStackSess(s, v, stParent)
		// residual ERROR sticky — no invent soft-skip stack scan past hard IR hole
		// (IsVarOnStack may sticky residual false then leave later stack pointees live)
		if sessHasError(s) {
			return nil
		}
		if !onStack {
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
// facts slice always live; sticky (no invent soft-skip mark past hole).}

func MarkFuncEndOnFacts(facts *[]*FactPointTo, fn *Function, stParent *Block) {
	MarkFuncEndOnFactsSess(nil, facts, fn, stParent)
}

func MarkFuncEndOnFactsSess(s *Session, facts *[]*FactPointTo, fn *Function, stParent *Block) {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// incomplete maps/stack fail closed sticky (no invent keep facts past holes)
	if !FactsComplete(*facts) {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	if fn != nil && !fn.StackScanComplete(stParent) {
		*facts = IncompleteFactSlice()
		// residual ERROR sticky — no invent soft-mark past StackScan residual
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return
	}
	for i, f := range *facts {
		// nil without error = no lattice change; sticky = incomplete fail closed
		if nf := f.MarkFuncEnd(fn, stParent); nf != nil {
			(*facts)[i] = nf
		} else if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return
		}
	}
}

// indexExprUsesVar reports whether a string index expression refers to indexVar.
// Indices are stored as strings (e.g. "i", "(i + 2)"); approximate Expression::use_var.
// Variable always live; sticky false (no invent not-used soft-skip past hole).
// Empty idx is complete not-used.}

func indexExprUsesVar(idx string, indexVar *Variable) bool {
	return indexExprUsesVarSess(nil, idx, indexVar)
}

func indexExprUsesVarSess(s *Session, idx string, indexVar *Variable) bool {
	if indexVar == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if idx == "" {
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
// Fact + indexVar always live; sticky nil (no invent identity soft-skip past hole).
func (f *FactPointTo) UpdateWithModifiedIndex(indexVar *Variable) *FactPointTo {
	return f.UpdateWithModifiedIndexSess(nil, indexVar)
}

func (f *FactPointTo) UpdateWithModifiedIndexSess(s *Session, indexVar *Variable) *FactPointTo {
	if f == nil || indexVar == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	pointees := append([]*Variable(nil), f.PointTo...)
	changed := false
	for j, v := range f.PointTo {
		// Variable* always live in PointTo; nil hole sticky fail closed (no invent skip)
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		// walk to root field_var_of (FactPointTo.cpp:718–720)
		root := v
		for root.FieldVarOf != nil {
			root = root.FieldVarOf
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent soft-skip shell as non-itemized then complete identity success)
		if root.IsArray && root.AsArray == nil {
			sessNoteError(s, ErrGeneric)
			return nil
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
					sessNoteError(s, ErrGeneric)
					return nil
				}
				if exp.UseVarSess(s, indexVar) {
					modified = append(modified, k)
				}
			}
		} else {
			for k, exp := range av.Indices {
				if indexExprUsesVarSess(s, exp, indexVar) {
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
			newAV.SetIndexSess(s, k, "-1")
		}
		// FactPointTo.cpp:740 — pointees[j] = new_av (array root, even if field walked)
		pointees[j] = &newAV.Variable
		changed = true
	}
	if !changed {
		return f
	}
	return MakeFactPointToSetSess(s, f.Var, pointees)
}

// UpdateFactsWithModifiedIndex mirrors FactPointTo::update_facts_with_modified_index.
// FactPointTo.cpp:751–761 — rewrite each point-to fact when indexVar is modified.
// Fact* always live; nil hole or failed rewrite fails closed sticky (facts incomplete).
// facts + indexVar always live; sticky (no invent soft-skip update past hole).
func UpdateFactsWithModifiedIndex(facts *[]*FactPointTo, indexVar *Variable) {
	UpdateFactsWithModifiedIndexSess(nil, facts, indexVar)
}

func UpdateFactsWithModifiedIndexSess(s *Session, facts *[]*FactPointTo, indexVar *Variable) {
	if facts == nil || indexVar == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !FactsComplete(*facts) {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	for i, fp := range *facts {
		newFP := fp.UpdateWithModifiedIndexSess(s, indexVar)
		// UpdateWithModifiedIndex nil = incomplete pointees
		if newFP == nil {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
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
// IncompleteVariables (not bare nil — VariablesComplete(nil)/len==0 invent empty skip).
// Complete empty (specials-only / no pointees) returns non-nil empty slice.
func MergePointeesOfPointers(ptrs []*Variable, facts []*FactPointTo) []*Variable {
	return MergePointeesOfPointersSess(nil, ptrs, facts)
}

func MergePointeesOfPointersSess(s *Session, ptrs []*Variable, facts []*FactPointTo) []*Variable {
	// incomplete fact map fails closed non-sticky (fact-map soft re-pick factories)
	if !FactsComplete(facts) {
		return IncompleteVariables()
	}
	// Variable* always live; sticky IncompleteVariables (no invent soft-skip ptr hole)
	if !VariablesComplete(ptrs) {
		sessNoteError(s, ErrGeneric)
		return IncompleteVariables()
	}
	out := make([]*Variable, 0)
	seen := make(map[*Variable]bool)
	for _, p := range ptrs {
		if IsSpecialPtr(p) {
			continue
		}
		ft := FindRelatedPointTo(facts, p)
		// residual ERROR sticky — no invent soft-merge pointees past FindRelated residual
		if sessHasError(s) {
			return IncompleteVariables()
		}
		// FactPointTo.cpp:691–696 — assert(exist_fact); if (exist_fact) merge.
		// NDEBUG elides assert: missing fact skips that pointer (param mid-create,
		// or OOS pointee still listed after UpdateFactsForOOSVars). Do not treat as
		// IncompleteVariables — that invents visit_facts reject when release csmith
		// continues with remaining pointees (seed-2 e2308: g_134→g_80 no fact).
		if ft == nil {
			continue
		}
		for _, pointee := range ft.PointTo {
			// PointTo Variable* always live; sticky (no invent soft-skip pointee hole)
			if pointee == nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteVariables()
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
// Incomplete merge → IncompleteVariables (not bare nil invent empty complete).
// Variable always live; sticky IncompleteVariables (no invent soft-skip past hole).
// Missing fact / incomplete map stays non-sticky IncompleteVariables (soft re-pick).}

func MergePointeesOfPointer(ptr *Variable, indirect int, facts []*FactPointTo) []*Variable {
	return MergePointeesOfPointerSess(nil, ptr, indirect, facts)
}

func MergePointeesOfPointerSess(s *Session, ptr *Variable, indirect int, facts []*FactPointTo) []*Variable {
	if ptr == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteVariables()
	}
	tmp := []*Variable{ptr}
	for indirect > 0 {
		tmp = MergePointeesOfPointersSess(s, tmp, facts)
		// incomplete merge (missing fact / holes) — stop, do not invent empty
		if !VariablesComplete(tmp) {
			return IncompleteVariables()
		}
		indirect--
	}
	return tmp
}

// IsPointingToLocals mirrors FactPointTo::is_pointing_to_locals.
// FactPointTo.cpp:487–526.
// Variable always live; sticky true (no invent not-local soft-skip past hole).
// Incomplete fact maps / stack scans / nil holes fail closed sticky true
// (no invent "not pointing to locals" / soft re-pick past holes).
// Type* always live for non-special subjects/pointees; Type-nil sticky true
// (IsPointer residual ERROR+false invents not-pointing-to-locals past shell).
// MergePointees incomplete stays non-sticky true (fact-map soft re-pick).}

func IsPointingToLocals(v *Variable, b *Block, indirection int, facts []*FactPointTo) bool {
	return IsPointingToLocalsSess(nil, v, b, indirection, facts)
}

func IsPointingToLocalsSess(s *Session, v *Variable, b *Block, indirection int, facts []*FactPointTo) bool {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// incomplete LocalVars/Param sticky (membership short-circuit invents not-local)
	if b != nil && !b.StackScanComplete() {
		// residual ERROR sticky — no invent soft not-local past StackScan residual
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return true
	}
	if indirection == -1 {
		ok := v.IsVisibleLocalSess(s, b)
		// residual ERROR sticky — no invent not-local soft-skip past IsVisibleLocal hole
		if sessHasError(s) {
			return true
		}
		return ok
	}
	// Type* always live for non-special subjects; Type-nil sticky true
	// (no invent IsPointer residual false as not-pointing-to-locals past shell)
	if !IsSpecialPtr(v) && v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !v.IsPointer() {
		// residual ERROR sticky — no invent not-pointer soft-skip past IsPointer hole
		if sessHasError(s) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue pointees past IsPointer residual true path
	if sessHasError(s) {
		return true
	}
	// FactPointTo.cpp:506–508 — array / array-field subjects use collective for fact lookup
	// (itemized members share the collective's points-to; without this, FindRelatedPointTo
	// misses and as_return/no_return_dead_ptr wrongly allows local-pointing array elems).
	isAF := v.IsArrayFieldSess(s)
	if sessHasError(s) {
		return true
	}
	if v.IsArray || isAF {
		coll := varCollectiveSess(s, v)
		if sessHasError(s) {
			return true
		}
		if coll != nil {
			v = coll
		}
	}
	// incomplete fact maps sticky true
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return true
	}
	var pointees []*Variable
	if indirection == 0 {
		ft := FindRelatedPointTo(facts, v)
		// residual ERROR sticky — no invent not-pointing soft-skip past FindRelated hole
		if sessHasError(s) {
			return true
		}
		if ft == nil {
			return false
		}
		pointees = ft.PointTo
	} else {
		pointees = MergePointeesOfPointer(v, indirection, facts)
		// incomplete merge non-sticky true (soft re-pick)
		if !VariablesComplete(pointees) {
			return true
		}
		// residual ERROR sticky — no invent soft-continue past MergePointees residual
		if sessHasError(s) {
			return true
		}
	}
	for _, p := range pointees {
		if p == nil {
			// PointTo nil hole sticky
			sessNoteError(s, ErrGeneric)
			return true
		}
		if IsSpecialPtr(p) {
			continue
		}
		// Type-nil non-special pointee sticky true before IsPointer residual false skip
		if p.Type == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if p.IsVisibleLocalSess(s, b) {
			// residual ERROR sticky — no invent pointing-true past IsVisibleLocal hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue not-local past IsVisibleLocal residual
		if sessHasError(s) {
			return true
		}
		// recurse one level of pointees that are pointers
		if p.IsPointer() {
			// residual ERROR sticky — no invent soft-skip recurse past IsPointer residual
			if sessHasError(s) {
				return true
			}
			il := p.Type.IndirectLevelSess(s)
			// residual ERROR sticky — no invent soft-recurse past IndirectLevel residual
			if sessHasError(s) {
				return true
			}
			for j := 0; j < il; j++ {
				nested := MergePointeesOfPointer(p, j+1, facts)
				// residual ERROR sticky — no invent soft-recurse past MergePointees residual
				if sessHasError(s) {
					return true
				}
				if !VariablesComplete(nested) {
					// incomplete MergePointees non-sticky
					return true
				}
				for _, n := range nested {
					if n == nil {
						sessNoteError(s, ErrGeneric)
						return true
					}
					if IsSpecialPtr(n) {
						continue
					}
					if n.IsVisibleLocalSess(s, b) {
						// residual ERROR sticky — no invent pointing-true past nested IsVisibleLocal hole
						if sessHasError(s) {
							return true
						}
						return true
					}
					// residual ERROR sticky — no invent soft-continue nested not-local past residual
					if sessHasError(s) {
						return true
					}
				}
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-skip non-pointer past IsPointer residual false path
			return true
		}
	}
	return false
}

// Session AllPtrs / AllAliases mirror FactPointTo.cpp:67–68 aggregates.

// ClearPointToAggregates resets all_ptrs / all_aliases (generation start).}

func ClearPointToAggregates() {
	ClearPointToAggregatesSess(nil)
}

// ClearPointToAggregatesSess clears pointer aggregates on an explicit session bag.
func ClearPointToAggregatesSess(s *Session) {
	s = sessOrAmbient(s)
	s.AllPtrs = nil
	s.AllAliases = nil
}

// UpdatePtrAliases mirrors FactPointTo::update_ptr_aliases.
// FactPointTo.cpp:764–790 — merge point-to sets into parallel ptr/alias vectors.
// UpdatePtrAliases merges facts into parallel ptrs/aliases tables.
// Fact* always live; nil hole fails closed sticky (false — no invent skip partial
// alias / soft re-pick past holes). Returns false on incomplete maps; true when done.
func UpdatePtrAliases(facts []*FactPointTo, ptrs *[]*Variable, aliases *[][]*Variable) bool {
	return UpdatePtrAliasesSess(nil, facts, ptrs, aliases)
}

func UpdatePtrAliasesSess(s *Session, facts []*FactPointTo, ptrs *[]*Variable, aliases *[][]*Variable) bool {
	if ptrs == nil || aliases == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, f := range facts {
		// Fact* always live; nil hole sticky
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		if f.Var == nil {
			sessNoteError(s, ErrGeneric)
			return false
		}
		// FactPointTo.cpp: type != 0 — specials may have Type-nil; other Type-nil sticky
		if f.Var.Type == nil {
			if IsSpecialPtr(f.Var) {
				continue
			}
			sessNoteError(s, ErrGeneric)
			return false
		}
		// PointTo Variable* always live; nil hole sticky
		for _, v := range f.PointTo {
			if v == nil {
				sessNoteError(s, ErrGeneric)
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

// FunctionsComplete reports every Function* is live (no nil holes).
// Note: FunctionsComplete(nil)==true (complete empty). Fail-closed incomplete
// Funcs wipes / walks must not invent skip hole as absent function.}

func FunctionsComplete(funcs []*Function) bool {
	for _, f := range funcs {
		if f == nil {
			return false
		}
	}
	return true
}

// IncompleteFunctions is the fail-closed incomplete Function* list marker.
func IncompleteFunctions() []*Function {
	return []*Function{nil}
}

// AggregateAllPointToSets mirrors FactPointTo::aggregate_all_pointto_sets.
// FactPointTo.cpp:792–804 — scan each non-builtin func FactMgr map_facts_out.
// FactPointTo.cpp:803 — assert(all_ptrs.size() == all_aliases.size()); kept by UpdatePtrAliases.
// Incomplete fact maps / Funcs list fail closed (clear aggregates — no invent partial AllPtrs).
func AggregateAllPointToSets(funcs []*Function, fms *FactMgrMap) {
	AggregateAllPointToSetsSess(nil, funcs, fms)
}

// AggregateAllPointToSetsSess is AggregateAllPointToSets on an explicit session bag.
func AggregateAllPointToSetsSess(s *Session, funcs []*Function, fms *FactMgrMap) {
	s = sessOrAmbient(s)
	ClearPointToAggregatesSess(s)
	// incomplete Funcs list fails closed sticky (no invent partial aggregate success)
	if !FunctionsComplete(funcs) {
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		// FactMgr always paired for user funcs after make_random_signature / make_first
		// no invent skip missing FM (partial aggregate)
		if fms == nil {
			ClearPointToAggregatesSess(s)
			sessNoteError(s, ErrGeneric)
			return
		}
		fm := fms.ForFuncSess(s, f)
		if fm == nil {
			ClearPointToAggregatesSess(s)
			sessNoteError(s, ErrGeneric)
			return
		}
		// prefer map_facts_out values; also include GlobalFacts
		for _, facts := range fm.MapFactsOut {
			if !UpdatePtrAliasesSess(s, facts, &s.AllPtrs, &s.AllAliases) {
				ClearPointToAggregatesSess(s)
				// UpdatePtrAliases already sticky; keep cleared (no invent partial AllPtrs)
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return
			}
		}
		if !UpdatePtrAliasesSess(s, fm.GlobalFacts, &s.AllPtrs, &s.AllAliases) {
			ClearPointToAggregatesSess(s)
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return
		}
	}
	// FactPointTo.cpp:803 — sizes must stay paired sticky (no invent desync success)
	if len(s.AllPtrs) != len(s.AllAliases) {
		ClearPointToAggregatesSess(s)
		sessNoteError(s, ErrGeneric)
	}
}

// CopyFacts mirrors copy_facts — deep clone FactPointTo slice.
// Fact.cpp:213–220. Alias for CloneFactSlice.
func CopyFacts(facts []*FactPointTo) []*FactPointTo {
	return CloneFactSlice(facts)
}

// CopyFactsSess is CopyFacts on an explicit session bag.
func CopyFactsSess(s *Session, facts []*FactPointTo) []*FactPointTo {
	return CloneFactSliceSess(s, facts)
}

// CombineFacts mirrors combine_facts — join_visits across revisits.
// Fact.cpp:225–235. Alias for JoinVisitsInto.
func CombineFacts(facts *[]*FactPointTo, facts2 []*FactPointTo) {
	_ = JoinVisitsInto(facts, facts2)
}

// PrintFacts mirrors print_facts — concatenate OutputAssertion lines.
// Fact.cpp:263–268.
func PrintFacts(facts []*FactPointTo, stParent *Block) string {
	return PrintFactsSess(nil, facts, stParent)
}

func PrintFactsSess(s *Session, facts []*FactPointTo, stParent *Block) string {
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, f := range facts {
		b.WriteString(f.OutputAssertionSess(s, stParent, ""))
	}
	return b.String()
}

// PrintVarFact mirrors print_var_fact — assertions for one variable name.
// Fact.cpp:270–277.
func PrintVarFact(facts []*FactPointTo, vname string, stParent *Block) string {
	return PrintVarFactSess(nil, facts, vname, stParent)
}

func PrintVarFactSess(s *Session, facts []*FactPointTo, vname string, stParent *Block) string {
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, f := range facts {
		if f.Var == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if f.Var.Name == vname {
			b.WriteString(f.OutputAssertionSess(s, stParent, ""))
		}
	}
	return b.String()
}

// AbstractFactForReturn mirrors Fact::abstract_fact_for_return.
// Fact.cpp:76–83 — abstract_fact_for_assign(facts, Lhs(func.rv), expr).
func AbstractFactForReturn(facts []*FactPointTo, expr *Expression, fn *Function) []*FactPointTo {
	return AbstractFactForReturnSess(nil, facts, expr, fn)
}

// AbstractFactForReturnSess is AbstractFactForReturn with sticky on run bag.
func AbstractFactForReturnSess(s *Session, facts []*FactPointTo, expr *Expression, fn *Function) []*FactPointTo {
	if fn == nil || fn.RV == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	// Expression* always live on return; sticky fail closed
	if expr == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	out, _ := AbstractFactForAssignSess(s, facts, fn.RV, 0, expr)
	return out
}

// FactDoFinalization mirrors Fact::doFinalization.
// Fact.cpp:110–115 — clear Fact::facts_ registry; Go uses ClearPointToAggregates.
func FactDoFinalization() {
	FactDoFinalizationSess(nil)
}

// FactDoFinalizationSess clears Fact aggregates on an explicit session bag.
func FactDoFinalizationSess(s *Session) {
	ClearPointToAggregatesSess(s)
}
