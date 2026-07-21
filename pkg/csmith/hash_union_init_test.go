package csmith

import (
	"strings"
	"testing"
)

// Variable.cpp:891–898 — hash filters union fields via is_field_readable on
// first-func global_facts. Init abstract is last_written 0 for constant init.
func TestUnionWriteFactsForHashFromInit(t *testing.T) {
	ClearError()
	opts := Defaults()
	opts.Seed = 999
	g := NewProgramGenerator(opts)
	_ = g.GoGenerator()
	if HasError() {
		t.Log("gen err", GetError())
	}
	uf := g.unionWriteFactsForHash()
	if !UnionFactsComplete(uf) || HasError() {
		t.Fatal("incomplete", uf, HasError(), GetError())
	}
	// g_605 / g_467 should be readable as field 0 only
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
		if !IsFieldReadable(g605, 0, uf) || IsFieldReadable(g605, 4, uf) {
			t.Fatalf("g_605 want only f0 readable")
		}
	}
	if g467 != nil {
		if !IsFieldReadable(g467, 0, uf) {
			t.Fatalf("g_467 want f0 readable, live was BOTTOM")
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
