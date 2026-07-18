// Upstream: FactMgr.h / FactMgr.cpp (per-function DFA facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// FactMgr mirrors FactMgr for a function — global_facts + stm maps.
// GlobalFacts holds FactPointTo; UnionFacts holds FactUnion.
type FactMgr struct {
	// Func is the owning function (FactMgr.cpp constructor).
	Func *Function
	// GlobalFacts mirrors global_facts (FactPointTo subset).
	GlobalFacts []*FactPointTo
	// UnionFacts is FactUnion subset of global_facts.
	UnionFacts []*FactUnion
	// CFGEdges mirrors cfg_edges.
	CFGEdges []*CFGEdge
	// MapStmEffect mirrors map_stm_effect — keyed by Statement::stm_id.
	// FactMgr.h:165.
	MapStmEffect map[int]Effect
	// MapFactsIn / MapFactsOut mirror map_facts_in/out (point-to slices by stm_id).
	// FactMgr.h:161–163.
	MapFactsIn  map[int][]*FactPointTo
	MapFactsOut map[int][]*FactPointTo
}

// NewFactMgr constructs a FactMgr for f (FactMgr::FactMgr(Function*)).
func NewFactMgr(f *Function) *FactMgr {
	return &FactMgr{
		Func:         f,
		MapStmEffect: make(map[int]Effect),
		MapFactsIn:   make(map[int][]*FactPointTo),
		MapFactsOut:  make(map[int][]*FactPointTo),
	}
}

// SetMapFactsIn records pre-statement facts (FactMgr::map_facts_in[s] = facts).
func (fm *FactMgr) SetMapFactsIn(stmID int, facts []*FactPointTo) {
	if fm == nil || stmID <= 0 {
		return
	}
	if fm.MapFactsIn == nil {
		fm.MapFactsIn = make(map[int][]*FactPointTo)
	}
	fm.MapFactsIn[stmID] = CloneFactSlice(facts)
}

// SetMapFactsOut records post-statement facts.
func (fm *FactMgr) SetMapFactsOut(stmID int, facts []*FactPointTo) {
	if fm == nil || stmID <= 0 {
		return
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	fm.MapFactsOut[stmID] = CloneFactSlice(facts)
}

// SetMapFactsOutForStmt mirrors FactMgr::set_fact_out with jump/return filtering.
// FactMgr.cpp:257–274 — drop loop/function locals for break/continue/return.
func (fm *FactMgr) SetMapFactsOutForStmt(st *Stmt, facts []*FactPointTo, blk *Block) {
	if fm == nil || st == nil {
		return
	}
	cp := CloneFactSlice(facts)
	switch st.Kind {
	case StmtContinue, StmtBreak:
		cp = RemoveLoopLocalFacts(cp, blk)
	case StmtReturn:
		cp = RemoveFunctionLocalFacts(cp, fm.Func)
	case StmtGoto:
		// full dest update deferred; drop function-locals as approximation for far jumps
		cp = RemoveFunctionLocalFacts(cp, fm.Func)
	}
	if st.StmID > 0 {
		fm.SetMapFactsOut(st.StmID, cp)
	}
}

// RemoveLoopLocalFacts mirrors FactMgr::remove_loop_local_facts.
// FactMgr.cpp:601–612 — drop facts for locals in loop block chain.
func RemoveLoopLocalFacts(facts []*FactPointTo, blk *Block) []*FactPointTo {
	if blk == nil {
		return facts
	}
	var locals []*Variable
	b := blk
	for b != nil {
		locals = append(locals, b.LocalVars...)
		if b.Looping {
			break
		}
		b = b.Parent
	}
	return filterFactsNotInVars(facts, locals)
}

// RemoveFunctionLocalFacts mirrors FactMgr::remove_function_local_facts subset.
// FactMgr.cpp:179+ — drop l_/p_ and function-local vars.
func RemoveFunctionLocalFacts(facts []*FactPointTo, f *Function) []*FactPointTo {
	var locals []*Variable
	if f != nil {
		locals = append(locals, f.Param...)
		for _, b := range f.Blocks {
			if b != nil {
				locals = append(locals, b.LocalVars...)
			}
		}
	}
	// also drop by name prefix when function unknown
	out := make([]*FactPointTo, 0, len(facts))
	for _, fact := range facts {
		if fact == nil || fact.Var == nil {
			continue
		}
		if fact.Var.IsLocal() || fact.Var.IsArgument() {
			continue
		}
		if IsVariableInSet(locals, fact.Var) {
			continue
		}
		out = append(out, fact)
	}
	return out
}

func filterFactsNotInVars(facts []*FactPointTo, drop []*Variable) []*FactPointTo {
	if len(drop) == 0 {
		return facts
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
		if f == nil || f.Var == nil {
			continue
		}
		if IsVariableInSet(drop, f.Var) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// SetMapStmEffect records effect for a statement (map_stm_effect).
func (fm *FactMgr) SetMapStmEffect(stmID int, eff Effect) {
	if fm == nil || stmID <= 0 {
		return
	}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[stmID] = eff
}

// GetMapStmEffect returns stored effect or empty.
func (fm *FactMgr) GetMapStmEffect(stmID int) Effect {
	if fm == nil || fm.MapStmEffect == nil {
		return EmptyEffect()
	}
	if e, ok := fm.MapStmEffect[stmID]; ok {
		return e
	}
	return EmptyEffect()
}

// FactMgrMap is Function::FMList session map (func → FactMgr).
type FactMgrMap struct {
	byFunc map[*Function]*FactMgr
}

// NewFactMgrMap creates an empty FMList.
func NewFactMgrMap() *FactMgrMap {
	return &FactMgrMap{byFunc: make(map[*Function]*FactMgr)}
}

// ForFunc returns (creating if needed) the FactMgr for f.
// get_fact_mgr_for_func.
func (m *FactMgrMap) ForFunc(f *Function) *FactMgr {
	if m == nil || f == nil {
		return nil
	}
	if m.byFunc == nil {
		m.byFunc = make(map[*Function]*FactMgr)
	}
	if fm, ok := m.byFunc[f]; ok {
		return fm
	}
	fm := NewFactMgr(f)
	m.byFunc[f] = fm
	return fm
}

// AddNewVarFact mirrors FactMgr::add_new_var_fact for point-to init.
// FactMgr.cpp:118–131 + Fact::abstract_fact_for_var_init (pointer init).
func (fm *FactMgr) AddNewVarFact(v *Variable) {
	if fm == nil || v == nil {
		return
	}
	// only pointer vars get FactPointTo in this skeleton
	if !v.IsPointer() {
		// aggregates: add field pointer facts
		for _, f := range v.FieldVars {
			fm.AddNewVarFact(f)
		}
		return
	}
	if FindRelatedPointTo(fm.GlobalFacts, v) != nil {
		return
	}
	// Fact.cpp:85–95 — abstract assign from init when present
	if v.Init != nil {
		rhs := &Expression{Term: TermConstant, Con: v.Init}
		newFacts := AbstractFactForAssign(nil, v, 0, rhs)
		if len(newFacts) > 0 {
			for _, f := range newFacts {
				fm.GlobalFacts = MergeFactInto(fm.GlobalFacts, f)
			}
			return
		}
	}
	fm.GlobalFacts = append(fm.GlobalFacts, NewFactPointTo(v))
}

// AddNewVarFactAndUpdate mirrors add_new_var_fact_and_update_inout_maps
// without stm maps — only global_facts.
// FactMgr.cpp:69–85 subset.
func (fm *FactMgr) AddNewVarFactAndUpdate(blk *Block, v *Variable) {
	_ = blk
	fm.AddNewVarFact(v)
}

// UpdateFactForAssign mirrors FactMgr::update_fact_for_assign(Lhs, Expression, facts).
// FactMgr.cpp:370–395 subset — apply AbstractFactForAssign into GlobalFacts.
func (fm *FactMgr) UpdateFactForAssign(lhs *Variable, lhsIndir int, rhs *Expression) bool {
	if fm == nil || lhs == nil {
		return false
	}
	changed := false
	newFacts := AbstractFactForAssign(fm.GlobalFacts, lhs, lhsIndir, rhs)
	for _, f := range newFacts {
		fm.GlobalFacts = MergeFactInto(fm.GlobalFacts, f)
		changed = true
	}
	// FactUnion: writing a union field records last_written_fid
	if lhsIndir == 0 && lhs.IsInsideUnionField() {
		uf := lhs
		for uf != nil && !uf.IsUnionField() {
			uf = uf.FieldVarOf
		}
		if uf != nil && uf.FieldVarOf != nil {
			parent := uf.FieldVarOf
			fid := uf.GetFieldID()
			fm.UnionFacts = MergeUnionFact(fm.UnionFacts, MakeFactUnion(parent, fid))
			changed = true
		}
	}
	return changed
}

// MergeUnionFact replaces or appends FactUnion for the same union var.
func MergeUnionFact(facts []*FactUnion, f *FactUnion) []*FactUnion {
	if f == nil {
		return facts
	}
	for i, old := range facts {
		if old != nil && old.Var == f.Var {
			facts[i] = f
			return facts
		}
	}
	return append(facts, f)
}

// CreateCFGEdge mirrors FactMgr::create_cfg_edge.
// FactMgr.cpp:597–598.
func (fm *FactMgr) CreateCFGEdge(srcID int, dest *Block, postDest, backLink bool) {
	fm.CreateCFGEdgeTo(srcID, dest, 0, postDest, backLink)
}

// CreateCFGEdgeTo is create_cfg_edge with optional dest statement id (goto).
func (fm *FactMgr) CreateCFGEdgeTo(srcID int, dest *Block, destStmID int, postDest, backLink bool) {
	if fm == nil || dest == nil || srcID == 0 {
		return
	}
	fm.CFGEdges = append(fm.CFGEdges, &CFGEdge{
		SrcID:     srcID,
		DestBlock: dest,
		DestStmID: destStmID,
		PostDest:  postDest,
		BackLink:  backLink,
	})
}

// MakeupNewVarFacts mirrors FactMgr::makeup_new_var_facts.
// FactMgr.cpp:494–507 — add facts for globals/locals created after old_facts snapshot.
func MakeupNewVarFacts(oldFacts *[]*FactPointTo, newFacts []*FactPointTo) {
	if oldFacts == nil {
		return
	}
	for _, f := range newFacts {
		if f == nil || f.Var == nil {
			continue
		}
		v := f.Var
		if !v.IsGlobal() && !v.IsLocal() {
			continue
		}
		if FindRelatedPointTo(*oldFacts, v) == nil {
			// add_new_var_fact into old set
			if v.IsPointer() {
				*oldFacts = append(*oldFacts, NewFactPointTo(v))
			}
		}
	}
}

// FindDanglingGlobalPtrs mirrors FactMgr::find_dangling_global_ptrs.
// FactMgr.cpp:688–700 — non-const global pointers that are dead at function exit.
func (fm *FactMgr) FindDanglingGlobalPtrs(f *Function) {
	if fm == nil || f == nil {
		return
	}
	f.DeadGlobals = f.DeadGlobals[:0]
	for _, fact := range fm.GlobalFacts {
		if fact == nil || fact.Var == nil {
			continue
		}
		v := fact.Var
		// const pointers should never be dangling; only globals
		if v.IsConst() || !v.IsGlobal() {
			continue
		}
		if fact.IsDead() {
			f.DeadGlobals = append(f.DeadGlobals, v)
		}
	}
}

// UpdateFactForReturn mirrors FactMgr::update_fact_for_return.
// FactMgr.cpp:406–418 + Fact::abstract_fact_for_return — assign expr into func.rv.
func (fm *FactMgr) UpdateFactForReturn(rv *Variable, expr *Expression) bool {
	if fm == nil || rv == nil {
		return false
	}
	// abstract_fact_for_return = abstract_fact_for_assign(facts, Lhs(rv), expr)
	return fm.UpdateFactForAssign(rv, 0, expr)
}

// UpdateFactsForOOSVars mirrors FactMgr::update_facts_for_oos_vars.
// FactMgr.cpp:141–172 — drop facts for oos vars; mark pointees garbage.
func (fm *FactMgr) UpdateFactsForOOSVars(vars []*Variable) {
	if fm == nil || len(vars) == 0 {
		return
	}
	// remove facts whose subject matches an oos var
	out := fm.GlobalFacts[:0]
	for _, f := range fm.GlobalFacts {
		if f == nil || f.Var == nil {
			continue
		}
		drop := false
		for _, v := range vars {
			if v != nil && v.Match(f.Var) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, f)
		}
	}
	fm.GlobalFacts = out
	// mark remaining facts that point into oos vars as dead
	for i, f := range fm.GlobalFacts {
		if f == nil {
			continue
		}
		cur := f
		for _, v := range vars {
			if nf := cur.MarkDeadVar(v); nf != nil {
				cur = nf
			}
		}
		fm.GlobalFacts[i] = cur
	}
}
