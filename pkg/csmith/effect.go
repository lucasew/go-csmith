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

// Effect is a minimal Effect.cpp stand-in (purity / SE-free + read/write sets).
type Effect struct {
	pure           bool
	sideEffectFree bool
	// written tracks variables written in this effect (Effect::write_vars subset).
	written map[*Variable]bool
	// read tracks variables read (Effect::read_vars subset).
	read map[*Variable]bool
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

// AddExternalEffect mirrors Effect::add_external_effect — only global reads/writes.
// Effect.cpp:192–215.
func (e Effect) AddExternalEffect(other Effect) Effect {
	out := e
	for v, ok := range other.read {
		if ok && v != nil && v.IsGlobal() {
			out = out.ReadVar(v)
		}
	}
	for v, ok := range other.written {
		if ok && v != nil && v.IsGlobal() {
			out = out.WriteVar(v)
			out.pure = false
		}
	}
	out.sideEffectFree = out.sideEffectFree && other.sideEffectFree
	return out
}

// AddEffect mirrors Effect::add_effect — union of reads/writes; pure/SE-free AND.
// Effect.cpp:157–186 (lhs_write_vars omitted).
func (e Effect) AddEffect(other Effect) Effect {
	out := e
	if other.read != nil {
		if out.read == nil {
			out.read = make(map[*Variable]bool)
		}
		nr := make(map[*Variable]bool, len(out.read)+len(other.read))
		for k, v := range out.read {
			nr[k] = v
		}
		for k, v := range other.read {
			if v {
				nr[k] = true
			}
		}
		out.read = nr
	}
	if other.written != nil {
		if out.written == nil {
			out.written = make(map[*Variable]bool)
		}
		nw := make(map[*Variable]bool, len(out.written)+len(other.written))
		for k, v := range out.written {
			nw[k] = v
		}
		for k, v := range other.written {
			if v {
				nw[k] = true
			}
		}
		out.written = nw
	}
	out.pure = e.pure && other.pure
	out.sideEffectFree = e.sideEffectFree && other.sideEffectFree
	return out
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

// ReadVar mirrors Effect::read_var — records a read (does not alone clear SE-free).
// Effect.cpp read_var path (simplified).
func (e Effect) ReadVar(v *Variable) Effect {
	if v == nil {
		return e
	}
	nr := make(map[*Variable]bool, len(e.read)+1)
	for k, val := range e.read {
		nr[k] = val
	}
	nr[v] = true
	e.read = nr
	return e
}

// IsWritten mirrors Effect::is_written — exact or parent field_var_of.
// Effect.cpp:333–345.
func (e Effect) IsWritten(v *Variable) bool {
	if v == nil {
		return false
	}
	if e.written[v] {
		return true
	}
	// if we write a struct/union, all fields are written too
	if v.FieldVarOf != nil {
		return e.IsWritten(v.FieldVarOf)
	}
	return false
}

// ReadVars mirrors Effect::get_read_vars — list of read variables (stable order by name).
func (e Effect) ReadVars() []*Variable {
	if len(e.read) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.read))
	for v, ok := range e.read {
		if ok && v != nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// WrittenVars mirrors Effect::get_write_vars subset.
func (e Effect) WrittenVars() []*Variable {
	if len(e.written) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.written))
	for v, ok := range e.written {
		if ok && v != nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsRead mirrors Effect::is_read — exact or struct parent (not union).
// Effect.cpp:276–289.
func (e Effect) IsRead(v *Variable) bool {
	if v == nil {
		return false
	}
	if e.read[v] {
		return true
	}
	// struct fields inherit parent read; unions do not
	if v.FieldVarOf != nil && v.FieldVarOf.Type != nil && v.FieldVarOf.Type.IsStruct() {
		return e.IsRead(v.FieldVarOf)
	}
	return false
}

// FieldIsRead mirrors Effect::field_is_read — any field of aggregate read.
// Effect.cpp:389–399.
func (e Effect) FieldIsRead(v *Variable) bool {
	if v == nil || !v.IsAggregate() {
		return false
	}
	for _, f := range v.FieldVars {
		if e.IsRead(f) || e.FieldIsRead(f) {
			return true
		}
	}
	return false
}

// FieldIsWritten mirrors Effect::field_is_written.
// Effect.cpp:404–414.
func (e Effect) FieldIsWritten(v *Variable) bool {
	if v == nil || !v.IsAggregate() {
		return false
	}
	for _, f := range v.FieldVars {
		if e.IsWritten(f) || e.FieldIsWritten(f) {
			return true
		}
	}
	return false
}

// SiblingUnionFieldIsRead mirrors Effect::sibling_union_field_is_read.
// Effect.cpp:416–428 — another field of the same container union was read.
func (e Effect) SiblingUnionFieldIsRead(v *Variable) bool {
	if v == nil {
		return false
	}
	you := v.GetCollective().GetContainerUnion()
	if you == nil {
		return false
	}
	for r := range e.read {
		if r == nil || !e.read[r] {
			continue
		}
		me := r.GetCollective().GetContainerUnion()
		if me == you {
			return true
		}
	}
	return false
}

// SiblingUnionFieldIsWritten mirrors Effect::sibling_union_field_is_written.
// Effect.cpp:430–441.
func (e Effect) SiblingUnionFieldIsWritten(v *Variable) bool {
	if v == nil {
		return false
	}
	you := v.GetCollective().GetContainerUnion()
	if you == nil {
		return false
	}
	for w := range e.written {
		if w == nil || !e.written[w] {
			continue
		}
		me := w.GetCollective().GetContainerUnion()
		if me == you {
			return true
		}
	}
	return false
}

// IsReadPartially mirrors Effect::is_read_partially.
// Effect.cpp:444–446.
func (e Effect) IsReadPartially(v *Variable) bool {
	return e.IsRead(v) || e.FieldIsRead(v) || e.SiblingUnionFieldIsRead(v)
}

// IsWrittenPartially mirrors Effect::is_written_partially.
// Effect.cpp:448–451.
func (e Effect) IsWrittenPartially(v *Variable) bool {
	return e.IsWritten(v) || e.FieldIsWritten(v) || e.SiblingUnionFieldIsWritten(v)
}

// CommentOutput mirrors Effect::Output as a C block-comment line for Function::Output.
// Effect.cpp:507–529 — " * reads :" / " * writes:" lists.
// Write names sorted for deterministic emit (Go map iteration is random).
func (e Effect) CommentOutput() string {
	var b strings.Builder
	b.WriteString("/*\n")
	b.WriteString(" * reads :")
	rnames := make([]string, 0, len(e.read))
	for v := range e.read {
		if v != nil && e.read[v] {
			rnames = append(rnames, v.Name)
		}
	}
	sort.Strings(rnames)
	for _, n := range rnames {
		b.WriteString(" ")
		b.WriteString(n)
	}
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

// MergeEffects combines two post-branch effects (union of reads/writes; SE-free only if both are).
// Approximates StatementIf fact/effect merge without FactMgr.
func MergeEffects(a, b Effect) Effect {
	out := Effect{
		pure:           a.pure && b.pure,
		sideEffectFree: a.sideEffectFree && b.sideEffectFree,
	}
	if len(a.written)+len(b.written) > 0 {
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
	}
	if len(a.read)+len(b.read) > 0 {
		out.read = make(map[*Variable]bool, len(a.read)+len(b.read))
		for k, v := range a.read {
			if v {
				out.read[k] = true
			}
		}
		for k, v := range b.read {
			if v {
				out.read[k] = true
			}
		}
	}
	if len(out.written) > 0 {
		out.pure = false
		out.sideEffectFree = false
	}
	return out
}
