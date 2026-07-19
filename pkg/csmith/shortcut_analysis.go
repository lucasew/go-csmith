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
// Incomplete maps/PointTo fail closed (no invent same-as-skip past holes).
func SameFacts(a, b []*FactPointTo) bool {
	if !FactsComplete(a) || !FactsComplete(b) {
		return false
	}
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
// Incomplete map fails closed -1 (no invent soft-skip hole and match later).
func FindFact(facts []*FactPointTo, want *FactPointTo) int {
	if want == nil || !FactsComplete(facts) {
		return -1
	}
	// want must be a complete fact for Equal to be meaningful
	if !FactsComplete([]*FactPointTo{want}) {
		return -1
	}
	for i, f := range facts {
		if f.Equal(want) {
			return i
		}
	}
	return -1
}

// SubsetFacts mirrors subset_facts — each f1 is implied by related f2.
// Fact.cpp:249–260.
// Incomplete maps/PointTo fail closed (no invent subset past holes).
func SubsetFacts(a, b []*FactPointTo) bool {
	if !FactsComplete(a) || !FactsComplete(b) {
		return false
	}
	if len(a) != len(b) {
		// upstream requires same size
		return false
	}
	for _, f1 := range a {
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
	return FindStmtInTree(root, target.StmID) != nil
}

// FindStmtInTree finds a statement by stm_id inside root's tree (self + nested blocks).
// Walks get_blocks only (kind-gated) — no invent search via stray Then on non-compound.
// Incomplete Block* hole fails closed nil (no invent soft-skip arm and match later).
func FindStmtInTree(root *Stmt, stmID int) *Stmt {
	if root == nil || stmID <= 0 {
		return nil
	}
	if root.StmID == stmID {
		return root
	}
	blks := GetBlocksStmt(root)
	// pre-validate complete get_blocks before invent match past incomplete arm
	for _, b := range blks {
		if b == nil {
			return nil
		}
	}
	for _, b := range blks {
		for i := range b.Stmts {
			if s := FindStmtInTree(&b.Stmts[i], stmID); s != nil {
				return s
			}
		}
	}
	return nil
}

// MarkContainedGotosVisited mirrors validate_and_update_facts shortcut==0 path.
// Statement.cpp:580–595 — mark goto edges sourced inside this statement visited
// so fixed-point can feed goto outputs into the label target.
func MarkContainedGotosVisited(root *Stmt, fm *FactMgr) {
	if root == nil || fm == nil {
		return
	}
	// CFGEdge* always live; pre-scan for holes so we never invent partial
	// mark-as-visited before hitting a nil edge (two-phase: validate then mark).
	for _, e := range fm.CFGEdges {
		if e == nil {
			return
		}
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	for _, e := range fm.CFGEdges {
		if e.SrcID <= 0 {
			continue
		}
		s := FindStmtInTree(root, e.SrcID)
		if s != nil && s.Kind == StmtGoto {
			fm.MapVisited[e.SrcID] = true
		}
	}
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
// Statement.cpp:769–804 — unvisited goto out of tree, or visited goto whose
// dest facts are not implied by jump-src outs.
func ContainsUnfixedGoto(root *Stmt, fm *FactMgr) bool {
	// Statement.cpp:770–771 — get_fact_mgr_for_func; assert(fm)
	// fail closed: nil FM is unfixed (no invent "all gotos fixed")
	if root == nil {
		return false
	}
	if fm == nil {
		return true
	}
	ids := map[int]bool{}
	if !collectStmIDs(root, ids) {
		// incomplete get_blocks tree — fail closed unfixed
		return true
	}
	return containsUnfixedGotoIDs(ids, fm)
}

// ContainsUnfixedGotoBlock mirrors contains_unfixed_goto when root is a Block.
// Block is a Statement in C++; walk nested stmts for contains_stmt(goto/dest).
func ContainsUnfixedGotoBlock(b *Block, fm *FactMgr) bool {
	// assert(fm) — fail closed unfixed without FactMgr
	if b == nil {
		return false
	}
	if fm == nil {
		return true
	}
	ids := map[int]bool{}
	if b.StmID > 0 {
		ids[b.StmID] = true
	}
	for i := range b.Stmts {
		if !collectStmIDs(&b.Stmts[i], ids) {
			return true
		}
	}
	return containsUnfixedGotoIDs(ids, fm)
}

// containsUnfixedGotoIDs is the CFG scan for Statement.cpp:769–804.
func containsUnfixedGotoIDs(ids map[int]bool, fm *FactMgr) bool {
	if fm == nil || len(ids) == 0 {
		return false
	}
	for _, e := range fm.CFGEdges {
		// CFGEdge* always live; nil hole is unfixed (no invent skip as fixed)
		if e == nil {
			return true
		}
		if e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:781 — edge->src is eGoto and contains_stmt(src)
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			if src == nil || src.Kind != StmtGoto {
				continue
			}
		}
		if !ids[e.SrcID] {
			continue
		}
		visited := fm.MapVisited != nil && fm.MapVisited[e.SrcID]
		// contains_stmt(dest): dest stm_id in tree (block dest DestStmID==0 rare for goto)
		destInside := e.DestStmID > 0 && ids[e.DestStmID]
		// Statement.cpp:781–784 — unvisited goto to dest outside this tree
		if !visited && !destInside {
			return true
		}
		// Statement.cpp:785–803 — visited goto into this tree; re-analyze if dest
		// facts not implied by jump-src outs (or dest in empty while src out nonempty)
		if visited && destInside {
			srcOut := fm.MapFactsOut[e.SrcID]
			destIn := fm.MapFactsIn[e.DestStmID]
			// incomplete maps fail closed unfixed — no invent "all fixed" when
			// FindRelatedPointTo returns nil past a hole before a related fact
			if !FactsComplete(srcOut) || !FactsComplete(destIn) {
				return true
			}
			if len(srcOut) > 0 && len(destIn) == 0 {
				return true
			}
			for _, f := range destIn {
				if f.Var.IsRV() {
					continue
				}
				// Statement.cpp:797–800 — jump_src_f && !f->imply(*jump_src_f)
				jumpSrc := FindRelatedPointTo(srcOut, f.Var)
				if jumpSrc != nil && !f.Imply(jumpSrc) {
					return true
				}
			}
		}
	}
	return false
}

// collectStmIDs records StmIDs under st via get_blocks. Returns false on incomplete
// Block* hole (no invent partial id set then claim all gotos fixed).
func collectStmIDs(st *Stmt, ids map[int]bool) bool {
	if st == nil {
		return false
	}
	if st.StmID > 0 {
		ids[st.StmID] = true
	}
	// get_blocks only — no invent collect via stray Then on assign/break
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			return false
		}
		for i := range b.Stmts {
			if !collectStmIDs(&b.Stmts[i], ids) {
				return false
			}
		}
	}
	return true
}

// ShortcutAnalysis mirrors Statement::shortcut_analysis.
// Statement.cpp:545–567 — 0 reuse, 1 conflict, 2 none.
// facts is updated to map_facts_out on success (0).
// Incomplete or missing map_facts_out fails closed (ShortcutNone) — no invent
// reuse success while leaving inputs unchanged or cloning past nil holes.
func ShortcutAnalysis(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options) int {
	if st == nil || facts == nil || cg == nil || cg.FM == nil {
		return ShortcutNone
	}
	// Statement::stm_id always live; StmID 0 is not a map key (no invent reuse via 0)
	if st.StmID <= 0 {
		return ShortcutNone
	}
	fm := cg.FM
	in, ok := fm.MapFactsIn[st.StmID]
	if !ok {
		return ShortcutNone
	}
	// Fact* always live in maps; incomplete in/inputs fail closed (SameFacts also rejects holes)
	if !FactsComplete(*facts) || !FactsComplete(in) {
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
	// Statement.cpp:559 — inputs = map_facts_out[this]; out must be present and complete
	// no invent ShortcutOK when out missing (would keep inputs) or has nil holes
	out, ok := fm.MapFactsOut[st.StmID]
	if !ok || !FactsComplete(out) {
		return ShortcutNone
	}
	*facts = CloneFactSlice(out)
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

// FailedStm mirrors Statement::failed_stm — last non-compound visit_facts failure.
// Statement.cpp:88 / Statement.h:218 — set in stm_visit_facts when !ok && !is_compound.
var FailedStm *Stmt

// StmVisitFacts mirrors Statement::stm_visit_facts.
// Statement.cpp:609–626 — clear effect_stm; visit_facts; remove_rv_facts;
// always record map_accum_effect and map_visited (even on failure).
// Incomplete inputs or post-visit GlobalFacts fail closed (nil facts, false) —
// no invent cleaned clone of holes while still reporting visit success.
func StmVisitFacts(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// Statement.cpp:609+ — always live Statement* + inputs + cg_context
	// no soft invent true on incomplete call
	if st == nil || facts == nil || cg == nil {
		return false
	}
	// Fact* always live; incomplete working set fails closed before visit
	if !FactsComplete(*facts) {
		return false
	}
	// Statement.cpp:611 — get_effect_stm().clear()
	cg.ClearEffectStm()
	// Statement.cpp:612 — curr_blk = parent (stack top is current block in Go)
	if cg.FM != nil {
		cg.FM.GlobalFacts = *facts
	}
	ok := VisitFactsStmt(st, cg, opts)
	// Statement.cpp:615–617 — failed_stm = this when !ok && !is_compound
	if !ok && !IsCompound(st.Kind) {
		FailedStm = st
	}
	if cg.FM != nil {
		// Statement.cpp:621–624 — remove_rv; accum; visited always set
		// incomplete GlobalFacts after visit: nil facts + false (no invent clean slice)
		if !FactsComplete(cg.FM.GlobalFacts) {
			*facts = nil
			cg.FM.GlobalFacts = nil
			ok = false
		} else {
			*facts = CloneFactSlice(cg.FM.GlobalFacts)
			cg.FM.RemoveRVFacts(facts)
			cg.FM.GlobalFacts = *facts
		}
		// Statement::stm_id always live; StmID 0 fails closed (C++ always
		// records map_accum_effect / map_visited — no invent soft-skip maps)
		if st.StmID <= 0 {
			*facts = nil
			cg.FM.GlobalFacts = nil
			return false
		}
		if cg.FM.MapAccumEffect == nil {
			cg.FM.MapAccumEffect = make(map[int]Effect)
		}
		cg.FM.MapAccumEffect[st.StmID] = cg.AccumEffect()
		if cg.FM.MapVisited == nil {
			cg.FM.MapVisited = make(map[int]bool)
		}
		cg.FM.MapVisited[st.StmID] = true
	}
	return ok
}

// ValidateAndUpdateFacts mirrors Statement::validate_and_update_facts.
// Statement.cpp:569–606 — shortcut; else stm_visit_facts then set_fact_in/out.
// Incomplete working facts fail closed (false) — no invent pre-visit copy past holes.
func ValidateAndUpdateFacts(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	// Statement.cpp:574+ — always live this + inputs + cg_context
	// no soft invent true on incomplete call
	if st == nil || facts == nil || cg == nil {
		return false
	}
	if !FactsComplete(*facts) {
		return false
	}
	// sync FM global facts with working set
	if cg.FM != nil {
		// Statement::stm_id always live; StmID 0 fails closed (no invent
		// validate success without set_fact_in/out)
		if st.StmID <= 0 {
			return false
		}
		cg.FM.GlobalFacts = *facts
	}
	sc := ShortcutAnalysis(st, facts, cg, opts)
	switch sc {
	case ShortcutOK:
		// incomplete clone of out fails closed (no invent shortcut success)
		if !FactsComplete(*facts) {
			return false
		}
		// Statement.cpp:580–595 — mark contained gotos visited on shortcut reuse
		if cg.FM != nil {
			MarkContainedGotosVisited(st, cg.FM)
		}
		return true
	case ShortcutConflict:
		return false
	}
	// Statement.cpp:600–605 — copy pre-visit inputs; stm_visit; set in/out only on success
	inputsCopy := CloneFactSlice(*facts)
	if !StmVisitFacts(st, facts, cg, opts) {
		return false
	}
	// incomplete post-visit must not invent set_fact_in/out success
	if !FactsComplete(*facts) {
		return false
	}
	if cg.FM != nil {
		// Statement.cpp:604–605 — set_fact_in(pre); set_fact_out(post)
		cg.FM.SetMapFactsIn(st.StmID, inputsCopy)
		cg.FM.SetMapFactsOutForStmt(st, *facts, blk)
	}
	return true
}
