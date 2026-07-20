package csmith

import "testing"

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
