// Upstream: Effect.h / Effect.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Access mirrors Effect::Access.
type Access int

const (
	// AccessRead is Effect::Access::READ.
	AccessRead Access = iota
	// AccessWrite is Effect::Access::WRITE.
	AccessWrite
)

// Effect is a minimal Effect.cpp stand-in (purity / SE-free + write set subset).
type Effect struct {
	pure           bool
	sideEffectFree bool
	// written tracks variables written in this effect (Effect::write_vars subset).
	written map[*Variable]bool
}

// EmptyEffect is Effect::empty_effect (pure, side-effect free).
// Effect.cpp: Effect::Effect() : pure(true), side_effect_free(true)
func EmptyEffect() Effect {
	return Effect{pure: true, sideEffectFree: true}
}

// IsPure mirrors Effect::is_pure.
func (e Effect) IsPure() bool { return e.pure }

// IsSideEffectFree mirrors Effect::is_side_effect_free.
func (e Effect) IsSideEffectFree() bool { return e.sideEffectFree }

// WithSideEffects returns a non-SE-free effect (for tests / context).
func WithSideEffects() Effect {
	return Effect{pure: false, sideEffectFree: false}
}

// WriteVar mirrors Effect::write_var — marks impure and records write.
// Effect.cpp write_var path (simplified, no partial/field).
func (e Effect) WriteVar(v *Variable) Effect {
	e.pure = false
	e.sideEffectFree = false
	if v != nil {
		if e.written == nil {
			e.written = make(map[*Variable]bool)
		}
		// copy-on-write for value semantics
		nw := make(map[*Variable]bool, len(e.written)+1)
		for k, val := range e.written {
			nw[k] = val
		}
		nw[v] = true
		e.written = nw
	}
	return e
}

// IsWritten mirrors Effect::is_written (exact var).
func (e Effect) IsWritten(v *Variable) bool {
	return v != nil && e.written[v]
}
