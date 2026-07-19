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
func AddReturnFactForInvocation(fi *Invocation, f *FactPointTo) {
	if fi == nil || f == nil {
		return
	}
	// keep parallel slices; desync is broken IR — reset rather than invent
	if len(returnFactInvocations) != len(returnFactPoints) {
		returnFactInvocations = nil
		returnFactPoints = nil
	}
	for i, inv := range returnFactInvocations {
		if inv == fi && returnFactPoints[i] != nil && returnFactPoints[i].Var == f.Var {
			returnFactPoints[i] = f
			return
		}
	}
	returnFactInvocations = append(returnFactInvocations, fi)
	returnFactPoints = append(returnFactPoints, f)
}

// GetReturnFactForInvocation mirrors get_return_fact_for_invocation (point-to).
// FunctionInvocationUser.cpp:76–91 — assert parallel sizes.
func GetReturnFactForInvocation(fi *Invocation, v *Variable) *FactPointTo {
	if fi == nil || v == nil {
		return nil
	}
	if len(returnFactInvocations) != len(returnFactPoints) {
		return nil
	}
	for i, inv := range returnFactInvocations {
		if inv == fi && returnFactPoints[i] != nil && returnFactPoints[i].Var == v {
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
func (fi *Invocation) SaveReturnFacts(facts []*FactPointTo) {
	if fi == nil || fi.User == nil || fi.User.RV == nil {
		return
	}
	for _, f := range facts {
		if f != nil && f.Var != nil && fi.User.RV.Match(f.Var) {
			AddReturnFactForInvocation(fi, f)
		}
	}
}

// RenewFact mirrors renew_fact — replace related or append.
// Fact.cpp:175–191.
// Fact* always live; nil nf or subject-map hole fails closed (false, no invent skip).
func RenewFact(facts *[]*FactPointTo, nf *FactPointTo) bool {
	if facts == nil || nf == nil || nf.Var == nil {
		return false
	}
	if !FactsComplete(*facts) {
		return false
	}
	for i, f := range *facts {
		if f.Var == nf.Var || f.Var.Match(nf.Var) {
			if f.Equal(nf) {
				return false
			}
			(*facts)[i] = nf
			return true
		}
	}
	*facts = append(*facts, nf)
	return true
}

// RenewFacts mirrors renew_facts.
// Fact.cpp:203–210.
// Fact* always live; nil hole in newFacts fails closed (false, no invent partial renew).
func RenewFacts(facts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if facts == nil {
		return false
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		return false
	}
	changed := false
	for _, nf := range newFacts {
		if RenewFact(facts, nf) {
			changed = true
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
	if fi == nil {
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
		if fi.Args[i] != nil && FuncCount(fi.Args[i]) > 0 {
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
func (fi *Invocation) VisitUnorderedParams(facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// FunctionInvocation.cpp:457+ — always live this + facts; no soft invent true
	if fi == nil || facts == nil || cg == nil {
		return false
	}
	inputsCopy := CloneFactSlice(*facts)
	orders := fi.PermuteParamOrders()
	// FunctionInvocation.cpp:462 — assert(orders.size() > 0); no soft invent success
	if len(orders) == 0 {
		return false
	}
	var merged []*FactPointTo
	for i, order := range orders {
		cur := CloneFactSlice(inputsCopy)
		for _, paramID := range order {
			if paramID < 0 || paramID >= len(fi.Args) {
				return false
			}
			arg := fi.Args[paramID]
			// param_value[i] always non-null after ERROR_GUARD
			if arg == nil {
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
				cur = CloneFactSlice(cg.FM.GlobalFacts)
			}
		}
		if i == 0 {
			merged = cur
		} else {
			MergeFacts(&merged, cur)
		}
	}
	*facts = merged
	if cg.FM != nil {
		cg.FM.GlobalFacts = merged
	}
	return true
}

// NeedsRevisit reports whether build_invocation would re-analyze the callee.
// FunctionInvocationUser.cpp:274–276.
func (f *Function) NeedsRevisit() bool {
	if f == nil {
		return false
	}
	return f.FactChanged || f.UnionFieldRead || f.IsPointerReferenced()
}

// IsPointerReferenced mirrors Function::is_pointer_referenced.
// Function.h:110.
func (f *Function) IsPointerReferenced() bool {
	return f != nil && len(f.ReferencedPtrs) > 0
}

// RevisitUserInvocation mirrors FunctionInvocationUser::revisit.
// FunctionInvocationUser.cpp:309–352.
func RevisitUserInvocation(fi *Invocation, facts *[]*FactPointTo, cg *CGContext, opts Options) bool {
	// FunctionInvocationUser.cpp:309+ — get_fact_mgr_for_func + body always live
	// no soft invent true for incomplete caller context
	if fi == nil || fi.User == nil || facts == nil || cg == nil {
		return false
	}
	f := fi.User
	if f.Body == nil {
		return false
	}
	// callee FactMgr — prefer function's FM from caller's package map if same
	fm := cg.FM
	// when revisiting callee, use a dedicated FM on the function if stored
	// light: use caller FM but clear visited for body analysis
	if fm == nil {
		return false
	}
	// backup maps
	inCopy := cloneFactMap(fm.MapFactsIn)
	outCopy := cloneFactMap(fm.MapFactsOut)
	effCopy := cloneEffectMap(fm.MapStmEffect)
	accCopy := cloneEffectMap(fm.MapAccumEffect)
	inputsCopy := CloneFactSlice(*facts)

	fm.ClearMapVisited()
	f.VisitedCnt++
	if f.VisitedCnt == 1 {
		fm.SetupInOutMaps(true)
	}
	// handover params into inputs
	fm.CallerToCalleeHandover(fi.Args, facts)
	// visit body
	savedGlobal := fm.GlobalFacts
	fm.GlobalFacts = *facts
	bodyCG := *cg
	bodyCG.CurrentFunc = f
	bodyCG.FM = fm
	ok := VisitFactsBlock(f.Body, &bodyCG, opts)
	if !ok {
		fm.MapFactsIn = inCopy
		fm.MapFactsOut = outCopy
		fm.MapStmEffect = effCopy
		fm.MapAccumEffect = accCopy
		*facts = inputsCopy
		fm.GlobalFacts = savedGlobal
		return false
	}
	*facts = CloneFactSlice(fm.GlobalFacts)
	// body effect
	if f.Body.StmID > 0 {
		cg.AddEffect(fm.GetMapStmEffect(f.Body.StmID), false)
	}
	var retFacts []*FactPointTo
	AddBackReturnFacts(f.Body, fm, &retFacts)
	fi.SaveReturnFacts(retFacts)
	MergeFacts(facts, retFacts)
	// drop param locals OOS
	UpdateFactsForOOSVars(f.Param, facts)
	fm.SetupInOutMaps(false)
	// accum effect context on function
	f.AccumEffContext = f.AccumEffContext.AddExternalEffect(cg.EffectContext())
	// renew into original inputs_copy
	RenewFacts(&inputsCopy, *facts)
	*facts = inputsCopy
	fm.GlobalFacts = *facts
	return true
}

func cloneFactMap(m map[int][]*FactPointTo) map[int][]*FactPointTo {
	if m == nil {
		return make(map[int][]*FactPointTo)
	}
	out := make(map[int][]*FactPointTo, len(m))
	for k, v := range m {
		out[k] = CloneFactSlice(v)
	}
	return out
}

func cloneEffectMap(m map[int]Effect) map[int]Effect {
	if m == nil {
		return make(map[int]Effect)
	}
	out := make(map[int]Effect, len(m))
	for k, v := range m {
		out[k] = v.Clone()
	}
	return out
}

// GetQualifiers mirrors FunctionInvocation::get_qualifiers.
// FunctionInvocation.cpp:482–498 — user rv qfer; else non-const non-vol.
func (fi *Invocation) GetQualifiers() CVQualifiers {
	// FunctionInvocation.cpp:486–491 — eFuncCall: assert(func); assert(rv)
	if fi != nil && fi.User != nil {
		// fail closed bare qfer when RV missing (no invent arbitrary qualifiers)
		if fi.User.RV == nil {
			return NewCVQualifiers([]bool{false}, []bool{false})
		}
		return fi.User.RV.Qfer
	}
	// binary/unary — non-const non-volatile int-like
	return NewCVQualifiers([]bool{false}, []bool{false})
}
