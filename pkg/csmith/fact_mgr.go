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
	fm.MapFactsIn[stmID] = storeFactMapEntry(facts)
}

// SetMapFactsOut records post-statement facts.
func (fm *FactMgr) SetMapFactsOut(stmID int, facts []*FactPointTo) {
	if fm == nil || stmID <= 0 {
		return
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	fm.MapFactsOut[stmID] = storeFactMapEntry(facts)
}

// storeFactMapEntry normalizes fact-map values so incomplete is not confused with
// complete empty. FactsComplete(nil)==true would invent empty success if incomplete
// were stored as bare nil; incomplete uses a nil-hole slice (FactsComplete false).
// Complete empty uses a non-nil empty slice (FactsComplete true).
func storeFactMapEntry(facts []*FactPointTo) []*FactPointTo {
	if !FactsComplete(facts) {
		return IncompleteFactSlice()
	}
	cl := CloneFactSlice(facts)
	if cl == nil {
		return []*FactPointTo{}
	}
	return cl
}

// GetMapFactsIn returns map_facts_in for a live stm_id.
// StmID ≤0 fails closed IncompleteFactSlice (no invent MapFactsIn[0] miss as
// empty-complete merge/visit success). Missing live key → complete empty {}.
func (fm *FactMgr) GetMapFactsIn(stmID int) []*FactPointTo {
	if stmID <= 0 {
		return IncompleteFactSlice()
	}
	if fm == nil || fm.MapFactsIn == nil {
		return []*FactPointTo{}
	}
	if facts, ok := fm.MapFactsIn[stmID]; ok {
		if !FactsComplete(facts) {
			return IncompleteFactSlice()
		}
		return facts
	}
	return []*FactPointTo{}
}

// GetMapFactsOut returns map_facts_out for a live stm_id.
// StmID ≤0 fails closed IncompleteFactSlice (no invent MapFactsOut[0] empty-complete).
// Missing live key → complete empty {}.
func (fm *FactMgr) GetMapFactsOut(stmID int) []*FactPointTo {
	if stmID <= 0 {
		return IncompleteFactSlice()
	}
	if fm == nil || fm.MapFactsOut == nil {
		return []*FactPointTo{}
	}
	if facts, ok := fm.MapFactsOut[stmID]; ok {
		if !FactsComplete(facts) {
			return IncompleteFactSlice()
		}
		return facts
	}
	return []*FactPointTo{}
}

// GetMapFactsInFinal is GetMapFactsIn for map_facts_in_final.
func (fm *FactMgr) GetMapFactsInFinal(stmID int) []*FactPointTo {
	if stmID <= 0 {
		return IncompleteFactSlice()
	}
	if fm == nil || fm.MapFactsInFinal == nil {
		return []*FactPointTo{}
	}
	if facts, ok := fm.MapFactsInFinal[stmID]; ok {
		if !FactsComplete(facts) {
			return IncompleteFactSlice()
		}
		return facts
	}
	return []*FactPointTo{}
}

// GetMapFactsOutFinal is GetMapFactsOut for map_facts_out_final.
func (fm *FactMgr) GetMapFactsOutFinal(stmID int) []*FactPointTo {
	if stmID <= 0 {
		return IncompleteFactSlice()
	}
	if fm == nil || fm.MapFactsOutFinal == nil {
		return []*FactPointTo{}
	}
	if facts, ok := fm.MapFactsOutFinal[stmID]; ok {
		if !FactsComplete(facts) {
			return IncompleteFactSlice()
		}
		return facts
	}
	return []*FactPointTo{}
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
	// Statement::stm_id always live; StmID 0 fails closed sticky (no invent silent
	// set_fact_out success without map entry / soft re-pick past missing out)
	if st.StmID <= 0 {
		SetError(ErrGeneric)
		return
	}
	// incomplete source facts fail closed sticky — hole marker (not SetMapFactsOut(nil)
	// which storeFactMapEntry would invent as complete empty)
	if !FactsComplete(facts) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		SetError(ErrGeneric)
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
		// when dest unknown (wrong filter vs update_facts_for_dest).
		// IncompleteFactSlice sticky — bare nil + SetMapFactsOut invents complete empty.
		if fm.Func == nil {
			cp = IncompleteFactSlice()
			SetError(ErrGeneric)
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
	// incomplete after filter/dest — store hole sticky (no invent complete out map)
	if !FactsComplete(cp) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	fm.SetMapFactsOut(st.StmID, cp)
}

// FindParentBlockOfStmID walks function blocks for the parent of stm_id.
// Used when StatementGoto::dest parent is not stored on Stmt.
// Block* always live on Function.Blocks; nil hole fails closed (nil — no invent skip).
// Nested walk uses get_blocks only; incomplete if-arm skips that compound's children
// (no invent soft-skip missing arm then find under sibling arm of same if).
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
// Incomplete Param/LocalVars at visibility sites fail closed sticky IncompleteFactSlice
// on map entry (no invent soft-skip append as absent / soft re-pick past incomplete out).
// Incomplete fact PointTo also fails closed sticky hole marker (no invent clone partial).
func (fm *FactMgr) AddFactOut(st *Stmt, stParent *Block, fact *FactPointTo) {
	if fm == nil || st == nil || fact == nil || fact.Var == nil {
		return
	}
	// StmID 0 fails closed sticky (no invent silent add_fact_out without map entry)
	if st.StmID <= 0 {
		SetError(ErrGeneric)
		return
	}
	// ensure map exists before fail-closed writes
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	// incomplete subject fact — sticky hole marker (not soft-skip or invent cleaned clone)
	if !FactsComplete([]*FactPointTo{fact}) {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// already incomplete out map — stay incomplete sticky (no invent append onto hole)
	if prev, ok := fm.MapFactsOut[st.StmID]; ok && !FactsComplete(prev) {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	f := fm.Func
	// visibility needs complete stack for non-globals
	if f != nil && !fact.Var.IsGlobal() {
		if !f.StackScanComplete(stParent) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			SetError(ErrGeneric)
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
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				SetError(ErrGeneric)
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
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			if !f.IsVarVisible(fact.Var, destParent) {
				return
			}
		}
	}
	cl := fact.Clone()
	if cl == nil {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	fm.MapFactsOut[st.StmID] = append(fm.MapFactsOut[st.StmID], cl)
}

// UpdateFactsForDest mirrors FactMgr::update_facts_for_dest.
// FactMgr.cpp:424–456 — merge facts; OOS locals at dest become garbage/dropped.
// Incomplete inputs fail closed sticky via IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-complete dest facts / soft re-pick past wipe).
func UpdateFactsForDest(factsIn []*FactPointTo, factsOut *[]*FactPointTo, f *Function, destParent *Block) {
	if factsOut == nil {
		return
	}
	// FactMgr.cpp:427–428 — dest->func; assert(func)
	// no soft invent dest facts without function (OOS walk needs f)
	if f == nil {
		*factsOut = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Fact* always live; nil hole fails closed sticky (no invent skip partial dest update)
	for _, fact := range factsIn {
		if fact == nil || fact.Var == nil {
			*factsOut = IncompleteFactSlice()
			SetError(ErrGeneric)
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
			// Variable* always live in PointTo; nil hole fails closed whole dest update sticky
			// (no invent soft-skip hole and still OOS-scan later pointees)
			if p == nil {
				*factsOut = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			if !IsSpecialPtr(p) && f.IsVarOOS(p, destParent) {
				addOOS(p)
			}
		}
		merged := MergeFactInto(*factsOut, fact)
		if !FactsComplete(merged) {
			*factsOut = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
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
// Incomplete oldFacts / makeup fail closed sticky (no invent clean clone + partial
// makeup, no soft re-pick past wiped GlobalFacts).
func (fm *FactMgr) RestoreFacts(oldFacts []*FactPointTo) {
	if fm == nil {
		return
	}
	// nil oldFacts is empty restore; non-nil with holes → CloneFactSlice nil
	if oldFacts != nil && !FactsComplete(oldFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	cp := CloneFactSlice(oldFacts)
	if !MakeupNewVarFacts(&cp, fm.GlobalFacts) {
		// incomplete GlobalFacts or mid-makeup hole — fail closed sticky
		fm.GlobalFacts = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	fm.GlobalFacts = cp
}

// SetupInOutMaps mirrors FactMgr::setup_in_out_maps.
// FactMgr.cpp:208–246 — first_time clones into final; else combine.
// Fact* always live; incomplete source maps fail closed sticky (final hole marker —
// no invent cleaned partial clone / soft re-pick past incomplete finals).
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
			// storeFactMapEntry: incomplete → hole marker sticky
			if !FactsComplete(facts) {
				fm.MapFactsInFinal[id] = IncompleteFactSlice()
				SetError(ErrGeneric)
				continue
			}
			fm.MapFactsInFinal[id] = storeFactMapEntry(facts)
		}
		for id, facts := range fm.MapFactsOut {
			if !FactsComplete(facts) {
				fm.MapFactsOutFinal[id] = IncompleteFactSlice()
				SetError(ErrGeneric)
				continue
			}
			fm.MapFactsOutFinal[id] = storeFactMapEntry(facts)
		}
		return
	}
	// combine current maps into final
	// Fact* always live; incomplete maps or failed merge fail closed sticky
	// (no invent partial join or bare-nil complete empty)
	for id, facts2 := range fm.MapFactsIn {
		facts1 := fm.MapFactsInFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			// hole marker sticky (not bare nil — FactsComplete(nil) invents empty complete)
			fm.MapFactsInFinal[id] = IncompleteFactSlice()
			SetError(ErrGeneric)
			continue
		}
		// MergeFacts clears *facts sticky on incomplete mid-join
		_ = MergeFacts(&facts1, facts2)
		if !FactsComplete(facts1) {
			fm.MapFactsInFinal[id] = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			continue
		}
		fm.MapFactsInFinal[id] = storeFactMapEntry(facts1)
	}
	for id, facts2 := range fm.MapFactsOut {
		facts1 := fm.MapFactsOutFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			fm.MapFactsOutFinal[id] = IncompleteFactSlice()
			SetError(ErrGeneric)
			continue
		}
		_ = MergeFacts(&facts1, facts2)
		if !FactsComplete(facts1) {
			fm.MapFactsOutFinal[id] = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
			continue
		}
		fm.MapFactsOutFinal[id] = storeFactMapEntry(facts1)
	}
}

// BackupStmFactMaps mirrors FactMgr::backup_stm_fact_maps for a statement tree.
// FactMgr.cpp:516–531 — copy in/out maps for stm and nested blocks.
// Incomplete source maps store hole markers (no invent cleaned partial clones).
// Incomplete get_blocks tree (nil if-arm) fails closed sticky: root maps backed as
// IncompleteFactSlice (no invent root-only complete backup / soft re-pick past hole).
func (fm *FactMgr) BackupStmFactMaps(st *Stmt, factsIn, factsOut map[int][]*FactPointTo) {
	if fm == nil || st == nil {
		return
	}
	if factsIn == nil || factsOut == nil {
		return
	}
	blks := GetBlocksStmt(st)
	incomplete := false
	for _, b := range blks {
		if b == nil {
			incomplete = true
			break
		}
	}
	if incomplete {
		// fail closed sticky whole stm backup — not invent complete root + missing nested
		if st.StmID > 0 {
			factsIn[st.StmID] = IncompleteFactSlice()
			factsOut[st.StmID] = IncompleteFactSlice()
		}
		SetError(ErrGeneric)
		return
	}
	for _, b := range blks {
		fm.backupBlockFactMaps(b, factsIn, factsOut)
	}
	if st.StmID > 0 {
		if in, ok := fm.MapFactsIn[st.StmID]; ok {
			factsIn[st.StmID] = storeFactMapEntry(in)
		}
		if out, ok := fm.MapFactsOut[st.StmID]; ok {
			factsOut[st.StmID] = storeFactMapEntry(out)
		}
	}
}

func (fm *FactMgr) backupBlockFactMaps(b *Block, factsIn, factsOut map[int][]*FactPointTo) {
	if b == nil {
		return
	}
	if b.StmID > 0 {
		if in, ok := fm.MapFactsIn[b.StmID]; ok {
			factsIn[b.StmID] = storeFactMapEntry(in)
		}
		if out, ok := fm.MapFactsOut[b.StmID]; ok {
			factsOut[b.StmID] = storeFactMapEntry(out)
		}
	}
	for i := range b.Stmts {
		fm.BackupStmFactMaps(&b.Stmts[i], factsIn, factsOut)
	}
}

// RestoreStmFactMaps mirrors FactMgr::restore_stm_fact_maps.
// FactMgr.cpp:533–548.
// Incomplete backup entries restore as hole markers (storeFactMapEntry).
// Incomplete get_blocks tree fails closed sticky: root maps set IncompleteFactSlice
// (no invent soft-skip nil arm then restore root/sibling as complete tree).
func (fm *FactMgr) RestoreStmFactMaps(st *Stmt, factsIn, factsOut map[int][]*FactPointTo) {
	if fm == nil || st == nil {
		return
	}
	if fm.MapFactsIn == nil {
		fm.MapFactsIn = make(map[int][]*FactPointTo)
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	blks := GetBlocksStmt(st)
	incomplete := false
	for _, b := range blks {
		if b == nil {
			incomplete = true
			break
		}
	}
	if incomplete {
		if st.StmID > 0 {
			fm.MapFactsIn[st.StmID] = IncompleteFactSlice()
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		}
		SetError(ErrGeneric)
		return
	}
	for _, b := range blks {
		fm.restoreBlockFactMaps(b, factsIn, factsOut)
	}
	if st.StmID > 0 {
		if in, ok := factsIn[st.StmID]; ok {
			fm.MapFactsIn[st.StmID] = storeFactMapEntry(in)
		} else {
			delete(fm.MapFactsIn, st.StmID)
		}
		if out, ok := factsOut[st.StmID]; ok {
			fm.MapFactsOut[st.StmID] = storeFactMapEntry(out)
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
			fm.MapFactsIn[b.StmID] = storeFactMapEntry(in)
		} else {
			delete(fm.MapFactsIn, b.StmID)
		}
		if out, ok := factsOut[b.StmID]; ok {
			fm.MapFactsOut[b.StmID] = storeFactMapEntry(out)
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
// Incomplete in/out maps fail closed IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-update success via hole skip).
func (fm *FactMgr) FindUpdatedFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.GetMapFactsIn(stmID)
	out := fm.GetMapFactsOut(stmID)
	if !FactsComplete(in) || !FactsComplete(out) {
		return IncompleteFactSlice()
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
// Incomplete in/out maps fail closed IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-update success via hole skip).
func (fm *FactMgr) FindUpdatedFinalFacts(stmID int) []*FactPointTo {
	if fm == nil || stmID <= 0 {
		return nil
	}
	in := fm.GetMapFactsInFinal(stmID)
	out := fm.GetMapFactsOutFinal(stmID)
	if !FactsComplete(in) || !FactsComplete(out) {
		return IncompleteFactSlice()
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
// Incomplete facts/locals/clone fail closed sticky (no invent cleaned OOS filter
// / soft re-pick past wiped break-continue out maps).
func RemoveLoopLocalFacts(facts []*FactPointTo, blk *Block) []*FactPointTo {
	if blk == nil {
		return facts
	}
	// incomplete facts fail closed sticky before OOS
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
	}
	locals := collectLoopLocalVars(blk)
	// incomplete LocalVars hole — fail closed sticky
	if !VariablesComplete(locals) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
	}
	out := CloneFactSlice(facts)
	// incomplete clone is hole marker sticky (not bare nil invent empty complete)
	if !FactsComplete(out) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
	}
	// Statement.cpp set_fact_out / FactMgr.cpp:607–611
	UpdateFactsForOOSVars(locals, &out)
	if !FactsComplete(out) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return IncompleteFactSlice()
	}
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
// Variable* always live on LocalVars; nil hole fails closed IncompleteVariables
// (not bare nil invent empty-complete loop-local set).
// Empty complete walk returns non-nil empty slice.
func collectLoopLocalVars(blk *Block) []*Variable {
	locals := make([]*Variable, 0)
	b := blk
	for b != nil {
		if !VariablesComplete(b.LocalVars) {
			return IncompleteVariables()
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
// stack lists fail closed sticky (no invent keep stack locals when IsVarOnStack
// returns false past a hole, or soft re-pick past wiped return out maps).
func RemoveFunctionLocalFactsAt(facts []*FactPointTo, f *Function, stParent *Block) []*FactPointTo {
	if !FactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
	}
	if f != nil && stParent != nil && !f.StackScanComplete(stParent) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
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
			SetError(ErrGeneric)
			return IncompleteFactSlice()
		}
		out = append(out, cl)
	}
	// FactMgr.cpp:196–204 — remaining facts may point to stack locals → garbage
	MarkFuncEndOnFacts(&out, f, stParent)
	// MarkFuncEndOnFacts clears *facts on incomplete after mark sticky
	if !FactsComplete(out) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	return out
}

// filterFactsNotInVars drops subjects in drop. Fact* always live;
// incomplete maps/pointees or incomplete drop list fail closed sticky
// (no invent keep subjects that match only after a drop-list hole).
func filterFactsNotInVars(facts []*FactPointTo, drop []*Variable) []*FactPointTo {
	if len(drop) == 0 {
		return facts
	}
	if !FactsComplete(facts) || !VariablesComplete(drop) {
		SetError(ErrGeneric)
		return IncompleteFactSlice()
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

// GetMapStmEffect returns stored effect or empty for a live stm_id key.
// StmID ≤0 fails closed IncompleteEffect (no invent empty pure map default
// for incomplete statement keys used by SetAccumulatedEffect / body effect merge).
// Missing map entry for a live id is C++ map[] default empty complete.
func (fm *FactMgr) GetMapStmEffect(stmID int) Effect {
	if stmID <= 0 {
		return IncompleteEffect()
	}
	if fm == nil || fm.MapStmEffect == nil {
		return EmptyEffect()
	}
	if e, ok := fm.MapStmEffect[stmID]; ok {
		return e
	}
	return EmptyEffect()
}

// GetMapAccumEffect returns stored map_accum_effect or empty for a live stm_id.
// StmID ≤0 fails closed IncompleteEffect (no invent empty-complete zero Effect
// via map miss on incomplete keys — ReadVars/AddEffect would invent pure).
func (fm *FactMgr) GetMapAccumEffect(stmID int) Effect {
	if stmID <= 0 {
		return IncompleteEffect()
	}
	if fm == nil || fm.MapAccumEffect == nil {
		return EmptyEffect()
	}
	if e, ok := fm.MapAccumEffect[stmID]; ok {
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
// Incomplete IR returns IncompleteFactSlice / IncompleteUnionFactSlice (not bare
// nil — FactsComplete(nil)/UnionFactsComplete(nil) invent empty init success so
// AddNewVarFact soft-skips as “no fact to add”).
func AbstractFactForVarInit(v *Variable) (pt []*FactPointTo, un []*FactUnion) {
	if v == nil || v.Type == nil {
		// incomplete var IR (not “non-pointer complete empty”)
		return IncompleteFactSlice(), IncompleteUnionFactSlice()
	}
	if !v.IsPointer() && !v.Type.IsUnion() {
		// complete empty — not a point-to/union subject
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
		// incomplete union abstract is hole marker (not bare nil invent empty)
		if !UnionFactsComplete(un) {
			return nil, IncompleteUnionFactSlice()
		}
		return nil, un
	}
	// pointer (Fact.cpp:94–95)
	// Fact.cpp:94–95 — abstract_fact_for_assign; assert(lvar_cnt == 1)
	pt = AbstractFactForAssign(nil, v, 0, rhs)
	// incomplete / multi / zero — hole marker (no invent empty init for AddNewVarFact)
	if !FactsComplete(pt) || len(pt) != 1 {
		return IncompleteFactSlice(), nil
	}
	// Fact.cpp:97–109 — more init values on array of pointers
	// Fact.cpp:99 — assert(av) when isArray (AsArray set)
	if v.IsArray && v.AsArray == nil {
		return IncompleteFactSlice(), nil
	}
	if av := v.AsArray; av != nil {
		// Fact.cpp:100–106 — get_more_init_values() Expression* only
		// no invent Constant from InitValues to_string() list
		for _, e := range av.InitExprs {
			// Expression* always live in C++; nil hole is broken IR — fail closed
			if e == nil {
				return IncompleteFactSlice(), nil
			}
			more := AbstractFactForAssign(nil, v, 0, e)
			if !FactsComplete(more) {
				return IncompleteFactSlice(), nil
			}
			for _, f := range more {
				// Fact* always live; MergeFactInto incomplete = hole marker
				if f == nil {
					return IncompleteFactSlice(), nil
				}
				merged := MergeFactInto(pt, f)
				if !FactsComplete(merged) {
					return IncompleteFactSlice(), nil
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
				// incomplete FieldVars — clear partial aggregate makeup sticky
				// (no invent soft re-pick AddNewVarFact success past holes)
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			fm.AddNewVarFact(f)
			// child may have cleared on hole / merge fail
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
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
		// incomplete abstract must not invent skip (no fact to add) — sticky ERROR
		if !FactsComplete(pt) {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		// Fact.cpp:94–95 assert(lvar_cnt==1) — no soft invent NewFactPointTo when empty
		for _, f := range pt {
			if f == nil {
				// incomplete abstract list — clear partial GlobalFacts sticky
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			merged := MergeFactInto(fm.GlobalFacts, f)
			if !FactsComplete(merged) {
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			fm.GlobalFacts = merged
		}
	}
	if wantUn {
		// incomplete union abstract must not invent skip empty UnionFacts merge sticky
		// also wipe GlobalFacts so callers checking FactsComplete abort (not leave
		// sticky-only poison with complete GlobalFacts)
		if !UnionFactsComplete(un) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		for _, uf := range un {
			if uf == nil {
				fm.UnionFacts = IncompleteUnionFactSlice()
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			merged := MergeUnionFact(fm.UnionFacts, uf)
			if !UnionFactsComplete(merged) {
				fm.UnionFacts = IncompleteUnionFactSlice()
				fm.GlobalFacts = IncompleteFactSlice()
				SetError(ErrGeneric)
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
// Incomplete GlobalFacts / Clone fail closed sticky (IncompleteFactSlice on
// GlobalFacts + SetError; incomplete map entries stay incomplete local markers —
// no invent append past hole / soft re-pick past wiped GlobalFacts).
func (fm *FactMgr) AddNewVarFactAndUpdate(blk *Block, v *Variable) {
	if fm == nil || v == nil {
		return
	}
	// FactMgr.cpp:72 — assert(var->is_global()) when blk==nullptr
	// no soft invent facts for non-global "global create" path
	if blk == nil && !v.IsGlobal() {
		return
	}
	// incomplete subject map before add — fail closed sticky (no invent push onto holes)
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// snapshot length to detect newly merged facts
	beforePT := len(fm.GlobalFacts)
	fm.AddNewVarFact(v)
	// AddNewVarFact may wipe GlobalFacts incomplete — stop map push sticky
	if !FactsComplete(fm.GlobalFacts) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return
	}
	// FactMgr.cpp:77–104 — push each new init fact into maps
	// Fact* always live after add; nil / incomplete Clone fails closed sticky wipe
	// no invent MapFactsIn-only push when MapFactsOut is nil (one-sided invent)
	for i := beforePT; i < len(fm.GlobalFacts); i++ {
		f := fm.GlobalFacts[i]
		if f == nil {
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		cl := f.Clone()
		if cl == nil {
			// incomplete PointTo on new fact — fail closed sticky
			fm.GlobalFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		// map_facts_in: stm in_block(blk) || blk==null
		if fm.MapFactsIn != nil {
			for id := range fm.MapFactsIn {
				if blk != nil && !stmtIDInBlock(fm.Func, id, blk) {
					continue
				}
				// incomplete map slot — stay incomplete (no invent soft-append past hole)
				if !FactsComplete(fm.MapFactsIn[id]) {
					fm.MapFactsIn[id] = IncompleteFactSlice()
					continue
				}
				fm.MapFactsIn[id] = append(fm.MapFactsIn[id], cl)
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
				if !FactsComplete(fm.MapFactsOut[id]) {
					fm.MapFactsOut[id] = IncompleteFactSlice()
					continue
				}
				c2 := f.Clone()
				if c2 == nil {
					fm.GlobalFacts = IncompleteFactSlice()
					SetError(ErrGeneric)
					return
				}
				fm.MapFactsOut[id] = append(fm.MapFactsOut[id], c2)
			}
		} else {
			// FactMgr.cpp:99–100 — add_fact_out(stm, f) with visibility filters
			// Statement* always resolvable for ids under blk; unresolved id fails closed
			// IncompleteFactSlice on that out slot only (no invent skip as absent, and no
			// wipe GlobalFacts mid-generation which would poison ERROR_GUARD paths).
			for id := range fm.MapFactsOut {
				if !stmtIDInBlock(fm.Func, id, blk) {
					continue
				}
				st := FindStmtByID(fm.Func, id)
				if st == nil {
					fm.MapFactsOut[id] = IncompleteFactSlice()
					continue
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
// Incomplete lhs/facts/merge fails closed IncompleteVariables (not bare nil invent
// lvar_cnt==0, and not IncompleteVariables len==1 invent definitive renew without check).
func lhsAssignPointees(facts []*FactPointTo, lhs *Variable, lhsIndir int) []*Variable {
	if lhs == nil {
		return IncompleteVariables()
	}
	if !FactsComplete(facts) {
		return IncompleteVariables()
	}
	coll := lhs.GetCollective()
	if coll == nil {
		// incomplete field path collective — no invent level-0 merge of self
		return IncompleteVariables()
	}
	lvars := MergePointeesOfPointer(coll, lhsIndir, facts)
	if !VariablesComplete(lvars) {
		return IncompleteVariables()
	}
	if lhsIndir == 0 && lhs.Type != nil && lhs.Type.ptrTo != nil && len(lvars) == 0 {
		lvars = []*Variable{coll}
	}
	return lvars
}

// applyPointToAssignFacts applies point-to facts from abstract_fact_for_assign.
// FactMgr.cpp:376–388 — renew when definitive single non-array LHS; else merge.
// Returns (changed, ok). ok=false means incomplete map/merge — no invent apply success.
// Incomplete *facts is wiped to IncompleteFactSlice. Incomplete newFacts alone fails
// closed without wiping prior complete *facts (factory re-pick must not poison FM).
// empty complete newFacts is ok with changed=false.
func applyPointToAssignFacts(facts *[]*FactPointTo, lhs *Variable, lhsIndir int, newFacts []*FactPointTo) (changed bool, ok bool) {
	if facts == nil {
		return false, false
	}
	if !FactsComplete(*facts) {
		// incomplete subject map wiped — sticky (no invent soft re-pick past wiped FM)
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false, false
	}
	// incomplete abstract must not invent empty-apply success (len(nil)==0)
	// leave prior complete *facts for factory soft re-pick (no sticky)
	if !FactsComplete(newFacts) {
		return false, false
	}
	if len(newFacts) == 0 {
		return false, true
	}
	lvars := lhsAssignPointees(*facts, lhs, lhsIndir)
	// incomplete pointees must not invent lvar_cnt via len(IncompleteVariables)==1 renew
	// or len(nil)==0 merge-as-empty success
	if !VariablesComplete(lvars) {
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false, false
	}
	lvarCnt := len(lvars)
	// when AbstractFactForAssign used direct pointer path, lvarCnt matches transfer targets
	if lvarCnt == 0 && lhs != nil && lhsIndir == 0 && lhs.IsPointer() {
		lvarCnt = 1
	}
	if lvarCnt == 1 && newFacts[0] != nil && newFacts[0].Var != nil && !newFacts[0].Var.IsArray {
		// definitive assignment — renew (strong replace)
		_ = RenewFact(facts, newFacts[0])
		for j := 1; j < len(newFacts); j++ {
			merged := MergeFactInto(*facts, newFacts[j])
			if !FactsComplete(merged) {
				*facts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return false, false
			}
			*facts = merged
		}
		return true, true
	}
	for _, f := range newFacts {
		merged := MergeFactInto(*facts, f)
		if !FactsComplete(merged) {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
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
	// incomplete abstract must not invent empty apply success then union merge
	ptChanged, ptOK := applyPointToAssignFacts(&fm.GlobalFacts, lhs, lhsIndir, newFacts)
	if !ptOK {
		// if GlobalFacts was wiped (was already incomplete), also wipe union;
		// incomplete newFacts alone leaves complete GlobalFacts for factory re-pick
		if !FactsComplete(fm.GlobalFacts) {
			fm.UnionFacts = IncompleteUnionFactSlice()
		}
		// sticky already set when map wiped; ensure sticky if apply left HasError
		if !HasError() && !FactsComplete(fm.GlobalFacts) {
			SetError(ErrGeneric)
		}
		return false
	}
	if ptChanged {
		changed = true
	}
	// FactUnion::abstract_fact_for_assign (meta_facts loop)
	ufacts, _ := AbstractFactUnionForAssign(fm.UnionFacts, fm.GlobalFacts, lhs, lhsIndir, rhs)
	// incomplete abstract must not invent empty union merge success; leave prior
	// complete UnionFacts for factory re-pick (do not poison)
	if !UnionFactsComplete(ufacts) {
		return false
	}
	for _, uf := range ufacts {
		// FactUnion* always live from complete abstract — nil hole sticky wipe
		if uf == nil {
			fm.UnionFacts = IncompleteUnionFactSlice()
			SetError(ErrGeneric)
			return false
		}
		merged := MergeUnionFact(fm.UnionFacts, uf)
		if !UnionFactsComplete(merged) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
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

// MergeUnionFact replaces or appends a union fact by subject.
// FactUnion* always live; nil f or map hole fails closed sticky IncompleteUnionFactSlice
// (no invent empty-complete via UnionFactsComplete(nil) / soft re-pick past wipe).
func MergeUnionFact(facts []*FactUnion, f *FactUnion) []*FactUnion {
	if f == nil {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if !UnionFactsComplete(facts) {
		SetError(ErrGeneric)
		return IncompleteUnionFactSlice()
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
	// incomplete working/snapshot sets fail closed sticky before partial makeup
	if !FactsComplete(*oldFacts) || !FactsComplete(newFacts) {
		*oldFacts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return false
	}
	for _, f := range newFacts {
		// no invent soft-continue past nil fact holes (also covered by FactsComplete)
		if f == nil || f.Var == nil {
			*oldFacts = IncompleteFactSlice()
			SetError(ErrGeneric)
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
			// stop sticky — no invent continue loop and re-append later vars onto nil.
			if *oldFacts == nil || !FactsComplete(*oldFacts) {
				*oldFacts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
		}
	}
	return true
}

// AddNewVarFactInto mirrors FactMgr::add_new_var_fact(v, facts).
// FactMgr.cpp:118–131 — abstract_fact_for_var_init into the given fact slice.
// Variable* FieldVars always live; nil hole fails closed (*facts = IncompleteFactSlice() — no invent
// soft-skip hole and still makeup later fields as complete).
func AddNewVarFactInto(v *Variable, facts *[]*FactPointTo) {
	if facts == nil {
		return
	}
	// Variable* always live; nil v hole fails closed sticky (clear — no invent skip as absent)
	if v == nil {
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
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
				*facts = IncompleteFactSlice()
				SetError(ErrGeneric)
				return
			}
			AddNewVarFactInto(f, facts)
			// incomplete hole marker is non-nil; FactsComplete false
			if !FactsComplete(*facts) {
				*facts = IncompleteFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
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
	// incomplete abstract must not invent skip (no fact to add) sticky
	if !FactsComplete(pt) {
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	// Fact.cpp:94–95 assert(lvar_cnt==1) — no invent garbage shell when empty
	// Fact* always live from abstract; nil hole fails closed (*facts cleared) sticky
	for _, f := range pt {
		if f == nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		cl := f.Clone()
		if cl == nil {
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		if FindRelatedPointTo(*facts, f.Var) == nil {
			*facts = append(*facts, cl)
		}
	}
}

// FindDanglingGlobalPtrs mirrors FactMgr::find_dangling_global_ptrs.
// FactMgr.cpp:688–700 — non-const global pointers that are dead at function exit.
// Incomplete GlobalFacts fails closed IncompleteVariables DeadGlobals
// (not bare empty invent "no dangling" via VariablesComplete(nil)/len==0).
func (fm *FactMgr) FindDanglingGlobalPtrs(f *Function) {
	if fm == nil || f == nil {
		return
	}
	f.DeadGlobals = f.DeadGlobals[:0]
	if !FactsComplete(fm.GlobalFacts) {
		// incomplete map fails closed sticky (no invent empty DeadGlobals success)
		f.DeadGlobals = IncompleteVariables()
		SetError(ErrGeneric)
		return
	}
	for _, fact := range fm.GlobalFacts {
		// FactsComplete guarantees live fact.Var
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
// Incomplete assign fails closed sticky (false; no invent SetMapFactsOut from wiped map).
func (fm *FactMgr) UpdateFactForReturnStmt(st *Stmt, rv *Variable, expr *Expression) bool {
	// Expression* always live on StatementReturn; nil expr fails closed
	// (no invent garbage RHS transfer as stand-in for missing return value IR)
	if fm == nil || rv == nil || expr == nil {
		return false
	}
	// abstract_fact_for_return ≈ abstract_fact_for_assign(facts, Lhs(rv), expr)
	// FactMgr.cpp:408–416 — merge into inputs; fact_changed on merge
	changed := fm.UpdateFactForAssign(rv, 0, expr)
	// incomplete GlobalFacts after assign — sticky; do not invent return out map
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
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
			// incomplete Param list fails closed sticky — no invent skip remaining params
			*facts = IncompleteFactSlice()
			SetError(ErrGeneric)
			return
		}
		var arg *Expression
		if i < len(args) {
			arg = args[i]
		}
		// FactMgr.cpp:113–114 — always update_fact_for_assign (all params, not pointer-only)
		// false alone may mean no lattice change; incomplete clears *facts sticky
		_ = fm.UpdateFactForAssignInto(p, 0, arg, facts)
		if !FactsComplete(*facts) {
			*facts = IncompleteFactSlice()
			if !HasError() {
				SetError(ErrGeneric)
			}
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
		// only wipe union when *facts was incomplete (wiped); incomplete abstract alone
		// leaves prior complete map for factory re-pick
		if fm != nil && !FactsComplete(*facts) {
			fm.UnionFacts = IncompleteUnionFactSlice()
		}
		return false
	}
	if ptChanged {
		changed = true
	}
	if fm != nil {
		ufacts, _ := AbstractFactUnionForAssign(fm.UnionFacts, *facts, lhs, lhsIndir, rhs)
		// incomplete abstract: fail closed without poisoning prior complete UnionFacts
		if !UnionFactsComplete(ufacts) {
			return false
		}
		for _, uf := range ufacts {
			// FactUnion* always live from complete abstract — nil hole sticky wipe
			if uf == nil {
				fm.UnionFacts = IncompleteUnionFactSlice()
				SetError(ErrGeneric)
				return false
			}
			merged := MergeUnionFact(fm.UnionFacts, uf)
			if !UnionFactsComplete(merged) {
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !HasError() {
					SetError(ErrGeneric)
				}
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
	// incomplete inputs fail closed sticky before partition (no invent drop via hole skip)
	if !FactsComplete(*inputs) {
		*inputs = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	if !VariablesComplete(fm.Func.Param) {
		*inputs = IncompleteFactSlice()
		SetError(ErrGeneric)
		return
	}
	fm.AddParamFacts(args, inputs)
	if !FactsComplete(*inputs) {
		*inputs = IncompleteFactSlice()
		if !HasError() {
			SetError(ErrGeneric)
		}
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
		// incomplete map fails closed sticky (no invent clean filter past holes)
		*facts = IncompleteFactSlice()
		SetError(ErrGeneric)
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
