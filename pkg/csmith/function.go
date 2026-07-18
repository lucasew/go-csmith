// Upstream: Function.h / Function.cpp
// (RandomFunctionName, RandomReturnType, make_random_signature, make_first, GenerateParameterList).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Function mirrors Function (signature + body block; DFA/facts deferred).
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
	IsBuilt    bool // BuildState::Built after GenerateBody/make_first
	// Labels legacy end-of-body targets (superseded by Stmt SourceLabel / StmtLabel).
	Labels []string
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
	// inline flip if enabled
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
	}
	if list != nil {
		list.Funcs = append(list.Funcs, f)
	}
	return f
}

// MakeFirst mirrors Function::make_first without ExtensionMgr params and without DFA.
// Function.cpp:443–477 — RandomReturnType, rv qfer via random_qualifiers(ty) no-context,
// empty param list when extension null, GenerateBody.
func MakeFirst(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	sym *GenSym,
	tables *ExprTables,
	stmtTab *ThresholdTable,
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
	ty := RandomReturnType(r, probs, env, opts)
	name := RandomFunctionName(sym)
	f := &Function{Name: name, ReturnType: ty}
	// CVQualifiers::random_qualifiers(ty) — no context, no_volatile
	retQ := RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)
	// ExtensionMgr null → no params
	// GenerateBody
	cg := WithFunc(f, EmptyEffect())
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
	f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	f.IsBuilt = true
	if opts.InlineFunction && r.RndFlipcoin(uint32(probs.Single(PInlineFunctionProb))) {
		f.IsInlined = true
	}
	// f already registered on list before body generation
	return f
}

// GenerateBody mirrors Function::GenerateBody — body = Block::make_random.
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
	cg := prev
	cg.CurrentFunc = f
	if prev.Funcs != nil {
		cg.Funcs = prev.Funcs
	}
	f.Body = MakeRandomBlock(r, opts, probs, vs, tables, stmtTab, cg, false)
	f.IsBuilt = true
}

// OutputForwardDecl emits a C prototype.
func (f *Function) OutputForwardDecl() string {
	if f == nil {
		return ""
	}
	ret := "void"
	if f.ReturnType != nil {
		ret = f.ReturnType.CName()
	}
	s := ret + " " + f.Name + "("
	for i, p := range f.Param {
		if i > 0 {
			s += ", "
		}
		if p != nil && p.Type != nil {
			s += p.Type.CName() + " " + p.Name
		}
	}
	if len(f.Param) == 0 {
		s += "void"
	}
	return s + ");"
}

// Output emits a C function definition (minimal statements).
func (f *Function) Output() string {
	if f == nil {
		return ""
	}
	ret := "void"
	if f.ReturnType != nil {
		ret = f.ReturnType.CName()
	}
	s := ""
	if f.IsInlined {
		s += "inline "
	}
	s += ret + " " + f.Name + "("
	for i, p := range f.Param {
		if i > 0 {
			s += ", "
		}
		if p != nil && p.Type != nil {
			// output first quals simply
			if p.IsConst() {
				s += "const "
			}
			if p.IsVolatile() {
				s += "volatile "
			}
			s += p.Type.CName() + " " + p.Name
		}
	}
	if len(f.Param) == 0 {
		s += "void"
	}
	s += ")\n"
	if f.Body != nil {
		// indent 0: function body braces at column 0 (Block::Output / DefaultOutputMgr style).
		// Labels now live on statements (back-edge) or StmtLabel markers (forward).
		s += f.Body.Output(0)
	} else {
		s += "{\n}\n"
	}
	return s
}
