// Upstream: FunctionInvocation.cpp / FunctionInvocationUser.cpp (make_random user/std paths).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// Invocation is a minimal FunctionInvocation (user call or binary/unary op).
type Invocation struct {
	// User is non-nil for program-defined calls.
	User *Function
	// Failed mirrors FunctionInvocation::failed.
	Failed bool
	// IsStd is true for unary/binary "stdlib" ops (not user).
	IsStd bool
	// UnaryOp / BinaryOp for std ops; BinaryOp uses left/right.
	IsUnary bool
	Unary   string // "+", "-", "~", "!"
	Binary  string // "+", "-", etc.
	// Args are parameter expressions (operands).
	Args []*Expression
	// Safe is set for std binary/unary under avoid_signed_overflow.
	Safe *SafeOpFlags
	// Tmp1/Tmp2 are math_notmp temporary names (FunctionInvocationBinary tmp_var1/2).
	Tmp1, Tmp2 string
	// MathNoTmp mirrors CGOptions::math_notmp for Output.
	MathNoTmp bool
	// OutSafeMath mirrors CGOptions::avoid_signed_overflow at emit time.
	OutSafeMath bool
	// OutIdentifyWrappers mirrors CGOptions::identify_wrappers.
	OutIdentifyWrappers bool
	// OutSafeMathWrappers is --safe-math-wrapper id list (empty = all allowed).
	OutSafeMathWrappers string
	// PtrCmp mirrors FunctionInvocation::ptr_cmp — pointer ==/!= operands.
	// FunctionInvocation.cpp:355.
	PtrCmp bool
}

// setOutOpts snapshots CGOptions bits needed by Output (no live Options at emit).
func (fi *Invocation) setOutOpts(opts Options) {
	if fi == nil {
		return
	}
	fi.OutSafeMath = opts.SafeMath
	fi.OutIdentifyWrappers = opts.IdentifyWrappers
	fi.OutSafeMathWrappers = opts.SafeMathWrappers
}

// wrapperOpts reconstructs Options for SafeMathWrapperAllowed checks.
func (fi *Invocation) wrapperOpts() Options {
	return Options{SafeMathWrappers: fi.OutSafeMathWrappers}
}

// Output C for the invocation.
// FunctionInvocationUnary::Output / FunctionInvocationBinary::Output —
// safe_* when avoid_signed_overflow + wrapper allowed; float unary uses standard op.
func (fi *Invocation) Output() string {
	if fi == nil || fi.Failed {
		return "/*bad_call*/"
	}
	if fi.User != nil {
		var b strings.Builder
		b.WriteString(fi.User.Name)
		b.WriteString("(")
		for i, a := range fi.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			if a != nil {
				b.WriteString(a.Output())
			} else {
				b.WriteString("0")
			}
		}
		b.WriteString(")")
		return b.String()
	}
	if fi.IsStd {
		a0, a1 := "0", "0"
		if len(fi.Args) >= 1 && fi.Args[0] != nil {
			a0 = fi.Args[0].Output()
		}
		if len(fi.Args) >= 2 && fi.Args[1] != nil {
			a1 = fi.Args[1].Output()
		}
		if fi.IsUnary && len(fi.Args) >= 1 {
			return fi.outputUnary(a0)
		}
		if !fi.IsUnary && len(fi.Args) >= 2 {
			return fi.outputBinary(a0, a1)
		}
	}
	return "/*invoke*/"
}

// outputUnary mirrors FunctionInvocationUnary::Output.
// FunctionInvocationUnary.cpp:192–243.
func (fi *Invocation) outputUnary(a0 string) string {
	// FunctionInvocationUnary.cpp:200–224 — eMinus + avoid_signed_overflow
	if fi.Unary == "-" && fi.Safe != nil && fi.OutSafeMath {
		// float size: standard minus (no safe unary float func)
		// FunctionInvocationUnary.cpp:203 / 220–223
		if fi.Safe.Size == SafeFloat {
			return fmt.Sprintf("(-%s)", a0)
		}
		fname := fi.Safe.UnaryMinusFuncName()
		id := SafeOpFlagsToID(fname)
		// FunctionInvocationUnary.cpp:208–218 — safe_math_wrapper filter
		if SafeMathWrapperAllowed(fi.wrapperOpts(), id) {
			var b strings.Builder
			b.WriteString("(")
			b.WriteString(fname)
			b.WriteString("(")
			if fi.MathNoTmp && fi.Tmp1 != "" {
				b.WriteString(fi.Tmp1)
				b.WriteString(", ")
			}
			b.WriteString(a0)
			if fi.OutIdentifyWrappers {
				b.WriteString(", ")
				b.WriteString(Int2Str(id))
			}
			b.WriteString("))")
			return b.String()
		}
		// wrapper denied → cast + standard (need_cast fallthrough)
		// FunctionInvocationUnary.cpp:226–239
		return fmt.Sprintf("(-(%s)%s)", fi.Safe.SizeToken(), a0)
	}
	// FunctionInvocationUnary.cpp:229–240 — ePlus/eNot/eBitNot or non-safe minus
	if fi.Unary == "-" && fi.Safe != nil && !fi.OutSafeMath {
		// need_cast when Safe flags exist but avoid_signed_overflow off
		return fmt.Sprintf("(-(%s)%s)", fi.Safe.SizeToken(), a0)
	}
	return fmt.Sprintf("(%s(%s))", fi.Unary, a0)
}

// outputBinary mirrors FunctionInvocationBinary::Output.
// FunctionInvocationBinary.cpp:350–426.
func (fi *Invocation) outputBinary(a0, a1 string) string {
	// FunctionInvocationBinary.cpp:357–361 — mutated array subscript add without flags
	if fi.Binary == "+" && fi.Safe == nil {
		return fmt.Sprintf("(%s + %s)", a0, a1)
	}
	// FunctionInvocationBinary.cpp:363–399 — arith/shift + avoid_signed_overflow
	if fi.Safe != nil && SafeOpsBinary(fi.Binary) && fi.OutSafeMath {
		if fname := fi.Safe.BinaryFuncName(fi.Binary); fname != "" {
			id := SafeOpFlagsToID(fname)
			if SafeMathWrapperAllowed(fi.wrapperOpts(), id) {
				var b strings.Builder
				b.WriteString("(")
				b.WriteString(fname)
				b.WriteString("(")
				if fi.MathNoTmp && fi.Tmp1 != "" {
					b.WriteString(fi.Tmp1)
					b.WriteString(", ")
				}
				b.WriteString(a0)
				b.WriteString(", ")
				if fi.MathNoTmp && fi.Tmp2 != "" {
					b.WriteString(fi.Tmp2)
					b.WriteString(", ")
				}
				b.WriteString(a1)
				if fi.OutIdentifyWrappers {
					b.WriteString(", ")
					b.WriteString(Int2Str(id))
				}
				b.WriteString("))")
				return b.String()
			}
			// wrapper denied → cast both operands (need_cast fallthrough)
			// FunctionInvocationBinary.cpp:400–414
			cast := fi.Safe.SizeToken()
			return fmt.Sprintf("((%s)%s %s (%s)%s)", cast, a0, fi.Binary, cast, a1)
		}
	}
	// need_cast when Safe present but SafeMath off for arith/shift
	if fi.Safe != nil && SafeOpsBinary(fi.Binary) && !fi.OutSafeMath {
		cast := fi.Safe.SizeToken()
		return fmt.Sprintf("((%s)%s %s (%s)%s)", cast, a0, fi.Binary, cast, a1)
	}
	return fmt.Sprintf("(%s %s %s)", a0, fi.Binary, a1)
}

// ReachMaxFunctions mirrors Function::reach_max_functions_cnt.
func ReachMaxFunctions(list *FunctionList, opts Options) bool {
	if list == nil {
		return false
	}
	n := 0
	for _, f := range list.Funcs {
		if f != nil && !f.IsBuiltin {
			n++
		}
	}
	return n >= opts.MaxFuncs
}

// ChooseFunc mirrors Function::choose_func — filter by return type convert, then choose_ok style.
// Function.cpp:279+ — skip unbuilt, builtins, effect conflict with context.
// cg may be nil (no conflict filter).
func ChooseFunc(r *Rng, funcs []*Function, ret *Type, exclude *Function) *Function {
	return ChooseFuncContext(r, funcs, ret, exclude, nil, Options{})
}

// ChooseFuncContext is ChooseFunc with CGContext for in_conflict / strict_volatile.
// Function.cpp:279–340 — separate user vs builtin pools; BuiltinFunctionProb pick.
func ChooseFuncContext(r *Rng, funcs []*Function, ret *Type, exclude *Function, cg *CGContext, opts Options) *Function {
	var ok, okBuiltin []*Function
	for _, f := range funcs {
		if f == nil || f == exclude || !f.IsEffectKnown() {
			// is_effect_known() == false for Unbuilt/Building
			continue
		}
		if ret != nil && f.ReturnType != nil && !ret.Match(f.ReturnType, MatchConvert) {
			continue
		}
		if ret != nil && f.ReturnType == nil {
			continue
		}
		// Function.cpp:303–306 — in_conflict with callee feffect
		if cg != nil && cg.InConflict(f.FEffect) {
			continue
		}
		// Function.cpp:307–313 — strict_volatile_rule
		if opts.StrictVolatileRule && cg != nil {
			if !f.FEffect.IsSideEffectFree() && !cg.EffectContext().IsSideEffectFree() {
				continue
			}
		}
		// Function.cpp:318–321 — has_race_with (optional, often commented; we apply)
		if cg != nil && f.FEffect.HasRaceWith(cg.EffectContext()) {
			continue
		}
		if f.IsBuiltin {
			if opts.Builtins {
				okBuiltin = append(okBuiltin, f)
			}
			continue
		}
		ok = append(ok, f)
	}
	// Function.cpp:330–333 — prefer builtin with BuiltinFunctionProb
	if opts.Builtins && len(okBuiltin) > 0 && r != nil {
		p := opts.BuiltinFunctionProb
		if p <= 0 {
			p = 50
		}
		if r.RndFlipcoin(uint32(p)) {
			return getOneFunction(r, okBuiltin)
		}
	}
	if f := getOneFunction(r, ok); f != nil {
		return f
	}
	// fallback builtins if no user funcs
	return getOneFunction(r, okBuiltin)
}

// getOneFunction mirrors Function::get_one_function — random pick.
// Function.cpp:262–276.
func getOneFunction(r *Rng, funcs []*Function) *Function {
	n := len(funcs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return funcs[0]
	}
	if r == nil {
		return funcs[0]
	}
	return funcs[r.RndUpto(uint32(n))]
}

// ExpressionFunctionProbability mirrors ExpressionFuncall.cpp:57–62.
// true → prefer std (unary/binary); false → user function.
func ExpressionFunctionProbability(r *Rng, list *FunctionList, opts Options) bool {
	if ReachMaxFunctions(list, opts) && !opts.Builtins {
		return true
	}
	return r.RndFlipcoin(80)
}

// GetFirstFunction mirrors GetFirstFunction — first entry in FuncList / func_1.
// Function.cpp / FunctionInvocationUser.cpp:274.
func GetFirstFunction(list *FunctionList) *Function {
	if list == nil || len(list.Funcs) == 0 {
		return nil
	}
	return list.Funcs[0]
}

// BuildUserInvocation mirrors FunctionInvocationUser::build_invocation.
// FunctionInvocationUser.cpp:246–303 — params; then revisit or add_external_effect.
func BuildUserInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	list *FunctionList,
	callee *Function,
) *Invocation {
	if callee == nil {
		return &Invocation{Failed: true}
	}
	fi := &Invocation{User: callee}
	// FunctionInvocationUser.cpp:249–270 — running effect context across params
	running := cg.EffectContext()
	// Expression.cpp:213–218 — leaf/user-call args bump depth for subsequent params
	argDepth := cg.ExprDepth
	for _, p := range callee.Param {
		ty := GetIntType()
		var qfer *CVQualifiers
		if p != nil {
			if p.Type != nil {
				ty = p.Type
			}
			q := p.Qfer
			qfer = &q
		}
		paramAccum := EmptyEffect()
		paramCG := cg
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.ExprDepth = argDepth
		arg := MakeRandomParam(r, opts, tables, vs, paramCG, ty, qfer, argDepth, list)
		if arg == nil {
			arg = makeExpressionVariableFlags(r, vs, paramCG, ty, qfer, true, false)
		}
		if arg != nil {
			// FunctionInvocationUser.cpp:190 — check_and_set_cast (lang_cpp)
			arg.CheckAndSetCastOpts(ty, opts)
			if BumpsExprDepth(arg) {
				argDepth++
			}
		}
		fi.Args = append(fi.Args, arg)
		cg.MergeParamContext(paramCG, true)
		running = running.AddEffect(paramAccum)
	}

	// FunctionInvocationUser.cpp:272–301
	fi.Failed = false
	first := GetFirstFunction(list)
	// skip revisit for first function (func_1) — no params, single call, DFA hack
	if callee != first && callee.NeedsRevisit() {
		// FunctionInvocationUser.cpp:277–291 — revisit with accum_eff_context
		effectAccum := EmptyEffect()
		effectContext := cg.EffectContext().AddEffect(callee.AccumEffContext)
		newCG := cg
		newCG.effectContext = effectContext
		newCG.EffectAccum = &effectAccum
		// keep caller FM for global_facts input; RevisitUserInvocation swaps CurrentFunc
		var facts []*FactPointTo
		if cg.FM != nil {
			facts = CloneFactSlice(cg.FM.GlobalFacts)
		}
		ok := RevisitUserInvocation(fi, &facts, &newCG, opts)
		fi.Failed = !ok
		if ok {
			// FunctionInvocationUser.cpp:284–290
			if cg.CurrentBlock() != nil {
				cg.AddVisibleEffectAt(effectAccum, cg.CurrentBlock())
			}
			callee.FEffect = callee.FEffect.AddExternalEffectWithCallers(effectAccum, cg.CallChain)
			if cg.FM != nil {
				cg.FM.GlobalFacts = facts
			}
		}
	} else {
		// FunctionInvocationUser.cpp:293–297 — static effect, no re-analyze
		// add_external_effect(func->get_feffect())
		if callee.IsEffectKnown() {
			cg.AddExternalEffect(callee.FEffect)
		}
	}
	_ = probs
	_ = r
	return fi
}

// BuildInvocationAndFunction mirrors build_invocation_and_function.
// FunctionInvocationUser.cpp:173–241 — signature, params first, handover with args,
// generate_body_with_known_params, return facts, renew_facts, effect handoff.
func BuildInvocationAndFunction(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
	list *FunctionList,
	retType *Type,
) *Invocation {
	if list == nil || ReachMaxFunctions(list, opts) {
		return &Invocation{Failed: true}
	}
	if retType == nil {
		retType = GetIntType()
	}
	// FunctionInvocationUser.cpp:179 — make_random_signature
	callee := MakeRandomSignature(r, opts, probs, vs, &vs.Sym, cg, retType, nil, list)
	if callee == nil {
		return &Invocation{Failed: true}
	}

	// FunctionInvocationUser.cpp:181–197 — build all parameters before body
	fi := &Invocation{User: callee}
	running := cg.EffectContext()
	argDepth := cg.ExprDepth
	for _, p := range callee.Param {
		ty := GetIntType()
		var qfer *CVQualifiers
		if p != nil {
			if p.Type != nil {
				ty = p.Type
			}
			q := p.Qfer
			qfer = &q
		}
		paramAccum := EmptyEffect()
		paramCG := cg
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.ExprDepth = argDepth
		arg := MakeRandomParam(r, opts, tables, vs, paramCG, ty, qfer, argDepth, list)
		if arg == nil {
			arg = makeExpressionVariableFlags(r, vs, paramCG, ty, qfer, true, false)
		}
		if arg != nil {
			// FunctionInvocationUser.cpp:261 — check_and_set_cast (lang_cpp)
			arg.CheckAndSetCastOpts(ty, opts)
			if BumpsExprDepth(arg) {
				argDepth++
			}
		}
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:195–196
		cg.MergeParamContext(paramCG, true)
		running = running.AddEffect(paramAccum)
	}

	// FunctionInvocationUser.cpp:203–206 — hand-over from caller to callee with args
	var callerFM *FactMgr
	if cg.FM != nil {
		callerFM = cg.FM
	}
	calFM := NewFactMgr(callee)
	facts := []*FactPointTo{}
	if callerFM != nil {
		facts = CloneFactSlice(callerFM.GlobalFacts)
	}
	calFM.CallerToCalleeHandover(fi.Args, &facts)
	calFM.GlobalFacts = facts

	// FunctionInvocationUser.cpp:208–210 — generate_body_with_known_params
	effectAccum := EmptyEffect()
	bodyCG := cg
	bodyCG.CurrentFunc = callee
	bodyCG.FM = calFM
	bodyCG.Flags = 0
	bodyCG.EffectAccum = &effectAccum
	// RWDirective + call chain set inside GenerateBodyWithKnownParams
	callee.GenerateBodyWithKnownParams(r, opts, probs, vs, tables, stmtTab, bodyCG)

	// FunctionInvocationUser.cpp:212–215 — ret_facts from body + add_back_return_facts
	retFacts := []*FactPointTo{}
	if callee.Body != nil && callee.Body.StmID > 0 {
		if out, ok := calFM.MapFactsOut[callee.Body.StmID]; ok {
			retFacts = CloneFactSlice(out)
		}
	}
	if len(retFacts) == 0 {
		retFacts = CloneFactSlice(calFM.GlobalFacts)
	}
	if callee.Body != nil {
		AddBackReturnFacts(callee.Body, calFM, &retFacts)
	}
	fi.SaveReturnFacts(retFacts)

	// FunctionInvocationUser.cpp:219 — setup_in_out_maps(true)
	calFM.SetupInOutMaps(true)

	// FunctionInvocationUser.cpp:221 — renew_facts(caller, ret_facts)
	if callerFM != nil {
		RenewFacts(&callerFM.GlobalFacts, retFacts)
		// FunctionInvocationUser.cpp:234–238 — new globals facts
		for _, v := range callee.NewGlobals {
			callerFM.AddNewVarFactAndUpdate(nil, v)
		}
	}

	// FunctionInvocationUser.cpp:223–228 — effect hand-over
	callee.AccumEffContext = callee.AccumEffContext.AddExternalEffect(cg.EffectContext())
	// feffect.add_external_effect(effect_accum, call_chain)
	callee.FEffect = callee.FEffect.AddExternalEffectWithCallers(effectAccum, cg.CallChain)
	// also keep ComputeSummary body effect already applied in GenerateBody
	cg.AddVisibleEffectAt(effectAccum, cg.CurrentBlock())

	// FunctionInvocationUser.cpp:230–233 — new_globals hand-over
	if cg.CurrentFunc != nil && len(callee.NewGlobals) > 0 {
		cg.CurrentFunc.NewGlobals = append(cg.CurrentFunc.NewGlobals, callee.NewGlobals...)
	}

	// FunctionInvocationUser.cpp:240
	callee.VisitedCnt = 1
	return fi
}

// MakeRandomBinaryInvocation mirrors FunctionInvocation::make_random_binary.
// FunctionInvocation.cpp:171–288 — 10% ptr comparison; BINARY_OPS_PROB_FILTER;
// ordered/unordered RHS effect contexts.
func MakeRandomBinaryInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
) *Invocation {
	if typ == nil {
		typ = GetIntType()
	}
	// FunctionInvocation.cpp:174–177 — 10% pointer comparison when derived exist
	if r.RndFlipcoin(10) {
		var env *TypeEnv
		if vs != nil {
			env = vs.Types
		}
		if env == nil {
			env = cg.Types
		}
		if env != nil && env.HasPointerType() {
			if fi := MakeRandomBinaryPtrComparison(r, opts, probs, vs, tables, cg, env); fi != nil {
				return fi
			}
		}
	}
	op := PickBinaryOp(r, opts)
	// float filter if we ever pass float types
	if typ.IsSimple() && typ.Simple() == EFloat && !BinaryOpWorksForFloat(op) {
		op = PickBinaryOp(r, opts)
	}
	opStr := op.BinaryOpC()
	// FunctionInvocation.cpp:188–207 — always SafeOpFlags::make_random_binary; operands use get_lhs/rhs_type
	var flags *SafeOpFlags
	lhsTy, rhsTy := typ, typ
	// C++ always builds flags; CreateFunctionInvocationBinary only allocates tmps for safe_ops
	flags = MakeRandomBinaryKind(r, opts, probs, typ, typ, typ, SafeOpBinary, op)
	if flags != nil {
		lhsTy = flags.LHSType()
		rhsTy = flags.RHSType()
	}
	// non-arith/shift: keep flags for typing but Output ignores safe path (SafeOpsBinary filter)
	if !SafeOpsBinary(opStr) {
		// still keep flags (C++ does); Output uses standard tokens
	}
	// Operands: no nested Function (depth + leaf bias) — avoids exponential recursion.
	// FunctionInvocation.cpp:208–261 — ordered (&&/||) RHS under original effect context;
	// unordered RHS under original + LHS accum.
	// Expression.cpp:213–218 — each leaf/user-call bumps depth for subsequent siblings
	d := cg.ExprDepth
	preLeft := EmptyEffect()
	if cg.EffectAccum != nil {
		preLeft = *cg.EffectAccum
	}
	left := MakeRandomExpression(r, opts, tables, vs, cg, lhsTy, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, cg, lhsTy, nil, true, false, TermConstant, d)
	}
	if BumpsExprDepth(left) {
		d++
	}
	var right *Expression
	if op == BinLShift || op == BinRShift {
		// prefer constant shift amount (FunctionInvocation.cpp:236–244)
		// ShiftByNonConstantProb default 50
		if !r.RndFlipcoin(50) {
			// Constant::make_random_upto(SizeInBytes*8)
			bits := uint32(32)
			if lhsTy != nil {
				if sb := lhsTy.SizeInBytes(); sb > 0 {
					bits = uint32(sb * 8)
				}
			}
			right = &Expression{Term: TermConstant, Con: MakeRandomUpto(bits, r)}
			// constant bumps depth (Expression.cpp:213–218)
			d++
		}
	}
	if right == nil {
		if IsOrderedBinary(op) && cg.EffectAccum != nil {
			// RHS under pre-left context only; merge back (FunctionInvocation.cpp:222–226)
			postLeft := *cg.EffectAccum
			*cg.EffectAccum = preLeft
			// ambient context stays pre-left (effect_context); ordered short-circuit
			right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, MaxTermTypes, d)
			if right == nil {
				right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, TermConstant, d)
			}
			*cg.EffectAccum = MergeEffects(postLeft, *cg.EffectAccum)
		} else {
			// unordered: RHS under original + LHS effects as effect_context
			// FunctionInvocation.cpp:228–234
			rhsCG := cg
			if cg.EffectAccum != nil {
				rhsCG.effectContext = *cg.EffectAccum
			}
			right = MakeRandomExpression(r, opts, tables, vs, rhsCG, rhsTy, nil, true, false, MaxTermTypes, d)
			if right == nil {
				right = MakeRandomExpression(r, opts, tables, vs, rhsCG, rhsTy, nil, true, false, TermConstant, d)
			}
			// fold RHS accum into caller's accum
			if cg.EffectAccum != nil && rhsCG.EffectAccum != nil && rhsCG.EffectAccum != cg.EffectAccum {
				*cg.EffectAccum = cg.EffectAccum.AddEffect(*rhsCG.EffectAccum)
			}
		}
	}
	// avoid div/mod by zero-ish constant: re-pick op excluding div/mod/shift
	if (op == BinMod || op == BinDiv) && right != nil && right.Term == TermConstant && right.Con != nil {
		if right.Con.Value == "0" || right.Con.Value == "1" {
			op = BinAdd
			opStr = op.BinaryOpC()
			// keep flags if still safe-ops
			if flags != nil && !SafeOpsBinary(opStr) {
				flags = nil
			}
		}
	}
	// CompatibleChecker on binary operands when enabled
	if CompatibleCheckExprs(opts, left, right) {
		right = &Expression{Term: TermConstant, Con: MakeInt(1)}
	}
	inv := &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}, Safe: flags}
	inv.setOutOpts(opts)
	// CreateFunctionInvocationBinary tmp vars when math_notmp (FunctionInvocationBinary.cpp:59–75)
	// C++ always creates tmps for safe_ops; we allocate only when MathNoTmp will emit them.
	if flags != nil && opts.MathNoTmp && SafeOpsBinary(opStr) && flags.Size != SafeFloat {
		inv.MathNoTmp = true
		st := EInt
		if ty := flags.LHSType(); ty != nil && ty.IsSimple() {
			st = ty.Simple()
		}
		if blk := currentBlock(cg); blk != nil {
			var sym *GenSym
			if vs != nil {
				sym = &vs.Sym
			}
			inv.Tmp1 = blk.CreateNewTmpVar(sym, st)
			// shifts use op2 type for tmp2; else same as op1 (upstream)
			st2 := st
			if op == BinLShift || op == BinRShift {
				if ty := flags.RHSType(); ty != nil && ty.IsSimple() {
					st2 = ty.Simple()
				}
			}
			inv.Tmp2 = blk.CreateNewTmpVar(sym, st2)
		}
	}
	return inv
}

// currentBlock returns the top of Function::stack (current block for tmp vars).
func currentBlock(cg CGContext) *Block {
	if cg.CurrentFunc == nil || len(cg.CurrentFunc.Stack) == 0 {
		return nil
	}
	return cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
}

// MakeRandomBinaryPtrComparison mirrors make_random_binary_ptr_comparison.
// FunctionInvocation.cpp:294–360 — == or != on random pointer type operands;
// NO_DANGLING_PTR; ordered/unordered RHS effect contexts; bookkeeping.
func MakeRandomBinaryPtrComparison(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	env *TypeEnv,
) *Invocation {
	if r == nil || env == nil || !env.HasPointerType() {
		return nil
	}
	// FunctionInvocation.cpp:295–296 — eCmpEq or eCmpNe
	op := BinCmpEq
	if r.RndFlipcoin(50) {
		op = BinCmpNe
	}
	opStr := op.BinaryOpC()
	// FunctionInvocation.cpp:297–299 creates SafeOpFlags but Output for ptr_cmp
	// does not use safe wrappers (pointer ==/!= emit as infix). Leave Safe nil.
	_ = probs
	// Type::choose_random_pointer_type
	ptrTy := env.ChooseRandomPointerType(r)
	if ptrTy == nil {
		return nil
	}
	d := cg.ExprDepth + 1
	// FunctionInvocation.cpp:307–313 — LHS under ambient + NO_DANGLING_PTR + no_func
	lhsAccum := EmptyEffect()
	lhsCG := cg
	lhsCG.Flags |= FlagNoDanglingPtr
	lhsCG.EffectAccum = &lhsAccum
	left := MakeRandomExpression(r, opts, tables, vs, lhsCG, ptrTy, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, lhsCG, ptrTy, nil, true, false, TermVariable, d)
	}
	cg.MergeParamContext(lhsCG, true)

	// FunctionInvocation.cpp:317–320 — if LHS const, force RHS variable
	tt := MaxTermTypes
	if left != nil && left.Term == TermConstant {
		tt = TermVariable
	}
	var right *Expression
	// FunctionInvocation.cpp:326–345 — ordered short-circuit ops use original context;
	// ==/!= are unordered → RHS under original + LHS accum as effect_context
	if IsOrderedBinary(op) {
		oldFlags := cg.Flags
		cg.Flags |= FlagNoDanglingPtr
		right = MakeRandomExpression(r, opts, tables, vs, cg, ptrTy, nil, true, false, tt, d)
		if right == nil {
			right = MakeRandomExpression(r, opts, tables, vs, cg, ptrTy, nil, true, false, TermVariable, d)
		}
		cg.Flags = oldFlags
	} else {
		rhsAccum := EmptyEffect()
		rhsCG := cg
		rhsCG.effectContext = cg.EffectContext().AddEffect(lhsAccum)
		rhsCG.EffectAccum = &rhsAccum
		rhsCG.Flags |= FlagNoDanglingPtr
		right = MakeRandomExpression(r, opts, tables, vs, rhsCG, ptrTy, nil, true, false, tt, d)
		if right == nil {
			right = MakeRandomExpression(r, opts, tables, vs, rhsCG, ptrTy, nil, true, false, TermVariable, d)
		}
		cg.MergeParamContext(rhsCG, true)
	}
	if left == nil || right == nil {
		return nil
	}
	// FunctionInvocation.cpp:349 — typecast RHS to LHS type if needed (lang_cpp)
	right.CheckAndSetCastOpts(left.GetType(), opts)
	// FunctionInvocation.cpp:358 — bookkeeping
	RecordPointerComparisons(left, right)
	// FunctionInvocation.cpp:297–302 — flags always; Output uses standard ==/!= (not safe_ops)
	flags := MakeRandomBinaryKind(r, opts, probs, GetIntType(), GetIntType(), GetIntType(), SafeOpBinary, op)
	inv := &Invocation{
		IsStd:  true,
		Binary: opStr,
		Args:   []*Expression{left, right},
		Safe:   flags,
		PtrCmp: true,
	}
	inv.setOutOpts(opts)
	return inv
}

// MakeBinary mirrors FunctionInvocation::make_binary.
// FunctionInvocation.cpp:561–579 — SafeOpFlags from lhs/rhs types; attach operands.
// Used by StatementFor test expressions (and array mutate paths).
func MakeBinary(
	r *Rng,
	opts Options,
	probs *Probabilities,
	cg CGContext,
	op BinaryOp,
	lhs, rhs *Expression,
) *Invocation {
	if lhs == nil || rhs == nil {
		return nil
	}
	lt, rt := lhs.GetType(), rhs.GetType()
	// FunctionInvocation.cpp:566–568 — rv_type nullptr; op1/op2 from operands
	flags := MakeRandomBinaryKind(r, opts, probs, nil, lt, rt, SafeOpBinary, op)
	inv := &Invocation{
		IsStd:  true,
		Binary: op.BinaryOpC(),
		Args:   []*Expression{lhs, rhs},
		Safe:   flags,
	}
	inv.setOutOpts(opts)
	// CreateFunctionInvocationBinary tmps when math_notmp + safe_ops
	if flags != nil && opts.MathNoTmp && SafeOpsBinary(op.BinaryOpC()) && flags.Size != SafeFloat {
		inv.MathNoTmp = true
		st := EInt
		if ty := flags.LHSType(); ty != nil && ty.IsSimple() {
			st = ty.Simple()
		}
		if blk := currentBlock(cg); blk != nil {
			inv.Tmp1 = blk.CreateNewTmpVar(nil, st)
			st2 := st
			if op == BinLShift || op == BinRShift {
				if ty := flags.RHSType(); ty != nil && ty.IsSimple() {
					st2 = ty.Simple()
				}
			}
			inv.Tmp2 = blk.CreateNewTmpVar(nil, st2)
		}
	}
	return inv
}

// MakeRandomUnaryInvocation mirrors make_random_unary.
// FunctionInvocation.cpp:141–165 — eUnaryOps via UNARY_OPS_PROB_FILTER;
// always SafeOpFlags::make_random_unary; operand of get_lhs_type.
func MakeRandomUnaryInvocation(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	typ *Type,
) *Invocation {
	if typ == nil {
		typ = GetIntType()
	}
	// FunctionInvocation.cpp:146–149 — pick unary op (reject float-invalid when float type)
	var uop UnaryOp
	for tries := 0; tries < 16; tries++ {
		uop = PickUnaryOp(r, opts)
		if typ.IsFloat() && !UnaryOpWorksForFloat(uop) {
			continue
		}
		break
	}
	op := uop.UnaryOpC()
	// FunctionInvocation.cpp:151–155 — always make_random_unary then operand type from flags
	probs := NewProbabilities(opts)
	if vs != nil && vs.Probs != nil {
		probs = vs.Probs
	}
	flags := MakeRandomUnary(r, opts, probs, typ, nil, uop)
	argTy := typ
	if flags != nil {
		argTy = flags.LHSType()
	}
	// FunctionInvocation.cpp:157–159 — CreateFunctionInvocationUnary; operand under expr_depth
	d := cg.ExprDepth
	arg := MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, true, false, MaxTermTypes, d)
	if arg == nil {
		arg = MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, true, false, TermConstant, d)
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}, Safe: flags}
	inv.setOutOpts(opts)
	// FunctionInvocationUnary.cpp:51–60 — tmp when flags (we only emit when MathNoTmp + minus non-float)
	if flags != nil && opts.MathNoTmp && op == "-" && flags.Size != SafeFloat {
		inv.MathNoTmp = true
		st := EInt
		if ty := flags.LHSType(); ty != nil && ty.IsSimple() {
			st = ty.Simple()
		}
		if blk := currentBlock(cg); blk != nil {
			var sym *GenSym
			if vs != nil {
				sym = &vs.Sym
			}
			inv.Tmp1 = blk.CreateNewTmpVar(sym, st)
		}
	}
	return inv
}

// MakeRandomInvocation mirrors FunctionInvocation::make_random.
// FunctionInvocation.cpp:78–120.
// typ may be nil (StatementExpr) — choose_func ignores return type; new funcs use RandomReturnType.
func MakeRandomInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	list *FunctionList,
	typ *Type,
	qfer *CVQualifiers,
	stdFunc bool,
) *Invocation {
	_ = qfer
	// Match type for choose_func: nil means any return type (C++ type=0).
	matchType := typ
	// Concrete type for std ops / new signatures.
	workType := typ
	if workType == nil {
		workType = GetIntType()
	}
	// non-simple / void → force user path (std_func false)
	if workType.PtrType() != nil || (workType.IsSimple() && workType.Simple() == EVoid) {
		stdFunc = false
	}

	var fi *Invocation
	if !stdFunc {
		var callee *Function
		if r.RndFlipcoin(50) && list != nil {
			// Function.cpp:choose_func with in_conflict / strict_volatile
			cgp := &cg
			callee = ChooseFuncContext(r, list.Funcs, matchType, cg.CurrentFunc, cgp, opts)
		}
		if callee != nil {
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
		} else if list != nil && !ReachMaxFunctions(list, opts) {
			// build_invocation_and_function (FunctionInvocationUser.cpp:170+)
			sigType := workType
			if typ == nil {
				var env *TypeEnv
				if list != nil {
					env = list.Types
				}
				if env == nil {
					env = cg.Types
				}
				sigType = RandomReturnType(r, probs, env, opts)
			}
			fi = BuildInvocationAndFunction(r, opts, probs, vs, tables, NewStatementThresholdTable(opts), cg, list, sigType)
		} else {
			return &Invocation{Failed: true}
		}
	}
	if fi == nil {
		// std unary/binary
		if r.RndFlipcoin(uint32(probs.Single(PStdUnaryFuncProb))) {
			fi = MakeRandomUnaryInvocation(r, opts, vs, tables, cg, workType)
		} else {
			fi = MakeRandomBinaryInvocation(r, opts, probs, vs, tables, cg, workType)
		}
	}
	return fi
}
