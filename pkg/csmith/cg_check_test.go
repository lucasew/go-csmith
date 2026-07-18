package csmith

import (
	"testing"
)

func TestCheckReadVarBasic(t *testing.T) {
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if !cg.CheckReadVar(v, nil) {
		t.Fatal("read ok")
	}
	if !eff.IsRead(v) {
		t.Fatal("not recorded")
	}
}

func TestCheckReadVarNonReadable(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext().WithRW(&RWDirective{NoReadVars: []*Variable{v}})
	if cg.CheckReadVar(v, nil) {
		t.Fatal("should reject")
	}
}

func TestCheckWriteVarConst(t *testing.T) {
	v := CreateVariableScalars("g_c", GetIntType(), true, false)
	cg := EmptyCGContext()
	if cg.CheckWriteVar(v, nil) {
		t.Fatal("const write")
	}
}

func TestCheckWriteVarIVBound(t *testing.T) {
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	cg := EmptyCGContext()
	cg.AddIVBound(iv, 10)
	if cg.CheckWriteVar(iv, nil) {
		t.Fatal("iv write")
	}
}

func TestCheckReadVarVolatileNeedsSEFree(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	// non-SE-free context rejects volatile read
	cg := WithEffectContext(WithSideEffects())
	if cg.CheckReadVar(v, nil) {
		t.Fatal("volatile in SE context")
	}
	// SE-free allows
	cg2 := EmptyCGContext()
	if !cg2.CheckReadVar(v, nil) {
		t.Fatal("volatile SE-free")
	}
}

func TestCheckWriteVarPartialConflict(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	eff := EmptyEffect().WriteVar(v)
	cg := WithEffectContext(eff)
	if cg.CheckWriteVar(v, nil) {
		t.Fatal("partial write conflict")
	}
	if cg.CheckReadVar(v, nil) {
		t.Fatal("partial write blocks read")
	}
}

func TestVisitFactsLhsScalar(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsWritten(v) {
		t.Fatal("write")
	}
}

func TestVisitFactsExpressionVariableAddrBitfield(t *testing.T) {
	v := CreateVariableScalars("g_bf", GetIntType(), false, false)
	v.IsBitfield = true
	// address-of bitfield: ExprType pointer → ind -1
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerTo(GetIntType())}
	cg := EmptyCGContext()
	if cg.VisitFactsExpressionVariable(e, Defaults()) {
		t.Fatal("bitfield addr")
	}
}

func TestReadPointedNullRejected(t *testing.T) {
	opts := Defaults()
	opts.NullPointerDerefProb = 0
	opts.DanglingPtrDerefProb = 0
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if cg.ReadPointed(p, 1, fm.GlobalFacts, opts) {
		t.Fatal("null pointee")
	}
}

func TestAccessDerefVolatile(t *testing.T) {
	// strict rule: volatile after deref clears SE-free
	// IsVolatiles layout: index len-deref-1; for deref=1 need volatiles[0]=true
	q := NewCVQualifiers([]bool{false, false}, []bool{true, false})
	v := CreateVariableQfer("g_p", PointerTo(GetIntType()), q)
	if !v.IsVolatileAfterDeref(1) {
		t.Fatal("qfer layout")
	}
	e := EmptyEffect()
	e2 := e.AccessDerefVolatile(v, 1, true)
	if e2.IsSideEffectFree() {
		t.Fatal("should clear SE-free")
	}
	// non-strict leaves SE-free
	e3 := e.AccessDerefVolatile(v, 1, false)
	if !e3.IsSideEffectFree() {
		t.Fatal("non-strict")
	}
}
