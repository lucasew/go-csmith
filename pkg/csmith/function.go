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
	Name       string
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
func (f *Function) PairedFactMgr() *FactMgr {
	if f == nil {
		return nil
	}
	return f.factMgr
}

// ensurePairedFactMgr returns the paired FactMgr, creating once at signature-time
// semantics (Function.cpp FMList.push_back(new FactMgr(f))).
func (f *Function) ensurePairedFactMgr() *FactMgr {
	if f == nil {
		return nil
	}
	if f.factMgr == nil {
		f.factMgr = NewFactMgr(f)
	}
	return f.factMgr
}

// IsEffectKnown mirrors Function::is_effect_known — true only when Built.
// Function.h:96–97.
func (f *Function) IsEffectKnown() bool {
	return f != nil && f.BuildState == BuildBuilt
}

// markBuilt sets Built state and IsBuilt flag.
func (f *Function) markBuilt() {
	if f == nil {
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
	_ = sym
	return Gensym("func_")
}

// RandomReturnType mirrors Function.cpp RandomReturnType → Type::choose_random.
// Function.cpp:256–259 — no soft invent nonvoid simple when AllTypes empty.
func RandomReturnType(r *Rng, probs *Probabilities, env *TypeEnv, opts Options) *Type {
	// Type::choose_random requires AllTypes; ERROR_GUARD path → nil
	if env == nil || len(env.AllTypes) == 0 {
		return nil
	}
	return env.ChooseRandom(r, opts, probs, false)
}

// ParamListProbability mirrors Function.cpp ParamListProbability → rnd_upto(max_params).
func ParamListProbability(r *Rng, opts Options) uint32 {
	if r == nil {
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
	// Function.cpp:401+ — always has RNG; no soft invent NewRng(0)
	if r == nil {
		return nil
	}
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	if probs == nil {
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
		if retType == nil || HasError() {
			return nil
		}
	}
	// Function.cpp:407 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunction, nullptr)
	if DepthGuardByType(opts, DtFunction) == BadDepth {
		return nil
	}
	// Function.cpp:408 ERROR_GUARD after DEPTH_GUARD
	if HasError() {
		return nil
	}
	name := RandomFunctionName(sym)
	f := &Function{Name: name, AliasName: name + "_alias", ReturnType: retType}
	// rv dummy: CVQualifiers::random_qualifiers(type, READ, cg, true) if qfer nil;
	// else qfer->random_qualifiers(true, READ, cg) (Function.cpp:416–418).
	var retQ CVQualifiers
	if qfer == nil {
		retQ = RandomQualifiersDefaultProbs(retType, AccessRead, cg, true, opts, probs, r)
	} else {
		retQ = qfer.RandomQualifiersFrom(true, AccessRead, cg, opts, probs, r)
	}
	// Function.cpp:419 ERROR_GUARD after random_qualifiers
	if HasError() {
		return nil
	}
	f.RV = CreateVariableQfer(name+"_rv", retType, retQ)
	// Function.cpp:419–420 — CreateVariable + ERROR_GUARD path; no soft invent signature without rv
	if f.RV == nil || HasError() {
		return nil
	}
	// GenerateParameterList: for i=0; i<=max; i++
	max := ParamListProbability(r, opts)
	if HasError() {
		return nil
	}
	for i := uint32(0); i <= max; i++ {
		vs.GenerateParameterVariable(f, r)
		// ERROR_RETURN style from GenerateParameterVariable
		if HasError() {
			return nil
		}
	}
	// Function.cpp:422 — FMList.push_back(new FactMgr(f)); always at signature
	f.ensurePairedFactMgr()
	// inline flip if enabled
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
	}
	if HasError() {
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
	f := MakeRandomSignature(r, opts, probs, vs, sym, cg, retType, qfer, list)
	// Function.cpp:434 ERROR_GUARD after signature
	if f == nil || HasError() {
		return nil
	}
	// Function.cpp:422 FMList entry from signature — get_fact_mgr_for_func (no invent second)
	fm := f.PairedFactMgr()
	if fm == nil {
		return nil
	}
	if vs != nil {
		for _, gv := range vs.GlobalList {
			fm.AddNewVarFact(gv)
		}
	}
	bodyCG := cg
	bodyCG.CurrentFunc = f
	bodyCG.FM = fm
	if list != nil {
		bodyCG = bodyCG.WithFuncList(list)
	}
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, bodyCG)
	// Function.cpp:436 ERROR_GUARD after GenerateBody
	if HasError() {
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
	// Function.cpp:443+ — always has RNG; no soft invent NewRng(0)
	if r == nil {
		return nil
	}
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	if probs == nil {
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
	if ty == nil || HasError() {
		return nil
	}
	name := RandomFunctionName(sym)
	f := &Function{Name: name, AliasName: name + "_alias", ReturnType: ty}
	// Function.cpp:452–453 — CVQualifiers::random_qualifiers(ty); ERROR_GUARD
	retQ := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	if HasError() {
		return nil
	}
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)
	// Function.cpp:453 — CreateVariable + ERROR_GUARD; no soft invent first without rv
	if f.RV == nil || HasError() {
		return nil
	}

	// Function.cpp:457–458 — FactMgr with empty global facts (FMList.push_back)
	fm := f.ensurePairedFactMgr()
	if fmMap != nil {
		// register same instance into session FMList map
		_ = fmMap.ForFunc(f)
	}
	// seed existing globals so first function sees them (generation convenience)
	if vs != nil {
		for _, gv := range vs.GlobalList {
			fm.AddNewVarFact(gv)
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
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if list != nil {
		cg = cg.WithFuncList(list)
	}
	if env != nil {
		cg.Types = env
	}
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, cg)
	// sticky error from body — do not invent Built first function
	if HasError() || f.Body == nil {
		return nil
	}

	// Function.cpp:464–465 — inline flip after body
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
	}
	if HasError() {
		return nil
	}

	// Function.cpp:466 — setup_in_out_maps(true)
	fm.SetupInOutMaps(true)

	// Function.cpp:468–470 — global_facts = map_facts_out[body] + add_back_return_facts
	if f.Body != nil && f.Body.StmID > 0 {
		if out, ok := fm.MapFactsOut[f.Body.StmID]; ok {
			fm.GlobalFacts = CloneFactSlice(out)
		}
		AddBackReturnFacts(f.Body, fm, &fm.GlobalFacts)
	}

	// Function.cpp:472–473 — dangling global pointers
	if opts.DanglingGlobalPointers {
		fm.FindDanglingGlobalPtrs(f)
	}

	// Function.cpp:475 — InitializeAttributes (package generators already cover emission;
	// ensure func attr generator is ready when function attributes enabled)
	if opts.FunctionAttributes {
		_ = EnsureFuncAttrGenerator()
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
		return
	}
	// Function.cpp:626–629 / 668–671 — ignore regenerate
	if f.BuildState != BuildUnbuilt {
		return
	}
	f.BuildState = BuildBuilding

	// Function.cpp:633–634 / 675–676 — CGContext(this, prev.effect_context, &effect_accum)
	bodyEff := EmptyEffect()
	if prev.EffectAccum != nil {
		// known-params path: caller already points EffectAccum at callee accum
		bodyEff = *prev.EffectAccum
	}
	cg := prev
	cg.CurrentFunc = f
	cg.EffectAccum = &bodyEff
	cg.Flags = 0
	// Function.cpp:635 / 677 — extend_call_chain
	cg.ExtendCallChain(prev)
	if prev.Funcs != nil {
		cg.Funcs = prev.Funcs
	}
	// Function.cpp:635 / 674 — get_fact_mgr_for_func(this); no invent NewFactMgr here
	if cg.FM == nil {
		cg.FM = f.PairedFactMgr()
	}
	if cg.FM == nil {
		// get_fact_mgr_for_func returned null — fail closed (no soft invent FM)
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// Function.cpp:680–685 — inherit external no-reads/writes (known-params only)
	if knownParams {
		if rwd := prev.BuildCalleeRWDirective(cg.FM.GlobalFacts); rwd != nil {
			cg.RW = rwd
		}
	}

	// Function.cpp:637–641 — pointer params → tbd (GenerateBody; known-params already handed over)
	if !knownParams {
		for _, p := range f.Param {
			if p != nil && p.IsPointer() {
				if FindRelatedPointTo(cg.FM.GlobalFacts, p) == nil {
					cg.FM.GlobalFacts = append(cg.FM.GlobalFacts, MakeFactPointTo(p, TBDPtr))
				}
			}
		}
	}

	// Function.cpp:643–648 — builtin dummy vs make_random
	// make_dummy_block needs CGContext (fact_in + post_creation); no soft empty shell
	if f.IsBuiltin {
		f.Body = MakeDummyBlockCG(&cg, opts)
	} else {
		f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &cg, false)
	}
	// Function.cpp:647 / 689 — ERROR_RETURN(); body->set_depth_protect
	// sticky error aborts; null body without error would crash C++ on body->
	if HasError() {
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
	if cg.FM != nil && len(f.Blocks) > 0 {
		var locals []*Variable
		for _, blk := range f.Blocks {
			if blk != nil {
				locals = append(locals, blk.LocalVars...)
			}
		}
		for i, fact := range cg.FM.GlobalFacts {
			if nf := fact.MarkFuncEndLocals(locals); nf != nil {
				cg.FM.GlobalFacts[i] = nf
			}
		}
		if opts.DanglingGlobalPointers {
			cg.FM.FindDanglingGlobalPtrs(f)
		}
	}

	// Function.cpp:652–656 — get_referenced_ptrs + feffect from map_stm_effect[body]
	// ComputeSummary collects ReferencedPtrs; prefer map_stm_effect[body] when set
	summaryEff := bodyEff
	if cg.FM != nil && f.Body != nil && f.Body.StmID > 0 {
		if e := cg.FM.GetMapStmEffect(f.Body.StmID); !e.IsEmpty() {
			summaryEff = e
		}
	}
	f.ComputeSummary(summaryEff)

	// Function.cpp:658 / 694 — make_return_const; ERROR_RETURN
	f.MakeReturnConst(opts, probs, r)
	if HasError() {
		f.BuildState = BuildUnbuilt
		f.IsBuilt = false
		return
	}

	// keep EffectAccum in sync for caller of known-params
	if prev.EffectAccum != nil {
		*prev.EffectAccum = bodyEff
	}

	// add_back_return_facts into global_facts (Function.cpp:470 path / handoff)
	if cg.FM != nil && f.Body != nil {
		AddBackReturnFacts(f.Body, cg.FM, &cg.FM.GlobalFacts)
	}
	// Function.cpp:661–662 — Mark Built
	f.markBuilt()
}

// MakeReturnConst mirrors Function::make_return_const.
// Function.cpp:608–615 — depth_protect + need_return_stmt → random constant.
// probs is session Probabilities (C++ singleton); no invent NewProbabilities(opts).
func (f *Function) MakeReturnConst(opts Options, probs *Probabilities, r *Rng) {
	if f == nil || !opts.DepthProtect || !f.NeedReturnStmt() {
		return
	}
	// Function.cpp:610–612 — assert(return_type); assert simple != eVoid
	if f.ReturnType == nil {
		return
	}
	if f.ReturnType.IsSimple() && f.ReturnType.Simple() == EVoid {
		// need_return_stmt is false for void; fail closed if called wrongly
		return
	}
	// Function.cpp:612 — Constant::make_random; no invent "0" on nil RNG/fail
	if r == nil {
		return
	}
	// session probs; aggregate ret_c needs live tables (nil → fail closed, no invent)
	f.RetConst = MakeRandom(f.ReturnType, opts, probs, r)
	// Function.cpp:614 ERROR_RETURN after Constant::make_random
	// sticky error left for GenerateBody ERROR_RETURN; nil const is incomplete IR
	if HasError() || f.RetConst == nil {
		f.RetConst = nil
		return
	}
}

// returnTypeC is qualified return type (from RV qfer when present).
func (f *Function) returnTypeC() string {
	if f == nil {
		return "void"
	}
	if f.RV != nil && f.RV.Type != nil {
		return f.RV.Qfer.OutputQualifiedType(f.RV.Type)
	}
	if f.ReturnType != nil {
		return f.ReturnType.CName()
	}
	return "void"
}

// paramListC emits parameter list with qualified types.
// Function.cpp:501–512 — empty → void.
// opts nil uses ProcessOptions for arg_structs/arg_unions asserts.
func (f *Function) paramListC() string {
	return f.paramListCOpts(ProcessOptions())
}

// paramListCOpts returns param C list, or "" if IR violates arg_structs/arg_unions asserts.
func (f *Function) paramListCOpts(opts Options) string {
	if f == nil || len(f.Param) == 0 {
		// Function.cpp:502–504 — assert(Type::void_type); emit void
		return "void"
	}
	var b strings.Builder
	for i, p := range f.Param {
		if p == nil || p.Type == nil {
			// incomplete param IR — fail closed (no invent type name)
			return ""
		}
		// Function.cpp:489–491 — assert(!arg_structs → not struct; same unions)
		if !opts.ArgStructs && p.Type.IsStruct() {
			return ""
		}
		if !opts.ArgUnions && p.Type.IsUnion() {
			return ""
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Qfer.OutputQualifiedType(p.Type))
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
	if f == nil {
		return ""
	}
	// Function.cpp:517–520 — assert no return struct/union when options off
	// fail closed: empty header (no invent alternate scalar return)
	rt := f.ReturnType
	if f.RV != nil && f.RV.Type != nil {
		rt = f.RV.Type
	}
	if rt != nil {
		if !opts.ReturnStructs && rt.IsStruct() {
			return ""
		}
		if !opts.ReturnUnions && rt.IsUnion() {
			return ""
		}
	}
	params := f.paramListCOpts(opts)
	if params == "" {
		// assert-path fail closed on forbidden/incomplete params
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
	b.WriteString(f.returnTypeC())
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
func (f *Function) OutputForwardDeclOpts(forceStatic bool, r *Rng, withAttrs bool) string {
	if f == nil || f.IsBuiltin {
		return ""
	}
	s := f.OutputHeader(forceStatic)
	if withAttrs && r != nil {
		s += EnsureFuncAttrGenerator().Output(r)
	}
	s += ";"
	return s
}

// OutputHeaderAlias mirrors Function::OutputHeaderAlias.
// Function.cpp:533–541 — static? + type alias_name(params) __attribute__((alias("name"))).
func (f *Function) OutputHeaderAlias(forceStatic bool) string {
	if f == nil {
		return ""
	}
	alias := f.AliasName
	if alias == "" {
		alias = f.Name + "_alias"
	}
	var b strings.Builder
	if forceStatic {
		b.WriteString("static ")
	}
	b.WriteString(f.returnTypeC())
	b.WriteString(" ")
	b.WriteString(alias)
	b.WriteString("(")
	b.WriteString(f.paramListC())
	b.WriteString(") __attribute__((alias(\"")
	b.WriteString(f.Name)
	b.WriteString("\")))")
	return b.String()
}

// OutputForwardDeclAlias mirrors Function::OutputForwardDeclAlias.
// Function.cpp:555–559 — OutputHeaderAlias + ";".
func (f *Function) OutputForwardDeclAlias(forceStatic bool) string {
	if f == nil || f.IsBuiltin {
		return ""
	}
	return f.OutputHeaderAlias(forceStatic) + ";"
}

// Output emits a C function definition (minimal statements).
// Function.cpp:565–598 — builtins emit nothing.
func (f *Function) Output() string {
	return f.OutputOpts(false, false, nil)
}

// OutputOpts adds force_static and optional function attributes on the header.
func (f *Function) OutputOpts(forceStatic, withAttrs bool, r *Rng) string {
	if f == nil || f.IsBuiltin {
		return ""
	}
	s := ""
	// Function.cpp:567 — output_comment_line separator
	s += OutputCommentLine("------------------------------------------", false, f.EmitConcise)
	// Function.cpp:568–570 — feffect.Output when !concise
	if !f.EmitConcise {
		s += f.FEffect.CommentOutput()
	}
	// Function.cpp:572 — OutputHeader
	s += f.OutputHeader(forceStatic)
	if withAttrs && r != nil {
		s += EnsureFuncAttrGenerator().Output(r)
	}
	s += "\n"
	// Function.cpp:575–598 — depth_protect wraps body
	if f.DepthProtect {
		s += "if (DEPTH < MAX_DEPTH) \n"
	}
	// Function.cpp:591 — body->Output always; no soft invent empty "{}" when Body missing
	if f.Body != nil {
		// indent 0: function body braces at column 0 (Block::Output / DefaultOutputMgr style).
		s += f.Body.Output(0)
	}
	if f.DepthProtect {
		// Function.cpp:593–597 — else; ret_c->Output; no soft invent "0" when ret_c null
		if f.RetConst != nil {
			s += "else\n"
			s += "return "
			s += f.RetConst.Value
			s += ";\n"
		}
	}
	return s
}
