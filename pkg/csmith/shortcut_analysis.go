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
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for _, f := range a {
		idx := FindFact(b, f)
		// residual ERROR sticky — no invent soft-continue same past FindFact residual hole
		if sessHasError(nil) {
			return false
		}
		if idx < 0 {
			return false
		}
	}
	return true
}

// SameUnionFacts mirrors same_facts for the eUnionWrite partition of a FactVec.
// Fact.cpp:237–246 — C++ same_facts walks the full FactVec (ePointTo + eUnionWrite).
// Soft invent was PT-only SameFacts in shortcut → reuse when last-written field lattice
// differed (IsNonreadableField over/under-filters choose_var).
func SameUnionFacts(a, b []*FactUnion) bool {
	if !UnionFactsComplete(a) || !UnionFactsComplete(b) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for _, f := range a {
		idx := FindUnionFact(b, f)
		if sessHasError(nil) {
			return false
		}
		if idx < 0 {
			return false
		}
	}
	return true
}

// FindUnionFact mirrors find_fact for FactUnion (equal by subject + last_written_fid).
func FindUnionFact(facts []*FactUnion, want *FactUnion) int {
	if want == nil {
		return -1
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	if !UnionFactsComplete([]*FactUnion{want}) {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	for i, f := range facts {
		if f.Equal(want) {
			if sessHasError(nil) {
				return -1
			}
			return i
		}
		if sessHasError(nil) {
			return -1
		}
	}
	return -1
}

// SameFactVec mirrors same_facts on a full FactVec (ePointTo + eUnionWrite partitions).
// Fact.cpp:237–246 — total size must match; each fact finds an equal in the other env.
func SameFactVec(ptA []*FactPointTo, uA []*FactUnion, ptB []*FactPointTo, uB []*FactUnion) bool {
	if !FactsComplete(ptA) || !FactsComplete(ptB) || !UnionFactsComplete(uA) || !UnionFactsComplete(uB) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(ptA)+len(uA) != len(ptB)+len(uB) {
		return false
	}
	if !SameFacts(ptA, ptB) {
		return false
	}
	if sessHasError(nil) {
		return false
	}
	if !SameUnionFacts(uA, uB) {
		return false
	}
	return !sessHasError(nil)
}

// FindFact mirrors find_fact — equal fact in vector, or -1.
// Fact.cpp find_fact by equal().
// Incomplete map fails closed sticky -1 (no invent soft-skip hole and match later).
func FindFact(facts []*FactPointTo, want *FactPointTo) int {
	if want == nil {
		return -1
	}
	if !FactsComplete(facts) {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	// want must be a complete fact for Equal to be meaningful sticky
	if !FactsComplete([]*FactPointTo{want}) {
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	for i, f := range facts {
		if f.Equal(want) {
			// residual ERROR sticky — no invent match-index true past Equal hole
			if sessHasError(nil) {
				return -1
			}
			return i
		}
		// residual ERROR sticky — no invent soft-continue later match past Equal residual false
		if sessHasError(nil) {
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
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		// upstream requires same size
		return false
	}
	for _, f1 := range a {
		f2 := FindRelatedPointTo(b, f1.Var)
		// residual ERROR sticky — no invent soft-continue not-subset past FindRelated hole
		if sessHasError(nil) {
			return false
		}
		if f2 == nil {
			return false
		}
		ok := f2.Imply(f1)
		// residual ERROR sticky — no invent soft-continue not-subset past Imply hole
		if sessHasError(nil) {
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

// SubsetUnionFacts mirrors subset_facts for the eUnionWrite partition.
// Fact.cpp:249–260 — same size; each f1 has related f2 that implies f1.
func SubsetUnionFacts(a, b []*FactUnion) bool {
	if !UnionFactsComplete(a) || !UnionFactsComplete(b) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for _, f1 := range a {
		if f1 == nil || f1.Var == nil {
			sessNoteError(nil, ErrGeneric)
			return false
		}
		f2 := FindRelatedUnion(b, f1.Var)
		if sessHasError(nil) {
			return false
		}
		if f2 == nil {
			return false
		}
		ok := f2.Imply(f1)
		if sessHasError(nil) {
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

// SubsetFactVec mirrors subset_facts on a full FactVec (ePointTo + eUnionWrite).
// Fact.cpp:249–260 — total size match; each fact implied by related in other env.
func SubsetFactVec(ptA []*FactPointTo, uA []*FactUnion, ptB []*FactPointTo, uB []*FactUnion) bool {
	if !FactsComplete(ptA) || !FactsComplete(ptB) || !UnionFactsComplete(uA) || !UnionFactsComplete(uB) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if len(ptA)+len(uA) != len(ptB)+len(uB) {
		return false
	}
	if !SubsetFacts(ptA, ptB) {
		return false
	}
	if sessHasError(nil) {
		return false
	}
	if !SubsetUnionFacts(uA, uB) {
		return false
	}
	return !sessHasError(nil)
}

// IsCtrlStmt mirrors Statement::is_ctrl_stmt — break/continue/goto only.
// Statement.h:164–167 — eContinue | eBreak | eGoto (not eReturn; return may pure-shortcut).
// Statement always live; sticky false (no invent not-ctrl soft-skip past hole).
func IsCtrlStmt(st *Stmt) bool {
	if st == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	switch st.Kind {
	case StmtBreak, StmtContinue, StmtGoto:
		return true
	default:
		return false
	}
}

// ContainsStmt reports whether root statement tree contains target by StmID.
// Statement always live; sticky false (no invent not-contained soft-skip past hole).
func ContainsStmt(root, target *Stmt) bool {
	if root == nil || target == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return FindStmtInTree(root, target.StmID) != nil
}

// FindStmtInTree finds a statement by stm_id inside root's tree (self + nested blocks).
// Walks get_blocks only (kind-gated) — no invent search via stray Then on non-compound.
// Incomplete Statement / StmID sticky nil (no invent soft-skip miss / soft re-pick).
// Incomplete Block* hole fails closed sticky nil (no invent soft-skip arm / soft re-pick).
func FindStmtInTree(root *Stmt, stmID int) *Stmt {
	if root == nil || StmIDUnset(stmID) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if root.StmID == stmID {
		return root
	}
	blks := GetBlocksStmt(root)
	// pre-validate complete get_blocks before invent match past incomplete arm sticky
	for _, b := range blks {
		if b == nil {
			sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
		return
	}
	// incomplete CFG sticky (no invent partial mark-as-visited / soft re-pick past holes)
	if !CFGEdgesComplete(fm.CFGEdges) {
		sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if fm == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	ids := map[int]bool{}
	if !collectStmIDs(root, ids) {
		// incomplete get_blocks tree sticky unfixed
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return true
	}
	ok := containsUnfixedGotoIDs(ids, fm)
	// residual ERROR sticky — no invent fixed/unfixed soft-skip past CFG residual hole
	if sessHasError(nil) {
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
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if fm == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	ids := map[int]bool{}
	if !StmIDUnset(b.StmID) {
		ids[b.StmID] = true
	}
	for i := range b.Stmts {
		if !collectStmIDs(&b.Stmts[i], ids) {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return true
		}
	}
	ok := containsUnfixedGotoIDs(ids, fm)
	// residual ERROR sticky — no invent fixed/unfixed soft-skip past CFG residual hole
	if sessHasError(nil) {
		return true
	}
	return ok
}

// containsUnfixedGotoIDs is the CFG scan for Statement.cpp:769–804.
//
// Statement.cpp:781–784 (cond1): goto *src* inside this tree, unvisited, dest outside.
// Statement.cpp:785–803 (cond2): visited goto whose *dest* is inside this tree —
// src need not be inside (nGotoIn to a plain assign / label target). Soft invent
// required ids[src] for both arms → missed inbound gotos → false ShortcutOK
// (seed-7 func_33 remaining assign: UP unfixed=1 nGotoIn=1, GO SC_OK).
func containsUnfixedGotoIDs(ids map[int]bool, fm *FactMgr) bool {
	if fm == nil || len(ids) == 0 {
		return false
	}
	// incomplete CFG sticky unfixed (no invent skip holes as fixed / soft re-pick)
	if !CFGEdgesComplete(fm.CFGEdges) {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	for _, e := range fm.CFGEdges {
		if e.SrcID <= 0 {
			continue
		}
		// Statement.cpp:781/785 — edge->src is eGoto (not loop/block CFG edges).
		if fm.Func != nil {
			src := FindStmtByID(fm.Func, e.SrcID)
			// residual ERROR sticky — no invent soft-continue fixed-scan past FindStmt hole
			// (incomplete if-arm residual soft invents skip then later invents fixed tree)
			if sessHasError(nil) {
				return true
			}
			if src == nil || src.Kind != StmtGoto {
				continue
			}
		} else if !ids[e.SrcID] {
			// No Func to Kind-check: only edges whose src is in the tree were
			// considered historically (tests without Func). Cond2 for external
			// goto requires Func + Kind.
			continue
		}
		srcInside := ids[e.SrcID]
		// contains_stmt(dest): dest stm_id in tree
		destInside := !StmIDUnset(e.DestStmID) && ids[e.DestStmID]
		visited := fm.MapVisited != nil && fm.MapVisited[e.SrcID]
		// Statement.cpp:781–784 — unvisited goto *contained in this*, dest outside
		if srcInside && !visited && !destInside {
			return true
		}
		// Statement.cpp:785–803 — visited goto *into* this tree (src may be outside);
		// re-analyze if dest facts not implied by jump-src outs
		// (or dest in empty while src out nonempty).
		// C++ map_facts_in/out are full FactVec (ePointTo + eUnionWrite). Soft invent
		// was PT-only: seed-895 if-926 pure-shortcut when goto925→905 eUnionWrite
		// dest-in did not imply jump-src out (UP unfixed=1 → visit for g_924.f2
		// drops IV read → feffect order).
		if visited && destInside {
			srcOut := fm.GetMapFactsOut(e.SrcID)
			destIn := fm.GetMapFactsIn(e.DestStmID)
			srcOutU := fm.GetMapUnionFactsOut(e.SrcID)
			destInU := fm.GetMapUnionFactsIn(e.DestStmID)
			// incomplete maps sticky unfixed (GetMap may already SetError)
			if !FactsComplete(srcOut) || !FactsComplete(destIn) ||
				!UnionFactsComplete(srcOutU) || !UnionFactsComplete(destInU) {
				if !sessHasError(nil) {
					sessNoteError(nil, ErrGeneric)
				}
				return true
			}
			// Statement.cpp:901–903 — full FactVec empty check
			srcNonEmpty := len(srcOut) > 0 || len(srcOutU) > 0
			destEmpty := len(destIn) == 0 && len(destInU) == 0
			if srcNonEmpty && destEmpty {
				return true
			}
			for _, f := range destIn {
				if f.Var == nil {
					sessNoteError(nil, ErrGeneric)
					return true
				}
				if f.Var.IsRV() {
					// residual ERROR sticky — no invent soft-continue unfixed scan past IsRV hole
					if sessHasError(nil) {
						return true
					}
					continue
				}
				// residual ERROR sticky — no invent soft-continue past IsRV residual false path
				if sessHasError(nil) {
					return true
				}
				// Statement.cpp:797–800 — jump_src_f && !f->imply(*jump_src_f)
				jumpSrc := FindRelatedPointTo(srcOut, f.Var)
				// residual ERROR sticky — no invent soft-continue fixed past FindRelated hole
				if sessHasError(nil) {
					return true
				}
				if jumpSrc != nil {
					ok := f.Imply(jumpSrc)
					// residual ERROR sticky — no invent soft-continue fixed past Imply hole
					if sessHasError(nil) {
						return true
					}
					if !ok {
						return true
					}
				}
			}
			// eUnionWrite half of full FactVec (same imply gate as ePointTo)
			for _, fu := range destInU {
				if fu == nil || fu.Var == nil {
					sessNoteError(nil, ErrGeneric)
					return true
				}
				if fu.Var.IsRV() {
					if sessHasError(nil) {
						return true
					}
					continue
				}
				if sessHasError(nil) {
					return true
				}
				jumpSrcU := FindRelatedUnion(srcOutU, fu.Var)
				if sessHasError(nil) {
					return true
				}
				if jumpSrcU != nil {
					ok := fu.Imply(jumpSrcU)
					if sessHasError(nil) {
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
	if !StmIDUnset(st.StmID) {
		ids[st.StmID] = true
	}
	// get_blocks only — no invent collect via stray Then on assign/break
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			sessNoteError(nil, ErrGeneric)
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
		sessNoteError(nil, ErrGeneric)
		return ShortcutNone
	}
	if cg.FM == nil {
		return ShortcutNone
	}
	// Statement::stm_id always live; StmID 0 is not a map key (no invent reuse via 0)
	if StmIDUnset(st.StmID) {
		return ShortcutNone
	}
	fm := cg.FM
	// C++ map_facts_in[this] — missing key is empty FactVec (std::map::operator[]).
	in := fm.GetMapFactsIn(st.StmID)
	// Fact* always live in maps; incomplete in/inputs fail closed (SameFacts also rejects holes)
	if !FactsComplete(*facts) || !FactsComplete(in) {
		return ShortcutNone
	}
	// Statement.cpp:551 — same_facts(inputs, map_facts_in[this]) on full FactVec
	// (ePointTo + eUnionWrite). Soft invent was PT-only SameFacts → ShortcutOK when
	// last-written field lattice differed (IsNonreadableField / choose_var skew).
	// Live fm.UnionFacts is the working eUnionWrite half paired with *facts
	// (FindFixedPointBlock currentUnions + ValidateAndUpdateFacts pre-visit).
	if !UnionFactsComplete(fm.UnionFacts) {
		return ShortcutNone
	}
	inU := fm.GetMapUnionFactsIn(st.StmID)
	if !UnionFactsComplete(inU) {
		return ShortcutNone
	}
	if !SameFactVec(*facts, fm.UnionFacts, in, inU) || IsCtrlStmt(st) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past same_facts residual
		if sessHasError(nil) {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past same_facts true path
	if sessHasError(nil) {
		return ShortcutNone
	}
	// contains_unfixed_goto — conservative none if any unvisited goto in tree
	if ContainsUnfixedGoto(st, fm) {
		// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual true
		if sessHasError(nil) {
			return ShortcutNone
		}
		return ShortcutNone
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past unfixed residual false
	if sessHasError(nil) {
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
		if sessHasError(nil) {
			return ShortcutConflict
		}
		return ShortcutConflict
	}
	// residual ERROR sticky — no invent soft-continue ShortcutOK past InConflict residual false
	if sessHasError(nil) {
		return ShortcutConflict
	}
	// Statement.cpp:559 — inputs = map_facts_out[this]; C++ map[] empty if missing.
	out := fm.GetMapFactsOut(st.StmID)
	if !FactsComplete(out) {
		return ShortcutNone
	}
	// Statement.cpp:559 — full FactVec assign: ePointTo + eUnionWrite.
	// Soft invent was PT-only *facts = out leaving live UnionFacts at entry lattice
	// → IsNonreadableField over-filtered choose_var (seed-7 ok 26 vs UP 56).
	outU := fm.GetMapUnionFactsOut(st.StmID)
	if !UnionFactsComplete(outU) {
		return ShortcutNone
	}
	*facts = CloneFactSlice(out)
	clU := CloneUnionFactSliceDeep(outU)
	if !UnionFactsComplete(clU) {
		return ShortcutNone
	}
	fm.UnionFacts = clU
	cg.AddEffect(eff, false)
	// residual ERROR sticky — no invent soft-continue ShortcutOK past AddEffect residual
	if sessHasError(nil) {
		return ShortcutNone
	}
	if !EffectComplete(cg.EffectStm) {
		return ShortcutNone
	}
	acc := cg.AccumEffect()
	if !EffectComplete(acc) {
		return ShortcutNone
	}
	fm.SetMapAccumEffect(st.StmID, acc)
	// residual ERROR sticky — no invent ShortcutOK past SetMapAccumEffect residual
	if sessHasError(nil) {
		return ShortcutNone
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[st.StmID] = true
	return ShortcutOK
}

// setFailedStmSess records the last non-compound visit_facts failure on s.
func setFailedStmSess(s *Session, st *Stmt) {
	sessOrAmbient(s).FailedStm = st
}

// FailedStmSess returns the last failed statement on s (or ambient).
func FailedStmSess(s *Session) *Stmt {
	return sessOrAmbient(s).FailedStm
}

// Session.FailedStm mirrors Statement::failed_stm — last non-compound visit_facts failure.
// Statement.cpp:88 / Statement.h:218 — set in stm_visit_facts when !ok && !is_compound.

// StmVisitFacts mirrors Statement::stm_visit_facts.
// Statement.cpp:609–626 — clear effect_stm; visit_facts; remove_rv_facts;
// always record map_accum_effect and map_visited (even on failure).
// Incomplete inputs or post-visit GlobalFacts fail closed (nil facts, false) —
// no invent cleaned clone of holes while still reporting visit success.
func StmVisitFacts(st *Stmt, facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// Statement.cpp:609+ — always live Statement* + inputs + cg_context
	// incomplete call sticky (no soft invent true / soft re-pick past holes)
	if st == nil || facts == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// Fact* always live; incomplete working set sticky before visit
	if !FactsComplete(*facts) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// Statement.cpp:611 — get_effect_stm().clear()
	// curr_blk = parent is set by ValidateAndUpdateFacts / caller (stmt Parent is
	// the containing Block, not a Stmt field in Go).
	cg.ClearEffectStm()
	// Statement.cpp:609–626 — stm_visit_facts mutates inputs only; does not assign
	// fm->global_facts = inputs. Go VisitFacts* use GlobalFacts as the working set:
	// load *facts only (no invent mergeMayNullFromLive). C++ mid-gen may-null enters
	// FP via self-back merge of map_facts_out (Block.cpp:693+531–536) once per
	// iteration — not per-stmt live rejoin (that over-stripped seed-2 e12688:
	// Function.Blocks n=11 vs UP n=14). Always restore pre-visit live GlobalFacts
	// after harvest so mid-gen lattice survives validation (e10107).
	var liveSaved []*FactPointTo
	haveLive := false
	if cg.FM != nil {
		if FactsComplete(cg.FM.GlobalFacts) {
			liveSaved = CloneFactSlice(cg.FM.GlobalFacts)
			if liveSaved == nil {
				liveSaved = []*FactPointTo{}
			}
			if FactsComplete(liveSaved) {
				haveLive = true
			}
		}
		cl := CloneFactSlice(*facts)
		if sessHasError(nil) || !FactsComplete(cl) {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return false
		}
		cg.FM.SetGlobalFacts(cl, "StmVisitFacts_work")
	}
	ok := VisitFactsStmt(st, cg, opts)
	// Statement.cpp:615–617 — failed_stm = this when !ok && !is_compound
	if !ok && !IsCompound(st.Kind) {
		setFailedStmSess(cgSess(cg), st)
	}
	if cg.FM != nil {
		// Statement.cpp:621–624 — remove_rv on inputs; accum; visited always set
		if !FactsComplete(cg.FM.GlobalFacts) {
			*facts = IncompleteFactSlice()
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			ok = false
		} else {
			*facts = CloneFactSlice(cg.FM.GlobalFacts)
			if sessHasError(nil) || !FactsComplete(*facts) {
				// residual visit HasError with complete empty work: still harvest
				if !FactsComplete(*facts) {
					*facts = IncompleteFactSlice()
					if !sessHasError(nil) {
						sessNoteError(nil, ErrGeneric)
					}
					ok = false
				}
			}
			if FactsComplete(*facts) {
				cg.FM.RemoveRVFacts(facts)
				if !FactsComplete(*facts) {
					if !sessHasError(nil) {
						sessNoteError(nil, ErrGeneric)
					}
					ok = false
				}
			}
		}
		// C++ never assigns global_facts from inputs — restore mid-gen live
		if haveLive {
			cg.FM.SetGlobalFacts(liveSaved, "StmVisitFacts_restore_live")
		} else if !FactsComplete(*facts) {
			cg.FM.GlobalFacts = IncompleteFactSlice()
		} else {
			cg.FM.SetGlobalFacts(*facts, "StmVisitFacts_work_as_live")
		}
		// Statement::stm_id always live; StmID 0 fails closed sticky (C++ always
		// records map_accum_effect / map_visited — no invent soft-skip maps)
		if StmIDUnset(st.StmID) {
			*facts = IncompleteFactSlice()
			if !haveLive {
				cg.FM.GlobalFacts = IncompleteFactSlice()
			}
			sessNoteError(nil, ErrGeneric)
			return false
		}
		// Incomplete accum fails closed sticky visit (record IncompleteEffect; no invent ok true)
		acc := cg.AccumEffect()
		if !EffectComplete(acc) {
			ok = false
			acc = IncompleteEffect()
			sessNoteError(nil, ErrGeneric)
		}
		cg.FM.SetMapAccumEffect(st.StmID, acc)
		// residual ERROR sticky — visit already may be false; still record incomplete marker
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
	// incomplete call sticky (no soft invent true / soft re-pick past holes)
	if st == nil || facts == nil || cg == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// Statement::stm_id always live; StmID 0 sticky (no invent
	// validate success without set_fact_in/out)
	if cg.FM != nil && StmIDUnset(st.StmID) {
		sessNoteError(nil, ErrGeneric)
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
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
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
	// inputs_copy is full FactVec (ePointTo + eUnionWrite). Soft invent was PT-only
	// clone then SetMapFactsIn pairing live (post-visit) UnionFacts → map_facts_in held
	// post last-writes while point-to was pre (same_facts / IsNonreadableField skew).
	inputsCopy := CloneFactSlice(*facts)
	// incomplete pre-visit clone sticky (CloneFactSlice already sticks on holes)
	if !FactsComplete(inputsCopy) {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return false
	}
	var unionInCopy []*FactUnion
	if cg.FM != nil {
		if !UnionFactsComplete(cg.FM.UnionFacts) {
			sessNoteError(nil, ErrGeneric)
			return false
		}
		// deep clone: visit may Join/SetBottom in place on live FactUnion objects
		unionInCopy = CloneUnionFactSliceDeep(cg.FM.UnionFacts)
		if !UnionFactsComplete(unionInCopy) {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return false
		}
	}
	// Statement.cpp:612 — curr_blk = parent (containing block of this statement).
	// Soft invent used only stack-top CurrentBlock(); StatementReturn.cpp:83 and
	// extend_call_chain fallback need curr_blk = parent during FP revisit.
	cg.CurrBlk = blk
	if !StmVisitFacts(st, facts, cg, opts) {
		return false
	}
	// incomplete post-visit sticky (no invent set_fact_in/out success / soft re-pick past hole)
	if !FactsComplete(*facts) {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return false
	}
	if cg.FM != nil {
		// Statement.cpp:604–605 — set_fact_in(pre full FactVec); set_fact_out(post full)
		cg.FM.SetMapFactsInPair(st.StmID, inputsCopy, unionInCopy)
		cg.FM.SetMapFactsOutForStmt(st, *facts, blk)
	}
	return true
}
