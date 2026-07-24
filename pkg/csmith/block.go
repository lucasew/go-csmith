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
	// Rhs mirrors StatementAssign::rhs — canonized compound form
	// (ExpressionFuncall for "i += e" → i + e). FactMgr::update_fact_for_assign
	// uses get_rhs(); OutputAsExpr still uses expr (get_expr).
	// StatementAssign.h:149–151.
	Rhs *Expression
}

// Statement::sid lives on Session.NextStmID (see session.go).

// IncompleteStmID is the unset/incomplete sentinel (not a C++ stm_id).
// Valid C++ ids are 0,1,2,… — never use 0 as “missing”.
const IncompleteStmID = -1

// AllocStmID mirrors Statement ctor: stm_id = sid; sid++.
// Statement.cpp:370–371 — first statement gets 0.
func AllocStmID() int {
	s := currentSession()
	id := s.NextStmID
	s.NextStmID++
	return id
}

// StmIDUnset reports a never-allocated id (zero-value after IncompleteStmID convention,
// or legacy incomplete shells using IncompleteStmID).
// Valid id 0 must pass (C++ first Block).
func StmIDUnset(id int) bool { return id < 0 }

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

// BlockSize mirrors Block::block_size.
// Block.h:85 — CGOptions::max_block_size captured at construction.
func (b *Block) BlockSize() int {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	return b.blockSize
}

// GetDepthProtect mirrors Block::get_depth_protect.
// Block.h:76.
func (b *Block) GetDepthProtect() bool {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return b.EmitDepthProtect
}

// SetDepthProtect mirrors Block::set_depth_protect — returns new value.
// Block.h:72–74.
func (b *Block) SetDepthProtect(v bool) bool {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	b.EmitDepthProtect = v
	return v
}

// PushStmt mirrors stms.push_back for a complete Statement.
// Incomplete Stmt Kind sticky (no invent append hole).
func (b *Block) PushStmt(st Stmt) {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	b.Stmts = append(b.Stmts, st)
}

// FindBlockByID mirrors find_block_by_id.
// Block.cpp:69–83 — scan non-builtin Function::blocks for stm_id.
// Incomplete funcs sticky nil.
func FindBlockByID(funcs []*Function, blkID int) *Block {
	if !FunctionsComplete(funcs) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// incomplete id sticky (no invent match-first soft-pick)
	if blkID <= 0 {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		for _, b := range f.Blocks {
			if b == nil {
				sessNoteError(nil, ErrGeneric)
				return nil
			}
			if b.StmID == blkID {
				return b
			}
		}
	}
	return nil
}

// OutputStatementList mirrors static OutputStatementList.
// Block.cpp:235–241 — pre_output + Output + post_output per statement.
// Implemented via a temporary Block carrying the same emit flags as the parent
// path in Block.Output (statement switch lives there).
// Incomplete list sticky "" (no invent partial emit past hole).
func OutputStatementList(stms []Stmt, parent *Block, indent int) string {
	// empty list soft empty section
	if len(stms) == 0 {
		return ""
	}
	// Build a transient block with parent's emit flags and only these statements.
	// Strip braces by reusing Block.outputStmtsOnly.
	tmp := &Block{Stmts: stms}
	if parent != nil {
		tmp.EmitFM = parent.EmitFM
		tmp.EmitStepHash = parent.EmitStepHash
		tmp.EmitLabelAttrs = parent.EmitLabelAttrs
		tmp.LabelAttrRng = parent.LabelAttrRng
		tmp.EmitParanoid = parent.EmitParanoid
		tmp.EmitConcise = parent.EmitConcise
		tmp.EmitDepthProtect = parent.EmitDepthProtect
	}
	return tmp.outputStmtsOnly(indent)
}

// GetLastStm mirrors Block::get_last_stm — last effective statement.
// Block.cpp:336–346 — last stmt, but stop early if return encountered.
// Incomplete Block sticky nil (no invent soft-skip empty last / soft re-pick past hole).
func (b *Block) GetLastStm() *Stmt {
	// Block always live; sticky incomplete no invent nil last soft-skip
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if len(b.Stmts) == 0 {
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
// Incomplete Block/last sticky false (no invent fall-through / soft re-pick past holes).
func (b *Block) FromTailToHead() bool {
	// Block always live; sticky incomplete no invent fall-through soft-skip
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !b.Looping || len(b.Stmts) == 0 {
		return false
	}
	s := b.GetLastStm()
	if s == nil {
		// incomplete last stmt sticky no fall-through
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// residual ERROR sticky — no invent soft-continue fall-through past GetLastStm residual
	if sessHasError(nil) {
		return false
	}
	if s.MustJump() {
		// residual ERROR sticky — no invent no-fall-through true past MustJump residual hole
		if sessHasError(nil) {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent fall-through true past MustJump residual false path
	if sessHasError(nil) {
		return false
	}
	return true
}

// SetAccumulatedEffect mirrors Block::set_accumulated_effect.
// Block.cpp:571–580 — union of map_stm_effect for each statement.
// Statement::stm_id always live after create; StmID 0 is incomplete IR.
// Incomplete stmts / effects fail closed sticky IncompleteEffect (not EmptyEffect —
// IsEmpty/pure invent empty-complete block accum past StmID 0 soft-skip).
func (b *Block) SetAccumulatedEffect(fm *FactMgr) Effect {
	if b == nil || fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	// Block::stm_id always live; StmID 0 fails closed sticky incomplete (no invent
	// empty-complete accum return without map_stm_effect[block] recorded)
	if StmIDUnset(b.StmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	eff := EmptyEffect()
	for i := range b.Stmts {
		st := &b.Stmts[i]
		if StmIDUnset(st.StmID) {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			sessNoteError(fmSess(fm), ErrGeneric)
			return inc
		}
		// map_stm_effect[] defaults empty Effect in C++; incomplete map keys fail closed sticky
		se := fm.GetMapStmEffect(st.StmID)
		if !EffectComplete(se) {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			sessNoteError(fmSess(fm), ErrGeneric)
			return inc
		}
		eff = eff.AddEffect(se)
		if !EffectComplete(eff) {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return inc
		}
	}
	fm.SetMapStmEffect(b.StmID, eff)
	return eff
}

// RandomParentBlock mirrors Block::random_parent_block.
// Block.cpp:295–308 — optional nil (global) first when allowGlobal; then self+ancestors;
// rnd_upto(blks.size()). C++ uses CGOptions::global_variables() for the nil slot
// (StatementArrayOp::make_random_array_init always hits this with defaults).
func (b *Block) RandomParentBlock(r *Rng, allowGlobal bool) *Block {
	// Block.cpp:295–308 — rnd_upto(blks); ERROR_GUARD(nullptr); no soft invent self
	// sticky only on nil RNG (live this); nil receiver is broken call non-sticky
	if b == nil {
		return nil
	}
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	var blks []*Block
	if allowGlobal {
		// Block.cpp:297–299 — blks.push_back(nullptr) when global_variables()
		blks = append(blks, nil)
	}
	for cur := b; cur != nil; cur = cur.Parent {
		blks = append(blks, cur)
	}
	if len(blks) == 0 {
		return nil
	}
	idx := r.RndUpto(uint32(len(blks)))
	// Block.cpp:306 ERROR_GUARD
	if sessHasError(nil) {
		return nil
	}
	return blks[idx]
}

// MustBreakOrReturn mirrors Block::must_break_or_return without FactMgr.
// Block.cpp:342–357 — last must_return (not must_jump) unless escape back-edge.
// Prefer MustBreakOrReturnFull(fm) when CFG is available.
func (b *Block) MustBreakOrReturn() bool {
	return b.MustBreakOrReturnFull(b.EmitFM)
}

// StackScanComplete reports Param + LocalVars parent-chain have no nil holes.
// Incomplete lists must not invent not-on-stack membership for selection/mark paths.
// Block always live at stack scan; nil shell sticky false (no invent incomplete-scan
// soft-miss without ERROR so soft re-pick cannot treat hole as clean incomplete).
func (b *Block) StackScanComplete() bool {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	f := b.Func
	for bb := b; f == nil && bb != nil; bb = bb.Parent {
		f = bb.Func
	}
	if f != nil {
		for _, p := range f.Param {
			if p == nil {
				return false
			}
		}
	}
	for bb := b; bb != nil; bb = bb.Parent {
		for _, loc := range bb.LocalVars {
			if loc == nil {
				return false
			}
		}
	}
	return true
}

// IsVarOnStack mirrors Block::is_var_on_stack.
// Block.cpp:443–456 — params + local_vars chain.
// IsVarOnStack reports whether v is a param or local visible on this block chain.
// Incomplete Block/Variable/Param/LocalVars sticky false (no invent not-on-stack
// / soft re-pick past holes).
func (b *Block) IsVarOnStack(v *Variable) bool {
	// Block + Variable always live; sticky incomplete no invent not-on-stack
	if b == nil || v == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !b.StackScanComplete() {
		// incomplete Param/LocalVars sticky fail closed not-on-stack
		// residual ERROR sticky — no invent soft not-on-stack past StackScan residual
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return false
	}
	f := b.Func
	for bb := b; f == nil && bb != nil; bb = bb.Parent {
		f = bb.Func
	}
	if f != nil {
		for _, p := range f.Param {
			// Param live after StackScanComplete; nil hole already sticky above
			if p == nil {
				sessNoteError(nil, ErrGeneric)
				return false
			}
			if p.Match(v) {
				// residual ERROR sticky — no invent on-stack true past Match hole
				if sessHasError(nil) {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue then true later past Match hole
			if sessHasError(nil) {
				return false
			}
		}
	}
	for bb := b; bb != nil; bb = bb.Parent {
		for _, loc := range bb.LocalVars {
			if loc == nil {
				sessNoteError(nil, ErrGeneric)
				return false
			}
			if loc == v {
				return true
			}
			if loc.Match(v) {
				// residual ERROR sticky — no invent on-stack true past Match hole
				if sessHasError(nil) {
					return false
				}
				return true
			}
			// residual ERROR sticky — no invent soft-continue then true later past Match hole
			if sessHasError(nil) {
				return false
			}
		}
	}
	return false
}

// CreateNewTmpVar mirrors Block::create_new_tmp_var.
// Block.cpp:216–219 — always gensym("t_") (util.cpp process-wide gensym_count);
// no invent VS.Sym private counter (that desynced t_ from g_/l_/func_).
// sym is ignored; kept for call-site compatibility.
func (b *Block) CreateNewTmpVar(sym *GenSym, st ESimpleType) string {
	_ = sym
	// Block.cpp:216–219 — this always live; gensym + macro_tmp_vars insert together
	// sticky no invent bare t_N without block registration (would emit undeclared use)
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Block.cpp:217 — const string var_name = gensym("t_"); sticky no invent bare ""
	name := Gensym("t_")
	if name == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	if b.TmpVars == nil {
		b.TmpVars = make(map[string]ESimpleType)
	}
	b.TmpVars[name] = st
	return name
}

// BlockProbability mirrors Block.cpp BlockProbability.
// Block.cpp:87–93 — VectorFilter Keep on {block_size-1} then
// filter.disable(fDefault). In random mode valid_filter() is false so
// filter() never rejects → uniform rnd_upto(block_size) in [0, block_size).
func BlockProbability(blockSize int, r *Rng) int {
	if blockSize < 1 {
		return 0
	}
	if r == nil {
		// C++ always has RNG; sticky fail-closed → 0
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	// Block.cpp:88–92 — Keep {block_size-1}, disable fDefault, rnd_upto
	f := NewVectorFilterItems([]int{blockSize - 1}, FilterModeKeep)
	f.Disable(FilterKindDefault)
	return int(r.RndUptoFilter(uint32(blockSize), f))
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
	// Block::make_random always has RNG + CGContext sticky
	if r == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// Block.cpp:120 — assert(curr_func) sticky; no soft invent parentless block
	f := cg.CurrentFunc
	if f == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky before stack push (no invent block past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	parent := (*Block)(nil)
	if len(f.Stack) > 0 {
		parent = f.Stack[len(f.Stack)-1]
	}
	b := &Block{
		Parent:           parent,
		Func:             f,
		Looping:          looping,
		blockSize:        opts.MaxBlockSize,
		EmitDepthProtect: opts.DepthProtect,
		// step_hash def/decl/call gated with ComputeHash (hashHelpersEnabled)
		// no invent step_hash(n) without live helper defs
		EmitStepHash:   opts.StepHashByStmt && opts.ComputeHash,
		EmitLabelAttrs: opts.LabelAttributes,
		LabelAttrRng:   r,
		StmID:          AllocStmID(),
		// Block.cpp:127 — in_array_loop when induction bounds non-empty
		InArrayLoop:  len(cg.IVBounds) > 0,
		EmitParanoid: opts.Paranoid,
		EmitConcise:  opts.Concise,
		EmitFM:       cg.FM,
	}
	// Block.cpp:132–133 — stack + blocks push
	f.Stack = append(f.Stack, b)
	f.Blocks = append(f.Blocks, b)
	// DepthSpec::depth_guard_by_type(dtBlock) — random mode always GOOD
	if DepthGuardByType(opts, "dtBlock") == BadDepth {
		abortBlockMake(f, b)
		return nil
	}
	max := BlockProbability(b.blockSize, r)
	// Block.cpp:136–140 — ERROR after BlockProbability → delete block
	if sessHasError(cgSess(cg)) {
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
	// Incomplete accum/facts fail closed (no invent post_creation / return block past holes)
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		if !EffectComplete(*cg.EffectAccum) {
			abortBlockMake(f, b)
			sessNoteError(cgSess(cg), ErrGeneric)
			return nil
		}
		preEffect = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-block past Effect Clone residual
		if sessHasError(cgSess(cg)) {
			abortBlockMake(f, b)
			return nil
		}
	}
	if !EffectComplete(preEffect) {
		abortBlockMake(f, b)
		sessNoteError(cgSess(cg), ErrGeneric)
		return nil
	}
	// StmID always allocated at make; FM path always records map_facts_in
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			abortBlockMake(f, b)
			sessNoteError(cgSess(cg), ErrGeneric)
			return nil
		}
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
		// Factories always AllocStmID (C++ Statement ctor). Do not re-alloc on
		// StmID==0 — 0 is a valid first id after fair sid.
		if StmIDUnset(st.StmID) {
			st.StmID = AllocStmID()
		}
		if pendingFwd != "" {
			if st.SourceLabel == "" {
				st.SourceLabel = pendingFwd
			} else {
				// already labeled — keep pending as no-op marker after previous
				// StmtLabel is Go emit-only; still needs a sid for map keys if visited.
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
		must := st.MustReturn()
		// residual ERROR sticky — no invent soft-continue more stmts past MustReturn residual
		if sessHasError(cgSess(cg)) {
			break
		}
		if must {
			break
		}
	}
	if pendingFwd != "" {
		b.Stmts = append(b.Stmts, Stmt{Kind: StmtLabel, SourceLabel: pendingFwd, StmID: AllocStmID()})
	}
	// Block.cpp:157–161 — ERROR after stmt loop → delete block
	if sessHasError(cgSess(cg)) {
		abortBlockMake(f, b)
		return nil
	}
	// Block.cpp:164–166 — nested loop for must-use multi-dim arrays
	if b.NeedNestedLoop(*cg, r) && cg.BlkDepth < opts.MaxBlockDepth {
		b.AppendNestedLoop(r, opts, probs, vs, tables, stmtTab, cg)
		// append_nested_loop ERROR_GUARD(nullptr) on for make fail
		if sessHasError(cgSess(cg)) {
			abortBlockMake(f, b)
			return nil
		}
	}
	// Block::post_creation_analysis (Block.cpp:682–742)
	// Upstream appends return only inside post_creation when still missing.
	// Without FactMgr, append return here so function bodies stay valid C.
	if cg.FM == nil && parent == nil && f != nil && f.NeedReturnStmt() {
		must := b.MustReturn()
		// residual ERROR sticky — no invent soft-append return past MustReturn residual
		if sessHasError(cgSess(cg)) {
			abortBlockMake(f, b)
			return nil
		}
		if !must {
			ret := MakeRandomReturn(r, opts, vs, cg)
			if stmtOK(ret) {
				if StmIDUnset(ret.StmID) {
					ret.StmID = AllocStmID()
				}
				b.Stmts = append(b.Stmts, ret)
			}
		}
	}
	// b.StmID allocated at make (line above); never re-alloc valid id 0
	if StmIDUnset(b.StmID) {
		b.StmID = AllocStmID()
	}
	b.PostCreationAnalysis(cg, opts, preEffect, r, vs)
	// incomplete post-creation GlobalFacts fail closed even without sticky ERROR
	// (no invent return live block past IncompleteFactSlice wipe)
	if sessHasError(cgSess(cg)) || (cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts)) {
		// Block.cpp:170–174 — ERROR after post_creation → delete
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		abortBlockMake(f, b)
		return nil
	}
	// Block.cpp:178 — stack.pop_back() (always; C++ does not identity-check).
	// Identity-safe pop skipped the frame when a nested invented FP push left a
	// non-self top, leaking stack depth for SelectParentLocal (seed-2 e13830).
	if f != nil && len(f.Stack) > 0 {
		f.Stack = f.Stack[:len(f.Stack)-1]
	}
	// Block.cpp:187 — Error::set_error(SUCCESS)
	sessClearError(cgSess(cg))
	return b
}

// abortBlockMake pops stack after a failed Block::make_random.
// Block.cpp:142–174 — on ERROR: stack.pop_back(); delete b; return nullptr.
// C++ does NOT erase from func->blocks (only remove_stmt does Block.cpp:653–660).
// Leaving the entry matches StatementGoto::make_random's vector copy of func->blocks
// (seed-2 first_div e12688: invent erase → n=11 vs upstream n=14).
// ~Block clears stms (nested Statement destructors delete nested Blocks) so
// find_good_jump_block treats the dangling entry as empty (StatementGoto.cpp:333–336).
// Soft invent left live Stmts on aborted blocks → usable goto pool inflation
// (seed 11466719812903307384).
// Function + Block always live on make abort; sticky (no invent soft-skip cleanup past hole).
func abortBlockMake(f *Function, b *Block) {
	if f == nil || b == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if n := len(f.Stack); n > 0 && f.Stack[n-1] == b {
		f.Stack = f.Stack[:n-1]
	}
	// no invent f.Blocks erase — C++ delete leaves the pointer in func->blocks
	tombstoneBlock(b)
}

// tombstoneBlock mirrors C++ delete on a Block* that remains on func->blocks:
// Block::~Block clears stms; nested StatementIf/For destructors delete branch/body
// Blocks (also left empty on func->blocks). Does not erase Function.Blocks.
func tombstoneBlock(b *Block) {
	if b == nil {
		return
	}
	for i := range b.Stmts {
		tombstoneStmt(&b.Stmts[i])
	}
	b.Stmts = nil
	b.LocalVars = nil
	b.BreakStmIDs = nil
}

// tombstoneStmt mirrors delete on a Statement* owned by a tombstoned Block.
func tombstoneStmt(st *Stmt) {
	if st == nil {
		return
	}
	if st.Then != nil {
		tombstoneBlock(st.Then)
		st.Then = nil
	}
	if st.Else != nil {
		tombstoneBlock(st.Else)
		st.Else = nil
	}
}

// PostCreationAnalysis mirrors Block::post_creation_analysis.
// Block.cpp:682–742 — effects, OOS, optional fixed-point with remove_stmt, append_return.
// Incomplete preEffect / StmID 0 fails closed sticky (no invent fixed-point / map
// record / soft-reset EffectAccum from IncompleteEffect shell).
// Block + CGContext always live; sticky (no invent soft-skip past hole).
// Nil FM is non-sticky soft re-pick (sticky poisons soft factories without FM).
func (b *Block) PostCreationAnalysis(cg *CGContext, opts Options, preEffect Effect, r *Rng, vs *VariableSelector) {
	if b == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return
	}
	fm := cg.FM
	if fm == nil {
		return
	}
	if StmIDUnset(b.StmID) {
		fm.GlobalFacts = IncompleteFactSlice()
		sessNoteError(cgSess(cg), ErrGeneric)
		return
	}
	if !EffectComplete(preEffect) {
		fm.GlobalFacts = IncompleteFactSlice()
		sessNoteError(cgSess(cg), ErrGeneric)
		return
	}
	if fm.MapVisited == nil {
		fm.MapVisited = make(map[int]bool)
	}
	// Block.cpp:687 — map_visited[this]=true before find_fixed_point so the first
	// iteration merges back-edge map_facts_out (incl. self-back post-OOS with body
	// effects such as may-null) into current_inputs (Block.cpp:525–536). Skipping
	// this left map_visited false + visit_once false → pure shortcut on entry and
	// map_facts_in never absorbed may-null; post_loop then wiped live lattice
	// (seed-2 first_div 10107: auto_statement_for_631 WIPE).
	fm.MapVisited[b.StmID] = true
	b.SetAccumulatedEffect(fm)
	// incomplete block map_stm_effect fails closed (no invent continue post-analysis)
	if !EffectComplete(fm.GetMapStmEffect(b.StmID)) {
		fm.GlobalFacts = IncompleteFactSlice()
		sessNoteError(cgSess(cg), ErrGeneric)
		return
	}
	// incomplete GlobalFacts fail closed sticky (no invent cleaned postFacts / OOS from holes)
	// Use IncompleteFactSlice — bare nil invents empty success via FactsComplete(nil)
	// postFacts / postUnion split C++ post_facts FactVec (ePointTo + eUnionWrite).
	// Block.cpp:690 snapshots full global_facts before OOS; 747 restores for
	// append_return_stmt. Soft invent restored PT-only → body-local union fields
	// nonreadable at return choose (seed-49 l_593[3][2].f0 pool n=17 vs UP n=18).
	var postFacts []*FactPointTo
	var postUnion []*FactUnion
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		postFacts = IncompleteFactSlice()
		postUnion = IncompleteUnionFactSlice()
		fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
		sessNoteError(cgSess(cg), ErrGeneric)
		return
	} else {
		// Block.cpp:690–693 — post_facts snapshot; OOS for map_out.
		// C++ mutates global_facts then runs FP on a separate inputs vector.
		// Go StmVisitFacts uses GlobalFacts as the working set and saves live as
		// liveSaved — OOS on GlobalFacts before FP poisons re-analysis with body-local
		// garbage pointees (seed-2 l_260). Build post-OOS map_out on a clone; keep
		// GlobalFacts pre-OOS during FP; install map_out / OOS at end.
		postFacts = CloneFactSlice(fm.GlobalFacts)
		// residual ERROR sticky — no invent soft-post past CloneFactSlice residual
		if sessHasError(cgSess(cg)) {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			postUnion = IncompleteUnionFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		// Block.cpp:690 — full FactVec copy includes eUnionWrite.
		if !UnionFactsComplete(fm.UnionFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			postFacts = IncompleteFactSlice()
			postUnion = IncompleteUnionFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			sessNoteError(cgSess(cg), ErrGeneric)
			return
		}
		postUnion = CloneUnionFactSliceDeep(fm.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(postUnion) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			postFacts = IncompleteFactSlice()
			postUnion = IncompleteUnionFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		if postUnion == nil {
			postUnion = []*FactUnion{}
		}
		outPost := CloneFactSlice(fm.GlobalFacts)
		if sessHasError(cgSess(cg)) || !FactsComplete(outPost) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		if len(b.LocalVars) > 0 {
			UpdateFactsForOOSVars(b.LocalVars, &outPost)
			if !FactsComplete(outPost) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				return
			}
		}
		fm.RemoveRVFacts(&outPost)
		if !FactsComplete(outPost) {
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		// FactMgr.cpp:268–270 — set_fact_out(Block*): parent==nullptr → remove_function_local_facts
		fm.SetMapFactsOutForBlock(b, outPost)
		if sessHasError(cgSess(cg)) {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			return
		}

		// Block.cpp:696–697 — fixed-point when:
		//   is_loop_body || need_revisit || has_edge_in(false, true)
		// has_edge_in: Statement.cpp:434–446 — e->dest == this (the block statement).
		// Do not invent ContainsBackEdge (dest->parent==this) or FindEdgesInToBlock:
		// those force FP on blocks C++ leaves with mid-gen global_facts (seed-2 e10107
		// wipe via auto_block_959 after unnecessary FP; e12688 over-strip).
		mustBR := b.MustBreakOrReturnFull(fm)
		// residual ERROR sticky — no invent soft-fixed-point past MustBreakOrReturn residual
		if sessHasError(cgSess(cg)) {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		isLoopBody := !mustBR && b.Looping
		hasBack := fm.HasEdgeIn(b.StmID, false, true)
		// residual ERROR sticky — HasEdgeIn sets sticky on incomplete CFG
		if sessHasError(cgSess(cg)) {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		if isLoopBody || b.NeedRevisit || hasBack {
			selfBack := false
			if isLoopBody {
				fromTail := b.FromTailToHead()
				// residual ERROR sticky — no invent soft-self-back past FromTailToHead residual
				if sessHasError(cgSess(cg)) {
					fm.GlobalFacts = IncompleteFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					return
				}
				if fromTail {
					selfBack = true
					fm.CreateCFGEdge(b.StmID, b, false, true)
				}
			}
			// incomplete MapFactsIn fails closed — C++ map[] missing is empty complete;
			// holes must not invent empty fixed-point re-analysis as success
			in0 := fm.GetMapFactsIn(b.StmID)
			if !FactsComplete(in0) {
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				sessNoteError(cgSess(cg), ErrGeneric)
				return
			} else {
				factsCopy := CloneFactSlice(in0)
				// residual ERROR sticky — no invent soft-fixed-point past CloneFactSlice residual
				if sessHasError(cgSess(cg)) {
					fm.GlobalFacts = IncompleteFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					return
				}
				// Block.cpp:703 — facts_copy = map_facts_in[this] is a full FactVec
				// (ePointTo + eUnionWrite). Go splits categories: factsCopy is the
				// point-to half. Snapshot eUnionWrite entry here (before FP / strip
				// retries): FindFixedPointBlock seeds currentUnions from live
				// UnionFacts (visit path: caller already installed entry). At
				// post_creation, live is still the post-generation lattice (e.g.
				// BOTTOM after if-combine) while map_facts_in holds the block entry.
				// Soft invent left live as currentUnions → set_fact_in wrote BOTTOM
				// into map_in and break re-visits saw BOTTOM (seed-123 g_721:
				// post_loop map_in BOTTOM + break merge after for body with
				// if/else for-IV-only-in-else). Install this snapshot as live before
				// each find_fixed_point so currentUnions matches C++ facts_copy.
				inU0 := fm.GetMapUnionFactsIn(b.StmID)
				if !UnionFactsComplete(inU0) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					sessNoteError(cgSess(cg), ErrGeneric)
					return
				}
				entryUnionsSnap := CloneUnionFactSliceDeep(inU0)
				if sessHasError(cgSess(cg)) || !UnionFactsComplete(entryUnionsSnap) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					return
				}
				if entryUnionsSnap == nil {
					entryUnionsSnap = []*FactUnion{}
				}
				// reset accum to pre-block effect
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preEffect.Clone()
					// residual ERROR sticky — no invent soft-reset past Effect Clone residual
					if sessHasError(cgSess(cg)) {
						fm.GlobalFacts = IncompleteFactSlice()
						postFacts = IncompleteFactSlice()
						fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
						return
					}
				}
				for {
					// Re-install entry eUnionWrite each attempt (reset_stm_fact_maps
					// / prior FP may have left live at mid-body last-writes).
					entryU := CloneUnionFactSliceDeep(entryUnionsSnap)
					if sessHasError(cgSess(cg)) || !UnionFactsComplete(entryU) {
						fm.GlobalFacts = IncompleteFactSlice()
						fm.UnionFacts = IncompleteUnionFactSlice()
						postFacts = IncompleteFactSlice()
						fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
						if !sessHasError(cgSess(cg)) {
							sessNoteError(cgSess(cg), ErrGeneric)
						}
						return
					}
					if entryU == nil {
						fm.UnionFacts = []*FactUnion{}
					} else {
						fm.UnionFacts = entryU
					}
					fpOut, fpUnions, failIdx, ok := FindFixedPointBlock(b, factsCopy, cg, opts, b.NeedRevisit)
					if ok {
						// Block.cpp:706–728 + find_fixed_point Block.cpp:558 —
						// full visit assigns post_facts = pre-OOS outputs; pure
						// shortcut leaves post_facts (line-690 snapshot) unchanged.
						// FindFixedPointBlock returns nil,nil on pure shortcut.
						// Full FactVec: also refresh postUnion from pre-OOS eUnionWrite
						// captured at the same visit (not live after ShortcutAnalysis
						// installs post-OOS map_union_out).
						if fpOut != nil {
							postFacts = fpOut
							if !UnionFactsComplete(fpUnions) {
								fm.GlobalFacts = IncompleteFactSlice()
								fm.UnionFacts = IncompleteUnionFactSlice()
								postFacts = IncompleteFactSlice()
								postUnion = IncompleteUnionFactSlice()
								fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
								sessNoteError(cgSess(cg), ErrGeneric)
								return
							}
							postUnion = fpUnions
							if postUnion == nil {
								postUnion = []*FactUnion{}
							}
						}
						break
					}
					// remove from fail index through end (Block.cpp:709–714)
					if failIdx < 0 {
						failIdx = 0
					}
					for failIdx < len(b.Stmts) {
						id := b.Stmts[failIdx].StmID
						if id == 0 {
							// incomplete stm_id — fail closed strip tail (no invent
							// soft-skip hole and keep later stmts as complete block)
							b.Stmts = append(b.Stmts[:failIdx], b.Stmts[failIdx+1:]...)
							continue
						}
						if n := b.RemoveStmt(id, fm); n == 0 {
							b.Stmts = append(b.Stmts[:failIdx], b.Stmts[failIdx+1:]...)
						}
					}
					b.NeedRevisit = true
					fm.ResetBlockFactMaps(b)
					if !selfBack {
						fromTail := b.FromTailToHead()
						// residual ERROR sticky — no invent soft-self-back past FromTailToHead residual
						if sessHasError(cgSess(cg)) {
							fm.GlobalFacts = IncompleteFactSlice()
							postFacts = IncompleteFactSlice()
							fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
							return
						}
						if fromTail {
							selfBack = true
							fm.CreateCFGEdge(b.StmID, b, false, true)
						}
					}
					if cg.EffectAccum != nil {
						*cg.EffectAccum = preEffect.Clone()
						// residual ERROR sticky — no invent soft-reset past Effect Clone residual
						if sessHasError(cgSess(cg)) {
							fm.GlobalFacts = IncompleteFactSlice()
							postFacts = IncompleteFactSlice()
							fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
							return
						}
					}
					// Block.cpp:706–728 — after strip + reset_stm_fact_maps, always re-enter
					// find_fixed_point (even with empty stms). Empty body still set_fact_in/out
					// from inputs. Breaking here left MapFactsIn/Out deleted → complete-empty
					// postLoop/global_facts (seed-2 e2308: EV rejects ** with nfacts=0).
					if len(b.Stmts) == 0 {
						entryUEmpty := CloneUnionFactSliceDeep(entryUnionsSnap)
						if sessHasError(cgSess(cg)) || !UnionFactsComplete(entryUEmpty) {
							fm.GlobalFacts = IncompleteFactSlice()
							fm.UnionFacts = IncompleteUnionFactSlice()
							postFacts = IncompleteFactSlice()
							fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
							if !sessHasError(cgSess(cg)) {
								sessNoteError(cgSess(cg), ErrGeneric)
							}
							return
						}
						if entryUEmpty == nil {
							fm.UnionFacts = []*FactUnion{}
						} else {
							fm.UnionFacts = entryUEmpty
						}
						fpEmpty, fpEmptyU, _, okEmpty := FindFixedPointBlock(b, factsCopy, cg, opts, true)
						if okEmpty {
							if fpEmpty != nil {
								postFacts = fpEmpty
								if !UnionFactsComplete(fpEmptyU) {
									fm.GlobalFacts = IncompleteFactSlice()
									fm.UnionFacts = IncompleteUnionFactSlice()
									postFacts = IncompleteFactSlice()
									postUnion = IncompleteUnionFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									sessNoteError(cgSess(cg), ErrGeneric)
									return
								}
								postUnion = fpEmptyU
								if postUnion == nil {
									postUnion = []*FactUnion{}
								}
							}
						} else {
							// install empty-body maps from entry facts (C++ find_fixed_point)
							if !FactsComplete(factsCopy) {
								fm.GlobalFacts = IncompleteFactSlice()
								postFacts = IncompleteFactSlice()
								fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
								sessNoteError(cgSess(cg), ErrGeneric)
								return
							}
							fm.SetMapFactsIn(b.StmID, factsCopy)
							// pre-OOS outputs = entry + local facts (Block.cpp:558)
							preOOS := CloneFactSlice(factsCopy)
							for _, v := range b.LocalVars {
								if v == nil {
									fm.GlobalFacts = IncompleteFactSlice()
									postFacts = IncompleteFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									sessNoteError(cgSess(cg), ErrGeneric)
									return
								}
								AddNewVarFactTo(v, &preOOS)
								if !FactsComplete(preOOS) {
									fm.GlobalFacts = IncompleteFactSlice()
									postFacts = IncompleteFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									sessNoteError(cgSess(cg), ErrGeneric)
									return
								}
							}
							postFacts = preOOS
							// PT-only OOS on outCopy; SetMapFactsOutForBlock OOS-clones
							// live unions for map_union_out (do not mutate live via
							// fm.UpdateFactsForOOSVars — that also strips UnionFacts).
							outCopy := CloneFactSlice(preOOS)
							if len(b.LocalVars) > 0 {
								UpdateFactsForOOSVars(b.LocalVars, &outCopy)
								if !FactsComplete(outCopy) {
									if !sessHasError(cgSess(cg)) {
										sessNoteError(cgSess(cg), ErrGeneric)
									}
									fm.GlobalFacts = IncompleteFactSlice()
									postFacts = IncompleteFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									return
								}
							}
							fm.SetMapFactsOutForBlock(b, outCopy)
							if sessHasError(cgSess(cg)) {
								fm.GlobalFacts = IncompleteFactSlice()
								postFacts = IncompleteFactSlice()
								return
							}
						}
						break
					}
				}
				// Block.cpp:729 — global_facts = map_facts_out[this]  // full FactVec
				// post_facts already set by find_fixed_point (pre-OOS) or line-690
				// Soft invent was SetGlobalFacts(PT-only): UnionFacts left mid-FP.
				// incomplete out fails closed (hole marker — no invent keep prior / empty)
				out := fm.GetMapFactsOut(b.StmID)
				unOut := fm.GetMapUnionFactsOut(b.StmID)
				if !FactsComplete(out) || !UnionFactsComplete(unOut) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					sessNoteError(cgSess(cg), ErrGeneric)
					return
				}
				fm.AssignGlobalFactsFromMapOut(b.StmID)
				// residual ERROR sticky — no invent soft-out past full FactVec assign residual
				if sessHasError(cgSess(cg)) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					return
				}
			}
		} else {
			// No FP: Block.cpp:723–726 mutates live global_facts in place
			// (OOS locals + remove_rv); set_fact_out only fills map_out.
			// Block.cpp:772 assigns map_facts_out → global_facts only on the FP
			// arm. Soft invent (1) SetGlobalFacts(PT-only) left live UnionFacts
			// mid-body (local eUnionWrite subjects) → same_facts size skew;
			// (2) AssignGlobalFactsFromMapOut wrongly applied remove_function_local
			// (parent==nullptr) to the *live* env — C++ never does that on no-FP.
			// Live = post-OOS full FactVec: outPost for PT + OOS UnionFacts locals.
			if !FactsComplete(outPost) {
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				sessNoteError(cgSess(cg), ErrGeneric)
				return
			}
			fm.SetGlobalFacts(CloneFactSlice(outPost), "auto_block_oos_no_fp")
			if sessHasError(cgSess(cg)) {
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				return
			}
			// FactMgr.cpp:141–156 — OOS erase is category-agnostic (eUnionWrite too).
			if !UnionFactsComplete(fm.UnionFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				sessNoteError(cgSess(cg), ErrGeneric)
				return
			}
			if len(b.LocalVars) > 0 {
				UpdateUnionFactsForOOSVars(b.LocalVars, &fm.UnionFacts)
				if !UnionFactsComplete(fm.UnionFacts) || sessHasError(cgSess(cg)) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					return
				}
			}
			// C++ does not create self-back outside the FP branch (is_loop_body ||
			// need_revisit || has_edge_in). Soft invent created self-back when
			// Looping && !isLoopBody (must_break_or_return) && from_tail — that
			// path never runs FP strip, so later re-visits merge a never-validated
			// self-back. Only create self-back when C++ would (isLoopBody arm above).
		}
	}
	// Block.cpp:687 already set map_visited; find_fixed_point also sets (561). Reaffirm.
	fm.MapVisited[b.StmID] = true
	// Block.cpp:734–741 — append return for top-level body when still missing
	// incomplete postFacts must not invent return gen via FactsComplete(nil) empty
	if b.Parent == nil && b.Func != nil && b.Func.NeedReturnStmt() {
		must := b.MustReturn()
		// residual ERROR sticky — no invent soft-append return past MustReturn residual
		if sessHasError(cgSess(cg)) {
			fm.GlobalFacts = IncompleteFactSlice()
			if !FactsComplete(postFacts) {
				postFacts = IncompleteFactSlice()
			}
			return
		}
		if !must {
			// Block.cpp:747 — fm->global_facts = post_facts (full FactVec pre-OOS).
			// Soft invent restored PT-only; live UnionFacts stayed post-OOS so
			// is_nonreadable_field dropped body-local union fields at return choose.
			if !FactsComplete(postFacts) || !UnionFactsComplete(postUnion) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				sessNoteError(cgSess(cg), ErrGeneric)
				return
			}
			fm.SetGlobalFacts(postFacts, "auto_block_1002")
			// AppendReturnStmt soft-nils without RNG (MakeDummyBlock tests). C++ only
			// reaches 747 when make_random(eReturn) has live RNG — restore eUnionWrite
			// only when we will actually append so a soft-skip does not reinstall
			// pre-OOS body-local unions after the no-FP OOS path.
			if r != nil {
				restU := CloneUnionFactSliceDeep(postUnion)
				if sessHasError(cgSess(cg)) || !UnionFactsComplete(restU) {
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					return
				}
				if restU == nil {
					fm.UnionFacts = []*FactUnion{}
				} else {
					fm.UnionFacts = restU
				}
			}
			if b.AppendReturnStmt(r, opts, vs, cg) == nil {
				// append_return_stmt ERROR_GUARD / assert(visited) leave sticky error
				return
			}
			// Block.cpp:740 — set_fact_out(this, map_facts_out[sr])
			// C++ map[] always reads sr out (missing → empty); no invent skip set_fact_out
			// Body parent==nullptr → remove_function_local_facts again (FactMgr.cpp:268–270).
			if len(b.Stmts) > 0 {
				sr := &b.Stmts[len(b.Stmts)-1]
				// return stm_id always live after append_return; StmID 0 → Incomplete via getter
				out := fm.GetMapFactsOut(sr.StmID)
				if FactsComplete(out) {
					fm.SetMapFactsOutForBlock(b, out)
					if sessHasError(cgSess(cg)) {
						return
					}
				} else {
					// incomplete sr out — fail closed sticky hole marker (not empty complete)
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					sessNoteError(cgSess(cg), ErrGeneric)
					return
				}
			}
		}
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
	// Statement.cpp:243 default t = MAX_STATEMENT_TYPE → random via filter
	return makeRandomStmtForced(r, opts, probs, vs, tables, stmtTab, cg, b, MaxStatementType)
}

// makeRandomStmtForced mirrors Statement::make_random(cg, t) with optional forced kind.
// forceKind == MaxStatementType → StatementProbability each try (default make_random).
// forceKind otherwise → first try uses forceKind; null without error re-picks randomly
// (Statement.cpp:314–316 recursive make_random() without forced t).
// Block::append_nested_loop calls make_random(cg, eFor) — Statement.cpp / Block.cpp:424.
func makeRandomStmtForced(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
	b *Block,
	forceKind StatementType,
) Stmt {
	// Statement.cpp always has RNG + CGContext; sticky no invent MAX-kind shell without them
	if r == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient EffectContext fails closed sticky before re-pick loop
	// (no invent stmt under incomplete context shells; EffectStm is cleared per try)
	if !EffectComplete(cg.EffectContext()) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// Statement.cpp:243–244 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtStatement, t, nullptr)
	// flag is the requested t (eFor for append_nested_loop; MAX when choosing randomly).
	guardT := int(forceKind)
	if forceKind == MaxStatementType {
		guardT = int(MaxStatementType)
	}
	if DepthGuardByTypeFlag(opts, DtStatement, guardT) == BadDepth {
		return Stmt{}
	}
	// Statement static ProbabilityTable always live; sticky no invent NewStatementThresholdTable
	if stmtTab == nil {
		stmtTab = sessStmtTab(cgSess(cg))
	}
	if stmtTab == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	// StatementFilter (Statement.cpp:150–182)
	f := filterFunc(func(v uint32) bool {
		k := NumberToType(stmtTab, v)
		// Statement.cpp:158–160 — PartialExpander::expand_check
		if !ExpandCheckSess(cgSess(cg), k) {
			return true
		}
		// Statement.cpp:164–166 — eBlock always filtered
		if k == StmtBlock {
			return true
		}
		// Statement.cpp:167–169 — void functions cannot return
		if k == StmtReturn && cg.CurrentFunc != nil && cg.CurrentFunc.ReturnType != nil {
			isSimple := cg.CurrentFunc.ReturnType.IsSimple()
			// residual ERROR sticky — no invent filter keep/reject past IsSimple residual
			if sessHasError(cgSess(cg)) {
				return true
			}
			if isSimple {
				st := cg.CurrentFunc.ReturnType.Simple()
				// residual ERROR sticky — no invent filter keep/reject past Simple residual
				if sessHasError(cgSess(cg)) {
					return true
				}
				if st == EVoid {
					return true
				}
			}
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
		// ReachMaxFunctions nil-Func holes are non-sticky restrictive max (soft re-pick)
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
		var kind StatementType
		if tries == 0 && forceKind != MaxStatementType {
			// Block.cpp:424 / Statement.cpp:259 — caller passed eFor (or other forced t)
			kind = forceKind
		} else {
			kind = StatementProbabilityFilter(r, stmtTab, f)
		}
		// Statement.cpp:248–250 — stop_by_stmt forces return after sid threshold
		if opts.StopByStmt >= 0 && currentSession().NextStmID >= opts.StopByStmt {
			kind = StmtReturn
		}
		// Statement.cpp:260–261 — pre_facts / pre_effect (accum) snapshot before make
		// C++: FactVec pre_facts = fm->global_facts; shallow copy of Fact* vector
		// (ePointTo + eUnionWrite). Nested ExpressionAssign merges replace pointers
		// in global_facts only; pre_facts keeps the pre-make Fact* set for set_fact_in.
		// Deep CloneFactSlice isolated pointees incorrectly vs that sharing model
		// (seed-2 e10107 may-null). incomplete GlobalFacts/UnionFacts/accum fail
		// closed sticky (no invent cleaned pre-stmt snapshot or soft re-pick past holes)
		var preFacts []*FactPointTo
		var preUnion []*FactUnion
		if cg.FM != nil {
			if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return Stmt{}
			}
			preFacts = append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
			preUnion = append([]*FactUnion(nil), cg.FM.UnionFacts...)
		}
		preEffect := EmptyEffect()
		if cg.EffectAccum != nil {
			if !EffectComplete(*cg.EffectAccum) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return Stmt{}
			}
			preEffect = cg.EffectAccum.Clone()
			// residual ERROR sticky — no invent soft-stmt past Effect Clone residual
			if sessHasError(cgSess(cg)) {
				return Stmt{}
			}
		}
		if !EffectComplete(preEffect) {
			sessNoteError(cgSess(cg), ErrGeneric)
			return Stmt{}
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
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		if stmtOK(st) {
			// Statement.cpp:320 — post_creation_analysis(pre_facts, pre_effect)
			// incomplete post-creation must not invent stmt success past wiped facts
			PostCreationAnalysis(&st, preFacts, preUnion, preEffect, cg, opts)
			if sessHasError(cgSess(cg)) || (cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts)) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return Stmt{}
			}
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
	// Statement.cpp always has live RNG + CGContext; sticky fail closed (no invent shell)
	if r == nil || cg == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
	switch kind {
	case StmtReturn:
		return MakeRandomReturn(r, opts, vs, cg)
	case StmtAssign:
		// Write effects: Lhs::visit_facts + merge_param_context inside MakeRandomAssign.
		// No NoteWrite(LhsVar) — that wrongly marks pointers on *p=… (see StatementAssign).
		st := MakeRandomAssign(r, opts, probs, vs, tables, cg, nil)
		// residual ERROR sticky — no invent soft-return assign past MakeRandomAssign residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtBreak:
		st := MakeRandomBreak(r, opts, vs, tables, cg)
		// residual ERROR sticky — no invent soft-return break past MakeRandomBreak residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtContinue:
		st := MakeRandomContinue(r, opts, vs, tables, cg, b)
		// residual ERROR sticky — no invent soft-return continue past MakeRandomContinue residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtIfElse:
		if st := MakeRandomIf(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			// residual ERROR sticky — no invent soft-return if past MakeRandomIf residual
			if sessHasError(cgSess(cg)) {
				return Stmt{}
			}
			return *st
		}
		// residual ERROR sticky — no invent soft re-pick past MakeRandomIf residual nil
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		// null factory → re-pick (Statement.cpp:314); incomplete shell fails stmtOK
		return Stmt{}
	case StmtFor:
		if st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			// residual ERROR sticky — no invent soft-return for past MakeRandomFor residual
			if sessHasError(cgSess(cg)) {
				return Stmt{}
			}
			return *st
		}
		// residual ERROR sticky — no invent soft re-pick past MakeRandomFor residual nil
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return Stmt{}
	case StmtArrayOp:
		st := MakeRandomArrayOp(r, opts, probs, vs, tables, stmtTab, cg)
		// residual ERROR sticky — no invent soft-return array-op past MakeRandomArrayOp residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtGoto:
		st := MakeRandomGoto(r, opts, probs, vs, tables, cg, b)
		// residual ERROR sticky — no invent soft-return goto past MakeRandomGoto residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtInvoke:
		st := MakeRandomExprStmt(r, opts, probs, vs, tables, cg)
		// residual ERROR sticky — no invent soft-return invoke past MakeRandomExprStmt residual
		if sessHasError(cgSess(cg)) {
			return Stmt{}
		}
		return st
	case StmtBlock:
		// Statement.cpp:281–282 — Block::make_random; filter usually drops eBlock
		if nested := MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false); nested != nil {
			return Stmt{Kind: StmtBlock, Then: nested, StmID: nested.StmID}
		}
		return Stmt{}
	default:
		// Statement.cpp:275–277 — assert(!"unknown Statement type"); fail closed
		sessNoteError(cgSess(cg), ErrGeneric)
		return Stmt{}
	}
}

// stmtOK reports whether a generated statement is usable (non-null factory).
// Incomplete shells fail closed false (no invent usable Kind-only / partial for/if IR).
func stmtOK(st Stmt) bool {
	switch st.Kind {
	case StmtAssign:
		// StatementAssign always has live lhs + rhs after make_random
		if st.Expr == nil {
			return false
		}
		return st.LhsVar != nil || st.ArrayAccess != "" || (st.Lhs != nil && st.Lhs.Var != nil)
	case StmtInvoke:
		return st.Expr != nil && st.Expr.Invoke != nil && !st.Expr.Invoke.Failed
	case StmtFor:
		// StatementFor always has init/test/incr + body (StatementFor.cpp make_random)
		// no invent OK from IV alone without test/body
		if st.Loop == nil || st.Loop.IV == nil {
			return false
		}
		if st.Loop.InitStmt == nil || st.Loop.TestExpr == nil || st.Loop.IncrStmt == nil {
			return false
		}
		return st.Then != nil
	case StmtArrayOp:
		// StatementArrayOp.cpp make_random_array_init: LoopControl with numeric
		// init/limit/incr + IV (no InitStmt/TestExpr/IncrStmt — those are For-only).
		// Nested multi-dim wraps Loop + Then; body always present.
		// Fair: rejecting array-init as !stmtOK re-picks the statement kind and
		// shifts the whole block (seed-2 e33136: UP keeps ArrayOp then later
		// append_return Select U1 vs Go soft-fail → For/Goto F40).
		if st.Then == nil {
			return false
		}
		if st.Loop != nil {
			return st.Loop.IV != nil
		}
		// zero-dim / degenerate still needs access or body (no invent Kind-only)
		return st.ArrayAccess != ""
	case StmtGoto:
		// StatementGoto always has live test + label
		return st.Label != "" && st.Expr != nil
	case StmtReturn:
		return st.Expr != nil
	case StmtIfElse:
		// StatementIf always has test + both arms
		return st.Expr != nil && st.Then != nil && st.Else != nil
	case StmtBlock:
		// nested Block::make_random requires live Then body
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

// outputStmtsOnly emits Statement list at indent levels (Block.cpp OutputStatementList).
// indent is statement base indent (spaces/4); uses Emit* flags on b.
func (b *Block) outputStmtsOnly(indent int) string {
	return b.outputStmtsOnlyOpts(indent, false, ProcessOptions())
}

// outputStmtsOnlyOpts is outputStmtsOnly with optional PreOutput skip.
// skipPre: multi-dim StatementArrayOp nests Output-only shells that share one
// C++ stm_id (MakeRandomArrayInit). C++ Statement::pre_output runs once per
// Statement; re-running PreOutput on nested shells re-emits the same lbl_N
// (seed 86: UP one lbl_1132 vs GO three inside nested fors). Nested shells
// still emit for-headers/body; only pre_output is suppressed.
func (b *Block) outputStmtsOnlyOpts(indent int, skipPre bool, opts Options) string {
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	inner := strings.Repeat("    ", indent)
	var sb strings.Builder
	for _, st := range b.Stmts {
		// Statement::pre_output — label from jump sources / SourceLabel, else step_hash
		// Statement.cpp:905–917 — goto target skips output_hash
		var pre string
		var isGotoTarget bool
		if !skipPre {
			pre, isGotoTarget = PreOutput(&st, b.EmitFM, b.EmitStepHash, b.EmitLabelAttrs, b.LabelAttrRng, inner)
			// residual ERROR sticky — no invent soft-continue stmt emit past PreOutput hole
			if sessHasError(nil) {
				return ""
			}
		}
		if pre != "" {
			sb.WriteString(pre)
		}
		// Statement.cpp:911–913 — output_skipped_var_inits after label is commented out upstream
		_ = isGotoTarget
		if st.Kind == StmtLabel {
			// Statement label is empty statement after pre_output label:
			// only emit ";" when a label was actually written (goto target)
			// no invent bare ";" without label
			if pre != "" {
				sb.WriteString(inner + "    ;\n")
			}
			continue
		}
		// build statement body first — no invent indent-only lines for incomplete IR
		var content strings.Builder
		switch st.Kind {
		case StmtReturn:
			// StatementReturn.cpp:125–134 — always ExpressionVariable var (no invent bare return;)
			// incomplete sticky fails whole block (no invent soft-skip stmt and still emit later)
			if st.Expr == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			exprOut := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if sessHasError(nil) {
				return ""
			}
			if exprOut == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			// StatementReturn.cpp:127–129 — DEPTH-- when CGOptions::depth_protect()
			if opts.DepthProtect {
				content.WriteString("DEPTH--;\n")
				content.WriteString(inner)
			}
			// StatementReturn.cpp:131–133 — "return " + var.Output + ";"
			content.WriteString("return " + exprOut + ";\n")
		case StmtAssign:
			// StatementArrayOp init body: aggregate constant needs tmp
			// StatementArrayOp.cpp:237–248
			if st.ArrayAccess != "" && st.Expr != nil &&
				st.Expr.Term == TermConstant && st.LhsVar != nil &&
				st.LhsVar.Type != nil && st.LhsVar.Type.IsAggregate() {
				ty := st.LhsVar.Type.CName()
				// residual ERROR sticky — no invent soft-continue past CName residual
				if sessHasError(nil) {
					return ""
				}
				rhs := st.Expr.Output()
				// residual ERROR sticky — no invent soft-continue past Output residual
				if sessHasError(nil) {
					return ""
				}
				if ty == "" || rhs == "" {
					sessNoteError(nil, ErrGeneric)
					return ""
				}
				content.WriteString(ty + " tmp = " + rhs + ";\n")
				content.WriteString(inner + st.ArrayAccess + " = tmp;\n")
				break
			}
			// StatementAssign::OutputAsExpr — CGOptions::identify_wrappers process-wide
			wrap := st.LhsVar != nil && st.LhsVar.UseVolRVal
			// no soft invent Defaults() / force IdentifyWrappers=false
			asExpr := OutputAssignAsExprOpts(&st, wrap, opts)
			// residual ERROR sticky — no invent soft-continue stmt past OutputAssign residual
			if sessHasError(nil) {
				return ""
			}
			if asExpr != "" {
				content.WriteString(asExpr + ";\n")
			} else if st.ArrayAccess != "" && st.Expr != nil {
				// array_init simple: a[i] = expr
				rhs := st.Expr.Output()
				// residual ERROR sticky — no invent soft-continue stmt past Output residual
				if sessHasError(nil) {
					return ""
				}
				if rhs == "" {
					sessNoteError(nil, ErrGeneric)
					return ""
				}
				content.WriteString(st.ArrayAccess + " = " + rhs + ";\n")
			} else {
				// incomplete assign IR sticky — fail whole block (no invent soft-skip)
				sessNoteError(nil, ErrGeneric)
				return ""
			}
		case StmtBreak:
			// StatementBreak.cpp:117–118 — test.Output always live; sticky no invent if () break
			if st.Expr == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if sessHasError(nil) {
				return ""
			}
			if test == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    break;\n")
		case StmtContinue:
			// StatementContinue.cpp — test.Output always live; sticky no invent if () continue
			if st.Expr == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if sessHasError(nil) {
				return ""
			}
			if test == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    continue;\n")
		case StmtFor:
			// StatementFor.cpp:422–424 — output_header(indent); body.Output(indent)
			// same indent as for (not indent+1). sticky no invent for(;;) / missing body
			if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			hdr := forHeaderOutput(st.Loop)
			// residual ERROR sticky — no invent soft-continue body past header residual
			if sessHasError(nil) {
				return ""
			}
			// body pad matches statement indent; outer sb prefixes first line with inner
			bodyOut := st.Then.OutputOpts(indent, opts)
			// residual ERROR sticky — no invent soft-continue stmt past body residual
			if sessHasError(nil) {
				return ""
			}
			if hdr == "" || bodyOut == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString(hdr + "\n")
			content.WriteString(bodyOut)
		case StmtIfElse:
			// StatementIf.cpp:139–159 — if_true/if_false.Output(indent) same as condition
			// sticky no invent if () / missing branches / empty test or branch Output
			if st.Expr == nil || st.Then == nil || st.Else == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue arms past test residual
			if sessHasError(nil) {
				return ""
			}
			thenOut := st.Then.OutputOpts(indent, opts)
			// residual ERROR sticky — no invent soft-continue else past Then residual
			if sessHasError(nil) {
				return ""
			}
			elseOut := st.Else.OutputOpts(indent, opts)
			// residual ERROR sticky — no invent soft-continue stmt past Else residual
			if sessHasError(nil) {
				return ""
			}
			if test == "" || thenOut == "" || elseOut == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(thenOut)
			content.WriteString(inner + "else\n")
			content.WriteString(elseOut)
		case StmtGoto:
			// StatementGoto.cpp:252–253 — test.Output always live; sticky no invent if () goto
			if st.Label == "" || st.Expr == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if sessHasError(nil) {
				return ""
			}
			if test == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    goto " + st.Label + ";\n")
		case StmtArrayOp:
			// StatementArrayOp.cpp:225–267 — header; body Block OR bare-brace init_value
			// sticky no invent header without body/init
			if st.Loop == nil || st.Loop.IV == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			hdr := arrayOpHeaderOutput(st.Loop, opts)
			// residual ERROR sticky — no invent soft-continue body past header residual
			if sessHasError(nil) {
				return ""
			}
			if hdr == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString(hdr + "\n")
			// StatementArrayOp.cpp:231–258 — init_value path: bare "{" (no block id)
			// ArrayAccess set on make_random_array_init; C++ body==null.
			if st.ArrayAccess != "" && st.Expr != nil {
				// pad matches for indent (content gets outer inner on first line only)
				pad := strings.Repeat("    ", indent)
				content.WriteString(pad + "{\n")
				// StatementArrayOp.cpp:237–254 — aggregate constant → tmp; else direct
				assignPad := strings.Repeat("    ", indent+1)
				if st.Expr.Term == TermConstant && st.LhsVar != nil && st.LhsVar.Type != nil && st.LhsVar.Type.IsAggregate() {
					ty := st.LhsVar.Type.CName()
					rhs := st.Expr.Output()
					if sessHasError(nil) {
						return ""
					}
					if ty == "" || rhs == "" {
						sessNoteError(nil, ErrGeneric)
						return ""
					}
					content.WriteString(assignPad + ty + " tmp = " + rhs + ";\n")
					content.WriteString(assignPad + st.ArrayAccess + " = tmp;\n")
				} else {
					rhs := st.Expr.Output()
					if sessHasError(nil) {
						return ""
					}
					if rhs == "" {
						sessNoteError(nil, ErrGeneric)
						return ""
					}
					content.WriteString(assignPad + st.ArrayAccess + " = " + rhs + ";\n")
				}
				content.WriteString(pad + "}\n")
				break
			}
			// StatementArrayOp.cpp:229–230 — body->Output(indent) when body non-null
			if st.Then == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			// Nested multi-dim may wrap child StmtArrayOp in a synthetic Block.
			// Intermediate bare "{" between dims: StatementArrayOp.cpp:198–200.
			if len(st.Then.Stmts) == 1 && st.Then.Stmts[0].Kind == StmtArrayOp {
				// emit child at same indent under bare brace like C++ header nest
				pad := strings.Repeat("    ", indent)
				content.WriteString(pad + "{\n")
				// child's for is at indent+1; use output of one statement via temp block
				// skipPre: nested shells share outermost stm_id — one pre_output only
				// (Statement.cpp:905–917 once per StatementArrayOp object).
				child := st.Then.Stmts[0]
				nest := &Block{Stmts: []Stmt{child}, EmitFM: b.EmitFM, EmitStepHash: b.EmitStepHash,
					EmitLabelAttrs: b.EmitLabelAttrs, LabelAttrRng: b.LabelAttrRng,
					EmitParanoid: b.EmitParanoid, EmitConcise: b.EmitConcise}
				childOut := nest.outputStmtsOnlyOpts(indent+1, true, opts)
				if sessHasError(nil) {
					return ""
				}
				if childOut == "" {
					sessNoteError(nil, ErrGeneric)
					return ""
				}
				content.WriteString(childOut)
				content.WriteString(pad + "}\n")
				break
			}
			bodyOut := st.Then.OutputOpts(indent, opts)
			// residual ERROR sticky — no invent soft-continue stmt past body residual
			if sessHasError(nil) {
				return ""
			}
			if bodyOut == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString(bodyOut)
		case StmtInvoke:
			// StatementExpr::Output — expr.Output(); ";"
			// incomplete sticky fails whole block (no invent soft-skip empty invoke)
			if st.Expr == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			out := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if sessHasError(nil) {
				return ""
			}
			if out == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString(out + ";\n")
		case StmtBlock:
			// Block is Statement; OutputStatementList calls Block::Output at same indent
			// sticky no invent empty nested shell
			if st.Then == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			bodyOut := st.Then.OutputOpts(indent, opts)
			// residual ERROR sticky — no invent soft-continue stmt past nested residual
			if sessHasError(nil) {
				return ""
			}
			if bodyOut == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			content.WriteString(bodyOut)
		default:
			// unknown/zero Kind in live body is incomplete IR sticky — fail whole block
			// (no invent soft-skip hole and still emit later stmts)
			// StmtLabel handled earlier via continue
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		if content.Len() > 0 {
			sb.WriteString(inner)
			sb.WriteString(content.String())
		}
		// Statement::post_output — paranoid fact assertions (Statement.cpp:919–924)
		if b.EmitParanoid && b.EmitFM != nil {
			post := PostOutput(&st, b, b.EmitFM, true, b.EmitConcise, inner)
			// residual ERROR sticky — no invent soft-continue stmt emit past PostOutput hole
			if sessHasError(nil) {
				return ""
			}
			sb.WriteString(post)
		}
	}
	return sb.String()
}

// Output emits C for the block with indent levels.
func (b *Block) Output(indent int) string {
	return b.OutputOpts(indent, ProcessOptions())
}

// OutputOpts is Block.Output with explicit session Options (no ambient ProcessOptions).
func (b *Block) OutputOpts(indent int, opts Options) string {
	// Block.cpp:248+ — always live this; sticky no invent empty "{}" shell for nil
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
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
	// Block.cpp:255–257 — CGOptions::depth_protect(), not Block::depth_protect flag.
	// Function sets body->set_depth_protect(true) always; emit still gates on CGOptions.
	if opts.DepthProtect {
		sb.WriteString(inner + "DEPTH++;\n")
	}
	// Block.cpp:261–262 — OutputTmpVariableList only when CGOptions::math_notmp().
	// Tmps are still created during generation (gensym side-effect) either way.
	if opts.MathNoTmp && len(b.TmpVars) > 0 {
		names := make([]string, 0, len(b.TmpVars))
		for name := range b.TmpVars {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			// macro_tmp_vars name + type always live; sticky no invent "int  = 0;" / skip holes
			if name == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			// eSimpleType always valid in macro_tmp_vars; OOB/invalid sticky fail closed
			// (GetSimpleType nil — no invent "int" for broken tmp type)
			ty := GetSimpleType(b.TmpVars[name])
			if ty == nil {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			cn := ty.CName()
			// residual ERROR sticky — no invent soft-continue tmp decl past CName residual
			if sessHasError(nil) {
				return ""
			}
			if cn == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			sb.WriteString(inner)
			sb.WriteString(cn + " " + name + " = 0;\n")
		}
	}
	// OutputVariableList(local_vars) — Variable.cpp:855–864
	// Incomplete LocalVars fails closed sticky whole block (no invent soft-skip hole partial)
	if !VariablesComplete(b.LocalVars) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Block.cpp:268 — OutputVariableList(local_vars): defs then OutputArrayInitializers
	// for non-global lists (Variable.cpp:861–863). Do not invent a loopInits-only gate:
	// C++ still emits "int i, j, k;" when every array is brace-init (seed-2 func_67).
	if len(b.LocalVars) > 0 {
		listOut := OutputVariableListOpts(b.LocalVars, inner, false, opts)
		// residual ERROR sticky — no invent soft-continue stmts past OutputVariableList residual
		if sessHasError(nil) {
			return ""
		}
		sb.WriteString(listOut)
	}
	// Block.cpp:235–241 OutputStatementList
	// Only fail closed on residuals raised during stmt emit (not pre-existing sticky).
	hadErr := sessHasError(nil)
	stmtsOut := b.outputStmtsOnlyOpts(indent+1, false, opts)
	if stmtsOut == "" && sessHasError(nil) && !hadErr {
		// residual during stmt list — no invent braces-only success past hole
		return ""
	}
	sb.WriteString(stmtsOut)
	// Block.cpp:266–267 — CGOptions::depth_protect() (not body depth_protect flag)
	if opts.DepthProtect {
		sb.WriteString(inner + "DEPTH--;\n")
	}
	sb.WriteString(pad + "}\n")
	return sb.String()
}
