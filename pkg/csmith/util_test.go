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
