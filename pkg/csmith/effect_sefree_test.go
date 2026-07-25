package csmith

import "testing"

func TestWriteVarNonVolatileKeepsSEFree(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g8 := CreateVariableScalars("g_8", GetIntType(), false, false)
	if g8.IsVolatile() {
		t.Fatal("g_8 must not be volatile")
	}
	e := EmptyEffect().WriteVar(g8)
	if !e.IsSideEffectFree() {
		t.Fatalf("WriteVar non-vol must keep SE-free, got SE=%v err=%v", e.IsSideEffectFree(), HasErrorSess(testAmbientSession))
	}
	acc := EmptyEffect().AddExternalEffect(e)
	if !acc.IsSideEffectFree() {
		t.Fatalf("AddExternalEffect of non-vol write must keep SE-free, SE=%v err=%v w=%v", acc.IsSideEffectFree(), HasErrorSess(testAmbientSession), acc.WrittenVars())
	}
	merged := EmptyEffect().AddEffect(acc)
	if !merged.IsSideEffectFree() {
		t.Fatalf("AddEffect of SE-free accum must keep SE-free, SE=%v", merged.IsSideEffectFree())
	}
	ClearErrorSess(testAmbientSession)
}

// TestZeroAccumEffContextAddExternal — Function::accum_eff_context is Effect() in C++
// (SE-free). Go zero Effect has sideEffectFree=false; AddExternalEffect must
// normalize empty base so BUILD revisit does not inherit non-SE-free context
// (seed-2 func_49 / first_div e37241).
func TestZeroAccumEffContextAddExternal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	var zero Effect // Go zero — not C++ Effect()
	g8 := CreateVariableScalars("g_8", GetIntType(), false, false)
	ctx := EmptyEffect().WriteVar(g8)
	if !ctx.IsSideEffectFree() {
		t.Fatal("non-vol write context must be SE-free")
	}
	acc := zero.AddExternalEffect(ctx)
	if !acc.IsSideEffectFree() {
		t.Fatalf("AddExternalEffect onto zero Accum must yield SE-free for non-vol write, SE=%v w=%v pure=%v",
			acc.IsSideEffectFree(), acc.WrittenVars(), acc.IsPure())
	}
	// BUILD path: parent.AddEffect(accum)
	merged := EmptyEffect().AddEffect(acc)
	if !merged.IsSideEffectFree() {
		t.Fatalf("BUILD effect_context must stay SE-free, SE=%v", merged.IsSideEffectFree())
	}
	// Factory initializes AccumEffContext
	f := &Function{Name: "func_x", ReturnType: GetIntType(), AccumEffContext: EmptyEffect(), FEffect: EmptyEffect()}
	if !f.AccumEffContext.IsSideEffectFree() || !f.FEffect.IsSideEffectFree() {
		t.Fatal("Function AccumEffContext/FEffect EmptyEffect must be SE-free")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddExternalEffectGlobalWriteKeepsSEFree(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g8 := CreateVariableScalars("g_8", GetIntType(), false, false)
	if !g8.IsGlobal() {
		t.Fatal("g_8 name must be IsGlobal")
	}
	ctx := EmptyEffect().WriteVar(g8)
	acc2 := EmptyEffect().AddExternalEffect(ctx)
	if !acc2.IsSideEffectFree() {
		t.Fatalf("AddExternalEffect non-vol global write must stay SE-free, SE=%v pure=%v w=%d err=%v",
			acc2.IsSideEffectFree(), acc2.IsPure(), len(acc2.WrittenVars()), HasErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

// TestWithSideEffectsStillBlocksVolatile — empty-map heuristic must not treat
// WithSideEffects() as SE-free (restrictive ambient without named vars).
func TestWithSideEffectsStillBlocksVolatile(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if WithSideEffects().IsSideEffectFree() {
		t.Fatal("WithSideEffects must not be SE-free")
	}
	ClearErrorSess(testAmbientSession)
}
