package csmith

import (
	"strings"
	"testing"
)

func TestParseStringOptions(t *testing.T) {
	// CGOptions.cpp:562–577
	if ParseStringOptions("") != nil {
		t.Fatal("empty")
	}
	got := ParseStringOptions("v1,v2,v3")
	if len(got) != 3 || got[0] != "v1" || got[1] != "v2" || got[2] != "v3" {
		t.Fatal(got)
	}
	// no trim
	got = ParseStringOptions("a, b")
	if len(got) != 2 || got[1] != " b" {
		t.Fatal(got)
	}
	// single token
	got = ParseStringOptions("only")
	if len(got) != 1 || got[0] != "only" {
		t.Fatal(got)
	}
}

func TestMonitoredFuncs(t *testing.T) {
	// OutputMgr.cpp:81–86
	ClearMonitoredFuncsSess(testAmbientSession)
	defer ClearMonitoredFuncsSess(testAmbientSession)
	if !IsMonitoredFuncSess(testAmbientSession) {
		t.Fatal("empty list → all monitored")
	}
	SetMonitoredFuncsSess(testAmbientSession, "func_1,func_2")
	SetCurrFuncSess(testAmbientSession, "func_1")
	if !IsMonitoredFuncSess(testAmbientSession) {
		t.Fatal("func_1 in list")
	}
	SetCurrFuncSess(testAmbientSession, "func_3")
	if IsMonitoredFuncSess(testAmbientSession) {
		t.Fatal("func_3 not in list")
	}
	// Options.ApplyMonitoredFuncs
	o := Defaults()
	o.MonitorFuncs = "main"
	o.ApplyMonitoredFuncsSess(testAmbientSession)
	SetCurrFuncSess(testAmbientSession, "main")
	if !IsMonitoredFuncSess(testAmbientSession) {
		t.Fatal("main")
	}
}

func TestIsX8664(t *testing.T) {
	// Just ensure it returns a bool without invent; architecture-dependent.
	_ = IsX8664()
}

func TestFuncAttrFlagAndVolTestsMach(t *testing.T) {
	o := Defaults()
	if o.FuncAttrFlag() {
		t.Fatal("default false")
	}
	o.FunctionAttributes = true
	if !o.FuncAttrFlag() {
		t.Fatal("flag")
	}
	o.VolTestsMach = "x86"
	if o.VolTestsMachValue() != "x86" {
		t.Fatal(o.VolTestsMachValue())
	}
}

func TestSetVolTests(t *testing.T) {
	// Historical CGOptions::set_vol_tests (csmith-2.1.0); pin header-only.
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	if o.SetVolTestsSess(testAmbientSession, "arm") {
		t.Fatal("invalid mach must fail closed")
	}
	if o.VolTestsMach != "" {
		t.Fatal("must not invent store")
	}
	if !o.SetVolTestsSess(testAmbientSession, "x86") || o.VolTestsMach != "x86" {
		t.Fatal(o.VolTestsMach)
	}
	if !o.SetVolTestsSess(testAmbientSession, "x86_64") || o.VolTestsMach != "x86_64" {
		t.Fatal(o.VolTestsMach)
	}
	// nil Options sticky
	var nilO *Options
	if nilO.SetVolTestsSess(testAmbientSession, "x86") || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil SetVolTests sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputMgrHashHelpers(t *testing.T) {
	// OutputMgr.cpp:200–208, 156–167, 352
	ClearErrorSess(testAmbientSession)
	if OutputHashFuncDecl() != "void csmith_compute_hash(void);\n\n" {
		t.Fatal(OutputHashFuncDecl())
	}
	if OutputStepHashFuncDecl() != "void step_hash(int stmt_id);\n\n" {
		t.Fatal(OutputStepHashFuncDecl())
	}
	if OutputHashFuncInvocation(1) != "    csmith_compute_hash();\n" {
		t.Fatal(OutputHashFuncInvocation(1))
	}
	ClearMonitoredFuncsSess(testAmbientSession)
	defer ClearMonitoredFuncsSess(testAmbientSession)
	if got := OutputStepHashFuncInvocationSess(testAmbientSession, 1, 7); got != "    step_hash(7);\n" {
		t.Fatal(got)
	}
	// not monitored → soft empty
	SetMonitoredFuncsSess(testAmbientSession, "only_this")
	SetCurrFuncSess(testAmbientSession, "other")
	if OutputStepHashFuncInvocationSess(testAmbientSession, 1, 7) != "" {
		t.Fatal("unmonitored soft empty")
	}
	// incomplete id sticky
	SetCurrFuncSess(testAmbientSession, "only_this")
	if OutputStepHashFuncInvocationSess(testAmbientSession, 1, 0) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("stmt_id 0 sticky")
	}
	ClearErrorSess(testAmbientSession)
	if ReallyOutputLn() != "\n" || OutputLn() != "\n" {
		t.Fatal("newline helpers")
	}
}

func TestDefaultOutputMgrSplitPaths(t *testing.T) {
	// DefaultOutputMgr.cpp:79–101, 207
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	if IsSplit(o) {
		t.Fatal("default not split")
	}
	o.MaxSplitFiles = 3
	if !IsSplit(o) {
		t.Fatal("split")
	}
	// empty dir sticky
	if SplitOutputFilePathSess(testAmbientSession, o, 0) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty SplitFilesDir sticky")
	}
	ClearErrorSess(testAmbientSession)
	o.SplitFilesDir = "/tmp/csmith-split-test"
	p := SplitOutputFilePathSess(testAmbientSession, o, 2)
	if !strings.HasSuffix(p, "rnd_output2.c") {
		t.Fatal(p)
	}
	if !strings.HasSuffix(SplitGlobalsHeaderPathSess(testAmbientSession, o), "rnd_globals.h") {
		t.Fatal(SplitGlobalsHeaderPathSess(testAmbientSession, o))
	}
	// negative index sticky
	if SplitOutputFilePathSess(testAmbientSession, o, -1) != "" || !HasErrorSess(testAmbientSession) {
		t.Fatal("neg index")
	}
	ClearErrorSess(testAmbientSession)
	body := SplitGlobalsHeaderBody("extern int g;\n", "/* structs */\n")
	if !strings.Contains(body, "#ifndef RND_GLOBALS_H") || !strings.Contains(body, "extern int g;") {
		t.Fatal(body)
	}
	sec := SplitSecondaryHeaderPreamble(true)
	if !strings.Contains(sec, "assert.h") || !strings.Contains(sec, "rnd_globals.h") {
		t.Fatal(sec)
	}
	if SplitPrimaryHeaderInclude() != "#include \"rnd_globals.h\"\n" {
		t.Fatal(SplitPrimaryHeaderInclude())
	}
	// CreateOutputDir empty sticky
	if CreateOutputDirSess(testAmbientSession, "") || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty dir sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestDFSCompactEmitGates(t *testing.T) {
	// DFSOutputMgr.cpp:94–108
	ClearErrorSess(testAmbientSession)
	if CompactOutputLn(true) != "" || CompactOutputLn(false) != "\n" {
		t.Fatal("compact ln")
	}
	if CompactOutputCommentLineSess(testAmbientSession, "x", true, false, false) != "" {
		t.Fatal("compact comment")
	}
	if CompactOutputTab(2, true) != "" {
		t.Fatal("compact tab")
	}
	if CompactOutputTab(1, false) != "    " {
		t.Fatal(CompactOutputTab(1, false))
	}
}
