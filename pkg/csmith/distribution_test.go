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
	ClearErrorSess(testAmbientSession)
	(*ThresholdTable)(nil).Add(1, 2)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ThresholdTable Add must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*ThresholdTable)(nil).GetValue(0) != -1 {
		t.Fatal("nil GetValue must return -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GetValue must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestDistributionTableKeyToProb(t *testing.T) {
	// Probabilities.cpp:1031–1038 key_to_prob
	var d DistributionTable
	d.AddEntry(10, 70)
	d.AddEntry(20, 0) // weight 0 still recorded (C++ always push)
	d.AddEntry(30, 30)
	if d.Max() != 100 {
		t.Fatalf("max with zero-weight entry: got %d want 100", d.Max())
	}
	if d.KeyToProb(10) != 70 || d.KeyToProb(20) != 0 || d.KeyToProb(30) != 30 {
		t.Fatalf("key_to_prob: 10=%d 20=%d 30=%d", d.KeyToProb(10), d.KeyToProb(20), d.KeyToProb(30))
	}
	if d.KeyToProb(99) != 0 {
		t.Fatal("missing key → 0")
	}
	// zero-weight key never selected by rnd_num_to_key
	if d.RndNumToKey(0) != 10 || d.RndNumToKey(69) != 10 {
		t.Fatal("band before zero-weight")
	}
	if d.RndNumToKey(70) != 30 {
		t.Fatal("after zero-weight entry, next band is 30")
	}
}

func TestDistributionTableNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	(*DistributionTable)(nil).AddEntry(1, 10)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil DistributionTable AddEntry must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).Max() != 0 {
		t.Fatal("nil Max must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Max must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).RndNumToKey(0) != -1 {
		t.Fatal("nil RndNumToKey must return -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RndNumToKey must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).KeyToProb(1) != 0 {
		t.Fatal("nil KeyToProb must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil KeyToProb must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
