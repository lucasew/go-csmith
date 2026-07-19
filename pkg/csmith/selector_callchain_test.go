package csmith

import (
	"strings"
	"testing"
)

func TestMatchVarName(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	if v.MatchVarName("g_1") != v {
		t.Fatal("self")
	}
	// struct field
	st := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType()}}}
	sv := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Skip("no fields")
	}
	f0 := sv.FieldVars[0]
	if sv.MatchVarName(f0.Name) != f0 {
		t.Fatal("field", f0.Name)
	}
	if v.MatchVarName("nope") != nil {
		t.Fatal("miss")
	}
}

func TestFindVarByName(t *testing.T) {
	ClearError()
	vs := NewVariableSelector(Defaults())
	v := CreateVariableScalars("g_x", GetIntType(), true, false)
	vs.AllVars = append(vs.AllVars, v)
	if vs.FindVarByName("g_x") != v {
		t.Fatal("find")
	}
	if vs.FindVarByName("g_y") != nil {
		t.Fatal("miss")
	}
	// empty name sticky
	ClearError()
	if vs.FindVarByName("") != nil {
		t.Fatal("empty name must fail closed")
	}
	if !HasError() {
		t.Fatal("empty name FindVarByName must SetError sticky")
	}
	// nil hole sticky
	ClearError()
	vs.AllVars = []*Variable{v, nil}
	if vs.FindVarByName("g_y") != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil hole FindVarByName must SetError sticky")
	}
	ClearError()
}

func TestIsSeenName(t *testing.T) {
	if !IsSeenName([]string{"g_a"}, "g_a[0]") {
		t.Fatal("seen")
	}
	if IsSeenName([]string{"g_a"}, "g_b[0]") {
		t.Fatal("not")
	}
}

func TestItemizeArrayWithIV(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("create")
	}
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.IVBounds = map[*Variable]int{iv: 0}
	// need dim 1 array with size > 0
	if len(av.Sizes) < 1 {
		av.Sizes = []int{4}
	}
	// one IV for one dim
	item := vs.ItemizeArray(NewRng(2), cg, av)
	if item == nil {
		t.Fatal("itemize")
	}
	if item.Collective != av {
		t.Fatal("collective")
	}
	if len(item.Indices) != len(av.Sizes) {
		t.Fatal(item.Indices)
	}
	// index uses iv name
	if !strings.Contains(item.Indices[0], "i") {
		t.Fatal(item.Indices[0])
	}
	out := item.OutputAccess()
	if !strings.Contains(out, "g_a[") {
		t.Fatal(out)
	}
}

func TestItemizeArrayTooFewIV(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", IsArray: true, Type: GetIntType()},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	cg := EmptyCGContext()
	// only one IV for 2 dims
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	cg.IVBounds = map[*Variable]int{iv: 0}
	if vs.ItemizeArray(NewRng(1), cg, av) != nil {
		t.Fatal("expect nil")
	}
}

func TestOutputCallChain(t *testing.T) {
	f := &Function{Name: "func_1"}
	b1 := &Block{Func: f}
	b2 := &Block{Func: f}
	cg := EmptyCGContext()
	cg.CallChain = []*Block{b1, b2}
	s := cg.OutputCallChain()
	if !strings.Contains(s, "func_1") || !strings.Contains(s, " -> ") {
		t.Fatal(s)
	}
	// incomplete frame — no invent "?" / blank " in "
	cg.CallChain = []*Block{b1, {Func: nil}}
	if out := cg.OutputCallChain(); out != "" {
		t.Fatal("nil Func must fail closed call chain", out)
	}
	cg.CallChain = []*Block{{Func: &Function{Name: ""}}}
	if out := cg.OutputCallChain(); out != "" {
		t.Fatal("empty func name must fail closed call chain", out)
	}
}

func TestVariableSelectorDoFinalization(t *testing.T) {
	vs := NewVariableSelector(Defaults())
	vs.AllVars = []*Variable{CreateVariableScalars("g_1", GetIntType(), true, false)}
	vs.GlobalList = vs.AllVars
	vs.DoFinalization()
	if len(vs.AllVars) != 0 || len(vs.GlobalList) != 0 {
		t.Fatal("cleared")
	}
}
