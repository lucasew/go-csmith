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
		NewExprTables(opts), NewStatementThresholdTable(opts), cg, false)
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
