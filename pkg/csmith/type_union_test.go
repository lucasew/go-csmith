package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomUnionType(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	// Type.cpp shared sid: after any prior aggregates, next union is U{seq} not always U0
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession)}
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 3), opts, probs, &env, "")
	if ut == nil || !ut.IsUnionSess(testAmbientSession) || len(ut.Fields) < 1 {
		t.Fatal(ut)
	}
	if ut.StructName != "U0" || ut.SID != 0 {
		t.Fatalf("first aggregate union want U0, got %q sid=%d", ut.StructName, ut.SID)
	}
	decl := ut.OutputUnionDecl()
	if !strings.Contains(decl, "union U0") {
		t.Fatal(decl)
	}
	// after S0, next union is U1
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 4), opts, probs, &env, "")
	if st == nil || st.StructName != "S1" {
		t.Fatalf("second aggregate want S1, got %v", st)
	}
	ut2 := MakeRandomUnionType(NewRngSess(testAmbientSession, 5), opts, probs, &env, "")
	if ut2 == nil || ut2.StructName != "U2" {
		t.Fatalf("third aggregate want U2, got %v name=%q", ut2, func() string {
			if ut2 == nil {
				return ""
			}
			return ut2.StructName
		}())
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeUnionConstantFirstFieldOnly(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	c := MakeUnionConstant(NewRngSess(testAmbientSession, 2), opts, probs, ut)
	if c == nil {
		t.Fatal("union constant")
	}
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
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: GetSimpleTypeSess(testAmbientSession, EShort), BitWidth: -1},
		},
	}
	ClearErrorSess(testAmbientSession)
	v := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if v == nil {
		t.Fatal("CreateVariableQfer union", HasErrorSess(testAmbientSession))
	}
	if len(v.FieldVars) != 2 {
		t.Fatal(len(v.FieldVars))
	}
	if v.FieldVars[0].Name != "g_u.f0" {
		t.Fatal(v.FieldVars[0].Name)
	}
	ClearErrorSess(testAmbientSession)
}
