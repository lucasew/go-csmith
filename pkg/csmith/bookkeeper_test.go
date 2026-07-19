package csmith

import (
	"strings"
	"testing"
)

func TestIncrCounterAndCalcTotal(t *testing.T) {
	var c []int
	IncrCounter(&c, 0)
	IncrCounter(&c, 2)
	IncrCounter(&c, 2)
	if len(c) != 3 || c[0] != 1 || c[1] != 0 || c[2] != 2 {
		t.Fatal(c)
	}
	if CalcTotal(c) != 3 {
		t.Fatal(CalcTotal(c))
	}
}

func TestRecordAddressTaken(t *testing.T) {
	BookkeeperDoFinalization()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	RecordAddressTaken(v)
	if !v.IsAddrTaken || addressTakenCnt != 1 {
		t.Fatal(addressTakenCnt, v.IsAddrTaken)
	}
}

func TestRecordVolatileAccess(t *testing.T) {
	BookkeeperDoFinalization()
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	RecordVolatileAccess(v, 0, false)
	if readVolatileCnt != 1 {
		t.Fatal(readVolatileCnt)
	}
	RecordVolatileAccess(v, 0, true)
	if writeVolatileCnt != 1 {
		t.Fatal(writeVolatileCnt)
	}
	nv := CreateVariableScalars("g_n", GetIntType(), false, false)
	RecordVolatileAccess(nv, 0, false)
	if readNonVolatileCnt != 1 {
		t.Fatal(readNonVolatileCnt)
	}
}

func TestRecordJumpsAndVarFreshness(t *testing.T) {
	BookkeeperDoFinalization()
	RecordForwardJump()
	RecordBackwardJump()
	RecordBackwardJump()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	RecordVarCreated(v)
	RecordVarReused()
	if forwardJumpCnt != 1 || backwardJumpCnt != 2 {
		t.Fatal(forwardJumpCnt, backwardJumpCnt)
	}
	if useNewVarCnt != 1 || useOldVarCnt != 1 {
		t.Fatal(useNewVarCnt, useOldVarCnt)
	}
	out := OutputStatistics(nil, Defaults())
	if !strings.Contains(out, "forward jumps: 1") || !strings.Contains(out, "backward jumps: 2") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "percentage a fresh-made variable is used: 50") {
		t.Fatal(out)
	}
}

func TestOutputTailStatistics(t *testing.T) {
	BookkeeperDoFinalization()
	opts := Defaults()
	opts.Seed = 4
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/************************ statistics *************************") {
		t.Fatal("missing statistics block")
	}
	if !strings.Contains(out, "end of statistics") {
		t.Fatal(out[len(out)-400:])
	}
	if !strings.Contains(out, "XXX stmts:") {
		t.Fatal("missing stmts counter")
	}
	// concise skips
	opts.Concise = true
	out2, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out2, "statistics *************************") {
		t.Fatal("concise should skip stats")
	}
}

func TestRecordPointerComparisons(t *testing.T) {
	BookkeeperDoFinalization()
	pt := PointerTo(GetIntType())
	lhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_p", pt, false, false), ExprType: pt}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}, ExprType: pt}
	RecordPointerComparisons(lhs, rhs)
	if cmpPtrToNull != 1 {
		t.Fatal(cmpPtrToNull)
	}
	// incomplete type IR sticky must not invent ptr-vs-ptr via level 0
	BookkeeperDoFinalization()
	ClearError()
	broken := &Expression{Term: TermVariable, Var: &Variable{Name: "x"}, ExprType: pt}
	good := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_q", pt, false, false), ExprType: pt}
	RecordPointerComparisons(broken, good)
	if cmpPtrToPtr != 0 || cmpPtrToAddr != 0 {
		t.Fatal("incomplete IndirectLevel must not invent compare counts", cmpPtrToPtr, cmpPtrToAddr)
	}
	if !HasError() {
		t.Fatal("incomplete IndirectLevel RecordPointerComparisons must SetError sticky")
	}
	ClearError()
}

func TestStatBlkDepthsUsesGetBlkDepth(t *testing.T) {
	// Bookkeeper.cpp:128–140 — depth = get_blk_depth()-1 via Parent chain
	BookkeeperDoFinalization()
	outer := &Block{}
	inner := &Block{Parent: outer}
	outer.Parent = nil
	// body = outer with if whose Then is inner (Else empty but live)
	f := &Function{Name: "func_1", Body: outer}
	// assign at body depth (parent=outer → depth 1 → bucket 0)
	// assign inside then (parent=inner → depth 2 → bucket 1)
	// StatementIf always has both arms
	outer.Stmts = []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtIfElse, StmID: 2, Then: inner, Else: &Block{Parent: outer}},
	}
	inner.Stmts = []Stmt{{Kind: StmtAssign, StmID: 3}}
	n := StatBlkDepths([]*Function{f})
	// 3 non-block stmts: assign, if, assign
	if n != 3 {
		t.Fatalf("count %d", n)
	}
	if len(blkDepthCnts) < 2 || blkDepthCnts[0] < 2 {
		// body-level: assign + if at bucket 0
		t.Fatalf("depth0 %+v", blkDepthCnts)
	}
	if blkDepthCnts[1] < 1 {
		t.Fatalf("depth1 %+v", blkDepthCnts)
	}
}

func TestStatExprDepthsIncompleteExprFailClosed(t *testing.T) {
	// incomplete IR sticky clear depths (no invent leaf 0 / soft re-pick)
	ClearError()
	exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, Expr: &Expression{Term: TermFunction}}, // nil Invoke
	}}}
	StatExprDepths([]*Function{f})
	if exprDepthCnts != nil {
		t.Fatal("incomplete expr must clear depth counts, not invent leaf 0", exprDepthCnts)
	}
	if !HasError() {
		t.Fatal("incomplete expr StatExprDepths must SetError sticky")
	}
	ClearError()
	// incomplete Funcs list sticky
	exprDepthCnts = []int{99}
	good := &Function{Name: "g", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}},
	}}}
	StatExprDepths([]*Function{nil, good})
	if exprDepthCnts != nil {
		t.Fatal("Funcs hole must clear depths, not invent count past hole", exprDepthCnts)
	}
	if !HasError() {
		t.Fatal("Funcs hole StatExprDepths must SetError sticky")
	}
	ClearError()
	if StatBlkDepths([]*Function{nil, good}) != 0 || blkDepthCnts != nil {
		t.Fatal("Funcs hole must zero StatBlkDepths")
	}
	if !HasError() {
		t.Fatal("Funcs hole StatBlkDepths must SetError sticky")
	}
	ClearError()
}

func TestStatExprDepthsForTestExpr(t *testing.T) {
	// StatementFor::get_exprs pushes test; soft invent skip would leave counts empty
	ClearError()
	BookkeeperDoFinalization()
	test := &Expression{Term: TermConstant, Con: MakeInt(1)}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtFor, Loop: &LoopControl{TestExpr: test}, Then: &Block{}},
	}}}
	StatExprDepths([]*Function{f})
	if len(exprDepthCnts) < 1 || exprDepthCnts[0] < 1 {
		t.Fatalf("for-test must count: %+v", exprDepthCnts)
	}
}

func TestStatExprDepthsForMissingTestFailClosed(t *testing.T) {
	// incomplete for Loop without TestExpr sticky clear
	ClearError()
	exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtFor, Loop: &LoopControl{IV: CreateVariableScalars("i", GetIntType(), false, false)}, Then: &Block{}},
	}}}
	StatExprDepths([]*Function{f})
	if exprDepthCnts != nil {
		t.Fatal("for without TestExpr must fail closed clear, not invent skip", exprDepthCnts)
	}
	if !HasError() {
		t.Fatal("for without TestExpr StatExprDepths must SetError sticky")
	}
	ClearError()
}

func TestStatExprDepthsAssignNilExprFailClosed(t *testing.T) {
	// C++ get_exprs always live for assign; nil Expr sticky clear
	ClearError()
	exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
	}}}
	StatExprDepths([]*Function{f})
	if exprDepthCnts != nil {
		t.Fatal("assign without Expr must fail closed clear depths", exprDepthCnts)
	}
	if !HasError() {
		t.Fatal("assign without Expr StatExprDepths must SetError sticky")
	}
	ClearError()
}
