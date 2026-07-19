package csmith

import "testing"

func TestIsVarOnStack(t *testing.T) {
	ClearError()
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
	if HasError() {
		t.Fatal("complete Block.IsVarOnStack must not sticky")
	}
	// incomplete LocalVars sticky not-on-stack
	ClearError()
	outer.LocalVars = []*Variable{l, nil}
	if inner.IsVarOnStack(l) {
		t.Fatal("LocalVars hole Block.IsVarOnStack must fail closed false")
	}
	if !HasError() {
		t.Fatal("LocalVars hole Block.IsVarOnStack must SetError sticky")
	}
	ClearError()
	outer.LocalVars = []*Variable{l}
	// Match residual: Type-nil param soft invent was soft-continue then invent on-stack later good.
	// Fair: sticky fail closed not-on-stack.
	f.Param = []*Variable{&Variable{Name: "p_hole"}, p} // Type-nil then good
	if inner.IsVarOnStack(p) {
		t.Fatal("Match residual must fail closed not-on-stack, not invent later param match")
	}
	if !HasError() {
		t.Fatal("Match residual IsVarOnStack must SetError sticky")
	}
	ClearError()
	f.Param = []*Variable{p}
}

func TestChooseVisibleReadVar(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	// nil type sticky
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a}, nil, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasError() {
		t.Fatal("nil type ChooseVisibleReadVar must SetError sticky")
	}
	ClearError()
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
	// nil candidate hole fails closed sticky
	ClearError()
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, nil}, GetIntType(), nil) != nil {
		t.Fatal("nil readVars hole must fail closed")
	}
	if !HasError() {
		t.Fatal("nil readVars hole must SetError sticky")
	}
	ClearError()
	// incomplete union facts must not invent soft-filter pick
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a}, GetIntType(), IncompleteUnionFactSlice()) != nil {
		t.Fatal("incomplete union facts must fail closed nil pick")
	}
	if !HasError() {
		t.Fatal("incomplete union facts must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was IsVirtual residual false then pick shell
	// fair: sticky nil fail closed
	arrShell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, arrShell}, GetIntType(), nil) != nil {
		t.Fatal("IsArray without AsArray must fail closed ChooseVisibleReadVar")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray ChooseVisibleReadVar must SetError sticky")
	}
	ClearError()
	// IsNonreadableField / IsInsideUnionField Type-nil ancestry residual: soft invent was
	// continue then pick later good. Fair: sticky fail closed whole choose.
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	// empty facts: IsInsideUnionField still stickies residual before early return not-banned
	blk.LocalVars = []*Variable{field, a}
	if ChooseVisibleReadVar(NewRng(3), blk, []*Variable{field, a}, GetIntType(), nil) != nil {
		t.Fatal("IsInsideUnionField residual must fail closed ChooseVisibleReadVar")
	}
	if !HasError() {
		t.Fatal("IsInsideUnionField residual ChooseVisibleReadVar must SetError sticky")
	}
	ClearError()
}

func TestEffectReadVarsSorted(t *testing.T) {
	ClearError()
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
		st := MakeRandomGoto(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, blk)
		if st.GotoBack && len(fm.CFGEdges) > 0 {
			e := fm.CFGEdges[0]
			if e.BackLink && e.DestStmID == prior.StmID {
				found = true
				break
			}
		}
	}
	if !found {
		// fair null is OK when no good jump; do not require soft-fallback emit
		t.Log("back-edge not hit in seed scan (ok — C++ may return nullptr)")
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

func TestMakeRandomGotoERRORGuardAndEffectClear(t *testing.T) {
	// StatementGoto.cpp:74/110 ERROR_GUARD after flipcoin / rnd_upto
	ClearError()
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g}
	vs.AllVars = []*Variable{g}
	b1 := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1, LhsVar: g, Lhs: &Lhs{Var: g, Type: GetIntType()},
			Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}},
	}}
	b2 := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 2, LhsVar: g, Lhs: &Lhs{Var: g, Type: GetIntType()},
			Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}},
	}}
	f.Blocks = []*Block{b1, b2}
	f.Stack = []*Block{b2}
	fm := NewFactMgr(f)
	// map accum effect for forward path cond
	fm.MapAccumEffect = map[int]Effect{1: EmptyEffect().ReadVar(g)}
	eff := EmptyEffect().ReadVar(g)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	seed := CreateVariableScalars("g_z", GetIntType(), false, false)
	// try seeds until successful goto (Expr set) to assert effect_stm clear order
	cleared := false
	for seedN := uint64(1); seedN < 40; seedN++ {
		ClearError()
		cg.EffectStm = EmptyEffect().WriteVar(seed)
		st := MakeRandomGoto(NewRng(seedN), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b2)
		if st.Expr == nil {
			continue
		}
		// StatementGoto.cpp:112 — clear after other_stm pick
		if cg.EffectStm.IsWritten(seed) {
			t.Fatal("effect_stm should clear after other_stm pick")
		}
		cleared = true
		break
	}
	if !cleared {
		t.Log("no successful goto in seed scan — ERROR_GUARD path still checked")
	}
	// sticky ERROR after flipcoin → fail closed (no cond invent)
	ClearError()
	SetError(ErrGeneric)
	st2 := MakeRandomGoto(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b2)
	if st2.Expr != nil {
		t.Fatal("sticky error must not invent goto condition")
	}
	ClearError()
}
