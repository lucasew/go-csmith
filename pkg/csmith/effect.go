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
	// incomplete marks fail-closed IR (nil map keys / incomplete merge inputs).
	// Distinct from complete empty (EmptyEffect): EffectComplete false.
	incomplete bool
	// written tracks variables written in this effect (Effect::write_vars subset).
	written map[*Variable]bool
	// read tracks variables read (Effect::read_vars subset).
	read map[*Variable]bool
	// lhsWrite tracks LHS write vars (Effect::lhs_write_vars).
	// Effect.h:104 — used after assign visit to extend running context.
	lhsWrite map[*Variable]bool
}

// EmptyEffect is Effect::empty_effect (pure, side-effect free).
// Effect.cpp: Effect::Effect() : pure(true), side_effect_free(true)
func EmptyEffect() Effect {
	return Effect{pure: true, sideEffectFree: true}
}

// IncompleteEffect is the fail-closed incomplete effect marker.
// EffectComplete returns false. Distinct from EmptyEffect (complete empty).
// Use when merge/write-set hits nil holes so callers cannot invent empty-complete
// success via IsEmpty / pure / side-effect-free checks on leave-base returns.
func IncompleteEffect() Effect {
	return Effect{incomplete: true, pure: false, sideEffectFree: false}
}

// EffectComplete reports the effect is not a fail-closed incomplete marker
// and has no nil Variable* map keys.
func EffectComplete(e Effect) bool {
	if e.incomplete {
		return false
	}
	return effectMapKeysComplete(e.read) && effectMapKeysComplete(e.written) && effectMapKeysComplete(e.lhsWrite)
}

// Clone mirrors Effect copy ctor with deep map copies (Go maps are shared refs).
// Effect.cpp:84–89.
func (e Effect) Clone() Effect {
	out := Effect{pure: e.pure, sideEffectFree: e.sideEffectFree, incomplete: e.incomplete}
	if e.incomplete {
		return IncompleteEffect()
	}
	if len(e.read) > 0 {
		out.read = make(map[*Variable]bool, len(e.read))
		for k, v := range e.read {
			out.read[k] = v
		}
	}
	if len(e.written) > 0 {
		out.written = make(map[*Variable]bool, len(e.written))
		for k, v := range e.written {
			out.written[k] = v
		}
	}
	if len(e.lhsWrite) > 0 {
		out.lhsWrite = make(map[*Variable]bool, len(e.lhsWrite))
		for k, v := range e.lhsWrite {
			out.lhsWrite[k] = v
		}
	}
	return out
}

// IsPure mirrors Effect::is_pure.
// Incomplete effects sticky not-pure (no invent pure success / soft re-pick past holes).
func (e Effect) IsPure() bool {
	if e.incomplete {
		SetError(ErrGeneric)
		return false
	}
	return e.pure
}

// IsSideEffectFree mirrors Effect::is_side_effect_free.
// Incomplete effects sticky not SE-free (no invent SE-free success / soft re-pick past holes).
func (e Effect) IsSideEffectFree() bool {
	if e.incomplete {
		SetError(ErrGeneric)
		return false
	}
	return e.sideEffectFree
}

// WithSideEffects returns a non-SE-free effect (for tests / context).
func WithSideEffects() Effect {
	return Effect{pure: false, sideEffectFree: false}
}

// AddExternalEffect mirrors Effect::add_external_effect — only global reads/writes.
// Effect.cpp:192–215.
func (e Effect) AddExternalEffect(other Effect) Effect {
	return e.AddExternalEffectWithCallers(other, nil)
}

// AddExternalEffectWithCallers mirrors Effect::add_external_effect(e, call_chain).
// Effect.cpp:221–269 — globals always; non-globals only if on a call_chain stack frame.
// Variable* always live in effect lists; nil hole fails closed sticky IncompleteEffect
// (no invent partial external merge past holes / soft re-pick). Incomplete Param/LocalVars
// on a frame also fails closed sticky (no invent not-on-chain via IsVarOnStack false past hole).
func (e Effect) AddExternalEffectWithCallers(other Effect, callChain []*Block) Effect {
	// incomplete effect maps / call chain fail closed sticky IncompleteEffect
	// (no invent leave-base empty-complete success)
	if !EffectComplete(e) || !EffectComplete(other) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	reads := other.ReadVars()
	writes := other.WrittenVars()
	if !VariablesComplete(reads) || !VariablesComplete(writes) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	if !BlocksComplete(callChain) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	for _, b := range callChain {
		if !b.StackScanComplete() {
			SetError(ErrGeneric)
			return IncompleteEffect()
		}
	}
	out := e
	for _, v := range reads {
		if v.IsGlobal() {
			// residual ERROR sticky — no invent soft-continue merge past IsGlobal hole
			if HasError() {
				return IncompleteEffect()
			}
			out = out.ReadVar(v)
			continue
		}
		if varOnCallChain(v, callChain) {
			// residual ERROR sticky — no invent soft-skip past IsVarOnStack hole
			if HasError() {
				return IncompleteEffect()
			}
			out = out.ReadVar(v)
		} else if HasError() {
			// residual ERROR sticky — no invent not-on-chain soft-skip past hole
			return IncompleteEffect()
		}
	}
	for _, v := range writes {
		if v.IsGlobal() {
			// residual ERROR sticky — no invent soft-continue merge past IsGlobal hole
			if HasError() {
				return IncompleteEffect()
			}
			out = out.WriteVar(v)
			out.pure = false
			continue
		}
		if varOnCallChain(v, callChain) {
			// residual ERROR sticky — no invent soft-skip past IsVarOnStack hole
			if HasError() {
				return IncompleteEffect()
			}
			out = out.WriteVar(v)
			out.pure = false
		} else if HasError() {
			// residual ERROR sticky — no invent not-on-chain soft-skip past hole
			return IncompleteEffect()
		}
	}
	out.sideEffectFree = out.sideEffectFree && other.sideEffectFree
	return out
}

func varOnCallChain(v *Variable, chain []*Block) bool {
	for _, b := range chain {
		// chain pre-validated complete at AddExternalEffectWithCallers
		if b == nil {
			continue
		}
		if !b.StackScanComplete() {
			// incomplete stack sticky not-on-chain residual for caller IncompleteEffect
			SetError(ErrGeneric)
			return false
		}
		if b.IsVarOnStack(v) {
			// residual ERROR sticky — no invent on-chain true past IsVarOnStack hole
			if HasError() {
				return false
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue not-on-chain past IsVarOnStack residual
		if HasError() {
			return false
		}
	}
	return false
}

// AddEffect mirrors Effect::add_effect(e) without LHS set merge.
// Effect.cpp:157–186.
func (e Effect) AddEffect(other Effect) Effect {
	return e.AddEffectOpts(other, false)
}

// AddEffectOpts mirrors Effect::add_effect(e, include_lhs_effects).
// Variable* always live as map keys; incomplete either side fails closed sticky
// IncompleteEffect (no invent leave-base empty-complete merge / soft re-pick past hole).
func (e Effect) AddEffectOpts(other Effect, includeLHS bool) Effect {
	if !EffectComplete(e) || !EffectComplete(other) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
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
	if includeLHS && other.lhsWrite != nil {
		if out.lhsWrite == nil {
			out.lhsWrite = make(map[*Variable]bool)
		}
		nl := make(map[*Variable]bool, len(out.lhsWrite)+len(other.lhsWrite))
		for k, v := range out.lhsWrite {
			nl[k] = v
		}
		for k, v := range other.lhsWrite {
			if v {
				nl[k] = true
			}
		}
		out.lhsWrite = nl
	}
	out.pure = e.pure && other.pure
	out.sideEffectFree = e.sideEffectFree && other.sideEffectFree
	return out
}

// WriteVarSet mirrors Effect::write_var_set — write each var.
// Effect.cpp:148–152.
// Variable* always live; incomplete list or base fails closed sticky IncompleteEffect
// (no invent partial writes / soft re-pick leave-base empty-complete success).
func (e Effect) WriteVarSet(vars []*Variable) Effect {
	if !EffectComplete(e) || !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	for _, v := range vars {
		out = out.WriteVar(v)
	}
	return out
}

// SetLhsWriteVars mirrors Effect::set_lhs_write_vars from current write_vars.
// Lhs.cpp:348–351 — after successful LHS visit.
// Incomplete write map fails closed sticky IncompleteEffect (no invent empty lhsWrite).
func (e Effect) SetLhsWriteVarsFromWritten() Effect {
	if !EffectComplete(e) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	if len(e.written) == 0 {
		out.lhsWrite = nil
		return out
	}
	out.lhsWrite = make(map[*Variable]bool, len(e.written))
	for k, v := range e.written {
		if k == nil {
			SetError(ErrGeneric)
			return IncompleteEffect()
		}
		if v {
			out.lhsWrite[k] = true
		}
	}
	return out
}

// LhsWriteVars returns lhs_write_vars as a slice (stable name order).
// Incomplete effect / nil map keys fail closed sticky IncompleteVariables
// (not bare nil invent empty-complete lhs set via VariablesComplete(nil)/len==0 —
// assign merge uses incomplete as WriteVarSet input; soft re-pick past hole).
func (e Effect) LhsWriteVars() []*Variable {
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if len(e.lhsWrite) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.lhsWrite))
	for v, ok := range e.lhsWrite {
		if !ok {
			continue
		}
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// WriteVar mirrors Effect::write_var.
// Effect.cpp:137–146 — record write; SE-free &= !volatile && !access_once;
// pure left unchanged (upstream "pure = pure").
// Incomplete base / nil Variable sticky IncompleteEffect (no invent grow write map
// / soft-skip identity past holes).
func (e Effect) WriteVar(v *Variable) Effect {
	// Variable always live; sticky incomplete no invent identity no-op past hole
	if v == nil {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
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
	// Effect.cpp:144–145 — SE-free means volatile/access_once free
	if v.IsVolatile() || v.IsAccessOnce {
		e.sideEffectFree = false
	}
	return e
}

// ReadVar mirrors Effect::read_var.
// Effect.cpp:116–122 — record read; pure &= const&&!vol&&!access_once;
// SE-free &= !vol&&!access_once.
// Incomplete base / nil Variable sticky IncompleteEffect (no invent grow read map
// / soft-skip identity past holes).
func (e Effect) ReadVar(v *Variable) Effect {
	// Variable always live; sticky incomplete no invent identity no-op past hole
	if v == nil {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	nr := make(map[*Variable]bool, len(e.read)+1)
	for k, val := range e.read {
		nr[k] = val
	}
	nr[v] = true
	e.read = nr
	// Effect.cpp:120–121
	if !(v.IsConst() && !v.IsVolatile() && !v.IsAccessOnce) {
		e.pure = false
	}
	if v.IsVolatile() || v.IsAccessOnce {
		e.sideEffectFree = false
	}
	return e
}

// IsWritten mirrors Effect::is_written — exact or parent field_var_of.
// Effect.cpp:333–345.
// Incomplete effect sticky true (no invent not-written / conflict-free past holes).
// Type-nil parent sticky written true (restrictive — no invent not-written /
// conflict-free soft-skip past incomplete parent type shell; mirrors IsRead).
func (e Effect) IsWritten(v *Variable) bool {
	// Variable always live; sticky incomplete no invent not-written soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as written (restrictive)
		SetError(ErrGeneric)
		return true
	}
	if e.written[v] {
		return true
	}
	// if we write a struct/union, all fields are written too
	// Type-nil parent sticky written true (restrictive — no invent not-written
	// / conflict-free soft-skip past incomplete parent type shell)
	if v.FieldVarOf != nil {
		if v.FieldVarOf.Type == nil {
			SetError(ErrGeneric)
			return true
		}
		return e.IsWritten(v.FieldVarOf)
	}
	return false
}

// ReadVars mirrors Effect::get_read_vars — list of read variables (stable order by name).
// Incomplete effect / nil map keys fail closed sticky IncompleteVariables
// (no invent skip hole as absent read / soft re-pick empty-complete read set).
func (e Effect) ReadVars() []*Variable {
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if len(e.read) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.read))
	for v, ok := range e.read {
		if !ok {
			continue
		}
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// WrittenVars mirrors Effect::get_write_vars subset.
// Incomplete effect / nil map keys fail closed sticky IncompleteVariables
// (no invent skip hole as absent write / soft re-pick empty-complete write set).
func (e Effect) WrittenVars() []*Variable {
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if len(e.written) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.written))
	for v, ok := range e.written {
		if !ok {
			continue
		}
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsRead mirrors Effect::is_read — exact or struct parent (not union).
// Effect.cpp:276–289.
// Incomplete effect sticky true (no invent not-read / conflict-free past holes).
func (e Effect) IsRead(v *Variable) bool {
	// Variable always live; sticky incomplete no invent not-read soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as read (restrictive)
		SetError(ErrGeneric)
		return true
	}
	if e.read[v] {
		return true
	}
	// struct fields inherit parent read; unions do not
	// Type-nil parent sticky read true (restrictive — no invent not-read / conflict-free
	// soft-skip past incomplete parent type shell)
	if v.FieldVarOf != nil {
		if v.FieldVarOf.Type == nil {
			SetError(ErrGeneric)
			return true
		}
		if v.FieldVarOf.Type.IsStruct() {
			return e.IsRead(v.FieldVarOf)
		}
	}
	return false
}

// FieldIsRead mirrors Effect::field_is_read — any field of aggregate read.
// Effect.cpp:389–399.
// Variable always live; sticky true (no invent no-field-read soft-skip past hole).
// Type* always live for non-special subjects; Type-nil sticky true (IsAggregate
// residual ERROR+false invents no-field-read / conflict-free past Type-nil shell).
// Variable* always live in FieldVars; nil hole sticky fail closed as true (no invent none).
// Incomplete effect fails closed as true sticky (incomplete marker soft re-pick banned).
func (e Effect) FieldIsRead(v *Variable) bool {
	if v == nil {
		SetError(ErrGeneric)
		return true
	}
	// Type* always live for non-special subjects; Type-nil sticky true
	// (no invent IsAggregate residual false as no-field-read past shell)
	if !IsSpecialPtr(v) && v.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	if !v.IsAggregate() {
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as field-read (restrictive)
		SetError(ErrGeneric)
		return true
	}
	for _, f := range v.FieldVars {
		if f == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.IsRead(f) {
			// residual ERROR sticky — no invent field-read true past IsRead hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue nested past IsRead residual false
		if HasError() {
			return true
		}
		if e.FieldIsRead(f) {
			// residual ERROR sticky — no invent field-read true past nested FieldIsRead hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual
		if HasError() {
			return true
		}
	}
	return false
}

// FieldIsWritten mirrors Effect::field_is_written.
// Effect.cpp:404–414.
// Variable always live; sticky true (no invent no-field-write soft-skip past hole).
// Type* always live for non-special subjects; Type-nil sticky true (IsAggregate
// residual ERROR+false invents no-field-write / conflict-free past Type-nil shell).
// Variable* always live in FieldVars; nil hole sticky fail closed as true (no invent none).
// Incomplete effect sticky true (no invent no-field-write soft re-pick past holes).
func (e Effect) FieldIsWritten(v *Variable) bool {
	if v == nil {
		SetError(ErrGeneric)
		return true
	}
	// Type* always live for non-special subjects; Type-nil sticky true
	// (no invent IsAggregate residual false as no-field-write past shell)
	if !IsSpecialPtr(v) && v.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	if !v.IsAggregate() {
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as field-written (restrictive)
		SetError(ErrGeneric)
		return true
	}
	for _, f := range v.FieldVars {
		if f == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.IsWritten(f) {
			// residual ERROR sticky — no invent field-written true past IsWritten hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue nested past IsWritten residual false
		if HasError() {
			return true
		}
		if e.FieldIsWritten(f) {
			// residual ERROR sticky — no invent field-written true past nested FieldIsWritten hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual
		if HasError() {
			return true
		}
	}
	return false
}


// ancestryTypeHole reports Type-nil on FieldVarOf chain (excluding specials).
// Used by SiblingUnionField* so residual global HasError cannot invent sibling-use.
func ancestryTypeHole(v *Variable) bool {
	for p := v; p != nil; p = p.FieldVarOf {
		if IsSpecialPtr(p) {
			return false
		}
		if p.Type == nil {
			return true
		}
	}
	return false
}

// SiblingUnionFieldIsRead mirrors Effect::sibling_union_field_is_read.
// Effect.cpp:416–428 — another field of the same container union was read.
// Variable* always live as map keys; nil hole sticky fail closed as true (no invent none).
// Incomplete GetCollective sticky fail closed as true (no invent no-sibling / panic).
// Incomplete effect sticky true (no invent no-sibling soft re-pick past holes).
func (e Effect) SiblingUnionFieldIsRead(v *Variable) bool {
	// Variable always live; sticky incomplete no invent no-sibling soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if e.incomplete {
		SetError(ErrGeneric)
		return true
	}
	youColl := v.GetCollective()
	if youColl == nil {
		SetError(ErrGeneric)
		return true
	}
	you := youColl.GetContainerUnion()
	if you == nil {
		// Type-nil ancestry (not residual global HasError): restrictive true
		// (no invent no-sibling / conflict-free past incomplete container shell)
		if ancestryTypeHole(youColl) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return true
		}
		// complete no container union
		return false
	}
	for r := range e.read {
		if r == nil {
			SetError(ErrGeneric)
			return true
		}
		if !e.read[r] {
			continue
		}
		rColl := r.GetCollective()
		if rColl == nil {
			SetError(ErrGeneric)
			return true
		}
		me := rColl.GetContainerUnion()
		// residual ERROR sticky — no invent soft-continue no-sibling past GetContainerUnion hole
		if HasError() {
			return true
		}
		if me == you {
			return true
		}
	}
	return false
}

// SiblingUnionFieldIsWritten mirrors Effect::sibling_union_field_is_written.
// Effect.cpp:430–441.
// Variable* always live as map keys; nil hole sticky fail closed as true (no invent none).
// Incomplete GetCollective sticky fail closed as true (no invent no-sibling / panic).
// Incomplete effect sticky true (no invent no-sibling soft re-pick past holes).
func (e Effect) SiblingUnionFieldIsWritten(v *Variable) bool {
	// Variable always live; sticky incomplete no invent no-sibling soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if e.incomplete {
		SetError(ErrGeneric)
		return true
	}
	youColl := v.GetCollective()
	if youColl == nil {
		SetError(ErrGeneric)
		return true
	}
	you := youColl.GetContainerUnion()
	if you == nil {
		// Type-nil ancestry (not residual global HasError): restrictive true
		// (no invent no-sibling / conflict-free past incomplete container shell)
		if ancestryTypeHole(youColl) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return true
		}
		// complete no container union
		return false
	}
	for w := range e.written {
		if w == nil {
			SetError(ErrGeneric)
			return true
		}
		if !e.written[w] {
			continue
		}
		wColl := w.GetCollective()
		if wColl == nil {
			SetError(ErrGeneric)
			return true
		}
		me := wColl.GetContainerUnion()
		// residual ERROR sticky — no invent soft-continue no-sibling past GetContainerUnion hole
		if HasError() {
			return true
		}
		if me == you {
			return true
		}
	}
	return false
}

// HasRaceWith mirrors Effect::has_race_with.
// Effect.cpp:480–484 — non-empty intersection of read/write sets.
// Incomplete either side fails closed sticky as race (no invent race-free / soft re-pick).
func (e Effect) HasRaceWith(other Effect) bool {
	if !EffectComplete(e) || !EffectComplete(other) {
		SetError(ErrGeneric)
		return true
	}
	for _, v := range e.ReadVars() {
		if other.IsWritten(v) {
			// residual ERROR sticky — no invent race true past IsWritten hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue race-free past IsWritten residual
		if HasError() {
			return true
		}
	}
	for _, v := range e.WrittenVars() {
		if other.IsRead(v) {
			// residual ERROR sticky — no invent race true past IsRead hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue race-free past IsRead residual
		if HasError() {
			return true
		}
		if other.IsWritten(v) {
			// residual ERROR sticky — no invent race true past IsWritten hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue race-free past IsWritten residual
		if HasError() {
			return true
		}
	}
	return false
}

// IsEmpty mirrors Effect::is_empty — no reads and no writes.
// Effect.cpp:490–492.
// Incomplete sticky not-empty (no invent empty-complete free effect past holes).
func (e Effect) IsEmpty() bool {
	if e.incomplete {
		// IncompleteEffect sticky not-empty (restrictive — no invent empty-complete)
		SetError(ErrGeneric)
		return false
	}
	return len(e.ReadVars()) == 0 && len(e.WrittenVars()) == 0
}

// Clear mirrors Effect::clear — empty pure SE-free effect.
// Effect.cpp:497–501.
// Effect* always live at clear; sticky (no invent soft-skip clear past hole).
// Incomplete base stays IncompleteEffect sticky (no invent wipe hole shell to empty pure
// / soft re-pick past incomplete clear).
func (e *Effect) Clear() {
	if e == nil {
		SetError(ErrGeneric)
		return
	}
	if e.incomplete {
		*e = IncompleteEffect()
		SetError(ErrGeneric)
		return
	}
	*e = EmptyEffect()
}

// AccessDerefVolatile mirrors Effect::access_deref_volatile.
// Effect.cpp:124–135 — under strict_volatile_rule, clear SE-free if volatile after deref.
// Incomplete base fails closed sticky IncompleteEffect (no invent SE-free tweak on hole shell).
// Under strictVolatile, Variable always live; sticky IncompleteEffect (no invent SE-free skip).
func (e Effect) AccessDerefVolatile(v *Variable, derefLevel int, strictVolatile bool) Effect {
	if e.incomplete {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	if !strictVolatile {
		return e
	}
	if v == nil {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	level := derefLevel
	for level > 0 {
		if v.IsVolatileAfterDeref(level) {
			out.sideEffectFree = false
			return out
		}
		level--
	}
	return out
}

// IsReadPartially mirrors Effect::is_read_partially.
// Effect.cpp:444–446.
// Incomplete effect fails closed as true via IsRead/FieldIs*/Sibling* membership.
func (e Effect) IsReadPartially(v *Variable) bool {
	if e.IsRead(v) {
		// residual ERROR sticky — no invent partial-read true past IsRead hole
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue partial past IsRead residual false
	if HasError() {
		return true
	}
	if e.FieldIsRead(v) {
		if HasError() {
			return true
		}
		return true
	}
	if HasError() {
		return true
	}
	if e.SiblingUnionFieldIsRead(v) {
		if HasError() {
			return true
		}
		return true
	}
	if HasError() {
		return true
	}
	return false
}

// IsWrittenPartially mirrors Effect::is_written_partially.
// Effect.cpp:448–451.
// Incomplete effect fails closed as true via IsWritten/FieldIs*/Sibling* membership.
func (e Effect) IsWrittenPartially(v *Variable) bool {
	if e.IsWritten(v) {
		// residual ERROR sticky — no invent partial-write true past IsWritten hole
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue partial past IsWritten residual false
	if HasError() {
		return true
	}
	if e.FieldIsWritten(v) {
		if HasError() {
			return true
		}
		return true
	}
	if HasError() {
		return true
	}
	if e.SiblingUnionFieldIsWritten(v) {
		if HasError() {
			return true
		}
		return true
	}
	if HasError() {
		return true
	}
	return false
}

// HasGlobalEffect mirrors Effect::has_global_effect.
// Effect.cpp:543–562 — any global in read or write sets.
// Incomplete effect / nil map keys sticky true (no invent pure / no-global success).
func (e Effect) HasGlobalEffect() bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as has-global (restrictive)
		SetError(ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.read[v] && v.IsGlobal() {
			// residual ERROR sticky — no invent has-global true past IsGlobal hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-global past IsGlobal residual
		if HasError() {
			return true
		}
	}
	for v := range e.written {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.written[v] && v.IsGlobal() {
			// residual ERROR sticky — no invent has-global true past IsGlobal hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-global past IsGlobal residual
		if HasError() {
			return true
		}
	}
	return false
}

// UpdatePurity mirrors Effect::update_purity.
// Effect.cpp:535–538 — pure cleared when has_global_effect.
// Effect always live; sticky (no invent soft-skip purity update past hole).
func (e *Effect) UpdatePurity() {
	if e == nil {
		SetError(ErrGeneric)
		return
	}
	if e.HasGlobalEffect() {
		e.pure = false
	}
}

// UnionFieldIsRead mirrors Effect::union_field_is_read.
// Effect.cpp:565–572 — any read var is_inside_union_field.
// Incomplete effect / nil key fails closed as true (no invent none on
// IncompleteEffect empty maps).
func (e Effect) UnionFieldIsRead() bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as union-field-read (restrictive)
		SetError(ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.read[v] && v.IsInsideUnionField() {
			// residual ERROR sticky — no invent union-read true past IsInsideUnionField hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-union-read past IsInside residual
		if HasError() {
			return true
		}
	}
	return false
}

// effectMapKeysComplete reports map has no nil Variable* keys.
func effectMapKeysComplete(m map[*Variable]bool) bool {
	for v := range m {
		if v == nil {
			return false
		}
	}
	return true
}

// Consolidate mirrors Effect::consolidate.
// Effect.cpp:456–475 — drop field reads/writes covered by parent aggregate access.
// Effect* always live; sticky (no invent soft-skip consolidate past hole).
// Incomplete maps fail closed sticky IncompleteEffect (no invent partial deletes /
// leave-base complete success past holes under random map order / soft re-pick).
func (e *Effect) Consolidate() {
	if e == nil {
		SetError(ErrGeneric)
		return
	}
	if e.incomplete || !effectMapKeysComplete(e.read) || !effectMapKeysComplete(e.written) {
		*e = IncompleteEffect()
		SetError(ErrGeneric)
		return
	}
	// remove field reads when parent is also read
	for v := range e.read {
		if !e.read[v] {
			continue
		}
		if !v.IsFieldVar() {
			// residual ERROR sticky — no invent soft-skip consolidate past IsFieldVar hole
			if HasError() {
				*e = IncompleteEffect()
				return
			}
			continue
		}
		parent := v.FieldVarOf
		// FieldVarOf always live for field vars; Type* always live for non-special parents
		// Type-nil parent sticky IncompleteEffect (no invent leave-base complete past hole)
		if parent == nil {
			SetError(ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if parent.Type == nil && !IsSpecialPtr(parent) {
			SetError(ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if e.IsRead(parent) {
			// residual ERROR sticky — no invent soft-delete past IsRead hole
			if HasError() {
				*e = IncompleteEffect()
				return
			}
			delete(e.read, v)
		} else if HasError() {
			// residual ERROR sticky — no invent leave-base complete past IsRead hole
			*e = IncompleteEffect()
			return
		}
	}
	// remove field writes when parent is also written
	for v := range e.written {
		if !e.written[v] {
			continue
		}
		if !v.IsFieldVar() {
			// residual ERROR sticky — no invent soft-skip consolidate past IsFieldVar hole
			if HasError() {
				*e = IncompleteEffect()
				return
			}
			continue
		}
		parent := v.FieldVarOf
		if parent == nil {
			SetError(ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if parent.Type == nil && !IsSpecialPtr(parent) {
			SetError(ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if e.IsWritten(parent) {
			// residual ERROR sticky — no invent soft-delete past IsWritten hole
			if HasError() {
				*e = IncompleteEffect()
				return
			}
			delete(e.written, v)
		} else if HasError() {
			// residual ERROR sticky — no invent leave-base complete past IsWritten hole
			*e = IncompleteEffect()
			return
		}
	}
}

// IsReadByName mirrors Effect::is_read(string).
// Effect.cpp:295–308.
// Incomplete effect / nil key fails closed as true (no invent not-read).
// Empty name sticky true (restrictive — no invent not-read soft-skip past incomplete
// identifier query that would soft-match every empty-named var as non-hit).
func (e Effect) IsReadByName(name string) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as read (restrictive)
		SetError(ErrGeneric)
		return true
	}
	if name == "" {
		SetError(ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.read[v] && v.Name == name {
			return true
		}
	}
	return false
}

// IsWrittenByName mirrors Effect::is_written(string).
// Effect.cpp:351–364.
// Incomplete effect / nil key sticky true (no invent not-written soft re-pick).
// Empty name sticky true (restrictive — no invent not-written soft-skip past hole).
func (e Effect) IsWrittenByName(name string) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as written (restrictive)
		SetError(ErrGeneric)
		return true
	}
	if name == "" {
		SetError(ErrGeneric)
		return true
	}
	for v := range e.written {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if e.written[v] && v.Name == name {
			return true
		}
	}
	return false
}

// CommentOutput mirrors Effect::Output as a C block-comment line for Function::Output.
// Effect.cpp:507–529 — " * reads :" / " * writes:" lists.
// Write names sorted for deterministic emit (Go map iteration is random).
// Incomplete effect / nil key fails closed sticky as empty comment (no invent partial list).
func (e Effect) CommentOutput() string {
	if e.incomplete {
		SetError(ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString("/*\n")
	b.WriteString(" * reads :")
	rnames := make([]string, 0, len(e.read))
	for v := range e.read {
		// Effect.cpp: names from live Variable*; no invent blank tokens for empty Name
		if v == nil {
			SetError(ErrGeneric)
			return ""
		}
		if e.read[v] && v.Name != "" {
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
		if v == nil {
			SetError(ErrGeneric)
			return ""
		}
		if e.written[v] && v.Name != "" {
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
// Incomplete either arm fails closed sticky IncompleteEffect (no invent pure/empty-complete
// merge past incomplete map_stm_effect / accum holes — VisitFacts / generation would
// treat merged as success and poison parent accum as complete / soft re-pick).
func MergeEffects(a, b Effect) Effect {
	if !EffectComplete(a) || !EffectComplete(b) {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	out := Effect{
		pure:           a.pure && b.pure,
		sideEffectFree: a.sideEffectFree && b.sideEffectFree,
	}
	if len(a.written)+len(b.written) > 0 {
		out.written = make(map[*Variable]bool, len(a.written)+len(b.written))
		for k, v := range a.written {
			if k == nil {
				SetError(ErrGeneric)
				return IncompleteEffect()
			}
			if v {
				out.written[k] = true
			}
		}
		for k, v := range b.written {
			if k == nil {
				SetError(ErrGeneric)
				return IncompleteEffect()
			}
			if v {
				out.written[k] = true
			}
		}
	}
	if len(a.read)+len(b.read) > 0 {
		out.read = make(map[*Variable]bool, len(a.read)+len(b.read))
		for k, v := range a.read {
			if k == nil {
				SetError(ErrGeneric)
				return IncompleteEffect()
			}
			if v {
				out.read[k] = true
			}
		}
		for k, v := range b.read {
			if k == nil {
				SetError(ErrGeneric)
				return IncompleteEffect()
			}
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
