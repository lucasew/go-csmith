package csmith

import (
	"strings"
	"testing"
)

func TestNewCtrlVarsNoInventDimOne(t *testing.T) {
	// Variable.cpp:755 — for i < max_array_dimensions(); no invent maxDim=1 when 0
	CtrlVarsDoFinalization()
	ctrl := NewCtrlVars(0, false)
	if len(ctrl) != 0 {
		t.Fatalf("maxDim 0 must yield empty ctrl, got %d", len(ctrl))
	}
	// OutputArrayInitializers must not invent inits without matching ctrl decl
	opts := Defaults()
	opts.MaxArrayDim = 0
	v := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if out := OutputArrayInitializers([]*Variable{v}, opts, "    "); out != "" {
		t.Fatal("must fail closed without ctrl decl", out)
	}
	CtrlVarsDoFinalization()
}

func TestNewCtrlVarsLetters(t *testing.T) {
	CtrlVarsDoFinalization()
	opts := Defaults()
	c1 := GetNewCtrlVars(opts)
	if len(c1) != opts.MaxArrayDim {
		t.Fatalf("len %d", len(c1))
	}
	if c1[0].Name != "i" || c1[1].Name != "j" || c1[2].Name != "k" {
		t.Fatalf("%v %v %v", c1[0].Name, c1[1].Name, c1[2].Name)
	}
	if GetLastCtrlVars()[0].Name != "i" {
		t.Fatal("last")
	}
	opts.FreshArrayCtrlVarNames = true
	c2 := GetNewCtrlVars(opts)
	if c2[0].Name != "i1" {
		t.Fatalf("fresh got %q", c2[0].Name)
	}
	decl := OutputArrayCtrlVars(c1, 2, "    ")
	if decl != "    int i, j;\n" {
		t.Fatal(decl)
	}
	// Variable.cpp:806 — ctrl_vars[i] always live; no invent empty names
	broken := []*Variable{c1[0], nil}
	if out := OutputArrayCtrlVars(broken, 2, ""); out != "" {
		t.Fatal("nil ctrl slot must fail closed", out)
	}
	emptyName := []*Variable{c1[0], {Name: "", Type: GetIntType()}}
	if out := OutputArrayCtrlVars(emptyName, 2, ""); out != "" {
		t.Fatal("empty ctrl name must fail closed", out)
	}
	CtrlVarsDoFinalization()
	if GetLastCtrlVars() != nil {
		t.Fatal("cleared")
	}
	// incomplete last vector must IncompleteVariables (not bare nil empty-complete)
	_ = NewCtrlVars(2, false)
	ctrlVarsVectors[len(ctrlVarsVectors)-1] = append(ctrlVarsVectors[len(ctrlVarsVectors)-1], nil)
	if VariablesComplete(GetLastCtrlVars()) {
		t.Fatal("incomplete last ctrl must IncompleteVariables")
	}
	CtrlVarsDoFinalization()
}

func TestOutputLowerBoundArray(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}},
		Sizes:    []int{2, 3},
	}
	if av.OutputLowerBound() != "g_a[0][0]" {
		t.Fatal(av.OutputLowerBound())
	}
	if av.OutputWithIndices([]string{"i", "j"}) != "g_a[i][j]" {
		t.Fatal(av.OutputWithIndices([]string{"i", "j"}))
	}
}

func TestOutputUpperBoundField(t *testing.T) {
	parent := CreateVariableScalars("g_s", GetIntType(), false, false)
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	if field.OutputUpperBound(false) != "g_s.f0" {
		t.Fatal(field.OutputUpperBound(false))
	}
}

func TestOutputForComment(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if v.OutputForComment(false) != "g_1" {
		t.Fatal(v.OutputForComment(false))
	}
}

func TestOutputArrayInitializersCtrlDecl(t *testing.T) {
	CtrlVarsDoFinalization()
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_1", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("nil")
	}
	v := &av.Variable
	v.AsArray = av
	out := OutputArrayInitializers([]*Variable{v}, opts, "    ")
	// GetMaxArrayDimension from sizes; declare letter names up to dimen
	if !strings.Contains(out, "int i") {
		t.Fatal(out)
	}
	// nil hole fails closed sticky
	ClearError()
	if s := OutputArrayInitializers([]*Variable{v, nil}, opts, "    "); s != "" {
		t.Fatal("nil hole must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearError()
	// incomplete loop-init (local non-const without init) fails closed
	blk := &Block{}
	loc := &ArrayVariable{
		Variable: Variable{Name: "l_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
		Block:    blk, // non-nil → not IsGlobal; needs loop init
		// no Init / InitValues → OutputInit empty
	}
	loc.AsArray = loc
	if s := OutputArrayInitializers([]*Variable{&loc.Variable}, opts, "    "); s != "" {
		t.Fatal("incomplete array init must fail closed", s)
	}
}
