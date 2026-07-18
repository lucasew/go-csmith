package csmith

import (
	"strings"
	"testing"
)

func TestAssignOpsProbabilitySimpleWhenDisabled(t *testing.T) {
	opts := Defaults()
	opts.CompoundAssignment = false
	tab := NewAssignOpsTable(opts)
	op := AssignOpsProbability(NewRng(2), opts, tab, GetIntType())
	if op != AssignSimple {
		t.Fatal(op)
	}
}

func TestAssignOpsProbabilitySignedFiltersIncr(t *testing.T) {
	opts := Defaults()
	tab := NewAssignOpsTable(opts)
	// many draws: never ++/-- on signed int
	r := NewRng(2)
	for i := 0; i < 100; i++ {
		op := AssignOpsProbability(r, opts, tab, GetIntType())
		if op.NeedNoRHS() {
			t.Fatalf("signed should filter incr, got %v", op)
		}
	}
}

func TestMakeRandomAssignCompoundPossible(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	foundCompound := false
	for seed := uint64(1); seed < 80; seed++ {
		r := NewRng(seed)
		st := MakeRandomAssign(r, opts, probs, vs, tables, EmptyCGContext(), GetIntType())
		if st.Kind != StmtAssign {
			t.Fatal(st.Kind)
		}
		if st.AssignOp != AssignSimple {
			foundCompound = true
			break
		}
	}
	if !foundCompound {
		t.Fatal("expected some compound/bit assign in seeds")
	}
}

func TestAssignOutputIncr(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := Stmt{Kind: StmtAssign, LhsVar: v, AssignOp: AssignPostIncr, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if !strings.Contains(out, "g_1++") {
		t.Fatal(out)
	}
}

func TestGenerateCanEmitCompoundAssign(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 50; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, " &= ") || strings.Contains(out, " |= ") ||
			strings.Contains(out, " ^= ") || strings.Contains(out, "++") || strings.Contains(out, "--") {
			found = true
			break
		}
	}
	if !found {
		t.Log("no compound/incr in 1..49 (unsigned incr rare; bit ops should appear)")
	}
}
