// Upstream: CGContext.h / CGContext.cpp (empty context + effect_context accessors).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"os"
	"strings"
)

// RWDirective mirrors CGContext.h RWDirective — must/no read/write sets.
// Directs VariableSelector without changing language semantics alone.
type RWDirective struct {
	NoReadVars    []*Variable
	NoWriteVars   []*Variable
	MustReadVars  []*Variable
	MustWriteVars []*Variable
}

// CGContext is a minimal CGContext for paths that only need effect_context.
// Function/block/fact fields land with later ports.
type CGContext struct {
	effectContext Effect
	// CurrentFunc mirrors current_func_ (Function*).
	CurrentFunc *Function
	// BlkDepth mirrors blk_depth.
	BlkDepth int
	// Flags mirrors CGContext flags (IN_LOOP etc.).
	Flags uint
	// ExprDepth mirrors expr_depth.
	ExprDepth int
	// Funcs is the session FuncList for choose_func / create.
	Funcs *FunctionList
	// Types is session derived_types.
	Types *TypeEnv
	// MustUseArrays mirrors rw_directive must-use arrays for array-loop for-control.
	// StatementFor::make_iteration when find_must_use_arrays nonempty.
	MustUseArrays []*ArrayVariable
	// EffectAccum is optional mutable effect (Effect::effect_accum) for write tracking.
	EffectAccum *Effect
	// EffectStm mirrors effect_stm_ — per-statement effect (cleared before new stmts).
	EffectStm Effect
	// FM is optional FactMgr for the current function (get_fact_mgr).
	FM *FactMgr
	// RW mirrors rw_directive (optional).
	RW *RWDirective
	// IVBounds mirrors iv_bounds — loop induction variables must not be written.
	// Value is bound (unused for eligibility; presence matters).
	IVBounds map[*Variable]int
	// CallChain mirrors call_chain — caller blocks for external effect tracking.
	CallChain []*Block
	// CurrRHS mirrors curr_rhs — RHS expression when validating LHS (assign).
	// CGContext.h:178.
	CurrRHS *Expression
}

// EmptyCGContext mirrors CGContext::get_empty_context() (empty effect context).
func EmptyCGContext() CGContext {
	return CGContext{effectContext: EmptyEffect()}
}

// EffectContext mirrors CGContext::get_effect_context.
// CGContext.h:118 — ambient restriction context (not the accum).
func (c CGContext) EffectContext() Effect {
	return c.effectContext
}

// AccumEffect mirrors CGContext::get_accum_effect.
// CGContext.h:121–123 — *effect_accum if set, else empty.
func (c CGContext) AccumEffect() Effect {
	if c.EffectAccum != nil {
		return *c.EffectAccum
	}
	return EmptyEffect()
}

// NoteWrite records a variable write into EffectAccum if present.
// Also updates CurrentFunc.FEffect for globals (Function::feffect external).
// Variable always live; sticky (no invent soft-skip write past hole).
// Incomplete accum fails closed sticky error (no invent silent grow on hole shell).
func (c CGContext) NoteWrite(v *Variable) {
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
		*c.EffectAccum = c.EffectAccum.WriteVar(v)
		// residual ERROR sticky — no invent soft-continue FEffect past WriteVar residual
		if HasError() {
			return
		}
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual hole
		if HasError() {
			return
		}
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.WriteVar(v)
		// residual ERROR sticky — no invent soft-complete write past FEffect WriteVar residual
		if HasError() {
			return
		}
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
		}
	} else if HasError() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual false
		return
	}
}

// NoteRead records a variable read into EffectAccum and FEffect for globals.
// Variable always live; sticky (no invent soft-skip read past hole).
// Incomplete accum fails closed sticky error (no invent silent grow on hole shell).
func (c CGContext) NoteRead(v *Variable) {
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
		*c.EffectAccum = c.EffectAccum.ReadVar(v)
		// residual ERROR sticky — no invent soft-continue FEffect past ReadVar residual
		if HasError() {
			return
		}
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual hole
		if HasError() {
			return
		}
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.ReadVar(v)
		// residual ERROR sticky — no invent soft-complete read past FEffect ReadVar residual
		if HasError() {
			return
		}
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
		}
	} else if HasError() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual false
		return
	}
}

// WithEffectContext returns a context with the given effect_context.
func WithEffectContext(eff Effect) CGContext {
	return CGContext{effectContext: eff}
}

// WithFunc returns a context for generating inside f.
func WithFunc(f *Function, eff Effect) CGContext {
	return CGContext{effectContext: eff, CurrentFunc: f}
}

// WithFuncList attaches the session function list.
func (c CGContext) WithFuncList(list *FunctionList) CGContext {
	c.Funcs = list
	return c
}

// WithFactMgr attaches a FactMgr (get_fact_mgr path).
func (c CGContext) WithFactMgr(fm *FactMgr) CGContext {
	c.FM = fm
	return c
}

// CurrentBlock mirrors CGContext::get_current_block — top of function stack.
func (c CGContext) CurrentBlock() *Block {
	if c.CurrentFunc == nil || len(c.CurrentFunc.Stack) == 0 {
		return nil
	}
	return c.CurrentFunc.Stack[len(c.CurrentFunc.Stack)-1]
}

// CGContext flag bits (CGContext.h).
const (
	// FlagInLoop is IN_LOOP.
	FlagInLoop uint = 2
	// FlagNoDanglingPtr is NO_DANGLING_PTR.
	FlagNoDanglingPtr uint = 8
)

// Variable scope codes (CGContext.h).
const (
	// ScopeGlobal is find_variable_scope for globals (-1).
	ScopeGlobalVar = -1
	// ScopeInvisible is INVISIBLE — on a caller stack frame.
	ScopeInvisible = 9999
	// ScopeInactive is INACTIVE — not found.
	ScopeInactive = 8888
)

// FindVariableScope mirrors CGContext::find_variable_scope.
// CGContext.cpp:431–468 — -1 global; 0 param; 1+ block depth; INVISIBLE/INACTIVE.
func (c CGContext) FindVariableScope(v *Variable) int {
	// Variable always live; sticky incomplete ScopeInactive (no invent soft re-pick)
	if v == nil {
		SetError(ErrGeneric)
		return ScopeInactive
	}
	if v.IsGlobal() {
		// residual ERROR sticky — no invent global-scope past IsGlobal residual hole
		if HasError() {
			return ScopeInactive
		}
		return ScopeGlobalVar
	}
	// residual ERROR sticky — no invent soft-continue scope past IsGlobal residual false
	if HasError() {
		return ScopeInactive
	}
	// non-global scope needs live curr_func (CGContext.cpp always has it for locals/params);
	// sticky ScopeInactive (no invent "not found" soft re-pick past missing frame shell)
	f := c.CurrentFunc
	if f == nil {
		SetError(ErrGeneric)
		return ScopeInactive
	}
	// params → 0
	// Variable* always live on Param; nil hole sticky fail closed as inactive
	for _, p := range f.Param {
		if p == nil {
			SetError(ErrGeneric)
			return ScopeInactive
		}
		if p.Match(v) {
			// residual ERROR sticky — no invent param-scope true past Match hole
			if HasError() {
				return ScopeInactive
			}
			return 0
		}
		// residual ERROR sticky — no invent soft-continue then later scope past Match residual
		if HasError() {
			return ScopeInactive
		}
	}
	// visible in current function blocks
	b := c.CurrentBlock()
	idx := 1
	for b != nil {
		for _, loc := range b.LocalVars {
			if loc == nil {
				// incomplete LocalVars sticky ScopeInactive
				SetError(ErrGeneric)
				return ScopeInactive
			}
			if loc == v {
				return idx
			}
		}
		b = b.Parent
		idx++
	}
	// exist on one of the stack frames (caller) → INVISIBLE
	for i := len(c.CallChain) - 1; i >= 0; i-- {
		b = c.CallChain[i]
		if b == nil {
			// incomplete call_chain sticky ScopeInactive
			SetError(ErrGeneric)
			return ScopeInactive
		}
		for b != nil {
			for _, loc := range b.LocalVars {
				if loc == nil {
					SetError(ErrGeneric)
					return ScopeInactive
				}
				if loc == v {
					return ScopeInvisible
				}
			}
			b = b.Parent
		}
	}
	return ScopeInactive
}

// InLoop is true when flags include IN_LOOP (2).
func (c CGContext) InLoop() bool { return c.Flags&FlagInLoop != 0 }

// NoDanglingPtr is true when flags include NO_DANGLING_PTR (8).
func (c CGContext) NoDanglingPtr() bool { return c.Flags&FlagNoDanglingPtr != 0 }

// WithFlags returns a copy with additional flags OR'd in.
// IVBounds is deep-copied (see CloneSubcontext) so loop-body contexts do not
// share the caller's map pointer.
func (c CGContext) WithFlags(f uint) CGContext {
	c = c.CloneSubcontext()
	c.Flags |= f
	return c
}

// CloneSubcontext returns a value copy whose iv_bounds map is independent.
// C++ CGContext copy constructors deep-copy std::map iv_bounds (CGContext.cpp:74–100).
// A shallow Go struct copy would share the map: AddIVBound/RemoveIVBound on a child
// context then leaks/strips IVs on the parent (ItemizeArray ok_ivs n inflated).
func (c CGContext) CloneSubcontext() CGContext {
	if c.IVBounds != nil {
		m := make(map[*Variable]int, len(c.IVBounds))
		for k, v := range c.IVBounds {
			m[k] = v
		}
		c.IVBounds = m
	}
	return c
}

// WithRW attaches an RWDirective.
func (c CGContext) WithRW(rw *RWDirective) CGContext {
	c.RW = rw
	return c
}

// ClearEffectStm mirrors get_effect_stm().clear().
// CGContext always live; sticky (no invent soft-skip clear past hole).
func (c *CGContext) ClearEffectStm() {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	c.EffectStm = EmptyEffect()
}

// ResetEffectAccum mirrors CGContext::reset_effect_accum.
// CGContext.h:156 — replace effect_accum with a snapshot (e.g. after failed DFA).
// CGContext always live; sticky (no invent soft-skip reset past hole).
func (c *CGContext) ResetEffectAccum(e Effect) {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	if os.Getenv("CSMITH_DEBUG_G32") != "" && c.EffectAccum != nil {
		had32 := c.EffectAccum.IsReadByName("g_32")
		ClearError()
		if had32 {
			fmt.Fprintf(os.Stderr, "RESET_ACCUM had_g32 nread=%d\n", len(c.EffectAccum.ReadVars()))
			ClearError()
		}
	}
	cp := e.Clone()
	// residual ERROR sticky — no invent soft-reset past IncompleteEffect Clone residual
	if HasError() {
		return
	}
	if !EffectComplete(cp) {
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = cp
		return
	}
	c.EffectAccum = &cp
}

// ExtendCallChain mirrors CGContext::extend_call_chain.
// CGContext.cpp:470–478 — copy parent chain, push current block.
// Block* always live on call_chain; nil hole sticky empty chain
// (no invent keep-hole chain that soft-skips frames later).
func (c *CGContext) ExtendCallChain(from CGContext) {
	// CGContext always live for extend; sticky incomplete shell no invent no-op
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	for _, b := range from.CallChain {
		if b == nil {
			// incomplete call_chain sticky wipe (no invent soft-skip hole frames)
			SetError(ErrGeneric)
			c.CallChain = nil
			return
		}
	}
	c.CallChain = append([]*Block(nil), from.CallChain...)
	b := from.CurrentBlock()
	if b != nil {
		c.CallChain = append(c.CallChain, b)
	}
}

// OutputCallChain mirrors CGContext::output_call_chain.
// CGContext.cpp:481–490 — "b%p in func -> ..." debug line.
// C++ always has live Block* and b->func->name; incomplete frames fail closed
// (no invent "?" / blank " in " / skip holes that soft-rewrite the chain).
func (c CGContext) OutputCallChain() string {
	var b strings.Builder
	for i, blk := range c.CallChain {
		// CGContext.cpp:484 — call_chain[i] always live Block*; incomplete sticky
		// (no invent "?" / blank " in " / skip holes that soft-rewrite the chain)
		if blk == nil || blk.Func == nil || blk.Func.Name == "" {
			SetError(ErrGeneric)
			return ""
		}
		if i > 0 {
			b.WriteString(" -> ")
		}
		b.WriteString("b")
		b.WriteString(fmt.Sprintf("%p", blk))
		b.WriteString(" in ")
		b.WriteString(blk.Func.Name)
	}
	b.WriteString("\n")
	return b.String()
}

// AddExternalEffect mirrors CGContext::add_external_effect.
// CGContext.cpp:399–404 — merge global-only effects into accum and stm.
// Incomplete e or base fails closed sticky error (no invent silent IncompleteEffect poison
// as successful merge / under-report callee effects).
// CGContext always live; sticky (no invent soft-skip merge past hole).
func (c *CGContext) AddExternalEffect(e Effect) {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(e) || !EffectComplete(c.EffectStm) ||
		(c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddExternalEffect(e)
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
	}
	c.EffectStm = c.EffectStm.AddExternalEffect(e)
	if !EffectComplete(c.EffectStm) {
		SetError(ErrGeneric)
		return
	}
	if c.CurrentFunc != nil {
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.AddExternalEffect(e)
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
		}
	}
}

// AddVisibleEffect mirrors CGContext::add_visible_effect with current block.
// CGContext.cpp:411–417 — call_chain + b as callers for external effect.
func (c *CGContext) AddVisibleEffect(e Effect) {
	c.AddVisibleEffectAt(e, c.CurrentBlock())
}

// AddVisibleEffectAt mirrors CGContext::add_visible_effect(e, b).
// CGContext.cpp:411–417 — callers = call_chain then b.
// Block* always live on call_chain; nil hole or incomplete Param/LocalVars fails
// closed sticky error (no invent soft-skip merge as no-effect success / partial merge).
// AddVisibleEffectAt merges caller-visible external effect.
// CGContext always live; sticky (no invent soft-skip merge past hole).
func (c *CGContext) AddVisibleEffectAt(e Effect, b *Block) {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(e) || !EffectComplete(c.EffectStm) ||
		(c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return
	}
	for _, cb := range c.CallChain {
		if cb == nil || !cb.StackScanComplete() {
			// incomplete call_chain / stack lists — fail closed sticky (no invent skip merge)
			// residual ERROR sticky — no invent soft-skip merge past StackScan residual
			if !HasError() {
				SetError(ErrGeneric)
			}
			return
		}
	}
	if b != nil && !b.StackScanComplete() {
		// residual ERROR sticky — no invent soft-skip merge past StackScan residual
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	callers := append([]*Block(nil), c.CallChain...)
	if b != nil {
		callers = append(callers, b)
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddExternalEffectWithCallers(e, callers)
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
	}
	c.EffectStm = c.EffectStm.AddExternalEffectWithCallers(e, callers)
	if !EffectComplete(c.EffectStm) {
		SetError(ErrGeneric)
		return
	}
	if c.CurrentFunc != nil {
		// FEffect remains global-external summary for the function
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.AddExternalEffect(e)
		if !EffectComplete(c.CurrentFunc.FEffect) {
			SetError(ErrGeneric)
		}
	}
}

// AddEffect mirrors CGContext::add_effect — fold into accum and effect_stm.
// CGContext.cpp:382–388.
// Incomplete e or base fails closed sticky error (no invent silent IncompleteEffect poison).
// AddEffect folds e into accum and effect_stm.
// CGContext always live; sticky (no invent soft-skip fold past hole).
func (c *CGContext) AddEffect(e Effect, includeLHS bool) {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(e) || !EffectComplete(c.EffectStm) ||
		(c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddEffectOpts(e, includeLHS)
		// residual ERROR sticky — no invent soft-continue stm past accum AddEffect residual
		if HasError() {
			return
		}
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return
		}
	}
	// CGContext.cpp:386 — effect_stm.add_effect(e) always default include_lhs=false
	c.EffectStm = c.EffectStm.AddEffectOpts(e, false)
	// residual ERROR sticky — no invent soft-complete merge past stm AddEffect residual
	if HasError() {
		return
	}
	if !EffectComplete(c.EffectStm) {
		SetError(ErrGeneric)
	}
}

// MergeParamContext mirrors CGContext::merge_param_context.
// CGContext.cpp:390–394 — add_effect(*param.effect_accum, include_lhs); copy expr_depth.
// Does not merge param.effect_stm (only accum is folded via add_effect).
// Incomplete effect merge fails closed sticky (AddEffect SetError); do not invent
// expr_depth handoff after a failed effect merge.
// MergeParamContext folds param accum into caller and copies expr_depth.
// CGContext always live; sticky (no invent soft-skip merge past hole).
func (c *CGContext) MergeParamContext(param CGContext, includeLHS bool) {
	if c == nil {
		SetError(ErrGeneric)
		return
	}
	if param.EffectAccum != nil {
		// CGContext.cpp:392 — add_effect(*param.get_effect_accum(), include_lhs_effects)
		c.AddEffect(*param.EffectAccum, includeLHS)
		if HasError() {
			return
		}
	}
	// CGContext.cpp:393 — expr_depth = param_cg_context.expr_depth
	c.ExprDepth = param.ExprDepth
}

// FindMustUseArrays mirrors RWDirective::find_must_use_arrays.
// CGContext.cpp:610–624 — unique arrays from must_read and must_write.
// Variable* always live on must-use lists; nil hole sticky fails closed (nil out).
// IsArray without AsArray sticky fails closed (no invent soft-skip broken array
// as absent then complete empty must-use pool / soft re-pick past hole).
func (rw *RWDirective) FindMustUseArrays() []*ArrayVariable {
	// RWDirective always live when queried; nil RW complete empty (no must-use)
	if rw == nil {
		return nil
	}
	out := make([]*ArrayVariable, 0)
	seen := make(map[*Variable]bool)
	add := func(v *Variable) bool {
		if v == nil {
			// incomplete must-use list sticky (no invent soft-skip hole as absent)
			SetError(ErrGeneric)
			return false
		}
		if !v.IsArray || seen[v] {
			return true
		}
		seen[v] = true
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent soft-skip shell as absent then empty complete pool)
		if v.AsArray == nil {
			SetError(ErrGeneric)
			return false
		}
		out = append(out, v.AsArray)
		return true
	}
	for _, v := range rw.MustReadVars {
		if !add(v) {
			return nil
		}
	}
	for _, v := range rw.MustWriteVars {
		if !add(v) {
			return nil
		}
	}
	return out
}

// IsNonReadable mirrors CGContext::is_nonreadable.
// CGContext.cpp:118–128 — match against no_read_vars.
// Variable* always live on NoReadVars; nil hole sticky nonreadable
// (no invent readable past hole / soft re-pick).
func (c CGContext) IsNonReadable(v *Variable) bool {
	// subject always live; nil subject complete readable (no RW ban)
	if c.RW == nil || v == nil {
		return false
	}
	for _, nr := range c.RW.NoReadVars {
		if nr == nil {
			// incomplete NoReadVars sticky nonreadable (restrictive)
			SetError(ErrGeneric)
			return true
		}
		if nr.Match(v) {
			// residual ERROR sticky — no invent nonreadable true past Match hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue readable past Match residual false path
		if HasError() {
			return true
		}
	}
	return false
}

// IsNonWritable mirrors CGContext::is_nonwritable.
// CGContext.cpp:133–149 — loose_match no_write_vars; IV bounds.
// Variable* always live on NoWriteVars/IVBounds; nil hole sticky nonwritable
// (no invent writable past hole / soft re-pick).
func (c CGContext) IsNonWritable(v *Variable) bool {
	// subject always live; nil subject complete writable
	if v == nil {
		return false
	}
	if c.RW != nil {
		for _, nw := range c.RW.NoWriteVars {
			if nw == nil {
				// incomplete NoWriteVars sticky nonwritable (restrictive)
				SetError(ErrGeneric)
				return true
			}
			if nw.LooseMatch(v) || v.LooseMatch(nw) {
				// residual ERROR sticky — no invent nonwritable true past LooseMatch hole
				if HasError() {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue writable past LooseMatch residual
			if HasError() {
				return true
			}
		}
	}
	// not writing to loop IVs (avoid infinite loops)
	for iv := range c.IVBounds {
		if iv == nil {
			// incomplete IVBounds sticky nonwritable
			SetError(ErrGeneric)
			return true
		}
		if v.LooseMatch(iv) {
			// residual ERROR sticky — no invent nonwritable true past LooseMatch hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue writable past LooseMatch residual
		if HasError() {
			return true
		}
	}
	return false
}

// AddIVBound records a loop induction variable (StatementFor.cpp:441–443).
// CGContext + IV always live; sticky (no invent soft-skip IV bound past hole).
func (c *CGContext) AddIVBound(iv *Variable, bound int) {
	if c == nil || iv == nil {
		SetError(ErrGeneric)
		return
	}
	if c.IVBounds == nil {
		c.IVBounds = make(map[*Variable]int)
	}
	c.IVBounds[iv] = bound
}

// RemoveIVBound clears a loop induction variable (StatementFor.cpp:447, 470).
// CGContext + IV always live; sticky (no invent soft-skip remove past hole).
// Nil IVBounds is complete no-op (no bounds to clear).
func (c *CGContext) RemoveIVBound(iv *Variable) {
	if c == nil || iv == nil {
		SetError(ErrGeneric)
		return
	}
	if c.IVBounds == nil {
		return
	}
	delete(c.IVBounds, iv)
}

// VariablesComplete reports set has no nil Variable* holes.
// Incomplete lists must not invent membership via IsVariableInSet (false past a
// hole drops params/locals from keep sets / lower_block coverage).
// Note: VariablesComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteVariables() so bare nil cannot invent empty-complete
// expand/frame/RW lists.
func VariablesComplete(set []*Variable) bool {
	for _, x := range set {
		if x == nil {
			return false
		}
	}
	return true
}

// IncompleteVariables is the fail-closed incomplete Variable* list marker.
// VariablesComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteVariables() []*Variable {
	return []*Variable{nil}
}

// IsVariableInSet reports whether v appears in set (pointer equality).
// Nil slots are never matches for a live v (pointer equality); callers that
// need fail-closed incomplete lists must use VariablesComplete first.
func IsVariableInSet(set []*Variable, v *Variable) bool {
	// subject always live; nil subject complete not-in-set
	if v == nil {
		return false
	}
	if !VariablesComplete(set) {
		// incomplete membership is false for the bit — non-sticky soft filter;
		// callers that must not invent not-in-set use VariablesComplete + fail closed
		return false
	}
	for _, x := range set {
		if x == v {
			return true
		}
	}
	return false
}

// ReadIndices mirrors CGContext::read_indices for array subscript expressions.
// CGContext.cpp:352–380 — visit IndexExprs on itemized arrays; walk field_var_of;
// array fields walk to parent array.
// Hard IR (nil subject, IsArray without AsArray, missing parent array) sticky.
func (c *CGContext) ReadIndices(v *Variable, facts []*FactPointTo) bool {
	// CGContext.cpp:352+ — always live this + v; hard IR sticky (no soft invent true)
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.IsArray || v.AsArray != nil {
		av := v.AsArray
		// C++ static_cast ArrayVariable* on isArray; missing AsArray is broken IR sticky
		if av == nil {
			SetError(ErrGeneric)
			return false
		}
		// Expression::visit_facts reads process CGOptions; no Defaults invent
		opts := ProcessOptions()
		// CGContext.cpp:356–363 — visit each index expression (live Expression*)
		// incomplete IndexExprs fails closed sticky (no invent soft-skip nil index)
		if !ExpressionsComplete(av.IndexExprs) {
			SetError(ErrGeneric)
			return false
		}
		for _, e := range av.IndexExprs {
			// work on a context copy so index visits don't clobber caller stm incorrectly
			// (upstream mutates facts_copy; VisitFactsExpression uses GlobalFacts on FM)
			if !VisitFactsExpression(e, c, opts) {
				return false
			}
			// residual ERROR sticky — no invent soft-continue later indices past visit residual
			if HasError() {
				return false
			}
		}
		if av.FieldVarOf != nil {
			ok := c.ReadIndices(av.FieldVarOf, facts)
			// residual ERROR sticky — no invent soft-skip parent indices past nested residual
			if HasError() {
				return false
			}
			return ok
		}
		return true
	}
	if v.IsArrayField() {
		// residual ERROR sticky — no invent soft-continue field-walk past IsArrayField hole
		if HasError() {
			return false
		}
		// CGContext.cpp:368–374 — walk to parent array; assert(v) found
		p := v
		for p != nil && !p.IsArray && p.AsArray == nil {
			p = p.FieldVarOf
		}
		// CGContext.cpp:373 — assert(v); hard IR sticky when parent missing
		if p == nil {
			SetError(ErrGeneric)
			return false
		}
		ok := c.ReadIndices(p, facts)
		// residual ERROR sticky — no invent soft-skip parent indices past nested residual
		if HasError() {
			return false
		}
		return ok
	}
	// residual ERROR sticky — no invent complete true past IsArrayField residual false path
	if HasError() {
		return false
	}
	return true
}

// ReadVar mirrors CGContext::read_var — force read into accum + stm.
// CGContext.cpp:175–185.
// Incomplete GetCollective fails closed (no invent read/write on nil collective shell).
// Incomplete EffectStm/accum fails closed sticky error (no invent silent grow on hole shell).
// ReadVar forces a read into accum + stm.
// CGContext + Variable always live; sticky (no invent soft-skip read past hole).
func (c *CGContext) ReadVar(v *Variable) {
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return
	}
	v = v.GetCollective()
	// residual ERROR sticky — no invent soft-continue read past GetCollective residual
	if HasError() {
		return
	}
	if v == nil {
		// incomplete field/array collective path — fail closed sticky error
		SetError(ErrGeneric)
		return
	}
	if c.IsNonReadable(v) {
		// residual ERROR sticky — no invent soft-skip read past IsNonReadable residual
		if HasError() {
			return
		}
		// CGContext.cpp:178 — assert(!"attempted read from a nonreadable variable")
		// no soft invent silent skip: set sticky error for ERROR_GUARD callers
		SetError(ErrGeneric)
		return
	}
	// residual ERROR sticky — no invent soft-continue read past IsNonReadable residual false
	if HasError() {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.ReadVar(v)
		// residual ERROR sticky — no invent soft-continue stm read past accum ReadVar residual
		if HasError() {
			return
		}
	}
	c.EffectStm = c.EffectStm.ReadVar(v)
	// residual ERROR sticky — no invent soft-continue FEffect past stm ReadVar residual
	if HasError() {
		return
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual hole
		if HasError() {
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.ReadVar(v)
		// residual ERROR sticky — no invent soft-complete read past FEffect ReadVar residual
		if HasError() {
			return
		}
	} else if HasError() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual false
		return
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
	}
}

// WriteVar mirrors CGContext::write_var.
// CGContext.cpp:307–317.
// Incomplete GetCollective fails closed (no invent write on nil collective shell).
// Incomplete EffectStm/accum fails closed sticky error (no invent silent grow on hole shell).
// WriteVar forces a write into accum + stm.
// CGContext + Variable always live; sticky (no invent soft-skip write past hole).
func (c *CGContext) WriteVar(v *Variable) {
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return
	}
	v = v.GetCollective()
	// residual ERROR sticky — no invent soft-continue write past GetCollective residual
	if HasError() {
		return
	}
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	if c.IsNonWritable(v) {
		// residual ERROR sticky — no invent soft-skip write past IsNonWritable residual
		if HasError() {
			return
		}
		// CGContext.cpp:310 — assert(!"attempted write to a nonwritable variable")
		// no soft invent silent skip
		SetError(ErrGeneric)
		return
	}
	// residual ERROR sticky — no invent soft-continue write past IsNonWritable residual false
	if HasError() {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.WriteVar(v)
		// residual ERROR sticky — no invent soft-continue stm write past accum WriteVar residual
		if HasError() {
			return
		}
	}
	c.EffectStm = c.EffectStm.WriteVar(v)
	// residual ERROR sticky — no invent soft-continue FEffect past stm WriteVar residual
	if HasError() {
		return
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual hole
		if HasError() {
			return
		}
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.WriteVar(v)
		// residual ERROR sticky — no invent soft-complete write past FEffect WriteVar residual
		if HasError() {
			return
		}
	} else if HasError() {
		// residual ERROR sticky — no invent soft-skip FEffect past IsGlobal residual false
		return
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
	}
}

// CheckDerefVolatile mirrors CGContext::check_deref_volatile.
// CGContext.cpp:152–169.
// Incomplete ambient/stm sticky (no invent OK under IncompleteEffect / soft re-pick).
func (c *CGContext) CheckDerefVolatile(v *Variable, derefLevel int, opts Options) bool {
	// CGContext.cpp:153 — assert(v && "nullptr Variable!"); hard IR sticky
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return false
	}
	if !opts.StrictVolatileRule {
		return true
	}
	// Incomplete ambient fails closed sticky (no invent OK / soft re-pick past hole)
	if !EffectComplete(c.EffectContext()) {
		SetError(ErrGeneric)
		return false
	}
	if !c.EffectContext().IsSideEffectFree() {
		// residual ERROR sticky — no invent OK past IsSideEffectFree residual false path
		if HasError() {
			return false
		}
		level := derefLevel
		for level > 0 {
			if v.IsVolatileAfterDeref(level) {
				// residual ERROR sticky — no invent OK past IsVolatileAfterDeref residual
				if HasError() {
					return false
				}
				// policy reject (volatile under impure) — not incomplete IR sticky
				return false
			}
			// residual ERROR sticky — no invent soft-continue peel past residual false path
			if HasError() {
				return false
			}
			level--
		}
	} else if HasError() {
		// residual ERROR sticky — no invent SE-free true past residual hole
		return false
	}
	// Incomplete accum/stm AccessDerefVolatile sticky (AccessDerefVolatile sets ERROR)
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AccessDerefVolatile(v, derefLevel, true)
		// residual ERROR sticky — no invent soft-continue stm past accum AccessDeref residual
		if HasError() {
			return false
		}
		if !EffectComplete(*c.EffectAccum) {
			SetError(ErrGeneric)
			return false
		}
	}
	c.EffectStm = c.EffectStm.AccessDerefVolatile(v, derefLevel, true)
	// residual ERROR sticky — no invent soft-OK past stm AccessDeref residual
	if HasError() {
		return false
	}
	if !EffectComplete(c.EffectStm) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	return true
}

// pointToFacts returns FactMgr global points-to facts (or nil).
func (c CGContext) pointToFacts() []*FactPointTo {
	if c.FM == nil {
		return nil
	}
	return c.FM.GlobalFacts
}

// unionFacts returns FactMgr union facts when present.
func (c CGContext) unionFacts() []*FactUnion {
	if c.FM == nil {
		return nil
	}
	return c.FM.UnionFacts
}

// CheckReadVar mirrors CGContext::check_read_var.
// CGContext.cpp:191–213 — indices, nonreadable field/context, partial write, volatile, dangling.
// Incomplete EffectStm/accum / GetCollective sticky (no invent read success / soft re-pick).
// Type* always live for non-special subjects; Type-nil sticky false (IsPointer residual
// ERROR+false skips dangling then invents ReadVar true past Type-nil shell).
// Policy rejects (nonreadable, partial write, volatile, dangling) stay non-sticky false.
func (c *CGContext) CheckReadVar(v *Variable, facts []*FactPointTo) bool {
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return false
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return false
	}
	if !c.ReadIndices(v, facts) {
		// ReadIndices hard IR already sticky; visit fail may not be
		return false
	}
	v = v.GetCollective()
	if v == nil {
		// GetCollective already SetError sticky
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if IsNonreadableField(v, c.unionFacts()) {
		// residual ERROR sticky — no invent read-ok past nonreadable hole
		return false
	}
	// residual ERROR sticky — no invent read-ok past IsInsideUnionField residual false path
	if HasError() {
		return false
	}
	if c.IsNonReadable(v) {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	if c.EffectContext().IsWrittenPartially(v) {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	if v.IsVolatile() && !c.EffectContext().IsSideEffectFree() {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	// Type* always live for non-special subjects; Type-nil sticky false
	// (IsPointer residual ERROR+false skips dangling then invents ReadVar true)
	if !IsSpecialPtr(v) && v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	// FactPointTo::is_dangling_ptr uses CGOptions::dead_pointer_dereference_prob()
	// CGOptions::dead_pointer_dereference_prob only (no dual residual knob)
	if v.IsPointer() && IsDanglingPtr(v, facts, ProcessOptions().DeadPointerDerefProb) {
		// residual ERROR sticky — no invent read-ok past IsPointer/dangling hole
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	c.ReadVar(v)
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// never invent read-complete success with residual ERROR set
	if HasError() {
		return false
	}
	return true
}

// CheckWriteVar mirrors CGContext::check_write_var.
// CGContext.cpp:323–349.
// Incomplete EffectStm/accum / GetCollective sticky (no invent write success / soft re-pick).
// Type* always live for non-special subjects; Type-nil sticky false (IsPointer residual
// ERROR+false skips dangling then invents WriteVar true past Type-nil shell).
// Policy rejects (const, nonwritable, partial, volatile, dangling) stay non-sticky false.
func (c *CGContext) CheckWriteVar(v *Variable, facts []*FactPointTo) bool {
	if c == nil || v == nil {
		SetError(ErrGeneric)
		return false
	}
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		SetError(ErrGeneric)
		return false
	}
	if !c.ReadIndices(v, facts) {
		return false
	}
	v = v.GetCollective()
	if v == nil {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if c.IsNonWritable(v) || v.IsConst() {
		// residual ERROR sticky — no invent write-ok past nonwritable/const hole
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	eff := c.EffectContext()
	if eff.IsWrittenPartially(v) || eff.IsReadPartially(v) {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	if v.IsVolatile() && !eff.IsSideEffectFree() {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	// Type* always live for non-special subjects; Type-nil sticky false
	// (IsPointer residual ERROR+false skips dangling then invents WriteVar true)
	if !IsSpecialPtr(v) && v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	// CGContext.cpp:342–344 + is_dangling_ptr dead_pointer_dereference_prob
	if c.NoDanglingPtr() && v.IsPointer() && IsDanglingPtr(v, facts, ProcessOptions().DeadPointerDerefProb) {
		if HasError() {
			return false
		}
		return false
	}
	if HasError() {
		return false
	}
	c.WriteVar(v)
	if !EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum)) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// never invent write-complete success with residual ERROR set
	if HasError() {
		return false
	}
	return true
}

// ReadPointed mirrors CGContext::read_pointed — recursive pointee reads.
// CGContext.cpp:216–252.
// Hard IR (nil subject, incomplete collective, nil pointee holes) sticky;
// empty/null/dead policy rejects and incomplete MergePointees stay non-sticky.
func (c *CGContext) ReadPointed(v *Variable, indirect int, facts []*FactPointTo, opts Options) bool {
	if c == nil || v == nil || indirect <= 0 {
		SetError(ErrGeneric)
		return false
	}
	// CGContext.cpp:218 — Effect effect_accum_copy = *effect_accum (deep vector copy).
	// Clone() matches; shallow *EffectAccum shares maps with live accum.
	var accumCopy *Effect
	if c.EffectAccum != nil {
		cp := c.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-read_pointed past Effect Clone residual
		if HasError() {
			return false
		}
		if !EffectComplete(cp) {
			SetError(ErrGeneric)
			return false
		}
		accumCopy = &cp
	}
	IncrCounter(&dereferenceLevelCnts, indirect)
	allowNull := opts.NullPointerDerefProb > 0
	allowDead := opts.DeadPointerDerefProb > 0
	if !c.ReadIndices(v, facts) {
		return false
	}
	// residual ERROR sticky — no invent soft-continue pointees past ReadIndices residual
	if HasError() {
		return false
	}
	// incomplete collective sticky via GetCollective
	coll := v.GetCollective()
	if coll == nil {
		if accumCopy != nil && c.EffectAccum != nil {
			*c.EffectAccum = *accumCopy
		}
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	tmp := []*Variable{coll}
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		// incomplete pointees non-sticky hole; empty/null/dead policy non-sticky
		if !VariablesComplete(tmp) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			// MergePointees incomplete stays non-sticky for fact-map soft re-pick
			return false
		}
		// residual ERROR sticky — no invent soft-continue pointees past MergePointees residual
		if HasError() {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		if len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			// Variable* always live after VariablesComplete
			if IsSpecialPtr(pointee) {
				continue
			}
			if !c.CheckReadVar(pointee, facts) {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
			// residual ERROR sticky — no invent soft-continue later pointees past CheckReadVar residual
			if HasError() {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
		}
	}
	return true
}

// WritePointed mirrors CGContext::write_pointed — last hop is write, intermediates read.
// CGContext.cpp:255–304.
// Hard IR (nil lhs, incomplete collective) sticky; MergePointees incomplete non-sticky.
func (c *CGContext) WritePointed(lhs *Lhs, facts []*FactPointTo, opts Options) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete Lhs type IR sticky (no invent non-deref write-pointed)
	indirect, ok := lhs.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return false
	}
	if indirect <= 0 {
		// not a deref write — complete false (caller uses CheckWriteVar)
		return false
	}
	// CGContext.cpp:257 — Effect effect_accum_copy = *effect_accum (deep copy)
	var accumCopy *Effect
	if c.EffectAccum != nil {
		cp := c.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-write_pointed past Effect Clone residual
		if HasError() {
			return false
		}
		if !EffectComplete(cp) {
			SetError(ErrGeneric)
			return false
		}
		accumCopy = &cp
	}
	IncrCounter(&dereferenceLevelCnts, indirect)
	if !c.ReadIndices(lhs.Var, facts) {
		return false
	}
	// incomplete collective sticky via GetCollective
	coll := lhs.Var.GetCollective()
	if coll == nil {
		if accumCopy != nil && c.EffectAccum != nil {
			*c.EffectAccum = *accumCopy
		}
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	tmp := []*Variable{coll}
	allowNull := opts.NullPointerDerefProb > 0
	allowDead := opts.DeadPointerDerefProb > 0
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		if !VariablesComplete(tmp) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			// incomplete MergePointees non-sticky
			return false
		}
		if len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			// Variable* always live after VariablesComplete
			if IsSpecialPtr(pointee) {
				continue
			}
			var succ bool
			if indirect == 0 {
				succ = c.CheckWriteVar(pointee, facts)
			} else {
				succ = c.CheckReadVar(pointee, facts)
			}
			if !succ {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
			// residual ERROR sticky — no invent soft-continue later pointees past Check residual
			if HasError() {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
		}
	}
	return true
}

// VisitFactsExpressionVariable mirrors ExpressionVariable::visit_facts.
// ExpressionVariable.cpp:237–274.
func (c *CGContext) VisitFactsExpressionVariable(e *Expression, opts Options) bool {
	// incomplete ExpressionVariable shell sticky
	if c == nil || e == nil || e.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	facts := c.pointToFacts()
	// incomplete type IR sticky (no invent non-deref level-0 visit success)
	deref, ok := e.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return false
	}
	v := e.Var
	if deref > 0 {
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			// invalid ptr policy / incomplete maps (IsValidPtr may sticky)
			// residual ERROR sticky — no invent soft-continue visit past IsValidPtr residual
			if HasError() {
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past IsValidPtr residual true path
		if HasError() {
			return false
		}
		if !c.CheckReadVar(v, facts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past CheckReadVar residual
		if HasError() {
			return false
		}
		if !c.ReadPointed(v, deref, facts, opts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past ReadPointed residual
		if HasError() {
			return false
		}
		volOK := c.CheckDerefVolatile(v, deref, opts)
		// residual ERROR sticky — no invent visit success past CheckDerefVolatile residual
		if HasError() {
			return false
		}
		return volOK
	}
	if deref < 0 {
		// address-of: forbid bitfields — policy non-sticky
		if v.IsBitfield {
			return false
		}
		return true
	}
	readOK := c.CheckReadVar(v, facts)
	// residual ERROR sticky — no invent visit success past CheckReadVar residual
	if HasError() {
		return false
	}
	return readOK
}

// AllowVolatile mirrors CGContext::allow_volatile.
// CGContext.cpp:517–518 — only when effect_context is side-effect free.
// Incomplete ambient sticky false (no invent allow-vol under IncompleteEffect).
func (c CGContext) AllowVolatile() bool {
	if !EffectComplete(c.EffectContext()) {
		SetError(ErrGeneric)
		return false
	}
	ok := c.EffectContext().IsSideEffectFree()
	// residual ERROR sticky — no invent allow-vol true past IsSideEffectFree residual hole
	if HasError() {
		return false
	}
	return ok
}

// AllowConst mirrors CGContext::allow_const — const only for non-WRITE access.
// CGContext.cpp:521–523.
func (c CGContext) AllowConst(access Access) bool {
	return access != AccessWrite
}

// AcceptType mirrors CGContext::accept_type.
// CGContext.cpp:525–528 — reject volatile aggregates when not SE-free.
// Nil type sticky; incomplete ambient sticky (no invent accept under IncompleteEffect).
func (c CGContext) AcceptType(t *Type) bool {
	if t == nil {
		SetError(ErrGeneric)
		return false
	}
	if !EffectComplete(c.EffectContext()) {
		SetError(ErrGeneric)
		return false
	}
	if c.EffectContext().IsSideEffectFree() {
		// residual ERROR sticky — no invent accept past IsSideEffectFree hole
		if HasError() {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue vol check past IsSideEffectFree residual
	if HasError() {
		return false
	}
	vol := t.IsVolatileStructUnion()
	// residual ERROR sticky — no invent accept true past IsVolatileStructUnion residual
	if HasError() {
		return false
	}
	return !vol
}

// InConflict mirrors CGContext::in_conflict — callee effect vs current context.
// CGContext.cpp:531–564.
// Variable* always live in effect lists; nil hole fails closed sticky as conflict
// (no invent skip / soft re-pick conflict-free incomplete effect).
// Incomplete eff or effect_context sticky as conflict.
func (c CGContext) InConflict(eff Effect) bool {
	if !EffectComplete(eff) || !EffectComplete(c.EffectContext()) {
		SetError(ErrGeneric)
		return true
	}
	for _, v := range eff.ReadVars() {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if c.IsNonReadable(v) {
			// residual ERROR sticky — no invent conflict true past IsNonReadable hole
			return true
		}
		// residual ERROR sticky — no invent soft-continue conflict scan past IsNonReadable residual
		if HasError() {
			return true
		}
		if c.EffectContext().IsWrittenPartially(v) {
			// residual ERROR sticky — no invent conflict true past IsWrittenPartially residual hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue conflict scan past IsWrittenPartially residual false
		if HasError() {
			return true
		}
		if v.IsVolatile() && !c.EffectContext().IsSideEffectFree() {
			// residual ERROR sticky — no invent conflict past IsVolatile/SE residual
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue conflict scan past IsVolatile residual false
		if HasError() {
			return true
		}
	}
	for _, v := range eff.WrittenVars() {
		if v == nil {
			SetError(ErrGeneric)
			return true
		}
		if c.IsNonWritable(v) || v.IsConst() {
			// residual ERROR sticky — no invent conflict true past IsNonWritable/IsConst hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue conflict scan past IsNonWritable residual false
		if HasError() {
			return true
		}
		ctx := c.EffectContext()
		if ctx.IsWrittenPartially(v) || ctx.IsReadPartially(v) {
			// residual ERROR sticky — no invent conflict true past partial residual hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue conflict scan past partial residual false
		if HasError() {
			return true
		}
		if v.IsVolatile() && !ctx.IsSideEffectFree() {
			if HasError() {
				return true
			}
			return true
		}
		if HasError() {
			return true
		}
	}
	return false
}

// IsFrameVar mirrors CGContext::is_frame_var — visible local in current/call_chain.
// CGContext.cpp:492–504.
// Incomplete Param/LocalVars on current or call_chain frames: membership false
// (IsVisibleLocal short-circuits); FindReachableFrameVars fails closed on incomplete
// stacks so under-reporting frame pointees is not invented as complete empty.
func (c CGContext) IsFrameVar(v *Variable) bool {
	// Variable always live; sticky incomplete no invent not-frame soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	// CGContext.cpp:493–494 — get_current_block(); assert(b)
	// no current block: complete not-frame (FindReachableFrameVars treats empty frames
	// as complete; sticky here poisons BuildCalleeRWDirective / generation soft paths)
	b := c.CurrentBlock()
	if b == nil {
		return false
	}
	if !b.StackScanComplete() {
		// incomplete stack sticky (no invent not-frame / soft re-pick past hole)
		// residual ERROR sticky — no invent soft not-frame past StackScan residual
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if v.IsVisibleLocal(b) {
		// residual ERROR sticky — no invent frame-true past IsVisibleLocal hole
		if HasError() {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent not-frame soft-skip past IsVisibleLocal hole
	if HasError() {
		return false
	}
	for _, cb := range c.CallChain {
		// Block* always live on call_chain; nil / incomplete stack sticky fail closed
		// (no invent skip hole and still match a later frame)
		if cb == nil || !cb.StackScanComplete() {
			// residual ERROR sticky — no invent soft not-frame past StackScan residual
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		if v.IsVisibleLocal(cb) {
			if HasError() {
				return false
			}
			return true
		}
		if HasError() {
			return false
		}
	}
	return false
}

// frameStacksComplete reports current block + call_chain stacks have no holes.
// No current block: no frames are scanned (IsFrameVar fails closed without curr_blk);
// that empty-frame env is complete. Incomplete stacks only when a live frame is present.
func (c CGContext) frameStacksComplete() bool {
	b := c.CurrentBlock()
	if b == nil {
		return true
	}
	if !b.StackScanComplete() {
		// residual ERROR sticky — no invent soft-complete frames past StackScan residual
		if HasError() {
			return false
		}
		return false
	}
	for _, cb := range c.CallChain {
		if cb == nil || !cb.StackScanComplete() {
			if HasError() {
				return false
			}
			return false
		}
	}
	return true
}

// FindReachableFrameVars mirrors CGContext::find_reachable_frame_vars.
// CGContext.cpp:566–578 — pointees that are frame locals.
// Fact* always live; incomplete map/pointees or incomplete frame stacks fail closed sticky
// IncompleteVariables (not bare nil invent empty-complete / soft re-pick past holes).
// Complete empty returns non-nil empty slice.
func (c CGContext) FindReachableFrameVars(facts []*FactPointTo) []*Variable {
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if !c.frameStacksComplete() {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	out := make([]*Variable, 0)
	seen := map[*Variable]bool{}
	for _, f := range facts {
		for _, p := range f.PointTo {
			// pointee Variable* always live in point-to sets (specials are non-nil)
			if p == nil {
				SetError(ErrGeneric)
				return IncompleteVariables()
			}
			if IsSpecialPtr(p) || seen[p] {
				continue
			}
			if c.IsFrameVar(p) {
				// residual ERROR sticky — no invent soft-continue frame scan past hole
				if HasError() {
					return IncompleteVariables()
				}
				seen[p] = true
				out = append(out, p)
			} else if HasError() {
				// residual ERROR sticky — no invent soft-skip not-frame past hard IR hole
				return IncompleteVariables()
			}
		}
	}
	return out
}

// GetExternalNoReadsWrites mirrors CGContext::get_external_no_reads_writes.
// CGContext.cpp:581–607 — globals + frame_vars from RW + global IVs as no-write.
// Variable* always live on RW/IV lists; nil hole fails closed sticky IncompleteVariables
// on both outs (no invent empty partial / soft re-pick past unrestricted empty RW).
func (c CGContext) GetExternalNoReadsWrites(frameVars []*Variable) (noReads, noWrites []*Variable) {
	inFrame := func(v *Variable) bool {
		if v == nil {
			return false
		}
		isG := v.IsGlobal()
		// residual ERROR sticky — no invent soft-frame past IsGlobal residual
		if HasError() {
			return false
		}
		if isG {
			return true
		}
		ok := IsVariableInSet(frameVars, v)
		// residual ERROR sticky — no invent soft-frame past IsVariableInSet residual
		if HasError() {
			return false
		}
		return ok
	}
	// incomplete frame list must not invent membership past holes
	if frameVars != nil && !VariablesComplete(frameVars) {
		SetError(ErrGeneric)
		return IncompleteVariables(), IncompleteVariables()
	}
	if c.RW != nil {
		for _, v := range c.RW.NoReadVars {
			if v == nil {
				SetError(ErrGeneric)
				return IncompleteVariables(), IncompleteVariables()
			}
			if inFrame(v) {
				// residual ERROR sticky — no invent soft-continue later no-reads past residual
				if HasError() {
					return IncompleteVariables(), IncompleteVariables()
				}
				noReads = append(noReads, v)
			} else if HasError() {
				// residual ERROR sticky — no invent soft-skip no-read past residual false
				return IncompleteVariables(), IncompleteVariables()
			}
		}
		for _, v := range c.RW.NoWriteVars {
			if v == nil {
				SetError(ErrGeneric)
				return IncompleteVariables(), IncompleteVariables()
			}
			if inFrame(v) {
				if HasError() {
					return IncompleteVariables(), IncompleteVariables()
				}
				noWrites = append(noWrites, v)
			} else if HasError() {
				return IncompleteVariables(), IncompleteVariables()
			}
		}
	}
	// convert global / frame IVs into non-writables
	for iv := range c.IVBounds {
		if iv == nil {
			SetError(ErrGeneric)
			return IncompleteVariables(), IncompleteVariables()
		}
		if inFrame(iv) {
			if HasError() {
				return IncompleteVariables(), IncompleteVariables()
			}
			noWrites = append(noWrites, iv)
		} else if HasError() {
			return IncompleteVariables(), IncompleteVariables()
		}
	}
	return noReads, noWrites
}

// BuildCalleeRWDirective builds RW for generate_body_with_known_params path.
// Function.cpp:675–681 — inherit external no-read/write from caller.
// Incomplete frame facts fail closed: inherit full caller NoRead/NoWrite without
// inventing unrestricted empty RW from incomplete reachable frames.
func (c CGContext) BuildCalleeRWDirective(facts []*FactPointTo) *RWDirective {
	frame := c.FindReachableFrameVars(facts)
	// residual ERROR sticky / incomplete frame — no invent unrestricted nil RW;
	// inherit full external NoRead/NoWrite (restrictive) and keep sticky ERROR
	if HasError() || !VariablesComplete(frame) {
		// incomplete facts — fail closed sticky full external lists (no invent empty RW
		// or soft re-pick unrestricted nil past holes)
		if !HasError() {
			SetError(ErrGeneric)
		}
		if c.RW == nil {
			return &RWDirective{}
		}
		// copy lists; nil holes already fail GetExternal path — copy as-is
		nr := append([]*Variable(nil), c.RW.NoReadVars...)
		nw := append([]*Variable(nil), c.RW.NoWriteVars...)
		for _, v := range nr {
			if v == nil {
				return &RWDirective{}
			}
		}
		for _, v := range nw {
			if v == nil {
				return &RWDirective{}
			}
		}
		if len(nr) == 0 && len(nw) == 0 {
			return &RWDirective{}
		}
		return &RWDirective{NoReadVars: nr, NoWriteVars: nw}
	}
	nr, nw := c.GetExternalNoReadsWrites(frame)
	// incomplete RW/IV lists — fail closed sticky empty RW directive (no invent unrestricted)
	if !VariablesComplete(nr) || !VariablesComplete(nw) {
		SetError(ErrGeneric)
		return &RWDirective{}
	}
	if len(nr) == 0 && len(nw) == 0 {
		return nil
	}
	return &RWDirective{NoReadVars: nr, NoWriteVars: nw}
}

// PtrModifiedInRhs mirrors Lhs::ptr_modified_in_rhs.
// Lhs.cpp:240–261 — intermediate pointers written in effect_stm from RHS.
// Incomplete pointees fail closed as modified (no invent unmodified).
// PtrModifiedInRhs mirrors Lhs.cpp:233–257 — intermediate pointer levels written by RHS.
// Hard IR incomplete sticky as modified (no invent unmodified / soft re-pick past holes);
// MergePointees incomplete stays non-sticky true (fact-map soft re-pick).
func (c *CGContext) PtrModifiedInRhs(lhs *Lhs, facts []*FactPointTo) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete Lhs type IR sticky as modified (no invent non-deref path)
	indirect, ok := lhs.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return true
	}
	// Lhs.cpp:243 — assert(indirect > 0); non-deref LHS is not this path
	if indirect <= 0 {
		return false
	}
	// if the pointer variable itself was written by RHS
	if c.EffectStm.IsWritten(lhs.Var) {
		// residual ERROR sticky — no invent modified true past IsWritten hole
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue unmodified past IsWritten residual false
	if HasError() {
		return true
	}
	// incomplete collective sticky as modified (GetCollective already SetError)
	coll := lhs.Var.GetCollective()
	if coll == nil {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	tmp := []*Variable{coll}
	// only intermediate pointer levels (not ultimate pointees)
	for indirect > 1 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		// incomplete pointees non-sticky true (fact-map soft re-pick)
		if !VariablesComplete(tmp) {
			return true
		}
		for _, v := range tmp {
			if c.EffectStm.IsWritten(v) {
				// residual ERROR sticky — no invent modified true past IsWritten hole
				if HasError() {
					return true
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue later pointees past IsWritten residual
			if HasError() {
				return true
			}
		}
	}
	return false
}

// VisitFactsLhs mirrors Lhs::visit_facts.
// Incomplete Lhs type IR fails closed sticky (no invent bare non-deref visit success
// / soft re-pick past broken LHS shells). Policy rejects stay non-sticky false.
// Lhs.cpp:301–356 — compound read-first; curr_rhs overlap; write/write_pointed.
func (c *CGContext) VisitFactsLhs(lhs *Lhs, opts Options) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	facts := c.pointToFacts()
	v := lhs.Var
	// compound assign: validate as read first (Lhs.cpp:307–311)
	if lhs.CompoundAssign {
		ty := lhs.GetType()
		// residual ERROR sticky — no invent soft-continue visit past GetType residual
		if HasError() {
			return false
		}
		ev := &Expression{Term: TermVariable, Var: v, ExprType: ty}
		if !c.VisitFactsExpressionVariable(ev, opts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past compound read residual
		if HasError() {
			return false
		}
	}
	// Lhs.cpp:313–315 — visit array indices
	if !lhs.VisitIndices(c, opts) {
		return false
	}
	// residual ERROR sticky — no invent soft-continue visit past VisitIndices residual true
	if HasError() {
		return false
	}
	// avoid overlapping union field assign a.x = a.y (Lhs.cpp:318–328)
	if c.CurrRHS != nil {
		lhsExpr := LhsAsExpression(lhs)
		// Lhs always live for this check; incomplete shell sticky
		if lhsExpr == nil {
			SetError(ErrGeneric)
			return false
		}
		// complete get_eval_to_subexps always ≥1 entry; incomplete sticky via GetEvalToSubexps
		// (IncompleteExpressions — no invent skip overlap as success past incomplete RHS)
		subs := GetEvalToSubexps(c.CurrRHS)
		// residual ERROR sticky — no invent soft-skip overlap past GetEvalToSubexps residual
		if HasError() {
			return false
		}
		if !ExpressionsComplete(subs) || len(subs) == 0 {
			SetError(ErrGeneric)
			return false
		}
		for _, sub := range subs {
			if sub.Term == TermVariable || sub.Term == TermLhs {
				if HaveOverlappingFields(sub, lhsExpr, facts) {
					// policy / incomplete overlap fail closed false (FindUnion may sticky)
					return false
				}
			}
		}
	}
	// incomplete Lhs type IR sticky (no invent non-deref level-0 visit success)
	deref, ok := lhs.IndirectLevelComplete()
	if !ok {
		SetError(ErrGeneric)
		return false
	}
	valid := false
	if deref > 0 {
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			// residual ERROR sticky — no invent soft-continue visit past IsValidPtr residual
			if HasError() {
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past IsValidPtr residual true path
		if HasError() {
			return false
		}
		// Lhs.cpp:337–339 — pointer modified in RHS
		if c.PtrModifiedInRhs(lhs, facts) {
			// residual ERROR sticky — no invent soft-continue visit past PtrModified residual true
			if HasError() {
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past PtrModified residual false
		if HasError() {
			return false
		}
		// read the pointer itself then write pointees
		if !c.CheckReadVar(v, facts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past CheckReadVar residual
		if HasError() {
			return false
		}
		if !c.WritePointed(lhs, facts, opts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past WritePointed residual
		if HasError() {
			return false
		}
		if !c.CheckDerefVolatile(v, deref, opts) {
			return false
		}
		// residual ERROR sticky — no invent soft-continue visit past CheckDerefVolatile residual
		if HasError() {
			return false
		}
		valid = true
	} else {
		valid = c.CheckWriteVar(v, facts)
		// residual ERROR sticky — no invent soft-continue visit past CheckWriteVar residual
		if HasError() {
			return false
		}
	}
	// Lhs.cpp:348–351 — set_lhs_write_vars from write_vars on accum
	// Incomplete SetLhsWriteVars / stm sticky (no invent visit true after poison)
	if valid && c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.SetLhsWriteVarsFromWritten()
		if !EffectComplete(*c.EffectAccum) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
	}
	if valid && (!EffectComplete(c.EffectStm) || (c.EffectAccum != nil && !EffectComplete(*c.EffectAccum))) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	return valid
}
