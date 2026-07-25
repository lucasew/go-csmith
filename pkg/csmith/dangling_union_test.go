package csmith

import (
	"strings"
	"testing"
)

func TestGetContainerUnion(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	// force a union if possible
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 3), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("no fields")
	}
	if uv.FieldVars[0].GetContainerUnionSess(testAmbientSession) != uv {
		t.Fatal("field container")
	}
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	if iv.GetContainerUnionSess(testAmbientSession) != nil {
		t.Fatal("int")
	}
	// Variable always live; sticky nil (no invent no-container soft-skip)
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).GetContainerUnionSess(testAmbientSession) != nil {
		t.Fatal("nil GetContainerUnion must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetContainerUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil on ancestry sticky (no invent skip hole as no-container)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	if field.GetContainerUnionSess(testAmbientSession) != nil {
		t.Fatal("Type-nil parent GetContainerUnion must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent GetContainerUnion must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSiblingUnionPartial(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort), GetSimpleTypeSess(testAmbientSession, EUInt)}
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("need union with 2+ fields")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]
	e := EmptyEffect().WriteVarSess(testAmbientSession, f0)
	if !e.SiblingUnionFieldIsWrittenSess(testAmbientSession, f1) {
		t.Fatal("sibling write")
	}
	if !e.IsWrittenPartiallySess(testAmbientSession, f1) {
		t.Fatal("partial")
	}
	// incomplete GetCollective on map key fails closed as sibling conflict
	e2 := EmptyEffect()
	e2.written = map[*Variable]bool{nil: true}
	if !e2.SiblingUnionFieldIsWrittenSess(testAmbientSession, f1) {
		t.Fatal("nil write key must fail closed true")
	}
	// incomplete FieldVars collective on written key fails closed sticky true
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: ut, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVarsSess(testAmbientSession)
	if len(parent.FieldVars) < 2 {
		t.Skip("need 2 fields")
	}
	item := parent.ItemizeConstIndices([]int{0}, nil)
	item.CreateFieldVarsSess(testAmbientSession)
	if len(item.FieldVars) < 1 {
		t.Skip("item fields")
	}
	fld := item.FieldVars[0]
	item.FieldVars = append(item.FieldVars, nil)
	e3 := EmptyEffect().WriteVarSess(testAmbientSession, f0)
	// subject with incomplete collective must fail closed true sticky
	if !e3.SiblingUnionFieldIsWrittenSess(testAmbientSession, fld) {
		t.Fatal("incomplete collective subject must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete collective SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindDanglingGlobalPtrs(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgrSess(testAmbientSession, f)
	// dead global pointer
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm.GlobalFacts = []*FactPointTo{NewFactPointToSess(testAmbientSession, p)}
	fm.FindDanglingGlobalPtrs(f)
	if len(f.DeadGlobals) != 1 || f.DeadGlobals[0] != p {
		t.Fatalf("%v", f.DeadGlobals)
	}
	// const not listed
	f.DeadGlobals = nil
	cp := CreateVariableScalarsSess(testAmbientSession, "g_cp", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{NewFactPointToSess(testAmbientSession, cp)}
	fm.FindDanglingGlobalPtrs(f)
	if len(f.DeadGlobals) != 0 {
		t.Fatal("const")
	}
	// incomplete GlobalFacts must IncompleteVariables DeadGlobals sticky
	// (not invent empty-complete "no dangling")
	ClearErrorSess(testAmbientSession)
	fm.GlobalFacts = IncompleteFactSlice()
	fm.FindDanglingGlobalPtrs(f)
	if VariablesComplete(f.DeadGlobals) {
		t.Fatal("incomplete facts must IncompleteVariables DeadGlobals")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsDead residual: PointTo nil hole stickies ERROR+true. Soft invent was append as
	// dead then soft-continue later complete sibling into DeadGlobals. Fair: wipe incomplete.
	p2 := CreateVariableScalarsSess(testAmbientSession, "g_p2", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	goodDead := CreateVariableScalarsSess(testAmbientSession, "g_p3", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm.GlobalFacts = []*FactPointTo{
		{Var: p2, PointTo: []*Variable{nil}}, // residual IsDead
		NewFactPointToSess(testAmbientSession, goodDead),
	}
	f.DeadGlobals = nil
	fm.FindDanglingGlobalPtrs(f)
	if VariablesComplete(f.DeadGlobals) {
		t.Fatal("IsDead residual must IncompleteVariables DeadGlobals, not invent later dead")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsDead residual FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputPtrResets(t *testing.T) {
	CtrlVarsDoFinalizationSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	out := OutputPtrResets([]*Variable{p}, Defaults())
	if !strings.Contains(out, "g_p = 0") {
		t.Fatal(out)
	}
	if OutputPtrResets(nil, Defaults()) != "" {
		t.Fatal("empty")
	}
	// incomplete list fails closed sticky (no invent soft-skip hole)
	ClearErrorSess(testAmbientSession)
	if OutputPtrResets([]*Variable{p, nil}, Defaults()) != "" {
		t.Fatal("nil hole must fail closed whole resets")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole OutputPtrResets must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was synthetic ArrayVariable shell
	shell := &Variable{Name: "g_a", Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), IsArray: true, ArraySizes: []int{2}}
	if OutputPtrResets([]*Variable{shell}, Defaults()) != "" {
		t.Fatal("IsArray without AsArray must fail closed whole resets")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputPtrResets must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStepHashBody(t *testing.T) {
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	g := NewProgramGenerator(NewSession(opts))
	// minimal so OutputHashFuncDef runs
	s := g.OutputHashFuncDef()
	if !strings.Contains(s, "crc32_gentab") || !strings.Contains(s, "step_hash") {
		t.Fatal(s)
	}
}

func TestLooseMatchUnion(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort), GetSimpleTypeSess(testAmbientSession, EUInt)}
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 7), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	if !uv.FieldVars[0].LooseMatchSess(testAmbientSession, uv.FieldVars[1]) {
		t.Fatal("loose")
	}
}
