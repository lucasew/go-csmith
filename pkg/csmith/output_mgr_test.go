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
	ClearMonitoredFuncs()
	defer ClearMonitoredFuncs()
	if !IsMonitoredFunc() {
		t.Fatal("empty list → all monitored")
	}
	SetMonitoredFuncs("func_1,func_2")
	SetCurrFunc("func_1")
	if !IsMonitoredFunc() {
		t.Fatal("func_1 in list")
	}
	SetCurrFunc("func_3")
	if IsMonitoredFunc() {
		t.Fatal("func_3 not in list")
	}
	// Options.ApplyMonitoredFuncs
	o := Defaults()
	o.MonitorFuncs = "main"
	o.ApplyMonitoredFuncs()
	SetCurrFunc("main")
	if !IsMonitoredFunc() {
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
	ClearError()
	o := Defaults()
	if o.SetVolTests("arm") {
		t.Fatal("invalid mach must fail closed")
	}
	if o.VolTestsMach != "" {
		t.Fatal("must not invent store")
	}
	if !o.SetVolTests("x86") || o.VolTestsMach != "x86" {
		t.Fatal(o.VolTestsMach)
	}
	if !o.SetVolTests("x86_64") || o.VolTestsMach != "x86_64" {
		t.Fatal(o.VolTestsMach)
	}
	// nil Options sticky
	var nilO *Options
	if nilO.SetVolTests("x86") || !HasError() {
		t.Fatal("nil SetVolTests sticky")
	}
	ClearError()
}

func TestOutputMgrHashHelpers(t *testing.T) {
	// OutputMgr.cpp:200–208, 156–167, 352
	ClearError()
	if OutputHashFuncDecl() != "void csmith_compute_hash(void);\n\n" {
		t.Fatal(OutputHashFuncDecl())
	}
	if OutputStepHashFuncDecl() != "void step_hash(int stmt_id);\n\n" {
		t.Fatal(OutputStepHashFuncDecl())
	}
	if OutputHashFuncInvocation(1) != "    csmith_compute_hash();\n" {
		t.Fatal(OutputHashFuncInvocation(1))
	}
	ClearMonitoredFuncs()
	defer ClearMonitoredFuncs()
	if got := OutputStepHashFuncInvocation(1, 7); got != "    step_hash(7);\n" {
		t.Fatal(got)
	}
	// not monitored → soft empty
	SetMonitoredFuncs("only_this")
	SetCurrFunc("other")
	if OutputStepHashFuncInvocation(1, 7) != "" {
		t.Fatal("unmonitored soft empty")
	}
	// incomplete id sticky
	SetCurrFunc("only_this")
	if OutputStepHashFuncInvocation(1, 0) != "" || !HasError() {
		t.Fatal("stmt_id 0 sticky")
	}
	ClearError()
	if ReallyOutputLn() != "\n" || OutputLn() != "\n" {
		t.Fatal("newline helpers")
	}
}

func TestDefaultOutputMgrSplitPaths(t *testing.T) {
	// DefaultOutputMgr.cpp:79–101, 207
	ClearError()
	o := Defaults()
	if IsSplit(o) {
		t.Fatal("default not split")
	}
	o.MaxSplitFiles = 3
	if !IsSplit(o) {
		t.Fatal("split")
	}
	// empty dir sticky
	if SplitOutputFilePath(o, 0) != "" || !HasError() {
		t.Fatal("empty SplitFilesDir sticky")
	}
	ClearError()
	o.SplitFilesDir = "/tmp/csmith-split-test"
	p := SplitOutputFilePath(o, 2)
	if !strings.HasSuffix(p, "rnd_output2.c") {
		t.Fatal(p)
	}
	if !strings.HasSuffix(SplitGlobalsHeaderPath(o), "rnd_globals.h") {
		t.Fatal(SplitGlobalsHeaderPath(o))
	}
	// negative index sticky
	if SplitOutputFilePath(o, -1) != "" || !HasError() {
		t.Fatal("neg index")
	}
	ClearError()
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
	if CreateOutputDir("") || !HasError() {
		t.Fatal("empty dir sticky")
	}
	ClearError()
}

func TestDFSCompactEmitGates(t *testing.T) {
	// DFSOutputMgr.cpp:94–108
	ClearError()
	if CompactOutputLn(true) != "" || CompactOutputLn(false) != "\n" {
		t.Fatal("compact ln")
	}
	if CompactOutputCommentLine("x", true, false, false) != "" {
		t.Fatal("compact comment")
	}
	if CompactOutputTab(2, true) != "" {
		t.Fatal("compact tab")
	}
	if CompactOutputTab(1, false) != "    " {
		t.Fatal(CompactOutputTab(1, false))
	}
}
