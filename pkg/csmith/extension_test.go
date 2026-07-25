package csmith

import (
	"strings"
	"testing"
)

func TestExtensionValueAndInitialize(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if NewExtensionValue(nil, "x") != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type sticky")
	}
	ClearErrorSess(testAmbientSession)
	if NewExtensionValue(GetIntTypeSess(testAmbientSession), "") != nil || !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name sticky")
	}
	ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	probs := NewProbabilities(Defaults())
	vals := AbsExtensionInitialize(3, r, probs)
	if vals == nil || len(vals) != 3 || HasErrorSess(testAmbientSession) {
		t.Fatal(len(vals), HasErrorSess(testAmbientSession))
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
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1"}
	ev := NewExtensionValue(GetIntTypeSess(testAmbientSession), "x0")
	inv := AbsExtensionMakeFuncInvocation(f, []*ExtensionValue{ev})
	if inv == nil || HasErrorSess(testAmbientSession) {
		t.Fatal(HasErrorSess(testAmbientSession))
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
	ClearErrorSess(testAmbientSession)
	DestroyExtension()
	CreateExtension(Defaults())
	if ExtensionActiveSess(testAmbientSession) || HasErrorSess(testAmbientSession) {
		t.Fatal("null extension")
	}
	if ExtensionMgrOutputHeaderSess(testAmbientSession) != "" {
		t.Fatal("header")
	}
	if ExtensionMgrOutputTailSess(testAmbientSession) != "    return 0;\n" {
		t.Fatal(ExtensionMgrOutputTailSess(testAmbientSession))
	}
	init := ExtensionMgrOutputInit(true)
	if !strings.Contains(init, "argc") || !strings.Contains(init, "{") {
		t.Fatal(init)
	}
	// klee with live RNG creates extension
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.Klee = true
	o.Func1MaxParams = 2
	SetProcessOptionsSess(testAmbientSession, o)
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 1))
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(o))
	CreateExtension(o)
	if HasErrorSess(testAmbientSession) || !ExtensionActiveSess(testAmbientSession) || ExtensionKindSess(testAmbientSession) != "klee" {
		t.Fatal("klee create", HasErrorSess(testAmbientSession), ExtensionKindSess(testAmbientSession))
	}
	if !strings.Contains(ExtensionMgrOutputHeaderSess(testAmbientSession), "klee/klee.h") {
		t.Fatal(ExtensionMgrOutputHeaderSess(testAmbientSession))
	}
	DestroyExtension()
	ReinstallTestProcessSingletons()
}

func TestInitPartialExpanderEmptyFail(t *testing.T) {
	// C++ parse_options("") fails
	ClearPartialExpanderSess(testAmbientSession)
	if InitPartialExpander("") {
		t.Fatal("empty must fail")
	}
	// FromOptions empty still clears (not init)
	if !InitPartialExpanderFromOptionsSess(testAmbientSession, Defaults()) {
		t.Fatal("from empty opts ok")
	}
}
