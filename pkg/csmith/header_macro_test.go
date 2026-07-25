package csmith

import (
	"strings"
	"testing"
)

func TestOutputHeaderAccessOnceMacro(t *testing.T) {
	opts := Defaults()
	opts.AccessOnce = true
	opts.Seed = 2
	g := NewProgramGenerator(NewSession(opts))
	h := g.OutputHeader()
	if !strings.Contains(h, "ACCESS_ONCE") || !strings.Contains(h, "typeof") {
		t.Fatal(h)
	}
}

func TestOutputHeaderWrapVolatiles(t *testing.T) {
	opts := Defaults()
	opts.WrapVolatiles = true
	g := NewProgramGenerator(NewSession(opts))
	h := g.OutputHeader()
	if !strings.Contains(h, "volatile_runtime.h") {
		t.Fatal(h)
	}
}

func TestVolLValLhs(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, true)
	v.UseVolRVal = true
	if v.OutputLhsCOptsSess(testAmbientSession, false) != "VOL_LVAL(g_1, int32_t)" {
		t.Fatal(v.OutputLhsCOptsSess(testAmbientSession, false))
	}
	if v.OutputCSess(testAmbientSession, false) != "VOL_RVAL(g_1, int32_t)" {
		t.Fatal(v.OutputCSess(testAmbientSession, false))
	}
}

func TestCGContextFlags(t *testing.T) {
	c := EmptyCGContext().WithSession(testAmbientSession).WithFlags(FlagInLoop | FlagNoDanglingPtr)
	if !c.InLoop() || !c.NoDanglingPtr() {
		t.Fatal(c.Flags)
	}
}
