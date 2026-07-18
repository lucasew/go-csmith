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

// NewVariableSelector constructs an empty selector with opts-derived probs.
func NewVariableSelector(opts Options) *VariableSelector {
	return &VariableSelector{
		Opts:  opts,
		Probs: NewProbabilities(opts),
	}
}

// RandomGlobalName mirrors VariableSelector.cpp RandomGlobalName → gensym("g_").
func (vs *VariableSelector) RandomGlobalName() string {
	return vs.Sym.Next("g_")
}

// RandomLocalName mirrors RandomLocalName → gensym("l_").
func (vs *VariableSelector) RandomLocalName() string {
	return vs.Sym.Next("l_")
}

// RandomParamName mirrors RandomParamName → gensym("p_").
func (vs *VariableSelector) RandomParamName() string {
	return vs.Sym.Next("p_")
}

// ChooseVisibleReadVar mirrors VariableSelector::choose_visible_read_var.
// VariableSelector.cpp:361–377 — expand structs; match convert; on stack or global; not vol.
func ChooseVisibleReadVar(
	r *Rng,
	b *Block,
	readVars []*Variable,
	typ *Type,
	unionFacts []*FactUnion,
) *Variable {
	// VariableSelector.cpp:363 — type from caller (goto uses get_int_type); no invent
	if typ == nil {
		return nil
	}
	expanded := ExpandStructUnionVars(append([]*Variable(nil), readVars...), typ)
	var ok []*Variable
	for _, v := range expanded {
		if v == nil || v.Type == nil || v.IsVirtual() || v.IsVolatile() {
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
	return ChooseOKVar(r, ok)
}

// FindVarByName mirrors VariableSelector::find_var_by_name.
// VariableSelector.cpp:1571–1579 — scan AllVars via match_var_name.
func (vs *VariableSelector) FindVarByName(name string) *Variable {
	if vs == nil || name == "" {
		return nil
	}
	for _, v := range vs.AllVars {
		if m := v.MatchVarName(name); m != nil {
			return m
		}
	}
	// also search arrays' Variable wrappers
	for _, av := range vs.Arrays {
		if av == nil {
			continue
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
			if iv == nil || bound == InvalidIVBound {
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
			ArraySizes: av.Sizes,
			ArrayInits: av.ArrayInits,
		},
		Sizes:      av.Sizes,
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
func ChooseOKVar(r *Rng, vars []*Variable) *Variable {
	n := len(vars)
	if n == 0 {
		return nil
	}
	var v *Variable
	if n == 1 {
		v = vars[0]
	} else {
		// VariableSelector.cpp:324 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr)
		// random mode always GOOD; still call for fair wiring (use Defaults opts)
		if DepthGuardByDepth(Defaults(), 1) == BadDepth {
			return nil
		}
		// VariableSelector.cpp:326–329 — rnd_upto(len); no soft invent vars[0] without RNG
		if r == nil {
			return nil
		}
		v = vars[r.RndUpto(uint32(n))]
	}
	// if collective array, return itemized member (VariableSelector.cpp:332–337)
	if v != nil && v.IsArray && v.AsArray != nil && v.AsArray.Collective == nil && r != nil {
		if item := v.AsArray.Itemize(r); item != nil {
			return &item.Variable
		}
	}
	return v
}

// ChooseOKVarExactType filters vars whose Type matches want with eExact.
func ChooseOKVarExactType(r *Rng, vars []*Variable, want *Type) *Variable {
	return ChooseOKVarMatch(r, vars, want, MatchExact, false)
}

// FindAllVisibleVars mirrors VariableSelector::find_all_visible_vars.
// VariableSelector.cpp:752–759 — GlobalList + block chain locals (no params).
func (vs *VariableSelector) FindAllVisibleVars(b *Block) []*Variable {
	var vars []*Variable
	if vs != nil {
		vars = append(vars, vs.GlobalList...)
	}
	for b != nil {
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
			if p := walk(st.Then); p != nil {
				return p
			}
			if p := walk(st.Else); p != nil {
				return p
			}
		}
		return nil
	}
	return walk(root)
}

// findStmtByIDInTree finds a statement by stm_id under root (self + nested blocks).
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
			if s := walk(st.Then); s != nil {
				return s
			}
			if s := walk(st.Else); s != nil {
				return s
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
	root := rootBlock(b)
	for {
		expanded := false
		for _, e := range fm.CFGEdges {
			if e == nil || e.SrcID <= 0 {
				continue
			}
			src := findStmtByIDInTree(root, e.SrcID)
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
				if b == nil {
					// should not happen if src is in the function tree
					return root
				}
				root = rootBlock(b)
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
func LowerBlockForVars(blks []*Block, vars []*Variable) (blk *Block, remaining []*Variable) {
	remaining = append([]*Variable(nil), vars...)
	for _, b := range blks {
		if b == nil {
			continue
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
func (vs *VariableSelector) FindAllNonArrayVisibleVars(b *Block) []*Variable {
	var vars []*Variable
	if vs != nil {
		for _, v := range vs.GlobalList {
			if v != nil && !v.IsArray {
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
		vars = append(vars, f.Param...)
	}
	for b != nil {
		for _, v := range b.LocalVars {
			if v != nil && !v.IsArray {
				vars = append(vars, v)
			}
		}
		b = b.Parent
	}
	return vars
}

// GetAllLocalVars mirrors VariableSelector::get_all_local_vars.
// VariableSelector.cpp:747–751.
func GetAllLocalVars(b *Block) []*Variable {
	var vars []*Variable
	for b != nil {
		vars = append(vars, b.LocalVars...)
		b = b.Parent
	}
	return vars
}

// GetAllArrayVars mirrors VariableSelector::get_all_array_vars.
// VariableSelector.cpp:737–745 — collect array globals (invalid for ccomp pointer init).
func (vs *VariableSelector) GetAllArrayVars() []*Variable {
	var out []*Variable
	if vs == nil {
		return out
	}
	for _, v := range vs.GlobalList {
		if v != nil && v.IsArray {
			out = append(out, v)
		}
	}
	// Arrays list may hold collectives not yet on GlobalList
	for _, av := range vs.Arrays {
		if av == nil {
			continue
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
	if vs == nil || t == nil || r == nil {
		return nil
	}
	// assert qf sanity: allow nil → empty non-wildcard
	var qfer CVQualifiers
	if qf != nil {
		qfer = *qf
	} else {
		qfer = NewCVQualifiers([]bool{false}, []bool{false})
	}
	// VariableSelector.cpp:833 — initializer must not be stricter than var
	qfer.AcceptStricter = false

	// VariableSelector.cpp:836–841 — non-pointer or 20% chance → constant
	if !t.IsPointerLike() || r.RndFlipcoin(20) {
		if t.IsSimple() && t.simple == EVoid {
			return nil
		}
		c := MakeRandom(t, vs.Opts, r)
		if c == nil {
			return nil
		}
		return &Expression{Term: TermConstant, Con: c, ExprType: t}
	}

	// pointer path: select visible var of pointee type
	pointee := t.PtrType()
	if pointee == nil {
		return nil
	}
	vars := vs.FindAllVisibleVars(b)
	noUnion := !vs.Opts.TakeUnionFieldAddr
	var invalid []*Variable
	var chosen *Variable
	// VariableSelector.cpp:853–862
	if b == nil && vs.Opts.CComp {
		invalid = vs.GetAllArrayVars()
		chosen = ChooseVarFull(r, vars, access, cg, pointee, &qfer, MatchExact,
			invalid, true, true, noUnion)
	} else {
		if !vs.Opts.AddrTakenOfLocals {
			invalid = GetAllLocalVars(b)
		}
		chosen = ChooseVarFull(r, vars, access, cg, pointee, &qfer, MatchExact,
			invalid, true, false, noUnion)
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
		var tt *Type
		if useLocal {
			tt = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, pointee, true)
		} else {
			tt = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, pointee, false)
		}
		if tt == nil {
			return nil
		}
		if vs.Opts.AddrTakenOfLocals && useLocal && b != nil {
			chosen = vs.GenerateNewParentLocal(b, AccessRead, cg, tt, &qferDeref, r)
			if chosen != nil && chosen.Type != nil {
				RecordVolatileAccess(chosen, chosen.Type.IndirectLevel()-tt.IndirectLevel(), false)
			}
		} else {
			if vs.Opts.CComp {
				chosen = vs.GenerateNewNonArrayGlobal(AccessRead, cg, tt, &qferDeref, r)
			} else {
				chosen = vs.GenerateNewGlobal(AccessRead, cg, tt, &qferDeref, r)
			}
		}
		if chosen == nil {
			return nil
		}
		RecordAddressTaken(chosen)
	} else if chosen.Type != nil {
		// VariableSelector.cpp:905–909
		derefLevel := chosen.Type.IndirectLevel() - t.IndirectLevel()
		if derefLevel < 0 {
			RecordAddressTaken(chosen)
		}
	}
	// ExpressionVariable(*var, t) — desired type is pointer being inited
	return &Expression{Term: TermVariable, Var: chosen, ExprType: t}
}

// IsEligibleVar mirrors VariableSelector::is_eligible_var (core effect rules).
// VariableSelector.cpp:216–290 — itemized collective: read_indices first;
// then effect/const/volatile/FactUnion nonreadable checks on collective.
func IsEligibleVar(v *Variable, derefLevel int, access Access, cg CGContext) bool {
	if v == nil {
		return false
	}
	// VariableSelector.cpp:221–227 — itemized member → read_indices then use collective
	coll := v.GetCollective()
	if coll != v {
		cgp := cg
		var facts []*FactPointTo
		if cg.FM != nil {
			facts = cg.FM.GlobalFacts
		}
		if !cgp.ReadIndices(v, facts) {
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
func HasEligibleVolatileVarQfer(vars []*Variable, typ *Type, qfer *CVQualifiers, access Access, cg CGContext) bool {
	for _, v := range vars {
		if v == nil || v.Type == nil {
			continue
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
func HasDereferenceableVar(vars []*Variable, typ *Type, cg CGContext, opts Options) bool {
	if typ == nil {
		return false
	}
	var facts []*FactPointTo
	if cg.FM != nil {
		facts = cg.FM.GlobalFacts
	}
	for _, v := range vars {
		if v == nil || v.Type == nil {
			continue
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
	if vs == nil || cg.RW == nil || typ == nil {
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
	for i := 0; i < len(*list); i++ {
		v := (*list)[i]
		if v == nil {
			continue
		}
		// is_visible (VariableSelector.cpp:1523)
		if !v.IsVisible(blk) {
			continue
		}
		if v.Type == nil || !typ.Match(v.Type, mt) {
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
		if v.IsArray && v.AsArray != nil && r != nil {
			// VariableSelector.cpp:1545–1546 — itemize_array only (no random fallback)
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
	if want == nil {
		var ok []*Variable
		for _, v := range vars {
			if v == nil {
				continue
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
	}
	// VariableSelector.cpp:420–421 — has_eligible_volatile_var (side-effect: volatile_avail)
	_ = HasEligibleVolatileVarQfer(cands, want, qfer, access, cg)
	// VariableSelector.cpp:412–419 — pointer_avail_for_dereference bookkeeping
	opts := Defaults()
	if HasDereferenceableVar(cands, want, cg, opts) {
		RecordPointerAvailForDeref()
	}
	matchExact := false
	var ok []*Variable
	for _, x := range cands {
		if x == nil || x.Type == nil {
			continue
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
func chooseVarFromOK(r *Rng, want *Type, ok []*Variable, opts Options) *Variable {
	// VariableSelector.cpp:459–471 — artificially increase odds of dereferencing
	if want != nil && len(ok) > 1 {
		var ptrs []*Variable
		wantInd := want.IndirectLevel()
		for _, vv := range ok {
			if vv == nil || vv.Type == nil {
				continue
			}
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
			if vv == nil || vv.Type == nil {
				continue
			}
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
func ExpandStructUnionVars(vars []*Variable, want *Type) []*Variable {
	out := append([]*Variable(nil), vars...)
	for i := 0; i < len(out); i++ {
		v := out[i]
		if v == nil || v.IsVirtual() {
			continue
		}
		// don't break up a struct if it matches the given type
		if v.Type != nil && v.Type.IsAggregate() && v.Type != want {
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
	var ok []*Variable
	for _, x := range cands {
		if x == nil || x.Type == nil {
			continue
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
// VariableSelector.cpp:518–536 — array flip or scalar; make_init_value for init.
func (vs *VariableSelector) createAndInitialize(
	access Access,
	cg CGContext,
	t *Type,
	qfer CVQualifiers,
	blk *Block,
	name string,
	r *Rng,
) *Variable {
	if vs == nil || t == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:525–530 — NewArrayVariableProb → create_array_and_itemize
	if vs.Opts.Arrays && r.RndFlipcoin(uint32(vs.Probs.Single(PNewArrayVariableProb))) {
		var init *Constant
		if vs.Opts.StrictConstArrays {
			init = MakeRandom(t, vs.Opts, r)
		} else {
			if ie := vs.MakeInitValue(access, cg, t, &qfer, blk, r); ie != nil && ie.Term == TermConstant {
				init = ie.Con
			} else {
				init = MakeRandom(t, vs.Opts, r)
			}
		}
		av := CreateArrayVariable(r, vs.Opts, blk, name, t, init, qfer)
		if av != nil {
			vs.AllVars = append(vs.AllVars, &av.Variable)
			if blk != nil {
				blk.LocalVars = append(blk.LocalVars, &av.Variable)
			}
			// store full array on a side list for emission
			vs.Arrays = append(vs.Arrays, av)
			vs.VarCreated = true
			return &av.Variable
		}
	}
	v := CreateVariableQfer(name, t, qfer)
	if v == nil {
		return nil
	}
	// VariableSelector.cpp:532–534 — make_init_value
	applyInitExpr(v, vs.MakeInitValue(access, cg, t, &qfer, blk, r))
	// VariableSelector.cpp:568–569 — access_once flip (on GenerateNewGlobal path;
	// keep for create_and_initialize parity when used from local/global create)
	if vs.Opts.AccessOnce && vs.Probs != nil && r.RndFlipcoin(uint32(vs.Probs.Single(PAccessOnceVariableProb))) {
		v.IsAccessOnce = true
	}
	// wrap_volatiles → VOL_RVAL on Output
	if vs.Opts.WrapVolatiles && v.IsVolatile() {
		v.UseVolRVal = true
	}
	if blk != nil {
		blk.LocalVars = append(blk.LocalVars, v)
	}
	vs.AllVars = append(vs.AllVars, v)
	// FactMgr::add_new_var_fact_and_update_inout_maps when FM present
	if cg.FM != nil {
		cg.FM.AddNewVarFactAndUpdate(blk, v)
	}
	vs.VarCreated = true
	return v
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
	if vs == nil || t == nil || r == nil {
		return nil
	}
	if !vs.Opts.GlobalVariables {
		return nil
	}
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, false, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	name := vs.RandomGlobalName()
	vs.TmpCount++
	// always non-array: make_init + new_variable (skip array flip)
	v := CreateVariableQfer(name, t, varQfer)
	if v == nil {
		return nil
	}
	applyInitExpr(v, vs.MakeInitValue(access, cg, t, &varQfer, nil, r))
	vs.AllVars = append(vs.AllVars, v)
	vs.GlobalList = append(vs.GlobalList, v)
	if !varQfer.IsVolatile() {
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, v)
	}
	if cg.FM != nil {
		cg.FM.AddNewVarFactAndUpdate(nil, v)
	}
	vs.VarCreated = true
	return v
}

// GenerateNewGlobal mirrors VariableSelector::GenerateNewGlobal for simple types:
// random_qualifiers (or copy qfer), RandomGlobalName, CreateVariable, push GlobalList.
// VariableSelector.cpp:546–575.
func (vs *VariableSelector) GenerateNewGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	if vs == nil || t == nil {
		return nil
	}
	if !vs.Opts.GlobalVariables {
		return nil
	}
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		// CVQualifiers::random_qualifiers(t, access, cg, false)
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, false, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	name := vs.RandomGlobalName()
	vs.TmpCount++
	v := vs.createAndInitialize(access, cg, t, varQfer, nil, name, r)
	if v == nil {
		return nil
	}
	vs.GlobalList = append(vs.GlobalList, v)
	if !varQfer.IsVolatile() {
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	// VariableSelector.cpp:565 — current_func()->new_globals
	if cg.CurrentFunc != nil {
		cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, v)
	}
	// VariableSelector.cpp:1230–1236 — use_new_var + struct depth / union stats
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
	if vs == nil {
		return nil
	}
	// choose_var(GlobalList, …, mt, invalid_vars)
	v := ChooseVarFull(r, vs.GlobalList, access, cg, t, qfer, mt, invalidVars, false, false, false)
	if v != nil {
		return v
	}
	// VariableSelector.cpp:677–684 — expand_struct eager path
	if vs.Opts.ExpandStruct {
		if v = vs.EagerCreateGlobalStruct(access, cg, t, qfer, r, mt); v != nil {
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
	t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, noVol)
	if t2 == nil {
		t2 = t
	}
	return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
}

// chooseRandomStructFromType mirrors Type::choose_random_struct_from_type.
// Type.cpp:570–586 — if ok structs exist pick one; else return original type.
func chooseRandomStructFromType(env *TypeEnv, typ *Type, noVolatile bool, r *Rng) *Type {
	if typ == nil || r == nil {
		return typ
	}
	cands := okStructUnionLTypes(env, noVolatile, true, true)
	if len(cands) == 0 {
		return typ
	}
	st := cands[r.RndUpto(uint32(len(cands)))]
	if st == nil {
		return typ
	}
	return st
}

// EagerCreateGlobalStruct mirrors VariableSelector::eager_create_global_struct.
// VariableSelector.cpp:607–633 — create a random ok struct global, then choose_var field.
func (vs *VariableSelector) EagerCreateGlobalStruct(
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if vs == nil || typ == nil || r == nil {
		return nil
	}
	level := typ.IndirectLevel()
	if level > 1 {
		return nil
	}
	st := chooseRandomStructFromType(vs.Types, typ, false, r)
	if st == nil {
		return nil
	}
	// level 0: create struct; level 1: still create struct (fields may match *T after expand)
	_ = vs.GenerateNewGlobal(access, cg, st, qfer, r)
	return ChooseVar(r, vs.GlobalList, access, cg, typ, mt)
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
) *Variable {
	if vs == nil || block == nil || typ == nil || r == nil {
		return nil
	}
	level := typ.IndirectLevel()
	if level > 1 {
		return nil
	}
	// choose_random_struct_from_type(type, true) — no_volatile for locals
	st := chooseRandomStructFromType(vs.Types, typ, true, r)
	if st == nil {
		return nil
	}
	_ = vs.GenerateNewParentLocal(block, access, cg, st, qfer, r)
	return ChooseVar(r, block.LocalVars, access, cg, typ, mt)
}

// GenerateParameterVariableTyped mirrors
// VariableSelector::GenerateParameterVariable(type, qfer).
// VariableSelector.cpp:955–957.
func (vs *VariableSelector) GenerateParameterVariableTyped(typ *Type, qfer CVQualifiers) *Variable {
	if vs == nil {
		return nil
	}
	name := vs.RandomParamName()
	v := CreateVariableQfer(name, typ, qfer)
	if v == nil {
		return nil
	}
	vs.AllVars = append(vs.AllVars, v)
	return v
}

// GenerateParameterVariable mirrors VariableSelector::GenerateParameterVariable(Function&).
// VariableSelector.cpp:963–979 — 40% pointer when derived exist; else nonvoid nonvolatile;
// CGOptions arg_structs / arg_unions gates.
func (vs *VariableSelector) GenerateParameterVariable(f *Function, r *Rng) *Variable {
	if vs == nil || f == nil || r == nil {
		return nil
	}
	var t *Type
	// VariableSelector.cpp:966–972 — has_pointer_type() && flipcoin(40)
	if vs.Types != nil && vs.Types.HasPointerType() && r.RndFlipcoin(40) {
		// Type::choose_random_pointer_type — pick existing derived pointer
		t = vs.Types.ChooseRandomPointerType(r)
		if t == nil {
			t = vs.Types.MakeRandomPointerType(r, vs.Opts, vs.Probs)
		}
	} else if vs.Types != nil && len(vs.Types.AllTypes) > 0 {
		// Type::choose_random_nonvoid_nonvolatile (filters arg_structs/arg_unions)
		t = vs.Types.ChooseRandomNonvoidNonvolatile(r, vs.Opts, vs.Probs)
	} else {
		st := ChooseRandomNonvoidSimple(r, vs.Probs)
		t = GetSimpleType(st)
	}
	// VariableSelector.cpp:973–976 — ERROR_RETURN on null; assert non-void simple
	// no soft invent GetIntType (arg_structs filter is in NonVoidNonVolatileTypeFilter)
	if t == nil {
		return nil
	}
	if t.IsSimple() && t.Simple() == EVoid {
		return nil
	}
	// CVQualifiers::random_qualifiers(t) — no context
	qfer := RandomQualifiersNoContextNoVolatile(t, vs.Opts, vs.Probs, r)
	v := vs.GenerateParameterVariableTyped(t, qfer)
	if v != nil {
		f.Param = append(f.Param, v)
	}
	return v
}

// SelectLoopCtrlVar mirrors VariableSelector::SelectLoopCtrlVar.
// VariableSelector.cpp:1146–1179 — non-array visible, has_int_field, drop union+ptr;
// choose_var(WRITE, eConvert, invalid, no_bitfield=true).
func (vs *VariableSelector) SelectLoopCtrlVar(r *Rng, cg CGContext, invalid map[*Variable]bool) *Variable {
	if vs == nil || r == nil {
		return nil
	}
	ty := GetIntType()
	blk := cg.CurrentBlock()
	vars := vs.FindAllNonArrayVisibleVars(blk)
	// VariableSelector.cpp:1155–1168 — remove no-int-field and union-with-pointer
	var filtered []*Variable
	var invalidSlice []*Variable
	for _, v := range vars {
		if v == nil || v.Type == nil {
			continue
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
	if vs == nil || block == nil || t == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:920–923 — volatile struct/union field(s) → global
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
		// random_qualifiers(t, access, cg, true) — no_volatile true for locals path in some call sites;
		// GenerateNewParentLocal uses random_qualifiers(t, access, cg, true) — third bool is no_volatile.
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, true, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	// restrict(access, cg): WRITE clears const; non-SE-free clears vol
	if access == AccessWrite && len(varQfer.IsConsts) > 0 {
		varQfer.IsConsts[len(varQfer.IsConsts)-1] = false
	}
	if !cg.EffectContext().IsSideEffectFree() && len(varQfer.IsVolatiles) > 0 {
		varQfer.IsVolatiles[len(varQfer.IsVolatiles)-1] = false
	}
	name := vs.RandomLocalName()
	v := vs.createAndInitialize(access, cg, t, varQfer, block, name, r)
	return v
}

// SelectArray mirrors VariableSelector::select_array.
// VariableSelector.cpp:1384–1436 — visible non-itemized arrays; else create_random_array.
// SelectArray mirrors VariableSelector::select_array.
// VariableSelector.cpp:1384–1436 — visible collective arrays with effect filters.
func (vs *VariableSelector) SelectArray(r *Rng, cg CGContext) *ArrayVariable {
	if vs == nil || r == nil {
		return nil
	}
	blk := cg.CurrentBlock()
	vars := vs.FindAllVisibleVars(blk)
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
	for _, v := range vars {
		if v == nil || !v.IsArray {
			continue
		}
		if v.AsArray != nil {
			add(v.AsArray)
		}
	}
	for _, av := range vs.Arrays {
		add(av)
	}
	n := len(arrayVars)
	if n == 0 {
		return vs.CreateRandomArray(r, cg)
	}
	if n == 1 {
		return arrayVars[0]
	}
	return arrayVars[r.RndUpto(uint32(n))]
}

// CreateRandomArray mirrors VariableSelector::create_random_array.
// VariableSelector.cpp:1340–1379 — global 25%; choose_random_nonvoid(_nonvolatile);
// skip const struct/union and !accept_type; DFA new-var facts.
func (vs *VariableSelector) CreateRandomArray(r *Rng, cg CGContext) *ArrayVariable {
	if vs == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:1341 — global_variables && rnd_flipcoin(25)
	asGlobal := vs.Opts.GlobalVariables && r.RndFlipcoin(25)
	var name string
	var blk *Block
	if asGlobal {
		name = vs.RandomGlobalName()
	} else {
		name = vs.RandomLocalName()
		if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
			idx := r.RndUpto(uint32(len(cg.CurrentFunc.Stack)))
			blk = cg.CurrentFunc.Stack[idx]
			// VariableSelector.cpp:1353–1354 — expand_block_for_goto
			blk = ExpandBlockForGoto(blk, cg)
		}
	}
	// VariableSelector.cpp:1356–1361 — do while const_struct_union || !accept_type
	// C++ loops until success or ERROR_GUARD; cap high (no soft invent nil early)
	var elem *Type
	for tries := 0; tries < 256; tries++ {
		if asGlobal {
			if vs.Types != nil {
				elem = vs.Types.ChooseRandomNonvoid(r, vs.Opts, vs.Probs)
			} else {
				elem = GetSimpleType(ChooseRandomNonvoidSimple(r, vs.Probs))
			}
		} else {
			if vs.Types != nil {
				elem = vs.Types.ChooseRandomNonvoidNonvolatile(r, vs.Opts, vs.Probs)
			} else {
				elem = GetSimpleType(ChooseRandomNonvoidSimple(r, vs.Probs))
			}
		}
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
	// VariableSelector.cpp:1362–1363 — non-const non-vol storage qfer
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	init := MakeRandom(elem, vs.Opts, r)
	av := CreateArrayVariable(r, vs.Opts, blk, name, elem, init, qfer)
	if av == nil {
		return nil
	}
	// VariableSelector.cpp:1368 — AllVars
	vs.AllVars = append(vs.AllVars, &av.Variable)
	vs.Arrays = append(vs.Arrays, av)
	// VariableSelector.cpp:1371–1377 — DFA facts + inventory
	if asGlobal {
		vs.GlobalList = append(vs.GlobalList, &av.Variable)
		if !av.IsVolatile() {
			vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, &av.Variable)
		}
		if cg.CurrentFunc != nil {
			cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, &av.Variable)
		}
		if cg.FM != nil {
			// VariableSelector.cpp:1373–1374
			cg.FM.AddNewVarFactAndUpdate(nil, &av.Variable)
		}
	} else if blk != nil {
		blk.LocalVars = append(blk.LocalVars, &av.Variable)
		if cg.FM != nil {
			// VariableSelector.cpp:1376–1377
			cg.FM.AddNewVarFactAndUpdate(blk, &av.Variable)
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

// VariableSelectionProbability mirrors VariableSelectionProbability.
// VariableSelector.cpp:1043–1059 — rnd_upto(100); scopeTable get_value.
func VariableSelectionProbability(r *Rng, opts Options) VariableScope {
	if r == nil {
		return ScopeNewValue
	}
	tab := NewScopeThresholdTable(opts)
	v := r.RndUpto(100)
	sc := tab.GetValue(int(v))
	if sc < 0 {
		return ScopeNewValue
	}
	return VariableScope(sc)
}

// VariableCreationProbability mirrors VariableCreationProbability.
// VariableSelector.cpp:1063–1070 — flipcoin(10) global if allowed else local.
func VariableCreationProbability(r *Rng, opts Options) VariableScope {
	if opts.GlobalVariables && r != nil && r.RndFlipcoin(10) {
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
	if vs == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:1190–1191 — DEPTH_GUARD_BY_TYPE_RETURN_WITH_FLAG(dtSelectVariable, scope, nullptr)
	// scope not chosen yet → MAX_VAR_SCOPE flag (mirrors call with default MAX)
	if DepthGuardByTypeFlag(vs.Opts, DtSelectVariable, int(MaxVarScope)) == BadDepth {
		return nil
	}
	vs.VarCreated = false
	scope := VariableSelectionProbability(r, vs.Opts)
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
	default:
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
	}
	// if scope pick failed (e.g. no params), fall through to create
	if v == nil {
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
	}
	// VariableSelector.cpp:1225–1227 — non-SE-free context: assert(!is_volatile())
	if v != nil && !cg.EffectContext().IsSideEffectFree() && v.IsVolatile() {
		// must not return volatile under impure effect_context
		vs.VarCreated = false
		v2 := vs.GenerateNewVariable(access, cg, t, qfer, r)
		if v2 != nil && !v2.IsVolatile() {
			v = v2
		} else {
			v = nil
		}
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
	if vs == nil || cg.CurrentFunc == nil || r == nil {
		return nil
	}
	// VariableSelector.cpp:989 — DEPTH_GUARD_BY_TYPE_RETURN(dtSelectParentLocal, nullptr)
	if DepthGuardByType(vs.Opts, DtSelectParentLocal) == BadDepth {
		return nil
	}
	stack := cg.CurrentFunc.Stack
	if len(stack) == 0 {
		return nil
	}
	// VariableSelector.cpp:1001–1003 — rnd_upto(stack.size())
	blk := stack[r.RndUpto(uint32(len(stack)))]
	if blk == nil {
		return nil
	}
	// empty locals: expand_struct eager then GenerateNewParentLocal
	if len(blk.LocalVars) == 0 {
		if vs.Opts.ExpandStruct {
			if v := vs.EagerCreateLocalStruct(blk, access, cg, t, qfer, r, mt); v != nil {
				return v
			}
		}
		// VariableSelector.cpp:1011 — random_type_from_type(type, true, false) no_vol=true
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true)
		if t2 == nil {
			t2 = t
		}
		return vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
	}
	// VariableSelector.cpp:1019–1028 — simple nonvoid → match as int; else random_type_from_type no_vol
	matchT := t
	if t != nil && t.IsSimple() && t.Simple() != EVoid {
		matchT = GetIntType()
	} else {
		matchT = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true)
		if matchT == nil {
			matchT = t
		}
	}
	if v := ChooseVarFull(r, blk.LocalVars, access, cg, matchT, qfer, mt, invalidVars, false, false, false); v != nil {
		return v
	}
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
	if len(cg.CurrentFunc.Param) == 0 {
		return vs.SelectParentLocalInv(access, cg, t, qfer, r, mt, invalidVars)
	}
	if v := ChooseVarFull(r, cg.CurrentFunc.Param, access, cg, t, qfer, mt, invalidVars, false, false, false); v != nil {
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
	if vs == nil || t == nil {
		return nil
	}
	// VariableSelector.cpp:1093 — DEPTH_GUARD_BY_TYPE_RETURN(dtGenerateNewVariable, nullptr)
	if DepthGuardByType(vs.Opts, DtGenerateNewVariable) == BadDepth {
		return nil
	}
	scope := VariableCreationProbability(r, vs.Opts)
	switch scope {
	case ScopeGlobal:
		// VariableSelector.cpp:1100 — DEPTH_GUARD_BY_TYPE_RETURN(dtGenerateNewGlobal, nullptr)
		if DepthGuardByType(vs.Opts, DtGenerateNewGlobal) == BadDepth {
			return nil
		}
		// VariableSelector.cpp:1105 — random_type_from_type(type) default no_vol=false
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, false)
		if t2 == nil {
			t2 = t
		}
		return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	default:
		// VariableSelector.cpp:1114–1115 — DEPTH_GUARD_BY_DEPTH for parent-local create
		if DepthGuardByDepth(vs.Opts, MinimalDepth(DtGenerateNewParentLocal, 0)) == BadDepth {
			return nil
		}
		if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
			// VariableSelector.cpp:1118 — rnd_upto(func.stack.size())
			blk := cg.CurrentFunc.Stack[r.RndUpto(uint32(len(cg.CurrentFunc.Stack)))]
			// VariableSelector.cpp:1129 — random_type_from_type(type, true, false)
			t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true)
			if t2 == nil {
				t2 = t
			}
			return vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
		}
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, false)
		if t2 == nil {
			t2 = t
		}
		return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	}
}
