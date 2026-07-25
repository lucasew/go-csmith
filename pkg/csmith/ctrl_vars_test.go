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
	ClearErrorSess(testAmbientSession)
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
	if HasErrorSess(testAmbientSession) {
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
	ClearErrorSess(testAmbientSession)
	if out := OutputArrayCtrlVars(broken, 2, ""); out != "" {
		t.Fatal("nil ctrl slot must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ctrl slot must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	emptyName := []*Variable{c1[0], {Name: "", Type: GetIntType()}}
	if out := OutputArrayCtrlVars(emptyName, 2, ""); out != "" {
		t.Fatal("empty ctrl name must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty ctrl name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable.cpp:802 assert undersize — config soft re-pick non-sticky empty
	// (sticky poisons Generate when array rank exceeds MaxArrayDim)
	short := []*Variable{c1[0]}
	if out := OutputArrayCtrlVars(short, 2, ""); out != "" {
		t.Fatal("dimen > len(ctrl) must fail closed", out)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("dimen > len(ctrl) must stay non-sticky config soft re-pick")
	}
	CtrlVarsDoFinalization()
	if GetLastCtrlVars() != nil {
		t.Fatal("cleared")
	}
	// incomplete last vector must IncompleteVariables sticky (not bare nil empty-complete)
	ClearErrorSess(testAmbientSession)
	_ = NewCtrlVars(2, false)
	currentSession().CtrlVarsVectors[len(currentSession().CtrlVarsVectors)-1] = append(currentSession().CtrlVarsVectors[len(currentSession().CtrlVarsVectors)-1], nil)
	if VariablesComplete(GetLastCtrlVars()) {
		t.Fatal("incomplete last ctrl must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete last ctrl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
	if GetMaxArrayDimension([]*Variable{v, nil}) >= 0 {
		t.Fatal("nil hole GetMaxArrayDimension must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole GetMaxArrayDimension must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := OutputArrayInitializers([]*Variable{v, nil}, opts, "    "); s != "" {
		t.Fatal("nil hole must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete loop-init (local non-const without init) fails closed sticky
	ClearErrorSess(testAmbientSession)
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
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete loop-init OutputArrayInitializers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was synthetic ArrayVariable shell from
	// ArraySizes then partial loop inits. Fair: sticky empty whole section.
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if GetMaxArrayDimension([]*Variable{shell}) >= 0 {
		t.Fatal("IsArray without AsArray GetMaxArrayDimension must fail closed -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray GetMaxArrayDimension must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := OutputArrayInitializers([]*Variable{shell}, opts, "    "); s != "" {
		t.Fatal("IsArray without AsArray must fail closed empty, not invent synthetic", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputArrayInitializers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// NoLoopInitializer residual: Type-nil stickies true; soft invent was soft-skip then
	// partial inits for later arrays. Fair: sticky fail closed whole section.
	// Need live AsArray with Type-nil after GetMaxArrayDimension validation path.
	// Use Type-nil via NoLoopInitializer on live array that has Type then clear — not possible mid-loop.
	// Use IsConst residual via nil Qfer unpaired? IsConst only stickies nil Variable.
	// GetMaxArrayDimension requires complete vars + live AsArray; NoLoopInitializer Type-nil stickies.
	avBroken := &ArrayVariable{
		Variable: Variable{Name: "g_b", Type: nil, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	avBroken.AsArray = avBroken
	// GetMaxArrayDimension uses AsArray.Sizes — Type nil still dims from sizes
	// NoLoopInitializer(av) Type nil stickies residual true → soft-skip invent next
	avGood := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_c", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if avGood == nil {
		t.Fatal("good array")
	}
	if s := OutputArrayInitializers([]*Variable{&avBroken.Variable, &avGood.Variable}, opts, "    "); s != "" {
		t.Fatal("NoLoopInitializer residual must fail closed empty", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("NoLoopInitializer residual OutputArrayInitializers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestOutputArrayInitializersBraceOnlyStillCtrlDecl — Variable.cpp:829–841 + 861–863.
// Even when every local array is brace-init (no_loop_initializer true), C++ still
// allocates get_new_ctrl_vars and emits OutputArrayCtrlVars whenever dimen > 0.
// Seed-2 func_67: "int i, j, k;" after l_137 brace init with no loop inits.
func TestOutputArrayInitializersBraceOnlyStillCtrlDecl(t *testing.T) {
	CtrlVarsDoFinalization()
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	// local array with alt init_values → NoLoopInitializer true (brace path)
	av := &ArrayVariable{
		Variable: Variable{
			Name:       "l_137",
			Type:       GetIntType(),
			IsArray:    true,
			ArraySizes: []int{2, 3},
			Init:       MakeInt(0),
		},
		Sizes:      []int{2, 3},
		InitValues: []string{"0", "1"},
		Block:      &Block{}, // non-nil → not global
	}
	av.AsArray = av
	if !av.NoLoopInitializer() {
		t.Fatal("brace-init local must NoLoopInitializer")
	}
	out := OutputArrayInitializers([]*Variable{&av.Variable}, opts, "    ")
	if HasErrorSess(testAmbientSession) {
		t.Fatal("brace-only OutputArrayInitializers must not sticky", GetErrorSess(testAmbientSession))
	}
	// still declare ctrl vars for dimen=2
	if !strings.Contains(out, "int i, j;") {
		t.Fatalf("want ctrl decl for brace-only arrays, got %q", out)
	}
	// no loop body for brace-init
	if strings.Contains(out, "for (") {
		t.Fatal("brace-only must not emit loop init", out)
	}
	// OutputVariableList for locals must chain OutputArrayInitializers
	// Variable.cpp:861–863
	CtrlVarsDoFinalization()
	ClearErrorSess(testAmbientSession)
	list := OutputVariableList([]*Variable{&av.Variable}, "  ", false)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if !strings.Contains(list, "l_137") || !strings.Contains(list, "int i, j;") {
		t.Fatalf("OutputVariableList locals must emit brace def + ctrl decl, got %q", list)
	}
	// Block.Output uses OutputVariableList — same contract
	CtrlVarsDoFinalization()
	ClearErrorSess(testAmbientSession)
	blk := &Block{LocalVars: []*Variable{&av.Variable}, StmID: 1}
	bout := blk.Output(0)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	if !strings.Contains(bout, "int i, j;") {
		t.Fatalf("Block.Output brace-only local array must emit ctrl decl, got %q", bout)
	}
	CtrlVarsDoFinalization()
}

func TestOutputForCommentNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).OutputForComment(false) != "" {
		t.Fatal("nil Variable OutputForComment must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable OutputForComment must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).GetActualName(false) != "" {
		t.Fatal("nil Variable GetActualName must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable GetActualName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
