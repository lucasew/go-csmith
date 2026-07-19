// Upstream: Function.cpp is_var_on_stack / is_var_visible / is_var_oos.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// StackScanComplete reports Param + LocalVars parent-chain have no nil holes.
// Incomplete lists must not invent "not on stack" membership for mark_func_end /
// remove_function_local_facts (false from a hole-shorted scan).
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
// Variable* always live on Param/LocalVars; nil hole fails closed as false for the
// membership bit — callers that need fail-closed OOS/mark use StackScanComplete.
func (f *Function) IsVarOnStack(v *Variable, stParent *Block) bool {
	if f == nil || v == nil {
		return false
	}
	if !f.StackScanComplete(stParent) {
		return false
	}
	for _, p := range f.Param {
		if p.Match(v) {
			return true
		}
	}
	for b := stParent; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == v || loc.Match(v) {
				return true
			}
		}
	}
	return false
}

// IsVarVisible mirrors Function::is_var_visible.
// Function.cpp:204–205 — global or on stack at statement.
func (f *Function) IsVarVisible(v *Variable, stParent *Block) bool {
	if v == nil {
		return false
	}
	if v.IsGlobal() {
		return true
	}
	return f.IsVarOnStack(v, stParent)
}

// IsVarOOS mirrors Function::is_var_oos.
// Function.cpp:214–224 — not visible at stm but is a local of this function.
// Block*/Variable* always live; nil holes fail closed as OOS (no invent not-OOS).
func (f *Function) IsVarOOS(v *Variable, stParent *Block) bool {
	if f == nil || v == nil {
		return false
	}
	if f.IsVarVisible(v, stParent) {
		return false
	}
	for _, b := range f.Blocks {
		if b == nil {
			return true
		}
		for _, loc := range b.LocalVars {
			if loc == nil {
				return true
			}
			if loc.Match(v) || loc == v {
				return true
			}
		}
	}
	return false
}

// AddBackReturnFacts mirrors Statement::add_back_return_facts / Block walk.
// Statement.cpp:525–537 — merge map_facts_out of every return into facts.
// Incomplete map_facts_out / mid-join / nil block hole fails closed: *facts = nil
// and the walk stops (no invent keep merging later returns after a failed merge).
func AddBackReturnFacts(b *Block, fm *FactMgr, facts *[]*FactPointTo) {
	if b == nil || fm == nil || facts == nil {
		return
	}
	_ = addBackReturnFactsBlock(b, fm, facts)
}

// addBackReturnFactsBlock returns false when the accumulator is fail-closed incomplete.
func addBackReturnFactsBlock(b *Block, fm *FactMgr, facts *[]*FactPointTo) bool {
	if b == nil || fm == nil || facts == nil {
		if facts != nil {
			*facts = nil
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
			*facts = nil
		}
		return false
	}
	if st.Kind == StmtReturn {
		// Statement.cpp:528 — merge_facts(facts, map_facts_out[this])
		// C++ map[] always; missing → empty merge; incomplete fails closed
		out := fm.MapFactsOut[st.StmID]
		if !FactsComplete(out) || !FactsComplete(*facts) {
			*facts = nil
			return false
		}
		// MergeFacts clears *facts on incomplete mid-join — fail closed
		_ = MergeFacts(facts, out)
		if !FactsComplete(*facts) {
			*facts = nil
			return false
		}
		return true
	}
	// Statement.cpp:530–535 — get_blocks then recurse (Then/Else for if/for)
	for _, blk := range GetBlocksStmt(st) {
		// Block* always live from get_blocks; nil hole fails closed
		if blk == nil {
			*facts = nil
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
// Variable* in OOS list always live; nil hole fails closed same way.
func UpdateFactsForOOSVars(vars []*Variable, facts *[]*FactPointTo) {
	if facts == nil || len(vars) == 0 {
		return
	}
	if !FactsComplete(*facts) {
		*facts = nil
		return
	}
	for _, v := range vars {
		if v == nil {
			*facts = nil
			return
		}
	}
	// remove facts whose subject matches an OOS var
	out := make([]*FactPointTo, 0, len(*facts))
	for _, f := range *facts {
		drop := false
		for _, v := range vars {
			if v.Match(f.Var) {
				drop = true
				break
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
				cur = nf
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
	if comment == "" {
		return ""
	}
	return "/* " + comment + " */\n"
}
