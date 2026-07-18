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
