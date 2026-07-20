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
// Invocation always live; sticky (no invent soft-skip out-opts past hole).
func (fi *Invocation) setOutOpts(opts Options) {
	if fi == nil {
		SetError(ErrGeneric)
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
	// FunctionInvocation always live at Output; sticky no invent empty call without it
	if fi == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Failed invocations are not emitted (ExpressionFuncall replaces with var).
	// No soft invent /*bad_call*/ / /*invoke*/ comments. Failed stays non-sticky soft re-pick.
	if fi.Failed {
		return ""
	}
	if fi.User != nil {
		// FunctionInvocationUser::Output — func name + param_value[i] always live
		// sticky no invent "()" / empty slots "f(a, , c)" or soft "0" for nil/empty args
		if fi.User.Name == "" {
			SetError(ErrGeneric)
			return ""
		}
		var parts []string
		for _, a := range fi.Args {
			if a == nil {
				SetError(ErrGeneric)
				return ""
			}
			out := a.Output()
			// residual ERROR sticky — no invent soft-continue later args past Output residual
			if HasError() {
				return ""
			}
			if out == "" {
				// incomplete arg IR — sticky fail closed whole call
				SetError(ErrGeneric)
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
		// FunctionInvocationUnary/Binary::Output — param_value[i]->Output; sticky no soft invent "0"
		if fi.IsUnary {
			// assert known unary op + non-empty param
			switch fi.Unary {
			case "+", "-", "!", "~":
			default:
				// FunctionInvocationUnary.cpp:197 assert invalid operator sticky
				SetError(ErrGeneric)
				return ""
			}
			if len(fi.Args) < 1 || fi.Args[0] == nil {
				SetError(ErrGeneric)
				return ""
			}
			a0 := fi.Args[0].Output()
			// residual ERROR sticky — no invent soft-empty unary past Output residual
			if HasError() {
				return ""
			}
			if a0 == "" {
				SetError(ErrGeneric)
				return ""
			}
			out := fi.outputUnary(a0)
			// residual ERROR sticky — no invent soft-empty unary past outputUnary residual
			if HasError() {
				return ""
			}
			return out
		}
		// binary: need two live args with non-empty Output
		if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
			SetError(ErrGeneric)
			return ""
		}
		if _, ok := BinaryOpFromString(fi.Binary); !ok && fi.Binary != "+" {
			// invalid op sticky (except bare + for array mutate without flags)
			SetError(ErrGeneric)
			return ""
		}
		a0 := fi.Args[0].Output()
		// residual ERROR sticky — no invent soft-continue a1 past a0 Output residual
		if HasError() {
			return ""
		}
		a1 := fi.Args[1].Output()
		// residual ERROR sticky — no invent soft-empty binary past a1 Output residual
		if HasError() {
			return ""
		}
		if a0 == "" || a1 == "" {
			SetError(ErrGeneric)
			return ""
		}
		out := fi.outputBinary(a0, a1)
		// residual ERROR sticky — no invent soft-empty binary past outputBinary residual
		if HasError() {
			return ""
		}
		return out
	}
	// incomplete non-user non-std shell sticky
	SetError(ErrGeneric)
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
		// assert invalid operator — sticky no invent emit
		SetError(ErrGeneric)
		return ""
	}
}

// unaryCastMinus is (-(size)arg); empty size token sticky fail closed (no invent "(-()x)").
func unaryCastMinus(cast, a0 string) string {
	if cast == "" || a0 == "" {
		SetError(ErrGeneric)
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

// binaryCastOp is ((cast)a0 op (cast)a1); empty cast sticky fail closed (no invent "(()a + ()b)").
func binaryCastOp(cast, a0, op, a1 string) string {
	if cast == "" || a0 == "" || a1 == "" || op == "" {
		SetError(ErrGeneric)
		return ""
	}
	return fmt.Sprintf("((%s)%s %s (%s)%s)", cast, a0, op, cast, a1)
}

// ReachMaxFunctions mirrors Function::reach_max_functions_cnt.
// Function* always live on Funcs; nil hole fails closed as at-max
// (no invent undercount that allows more functions). Non-sticky: at-max is a
// soft re-pick gate, not ERROR_GUARD sticky (would leave unbuilt shells).
func ReachMaxFunctions(list *FunctionList, opts Options) bool {
	if list == nil {
		return false
	}
	n := 0
	for _, f := range list.Funcs {
		if f == nil {
			// incomplete Funcs fail closed as max-reached (restrictive filter — no invent
			// room for more). Non-sticky: soft re-pick factories must not get ambient ERROR
			// poison from list holes (see TestReachMaxFunctions / GenerateFunctions).
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
// Incomplete Funcs / ambient / callee IR fails closed sticky (no invent soft re-pick
// past holes as absent / conflict-free while a later complete entry would succeed).
func ChooseFuncContext(r *Rng, funcs []*Function, ret *Type, exclude *Function, cg *CGContext, opts Options, qfer *CVQualifiers) *Function {
	// incomplete Funcs list fails closed sticky (no invent soft-skip nil hole as absent)
	if !FunctionsComplete(funcs) {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent conflict-filter under hole shells)
	if cg != nil {
		if !EffectComplete(cg.EffectContext()) ||
			(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
			!EffectComplete(cg.EffectStm) {
			SetError(ErrGeneric)
			return nil
		}
		if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
	}
	var ok, okBuiltin []*Function
	for _, f := range funcs {
		// pre-validated FunctionsComplete
		if f == exclude || !f.IsEffectKnown() {
			// residual ERROR sticky — no invent soft-continue then pick later past IsEffectKnown hole
			if HasError() {
				return nil
			}
			// is_effect_known() == false for Unbuilt/Building
			continue
		}
		// residual ERROR sticky — no invent soft-continue known-true past IsEffectKnown residual
		if HasError() {
			return nil
		}
		// Function.cpp:288–289 — type->is_convertable(return_type)
		// C++ always has live return_type*; nil is incomplete IR
		// fail closed sticky whole choose (no invent soft-skip broken func as absent)
		if ret != nil && f.ReturnType == nil {
			SetError(ErrGeneric)
			return nil
		}
		if ret != nil && f.ReturnType != nil {
			ok := ret.IsConvertableOpts(f.ReturnType, opts)
			// residual ERROR sticky — no invent soft-continue then pick later past IsConvertable residual
			if HasError() {
				return nil
			}
			if !ok {
				continue
			}
		}
		// Function.cpp:294–295 — qfer->match(rv->qfer); RV always live after create
		// incomplete RV fails closed sticky (no invent soft-skip as match / re-pick)
		if qfer != nil && !qfer.Wildcard {
			if f.RV == nil {
				SetError(ErrGeneric)
				return nil
			}
			if !qfer.Match(f.RV.Qfer, false) {
				// residual ERROR sticky — no invent soft-continue then pick later past Match hole
				if HasError() {
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-keep match past Match residual true path
			if HasError() {
				return nil
			}
		}
		// incomplete callee FEffect fails closed sticky (no invent skip as conflict past hole)
		if cg != nil && !EffectComplete(f.FEffect) {
			SetError(ErrGeneric)
			return nil
		}
		// Function.cpp:303–306 — in_conflict with callee feffect
		if cg != nil && cg.InConflict(f.FEffect) {
			// residual ERROR sticky — no invent soft-continue conflict-skip past hard IR
			if HasError() {
				return nil
			}
			continue
		}
		// residual ERROR sticky — no invent soft-keep after InConflict residual false
		if HasError() {
			return nil
		}
		// Function.cpp:307–313 — strict_volatile_rule
		if opts.StrictVolatileRule && cg != nil {
			if !f.FEffect.IsSideEffectFree() && !cg.EffectContext().IsSideEffectFree() {
				// residual ERROR sticky — no invent soft-continue then pick later past SE residual
				if HasError() {
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-continue SE-free path past residual hole
			if HasError() {
				return nil
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
	// CGOptions::BuiltinFunctionProb() as-is; 0 means never (no invent default 50)
	var f *Function
	if opts.Builtins && len(okBuiltin) > 0 && r != nil {
		p := opts.BuiltinFunctionProb
		if p > 0 && r.RndFlipcoin(uint32(p)) {
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
	// Function.cpp:262–276 — rnd_upto(ok_size) when n>1; sticky no invent funcs[0]
	n := len(funcs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return funcs[0]
	}
	if r == nil {
		SetError(ErrGeneric)
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
	// non-sticky false (prefer user) would invent; sticky poisons ExpressionFuncall soft paths —
	// keep fail closed false without sticky (callers always pass process RNG in Generate)
	if r == nil {
		return false
	}
	return r.RndFlipcoin(80)
}

// GetFirstFunction mirrors GetFirstFunction — first entry in FuncList / func_1.
// Function.cpp / FunctionInvocationUser.cpp:274.
// Nil list / empty Funcs is complete miss (no invent first of empty; isolated
// BuildUserInvocation may pass nil list as "no first-function identity").
// Function* always live at [0]; nil hole sticky (no invent scan later).
func GetFirstFunction(list *FunctionList) *Function {
	if list == nil || len(list.Funcs) == 0 {
		return nil
	}
	// C++ first_function is funcs[0]; incomplete IR at front sticky fail closed
	if list.Funcs[0] == nil {
		SetError(ErrGeneric)
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
	// FunctionInvocationUser.cpp always has RNG + CGContext; sticky Failed
	// (no invent call shell / soft re-pick past hole)
	if r == nil || callee == nil || cg == nil {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent param gen / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return &Invocation{User: callee, Failed: true}
	}
	fi := &Invocation{User: callee}
	// FunctionInvocationUser.cpp:249–270 — running effect context across params
	running := cg.EffectContext()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:256–258 — v->type / &v->qfer; sticky no invent soft-skip param hole
		if p == nil || p.Type == nil {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		// FunctionInvocationUser.cpp:252–254 — param_cg(cg, running_eff_context, &param_eff_accum)
		paramAccum := EmptyEffect()
		paramCG := cg.CloneSubcontext()
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		// Expression::make_random_param bumps paramCG.ExprDepth; merge copies it
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:259 — ERROR_GUARD(false); sticky error or null param → fail
		// null param without sticky invents soft re-pick past ERROR_GUARD miss
		if arg == nil || HasError() {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:261 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		// residual ERROR sticky — no invent param past CheckAndSetCast residual hole
		if HasError() {
			fi.Failed = true
			return fi
		}
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:264–267 — running first, then merge_param_context(default include_lhs=false)
		// Incomplete param accum fails closed sticky (no invent more params / soft re-pick past holes)
		running = running.AddEffect(paramAccum)
		// residual ERROR sticky — no invent soft-continue later params past AddEffect residual
		if HasError() {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(running) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		cg.MergeParamContext(paramCG, false)
		// residual ERROR sticky — no invent soft-continue later params past MergeParam residual
		if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
	}

	// FunctionInvocationUser.cpp:272–301
	fi.Failed = false
	first := GetFirstFunction(list)
	// skip revisit for first function (func_1) — no params, single call, DFA hack
	needRev := callee != first && callee.NeedsRevisit()
	// residual ERROR sticky — no invent soft-skip revisit past NeedsRevisit residual
	if HasError() {
		fi.Failed = true
		return fi
	}
	if needRev {
		// FunctionInvocationUser.cpp:277–291 — revisit with accum_eff_context
		// Incomplete AccumEffContext fails closed sticky (no invent revisit under incomplete ambient)
		effectAccum := EmptyEffect()
		effectContext := cg.EffectContext().AddEffect(callee.AccumEffContext)
		// residual ERROR sticky — no invent soft-continue revisit past AddEffect residual
		if HasError() {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(effectContext) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		newCG := cg.CloneSubcontext()
		newCG.effectContext = effectContext
		newCG.EffectAccum = &effectAccum
		// FunctionInvocationUser.cpp:284 — revisit(fm->global_facts, new_context)
		// where fm = get_fact_mgr(&cg_context) is the CALLER FactMgr. C++ mutates
		// caller global_facts in place (handover drops frame locals, then renew
		// restores from inputs_copy). Do not deep-clone then reinstall: that path
		// can drop mid-gen may-null lattice (seed-2 first_div 10107 l_233).
		if cg.FM == nil {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		ok := RevisitUserInvocation(fi, &cg.FM.GlobalFacts, &newCG, opts)
		fi.Failed = !ok
		if ok {
			// FunctionInvocationUser.cpp:284–290
			// Incomplete effect hand-over fails closed sticky (no invent silent Incomplete FEffect)
			if cg.CurrentBlock() != nil {
				cg.AddVisibleEffectAt(effectAccum, cg.CurrentBlock())
				if HasError() {
					fi.Failed = true
					return fi
				}
			}
			if !EffectComplete(effectAccum) || !EffectComplete(callee.FEffect) {
				SetError(ErrGeneric)
				fi.Failed = true
				return fi
			}
			callee.FEffect = callee.FEffect.AddExternalEffectWithCallers(effectAccum, cg.CallChain)
			if !EffectComplete(callee.FEffect) {
				SetError(ErrGeneric)
				fi.Failed = true
				return fi
			}
		}
	} else {
		// FunctionInvocationUser.cpp:293–297 — static effect, no re-analyze
		// add_external_effect(func->get_feffect())
		if callee.IsEffectKnown() {
			// residual ERROR sticky — no invent static-effect path past IsEffectKnown hole
			if HasError() {
				fi.Failed = true
				return fi
			}
			cg.AddExternalEffect(callee.FEffect)
			if HasError() {
				fi.Failed = true
				return fi
			}
		} else if HasError() {
			// residual ERROR sticky — no invent soft-skip effect fold past IsEffectKnown residual false
			fi.Failed = true
			return fi
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
// qfer is the Expression/FunctionInvocation qfer passed to make_random_signature
// (FunctionInvocationUser.cpp:179 — make_random_signature(cg, type, qfer)).
// nil → full random_qualifiers(type,…); non-nil → qfer->random_qualifiers relative.
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
	qfer *CVQualifiers,
) *Invocation {
	// FunctionInvocationUser.cpp always has RNG + CGContext + FuncList; sticky Failed
	// (no invent call+func shell past hole). At-max is complete soft Failed (no sticky).
	if r == nil || cg == nil || list == nil {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	if ReachMaxFunctions(list, opts) {
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:175 — assert(type); sticky Failed (no invent return type)
	if retType == nil {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent signature/params past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:179 — make_random_signature(cg, type, qfer)
	// Do not invent nil qfer when caller passed one (seed-2 e447: RV qfer draws
	// differed vs always RandomQualifiersDefaultProbs).
	callee := MakeRandomSignature(r, opts, probs, vs, &vs.Sym, *cg, retType, qfer, list)
	if callee == nil {
		// signature ERROR_GUARD sticky when not already set
		if !HasError() {
			SetError(ErrGeneric)
		}
		return &Invocation{Failed: true}
	}

	// FunctionInvocationUser.cpp:181–197 — build all parameters before body
	fi := &Invocation{User: callee}
	running := cg.EffectContext()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:185–187 — v->type; sticky no invent soft-skip param hole
		if p == nil || p.Type == nil {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		paramAccum := EmptyEffect()
		// CGContext.cpp:74–82 — param context deep-copies iv_bounds
		paramCG := cg.CloneSubcontext()
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:186–187 — make_random_param; ERROR_GUARD after sticky error
		// null param without sticky invents soft re-pick past ERROR_GUARD miss
		if arg == nil || HasError() {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:190 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		// residual ERROR sticky — no invent param past CheckAndSetCast residual hole
		if HasError() {
			fi.Failed = true
			return fi
		}
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:193–196 — running.add_effect then merge_param_context(default false)
		// Incomplete param accum fails closed sticky (no invent more params / soft re-pick past holes)
		running = running.AddEffect(paramAccum)
		// residual ERROR sticky — no invent soft-continue later params past AddEffect residual
		if HasError() {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(running) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		cg.MergeParamContext(paramCG, false)
		if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
	}

	// FunctionInvocationUser.cpp:203–206 — hand-over from caller to callee with args
	// get_fact_mgr_for_func(func) after make_random_signature (FMList); sticky no invent without FM
	var callerFM *FactMgr
	if cg.FM != nil {
		callerFM = cg.FM
	}
	calFM := callee.PairedFactMgr()
	if calFM == nil {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	facts := []*FactPointTo{}
	if callerFM != nil {
		// incomplete caller GlobalFacts fail closed sticky (no invent cleaned handover)
		if !FactsComplete(callerFM.GlobalFacts) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		facts = CloneFactSlice(callerFM.GlobalFacts)
		// residual ERROR sticky — no invent soft-handover past CloneFactSlice residual
		if HasError() {
			fi.Failed = true
			return fi
		}
	}
	calFM.CallerToCalleeHandover(fi.Args, &facts)
	// residual ERROR sticky — no invent soft-handover past CallerToCallee residual
	if HasError() {
		fi.Failed = true
		return fi
	}
	calFM.GlobalFacts = facts

	// FunctionInvocationUser.cpp:208–210 — generate_body_with_known_params
	effectAccum := EmptyEffect()
	bodyCG := cg.CloneSubcontext()
	bodyCG.CurrentFunc = callee
	bodyCG.FM = calFM
	bodyCG.Flags = 0
	bodyCG.EffectAccum = &effectAccum
	// RWDirective + call chain set inside GenerateBodyWithKnownParams
	callee.GenerateBodyWithKnownParams(r, opts, probs, vs, tables, stmtTab, bodyCG)

	// FunctionInvocationUser.cpp:212–215 — ret_facts = map_facts_out[body]
	// then add_back_return_facts. GetMapFactsOut: StmID 0 Incomplete; missing → empty.
	// Incomplete out / add_back / missing body fail closed sticky — no invent soft-merge returns
	var retFacts []*FactPointTo
	if callee.Body == nil {
		// GenerateBody must leave live body; sticky Failed (no invent soft-skip without body)
		if !HasError() {
			SetError(ErrGeneric)
		}
		fi.Failed = true
		return fi
	}
	out := calFM.GetMapFactsOut(callee.Body.StmID)
	// residual ERROR sticky — no invent soft-ret facts past GetMapFactsOut residual
	if HasError() {
		fi.Failed = true
		return fi
	}
	if !FactsComplete(out) {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	retFacts = CloneFactSlice(out)
	// residual ERROR sticky — no invent soft-ret facts past CloneFactSlice residual
	if HasError() {
		fi.Failed = true
		return fi
	}
	if !AddBackReturnFacts(callee.Body, calFM, &retFacts) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		fi.Failed = true
		return fi
	}
	fi.SaveReturnFacts(retFacts)

	// FunctionInvocationUser.cpp:219 — setup_in_out_maps(true)
	calFM.SetupInOutMaps(true)

	// FunctionInvocationUser.cpp:221 — renew_facts(caller, ret_facts)
	if callerFM != nil {
		// complete retFacts (may be empty nil) required; incomplete caller fails closed sticky
		// (no invent RenewFacts no-op success past incomplete then keep prior)
		if !FactsComplete(callerFM.GlobalFacts) || !FactsComplete(retFacts) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		_ = RenewFacts(&callerFM.GlobalFacts, retFacts)
		if !FactsComplete(callerFM.GlobalFacts) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:234–238 — new globals facts
		// Incomplete NewGlobals fails closed sticky (no invent soft-skip hole / partial push)
		if !VariablesComplete(callee.NewGlobals) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
		for _, v := range callee.NewGlobals {
			callerFM.AddNewVarFactAndUpdate(nil, v)
			if !FactsComplete(callerFM.GlobalFacts) {
				SetError(ErrGeneric)
				fi.Failed = true
				return fi
			}
		}
	}

	// FunctionInvocationUser.cpp:223–228 — effect hand-over
	// Incomplete external merge fails closed sticky (no invent silent Incomplete shells)
	if !EffectComplete(cg.EffectContext()) || !EffectComplete(callee.AccumEffContext) {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	callee.AccumEffContext = callee.AccumEffContext.AddExternalEffect(cg.EffectContext())
	if !EffectComplete(callee.AccumEffContext) {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	// feffect.add_external_effect(effect_accum, call_chain)
	if !EffectComplete(effectAccum) || !EffectComplete(callee.FEffect) {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	callee.FEffect = callee.FEffect.AddExternalEffectWithCallers(effectAccum, cg.CallChain)
	if !EffectComplete(callee.FEffect) {
		SetError(ErrGeneric)
		fi.Failed = true
		return fi
	}
	// also keep ComputeSummary body effect already applied in GenerateBody
	cg.AddVisibleEffectAt(effectAccum, cg.CurrentBlock())
	if HasError() {
		fi.Failed = true
		return fi
	}

	// FunctionInvocationUser.cpp:230–233 — new_globals hand-over
	if cg.CurrentFunc != nil && len(callee.NewGlobals) > 0 {
		if !VariablesComplete(callee.NewGlobals) {
			SetError(ErrGeneric)
			fi.Failed = true
			return fi
		}
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
	// FunctionInvocation.cpp always has RNG + CGContext sticky; no invent binary shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent binary / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:173 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomBinary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationRandomBinary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:171+ — type may be nullptr (StatementExpr); no GetIntType invent
	// FunctionInvocation.cpp:174–177 — 10% pointer comparison when derived exist.
	// C++: if (flip && has_pointer_type()) { ERROR_GUARD; return make_ptr_comparison(...); }
	// Always return that result (including nullptr) — do NOT fall through to
	// PickBinaryOp when ptr comparison fails (seed-2 e9211: UP U18 after matched
	// ptr-cmp events meant Go was on a different post-success path; fall-through
	// also invents scalar binary C++ never takes after a failed ptr-cmp attempt).
	if r.RndFlipcoin(10) {
		var env *TypeEnv
		if vs != nil {
			env = vs.Types
		}
		if env == nil {
			env = cg.Types
		}
		if env != nil && env.HasPointerType() {
			// ERROR_GUARD after flipcoin before call is implicit via HasError checks in callee
			if HasError() {
				return nil
			}
			return MakeRandomBinaryPtrComparison(r, opts, probs, vs, tables, cg, env)
		}
	}
	// FunctionInvocation.cpp:179–183 — do { pick } while (type->is_float() && !works)
	// FunctionInvocation.cpp:185 — assert(type); nil type allowed only for non-float paths (library)
	op := PickBinaryOp(r, opts)
	if typ != nil {
		isF := typ.IsFloat()
		// residual ERROR sticky — no invent soft-continue pick past IsFloat residual
		if HasError() {
			return nil
		}
		if isF {
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
	}
	// PickBinaryOp MAX / empty token — sticky no invent infix shell without live op
	opStr := op.BinaryOpC()
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		SetError(ErrGeneric)
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
	// FunctionInvocation.cpp:219–221 — assert(lhs_type && rhs_type) sticky
	lhsTy = flags.LHSType()
	rhsTy = flags.RHSType()
	if lhsTy == nil || rhsTy == nil {
		SetError(ErrGeneric)
		return nil
	}
	// non-arith/shift: keep flags for typing but Output ignores safe path (SafeOpsBinary filter)
	if !SafeOpsBinary(opStr) {
		// still keep flags (C++ does); Output uses standard tokens
	}

	// FunctionInvocation.cpp:201–204 — CreateFunctionInvocationBinary BEFORE operands.
	// Tmps advance process gensym_count; creating after LHS/RHS desynced all later
	// l_/g_/p_ names from C++ (seed-2: UP func_1,t_2,t_3,l_4… vs Go func_1,l_2…).
	tmp1, tmp2 := createBinarySafeTmps(*cg, vs, flags, op)
	// assert(blk): safe_ops require live block temps — fail closed, no invent bare ops
	if flags != nil && SafeOpsBinary(opStr) && tmp1 == "" {
		return nil
	}

	// FunctionInvocation.cpp:208–216 — LHS under dedicated accum + ambient effect_context
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
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
	// incomplete effect after lhs merge fails closed sticky (no invent RHS / soft re-pick)
	if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}

	// FunctionInvocation.cpp:222 — snapshot facts before RHS (ordered merge)
	// incomplete GlobalFacts fail closed sticky (no invent cleaned snapshot)
	var factsCopy []*FactPointTo
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
		factsCopy = CloneFactSlice(cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-binary past CloneFactSlice residual
		if HasError() {
			return nil
		}
	}

	var right *Expression
	if op == BinLShift || op == BinRShift {
		// FunctionInvocation.cpp:236–244 — rnd_flipcoin(ShiftByNonConstantProb())
		// C++ Probabilities singleton; nil session → 0% (no invent hard-coded 50)
		shiftNonConst := 0
		if probs != nil {
			shiftNonConst = probs.Single(PShiftByNonConstantProb)
		} else if p := ProcessProbabilities(); p != nil {
			shiftNonConst = p.Single(PShiftByNonConstantProb)
		}
		// not_constant = flip; constant path when !not_constant
		if !r.RndFlipcoin(uint32(shiftNonConst)) {
			// FunctionInvocation.cpp:241 — make_random_upto(lhs_type->SizeInBytes() * 8)
			// Type always live after flags assert; SizeInBytes 0 is incomplete
			// (no invent default 32-bit width)
			if lhsTy != nil {
				sb := lhsTy.SizeInBytes()
				// residual ERROR sticky — no invent soft-shift const past SizeInBytes residual
				if HasError() {
					return nil
				}
				if sb > 0 {
					bits := uint32(sb * 8)
					// Constant::make_random_upto; ERROR_GUARD — no invent shell with nil Con
					if c := MakeRandomUpto(bits, r); c != nil && !HasError() {
						// FunctionInvocation.cpp:241–243 — Constant::make_random_upto as RHS.
						// Not Expression::make_random — C++ does NOT bump expr_depth here
						// (depth++ only in Expression.cpp:213–218 after make_random).
						// Extra bump made Go hit max_expr_depth one level early (seed-2 e9188:
						// UP U120 Function vs Go depth-gate filtered Function → Constant).
						right = &Expression{Term: TermConstant, Con: c}
					}
				}
			}
		}
	}
	if right == nil {
		if IsOrderedBinary(op) {
			// FunctionInvocation.cpp:224–226 — RHS under original cg_context
			right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, false, false, MaxTermTypes, cg.ExprDepth)
		} else {
			// FunctionInvocation.cpp:228–234 / 255 — combined effect_context + separate accum
			// Incomplete lhs accum fails closed sticky (no invent RHS under incomplete ambient)
			rhsAccum := EmptyEffect()
			rhsCG := cg.CloneSubcontext()
			rhsCtx := cg.EffectContext().AddEffectOpts(lhsAccum, true)
			if !EffectComplete(rhsCtx) {
				SetError(ErrGeneric)
				return nil
			}
			rhsCG.effectContext = rhsCtx
			rhsCG.EffectAccum = &rhsAccum
			rhsCG.EffectStm = EmptyEffect()
			// Shift constant path may already have set right above.
			if right == nil {
				right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, rhsTy, nil, false, false, MaxTermTypes, rhsCG.ExprDepth)
			}
			// FunctionInvocation.cpp:246–253 — div/mod zero-guard BEFORE merge (C++ order)
			// rhs->equals(0) || rhs->is_0_or_1() (all comparison Funcalls are is_0_or_1).
			// Then rnd_upto(MAX_BINARY_OP, filter) rejecting mod/div/shifts.
			if right != nil && !HasError() && (op == BinMod || op == BinDiv) {
				eq0 := right.EqualsInt(0)
				if HasError() {
					return nil
				}
				is01 := right.Is0Or1()
				if HasError() {
					return nil
				}
				if eq0 || is01 {
					lhsF, rhsF := false, false
					if lhsTy != nil {
						lhsF = lhsTy.IsFloat()
						if HasError() {
							return nil
						}
					}
					if rhsTy != nil {
						rhsF = rhsTy.IsFloat()
						if HasError() {
							return nil
						}
					}
					if !lhsF && !rhsF {
						f := NewVectorFilterItems([]int{
							int(BinMod), int(BinDiv), int(BinLShift), int(BinRShift),
						}, FilterModeOut)
						op = BinaryOp(r.RndUptoFilter(uint32(MaxBinaryOp), f))
						if HasError() {
							return nil
						}
						opStr = op.BinaryOpC()
					}
				}
			}
			// FunctionInvocation.cpp:255 — merge_param_context(rhs)
			cg.MergeParamContext(rhsCG, true)
			if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return nil
			}
		}
	}
	// FunctionInvocation.cpp:257 — ERROR_GUARD_AND_DEL2
	if right == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:266–273 — CompatibleChecker hard-fail (nullptr)
	if CompatibleCheckExprs(opts, left, right) {
		// residual ERROR sticky — no invent soft-binary past CompatibleCheck residual true
		if HasError() {
			return nil
		}
		SetError(ErrCompatibleCheck)
		return nil
	}
	// residual ERROR sticky — no invent soft-binary past CompatibleCheck residual false
	if HasError() {
		return nil
	}
	// FunctionInvocation.cpp:275–279 — ordered ops merge facts (short-circuit RHS may skip)
	if IsOrderedBinary(op) && cg.FM != nil && factsCopy != nil {
		if !MakeupNewVarFacts(&factsCopy, cg.FM.GlobalFacts) ||
			!FactsComplete(factsCopy) || !FactsComplete(cg.FM.GlobalFacts) {
			// incomplete makeup/merge base — fail closed sticky, no invent bare binary
			SetError(ErrGeneric)
			return nil
		}
		_ = MergeFacts(&cg.FM.GlobalFacts, factsCopy)
		// residual ERROR sticky — no invent soft-binary past MergeFacts residual
		if HasError() {
			return nil
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			SetError(ErrGeneric)
			return nil
		}
	}
	inv := &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}, Safe: flags, Tmp1: tmp1, Tmp2: tmp2}
	inv.setOutOpts(opts)
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
	// FunctionInvocation always has RNG + CGContext; sticky no invent ptr-cmp shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// no pointer types: soft re-pick scalar binary (not broken IR)
	if env == nil || !env.HasPointerType() {
		return nil
	}
	// incomplete ambient fails closed sticky (no invent ptr cmp / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:295–296 — rnd_flipcoin(50) ? eCmpEq : eCmpNe
	// (true → ==, false → !=). Do not invert polarity.
	op := BinCmpNe
	if r.RndFlipcoin(50) {
		op = BinCmpEq
	}
	opStr := op.BinaryOpC()
	// FunctionInvocation.cpp:297–299 — SafeOpFlags::make_random_binary(get_int_type(),
	// nullptr, nullptr, sOpBinary, op) BEFORE choose_random_pointer_type.
	// Output for ptr_cmp still uses standard ==/!= (not safe_* wrappers), but the
	// RNG draws for signedness + size still run (seed-2 e129 was F50 from flags).
	flags := MakeRandomBinaryKind(r, opts, probs, GetIntType(), nil, nil, SafeOpBinary, op)
	// ERROR_GUARD after make_random_binary; no soft invent nil-flags ptr comparison
	if flags == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:301–303 — CreateFunctionInvocationBinary (no RNG for
	// ==/!=: safe_ops false → no tmp vars).
	// FunctionInvocation.cpp:304 — Type::choose_random_pointer_type after flags
	ptrTy := env.ChooseRandomPointerType(r)
	if ptrTy == nil {
		return nil
	}
	// FunctionInvocation.cpp:307–313 — LHS under ambient + NO_DANGLING_PTR + no_func=true
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
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
	// incomplete effect after lhs merge fails closed sticky (no invent RHS / soft re-pick)
	if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return nil
	}

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
		rhsCG := cg.CloneSubcontext()
		// FunctionInvocation.cpp:338–342 — effect_context + lhs_eff_accum
		// Incomplete lhs accum fails closed sticky (no invent RHS under incomplete ambient)
		rhsCtx := cg.EffectContext().AddEffect(lhsAccum)
		// residual ERROR sticky — no invent soft-continue RHS past AddEffect residual
		if HasError() {
			return nil
		}
		if !EffectComplete(rhsCtx) {
			SetError(ErrGeneric)
			return nil
		}
		rhsCG.effectContext = rhsCtx
		rhsCG.EffectAccum = &rhsAccum
		rhsCG.EffectStm = EmptyEffect()
		rhsCG.Flags |= FlagNoDanglingPtr
		right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, ptrTy, nil, true, false, tt, rhsCG.ExprDepth)
		// FunctionInvocation.cpp:345
		cg.MergeParamContext(rhsCG, true)
		if HasError() || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
	}
	if right == nil || HasError() {
		return nil
	}
	// FunctionInvocation.cpp:349 — typecast RHS to LHS type if needed (lang_cpp)
	lt := left.GetType()
	// residual ERROR sticky — no invent ptr-cmp past GetType residual hole
	if HasError() {
		return nil
	}
	right.CheckAndSetCastOpts(lt, opts)
	// residual ERROR sticky — no invent ptr-cmp past CheckAndSetCast residual hole
	if HasError() {
		return nil
	}
	// FunctionInvocation.cpp:358 — bookkeeping
	RecordPointerComparisons(left, right)
	// residual ERROR sticky — no invent ptr-cmp past RecordPointerComparisons residual hole
	if HasError() {
		return nil
	}
	// flags already drawn before pointer type (FunctionInvocation.cpp:297–304 order)
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
	// FunctionInvocation.cpp:565+ — always has RNG + live operands sticky
	// (no invent binary shell without them / soft re-pick past holes)
	if r == nil || lhs == nil || rhs == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent binary shell / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:565 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationBinary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationBinary) == BadDepth {
		return nil
	}
	// invalid / MAX op — sticky no invent empty Binary token shell
	opStr := op.BinaryOpC()
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		SetError(ErrGeneric)
		return nil
	}
	lt, rt := lhs.GetType(), rhs.GetType()
	// residual ERROR sticky — no invent binary shell past GetType residual hole
	if HasError() {
		return nil
	}
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
	// FunctionInvocation.cpp always has RNG + CGContext sticky; no invent unary shell without them
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent unary / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:143 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomUnary, nullptr)
	if DepthGuardByType(opts, DtFunctionInvocationRandomUnary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:144 — assert(type) sticky
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:146–149 — do { pick } while (float && !works); no soft invent invalid
	var uop UnaryOp
	validU := false
	// C++ unbounded do-while; cap high (no soft invent invalid float op)
	for tries := 0; tries < 256; tries++ {
		uop = PickUnaryOp(r, opts)
		isF := typ.IsFloat()
		// residual ERROR sticky — no invent soft-continue unary pick past IsFloat residual
		if HasError() {
			return nil
		}
		if !isF || UnaryOpWorksForFloat(uop) {
			validU = true
			break
		}
	}
	if !validU {
		return nil
	}
	// PickUnaryOp MAX / empty token — sticky no invent unary shell without live op
	op := uop.UnaryOpC()
	if int(uop) < 0 || int(uop) >= MaxUnaryOp || op == "" {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:151–155 — always make_random_unary then operand type from flags
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	var probs *Probabilities
	if vs != nil {
		probs = vs.Probs
	}
	flags := MakeRandomUnary(r, opts, probs, typ, nil, uop)
	// FunctionInvocation.cpp:152–154 — ERROR_GUARD; type = flags->get_lhs_type(); assert(type) sticky
	if flags == nil {
		return nil
	}
	argTy := flags.LHSType()
	if argTy == nil {
		SetError(ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:156–159 — CreateFunctionInvocationUnary BEFORE operand
	// (gensym t_ before any names the operand creates).
	tmp1 := createUnarySafeTmp(*cg, vs, flags)
	// FunctionInvocationUnary.cpp:57 assert(blk) — fail closed when flags but no tmp
	if flags != nil && tmp1 == "" {
		return nil
	}
	// FunctionInvocation.cpp:160–162 — Expression::make_random(cg, type) — no_func=false
	arg := MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, false, false, MaxTermTypes, cg.ExprDepth)
	// FunctionInvocation.cpp:161 — ERROR_GUARD_AND_DEL1
	if arg == nil || HasError() {
		return nil
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}, Safe: flags, Tmp1: tmp1}
	inv.setOutOpts(opts)
	return inv
}

// createBinarySafeTmps mirrors FunctionInvocationBinary::CreateFunctionInvocationBinary
// temp allocation. FunctionInvocationBinary.cpp:59–75 — always when flags && safe_ops;
// no soft invent skip on !MathNoTmp or float size.
func createBinarySafeTmps(cg CGContext, vs *VariableSelector, flags *SafeOpFlags, op BinaryOp) (tmp1, tmp2 string) {
	if flags == nil || !SafeOpsBinary(op.BinaryOpC()) {
		return "", ""
	}
	// FunctionInvocationBinary.cpp:59–75 — no EffectComplete/FactsComplete gate before
	// create_new_tmp_var; only flags + safe_ops + blk. Extra gates skipped gensym t_
	// while still failing the binary (name desync vs C++).
	blk := currentBlock(cg)
	// FunctionInvocationBinary.cpp:68 — assert(blk); non-sticky soft re-pick when no stack
	// (sticky would poison library MakeBinary without parent block)
	if blk == nil {
		return "", ""
	}
	// FunctionInvocationBinary.cpp:64–66 — flags_to_type must yield simple type sticky
	ty1 := flags.LHSType()
	if ty1 == nil {
		SetError(ErrGeneric)
		return "", ""
	}
	if !ty1.IsSimple() {
		// residual ERROR sticky — no invent soft-tmp past IsSimple residual
		if HasError() {
			return "", ""
		}
		SetError(ErrGeneric)
		return "", ""
	}
	// residual ERROR sticky — no invent soft-tmp past IsSimple residual true
	if HasError() {
		return "", ""
	}
	st := ty1.Simple()
	var sym *GenSym
	if vs != nil {
		sym = &vs.Sym
	}
	tmp1 = blk.CreateNewTmpVar(sym, st)
	// residual ERROR sticky — no invent soft-tmp past CreateNewTmpVar residual
	if HasError() {
		return "", ""
	}
	st2 := st
	// FunctionInvocationBinary.cpp:64–78 — flags_to_type(op2) always live for shifts;
	// sticky no invent type1 stand-in for type2 past missing/non-simple RHS type shell
	if op == BinLShift || op == BinRShift {
		ty := flags.RHSType()
		if ty == nil {
			SetError(ErrGeneric)
			return "", ""
		}
		if !ty.IsSimple() {
			if HasError() {
				return "", ""
			}
			SetError(ErrGeneric)
			return "", ""
		}
		if HasError() {
			return "", ""
		}
		st2 = ty.Simple()
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
	// FunctionInvocationUnary.cpp:51–60 — no ambient EffectComplete gate before
	// create_new_tmp_var (same as binary Create* path).
	blk := currentBlock(cg)
	// FunctionInvocationUnary.cpp:57 — assert(blk); non-sticky soft re-pick when no stack
	if blk == nil {
		return ""
	}
	// flags_to_type must yield simple type sticky
	ty := flags.LHSType()
	if ty == nil {
		SetError(ErrGeneric)
		return ""
	}
	if !ty.IsSimple() {
		// residual ERROR sticky — no invent soft-tmp past IsSimple residual
		if HasError() {
			return ""
		}
		SetError(ErrGeneric)
		return ""
	}
	// residual ERROR sticky — no invent soft-tmp past IsSimple residual true
	if HasError() {
		return ""
	}
	var sym *GenSym
	if vs != nil {
		sym = &vs.Sym
	}
	tmp := blk.CreateNewTmpVar(sym, ty.Simple())
	// residual ERROR sticky — no invent soft-tmp past CreateNewTmpVar residual
	if HasError() {
		return ""
	}
	return tmp
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
	// FunctionInvocation.cpp always has RNG + CGContext; sticky Failed
	// (no invent invoke shell / soft re-pick past hole)
	if r == nil || cg == nil {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent choose/build / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		SetError(ErrGeneric)
		return &Invocation{Failed: true}
	}
	// Match type for choose_func: nil means any return type (C++ type=0).
	matchType := typ
	// FunctionInvocation.cpp:71–73 path — non-simple/void force user path (type known)
	if typ != nil {
		pt := typ.PtrType()
		// residual ERROR sticky — no invent soft-std path past PtrType residual
		if HasError() {
			return &Invocation{Failed: true}
		}
		if pt != nil {
			stdFunc = false
		} else {
			simple := typ.IsSimple()
			// residual ERROR sticky — no invent soft-std path past IsSimple residual
			if HasError() {
				return &Invocation{Failed: true}
			}
			if simple && typ.Simple() == EVoid {
				stdFunc = false
			}
		}
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
				// ERROR_GUARD when choose_random fails; sticky Failed (no invent GetIntType return)
				if sigType == nil {
					if !HasError() {
						SetError(ErrGeneric)
					}
					return &Invocation{Failed: true}
				}
			}
			// Statement probability table is process/session singleton; sticky no invent second table
			stmtTab := ProcessStmtTab()
			if stmtTab == nil {
				SetError(ErrGeneric)
				return &Invocation{Failed: true}
			}
			// FunctionInvocationUser.cpp:179 — pass Expression qfer into signature RV qfer
			fi = BuildInvocationAndFunction(r, opts, probs, vs, tables, stmtTab, cg, list, sigType, qfer)
			if fi != nil && !fi.Failed && cg.CurrentFunc != nil && fi.User != nil {
				cg.CurrentFunc.FactChanged = cg.CurrentFunc.FactChanged || fi.User.FactChanged
			}
		} else {
			// FunctionInvocation.cpp:102–106 — failed when at max funcs (complete soft Failed)
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
	// incomplete null without sticky error → Failed shell (ExpressionFuncall var soft re-pick)
	// sticky ERROR already set → still Failed shell for callers that only check .Failed
	// no invent success IR
	if fi == nil {
		return &Invocation{Failed: true}
	}
	return fi
}
