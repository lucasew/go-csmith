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
		// residual ERROR sticky — no invent soft not-on-stack past StackScan residual
		if !HasError() {
			SetError(ErrGeneric)
		}
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
	// Function.cpp:192–198 — find_variable_in_set(b->local_vars, var) uses
	// Variable::match (Variable.cpp:103–111), not pointer identity.
	// is_variable_in_set uses ==; find_variable_in_set uses match (aggregate
	// has_field_var). So fields of stack aggregates are on-stack: mark_func_end
	// on eReturn set_fact_out (FactMgr.cpp:269–271) garbage field pointees.
	// Seed-30: g_113 held live l_531.f0 after func_69 return map_out because
	// locals used == only; upstream match(l_531, l_531.f0) → on-stack → garbage.
	for b := stParent; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == nil {
				SetError(ErrGeneric)
				return false
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
		// residual ERROR sticky — no invent soft-OOS past StackScan residual
		if !HasError() {
			SetError(ErrGeneric)
		}
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
	// Function.cpp:217–220 — find_variable_in_set(blocks[i]->local_vars, var)
	// uses Variable::match (Variable.cpp:103–111, 254–258):
	//   if type && v->type && aggregate: (this==v) || has_field_var(v)
	//   else: this==v
	// Soft invent used pointer identity only so field pointees of later-sibling
	// locals (l_298.f0) were not OOS at earlier for dest → map_facts_out[goto]
	// kept live field instead of garbage (seed 17809409409875472624). Do not call
	// Variable.Match here: it stickies Type-nil (C++ match falls through to ==).
	// Fields still in scope return false above via IsVarOnStack Match.
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
			if loc == v {
				return true
			}
			// Variable.cpp:254–258 — aggregate match includes fields when both typed
			if loc.Type != nil && v.Type != nil {
				agg := loc.Type.IsAggregate()
				if HasError() {
					return true
				}
				if agg {
					if loc.HasFieldVar(v) {
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
		}
	}
	return false
}

// AddBackReturnFacts mirrors Statement::add_back_return_facts / Block walk.
// Statement.cpp:525–537 — merge_facts(facts, map_facts_out[return]) for the full
// FactVec (ePointTo + eUnionWrite). Soft invent was point-to-only: return outs
// that wrote a union field (last_written=1) never joined the live lattice, so
// IsNonreadableField kept f0 eligible after early-return paths (seed-123
// ChooseOKVar n=51 vs UP n=50 → g_831 vs g_1248).
// Incomplete map_facts_out / mid-join / nil block hole fails closed sticky:
// both partitions wiped + SetError; walk stops (no invent keep merging later
// returns after a failed merge).
// Returns false when incomplete so callers do not invent success via
// FactsComplete(nil)==true after a fail-closed wipe.
func AddBackReturnFacts(b *Block, fm *FactMgr, facts *[]*FactPointTo, unions *[]*FactUnion) bool {
	if b == nil || fm == nil || facts == nil || unions == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
		}
		if unions != nil {
			*unions = IncompleteUnionFactSlice()
		}
		SetError(ErrGeneric)
		return false
	}
	return addBackReturnFactsBlock(b, fm, facts, unions)
}

// addBackReturnFactsBlock returns false when the accumulator is fail-closed incomplete.
func addBackReturnFactsBlock(b *Block, fm *FactMgr, facts *[]*FactPointTo, unions *[]*FactUnion) bool {
	if b == nil || fm == nil || facts == nil || unions == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
		}
		if unions != nil {
			*unions = IncompleteUnionFactSlice()
		}
		SetError(ErrGeneric)
		return false
	}
	for i := range b.Stmts {
		if !addBackReturnFactsStmt(&b.Stmts[i], fm, facts, unions) {
			return false
		}
	}
	return true
}

// addBackReturnFactsStmt returns false when facts must stay fail-closed incomplete.
func addBackReturnFactsStmt(st *Stmt, fm *FactMgr, facts *[]*FactPointTo, unions *[]*FactUnion) bool {
	if st == nil || facts == nil || unions == nil {
		if facts != nil {
			*facts = IncompleteFactSlice()
		}
		if unions != nil {
			*unions = IncompleteUnionFactSlice()
		}
		SetError(ErrGeneric)
		return false
	}
	if st.Kind == StmtReturn {
		// Statement.cpp:528 — merge_facts(facts, map_facts_out[this]) full FactVec
		// GetMapFactsOut / GetMapUnionFactsOut: StmID unset → Incomplete
		out := fm.GetMapFactsOut(st.StmID)
		outU := fm.GetMapUnionFactsOut(st.StmID)
		if !FactsComplete(out) || !FactsComplete(*facts) ||
			!UnionFactsComplete(outU) || !UnionFactsComplete(*unions) {
			*facts = IncompleteFactSlice()
			*unions = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			return false
		}
		_ = MergeFacts(facts, out)
		if !FactsComplete(*facts) {
			*facts = IncompleteFactSlice()
			*unions = IncompleteUnionFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		// eUnionWrite half of merge_facts (Fact.cpp:192–199 + FactUnion::join)
		for _, nf := range outU {
			if nf == nil {
				*facts = IncompleteFactSlice()
				*unions = IncompleteUnionFactSlice()
				SetError(ErrGeneric)
				return false
			}
			*unions = MergeUnionFact(*unions, nf)
			if !UnionFactsComplete(*unions) {
				*facts = IncompleteFactSlice()
				*unions = IncompleteUnionFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
		return true
	}
	// Statement.cpp:530–535 — get_blocks then recurse (Then/Else for if/for)
	for _, blk := range GetBlocksStmt(st) {
		// Block* always live from get_blocks; nil hole fails closed sticky
		if blk == nil {
			*facts = IncompleteFactSlice()
			*unions = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			return false
		}
		if !addBackReturnFactsBlock(blk, fm, facts, unions) {
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

// DropFactSubjectsByVars removes point-to facts whose subject is in vars
// (pointer identity). Does not MarkDeadVar pointees.
//
// Used to keep map_facts_in[block] as a true entry env without the block's own
// LocalVars. Back-edge merge of goto outs can reintroduce body locals into
// current_inputs before set_fact_in (Block.cpp:531–557); StatementFor
// post_loop / visit then merge_jump_facts invents garbage for those subjects
// because break map_facts_out correctly strips them via remove_loop_local
// (seed-7 for 640: l_1402 in map_in + body LocalVars, missing from break out).
// FactMgr.cpp:257–262 remove_loop_local + 575–579 invent-garbage path.
func DropFactSubjectsByVars(facts []*FactPointTo, vars []*Variable) []*FactPointTo {
	if len(vars) == 0 {
		return facts
	}
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
	}
	// incomplete vars list fail closed
	for _, v := range vars {
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteFactSlice()
		}
	}
	drop := make(map[*Variable]bool, len(vars))
	for _, v := range vars {
		drop[v] = true
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
		if f == nil {
			SetError(ErrGeneric)
			return IncompleteFactSlice()
		}
		if f.Var != nil && drop[f.Var] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// DropUnionSubjectsByVars removes eUnionWrite facts whose subject is in vars.
func DropUnionSubjectsByVars(facts []*FactUnion, vars []*Variable) []*FactUnion {
	if len(vars) == 0 {
		return facts
	}
	if !UnionFactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	for _, v := range vars {
		if v == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
	}
	drop := make(map[*Variable]bool, len(vars))
	for _, v := range vars {
		drop[v] = true
	}
	out := make([]*FactUnion, 0, len(facts))
	for _, f := range facts {
		if f == nil {
			SetError(ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if f.Var != nil && drop[f.Var] {
			continue
		}
		out = append(out, f)
	}
	return out
}

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
