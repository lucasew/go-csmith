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
	fi.setOutOptsSess(testAmbientSession, opts)
}

// setOutOptsSess is setOutOpts with explicit session residual sticky.
func (fi *Invocation) setOutOptsSess(s *Session, opts Options) {
	if fi == nil {
		sessNoteError(s, ErrGeneric)
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
// Ambient ProcessOptions bridge; emit prefers OutputSess / OutputOptsSess.
func (fi *Invocation) Output() string {
	return fi.OutputSess(testAmbientSession)
}

// OutputSess is Output with Options/sticky from an explicit session bag.
func (fi *Invocation) OutputSess(s *Session) string {
	return fi.OutputOptsSess(s, sessOpts(s))
}

// OutputOpts is Output with explicit session Options (arg / wrapper emit).
func (fi *Invocation) OutputOpts(opts Options) string {
	return fi.OutputOptsSess(testAmbientSession, opts)
}

func (fi *Invocation) OutputOptsSess(s *Session, opts Options) string {
	// FunctionInvocation always live at Output; sticky no invent empty call without it
	if fi == nil {
		sessNoteError(s, ErrGeneric)
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
			sessNoteError(s, ErrGeneric)
			return ""
		}
		var parts []string
		for _, a := range fi.Args {
			if a == nil {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			out := a.OutputOptsSess(s, opts)
			// residual ERROR sticky — no invent soft-continue later args past Output residual
			if sessHasError(s) {
				return ""
			}
			if out == "" {
				// incomplete arg IR — sticky fail closed whole call
				sessNoteError(s, ErrGeneric)
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
				sessNoteError(s, ErrGeneric)
				return ""
			}
			if len(fi.Args) < 1 || fi.Args[0] == nil {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			a0 := fi.Args[0].OutputOptsSess(s, opts)
			// residual ERROR sticky — no invent soft-empty unary past Output residual
			if sessHasError(s) {
				return ""
			}
			if a0 == "" {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			out := fi.outputUnarySess(s, a0)
			// residual ERROR sticky — no invent soft-empty unary past outputUnary residual
			if sessHasError(s) {
				return ""
			}
			return out
		}
		// binary: need two live args with non-empty Output
		if len(fi.Args) < 2 || fi.Args[0] == nil || fi.Args[1] == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if _, ok := BinaryOpFromString(fi.Binary); !ok && fi.Binary != "+" {
			// invalid op sticky (except bare + for array mutate without flags)
			sessNoteError(s, ErrGeneric)
			return ""
		}
		a0 := fi.Args[0].OutputOptsSess(s, opts)
		// residual ERROR sticky — no invent soft-continue a1 past a0 Output residual
		if sessHasError(s) {
			return ""
		}
		a1 := fi.Args[1].OutputOptsSess(s, opts)
		// residual ERROR sticky — no invent soft-empty binary past a1 Output residual
		if sessHasError(s) {
			return ""
		}
		if a0 == "" || a1 == "" {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		out := fi.outputBinarySess(s, a0, a1)
		// residual ERROR sticky — no invent soft-empty binary past outputBinary residual
		if sessHasError(s) {
			return ""
		}
		return out
	}
	// incomplete non-user non-std shell sticky
	sessNoteError(s, ErrGeneric)
	return ""
}

// outputUnary mirrors FunctionInvocationUnary::Output.
// FunctionInvocationUnary.cpp:192–243.
func (fi *Invocation) outputUnary(a0 string) string {
	return fi.outputUnarySess(testAmbientSession, a0)
}

// outputUnarySess is outputUnary registering wrapper names on bag s.
func (fi *Invocation) outputUnarySess(s *Session, a0 string) string {
	// FunctionInvocationUnary.cpp:200–224 — eMinus + avoid_signed_overflow
	if fi.Unary == "-" && fi.Safe != nil && fi.OutSafeMath {
		// float size: standard minus (no safe unary float func)
		// FunctionInvocationUnary.cpp:203 / 220–223
		if fi.Safe.Size == SafeFloat {
			return fmt.Sprintf("(-%s)", a0)
		}
		fname := fi.Safe.UnaryMinusFuncNameSess(s)
		// SafeOpFlags.cpp:325 assert / empty name → cast path (no invent wrapper name)
		if fname == "" {
			return unaryCastMinusSess(s, fi.Safe.SizeTokenSess(s), a0)
		}
		id := SafeOpFlagsToIDSess(s, fname)
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
		return unaryCastMinusSess(s, fi.Safe.SizeTokenSess(s), a0)
	}
	// FunctionInvocationUnary.cpp:229–240 — ePlus/eNot/eBitNot or non-safe minus
	if fi.Unary == "-" && fi.Safe != nil && !fi.OutSafeMath {
		// need_cast when Safe flags exist but avoid_signed_overflow off
		return unaryCastMinusSess(s, fi.Safe.SizeTokenSess(s), a0)
	}
	// FunctionInvocationUnary.cpp:192–242 — outer "(" + op + [cast] + arg.Output + ")"
	// C++ does NOT wrap arg in extra parens after op (param_value[0]->Output only).
	// Unfair "(%s(%s))" produced (~((safe_unary…))) vs C++ (~(safe_unary…)).
	switch fi.Unary {
	case "+", "-", "!", "~":
		return fmt.Sprintf("(%s%s)", fi.Unary, a0)
	default:
		// assert invalid operator — sticky no invent emit
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// unaryCastMinus is (-(size)arg); empty size token sticky fail closed (no invent "(-()x)").
func unaryCastMinus(cast, a0 string) string {
	return unaryCastMinusSess(testAmbientSession, cast, a0)
}

func unaryCastMinusSess(s *Session, cast, a0 string) string {
	if cast == "" || a0 == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return fmt.Sprintf("(-(%s)%s)", cast, a0)
}

// outputBinary mirrors FunctionInvocationBinary::Output.
// FunctionInvocationBinary.cpp:350–426.
func (fi *Invocation) outputBinary(a0, a1 string) string {
	return fi.outputBinarySess(testAmbientSession, a0, a1)
}

// outputBinarySess is outputBinary registering wrapper names on bag s.
func (fi *Invocation) outputBinarySess(s *Session, a0, a1 string) string {
	// FunctionInvocationBinary.cpp:357–361 — mutated array subscript add without flags
	if fi.Binary == "+" && fi.Safe == nil {
		return fmt.Sprintf("(%s + %s)", a0, a1)
	}
	// FunctionInvocationBinary.cpp:363–399 — arith/shift + avoid_signed_overflow
	if fi.Safe != nil && SafeOpsBinary(fi.Binary) && fi.OutSafeMath {
		if fname := fi.Safe.BinaryFuncNameSess(s, fi.Binary); fname != "" {
			id := SafeOpFlagsToIDSess(s, fname)
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
			return binaryCastOpSess(s, fi.Safe.SizeTokenSess(s), a0, fi.Binary, a1)
		}
	}
	// need_cast when Safe present but SafeMath off for arith/shift
	if fi.Safe != nil && SafeOpsBinary(fi.Binary) && !fi.OutSafeMath {
		return binaryCastOpSess(s, fi.Safe.SizeTokenSess(s), a0, fi.Binary, a1)
	}
	return fmt.Sprintf("(%s %s %s)", a0, fi.Binary, a1)
}

// binaryCastOp is ((cast)a0 op (cast)a1); empty cast sticky fail closed (no invent "(()a + ()b)").
func binaryCastOp(cast, a0, op, a1 string) string {
	return binaryCastOpSess(testAmbientSession, cast, a0, op, a1)
}

func binaryCastOpSess(s *Session, cast, a0, op, a1 string) string {
	if cast == "" || a0 == "" || a1 == "" || op == "" {
		sessNoteError(s, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent conflict-filter under hole shells)
	if cg != nil {
		if !EffectComplete(cg.EffectContext()) ||
			(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
			!EffectComplete(cg.EffectStm) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
	}
	var ok, okBuiltin []*Function
	for _, f := range funcs {
		// pre-validated FunctionsComplete
		if f == exclude || !f.IsEffectKnownSess(sessFromCG(cg)) {
			// residual ERROR sticky — no invent soft-continue then pick later past IsEffectKnown hole
			if hasErrCG(cg) {
				return nil
			}
			// is_effect_known() == false for Unbuilt/Building
			continue
		}
		// residual ERROR sticky — no invent soft-continue known-true past IsEffectKnown residual
		if hasErrCG(cg) {
			return nil
		}
		// Function.cpp:288–289 — type->is_convertable(return_type)
		// C++ always has live return_type*; nil is incomplete IR
		// fail closed sticky whole choose (no invent soft-skip broken func as absent)
		if ret != nil && f.ReturnType == nil {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		if ret != nil && f.ReturnType != nil {
			ok := ret.IsConvertableOptsSess(sessFromCG(cg), f.ReturnType, opts)
			// residual ERROR sticky — no invent soft-continue then pick later past IsConvertable residual
			if hasErrCG(cg) {
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
				noteErrCG(cg, ErrGeneric)
				return nil
			}
			if !qfer.MatchSess(sessFromCG(cg), f.RV.Qfer, false) {
				// residual ERROR sticky — no invent soft-continue then pick later past Match hole
				if hasErrCG(cg) {
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-keep match past Match residual true path
			if hasErrCG(cg) {
				return nil
			}
		}
		// incomplete callee FEffect fails closed sticky (no invent skip as conflict past hole)
		if cg != nil && !EffectComplete(f.FEffect) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// Function.cpp:303–306 — in_conflict with callee feffect
		if cg != nil && cg.InConflict(f.FEffect) {
			// residual ERROR sticky — no invent soft-continue conflict-skip past hard IR
			if hasErrCG(cg) {
				return nil
			}
			continue
		}
		// residual ERROR sticky — no invent soft-keep after InConflict residual false
		if hasErrCG(cg) {
			return nil
		}
		// Function.cpp:307–313 — strict_volatile_rule
		if opts.StrictVolatileRule && cg != nil {
			if !f.FEffect.IsSideEffectFreeSess(sessFromCG(cg)) && !cg.EffectContext().IsSideEffectFreeSess(sessFromCG(cg)) {
				// residual ERROR sticky — no invent soft-continue then pick later past SE residual
				if hasErrCG(cg) {
					return nil
				}
				continue
			}
			// residual ERROR sticky — no invent soft-continue SE-free path past residual hole
			if hasErrCG(cg) {
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
		if p > 0 && r.RndFlipcoinSess(sessFromCG(cg), uint32(p)) {
			f = getOneFunctionSess(sessFromCG(cg), r, okBuiltin)
		}
	}
	if f == nil {
		f = getOneFunctionSess(sessFromCG(cg), r, ok)
	}
	return f
}

// getOneFunction mirrors Function::get_one_function — random pick.
// Function.cpp:262–276.
func getOneFunction(r *Rng, funcs []*Function) *Function {
	return getOneFunctionSess(testAmbientSession, r, funcs)
}

// getOneFunctionSess is getOneFunction with explicit session residual sticky.
func getOneFunctionSess(s *Session, r *Rng, funcs []*Function) *Function {
	// Function.cpp:262–276 — rnd_upto(ok_size) when n>1; sticky no invent funcs[0]
	n := len(funcs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return funcs[0]
	}
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return funcs[r.RndUptoSess(s, uint32(n))]
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
	return r.RndFlipcoinSess(rSess(r), 80)
}

// GetFirstFunction mirrors GetFirstFunction — first entry in FuncList / func_1.
// Function.cpp / FunctionInvocationUser.cpp:274.
// Nil list / empty Funcs is complete miss (no invent first of empty; isolated
// BuildUserInvocation may pass nil list as "no first-function identity").
// Function* always live at [0]; nil hole sticky (no invent scan later).
func GetFirstFunction(list *FunctionList) *Function {
	return GetFirstFunctionSess(testAmbientSession, list)
}

// GetFirstFunctionSess is GetFirstFunction with explicit session residual sticky.
func GetFirstFunctionSess(s *Session, list *FunctionList) *Function {
	if list == nil || len(list.Funcs) == 0 {
		return nil
	}
	// C++ first_function is funcs[0]; incomplete IR at front sticky fail closed
	if list.Funcs[0] == nil {
		sessNoteError(s, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent param gen / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return &Invocation{User: callee, Failed: true}
	}
	fi := &Invocation{User: callee}
	// FunctionInvocationUser.cpp:249–270 — running effect context across params
	// Effect.cpp:84–89 — ambient is value-copied into child contexts (deep vector copy).
	running := cg.EffectContext().detachMaps()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:256–258 — v->type / &v->qfer; sticky no invent soft-skip param hole
		if p == nil || p.Type == nil {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		// FunctionInvocationUser.cpp:252–254 — param_cg(cg, running_eff_context, &param_eff_accum)
		// CGContext.cpp:74–82 — curr_rhs(nullptr) on (cgc, eff_context, accum)
		paramAccum := EmptyEffect()
		paramCG := cg.CloneSubcontext()
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		paramCG.CurrRHS = nil
		// Expression::make_random_param bumps paramCG.ExprDepth; merge copies it
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:259 — ERROR_GUARD(false); sticky error or null param → fail
		// null param without sticky invents soft re-pick past ERROR_GUARD miss
		if arg == nil || hasErrCG(cg) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:261 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		// residual ERROR sticky — no invent param past CheckAndSetCast residual hole
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:264–267 — running first, then merge_param_context(default include_lhs=false)
		// Incomplete param accum fails closed sticky (no invent more params / soft re-pick past holes)
		running = running.AddEffectSess(sessFromCG(cg), paramAccum)
		// residual ERROR sticky — no invent soft-continue later params past AddEffect residual
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(running) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		cg.MergeParamContext(paramCG, false)
		// residual ERROR sticky — no invent soft-continue later params past MergeParam residual
		if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
	}

	// FunctionInvocationUser.cpp:272–301
	fi.Failed = false
	first := GetFirstFunctionSess(sessFromCG(cg), list)
	// skip revisit for first function (func_1) — no params, single call, DFA hack
	needRev := callee != first && callee.NeedsRevisitSess(sessFromCG(cg))
	// residual ERROR sticky — no invent soft-skip revisit past NeedsRevisit residual
	if hasErrCG(cg) {
		fi.Failed = true
		return fi
	}
	if needRev {
		// FunctionInvocationUser.cpp:277–291 — revisit with accum_eff_context
		// Incomplete AccumEffContext fails closed sticky (no invent revisit under incomplete ambient)
		effectAccum := EmptyEffect()
		effectContext := cg.EffectContext().AddEffectSess(sessFromCG(cg), callee.AccumEffContext)
		// residual ERROR sticky — no invent soft-continue revisit past AddEffect residual
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(effectContext) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:282–284 —
		//   CGContext new_context(cg_context, func, effect_context, &effect_accum);
		// CGContext.cpp:85–93 — current_func=callee, blk_depth=0, expr_depth=0,
		// extend_call_chain. Must clear CurrRHS/EffectStm from caller (ExpressionAssign
		// param gen leaves CurrRHS set); otherwise Lhs::visit_facts overlap / ptr_modified
		// checks against the outer RHS and falsely fail body revisit (seed-2 func_49
		// BUILD_REV after ExpressionAssign param / first_div e37241).
		newCG := cg.CloneSubcontext()
		newCG.CurrentFunc = callee
		newCG.effectContext = effectContext
		newCG.EffectAccum = &effectAccum
		newCG.EffectStm = EmptyEffect()
		newCG.CurrRHS = nil
		newCG.ExprDepth = 0
		newCG.BlkDepth = 0
		newCG.ExtendCallChain(*cg)
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:284 — revisit(fm->global_facts, new_context)
		// where fm = get_fact_mgr(&cg_context) is the CALLER FactMgr. C++ mutates
		// caller global_facts in place (handover drops frame locals, then renew
		// restores from inputs_copy). Do not deep-clone then reinstall: that path
		// can drop mid-gen may-null lattice (seed-2 first_div 10107 l_233).
		if cg.FM == nil {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		ok := RevisitUserInvocation(fi, &cg.FM.GlobalFacts, &newCG, opts)
		fi.Failed = !ok
		if ok {
			// FunctionInvocationUser.cpp:287–289 — gen-time revisit handoff:
			//   assert(cg_context.get_current_block());
			//   add_visible_effect(*accum, get_current_block());
			// Visit path uses curr_blk (FunctionInvocation.cpp:543–546 / VisitFactsInvocation).
			// Do not prefer AnalysisBlock here: mid-gen FP leaves CurrBlk on a nested
			// statement parent; C++ gen always folds visible effect at stack top.
			// Incomplete effect hand-over fails closed sticky (no invent silent Incomplete FEffect)
			blk := cg.CurrentBlock()
			if blk != nil {
				cg.AddVisibleEffectAt(effectAccum, blk)
				if hasErrCG(cg) {
					fi.Failed = true
					return fi
				}
			}
			if !EffectComplete(effectAccum) || !EffectComplete(callee.FEffect) {
				noteErrCG(cg, ErrGeneric)
				fi.Failed = true
				return fi
			}
			callee.FEffect = callee.FEffect.AddExternalEffectWithCallersSess(sessFromCG(cg), effectAccum, cg.CallChain)
			if !EffectComplete(callee.FEffect) {
				noteErrCG(cg, ErrGeneric)
				fi.Failed = true
				return fi
			}
		}
	} else {
		// FunctionInvocationUser.cpp:293–297 — static effect, no re-analyze
		// add_external_effect(func->get_feffect())
		if callee.IsEffectKnownSess(sessFromCG(cg)) {
			// residual ERROR sticky — no invent static-effect path past IsEffectKnown hole
			if hasErrCG(cg) {
				fi.Failed = true
				return fi
			}
			cg.AddExternalEffect(callee.FEffect)
			if hasErrCG(cg) {
				fi.Failed = true
				return fi
			}
		} else if hasErrCG(cg) {
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
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	if ReachMaxFunctions(list, opts) {
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:175 — assert(type); sticky Failed (no invent return type)
	if retType == nil {
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent signature/params past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	// FunctionInvocationUser.cpp:179 — make_random_signature(cg, type, qfer)
	// Do not invent nil qfer when caller passed one (seed-2 e447: RV qfer draws
	// differed vs always RandomQualifiersDefaultProbs).
	callee := MakeRandomSignature(r, opts, probs, vs, &vs.Sym, *cg, retType, qfer, list)
	if callee == nil {
		// signature ERROR_GUARD sticky when not already set
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return &Invocation{Failed: true}
	}

	// FunctionInvocationUser.cpp:181–197 — build all parameters before body
	fi := &Invocation{User: callee}
	running := cg.EffectContext().detachMaps()
	for _, p := range callee.Param {
		// FunctionInvocationUser.cpp:185–187 — v->type; sticky no invent soft-skip param hole
		if p == nil || p.Type == nil {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		ty := p.Type
		q := p.Qfer
		qfer := &q
		paramAccum := EmptyEffect()
		// CGContext.cpp:74–82 — param context deep-copies iv_bounds; curr_rhs(nullptr)
		paramCG := cg.CloneSubcontext()
		paramCG.effectContext = running
		paramCG.EffectAccum = &paramAccum
		paramCG.EffectStm = EmptyEffect()
		paramCG.CurrRHS = nil
		arg := MakeRandomParam(r, opts, tables, vs, &paramCG, ty, qfer, paramCG.ExprDepth, list)
		// FunctionInvocationUser.cpp:186–187 — make_random_param; ERROR_GUARD after sticky error
		// null param without sticky invents soft re-pick past ERROR_GUARD miss
		if arg == nil || hasErrCG(cg) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:190 — check_and_set_cast (lang_cpp)
		arg.CheckAndSetCastOpts(ty, opts)
		// residual ERROR sticky — no invent param past CheckAndSetCast residual hole
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		fi.Args = append(fi.Args, arg)
		// FunctionInvocationUser.cpp:193–196 — running.add_effect then merge_param_context(default false)
		// Incomplete param accum fails closed sticky (no invent more params / soft re-pick past holes)
		running = running.AddEffectSess(sessFromCG(cg), paramAccum)
		// residual ERROR sticky — no invent soft-continue later params past AddEffect residual
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		if !EffectComplete(running) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		cg.MergeParamContext(paramCG, false)
		if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
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
	calFM := callee.PairedFactMgrSess(sessFromCG(cg))
	if calFM == nil {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	facts := []*FactPointTo{}
	if callerFM != nil {
		// incomplete caller GlobalFacts fail closed sticky (no invent cleaned handover)
		if !FactsComplete(callerFM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		facts = CloneFactSliceSess(sessFromCG(cg), callerFM.GlobalFacts)
		// residual ERROR sticky — no invent soft-handover past CloneFactSlice residual
		if hasErrCG(cg) {
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:206 — fm->global_facts = caller_fm->global_facts
		// C++ FactVec includes eUnionWrite; Go splits UnionFacts from GlobalFacts.
		// Incomplete caller UnionFacts fail closed sticky (no invent empty-complete handover).
		if !UnionFactsComplete(callerFM.UnionFacts) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		calFM.UnionFacts = CloneUnionFactSliceSess(sessFromCG(cg), callerFM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(calFM.UnionFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
	}
	calFM.CallerToCalleeHandover(fi.Args, &facts)
	// residual ERROR sticky — no invent soft-handover past CallerToCallee residual
	if hasErrCG(cg) {
		fi.Failed = true
		return fi
	}
	// FactMgr.cpp:324–353 — partition drops non-kept subjects of all Fact categories.
	// Filter UnionFacts after PT partition so globals/params/transitive pointees remain.
	if callerFM != nil {
		calFM.FilterUnionFactsForHandover(facts)
		if hasErrCG(cg) || !UnionFactsComplete(calFM.UnionFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
	}
	calFM.SetGlobalFacts(facts, "auto_function_invocation_832")

	// FunctionInvocationUser.cpp:208–210 — generate_body_with_known_params
	// Function.cpp:674–677 — CGContext(this, prev.effect_context, &effect_accum);
	// extend_call_chain(prev) uses CALLER get_current_block(). Do not set
	// bodyCG.CurrentFunc=callee here: generateBodyCore sets CurrentFunc then
	// ExtendCallChain(prev); if prev.CurrentFunc were already callee, CurrentBlock()
	// is empty and the caller's frame is omitted from call_chain (AddVisibleEffect /
	// RWDirective / frame visibility wrong → bloated lhsAccum on nested call sites,
	// seed-7 ChooseOKVar n=26 vs UP n=56).
	effectAccum := EmptyEffect()
	bodyCG := cg.CloneSubcontext()
	bodyCG.FM = calFM
	bodyCG.Flags = 0
	bodyCG.EffectAccum = &effectAccum
	// RWDirective + call chain set inside GenerateBodyWithKnownParams
	callee.GenerateBodyWithKnownParams(r, opts, probs, vs, tables, stmtTab, bodyCG)

	// FunctionInvocationUser.cpp:212–215 — ret_facts = map_facts_out[body]
	// then add_back_return_facts (full FactVec: ePointTo + eUnionWrite).
	// Incomplete out / add_back / missing body fail closed sticky.
	var retFacts []*FactPointTo
	var retUnions []*FactUnion
	if callee.Body == nil {
		// GenerateBody must leave live body; sticky Failed (no invent soft-skip without body)
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		fi.Failed = true
		return fi
	}
	out := calFM.GetMapFactsOut(callee.Body.StmID)
	outU := calFM.GetMapUnionFactsOut(callee.Body.StmID)
	// residual ERROR sticky — no invent soft-ret facts past GetMap* residual
	if hasErrCG(cg) {
		fi.Failed = true
		return fi
	}
	if !FactsComplete(out) || !UnionFactsComplete(outU) {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	retFacts = CloneFactSliceSess(sessFromCG(cg), out)
	retUnions = CloneUnionFactSliceDeepSess(sessFromCG(cg), outU)
	if hasErrCG(cg) || !FactsComplete(retFacts) || !UnionFactsComplete(retUnions) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		fi.Failed = true
		return fi
	}
	if !AddBackReturnFacts(callee.Body, calFM, &retFacts, &retUnions) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		fi.Failed = true
		return fi
	}
	// FunctionInvocationUser.cpp:214 / 358–365 — save_return_fact full FactVec
	// (ePointTo + eUnionWrite). Soft invent was PT-only registry so FactUnion
	// rhs_to_lhs_transfer for FuncCall params missed rv_fact (seed-213 p_34).
	fi.SaveReturnFactsSess(sessFromCG(cg), retFacts)
	fi.SaveReturnUnionFactsSess(sessFromCG(cg), retUnions)
	if hasErrCG(cg) {
		fi.Failed = true
		return fi
	}

	// FunctionInvocationUser.cpp:219 — setup_in_out_maps(true)
	calFM.SetupInOutMaps(true)

	// FunctionInvocationUser.cpp:221 — renew_facts(caller, ret_facts) full FactVec
	if callerFM != nil {
		// complete retFacts (may be empty nil) required; incomplete caller fails closed sticky
		// (no invent RenewFacts no-op success past incomplete then keep prior)
		if !FactsComplete(callerFM.GlobalFacts) || !FactsComplete(retFacts) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		_ = RenewFactsSess(sessFromCG(cg), &callerFM.GlobalFacts, retFacts)
		if !FactsComplete(callerFM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		// eUnionWrite half of ret_facts after add_back_return_facts (globals only
		// for caller renew — locals stripped by remove_function_local on body out).
		if !UnionFactsComplete(callerFM.UnionFacts) || !UnionFactsComplete(retUnions) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		retUF := GlobalUnionFactsOnlySess(sessFromCG(cg), retUnions)
		if !UnionFactsComplete(retUF) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		_ = RenewUnionFactsSess(sessFromCG(cg), &callerFM.UnionFacts, retUF)
		if !UnionFactsComplete(callerFM.UnionFacts) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			fi.Failed = true
			return fi
		}
		// FunctionInvocationUser.cpp:234–238 — new globals facts
		// Incomplete NewGlobals fails closed sticky (no invent soft-skip hole / partial push)
		if !VariablesComplete(callee.NewGlobals) {
			noteErrCG(cg, ErrGeneric)
			fi.Failed = true
			return fi
		}
		for _, v := range callee.NewGlobals {
			callerFM.AddNewVarFactAndUpdate(nil, v)
			if !FactsComplete(callerFM.GlobalFacts) {
				noteErrCG(cg, ErrGeneric)
				fi.Failed = true
				return fi
			}
		}
	}

	// FunctionInvocationUser.cpp:223–228 — effect hand-over
	// Incomplete external merge fails closed sticky (no invent silent Incomplete shells)
	if !EffectComplete(cg.EffectContext()) || !EffectComplete(callee.AccumEffContext) {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	callee.AccumEffContext = callee.AccumEffContext.AddExternalEffectSess(sessFromCG(cg), cg.EffectContext())
	if !EffectComplete(callee.AccumEffContext) {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	// feffect.add_external_effect(effect_accum, call_chain)
	if !EffectComplete(effectAccum) || !EffectComplete(callee.FEffect) {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	callee.FEffect = callee.FEffect.AddExternalEffectWithCallersSess(sessFromCG(cg), effectAccum, cg.CallChain)
	if !EffectComplete(callee.FEffect) {
		noteErrCG(cg, ErrGeneric)
		fi.Failed = true
		return fi
	}
	// also keep ComputeSummary body effect already applied in GenerateBody
	cg.AddVisibleEffectAt(effectAccum, cg.CurrentBlock())
	if hasErrCG(cg) {
		fi.Failed = true
		return fi
	}

	// FunctionInvocationUser.cpp:230–233 — new_globals hand-over
	if cg.CurrentFunc != nil && len(callee.NewGlobals) > 0 {
		if !VariablesComplete(callee.NewGlobals) {
			noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent binary / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:173 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomBinary, nullptr)
	if DepthGuardByTypeSess(sessFromCG(cg), opts, DtFunctionInvocationRandomBinary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:171+ — type may be nullptr (StatementExpr); no GetIntType invent
	// FunctionInvocation.cpp:174–177 — 10% pointer comparison when derived exist.
	// C++: if (flip && has_pointer_type()) { ERROR_GUARD; return make_ptr_comparison(...); }
	// Always return that result (including nullptr) — do NOT fall through to
	// PickBinaryOp when ptr comparison fails (seed-2 e9211: UP U18 after matched
	// ptr-cmp events meant Go was on a different post-success path; fall-through
	// also invents scalar binary C++ never takes after a failed ptr-cmp attempt).
	if r.RndFlipcoinSess(sessFromCG(cg), 10) {
		var env *TypeEnv
		if vs != nil {
			env = vs.Types
		}
		if env == nil {
			env = cg.Types
		}
		if env != nil && env.HasPointerType() {
			// ERROR_GUARD after flipcoin before call is implicit via HasError checks in callee
			if hasErrCG(cg) {
				return nil
			}
			return MakeRandomBinaryPtrComparison(r, opts, probs, vs, tables, cg, env)
		}
	}
	// FunctionInvocation.cpp:179–183 — do { pick } while (type->is_float() && !works)
	// FunctionInvocation.cpp:185 — assert(type); nil type allowed only for non-float paths (library)
	// Prefer caller probs; fall back to session bag (not ambient-only Process*).
	binProbs := probs
	if binProbs == nil {
		binProbs = sessProbs(sessFromCG(cg))
	}
	op := PickBinaryOpProbsSess(sessFromCG(cg), r, opts, binProbs)
	if typ != nil {
		isF := typ.IsFloatSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-continue pick past IsFloat residual
		if hasErrCG(cg) {
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
				op = PickBinaryOpProbsSess(sessFromCG(cg), r, opts, binProbs)
			}
			if !validB {
				return nil
			}
		}
	}
	// PickBinaryOp MAX / empty token — sticky no invent infix shell without live op
	opStr := op.BinaryOpCSess(sessFromCG(cg))
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:188–207 — always SafeOpFlags::make_random_binary; operands use get_lhs/rhs_type
	var flags *SafeOpFlags
	lhsTy, rhsTy := typ, typ
	// C++ always builds flags; CreateFunctionInvocationBinary only allocates tmps for safe_ops
	flags = MakeRandomBinaryKindSess(sessFromCG(cg), r, opts, probs, typ, typ, typ, SafeOpBinary, op)
	if flags == nil {
		// SafeOpFlags DEPTH_GUARD / ERROR_GUARD
		return nil
	}
	// FunctionInvocation.cpp:219–221 — assert(lhs_type && rhs_type) sticky
	lhsTy = flags.LHSTypeSess(sessFromCG(cg))
	rhsTy = flags.RHSTypeSess(sessFromCG(cg))
	if lhsTy == nil || rhsTy == nil {
		noteErrCG(cg, ErrGeneric)
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
	// CGContext.cpp:74–82 — CGContext(cgc, eff_context, accum): curr_rhs(nullptr), effect_stm().
	// Soft invent kept outer CurrRHS (e.g. assign Lhs binary) into operand subcontexts so
	// nested Lhs::visit_facts ran overlap checks against the wrong RHS (C++ always nulls).
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
	lhsCG.effectContext = cg.EffectContext().detachMaps()
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = EmptyEffect()
	lhsCG.CurrRHS = nil
	// FunctionInvocation.cpp:216 — Expression::make_random(lhs_cg, lhs_type) — no_func=false
	left := MakeRandomExpression(r, opts, tables, vs, &lhsCG, lhsTy, nil, false, false, MaxTermTypes, lhsCG.ExprDepth)
	// FunctionInvocation.cpp:217 — ERROR_GUARD_AND_DEL1(nullptr, fi)
	if left == nil || hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:221 — merge_param_context(lhs) (effects + expr_depth)
	cg.MergeParamContext(lhsCG, true)
	// incomplete effect after lhs merge fails closed sticky (no invent RHS / soft re-pick)
	if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
		}
		return nil
	}

	// FunctionInvocation.cpp:222 — snapshot facts before RHS (ordered merge)
	// incomplete GlobalFacts fail closed sticky (no invent cleaned snapshot)
	// Full FactVec: ePointTo + eUnionWrite (C++ facts_copy = global_facts).
	var factsCopy []*FactPointTo
	var unionCopy []*FactUnion
	if cg.FM != nil {
		if !FactsComplete(cg.FM.GlobalFacts) || !UnionFactsComplete(cg.FM.UnionFacts) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		factsCopy = CloneFactSliceSess(sessFromCG(cg), cg.FM.GlobalFacts)
		// residual ERROR sticky — no invent soft-binary past CloneFactSlice residual
		if hasErrCG(cg) {
			return nil
		}
		unionCopy = CloneUnionFactSliceSess(sessFromCG(cg), cg.FM.UnionFacts)
		if hasErrCG(cg) || !UnionFactsComplete(unionCopy) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return nil
		}
	}

	// FunctionInvocation.cpp:222–255 — ordered RHS on original cg; else combined effect + shifts.
	var right *Expression
	if IsOrderedBinary(op) {
		// FunctionInvocation.cpp:224–226 — RHS under original cg_context
		right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, false, false, MaxTermTypes, cg.ExprDepth)
	} else {
		// FunctionInvocation.cpp:228–234 / 255 — combined effect_context + separate accum
		// Incomplete lhs accum fails closed sticky (no invent RHS under incomplete ambient)
		rhsAccum := EmptyEffect()
		rhsCG := cg.CloneSubcontext()
		rhsCtx := cg.EffectContext().AddEffectOptsSess(sessFromCG(cg), lhsAccum, true)
		if !EffectComplete(rhsCtx) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// CGContext.cpp:74–82 — curr_rhs(nullptr) on (cgc, eff_context, accum)
		rhsCG.effectContext = rhsCtx
		rhsCG.EffectAccum = &rhsAccum
		rhsCG.EffectStm = EmptyEffect()
		rhsCG.CurrRHS = nil
		if op == BinLShift || op == BinRShift {
			// FunctionInvocation.cpp:236–244 — rnd_flipcoin(ShiftByNonConstantProb())
			// C++ Probabilities singleton; nil session → 0% (no invent hard-coded 50)
			shiftNonConst := 0
			if probs != nil {
				shiftNonConst = probs.SingleSess(sessFromCG(cg), PShiftByNonConstantProb)
			} else if p := sessProbs(sessFromCG(cg)); p != nil {
				shiftNonConst = p.SingleSess(sessFromCG(cg), PShiftByNonConstantProb)
			}
			// not_constant = flip; constant path when !not_constant
			if !r.RndFlipcoinSess(sessFromCG(cg), uint32(shiftNonConst)) {
				// FunctionInvocation.cpp:241 — make_random_upto(lhs_type->SizeInBytes() * 8)
				// Type always live after flags assert; SizeInBytes 0 is incomplete
				// (no invent default 32-bit width)
				if lhsTy != nil {
					sb := lhsTy.SizeInBytesSess(sessFromCG(cg))
					// residual ERROR sticky — no invent soft-shift const past SizeInBytes residual
					if hasErrCG(cg) {
						return nil
					}
					if sb > 0 {
						bits := uint32(sb * 8)
						// Constant::make_random_upto; ERROR_GUARD — no invent shell with nil Con
						if c := MakeRandomUptoSess(sessFromCG(cg), bits, r); c != nil && !hasErrCG(cg) {
							// FunctionInvocation.cpp:241–243 — Constant::make_random_upto as RHS.
							// Not Expression::make_random — C++ does NOT bump expr_depth here
							// (depth++ only in Expression.cpp:213–218 after make_random).
							// Extra bump made Go hit max_expr_depth one level early (seed-2 e9188:
							// UP U120 Function vs Go depth-gate filtered Function → Constant).
							right = &Expression{Term: TermConstant, Con: c}
						}
					}
				}
			} else {
				// FunctionInvocation.cpp:243–244 — make_random(..., no_func=false, no_const=true)
				// Avoid negative / oversized shift amounts via constant-filter (not make_random_upto).
				right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, rhsTy, nil, false, true, MaxTermTypes, rhsCG.ExprDepth)
			}
		} else {
			// FunctionInvocation.cpp:247 — Expression::make_random(rhs_cg_context, rhs_type)
			right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, rhsTy, nil, false, false, MaxTermTypes, rhsCG.ExprDepth)
			// FunctionInvocation.cpp:246–253 — div/mod zero-guard BEFORE merge (C++ order)
			// rhs->equals(0) || rhs->is_0_or_1() (all comparison Funcalls are is_0_or_1).
			// Then rnd_upto(MAX_BINARY_OP, filter) rejecting mod/div/shifts.
			if right != nil && !hasErrCG(cg) && (op == BinMod || op == BinDiv) {
				eq0 := right.EqualsIntSess(sessFromCG(cg), 0)
				if hasErrCG(cg) {
					return nil
				}
				is01 := right.Is0Or1Sess(sessFromCG(cg))
				if hasErrCG(cg) {
					return nil
				}
				if eq0 || is01 {
					lhsF, rhsF := false, false
					if lhsTy != nil {
						lhsF = lhsTy.IsFloatSess(sessFromCG(cg))
						if hasErrCG(cg) {
							return nil
						}
					}
					if rhsTy != nil {
						rhsF = rhsTy.IsFloatSess(sessFromCG(cg))
						if hasErrCG(cg) {
							return nil
						}
					}
					if !lhsF && !rhsF {
						f := NewVectorFilterItemsSess(sessFromCG(cg), []int{
							int(BinMod), int(BinDiv), int(BinLShift), int(BinRShift),
						}, FilterModeOut)
						op = BinaryOp(r.RndUptoFilterSess(sessFromCG(cg), uint32(MaxBinaryOp), f))
						if hasErrCG(cg) {
							return nil
						}
						opStr = op.BinaryOpCSess(sessFromCG(cg))
					}
				}
			}
		}
		// FunctionInvocation.cpp:255 — merge_param_context(rhs) (incl. shift constant path)
		cg.MergeParamContext(rhsCG, true)
		if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return nil
		}
	}
	// FunctionInvocation.cpp:257 — ERROR_GUARD_AND_DEL2
	if right == nil || hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:266–273 — CompatibleChecker hard-fail (nullptr)
	if CompatibleCheckExprsSess(sessFromCG(cg), opts, left, right) {
		// residual ERROR sticky — no invent soft-binary past CompatibleCheck residual true
		if hasErrCG(cg) {
			return nil
		}
		noteErrCG(cg, ErrCompatibleCheck)
		return nil
	}
	// residual ERROR sticky — no invent soft-binary past CompatibleCheck residual false
	if hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:275–279 — ordered ops merge facts (short-circuit RHS may skip)
	// full FactVec: makeup_new_var_facts then merge_facts(global, facts_copy).
	// C++ always runs this on the post-LHS snapshot vector, including empty
	// (n=0). Soft invent: `factsCopy != nil` skipped the whole block when
	// GlobalFacts was nil (CloneFactSliceSess(sessFromCG(cg), nil)==nil; NewFactMgr zero value),
	// so the first-program && never makeup-joined empty post-LHS with post-RHS
	// live last=fN → BOTTOM (seed 199: UP nCopy=0 then JOIN 0⊕3, Go kept f3).
	if IsOrderedBinary(op) && cg.FM != nil {
		if !MakeupNewVarFactsSess(sessFromCG(cg), &factsCopy, cg.FM.GlobalFacts) ||
			!makeupNewUnionFactsSess(sessFromCG(cg), &unionCopy, cg.FM.UnionFacts) ||
			!FactsComplete(factsCopy) || !FactsComplete(cg.FM.GlobalFacts) ||
			!UnionFactsComplete(unionCopy) || !UnionFactsComplete(cg.FM.UnionFacts) {
			// incomplete makeup/merge base — fail closed sticky, no invent bare binary
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		_ = MergeFactsSess(sessFromCG(cg), &cg.FM.GlobalFacts, factsCopy)
		// residual ERROR sticky — no invent soft-binary past MergeFacts residual
		if hasErrCG(cg) {
			return nil
		}
		if !FactsComplete(cg.FM.GlobalFacts) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		// eUnionWrite half of merge_facts(global_facts, facts_copy)
		for _, f := range unionCopy {
			if f == nil {
				noteErrCG(cg, ErrGeneric)
				return nil
			}
			cg.FM.UnionFacts = MergeUnionFactSess(sessFromCG(cg), cg.FM.UnionFacts, f)
			if hasErrCG(cg) || !UnionFactsComplete(cg.FM.UnionFacts) {
				if !hasErrCG(cg) {
					noteErrCG(cg, ErrGeneric)
				}
				return nil
			}
		}
	}
	inv := &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}, Safe: flags, Tmp1: tmp1, Tmp2: tmp2}
	inv.setOutOptsSess(sessFromCG(cg), opts)
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
		noteErrCG(cg, ErrGeneric)
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
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:295–296 — rnd_flipcoin(50) ? eCmpEq : eCmpNe
	// (true → ==, false → !=). Do not invert polarity.
	op := BinCmpNe
	if r.RndFlipcoinSess(sessFromCG(cg), 50) {
		op = BinCmpEq
	}
	opStr := op.BinaryOpCSess(sessFromCG(cg))
	// FunctionInvocation.cpp:297–299 — SafeOpFlags::make_random_binary(get_int_type(),
	// nullptr, nullptr, sOpBinary, op) BEFORE choose_random_pointer_type.
	// Output for ptr_cmp still uses standard ==/!= (not safe_* wrappers), but the
	// RNG draws for signedness + size still run (seed-2 e129 was F50 from flags).
	flags := MakeRandomBinaryKindSess(sessFromCG(cg), r, opts, probs, GetIntTypeSess(sessFromCG(cg)), nil, nil, SafeOpBinary, op)
	// ERROR_GUARD after make_random_binary; no soft invent nil-flags ptr comparison
	if flags == nil || hasErrCG(cg) {
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
	// CGContext.cpp:74–82 — curr_rhs(nullptr) on param/binary-style subcontext.
	lhsAccum := EmptyEffect()
	lhsCG := cg.CloneSubcontext()
	lhsCG.effectContext = cg.EffectContext().detachMaps()
	lhsCG.Flags |= FlagNoDanglingPtr
	lhsCG.EffectAccum = &lhsAccum
	lhsCG.EffectStm = EmptyEffect()
	lhsCG.CurrRHS = nil
	// make_random(lhs_cg, type, 0, true) — no_func true
	left := MakeRandomExpression(r, opts, tables, vs, &lhsCG, ptrTy, nil, true, false, MaxTermTypes, lhsCG.ExprDepth)
	if left == nil || hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:313 — merge_param_context(lhs)
	cg.MergeParamContext(lhsCG, true)
	// incomplete effect after lhs merge fails closed sticky (no invent RHS / soft re-pick)
	if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
		if !hasErrCG(cg) {
			noteErrCG(cg, ErrGeneric)
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
		rhsCtx := cg.EffectContext().AddEffectSess(sessFromCG(cg), lhsAccum)
		// residual ERROR sticky — no invent soft-continue RHS past AddEffect residual
		if hasErrCG(cg) {
			return nil
		}
		if !EffectComplete(rhsCtx) {
			noteErrCG(cg, ErrGeneric)
			return nil
		}
		rhsCG.effectContext = rhsCtx
		rhsCG.EffectAccum = &rhsAccum
		rhsCG.EffectStm = EmptyEffect()
		rhsCG.CurrRHS = nil
		rhsCG.Flags |= FlagNoDanglingPtr
		right = MakeRandomExpression(r, opts, tables, vs, &rhsCG, ptrTy, nil, true, false, tt, rhsCG.ExprDepth)
		// FunctionInvocation.cpp:345
		cg.MergeParamContext(rhsCG, true)
		if hasErrCG(cg) || !EffectComplete(cg.EffectStm) || (cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) {
			if !hasErrCG(cg) {
				noteErrCG(cg, ErrGeneric)
			}
			return nil
		}
	}
	if right == nil || hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:349 — typecast RHS to LHS type if needed (lang_cpp)
	lt := left.GetTypeSess(sessFromCG(cg))
	// residual ERROR sticky — no invent ptr-cmp past GetType residual hole
	if hasErrCG(cg) {
		return nil
	}
	right.CheckAndSetCastOptsSess(sessFromCG(cg), lt, opts)
	// residual ERROR sticky — no invent ptr-cmp past CheckAndSetCast residual hole
	if hasErrCG(cg) {
		return nil
	}
	// FunctionInvocation.cpp:358 — bookkeeping
	RecordPointerComparisonsSess(sessFromCG(cg), left, right)
	// residual ERROR sticky — no invent ptr-cmp past RecordPointerComparisons residual hole
	if hasErrCG(cg) {
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
	inv.setOutOptsSess(sessFromCG(cg), opts)
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
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent binary shell / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:565 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationBinary, nullptr)
	if DepthGuardByTypeSess(sessFromCG(&cg), opts, DtFunctionInvocationBinary) == BadDepth {
		return nil
	}
	// invalid / MAX op — sticky no invent empty Binary token shell
	opStr := op.BinaryOpCSess(sessFromCG(&cg))
	if int(op) < 0 || int(op) >= MaxBinaryOp || opStr == "" {
		noteErrCG(&cg, ErrGeneric)
		return nil
	}
	lt, rt := lhs.GetTypeSess(sessFromCG(&cg)), rhs.GetTypeSess(sessFromCG(&cg))
	// residual ERROR sticky — no invent binary shell past GetType residual hole
	if hasErrCG(&cg) {
		return nil
	}
	// FunctionInvocation.cpp:566–568 — rv_type nullptr; op1/op2 from operands
	flags := MakeRandomBinaryKindSess(sessFromCG(&cg), r, opts, probs, nil, lt, rt, SafeOpBinary, op)
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
	inv.setOutOptsSess(sessFromCG(&cg), opts)
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
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// incomplete ambient fails closed sticky (no invent unary / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:143 — DEPTH_GUARD_BY_TYPE_RETURN(dtFunctionInvocationRandomUnary, nullptr)
	if DepthGuardByTypeSess(sessFromCG(cg), opts, DtFunctionInvocationRandomUnary) == BadDepth {
		return nil
	}
	// FunctionInvocation.cpp:144 — assert(type) sticky
	if typ == nil {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:146–149 — do { pick } while (float && !works); no soft invent invalid
	var uop UnaryOp
	validU := false
	// Unary make_random has no Probabilities arg; use session bag (cg.Sess under Generate).
	unProbs := sessProbs(sessFromCG(cg))
	// C++ unbounded do-while; cap high (no soft invent invalid float op)
	for tries := 0; tries < 256; tries++ {
		uop = PickUnaryOpProbsSess(sessFromCG(cg), r, opts, unProbs)
		isF := typ.IsFloatSess(sessFromCG(cg))
		// residual ERROR sticky — no invent soft-continue unary pick past IsFloat residual
		if hasErrCG(cg) {
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
	op := uop.UnaryOpCSess(sessFromCG(cg))
	if int(uop) < 0 || int(uop) >= MaxUnaryOp || op == "" {
		noteErrCG(cg, ErrGeneric)
		return nil
	}
	// FunctionInvocation.cpp:151–155 — always make_random_unary then operand type from flags
	// Probabilities singleton always live in C++; no invent NewProbabilities(opts)
	var probs *Probabilities
	if vs != nil {
		probs = vs.Probs
	}
	flags := MakeRandomUnarySess(sessFromCG(cg), r, opts, probs, typ, nil, uop)
	// FunctionInvocation.cpp:152–154 — ERROR_GUARD; type = flags->get_lhs_type(); assert(type) sticky
	if flags == nil {
		return nil
	}
	argTy := flags.LHSTypeSess(sessFromCG(cg))
	if argTy == nil {
		noteErrCG(cg, ErrGeneric)
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
	if arg == nil || hasErrCG(cg) {
		return nil
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}, Safe: flags, Tmp1: tmp1}
	inv.setOutOptsSess(sessFromCG(cg), opts)
	return inv
}

// createBinarySafeTmps mirrors FunctionInvocationBinary::CreateFunctionInvocationBinary
// temp allocation. FunctionInvocationBinary.cpp:59–75 — always when flags && safe_ops;
// no soft invent skip on !MathNoTmp or float size.
func createBinarySafeTmps(cg CGContext, vs *VariableSelector, flags *SafeOpFlags, op BinaryOp) (tmp1, tmp2 string) {
	if flags == nil || !SafeOpsBinary(op.BinaryOpCSess(sessFromCG(&cg))) {
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
	ty1 := flags.LHSTypeSess(sessFromCG(&cg))
	if ty1 == nil {
		noteErrCG(&cg, ErrGeneric)
		return "", ""
	}
	if !ty1.IsSimpleSess(sessFromCG(&cg)) {
		// residual ERROR sticky — no invent soft-tmp past IsSimple residual
		if hasErrCG(&cg) {
			return "", ""
		}
		noteErrCG(&cg, ErrGeneric)
		return "", ""
	}
	// residual ERROR sticky — no invent soft-tmp past IsSimple residual true
	if hasErrCG(&cg) {
		return "", ""
	}
	st := ty1.SimpleSess(sessFromCG(&cg))
	tmp1 = blk.CreateNewTmpVarSess(firstSess(sessFromVS(vs), sessFromCG(&cg)), st)
	// residual ERROR sticky — no invent soft-tmp past CreateNewTmpVar residual
	if hasErrCG(&cg) {
		return "", ""
	}
	st2 := st
	// FunctionInvocationBinary.cpp:64–78 — flags_to_type(op2) always live for shifts;
	// sticky no invent type1 stand-in for type2 past missing/non-simple RHS type shell
	if op == BinLShift || op == BinRShift {
		ty := flags.RHSTypeSess(sessFromCG(&cg))
		if ty == nil {
			noteErrCG(&cg, ErrGeneric)
			return "", ""
		}
		if !ty.IsSimpleSess(sessFromCG(&cg)) {
			if hasErrCG(&cg) {
				return "", ""
			}
			noteErrCG(&cg, ErrGeneric)
			return "", ""
		}
		if hasErrCG(&cg) {
			return "", ""
		}
		st2 = ty.SimpleSess(sessFromCG(&cg))
	}
	tmp2 = blk.CreateNewTmpVarSess(firstSess(sessFromVS(vs), sessFromCG(&cg)), st2)
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
	ty := flags.LHSTypeSess(sessFromCG(&cg))
	if ty == nil {
		noteErrCG(&cg, ErrGeneric)
		return ""
	}
	if !ty.IsSimpleSess(sessFromCG(&cg)) {
		// residual ERROR sticky — no invent soft-tmp past IsSimple residual
		if hasErrCG(&cg) {
			return ""
		}
		noteErrCG(&cg, ErrGeneric)
		return ""
	}
	// residual ERROR sticky — no invent soft-tmp past IsSimple residual true
	if hasErrCG(&cg) {
		return ""
	}
	tmp := blk.CreateNewTmpVarSess(firstSess(sessFromVS(vs), sessFromCG(&cg)), ty.SimpleSess(sessFromCG(&cg)))
	// residual ERROR sticky — no invent soft-tmp past CreateNewTmpVar residual
	if hasErrCG(&cg) {
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
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	// incomplete ambient fails closed sticky (no invent choose/build / soft re-pick past holes)
	if !EffectComplete(cg.EffectContext()) ||
		(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
		!EffectComplete(cg.EffectStm) {
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
		noteErrCG(cg, ErrGeneric)
		return &Invocation{Failed: true}
	}
	// Match type for choose_func: nil means any return type (C++ type=0).
	matchType := typ
	// FunctionInvocation.cpp:71–73 path — non-simple/void force user path (type known)
	if typ != nil {
		pt := typ.PtrTypeSess(cgSess(cg))
		// residual ERROR sticky — no invent soft-std path past PtrType residual
		if hasErrCG(cg) {
			return &Invocation{Failed: true}
		}
		if pt != nil {
			stdFunc = false
		} else {
			simple := typ.IsSimpleSess(cgSess(cg))
			// residual ERROR sticky — no invent soft-std path past IsSimple residual
			if hasErrCG(cg) {
				return &Invocation{Failed: true}
			}
			if simple && typ.SimpleSess(cgSess(cg)) == EVoid {
				stdFunc = false
			}
		}
	}

	var fi *Invocation
	if !stdFunc {
		var callee *Function
		// FunctionInvocation.cpp:87 — pure_rnd_flipcoin(50) (random mode == rnd)
		if r.RndFlipcoinSess(sessFromCG(cg), 50) && list != nil {
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
					if !hasErrCG(cg) {
						noteErrCG(cg, ErrGeneric)
					}
					return &Invocation{Failed: true}
				}
			}
			// Statement probability table is session singleton; sticky no invent second table
			stmtTab := sessStmtTab(sessFromCG(cg))
			if stmtTab == nil {
				noteErrCG(cg, ErrGeneric)
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
		// FunctionInvocation.cpp:111–118 — always rnd_flipcoin(StdUnaryFuncProb());
		// then unary if true else binary. C++ draws even when type is null
		// (unary asserts(type) after a true draw). Skipping the draw when
		// typ==nil desyncs RNG vs upstream (unfair soft prefer-binary without F5).
		// Fair: always Flipcoin; unary only when flag && typ live.
		if probs == nil || r == nil {
			noteErrCG(cg, ErrGeneric)
			return &Invocation{Failed: true}
		}
		stdUnary := r.RndFlipcoinSess(sessFromCG(cg), uint32(probs.SingleSess(sessFromCG(cg), PStdUnaryFuncProb)))
		// FunctionInvocation.cpp ERROR_GUARD after flipcoin
		if hasErrCG(cg) {
			return &Invocation{Failed: true}
		}
		if stdUnary {
			// FunctionInvocation.cpp:143 assert(type); NDEBUG Release continues into
			// type->… UB — fail closed sticky (no invent binary after a true unary draw).
			if typ == nil {
				noteErrCG(cg, ErrGeneric)
				return &Invocation{Failed: true}
			}
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
