package csmith

import (
	"strings"
	"testing"
)

func TestMatchVarName(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	if v.MatchVarNameSess(testAmbientSession, "g_1") != v {
		t.Fatal("self")
	}
	// struct field
	st := &Type{isStruct: true, Fields: []StructField{{Name: "f0", Type: GetIntType()}}}
	sv := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Skip("no fields")
	}
	f0 := sv.FieldVars[0]
	if sv.MatchVarNameSess(testAmbientSession, f0.Name) != f0 {
		t.Fatal("field", f0.Name)
	}
	if v.MatchVarNameSess(testAmbientSession, "nope") != nil {
		t.Fatal("miss")
	}
}

func TestFindVarByName(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	vs := NewVariableSelector(testAmbientSession, Defaults())
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntType(), true, false)
	vs.AllVars = append(vs.AllVars, v)
	if vs.FindVarByName("g_x") != v {
		t.Fatal("find")
	}
	if vs.FindVarByName("g_y") != nil {
		t.Fatal("miss")
	}
	// nil VS sticky
	ClearErrorSess(testAmbientSession)
	if (*VariableSelector)(nil).FindVarByName("g_x") != nil {
		t.Fatal("nil VS must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil VS FindVarByName must SetError sticky")
	}
	// empty name sticky
	ClearErrorSess(testAmbientSession)
	if vs.FindVarByName("") != nil {
		t.Fatal("empty name must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name FindVarByName must SetError sticky")
	}
	// nil hole sticky
	ClearErrorSess(testAmbientSession)
	vs.AllVars = []*Variable{v, nil}
	if vs.FindVarByName("g_y") != nil {
		t.Fatal("nil hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole FindVarByName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsSeenName(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !IsSeenNameSess(testAmbientSession, []string{"g_a"}, "g_a[0]") {
		t.Fatal("seen")
	}
	if IsSeenNameSess(testAmbientSession, []string{"g_a"}, "g_b[0]") {
		t.Fatal("not")
	}
	// empty name sticky (no invent not-seen soft-miss past incomplete subject)
	if IsSeenNameSess(testAmbientSession, []string{"g_a"}, "") {
		t.Fatal("empty name must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name IsSeenName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty seen entry sticky (no invent is-seen via bare "[" prefix)
	if IsSeenNameSess(testAmbientSession, []string{""}, "g_a[0]") {
		t.Fatal("empty seen entry must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty seen entry IsSeenName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestItemizeArrayWithIV(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("create")
	}
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntType(), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.IVBounds = map[*Variable]int{iv: 0}
	// need dim 1 array with size > 0
	if len(av.Sizes) < 1 {
		av.Sizes = []int{4}
	}
	// one IV for one dim
	item := vs.ItemizeArray(NewRngSess(testAmbientSession, 2), cg, av)
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
	out := item.OutputAccessSess(testAmbientSession)
	if !strings.Contains(out, "g_a[") {
		t.Fatal(out)
	}
}

func TestItemizeArrayTooFewIV(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", IsArray: true, Type: GetIntType()},
		Sizes:    []int{2, 3},
	}
	av.AsArray = av
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// only one IV for 2 dims
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntType(), false, false)
	cg.IVBounds = map[*Variable]int{iv: 0}
	if vs.ItemizeArray(NewRngSess(testAmbientSession, 1), cg, av) != nil {
		t.Fatal("expect nil")
	}
}

func TestOutputCallChain(t *testing.T) {
	f := &Function{Name: "func_1"}
	b1 := &Block{Func: f}
	b2 := &Block{Func: f}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.CallChain = []*Block{b1, b2}
	s := cg.OutputCallChain()
	if !strings.Contains(s, "func_1") || !strings.Contains(s, " -> ") {
		t.Fatal(s)
	}
	// incomplete frame sticky — no invent "?" / blank " in "
	ClearErrorSess(testAmbientSession)
	cg.CallChain = []*Block{b1, {Func: nil}}
	if out := cg.OutputCallChain(); out != "" {
		t.Fatal("nil Func must fail closed call chain", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Func call chain must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg.CallChain = []*Block{{Func: &Function{Name: ""}}}
	if out := cg.OutputCallChain(); out != "" {
		t.Fatal("empty func name must fail closed call chain", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty func name call chain must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariableSelectorDoFinalization(t *testing.T) {
	vs := NewVariableSelector(testAmbientSession, Defaults())
	vs.AllVars = []*Variable{CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)}
	vs.GlobalList = vs.AllVars
	vs.DoFinalization()
	if len(vs.AllVars) != 0 || len(vs.GlobalList) != 0 {
		t.Fatal("cleared")
	}
	// VariableSelector always live; sticky no invent soft-skip finalization past hole
	ClearErrorSess(testAmbientSession)
	(*VariableSelector)(nil).DoFinalization()
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil DoFinalization must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMatchVarNameNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).MatchVarNameSess(testAmbientSession, "g_1") != nil {
		t.Fatal("nil Variable MatchVarName must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable MatchVarName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty query soft miss
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	if v.MatchVarNameSess(testAmbientSession, "") != nil {
		t.Fatal("empty name MatchVarName must soft miss")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty name MatchVarName must stay non-sticky soft miss")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was bare-name OutputC match on array path
	// (Name identity is complete; query non-exact name hits array Output branch)
	shell := &Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if shell.MatchVarNameSess(testAmbientSession, "g_b[0]") != nil {
		t.Fatal("IsArray without AsArray array-output MatchVarName must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray MatchVarName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
