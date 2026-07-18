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

func TestCollectExpandable(t *testing.T) {
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
