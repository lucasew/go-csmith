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
	ut := MakeRandomUnionType(NewRng(3), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("no fields")
	}
	if uv.FieldVars[0].GetContainerUnion() != uv {
		t.Fatal("field container")
	}
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	if iv.GetContainerUnion() != nil {
		t.Fatal("int")
	}
	// Variable always live; sticky nil (no invent no-container soft-skip)
	ClearError()
	if (*Variable)(nil).GetContainerUnion() != nil {
		t.Fatal("nil GetContainerUnion must fail closed")
	}
	if !HasError() {
		t.Fatal("nil GetContainerUnion must SetError sticky")
	}
	ClearError()
	// Type-nil on ancestry sticky (no invent skip hole as no-container)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	if field.GetContainerUnion() != nil {
		t.Fatal("Type-nil parent GetContainerUnion must fail closed nil")
	}
	if !HasError() {
		t.Fatal("Type-nil parent GetContainerUnion must SetError sticky")
	}
	ClearError()
}

func TestSiblingUnionPartial(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("need union with 2+ fields")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]
	e := EmptyEffect().WriteVar(f0)
	if !e.SiblingUnionFieldIsWritten(f1) {
		t.Fatal("sibling write")
	}
	if !e.IsWrittenPartially(f1) {
		t.Fatal("partial")
	}
	// incomplete GetCollective on map key fails closed as sibling conflict
	e2 := EmptyEffect()
	e2.written = map[*Variable]bool{nil: true}
	if !e2.SiblingUnionFieldIsWritten(f1) {
		t.Fatal("nil write key must fail closed true")
	}
	// incomplete FieldVars collective on written key fails closed sticky true
	ClearError()
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: ut, IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVars()
	if len(parent.FieldVars) < 2 {
		t.Skip("need 2 fields")
	}
	item := parent.ItemizeConstIndices([]int{0}, nil)
	item.CreateFieldVars()
	if len(item.FieldVars) < 1 {
		t.Skip("item fields")
	}
	fld := item.FieldVars[0]
	item.FieldVars = append(item.FieldVars, nil)
	e3 := EmptyEffect().WriteVar(f0)
	// subject with incomplete collective must fail closed true sticky
	if !e3.SiblingUnionFieldIsWritten(fld) {
		t.Fatal("incomplete collective subject must fail closed true")
	}
	if !HasError() {
		t.Fatal("incomplete collective SiblingUnionFieldIsWritten must SetError sticky")
	}
	ClearError()
}

func TestFindDanglingGlobalPtrs(t *testing.T) {
	f := &Function{Name: "func_1"}
	fm := NewFactMgr(f)
	// dead global pointer
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{NewFactPointTo(p)}
	fm.FindDanglingGlobalPtrs(f)
	if len(f.DeadGlobals) != 1 || f.DeadGlobals[0] != p {
		t.Fatalf("%v", f.DeadGlobals)
	}
	// const not listed
	f.DeadGlobals = nil
	cp := CreateVariableScalars("g_cp", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{NewFactPointTo(cp)}
	fm.FindDanglingGlobalPtrs(f)
	if len(f.DeadGlobals) != 0 {
		t.Fatal("const")
	}
	// incomplete GlobalFacts must IncompleteVariables DeadGlobals sticky
	// (not invent empty-complete "no dangling")
	ClearError()
	fm.GlobalFacts = IncompleteFactSlice()
	fm.FindDanglingGlobalPtrs(f)
	if VariablesComplete(f.DeadGlobals) {
		t.Fatal("incomplete facts must IncompleteVariables DeadGlobals")
	}
	if !HasError() {
		t.Fatal("incomplete facts must SetError sticky")
	}
	ClearError()
	// IsDead residual: PointTo nil hole stickies ERROR+true. Soft invent was append as
	// dead then soft-continue later complete sibling into DeadGlobals. Fair: wipe incomplete.
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), false, false)
	goodDead := CreateVariableScalars("g_p3", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{
		{Var: p2, PointTo: []*Variable{nil}}, // residual IsDead
		NewFactPointTo(goodDead),
	}
	f.DeadGlobals = nil
	fm.FindDanglingGlobalPtrs(f)
	if VariablesComplete(f.DeadGlobals) {
		t.Fatal("IsDead residual must IncompleteVariables DeadGlobals, not invent later dead")
	}
	if !HasError() {
		t.Fatal("IsDead residual FindDanglingGlobalPtrs must SetError sticky")
	}
	ClearError()
}

func TestOutputPtrResets(t *testing.T) {
	CtrlVarsDoFinalization()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	out := OutputPtrResets([]*Variable{p}, Defaults())
	if !strings.Contains(out, "g_p = 0") {
		t.Fatal(out)
	}
	if OutputPtrResets(nil, Defaults()) != "" {
		t.Fatal("empty")
	}
	// incomplete list fails closed sticky (no invent soft-skip hole)
	ClearError()
	if OutputPtrResets([]*Variable{p, nil}, Defaults()) != "" {
		t.Fatal("nil hole must fail closed whole resets")
	}
	if !HasError() {
		t.Fatal("nil hole OutputPtrResets must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was synthetic ArrayVariable shell
	shell := &Variable{Name: "g_a", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{2}}
	if OutputPtrResets([]*Variable{shell}, Defaults()) != "" {
		t.Fatal("IsArray without AsArray must fail closed whole resets")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputPtrResets must SetError sticky")
	}
	ClearError()
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
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(7), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	if !uv.FieldVars[0].LooseMatch(uv.FieldVars[1]) {
		t.Fatal("loose")
	}
}
