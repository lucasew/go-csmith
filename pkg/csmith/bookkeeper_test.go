package csmith

import (
	"strings"
	"testing"
)

func TestIncrCounterAndCalcTotal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
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
	// Counters always live; sticky (no invent soft-skip stats past hole)
	IncrCounter(nil, 0)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil counters IncrCounter must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// pos < 0 complete no-op
	IncrCounter(&c, -1)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("neg pos IncrCounter must complete no-op")
	}
	if len(c) != 3 {
		t.Fatal("neg pos must not mutate", c)
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordAddressTaken(t *testing.T) {
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	RecordAddressTaken(v)
	if !v.IsAddrTaken || currentSession().BK.addressTakenCnt != 1 {
		t.Fatal(currentSession().BK.addressTakenCnt, v.IsAddrTaken)
	}
	// Bookkeeper.cpp:325–326 assert(var/type) sticky
	RecordAddressTaken(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	RecordAddressTaken(&Variable{Name: "x", Type: nil})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// HasBitfields residual soft invent was soft-count / soft-skip complete stats.
	// Fair: sticky stop (no invent partial bitfield address-taken count past hole).
	BookkeeperDoFinalization()
	holeTy := &Type{isStruct: true, Fields: []StructField{{Type: nil, BitWidth: -1}}}
	hv := &Variable{Name: "g_h", Type: holeTy}
	before := currentSession().BK.varsWithBitfieldsAddressTakenCnt
	RecordAddressTaken(hv)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasBitfields residual RecordAddressTaken must SetError sticky")
	}
	if currentSession().BK.varsWithBitfieldsAddressTakenCnt != before {
		t.Fatal("HasBitfields residual must not invent bitfield address-taken count")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordVolatileAccess(t *testing.T) {
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	RecordVolatileAccess(v, 0, false)
	if currentSession().BK.readVolatileCnt != 1 {
		t.Fatal(currentSession().BK.readVolatileCnt)
	}
	RecordVolatileAccess(v, 0, true)
	if currentSession().BK.writeVolatileCnt != 1 {
		t.Fatal(currentSession().BK.writeVolatileCnt)
	}
	nv := CreateVariableScalars("g_n", GetIntType(), false, false)
	RecordVolatileAccess(nv, 0, false)
	if currentSession().BK.readNonVolatileCnt != 1 {
		t.Fatal(currentSession().BK.readNonVolatileCnt)
	}
	// Bookkeeper.cpp:388 assert(var) sticky
	RecordVolatileAccess(nil, 0, false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVolatileAfterDeref residual soft invent was soft-continue non-vol peel stats.
	// Fair: sticky stop (Type-nil peel residual).
	BookkeeperDoFinalization()
	hole := &Variable{Name: "g_p", Type: nil, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	beforeR := currentSession().BK.readVolatileCnt + currentSession().BK.readNonVolatileCnt
	RecordVolatileAccess(hole, 0, false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVolatileAfterDeref residual RecordVolatileAccess must SetError sticky")
	}
	if currentSession().BK.readVolatileCnt+currentSession().BK.readNonVolatileCnt != beforeR {
		t.Fatal("IsVolatileAfterDeref residual must not invent peel access counts")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordJumpsAndVarFreshness(t *testing.T) {
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	RecordForwardJump()
	RecordBackwardJump()
	RecordBackwardJump()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	RecordVarCreated(v)
	RecordVarReused()
	if currentSession().BK.forwardJumpCnt != 1 || currentSession().BK.backwardJumpCnt != 2 {
		t.Fatal(currentSession().BK.forwardJumpCnt, currentSession().BK.backwardJumpCnt)
	}
	if currentSession().BK.useNewVarCnt != 1 || currentSession().BK.useOldVarCnt != 1 {
		t.Fatal(currentSession().BK.useNewVarCnt, currentSession().BK.useOldVarCnt)
	}
	out := OutputStatistics(nil, Defaults())
	if !strings.Contains(out, "forward jumps: 1") || !strings.Contains(out, "backward jumps: 2") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "percentage a fresh-made variable is used: 50") {
		t.Fatal(out)
	}
	// Variable + Type always live; sticky (no invent soft-skip create stats past hole)
	RecordVarCreated(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var RecordVarCreated must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	RecordVarCreated(&Variable{Name: "x", Type: nil})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type RecordVarCreated must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil builder sticky (no invent silent skip of stats lines / undercount)
	formattedOutput(nil, "x: ", 1)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil builder formattedOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	formattedOutputf(nil, "x: ", 1.0)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil builder formattedOutputf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordBitfieldsAndPointerCmpSticky(t *testing.T) {
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	// Variable + Type always live; sticky
	RecordBitfieldsReads(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var RecordBitfieldsReads must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	RecordBitfieldsWrites(&Variable{Name: "x", Type: nil})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type RecordBitfieldsWrites must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Expression operands always live; sticky
	RecordPointerComparisons(nil, &Expression{Term: TermConstant})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs RecordPointerComparisons must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// HasBitfields residual soft invent was soft-count / soft-skip complete stats.
	// Fair: sticky stop.
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	holeTy := &Type{isStruct: true, Fields: []StructField{{Type: nil, BitWidth: -1}}}
	beforeS := currentSession().BK.structsWithBitfields
	RecordTypeWithBitfields(holeTy)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("HasBitfields residual RecordTypeWithBitfields must SetError sticky")
	}
	if currentSession().BK.structsWithBitfields != beforeS {
		t.Fatal("HasBitfields residual must not invent currentSession().BK.structsWithBitfields count")
	}
	ClearErrorSess(testAmbientSession)
	// Type always live; sticky — non-aggregate complete no-op
	RecordTypeWithBitfields(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type RecordTypeWithBitfields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	RecordTypeWithBitfields(GetIntType())
	if HasErrorSess(testAmbientSession) {
		t.Fatal("non-aggregate RecordTypeWithBitfields must complete no-op")
	}
	ClearErrorSess(testAmbientSession)
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

func TestOutputStatisticsStatResidualSticky(t *testing.T) {
	// Stat residual soft invent was soft-continue later stats invent partial OutputStatistics.
	ClearErrorSess(testAmbientSession)
	// incomplete Funcs → StatExprDepths stickies ERROR
	if s := OutputStatistics([]*Function{nil}, Defaults()); s != "" {
		t.Fatal("Stat residual must fail closed OutputStatistics, not invent partial", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Stat residual OutputStatistics must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := OutputTail([]*Function{nil}, Defaults()); s != "" {
		t.Fatal("Stat residual must fail closed OutputTail, not invent stats shell", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Stat residual OutputTail must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordPointerComparisons(t *testing.T) {
	BookkeeperDoFinalization()
	pt := PointerTo(GetIntType())
	lhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_p", pt, false, false), ExprType: pt}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}, ExprType: pt}
	RecordPointerComparisons(lhs, rhs)
	if currentSession().BK.cmpPtrToNull != 1 {
		t.Fatal(currentSession().BK.cmpPtrToNull)
	}
	// incomplete type IR sticky must not invent ptr-vs-ptr via level 0
	BookkeeperDoFinalization()
	ClearErrorSess(testAmbientSession)
	broken := &Expression{Term: TermVariable, Var: &Variable{Name: "x"}, ExprType: pt}
	good := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_q", pt, false, false), ExprType: pt}
	RecordPointerComparisons(broken, good)
	if currentSession().BK.cmpPtrToPtr != 0 || currentSession().BK.cmpPtrToAddr != 0 {
		t.Fatal("incomplete IndirectLevel must not invent compare counts", currentSession().BK.cmpPtrToPtr, currentSession().BK.cmpPtrToAddr)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete IndirectLevel RecordPointerComparisons must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// GetType residual soft invent was soft-skip non-pointer then invent complete stats no-op.
	BookkeeperDoFinalization()
	typeHole := &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}
	RecordPointerComparisons(typeHole, good)
	if currentSession().BK.cmpPtrToNull != 0 || currentSession().BK.cmpPtrToPtr != 0 || currentSession().BK.cmpPtrToAddr != 0 {
		t.Fatal("GetType residual must not invent compare counts", currentSession().BK.cmpPtrToNull, currentSession().BK.cmpPtrToPtr, currentSession().BK.cmpPtrToAddr)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual RecordPointerComparisons must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	if len(currentSession().BK.blkDepthCnts) < 2 || currentSession().BK.blkDepthCnts[0] < 2 {
		// body-level: assign + if at bucket 0
		t.Fatalf("depth0 %+v", currentSession().BK.blkDepthCnts)
	}
	if currentSession().BK.blkDepthCnts[1] < 1 {
		t.Fatalf("depth1 %+v", currentSession().BK.blkDepthCnts)
	}
}

func TestStatExprDepthsIncompleteExprFailClosed(t *testing.T) {
	// incomplete IR sticky clear depths (no invent leaf 0 / soft re-pick)
	ClearErrorSess(testAmbientSession)
	currentSession().BK.exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, Expr: &Expression{Term: TermFunction}}, // nil Invoke
	}}}
	StatExprDepths([]*Function{f})
	if currentSession().BK.exprDepthCnts != nil {
		t.Fatal("incomplete expr must clear depth counts, not invent leaf 0", currentSession().BK.exprDepthCnts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete expr StatExprDepths must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Funcs list sticky
	currentSession().BK.exprDepthCnts = []int{99}
	good := &Function{Name: "g", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}},
	}}}
	StatExprDepths([]*Function{nil, good})
	if currentSession().BK.exprDepthCnts != nil {
		t.Fatal("Funcs hole must clear depths, not invent count past hole", currentSession().BK.exprDepthCnts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcs hole StatExprDepths must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if StatBlkDepths([]*Function{nil, good}) != 0 || currentSession().BK.blkDepthCnts != nil {
		t.Fatal("Funcs hole must zero StatBlkDepths")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Funcs hole StatBlkDepths must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStatExprDepthsForTestExpr(t *testing.T) {
	// StatementFor::get_exprs pushes test; soft invent skip would leave counts empty
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
	test := &Expression{Term: TermConstant, Con: MakeInt(1)}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtFor, Loop: &LoopControl{TestExpr: test}, Then: &Block{}},
	}}}
	StatExprDepths([]*Function{f})
	if len(currentSession().BK.exprDepthCnts) < 1 || currentSession().BK.exprDepthCnts[0] < 1 {
		t.Fatalf("for-test must count: %+v", currentSession().BK.exprDepthCnts)
	}
}

func TestStatExprDepthsForMissingTestFailClosed(t *testing.T) {
	// incomplete for Loop without TestExpr sticky clear
	ClearErrorSess(testAmbientSession)
	currentSession().BK.exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtFor, Loop: &LoopControl{IV: CreateVariableScalars("i", GetIntType(), false, false)}, Then: &Block{}},
	}}}
	StatExprDepths([]*Function{f})
	if currentSession().BK.exprDepthCnts != nil {
		t.Fatal("for without TestExpr must fail closed clear, not invent skip", currentSession().BK.exprDepthCnts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("for without TestExpr StatExprDepths must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStatExprDepthsAssignNilExprFailClosed(t *testing.T) {
	// C++ get_exprs always live for assign; nil Expr sticky clear
	ClearErrorSess(testAmbientSession)
	currentSession().BK.exprDepthCnts = []int{99}
	f := &Function{Name: "f", Body: &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
	}}}
	StatExprDepths([]*Function{f})
	if currentSession().BK.exprDepthCnts != nil {
		t.Fatal("assign without Expr must fail closed clear depths", currentSession().BK.exprDepthCnts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("assign without Expr StatExprDepths must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordVarCreatedStructDepthResidualSticky(t *testing.T) {
	// StructDepth residual soft invent was invent soft-count depth/union past hole.
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
	// Aggregate with nil field Type → StructDepth residual incompleteStructDepth + ERROR
	st := &Type{isStruct: true, StructName: "S0", Fields: []StructField{
		{Name: "f0", Type: nil, BitWidth: -1},
	}}
	v := &Variable{Name: "g_s", Type: st, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	before := currentSession().BK.useNewVarCnt
	RecordVarCreated(v)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StructDepth residual RecordVarCreated must SetError sticky")
	}
	// currentSession().BK.useNewVarCnt may increment before residual stop — fair fail-closed means no further invent
	if currentSession().BK.useNewVarCnt < before {
		t.Fatal("counter regression")
	}
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
}

func TestRecordTypeWithBitfieldsIsAggregateResidualSticky(t *testing.T) {
	// IsAggregate residual soft invent was invent soft-count bitfields past non-aggregate.
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
	// non-aggregate complete no-op
	RecordTypeWithBitfields(GetIntType())
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete non-aggregate RecordTypeWithBitfields must not sticky")
	}
	// nil sticky
	RecordTypeWithBitfields(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RecordTypeWithBitfields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
}

func TestRecordPointerComparisonsIsPointerLikeResidualSticky(t *testing.T) {
	// IsPointerLike residual soft invent was invent soft-skip then later cmp counts.
	// Type-nil GetType residual already sticky; complete non-pointer soft-skip hygiene.
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
	a := &Expression{Term: TermConstant, Con: MakeInt(1)}
	b := &Expression{Term: TermConstant, Con: MakeInt(2)}
	before := currentSession().BK.cmpPtrToNull + currentSession().BK.cmpPtrToPtr + currentSession().BK.cmpPtrToAddr
	RecordPointerComparisons(a, b)
	// non-pointer soft skip no count
	if currentSession().BK.cmpPtrToNull+currentSession().BK.cmpPtrToPtr+currentSession().BK.cmpPtrToAddr != before {
		t.Fatal("non-pointer must not invent cmp counts")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete non-pointer RecordPointerComparisons must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil GetType residual
	hole := &Expression{Term: TermVariable, Var: &Variable{Name: "g_p", Type: nil}}
	ok := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)}
	RecordPointerComparisons(hole, ok)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual RecordPointerComparisons must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	BookkeeperDoFinalization()
}
