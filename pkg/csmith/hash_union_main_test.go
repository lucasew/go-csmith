package csmith

import (
	"strings"
	"testing"
)

func TestHashGlobalVariablesWithUnionFacts(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	vs := NewVariableSelector(Defaults())
	vs.GlobalList = []*Variable{uv}
	// nil facts → all fields
	all := HashGlobalVariables(vs)
	if !strings.Contains(all, "g_u.f0") || !strings.Contains(all, "g_u.f1") {
		t.Fatal(all)
	}
	// only field 0 readable
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := HashGlobalVariablesWithUnionFacts(vs, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal(out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("must skip f1", out)
	}
}

func TestProgramGeneratorHashGlobalsUsesFM(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	g := NewProgramGenerator(opts)
	g.GenerateAllTypes()
	// seed a union global
	ut := &Type{
		isUnion: true, StructName: "U1",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	g.VS.GlobalList = append(g.VS.GlobalList, uv)
	g.GenerateFunctions()
	// attach union fact on first func FM
	if len(g.Funcs.Funcs) > 0 {
		fm := g.FactMgrs.ForFunc(g.Funcs.Funcs[0])
		if fm != nil {
			fm.UnionFacts = []*FactUnion{MakeFactUnion(uv, 0)}
		}
	}
	out := g.hashGlobals()
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("unread field hashed", out)
	}
}

func TestMakeExpressionAssignSetsTypeAndFacts(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	e := MakeExpressionAssign(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType(), nil)
	if e == nil || e.Term != TermAssignment || e.Assign == nil {
		t.Fatal(e)
	}
	if e.ExprType == nil {
		t.Fatal("ExprType")
	}
	// Output is parenthesized assign
	s := e.Output()
	if !strings.Contains(s, "(") || !strings.Contains(s, ")") {
		t.Fatal(s)
	}
}
