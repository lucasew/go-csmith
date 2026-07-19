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
	// (MaxArrayDim undersize is config soft re-pick — non-sticky empty)
	ClearError()
	opts := Defaults()
	opts.MaxArrayDim = 0
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	av.AsArray = av
	if out := OutputArrayInitializers([]*Variable{&av.Variable}, opts, "    "); out != "" {
		t.Fatal("must fail closed without ctrl decl", out)
	}
	if HasError() {
		t.Fatal("MaxArrayDim undersize must stay non-sticky config soft re-pick")
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
	// Variable.cpp:806 — ctrl_vars[i] always live; nil fails closed sticky
	broken := []*Variable{c1[0], nil}
	ClearError()
	if out := OutputArrayCtrlVars(broken, 2, ""); out != "" {
		t.Fatal("nil ctrl slot must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil ctrl slot must SetError sticky")
	}
	ClearError()
	emptyName := []*Variable{c1[0], {Name: "", Type: GetIntType()}}
	if out := OutputArrayCtrlVars(emptyName, 2, ""); out != "" {
		t.Fatal("empty ctrl name must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty ctrl name must SetError sticky")
	}
	ClearError()
	// Variable.cpp:802 assert undersize — config soft re-pick non-sticky empty
	// (sticky poisons Generate when array rank exceeds MaxArrayDim)
	short := []*Variable{c1[0]}
	if out := OutputArrayCtrlVars(short, 2, ""); out != "" {
		t.Fatal("dimen > len(ctrl) must fail closed", out)
	}
	if HasError() {
		t.Fatal("dimen > len(ctrl) must stay non-sticky config soft re-pick")
	}
	CtrlVarsDoFinalization()
	if GetLastCtrlVars() != nil {
		t.Fatal("cleared")
	}
	// incomplete last vector must IncompleteVariables sticky (not bare nil empty-complete)
	ClearError()
	_ = NewCtrlVars(2, false)
	ctrlVarsVectors[len(ctrlVarsVectors)-1] = append(ctrlVarsVectors[len(ctrlVarsVectors)-1], nil)
	if VariablesComplete(GetLastCtrlVars()) {
		t.Fatal("incomplete last ctrl must IncompleteVariables")
	}
	if !HasError() {
		t.Fatal("incomplete last ctrl must SetError sticky")
	}
	ClearError()
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
	// nil hole fails closed sticky (GetMaxArrayDimension + OutputArrayInitializers)
	ClearError()
	if GetMaxArrayDimension([]*Variable{v, nil}) >= 0 {
		t.Fatal("nil hole GetMaxArrayDimension must fail closed -1")
	}
	if !HasError() {
		t.Fatal("nil hole GetMaxArrayDimension must SetError sticky")
	}
	ClearError()
	if s := OutputArrayInitializers([]*Variable{v, nil}, opts, "    "); s != "" {
		t.Fatal("nil hole must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearError()
	// incomplete loop-init (local non-const without init) fails closed sticky
	ClearError()
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
	if !HasError() {
		t.Fatal("incomplete loop-init OutputArrayInitializers must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was synthetic ArrayVariable shell from
	// ArraySizes then partial loop inits. Fair: sticky empty whole section.
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if GetMaxArrayDimension([]*Variable{shell}) >= 0 {
		t.Fatal("IsArray without AsArray GetMaxArrayDimension must fail closed -1")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray GetMaxArrayDimension must SetError sticky")
	}
	ClearError()
	if s := OutputArrayInitializers([]*Variable{shell}, opts, "    "); s != "" {
		t.Fatal("IsArray without AsArray must fail closed empty, not invent synthetic", s)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputArrayInitializers must SetError sticky")
	}
	ClearError()
}

func TestOutputForCommentNilSticky(t *testing.T) {
	ClearError()
	if (*Variable)(nil).OutputForComment(false) != "" {
		t.Fatal("nil Variable OutputForComment must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Variable OutputForComment must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).GetActualName(false) != "" {
		t.Fatal("nil Variable GetActualName must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Variable GetActualName must SetError sticky")
	}
	ClearError()
}
