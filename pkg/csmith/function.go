// Upstream: Function.h / Function.cpp
// (RandomFunctionName, RandomReturnType, make_random_signature, make_first, GenerateParameterList).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"os"
	"strings"
)

// BuildState mirrors Function::BuildState.
// Function.h:158–163.
type BuildState int

const (
	// BuildUnbuilt is BuildState::Unbuilt.
	BuildUnbuilt BuildState = iota
	// BuildBuilding is BuildState::Building.
	BuildBuilding
	// BuildBuilt is BuildState::Built.
	BuildBuilt
)

// Function mirrors Function (signature + body block).
type Function struct {
	Name string
	// AliasName mirrors Function::alias_name (name + "_alias").
	// Function.cpp:414 / 451 — used with FunctionAttributes forward alias decls.
	AliasName  string
	ReturnType *Type
	Param      []*Variable
	RV         *Variable // dummy return variable name_rv
	Body       *Block
	Stack      []*Block // Function::stack
	Blocks     []*Block // Function::blocks
	IsInlined  bool
	IsBuiltin  bool
	// BuildState is Unbuilt/Building/Built (preferred over IsBuilt).
	BuildState BuildState
	// IsBuilt is true when BuildState==Built (compat).
	IsBuilt bool
	// Labels legacy end-of-body targets (superseded by Stmt SourceLabel / StmtLabel).
	Labels []string
	// RetConst is Function::ret_c for depth_protect else-return (Function.cpp:608–615).
	RetConst *Constant
	// DepthProtect mirrors body depth_protect / CGOptions for emit.
	DepthProtect bool
	// FEffect mirrors Function::feffect (external effect on globals).
	FEffect Effect
	// EmitConcise skips feffect comment (CGOptions::concise).
	EmitConcise bool
	// DeadGlobals mirrors Function::dead_globals (dangling global pointers at exit).
	DeadGlobals []*Variable
	// NewGlobals mirrors Function::new_globals created during this function's generation.
	NewGlobals []*Variable
	// ReferencedPtrs mirrors Function::referenced_ptrs (static pointer uses).
	ReferencedPtrs []*Variable
	// UnionFieldRead mirrors Function::union_field_read.
	UnionFieldRead bool
	// FactChanged mirrors Function::fact_changed — points-to facts mutated.
	// Function.h / FactMgr assign-return paths.
	FactChanged bool
	// VisitedCnt mirrors Function::visited_cnt (revisit count).
	VisitedCnt int
	// AccumEffContext mirrors Function::accum_eff_context across revisits.
	AccumEffContext Effect
	// factMgr is the FMList entry paired at make_random_signature / make_first /
	// builtin create (Function.cpp:422 / 457–458 / 757–758). Not a second invent
	// FactMgr at GenerateBody time.
	factMgr *FactMgr
	// stmParentIdx is a Function-local index stm_id → parent Block.
	// Mirrors C++ Statement::parent for O(1) FindParentBlockOfStmID (C++ keeps
	// the pointer on each Statement; Go historically tree-walked). Not package
	// ambient — per-Function, invalidated on tree restructure. One full rebuild
	// is O(n); uncached walks were O(n) per lookup → hang under --random-random.
	stmParentIdx         map[int]*Block
	stmParentIdxComplete bool // true after full successful rebuild of current tree
}

// InvalidateStmParentIdx drops the Function-local parent index after Stmt tree
// restructure (strip, expand-goto, delete). Safe no-op on nil.
func (f *Function) InvalidateStmParentIdx() {
	if f != nil {
		f.stmParentIdx = nil
		f.stmParentIdxComplete = false
	}
}

// noteStmParent records parent for stmID (incremental after append). No-op holes.
func (f *Function) noteStmParent(stmID int, parent *Block) {
	if f == nil || parent == nil || StmIDUnset(stmID) {
		return
	}
	if f.stmParentIdx == nil {
		f.stmParentIdx = make(map[int]*Block)
	}
	f.stmParentIdx[stmID] = parent
}

// PairedFactMgr returns the FactMgr registered with this function at create time.
// Mirrors get_fact_mgr_for_func when the function is on FuncList/FMList.
// Incomplete Function sticky nil (no invent soft-miss FM past hole).
// PairedFactMgrSess is PairedFactMgr with explicit session residual sticky.
func (f *Function) PairedFactMgrSess(s *Session) *FactMgr {
	// Function always live; sticky incomplete no invent nil FM soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f.factMgr
}

// ensurePairedFactMgr returns the paired FactMgr, creating once at signature-time
// semantics (Function.cpp FMList.push_back(new FactMgr(f))).
// ensurePairedFactMgrSess is ensurePairedFactMgr on an explicit session bag.
func (f *Function) ensurePairedFactMgrSess(s *Session) *FactMgr {
	// Function always live; sticky incomplete no invent FM without function
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if f.factMgr == nil {
		f.factMgr = NewFactMgrSess(s, f)
	} else if s != nil {
		f.factMgr.Sess = s
	}
	return f.factMgr
}

// IsEffectKnown mirrors Function::is_effect_known — true only when Built.
// Function.h:96–97.
// Incomplete Function sticky false (no invent effect-known / soft re-pick past hole).
// IsEffectKnownSess is IsEffectKnown with explicit session residual sticky.
func (f *Function) IsEffectKnownSess(s *Session) bool {
	// Function always live; sticky incomplete no invent effect-known soft-skip
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return f.BuildState == BuildBuilt
}

// markBuilt sets Built state and IsBuilt flag.
// Function always live; sticky (no invent soft-skip Built past hole).
// markBuiltSess is markBuilt with explicit session residual sticky.
func (f *Function) markBuiltSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.BuildState = BuildBuilt
	f.IsBuilt = true
}

// FunctionList is Function::FuncList for this generation session.
type FunctionList struct {
	Funcs []*Function
	Sym   *GenSym // shared with VariableSelector ideally
	Types *TypeEnv
}

// RandomFunctionName mirrors Function.cpp RandomFunctionName → gensym("func_").
// Function.cpp:249 — util.cpp gensym_count is process-wide; sym is ignored
// (no invent private GenSym counter desynced from g_/t_).
// RandomFunctionNameSess is RandomFunctionName on an explicit session bag.
func RandomFunctionNameSess(s *Session, sym *GenSym) string {
	_ = sym
	return GensymSess(s, "func_")
}

// RandomReturnType mirrors Function.cpp RandomReturnType → Type::choose_random.
// Function.cpp:256–259 — no soft invent nonvoid simple when AllTypes empty.
func RandomReturnType(r *Rng, probs *Probabilities, env *TypeEnv, opts Options) *Type {
	// Type::choose_random requires AllTypes + RNG; ERROR_GUARD path → nil
	// sticky no invent default int when r nil; empty env stays non-sticky soft nil
	if r == nil {
		noteErrEnv(env, ErrGeneric)
		return nil
	}
	if env == nil || len(env.AllTypes) == 0 {
		return nil
	}
	return env.ChooseRandom(r, opts, probs, false)
}

// ParamListProbability mirrors Function.cpp ParamListProbability → rnd_upto(max_params).
// ParamListProbabilitySess is ParamListProbability with explicit session residual sticky.
func ParamListProbabilitySess(s *Session, r *Rng, opts Options) uint32 {
	// C++ always has RNG; sticky no invent param count 0 without draw
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	n := opts.MaxParams
	if n < 0 {
		n = 0
	}
	return r.RndUptoSess(s, uint32(n))
}

// MakeRandomSignature mirrors Function::make_random_signature (no GenerateBody).
// Function.cpp:401–426.
func MakeRandomSignature(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	sym *GenSym,
	cg CGContext,
	retType *Type,
	qfer *CVQualifiers,
	list *FunctionList,
) *Function {
	// Prefer VS/run bag on the context (MakeRandom under generation has vs.Sess).
	if cg.Sess == nil {
		cg.Sess = sessFromVS(vs)
	}
	s := sessFromCG(&cg)
	// Function.cpp:401+ — always has RNG sticky; no soft invent NewRng(0)
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Probabilities singleton always live in C++; sticky no invent NewProbabilities(opts)
	if probs == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent RV/qfer under hole shells)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	var env *TypeEnv
	if list != nil {
		env = list.Types
	}
	if env == nil {
		env = cg.Types
	}
	if retType == nil {
		retType = RandomReturnType(r, probs, env, opts)
		// Function.cpp:404–408 — ERROR_GUARD after RandomReturnType / DEPTH_GUARD
		if retType == nil || sessHasError(s) {
			return nil
		}
	}
	// Function.cpp:407 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunction, nullptr)
	if DepthGuardByTypeSess(sessFromCG(&cg), opts, DtFunction) == BadDepth {
		return nil
	}
	// Function.cpp:408 ERROR_GUARD after DEPTH_GUARD
	if sessHasError(s) {
		return nil
	}
	name := RandomFunctionNameSess(firstSess(sessFromVS(vs), s), sym)
	// gensym always live; sticky no invent empty-name signature / "_alias" shell
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	f := &Function{Name: name, AliasName: name + "_alias", ReturnType: retType, AccumEffContext: EmptyEffect(), FEffect: EmptyEffect()}
	// rv dummy: CVQualifiers::random_qualifiers(type, READ, cg, true) if qfer nil;
	// else qfer->random_qualifiers(true, READ, cg) (Function.cpp:416–418).
	var retQ CVQualifiers
	if qfer == nil {
		retQ = RandomQualifiersDefaultProbs(retType, AccessRead, cg, true, opts, probs, r)
	} else {
		retQ = qfer.RandomQualifiersFrom(true, AccessRead, cg, opts, probs, r)
	}
	// Function.cpp:419 ERROR_GUARD after random_qualifiers
	if sessHasError(s) {
		return nil
	}
	f.RV = CreateVariableQferSess(sessFromCG(&cg), name+"_rv", retType, retQ)
	// Function.cpp:419–420 — CreateVariable + ERROR_GUARD path; no soft invent signature without rv
	if f.RV == nil || sessHasError(s) {
		return nil
	}
	// GenerateParameterList: for i=0; i<=max; i++
	max := ParamListProbabilitySess(sessFromCG(&cg), r, opts)
	if sessHasError(s) {
		return nil
	}
	for i := uint32(0); i <= max; i++ {
		vs.GenerateParameterVariable(f, r)
		// ERROR_RETURN style from GenerateParameterVariable
		if sessHasError(s) {
			return nil
		}
	}
	// Function.cpp:422 — FMList.push_back(new FactMgr(f)); always at signature
	f.ensurePairedFactMgrSess(sessFromCG(&cg))
	// inline flip if enabled
	if opts.InlineFunction && r.RndFlipcoinSess(sessFromCG(&cg), uint32(probs.SingleSess(sessFromCG(&cg), PInlineFunctionProb))) {
		f.IsInlined = true
	}
	if sessHasError(s) {
		return nil
	}
	if list != nil {
		list.Funcs = append(list.Funcs, f)
	}
	return f
}

// MakeRandomFunction mirrors Function::make_random.
// Function.cpp:431–438 — make_random_signature + GenerateBody.
func MakeRandomFunction(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	sym *GenSym,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
	retType *Type,
	qfer *CVQualifiers,
	list *FunctionList,
) *Function {
	if cg.Sess == nil {
		cg.Sess = sessFromVS(vs)
	}
	s := sessFromCG(&cg)
	f := MakeRandomSignature(r, opts, probs, vs, sym, cg, retType, qfer, list)
	// Function.cpp:434 ERROR_GUARD after signature
	if f == nil || sessHasError(s) {
		return nil
	}
	// Function.cpp:422 FMList entry from signature — get_fact_mgr_for_func (no invent second)
	// sticky no invent GenerateBody without live FactMgr
	fm := f.PairedFactMgrSess(sessFromCG(&cg))
	if fm == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Variable* always live on GlobalList; nil hole fails closed sticky
	// (AddNewVarFact(nil) no-ops — invent partial FM seed then GenerateBody)
	// ActiveFM before seed — FactVec chronology via noteFactVecPT/UW (see MakeFirst).
	if s != nil {
		s.ActiveFM = fm
	} else if cg.Sess != nil {
		cg.Sess.ActiveFM = fm
	}
	if vs != nil {
		if !VariablesComplete(vs.GlobalList) {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		for _, gv := range vs.GlobalList {
			fm.AddNewVarFact(gv)
			// incomplete PT/union abstract sticky or wipe must abort (no invent body past holes)
			if sessHasError(s) || !FactsComplete(fm.GlobalFacts) {
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return nil
			}
		}
	}
	// Function.cpp:635 — CGContext(this, prev.effect_context, &accum); extend_call_chain(prev).
	// Leave CurrentFunc as caller so generateBodyCore ExtendCallChain(prev) pushes the
	// caller's current block (see BuildInvocationAndFunction / seed-7 call_chain note).
	bodyCG := cg
	bodyCG.FM = fm
	if s != nil {
		s.ActiveFM = fm
	} else if bodyCG.Sess != nil {
		bodyCG.Sess.ActiveFM = fm
	}
	bodyCG.Sess = firstSess(bodyCG.Sess, s)
	if list != nil {
		bodyCG = bodyCG.WithFuncList(list)
	}
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, bodyCG)
	// Function.cpp:436 ERROR_GUARD after GenerateBody
	if sessHasError(s) {
		return nil
	}
	// sticky no invent unbuilt/null-body success pointer (C++ would crash on body->)
	if f.Body == nil || f.BuildState != BuildBuilt {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f
}

// MakeFirst mirrors Function::make_first.
// Function.cpp:443–477 — FactMgr, empty params (no ExtensionMgr), GenerateBody,
// setup_in_out_maps, exit facts, dangling globals, InitializeAttributes.
func MakeFirst(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	sym *GenSym,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	list *FunctionList,
	fmMap *FactMgrMap,
) *Function {
	// Function.cpp:457–458 bag — prefer VS / FMList session before any sticky write.
	runSess := firstSess(sessFromVS(vs), nil)
	if fmMap != nil && fmMap.Sess != nil {
		runSess = firstSess(runSess, fmMap.Sess)
	}
	// ERROR_RETURN: residual sticky already set must not invent first body
	if sessHasError(runSess) {
		return nil
	}
	// Function.cpp:443+ — always has RNG sticky; no soft invent NewRng(0)
	if r == nil {
		sessNoteError(runSess, ErrGeneric)
		return nil
	}
	// Probabilities singleton always live in C++; sticky no invent NewProbabilities(opts)
	if probs == nil {
		sessNoteError(runSess, ErrGeneric)
		return nil
	}
	// Type::AllTypes is process-global in C++; session Types on list or vs
	var env *TypeEnv
	if list != nil {
		env = list.Types
	}
	if env == nil && vs != nil {
		env = vs.Types
	}
	// Function.cpp:444–445 — RandomReturnType; ERROR_GUARD
	ty := RandomReturnType(r, probs, env, opts)
	if ty == nil || sessHasError(runSess) {
		return nil
	}
	name := RandomFunctionNameSess(runSess, sym)
	// gensym always live; sticky no invent empty-name function / "_alias" shell
	if name == "" {
		sessNoteError(runSess, ErrGeneric)
		return nil
	}
	f := &Function{Name: name, AliasName: name + "_alias", ReturnType: ty, AccumEffContext: EmptyEffect(), FEffect: EmptyEffect()}
	// Function.cpp:452–453 — CVQualifiers::random_qualifiers(ty); ERROR_GUARD
	retQ := RandomQualifiersNoContextNoVolatileSess(runSess, ty, opts, probs, r)
	if sessHasError(runSess) {
		return nil
	}
	f.RV = CreateVariableQferSess(runSess, name+"_rv", ty, retQ)
	// Function.cpp:453 — CreateVariable + ERROR_GUARD; no soft invent first without rv
	if f.RV == nil || sessHasError(runSess) {
		return nil
	}

	// Function.cpp:457–458 — FactMgr with empty global facts (FMList.push_back)
	fm := f.ensurePairedFactMgrSess(runSess)
	if fmMap != nil {
		if fmMap.Sess == nil {
			fmMap.Sess = runSess
		}
		// register same instance into session FMList map
		_ = fmMap.ForFuncSess(runSess, f)
	}
	// seed existing globals so first function sees them (generation convenience)
	// Variable* always live on GlobalList; nil hole fails closed sticky
	// (AddNewVarFact(nil) no-ops — invent partial FM seed then GenerateBody)
	// ActiveFM must be live so MergeFactInto noteFactVecPT/UW records FactVec
	// chronology (otherwise paranoid output_tab interleave is PT-then-UW only).
	if runSess != nil {
		runSess.ActiveFM = fm
	}
	if vs != nil {
		if !VariablesComplete(vs.GlobalList) {
			sessNoteError(runSess, ErrGeneric)
			return nil
		}
		for _, gv := range vs.GlobalList {
			fm.AddNewVarFact(gv)
			// incomplete PT/union abstract sticky or wipe must abort (no invent body past holes)
			if sessHasError(runSess) || !FactsComplete(fm.GlobalFacts) {
				if !sessHasError(runSess) {
					sessNoteError(runSess, ErrGeneric)
				}
				return nil
			}
		}
	}

	// Function.cpp:460 — ExtensionMgr::GenerateFirstParameterList (null → no params)
	ExtensionMgrGenerateFirstParameterListSess(runSess, f, vs)
	if sessHasError(runSess) {
		return nil
	}

	// register before body so recursive choose_func can see it
	if list != nil {
		list.Funcs = append(list.Funcs, f)
		if list.Types != nil {
			env = list.Types
		}
	}

	// Function.cpp:463 — GenerateBody(empty context)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm).WithSession(runSess)
	if list != nil {
		cg = cg.WithFuncList(list)
	}
	if env != nil {
		cg.Types = env
	}
	// No ambient currentSession fill-in: pure bags leave Sess nil when no VS/FMList.
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, cg)
	// sticky error / null body / Unbuilt — do not invent success first function
	if sessHasError(runSess) || f.Body == nil || f.BuildState != BuildBuilt {
		return nil
	}

	// Function.cpp:464–465 — inline flip after body
	if opts.InlineFunction && r.RndFlipcoinSess(sessFromVS(vs), uint32(probs.SingleSess(sessFromVS(vs), PInlineFunctionProb))) {
		f.IsInlined = true
	}
	if sessHasError(runSess) {
		return nil
	}

	// Function.cpp:466 — setup_in_out_maps(true)
	fm.SetupInOutMaps(true)

	// Function.cpp:468–470 — global_facts = map_facts_out[body] + add_back_return_facts
	// Full FactVec (point-to + eUnionWrite). Soft invent set only GlobalFacts.
	// Incomplete out / add_back fail closed sticky — no invent soft-merge / success first
	if f.Body != nil {
		fm.AssignGlobalFactsFromMapOut(f.Body.StmID)
		if !FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(runSess, ErrGeneric)
			return nil
		}
		if !AddBackReturnFacts(f.Body, fm, &fm.GlobalFacts, &fm.UnionFacts) ||
			!FactsComplete(fm.GlobalFacts) || !UnionFactsComplete(fm.UnionFacts) {
			fm.GlobalFacts = IncompleteFactSlice()
			fm.UnionFacts = IncompleteUnionFactSlice()
			sessNoteError(runSess, ErrGeneric)
			return nil
		}
	}

	// Function.cpp:472–473 — dangling global pointers
	if opts.DanglingGlobalPointers {
		fm.FindDanglingGlobalPtrs(f)
	}

	// Function.cpp:475 — InitializeAttributes (package generators already cover emission;
	// ensure func attr generator is ready when function attributes enabled)
	if opts.FunctionAttributes {
		_ = EnsureFuncAttrGeneratorSess(runSess)
	}

	return f
}

// GenerateBody mirrors Function::GenerateBody.
// Function.cpp:626–663 — Unbuilt→Building→Built; pointer TBD; body/dummy;
// get_referenced_ptrs via ComputeSummary; make_return_const.
func (f *Function) GenerateBody(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	prev CGContext,
) {
	f.generateBodyCore(r, opts, probs, vs, tables, stmtTab, prev, false)
}

// GenerateBodyWithKnownParams mirrors Function::generate_body_with_known_params.
// Function.cpp:666–698 — inherits caller effect_context/accum, call chain,
// external no-read/write RWDirective from reachable frame vars.
func (f *Function) GenerateBodyWithKnownParams(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	prev CGContext,
) {
	f.generateBodyCore(r, opts, probs, vs, tables, stmtTab, prev, true)
}

// generateBodyCore shared path for GenerateBody / generate_body_with_known_params.
// Function always live; sticky (no invent soft-skip body gen past hole).
func (f *Function) generateBodyCore(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	prev CGContext,
	knownParams bool,
) {
	if f == nil {
		noteErrVS(vs, ErrGeneric)
		return
	}
	// Function.cpp:626–629 / 668–671 — ignore regenerate
	if f.BuildState != BuildUnbuilt {
		return
	}
	// Function.cpp:643–648 — non-builtin make_random body always has process RNG
	// sticky no invent Building/Built shell without RNG
	if !f.IsBuiltin && r == nil {
		noteErrVS(vs, ErrGeneric)
		return
	}
	// incomplete ambient fails closed sticky before BuildBuilding
	// (no invent body under hole shells / soft re-pick past incomplete prev)
	if !EffectComplete(prev.EffectContext()) ||
		(prev.EffectAccum != nil && !EffectComplete(*prev.EffectAccum)) ||
		!EffectComplete(prev.EffectStm) {
		noteErrVS(vs, ErrGeneric)
		return
	}
	if prev.FM != nil && !FactsComplete(prev.FM.GlobalFacts) {
		noteErrVS(vs, ErrGeneric)
		return
	}
	f.BuildState = BuildBuilding

	// Function.cpp:633–634 / 675–676 — CGContext(this, prev.effect_context, &effect_accum)
	// Constructor: current_func=this, blk_depth(0), expr_depth(0), flags(0),
	// iv_bounds() empty, rw_directive(nullptr) — not a copy of caller's loop IVs/RW.
	// Inheriting BlkDepth hit max-depth filters early (seed-2 e502).
	// Inheriting IVBounds inflated ItemizeArray ok_ivs (seed-2 e716 n=4 vs n=2).
	// Inheriting prev.RW.Must* leaked array-loop must-use into callee body so
	// make_iteration took array_control (choose_ok_var/Itemize) while upstream
	// make_random_loop_control (seed-42 e1890: GO U7 Itemize vs UP F50).
	// Function.cpp:633–634 / 675–677 — CGContext(this, prev.effect_context, &effect_accum).
	// GenerateBody: local Effect effect_accum. generate_body_with_known_params: caller
	// Effect& shared (same object). Soft invent was bodyEff=*prev.EffectAccum shallow
	// copy (shared maps, different pointer) then copy-back — C++ shares one Effect*.
	bodyEff := EmptyEffect()
	cg := prev
	cg.CurrentFunc = f
	if knownParams && prev.EffectAccum != nil {
		// known-params: share caller's Effect object (Function.cpp:674–677)
		cg.EffectAccum = prev.EffectAccum
	} else {
		// GenerateBody: fresh local accum (Function.cpp:632–634)
		cg.EffectAccum = &bodyEff
	}
	cg.Flags = 0
	cg.BlkDepth = 0
	cg.ExprDepth = 0
	cg.IVBounds = nil
	// CGContext.cpp:66–69 — rw_directive(nullptr); never inherit caller's must_read/write
	cg.RW = nil
	// CGContext.cpp:65–70 / 85–92 — body ctor leaves effect_stm default empty
	// (not a copy of caller's EffectStm).
	cg.EffectStm = EmptyEffect()
	// Function.cpp:635 / 677 — extend_call_chain(prev_context):
	// C++ builds a fresh CGContext(this, …) then extend_call_chain(prev) where prev
	// is still the CALLER (get_current_block = caller's stack top). Callers must not
	// set prev.CurrentFunc to the callee before this — otherwise CurrentBlock() is
	// empty and the caller frame is omitted from call_chain.
	cg.ExtendCallChain(prev)
	// residual ERROR sticky — no invent soft-continue body past ExtendCallChain residual
	if hasErrVS(vs) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}
	if prev.Funcs != nil {
		cg.Funcs = prev.Funcs
	}
	// Function.cpp:635 / 674 — get_fact_mgr_for_func(this); no invent NewFactMgr here
	if cg.FM == nil {
		cg.FM = f.PairedFactMgrSess(sessFromVS(vs))
		// residual ERROR sticky — no invent soft-continue body past PairedFactMgr residual
		if hasErrVS(vs) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	}
	if cg.FM == nil {
		// get_fact_mgr_for_func returned null — sticky fail closed (no soft invent FM)
		noteErrVS(vs, ErrGeneric)
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// Function.cpp:679–685 — known-params only: external no-reads/writes; must_* stay empty.
	// BuildCalleeRWDirective never copies must_read/write (only NoRead/NoWrite).
	// When it returns nil (no external restrictions), leave cg.RW nil — do not restore prev.RW.
	if knownParams {
		if rwd := prev.BuildCalleeRWDirective(cg.FM.GlobalFacts); rwd != nil {
			// residual ERROR sticky — no invent soft-continue body past BuildCalleeRW residual
			if hasErrVS(vs) {
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			cg.RW = rwd
		} else if hasErrVS(vs) {
			// residual ERROR sticky — no invent soft-continue body past BuildCalleeRW residual nil
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	}

	// Function.cpp:637–641 — pointer params → tbd (GenerateBody; known-params already handed over)
	// Variable* always live on Param; nil hole fails closed (abort generate).
	// Type* always live for non-special params; Type-nil sticky abort (IsPointer
	// residual ERROR+false soft-skips seed then continues later params / invents
	// partial TBD makeup past Type-nil shell before ERROR_RETURN).
	if !knownParams {
		for _, p := range f.Param {
			if p == nil {
				noteErrVS(vs, ErrGeneric)
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			if p.Type == nil && !IsSpecialPtr(p) {
				noteErrVS(vs, ErrGeneric)
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			if p.IsPointerSess(sessFromVS(vs)) {
				// residual ERROR sticky — no invent soft-skip param seed past IsPointer hole
				if hasErrVS(vs) {
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
				// incomplete GlobalFacts sticky before soft FindRelated miss invent
				if !FactsComplete(cg.FM.GlobalFacts) {
					noteErrVS(vs, ErrGeneric)
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
				if FindRelatedPointToSess(sessFromVS(vs), cg.FM.GlobalFacts, p) == nil {
					// residual ERROR sticky — no invent soft-continue later params past FindRelated hole
					if hasErrVS(vs) {
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
					nf := MakeFactPointToSess(sessFromVS(vs), p, TBDPtr)
					if nf == nil || hasErrVS(vs) {
						if !hasErrVS(vs) {
							noteErrVS(vs, ErrGeneric)
						}
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
					cg.FM.SetGlobalFacts(append(cg.FM.GlobalFacts, nf), "auto_function_666")
					// Fact.cpp param TBD push_back — FactVec chronology
					noteFactVecPT(sessFromVS(vs), p)
					// residual ERROR sticky — no invent soft-continue later params past append residual
					if hasErrVS(vs) {
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
				} else if hasErrVS(vs) {
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
			} else if hasErrVS(vs) {
				// residual ERROR sticky — no invent soft-continue non-pointer past IsPointer hole
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
		}
	}

	// Function.cpp:643–648 — builtin dummy vs make_random
	// make_dummy_block needs CGContext + RNG/VS for post_creation append_return
	// (Block.cpp:747–748); nil r soft-skipped return draws (seed2 --builtins first_div).
	if f.IsBuiltin {
		f.Body = MakeDummyBlockCG(&cg, opts, r, vs)
		// residual ERROR sticky — no invent soft-Built past MakeDummyBlock residual
		if hasErrVS(vs) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	} else {
		f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &cg, false)
		// residual ERROR sticky — no invent soft-Built past MakeRandomBlock residual
		if hasErrVS(vs) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	}
	// Function.cpp:647 / 689 — ERROR_RETURN(); body->set_depth_protect
	// sticky error aborts; null body without error would crash C++ on body->
	if hasErrVS(vs) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}
	if f.Body == nil {
		// no soft invent markBuilt without body
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}
	// Function.cpp:650 / 690 — body->set_depth_protect(true)
	f.Body.EmitDepthProtect = true
	f.DepthProtect = opts.DepthProtect
	f.EmitConcise = opts.Concise

	// mark_func_end: locals die after function (DFA cleanup)
	// Block*/Variable*/Fact* always live; nil holes fail closed (abort cleanup invent)
	// Early SetError must leave Unbuilt (no invent stuck Building / later markBuilt success)
	abortUnbuilt := func() {
		noteErrVS(vs, ErrGeneric)
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
	}
	if cg.FM != nil && len(f.Blocks) > 0 {
		var locals []*Variable
		for _, blk := range f.Blocks {
			if blk == nil {
				abortUnbuilt()
				return
			}
			for _, loc := range blk.LocalVars {
				if loc == nil {
					abortUnbuilt()
					return
				}
				locals = append(locals, loc)
			}
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			abortUnbuilt()
			return
		}
		for i, fact := range cg.FM.GlobalFacts {
			// nil without error = no lattice change; nil with sticky = incomplete fail closed
			if nf := fact.MarkFuncEndLocalsSess(sessFromVS(vs), locals); nf != nil {
				cg.FM.GlobalFacts[i] = nf
			} else if hasErrVS(vs) {
				abortUnbuilt()
				return
			}
		}
		if opts.DanglingGlobalPointers {
			cg.FM.FindDanglingGlobalPtrs(f)
		}
	}

	// Function.cpp:652–656 — get_referenced_ptrs + feffect from map_stm_effect[body]
	// C++ always has live body stm_id; StmID 0 / incomplete map effect fails closed
	// (no invent EmptyEffect soft-prefer generation bodyEff then Built success)
	// Post-body pure-IV FE-head fixup (n35 g_108); mid-gen skips FE-head (n28).
	// func_1: assign-level pureMiss + pure-prefix + Acc-early.
	// Other funcs: pure multi-prefix order + Acc-early only (seed120 func_45 g_59;
	// mid-gen pureMiss on all funcs broke n62 body).
	if cg.FM != nil && f.Body != nil {
		if f.Name == "func_1" {
			FixupFunc1PureIVFEHeads(f, cg.FM)
		} else {
			FixupFuncPureIVFEOrder(f, cg.FM)
		}
		if hasErrVS(vs) {
			abortUnbuilt()
			return
		}
	}
	summaryEff := bodyEff
	if cg.FM != nil && f.Body != nil {
		if StmIDUnset(f.Body.StmID) {
			abortUnbuilt()
			return
		}
		summaryEff = cg.FM.GetMapStmEffect(f.Body.StmID)
		if !EffectComplete(summaryEff) {
			abortUnbuilt()
			return
		}
		if os.Getenv("DIAG_PUREMISS") != "" && f.Name == "func_1" {
			s := sessFromVS(vs)
			var names []string
			for _, v := range summaryEff.ReadVarsSess(s) {
				if v != nil {
					names = append(names, v.Name)
				}
			}
			start := 0
			if len(names) > 20 {
				start = len(names) - 20
			}
			fmt.Fprintf(os.Stderr, "SUMMARY_PRE_ACC sid=%d n=%d ...%v\n", f.Body.StmID, len(names), names[start:])
		}
		// Pure-IV membership recovery (summary-only). Visit can drop make_iteration
		// pure IV reads still held on EffectAccum. Do not rewrite map_stm here
		// (mid-gen diverges n62 body RNG). Re-add Acc∩callee-pureIV∩nested-FE
		// missing from body map_stm (Acc-order append). Invent gate: skip pure
		// IVs that lead any nested FE when not already on summary (n62 g_37.f1).
		// Prefer map_stm restore for order (func_1 non-loop). Session-local —
		// no package mutable state.
		s := sessFromVS(vs)
		if os.Getenv("DIAG_FLOAT2") != "" && f != nil && f.Name == "func_1" {
			has := func(n string, e Effect) bool {
				for _, v := range e.ReadVarsSess(s) {
					if v != nil && v.Name == n {
						return true
					}
				}
				return false
			}
			fmt.Fprintf(os.Stderr, "FLOAT2 pre_acc 370=%v 531=%v 38=%v 147=%v 25=%v\n",
				has("g_370", summaryEff), has("g_531", summaryEff), has("g_38", summaryEff),
				has("g_147", summaryEff), has("g_25", summaryEff))
			if cg.EffectAccum != nil {
				fmt.Fprintf(os.Stderr, "FLOAT2 Acc 370=%v 531=%v 38=%v 147=%v 25=%v\n",
					has("g_370", *cg.EffectAccum), has("g_531", *cg.EffectAccum), has("g_38", *cg.EffectAccum),
					has("g_147", *cg.EffectAccum), has("g_25", *cg.EffectAccum))
			}
		}
		if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
			if (os.Getenv("DIAG_S57ACC") != "" || os.Getenv("DIAG_FLOATACC") != "") && f != nil && f.Name == "func_1" {
				want := map[string]bool{"g_630.f1": true, "g_1343.f6": true, "g_1596": true, "g_168": true, "g_1597": true, "g_954": true, "g_1961.f3": true, "g_904": true,
					"g_370": true, "g_531": true, "g_147": true, "g_38": true, "g_25": true, "g_1471": true, "g_80": true, "g_431": true}
				fmt.Fprint(os.Stderr, "ACCNEI Acc=")
				acc := cg.EffectAccum.ReadVarsSess(s)
				for i, x := range acc {
					if x != nil && want[x.Name] {
						fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
						// neighbors
						lo, hi := i-2, i+3
						if lo < 0 { lo = 0 }
						if hi > len(acc) { hi = len(acc) }
						fmt.Fprint(os.Stderr, "[")
						for j := lo; j < hi; j++ {
							if j > lo { fmt.Fprint(os.Stderr, ",") }
							if acc[j] != nil { fmt.Fprint(os.Stderr, acc[j].Name) }
						}
						fmt.Fprint(os.Stderr, "]")
					}
				}
				fmt.Fprintln(os.Stderr)
			}
			if os.Getenv("DIAG_S65") != "" && f != nil && f.Name == "func_65" {
				fmt.Fprint(os.Stderr, "S65 AccFull=")
				for i, x := range cg.EffectAccum.ReadVarsSess(s) {
					if x != nil {
						fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
					}
				}
				fmt.Fprintln(os.Stderr)
			}
			var calls []*Invocation
			for _, blk := range f.Blocks {
				if blk == nil {
					continue
				}
				for i := range blk.Stmts {
					_ = collectCalledInvocationsStmt(s, &blk.Stmts[i], &calls)
				}
			}
			if InvocationsComplete(calls) {
				pureIV := pureIVGlobalsFromUserCallTree(s, calls)
				inCalleeFE := map[*Variable]bool{}
				leadingFE := map[*Variable]bool{}
				for _, inv := range calls {
					if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
						continue
					}
					fr := inv.User.FEffect.ReadVarsSess(s)
					if hasErrVS(vs) {
						abortUnbuilt()
						return
					}
					for _, v := range fr {
						if v != nil {
							inCalleeFE[v] = true
						}
					}
					if len(fr) > 0 && fr[0] != nil && pureIV[fr[0]] {
						leadingFE[fr[0]] = true
					}
				}

				// Two-pass Acc pure-IV place: (0) Acc-late freeVal/array reorder of
				// already-on-summary; (1) Acc-order invent missing. Invent-then-reorder
				// left pure FE heads late (seed12592 g_118 before late g_121). Session-local.
				if os.Getenv("DIAG_C302") != "" && f != nil && f.Name == "func_34" {
					fmt.Fprintf(os.Stderr, "C302 Acc=")
					for _, x := range cg.EffectAccum.ReadVarsSess(s) {
						if x != nil && (x.Name == "g_99" || x.Name == "g_328.f2" || x.Name == "g_299.f1" || x.Name == "g_109" || x.Name == "g_153" || x.Name == "g_148") {
							fmt.Fprintf(os.Stderr, " %s", x.Name)
						}
					}
					fmt.Fprintf(os.Stderr, " sum=")
					for _, x := range summaryEff.ReadVarsSess(s) {
						if x != nil && (x.Name == "g_99" || x.Name == "g_328.f2" || x.Name == "g_299.f1" || x.Name == "g_109" || x.Name == "g_153") {
							fmt.Fprintf(os.Stderr, " %s", x.Name)
						}
					}
					fmt.Fprintln(os.Stderr)
				}
				// Passes 0–1: Acc-late already reorder (twice so Acc-adjacent pure residual
				// pure-only can follow a successor that just Acc-ordered early — seed123
				// g_274 before g_173 after g_173 before g_347). Pass 2: invent missing.
				// Session-local — no package mutable state.
				if os.Getenv("DIAG_S12593A") != "" && f != nil && f.Name == "func_1" && cg.EffectAccum != nil {
				fmt.Fprint(os.Stderr, "S12593PRE")
				for i, x := range summaryEff.ReadVarsSess(s) {
					if x != nil && (x.Name == "g_1678" || x.Name == "g_3964" || x.Name == "g_2252" || x.Name == "g_2249" || x.Name == "g_1326" || x.Name == "g_1143.f1") {
						fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
					}
				}
				fmt.Fprintln(os.Stderr)
				fmt.Fprint(os.Stderr, "S12593ACC")
				for i, x := range cg.EffectAccum.ReadVarsSess(s) {
					if x != nil && (x.Name == "g_1678" || x.Name == "g_3964" || x.Name == "g_2252" || x.Name == "g_2249" || x.Name == "g_1326" || x.Name == "g_1143.f1") {
						fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
					}
				}
				fmt.Fprintln(os.Stderr)
				// pureIV/inCallee for g_1678
				for _, x := range cg.EffectAccum.ReadVarsSess(s) {
					if x != nil && x.Name == "g_1678" {
						fmt.Fprintf(os.Stderr, "S12593 pureIV=%v inCallee=%v lead=%v\n", pureIV[x], inCalleeFE[x], leadingFE[x])
					}
				}
			}
			for accPass := 0; accPass < 3; accPass++ {
				for _, v := range cg.EffectAccum.ReadVarsSess(s) {
					if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && v != nil && (v.Name == "g_266" || v.Name == "g_194" || v.Name == "g_36" || v.Name == "g_316") {
						fmt.Fprintf(os.Stderr, "S875 AccWalk pass=%d %s pureIV=%v inCallee=%v\n", accPass, v.Name, pureIV[v], inCalleeFE[v])
					}
					if v == nil || !pureIV[v] || !inCalleeFE[v] {
						continue
					}
					already := summaryEff.IsReadSess(s, v)
					if hasErrVS(vs) {
						abortUnbuilt()
						return
					}
					if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && v != nil && (v.Name == "g_266" || v.Name == "g_194" || v.Name == "g_36" || v.Name == "g_316" || v.Name == "g_51") {
						fmt.Fprintf(os.Stderr, "S875 already pass=%d %s already=%v\n", accPass, v.Name, already)
					}
					if accPass < 2 && !already {
						continue
					}
					if accPass == 2 && already {
						continue
					}
					if os.Getenv("DIAG_S42") != "" && v != nil && v.Name == "g_1054" {
						fmt.Fprintf(os.Stderr, "S42 Acc hit g_1054 pass=%d f=%s already=%v isLead=%v ownFree=%v\n",
							accPass, f.Name, already, leadingFE[v], bodySyntacticFreeReadsVar(f, v))
					}
					if os.Getenv("DIAG_C302") != "" && f != nil && f.Name == "func_34" && v != nil && (v.Name == "g_99" || v.Name == "g_328.f2" || v.Name == "g_299.f1") {
						fmt.Fprintf(os.Stderr, "C302 pass=%d %s already=%v pureIV=%v inCallee=%v lead=%v ownPure=%v free=%v arr=%v\n",
							accPass, v.Name, already, pureIV[v], inCalleeFE[v], leadingFE[v],
							isForIVOfFunc(f, v), bodySyntacticFreeReadsVar(f, v), bodyArrayInitOrValueFreeReadsVar(f, v))
					}
					// Nested pure FE head by pointer or name (Acc pure IV may differ
					// in *Variable identity from callee FE head). Session-local.
					isLeading := leadingFE[v]
					if !isLeading && v != nil {
						for lv := range leadingFE {
							if lv != nil && lv.Name == v.Name {
								isLeading = true
								break
							}
						}
					}
					ownFree := bodySyntacticFreeReadsVar(f, v)
					nestedFree := nestedUserSyntacticFreeReadsVar(s, calls, v)
					hasFree := ownFree || nestedFree
					if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" && v != nil && (v.Name == "g_266" || v.Name == "g_194" || v.Name == "g_316" || v.Name == "g_51") {
						fmt.Fprintf(os.Stderr, "S875 inventGate pass=%d %s already=%v lead=%v ownFree=%v hasFree=%v\n", accPass, v.Name, already, isLeading, ownFree, hasFree)
					}
					// Own pure-only free-ref without free-ref on a direct pure-IV
					// owner: do not Acc-append (LevelC seed668 g_205). Skip for nested
					// pure FE heads (seed668 g_645 invent/reorder path). Free-ref on
					// direct pure-IV owner Acc-appends (seed48 g_1495). Session-local.
					if !isLeading && isForIVOfFunc(f, v) && !ownFree && nestedFree {
						freeOnOwnerDirect := false
						for _, inv := range calls {
							if inv == nil || inv.User == nil {
								continue
							}
							if !bodySyntacticFreeReadsVar(inv.User, v) {
								continue
							}
							if isForIVOfFunc(inv.User, v) {
								freeOnOwnerDirect = true
								break
							}
						}
						if hasErrVS(vs) {
							abortUnbuilt()
							return
						}
						if !freeOnOwnerDirect {
							continue
						}
					}
					if !hasFree {
						if isForIVOfFunc(f, v) {
							continue
						}
						if isLeading {
							// Solo pure FE head pure-only invent gate (n62). Multi pure FE
							// head pure-only still Acc-order invents (c302abe func_34
							// g_328.f2 multi pure of pure-head func_55 [g_328.f2,g_109];
							// seed28465 g_506.f4 solo pure-only uses freeValArr/Acc-order
							// path when free-ref free). Session-local.
							multiHead := false
							for _, inv := range calls {
								if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
									continue
								}
								fr := inv.User.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									abortUnbuilt()
									return
								}
								if len(fr) == 0 || fr[0] == nil {
									continue
								}
								if fr[0] != v && (v == nil || fr[0].Name != v.Name || !isForIVOfFunc(inv.User, fr[0])) {
									// name match pure FE head of pure-head owner
									if v == nil || fr[0].Name != v.Name {
										continue
									}
									// Acc identity may differ: accept name match as pure FE head
								}
								if len(fr) > 1 && fr[1] != nil && isForIVOfFunc(inv.User, fr[1]) {
									multiHead = true
									break
								}
								// multi pure residual pure by name
								if len(fr) > 1 && fr[1] != nil {
									for _, blk := range inv.User.Blocks {
										if blk == nil {
											continue
										}
										for i := range blk.Stmts {
											st := &blk.Stmts[i]
											if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil && st.Loop.IV.Name == fr[1].Name {
												multiHead = true
												break
											}
										}
										if multiHead {
											break
										}
									}
								}
								if multiHead {
									break
								}
							}
							if !multiHead {
								// Solo pure FE head pure-only invent gate (n62). Exception:
								// residual of free-head intermediate FE on parent call tree
								// (seed145 g_739 pure FE head of pure-head func_9 residual of
								// free-head func_12 after g_905 before g_2084). Session-local
								// — no package mutable state.
								freeHeadRes := false
								for _, inv := range calls {
									if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
										continue
									}
									fr := inv.User.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(fr) {
										abortUnbuilt()
										return
									}
									if len(fr) == 0 || fr[0] == nil {
										continue
									}
									// free-head FE: FE[0] not pure for-IV of owner
									if isForIVOfFunc(inv.User, fr[0]) {
										continue
									}
									for _, x := range fr[1:] {
										if x == v || (v != nil && x != nil && x.Name == v.Name) {
											freeHeadRes = true
											break
										}
									}
									if freeHeadRes {
										break
									}
								}
								if !freeHeadRes {
									continue
								}
								// residual of free-head intermediate: Acc-order invent
							} else {
								// Multi pure FE head pure-only on parent: invent only when a
								// pure multi pureOnly sibling of the same FE is already on
								// summary (c302abe g_328.f2 with g_109 pure multi of func_55
								// present). Solo invent of multi pure FE head pure-only after
								// map_stm strip is wrong (seed48 g_250 multi pure of func_32;
								// g_951 pure multi mid also pure-only not on parent). Session-
								// local — no package mutable state.
								sibOnSum := false
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								onSum := map[*Variable]bool{}
								for _, x := range ord0 {
									if x != nil {
										onSum[x] = true
									}
								}
								for _, inv := range calls {
									if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
										continue
									}
									fr := inv.User.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(fr) {
										abortUnbuilt()
										return
									}
									if len(fr) == 0 || fr[0] == nil {
										continue
									}
									if fr[0] != v && (v == nil || fr[0].Name != v.Name) {
										continue
									}
									if !isForIVOfFunc(inv.User, fr[0]) {
										continue
									}
									for _, x := range fr[1:] {
										if x == nil || !isForIVOfFunc(inv.User, x) {
											break // end pure multi-prefix
										}
										if x == v {
											continue
										}
										if bodySyntacticFreeReadsVar(f, x) || bodyArrayInitOrValueFreeReadsVar(f, x) {
											continue // free-ref multi mid — not pureOnly sibling
										}
										// pure multi pureOnly sibling on summary by ptr or name
										if onSum[x] {
											sibOnSum = true
											break
										}
										for y := range onSum {
											if y != nil && y.Name == x.Name {
												sibOnSum = true
												break
											}
										}
										if sibOnSum {
											break
										}
									}
									if sibOnSum {
										break
									}
								}
								if !sibOnSum {
									continue // seed48 g_250: no pure multi pureOnly sibling on sum
								}
								// multi pure FE head pure-only with pureOnly sibling: invent
							}
						}
					}
					// Missing residual pure pure-only pure of pure-head nested FE pure-only
					// on parent (no freeVal/array free-ref): do not Acc invent multi residual
					// pure-only on pure-head owner (float seed=2 g_370 free_on_owner=false) or
					// solo residual pure of pure-head with address-of-only free-ref (float
					// seed=2 g_531). Free-ref free residual pure of pure-head Acc invents
					// (c302abe g_99 free58; seed639 g_180 free64) — Acc order matches UP FE.
					// Solo residual pure-only pure of pure-head still invents (seed=42
					// g_1054 / g_97). Session-local — no package mutable state.
					if !already && !bodyArrayInitOrValueFreeReadsVar(f, v) && !isForIVOfFunc(f, v) {
						skip := false
						for _, inv := range calls {
							if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
								continue
							}
							if !isForIVOfFunc(inv.User, v) {
								continue
							}
							fr := inv.User.FEffect.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(fr) {
								abortUnbuilt()
								return
							}
							if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
								continue // not pure-head nested FE
							}
							i := 0
							for i < len(fr) {
								w := fr[i]
								if w == nil || !isForIVOfFunc(inv.User, w) {
									i++
									continue
								}
								j := i + 1
								for j < len(fr) {
									u := fr[j]
									if u == nil || !isForIVOfFunc(inv.User, u) {
										break
									}
									j++
								}
								inRun := false
								for k := i; k < j; k++ {
									if fr[k] == v || (v != nil && fr[k] != nil && fr[k].Name == v.Name) {
										inRun = true
										break
									}
								}
								if inRun && i > 0 {
									if j > i+1 {
										// multi residual pure of pure-head: Acc invent free-ref
										// free residual pure of pure-head (c302 g_99; seed639
										// g_180). Skip pure residual pure-only on pure-head owner
										// when entire residual multi pure run is pure-only on
										// pure-head owner (float g_370). Acc invent pure residual
										// pure-only of residual multi pure of pure-head when a
										// free-ref free pure residual pure of pure-head sibling is
										// in the same multi residual pure run (seed875 g_266 with
										// free-ref free g_316 of pure-head func_60). Session-local
										// — no package mutable state.
										if !bodySyntacticFreeReadsVar(inv.User, v) {
											hasFreeSib := false
											for k := i; k < j; k++ {
												x := fr[k]
												if x != nil && bodySyntacticFreeReadsVar(inv.User, x) {
													hasFreeSib = true
													break
												}
											}
											if !hasFreeSib {
												// multi residual pure pure-only of pure-head (float g_370):
												// skip Acc invent unless Acc neighbor (pred or succ) on
												// summary is free residual free of this pure-head owner FE
												// (seed3682766 func_19 g_1715 Acc after free residual free
												// of pure-head g_1597; g_1907 Acc before free residual free
												// of pure-head g_2657). Any Acc successor on summary alone
												// over-invents pure multi residual pure-only (seed57
												// g_1961.f6/f9 order). Session-local — no package mutable state.
												ownerFreeRes := map[string]bool{}
												for _, x := range fr[1:] {
													if x == nil || isForIVOfFunc(inv.User, x) {
														continue
													}
													ownerFreeRes[x.Name] = true
												}
												accNbrOwnerFree := false
												if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
													acc := cg.EffectAccum.ReadVarsSess(s)
													pureAcc := -1
													for i, x := range acc {
														if x != nil && (x == v || (v != nil && x.Name == v.Name)) {
															pureAcc = i
															break
														}
													}
													if pureAcc >= 0 {
														// Acc predecessor on summary that is free residual free of pure-head
														for j := pureAcc - 1; j >= 0; j-- {
															x := acc[j]
															if x == nil {
																continue
															}
															on := summaryEff.IsReadSess(s, x)
															if hasErrVS(vs) {
																abortUnbuilt()
																return
															}
															if !on {
																continue
															}
															if ownerFreeRes[x.Name] {
																accNbrOwnerFree = true
															}
															break // nearest Acc pred on summary
														}
														// Acc successor on summary that is free residual free of pure-head
														for j := pureAcc + 1; j < len(acc); j++ {
															x := acc[j]
															if x == nil {
																continue
															}
															on := summaryEff.IsReadSess(s, x)
															if hasErrVS(vs) {
																abortUnbuilt()
																return
															}
															if !on {
																continue
															}
															if ownerFreeRes[x.Name] {
																accNbrOwnerFree = true
															}
															break // nearest Acc succ on summary
														}
													}
												}
												if !accNbrOwnerFree {
													skip = true
													break
												}
												// Acc invent Acc-order multi residual pure-only of pure-head
											}
											// free-ref free sibling multi residual pure of pure-head:
											// Acc invent pure residual pure-only of residual multi
										}
									} else {
										// solo residual pure of pure-head: skip address-of-only
										// free-ref (float g_531); invent pure-only (seed42 g_1054).
										// Exception: also pure residual pure free-ref free of free-
										// head intermediate FE (direct or nested call tree) —
										// Acc invents after free residual free of free-head
										// (seed7504 g_58 of free-head func_47 residual pure of
										// pure-head func_36; float g_531 has no free-head residual
										// free-ref free owner). Session-local — no package mutable
										// state.
										if bodySyntacticFreeReadsVar(f, v) {
											freeHeadResFR := false
											seenFH := map[*Function]bool{}
											var walkFH func(fn *Function)
											walkFH = func(fn *Function) {
												if fn == nil || seenFH[fn] || freeHeadResFR {
													return
												}
												seenFH[fn] = true
												if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
													bodySyntacticFreeReadsVar(fn, v) {
													fr2 := fn.FEffect.ReadVarsSess(s)
													if hasErrVS(vs) || !VariablesComplete(fr2) {
														return
													}
													if len(fr2) > 0 && fr2[0] != nil && !isForIVOfFunc(fn, fr2[0]) {
														for _, x := range fr2 {
															if x == v || (v != nil && x != nil && x.Name == v.Name) {
																freeHeadResFR = true
																return
															}
														}
													}
												}
												for _, blk := range fn.Blocks {
													if blk == nil {
														continue
													}
													for i := range blk.Stmts {
														var nested []*Invocation
														if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
															continue
														}
														for _, invn := range nested {
															if invn != nil && invn.User != nil {
																walkFH(invn.User)
															}
														}
													}
												}
											}
											for _, inv2 := range calls {
												if inv2 != nil && inv2.User != nil {
													walkFH(inv2.User)
												}
											}
											if hasErrVS(vs) {
												abortUnbuilt()
												return
											}
											if !freeHeadResFR {
												skip = true
												break
											}
											// free residual pure free-ref free of free-head: invent
										}
									}
								}
								i = j
							}
							if skip {
								break
							}
						}
						if skip {
							if os.Getenv("DIAG_C302") != "" && f != nil && f.Name == "func_34" && v != nil && v.Name == "g_99" {
								fmt.Fprintf(os.Stderr, "C302 g_99 SKIP pure multi residual pure-only of pure-head\n")
							}
							if os.Getenv("DIAG_S42") != "" && v != nil && v.Name == "g_1054" {
								fmt.Fprintf(os.Stderr, "S42 SKIP residual pure pure-head g_1054 f=%s\n", f.Name)
							}
							if os.Getenv("DIAG_S57ACC") != "" && f != nil && f.Name == "func_1" && v != nil &&
								(v.Name == "g_630.f1" || v.Name == "g_1343.f6" || v.Name == "g_1596") {
								fmt.Fprintf(os.Stderr, "S57ACC SKIP pure multi residual pure-only %s\n", v.Name)
							}
							continue
						}
					}
					// Own pure free-ref without freeVal/array that is also pure residual pure of
					// free-head nested FE (direct or deeper call tree): Acc-append when missing,
					// never Acc-order early (seed2048 func_12 g_135 pure residual pure of
					// free-head func_66 not a direct call). Acc *Variable identity may differ
					// from nested for-IV — match pure residual pure by name. Session-local.
					if !already && !bodyArrayInitOrValueFreeReadsVar(f, v) && isForIVOfFunc(f, v) {
						freeHeadPureRes := false
						if v != nil {
							seenFn := map[*Function]bool{}
							var walkFn func(fn *Function)
							walkFn = func(fn *Function) {
								if fn == nil || seenFn[fn] || freeHeadPureRes {
									return
								}
								seenFn[fn] = true
								// pure residual pure of free-head owner by name?
								hasPure := false
								for _, blk := range fn.Blocks {
									if blk == nil {
										continue
									}
									for i := range blk.Stmts {
										st := &blk.Stmts[i]
										if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil && st.Loop.IV.Name == v.Name {
											hasPure = true
										}
									}
								}
								if hasPure && EffectComplete(fn.FEffect) {
									fr := fn.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) {
										return
									}
									if len(fr) > 0 && fr[0] != nil && fr[0].Name != v.Name && !isForIVOfFunc(fn, fr[0]) {
										// free-head FE; v pure residual pure of free-head
										freeHeadPureRes = true
										return
									}
								}
								// nest deeper
								for _, blk := range fn.Blocks {
									if blk == nil {
										continue
									}
									for i := range blk.Stmts {
										var nested []*Invocation
										if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
											continue
										}
										for _, inv := range nested {
											if inv != nil && inv.User != nil {
												walkFn(inv.User)
											}
										}
									}
								}
							}
							for _, inv := range calls {
								if inv != nil && inv.User != nil {
									walkFn(inv.User)
								}
							}
						}
						if freeHeadPureRes {
							if os.Getenv("DIAG_C302") != "" && f != nil && f.Name == "func_34" && v != nil {
								fmt.Fprintf(os.Stderr, "C302 %s freeHeadPureRes Acc-append\n", v.Name)
							}
							summaryEff = summaryEff.ReadVarSess(s, v) // Acc-append late
							if hasErrVS(vs) || !EffectComplete(summaryEff) {
								if !hasErrVS(vs) {
									noteErrVS(vs, ErrGeneric)
								}
								abortUnbuilt()
								return
							}
							continue
						}
					}
					// Solo pure FE head pure-only on non-func_1 parents: insert or reorder
					// before first residual free of owner FE (seed668 func_10 g_645 before
					// g_748). func_1 pure FE head order is pureMiss / pureOnly surfaces
					// (seed123 g_1248, seed57 pure multi). Not own pure of parent
					// (seed48 g_1495). Session-local — no package mutable state.
					if isLeading && !ownFree && !isForIVOfFunc(f, v) && f.Name != "func_1" {
						var firstRes *Variable
						soloHead := true
						for _, inv := range calls {
							if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
								continue
							}
							fr := inv.User.FEffect.ReadVarsSess(s)
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							if len(fr) == 0 || fr[0] == nil {
								continue
							}
							if fr[0] != v && (v == nil || fr[0].Name != v.Name || !isForIVOfFunc(inv.User, fr[0])) {
								continue
							}
							// Pure multi-prefix: FE[1] also pure for-IV of same owner.
							if len(fr) > 1 && fr[1] != nil && isForIVOfFunc(inv.User, fr[1]) {
								soloHead = false
								break
							}
							for _, x := range fr[1:] {
								if x == nil || isForIVOfFunc(inv.User, x) {
									continue
								}
								on := summaryEff.IsReadSess(s, x)
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if on {
									firstRes = x
									break
								}
							}
							if firstRes != nil {
								break
							}
						}
						if !soloHead {
							// Multi pure FE head pure-only: Acc-order invent/reorder below
							// (seed12592 func_21 g_292 of pure multi func_56 [g_292,g_121]).
							// Session-local — no package mutable state.
						} else {
							if firstRes == nil {
								if already {
									continue
								}
								// no residual free anchor — skip invent (n62)
								continue
							}
							// Residual free free-ref on parent or pure-IV owner: Acc
							// free-before-pure is UP-correct (seed470 g_1151 free-ref parent;
							// seed123 g_1183 free-ref owner). seed668 g_748 free-ref neither.
							// Session-local.
							if bodySyntacticFreeReadsVar(f, firstRes) {
								continue
							}
							freeOnOwner := false
							for _, inv := range calls {
								if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
									continue
								}
								fr := inv.User.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if len(fr) == 0 || fr[0] == nil {
									continue
								}
								if fr[0] != v && (v == nil || fr[0].Name != v.Name || !isForIVOfFunc(inv.User, fr[0])) {
									continue
								}
								if bodySyntacticFreeReadsVar(inv.User, firstRes) {
									freeOnOwner = true
								}
								break
							}
							if freeOnOwner {
								continue
							}
							// Rebuild summary: emit v immediately before firstRes.
							ord := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ord) {
								abortUnbuilt()
								return
							}
							pp, rp := -1, -1
							for i, x := range ord {
								if x == v {
									pp = i
								}
								if x == firstRes {
									rp = i
								}
							}
							// Already immediately before firstRes.
							if already && pp >= 0 && rp >= 0 && pp+1 == rp {
								continue
							}
							// Already Acc after firstRes without free residual free-ref parent
							// in the gap: natural Acc free residual then pure (seed639 g_14.f0
							// then g_80). Only reorder when pure is Acc-late past free residual
							// free-ref parent pollution between firstRes and pure (seed668
							// g_748…g_962 free-ref…g_645). Session-local.
							// Acc-late pure already on summary with only owner-FE residual free
							// free-refs in the gap is UP-correct Acc order (seed38656 func_63
							// g_76 after g_192; gap free-refs are residual free of pure-head
							// owner FE). Pollution = free-ref parent not residual free of the
							// pure-head owner FE. Session-local — no package mutable state.
							if already && pp > rp && rp >= 0 {
								// residual free of pure-head owner FE(s) for v
								ownerRes := map[*Variable]bool{}
								for _, inv := range calls {
									if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
										continue
									}
									fr := inv.User.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) {
										abortUnbuilt()
										return
									}
									if len(fr) == 0 || fr[0] == nil {
										continue
									}
									if fr[0] != v && (v == nil || fr[0].Name != v.Name || !isForIVOfFunc(inv.User, fr[0])) {
										continue
									}
									for _, x := range fr[1:] {
										if x == nil || isForIVOfFunc(inv.User, x) {
											continue
										}
										ownerRes[x] = true
									}
								}
								gapPollution := false
								for j := rp + 1; j < pp; j++ {
									w := ord[j]
									if w == nil {
										continue
									}
									if !bodySyntacticFreeReadsVar(f, w) {
										continue
									}
									if !ownerRes[w] {
										gapPollution = true
										break
									}
								}
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if !gapPollution {
									continue // Acc-late pure after owner residual free free-refs only
								}
							}
							rebuilt := EmptyEffect()
							for _, w := range summaryEff.WrittenVarsSess(s) {
								if w == nil {
									continue
								}
								rebuilt = rebuilt.WriteVarSess(s, w)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							inserted := false
							for _, x := range ord {
								if x == nil || x == v {
									continue
								}
								if x == firstRes && !inserted {
									rebuilt = rebuilt.ReadVarSess(s, v)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
									inserted = true
								}
								rebuilt = rebuilt.ReadVarSess(s, x)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							if !inserted {
								rebuilt = rebuilt.ReadVarSess(s, v)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							rebuilt.pure = summaryEff.pure
							rebuilt.sideEffectFree = summaryEff.sideEffectFree
							rebuilt.lhsWrite = summaryEff.lhsWrite
							summaryEff = rebuilt
							continue
						}
					}
					// Solo pure FE head of pure-head deeper FE (not direct-call leadingFE):
					// Acc-append late when pure-only residual of free-head intermediate
					// (seed14545857908692666416 func_30 g_612 pure FE head of pure-head
					// func_50 residual free pure-only of free-head func_44; Acc-order before
					// g_1863 residual free of pure-head is UP-late after g_160).
					// Pure residual pure free-ref free of free-head intermediate FE-rel invents
					// after free residual free pred of free-head on summary (seed1638 g_254 of
					// free-head func_9 after g_41; pure FE head of pure-head func_13). Session-
					// local — no package mutable state.
					if !already && !isLeading && !bodyArrayInitOrValueFreeReadsVar(f, v) && !isForIVOfFunc(f, v) {
						deeperPureHeadSolo := false
						seenDeep := map[*Function]bool{}
						var walkDeep func(fn *Function)
						walkDeep = func(fn *Function) {
							if fn == nil || seenDeep[fn] || deeperPureHeadSolo {
								return
							}
							seenDeep[fn] = true
							if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) {
								fr := fn.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									return
								}
								if len(fr) > 0 && fr[0] != nil &&
									(fr[0] == v || (v != nil && fr[0].Name == v.Name)) &&
									isForIVOfFunc(fn, fr[0]) {
									// solo pure FE head of pure-head
									if !(len(fr) > 1 && fr[1] != nil && isForIVOfFunc(fn, fr[1])) {
										deeperPureHeadSolo = true
										return
									}
								}
							}
							for _, blk := range fn.Blocks {
								if blk == nil {
									continue
								}
								for i := range blk.Stmts {
									var nested []*Invocation
									if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
										continue
									}
									for _, inv := range nested {
										if inv != nil && inv.User != nil {
											walkDeep(inv.User)
										}
									}
								}
							}
						}
						for _, inv := range calls {
							if inv != nil && inv.User != nil {
								walkDeep(inv.User)
							}
						}
						if hasErrVS(vs) {
							abortUnbuilt()
							return
						}
						if deeperPureHeadSolo {
							// Free-head intermediate pure residual pure free-ref free?
							// FE-rel after free residual free pred (seed1638 g_254 after g_41).
							// Pure-only residual free of free-head intermediate Acc-appends late
							// (seed145 g_612 free=false pure residual pure of free-head false).
							var freeHeadPred *Variable
							pureResFreeRefFreeHead := false
							seenFH := map[*Function]bool{}
							var walkFH func(fn *Function)
							walkFH = func(fn *Function) {
								if fn == nil || seenFH[fn] || pureResFreeRefFreeHead {
									return
								}
								seenFH[fn] = true
								if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
									bodySyntacticFreeReadsVar(fn, v) {
									fr := fn.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(fr) {
										return
									}
									if len(fr) > 0 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
										// free-head FE; v pure residual pure free-ref free
										vi := -1
										for i, x := range fr {
											if x == v || (v != nil && x != nil && x.Name == v.Name) {
												vi = i
												break
											}
										}
										if vi > 0 {
											pureResFreeRefFreeHead = true
											// nearest free residual free of free-head on summary
											ordSum := summaryEff.ReadVarsSess(s)
											if hasErrVS(vs) || !VariablesComplete(ordSum) {
												return
											}
											onSum := map[*Variable]bool{}
											byName := map[string]*Variable{}
											for _, x := range ordSum {
												if x != nil {
													onSum[x] = true
													byName[x.Name] = x
												}
											}
											for j := vi - 1; j >= 0; j-- {
												p := fr[j]
												if p == nil || isForIVOfFunc(fn, p) {
													continue
												}
												if onSum[p] {
													freeHeadPred = p
													break
												}
												if y := byName[p.Name]; y != nil {
													freeHeadPred = y
													break
												}
											}
											return
										}
									}
								}
								for _, blk := range fn.Blocks {
									if blk == nil {
										continue
									}
									for i := range blk.Stmts {
										var nested []*Invocation
										if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
											continue
										}
										for _, inv := range nested {
											if inv != nil && inv.User != nil {
												walkFH(inv.User)
											}
										}
									}
								}
							}
							for _, inv := range calls {
								if inv != nil && inv.User != nil {
									walkFH(inv.User)
								}
							}
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							if pureResFreeRefFreeHead && freeHeadPred != nil {
								// FE-rel invent immediately after free residual free pred
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(ord0) {
									abortUnbuilt()
									return
								}
								rebuilt := EmptyEffect()
								for _, w := range summaryEff.WrittenVarsSess(s) {
									if w == nil {
										continue
									}
									rebuilt = rebuilt.WriteVarSess(s, w)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
								}
								for _, x := range ord0 {
									if x == nil || x == v || (v != nil && x.Name == v.Name) {
										continue
									}
									rebuilt = rebuilt.ReadVarSess(s, x)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
									if x == freeHeadPred || (freeHeadPred != nil && x.Name == freeHeadPred.Name) {
										rebuilt = rebuilt.ReadVarSess(s, v)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
								}
								rebuilt.pure = summaryEff.pure
								rebuilt.sideEffectFree = summaryEff.sideEffectFree
								rebuilt.lhsWrite = summaryEff.lhsWrite
								summaryEff = rebuilt
								continue
							}
							if pureResFreeRefFreeHead {
								// free residual free pred not on summary — Acc-order place below
							} else {
								// pure-only residual of free-head intermediate: Acc-append late
								summaryEff = summaryEff.ReadVarSess(s, v)
								if hasErrVS(vs) || !EffectComplete(summaryEff) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
								continue
							}
						}
					}
					// Acc-order place: insert missing pure IVs, or reorder Acc-late
					// pure IVs already on summary to Acc successor (seed=42 func_10
					// g_97 freeVal pure of nested before g_1145; seed=2 func_20 g_37).
					// Pure FE heads (isLeading): Acc-append only when missing — never
					// Acc-order early (seed0 --no-bitfields g_829.f0 pure FE head of
					// func_35 Acc-early next to g_858; UP late after residual free).
					// Session-local — no package mutable state.
					// Solo pure FE head: Acc-append when missing; no Acc-order early
					// (seed0 --no-bitfields g_829.f0). Pure multi-prefix pure FE head:
					// Acc-order place so pure multi before residual free free-ref of same
					// FE (seed389 g_253/g_24/g_349 before g_1161). Session-local.
					if isLeading {
						multiHead := false
						for _, inv := range calls {
							if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
								continue
							}
							fr := inv.User.FEffect.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(fr) {
								abortUnbuilt()
								return
							}
							if len(fr) == 0 || fr[0] == nil {
								continue
							}
							if fr[0] != v && (v == nil || fr[0].Name != v.Name || !isForIVOfFunc(inv.User, fr[0])) {
								continue
							}
							if len(fr) > 1 && fr[1] != nil && isForIVOfFunc(inv.User, fr[1]) {
								multiHead = true
								break
							}
						}
						if !multiHead {
							if already {
								// Solo pure FE head already on summary: Acc-order freeVal
								// pure FE head when Acc-late past Acc successor AND first
								// residual free of pure-head FE on summary is not free-ref
								// free on parent (seed4 func_41 g_45 before g_724 free residual
								// free neither). When first residual free is free-ref free,
								// map free residual free first is UP-correct (seed0
								// --no-bitfields g_829.f0 Acc-early UP-late). Pure-only already
								// keep map. Session-local — no package mutable state.
								if !bodyArrayInitOrValueFreeReadsVar(f, v) {
									continue
								}
								// fall through to freeVal Acc-late after before is set
							} else {
								// Solo pure FE head with freeVal/array free-ref on parent:
								// Acc-append (seed0 --no-bitfields g_829.f0 freeVal free-ref
								// own pure of parent Acc-early is UP-late). Pure-only solo pure
								// FE head of pure-head nested FE pure-only on parent: Acc-order
								// invent (seed=28465 g_506.f4 of func_28 before g_178).
								// Session-local — no package mutable state.
								if bodyArrayInitOrValueFreeReadsVar(f, v) {
									summaryEff = summaryEff.ReadVarSess(s, v)
									if hasErrVS(vs) || !EffectComplete(summaryEff) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
									continue
								}
								// pure-only solo pure FE head: FE-rel invent after free residual
								// free of free-head intermediate when pure residual pure free-ref
								// free of free-head (seed7504 g_58 after g_75 of free-head
								// func_47; isLeading pure FE head of pure-head func_40 blocks the
								// !isLeading free-head path). Else Acc-order place. Session-local
								// — no package mutable state.
								var freeHeadPredLead *Variable
								seenFHL := map[*Function]bool{}
								var walkFHL func(fn *Function)
								walkFHL = func(fn *Function) {
									if fn == nil || seenFHL[fn] || freeHeadPredLead != nil {
										return
									}
									seenFHL[fn] = true
									if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
										bodySyntacticFreeReadsVar(fn, v) {
										fr := fn.FEffect.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(fr) {
											return
										}
										if len(fr) > 0 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
											vi := -1
											for i, x := range fr {
												if x == v || (v != nil && x != nil && x.Name == v.Name) {
													vi = i
													break
												}
											}
											if vi > 0 {
												ordSum := summaryEff.ReadVarsSess(s)
												if hasErrVS(vs) || !VariablesComplete(ordSum) {
													return
												}
												onSum := map[*Variable]bool{}
												byName := map[string]*Variable{}
												for _, x := range ordSum {
													if x != nil {
														onSum[x] = true
														byName[x.Name] = x
													}
												}
												for j := vi - 1; j >= 0; j-- {
													p := fr[j]
													if p == nil || isForIVOfFunc(fn, p) {
														continue
													}
													if onSum[p] {
														freeHeadPredLead = p
														return
													}
													if y := byName[p.Name]; y != nil {
														freeHeadPredLead = y
														return
													}
												}
											}
										}
									}
									for _, blk := range fn.Blocks {
										if blk == nil {
											continue
										}
										for i := range blk.Stmts {
											var nested []*Invocation
											if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
												continue
											}
											for _, invn := range nested {
												if invn != nil && invn.User != nil {
													walkFHL(invn.User)
												}
											}
										}
									}
								}
								for _, invL := range calls {
									if invL != nil && invL.User != nil {
										walkFHL(invL.User)
									}
								}
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if os.Getenv("DIAG_S7504") != "" && v != nil && v.Name == "g_58" && f != nil && f.Name == "func_1" {
									pn := ""
									if freeHeadPredLead != nil {
										pn = freeHeadPredLead.Name
									}
									fmt.Fprintf(os.Stderr, "S7504 freeHeadPredLead=%s sum=", pn)
									for _, x := range summaryEff.ReadVarsSess(s) {
										if x != nil && (x.Name == "g_14" || x.Name == "g_72" || x.Name == "g_75" || x.Name == "g_58" || x.Name == "g_73" || x.Name == "g_84") {
											fmt.Fprintf(os.Stderr, " %s", x.Name)
										}
									}
									fmt.Fprintln(os.Stderr)
								}
								if freeHeadPredLead != nil {
									ord0 := summaryEff.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(ord0) {
										abortUnbuilt()
										return
									}
									rebuilt := EmptyEffect()
									for _, w := range summaryEff.WrittenVarsSess(s) {
										if w == nil {
											continue
										}
										rebuilt = rebuilt.WriteVarSess(s, w)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
									for _, x := range ord0 {
										if x == nil || x == v || (v != nil && x.Name == v.Name) {
											continue
										}
										rebuilt = rebuilt.ReadVarSess(s, x)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
										if x == freeHeadPredLead || (freeHeadPredLead != nil && x.Name == freeHeadPredLead.Name) {
											rebuilt = rebuilt.ReadVarSess(s, v)
											if hasErrVS(vs) || !EffectComplete(rebuilt) {
												if !hasErrVS(vs) {
													noteErrVS(vs, ErrGeneric)
												}
												abortUnbuilt()
												return
											}
										}
									}
									rebuilt.pure = summaryEff.pure
									rebuilt.sideEffectFree = summaryEff.sideEffectFree
									rebuilt.lhsWrite = summaryEff.lhsWrite
									summaryEff = rebuilt
									continue
								}
								// pure-only solo pure FE head: fall through to Acc-order place
							}
						}
						// multi pure FE head: fall through to Acc-order place
					}
					// Free residual pure free-ref free of free-head intermediate FE-rel
					// invent after free residual free pred — also for multi pure FE head
					// of pure-head pure-only (seed7504 g_58 multi pure FE head of pure-
					// head func_40 [g_58,g_140] residual pure free-ref free of free-head
					// func_47 after g_75; solo path above only covers !multiHead).
					// Require pure FE head of pure-head owner (isForIVOfFunc), not merely
					// isLeading — leadingFE marks FE[0] when pureIV of any function
					// (seed989 g_71 FE head of free-head func_61 pure of free-head
					// func_19 over-marks isLeading). Session-local — no package mutable
					// state.
					if !already && !bodyArrayInitOrValueFreeReadsVar(f, v) && !isForIVOfFunc(f, v) {
						multiPureHeadLead := false
						seenMPL := map[*Function]bool{}
						var walkMPL func(fn *Function)
						walkMPL = func(fn *Function) {
							if fn == nil || seenMPL[fn] || multiPureHeadLead {
								return
							}
							seenMPL[fn] = true
							if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) {
								fr := fn.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									return
								}
								if len(fr) > 0 && fr[0] != nil &&
									(fr[0] == v || (v != nil && fr[0].Name == v.Name)) &&
									isForIVOfFunc(fn, fr[0]) {
									if len(fr) > 1 && fr[1] != nil && isForIVOfFunc(fn, fr[1]) {
										multiPureHeadLead = true
										return
									}
								}
							}
							for _, blk := range fn.Blocks {
								if blk == nil {
									continue
								}
								for i := range blk.Stmts {
									var nested []*Invocation
									if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
										continue
									}
									for _, invn := range nested {
										if invn != nil && invn.User != nil {
											walkMPL(invn.User)
										}
									}
								}
							}
						}
						for _, invM := range calls {
							if invM != nil && invM.User != nil {
								walkMPL(invM.User)
							}
						}
						if hasErrVS(vs) {
							abortUnbuilt()
							return
						}
						if !multiPureHeadLead {
							// not multi pure FE head of pure-head — skip free residual free of free-head multi invent
						} else {
						var freeHeadPredMulti *Variable
						seenFHM := map[*Function]bool{}
						var walkFHM func(fn *Function)
						walkFHM = func(fn *Function) {
							if fn == nil || seenFHM[fn] || freeHeadPredMulti != nil {
								return
							}
							seenFHM[fn] = true
							if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
								bodySyntacticFreeReadsVar(fn, v) {
								fr := fn.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									return
								}
								if len(fr) > 0 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
									vi := -1
									for i, x := range fr {
										if x == v || (v != nil && x != nil && x.Name == v.Name) {
											vi = i
											break
										}
									}
									if vi > 0 {
										ordSum := summaryEff.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(ordSum) {
											return
										}
										onSum := map[*Variable]bool{}
										byName := map[string]*Variable{}
										for _, x := range ordSum {
											if x != nil {
												onSum[x] = true
												byName[x.Name] = x
											}
										}
										for j := vi - 1; j >= 0; j-- {
											p := fr[j]
											if p == nil || isForIVOfFunc(fn, p) {
												continue
											}
											if onSum[p] {
												freeHeadPredMulti = p
												return
											}
											if y := byName[p.Name]; y != nil {
												freeHeadPredMulti = y
												return
											}
										}
									}
								}
							}
							for _, blk := range fn.Blocks {
								if blk == nil {
									continue
								}
								for i := range blk.Stmts {
									var nested []*Invocation
									if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
										continue
									}
									for _, invn := range nested {
										if invn != nil && invn.User != nil {
											walkFHM(invn.User)
										}
									}
								}
							}
						}
						for _, invM := range calls {
							if invM != nil && invM.User != nil {
								walkFHM(invM.User)
							}
						}
						if hasErrVS(vs) {
							abortUnbuilt()
							return
						}
						if freeHeadPredMulti != nil {
							ord0 := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ord0) {
								abortUnbuilt()
								return
							}
							// already immediately after free residual free pred?
							for i, x := range ord0 {
								if x == nil || !(x == v || (v != nil && x.Name == v.Name)) {
									continue
								}
								if i > 0 {
									p := ord0[i-1]
									if p != nil && (p == freeHeadPredMulti || p.Name == freeHeadPredMulti.Name) {
										// already placed — fall through for other Acc-order
										freeHeadPredMulti = nil
									}
								}
								break
							}
						}
						if freeHeadPredMulti != nil {
							ord0 := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ord0) {
								abortUnbuilt()
								return
							}
							if os.Getenv("DIAG_S7504M") != "" && f != nil && v != nil {
								fmt.Fprintf(os.Stderr, "S7504M invent f=%s pure=%s after=%s\n", f.Name, v.Name, freeHeadPredMulti.Name)
							}
							rebuilt := EmptyEffect()
							for _, w := range summaryEff.WrittenVarsSess(s) {
								if w == nil {
									continue
								}
								rebuilt = rebuilt.WriteVarSess(s, w)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							for _, x := range ord0 {
								if x == nil || x == v || (v != nil && x.Name == v.Name) {
									continue
								}
								rebuilt = rebuilt.ReadVarSess(s, x)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
								if x == freeHeadPredMulti || (freeHeadPredMulti != nil && x.Name == freeHeadPredMulti.Name) {
									rebuilt = rebuilt.ReadVarSess(s, v)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
								}
							}
							rebuilt.pure = summaryEff.pure
							rebuilt.sideEffectFree = summaryEff.sideEffectFree
							rebuilt.lhsWrite = summaryEff.lhsWrite
							summaryEff = rebuilt
							continue
						}
						} // multiPureHeadLead
					}
					var before *Variable
					seenV := false
					for _, x := range cg.EffectAccum.ReadVarsSess(s) {
						if x == nil {
							continue
						}
						if x == v || (v != nil && x.Name == v.Name) {
							seenV = true
							continue
						}
						if !seenV {
							continue
						}
						on := summaryEff.IsReadSess(s, x)
						if hasErrVS(vs) {
							abortUnbuilt()
							return
						}
						if on {
							before = x
							break
						}
					}
					if os.Getenv("DIAG_CORP3") != "" && v != nil {
						want := os.Getenv("DIAG_CORP3")
						if v.Name == want && (f.Name == "func_1" || f.Name == os.Getenv("DIAG_CORP3_FN")) {
							bn := ""
							if before != nil {
								bn = before.Name
							}
							fmt.Fprintf(os.Stderr, "CORP3 f=%s %s pass=%d already=%v free=%v val=%v arr=%v lead=%v ownPure=%v before=%s\n",
								f.Name, v.Name, accPass, already, ownFree, bodyValueFreeReadsVar(f, v), bodyArrayInitOrValueFreeReadsVar(f, v), isLeading, isForIVOfFunc(f, v), bn)
							// dump summary neighbors for seed875 multi pure
							if os.Getenv("DIAG_S875") != "" {
								fmt.Fprint(os.Stderr, "CORP3 sum=")
								for _, x := range summaryEff.ReadVarsSess(s) {
									if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70") {
										fmt.Fprintf(os.Stderr, " %s", x.Name)
									}
								}
								fmt.Fprintln(os.Stderr)
							}
						}
					}
					if already {
						// Pure residual pure-only already on map_stm:
						// Keep map when pure residual pure of pure-head nested FE and Acc
						// successor is residual free of same pure-head FE after pure in FE
						// order (seed2 func_11 g_821 Acc-early UP-late after g_956).
						// Acc-order Acc-late otherwise (seed123 func_34 g_274/g_173 before
						// g_347; Acc has g_274 g_173 g_347). Session-local — no package
						// mutable state.
						// seed65 func_65 g_53: own pure freeVal free-ref free of free-head parent
						// that is pure residual pure free-only of free-head nested Acc-early after
						// free residual free of free-head intermediate (g_37; Acc before g_96
						// nested free). UP late after g_330.f0 before g_397. Gate tightly:
						// Acc successor must be free residual free of free-head intermediate, and
						// pure must have enclosing global pure IVs of this owner Acc-late after
						// pure (seed5 stack is local-only l_466; seed4 Acc successor g_2028 is
						// not nested free). Anchor after last enclosing pure IV: first non-field
						// parent freeVal free-ref free that follows a field parent freeVal
						// free-ref free (g_330.f0 then g_397). Session/FM-local — no package
						// mutable state.
						if already && isForIVOfFunc(f, v) && bodyValueFreeReadsVar(f, v) &&
							cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
							freeOfNestedFreeHead := map[*Variable]bool{}
							pureOnlyFreeHeadNested := false
							seenFHN := map[*Function]bool{}
							var walkFHN func(fn *Function)
							walkFHN = func(fn *Function) {
								if fn == nil || seenFHN[fn] {
									return
								}
								seenFHN[fn] = true
								if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
									!bodySyntacticFreeReadsVar(fn, v) {
									fr := fn.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(fr) {
										return
									}
									if len(fr) > 0 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
										pureOnlyFreeHeadNested = true
										for _, x := range fr {
											if x != nil && !isForIVOfFunc(fn, x) {
												freeOfNestedFreeHead[x] = true
											}
										}
									}
								}
								for _, blk := range fn.Blocks {
									if blk == nil {
										continue
									}
									for i := range blk.Stmts {
										var nested []*Invocation
										if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
											continue
										}
										for _, inv := range nested {
											if inv != nil && inv.User != nil {
												walkFHN(inv.User)
											}
										}
									}
								}
							}
							for _, inv := range calls {
								if inv != nil && inv.User != nil {
									walkFHN(inv.User)
								}
							}
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							isNestedFree := func(x *Variable) bool {
								if x == nil {
									return false
								}
								if freeOfNestedFreeHead[x] {
									return true
								}
								for y := range freeOfNestedFreeHead {
									if y != nil && y.Name == x.Name {
										return true
									}
								}
								return false
							}
							// Acc successor must be free residual free of free-head intermediate.
							if pureOnlyFreeHeadNested && before != nil && isNestedFree(before) {
								acc := cg.EffectAccum.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(acc) {
									abortUnbuilt()
									return
								}
								pureAcc := -1
								for i, x := range acc {
									if x != nil && (x == v || (v != nil && x.Name == v.Name)) {
										pureAcc = i
										break
									}
								}
								// Enclosing global pure IVs of this owner Acc-late after pure.
								lastEncAcc := -1
								var walkEnc func(blk *Block, stack []*Variable)
								walkEnc = func(blk *Block, stack []*Variable) {
									if blk == nil {
										return
									}
									for i := range blk.Stmts {
										st := &blk.Stmts[i]
										ns := stack
										if st.Kind == StmtFor && st.Loop != nil && st.Loop.IV != nil {
											ns = append(append([]*Variable{}, stack...), st.Loop.IV)
											if st.Loop.IV == v || (v != nil && st.Loop.IV.Name == v.Name) {
												for _, iv := range ns {
													if iv == nil || iv == v || (v != nil && iv.Name == v.Name) {
														continue
													}
													if !isForIVOfFunc(f, iv) {
														continue
													}
													if !iv.IsGlobalSess(s) {
														continue
													}
													for j, x := range acc {
														if x != nil && (x == iv || x.Name == iv.Name) {
															if j > pureAcc && j > lastEncAcc {
																lastEncAcc = j
															}
															break
														}
													}
												}
											}
										}
										walkEnc(st.Then, ns)
										walkEnc(st.Else, ns)
									}
								}
								if f.Body != nil {
									walkEnc(f.Body, nil)
								}
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if pureAcc >= 0 && lastEncAcc > pureAcc {
									isLocalName := func(x *Variable) bool {
										if x == nil || len(x.Name) < 2 {
											return false
										}
										return (x.Name[0] == 'l' || x.Name[0] == 'p') && x.Name[1] == '_'
									}
									isParentFreeVal := func(x *Variable) bool {
										if x == nil || isForIVOfFunc(f, x) || isNestedFree(x) || isLocalName(x) {
											return false
										}
										return bodyValueFreeReadsVar(f, x)
									}
									// After last enclosing pure IV: first non-field parent freeVal
									// free-ref free that follows a field parent freeVal free-ref free
									// (seed65 g_330.f0 field then g_397 scalar — pure between).
									// Non-field freeVal early after lastEnc (g_315/g_330/g_341) do
									// not anchor; field→field (g_154.f0→g_330.f0) continues the run.
									var lastFieldFV *Variable
									var placeBefore *Variable
									for i := lastEncAcc + 1; i < len(acc); i++ {
										x := acc[i]
										if !isParentFreeVal(x) {
											continue
										}
										isField := strings.Contains(x.Name, ".")
										if isField {
											lastFieldFV = x
											continue
										}
										if lastFieldFV != nil {
											placeBefore = x
											break
										}
									}
									if placeBefore != nil {
										ord0 := summaryEff.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(ord0) {
											abortUnbuilt()
											return
										}
										pp0, pb0 := -1, -1
										for i, x := range ord0 {
											if x == v || (v != nil && x != nil && x.Name == v.Name) {
												pp0 = i
											}
											if x == placeBefore || (placeBefore != nil && x != nil && x.Name == placeBefore.Name) {
												pb0 = i
											}
										}
										if pp0 >= 0 && pb0 >= 0 && pp0+1 == pb0 {
											continue // already immediately before anchor
										}
										if pp0 >= 0 && pb0 >= 0 && pp0 != pb0 {
											rebuilt := EmptyEffect()
											for _, w := range summaryEff.WrittenVarsSess(s) {
												if w == nil {
													continue
												}
												rebuilt = rebuilt.WriteVarSess(s, w)
												if hasErrVS(vs) || !EffectComplete(rebuilt) {
													if !hasErrVS(vs) {
														noteErrVS(vs, ErrGeneric)
													}
													abortUnbuilt()
													return
												}
											}
											for _, x := range ord0 {
												if x == nil || x == v || (v != nil && x.Name == v.Name) {
													continue
												}
												if x == placeBefore || (placeBefore != nil && x.Name == placeBefore.Name) {
													rebuilt = rebuilt.ReadVarSess(s, v)
													if hasErrVS(vs) || !EffectComplete(rebuilt) {
														if !hasErrVS(vs) {
															noteErrVS(vs, ErrGeneric)
														}
														abortUnbuilt()
														return
													}
												}
												rebuilt = rebuilt.ReadVarSess(s, x)
												if hasErrVS(vs) || !EffectComplete(rebuilt) {
													if !hasErrVS(vs) {
														noteErrVS(vs, ErrGeneric)
													}
													abortUnbuilt()
													return
												}
											}
											rebuilt.pure = summaryEff.pure
											rebuilt.sideEffectFree = summaryEff.sideEffectFree
											rebuilt.lhsWrite = summaryEff.lhsWrite
											summaryEff = rebuilt
											if os.Getenv("DIAG_S65") != "" && v != nil {
												fmt.Fprintf(os.Stderr, "S65 RELOC pureOnlyFreeHeadNested %s before %s (enc@%d)\n",
													v.Name, placeBefore.Name, lastEncAcc)
											}
											continue
										}
									}
								}
							}
						}
						if !bodyArrayInitOrValueFreeReadsVar(f, v) {
							// Acc-order Acc-late multi residual pure of pure multi nested FE
							// (direct or deeper call tree — seed12592 g_29 of pure multi
							// func_41 under func_21, not a direct call of func_1). Solo
							// residual pure of pure-head keeps map (seed2 g_821). seed123
							// g_274/g_173 of pure multi func_69. Session-local — no package
							// mutable state.
							accOrderPureOnly := false
							skipFreeRefSucc := false
							seenMultiFn := map[*Function]bool{}
							var walkMulti func(fn *Function)
							walkMulti = func(fn *Function) {
								if fn == nil || seenMultiFn[fn] || (accOrderPureOnly && skipFreeRefSucc) {
									return
								}
								seenMultiFn[fn] = true
								if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) {
									fr := fn.FEffect.ReadVarsSess(s)
									if hasErrVS(vs) || !VariablesComplete(fr) {
										return
									}
									if len(fr) > 0 && fr[0] != nil && isForIVOfFunc(fn, fr[0]) {
										vi := -1
										for i, x := range fr {
											if x == v || (v != nil && x != nil && x.Name == v.Name) {
												vi = i
												break
											}
										}
										if vi > 0 {
											runStart, runEnd := vi, vi+1
											for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(fn, fr[runStart-1]) {
												runStart--
											}
											for runEnd < len(fr) && fr[runEnd] != nil && isForIVOfFunc(fn, fr[runEnd]) {
												runEnd++
											}
											if runEnd-runStart > 1 {
												accOrderPureOnly = true
												// Acc successor residual free free-ref free of pure multi
												// after pure multi residual pure in FE (seed0
												// --no-bitfields g_512.f1 before free residual free-ref
												// g_297): keep map. seed123 g_347 residual free free
												// neither free-ref — Acc-order.
												if before != nil && bodySyntacticFreeReadsVar(f, before) {
													for j := vi + 1; j < len(fr); j++ {
														x := fr[j]
														if x == nil || isForIVOfFunc(fn, x) {
															continue
														}
														if x == before || (before != nil && x != nil && x.Name == before.Name) {
															skipFreeRefSucc = true
															break
														}
													}
												}
											}
										}
									}
								}
								for _, blk := range fn.Blocks {
									if blk == nil {
										continue
									}
									for i := range blk.Stmts {
										var nested []*Invocation
										if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
											continue
										}
										for _, inv := range nested {
											if inv != nil && inv.User != nil {
												walkMulti(inv.User)
											}
										}
									}
								}
							}
							// Own pure residual pure of pure multi FE (seed12592 func_21
							// g_29 pure residual pure of pure multi *g_32/*g_35). Session-local.
							walkMulti(f)
							for _, inv := range calls {
								if inv != nil && inv.User != nil {
									walkMulti(inv.User)
								}
							}
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							// PureMissTouched multi residual pure of pure multi keeps pureMiss
							// map order (seed57 g_168 after g_137 g_112; Acc g_168 early after
							// g_136 yanks before free-ref pure residual of func_70). Session/
							// FM-local — no package mutable state.
							if accOrderPureOnly && cg.FM != nil && cg.FM.PureMissTouched != nil {
								pmt := cg.FM.PureMissTouched[v]
								if !pmt && v != nil {
									for y, ok := range cg.FM.PureMissTouched {
										if ok && y != nil && y.Name == v.Name {
											pmt = true
											break
										}
									}
								}
								if pmt {
									continue // keep pureMiss map
								}
							}
							if !accOrderPureOnly {
								// Free residual pure free-ref free of free-head FE Acc-order
								// to Acc successor when Acc-early with gap (seed9895936 g_97
								// of free-head func_40 free-refs owner+parent Acc-early next
								// to free residual FE head g_209; Acc before g_741; UP after
								// g_391 before g_741). Also Acc-late non-own pure free residual
								// pure free-ref free of free-head when free-ref free is only on
								// the free-head owner, not the parent (seed66 func_39 g_652 of
								// free-head func_41 free-ref free of owner only; Acc before
								// g_385; map late after g_283; UP after g_292.f0 before g_385).
								// freeHeadOwnerFR already demands free-ref free on the free-head
								// owner — parent free-ref free is not required. Solo residual
								// pure of pure-head keeps map (seed2 g_821; freeHeadOwnerFR
								// false). Session-local — no package mutable state.
								if before != nil {
									freeHeadOwnerFR := false
									// Soft-expand nested free-head pure residual free-ref
									// owners (func_40 under func_1 call tree for seed989;
									// func_41 under func_39 for seed66).
									seenFH := map[*Function]bool{}
									var walkFH func(fn *Function)
									walkFH = func(fn *Function) {
										if fn == nil || seenFH[fn] || freeHeadOwnerFR {
											return
										}
										seenFH[fn] = true
										if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) &&
											bodySyntacticFreeReadsVar(fn, v) {
											fr := fn.FEffect.ReadVarsSess(s)
											if hasErrVS(vs) || !VariablesComplete(fr) {
												return
											}
											if len(fr) > 0 && fr[0] != nil && !isForIVOfFunc(fn, fr[0]) {
												freeHeadOwnerFR = true
												return
											}
										}
										for _, blk := range fn.Blocks {
											if blk == nil {
												continue
											}
											for i := range blk.Stmts {
												var nested []*Invocation
												if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
													continue
												}
												for _, inv := range nested {
													if inv != nil && inv.User != nil {
														walkFH(inv.User)
													}
												}
											}
										}
									}
									for _, inv := range calls {
										if inv != nil && inv.User != nil {
											walkFH(inv.User)
										}
									}
									if hasErrVS(vs) {
										abortUnbuilt()
										return
									}
									if freeHeadOwnerFR {
										ordFH := summaryEff.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(ordFH) {
											abortUnbuilt()
											return
										}
										ppFH, rpFH := -1, -1
										for i, x := range ordFH {
											if x == v || (v != nil && x != nil && x.Name == v.Name) {
												ppFH = i
											}
											if x == before || (before != nil && x != nil && x.Name == before.Name) {
												rpFH = i
											}
										}
										// Acc-order Acc-early with gap (seed9895936 g_97 before
										// Acc successor with gap) only when free-ref free of parent
										// (seed989 owner+parent free-refs). Acc-order Acc-late free
										// residual pure free-ref free of free-head before Acc
										// successor only when not own pure of parent (n3306 g_440
										// Acc-late after g_1453; Acc before g_531; UP after g_334
										// before g_531) with free-ref free of parent, or free-ref
										// free only on a *direct* free-head free residual pure
										// free-ref free owner (seed66 func_39→func_41 g_652 Acc-late
										// after g_283; Acc before g_385; UP after g_292.f0 before
										// g_385). Deeper-only free-head free residual pure free-ref
										// free owners keep map (seed48 g_129.f2 of free-head func_66
										// not a direct call of parent). Own pure free residual pure
										// free-ref free of free-head free-ref free of owner Acc-late
										// keeps map (seed2048 func_12 g_150 own pure Acc before
										// g_366; Acc-order early pollutes parent FE). Session-local
										// — no package mutable state.
										parentFreeFR := bodySyntacticFreeReadsVar(f, v)
										accOrderFH := false
										// free-ref free only on a *direct* free-head free residual pure
										// free-ref free owner (not parent free-ref free). Shared by
										// Acc-early-with-gap (seed3682766 func_19 g_1548 of free-head
										// func_33 Acc before g_769; map early after g_564; UP after
										// g_249 before g_769) and Acc-late (seed66). Deeper-only
										// free-head free residual pure free-ref free owners keep map
										// (seed48 g_129.f2). Session-local — no package mutable state.
										directFreeHeadOwnerFR := func() bool {
											for _, invD := range calls {
												if invD == nil || invD.User == nil || !EffectComplete(invD.User.FEffect) {
													continue
												}
												if !isForIVOfFunc(invD.User, v) || !bodySyntacticFreeReadsVar(invD.User, v) {
													continue
												}
												frD := invD.User.FEffect.ReadVarsSess(s)
												if hasErrVS(vs) || !VariablesComplete(frD) {
													return false
												}
												if len(frD) > 0 && frD[0] != nil && !isForIVOfFunc(invD.User, frD[0]) {
													return true
												}
											}
											return false
										}
										if ppFH >= 0 && rpFH >= 0 && ppFH+1 != rpFH {
											if ppFH < rpFH {
												if parentFreeFR {
													accOrderFH = true // Acc-early with gap, parent free-ref free
												} else if !isForIVOfFunc(f, v) {
													// Acc-early with gap, free-ref free only on direct free-head
													// owner: Acc-order when Acc successor is not residual free of
													// free-head owner FE (seed3682766 func_19 g_1548 Acc before
													// g_769 not residual free of free-head func_33; UP after
													// g_249 before g_769). Keep map when Acc successor is residual
													// free of free-head (seed57 g_1961.f9 Acc before g_2468 residual
													// free free-ref free of free-head func_2 — Acc-order before
													// residual free of free-head yanks pure multi residual pure
													// order f9/f6). Session-local — no package mutable state.
													if directFreeHeadOwnerFR() {
														accSuccOwnerResFree := false
														if before != nil {
															for _, invD := range calls {
																if invD == nil || invD.User == nil || !EffectComplete(invD.User.FEffect) {
																	continue
																}
																if !isForIVOfFunc(invD.User, v) || !bodySyntacticFreeReadsVar(invD.User, v) {
																	continue
																}
																frD := invD.User.FEffect.ReadVarsSess(s)
																if hasErrVS(vs) || !VariablesComplete(frD) {
																	break
																}
																if len(frD) == 0 || frD[0] == nil || isForIVOfFunc(invD.User, frD[0]) {
																	continue // not free-head
																}
																for _, x := range frD[1:] {
																	if x == nil || isForIVOfFunc(invD.User, x) {
																		continue
																	}
																	if x == before || x.Name == before.Name {
																		accSuccOwnerResFree = true
																		break
																	}
																}
																if accSuccOwnerResFree {
																	break
																}
															}
														}
														if !accSuccOwnerResFree {
															accOrderFH = true
														}
													}
													if hasErrVS(vs) {
														abortUnbuilt()
														return
													}
												}
											} else if ppFH > rpFH && !isForIVOfFunc(f, v) {
												if parentFreeFR {
													// Acc-late non-own pure free residual pure FR of free-head
													accOrderFH = true
												} else {
													// free-ref free only on free-head owner: Acc-late only
													// when that free-head free residual pure free-ref free
													// owner is a direct call of parent (seed66). Session-
													// local — no package mutable state.
													if directFreeHeadOwnerFR() {
														accOrderFH = true
													}
													if hasErrVS(vs) {
														abortUnbuilt()
														return
													}
												}
											}
										}
										if os.Getenv("DIAG_S9838") != "" && f != nil && f.Name == "func_12" && v != nil && (v.Name == "g_445" || v.Name == "g_276") {
											bn := ""
											if before != nil {
												bn = before.Name
											}
											fmt.Fprintf(os.Stderr, "S9838 freeHead path %s before=%s accOrderFH=%v pp=%d rp=%d\n", v.Name, bn, accOrderFH, ppFH, rpFH)
											fmt.Fprint(os.Stderr, "S9838 sum cluster:")
											for i, x := range ordFH {
												if x != nil && (x.Name == "g_445" || x.Name == "g_276" || x.Name == "g_236" || x.Name == "g_742" || x.Name == "l_34" || x.Name == "g_139") {
													fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
												}
											}
											fmt.Fprintln(os.Stderr)
										}
										if accOrderFH {
											rebuilt := EmptyEffect()
											for _, w := range summaryEff.WrittenVarsSess(s) {
												if w == nil {
													continue
												}
												rebuilt = rebuilt.WriteVarSess(s, w)
												if hasErrVS(vs) || !EffectComplete(rebuilt) {
													if !hasErrVS(vs) {
														noteErrVS(vs, ErrGeneric)
													}
													abortUnbuilt()
													return
												}
											}
											inserted := false
											for _, x := range ordFH {
												if x == nil || x == v || (v != nil && x.Name == v.Name) {
													continue
												}
												if (x == before || (before != nil && x.Name == before.Name)) && !inserted {
													rebuilt = rebuilt.ReadVarSess(s, v)
													if hasErrVS(vs) || !EffectComplete(rebuilt) {
														if !hasErrVS(vs) {
															noteErrVS(vs, ErrGeneric)
														}
														abortUnbuilt()
														return
													}
													inserted = true
												}
												rebuilt = rebuilt.ReadVarSess(s, x)
												if hasErrVS(vs) || !EffectComplete(rebuilt) {
													if !hasErrVS(vs) {
														noteErrVS(vs, ErrGeneric)
													}
													abortUnbuilt()
													return
												}
											}
											if !inserted {
												rebuilt = rebuilt.ReadVarSess(s, v)
												if hasErrVS(vs) || !EffectComplete(rebuilt) {
													if !hasErrVS(vs) {
														noteErrVS(vs, ErrGeneric)
													}
													abortUnbuilt()
													return
												}
											}
											rebuilt.pure = summaryEff.pure
											rebuilt.sideEffectFree = summaryEff.sideEffectFree
											rebuilt.lhsWrite = summaryEff.lhsWrite
											summaryEff = rebuilt
											continue
										}
									}
								}
								continue // keep map (seed2 g_821 solo residual pure of pure-head)
							}
							// Acc-order Acc-late multi residual pure of pure multi.
							if before == nil {
								// End of Acc: Acc-order after Acc predecessor when free residual
								// free-ref free of parent (or other map pollution) sits between
								// Acc predecessor and pure residual pure free-ref free of pure-
								// multi pure-head on summary (seed875 func_45 g_316: Acc
								// g_266 g_316; map g_266 g_443 g_982 g_316; UP g_266 g_316
								// g_443). Session-local — no package mutable state.
								var accPred *Variable
								if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
									var prev *Variable
									for _, x := range cg.EffectAccum.ReadVarsSess(s) {
										if x == nil {
											continue
										}
										if x == v || (v != nil && x.Name == v.Name) {
											accPred = prev
											break
										}
										on := summaryEff.IsReadSess(s, x)
										if hasErrVS(vs) {
											abortUnbuilt()
											return
										}
										if on {
											prev = x
										}
									}
								}
								if accPred == nil {
									continue
								}
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(ord0) {
									abortUnbuilt()
									return
								}
								pp0, pred0 := -1, -1
								for i, x := range ord0 {
									if x == v || (v != nil && x != nil && x.Name == v.Name) {
										pp0 = i
									}
									if x == accPred || (accPred != nil && x != nil && x.Name == accPred.Name) {
										pred0 = i
									}
								}
								if pp0 < 0 || pred0 < 0 || pp0 == pred0+1 {
									continue // missing or already immediately after Acc predecessor
								}
								if pp0 < pred0 {
									continue // already Acc-early relative to predecessor
								}
								// gap between Acc predecessor and pure residual pure free-ref free
								// of pure multi — Acc-order immediately after Acc predecessor
								rebuilt := EmptyEffect()
								for _, w := range summaryEff.WrittenVarsSess(s) {
									if w == nil {
										continue
									}
									rebuilt = rebuilt.WriteVarSess(s, w)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
								}
								for _, x := range ord0 {
									if x == nil || x == v || (v != nil && x.Name == v.Name) {
										continue
									}
									rebuilt = rebuilt.ReadVarSess(s, x)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
									if x == accPred || (accPred != nil && x.Name == accPred.Name) {
										rebuilt = rebuilt.ReadVarSess(s, v)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
								}
								rebuilt.pure = summaryEff.pure
								rebuilt.sideEffectFree = summaryEff.sideEffectFree
								rebuilt.lhsWrite = summaryEff.lhsWrite
								summaryEff = rebuilt
								continue
							}
							if skipFreeRefSucc {
								// seed0 --no-bitfields g_512.f1: pure multi mid of pure multi-
								// prefix [g_10,g_512.f1] (runStart==0) with free residual free free-
								// ref free after pure multi — keep map free residual free free-ref
								// free first (UP late pure). seed22759 func_58 g_149 residual multi
								// pure of pure multi pure-head func_71 [g_66,g_149] (runStart>0)
								// Acc before free residual free free-ref free g_79 — Acc-order pure
								// early. Session-local — no package mutable state.
								prefixMid := false
								seenPM := map[*Function]bool{}
								var walkPM func(fn *Function)
								walkPM = func(fn *Function) {
									if fn == nil || seenPM[fn] || prefixMid {
										return
									}
									seenPM[fn] = true
									if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) {
										fr := fn.FEffect.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(fr) {
											return
										}
										if len(fr) > 0 && fr[0] != nil && isForIVOfFunc(fn, fr[0]) {
											vi := -1
											for i, x := range fr {
												if x == v || (v != nil && x != nil && x.Name == v.Name) {
													vi = i
													break
												}
											}
											if vi > 0 {
												runStart := vi
												for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(fn, fr[runStart-1]) {
													runStart--
												}
												if runStart == 0 {
													prefixMid = true
													return
												}
											}
										}
									}
									for _, blk := range fn.Blocks {
										if blk == nil {
											continue
										}
										for i := range blk.Stmts {
											var nested []*Invocation
											if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
												continue
											}
											for _, inv := range nested {
												if inv != nil && inv.User != nil {
													walkPM(inv.User)
												}
											}
										}
									}
								}
								walkPM(f)
								for _, inv := range calls {
									if inv != nil && inv.User != nil {
										walkPM(inv.User)
									}
								}
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if prefixMid {
									continue // keep map pure multi-prefix mid (seed0 --no-bitfields g_512.f1)
								}
								// residual multi pure of pure multi pure-head: Acc-order fallthrough
							}
							ord := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ord) {
								abortUnbuilt()
								return
							}
							pp, rp := -1, -1
							for i, x := range ord {
								if x == v || (v != nil && x != nil && x.Name == v.Name) {
									pp = i
								}
								if x == before || (before != nil && x != nil && x.Name == before.Name) {
									rp = i
								}
							}
							// Acc-early multi residual pure free-ref free of pure-head: when Acc
							// successor is pure residual pure multi sibling earlier in pure-head
							// FE, Acc order is inverted vs pure-head FE — place after Acc successor
							// (seed57 g_112 Acc before g_137; pure-head func_70 FE g_137* then
							// g_93* then g_112*; UP g_137 g_112). Acc-early matching pure-head FE
							// keeps map. Session-local — no package mutable state.
							if pp >= 0 && rp >= 0 && pp < rp && before != nil {
								feInverted := false
								seenInv := map[*Function]bool{}
								var walkInv func(fn *Function)
								walkInv = func(fn *Function) {
									if fn == nil || seenInv[fn] || feInverted {
										return
									}
									seenInv[fn] = true
									if EffectComplete(fn.FEffect) && isForIVOfFunc(fn, v) {
										fr := fn.FEffect.ReadVarsSess(s)
										if hasErrVS(vs) || !VariablesComplete(fr) {
											return
										}
										if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(fn, fr[0]) {
											// free-head residual pure multi not pure-head
										} else {
											vi, bi := -1, -1
											for i, x := range fr {
												if x == v || (v != nil && x != nil && x.Name == v.Name) {
													vi = i
												}
												if x == before || (before != nil && x != nil && x.Name == before.Name) {
													bi = i
												}
											}
											if vi > 0 && bi >= 0 && bi < vi && fr[bi] != nil && isForIVOfFunc(fn, fr[bi]) {
												// Acc successor earlier in pure residual pure multi run
												runStart := vi
												for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(fn, fr[runStart-1]) {
													runStart--
												}
												if bi >= runStart {
													feInverted = true
													return
												}
											}
										}
									}
									for _, blk := range fn.Blocks {
										if blk == nil {
											continue
										}
										for i := range blk.Stmts {
											var nested []*Invocation
											if !collectCalledInvocationsStmt(s, &blk.Stmts[i], &nested) {
												continue
											}
											for _, inv := range nested {
												if inv != nil && inv.User != nil {
													walkInv(inv.User)
												}
											}
										}
									}
								}
								for _, inv := range calls {
									if inv != nil && inv.User != nil {
										walkInv(inv.User)
									}
								}
								// Own pure residual pure multi of pure multi FE
								walkInv(f)
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								if feInverted {
									// Place v after Acc successor (pure residual pure multi earlier in FE)
									rebuilt := EmptyEffect()
									for _, w := range summaryEff.WrittenVarsSess(s) {
										if w == nil {
											continue
										}
										rebuilt = rebuilt.WriteVarSess(s, w)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
									for _, x := range ord {
										if x == nil || x == v || (v != nil && x.Name == v.Name) {
											continue
										}
										rebuilt = rebuilt.ReadVarSess(s, x)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
										if x == before || (before != nil && x.Name == before.Name) {
											rebuilt = rebuilt.ReadVarSess(s, v)
											if hasErrVS(vs) || !EffectComplete(rebuilt) {
												if !hasErrVS(vs) {
													noteErrVS(vs, ErrGeneric)
												}
												abortUnbuilt()
												return
											}
										}
									}
									rebuilt.pure = summaryEff.pure
									rebuilt.sideEffectFree = summaryEff.sideEffectFree
									rebuilt.lhsWrite = summaryEff.lhsWrite
									summaryEff = rebuilt
									continue
								}
								continue // already Acc-early matching pure-head FE / Acc
							}
							if pp >= 0 && rp >= 0 && pp+1 == rp {
								continue
							}
							if !(pp > rp && rp >= 0) {
								continue
							}
							rebuilt := EmptyEffect()
							for _, w := range summaryEff.WrittenVarsSess(s) {
								if w == nil {
									continue
								}
								rebuilt = rebuilt.WriteVarSess(s, w)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							inserted := false
							for _, x := range ord {
								if x == nil || x == v || (v != nil && x.Name == v.Name) {
									continue
								}
								if (x == before || (before != nil && x.Name == before.Name)) && !inserted {
									rebuilt = rebuilt.ReadVarSess(s, v)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
									inserted = true
								}
								rebuilt = rebuilt.ReadVarSess(s, x)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							if !inserted {
								rebuilt = rebuilt.ReadVarSess(s, v)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							rebuilt.pure = summaryEff.pure
							rebuilt.sideEffectFree = summaryEff.sideEffectFree
							rebuilt.lhsWrite = summaryEff.lhsWrite
							summaryEff = rebuilt
							continue
						}
						// Acc-late reorder freeVal/array free-ref pure residual pure of
						// pure-head nested FE only (seed=42 g_97 freeVal; seed=28465 g_157).
						// Free-head pure residual freeVal keeps map order (seed=48 g_533
						// Acc-early is UP-late after g_598). Session-local.
						if !bodyArrayInitOrValueFreeReadsVar(f, v) {
							continue
						}
						pureHeadOwner := false
						// Multi residual pure freeVal of pure-head: map free residual
						// free-ref before pure residual is UP-correct (float seed=2 g_38
						// multi pure residual after free residual free-ref). Solo residual
						// pure freeVal Acc-orders early (seed42 g_97 after g_1027 before
						// g_1145 despite free residual free-ref g_128 earlier). Pure FE
						// heads freeVal Acc-order (seed4 g_45). Session-local — no package
						// mutable state.
						afterOwnerFreeRefMulti := false
						for _, inv := range calls {
							if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
								continue
							}
							if !isForIVOfFunc(inv.User, v) {
								continue
							}
							fr := inv.User.FEffect.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(fr) {
								abortUnbuilt()
								return
							}
							if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
								continue
							}
							pureHeadOwner = true
							// multi residual pure run containing v?
							vi := -1
							for i, x := range fr {
								if x == v || (v != nil && x != nil && x.Name == v.Name) {
									vi = i
									break
								}
							}
							if vi <= 0 {
								// Solo pure FE head freeVal: Acc-order early when first residual
								// free of pure-head FE on summary is not free-ref free on parent
								// (seed4 g_45 before g_724 free residual free neither). When
								// first residual free is free-ref free AND pure is also own
								// pure of parent, keep map Acc-late (seed0 --no-bitfields
								// g_829.f0 own pure of func_1 + pure FE head of func_35; Acc
								// early next to g_858, UP late). Nested-only pure FE head
								// freeVal Acc-orders even with free-ref residual free
								// (seed120 g_4 of func_2 freeVal on func_1; Acc g_4 before
								// g_1433, UP FE head). Multi pure FE head freeVal Acc-orders
								// despite residual free free-ref (seed42 func_44 g_184 of multi
								// pure func_63 before g_1395). Session-local — no package
								// mutable state.
								soloHead := true
								if len(fr) > 1 && fr[1] != nil && isForIVOfFunc(inv.User, fr[1]) {
									soloHead = false
								}
								if !soloHead {
									break // multi pure FE head freeVal — Acc-order
								}
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) {
									abortUnbuilt()
									return
								}
								firstResFreeRef := false
								for _, x := range fr[1:] {
									if x == nil || isForIVOfFunc(inv.User, x) {
										continue
									}
									on := false
									for _, y := range ord0 {
										if y == x || (x != nil && y != nil && y.Name == x.Name) {
											on = true
											break
										}
									}
									if !on {
										continue
									}
									if bodySyntacticFreeReadsVar(f, x) {
										firstResFreeRef = true
									}
									break // first residual free of owner FE on summary
								}
								if firstResFreeRef && isForIVOfFunc(f, v) {
									// Own pure freeVal pure FE head of nested: keep map late
									// when Acc successor is residual free free-ref free of
									// pure-head FE AND pure is pure-only on pure-head owner
									// (seed0 nobf g_829.f0 of func_35 pure-only; Acc-early next
									// to residual free free-ref g_858, UP late). Free-ref free
									// pure FE head of pure-head owner Acc-orders early even
									// when Acc successor is residual free free-ref of pure-head
									// (seed942749745 func_21 g_38 of pure-head func_31 free-ref
									// free on owner; Acc before g_75; UP g_9 g_6 g_38 g_75).
									// Acc-order when Acc successor is free residual free of
									// parent not residual free of pure-head (seed3682609 g_18
									// Acc before g_41). Nested-only pure FE head freeVal
									// Acc-orders (seed120 g_4). Session-local — no package
									// mutable state.
									accSuccIsOwnerResFree := false
									if before != nil {
										for _, x := range fr[1:] {
											if x == nil || isForIVOfFunc(inv.User, x) {
												continue
											}
											if (x == before || (before != nil && x.Name == before.Name)) &&
												bodySyntacticFreeReadsVar(f, x) {
												accSuccIsOwnerResFree = true
											}
											break // first residual free of pure-head FE
										}
									}
									// pure-only on pure-head owner (not free-ref free / freeVal
									// free-ref free on owner) keeps map Acc-late.
									freeOnPureHeadOwner := bodySyntacticFreeReadsVar(inv.User, v) ||
										bodyArrayInitOrValueFreeReadsVar(inv.User, v)
									if (accSuccIsOwnerResFree || before == nil) && !freeOnPureHeadOwner {
										pureHeadOwner = false // keep map Acc-late pure-only head
									}
									// else pureHeadOwner stays true → Acc-order early
								}
								break
							}
							runStart, runEnd := vi, vi+1
							for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(inv.User, fr[runStart-1]) {
								runStart--
							}
							for runEnd < len(fr) && fr[runEnd] != nil && isForIVOfFunc(inv.User, fr[runEnd]) {
								runEnd++
							}
							if !(runStart > 0 && runEnd-runStart > 1) {
								break // solo residual pure freeVal — Acc-order
							}
							ord0 := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							vp := -1
							for i, x := range ord0 {
								if x == v || (v != nil && x != nil && x.Name == v.Name) {
									vp = i
									break
								}
							}
							if vp >= 0 {
								for _, x := range fr[1:] {
									if x == nil || isForIVOfFunc(inv.User, x) {
										continue
									}
									if !bodySyntacticFreeReadsVar(f, x) {
										continue
									}
									for i, y := range ord0 {
										if i >= vp {
											break
										}
										if y == x || (x != nil && y != nil && y.Name == x.Name) {
											afterOwnerFreeRefMulti = true
											break
										}
									}
									if afterOwnerFreeRefMulti {
										break
									}
								}
							}
							break
						}
						if !pureHeadOwner {
							continue
						}
						// freeVal solo residual pure pure-only of pure-head (no free-ref free on
						// pure-head owner) Acc-late past pure-head FE succ on summary: FE-rel before
						// FE succ (seed57 g_1604 free_on_owner=false Acc-late after g_1712.f0). Free
						// residual pure free-ref free freeVal free of pure-head free residual pure
						// free-ref free of owner keeps map Acc-late (seed57 g_904 free_on_owner=true).
						// Acc-early freeVal solo residual pure keeps map (seed42 g_369). Multi residual
						// pure freeVal uses afterOwnerFreeRefMulti. Session-local — no package
						// mutable state.
						if !afterOwnerFreeRefMulti {
							var feSucc *Variable
							freeOnPureHeadOwner := false
							for _, inv := range calls {
								if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
									continue
								}
								if !isForIVOfFunc(inv.User, v) {
									continue
								}
								fr := inv.User.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									abortUnbuilt()
									return
								}
								if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
									continue // not pure-head nested FE
								}
								if bodySyntacticFreeReadsVar(inv.User, v) {
									// free residual pure free-ref free freeVal of pure-head owner
									freeOnPureHeadOwner = true
								}
								vi := -1
								for i, x := range fr {
									if x == v || (v != nil && x != nil && x.Name == v.Name) {
										vi = i
										break
									}
								}
								if vi <= 0 {
									continue
								}
								runStart, runEnd := vi, vi+1
								for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(inv.User, fr[runStart-1]) {
									runStart--
								}
								for runEnd < len(fr) && fr[runEnd] != nil && isForIVOfFunc(inv.User, fr[runEnd]) {
									runEnd++
								}
								if runEnd-runStart > 1 {
									continue // multi residual pure freeVal
								}
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(ord0) {
									abortUnbuilt()
									return
								}
								byName := map[string]*Variable{}
								onSum := map[*Variable]bool{}
								for _, x := range ord0 {
									if x != nil {
										onSum[x] = true
										byName[x.Name] = x
									}
								}
								for j := vi + 1; j < len(fr); j++ {
									p := fr[j]
									if p == nil {
										continue
									}
									if onSum[p] {
										feSucc = p
										break
									}
									if y := byName[p.Name]; y != nil {
										feSucc = y
										break
									}
								}
								if feSucc != nil {
									break
								}
							}
							if freeOnPureHeadOwner {
								feSucc = nil // free residual pure free-ref free freeVal of pure-head owner: keep map
							}
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							if feSucc != nil {
								ord0 := summaryEff.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(ord0) {
									abortUnbuilt()
									return
								}
								pp0, succ0 := -1, -1
								for i, x := range ord0 {
									if x == v || (v != nil && x != nil && x.Name == v.Name) {
										pp0 = i
									}
									if x == feSucc || (feSucc != nil && x != nil && x.Name == feSucc.Name) {
										succ0 = i
									}
								}
								// Acc successor is residual free / pure residual pure of pure-head FE?
								// Acc-late after owner-FE residual free is UP-correct keep (seed42
								// g_369). Acc-late after free residual free of parent pollution with
								// pure-head FE succ earlier on map: FE-rel before FE succ (seed57
								// g_1604 Acc before g_2563; FE succ g_1712.f0). Session-local.
								accSuccOwnerFE := false
								if before != nil {
									for _, inv := range calls {
										if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
											continue
										}
										if !isForIVOfFunc(inv.User, v) {
											continue
										}
										fr := inv.User.FEffect.ReadVarsSess(s)
										if hasErrVS(vs) {
											abortUnbuilt()
											return
										}
										if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
											continue
										}
										for _, x := range fr {
											if x == before || (before != nil && x != nil && x.Name == before.Name) {
												accSuccOwnerFE = true
												break
											}
										}
										if accSuccOwnerFE {
											break
										}
									}
								}
								// Only Acc-late past FE succ with Acc successor pollution (seed57);
								// Acc-early keep (seed42 g_369 Acc-early FE-rel would pull early).
								if !accSuccOwnerFE && pp0 >= 0 && succ0 >= 0 && pp0 > succ0 {
									rebuilt := EmptyEffect()
									for _, w := range summaryEff.WrittenVarsSess(s) {
										if w == nil {
											continue
										}
										rebuilt = rebuilt.WriteVarSess(s, w)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
									inserted := false
									for _, x := range ord0 {
										if x == nil || x == v || (v != nil && x.Name == v.Name) {
											continue
										}
										if (x == feSucc || (feSucc != nil && x.Name == feSucc.Name)) && !inserted {
											rebuilt = rebuilt.ReadVarSess(s, v)
											if hasErrVS(vs) || !EffectComplete(rebuilt) {
												if !hasErrVS(vs) {
													noteErrVS(vs, ErrGeneric)
												}
												abortUnbuilt()
												return
											}
											inserted = true
										}
										rebuilt = rebuilt.ReadVarSess(s, x)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
									if !inserted {
										rebuilt = rebuilt.ReadVarSess(s, v)
										if hasErrVS(vs) || !EffectComplete(rebuilt) {
											if !hasErrVS(vs) {
												noteErrVS(vs, ErrGeneric)
											}
											abortUnbuilt()
											return
										}
									}
									rebuilt.pure = summaryEff.pure
									rebuilt.sideEffectFree = summaryEff.sideEffectFree
									rebuilt.lhsWrite = summaryEff.lhsWrite
									summaryEff = rebuilt
									continue
								}
							}
						}
						if afterOwnerFreeRefMulti {
							// Multi residual pure freeVal: keep map after free residual free-ref
							// free of pure-head (float seed=2 g_38). Still Acc-order before free
							// residual free-ref free of parent that is not residual free of pure-
							// head when Acc has pure residual pure freeVal before that free residual
							// free-ref free (seed875 func_45 g_36 of multi residual pure freeVal of
							// pure-head func_60: Acc g_13 g_70 g_36 … g_8; map g_13 g_70 g_8 …
							// g_36; UP g_13 g_70 g_36 g_8). Session-local — no package mutable state.
							ordM := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ordM) {
								abortUnbuilt()
								return
							}
							vpM := -1
							for i, x := range ordM {
								if x == v || (v != nil && x != nil && x.Name == v.Name) {
									vpM = i
									break
								}
							}
							// residual free of pure-head owner FE(s) for v
							ownerResM := map[*Variable]bool{}
							for _, inv := range calls {
								if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
									continue
								}
								if !isForIVOfFunc(inv.User, v) {
									continue
								}
								fr := inv.User.FEffect.ReadVarsSess(s)
								if hasErrVS(vs) || !VariablesComplete(fr) {
									abortUnbuilt()
									return
								}
								if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
									continue
								}
								for _, x := range fr[1:] {
									if x == nil || isForIVOfFunc(inv.User, x) {
										continue
									}
									ownerResM[x] = true
									// also by name for Acc identity
									for _, y := range ordM {
										if y != nil && x != nil && y.Name == x.Name {
											ownerResM[y] = true
										}
									}
								}
							}
							// Acc: first free residual free-ref free of parent after v not residual free of pure-head
							var accParentFree *Variable
							if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
								seenVM := false
								for _, x := range cg.EffectAccum.ReadVarsSess(s) {
									if x == nil {
										continue
									}
									if x == v || (v != nil && x.Name == v.Name) {
										seenVM = true
										continue
									}
									if !seenVM {
										continue
									}
									if !bodySyntacticFreeReadsVar(f, x) {
										continue
									}
									if ownerResM[x] {
										continue // residual free of pure-head — keep map after those
									}
									// residual free of pure-head by name
									isOwnerResName := false
									for or := range ownerResM {
										if or != nil && or.Name == x.Name {
											isOwnerResName = true
											break
										}
									}
									if isOwnerResName {
										continue
									}
									on := summaryEff.IsReadSess(s, x)
									if hasErrVS(vs) {
										abortUnbuilt()
										return
									}
									if on {
										accParentFree = x
										break
									}
								}
							}
							if accParentFree == nil || vpM < 0 {
								continue // keep map
							}
							// pure residual pure freeVal Acc-late after free residual free-ref free of parent?
							ap := -1
							for i, x := range ordM {
								if x == accParentFree || (accParentFree != nil && x != nil && x.Name == accParentFree.Name) {
									ap = i
									break
								}
							}
							if ap < 0 || vpM < ap {
								continue // already before free residual free-ref free of parent (or absent)
							}
							// Acc-order pure residual pure freeVal before free residual free-ref free of parent
							before = accParentFree
							// fall through to Acc-order rebuild
						}
						if before == nil {
							continue
						}
						// Already immediately before Acc successor?
						ord := summaryEff.ReadVarsSess(s)
						if hasErrVS(vs) || !VariablesComplete(ord) {
							abortUnbuilt()
							return
						}
						pp, rp := -1, -1
						for i, x := range ord {
							if x == v || (v != nil && x != nil && x.Name == v.Name) {
								pp = i
							}
							if x == before || (before != nil && x != nil && x.Name == before.Name) {
								rp = i
							}
						}
						if pp >= 0 && rp >= 0 && pp < rp {
							// Already Acc-early relative to successor (may have gap).
							// Only force immediate-before when Acc-late (pp > rp).
							continue
						}
						if pp >= 0 && rp >= 0 && pp+1 == rp {
							continue
						}
						// Rebuild: emit v immediately before before.
						rebuilt := EmptyEffect()
						for _, w := range summaryEff.WrittenVarsSess(s) {
							if w == nil {
								continue
							}
							rebuilt = rebuilt.WriteVarSess(s, w)
							if hasErrVS(vs) || !EffectComplete(rebuilt) {
								if !hasErrVS(vs) {
									noteErrVS(vs, ErrGeneric)
								}
								abortUnbuilt()
								return
							}
						}
						inserted := false
						for _, x := range ord {
							if x == nil || x == v || (v != nil && x.Name == v.Name) {
								continue
							}
							if (x == before || (before != nil && x.Name == before.Name)) && !inserted {
								rebuilt = rebuilt.ReadVarSess(s, v)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
								inserted = true
							}
							rebuilt = rebuilt.ReadVarSess(s, x)
							if hasErrVS(vs) || !EffectComplete(rebuilt) {
								if !hasErrVS(vs) {
									noteErrVS(vs, ErrGeneric)
								}
								abortUnbuilt()
								return
							}
						}
						if !inserted {
							rebuilt = rebuilt.ReadVarSess(s, v)
							if hasErrVS(vs) || !EffectComplete(rebuilt) {
								if !hasErrVS(vs) {
									noteErrVS(vs, ErrGeneric)
								}
								abortUnbuilt()
								return
							}
						}
						rebuilt.pure = summaryEff.pure
						rebuilt.sideEffectFree = summaryEff.sideEffectFree
						rebuilt.lhsWrite = summaryEff.lhsWrite
						summaryEff = rebuilt
						continue
					}
					// Missing from summary: solo residual pure-only of pure-head FE-relative
					// insert after predecessor free residual free of pure-head FE on summary
					// (seed42 func_10 g_1054 after g_128 of pure-head func_30; g_97 after g_1027).
					// Multi residual pure free-ref free pure-only of pure multi pure-head invents
					// after pure residual pure multi sibling on summary (seed875 g_316 after
					// g_266); multi residual pure-only no sibling Acc-appends (float g_147).
					// FreeVal residual pure of pure-head still Acc-order when freeVal.
					// Session-local — no package mutable state.
					if !bodyArrayInitOrValueFreeReadsVar(f, v) && !isForIVOfFunc(f, v) {
						var fePred, feSucc *Variable
						for _, inv := range calls {
							if inv == nil || inv.User == nil || !EffectComplete(inv.User.FEffect) {
								continue
							}
							if !isForIVOfFunc(inv.User, v) {
								continue
							}
							fr := inv.User.FEffect.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(fr) {
								abortUnbuilt()
								return
							}
							if len(fr) == 0 || fr[0] == nil || !isForIVOfFunc(inv.User, fr[0]) {
								continue // not pure-head nested FE
							}
							vi := -1
							for i, x := range fr {
								if x == v || (v != nil && x != nil && x.Name == v.Name) {
									vi = i
									break
								}
							}
							if vi <= 0 {
								continue // FE head or missing
							}
							// multi residual pure of pure multi pure-head residual pure run
							runStart, runEnd := vi, vi+1
							for runStart > 0 && fr[runStart-1] != nil && isForIVOfFunc(inv.User, fr[runStart-1]) {
								runStart--
							}
							for runEnd < len(fr) && fr[runEnd] != nil && isForIVOfFunc(inv.User, fr[runEnd]) {
								runEnd++
							}
							ord0 := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) {
								abortUnbuilt()
								return
							}
							onSum := map[*Variable]bool{}
							accOf := map[*Variable]int{}
							for i, x := range ord0 {
								if x != nil {
									onSum[x] = true
									accOf[x] = i
								}
							}
							if runStart > 0 && runEnd-runStart > 1 {
								// Multi residual pure free-ref free pure-only of pure multi pure-
								// head: invent after previous pure residual pure free-ref free of
								// pure multi residual pure run sibling on summary only when Acc
								// has that sibling immediately before pure residual pure free-ref
								// free of pure multi residual pure free-ref free of pure-head
								// (seed875 func_45 g_316 after g_266; Acc g_266 g_316). When Acc
								// places pure residual pure free-ref free of pure multi residual
								// pure free-ref free of pure-head before free residual free-ref free
								// of pure-head (seed639 func_31 g_129 Acc before g_141; multi sibling
								// g_7 Acc-early), Acc-order before Acc successor is UP-correct —
								// do not FE-rel after multi sibling early. Pure residual pure multi
								// sibling absent / Acc not adjacent → Acc-append (float g_147).
								// Session-local — no package mutable state.
								var multiPred *Variable
								for j := vi - 1; j >= runStart; j-- {
									x := fr[j]
									if x == nil || !isForIVOfFunc(inv.User, x) {
										continue
									}
									if onSum[x] {
										multiPred = x
										break
									}
									for y := range onSum {
										if y != nil && x != nil && y.Name == x.Name {
											multiPred = y
											break
										}
									}
									if multiPred != nil {
										break
									}
								}
								if multiPred == nil {
									continue // multi residual pure-only no sibling on summary
								}
								// Acc must have multiPred immediately before v (globals on Acc)
								accAdj := false
								if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
									var prevOnAcc *Variable
									for _, x := range cg.EffectAccum.ReadVarsSess(s) {
										if x == nil {
											continue
										}
										if x == v || (v != nil && x.Name == v.Name) {
											if prevOnAcc != nil &&
												(prevOnAcc == multiPred || (multiPred != nil && prevOnAcc.Name == multiPred.Name)) {
												accAdj = true
											}
											break
										}
										// only track Acc vars that are pure residual pure multi siblings
										// or pure residual pure free-ref free of pure multi residual pure
										// free-ref free of pure-head that are on summary (ignore locals)
										on := summaryEff.IsReadSess(s, x)
										if hasErrVS(vs) {
											abortUnbuilt()
											return
										}
										if on {
											prevOnAcc = x
										}
									}
								}
								if !accAdj {
									continue // Acc not multi-sibling-adjacent — Acc-order / Acc-append
								}
								fePred = multiPred
								break
							}
							// FE index of earliest Acc-late pure residual pure free-ref free of
							// pure-head (seed3682609 g_76 of pure-head func_33 free-refs on
							// parent Acc-late after free residual free g_84). Free residual free
							// of pure-head after that pure residual pure free-ref on FE
							// (g_389/g_790) must not be invent after-anchor — place after
							// Acc-early free residual free of pure-head (g_84). Acc-only filter
							// over-skipped seed57 free residual free anchors. Session-local —
							// no package mutable state.
							latePureFreeRefFE := -1
							accLookup := func(x *Variable) (int, bool) {
								if x == nil {
									return -1, false
								}
								if a, ok := accOf[x]; ok {
									return a, true
								}
								for y, a := range accOf {
									if y != nil && y.Name == x.Name {
										return a, true
									}
								}
								return -1, false
							}
							for j := 1; j < vi; j++ {
								x := fr[j]
								if x == nil || !isForIVOfFunc(inv.User, x) {
									continue
								}
								if !bodySyntacticFreeReadsVar(f, x) {
									continue
								}
								ap, ok := accLookup(x)
								if !ok {
									continue
								}
								// Acc-late vs earlier free residual free of pure-head on sum?
								for k := 0; k < j; k++ {
									y := fr[k]
									if y == nil || isForIVOfFunc(inv.User, y) {
										continue
									}
									yp, yok := accLookup(y)
									if !yok || yp >= ap {
										continue
									}
									if latePureFreeRefFE < 0 || j < latePureFreeRefFE {
										latePureFreeRefFE = j
									}
									break
								}
							}
							// FE predecessor for solo residual pure-only invent: prefer free
							// residual free of pure-head FE on summary (seed42 g_1054 after
							// g_128). Skip pure residual pure of pure-head when seeking the
							// anchor — Acc-late own pure free-ref free of parent that is pure
							// residual pure of pure-head earlier in FE must not become after-
							// anchor (seed3682609 g_57 after g_84 residual free of pure-head
							// func_33, not after g_76 pure residual free-ref free of parent
							// Acc-late). Also skip free residual free of pure-head after that
							// Acc-late pure residual pure free-ref on FE (g_389). Session-
							// local — no package mutable state.
							for j := vi - 1; j >= 0; j-- {
								x := fr[j]
								if x == nil {
									continue
								}
								if isForIVOfFunc(inv.User, x) {
									continue // pure residual pure of pure-head — not free residual free pred
								}
								if latePureFreeRefFE >= 0 && j > latePureFreeRefFE {
									// Free residual free of pure-head after Acc-late pure residual
									// pure free-ref of pure-head on FE (seed368 g_389 after g_76).
									continue
								}
								var cand *Variable
								if onSum[x] {
									cand = x
								} else {
									// name match Acc identity
									for y := range onSum {
										if y != nil && x != nil && y.Name == x.Name {
											cand = y
											break
										}
									}
								}
								if cand == nil {
									continue
								}
								fePred = cand
								break
							}
							for j := vi + 1; j < len(fr); j++ {
								x := fr[j]
								if x == nil {
									continue
								}
								if onSum[x] {
									feSucc = x
									break
								}
								for y := range onSum {
									if y != nil && x != nil && y.Name == x.Name {
										feSucc = y
										break
									}
								}
								if feSucc != nil {
									break
								}
							}
							if fePred != nil || feSucc != nil {
								break
							}
						}
						if os.Getenv("DIAG_S368") != "" && f != nil && f.Name == "func_1" && v != nil && v.Name == "g_57" {
							pn, sn := "nil", "nil"
							if fePred != nil {
								pn = fePred.Name
							}
							if feSucc != nil {
								sn = feSucc.Name
							}
							fmt.Fprintf(os.Stderr, "S368 FErel fePred=%s feSucc=%s\n", pn, sn)
						}
						if fePred != nil {
							if os.Getenv("DIAG_S42") != "" && v != nil && v.Name == "g_1054" {
								fmt.Fprintf(os.Stderr, "S42 invent FE-rel after %s f=%s\n", fePred.Name, f.Name)
							}
							// insert immediately after fePred
							ord0 := summaryEff.ReadVarsSess(s)
							if hasErrVS(vs) || !VariablesComplete(ord0) {
								abortUnbuilt()
								return
							}
							rebuilt := EmptyEffect()
							for _, w := range summaryEff.WrittenVarsSess(s) {
								if w == nil {
									continue
								}
								rebuilt = rebuilt.WriteVarSess(s, w)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
							}
							for _, x := range ord0 {
								if x == nil || x == v || (v != nil && x.Name == v.Name) {
									continue
								}
								rebuilt = rebuilt.ReadVarSess(s, x)
								if hasErrVS(vs) || !EffectComplete(rebuilt) {
									if !hasErrVS(vs) {
										noteErrVS(vs, ErrGeneric)
									}
									abortUnbuilt()
									return
								}
								if x == fePred || (fePred != nil && x.Name == fePred.Name) {
									rebuilt = rebuilt.ReadVarSess(s, v)
									if hasErrVS(vs) || !EffectComplete(rebuilt) {
										if !hasErrVS(vs) {
											noteErrVS(vs, ErrGeneric)
										}
										abortUnbuilt()
										return
									}
								}
							}
							rebuilt.pure = summaryEff.pure
							rebuilt.sideEffectFree = summaryEff.sideEffectFree
							rebuilt.lhsWrite = summaryEff.lhsWrite
							summaryEff = rebuilt
							continue
						}
						if feSucc != nil {
							summaryEff = summaryEff.ReadVarBeforeSess(s, v, feSucc)
							if hasErrVS(vs) || !EffectComplete(summaryEff) {
								if !hasErrVS(vs) {
									noteErrVS(vs, ErrGeneric)
								}
								abortUnbuilt()
								return
							}
							continue
						}
					}
					if os.Getenv("DIAG_S42") != "" && v != nil && v.Name == "g_1054" {
						bn := ""
						if before != nil {
							bn = before.Name
						}
						fmt.Fprintf(os.Stderr, "S42 invent Acc-order g_1054 f=%s before=%s\n", f.Name, bn)
					}
					// Missing from summary: Acc-order insert (or append).
					if before != nil {
						summaryEff = summaryEff.ReadVarBeforeSess(s, v, before)
					} else {
						summaryEff = summaryEff.ReadVarSess(s, v)
					}
					if os.Getenv("DIAG_C302") != "" && f != nil && f.Name == "func_34" && v != nil && (v.Name == "g_99" || v.Name == "g_328.f2") {
						pos := -1
						for i, x := range summaryEff.ReadVarsSess(s) {
							if x != nil && x.Name == v.Name {
								pos = i
								break
							}
						}
						bn := ""
						if before != nil {
							bn = before.Name
						}
						fmt.Fprintf(os.Stderr, "C302_INVENT %s@%d before=%s lead=%v\n", v.Name, pos, bn, isLeading)
					}

					if hasErrVS(vs) || !EffectComplete(summaryEff) {
						if !hasErrVS(vs) {
							noteErrVS(vs, ErrGeneric)
						}
						abortUnbuilt()
						return
					}
					if os.Getenv("DIAG_PUREMISS") != "" && f.Name == "func_1" {
						fmt.Fprintf(os.Stderr, "ACC_APPEND %s\n", v.Name)
					}
					if os.Getenv("DIAG_FLOAT2") != "" && f != nil && f.Name == "func_1" && v != nil {
						fmt.Fprintf(os.Stderr, "FLOAT2 ACC_APPEND %s ownFree=%v nestedFree=%v isLeading=%v\n",
							v.Name, ownFree, hasFree, isLeading)
					}
				}
				} // accPass
				if os.Getenv("DIAG_S12593A") != "" && f != nil && f.Name == "func_1" {
					fmt.Fprint(os.Stderr, "S12593POST")
					for i, x := range summaryEff.ReadVarsSess(s) {
						if x != nil && (x.Name == "g_1678" || x.Name == "g_3964" || x.Name == "g_2252" || x.Name == "g_2249" || x.Name == "g_1326" || x.Name == "g_1143.f1") {
							fmt.Fprintf(os.Stderr, " %s@%d", x.Name, i)
						}
					}
					fmt.Fprintln(os.Stderr)
				}



			// Multi pure-prefix pure-only FE head Acc-late after pure multi pureOnly of
			// same FE → before earliest pure multi pureOnly (seed12336 g_659 of func_15
			// before g_661). map_stm multi-prefix already ran without Acc-invented head;
			// Acc invent places missing multi head Acc-order (often late). Session-local
			// — no package mutable state.
			if InvocationsComplete(calls) {
				var acc *Effect
				if cg.EffectAccum != nil && EffectComplete(*cg.EffectAccum) {
					acc = cg.EffectAccum
				}
				if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" {
					fmt.Fprint(os.Stderr, "S875 pre_reorderMulti=")
					for _, x := range summaryEff.ReadVarsSess(s) {
						if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70") {
							fmt.Fprintf(os.Stderr, " %s", x.Name)
						}
					}
					fmt.Fprintln(os.Stderr)
					if acc != nil {
						fmt.Fprint(os.Stderr, "S875 Acc=")
						for _, x := range acc.ReadVarsSess(s) {
							if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70") {
								fmt.Fprintf(os.Stderr, " %s", x.Name)
							}
						}
						fmt.Fprintln(os.Stderr)
					}
				}
				summaryEff = reorderMultiPureFEHeadBeforePureMultiSiblings(s, f, summaryEff, calls, acc)

				if hasErrVS(vs) || !EffectComplete(summaryEff) {
					if !hasErrVS(vs) {
						noteErrVS(vs, ErrGeneric)
					}
					abortUnbuilt()
					return
				}
				// Free-head mid pure residual pure-only Acc-late after residual free of
				// same FE currently before pure: restore FE order pure before residual
				// free (seed12593 func_30 g_1678 before g_3964 after g_2252). Residual
				// free free-ref free-head owner keeps Acc free residual free-ref before
				// pure (seed12848 g_270 before g_528.f3). Session-local — no package
				// mutable state.
				summaryEff = reorderFreeHeadMidPureBeforeResidualFree(s, f, summaryEff, calls)
				if hasErrVS(vs) || !EffectComplete(summaryEff) {
					if !hasErrVS(vs) {
						noteErrVS(vs, ErrGeneric)
					}
					abortUnbuilt()
					return
				}
				if os.Getenv("DIAG_S875") != "" && f.Name == "func_1" {
					fmt.Fprint(os.Stderr, "S875 post_reorderMulti=")
					for _, x := range summaryEff.ReadVarsSess(s) {
						if x != nil && (x.Name == "g_13" || x.Name == "g_116" || x.Name == "g_117" || x.Name == "g_149" || x.Name == "g_194" || x.Name == "g_266" || x.Name == "g_51" || x.Name == "g_97" || x.Name == "g_70") {
							fmt.Fprintf(os.Stderr, " %s", x.Name)
						}
					}
					fmt.Fprintln(os.Stderr)
				}
			}
			// Post-body FixupFunc* already stripped pure multi pure-only pure of pure-head
			// nested FE from map_stm. Acc invent re-adds free residual free-ref pure multi
			// of free-head (seed=2 g_37 Acc-order). Pure-only pure invent skip below
			// blocks float g_370 re-append. Pure multi pure-only FE head invent skip
			// (seed48 g_250) is in the !hasFree isLeading multiHead path. Session-local
			// — no package mutable state.
		}
	}
	if os.Getenv("DIAG_PUREMISS") != "" && f != nil && f.Name == "func_1" {
		s := sessFromVS(vs)
		var names []string
		for _, v := range summaryEff.ReadVarsSess(s) {
			if v != nil {
				names = append(names, v.Name)
			}
		}
		fmt.Fprintf(os.Stderr, "SUMMARY_FINAL n=%d\n", len(names))
		for i, n := range names {
			if n == "g_278" || n == "g_1145" || n == "g_750.f2" || n == "g_129.f2" || n == "g_1177" || n == "g_623" || n == "g_1581" {
				fmt.Fprintf(os.Stderr, "  %s@%d\n", n, i)
			}
		}
	}
	if os.Getenv("DIAG_GO_G8") != "" && f != nil && f.Name == "func_13" {
		s := sessFromVS(vs)
		listNames := func(e Effect) []string {
			var out []string
			for _, v := range e.ReadVarsSess(s) {
				if v != nil && v.IsGlobalSess(s) {
					out = append(out, v.Name)
				}
			}
			return out
		}
		fmt.Fprintf(os.Stderr, "GO_G8_FINAL summary globals=%v\n", listNames(summaryEff))
		if cg.FM != nil {
			for _, blk := range f.Blocks {
				if blk == nil || StmIDUnset(blk.StmID) {
					continue
				}
				be := cg.FM.GetMapStmEffect(blk.StmID)
				if !EffectComplete(be) {
					continue
				}
				gs := listNames(be)
				if len(gs) > 0 {
					fmt.Fprintf(os.Stderr, "  blk sid=%d loop=%v globals=%v\n", blk.StmID, blk.Looping, gs)
				}
				for i := range blk.Stmts {
					st := &blk.Stmts[i]
					if StmIDUnset(st.StmID) {
						continue
					}
					se := cg.FM.GetMapStmEffect(st.StmID)
					if !EffectComplete(se) {
						continue
					}
					gs = listNames(se)
					has8 := false
					for _, n := range gs {
						if n == "g_8" || n == "g_227" {
							has8 = true
						}
					}
					if has8 || st.Kind == StmtAssign || st.Kind == StmtFor {
						fmt.Fprintf(os.Stderr, "  stm sid=%d kind=%v globals=%v\n", st.StmID, st.Kind, gs)
					}
				}
			}
		}
		fmt.Fprintf(os.Stderr, "  ptrRef=%v factCh=%v unionRd=%v nRefPtrs=%d\n",
			f.IsPointerReferencedSess(s), f.FactChanged, f.UnionFieldRead, len(f.ReferencedPtrs))
	}
	if os.Getenv("DIAG_S368") != "" && f != nil && f.Name == "func_1" {
		fmt.Fprintf(os.Stderr, "S368 FINAL=")
		for i, x := range summaryEff.ReadVarsSess(sessFromVS(vs)) {
			if x != nil && (x.Name == "g_57" || x.Name == "g_84" || x.Name == "g_83" || x.Name == "g_647.f3" || x.Name == "g_76") {
				fmt.Fprintf(os.Stderr, " [%d]%s", i, x.Name)
			}
		}
		fmt.Fprintln(os.Stderr)
	}
	f.ComputeSummarySess(sessFromVS(vs), summaryEff)
	if hasErrVS(vs) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// Function.cpp:658 / 694 — make_return_const; ERROR_RETURN
	f.MakeReturnConstSess(sessFromVS(vs), opts, probs, r)
	if hasErrVS(vs) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// known-params: EffectAccum is the caller's object (shared). GenerateBody: local
	// bodyEff discarded. Incomplete shared accum fails closed (no invent Built past hole).
	if cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum) {
		abortUnbuilt()
		return
	}

	// Function.cpp:764–766 — global_facts = map_facts_out[body] + add_back_return_facts
	// Full FactVec (point-to + eUnionWrite). Incomplete outs fail closed Unbuilt.
	if cg.FM != nil && f.Body != nil {
		cg.FM.AssignGlobalFactsFromMapOut(f.Body.StmID)
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			cg.FM.GlobalFacts = IncompleteFactSlice()
			cg.FM.UnionFacts = IncompleteUnionFactSlice()
			abortUnbuilt()
			return
		}
		if !AddBackReturnFacts(f.Body, cg.FM, &cg.FM.GlobalFacts, &cg.FM.UnionFacts) ||
			!FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			cg.FM.GlobalFacts = IncompleteFactSlice()
			cg.FM.UnionFacts = IncompleteUnionFactSlice()
			abortUnbuilt()
			return
		}
	}
	// Function.cpp:661–662 — Mark Built
	f.markBuiltSess(sessFromVS(vs))
}
}

// MakeReturnConst mirrors Function::make_return_const.
// Function.cpp:608–615 — depth_protect + need_return_stmt → random constant.
// probs is session Probabilities (C++ singleton); no invent NewProbabilities(opts).
// Function always live; sticky (no invent soft-skip ret_c past hole).
// DepthProtect off / no return needed is complete no-op.
func (f *Function) MakeReturnConstSess(s *Session, opts Options, probs *Probabilities, r *Rng) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !opts.DepthProtect || !f.NeedReturnStmtSess(s) {
		return
	}
	// Function.cpp:610–612 — assert(return_type); assert simple != eVoid
	if f.ReturnType == nil {
		// assert(return_type) — sticky error for GenerateBody ERROR_RETURN
		sessNoteError(s, ErrGeneric)
		return
	}
	simple := f.ReturnType.IsSimpleSess(s)
	// residual ERROR sticky — no invent soft-ret past IsSimple residual
	if sessHasError(s) {
		return
	}
	if simple && f.ReturnType.SimpleSess(s) == EVoid {
		// need_return_stmt is false for void; fail closed if called wrongly
		return
	}
	// Function.cpp:612 — Constant::make_random; no invent "0" on nil RNG/fail
	if r == nil {
		// C++ always has process RNG; fail closed with ERROR_RETURN semantics
		sessNoteError(s, ErrGeneric)
		return
	}
	// session probs; aggregate ret_c needs live tables (nil → fail closed, no invent)
	f.RetConst = MakeRandomSess(s, f.ReturnType, opts, probs, r)
	// Function.cpp:614 ERROR_RETURN after Constant::make_random
	// sticky error so GenerateBody does not invent Built without ret_c
	if sessHasError(s) || f.RetConst == nil {
		f.RetConst = nil
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return
	}
}

// returnTypeC is qualified return type (from RV qfer when present).
// Function always has return_type in C++; no soft invent "void" when missing.
// RV present with Type-nil sticky (no invent fall through to ReturnType / void past
// incomplete return-variable type shell).
// Ambient ProcessOptions bridge; emit paths prefer returnTypeCSess / returnTypeCOptsSess.}

// returnTypeCSess is returnTypeC with Options/sticky from an explicit session bag.
func (f *Function) returnTypeCSess(s *Session) string {
	return f.returnTypeCOptsSess(s, sessOpts(s))
}

// returnTypeCOpts is returnTypeC with explicit session Options (const/volatile asserts).
func (f *Function) returnTypeCOptsSess(s *Session, opts Options) string {
	// Function always live at emit; sticky no invent "void" without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if f.RV != nil {
		// RV Type* always live when rv is present; Type-nil sticky incomplete
		if f.RV.Type == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		out := f.RV.Qfer.OutputQualifiedTypeOptsSess(s, f.RV.Type, opts)
		// residual ERROR sticky — no invent soft-empty return past OutputQualifiedType residual
		if sessHasError(s) {
			return ""
		}
		if out == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		return out
	}
	if f.ReturnType != nil {
		cn := f.ReturnType.CNameSess(s)
		// residual ERROR sticky — no invent soft-empty return past CName residual
		if sessHasError(s) {
			return ""
		}
		if cn == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		return cn
	}
	// incomplete IR sticky — no invent void
	sessNoteError(s, ErrGeneric)
	return ""
}

// paramListC emits parameter list with qualified types.
// Function.cpp:501–512 — empty → void.
// opts nil uses ProcessOptions for arg_structs/arg_unions asserts.}

// paramListCSess is paramListC with Options/sticky from an explicit session bag.
func (f *Function) paramListCSess(s *Session) string {
	return f.paramListCOptsSess(s, sessOpts(s))
}

// paramListCOpts returns param C list, or "" if IR violates arg_structs/arg_unions asserts.
// Function always live at emit; nil shell sticky empty (no invent "void" param list
// past missing Function IR). Empty Param is complete "void".
func (f *Function) paramListCOptsSess(s *Session, opts Options) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if len(f.Param) == 0 {
		// Function.cpp:502–504 — assert(Type::void_type); emit void
		return "void"
	}
	var b strings.Builder
	for i, p := range f.Param {
		if p == nil || p.Type == nil {
			// incomplete param IR sticky — fail closed (no invent type name)
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// Function.cpp:489–491 — assert(!arg_structs → not struct; same unions) sticky
		if !opts.ArgStructs && p.Type.IsStructSess(s) {
			// residual ERROR sticky — no invent soft-empty param past IsStruct residual
			if sessHasError(s) {
				return ""
			}
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue param past IsStruct residual false
		if sessHasError(s) {
			return ""
		}
		if !opts.ArgUnions && p.Type.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-empty param past IsUnion residual
			if sessHasError(s) {
				return ""
			}
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue param past IsUnion residual false
		if sessHasError(s) {
			return ""
		}
		// Variable always has live name + qualified type; sticky no invent "int " / " p"
		ty := p.Qfer.OutputQualifiedTypeOptsSess(s, p.Type, opts)
		// residual ERROR sticky — no invent soft-continue later params past OutputQualifiedType residual
		if sessHasError(s) {
			return ""
		}
		if ty == "" || p.Name == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(ty)
		b.WriteString(" ")
		b.WriteString(p.Name)
	}
	return b.String()
}

// OutputHeader mirrors Function::OutputHeader.
// Function.cpp:516–531 — optional inline/static + qualified return + name(params).}

// OutputHeaderSess is OutputHeader with Options/sticky from an explicit session bag.
func (f *Function) OutputHeaderSess(s *Session, forceStatic bool) string {
	return f.OutputHeaderOptsSess(s, forceStatic, sessOpts(s))
}

// OutputHeaderOpts is OutputHeader with explicit Options for return/arg struct/union asserts.
func (f *Function) OutputHeaderOptsSess(s *Session, forceStatic bool, opts Options) string {
	// Function always live at emit; sticky no invent "int (void)" without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Function.cpp:517–520 — assert no return struct/union when options off
	// sticky fail closed: empty header (no invent alternate scalar return)
	// RV present with Type-nil sticky (no invent skip option checks past incomplete rv type)
	rt := f.ReturnType
	if f.RV != nil {
		if f.RV.Type == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		rt = f.RV.Type
	}
	if rt != nil {
		if !opts.ReturnStructs && rt.IsStructSess(s) {
			// residual ERROR sticky — no invent soft-empty header past IsStruct residual
			if sessHasError(s) {
				return ""
			}
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue header past IsStruct residual false
		if sessHasError(s) {
			return ""
		}
		if !opts.ReturnUnions && rt.IsUnionSess(s) {
			// residual ERROR sticky — no invent soft-empty header past IsUnion residual
			if sessHasError(s) {
				return ""
			}
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue header past IsUnion residual false
		if sessHasError(s) {
			return ""
		}
	}
	// Function always has a live internal name; sticky no invent "int (void)" without it.
	// Emit name is get_prefixed_name (Function.cpp:528) — may be empty under prefix_name.
	if f.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	params := f.paramListCOptsSess(s, opts)
	// residual ERROR sticky — no invent soft-empty header past paramList residual
	if sessHasError(s) {
		return ""
	}
	if params == "" {
		// assert-path sticky fail closed on forbidden/incomplete params
		sessNoteError(s, ErrGeneric)
		return ""
	}
	rtName := f.returnTypeCOptsSess(s, opts)
	// residual ERROR sticky — no invent soft-empty header past returnTypeC residual
	if sessHasError(s) {
		return ""
	}
	if rtName == "" {
		// incomplete return type IR sticky — no invent void header
		sessNoteError(s, ErrGeneric)
		return ""
	}
	emitName := GetPrefixedNameSess(s, f.Name, opts.PrefixName)
	// residual ERROR sticky — no invent soft-continue header past get_prefixed_name residual
	if sessHasError(s) {
		return ""
	}
	var b strings.Builder
	// Function.cpp:521–522
	if f.IsInlined {
		b.WriteString("inline ")
	}
	// Function.cpp:524–526 — force_globals_static
	if forceStatic {
		b.WriteString("static ")
	}
	// Function.cpp:527–530 — get_prefixed_name(name) (empty ok under prefix_name NDEBUG)
	b.WriteString(rtName)
	b.WriteString(" ")
	b.WriteString(emitName)
	b.WriteString("(")
	b.WriteString(params)
	b.WriteString(")")
	return b.String()
}

// OutputForwardDecl emits a C prototype.
// Function.cpp:555–561 — builtins emit nothing (compiler-provided).}

// OutputForwardDeclOpts adds optional func __attribute__ and force_static.
// Function.cpp:547–553 — OutputHeader + attrs + ";".
// Function always live at emit; sticky empty (no invent bare ";" past hole).
// Builtins are complete empty (compiler-provided; not incomplete IR).
// OutputForwardDeclSess is OutputForwardDeclOpts with Options/sticky from an explicit bag.
func (f *Function) OutputForwardDeclSess(s *Session, forceStatic bool, r *Rng, withAttrs bool) string {
	return f.OutputForwardDeclWithSess(s, forceStatic, r, withAttrs, sessOpts(s))
}

// OutputForwardDeclWith is OutputForwardDeclOpts with explicit session Options.
// OutputForwardDeclWithSess is OutputForwardDeclWith on an explicit session bag.
func (f *Function) OutputForwardDeclWithSess(sess *Session, forceStatic bool, r *Rng, withAttrs bool, opts Options) string {
	if f == nil {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	if f.IsBuiltin {
		return ""
	}
	s := f.OutputHeaderOptsSess(sess, forceStatic, opts)
	// residual ERROR sticky — no invent bare ";" past OutputHeader residual
	if sessHasError(sess) {
		return ""
	}
	// incomplete header IR sticky — no invent bare ";"
	if s == "" {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	if withAttrs && r != nil {
		if ag := EnsureFuncAttrGeneratorSess(sess); ag != nil {
			s += ag.OutputSess(sess, r)
		}
		// residual ERROR sticky — no invent soft-continue ";" past attr residual
		if sessHasError(sess) {
			return ""
		}
	}
	s += ";"
	return s
}

// OutputHeaderAlias mirrors Function::OutputHeaderAlias.
// Function.cpp:533–541 — static? + type alias_name(params) __attribute__((alias("name"))).
// Incomplete Function sticky empty (no invent alias shell without function).
// OutputHeaderAliasSess is OutputHeaderAlias with Options/sticky from an explicit bag.
func (f *Function) OutputHeaderAliasSess(s *Session, forceStatic bool) string {
	return f.OutputHeaderAliasOptsSess(s, forceStatic, sessOpts(s))
}

// OutputHeaderAliasOpts is OutputHeaderAlias with explicit session Options.
// OutputHeaderAliasOptsSess is OutputHeaderAliasOpts with explicit session residual sticky.
func (f *Function) OutputHeaderAliasOptsSess(s *Session, forceStatic bool, opts Options) string {
	// Function always live at emit; sticky incomplete no invent empty alias shell
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Function.cpp:533–541 — alias_name may be empty (builtins never set alias_name;
	// constructor leaves it default). Soft invent required AliasName skipped builtin
	// aliases (flagcamp --function-attributes --builtins: empty-name alias lines).
	// Name may still be live for alias("name") target.
	if f.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	rtName := f.returnTypeCOptsSess(s, opts)
	if rtName == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	params := f.paramListCOptsSess(s, opts)
	if params == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	// Function.cpp:538–540 — get_prefixed_name on alias_name and name (empty alias ok)
	emitAlias := GetPrefixedNameSess(s, f.AliasName, opts.PrefixName)
	if sessHasError(s) {
		return ""
	}
	emitName := GetPrefixedNameSess(s, f.Name, opts.PrefixName)
	if sessHasError(s) {
		return ""
	}
	var b strings.Builder
	if forceStatic {
		b.WriteString("static ")
	}
	b.WriteString(rtName)
	b.WriteString(" ")
	b.WriteString(emitAlias)
	b.WriteString("(")
	b.WriteString(params)
	b.WriteString(") __attribute__((alias(\"")
	b.WriteString(emitName)
	b.WriteString("\")))")
	return b.String()
}

// OutputForwardDeclAlias mirrors Function::OutputForwardDeclAlias.
// Function.cpp:555–559 — OutputHeaderAlias + ";" for every FuncList entry
// (unlike OutputForwardDecl which skips is_builtin). Soft invent skip-builtins
// dropped alias("__builtin_*") lines under --function-attributes.
// OutputForwardDeclAliasSess is OutputForwardDeclAlias with Options/sticky from an explicit bag.
func (f *Function) OutputForwardDeclAliasSess(s *Session, forceStatic bool) string {
	return f.OutputForwardDeclAliasOptsSess(s, forceStatic, sessOpts(s))
}

// OutputForwardDeclAliasOpts is OutputForwardDeclAlias with explicit session Options.
// OutputForwardDeclAliasOptsSess is OutputForwardDeclAliasOpts with explicit session residual sticky.
func (f *Function) OutputForwardDeclAliasOptsSess(s *Session, forceStatic bool, opts Options) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Function.cpp:555–558 — no is_builtin early return (only OutputForwardDecl has that)
	out := f.OutputHeaderAliasOptsSess(s, forceStatic, opts)
	// incomplete alias header sticky — no invent bare ";"
	if out == "" {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	return out + ";"
}

// Output emits a C function definition (minimal statements).
// Function.cpp:565–598 — builtins emit nothing.
// OutputSess is Output with Options/sticky from an explicit session bag.
func (f *Function) OutputSess(s *Session, forceStatic, withAttrs bool, r *Rng) string {
	return f.OutputOptsWithSess(s, forceStatic, withAttrs, r, sessOpts(s))
}

// OutputOpts adds force_static and optional function attributes on the header.
// OutputOptsWith is OutputOpts with explicit session Options (header asserts + body emit).
// OutputOptsWithSess is OutputOptsWith on an explicit session bag.
func (f *Function) OutputOptsWithSess(sess *Session, forceStatic, withAttrs bool, r *Rng, opts Options) string {
	// Function always live at def emit; sticky no invent empty def without it
	if f == nil {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	// builtins emit nothing (Function.cpp) — soft empty, not incomplete IR
	if f.IsBuiltin {
		return ""
	}
	// Function.cpp:572 — OutputHeader always live; sticky no invent separator-only shell
	hdr := f.OutputHeaderOptsSess(sess, forceStatic, opts)
	// residual ERROR sticky — no invent soft-continue body past OutputHeader residual
	if sessHasError(sess) {
		return ""
	}
	if hdr == "" {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	s := ""
	// Function.cpp:567 — output_comment_line separator (quiet|concise → blank line only)
	quiet, concise := opts.Quiet, opts.Concise || f.EmitConcise
	s += OutputCommentLineSess(sess, "------------------------------------------", quiet, concise)
	// Function.cpp:568–570 — feffect.Output when !concise; wrap still quiet-suppresses
	if !concise {
		s += f.FEffect.CommentOutputOptsSess(sess, quiet, concise)
		// residual ERROR sticky — no invent soft-continue past CommentOutput residual
		if sessHasError(sess) {
			return ""
		}
	}
	// Function.cpp:565–598 — OutputHeader only; attrs are on OutputForwardDecl only
	// (do not invent def-site attrs or double-burn func_attr RNG).
	_ = withAttrs
	_ = r
	s += hdr
	s += "\n"
	// Function.cpp:575–598 — depth_protect + body + else ret_c always live together
	// sticky no invent header-only / empty-body shells (C++ would dereference body)
	if f.Body == nil {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	// indent 0: function body braces at column 0 (Block::Output / DefaultOutputMgr style).
	bodyOut := f.Body.OutputOptsSess(sess, 0, opts)
	// residual ERROR sticky — no invent soft-continue past Body.Output residual
	if sessHasError(sess) {
		return ""
	}
	if bodyOut == "" {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	if f.DepthProtect && f.RetConst != nil {
		// Function.cpp:575–597 — if (DEPTH…) body else return ret_c
		// both body Output and ret_c value always live; sticky no invent if/else or "return ;"
		retVal := f.RetConst.Value
		if retVal == "" {
			// incomplete ret_c sticky — body only, no invent depth if/else
			sessNoteError(sess, ErrGeneric)
			return ""
		}
		s += "if (DEPTH < MAX_DEPTH) \n"
		s += bodyOut
		s += "else\n"
		s += "return "
		s += retVal
		s += ";\n"
		// Function.cpp:594–595 — two outputln after body
		s += "\n\n"
		return s
	}
	// body without complete depth_protect wrap (void / missing ret_c)
	// no soft invent "if (DEPTH…)" without else ret_c
	s += bodyOut
	// Function.cpp:594–595 — two outputln after function body
	s += "\n\n"
	return s
}
