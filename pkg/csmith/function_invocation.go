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
}

// Output C for the invocation.
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
		if fi.IsUnary && len(fi.Args) >= 1 {
			return fmt.Sprintf("(%s(%s))", fi.Unary, fi.Args[0].Output())
		}
		if !fi.IsUnary && len(fi.Args) >= 2 {
			return fmt.Sprintf("(%s %s %s)", fi.Args[0].Output(), fi.Binary, fi.Args[1].Output())
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
func ChooseFunc(r *Rng, funcs []*Function, ret *Type) *Function {
	var ok []*Function
	for _, f := range funcs {
		if f == nil || f.IsBuiltin {
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

// MakeRandomBinaryInvocation mirrors FunctionInvocation::make_random_binary (arithmetic subset).
// Uses simple ops without SafeOpFlags wrappers yet.
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
	// skip pointer comparison path for now
	ops := []string{"+", "-", "*", "/"}
	if !opts.Muls {
		ops = []string{"+", "-", "/"}
	}
	if !opts.Divs {
		ops = []string{"+", "-"}
		if opts.Muls {
			ops = []string{"+", "-", "*"}
		}
	}
	op := ops[r.RndUpto(uint32(len(ops)))]
	// Operands: no nested Function (depth + leaf bias) — avoids exponential recursion.
	d := cg.ExprDepth + 1
	left := MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, MaxTermTypes, d)
	if left == nil {
		left = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, d)
	}
	right := MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, MaxTermTypes, d)
	if right == nil {
		right = MakeRandomExpression(r, opts, tables, vs, cg, typ, nil, true, false, TermConstant, d)
	}
	_ = probs
	return &Invocation{IsStd: true, Binary: op, Args: []*Expression{left, right}}
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
	return &Invocation{IsStd: true, IsUnary: true, Unary: op, Args: []*Expression{arg}}
}

// MakeRandomInvocation mirrors FunctionInvocation::make_random.
// FunctionInvocation.cpp:78–120.
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
	if typ == nil {
		typ = GetIntType()
	}
	// non-simple / void → force user path (std_func false)
	if typ.PtrType() != nil || (typ.IsSimple() && typ.Simple() == EVoid) {
		stdFunc = false
	}

	var fi *Invocation
	if !stdFunc {
		var callee *Function
		if r.RndFlipcoin(50) && list != nil {
			callee = ChooseFunc(r, list.Funcs, typ)
		}
		if callee != nil {
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
		} else if list != nil && !ReachMaxFunctions(list, opts) {
			// build_invocation_and_function → make_random_signature only (body later)
			callee = MakeRandomSignature(r, opts, probs, vs, &vs.Sym, cg, typ, nil, list)
			fi = BuildUserInvocation(r, opts, probs, vs, tables, cg, list, callee)
		} else {
			return &Invocation{Failed: true}
		}
	}
	if fi == nil {
		// std unary/binary
		if r.RndFlipcoin(uint32(probs.Single(PStdUnaryFuncProb))) {
			fi = MakeRandomUnaryInvocation(r, opts, vs, tables, cg, typ)
		} else {
			fi = MakeRandomBinaryInvocation(r, opts, probs, vs, tables, cg, typ)
		}
	}
	return fi
}
