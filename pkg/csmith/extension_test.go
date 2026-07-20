package csmith

import (
	"strings"
	"testing"
)

func TestExtensionValueAndInitialize(t *testing.T) {
	ClearError()
	if NewExtensionValue(nil, "x") != nil || !HasError() {
		t.Fatal("nil type sticky")
	}
	ClearError()
	if NewExtensionValue(GetIntType(), "") != nil || !HasError() {
		t.Fatal("empty name sticky")
	}
	ClearError()
	r := NewRng(2)
	probs := NewProbabilities(Defaults())
	vals := AbsExtensionInitialize(3, r, probs)
	if vals == nil || len(vals) != 3 || HasError() {
		t.Fatal(len(vals), HasError())
	}
	if vals[0].Name != "x0" || vals[2].Name != "x2" {
		t.Fatal(vals[0].Name, vals[2].Name)
	}
	defs := AbsExtensionDefaultOutputDefinitions(vals, true)
	if !strings.Contains(defs, "x0 = 0") || !strings.Contains(defs, "x1 = 0") {
		t.Fatal(defs)
	}
}

func TestAbsExtensionMakeInvocation(t *testing.T) {
	ClearError()
	f := &Function{Name: "func_1"}
	ev := NewExtensionValue(GetIntType(), "x0")
	inv := AbsExtensionMakeFuncInvocation(f, []*ExtensionValue{ev})
	if inv == nil || HasError() {
		t.Fatal(HasError())
	}
	out := inv.Output()
	if !strings.Contains(out, "func_1") || !strings.Contains(out, "x0") {
		t.Fatal(out)
	}
	first := AbsExtensionOutputFirstFunInvocation(out)
	if !strings.HasPrefix(first, "    ") || !strings.HasSuffix(first, ";\n") {
		t.Fatal(first)
	}
}

func TestExtensionMgrNullPath(t *testing.T) {
	ClearError()
	DestroyExtension()
	CreateExtension(Defaults())
	if ExtensionActive() || HasError() {
		t.Fatal("null extension")
	}
	if ExtensionMgrOutputHeader() != "" {
		t.Fatal("header")
	}
	if ExtensionMgrOutputTail() != "    return 0;\n" {
		t.Fatal(ExtensionMgrOutputTail())
	}
	init := ExtensionMgrOutputInit(true)
	if !strings.Contains(init, "argc") || !strings.Contains(init, "{") {
		t.Fatal(init)
	}
	// klee with live RNG creates extension
	ClearError()
	o := Defaults()
	o.Klee = true
	o.Func1MaxParams = 2
	SetProcessOptions(o)
	SetProcessRng(NewRng(1))
	SetProcessProbabilities(NewProbabilities(o))
	CreateExtension(o)
	if HasError() || !ExtensionActive() || ExtensionKind() != "klee" {
		t.Fatal("klee create", HasError(), ExtensionKind())
	}
	if !strings.Contains(ExtensionMgrOutputHeader(), "klee/klee.h") {
		t.Fatal(ExtensionMgrOutputHeader())
	}
	DestroyExtension()
	ReinstallTestProcessSingletons()
}

func TestInitPartialExpanderEmptyFail(t *testing.T) {
	// C++ parse_options("") fails
	ClearPartialExpander()
	if InitPartialExpander("") {
		t.Fatal("empty must fail")
	}
	// FromOptions empty still clears (not init)
	if !InitPartialExpanderFromOptions(Defaults()) {
		t.Fatal("from empty opts ok")
	}
}
