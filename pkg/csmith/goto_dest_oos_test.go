package csmith

import "testing"

// FactMgr.cpp:262–266 set_fact_out(goto) → update_facts_for_dest(facts, dest).
// Function.cpp:214–224 is_var_oos uses find_variable_in_set → Variable::match
// (Variable.cpp:103–111 / 254–258): aggregate matches its fields.
// Soft invent used pointer identity in IsVarOOS Blocks scan so field pointees
// of later-sibling locals (e.g. l_298.f0) were not OOS at earlier for dest →
// map_facts_out[goto] kept live field instead of garbage → !imply only, full
// VisitFacts still valid, for kept (seed 17809409409875472624 func_61).
func TestIsVarOOSFieldOfLaterSiblingLocal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	f := &Function{Name: "func_61", ReturnType: GetIntTypeSess(testAmbientSession)}
	body := &Block{StmID: 8, Func: f}
	for106 := &Block{StmID: 106, Func: f, Parent: body, Looping: true}
	for146 := &Block{StmID: 146, Func: f, Parent: body, Looping: true}
	l298 := CreateVariableScalarsSess(testAmbientSession, "l_298", st, false, false)
	l298.CreateFieldVarsSess(testAmbientSession)
	if len(l298.FieldVars) < 1 {
		t.Fatal("need f0")
	}
	f0 := l298.FieldVars[0]
	for146.LocalVars = []*Variable{l298}
	f.Blocks = []*Block{body, for106, for146}
	f.Stack = []*Block{body}

	// Field not on stack at body (parent of for142) — only for146 has l_298
	if f.IsVarOnStackSess(testAmbientSession, f0, body) {
		t.Fatal("f0 must not be on stack at function body")
	}
	// C++ find_variable_in_set(local_vars, f0) matches parent aggregate → OOS
	if !f.IsVarOOSSess(testAmbientSession, f0, body) {
		t.Fatal("field of later-sibling local must IsVarOOS at earlier dest parent")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}

	// Live stack aggregate field still not OOS
	body.LocalVars = []*Variable{l298}
	if f.IsVarOOSSess(testAmbientSession, f0, body) {
		t.Fatal("field of live stack aggregate must not be OOS")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUpdateFactsForDestMarksFieldPointeeDead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	st := &Type{
		isStruct: true, StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	f := &Function{Name: "func_61", ReturnType: GetIntTypeSess(testAmbientSession)}
	body := &Block{StmID: 8, Func: f}
	for146 := &Block{StmID: 146, Func: f, Parent: body, Looping: true}
	l298 := CreateVariableScalarsSess(testAmbientSession, "l_298", st, false, false)
	l298.CreateFieldVarsSess(testAmbientSession)
	f0 := l298.FieldVars[0]
	for146.LocalVars = []*Variable{l298}
	f.Blocks = []*Block{body, for146}
	f.Stack = []*Block{body}

	g77 := CreateVariableScalarsSess(testAmbientSession, "g_77", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	g67 := CreateVariableScalarsSess(testAmbientSession, "g_67", GetIntTypeSess(testAmbientSession), false, false)
	factsIn := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, g77, []*Variable{g67, f0})}
	factsOut := []*FactPointTo{}
	UpdateFactsForDestSess(testAmbientSession, factsIn, &factsOut, f, body)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("sticky: %v", GetErrorSess(testAmbientSession))
	}
	got := FindRelatedPointToSess(testAmbientSession, factsOut, g77)
	if got == nil {
		t.Fatal("missing g77")
	}
	if !got.IsDeadSess(testAmbientSession) {
		names := []string{}
		for _, p := range got.PointTo {
			if p != nil {
				names = append(names, p.Name)
			}
		}
		t.Fatalf("OOS field pointee must mark_dead; pts=%v", names)
	}
	ClearErrorSess(testAmbientSession)
}
