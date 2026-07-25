package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomSignatureSetsAlias(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := MakeRandomSignature(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, &vs.Sym, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, nil)
	if f == nil || f.AliasName != f.Name+"_alias" {
		t.Fatalf("%+v", f)
	}
}

func TestOutputForwardDeclAlias(t *testing.T) {
	f := &Function{
		Name: "func_1", AliasName: "func_1_alias",
		ReturnType: GetIntTypeSess(testAmbientSession),
		RV:         CreateVariableQferSess(testAmbientSession, "func_1_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})),
	}
	out := f.OutputForwardDeclAliasSess(testAmbientSession, true)
	if !strings.Contains(out, "static ") ||
		!strings.Contains(out, "func_1_alias") ||
		!strings.Contains(out, `__attribute__((alias("func_1")))`) {
		t.Fatal(out)
	}
}

func TestOutputFunctionsEmitsAliasDecls(t *testing.T) {
	opts := Defaults()
	opts.FunctionAttributes = true
	g := NewProgramGenerator(NewSession(opts))
	g.Funcs.Funcs = []*Function{{
		Name: "func_1", AliasName: "func_1_alias",
		ReturnType: GetIntTypeSess(testAmbientSession),
		IsBuilt:    true, BuildState: BuildBuilt,
		RV:   CreateVariableQferSess(testAmbientSession, "func_1_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})),
		Body: &Block{},
	}}
	out := g.OutputFunctions()
	if !strings.Contains(out, "FORWARD ALIAS DECLARATIONS") ||
		!strings.Contains(out, "func_1_alias") {
		t.Fatal(out)
	}
}

func TestMakeOneStructFieldRespectsMaxNest(t *testing.T) {
	opts := Defaults()
	opts.MaxNestedStructLevel = 1
	probs := NewProbabilities(opts)
	// deep struct: S with nested already at depth 1+
	// StructDepth of plain S0 with no nested fields is 1 → >= max 1 → rejected
	env := &TypeEnv{Sess: testAmbientSession}
	deep := &Type{isStruct: true, StructName: "Sdeep", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	// depth 1 — AllTypes is ChooseRandomTypeFilter pool (Type.cpp:687)
	env.StructTypes = []*Type{deep}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), deep}
	// force nested pick: many trials with seed that might pick nest — with max 1, all rejected → simple
	for seed := uint64(1); seed < 40; seed++ {
		f := MakeOneStructField(NewRngSess(testAmbientSession, seed), opts, probs, env, 0)
		if f.Type != nil && f.Type.IsStructSess(testAmbientSession) {
			t.Fatalf("nested not allowed at max=1 got %s depth %d", f.Type.StructName, f.Type.StructDepthSess(testAmbientSession))
		}
	}
	// max=2 allows depth-1 structs
	opts.MaxNestedStructLevel = 2
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		f := MakeOneStructField(NewRngSess(testAmbientSession, seed), opts, probs, env, 0)
		if f.Type != nil && f.Type.IsStructSess(testAmbientSession) && f.Type.StructName == "Sdeep" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected some nest when max=2")
	}
}
