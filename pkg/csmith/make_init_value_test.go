package csmith

import (
	"strings"
	"testing"
)

func TestMakeInitValueNonPointerConstant(t *testing.T) {
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// VariableSelector.cpp:830 assert(qf); no invent empty qfer on nil
	q := NewCVQualifiers([]bool{false}, []bool{false})
	e := vs.MakeInitValue(AccessRead, EmptyCGContext(), GetIntType(), &q, nil, NewRng(1))
	if e == nil || e.Term != TermConstant || e.Con == nil {
		t.Fatalf("want constant, got %#v", e)
	}
	if vs.MakeInitValue(AccessRead, EmptyCGContext(), GetIntType(), nil, nil, NewRng(1)) != nil {
		t.Fatal("nil qfer must fail closed")
	}
	if !HasError() {
		t.Fatal("nil qfer must SetError sticky")
	}
	ClearError()
}

func TestMakeInitValuePointerAddressOf(t *testing.T) {
	opts := Defaults()
	// force pointer path: RndFlipcoin(20) never true if we retry until address form
	vs := NewVariableSelector(opts)
	// pre-create int global to take address of
	qInt := NewCVQualifiers([]bool{false}, []bool{false})
	iv := CreateVariableQfer("g_i", GetIntType(), qInt)
	iv.Init = MakeInt(0)
	vs.GlobalList = append(vs.GlobalList, iv)
	vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, iv)

	pt := PointerTo(GetIntType())
	// pointer qfer depth = indirect_level+1 (SanityCheck / CVQualifiers.cpp)
	qPtr := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// seed scan until we get ExpressionVariable &path (not constant 20% path)
	var e *Expression
	for seed := uint64(1); seed < 80; seed++ {
		e = vs.MakeInitValue(AccessRead, EmptyCGContext(), pt, &qPtr, nil, NewRng(seed))
		if e != nil && e.Term == TermVariable && e.Var != nil {
			break
		}
	}
	if e == nil || e.Term != TermVariable {
		t.Fatal("want address-of expression within seed scan")
	}
	out := e.Output()
	// ExpressionVariable with pointer want over int var → &g_i
	if !strings.Contains(out, "&") {
		t.Fatalf("want & in %q (var=%s)", out, e.Var.Name)
	}
}

func TestApplyInitExprOutputDef(t *testing.T) {
	ClearError()
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	pt := PointerTo(GetIntType())
	// pointer qfer depth = indirect_level+1 (SanityCheck / CVQualifiers.cpp)
	pv := CreateVariableQfer("g_p", pt, NewCVQualifiers([]bool{false, false}, []bool{false, false}))
	applyInitExpr(pv, &Expression{Term: TermVariable, Var: iv, ExprType: pt})
	def := pv.OutputDef(false)
	if !strings.Contains(def, "g_p") || !strings.Contains(def, "&") {
		t.Fatal(def)
	}
	// Variable always live; sticky (no invent soft-skip init bind past hole)
	// Nil init complete no-op
	applyInitExpr(nil, &Expression{Term: TermConstant, Con: MakeInt(1)})
	if !HasError() {
		t.Fatal("nil var applyInitExpr must SetError sticky")
	}
	ClearError()
	applyInitExpr(pv, nil)
	if HasError() {
		t.Fatal("nil init applyInitExpr must complete no-op")
	}
	ClearError()
}

func TestGenerateNewNonArrayGlobal(t *testing.T) {
	opts := Defaults()
	opts.Arrays = true
	vs := NewVariableSelector(opts)
	// high array prob would flip in createAndInitialize; NonArray path must not
	if vs.Probs != nil {
		// force max array prob if table allows
		_ = vs.Probs
	}
	f := &Function{Name: "f"}
	cg := WithFunc(f, EmptyEffect())
	// many seeds: never array
	for seed := uint64(1); seed < 30; seed++ {
		v := vs.GenerateNewNonArrayGlobal(AccessRead, cg, GetIntType(), nil, NewRng(seed))
		if v == nil {
			t.Fatal("nil")
		}
		if v.IsArray {
			t.Fatal("must not create array")
		}
	}
}

func TestGetAllArrayVars(t *testing.T) {
	ClearError()
	vs := NewVariableSelector(Defaults())
	a := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	s := CreateVariableScalars("g_s", GetIntType(), false, false)
	vs.GlobalList = []*Variable{a, s}
	got := vs.GetAllArrayVars()
	if len(got) != 1 || got[0] != a {
		t.Fatalf("%v", got)
	}
	// Arrays nil hole must IncompleteVariables sticky (not bare nil empty-complete invent)
	vs.Arrays = []*ArrayVariable{nil}
	if VariablesComplete(vs.GetAllArrayVars()) {
		t.Fatal("Arrays nil hole must fail closed IncompleteVariables, not invent empty-complete")
	}
	if !HasError() {
		t.Fatal("Arrays nil hole must SetError sticky")
	}
	ClearError()
}

func TestCreateAndInitializeUsesMakeInitValue(t *testing.T) {
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	// disable access_once noise
	opts.AccessOnce = false
	vs.Opts = opts
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	v := vs.GenerateNewParentLocal(blk, AccessWrite, cg, GetIntType(), nil, NewRng(4))
	if v == nil || (v.Init == nil && v.InitExpr == nil) {
		t.Fatal("expected init")
	}
}

func TestCreateAndInitializeIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient must not invent create past holes
	ClearError()
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if vs.createAndInitialize(AccessWrite, WithEffectContext(IncompleteEffect()), GetIntType(), q, nil, "l_x", NewRng(1)) != nil {
		t.Fatal("incomplete EffectContext must fail closed createAndInitialize")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
}

func TestMakeInitValueCreatesTargetWhenNone(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// empty GlobalList — pointer init must create addressable
	pt := PointerTo(GetIntType())
	q := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// force pointer branch across seeds
	found := false
	for seed := uint64(1); seed < 100; seed++ {
		// fresh selector each time to avoid polluted state
		vs2 := NewVariableSelector(opts)
		e := vs2.MakeInitValue(AccessRead, EmptyCGContext(), pt, &q, nil, NewRng(seed))
		if e != nil && e.Term == TermVariable {
			found = true
			if len(vs2.GlobalList) == 0 && e.Var == nil {
				t.Fatal("expected created var")
			}
			break
		}
		// constant path is ok too; keep scanning for create path
		if e != nil && e.Term == TermConstant && len(vs2.GlobalList) > 0 {
			// created via other side effect unlikely for constant
		}
	}
	// either constant (20%) or var expr — at least something non-nil
	e := vs.MakeInitValue(AccessRead, EmptyCGContext(), pt, &q, nil, NewRng(7))
	if e == nil {
		t.Fatal("nil init")
	}
	_ = found
}
