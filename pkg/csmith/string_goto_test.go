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

func TestOutputSkippedVarInitsUsesInitExpr(t *testing.T) {
	// StatementGoto.cpp:271 — v->init->Output (InitExpr preferred)
	loc := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	loc.Name = "l_p"
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	loc.InitExpr = &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	// force address-like output path: variable expr of pointed type often emits name
	// set InitExpr to constant pointer-ish "0" via constant for stable assert
	loc.InitExpr = &Expression{Term: TermConstant, Con: MakeInt(0), ExprType: PointerTo(GetIntType())}
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{loc}}
	out := OutputSkippedVarInits(st, "")
	if !strings.Contains(out, "l_p = 0;") {
		t.Fatal(out)
	}
	// InitExpr wins over Init
	loc.Init = MakeInt(99)
	out2 := OutputSkippedVarInits(st, "")
	if strings.Contains(out2, "99") {
		t.Fatal("InitExpr should win", out2)
	}
}

func TestVariableInitOutput(t *testing.T) {
	v := CreateVariableScalars("l_1", GetIntType(), false, false)
	if variableInitOutput(v) != "0" {
		t.Fatal("no init")
	}
	v.Init = MakeInt(5)
	if variableInitOutput(v) != "5" {
		t.Fatal(variableInitOutput(v))
	}
	v.InitExpr = &Expression{Term: TermConstant, Con: MakeInt(7)}
	if variableInitOutput(v) != "7" {
		t.Fatal(variableInitOutput(v))
	}
}
