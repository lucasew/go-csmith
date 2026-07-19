// Upstream: VariableSelector.cpp / VariableSelector.h
// (GlobalList, choose_ok_var, GenerateNewGlobal name+qfer path, SelectGlobal empty→create).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VariableSelector holds AllVars / GlobalList inventories (static vectors in C++).
type VariableSelector struct {
	AllVars                []*Variable
	GlobalList             []*Variable
	GlobalNonvolatilesList []*Variable
	Sym                    GenSym
	Opts                   Options
	Probs                  *Probabilities
	// TmpCount is VariableSelector::tmp_count (incremented in GenerateNewGlobal).
	TmpCount int
	// VarCreated is VariableSelector::var_created.
	VarCreated bool
	// Types is session Type::derived_types (optional).
	Types *TypeEnv
	// Arrays tracks ArrayVariables for emission.
	Arrays []*ArrayVariable
}

// NewVariableSelector constructs an empty selector sharing process Probabilities
// (C++ Probabilities singleton). No invent second NewProbabilities(opts) when
// process unset — Probs may be nil (fail closed on draws that need tables).
func NewVariableSelector(opts Options) *VariableSelector {
	return NewVariableSelectorProbs(opts, ProcessProbabilities())
}

// NewVariableSelectorProbs constructs a selector sharing session Probabilities
// (C++ process singleton). probs may be nil; callers that need tables pass live ones.
func NewVariableSelectorProbs(opts Options, probs *Probabilities) *VariableSelector {
	return &VariableSelector{
		Opts:  opts,
		Probs: probs,
	}
}

// RandomGlobalName mirrors VariableSelector.cpp RandomGlobalName → gensym("g_").
// util.cpp gensym_count is process-wide (shared with t_/func_/lbl_); no invent
// private VS.Sym counter that desyncs from create_new_tmp_var.
func (vs *VariableSelector) RandomGlobalName() string {
	return Gensym("g_")
}

// RandomLocalName mirrors RandomLocalName → gensym("l_").
func (vs *VariableSelector) RandomLocalName() string {
	return Gensym("l_")
}

// RandomParamName mirrors RandomParamName → gensym("p_").
func (vs *VariableSelector) RandomParamName() string {
	return Gensym("p_")
}

// atMaxGlobals reports whether GlobalList has hit the library MaxGlobals cap.
// Go-only Options.MaxGlobals (not CGOptions); fail closed create at cap —
// no invent unbounded GlobalList growth past the configured limit.
func (vs *VariableSelector) atMaxGlobals() bool {
	if vs == nil || vs.Opts.MaxGlobals < 1 {
		return false
	}
	return len(vs.GlobalList) >= vs.Opts.MaxGlobals
}

// ChooseVisibleReadVar mirrors VariableSelector::choose_visible_read_var.
// VariableSelector.cpp:361–377 — expand structs; match convert; on stack or global; not vol.
// Variable* always live; expand/list holes fail closed (nil pick).
func ChooseVisibleReadVar(
	r *Rng,
	b *Block,
	readVars []*Variable,
	typ *Type,
	unionFacts []*FactUnion,
) *Variable {
	// VariableSelector.cpp:363 — type from caller (goto uses get_int_type); sticky no invent
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete union fact map fails closed sticky (no invent soft-filter nonreadable past holes)
	if unionFacts != nil && !UnionFactsComplete(unionFacts) {
		SetError(ErrGeneric)
		return nil
	}
	expanded := ExpandStructUnionVars(append([]*Variable(nil), readVars...), typ)
	// IncompleteVariables expand — fail closed sticky (not invent filter past hole)
	if !VariablesComplete(expanded) {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete stack lists must not invent filter that drops all locals
	if b != nil && !b.StackScanComplete() {
		SetError(ErrGeneric)
		return nil
	}
	var ok []*Variable
	for _, v := range expanded {
		// pre-validated VariablesComplete; Type always live for match
		if v.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		if v.IsVirtual() || v.IsVolatile() {
			continue
		}
		if !typ.Match(v.Type, MatchConvert) {
			continue
		}
		onStack := b != nil && b.IsVarOnStack(v)
		if !onStack && !v.IsGlobal() {
			continue
		}
		if IsNonreadableField(v, unionFacts) {
			continue
		}
		ok = append(ok, v)
	}
	// VariableSelector.cpp always has RNG for multi-pick; ChooseOKVar handles n==0/1/nil r
	return ChooseOKVar(r, ok)
}

// FindVarByName mirrors VariableSelector::find_var_by_name.
// VariableSelector.cpp:1571–1579 — scan AllVars via match_var_name.
// Variable* always live on AllVars; nil hole fails closed (no invent skip).
func (vs *VariableSelector) FindVarByName(name string) *Variable {
	// VariableSelector always live; sticky no invent name lookup without it
	if vs == nil {
		SetError(ErrGeneric)
		return nil
	}
	// empty name incomplete sticky (no invent match-all / soft re-pick)
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	for _, v := range vs.AllVars {
		// Variable* always live on AllVars; nil hole sticky fail closed
		if v == nil {
			SetError(ErrGeneric)
			return nil
		}
		if m := v.MatchVarName(name); m != nil {
			return m
		}
	}
	// also search arrays' Variable wrappers
	// ArrayVariable* always live on Arrays; nil hole sticky fail closed
	for _, av := range vs.Arrays {
		if av == nil {
			SetError(ErrGeneric)
			return nil
		}
		if m := av.Variable.MatchVarName(name); m != nil {
			return m
		}
	}
	return nil
}

// DoFinalization mirrors VariableSelector::doFinalization.
// VariableSelector.cpp:1584–1592 — clear AllVars / GlobalList / nonvolatiles.
func (vs *VariableSelector) DoFinalization() {
	if vs == nil {
		return
	}
	vs.AllVars = nil
	vs.GlobalList = nil
	vs.GlobalNonvolatilesList = nil
	vs.Arrays = nil
	vs.TmpCount = 0
	vs.VarCreated = false
}

// InvalidIVBound mirrors ArrayVariable.h INVALID_BOUND (0xFFFFFFFF).
// Stored as -1 in IVBounds maps when the bound is unknown/unusable.
const InvalidIVBound = -1

// ItemizeArray mirrors VariableSelector::itemize_array.
// VariableSelector.cpp:1440–1500 — per-dim IV in bounds (sorted by name);
// optional +offset as FunctionInvocationBinary eAdd.
func (vs *VariableSelector) ItemizeArray(r *Rng, cg CGContext, av *ArrayVariable) *ArrayVariable {
	if av == nil || r == nil {
		return nil
	}
	if av.Collective != nil {
		return av
	}
	// incomplete ambient / facts fail closed sticky before IV pick (no invent soft re-pick)
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
	dims := len(av.Sizes)
	if dims == 0 {
		return nil
	}
	// VariableSelector.cpp:1442 — need at least one IV entry per dimension
	if len(cg.IVBounds) < dims {
		return nil
	}
	indices := make([]string, 0, dims)
	indexExprs := make([]*Expression, 0, dims)
	for d := 0; d < dims; d++ {
		dimenLen := av.Sizes[d]
		// VariableSelector.cpp:1448–1476 — ok_ivs sorted by name insert
		var ok []*Variable
		boundOf := map[*Variable]int{}
		for iv, bound := range cg.IVBounds {
			// Variable* always live as IVBounds keys; nil key fails closed sticky
			// (no invent partial ok_ivs pool by soft-skipping holes)
			if iv == nil {
				SetError(ErrGeneric)
				return nil
			}
			if bound == InvalidIVBound {
				continue
			}
			// iter->second != INVALID && bound < dimen_len
			if bound < 0 || bound >= dimenLen {
				continue
			}
			if iv.Type != nil && iv.Type.IsFloat() {
				continue
			}
			// VariableSelector.cpp:1455–1456 — signed char index option
			if !cgHasSignedCharIndex(vs) && iv.Type != nil && iv.Type.IsSignedChar() {
				continue
			}
			// VariableSelector.cpp:1457–1458 — ccomp packed aggregate field IV
			if vs != nil && vs.Opts.CComp && iv.IsPackedAggregateFieldVar() {
				continue
			}
			// insert sorted by name for platform-stable order
			inserted := false
			for j := 0; j < len(ok); j++ {
				if ok[j].Name > iv.Name {
					ok = append(ok[:j], append([]*Variable{iv}, ok[j:]...)...)
					inserted = true
					break
				}
			}
			if !inserted {
				ok = append(ok, iv)
			}
			boundOf[iv] = bound
		}
		v := ChooseOKVar(r, ok)
		if v == nil {
			return nil
		}
		// ExpressionVariable(*v) then optional (v + offset)
		// VariableSelector.cpp:1492–1497 — FunctionInvocationBinary(eAdd, …, flags=0)
		// null op_flags → Output emits plain "a + b" (FunctionInvocationBinary.cpp:357–361)
		idxExpr := &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
		idx := v.Name
		remain := dimenLen - boundOf[v]
		if remain > 1 {
			off := int(r.RndUpto(uint32(remain)))
			if off > 0 {
				offExpr := &Expression{
					Term: TermConstant, Con: MakeInt(off), ExprType: GetIntType(),
				}
				fi := &Invocation{
					IsStd:  true,
					Binary: "+",
					Args:   []*Expression{idxExpr, offExpr},
					// Safe nil: ArrayVariable index add must not use safe_* wrappers
				}
				idxExpr = &Expression{Term: TermFunction, Invoke: fi, ExprType: GetIntType()}
				idx = idxExpr.Output()
			}
		}
		indices = append(indices, idx)
		indexExprs = append(indexExprs, idxExpr)
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer,
			IsArray:    true,
			Init:       av.Init,
			InitExpr:   av.InitExpr,
			ArraySizes: av.Sizes,
			ArrayInits: av.ArrayInits,
		},
		Sizes:      av.Sizes,
		InitExprs:  append([]*Expression(nil), av.InitExprs...),
		InitValues: av.InitValues,
		Block:      cg.CurrentBlock(),
		Collective: av,
		Indices:    indices,
		IndexExprs: indexExprs,
	}
	item.AsArray = item
	// ArrayVariable.cpp:372–375 — create_field_vars for aggregate element type
	if item.Type != nil && item.Type.IsAggregate() {
		item.CreateFieldVars()
	}
	if vs != nil {
		vs.AllVars = append(vs.AllVars, &item.Variable)
	}
	// ArrayVariable.cpp:376 — blk->local_vars.push_back when parent block set
	if blk := cg.CurrentBlock(); blk != nil {
		item.Block = blk
		// itemized members tracked on block for is_variant / rnd_mutate
		found := false
		for _, lv := range blk.LocalVars {
			if lv == &item.Variable {
				found = true
				break
			}
		}
		if !found {
			blk.LocalVars = append(blk.LocalVars, &item.Variable)
		}
	}
	return item
}

func cgHasSignedCharIndex(vs *VariableSelector) bool {
	if vs == nil {
		return true
	}
	return vs.Opts.SignedCharIndex
}

// ChooseOKVar mirrors VariableSelector::choose_ok_var(vector<Variable*>).
// VariableSelector.cpp:318–337 — rnd pick; collective array → itemize.
// ChooseOKVar picks one eligible variable (optionally itemizing arrays).
// Incomplete candidate list fails closed sticky (nil pick — no invent skip hole).
func ChooseOKVar(r *Rng, vars []*Variable) *Variable {
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return nil
	}
	n := len(vars)
	if n == 0 {
		return nil
	}
	var v *Variable
	if n == 1 {
		v = vars[0]
	} else {
		// VariableSelector.cpp:324 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr)
		// random mode always GOOD; still call for fair wiring (process CGOptions)
		if DepthGuardByDepth(ProcessOptions(), 1) == BadDepth {
			return nil
		}
		// VariableSelector.cpp:326–329 — rnd_upto(len); sticky no invent vars[0] without RNG
		if r == nil {
			SetError(ErrGeneric)
			return nil
		}
		idx := r.RndUpto(uint32(n))
		// VariableSelector.cpp:327 ERROR_GUARD
		if HasError() {
			return nil
		}
		v = vars[idx]
	}
	// if collective array, return itemized member (VariableSelector.cpp:332–337)
	// C++ always itemize(); sticky no soft return collective on itemize fail / missing RNG
	if v != nil && v.IsArray && v.AsArray != nil && v.AsArray.Collective == nil {
		if r == nil {
			SetError(ErrGeneric)
			return nil
		}
		item := v.AsArray.Itemize(r)
		if item == nil {
			return nil
		}
		return &item.Variable
	}
	return v
}

// ChooseOKVarExactType filters vars whose Type matches want with eExact.
func ChooseOKVarExactType(r *Rng, vars []*Variable, want *Type) *Variable {
	return ChooseOKVarMatch(r, vars, want, MatchExact, false)
}

// FindAllVisibleVars mirrors VariableSelector::find_all_visible_vars.
// VariableSelector.cpp:752–759 — GlobalList + block chain locals (no params).
// FindAllVisibleVars mirrors VariableSelector::find_all_visible_vars.
// Variable* always live on GlobalList/LocalVars; nil hole fails closed
// IncompleteVariables (not bare nil invent empty-complete visible pool).
// Complete empty returns non-nil empty slice.
func (vs *VariableSelector) FindAllVisibleVars(b *Block) []*Variable {
	vars := make([]*Variable, 0)
	if vs != nil {
		if !VariablesComplete(vs.GlobalList) {
			// incomplete GlobalList fails closed sticky (no invent soft re-pick empty-complete)
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		vars = append(vars, vs.GlobalList...)
	}
	for b != nil {
		if !VariablesComplete(b.LocalVars) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		vars = append(vars, b.LocalVars...)
		b = b.Parent
	}
	return vars
}

// rootBlock walks to the outermost parent block.
func rootBlock(b *Block) *Block {
	for b != nil && b.Parent != nil {
		b = b.Parent
	}
	return b
}

// findParentOfStmIDInTree returns the block that directly owns stmID under root.
// Walks get_blocks only; incomplete if-arm skips that compound's children
// (no invent soft-skip missing arm then find under sibling arm / stray Then on assign).
func findParentOfStmIDInTree(root *Block, stmID int) *Block {
	if root == nil || stmID <= 0 {
		return nil
	}
	var walk func(b *Block) *Block
	walk = func(b *Block) *Block {
		if b == nil {
			return nil
		}
		for i := range b.Stmts {
			st := &b.Stmts[i]
			if st.StmID == stmID {
				return b
			}
			blks := GetBlocksStmt(st)
			incomplete := false
			for _, nb := range blks {
				if nb == nil {
					incomplete = true
					break
				}
			}
			if incomplete {
				continue
			}
			for _, nb := range blks {
				if p := walk(nb); p != nil {
					return p
				}
			}
		}
		return nil
	}
	return walk(root)
}

// findStmtByIDInTree finds a statement by stm_id under root (self + nested blocks).
// Walks get_blocks only; incomplete if-arm skips that compound's children.
func findStmtByIDInTree(root *Block, stmID int) *Stmt {
	if root == nil || stmID <= 0 {
		return nil
	}
	var walk func(b *Block) *Stmt
	walk = func(b *Block) *Stmt {
		if b == nil {
			return nil
		}
		for i := range b.Stmts {
			st := &b.Stmts[i]
			if st.StmID == stmID {
				return st
			}
			blks := GetBlocksStmt(st)
			incomplete := false
			for _, nb := range blks {
				if nb == nil {
					incomplete = true
					break
				}
			}
			if incomplete {
				continue
			}
			for _, nb := range blks {
				if s := walk(nb); s != nil {
					return s
				}
			}
		}
		return nil
	}
	return walk(root)
}

// BlockContainsStmID mirrors Block::contains_stmt for a statement id.
// Statement.cpp:684–705 — parent chain of s includes this block.
func BlockContainsStmID(b *Block, stmID int) bool {
	if b == nil || stmID <= 0 {
		return false
	}
	if b.StmID == stmID {
		return true
	}
	root := rootBlock(b)
	p := findParentOfStmIDInTree(root, stmID)
	for cur := p; cur != nil; cur = cur.Parent {
		if cur == b {
			return true
		}
	}
	return false
}

// ExpandBlockForGoto mirrors VariableSelector::expand_block_for_goto.
// VariableSelector.cpp:765–787 — climb parents so new locals are visible at
// both a goto dest inside b and the goto source outside b.
func ExpandBlockForGoto(b *Block, cg CGContext) *Block {
	if b == nil {
		return nil
	}
	fm := cg.FM
	if fm == nil {
		return b
	}
	// C++ edge->src is a live Statement*; look up via function tree (not only root of b)
	// CFGEdge* always live; nil hole fails closed (no invent skip as absent goto)
	for {
		expanded := false
		for _, e := range fm.CFGEdges {
			if e == nil {
				// incomplete CFG list hole — sticky (no invent soft-skip edge)
				SetError(ErrGeneric)
				return nil
			}
			if e.SrcID <= 0 {
				continue
			}
			// VariableSelector.cpp:773 — edge->src->eType == eGoto
			src := FindStmtByID(fm.Func, e.SrcID)
			if src == nil {
				src = findStmtByIDInTree(rootBlock(b), e.SrcID)
			}
			if src == nil || src.Kind != StmtGoto {
				continue
			}
			destID := e.DestStmID
			if destID <= 0 && e.DestBlock != nil {
				destID = e.DestBlock.StmID
			}
			if destID <= 0 {
				// StatementGoto dest may be stored only on the goto stmt
				if src.GotoDestStmID > 0 {
					destID = src.GotoDestStmID
				}
			}
			if destID <= 0 {
				continue
			}
			// VariableSelector.cpp:773–779
			if BlockContainsStmID(b, destID) && !BlockContainsStmID(b, e.SrcID) {
				for b != nil && !BlockContainsStmID(b, e.SrcID) {
					b = b.Parent
				}
				// VariableSelector.cpp:778 — assert(b); no soft invent return root
				// non-sticky null (sticky poisons Generate when climb fails; soft re-pick)
				if b == nil {
					return nil
				}
				expanded = true
				break
			}
		}
		if !expanded {
			break
		}
	}
	return b
}

// LowerBlockForVars mirrors VariableSelector::lower_block_for_vars.
// VariableSelector.cpp:793–815 — return first block in blks that covers all
// vars as locals; remaining uncovered vars returned for callers.
// Block*/Variable* always live; nil hole or incomplete LocalVars fails closed
// (nil blk, IncompleteVariables remaining — no invent empty remaining past hole).
func LowerBlockForVars(blks []*Block, vars []*Variable) (blk *Block, remaining []*Variable) {
	if !VariablesComplete(vars) {
		// incomplete vars/blocks fail closed sticky (no invent soft re-pick remaining)
		SetError(ErrGeneric)
		return nil, IncompleteVariables()
	}
	remaining = append([]*Variable(nil), vars...)
	for _, b := range blks {
		if b == nil || !VariablesComplete(b.LocalVars) {
			SetError(ErrGeneric)
			return nil, IncompleteVariables()
		}
		var next []*Variable
		for _, v := range remaining {
			if !IsVariableInSet(b.LocalVars, v) {
				next = append(next, v)
			}
		}
		remaining = next
		if len(remaining) == 0 {
			return b, remaining
		}
	}
	// globals/params only → nil block
	return nil, remaining
}

// FindAllNonArrayVisibleVars mirrors find_all_non_array_visible_vars.
// VariableSelector.cpp:713–735 — non-array globals, params, non-array locals.
// Variable* always live; nil hole fails closed IncompleteVariables
// (not bare nil invent empty-complete pool).
func (vs *VariableSelector) FindAllNonArrayVisibleVars(b *Block) []*Variable {
	vars := make([]*Variable, 0)
	if vs != nil {
		if !VariablesComplete(vs.GlobalList) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		for _, v := range vs.GlobalList {
			if !v.IsArray {
				vars = append(vars, v)
			}
		}
	}
	// resolve func via block or parents (inner blocks may leave Func nil)
	var f *Function
	for bb := b; bb != nil; bb = bb.Parent {
		if bb.Func != nil {
			f = bb.Func
			break
		}
	}
	if f != nil {
		if !VariablesComplete(f.Param) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		vars = append(vars, f.Param...)
	}
	for b != nil {
		if !VariablesComplete(b.LocalVars) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		for _, v := range b.LocalVars {
			if !v.IsArray {
				vars = append(vars, v)
			}
		}
		b = b.Parent
	}
	return vars
}

// GetAllLocalVars mirrors VariableSelector::get_all_local_vars.
// VariableSelector.cpp:747–751.
// Variable* always live on LocalVars; nil hole fails closed IncompleteVariables
// (not bare nil invent empty-complete local pool).
func GetAllLocalVars(b *Block) []*Variable {
	vars := make([]*Variable, 0)
	for b != nil {
		if !VariablesComplete(b.LocalVars) {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		vars = append(vars, b.LocalVars...)
		b = b.Parent
	}
	return vars
}

// GetAllArrayVars mirrors VariableSelector::get_all_array_vars.
// VariableSelector.cpp:737–745 — collect array globals (invalid for ccomp pointer init).
// Variable* always live; nil hole fails closed IncompleteVariables
// (not bare nil invent empty-complete array list).
func (vs *VariableSelector) GetAllArrayVars() []*Variable {
	out := make([]*Variable, 0)
	if vs == nil {
		return out
	}
	if !VariablesComplete(vs.GlobalList) {
		// incomplete GlobalList fails closed sticky (no invent empty-complete array pool)
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	for _, v := range vs.GlobalList {
		if v.IsArray {
			out = append(out, v)
		}
	}
	// Arrays list may hold collectives not yet on GlobalList
	// ArrayVariable* always live; bare nil return invents VariablesComplete(nil) empty-complete
	for _, av := range vs.Arrays {
		if av == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		if av.IsGlobal() && av.Collective == nil {
			if !IsVariableInSet(out, &av.Variable) {
				out = append(out, &av.Variable)
			}
		}
	}
	return out
}

// MakeInitValue mirrors VariableSelector::make_init_value.
// VariableSelector.cpp:824–912 — non-pointer (or 20% flip) → Constant;
// pointer → choose exact pointee / create & take address.
func (vs *VariableSelector) MakeInitValue(
	access Access,
	cg CGContext,
	t *Type,
	qf *CVQualifiers,
	b *Block,
	r *Rng,
) *Expression {
	// VariableSelector always has VS + type + RNG; sticky no invent init shell without them
	if vs == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before const/pointer pick
	// (no invent init / soft re-pick past holes under incomplete shells)
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
	// VariableSelector.cpp:830 — assert(qf && qf->sanity_check(t)); no invent empty qfer
	if qf == nil || !qf.SanityCheck(t) {
		SetError(ErrGeneric)
		return nil
	}
	qfer := *qf
	// VariableSelector.cpp:833 — initializer must not be stricter than var
	qfer.AcceptStricter = false

	// VariableSelector.cpp:836–841 — non-pointer or 20% chance → constant
	if !t.IsPointerLike() || r.RndFlipcoin(20) {
		// VariableSelector.cpp:837 ERROR_GUARD
		if HasError() {
			return nil
		}
		// VariableSelector.cpp:838–839 — assert simple != void sticky
		if t.IsSimple() && t.simple == EVoid {
			SetError(ErrGeneric)
			return nil
		}
		c := MakeRandom(t, vs.Opts, vs.Probs, r)
		// VariableSelector.cpp:842 ERROR_GUARD after make_random
		if c == nil || HasError() {
			return nil
		}
		return &Expression{Term: TermConstant, Con: c, ExprType: t}
	}

	// VariableSelector.cpp:842 ERROR_GUARD
	if HasError() {
		return nil
	}
	// pointer path: select visible var of pointee type
	pointee := t.PtrType()
	// VariableSelector.cpp:845 assert(type) sticky
	if pointee == nil {
		SetError(ErrGeneric)
		return nil
	}
	vars := vs.FindAllVisibleVars(b)
	// incomplete visible pool — fail closed sticky (no invent choose from partial)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return nil
	}
	noUnion := !vs.Opts.TakeUnionFieldAddr
	var invalid []*Variable
	var chosen *Variable
	// VariableSelector.cpp:853–862
	if b == nil && vs.Opts.CComp {
		invalid = vs.GetAllArrayVars()
		if !VariablesComplete(invalid) {
			SetError(ErrGeneric)
			return nil
		}
		chosen = ChooseVarFull(r, vars, access, cg, pointee, &qfer, MatchExact,
			invalid, true, true, noUnion)
	} else {
		if !vs.Opts.AddrTakenOfLocals {
			invalid = GetAllLocalVars(b)
			if !VariablesComplete(invalid) {
				SetError(ErrGeneric)
				return nil
			}
		}
		chosen = ChooseVarFull(r, vars, access, cg, pointee, &qfer, MatchExact,
			invalid, true, false, noUnion)
	}
	// VariableSelector.cpp:864 ERROR_GUARD
	if HasError() {
		return nil
	}

	if chosen == nil {
		// VariableSelector.cpp:866–904 — create suitable addressable
		if DepthGuardByType(vs.Opts, DtInitPointerValue) == BadDepth {
			return nil
		}
		noVolatile := false
		if vs.Opts.StrictVolatileRule {
			noVolatile = !cg.EffectContext().IsSideEffectFree()
		}
		qferDeref := qfer.RandomLooseQualifiers(noVolatile, access, cg, vs.Opts, vs.Probs, r)
		qferDeref.RemoveQualifiers(1)
		qferDeref.AcceptStricter = false
		// use_local: no globals OR (block set, pointee is pointer, non-vol qfer)
		useLocal := !vs.Opts.GlobalVariables ||
			(b != nil && pointee.IsPointerLike() && !qferDeref.IsVolatile())
		// VariableSelector.cpp:882–883 — strict_simple_type=true (no simple re-roll)
		var tt *Type
		if useLocal {
			tt = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, pointee, true, true)
		} else {
			tt = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, pointee, false, true)
		}
		// VariableSelector.cpp:884 ERROR_GUARD
		if tt == nil || HasError() {
			return nil
		}
		if vs.Opts.AddrTakenOfLocals && useLocal && b != nil {
			chosen = vs.GenerateNewParentLocal(b, AccessRead, cg, tt, &qferDeref, r)
			// VariableSelector.cpp:890 ERROR_GUARD
			if chosen == nil || HasError() {
				return nil
			}
			if chosen.Type != nil {
				RecordVolatileAccess(chosen, chosen.Type.IndirectLevel()-tt.IndirectLevel(), false)
			}
		} else {
			if vs.Opts.CComp {
				chosen = vs.GenerateNewNonArrayGlobal(AccessRead, cg, tt, &qferDeref, r)
			} else {
				chosen = vs.GenerateNewGlobal(AccessRead, cg, tt, &qferDeref, r)
			}
			// VariableSelector.cpp:901 ERROR_GUARD
			if chosen == nil || HasError() {
				return nil
			}
		}
		RecordAddressTaken(chosen)
	} else if chosen.Type != nil {
		// VariableSelector.cpp:905–909
		derefLevel := chosen.Type.IndirectLevel() - t.IndirectLevel()
		if derefLevel < 0 {
			RecordAddressTaken(chosen)
		}
	}
	// VariableSelector.cpp:910 assert(var); defensive after create paths (ERROR_GUARD earlier)
	if chosen == nil {
		return nil
	}
	// ExpressionVariable(*var, t) — desired type is pointer being inited
	return &Expression{Term: TermVariable, Var: chosen, ExprType: t}
}

// IsEligibleVar mirrors VariableSelector::is_eligible_var (core effect rules).
// VariableSelector.cpp:216–290 — itemized collective: read_indices first;
// then effect/const/volatile/FactUnion nonreadable checks on collective.
func IsEligibleVar(v *Variable, derefLevel int, access Access, cg CGContext) bool {
	// Variable always live; sticky incomplete no invent not-eligible soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete ambient fails closed sticky (no invent eligible / soft-skip as absent re-pick)
	if !EffectComplete(cg.EffectContext()) {
		SetError(ErrGeneric)
		return false
	}
	// VariableSelector.cpp:221–227 — itemized member → read_indices then use collective
	// Incomplete GetCollective fails closed sticky (no invent not-eligible past hole)
	coll := v.GetCollective()
	if coll == nil {
		SetError(ErrGeneric)
		return false
	}
	if coll != v {
		cgp := cg
		var facts []*FactPointTo
		if cg.FM != nil {
			// incomplete GlobalFacts fail closed sticky (no invent ReadIndices past holes)
			if !FactsComplete(cg.FM.GlobalFacts) {
				SetError(ErrGeneric)
				return false
			}
			facts = cg.FM.GlobalFacts
		}
		if !cgp.ReadIndices(v, facts) {
			// ReadIndices may leave sticky; fail closed as not eligible
			return false
		}
		v = coll
	}

	// VariableSelector.cpp:232–234 — partial volatile through pointer
	if derefLevel > 0 && v.IsPartialVolatileAfterDeref(derefLevel) {
		return false
	}
	eff := cg.EffectContext()
	isConst := v.IsConstAfterDeref(derefLevel)
	isVol := v.IsVolatileAfterDeref(derefLevel) || v.IsVolatile()

	// volatile + non-SE-free context → reject
	if isVol && !eff.IsSideEffectFree() {
		return false
	}
	// cannot read/write a var being written in context
	if access == AccessRead || access == AccessWrite {
		if eff.IsWrittenPartially(v) {
			return false
		}
	}
	// cannot write a var being read (deref_level==0)
	if access == AccessWrite && derefLevel == 0 && eff.IsReadPartially(v) {
		return false
	}
	// cannot write const
	if access == AccessWrite && isConst {
		return false
	}
	// VariableSelector.cpp:277–287 — nonreadable / nonwritable from context
	if access == AccessRead && cg.IsNonReadable(v) {
		return false
	}
	if access == AccessWrite && cg.IsNonWritable(v) {
		return false
	}
	// FactUnion::is_nonreadable_field on READ (VariableSelector.cpp:279–280)
	if access == AccessRead && cg.FM != nil {
		if IsNonreadableField(v, cg.FM.UnionFacts) {
			return false
		}
	}
	return true
}

// HasEligibleVolatileVar mirrors VariableSelector::has_eligible_volatile_var.
// VariableSelector.cpp:294–316 — flexible match + qfer match_indirect + eligible +
// is_volatile; increments Bookkeeper::volatile_avail on first hit.
func HasEligibleVolatileVar(vars []*Variable, typ *Type, access Access, cg CGContext) bool {
	return HasEligibleVolatileVarQfer(vars, typ, nil, access, cg)
}

// HasEligibleVolatileVarQfer is has_eligible_volatile_var with CVQualifiers filter.
// VariableSelector.cpp:294–316.
// Incomplete candidate list fails closed false (no invent skip hole as absent).
func HasEligibleVolatileVarQfer(vars []*Variable, typ *Type, qfer *CVQualifiers, access Access, cg CGContext) bool {
	// incomplete candidate list fails closed sticky (no invent skip hole as absent)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return false
	}
	for _, v := range vars {
		// incomplete type IR fails closed sticky (no invent filter past hole)
		if v.Type == nil {
			SetError(ErrGeneric)
			return false
		}
		if typ != nil && !typ.Match(v.Type, MatchFlexible) {
			continue
		}
		// VariableSelector.cpp:301–303 — qfer->match_indirect(var->qfer)
		if qfer != nil && !qfer.Wildcard {
			if !qfer.MatchIndirect(v.Qfer, false) {
				continue
			}
		}
		deref := 0
		if typ != nil {
			deref = v.Type.IndirectLevel() - typ.IndirectLevel()
		}
		if IsEligibleVar(v, deref, access, cg) && v.IsVolatile() {
			// VariableSelector.cpp:311 — Bookkeeper::volatile_avail++
			RecordVolatileAvail()
			return true
		}
	}
	return false
}

// HasDereferenceableVar mirrors VariableSelector::has_dereferenceable_var.
// VariableSelector.cpp:198–210 — type is_dereferenced_from + is_valid_ptr.
// Incomplete candidate list fails closed false (no invent skip hole).
func HasDereferenceableVar(vars []*Variable, typ *Type, cg CGContext, opts Options) bool {
	if typ == nil {
		return false
	}
	// incomplete candidate list fails closed sticky (no invent skip hole)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return false
	}
	var facts []*FactPointTo
	if cg.FM != nil {
		// incomplete GlobalFacts fail closed sticky (no invent is_valid_ptr past holes)
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return false
		}
		facts = cg.FM.GlobalFacts
	}
	for _, v := range vars {
		if v.Type == nil {
			SetError(ErrGeneric)
			return false
		}
		if typ.IsDereferencedFrom(v.Type) && IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			return true
		}
	}
	return false
}

// SelectMustUseVar mirrors VariableSelector::select_must_use_var.
// VariableSelector.cpp:1504–1560 — must_read eFlexible / must_write eDerefExact;
// itemize arrays; 75% erase after use.
func (vs *VariableSelector) SelectMustUseVar(
	r *Rng,
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
) *Variable {
	// VariableSelector always has VS + type; sticky no invent must-use shell without them
	if vs == nil || typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	// no RWDirective: soft re-pick (must-use list absent — not broken IR)
	if cg.RW == nil {
		return nil
	}
	// itemize / choose always has process RNG sticky
	if r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before must-use scan (no invent soft re-pick)
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
	var list *[]*Variable
	if access == AccessRead {
		list = &cg.RW.MustReadVars
	} else {
		list = &cg.RW.MustWriteVars
	}
	// VariableSelector.cpp:1514–1516
	mt := MatchFlexible
	if access == AccessWrite {
		mt = MatchDerefExact
	}
	blk := cg.CurrentBlock()
	// incomplete Param/LocalVars must not invent IsVisible false and skip must-use vars
	if blk != nil && !blk.StackScanComplete() {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete must-use list fails closed sticky (no invent partial scan)
	if !VariablesComplete(*list) {
		SetError(ErrGeneric)
		return nil
	}
	for i := 0; i < len(*list); i++ {
		v := (*list)[i]
		// Variable* always live in must-use lists; nil hole fails closed sticky
		// (no invent skip to next entry / partial must-use)
		if v == nil {
			SetError(ErrGeneric)
			return nil
		}
		// is_visible (VariableSelector.cpp:1523)
		if !v.IsVisible(blk) {
			continue
		}
		// Variable::type always live; Type-nil fails closed sticky (no invent soft-skip
		// incomplete must-use entry and still pick a later list member)
		if v.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		if !typ.Match(v.Type, mt) {
			continue
		}
		if qfer != nil && !qfer.Wildcard {
			// qfer->match(v->qfer)
			if !qfer.Match(v.Qfer, false) {
				continue
			}
		}
		deref := v.Type.IndirectLevel() - typ.IndirectLevel()
		// VariableSelector.cpp:1529–1532 — WRITE rejects const after deref
		if access == AccessWrite && v.Qfer.IsConstAfterDeref(deref) {
			continue
		}
		var out *Variable
		if v.IsArray && v.AsArray != nil {
			// VariableSelector.cpp:1528–1530 — always itemize_array; no bare collective
			// (C++ var = itemize_array(...); if null, try next — never return collective)
			// RNG always live for itemize; sticky nil r (no invent skip to next)
			if r == nil {
				SetError(ErrGeneric)
				return nil
			}
			item := vs.ItemizeArray(r, cg, v.AsArray)
			if item != nil {
				out = &item.Variable
			}
			// if itemize fails, try next must-use entry (do not fall back to bare array)
		} else {
			out = v
		}
		if out != nil {
			// 75% erase from must-use list (VariableSelector.cpp:1552–1555)
			// C++ always has RNG for flip; no invent forced erase without draw
			if r != nil && r.RndFlipcoin(75) {
				*list = append((*list)[:i], (*list)[i+1:]...)
			}
			return out
		}
	}
	return nil
}

// ChooseVar mirrors VariableSelector::choose_var type+eligibility filter.
// VariableSelector.cpp:394–447.
func ChooseVar(
	r *Rng,
	vars []*Variable,
	access Access,
	cg CGContext,
	want *Type,
	mt MatchType,
) *Variable {
	return ChooseVarQfer(r, vars, access, cg, want, nil, mt)
}

// ChooseVarQfer is choose_var with optional CVQualifiers filter.
func ChooseVarQfer(
	r *Rng,
	vars []*Variable,
	access Access,
	cg CGContext,
	want *Type,
	qfer *CVQualifiers,
	mt MatchType,
) *Variable {
	return ChooseVarFull(r, vars, access, cg, want, qfer, mt, nil, false, false, false)
}

// ChooseVarFull mirrors VariableSelector::choose_var full signature.
// VariableSelector.cpp:394–516 — expand_struct, invalid_vars, no_bitfield, no_union,
// then artificial bias toward dereference / address-of among ok_vars.
func ChooseVarFull(
	r *Rng,
	vars []*Variable,
	access Access,
	cg CGContext,
	want *Type,
	qfer *CVQualifiers,
	mt MatchType,
	invalidVars []*Variable,
	noBitfield, noExpandStructUnion, noUnion bool,
) *Variable {
	// incomplete ambient / facts fail closed sticky before eligibility scan
	// (no invent soft-skip as absent / soft re-pick past holes)
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
	// incomplete candidate / invalid lists — fail closed sticky (no invent choose past hole)
	if !VariablesComplete(vars) || !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	if want == nil {
		var ok []*Variable
		for _, v := range vars {
			// Variable::type always live; Type-nil fails closed sticky (no invent eligible
			// without type IR via soft-skip)
			if v.Type == nil {
				SetError(ErrGeneric)
				return nil
			}
			if IsVariableInSet(invalidVars, v) {
				continue
			}
			if noBitfield && v.IsBitfield {
				continue
			}
			if noUnion && v.IsInsideUnionField() {
				continue
			}
			if IsEligibleVar(v, 0, access, cg) {
				ok = append(ok, v)
			}
		}
		return ChooseOKVar(r, ok)
	}
	cands := vars
	// VariableSelector.cpp:405–410 — expand when type simple/aggregate
	if !noExpandStructUnion && (want.IsSimple() || want.IsAggregate()) {
		cands = ExpandStructUnionVars(vars, want)
		if !VariablesComplete(cands) {
			SetError(ErrGeneric)
			return nil
		}
	}
	// VariableSelector.cpp:420–421 — has_eligible_volatile_var (side-effect: volatile_avail)
	_ = HasEligibleVolatileVarQfer(cands, want, qfer, access, cg)
	// VariableSelector.cpp:412–419 — pointer_avail_for_dereference bookkeeping
	// FactPointTo::is_valid_ptr reads process CGOptions null/dead deref probs
	opts := ProcessOptions()
	if HasDereferenceableVar(cands, want, cg, opts) {
		RecordPointerAvailForDeref()
	}
	// CVQualifiers::match_indirect → match → CGOptions::match_exact_qualifiers()
	// no soft invent matchExact:=false ignoring process / StatementAssign force
	matchExact := opts.MatchExactQualifiers
	var ok []*Variable
	for _, x := range cands {
		if x.Type == nil {
			// incomplete type IR — fail closed sticky (no invent skip candidate)
			SetError(ErrGeneric)
			return nil
		}
		// VariableSelector.cpp:424–429
		if noBitfield && x.IsBitfield {
			continue
		}
		if noUnion && x.IsInsideUnionField() {
			continue
		}
		if !want.Match(x.Type, mt) {
			continue
		}
		if qfer != nil && !qfer.Wildcard {
			if !qfer.MatchIndirect(x.Qfer, matchExact) {
				continue
			}
		}
		if IsVariableInSet(invalidVars, x) {
			continue
		}
		deref := x.Type.IndirectLevel() - want.IndirectLevel()
		if !IsEligibleVar(x, deref, access, cg) {
			continue
		}
		ok = append(ok, x)
	}
	return chooseVarFromOK(r, want, ok, opts)
}

// chooseVarFromOK mirrors VariableSelector::choose_var post-filter bias.
// VariableSelector.cpp:459–516 — prefer deref of higher-indirection vars, then
// address-of lower-indirection (respecting take_union_field_addr); else uniform.
// Volatile bias block is disabled upstream (if (0)).
// chooseVarFromOK biases among already-filtered candidates.
// Variable* always live in ok; nil hole fails closed (nil pick).
func chooseVarFromOK(r *Rng, want *Type, ok []*Variable, opts Options) *Variable {
	for _, vv := range ok {
		if vv == nil || vv.Type == nil {
			// incomplete ok pool fails closed sticky (no invent soft-skip / empty pick)
			SetError(ErrGeneric)
			return nil
		}
	}
	// VariableSelector.cpp:459–471 — artificially increase odds of dereferencing
	if want != nil && len(ok) > 1 {
		var ptrs []*Variable
		wantInd := want.IndirectLevel()
		for _, vv := range ok {
			if wantInd < vv.Type.IndirectLevel() {
				ptrs = append(ptrs, vv)
			}
		}
		if v := ChooseOKVar(r, ptrs); v != nil {
			return v
		}
	}
	// VariableSelector.cpp:484–514 — artificially increase odds of taking address
	if want != nil && want.IsPointerLike() && len(ok) > 1 {
		var addressable []*Variable
		wantInd := want.IndirectLevel()
		for _, vv := range ok {
			if wantInd > vv.Type.IndirectLevel() {
				// VariableSelector.cpp:490–494
				if !opts.TakeUnionFieldAddr && vv.IsInsideUnionField() {
					continue
				}
				addressable = append(addressable, vv)
			}
		}
		if v := ChooseOKVar(r, addressable); v != nil {
			return v
		}
	}
	return ChooseOKVar(r, ok)
}

// ExpandStructUnionVars mirrors VariableSelector::expand_struct_union_vars.
// VariableSelector.cpp:156–173 — replace non-matching aggregates with field_vars.
// Variable* always live; nil hole / incomplete FieldVars fails closed
// IncompleteVariables (not bare nil invent empty-complete expand pool).
func ExpandStructUnionVars(vars []*Variable, want *Type) []*Variable {
	if !VariablesComplete(vars) {
		// incomplete pool fails closed sticky (no invent soft re-pick empty expand)
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	out := append([]*Variable(nil), vars...)
	for i := 0; i < len(out); i++ {
		v := out[i]
		if v.IsVirtual() {
			continue
		}
		// don't break up a struct if it matches the given type
		if v.Type != nil && v.Type.IsAggregate() && v.Type != want {
			// FieldVars always live; incomplete fails closed sticky
			if !v.FieldVarsComplete() {
				SetError(ErrGeneric)
				return IncompleteVariables()
			}
			// erase i, append field_vars at end (upstream insert end + i--)
			fields := v.FieldVars
			out = append(out[:i], out[i+1:]...)
			out = append(out, fields...)
			i--
		}
	}
	return out
}

// ChooseOKVarMatch mirrors choose_var type filter + choose_ok_var.
// VariableSelector.cpp choose_var — expand_struct_union_vars; Type::match(mt); optional skip const.
func ChooseOKVarMatch(r *Rng, vars []*Variable, want *Type, mt MatchType, skipConst bool) *Variable {
	if want == nil {
		return ChooseOKVar(r, vars)
	}
	// expand aggregates when want is simple or aggregate (choose_var:403–406)
	cands := vars
	if want.IsSimple() || want.IsAggregate() {
		cands = ExpandStructUnionVars(vars, want)
	}
	// incomplete expand / candidate list — fail closed sticky
	if !VariablesComplete(cands) {
		SetError(ErrGeneric)
		return nil
	}
	var ok []*Variable
	for _, x := range cands {
		if x.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		if skipConst && x.IsConst() {
			continue
		}
		if want.Match(x.Type, mt) {
			ok = append(ok, x)
		}
	}
	return ChooseOKVar(r, ok)
}

func typesMatchExact(a, b *Type) bool {
	// Type::match eExact is pointer identity (cached simple/pointer types).
	if a == nil || b == nil {
		return a == b
	}
	return a.Match(b, MatchExact)
}

// applyInitExpr stores make_init_value result on Variable (Constant and/or full expr).
func applyInitExpr(v *Variable, init *Expression) {
	if v == nil || init == nil {
		return
	}
	v.InitExpr = init
	if init.Term == TermConstant && init.Con != nil {
		v.Init = init.Con
	}
}

// createAndInitialize mirrors VariableSelector::create_and_initialize.
// VariableSelector.cpp:518–536 — array flip → create_array_and_itemize (returns itemize());
// else make_init_value + new_variable. Does NOT push GlobalList/LocalVars/FM/access_once
// (callers GenerateNewGlobal / GenerateNewParentLocal do).
func (vs *VariableSelector) createAndInitialize(
	access Access,
	cg CGContext,
	t *Type,
	qfer CVQualifiers,
	blk *Block,
	name string,
	r *Rng,
) *Variable {
	// VariableSelector always has VS + type + RNG; sticky no invent create shell without them
	if vs == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// name always live from gensym; sticky no invent empty-name create path
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent soft re-pick)
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
	// VariableSelector.cpp:525–530 — NewArrayVariableProb → create_array_and_itemize
	if vs.Opts.Arrays && vs.Probs != nil && r.RndFlipcoin(uint32(vs.Probs.Single(PNewArrayVariableProb))) {
		// VariableSelector.cpp:526–529 — strict_const → Constant::make_random; else make_init_value
		var init *Constant
		var ie *Expression
		if vs.Opts.StrictConstArrays {
			// VariableSelector.cpp:526–527 — Constant::make_random; ERROR_GUARD
			// no invent array with nil init when make_random fails
			init = MakeRandom(t, vs.Opts, vs.Probs, r)
			if init == nil || HasError() {
				return nil
			}
			ie = &Expression{Term: TermConstant, Con: init, ExprType: t}
		} else {
			// VariableSelector.cpp:528–529 — make_init_value; ERROR_GUARD
			// no invent array without live Expression* init
			ie = vs.MakeInitValue(access, cg, t, &qfer, blk, r)
			if ie == nil || HasError() {
				return nil
			}
			if ie.Term == TermConstant {
				init = ie.Con
			}
			// Expression init may be non-constant; still store on array InitExpr
		}
		// VariableSelector.cpp:1325–1333 — CreateArrayVariable; AllVars; return itemize()
		// no soft invent scalar fallback when array create fails after flip
		if HasError() {
			return nil
		}
		av := CreateArrayVariable(r, vs.Opts, vs.Probs, vs, &cg, blk, name, t, init, qfer)
		if av == nil || HasError() {
			return nil
		}
		if ie != nil {
			av.InitExpr = ie
		}
		vs.AllVars = append(vs.AllVars, &av.Variable)
		vs.Arrays = append(vs.Arrays, av)
		// ArrayVariable.cpp:249–275 — itemize() random indices; AllVars; field_vars
		item := av.ItemizeInto(r, vs)
		if item == nil {
			return nil
		}
		// VariableSelector.cpp:535 — assert(var); return itemized member
		vs.VarCreated = true
		return &item.Variable
	}
	// VariableSelector.cpp:531–533 — make_init_value + new_variable
	// make_init_value always returns Expression* or ERROR_GUARD(nullptr)
	ie := vs.MakeInitValue(access, cg, t, &qfer, blk, r)
	if ie == nil || HasError() {
		return nil
	}
	v := CreateVariableWithInit(name, t, nil, qfer)
	if v == nil {
		// VariableSelector.cpp:535 assert(var) sticky
		SetError(ErrGeneric)
		return nil
	}
	applyInitExpr(v, ie)
	vs.AllVars = append(vs.AllVars, v)
	vs.VarCreated = true
	return v
}

// varCollective returns get_collective() for FM (itemized array → parent collective).
func varCollective(v *Variable) *Variable {
	if v == nil {
		return nil
	}
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return &v.AsArray.Collective.Variable
	}
	return v.GetCollective()
}

// GenerateNewNonArrayGlobal mirrors VariableSelector::GenerateNewNonArrayGlobal.
// VariableSelector.cpp:578–606 — force scalar global (no array flip).
func (vs *VariableSelector) GenerateNewNonArrayGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	// VariableSelector always has VS + type + RNG; sticky no invent global shell without them
	if vs == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:129 assert global_variables; library → nil
	if !vs.Opts.GlobalVariables {
		return nil
	}
	if vs.atMaxGlobals() {
		return nil
	}
	// VariableSelector.cpp:580 ERROR_GUARD
	if HasError() {
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent global past holes)
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
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, false, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	// VariableSelector.cpp:585 ERROR_GUARD after random_qualifiers
	if HasError() {
		return nil
	}
	name := vs.RandomGlobalName()
	// gensym always live; sticky no invent empty-name non-array global shell
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	vs.TmpCount++
	// VariableSelector.cpp:589–592 — make_init then new_variable (skip array flip)
	// use &varQfer (C++ may pass original qfer; assert requires non-null qf)
	// make_init_value always Expression* or ERROR_GUARD — no invent uninit var
	ie := vs.MakeInitValue(access, cg, t, &varQfer, nil, r)
	if ie == nil || HasError() {
		return nil
	}
	v := CreateVariableWithInit(name, t, nil, varQfer)
	if v == nil {
		return nil
	}
	applyInitExpr(v, ie)
	// VariableSelector.cpp:147–149 new_variable → AllVars
	vs.AllVars = append(vs.AllVars, v)
	vs.GlobalList = append(vs.GlobalList, v)
	// VariableSelector.cpp:596–597 — FM on collective
	// Incomplete GetCollective must not invent success without facts (AddNewVarFactAndUpdate(nil) no-ops)
	// Incomplete GlobalFacts after register must not invent create success
	if cg.FM != nil {
		coll := varCollective(v)
		if coll == nil {
			if n := len(vs.GlobalList); n > 0 && vs.GlobalList[n-1] == v {
				vs.GlobalList = vs.GlobalList[:n-1]
			}
			if n := len(vs.AllVars); n > 0 && vs.AllVars[n-1] == v {
				vs.AllVars = vs.AllVars[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
		cg.FM.AddNewVarFactAndUpdate(nil, coll)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if n := len(vs.GlobalList); n > 0 && vs.GlobalList[n-1] == v {
				vs.GlobalList = vs.GlobalList[:n-1]
			}
			if n := len(vs.AllVars); n > 0 && vs.AllVars[n-1] == v {
				vs.AllVars = vs.AllVars[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
	}
	// VariableSelector.cpp:598 — current_func new_globals
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, v)
	}
	// VariableSelector.cpp:600–602 — no access_once on NonArray path
	if !varQfer.IsVolatile() {
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	vs.VarCreated = true
	return v
}

// GenerateNewGlobal mirrors VariableSelector::GenerateNewGlobal for simple types:
// random_qualifiers (or copy qfer), RandomGlobalName, create_and_initialize, GlobalList.
// VariableSelector.cpp:546–575.
func (vs *VariableSelector) GenerateNewGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	// VariableSelector always has VS + type + RNG; sticky no invent global shell without them
	if vs == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	if !vs.Opts.GlobalVariables {
		return nil
	}
	if vs.atMaxGlobals() {
		return nil
	}
	// VariableSelector.cpp:550 ERROR_GUARD
	if HasError() {
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent global past holes)
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
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		// CVQualifiers::random_qualifiers(t, access, cg, false)
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, false, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	// VariableSelector.cpp:555 ERROR_GUARD after random_qualifiers
	if HasError() {
		return nil
	}
	name := vs.RandomGlobalName()
	// gensym always live; sticky no invent empty-name global shell
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	vs.TmpCount++
	v := vs.createAndInitialize(access, cg, t, varQfer, nil, name, r)
	if v == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:561 — GlobalList (itemized array member when array path)
	vs.GlobalList = append(vs.GlobalList, v)
	// VariableSelector.cpp:563–564 — FM on collective
	// Incomplete GetCollective must not invent success without facts (AddNewVarFactAndUpdate(nil) no-ops)
	// Incomplete GlobalFacts after register must not invent create success
	if cg.FM != nil {
		coll := varCollective(v)
		if coll == nil {
			// drop partial GlobalList registration (no invent orphan global past hole)
			if n := len(vs.GlobalList); n > 0 && vs.GlobalList[n-1] == v {
				vs.GlobalList = vs.GlobalList[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
		cg.FM.AddNewVarFactAndUpdate(nil, coll)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if n := len(vs.GlobalList); n > 0 && vs.GlobalList[n-1] == v {
				vs.GlobalList = vs.GlobalList[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
	}
	// VariableSelector.cpp:565 — current_func()->new_globals
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, v)
	}
	// VariableSelector.cpp:567–572 — access_once only for non-volatile globals
	if !varQfer.IsVolatile() {
		if vs.Opts.AccessOnce && vs.Probs != nil && r.RndFlipcoin(uint32(vs.Probs.Single(PAccessOnceVariableProb))) {
			v.IsAccessOnce = true
		}
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	// wrap_volatiles → VOL_RVAL on Output
	if vs.Opts.WrapVolatiles && v.IsVolatile() {
		v.UseVolRVal = true
	}
	vs.VarCreated = true
	// VariableSelector.cpp:1230–1236 — use_new_var stats
	RecordVarCreated(v)
	return v
}

// SelectGlobal mirrors VariableSelector::SelectGlobal.
// VariableSelector.cpp:669–695 — choose_var; expand_struct eager create; else GenerateNewGlobal.
func (vs *VariableSelector) SelectGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	return vs.SelectGlobalMT(access, cg, t, qfer, r, MatchFlexible, nil)
}

// SelectGlobalMT is SelectGlobal with match type and invalid_vars.
// VariableSelector.cpp:669–695.
func (vs *VariableSelector) SelectGlobalMT(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars []*Variable,
) *Variable {
	// VariableSelector always live; sticky no invent SelectGlobal without VS
	if vs == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before choose/create (no invent soft re-pick)
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
	// incomplete GlobalList / invalid_vars fail closed sticky (no invent soft-skip holes)
	if !VariablesComplete(vs.GlobalList) || !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp always has process RNG for multi-choose / create paths
	// n==1 ChooseOKVar can skip draw; multi without r sticky fail closed
	if r == nil && len(vs.GlobalList) != 1 {
		SetError(ErrGeneric)
		return nil
	}
	// choose_var(GlobalList, …, mt, invalid_vars)
	v := ChooseVarFull(r, vs.GlobalList, access, cg, t, qfer, mt, invalidVars, false, false, false)
	if v != nil {
		return v
	}
	// VariableSelector.cpp:677–684 — expand_struct eager path (invalid_vars through)
	if vs.Opts.ExpandStruct {
		if v = vs.EagerCreateGlobalStruct(access, cg, t, qfer, r, mt, invalidVars); v != nil {
			return v
		}
	}
	// VariableSelector.cpp:685 — DEPTH_GUARD_BY_TYPE_RETURN(dtSelectGlobal, nullptr)
	// only on the GenerateNewGlobal path after choose_var miss
	if DepthGuardByType(vs.Opts, DtSelectGlobal) == BadDepth {
		return nil
	}
	// VariableSelector.cpp:685–694 — random_type_from_type then GenerateNewGlobal
	noVol := qfer != nil && !qfer.Wildcard && !qfer.IsVolatile()
	// VariableSelector.cpp:690 — random_type_from_type(type, no_volatile) defaults strict_simple=false
	t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, noVol, false)
	// VariableSelector.cpp:691–693 — ERROR_GUARD(nullptr); no soft invent keep original type
	if t2 == nil || HasError() {
		return nil
	}
	v = vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	if v == nil || HasError() {
		return nil
	}
	return v
}

// chooseRandomStructFromType mirrors Type::choose_random_struct_from_type.
// Type.cpp:570–586 — if ok structs exist pick one; else return original type.
// Incomplete ok pool fails closed nil (no invent keep original typ past hole
// via len(nil)==0 empty-complete success).
func chooseRandomStructFromType(env *TypeEnv, typ *Type, noVolatile bool, r *Rng) *Type {
	if typ == nil || r == nil {
		return typ
	}
	cands := okStructUnionLTypes(env, noVolatile, true, true)
	// incomplete ok pool fails closed sticky (no invent keep original typ past hole)
	if !typesComplete(cands) {
		SetError(ErrGeneric)
		return nil
	}
	if len(cands) == 0 {
		return typ
	}
	st := cands[r.RndUpto(uint32(len(cands)))]
	if st == nil {
		SetError(ErrGeneric)
		return nil
	}
	return st
}

// EagerCreateGlobalStruct mirrors VariableSelector::eager_create_global_struct.
// VariableSelector.cpp:607–633 — create a random ok struct global, then choose_var field.
// invalidVars passed through to choose_var (C++ signature).
func (vs *VariableSelector) EagerCreateGlobalStruct(
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars ...[]*Variable,
) *Variable {
	// VariableSelector always has VS + type + RNG; sticky no invent eager global struct without them
	if vs == nil || typ == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent soft re-pick)
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
	// VariableSelector.cpp:611 assert(type)
	level := typ.IndirectLevel()
	var inv []*Variable
	if len(invalidVars) > 0 {
		inv = invalidVars[0]
	}
	if !VariablesComplete(inv) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:613–630
	var st *Type
	var createQfer *CVQualifiers
	switch level {
	case 0:
		// choose_random_struct_from_type(type, false)
		st = chooseRandomStructFromType(vs.Types, typ, false, r)
		createQfer = qfer
	case 1:
		// C++ source has t->ptr_type with t==null (upstream bug); fair uses type->ptr_type
		pointee := typ.PtrType()
		if pointee == nil {
			return nil
		}
		st = chooseRandomStructFromType(vs.Types, pointee, false, r)
		if qfer != nil {
			// VariableSelector.cpp:621–622 — qfer->indirect_qualifiers(level)
			q1 := qfer.IndirectQualifiers(level)
			createQfer = &q1
		} else {
			createQfer = nil
		}
	default:
		return nil
	}
	// ERROR_GUARD if choose_random_struct / Generate fails
	if st == nil || HasError() {
		return nil
	}
	if vs.GenerateNewGlobal(access, cg, st, createQfer, r) == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:631–632 — choose_var(GlobalList, …, invalid_vars)
	return ChooseVarFull(r, vs.GlobalList, access, cg, typ, qfer, mt, inv, false, false, false)
}

// EagerCreateLocalStruct mirrors VariableSelector::eager_create_local_struct.
// VariableSelector.cpp:635–666 — create struct local on block, choose_var field.
func (vs *VariableSelector) EagerCreateLocalStruct(
	block *Block,
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars ...[]*Variable,
) *Variable {
	// VariableSelector always has VS + block + type + RNG; sticky no invent eager local struct without them
	if vs == nil || block == nil || typ == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent soft re-pick)
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
	// VariableSelector.cpp:641 assert(type)
	level := typ.IndirectLevel()
	var inv []*Variable
	if len(invalidVars) > 0 {
		inv = invalidVars[0]
	}
	if !VariablesComplete(inv) {
		SetError(ErrGeneric)
		return nil
	}
	var st *Type
	var createQfer *CVQualifiers
	switch level {
	case 0:
		// choose_random_struct_from_type(type, true) — no_volatile for locals
		st = chooseRandomStructFromType(vs.Types, typ, true, r)
		createQfer = qfer
	case 1:
		// fair type->ptr_type (upstream t->ptr_type with t==0)
		pointee := typ.PtrType()
		if pointee == nil {
			return nil
		}
		st = chooseRandomStructFromType(vs.Types, pointee, true, r)
		if qfer != nil {
			q1 := qfer.IndirectQualifiers(level)
			createQfer = &q1
		} else {
			createQfer = nil
		}
	default:
		return nil
	}
	// VariableSelector.cpp:654–656 — ERROR_GUARD; if (!t) return nullptr
	if st == nil || HasError() {
		return nil
	}
	if vs.GenerateNewParentLocal(block, access, cg, st, createQfer, r) == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:661–663 — choose_var(block.local_vars, …, invalid_vars)
	return ChooseVarFull(r, block.LocalVars, access, cg, typ, qfer, mt, inv, false, false, false)
}

// GenerateParameterVariableTyped mirrors
// VariableSelector::GenerateParameterVariable(type, qfer).
// VariableSelector.cpp:955–957.
func (vs *VariableSelector) GenerateParameterVariableTyped(typ *Type, qfer CVQualifiers) *Variable {
	// VariableSelector always live; sticky no invent param shell without VS
	if vs == nil {
		SetError(ErrGeneric)
		return nil
	}
	name := vs.RandomParamName()
	// gensym always live; sticky no invent empty-name parameter shell
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	v := CreateVariableQfer(name, typ, qfer)
	if v == nil {
		SetError(ErrGeneric)
		return nil
	}
	vs.AllVars = append(vs.AllVars, v)
	return v
}

// GenerateParameterVariable mirrors VariableSelector::GenerateParameterVariable(Function&).
// VariableSelector.cpp:963–981 — 40% pointer when derived exist; else nonvoid nonvolatile;
// ERROR_RETURN after each RNG; no make_random_pointer / simple invent.
func (vs *VariableSelector) GenerateParameterVariable(f *Function, r *Rng) *Variable {
	// VariableSelector always has VS + Function + RNG; sticky no invent param without them
	if vs == nil || f == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:967 ERROR_RETURN after flipcoin setup
	if HasError() {
		return nil
	}
	// VariableSelector.cpp:966–972 — has_pointer_type() && flipcoin(40)
	// no soft invent MakeRandomPointerType when choose returns nil
	rndPtr := r.RndFlipcoin(40)
	if HasError() {
		return nil
	}
	var t *Type
	if vs.Types != nil && vs.Types.HasPointerType() && rndPtr {
		// Type::choose_random_pointer_type — pick existing derived only
		t = vs.Types.ChooseRandomPointerType(r)
	} else if vs.Types != nil {
		// Type::choose_random_nonvoid_nonvolatile (arg_structs/arg_unions in filter)
		// no soft invent GetSimpleType when AllTypes empty
		t = vs.Types.ChooseRandomNonvoidNonvolatile(r, vs.Opts, vs.Probs)
	}
	// VariableSelector.cpp:972 ERROR_RETURN
	if t == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:973–974 assert non-void simple sticky
	if t.IsSimple() && t.Simple() == EVoid {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:976 — CVQualifiers::random_qualifiers(t)
	qfer := RandomQualifiersNoContextNoVolatile(t, vs.Opts, vs.Probs, r)
	// VariableSelector.cpp:977 ERROR_RETURN
	if HasError() {
		return nil
	}
	v := vs.GenerateParameterVariableTyped(t, qfer)
	// VariableSelector.cpp:979–980 ERROR_RETURN; param.push_back
	if v == nil || HasError() {
		return nil
	}
	f.Param = append(f.Param, v)
	return v
}

// SelectLoopCtrlVar mirrors VariableSelector::SelectLoopCtrlVar.
// VariableSelector.cpp:1146–1179 — non-array visible, has_int_field, drop union+ptr;
// choose_var(WRITE, eConvert, invalid, no_bitfield=true).
func (vs *VariableSelector) SelectLoopCtrlVar(r *Rng, cg CGContext, invalid map[*Variable]bool) *Variable {
	// VariableSelector always has VS + RNG; sticky no invent loop-ctrl shell without them
	if vs == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before filter (no invent soft re-pick)
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
	ty := GetIntType()
	blk := cg.CurrentBlock()
	vars := vs.FindAllNonArrayVisibleVars(blk)
	// incomplete visible pool — fail closed sticky (no invent loop ctrl from partial)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1155–1168 — remove no-int-field and union-with-pointer
	var filtered []*Variable
	var invalidSlice []*Variable
	for _, v := range vars {
		if v.Type == nil {
			SetError(ErrGeneric)
			return nil
		}
		if invalid[v] {
			invalidSlice = append(invalidSlice, v)
			continue
		}
		if !v.Type.HasIntField() {
			continue
		}
		if v.Type.IsUnion() && v.Type.ContainPointerField() {
			continue
		}
		filtered = append(filtered, v)
	}
	// also pass invalid map entries not already in filtered pool
	for v := range invalid {
		if v != nil && !IsVariableInSet(invalidSlice, v) {
			invalidSlice = append(invalidSlice, v)
		}
	}
	// VariableSelector.cpp:1169–1170 — choose_var(..., eConvert, invalid_vars, no_bitfield=true)
	if v := ChooseVarFull(r, filtered, AccessWrite, cg, ty, nil, MatchConvert, invalidSlice, true, false, false); v != nil {
		return v
	}
	// VariableSelector.cpp:1170 ERROR_GUARD after choose_var
	if HasError() {
		return nil
	}
	// VariableSelector.cpp:1172–1178 — create global or parent local
	if vs.Opts.GlobalVariables {
		return vs.GenerateNewGlobal(AccessWrite, cg, ty, nil, r)
	}
	if blk != nil {
		return vs.GenerateNewParentLocal(blk, AccessWrite, cg, ty, nil, r)
	}
	return nil
}

// GenerateNewParentLocal mirrors VariableSelector::GenerateNewParentLocal.
// VariableSelector.cpp:915–947 — volatile aggregate → global; expand_block_for_goto;
// random_qualifiers no_volatile; create_and_initialize into enlarged block.
func (vs *VariableSelector) GenerateNewParentLocal(
	block *Block,
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	// VariableSelector always has VS + block + type + RNG; sticky no invent local shell without them
	if vs == nil || block == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:920 ERROR_GUARD
	if HasError() {
		return nil
	}
	// incomplete ambient / facts fail closed sticky before create (no invent local past holes)
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
	// VariableSelector.cpp:921 assert(t); 920–923 — volatile struct/union field(s) → global
	if t.IsAggregate() && t.IsVolatileStructUnion() {
		return vs.GenerateNewGlobal(access, cg, t, qfer, r)
	}
	// VariableSelector.cpp:924–928 — enlarge for goto visibility
	block = ExpandBlockForGoto(block, cg)
	if block == nil {
		return nil
	}
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		// random_qualifiers(t, access, cg, true) — no_volatile true for locals
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, true, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	// VariableSelector.cpp:937 ERROR_GUARD after random_qualifiers
	if HasError() {
		return nil
	}
	// VariableSelector.cpp:938 — restrict(access, cg_context)
	varQfer.Restrict(access, cg)
	// VariableSelector.cpp:939 — assert(var_qfer.sanity_check(t)); no soft invent bad qfer
	if !varQfer.SanityCheck(t) {
		SetError(ErrGeneric)
		return nil
	}
	name := vs.RandomLocalName()
	// gensym always live; sticky no invent empty-name parent-local shell
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	v := vs.createAndInitialize(access, cg, t, varQfer, block, name, r)
	if v == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:944 — blk->local_vars.push_back(var)
	// create_and_initialize does not push locals (fair with C++)
	block.LocalVars = append(block.LocalVars, v)
	// VariableSelector.cpp:945–946 — FM on collective
	// Incomplete GetCollective must not invent success without facts (AddNewVarFactAndUpdate(nil) no-ops)
	// Incomplete GlobalFacts after register must not invent create success
	if cg.FM != nil {
		coll := varCollective(v)
		if coll == nil {
			if n := len(block.LocalVars); n > 0 && block.LocalVars[n-1] == v {
				block.LocalVars = block.LocalVars[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
		cg.FM.AddNewVarFactAndUpdate(block, coll)
		if !FactsComplete(cg.FM.GlobalFacts) {
			if n := len(block.LocalVars); n > 0 && block.LocalVars[n-1] == v {
				block.LocalVars = block.LocalVars[:n-1]
			}
			SetError(ErrGeneric)
			return nil
		}
	}
	// wrap_volatiles for Output
	if vs.Opts.WrapVolatiles && v.IsVolatile() {
		v.UseVolRVal = true
	}
	vs.VarCreated = true
	return v
}

// SelectArray mirrors VariableSelector::select_array.
// VariableSelector.cpp:1384–1436 — visible non-itemized arrays; else create_random_array.
// SelectArray mirrors VariableSelector::select_array.
// VariableSelector.cpp:1384–1436 — visible collective arrays with effect filters.
func (vs *VariableSelector) SelectArray(r *Rng, cg CGContext) *ArrayVariable {
	// VariableSelector always has VS + RNG; sticky no invent array select without them
	if vs == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before filters (no invent soft re-pick past holes)
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
	blk := cg.CurrentBlock()
	vars := vs.FindAllVisibleVars(blk)
	// incomplete GlobalList/LocalVars hole — fail closed sticky (no invent array select soft nil)
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return nil
	}
	// also include Arrays list members that may not be on GlobalList yet
	seen := map[*ArrayVariable]bool{}
	var arrayVars []*ArrayVariable
	add := func(av *ArrayVariable) {
		if av == nil || av.Collective != nil || seen[av] {
			return
		}
		// VariableSelector.cpp:1393–1412
		if cg.EffectContext().IsReadPartially(&av.Variable) ||
			cg.EffectContext().IsWrittenPartially(&av.Variable) {
			return
		}
		if !cg.EffectContext().IsSideEffectFree() && av.IsVolatile() {
			return
		}
		if av.IsConst() {
			return
		}
		if cg.IsNonWritable(&av.Variable) {
			return
		}
		if av.Type != nil && av.Type.IsConstStructUnion() {
			return
		}
		if vs.Opts.StrictVolatileRule && av.IsVolatile() {
			return
		}
		seen[av] = true
		arrayVars = append(arrayVars, av)
	}
	// Variable* always live in visible list; nil hole fails closed sticky (FindAll already)
	for _, v := range vars {
		if v == nil {
			SetError(ErrGeneric)
			return nil
		}
		if !v.IsArray {
			continue
		}
		// IsArray without AsArray is incomplete IR — fail closed sticky
		// (no invent soft-skip broken array as absent then pick another)
		if v.AsArray == nil {
			SetError(ErrGeneric)
			return nil
		}
		add(v.AsArray)
	}
	// ArrayVariable* on Arrays list; nil hole fails closed sticky
	for _, av := range vs.Arrays {
		if av == nil {
			SetError(ErrGeneric)
			return nil
		}
		add(av)
	}
	n := len(arrayVars)
	// VariableSelector.cpp:1428–1435
	if n == 0 {
		return vs.CreateRandomArray(r, cg)
	}
	if n == 1 {
		return arrayVars[0]
	}
	idx := r.RndUpto(uint32(n))
	// VariableSelector.cpp:1434 ERROR_GUARD(nullptr)
	if HasError() {
		return nil
	}
	return arrayVars[idx]
}

// CreateRandomArray mirrors VariableSelector::create_random_array.
// VariableSelector.cpp:1340–1379 — global 25%; choose_random_nonvoid(_nonvolatile);
// skip const struct/union and !accept_type; DFA new-var facts.
// CreateArrayVariable (C++ ArrayVariable.cpp:190–191) also registers GlobalList/local.
func (vs *VariableSelector) CreateRandomArray(r *Rng, cg CGContext) *ArrayVariable {
	// VariableSelector always has VS + RNG; sticky no invent random array without them
	if vs == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before flipcoin (no invent soft re-pick)
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
	// VariableSelector.cpp:1341–1342 — global_variables && rnd_flipcoin(25); ERROR_GUARD
	asGlobal := vs.Opts.GlobalVariables && r.RndFlipcoin(25)
	if HasError() {
		return nil
	}
	// Go MaxGlobals library cap applies to global array creates too
	if asGlobal && vs.atMaxGlobals() {
		return nil
	}
	var name string
	var blk *Block
	if asGlobal {
		name = vs.RandomGlobalName()
	} else {
		// VariableSelector.cpp:1348–1354 — RandomLocalName + rnd_upto(stack.size())
		// empty stack: C++ OOB / ERROR; no soft invent blk=nil local array
		if cg.CurrentFunc == nil || len(cg.CurrentFunc.Stack) == 0 {
			return nil
		}
		// incomplete Stack fails closed sticky (no invent soft-skip nil frame)
		if !BlocksComplete(cg.CurrentFunc.Stack) {
			SetError(ErrGeneric)
			return nil
		}
		name = vs.RandomLocalName()
		idx := r.RndUpto(uint32(len(cg.CurrentFunc.Stack)))
		if HasError() {
			return nil
		}
		blk = cg.CurrentFunc.Stack[idx]
		// VariableSelector.cpp:1353–1354 — expand_block_for_goto
		blk = ExpandBlockForGoto(blk, cg)
		if blk == nil {
			return nil
		}
	}
	// gensym always live; sticky no invent empty-name array shell
	if name == "" {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1356–1361 — do while const_struct_union || !accept_type
	// C++ loops until success or ERROR_GUARD; cap high (no soft invent nil early)
	// no soft invent GetSimpleType when Types nil — fail closed like ERROR_GUARD
	var elem *Type
	for tries := 0; tries < 256; tries++ {
		if vs.Types == nil {
			return nil
		}
		if asGlobal {
			elem = vs.Types.ChooseRandomNonvoid(r, vs.Opts, vs.Probs)
		} else {
			elem = vs.Types.ChooseRandomNonvoidNonvolatile(r, vs.Opts, vs.Probs)
		}
		// VariableSelector.cpp:1360 ERROR_GUARD
		if HasError() {
			return nil
		}
		if elem == nil {
			continue
		}
		if elem.IsConstStructUnion() {
			continue
		}
		// VariableSelector.cpp:1361 — cg_context.accept_type(type)
		if !cg.AcceptType(elem) {
			continue
		}
		break
	}
	// VariableSelector.cpp:1351–1361 — no soft invent int element after failed picks
	if elem == nil {
		return nil
	}
	// VariableSelector.cpp:1362–1363 — qfer.add_qualifiers(false, false)
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	// VariableSelector.cpp:1364 — Constant::make_random(type); ERROR_GUARD path
	// no invent CreateArrayVariable with nil init when make_random fails
	init := MakeRandom(elem, vs.Opts, vs.Probs, r)
	if init == nil || HasError() {
		return nil
	}
	av := CreateArrayVariable(r, vs.Opts, vs.Probs, vs, &cg, blk, name, elem, init, qfer)
	if av == nil || HasError() {
		return nil
	}
	// VariableSelector.cpp:1368 — AllVars
	vs.AllVars = append(vs.AllVars, &av.Variable)
	vs.Arrays = append(vs.Arrays, av)
	// ArrayVariable.cpp:190–191 — CreateArrayVariable already registered GlobalList/local
	// VariableSelector.cpp:1371–1377 — DFA facts + new_globals (not a second list push)
	if asGlobal {
		if !av.IsVolatile() {
			vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, &av.Variable)
		}
		if cg.CurrentFunc != nil {
			cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, &av.Variable)
		}
		if cg.FM != nil {
			cg.FM.AddNewVarFactAndUpdate(nil, &av.Variable)
			// Incomplete GlobalFacts after register must not invent create success
			if !FactsComplete(cg.FM.GlobalFacts) {
				SetError(ErrGeneric)
				return nil
			}
		}
	} else if blk != nil {
		if cg.FM != nil {
			cg.FM.AddNewVarFactAndUpdate(blk, &av.Variable)
			if !FactsComplete(cg.FM.GlobalFacts) {
				SetError(ErrGeneric)
				return nil
			}
		}
	}
	vs.VarCreated = true
	return av
}

// VariableScope mirrors eVariableScope.
type VariableScope int

const (
	ScopeGlobal VariableScope = iota
	ScopeParentLocal
	ScopeParentParam
	ScopeNewValue
	MaxVarScope
)

// NewScopeThresholdTable mirrors InitScopeTable / VariableSelectionProbability table.
// VariableSelector.cpp:112–121 — with globals: 35 Global, 65 Local, 95 Param, 100 New;
// without: 50 Local, 95 Param, 100 New.
func NewScopeThresholdTable(opts Options) *ThresholdTable {
	t := &ThresholdTable{}
	if opts.GlobalVariables {
		t.Add(35, int(ScopeGlobal))
		t.Add(65, int(ScopeParentLocal))
		t.Add(95, int(ScopeParentParam))
		t.Add(100, int(ScopeNewValue))
	} else {
		t.Add(50, int(ScopeParentLocal))
		t.Add(95, int(ScopeParentParam))
		t.Add(100, int(ScopeNewValue))
	}
	return t
}

// variableSelectFilter mirrors VariableSelectFilter.
// VariableSelector.cpp:98–105 — reject eParentParam when parent.param.empty().
// Filter true = reject (re-roll rnd_upto).
// tab must be the live scopeTable_ (same instance as VariableSelectionProbabilityCG).
func variableSelectFilter(tab *ThresholdTable, cg *CGContext) Filter {
	return filterFunc(func(v uint32) bool {
		if tab == nil {
			// no soft invent scope mapping without InitScopeTable
			return true
		}
		sc := VariableScope(tab.GetValue(int(v)))
		if sc != ScopeParentParam {
			return false
		}
		// VariableSelector.cpp:100–102 — empty params → filter out ParentParam
		if cg == nil || cg.CurrentFunc == nil || len(cg.CurrentFunc.Param) == 0 {
			return true
		}
		return false
	})
}

// VariableSelectionProbability mirrors VariableSelectionProbability without filter
// (tests / call sites that only need unfiltered draw).
// VariableSelector.cpp:1043–1059 — upper=MAX, filter=nullptr.
func VariableSelectionProbability(r *Rng, opts Options) VariableScope {
	return VariableSelectionProbabilityCG(r, opts, nil, MaxVarScope)
}

// VariableSelectionProbabilityCG mirrors VariableSelectionProbability(upper, filter).
// VariableSelector.cpp:1043–1059 — do { rnd_upto(100, filter); if scope < upper return }.
func VariableSelectionProbabilityCG(r *Rng, opts Options, cg *CGContext, upper VariableScope) VariableScope {
	// VariableSelector.cpp:1053 — ERROR_GUARD(MAX_VAR_SCOPE) sticky; no soft invent ScopeNewValue
	if r == nil {
		SetError(ErrGeneric)
		return MaxVarScope
	}
	// incomplete Param fails closed sticky (no invent filter ParentParam via len-hole)
	if cg != nil && cg.CurrentFunc != nil && !VariablesComplete(cg.CurrentFunc.Param) {
		SetError(ErrGeneric)
		return MaxVarScope
	}
	// VariableSelector.cpp:1050 — InitScopeTable(); use process scopeTable_ only
	// (no invent NewScopeThresholdTable per draw)
	tab := ProcessScopeTab()
	if tab == nil {
		// library path without InitScopeTable — sticky ERROR_GUARD MAX
		_ = opts
		SetError(ErrGeneric)
		return MaxVarScope
	}
	filt := variableSelectFilter(tab, cg)
	// C++ unbounded do-while; cap high (no soft invent MAX early)
	for tries := 0; tries < 256; tries++ {
		i := r.RndUptoFilter(100, filt)
		if HasError() {
			return MaxVarScope
		}
		sc := VariableScope(tab.GetValue(int(i)))
		if sc < 0 {
			return MaxVarScope
		}
		// VariableSelector.cpp:1055–1057 — scope < upper
		if sc < upper {
			return sc
		}
	}
	// VariableSelector.cpp:1059
	return MaxVarScope
}

// VariableCreationProbability mirrors VariableCreationProbability.
// VariableSelector.cpp:1063–1070 — flipcoin(10) global if allowed else local.
func VariableCreationProbability(r *Rng, opts Options) VariableScope {
	// VariableSelector.cpp:1065 — ERROR_GUARD(MAX_VAR_SCOPE) sticky; no soft invent ParentLocal without RNG
	if r == nil {
		SetError(ErrGeneric)
		return MaxVarScope
	}
	flag := opts.GlobalVariables && r.RndFlipcoin(10)
	// VariableSelector.cpp:1065 ERROR_GUARD after flipcoin
	if HasError() {
		return MaxVarScope
	}
	if flag {
		return ScopeGlobal
	}
	return ScopeParentLocal
}

// Select mirrors VariableSelector::select.
// VariableSelector.cpp:1189–1243 — pick scope; bookkeep new/old; reject vol in non-SE-free.
func (vs *VariableSelector) Select(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	return vs.SelectWithInvalid(access, cg, t, qfer, r, mt, nil)
}

// SelectWithInvalid is select with invalid_vars list.
// VariableSelector.cpp:1189–1243.
func (vs *VariableSelector) SelectWithInvalid(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars []*Variable,
) *Variable {
	// select always has VS + RNG sticky; no invent select shell without them
	if vs == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before scope pick (no invent select past holes)
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
	// incomplete invalid_vars fails closed sticky (no invent select past hole list)
	if !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1190–1191 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtSelectVariable, scope, nullptr)
	// scope not chosen yet → MAX_VAR_SCOPE flag (mirrors call with default MAX)
	if DepthGuardByTypeFlag(vs.Opts, DtSelectVariable, int(MaxVarScope)) == BadDepth {
		return nil
	}
	vs.VarCreated = false
	// VariableSelector.cpp:1192–1196 — VariableSelectFilter + VariableSelectionProbability(MAX, &filter)
	scope := VariableSelectionProbabilityCG(r, vs.Opts, &cg, MaxVarScope)
	// VariableSelector.cpp:1196 ERROR_GUARD
	if scope == MaxVarScope || HasError() {
		return nil
	}
	var v *Variable
	switch scope {
	case ScopeGlobal:
		v = vs.SelectGlobalMT(access, cg, t, qfer, r, mt, invalidVars)
	case ScopeParentLocal:
		v = vs.SelectParentLocalInv(access, cg, t, qfer, r, mt, invalidVars)
	case ScopeParentParam:
		v = vs.SelectParentParamInv(access, cg, t, qfer, r, mt, invalidVars)
	case ScopeNewValue:
		// VariableSelector.cpp:1217–1219 — GenerateNewVariable; expand_struct sets ERROR
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
		if vs.Opts.ExpandStruct {
			// ERROR_GUARD(nullptr) after switch discards selection
			SetError(ErrGeneric)
			return nil
		}
	case MaxVarScope:
		// VariableSelector.cpp:1222–1223 — assert(0) sticky; no soft invent GenerateNewVariable
		SetError(ErrGeneric)
		return nil
	default:
		// unknown scope sticky — no soft invent create
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1224 — ERROR_GUARD(nullptr); null scope pick stays null (no soft create)
	if HasError() {
		return nil
	}
	// VariableSelector.cpp:1225–1227 — non-SE-free context: assert(!is_volatile())
	// non-sticky null soft re-pick (sticky poisons generation when vol slips through filter)
	if v != nil && !cg.EffectContext().IsSideEffectFree() && v.IsVolatile() {
		return nil
	}
	// VariableSelector.cpp:1229–1239 — record statistics
	if v != nil {
		if vs.VarCreated {
			RecordVarCreated(v)
		} else {
			RecordVarReused()
		}
	}
	return v
}

// SelectParentLocal mirrors VariableSelector::SelectParentLocal.
// VariableSelector.cpp:987–1041 — rnd stack index; expand_struct on empty; choose_var or create.
func (vs *VariableSelector) SelectParentLocal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	return vs.SelectParentLocalInv(access, cg, t, qfer, r, mt, nil)
}

// SelectParentLocalInv is SelectParentLocal with invalid_vars.
func (vs *VariableSelector) SelectParentLocalInv(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars []*Variable,
) *Variable {
	// VariableSelector always has VS + RNG; sticky no invent parent-local select without them
	if vs == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// no CurrentFunc: soft re-pick (select scopes without live func stack)
	if cg.CurrentFunc == nil {
		return nil
	}
	// incomplete ambient / facts fail closed sticky before stack pick (no invent soft re-pick)
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
	// VariableSelector.cpp:989 — DEPTH_GUARD_BY_TYPE_RETURN(dtSelectParentLocal, nullptr)
	if DepthGuardByType(vs.Opts, DtSelectParentLocal) == BadDepth {
		return nil
	}
	// VariableSelector.cpp:991–996 — assert(!stack.empty()); no soft invent global/param
	stack := cg.CurrentFunc.Stack
	if len(stack) == 0 {
		return nil
	}
	// incomplete Stack list fails closed sticky (no invent soft-skip nil frame)
	if !BlocksComplete(stack) {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete invalid_vars fails closed sticky
	if !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1001–1003 — rnd_upto(stack.size()); ERROR_GUARD(nullptr)
	blk := stack[r.RndUpto(uint32(len(stack)))]
	if HasError() {
		return nil
	}
	if blk == nil {
		SetError(ErrGeneric)
		return nil
	}
	// empty locals: expand_struct eager then GenerateNewParentLocal
	// VariableSelector.cpp:1007–1013 — eager_create_local_struct(…, invalid_vars)
	if len(blk.LocalVars) == 0 {
		if vs.Opts.ExpandStruct {
			if v := vs.EagerCreateLocalStruct(blk, access, cg, t, qfer, r, mt, invalidVars); v != nil {
				return v
			}
			// VariableSelector.cpp:1009–1010 — ERROR_GUARD after eager_create
			if HasError() {
				return nil
			}
		}
		// VariableSelector.cpp:1013–1015 — random_type_from_type(type, true, false); ERROR_GUARD
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true, false)
		if t2 == nil || HasError() {
			return nil
		}
		return vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
	}
	// incomplete LocalVars fails closed sticky before choose
	if !VariablesComplete(blk.LocalVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1019–1028 — simple nonvoid → match as int; else random_type_from_type no_vol
	matchT := t
	if t != nil && t.IsSimple() && t.Simple() != EVoid {
		// VariableSelector.cpp:1019–1020 — get_int_type() (upstream type widen for locals)
		matchT = GetIntType()
	} else {
		// VariableSelector.cpp:1021–1023 — random_type_from_type(type, true, false); ERROR_GUARD
		matchT = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true, false)
		// no soft invent keep original type when random_type_from_type fails
		if matchT == nil || HasError() {
			return nil
		}
	}
	// VariableSelector.cpp:1026–1028 — choose_var; ERROR_GUARD
	v := ChooseVarFull(r, blk.LocalVars, access, cg, matchT, qfer, mt, invalidVars, false, false, false)
	if HasError() {
		return nil
	}
	if v != nil {
		return v
	}
	// VariableSelector.cpp:1038 — GenerateNewParentLocal on miss
	return vs.GenerateNewParentLocal(blk, access, cg, matchT, qfer, r)
}

// SelectParentParam mirrors VariableSelector::SelectParentParam.
// VariableSelector.cpp:1074–1087 — choose param; empty/miss → SelectParentLocal.
func (vs *VariableSelector) SelectParentParam(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	return vs.SelectParentParamInv(access, cg, t, qfer, r, mt, nil)
}

// SelectParentParamInv is SelectParentParam with invalid_vars.
// VariableSelector.cpp:1074–1087.
func (vs *VariableSelector) SelectParentParamInv(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
	invalidVars []*Variable,
) *Variable {
	if cg.CurrentFunc == nil {
		return nil
	}
	// incomplete ambient / facts fail closed sticky before param choose (no invent soft re-pick)
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
	// incomplete Param list fails closed sticky (no invent soft-skip hole as absent param)
	if !VariablesComplete(cg.CurrentFunc.Param) {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete invalid_vars fails closed sticky
	if !VariablesComplete(invalidVars) {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp always has process RNG for multi-choose / parent-local create
	// n==1 param can skip draw; multi without r sticky fail closed
	if r == nil && len(cg.CurrentFunc.Param) != 1 {
		SetError(ErrGeneric)
		return nil
	}
	// VariableSelector.cpp:1079–1080 — empty param → SelectParentLocal
	if len(cg.CurrentFunc.Param) == 0 {
		return vs.SelectParentLocalInv(access, cg, t, qfer, r, mt, invalidVars)
	}
	v := ChooseVarFull(r, cg.CurrentFunc.Param, access, cg, t, qfer, mt, invalidVars, false, false, false)
	// VariableSelector.cpp:1082 ERROR_GUARD
	if HasError() {
		return nil
	}
	// VariableSelector.cpp:1083–1086 — miss → SelectParentLocal
	if v != nil {
		return v
	}
	return vs.SelectParentLocalInv(access, cg, t, qfer, r, mt, invalidVars)
}

// GenerateNewVariable mirrors VariableSelector::GenerateNewVariable.
// VariableSelector.cpp:1090–1140 — VariableCreationProbability → global or parent local.
func (vs *VariableSelector) GenerateNewVariable(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	// VariableSelector.cpp:1090+ — always has VS + type + RNG; sticky no invent without them
	if vs == nil || t == nil || r == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky before scope pick (no invent new var past holes)
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
	// VariableSelector.cpp:1093 — DEPTH_GUARD_BY_TYPE_RETURN(dtGenerateNewVariable, nullptr)
	if DepthGuardByType(vs.Opts, DtGenerateNewVariable) == BadDepth {
		return nil
	}
	scope := VariableCreationProbability(r, vs.Opts)
	// VariableSelector.cpp:1096–1097 — ERROR_GUARD(nullptr) when creation scope is MAX
	if scope == MaxVarScope || HasError() {
		return nil
	}
	var v *Variable
	switch scope {
	case ScopeGlobal:
		// VariableSelector.cpp:1100 — DEPTH_GUARD_BY_TYPE_RETURN(dtGenerateNewGlobal, nullptr)
		if DepthGuardByType(vs.Opts, DtGenerateNewGlobal) == BadDepth {
			return nil
		}
		// VariableSelector.cpp:1104–1107 — !is_random && GlobalList.empty → ERROR
		// library random mode always has is_random; skip dfs_exhaustive invent
		// VariableSelector.cpp:1108 — random_type_from_type(type) defaults
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, false, false)
		// VariableSelector.cpp:1109 ERROR_GUARD
		if t2 == nil || HasError() {
			return nil
		}
		v = vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	case ScopeParentLocal:
		// VariableSelector.cpp:1114–1115 — DEPTH_GUARD_BY_DEPTH for parent-local create
		if DepthGuardByDepth(vs.Opts, MinimalDepth(DtGenerateNewParentLocal, 0)) == BadDepth {
			return nil
		}
		if r == nil {
			return nil
		}
		if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
			// VariableSelector.cpp:1116–1117 — rnd_upto(func.stack.size()); ERROR_GUARD
			blk := cg.CurrentFunc.Stack[r.RndUpto(uint32(len(cg.CurrentFunc.Stack)))]
			if HasError() {
				return nil
			}
			// VariableSelector.cpp:1126 — random_type_from_type(type, true, false)
			t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true, false)
			// VariableSelector.cpp:1127 ERROR_GUARD
			if t2 == nil || HasError() {
				return nil
			}
			v = vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
		} else {
			// empty stack: C++ would rnd_upto(0); library → nil (no soft invent global)
			return nil
		}
	default:
		// only Global / ParentLocal from VariableCreationProbability
		return nil
	}
	// VariableSelector.cpp:1135 ERROR_GUARD; 1136 var_created
	if v == nil || HasError() {
		return nil
	}
	vs.VarCreated = true
	return v
}
