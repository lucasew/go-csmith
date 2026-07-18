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
}

// Output C for the invocation.
// FunctionInvocationBinary::Output — safe_*_func when Safe set (avoid_signed_overflow).
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
			if fi.Unary == "-" && fi.Safe != nil {
				// FunctionInvocationUnary::Output — math_notmp inserts tmp first
				if fi.MathNoTmp && fi.Tmp1 != "" {
					return fmt.Sprintf("(%s(%s, %s))", fi.Safe.UnaryMinusFuncName(), fi.Tmp1, a0)
				}
				return fmt.Sprintf("(%s(%s))", fi.Safe.UnaryMinusFuncName(), a0)
			}
			return fmt.Sprintf("(%s(%s))", fi.Unary, a0)
		}
		if !fi.IsUnary && len(fi.Args) >= 2 {
			if fi.Safe != nil && SafeOpsBinary(fi.Binary) {
				if fname := fi.Safe.BinaryFuncName(fi.Binary); fname != "" {
					// FunctionInvocationBinary::Output — math_notmp: fname(tmp1, a0, tmp2, a1)
					if fi.MathNoTmp && fi.Tmp1 != "" && fi.Tmp2 != "" {
						return fmt.Sprintf("(%s(%s, %s, %s, %s))", fname, fi.Tmp1, a0, fi.Tmp2, a1)
					}
					return fmt.Sprintf("(%s(%s, %s))", fname, a0, a1)
				}
			}
			return fmt.Sprintf("(%s %s %s)", a0, fi.Binary, a1)
		}
	}
	return "/*invoke*/"
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
// Function.cpp:279+ — skip unbuilt (is_effect_known false) and builtins.
func ChooseFunc(r *Rng, funcs []*Function, ret *Type, exclude *Function) *Function {
	var ok []*Function
	for _, f := range funcs {
		if f == nil || f.IsBuiltin || f == exclude || !f.IsEffectKnown() {
			// is_effect_known() == false for Unbuilt/Building
			continue
		}
		if ret == nil || f.ReturnType == nil || ret.Match(f.ReturnType, MatchConvert) {
			ok = append(ok, f)
		}
	}
	n := len(ok)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return ok[0]
	}
	return ok[r.RndUpto(uint32(n))]
}

// ExpressionFunctionProbability mirrors ExpressionFuncall.cpp:57–62.
// true → prefer std (unary/binary); false → user function.
func ExpressionFunctionProbability(r *Rng, list *FunctionList, opts Options) bool {
	if ReachMaxFunctions(list, opts) && !opts.Builtins {
		return true
	}
	return r.RndFlipcoin(80)
}

// BuildUserInvocation mirrors FunctionInvocationUser::build_invocation arg loop.
// FunctionInvocationUser.cpp:188 — Expression::make_random_param(param type, param qfer).
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
		// make_random_param (param table: no constant args; as_param=true)
		arg := MakeRandomParam(r, opts, tables, vs, cg, ty, qfer, cg.ExprDepth+1, list)
		if arg == nil {
			arg = makeExpressionVariableFlags(r, vs, cg, ty, qfer, true, false)
		}
		fi.Args = append(fi.Args, arg)
	}
	_ = probs
	return fi
}

// MakeRandomBinaryInvocation mirrors FunctionInvocation::make_random_binary.
// FunctionInvocation.cpp:171–288 — full eBinaryOps via BINARY_OPS_PROB_FILTER;
// pointer-comparison branch and FactMgr omitted.
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
	// FunctionInvocation.cpp:188–207 — SafeOpFlags first; operands use get_lhs/rhs_type
	var flags *SafeOpFlags
	lhsTy, rhsTy := typ, typ
	if opts.SafeMath && SafeOpsBinary(opStr) {
		flags = MakeRandomBinary(r, opts, probs, typ)
		if flags != nil {
			lhsTy = flags.LHSType()
			rhsTy = flags.RHSType()
		}
	}
	// Operands: no nested Function (depth + leaf bias) — avoids exponential recursion.
	// FunctionInvocation.cpp:208–261 — ordered (&&/||) RHS under original effect context;
	// unordered RHS under original + LHS accum.
	d := cg.ExprDepth + 1
	preLeft := EmptyEffect()
	if cg.EffectAccum != nil {
		preLeft = *cg.EffectAccum
	}
	left := MakeRandomExpression(r, opts, tables, vs, cg, lhsTy, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, cg, lhsTy, nil, true, false, TermConstant, d)
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
		}
	}
	if right == nil {
		if IsOrderedBinary(op) && cg.EffectAccum != nil {
			// RHS under pre-left context only; merge back (FunctionInvocation.cpp:222–226)
			postLeft := *cg.EffectAccum
			*cg.EffectAccum = preLeft
			right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, MaxTermTypes, d)
			if right == nil {
				right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, TermConstant, d)
			}
			*cg.EffectAccum = MergeEffects(postLeft, *cg.EffectAccum)
		} else {
			// unordered: RHS sees LHS effects via current EffectAccum
			right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, MaxTermTypes, d)
			if right == nil {
				right = MakeRandomExpression(r, opts, tables, vs, cg, rhsTy, nil, true, false, TermConstant, d)
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
	// CreateFunctionInvocationBinary tmp vars when math_notmp (FunctionInvocationBinary.cpp:59–75)
	if flags != nil && opts.MathNoTmp && SafeOpsBinary(opStr) {
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
// FunctionInvocation.cpp:294–360 — == or != on random pointer type operands.
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
	// eCmpEq or eCmpNe
	op := BinCmpEq
	if r.RndFlipcoin(50) {
		op = BinCmpNe
	}
	opStr := op.BinaryOpC()
	// Type::choose_random_pointer_type
	ptrTy := env.ChooseRandomPointerType(r)
	if ptrTy == nil {
		return nil
	}
	d := cg.ExprDepth + 1
	// FunctionInvocation.cpp:311 — NO_DANGLING_PTR on operand contexts
	pcg := cg.WithFlags(FlagNoDanglingPtr)
	// no_func on both sides (true); const ok on LHS
	left := MakeRandomExpression(r, opts, tables, vs, pcg, ptrTy, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, pcg, ptrTy, nil, true, false, TermVariable, d)
	}
	tt := MaxTermTypes
	if left != nil && left.Term == TermConstant {
		tt = TermVariable
	}
	right := MakeRandomExpression(r, opts, tables, vs, pcg, ptrTy, nil, true, false, tt, d)
	if right == nil {
		right = MakeRandomExpression(r, opts, tables, vs, pcg, ptrTy, nil, true, false, TermVariable, d)
	}
	_ = probs
	// pointer comparisons do not use safe math wrappers
	return &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}}
}

// MakeRandomUnaryInvocation mirrors make_random_unary.
// FunctionInvocation.cpp:141–165 — eUnaryOps via UNARY_OPS_PROB_FILTER.
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
	uop := PickUnaryOp(r, opts)
	op := uop.UnaryOpC()
	// SafeOpFlags::make_random_unary then operand of get_lhs_type (FunctionInvocation.cpp:141–165)
	argTy := typ
	var flags *SafeOpFlags
	if opts.SafeMath && op == "-" {
		flags = MakeRandomBinary(r, opts, NewProbabilities(opts), typ)
		if flags != nil {
			argTy = flags.LHSType()
		}
	}
	d := cg.ExprDepth + 1
	arg := MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, true, false, MaxTermTypes, d)
	if arg == nil {
		arg = MakeRandomExpression(r, opts, tables, vs, cg, argTy, nil, true, false, TermConstant, d)
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}, Safe: flags}
	if flags != nil && opts.MathNoTmp && op == "-" {
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
			callee = ChooseFunc(r, list.Funcs, matchType, cg.CurrentFunc)
		}
		if callee != nil {
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
		} else if list != nil && !ReachMaxFunctions(list, opts) {
			// build_invocation_and_function → make_random_signature only (body later)
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
			callee = MakeRandomSignature(r, opts, probs, vs, &vs.Sym, cg, sigType, nil, list)
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
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
