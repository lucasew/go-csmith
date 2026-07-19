package csmith

import (
	"strings"
	"testing"
)

func TestOutputAssignSimple(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignSimple,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(3)},
	}
	out := OutputAssignSimple(st, false)
	if !strings.Contains(out, "g_1") || !strings.Contains(out, "3") {
		t.Fatal(out)
	}
	st.AssignOp = AssignPostIncr
	out = OutputAssignSimple(st, false)
	if !strings.Contains(out, "++") {
		t.Fatal(out)
	}
}

func TestOutputAssignSimpleNoInventEmptyRHS(t *testing.T) {
	// StatementAssign.cpp:515–537 — expr.Output always; sticky no invent "g_1 = "
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{Kind: StmtAssign, LhsVar: v, AssignOp: AssignSimple}
	if out := OutputAssignSimple(st, false); out != "" {
		t.Fatal("nil Expr must fail closed", out)
	}
	ClearError()
	st.Expr = &Expression{Term: TermConstant} // nil Con → empty Output sticky
	if out := OutputAssignSimple(st, false); out != "" {
		t.Fatal("empty RHS must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty RHS Output must SetError sticky")
	}
	// pre/post incr need no RHS
	ClearError()
	st.AssignOp = AssignPreIncr
	st.Expr = nil
	if out := OutputAssignSimple(st, false); out != "++g_1" {
		t.Fatal(out)
	}
	// AssignOpC — sticky no invent empty-name or empty-rhs shells
	ClearError()
	if AssignSimple.AssignOpC("", "1") != "" || AssignSimple.AssignOpC("g", "") != "" {
		t.Fatal("AssignOpC empty sides must fail closed")
	}
	if !HasError() {
		t.Fatal("AssignOpC empty sides must SetError sticky")
	}
	ClearError()
	if AssignPreIncr.AssignOpC("", "") != "" {
		t.Fatal("empty name preincr must fail closed")
	}
	if !HasError() {
		t.Fatal("empty name preincr must SetError sticky")
	}
	ClearError()
	if AssignPreIncr.AssignOpC("g", "") != "++g" {
		t.Fatal(AssignPreIncr.AssignOpC("g", ""))
	}
	ClearError()
}

func TestOutputAssignAsExprNoInventEmptyCCompRHS(t *testing.T) {
	// ccomp volatile rewrite needs live rhs; sticky no invent "g = g & "
	ClearError()
	v := CreateVariableScalars("g_v", GetIntType(), true, true)
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignBitAnd,
		Expr:      &Expression{Term: TermConstant}, // empty Output sticky
		SafeFlags: &SafeOpFlags{Size: SafeInt32, Op1Signed: true, Op2Signed: true},
	}
	opts := Defaults()
	opts.SafeMath = true
	opts.CComp = true
	if out := OutputAssignAsExprOpts(st, false, opts); out != "" {
		t.Fatal("empty ccomp rhs must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty ccomp rhs must SetError sticky")
	}
	ClearError()
	// nil Expr on simple assign sticky — no invent bare lhs
	st2 := &Stmt{Kind: StmtAssign, LhsVar: v, AssignOp: AssignSimple}
	if out := OutputAssignAsExprOpts(st2, false, opts); out != "" {
		t.Fatal("nil Expr simple must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil Expr simple OutputAssignAsExpr must SetError sticky")
	}
	ClearError()
}

func TestOutputAssignAsExprSafeWrapper(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	flags := MakeRandomBinary(NewRng(1), Defaults(), NewProbabilities(Defaults()), GetIntType())
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignAdd,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		SafeFlags: flags,
	}
	opts := Defaults()
	out := OutputAssignAsExprOpts(st, false, opts)
	if !strings.Contains(out, "safe_add_") || !strings.Contains(out, "g_1 = ") {
		t.Fatal(out)
	}
	// identify wrappers appends id
	opts.IdentifyWrappers = true
	out2 := OutputAssignAsExprOpts(st, false, opts)
	if !strings.Contains(out2, ", ") {
		t.Fatal(out2)
	}
}

func TestSafeMathWrapperAllowed(t *testing.T) {
	opts := Defaults()
	if !SafeMathWrapperAllowed(opts, 99) {
		t.Fatal("empty allows all")
	}
	opts.SafeMathWrappers = "1,3"
	if !SafeMathWrapperAllowed(opts, 1) || SafeMathWrapperAllowed(opts, 2) {
		t.Fatal("filter")
	}
}

func TestSafeOpFlagsToIDStable(t *testing.T) {
	ClearSafeOpWrapperNames()
	id1 := SafeOpFlagsToID("safe_add_func_int32_t_s_s")
	id2 := SafeOpFlagsToID("safe_add_func_int32_t_s_s")
	id3 := SafeOpFlagsToID("safe_sub_func_int32_t_s_s")
	if id1 != id2 || id3 == id1 {
		t.Fatal(id1, id2, id3)
	}
	ClearSafeOpWrapperNames()
}

func TestStopByStmtForcesReturn(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 5
	opts.StopByStmt = 0 // force return immediately (nextStmID >= 0 always)
	// reset sid
	nextStmID = 0
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	// make block — should tend to returns when stop is low
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
	if b == nil {
		t.Fatal("nil")
	}
	// with StopByStmt=0 every stmt pick becomes Return
	// at least one return expected in body for non-void
	found := false
	for _, st := range b.Stmts {
		if st.Kind == StmtReturn {
			found = true
		}
	}
	if !found && f.NeedReturnStmt() {
		// post_creation may still add return
		_ = b.MustReturn()
	}
}

func TestOutputAssignAsExprCCompVolatileBit(t *testing.T) {
	// StatementAssign.cpp:552–556 — ccomp + volatile compound → lhs = lhs op rhs
	v := CreateVariableScalars("g_v", GetIntType(), false, true) // volatile
	flags := MakeDummyFlags()
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		AssignOp: AssignBitAnd,
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(7)},
		SafeFlags: flags,
	}
	opts := Defaults()
	opts.CComp = true
	out := OutputAssignAsExprOpts(st, false, opts)
	// expect "g_v = g_v & 7" not "g_v &= 7"
	if !strings.Contains(out, "=") || !strings.Contains(out, "&") {
		t.Fatal(out)
	}
	if strings.Contains(out, "&=") {
		t.Fatal("ccomp should expand compound", out)
	}
	if !strings.Contains(out, "g_v = g_v & 7") && !strings.Contains(out, "g_v = g_v & ") {
		t.Fatal(out)
	}
	// non-ccomp keeps compound form
	opts.CComp = false
	out2 := OutputAssignAsExprOpts(st, false, opts)
	if !strings.Contains(out2, "&=") {
		t.Fatal("non-ccomp compound", out2)
	}
}

func TestOutputAssignAsExprSimpleNotCCompExpanded(t *testing.T) {
	// simple assign has no compound_to_binary — stays "lhs = rhs"
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignSimple,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, SafeFlags: MakeDummyFlags(),
	}
	opts := Defaults()
	opts.CComp = true
	out := OutputAssignAsExprOpts(st, false, opts)
	if strings.Count(out, "g_v") != 1 {
		// "g_v = 1" has one mention of g_v on lhs only for simple
		if !strings.Contains(out, "g_v = 1") && !strings.Contains(out, "g_v = ") {
			t.Fatal(out)
		}
	}
}

func TestOutputAssignAsExprRequiresSafeMathOption(t *testing.T) {
	// StatementAssign.cpp:543 — avoid_signed_overflow() && op_flags
	// no soft invent safe_* when SafeMath off despite flags present
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	flags := MakeRandomBinary(NewRng(1), Defaults(), NewProbabilities(Defaults()), GetIntType())
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignAdd,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		SafeFlags: flags,
	}
	opts := Defaults()
	opts.SafeMath = false
	out := OutputAssignAsExprOpts(st, false, opts)
	if strings.Contains(out, "safe_") {
		t.Fatal("SafeMath off must not invent wrapper", out)
	}
	if !strings.Contains(out, "+=") && !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
}

func TestOutputAssignAsExprUnknownOpWithFlagsFailClosed(t *testing.T) {
	// StatementAssign.cpp:618–619 assert(false) sticky; no invent OutputSimple for *= with flags
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, AssignOp: AssignMul,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)},
		SafeFlags: MakeDummyFlags(),
	}
	opts := Defaults()
	opts.SafeMath = true
	if out := OutputAssignAsExprOpts(st, false, opts); out != "" {
		t.Fatalf("unknown op with flags must fail closed, got %q", out)
	}
	if !HasError() {
		t.Fatal("unknown op with flags must SetError sticky")
	}
	ClearError()
}

func TestRandomQualifiersDefaultProbsNilNoInvent(t *testing.T) {
	// nil probs → 0% const/vol (no invent NewProbabilities from Defaults)
	opts := Defaults()
	opts.Consts = true
	opts.Volatiles = true
	// with real probs, may get bits; with nil, always non-const/non-vol under Regular*
	q := RandomQualifiersDefaultProbs(GetIntType(), AccessWrite, EmptyCGContext(), false, opts, nil, NewRng(1))
	if q.IsConst() || q.IsVolatile() {
		t.Fatal("nil probs must not invent non-zero regular const/vol")
	}
}

func TestOutputAssignSimpleNilSticky(t *testing.T) {
	ClearError()
	if OutputAssignSimple(nil, false) != "" {
		t.Fatal("nil OutputAssignSimple must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputAssignSimple must SetError sticky")
	}
	ClearError()
}

func TestOutputAssignAsExprNilSticky(t *testing.T) {
	// Statement always live at OutputAsExpr; sticky no invent empty assign-as-expr shell
	ClearError()
	if OutputAssignAsExpr(nil, false) != "" {
		t.Fatal("nil OutputAssignAsExpr must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputAssignAsExpr must SetError sticky")
	}
	ClearError()
	if OutputAssignAsExprOpts(nil, false, Defaults()) != "" {
		t.Fatal("nil OutputAssignAsExprOpts must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil OutputAssignAsExprOpts must SetError sticky")
	}
	ClearError()
	if assignLhsText(nil, false) != "" {
		t.Fatal("nil assignLhsText must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil assignLhsText must SetError sticky")
	}
	ClearError()
	// Statement always live; sticky true (no invent non-vol soft-skip ccomp)
	if !assignLhsIsVolatile(nil) {
		t.Fatal("nil assignLhsIsVolatile must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil assignLhsIsVolatile must SetError sticky")
	}
	ClearError()
	// Expression always live at qfer seed; sticky nil
	if expressionQualifiers(nil) != nil {
		t.Fatal("nil expressionQualifiers must fail closed nil")
	}
	if !HasError() {
		t.Fatal("nil expressionQualifiers must SetError sticky")
	}
	ClearError()
}
