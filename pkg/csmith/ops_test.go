package csmith

import "testing"

func TestPickBinaryOpFullRange(t *testing.T) {
	// FunctionInvocation.cpp:179 — BINARY_OPS_PROB_FILTER from process Probabilities
	opts := Defaults()
	prev := ProcessProbabilities()
	SetProcessProbabilities(NewProbabilities(opts))
	defer SetProcessProbabilities(prev)
	seen := map[BinaryOp]bool{}
	r := NewRng(1)
	for i := 0; i < 500; i++ {
		op := PickBinaryOp(r, opts)
		if int(op) < 0 || int(op) >= MaxBinaryOp {
			t.Fatalf("out of range %v", op)
		}
		seen[op] = true
	}
	// with defaults all ops weight 1 — expect most of them
	if len(seen) < 10 {
		t.Fatalf("too few distinct ops: %d %#v", len(seen), seen)
	}
}

func TestPickBinaryOpRespectsNoMuls(t *testing.T) {
	// Probabilities.cpp:711–717 — mul/div weight 0 when CGOptions off
	opts := Defaults()
	opts.Muls = false
	opts.Divs = false
	prev := ProcessProbabilities()
	SetProcessProbabilities(NewProbabilities(opts))
	defer SetProcessProbabilities(prev)
	r := NewRng(2)
	for i := 0; i < 200; i++ {
		op := PickBinaryOp(r, opts)
		if op == BinMul || op == BinDiv {
			t.Fatalf("got disabled op %v", op)
		}
	}
}

func TestPickBinaryOpNilProbsFailClosed(t *testing.T) {
	// no soft invent BinaryOpsFilter(opts) when process Probabilities unset
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
	op := PickBinaryOp(NewRng(1), Defaults())
	if int(op) != MaxBinaryOp {
		t.Fatalf("want MAX without process probs, got %v", op)
	}
}

func TestBinaryOpCTokens(t *testing.T) {
	if BinAdd.BinaryOpC() != "+" || BinAnd.BinaryOpC() != "&&" || BinLShift.BinaryOpC() != "<<" {
		t.Fatal("token map")
	}
	if BinCmpLt.CmpOpC() != "<" {
		t.Fatal("cmp")
	}
}

func TestSafeShiftFuncNameUsesOp2(t *testing.T) {
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: false, IsFunc: true, Size: SafeInt32}
	name := f.BinaryFuncName("<<")
	if name != "safe_lshift_func_int32_t_s_u" {
		t.Fatalf("got %s", name)
	}
	// non-shift still doubles op1
	add := f.BinaryFuncName("+")
	if add != "safe_add_func_int32_t_s_s" {
		t.Fatalf("add %s", add)
	}
}
