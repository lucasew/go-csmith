package csmith

import "testing"

func TestWriteVarNonVolatileKeepsSEFree(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g8 := CreateVariableScalars("g_8", GetIntType(), false, false)
	if g8.IsVolatile() {
		t.Fatal("g_8 must not be volatile")
	}
	e := EmptyEffect().WriteVarSess(testAmbientSession, g8)
	if !e.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("WriteVar non-vol must keep SE-free, got SE=%v err=%v", e.IsSideEffectFreeSess(testAmbientSession), HasErrorSess(testAmbientSession))
	}
	acc := EmptyEffect().AddExternalEffectSess(testAmbientSession, e)
	if !acc.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("AddExternalEffect of non-vol write must keep SE-free, SE=%v err=%v w=%v", acc.IsSideEffectFreeSess(testAmbientSession), HasErrorSess(testAmbientSession), acc.WrittenVarsSess(testAmbientSession))
	}
	merged := EmptyEffect().AddEffectSess(testAmbientSession, acc)
	if !merged.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("AddEffect of SE-free accum must keep SE-free, SE=%v", merged.IsSideEffectFreeSess(testAmbientSession))
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
	ctx := EmptyEffect().WriteVarSess(testAmbientSession, g8)
	if !ctx.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("non-vol write context must be SE-free")
	}
	acc := zero.AddExternalEffectSess(testAmbientSession, ctx)
	if !acc.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("AddExternalEffect onto zero Accum must yield SE-free for non-vol write, SE=%v w=%v pure=%v",
			acc.IsSideEffectFreeSess(testAmbientSession), acc.WrittenVarsSess(testAmbientSession), acc.IsPureSess(testAmbientSession))
	}
	// BUILD path: parent.AddEffectSess(testAmbientSession, accum)
	merged := EmptyEffect().AddEffectSess(testAmbientSession, acc)
	if !merged.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("BUILD effect_context must stay SE-free, SE=%v", merged.IsSideEffectFreeSess(testAmbientSession))
	}
	// Factory initializes AccumEffContext
	f := &Function{Name: "func_x", ReturnType: GetIntType(), AccumEffContext: EmptyEffect(), FEffect: EmptyEffect()}
	if !f.AccumEffContext.IsSideEffectFreeSess(testAmbientSession) || !f.FEffect.IsSideEffectFreeSess(testAmbientSession) {
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
	ctx := EmptyEffect().WriteVarSess(testAmbientSession, g8)
	acc2 := EmptyEffect().AddExternalEffectSess(testAmbientSession, ctx)
	if !acc2.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatalf("AddExternalEffect non-vol global write must stay SE-free, SE=%v pure=%v w=%d err=%v",
			acc2.IsSideEffectFreeSess(testAmbientSession), acc2.IsPureSess(testAmbientSession), len(acc2.WrittenVarsSess(testAmbientSession)), HasErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

// TestWithSideEffectsStillBlocksVolatile — empty-map heuristic must not treat
// WithSideEffects() as SE-free (restrictive ambient without named vars).
func TestWithSideEffectsStillBlocksVolatile(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if WithSideEffects().IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("WithSideEffects must not be SE-free")
	}
	ClearErrorSess(testAmbientSession)
}
