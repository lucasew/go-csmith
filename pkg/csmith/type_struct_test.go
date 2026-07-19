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

func TestMakeRandomStructUnionTypeNilRNGSticky(t *testing.T) {
	// Type.cpp always has process RNG; sticky no invent aggregate type shells
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType()}
	if MakeRandomStructType(nil, opts, probs, &env, "S0") != nil {
		t.Fatal("nil RNG struct must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomStructType must SetError sticky")
	}
	ClearError()
	if MakeRandomUnionType(nil, opts, probs, &env, "U0") != nil {
		t.Fatal("nil RNG union must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeRandomUnionType must SetError sticky")
	}
	ClearError()
	opts.FixedStructFields = true
	opts.MaxStructFields = 0
	if MakeRandomStructType(NewRng(1), opts, probs, &env, "Sempty") != nil {
		t.Fatal("zero fields must fail closed")
	}
	if !HasError() {
		t.Fatal("zero-field MakeRandomStructType must SetError sticky")
	}
	ClearError()
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

func TestTypeGenNoInventNilRngOrProbs(t *testing.T) {
	// C++ always has RNG + Probabilities sticky; no invent fields/aggregates without them
	ClearError()
	opts := Defaults()
	if MoreTypesProbability(nil, NewProbabilities(opts), 20) {
		t.Fatal("nil RNG past threshold must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil RNG MoreTypesProbability must SetError sticky")
	}
	ClearError()
	if f := MakeOneStructField(nil, opts, NewProbabilities(opts), &TypeEnv{AllTypes: []*Type{GetIntType()}}, 0); f.Type != nil {
		t.Fatal("nil RNG MakeOneStructField must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeOneStructField must SetError sticky")
	}
	ClearError()
	if f := MakeOneStructField(NewRng(1), opts, nil, &TypeEnv{AllTypes: []*Type{GetIntType()}}, 0); f.Type != nil {
		t.Fatal("nil probs MakeOneStructField must fail closed")
	}
	if !HasError() {
		t.Fatal("nil probs MakeOneStructField must SetError sticky")
	}
	ClearError()
	if f := MakeOneUnionField(nil, opts, NewProbabilities(opts), &TypeEnv{AllTypes: []*Type{GetIntType()}}, 0); f.Type != nil {
		t.Fatal("nil RNG MakeOneUnionField must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeOneUnionField must SetError sticky")
	}
	ClearError()
	var env TypeEnv
	opts.Structs = true
	opts.Unions = true
	GenerateAllTypesEnv(nil, opts, NewProbabilities(opts), &env)
	if len(env.StructTypes) != 0 || len(env.UnionTypes) != 0 {
		t.Fatal("nil RNG must not invent aggregate types", len(env.StructTypes), len(env.UnionTypes))
	}
	if len(env.AllTypes) == 0 {
		t.Fatal("simples still seeded")
	}
	ClearError()
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
	// Constant.cpp always has RNG sticky; no invent "{}" aggregate shell
	ClearError()
	if MakeStructConstant(nil, opts, probs, st) != nil {
		t.Fatal("nil RNG struct constant")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeStructConstant must SetError sticky")
	}
	ClearError()
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	if MakeUnionConstant(nil, opts, probs, ut) != nil {
		t.Fatal("nil RNG union constant")
	}
	if !HasError() {
		t.Fatal("nil RNG MakeUnionConstant must SetError sticky")
	}
	ClearError()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
