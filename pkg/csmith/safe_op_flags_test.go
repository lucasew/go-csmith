package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomBinarySafeName(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	r := NewRng(2)
	f := MakeRandomBinary(r, opts, probs, GetIntType())
	if f == nil {
		t.Fatal("nil flags")
	}
	name := f.BinaryFuncName("+")
	if !strings.HasPrefix(name, "safe_add_func_") {
		t.Fatalf("%q", name)
	}
	if !strings.Contains(name, "int") {
		t.Fatalf("size missing %q", name)
	}
	// ends with _s_s or _u_u
	if !strings.HasSuffix(name, "_s_s") && !strings.HasSuffix(name, "_u_u") {
		t.Fatalf("sign suffix %q", name)
	}
}

func TestSafeBinaryInvocationOutput(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// FunctionInvocationBinary.cpp:68 assert(blk) for safe_ops temps
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// Full eBinaryOps includes cmp/logic; Output uses safe_* only for arith/shift.
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		ClearError()
		cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		fi = MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, tables, &cg, GetIntType())
		if fi != nil && fi.Safe != nil && SafeOpsBinary(fi.Binary) {
			break
		}
		fi = nil
	}
	if fi == nil {
		t.Fatal("expected Safe flags + arith/shift op")
	}
	out := fi.Output()
	if !strings.Contains(out, "safe_") {
		t.Fatalf("expected safe wrapper: %s", out)
	}
}

func TestNoSafeWhenDisabled(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = false
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// C++ always builds SafeOpFlags; Output must not emit safe_* when SafeMath off.
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	fi := MakeRandomBinaryInvocation(NewRng(3), opts, probs, vs, tables, &cg, GetIntType())
	if fi == nil {
		t.Fatal("nil inv")
	}
	if !fi.OutSafeMath {
		// ok
	} else {
		t.Fatal("OutSafeMath should mirror opts.SafeMath=false")
	}
	out := fi.Output()
	if strings.Contains(out, "safe_") {
		t.Fatalf("unexpected safe: %s", out)
	}
}

func TestGenerateUsesSafeMathByDefault(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 30; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "safe_add_func_") || strings.Contains(out, "safe_sub_func_") ||
			strings.Contains(out, "safe_mul_func_") || strings.Contains(out, "safe_div_func_") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected safe_*_func in some default seed")
	}
}

func TestPickSafeOpSizeFromSessionProbs(t *testing.T) {
	// SafeOpFlags.cpp:164 — SAFE_OPS_SIZE_PROB_FILTER from Probabilities group
	opts := Defaults()
	probs := NewProbabilities(opts)
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
	// nil probs arg + nil process → fail closed
	if _, ok := pickSafeOpSize(NewRng(1), nil); ok {
		t.Fatal("nil probs must fail closed")
	}
	// explicit probs works
	sz, ok := pickSafeOpSize(NewRng(2), probs)
	if !ok || int(sz) < 0 || int(sz) >= MaxSafeOpSizeNonFloat {
		t.Fatalf("got %v ok=%v", sz, ok)
	}
	// Int8 gated
	opts.Int8 = false
	opts.UInt8 = false
	probs2 := NewProbabilities(opts)
	for i := 0; i < 100; i++ {
		sz, ok := pickSafeOpSize(NewRng(uint64(i+1)), probs2)
		if !ok {
			t.Fatal("want size")
		}
		if sz == SafeInt8 {
			t.Fatal("Int8 disabled")
		}
	}
}
