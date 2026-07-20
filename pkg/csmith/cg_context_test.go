package csmith

import "testing"

func TestCloneSubcontextDeepCopiesIVBounds(t *testing.T) {
	// CGContext.cpp copy ctor deep-copies iv_bounds map
	ClearError()
	iv1 := CreateVariableScalars("i1", GetIntType(), false, false)
	iv2 := CreateVariableScalars("i2", GetIntType(), false, false)
	parent := EmptyCGContext()
	parent.AddIVBound(iv1, 3)
	child := parent.CloneSubcontext()
	child.AddIVBound(iv2, 5)
	if _, ok := parent.IVBounds[iv2]; ok {
		t.Fatal("parent must not see child AddIVBound")
	}
	if parent.IVBounds[iv1] != 3 {
		t.Fatal("parent iv1")
	}
	if child.IVBounds[iv1] != 3 || child.IVBounds[iv2] != 5 {
		t.Fatal(child.IVBounds)
	}
	child.RemoveIVBound(iv1)
	if _, ok := parent.IVBounds[iv1]; !ok {
		t.Fatal("parent must keep iv1 after child Remove")
	}
	// WithFlags also isolates
	parent2 := EmptyCGContext()
	parent2.AddIVBound(iv1, 1)
	loop := parent2.WithFlags(FlagInLoop)
	loop.AddIVBound(iv2, 2)
	if _, ok := parent2.IVBounds[iv2]; ok {
		t.Fatal("WithFlags body must not share IVBounds map")
	}
	if !loop.InLoop() {
		t.Fatal("FlagInLoop")
	}
	ClearError()
}
