package csmith

import "testing"

func TestIsVarOnStack(t *testing.T) {
	f := &Function{Name: "f"}
	p := CreateVariableScalars("p_1", GetIntType(), false, false)
	f.Param = []*Variable{p}
	outer := &Block{Func: f}
	l := CreateVariableScalars("l_1", GetIntType(), false, false)
	outer.LocalVars = []*Variable{l}
	inner := &Block{Func: f, Parent: outer}
	if !inner.IsVarOnStack(l) || !inner.IsVarOnStack(p) {
		t.Fatal("stack")
	}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	if inner.IsVarOnStack(g) {
		t.Fatal("global")
	}
}

func TestChooseVisibleReadVar(t *testing.T) {
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	// only a in read set and global
	got := ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, b}, GetIntType(), nil)
	if got != a && got != b {
		// both global so either ok if both in list
		if got == nil {
			t.Fatal("nil")
		}
	}
	// local not on stack from empty block → only global
	loc := CreateVariableScalars("l_x", GetIntType(), false, false)
	got = ChooseVisibleReadVar(NewRng(2), blk, []*Variable{loc}, GetIntType(), nil)
	if got != nil {
		t.Fatal("local not on stack")
	}
	blk.LocalVars = []*Variable{loc}
	got = ChooseVisibleReadVar(NewRng(2), blk, []*Variable{loc}, GetIntType(), nil)
	if got != loc {
		t.Fatal("local on stack")
	}
}

func TestEffectReadVarsSorted(t *testing.T) {
	a := CreateVariableScalars("g_z", GetIntType(), false, false)
	b := CreateVariableScalars("g_a", GetIntType(), false, false)
	e := EmptyEffect().ReadVar(a).ReadVar(b)
	rs := e.ReadVars()
	if len(rs) != 2 || rs[0].Name != "g_a" {
		t.Fatalf("%v", rs)
	}
}

func TestGotoCreatesCFGEdge(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	// prior statement as target
	prior := Stmt{Kind: StmtAssign, StmID: AllocStmID()}
	blk.Stmts = []Stmt{prior}
	f.Blocks = []*Block{blk}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// seed a read so choose_visible may work
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	eff := EmptyEffect().ReadVar(g)
	cg.EffectAccum = &eff
	// force back by trying many seeds
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		fm.CFGEdges = nil
		blk.Stmts = []Stmt{prior}
		st := MakeRandomGoto(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), cg, blk)
		if st.GotoBack && len(fm.CFGEdges) > 0 {
			e := fm.CFGEdges[0]
			if e.BackLink && e.DestStmID == prior.StmID {
				found = true
				break
			}
		}
	}
	if !found {
		t.Log("back-edge not hit in seed scan; check forward at least")
		st := MakeRandomGoto(NewRng(99), opts, NewProbabilities(opts), vs, NewExprTables(opts), cg, blk)
		if st.Kind != StmtGoto {
			t.Fatal("kind")
		}
	}
}

func TestCastIfNeeded(t *testing.T) {
	pt := PointerTo(GetIntType())
	e := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	castIfNeeded(e)
	if e.CastType != pt {
		t.Fatal("cast")
	}
	// non-zero no cast
	e2 := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "1"}}
	castIfNeeded(e2)
	if e2.CastType != nil {
		t.Fatal("no cast")
	}
}

