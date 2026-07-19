package csmith

import (
	"strings"
	"testing"
)

func TestCreateFieldVars(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp GenerateSimpleTypes before make_random_struct_type / field choose
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(2), opts, probs, env, "S0")
	v := CreateVariableQfer("g_1", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if v == nil || len(v.FieldVars) != len(st.Fields) {
		t.Fatalf("fields %d want %d", len(v.FieldVars), len(st.Fields))
	}
	if v.FieldVars[0].Name != "g_1.f0" {
		t.Fatal(v.FieldVars[0].Name)
	}
	if v.FieldVars[0].FieldVarOf != v {
		t.Fatal("parent")
	}
	// nested expand
	for _, f := range v.FieldVars {
		if f.Type != nil && f.Type.IsStruct() && len(f.FieldVars) == 0 {
			t.Fatal("nested not expanded")
		}
	}
}

func TestCreateFieldVarsFailIncompleteNotEmptyComplete(t *testing.T) {
	// soft invent: fail() wiped FieldVars=nil → FieldVarsComplete invents zero fields
	// while Type.Fields still non-empty; fair: IncompleteVariables + sticky ERROR
	st := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1}, // incomplete field type
	}}
	v := &Variable{Name: "g_bad", Type: st, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	ClearError()
	v.CreateFieldVars()
	if v.FieldVarsComplete() {
		t.Fatal("CreateFieldVars fail must IncompleteVariables, not empty-complete nil")
	}
	if VariablesComplete(v.FieldVars) {
		t.Fatal("want IncompleteVariables marker")
	}
	if !HasError() {
		t.Fatal("CreateFieldVars fail must SetError sticky")
	}
	ClearError()
}

func TestHasFieldVarNilHole(t *testing.T) {
	parent := &Variable{Name: "s", Type: &Type{isStruct: true}}
	child := &Variable{Name: "s.f0", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{nil, child}
	if parent.FieldVarsComplete() {
		t.Fatal("FieldVars hole must be incomplete")
	}
	if parent.HasFieldVar(child) {
		t.Fatal("nil FieldVars hole must fail closed (no invent skip to later)")
	}
	// MarkDeadVar / OOS must not invent leave field pointees live past hole
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, child)}
	UpdateFactsForOOSVars([]*Variable{parent}, &facts)
	if FactsComplete(facts) {
		t.Fatal("OOS incomplete FieldVars must clear facts, not invent live pointee", facts)
	}
	if !HasError() {
		t.Fatal("OOS incomplete FieldVars must SetError sticky")
	}
	ClearError()
}

func TestFindReachableFrameVarsIncompleteStackFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsFrameVar false → empty frame set as complete
	ClearError()
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	facts := []*FactPointTo{MakeFactPointTo(p, loc)}
	if VariablesComplete(cg.FindReachableFrameVars(facts)) {
		t.Fatal("incomplete frame stack must fail closed incomplete, not invent empty complete")
	}
	if !HasError() {
		t.Fatal("incomplete frame stack must SetError sticky")
	}
	ClearError()
}

func TestCollectExpandable(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(5), opts, probs, env, "S0")
	v := CreateVariableQfer("g_2", st, NewCVQualifiers([]bool{false}, []bool{false}))
	all := v.CollectExpandable()
	if len(all) < 1+len(st.Fields) {
		t.Fatal(len(all))
	}
	// nil FieldVars hole fails closed sticky incomplete (not bare nil invent empty complete)
	v.FieldVars = append(v.FieldVars, nil)
	if VariablesComplete(v.CollectExpandable()) {
		t.Fatal("nil field hole must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil field hole must SetError sticky")
	}
	ClearError()
}

func TestFieldVolatileOrFromParent(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(2), opts, probs, env, "S0")
	// parent volatile
	v := CreateVariableQfer("g_3", st, NewCVQualifiers([]bool{false}, []bool{true}))
	for _, f := range v.FieldVars {
		if !f.IsVolatile() {
			t.Fatalf("%s should inherit volatile", f.Name)
		}
	}
}

func TestGenerateCanUseStructField(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, ".f0") || strings.Contains(out, ".f1") {
			found = true
			break
		}
	}
	if !found {
		t.Log("struct field names may appear only when used as expr; decls use brace init")
	}
}
