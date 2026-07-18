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
func RandomFunctionName(sym *GenSym) string {
	if sym == nil {
		sym = &GenSym{}
	}
	return sym.Next("func_")
}

// RandomReturnType mirrors Function.cpp RandomReturnType → Type::choose_random.
// Function.cpp:256–259. When env is nil/empty, fall back to nonvoid simple.
func RandomReturnType(r *Rng, probs *Probabilities, env *TypeEnv, opts Options) *Type {
	if env != nil && len(env.AllTypes) > 0 {
		return env.ChooseRandom(r, opts, probs, false)
	}
	st := ChooseRandomNonvoidSimple(r, probs)
	return GetSimpleType(st)
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
	if r == nil {
		r = NewRng(0)
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
	}
	name := RandomFunctionName(sym)
	f := &Function{Name: name, ReturnType: retType}
	// rv dummy: CVQualifiers::random_qualifiers(type, READ, cg, true) if qfer nil
	var retQ CVQualifiers
	if qfer == nil {
		retQ = RandomQualifiersDefaultProbs(retType, AccessRead, cg, true, opts, probs, r)
	} else {
		// qfer->random_qualifiers(true, READ, cg) deferred → use no_volatile random from type
		retQ = RandomQualifiersDefaultProbs(retType, AccessRead, cg, true, opts, probs, r)
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

// MakeFirst mirrors Function::make_first.
// Function.cpp:443–477 — FactMgr, empty params (no ExtensionMgr), GenerateBody.
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
	if r == nil {
		r = NewRng(0)
	}
	if probs == nil {
		probs = NewProbabilities(opts)
	}
	var env *TypeEnv
	if list != nil {
		env = list.Types
	}
	ty := RandomReturnType(r, probs, env, opts)
	name := RandomFunctionName(sym)
	f := &Function{Name: name, ReturnType: ty}
	// CVQualifiers::random_qualifiers(ty) — no context, no_volatile
	retQ := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)
	// FactMgr for this function (Function.cpp:457–458)
	var fm *FactMgr
	if fmMap != nil {
		fm = fmMap.ForFunc(f)
	} else {
		fm = NewFactMgr(f)
	}
	// seed existing globals
	if vs != nil {
		for _, gv := range vs.GlobalList {
			fm.AddNewVarFact(gv)
		}
	}
	// ExtensionMgr null → no params
	// GenerateBody
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if list != nil {
		cg = cg.WithFuncList(list)
		if list.Types != nil {
			cg.Types = list.Types
		}
	}
	if env != nil {
		cg.Types = env
	}
	// register f before body so recursive choose_func can see it
	if list != nil {
		list.Funcs = append(list.Funcs, f)
	}
	// Function.cpp:631 — Building while generating body
	f.BuildState = BuildBuilding
	// pointer params start as tbd (Function.cpp:637–641)
	for _, p := range f.Param {
		if p != nil && p.IsPointer() {
			if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
				fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointTo(p, TBDPtr))
			}
		}
	}
	// body effect accum for compute_summary / feffect
	bodyEff := EmptyEffect()
	cg.EffectAccum = &bodyEff
	f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	f.DepthProtect = opts.DepthProtect
	f.EmitConcise = opts.Concise
	// mark pointees that were locals as dead after function (mark_func_end subset)
	if f.Body != nil {
		var locals []*Variable
		for _, blk := range f.Blocks {
			if blk != nil {
				locals = append(locals, blk.LocalVars...)
			}
		}
		if len(locals) > 0 {
			for i, fact := range fm.GlobalFacts {
				if nf := fact.MarkFuncEndLocals(locals); nf != nil {
					fm.GlobalFacts[i] = nf
				}
			}
		}
	}
	// FactMgr::find_dangling_global_ptrs (Function.cpp:472–473)
	if opts.DanglingGlobalPointers {
		fm.FindDanglingGlobalPtrs(f)
	}
	// Function::make_return_const — Function.cpp:608–615
	if opts.DepthProtect && f.NeedReturnStmt() {
		f.RetConst = MakeRandom(f.ReturnType, opts, r)
	}
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
	}
	// Function.cpp:652–658 / compute_summary — referenced ptrs + external effect
	f.ComputeSummary(bodyEff)
	f.markBuilt()
	return f
}

// GenerateBody mirrors Function::GenerateBody — body = Block::make_random.
// Function.cpp:626–663 — Unbuilt→Building→Built; pointer param tbd facts.
func (f *Function) GenerateBody(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	prev CGContext,
) {
	if f == nil {
		return
	}
	// ignore regenerate if already building/built (Function.cpp:626–629)
	if f.BuildState != BuildUnbuilt {
		return
	}
	f.BuildState = BuildBuilding
	cg := prev
	cg.CurrentFunc = f
	if prev.Funcs != nil {
		cg.Funcs = prev.Funcs
	}
	// ensure FactMgr when caller did not attach one
	if cg.FM == nil {
		cg.FM = NewFactMgr(f)
	}
	// pointer params → tbd (Function.cpp:637–641)
	for _, p := range f.Param {
		if p != nil && p.IsPointer() {
			if FindRelatedPointTo(cg.FM.GlobalFacts, p) == nil {
				cg.FM.GlobalFacts = append(cg.FM.GlobalFacts, MakeFactPointTo(p, TBDPtr))
			}
		}
	}
	bodyEff := EmptyEffect()
	if cg.EffectAccum == nil {
		cg.EffectAccum = &bodyEff
	} else {
		bodyEff = *cg.EffectAccum
		cg.EffectAccum = &bodyEff
	}
	f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	f.DepthProtect = opts.DepthProtect
	f.EmitConcise = opts.Concise
	// mark_func_end: locals die after function
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
	if opts.DepthProtect && f.NeedReturnStmt() {
		f.RetConst = MakeRandom(f.ReturnType, opts, r)
	}
	// Function.cpp:691 — compute_summary after body
	if cg.EffectAccum != nil {
		f.ComputeSummary(*cg.EffectAccum)
	} else {
		f.ComputeSummary(bodyEff)
	}
	f.markBuilt()
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

// OutputForwardDecl emits a C prototype.
func (f *Function) OutputForwardDecl() string {
	if f == nil {
		return ""
	}
	s := f.returnTypeC() + " " + f.Name + "(" + f.paramListC() + ");"
	return s
}

// Output emits a C function definition (minimal statements).
func (f *Function) Output() string {
	if f == nil {
		return ""
	}
	s := ""
	// Function.cpp:568–570 — feffect.Output when !concise
	if !f.EmitConcise {
		s += f.FEffect.CommentOutput()
	}
	if f.IsInlined {
		s += "inline "
	}
	// Function.cpp:528 — get_prefixed_name(name); default random returns name
	s += f.returnTypeC() + " " + f.Name + "(" + f.paramListC() + ")\n"
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
