package csmith

import (
	"strings"
	"testing"
)

func TestStr2Int(t *testing.T) {
	if Str2Int("42") != 42 {
		t.Fatal(Str2Int("42"))
	}
	if Str2Int("(7)") != 7 {
		t.Fatal(Str2Int("(7)"))
	}
	if Str2Int("0x10") != 16 {
		t.Fatal(Str2Int("0x10"))
	}
}

func TestChopEmptyEndWith(t *testing.T) {
	if Chop("  ab\t") != "ab" {
		t.Fatal(Chop("  ab\t"))
	}
	if !EmptyLine("  \t") || EmptyLine("x") {
		t.Fatal("empty")
	}
	if !EndWith("hello", "lo") {
		t.Fatal("end")
	}
}

func TestSplitIntString(t *testing.T) {
	got := SplitIntString("1, 2, 0xA", ",")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 10 {
		t.Fatal(got)
	}
}

func TestBreakupAssigns(t *testing.T) {
	vs, vals := BreakupAssigns("a = 1; b=2;")
	if len(vs) != 2 || vs[0] != "a" || vals[0] != "1" || vs[1] != "b" || vals[1] != "2" {
		t.Fatal(vs, vals)
	}
}

func TestStr2Int64(t *testing.T) {
	if Str2Int64("0xFF") != 255 {
		t.Fatal(Str2Int64("0xFF"))
	}
	if Int642Str(99) != "99" {
		t.Fatal(Int642Str(99))
	}
}

func TestCollectAndOutputSkippedInits(t *testing.T) {
	outer := &Block{}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	loc.Init = MakeInt(3)
	inner := &Block{Parent: outer, LocalVars: []*Variable{loc}}
	skipped := CollectInitSkippedVars(outer, inner)
	if len(skipped) != 1 || skipped[0] != loc {
		t.Fatal(skipped)
	}
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: skipped}
	out := OutputSkippedVarInits(st, "    ")
	if !strings.Contains(out, "l_1 = 3;") {
		t.Fatal(out)
	}
}

func TestSkippedInitsAtLabelEmit(t *testing.T) {
	loc := CreateVariableScalars("l_2", GetIntType(), false, false)
	loc.Name = "l_2"
	loc.Init = MakeInt(9)
	b := &Block{Stmts: []Stmt{
		{Kind: StmtGoto, Label: "lbl_x", InitSkippedVars: []*Variable{loc},
			Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}},
		{Kind: StmtAssign, SourceLabel: "lbl_x",
			LhsVar: CreateVariableScalars("g_1", GetIntType(), true, false),
			AssignOp: AssignSimple,
			Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)}},
	}}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_x:") || !strings.Contains(out, "l_2 = 9;") {
		t.Fatal(out)
	}
}
