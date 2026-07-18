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
	IsUnary  bool
	Unary    string // "+", "-", "~", "!"
	Binary   string // "+", "-", etc.
	// Args are parameter expressions (operands).
	Args []*Expression
	// Safe is set for std binary/unary under avoid_signed_overflow.
	Safe *SafeOpFlags
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
				return fmt.Sprintf("(%s(%s))", fi.Safe.UnaryMinusFuncName(), a0)
			}
			return fmt.Sprintf("(%s(%s))", fi.Unary, a0)
		}
		if !fi.IsUnary && len(fi.Args) >= 2 {
			if fi.Safe != nil && SafeOpsBinary(fi.Binary) {
				if fname := fi.Safe.BinaryFuncName(fi.Binary); fname != "" {
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
// Function.cpp:279+ simplified: match return type with eConvert.
func ChooseFunc(r *Rng, funcs []*Function, ret *Type, exclude *Function) *Function {
	var ok []*Function
	for _, f := range funcs {
		if f == nil || f.IsBuiltin || f == exclude {
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

// BuildUserInvocation builds args for callee via Expression::make_random_param-ish (variable/const).
// FunctionInvocationUser::build_invocation simplified — one expr per param.
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
		if p != nil && p.Type != nil {
			ty = p.Type
		}
		// make_random_param: no nested user-func explosion — variable preferred, no const.
		arg := MakeRandomExpression(r, opts, tables, vs, cg, ty, nil, true, true, MaxTermTypes, cg.ExprDepth+1)
		if arg == nil {
			arg = MakeRandomExpression(r, opts, tables, vs, cg, ty, nil, true, false, TermVariable, cg.ExprDepth+1)
		}
		fi.Args = append(fi.Args, arg)
	}
	_ = probs
	_ = list
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
	// skip make_random_binary_ptr_comparison (10% when pointers exist) for now
	op := PickBinaryOp(r, opts)
	// float filter if we ever pass float types
	if typ.IsSimple() && typ.Simple() == EFloat && !BinaryOpWorksForFloat(op) {
		op = PickBinaryOp(r, opts)
	}
	opStr := op.BinaryOpC()
	// Operands: no nested Function (depth + leaf bias) — avoids exponential recursion.
	d := cg.ExprDepth + 1
	left := MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, d)
	}
	var right *Expression
	if op == BinLShift || op == BinRShift {
		// prefer constant shift amount (FunctionInvocation.cpp:236–244 simplified)
		// ShiftByNonConstantProb default ~50 → flipcoin for non-constant
		if !r.RndFlipcoin(50) {
			// Constant::make_random_upto(SizeInBytes*8); int → 32 bits
			right = &Expression{Term: TermConstant, Con: MakeRandomUpto(32, r)}
		}
	}
	if right == nil {
		// ordered ops (&& ||) could use original effect context; we omit FactMgr merge
		right = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, MaxTermTypes, d)
		if right == nil {
			right = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, d)
		}
	}
	// avoid div/mod by zero-ish constant: re-pick op excluding div/mod/shift
	if (op == BinMod || op == BinDiv) && right != nil && right.Term == TermConstant && right.Con != nil {
		if right.Con.Value == "0" || right.Con.Value == "1" {
			// VectorFilter out mod/div/shifts — simplified re-pick arithmetic
			op = BinAdd
			opStr = op.BinaryOpC()
		}
	}
	inv := &Invocation{IsStd: true, Binary: opStr, Args: []*Expression{left, right}}
	// CGOptions::avoid_signed_overflow → SafeMath for safe_ops subset
	if opts.SafeMath && SafeOpsBinary(opStr) {
		inv.Safe = MakeRandomBinary(r, opts, probs, typ)
	}
	_ = IsOrderedBinary // cite path; effect merge deferred
	return inv
}

// MakeRandomUnaryInvocation mirrors make_random_unary subset: + - ~ !
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
	ops := []string{"-", "~", "!"}
	if opts.UnaryPlusOperator {
		ops = append([]string{"+"}, ops...)
	}
	op := ops[r.RndUpto(uint32(len(ops)))]
	d := cg.ExprDepth + 1
	arg := MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, MaxTermTypes, d)
	if arg == nil {
		arg = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, d)
	}
	inv := &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}}
	if opts.SafeMath && op == "-" {
		inv.Safe = MakeRandomBinary(r, opts, NewProbabilities(opts), typ)
		// unary only needs op1; re-roll size/sign via same helper is OK
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
				sigType = RandomReturnType(r, probs)
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
