package csmith

import (
	"strings"
	"testing"
)

func TestCreateFieldVars(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	// Type.cpp GenerateSimpleTypes before make_random_struct_type / field choose
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(2), opts, probs, env, "S0")
	if st == nil {
		t.Fatal("MakeRandomStructType", HasError())
	}
	v := CreateVariableQfer("g_1", st, NewCVQualifiers([]bool{false}, []bool{false}))
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
		if f.Type != nil && f.Type.IsStruct() && len(f.FieldVars) == 0 {
			t.Fatal("nested not expanded")
		}
	}
}

func TestCreateFieldVarsFailIncompleteNotEmptyComplete(t *testing.T) {
	// soft invent: fail() wiped FieldVars=nil → FieldVarsComplete invents zero fields
	// while Type.Fields still non-empty; fair: IncompleteVariables + sticky ERROR
	st := &Type{isStruct: true, StructName: "Sbad", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1}, // incomplete field type
	}}
	v := &Variable{Name: "g_bad", Type: st, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	ClearError()
	v.CreateFieldVars()
	if v.FieldVarsComplete() {
		t.Fatal("CreateFieldVars fail must IncompleteVariables, not empty-complete nil")
	}
	if VariablesComplete(v.FieldVars) {
		t.Fatal("want IncompleteVariables marker")
	}
	if !HasError() {
		t.Fatal("CreateFieldVars fail must SetError sticky")
	}
	ClearError()
	// Variable.cpp:338 assert aggregate sticky — no invent fields on scalar
	scalar := CreateVariableScalars("g_i", GetIntType(), false, false)
	scalar.CreateFieldVars()
	if !HasError() {
		t.Fatal("non-aggregate CreateFieldVars must SetError sticky")
	}
	if len(scalar.FieldVars) != 0 {
		t.Fatal("non-aggregate must not invent FieldVars", scalar.FieldVars)
	}
	ClearError()
	// nested field create: Type-nil outermost top sticky fail (no invent make_random init)
	// plant as field_var_of chain so top walk hits Type-nil container
	top := &Variable{Name: "g_top"} // Type nil
	st2 := &Type{isStruct: true, StructName: "Snest", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	nested := &Variable{Name: "g_top.inner", Type: st2, FieldVarOf: top, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	nested.CreateFieldVars()
	if nested.FieldVarsComplete() {
		t.Fatal("Type-nil top CreateFieldVars must IncompleteVariables")
	}
	if !HasError() {
		t.Fatal("Type-nil top CreateFieldVars must SetError sticky")
	}
	ClearError()
}

func TestHasFieldVarNilHole(t *testing.T) {
	parent := &Variable{Name: "s", Type: &Type{isStruct: true}}
	child := &Variable{Name: "s.f0", Type: GetIntType(), FieldVarOf: parent}
	parent.FieldVars = []*Variable{nil, child}
	if parent.FieldVarsComplete() {
		t.Fatal("FieldVars hole must be incomplete")
	}
	if parent.HasFieldVar(child) {
		t.Fatal("nil FieldVars hole must fail closed (no invent skip to later)")
	}
	// nested HasFieldVar residual: soft invent was soft-continue later field invent not-has-field.
	// Fair: sticky fail closed false (FieldVarsComplete false already). Covered above.
	// MarkDeadVar / OOS must not invent leave field pointees live past hole
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, child)}
	UpdateFactsForOOSVars([]*Variable{parent}, &facts)
	if FactsComplete(facts) {
		t.Fatal("OOS incomplete FieldVars must clear facts, not invent live pointee", facts)
	}
	if !HasError() {
		t.Fatal("OOS incomplete FieldVars must SetError sticky")
	}
	ClearError()
}

func TestFindReachableFrameVarsIncompleteStackFailClosed(t *testing.T) {
	// soft invent: LocalVars hole → IsFrameVar false → empty frame set as complete
	ClearError()
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	blk := &Block{Func: f, LocalVars: []*Variable{loc, nil}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	facts := []*FactPointTo{MakeFactPointTo(p, loc)}
	if VariablesComplete(cg.FindReachableFrameVars(facts)) {
		t.Fatal("incomplete frame stack must fail closed incomplete, not invent empty complete")
	}
	if !HasError() {
		t.Fatal("incomplete frame stack must SetError sticky")
	}
	ClearError()
}

func TestFindReachableFrameVarsIsVisibleResidualSticky(t *testing.T) {
	// IsVisibleLocal residual soft invent was soft-skip not-frame then continue later pointee.
	// Fair: sticky IncompleteVariables fail closed whole FindReachableFrameVars.
	ClearError()
	f := &Function{Name: "f"}
	// non-global local name
	loc := &Variable{Name: "l_1", Type: GetIntType()}
	// incomplete Param hole so IsVisibleLocal stickies when scanning non-match
	f.Param = []*Variable{nil}
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// second good pointee that would invent partial frame set after residual soft-skip
	loc2 := CreateVariableScalars("l_2", GetIntType(), false, false)
	loc2.Name = "l_2"
	facts := []*FactPointTo{MakeFactPointTo(p, loc), MakeFactPointTo(p, loc2)}
	if VariablesComplete(cg.FindReachableFrameVars(facts)) {
		t.Fatal("IsVisibleLocal residual must fail closed incomplete frame set")
	}
	if !HasError() {
		t.Fatal("IsVisibleLocal residual FindReachableFrameVars must SetError sticky")
	}
	ClearError()
}

func TestCollectExpandable(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(5), opts, probs, env, "S0")
	v := CreateVariableQfer("g_2", st, NewCVQualifiers([]bool{false}, []bool{false}))
	all := v.CollectExpandable()
	if len(all) < 1+len(st.Fields) {
		t.Fatal(len(all))
	}
	// nil FieldVars hole fails closed sticky incomplete (not bare nil invent empty complete)
	v.FieldVars = append(v.FieldVars, nil)
	if VariablesComplete(v.CollectExpandable()) {
		t.Fatal("nil field hole must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nil field hole must SetError sticky")
	}
	ClearError()
	// special Type-nil complete expand as self only
	got := NullPtr.CollectExpandable()
	if !VariablesComplete(got) || len(got) != 1 || got[0] != NullPtr {
		t.Fatal("special CollectExpandable must stay complete self-only", got)
	}
	if HasError() {
		t.Fatal("special CollectExpandable must not sticky")
	}
	ClearError()
	// non-special Type-nil sticky IncompleteVariables (no invent expand pool past hole)
	if VariablesComplete((&Variable{Name: "broken"}).CollectExpandable()) {
		t.Fatal("Type-nil CollectExpandable must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("Type-nil CollectExpandable must SetError sticky")
	}
	ClearError()
	// nested CollectExpandable residual soft invent was soft-continue invent complete expand pool.
	// Fair: sticky IncompleteVariables.
	nestHole := &Variable{Name: "nest", Type: &Type{isStruct: true}, FieldVars: []*Variable{nil}}
	outer := &Variable{
		Name: "outer", Type: &Type{isStruct: true},
		FieldVars: []*Variable{nestHole, CreateVariableScalars("g_ok", GetIntType(), false, false)},
	}
	if VariablesComplete(outer.CollectExpandable()) {
		t.Fatal("nested residual CollectExpandable must fail closed incomplete")
	}
	if !HasError() {
		t.Fatal("nested residual CollectExpandable must SetError sticky")
	}
	ClearError()
}

func TestFieldVolatileOrFromParent(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(1), opts, probs, env)
	st := MakeRandomStructType(NewRng(2), opts, probs, env, "S0")
	// parent volatile
	v := CreateVariableQfer("g_3", st, NewCVQualifiers([]bool{false}, []bool{true}))
	for _, f := range v.FieldVars {
		if !f.IsVolatile() {
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
