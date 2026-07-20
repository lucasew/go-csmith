package csmith

import "testing"

func TestMakeRandomBinaryPtrComparisonFlags(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	pt := env.FindPointerType(GetIntType(), true)
	if pt == nil || !env.HasPointerType() {
		t.Fatal("pointer type")
	}
	vs.Types = env
	cg := EmptyCGContext()
	cg.Types = env
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	BookkeeperDoFinalization()
	fi := MakeRandomBinaryPtrComparison(NewRng(5), opts, probs, vs, NewExprTables(opts), &cg, env)
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
}

func TestMakeRandomBinaryMayPickPtrCmp(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	_ = env.FindPointerType(GetIntType(), true)
	vs.Types = env
	cg := EmptyCGContext()
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
	BookkeeperDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	lhs := &Expression{Term: TermVariable, Var: p, ExprType: PointerTo(GetIntType())}
	rhs := &Expression{
		Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		ExprType: PointerTo(GetIntType()),
	}
	RecordPointerComparisons(lhs, rhs)
}

func TestPtrCmpCastGetTypeResidualNoInventShell(t *testing.T) {
	// Soft invent was left.GetType residual / CheckAndSetCast residual then invent PtrCmp inv.
	// Fair: residual stickies fail closed before flags shell.
	ClearError()
	opts := Defaults()
	opts.LangCPP = true
	// Type-nil left GetType residual
	left := &Expression{Term: TermVariable, Var: &Variable{Name: "g_hole"}}
	lt := left.GetType()
	if lt != nil || !HasError() {
		t.Fatal("Type-nil left GetType must residual sticky nil")
	}
	// ptr-cmp path after GetType residual: fail closed (no invent inv shell)
	ClearError()
	// CheckAndSetCast residual on incomplete right after live left type
	rightHole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_r"}}
	rightHole.CheckAndSetCastOpts(PointerTo(GetIntType()), opts)
	if rightHole.CastType != nil {
		t.Fatal("GetTypeUncast residual must not invent CastType on right")
	}
	if !HasError() {
		t.Fatal("CheckAndSetCast residual must SetError sticky")
	}
	ClearError()
}
