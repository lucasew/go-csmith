package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomStructType(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	// Type.cpp GenerateSimpleTypes before make_random_struct_type
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	r := NewRng(2)
	st := MakeRandomStructType(r, opts, probs, &env, "S0")
	if st == nil || !st.IsStruct() || len(st.Fields) < 1 {
		t.Fatal(st)
	}
	if len(env.StructTypes) != 1 {
		t.Fatal(env.StructTypes)
	}
	for _, f := range st.Fields {
		if f.Type == nil {
			t.Fatal("no nil-type field invent")
		}
	}
	decl := st.OutputStructDecl()
	if !strings.Contains(decl, "struct S0") || !strings.Contains(decl, "f0") {
		t.Fatal(decl)
	}
}

func TestGenerateAllTypesEnvCreatesStructs(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	GenerateAllTypesEnv(NewRng(2), opts, probs, &env)
	if len(env.StructTypes) < 1 {
		t.Fatal("expected structs under MoreTypesProbability")
	}
}

func TestGenerateEmitsStructDecl(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "STRUCT TYPES") && !strings.Contains(out, "struct S") {
		// may still generate
		t.Log(out[:min(500, len(out))])
	}
	if !strings.Contains(out, "struct S") {
		t.Fatal("expected struct S* in output")
	}
}

func TestMakeStructConstant(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(3), opts, probs, &env, "S0")
	if st == nil {
		t.Fatal("struct")
	}
	c := MakeStructConstant(NewRng(4), opts, probs, st)
	if c == nil || !strings.HasPrefix(c.Value, "{") {
		t.Fatal(c)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
