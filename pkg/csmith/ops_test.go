package csmith

import "testing"

func TestPickBinaryOpFullRange(t *testing.T) {
	// FunctionInvocation.cpp:179 — BINARY_OPS_PROB_FILTER from process Probabilities
	opts := Defaults()
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	seen := map[BinaryOp]bool{}
	r := NewRngSess(testAmbientSession, 1)
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
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 200; i++ {
		op := PickBinaryOp(r, opts)
		if op == BinMul || op == BinDiv {
			t.Fatalf("got disabled op %v", op)
		}
	}
}

func TestPickBinaryOpNilProbsFailClosed(t *testing.T) {
	// no soft invent BinaryOpsFilter(opts) when process Probabilities unset
	// non-sticky MAX (sticky poisons unit paths without process singleton)
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	op := PickBinaryOp(NewRngSess(testAmbientSession, 1), Defaults())
	if int(op) != MaxBinaryOp {
		t.Fatalf("want MAX without process probs, got %v", op)
	}
}

func TestPickBinaryUnaryOpNilRNGSticky(t *testing.T) {
	// always rnd_upto; sticky no invent eAdd/eMinus without RNG
	ClearErrorSess(testAmbientSession)
	if int(PickBinaryOp(nil, Defaults())) != MaxBinaryOp {
		t.Fatal("nil RNG PickBinaryOp must fail closed MAX")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG PickBinaryOp must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if int(PickUnaryOp(nil, Defaults())) != MaxUnaryOp {
		t.Fatal("nil RNG PickUnaryOp must fail closed MAX")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG PickUnaryOp must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBinaryOpCTokens(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if BinAdd.BinaryOpC() != "+" || BinAnd.BinaryOpC() != "&&" || BinLShift.BinaryOpC() != "<<" {
		t.Fatal("token map")
	}
	if BinCmpLt.CmpOpC() != "<" {
		t.Fatal("cmp")
	}
	// invalid op tokens sticky — no invent "+" / "<" / "-"
	if BinaryOp(MaxBinaryOp).BinaryOpC() != "" {
		t.Fatal("invalid BinaryOpC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid BinaryOpC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if BinAdd.CmpOpC() != "" {
		t.Fatal("non-cmp CmpOpC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-cmp CmpOpC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if UnaryOp(MaxUnaryOp).UnaryOpC() != "" {
		t.Fatal("invalid UnaryOpC must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("invalid UnaryOpC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
