// Upstream: Effect.h / Effect.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"sort"
	"strings"
)

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

// CommentOutput mirrors Effect::Output as a C block-comment line for Function::Output.
// Effect.cpp:507–529 — " * reads :" / " * writes:" lists.
// Write names sorted for deterministic emit (Go map iteration is random).
func (e Effect) CommentOutput() string {
	var b strings.Builder
	b.WriteString("/*\n")
	b.WriteString(" * reads :")
	// read set not tracked yet
	b.WriteString("\n")
	b.WriteString(" * writes:")
	names := make([]string, 0, len(e.written))
	for v := range e.written {
		if v != nil && e.written[v] {
			names = append(names, v.Name)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		b.WriteString(" ")
		b.WriteString(n)
	}
	b.WriteString("\n")
	b.WriteString(" */\n")
	return b.String()
}

// MergeEffects combines two post-branch effects (union of writes; SE-free only if both are).
// Approximates StatementIf fact/effect merge without FactMgr.
func MergeEffects(a, b Effect) Effect {
	out := Effect{
		pure:           a.pure && b.pure,
		sideEffectFree: a.sideEffectFree && b.sideEffectFree,
	}
	if len(a.written) == 0 && len(b.written) == 0 {
		return out
	}
	out.written = make(map[*Variable]bool, len(a.written)+len(b.written))
	for k, v := range a.written {
		if v {
			out.written[k] = true
		}
	}
	for k, v := range b.written {
		if v {
			out.written[k] = true
		}
	}
	if len(out.written) > 0 {
		out.pure = false
		out.sideEffectFree = false
	}
	return out
}
