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

func TestCreateNewTmpVarAlwaysGensym(t *testing.T) {
	// Block.cpp:216–219 — gensym("t_"); no len(TmpVars) soft invent
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
	// nil GenSym uses package util counter (not soft t_+len restarting at 1)
	ResetDefaultGensym()
	x := b.CreateNewTmpVar(nil, EInt)
	y := b.CreateNewTmpVar(nil, EInt)
	if x == y {
		t.Fatal("nil GenSym must advance package gensym", x, y)
	}
	if x != "t_1" || y != "t_2" {
		t.Fatalf("package gensym sequence %q %q", x, y)
	}
}
