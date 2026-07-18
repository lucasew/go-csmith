package csmith

import "testing"

func TestConstantEquals(t *testing.T) {
	if !MakeInt(0).Equals(0) {
		t.Fatal("0")
	}
	if MakeInt(3).Equals(0) {
		t.Fatal("3")
	}
}

func TestRhsToLhsTransferNullConst(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || !facts[0].IsNull() {
		t.Fatalf("%+v", facts)
	}
}

func TestRhsToLhsTransferAddrOf(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	// ExpressionVariable with ExprType = pointer means &g_t if var is int?
	// IndirectLevel = var.level - exprType.level; int(0) - ptr(1) = -1 → address-of
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	if rhs.IndirectLevel() != -1 {
		t.Fatalf("indir %d", rhs.IndirectLevel())
	}
	facts := RhsToLhsTransfer(nil, []*Variable{p}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts[0])
	}
}

func TestRhsToLhsTransferCopy(t *testing.T) {
	p1 := CreateVariableScalars("g_p1", PointerTo(GetIntType()), false, false)
	p2 := CreateVariableScalars("g_p2", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	in := []*FactPointTo{MakeFactPointTo(p2, tgt)}
	rhs := &Expression{Term: TermVariable, Var: p2, ExprType: PointerTo(GetIntType())}
	// p2 level 1, expr 1 → indirect 0; merge uses fact of p2
	facts := RhsToLhsTransfer(in, []*Variable{p1}, rhs)
	if len(facts) != 1 || facts[0].PointTo[0] != tgt {
		t.Fatalf("%+v", facts)
	}
}

func TestUpdateFactForAssign(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.AddNewVarFact(p)
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsDead() {
		t.Fatal("init")
	}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	if !fm.UpdateFactForAssign(p, 0, rhs) {
		t.Fatal("update")
	}
	if !FindRelatedPointTo(fm.GlobalFacts, p).IsNull() {
		t.Fatal("null after assign")
	}
}

func TestAbstractFactNonPointerLHS(t *testing.T) {
	v := CreateVariableScalars("g_i", GetIntType(), false, false)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if AbstractFactForAssign(nil, v, 0, rhs) != nil {
		t.Fatal("non-ptr")
	}
}
