// Upstream: Block.h / Block.cpp (BlockProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"sort"
	"strings"
)

// Stmt is a minimal statement record (emit only; full Statement subclasses later).
type Stmt struct {
	Kind StatementType
	// Expr is return value / assign RHS / if-test when present.
	Expr *Expression
	// LhsVar is assign target when Kind==StmtAssign.
	LhsVar *Variable
	// Lhs is full Lhs (var + desired type) when available for Output.
	Lhs *Lhs
	// Then/Else for if; Then is for-body for for.
	Then *Block
	Else *Block
	// Loop holds for-loop control (init/test/incr).
	Loop *LoopControl
	// AssignOp for StmtAssign (default simple).
	AssignOp AssignOp
	// ArrayAccess if set, used as LHS text (itemized array).
	ArrayAccess string
	// Label for goto target name (StmtGoto).
	Label string
	// SourceLabel is emitted before this statement (back-edge target).
	SourceLabel string
	// LabelAttr is optional __attribute__((...)) after label (pre_output).
	LabelAttr string
	// GotoForward: after this goto, insert a labeled no-op in the block.
	GotoForward bool
	// GotoBack: label lives on an earlier statement (SourceLabel).
	GotoBack bool
	// InitSkippedVars mirrors StatementGoto::init_skipped_vars.
	// StatementGoto.cpp:223 — locals whose inits are skipped by this jump.
	InitSkippedVars []*Variable
	// GotoDestStmID is the destination statement stm_id for goto (StatementGoto::dest).
	GotoDestStmID int
	// GotoDestParent is dest statement's parent block (StatementGoto::dest->parent).
	// Used by FactMgr::add_fact_out visibility (FactMgr.cpp:296–300).
	GotoDestParent *Block
	// StmID mirrors Statement::stm_id for step_hash.
	StmID int
	// SafeFlags / Tmp1 / Tmp2 for compound assign safe-math OutputAsExpr.
	// StatementAssign.cpp make_possible_compound_assign.
	SafeFlags *SafeOpFlags
	Tmp1      string
	Tmp2      string
}

// nextStmID is Statement::sid allocator.
var nextStmID int

// AllocStmID mirrors Statement constructor stm_id = ++sid.
func AllocStmID() int {
	nextStmID++
	return nextStmID
}

// Block mirrors Block : Statement with local_vars and stms.
type Block struct {
	Parent    *Block
	Func      *Function
	LocalVars []*Variable
	Stmts     []Stmt
	Looping   bool
	blockSize int // CGOptions::max_block_size at creation
	// TmpVars mirrors macro_tmp_vars (gensym t_ for safe math).
	TmpVars map[string]ESimpleType
	// EmitDepthProtect: emit DEPTH++/-- when CGOptions::depth_protect (Block.cpp:255–267).
	EmitDepthProtect bool
	// EmitStepHash: emit step_hash(stm_id) before each stmt (CGOptions::step_hash_by_stmt).
	EmitStepHash bool
	// BreakStmIDs mirrors Block::break_stms (stm_id list).
	BreakStmIDs []int
	// InArrayLoop mirrors Block::in_array_loop — disallow goto in/out.
	InArrayLoop bool
	// NeedRevisit mirrors Block::need_revisit — force full re-analysis.
	// Block.cpp:195.
	NeedRevisit bool
	// StmID mirrors Statement::stm_id for the block itself (compound stmt).
	StmID int
	// EmitLabelAttrs: emit __attribute__ on goto labels (CGOptions::label_attributes).
	EmitLabelAttrs bool
	// LabelAttrRng seed for attributes when EmitLabelAttrs (optional; use package gen).
	LabelAttrRng *Rng
	// EmitParanoid / EmitConcise / EmitFM: Statement::post_output assertions.
	// Block.cpp Output + Statement.cpp:919–924 when CGOptions::paranoid.
	EmitParanoid bool
	EmitConcise  bool
	EmitFM       *FactMgr
}

// GetLastStm mirrors Block::get_last_stm — last effective statement.
// Block.cpp:336–346 — last stmt, but stop early if return encountered.
func (b *Block) GetLastStm() *Stmt {
	if b == nil || len(b.Stmts) == 0 {
		return nil
	}
	var last *Stmt
	for i := range b.Stmts {
		last = &b.Stmts[i]
		if last.Kind == StmtReturn {
			break
		}
	}
	return last
}

// FromTailToHead mirrors Block::from_tail_to_head.
// Block.cpp:362–372 — looping body may fall through to head if last does not must_jump.
func (b *Block) FromTailToHead() bool {
	if b == nil || !b.Looping || len(b.Stmts) == 0 {
		return false
	}
	s := b.GetLastStm()
	if s == nil {
		return false
	}
	if s.MustJump() {
		return false
	}
	return true
}

// SetAccumulatedEffect mirrors Block::set_accumulated_effect.
// Block.cpp:571–580 — union of map_stm_effect for each statement.
func (b *Block) SetAccumulatedEffect(fm *FactMgr) Effect {
	eff := EmptyEffect()
	if b == nil || fm == nil {
		return eff
	}
	for i := range b.Stmts {
		st := &b.Stmts[i]
		if st.StmID > 0 {
			eff = eff.AddEffect(fm.GetMapStmEffect(st.StmID))
		}
	}
	if b.StmID > 0 {
		fm.SetMapStmEffect(b.StmID, eff)
	}
	return eff
}

// RandomParentBlock mirrors Block::random_parent_block.
// Block.cpp:353–370 — self and ancestors; optional nil global if GlobalVariables.
func (b *Block) RandomParentBlock(r *Rng, allowGlobal bool) *Block {
	// Block.cpp:295–307 — rnd_upto(blks); ERROR_GUARD(nullptr); no soft invent self
	if r == nil || b == nil {
		return nil
	}
	var blks []*Block
	if allowGlobal {
		blks = append(blks, nil)
	}
	for cur := b; cur != nil; cur = cur.Parent {
		blks = append(blks, cur)
	}
	if len(blks) == 0 {
		return nil
	}
	return blks[r.RndUpto(uint32(len(blks)))]
}

// MustBreakOrReturn mirrors Block::must_break_or_return without FactMgr.
// Block.cpp:342–357 — last must_return (not must_jump) unless escape back-edge.
// Prefer MustBreakOrReturnFull(fm) when CFG is available.
func (b *Block) MustBreakOrReturn() bool {
	return b.MustBreakOrReturnFull(b.EmitFM)
}

// IsVarOnStack mirrors Block::is_var_on_stack.
// Block.cpp:443–456 — params + local_vars chain.
func (b *Block) IsVarOnStack(v *Variable) bool {
	if b == nil || v == nil {
		return false
	}
	f := b.Func
	for bb := b; f == nil && bb != nil; bb = bb.Parent {
		f = bb.Func
	}
	if f != nil {
		for _, p := range f.Param {
			if p != nil && p.Match(v) {
				return true
			}
		}
	}
	for bb := b; bb != nil; bb = bb.Parent {
		for _, loc := range bb.LocalVars {
			if loc == v || (loc != nil && loc.Match(v)) {
				return true
			}
		}
	}
	return false
}

// CreateNewTmpVar mirrors Block::create_new_tmp_var.
// Block.cpp:216–219 — always gensym("t_") (util.cpp global counter); no
// len(TmpVars) soft names when GenSym is nil.
func (b *Block) CreateNewTmpVar(sym *GenSym, st ESimpleType) string {
	// Block.cpp:217 — const string var_name = gensym("t_");
	name := ""
	if sym != nil {
		name = sym.Next("t_")
	} else {
		name = Gensym("t_")
	}
	if b == nil {
		return name
	}
	if b.TmpVars == nil {
		b.TmpVars = make(map[string]ESimpleType)
	}
	b.TmpVars[name] = st
	return name
}

// BlockProbability mirrors Block.cpp BlockProbability.

// Keep-filter on {block_size-1} forces return value max_block_size-1.
// Block.cpp:88–94.
func BlockProbability(blockSize int, r *Rng) int {
	if blockSize < 1 {
		return 0
	}
	// rnd_upto(block_size) with Keep filter only accepting block_size-1
	// → always block_size-1 after retries.
	_ = r
	return blockSize - 1
}

// MakeRandomBlock mirrors Block::make_random.
// Block.cpp:115–226 — statements, optional nested loop, post_creation_analysis.
// cg is *CGContext (C++ CGContext&) so stmt effect_stm/expr_depth and
// post_creation_analysis mutate the caller's context.
func MakeRandomBlock(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
	looping bool,
) *Block {
	if r == nil || cg == nil {
		return nil
	}
	f := cg.CurrentFunc
	parent := (*Block)(nil)
	if f != nil && len(f.Stack) > 0 {
		parent = f.Stack[len(f.Stack)-1]
	}
	b := &Block{
		Parent:           parent,
		Func:             f,
		Looping:          looping,
		blockSize:        opts.MaxBlockSize,
		EmitDepthProtect: opts.DepthProtect,
		EmitStepHash:     opts.StepHashByStmt,
		EmitLabelAttrs:   opts.LabelAttributes,
		LabelAttrRng:     r,
		StmID:            AllocStmID(),
		// Block.cpp:127 — in_array_loop when induction bounds non-empty
		InArrayLoop:  len(cg.IVBounds) > 0,
		EmitParanoid: opts.Paranoid,
		EmitConcise:  opts.Concise,
		EmitFM:       cg.FM,
	}
	if f != nil {
		f.Stack = append(f.Stack, b)
		f.Blocks = append(f.Blocks, b)
	}
	// DepthSpec::depth_guard_by_type(dtBlock) — random mode always GOOD
	if DepthGuardByType(opts, "dtBlock") == BadDepth {
		abortBlockMake(f, b)
		return nil
	}
	max := BlockProbability(b.blockSize, r)
	// Block.cpp:136–140 — ERROR after BlockProbability → delete block
	if HasError() {
		abortBlockMake(f, b)
		return nil
	}
	// Note: blk_depth is bumped in Statement::make_random for compound stmts
	// (Statement.cpp:267–269), not when entering Block::make_random.
	// Running effect accum for this block (side-effect / no_volatile for SelectLType)
	if cg.EffectAccum == nil {
		eff := EmptyEffect()
		cg.EffectAccum = &eff
	}
	// Block.cpp:134–138 — snapshot facts-in and pre_effect for post_creation
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		preEffect = cg.EffectAccum.Clone()
	}
	if cg.FM != nil && b.StmID > 0 {
		cg.FM.SetMapFactsIn(b.StmID, cg.FM.GlobalFacts)
	}
	// Forward goto: prefer labeling the next real statement; no-op if goto is last.
	pendingFwd := ""
	for i := 0; i <= max; i++ {
		st := makeRandomStmt(r, opts, probs, vs, tables, stmtTab, cg, b)
		// Block.cpp:142–146 — null Statement* (exhaustive / failed factories) → break
		if !stmtOK(st) {
			break
		}
		if st.StmID == 0 {
			st.StmID = AllocStmID()
		}
		if pendingFwd != "" {
			if st.SourceLabel == "" {
				st.SourceLabel = pendingFwd
			} else {
				// already labeled — keep pending as no-op marker after previous
				lab := Stmt{Kind: StmtLabel, SourceLabel: pendingFwd, StmID: AllocStmID()}
				b.Stmts = append(b.Stmts, lab)
			}
			pendingFwd = ""
		}
		b.Stmts = append(b.Stmts, st)
		if st.Kind == StmtGoto && st.GotoForward && st.Label != "" {
			pendingFwd = st.Label
		}
		// Block.cpp:152 — stop when statement must_return
		if st.MustReturn() {
			break
		}
	}
	if pendingFwd != "" {
		b.Stmts = append(b.Stmts, Stmt{Kind: StmtLabel, SourceLabel: pendingFwd, StmID: AllocStmID()})
	}
	// Block.cpp:157–161 — ERROR after stmt loop → delete block
	if HasError() {
		abortBlockMake(f, b)
		return nil
	}
	// Block.cpp:164–166 — nested loop for must-use multi-dim arrays
	if b.NeedNestedLoop(*cg, r) && cg.BlkDepth < opts.MaxBlockDepth {
		b.AppendNestedLoop(r, opts, probs, vs, tables, stmtTab, cg)
	}
	// Block::post_creation_analysis (Block.cpp:682–742)
	// Upstream appends return only inside post_creation when still missing.
	// Without FactMgr, append return here so function bodies stay valid C.
	if cg.FM == nil && parent == nil && f != nil && f.NeedReturnStmt() && !b.MustReturn() {
		ret := MakeRandomReturn(r, opts, vs, cg)
		if stmtOK(ret) {
			if ret.StmID == 0 {
				ret.StmID = AllocStmID()
			}
			b.Stmts = append(b.Stmts, ret)
		}
	}
	if b.StmID == 0 {
		b.StmID = AllocStmID()
	}
	b.PostCreationAnalysis(cg, opts, preEffect, r, vs)
	if HasError() {
		// Block.cpp:170–174 — ERROR after post_creation → delete
		abortBlockMake(f, b)
		return nil
	}
	if f != nil && len(f.Stack) > 0 {
		f.Stack = f.Stack[:len(f.Stack)-1]
	}
	// Block.cpp:187 — Error::set_error(SUCCESS)
	ClearError()
	return b
}

// abortBlockMake pops stack and unregisters a failed Block::make_random (C++ delete b).
func abortBlockMake(f *Function, b *Block) {
	if f == nil || b == nil {
		return
	}
	if n := len(f.Stack); n > 0 && f.Stack[n-1] == b {
		f.Stack = f.Stack[:n-1]
	}
	for i, x := range f.Blocks {
		if x == b {
			f.Blocks = append(f.Blocks[:i], f.Blocks[i+1:]...)
			break
		}
	}
}

// PostCreationAnalysis mirrors Block::post_creation_analysis.
// Block.cpp:682–742 — effects, OOS, optional fixed-point with remove_stmt, append_return.
func (b *Block) PostCreationAnalysis(cg *CGContext, opts Options, preEffect Effect, r *Rng, vs *VariableSelector) {
	if b == nil || cg == nil {
		return
	}
	fm := cg.FM
	if fm == nil {
		return
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	fm.MapVisited[b.StmID] = true
	b.SetAccumulatedEffect(fm)
	postFacts := CloneFactSlice(fm.GlobalFacts)
	if len(b.LocalVars) > 0 {
		fm.UpdateFactsForOOSVars(b.LocalVars)
	}
	fm.RemoveRVFacts(&fm.GlobalFacts)
	fm.SetMapFactsOut(b.StmID, fm.GlobalFacts)

	// Block.cpp:696–732 — fixed-point when loop body / revisit / back edges
	mustBR := b.MustBreakOrReturnFull(fm)
	isLoopBody := !mustBR && b.Looping
	hasBack := fm.HasEdgeIn(b.StmID, false, true) ||
		len(fm.FindEdgesInToBlock(b, false, true)) > 0 ||
		b.ContainsBackEdge(fm)
	if isLoopBody || b.NeedRevisit || hasBack {
		selfBack := false
		if isLoopBody && b.FromTailToHead() {
			selfBack = true
			fm.CreateCFGEdge(b.StmID, b, false, true)
		}
		factsCopy := CloneFactSlice(fm.MapFactsIn[b.StmID])
		// reset accum to pre-block effect
		if cg.EffectAccum != nil {
			*cg.EffectAccum = preEffect.Clone()
		}
		for {
			out, failIdx, ok := FindFixedPointBlock(b, factsCopy, cg, opts, b.NeedRevisit)
			if ok {
				postFacts = out
				break
			}
			// remove from fail index through end (Block.cpp:709–714)
			if failIdx < 0 {
				failIdx = 0
			}
			for failIdx < len(b.Stmts) {
				id := b.Stmts[failIdx].StmID
				if id == 0 {
					b.Stmts = append(b.Stmts[:failIdx], b.Stmts[failIdx+1:]...)
					continue
				}
				if n := b.RemoveStmt(id, fm); n == 0 {
					b.Stmts = append(b.Stmts[:failIdx], b.Stmts[failIdx+1:]...)
				}
			}
			b.NeedRevisit = true
			fm.ResetBlockFactMaps(b)
			if !selfBack && b.FromTailToHead() {
				selfBack = true
				fm.CreateCFGEdge(b.StmID, b, false, true)
			}
			if cg.EffectAccum != nil {
				*cg.EffectAccum = preEffect.Clone()
			}
			if len(b.Stmts) == 0 {
				break
			}
		}
		if out, ok := fm.MapFactsOut[b.StmID]; ok {
			fm.GlobalFacts = CloneFactSlice(out)
		}
	} else if b.Looping && b.FromTailToHead() {
		fm.CreateCFGEdge(b.StmID, b, false, true)
	}
	// Block.cpp:734–741 — append return for top-level body when still missing
	if b.Parent == nil && b.Func != nil && b.Func.NeedReturnStmt() && !b.MustReturn() {
		fm.GlobalFacts = postFacts
		b.AppendReturnStmt(r, opts, vs, cg)
	}
}

// makeRandomStmt picks a statement kind and fills a Stmt.
// Statement::make_random — filter + dispatch; retry on failed factory (Statement.cpp:314–316).
func makeRandomStmt(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
	b *Block,
) Stmt {
	if cg == nil {
		return Stmt{}
	}
	// Statement.cpp:243–244 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtStatement, t, nullptr)
	// t is MAX_STATEMENT_TYPE when choosing randomly (flag = MaxStatementType).
	if DepthGuardByTypeFlag(opts, DtStatement, int(MaxStatementType)) == BadDepth {
		return Stmt{}
	}
	if stmtTab == nil {
		stmtTab = NewStatementThresholdTable(opts)
	}
	// StatementFilter (Statement.cpp:150–182)
	f := filterFunc(func(v uint32) bool {
		k := NumberToType(stmtTab, v)
		// Statement.cpp:158–160 — PartialExpander::expand_check
		if !ExpandCheck(k) {
			return true
		}
		// Statement.cpp:164–166 — eBlock always filtered
		if k == StmtBlock {
			return true
		}
		// Statement.cpp:167–169 — void functions cannot return
		if k == StmtReturn && cg.CurrentFunc != nil && cg.CurrentFunc.ReturnType != nil &&
			cg.CurrentFunc.ReturnType.IsSimple() && cg.CurrentFunc.ReturnType.Simple() == EVoid {
			return true
		}
		// Statement.cpp:171–173 — break/continue only in loops
		if (k == StmtBreak || k == StmtContinue) && !cg.InLoop() {
			return true
		}
		// Statement.cpp:176–178 — max nesting: filter compounds
		if cg.BlkDepth >= opts.MaxBlockDepth {
			return IsCompound(k)
		}
		// Statement.cpp:179–183 — at max funcs: filter only Invoke (allow others)
		if ReachMaxFunctions(cg.Funcs, opts) {
			return k == StmtInvoke
		}
		return false
	})
	// retry failed factories (null Statement* upstream) — Statement.cpp:314–316
	// C++: if s==0 without error, make_random re-picks forever; cap high (no soft invent empty early)
	for tries := 0; tries < 256; tries++ {
		// Statement.cpp:261–265 — clear effect_stm; expr_depth = 0
		cg.EffectStm = EmptyEffect()
		cg.ExprDepth = 0
		kind := StatementProbabilityFilter(r, stmtTab, f)
		// Statement.cpp:248–250 — stop_by_stmt forces return after sid threshold
		if opts.StopByStmt >= 0 && nextStmID >= opts.StopByStmt {
			kind = StmtReturn
		}
		// Statement.cpp:260–261 — pre_facts / pre_effect (accum) snapshot before make
		var preFacts []*FactPointTo
		if cg.FM != nil {
			preFacts = CloneFactSlice(cg.FM.GlobalFacts)
		}
		preEffect := EmptyEffect()
		if cg.EffectAccum != nil {
			preEffect = cg.EffectAccum.Clone()
		}
		// Statement.cpp:267–269 / 306–308 — compound stmts bump blk_depth around factory
		if IsCompound(kind) {
			cg.BlkDepth++
		}
		st := makeRandomStmtKind(r, opts, probs, vs, tables, stmtTab, cg, b, kind)
		if IsCompound(kind) {
			cg.BlkDepth--
		}
		// Statement.cpp:309 — ERROR_GUARD(nullptr): sticky error aborts without re-pick
		if HasError() {
			return Stmt{}
		}
		if stmtOK(st) {
			// Statement.cpp:320 — post_creation_analysis(pre_facts, pre_effect)
			PostCreationAnalysis(&st, preFacts, preEffect, cg, opts)
			return st
		}
		// s == 0 without error — re-pick type (Statement.cpp:314–316)
	}
	// bounded library limit (C++ recurses forever); empty stmt is not appended usable
	return Stmt{}
}

func makeRandomStmtKind(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
	b *Block,
	kind StatementType,
) Stmt {
	if cg == nil {
		return Stmt{Kind: kind}
	}
	switch kind {
	case StmtReturn:
		return MakeRandomReturn(r, opts, vs, cg)
	case StmtAssign:
		st := MakeRandomAssign(r, opts, probs, vs, tables, cg, nil)
		// Effect::write_var on LHS (CGContext effect_accum)
		if st.LhsVar != nil {
			cg.NoteWrite(st.LhsVar)
		}
		return st
	case StmtBreak:
		return MakeRandomBreak(r, opts, vs, tables, cg)
	case StmtContinue:
		return MakeRandomContinue(r, opts, vs, tables, cg, b)
	case StmtIfElse:
		if st := MakeRandomIf(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			return *st
		}
		return Stmt{Kind: StmtIfElse}
	case StmtFor:
		if st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			return *st
		}
		return Stmt{Kind: StmtFor}
	case StmtArrayOp:
		return MakeRandomArrayOp(r, opts, probs, vs, tables, stmtTab, cg)
	case StmtGoto:
		return MakeRandomGoto(r, opts, probs, vs, tables, cg, b)
	case StmtInvoke:
		return MakeRandomExprStmt(r, opts, probs, vs, tables, cg)
	default:
		return Stmt{Kind: kind}
	}
}

// stmtOK reports whether a generated statement is usable (non-null factory).
func stmtOK(st Stmt) bool {
	switch st.Kind {
	case StmtAssign:
		return st.LhsVar != nil || st.ArrayAccess != ""
	case StmtInvoke:
		return st.Expr != nil && st.Expr.Invoke != nil && !st.Expr.Invoke.Failed
	case StmtFor, StmtArrayOp:
		return st.Loop != nil && st.Loop.IV != nil
	case StmtGoto:
		return st.Label != ""
	case StmtReturn:
		return st.Expr != nil
	case StmtIfElse:
		return st.Then != nil
	case StmtContinue, StmtBreak:
		// factories always set test expr; Expr-less marks nullptr reject (e.g. continue first-stmt)
		return st.Expr != nil
	case StmtLabel:
		return st.SourceLabel != ""
	default:
		// zero-value / unknown kind from failed make_random (Statement.cpp:314 null)
		return false
	}
}

// Output emits C for the block with indent levels.
func (b *Block) Output(indent int) string {
	if b == nil {
		return "{\n}\n"
	}
	pad := strings.Repeat("    ", indent)
	inner := strings.Repeat("    ", indent+1)
	var sb strings.Builder
	// Block.cpp:250–253 — "{ " + /* block id: stm_id */
	sb.WriteString(pad + "{ ")
	if b.EmitConcise {
		sb.WriteString("\n")
	} else {
		// OutputMgr::output_comment_line — skip when quiet/concise (EmitConcise)
		sb.WriteString(OutputCommentLine("block id: "+Int2Str(b.StmID), false, false))
	}
	// Block.cpp:255–257
	if b.EmitDepthProtect {
		sb.WriteString(inner + "DEPTH++;\n")
	}
	// Block::OutputTmpVariableList — sorted names for deterministic emit
	if len(b.TmpVars) > 0 {
		names := make([]string, 0, len(b.TmpVars))
		for name := range b.TmpVars {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			st := b.TmpVars[name]
			sb.WriteString(inner)
			sb.WriteString(GetSimpleType(st).CName() + " " + name + " = 0;\n")
		}
	}
	// OutputVariableList(local_vars) — Variable.cpp Output
	var loopInits []*ArrayVariable
	maxDim := 0
	for _, lv := range b.LocalVars {
		if lv == nil || lv.Type == nil {
			continue
		}
		sb.WriteString(inner)
		if lv.IsArray {
			av := lv.AsArray
			if av == nil && len(lv.ArraySizes) > 0 {
				av = &ArrayVariable{
					Variable:   *lv,
					Sizes:      lv.ArraySizes,
					InitValues: lv.ArrayInits,
				}
			}
			if av != nil {
				sb.WriteString(av.OutputDef())
				sb.WriteString("\n")
				if !av.NoLoopInitializer() {
					loopInits = append(loopInits, av)
					if len(av.Sizes) > maxDim {
						maxDim = len(av.Sizes)
					}
				}
				continue
			}
		}
		// Variable::Output for locals (no force static)
		sb.WriteString(lv.OutputDef(false))
		sb.WriteString("\n")
	}
	// OutputArrayInitializers for locals without brace init
	// Variable.cpp:829–841 — new_ctrl_vars + OutputArrayCtrlVars
	if len(loopInits) > 0 {
		// CGOptions::fresh_array_ctrl_var_names / max dimensions via process opts
		opts := ProcessOptions()
		ctrlVars := NewCtrlVars(maxDim, opts.FreshArrayCtrlVarNames)
		sb.WriteString(OutputArrayCtrlVars(ctrlVars, maxDim, inner))
		ctrl := CtrlVarNames(ctrlVars)
		for _, av := range loopInits {
			sb.WriteString(av.OutputInit(inner, ctrl))
		}
	}
	for _, st := range b.Stmts {
		// Statement::pre_output — label from jump sources / SourceLabel, else step_hash
		// Statement.cpp:905–917 — goto target skips output_hash
		pre, isGotoTarget := PreOutput(&st, b.EmitFM, b.EmitStepHash, b.EmitLabelAttrs, b.LabelAttrRng, inner)
		if pre != "" {
			sb.WriteString(pre)
		}
		// Statement.cpp:911–913 — output_skipped_var_inits after label is commented out upstream
		_ = isGotoTarget
		if st.Kind == StmtLabel {
			sb.WriteString(inner + "    ;\n")
			continue
		}
		sb.WriteString(inner)
		switch st.Kind {
		case StmtReturn:
			// StatementReturn.cpp:125–134 — DEPTH-- before return when depth_protect
			if b.EmitDepthProtect {
				sb.WriteString("DEPTH--;\n")
				sb.WriteString(inner)
			}
			sb.WriteString("return")
			if st.Expr != nil {
				sb.WriteString(" " + st.Expr.Output())
			}
			sb.WriteString(";\n")
		case StmtAssign:
			// StatementArrayOp init body: aggregate constant needs tmp
			// StatementArrayOp.cpp:237–248
			if st.ArrayAccess != "" && st.Expr != nil &&
				st.Expr.Term == TermConstant && st.LhsVar != nil &&
				st.LhsVar.Type != nil && st.LhsVar.Type.IsAggregate() {
				ty := st.LhsVar.Type.CName()
				sb.WriteString(ty + " tmp = " + st.Expr.Output() + ";\n")
				sb.WriteString(inner + st.ArrayAccess + " = tmp;\n")
				break
			}
			// StatementAssign::OutputAsExpr — CGOptions::identify_wrappers process-wide
			wrap := st.LhsVar != nil && st.LhsVar.UseVolRVal
			// no soft invent Defaults() / force IdentifyWrappers=false
			asExpr := OutputAssignAsExprOpts(&st, wrap, ProcessOptions())
			if asExpr != "" {
				sb.WriteString(asExpr + ";\n")
			} else if st.ArrayAccess != "" && st.Expr != nil {
				// array_init simple: a[i] = expr
				sb.WriteString(st.ArrayAccess + " = " + st.Expr.Output() + ";\n")
			}
			// incomplete assign IR — no soft invent /* assign */
		case StmtBreak:
			// StatementBreak.cpp:117–118 — test.Output; no soft invent "1"
			sb.WriteString("if (")
			if st.Expr != nil {
				sb.WriteString(st.Expr.Output())
			}
			sb.WriteString(")\n")
			sb.WriteString(inner + "    break;\n")
		case StmtContinue:
			// StatementContinue.cpp — if (test) continue; no soft invent "1"
			sb.WriteString("if (")
			if st.Expr != nil {
				sb.WriteString(st.Expr.Output())
			}
			sb.WriteString(")\n")
			sb.WriteString(inner + "    continue;\n")
		case StmtFor:
			if st.Loop != nil && st.Loop.IV != nil {
				// StatementFor::Output — header from make_iteration IR
				sb.WriteString(forHeaderOutput(st.Loop) + "\n")
				if st.Then != nil {
					sb.WriteString(st.Then.Output(indent + 1))
				} else {
					sb.WriteString(inner + "{\n" + inner + "}\n")
				}
			}
			// incomplete for IR — no soft invent /* for-stub */
		case StmtIfElse:
			// StatementIf.cpp:147–148 — test.Output; no soft invent "0"
			sb.WriteString("if (")
			if st.Expr != nil {
				sb.WriteString(st.Expr.Output())
			}
			sb.WriteString(")\n")
			if st.Then != nil {
				sb.WriteString(st.Then.Output(indent + 1))
			} else {
				sb.WriteString(inner + "{\n" + inner + "}\n")
			}
			sb.WriteString(inner + "else\n")
			if st.Else != nil {
				sb.WriteString(st.Else.Output(indent + 1))
			} else {
				sb.WriteString(inner + "{\n" + inner + "}\n")
			}
		case StmtGoto:
			// StatementGoto.cpp:252–253 — test.Output; no soft invent "0"
			if st.Label != "" {
				sb.WriteString("if (")
				if st.Expr != nil {
					sb.WriteString(st.Expr.Output())
				}
				sb.WriteString(")\n")
				sb.WriteString(inner + "    goto " + st.Label + ";\n")
			}
			// incomplete goto IR — no soft invent /* goto-stub */
		case StmtArrayOp:
			// Emit as for-loop over array write (MakeRandomArrayOp filled Loop+Then).
			if st.Loop != nil && st.Loop.IV != nil {
				sb.WriteString(forHeaderOutput(st.Loop) + "\n")
				if st.Then != nil {
					sb.WriteString(st.Then.Output(indent + 1))
				} else {
					sb.WriteString(inner + "{\n" + inner + "}\n")
				}
			}
			// incomplete arrayop IR — no soft invent /* arrayop-stub */
		case StmtInvoke:
			// StatementExpr::Output — expr.Output(); ";"
			// no soft invent /* invoke */ when expr missing (stmtOK rejects null)
			if st.Expr != nil {
				sb.WriteString(st.Expr.Output() + ";\n")
			}
		default:
			// incomplete IR — no soft invent comment stub
		}
		// Statement::post_output — paranoid fact assertions (Statement.cpp:919–924)
		if b.EmitParanoid && b.EmitFM != nil {
			sb.WriteString(PostOutput(&st, b, b.EmitFM, true, b.EmitConcise, inner))
		}
	}
	// Block.cpp:266–267
	if b.EmitDepthProtect {
		sb.WriteString(inner + "DEPTH--;\n")
	}
	sb.WriteString(pad + "}\n")
	return sb.String()
}
