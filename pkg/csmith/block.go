// Upstream: Block.h / Block.cpp (BlockProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"os"
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

// nextStmID is Statement::sid allocator.
// Note: C++ Statement.cpp:366–367 assigns then increments (first id 0). Go keeps
// pre-increment first id 1 because StmID 0 is the incomplete-IR sentinel across
// visit/eligible paths; block-id emit is thus +1 vs C++ until that sentinel is
// split from valid id 0 (fair follow-up).
var nextStmID int

// AllocStmID allocates a live statement id (never 0 — reserved incomplete).
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

// BlockSize mirrors Block::block_size.
// Block.h:85 — CGOptions::max_block_size captured at construction.
func (b *Block) BlockSize() int {
	if b == nil {
		SetError(ErrGeneric)
		return 0
	}
	return b.blockSize
}

// GetDepthProtect mirrors Block::get_depth_protect.
// Block.h:76.
func (b *Block) GetDepthProtect() bool {
	if b == nil {
		SetError(ErrGeneric)
		return false
	}
	return b.EmitDepthProtect
}

// SetDepthProtect mirrors Block::set_depth_protect — returns new value.
// Block.h:72–74.
func (b *Block) SetDepthProtect(v bool) bool {
	if b == nil {
		SetError(ErrGeneric)
		return false
	}
	b.EmitDepthProtect = v
	return v
}

// PushStmt mirrors stms.push_back for a complete Statement.
// Incomplete Stmt Kind sticky (no invent append hole).
func (b *Block) PushStmt(st Stmt) {
	if b == nil {
		SetError(ErrGeneric)
		return
	}
	b.Stmts = append(b.Stmts, st)
}

// FindBlockByID mirrors find_block_by_id.
// Block.cpp:69–83 — scan non-builtin Function::blocks for stm_id.
// Incomplete funcs sticky nil.
func FindBlockByID(funcs []*Function, blkID int) *Block {
	if !FunctionsComplete(funcs) {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete id sticky (no invent match-first soft-pick)
	if blkID <= 0 {
		SetError(ErrGeneric)
		return nil
	}
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		for _, b := range f.Blocks {
			if b == nil {
				SetError(ErrGeneric)
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
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	if !b.Looping || len(b.Stmts) == 0 {
		return false
	}
	s := b.GetLastStm()
	if s == nil {
		// incomplete last stmt sticky no fall-through
		SetError(ErrGeneric)
		return false
	}
	// residual ERROR sticky — no invent soft-continue fall-through past GetLastStm residual
	if HasError() {
		return false
	}
	if s.MustJump() {
		// residual ERROR sticky — no invent no-fall-through true past MustJump residual hole
		if HasError() {
			return false
		}
		return false
	}
	// residual ERROR sticky — no invent fall-through true past MustJump residual false path
	if HasError() {
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
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	// Block::stm_id always live; StmID 0 fails closed sticky incomplete (no invent
	// empty-complete accum return without map_stm_effect[block] recorded)
	if b.StmID <= 0 {
		SetError(ErrGeneric)
		return IncompleteEffect()
	}
	eff := EmptyEffect()
	for i := range b.Stmts {
		st := &b.Stmts[i]
		if st.StmID <= 0 {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			SetError(ErrGeneric)
			return inc
		}
		// map_stm_effect[] defaults empty Effect in C++; incomplete map keys fail closed sticky
		se := fm.GetMapStmEffect(st.StmID)
		if !EffectComplete(se) {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			SetError(ErrGeneric)
			return inc
		}
		eff = eff.AddEffect(se)
		if !EffectComplete(eff) {
			inc := IncompleteEffect()
			fm.SetMapStmEffect(b.StmID, inc)
			if !HasError() {
				SetError(ErrGeneric)
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
		SetError(ErrGeneric)
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
	if HasError() {
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
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return false
	}
	if !b.StackScanComplete() {
		// incomplete Param/LocalVars sticky fail closed not-on-stack
		// residual ERROR sticky — no invent soft not-on-stack past StackScan residual
		if !HasError() {
			SetError(ErrGeneric)
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
	}
	for bb := b; bb != nil; bb = bb.Parent {
		for _, loc := range bb.LocalVars {
			if loc == nil {
				SetError(ErrGeneric)
				return false
			}
			if loc == v {
				return true
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

// CreateNewTmpVar mirrors Block::create_new_tmp_var.
// Block.cpp:216–219 — always gensym("t_") (util.cpp process-wide gensym_count);
// no invent VS.Sym private counter (that desynced t_ from g_/l_/func_).
// sym is ignored; kept for call-site compatibility.
func (b *Block) CreateNewTmpVar(sym *GenSym, st ESimpleType) string {
	_ = sym
	// Block.cpp:216–219 — this always live; gensym + macro_tmp_vars insert together
	// sticky no invent bare t_N without block registration (would emit undeclared use)
	if b == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Block.cpp:217 — const string var_name = gensym("t_"); sticky no invent bare ""
	name := Gensym("t_")
	if name == "" {
		if !HasError() {
			SetError(ErrGeneric)
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
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return nil
	}
	// Block.cpp:120 — assert(curr_func) sticky; no soft invent parentless block
	f := cg.CurrentFunc
	if f == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky before stack push (no invent block past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
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
	if os.Getenv("CSMITH_DUMP_STACK") != "" {
		fmt.Fprintf(os.Stderr, "GO_BPUSH id=%d n=%d loop=%v arr=%v depth=%d\n", b.StmID, len(f.Stack), looping, b.InArrayLoop, cg.BlkDepth)
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
	// Incomplete accum/facts fail closed (no invent post_creation / return block past holes)
	preEffect := EmptyEffect()
	if cg.EffectAccum != nil {
		if !EffectComplete(*cg.EffectAccum) {
			abortBlockMake(f, b)
			SetError(ErrGeneric)
			return nil
		}
		preEffect = cg.EffectAccum.Clone()
		// residual ERROR sticky — no invent soft-block past Effect Clone residual
		if HasError() {
			abortBlockMake(f, b)
			return nil
		}
	}
	if !EffectComplete(preEffect) {
		abortBlockMake(f, b)
		SetError(ErrGeneric)
		return nil
	}
	// StmID always allocated at make; FM path always records map_facts_in
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			abortBlockMake(f, b)
			SetError(ErrGeneric)
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
		must := st.MustReturn()
		// residual ERROR sticky — no invent soft-continue more stmts past MustReturn residual
		if HasError() {
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
	if HasError() {
		abortBlockMake(f, b)
		return nil
	}
	// Block.cpp:164–166 — nested loop for must-use multi-dim arrays
	if b.NeedNestedLoop(*cg, r) && cg.BlkDepth < opts.MaxBlockDepth {
		b.AppendNestedLoop(r, opts, probs, vs, tables, stmtTab, cg)
		// append_nested_loop ERROR_GUARD(nullptr) on for make fail
		if HasError() {
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
		if HasError() {
			abortBlockMake(f, b)
			return nil
		}
		if !must {
			ret := MakeRandomReturn(r, opts, vs, cg)
			if stmtOK(ret) {
				if ret.StmID == 0 {
					ret.StmID = AllocStmID()
				}
				b.Stmts = append(b.Stmts, ret)
			}
		}
	}
	if b.StmID == 0 {
		b.StmID = AllocStmID()
	}
	b.PostCreationAnalysis(cg, opts, preEffect, r, vs)
	// incomplete post-creation GlobalFacts fail closed even without sticky ERROR
	// (no invent return live block past IncompleteFactSlice wipe)
	if HasError() || (cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts)) {
		// Block.cpp:170–174 — ERROR after post_creation → delete
		if !HasError() {
			SetError(ErrGeneric)
		}
		abortBlockMake(f, b)
		return nil
	}
	// Block.cpp:178 — stack.pop_back(); identity-safe like abortBlockMake.
	if f != nil {
		if n := len(f.Stack); n > 0 && f.Stack[n-1] == b {
			f.Stack = f.Stack[:n-1]
		}
	}
	// Block.cpp:187 — Error::set_error(SUCCESS)
	ClearError()
	return b
}

// abortBlockMake pops stack after a failed Block::make_random.
// Block.cpp:142–174 — on ERROR: stack.pop_back(); delete b; return nullptr.
// C++ does NOT erase from func->blocks (only remove_stmt does Block.cpp:653–660).
// Leaving the entry matches StatementGoto::make_random's vector copy of func->blocks
// (seed-2 first_div e12688: invent erase → n=11 vs upstream n=14).
// Function + Block always live on make abort; sticky (no invent soft-skip cleanup past hole).
func abortBlockMake(f *Function, b *Block) {
	if f == nil || b == nil {
		SetError(ErrGeneric)
		return
	}
	if n := len(f.Stack); n > 0 && f.Stack[n-1] == b {
		f.Stack = f.Stack[:n-1]
	}
	// no invent f.Blocks erase — C++ delete leaves the pointer in func->blocks
}

// PostCreationAnalysis mirrors Block::post_creation_analysis.
// Block.cpp:682–742 — effects, OOS, optional fixed-point with remove_stmt, append_return.
// Incomplete preEffect / StmID 0 fails closed sticky (no invent fixed-point / map
// record / soft-reset EffectAccum from IncompleteEffect shell).
// Block + CGContext always live; sticky (no invent soft-skip past hole).
// Nil FM is non-sticky soft re-pick (sticky poisons soft factories without FM).
func (b *Block) PostCreationAnalysis(cg *CGContext, opts Options, preEffect Effect, r *Rng, vs *VariableSelector) {
	if b == nil || cg == nil {
		SetError(ErrGeneric)
		return
	}
	fm := cg.FM
	if fm == nil {
		return
	}
	if b.StmID <= 0 {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	if !EffectComplete(preEffect) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
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
		SetError(ErrGeneric)
		return
	}
	// incomplete GlobalFacts fail closed sticky (no invent cleaned postFacts / OOS from holes)
	// Use IncompleteFactSlice — bare nil invents empty success via FactsComplete(nil)
	var postFacts []*FactPointTo
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		postFacts = IncompleteFactSlice()
		fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
		SetError(ErrGeneric)
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
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		outPost := CloneFactSlice(fm.GlobalFacts)
		if HasError() || !FactsComplete(outPost) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		if len(b.LocalVars) > 0 {
			UpdateFactsForOOSVars(b.LocalVars, &outPost)
			if !FactsComplete(outPost) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				return
			}
		}
		fm.RemoveRVFacts(&outPost)
		if !FactsComplete(outPost) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		fm.SetMapFactsOut(b.StmID, outPost)

		// Block.cpp:696–697 — fixed-point when:
		//   is_loop_body || need_revisit || has_edge_in(false, true)
		// has_edge_in: Statement.cpp:434–446 — e->dest == this (the block statement).
		// Do not invent ContainsBackEdge (dest->parent==this) or FindEdgesInToBlock:
		// those force FP on blocks C++ leaves with mid-gen global_facts (seed-2 e10107
		// wipe via auto_block_959 after unnecessary FP; e12688 over-strip).
		mustBR := b.MustBreakOrReturnFull(fm)
		// residual ERROR sticky — no invent soft-fixed-point past MustBreakOrReturn residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			postFacts = IncompleteFactSlice()
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			return
		}
		isLoopBody := !mustBR && b.Looping
		hasBack := fm.HasEdgeIn(b.StmID, false, true)
		// residual ERROR sticky — HasEdgeIn sets sticky on incomplete CFG
		if HasError() {
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
				if HasError() {
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
				SetError(ErrGeneric)
				return
			} else {
				factsCopy := CloneFactSlice(in0)
				// residual ERROR sticky — no invent soft-fixed-point past CloneFactSlice residual
				if HasError() {
					fm.GlobalFacts = IncompleteFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					return
				}
				// reset accum to pre-block effect
				if cg.EffectAccum != nil {
					*cg.EffectAccum = preEffect.Clone()
					// residual ERROR sticky — no invent soft-reset past Effect Clone residual
					if HasError() {
						fm.GlobalFacts = IncompleteFactSlice()
						postFacts = IncompleteFactSlice()
						fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
						return
					}
				}
				for {
					fpOut, failIdx, ok := FindFixedPointBlock(b, factsCopy, cg, opts, b.NeedRevisit)
					if ok {
						// Block.cpp:706–728 + find_fixed_point Block.cpp:558 —
						// full visit assigns post_facts = pre-OOS outputs; pure
						// shortcut leaves post_facts (line-690 snapshot) unchanged.
						// FindFixedPointBlock returns nil on pure shortcut.
						if fpOut != nil {
							postFacts = fpOut
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
						if HasError() {
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
						if HasError() {
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
						fpEmpty, _, okEmpty := FindFixedPointBlock(b, factsCopy, cg, opts, true)
						if okEmpty {
							if fpEmpty != nil {
								postFacts = fpEmpty
							}
						} else {
							// install empty-body maps from entry facts (C++ find_fixed_point)
							if !FactsComplete(factsCopy) {
								fm.GlobalFacts = IncompleteFactSlice()
								postFacts = IncompleteFactSlice()
								fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
								SetError(ErrGeneric)
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
									SetError(ErrGeneric)
									return
								}
								AddNewVarFactTo(v, &preOOS)
								if !FactsComplete(preOOS) {
									fm.GlobalFacts = IncompleteFactSlice()
									postFacts = IncompleteFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									SetError(ErrGeneric)
									return
								}
							}
							postFacts = preOOS
							outCopy := CloneFactSlice(preOOS)
							if len(b.LocalVars) > 0 {
								tmp := outCopy
								saved := fm.GlobalFacts
								fm.SetGlobalFacts(tmp, "auto_block_931")
								fm.UpdateFactsForOOSVars(b.LocalVars)
								outCopy = fm.GlobalFacts
								fm.SetGlobalFacts(saved, "auto_block_934")
								if !FactsComplete(outCopy) {
									fm.GlobalFacts = IncompleteFactSlice()
									postFacts = IncompleteFactSlice()
									fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
									SetError(ErrGeneric)
									return
								}
							}
							fm.SetMapFactsOut(b.StmID, outCopy)
						}
						break
					}
				}
				// Block.cpp:729 — global_facts = map_facts_out[this]
				// post_facts already set by find_fixed_point (pre-OOS) or line-690
				// incomplete out fails closed (hole marker — no invent keep prior / empty)
				out := fm.GetMapFactsOut(b.StmID)
				if !FactsComplete(out) {
					fm.GlobalFacts = IncompleteFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					SetError(ErrGeneric)
					return
				} else {
					fm.SetGlobalFacts(CloneFactSlice(out), "auto_block_959")
					// residual ERROR sticky — no invent soft-out past CloneFactSlice residual
					if HasError() {
						fm.GlobalFacts = IncompleteFactSlice()
						postFacts = IncompleteFactSlice()
						fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
						return
					}
				}
			}
		} else {
			// No FP: C++ leaves global_facts post-OOS (Block.cpp:690–693 only).
			if !FactsComplete(outPost) {
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				SetError(ErrGeneric)
				return
			}
			fm.SetGlobalFacts(CloneFactSlice(outPost), "auto_block_oos_no_fp")
			if HasError() {
				fm.GlobalFacts = IncompleteFactSlice()
				postFacts = IncompleteFactSlice()
				fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
				return
			}
			if b.Looping {
				fromTail := b.FromTailToHead()
				// residual ERROR sticky — no invent soft-self-back past FromTailToHead residual
				if HasError() {
					fm.GlobalFacts = IncompleteFactSlice()
					postFacts = IncompleteFactSlice()
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					return
				}
				if fromTail {
					fm.CreateCFGEdge(b.StmID, b, false, true)
				}
			}
		}
	}
	// Block.cpp:687 already set map_visited; find_fixed_point also sets (561). Reaffirm.
	fm.MapVisited[b.StmID] = true
	// Block.cpp:734–741 — append return for top-level body when still missing
	// incomplete postFacts must not invent return gen via FactsComplete(nil) empty
	if b.Parent == nil && b.Func != nil && b.Func.NeedReturnStmt() {
		must := b.MustReturn()
		// residual ERROR sticky — no invent soft-append return past MustReturn residual
		if HasError() {
			fm.GlobalFacts = IncompleteFactSlice()
			if !FactsComplete(postFacts) {
				postFacts = IncompleteFactSlice()
			}
			return
		}
		if !must {
			if !FactsComplete(postFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			fm.SetGlobalFacts(postFacts, "auto_block_1002")
			if b.AppendReturnStmt(r, opts, vs, cg) == nil {
				// append_return_stmt ERROR_GUARD / assert(visited) leave sticky error
				return
			}
			// Block.cpp:740 — set_fact_out(this, map_facts_out[sr])
			// C++ map[] always reads sr out (missing → empty); no invent skip set_fact_out
			if len(b.Stmts) > 0 {
				sr := &b.Stmts[len(b.Stmts)-1]
				// return stm_id always live after append_return; StmID 0 → Incomplete via getter
				out := fm.GetMapFactsOut(sr.StmID)
				if FactsComplete(out) {
					fm.SetMapFactsOut(b.StmID, out)
				} else {
					// incomplete sr out — fail closed sticky hole marker (not empty complete)
					fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
					SetError(ErrGeneric)
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
	// Statement.cpp always has RNG + CGContext; sticky no invent MAX-kind shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return Stmt{}
	}
	// incomplete ambient EffectContext fails closed sticky before re-pick loop
	// (no invent stmt under incomplete context shells; EffectStm is cleared per try)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return Stmt{}
	}
	// Statement.cpp:243–244 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtStatement, t, nullptr)
	// t is MAX_STATEMENT_TYPE when choosing randomly (flag = MaxStatementType).
	if DepthGuardByTypeFlag(opts, DtStatement, int(MaxStatementType)) == BadDepth {
		return Stmt{}
	}
	// Statement static ProbabilityTable always live; sticky no invent NewStatementThresholdTable
	if stmtTab == nil {
		stmtTab = ProcessStmtTab()
	}
	if stmtTab == nil {
		SetError(ErrGeneric)
		return Stmt{}
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
		if k == StmtReturn && cg.CurrentFunc != nil && cg.CurrentFunc.ReturnType != nil {
			isSimple := cg.CurrentFunc.ReturnType.IsSimple()
			// residual ERROR sticky — no invent filter keep/reject past IsSimple residual
			if HasError() {
				return true
			}
			if isSimple {
				st := cg.CurrentFunc.ReturnType.Simple()
				// residual ERROR sticky — no invent filter keep/reject past Simple residual
				if HasError() {
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
		kind := StatementProbabilityFilter(r, stmtTab, f)
		// Statement.cpp:248–250 — stop_by_stmt forces return after sid threshold
		if opts.StopByStmt >= 0 && nextStmID >= opts.StopByStmt {
			kind = StmtReturn
		}
		// Statement.cpp:260–261 — pre_facts / pre_effect (accum) snapshot before make
		// C++: FactVec pre_facts = fm->global_facts; shallow copy of Fact* vector.
		// Nested ExpressionAssign merges replace pointers in global_facts only;
		// pre_facts keeps the pre-make Fact* set for set_fact_in. Deep CloneFactSlice
		// isolated pointees incorrectly vs that sharing model (seed-2 e10107 may-null).
		// incomplete GlobalFacts/accum fail closed sticky (no invent cleaned pre-stmt snapshot
		// or soft re-pick past holes)
		var preFacts []*FactPointTo
		if cg.FM != nil {
			if !FactsComplete(cg.FM.GlobalFacts) {
				SetError(ErrGeneric)
				return Stmt{}
			}
			preFacts = append([]*FactPointTo(nil), cg.FM.GlobalFacts...)
		}
		preEffect := EmptyEffect()
		if cg.EffectAccum != nil {
			if !EffectComplete(*cg.EffectAccum) {
				SetError(ErrGeneric)
				return Stmt{}
			}
			preEffect = cg.EffectAccum.Clone()
			// residual ERROR sticky — no invent soft-stmt past Effect Clone residual
			if HasError() {
				return Stmt{}
			}
		}
		if !EffectComplete(preEffect) {
			SetError(ErrGeneric)
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
		if HasError() {
			return Stmt{}
		}
		if stmtOK(st) {
			// Statement.cpp:320 — post_creation_analysis(pre_facts, pre_effect)
			// incomplete post-creation must not invent stmt success past wiped facts
			PostCreationAnalysis(&st, preFacts, preEffect, cg, opts)
			if HasError() || (cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts)) {
				if !HasError() {
					SetError(ErrGeneric)
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
		SetError(ErrGeneric)
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
		if HasError() {
			return Stmt{}
		}
		return st
	case StmtBreak:
		st := MakeRandomBreak(r, opts, vs, tables, cg)
		// residual ERROR sticky — no invent soft-return break past MakeRandomBreak residual
		if HasError() {
			return Stmt{}
		}
		return st
	case StmtContinue:
		st := MakeRandomContinue(r, opts, vs, tables, cg, b)
		// residual ERROR sticky — no invent soft-return continue past MakeRandomContinue residual
		if HasError() {
			return Stmt{}
		}
		return st
	case StmtIfElse:
		if st := MakeRandomIf(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			// residual ERROR sticky — no invent soft-return if past MakeRandomIf residual
			if HasError() {
				return Stmt{}
			}
			return *st
		}
		// residual ERROR sticky — no invent soft re-pick past MakeRandomIf residual nil
		if HasError() {
			return Stmt{}
		}
		// null factory → re-pick (Statement.cpp:314); incomplete shell fails stmtOK
		return Stmt{}
	case StmtFor:
		if st := MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg); st != nil {
			// residual ERROR sticky — no invent soft-return for past MakeRandomFor residual
			if HasError() {
				return Stmt{}
			}
			return *st
		}
		// residual ERROR sticky — no invent soft re-pick past MakeRandomFor residual nil
		if HasError() {
			return Stmt{}
		}
		return Stmt{}
	case StmtArrayOp:
		st := MakeRandomArrayOp(r, opts, probs, vs, tables, stmtTab, cg)
		// residual ERROR sticky — no invent soft-return array-op past MakeRandomArrayOp residual
		if HasError() {
			return Stmt{}
		}
		return st
	case StmtGoto:
		st := MakeRandomGoto(r, opts, probs, vs, tables, cg, b)
		// residual ERROR sticky — no invent soft-return goto past MakeRandomGoto residual
		if HasError() {
			return Stmt{}
		}
		return st
	case StmtInvoke:
		st := MakeRandomExprStmt(r, opts, probs, vs, tables, cg)
		// residual ERROR sticky — no invent soft-return invoke past MakeRandomExprStmt residual
		if HasError() {
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
		SetError(ErrGeneric)
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
	case StmtFor, StmtArrayOp:
		// StatementFor always has init/test/incr + body (StatementFor.cpp make_random)
		// no invent OK from IV alone without test/body
		if st.Loop == nil || st.Loop.IV == nil {
			return false
		}
		if st.Loop.InitStmt == nil || st.Loop.TestExpr == nil || st.Loop.IncrStmt == nil {
			return false
		}
		return st.Then != nil
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
	if b == nil {
		SetError(ErrGeneric)
		return ""
	}
	inner := strings.Repeat("    ", indent)
	var sb strings.Builder
	for _, st := range b.Stmts {
		// Statement::pre_output — label from jump sources / SourceLabel, else step_hash
		// Statement.cpp:905–917 — goto target skips output_hash
		pre, isGotoTarget := PreOutput(&st, b.EmitFM, b.EmitStepHash, b.EmitLabelAttrs, b.LabelAttrRng, inner)
		// residual ERROR sticky — no invent soft-continue stmt emit past PreOutput hole
		if HasError() {
			return ""
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
				SetError(ErrGeneric)
				return ""
			}
			exprOut := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if HasError() {
				return ""
			}
			if exprOut == "" {
				SetError(ErrGeneric)
				return ""
			}
			// StatementReturn.cpp:127–129 — DEPTH-- when CGOptions::depth_protect()
			if ProcessOptions().DepthProtect {
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
				if HasError() {
					return ""
				}
				rhs := st.Expr.Output()
				// residual ERROR sticky — no invent soft-continue past Output residual
				if HasError() {
					return ""
				}
				if ty == "" || rhs == "" {
					SetError(ErrGeneric)
					return ""
				}
				content.WriteString(ty + " tmp = " + rhs + ";\n")
				content.WriteString(inner + st.ArrayAccess + " = tmp;\n")
				break
			}
			// StatementAssign::OutputAsExpr — CGOptions::identify_wrappers process-wide
			wrap := st.LhsVar != nil && st.LhsVar.UseVolRVal
			// no soft invent Defaults() / force IdentifyWrappers=false
			asExpr := OutputAssignAsExprOpts(&st, wrap, ProcessOptions())
			// residual ERROR sticky — no invent soft-continue stmt past OutputAssign residual
			if HasError() {
				return ""
			}
			if asExpr != "" {
				content.WriteString(asExpr + ";\n")
			} else if st.ArrayAccess != "" && st.Expr != nil {
				// array_init simple: a[i] = expr
				rhs := st.Expr.Output()
				// residual ERROR sticky — no invent soft-continue stmt past Output residual
				if HasError() {
					return ""
				}
				if rhs == "" {
					SetError(ErrGeneric)
					return ""
				}
				content.WriteString(st.ArrayAccess + " = " + rhs + ";\n")
			} else {
				// incomplete assign IR sticky — fail whole block (no invent soft-skip)
				SetError(ErrGeneric)
				return ""
			}
		case StmtBreak:
			// StatementBreak.cpp:117–118 — test.Output always live; sticky no invent if () break
			if st.Expr == nil {
				SetError(ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if HasError() {
				return ""
			}
			if test == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    break;\n")
		case StmtContinue:
			// StatementContinue.cpp — test.Output always live; sticky no invent if () continue
			if st.Expr == nil {
				SetError(ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if HasError() {
				return ""
			}
			if test == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    continue;\n")
		case StmtFor:
			// StatementFor::Output — header + body Block always live
			// sticky no invent for(;;) / header without body / body without header
			if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
				SetError(ErrGeneric)
				return ""
			}
			hdr := forHeaderOutput(st.Loop)
			// residual ERROR sticky — no invent soft-continue body past header residual
			if HasError() {
				return ""
			}
			bodyOut := st.Then.Output(indent + 1)
			// residual ERROR sticky — no invent soft-continue stmt past body residual
			if HasError() {
				return ""
			}
			if hdr == "" || bodyOut == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString(hdr + "\n")
			content.WriteString(bodyOut)
		case StmtIfElse:
			// StatementIf.cpp:147–159 — test + if_true + else + if_false always live
			// sticky no invent if () / missing branches / empty test or branch Output
			if st.Expr == nil || st.Then == nil || st.Else == nil {
				SetError(ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue arms past test residual
			if HasError() {
				return ""
			}
			thenOut := st.Then.Output(indent + 1)
			// residual ERROR sticky — no invent soft-continue else past Then residual
			if HasError() {
				return ""
			}
			elseOut := st.Else.Output(indent + 1)
			// residual ERROR sticky — no invent soft-continue stmt past Else residual
			if HasError() {
				return ""
			}
			if test == "" || thenOut == "" || elseOut == "" {
				SetError(ErrGeneric)
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
				SetError(ErrGeneric)
				return ""
			}
			test := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if HasError() {
				return ""
			}
			if test == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString("if (")
			content.WriteString(test)
			content.WriteString(")\n")
			content.WriteString(inner + "    goto " + st.Label + ";\n")
		case StmtArrayOp:
			// StatementArrayOp::output_header + body/init block always live
			// nested dims carry Then; array-loop path reuses for body as Then
			// sticky no invent header without body
			if st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
				SetError(ErrGeneric)
				return ""
			}
			hdr := arrayOpHeaderOutput(st.Loop, ProcessOptions())
			// residual ERROR sticky — no invent soft-continue body past header residual
			if HasError() {
				return ""
			}
			bodyOut := st.Then.Output(indent + 1)
			// residual ERROR sticky — no invent soft-continue stmt past body residual
			if HasError() {
				return ""
			}
			if hdr == "" || bodyOut == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString(hdr + "\n")
			content.WriteString(bodyOut)
		case StmtInvoke:
			// StatementExpr::Output — expr.Output(); ";"
			// incomplete sticky fails whole block (no invent soft-skip empty invoke)
			if st.Expr == nil {
				SetError(ErrGeneric)
				return ""
			}
			out := st.Expr.Output()
			// residual ERROR sticky — no invent soft-continue stmt past Output residual
			if HasError() {
				return ""
			}
			if out == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString(out + ";\n")
		case StmtBlock:
			// Statement.cpp:281–282 — nested Block::Output always live sticky
			if st.Then == nil {
				SetError(ErrGeneric)
				return ""
			}
			bodyOut := st.Then.Output(indent + 1)
			// residual ERROR sticky — no invent soft-continue stmt past nested residual
			if HasError() {
				return ""
			}
			if bodyOut == "" {
				SetError(ErrGeneric)
				return ""
			}
			content.WriteString(bodyOut)
		default:
			// unknown/zero Kind in live body is incomplete IR sticky — fail whole block
			// (no invent soft-skip hole and still emit later stmts)
			// StmtLabel handled earlier via continue
			SetError(ErrGeneric)
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
			if HasError() {
				return ""
			}
			sb.WriteString(post)
		}
	}
	return sb.String()
}

// Output emits C for the block with indent levels.
func (b *Block) Output(indent int) string {
	// Block.cpp:248+ — always live this; sticky no invent empty "{}" shell for nil
	if b == nil {
		SetError(ErrGeneric)
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
	if ProcessOptions().DepthProtect {
		sb.WriteString(inner + "DEPTH++;\n")
	}
	// Block.cpp:261–262 — OutputTmpVariableList only when CGOptions::math_notmp().
	// Tmps are still created during generation (gensym side-effect) either way.
	if ProcessOptions().MathNoTmp && len(b.TmpVars) > 0 {
		names := make([]string, 0, len(b.TmpVars))
		for name := range b.TmpVars {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			// macro_tmp_vars name + type always live; sticky no invent "int  = 0;" / skip holes
			if name == "" {
				SetError(ErrGeneric)
				return ""
			}
			// eSimpleType always valid in macro_tmp_vars; OOB/invalid sticky fail closed
			// (GetSimpleType nil — no invent "int" for broken tmp type)
			ty := GetSimpleType(b.TmpVars[name])
			if ty == nil {
				SetError(ErrGeneric)
				return ""
			}
			cn := ty.CName()
			// residual ERROR sticky — no invent soft-continue tmp decl past CName residual
			if HasError() {
				return ""
			}
			if cn == "" {
				SetError(ErrGeneric)
				return ""
			}
			sb.WriteString(inner)
			sb.WriteString(cn + " " + name + " = 0;\n")
		}
	}
	// OutputVariableList(local_vars) — Variable.cpp Output
	// Incomplete LocalVars fails closed sticky whole block (no invent soft-skip hole partial)
	if !VariablesComplete(b.LocalVars) {
		SetError(ErrGeneric)
		return ""
	}
	var loopInits []*ArrayVariable
	maxDim := 0
	for _, lv := range b.LocalVars {
		if lv.Type == nil {
			SetError(ErrGeneric)
			return ""
		}
		if lv.IsArray {
			// C++ static_cast ArrayVariable* when isArray; missing AsArray is broken IR
			// sticky (no invent synthetic shell from ArraySizes past incomplete AsArray)
			if lv.AsArray == nil {
				SetError(ErrGeneric)
				return ""
			}
			av := lv.AsArray
			// ArrayVariable.cpp:493 — only collective emits def; itemized dual-count skip
			// (C++ LocalVars may hold itemize() member alongside parent)
			if av.Collective != nil {
				continue
			}
			// incomplete array def sticky — fail closed whole block
			def := av.OutputDef()
			// residual ERROR sticky — no invent soft-continue later locals past OutputDef residual
			if HasError() {
				return ""
			}
			if def == "" {
				SetError(ErrGeneric)
				return ""
			}
			sb.WriteString(inner)
			sb.WriteString(def)
			sb.WriteString("\n")
			if !av.NoLoopInitializer() {
				// residual ERROR sticky — no invent soft-continue loop-init past hole
				if HasError() {
					return ""
				}
				loopInits = append(loopInits, av)
				if len(av.Sizes) > maxDim {
					maxDim = len(av.Sizes)
				}
			} else if HasError() {
				// residual ERROR sticky — no invent soft-skip NoLoopInitializer past hole
				return ""
			}
			continue
		}
		// Variable::Output for locals (no force static)
		def := lv.OutputDef(false)
		// residual ERROR sticky — no invent soft-continue later locals past OutputDef residual
		if HasError() {
			return ""
		}
		if def == "" {
			SetError(ErrGeneric)
			return ""
		}
		sb.WriteString(inner)
		sb.WriteString(def)
		sb.WriteString("\n")
	}
	// OutputArrayInitializers for locals without brace init
	// Variable.cpp:829–841 — new_ctrl_vars + OutputArrayCtrlVars
	if len(loopInits) > 0 {
		// CGOptions::fresh_array_ctrl_var_names / max dimensions via process opts
		opts := ProcessOptions()
		ctrlVars := NewCtrlVars(maxDim, opts.FreshArrayCtrlVarNames)
		// sticky no invent inits without live ctrl decl
		decl := OutputArrayCtrlVars(ctrlVars, maxDim, inner)
		if decl == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		sb.WriteString(decl)
		ctrl := CtrlVarNames(ctrlVars)
		for _, av := range loopInits {
			initOut := av.OutputInit(inner, ctrl)
			// residual ERROR sticky — no invent soft-continue later inits past OutputInit residual
			if HasError() {
				return ""
			}
			if initOut == "" {
				// incomplete array init IR sticky — fail closed whole block
				SetError(ErrGeneric)
				return ""
			}
			sb.WriteString(initOut)
		}
	}
	// Block.cpp:235–241 OutputStatementList
	// Only fail closed on residuals raised during stmt emit (not pre-existing sticky).
	hadErr := HasError()
	stmtsOut := b.outputStmtsOnly(indent + 1)
	if stmtsOut == "" && HasError() && !hadErr {
		// residual during stmt list — no invent braces-only success past hole
		return ""
	}
	sb.WriteString(stmtsOut)
	// Block.cpp:266–267 — CGOptions::depth_protect() (not body depth_protect flag)
	if ProcessOptions().DepthProtect {
		sb.WriteString(inner + "DEPTH--;\n")
	}
	sb.WriteString(pad + "}\n")
	return sb.String()
}
