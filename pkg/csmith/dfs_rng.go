// Upstream: DFSRndNumGenerator.h / DFSRndNumGenerator.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "fmt"

// dfsSearchState mirrors DFSRndNumGenerator::SearchState.
// DFSRndNumGenerator.cpp:46–116.
type dfsSearchState struct {
	init  bool
	value int
	bound int
	index int
}

// dfsEngine mirrors DFSRndNumGenerator search fields (not on DefaultRng).
// DFSRndNumGenerator.cpp:120–124 ctor state.
type dfsEngine struct {
	decisionDepth    int
	currentPos       int
	allDone          bool
	seq              *LinearSequence
	useDebugSequence bool
	states           []dfsSearchState
	maxDepth         int
}

// processDFSImpl mirrors DFSRndNumGenerator::impl_ singleton.
var processDFSImpl *Rng

// NewDFSRng mirrors DFSRndNumGenerator ctor + make_rndnum_generator (with seed for Abs LCG).
// AbsRndNumGenerator.cpp:70 seedrand then DFSRndNumGenerator::make_rndnum_generator.
// opts.MaxExhaustiveDepth sizes the SearchState vector (CGOptions::max_exhaustive_depth).
// Incomplete maxDepth<=0 sticky nil (no invent empty engine that always EXCEED).
func NewDFSRng(seed uint64, opts Options) *Rng {
	maxDepth := opts.MaxExhaustiveDepth
	// C++ init_states(max) with max<=0 yields empty states_; every choice EXCEED.
	// Library fail-closed: require positive depth for a usable DFS engine.
	if maxDepth <= 0 {
		SetError(ErrGeneric)
		return nil
	}
	r := NewRng(seed)
	r.kind = RngKindDFS
	eng := &dfsEngine{
		decisionDepth: -1,
		currentPos:    -1,
		allDone:       false,
		seq:           MakeSequence(),
		maxDepth:      maxDepth,
		states:        make([]dfsSearchState, maxDepth),
	}
	for i := range eng.states {
		eng.states[i].index = i
	}
	// DFSRndNumGenerator.cpp:147–155 — optional dfs_debug_sequence
	if dbg := opts.DFSDebugSequence; dbg != "" {
		nums, ok := ParseSequenceLine(dbg, CurrentSepChar())
		if !ok {
			// C++ assert("dfs debugging sequence error!"); sticky fail closed
			SetError(ErrGeneric)
			return nil
		}
		eng.initializeSequence(nums)
		eng.useDebugSequence = true
	}
	r.dfs = eng
	return r
}

// makeDFSRndNumGeneratorOpts mirrors DFSRndNumGenerator::make_rndnum_generator singleton.
// DFSRndNumGenerator.cpp:137–158.
// opts must be supplied by caller (avoids ProcessOptions RLock under processOptsMu).
func makeDFSRndNumGeneratorOpts(seed uint64, opts Options) *Rng {
	if processDFSImpl != nil {
		return processDFSImpl
	}
	r := NewDFSRng(seed, opts)
	if r == nil {
		return nil
	}
	processDFSImpl = r
	return r
}

// clearDFSImpl drops the process DFS singleton (RandomNumber::doFinalization path).
func clearDFSImpl() {
	if processDFSImpl != nil {
		// DFSRndNumGenerator dtor → SequenceFactory::destroy_sequences
		DestroySequences()
		processDFSImpl = nil
	}
}

func (e *dfsEngine) initializeSequence(v []int) {
	// DFSRndNumGenerator.cpp:161–165
	if e == nil || e.seq == nil {
		SetError(ErrGeneric)
		return
	}
	for i, n := range v {
		e.seq.AddNumber(n, 0, i)
	}
}

// EagerBacktracking mirrors DFSRndNumGenerator::eager_backtracking.
// DFSRndNumGenerator.cpp:181–206 — true means do eager backtracking (sets BACKTRACKING_ERROR).
func (r *Rng) EagerBacktracking(depthNeeded int) bool {
	if r == nil || r.kind != RngKindDFS || r.dfs == nil {
		SetError(ErrGeneric)
		return false
	}
	e := r.dfs
	if e.currentPos <= 0 {
		return false
	}
	maxDepth := e.maxDepth
	remain := maxDepth - e.currentPos
	if remain >= depthNeeded {
		return false
	}
	if e.currentPos > e.decisionDepth {
		SetError(ErrBacktracking)
		return true
	}
	e.decisionDepth = e.currentPos
	for i := e.currentPos + 1; i < maxDepth; i++ {
		e.states[i].init = false
	}
	SetError(ErrBacktracking)
	return true
}

// DFSGetDecisionDepth mirrors get_decision_depth.
func (r *Rng) DFSGetDecisionDepth() int {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return -1
	}
	return r.dfs.decisionDepth
}

// DFSGetCurrentPos mirrors get_current_pos.
func (r *Rng) DFSGetCurrentPos() int {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return -1
	}
	return r.dfs.currentPos
}

// DFSSetCurrentPos mirrors set_current_pos.
func (r *Rng) DFSSetCurrentPos(pos int) {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return
	}
	r.dfs.currentPos = pos
}

// DFSGetAllDone mirrors get_all_done.
func (r *Rng) DFSGetAllDone() bool {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return false
	}
	return r.dfs.allDone
}

// DFSResetState mirrors reset_state.
// DFSRndNumGenerator.cpp:381–385.
func (r *Rng) DFSResetState() {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return
	}
	e := r.dfs
	e.currentPos = -1
	r.traceString = ""
	if e.seq != nil {
		e.seq.Clear()
	}
}

// filterInvalidNums mirrors filter_invalid_nums.
// DFSRndNumGenerator.cpp:230–235.
func filterInvalidNums(invalid []int, v int) bool {
	for _, x := range invalid {
		if x == v {
			return true
		}
	}
	return false
}

// dfsRevisitNode mirrors revisit_node.
// DFSRndNumGenerator.cpp:208–227.
func (e *dfsEngine) revisitNode(state *dfsSearchState, localPos, bound int, f Filter) int {
	rv := state.value
	if f != nil {
		// C++ asserts rv < bound; sticky fail closed
		if rv >= bound {
			SetError(ErrGeneric)
			return -1
		}
		_ = f.Filter(uint32(rv))
		if HasError() {
			return -1
		}
		if e.currentPos >= e.maxDepth {
			SetError(ErrGeneric)
			return -1
		}
	}
	if e.seq == nil {
		SetError(ErrGeneric)
		return -1
	}
	e.seq.AddNumber(rv, bound, localPos)
	return rv
}

// dfsRandomChoice mirrors random_choice.
// DFSRndNumGenerator.cpp:238–348.
// Returns choice in [0,bound) or -1 on backtrack/error (error sticky).
func (r *Rng) dfsRandomChoice(bound int, f Filter, invalid []int) int {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return -1
	}
	e := r.dfs
	err := GetError()
	if err == ErrBacktracking {
		return -1
	}
	if err != ErrSuccess {
		// C++ assert request in error state — sticky keep existing code
		return -1
	}
	if bound <= 0 {
		// undefined domain sticky (no invent fixed 0 choice)
		SetError(ErrGeneric)
		return -1
	}

	e.currentPos++
	if e.useDebugSequence {
		if e.seq == nil {
			SetError(ErrGeneric)
			return -1
		}
		rv := e.seq.GetNumberByPos(e.currentPos)
		if HasError() {
			return -1
		}
		if f != nil {
			_ = f.Filter(uint32(rv))
		}
		if e.currentPos >= e.seq.SequenceLength()-1 {
			e.allDone = true
		}
		return rv
	}

	localPos := e.currentPos
	if e.currentPos >= e.maxDepth || e.decisionDepth >= e.maxDepth {
		SetError(ErrExceedMaxDepth)
		return -1
	}
	// states always sized to maxDepth at construction
	if e.currentPos < 0 || e.currentPos >= len(e.states) {
		SetError(ErrGeneric)
		return -1
	}
	state := &e.states[e.currentPos]
	state.bound = bound

	// Revisit a node
	if e.currentPos < e.decisionDepth && state.init {
		return e.revisitNode(state, localPos, bound, f)
	}

	if state.init {
		v := state.value
		localDecision := e.decisionDepth
		for {
			v++
			state.value = v
			e.currentPos = localPos
			e.decisionDepth = localDecision
			if HasError() {
				return -1
			}
			if !(v < bound && ((f != nil && f.Filter(uint32(v))) || filterInvalidNums(invalid, v))) {
				break
			}
		}
		state.value = v
		if state.value >= bound {
			// backtracking
			e.currentPos = localPos
			for i := e.currentPos; i < e.maxDepth; i++ {
				e.states[i].init = false
			}
			e.decisionDepth--
			if e.decisionDepth < 0 {
				e.allDone = true
			}
			SetError(ErrBacktracking)
			return -1
		}
		if HasError() {
			return -1
		}
		rv := state.value
		if e.seq == nil {
			SetError(ErrGeneric)
			return -1
		}
		e.seq.AddNumber(rv, bound, localPos)
		return rv
	}

	// First time to visit this node
	v := 0
	e.decisionDepth++
	state.init = true
	state.value = v
	state.bound = bound

	for v < bound && ((f != nil && f.Filter(uint32(v))) || filterInvalidNums(invalid, v)) {
		for i := e.decisionDepth; i < e.maxDepth; i++ {
			e.states[i].value = 0
		}
		if HasError() {
			return -1
		}
		e.decisionDepth = e.currentPos
		e.currentPos = localPos
		v++
	}
	e.decisionDepth = e.currentPos

	if v >= bound {
		e.currentPos = localPos
		for i := e.currentPos; i < e.maxDepth; i++ {
			e.states[i].init = false
		}
		e.decisionDepth--
		if e.decisionDepth < 0 {
			e.allDone = true
		}
		SetError(ErrBacktracking)
		return -1
	}

	state.value = v
	if HasError() {
		return -1
	}
	if e.seq == nil {
		SetError(ErrGeneric)
		return -1
	}
	e.seq.AddNumber(v, bound, localPos)
	return v
}

// DFSLogDepth mirrors log_depth.
// DFSRndNumGenerator.cpp:351–365.
func (r *Rng) DFSLogDepth(d int, where, log string) {
	if r == nil || r.dfs == nil {
		SetError(ErrGeneric)
		return
	}
	e := r.dfs
	var prefix string
	if log != "" {
		prefix = "[" + log + "]"
	}
	if where != "" {
		r.traceString += fmt.Sprintf("%s%d(%s, pos = %d, current_decision_depth=%d)->",
			prefix, d, where, e.currentPos, e.decisionDepth)
	} else {
		r.traceString += fmt.Sprintf("%s%d(..., pos = %d, current_decision_depth=%d)->",
			prefix, d, e.currentPos, e.decisionDepth)
	}
}

// GetPrefixedNameDFS mirrors DFSRndNumGenerator::get_prefixed_name.
// DFSRndNumGenerator.cpp:397–403 — "p_" + sequence + sep + name.
func (r *Rng) GetPrefixedNameDFS(name string) string {
	if r == nil || r.dfs == nil || r.dfs.seq == nil {
		SetError(ErrGeneric)
		return name
	}
	seq := r.dfs.seq.GetSequence()
	if HasError() {
		// empty sequence sticky — no invent "p__name"
		return name
	}
	return "p_" + seq + string(r.dfs.seq.SepChar()) + name
}

// dfsRndUpto mirrors DFSRndNumGenerator::rnd_upto.
// DFSRndNumGenerator.cpp:411–415.
func (r *Rng) dfsRndUpto(n uint32, f Filter) uint32 {
	x := r.dfsRandomChoice(int(n), f, nil)
	// C++ casts -1 → unsigned max; callers ERROR_GUARD first
	if x < 0 {
		return uint32(int32(x))
	}
	return uint32(x)
}

// dfsRndFlipcoin mirrors DFSRndNumGenerator::rnd_flipcoin.
// DFSRndNumGenerator.cpp:417–431 — p==100 forces 1; p==0 forces 0 via invalid list.
func (r *Rng) dfsRndFlipcoin(p uint32, f Filter) bool {
	var invalid []int
	var y int
	switch {
	case p == 100:
		invalid = []int{0}
		y = r.dfsRandomChoice(2, f, invalid)
	case p == 0:
		invalid = []int{1}
		y = r.dfsRandomChoice(2, f, invalid)
	default:
		y = r.dfsRandomChoice(2, f, nil)
	}
	// C++ return y as bool: -1 → true; 0 → false; 1 → true
	return y != 0
}

// Format helper for tests — sequence string via engine.
func (r *Rng) dfsSequenceString() string {
	if r == nil || r.dfs == nil || r.dfs.seq == nil {
		return ""
	}
	return r.dfs.seq.GetSequence()
}

