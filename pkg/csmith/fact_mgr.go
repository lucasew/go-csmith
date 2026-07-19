// Upstream: FactMgr.h / FactMgr.cpp (per-function DFA facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// eFactCategory (Fact.h) — bit flags for FactMgr::meta_facts registration.
const (
	// FactCategoryPointTo is ePointTo.
	FactCategoryPointTo = 1
	// FactCategoryUnionWrite is eUnionWrite.
	FactCategoryUnionWrite = 2
	// DefaultInterestedFacts matches CGOptions default (point-to | union-write).
	DefaultInterestedFacts = FactCategoryPointTo | FactCategoryUnionWrite
)

// meta_facts enable flags (FactMgr.cpp:67 meta_facts vector).
// When false, corresponding abstract_fact paths are skipped.
var (
	metaFactPointToEnabled = true
	metaFactUnionEnabled   = true
)

// AddInterestedFacts mirrors FactMgr::add_interested_facts.
// FactMgr.cpp:475–486 — register meta fact kinds for DFA.
func AddInterestedFacts(interests int) {
	metaFactPointToEnabled = interests&FactCategoryPointTo != 0
	metaFactUnionEnabled = interests&FactCategoryUnionWrite != 0
}

// MetaFactPointToEnabled reports whether point-to analysis is active.
func MetaFactPointToEnabled() bool { return metaFactPointToEnabled }

// MetaFactUnionEnabled reports whether union-write analysis is active.
func MetaFactUnionEnabled() bool { return metaFactUnionEnabled }

// ClearMetaFacts restores default interested facts (both on).
// Called from DoFinalization between generations.
func ClearMetaFacts() {
	metaFactPointToEnabled = true
	metaFactUnionEnabled = true
}

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
	// incomplete facts store nil (no invent cleaned partial map entry)
	if !FactsComplete(facts) {
		fm.MapFactsIn[stmID] = nil
		return
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
	// incomplete facts store nil (no invent cleaned partial map entry)
	if !FactsComplete(facts) {
		fm.MapFactsOut[stmID] = nil
		return
	}
	fm.MapFactsOut[stmID] = CloneFactSlice(facts)
}

// SetMapFactsOutForStmt mirrors FactMgr::set_fact_out with jump/return filtering.
// FactMgr.cpp:257–274 — drop loop/function locals for break/continue/return/goto.
// blk is s->parent (statement parent block).
func (fm *FactMgr) SetMapFactsOutForStmt(st *Stmt, facts []*FactPointTo, blk *Block) {
	// Goto dest from StatementGoto::dest (no soft invent always-nil destParent).
	var destParent *Block
	if st != nil && st.Kind == StmtGoto {
		destParent = st.GotoDestParent
		if destParent == nil && st.GotoDestStmID > 0 && fm != nil && fm.Func != nil {
			destParent = FindParentBlockOfStmID(fm.Func, st.GotoDestStmID)
		}
	}
	fm.SetMapFactsOutForStmtDest(st, facts, blk, destParent)
}

// SetMapFactsOutForStmtDest is set_fact_out with optional goto dest parent override.
func (fm *FactMgr) SetMapFactsOutForStmtDest(st *Stmt, facts []*FactPointTo, blk, destParent *Block) {
	if fm == nil || st == nil {
		return
	}
	// incomplete source facts fail closed (nil out — no invent cleaned set_fact_out)
	if !FactsComplete(facts) {
		if st.StmID > 0 {
			fm.SetMapFactsOut(st.StmID, nil)
		}
		return
	}
	cp := CloneFactSlice(facts)
	switch st.Kind {
	case StmtContinue, StmtBreak:
		// FactMgr.cpp:257–262 — remove_loop_local_facts(s, facts_copy)
		cp = RemoveLoopLocalFactsForStmt(cp, st, blk)
	case StmtReturn:
		// FactMgr.cpp:268–270 — remove_function_local_facts(facts_copy, s)
		// stack check uses s->parent (blk); no invent f.Body-only walk
		cp = RemoveFunctionLocalFactsAt(cp, fm.Func, blk)
	case StmtGoto:
		// FactMgr.cpp:263–266 — update_facts_for_dest(facts, facts_copy, sg->dest)
		dp := destParent
		if dp == nil {
			dp = st.GotoDestParent
		}
		if dp == nil && st.GotoDestStmID > 0 && fm.Func != nil {
			dp = FindParentBlockOfStmID(fm.Func, st.GotoDestStmID)
		}
		// FactMgr.cpp:427–428 assert(func); no soft invent RemoveFunctionLocalFacts
		// when dest unknown (wrong filter vs update_facts_for_dest)
		if fm.Func == nil {
			cp = nil
		} else {
			out := []*FactPointTo{}
			UpdateFactsForDest(cp, &out, fm.Func, dp)
			cp = out
		}
	default:
		// FactMgr.cpp:268 — eReturn || s->parent == nullptr → remove function locals
		if blk == nil {
			cp = RemoveFunctionLocalFactsAt(cp, fm.Func, nil)
		}
	}
	if st.StmID > 0 {
		fm.SetMapFactsOut(st.StmID, cp)
	}
}

// FindParentBlockOfStmID walks function blocks for the parent of stm_id.
// Used when StatementGoto::dest parent is not stored on Stmt.
// Block* always live on Function.Blocks; nil hole fails closed (nil — no invent skip).
func FindParentBlockOfStmID(f *Function, stmID int) *Block {
	if f == nil || stmID <= 0 {
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
	for _, b := range f.Blocks {
		if b == nil {
			return nil
		}
		if p := walk(b); p != nil {
			return p
		}
	}
	return nil
}

// FindStmtByID returns the statement with stm_id in f's block tree.
// Complements FindParentBlockOfStmID for CFG edge source resolution.
func FindStmtByID(f *Function, stmID int) *Stmt {
	if f == nil || stmID <= 0 {
		return nil
	}
	b := FindParentBlockOfStmID(f, stmID)
	if b == nil {
		return nil
	}
	for i := range b.Stmts {
		if b.Stmts[i].StmID == stmID {
			return &b.Stmts[i]
		}
	}
	return nil
}

// AddFactOut mirrors FactMgr::add_fact_out.
// FactMgr.cpp:281–308 — append one fact to map_facts_out if visible at stm;
// drop non-globals on return; drop loop-invisible on break/continue.
// Incomplete Param/LocalVars at visibility sites fail closed (no invent drop of
// a stack local solely because IsVarOnStack returned false past a hole — and no
// invent append when visibility cannot be decided). Non-globals require complete
// stack scan; incomplete → skip append.
func (fm *FactMgr) AddFactOut(st *Stmt, stParent *Block, fact *FactPointTo) {
	if fm == nil || st == nil || fact == nil || fact.Var == nil || st.StmID <= 0 {
		return
	}
	f := fm.Func
	// visibility needs complete stack for non-globals
	if f != nil && !fact.Var.IsGlobal() {
		if !f.StackScanComplete(stParent) {
			return
		}
		if !f.IsVarVisible(fact.Var, stParent) {
			return
		}
	} else if f != nil && !f.IsVarVisible(fact.Var, stParent) {
		return
	}
	switch st.Kind {
	case StmtReturn:
		if !fact.Var.IsGlobal() {
			return
		}
	case StmtBreak, StmtContinue:
		// find enclosing loop block
		b := stParent
		for b != nil && !b.Looping {
			b = b.Parent
		}
		if f != nil && !fact.Var.IsGlobal() {
			if !f.StackScanComplete(b) {
				return
			}
		}
		if f != nil && !f.IsVarVisible(fact.Var, b) {
			return
		}
	case StmtGoto:
		// FactMgr.cpp:296–300 — drop if var not visible at StatementGoto::dest.
		// Prefer GotoDestParent; else resolve parent of GotoDestStmID via function blocks.
		destParent := st.GotoDestParent
		if destParent == nil && st.GotoDestStmID > 0 && f != nil {
			destParent = FindParentBlockOfStmID(f, st.GotoDestStmID)
		}
		if destParent != nil && f != nil {
			if !fact.Var.IsGlobal() && !f.StackScanComplete(destParent) {
				return
			}
			if !f.IsVarVisible(fact.Var, destParent) {
				return
			}
		}
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	fm.MapFactsOut[st.StmID] = append(fm.MapFactsOut[st.StmID], fact.Clone())
}

// UpdateFactsForDest mirrors FactMgr::update_facts_for_dest.
// FactMgr.cpp:424–456 — merge facts; OOS locals at dest become garbage/dropped.
func UpdateFactsForDest(factsIn []*FactPointTo, factsOut *[]*FactPointTo, f *Function, destParent *Block) {
	if factsOut == nil {
		return
	}
	// FactMgr.cpp:427–428 — dest->func; assert(func)
	// no soft invent dest facts without function (OOS walk needs f)
	if f == nil {
		*factsOut = nil
		return
	}
	// Fact* always live; nil hole fails closed (no invent skip partial dest update)
	for _, fact := range factsIn {
		if fact == nil || fact.Var == nil {
			*factsOut = nil
			return
		}
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
		// skip return variables
		if isReturnVar(fact.Var) {
			continue
		}
		if f.IsVarOOS(fact.Var, destParent) {
			addOOS(fact.Var)
		}
		for _, p := range fact.PointTo {
			// Variable* always live in PointTo; nil hole fails closed whole dest update
			// (no invent soft-skip hole and still OOS-scan later pointees)
			if p == nil {
				*factsOut = nil
				return
			}
			if !IsSpecialPtr(p) && f.IsVarOOS(p, destParent) {
				addOOS(p)
			}
		}
		merged := MergeFactInto(*factsOut, fact)
		if merged == nil {
			*factsOut = nil
			return
		}
		*factsOut = merged
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
// Incomplete oldFacts / makeup fail closed (no invent clean clone + partial makeup).
func (fm *FactMgr) RestoreFacts(oldFacts []*FactPointTo) {
	if fm == nil {
		return
	}
	// nil oldFacts is empty restore; non-nil with holes → CloneFactSlice nil
	if oldFacts != nil && !FactsComplete(oldFacts) {
		fm.GlobalFacts = nil
		return
	}
	cp := CloneFactSlice(oldFacts)
	if !MakeupNewVarFacts(&cp, fm.GlobalFacts) {
		// incomplete GlobalFacts or mid-makeup hole — fail closed, no invent partial
		fm.GlobalFacts = nil
		return
	}
	fm.GlobalFacts = cp
}

// SetupInOutMaps mirrors FactMgr::setup_in_out_maps.
// FactMgr.cpp:208–246 — first_time clones into final; else combine.
// Fact* always live; incomplete source maps fail closed (nil final entry —
// no invent cleaned partial clone of holes on first_time or soft-join later).
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
			if !FactsComplete(facts) {
				fm.MapFactsInFinal[id] = nil
				continue
			}
			// CloneFactSlice nil = incomplete clone; no invent empty final from fail
			cl := CloneFactSlice(facts)
			if cl == nil && facts != nil {
				fm.MapFactsInFinal[id] = nil
				continue
			}
			fm.MapFactsInFinal[id] = cl
		}
		for id, facts := range fm.MapFactsOut {
			if !FactsComplete(facts) {
				fm.MapFactsOutFinal[id] = nil
				continue
			}
			cl := CloneFactSlice(facts)
			if cl == nil && facts != nil {
				fm.MapFactsOutFinal[id] = nil
				continue
			}
			fm.MapFactsOutFinal[id] = cl
		}
		return
	}
	// combine current maps into final
	// Fact* always live; incomplete maps or failed merge fail closed (nil final, no invent partial join)
	for id, facts2 := range fm.MapFactsIn {
		facts1 := fm.MapFactsInFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			fm.MapFactsInFinal[id] = nil
			continue
		}
		// MergeFacts clears *facts on incomplete mid-join; false alone may mean no lattice change
		_ = MergeFacts(&facts1, facts2)
		if !FactsComplete(facts1) {
			fm.MapFactsInFinal[id] = nil
			continue
		}
		fm.MapFactsInFinal[id] = facts1
	}
	for id, facts2 := range fm.MapFactsOut {
		facts1 := fm.MapFactsOutFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			fm.MapFactsOutFinal[id] = nil
			continue
		}
		_ = MergeFacts(&facts1, facts2)
		if !FactsComplete(facts1) {
			fm.MapFactsOutFinal[id] = nil
			continue
		}
		fm.MapFactsOutFinal[id] = facts1
	}
}

// BackupStmFactMaps mirrors FactMgr::backup_stm_fact_maps for a statement tree.
// FactMgr.cpp:516–531 — copy in/out maps for stm and nested blocks.
// Incomplete source maps store nil (no invent cleaned partial clone of holes).
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
			if !FactsComplete(in) {
				factsIn[st.StmID] = nil
			} else {
				factsIn[st.StmID] = CloneFactSlice(in)
			}
		}
		if out, ok := fm.MapFactsOut[st.StmID]; ok {
			if !FactsComplete(out) {
				factsOut[st.StmID] = nil
			} else {
				factsOut[st.StmID] = CloneFactSlice(out)
			}
		}
	}
}

func (fm *FactMgr) backupBlockFactMaps(b *Block, factsIn, factsOut map[int][]*FactPointTo) {
	if b == nil {
		return
	}
	if b.StmID > 0 {
		if in, ok := fm.MapFactsIn[b.StmID]; ok {
			if !FactsComplete(in) {
				factsIn[b.StmID] = nil
			} else {
				factsIn[b.StmID] = CloneFactSlice(in)
			}
		}
		if out, ok := fm.MapFactsOut[b.StmID]; ok {
			if !FactsComplete(out) {
				factsOut[b.StmID] = nil
			} else {
				factsOut[b.StmID] = CloneFactSlice(out)
			}
		}
	}
	for i := range b.Stmts {
		fm.BackupStmFactMaps(&b.Stmts[i], factsIn, factsOut)
	}
}

// RestoreStmFactMaps mirrors FactMgr::restore_stm_fact_maps.
// FactMgr.cpp:533–548.
// Incomplete backup entries restore as nil (no invent cleaned partial clone).
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
			if !FactsComplete(in) {
				fm.MapFactsIn[st.StmID] = nil
			} else {
				fm.MapFactsIn[st.StmID] = CloneFactSlice(in)
			}
		} else {
			delete(fm.MapFactsIn, st.StmID)
		}
		if out, ok := factsOut[st.StmID]; ok {
			if !FactsComplete(out) {
				fm.MapFactsOut[st.StmID] = nil
			} else {
				fm.MapFactsOut[st.StmID] = CloneFactSlice(out)
			}
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
			if !FactsComplete(in) {
				fm.MapFactsIn[b.StmID] = nil
			} else {
				fm.MapFactsIn[b.StmID] = CloneFactSlice(in)
			}
		} else {
			delete(fm.MapFactsIn, b.StmID)
		}
		if out, ok := factsOut[b.StmID]; ok {
			if !FactsComplete(out) {
				fm.MapFactsOut[b.StmID] = nil
			} else {
				fm.MapFactsOut[b.StmID] = CloneFactSlice(out)
			}
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
// Incomplete in/out maps fail closed (nil — no invent empty-update via hole skip).
func (fm *FactMgr) FindUpdatedFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.MapFactsIn[stmID]
	out := fm.MapFactsOut[stmID]
	if !FactsComplete(in) || !FactsComplete(out) {
		return nil
	}
	var updated []*FactPointTo
	for _, f := range out {
		// FactMgr.cpp:659–662 — assert(prev_f); only changed when prev exists
		// no soft invent "new out-only fact" as updated
		prev := FindRelatedPointTo(in, f.Var)
		if prev == nil {
			continue
		}
		if !f.Equal(prev) {
			updated = append(updated, f)
		}
	}
	return updated
}

// FindUpdatedFinalFacts mirrors FactMgr::find_updated_final_facts.
// FactMgr.cpp:667–686 — final maps; always include rv facts.
// Incomplete in/out maps fail closed (nil — no invent empty-update via hole skip).
func (fm *FactMgr) FindUpdatedFinalFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.MapFactsInFinal[stmID]
	out := fm.MapFactsOutFinal[stmID]
	if !FactsComplete(in) || !FactsComplete(out) {
		return nil
	}
	var updated []*FactPointTo
	for _, f := range out {
		// FactMgr.cpp:676–677 — rv facts always listed (no pre-fact required)
		if fm.Func != nil && fm.Func.RV != nil && fm.Func.RV.Match(f.Var) {
			updated = append(updated, f)
			continue
		}
		// FactMgr.cpp:679–682 — assert(prev_f); no soft invent missing prev as change
		prev := FindRelatedPointTo(in, f.Var)
		if prev == nil {
			continue
		}
		if !f.Equal(prev) {
			updated = append(updated, f)
		}
	}
	return updated
}

// RemoveLoopLocalFacts mirrors FactMgr::remove_loop_local_facts for a block.
// FactMgr.cpp:601–612 — collect locals from blk up through enclosing loop,
// then update_facts_for_oos_vars (drop subjects + mark pointees garbage).
// Incomplete facts/locals/clone fail closed nil (no invent cleaned OOS filter).
func RemoveLoopLocalFacts(facts []*FactPointTo, blk *Block) []*FactPointTo {
	if blk == nil {
		return facts
	}
	// incomplete facts fail closed before OOS
	if !FactsComplete(facts) {
		return nil
	}
	locals := collectLoopLocalVars(blk)
	// nil locals = incomplete LocalVars hole — fail closed
	if locals == nil {
		return nil
	}
	out := CloneFactSlice(facts)
	// complete empty clones as non-nil empty; nil clone = incomplete
	if out == nil && facts != nil {
		return nil
	}
	// Statement.cpp set_fact_out / FactMgr.cpp:607–611
	UpdateFactsForOOSVars(locals, &out)
	return out
}

// RemoveLoopLocalFactsForStmt mirrors remove_loop_local_facts(Statement*).
// FactMgr.cpp:603–605 — block stmt uses itself; else use parent.
func RemoveLoopLocalFactsForStmt(facts []*FactPointTo, st *Stmt, parent *Block) []*FactPointTo {
	b := parent
	if st != nil && st.Kind == StmtBlock && st.Then != nil {
		b = st.Then
	}
	return RemoveLoopLocalFacts(facts, b)
}

// collectLoopLocalVars walks blk → parents until a looping block (inclusive).
// FactMgr.cpp:605–610.
// Variable* always live on LocalVars; nil hole fails closed (nil — no invent skip).
// Empty complete walk returns non-nil empty slice.
func collectLoopLocalVars(blk *Block) []*Variable {
	locals := make([]*Variable, 0)
	b := blk
	for b != nil {
		for _, v := range b.LocalVars {
			if v == nil {
				return nil
			}
		}
		locals = append(locals, b.LocalVars...)
		if b.Looping {
			break
		}
		b = b.Parent
	}
	return locals
}

// RemoveFunctionLocalFacts mirrors FactMgr::remove_function_local_facts
// with stParent=Body (function exit). FactMgr.cpp:179–205.
func RemoveFunctionLocalFacts(facts []*FactPointTo, f *Function) []*FactPointTo {
	var parent *Block
	if f != nil {
		parent = f.Body
	}
	return RemoveFunctionLocalFactsAt(facts, f, parent)
}

// RemoveFunctionLocalFactsAt mirrors FactMgr::remove_function_local_facts.
// FactMgr.cpp:179–205 — drop stack/other-rv subjects; mark_func_end on remaining.
// Fact* always live; incomplete PointTo/fact holes or incomplete Param/LocalVars
// stack lists fail closed (nil out — no invent keep stack locals when IsVarOnStack
// returns false past a hole, or leave stack pointees live).
func RemoveFunctionLocalFactsAt(facts []*FactPointTo, f *Function, stParent *Block) []*FactPointTo {
	if !FactsComplete(facts) {
		return nil
	}
	if f != nil && stParent != nil && !f.StackScanComplete(stParent) {
		return nil
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, fact := range facts {
		// FactMgr.cpp:191–195 — is_var_on_stack OR other-function RV
		if f != nil && stParent != nil && f.IsVarOnStack(fact.Var, stParent) {
			continue
		}
		if fact.Var.IsRV() && (f == nil || f.RV == nil || !f.RV.Match(fact.Var)) {
			continue
		}
		cl := fact.Clone()
		if cl == nil {
			return nil
		}
		out = append(out, cl)
	}
	// FactMgr.cpp:196–204 — remaining facts may point to stack locals → garbage
	MarkFuncEndOnFacts(&out, f, stParent)
	// MarkFuncEndOnFacts clears *facts on incomplete after mark
	if !FactsComplete(out) {
		return nil
	}
	return out
}

// filterFactsNotInVars drops subjects in drop. Fact* always live;
// incomplete maps/pointees or incomplete drop list fail closed (nil out —
// no invent keep subjects that match only after a drop-list hole).
func filterFactsNotInVars(facts []*FactPointTo, drop []*Variable) []*FactPointTo {
	if len(drop) == 0 {
		return facts
	}
	if !FactsComplete(facts) || !VariablesComplete(drop) {
		return nil
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, f := range facts {
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

// ForFunc returns the FactMgr for f (session FMList).
// Prefer the FactMgr paired at make_random_signature / make_first (Function.cpp:422);
// only create when registering a function that has no paired entry yet.
// get_fact_mgr_for_func itself only looks up — create happens at signature time.
func (m *FactMgrMap) ForFunc(f *Function) *FactMgr {
	if m == nil || f == nil {
		return nil
	}
	if m.byFunc == nil {
		m.byFunc = make(map[*Function]*FactMgr)
	}
	if fm, ok := m.byFunc[f]; ok {
		// keep Function pairing in sync
		if f.factMgr == nil {
			f.factMgr = fm
		}
		return fm
	}
	// reuse paired FactMgr from signature create (no invent second manager)
	if f.factMgr != nil {
		m.byFunc[f] = f.factMgr
		return f.factMgr
	}
	fm := NewFactMgr(f)
	f.factMgr = fm
	m.byFunc[f] = fm
	return fm
}

// AbstractFactForVarInit mirrors Fact::abstract_fact_for_var_init.
// Fact.cpp:85–112 — pointer/union only; assign from init; array alt inits merge.
func AbstractFactForVarInit(v *Variable) (pt []*FactPointTo, un []*FactUnion) {
	if v == nil || v.Type == nil {
		return nil, nil
	}
	if !v.IsPointer() && !v.Type.IsUnion() {
		return nil, nil
	}
	var rhs *Expression
	if v.InitExpr != nil {
		rhs = v.InitExpr
	} else if v.Init != nil {
		rhs = &Expression{Term: TermConstant, Con: v.Init, ExprType: v.Type}
	}
	if v.Type.IsUnion() {
		un, _ = AbstractFactUnionForAssign(nil, nil, v, 0, rhs)
		return nil, un
	}
	// pointer (Fact.cpp:94–95)
	// Fact.cpp:94–95 — abstract_fact_for_assign; assert(lvar_cnt == 1)
	pt = AbstractFactForAssign(nil, v, 0, rhs)
	if len(pt) != 1 {
		// fail closed — no soft invent multi/zero LHS init facts
		return nil, nil
	}
	// Fact.cpp:97–109 — more init values on array of pointers
	// Fact.cpp:99 — assert(av) when isArray (AsArray set)
	if v.IsArray && v.AsArray == nil {
		return nil, nil
	}
	if av := v.AsArray; av != nil {
		// Fact.cpp:100–106 — get_more_init_values() Expression* only
		// no invent Constant from InitValues to_string() list
		for _, e := range av.InitExprs {
			// Expression* always live in C++; nil hole is broken IR — fail closed
			if e == nil {
				return nil, nil
			}
			more := AbstractFactForAssign(nil, v, 0, e)
			for _, f := range more {
				// Fact* always live; MergeFactInto nil = incomplete
				if f == nil {
					return nil, nil
				}
				merged := MergeFactInto(pt, f)
				if merged == nil {
					return nil, nil
				}
				pt = merged
			}
		}
	}
	return pt, nil
}

// AddNewVarFact mirrors FactMgr::add_new_var_fact for point-to/union init.
// FactMgr.cpp:118–131 + Fact::abstract_fact_for_var_init via meta_facts loop.
// AddNewVarFact abstracts init facts for a new variable into GlobalFacts.
// Variable* FieldVars always live; nil hole fails closed (GlobalFacts cleared —
// no invent soft-skip field hole and still keep partial prior field merges).
// Fact* from abstract always live; nil hole fails closed (GlobalFacts cleared).
func (fm *FactMgr) AddNewVarFact(v *Variable) {
	if fm == nil || v == nil {
		return
	}
	// recurse into aggregate fields (pointer members)
	if !v.IsPointer() && (v.Type == nil || !v.Type.IsUnion()) {
		for _, f := range v.FieldVars {
			if f == nil {
				// incomplete FieldVars — clear partial aggregate makeup
				fm.GlobalFacts = nil
				return
			}
			fm.AddNewVarFact(f)
			// child may have cleared on hole / merge fail
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = nil
				return
			}
		}
		return
	}
	// FactMgr.cpp:77–79 — only meta_facts that were registered
	wantPT := metaFactPointToEnabled && v.IsPointer()
	wantUn := metaFactUnionEnabled && v.Type != nil && v.Type.IsUnion()
	if !wantPT && !wantUn {
		return
	}
	if wantPT && FindRelatedPointTo(fm.GlobalFacts, v) != nil {
		return
	}
	if wantUn && FindRelatedUnion(fm.UnionFacts, v) != nil {
		return
	}
	pt, un := AbstractFactForVarInit(v)
	if wantPT {
		// Fact.cpp:94–95 assert(lvar_cnt==1) — no soft invent NewFactPointTo when empty
		for _, f := range pt {
			if f == nil {
				// incomplete abstract list — clear partial GlobalFacts, no invent keep half-merge
				fm.GlobalFacts = nil
				return
			}
			merged := MergeFactInto(fm.GlobalFacts, f)
			if merged == nil {
				fm.GlobalFacts = nil
				return
			}
			fm.GlobalFacts = merged
		}
	}
	if wantUn {
		for _, uf := range un {
			if uf == nil {
				fm.UnionFacts = nil
				return
			}
			merged := MergeUnionFact(fm.UnionFacts, uf)
			if merged == nil {
				fm.UnionFacts = nil
				return
			}
			fm.UnionFacts = merged
		}
		// union abstract returns fact(s) when valid; no invent TOP on empty/fail
	}
}

// AddNewVarFactAndUpdate mirrors add_new_var_fact_and_update_inout_maps.
// FactMgr.cpp:69–110 — abstract_fact_for_var_init into global_facts and
// map_facts_in/out for statements under blk (or all when blk is nil).
func (fm *FactMgr) AddNewVarFactAndUpdate(blk *Block, v *Variable) {
	if fm == nil || v == nil {
		return
	}
	// FactMgr.cpp:72 — assert(var->is_global()) when blk==nullptr
	// no soft invent facts for non-global "global create" path
	if blk == nil && !v.IsGlobal() {
		return
	}
	// snapshot length to detect newly merged facts
	beforePT := len(fm.GlobalFacts)
	fm.AddNewVarFact(v)
	// FactMgr.cpp:77–104 — push each new init fact into maps
	// Fact* always live after add; nil hole fails closed (no invent partial map push)
	// no invent MapFactsIn-only push when MapFactsOut is nil (one-sided invent)
	for i := beforePT; i < len(fm.GlobalFacts); i++ {
		f := fm.GlobalFacts[i]
		if f == nil {
			return
		}
		// map_facts_in: stm in_block(blk) || blk==null
		if fm.MapFactsIn != nil {
			for id := range fm.MapFactsIn {
				if blk == nil || stmtIDInBlock(fm.Func, id, blk) {
					fm.MapFactsIn[id] = append(fm.MapFactsIn[id], f.Clone())
				}
			}
		}
		// map_facts_out — required when In was updated for dual-map coherence
		if fm.MapFactsOut == nil {
			// incomplete FM — stop further dual push (no invent skip remaining facts)
			if fm.MapFactsIn != nil {
				return
			}
			continue
		}
		if blk == nil {
			// FactMgr.cpp:102–103 — append to all outs
			for id := range fm.MapFactsOut {
				fm.MapFactsOut[id] = append(fm.MapFactsOut[id], f.Clone())
			}
		} else {
			// FactMgr.cpp:99–100 — add_fact_out(stm, f) with visibility filters
			// Statement* always resolvable for ids under blk; nil fails closed
			// (stop map push — no invent skip stale/missing stm as absent)
			for id := range fm.MapFactsOut {
				if !stmtIDInBlock(fm.Func, id, blk) {
					continue
				}
				st := FindStmtByID(fm.Func, id)
				if st == nil {
					return
				}
				parent := FindParentBlockOfStmID(fm.Func, id)
				fm.AddFactOut(st, parent, f)
			}
		}
	}
}

// stmtIDInBlock reports Statement::in_block(blk) for a statement id under func.
func stmtIDInBlock(f *Function, stmID int, blk *Block) bool {
	if blk == nil || stmID <= 0 {
		return false
	}
	// BlockContainsStmID walks nested Then/Else under blk
	return BlockContainsStmID(blk, stmID)
}

// lhsAssignPointees mirrors merge_pointees_of_pointer used by abstract_fact_for_assign.
// Used to decide renew (lvar_cnt==1) vs merge (may-point-to).
func lhsAssignPointees(facts []*FactPointTo, lhs *Variable, lhsIndir int) []*Variable {
	if lhs == nil {
		return nil
	}
	lvars := MergePointeesOfPointer(lhs.GetCollective(), lhsIndir, facts)
	if lhsIndir == 0 && lhs.Type != nil && lhs.Type.ptrTo != nil && len(lvars) == 0 {
		lvars = []*Variable{lhs.GetCollective()}
	}
	return lvars
}

// applyPointToAssignFacts applies point-to facts from abstract_fact_for_assign.
// FactMgr.cpp:376–388 — renew when definitive single non-array LHS; else merge.
// Returns (changed, ok). ok=false means incomplete map/merge — *facts cleared
// (no invent success past holes / soft-continue merge list / treat incomplete as no-op).
// empty newFacts is ok with changed=false.
func applyPointToAssignFacts(facts *[]*FactPointTo, lhs *Variable, lhsIndir int, newFacts []*FactPointTo) (changed bool, ok bool) {
	if facts == nil {
		return false, false
	}
	if len(newFacts) == 0 {
		return false, true
	}
	if !FactsComplete(*facts) || !FactsComplete(newFacts) {
		*facts = nil
		return false, false
	}
	lvarCnt := len(lhsAssignPointees(*facts, lhs, lhsIndir))
	// when AbstractFactForAssign used direct pointer path, lvarCnt matches transfer targets
	if lvarCnt == 0 && lhs != nil && lhsIndir == 0 && lhs.IsPointer() {
		lvarCnt = 1
	}
	if lvarCnt == 1 && newFacts[0] != nil && newFacts[0].Var != nil && !newFacts[0].Var.IsArray {
		// definitive assignment — renew (strong replace)
		_ = RenewFact(facts, newFacts[0])
		for j := 1; j < len(newFacts); j++ {
			merged := MergeFactInto(*facts, newFacts[j])
			if merged == nil {
				*facts = nil
				return false, false
			}
			*facts = merged
		}
		return true, true
	}
	for _, f := range newFacts {
		merged := MergeFactInto(*facts, f)
		if merged == nil {
			*facts = nil
			return false, false
		}
		*facts = merged
	}
	return true, true
}

// UpdateFactForAssign mirrors FactMgr::update_fact_for_assign(Lhs, Expression, facts).
// FactMgr.cpp:370–395 — renew vs merge; FactUnion abstract_fact_for_assign.
// Incomplete point-to apply or union merge fails closed (false; GlobalFacts and/or
// UnionFacts cleared — no invent continue union after wiped point-to or partial maps).
func (fm *FactMgr) UpdateFactForAssign(lhs *Variable, lhsIndir int, rhs *Expression) bool {
	if fm == nil || lhs == nil {
		return false
	}
	changed := false
	newFacts := AbstractFactForAssign(fm.GlobalFacts, lhs, lhsIndir, rhs)
	// incomplete abstract (lvars/transfer hole) — nil newFacts with pointer lhs
	// that expected transfer must not invent empty apply success then union merge
	ptChanged, ptOK := applyPointToAssignFacts(&fm.GlobalFacts, lhs, lhsIndir, newFacts)
	if !ptOK {
		// point-to incomplete — fail closed, do not invent union merge on wiped map
		fm.UnionFacts = nil
		return false
	}
	if ptChanged {
		changed = true
	}
	// FactUnion::abstract_fact_for_assign (meta_facts loop)
	ufacts, _ := AbstractFactUnionForAssign(fm.UnionFacts, fm.GlobalFacts, lhs, lhsIndir, rhs)
	for _, uf := range ufacts {
		// FactUnion* always live from abstract; nil hole fails closed
		if uf == nil {
			fm.UnionFacts = nil
			return false
		}
		merged := MergeUnionFact(fm.UnionFacts, uf)
		if merged == nil {
			fm.UnionFacts = nil
			return false
		}
		fm.UnionFacts = merged
		changed = true
	}
	// FactMgr.cpp:400 — assign that changes facts marks function fact_changed
	if changed && fm.Func != nil {
		fm.Func.FactChanged = true
	}
	return changed
}

// MergeUnionFact replaces or appends FactUnion for the same union var.
// MergeUnionFact replaces or appends a union fact by subject.
// FactUnion* always live; nil f or map hole fails closed (nil out).
// Incomplete map must not invent replace success when match appears before a hole.
func MergeUnionFact(facts []*FactUnion, f *FactUnion) []*FactUnion {
	if f == nil {
		return nil
	}
	for _, old := range facts {
		if old == nil {
			return nil
		}
	}
	for i, old := range facts {
		if old.Var == f.Var {
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
	if fm == nil || srcID == 0 {
		return
	}
	// allow dest nil when destStmID set (break → for-statement edge)
	if dest == nil && destStmID <= 0 {
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
// Fact* always live; incomplete old/new maps or AddNewVarFactInto field holes fail
// closed (nil oldFacts, false — no invent soft-skip holes as absent new var,
// partial makeup, or re-accumulate later vars after *oldFacts was cleared).
// Returns true when makeup completed with a complete *oldFacts accumulator.
func MakeupNewVarFacts(oldFacts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	if oldFacts == nil {
		return false
	}
	// incomplete working/snapshot sets fail closed before partial makeup
	if !FactsComplete(*oldFacts) || !FactsComplete(newFacts) {
		*oldFacts = nil
		return false
	}
	for _, f := range newFacts {
		// no invent soft-continue past nil fact holes (also covered by FactsComplete)
		if f == nil || f.Var == nil {
			*oldFacts = nil
			return false
		}
		v := f.Var
		if !v.IsGlobal() && !v.IsLocal() {
			continue
		}
		if FindRelatedPointTo(*oldFacts, v) == nil {
			// FactMgr.cpp:504 — add_new_var_fact(v, old_facts) → abstract_fact_for_var_init
			// no invent NewFactPointTo garbage (tbd/garbage default) for live inits
			AddNewVarFactInto(v, oldFacts)
			// AddNewVarFactInto may clear *oldFacts on FieldVars/abstract holes;
			// stop — no invent continue loop and re-append later vars onto nil.
			if *oldFacts == nil || !FactsComplete(*oldFacts) {
				*oldFacts = nil
				return false
			}
		}
	}
	return true
}

// AddNewVarFactInto mirrors FactMgr::add_new_var_fact(v, facts).
// FactMgr.cpp:118–131 — abstract_fact_for_var_init into the given fact slice.
// Variable* FieldVars always live; nil hole fails closed (*facts = nil — no invent
// soft-skip hole and still makeup later fields as complete).
func AddNewVarFactInto(v *Variable, facts *[]*FactPointTo) {
	if facts == nil {
		return
	}
	// Variable* always live; nil v hole fails closed (clear — no invent skip as absent)
	if v == nil {
		*facts = nil
		return
	}
	// FactMgr.cpp:77–79 — only when PointTo meta_facts registered
	if !metaFactPointToEnabled {
		return
	}
	// recurse into aggregate fields (pointer members) like AddNewVarFact
	if !v.IsPointer() && (v.Type == nil || !v.Type.IsUnion()) {
		for _, f := range v.FieldVars {
			if f == nil {
				*facts = nil
				return
			}
			AddNewVarFactInto(f, facts)
			if *facts == nil {
				return
			}
		}
		return
	}
	if !v.IsPointer() {
		return
	}
	if FindRelatedPointTo(*facts, v) != nil {
		return
	}
	pt, _ := AbstractFactForVarInit(v)
	// Fact.cpp:94–95 assert(lvar_cnt==1) — no invent garbage shell when empty
	// Fact* always live from abstract; nil hole fails closed (*facts cleared)
	for _, f := range pt {
		if f == nil {
			*facts = nil
			return
		}
		if FindRelatedPointTo(*facts, f.Var) == nil {
			*facts = append(*facts, f.Clone())
		}
	}
}

// FindDanglingGlobalPtrs mirrors FactMgr::find_dangling_global_ptrs.
// FactMgr.cpp:688–700 — non-const global pointers that are dead at function exit.
// Fact* always live; nil hole fails closed (empty DeadGlobals, no invent partial).
func (fm *FactMgr) FindDanglingGlobalPtrs(f *Function) {
	if fm == nil || f == nil {
		return
	}
	f.DeadGlobals = f.DeadGlobals[:0]
	if !FactsComplete(fm.GlobalFacts) {
		return
	}
	for _, fact := range fm.GlobalFacts {
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

// UpdateFactForReturn mirrors FactMgr::update_fact_for_return without set_fact_out.
// Prefer UpdateFactForReturnStmt when the return Statement is available.
func (fm *FactMgr) UpdateFactForReturn(rv *Variable, expr *Expression) bool {
	return fm.UpdateFactForReturnStmt(nil, rv, expr)
}

// UpdateFactForReturnStmt mirrors FactMgr::update_fact_for_return.
// FactMgr.cpp:406–421 — abstract_fact_for_return into global_facts; set_fact_out(sr).
// Incomplete assign fails closed (false; no invent SetMapFactsOut from wiped map).
func (fm *FactMgr) UpdateFactForReturnStmt(st *Stmt, rv *Variable, expr *Expression) bool {
	// Expression* always live on StatementReturn; nil expr fails closed
	// (no invent garbage RHS transfer as stand-in for missing return value IR)
	if fm == nil || rv == nil || expr == nil {
		return false
	}
	// abstract_fact_for_return ≈ abstract_fact_for_assign(facts, Lhs(rv), expr)
	// FactMgr.cpp:408–416 — merge into inputs; fact_changed on merge
	changed := fm.UpdateFactForAssign(rv, 0, expr)
	// incomplete GlobalFacts after assign — do not invent return out map
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = nil
		return false
	}
	// FactMgr.cpp:418–420 — incorporate current facts into return outs
	if st != nil {
		// set_fact_out for return drops function-locals (FactMgr.cpp:270–272)
		fm.SetMapFactsOutForStmt(st, fm.GlobalFacts, nil)
	}
	return changed
}

// UpdateFactsForOOSVars mirrors FactMgr::update_facts_for_oos_vars.
// FactMgr.cpp:141–172 — drop facts for oos vars; mark pointees garbage.
// Delegates to package UpdateFactsForOOSVars (fail closed on fact/var holes).
func (fm *FactMgr) UpdateFactsForOOSVars(vars []*Variable) {
	if fm == nil || len(vars) == 0 {
		return
	}
	// reuse slice-level fail-closed filter (nil holes → GlobalFacts nil)
	facts := fm.GlobalFacts
	UpdateFactsForOOSVars(vars, &facts)
	fm.GlobalFacts = facts
}

// AddParamFacts mirrors FactMgr::add_param_facts.
// FactMgr.cpp:108–116 — update_fact_for_assign each param from arg expression.
// No invent NewFactPointTo when arg missing: nil rhs goes through abstract
// (FactPointTo.cpp:168–169 → garbage for pointers), same as C++ nullptr value.
// Variable* params always live; nil param hole or incomplete assign fails closed
// (*facts nil, stop — no invent skip remaining params / re-accumulate after wipe).
func (fm *FactMgr) AddParamFacts(args []*Expression, facts *[]*FactPointTo) {
	if fm == nil || fm.Func == nil || facts == nil {
		return
	}
	for i, p := range fm.Func.Param {
		if p == nil {
			// incomplete Param list — no invent skip remaining params
			*facts = nil
			return
		}
		var arg *Expression
		if i < len(args) {
			arg = args[i]
		}
		// FactMgr.cpp:113–114 — always update_fact_for_assign (all params, not pointer-only)
		// false alone may mean no lattice change; incomplete clears *facts
		_ = fm.UpdateFactForAssignInto(p, 0, arg, facts)
		if !FactsComplete(*facts) {
			*facts = nil
			return
		}
	}
}

// UpdateFactForAssignInto is UpdateFactForAssign writing into a fact slice.
// FactMgr.cpp:370–395 — same renew/merge rules as UpdateFactForAssign.
// Incomplete point-to apply or union merge fails closed like UpdateFactForAssign
// (no invent continue union after wiped *facts).
func (fm *FactMgr) UpdateFactForAssignInto(lhs *Variable, lhsIndir int, rhs *Expression, facts *[]*FactPointTo) bool {
	if facts == nil || lhs == nil {
		return false
	}
	changed := false
	newFacts := AbstractFactForAssign(*facts, lhs, lhsIndir, rhs)
	ptChanged, ptOK := applyPointToAssignFacts(facts, lhs, lhsIndir, newFacts)
	if !ptOK {
		if fm != nil {
			fm.UnionFacts = nil
		}
		return false
	}
	if ptChanged {
		changed = true
	}
	if fm != nil {
		ufacts, _ := AbstractFactUnionForAssign(fm.UnionFacts, *facts, lhs, lhsIndir, rhs)
		for _, uf := range ufacts {
			if uf == nil {
				fm.UnionFacts = nil
				return false
			}
			merged := MergeUnionFact(fm.UnionFacts, uf)
			if merged == nil {
				fm.UnionFacts = nil
				return false
			}
			fm.UnionFacts = merged
			changed = true
		}
		if changed && fm.Func != nil {
			fm.Func.FactChanged = true
		}
	}
	return changed
}

// PointsTo reports whether this fact's set contains v.
// Incomplete PointTo (nil hole) fails closed true — no invent not-points-to past holes.
func (f *FactPointTo) PointsTo(v *Variable) bool {
	if f == nil || v == nil {
		return false
	}
	for _, p := range f.PointTo {
		if p == nil {
			return true
		}
		if p == v {
			return true
		}
	}
	return false
}

// CallerToCalleeHandover mirrors FactMgr::caller_to_callee_handover.
// FactMgr.cpp:312–353 — param facts; keep globals/params and transitively pointed stack vars.
// Fact* always live; nil hole fails closed (inputs nil, no invent clean partition).
// Incomplete Param list fails closed (nil inputs — no invent drop param facts
// because IsVariableInSet returned false past a Param hole).
func (fm *FactMgr) CallerToCalleeHandover(args []*Expression, inputs *[]*FactPointTo) {
	// FactMgr always bound to a Function; nil Func is broken IR (no invent param partition)
	if fm == nil || inputs == nil || fm.Func == nil {
		return
	}
	if !VariablesComplete(fm.Func.Param) {
		*inputs = nil
		return
	}
	fm.AddParamFacts(args, inputs)
	if !FactsComplete(*inputs) {
		*inputs = nil
		return
	}
	// partition: keep globals and params
	var keep, rest []*FactPointTo
	for _, f := range *inputs {
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
			for _, kf := range keep {
				if kf.PointsTo(rf.Var) {
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
// Fact* always live; nil hole fails closed (facts nil, no invent clean filter).
func (fm *FactMgr) RemoveRVFacts(facts *[]*FactPointTo) {
	if fm == nil || facts == nil {
		return
	}
	if !FactsComplete(*facts) {
		*facts = nil
		return
	}
	out := make([]*FactPointTo, 0, len(*facts))
	for _, f := range *facts {
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
