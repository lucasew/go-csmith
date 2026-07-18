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
	// CurrRHS mirrors curr_rhs — RHS expression when validating LHS (assign).
	// CGContext.h:178.
	CurrRHS *Expression
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

// MergeParamContext mirrors CGContext::merge_param_context.
// CGContext.cpp:390–394 — fold param accum into this; copy expr_depth.
func (c *CGContext) MergeParamContext(param CGContext, includeLHS bool) {
	if c == nil {
		return
	}
	_ = includeLHS
	if param.EffectAccum != nil {
		if c.EffectAccum != nil {
			*c.EffectAccum = c.EffectAccum.AddEffect(*param.EffectAccum)
		} else {
			// fold into effect_context via EffectStm
			c.EffectStm = c.EffectStm.AddEffect(*param.EffectAccum)
		}
	}
	c.ExprDepth = param.ExprDepth
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

// IsVariableInSet reports whether v appears in set (pointer equality).
func IsVariableInSet(set []*Variable, v *Variable) bool {
	if v == nil {
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
// CGContext.cpp:352–380 — no Indices on our ArrayVariable → always true for now.
func (c *CGContext) ReadIndices(v *Variable, facts []*FactPointTo) bool {
	_ = c
	_ = facts
	if v == nil {
		return true
	}
	// ArrayVariable indices as Expressions not yet stored; skip walk.
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
		// upstream asserts; we skip recording
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
	if v == nil {
		return true
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
	if v.IsPointer() && IsDanglingPtr(v, facts, 0) {
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
	if c.NoDanglingPtr() && v.IsPointer() && IsDanglingPtr(v, facts, 0) {
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
	allowDead := opts.DanglingPtrDerefProb > 0
	if !c.ReadIndices(v, facts) {
		return false
	}
	tmp := []*Variable{v.GetCollective()}
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		if len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			if pointee == nil || IsSpecialPtr(pointee) {
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
	allowDead := opts.DanglingPtrDerefProb > 0
	for indirect > 0 {
		indirect--
		tmp = MergePointeesOfPointers(tmp, facts)
		if len(tmp) == 0 ||
			(!allowNull && IsVariableInSet(tmp, NullPtr)) ||
			(!allowDead && IsVariableInSet(tmp, GarbagePtr)) {
			if accumCopy != nil && c.EffectAccum != nil {
				*c.EffectAccum = *accumCopy
			}
			return false
		}
		for _, pointee := range tmp {
			if pointee == nil || IsSpecialPtr(pointee) {
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
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DanglingPtrDerefProb) {
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
func (c CGContext) InConflict(eff Effect) bool {
	for _, v := range eff.ReadVars() {
		if v == nil {
			continue
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
			continue
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
func (c CGContext) IsFrameVar(v *Variable) bool {
	if v == nil {
		return false
	}
	if b := c.CurrentBlock(); b != nil && v.IsVisibleLocal(b) {
		return true
	}
	for _, b := range c.CallChain {
		if b != nil && v.IsVisibleLocal(b) {
			return true
		}
	}
	return false
}

// FindReachableFrameVars mirrors CGContext::find_reachable_frame_vars.
// CGContext.cpp:566–578 — pointees that are frame locals.
func (c CGContext) FindReachableFrameVars(facts []*FactPointTo) []*Variable {
	var out []*Variable
	seen := map[*Variable]bool{}
	for _, f := range facts {
		if f == nil {
			continue
		}
		for _, p := range f.PointTo {
			if p == nil || IsSpecialPtr(p) || seen[p] {
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
			if inFrame(v) {
				noReads = append(noReads, v)
			}
		}
		for _, v := range c.RW.NoWriteVars {
			if inFrame(v) {
				noWrites = append(noWrites, v)
			}
		}
	}
	// convert global / frame IVs into non-writables
	for iv := range c.IVBounds {
		if inFrame(iv) {
			noWrites = append(noWrites, iv)
		}
	}
	return noReads, noWrites
}

// BuildCalleeRWDirective builds RW for generate_body_with_known_params path.
// Function.cpp:675–681 — inherit external no-read/write from caller.
func (c CGContext) BuildCalleeRWDirective(facts []*FactPointTo) *RWDirective {
	frame := c.FindReachableFrameVars(facts)
	nr, nw := c.GetExternalNoReadsWrites(frame)
	if len(nr) == 0 && len(nw) == 0 {
		return nil
	}
	return &RWDirective{NoReadVars: nr, NoWriteVars: nw}
}

// VisitFactsLhs mirrors Lhs::visit_facts core checks (without index walk).
// Lhs.cpp:301+ subset — check_write_var or write_pointed + deref volatile.
func (c *CGContext) VisitFactsLhs(lhs *Lhs, opts Options) bool {
	if c == nil || lhs == nil || lhs.Var == nil {
		return false
	}
	facts := c.pointToFacts()
	deref := lhs.IndirectLevel()
	v := lhs.Var
	if deref > 0 {
		if !IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DanglingPtrDerefProb) {
			return false
		}
		// read the pointer itself then write pointees
		if !c.CheckReadVar(v, facts) {
			return false
		}
		if !c.WritePointed(lhs, facts, opts) {
			return false
		}
		return c.CheckDerefVolatile(v, deref, opts)
	}
	return c.CheckWriteVar(v, facts)
}
