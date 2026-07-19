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
	// incomplete OutputDef — no invent indent-only blank lines
	broken := &Variable{Name: "g_x", Type: GetIntType()} // no init
	if s := OutputVariableList([]*Variable{broken}, "    ", true); s != "" {
		t.Fatal("empty defs must fail closed empty list", s)
	}
	// incomplete entry fails whole list (no invent skip holes)
	if s := OutputVariableList([]*Variable{a, broken}, "  ", true); s != "" {
		t.Fatal("mixed incomplete must fail closed whole list", s)
	}
	if s := OutputVariableList([]*Variable{a, nil}, "  ", true); s != "" {
		t.Fatal("nil hole must fail closed whole list", s)
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
	// no invent section-only header when all defs empty
	if s := OutputGlobalVariables([]*Variable{{Name: "g_x", Type: GetIntType()}}); s != "" {
		t.Fatal("empty globals must fail closed section", s)
	}
	if s := OutputGlobalVariablesDecls(nil, "extern "); s != "" {
		t.Fatal("nil globals must fail closed section", s)
	}
}

func TestMakeRandomIfFunc1UncertainPath(t *testing.T) {
	// smoke: func_1 with FM does not panic on if generation
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f)), GetIntType(), nil, NewRng(1))
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	// seed may fail; retry
	var st *Stmt
	for seed := uint64(1); seed < 40; seed++ {
		ClearError()
		cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		cg2.EffectAccum = &eff
		cg2.Types = vs.Types
		st = MakeRandomIf(NewRng(seed), opts, probs, vs, NewExprTables(opts),
			NewStatementThresholdTable(opts), &cg2)
		if st != nil && st.Kind == StmtIfElse {
			break
		}
	}
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
}
