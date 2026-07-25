package csmith

import (
	"strings"
	"testing"
)

func TestCreateDefaultOutputMgrSplit(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ClearOutputMgrSess(testAmbientSession)
	o := Defaults()
	o.MaxSplitFiles = 3
	o.SplitFilesDir = "/tmp/csmith-split-unit"
	if !CreateDefaultOutputMgr(o) || HasErrorSess(testAmbientSession) {
		t.Fatal("create", HasErrorSess(testAmbientSession))
	}
	paths := ProcessSplitPathsSess(testAmbientSession)
	if len(paths) != 3 {
		t.Fatal(paths)
	}
	if !strings.HasSuffix(paths[1], "rnd_output1.c") {
		t.Fatal(paths[1])
	}
	if GetMainOutPath(o) != paths[0] {
		t.Fatal(GetMainOutPath(o))
	}
	ClearOutputMgrSess(testAmbientSession)
}

func TestCreateDefaultOutputMgrSplitDirSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ClearOutputMgrSess(testAmbientSession)
	o := Defaults()
	o.MaxSplitFiles = 2
	o.SplitFilesDir = ""
	if CreateDefaultOutputMgr(o) || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty dir sticky")
	}
	ClearErrorSess(testAmbientSession)
	ClearOutputMgrSess(testAmbientSession)
}

func TestRandomOutputVarDefsAssign(t *testing.T) {
	// DefaultOutputMgr.cpp:144–151 pure_rnd_upto per global
	ClearErrorSess(testAmbientSession)
	defer func() {
		RandomNumberDoFinalizationSess(testAmbientSession)
		ReinstallTestProcessSingletons()
		ClearErrorSess(testAmbientSession)
	}()
	CreateRandomNumberInstanceSess(testAmbientSession, RngKindDefault, 2)
	o := Defaults()
	SetProcessOptionsSess(testAmbientSession, o)
	v1 := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	v2 := CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntTypeSess(testAmbientSession), true, false)
	out := RandomOutputVarDefs([]*Variable{v1, v2}, 2, true)
	if out == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("var defs", HasErrorSess(testAmbientSession))
	}
	if len(out) != 2 {
		t.Fatal(len(out))
	}
	// both defs placed somewhere
	joined := out[0] + out[1]
	if !strings.Contains(joined, "g_1") || !strings.Contains(joined, "g_2") {
		t.Fatal(joined)
	}
}

func TestRandomOutputDefsEmptyFilesSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if RandomOutputVarDefs(nil, 0, true) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("nFiles 0 sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSplitAllHeadersContent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	h := SplitAllHeadersContent(3, true, "void foo(void);\n")
	if len(h) != 3 {
		t.Fatal(len(h))
	}
	if !strings.Contains(h[1], "assert.h") {
		t.Fatal(h[1])
	}
	if !strings.Contains(h[0], "rnd_globals.h") {
		t.Fatal(h[0])
	}
	for i := 0; i < 3; i++ {
		if !strings.Contains(h[i], "void foo(void);") {
			t.Fatal(i, h[i])
		}
	}
}

func TestCreateDFSOutputMgr(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ClearOutputMgrSess(testAmbientSession)
	o := Defaults()
	CreateDFSOutputMgr(o)
	if ProcessOutputMgrKindSess(testAmbientSession) != OutputMgrKindDFS {
		t.Fatal(ProcessOutputMgrKindSess(testAmbientSession))
	}
	if ProcessStructOutputSess(testAmbientSession) != DefaultStructOutputName {
		t.Fatal(ProcessStructOutputSess(testAmbientSession))
	}
	o.StructOutput = "my_structs.h"
	CreateDFSOutputMgr(o)
	if ProcessStructOutputSess(testAmbientSession) != "my_structs.h" {
		t.Fatal(ProcessStructOutputSess(testAmbientSession))
	}
	if DFSOutputHeader("HDR\n", true) != "" {
		t.Fatal("compact skip")
	}
	if DFSOutputHeader("HDR\n", false) != "HDR\n" {
		t.Fatal("non-compact")
	}
	ClearOutputMgrSess(testAmbientSession)
}

func TestGetCountPrefix(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := &ProgramGenerator{Sess: testAmbientSession, OutputKind: OutputMgrKindDefault, GoodCount: 3}
	if g.GetCountPrefix("x") != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("default assert sticky")
	}
	ClearErrorSess(testAmbientSession)
	g.OutputKind = OutputMgrKindDFS
	if g.GetCountPrefix("foo") != "p_3_foo" {
		t.Fatal(g.GetCountPrefix("foo"))
	}
	if g.GetCountPrefix("") != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestProcessProgramGenerator(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	ClearProcessProgramGeneratorSess(testAmbientSession)
	if ProcessProgramGeneratorSess(testAmbientSession) != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil sticky")
	}
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	s := NewSession(o)
	g := NewProgramGenerator(s)
	// Generator is owned by the session bag (not ambient after nested activate returns).
	if g == nil || g.Sess != s || s.ProgramGen != g {
		t.Fatal("ProgramGenerator.Sess / s.ProgramGen must wire the run bag")
	}
	if g.GetOutputMgrKind() != OutputMgrKindDefault {
		t.Fatal(g.GetOutputMgrKind())
	}
	DoFinalizationSess(testAmbientSession)
	ReinstallTestProcessSingletons()
	if ProcessProgramGeneratorSess(testAmbientSession) != nil {
		// finalization clears; may sticky on Get
		ClearErrorSess(testAmbientSession)
	}
}

func TestNewProgramGeneratorDFSSelectsKind(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	prevO := ProcessOptionsSess(testAmbientSession)
	defer func() {
		DoFinalizationSess(testAmbientSession)
		ReinstallTestProcessSingletons()
		SetProcessOptionsSess(testAmbientSession, prevO)
		ClearErrorSess(testAmbientSession)
	}()
	o := Defaults()
	o.DFSExhaustive = true
	o.RandomBased = false
	o.MaxExhaustiveDepth = 6
	s := NewSession(o)
	g := NewProgramGenerator(s)
	if g == nil || g.OutputKind != OutputMgrKindDFS {
		t.Fatal(g)
	}
	if g.Rng == nil || g.Rng.Kind() != RngKindDFS {
		t.Fatal("DFS rng", g.Rng)
	}
	// Output mgr kind is on the session bag (not ambient after construction).
	if s.OutputMgrKind != OutputMgrKindDFS {
		t.Fatal(s.OutputMgrKind)
	}
}
