package csmith

import (
	"strings"
	"testing"
)

func TestFactUnionLatticeConstants(t *testing.T) {
	// match upstream TOP=-2 BOTTOM=-1
	if FactUnionTop != -2 || FactUnionBottom != -1 {
		t.Fatalf("top=%d bottom=%d", FactUnionTop, FactUnionBottom)
	}
}

func TestFactUnionJoinAndImply(t *testing.T) {
	uv := &Variable{Name: "g_u"}
	a := MakeFactUnion(uv, 0)
	b := MakeFactUnion(uv, 1)
	if a.Imply(b) {
		t.Fatal("0 does not imply 1")
	}
	if !a.Imply(a) {
		t.Fatal("equal")
	}
	bot := MakeFactUnion(uv, FactUnionBottom)
	if !bot.Imply(a) {
		t.Fatal("bottom implies all")
	}
	if a.Imply(bot) {
		t.Fatal("concrete does not imply bottom")
	}
	// join different → bottom
	cp := a.Clone()
	if !cp.Join(b) {
		t.Fatal("join should change")
	}
	if !cp.IsBottom() {
		t.Fatal(cp.LastWrittenFID)
	}
	// join with equal → no change
	c2 := MakeFactUnion(uv, 0)
	if c2.Join(MakeFactUnion(uv, 0)) {
		t.Fatal("no change")
	}
}

func TestIsFieldReadable(t *testing.T) {
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	uv := &Variable{Name: "g_u", Type: ut}
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if !IsFieldReadable(uv, 0, facts) {
		t.Fatal("f0")
	}
	if IsFieldReadable(uv, 1, facts) {
		t.Fatal("f1")
	}
}

func TestFactUnionOutput(t *testing.T) {
	uv := CreateVariableScalars("g_u", GetIntType(), true, false)
	f := MakeFactUnion(uv, 2)
	s := f.Output()
	if !strings.Contains(s, "g_u") || !strings.Contains(s, "2") {
		t.Fatal(s)
	}
}

func TestJoinVarFactsUnion(t *testing.T) {
	u1 := &Variable{Name: "g_u1"}
	facts := []*FactUnion{MakeFactUnion(u1, 0), MakeFactUnion(u1, 1)}
	// same var twice with different fids — FindRelated finds first only
	// join list with one var
	j := JoinVarFactsUnion(facts, []*Variable{u1})
	if j == nil || j.LastWrittenFID != 0 {
		t.Fatal(j)
	}
}
