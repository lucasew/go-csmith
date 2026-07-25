package csmith

import "testing"

func TestDistributionTableRndNumToKey(t *testing.T) {
	// Expression.cpp weights: F70 V20 C10 A10 Cma10 → max 120
	var d DistributionTable
	d.AddEntrySess(testAmbientSession, int(TermFunction), 70)
	d.AddEntrySess(testAmbientSession, int(TermVariable), 20)
	d.AddEntrySess(testAmbientSession, int(TermConstant), 10)
	d.AddEntrySess(testAmbientSession, int(TermAssignment), 10)
	d.AddEntrySess(testAmbientSession, int(TermCommaExpr), 10)
	if d.MaxSess(testAmbientSession) != 120 {
		t.Fatalf("max %d", d.MaxSess(testAmbientSession))
	}
	// rnd 0..69 → Function
	if d.RndNumToKeySess(testAmbientSession, 0) != int(TermFunction) || d.RndNumToKeySess(testAmbientSession, 69) != int(TermFunction) {
		t.Fatal("function band")
	}
	// 70..89 → Variable
	if d.RndNumToKeySess(testAmbientSession, 70) != int(TermVariable) || d.RndNumToKeySess(testAmbientSession, 89) != int(TermVariable) {
		t.Fatal("variable band")
	}
	// 90..99 → Constant
	if d.RndNumToKeySess(testAmbientSession, 90) != int(TermConstant) {
		t.Fatal("constant band")
	}
	// 100..109 → Assign
	if d.RndNumToKeySess(testAmbientSession, 100) != int(TermAssignment) {
		t.Fatal("assign band")
	}
	// 110..119 → Comma
	if d.RndNumToKeySess(testAmbientSession, 110) != int(TermCommaExpr) {
		t.Fatal("comma band")
	}
}

func TestThresholdNumberToType(t *testing.T) {
	// defaults: jumps+arrays
	tab := NewStatementThresholdTableSess(testAmbientSession, Defaults())
	// 0..14 IfElse, 15..29 For, …
	if NumberToTypeSess(testAmbientSession, tab, 0) != StmtIfElse || NumberToTypeSess(testAmbientSession, tab, 14) != StmtIfElse {
		t.Fatal("ifelse")
	}
	if NumberToTypeSess(testAmbientSession, tab, 15) != StmtFor || NumberToTypeSess(testAmbientSession, tab, 29) != StmtFor {
		t.Fatal("for")
	}
	if NumberToTypeSess(testAmbientSession, tab, 30) != StmtReturn {
		t.Fatal("return")
	}
	if NumberToTypeSess(testAmbientSession, tab, 35) != StmtContinue {
		t.Fatal("continue")
	}
	if NumberToTypeSess(testAmbientSession, tab, 40) != StmtBreak {
		t.Fatal("break")
	}
	if NumberToTypeSess(testAmbientSession, tab, 45) != StmtGoto {
		t.Fatal("goto")
	}
	if NumberToTypeSess(testAmbientSession, tab, 50) != StmtArrayOp || NumberToTypeSess(testAmbientSession, tab, 59) != StmtArrayOp {
		t.Fatal("arrayop")
	}
	if NumberToTypeSess(testAmbientSession, tab, 60) != StmtAssign || NumberToTypeSess(testAmbientSession, tab, 99) != StmtAssign {
		t.Fatal("assign")
	}
}

func TestThresholdTableNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	(*ThresholdTable)(nil).AddSess(testAmbientSession, 1, 2)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ThresholdTable Add must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*ThresholdTable)(nil).GetValueSess(testAmbientSession, 0) != -1 {
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
	d.AddEntrySess(testAmbientSession, 10, 70)
	d.AddEntrySess(testAmbientSession, 20, 0) // weight 0 still recorded (C++ always push)
	d.AddEntrySess(testAmbientSession, 30, 30)
	if d.MaxSess(testAmbientSession) != 100 {
		t.Fatalf("max with zero-weight entry: got %d want 100", d.MaxSess(testAmbientSession))
	}
	if d.KeyToProbSess(testAmbientSession, 10) != 70 || d.KeyToProbSess(testAmbientSession, 20) != 0 || d.KeyToProbSess(testAmbientSession, 30) != 30 {
		t.Fatalf("key_to_prob: 10=%d 20=%d 30=%d", d.KeyToProbSess(testAmbientSession, 10), d.KeyToProbSess(testAmbientSession, 20), d.KeyToProbSess(testAmbientSession, 30))
	}
	if d.KeyToProbSess(testAmbientSession, 99) != 0 {
		t.Fatal("missing key → 0")
	}
	// zero-weight key never selected by rnd_num_to_key
	if d.RndNumToKeySess(testAmbientSession, 0) != 10 || d.RndNumToKeySess(testAmbientSession, 69) != 10 {
		t.Fatal("band before zero-weight")
	}
	if d.RndNumToKeySess(testAmbientSession, 70) != 30 {
		t.Fatal("after zero-weight entry, next band is 30")
	}
}

func TestDistributionTableNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	(*DistributionTable)(nil).AddEntrySess(testAmbientSession, 1, 10)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil DistributionTable AddEntry must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).MaxSess(testAmbientSession) != 0 {
		t.Fatal("nil Max must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Max must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).RndNumToKeySess(testAmbientSession, 0) != -1 {
		t.Fatal("nil RndNumToKey must return -1")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RndNumToKey must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*DistributionTable)(nil).KeyToProbSess(testAmbientSession, 1) != 0 {
		t.Fatal("nil KeyToProb must return 0")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil KeyToProb must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
