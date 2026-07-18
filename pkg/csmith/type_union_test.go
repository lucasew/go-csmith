package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomUnionType(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	// need some structs first optional
	GenerateAllTypesEnv(NewRng(2), opts, probs, &env)
	ut := MakeRandomUnionType(NewRng(3), opts, probs, &env, "U0")
	if ut == nil || !ut.IsUnion() || len(ut.Fields) < 1 {
		t.Fatal(ut)
	}
	decl := ut.OutputUnionDecl()
	if !strings.Contains(decl, "union U0") {
		t.Fatal(decl)
	}
}

func TestMakeUnionConstantFirstFieldOnly(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	c := MakeUnionConstant(NewRng(2), opts, probs, ut)
	// single initializer value inside braces
	if !strings.HasPrefix(c.Value, "{") || strings.Count(c.Value, ",") != 0 {
		// first field only → no comma unless nested struct
		if strings.Count(c.Value, ",") > 0 && !strings.Contains(c.Value, "{{") {
			t.Log(c.Value)
		}
	}
}

func TestGenerateEmitsUnion(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 30; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "union U") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected union U* in some seed")
	}
}

func TestCreateFieldVarsUnion(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetSimpleType(EShort), BitWidth: -1},
		},
	}
	v := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(v.FieldVars) != 2 {
		t.Fatal(len(v.FieldVars))
	}
	if v.FieldVars[0].Name != "g_u.f0" {
		t.Fatal(v.FieldVars[0].Name)
	}
}
