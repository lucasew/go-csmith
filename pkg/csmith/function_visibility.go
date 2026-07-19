// Upstream: Function.cpp is_var_on_stack / is_var_visible / is_var_oos.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// StackScanComplete reports Param + LocalVars parent-chain have no nil holes.
// Incomplete lists must not invent "not on stack" membership for mark_func_end /
// remove_function_local_facts (false from a hole-shorted scan).
// Function nil is incomplete (false); does not SetError — predicate only.
func (f *Function) StackScanComplete(stParent *Block) bool {
	if f == nil {
		return false
	}
	for _, p := range f.Param {
		if p == nil {
			return false
		}
	}
	for b := stParent; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == nil {
				return false
			}
		}
	}
	return true
}

// IsVarOnStack mirrors Function::is_var_on_stack(var, stm).
// Function.cpp:185–201 — param or local in stm's parent block chain.
// stParent is the statement's parent block (Stmt has no Parent field; pass enclosing block).
// Incomplete Function/Variable/Param/LocalVars sticky false (no invent not-on-stack
// / soft re-pick past holes).
func (f *Function) IsVarOnStack(v *Variable, stParent *Block) bool {
	// Function + Variable always live; sticky incomplete no invent not-on-stack
	if f == nil || v == nil {
		SetError(ErrGeneric)
		return false
	}
	if !f.StackScanComplete(stParent) {
		// incomplete Param/LocalVars sticky fail closed not-on-stack
		SetError(ErrGeneric)
		return false
	}
	for _, p := range f.Param {
		// Param live after StackScanComplete; nil hole already sticky above
		if p == nil {
			SetError(ErrGeneric)
			return false
		}
		if p.Match(v) {
			// residual ERROR sticky — no invent on-stack true past Match hole
			if HasError() {
				return false
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue then true later past Match hole
		if HasError() {
			return false
		}
	}
	for b := stParent; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == nil {
				SetError(ErrGeneric)
				return false
			}
			if loc == v {
				return true
			}
			if loc.Match(v) {
				// residual ERROR sticky — no invent on-stack true past Match hole
				if HasError() {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue then true later past Match hole
			if HasError() {
				return false
			}
		}
	}
	return false
}

// IsVarVisible mirrors Function::is_var_visible.
// Function.cpp:204–205 — global or on stack at statement.
// Incomplete Variable sticky false; incomplete stack via IsVarOnStack sticky.
func (f *Function) IsVarVisible(v *Variable, stParent *Block) bool {
	// Variable always live; sticky incomplete no invent not-visible soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.IsGlobal() {
		// residual ERROR sticky — no invent visible-true past IsGlobal hole
		if HasError() {
			return false
		}
		return true
	}
	// residual ERROR sticky — no invent not-global soft-skip past IsGlobal hole
	if HasError() {
		return false
	}
	// nil Function for non-global sticky not-visible (IsVarOnStack also stickies)
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	ok := f.IsVarOnStack(v, stParent)
	// residual ERROR sticky — no invent visible/not-visible soft-skip past stack hole
	if HasError() {
		return false
	}
	return ok
}

// IsVarOOS mirrors Function::is_var_oos.
// Function.cpp:214–224 — not visible at stm but is a local of this function.
// Incomplete Function/Variable/stack/Blocks sticky true OOS (no invent not-OOS
// / soft re-pick past holes).
func (f *Function) IsVarOOS(v *Variable, stParent *Block) bool {
	// Function + Variable always live; sticky incomplete OOS fail closed
	if f == nil || v == nil {
		SetError(ErrGeneric)
		// nil Function: cannot be OOS local of unknown func — fail closed false
		// would invent not-OOS; true OOS is safer for dead marking, but C++ has
		// live Function*. Sticky false for nil shell (assert path).
		return false
	}
	if !f.StackScanComplete(stParent) {
		// incomplete stack sticky OOS (no invent not-OOS when scan short-circuits)
		SetError(ErrGeneric)
		return true
	}
	if f.IsVarVisible(v, stParent) {
		// residual ERROR sticky — no invent not-OOS past IsVarVisible hole
		if HasError() {
			return true
		}
		return false
	}
	// residual ERROR sticky — no invent soft-continue Blocks scan past IsVarVisible hole
	if HasError() {
		return true
	}
	for _, b := range f.Blocks {
		if b == nil {
			// incomplete Blocks sticky OOS
			SetError(ErrGeneric)
			return true
		}
		for _, loc := range b.LocalVars {
			if loc == nil {
				SetError(ErrGeneric)
				return true
			}
			if loc.Match(v) || loc == v {
				// residual ERROR sticky — no invent OOS-true past Match hole
				if HasError() {
					return true
				}
				return true
			}
			if HasError() {
				return true
			}
		}
	}
	return false
}

// AddBackReturnFacts mirrors Statement::add_back_return_facts / Block walk.
// Statement.cpp:525–537 — merge map_facts_out of every return into facts.
// Incomplete map_facts_out / mid-join / nil block hole fails closed sticky:
// *facts = IncompleteFactSlice() + SetError and the walk stops (no invent keep
// merging later returns after a failed merge / soft re-pick past wiped facts).
// Returns false when incomplete (*facts wiped) so callers do not invent success
// via FactsComplete(nil)==true after a fail-closed wipe.
func AddBackReturnFacts(b *Block, fm *FactMgr, facts *[]*FactPointTo) bool {
	if b == nil || fm == nil || facts == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
		}
		return false
	}
	return addBackReturnFactsBlock(b, fm, facts)
}

// addBackReturnFactsBlock returns false when the accumulator is fail-closed incomplete.
func addBackReturnFactsBlock(b *Block, fm *FactMgr, facts *[]*FactPointTo) bool {
	if b == nil || fm == nil || facts == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
		}
		return false
	}
	for i := range b.Stmts {
		if !addBackReturnFactsStmt(&b.Stmts[i], fm, facts) {
			return false
		}
	}
	return true
}

// addBackReturnFactsStmt returns false when facts must stay fail-closed incomplete.
func addBackReturnFactsStmt(st *Stmt, fm *FactMgr, facts *[]*FactPointTo) bool {
	if st == nil || facts == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
		}
		return false
	}
	if st.Kind == StmtReturn {
		// Statement.cpp:528 — merge_facts(facts, map_facts_out[this])
		// GetMapFactsOut: StmID 0 IncompleteFactSlice (no invent empty-complete)
		out := fm.GetMapFactsOut(st.StmID)
		if !FactsComplete(out) || !FactsComplete(*facts) {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return false
		}
		// MergeFacts clears *facts sticky on incomplete mid-join
		_ = MergeFacts(facts, out)
		if !FactsComplete(*facts) {
			*facts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		return true
	}
	// Statement.cpp:530–535 — get_blocks then recurse (Then/Else for if/for)
	for _, blk := range GetBlocksStmt(st) {
		// Block* always live from get_blocks; nil hole fails closed sticky
		if blk == nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return false
		}
		if !addBackReturnFactsBlock(blk, fm, facts) {
			return false
		}
	}
	return true
}

// UpdateFactsForOOSVars mirrors FactMgr::update_facts_for_oos_vars.
// FactMgr.cpp:141–170 — drop facts for vars; mark pointees as garbage/dead.
// Fact* always live; nil hole fails closed (nil facts, no invent clean filter).
// Variable* in OOS list always live with complete FieldVars; nil / incomplete
// FieldVars fail closed (nil facts — no invent leave field pointees live when
// MarkDeadVar cannot see past a FieldVars hole).
// facts always live; sticky (no invent soft-skip OOS cleanup past hole).
// Empty vars is complete no-op.
func UpdateFactsForOOSVars(vars []*Variable, facts *[]*FactPointTo) {
	if facts == nil {
		SetError(ErrGeneric)
		return
	}
	if len(vars) == 0 {
		return
	}
	if !FactsComplete(*facts) {
		// incomplete map/vars fail closed sticky (no invent soft re-pick OOS cleanup)
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	for _, v := range vars {
		if v == nil || !v.FieldVarsComplete() {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
	}
	// remove facts whose subject matches an OOS var
	out := make([]*FactPointTo, 0, len(*facts))
	for _, f := range *facts {
		drop := false
		for _, v := range vars {
			if v.Match(f.Var) {
				// residual ERROR sticky — no invent drop-true past Match hole
				if HasError() {
					*facts = IncompleteFactSlice()
					return
				}
				drop = true
				break
			}
			// residual ERROR sticky — no invent soft-continue keep later past Match residual
			// (Type-nil Match ERROR+false soft invents not-drop then keep fact)
			if HasError() {
				*facts = IncompleteFactSlice()
				return
			}
		}
		if !drop {
			out = append(out, f)
		}
	}
	// mark pointees that are OOS as dead/garbage
	for i, f := range out {
		cur := f
		for _, v := range vars {
			if nf := cur.MarkDeadVar(v); nf != nil {
				// residual ERROR sticky — no invent mark-dead success past MarkDeadVar hole
				if HasError() {
					*facts = IncompleteFactSlice()
					return
				}
				cur = nf
			} else if HasError() {
				// residual ERROR sticky — no invent soft-continue later mark past MarkDead residual
				*facts = IncompleteFactSlice()
				return
			}
		}
		out[i] = cur
	}
	*facts = out
}

// OutputCommentLine mirrors OutputMgr::output_comment_line.
// OutputMgr.cpp:314–320 — "/* comment */\n" unless quiet/concise.
// empty comment is incomplete IR — no invent "/*  */" shell (still emits "\n" when quiet/concise).
func OutputCommentLine(comment string, quiet, concise bool) string {
	if quiet || concise {
		return "\n"
	}
	// empty comment sticky (no invent "/*  */" / soft re-pick blank shell)
	if comment == "" {
		SetError(ErrGeneric)
		return ""
	}
	return "/* " + comment + " */\n"
}
