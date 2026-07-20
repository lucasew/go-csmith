// Upstream: FunctionInvocation::permute_param_oders / visit_unordered_params;
// FunctionInvocationUser::revisit / save_return_fact;
// Fact::renew_fact / renew_facts; return-fact registry.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// return-fact registry (FunctionInvocationUser.cpp static invocations/return_facts).
var (
	returnFactInvocations []*Invocation
	returnFactPoints      []*FactPointTo
)

// AddReturnFactForInvocation mirrors add_return_fact_for_invocation.
// FunctionInvocationUser.cpp:91–102 — assert(invocations.size() == return_facts.size()).
// Incomplete PointTo fails closed (no invent registry of broken facts).
// Incomplete invocation registry slots fail closed wipe (no invent soft-skip
// nil inv and still match/re-seed a later parallel slot).
func AddReturnFactForInvocation(fi *Invocation, f *FactPointTo) {
	// incomplete inv/fact sticky (no invent registry of broken facts / soft re-pick)
	if fi == nil || f == nil || !FactsComplete([]*FactPointTo{f}) {
		SetError(ErrGeneric)
		return
	}
	// keep parallel slices; desync is broken IR sticky — reset rather than invent
	if len(returnFactInvocations) != len(returnFactPoints) {
		returnFactInvocations = nil
		returnFactPoints = nil
		SetError(ErrGeneric)
		return
	}
	for i, inv := range returnFactInvocations {
		// nil registry slot sticky — clear, no invent re-seed
		if inv == nil || returnFactPoints[i] == nil {
			returnFactInvocations = nil
			returnFactPoints = nil
			SetError(ErrGeneric)
			return
		}
		if inv == fi && returnFactPoints[i].Var == f.Var {
			returnFactPoints[i] = f
			return
		}
	}
	returnFactInvocations = append(returnFactInvocations, fi)
	returnFactPoints = append(returnFactPoints, f)
}

// GetReturnFactForInvocation mirrors get_return_fact_for_invocation (point-to).
// FunctionInvocationUser.cpp:76–91 — assert parallel sizes.
// Incomplete Invocation/Variable/registry sticky nil (no invent soft-skip hole to later match).
func GetReturnFactForInvocation(fi *Invocation, v *Variable) *FactPointTo {
	// Invocation + subject always live; sticky incomplete no invent miss soft-skip
	if fi == nil || v == nil {
		SetError(ErrGeneric)
		return nil
	}
	if len(returnFactInvocations) != len(returnFactPoints) {
		SetError(ErrGeneric)
		return nil
	}
	for i, inv := range returnFactInvocations {
		// Invocation* always live on registry; nil hole sticky
		if inv == nil || returnFactPoints[i] == nil {
			SetError(ErrGeneric)
			return nil
		}
		if inv != fi {
			continue
		}
		if returnFactPoints[i].Var == v {
			return returnFactPoints[i]
		}
	}
	return nil
}

// InvocationReturnFactsDoFinalization mirrors FunctionInvocationUser::doFinalization.
// FunctionInvocationUser.cpp:368–371.
func InvocationReturnFactsDoFinalization() {
	returnFactInvocations = nil
	returnFactPoints = nil
}

// SaveReturnFacts mirrors FunctionInvocationUser::save_return_fact.
// FunctionInvocationUser.cpp:358–365 — facts matching func.rv.
// Incomplete maps fail closed (no invent soft-skip holes and still save later).
func (fi *Invocation) SaveReturnFacts(facts []*FactPointTo) {
	// nil fi/User is hard IR sticky; nil RV is complete no-op (void/non-pointer return)
	if fi == nil || fi.User == nil {
		SetError(ErrGeneric)
		return
	}
	if fi.User.RV == nil {
		return
	}
	// incomplete maps sticky (no invent soft-skip holes and still save later)
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return
	}
	for _, f := range facts {
		// Fact* always live after FactsComplete
		if f == nil || f.Var == nil {
			SetError(ErrGeneric)
			return
		}
		if fi.User.RV.Match(f.Var) {
			// residual ERROR sticky — no invent soft-continue save past Match hole
			if HasError() {
				return
			}
			AddReturnFactForInvocation(fi, f)
			// residual ERROR sticky — no invent soft-continue registry past Add hole
			if HasError() {
				return
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-skip not-match then save later
			return
		}
	}
}

// RenewFact mirrors renew_fact — replace related or append.
// Fact.cpp:175–191.
// Fact* always live; incomplete subject map or nf PointTo fails closed
// (*facts IncompleteFactSlice, false — no invent renew / leave incomplete as no-op).
func RenewFact(facts *[]*FactPointTo, nf *FactPointTo) bool {
	// facts pointer always live for renew; sticky incomplete no invent no-op soft-skip
	if facts == nil {
		SetError(ErrGeneric)
		return false
	}
	if nf == nil || nf.Var == nil {
		// incomplete renew wiped sticky (no invent soft re-pick past hole)
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete([]*FactPointTo{nf}) {
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false
	}
	for i, f := range *facts {
		// Fact* always live after FactsComplete
		if f == nil || f.Var == nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return false
		}
		if f.Var == nf.Var || f.Var.Match(nf.Var) {
			// residual ERROR sticky — no invent soft-continue later match past Match hole
			if HasError() {
				*facts = IncompleteFactSlice()
				return false
			}
			if f.Equal(nf) {
				// residual ERROR sticky — no invent no-change soft-success past Equal hole
				if HasError() {
					*facts = IncompleteFactSlice()
					return false
				}
				return false
			}
			// residual ERROR sticky — no invent replace past Equal hole
			if HasError() {
				*facts = IncompleteFactSlice()
				return false
			}
			(*facts)[i] = nf
			return true
		}
		// residual ERROR sticky — no invent soft-skip not-match then renew later
		if HasError() {
			*facts = IncompleteFactSlice()
			return false
		}
	}
	*facts = append(*facts, nf)
	return true
}

// RenewFacts mirrors renew_facts.
// Fact.cpp:203–210.
// Incomplete maps fail closed (*facts nil, false — no invent partial renew).
func RenewFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	// facts pointer always live for renew; sticky incomplete no invent no-op soft-skip
	if facts == nil {
		SetError(ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		// incomplete maps wiped sticky (no invent soft re-pick past hole)
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if RenewFact(facts, nf) {
			// residual ERROR sticky — no invent soft-continue partial renew past hole
			if HasError() {
				*facts = IncompleteFactSlice()
				return false
			}
			changed = true
		} else if HasError() {
			// residual ERROR sticky — no invent soft-skip failed renew then renew later
			*facts = IncompleteFactSlice()
			return false
		}
	}
	return changed
}

// PermuteParamOrders mirrors FunctionInvocation::permute_param_oders.
// FunctionInvocation.cpp:418–455 — for 2 args both orders; else permute call-bearing slots.
// Returns sequences of arg indices.
// Empty result matches C++ when n!=2 and no call-bearing params (permute empty base).
// FunctionInvocation.cpp:462 — assert(orders.size() > 0) on visit_unordered path.
func (fi *Invocation) PermuteParamOrders() [][]int {
	// Invocation always live; sticky incomplete no invent empty orders soft-skip
	if fi == nil {
		SetError(ErrGeneric)
		return nil
	}
	n := len(fi.Args)
	// FunctionInvocation.cpp:424–432 — shortcut for 2 parameters
	if n == 2 {
		return [][]int{{0, 1}, {1, 0}}
	}
	// FunctionInvocation.cpp:434–453 — base order; only call-bearing slots permute
	// util.cpp:85–90 — permute(empty) → empty out; no soft invent identity order
	base := make([]int, 0, n)
	retBase := make([]int, n)
	for i := 0; i < n; i++ {
		retBase[i] = i
		// param_value[i] always live; nil / incomplete FuncCount sticky fail closed
		// (no invent skip hole as non-call when building permute slots)
		if fi.Args[i] == nil {
			SetError(ErrGeneric)
			return nil
		}
		fc := FuncCount(fi.Args[i])
		if fc < 0 {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		if fc > 0 {
			base = append(base, i)
		}
	}
	// all permutations of call-bearing positions (capped)
	perms := permuteInts(base)
	if len(perms) == 0 {
		// empty base → empty orders (C++); visit_unordered asserts size>0
		return nil
	}
	if len(perms) > 24 {
		perms = perms[:24]
	}
	var ret [][]int
	for _, newSeq := range perms {
		tmp := append([]int(nil), retBase...)
		// plug: for j, orig_pos=base[j], tmp[orig_pos]=newSeq[j]
		// upstream: tmp[orig_pos] = new_pos where new_pos = new_seq[j]
		for j := 0; j < len(newSeq) && j < len(base); j++ {
			origPos := base[j]
			newPos := newSeq[j]
			tmp[origPos] = newPos
		}
		ret = append(ret, tmp)
	}
	return ret
}

// permuteInts all permutations of a (heap's algorithm), small n only.
// util.cpp:85–90 — empty input → empty out (no soft invent one empty order).
func permuteInts(a []int) [][]int {
	if len(a) == 0 {
		return nil
	}
	if len(a) == 1 {
		return [][]int{{a[0]}}
	}
	var out [][]int
	var gen func([]int, int)
	gen = func(arr []int, k int) {
		if k == 1 {
			cp := append([]int(nil), arr...)
			out = append(out, cp)
			return
		}
		for i := 0; i < k; i++ {
			gen(arr, k-1)
			if k%2 == 0 {
				arr[i], arr[k-1] = arr[k-1], arr[i]
			} else {
				arr[0], arr[k-1] = arr[k-1], arr[0]
			}
		}
	}
	cp := append([]int(nil), a...)
	gen(cp, len(cp))
	return out
}

// VisitUnorderedParams mirrors FunctionInvocation::visit_unordered_params.
// FunctionInvocation.cpp:457–480 — visit args in all orders; merge facts.
// Hard IR incomplete sticky (nil call/facts, empty orders, nil args, incomplete maps);
// VisitFactsExpression policy fails stay non-sticky false.
func (fi *Invocation) VisitUnorderedParams(facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// FunctionInvocation.cpp:457+ — always live this + facts sticky
	if fi == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete working facts sticky (no invent cleaned permute base / soft re-pick)
	if !FactsComplete(*facts) {
		SetError(ErrGeneric)
		return false
	}
	inputsCopy := CloneFactSlice(*facts)
	orders := fi.PermuteParamOrders()
	// FunctionInvocation.cpp:462 — assert(orders.size() > 0) sticky
	if len(orders) == 0 {
		SetError(ErrGeneric)
		return false
	}
	var merged []*FactPointTo
	for i, order := range orders {
		cur := CloneFactSlice(inputsCopy)
		for _, paramID := range order {
			if paramID < 0 || paramID >= len(fi.Args) {
				SetError(ErrGeneric)
				return false
			}
			arg := fi.Args[paramID]
			// param_value[i] always non-null after ERROR_GUARD sticky
			if arg == nil {
				SetError(ErrGeneric)
				return false
			}
			// visit under working facts
			if cg.FM != nil {
				cg.FM.GlobalFacts = cur
			}
			if !VisitFactsExpression(arg, cg, opts) {
				return false
			}
			if cg.FM != nil {
				// incomplete GlobalFacts after arg visit sticky
				if !FactsComplete(cg.FM.GlobalFacts) {
					if !HasError() {
						SetError(ErrGeneric)
					}
					return false
				}
				cur = CloneFactSlice(cg.FM.GlobalFacts)
			}
		}
		if i == 0 {
			merged = cur
		} else {
			if !FactsComplete(merged) || !FactsComplete(cur) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			// MergeFacts sticky on incomplete mid-join
			_ = MergeFacts(&merged, cur)
			if !FactsComplete(merged) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
	}
	if !FactsComplete(merged) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	*facts = merged
	if cg.FM != nil {
		cg.FM.GlobalFacts = merged
	}
	return true
}

// NeedsRevisit reports whether build_invocation would re-analyze the callee.
// FunctionInvocationUser.cpp:274–276.
// Incomplete Function sticky false (no invent not-revisit soft-skip past hole).
func (f *Function) NeedsRevisit() bool {
	// Function always live; sticky incomplete no invent not-revisit soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	if f.FactChanged || f.UnionFieldRead {
		return true
	}
	// residual ERROR sticky — no invent not-revisit soft-skip past IsPointerReferenced hole
	ref := f.IsPointerReferenced()
	if HasError() {
		// incomplete ReferencedPtrs already sticky true; keep residual restrictive
		return true
	}
	return ref
}

// IsPointerReferenced mirrors Function::is_pointer_referenced.
// Function.h:110.
// Incomplete ReferencedPtrs sticky true (NeedsRevisit — no invent
// "no pointers" via VariablesComplete(nil)/len==0 empty-complete).
func (f *Function) IsPointerReferenced() bool {
	// Function always live; sticky incomplete no invent no-ptrs soft-skip
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	if !VariablesComplete(f.ReferencedPtrs) {
		// incomplete ReferencedPtrs sticky has-pointers (restrictive revisit)
		SetError(ErrGeneric)
		return true
	}
	return len(f.ReferencedPtrs) > 0
}

// RevisitUserInvocation mirrors FunctionInvocationUser::revisit.
// FunctionInvocationUser.cpp:309–352.
// Hard IR incomplete sticky (nil call/body/FM/facts, StmID 0, incomplete maps);
// VisitFactsBlock policy fails stay non-sticky (restore maps without new SetError).
func RevisitUserInvocation(fi *Invocation, facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// FunctionInvocationUser.cpp:309+ — get_fact_mgr_for_func + body always live sticky
	if fi == nil || fi.User == nil || facts == nil || cg == nil {
		SetError(ErrGeneric)
		return false
	}
	f := fi.User
	if f.Body == nil {
		SetError(ErrGeneric)
		return false
	}
	// FunctionInvocationUser.cpp:311 — FactMgr *fm = get_fact_mgr_for_func(func);
	// Must use the callee's paired FactMgr (map_facts_in/out for body stmts), not
	// the caller's cg.FM. Using caller maps made VisitFactsBlock fail on nested
	// calls in if-conditions, fixed-point stripped the if, and func->blocks lost
	// nested arms (seed-2 e2342: goto nblocks 8 vs UP 10).
	fm := f.PairedFactMgr()
	if fm == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete caller facts sticky (no invent cleaned revisit / soft re-pick)
	if !FactsComplete(*facts) {
		SetError(ErrGeneric)
		return false
	}
	// backup maps
	inCopy := cloneFactMap(fm.MapFactsIn)
	outCopy := cloneFactMap(fm.MapFactsOut)
	effCopy := cloneEffectMap(fm.MapStmEffect)
	accCopy := cloneEffectMap(fm.MapAccumEffect)
	// FunctionInvocationUser.cpp:315 — inputs_copy = inputs (pre-handover caller lattice).
	// Deep-clone so handover/body cannot orphan mid-gen may-null on the live *facts slice
	// when *facts aliases caller GlobalFacts (build_invocation passes global_facts by ref).
	inputsCopy := CloneFactSlice(*facts)
	// Working lattice for handover + body visit. C++ mutates `inputs` in place for the
	// visit_facts walk; we keep a separate work slice so callee FactMgr.GlobalFacts is
	// never aliased to the caller's GlobalFacts slice (that alias polluted callee FM
	// and could leave caller on a post-handover slice without frame locals).
	work := CloneFactSlice(*facts)

	restore := func() {
		fm.MapFactsIn = inCopy
		fm.MapFactsOut = outCopy
		fm.MapStmEffect = effCopy
		fm.MapAccumEffect = accCopy
		*facts = inputsCopy
	}

	fm.ClearMapVisited()
	f.VisitedCnt++
	if f.VisitedCnt == 1 {
		fm.SetupInOutMaps(true)
	}
	// handover params into working lattice (not directly into caller GlobalFacts)
	fm.CallerToCalleeHandover(fi.Args, &work)
	if !FactsComplete(work) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// visit body — install work into callee FM only for the duration of VisitFactsBlock
	// (C++ visit_facts mutates the inputs vector; Go visit uses FM.GlobalFacts).
	savedGlobal := fm.GlobalFacts
	fm.GlobalFacts = work
	bodyCG := cg.CloneSubcontext()
	bodyCG.CurrentFunc = f
	bodyCG.FM = fm
	ok := VisitFactsBlock(f.Body, &bodyCG, opts)
	if !ok {
		// policy / body visit fail — restore maps; analysis fail is soft (C++ log_analysis_fail
		// returns false without leaving permanent Error for caller generation). Sticky
		// ERROR from incomplete IR during nested visit would poison subsequent soft paths.
		restore()
		fm.GlobalFacts = savedGlobal
		ClearError()
		return false
	}
	// incomplete body GlobalFacts sticky
	if !FactsComplete(fm.GlobalFacts) {
		restore()
		fm.GlobalFacts = savedGlobal
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	work = CloneFactSlice(fm.GlobalFacts)
	// Restore callee GlobalFacts immediately — do not leave caller lattice installed.
	fm.GlobalFacts = savedGlobal
	// body Block::stm_id always live; StmID 0 sticky
	if f.Body.StmID <= 0 {
		restore()
		SetError(ErrGeneric)
		return false
	}
	// Incomplete body map_stm_effect sticky
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	cg.AddEffect(bodyEff, false)
	// residual ERROR sticky — no invent soft-continue ret-facts past AddEffect residual
	if HasError() {
		restore()
		return false
	}
	if !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// FunctionInvocationUser.cpp: ret from map_facts_out[body] + add_back_return_facts
	bodyOut := fm.GetMapFactsOut(f.Body.StmID)
	if !FactsComplete(bodyOut) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	retFacts := CloneFactSlice(bodyOut)
	if !AddBackReturnFacts(f.Body, fm, &retFacts) || !FactsComplete(retFacts) || !FactsComplete(work) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	fi.SaveReturnFacts(retFacts)
	_ = MergeFacts(&work, retFacts)
	if !FactsComplete(work) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// drop param locals OOS
	UpdateFactsForOOSVars(f.Param, &work)
	if !FactsComplete(work) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	fm.SetupInOutMaps(false)
	// Incomplete external merge sticky
	if !EffectComplete(cg.EffectContext()) || !EffectComplete(f.AccumEffContext) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	f.AccumEffContext = f.AccumEffContext.AddExternalEffect(cg.EffectContext())
	if !EffectComplete(f.AccumEffContext) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	// FunctionInvocationUser.cpp:348–350 — renew_facts(inputs_copy, inputs); inputs = inputs_copy
	// Restore pre-handover caller lattice (incl. frame-local may-null) then apply body deltas.
	_ = RenewFacts(&inputsCopy, work)
	if !FactsComplete(inputsCopy) {
		restore()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	*facts = inputsCopy
	return true
}

func cloneFactMap(m map[int][]*FactPointTo) map[int][]*FactPointTo {
	if m == nil {
		return make(map[int][]*FactPointTo)
	}
	out := make(map[int][]*FactPointTo, len(m))
	for k, v := range m {
		// incomplete → hole marker (not bare nil invent empty complete)
		out[k] = storeFactMapEntry(v)
	}
	return out
}

func cloneEffectMap(m map[int]Effect) map[int]Effect {
	if m == nil {
		return make(map[int]Effect)
	}
	out := make(map[int]Effect, len(m))
	for k, v := range m {
		cp := v.Clone()
		// residual ERROR sticky — no invent soft-clone map past IncompleteEffect residual
		if HasError() {
			return make(map[int]Effect)
		}
		out[k] = cp
	}
	return out
}

// GetQualifiers mirrors FunctionInvocation::get_qualifiers.
// FunctionInvocation.cpp:482–498 — user rv qfer; else non-const non-vol.
// Incomplete Invocation / user RV fails closed sticky empty (no invent
// storage-level non-const non-vol shell / soft re-pick past holes).
func (fi *Invocation) GetQualifiers() CVQualifiers {
	// FunctionInvocation always live; sticky incomplete no invent default quals
	if fi == nil {
		SetError(ErrGeneric)
		return CVQualifiers{}
	}
	// FunctionInvocation.cpp:486–491 — eFuncCall: assert(func); assert(rv)
	if fi.User != nil {
		// assert(rv) path — incomplete RV sticky empty qfer
		// (no invent NewCVQualifiers(false,false) storage-level shell)
		if fi.User.RV == nil {
			SetError(ErrGeneric)
			return CVQualifiers{}
		}
		return fi.User.RV.Qfer
	}
	// binary/unary — non-const non-volatile int-like (C++ default for std ops)
	return NewCVQualifiers([]bool{false}, []bool{false})
}
