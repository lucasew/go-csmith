// Upstream: StatementGoto.cpp (make_random) with back/forward edge selection.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Session.StmLabels mirrors StatementGoto::stm_labels — dest statement → shared label.
// StatementGoto.cpp:55, 224–229.

// LabelForGotoDest returns existing or new gensym label for a jump destination.
// StatementGoto.cpp:224–229 — reuse stm_labels[dest] when present; else gensym("lbl_").
// no invent fixed "lbl_1" when nextLabel is nil
func LabelForGotoDest(destStmID int, nextLabel func() string) string {
	return LabelForGotoDestSess(testAmbientSession, destStmID, nextLabel)
}

// LabelForGotoDestSess is LabelForGotoDest on an explicit session bag.
func LabelForGotoDestSess(s *Session, destStmID int, nextLabel func() string) string {
	s = sessOrAmbient(s)
	if s.StmLabels == nil {
		s.StmLabels = map[int]string{}
	}
	if !StmIDUnset(destStmID) {
		if lab, ok := s.StmLabels[destStmID]; ok && lab != "" {
			return lab
		}
	}
	// StatementGoto.cpp:227 — gensym("lbl_"); process-wide util.cpp counter
	lab := ""
	if nextLabel != nil {
		lab = nextLabel()
	} else {
		lab = GensymSess(s, "lbl_")
	}
	// incomplete empty label is broken IR sticky — fail closed (no invent "goto :")
	if lab == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !StmIDUnset(destStmID) {
		s.StmLabels[destStmID] = lab
	}
	return lab
}

// GotoLabelsDoFinalization mirrors StatementGoto::doFinalization.
// StatementGoto.cpp:404 — stm_labels.clear().
func GotoLabelsDoFinalization() {
	GotoLabelsDoFinalizationSess(testAmbientSession)
}

// GotoLabelsDoFinalizationSess clears stm_labels on an explicit session bag.
func GotoLabelsDoFinalizationSess(s *Session) {
	sessOrAmbient(s).StmLabels = map[int]string{}
}

// setStmLabelSess records a label for destStmID on the session bag.
func setStmLabelSess(s *Session, destStmID int, label string) {
	s = sessOrAmbient(s)
	if s.StmLabels == nil {
		s.StmLabels = map[int]string{}
	}
	if !StmIDUnset(destStmID) {
		s.StmLabels[destStmID] = label
	}
}

// lookupStmLabelSess returns a registered label for destStmID when present.
func lookupStmLabelSess(s *Session, destStmID int) string {
	s = sessOrAmbient(s)
	if lab, ok := s.StmLabels[destStmID]; ok && lab != "" {
		return lab
	}
	return ""
}

// goodGotoTarget reports statements that may receive a label (not jump/return-ish).
// StatementGoto.cpp:99–109 — disallow break/continue/goto/return as targets.
func goodGotoTarget(st Stmt) bool {
	switch st.Kind {
	case StmtReturn, StmtBreak, StmtContinue, StmtGoto:
		return false
	default:
		return true
	}
}

// ContainsStmt reports whether b (or nested get_blocks) holds st.
// Used by tests and tree walks. For NeedRevisit LCA, prefer BlockContainsViaParent
// (Statement.cpp:789–795 Block case).
// Kind-gated get_blocks only; nil arm sticky false (no invent membership
// by soft-skipping a missing if-arm / stray Then on assign).
func (b *Block) ContainsStmt(st *Stmt) bool {
	return b.ContainsStmtSess(testAmbientSession, st)
}

// ContainsStmtSess is ContainsStmt with explicit session residual sticky.
func (b *Block) ContainsStmtSess(s *Session, st *Stmt) bool {
	// Block + Statement always live; sticky incomplete no invent not-contain soft-skip
	if b == nil || st == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	for i := range b.Stmts {
		cur := &b.Stmts[i]
		if cur == st {
			return true
		}
		if !StmIDUnset(st.StmID) && cur.StmID == st.StmID {
			return true
		}
		blks := GetBlocksStmtSess(s, cur)
		for _, nb := range blks {
			if nb == nil {
				// incomplete arm sticky fail closed not-contain
				sessNoteError(s, ErrGeneric)
				return false
			}
		}
		for _, nb := range blks {
			if nb.ContainsStmtSess(s, st) {
				return true
			}
		}
	}
	return false
}

// BlockContainsViaParent mirrors Statement::contains_stmt when *this is a Block
// (Statement.cpp:789–795): true if b is on destParent's parent chain
// (destParent is Statement::parent of the dest statement).
// Soft invent walked b.Stmts / GetBlocksStmt so LCA missed the function body when
// the if holding dest was not yet appended to body.Stmts during MakeRandomIf
// (seed-154: body NeedRevisit stayed false → no body FP → effect_accum kept
// make_iteration IV reads in feffect vs UP body FP cleaning them).
func BlockContainsViaParent(b, destParent *Block) bool {
	return BlockContainsViaParentSess(testAmbientSession, b, destParent)
}

// BlockContainsViaParentSess is BlockContainsViaParent with explicit session residual sticky.
func BlockContainsViaParentSess(s *Session, b, destParent *Block) bool {
	if b == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// destParent nil: dest has no parent (impossible for live stmt in C++) sticky false
	for d := destParent; d != nil; d = d.Parent {
		if d == b {
			return true
		}
	}
	return false
}

// MarkNeedRevisitLCA sets NeedRevisit on the least incomplete ancestor of curr
// that contains dest (StatementGoto.cpp:141–147).
// destParent is dest->parent (ok_blk for the chosen other_stm).
func MarkNeedRevisitLCA(curr *Block, dest *Stmt) {
	MarkNeedRevisitLCASess(testAmbientSession, curr, dest)
}

// MarkNeedRevisitLCASess is MarkNeedRevisitLCA with explicit session residual sticky.
func MarkNeedRevisitLCASess(s *Session, curr *Block, dest *Stmt) {
	MarkNeedRevisitLCAParentSess(s, curr, dest, nil)
}

// MarkNeedRevisitLCAParent is the StatementGoto.cpp:141–147 LCA walk with an
// explicit dest parent for Block contains_stmt (parent-chain) semantics.
func MarkNeedRevisitLCAParent(curr *Block, dest *Stmt, destParent *Block) {
	MarkNeedRevisitLCAParentSess(testAmbientSession, curr, dest, destParent)
}

// MarkNeedRevisitLCAParentSess is MarkNeedRevisitLCAParent with explicit session residual sticky.
func MarkNeedRevisitLCAParentSess(s *Session, curr *Block, dest *Stmt, destParent *Block) {
	// StatementGoto.cpp:143–147 — while (!contains) b=parent; assert(b); need_revisit
	// no soft invent NeedRevisit on curr when no ancestor contains dest
	if dest == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// Prefer destParent (C++ other_stm->parent). When nil, fall back to tree walk
	// ContainsStmt (tests / incomplete IR).
	for b := curr; b != nil; b = b.Parent {
		var has bool
		if destParent != nil {
			has = BlockContainsViaParentSess(s, b, destParent)
		} else {
			has = b.ContainsStmtSess(s, dest)
		}
		if sessHasError(s) {
			return
		}
		if has {
			b.NeedRevisit = true
			return
		}
	}
}

// HasInitSkippedVars mirrors StatementGoto::has_init_skipped_vars.
// StatementGoto.cpp:281–306 — jump into/out would skip locals of intermediate blocks.
// destParent nil is complete false (no dest chain). src nil sticky has-skipped
// (no invent none / soft re-pick past hole).
// Incomplete LocalVars fails closed sticky as has-skipped (no invent none / soft re-pick).
func HasInitSkippedVars(src *Block, destParent *Block) bool {
	return HasInitSkippedVarsSess(testAmbientSession, src, destParent)
}

func HasInitSkippedVarsSess(s *Session, src *Block, destParent *Block) bool {
	if destParent == nil {
		return false
	}
	if src == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	skipped := CollectInitSkippedVarsSess(s, src, destParent)
	if !VariablesComplete(skipped) {
		// CollectInitSkippedVars already SetError sticky
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return true
	}
	return len(skipped) > 0
}

// CollectInitSkippedVars collects locals whose initialization is skipped by a jump.
// StatementGoto.cpp:281–306 — walk dest parent chain until src.
// Variable* always live on LocalVars; nil hole → IncompleteVariables (not bare nil —
// VariablesComplete(nil)/len==0 invent empty-complete skip list success).
// Complete scan with no skipped vars → empty non-nil slice.
// destParent nil → complete empty (no dest chain).
// Incomplete LocalVars fails closed sticky IncompleteVariables (no soft re-pick past hole).}

func CollectInitSkippedVars(src *Block, destParent *Block) []*Variable {
	return CollectInitSkippedVarsSess(testAmbientSession, src, destParent)
}

func CollectInitSkippedVarsSess(s *Session, src *Block, destParent *Block) []*Variable {
	if destParent == nil {
		return []*Variable{}
	}
	// StatementGoto.cpp:286–290 — climb dest->parent … until src
	reachedSrc := false
	intermediate := make([]*Variable, 0)
	for b := destParent; b != nil; b = b.Parent {
		if b == src {
			reachedSrc = true
			break
		}
		for _, loc := range b.LocalVars {
			if loc == nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteVariables()
			}
			intermediate = append(intermediate, loc)
		}
	}
	skipped := make([]*Variable, 0)
	for _, v := range intermediate {
		// StatementGoto.cpp:296–304 — after climb b==src → all intermediate skipped;
		// else !is_visible_local(src)
		if reachedSrc || src == nil {
			skipped = append(skipped, v)
			continue
		}
		visible := v.IsVisibleLocalSess(s, src)
		// residual ERROR sticky — no invent complete skip list past IsVisibleLocal hard IR hole
		if sessHasError(s) {
			return IncompleteVariables()
		}
		if !visible {
			skipped = append(skipped, v)
		}
	}
	return skipped
}

// OutputSkippedVarInits mirrors StatementGoto::output_skipped_var_inits.
// StatementGoto.cpp:264–275 — re-init skipped locals at destination label via init->Output.
// Incomplete InitSkippedVars fails closed sticky empty (no invent soft-skip hole partial re-inits).}

func OutputSkippedVarInits(st *Stmt, indent string) string {
	return OutputSkippedVarInitsSess(testAmbientSession, st, indent)
}

func OutputSkippedVarInitsSess(s *Session, st *Stmt, indent string) string {
	// Statement always live at goto dest re-init; sticky no invent re-inits without it
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// no skipped locals: soft empty
	if len(st.InitSkippedVars) == 0 {
		return ""
	}
	if !VariablesComplete(st.InitSkippedVars) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, v := range st.InitSkippedVars {
		// pre-validated VariablesComplete
		// StatementGoto.cpp:271 — assert(v->init); sticky no invent "name = ;" for missing init
		init := variableInitOutputSess(s, v)
		// residual ERROR sticky — no invent soft-continue later re-inits past init residual
		if sessHasError(s) {
			return ""
		}
		if init == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// get_actual_name always live; sticky no invent " = init;" without identifier
		name := v.GetActualNameSess(s, false)
		// residual ERROR sticky — no invent soft-continue later re-inits past GetActualName residual
		if sessHasError(s) {
			return ""
		}
		if name == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		b.WriteString(indent)
		b.WriteString(name)
		b.WriteString(" = ")
		b.WriteString(init)
		b.WriteString(";\n")
	}
	return b.String()
}

// variableInitOutput mirrors Variable::init->Output for re-init at goto dest.
// Prefer InitExpr (full Expression*); else Constant value.
// StatementGoto.cpp:271 — assert(v->init); v->init->Output(out) — no soft invent "0".}

func variableInitOutput(v *Variable) string {
	return variableInitOutputSess(testAmbientSession, v)
}

func variableInitOutputSess(s *Session, v *Variable) string {
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Variable.cpp:656 / OutputDef — InitExpr first
	if v.InitExpr != nil {
		out := v.InitExpr.OutputSess(s)
		// residual ERROR sticky — no invent soft-fallback Init past Output residual hole
		if sessHasError(s) {
			return ""
		}
		// incomplete InitExpr IR sticky — fail closed empty (no invent "0")
		if out != "" {
			return out
		}
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if v.Init != nil && v.Init.Value != "" {
		return v.Init.Value
	}
	// C++ assert(v->init); missing init is broken IR sticky
	sessNoteError(s, ErrGeneric)
	return ""
}

// copyBlocksNoHole copies blocks; nil hole sticky fails closed (ok=false).
// Block* always live on Function.Blocks; no invent soft-skip holes as absent.

func copyBlocksNoHole(blocks []*Block) (out []*Block, ok bool) {
	return copyBlocksNoHoleSess(testAmbientSession, blocks)
}

// copyBlocksNoHoleSess is copyBlocksNoHole with explicit session residual sticky.
func copyBlocksNoHoleSess(s *Session, blocks []*Block) (out []*Block, ok bool) {
	out = make([]*Block, 0, len(blocks))
	for _, b := range blocks {
		if b == nil {
			// incomplete Blocks sticky fail closed (no invent soft-skip hole as absent)
			sessNoteError(s, ErrGeneric)
			return nil, false
		}
		out = append(out, b)
	}
	return out, true
}

// FindGoodJumpBlock mirrors StatementGoto::find_good_jump_block.
// StatementGoto.cpp:309–354 — pick a block suitable as jump source/dest.
// asDest true: block is jump destination; false: block is jump source (contains goto).
// Mutates blocks slice by removing bad candidates (caller should pass a copy).
// Incomplete Blocks list fails closed sticky (no invent soft-skip hole / re-pick past hole).
func FindGoodJumpBlock(r *Rng, blocks []*Block, curr *Block, asDest bool) *Block {
	return FindGoodJumpBlockSess(testAmbientSession, r, blocks, curr, asDest)
}

func FindGoodJumpBlockSess(s *Session, r *Rng, blocks []*Block, curr *Block, asDest bool) *Block {
	// StatementGoto always has process RNG; sticky no invent jump block without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// empty blocks pool: soft re-pick (no candidates)
	if len(blocks) == 0 {
		return nil
	}
	// incomplete Blocks pool fails closed sticky (no invent soft-skip nil hole as absent)
	if !BlocksComplete(blocks) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// StatementGoto.cpp:314–324
	if curr != nil {
		if curr.InArrayLoop && !asDest {
			return nil
		}
		if len(curr.Stmts) == 0 && !asDest {
			return nil
		}
		if asDest {
			if last := curr.GetLastStmSess(s); last != nil {
				must := last.MustReturnSess(s)
				// residual ERROR sticky — no invent soft-reject/allow dest past MustReturn residual
				if sessHasError(s) {
					return nil
				}
				if must {
					return nil
				}
			}
		}
	}
	// work on a mutable copy (C++ mutates vector in place via erase)
	// pre-validated BlocksComplete
	blks := append([]*Block(nil), blocks...)
	for len(blks) > 0 {
		idx := int(r.RndUptoSess(s, uint32(len(blks))))
		// StatementGoto.cpp:326 ERROR_GUARD
		if sessHasError(s) {
			return nil
		}
		b := blks[idx]
		// StatementGoto.cpp:328–331 — disallow array-loop dest
		if b.InArrayLoop && asDest {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		// StatementGoto.cpp:333–336 — empty stms
		if len(b.Stmts) == 0 {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		// StatementGoto.cpp:339–343 — sole return cannot be jump source
		if !asDest && len(b.Stmts) == 1 && b.Stmts[0].Kind == StmtReturn {
			blks = append(blks[:idx], blks[idx+1:]...)
			continue
		}
		if b == curr {
			return b
		}
		// StatementGoto.cpp:348–352 — has_init_skipped_vars(src, dest_stmt)
		// as_dest: (curr, b.stms[0]); !as_dest: (b, curr.stms[0])
		if asDest {
			if curr != nil && HasInitSkippedVarsSess(s, curr, b) {
				// residual ERROR sticky — no invent soft-continue then pick later past hole
				if sessHasError(s) {
					return nil
				}
				blks = append(blks[:idx], blks[idx+1:]...)
				continue
			}
		} else {
			if curr != nil && len(curr.Stmts) > 0 && HasInitSkippedVarsSess(s, b, curr) {
				// residual ERROR sticky — no invent soft-continue then pick later past hole
				if sessHasError(s) {
					return nil
				}
				blks = append(blks[:idx], blks[idx+1:]...)
				continue
			}
		}
		return b
	}
	return nil
}

// makeGotoFailed is StatementGoto::make_random returning nullptr (empty Stmt;
// no invent Kind-only shell — stmtOK rejects zero-value).}

func makeGotoFailed() Stmt {
	return Stmt{}
}

// MakeRandomGoto mirrors StatementGoto::make_random.
// StatementGoto.cpp:61–212 — find_good_jump_block; choose_visible_read_var;
// back-edge returns goto; forward inserts after other_stm and returns nullptr.
// cg is *CGContext (C++ CGContext&) so effect_stm clear sticks.
func MakeRandomGoto(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	blk *Block,
) Stmt {
	_ = probs
	// StatementGoto always has RNG + CG + curr_func; sticky no invent shell without them
	if r == nil || cg == nil || cg.CurrentFunc == nil {
		noteErrCG(cg, ErrGeneric)
		return makeGotoFailed()
	}
	// StatementGoto.cpp:66–67 — FactMgr always present (get_fact_mgr);
	// non-sticky soft re-pick when FM missing (sticky poisons MakeRandomGoto soft factory)
	if cg.FM == nil {
		return makeGotoFailed()
	}
	// incomplete ambient fails closed sticky (no invent goto / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return makeGotoFailed()
	}
	// incomplete GlobalFacts fail closed sticky (no invent goto under hole shells)
	if !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return makeGotoFailed()
	}

	// 40% prefer back-edge (StatementGoto.cpp:73–84)
	wantBack := r.RndFlipcoinSess(sessFromCG(cg), 40)
	// StatementGoto.cpp:74 ERROR_GUARD
	if hasErrCG(cg) {
		return makeGotoFailed()
	}
	// StatementGoto.cpp:70–84 — vector copy of func->blocks only (no invent append curr)
	// Block* always live on Function.Blocks; nil hole fails closed sticky
	blocks, ok := copyBlocksNoHoleSess(sessFromCG(cg), cg.CurrentFunc.Blocks)
	if !ok {
		noteErrCG(cg, ErrGeneric)
		return makeGotoFailed()
	}

	var okBlk *Block
	backEdge := false
	if wantBack {
		// as_dest=true: pick destination for backward jump
		okBlk = FindGoodJumpBlockSess(sessFromCG(cg), r, blocks, blk, true)
		if okBlk != nil {
			backEdge = true
		}
	}
	if okBlk == nil {
		// StatementGoto.cpp:81–84 — forward: re-copy func->blocks; as_dest=false
		backEdge = false
		blocks, ok = copyBlocksNoHoleSess(sessFromCG(cg), cg.CurrentFunc.Blocks)
		if !ok {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		okBlk = FindGoodJumpBlockSess(sessFromCG(cg), r, blocks, blk, false)
	}
	if okBlk == nil {
		// StatementGoto.cpp:86–87 — return nullptr
		return makeGotoFailed()
	}

	// StatementGoto.cpp:89–92 — stm is curr_blk->stms.back() (jump dest for forward)
	var dest *Stmt
	if blk != nil && len(blk.Stmts) > 0 {
		dest = &blk.Stmts[len(blk.Stmts)-1]
	}

	// StatementGoto.cpp:97–106 — s != stm && !must_return only (pointer identity).
	// Soft invent also excluded by StmID equality across blocks (not in C++) which
	// could shrink ok_stms / cond pool when ids collided or were reused incorrectly.
	var okStms []int
	for i := range okBlk.Stmts {
		s := &okBlk.Stmts[i]
		if dest != nil && s == dest {
			continue
		}
		if s.MustReturnSess(sessFromCG(cg)) {
			// residual ERROR sticky — no invent soft-skip must-return then pick later target
			if hasErrCG(cg) {
				return makeGotoFailed()
			}
			continue
		}
		okStms = append(okStms, i)
	}
	if len(okStms) == 0 {
		// StatementGoto.cpp:109–212 — empty ok_stms → fall through to nullptr
		return makeGotoFailed()
	}
	ti := okStms[r.RndUptoSess(sessFromCG(cg), uint32(len(okStms)))]
	// StatementGoto.cpp:110 ERROR_GUARD
	if hasErrCG(cg) {
		return makeGotoFailed()
	}
	other := &okBlk.Stmts[ti]
	if StmIDUnset(other.StmID) {
		other.StmID = AllocStmIDSess(sessFromCG(cg))
	}

	// StatementGoto.cpp:112 — clear effect_stm after other_stm pick (not before)
	cg.ClearEffectStm()

	// condition: prefer already-read visible var (StatementGoto.cpp:117–132)
	// C++ choose_visible_read_var facts arg:
	//   back:  fm->global_facts (live ePointTo+eUnionWrite)
	//   forward: fm->map_facts_out[other_stm] (historical out lattice at jump src)
	// Go splits eUnionWrite → UnionFacts / MapUnionFactsOut. Soft invent was
	// always using live UnionFacts on forward → IsNonreadableField over-filter
	// (or wrong last-writes) → empty cond pool → goto fail → first_div (seed 42).
	var uf []*FactUnion
	var readVars []*Variable
	condBlk := blk
	if backEdge {
		// StatementGoto.cpp:119–122 — accum read_vars + global_facts
		if cg.EffectAccum != nil {
			readVars = cg.EffectAccum.ReadVarsSess(sessFromCG(cg))
		}
		if cg.FM != nil {
			uf = cg.FM.UnionFacts
		}
	} else {
		// StatementGoto.cpp:125–128 — map_accum_effect[other] read_vars +
		// map_facts_out[other] for is_nonreadable_field
		// C++ map[] always (missing live id → empty); StmID 0 IncompleteEffect
		condBlk = okBlk
		if cg.FM != nil {
			readVars = cg.FM.GetMapAccumEffect(other.StmID).ReadVarsSess(sessFromCG(cg))
			// residual ERROR sticky — no invent soft-empty read past GetMapAccum residual
			if hasErrCG(cg) {
				return makeGotoFailed()
			}
			uf = cg.FM.GetMapUnionFactsOut(other.StmID)
			// residual ERROR sticky — no invent soft-live UnionFacts past MapUnionOut hole
			if hasErrCG(cg) {
				return makeGotoFailed()
			}
		}
	}
	var cond *Expression
	if len(readVars) > 0 {
		if v := ChooseVisibleReadVarOptsSess(sessFromCG(cg), r, condBlk, readVars, GetIntTypeSess(sessFromCG(cg)), uf, sessOpts(sessFromCG(cg))); v != nil {
			// StatementGoto.cpp:131–133 — ExpressionVariable(*cond_var) only.
			// C++ does not call read_var here; visit_facts later uses check_read_var.
			// Soft invent was NoteRead/ReadVar during make_random, which pushed the
			// cond into effect_accum+effect_stm early and bloated map_accum_effect
			// / later ambient (binary RHS seFree / write filters).
			cond = &Expression{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(sessFromCG(cg))}
		}
	}
	// StatementGoto.cpp:130–132 — return nullptr when cond_var missing
	if cond == nil {
		return makeGotoFailed()
	}

	// util.cpp gensym_count process-wide; no invent VS.Sym private or fixed "lbl_1"
	nextLab := func() string {
		return GensymSess(sessFromCG(cg), "lbl_")
	}

	if backEdge {
		// StatementGoto.cpp:138–150 — goto in curr jumps to other_stm
		label := other.SourceLabel
		if label == "" {
			label = LabelForGotoDestSess(sessFromCG(cg), other.StmID, nextLab)
			other.SourceLabel = label
		} else if !StmIDUnset(other.StmID) {
			setStmLabelSess(sessFromCG(cg), other.StmID, label)
		}
		// incomplete LocalVars on intermediate blocks fails closed sticky (Collect nil)
		// no invent goto with empty InitSkippedVars when skip list is incomplete
		skipped := CollectInitSkippedVarsSess(sessFromCG(cg), blk, okBlk)
		if !VariablesComplete(skipped) {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		st := Stmt{
			Kind: StmtGoto, Expr: cond, Label: label, StmID: AllocStmIDSess(sessFromCG(cg)),
			GotoBack:        true,
			GotoDestStmID:   other.StmID,
			GotoDestParent:  okBlk,
			InitSkippedVars: skipped,
		}
		// StatementGoto.cpp:139 — create_cfg_edge(sg, other_stm, false, true)
		if cg.FM != nil {
			cg.FM.CreateCFGEdgeTo(st.StmID, okBlk, other.StmID, false, true)
		}
		// StatementGoto.cpp:141–147 — LCA need_revisit
		// okBlk is other_stm->parent (dest Statement::parent).
		MarkNeedRevisitLCAParentSess(sessFromCG(cg), blk, other, okBlk)
		// StatementGoto.cpp:149 — Bookkeeper::backward_jump_cnt++
		RecordBackwardJumpSess(sessFromCG(cg))
		return st
	}

	// Forward path — StatementGoto.cpp:152–211
	// Dest is last stmt of curr; goto is inserted after other_stm in okBlk.
	// C++ returns nullptr after insert (side-effect only); stmtOK rejects empty label.
	if dest == nil {
		// no stm in curr_blk yet — cannot form forward dest
		return makeGotoFailed()
	}
	if StmIDUnset(dest.StmID) {
		dest.StmID = AllocStmIDSess(sessFromCG(cg))
	}
	// StatementGoto.cpp:185 — StatementGoto ctor gensyms label only after DFA
	// validation succeeds. Do not gensym here: failed visit_facts/merge must not
	// burn util.cpp gensym_count (seed-2: extra lbl_710 desynced all later names).

	fm := cg.FM
	foundNewFacts := false
	var gotoIn, gotoOut, stmInMerged, stmOut []*FactPointTo
	var gotoInU, gotoOutU, stmInMergedU, stmOutU []*FactUnion
	if fm != nil {
		// StatementGoto.cpp:159–162 — ctrl uses facts_in, else facts_out
		// Full FactVec (ePointTo + eUnionWrite). Soft invent was PT-only:
		// merge_jump never BOTTOM-joined missing eUnionWrite, and post-insert
		// global_facts install never rewound UnionFacts (seed-104: else-start
		// forward goto left g_111 last=0 vs UP BOTTOM → ChooseOKVar +1).
		// C++ map[] always; incomplete maps fail closed (no invent partial goto)
		var srcFacts []*FactPointTo
		var srcUnions []*FactUnion
		if IsCtrlStmt(other) {
			srcFacts = fm.GetMapFactsIn(other.StmID)
			srcUnions = fm.GetMapUnionFactsIn(other.StmID)
		} else {
			srcFacts = fm.GetMapFactsOut(other.StmID)
			srcUnions = fm.GetMapUnionFactsOut(other.StmID)
		}
		if !FactsComplete(srcFacts) || !UnionFactsComplete(srcUnions) {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		gotoIn = CloneFactSliceSess(sessFromCG(cg), srcFacts)
		gotoInU = CloneUnionFactSliceDeepSess(sessFromCG(cg), srcUnions)
		if hasErrCG(cg) || !UnionFactsComplete(gotoInU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		if gotoInU == nil {
			gotoInU = []*FactUnion{}
		}
		// StatementGoto.cpp:163 — update_facts_for_dest(goto_in, goto_out, stm)
		// Full FactVec: PT half + eUnionWrite half (merge then OOS drop).
		UpdateFactsForDestSess(sessFromCG(cg), gotoIn, &gotoOut, fm.Func, blk)
		if hasErrCG(cg) || !FactsComplete(gotoOut) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		UpdateUnionFactsForDestSess(sessFromCG(cg), gotoInU, &gotoOutU, fm.Func, blk)
		if hasErrCG(cg) || !UnionFactsComplete(gotoOutU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		// StatementGoto.cpp:164–166 — merge effect from goto src (map[] zero if missing live id)
		// Incomplete map_accum_effect fails closed sticky (no invent AddEffect poison then success)
		preEffect := cg.AccumEffect()
		srcAcc := fm.GetMapAccumEffect(other.StmID)
		if !EffectComplete(srcAcc) {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		cg.AddEffect(srcAcc, true)
		if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			cg.ResetEffectAccum(preEffect)
			return makeGotoFailed()
		}
		// StatementGoto.cpp:167–182
		// FactMgr.cpp:569–588 merge_jump_facts is full FactVec.
		destIn := fm.GetMapFactsIn(dest.StmID)
		destInU := fm.GetMapUnionFactsIn(dest.StmID)
		if !FactsComplete(destIn) || !UnionFactsComplete(destInU) {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		stmInMerged = CloneFactSliceSess(sessFromCG(cg), destIn)
		stmInMergedU = CloneUnionFactSliceDeepSess(sessFromCG(cg), destInU)
		if hasErrCG(cg) || !UnionFactsComplete(stmInMergedU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		if stmInMergedU == nil {
			stmInMergedU = []*FactUnion{}
		}
		// Snapshot lattice for change detection (merge_jump may BOTTOM missing unions).
		preUnionLast := map[*Variable]int{}
		for _, uf := range stmInMergedU {
			if uf != nil && uf.Var != nil {
				preUnionLast[uf.Var] = uf.LastWrittenFID
			}
		}
		// tryMerge distinguishes incomplete wipe from complete no-change
		// (no invent treat MergeJumpFacts false as "unchanged" after wipe)
		changed, mok := tryMergeJumpFactsSess(sessFromCG(cg), &stmInMerged, gotoOut)
		if !mok {
			noteErrCG(cg, ErrGeneric)
			return makeGotoFailed()
		}
		if !mergeJumpUnionFactsSess(sessFromCG(cg), &stmInMergedU, gotoOutU) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		unionChanged := false
		if len(stmInMergedU) != len(preUnionLast) {
			unionChanged = true
		} else {
			for _, uf := range stmInMergedU {
				if uf == nil || uf.Var == nil {
					unionChanged = true
					break
				}
				if prev, ok := preUnionLast[uf.Var]; !ok || prev != uf.LastWrittenFID {
					unionChanged = true
					break
				}
			}
		}
		if changed || unionChanged {
			stmOut = CloneFactSliceSess(sessFromCG(cg), stmInMerged)
			stmOutU = CloneUnionFactSliceDeepSess(sessFromCG(cg), stmInMergedU)
			if hasErrCG(cg) || !UnionFactsComplete(stmOutU) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			foundNewFacts = true
			factsInCopy := make(map[int][]*FactPointTo)
			factsOutCopy := make(map[int][]*FactPointTo)
			unionInCopy := make(map[int][]*FactUnion)
			unionOutCopy := make(map[int][]*FactUnion)
			fm.BackupStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
			// feed merged facts as global for visit (stm_visit_facts inputs)
			// Full FactVec: ePointTo + eUnionWrite (FactMgr.cpp backup/restore).
			if !FactsComplete(stmInMerged) || !UnionFactsComplete(stmInMergedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				noteErrCG(cg, ErrGeneric)
				return makeGotoFailed()
			}
			// map_facts_in[dest] is the pre-make snapshot of dest and can lack
			// facts for locals created later in the same block (or during dest
			// generation). C++ stm_visit_facts mutates only the inputs FactVec
			// while global_facts stays live; after visit it assigns
			// global_facts = map_facts_out[dest]. Go VisitFacts uses GlobalFacts
			// as the working set, so replacing it with raw map_in wipes later
			// locals (seed-2 e19427: l_432 fact lost → opportunistic_validate
			// fail → extra Select). MakeupNewVarFacts restores those locals
			// from the live GlobalFacts into the visit inputs (FactMgr.cpp:494–508).
			// Keep post–merge_jump lattice (e.g. BOTTOM) — do not reload map_in.
			liveSaved := CloneFactSliceSess(sessFromCG(cg), fm.GlobalFacts)
			liveSavedU := CloneUnionFactSliceDeepSess(sessFromCG(cg), fm.UnionFacts)
			if !FactsComplete(liveSaved) || !UnionFactsComplete(liveSavedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			if !MakeupNewVarFactsSess(sessFromCG(cg), &stmInMerged, liveSaved) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			if !makeupNewUnionFactsSess(sessFromCG(cg), &stmInMergedU, liveSavedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			if !UnionFactsComplete(stmInMergedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				noteErrCG(cg, ErrGeneric)
				return makeGotoFailed()
			}
			// eUnionWrite visit inputs (StmVisitFacts swaps only ePointTo GlobalFacts)
			// Deep install so visit join/renew cannot alias map_facts_in subjects.
			if stmInMergedU == nil {
				fm.UnionFacts = []*FactUnion{}
			} else {
				clU := CloneUnionFactSliceDeepSess(sessFromCG(cg), stmInMergedU)
				if hasErrCG(cg) || !UnionFactsComplete(clU) {
					fm.UnionFacts = liveSavedU
					fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
					cg.ResetEffectAccum(preEffect)
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
					}
					return makeGotoFailed()
				}
				if clU == nil {
					fm.UnionFacts = []*FactUnion{}
				} else {
					fm.UnionFacts = clU
				}
			}
			// StatementGoto.cpp:171 — stm->stm_visit_facts(stm_out, cg_context)
			// Statement.cpp:611 — get_effect_stm().clear() before visit_facts.
			// Soft invent called VisitFactsStmt after add_effect(map_accum[other]),
			// so EffectStm kept pollution and StatementFor::visit_facts snapshotted
			// a non-empty pre-init effect into map_stm_effect[for] (seed-42 func_68:
			// gen IV read g_77 first vs UP visit order g_16 g_22 g_77).
			// Do not SetGlobalFacts(work) here: StmVisitFacts captures live
			// GlobalFacts as restore target then loads *facts as the working set.
			work := CloneFactSliceSess(sessFromCG(cg), stmInMerged)
			if hasErrCG(cg) || !FactsComplete(work) {
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			// Statement.cpp:612 — curr_blk = parent of dest (forward: curr/blk)
			cg.CurrBlk = blk
			if !StmVisitFacts(dest, &work, cg, opts) {
				// StmVisitFacts restores point-to GlobalFacts to pre-call live
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				return makeGotoFailed()
			}
			// visit outs are in work (C++ stm_out) + fm.UnionFacts; incomplete fails closed
			if !FactsComplete(work) || !UnionFactsComplete(fm.UnionFacts) {
				fm.SetGlobalFacts(liveSaved, "auto_statement_goto_restore")
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				noteErrCG(cg, ErrGeneric)
				return makeGotoFailed()
			}
			stmOut = work
			// Capture post-visit eUnionWrite before restoring pre-visit live
			// (set_fact_out pairs this lattice; C++ stm_out is the visit inputs FactVec).
			stmOutU = CloneUnionFactSliceDeepSess(sessFromCG(cg), fm.UnionFacts)
			if hasErrCG(cg) || !UnionFactsComplete(stmOutU) {
				fm.SetGlobalFacts(liveSaved, "auto_statement_goto_restore")
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return makeGotoFailed()
			}
			if stmOutU == nil {
				stmOutU = []*FactUnion{}
			}
			// C++ leaves global_facts alone until StatementGoto.cpp:204; restore
			// union half to pre-visit live (StmVisitFacts already restored PT).
			fm.UnionFacts = liveSavedU
			// StatementGoto.cpp:178–181 — if dest contains other, recompute goto_out.
			// C++ goto_in is a live reference to map_facts_in/out[other_stm]; visit may
			// have updated those maps (StatementGoto.cpp:179–180). Soft invent froze
			// gotoIn at pre-visit clone so recompute used stale lattice (seed 114667:
			// goto map_out kept g_124={g_106} while live other map_out had l_2181 range
			// → contains_unfixed_goto false → pure-shortcut need_revisit LCA →
			// Func.Blocks n=37 vs UP n=3 at FindGoodJumpBlock).
			if ContainsStmt(dest, other) {
				if IsCtrlStmt(other) {
					gotoIn = CloneFactSliceSess(sessFromCG(cg), fm.GetMapFactsIn(other.StmID))
					gotoInU = CloneUnionFactSliceDeepSess(sessFromCG(cg), fm.GetMapUnionFactsIn(other.StmID))
				} else {
					gotoIn = CloneFactSliceSess(sessFromCG(cg), fm.GetMapFactsOut(other.StmID))
					gotoInU = CloneUnionFactSliceDeepSess(sessFromCG(cg), fm.GetMapUnionFactsOut(other.StmID))
				}
				if hasErrCG(cg) || !FactsComplete(gotoIn) || !UnionFactsComplete(gotoInU) {
					fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
					cg.ResetEffectAccum(preEffect)
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
					}
					return makeGotoFailed()
				}
				if gotoInU == nil {
					gotoInU = []*FactUnion{}
				}
				gotoOut = nil
				gotoOutU = nil
				UpdateFactsForDestSess(sessFromCG(cg), gotoIn, &gotoOut, fm.Func, blk)
				UpdateUnionFactsForDestSess(sessFromCG(cg), gotoInU, &gotoOutU, fm.Func, blk)
				if hasErrCG(cg) || !FactsComplete(gotoOut) || !UnionFactsComplete(gotoOutU) {
					fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
					cg.ResetEffectAccum(preEffect)
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
					}
					return makeGotoFailed()
				}
			}
		}
	}

	// StatementGoto.cpp:184–192 — insert goto after other_stm in other_blk
	// incomplete LocalVars on intermediate blocks fails closed sticky (IncompleteVariables)
	// no invent forward goto with empty InitSkippedVars when skip list is incomplete
	skippedFwd := CollectInitSkippedVarsSess(sessFromCG(cg), okBlk, blk)
	if !VariablesComplete(skippedFwd) {
		noteErrCG(cg, ErrGeneric)
		return makeGotoFailed()
	}
	// StatementGoto.cpp:185 + ctor 220–229 — gensym only on successful insert path.
	// Capture dest identity by StmID before any slice insert: C++ Statement* is heap-
	// stable; Go Stmt values live in slices. Mid-slice insert into the same block
	// (okBlk == blk) shifts elements so &blk.Stmts[i] no longer names dest
	// (seed-42: CreateCFGEdgeTo / set_fact used the shifted slot → label on the
	// wrong statement: UP lbl before p_64 assign, GO before prior assign).
	if dest == nil {
		return makeGotoFailed()
	}
	if StmIDUnset(dest.StmID) {
		dest.StmID = AllocStmIDSess(sessFromCG(cg))
	}
	destID := dest.StmID
	destIsCtrl := IsCtrlStmt(dest) || dest.Kind == StmtReturn
	// residual ERROR sticky — no invent soft-insert past IsCtrlStmt residual
	if hasErrCG(cg) {
		return makeGotoFailed()
	}
	label := dest.SourceLabel
	if label == "" {
		label = LabelForGotoDestSess(sessFromCG(cg), destID, nextLab)
		if label == "" {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return makeGotoFailed()
		}
		dest.SourceLabel = label
	} else {
		setStmLabelSess(sessFromCG(cg), destID, label)
	}
	sg := Stmt{
		Kind:            StmtGoto,
		Expr:            cond,
		Label:           label,
		StmID:           AllocStmIDSess(sessFromCG(cg)),
		GotoForward:     true,
		GotoDestStmID:   destID,
		GotoDestParent:  blk,
		InitSkippedVars: skippedFwd,
	}
	// re-resolve other index by StmID (other *Stmt may also be invalid after insert)
	otherID := other.StmID
	insertAt := -1
	for i := range okBlk.Stmts {
		if okBlk.Stmts[i].StmID == otherID {
			insertAt = i
			break
		}
	}
	if insertAt < 0 {
		insertAt = ti
	}
	okBlk.Stmts = append(okBlk.Stmts[:insertAt+1], append([]Stmt{sg}, okBlk.Stmts[insertAt+1:]...)...)
	// re-apply SourceLabel on dest by id after possible same-slice shift
	if blk != nil {
		for i := range blk.Stmts {
			if blk.Stmts[i].StmID == destID {
				blk.Stmts[i].SourceLabel = label
				break
			}
		}
	}
	// pointer to inserted stmt for fact maps (valid after append)
	ins := &okBlk.Stmts[insertAt+1]

	if fm != nil {
		// StatementGoto.cpp:195–202 — set_fact_in/out for goto is full FactVec
		// (goto_in / goto_out from update_facts_for_dest, both partitions).
		if gotoInU == nil {
			gotoInU = []*FactUnion{}
		}
		if gotoOutU == nil {
			gotoOutU = []*FactUnion{}
		}
		fm.SetMapFactsInPair(ins.StmID, gotoIn, gotoInU)
		fm.SetMapFactsOutPair(ins.StmID, gotoOut, gotoOutU)
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[ins.StmID] = true
		if foundNewFacts {
			// StatementGoto.cpp:200–201 — set_fact_in(stm, stm_in); set_fact_out(stm, stm_out)
			// Full FactVec including post–merge_jump / post-visit eUnionWrite.
			if stmInMergedU == nil {
				stmInMergedU = []*FactUnion{}
			}
			if stmOutU == nil {
				stmOutU = []*FactUnion{}
			}
			fm.SetMapFactsInPair(destID, stmInMerged, stmInMergedU)
			fm.SetMapFactsOutPair(destID, stmOut, stmOutU)
		}
		// StatementGoto.cpp:203 — create_cfg_edge(sg, stm, false, false)
		fm.CreateCFGEdgeTo(ins.StmID, blk, destID, false, false)
		// StatementGoto.cpp:204–210 — global_facts = map_facts_out[stm]
		// Full FactVec (ePointTo + eUnionWrite). Soft invent was SetGlobalFacts(PT-only)
		// so live UnionFacts never rewound after forward goto (seed-104 g_111).
		// Incomplete out/in fails closed sticky (no invent soft re-pick past wiped facts)
		if destIsCtrl {
			// ctrl/return: use map_facts_in[stm] (altered outs for OOS)
			fm.AssignGlobalFactsFromMapIn(destID)
		} else {
			fm.AssignGlobalFactsFromMapOut(destID)
		}
	}
	// StatementGoto.cpp:211
	RecordForwardJumpSess(sessFromFM(fm))
	// StatementGoto.cpp:212 — return nullptr (goto already in other_blk)
	return makeGotoFailed()
}
