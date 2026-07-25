package csmith

import (
	"strings"
	"testing"
)

// CreateArrayVariable: total=8 → pure_rnd_upto(4) ∈ 0..3 alts.
// Seed-2 func_11 brace multi-value needs InitExprs; empty alts → single-value only.
func TestCreateArrayVariableProducesAlts(t *testing.T) {
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	elem := GetSimpleTypeSess(testAmbientSession, EUInt)
	if elem == nil {
		t.Fatal("no uint")
	}
	withAlts := 0
	n8 := 0
	for seed := uint64(1); seed <= 500; seed++ {
		ClearErrorSess(testAmbientSession)
		ResetArrayInitSeedSess(testAmbientSession)
		r := NewRngSess(testAmbientSession, seed)
		SetProcessRngSess(testAmbientSession, r)
		vs := NewVariableSelector(testAmbientSession, opts)
		vs.Probs = probs
		blk := &Block{StmID: 1}
		init := MakeRandomSess(testAmbientSession, elem, opts, probs, r)
		if init == nil {
			continue
		}
		r = NewRngSess(testAmbientSession, seed)
		SetProcessRngSess(testAmbientSession, r)
		av := CreateArrayVariable(r, opts, probs, vs, nil, blk, "l_arr", elem, init, NewCVQualifiersSess(testAmbientSession, nil, nil))
		if av == nil || HasErrorSess(testAmbientSession) {
			continue
		}
		total := 1
		for _, s := range av.Sizes {
			total *= s
		}
		if total != 8 {
			continue
		}
		n8++
		if len(av.InitExprs) > 0 {
			withAlts++
		}
	}
	if n8 < 5 {
		t.Fatalf("too few size-8 arrays in band: %d", n8)
	}
	// P(init_num=0)=1/4 for pure_rnd_upto(4); expect majority with alts
	if withAlts*4 < n8*2 {
		t.Fatalf("size-8 with alts %d/%d too low", withAlts, n8)
	}
	t.Logf("size-8 arrays=%d withAlts=%d", n8, withAlts)
}

// force_non_uniform with n=3 (not power-of-2) must vary indices and emit
// more than one init token. ArrayVariable.cpp:433–437 seed formula.
func TestBuildInitRecursiveThreeStringsVaries(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ResetArrayInitSeedSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	elem := GetSimpleTypeSess(testAmbientSession, EUInt)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "l_t", Type: elem, IsArray: true,
			InitExpr: &Expression{Term: TermConstant, Con: &Constant{Value: "A", Type: elem}, ExprType: elem},
		},
		Sizes: []int{8},
		InitExprs: []*Expression{
			{Term: TermConstant, Con: &Constant{Value: "B", Type: elem}, ExprType: elem},
			{Term: TermConstant, Con: &Constant{Value: "C", Type: elem}, ExprType: elem},
		},
		InitValues: []string{"B", "C"},
	}
	av.AsArray = av
	def := av.OutputDefSess(testAmbientSession, Defaults())
	if def == "" || HasErrorSess(testAmbientSession) {
		t.Fatalf("OutputDef fail err=%v", GetErrorSess(testAmbientSession))
	}
	// With n=3, seed 0xABCDEF yields varied indices (not all A).
	if !strings.Contains(def, "B") && !strings.Contains(def, "C") {
		t.Fatalf("want non-primary tokens in non-uniform brace, got %s", def)
	}
}
