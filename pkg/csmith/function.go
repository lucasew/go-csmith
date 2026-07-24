// Upstream: Function.h / Function.cpp
// (RandomFunctionName, RandomReturnType, make_random_signature, make_first, GenerateParameterList).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

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
}

// PairedFactMgr returns the FactMgr registered with this function at create time.
// Mirrors get_fact_mgr_for_func when the function is on FuncList/FMList.
// Incomplete Function sticky nil (no invent soft-miss FM past hole).
func (f *Function) PairedFactMgr() *FactMgr {
	// Function always live; sticky incomplete no invent nil FM soft-skip
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	return f.factMgr
}

// ensurePairedFactMgr returns the paired FactMgr, creating once at signature-time
// semantics (Function.cpp FMList.push_back(new FactMgr(f))).
func (f *Function) ensurePairedFactMgr() *FactMgr {
	return f.ensurePairedFactMgrSess(nil)
}

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
func (f *Function) IsEffectKnown() bool {
	// Function always live; sticky incomplete no invent effect-known soft-skip
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return f.BuildState == BuildBuilt
}

// markBuilt sets Built state and IsBuilt flag.
// Function always live; sticky (no invent soft-skip Built past hole).
func (f *Function) markBuilt() {
	if f == nil {
		sessNoteError(nil, ErrGeneric)
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
func RandomFunctionName(sym *GenSym) string {
	return RandomFunctionNameSess(nil, sym)
}

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
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if env == nil || len(env.AllTypes) == 0 {
		return nil
	}
	return env.ChooseRandom(r, opts, probs, false)
}

// ParamListProbability mirrors Function.cpp ParamListProbability → rnd_upto(max_params).
func ParamListProbability(r *Rng, opts Options) uint32 {
	// C++ always has RNG; sticky no invent param count 0 without draw
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	n := opts.MaxParams
	if n < 0 {
		n = 0
	}
	return r.RndUpto(uint32(n))
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
		cg.Sess = vsSess(vs)
	}
	s := cg.Sess
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
	if DepthGuardByType(opts, DtFunction) == BadDepth {
		return nil
	}
	// Function.cpp:408 ERROR_GUARD after DEPTH_GUARD
	if sessHasError(s) {
		return nil
	}
	name := RandomFunctionNameSess(firstSess(vsSess(vs), s), sym)
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
	f.RV = CreateVariableQfer(name+"_rv", retType, retQ)
	// Function.cpp:419–420 — CreateVariable + ERROR_GUARD path; no soft invent signature without rv
	if f.RV == nil || sessHasError(s) {
		return nil
	}
	// GenerateParameterList: for i=0; i<=max; i++
	max := ParamListProbability(r, opts)
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
	f.ensurePairedFactMgrSess(s)
	// inline flip if enabled
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
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
		cg.Sess = vsSess(vs)
	}
	s := cg.Sess
	f := MakeRandomSignature(r, opts, probs, vs, sym, cg, retType, qfer, list)
	// Function.cpp:434 ERROR_GUARD after signature
	if f == nil || sessHasError(s) {
		return nil
	}
	// Function.cpp:422 FMList entry from signature — get_fact_mgr_for_func (no invent second)
	// sticky no invent GenerateBody without live FactMgr
	fm := f.PairedFactMgr()
	if fm == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Variable* always live on GlobalList; nil hole fails closed sticky
	// (AddNewVarFact(nil) no-ops — invent partial FM seed then GenerateBody)
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
	runSess := firstSess(vsSess(vs), nil)
	if fmMap != nil && fmMap.Sess != nil {
		runSess = firstSess(runSess, fmMap.Sess)
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
	retQ := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	if sessHasError(runSess) {
		return nil
	}
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)
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
		_ = fmMap.ForFunc(f)
	}
	// seed existing globals so first function sees them (generation convenience)
	// Variable* always live on GlobalList; nil hole fails closed sticky
	// (AddNewVarFact(nil) no-ops — invent partial FM seed then GenerateBody)
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
	// Bridge: ambient only when no VS/FMList session (unit tests).
	if cg.Sess == nil {
		cg.Sess = currentSession()
	}
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, cg)
	// sticky error / null body / Unbuilt — do not invent success first function
	if sessHasError(runSess) || f.Body == nil || f.BuildState != BuildBuilt {
		return nil
	}

	// Function.cpp:464–465 — inline flip after body
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
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
		sessNoteError(nil, ErrGeneric)
		return
	}
	// Function.cpp:626–629 / 668–671 — ignore regenerate
	if f.BuildState != BuildUnbuilt {
		return
	}
	// Function.cpp:643–648 — non-builtin make_random body always has process RNG
	// sticky no invent Building/Built shell without RNG
	if !f.IsBuiltin && r == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	// incomplete ambient fails closed sticky before BuildBuilding
	// (no invent body under hole shells / soft re-pick past incomplete prev)
	if !EffectComplete(prev.EffectContext()) ||
		(prev.EffectAccum != nil && !EffectComplete(*prev.EffectAccum)) ||
		!EffectComplete(prev.EffectStm) {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if prev.FM != nil && !FactsComplete(prev.FM.GlobalFacts) {
		sessNoteError(nil, ErrGeneric)
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
	if sessHasError(nil) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}
	if prev.Funcs != nil {
		cg.Funcs = prev.Funcs
	}
	// Function.cpp:635 / 674 — get_fact_mgr_for_func(this); no invent NewFactMgr here
	if cg.FM == nil {
		cg.FM = f.PairedFactMgr()
		// residual ERROR sticky — no invent soft-continue body past PairedFactMgr residual
		if sessHasError(nil) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	}
	if cg.FM == nil {
		// get_fact_mgr_for_func returned null — sticky fail closed (no soft invent FM)
		sessNoteError(nil, ErrGeneric)
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
			if sessHasError(nil) {
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			cg.RW = rwd
		} else if sessHasError(nil) {
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
				sessNoteError(nil, ErrGeneric)
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			if p.Type == nil && !IsSpecialPtr(p) {
				sessNoteError(nil, ErrGeneric)
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
			if p.IsPointer() {
				// residual ERROR sticky — no invent soft-skip param seed past IsPointer hole
				if sessHasError(nil) {
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
				// incomplete GlobalFacts sticky before soft FindRelated miss invent
				if !FactsComplete(cg.FM.GlobalFacts) {
					sessNoteError(nil, ErrGeneric)
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
				if FindRelatedPointTo(cg.FM.GlobalFacts, p) == nil {
					// residual ERROR sticky — no invent soft-continue later params past FindRelated hole
					if sessHasError(nil) {
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
					nf := MakeFactPointTo(p, TBDPtr)
					if nf == nil || sessHasError(nil) {
						if !sessHasError(nil) {
							sessNoteError(nil, ErrGeneric)
						}
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
					cg.FM.SetGlobalFacts(append(cg.FM.GlobalFacts, nf), "auto_function_666")
					// residual ERROR sticky — no invent soft-continue later params past append residual
					if sessHasError(nil) {
						f.BuildState = BuildUnbuilt
						f.IsBuilt = false
						return
					}
				} else if sessHasError(nil) {
					f.BuildState = BuildUnbuilt
					f.IsBuilt = false
					return
				}
			} else if sessHasError(nil) {
				// residual ERROR sticky — no invent soft-continue non-pointer past IsPointer hole
				f.BuildState = BuildUnbuilt
				f.IsBuilt = false
				return
			}
		}
	}

	// Function.cpp:643–648 — builtin dummy vs make_random
	// make_dummy_block needs CGContext (fact_in + post_creation); no soft empty shell
	if f.IsBuiltin {
		f.Body = MakeDummyBlockCG(&cg, opts)
		// residual ERROR sticky — no invent soft-Built past MakeDummyBlock residual
		if sessHasError(nil) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	} else {
		f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &cg, false)
		// residual ERROR sticky — no invent soft-Built past MakeRandomBlock residual
		if sessHasError(nil) {
			f.BuildState = BuildUnbuilt
			f.IsBuilt = false
			return
		}
	}
	// Function.cpp:647 / 689 — ERROR_RETURN(); body->set_depth_protect
	// sticky error aborts; null body without error would crash C++ on body->
	if sessHasError(nil) {
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
		sessNoteError(nil, ErrGeneric)
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
			if nf := fact.MarkFuncEndLocals(locals); nf != nil {
				cg.FM.GlobalFacts[i] = nf
			} else if sessHasError(nil) {
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
	}
	f.ComputeSummary(summaryEff)
	if sessHasError(nil) {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// Function.cpp:658 / 694 — make_return_const; ERROR_RETURN
	f.MakeReturnConst(opts, probs, r)
	if sessHasError(nil) {
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
	f.markBuilt()
}

// MakeReturnConst mirrors Function::make_return_const.
// Function.cpp:608–615 — depth_protect + need_return_stmt → random constant.
// probs is session Probabilities (C++ singleton); no invent NewProbabilities(opts).
// Function always live; sticky (no invent soft-skip ret_c past hole).
// DepthProtect off / no return needed is complete no-op.
func (f *Function) MakeReturnConst(opts Options, probs *Probabilities, r *Rng) {
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if !opts.DepthProtect || !f.NeedReturnStmt() {
		return
	}
	// Function.cpp:610–612 — assert(return_type); assert simple != eVoid
	if f.ReturnType == nil {
		// assert(return_type) — sticky error for GenerateBody ERROR_RETURN
		sessNoteError(nil, ErrGeneric)
		return
	}
	simple := f.ReturnType.IsSimple()
	// residual ERROR sticky — no invent soft-ret past IsSimple residual
	if sessHasError(nil) {
		return
	}
	if simple && f.ReturnType.Simple() == EVoid {
		// need_return_stmt is false for void; fail closed if called wrongly
		return
	}
	// Function.cpp:612 — Constant::make_random; no invent "0" on nil RNG/fail
	if r == nil {
		// C++ always has process RNG; fail closed with ERROR_RETURN semantics
		sessNoteError(nil, ErrGeneric)
		return
	}
	// session probs; aggregate ret_c needs live tables (nil → fail closed, no invent)
	f.RetConst = MakeRandom(f.ReturnType, opts, probs, r)
	// Function.cpp:614 ERROR_RETURN after Constant::make_random
	// sticky error so GenerateBody does not invent Built without ret_c
	if sessHasError(nil) || f.RetConst == nil {
		f.RetConst = nil
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return
	}
}

// returnTypeC is qualified return type (from RV qfer when present).
// Function always has return_type in C++; no soft invent "void" when missing.
// RV present with Type-nil sticky (no invent fall through to ReturnType / void past
// incomplete return-variable type shell).
// Ambient ProcessOptions bridge; emit paths prefer returnTypeCOpts.
func (f *Function) returnTypeC() string {
	return f.returnTypeCOpts(ProcessOptions())
}

// returnTypeCOpts is returnTypeC with explicit session Options (const/volatile asserts).
func (f *Function) returnTypeCOpts(opts Options) string {
	// Function always live at emit; sticky no invent "void" without it
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if f.RV != nil {
		// RV Type* always live when rv is present; Type-nil sticky incomplete
		if f.RV.Type == nil {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		out := f.RV.Qfer.OutputQualifiedTypeOpts(f.RV.Type, opts)
		// residual ERROR sticky — no invent soft-empty return past OutputQualifiedType residual
		if sessHasError(nil) {
			return ""
		}
		if out == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		return out
	}
	if f.ReturnType != nil {
		cn := f.ReturnType.CName()
		// residual ERROR sticky — no invent soft-empty return past CName residual
		if sessHasError(nil) {
			return ""
		}
		if cn == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		return cn
	}
	// incomplete IR sticky — no invent void
	sessNoteError(nil, ErrGeneric)
	return ""
}

// paramListC emits parameter list with qualified types.
// Function.cpp:501–512 — empty → void.
// opts nil uses ProcessOptions for arg_structs/arg_unions asserts.
func (f *Function) paramListC() string {
	return f.paramListCOpts(ProcessOptions())
}

// paramListCOpts returns param C list, or "" if IR violates arg_structs/arg_unions asserts.
// Function always live at emit; nil shell sticky empty (no invent "void" param list
// past missing Function IR). Empty Param is complete "void".
func (f *Function) paramListCOpts(opts Options) string {
	if f == nil {
		sessNoteError(nil, ErrGeneric)
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
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// Function.cpp:489–491 — assert(!arg_structs → not struct; same unions) sticky
		if !opts.ArgStructs && p.Type.IsStruct() {
			// residual ERROR sticky — no invent soft-empty param past IsStruct residual
			if sessHasError(nil) {
				return ""
			}
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue param past IsStruct residual false
		if sessHasError(nil) {
			return ""
		}
		if !opts.ArgUnions && p.Type.IsUnion() {
			// residual ERROR sticky — no invent soft-empty param past IsUnion residual
			if sessHasError(nil) {
				return ""
			}
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue param past IsUnion residual false
		if sessHasError(nil) {
			return ""
		}
		// Variable always has live name + qualified type; sticky no invent "int " / " p"
		ty := p.Qfer.OutputQualifiedTypeOpts(p.Type, opts)
		// residual ERROR sticky — no invent soft-continue later params past OutputQualifiedType residual
		if sessHasError(nil) {
			return ""
		}
		if ty == "" || p.Name == "" {
			sessNoteError(nil, ErrGeneric)
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
// Function.cpp:516–531 — optional inline/static + qualified return + name(params).
func (f *Function) OutputHeader(forceStatic bool) string {
	return f.OutputHeaderOpts(forceStatic, ProcessOptions())
}

// OutputHeaderOpts is OutputHeader with explicit Options for return/arg struct/union asserts.
func (f *Function) OutputHeaderOpts(forceStatic bool, opts Options) string {
	// Function always live at emit; sticky no invent "int (void)" without it
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Function.cpp:517–520 — assert no return struct/union when options off
	// sticky fail closed: empty header (no invent alternate scalar return)
	// RV present with Type-nil sticky (no invent skip option checks past incomplete rv type)
	rt := f.ReturnType
	if f.RV != nil {
		if f.RV.Type == nil {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		rt = f.RV.Type
	}
	if rt != nil {
		if !opts.ReturnStructs && rt.IsStruct() {
			// residual ERROR sticky — no invent soft-empty header past IsStruct residual
			if sessHasError(nil) {
				return ""
			}
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue header past IsStruct residual false
		if sessHasError(nil) {
			return ""
		}
		if !opts.ReturnUnions && rt.IsUnion() {
			// residual ERROR sticky — no invent soft-empty header past IsUnion residual
			if sessHasError(nil) {
				return ""
			}
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		// residual ERROR sticky — no invent soft-continue header past IsUnion residual false
		if sessHasError(nil) {
			return ""
		}
	}
	// Function always has a live name; sticky no invent "int (void)" without name
	if f.Name == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	params := f.paramListCOpts(opts)
	// residual ERROR sticky — no invent soft-empty header past paramList residual
	if sessHasError(nil) {
		return ""
	}
	if params == "" {
		// assert-path sticky fail closed on forbidden/incomplete params
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	rtName := f.returnTypeCOpts(opts)
	// residual ERROR sticky — no invent soft-empty header past returnTypeC residual
	if sessHasError(nil) {
		return ""
	}
	if rtName == "" {
		// incomplete return type IR sticky — no invent void header
		sessNoteError(nil, ErrGeneric)
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
	// Function.cpp:527–530
	b.WriteString(rtName)
	b.WriteString(" ")
	b.WriteString(f.Name)
	b.WriteString("(")
	b.WriteString(params)
	b.WriteString(")")
	return b.String()
}

// OutputForwardDecl emits a C prototype.
// Function.cpp:555–561 — builtins emit nothing (compiler-provided).
func (f *Function) OutputForwardDecl() string {
	return f.OutputForwardDeclOpts(false, nil, false)
}

// OutputForwardDeclOpts adds optional func __attribute__ and force_static.
// Function.cpp:547–553 — OutputHeader + attrs + ";".
// Function always live at emit; sticky empty (no invent bare ";" past hole).
// Builtins are complete empty (compiler-provided; not incomplete IR).
func (f *Function) OutputForwardDeclOpts(forceStatic bool, r *Rng, withAttrs bool) string {
	return f.OutputForwardDeclWith(forceStatic, r, withAttrs, ProcessOptions())
}

// OutputForwardDeclWith is OutputForwardDeclOpts with explicit session Options.
func (f *Function) OutputForwardDeclWith(forceStatic bool, r *Rng, withAttrs bool, opts Options) string {
	return f.OutputForwardDeclWithSess(nil, forceStatic, r, withAttrs, opts)
}

// OutputForwardDeclWithSess is OutputForwardDeclWith on an explicit session bag.
func (f *Function) OutputForwardDeclWithSess(sess *Session, forceStatic bool, r *Rng, withAttrs bool, opts Options) string {
	if f == nil {
		sessNoteError(sess, ErrGeneric)
		return ""
	}
	if f.IsBuiltin {
		return ""
	}
	s := f.OutputHeaderOpts(forceStatic, opts)
	// residual ERROR sticky — no invent bare ";" past OutputHeader residual
	if sessHasError(nil) {
		return ""
	}
	// incomplete header IR sticky — no invent bare ";"
	if s == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if withAttrs && r != nil {
		if ag := EnsureFuncAttrGeneratorSess(sess); ag != nil {
			s += ag.Output(r)
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
func (f *Function) OutputHeaderAlias(forceStatic bool) string {
	return f.OutputHeaderAliasOpts(forceStatic, ProcessOptions())
}

// OutputHeaderAliasOpts is OutputHeaderAlias with explicit session Options.
func (f *Function) OutputHeaderAliasOpts(forceStatic bool, opts Options) string {
	// Function always live at emit; sticky incomplete no invent empty alias shell
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Function::alias_name set at create (name + "_alias"); sticky no invent when missing
	if f.AliasName == "" || f.Name == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	rtName := f.returnTypeCOpts(opts)
	if rtName == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	params := f.paramListCOpts(opts)
	if params == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	var b strings.Builder
	if forceStatic {
		b.WriteString("static ")
	}
	b.WriteString(rtName)
	b.WriteString(" ")
	b.WriteString(f.AliasName)
	b.WriteString("(")
	b.WriteString(params)
	b.WriteString(") __attribute__((alias(\"")
	b.WriteString(f.Name)
	b.WriteString("\")))")
	return b.String()
}

// OutputForwardDeclAlias mirrors Function::OutputForwardDeclAlias.
// Function.cpp:555–559 — OutputHeaderAlias + ";".
// Function always live at emit; sticky empty (no invent bare ";" past hole).
// Builtins are complete empty (compiler-provided; not incomplete IR).
func (f *Function) OutputForwardDeclAlias(forceStatic bool) string {
	return f.OutputForwardDeclAliasOpts(forceStatic, ProcessOptions())
}

// OutputForwardDeclAliasOpts is OutputForwardDeclAlias with explicit session Options.
func (f *Function) OutputForwardDeclAliasOpts(forceStatic bool, opts Options) string {
	if f == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if f.IsBuiltin {
		return ""
	}
	s := f.OutputHeaderAliasOpts(forceStatic, opts)
	// incomplete alias header sticky — no invent bare ";"
	if s == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	return s + ";"
}

// Output emits a C function definition (minimal statements).
// Function.cpp:565–598 — builtins emit nothing.
func (f *Function) Output() string {
	return f.OutputOpts(false, false, nil)
}

// OutputOpts adds force_static and optional function attributes on the header.
func (f *Function) OutputOpts(forceStatic, withAttrs bool, r *Rng) string {
	return f.OutputOptsWith(forceStatic, withAttrs, r, ProcessOptions())
}

// OutputOptsWith is OutputOpts with explicit session Options (header asserts + body emit).
func (f *Function) OutputOptsWith(forceStatic, withAttrs bool, r *Rng, opts Options) string {
	return f.OutputOptsWithSess(nil, forceStatic, withAttrs, r, opts)
}

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
	hdr := f.OutputHeaderOpts(forceStatic, opts)
	// residual ERROR sticky — no invent soft-continue body past OutputHeader residual
	if sessHasError(nil) {
		return ""
	}
	if hdr == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	s := ""
	// Function.cpp:567 — output_comment_line separator
	s += OutputCommentLine("------------------------------------------", false, f.EmitConcise)
	// Function.cpp:568–570 — feffect.Output when !concise
	if !f.EmitConcise {
		s += f.FEffect.CommentOutput()
		// residual ERROR sticky — no invent soft-continue past CommentOutput residual
		if sessHasError(nil) {
			return ""
		}
	}
	s += hdr
	if withAttrs && r != nil {
		if ag := EnsureFuncAttrGeneratorSess(sess); ag != nil {
			s += ag.Output(r)
		}
		// residual ERROR sticky — no invent soft-continue body past attr residual
		if sessHasError(sess) {
			return ""
		}
	}
	s += "\n"
	// Function.cpp:575–598 — depth_protect + body + else ret_c always live together
	// sticky no invent header-only / empty-body shells (C++ would dereference body)
	if f.Body == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// indent 0: function body braces at column 0 (Block::Output / DefaultOutputMgr style).
	bodyOut := f.Body.OutputOpts(0, opts)
	// residual ERROR sticky — no invent soft-continue past Body.Output residual
	if sessHasError(nil) {
		return ""
	}
	if bodyOut == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if f.DepthProtect && f.RetConst != nil {
		// Function.cpp:575–597 — if (DEPTH…) body else return ret_c
		// both body Output and ret_c value always live; sticky no invent if/else or "return ;"
		retVal := f.RetConst.Value
		if retVal == "" {
			// incomplete ret_c sticky — body only, no invent depth if/else
			sessNoteError(nil, ErrGeneric)
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
