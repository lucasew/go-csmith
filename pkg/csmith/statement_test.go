package csmith

import "testing"

func TestStatementProbabilitySeed2(t *testing.T) {
	tab := NewStatementThresholdTable(Defaults())
	r := NewRng(2)
	// first RndUpto(100) seed2 = 1959434203 % 100 = 3 → IfElse
	st := StatementProbability(r, tab)
	if st != StmtIfElse {
		t.Fatalf("got %v want IfElse", st)
	}
}

func TestStatementProbabilityFilterRejectCompound(t *testing.T) {
	// Simulate max block depth: reject compound types via custom filter on the U100 value.
	tab := NewStatementThresholdTable(Defaults())
	// Filter that rejects values mapping to compound statements
	f := filterFunc(func(v uint32) bool {
		return IsCompound(NumberToType(tab, v))
	})
	r := NewRng(2)
	for i := 0; i < 30; i++ {
		st := StatementProbabilityFilter(r, tab, f)
		if IsCompound(st) {
			t.Fatalf("compound slipped through: %v", st)
		}
	}
}

func TestIsCompound(t *testing.T) {
	if !IsCompound(StmtFor) || IsCompound(StmtAssign) {
		t.Fatal("is_compound")
	}
}
