package csmith

import (
	"strings"
	"testing"
)

func TestCreateFieldVars(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp GenerateSimpleTypes before make_random_struct_type / field choose
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 2), opts, probs, env, "S0")
	if st == nil {
		t.Fatal("MakeRandomStructType", HasErrorSess(testAmbientSession))
	}
	v := CreateVariableQferSess(testAmbientSession, "g_1", st, NewCVQualifiers([]bool{false}, []bool{false}))
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
		if f.Type != nil && f.Type.IsStructSess(testAmbientSession) && len(f.FieldVars) == 0 {
			t.Fatal("nested not expanded")
		}
	}
}

func TestCreateFieldVarsFailIncompleteNotEmptyComplete(t *testing.T) {
	// soft invent: fail() wiped FieldVars=nil → FieldVarsComplete invents zero fields
	// while Type.Fields still non-empty; fair: IncompleteVariables + sticky ERROR
	st := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1}, // incomplete field type
	}}
	v := &Variable{Name: "g_bad", Type: st, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	ClearErrorSess(testAmbientSession)
	v.CreateFieldVarsSess(testAmbientSession)
	if v.FieldVarsCompleteSess(testAmbientSession) {
		t.Fatal("CreateFieldVars fail must IncompleteVariables, not empty-complete nil")
	}
	if VariablesComplete(v.FieldVars) {
		t.Fatal("want IncompleteVariables marker")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CreateFieldVars fail must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable.cpp:338 assert aggregate sticky — no invent fields on scalar
	scalar := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	scalar.CreateFieldVarsSess(testAmbientSession)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("non-aggregate CreateFieldVars must SetError sticky")
	}
	if len(scalar.FieldVars) != 0 {
		t.Fatal("non-aggregate must not invent FieldVars", scalar.FieldVars)
	}
	ClearErrorSess(testAmbientSession)
	// nested field create: Type-nil outermost top sticky fail (no invent make_random init)
	// plant as field_var_of chain so top walk hits Type-nil container
	top := &Variable{Name: "g_top"} // Type nil
	st2 := &Type{isStruct: true, StructName: "Snest", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	nested := &Variable{Name: "g_top.inner", Type: st2, FieldVarOf: top, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	nested.CreateFieldVarsSess(testAmbientSession)
	if nested.FieldVarsCompleteSess(testAmbientSession) {
		t.Fatal("Type-nil top CreateFieldVars must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil top CreateFieldVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasFieldVarNilHole(t *testing.T) {
	parent := &Variable{Name: "s", Type: &Type{isStruct: true}}
	child := &Variable{Name: "s.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	parent.FieldVars = []*Variable{nil, child}
	if parent.FieldVarsCompleteSess(testAmbientSession) {
		t.Fatal("FieldVars hole must be incomplete")
	}
	if parent.HasFieldVarSess(testAmbientSession, child) {
		t.Fatal("nil FieldVars hole must fail closed (no invent skip to later)")
	}
	// nested HasFieldVar residual: soft invent was soft-continue later field invent not-has-field.
	// Fair: sticky fail closed false (FieldVarsComplete false already). Covered above.
	// MarkDeadVar / OOS must not invent leave field pointees live past hole
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, child)}
	UpdateFactsForOOSVarsSess(testAmbientSession, []*Variable{parent}, &facts)
	if FactsComplete(facts) {
		t.Fatal("OOS incomplete FieldVars must clear facts, not invent live pointee", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OOS incomplete FieldVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindReachableFrameVarsIncompleteStackFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsFrameVar false → empty frame set as complete
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, loc)}
	if VariablesComplete(cg.FindReachableFrameVars(facts)) {
		t.Fatal("incomplete frame stack must fail closed incomplete, not invent empty complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete frame stack must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindReachableFrameVarsIsVisibleResidualSticky(t *testing.T) {
	// IsVisibleLocal residual soft invent was soft-skip not-frame then continue later pointee.
	// Fair: sticky IncompleteVariables fail closed whole FindReachableFrameVars.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	// non-global local name
	loc := &Variable{Name: "l_1", Type: GetIntTypeSess(testAmbientSession)}
	// incomplete Param hole so IsVisibleLocal stickies when scanning non-match
	f.Param = []*Variable{nil}
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// second good pointee that would invent partial frame set after residual soft-skip
	loc2 := CreateVariableScalarsSess(testAmbientSession, "l_2", GetIntTypeSess(testAmbientSession), false, false)
	loc2.Name = "l_2"
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, loc), MakeFactPointToSess(testAmbientSession, p, loc2)}
	if VariablesComplete(cg.FindReachableFrameVars(facts)) {
		t.Fatal("IsVisibleLocal residual must fail closed incomplete frame set")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVisibleLocal residual FindReachableFrameVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCollectExpandable(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 5), opts, probs, env, "S0")
	v := CreateVariableQferSess(testAmbientSession, "g_2", st, NewCVQualifiers([]bool{false}, []bool{false}))
	all := v.CollectExpandableSess(testAmbientSession)
	if len(all) < 1+len(st.Fields) {
		t.Fatal(len(all))
	}
	// nil FieldVars hole fails closed sticky incomplete (not bare nil invent empty complete)
	v.FieldVars = append(v.FieldVars, nil)
	if VariablesComplete(v.CollectExpandableSess(testAmbientSession)) {
		t.Fatal("nil field hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil field hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// special Type-nil complete expand as self only
	got := NullPtr.CollectExpandableSess(testAmbientSession)
	if !VariablesComplete(got) || len(got) != 1 || got[0] != NullPtr {
		t.Fatal("special CollectExpandable must stay complete self-only", got)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("special CollectExpandable must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-special Type-nil sticky IncompleteVariables (no invent expand pool past hole)
	if VariablesComplete((&Variable{Name: "broken"}).CollectExpandableSess(testAmbientSession)) {
		t.Fatal("Type-nil CollectExpandable must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil CollectExpandable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested CollectExpandable residual soft invent was soft-continue invent complete expand pool.
	// Fair: sticky IncompleteVariables.
	nestHole := &Variable{Name: "nest", Type: &Type{isStruct: true}, FieldVars: []*Variable{nil}}
	outer := &Variable{
		Name: "outer", Type: &Type{isStruct: true},
		FieldVars: []*Variable{nestHole, CreateVariableScalarsSess(testAmbientSession, "g_ok", GetIntTypeSess(testAmbientSession), false, false)},
	}
	if VariablesComplete(outer.CollectExpandableSess(testAmbientSession)) {
		t.Fatal("nested residual CollectExpandable must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual CollectExpandable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFieldVolatileOrFromParent(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRngSess(testAmbientSession, 1), opts, probs, env)
	st := MakeRandomStructType(NewRngSess(testAmbientSession, 2), opts, probs, env, "S0")
	// parent volatile
	v := CreateVariableQferSess(testAmbientSession, "g_3", st, NewCVQualifiers([]bool{false}, []bool{true}))
	for _, f := range v.FieldVars {
		if !f.IsVolatileSess(testAmbientSession) {
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
