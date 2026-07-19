// Upstream: CVQualifiers.h / CVQualifiers.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

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
// CVQualifiers.cpp:96 — assert(is_consts.size() == is_volatiles.size()) when both non-empty.
func NewCVQualifiers(consts, vols []bool) CVQualifiers {
	// fail closed: truncate to min length so depths stay paired (no invent pad false)
	if len(consts) != len(vols) {
		n := len(consts)
		if len(vols) < n {
			n = len(vols)
		}
		if n < 0 {
			n = 0
		}
		consts = consts[:n]
		vols = vols[:n]
	}
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
// CVQualifiers.cpp:564–570 — assert(len > deref_level); OOB is broken IR.
func (q CVQualifiers) IsConstAfterDeref(derefLevel int) bool {
	if derefLevel < 0 {
		return false
	}
	lenC := len(q.IsConsts)
	// zero-value qfer (no storage slot): not const; C++ live vars always have ≥1 level
	if lenC == 0 {
		return false
	}
	// assert(len > deref_level) fail-closed as const (no soft invent non-const WRITE)
	if lenC <= derefLevel {
		return true
	}
	return q.IsConsts[lenC-derefLevel-1]
}

// IsVolatileAfterDeref mirrors CVQualifiers::is_volatile_after_deref.
// CVQualifiers.cpp:573–585 — assert(len > deref_level); OOB is broken IR.
func (q CVQualifiers) IsVolatileAfterDeref(derefLevel int) bool {
	if derefLevel < 0 {
		return false
	}
	lenV := len(q.IsVolatiles)
	if lenV == 0 {
		return false
	}
	// assert fail-closed as volatile (no soft invent non-vol access)
	if lenV <= derefLevel {
		return true
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
	// CVQualifiers.cpp:96 — assert own vectors same size
	if len(q.IsConsts) != len(q.IsVolatiles) || len(other.IsConsts) != len(other.IsVolatiles) {
		return false
	}
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
// CVQualifiers.cpp:137–152 — CGOptions::match_exact_qualifiers() is process-wide;
// matchExact ORs with ProcessOptions so ChooseVarFull / select see session force.
// Explicit matchExact=true still works for unit tests without SetProcessOptions.
func (q CVQualifiers) Match(other CVQualifiers, matchExact bool) bool {
	if q.Wildcard {
		return true
	}
	// CVQualifiers.cpp:141–143 — if (CGOptions::match_exact_qualifiers())
	if matchExact || ProcessOptions().MatchExactQualifiers {
		return boolsEqual(q.IsConsts, other.IsConsts) && boolsEqual(q.IsVolatiles, other.IsVolatiles)
	}
	// both non-pointer (one level) → true
	// CVQualifiers.cpp:148 — assert(is_consts.size() == is_volatiles.size())
	if len(q.IsConsts) == len(other.IsConsts) && len(q.IsConsts) == 1 {
		if len(q.IsConsts) != len(q.IsVolatiles) {
			return false
		}
		return true
	}
	if !q.AcceptStricter {
		return q.StricterThan(other)
	}
	return other.StricterThan(q)
}

// AddQualifiers mirrors CVQualifiers::add_qualifiers — push one level.
// CVQualifiers.cpp:460–463.
func (q *CVQualifiers) AddQualifiers(isConst, isVolatile bool) {
	if q == nil {
		return
	}
	q.IsConsts = append(q.IsConsts, isConst)
	q.IsVolatiles = append(q.IsVolatiles, isVolatile)
}

// RemoveQualifiers mirrors CVQualifiers::remove_qualifiers — pop_back len times.
// CVQualifiers.cpp:497–502.
func (q *CVQualifiers) RemoveQualifiers(length int) {
	if q == nil || length <= 0 {
		return
	}
	for i := 0; i < length; i++ {
		if len(q.IsConsts) == 0 {
			break
		}
		q.IsConsts = q.IsConsts[:len(q.IsConsts)-1]
		if len(q.IsVolatiles) > 0 {
			q.IsVolatiles = q.IsVolatiles[:len(q.IsVolatiles)-1]
		}
	}
}

// IndirectQualifiers mirrors CVQualifiers::indirect_qualifiers.
// CVQualifiers.cpp:504–521 — level<0 address (add); level>0 deref (remove_qualifiers).
func (q CVQualifiers) IndirectQualifiers(level int) CVQualifiers {
	if level == 0 || q.Wildcard {
		return q
	}
	if level < 0 {
		// CVQualifiers.cpp:510 — assert(level == -1); multi-level & sticky fail closed as empty
		if level != -1 {
			SetError(ErrGeneric)
			return CVQualifiers{}
		}
		// address-of: add one false,false level (push_back)
		out := q
		out.IsConsts = append(append([]bool(nil), q.IsConsts...), false)
		out.IsVolatiles = append(append([]bool(nil), q.IsVolatiles...), false)
		return out
	}
	// dereference: pop_back `level` times
	out := NewCVQualifiers(
		append([]bool(nil), q.IsConsts...),
		append([]bool(nil), q.IsVolatiles...),
	)
	out.AcceptStricter = q.AcceptStricter
	out.RemoveQualifiers(level)
	return out
}

// SanityCheck mirrors CVQualifiers::sanity_check.
// CVQualifiers.cpp:526–531 — assert(t); assert(level>=0); depth == indirect+1.
func (q CVQualifiers) SanityCheck(t *Type) bool {
	// CVQualifiers.cpp:527 assert(t) sticky
	if t == nil {
		SetError(ErrGeneric)
		return false
	}
	if q.Wildcard {
		return true
	}
	level := t.IndirectLevel()
	// CVQualifiers.cpp:529 assert(level >= 0) sticky
	if level < 0 {
		SetError(ErrGeneric)
		return false
	}
	return len(q.IsConsts) == len(q.IsVolatiles) &&
		len(q.IsConsts) == level+1
}

// RandomStricterConsts mirrors CVQualifiers::random_stricter_consts.
// CVQualifiers.cpp:375–397.
// C++ always has process RNG + Probabilities; nil r fails closed nil (no invent
// identity bits without draw). Nil probs → 0% (no invent default 50).
func (q CVQualifiers) RandomStricterConsts(r *Rng, opts Options, probs *Probabilities) []bool {
	depth := len(q.IsConsts)
	if opts.MatchExactQualifiers {
		return append([]bool(nil), q.IsConsts...)
	}
	// CVQualifiers.cpp always has process RNG sticky; no invent identity without draw
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// nil probs → 0% (no invent NewProbabilities / hard-coded 50)
	p := 0
	if probs != nil {
		p = probs.Single(PStricterConstProb)
	}
	out := make([]bool, 0, depth)
	for i := 0; i < depth; i++ {
		// special case: already const, or more than 2 levels of * after → keep
		if q.IsConsts[i] || (depth-i > 2) {
			out = append(out, q.IsConsts[i])
			continue
		}
		if i < len(q.IsVolatiles) && q.IsVolatiles[i] && !opts.AllowConstVolatile {
			out = append(out, false)
			continue
		}
		// CVQualifiers.cpp:390 — DEPTH_GUARD_BY_DEPTH_RETURN(1, consts)
		if DepthGuardByDepth(opts, 1) == BadDepth {
			for j := i; j < depth; j++ {
				out = append(out, q.IsConsts[j])
			}
			return out
		}
		out = append(out, r.RndFlipcoin(uint32(p)))
	}
	return out
}

// RandomStricterVolatiles mirrors CVQualifiers::random_stricter_volatiles.
// CVQualifiers.cpp:399–420.
// C++ always has process RNG + Probabilities; nil r sticky fail closed nil.
// Nil probs → 0% (no invent default 50).
func (q CVQualifiers) RandomStricterVolatiles(r *Rng, opts Options, probs *Probabilities) []bool {
	depth := len(q.IsVolatiles)
	if opts.MatchExactQualifiers {
		return append([]bool(nil), q.IsVolatiles...)
	}
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	p := 0
	if probs != nil {
		p = probs.Single(PRegularVolatileProb)
	}
	out := make([]bool, 0, depth)
	for i := 0; i < depth; i++ {
		// first storage must match when multi-level; deep levels match
		if q.IsVolatiles[i] || (i == 0 && depth > 1) || (depth-i > 2) {
			out = append(out, q.IsVolatiles[i])
			continue
		}
		if i < len(q.IsConsts) && q.IsConsts[i] && !opts.AllowConstVolatile {
			out = append(out, false)
			continue
		}
		// CVQualifiers.cpp:412 — DEPTH_GUARD_BY_DEPTH_RETURN(1, volatiles)
		if DepthGuardByDepth(opts, 1) == BadDepth {
			for j := i; j < depth; j++ {
				out = append(out, q.IsVolatiles[j])
			}
			MakeScalarVolatiles(opts, out)
			return out
		}
		out = append(out, r.RndFlipcoin(uint32(p)))
	}
	MakeScalarVolatiles(opts, out)
	return out
}

// RandomLooserConsts mirrors CVQualifiers::random_looser_consts.
// CVQualifiers.cpp:422–439.
// C++ always has process RNG + Probabilities; nil r sticky fail closed nil.
// Nil probs → 0% (no invent default 50).
func (q CVQualifiers) RandomLooserConsts(r *Rng, opts Options, probs *Probabilities) []bool {
	depth := len(q.IsConsts)
	if opts.MatchExactQualifiers {
		return append([]bool(nil), q.IsConsts...)
	}
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	p := 0
	if probs != nil {
		p = probs.Single(PLooserConstProb)
	}
	out := make([]bool, 0, depth)
	for i := 0; i < depth; i++ {
		if !q.IsConsts[i] || (depth-i > 2) {
			out = append(out, q.IsConsts[i])
			continue
		}
		// CVQualifiers.cpp:432 — DEPTH_GUARD_BY_DEPTH_RETURN(1, consts)
		if DepthGuardByDepth(opts, 1) == BadDepth {
			for j := i; j < depth; j++ {
				out = append(out, q.IsConsts[j])
			}
			return out
		}
		out = append(out, r.RndFlipcoin(uint32(p)))
	}
	return out
}

// RandomLooserVolatiles mirrors CVQualifiers::random_looser_volatiles.
// CVQualifiers.cpp:441–457.
// C++ always has process RNG + Probabilities; nil r sticky fail closed nil.
// Nil probs → 0% (no invent default 50).
func (q CVQualifiers) RandomLooserVolatiles(r *Rng, opts Options, probs *Probabilities) []bool {
	depth := len(q.IsVolatiles)
	if opts.MatchExactQualifiers {
		return append([]bool(nil), q.IsVolatiles...)
	}
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	p := 0
	if probs != nil {
		p = probs.Single(PRegularVolatileProb)
	}
	out := make([]bool, 0, depth)
	for i := 0; i < depth; i++ {
		if !q.IsVolatiles[i] || (i == 0 && depth > 1) || (depth-i > 2) {
			out = append(out, q.IsVolatiles[i])
			continue
		}
		// CVQualifiers.cpp:450 — DEPTH_GUARD_BY_DEPTH_RETURN(1, volatiles)
		if DepthGuardByDepth(opts, 1) == BadDepth {
			for j := i; j < depth; j++ {
				out = append(out, q.IsVolatiles[j])
			}
			MakeScalarVolatiles(opts, out)
			return out
		}
		out = append(out, r.RndFlipcoin(uint32(p)))
	}
	MakeScalarVolatiles(opts, out)
	return out
}

// RandomQualifiersFrom mirrors CVQualifiers::random_qualifiers(no_vol, access, cg).
// CVQualifiers.cpp:194–225 — random relative to this qfer (stricter or looser).
// ERROR_GUARD paths: sticky error leaves partial qfer; callers check HasError.
func (q CVQualifiers) RandomQualifiersFrom(
	noVolatile bool,
	access Access,
	cg CGContext,
	opts Options,
	probs *Probabilities,
	r *Rng,
) CVQualifiers {
	if q.Wildcard {
		return CVQualifiers{Wildcard: true, AcceptStricter: q.AcceptStricter}
	}
	// CVQualifiers.cpp always has process RNG for random_* paths
	// no invent fixed non-stricter/looser shells when RNG missing
	if r == nil {
		return CVQualifiers{}
	}
	// incomplete ambient fails closed sticky (no invent looser/stricter qfer past holes)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	var vols []bool
	if noVolatile {
		vols = make([]bool, len(q.IsVolatiles))
	} else if !q.AcceptStricter {
		vols = q.RandomLooserVolatiles(r, opts, probs)
	} else {
		vols = q.RandomStricterVolatiles(r, opts, probs)
	}
	// CVQualifiers.cpp:209 — ERROR_GUARD after random_*_volatiles
	if HasError() {
		return NewCVQualifiers(nil, vols)
	}
	if !noVolatile && !cg.EffectContext().IsSideEffectFree() && len(vols) > 0 {
		vols[len(vols)-1] = false
	}
	MakeScalarVolatiles(opts, vols)
	// CVQualifiers.cpp:215 — ERROR_GUARD
	if HasError() {
		return NewCVQualifiers(nil, vols)
	}

	var consts []bool
	if !q.AcceptStricter {
		consts = q.RandomLooserConsts(r, opts, probs)
	} else {
		consts = q.RandomStricterConsts(r, opts, probs)
	}
	MakeScalarConsts(opts, consts)
	// CVQualifiers.cpp:219 — ERROR_GUARD after random_*_consts
	if HasError() {
		return NewCVQualifiers(consts, vols)
	}
	if access == AccessWrite && len(consts) > 0 {
		consts[len(consts)-1] = false
	}
	return NewCVQualifiers(consts, vols)
}

// RandomLooseQualifiers mirrors CVQualifiers::random_loose_qualifiers.
// CVQualifiers.cpp:231–259 — always looser (ignore AcceptStricter for direction).
func (q CVQualifiers) RandomLooseQualifiers(
	noVolatile bool,
	access Access,
	cg CGContext,
	opts Options,
	probs *Probabilities,
	r *Rng,
) CVQualifiers {
	if q.Wildcard {
		return CVQualifiers{Wildcard: true, AcceptStricter: q.AcceptStricter}
	}
	// CVQualifiers.cpp always has process RNG; no invent fixed looser shells
	if r == nil {
		return CVQualifiers{}
	}
	// incomplete ambient fails closed sticky (no invent looser qfer past holes)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	var vols []bool
	if noVolatile {
		vols = make([]bool, len(q.IsVolatiles))
	} else {
		vols = q.RandomLooserVolatiles(r, opts, probs)
		if !cg.EffectContext().IsSideEffectFree() && len(vols) > 0 {
			vols[len(vols)-1] = false
		}
	}
	MakeScalarVolatiles(opts, vols)
	consts := q.RandomLooserConsts(r, opts, probs)
	MakeScalarConsts(opts, consts)
	if access == AccessWrite && len(consts) > 0 {
		consts[len(consts)-1] = false
	}
	return NewCVQualifiers(consts, vols)
}

// RandomAddQualifiers mirrors CVQualifiers::random_add_qualifiers.
// CVQualifiers.cpp:467–494 — append one pointer level with const/volatile probs.
func (q CVQualifiers) RandomAddQualifiers(r *Rng, opts Options, probs *Probabilities, noVolatile bool) CVQualifiers {
	out := q
	out.IsConsts = append([]bool(nil), q.IsConsts...)
	out.IsVolatiles = append([]bool(nil), q.IsVolatiles...)
	if opts.MatchExactQualifiers {
		out.AddQualifiers(false, false)
		return out
	}
	// CVQualifiers.cpp:474–476 — DEPTH_GUARD_BY_DEPTH_RETURN(1 or 2, qfer)
	need := 1
	if !noVolatile {
		need = 2
	}
	if DepthGuardByDepth(opts, need) == BadDepth {
		return out
	}
	// CVQualifiers.cpp always has process RNG for const/vol flips
	// no invent fixed non-const non-vol pointer level without draw
	if r == nil {
		return q
	}
	isConst := false
	if opts.ConstPointers && probs != nil {
		isConst = r.RndFlipcoin(uint32(probs.Single(PRegularConstProb)))
	}
	isVol := false
	if !noVolatile && opts.VolatilePointers && probs != nil {
		isVol = r.RndFlipcoin(uint32(probs.Single(PRegularVolatileProb)))
	}
	out.AddQualifiers(isConst, isVol)
	return out
}

// OutputFirstQuals mirrors CVQualifiers::OutputFirstQuals.
// CVQualifiers.cpp:639–650 — leading const/volatile of level 0.
// assert(0) if const/vol bit set when option disabled — emit nothing for that bit.
func (q CVQualifiers) OutputFirstQuals() string {
	opts := ProcessOptions()
	var b strings.Builder
	if len(q.IsConsts) > 0 && q.IsConsts[0] {
		// CVQualifiers.cpp:641–642 — assert(consts())
		if opts.Consts {
			b.WriteString("const ")
		}
	}
	if len(q.IsVolatiles) > 0 && q.IsVolatiles[0] {
		// CVQualifiers.cpp:647–648 — assert(volatiles())
		if opts.Volatiles {
			b.WriteString("volatile ")
		}
	}
	return b.String()
}

// GetAllQualifiers mirrors CVQualifiers::get_all_qualifiers.
// CVQualifiers.cpp:617–637 — enumerate all const×volatile combos for one level.
// Probabilities are ignored for enumeration (upstream uses them only for enumerator filter).
func GetAllQualifiers(constProb, volatileProb uint32) []CVQualifiers {
	_ = constProb
	_ = volatileProb
	// full boolean product: (false,false), (true,false), (false,true), (true,true)
	// when prob is 0, still include false for that axis only if we followed
	// enumerator — upstream always enumerates both when elems added.
	// Fair port: always all four combinations (exhaustive).
	var out []CVQualifiers
	for _, c := range []bool{false, true} {
		for _, v := range []bool{false, true} {
			out = append(out, NewCVQualifiers([]bool{c}, []bool{v}))
		}
	}
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

// SetConst mirrors CVQualifiers::set_const.
// CVQualifiers.cpp:588–593 — is_consts[len - pos - 1] = is_const; pos default 0
// is storage (last). No soft invent grow when empty or pos OOB.
func (q *CVQualifiers) SetConst(isConst bool, pos int) {
	if q == nil {
		return
	}
	n := len(q.IsConsts)
	if n == 0 {
		return
	}
	if pos < 0 {
		pos = 0
	}
	idx := n - pos - 1
	if idx < 0 || idx >= n {
		return
	}
	q.IsConsts[idx] = isConst
}

// SetVolatile mirrors CVQualifiers::set_volatile.
// CVQualifiers.cpp:595–600 — is_volatiles[len - pos - 1]; no invent grow.
func (q *CVQualifiers) SetVolatile(isVol bool, pos int) {
	if q == nil {
		return
	}
	n := len(q.IsVolatiles)
	if n == 0 {
		return
	}
	if pos < 0 {
		pos = 0
	}
	idx := n - pos - 1
	if idx < 0 || idx >= n {
		return
	}
	q.IsVolatiles[idx] = isVol
}

// Restrict mirrors CVQualifiers::restrict.
// CVQualifiers.cpp:602–609 — WRITE → set_const(false); non-SE-free → set_volatile(false).
// Only storage slot (pos=0 → last); no dual-clear invent of first+last.
func (q *CVQualifiers) Restrict(access Access, cg CGContext) {
	if q == nil || q.Wildcard {
		return
	}
	// incomplete ambient fails closed sticky (no invent clear-vol via IncompleteEffect SE-false)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return
	}
	if access == AccessWrite {
		q.SetConst(false, 0)
	}
	if !cg.EffectContext().IsSideEffectFree() {
		q.SetVolatile(false, 0)
	}
}

// OutputQualifiedType mirrors CVQualifiers::output_qualified_type.
// CVQualifiers.cpp:530–556 — const/volatile interleaved with * and base type first.
// Uses ProcessOptions for CGOptions::consts/volatiles (assert when bit set but option off).
func (q CVQualifiers) OutputQualifiedType(t *Type) string {
	// CVQualifiers.cpp:532 — assert(t); sticky no soft invent "void" for nil type
	if t == nil {
		SetError(ErrGeneric)
		return ""
	}
	// CVQualifiers.cpp:533 — assert(sanity_check(t)); sticky no invent bare CName for bad layout
	if !q.Wildcard && len(q.IsConsts) > 0 && !q.SanityCheck(t) {
		SetError(ErrGeneric)
		return ""
	}
	opts := ProcessOptions()
	emitConst := func() bool { return opts.Consts }
	emitVol := func() bool { return opts.Volatiles }
	if q.Wildcard || len(q.IsConsts) == 0 {
		// bare type + single-level quals from storage
		var b strings.Builder
		// CVQualifiers.cpp:541–544 — assert(0) if const bit without Consts option
		if q.IsConst() && emitConst() {
			b.WriteString("const ")
		}
		if q.IsVolatile() && emitVol() {
			b.WriteString("volatile ")
		}
		b.WriteString(t.CName())
		return b.String()
	}
	base := t.BaseType()
	if base == nil {
		base = t
	}
	// For simple types with one qualifier level: "const volatile int"
	if t.IsSimple() || t.IsAggregate() {
		var b strings.Builder
		if len(q.IsConsts) > 0 && q.IsConsts[0] && emitConst() {
			b.WriteString("const ")
		}
		if len(q.IsVolatiles) > 0 && q.IsVolatiles[0] && emitVol() {
			b.WriteString("volatile ")
		}
		b.WriteString(t.CName())
		return b.String()
	}
	// pointer: const volatile base * const * ...
	var b strings.Builder
	for i := 0; i < len(q.IsConsts); i++ {
		if i > 0 {
			b.WriteString("*")
		}
		// CVQualifiers.cpp:540–544 / 545–552 — no invent const/vol when option disabled
		if q.IsConsts[i] && emitConst() {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString("const ")
		}
		if i < len(q.IsVolatiles) && q.IsVolatiles[i] && emitVol() {
			if i > 0 && !(q.IsConsts[i] && emitConst()) {
				b.WriteString(" ")
			}
			b.WriteString("volatile ")
		}
		if i == 0 {
			b.WriteString(base.CName())
			b.WriteString(" ")
		}
	}
	return strings.TrimSpace(b.String())
}

// isVolatileOKOnOneLevel mirrors is_volatile_ok_on_one_level (CVQualifiers.cpp).
// CVQualifiers.cpp:269–293 — non-C++ always OK; struct/union need has_assign_ops;
// union with struct field (or nested bad union) forbids volatile.
func isVolatileOKOnOneLevel(opts Options, t *Type) bool {
	if !opts.LangCPP {
		return true
	}
	if t == nil || (!t.IsStruct() && !t.IsUnion()) {
		return true
	}
	if !t.HasAssignOps {
		return false
	}
	if t.IsStruct() {
		return true
	}
	// Union: nested struct field blocks volatile; nested unions recurse.
	// Type* always live on Fields; nil hole fails closed (not volatile-OK).
	for _, f := range t.Fields {
		if f.Type == nil {
			return false
		}
		if f.Type.IsStruct() {
			return false
		}
		if f.Type.IsUnion() && !isVolatileOKOnOneLevel(opts, f.Type) {
			return false
		}
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
	// CVQualifiers.cpp:295+ — always has RNG; no soft invent NewRng(0)
	if r == nil {
		return CVQualifiers{}
	}
	// incomplete ambient fails closed sticky (no invent non-vol qfer / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return CVQualifiers{}
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
	// C++ Probabilities singleton always live; nil probs → 0% (no invent NewProbabilities)
	constP, volP := uint32(0), uint32(0)
	if probs != nil {
		constP = uint32(probs.Single(PRegularConstProb))
		volP = uint32(probs.Single(PRegularVolatileProb))
	}
	// RegularConstProb() / RegularVolatileProb() → single probs
	return RandomQualifiersForType(
		t, access, cg, noVolatile,
		constP, volP,
		opts, r,
	)
}

// RandomQualifiersNoContextNoVolatile mirrors CVQualifiers::random_qualifiers(const Type *t)
// → READ, empty context, no_volatile=true.
func RandomQualifiersNoContextNoVolatile(t *Type, opts Options, probs *Probabilities, r *Rng) CVQualifiers {
	return RandomQualifiersDefaultProbs(t, AccessRead, EmptyCGContext(), true, opts, probs, r)
}
