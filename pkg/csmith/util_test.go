package csmith

import "testing"

func TestGensymSequence(t *testing.T) {
	// util.cpp gensym: ++count appended to basename
	var g GenSym
	if g.Next("g_") != "g_1" {
		t.Fatal("first gensym")
	}
	if g.Next("g_") != "g_2" {
		t.Fatal("second gensym")
	}
	if g.Next("l_") != "l_3" {
		t.Fatal("shared counter across basenames")
	}
	g.Reset()
	if g.Next("g_") != "g_1" {
		t.Fatal("reset")
	}
}

func TestDoFinalizationResetsGensym(t *testing.T) {
	// DFSProgramGenerator.cpp:92 reset_gensym between runs
	ResetDefaultGensym()
	_ = Gensym("g_")
	DoFinalization()
	if Gensym("g_") != "g_1" {
		t.Fatal("DoFinalization must reset process gensym_count")
	}
}

func TestCreateNewTmpVarAlwaysGensym(t *testing.T) {
	// Block.cpp:216–219 — always gensym("t_") process-wide; ignore private GenSym
	ResetDefaultGensym()
	var sym GenSym
	b := &Block{}
	a := b.CreateNewTmpVar(&sym, EInt)
	c := b.CreateNewTmpVar(&sym, EShort)
	if a != "t_1" || c != "t_2" {
		t.Fatalf("want t_1,t_2 got %q,%q", a, c)
	}
	if b.TmpVars[a] != EInt || b.TmpVars[c] != EShort {
		t.Fatal(b.TmpVars)
	}
	// private GenSym must not invent separate stream
	x := b.CreateNewTmpVar(nil, EInt)
	y := b.CreateNewTmpVar(&sym, EInt)
	if x != "t_3" || y != "t_4" {
		t.Fatalf("process gensym sequence %q %q", x, y)
	}
	// g_/t_ share one util.cpp gensym_count
	if Gensym("g_") != "g_5" {
		t.Fatal("shared counter with RandomGlobalName path")
	}
	// nil Block — sticky no invent bare t_N without registration
	ClearError()
	var nb *Block
	if nb.CreateNewTmpVar(nil, EInt) != "" {
		t.Fatal("nil Block CreateNewTmpVar must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Block CreateNewTmpVar must SetError sticky")
	}
	ClearError()
	// empty gensym basename sticky — no invent bare "1"
	ClearError()
	if Gensym("") != "" {
		t.Fatal("empty basename must fail closed")
	}
	if !HasError() {
		t.Fatal("empty basename Gensym must SetError sticky")
	}
	ClearError()
}
