// Upstream: CGContext.h / CGContext.cpp (empty context + effect_context accessors).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
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
	if v == nil {
		return ScopeInactive
	}
	if v.IsGlobal() {
		return ScopeGlobalVar
	}
	f := c.CurrentFunc
	if f == nil {
		return ScopeInactive
	}
	// params → 0
	// Variable* always live on Param; nil hole fails closed as inactive
	for _, p := range f.Param {
		if p == nil {
			return ScopeInactive
		}
		if p.Match(v) {
			return 0
		}
	}
	// visible in current function blocks
	b := c.CurrentBlock()
	idx := 1
	for b != nil {
		for _, loc := range b.LocalVars {
			if loc == nil {
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
			return ScopeInactive
		}
		for b != nil {
			for _, loc := range b.LocalVars {
				if loc == nil {
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

// ResetEffectAccum mirrors CGContext::reset_effect_accum.
// CGContext.h:156 — replace effect_accum with a snapshot (e.g. after failed DFA).
func (c *CGContext) ResetEffectAccum(e Effect) {
	if c == nil {
		return
	}
	cp := e.Clone()
	if c.EffectAccum != nil {
		*c.EffectAccum = cp
		return
	}
	c.EffectAccum = &cp
}

// ExtendCallChain mirrors CGContext::extend_call_chain.
// CGContext.cpp:470–478 — copy parent chain, push current block.
// Block* always live on call_chain; nil hole fails closed (empty chain —
// no invent keep-hole chain that soft-skips frames later).
func (c *CGContext) ExtendCallChain(from CGContext) {
	if c == nil {
		return
	}
	for _, b := range from.CallChain {
		if b == nil {
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
		// CGContext.cpp:484 — call_chain[i] always live Block*
		if blk == nil || blk.Func == nil || blk.Func.Name == "" {
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

// AddVisibleEffect mirrors CGContext::add_visible_effect with current block.
// CGContext.cpp:411–417 — call_chain + b as callers for external effect.
func (c *CGContext) AddVisibleEffect(e Effect) {
	c.AddVisibleEffectAt(e, c.CurrentBlock())
}

// AddVisibleEffectAt mirrors CGContext::add_visible_effect(e, b).
// CGContext.cpp:411–417 — callers = call_chain then b.
// Block* always live on call_chain; nil hole or incomplete Param/LocalVars fails
// closed (skip merge — no invent partial external effect / not-on-chain past holes).
func (c *CGContext) AddVisibleEffectAt(e Effect, b *Block) {
	if c == nil {
		return
	}
	for _, cb := range c.CallChain {
		if cb == nil || !cb.StackScanComplete() {
			// incomplete call_chain / stack lists — fail closed without inventing merge
			return
		}
	}
	if b != nil && !b.StackScanComplete() {
		return
	}
	callers := append([]*Block(nil), c.CallChain...)
	if b != nil {
		callers = append(callers, b)
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddExternalEffectWithCallers(e, callers)
	}
	c.EffectStm = c.EffectStm.AddExternalEffectWithCallers(e, callers)
	if c.CurrentFunc != nil {
		// FEffect remains global-external summary for the function
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.AddExternalEffect(e)
	}
}

// AddEffect mirrors CGContext::add_effect — fold into accum and effect_stm.
// CGContext.cpp:382–388.
func (c *CGContext) AddEffect(e Effect, includeLHS bool) {
	if c == nil {
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AddEffectOpts(e, includeLHS)
	}
	// CGContext.cpp:386 — effect_stm.add_effect(e) always default include_lhs=false
	c.EffectStm = c.EffectStm.AddEffectOpts(e, false)
}

// MergeParamContext mirrors CGContext::merge_param_context.
// CGContext.cpp:390–394 — add_effect(*param.effect_accum, include_lhs); copy expr_depth.
// Does not merge param.effect_stm (only accum is folded via add_effect).
func (c *CGContext) MergeParamContext(param CGContext, includeLHS bool) {
	if c == nil {
		return
	}
	if param.EffectAccum != nil {
		// CGContext.cpp:392 — add_effect(*param.get_effect_accum(), include_lhs_effects)
		c.AddEffect(*param.EffectAccum, includeLHS)
	}
	// CGContext.cpp:393 — expr_depth = param_cg_context.expr_depth
	c.ExprDepth = param.ExprDepth
}

// FindMustUseArrays mirrors RWDirective::find_must_use_arrays.
// CGContext.cpp:610–624 — unique arrays from must_read and must_write.
// Variable* always live on must-use lists; nil hole fails closed (nil out).
func (rw *RWDirective) FindMustUseArrays() []*ArrayVariable {
	if rw == nil {
		return nil
	}
	out := make([]*ArrayVariable, 0)
	seen := make(map[*Variable]bool)
	add := func(v *Variable) bool {
		if v == nil {
			return false
		}
		if !v.IsArray || seen[v] {
			return true
		}
		seen[v] = true
		if v.AsArray != nil {
			out = append(out, v.AsArray)
		}
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
// Variable* always live on NoReadVars; nil hole fails closed as nonreadable.
func (c CGContext) IsNonReadable(v *Variable) bool {
	if c.RW == nil || v == nil {
		return false
	}
	for _, nr := range c.RW.NoReadVars {
		if nr == nil {
			return true
		}
		if nr.Match(v) {
			return true
		}
	}
	return false
}

// IsNonWritable mirrors CGContext::is_nonwritable.
// CGContext.cpp:133–149 — loose_match no_write_vars; IV bounds.
// Variable* always live on NoWriteVars/IVBounds; nil hole fails closed as nonwritable.
func (c CGContext) IsNonWritable(v *Variable) bool {
	if v == nil {
		return false
	}
	if c.RW != nil {
		for _, nw := range c.RW.NoWriteVars {
			if nw == nil {
				return true
			}
			if nw.LooseMatch(v) || v.LooseMatch(nw) {
				return true
			}
		}
	}
	// not writing to loop IVs (avoid infinite loops)
	for iv := range c.IVBounds {
		if iv == nil {
			return true
		}
		if v.LooseMatch(iv) {
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

// VariablesComplete reports set has no nil Variable* holes.
// Incomplete lists must not invent membership via IsVariableInSet (false past a
// hole drops params/locals from keep sets / lower_block coverage).
func VariablesComplete(set []*Variable) bool {
	for _, x := range set {
		if x == nil {
			return false
		}
	}
	return true
}

// IsVariableInSet reports whether v appears in set (pointer equality).
// Nil slots are never matches for a live v (pointer equality); callers that
// need fail-closed incomplete lists must use VariablesComplete first.
func IsVariableInSet(set []*Variable, v *Variable) bool {
	if v == nil {
		return false
	}
	if !VariablesComplete(set) {
		// incomplete membership is false for the bit — callers that must not
		// invent not-in-set use VariablesComplete and fail closed themselves
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
func (c *CGContext) ReadIndices(v *Variable, facts []*FactPointTo) bool {
	// CGContext.cpp:352+ — always live this + v; no soft invent true on nil
	if c == nil || v == nil {
		return false
	}
	if v.IsArray || v.AsArray != nil {
		av := v.AsArray
		// C++ static_cast ArrayVariable* on isArray; missing AsArray is broken IR
		if av == nil {
			return false
		}
		// Expression::visit_facts reads process CGOptions; no Defaults invent
		opts := ProcessOptions()
		// CGContext.cpp:356–363 — visit each index expression (live Expression*)
		for _, e := range av.IndexExprs {
			// C++ av->get_indices()[i] always non-null; no soft skip
			if e == nil {
				return false
			}
			// work on a context copy so index visits don't clobber caller stm incorrectly
			// (upstream mutates facts_copy; VisitFactsExpression uses GlobalFacts on FM)
			if !VisitFactsExpression(e, c, opts) {
				return false
			}
		}
		if av.FieldVarOf != nil {
			return c.ReadIndices(av.FieldVarOf, facts)
		}
		return true
	}
	if v.IsArrayField() {
		// CGContext.cpp:368–374 — walk to parent array; assert(v) found
		p := v
		for p != nil && !p.IsArray && p.AsArray == nil {
			p = p.FieldVarOf
		}
		// CGContext.cpp:373 — assert(v); no soft invent true when parent missing
		if p == nil {
			return false
		}
		return c.ReadIndices(p, facts)
	}
	return true
}

// ReadVar mirrors CGContext::read_var — force read into accum + stm.
// CGContext.cpp:175–185.
func (c *CGContext) ReadVar(v *Variable) {
	if c == nil || v == nil {
		return
	}
	v = v.GetCollective()
	if c.IsNonReadable(v) {
		// CGContext.cpp:178 — assert(!"attempted read from a nonreadable variable")
		// no soft invent silent skip: set sticky error for ERROR_GUARD callers
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.ReadVar(v)
	}
	c.EffectStm = c.EffectStm.ReadVar(v)
	if c.CurrentFunc != nil && v.IsGlobal() {
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.ReadVar(v)
	}
}

// WriteVar mirrors CGContext::write_var.
// CGContext.cpp:307–317.
func (c *CGContext) WriteVar(v *Variable) {
	if c == nil || v == nil {
		return
	}
	v = v.GetCollective()
	if c.IsNonWritable(v) {
		// CGContext.cpp:310 — assert(!"attempted write to a nonwritable variable")
		// no soft invent silent skip
		SetError(ErrGeneric)
		return
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.WriteVar(v)
	}
	c.EffectStm = c.EffectStm.WriteVar(v)
	if c.CurrentFunc != nil && v.IsGlobal() {
		c.CurrentFunc.FEffect = c.CurrentFunc.FEffect.WriteVar(v)
	}
}

// CheckDerefVolatile mirrors CGContext::check_deref_volatile.
// CGContext.cpp:152–169.
func (c *CGContext) CheckDerefVolatile(v *Variable, derefLevel int, opts Options) bool {
	// CGContext.cpp:153 — assert(v && "nullptr Variable!"); no soft invent true
	if c == nil || v == nil {
		return false
	}
	if !opts.StrictVolatileRule {
		return true
	}
	if !c.EffectContext().IsSideEffectFree() {
		level := derefLevel
		for level > 0 {
			if v.IsVolatileAfterDeref(level) {
				return false
			}
			level--
		}
	}
	if c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.AccessDerefVolatile(v, derefLevel, true)
	}
	c.EffectStm = c.EffectStm.AccessDerefVolatile(v, derefLevel, true)
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
func (c *CGContext) CheckReadVar(v *Variable, facts []*FactPointTo) bool {
	if c == nil || v == nil {
		return false
	}
	if !c.ReadIndices(v, facts) {
		return false
	}
	v = v.GetCollective()
	if IsNonreadableField(v, c.unionFacts()) {
		return false
	}
	if c.IsNonReadable(v) {
		return false
	}
	if c.EffectContext().IsWrittenPartially(v) {
		return false
	}
	if v.IsVolatile() && !c.EffectContext().IsSideEffectFree() {
		return false
	}
	// FactPointTo::is_dangling_ptr uses CGOptions::dead_pointer_dereference_prob()
	// CGOptions::dead_pointer_dereference_prob only (no dual residual knob)
	if v.IsPointer() && IsDanglingPtr(v, facts, ProcessOptions().DeadPointerDerefProb) {
		return false
	}
	c.ReadVar(v)
	return true
}

// CheckWriteVar mirrors CGContext::check_write_var.
// CGContext.cpp:323–349.
func (c *CGContext) CheckWriteVar(v *Variable, facts []*FactPointTo) bool {
	if c == nil || v == nil {
		return false
	}
	if !c.ReadIndices(v, facts) {
		return false
	}
	v = v.GetCollective()
	if c.IsNonWritable(v) || v.IsConst() {
		return false
	}
	eff := c.EffectContext()
	if eff.IsWrittenPartially(v) || eff.IsReadPartially(v) {
		return false
	}
	if v.IsVolatile() && !eff.IsSideEffectFree() {
		return false
	}
	// CGContext.cpp:342–344 + is_dangling_ptr dead_pointer_dereference_prob
	if c.NoDanglingPtr() && v.IsPointer() && IsDanglingPtr(v, facts, ProcessOptions().DeadPointerDerefProb) {
		return false
	}
	c.WriteVar(v)
	return true
}

// ReadPointed mirrors CGContext::read_pointed — recursive pointee reads.
// CGContext.cpp:216–252.
func (c *CGContext) ReadPointed(v *Variable, indirect int, facts []*FactPointTo, opts Options) bool {
	if c == nil || v == nil || indirect <= 0 {
		return false
	}
	var accumCopy *Effect
	if c.EffectAccum != nil {
		cp := *c.EffectAccum
		accumCopy = &cp
	}
	IncrCounter(&dereferenceLevelCnts, indirect)
	allowNull := opts.NullPointerDerefProb > 0
	allowDead := opts.DeadPointerDerefProb > 0
	if !c.ReadIndices(v, facts) {
		return false
	}
	tmp := []*Variable{v.GetCollective()}
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		// nil = incomplete pointees; empty / null/dead disallowed → fail
		if tmp == nil || len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			// Variable* always live; nil hole fails closed
			if pointee == nil {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
			if IsSpecialPtr(pointee) {
				continue
			}
			if !c.CheckReadVar(pointee, facts) {
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
func (c *CGContext) WritePointed(lhs *Lhs, facts []*FactPointTo, opts Options) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		return false
	}
	indirect := lhs.IndirectLevel()
	if indirect <= 0 {
		return false
	}
	var accumCopy *Effect
	if c.EffectAccum != nil {
		cp := *c.EffectAccum
		accumCopy = &cp
	}
	IncrCounter(&dereferenceLevelCnts, indirect)
	if !c.ReadIndices(lhs.Var, facts) {
		return false
	}
	tmp := []*Variable{lhs.Var.GetCollective()}
	allowNull := opts.NullPointerDerefProb > 0
	allowDead := opts.DeadPointerDerefProb > 0
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		if tmp == nil || len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			// Variable* always live; nil hole fails closed
			if pointee == nil {
				if accumCopy != nil && c.EffectAccum != nil {
					*c.EffectAccum = *accumCopy
				}
				return false
			}
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
		}
	}
	return true
}

// VisitFactsExpressionVariable mirrors ExpressionVariable::visit_facts.
// ExpressionVariable.cpp:237–274.
func (c *CGContext) VisitFactsExpressionVariable(e *Expression, opts Options) bool {
	if c == nil || e == nil || e.Var == nil {
		return false
	}
	facts := c.pointToFacts()
	deref := e.IndirectLevel()
	v := e.Var
	if deref > 0 {
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			return false
		}
		if !c.CheckReadVar(v, facts) {
			return false
		}
		if !c.ReadPointed(v, deref, facts, opts) {
			return false
		}
		return c.CheckDerefVolatile(v, deref, opts)
	}
	if deref < 0 {
		// address-of: forbid bitfields
		if v.IsBitfield {
			return false
		}
		return true
	}
	return c.CheckReadVar(v, facts)
}

// AllowVolatile mirrors CGContext::allow_volatile.
// CGContext.cpp:517–518 — only when effect_context is side-effect free.
func (c CGContext) AllowVolatile() bool {
	return c.EffectContext().IsSideEffectFree()
}

// AllowConst mirrors CGContext::allow_const — const only for non-WRITE access.
// CGContext.cpp:521–523.
func (c CGContext) AllowConst(access Access) bool {
	return access != AccessWrite
}

// AcceptType mirrors CGContext::accept_type.
// CGContext.cpp:525–528 — reject volatile aggregates when not SE-free.
func (c CGContext) AcceptType(t *Type) bool {
	if t == nil {
		return true
	}
	return c.EffectContext().IsSideEffectFree() || !t.IsVolatileStructUnion()
}

// InConflict mirrors CGContext::in_conflict — callee effect vs current context.
// CGContext.cpp:531–564.
// Variable* always live in effect lists; nil hole fails closed as conflict
// (no invent skip as conflict-free incomplete effect).
func (c CGContext) InConflict(eff Effect) bool {
	for _, v := range eff.ReadVars() {
		if v == nil {
			return true
		}
		if c.IsNonReadable(v) {
			return true
		}
		if c.EffectContext().IsWrittenPartially(v) {
			return true
		}
		if v.IsVolatile() && !c.EffectContext().IsSideEffectFree() {
			return true
		}
	}
	for _, v := range eff.WrittenVars() {
		if v == nil {
			return true
		}
		if c.IsNonWritable(v) || v.IsConst() {
			return true
		}
		ctx := c.EffectContext()
		if ctx.IsWrittenPartially(v) || ctx.IsReadPartially(v) {
			return true
		}
		if v.IsVolatile() && !ctx.IsSideEffectFree() {
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
	if v == nil {
		return false
	}
	// CGContext.cpp:493–494 — get_current_block(); assert(b)
	// no soft invent frame via call_chain only when curr_blk missing
	b := c.CurrentBlock()
	if b == nil {
		return false
	}
	if !b.StackScanComplete() {
		return false
	}
	if v.IsVisibleLocal(b) {
		return true
	}
	for _, cb := range c.CallChain {
		// Block* always live on call_chain; nil / incomplete stack fails closed
		// (no invent skip hole and still match a later frame)
		if cb == nil || !cb.StackScanComplete() {
			return false
		}
		if v.IsVisibleLocal(cb) {
			return true
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
		return false
	}
	for _, cb := range c.CallChain {
		if cb == nil || !cb.StackScanComplete() {
			return false
		}
	}
	return true
}

// FindReachableFrameVars mirrors CGContext::find_reachable_frame_vars.
// CGContext.cpp:566–578 — pointees that are frame locals.
// Fact* always live; incomplete map/pointees or incomplete frame stacks fail closed
// (nil out — no invent empty frame set when IsFrameVar is false past a hole).
// Complete empty returns non-nil empty slice (no invent nil==incomplete).
func (c CGContext) FindReachableFrameVars(facts []*FactPointTo) []*Variable {
	if !FactsComplete(facts) {
		return nil
	}
	if !c.frameStacksComplete() {
		return nil
	}
	out := make([]*Variable, 0)
	seen := map[*Variable]bool{}
	for _, f := range facts {
		for _, p := range f.PointTo {
			// pointee Variable* always live in point-to sets (specials are non-nil)
			if p == nil {
				return nil
			}
			if IsSpecialPtr(p) || seen[p] {
				continue
			}
			if c.IsFrameVar(p) {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// GetExternalNoReadsWrites mirrors CGContext::get_external_no_reads_writes.
// CGContext.cpp:581–607 — globals + frame_vars from RW + global IVs as no-write.
// Variable* always live on RW/IV lists; nil hole fails closed (nil,nil — no invent partial).
func (c CGContext) GetExternalNoReadsWrites(frameVars []*Variable) (noReads, noWrites []*Variable) {
	inFrame := func(v *Variable) bool {
		if v == nil {
			return false
		}
		if v.IsGlobal() {
			return true
		}
		return IsVariableInSet(frameVars, v)
	}
	if c.RW != nil {
		for _, v := range c.RW.NoReadVars {
			if v == nil {
				return nil, nil
			}
			if inFrame(v) {
				noReads = append(noReads, v)
			}
		}
		for _, v := range c.RW.NoWriteVars {
			if v == nil {
				return nil, nil
			}
			if inFrame(v) {
				noWrites = append(noWrites, v)
			}
		}
	}
	// convert global / frame IVs into non-writables
	for iv := range c.IVBounds {
		if iv == nil {
			return nil, nil
		}
		if inFrame(iv) {
			noWrites = append(noWrites, iv)
		}
	}
	return noReads, noWrites
}

// BuildCalleeRWDirective builds RW for generate_body_with_known_params path.
// Function.cpp:675–681 — inherit external no-read/write from caller.
// Incomplete frame facts fail closed: inherit full caller NoRead/NoWrite without
// inventing nil (no restrictions) from incomplete reachable frames.
func (c CGContext) BuildCalleeRWDirective(facts []*FactPointTo) *RWDirective {
	frame := c.FindReachableFrameVars(facts)
	if frame == nil {
		// incomplete facts — fail closed full external lists (no invent empty RW)
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
	// GetExternal nil,nil = incomplete RW/IV lists
	if nr == nil && nw == nil && c.RW != nil {
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
func (c *CGContext) PtrModifiedInRhs(lhs *Lhs, facts []*FactPointTo) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		return false
	}
	indirect := lhs.IndirectLevel()
	// Lhs.cpp:243 — assert(indirect > 0); non-deref LHS is not this path
	if indirect <= 0 {
		return false
	}
	// if the pointer variable itself was written by RHS
	if c.EffectStm.IsWritten(lhs.Var) {
		return true
	}
	tmp := []*Variable{lhs.Var.GetCollective()}
	// only intermediate pointer levels (not ultimate pointees)
	for indirect > 1 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		// nil = incomplete pointees
		if tmp == nil {
			return true
		}
		for _, v := range tmp {
			if v == nil {
				return true
			}
			if c.EffectStm.IsWritten(v) {
				return true
			}
		}
	}
	return false
}

// VisitFactsLhs mirrors Lhs::visit_facts.
// Incomplete Lhs type IR fails closed (no invent bare non-deref visit success).
// Lhs.cpp:301–356 — compound read-first; curr_rhs overlap; write/write_pointed.
func (c *CGContext) VisitFactsLhs(lhs *Lhs, opts Options) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		return false
	}
	facts := c.pointToFacts()
	v := lhs.Var
	// compound assign: validate as read first (Lhs.cpp:307–311)
	if lhs.CompoundAssign {
		ev := &Expression{Term: TermVariable, Var: v, ExprType: lhs.GetType()}
		if !c.VisitFactsExpressionVariable(ev, opts) {
			return false
		}
	}
	// Lhs.cpp:313–315 — visit array indices
	if !lhs.VisitIndices(c, opts) {
		return false
	}
	// avoid overlapping union field assign a.x = a.y (Lhs.cpp:318–328)
	if c.CurrRHS != nil {
		lhsExpr := LhsAsExpression(lhs)
		// Lhs always live for this check; incomplete RHS subexps fail closed
		if lhsExpr == nil {
			return false
		}
		// complete get_eval_to_subexps always ≥1 entry; nil/empty fails closed
		// (no invent skip overlap check as success past incomplete RHS)
		subs := GetEvalToSubexps(c.CurrRHS)
		if len(subs) == 0 {
			return false
		}
		for _, sub := range subs {
			// Expression* always live in eval list; no invent skip nil holes
			if sub == nil {
				return false
			}
			if sub.Term == TermVariable || sub.Term == TermLhs {
				if HaveOverlappingFields(sub, lhsExpr, facts) {
					return false
				}
			}
		}
	}
	// incomplete Lhs type IR must not invent non-deref level-0 visit success
	deref, ok := lhs.IndirectLevelComplete()
	if !ok {
		return false
	}
	valid := false
	if deref > 0 {
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			return false
		}
		// Lhs.cpp:337–339 — pointer modified in RHS
		if c.PtrModifiedInRhs(lhs, facts) {
			return false
		}
		// read the pointer itself then write pointees
		if !c.CheckReadVar(v, facts) {
			return false
		}
		if !c.WritePointed(lhs, facts, opts) {
			return false
		}
		if !c.CheckDerefVolatile(v, deref, opts) {
			return false
		}
		valid = true
	} else {
		valid = c.CheckWriteVar(v, facts)
	}
	// Lhs.cpp:348–351 — set_lhs_write_vars from write_vars on accum
	if valid && c.EffectAccum != nil {
		*c.EffectAccum = c.EffectAccum.SetLhsWriteVarsFromWritten()
	}
	return valid
}
