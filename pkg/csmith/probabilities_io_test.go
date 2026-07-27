package csmith

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSNamePName(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if GetSNameSess(testAmbientSession, PMoreStructUnionProb) != "more_struct_union_type_prob" {
		t.Fatal(GetSNameSess(testAmbientSession, PMoreStructUnionProb))
	}
	pn, ok := GetPNameSess(testAmbientSession, "inline_function_prob")
	if !ok || pn != PInlineFunctionProb {
		t.Fatal(pn, ok)
	}
	if _, ok := GetPNameSess(testAmbientSession, "nope"); ok || !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestDumpAndParseSingle(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := NewProbabilities(Defaults())
	dump := p.DumpDefaultProbabilitiesSess(testAmbientSession)
	if !strings.Contains(dump, "more_struct_union_type_prob=50") {
		t.Fatal(dump[:200])
	}
	act := p.DumpActualProbabilitiesSess(testAmbientSession, 42)
	if !strings.Contains(act, "# Seed: 42") {
		t.Fatal(act[:80])
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "p.conf")
	if err := os.WriteFile(path, []byte("inline_function_prob=77\n# c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2 := NewProbabilities(Defaults())
	if msg, ok := p2.ParseConfigurationSess(testAmbientSession, path); !ok {
		t.Fatal(msg)
	}
	if p2.SingleSess(testAmbientSession, PInlineFunctionProb) != 77 {
		t.Fatal(p2.SingleSess(testAmbientSession, PInlineFunctionProb))
	}
	// group line fail closed
	if _, ok := p2.ParseLineSess(testAmbientSession, "[statement_prob,x=1]"); ok {
		t.Fatal("group must fail")
	}
	DestroyProcessProbabilitiesSess(testAmbientSession)
	if ProcessProbabilitiesSess(testAmbientSession) != nil {
		t.Fatal("destroy")
	}
	ReinstallTestProcessSingletons()
}

func TestParseStringIntArg(t *testing.T) {
	if s, ok := ParseStringArg("file.c"); !ok || s != "file.c" {
		t.Fatal(s, ok)
	}
	if _, ok := ParseStringArg("--seed"); ok {
		t.Fatal("flag")
	}
	if _, ok := ParseStringArg(""); ok {
		t.Fatal("empty")
	}
	if v, ok := ParseIntArg("123"); !ok || v != 123 {
		t.Fatal(v, ok)
	}
	if _, ok := ParseIntArg("abc"); ok {
		t.Fatal("non-int")
	}
	if ArgCheck(2, 2) == "" {
		t.Fatal("missing arg")
	}
	if ArgCheck(3, 1) != "" {
		t.Fatal("ok")
	}
	if !strings.Contains(PrintVersion(), "go-csmith") {
		t.Fatal(PrintVersion())
	}
	if !strings.Contains(PrintHelp(), "--seed") {
		t.Fatal(PrintHelp())
	}
}

func TestKleeCrestCoverageEmit(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	r := NewRngSess(testAmbientSession, 2)
	probs := NewProbabilities(Defaults())
	vals := AbsExtensionInitializeSess(testAmbientSession, 2, r, probs)
	if vals == nil {
		t.Fatal("init")
	}
	// Klee
	if !strings.Contains(KleeOutputHeader(), "klee") {
		t.Fatal("hdr")
	}
	init := KleeOutputInitSess(testAmbientSession, vals)
	if !strings.Contains(init, "klee_make_symbolic") || !strings.Contains(init, "x0") {
		t.Fatal(init)
	}
	// Crest
	if CrestTypeToStringSess(testAmbientSession, GetIntTypeSess(testAmbientSession)) != "int" {
		t.Fatal(CrestTypeToStringSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))
	}
	// CrestExtension.cpp:69–73 NDEBUG default → empty suffix (CREST_(x) for int128).
	ClearErrorSess(testAmbientSession)
	if CrestTypeToStringSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EUInt128)) != "" {
		t.Fatal("UInt128 Crest type token must be empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("UInt128 empty Crest token is complete NDEBUG path, not sticky")
	}
	cinit := CrestOutputInitSess(testAmbientSession, vals)
	if !strings.Contains(cinit, "CREST_") {
		t.Fatal(cinit)
	}
	// int128 param → CREST_(xN)
	u128 := NewExtensionValueSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EUInt128), "x4")
	c128 := CrestOutputSymbolicsSess(testAmbientSession, []*ExtensionValue{u128})
	if !strings.Contains(c128, "CREST_(x4);") {
		t.Fatal(c128)
	}
	// Coverage
	tests := CoverageGenerateValuesSess(testAmbientSession, vals, 2, r, Defaults(), probs)
	if len(tests) != 4 {
		t.Fatal(len(tests))
	}
	d := CoverageOutputDeclsSess(testAmbientSession, vals, tests, 2)
	if !strings.Contains(d, "a0[2]") || !strings.Contains(d, "test_index") {
		t.Fatal(d)
	}
	inv := CoverageOutputFirstFunInvocationSess(testAmbientSession, vals, "func_1()", 2)
	if !strings.Contains(inv, "for(test_index") || !strings.Contains(inv, "func_1();") {
		t.Fatal(inv)
	}
	ClearErrorSess(testAmbientSession)
}
