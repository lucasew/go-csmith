package csmith

import "testing"

func TestPickBinaryOpFullRange(t *testing.T) {
	opts := Defaults()
	seen := map[BinaryOp]bool{}
	r := NewRng(1)
	for i := 0; i < 500; i++ {
		op := PickBinaryOp(r, opts)
		if int(op) < 0 || int(op) >= MaxBinaryOp {
			t.Fatalf("out of range %v", op)
		}
		seen[op] = true
	}
	// with defaults all 18 ops weight 1 — expect most of them
	if len(seen) < 10 {
		t.Fatalf("too few distinct ops: %d %#v", len(seen), seen)
	}
}

func TestPickBinaryOpRespectsNoMuls(t *testing.T) {
	opts := Defaults()
	opts.Muls = false
	opts.Divs = false
	r := NewRng(2)
	for i := 0; i < 200; i++ {
		op := PickBinaryOp(r, opts)
		if op == BinMul || op == BinDiv {
			t.Fatalf("got disabled op %v", op)
		}
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
