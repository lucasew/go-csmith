// Upstream: CGContext.h / CGContext.cpp (empty context + effect_context accessors).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

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
}

// EmptyCGContext mirrors CGContext::get_empty_context() (empty effect context).
func EmptyCGContext() CGContext {
	return CGContext{effectContext: EmptyEffect()}
}

// EffectContext mirrors CGContext::get_effect_context.
// Prefers EffectAccum when set (running accum from statements in block).
func (c CGContext) EffectContext() Effect {
	if c.EffectAccum != nil {
		return *c.EffectAccum
	}
	return c.effectContext
}

// NoteWrite records a variable write into EffectAccum if present.
// Also updates CurrentFunc.FEffect for globals (Function::feffect external).
func (c CGContext) NoteWrite(v *Variable) {
	if v == nil {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.WriteVar(v)
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.WriteVar(v)
	}
}

// NoteRead records a variable read into EffectAccum and FEffect for globals.
func (c CGContext) NoteRead(v *Variable) {
	if v == nil {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.ReadVar(v)
	}
	if c.CurrentFunc != nil && v.IsGlobal() {
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.ReadVar(v)
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

// InLoop is true when flags include IN_LOOP (2).
func (c CGContext) InLoop() bool { return c.Flags&FlagInLoop != 0 }

// NoDanglingPtr is true when flags include NO_DANGLING_PTR (8).
func (c CGContext) NoDanglingPtr() bool { return c.Flags&FlagNoDanglingPtr != 0 }

// WithFlags returns a copy with additional flags OR'd in.
func (c CGContext) WithFlags(f uint) CGContext {
	c.Flags |= f
	return c
}

// WithRW attaches an RWDirective.
func (c CGContext) WithRW(rw *RWDirective) CGContext {
	c.RW = rw
	return c
}

// ClearEffectStm mirrors get_effect_stm().clear().
func (c *CGContext) ClearEffectStm() {
	if c == nil {
		return
	}
	c.EffectStm = EmptyEffect()
}

// ExtendCallChain mirrors CGContext::extend_call_chain.
// CGContext.cpp:470–478 — copy parent chain, push current block.
func (c *CGContext) ExtendCallChain(from CGContext) {
	if c == nil {
		return
	}
	c.CallChain = append([]*Block(nil), from.CallChain...)
	b := from.CurrentBlock()
	if b != nil {
		c.CallChain = append(c.CallChain, b)
	}
}

// AddExternalEffect mirrors CGContext::add_external_effect.
// CGContext.cpp:399–404 — merge global-only effects into accum and stm.
func (c *CGContext) AddExternalEffect(e Effect) {
	if c == nil {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddExternalEffect(e)
	}
	c.EffectStm = c.EffectStm.AddExternalEffect(e)
	if c.CurrentFunc != nil {
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.AddExternalEffect(e)
	}
}

// AddVisibleEffect mirrors CGContext::add_visible_effect.
// CGContext.cpp:411–417 — external effect with call_chain (simplified: same as external).
func (c *CGContext) AddVisibleEffect(e Effect) {
	// full call-chain filtering deferred; global external is the main transfer
	c.AddExternalEffect(e)
}

// FindMustUseArrays mirrors RWDirective::find_must_use_arrays.
// CGContext.cpp:610–624 — unique arrays from must_read and must_write.
func (rw *RWDirective) FindMustUseArrays() []*ArrayVariable {
	if rw == nil {
		return nil
	}
	var out []*ArrayVariable
	seen := make(map[*Variable]bool)
	add := func(v *Variable) {
		if v == nil || !v.IsArray || seen[v] {
			return
		}
		seen[v] = true
		if v.AsArray != nil {
			out = append(out, v.AsArray)
		}
	}
	for _, v := range rw.MustReadVars {
		add(v)
	}
	for _, v := range rw.MustWriteVars {
		add(v)
	}
	return out
}

// IsNonReadable mirrors CGContext::is_nonreadable.
// CGContext.cpp:118–128 — match against no_read_vars.
func (c CGContext) IsNonReadable(v *Variable) bool {
	if c.RW == nil || v == nil {
		return false
	}
	for _, nr := range c.RW.NoReadVars {
		if nr != nil && nr.Match(v) {
			return true
		}
	}
	return false
}

// IsNonWritable mirrors CGContext::is_nonwritable.
// CGContext.cpp:133–149 — loose_match no_write_vars; IV bounds.
func (c CGContext) IsNonWritable(v *Variable) bool {
	if v == nil {
		return false
	}
	if c.RW != nil {
		for _, nw := range c.RW.NoWriteVars {
			if nw != nil && (nw.LooseMatch(v) || v.LooseMatch(nw)) {
				return true
			}
		}
	}
	// not writing to loop IVs (avoid infinite loops)
	for iv := range c.IVBounds {
		if iv != nil && v.LooseMatch(iv) {
			return true
		}
	}
	return false
}

// AddIVBound records a loop induction variable (StatementFor.cpp:441–443).
func (c *CGContext) AddIVBound(iv *Variable, bound int) {
	if c == nil || iv == nil {
		return
	}
	if c.IVBounds == nil {
		c.IVBounds = make(map[*Variable]int)
	}
	c.IVBounds[iv] = bound
}

// RemoveIVBound clears a loop induction variable (StatementFor.cpp:447, 470).
func (c *CGContext) RemoveIVBound(iv *Variable) {
	if c == nil || c.IVBounds == nil || iv == nil {
		return
	}
	delete(c.IVBounds, iv)
}
