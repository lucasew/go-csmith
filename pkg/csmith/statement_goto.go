// Upstream: StatementGoto.cpp (make_random) with back/forward edge selection.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// stmLabels mirrors StatementGoto::stm_labels — dest statement → shared label.
// StatementGoto.cpp:55, 224–229.
var stmLabels = map[int]string{}

// LabelForGotoDest returns existing or new gensym label for a jump destination.
// StatementGoto.cpp:224–229 — reuse stm_labels[dest] when present; else gensym("lbl_").
// no invent fixed "lbl_1" when nextLabel is nil
func LabelForGotoDest(destStmID int, nextLabel func() string) string {
	if !StmIDUnset(destStmID) {
		if lab, ok := stmLabels[destStmID]; ok && lab != "" {
			return lab
		}
	}
	// StatementGoto.cpp:227 — gensym("lbl_"); process-wide util.cpp counter
	lab := ""
	if nextLabel != nil {
		lab = nextLabel()
	} else {
		lab = Gensym("lbl_")
	}
	// incomplete empty label is broken IR sticky — fail closed (no invent "goto :")
	if lab == "" {
		SetError(ErrGeneric)
		return ""
	}
	if !StmIDUnset(destStmID) {
		stmLabels[destStmID] = lab
	}
	return lab
}

// GotoLabelsDoFinalization mirrors StatementGoto::doFinalization.
// StatementGoto.cpp:404 — stm_labels.clear().
func GotoLabelsDoFinalization() {
	stmLabels = map[int]string{}
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
// Block::contains_stmt light — used for NeedRevisit LCA on back-edge goto.
// Kind-gated get_blocks only; nil arm sticky false (no invent membership
// by soft-skipping a missing if-arm / stray Then on assign).
func (b *Block) ContainsStmt(st *Stmt) bool {
	// Block + Statement always live; sticky incomplete no invent not-contain soft-skip
	if b == nil || st == nil {
		SetError(ErrGeneric)
		return false
	}
	for i := range b.Stmts {
		s := &b.Stmts[i]
		if s == st {
			return true
		}
		if !StmIDUnset(st.StmID) && s.StmID == st.StmID {
			return true
		}
		blks := GetBlocksStmt(s)
		for _, nb := range blks {
			if nb == nil {
				// incomplete arm sticky fail closed not-contain
				SetError(ErrGeneric)
				return false
			}
		}
		for _, nb := range blks {
			if nb.ContainsStmt(st) {
				return true
			}
		}
	}
	return false
}

// MarkNeedRevisitLCA sets NeedRevisit on the least incomplete ancestor of curr
// that contains dest (StatementGoto.cpp:141–147).
func MarkNeedRevisitLCA(curr *Block, dest *Stmt) {
	// StatementGoto.cpp:143–147 — while (!contains) b=parent; assert(b); need_revisit
	// no soft invent NeedRevisit on curr when no ancestor contains dest
	for b := curr; b != nil; b = b.Parent {
		if b.ContainsStmt(dest) {
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
	if destParent == nil {
		return false
	}
	if src == nil {
		SetError(ErrGeneric)
		return true
	}
	skipped := CollectInitSkippedVars(src, destParent)
	if !VariablesComplete(skipped) {
		// CollectInitSkippedVars already SetError sticky
		if !HasError() {
			SetError(ErrGeneric)
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
// Incomplete LocalVars fails closed sticky IncompleteVariables (no soft re-pick past hole).
func CollectInitSkippedVars(src *Block, destParent *Block) []*Variable {
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
				SetError(ErrGeneric)
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
		visible := v.IsVisibleLocal(src)
		// residual ERROR sticky — no invent complete skip list past IsVisibleLocal hard IR hole
		if HasError() {
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
// Incomplete InitSkippedVars fails closed sticky empty (no invent soft-skip hole partial re-inits).
func OutputSkippedVarInits(st *Stmt, indent string) string {
	// Statement always live at goto dest re-init; sticky no invent re-inits without it
	if st == nil {
		SetError(ErrGeneric)
		return ""
	}
	// no skipped locals: soft empty
	if len(st.InitSkippedVars) == 0 {
		return ""
	}
	if !VariablesComplete(st.InitSkippedVars) {
		SetError(ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, v := range st.InitSkippedVars {
		// pre-validated VariablesComplete
		// StatementGoto.cpp:271 — assert(v->init); sticky no invent "name = ;" for missing init
		init := variableInitOutput(v)
		// residual ERROR sticky — no invent soft-continue later re-inits past init residual
		if HasError() {
			return ""
		}
		if init == "" {
			SetError(ErrGeneric)
			return ""
		}
		// get_actual_name always live; sticky no invent " = init;" without identifier
		name := v.GetActualName(false)
		// residual ERROR sticky — no invent soft-continue later re-inits past GetActualName residual
		if HasError() {
			return ""
		}
		if name == "" {
			SetError(ErrGeneric)
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
// StatementGoto.cpp:271 — assert(v->init); v->init->Output(out) — no soft invent "0".
func variableInitOutput(v *Variable) string {
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:656 / OutputDef — InitExpr first
	if v.InitExpr != nil {
		out := v.InitExpr.Output()
		// residual ERROR sticky — no invent soft-fallback Init past Output residual hole
		if HasError() {
			return ""
		}
		// incomplete InitExpr IR sticky — fail closed empty (no invent "0")
		if out != "" {
			return out
		}
		SetError(ErrGeneric)
		return ""
	}
	if v.Init != nil && v.Init.Value != "" {
		return v.Init.Value
	}
	// C++ assert(v->init); missing init is broken IR sticky
	SetError(ErrGeneric)
	return ""
}

// copyBlocksNoHole copies blocks; nil hole sticky fails closed (ok=false).
// Block* always live on Function.Blocks; no invent soft-skip holes as absent.
func copyBlocksNoHole(blocks []*Block) (out []*Block, ok bool) {
	out = make([]*Block, 0, len(blocks))
	for _, b := range blocks {
		if b == nil {
			// incomplete Blocks sticky fail closed (no invent soft-skip hole as absent)
			SetError(ErrGeneric)
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
	// StatementGoto always has process RNG; sticky no invent jump block without it
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// empty blocks pool: soft re-pick (no candidates)
	if len(blocks) == 0 {
		return nil
	}
	// incomplete Blocks pool fails closed sticky (no invent soft-skip nil hole as absent)
	if !BlocksComplete(blocks) {
		SetError(ErrGeneric)
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
			if last := curr.GetLastStm(); last != nil {
				must := last.MustReturn()
				// residual ERROR sticky — no invent soft-reject/allow dest past MustReturn residual
				if HasError() {
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
		idx := int(r.RndUpto(uint32(len(blks))))
		// StatementGoto.cpp:326 ERROR_GUARD
		if HasError() {
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
			if curr != nil && HasInitSkippedVars(curr, b) {
				// residual ERROR sticky — no invent soft-continue then pick later past hole
				if HasError() {
					return nil
				}
				blks = append(blks[:idx], blks[idx+1:]...)
				continue
			}
		} else {
			if curr != nil && len(curr.Stmts) > 0 && HasInitSkippedVars(b, curr) {
				// residual ERROR sticky — no invent soft-continue then pick later past hole
				if HasError() {
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
// no invent Kind-only shell — stmtOK rejects zero-value).
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
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return makeGotoFailed()
	}
	// incomplete GlobalFacts fail closed sticky (no invent goto under hole shells)
	if !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return makeGotoFailed()
	}

	// 40% prefer back-edge (StatementGoto.cpp:73–84)
	wantBack := r.RndFlipcoin(40)
	// StatementGoto.cpp:74 ERROR_GUARD
	if HasError() {
		return makeGotoFailed()
	}
	// StatementGoto.cpp:70–84 — vector copy of func->blocks only (no invent append curr)
	// Block* always live on Function.Blocks; nil hole fails closed sticky
	blocks, ok := copyBlocksNoHole(cg.CurrentFunc.Blocks)
	if !ok {
		SetError(ErrGeneric)
		return makeGotoFailed()
	}

	var okBlk *Block
	backEdge := false
	if wantBack {
		// as_dest=true: pick destination for backward jump
		okBlk = FindGoodJumpBlock(r, blocks, blk, true)
		if okBlk != nil {
			backEdge = true
		}
	}
	if okBlk == nil {
		// StatementGoto.cpp:81–84 — forward: re-copy func->blocks; as_dest=false
		backEdge = false
		blocks, ok = copyBlocksNoHole(cg.CurrentFunc.Blocks)
		if !ok {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		okBlk = FindGoodJumpBlock(r, blocks, blk, false)
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

	// StatementGoto.cpp:97–106 — s != stm && !must_return only (not soft-filter break/continue)
	var okStms []int
	for i := range okBlk.Stmts {
		s := &okBlk.Stmts[i]
		if dest != nil && s == dest {
			continue
		}
		// also match by StmID when dest is in a different slice cell with same id
		if dest != nil && s.StmID != 0 && dest.StmID != 0 && s.StmID == dest.StmID {
			continue
		}
		if s.MustReturn() {
			// residual ERROR sticky — no invent soft-skip must-return then pick later target
			if HasError() {
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
	ti := okStms[r.RndUpto(uint32(len(okStms)))]
	// StatementGoto.cpp:110 ERROR_GUARD
	if HasError() {
		return makeGotoFailed()
	}
	other := &okBlk.Stmts[ti]
	if StmIDUnset(other.StmID) {
		other.StmID = AllocStmID()
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
			readVars = cg.EffectAccum.ReadVars()
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
			readVars = cg.FM.GetMapAccumEffect(other.StmID).ReadVars()
			// residual ERROR sticky — no invent soft-empty read past GetMapAccum residual
			if HasError() {
				return makeGotoFailed()
			}
			uf = cg.FM.GetMapUnionFactsOut(other.StmID)
			// residual ERROR sticky — no invent soft-live UnionFacts past MapUnionOut hole
			if HasError() {
				return makeGotoFailed()
			}
		}
	}
	var cond *Expression
	if len(readVars) > 0 {
		if v := ChooseVisibleReadVar(r, condBlk, readVars, GetIntType(), uf); v != nil {
			cg.NoteRead(v)
			cond = &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
		}
	}
	// StatementGoto.cpp:130–132 — return nullptr when cond_var missing
	if cond == nil {
		return makeGotoFailed()
	}

	// util.cpp gensym_count process-wide; no invent VS.Sym private or fixed "lbl_1"
	nextLab := func() string {
		return Gensym("lbl_")
	}

	if backEdge {
		// StatementGoto.cpp:138–150 — goto in curr jumps to other_stm
		label := other.SourceLabel
		if label == "" {
			label = LabelForGotoDest(other.StmID, nextLab)
			other.SourceLabel = label
		} else if !StmIDUnset(other.StmID) {
			stmLabels[other.StmID] = label
		}
		// incomplete LocalVars on intermediate blocks fails closed sticky (Collect nil)
		// no invent goto with empty InitSkippedVars when skip list is incomplete
		skipped := CollectInitSkippedVars(blk, okBlk)
		if !VariablesComplete(skipped) {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		st := Stmt{
			Kind: StmtGoto, Expr: cond, Label: label, StmID: AllocStmID(),
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
		MarkNeedRevisitLCA(blk, other)
		// StatementGoto.cpp:149 — Bookkeeper::backward_jump_cnt++
		RecordBackwardJump()
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
		dest.StmID = AllocStmID()
	}
	// StatementGoto.cpp:185 — StatementGoto ctor gensyms label only after DFA
	// validation succeeds. Do not gensym here: failed visit_facts/merge must not
	// burn util.cpp gensym_count (seed-2: extra lbl_710 desynced all later names).

	fm := cg.FM
	foundNewFacts := false
	var gotoIn, gotoOut, stmInMerged, stmOut []*FactPointTo
	if fm != nil {
		// StatementGoto.cpp:159–162 — ctrl uses facts_in, else facts_out
		// C++ map[] always; incomplete maps fail closed (no invent partial goto)
		var srcFacts []*FactPointTo
		if IsCtrlStmt(other) {
			srcFacts = fm.GetMapFactsIn(other.StmID)
		} else {
			srcFacts = fm.GetMapFactsOut(other.StmID)
		}
		if !FactsComplete(srcFacts) {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		gotoIn = CloneFactSlice(srcFacts)
		// StatementGoto.cpp:163 — update_facts_for_dest(goto_in, goto_out, stm)
		UpdateFactsForDest(gotoIn, &gotoOut, fm.Func, blk)
		// StatementGoto.cpp:164–166 — merge effect from goto src (map[] zero if missing live id)
		// Incomplete map_accum_effect fails closed sticky (no invent AddEffect poison then success)
		preEffect := cg.AccumEffect()
		srcAcc := fm.GetMapAccumEffect(other.StmID)
		if !EffectComplete(srcAcc) {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		cg.AddEffect(srcAcc, true)
		if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			cg.ResetEffectAccum(preEffect)
			return makeGotoFailed()
		}
		// StatementGoto.cpp:167–182
		destIn := fm.GetMapFactsIn(dest.StmID)
		if !FactsComplete(destIn) {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		stmInMerged = CloneFactSlice(destIn)
		// tryMerge distinguishes incomplete wipe from complete no-change
		// (no invent treat MergeJumpFacts false as "unchanged" after wipe)
		changed, mok := tryMergeJumpFacts(&stmInMerged, gotoOut)
		if !mok {
			SetError(ErrGeneric)
			return makeGotoFailed()
		}
		if changed {
			stmOut = CloneFactSlice(stmInMerged)
			foundNewFacts = true
			factsInCopy := make(map[int][]*FactPointTo)
			factsOutCopy := make(map[int][]*FactPointTo)
			unionInCopy := make(map[int][]*FactUnion)
			unionOutCopy := make(map[int][]*FactUnion)
			fm.BackupStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
			// feed merged facts as global for visit (stm_visit_facts inputs)
			// Full FactVec: ePointTo + eUnionWrite (FactMgr.cpp backup/restore).
			if !FactsComplete(stmInMerged) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				SetError(ErrGeneric)
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
			liveSaved := CloneFactSlice(fm.GlobalFacts)
			liveSavedU := CloneUnionFactSlice(fm.UnionFacts)
			if !FactsComplete(liveSaved) || !UnionFactsComplete(liveSavedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				SetError(ErrGeneric)
				return makeGotoFailed()
			}
			if !MakeupNewVarFacts(&stmInMerged, liveSaved) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !HasError() {
					SetError(ErrGeneric)
				}
				return makeGotoFailed()
			}
			// eUnionWrite half: map_in union + makeup new subjects from live
			stmInMergedU := fm.GetMapUnionFactsIn(dest.StmID)
			if !UnionFactsComplete(stmInMergedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				SetError(ErrGeneric)
				return makeGotoFailed()
			}
			stmInMergedU = append([]*FactUnion(nil), stmInMergedU...)
			if stmInMergedU == nil {
				stmInMergedU = []*FactUnion{}
			}
			if !makeupNewUnionFacts(&stmInMergedU, liveSavedU) {
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				if !HasError() {
					SetError(ErrGeneric)
				}
				return makeGotoFailed()
			}
			fm.SetGlobalFacts(CloneFactSlice(stmInMerged), "auto_statement_goto_664")
			if !UnionFactsComplete(stmInMergedU) {
				fm.SetGlobalFacts(liveSaved, "auto_statement_goto_restore")
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				SetError(ErrGeneric)
				return makeGotoFailed()
			}
			if stmInMergedU == nil {
				fm.UnionFacts = []*FactUnion{}
			} else {
				fm.UnionFacts = CloneUnionFactSlice(stmInMergedU)
			}
			if !VisitFactsStmt(dest, cg, opts) {
				fm.SetGlobalFacts(liveSaved, "auto_statement_goto_restore")
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				return makeGotoFailed()
			}
			// visit may update GlobalFacts/UnionFacts as outs; incomplete fails closed sticky
			if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
				fm.SetGlobalFacts(liveSaved, "auto_statement_goto_restore")
				fm.UnionFacts = liveSavedU
				fm.RestoreStmFactMaps(dest, factsInCopy, factsOutCopy, unionInCopy, unionOutCopy)
				cg.ResetEffectAccum(preEffect)
				SetError(ErrGeneric)
				return makeGotoFailed()
			}
			stmOut = CloneFactSlice(fm.GlobalFacts)
			// StatementGoto.cpp:178–181 — if dest contains other, recompute goto_out
			if ContainsStmt(dest, other) {
				gotoOut = nil
				UpdateFactsForDest(gotoIn, &gotoOut, fm.Func, blk)
			}
		}
	}

	// StatementGoto.cpp:184–192 — insert goto after other_stm in other_blk
	// incomplete LocalVars on intermediate blocks fails closed sticky (IncompleteVariables)
	// no invent forward goto with empty InitSkippedVars when skip list is incomplete
	skippedFwd := CollectInitSkippedVars(okBlk, blk)
	if !VariablesComplete(skippedFwd) {
		SetError(ErrGeneric)
		return makeGotoFailed()
	}
	// StatementGoto.cpp:185 + ctor 220–229 — gensym only on successful insert path
	label := dest.SourceLabel
	if label == "" {
		label = LabelForGotoDest(dest.StmID, nextLab)
		if label == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return makeGotoFailed()
		}
		dest.SourceLabel = label
	} else if !StmIDUnset(dest.StmID) {
		stmLabels[dest.StmID] = label
	}
	sg := Stmt{
		Kind:            StmtGoto,
		Expr:            cond,
		Label:           label,
		StmID:           AllocStmID(),
		GotoForward:     true,
		GotoDestStmID:   dest.StmID,
		GotoDestParent:  blk,
		InitSkippedVars: skippedFwd,
	}
	// re-resolve other index (slice stable until insert)
	insertAt := -1
	for i := range okBlk.Stmts {
		if okBlk.Stmts[i].StmID == other.StmID {
			insertAt = i
			break
		}
	}
	if insertAt < 0 {
		insertAt = ti
	}
	okBlk.Stmts = append(okBlk.Stmts[:insertAt+1], append([]Stmt{sg}, okBlk.Stmts[insertAt+1:]...)...)
	// pointer to inserted stmt for fact maps
	ins := &okBlk.Stmts[insertAt+1]

	if fm != nil {
		// StatementGoto.cpp:195–202
		fm.SetMapFactsIn(ins.StmID, gotoIn)
		fm.SetMapFactsOut(ins.StmID, gotoOut)
		if fm.MapVisited == nil {
			fm.MapVisited = make(map[int]bool)
		}
		fm.MapVisited[ins.StmID] = true
		if foundNewFacts {
			// StatementGoto.cpp:200–201 — set_fact_in(stm, stm_in); set_fact_out(stm, stm_out)
			fm.SetMapFactsIn(dest.StmID, stmInMerged)
			fm.SetMapFactsOut(dest.StmID, stmOut)
		}
		// StatementGoto.cpp:203 — create_cfg_edge(sg, stm, false, false)
		fm.CreateCFGEdgeTo(ins.StmID, blk, dest.StmID, false, false)
		// StatementGoto.cpp:204–210 — global_facts = map_facts_out[stm]
		// GetMapFacts*: StmID 0 Incomplete; missing live → empty complete
		// Incomplete out/in fails closed sticky (no invent soft re-pick past wiped facts)
		out := fm.GetMapFactsOut(dest.StmID)
		if !FactsComplete(out) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
		} else {
			fm.SetGlobalFacts(CloneFactSlice(out), "auto_statement_goto_742")
		}
		if IsCtrlStmt(dest) || dest.Kind == StmtReturn {
			// ctrl/return: use map_facts_in[stm] (altered outs for OOS)
			in := fm.GetMapFactsIn(dest.StmID)
			if !FactsComplete(in) {
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
			} else {
				fm.SetGlobalFacts(CloneFactSlice(in), "auto_statement_goto_751")
			}
		}
	}
	// StatementGoto.cpp:211
	RecordForwardJump()
	// StatementGoto.cpp:212 — return nullptr (goto already in other_blk)
	return makeGotoFailed()
}
