package csmith

import (
	"strings"
	"testing"
)

func TestHasInitSkippedVars(t *testing.T) {
	outer := &Block{}
	inner := &Block{Parent: outer}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	inner.LocalVars = []*Variable{loc}
	// jump from outer into inner skips l_1
	if !HasInitSkippedVars(outer, inner) {
		t.Fatal("skip")
	}
	// same block no skip
	if HasInitSkippedVars(inner, inner) {
		t.Fatal("same")
	}
}

func TestFindGoodJumpBlock(t *testing.T) {
	f := &Function{Name: "f"}
	b1 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	b2 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 2}}}
	f.Blocks = []*Block{b1, b2}
	got := FindGoodJumpBlock(NewRng(2), f.Blocks, b1, true)
	if got == nil {
		t.Fatal("nil")
	}
	// empty blocks rejected
	empty := &Block{Func: f}
	if FindGoodJumpBlock(NewRng(2), []*Block{empty}, b1, true) != nil {
		t.Fatal("empty")
	}
	// array loop dest rejected
	arr := &Block{Func: f, InArrayLoop: true, Stmts: []Stmt{{Kind: StmtAssign}}}
	if FindGoodJumpBlock(NewRng(2), []*Block{arr}, b1, true) != nil {
		t.Fatal("array dest")
	}
}

func TestOutputPtrResetsArray(t *testing.T) {
	CtrlVarsDoFinalization()
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_a", PointerTo(GetIntType()), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil {
		t.Fatal("av")
	}
	out := OutputPtrResets([]*Variable{&av.Variable}, opts)
	if !strings.Contains(out, "g_a") || !strings.Contains(out, "0") || !strings.Contains(out, "for (i = 0") {
		t.Fatal(out)
	}
}

func TestClearEffectStm(t *testing.T) {
	cg := EmptyCGContext()
	cg.EffectStm = EmptyEffect().WriteVar(CreateVariableScalars("g_x", GetIntType(), false, false))
	cg.ClearEffectStm()
	if !cg.EffectStm.IsSideEffectFree() {
		t.Fatal("clear")
	}
}

func TestGotoUsesFindGoodJumpBlock(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	b1 := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: AllocStmID()}}}
	f.Blocks = []*Block{b1}
	f.Stack = []*Block{b1}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	st := MakeRandomGoto(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b1)
	if st.Kind != StmtGoto {
		t.Fatal(st.Kind)
	}
}
