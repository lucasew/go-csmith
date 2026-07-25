package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomBinaryFloatPath(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	opts.EnableFloat = true
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	f := MakeRandomBinaryKindSess(testAmbientSession, NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), ft, ft, ft, SafeOpBinary, BinAdd)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Size != SafeFloat {
		t.Fatalf("size %v want SafeFloat", f.Size)
	}
	if !f.Op1Signed || !f.Op2Signed {
		t.Fatal("float always signed")
	}
	if f.SizeTokenSess(testAmbientSession) != "float" {
		t.Fatal(f.SizeTokenSess(testAmbientSession))
	}
}

func TestMakeRandomBinaryAssignKind(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	// assign kind: op2 == op1
	for seed := uint64(1); seed < 20; seed++ {
		f := MakeRandomBinaryKindSess(testAmbientSession, NewRngSess(testAmbientSession, seed), opts, NewProbabilities(opts), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), SafeOpAssign, BinAdd)
		if f == nil {
			t.Fatalf("seed %d: nil", seed)
		}
		if f.Op1Signed != f.Op2Signed {
			t.Fatalf("seed %d: assign op2 should match op1", seed)
		}
	}
}

func TestMakeRandomUnaryFloatPath(t *testing.T) {
	opts := Defaults()
	opts.EnableFloat = true
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	f := MakeRandomUnarySess(testAmbientSession, NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), ft, nil, UnMinus)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Size != SafeFloat {
		t.Fatalf("size %v want SafeFloat", f.Size)
	}
	if !f.Op1Signed || !f.Op2Signed {
		t.Fatal("float unary always signed")
	}
	// SafeOpFlags.cpp:325 — assert no float unary; fail closed empty non-sticky (no invent int32)
	ClearErrorSess(testAmbientSession)
	if name := f.UnaryMinusFuncNameSess(testAmbientSession); name != "" {
		t.Fatalf("float unary must fail closed, got %q", name)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("float UnaryMinusFuncName must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomUnaryIntPath(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.EnableFloat = false
	f := MakeRandomUnarySess(testAmbientSession, NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), GetIntTypeSess(testAmbientSession), nil, UnMinus)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Size == SafeFloat {
		t.Fatal("int unary must not pick SafeFloat")
	}
	name := f.UnaryMinusFuncNameSess(testAmbientSession)
	if !strings.HasPrefix(name, "safe_unary_minus_func_") {
		t.Fatalf("%q", name)
	}
}

func TestUnaryMinusFuncNameNilFailClosed(t *testing.T) {
	// sticky no soft invent default int32 name / SizeToken for nil flags
	ClearErrorSess(testAmbientSession)
	var f *SafeOpFlags
	if f.UnaryMinusFuncNameSess(testAmbientSession) != "" {
		t.Fatal("nil flags must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil UnaryMinusFuncName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f.BinaryFuncNameSess(testAmbientSession, "+") != "" {
		t.Fatal("nil BinaryFuncName must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil BinaryFuncName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f.SizeTokenSess(testAmbientSession) != "" {
		t.Fatal("nil SizeToken must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SizeToken must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f.LHSTypeSess(testAmbientSession) != nil || f.RHSTypeSess(testAmbientSession) != nil {
		t.Fatal("nil LHS/RHS type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil LHSType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCastOpNoInventEmptySizeToken(t *testing.T) {
	// invalid SafeOpSize → empty SizeToken; sticky no invent "(-()x)" / "(()a + ()b)"
	ClearErrorSess(testAmbientSession)
	if unaryCastMinus("", "x") != "" || unaryCastMinus("int32_t", "") != "" {
		t.Fatal("unary cast empty must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unary cast empty must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if binaryCastOp("", "a", "+", "b") != "" || binaryCastOp("int32_t", "a", "", "b") != "" {
		t.Fatal("binary cast empty must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("binary cast empty must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	fi := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:        []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}},
		Safe:        &SafeOpFlags{Size: SafeOpSize(99), Op1Signed: true},
		OutSafeMath: false,
	}
	if out := fi.Output(); out != "" {
		t.Fatal("invalid size unary cast must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid size unary cast must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinaryFuncNameInvalidSizeFailClosed(t *testing.T) {
	// SafeOpFlags.cpp:239 assert invalid size; sticky no invent safe_add_func__s_s
	ClearErrorSess(testAmbientSession)
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeOpSize(99)}
	if got := f.BinaryFuncNameSess(testAmbientSession, "+"); got != "" {
		t.Fatal("invalid size must fail closed BinaryFuncName", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid size BinaryFuncName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if got := f.BinaryFuncNameSess(testAmbientSession, "<<"); got != "" {
		t.Fatal("invalid size shift must fail closed", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid size shift must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinaryFuncNameFloat(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeFloat}
	if got := f.BinaryFuncNameSess(testAmbientSession, "+"); got != "safe_add_func_float_f_f" {
		t.Fatalf("add %q", got)
	}
	if got := f.BinaryFuncNameSess(testAmbientSession, "-"); got != "safe_sub_func_float_f_f" {
		t.Fatalf("sub %q", got)
	}
	if got := f.BinaryFuncNameSess(testAmbientSession, "*"); got != "safe_mul_func_float_f_f" {
		t.Fatalf("mul %q", got)
	}
	if got := f.BinaryFuncNameSess(testAmbientSession, "/"); got != "safe_div_func_float_f_f" {
		t.Fatalf("div %q", got)
	}
	if got := f.BinaryFuncNameSess(testAmbientSession, "%"); got != "" {
		t.Fatalf("mod should be empty for float, got %q", got)
	}
	// float mod empty is non-sticky (no wrapper, not broken IR)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("float mod empty must stay non-sticky")
	}
	// float unary minus name non-sticky empty (cast emit fallthrough)
	if f.UnaryMinusFuncNameSess(testAmbientSession) != "" {
		t.Fatal("float unary safe name must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("float UnaryMinusFuncName must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUnaryMinusOutputFloatUsesStandard(t *testing.T) {
	// FunctionInvocationUnary.cpp:220–223 — float size → standard minus
	arg := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}
	fi := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:        []*Expression{arg},
		Safe:        &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeFloat},
		OutSafeMath: true,
	}
	out := fi.Output()
	if strings.Contains(out, "safe_") {
		t.Fatalf("float unary must not use safe: %s", out)
	}
	if !strings.Contains(out, "-") {
		t.Fatal(out)
	}
}

func TestUnaryMinusOutputSafeAndIdentify(t *testing.T) {
	ClearSafeOpWrapperNamesSess(testAmbientSession)
	defer ClearSafeOpWrapperNamesSess(testAmbientSession)
	arg := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}
	fi := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:                []*Expression{arg},
		Safe:                &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32},
		OutSafeMath:         true,
		OutIdentifyWrappers: true,
	}
	out := fi.Output()
	if !strings.Contains(out, "safe_unary_minus_func_int32_t_s") {
		t.Fatal(out)
	}
	// identify_wrappers appends , id
	if !strings.Contains(out, ", ") {
		t.Fatalf("expected wrapper id arg: %s", out)
	}
}

func TestUnaryMinusOutputWrapperFilter(t *testing.T) {
	ClearSafeOpWrapperNamesSess(testAmbientSession)
	defer ClearSafeOpWrapperNamesSess(testAmbientSession)
	// pre-register so id is known; filter only id 1 — deny if id != 1
	fname := "safe_unary_minus_func_int32_t_s"
	id := SafeOpFlagsToIDSess(testAmbientSession, fname)
	arg := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}
	fi := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:                []*Expression{arg},
		Safe:                &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32},
		OutSafeMath:         true,
		OutSafeMathWrappers: "99999", // deny all real ids
	}
	_ = id
	out := fi.Output()
	if strings.Contains(out, "safe_") {
		t.Fatalf("wrapper denied should cast: %s", out)
	}
	if !strings.Contains(out, "int32_t") {
		t.Fatal(out)
	}
}

// FunctionInvocationUnary.cpp:229–242 — standard unary is "(" + op + arg.Output + ")"
// with no extra parens around the arg (param_value[0]->Output only).
// Seed-2: (~(safe_unary_minus…)) not (~((safe_unary_minus…))).
func TestUnaryStandardOutputNoExtraArgParens(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// nested: ~ applied to a safe unary-minus invocation
	innerArg := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, -10)}
	minus := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:        []*Expression{innerArg},
		Safe:        &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt8},
		OutSafeMath: true,
	}
	innerExpr := &Expression{Term: TermFunction, Invoke: minus}
	bitNot := &Invocation{
		IsStd: true, IsUnary: true, Unary: "~",
		Args: []*Expression{innerExpr},
	}
	out := bitNot.Output()
	// C++ shape: (~(safe_unary_minus_func_int8_t_s((-10L))))
	if !strings.Contains(out, "safe_unary_minus_func_int8_t_s") {
		t.Fatal(out)
	}
	if strings.Contains(out, "~((") {
		t.Fatalf("extra parens after ~: %s", out)
	}
	if !strings.HasPrefix(out, "(~(") || !strings.HasSuffix(out, "))") {
		t.Fatalf("want (~(safe…)), got %s", out)
	}
	// ! on bare constant: (!4294967295UL) not (!(4294967295UL)) — Constant may already paren
	// use a simple non-paren-wrapped path via variable name
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), true, false)
	not := &Invocation{
		IsStd: true, IsUnary: true, Unary: "!",
		Args: []*Expression{{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	nout := not.Output()
	if nout != "(!g_x)" {
		t.Fatalf("eNot want (!g_x) got %q", nout)
	}
	// unary plus
	plus := &Invocation{
		IsStd: true, IsUnary: true, Unary: "+",
		Args: []*Expression{{Term: TermVariable, Var: v, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	if plus.Output() != "(+g_x)" {
		t.Fatalf("ePlus want (+g_x) got %q", plus.Output())
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinaryOutputIdentifyWrappers(t *testing.T) {
	ClearSafeOpWrapperNamesSess(testAmbientSession)
	defer ClearSafeOpWrapperNamesSess(testAmbientSession)
	a0 := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}
	a1 := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 2)}
	fi := &Invocation{
		IsStd: true, Binary: "+",
		Args:                []*Expression{a0, a1},
		Safe:                &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32},
		OutSafeMath:         true,
		OutIdentifyWrappers: true,
	}
	out := fi.Output()
	if !strings.Contains(out, "safe_add_func_int32_t_s_s") {
		t.Fatal(out)
	}
	if !strings.Contains(out, ", ") {
		t.Fatalf("expected id: %s", out)
	}
}

func TestUnaryEqualsIntFold(t *testing.T) {
	zero := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}
	five := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 5)}
	not0 := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{five}}
	if !not0.EqualsIntSess(testAmbientSession, 0) {
		t.Fatal("!nonzero equals 0")
	}
	not1 := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{zero}}
	if !not1.EqualsIntSess(testAmbientSession, 1) {
		t.Fatal("!0 equals 1")
	}
	neg := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{five}}
	if !neg.EqualsIntSess(testAmbientSession, -5) {
		t.Fatal("-5 equals -5")
	}
}

func TestMakeRandomBinaryNoFloatWhenDisabled(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.EnableFloat = false
	ft := GetSimpleTypeSess(testAmbientSession, EFloat)
	f := MakeRandomBinaryKindSess(testAmbientSession, NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), ft, ft, ft, SafeOpBinary, BinAdd)
	if f == nil {
		t.Fatal("MakeRandomBinaryKind nil", HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	if f.Size == SafeFloat {
		t.Fatal("float size without EnableFloat")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputWrapperH(t *testing.T) {
	ClearSafeOpWrapperNamesSess(testAmbientSession)
	defer ClearSafeOpWrapperNamesSess(testAmbientSession)
	if OutputWrapperHSess(testAmbientSession, ) != "#define N_WRAP 0\n" {
		t.Fatal(OutputWrapperHSess(testAmbientSession, ))
	}
	_ = SafeOpFlagsToIDSess(testAmbientSession, "func_add_int32_t")
	_ = SafeOpFlagsToIDSess(testAmbientSession, "func_sub_int32_t")
	if WrapperNamesCountSess(testAmbientSession) != 2 {
		t.Fatal(WrapperNamesCountSess(testAmbientSession))
	}
	if OutputWrapperHSess(testAmbientSession, ) != "#define N_WRAP 2\n" {
		t.Fatal(OutputWrapperHSess(testAmbientSession, ))
	}
}

func TestGoGeneratorIdentifyWrappers(t *testing.T) {
	opts := Defaults()
	opts.IdentifyWrappers = true
	opts.SafeMath = true
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 2
	opts.MaxBlockDepth = 2
	// force some safe math usage by generating
	sess := NewSession(opts)
	ClearSafeOpWrapperNamesSess(sess)
	// pre-register on the run bag so N_WRAP non-zero even if gen avoids safe ops
	_ = SafeOpFlagsToIDSess(sess, "func_add_int32_t")
	g := NewProgramGenerator(sess)
	out := g.GoGenerator()
	if !strings.Contains(out, "wrapper.h") || !strings.Contains(out, "N_WRAP") {
		t.Fatal(out[len(out)-200:])
	}
	if g.WrapperHeader() != OutputWrapperHSess(g.Sess) {
		t.Fatal(g.WrapperHeader())
	}
	if !strings.Contains(out, "#define N_WRAP 1") && !strings.Contains(out, "#define N_WRAP ") {
		t.Fatal("wrapper section must emit N_WRAP from run bag", out[len(out)-200:])
	}
}
