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
// Function.cpp:249 — gensym uses process-wide counter; no soft invent private GenSym{}.
func RandomFunctionName(sym *GenSym) string {
	if sym == nil {
		return Gensym("func_")
	}
	return sym.Next("func_")
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
	if probs == nil {
		probs = NewProbabilities(opts)
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
		if retType == nil {
			return nil
		}
	}
	// Function.cpp:407 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunction, nullptr)
	if DepthGuardByType(opts, DtFunction) == BadDepth {
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
	f.RV = CreateVariableQfer(name+"_rv", retType, retQ)
	// GenerateParameterList: for i=0; i<=max; i++
	max := ParamListProbability(r, opts)
	for i := uint32(0); i <= max; i++ {
		vs.GenerateParameterVariable(f, r)
	}
	// Function.cpp:422 — FMList.push_back(new FactMgr(f))
	if cg.FM == nil {
		// attach when map provided via FuncList path later
	}
	// inline flip if enabled
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
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
	if f == nil {
		return nil
	}
	// attach FactMgr if not already from map
	if cg.FM == nil || cg.FM.Func != f {
		cg.FM = NewFactMgr(f)
		if vs != nil {
			for _, gv := range vs.GlobalList {
				cg.FM.AddNewVarFact(gv)
			}
		}
	}
	bodyCG := cg
	bodyCG.CurrentFunc = f
	bodyCG.FM = cg.FM
	if list != nil {
		bodyCG = bodyCG.WithFuncList(list)
	}
	f.GenerateBody(r, opts, probs, vs, tables, stmtTab, bodyCG)
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
	if probs == nil {
		probs = NewProbabilities(opts)
	}
	// Type::AllTypes is process-global in C++; session Types on list or vs
	var env *TypeEnv
	if list != nil {
		env = list.Types
	}
	if env == nil && vs != nil {
		env = vs.Types
	}
	// Function.cpp:444–454
	ty := RandomReturnType(r, probs, env, opts)
	// ERROR_GUARD when choose_random fails (empty AllTypes)
	if ty == nil {
		return nil
	}
	name := RandomFunctionName(sym)
	f := &Function{Name: name, AliasName: name + "_alias", ReturnType: ty}
	// CVQualifiers::random_qualifiers(ty) — no context
	retQ := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)

	// Function.cpp:457–458 — FactMgr with empty global facts
	var fm *FactMgr
	if fmMap != nil {
		fm = fmMap.ForFunc(f)
	} else {
		fm = NewFactMgr(f)
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

	// Function.cpp:464–465 — inline flip after body
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
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
	if cg.FM == nil {
		cg.FM = NewFactMgr(f)
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
	if f.IsBuiltin {
		f.Body = MakeDummyBlock(f)
	} else {
		f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, &cg, false)
	}
	// Function.cpp:650 / 690 — body->set_depth_protect(true)
	if f.Body != nil {
		f.Body.EmitDepthProtect = true
	}
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

	// Function.cpp:658 / 694 — make_return_const
	f.MakeReturnConst(opts, r)

	// keep EffectAccum in sync for caller of known-params
	if prev.EffectAccum != nil {
		*prev.EffectAccum = bodyEff
	}

	// add_back_return_facts into global_facts (Function.cpp:470 path / handoff)
	if cg.FM != nil && f.Body != nil {
		AddBackReturnFacts(f.Body, cg.FM, &cg.FM.GlobalFacts)
	}
	f.markBuilt()
}

// MakeReturnConst mirrors Function::make_return_const.
// Function.cpp:608–615 — depth_protect + need_return_stmt → random constant.
func (f *Function) MakeReturnConst(opts Options, r *Rng) {
	if f == nil || !opts.DepthProtect || !f.NeedReturnStmt() {
		return
	}
	if f.ReturnType != nil && f.ReturnType.IsSimple() && f.ReturnType.Simple() == EVoid {
		return
	}
	f.RetConst = MakeRandom(f.ReturnType, opts, r)
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
func (f *Function) paramListC() string {
	if f == nil || len(f.Param) == 0 {
		return "void"
	}
	var b strings.Builder
	for i, p := range f.Param {
		if i > 0 {
			b.WriteString(", ")
		}
		if p != nil && p.Type != nil {
			b.WriteString(p.Qfer.OutputQualifiedType(p.Type))
			b.WriteString(" ")
			b.WriteString(p.Name)
		}
	}
	return b.String()
}

// OutputHeader mirrors Function::OutputHeader.
// Function.cpp:516–531 — optional inline/static + qualified return + name(params).
func (f *Function) OutputHeader(forceStatic bool) string {
	if f == nil {
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
	b.WriteString(f.paramListC())
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
	if f.Body != nil {
		// indent 0: function body braces at column 0 (Block::Output / DefaultOutputMgr style).
		s += f.Body.Output(0)
	} else {
		s += "{\n}\n"
	}
	if f.DepthProtect {
		s += "else\n"
		s += "return "
		if f.RetConst != nil {
			s += f.RetConst.Value
		} else {
			s += "0"
		}
		s += ";\n"
	}
	return s
}
