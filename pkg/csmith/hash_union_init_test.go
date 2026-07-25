package csmith

import (
	"strings"
	"testing"
)

// Variable.cpp:891–898 — hash uses get_fact_mgr_for_func(GetFirstFunction())
// live global_facts (Go: first-func FM.UnionFacts), not init-abstract rebuild.
func TestUnionWriteFactsForHashUsesLiveFirstFunc(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// seed 999: upstream hashes g_605.f0 / g_467.f0 (live last_written=0)
	opts := Defaults()
	opts.Seed = 999
	g := NewProgramGenerator(NewSession(opts))
	_ = g.GoGenerator()
	if HasErrorSess(testAmbientSession) {
		t.Log("gen err", GetErrorSess(testAmbientSession))
	}
	uf := g.unionWriteFactsForHash()
	if !UnionFactsComplete(uf) || HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete", uf, HasErrorSess(testAmbientSession), GetErrorSess(testAmbientSession))
	}
	first := GetFirstFunction(&g.Funcs)
	if first == nil {
		t.Fatal("no first function")
	}
	fm := g.FactMgrs.ForFuncSess(testAmbientSession, first)
	if fm == nil || len(uf) != len(fm.UnionFacts) {
		t.Fatalf("hash facts must come from first-func live lattice len uf=%d fm=%v", len(uf), fm)
	}
	var g605, g467 *Variable
	for _, v := range g.VS.GlobalList {
		switch v.Name {
		case "g_605":
			g605 = v
		case "g_467":
			g467 = v
		}
	}
	if g605 != nil {
		if !IsFieldReadableSess(testAmbientSession, g605, 0, uf) || IsFieldReadableSess(testAmbientSession, g605, 4, uf) {
			t.Fatalf("g_605 want only f0 readable")
		}
	}
	if g467 != nil {
		if !IsFieldReadableSess(testAmbientSession, g467, 0, uf) {
			t.Fatalf("g_467 want f0 readable")
		}
	}
	h := g.hashGlobals()
	if !strings.Contains(h, `transparent_crc(g_605.f0`) {
		t.Fatalf("hash missing g_605.f0: %s", h)
	}
	if !strings.Contains(h, `transparent_crc(g_467.f0`) {
		t.Fatalf("hash missing g_467.f0: %s", h)
	}
	if strings.Contains(h, `transparent_crc(g_605.f4`) {
		t.Fatal("must not hash g_605.f4")
	}
}

// seed 34: live g_26 BOTTOM → no field crc; g_255 last=4 → g_255.f4 (Variable.cpp:893–898).
func TestUnionWriteFactsForHashSeed34(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Seed = 34
	g := NewProgramGenerator(NewSession(opts))
	src := g.GoGenerator()
	if src == "" {
		t.Fatal("empty gen", GetErrorSess(testAmbientSession))
	}
	uf := g.unionWriteFactsForHash()
	var g26, g255 *Variable
	for _, v := range g.VS.GlobalList {
		if v == nil {
			continue
		}
		switch v.Name {
		case "g_26":
			g26 = v
		case "g_255":
			g255 = v
		}
	}
	if g26 == nil || g255 == nil {
		t.Fatal("missing g_26/g_255")
	}
	for i := range g26.FieldVars {
		if IsFieldReadableSess(testAmbientSession, g26, i, uf) {
			t.Fatalf("g_26 f%d must not be readable (live BOTTOM)", i)
		}
	}
	if !IsFieldReadableSess(testAmbientSession, g255, 4, uf) {
		t.Fatal("g_255.f4 must be readable")
	}
	h := g.hashGlobals()
	if strings.Contains(h, "g_26.") {
		t.Fatal("must not hash g_26 fields", h)
	}
	if !strings.Contains(h, `transparent_crc(g_255.f4`) {
		t.Fatal("want g_255.f4 hash", h)
	}
}
