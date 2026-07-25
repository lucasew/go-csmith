package csmith

import "testing"

func TestMakeRandomBinaryPtrComparisonFlags(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	pt := env.FindPointerType(GetIntType(), true)
	if pt == nil || !env.HasPointerType() {
		t.Fatal("pointer type")
	}
	vs.Types = env
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = env
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	BookkeeperDoFinalizationSess(testAmbientSession)
	// Operands may soft-miss under sparse env; flags stream order changed so seed 5 alone
	// is not stable — find any successful ptr_cmp.
	var fi *Invocation
	for seed := uint64(1); seed < 50; seed++ {
		ClearErrorSess(testAmbientSession)
		cg2 := EmptyCGContext().WithSession(testAmbientSession)
		cg2.Types = env
		eff2 := EmptyEffect()
		cg2.EffectAccum = &eff2
		fi = MakeRandomBinaryPtrComparison(NewRng(seed), opts, probs, vs, NewExprTables(opts), &cg2, env)
		if fi != nil {
			break
		}
	}
	_ = cg
	if fi == nil {
		t.Fatal("nil")
	}
	if !fi.PtrCmp {
		t.Fatal("PtrCmp")
	}
	if fi.Binary != "==" && fi.Binary != "!=" {
		t.Fatal(fi.Binary)
	}
	if len(fi.Args) != 2 || fi.Args[0] == nil || fi.Args[1] == nil {
		t.Fatal(fi.Args)
	}
	// FunctionInvocation.cpp:297–299 — SafeOpFlags always built (even if Output is infix)
	if fi.Safe == nil {
		t.Fatal("SafeOpFlags required on ptr_cmp (RNG order 1:1 with C++)")
	}
}

func TestMakeRandomBinaryPtrComparisonFlagOrderAndEqPolarity(t *testing.T) {
	// FunctionInvocation.cpp:295–304 order:
	//   F50 eq/ne → make_random_binary (F signed, F signed, U size) → choose_random_pointer_type
	// eq polarity: flip true → ==, false → !=
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	if env.FindPointerType(GetIntType(), true) == nil {
		t.Fatal("pointer type")
	}
	vs.Types = env
	// Seed where first flip (eq/ne) is false → must be !=
	// And flags consume draws before any ChooseRandomPointerType U(n=1)
	// Use Rng depth only through the flag section via a harness that stops early:
	// Compare two runs: pure MakeRandomBinaryKind depth vs full ptr-cmp prefix.
	rFlags := NewRng(7)
	d0 := rFlags.RandDepth()
	// eq/ne draw alone
	eqFlip := rFlags.RndFlipcoin(50)
	_ = eqFlip
	flags := MakeRandomBinaryKind(rFlags, opts, probs, GetIntType(), nil, nil, SafeOpBinary, BinCmpEq)
	if flags == nil {
		t.Fatal("flags")
	}
	// after eq + op1 + op2 + size: depth +1 +2 flip +1 upto (filter may re-roll size)
	flagDepth := rFlags.RandDepth() - d0
	if flagDepth < 4 {
		t.Fatalf("expected at least eq+2signed+size draws, got depth delta %d", flagDepth)
	}

	// Full factory: first events must be F50 then flag F50 F50 U… then U1 pointer
	// Polarity: simulate C++ ternary with known flip outcomes via depth-matched seeds.
	// Seed 1: find a seed where first flip is 1 → expect "=="
	foundEq, foundNe := false, false
	for seed := uint64(1); seed < 200 && !(foundEq && foundNe); seed++ {
		ClearErrorSess(testAmbientSession)
		cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, nil))
		cg.Types = env
		fi := MakeRandomBinaryPtrComparison(NewRng(seed), opts, probs, vs, NewExprTables(opts), &cg, env)
		if fi == nil {
			continue
		}
		// Replay first flip alone with same seed
		r0 := NewRng(seed)
		wantEq := r0.RndFlipcoin(50)
		if wantEq && fi.Binary == "==" {
			foundEq = true
		}
		if !wantEq && fi.Binary == "!=" {
			foundNe = true
		}
		if wantEq && fi.Binary != "==" {
			t.Fatalf("seed %d flip true must be == got %s", seed, fi.Binary)
		}
		if !wantEq && fi.Binary != "!=" {
			t.Fatalf("seed %d flip false must be != got %s", seed, fi.Binary)
		}
	}
	if !foundEq || !foundNe {
		t.Fatalf("need both polarities in sample: eq=%v ne=%v", foundEq, foundNe)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBinaryMayPickPtrCmp(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	env := &TypeEnv{Sess: testAmbientSession}
	_ = env.FindPointerType(GetIntType(), true)
	vs.Types = env
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.Types = env
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	for seed := uint64(1); seed < 100; seed++ {
		fi := MakeRandomBinaryInvocation(NewRng(seed), opts, probs, vs, NewExprTables(opts), &cg, GetIntType())
		if fi != nil && fi.PtrCmp {
			if fi.Binary != "==" && fi.Binary != "!=" {
				t.Fatal(fi.Binary)
			}
			return
		}
	}
	t.Log("no ptr cmp in 100 seeds (10% * has_ptr; ok if rare)")
}

func TestRecordPointerComparisonsGetType(t *testing.T) {
	BookkeeperDoFinalizationSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	lhs := &Expression{Term: TermVariable, Var: p, ExprType: PointerTo(GetIntType())}
	rhs := &Expression{
		Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		ExprType: PointerTo(GetIntType()),
	}
	RecordPointerComparisonsSess(testAmbientSession, lhs, rhs)
}

func TestPtrCmpCastGetTypeResidualNoInventShell(t *testing.T) {
	// Soft invent was left.GetType residual / CheckAndSetCast residual then invent PtrCmp inv.
	// Fair: residual stickies fail closed before flags shell.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.LangCPP = true
	// Type-nil left GetType residual
	left := &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole"}}
	lt := left.GetType()
	if lt != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil left GetType must residual sticky nil")
	}
	// ptr-cmp path after GetType residual: fail closed (no invent inv shell)
	ClearErrorSess(testAmbientSession)
	// CheckAndSetCast residual on incomplete right after live left type
	rightHole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_r"}}
	rightHole.CheckAndSetCastOpts(PointerTo(GetIntType()), opts)
	if rightHole.CastType != nil {
		t.Fatal("GetTypeUncast residual must not invent CastType on right")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CheckAndSetCast residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
