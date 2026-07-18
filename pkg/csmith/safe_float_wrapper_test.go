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
	// no float unary safe wrapper — UnaryMinusFuncName falls back to int32
	if name := f.UnaryMinusFuncName(); name != "safe_unary_minus_func_int32_t_s" {
		t.Fatalf("unary minus name %q", name)
	}
}

func TestMakeRandomUnaryIntPath(t *testing.T) {
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

func TestBinaryFuncNameFloat(t *testing.T) {
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
}

func TestMakeRandomBinaryNoFloatWhenDisabled(t *testing.T) {
	opts := Defaults()
	opts.EnableFloat = false
	ft := GetSimpleType(EFloat)
	f := MakeRandomBinaryKind(NewRng(2), opts, NewProbabilities(opts), ft, ft, ft, SafeOpBinary, BinAdd)
	if f.Size == SafeFloat {
		t.Fatal("float size without EnableFloat")
	}
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
