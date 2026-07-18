package csmith

import (
	"strings"
	"testing"
)

func TestFactPointToOutputCondition(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	f := MakeFactPointTo(p, tgt)
	cond := f.OutputCondition()
	if !strings.Contains(cond, "g_p == &g_1") {
		t.Fatal(cond)
	}
	fn := MakeFactPointTo(p, NullPtr)
	if fn.OutputCondition() != "g_p == 0" {
		t.Fatal(fn.OutputCondition())
	}
	fd := MakeFactPointTo(p, GarbagePtr)
	if !strings.Contains(fd.OutputCondition(), "dangling") {
		t.Fatal(fd.OutputCondition())
	}
}

func TestOutputAssertionCommentedWhenNotAssertable(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := MakeFactPointTo(p, GarbagePtr)
	out := f.OutputAssertion(nil, "    ")
	if !strings.HasPrefix(strings.TrimSpace(out), "//assert") && !strings.Contains(out, "//assert") {
		t.Fatal(out)
	}
	// assertable: global → global
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	ok := MakeFactPointTo(p, tgt)
	out2 := ok.OutputAssertion(nil, "    ")
	if strings.Contains(out2, "//assert") {
		t.Fatal(out2)
	}
	if !strings.Contains(out2, "assert (") {
		t.Fatal(out2)
	}
}

func TestOutputAssertionsParanoid(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	// function reads/writes p so fact is printed
	f.FEffect = EmptyEffect().ReadVar(p).WriteVar(p)
	fm := NewFactMgr(f)
	fm.SetMapFactsIn(5, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(5, []*FactPointTo{MakeFactPointTo(p, tgt)})
	fm.SetupInOutMaps(true)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	out := fm.OutputAssertions(st, nil, "    ", true)
	if !strings.Contains(out, "statement id: 5") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "assert") {
		t.Fatal(out)
	}
}

func TestPostOutputInBlock(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_2", GetIntType(), true, false)
	f.FEffect = EmptyEffect().WriteVar(p).ReadVar(p)
	fm := NewFactMgr(f)
	fm.SetMapFactsIn(7, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(7, []*FactPointTo{MakeFactPointTo(p, tgt)})
	fm.SetupInOutMaps(true)
	b := &Block{
		EmitParanoid: true,
		EmitFM:       fm,
		Stmts: []Stmt{{
			Kind: StmtAssign, StmID: 7,
			LhsVar: p, Lhs: &Lhs{Var: p, Type: PointerTo(GetIntType())},
			Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)},
			AssignOp: AssignSimple,
		}},
	}
	out := b.Output(0)
	if !strings.Contains(out, "assert") {
		t.Fatal(out)
	}
}

func TestIsTopEmpty(t *testing.T) {
	f := &FactPointTo{Var: CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)}
	if !f.IsTop() {
		t.Fatal("empty is top")
	}
}
