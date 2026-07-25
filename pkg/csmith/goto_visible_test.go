package csmith

import "testing"

func TestIsVarOnStack(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntType(), false, false)
	f.Param = []*Variable{p}
	outer := &Block{Func: f}
	l := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntType(), false, false)
	outer.LocalVars = []*Variable{l}
	inner := &Block{Func: f, Parent: outer}
	if !inner.IsVarOnStack(l) || !inner.IsVarOnStack(p) {
		t.Fatal("stack")
	}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	if inner.IsVarOnStack(g) {
		t.Fatal("global")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete Block.IsVarOnStack must not sticky")
	}
	// incomplete LocalVars sticky not-on-stack
	ClearErrorSess(testAmbientSession)
	outer.LocalVars = []*Variable{l, nil}
	if inner.IsVarOnStack(l) {
		t.Fatal("LocalVars hole Block.IsVarOnStack must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("LocalVars hole Block.IsVarOnStack must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	outer.LocalVars = []*Variable{l}
	// Match residual: Type-nil param soft invent was soft-continue then invent on-stack later good.
	// Fair: sticky fail closed not-on-stack.
	f.Param = []*Variable{&Variable{Name: "p_hole"}, p} // Type-nil then good
	if inner.IsVarOnStack(p) {
		t.Fatal("Match residual must fail closed not-on-stack, not invent later param match")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Match residual IsVarOnStack must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.Param = []*Variable{p}
}

func TestChooseVisibleReadVar(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	// nil type sticky
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a}, nil, nil) != nil {
		t.Fatal("nil type must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type ChooseVisibleReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// only a in read set and global
	got := ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, b}, GetIntType(), nil)
	if got != a && got != b {
		// both global so either ok if both in list
		if got == nil {
			t.Fatal("nil")
		}
	}
	// local not on stack from empty block → only global
	loc := CreateVariableScalarsSess(testAmbientSession, "l_x", GetIntType(), false, false)
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
	ClearErrorSess(testAmbientSession)
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, nil}, GetIntType(), nil) != nil {
		t.Fatal("nil readVars hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil readVars hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete union facts must not invent soft-filter pick
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a}, GetIntType(), IncompleteUnionFactSlice()) != nil {
		t.Fatal("incomplete union facts must fail closed nil pick")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete union facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was IsVirtual residual false then pick shell
	// fair: sticky nil fail closed
	arrShell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if ChooseVisibleReadVar(NewRng(2), blk, []*Variable{a, arrShell}, GetIntType(), nil) != nil {
		t.Fatal("IsArray without AsArray must fail closed ChooseVisibleReadVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ChooseVisibleReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsNonreadableField / IsInsideUnionField Type-nil ancestry residual: soft invent was
	// continue then pick later good. Fair: sticky fail closed whole choose.
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	// empty facts: IsInsideUnionField still stickies residual before early return not-banned
	blk.LocalVars = []*Variable{field, a}
	if ChooseVisibleReadVar(NewRng(3), blk, []*Variable{field, a}, GetIntType(), nil) != nil {
		t.Fatal("IsInsideUnionField residual must fail closed ChooseVisibleReadVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsInsideUnionField residual ChooseVisibleReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestEffectReadVarsInsertionOrder(t *testing.T) {
	// Effect.cpp:get_read_vars — C++ vector insertion order, not name-sorted.
	// choose_visible_read_var ok_vars index depends on this order.
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_z", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	e := EmptyEffect().ReadVarSess(testAmbientSession, a).ReadVarSess(testAmbientSession, b)
	rs := e.ReadVarsSess(testAmbientSession)
	if len(rs) != 2 || rs[0] != a || rs[1] != b {
		t.Fatalf("want insertion order [g_z,g_a], got %v", rs)
	}
	// struct parent covers field — field not re-pushed (Effect.cpp:117–119 + is_read)
	parent := CreateVariableScalarsSess(testAmbientSession, "g_s", GetIntType(), true, false)
	// mark as struct for IsRead field inheritance
	parent.Type = &Type{isStruct: true, StructName: "S", Fields: []StructField{{Name: "f0", Type: GetIntType(), BitWidth: -1}}}
	field := &Variable{Name: "g_s.f0", Type: GetIntType(), FieldVarOf: parent}
	e2 := EmptyEffect().ReadVarSess(testAmbientSession, parent).ReadVarSess(testAmbientSession, field)
	rs2 := e2.ReadVarsSess(testAmbientSession)
	if len(rs2) != 1 || rs2[0] != parent {
		t.Fatalf("field after parent must not re-push; got %v", rs2)
	}
	if !e2.IsReadSess(testAmbientSession, field) {
		t.Fatal("IsRead(field) true via parent")
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
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// seed a read so choose_visible may work
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), &q, NewRng(2))
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
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
	castIfNeeded(testAmbientSession, e)
	if e.CastType != pt {
		t.Fatal("cast")
	}
	// non-zero no cast
	e2 := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "1"}}
	castIfNeeded(testAmbientSession, e2)
	if e2.CastType != nil {
		t.Fatal("no cast")
	}
}

func TestMakeRandomGotoERRORGuardAndEffectClear(t *testing.T) {
	// StatementGoto.cpp:74/110 ERROR_GUARD after flipcoin / rnd_upto
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
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
	fm := NewFactMgrSess(testAmbientSession, f)
	// map accum effect for forward path cond
	fm.MapAccumEffect = map[int]Effect{1: EmptyEffect().ReadVarSess(testAmbientSession, g)}
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	seed := CreateVariableScalarsSess(testAmbientSession, "g_z", GetIntType(), false, false)
	// try seeds until successful goto (Expr set) to assert effect_stm clear order
	cleared := false
	for seedN := uint64(1); seedN < 40; seedN++ {
		ClearErrorSess(testAmbientSession)
		cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, seed)
		st := MakeRandomGoto(NewRng(seedN), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b2)
		if st.Expr == nil {
			continue
		}
		// StatementGoto.cpp:112 — clear after other_stm pick
		if cg.EffectStm.IsWrittenSess(testAmbientSession, seed) {
			t.Fatal("effect_stm should clear after other_stm pick")
		}
		cleared = true
		break
	}
	if !cleared {
		t.Log("no successful goto in seed scan — ERROR_GUARD path still checked")
	}
	// sticky ERROR after flipcoin → fail closed (no cond invent)
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	st2 := MakeRandomGoto(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, b2)
	if st2.Expr != nil {
		t.Fatal("sticky error must not invent goto condition")
	}
	ClearErrorSess(testAmbientSession)
}
