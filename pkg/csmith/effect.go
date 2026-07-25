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
	// readOrder / writeOrder preserve Effect::read_vars/write_vars insertion order
	// (C++ vector). Maps alone are unordered; ChooseOKVar index depends on order
	// after expand_struct_union_vars (seed-2 e18618 ok_vars n).
	readOrder  []*Variable
	writeOrder []*Variable
	// lhsWrite tracks LHS write vars (Effect::lhs_write_vars).
	// Effect.h:104 — used after assign visit to extend running context.
	lhsWrite map[*Variable]bool
}

// EmptyEffect is Effect::empty_effect (pure, side-effect free).
// Effect.cpp: Effect::Effect() : pure(true), side_effect_free(true)
func EmptyEffect() Effect {
	return Effect{pure: true, sideEffectFree: true}
}

// normalizeEmptyFlags mirrors C++ Effect::Effect() defaults for a value with no
// recorded reads/writes. Go's zero Effect has pure=false and sideEffectFree=false
// (bool zero), unlike C++ which default-constructs pure=true and side_effect_free=true
// (Effect.cpp:77). Function::accum_eff_context / feffect rely on that empty state;
// zero-value AccumEffContext poisoned BUILD revisit (seed-2 func_49 volatile writes
// rejected when effect_context was not SE-free / first_div e37241).
func (e Effect) normalizeEmptyFlags() Effect {
	if e.incomplete {
		return e
	}
	if len(e.readOrder) == 0 && len(e.writeOrder) == 0 {
		e.pure = true
		e.sideEffectFree = true
	}
	return e
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

func (e Effect) CloneSess(s *Session) Effect {
	if e.incomplete {
		// residual ERROR sticky — no invent soft-complete clone past IncompleteEffect hole
		// (snapshot restore paths must HasError/EffectComplete fail closed)
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	return e.detachMaps()
}

// detachMaps deep-copies maps/slices without touching HasError.
// Used by MapAccumEffect store/load when ERROR may already be sticky from visit_facts.
func (e Effect) detachMaps() Effect {
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
	if len(e.readOrder) > 0 {
		out.readOrder = append([]*Variable(nil), e.readOrder...)
	}
	if len(e.written) > 0 {
		out.written = make(map[*Variable]bool, len(e.written))
		for k, v := range e.written {
			out.written[k] = v
		}
	}
	if len(e.writeOrder) > 0 {
		out.writeOrder = append([]*Variable(nil), e.writeOrder...)
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

func (e Effect) IsPureSess(s *Session) bool {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return e.pure
}

// IsSideEffectFree mirrors Effect::is_side_effect_free.
// Incomplete effects sticky not SE-free (no invent SE-free success / soft re-pick past holes).

func (e Effect) IsSideEffectFreeSess(s *Session) bool {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
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

func (e Effect) AddExternalEffectSess(s *Session, other Effect) Effect {
	return e.AddExternalEffectWithCallersSess(s, other, nil)
}

// AddExternalEffectWithCallers mirrors Effect::add_external_effect(e, call_chain).
// Effect.cpp:221–269 — globals always; non-globals only if on a call_chain stack frame.
// Variable* always live in effect lists; nil hole fails closed sticky IncompleteEffect
// (no invent partial external merge past holes / soft re-pick). Incomplete Param/LocalVars
// on a frame also fails closed sticky (no invent not-on-chain via IsVarOnStack false past hole).

func (e Effect) AddExternalEffectWithCallersSess(s *Session, other Effect, callChain []*Block) Effect {
	// incomplete effect maps / call chain fail closed sticky IncompleteEffect
	// (no invent leave-base empty-complete success)
	if !EffectComplete(e) || !EffectComplete(other) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	reads := other.ReadVarsSess(s)
	writes := other.WrittenVarsSess(s)
	if !VariablesComplete(reads) || !VariablesComplete(writes) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	if !BlocksComplete(callChain) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	for _, b := range callChain {
		if !b.StackScanCompleteSess(s) {
			// residual ERROR sticky — no invent soft-merge past StackScan residual
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteEffect()
		}
	}
	out := e.normalizeEmptyFlags()
	for _, v := range reads {
		if v.IsGlobalSess(s) {
			// residual ERROR sticky — no invent soft-continue merge past IsGlobal hole
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out = out.ReadVarSess(s, v)
			// residual ERROR sticky — no invent soft-continue later reads past ReadVar residual
			if sessHasError(s) {
				return IncompleteEffect()
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue merge past IsGlobal residual false
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if varOnCallChainSess(s, v, callChain) {
			// residual ERROR sticky — no invent soft-skip past IsVarOnStack hole
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out = out.ReadVarSess(s, v)
			// residual ERROR sticky — no invent soft-continue later reads past ReadVar residual
			if sessHasError(s) {
				return IncompleteEffect()
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent not-on-chain soft-skip past hole
			return IncompleteEffect()
		}
	}
	for _, v := range writes {
		if v.IsGlobalSess(s) {
			// residual ERROR sticky — no invent soft-continue merge past IsGlobal hole
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out = out.WriteVarSess(s, v)
			// residual ERROR sticky — no invent soft-continue later writes past WriteVar residual
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out.pure = false
			continue
		}
		// residual ERROR sticky — no invent soft-continue merge past IsGlobal residual false
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if varOnCallChainSess(s, v, callChain) {
			// residual ERROR sticky — no invent soft-skip past IsVarOnStack hole
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out = out.WriteVarSess(s, v)
			// residual ERROR sticky — no invent soft-continue later writes past WriteVar residual
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out.pure = false
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent not-on-chain soft-skip past hole
			return IncompleteEffect()
		}
	}
	out.sideEffectFree = out.sideEffectFree && other.sideEffectFree
	return out
}

func varOnCallChainSess(s *Session, v *Variable, chain []*Block) bool {
	for _, b := range chain {
		// chain pre-validated complete at AddExternalEffectWithCallers
		if b == nil {
			continue
		}
		if !b.StackScanCompleteSess(s) {
			// incomplete stack sticky not-on-chain residual for caller IncompleteEffect
			sessNoteError(s, ErrGeneric)
			return false
		}
		if b.IsVarOnStackSess(s, v) {
			// residual ERROR sticky — no invent on-chain true past IsVarOnStack hole
			if sessHasError(s) {
				return false
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue not-on-chain past IsVarOnStack residual
		if sessHasError(s) {
			return false
		}
	}
	return false
}

// AddEffect mirrors Effect::add_effect(e) without LHS set merge.
// Effect.cpp:157–186.

func (e Effect) AddEffectSess(s *Session, other Effect) Effect {
	return e.AddEffectOptsSess(s, other, false)
}

// AddEffectOpts mirrors Effect::add_effect(e, include_lhs_effects).
// Effect.cpp:167–180 — push each of other's reads/writes only if !is_read /
// !is_written on the destination (struct-parent coverage applies).
// Variable* always live as map keys; incomplete either side fails closed sticky
// IncompleteEffect (no invent leave-base empty-complete merge / soft re-pick past hole).

func (e Effect) AddEffectOptsSess(s *Session, other Effect, includeLHS bool) Effect {
	if !EffectComplete(e) || !EffectComplete(other) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	// Effect.cpp:167–172 — for each other read: if (!is_read) push
	reads := other.ReadVarsSess(s)
	if !VariablesComplete(reads) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	for _, v := range reads {
		// Effect.cpp:169–171 — if (!is_read) push only; purity via pure &= e.pure below
		already := out.IsReadSess(s, v)
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if already {
			continue
		}
		nr := make(map[*Variable]bool, len(out.read)+1)
		for k, val := range out.read {
			nr[k] = val
		}
		nr[v] = true
		out.read = nr
		out.readOrder = append(append([]*Variable(nil), out.readOrder...), v)
	}
	// Effect.cpp:174–179 — for each other write: if (!is_written) push
	writes := other.WrittenVarsSess(s)
	if !VariablesComplete(writes) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	for _, v := range writes {
		already := out.IsWrittenSess(s, v)
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if already {
			continue
		}
		nw := make(map[*Variable]bool, len(out.written)+1)
		for k, val := range out.written {
			nw[k] = val
		}
		nw[v] = true
		out.written = nw
		out.writeOrder = append(append([]*Variable(nil), out.writeOrder...), v)
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

func (e Effect) WriteVarSetSess(s *Session, vars []*Variable) Effect {
	if !EffectComplete(e) || !VariablesComplete(vars) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	for _, v := range vars {
		out = out.WriteVarSess(s, v)
		// residual ERROR sticky — no invent soft-continue later writes past WriteVar residual
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if !EffectComplete(out) {
			sessNoteError(s, ErrGeneric)
			return IncompleteEffect()
		}
	}
	return out
}

// SetLhsWriteVars mirrors Effect::set_lhs_write_vars from current write_vars.
// Lhs.cpp:348–351 — after successful LHS visit.
// Incomplete write map fails closed sticky IncompleteEffect (no invent empty lhsWrite).

func (e Effect) SetLhsWriteVarsFromWrittenSess(s *Session) Effect {
	if !EffectComplete(e) {
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
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

func (e Effect) LhsWriteVarsSess(s *Session) []*Variable {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// WriteVar mirrors Effect::write_var.
// Effect.cpp:137–146 — push only if !is_written(v); SE-free &= !volatile &&
// !access_once; pure left unchanged (upstream "pure = pure").
// Incomplete base / nil Variable sticky IncompleteEffect (no invent grow write map
// / soft-skip identity past holes).

func (e Effect) WriteVarSess(s *Session, v *Variable) Effect {
	// Variable always live; sticky incomplete no invent identity no-op past hole
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	// Effect.cpp:138–140 — only push if !is_written(v) (parent write covers fields)
	already := e.IsWrittenSess(s, v)
	// residual ERROR sticky — no invent grow write map past IsWritten hole
	if sessHasError(s) {
		return IncompleteEffect()
	}
	if !already {
		// copy-on-write for value semantics
		nw := make(map[*Variable]bool, len(e.written)+1)
		for k, val := range e.written {
			nw[k] = val
		}
		nw[v] = true
		e.written = nw
		e.writeOrder = append(append([]*Variable(nil), e.writeOrder...), v)
	}
	// Effect.cpp:144–145 — SE-free means volatile/access_once free (always apply)
	if v.IsVolatileSess(s) || v.IsAccessOnce {
		// residual ERROR sticky — no invent complete write map past IsVolatile residual
		if sessHasError(s) {
			return IncompleteEffect()
		}
		e.sideEffectFree = false
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent complete write map past IsVolatile residual false
		return IncompleteEffect()
	}
	return e
}

// ReadVar mirrors Effect::read_var.
// Effect.cpp:116–122 — push only if !is_read(v); pure &= const&&!vol&&!access_once;
// SE-free &= !vol&&!access_once. is_read covers struct parent → field, so a field
// is not re-pushed after its parent struct was already recorded (expand_struct would
// otherwise duplicate field entries in choose_visible_read_var pools).
// Incomplete base / nil Variable sticky IncompleteEffect (no invent grow read map
// / soft-skip identity past holes).

func (e Effect) ReadVarSess(s *Session, v *Variable) Effect {
	// Variable always live; sticky incomplete no invent identity no-op past hole
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	// Effect.cpp:117–119 — only push if !is_read(v)
	already := e.IsReadSess(s, v)
	// residual ERROR sticky — no invent grow read map past IsRead hole
	if sessHasError(s) {
		return IncompleteEffect()
	}
	if !already {
		nr := make(map[*Variable]bool, len(e.read)+1)
		for k, val := range e.read {
			nr[k] = val
		}
		nr[v] = true
		e.read = nr
		// C++ vector push_back — preserve insertion order for get_read_vars
		e.readOrder = append(append([]*Variable(nil), e.readOrder...), v)
	}
	// Effect.cpp:120–121 — purity always updated even when already is_read
	isConst := v.IsConstSess(s)
	// residual ERROR sticky — no invent complete read map past IsConst residual
	if sessHasError(s) {
		return IncompleteEffect()
	}
	isVol := v.IsVolatileSess(s)
	// residual ERROR sticky — no invent complete read map past IsVolatile residual
	if sessHasError(s) {
		return IncompleteEffect()
	}
	if !(isConst && !isVol && !v.IsAccessOnce) {
		e.pure = false
	}
	if isVol || v.IsAccessOnce {
		e.sideEffectFree = false
	}
	return e
}

// IsWritten mirrors Effect::is_written — exact or parent field_var_of.
// Effect.cpp:333–345.
// Incomplete effect sticky true (no invent not-written / conflict-free past holes).
// Type-nil parent sticky written true (restrictive — no invent not-written /
// conflict-free soft-skip past incomplete parent type shell; mirrors IsRead).

func (e Effect) IsWrittenSess(s *Session, v *Variable) bool {
	// Variable always live; sticky incomplete no invent not-written soft-skip
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as written (restrictive)
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return true
		}
		ok := e.IsWrittenSess(s, v.FieldVarOf)
		// residual ERROR sticky — no invent not-written soft-skip past nested IsWritten hole
		if sessHasError(s) {
			return true
		}
		return ok
	}
	return false
}

// ReadVars mirrors Effect::get_read_vars — insertion order (C++ vector).
// Incomplete effect / nil map keys fail closed sticky IncompleteVariables
// (no invent skip hole as absent read / soft re-pick empty-complete read set).

func (e Effect) ReadVarsSess(s *Session) []*Variable {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return IncompleteVariables()
	}
	if len(e.readOrder) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.readOrder))
	for _, v := range e.readOrder {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	return out
}

// WrittenVars mirrors Effect::get_write_vars — insertion order (C++ vector).
// Incomplete effect / nil map keys fail closed sticky IncompleteVariables
// (no invent skip hole as absent write / soft re-pick empty-complete write set).

func (e Effect) WrittenVarsSess(s *Session) []*Variable {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return IncompleteVariables()
	}
	if len(e.writeOrder) == 0 {
		return nil
	}
	out := make([]*Variable, 0, len(e.writeOrder))
	for _, v := range e.writeOrder {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteVariables()
		}
		out = append(out, v)
	}
	return out
}

// IsRead mirrors Effect::is_read — exact or struct parent (not union).
// Effect.cpp:276–289.
// Incomplete effect sticky true (no invent not-read / conflict-free past holes).

func (e Effect) IsReadSess(s *Session, v *Variable) bool {
	// Variable always live; sticky incomplete no invent not-read soft-skip
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as read (restrictive)
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return true
		}
		if v.FieldVarOf.Type.IsStructSess(s) {
			// residual ERROR sticky — no invent soft-continue struct-read past IsStruct residual
			if sessHasError(s) {
				return true
			}
			ok := e.IsReadSess(s, v.FieldVarOf)
			// residual ERROR sticky — no invent not-read soft-skip past nested IsRead hole
			if sessHasError(s) {
				return true
			}
			return ok
		}
		// residual ERROR sticky — no invent not-read soft-skip past IsStruct residual false
		if sessHasError(s) {
			return true
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

func (e Effect) FieldIsReadSess(s *Session, v *Variable) bool {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// Type* always live for non-special subjects; Type-nil sticky true
	// (no invent IsAggregate residual false as no-field-read past shell)
	if !IsSpecialPtr(v) && v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !v.IsAggregateSess(s) {
		// residual ERROR sticky — no invent no-field-read soft-skip past IsAggregate residual
		if sessHasError(s) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue field scan past IsAggregate residual true
	if sessHasError(s) {
		return true
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as field-read (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	for _, f := range v.FieldVars {
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.IsReadSess(s, f) {
			// residual ERROR sticky — no invent field-read true past IsRead hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue nested past IsRead residual false
		if sessHasError(s) {
			return true
		}
		if e.FieldIsReadSess(s, f) {
			// residual ERROR sticky — no invent field-read true past nested FieldIsRead hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual
		if sessHasError(s) {
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

func (e Effect) FieldIsWrittenSess(s *Session, v *Variable) bool {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// Type* always live for non-special subjects; Type-nil sticky true
	// (no invent IsAggregate residual false as no-field-write past shell)
	if !IsSpecialPtr(v) && v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !v.IsAggregateSess(s) {
		// residual ERROR sticky — no invent no-field-write soft-skip past IsAggregate residual
		if sessHasError(s) {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue field scan past IsAggregate residual true
	if sessHasError(s) {
		return true
	}
	if e.incomplete {
		// IncompleteEffect sticky fail closed as field-written (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	for _, f := range v.FieldVars {
		if f == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.IsWrittenSess(s, f) {
			// residual ERROR sticky — no invent field-written true past IsWritten hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue nested past IsWritten residual false
		if sessHasError(s) {
			return true
		}
		if e.FieldIsWrittenSess(s, f) {
			// residual ERROR sticky — no invent field-written true past nested FieldIsWritten hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue later fields past nested residual
		if sessHasError(s) {
			return true
		}
	}
	return false
}

// ancestryTypeHole reports Type-nil on FieldVarOf chain (excluding specials).
// Used by SiblingUnionField* so residual global HasError cannot invent sibling-use.}

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

func (e Effect) SiblingUnionFieldIsReadSess(s *Session, v *Variable) bool {
	// Variable always live; sticky incomplete no invent no-sibling soft-skip
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return true
	}
	youColl := v.GetCollectiveSess(s)
	// residual ERROR sticky — no invent soft no-sibling past GetCollective residual
	if sessHasError(s) {
		return true
	}
	if youColl == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	you := youColl.GetContainerUnionSess(s)
	// residual ERROR sticky — no invent soft no-sibling past GetContainerUnion residual
	if sessHasError(s) {
		return true
	}
	if you == nil {
		// Type-nil ancestry (not residual global HasError): restrictive true
		// (no invent no-sibling / conflict-free past incomplete container shell)
		if ancestryTypeHole(youColl) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return true
		}
		// complete no container union
		return false
	}
	for r := range e.read {
		if r == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if !e.read[r] {
			continue
		}
		rColl := r.GetCollectiveSess(s)
		// residual ERROR sticky — no invent soft-continue later reads past GetCollective residual
		if sessHasError(s) {
			return true
		}
		if rColl == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		me := rColl.GetContainerUnionSess(s)
		// residual ERROR sticky — no invent soft-continue no-sibling past GetContainerUnion hole
		if sessHasError(s) {
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

func (e Effect) SiblingUnionFieldIsWrittenSess(s *Session, v *Variable) bool {
	// Variable always live; sticky incomplete no invent no-sibling soft-skip
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return true
	}
	youColl := v.GetCollectiveSess(s)
	// residual ERROR sticky — no invent soft no-sibling past GetCollective residual
	if sessHasError(s) {
		return true
	}
	if youColl == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	you := youColl.GetContainerUnionSess(s)
	// residual ERROR sticky — no invent soft no-sibling past GetContainerUnion residual
	if sessHasError(s) {
		return true
	}
	if you == nil {
		// Type-nil ancestry (not residual global HasError): restrictive true
		// (no invent no-sibling / conflict-free past incomplete container shell)
		if ancestryTypeHole(youColl) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return true
		}
		// complete no container union
		return false
	}
	for w := range e.written {
		if w == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if !e.written[w] {
			continue
		}
		wColl := w.GetCollectiveSess(s)
		// residual ERROR sticky — no invent soft-continue later writes past GetCollective residual
		if sessHasError(s) {
			return true
		}
		if wColl == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		me := wColl.GetContainerUnionSess(s)
		// residual ERROR sticky — no invent soft-continue no-sibling past GetContainerUnion hole
		if sessHasError(s) {
			return true
		}
		if me == you {
			return true
		}
	}
	return false
}

// NonEmptyIntersectionSess mirrors Effect.cpp non_empty_intersection.
// Effect.cpp:56–69 — any pair where va[i]->match(vb[j]) || vb[j]->match(va[i]).
// Nil entries in either list fail closed sticky true (restrictive race).
// Non-Sess dual deleted — pass run bag or testAmbientSession explicitly.
func NonEmptyIntersectionSess(s *Session, va, vb []*Variable) bool {
	for _, a := range va {
		for _, b := range vb {
			if a == nil || b == nil {
				sessNoteError(s, ErrGeneric)
				return true
			}
			if a.MatchSess(s, b) {
				if sessHasError(s) {
					return true
				}
				return true
			}
			if sessHasError(s) {
				return true
			}
			if b.MatchSess(s, a) {
				if sessHasError(s) {
					return true
				}
				return true
			}
			if sessHasError(s) {
				return true
			}
		}
	}
	return false
}

// HasRaceWith mirrors Effect::has_race_with.
// Effect.cpp:480–484 — read∩write || write∩read || write∩write via non_empty_intersection.
// Incomplete either side fails closed sticky as race (no invent race-free / soft re-pick).

func (e Effect) HasRaceWithSess(s *Session, other Effect) bool {
	if !EffectComplete(e) || !EffectComplete(other) {
		sessNoteError(s, ErrGeneric)
		return true
	}
	ra, wa := e.ReadVarsSess(s), e.WrittenVarsSess(s)
	if sessHasError(s) {
		return true
	}
	rb, wb := other.ReadVarsSess(s), other.WrittenVarsSess(s)
	if sessHasError(s) {
		return true
	}
	if NonEmptyIntersectionSess(s, ra, wb) {
		return true
	}
	if sessHasError(s) {
		return true
	}
	if NonEmptyIntersectionSess(s, wa, rb) {
		return true
	}
	if sessHasError(s) {
		return true
	}
	if NonEmptyIntersectionSess(s, wa, wb) {
		return true
	}
	return sessHasError(s)
}

// IsEmpty mirrors Effect::is_empty — no reads and no writes.
// Effect.cpp:490–492.
// Incomplete sticky not-empty (no invent empty-complete free effect past holes).

func (e Effect) IsEmptySess(s *Session) bool {
	if e.incomplete {
		// IncompleteEffect sticky not-empty (restrictive — no invent empty-complete)
		sessNoteError(s, ErrGeneric)
		return false
	}
	return len(e.ReadVarsSess(s)) == 0 && len(e.WrittenVarsSess(s)) == 0
}

// Clear mirrors Effect::clear — empty pure SE-free effect.
// Effect.cpp:497–501.
// Effect* always live at clear; sticky (no invent soft-skip clear past hole).
// Incomplete base stays IncompleteEffect sticky (no invent wipe hole shell to empty pure
// / soft re-pick past incomplete clear).

func (e *Effect) ClearSess(s *Session) {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if e.incomplete {
		*e = IncompleteEffect()
		sessNoteError(s, ErrGeneric)
		return
	}
	*e = EmptyEffect()
}

// AccessDerefVolatile mirrors Effect::access_deref_volatile.
// Effect.cpp:124–135 — under strict_volatile_rule, clear SE-free if volatile after deref.
// Incomplete base fails closed sticky IncompleteEffect (no invent SE-free tweak on hole shell).
// Under strictVolatile, Variable always live; sticky IncompleteEffect (no invent SE-free skip).

func (e Effect) AccessDerefVolatileSess(s *Session, v *Variable, derefLevel int, strictVolatile bool) Effect {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	if !strictVolatile {
		return e
	}
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	out := e
	level := derefLevel
	for level > 0 {
		if v.IsVolatileAfterDerefSess(s, level) {
			// residual ERROR sticky — no invent complete SE-free tweak past residual hole
			if sessHasError(s) {
				return IncompleteEffect()
			}
			out.sideEffectFree = false
			return out
		}
		// residual ERROR sticky — no invent soft-continue peel past residual false path
		if sessHasError(s) {
			return IncompleteEffect()
		}
		level--
	}
	return out
}

// IsReadPartially mirrors Effect::is_read_partially.
// Effect.cpp:444–446.
// Incomplete effect fails closed as true via IsRead/FieldIs*/Sibling* membership.

func (e Effect) IsReadPartiallySess(s *Session, v *Variable) bool {
	if e.IsReadSess(s, v) {
		// residual ERROR sticky — no invent partial-read true past IsRead hole
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue partial past IsRead residual false
	if sessHasError(s) {
		return true
	}
	if e.FieldIsReadSess(s, v) {
		if sessHasError(s) {
			return true
		}
		return true
	}
	if sessHasError(s) {
		return true
	}
	if e.SiblingUnionFieldIsReadSess(s, v) {
		if sessHasError(s) {
			return true
		}
		return true
	}
	if sessHasError(s) {
		return true
	}
	return false
}

// IsWrittenPartially mirrors Effect::is_written_partially.
// Effect.cpp:448–451.
// Incomplete effect fails closed as true via IsWritten/FieldIs*/Sibling* membership.}

func (e Effect) IsWrittenPartiallySess(s *Session, v *Variable) bool {
	if e.IsWrittenSess(s, v) {
		// residual ERROR sticky — no invent partial-write true past IsWritten hole
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue partial past IsWritten residual false
	if sessHasError(s) {
		return true
	}
	if e.FieldIsWrittenSess(s, v) {
		if sessHasError(s) {
			return true
		}
		return true
	}
	if sessHasError(s) {
		return true
	}
	if e.SiblingUnionFieldIsWrittenSess(s, v) {
		if sessHasError(s) {
			return true
		}
		return true
	}
	if sessHasError(s) {
		return true
	}
	return false
}

// HasGlobalEffect mirrors Effect::has_global_effect.
// Effect.cpp:543–562 — any global in read or write sets.
// Incomplete effect / nil map keys sticky true (no invent pure / no-global success).

func (e Effect) HasGlobalEffectSess(s *Session) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as has-global (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.read[v] && v.IsGlobalSess(s) {
			// residual ERROR sticky — no invent has-global true past IsGlobal hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-global past IsGlobal residual
		if sessHasError(s) {
			return true
		}
	}
	for v := range e.written {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.written[v] && v.IsGlobalSess(s) {
			// residual ERROR sticky — no invent has-global true past IsGlobal hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-global past IsGlobal residual
		if sessHasError(s) {
			return true
		}
	}
	return false
}

// UpdatePurity mirrors Effect::update_purity.
// Effect.cpp:535–538 — pure cleared when has_global_effect.
// Effect always live; sticky (no invent soft-skip purity update past hole).

func (e *Effect) UpdatePuritySess(s *Session) {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if e.HasGlobalEffectSess(s) {
		e.pure = false
	}
}

// UnionFieldIsRead mirrors Effect::union_field_is_read.
// Effect.cpp:565–572 — any read var is_inside_union_field.
// Incomplete effect / nil key fails closed as true (no invent none on
// IncompleteEffect empty maps).

func (e Effect) UnionFieldIsReadSess(s *Session) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as union-field-read (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.read[v] && v.IsInsideUnionFieldSess(s) {
			// residual ERROR sticky — no invent union-read true past IsInsideUnionField hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue no-union-read past IsInside residual
		if sessHasError(s) {
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

func (e *Effect) ConsolidateSess(s *Session) {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if e.incomplete || !effectMapKeysComplete(e.read) || !effectMapKeysComplete(e.written) {
		*e = IncompleteEffect()
		sessNoteError(s, ErrGeneric)
		return
	}
	// remove field reads when parent is also read
	for v := range e.read {
		if !e.read[v] {
			continue
		}
		if !v.IsFieldVarSess(s) {
			// residual ERROR sticky — no invent soft-skip consolidate past IsFieldVar hole
			if sessHasError(s) {
				*e = IncompleteEffect()
				return
			}
			continue
		}
		parent := v.FieldVarOf
		// FieldVarOf always live for field vars; Type* always live for non-special parents
		// Type-nil parent sticky IncompleteEffect (no invent leave-base complete past hole)
		if parent == nil {
			sessNoteError(s, ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if parent.Type == nil && !IsSpecialPtr(parent) {
			sessNoteError(s, ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if e.IsReadSess(s, parent) {
			// residual ERROR sticky — no invent soft-delete past IsRead hole
			if sessHasError(s) {
				*e = IncompleteEffect()
				return
			}
			delete(e.read, v)
			// keep readOrder in sync with map
			for i, x := range e.readOrder {
				if x == v {
					e.readOrder = append(e.readOrder[:i], e.readOrder[i+1:]...)
					break
				}
			}
		} else if sessHasError(s) {
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
		if !v.IsFieldVarSess(s) {
			// residual ERROR sticky — no invent soft-skip consolidate past IsFieldVar hole
			if sessHasError(s) {
				*e = IncompleteEffect()
				return
			}
			continue
		}
		parent := v.FieldVarOf
		if parent == nil {
			sessNoteError(s, ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if parent.Type == nil && !IsSpecialPtr(parent) {
			sessNoteError(s, ErrGeneric)
			*e = IncompleteEffect()
			return
		}
		if e.IsWrittenSess(s, parent) {
			// residual ERROR sticky — no invent soft-delete past IsWritten hole
			if sessHasError(s) {
				*e = IncompleteEffect()
				return
			}
			delete(e.written, v)
			for i, x := range e.writeOrder {
				if x == v {
					e.writeOrder = append(e.writeOrder[:i], e.writeOrder[i+1:]...)
					break
				}
			}
		} else if sessHasError(s) {
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

func (e Effect) IsReadByNameSess(s *Session, name string) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as read (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return true
	}
	for v := range e.read {
		if v == nil {
			sessNoteError(s, ErrGeneric)
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

func (e Effect) IsWrittenByNameSess(s *Session, name string) bool {
	if e.incomplete {
		// IncompleteEffect sticky fail closed as written (restrictive)
		sessNoteError(s, ErrGeneric)
		return true
	}
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return true
	}
	for v := range e.written {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if e.written[v] && v.Name == name {
			return true
		}
	}
	return false
}

// CommentOutput mirrors Effect::Output + OutputMgr::output_comment_line.
// Effect.cpp:507–529 — insertion-order read_vars/write_vars via OutputForComment;
// then output_comment_line wraps "/* " + body + " */\n" (OutputMgr.cpp:314–320).
// Incomplete effect / nil var fails closed sticky empty (no invent partial list).

func (e Effect) CommentOutputSess(s *Session) string {
	return e.CommentOutputOptsSess(s, false, false)
}

// CommentOutputOptsSess is CommentOutput with quiet/concise for the wrap line
// (OutputMgr::output_comment_line: quiet|concise → blank only).
func (e Effect) CommentOutputOptsSess(s *Session, quiet, concise bool) string {
	if e.incomplete {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// nil map keys incomplete sticky (no invent skip as absent / partial comment)
	for v := range e.read {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	for v := range e.written {
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	// Effect.cpp:512–527 — body begins with newline then " * reads :" / " * writes:"
	var ss strings.Builder
	ss.WriteString("\n")
	ss.WriteString(" * reads :")
	reads := e.ReadVarsSess(s)
	// residual ERROR sticky — no invent soft-list past ReadVars residual
	if sessHasError(s) {
		return ""
	}
	if !VariablesComplete(reads) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	for _, v := range reads {
		// Effect.cpp:518 — Variable::OutputForComment → get_actual_name()
		name := v.OutputForCommentSess(s, false)
		if sessHasError(s) {
			return ""
		}
		if name == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		ss.WriteString(" ")
		ss.WriteString(name)
	}
	ss.WriteString("\n")
	ss.WriteString(" * writes:")
	writes := e.WrittenVarsSess(s)
	if sessHasError(s) {
		return ""
	}
	if !VariablesComplete(writes) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	for _, v := range writes {
		name := v.OutputForCommentSess(s, false)
		if sessHasError(s) {
			return ""
		}
		if name == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		ss.WriteString(" ")
		ss.WriteString(name)
	}
	ss.WriteString("\n")
	// OutputMgr.cpp:314–320 — quiet|concise → blank line only
	return OutputCommentLineSess(s, ss.String(), quiet, concise)
}

// MergeEffects combines two post-branch effects (union of reads/writes; SE-free only if both are).
// Approximates StatementIf fact/effect merge without FactMgr.
// Incomplete either arm fails closed sticky IncompleteEffect (no invent pure/empty-complete
// merge past incomplete map_stm_effect / accum holes — VisitFacts / generation would
// treat merged as success and poison parent accum as complete / soft re-pick).
//
// Uses is_read/is_written gates and preserves a-then-b insertion order (fair with
// C++ vector merges that skip already-covered vars).
// Non-Sess dual deleted — pass run bag or testAmbientSession explicitly.

func MergeEffectsSess(s *Session, a, b Effect) Effect {
	if !EffectComplete(a) || !EffectComplete(b) {
		sessNoteError(s, ErrGeneric)
		return IncompleteEffect()
	}
	out := Effect{
		pure:           a.pure && b.pure,
		sideEffectFree: a.sideEffectFree && b.sideEffectFree,
	}
	// a first, then b — push if not already is_read/is_written on out
	for _, v := range a.ReadVarsSess(s) {
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.IsReadSess(s, v) {
			if sessHasError(s) {
				return IncompleteEffect()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.read == nil {
			out.read = make(map[*Variable]bool)
		}
		out.read[v] = true
		out.readOrder = append(out.readOrder, v)
	}
	for _, v := range b.ReadVarsSess(s) {
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.IsReadSess(s, v) {
			if sessHasError(s) {
				return IncompleteEffect()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.read == nil {
			out.read = make(map[*Variable]bool)
		}
		out.read[v] = true
		out.readOrder = append(out.readOrder, v)
	}
	for _, v := range a.WrittenVarsSess(s) {
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.IsWrittenSess(s, v) {
			if sessHasError(s) {
				return IncompleteEffect()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.written == nil {
			out.written = make(map[*Variable]bool)
		}
		out.written[v] = true
		out.writeOrder = append(out.writeOrder, v)
	}
	for _, v := range b.WrittenVarsSess(s) {
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.IsWrittenSess(s, v) {
			if sessHasError(s) {
				return IncompleteEffect()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteEffect()
		}
		if out.written == nil {
			out.written = make(map[*Variable]bool)
		}
		out.written[v] = true
		out.writeOrder = append(out.writeOrder, v)
	}
	// Effect.cpp:add_effect — pure &= e.pure; side_effect_free &= e.side_effect_free
	// already set at construction. write_var only clears SE-free for volatile /
	// access_once (Effect.cpp:144), not every write. Invent soft-false SE-free here
	// poisoned revisit EffectAccum → AddVisibleEffect → binary RHS ambient half-size
	// ok pools (seed-7 e58205 ChooseOKVar n=26 vs UP n=56).
	return out
}
