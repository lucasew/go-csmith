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
	// force constants as operands via depth
	r := NewRng(3)
	fi := MakeRandomBinaryInvocation(r, opts, probs, vs, tables, EmptyCGContext(), GetIntType())
	if fi.Safe == nil {
		t.Fatal("expected Safe flags when SafeMath")
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
	fi := MakeRandomBinaryInvocation(NewRng(3), opts, probs, vs, tables, EmptyCGContext(), GetIntType())
	if fi.Safe != nil {
		t.Fatal("no flags when SafeMath off")
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
