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

func TestCheckReadVarDanglingUsesProcessDeadProb(t *testing.T) {
	// FactPointTo.cpp:476–482 — is_dangling when dead && dead_pointer_dereference_prob==0
	// CGOptions has dead_pointer_dereference_prob only (no dual DanglingPtrDerefProb invent)
	prev := ProcessOptions()
	defer SetProcessOptions(prev)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	cg := EmptyCGContext()
	// default dead prob 0 → dangling reject
	opts := Defaults()
	opts.DeadPointerDerefProb = 0
	SetProcessOptions(opts)
	if cg.CheckReadVar(p, facts) {
		t.Fatal("dead ptr with deadProb 0 must fail")
	}
	// dead_pointer_dereference_prob > 0 → is_dangling_ptr false; read allowed
	opts.DeadPointerDerefProb = 50
	SetProcessOptions(opts)
	if !cg.CheckReadVar(p, facts) {
		t.Fatal("dead ptr with deadProb>0 must not invent always-reject")
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
	opts.DeadPointerDerefProb = 0
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

func TestReadIndicesConstantOK(t *testing.T) {
	// CGContext.cpp:352–380 — constant index expressions always visit OK
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"1"},
		IndexExprs: []*Expression{{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntType()}},
	}
	item.AsArray = item
	cg := EmptyCGContext()
	if !cg.ReadIndices(&item.Variable, nil) {
		t.Fatal("const index")
	}
}

func TestReadIndicesVarRecordsRead(t *testing.T) {
	// Variable index: visit_facts records read of IV
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntType()}},
	}
	item.AsArray = item
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !cg.ReadIndices(&item.Variable, nil) {
		t.Fatal("var index")
	}
	if !eff.IsRead(iv) {
		t.Fatal("IV should be read via index")
	}
}

func TestReadIndicesArrayFieldWalksParent(t *testing.T) {
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}},
		Sizes:      []int{2},
		Collective: parent,
		Indices:    []string{"0"},
		IndexExprs: []*Expression{{Term: TermConstant, Con: MakeInt(0), ExprType: GetIntType()}},
	}
	item.AsArray = item
	field := &Variable{Name: "g_a[0].f0", Type: GetIntType(), FieldVarOf: &item.Variable}
	cg := EmptyCGContext()
	if !cg.ReadIndices(field, nil) {
		t.Fatal("array field")
	}
}

func TestVisitIndicesEffectContext(t *testing.T) {
	// Lhs.cpp:273–284 — IV ok under empty RHS context
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
	}
	parent.AsArray = parent
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{3}},
		Sizes:      []int{3},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntType()}},
	}
	item.AsArray = item
	lhs := &Lhs{Var: &item.Variable, Type: GetIntType()}
	cg := EmptyCGContext()
	if !lhs.VisitIndices(&cg, Defaults()) {
		t.Fatal("visit indices")
	}
	// IV written in ambient context → VisitFacts on index should fail CheckReadVar
	cg2 := WithEffectContext(EmptyEffect().WriteVar(iv))
	if lhs.VisitIndices(&cg2, Defaults()) {
		// may still pass if VisitFactsExpressionVariable is lenient — require failure when written partially
		// CheckReadVar rejects IsWrittenPartially
		t.Fatal("want reject when IV written in context")
	}
}
