// Upstream: FactMgr.h / FactMgr.cpp (per-function DFA facts; light skeleton).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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
//
// currentSession().InUserInvocationRevisit is true during FunctionInvocationUser::revisit
// (nested body re-analysis). Used for IsValidPtr collective fallback on
// only on revisit so gen-time ExpressionVariable select stream is unchanged.

// AddInterestedFacts mirrors FactMgr::add_interested_facts.
// FactMgr.cpp:475–486 — register meta fact kinds for DFA.
func AddInterestedFacts(interests int) {
	AddInterestedFactsSess(nil, interests)
}

// AddInterestedFactsSess sets meta-fact interest flags on an explicit session bag.
func AddInterestedFactsSess(s *Session, interests int) {
	s = sessOrAmbient(s)
	s.MetaFactPointToEnabled = interests&FactCategoryPointTo != 0
	s.MetaFactUnionEnabled = interests&FactCategoryUnionWrite != 0
}

// MetaFactPointToEnabled reports whether point-to analysis is active.
func MetaFactPointToEnabled() bool { return MetaFactPointToEnabledSess(nil) }

// MetaFactPointToEnabledSess reports point-to meta flag on an explicit session bag.
func MetaFactPointToEnabledSess(s *Session) bool {
	return sessOrAmbient(s).MetaFactPointToEnabled
}

// MetaFactUnionEnabled reports whether union-write analysis is active.
func MetaFactUnionEnabled() bool { return MetaFactUnionEnabledSess(nil) }

// MetaFactUnionEnabledSess reports union meta flag on an explicit session bag.
func MetaFactUnionEnabledSess(s *Session) bool {
	return sessOrAmbient(s).MetaFactUnionEnabled
}

// ClearMetaFacts restores default interested facts (both on).
// Called from DoFinalization between generations.
func ClearMetaFacts() {
	ClearMetaFactsSess(nil)
}

// ClearMetaFactsSess restores default meta flags on an explicit session bag.
func ClearMetaFactsSess(s *Session) {
	s = sessOrAmbient(s)
	s.MetaFactPointToEnabled = true
	s.MetaFactUnionEnabled = true
}

// FactMgr mirrors FactMgr for a function — global_facts + stm maps.
// GlobalFacts holds FactPointTo; UnionFacts holds FactUnion.
type FactMgr struct {
	// Sess is the pure-run bag when set (generation); nil in minimal unit tests.
	Sess *Session
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
	// MapUnionFactsIn / MapUnionFactsOut are the eUnionWrite partition of
	// C++ map_facts_in/out FactVec (FactMgr.cpp set_fact_in/out store full FactVec).
	// Without these, post_loop_analysis / restore_facts only rewind point-to and leave
	// UnionFacts at post-body last-writes → IsNonreadableField over-filters choose_var.
	MapUnionFactsIn  map[int][]*FactUnion
	MapUnionFactsOut map[int][]*FactUnion
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
// fmSess is nil-safe Sess for *FactMgr methods.
func fmSess(fm *FactMgr) *Session {
	if fm == nil {
		return nil
	}
	return fm.Sess
}

func NewFactMgr(f *Function) *FactMgr {
	return NewFactMgrSess(nil, f)
}

// NewFactMgrSess constructs a FactMgr on an explicit session bag (nil → ambient).
func NewFactMgrSess(s *Session, f *Function) *FactMgr {
	return &FactMgr{
		// Prefer explicit s; else active run bag under Generate; unit tests → defaultSession.
		Sess:             sessOrAmbient(s),
		Func:             f,
		MapStmEffect:     make(map[int]Effect),
		MapFactsIn:       make(map[int][]*FactPointTo),
		MapFactsOut:      make(map[int][]*FactPointTo),
		MapUnionFactsIn:  make(map[int][]*FactUnion),
		MapUnionFactsOut: make(map[int][]*FactUnion),
		MapFactsInFinal:  make(map[int][]*FactPointTo),
		MapFactsOutFinal: make(map[int][]*FactPointTo),
		MapAccumEffect:   make(map[int]Effect),
		MapVisited:       make(map[int]bool),
	}
}

// factHasL233MayNull reports whether facts contain may-null for variable l_233.
func factHasL233MayNull(facts []*FactPointTo) bool {
	for _, f := range facts {
		if f != nil && f.Var != nil && f.Var.Name == "l_233" && f.IsNull() {
			return true
		}
	}
	return false
}

// SetGlobalFacts assigns fm.GlobalFacts. With CSMITH_DEBUG_FACTS=1, logs when
// l_233 may-null is dropped by a full slice replacement (seed-2 first_div 10107).
func (fm *FactMgr) SetGlobalFacts(facts []*FactPointTo, tag string) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if os.Getenv("CSMITH_DEBUG_FACTS") != "" && factHasL233MayNull(fm.GlobalFacts) && !factHasL233MayNull(facts) {
		depth := uint64(0)
		if r := sessRng(fmSess(fm)); r != nil {
			depth = r.RandDepth()
		}
		fn := "?"
		if fm.Func != nil {
			fn = fm.Func.Name
		}
		_, file, line, _ := runtime.Caller(1)
		fmt.Fprintf(os.Stderr, "WIPE tag=%s func=%s d=%d from=%s:%d\n",
			tag, fn, depth, filepath.Base(file), line)
		var pcs [8]uintptr
		n := runtime.Callers(2, pcs[:])
		frames := runtime.CallersFrames(pcs[:n])
		fmt.Fprintf(os.Stderr, "  stack:")
		for {
			fr, more := frames.Next()
			fmt.Fprintf(os.Stderr, " %s:%d", filepath.Base(fr.File), fr.Line)
			if !more {
				break
			}
		}
		fmt.Fprintln(os.Stderr)
	}
	fm.GlobalFacts = facts
}

// SetMapFactsIn records pre-statement facts (FactMgr::map_facts_in[s] = facts).
// FactMgr.cpp:249–251 — map_facts_in[s] = full FactVec (ePointTo + eUnionWrite).
// Pairs live fm.UnionFacts as the eUnionWrite partition (block entry / live lattice).
// For pre-make snapshots taken before generation mutated unions, use SetMapFactsInPair.
// FactMgr + live stm_id always required; sticky (no invent soft-skip store past hole).
func (fm *FactMgr) SetMapFactsIn(stmID int, facts []*FactPointTo) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.SetMapFactsInPair(stmID, facts, fm.UnionFacts)
}

// SetMapFactsInPair stores map_facts_in point-to + eUnionWrite partitions together.
// FactMgr.cpp set_fact_in — one FactVec assignment covers both categories.
func (fm *FactMgr) SetMapFactsInPair(stmID int, facts []*FactPointTo, unionFacts []*FactUnion) {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapFactsIn == nil {
		fm.MapFactsIn = make(map[int][]*FactPointTo)
	}
	if fm.MapUnionFactsIn == nil {
		fm.MapUnionFactsIn = make(map[int][]*FactUnion)
	}
	fm.MapFactsIn[stmID] = storeFactMapEntry(facts)
	fm.MapUnionFactsIn[stmID] = storeUnionFactMapEntry(unionFacts)
}

// SetMapFactsOut records post-statement facts.
// FactMgr.cpp set_fact_out — full FactVec; pairs live fm.UnionFacts.
// FactMgr + live stm_id always required; sticky (no invent soft-skip store past hole).
// Prefer SetMapFactsOutForBlock for Block* so parent==nullptr filtering applies.
func (fm *FactMgr) SetMapFactsOut(stmID int, facts []*FactPointTo) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.SetMapFactsOutPair(stmID, facts, fm.UnionFacts)
}

// SetMapFactsOutForBlock mirrors FactMgr::set_fact_out when the Statement is a Block.
// FactMgr.cpp:257–274 + Block.cpp:693 / 561 — after OOS locals / remove_rv, store out.
// FactMgr.cpp:268–270 — eReturn || s->parent == nullptr → remove_function_local_facts
// (drop params/stack subjects + mark_func_end garbage on remaining pointees).
// Nested blocks (parent != nil) store facts as-is (OOS already applied by caller).
// Function-body map_facts_out is ret_facts source (FunctionInvocationUser.cpp:212–221);
// skipping remove_function_local_facts left param pointees live after callee return.
func (fm *FactMgr) SetMapFactsOutForBlock(b *Block, facts []*FactPointTo) {
	if fm == nil || b == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if StmIDUnset(b.StmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !FactsComplete(facts) {
		fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	cp := facts
	// FactMgr.cpp:268 — s->parent == nullptr (function body Block)
	if b.Parent == nil {
		// stm is the body; is_var_on_stack uses stm->parent (== nil) so only params
		// match as stack subjects; mark_func_end still marks param pointees garbage.
		cp = RemoveFunctionLocalFactsAtSess(fmSess(fm), facts, b.Func, b.Parent)
		if !FactsComplete(cp) {
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
	}
	// FactMgr.cpp:141–156 — OOS erase is category-agnostic (ePointTo + eUnionWrite).
	// Soft invent paired post-OOS point-to map_out with live UnionFacts that still
	// held body-local union subjects → same_facts size skew / extra re-visits.
	// Clone + OOS locals into map_union_out; do not mutate live UnionFacts here
	// (Block post_creation keeps pre-OOS live during find_fixed_point).
	if !UnionFactsComplete(fm.UnionFacts) {
		fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	outU := CloneUnionFactSliceDeep(fm.UnionFacts)
	if !UnionFactsComplete(outU) {
		fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	if len(b.LocalVars) > 0 {
		UpdateUnionFactsForOOSVarsSess(fmSess(fm), b.LocalVars, &outU)
		if !UnionFactsComplete(outU) {
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
	}
	if b.Parent == nil && b.Func != nil {
		outU = RemoveFunctionLocalUnionFactsAtSess(fmSess(fm), outU, b.Func, b.Parent)
		if !UnionFactsComplete(outU) {
			fm.SetMapFactsOut(b.StmID, IncompleteFactSlice())
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
	}
	fm.SetMapFactsOutPair(b.StmID, cp, outU)
}

// SetMapFactsOutPair stores map_facts_out point-to + eUnionWrite partitions together.
func (fm *FactMgr) SetMapFactsOutPair(stmID int, facts []*FactPointTo, unionFacts []*FactUnion) {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	if fm.MapUnionFactsOut == nil {
		fm.MapUnionFactsOut = make(map[int][]*FactUnion)
	}
	fm.MapFactsOut[stmID] = storeFactMapEntry(facts)
	fm.MapUnionFactsOut[stmID] = storeUnionFactMapEntry(unionFacts)
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

// storeUnionFactMapEntry mirrors storeFactMapEntry for FactUnion maps.
// Incomplete → IncompleteUnionFactSlice; complete empty → non-nil {}.
//
// Deep-clone FactUnion objects (not shallow slice copy). C++ FactVec stores
// Fact* that are replaced on renew/join (new heap Fact); soft invent shallow
// CloneUnionFactSlice left map_facts_in/out sharing live *FactUnion so a later
// Join/SetBottom on the live lattice rewrote historical arm outs (seed-123:
// combine_branch_facts then_fid=0 else_fid=1 bottomed g_721 while sibling
// unions kept init f0 → choose_var ok pool 36 vs UP 37).
func storeUnionFactMapEntry(facts []*FactUnion) []*FactUnion {
	if !UnionFactsComplete(facts) {
		return IncompleteUnionFactSlice()
	}
	cl := CloneUnionFactSliceDeep(facts)
	if sessHasError(nil) {
		return IncompleteUnionFactSlice()
	}
	if cl == nil {
		return []*FactUnion{}
	}
	return cl
}

// GetMapUnionFactsIn returns the eUnionWrite partition of map_facts_in[stm].
// Missing live key → complete empty {}. Incomplete stored slots stay markers.
// FactMgr + live stm_id always required; sticky IncompleteUnionFactSlice past hole.
func (fm *FactMgr) GetMapUnionFactsIn(stmID int) []*FactUnion {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if fm.MapUnionFactsIn == nil {
		return []*FactUnion{}
	}
	if facts, ok := fm.MapUnionFactsIn[stmID]; ok {
		if !UnionFactsComplete(facts) {
			return IncompleteUnionFactSlice()
		}
		return facts
	}
	return []*FactUnion{}
}

// GetMapUnionFactsOut returns the eUnionWrite partition of map_facts_out[stm].
func (fm *FactMgr) GetMapUnionFactsOut(stmID int) []*FactUnion {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if fm.MapUnionFactsOut == nil {
		return []*FactUnion{}
	}
	if facts, ok := fm.MapUnionFactsOut[stmID]; ok {
		if !UnionFactsComplete(facts) {
			return IncompleteUnionFactSlice()
		}
		return facts
	}
	return []*FactUnion{}
}

// AssignGlobalFactsFromMapIn assigns global_facts from map_facts_in[stm] for both
// categories. FactMgr.cpp / StatementFor.cpp:355 —
//
//	fm->global_facts = fm->map_facts_in[&body];
//
// Full FactVec replace (point-to + eUnionWrite). Incomplete either partition wipes both.
func (fm *FactMgr) AssignGlobalFactsFromMapIn(stmID int) {
	if fm == nil || StmIDUnset(stmID) {
		if fm != nil {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
		}
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	pt := fm.GetMapFactsIn(stmID)
	un := fm.GetMapUnionFactsIn(stmID)
	if !FactsComplete(pt) || !UnionFactsComplete(un) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.SetGlobalFacts(CloneFactSlice(pt), "auto_map_in_assign")
	// Deep install: live lattice must not alias map_facts_in FactUnion objects
	// (renew/join replace or mutate; maps must retain historical arm/entry lattice).
	cl := CloneUnionFactSliceDeep(un)
	if sessHasError(fmSess(fm)) || !UnionFactsComplete(cl) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	if cl == nil {
		fm.UnionFacts = []*FactUnion{}
	} else {
		fm.UnionFacts = cl
	}
}

// AssignGlobalFactsFromMapOut assigns global_facts from map_facts_out[stm] for both
// categories. FactMgr.cpp / Block.cpp:729 —
//
//	fm->global_facts = fm->map_facts_out[this];
//
// Full FactVec replace (point-to + eUnionWrite). Incomplete either partition wipes both.
func (fm *FactMgr) AssignGlobalFactsFromMapOut(stmID int) {
	if fm == nil || StmIDUnset(stmID) {
		if fm != nil {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
		}
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	pt := fm.GetMapFactsOut(stmID)
	un := fm.GetMapUnionFactsOut(stmID)
	if !FactsComplete(pt) || !UnionFactsComplete(un) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.SetGlobalFacts(CloneFactSlice(pt), "auto_map_out_assign")
	cl := CloneUnionFactSliceDeep(un)
	if sessHasError(fmSess(fm)) || !UnionFactsComplete(cl) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	if cl == nil {
		fm.UnionFacts = []*FactUnion{}
	} else {
		fm.UnionFacts = cl
	}
}

// GetMapFactsIn returns map_facts_in for a live stm_id.
// FactMgr always live; sticky IncompleteFactSlice (no invent empty-complete past hole).
// StmID ≤0 fails closed sticky IncompleteFactSlice (no invent MapFactsIn[0] miss as
// empty-complete merge/visit / soft re-pick past incomplete keys).
// Missing live key → complete empty {}. Incomplete stored slots stay markers
// (non-sticky local map holes for soft re-pick factories).
func (fm *FactMgr) GetMapFactsIn(stmID int) []*FactPointTo {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if fm.MapFactsIn == nil {
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
// FactMgr always live; sticky IncompleteFactSlice (no invent empty-complete past hole).
// StmID ≤0 fails closed sticky IncompleteFactSlice (no invent MapFactsOut[0]
// empty-complete / soft re-pick past incomplete keys).
// Missing live key → complete empty {}. Incomplete stored slots stay markers.
func (fm *FactMgr) GetMapFactsOut(stmID int) []*FactPointTo {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if fm.MapFactsOut == nil {
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
// FactMgr always live; sticky IncompleteFactSlice (no invent empty-complete past hole).
func (fm *FactMgr) GetMapFactsInFinal(stmID int) []*FactPointTo {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if fm.MapFactsInFinal == nil {
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
// FactMgr always live; sticky IncompleteFactSlice (no invent empty-complete past hole).
func (fm *FactMgr) GetMapFactsOutFinal(stmID int) []*FactPointTo {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	if fm.MapFactsOutFinal == nil {
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
		if destParent == nil && !StmIDUnset(st.GotoDestStmID) && fm != nil && fm.Func != nil {
			destParent = FindParentBlockOfStmID(fm.Func, st.GotoDestStmID)
		}
	}
	fm.SetMapFactsOutForStmtDest(st, facts, blk, destParent)
}

// SetMapFactsOutForStmtDest is set_fact_out with optional goto dest parent override.
// FactMgr + Statement always live; sticky (no invent soft-skip set_fact_out past hole).
func (fm *FactMgr) SetMapFactsOutForStmtDest(st *Stmt, facts []*FactPointTo, blk, destParent *Block) {
	if fm == nil || st == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// Statement::stm_id always live; StmID 0 fails closed sticky (no invent silent
	// set_fact_out success without map entry / soft re-pick past missing out)
	if StmIDUnset(st.StmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// incomplete source facts fail closed sticky — hole marker (not SetMapFactsOut(nil)
	// which storeFactMapEntry would invent as complete empty)
	if !FactsComplete(facts) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		if fm.MapUnionFactsOut == nil {
			fm.MapUnionFactsOut = make(map[int][]*FactUnion)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	cp := CloneFactSlice(facts)
	// FactMgr.cpp:257–274 — set_fact_out filters full FactVec (ePointTo + eUnionWrite).
	// Soft invent was PT-only RemoveLoopLocalFacts / RemoveFunctionLocalFacts then
	// SetMapFactsOut pairing unfiltered live UnionFacts → continue/break map_out kept
	// parent-block eUnionWrite subjects (seed-30: continue 379 map_out still had l_810,
	// back-edge into for body 333 polluted current_inputs → same_facts failed on for
	// g_37 → VisitFactsStatementFor overwrote make_iteration IV read in feffect).
	if !UnionFactsComplete(fm.UnionFacts) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		if fm.MapUnionFactsOut == nil {
			fm.MapUnionFactsOut = make(map[int][]*FactUnion)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	outU := CloneUnionFactSliceDeep(fm.UnionFacts)
	if sessHasError(fmSess(fm)) || !UnionFactsComplete(outU) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		if fm.MapUnionFactsOut == nil {
			fm.MapUnionFactsOut = make(map[int][]*FactUnion)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	if outU == nil {
		outU = []*FactUnion{}
	}
	switch st.Kind {
	case StmtContinue, StmtBreak:
		// FactMgr.cpp:257–262 — remove_loop_local_facts(s, facts_copy) full FactVec
		cp = RemoveLoopLocalFactsForStmt(cp, st, blk)
		outU = RemoveLoopLocalUnionFactsForStmt(outU, st, blk)
	case StmtReturn:
		// FactMgr.cpp:268–270 — remove_function_local_facts(facts_copy, s)
		// stack check uses s->parent (blk); no invent f.Body-only walk
		cp = RemoveFunctionLocalFactsAtSess(fmSess(fm), cp, fm.Func, blk)
		outU = RemoveFunctionLocalUnionFactsAtSess(fmSess(fm), outU, fm.Func, blk)
	case StmtGoto:
		// FactMgr.cpp:263–266 — update_facts_for_dest(facts, facts_copy, sg->dest)
		// Full FactVec: ePointTo + eUnionWrite both OOS-drop subjects not visible
		// at dest (FactMgr.cpp:450–482). Soft invent filtered only PT then stored
		// raw live outU → map_facts_out[goto] kept then-arm local eUnionWrite
		// (e.g. l_1372 last=0) after jump to sibling else where local is OOS.
		// choose_visible_read_var(map_facts_out[goto], …) then treated the field
		// as readable and inflated ok pool (seed 10613516242873274820:
		// nOk 36 vs UP 35; if (l_1156) vs if (l_670.f0)).
		dp := destParent
		if dp == nil {
			dp = st.GotoDestParent
		}
		if dp == nil && !StmIDUnset(st.GotoDestStmID) && fm.Func != nil {
			dp = FindParentBlockOfStmID(fm.Func, st.GotoDestStmID)
		}
		// FactMgr.cpp:427–428 assert(func); no soft invent RemoveFunctionLocalFacts
		// when dest unknown (wrong filter vs update_facts_for_dest).
		// IncompleteFactSlice sticky — bare nil + SetMapFactsOut invents complete empty.
		if fm.Func == nil {
			cp = IncompleteFactSlice()
			outU = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
		} else {
			out := []*FactPointTo{}
			UpdateFactsForDestSess(fmSess(fm), cp, &out, fm.Func, dp)
			cp = out
			// FactMgr.cpp:450–482 — same OOS filter on eUnionWrite partition
			var outUnions []*FactUnion
			UpdateUnionFactsForDestSess(fmSess(fm), outU, &outUnions, fm.Func, dp)
			outU = outUnions
		}
	default:
		// FactMgr.cpp:268 — eReturn || s->parent == nullptr → remove function locals
		if blk == nil {
			cp = RemoveFunctionLocalFactsAtSess(fmSess(fm), cp, fm.Func, nil)
			outU = RemoveFunctionLocalUnionFactsAtSess(fmSess(fm), outU, fm.Func, nil)
		}
	}
	// incomplete after filter/dest — store hole sticky (no invent complete out map)
	if !FactsComplete(cp) || !UnionFactsComplete(outU) {
		if fm.MapFactsOut == nil {
			fm.MapFactsOut = make(map[int][]*FactPointTo)
		}
		if fm.MapUnionFactsOut == nil {
			fm.MapUnionFactsOut = make(map[int][]*FactUnion)
		}
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	// Full FactVec out: filtered ePointTo + filtered eUnionWrite (not raw live unions).
	fm.SetMapFactsOutPair(st.StmID, cp, outU)
}

// FindParentBlockOfStmID walks function blocks for the parent of stm_id.
// Used when StatementGoto::dest parent is not stored on Stmt.
// Block* always live on Function.Blocks; nil hole fails closed (nil — no invent skip).
// Nested walk uses get_blocks only; incomplete if-arm sticky whole miss
// (no invent soft-continue past incomplete arm then miss a stmt in complete Then,
// or invent soft-skip missing arm then find under sibling of same if).
func FindParentBlockOfStmID(f *Function, stmID int) *Block {
	// Function + live StmID always required; sticky no invent parent miss soft-success
	if f == nil || StmIDUnset(stmID) {
		sessNoteError(nil, ErrGeneric)
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
			for _, nb := range blks {
				if nb == nil {
					// incomplete get_blocks arm sticky fail whole search
					// (no invent soft-continue past hole / miss Then when Else nil)
					sessNoteError(nil, ErrGeneric)
					return nil
				}
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
		// Block* always live on Function.Blocks; nil hole sticky miss (no invent soft-success)
		if b == nil {
			sessNoteError(nil, ErrGeneric)
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
// Function + live StmID always required; sticky nil (no invent miss soft-success past hole).
func FindStmtByID(f *Function, stmID int) *Stmt {
	if f == nil || StmIDUnset(stmID) {
		sessNoteError(nil, ErrGeneric)
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
// FactMgr + Statement + Fact always live; sticky (no invent soft-skip append past hole).
func (fm *FactMgr) AddFactOut(st *Stmt, stParent *Block, fact *FactPointTo) {
	if fm == nil || st == nil || fact == nil || fact.Var == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// StmID 0 fails closed sticky (no invent silent add_fact_out without map entry)
	if StmIDUnset(st.StmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// ensure map exists before fail-closed writes
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	// incomplete subject fact — sticky hole marker (not soft-skip or invent cleaned clone)
	if !FactsComplete([]*FactPointTo{fact}) {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// already incomplete out map — stay incomplete sticky (no invent append onto hole)
	if prev, ok := fm.MapFactsOut[st.StmID]; ok && !FactsComplete(prev) {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	f := fm.Func
	// visibility needs complete stack for non-globals
	if f != nil && !fact.Var.IsGlobal() {
		// residual ERROR sticky — no invent soft-skip AddFactOut past IsGlobal hole
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
		if !f.StackScanComplete(stParent) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			// residual ERROR sticky — no invent soft-skip AddFactOut past StackScan residual
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if !f.IsVarVisible(fact.Var, stParent) {
			// residual ERROR sticky — no invent soft-skip not-visible past hard IR hole
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			}
			return
		}
		// residual ERROR sticky — no invent append past IsVarVisible hole
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
	} else if f != nil {
		// residual from IsGlobal true path
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
		if !f.IsVarVisible(fact.Var, stParent) {
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
	}
	switch st.Kind {
	case StmtReturn:
		if !fact.Var.IsGlobal() {
			// residual ERROR sticky — no invent soft-drop return non-global past IsGlobal hole
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
	case StmtBreak, StmtContinue:
		// find enclosing loop block
		b := stParent
		for b != nil && !b.Looping {
			b = b.Parent
		}
		if f != nil && !fact.Var.IsGlobal() {
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				return
			}
			if !f.StackScanComplete(b) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
		} else if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
		if f != nil && !f.IsVarVisible(fact.Var, b) {
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			return
		}
	case StmtGoto:
		// FactMgr.cpp:296–300 — drop if var not visible at StatementGoto::dest.
		// Prefer GotoDestParent; else resolve parent of GotoDestStmID via function blocks.
		destParent := st.GotoDestParent
		if destParent == nil && !StmIDUnset(st.GotoDestStmID) && f != nil {
			destParent = FindParentBlockOfStmID(f, st.GotoDestStmID)
			// residual ERROR sticky — no invent soft-skip dest parent miss past hole
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				return
			}
		}
		if destParent != nil && f != nil {
			if !fact.Var.IsGlobal() && !f.StackScanComplete(destParent) {
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				return
			}
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				return
			}
			if !f.IsVarVisible(fact.Var, destParent) {
				if sessHasError(fmSess(fm)) {
					fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				}
				return
			}
			if sessHasError(fmSess(fm)) {
				fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
				return
			}
		}
	}
	cl := fact.Clone()
	// residual ERROR sticky — no invent soft-append past Clone residual
	if sessHasError(fmSess(fm)) {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		return
	}
	if cl == nil {
		fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.MapFactsOut[st.StmID] = append(fm.MapFactsOut[st.StmID], cl)
}

// AddFactOutUnion is the eUnionWrite half of FactMgr::add_fact_out.
// FactMgr.cpp:281–308 — same visibility filters as AddFactOut (return /
// break+continue loop head / goto dest). Soft invent of AddNewVarFactAndUpdate
// union-out path used only IsVarVisible(parent) → re-appended loop-nested
// union subjects onto continue/break map_out after remove_loop_local (seed
// 2020240685: continue 39 map_out kept l_237 → body 26 map_in pollution →
// post_loop break invent BOTTOM → VisitFacts nonreadable).
func (fm *FactMgr) AddFactOutUnion(st *Stmt, stParent *Block, fact *FactUnion) {
	if fm == nil || st == nil || fact == nil || fact.Var == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if StmIDUnset(st.StmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapUnionFactsOut == nil {
		fm.MapUnionFactsOut = make(map[int][]*FactUnion)
	}
	if !UnionFactsComplete([]*FactUnion{fact}) {
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if prev, ok := fm.MapUnionFactsOut[st.StmID]; ok && !UnionFactsComplete(prev) {
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	f := fm.Func
	if f != nil && !fact.Var.IsGlobal() {
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
		if !f.StackScanComplete(stParent) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		if !f.IsVarVisible(fact.Var, stParent) {
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
	} else if f != nil {
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
		if !f.IsVarVisible(fact.Var, stParent) {
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
	}
	switch st.Kind {
	case StmtReturn:
		if !fact.Var.IsGlobal() {
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
	case StmtBreak, StmtContinue:
		// FactMgr.cpp:288–296 — visible at enclosing looping block
		b := stParent
		for b != nil && !b.Looping {
			b = b.Parent
		}
		if f != nil && !fact.Var.IsGlobal() {
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				return
			}
			if !f.StackScanComplete(b) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
		} else if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
		if f != nil && !f.IsVarVisible(fact.Var, b) {
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			}
			return
		}
		if sessHasError(fmSess(fm)) {
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
			return
		}
	case StmtGoto:
		destParent := st.GotoDestParent
		if destParent == nil && !StmIDUnset(st.GotoDestStmID) && f != nil {
			destParent = FindParentBlockOfStmID(f, st.GotoDestStmID)
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				return
			}
		}
		if destParent != nil && f != nil {
			if !fact.Var.IsGlobal() && !f.StackScanComplete(destParent) {
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				return
			}
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				return
			}
			if !f.IsVarVisible(fact.Var, destParent) {
				if sessHasError(fmSess(fm)) {
					fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				}
				return
			}
			if sessHasError(fmSess(fm)) {
				fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
				return
			}
		}
	}
	cl := fact.Clone()
	if sessHasError(fmSess(fm)) {
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		return
	}
	if cl == nil {
		fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.MapUnionFactsOut[st.StmID] = append(fm.MapUnionFactsOut[st.StmID], cl)
}

// UpdateFactsForDest mirrors FactMgr::update_facts_for_dest.
// FactMgr.cpp:424–456 — merge facts; OOS locals at dest become garbage/dropped.
// Incomplete inputs fail closed sticky via IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-complete dest facts / soft re-pick past wipe).
// factsOut always live; sticky (no invent soft-skip dest update past hole).
func UpdateFactsForDest(factsIn []*FactPointTo, factsOut *[]*FactPointTo, f *Function, destParent *Block) {
	UpdateFactsForDestSess(nil, factsIn, factsOut, f, destParent)
}

func UpdateFactsForDestSess(s *Session, factsIn []*FactPointTo, factsOut *[]*FactPointTo, f *Function, destParent *Block) {
	if factsOut == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// FactMgr.cpp:427–428 — dest->func; assert(func)
	// no soft invent dest facts without function (OOS walk needs f)
	if f == nil {
		*factsOut = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	// Fact* always live; nil hole fails closed sticky (no invent skip partial dest update)
	for _, fact := range factsIn {
		if fact == nil || fact.Var == nil {
			*factsOut = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
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
			// residual ERROR sticky — no invent soft-continue OOS scan past hard IR hole
			if sessHasError(s) {
				*factsOut = IncompleteFactSlice()
				return
			}
			addOOS(fact.Var)
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent not-OOS soft-skip past hard IR hole
			*factsOut = IncompleteFactSlice()
			return
		}
		for _, p := range fact.PointTo {
			// Variable* always live in PointTo; nil hole fails closed whole dest update sticky
			// (no invent soft-skip hole and still OOS-scan later pointees)
			if p == nil {
				*factsOut = IncompleteFactSlice()
				sessNoteError(s, ErrGeneric)
				return
			}
			if !IsSpecialPtr(p) && f.IsVarOOS(p, destParent) {
				if sessHasError(s) {
					*factsOut = IncompleteFactSlice()
					return
				}
				addOOS(p)
			} else if sessHasError(s) {
				*factsOut = IncompleteFactSlice()
				return
			}
		}
		merged := MergeFactIntoSess(s, *factsOut, fact)
		if !FactsComplete(merged) {
			*factsOut = IncompleteFactSlice()
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return
		}
		*factsOut = merged
	}
	UpdateFactsForOOSVarsSess(s, oosVars, factsOut)
	// residual ERROR sticky — no invent complete dest facts past OOS update hole
	if sessHasError(s) {
		*factsOut = IncompleteFactSlice()
	}
}

// UpdateUnionFactsForDest is the eUnionWrite half of FactMgr::update_facts_for_dest.
// FactMgr.cpp:450–482 — merge non-rv facts into facts_out; drop OOS subjects.
// Used by StatementGoto forward path (full FactVec with UpdateFactsForDest).
// Incomplete inputs fail closed sticky IncompleteUnionFactSlice.}

func UpdateUnionFactsForDest(factsIn []*FactUnion, factsOut *[]*FactUnion, f *Function, destParent *Block) {
	UpdateUnionFactsForDestSess(nil, factsIn, factsOut, f, destParent)
}

func UpdateUnionFactsForDestSess(s *Session, factsIn []*FactUnion, factsOut *[]*FactUnion, f *Function, destParent *Block) {
	if factsOut == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if f == nil {
		*factsOut = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	if !UnionFactsComplete(factsIn) {
		*factsOut = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	if *factsOut == nil {
		*factsOut = []*FactUnion{}
	}
	var oosVars []*Variable
	seen := map[*Variable]bool{}
	addOOS := func(v *Variable) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		oosVars = append(oosVars, v)
	}
	for _, fact := range factsIn {
		if fact == nil || fact.Var == nil {
			*factsOut = IncompleteUnionFactSlice()
			sessNoteError(s, ErrGeneric)
			return
		}
		if isReturnVar(fact.Var) {
			continue
		}
		if f.IsVarOOS(fact.Var, destParent) {
			if sessHasError(s) {
				*factsOut = IncompleteUnionFactSlice()
				return
			}
			addOOS(fact.Var)
		} else if sessHasError(s) {
			*factsOut = IncompleteUnionFactSlice()
			return
		}
		// FactMgr.cpp:479 — merge_fact(facts_out, f) for every non-rv subject
		merged := MergeUnionFactSess(s, *factsOut, fact)
		if !UnionFactsComplete(merged) {
			*factsOut = IncompleteUnionFactSlice()
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return
		}
		*factsOut = merged
	}
	UpdateUnionFactsForOOSVarsSess(s, oosVars, factsOut)
	if sessHasError(s) {
		*factsOut = IncompleteUnionFactSlice()
	}
}

// ClearMapVisited mirrors FactMgr::clear_map_visited.
// FactMgr.cpp:510–514 — set all visited flags false (keep keys).
// FactMgr always live; sticky (no invent soft-skip clear past hole).
// Nil MapVisited is complete no-op (no keys to clear).}

func (fm *FactMgr) ClearMapVisited() {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapVisited == nil {
		return
	}
	for k := range fm.MapVisited {
		fm.MapVisited[k] = false
	}
}

// RestoreFacts mirrors FactMgr::restore_facts for the point-to partition only.
// Prefer RestoreFactsPair when the pre-snapshot also captured eUnionWrite (full FactVec).
// FactMgr.cpp:489–492 — makeup new vars into old, then replace global_facts.
// FactMgr always live; sticky (no invent soft-skip restore past hole).
// Incomplete oldFacts / makeup fail closed sticky (no invent clean clone + partial
// makeup, no soft re-pick past wiped GlobalFacts).
func (fm *FactMgr) RestoreFacts(oldFacts []*FactPointTo) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// Point-to-only restore keeps current UnionFacts (legacy tests). Generation
	// paths that snapshot full FactVec use RestoreFactsPair.
	fm.restoreFactsPT(oldFacts)
}

// RestoreFactsPair mirrors FactMgr::restore_facts on the full FactVec.
// FactMgr.cpp:489–492 — makeup_new_var_facts then global_facts = old_facts
// (ePointTo + eUnionWrite together).
func (fm *FactMgr) RestoreFactsPair(oldPT []*FactPointTo, oldUnion []*FactUnion) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// incomplete either partition — wipe both sticky
	if (oldPT != nil && !FactsComplete(oldPT)) || !UnionFactsComplete(oldUnion) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.restoreFactsPT(oldPT)
	if sessHasError(fmSess(fm)) || !FactsComplete(fm.GlobalFacts) {
		fm.UnionFacts = IncompleteUnionFactSlice()
		return
	}
	// FactMgr.cpp:489–492 — after makeup on full FactVec, assign. Union partition:
	// shallow Fact* vector copy of the pre-snapshot (CloneUnionFactSlice).
	// Makeup for newly created unions is already applied via AddNewVarFact on live
	// path; pre-snapshot may miss later unions — merge via MakeupNewVarFacts is
	// PT-centric; re-apply union side by keeping oldUnion subjects and letting
	// subsequent AddNewVarFact fill gaps (same as C++ makeup_new_var_facts walking
	// all categories in new_facts).
	// For vars created after old_facts, C++ add_new_var_fact adds init abstracts
	// into old_facts for both PT and Union. Go MakeupNewVarFacts only walks PT
	// in `work`; AddNewVarFact on FM updates both GlobalFacts and UnionFacts live.
	// After PT restore, re-makeup unions from live → into oldUnion then assign.
	workU := append([]*FactUnion(nil), oldUnion...)
	if workU == nil {
		workU = []*FactUnion{}
	}
	// Pull union subjects present in live but missing from old (created mid-body).
	if !makeupNewUnionFactsSess(fmSess(fm), &workU, fm.UnionFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	fm.UnionFacts = workU
}

// restoreFactsPT is the point-to half of restore_facts (FactMgr.cpp:489–492).
func (fm *FactMgr) restoreFactsPT(oldFacts []*FactPointTo) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// nil oldFacts is empty restore; non-nil with holes → fail closed sticky
	if oldFacts != nil && !FactsComplete(oldFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// FactMgr.cpp:489–492 — makeup_new_var_facts(old_facts, global_facts);
	// global_facts = old_facts. Snapshot is a Fact* vector (shallow); do not deep-clone
	// FactPointTo objects (CloneFactSlice would freeze pre-merge lattice and diverge
	// from C++ when Fact objects are still shared with the live env).
	// FactMgr.cpp:489–492 only — makeup then assign. No invent re-join of live
	// may-null into the restored snapshot (SPEC: no invent may-null reinject).
	work := append([]*FactPointTo(nil), oldFacts...)
	if !MakeupNewVarFactsSess(fmSess(fm), &work, fm.GlobalFacts) {
		// incomplete GlobalFacts or mid-makeup hole — fail closed sticky
		fm.GlobalFacts = IncompleteFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	fm.SetGlobalFacts(work, "auto_fact_mgr_restore")
}

// makeupNewUnionFacts mirrors makeup_new_var_facts for the eUnionWrite partition.
// FactMgr.cpp:494–508 — for each new_facts entry missing in old, add_new_var_fact.
// Here we only re-add init FactUnion for union subjects present in live but not old.
func makeupNewUnionFacts(oldFacts *[]*FactUnion, live []*FactUnion) bool {
	return makeupNewUnionFactsSess(nil, oldFacts, live)
}

func makeupNewUnionFactsSess(s *Session, oldFacts *[]*FactUnion, live []*FactUnion) bool {
	if oldFacts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !UnionFactsComplete(*oldFacts) || !UnionFactsComplete(live) {
		*oldFacts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, f := range live {
		if f == nil || f.Var == nil {
			*oldFacts = IncompleteUnionFactSlice()
			sessNoteError(s, ErrGeneric)
			return false
		}
		if FindRelatedUnion(*oldFacts, f.Var) != nil {
			if sessHasError(s) {
				*oldFacts = IncompleteUnionFactSlice()
				return false
			}
			continue
		}
		if sessHasError(s) {
			*oldFacts = IncompleteUnionFactSlice()
			return false
		}
		// FactMgr.cpp:503–504 — add_new_var_fact(v, old_facts) for missing subjects
		v := f.Var
		if !v.IsGlobal() && !v.IsLocal() {
			if sessHasError(s) {
				*oldFacts = IncompleteUnionFactSlice()
				return false
			}
			continue
		}
		if sessHasError(s) {
			*oldFacts = IncompleteUnionFactSlice()
			return false
		}
		_, unInit := AbstractFactForVarInitSess(s, v)
		if !UnionFactsComplete(unInit) {
			*oldFacts = IncompleteUnionFactSlice()
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return false
		}
		for _, init := range unInit {
			if init == nil {
				*oldFacts = IncompleteUnionFactSlice()
				sessNoteError(s, ErrGeneric)
				return false
			}
			merged := MergeUnionFactSess(s, *oldFacts, init)
			if !UnionFactsComplete(merged) {
				*oldFacts = IncompleteUnionFactSlice()
				sessNoteError(s, ErrGeneric)
				return false
			}
			*oldFacts = merged
		}
	}
	return true
}

// SetupInOutMaps mirrors FactMgr::setup_in_out_maps.
// FactMgr.cpp:208–246 — first_time clones into final; else combine.
// Fact* always live; incomplete source maps fail closed sticky (final hole marker —
// no invent cleaned partial clone / soft re-pick past incomplete finals).
// FactMgr always live; sticky (no invent soft-skip setup past hole).}

func (fm *FactMgr) SetupInOutMaps(firstTime bool) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapFactsInFinal == nil {
		fm.MapFactsInFinal = make(map[int][]*FactPointTo)
	}
	if fm.MapFactsOutFinal == nil {
		fm.MapFactsOutFinal = make(map[int][]*FactPointTo)
	}
	// fail closed whole setup on residual: wipe finals (no invent partial complete clones
	// under random map iteration order past a hard IR hole on another id)
	failClosedWipe := func(badIn, badOut int) {
		fm.MapFactsInFinal = make(map[int][]*FactPointTo)
		fm.MapFactsOutFinal = make(map[int][]*FactPointTo)
		if badIn != 0 {
			fm.MapFactsInFinal[badIn] = IncompleteFactSlice()
		}
		if badOut != 0 {
			fm.MapFactsOutFinal[badOut] = IncompleteFactSlice()
		}
		sessNoteError(fmSess(fm), ErrGeneric)
	}
	if firstTime {
		for id, facts := range fm.MapFactsIn {
			// storeFactMapEntry: incomplete → hole marker sticky whole setup
			if !FactsComplete(facts) {
				failClosedWipe(id, 0)
				return
			}
			fm.MapFactsInFinal[id] = storeFactMapEntry(facts)
		}
		for id, facts := range fm.MapFactsOut {
			if !FactsComplete(facts) {
				failClosedWipe(0, id)
				return
			}
			fm.MapFactsOutFinal[id] = storeFactMapEntry(facts)
		}
		return
	}
	// combine current maps into final
	// Fact* always live; incomplete maps or failed merge fail closed sticky
	// (no invent partial join or bare-nil complete empty / soft-continue other ids)
	for id, facts2 := range fm.MapFactsIn {
		facts1 := fm.MapFactsInFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			failClosedWipe(id, 0)
			return
		}
		// MergeFacts clears *facts sticky on incomplete mid-join
		_ = MergeFactsSess(fmSess(fm), &facts1, facts2)
		if !FactsComplete(facts1) {
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			failClosedWipe(id, 0)
			return
		}
		fm.MapFactsInFinal[id] = storeFactMapEntry(facts1)
	}
	for id, facts2 := range fm.MapFactsOut {
		facts1 := fm.MapFactsOutFinal[id]
		if !FactsComplete(facts1) || !FactsComplete(facts2) {
			failClosedWipe(0, id)
			return
		}
		_ = MergeFactsSess(fmSess(fm), &facts1, facts2)
		if !FactsComplete(facts1) {
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			failClosedWipe(0, id)
			return
		}
		fm.MapFactsOutFinal[id] = storeFactMapEntry(facts1)
	}
}

// BackupStmFactMaps mirrors FactMgr::backup_stm_fact_maps for a statement tree.
// FactMgr.cpp:516–531 — copy in/out maps for stm and nested blocks.
// C++ map_facts_in/out are full FactVec (ePointTo + eUnionWrite). Soft invent was
// PT-only backup so restore left MapUnionFacts* stale → IsNonreadableField drift
// after goto revisit / fixed-point (seed-7 eligible pool half-size).
// Incomplete source maps store hole markers (no invent cleaned partial clones).
// Incomplete get_blocks tree (nil if-arm) fails closed sticky: root maps backed as
// IncompleteFactSlice (no invent root-only complete backup / soft re-pick past hole).
// FactMgr + Statement + dest maps always live; sticky (no invent soft-skip backup past hole).
func (fm *FactMgr) BackupStmFactMaps(
	st *Stmt,
	factsIn, factsOut map[int][]*FactPointTo,
	unionIn, unionOut map[int][]*FactUnion,
) {
	if fm == nil || st == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if factsIn == nil || factsOut == nil || unionIn == nil || unionOut == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
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
		if !StmIDUnset(st.StmID) {
			factsIn[st.StmID] = IncompleteFactSlice()
			factsOut[st.StmID] = IncompleteFactSlice()
			unionIn[st.StmID] = IncompleteUnionFactSlice()
			unionOut[st.StmID] = IncompleteUnionFactSlice()
		}
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	for _, b := range blks {
		fm.backupBlockFactMaps(b, factsIn, factsOut, unionIn, unionOut)
	}
	if !StmIDUnset(st.StmID) {
		if in, ok := fm.MapFactsIn[st.StmID]; ok {
			factsIn[st.StmID] = storeFactMapEntry(in)
		}
		if out, ok := fm.MapFactsOut[st.StmID]; ok {
			factsOut[st.StmID] = storeFactMapEntry(out)
		}
		if in, ok := fm.MapUnionFactsIn[st.StmID]; ok {
			unionIn[st.StmID] = storeUnionFactMapEntry(in)
		}
		if out, ok := fm.MapUnionFactsOut[st.StmID]; ok {
			unionOut[st.StmID] = storeUnionFactMapEntry(out)
		}
	}
}

// backupBlockFactMaps walks one block tree for BackupStmFactMaps.
// Block always live after parent incomplete check; sticky (no invent soft-skip backup past hole).
func (fm *FactMgr) backupBlockFactMaps(
	b *Block,
	factsIn, factsOut map[int][]*FactPointTo,
	unionIn, unionOut map[int][]*FactUnion,
) {
	if b == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !StmIDUnset(b.StmID) {
		if in, ok := fm.MapFactsIn[b.StmID]; ok {
			factsIn[b.StmID] = storeFactMapEntry(in)
		}
		if out, ok := fm.MapFactsOut[b.StmID]; ok {
			factsOut[b.StmID] = storeFactMapEntry(out)
		}
		if in, ok := fm.MapUnionFactsIn[b.StmID]; ok {
			unionIn[b.StmID] = storeUnionFactMapEntry(in)
		}
		if out, ok := fm.MapUnionFactsOut[b.StmID]; ok {
			unionOut[b.StmID] = storeUnionFactMapEntry(out)
		}
	}
	for i := range b.Stmts {
		fm.BackupStmFactMaps(&b.Stmts[i], factsIn, factsOut, unionIn, unionOut)
	}
}

// RestoreStmFactMaps mirrors FactMgr::restore_stm_fact_maps.
// FactMgr.cpp:533–548 — full FactVec (ePointTo + eUnionWrite) partitions.
// Incomplete backup entries restore as hole markers (storeFactMapEntry).
// Incomplete get_blocks tree fails closed sticky: root maps set IncompleteFactSlice
// (no invent soft-skip nil arm then restore root/sibling as complete tree).
// FactMgr + Statement always live; sticky (no invent soft-skip restore past hole).
func (fm *FactMgr) RestoreStmFactMaps(
	st *Stmt,
	factsIn, factsOut map[int][]*FactPointTo,
	unionIn, unionOut map[int][]*FactUnion,
) {
	if fm == nil || st == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if factsIn == nil || factsOut == nil || unionIn == nil || unionOut == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapFactsIn == nil {
		fm.MapFactsIn = make(map[int][]*FactPointTo)
	}
	if fm.MapFactsOut == nil {
		fm.MapFactsOut = make(map[int][]*FactPointTo)
	}
	if fm.MapUnionFactsIn == nil {
		fm.MapUnionFactsIn = make(map[int][]*FactUnion)
	}
	if fm.MapUnionFactsOut == nil {
		fm.MapUnionFactsOut = make(map[int][]*FactUnion)
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
		if !StmIDUnset(st.StmID) {
			fm.MapFactsIn[st.StmID] = IncompleteFactSlice()
			fm.MapFactsOut[st.StmID] = IncompleteFactSlice()
			fm.MapUnionFactsIn[st.StmID] = IncompleteUnionFactSlice()
			fm.MapUnionFactsOut[st.StmID] = IncompleteUnionFactSlice()
		}
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	for _, b := range blks {
		fm.restoreBlockFactMaps(b, factsIn, factsOut, unionIn, unionOut)
	}
	if !StmIDUnset(st.StmID) {
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
		if in, ok := unionIn[st.StmID]; ok {
			fm.MapUnionFactsIn[st.StmID] = storeUnionFactMapEntry(in)
		} else {
			delete(fm.MapUnionFactsIn, st.StmID)
		}
		if out, ok := unionOut[st.StmID]; ok {
			fm.MapUnionFactsOut[st.StmID] = storeUnionFactMapEntry(out)
		} else {
			delete(fm.MapUnionFactsOut, st.StmID)
		}
	}
}

// restoreBlockFactMaps walks one block tree for RestoreStmFactMaps.
// Block always live after parent incomplete check; sticky (no invent soft-skip restore past hole).
func (fm *FactMgr) restoreBlockFactMaps(
	b *Block,
	factsIn, factsOut map[int][]*FactPointTo,
	unionIn, unionOut map[int][]*FactUnion,
) {
	if b == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !StmIDUnset(b.StmID) {
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
		if in, ok := unionIn[b.StmID]; ok {
			fm.MapUnionFactsIn[b.StmID] = storeUnionFactMapEntry(in)
		} else {
			delete(fm.MapUnionFactsIn, b.StmID)
		}
		if out, ok := unionOut[b.StmID]; ok {
			fm.MapUnionFactsOut[b.StmID] = storeUnionFactMapEntry(out)
		} else {
			delete(fm.MapUnionFactsOut, b.StmID)
		}
	}
	for i := range b.Stmts {
		fm.RestoreStmFactMaps(&b.Stmts[i], factsIn, factsOut, unionIn, unionOut)
	}
}

// FindUpdatedFacts mirrors FactMgr::find_updated_facts.
// FactMgr.cpp:652–665 — facts_out that differ from related facts_in.
// Incomplete FactMgr/StmID sticky IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-update success / soft re-pick past holes).
// Incomplete in/out maps fail closed sticky IncompleteFactSlice.
func (fm *FactMgr) FindUpdatedFacts(stmID int) []*FactPointTo {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	in := fm.GetMapFactsIn(stmID)
	out := fm.GetMapFactsOut(stmID)
	if !FactsComplete(in) || !FactsComplete(out) {
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	var updated []*FactPointTo
	for _, f := range out {
		// Fact* always live after FactsComplete
		if f == nil || f.Var == nil {
			sessNoteError(fmSess(fm), ErrGeneric)
			return IncompleteFactSlice()
		}
		// FactMgr.cpp:659–662 — assert(prev_f); only changed when prev exists
		// no soft invent "new out-only fact" as updated
		prev := FindRelatedPointTo(in, f.Var)
		// residual ERROR sticky — no invent soft-continue then partial updated past hole
		if sessHasError(fmSess(fm)) {
			return IncompleteFactSlice()
		}
		if prev == nil {
			continue
		}
		if !f.Equal(prev) {
			// residual ERROR sticky — no invent soft-continue past Equal hole
			if sessHasError(fmSess(fm)) {
				return IncompleteFactSlice()
			}
			updated = append(updated, f)
		} else if sessHasError(fmSess(fm)) {
			return IncompleteFactSlice()
		}
	}
	return updated
}

// FindUpdatedFinalFacts mirrors FactMgr::find_updated_final_facts.
// FactMgr.cpp:667–686 — final maps; always include rv facts.
// Incomplete FactMgr/StmID sticky IncompleteFactSlice (not bare nil —
// FactsComplete(nil)==true invents empty-update success / soft re-pick past holes).
// Incomplete in/out maps fail closed sticky IncompleteFactSlice.
func (fm *FactMgr) FindUpdatedFinalFacts(stmID int) []*FactPointTo {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteFactSlice()
	}
	in := fm.GetMapFactsInFinal(stmID)
	out := fm.GetMapFactsOutFinal(stmID)
	if !FactsComplete(in) || !FactsComplete(out) {
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	var updated []*FactPointTo
	for _, f := range out {
		if f == nil || f.Var == nil {
			sessNoteError(fmSess(fm), ErrGeneric)
			return IncompleteFactSlice()
		}
		// FactMgr.cpp:676–677 — rv facts always listed (no pre-fact required)
		if fm.Func != nil && fm.Func.RV != nil && fm.Func.RV.Match(f.Var) {
			// residual ERROR sticky — no invent soft-continue past Match hole
			if sessHasError(fmSess(fm)) {
				return IncompleteFactSlice()
			}
			updated = append(updated, f)
			continue
		}
		// residual ERROR sticky from Match false path
		if sessHasError(fmSess(fm)) {
			return IncompleteFactSlice()
		}
		// FactMgr.cpp:679–682 — assert(prev_f); no soft invent missing prev as change
		prev := FindRelatedPointTo(in, f.Var)
		// residual ERROR sticky — no invent soft-continue then partial updated past hole
		if sessHasError(fmSess(fm)) {
			return IncompleteFactSlice()
		}
		if prev == nil {
			continue
		}
		if !f.Equal(prev) {
			// residual ERROR sticky — no invent soft-continue past Equal hole
			if sessHasError(fmSess(fm)) {
				return IncompleteFactSlice()
			}
			updated = append(updated, f)
		} else if sessHasError(fmSess(fm)) {
			return IncompleteFactSlice()
		}
	}
	return updated
}

// RemoveLoopLocalFacts mirrors FactMgr::remove_loop_local_facts for a block.
// FactMgr.cpp:601–612 — collect locals from blk up through enclosing loop,
// then update_facts_for_oos_vars (drop subjects + mark pointees garbage).
// Block always live for break/continue OOS walk; nil blk sticky IncompleteFactSlice
// (no invent complete keep-all-facts soft-success past missing parent / loop chain).
// Incomplete facts/locals/clone fail closed sticky (no invent cleaned OOS filter
// / soft re-pick past wiped break-continue out maps).
func RemoveLoopLocalFacts(facts []*FactPointTo, blk *Block) []*FactPointTo {
	return RemoveLoopLocalFactsSess(nil, facts, blk)
}

func RemoveLoopLocalFactsSess(s *Session, facts []*FactPointTo, blk *Block) []*FactPointTo {
	// Block* always live for loop-local OOS; sticky no invent passthrough keep locals
	if blk == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	// incomplete facts fail closed sticky before OOS
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	locals := collectLoopLocalVars(blk)
	// incomplete LocalVars hole — fail closed sticky
	if !VariablesComplete(locals) {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	out := CloneFactSlice(facts)
	// incomplete clone is hole marker sticky (not bare nil invent empty complete)
	if !FactsComplete(out) {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	// Statement.cpp set_fact_out / FactMgr.cpp:607–611
	UpdateFactsForOOSVarsSess(s, locals, &out)
	if !FactsComplete(out) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
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
	return RemoveLoopLocalFactsSess(nil, facts, b)
}

// RemoveLoopLocalUnionFacts is the eUnionWrite half of remove_loop_local_facts.
// FactMgr.cpp:629–639 — update_facts_for_oos_vars is category-agnostic (FactVec).
// Soft invent left map_union_out[continue/break] with parent-block union subjects
// that are OOS at the loop head (seed-30 l_810 via continue back-edge into for body).
func RemoveLoopLocalUnionFacts(facts []*FactUnion, blk *Block) []*FactUnion {
	return RemoveLoopLocalUnionFactsSess(nil, facts, blk)
}

func RemoveLoopLocalUnionFactsSess(s *Session, facts []*FactUnion, blk *Block) []*FactUnion {
	if blk == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	locals := collectLoopLocalVars(blk)
	if !VariablesComplete(locals) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	out := CloneUnionFactSliceDeep(facts)
	if !UnionFactsComplete(out) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return IncompleteUnionFactSlice()
	}
	if len(locals) > 0 {
		UpdateUnionFactsForOOSVarsSess(s, locals, &out)
		if !UnionFactsComplete(out) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteUnionFactSlice()
		}
	}
	if out == nil {
		return []*FactUnion{}
	}
	return out
}

// RemoveLoopLocalUnionFactsForStmt is remove_loop_local_facts eUnionWrite for Statement*.
// FactMgr.cpp:603–605 — block stmt uses itself; else use parent.
func RemoveLoopLocalUnionFactsForStmt(facts []*FactUnion, st *Stmt, parent *Block) []*FactUnion {
	b := parent
	if st != nil && st.Kind == StmtBlock && st.Then != nil {
		b = st.Then
	}
	return RemoveLoopLocalUnionFactsSess(nil, facts, b)
}

// collectLoopLocalVars walks blk → parents until a looping block (inclusive).
// FactMgr.cpp:605–610.
// Variable* always live on LocalVars; nil hole fails closed sticky IncompleteVariables
// (not bare nil invent empty-complete loop-local set / soft re-pick past hole).
// Empty complete walk returns non-nil empty slice.
func collectLoopLocalVars(blk *Block) []*Variable {
	locals := make([]*Variable, 0)
	b := blk
	for b != nil {
		if !VariablesComplete(b.LocalVars) {
			sessNoteError(nil, ErrGeneric)
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
// stParent is Statement::parent for is_var_on_stack (nil when s is the function body
// Block with parent==nullptr). Params are still on-stack when stParent is nil
// (Function.cpp:187–190); do not invent soft-skip of param subjects in that case.
// Fact* always live; incomplete PointTo/fact holes or incomplete Param/LocalVars
// stack lists fail closed sticky (no invent keep stack locals when IsVarOnStack
// returns false past a hole, or soft re-pick past wiped return out maps).
func RemoveFunctionLocalFactsAt(facts []*FactPointTo, f *Function, stParent *Block) []*FactPointTo {
	return RemoveFunctionLocalFactsAtSess(nil, facts, f, stParent)
}

func RemoveFunctionLocalFactsAtSess(s *Session, facts []*FactPointTo, f *Function, stParent *Block) []*FactPointTo {
	if !FactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice()
	}
	// StackScanComplete(nil) still validates Param completeness
	if f != nil && !f.StackScanComplete(stParent) {
		// residual ERROR sticky — no invent soft-clean facts past StackScan residual
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	out := make([]*FactPointTo, 0, len(facts))
	for _, fact := range facts {
		// Fact* always live after FactsComplete
		if fact == nil || fact.Var == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		// FactMgr.cpp:191–195 — is_var_on_stack OR other-function RV
		// stParent may be nil (function body set_fact_out); IsVarOnStack still
		// matches params (Function.cpp:187–190).
		if f != nil && f.IsVarOnStack(fact.Var, stParent) {
			// residual ERROR sticky — no invent soft-skip stack fact past hard IR hole
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue keep fact past IsVarOnStack hole
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		if fact.Var.IsRV() && (f == nil || f.RV == nil || !f.RV.Match(fact.Var)) {
			// residual ERROR sticky — no invent soft-skip RV past Match hole
			if sessHasError(s) {
				return IncompleteFactSlice()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		cl := fact.Clone()
		// residual ERROR sticky — no invent soft-keep past Clone residual
		if sessHasError(s) {
			return IncompleteFactSlice()
		}
		if cl == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteFactSlice()
		}
		out = append(out, cl)
	}
	// FactMgr.cpp:196–204 — remaining facts may point to stack locals → garbage
	MarkFuncEndOnFactsSess(s, &out, f, stParent)
	// MarkFuncEndOnFacts clears *facts on incomplete after mark sticky
	if !FactsComplete(out) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return IncompleteFactSlice()
	}
	return out
}

// RemoveFunctionLocalUnionFactsAt is the eUnionWrite half of
// FactMgr::remove_function_local_facts (FactMgr.cpp:179–195 subject erase).
// Category-agnostic erase by is_var_on_stack / other-function RV; no mark_func_end
// (ePointTo only). Incomplete maps fail closed IncompleteUnionFactSlice.}

func RemoveFunctionLocalUnionFactsAt(facts []*FactUnion, f *Function, stParent *Block) []*FactUnion {
	return RemoveFunctionLocalUnionFactsAtSess(nil, facts, f, stParent)
}

func RemoveFunctionLocalUnionFactsAtSess(s *Session, facts []*FactUnion, f *Function, stParent *Block) []*FactUnion {
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if f != nil && !f.StackScanComplete(stParent) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return IncompleteUnionFactSlice()
	}
	out := make([]*FactUnion, 0, len(facts))
	for _, fact := range facts {
		if fact == nil || fact.Var == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if f != nil && f.IsVarOnStack(fact.Var, stParent) {
			if sessHasError(s) {
				return IncompleteUnionFactSlice()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		if fact.Var.IsRV() && (f == nil || f.RV == nil || !f.RV.Match(fact.Var)) {
			if sessHasError(s) {
				return IncompleteUnionFactSlice()
			}
			continue
		}
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		cl := fact.Clone()
		if cl == nil || sessHasError(s) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteUnionFactSlice()
		}
		out = append(out, cl)
	}
	return out
}

// filterFactsNotInVars drops subjects in drop. Fact* always live;
// incomplete maps/pointees or incomplete drop list fail closed sticky
// (no invent keep subjects that match only after a drop-list hole).}

func filterFactsNotInVars(facts []*FactPointTo, drop []*Variable) []*FactPointTo {
	if len(drop) == 0 {
		return facts
	}
	if !FactsComplete(facts) || !VariablesComplete(drop) {
		sessNoteError(nil, ErrGeneric)
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
// SetMapStmEffect records per-statement effect.
// FactMgr + live stm_id always required; sticky (no invent soft-skip store past hole).
// SetMapStmEffect records map_stm_effect[stm].
// C++ Effect assignment deep-copies read/write vectors (Effect.cpp:84–89).
// Go struct copy shares maps: store a detached snapshot so later EffectStm /
// EffectAccum COW growth cannot alias-corrupt the map entry (and so
// GetMapStmEffect + AddEffect cannot share live maps with map_stm_effect —
// seed-7 binary RHS ambient half-size choose_var ok pool vs upstream).
func (fm *FactMgr) SetMapStmEffect(stmID int, eff Effect) {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	if eff.incomplete {
		fm.MapStmEffect[stmID] = IncompleteEffect()
		return
	}
	// Detach without Clone residual path: visit_facts may already be sticky.
	fm.MapStmEffect[stmID] = eff.detachMaps()
}

// GetMapStmEffect returns a deep copy of stored map_stm_effect or empty for a live stm_id.
// FactMgr always live; sticky IncompleteEffect (no invent empty pure past hole).
// StmID ≤0 fails closed sticky IncompleteEffect (no invent empty pure map default
// / soft re-pick past incomplete statement keys for SetAccumulatedEffect merge).
// Missing map entry for a live id is C++ map[] default empty complete.
// Detach on read so callers that AddEffect into EffectAccum cannot share maps
// with the stored snapshot (mirrors GetMapAccumEffect).
func (fm *FactMgr) GetMapStmEffect(stmID int) Effect {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	if fm.MapStmEffect == nil {
		return EmptyEffect()
	}
	if e, ok := fm.MapStmEffect[stmID]; ok {
		if e.incomplete {
			return IncompleteEffect()
		}
		return e.detachMaps()
	}
	return EmptyEffect()
}

// SetMapAccumEffect stores map_accum_effect[stm] as a deep copy of acc.
// Statement.cpp:563 / 622 — Effect is value-copied into the map (Effect.cpp:84–89
// deep-copies read/write vectors). Go Effect struct copy shares maps; without
// Clone, later EffectAccum growth/reset can alias-corrupt the snapshot used by
// StatementGoto choose_visible_read_var (map_accum_effect[other].get_read_vars).
// Incomplete acc stores IncompleteEffect (no invent empty pure past hole).
// FactMgr + live stm_id always required; sticky no invent soft-skip store past hole.
//
// Pre-existing HasError (e.g. visit_facts already failed) must not block store:
// stm_visit_facts always records map_accum_effect even when visit returns false.
func (fm *FactMgr) SetMapAccumEffect(stmID int, acc Effect) {
	if fm == nil || StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if fm.MapAccumEffect == nil {
		fm.MapAccumEffect = make(map[int]Effect)
	}
	if acc.incomplete {
		fm.MapAccumEffect[stmID] = IncompleteEffect()
		return
	}
	// Nil map keys (broken IR) — sticky incomplete marker; do not invent pure empty
	if !effectMapKeysComplete(acc.read) || !effectMapKeysComplete(acc.written) ||
		!effectMapKeysComplete(acc.lhsWrite) {
		sessNoteError(fmSess(fm), ErrGeneric)
		fm.MapAccumEffect[stmID] = IncompleteEffect()
		return
	}
	// Deep-copy without requiring global HasError clear (visit_facts may already sticky).
	fm.MapAccumEffect[stmID] = acc.detachMaps()
}

// GetMapAccumEffect returns a deep copy of stored map_accum_effect or empty for a live stm_id.
// FactMgr always live; sticky IncompleteEffect (no invent empty pure past hole).
// StmID ≤0 fails closed sticky IncompleteEffect (no invent empty-complete zero Effect
// via map miss on incomplete keys — ReadVars/AddEffect would invent pure).
// Returned Effect is detached from the map (C++ returns const Effect& but callers
// only read; Go deep-copy prevents accidental shared-map mutation of the snapshot).
func (fm *FactMgr) GetMapAccumEffect(stmID int) Effect {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	if StmIDUnset(stmID) {
		sessNoteError(fmSess(fm), ErrGeneric)
		return IncompleteEffect()
	}
	if fm.MapAccumEffect == nil {
		return EmptyEffect()
	}
	if e, ok := fm.MapAccumEffect[stmID]; ok {
		if e.incomplete {
			return IncompleteEffect()
		}
		// Detach without Clone() residual path: callers may read map_accum while
		// HasError is already sticky from an unrelated visit failure.
		return e.detachMaps()
	}
	return EmptyEffect()
}

// FactMgrMap is Function::FMList session map (func → FactMgr).
type FactMgrMap struct {
	// Sess is the pure-run bag when set (ProgramGenerator); nil in minimal unit tests.
	Sess   *Session
	byFunc map[*Function]*FactMgr
}

// NewFactMgrMap creates an empty FMList.
func NewFactMgrMap() *FactMgrMap {
	return NewFactMgrMapSess(nil)
}

// NewFactMgrMapSess creates an FMList bound to an explicit session bag.
func NewFactMgrMapSess(s *Session) *FactMgrMap {
	return &FactMgrMap{Sess: s, byFunc: make(map[*Function]*FactMgr)}
}

// ForFunc returns the FactMgr for f (session FMList).
// Prefer the FactMgr paired at make_random_signature / make_first (Function.cpp:422);
// only create when registering a function that has no paired entry yet.
// get_fact_mgr_for_func itself only looks up — create happens at signature time.
// FactMgrMap + Function always live; sticky nil (no invent miss soft-skip past hole).
func (m *FactMgrMap) ForFunc(f *Function) *FactMgr {
	if m == nil || f == nil {
		sessNoteError(nil, ErrGeneric)
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
		if m.Sess != nil {
			fm.Sess = m.Sess
		}
		return fm
	}
	// reuse paired FactMgr from signature create (no invent second manager)
	if f.factMgr != nil {
		if m.Sess != nil {
			f.factMgr.Sess = m.Sess
		}
		m.byFunc[f] = f.factMgr
		return f.factMgr
	}
	fm := NewFactMgrSess(m.Sess, f)
	f.factMgr = fm
	m.byFunc[f] = fm
	return fm
}

// AbstractFactForVarInit mirrors Fact::abstract_fact_for_var_init.
// Fact.cpp:85–112 — pointer/union only; assign from init; array alt inits merge.
// Hard IR holes (nil var/Type, array without AsArray, nil InitExprs) fail closed
// sticky IncompleteFactSlice / IncompleteUnionFactSlice so soft re-pick cannot
// invent empty init success past broken IR. Incomplete abstract transfer results
// remain non-sticky hole markers (AddNewVarFact sticks after abstract).
func AbstractFactForVarInit(v *Variable) (pt []*FactPointTo, un []*FactUnion) {
	return AbstractFactForVarInitSess(nil, v)
}

func AbstractFactForVarInitSess(s *Session, v *Variable) (pt []*FactPointTo, un []*FactUnion) {
	if v == nil || v.Type == nil {
		// incomplete var IR sticky (not “non-pointer complete empty”)
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice(), IncompleteUnionFactSlice()
	}
	if !v.IsPointer() && !v.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-skip empty-init past IsPointer residual false
		if sessHasError(s) {
			return IncompleteFactSlice(), IncompleteUnionFactSlice()
		}
		// complete empty — not a point-to/union subject
		return nil, nil
	}
	// residual ERROR sticky — no invent soft-continue pointer/union path past IsPointer residual true
	if sessHasError(s) {
		return IncompleteFactSlice(), IncompleteUnionFactSlice()
	}
	// Fact.cpp:96–109 — primary Variable::init, then ArrayVariable more_init_values.
	// CreateArrayVariable stores alts only in InitExprs; createAndInitialize sets
	// InitExpr primary. Soft invent used nil primary + first AbstractFactForAssignSess(s, nil)
	// → GarbagePtr then merge alts still IsDead (seed-10054 IsValidPtr on local
	// pointer arrays during nested revisit). When InitExpr is unset, use InitExprs[0]
	// as primary and remaining as more (no invent garbage-first past empty primary).
	var rhs *Expression
	var moreStart int
	if v.InitExpr != nil {
		rhs = v.InitExpr
	} else if v.Init != nil {
		rhs = &Expression{Term: TermConstant, Con: v.Init, ExprType: v.Type}
	} else if v.AsArray != nil && len(v.AsArray.InitExprs) > 0 {
		rhs = v.AsArray.InitExprs[0]
		moreStart = 1
	}
	if v.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-continue union abstract past IsUnion residual
		if sessHasError(s) {
			return IncompleteFactSlice(), IncompleteUnionFactSlice()
		}
		un, _ = AbstractFactUnionForAssignSess(s, nil, nil, v, 0, nil, rhs)
		// residual ERROR sticky — no invent soft-empty init past AbstractFactUnion residual
		if sessHasError(s) {
			return IncompleteFactSlice(), IncompleteUnionFactSlice()
		}
		// incomplete union abstract is hole marker (not bare nil invent empty)
		// non-sticky for soft re-pick factories (AddParamFacts); AddNewVarFact sticks after
		if !UnionFactsComplete(un) {
			return nil, IncompleteUnionFactSlice()
		}
		return nil, un
	}
	// residual ERROR sticky — no invent soft-continue pointer path past IsUnion residual false
	if sessHasError(s) {
		return IncompleteFactSlice(), IncompleteUnionFactSlice()
	}
	// pointer (Fact.cpp:94–95)
	// Fact.cpp:94–95 — abstract_fact_for_assign; assert(lvar_cnt == 1)
	pt, _ = AbstractFactForAssignSess(s, nil, v, 0, rhs)
	// residual ERROR sticky — no invent soft-empty init past AbstractFact residual
	if sessHasError(s) {
		return IncompleteFactSlice(), IncompleteUnionFactSlice()
	}
	// incomplete / multi / zero — hole marker (no invent empty init for AddNewVarFact)
	// non-sticky IncompleteFactSlice for soft re-pick factories
	if !FactsComplete(pt) || len(pt) != 1 {
		return IncompleteFactSlice(), nil
	}
	// Fact.cpp:97–109 — more init values on array of pointers
	// Fact.cpp:99 — assert(av) when isArray (AsArray set)
	if v.IsArray && v.AsArray == nil {
		// hard IR: isArray without AsArray sticky (no invent skip more-inits)
		sessNoteError(s, ErrGeneric)
		return IncompleteFactSlice(), nil
	}
	if av := v.AsArray; av != nil {
		// Fact.cpp:100–106 — get_more_init_values() Expression* only
		// no invent Constant from InitValues to_string() list
		// moreStart skips InitExprs[0] when it was promoted to primary above
		for _, e := range av.InitExprs[moreStart:] {
			// Expression* always live in C++; nil hole is broken IR — sticky
			if e == nil {
				sessNoteError(s, ErrGeneric)
				return IncompleteFactSlice(), nil
			}
			more, _ := AbstractFactForAssignSess(s, nil, v, 0, e)
			// live Expression* alt path: incomplete abstract sticky
			// (no invent soft-skip incomplete init alt / soft re-pick past hole)
			if !FactsComplete(more) {
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return IncompleteFactSlice(), nil
			}
			for _, f := range more {
				// Fact* always live; nil hole sticky (no invent skip merge hole)
				if f == nil {
					sessNoteError(s, ErrGeneric)
					return IncompleteFactSlice(), nil
				}
				merged := MergeFactIntoSess(s, pt, f)
				// incomplete merge after live alt sticky (no invent partial init facts)
				if !FactsComplete(merged) {
					if !sessHasError(s) {
						sessNoteError(s, ErrGeneric)
					}
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
// Type* always live for non-special subjects; Type-nil sticky clear (no invent
// empty-FieldVars aggregate complete / field walk past Type-nil shell).
// Fact* from abstract always live; nil hole fails closed (GlobalFacts cleared).
// FactMgr + Variable always live; sticky (no invent soft-skip makeup past hole).}

func (fm *FactMgr) AddNewVarFact(v *Variable) {
	if fm == nil || v == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// Type* always live for non-special subjects; specials Type-nil by design
	// (not makeup subjects — complete skip, no invent field walk past shell)
	if v.Type == nil {
		if !IsSpecialPtr(v) {
			fm.GlobalFacts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	// recurse into aggregate fields (pointer members)
	if !v.IsPointer() && !v.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-continue field recurse past IsPointer residual false
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		for _, f := range v.FieldVars {
			if f == nil {
				// incomplete FieldVars — clear partial aggregate makeup sticky
				// (no invent soft re-pick AddNewVarFact success past holes)
				fm.GlobalFacts = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			fm.AddNewVarFact(f)
			// child may have cleared on hole / merge fail
			if !FactsComplete(fm.GlobalFacts) {
				fm.GlobalFacts = IncompleteFactSlice()
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				return
			}
		}
		return
	}
	// residual ERROR sticky — no invent soft-continue pointer/union makeup past IsPointer residual true
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	// FactMgr.cpp:77–79 — only meta_facts that were registered
	isPtr := v.IsPointer()
	// residual ERROR sticky — no invent soft-skip makeup past IsPointer residual
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	wantPT := MetaFactPointToEnabledSess(fmSess(fm)) && isPtr
	wantUn := MetaFactUnionEnabledSess(fmSess(fm)) && v.Type != nil && v.Type.IsUnion()
	// residual ERROR sticky — no invent soft-skip makeup past IsUnion residual
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		return
	}
	if !wantPT && !wantUn {
		return
	}
	if wantPT {
		rel := FindRelatedPointTo(fm.GlobalFacts, v)
		// residual ERROR sticky — no invent soft-skip makeup past FindRelated residual
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		if rel != nil {
			return
		}
	}
	if wantUn {
		relU := FindRelatedUnion(fm.UnionFacts, v)
		// residual ERROR sticky — no invent soft-skip makeup past FindRelatedUnion residual
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		if relU != nil {
			return
		}
	}
	pt, un := AbstractFactForVarInitSess(fmSess(fm), v)
	if wantPT {
		// incomplete abstract must not invent skip (no fact to add) — sticky ERROR
		if !FactsComplete(pt) {
			fm.GlobalFacts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		// Fact.cpp:94–95 assert(lvar_cnt==1) — no soft invent NewFactPointTo when empty
		for _, f := range pt {
			if f == nil {
				// incomplete abstract list — clear partial GlobalFacts sticky
				fm.GlobalFacts = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			merged := MergeFactIntoSess(fmSess(fm), fm.GlobalFacts, f)
			if !FactsComplete(merged) {
				fm.GlobalFacts = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			fm.SetGlobalFacts(merged, "auto_fact_mgr_1614")
		}
	}
	if wantUn {
		// incomplete union abstract must not invent skip empty UnionFacts merge sticky
		// also wipe GlobalFacts so callers checking FactsComplete abort (not leave
		// sticky-only poison with complete GlobalFacts)
		if !UnionFactsComplete(un) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			fm.GlobalFacts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		for _, uf := range un {
			if uf == nil {
				fm.UnionFacts = IncompleteUnionFactSlice()
				fm.GlobalFacts = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			merged := MergeUnionFactSess(fmSess(fm), fm.UnionFacts, uf)
			if !UnionFactsComplete(merged) {
				fm.UnionFacts = IncompleteUnionFactSlice()
				fm.GlobalFacts = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
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
// FactMgr + Variable always live; sticky (no invent soft-skip makeup past hole).
func (fm *FactMgr) AddNewVarFactAndUpdate(blk *Block, v *Variable) {
	if fm == nil || v == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// FactMgr.cpp:72 — assert(var->is_global()) when blk==nullptr
	// no soft invent facts for non-global "global create" path
	if blk == nil {
		isG := v.IsGlobal()
		// residual ERROR sticky — no invent soft-skip makeup past IsGlobal residual
		if sessHasError(fmSess(fm)) {
			return
		}
		if !isG {
			return
		}
	}
	// incomplete subject map before add — fail closed sticky (no invent push onto holes)
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// FactMgr.cpp:77–104 — abstract_fact_for_var_init; push into global_facts if
	// missing; ALWAYS push the *init* abstract fact into map_facts_in/out even
	// when global_facts already had a related fact (possibly post-analysis).
	// C++: for each f from abstract_fact_for_var_init:
	//   if (!find_related) global_facts.push_back(f);
	//   always map_facts_in/out.push_back(f);  // f is the INIT abstract, not the
	//   existing related fact. After callee RenewFacts leaves g_87→g_62 in the
	//   caller, new_globals handoff still appends init g_87→&g_64 into maps so
	//   StatementFor map_facts_in[body] restore can re-surface init pointees
	//   (seed-2: UP func_39 writes g_64; Go wrongly pushed existing g_62 into maps).
	// Also covers seed-2 e2308: handoff must still update maps when related exists.
	fm.AddNewVarFact(v)
	// AddNewVarFact may wipe GlobalFacts incomplete — stop map push sticky
	if !FactsComplete(fm.GlobalFacts) {
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	// Re-abstract init for map push (C++ always uses abstract_fact_for_var_init
	// results for maps, never the existing related GlobalFacts entry).
	// Soft invent returned early on nil ptInit (union-only subjects) before the
	// eUnionWrite map push below — mid-gen union globals never entered map_facts_in.
	ptInit, unInitEarly := AbstractFactForVarInitSess(fmSess(fm), v)
	// residual ERROR sticky — no invent soft-skip map push past Abstract residual
	if sessHasError(fmSess(fm)) {
		fm.GlobalFacts = IncompleteFactSlice()
		fm.UnionFacts = IncompleteUnionFactSlice()
		return
	}
	if !FactsComplete(ptInit) {
		// incomplete init abstract sticky fail; nil = complete empty (non-pointer /
		// pure union) — skip ePointTo map push, still do eUnionWrite below.
		if ptInit != nil {
			fm.GlobalFacts = IncompleteFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
		// fall through to union map push
	}
	toPush := ptInit
	// Fact* always live after add; nil / incomplete Clone fails closed sticky wipe
	// no invent MapFactsIn-only push when MapFactsOut is nil (one-sided invent)
	for _, f := range toPush {
		if f == nil {
			fm.GlobalFacts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		cl := f.Clone()
		// residual ERROR sticky — no invent soft-push past Clone residual
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			return
		}
		if cl == nil {
			// incomplete PointTo on new fact — fail closed sticky
			fm.GlobalFacts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		// map_facts_in: stm in_block(blk) || blk==null
		// Use parent-chain fall-back so mid-generation for/if bodies (Parent set,
		// not yet linked into parent.Stmts) still receive the new fact. MapFactsOut
		// keeps tree-only stmtIDInBlock — unlinked Block keys cannot FindStmtByID
		// and would IncompleteFactSlice the out slot (generation poison).
		if fm.MapFactsIn != nil {
			for id := range fm.MapFactsIn {
				if blk != nil && !stmtIDInBlockMapIn(fm.Func, id, blk) {
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
				// residual ERROR sticky — no invent soft-push past Clone residual
				if sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					return
				}
				if c2 == nil {
					fm.GlobalFacts = IncompleteFactSlice()
					sessNoteError(fmSess(fm), ErrGeneric)
					return
				}
				fm.MapFactsOut[id] = append(fm.MapFactsOut[id], c2)
			}
		} else {
			// FactMgr.cpp:95–103 — when blk!=null, for every map_facts_out entry call
			// add_fact_out(stm, f) with visibility filters only. C++ does NOT filter
			// by stm->in_block(blk) on the out map (in_block is only for map_facts_in
			// lines 88–93).
			// C++ Block : Statement, so map_facts_out[&if_true] is a Statement* key.
			// Go keys Block.StmID the same way; FindStmtByID only walks Stmts and
			// misses then/else/for-body blocks. Soft-skipping those left parent-locals
			// created mid-else off map_facts_out[if_true], so combine_branch_facts
			// never merged then-arm init points-to (seed-2 func_11: *l_1326 never
			// read g_99 after l_1326=&g_99 then reassigned in else).
			// AddFactOut already drops facts not visible at stm / goto dest.
			for id := range fm.MapFactsOut {
				st := FindStmtByID(fm.Func, id)
				// residual ERROR sticky — no invent soft-continue partial IncompleteFactSlice past FindStmt hole
				if sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					return
				}
				if st == nil {
					// Block StmID (if_true/if_false/for-body) — C++ add_fact_out(Block*)
					b := blockByStmID(fm.Func, id)
					if b == nil {
						// orphan map key soft-skip (no invent IncompleteFactSlice wipe)
						continue
					}
					// FactMgr.cpp:283 — is_var_visible(var, stm); for Block, is_var_on_stack
					// walks stm->parent (Block.cpp/Function.cpp:192).
					if fm.Func != nil {
						vis := fm.Func.IsVarVisible(f.Var, b.Parent)
						if sessHasError(fmSess(fm)) {
							fm.GlobalFacts = IncompleteFactSlice()
							return
						}
						if !vis {
							continue
						}
					}
					// eBlock: no return/break/continue/goto special cases in add_fact_out
					if !FactsComplete(fm.MapFactsOut[id]) {
						fm.MapFactsOut[id] = IncompleteFactSlice()
						continue
					}
					c2 := f.Clone()
					if sessHasError(fmSess(fm)) {
						fm.GlobalFacts = IncompleteFactSlice()
						return
					}
					if c2 == nil {
						fm.GlobalFacts = IncompleteFactSlice()
						sessNoteError(fmSess(fm), ErrGeneric)
						return
					}
					fm.MapFactsOut[id] = append(fm.MapFactsOut[id], c2)
					continue
				}
				parent := FindParentBlockOfStmID(fm.Func, id)
				// residual ERROR sticky — no invent soft-continue AddFactOut past parent residual hole
				if sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					return
				}
				fm.AddFactOut(st, parent, f)
				// residual ERROR sticky — AddFactOut fail-closed on incomplete IR
				if sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					return
				}
			}
		}
	}
	// FactMgr.cpp:69–110 — abstract_fact_for_var_init also yields eUnionWrite;
	// push init union facts into map_facts_in/out (MapUnionFacts*) like PT above.
	// Without this, post_loop AssignGlobalFactsFromMapIn rewinds to entry maps
	// missing mid-body union init facts → empty/stale UnionFacts → over-filter.
	//
	// C++ map_facts_in is one FactVec (both categories share keys). Soft invent
	// iterated only MapUnionFactsIn keys — stmts that had MapFactsIn but no
	// MapUnionFactsIn entry never received mid-gen eUnionWrite init facts, so
	// facts_copy = map_facts_in[body] lacked unions C++ has (FP / IsNonreadableField).
	if !UnionFactsComplete(fm.UnionFacts) {
		fm.UnionFacts = IncompleteUnionFactSlice()
		fm.GlobalFacts = IncompleteFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	// Prefer first abstract's eUnionWrite (same call as PT); re-abstract only if nil.
	unInit := unInitEarly
	if unInit == nil {
		_, unInit = AbstractFactForVarInitSess(fmSess(fm), v)
		if sessHasError(fmSess(fm)) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
	}
	if !UnionFactsComplete(unInit) {
		if unInit != nil {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
		}
		return
	}
	if fm.MapFactsIn == nil && fm.MapFactsOut == nil &&
		fm.MapUnionFactsIn == nil && fm.MapUnionFactsOut == nil {
		return
	}
	// Ensure union partitions exist whenever PT maps do (single FactVec keys)
	if fm.MapFactsIn != nil && fm.MapUnionFactsIn == nil {
		fm.MapUnionFactsIn = make(map[int][]*FactUnion)
	}
	if fm.MapFactsOut != nil && fm.MapUnionFactsOut == nil {
		fm.MapUnionFactsOut = make(map[int][]*FactUnion)
	}
	for _, uf := range unInit {
		if uf == nil {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		// map_facts_in eUnionWrite — iterate PT keys (C++ one map); create missing union slots
		if fm.MapFactsIn != nil {
			for id := range fm.MapFactsIn {
				if blk != nil && !stmtIDInBlockMapIn(fm.Func, id, blk) {
					continue
				}
				if fm.MapUnionFactsIn == nil {
					fm.MapUnionFactsIn = make(map[int][]*FactUnion)
				}
				slot, ok := fm.MapUnionFactsIn[id]
				if !ok {
					// complete empty eUnionWrite half for a PT-only key
					slot = []*FactUnion{}
				} else if !UnionFactsComplete(slot) {
					fm.MapUnionFactsIn[id] = IncompleteUnionFactSlice()
					continue
				}
				// shallow Fact* vector copy (same FactUnion object as C++ push)
				fm.MapUnionFactsIn[id] = append(slot, uf)
			}
		} else if fm.MapUnionFactsIn != nil {
			// union-only map (tests): keep prior behavior
			for id := range fm.MapUnionFactsIn {
				if blk != nil && !stmtIDInBlockMapIn(fm.Func, id, blk) {
					continue
				}
				if !UnionFactsComplete(fm.MapUnionFactsIn[id]) {
					fm.MapUnionFactsIn[id] = IncompleteUnionFactSlice()
					continue
				}
				fm.MapUnionFactsIn[id] = append(fm.MapUnionFactsIn[id], uf)
			}
		}
		// map_facts_out eUnionWrite — FactMgr.cpp:96–105: blk==nil push_all;
		// blk!=nil add_fact_out (return/break/continue/goto filters). Soft invent
		// used IsVarVisible(parent) only → continue map_out re-gained nested
		// union locals after remove_loop_local (seed 2020240685 l_237).
		if fm.MapFactsOut == nil && fm.MapUnionFactsOut == nil {
			continue
		}
		if blk == nil {
			// FactMgr.cpp:102–103 — global var: append to all outs (no add_fact_out)
			ids := map[int]struct{}{}
			if fm.MapFactsOut != nil {
				for id := range fm.MapFactsOut {
					ids[id] = struct{}{}
				}
			}
			if fm.MapUnionFactsOut != nil {
				for id := range fm.MapUnionFactsOut {
					ids[id] = struct{}{}
				}
			}
			if fm.MapUnionFactsOut == nil {
				fm.MapUnionFactsOut = make(map[int][]*FactUnion)
			}
			for id := range ids {
				slot, ok := fm.MapUnionFactsOut[id]
				if !ok {
					slot = []*FactUnion{}
				} else if !UnionFactsComplete(slot) {
					fm.MapUnionFactsOut[id] = IncompleteUnionFactSlice()
					continue
				}
				cp := uf.Clone()
				if cp == nil || sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					if !sessHasError(fmSess(fm)) {
						sessNoteError(fmSess(fm), ErrGeneric)
					}
					return
				}
				fm.MapUnionFactsOut[id] = append(slot, cp)
			}
			continue
		}
		// FactMgr.cpp:100–101 — add_fact_out for every map_facts_out key
		outIDs := map[int]struct{}{}
		if fm.MapFactsOut != nil {
			for id := range fm.MapFactsOut {
				outIDs[id] = struct{}{}
			}
		}
		if fm.MapUnionFactsOut != nil {
			for id := range fm.MapUnionFactsOut {
				outIDs[id] = struct{}{}
			}
		}
		if fm.MapUnionFactsOut == nil {
			fm.MapUnionFactsOut = make(map[int][]*FactUnion)
		}
		for id := range outIDs {
			st := FindStmtByID(fm.Func, id)
			if sessHasError(fmSess(fm)) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				return
			}
			if st == nil {
				// Block StmID key (if_true / for-body): eBlock path — IsVarVisible(parent)
				b := blockByStmID(fm.Func, id)
				if b == nil {
					continue
				}
				if fm.Func != nil && !uf.Var.IsGlobal() {
					vis := fm.Func.IsVarVisible(uf.Var, b.Parent)
					if sessHasError(fmSess(fm)) {
						fm.GlobalFacts = IncompleteFactSlice()
						fm.UnionFacts = IncompleteUnionFactSlice()
						return
					}
					if !vis {
						continue
					}
				}
				slot, ok := fm.MapUnionFactsOut[id]
				if !ok {
					slot = []*FactUnion{}
				} else if !UnionFactsComplete(slot) {
					fm.MapUnionFactsOut[id] = IncompleteUnionFactSlice()
					continue
				}
				cp := uf.Clone()
				if cp == nil || sessHasError(fmSess(fm)) {
					fm.GlobalFacts = IncompleteFactSlice()
					fm.UnionFacts = IncompleteUnionFactSlice()
					if !sessHasError(fmSess(fm)) {
						sessNoteError(fmSess(fm), ErrGeneric)
					}
					return
				}
				fm.MapUnionFactsOut[id] = append(slot, cp)
				continue
			}
			parent := FindParentBlockOfStmID(fm.Func, id)
			if sessHasError(fmSess(fm)) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				return
			}
			fm.AddFactOutUnion(st, parent, uf)
			if sessHasError(fmSess(fm)) {
				fm.GlobalFacts = IncompleteFactSlice()
				fm.UnionFacts = IncompleteUnionFactSlice()
				return
			}
		}
	}
}

// blockByStmID finds a Block on f.Blocks with the given StmID (Block is Statement
// in C++; map_facts_* keys may be Block.StmID for if_true/if_false/for-body).
// Soft-miss nil when not found (no sticky — map key may be orphan).
func blockByStmID(f *Function, stmID int) *Block {
	if f == nil || StmIDUnset(stmID) {
		return nil
	}
	for _, b := range f.Blocks {
		if b != nil && b.StmID == stmID {
			return b
		}
	}
	return nil
}

// stmtIDInBlock reports Statement::in_block(blk) for a statement id under func.
// Tree walk (BlockContainsStmID) for MapFactsOut — statement must already be
// linked so FindStmtByID can resolve it.
func stmtIDInBlock(f *Function, stmID int, blk *Block) bool {
	_ = f
	// Block + live StmID always required; sticky false (align BlockContainsStmID)
	if blk == nil || StmIDUnset(stmID) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// BlockContainsStmID walks nested Then/Else under blk
	return BlockContainsStmID(blk, stmID)
}

// stmtIDInBlockMapIn is Statement::in_block for map_facts_in updates.
// Statement.cpp:380–389 — walk parent chain for blk.
//
// Same as stmtIDInBlock when the statement is linked under blk. Additionally:
// mid-generation for/if bodies exist on Func.Blocks/Stack with Parent set before
// the enclosing For/If is linked into parent.Stmts — tree walk misses them.
// Fall back to Block.Parent chain so outer-parent-local facts reach
// map_facts_in[for-body]. Without that, post_loop Analysis wiped GlobalFacts and
// Lhs opportunistic_validate rejected the still-live local (seed-2 e9003:
// UP U120 vs Go F80 after SelectParentLocal l_138).
func stmtIDInBlockMapIn(f *Function, stmID int, blk *Block) bool {
	if blk == nil || StmIDUnset(stmID) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// Statement.cpp:380–389 — in_block walks parent starting at parent; a statement
	// (including Block-as-statement) is never in_block of itself.
	// BlockContainsStmID(b, b.StmID)==true must not invent self-membership: that
	// pushed body-locals into map_facts_in[body] (seed-2: l_260 on map_in[90] →
	// post_loop reintroduced the local → block-28 OOS of l_144 marked l_260
	// garbage → for 124 strip → e10107 may-null wipe).
	if stmID == blk.StmID {
		return false
	}
	if BlockContainsStmID(blk, stmID) {
		return true
	}
	// Tree miss: sticky incomplete IR from BlockContainsStmID stays fail closed.
	if sessHasError(nil) {
		return false
	}
	if f == nil {
		return false
	}
	for _, b := range f.Blocks {
		if b != nil && b.StmID == stmID {
			return blockParentChainContains(b, blk)
		}
	}
	for _, b := range f.Stack {
		if b != nil && b.StmID == stmID {
			return blockParentChainContains(b, blk)
		}
	}
	return false
}

// blockParentChainContains is Statement::in_block for a Block-as-statement:
// walk Parent (excluding self) looking for target.
func blockParentChainContains(b, target *Block) bool {
	if b == nil || target == nil {
		return false
	}
	for tmp := b.Parent; tmp != nil; tmp = tmp.Parent {
		if tmp == target {
			return true
		}
	}
	return false
}

// lhsAssignPointees mirrors merge_pointees_of_pointer used by abstract_fact_for_assign.
// Used to decide renew (lvar_cnt==1) vs merge (may-point-to).
// Incomplete lhs/facts/merge fails closed IncompleteVariables (not bare nil invent
// lvar_cnt==0, and not IncompleteVariables len==1 invent definitive renew without check).
// Variable always live; sticky IncompleteVariables (no invent soft-skip lhs past hole).
// Incomplete fact map / merge result stays non-sticky IncompleteVariables (soft re-pick).
func lhsAssignPointees(facts []*FactPointTo, lhs *Variable, lhsIndir int) []*Variable {
	if lhs == nil {
		sessNoteError(nil, ErrGeneric)
		return IncompleteVariables()
	}
	// Type* always live for abstract LHS; Type-nil non-special sticky
	// (no invent complete [lhs] soft-success / empty lvars past incomplete type shell)
	// Special null/garbage/tbd have Type nil by design — complete path below.
	if lhs.Type == nil && !IsSpecialPtr(lhs) {
		sessNoteError(nil, ErrGeneric)
		return IncompleteVariables()
	}
	// incomplete map non-sticky hole (fact-map soft re-pick factories)
	if !FactsComplete(facts) {
		return IncompleteVariables()
	}
	coll := lhs.GetCollective()
	// residual ERROR sticky — no invent soft-lvars past GetCollective residual
	if sessHasError(nil) {
		return IncompleteVariables()
	}
	if coll == nil {
		// incomplete field path collective sticky (hard IR hole)
		sessNoteError(nil, ErrGeneric)
		return IncompleteVariables()
	}
	lvars := MergePointeesOfPointer(coll, lhsIndir, facts)
	// residual ERROR sticky — no invent soft-lvars past MergePointees residual
	if sessHasError(nil) {
		return IncompleteVariables()
	}
	if !VariablesComplete(lvars) {
		// missing exist_fact / incomplete merge non-sticky (soft re-pick)
		return IncompleteVariables()
	}
	// FactMgr / abstract LHS pointer fallback when merge empty at indir 0
	if lhsIndir == 0 && len(lvars) == 0 && lhs.Type != nil && lhs.Type.ptrTo != nil {
		lvars = []*Variable{coll}
	}
	return lvars
}

// applyPointToAssignFacts applies point-to facts from abstract_fact_for_assign.
// FactMgr.cpp:376–388 — renew when lvar_cnt==1 non-array; else merge (may-point-to).
// lvarCnt is abstract_fact_for_assign's return (lvars.size(), includes specials
// that make_facts skips — seed-363: *p with p→{null,g_73} must merge not renew).
// Returns (changed, ok). ok=false means incomplete map/merge — no invent apply success.
// Incomplete *facts is wiped to IncompleteFactSlice. Incomplete newFacts alone fails
// closed without wiping prior complete *facts (factory re-pick must not poison FM).
// empty complete newFacts is ok with changed=false.
func applyPointToAssignFacts(facts *[]*FactPointTo, lhs *Variable, lhsIndir int, newFacts []*FactPointTo, lvarCnt int) (changed bool, ok bool) {
	return applyPointToAssignFactsSess(nil, facts, lhs, lhsIndir, newFacts, lvarCnt)
}

func applyPointToAssignFactsSess(s *Session, facts *[]*FactPointTo, lhs *Variable, lhsIndir int, newFacts []*FactPointTo, lvarCnt int) (changed bool, ok bool) {
	// facts accumulator always live; sticky (no invent soft-skip assign apply past hole)
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return false, false
	}
	if !FactsComplete(*facts) {
		// incomplete subject map wiped — sticky (no invent soft re-pick past wiped FM)
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false, false
	}
	// incomplete abstract must not invent empty-apply success (len(nil)==0)
	// leave prior complete *facts for factory soft re-pick (no sticky)
	if !FactsComplete(newFacts) {
		return false, false
	}
	// FactMgr.cpp:376 — if (facts.size() > 0)
	if len(newFacts) == 0 {
		return false, true
	}
	// FactMgr.cpp:380 — lvar_cnt == 1 && !isArray → renew; else merge
	if lvarCnt == 1 && newFacts[0] != nil && newFacts[0].Var != nil && !newFacts[0].Var.IsArray {
		// definitive assignment — renew (strong replace)
		// residual ERROR sticky — no invent soft-continue merge later past RenewFact hole
		if !RenewFactSess(s, facts, newFacts[0]) && sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false, false
		}
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false, false
		}
		for j := 1; j < len(newFacts); j++ {
			merged := MergeFactIntoSess(s, *facts, newFacts[j])
			if !FactsComplete(merged) {
				*facts = IncompleteFactSlice()
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return false, false
			}
			// residual ERROR sticky — no invent soft-continue merge past MergeFact hole
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return false, false
			}
			*facts = merged
		}
		return true, true
	}
	for _, f := range newFacts {
		merged := MergeFactIntoSess(s, *facts, f)
		if !FactsComplete(merged) {
			*facts = IncompleteFactSlice()
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return false, false
		}
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return false, false
		}
		*facts = merged
	}
	return true, true
}

// UpdateFactForAssign mirrors FactMgr::update_fact_for_assign(Lhs, Expression, facts)
// with bare Variable (lhsWant = Variable.Type). Prefer UpdateFactForAssignWant when
// Lhs::get_type() is available (deref-to-union / ExpressionAssign).}

func (fm *FactMgr) UpdateFactForAssign(lhs *Variable, lhsIndir int, rhs *Expression) bool {
	return fm.UpdateFactForAssignWant(lhs, lhsIndir, nil, rhs)
}

// UpdateFactForAssignWant is update_fact_for_assign with Lhs desired type.
// FactMgr.cpp:370–395 + FactUnion.cpp:133 — abstract uses lhs->get_type(), not
// var->type. Soft invent passed Variable only: (*union_ptr)= never transferred
// eUnionWrite (seed-177 g_88 stayed BOTTOM while UP renewed via (*l_90)=).
// Incomplete point-to apply or union merge fails closed (false; GlobalFacts and/or
// UnionFacts cleared — no invent continue union after wiped point-to or partial maps).
func (fm *FactMgr) UpdateFactForAssignWant(lhs *Variable, lhsIndir int, lhsWant *Type, rhs *Expression) bool {
	// FactMgr always has live Lhs subject; sticky no invent soft-skip assign update
	if fm == nil || lhs == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return false
	}
	changed := false
	newFacts, lvarCnt := AbstractFactForAssignSess(fmSess(fm), fm.GlobalFacts, lhs, lhsIndir, rhs)
	// incomplete abstract must not invent empty apply success then union merge
	ptChanged, ptOK := applyPointToAssignFactsSess(fmSess(fm), &fm.GlobalFacts, lhs, lhsIndir, newFacts, lvarCnt)
	if !ptOK {
		// if GlobalFacts was wiped (was already incomplete), also wipe union;
		// incomplete newFacts alone leaves complete GlobalFacts for factory re-pick
		if !FactsComplete(fm.GlobalFacts) {
			fm.UnionFacts = IncompleteUnionFactSlice()
		}
		// sticky already set when map wiped; ensure sticky if apply left HasError
		if !sessHasError(fmSess(fm)) && !FactsComplete(fm.GlobalFacts) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return false
	}
	if ptChanged {
		changed = true
	}
	// FactUnion::abstract_fact_for_assign (meta_facts loop)
	// FactMgr.cpp:376–388 — lvar_cnt==1 non-array → renew_fact; else merge_fact
	// lhsWant = Lhs::get_type(); nil → Variable.Type
	ufacts, lvarCnt := AbstractFactUnionForAssignSess(fmSess(fm), fm.UnionFacts, fm.GlobalFacts, lhs, lhsIndir, lhsWant, rhs)
	// incomplete abstract must not invent empty union merge success; leave prior
	// complete UnionFacts for factory re-pick (do not poison)
	if !UnionFactsComplete(ufacts) {
		return false
	}
	for _, uf := range ufacts {
		// FactUnion* always live from complete abstract — nil hole sticky wipe
		if uf == nil {
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
			return false
		}
		if lvarCnt == 1 && uf.Var != nil && !uf.Var.IsArray {
			// definitive assignment — renew (strong replace), FactMgr.cpp:379–381
			if RenewUnionFactSess(fmSess(fm), &fm.UnionFacts, uf) {
				changed = true
			}
			if sessHasError(fmSess(fm)) || !UnionFactsComplete(fm.UnionFacts) {
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				return false
			}
			continue
		}
		// may-assign — merge_fact lattice join
		merged := MergeUnionFactSess(fmSess(fm), fm.UnionFacts, uf)
		if !UnionFactsComplete(merged) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
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

// MergeUnionFact mirrors merge_fact for eUnionWrite (Fact.cpp:149–171).
// Related subject: if old already implies new, keep old; else clone new and join(old).
// Unrelated: append. Distinct from renew_fact / RenewUnionFact (strong replace).
// FactUnion* always live; nil f or map hole fails closed sticky IncompleteUnionFactSlice
// (no invent empty-complete via UnionFactsComplete(nil) / soft re-pick past wipe).
func MergeUnionFact(facts []*FactUnion, f *FactUnion) []*FactUnion {
	return MergeUnionFactSess(nil, facts, f)
}

func MergeUnionFactSess(s *Session, facts []*FactUnion, f *FactUnion) []*FactUnion {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	if !UnionFactsComplete(facts) {
		sessNoteError(s, ErrGeneric)
		return IncompleteUnionFactSlice()
	}
	for i, old := range facts {
		if old == nil {
			sessNoteError(s, ErrGeneric)
			return IncompleteUnionFactSlice()
		}
		if old.Var != f.Var {
			continue
		}
		// Fact.cpp:155–163 — if old.imply(new) keep old; else copy=new.clone(); copy.join(old)
		if old.Imply(f) {
			if sessHasError(s) {
				return IncompleteUnionFactSlice()
			}
			return facts
		}
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		cp := f.Clone()
		if cp == nil || sessHasError(s) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return IncompleteUnionFactSlice()
		}
		cp.Join(old)
		if sessHasError(s) {
			return IncompleteUnionFactSlice()
		}
		facts[i] = cp
		return facts
	}
	// Fact.cpp:167–169 — not found: push_back(new_fact)
	return append(facts, f)
}

// CreateCFGEdge mirrors FactMgr::create_cfg_edge when dest is a Block*.
// FactMgr.cpp:597–598 — CFGEdge dest is Statement* (Block is Statement).
// Store DestStmID = dest.StmID so Statement::find_edges_in / has_edge_in
// (e->dest == this) match via DestStmID; DestStmID 0 forced FindEdgesInToBlock invent.}

func (fm *FactMgr) CreateCFGEdge(srcID int, dest *Block, postDest, backLink bool) {
	destStmID := 0
	if dest != nil {
		destStmID = dest.StmID
	}
	fm.CreateCFGEdgeTo(srcID, dest, destStmID, postDest, backLink)
}

// CreateCFGEdgeTo is create_cfg_edge with optional dest statement id (goto).
// FactMgr always live; sticky (no invent soft-skip edge create past hole).
// Unset srcID (IncompleteStmID) no-op; valid src id 0 must create edges
// (Statement.cpp:370 — first Statement has stm_id 0).
func (fm *FactMgr) CreateCFGEdgeTo(srcID int, dest *Block, destStmID int, postDest, backLink bool) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if StmIDUnset(srcID) {
		return
	}
	// allow dest nil when destStmID set (break → for-statement edge)
	if dest == nil && StmIDUnset(destStmID) {
		return
	}
	// residual ERROR sticky — no invent soft-append edge past incomplete edge list residual
	if fm.CFGEdges != nil && !CFGEdgesComplete(fm.CFGEdges) {
		sessNoteError(fmSess(fm), ErrGeneric)
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
	return MakeupNewVarFactsSess(nil, oldFacts, newFacts)
}

func MakeupNewVarFactsSess(s *Session, oldFacts *[]*FactPointTo, newFacts []*FactPointTo) bool {
	// FactMgr always has live old_facts accumulator; sticky no invent soft-skip makeup
	if oldFacts == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// incomplete working/snapshot sets fail closed sticky before partial makeup
	if !FactsComplete(*oldFacts) || !FactsComplete(newFacts) {
		*oldFacts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, f := range newFacts {
		// no invent soft-continue past nil fact holes (also covered by FactsComplete)
		if f == nil || f.Var == nil {
			*oldFacts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
			return false
		}
		v := f.Var
		if !v.IsGlobal() && !v.IsLocal() {
			// residual ERROR sticky — no invent soft-continue makeup past IsGlobal/IsLocal hole
			if sessHasError(s) {
				*oldFacts = IncompleteFactSlice()
				return false
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue non-global past IsGlobal residual false path
		if sessHasError(s) {
			*oldFacts = IncompleteFactSlice()
			return false
		}
		related := FindRelatedPointTo(*oldFacts, v)
		// residual ERROR sticky — no invent soft-continue makeup later past FindRelated hole
		if sessHasError(s) {
			*oldFacts = IncompleteFactSlice()
			return false
		}
		if related == nil {
			// FactMgr.cpp:504 — add_new_var_fact(v, old_facts) → abstract_fact_for_var_init
			// no invent NewFactPointTo garbage (tbd/garbage default) for live inits
			AddNewVarFactIntoSess(s, v, oldFacts)
			// AddNewVarFactInto may clear *oldFacts on FieldVars/abstract holes;
			// stop sticky — no invent continue loop and re-append later vars onto nil.
			if *oldFacts == nil || !FactsComplete(*oldFacts) {
				*oldFacts = IncompleteFactSlice()
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return false
			}
			// residual ERROR sticky — no invent soft-continue later makeup past AddNewVar hole
			if sessHasError(s) {
				*oldFacts = IncompleteFactSlice()
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
// Type* always live for non-special subjects; Type-nil sticky clear (no invent
// empty-FieldVars aggregate complete / field walk past Type-nil shell via IsPointer residual).
// facts always live; sticky (no invent soft-skip makeup past hole).}

func AddNewVarFactInto(v *Variable, facts *[]*FactPointTo) {
	AddNewVarFactIntoSess(nil, v, facts)
}

func AddNewVarFactIntoSess(s *Session, v *Variable, facts *[]*FactPointTo) {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// Variable* always live; nil v hole fails closed sticky (clear — no invent skip as absent)
	if v == nil {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	// Type* always live for non-special subjects; specials Type-nil by design
	// (not makeup subjects — complete skip, no invent field walk past shell)
	if v.Type == nil {
		if !IsSpecialPtr(v) {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
		}
		return
	}
	// FactMgr.cpp:77–79 — only when PointTo meta_facts registered
	if !MetaFactPointToEnabledSess(nil) {
		return
	}
	// recurse into aggregate fields (pointer members) like AddNewVarFact
	isPtr := v.IsPointer()
	// residual ERROR sticky — no invent soft-skip field walk past IsPointer residual
	if sessHasError(s) {
		*facts = IncompleteFactSlice()
		return
	}
	isUn := v.Type.IsUnion()
	// residual ERROR sticky — no invent soft-skip field walk past IsUnion residual
	if sessHasError(s) {
		*facts = IncompleteFactSlice()
		return
	}
	if !isPtr && !isUn {
		for _, f := range v.FieldVars {
			if f == nil {
				*facts = IncompleteFactSlice()
				sessNoteError(s, ErrGeneric)
				return
			}
			AddNewVarFactIntoSess(s, f, facts)
			// incomplete hole marker is non-nil; FactsComplete false
			if !FactsComplete(*facts) {
				*facts = IncompleteFactSlice()
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return
			}
		}
		return
	}
	if !isPtr {
		// residual ERROR sticky — no invent soft-skip makeup past non-pointer non-union
		return
	}
	// residual ERROR sticky — no invent soft-continue makeup past IsPointer residual true
	if sessHasError(s) {
		*facts = IncompleteFactSlice()
		return
	}
	if FindRelatedPointTo(*facts, v) != nil {
		// residual ERROR sticky — no invent soft-skip found past FindRelated residual
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return
		}
		return
	}
	// residual ERROR sticky — no invent soft-continue not-found past FindRelated residual false
	if sessHasError(s) {
		*facts = IncompleteFactSlice()
		return
	}
	pt, _ := AbstractFactForVarInitSess(s, v)
	// incomplete abstract must not invent skip (no fact to add) sticky
	if !FactsComplete(pt) {
		*facts = IncompleteFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	// Fact.cpp:94–95 assert(lvar_cnt==1) — no invent garbage shell when empty
	// Fact* always live from abstract; nil hole fails closed (*facts cleared) sticky
	for _, f := range pt {
		if f == nil {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
			return
		}
		cl := f.Clone()
		// residual ERROR sticky — no invent soft-makeup past Clone residual
		if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return
		}
		if cl == nil {
			*facts = IncompleteFactSlice()
			sessNoteError(s, ErrGeneric)
			return
		}
		if FindRelatedPointTo(*facts, f.Var) == nil {
			// residual ERROR sticky — no invent soft-append past FindRelated residual
			if sessHasError(s) {
				*facts = IncompleteFactSlice()
				return
			}
			*facts = append(*facts, cl)
		} else if sessHasError(s) {
			*facts = IncompleteFactSlice()
			return
		}
		// ArrayVariable::itemize: facts abstract onto get_collective()
		// (FactPointTo.cpp:276–277). ExpressionVariable may hold itemized Variable*.
		// is_valid_ptr exact match: IsValidPtr falls back to collective during
		// FunctionInvocationUser::revisit only (FactPointTo.cpp:415–426). Do NOT
		// dual-register itemized subjects into the FactVec — that inflates lattice
		// size vs C++ and breaks same_facts during nested for shortcut reuse
		// (seed-90: 5 itemized extras → full for-visit drops make_iteration IV reads
		// from caller feffect).
	}
}

// FindDanglingGlobalPtrs mirrors FactMgr::find_dangling_global_ptrs.
// FactMgr.cpp:688–700 — non-const global pointers that are dead at function exit.
// Incomplete GlobalFacts fails closed IncompleteVariables DeadGlobals
// (not bare empty invent "no dangling" via VariablesComplete(nil)/len==0).
// FactMgr + Function always live; sticky (no invent soft-skip dangling scan past hole).}

func (fm *FactMgr) FindDanglingGlobalPtrs(f *Function) {
	if fm == nil || f == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	f.DeadGlobals = f.DeadGlobals[:0]
	if !FactsComplete(fm.GlobalFacts) {
		// incomplete map fails closed sticky (no invent empty DeadGlobals success)
		f.DeadGlobals = IncompleteVariables()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	for _, fact := range fm.GlobalFacts {
		// FactsComplete guarantees live fact.Var
		v := fact.Var
		// const pointers should never be dangling; only globals
		if v.IsConst() || !v.IsGlobal() {
			// residual ERROR sticky — no invent soft-continue dead scan past IsConst/IsGlobal hole
			if sessHasError(fmSess(fm)) {
				f.DeadGlobals = IncompleteVariables()
				return
			}
			continue
		}
		if fact.IsDead() {
			// residual ERROR sticky — no invent soft-continue later dead appends past IsDead hole
			if sessHasError(fmSess(fm)) {
				f.DeadGlobals = IncompleteVariables()
				return
			}
			f.DeadGlobals = append(f.DeadGlobals, v)
		} else if sessHasError(fmSess(fm)) {
			// residual ERROR sticky — no invent not-dead soft-skip then complete DeadGlobals past hole
			f.DeadGlobals = IncompleteVariables()
			return
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
	// Expression* always live on StatementReturn; sticky fail closed
	// (no invent garbage RHS transfer as stand-in for missing return value IR)
	if fm == nil || rv == nil || expr == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return false
	}
	// abstract_fact_for_return ≈ abstract_fact_for_assign(facts, Lhs(rv), expr)
	// FactMgr.cpp:408–416 — merge into inputs; fact_changed on merge
	changed := fm.UpdateFactForAssign(rv, 0, expr)
	// incomplete GlobalFacts after assign — sticky; do not invent return out map
	if !FactsComplete(fm.GlobalFacts) {
		fm.GlobalFacts = IncompleteFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
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
// FactMgr.cpp:141–172 — drop facts for oos vars (all Fact categories incl. eUnionWrite);
// mark pointees garbage.
// Delegates to package UpdateFactsForOOSVars (fail closed on fact/var holes).
// FactMgr always live; sticky (no invent soft-skip OOS update past hole).
// Empty vars is complete no-op.
func (fm *FactMgr) UpdateFactsForOOSVars(vars []*Variable) {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if len(vars) == 0 {
		return
	}
	// reuse slice-level fail-closed filter (nil holes → GlobalFacts nil)
	facts := fm.GlobalFacts
	UpdateFactsForOOSVarsSess(fmSess(fm), vars, &facts)
	fm.SetGlobalFacts(facts, "auto_fact_mgr_2437")
	// FactMgr.cpp:143–156 — erase any fact whose subject matches OOS var (FactUnion too).
	// Go keeps UnionFacts separate from GlobalFacts (point-to only).
	UpdateUnionFactsForOOSVarsSess(fmSess(fm), vars, &fm.UnionFacts)
}

// UpdateUnionFactsForOOSVars drops FactUnion subjects matching OOS vars.
// FactMgr.cpp:143–156 — match(f->get_var()) erase (category-agnostic).
// Incomplete maps / vars fail closed sticky IncompleteUnionFactSlice.
func UpdateUnionFactsForOOSVars(vars []*Variable, facts *[]*FactUnion) {
	UpdateUnionFactsForOOSVarsSess(nil, vars, facts)
}

func UpdateUnionFactsForOOSVarsSess(s *Session, vars []*Variable, facts *[]*FactUnion) {
	if facts == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if len(vars) == 0 {
		return
	}
	if !UnionFactsComplete(*facts) {
		*facts = IncompleteUnionFactSlice()
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, v := range vars {
		if v == nil {
			*facts = IncompleteUnionFactSlice()
			sessNoteError(s, ErrGeneric)
			return
		}
	}
	out := make([]*FactUnion, 0, len(*facts))
	for _, f := range *facts {
		drop := false
		for _, v := range vars {
			if v.Match(f.Var) {
				if sessHasError(s) {
					*facts = IncompleteUnionFactSlice()
					return
				}
				drop = true
				break
			}
			if sessHasError(s) {
				*facts = IncompleteUnionFactSlice()
				return
			}
		}
		if !drop {
			out = append(out, f)
		}
	}
	*facts = out
}

// FilterUnionFactsForHandover keeps FactUnion subjects that survive caller_to_callee
// partition (FactMgr.cpp:324–353): globals, params, or pointees of kept point-to facts.
// FunctionInvocationUser.cpp:206 assigns full FactVec then handover partitions it.
// Must run after CallerToCalleeHandover so keepPT is the post-partition lattice.
// Incomplete maps fail closed sticky IncompleteUnionFactSlice on fm.UnionFacts.}

func (fm *FactMgr) FilterUnionFactsForHandover(keepPT []*FactPointTo) {
	if fm == nil || fm.Func == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !UnionFactsComplete(fm.UnionFacts) {
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !FactsComplete(keepPT) {
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !VariablesComplete(fm.Func.Param) {
		fm.UnionFacts = IncompleteUnionFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	out := make([]*FactUnion, 0, len(fm.UnionFacts))
	for _, uf := range fm.UnionFacts {
		v := uf.Var
		isG := v.IsGlobal()
		if sessHasError(fmSess(fm)) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		keep := isG || IsVariableInSet(fm.Func.Param, v)
		if sessHasError(fmSess(fm)) {
			fm.UnionFacts = IncompleteUnionFactSlice()
			return
		}
		if !keep {
			for _, pt := range keepPT {
				ptOK := pt.PointsTo(v)
				if sessHasError(fmSess(fm)) {
					fm.UnionFacts = IncompleteUnionFactSlice()
					return
				}
				if ptOK {
					keep = true
					break
				}
			}
		}
		if keep {
			out = append(out, uf)
		}
	}
	fm.UnionFacts = out
}

// AddParamFacts mirrors FactMgr::add_param_facts.
// FactMgr.cpp:108–116 — update_fact_for_assign each param from arg expression.
// No invent NewFactPointTo when arg missing: nil rhs goes through abstract
// (FactPointTo.cpp:168–169 → garbage for pointers), same as C++ nullptr value.
// Variable* params always live; nil param hole or incomplete assign fails closed
// (*facts nil, stop — no invent skip remaining params / re-accumulate after wipe).
// FactMgr + Func + facts always live; sticky (no invent soft-skip param facts past hole).
func (fm *FactMgr) AddParamFacts(args []*Expression, facts *[]*FactPointTo) {
	if fm == nil || fm.Func == nil || facts == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	for i, p := range fm.Func.Param {
		if p == nil {
			// incomplete Param list fails closed sticky — no invent skip remaining params
			*facts = IncompleteFactSlice()
			sessNoteError(fmSess(fm), ErrGeneric)
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
			if !sessHasError(fmSess(fm)) {
				sessNoteError(fmSess(fm), ErrGeneric)
			}
			return
		}
	}
}

// UpdateFactForAssignInto is the Lhs*/Expression* overload writing into a fact slice.
// FactMgr.cpp:370–395 — renew/merge only; does NOT set Function::fact_changed.
// StatementAssign overload (UpdateFactForAssign) alone sets fact_changed
// (FactMgr.cpp:397–403). add_param_facts uses this path (FactMgr.cpp:108–114);
// inventing FactChanged here forced NeedsRevisit on callees that only received
// pointer params (seed 10482453124604569829: func_53 revisit dropped make_iteration
// IV read g_283 from caller map_accum while FEffect still listed it).
// Incomplete point-to apply or union merge fails closed like UpdateFactForAssign
// (no invent continue union after wiped *facts).
func (fm *FactMgr) UpdateFactForAssignInto(lhs *Variable, lhsIndir int, rhs *Expression, facts *[]*FactPointTo) bool {
	return fm.UpdateFactForAssignIntoWant(lhs, lhsIndir, nil, rhs, facts)
}

// UpdateFactForAssignIntoWant is assign-into with Lhs desired type (see UpdateFactForAssignWant).
// FactMgr.cpp:370–395 Lhs* path — no fact_changed (see UpdateFactForAssignInto).
func (fm *FactMgr) UpdateFactForAssignIntoWant(lhs *Variable, lhsIndir int, lhsWant *Type, rhs *Expression, facts *[]*FactPointTo) bool {
	// FactMgr assign-into always has live lhs + facts accumulator; sticky no invent soft-skip
	if facts == nil || lhs == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return false
	}
	changed := false
	newFacts, lvarCnt := AbstractFactForAssignSess(fmSess(fm), *facts, lhs, lhsIndir, rhs)
	ptChanged, ptOK := applyPointToAssignFactsSess(fmSess(fm), facts, lhs, lhsIndir, newFacts, lvarCnt)
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
		// FactMgr.cpp:376–388 — lvar_cnt==1 non-array → renew_fact; else merge_fact
		ufacts, lvarCnt := AbstractFactUnionForAssignSess(fmSess(fm), fm.UnionFacts, *facts, lhs, lhsIndir, lhsWant, rhs)
		// incomplete abstract: fail closed without poisoning prior complete UnionFacts
		if !UnionFactsComplete(ufacts) {
			return false
		}
		for _, uf := range ufacts {
			// FactUnion* always live from complete abstract — nil hole sticky wipe
			if uf == nil {
				fm.UnionFacts = IncompleteUnionFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return false
			}
			if lvarCnt == 1 && uf.Var != nil && !uf.Var.IsArray {
				if RenewUnionFactSess(fmSess(fm), &fm.UnionFacts, uf) {
					changed = true
				}
				if sessHasError(fmSess(fm)) || !UnionFactsComplete(fm.UnionFacts) {
					fm.UnionFacts = IncompleteUnionFactSlice()
					if !sessHasError(fmSess(fm)) {
						sessNoteError(fmSess(fm), ErrGeneric)
					}
					return false
				}
				continue
			}
			merged := MergeUnionFactSess(fmSess(fm), fm.UnionFacts, uf)
			if !UnionFactsComplete(merged) {
				fm.UnionFacts = IncompleteUnionFactSlice()
				if !sessHasError(fmSess(fm)) {
					sessNoteError(fmSess(fm), ErrGeneric)
				}
				return false
			}
			fm.UnionFacts = merged
			changed = true
		}
		// FactMgr.cpp:370–395 — Lhs* overload never touches func->fact_changed
	}
	return changed
}

// PointsTo mirrors FactPointTo::point_to — loose_match against any pointee.
// FactPointTo.cpp:398–405 — v->loose_match(pointee) || pointee->loose_match(v).
// Incomplete PointTo (nil hole) fails closed true — no invent not-points-to past holes.
func (f *FactPointTo) PointsTo(v *Variable) bool {
	// Fact + subject always live; sticky incomplete — fail closed as points-to
	// (no invent not-points-to / soft re-pick past hole)
	if f == nil || v == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	for _, p := range f.PointTo {
		if p == nil {
			sessNoteError(nil, ErrGeneric)
			return true
		}
		if v.LooseMatch(p) {
			if sessHasError(nil) {
				return true
			}
			return true
		}
		if sessHasError(nil) {
			return true
		}
		if p.LooseMatch(v) {
			if sessHasError(nil) {
				return true
			}
			return true
		}
		if sessHasError(nil) {
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
// FactMgr + Func + inputs always live; sticky (no invent soft-skip handover past hole).
func (fm *FactMgr) CallerToCalleeHandover(args []*Expression, inputs *[]*FactPointTo) {
	if fm == nil || inputs == nil || fm.Func == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// incomplete inputs fail closed sticky before partition (no invent drop via hole skip)
	if !FactsComplete(*inputs) {
		*inputs = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !VariablesComplete(fm.Func.Param) {
		*inputs = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	fm.AddParamFacts(args, inputs)
	if !FactsComplete(*inputs) {
		*inputs = IncompleteFactSlice()
		if !sessHasError(fmSess(fm)) {
			sessNoteError(fmSess(fm), ErrGeneric)
		}
		return
	}
	// partition: keep globals and params
	var keep, rest []*FactPointTo
	for _, f := range *inputs {
		v := f.Var
		isG := v.IsGlobal()
		// residual ERROR sticky — no invent soft-partition past IsGlobal residual
		if sessHasError(fmSess(fm)) {
			*inputs = IncompleteFactSlice()
			return
		}
		if isG || IsVariableInSet(fm.Func.Param, v) {
			// residual ERROR sticky — no invent soft-keep past IsVariableInSet residual
			if sessHasError(fmSess(fm)) {
				*inputs = IncompleteFactSlice()
				return
			}
			keep = append(keep, f)
		} else if sessHasError(fmSess(fm)) {
			// residual ERROR sticky — no invent soft-rest past IsVariableInSet residual false
			*inputs = IncompleteFactSlice()
			return
		} else {
			rest = append(rest, f)
		}
	}
	// transitively keep facts for variables pointed to by kept pointer facts
	for {
		cnt := len(keep)
		for i := 0; i < len(rest); i++ {
			rf := rest[i]
			// Fact* always live after FactsComplete partition; nil hole sticky wipe
			if rf == nil {
				*inputs = IncompleteFactSlice()
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			moved := false
			for _, kf := range keep {
				if kf == nil {
					*inputs = IncompleteFactSlice()
					sessNoteError(fmSess(fm), ErrGeneric)
					return
				}
				pt := kf.PointsTo(rf.Var)
				// residual ERROR sticky — no invent soft-partition past PointsTo residual
				if sessHasError(fmSess(fm)) {
					*inputs = IncompleteFactSlice()
					return
				}
				if pt {
					keep = append(keep, rf)
					rest = append(rest[:i], rest[i+1:]...)
					i--
					moved = true
					break
				}
			}
			if moved {
				continue
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
// FactMgr + facts always live; sticky (no invent soft-skip filter past hole).
func (fm *FactMgr) RemoveRVFacts(facts *[]*FactPointTo) {
	if fm == nil || facts == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	if !FactsComplete(*facts) {
		// incomplete map fails closed sticky (no invent clean filter past holes)
		*facts = IncompleteFactSlice()
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	out := make([]*FactPointTo, 0, len(*facts))
	for _, f := range *facts {
		if f.Var.IsRV() {
			// keep only this function's RV
			if fm.Func != nil && fm.Func.RV != nil {
				match := fm.Func.RV.Match(f.Var)
				// residual ERROR sticky — no invent soft-continue filter past Match hole
				// (Type-nil RV Match residual ERROR+false soft invents drop then keep later non-RV)
				if sessHasError(fmSess(fm)) {
					*facts = IncompleteFactSlice()
					return
				}
				if match {
					out = append(out, f)
				}
			}
			// fm.Func/RV nil: other-function RV drop is complete (no Match probe)
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

// SanityCheckMap mirrors FactMgr::sanity_check_map.
// FactMgr.cpp:703–729 — soft visibility checks on map_facts_in/out (asserts disabled).
// Incomplete maps sticky (no invent soft-pass past holes).
func (fm *FactMgr) SanityCheckMap() {
	if fm == nil {
		sessNoteError(fmSess(fm), ErrGeneric)
		return
	}
	// map_facts_in
	for stmID, facts := range fm.MapFactsIn {
		if !FactsComplete(facts) {
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		st := FindStmtByID(fm.Func, stmID)
		var parent *Block
		if st != nil {
			parent = FindParentBlockOfStmID(fm.Func, stmID)
		}
		for _, f := range facts {
			if f == nil || f.Var == nil {
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			v := f.Var
			if v.IsVisible(parent) {
				if sessHasError(fmSess(fm)) {
					return
				}
				continue
			}
			if sessHasError(fmSess(fm)) {
				return
			}
			// FactMgr.cpp:713–716 — body entry may include params with parent==0
			if parent == nil && fm.Func != nil && IsVariableInSet(fm.Func.Param, v) {
				continue
			}
			// upstream assert disabled — soft skip
		}
	}
	// map_facts_out
	for stmID, facts := range fm.MapFactsOut {
		if !FactsComplete(facts) {
			sessNoteError(fmSess(fm), ErrGeneric)
			return
		}
		parent := FindParentBlockOfStmID(fm.Func, stmID)
		for _, f := range facts {
			if f == nil || f.Var == nil {
				sessNoteError(fmSess(fm), ErrGeneric)
				return
			}
			v := f.Var
			vis := v.IsVisible(parent)
			if sessHasError(fmSess(fm)) {
				return
			}
			if !vis && fm.Func != nil && fm.Func.RV != nil {
				if fm.Func.RV.Match(v) {
					if sessHasError(fmSess(fm)) {
						return
					}
					continue
				}
				if sessHasError(fmSess(fm)) {
					return
				}
			}
			// soft skip when not visible (assert disabled)
		}
	}
}

// GetProgramEndFacts mirrors FactMgr::get_program_end_facts.
// FactMgr.cpp:732–735 — global_facts of first function's FactMgr.
// fms must hold session FMList; first is GetFirstFunction(list).
func GetProgramEndFacts(list *FunctionList, fms *FactMgrMap) []*FactPointTo {
	first := GetFirstFunction(list)
	if first == nil {
		// complete miss when list empty; sticky if hole in list
		if sessHasError(nil) {
			return IncompleteFactSlice()
		}
		return nil
	}
	if fms == nil {
		sessNoteError(nil, ErrGeneric)
		return IncompleteFactSlice()
	}
	fm := fms.ForFunc(first)
	if fm == nil {
		sessNoteError(nil, ErrGeneric)
		return IncompleteFactSlice()
	}
	if !FactsComplete(fm.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
		return IncompleteFactSlice()
	}
	return fm.GlobalFacts
}
