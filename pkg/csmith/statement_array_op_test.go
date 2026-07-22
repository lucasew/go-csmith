// Upstream: StatementArrayOp.cpp make_random_array_init (one Statement / stm_id).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "testing"

// TestMakeRandomArrayInitOneStmIDMultiDim — StatementArrayOp.cpp:145–150 creates
// one StatementArrayOp for all dimensions (single Statement ctor / stm_id).
// Soft invent AllocStmID per nested dim shifted later block ids (seed-7 func_41:
// 2D array-init → UP block 525 vs GO 526).
func TestMakeRandomArrayInitOneStmIDMultiDim(t *testing.T) {
	ClearError()
	defer ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Prefer multi-dim so nested shells exist
	av := CreateArrayVariable(NewRng(1), opts, probs, nil, nil, nil, "g_a", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("CreateArrayVariable")
	}
	av.Sizes = []int{3, 4}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	// Loop IVs for both dims
	for _, name := range []string{"i", "j", "k"} {
		iv := CreateVariableQfer(name, GetIntType(), q)
		vs.GlobalList = append(vs.GlobalList, iv)
	}
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f, StmID: 1}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// Count AllocStmID consumed by multi-dim array-init
	nextStmID = 200
	before := nextStmID
	st := MakeRandomArrayInit(NewRng(7), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if HasError() {
		t.Fatalf("MakeRandomArrayInit sticky: %v", GetError())
	}
	if st.Kind != StmtArrayOp {
		t.Fatalf("want StmtArrayOp got %v", st.Kind)
	}
	used := nextStmID - before
	if used != 1 {
		t.Fatalf("multi-dim array-init must AllocStmID once (C++ one Statement), used=%d st.StmID=%d", used, st.StmID)
	}
	if StmIDUnset(st.StmID) {
		t.Fatal("outermost must have live stm_id")
	}
	// Nested dim shells share the same stm_id (one C++ Statement)
	var check func(s Stmt)
	check = func(s Stmt) {
		if s.Kind != StmtArrayOp {
			return
		}
		if s.StmID != st.StmID {
			t.Fatalf("nested ArrayOp dim shell must share stm_id %d, got %d", st.StmID, s.StmID)
		}
		if s.Then == nil {
			return
		}
		for _, ch := range s.Then.Stmts {
			if ch.Kind == StmtArrayOp {
				check(ch)
			}
		}
	}
	check(st)
}
