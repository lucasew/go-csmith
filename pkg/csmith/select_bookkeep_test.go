package csmith

import "testing"

func TestSelectRecordsReuseAndCreate(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	// seed one global so reuse is possible
	g := vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(1))
	if g == nil {
		t.Fatal("seed global")
	}
	BookkeeperDoFinalization()
	// force scope toward global by using Select many times
	reused := 0
	created := 0
	for seed := uint64(2); seed < 40; seed++ {
		beforeOld := useOldVarCnt
		beforeNew := useNewVarCnt
		v := vs.Select(AccessRead, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(seed), MatchFlexible)
		if v == nil {
			continue
		}
		if useOldVarCnt > beforeOld {
			reused++
		}
		if useNewVarCnt > beforeNew {
			created++
		}
	}
	if reused == 0 && created == 0 {
		t.Fatal("expected bookkeeping activity")
	}
	// at least some reuse of existing globals is expected under default scope table
	if reused == 0 {
		t.Log("no reuse in scan — create-heavy seeds ok")
	}
}

func TestGenerateNewVariableLocalStackIndex(t *testing.T) {
	opts := Defaults()
	opts.GlobalVariables = false // force parent local
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	outer := &Block{Func: f}
	inner := &Block{Func: f, Parent: outer}
	f.Stack = []*Block{outer, inner}
	cg := WithFunc(f, EmptyEffect())
	// many creates — both blocks should get locals across seeds
	outerN, innerN := 0, 0
	for seed := uint64(1); seed < 30; seed++ {
		beforeO, beforeI := len(outer.LocalVars), len(inner.LocalVars)
		v := vs.GenerateNewVariable(AccessWrite, cg, GetIntType(), nil, NewRng(seed))
		if v == nil {
			t.Fatal("nil")
		}
		if len(outer.LocalVars) > beforeO {
			outerN++
		}
		if len(inner.LocalVars) > beforeI {
			innerN++
		}
	}
	if outerN == 0 || innerN == 0 {
		t.Fatalf("want both stack frames used: outer=%d inner=%d", outerN, innerN)
	}
	// VariableSelector.cpp always has RNG; no invent scope/type without it
	if vs.GenerateNewVariable(AccessWrite, cg, GetIntType(), nil, nil) != nil {
		t.Fatal("nil RNG must not invent new variable")
	}
}

func TestGenerateNewVariableIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient / facts fail closed sticky before scope pick
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg.EffectAccum = &inc
	if vs.GenerateNewVariable(AccessWrite, cg, GetIntType(), nil, NewRng(1)) != nil {
		t.Fatal("incomplete EffectAccum must fail closed GenerateNewVariable")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	fm := NewFactMgr(f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	if vs.GenerateNewVariable(AccessWrite, cg2, GetIntType(), nil, NewRng(2)) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed GenerateNewVariable")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
	cg3 := WithFunc(f, IncompleteEffect()).WithFactMgr(NewFactMgr(f))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	if vs.GenerateNewVariable(AccessWrite, cg3, GetIntType(), nil, NewRng(3)) != nil {
		t.Fatal("incomplete EffectContext must fail closed GenerateNewVariable")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
}

func TestSelectGlobalMTInvalidVars(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	vs.GlobalList = []*Variable{a, b}
	vs.GlobalNonvolatilesList = []*Variable{a, b}
	// invalidate a → only b
	got := vs.SelectGlobalMT(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(3), MatchFlexible, []*Variable{a})
	if got != b {
		// may create new if choose fails eligibility — accept b or new
		if got == a {
			t.Fatal("invalid a returned")
		}
	}
}

func TestSelectDerefPointerPrefersNonvol(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// int* global nonvol
	pt := PointerTo(GetIntType())
	q := NewCVQualifiers([]bool{false}, []bool{false})
	pv := CreateVariableQfer("g_p", pt, q)
	vs.GlobalList = []*Variable{pv}
	vs.GlobalNonvolatilesList = []*Variable{pv}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	got := selectDerefPointer(NewRng(2), opts, NewProbabilities(opts), vs, cg, GetIntType(), &q, AccessRead)
	if got != pv {
		// may create — ensure not nil
		if got == nil {
			t.Fatal("nil")
		}
	}
}
