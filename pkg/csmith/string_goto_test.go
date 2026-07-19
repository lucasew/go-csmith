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
	// StringUtils.cpp:153 — assert close paren; no invent parse
	if Str2Int("(7") != -1 {
		t.Fatal("unbalanced paren must fail closed")
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
	// StringUtils.cpp:222 assert(pair.size()==2); no soft invent skip
	vs, vals = BreakupAssigns("a=1; broken; c=3")
	if vs != nil || vals != nil {
		t.Fatal("malformed assign must fail whole parse")
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

func TestSkippedInitsAtLabelNotEmitted(t *testing.T) {
	// Statement.cpp:911–913 — output_skipped_var_inits after label is commented out
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
	if !strings.Contains(out, "lbl_x:") {
		t.Fatal(out)
	}
	if strings.Contains(out, "l_2 = 9;") {
		t.Fatal("skipped inits must not emit after label (C++ pre_output commented)", out)
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

func TestOutputSkippedVarInitsNoInventEmptyRHS(t *testing.T) {
	// StatementGoto.cpp:271 — assert(v->init); vars[i] always live
	// incomplete entry fails whole emit (no invent skip holes / partial list)
	v := CreateVariableWithInit("l_miss", GetIntType(), nil, NewCVQualifiers([]bool{false}, []bool{false}))
	v.Name = "l_miss"
	good := CreateVariableScalars("l_ok", GetIntType(), false, false)
	good.Name = "l_ok"
	good.Init = MakeInt(4)
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{v, good}}
	out := OutputSkippedVarInits(st, "")
	if out != "" {
		t.Fatal("incomplete re-init list must fail closed whole emit", out)
	}
	// no invent " = 5;" without identifier / partial good siblings
	anon := CreateVariableScalars("l_x", GetIntType(), false, false)
	anon.Name = ""
	anon.Init = MakeInt(5)
	st2 := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{anon, good}}
	out2 := OutputSkippedVarInits(st2, "")
	if out2 != "" {
		t.Fatal("empty name must fail closed whole emit", out2)
	}
	// complete list still emits
	st3 := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{good}}
	if out3 := OutputSkippedVarInits(st3, ""); !strings.Contains(out3, "l_ok = 4;") {
		t.Fatal(out3)
	}
}

func TestVariableInitOutput(t *testing.T) {
	// StatementGoto.cpp:271 — assert(v->init); no soft invent "0" when missing
	v := CreateVariableWithInit("l_1", GetIntType(), nil, NewCVQualifiers([]bool{false}, []bool{false}))
	if variableInitOutput(v) != "" {
		t.Fatal("nil init must not invent 0", variableInitOutput(v))
	}
	// Variable.cpp:395 — CreateVariableScalars always Constant::make_random
	v2 := CreateVariableScalars("l_2", GetIntType(), false, false)
	if variableInitOutput(v2) == "" {
		t.Fatal("scalars path always has init")
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
