package csmith

import "testing"

func TestDistributionTableRndNumToKey(t *testing.T) {
	// Expression.cpp weights: F70 V20 C10 A10 Cma10 → max 120
	var d DistributionTable
	d.AddEntry(int(TermFunction), 70)
	d.AddEntry(int(TermVariable), 20)
	d.AddEntry(int(TermConstant), 10)
	d.AddEntry(int(TermAssignment), 10)
	d.AddEntry(int(TermCommaExpr), 10)
	if d.Max() != 120 {
		t.Fatalf("max %d", d.Max())
	}
	// rnd 0..69 → Function
	if d.RndNumToKey(0) != int(TermFunction) || d.RndNumToKey(69) != int(TermFunction) {
		t.Fatal("function band")
	}
	// 70..89 → Variable
	if d.RndNumToKey(70) != int(TermVariable) || d.RndNumToKey(89) != int(TermVariable) {
		t.Fatal("variable band")
	}
	// 90..99 → Constant
	if d.RndNumToKey(90) != int(TermConstant) {
		t.Fatal("constant band")
	}
	// 100..109 → Assign
	if d.RndNumToKey(100) != int(TermAssignment) {
		t.Fatal("assign band")
	}
	// 110..119 → Comma
	if d.RndNumToKey(110) != int(TermCommaExpr) {
		t.Fatal("comma band")
	}
}

func TestThresholdNumberToType(t *testing.T) {
	// defaults: jumps+arrays
	tab := NewStatementThresholdTable(Defaults())
	// 0..14 IfElse, 15..29 For, …
	if NumberToType(tab, 0) != StmtIfElse || NumberToType(tab, 14) != StmtIfElse {
		t.Fatal("ifelse")
	}
	if NumberToType(tab, 15) != StmtFor || NumberToType(tab, 29) != StmtFor {
		t.Fatal("for")
	}
	if NumberToType(tab, 30) != StmtReturn {
		t.Fatal("return")
	}
	if NumberToType(tab, 35) != StmtContinue {
		t.Fatal("continue")
	}
	if NumberToType(tab, 40) != StmtBreak {
		t.Fatal("break")
	}
	if NumberToType(tab, 45) != StmtGoto {
		t.Fatal("goto")
	}
	if NumberToType(tab, 50) != StmtArrayOp || NumberToType(tab, 59) != StmtArrayOp {
		t.Fatal("arrayop")
	}
	if NumberToType(tab, 60) != StmtAssign || NumberToType(tab, 99) != StmtAssign {
		t.Fatal("assign")
	}
}

func TestThresholdTableNilSticky(t *testing.T) {
	ClearError()
	(*ThresholdTable)(nil).Add(1, 2)
	if !HasError() {
		t.Fatal("nil ThresholdTable Add must SetError sticky")
	}
	ClearError()
	if (*ThresholdTable)(nil).GetValue(0) != -1 {
		t.Fatal("nil GetValue must return -1")
	}
	if !HasError() {
		t.Fatal("nil GetValue must SetError sticky")
	}
	ClearError()
}
