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
	// point_to_vars always live; no invent skip nil holes in OR list
	broken := &FactPointTo{Var: p, PointTo: []*Variable{tgt, nil}}
	if cond := broken.OutputCondition(); cond != "" {
		t.Fatal("nil pointee must fail closed", cond)
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

func TestPreOutputLabelSkipsStepHash(t *testing.T) {
	// Statement.cpp:905–917 — jump target emits label, not step_hash
	st := &Stmt{Kind: StmtAssign, StmID: 5, SourceLabel: "lbl_1"}
	out, tgt := PreOutput(st, nil, true, false, nil, "    ")
	if !tgt || !strings.Contains(out, "lbl_1:") {
		t.Fatal(out, tgt)
	}
	if strings.Contains(out, "step_hash") {
		t.Fatal("goto target must not step_hash", out)
	}
}

func TestPreOutputStepHashWhenNotTarget(t *testing.T) {
	st := &Stmt{Kind: StmtAssign, StmID: 9}
	out, tgt := PreOutput(st, nil, true, false, nil, "  ")
	if tgt || out != "  step_hash(9);\n" {
		t.Fatal(out, tgt)
	}
}

func TestPreOutputFromCFGJumpSources(t *testing.T) {
	f := &Function{Name: "f"}
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtGoto, StmID: 2, Label: "lbl_cfg", GotoDestStmID: 1},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 2, DestStmID: 1}}
	st := &body.Stmts[0]
	out, tgt := PreOutput(st, fm, true, false, nil, "")
	if !tgt || !strings.Contains(out, "lbl_cfg:") {
		t.Fatal(out, tgt)
	}
}

func TestBlockOutputPreOutputNoHashOnLabel(t *testing.T) {
	b := &Block{
		EmitStepHash: true,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: 3, SourceLabel: "lbl_x",
				LhsVar: CreateVariableScalars("g_1", GetIntType(), false, false),
				Expr:   &Expression{Term: TermConstant, Con: MakeInt(1)},
				AssignOp: AssignSimple},
		},
	}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_x:") {
		t.Fatal(out)
	}
	// labeled stmt should not have step_hash(3)
	if strings.Contains(out, "step_hash(3)") {
		t.Fatal("labeled target should skip hash", out)
	}
}
