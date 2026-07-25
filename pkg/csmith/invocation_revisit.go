// Upstream: FunctionInvocation::permute_param_oders / visit_unordered_params;
// FunctionInvocationUser::revisit / save_return_fact;
// Fact::renew_fact / renew_facts; return-fact registry.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// return-fact registry (FunctionInvocationUser.cpp static invocations/return_facts).
// C++ stores Fact* of any eCat; Go splits ePointTo / eUnionWrite into parallel registries
// (same inv may appear once per category — FunctionInvocationUser.cpp:91–102).

// AddReturnFactForInvocation mirrors add_return_fact_for_invocation for ePointTo.
// FunctionInvocationUser.cpp:91–102 — assert(invocations.size() == return_facts.size()).
// Incomplete PointTo fails closed (no invent registry of broken facts).
// Incomplete invocation registry slots fail closed wipe (no invent soft-skip
// nil inv and still match/re-seed a later parallel slot).
func AddReturnFactForInvocation(fi *Invocation, f *FactPointTo) {
	AddReturnFactForInvocationSess(nil, fi, f)
}

// AddReturnFactForInvocationSess is AddReturnFactForInvocation on an explicit session bag.
func AddReturnFactForInvocationSess(s *Session, fi *Invocation, f *FactPointTo) {
	s = sessOrAmbient(s)
	// incomplete inv/fact sticky (no invent registry of broken facts / soft re-pick)
	if fi == nil || f == nil || !FactsComplete([]*FactPointTo{f}) {
		sessNoteError(s, ErrGeneric)
		return
	}
	// keep parallel slices; desync is broken IR sticky — reset rather than invent
	if len(s.ReturnFactInvocations) != len(s.ReturnFactPoints) {
		s.ReturnFactInvocations = nil
		s.ReturnFactPoints = nil
		sessNoteError(s, ErrGeneric)
		return
	}
	for i, inv := range s.ReturnFactInvocations {
		// nil registry slot sticky — clear, no invent re-seed
		if inv == nil || s.ReturnFactPoints[i] == nil {
			s.ReturnFactInvocations = nil
			s.ReturnFactPoints = nil
			sessNoteError(s, ErrGeneric)
			return
		}
		if inv == fi && s.ReturnFactPoints[i].Var == f.Var {
			s.ReturnFactPoints[i] = f
			return
		}
	}
	s.ReturnFactInvocations = append(s.ReturnFactInvocations, fi)
	s.ReturnFactPoints = append(s.ReturnFactPoints, f)
}

// AddReturnUnionFactForInvocation mirrors add_return_fact_for_invocation for eUnionWrite.
// FunctionInvocationUser.cpp:91–102 — same registry, is_related by eUnionWrite + subject.
func AddReturnUnionFactForInvocation(fi *Invocation, f *FactUnion) {
	AddReturnUnionFactForInvocationSess(nil, fi, f)
}

// AddReturnUnionFactForInvocationSess is AddReturnUnionFactForInvocation on an explicit session bag.
func AddReturnUnionFactForInvocationSess(s *Session, fi *Invocation, f *FactUnion) {
	s = sessOrAmbient(s)
	if fi == nil || f == nil || !UnionFactsComplete([]*FactUnion{f}) {
		sessNoteError(s, ErrGeneric)
		return
	}
	if len(s.ReturnUnionInvocations) != len(s.ReturnUnionFacts) {
		s.ReturnUnionInvocations = nil
		s.ReturnUnionFacts = nil
		sessNoteError(s, ErrGeneric)
		return
	}
	for i, inv := range s.ReturnUnionInvocations {
		if inv == nil || s.ReturnUnionFacts[i] == nil {
			s.ReturnUnionInvocations = nil
			s.ReturnUnionFacts = nil
			sessNoteError(s, ErrGeneric)
			return
		}
		// FactUnion::is_related — eUnionWrite + var pointer identity
		if inv == fi && s.ReturnUnionFacts[i].Var == f.Var {
			s.ReturnUnionFacts[i] = f
			return
		}
	}
	s.ReturnUnionInvocations = append(s.ReturnUnionInvocations, fi)
	s.ReturnUnionFacts = append(s.ReturnUnionFacts, f)
}

// GetReturnFactForInvocation mirrors get_return_fact_for_invocation (point-to).
// FunctionInvocationUser.cpp:76–91 — assert parallel sizes; eCat == ePointTo.
// Incomplete Invocation/Variable/registry sticky nil (no invent soft-skip hole to later match).
func GetReturnFactForInvocation(fi *Invocation, v *Variable) *FactPointTo {
	return GetReturnFactForInvocationSess(nil, fi, v)
}

// GetReturnFactForInvocationSess is GetReturnFactForInvocation on an explicit session bag.
func GetReturnFactForInvocationSess(s *Session, fi *Invocation, v *Variable) *FactPointTo {
	s = sessOrAmbient(s)
	// Invocation + subject always live; sticky incomplete no invent miss soft-skip
	if fi == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if len(s.ReturnFactInvocations) != len(s.ReturnFactPoints) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	for i, inv := range s.ReturnFactInvocations {
		// Invocation* always live on registry; nil hole sticky
		if inv == nil || s.ReturnFactPoints[i] == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if inv != fi {
			continue
		}
		if s.ReturnFactPoints[i].Var == v {
			return s.ReturnFactPoints[i]
		}
	}
	return nil
}

// GetReturnUnionFactForInvocation mirrors get_return_fact_for_invocation(…, eUnionWrite).
// FunctionInvocationUser.cpp:76–91; FactUnion.cpp:103–106.
func GetReturnUnionFactForInvocation(fi *Invocation, v *Variable) *FactUnion {
	return GetReturnUnionFactForInvocationSess(nil, fi, v)
}

// GetReturnUnionFactForInvocationSess is GetReturnUnionFactForInvocation on an explicit session bag.
func GetReturnUnionFactForInvocationSess(s *Session, fi *Invocation, v *Variable) *FactUnion {
	s = sessOrAmbient(s)
	if fi == nil || v == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if len(s.ReturnUnionInvocations) != len(s.ReturnUnionFacts) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	for i, inv := range s.ReturnUnionInvocations {
		if inv == nil || s.ReturnUnionFacts[i] == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		if inv != fi {
			continue
		}
		if s.ReturnUnionFacts[i].Var == v {
			return s.ReturnUnionFacts[i]
		}
	}
	return nil
}

// InvocationReturnFactsDoFinalization mirrors FunctionInvocationUser::doFinalization.
// FunctionInvocationUser.cpp:368–371.
func InvocationReturnFactsDoFinalization() {
	InvocationReturnFactsDoFinalizationSess(nil)
}

// InvocationReturnFactsDoFinalizationSess clears return-fact registries on s.
func InvocationReturnFactsDoFinalizationSess(s *Session) {
	s = sessOrAmbient(s)
	s.ReturnFactInvocations = nil
	s.ReturnFactPoints = nil
	s.ReturnUnionInvocations = nil
	s.ReturnUnionFacts = nil
}

// SaveReturnFacts mirrors FunctionInvocationUser::save_return_fact for ePointTo.
// FunctionInvocationUser.cpp:358–365 — facts matching func.rv.
// Incomplete maps fail closed (no invent soft-skip holes and still save later).
func (fi *Invocation) SaveReturnFacts(facts []*FactPointTo) {
	fi.SaveReturnFactsSess(nil, facts)
}

// SaveReturnFactsSess is SaveReturnFacts with explicit session residual sticky.
func (fi *Invocation) SaveReturnFactsSess(s *Session, facts []*FactPointTo) {
	// nil fi/User is hard IR sticky; nil RV is complete no-op (void/non-pointer return)
	if fi == nil || fi.User == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if fi.User.RV == nil {
		return
	}
	// incomplete maps sticky (no invent soft-skip holes and still save later)
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, f := range facts {
		// Fact* always live after FactsComplete
		if f == nil || f.Var == nil {
			sessNoteError(s, ErrGeneric)
			return
		}
		if fi.User.RV.Match(f.Var) {
			// residual ERROR sticky — no invent soft-continue save past Match hole
			if sessHasError(s) {
				return
			}
			AddReturnFactForInvocationSess(s, fi, f)
			// residual ERROR sticky — no invent soft-continue registry past Add hole
			if sessHasError(s) {
				return
			}
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-skip not-match then save later
			return
		}
	}
}

// SaveReturnUnionFacts mirrors save_return_fact for eUnionWrite facts.
// FunctionInvocationUser.cpp:358–365 — full FactVec includes FactUnion for union RV.
func (fi *Invocation) SaveReturnUnionFacts(facts []*FactUnion) {
	fi.SaveReturnUnionFactsSess(nil, facts)
}

// SaveReturnUnionFactsSess is SaveReturnUnionFacts with explicit session residual sticky.
func (fi *Invocation) SaveReturnUnionFactsSess(s *Session, facts []*FactUnion) {
	if fi == nil || fi.User == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if fi.User.RV == nil {
		return
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, f := range facts {
		if f == nil || f.Var == nil {
			sessNoteError(s, ErrGeneric)
			return
		}
		if fi.User.RV.Match(f.Var) {
			if sessHasError(s) {
				return
			}
			AddReturnUnionFactForInvocationSess(s, fi, f)
			if sessHasError(s) {
				return
			}
		} else if sessHasError(s) {
			return
		}
	}
}

// RenewFact mirrors renew_fact — replace related or append.
// Fact.cpp:175–191 — related by Fact::is_related only.
// FactPointTo.h:65–68 — is_related is ePointTo + var pointer identity.
// Soft invent used Variable.Match (aggregate-has-field) so renew of a field
// could replace an earlier aggregate subject's fact and leave the field's
// garbage fact in place (seed-7 l_1402: {garbage,g_1183} dangling on re-read).
// Fact* always live; incomplete subject map or nf PointTo fails closed
// (*facts IncompleteFactSlice, false — no invent renew / leave incomplete as no-op).
func RenewFact(facts *[]*FactPointTo, nf *FactPointTo) bool {
	return RenewFactSess(nil, facts, nf)
}

func RenewFactSess(s *Session, facts *[]*FactPointTo, nf *FactPointTo) bool {
	// facts pointer always live for renew; sticky incomplete no invent no-op soft-skip
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if nf == nil || nf.Var == nil {
		// incomplete renew wiped sticky (no invent soft re-pick past hole)
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete([]*FactPointTo{nf}) {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	for i, f := range *facts {
		// Fact* always live after FactsComplete
		if f == nil || f.Var == nil {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
			return false
		}
		// Fact.cpp:177 — new_fact->is_related(*facts[i]) only (not Variable::match)
		if !f.IsRelatedSess(s, nf) {
			// residual ERROR sticky — no invent soft-skip not-related then renew later
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false
			}
			continue
		}
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		if f.EqualSess(s, nf) {
			// residual ERROR sticky — no invent no-change soft-success past Equal hole
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false
			}
			return false
		}
		// residual ERROR sticky — no invent replace past Equal hole
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false
		}
		(*facts)[i] = nf
		return true
	}
	*facts = append(*facts, nf)
	return true
}

// RenewFacts mirrors renew_facts.
// Fact.cpp:203–210.
// Incomplete maps fail closed (*facts nil, false — no invent partial renew).}

func RenewFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	return RenewFactsSess(nil, facts, newFacts)
}

func RenewFactsSess(s *Session, facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	// facts pointer always live for renew; sticky incomplete no invent no-op soft-skip
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		// incomplete maps wiped sticky (no invent soft re-pick past hole)
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if RenewFactSess(s, facts, nf) {
			// residual ERROR sticky — no invent soft-continue partial renew past hole
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false
			}
			changed = true
		} else if sessHasError(s) {
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
// FunctionInvocation.cpp:462 — assert(orders.size() > 0) on visit_unordered path.}

func (fi *Invocation) PermuteParamOrders() [][]int {
	return fi.PermuteParamOrdersSess(nil)
}

// PermuteParamOrdersSess is PermuteParamOrders with explicit session residual sticky.
func (fi *Invocation) PermuteParamOrdersSess(s *Session) [][]int {
	// Invocation always live; sticky incomplete no invent empty orders soft-skip
	if fi == nil {
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return nil
		}
		fc := FuncCountSess(s, fi.Args[i])
		if fc < 0 {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
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
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// incomplete working facts sticky (no invent cleaned permute base / soft re-pick)
	if !FactsComplete(*facts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	inputsCopy := CloneFactSliceSess(cgSess(cg), *facts)
	orders := fi.PermuteParamOrdersSess(cgSess(cg))
	// FunctionInvocation.cpp:462 — assert(orders.size() > 0) sticky
	if len(orders) == 0 {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	var merged []*FactPointTo
	for i, order := range orders {
		cur := CloneFactSliceSess(cgSess(cg), inputsCopy)
		for _, paramID := range order {
			if paramID < 0 || paramID >= len(fi.Args) {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			arg := fi.Args[paramID]
			// param_value[i] always non-null after ERROR_GUARD sticky
			if arg == nil {
				sessNoteError(cgSess(cg), ErrGeneric)
				return false
			}
			// visit under working facts
			if cg.FM != nil {
				cg.FM.SetGlobalFacts(cur, "auto_invocation_revisit_349")
			}
			if !VisitFactsExpression(arg, cg, opts) {
				return false
			}
			if cg.FM != nil {
				// incomplete GlobalFacts after arg visit sticky
				if !FactsComplete(cg.FM.GlobalFacts) {
					if !sessHasError(cgSess(cg)) {
						sessNoteError(cgSess(cg), ErrGeneric)
					}
					return false
				}
				cur = CloneFactSliceSess(cgSess(cg), cg.FM.GlobalFacts)
			}
		}
		if i == 0 {
			merged = cur
		} else {
			if !FactsComplete(merged) || !FactsComplete(cur) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
			// MergeFacts sticky on incomplete mid-join
			_ = MergeFactsSess(cgSess(cg), &merged, cur)
			if !FactsComplete(merged) {
				if !sessHasError(cgSess(cg)) {
					sessNoteError(cgSess(cg), ErrGeneric)
				}
				return false
			}
		}
	}
	if !FactsComplete(merged) {
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	*facts = merged
	if cg.FM != nil {
		cg.FM.SetGlobalFacts(merged, "auto_invocation_revisit_392")
	}
	return true
}

// NeedsRevisit reports whether build_invocation would re-analyze the callee.
// FunctionInvocationUser.cpp:274–276.
// Incomplete Function sticky false (no invent not-revisit soft-skip past hole).
func (f *Function) NeedsRevisit() bool {
	return f.NeedsRevisitSess(nil)
}

// NeedsRevisitSess is NeedsRevisit with explicit session residual sticky.
func (f *Function) NeedsRevisitSess(s *Session) bool {
	// Function always live; sticky incomplete no invent not-revisit soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if f.FactChanged || f.UnionFieldRead {
		return true
	}
	// residual ERROR sticky — no invent not-revisit soft-skip past IsPointerReferenced hole
	ref := f.IsPointerReferencedSess(s)
	if sessHasError(s) {
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
	return f.IsPointerReferencedSess(nil)
}

// IsPointerReferencedSess is IsPointerReferenced with explicit session residual sticky.
func (f *Function) IsPointerReferencedSess(s *Session) bool {
	// Function always live; sticky incomplete no invent no-ptrs soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !VariablesComplete(f.ReferencedPtrs) {
		// incomplete ReferencedPtrs sticky has-pointers (restrictive revisit)
		sessNoteError(s, ErrGeneric)
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
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	f := fi.User
	if f.Body == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// FunctionInvocationUser.cpp:311 — FactMgr *fm = get_fact_mgr_for_func(func);
	// Must use the callee's paired FactMgr (map_facts_in/out for body stmts), not
	// the caller's cg.FM. Using caller maps made VisitFactsBlock fail on nested
	// calls in if-conditions, fixed-point stripped the if, and func->blocks lost
	// nested arms (seed-2 e2342: goto nblocks 8 vs UP 10).
	fm := f.PairedFactMgrSess(cgSess(cg))
	if fm == nil {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// incomplete caller facts sticky (no invent cleaned revisit / soft re-pick)
	if !FactsComplete(*facts) {
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// backup maps — FactVec partitions (ePointTo + eUnionWrite) + effects
	// FactMgr.cpp map_facts_in/out are full FactVec; Go splits PT/union maps.
	inCopy := cloneFactMap(fm.MapFactsIn)
	outCopy := cloneFactMap(fm.MapFactsOut)
	unionInCopy := cloneUnionFactMapSess(cgSess(cg), fm.MapUnionFactsIn)
	unionOutCopy := cloneUnionFactMapSess(cgSess(cg), fm.MapUnionFactsOut)
	effCopy := cloneEffectMapSess(cgSess(cg), fm.MapStmEffect)
	accCopy := cloneEffectMapSess(cgSess(cg), fm.MapAccumEffect)
	// Live UnionFacts on callee FM (replaced for visit; restored on fail).
	savedUnion := fm.UnionFacts
	// FunctionInvocationUser.cpp:315 — inputs_copy = inputs (pre-handover caller lattice).
	// Deep-clone so handover/body cannot orphan mid-gen may-null on the live *facts slice
	// when *facts aliases caller GlobalFacts (build_invocation passes global_facts by ref).
	inputsCopy := CloneFactSliceSess(cgSess(cg), *facts)
	// Working lattice for handover + body visit. C++ mutates `inputs` in place for the
	// visit_facts walk; we keep a separate work slice so callee FactMgr.GlobalFacts is
	// never aliased to the caller's GlobalFacts slice (that alias polluted callee FM
	// and could leave caller on a post-handover slice without frame locals).
	work := CloneFactSliceSess(cgSess(cg), *facts)

	restore := func() {
		fm.MapFactsIn = inCopy
		fm.MapFactsOut = outCopy
		fm.MapUnionFactsIn = unionInCopy
		fm.MapUnionFactsOut = unionOutCopy
		fm.MapStmEffect = effCopy
		fm.MapAccumEffect = accCopy
		fm.UnionFacts = savedUnion
		*facts = inputsCopy
	}

	fm.ClearMapVisited()
	f.VisitedCnt++
	if f.VisitedCnt == 1 {
		fm.SetupInOutMaps(true)
	}
	// IsValidPtr collective fallback for itemized arrays only while revisiting.
	// Prefer callee FM session / cg.Sess over ambient.
	revisitSess := fmSess(fm)
	if revisitSess == nil {
		revisitSess = cgSess(cg)
	}
	revisitSess = sessOrAmbient(revisitSess)
	prevRevisit := revisitSess.InUserInvocationRevisit
	revisitSess.InUserInvocationRevisit = true
	defer func() { revisitSess.InUserInvocationRevisit = prevRevisit }()
	// FunctionInvocationUser.cpp:206+324 — full FactVec handover includes eUnionWrite.
	// Build path clones caller UnionFacts then FilterUnionFactsForHandover (function_invocation.go).
	// Soft invent left stale callee UnionFacts across revisits → IsNonreadableField /
	// shortcut same_facts skew (seed-7 ChooseOKVar n=26 vs UP n=56).
	// Caller FM: VisitFactsInvocation / BuildUserInvocation pass newCG cloned from parent
	// (FM still caller's). When cg.FM is already the callee, keep existing UnionFacts
	// and only filter after PT partition.
	if cg.FM != nil && cg.FM != fm {
		if !UnionFactsComplete(cg.FM.UnionFacts) {
			restore()
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		clU := CloneUnionFactSlice(cg.FM.UnionFacts)
		if sessHasError(cgSess(cg)) || !UnionFactsComplete(clU) {
			restore()
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		fm.UnionFacts = clU
	} else if !UnionFactsComplete(fm.UnionFacts) {
		restore()
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// handover params into working lattice (not directly into caller GlobalFacts)
	fm.CallerToCalleeHandover(fi.Args, &work)
	if !FactsComplete(work) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// FactMgr.cpp:324–353 — drop union subjects not kept by PT partition
	fm.FilterUnionFactsForHandover(work)
	if sessHasError(cgSess(cg)) || !UnionFactsComplete(fm.UnionFacts) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// visit body — install work into callee FM only for the duration of VisitFactsBlock
	// (C++ visit_facts mutates the inputs vector; Go visit uses FM.GlobalFacts).
	savedGlobal := fm.GlobalFacts
	fm.SetGlobalFacts(work, "auto_invocation_revisit_506")
	bodyCG := cg.CloneSubcontext()
	bodyCG.CurrentFunc = f
	bodyCG.FM = fm
	ok := VisitFactsBlock(f.Body, &bodyCG, opts)
	if !ok {
		// policy / body visit fail — restore maps; analysis fail is soft (C++ log_analysis_fail
		// returns false without leaving permanent Error for caller generation). Sticky
		// ERROR from incomplete IR during nested visit would poison subsequent soft paths.
		restore()
		fm.SetGlobalFacts(savedGlobal, "auto_invocation_revisit_516")
		// Soft analysis fail must not leave sticky ERROR for later soft paths.
		sessClearError(cgSess(cg))
		return false
	}
	// incomplete body GlobalFacts sticky
	if !FactsComplete(fm.GlobalFacts) {
		restore()
		fm.SetGlobalFacts(savedGlobal, "auto_invocation_revisit_523")
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	work = CloneFactSliceSess(cgSess(cg), fm.GlobalFacts)
	// Restore callee GlobalFacts immediately — do not leave caller lattice installed.
	fm.SetGlobalFacts(savedGlobal, "auto_invocation_revisit_531")
	// body Block::stm_id always live; StmID 0 sticky
	if StmIDUnset(f.Body.StmID) {
		restore()
		sessNoteError(cgSess(cg), ErrGeneric)
		return false
	}
	// Incomplete body map_stm_effect sticky
	bodyEff := fm.GetMapStmEffect(f.Body.StmID)
	if !EffectComplete(bodyEff) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	cg.AddEffect(bodyEff, false)
	// residual ERROR sticky — no invent soft-continue ret-facts past AddEffect residual
	if sessHasError(cgSess(cg)) {
		restore()
		return false
	}
	if !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// FunctionInvocationUser.cpp:336–341 — ret_facts starts empty then
	// add_back_return_facts; merge_facts(inputs, ret_facts). Full FactVec.
	// Soft invent was point-to body-out only (missed return eUnionWrite joins).
	retFacts := []*FactPointTo{}
	retUnions := []*FactUnion{}
	if !AddBackReturnFacts(f.Body, fm, &retFacts, &retUnions) ||
		!FactsComplete(retFacts) || !UnionFactsComplete(retUnions) || !FactsComplete(work) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// FunctionInvocationUser.cpp:358–365 — save full FactVec matching rv (ePointTo + eUnionWrite).
	fi.SaveReturnFactsSess(cgSess(cg), retFacts)
	fi.SaveReturnUnionFactsSess(cgSess(cg), retUnions)
	if sessHasError(cgSess(cg)) {
		restore()
		return false
	}
	_ = MergeFactsSess(cgSess(cg), &work, retFacts)
	if !FactsComplete(work) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// eUnionWrite half of merge_facts(inputs, ret_facts)
	if !UnionFactsComplete(fm.UnionFacts) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	for _, nf := range retUnions {
		if nf == nil {
			restore()
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		fm.UnionFacts = MergeUnionFact(fm.UnionFacts, nf)
		if !UnionFactsComplete(fm.UnionFacts) {
			restore()
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
	// FunctionInvocationUser.cpp:344 — update_facts_for_oos_vars(func->param, inputs)
	// Full FactVec: ePointTo + eUnionWrite.
	UpdateFactsForOOSVarsSess(cgSess(cg), f.Param, &work)
	if !FactsComplete(work) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	UpdateUnionFactsForOOSVarsSess(cgSess(cg), f.Param, &fm.UnionFacts)
	if !UnionFactsComplete(fm.UnionFacts) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	fm.SetupInOutMaps(false)
	// Incomplete external merge sticky
	if !EffectComplete(cg.EffectContext()) || !EffectComplete(f.AccumEffContext) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	f.AccumEffContext = f.AccumEffContext.AddExternalEffect(cg.EffectContext())
	if !EffectComplete(f.AccumEffContext) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	// FunctionInvocationUser.cpp:348–350 — renew_facts(inputs_copy, inputs); inputs = inputs_copy
	// Restore pre-handover caller lattice (incl. frame-local may-null) then apply body deltas.
	// Full FactVec renew: ePointTo + global eUnionWrite (build path
	// function_invocation.go GlobalUnionFactsOnly + RenewUnionFacts).
	_ = RenewFactsSess(cgSess(cg), &inputsCopy, work)
	if !FactsComplete(inputsCopy) {
		restore()
		if !sessHasError(cgSess(cg)) {
			sessNoteError(cgSess(cg), ErrGeneric)
		}
		return false
	}
	*facts = inputsCopy
	// Renew caller's UnionFacts when cg.FM is the caller (VisitFactsInvocation path).
	if cg.FM != nil && cg.FM != fm {
		if !UnionFactsComplete(cg.FM.UnionFacts) {
			restore()
			sessNoteError(cgSess(cg), ErrGeneric)
			return false
		}
		retUF := GlobalUnionFactsOnly(fm.UnionFacts)
		if !UnionFactsComplete(retUF) {
			restore()
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
		_ = RenewUnionFactsSess(cgSess(cg), &cg.FM.UnionFacts, retUF)
		if !UnionFactsComplete(cg.FM.UnionFacts) {
			restore()
			if !sessHasError(cgSess(cg)) {
				sessNoteError(cgSess(cg), ErrGeneric)
			}
			return false
		}
	}
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

// cloneUnionFactMap deep-copies MapUnionFactsIn/Out for revisit restore
// (FactMgr.cpp map_facts_in/out full FactVec backup includes eUnionWrite).
func cloneUnionFactMap(m map[int][]*FactUnion) map[int][]*FactUnion {
	return cloneUnionFactMapSess(nil, m)
}

func cloneUnionFactMapSess(s *Session, m map[int][]*FactUnion) map[int][]*FactUnion {
	if m == nil {
		return make(map[int][]*FactUnion)
	}
	out := make(map[int][]*FactUnion, len(m))
	for k, v := range m {
		out[k] = storeUnionFactMapEntrySess(s, v)
	}
	return out
}

func cloneEffectMap(m map[int]Effect) map[int]Effect {
	return cloneEffectMapSess(nil, m)
}

func cloneEffectMapSess(s *Session, m map[int]Effect) map[int]Effect {
	if m == nil {
		return make(map[int]Effect)
	}
	out := make(map[int]Effect, len(m))
	for k, v := range m {
		cp := v.Clone()
		// residual ERROR sticky — no invent soft-clone map past IncompleteEffect residual
		if sessHasError(s) {
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
	return fi.GetQualifiersSess(nil)
}

// GetQualifiersSess is GetQualifiers with explicit session residual sticky.
func (fi *Invocation) GetQualifiersSess(s *Session) CVQualifiers {
	// FunctionInvocation always live; sticky incomplete no invent default quals
	if fi == nil {
		sessNoteError(s, ErrGeneric)
		return CVQualifiers{}
	}
	// FunctionInvocation.cpp:486–491 — eFuncCall: assert(func); assert(rv)
	if fi.User != nil {
		// assert(rv) path — incomplete RV sticky empty qfer
		// (no invent NewCVQualifiers(false,false) storage-level shell)
		if fi.User.RV == nil {
			sessNoteError(s, ErrGeneric)
			return CVQualifiers{}
		}
		return fi.User.RV.Qfer
	}
	// binary/unary — non-const non-volatile int-like (C++ default for std ops)
	return NewCVQualifiersSess(s, []bool{false}, []bool{false})
}
