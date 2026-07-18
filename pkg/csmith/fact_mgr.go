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
	// MapFactsInFinal / MapFactsOutFinal mirror map_facts_in/out_final.
	// FactMgr.h — combined across revisits via setup_in_out_maps.
	MapFactsInFinal  map[int][]*FactPointTo
	MapFactsOutFinal map[int][]*FactPointTo
	// MapAccumEffect mirrors map_accum_effect — accum after each statement.
	MapAccumEffect map[int]Effect
	// MapVisited mirrors map_visited — statement analyzed this pass.
	MapVisited map[int]bool
}

// NewFactMgr constructs a FactMgr for f (FactMgr::FactMgr(Function*)).
func NewFactMgr(f *Function) *FactMgr {
	return &FactMgr{
		Func:             f,
		MapStmEffect:     make(map[int]Effect),
		MapFactsIn:       make(map[int][]*FactPointTo),
		MapFactsOut:      make(map[int][]*FactPointTo),
		MapFactsInFinal:  make(map[int][]*FactPointTo),
		MapFactsOutFinal: make(map[int][]*FactPointTo),
		MapAccumEffect:   make(map[int]Effect),
		MapVisited:       make(map[int]bool),
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
// FactMgr.cpp:257–274 — drop loop/function locals for break/continue/return/goto.
// destParent is the goto destination's parent block when Kind==StmtGoto (optional).
func (fm *FactMgr) SetMapFactsOutForStmt(st *Stmt, facts []*FactPointTo, blk *Block) {
	fm.SetMapFactsOutForStmtDest(st, facts, blk, nil)
}

// SetMapFactsOutForStmtDest is set_fact_out with optional goto dest parent.
func (fm *FactMgr) SetMapFactsOutForStmtDest(st *Stmt, facts []*FactPointTo, blk, destParent *Block) {
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
		if destParent != nil && fm.Func != nil {
			out := []*FactPointTo{}
			UpdateFactsForDest(cp, &out, fm.Func, destParent)
			cp = out
		} else {
			// approximation when dest unknown: drop function-locals
			cp = RemoveFunctionLocalFacts(cp, fm.Func)
		}
	}
	if st.StmID > 0 {
		fm.SetMapFactsOut(st.StmID, cp)
	}
}

// UpdateFactsForDest mirrors FactMgr::update_facts_for_dest.
// FactMgr.cpp:424–456 — merge facts; OOS locals at dest become garbage/dropped.
func UpdateFactsForDest(factsIn []*FactPointTo, factsOut *[]*FactPointTo, f *Function, destParent *Block) {
	if factsOut == nil {
		return
	}
	var oosVars []*Variable
	seen := map[*Variable]bool{}
	addOOS := func(v *Variable) {
		if v == nil || seen[v] || IsSpecialPtr(v) {
			return
		}
		seen[v] = true
		oosVars = append(oosVars, v)
	}
	for _, fact := range factsIn {
		if fact == nil || fact.Var == nil {
			continue
		}
		// skip return variables
		if isReturnVar(fact.Var) {
			continue
		}
		if f != nil && f.IsVarOOS(fact.Var, destParent) {
			addOOS(fact.Var)
		}
		for _, p := range fact.PointTo {
			if p != nil && !IsSpecialPtr(p) && f != nil && f.IsVarOOS(p, destParent) {
				addOOS(p)
			}
		}
		*factsOut = MergeFactInto(*factsOut, fact)
	}
	UpdateFactsForOOSVars(oosVars, factsOut)
}

// ClearMapVisited mirrors FactMgr::clear_map_visited.
// FactMgr.cpp:510–514 — set all visited flags false (keep keys).
func (fm *FactMgr) ClearMapVisited() {
	if fm == nil || fm.MapVisited == nil {
		return
	}
	for k := range fm.MapVisited {
		fm.MapVisited[k] = false
	}
}

// RestoreFacts mirrors FactMgr::restore_facts.
// FactMgr.cpp:489–492 — makeup new vars into old, then replace global_facts.
func (fm *FactMgr) RestoreFacts(oldFacts []*FactPointTo) {
	if fm == nil {
		return
	}
	cp := CloneFactSlice(oldFacts)
	MakeupNewVarFacts(&cp, fm.GlobalFacts)
	fm.GlobalFacts = cp
}

// SetupInOutMaps mirrors FactMgr::setup_in_out_maps.
// FactMgr.cpp:208–246 — first_time clones into final; else combine.
func (fm *FactMgr) SetupInOutMaps(firstTime bool) {
	if fm == nil {
		return
	}
	if fm.MapFactsInFinal == nil {
		fm.MapFactsInFinal = make(map[int][]*FactPointTo)
	}
	if fm.MapFactsOutFinal == nil {
		fm.MapFactsOutFinal = make(map[int][]*FactPointTo)
	}
	if firstTime {
		for id, facts := range fm.MapFactsIn {
			fm.MapFactsInFinal[id] = CloneFactSlice(facts)
		}
		for id, facts := range fm.MapFactsOut {
			fm.MapFactsOutFinal[id] = CloneFactSlice(facts)
		}
		return
	}
	// combine current maps into final
	for id, facts2 := range fm.MapFactsIn {
		facts1 := fm.MapFactsInFinal[id]
		MergeFacts(&facts1, facts2)
		fm.MapFactsInFinal[id] = facts1
	}
	for id, facts2 := range fm.MapFactsOut {
		facts1 := fm.MapFactsOutFinal[id]
		MergeFacts(&facts1, facts2)
		fm.MapFactsOutFinal[id] = facts1
	}
}

// BackupStmFactMaps mirrors FactMgr::backup_stm_fact_maps for a statement tree.
// FactMgr.cpp:516–531 — copy in/out maps for stm and nested blocks.
func (fm *FactMgr) BackupStmFactMaps(st *Stmt, factsIn, factsOut map[int][]*FactPointTo) {
	if fm == nil || st == nil {
		return
	}
	if factsIn == nil || factsOut == nil {
		return
	}
	if st.Then != nil {
		fm.backupBlockFactMaps(st.Then, factsIn, factsOut)
	}
	if st.Else != nil {
		fm.backupBlockFactMaps(st.Else, factsIn, factsOut)
	}
	if st.StmID > 0 {
		if in, ok := fm.MapFactsIn[st.StmID]; ok {
			factsIn[st.StmID] = CloneFactSlice(in)
		}
		if out, ok := fm.MapFactsOut[st.StmID]; ok {
			factsOut[st.StmID] = CloneFactSlice(out)
		}
	}
}

func (fm *FactMgr) backupBlockFactMaps(b *Block, factsIn, factsOut map[int][]*FactPointTo) {
	if b == nil {
		return
	}
	if b.StmID > 0 {
		if in, ok := fm.MapFactsIn[b.StmID]; ok {
			factsIn[b.StmID] = CloneFactSlice(in)
		}
		if out, ok := fm.MapFactsOut[b.StmID]; ok {
			factsOut[b.StmID] = CloneFactSlice(out)
		}
	}
	for i := range b.Stmts {
		fm.BackupStmFactMaps(&b.Stmts[i], factsIn, factsOut)
	}
}

// RestoreStmFactMaps mirrors FactMgr::restore_stm_fact_maps.
// FactMgr.cpp:533–548.
func (fm *FactMgr) RestoreStmFactMaps(st *Stmt, factsIn, factsOut map[int][]*FactPointTo) {
	if fm == nil || st == nil {
		return
	}
	if st.Then != nil {
		fm.restoreBlockFactMaps(st.Then, factsIn, factsOut)
	}
	if st.Else != nil {
		fm.restoreBlockFactMaps(st.Else, factsIn, factsOut)
	}
	if st.StmID > 0 {
		if in, ok := factsIn[st.StmID]; ok {
			fm.MapFactsIn[st.StmID] = CloneFactSlice(in)
		} else {
			delete(fm.MapFactsIn, st.StmID)
		}
		if out, ok := factsOut[st.StmID]; ok {
			fm.MapFactsOut[st.StmID] = CloneFactSlice(out)
		} else {
			delete(fm.MapFactsOut, st.StmID)
		}
	}
}

func (fm *FactMgr) restoreBlockFactMaps(b *Block, factsIn, factsOut map[int][]*FactPointTo) {
	if b == nil {
		return
	}
	if b.StmID > 0 {
		if in, ok := factsIn[b.StmID]; ok {
			fm.MapFactsIn[b.StmID] = CloneFactSlice(in)
		} else {
			delete(fm.MapFactsIn, b.StmID)
		}
		if out, ok := factsOut[b.StmID]; ok {
			fm.MapFactsOut[b.StmID] = CloneFactSlice(out)
		} else {
			delete(fm.MapFactsOut, b.StmID)
		}
	}
	for i := range b.Stmts {
		fm.RestoreStmFactMaps(&b.Stmts[i], factsIn, factsOut)
	}
}

// FindUpdatedFacts mirrors FactMgr::find_updated_facts.
// FactMgr.cpp:652–665 — facts_out that differ from related facts_in.
func (fm *FactMgr) FindUpdatedFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.MapFactsIn[stmID]
	out := fm.MapFactsOut[stmID]
	var updated []*FactPointTo
	for _, f := range out {
		if f == nil {
			continue
		}
		prev := FindRelatedPointTo(in, f.Var)
		if prev == nil || !f.Equal(prev) {
			updated = append(updated, f)
		}
	}
	return updated
}

// FindUpdatedFinalFacts mirrors FactMgr::find_updated_final_facts.
// FactMgr.cpp:667–686 — final maps; always include rv facts.
func (fm *FactMgr) FindUpdatedFinalFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.MapFactsInFinal[stmID]
	out := fm.MapFactsOutFinal[stmID]
	var updated []*FactPointTo
	for _, f := range out {
		if f == nil || f.Var == nil {
			continue
		}
		if fm.Func != nil && fm.Func.RV != nil && fm.Func.RV.Match(f.Var) {
			updated = append(updated, f)
			continue
		}
		prev := FindRelatedPointTo(in, f.Var)
		if prev == nil || !f.Equal(prev) {
			updated = append(updated, f)
		}
	}
	return updated
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
	// FactMgr.cpp:400 — assign that changes facts marks function fact_changed
	if changed && fm.Func != nil {
		fm.Func.FactChanged = true
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
	// FactMgr.cpp:415 — fact_changed when return updates facts
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

// AddParamFacts mirrors FactMgr::add_param_facts.
// FactMgr.cpp:108–116 — assign each param from arg expression into facts.
func (fm *FactMgr) AddParamFacts(args []*Expression, facts *[]*FactPointTo) {
	if fm == nil || fm.Func == nil || facts == nil {
		return
	}
	for i, p := range fm.Func.Param {
		if p == nil || !p.IsPointer() {
			continue
		}
		var arg *Expression
		if i < len(args) {
			arg = args[i]
		}
		// update_fact_for_assign(param, arg)
		if arg != nil {
			fm.UpdateFactForAssignInto(p, 0, arg, facts)
		} else {
			*facts = MergeFactInto(*facts, NewFactPointTo(p))
		}
	}
}

// UpdateFactForAssignInto is UpdateFactForAssign writing into a fact slice.
func (fm *FactMgr) UpdateFactForAssignInto(lhs *Variable, lhsIndir int, rhs *Expression, facts *[]*FactPointTo) bool {
	if facts == nil || lhs == nil {
		return false
	}
	changed := false
	newFacts := AbstractFactForAssign(*facts, lhs, lhsIndir, rhs)
	for _, f := range newFacts {
		*facts = MergeFactInto(*facts, f)
		changed = true
	}
	return changed
}

// PointsTo reports whether this fact's set contains v.
func (f *FactPointTo) PointsTo(v *Variable) bool {
	if f == nil || v == nil {
		return false
	}
	return IsVariableInSet(f.PointTo, v)
}

// CallerToCalleeHandover mirrors FactMgr::caller_to_callee_handover.
// FactMgr.cpp:312–353 — param facts; keep globals/params and transitively pointed stack vars.
func (fm *FactMgr) CallerToCalleeHandover(args []*Expression, inputs *[]*FactPointTo) {
	if fm == nil || inputs == nil {
		return
	}
	fm.AddParamFacts(args, inputs)
	// partition: keep globals and params
	var keep, rest []*FactPointTo
	for _, f := range *inputs {
		if f == nil || f.Var == nil {
			continue
		}
		v := f.Var
		if v.IsGlobal() || IsVariableInSet(fm.Func.Param, v) {
			keep = append(keep, f)
		} else {
			rest = append(rest, f)
		}
	}
	// transitively keep facts for variables pointed to by kept pointer facts
	for {
		cnt := len(keep)
		for i := 0; i < len(rest); i++ {
			rf := rest[i]
			if rf == nil || rf.Var == nil {
				continue
			}
			for _, kf := range keep {
				if kf != nil && kf.PointsTo(rf.Var) {
					keep = append(keep, rf)
					rest = append(rest[:i], rest[i+1:]...)
					i--
					break
				}
			}
		}
		if len(keep) == cnt {
			break
		}
	}
	*inputs = keep
}

// RemoveRVFacts mirrors FactMgr::remove_rv_facts.
// FactMgr.cpp:358–368 — drop other functions' return variables.
func (fm *FactMgr) RemoveRVFacts(facts *[]*FactPointTo) {
	if fm == nil || facts == nil {
		return
	}
	out := make([]*FactPointTo, 0, len(*facts))
	for _, f := range *facts {
		if f == nil || f.Var == nil {
			continue
		}
		if f.Var.IsRV() {
			// keep only this function's RV
			if fm.Func != nil && fm.Func.RV != nil && fm.Func.RV.Match(f.Var) {
				out = append(out, f)
			}
			continue
		}
		out = append(out, f)
	}
	*facts = out
}

// OutputTab mirrors output_tab — indent spaces (4 per level).
// util.cpp / OutputMgr — TAB is typically 4 spaces.
func OutputTab(indent int) string {
	if indent <= 0 {
		return ""
	}
	s := ""
	for i := 0; i < indent; i++ {
		s += "    "
	}
	return s
}
