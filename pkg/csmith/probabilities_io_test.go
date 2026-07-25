package csmith

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSNamePName(t *testing.T) {
	ClearError()
	if GetSName(PMoreStructUnionProb) != "more_struct_union_type_prob" {
		t.Fatal(GetSName(PMoreStructUnionProb))
	}
	pn, ok := GetPName("inline_function_prob")
	if !ok || pn != PInlineFunctionProb {
		t.Fatal(pn, ok)
	}
	if _, ok := GetPName("nope"); ok || !HasError() {
		t.Fatal("unknown sticky")
	}
	ClearError()
}

func TestDumpAndParseSingle(t *testing.T) {
	ClearError()
	p := NewProbabilities(Defaults())
	dump := p.DumpDefaultProbabilities()
	if !strings.Contains(dump, "more_struct_union_type_prob=50") {
		t.Fatal(dump[:200])
	}
	act := p.DumpActualProbabilities(42)
	if !strings.Contains(act, "# Seed: 42") {
		t.Fatal(act[:80])
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "p.conf")
	if err := os.WriteFile(path, []byte("inline_function_prob=77\n# c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2 := NewProbabilities(Defaults())
	if msg, ok := p2.ParseConfiguration(path); !ok {
		t.Fatal(msg)
	}
	if p2.Single(PInlineFunctionProb) != 77 {
		t.Fatal(p2.Single(PInlineFunctionProb))
	}
	// group line fail closed
	if _, ok := p2.ParseLine("[statement_prob,x=1]"); ok {
		t.Fatal("group must fail")
	}
	DestroyProcessProbabilities()
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
	ClearError()
	r := NewRng(2)
	probs := NewProbabilities(Defaults())
	vals := AbsExtensionInitialize(2, r, probs)
	if vals == nil {
		t.Fatal("init")
	}
	// Klee
	if !strings.Contains(KleeOutputHeader(), "klee") {
		t.Fatal("hdr")
	}
	init := KleeOutputInit(vals)
	if !strings.Contains(init, "klee_make_symbolic") || !strings.Contains(init, "x0") {
		t.Fatal(init)
	}
	// Crest
	if CrestTypeToString(GetIntType()) != "int" {
		t.Fatal(CrestTypeToString(GetIntType()))
	}
	cinit := CrestOutputInit(vals)
	if !strings.Contains(cinit, "CREST_") {
		t.Fatal(cinit)
	}
	// Coverage
	tests := CoverageGenerateValues(vals, 2, r, Defaults(), probs)
	if len(tests) != 4 {
		t.Fatal(len(tests))
	}
	d := CoverageOutputDecls(vals, tests, 2)
	if !strings.Contains(d, "a0[2]") || !strings.Contains(d, "test_index") {
		t.Fatal(d)
	}
	inv := CoverageOutputFirstFunInvocation(vals, "func_1()", 2)
	if !strings.Contains(inv, "for(test_index") || !strings.Contains(inv, "func_1();") {
		t.Fatal(inv)
	}
	ClearError()
}
