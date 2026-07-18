package csmith

import (
	"strings"
	"testing"
)

func TestGetContainerUnion(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
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
}

func TestSiblingUnionPartial(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
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
}

func TestStepHashBody(t *testing.T) {
	opts := Defaults()
	opts.StepHashByStmt = true
	opts.ComputeHash = true
	g := NewProgramGenerator(opts)
	// minimal so OutputHashFuncDef runs
	s := g.OutputHashFuncDef()
	if !strings.Contains(s, "crc32_gentab") || !strings.Contains(s, "step_hash") {
		t.Fatal(s)
	}
}

func TestLooseMatchUnion(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
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
