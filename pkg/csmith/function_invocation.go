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
	// MathNoTmp only gates emit; temps are always allocated for safe_ops (C++ Create*)
	fi.MathNoTmp = opts.MathNoTmp
}

// wrapperOpts reconstructs Options for SafeMathWrapperAllowed checks.
func (fi *Invocation) wrapperOpts() Options {
	return Options{SafeMathWrappers: fi.OutSafeMathWrappers}
}

// Output C for the invocation.
// FunctionInvocationUnary::Output / FunctionInvocationBinary::Output —
// safe_* when avoid_signed_overflow + wrapper allowed; float unary uses standard op.
func (fi *Invocation) Output() string {
	// Failed invocations are not emitted (ExpressionFuncall replaces with var).
	// No soft invent /*bad_call*/ / /*invoke*/ comments.
	if fi == nil || fi.Failed {
		return ""
	}
	if fi.User != nil {
		// FunctionInvocationUser::Output — func name + param_value[i] always live
		// no invent "()" / empty slots "f(a, , c)" or soft "0" for nil/empty args
		if fi.User.Name == "" {
			return ""
		}
		var parts []string
		for _, a := range fi.Args {
			if a == nil {
				return ""
			}
			out := a.Output()
			if out == "" {
				// incomplete arg IR — fail closed whole call
				return ""
			}
			parts = append(parts, out)
		}
		var b strings.Builder
		b.WriteString(fi.User.Name)
		b.WriteString("(")
		for i, p := range parts {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p)
		}
		b.WriteString(")")
		return b.String()
	}
	if fi.IsStd {
		// FunctionInvocationUnary/Binary::Output — param_value[i]->Output; no soft invent "0"
		if fi.IsUnary {
			// assert known unary op + non-empty param
			switch fi.Unary {
			case "+", "-", "!", "~":
			default:
				// FunctionInvocationUnary.cpp:197 assert invalid operator
				return ""
			}
			if len(fi.Args) < 1 || fi.Args[0] == nil {
				return ""
			}
			a0 := fi.Args[0].Output()
			if a0 == "" {
				return ""
			}
			return fi.outputUnary(a0)
		}
		// binary: need two live args with non-empty Output
		if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
			return ""
		}
		if _, ok := BinaryOpFromString(fi.Binary); !ok && fi.Binary != "+" {
			// invalid op (except bare + for array mutate without flags)
			return ""
		}
		a0 := fi.Args[0].Output()
		a1 := fi.Args[1].Output()
		if a0 == "" || a1 == "" {
			return ""
		}
		return fi.outputBinary(a0, a1)
	}
	return ""
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
		// SafeOpFlags.cpp:325 assert / empty name → cast path (no invent wrapper name)
		if fname == "" {
			return unaryCastMinus(fi.Safe.SizeToken(), a0)
		}
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
		return unaryCastMinus(fi.Safe.SizeToken(), a0)
	}
	// FunctionInvocationUnary.cpp:229–240 — ePlus/eNot/eBitNot or non-safe minus
	if fi.Unary == "-" && fi.Safe != nil && !fi.OutSafeMath {
		// need_cast when Safe flags exist but avoid_signed_overflow off
		return unaryCastMinus(fi.Safe.SizeToken(), a0)
	}
	// FunctionInvocationUnary.cpp:226–239 — standard form (op)(arg)
	switch fi.Unary {
	case "+", "-", "!", "~":
		return fmt.Sprintf("(%s(%s))", fi.Unary, a0)
	default:
		// assert invalid operator — no invent emit
		return ""
	}
}

// unaryCastMinus is (-(size)arg); empty size token fail closed (no invent "(-()x)").
func unaryCastMinus(cast, a0 string) string {
	if cast == "" || a0 == "" {
		return ""
	}
	return fmt.Sprintf("(-(%s)%s)", cast, a0)
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
			return binaryCastOp(fi.Safe.SizeToken(), a0, fi.Binary, a1)
		}
	}
	// need_cast when Safe present but SafeMath off for arith/shift
	if fi.Safe != nil && SafeOpsBinary(fi.Binary) && !fi.OutSafeMath {
		return binaryCastOp(fi.Safe.SizeToken(), a0, fi.Binary, a1)
	}
	return fmt.Sprintf("(%s %s %s)", a0, fi.Binary, a1)
}

// binaryCastOp is ((cast)a0 op (cast)a1); empty cast fail closed (no invent "(()a + ()b)").
func binaryCastOp(cast, a0, op, a1 string) string {
	if cast == "" || a0 == "" || a1 == "" || op == "" {
		return ""
	}
	return fmt.Sprintf("((%s)%s %s (%s)%s)", cast, a0, op, cast, a1)
}

// ReachMaxFunctions mirrors Function::reach_max_functions_cnt.
// Function* always live on Funcs; nil hole fails closed as at-max
// (no invent undercount that allows more functions).
func ReachMaxFunctions(list *FunctionList, opts Options) bool {
	if list == nil {
		return false
	}
	n := 0
	for _, f := range list.Funcs {
		if f == nil {
			return true
		}
		if !f.IsBuiltin {
			n++
		}
	}
	return n >= opts.MaxFuncs
}

// ChooseFunc mirrors Function::choose_func — filter by return type convert, then choose_ok style.
// Function.cpp:279+ — skip unbuilt, builtins, effect conflict with context.
// cg may be nil (no conflict filter).
func ChooseFunc(r *Rng, funcs []*Function, ret *Type, exclude *Function) *Function {
	return ChooseFuncContext(r, funcs, ret, exclude, nil, Options{}, nil)
}

// ChooseFuncContext is ChooseFunc with CGContext for in_conflict / strict_volatile.
// Function.cpp:279–340 — separate user vs builtin pools; BuiltinFunctionProb pick.
// qfer when non-nil must match callee RV qualifiers (Function.cpp:294–295).
func ChooseFuncContext(r *Rng, funcs []*Function, ret *Type, exclude *Function, cg *CGContext, opts Options, qfer *CVQualifiers) *Function {
	var ok, okBuiltin []*Function
	for _, f := range funcs {
		// Function* always live on Funcs; nil hole fails closed (no invent skip)
		if f == nil {
			return nil
		}
		if f == exclude || !f.IsEffectKnown() {
			// is_effect_known() == false for Unbuilt/Building
			continue
		}
		// Function.cpp:288–289 — type->is_convertable(return_type)
		if ret != nil && f.ReturnType != nil && !ret.IsConvertableOpts(f.ReturnType, opts) {
			continue
		}
		if ret != nil && f.ReturnType == nil {
			continue
		}
		// Function.cpp:294–295 — qfer->match(rv->qfer)
		if qfer != nil && f.RV != nil && !qfer.Wildcard {
			if !qfer.Match(f.RV.Qfer, false) {
				continue
			}
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
		// Function.cpp:318–326 — has_race_with is commented out upstream; do not filter
		if f.IsBuiltin {
			if opts.Builtins {
				okBuiltin = append(okBuiltin, f)
			}
			continue
		}
		ok = append(ok, f)
	}
	// Function.cpp:330–337 — BuiltinFunctionProb → try builtin; else user only
	// (no soft fallback to builtins when user pool empty / not chosen)
	var f *Function
	if opts.Builtins && len(okBuiltin) > 0 && r != nil {
		p := opts.BuiltinFunctionProb
		if p <= 0 {
			p = 50
		}
		if r.RndFlipcoin(uint32(p)) {
			f = getOneFunction(r, okBuiltin)
		}
	}
	if f == nil {
		f = getOneFunction(r, ok)
	}
	return f
}

// getOneFunction mirrors Function::get_one_function — random pick.
// Function.cpp:262–276.
func getOneFunction(r *Rng, funcs []*Function) *Function {
	// Function.cpp:262–276 — rnd_upto(ok_size) when n>1; no soft invent funcs[0]
	n := len(funcs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return funcs[0]
	}
	if r == nil {
		return nil
	}
	return funcs[r.RndUpto(uint32(n))]
}

// ExpressionFunctionProbability mirrors ExpressionFuncall.cpp:57–62.
// true → prefer std (unary/binary); false → user function.
func ExpressionFunctionProbability(r *Rng, list *FunctionList, opts Options) bool {
	if ReachMaxFunctions(list, opts) && !opts.Builtins {
		return true
	}
	// ExpressionFuncall.cpp:57–62 — always rnd_flipcoin(80); no soft invent without RNG
	if r == nil {
		return false
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
// cg is *CGContext (C++ CGContext&) so merge_param_context / add_external_effect stick.
func BuildUserInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	list *FunctionList,
	callee *Function,
) *Invocation {
	// FunctionInvocationUser.cpp always has RNG + CGContext; no invent call shell without them
	if r == nil || callee == nil || cg == nil {
		return &Invocation{Failed: true}
	}
	fi := &Invocation{User: callee}
	// FunctionInvocationUser.cpp:249–270 — running effect context across params
	running := cg.EffectContext()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:256–258 — v->type / &v->qfer; no GetIntType invent
		if p == nil || p.Type == nil {
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		// FunctionInvocationUser.cpp:252–254 — param_cg(cg, running_eff_context, &param_eff_accum)
		paramAccum := EmptyEffect()
		paramCG := *cg
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		// Expression::make_random_param bumps paramCG.ExprDepth; merge copies it
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:259 — ERROR_GUARD(false); sticky error or null param → fail
		if arg == nil || HasError() {
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:261 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:264–267 — running first, then merge_param_context(default include_lhs=false)
		running = running.AddEffect(paramAccum)
		cg.MergeParamContext(paramCG, false)
	}

	// FunctionInvocationUser.cpp:272–301
	fi.Failed = false
	first := GetFirstFunction(list)
	// skip revisit for first function (func_1) — no params, single call, DFA hack
	if callee != first && callee.NeedsRevisit() {
		// FunctionInvocationUser.cpp:277–291 — revisit with accum_eff_context
		effectAccum := EmptyEffect()
		effectContext := cg.EffectContext().AddEffect(callee.AccumEffContext)
		newCG := *cg
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
// cg is *CGContext (C++ CGContext&).
func BuildInvocationAndFunction(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg *CGContext,
	list *FunctionList,
	retType *Type,
) *Invocation {
	// FunctionInvocationUser.cpp always has RNG + CGContext; no invent call+func without them
	if r == nil || cg == nil || list == nil || ReachMaxFunctions(list, opts) {
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:175 — assert(type); return type must be provided
	if retType == nil {
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:179 — make_random_signature
	callee := MakeRandomSignature(r, opts, probs, vs, &vs.Sym, *cg, retType, nil, list)
	if callee == nil {
		return &Invocation{Failed: true}
	}

	// FunctionInvocationUser.cpp:181–197 — build all parameters before body
	fi := &Invocation{User: callee}
	running := cg.EffectContext()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:185–187 — v->type; no GetIntType invent
		if p == nil || p.Type == nil {
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		paramAccum := EmptyEffect()
		paramCG := *cg
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:186–187 — make_random_param; C++ would ERROR_GUARD after sticky error
		// (build_invocation_and_function has no explicit ERROR_GUARD but uses param pointer)
		if arg == nil || HasError() {
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:190 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:193–196 — running.add_effect then merge_param_context(default false)
		running = running.AddEffect(paramAccum)
		cg.MergeParamContext(paramCG, false)
	}

	// FunctionInvocationUser.cpp:203–206 — hand-over from caller to callee with args
	// get_fact_mgr_for_func(func) after make_random_signature (FMList); no invent NewFactMgr
	var callerFM *FactMgr
	if cg.FM != nil {
		callerFM = cg.FM
	}
	calFM := callee.PairedFactMgr()
	if calFM == nil {
		fi.Failed = true
		return fi
	}
	facts := []*FactPointTo{}
	if callerFM != nil {
		facts = CloneFactSlice(callerFM.GlobalFacts)
	}
	calFM.CallerToCalleeHandover(fi.Args, &facts)
	calFM.GlobalFacts = facts

	// FunctionInvocationUser.cpp:208–210 — generate_body_with_known_params
	effectAccum := EmptyEffect()
	bodyCG := *cg
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
		// Variable* always live on NewGlobals; nil hole fails closed (mark failed)
		for _, v := range callee.NewGlobals {
			if v == nil {
				fi.Failed = true
				return fi
			}
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
// separate LHS CGContext + merge_param_context; ordered/unordered RHS.
// cg is *CGContext (C++ CGContext&) so merge_param_context expr_depth/effects stick.
func MakeRandomBinaryInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
) *Invocation {
	// FunctionInvocation.cpp always has RNG + CGContext; no invent binary shell without them
	if r == nil || cg == nil {
		return nil
	}
	// FunctionInvocation.cpp:173 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomBinary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationRandomBinary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:171+ — type may be nullptr (StatementExpr); no GetIntType invent
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
	// FunctionInvocation.cpp:179–183 — do { pick } while (type->is_float() && !works)
	// FunctionInvocation.cpp:185 — assert(type); nil type allowed only for non-float paths (library)
	op := PickBinaryOp(r, opts)
	if typ != nil && typ.IsFloat() {
		validB := false
		// C++ unbounded do-while; cap high (no soft invent invalid float op)
		for tries := 0; tries < 256; tries++ {
			if BinaryOpWorksForFloat(op) {
				validB = true
				break
			}
			op = PickBinaryOp(r, opts)
		}
		if !validB {
			return nil
		}
	}
	// PickBinaryOp MAX / empty token — no invent infix shell without live op
	opStr := op.BinaryOpC()
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		return nil
	}
	// FunctionInvocation.cpp:188–207 — always SafeOpFlags::make_random_binary; operands use get_lhs/rhs_type
	var flags *SafeOpFlags
	lhsTy, rhsTy := typ, typ
	// C++ always builds flags; CreateFunctionInvocationBinary only allocates tmps for safe_ops
	flags = MakeRandomBinaryKind(r, opts, probs, typ, typ, typ, SafeOpBinary, op)
	if flags == nil {
		// SafeOpFlags DEPTH_GUARD / ERROR_GUARD
		return nil
	}
	// FunctionInvocation.cpp:219–221 — assert(lhs_type && rhs_type)
	lhsTy = flags.LHSType()
	rhsTy = flags.RHSType()
	if lhsTy == nil || rhsTy == nil {
		return nil
	}
	// non-arith/shift: keep flags for typing but Output ignores safe path (SafeOpsBinary filter)
	if !SafeOpsBinary(opStr) {
		// still keep flags (C++ does); Output uses standard tokens
	}

	// FunctionInvocation.cpp:208–216 — LHS under dedicated accum + ambient effect_context
	lhsAccum := EmptyEffect()
	lhsCG := *cg
	lhsCG.effectContext = cg.EffectContext()
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = EmptyEffect()
	// FunctionInvocation.cpp:216 — Expression::make_random(lhs_cg, lhs_type) — no_func=false
	left := MakeRandomExpression(r, opts, tables, vs, &lhsCG, lhsTy, nil, false, false, MaxTermTypes, lhsCG.ExprDepth)
	// FunctionInvocation.cpp:217 — ERROR_GUARD_AND_DEL1(nullptr, fi)
	if left == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:221 — merge_param_context(lhs) (effects + expr_depth)
	cg.MergeParamContext(lhsCG, true)

	// FunctionInvocation.cpp:222 — snapshot facts before RHS (ordered merge)
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
	}

	var right *Expression
	if op == BinLShift || op == BinRShift {
		// FunctionInvocation.cpp:236–244 — ShiftByNonConstantProb (default 50)
		if !r.RndFlipcoin(50) {
			bits := uint32(32)
			if lhsTy != nil {
				if sb := lhsTy.SizeInBytes(); sb > 0 {
					bits = uint32(sb * 8)
				}
			}
			// Constant::make_random_upto; ERROR_GUARD — no invent shell with nil Con
			if c := MakeRandomUpto(bits, r); c != nil && !HasError() {
				right = &Expression{Term: TermConstant, Con: c}
				// Expression.cpp:213–218 — constant bumps expr_depth on caller context
				cg.ExprDepth++
			}
		}
	}
	if right == nil {
		if IsOrderedBinary(op) {
			// FunctionInvocation.cpp:224–226 — RHS under original cg_context
			right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, false, false, MaxTermTypes, cg.ExprDepth)
		} else {
			// FunctionInvocation.cpp:228–234 / 255 — combined effect_context + separate accum
			rhsAccum := EmptyEffect()
			rhsCG := *cg
			rhsCG.effectContext = cg.EffectContext().AddEffectOpts(lhsAccum, true)
			rhsCG.EffectAccum = &rhsAccum
			rhsCG.EffectStm = EmptyEffect()
			right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, rhsTy, nil, false, false, MaxTermTypes, rhsCG.ExprDepth)
			// FunctionInvocation.cpp:255 — merge_param_context(rhs)
			cg.MergeParamContext(rhsCG, true)
		}
	}
	// FunctionInvocation.cpp:257 — ERROR_GUARD_AND_DEL2
	if right == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:246–253 — avoid div/mod by 0 or 0/1 constant
	if (op == BinMod || op == BinDiv) && right.Term == TermConstant && right.Con != nil {
		if right.Con.Value == "0" || right.Con.Value == "1" {
			if lhsTy == nil || !lhsTy.IsFloat() {
				op = BinAdd
				opStr = op.BinaryOpC()
				if flags != nil && !SafeOpsBinary(opStr) {
					flags = nil
				}
			}
		}
	}
	// FunctionInvocation.cpp:266–273 — CompatibleChecker hard-fail (nullptr)
	if CompatibleCheckExprs(opts, left, right) {
		SetError(ErrCompatibleCheck)
		return nil
	}
	// FunctionInvocation.cpp:275–279 — ordered ops merge facts (short-circuit RHS may skip)
	if IsOrderedBinary(op) && cg.FM != nil && factsCopy != nil {
		MakeupNewVarFacts(&factsCopy, cg.FM.GlobalFacts)
		MergeFacts(&cg.FM.GlobalFacts, factsCopy)
	}
	inv := &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}, Safe: flags}
	inv.setOutOpts(opts)
	// FunctionInvocationBinary.cpp:59–75 — CreateFunctionInvocationBinary always creates
	// tmps for safe_ops when flags; math_notmp only affects Output.
	inv.Tmp1, inv.Tmp2 = createBinarySafeTmps(*cg, vs, flags, op)
	// assert(blk): safe_ops require live block temps — fail closed, no invent bare ops
	if flags != nil && SafeOpsBinary(opStr) && inv.Tmp1 == "" {
		return nil
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
// cg is *CGContext (C++ CGContext&).
func MakeRandomBinaryPtrComparison(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	env *TypeEnv,
) *Invocation {
	if r == nil || cg == nil || env == nil || !env.HasPointerType() {
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
	// FunctionInvocation.cpp:307–313 — LHS under ambient + NO_DANGLING_PTR + no_func=true
	lhsAccum := EmptyEffect()
	lhsCG := *cg
	lhsCG.effectContext = cg.EffectContext()
	lhsCG.Flags |= FlagNoDanglingPtr
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = EmptyEffect()
	// make_random(lhs_cg, type, 0, true) — no_func true
	left := MakeRandomExpression(r, opts, tables, vs, &lhsCG, ptrTy, nil, true, false, MaxTermTypes, lhsCG.ExprDepth)
	if left == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:313 — merge_param_context(lhs)
	cg.MergeParamContext(lhsCG, true)

	// FunctionInvocation.cpp:317–320 — if LHS const, force RHS variable
	tt := MaxTermTypes
	if left.Term == TermConstant {
		tt = TermVariable
	}
	var right *Expression
	// FunctionInvocation.cpp:326–345 — ordered short-circuit ops use original context;
	// ==/!= are unordered → RHS under original + LHS accum as effect_context
	if IsOrderedBinary(op) {
		oldFlags := cg.Flags
		cg.Flags |= FlagNoDanglingPtr
		// make_random(..., true, false, tt) — no_func true
		right = MakeRandomExpression(r, opts, tables, vs, cg, ptrTy, nil, true, false, tt, cg.ExprDepth)
		cg.Flags = oldFlags
	} else {
		rhsAccum := EmptyEffect()
		rhsCG := *cg
		// FunctionInvocation.cpp:338–342 — effect_context + lhs_eff_accum
		rhsCG.effectContext = cg.EffectContext().AddEffect(lhsAccum)
		rhsCG.EffectAccum = &rhsAccum
		rhsCG.EffectStm = EmptyEffect()
		rhsCG.Flags |= FlagNoDanglingPtr
		right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, ptrTy, nil, true, false, tt, rhsCG.ExprDepth)
		// FunctionInvocation.cpp:345
		cg.MergeParamContext(rhsCG, true)
	}
	if right == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:349 — typecast RHS to LHS type if needed (lang_cpp)
	right.CheckAndSetCastOpts(left.GetType(), opts)
	// FunctionInvocation.cpp:358 — bookkeeping
	RecordPointerComparisons(left, right)
	// FunctionInvocation.cpp:297–302 — flags always; Output uses standard ==/!= (not safe_ops)
	flags := MakeRandomBinaryKind(r, opts, probs, GetIntType(), GetIntType(), GetIntType(), SafeOpBinary, op)
	// ERROR_GUARD after make_random_binary; no soft invent nil-flags ptr comparison
	if flags == nil {
		return nil
	}
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
	// FunctionInvocation.cpp:565+ — always has RNG; no invent binary shell without it
	if r == nil || lhs == nil || rhs == nil {
		return nil
	}
	// FunctionInvocation.cpp:565 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationBinary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationBinary) == BadDepth {
		return nil
	}
	// invalid / MAX op — no invent empty Binary token shell
	opStr := op.BinaryOpC()
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		return nil
	}
	lt, rt := lhs.GetType(), rhs.GetType()
	// FunctionInvocation.cpp:566–568 — rv_type nullptr; op1/op2 from operands
	flags := MakeRandomBinaryKind(r, opts, probs, nil, lt, rt, SafeOpBinary, op)
	// FunctionInvocation.cpp:568 — ERROR_GUARD; no soft invent binary without flags
	if flags == nil {
		return nil
	}
	inv := &Invocation{
		IsStd:  true,
		Binary: opStr,
		Args:   []*Expression{lhs, rhs},
		Safe:   flags,
	}
	inv.setOutOpts(opts)
	// FunctionInvocationBinary.cpp:59–75 — always create tmps for safe_ops
	inv.Tmp1, inv.Tmp2 = createBinarySafeTmps(cg, nil, flags, op)
	if flags != nil && SafeOpsBinary(opStr) && inv.Tmp1 == "" {
		return nil
	}
	return inv
}

// MakeRandomUnaryInvocation mirrors make_random_unary.
// FunctionInvocation.cpp:141–165 — eUnaryOps via UNARY_OPS_PROB_FILTER;
// always SafeOpFlags::make_random_unary; operand of get_lhs_type.
// cg is *CGContext (C++ CGContext&) so operand visit_facts / expr_depth++ stick.
func MakeRandomUnaryInvocation(
	r *Rng,
	opts Options,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	typ *Type,
) *Invocation {
	// FunctionInvocation.cpp always has RNG + CGContext; no invent unary shell without them
	if r == nil || cg == nil {
		return nil
	}
	// FunctionInvocation.cpp:143 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomUnary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationRandomUnary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:144 — assert(type)
	if typ == nil {
		return nil
	}
	// FunctionInvocation.cpp:146–149 — do { pick } while (float && !works); no soft invent invalid
	var uop UnaryOp
	validU := false
	// C++ unbounded do-while; cap high (no soft invent invalid float op)
	for tries := 0; tries < 256; tries++ {
		uop = PickUnaryOp(r, opts)
		if !typ.IsFloat() || UnaryOpWorksForFloat(uop) {
			validU = true
			break
		}
	}
	if !validU {
		return nil
	}
	// PickUnaryOp MAX / empty token — no invent unary shell without live op
	op := uop.UnaryOpC()
	if int(uop) < 0 || int(uop) >= MaxUnaryOp || op == "" {
		return nil
	}
	// FunctionInvocation.cpp:151–155 — always make_random_unary then operand type from flags
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	var probs *Probabilities
	if vs != nil {
		probs = vs.Probs
	}
	flags := MakeRandomUnary(r, opts, probs, typ, nil, uop)
	// FunctionInvocation.cpp:152–154 — ERROR_GUARD; type = flags->get_lhs_type(); assert(type)
	if flags == nil {
		return nil
	}
	argTy := flags.LHSType()
	if argTy == nil {
		return nil
	}
	// FunctionInvocation.cpp:157–159 — Expression::make_random(cg, type) — no_func=false
	arg := MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, false, false, MaxTermTypes, cg.ExprDepth)
	// FunctionInvocation.cpp:158 — ERROR_GUARD_AND_DEL1
	if arg == nil || HasError() {
		return nil
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}, Safe: flags}
	inv.setOutOpts(opts)
	// FunctionInvocationUnary.cpp:51–60 — CreateFunctionInvocationUnary always creates
	// tmp when flags; math_notmp only affects Output (eMinus path).
	inv.Tmp1 = createUnarySafeTmp(*cg, vs, flags)
	// FunctionInvocationUnary.cpp:57 assert(blk) — fail closed when flags but no tmp
	if flags != nil && inv.Tmp1 == "" {
		return nil
	}
	return inv
}

// createBinarySafeTmps mirrors FunctionInvocationBinary::CreateFunctionInvocationBinary
// temp allocation. FunctionInvocationBinary.cpp:59–75 — always when flags && safe_ops;
// no soft invent skip on !MathNoTmp or float size.
func createBinarySafeTmps(cg CGContext, vs *VariableSelector, flags *SafeOpFlags, op BinaryOp) (tmp1, tmp2 string) {
	if flags == nil || !SafeOpsBinary(op.BinaryOpC()) {
		return "", ""
	}
	blk := currentBlock(cg)
	// FunctionInvocationBinary.cpp:68 — assert(blk); no soft invent safe_ops without temps
	if blk == nil {
		return "", ""
	}
	// FunctionInvocationBinary.cpp:64–66 — flags_to_type must yield a simple type
	ty1 := flags.LHSType()
	if ty1 == nil || !ty1.IsSimple() {
		return "", ""
	}
	st := ty1.Simple()
	var sym *GenSym
	if vs != nil {
		sym = &vs.Sym
	}
	tmp1 = blk.CreateNewTmpVar(sym, st)
	st2 := st
	if op == BinLShift || op == BinRShift {
		if ty := flags.RHSType(); ty != nil && ty.IsSimple() {
			st2 = ty.Simple()
		}
	}
	tmp2 = blk.CreateNewTmpVar(sym, st2)
	return tmp1, tmp2
}

// createUnarySafeTmp mirrors FunctionInvocationUnary::CreateFunctionInvocationUnary
// temp allocation. FunctionInvocationUnary.cpp:51–60 — always when flags non-nil.
func createUnarySafeTmp(cg CGContext, vs *VariableSelector, flags *SafeOpFlags) string {
	if flags == nil {
		return ""
	}
	blk := currentBlock(cg)
	// FunctionInvocationUnary.cpp:57 — assert(blk); no soft invent unary safe without tmp
	if blk == nil {
		return ""
	}
	ty := flags.LHSType()
	if ty == nil || !ty.IsSimple() {
		return ""
	}
	var sym *GenSym
	if vs != nil {
		sym = &vs.Sym
	}
	return blk.CreateNewTmpVar(sym, ty.Simple())
}

// MakeRandomInvocation mirrors FunctionInvocation::make_random.
// FunctionInvocation.cpp:78–120.
// typ may be nil (StatementExpr) — choose_func ignores return type; new funcs use RandomReturnType.
// cg is *CGContext (C++ CGContext&).
func MakeRandomInvocation(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg *CGContext,
	list *FunctionList,
	typ *Type,
	qfer *CVQualifiers,
	stdFunc bool,
) *Invocation {
	// FunctionInvocation.cpp always has RNG + CGContext; no invent invoke shell without them
	if r == nil || cg == nil {
		return &Invocation{Failed: true}
	}
	// Match type for choose_func: nil means any return type (C++ type=0).
	matchType := typ
	// FunctionInvocation.cpp:71–73 path — non-simple/void force user path (type known)
	if typ != nil && (typ.PtrType() != nil || (typ.IsSimple() && typ.Simple() == EVoid)) {
		stdFunc = false
	}

	var fi *Invocation
	if !stdFunc {
		var callee *Function
		// FunctionInvocation.cpp:87 — pure_rnd_flipcoin(50) (random mode == rnd)
		if r.RndFlipcoin(50) && list != nil {
			// Function.cpp:choose_func with in_conflict / strict_volatile / qfer
			callee = ChooseFuncContext(r, list.Funcs, matchType, cg.CurrentFunc, cg, opts, qfer)
		}
		if callee != nil {
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
			// FunctionInvocation.cpp:95–97 — caller.fact_changed |= callee.fact_changed
			if fi != nil && !fi.Failed && cg.CurrentFunc != nil && fi.User != nil {
				cg.CurrentFunc.FactChanged = cg.CurrentFunc.FactChanged || fi.User.FactChanged
			}
		} else if list != nil && !ReachMaxFunctions(list, opts) {
			// FunctionInvocationUser.cpp:175 — assert(type); RandomReturnType when type nil
			sigType := typ
			if sigType == nil {
				var env *TypeEnv
				if list != nil {
					env = list.Types
				}
				if env == nil {
					env = cg.Types
				}
				sigType = RandomReturnType(r, probs, env, opts)
				// ERROR_GUARD when choose_random fails; no soft invent GetIntType return
				if sigType == nil {
					return &Invocation{Failed: true}
				}
			}
			// Statement probability table is process/session singleton (no invent second table)
			stmtTab := ProcessStmtTab()
			if stmtTab == nil {
				return &Invocation{Failed: true}
			}
			fi = BuildInvocationAndFunction(r, opts, probs, vs, tables, stmtTab, cg, list, sigType)
			if fi != nil && !fi.Failed && cg.CurrentFunc != nil && fi.User != nil {
				cg.CurrentFunc.FactChanged = cg.CurrentFunc.FactChanged || fi.User.FactChanged
			}
		} else {
			// FunctionInvocation.cpp:102–106 — failed when at max funcs
			return &Invocation{Failed: true}
		}
	}
	if fi == nil {
		// FunctionInvocation.cpp:111–118 — StdUnaryFuncProb → unary else binary
		// unary asserts(type); when type nil prefer binary (no GetIntType invent)
		if typ != nil && r.RndFlipcoin(uint32(probs.Single(PStdUnaryFuncProb))) {
			fi = MakeRandomUnaryInvocation(r, opts, vs, tables, cg, typ)
		} else {
			fi = MakeRandomBinaryInvocation(r, opts, probs, vs, tables, cg, typ)
		}
	}
	// FunctionInvocation.cpp:119 — assert(fi != 0); std factories must yield live invoke
	// incomplete null without sticky error → failed shell (ExpressionFuncall var fallback)
	// no invent success IR
	if fi == nil {
		if HasError() {
			return nil
		}
		return &Invocation{Failed: true}
	}
	return fi
}
