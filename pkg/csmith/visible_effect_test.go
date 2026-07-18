package csmith

import (
	"strings"
	"testing"
)

func TestAddExternalEffectWithCallers(t *testing.T) {
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	// other has global + local write
	other := EmptyEffect().WriteVar(g).WriteVar(loc)
	// without chain: only global
	e1 := EmptyEffect().AddExternalEffect(other)
	if !e1.IsWritten(g) || e1.written[loc] {
		t.Fatal("globals only", e1)
	}
	// with chain containing loc: both
	e2 := EmptyEffect().AddExternalEffectWithCallers(other, []*Block{blk})
	if !e2.IsWritten(g) || !e2.written[loc] {
		t.Fatal("chain includes local", e2)
	}
}

func TestAddVisibleEffectUsesChain(t *testing.T) {
	f := &Function{Name: "func_1"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect())
	cg.EffectAccum = &eff
	// call chain empty; current block has loc
	other := EmptyEffect().WriteVar(loc)
	cg.AddVisibleEffect(other)
	if !cg.EffectAccum.written[loc] {
		t.Fatal("visible local via current block")
	}
}

func TestOutputVariableList(t *testing.T) {
	a := CreateVariableScalars("g_b", GetIntType(), true, false)
	b := CreateVariableScalars("g_a", GetIntType(), true, false)
	out := OutputVariableList([]*Variable{a, b}, "  ", true)
	// sorted by name: g_a then g_b
	ia := strings.Index(out, "g_a")
	ib := strings.Index(out, "g_b")
	if ia < 0 || ib < 0 || ia > ib {
		t.Fatal(out)
	}
}

func TestOutputGlobalVariables(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	out := OutputGlobalVariables([]*Variable{v})
	if !strings.Contains(out, "GLOBAL VARIABLES") || !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
	decl := OutputGlobalVariablesDecls([]*Variable{v}, "extern ")
	if !strings.Contains(decl, "extern ") {
		t.Fatal(decl)
	}
}

func TestMakeRandomIfFunc1UncertainPath(t *testing.T) {
	// smoke: func_1 with FM does not panic on if generation
	opts := Defaults()
	opts.MaxBlockSize = 1
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	st := MakeRandomIf(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts),
		NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
}
