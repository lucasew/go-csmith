// Upstream: Statement.cpp shortcut_analysis / same_facts / contains_unfixed_goto.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Shortcut result codes (Statement::shortcut_analysis).
const (
	// ShortcutOK reused previous analysis (return 0).
	ShortcutOK = 0
	// ShortcutConflict effect conflict (return 1).
	ShortcutConflict = 1
	// ShortcutNone no shortcut available (return 2).
	ShortcutNone = 2
)

// SameFacts mirrors same_facts for FactPointTo slices.
// Fact.cpp:237–246 — same size and each fact of facts1 found in facts2.
func SameFacts(a, b []*FactPointTo) bool {
	if len(a) != len(b) {
		return false
	}
	for _, f := range a {
		if FindFact(b, f) < 0 {
			return false
		}
	}
	return true
}

// FindFact mirrors find_fact — equal fact in vector, or -1.
// Fact.cpp find_fact by equal().
func FindFact(facts []*FactPointTo, want *FactPointTo) int {
	if want == nil {
		return -1
	}
	for i, f := range facts {
		if f != nil && f.Equal(want) {
			return i
		}
	}
	return -1
}

// SubsetFacts mirrors subset_facts — each f1 is implied by related f2.
// Fact.cpp:249–260.
func SubsetFacts(a, b []*FactPointTo) bool {
	if len(a) != len(b) {
		// upstream requires same size
		return false
	}
	for _, f1 := range a {
		if f1 == nil {
			continue
		}
		f2 := FindRelatedPointTo(b, f1.Var)
		if f2 == nil || !f2.Imply(f1) {
			return false
		}
	}
	return true
}

// IsCtrlStmt mirrors Statement::is_ctrl_stmt — break/continue/goto/return.
func IsCtrlStmt(st *Stmt) bool {
	if st == nil {
		return false
	}
	switch st.Kind {
	case StmtBreak, StmtContinue, StmtGoto, StmtReturn:
		return true
	default:
		return false
	}
}

// ContainsStmt reports whether root statement tree contains target by StmID.
func ContainsStmt(root, target *Stmt) bool {
	if root == nil || target == nil {
		return false
	}
	if root.StmID != 0 && root.StmID == target.StmID {
		return true
	}
	if root.Then != nil {
		for i := range root.Then.Stmts {
			if ContainsStmt(&root.Then.Stmts[i], target) {
				return true
			}
		}
	}
	if root.Else != nil {
		for i := range root.Else.Stmts {
			if ContainsStmt(&root.Else.Stmts[i], target) {
				return true
			}
		}
	}
	return false
}

// BlockContainsStmt walks a block for target stm_id.
func BlockContainsStmt(b *Block, target *Stmt) bool {
	if b == nil || target == nil {
		return false
	}
	for i := range b.Stmts {
		if ContainsStmt(&b.Stmts[i], target) {
			return true
		}
	}
	return false
}

// ContainsUnfixedGoto mirrors Statement::contains_unfixed_goto for a statement tree.
// Statement.cpp:769–804 — goto edge from inside root, dest outside, not yet visited.
func ContainsUnfixedGoto(root *Stmt, fm *FactMgr) bool {
	if root == nil || fm == nil {
		return false
	}
	ids := map[int]bool{}
	collectStmIDs(root, ids)
	for _, e := range fm.CFGEdges {
		if e == nil || e.SrcID <= 0 {
			continue
		}
		// only care about edges sourced inside this tree
		if !ids[e.SrcID] {
			continue
		}
		// unfixed: source not visited and dest outside this tree
		visited := fm.MapVisited != nil && fm.MapVisited[e.SrcID]
		destInside := e.DestStmID > 0 && ids[e.DestStmID]
		if !visited && !destInside {
			// likely backward/forward goto out of this statement
			if e.BackLink || e.DestStmID > 0 {
				return true
			}
		}
		// visited goto into this tree with empty dest facts vs nonempty src
		if visited && destInside {
			srcOut := fm.MapFactsOut[e.SrcID]
			destIn := fm.MapFactsIn[e.DestStmID]
			if len(srcOut) > 0 && len(destIn) == 0 {
				return true
			}
		}
	}
	return false
}

func collectStmIDs(st *Stmt, ids map[int]bool) {
	if st == nil {
		return
	}
	if st.StmID > 0 {
		ids[st.StmID] = true
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			collectStmIDs(&st.Then.Stmts[i], ids)
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			collectStmIDs(&st.Else.Stmts[i], ids)
		}
	}
}

// ShortcutAnalysis mirrors Statement::shortcut_analysis.
// Statement.cpp:545–567 — 0 reuse, 1 conflict, 2 none.
// facts is updated to map_facts_out on success (0).
func ShortcutAnalysis(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options) int {
	if st == nil || facts == nil || cg == nil || cg.FM == nil {
		return ShortcutNone
	}
	fm := cg.FM
	in, ok := fm.MapFactsIn[st.StmID]
	if !ok {
		return ShortcutNone
	}
	if !SameFacts(*facts, in) || IsCtrlStmt(st) {
		return ShortcutNone
	}
	// contains_unfixed_goto — conservative none if any unvisited goto in tree
	if ContainsUnfixedGoto(st, fm) {
		return ShortcutNone
	}
	eff := fm.GetMapStmEffect(st.StmID)
	if cg.InConflict(eff) {
		return ShortcutConflict
	}
	// reuse outputs
	if out, ok := fm.MapFactsOut[st.StmID]; ok {
		*facts = CloneFactSlice(out)
	}
	cg.AddEffect(eff, false)
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[st.StmID] = cg.AccumEffect()
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[st.StmID] = true
	return ShortcutOK
}

// ValidateAndUpdateFacts mirrors Statement::validate_and_update_facts skeleton.
// Statement.cpp:577+ — try shortcut; else visit_facts and record maps.
func ValidateAndUpdateFacts(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	if st == nil || facts == nil || cg == nil {
		return true
	}
	// sync FM global facts with working set
	if cg.FM != nil {
		cg.FM.GlobalFacts = *facts
	}
	sc := ShortcutAnalysis(st, facts, cg, opts)
	switch sc {
	case ShortcutOK:
		return true
	case ShortcutConflict:
		return false
	}
	// full visit
	if cg.FM != nil && st.StmID > 0 {
		cg.FM.SetMapFactsIn(st.StmID, *facts)
	}
	cg.ClearEffectStm()
	ok := VisitFactsStmt(st, cg, opts)
	if !ok {
		return false
	}
	if cg.FM != nil {
		*facts = CloneFactSlice(cg.FM.GlobalFacts)
		if st.StmID > 0 {
			cg.FM.SetMapFactsOutForStmt(st, *facts, blk)
			cg.FM.SetMapStmEffect(st.StmID, cg.EffectStm)
			if cg.FM.MapAccumEffect == nil {
				cg.FM.MapAccumEffect = make(map[int]Effect)
			}
			cg.FM.MapAccumEffect[st.StmID] = cg.AccumEffect()
			if cg.FM.MapVisited == nil {
				cg.FM.MapVisited = make(map[int]bool)
			}
			cg.FM.MapVisited[st.StmID] = true
		}
	}
	return true
}
