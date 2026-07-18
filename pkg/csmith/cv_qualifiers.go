// Upstream: CVQualifiers.h / CVQualifiers.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CVQualifiers mirrors CVQualifiers — per-indirection const/volatile vectors.
// Indexing: is_consts[0] is outermost pointer level when depth>1; last element is
// the storage object (see random_qualifiers vector construction in CVQualifiers.cpp).
type CVQualifiers struct {
	Wildcard       bool
	AcceptStricter bool
	IsConsts       []bool
	IsVolatiles    []bool
}

// NewCVQualifiers mirrors CVQualifiers(const vector<bool>&, const vector<bool>&).
func NewCVQualifiers(consts, vols []bool) CVQualifiers {
	return CVQualifiers{
		IsConsts:    append([]bool(nil), consts...),
		IsVolatiles: append([]bool(nil), vols...),
	}
}

// IsConst mirrors CVQualifiers::is_const → is_const_after_deref(0).
func (q CVQualifiers) IsConst() bool {
	return q.IsConstAfterDeref(0)
}

// IsVolatile mirrors CVQualifiers::is_volatile → is_volatile_after_deref(0).
func (q CVQualifiers) IsVolatile() bool {
	return q.IsVolatileAfterDeref(0)
}

// IsConstAfterDeref mirrors CVQualifiers::is_const_after_deref.
func (q CVQualifiers) IsConstAfterDeref(derefLevel int) bool {
	if derefLevel < 0 {
		return false
	}
	lenC := len(q.IsConsts)
	if lenC <= derefLevel {
		return false
	}
	return q.IsConsts[lenC-derefLevel-1]
}

// IsVolatileAfterDeref mirrors CVQualifiers::is_volatile_after_deref.
func (q CVQualifiers) IsVolatileAfterDeref(derefLevel int) bool {
	if derefLevel < 0 {
		return false
	}
	lenV := len(q.IsVolatiles)
	if lenV <= derefLevel {
		return false
	}
	return q.IsVolatiles[lenV-derefLevel-1]
}

// MakeScalarVolatiles mirrors CVQualifiers::make_scalar_volatiles.
// If !volatile_pointers || !global_variables, clear volatiles[i] for i>=1.
func MakeScalarVolatiles(opts Options, volatiles []bool) {
	if opts.VolatilePointers && opts.GlobalVariables {
		return
	}
	for i := 1; i < len(volatiles); i++ {
		volatiles[i] = false
	}
}

// MakeScalarConsts mirrors CVQualifiers::make_scalar_consts.
// If !const_pointers, clear consts[i] for i>=1.
func MakeScalarConsts(opts Options, consts []bool) {
	if opts.ConstPointers {
		return
	}
	for i := 1; i < len(consts); i++ {
		consts[i] = false
	}
}

// boolsEqual compares two bool slices.
func boolsEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// StricterThan mirrors CVQualifiers::stricter_than (const/vol depth match).
// CVQualifiers.cpp:95–120 subset — const: no looser const; multi-level ** special.
func (q CVQualifiers) StricterThan(other CVQualifiers) bool {
	if len(q.IsConsts) != len(other.IsConsts) || len(q.IsVolatiles) != len(other.IsVolatiles) {
		return false
	}
	depth := len(q.IsConsts)
	for i := 0; i < depth; i++ {
		// levels followed by two * must match const exactly
		if depth-i > 2 && q.IsConsts[i] != other.IsConsts[i] {
			return false
		}
		// other has const where we don't → we are not stricter
		if other.IsConsts[i] && !q.IsConsts[i] {
			return false
		}
	}
	for i := 0; i < depth; i++ {
		if other.IsVolatiles[i] && !q.IsVolatiles[i] {
			return false
		}
	}
	return true
}

// Match mirrors CVQualifiers::match.
// CVQualifiers.cpp:137–152.
func (q CVQualifiers) Match(other CVQualifiers, matchExact bool) bool {
	if q.Wildcard {
		return true
	}
	if matchExact {
		return boolsEqual(q.IsConsts, other.IsConsts) && boolsEqual(q.IsVolatiles, other.IsVolatiles)
	}
	// both non-pointer (one level) → true
	if len(q.IsConsts) == len(other.IsConsts) && len(q.IsConsts) == 1 {
		return true
	}
	if !q.AcceptStricter {
		return q.StricterThan(other)
	}
	return other.StricterThan(q)
}

// IndirectQualifiers mirrors CVQualifiers::indirect_qualifiers.
// CVQualifiers.cpp:504–521 — level<0 address; level>0 strip deref levels.
func (q CVQualifiers) IndirectQualifiers(level int) CVQualifiers {
	if level == 0 || q.Wildcard {
		return q
	}
	if level < 0 {
		// address-of: add one false,false level
		out := NewCVQualifiers(
			append(append([]bool(nil), q.IsConsts...), false),
			append(append([]bool(nil), q.IsVolatiles...), false),
		)
		out.Wildcard = q.Wildcard
		out.AcceptStricter = q.AcceptStricter
		return out
	}
	// dereference: remove `level` outermost entries from front? Upstream remove_qualifiers
	// removes from the start of the vector (outer pointers)
	if level >= len(q.IsConsts) {
		return NewCVQualifiers(nil, nil)
	}
	out := NewCVQualifiers(q.IsConsts[level:], q.IsVolatiles[level:])
	out.AcceptStricter = q.AcceptStricter
	return out
}

// MatchIndirect mirrors CVQualifiers::match_indirect.
// CVQualifiers.cpp:155–166.
func (q CVQualifiers) MatchIndirect(other CVQualifiers, matchExact bool) bool {
	if q.Wildcard {
		return true
	}
	if len(q.IsConsts) == len(other.IsConsts) {
		return q.Match(other, matchExact)
	}
	deref := len(other.IsConsts) - len(q.IsConsts)
	if deref < -1 {
		return false
	}
	return q.Match(other.IndirectQualifiers(deref), matchExact)
}

// isVolatileOKOnOneLevel mirrors is_volatile_ok_on_one_level (CVQualifiers.cpp).
// For non-C++ and non-struct/union types, always true. Struct/union path deferred.
func isVolatileOKOnOneLevel(opts Options, t *Type) bool {
	if opts.LangCPP {
		// Struct/union assign-ops rules not ported; simple types OK.
		if t != nil && t.IsSimple() {
			return true
		}
		// Until struct/union Types exist, treat non-simple as OK for C mode only.
		return true
	}
	return true
}

// RandomQualifiersForType mirrors CVQualifiers::random_qualifiers(
//
//	t, access, cg_context, no_volatile, const_prob, volatile_prob).
//
// CVQualifiers.cpp:295–353.
func RandomQualifiersForType(
	t *Type,
	access Access,
	cg CGContext,
	noVolatile bool,
	constProb, volatileProb uint32,
	opts Options,
	r *Rng,
) CVQualifiers {
	if t == nil {
		return CVQualifiers{}
	}
	if r == nil {
		r = NewRng(0)
	}

	var isConsts, isVolatiles []bool
	effectCtx := cg.EffectContext()

	// CVQualifiers.cpp:306–330 — pointer levels (t->ptr_type chain).
	level := 0
	tmp := t.PtrType()
	for tmp != nil {
		level++
		isConsts = append(isConsts, false)
		isVolatiles = append(isVolatiles, false)
		tmp = tmp.PtrType()
	}
	tmp = t.PtrType()
	for tmp != nil {
		volatileOK := isVolatileOKOnOneLevel(opts, tmp)
		isVolatile := false
		if volatileOK {
			// rnd_flipcoin(volatile_prob)
			isVolatile = r.RndFlipcoin(volatileProb)
		}
		isConst := r.RndFlipcoin(constProb)
		if isVolatile && isConst && !opts.AllowConstVolatile {
			isConst = false
		}
		if level < 1 {
			break
		}
		isConsts[level-1] = isConst
		isVolatiles[level-1] = isVolatile
		level--
		tmp = tmp.PtrType()
	}

	// CVQualifiers.cpp:332–343 — variable itself.
	volatileOK := effectCtx.IsSideEffectFree() && isVolatileOKOnOneLevel(opts, t)
	constOK := access != AccessWrite
	isVolatile := false
	if volatileOK {
		isVolatile = r.RndFlipcoin(volatileProb)
	}
	isConst := false
	if constOK {
		isConst = r.RndFlipcoin(constProb)
	}
	if isVolatile && isConst && !opts.AllowConstVolatile {
		isConst = false
	}
	isConsts = append(isConsts, isConst)
	isVolatiles = append(isVolatiles, isVolatile)

	if noVolatile {
		for i := range isVolatiles {
			isVolatiles[i] = false
		}
	}
	MakeScalarVolatiles(opts, isVolatiles)
	MakeScalarConsts(opts, isConsts)
	return NewCVQualifiers(isConsts, isVolatiles)
}

// RandomQualifiersDefaultProbs mirrors
// CVQualifiers::random_qualifiers(t, access, cg, no_volatile)
// using RegularConstProb / RegularVolatileProb from opts-derived Probabilities.
func RandomQualifiersDefaultProbs(
	t *Type,
	access Access,
	cg CGContext,
	noVolatile bool,
	opts Options,
	probs *Probabilities,
	r *Rng,
) CVQualifiers {
	if probs == nil {
		probs = NewProbabilities(opts)
	}
	// RegularConstProb() / RegularVolatileProb() → single probs
	return RandomQualifiersForType(
		t, access, cg, noVolatile,
		uint32(probs.Single(PRegularConstProb)),
		uint32(probs.Single(PRegularVolatileProb)),
		opts, r,
	)
}

// RandomQualifiersNoContextNoVolatile mirrors CVQualifiers::random_qualifiers(const Type *t)
// → READ, empty context, no_volatile=true.
func RandomQualifiersNoContextNoVolatile(t *Type, opts Options, probs *Probabilities, r *Rng) CVQualifiers {
	return RandomQualifiersDefaultProbs(t, AccessRead, EmptyCGContext(), true, opts, probs, r)
}
