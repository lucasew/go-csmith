package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomBinaryFloatPath(t *testing.T) {
	opts := Defaults()
	opts.EnableFloat = true
	ft := GetSimpleType(EFloat)
	f := MakeRandomBinaryKind(NewRng(1), opts, NewProbabilities(opts), ft, ft, ft, SafeOpBinary, BinAdd)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Size != SafeFloat {
		t.Fatalf("size %v want SafeFloat", f.Size)
	}
	if !f.Op1Signed || !f.Op2Signed {
		t.Fatal("float always signed")
	}
	if f.SizeToken() != "float" {
		t.Fatal(f.SizeToken())
	}
}

func TestMakeRandomBinaryAssignKind(t *testing.T) {
	opts := Defaults()
	// assign kind: op2 == op1
	for seed := uint64(1); seed < 20; seed++ {
		f := MakeRandomBinaryKind(NewRng(seed), opts, NewProbabilities(opts), GetIntType(), GetIntType(), GetIntType(), SafeOpAssign, BinAdd)
		if f.Op1Signed != f.Op2Signed {
			t.Fatalf("seed %d: assign op2 should match op1", seed)
		}
	}
}

func TestMakeRandomUnaryFloatPath(t *testing.T) {
	opts := Defaults()
	opts.EnableFloat = true
	ft := GetSimpleType(EFloat)
	f := MakeRandomUnary(NewRng(1), opts, NewProbabilities(opts), ft, nil, UnMinus)
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
	ClearError()
	if name := f.UnaryMinusFuncName(); name != "" {
		t.Fatalf("float unary must fail closed, got %q", name)
	}
	if HasError() {
		t.Fatal("float UnaryMinusFuncName must stay non-sticky")
	}
	ClearError()
}

func TestMakeRandomUnaryIntPath(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.EnableFloat = false
	f := MakeRandomUnary(NewRng(3), opts, NewProbabilities(opts), GetIntType(), nil, UnMinus)
	if f == nil {
		t.Fatal("nil")
	}
	if f.Size == SafeFloat {
		t.Fatal("int unary must not pick SafeFloat")
	}
	name := f.UnaryMinusFuncName()
	if !strings.HasPrefix(name, "safe_unary_minus_func_") {
		t.Fatalf("%q", name)
	}
}

func TestUnaryMinusFuncNameNilFailClosed(t *testing.T) {
	// no soft invent default int32 name for nil flags
	var f *SafeOpFlags
	if f.UnaryMinusFuncName() != "" {
		t.Fatal("nil flags must fail closed")
	}
}

func TestCastOpNoInventEmptySizeToken(t *testing.T) {
	// invalid SafeOpSize → empty SizeToken; sticky no invent "(-()x)" / "(()a + ()b)"
	ClearError()
	if unaryCastMinus("", "x") != "" || unaryCastMinus("int32_t", "") != "" {
		t.Fatal("unary cast empty must fail closed")
	}
	if !HasError() {
		t.Fatal("unary cast empty must SetError sticky")
	}
	ClearError()
	if binaryCastOp("", "a", "+", "b") != "" || binaryCastOp("int32_t", "a", "", "b") != "" {
		t.Fatal("binary cast empty must fail closed")
	}
	if !HasError() {
		t.Fatal("binary cast empty must SetError sticky")
	}
	ClearError()
	fi := &Invocation{
		IsStd: true, IsUnary: true, Unary: "-",
		Args:        []*Expression{{Term: TermConstant, Con: MakeInt(1)}},
		Safe:        &SafeOpFlags{Size: SafeOpSize(99), Op1Signed: true},
		OutSafeMath: false,
	}
	if out := fi.Output(); out != "" {
		t.Fatal("invalid size unary cast must fail closed", out)
	}
	if !HasError() {
		t.Fatal("invalid size unary cast must SetError sticky")
	}
	ClearError()
}

func TestBinaryFuncNameInvalidSizeFailClosed(t *testing.T) {
	// SafeOpFlags.cpp:239 assert invalid size; sticky no invent safe_add_func__s_s
	ClearError()
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeOpSize(99)}
	if got := f.BinaryFuncName("+"); got != "" {
		t.Fatal("invalid size must fail closed BinaryFuncName", got)
	}
	if !HasError() {
		t.Fatal("invalid size BinaryFuncName must SetError sticky")
	}
	ClearError()
	if got := f.BinaryFuncName("<<"); got != "" {
		t.Fatal("invalid size shift must fail closed", got)
	}
	if !HasError() {
		t.Fatal("invalid size shift must SetError sticky")
	}
	ClearError()
}

func TestBinaryFuncNameFloat(t *testing.T) {
	ClearError()
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeFloat}
	if got := f.BinaryFuncName("+"); got != "safe_add_func_float_f_f" {
		t.Fatalf("add %q", got)
	}
	if got := f.BinaryFuncName("-"); got != "safe_sub_func_float_f_f" {
		t.Fatalf("sub %q", got)
	}
	if got := f.BinaryFuncName("*"); got != "safe_mul_func_float_f_f" {
		t.Fatalf("mul %q", got)
	}
	if got := f.BinaryFuncName("/"); got != "safe_div_func_float_f_f" {
		t.Fatalf("div %q", got)
	}
	if got := f.BinaryFuncName("%"); got != "" {
		t.Fatalf("mod should be empty for float, got %q", got)
	}
	// float mod empty is non-sticky (no wrapper, not broken IR)
	if HasError() {
		t.Fatal("float mod empty must stay non-sticky")
	}
	// float unary minus name non-sticky empty (cast emit fallthrough)
	if f.UnaryMinusFuncName() != "" {
		t.Fatal("float unary safe name must fail closed")
	}
	if HasError() {
		t.Fatal("float UnaryMinusFuncName must stay non-sticky")
	}
	ClearError()
}

func TestUnaryMinusOutputFloatUsesStandard(t *testing.T) {
	// FunctionInvocationUnary.cpp:220–223 — float size → standard minus
	arg := &Expression{Term: TermConstant, Con: MakeInt(3)}
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
	ClearSafeOpWrapperNames()
	defer ClearSafeOpWrapperNames()
	arg := &Expression{Term: TermConstant, Con: MakeInt(3)}
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
	ClearSafeOpWrapperNames()
	defer ClearSafeOpWrapperNames()
	// pre-register so id is known; filter only id 1 — deny if id != 1
	fname := "safe_unary_minus_func_int32_t_s"
	id := SafeOpFlagsToID(fname)
	arg := &Expression{Term: TermConstant, Con: MakeInt(3)}
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

func TestBinaryOutputIdentifyWrappers(t *testing.T) {
	ClearSafeOpWrapperNames()
	defer ClearSafeOpWrapperNames()
	a0 := &Expression{Term: TermConstant, Con: MakeInt(1)}
	a1 := &Expression{Term: TermConstant, Con: MakeInt(2)}
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
	zero := &Expression{Term: TermConstant, Con: MakeInt(0)}
	five := &Expression{Term: TermConstant, Con: MakeInt(5)}
	not0 := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{five}}
	if !not0.EqualsInt(0) {
		t.Fatal("!nonzero equals 0")
	}
	not1 := &Invocation{IsStd: true, IsUnary: true, Unary: "!", Args: []*Expression{zero}}
	if !not1.EqualsInt(1) {
		t.Fatal("!0 equals 1")
	}
	neg := &Invocation{IsStd: true, IsUnary: true, Unary: "-", Args: []*Expression{five}}
	if !neg.EqualsInt(-5) {
		t.Fatal("-5 equals -5")
	}
}

func TestMakeRandomBinaryNoFloatWhenDisabled(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.EnableFloat = false
	ft := GetSimpleType(EFloat)
	f := MakeRandomBinaryKind(NewRng(2), opts, NewProbabilities(opts), ft, ft, ft, SafeOpBinary, BinAdd)
	if f == nil {
		t.Fatal("MakeRandomBinaryKind nil", HasError(), GetError())
	}
	if f.Size == SafeFloat {
		t.Fatal("float size without EnableFloat")
	}
	ClearError()
}

func TestOutputWrapperH(t *testing.T) {
	ClearSafeOpWrapperNames()
	defer ClearSafeOpWrapperNames()
	if OutputWrapperH() != "#define N_WRAP 0\n" {
		t.Fatal(OutputWrapperH())
	}
	_ = SafeOpFlagsToID("func_add_int32_t")
	_ = SafeOpFlagsToID("func_sub_int32_t")
	if WrapperNamesCount() != 2 {
		t.Fatal(WrapperNamesCount())
	}
	if OutputWrapperH() != "#define N_WRAP 2\n" {
		t.Fatal(OutputWrapperH())
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
	ClearSafeOpWrapperNames()
	defer ClearSafeOpWrapperNames()
	// pre-register so N_WRAP non-zero even if gen avoids safe ops
	_ = SafeOpFlagsToID("func_add_int32_t")
	g := NewProgramGenerator(opts)
	out := g.GoGenerator()
	if !strings.Contains(out, "wrapper.h") || !strings.Contains(out, "N_WRAP") {
		t.Fatal(out[len(out)-200:])
	}
	if g.WrapperHeader() != OutputWrapperH() {
		t.Fatal(g.WrapperHeader())
	}
}
