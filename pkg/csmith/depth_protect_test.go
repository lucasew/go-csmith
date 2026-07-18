package csmith

import (
	"strings"
	"testing"
)

func TestDepthGuardRandomModeAlwaysGood(t *testing.T) {
	opts := Defaults()
	if DepthGuardByDepth(opts, 99) != GoodDepth {
		t.Fatal("depth")
	}
	if DepthGuardByType(opts, "dtBlock") != GoodDepth {
		t.Fatal("type")
	}
}

func TestDepthProtectEmit(t *testing.T) {
	opts := Defaults()
	opts.DepthProtect = true
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#define MAX_DEPTH") {
		t.Fatal("header")
	}
	if !strings.Contains(out, "DEPTH++") || !strings.Contains(out, "DEPTH--") {
		t.Fatal("block depth")
	}
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") {
		t.Fatal("func guard")
	}
	if !strings.Contains(out, "else") {
		t.Fatal("else return")
	}
}

func TestMakeReturnConstWhenDepthProtect(t *testing.T) {
	opts := Defaults()
	opts.DepthProtect = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	if f == nil {
		t.Fatal("nil")
	}
	if f.NeedReturnStmt() && f.RetConst == nil {
		t.Fatal("expected ret_c")
	}
	out := f.Output()
	if !strings.Contains(out, "if (DEPTH < MAX_DEPTH)") {
		t.Fatal(out)
	}
}
