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
// Incomplete maps/PointTo fail closed sticky (no invent same-as-skip / soft re-pick past holes).
func SameFacts(a, b []*FactPointTo) bool {
	if !FactsComplete(a) || !FactsComplete(b) {
		SetError(ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for _, f := range a {
		idx := FindFact(b, f)
		// residual ERROR sticky — no invent soft-continue same past FindFact residual hole
		if HasError() {
			return false
		}
		if idx < 0 {
			return false
		}
	}
	return true
}

// FindFact mirrors find_fact — equal fact in vector, or -1.
// Fact.cpp find_fact by equal().
// Incomplete map fails closed sticky -1 (no invent soft-skip hole and match later).
func FindFact(facts []*FactPointTo, want *FactPointTo) int {
	if want == nil {
		return -1
	}
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return -1
	}
	// want must be a complete fact for Equal to be meaningful sticky
	if !FactsComplete([]*FactPointTo{want}) {
		SetError(ErrGeneric)
		return -1
	}
	for i, f := range facts {
		if f.Equal(want) {
			// residual ERROR sticky — no invent match-index true past Equal hole
			if HasError() {
				return -1
			}
			return i
		}
		// residual ERROR sticky — no invent soft-continue later match past Equal residual false
		if HasError() {
			return -1
		}
	}
	return -1
}

// SubsetFacts mirrors subset_facts — each f1 is implied by related f2.
// Fact.cpp:249–260.
// Incomplete maps/PointTo fail closed sticky (no invent subset / soft re-pick past holes).
func SubsetFacts(a, b []*FactPointTo) bool {
	if !FactsComplete(a) || !FactsComplete(b) {
		SetError(ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		// upstream requires same size
		return false
	}
	for _, f1 := range a {
		f2 := FindRelatedPointTo(b, f1.Var)
		// residual ERROR sticky — no invent soft-continue not-subset past FindRelated hole
		if HasError() {
			return false
		}
		if f2 == nil {
			return false
		}
		ok := f2.Imply(f1)
		// residual ERROR sticky — no invent soft-continue not-subset past Imply hole
		if HasError() {
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

// IsCtrlStmt mirrors Statement::is_ctrl_stmt — break/continue/goto/return.
// Statement always live; sticky false (no invent not-ctrl soft-skip past hole).
func IsCtrlStmt(st *Stmt) bool {
	if st == nil {
		SetError(ErrGeneric)
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
// Statement always live; sticky false (no invent not-contained soft-skip past hole).
func ContainsStmt(root, target *Stmt) bool {
	if root == nil || target == nil {
		SetError(ErrGeneric)
		return false
	}
	return FindStmtInTree(root, target.StmID) != nil
}

// FindStmtInTree finds a statement by stm_id inside root's tree (self + nested blocks).
// Walks get_blocks only (kind-gated) — no invent search via stray Then on non-compound.
// Incomplete Statement / StmID sticky nil (no invent soft-skip miss / soft re-pick).
// Incomplete Block* hole fails closed sticky nil (no invent soft-skip arm / soft re-pick).
func FindStmtInTree(root *Stmt, stmID int) *Stmt {
	if root == nil || stmID <= 0 {
		SetError(ErrGeneric)
		return nil
	}
	if root.StmID == stmID {
		return root
	}
	blks := GetBlocksStmt(root)
	// pre-validate complete get_blocks before invent match past incomplete arm sticky
	for _, b := range blks {
		if b == nil {
			SetError(ErrGeneric)
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
// Statement + FactMgr always live; sticky (no invent soft-skip mark past hole).
func MarkContainedGotosVisited(root *Stmt, fm *FactMgr) {
	if root == nil || fm == nil {
		SetError(ErrGeneric)
		return
	}
	// incomplete CFG sticky (no invent partial mark-as-visited / soft re-pick past holes)
	if !CFGEdgesComplete(fm.CFGEdges) {
		SetError(ErrGeneric)
		return
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
// Block + Statement always live; sticky false (no invent not-contained soft-skip past hole).
func BlockContainsStmt(b *Block, target *Stmt) bool {
	if b == nil || target == nil {
		SetError(ErrGeneric)
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
	// Statement always live; sticky unfixed (no invent all-fixed soft-skip past hole).
	// Statement.cpp:770–771 — get_fact_mgr_for_func; assert(fm)
	// fail closed sticky: nil FM is unfixed (no invent "all gotos fixed" / soft re-pick)
	if root == nil {
		SetError(ErrGeneric)
		return true
	}
	if fm == nil {
		SetError(ErrGeneric)
		return true
	}
	ids := map[int]bool{}
	if !collectStmIDs(root, ids) {
		// incomplete get_blocks tree sticky unfixed
		if !HasError() {
			SetError(ErrGeneric)
		}
		return true
	}
	ok := containsUnfixedGotoIDs(ids, fm)
	// residual ERROR sticky — no invent fixed/unfixed soft-skip past CFG residual hole
	if HasError() {
		return true
	}
	return ok
}

// ContainsUnfixedGotoBlock mirrors contains_unfixed_goto when root is a Block.
// Block is a Statement in C++; walk nested stmts for contains_stmt(goto/dest).
// Block always live; sticky unfixed (no invent all-fixed soft-skip past hole).
// assert(fm) sticky unfixed without FactMgr.
func ContainsUnfixedGotoBlock(b *Block, fm *FactMgr) bool {
	if b == nil {
		SetError(ErrGeneric)
		return true
	}
	if fm == nil {
		SetError(ErrGeneric)
		return true
	}
	ids := map[int]bool{}
	if b.StmID > 0 {
		ids[b.StmID] = true
	}
	for i := range b.Stmts {
		if !collectStmIDs(&b.Stmts[i], ids) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return true
		}
	}
	ok := containsUnfixedGotoIDs(ids, fm)
	// residual ERROR sticky — no invent fixed/unfixed soft-skip past CFG residual hole
	if HasError() {
		return true
	}
	return ok
}

// containsUnfixedGotoIDs is the CFG scan for Statement.cpp:769–804.
func containsUnfixedGotoIDs(ids map[int]bool, fm *FactMgr) bool {
	if fm == nil || len(ids) == 0 {
		return false
	}
	// incomplete CFG sticky unfixed (no invent skip holes as fixed / soft re-pick)
	if !CFGEdgesComplete(fm.CFGEdges) {
		SetError(ErrGeneric)
		return true
	}
	for _, e := range fm.CFGEdges {
		if e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:781 — edge->src is eGoto and contains_stmt(src)
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			// residual ERROR sticky — no invent soft-continue fixed-scan past FindStmt hole
			// (incomplete if-arm residual soft invents skip then later invents fixed tree)
			if HasError() {
				return true
			}
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
			srcOut := fm.GetMapFactsOut(e.SrcID)
			destIn := fm.GetMapFactsIn(e.DestStmID)
			// incomplete maps sticky unfixed (GetMap may already SetError)
			if !FactsComplete(srcOut) || !FactsComplete(destIn) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return true
			}
			if len(srcOut) > 0 && len(destIn) == 0 {
				return true
			}
			for _, f := range destIn {
				if f.Var == nil {
					SetError(ErrGeneric)
					return true
				}
				if f.Var.IsRV() {
					// residual ERROR sticky — no invent soft-continue unfixed scan past IsRV hole
					if HasError() {
						return true
					}
					continue
				}
				// residual ERROR sticky — no invent soft-continue past IsRV residual false path
				if HasError() {
					return true
				}
				// Statement.cpp:797–800 — jump_src_f && !f->imply(*jump_src_f)
				jumpSrc := FindRelatedPointTo(srcOut, f.Var)
				// residual ERROR sticky — no invent soft-continue fixed past FindRelated hole
				if HasError() {
					return true
				}
				if jumpSrc != nil {
					ok := f.Imply(jumpSrc)
					// residual ERROR sticky — no invent soft-continue fixed past Imply hole
					if HasError() {
						return true
					}
					if !ok {
						return true
					}
				}
			}
		}
	}
	return false
}

// collectStmIDs records StmIDs under st via get_blocks. Returns false on incomplete
// Block* hole sticky (no invent partial id set then claim all gotos fixed / soft re-pick).
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
			SetError(ErrGeneric)
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
// Statement + facts + CGContext always live; sticky ShortcutNone
// (no invent soft-skip shortcut past hole).
// Nil FM / StmID≤0 is non-sticky ShortcutNone (intentional reuse miss / soft re-pick).
func ShortcutAnalysis(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options) int {
	if st == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return ShortcutNone
	}
	if cg.FM == nil {
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
		// residual ERROR sticky — no invent soft-continue ShortcutOK past SameFacts residual
		if HasError() {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past SameFacts residual true path
	if HasError() {
		return ShortcutNone
	}
	// contains_unfixed_goto — conservative none if any unvisited goto in tree
	if ContainsUnfixedGoto(st, fm) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual true
		if HasError() {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual false
	if HasError() {
		return ShortcutNone
	}
	// Incomplete map_stm_effect / accum fails closed before AddEffect
	// (no invent ShortcutOK with poison; no sticky SetError on intentional none)
	eff := fm.GetMapStmEffect(st.StmID)
	if !EffectComplete(eff) {
		return ShortcutNone
	}
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		return ShortcutNone
	}
	if !EffectComplete(cg.EffectStm) {
		return ShortcutNone
	}
	if cg.InConflict(eff) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past InConflict residual true
		if HasError() {
			return ShortcutConflict
		}
		return ShortcutConflict
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past InConflict residual false
	if HasError() {
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
	// residual ERROR sticky — no invent soft-continue ShortcutOK past AddEffect residual
	if HasError() {
		return ShortcutNone
	}
	if !EffectComplete(cg.EffectStm) {
		return ShortcutNone
	}
	acc := cg.AccumEffect()
	if !EffectComplete(acc) {
		return ShortcutNone
	}
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	fm.MapAccumEffect[st.StmID] = acc
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
	// incomplete call sticky (no soft invent true / soft re-pick past holes)
	if st == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	// Fact* always live; incomplete working set sticky before visit
	if !FactsComplete(*facts) {
		SetError(ErrGeneric)
		return false
	}
	// Statement.cpp:611 — get_effect_stm().clear()
	cg.ClearEffectStm()
	// Statement.cpp:612 — curr_blk = parent (stack top is current block in Go)
	// C++ stm_visit_facts: visit_facts(inputs) mutates inputs only — does not assign
	// fm->global_facts = inputs. Mid-gen ExpressionAssign updates live on global_facts
	// (OPP reads global_facts). Go visit uses FM.GlobalFacts as the working set, so we
	// must load *facts into GlobalFacts for the walk — but must not drop may-null that
	// is already on GlobalFacts and missing from map_facts_in / fixed-point *facts
	// (seed-2 e10107: StmVisitFacts SetGlobalFacts wiped l_233 may-null).
	if cg.FM != nil {
		cl := CloneFactSlice(*facts)
		if HasError() || !FactsComplete(cl) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		cl = mergeMayNullFromLive(cg.FM.GlobalFacts, cl)
		if HasError() || !FactsComplete(cl) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return false
		}
		cg.FM.SetGlobalFacts(cl, "StmVisitFacts_work")
	}
	ok := VisitFactsStmt(st, cg, opts)
	// Statement.cpp:615–617 — failed_stm = this when !ok && !is_compound
	if !ok && !IsCompound(st.Kind) {
		FailedStm = st
	}
	if cg.FM != nil {
		// Statement.cpp:621–624 — remove_rv; accum; visited always set
		// incomplete GlobalFacts after visit: sticky wipe + false (no invent clean slice)
		if !FactsComplete(cg.FM.GlobalFacts) {
			*facts = IncompleteFactSlice()
			cg.FM.GlobalFacts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			ok = false
		} else {
			*facts = CloneFactSlice(cg.FM.GlobalFacts)
			if HasError() || !FactsComplete(*facts) {
				*facts = IncompleteFactSlice()
				cg.FM.GlobalFacts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				ok = false
			} else {
				cg.FM.RemoveRVFacts(facts)
				// RemoveRVFacts mutates *facts; keep GlobalFacts as the same post-RV set
				// C++ stm_visit_facts only remove_rv_facts(inputs) — no separate global_facts
				cg.FM.SetGlobalFacts(*facts, "auto_shortcut_analysis_550")
			}
		}
		// Statement::stm_id always live; StmID 0 fails closed sticky (C++ always
		// records map_accum_effect / map_visited — no invent soft-skip maps)
		if st.StmID <= 0 {
			*facts = IncompleteFactSlice()
			cg.FM.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return false
		}
		// Incomplete accum fails closed sticky visit (record IncompleteEffect; no invent ok true)
		acc := cg.AccumEffect()
		if !EffectComplete(acc) {
			ok = false
			acc = IncompleteEffect()
			SetError(ErrGeneric)
		}
		if cg.FM.MapAccumEffect == nil {
			cg.FM.MapAccumEffect = make(map[int]Effect)
		}
		cg.FM.MapAccumEffect[st.StmID] = acc
		if cg.FM.MapVisited == nil {
			cg.FM.MapVisited = make(map[int]bool)
		}
		cg.FM.MapVisited[st.StmID] = true
	}
	return ok
}

// mergeMayNullFromLive joins may-null lattice from live into work for subjects
// already present in work. Used when loading fixed-point / map_facts_in into
// GlobalFacts so mid-gen ExpressionAssign may-null is not dropped.
func mergeMayNullFromLive(live, work []*FactPointTo) []*FactPointTo {
	if !FactsComplete(live) || !FactsComplete(work) {
		return work
	}
	for _, f := range live {
		if f == nil || f.Var == nil || !f.IsNull() {
			continue
		}
		var subj *Variable
		if rel := FindRelatedPointTo(work, f.Var); rel != nil {
			subj = rel.Var
		} else {
			for _, w := range work {
				if w != nil && w.Var != nil && w.Var.Name == f.Var.Name {
					subj = w.Var
					break
				}
			}
		}
		if subj == nil {
			continue
		}
		bridge := MakeFactPointToSet(subj, f.PointTo)
		if bridge == nil {
			return IncompleteFactSlice()
		}
		merged := MergeFactInto(work, bridge)
		if !FactsComplete(merged) {
			return IncompleteFactSlice()
		}
		work = merged
	}
	return work
}

// ValidateAndUpdateFacts mirrors Statement::validate_and_update_facts.
// Statement.cpp:569–606 — shortcut; else stm_visit_facts then set_fact_in/out.
// Incomplete working facts fail closed (false) — no invent pre-visit copy past holes.
func ValidateAndUpdateFacts(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options, blk *Block) bool {
	// Statement.cpp:574+ — always live this + inputs + cg_context
	// incomplete call sticky (no soft invent true / soft re-pick past holes)
	if st == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) {
		SetError(ErrGeneric)
		return false
	}
	// Statement::stm_id always live; StmID 0 sticky (no invent
	// validate success without set_fact_in/out)
	if cg.FM != nil && st.StmID <= 0 {
		SetError(ErrGeneric)
		return false
	}
	// Statement.cpp:574–606 — validate_and_update_facts does NOT assign
	// fm->global_facts = inputs before shortcut/visit. Mid-gen ExpressionAssign
	// updates live on global_facts; map_facts_in / fixed-point *facts can lag.
	// Installing CloneFactSlice(*facts) here wiped l_233 may-null (seed-2 e10107:
	// WIPE at ValidateAndUpdateFacts before shortcut reuse returned without
	// re-visit). C++ opportunistic_validate reads global_facts, not map_facts_in.
	sc := ShortcutAnalysis(st, facts, cg, opts)
	switch sc {
	case ShortcutOK:
		// incomplete clone of out sticky (no invent shortcut success / soft re-pick)
		if !FactsComplete(*facts) {
			if !HasError() {
				SetError(ErrGeneric)
			}
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
	// incomplete pre-visit clone sticky (CloneFactSlice already sticks on holes)
	if !FactsComplete(inputsCopy) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if !StmVisitFacts(st, facts, cg, opts) {
		return false
	}
	// incomplete post-visit sticky (no invent set_fact_in/out success / soft re-pick past hole)
	if !FactsComplete(*facts) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if cg.FM != nil {
		// Statement.cpp:604–605 — set_fact_in(pre); set_fact_out(post)
		cg.FM.SetMapFactsIn(st.StmID, inputsCopy)
		cg.FM.SetMapFactsOutForStmt(st, *facts, blk)
	}
	return true
}
