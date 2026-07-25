// Upstream: StatementArrayOp.cpp make_random_array_init (one Statement / stm_id).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
	"testing"
)

// TestMakeRandomArrayInitOneStmIDMultiDim — StatementArrayOp.cpp:145–150 creates
// one StatementArrayOp for all dimensions (single Statement ctor / stm_id).
// Soft invent AllocStmID per nested dim shifted later block ids (seed-7 func_41:
// 2D array-init → UP block 525 vs GO 526).
func TestMakeRandomArrayInitOneStmIDMultiDim(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// Prefer multi-dim so nested shells exist
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, probs, nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeInt(0), q)
	if av == nil {
		t.Fatal("CreateArrayVariable")
	}
	av.Sizes = []int{3, 4}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	// Loop IVs for both dims
	for _, name := range []string{"i", "j", "k"} {
		iv := CreateVariableQferSess(testAmbientSession, name, GetIntTypeSess(testAmbientSession), q)
		vs.GlobalList = append(vs.GlobalList, iv)
	}
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, StmID: 1}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// Count AllocStmID consumed by multi-dim array-init
	currentSession().NextStmID = 200
	before := currentSession().NextStmID
	st := MakeRandomArrayInit(NewRngSess(testAmbientSession, 7), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("MakeRandomArrayInit sticky: %v", GetErrorSess(testAmbientSession))
	}
	if st.Kind != StmtArrayOp {
		t.Fatalf("want StmtArrayOp got %v", st.Kind)
	}
	used := currentSession().NextStmID - before
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

// TestMultiDimArrayOpLabelOnce — Statement.cpp:905–917 pre_output once per
// Statement. Multi-dim ArrayOp nests Output-only shells sharing stm_id; emitting
// PreOutput on each shell duplicated lbl_N (seed 86: UP one lbl_1132 vs GO three
// inside nested fors for 3D array-init). Nested shells skip PreOutput.
func TestMultiDimArrayOpLabelOnce(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	// 3-dim shells: outer + 2 nested
	iv0 := CreateVariableScalarsSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), false, false)
	iv1 := CreateVariableScalarsSess(testAmbientSession, "j", GetIntTypeSess(testAmbientSession), false, false)
	iv2 := CreateVariableScalarsSess(testAmbientSession, "k", GetIntTypeSess(testAmbientSession), false, false)
	if iv0 == nil || iv1 == nil || iv2 == nil {
		t.Fatal("iv")
	}
	av := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true},
		Sizes:    []int{2, 3, 1},
	}
	av.AsArray = av
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntTypeSess(testAmbientSession)}
	// Innermost → mid → outer (same StmID)
	sid := 42
	inner := Stmt{
		Kind: StmtArrayOp, StmID: sid, Loop: &LoopControl{IV: iv2, InitN: 0, LimitN: 1, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		ArrayAccess: "g_a[i][j][k]", Expr: rhs, LhsVar: &av.Variable, Then: &Block{},
	}
	mid := Stmt{
		Kind: StmtArrayOp, StmID: sid, Loop: &LoopControl{IV: iv1, InitN: 0, LimitN: 3, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		Expr: rhs, Then: &Block{Stmts: []Stmt{inner}},
	}
	outer := Stmt{
		Kind: StmtArrayOp, StmID: sid, Loop: &LoopControl{IV: iv0, InitN: 0, LimitN: 2, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		Expr: rhs, Then: &Block{Stmts: []Stmt{mid}}, SourceLabel: "lbl_1132",
	}
	// FM with jump source → PreOutput finds label via SourceLabel path without FM,
	// or use SourceLabel when FM nil.
	b := &Block{Stmts: []Stmt{outer}, EmitFM: nil}
	out := b.outputStmtsOnly(0)
	if HasErrorSess(testAmbientSession) || out == "" {
		t.Fatalf("emit empty/err: %q err=%v", out, HasErrorSess(testAmbientSession))
	}
	n := strings.Count(out, "lbl_1132:")
	if n != 1 {
		t.Fatalf("want label once (C++ one Statement pre_output), got %d in:\n%s", n, out)
	}
	// nested fors still present
	if strings.Count(out, "for (") != 3 {
		t.Fatalf("want 3 for headers, got %q", out)
	}
	ClearErrorSess(testAmbientSession)
}
