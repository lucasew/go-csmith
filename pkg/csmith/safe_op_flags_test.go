package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomBinarySafeName(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	r := NewRngSess(testAmbientSession, 2)
	f := MakeRandomBinarySess(testAmbientSession, r, opts, probs, GetIntTypeSess(testAmbientSession))
	if f == nil {
		t.Fatal("nil flags")
	}
	name := f.BinaryFuncNameSess(testAmbientSession, "+")
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

func TestMakeRandomSafeOpNilProbsNoInvent50(t *testing.T) {
	// nil probs → 0% signed coin (no invent default 50); pick size needs process probs
	// or fails closed. Ensure signed coin is not invent-50: Op1Signed false at 0%.
	opts := Defaults()
	// install process probs so size pick can succeed while call-site probs nil
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	for seed := uint64(1); seed < 20; seed++ {
		f := MakeRandomBinaryKindSess(testAmbientSession, NewRngSess(testAmbientSession, seed), opts, nil, GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), SafeOpBinary, BinAdd)
		if f == nil {
			// size pick may fail closed without call-site probs if process cleared
			continue
		}
		if f.Op1Signed || f.Op2Signed {
			t.Fatalf("nil probs must not invent signed true at 50%% seed=%d", seed)
		}
	}
	u := MakeRandomUnarySess(testAmbientSession, NewRngSess(testAmbientSession, 2), opts, nil, GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), UnMinus)
	if u != nil && u.Op1Signed {
		t.Fatal("nil probs unary must not invent signed true at 50%")
	}
}

func TestSafeBinaryInvocationOutput(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	// FunctionInvocationBinary.cpp:68 assert(blk) for safe_ops temps
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// Full eBinaryOps includes cmp/logic; Output uses safe_* only for arith/shift.
	var fi *Invocation
	for seed := uint64(1); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		fi = MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
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
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	// C++ always builds SafeOpFlags; Output must not emit safe_* when SafeMath off.
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	fi := MakeRandomBinaryInvocation(NewRngSess(testAmbientSession, 3), opts, probs, vs, tables, &cg, GetIntTypeSess(testAmbientSession))
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
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	ClearErrorSess(testAmbientSession)
	// nil probs arg + nil process → sticky fail closed
	if _, ok := pickSafeOpSizeSess(testAmbientSession, NewRngSess(testAmbientSession, 1), nil); ok {
		t.Fatal("nil probs must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil probs pickSafeOpSize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil RNG sticky
	if _, ok := pickSafeOpSizeSess(testAmbientSession, nil, probs); ok {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG pickSafeOpSize must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// explicit probs works
	sz, ok := pickSafeOpSizeSess(testAmbientSession, NewRngSess(testAmbientSession, 2), probs)
	if !ok || int(sz) < 0 || int(sz) >= MaxSafeOpSizeNonFloat {
		t.Fatalf("got %v ok=%v", sz, ok)
	}
	// Int8 gated
	opts.Int8 = false
	opts.UInt8 = false
	probs2 := NewProbabilities(opts)
	for i := 0; i < 100; i++ {
		sz, ok := pickSafeOpSizeSess(testAmbientSession, NewRngSess(testAmbientSession, uint64(i+1)), probs2)
		if !ok {
			t.Fatal("want size")
		}
		if sz == SafeInt8 {
			t.Fatal("Int8 disabled")
		}
	}
}

func TestMakeRandomSafeOpNilRNGSticky(t *testing.T) {
	// SafeOpFlags.cpp always uses rnd_*; no invent fixed flags
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if f := MakeRandomBinaryKindSess(testAmbientSession, nil, opts, NewProbabilities(opts), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), SafeOpBinary, BinAdd); f != nil {
		t.Fatal("nil RNG binary must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomBinaryKind must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if f := MakeRandomUnarySess(testAmbientSession, nil, opts, NewProbabilities(opts), GetIntTypeSess(testAmbientSession), GetIntTypeSess(testAmbientSession), UnMinus); f != nil {
		t.Fatal("nil RNG unary must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomUnary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeOpFlagsCloneNilSticky(t *testing.T) {
	// SafeOpFlags* always live at clone; sticky no invent soft-skip past hole
	ClearErrorSess(testAmbientSession)
	if (*SafeOpFlags)(nil).CloneSess(testAmbientSession) != nil {
		t.Fatal("nil Clone must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Clone must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeOpsSizeWeightNilSticky(t *testing.T) {
	// Probabilities always live at weight query; sticky 0 (no invent zero-weight soft-skip)
	ClearErrorSess(testAmbientSession)
	if (*Probabilities)(nil).SafeOpsSizeWeight(0) != 0 {
		t.Fatal("nil SafeOpsSizeWeight must fail closed 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SafeOpsSizeWeight must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeDummyFlagsAndGetters(t *testing.T) {
	// SafeOpFlags.cpp:61–63 make_dummy_flags; get_op*_sign / get_op_size
	f := MakeDummyFlags()
	if f.Op1SignSess(testAmbientSession) || f.Op2SignSess(testAmbientSession) || f.OpSizeSess(testAmbientSession) != SafeInt8 || f.IsFunc {
		t.Fatalf("dummy: %+v", f)
	}
	if f.CloneSess(testAmbientSession).Size != SafeInt8 {
		t.Fatal("clone")
	}
	// SafeOpKind order matches C++ enum class
	if SafeOpUnary != 0 || SafeOpBinary != 1 || SafeOpAssign != 2 || MaxSafeOpKind != 3 {
		t.Fatalf("kind order u=%d b=%d a=%d max=%d", SafeOpUnary, SafeOpBinary, SafeOpAssign, MaxSafeOpKind)
	}
	if MinimalDepth(DtSafeOpFlags, int(SafeOpBinary)) != 2 {
		t.Fatal("binary minimal depth")
	}
	if MinimalDepth(DtSafeOpFlags, int(SafeOpUnary)) != 3 {
		t.Fatal("unary minimal depth")
	}
	ClearErrorSess(testAmbientSession)
	if (*SafeOpFlags)(nil).Op1SignSess(testAmbientSession) || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Op1Sign sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeOpFlagsOutputPieces(t *testing.T) {
	// SafeOpFlags.cpp:219–255 OutputSize/FuncOrMacro/Sign/Op1/Op2
	f := &SafeOpFlags{Op1Signed: true, Op2Signed: false, IsFunc: true, Size: SafeInt32}
	if f.OutputFuncOrMacroSess(testAmbientSession) != "func_" || f.OutputOp1Sess(testAmbientSession) != "_s" || f.OutputOp2Sess(testAmbientSession) != "_u" {
		t.Fatalf("pieces: %q %q %q", f.OutputFuncOrMacroSess(testAmbientSession), f.OutputOp1Sess(testAmbientSession), f.OutputOp2Sess(testAmbientSession))
	}
	if f.OutputSizeSess(testAmbientSession) != "int32_t" {
		t.Fatal(f.OutputSizeSess(testAmbientSession))
	}
	// unsigned op1 prefixes size with u
	f.Op1Signed = false
	if f.OutputSizeSess(testAmbientSession) != "uint32_t" {
		t.Fatal(f.OutputSizeSess(testAmbientSession))
	}
	// float size token
	f.Size = SafeFloat
	if f.OutputSizeSess(testAmbientSession) != "float" {
		t.Fatal(f.OutputSizeSess(testAmbientSession))
	}
	// to_string binary via Output pieces
	f = &SafeOpFlags{Op1Signed: true, Op2Signed: true, IsFunc: true, Size: SafeInt32}
	if got := f.BinaryFuncNameSess(testAmbientSession, "+"); got != "safe_add_func_int32_t_s_s" {
		t.Fatal(got)
	}
	f.Op2Signed = false
	if got := f.BinaryFuncNameSess(testAmbientSession, "<<"); got != "safe_lshift_func_int32_t_s_u" {
		t.Fatal(got)
	}
	if got := f.UnaryMinusFuncNameSess(testAmbientSession); got != "safe_unary_minus_func_int32_t_s" {
		t.Fatal(got)
	}
	ClearErrorSess(testAmbientSession)
	if (*SafeOpFlags)(nil).OutputSizeSess(testAmbientSession) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil OutputSize sticky")
	}
	ClearErrorSess(testAmbientSession)
}
