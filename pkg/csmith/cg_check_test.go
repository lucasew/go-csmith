package csmith

import (
	"testing"
)

func TestCheckReadVarBasic(t *testing.T) {
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	if !cg.CheckReadVar(v, nil) {
		t.Fatal("read ok")
	}
	if !eff.IsReadSess(testAmbientSession, v) {
		t.Fatal("not recorded")
	}
}

func TestCheckReadVarNonReadable(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoReadVars: []*Variable{v}})
	if cg.CheckReadVar(v, nil) {
		t.Fatal("should reject")
	}
}

func TestCheckWriteVarConst(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if cg.CheckWriteVar(v, nil) {
		t.Fatal("const write")
	}
}

func TestCheckWriteVarIncompleteCollectiveFailClosed(t *testing.T) {
	// GetCollective nil on incomplete FieldVars must not invent write success / panic
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: &Type{isStruct: true, Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		}}, IsArray: true, ArraySizes: []int{2}},
		Sizes: []int{2},
	}
	parent.AsArray = parent
	parent.CreateFieldVarsSess(testAmbientSession)
	item := parent.ItemizeConstIndices([]int{0}, nil)
	if item == nil {
		t.Fatal("itemize")
	}
	item.CreateFieldVarsSess(testAmbientSession)
	if len(item.FieldVars) == 0 {
		t.Fatal("fields")
	}
	fld := item.FieldVars[0]
	item.FieldVars = append(item.FieldVars, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if cg.CheckWriteVar(fld, nil) {
		t.Fatal("incomplete collective must fail closed CheckWriteVar")
	}
	if cg.CheckReadVar(fld, nil) {
		t.Fatal("incomplete collective must fail closed CheckReadVar")
	}
	// force-write path sets sticky error, not invent silent skip
	cg.WriteVar(fld)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVar incomplete collective must SetError")
	}
	ClearErrorSess(testAmbientSession)
	// force path on incomplete EffectStm must SetError (no invent silent grow)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.EffectStm = IncompleteEffect()
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, false)
	cg2.WriteVar(v)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVar incomplete EffectStm must SetError")
	}
	ClearErrorSess(testAmbientSession)
	cg3 := EmptyCGContext().WithSession(testAmbientSession)
	cg3.EffectStm = IncompleteEffect()
	cg3.ReadVar(v)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ReadVar incomplete EffectStm must SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckWriteVarIVBound(t *testing.T) {
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.AddIVBound(iv, 10)
	if cg.CheckWriteVar(iv, nil) {
		t.Fatal("iv write")
	}
}

func TestCheckReadVarVolatileNeedsSEFree(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	// non-SE-free context rejects volatile read
	cg := WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)
	if cg.CheckReadVar(v, nil) {
		t.Fatal("volatile in SE context")
	}
	// SE-free allows
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	if !cg2.CheckReadVar(v, nil) {
		t.Fatal("volatile SE-free")
	}
}

func TestCheckWriteVarPartialConflict(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	eff := EmptyEffect().WriteVarSess(testAmbientSession, v)
	cg := WithEffectContext(eff).WithSession(testAmbientSession)
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
	prev := ProcessOptionsSess(testAmbientSession)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// default dead prob 0 → dangling reject
	opts := Defaults()
	opts.DeadPointerDerefProb = 0
	SetProcessOptionsSess(testAmbientSession, opts)
	if cg.CheckReadVar(p, facts) {
		t.Fatal("dead ptr with deadProb 0 must fail")
	}
	// dead_pointer_dereference_prob > 0 → is_dangling_ptr false; read allowed
	opts.DeadPointerDerefProb = 50
	SetProcessOptionsSess(testAmbientSession, opts)
	if !cg.CheckReadVar(p, facts) {
		t.Fatal("dead ptr with deadProb>0 must not invent always-reject")
	}
}

func TestVisitFactsLhsScalar(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("write")
	}
}

func TestVisitFactsExpressionVariableAddrBitfield(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_bf", GetIntTypeSess(testAmbientSession), false, false)
	v.IsBitfield = true
	// address-of bitfield: ExprType pointer → ind -1
	e := &Expression{Term: TermVariable, Var: v, ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if cg.VisitFactsExpressionVariable(e, Defaults()) {
		t.Fatal("bitfield addr")
	}
}

func TestVisitFactsLhsNoInventIncomplete(t *testing.T) {
	// incomplete Lhs hard IR sticky (no invent visit success / soft re-pick)
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if cg.VisitFactsLhs(nil, Defaults()) {
		t.Fatal("nil lhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cg.VisitFactsLhs(&Lhs{}, Defaults()) {
		t.Fatal("nil lhs.Var")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs.Var VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsValidPtr residual soft invent was soft-continue visit invent success.
	// Fair: sticky false. deref>0 + incomplete facts.
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	lhsPtr := &Lhs{Var: p, Type: GetIntTypeSess(testAmbientSession)} // *p
	cgPtr := EmptyCGContext().WithSession(testAmbientSession)
	cgPtr.EffectStm = EmptyEffect()
	cgPtr.FM = NewFactMgrSess(testAmbientSession, nil)
	cgPtr.FM.GlobalFacts = []*FactPointTo{nil} // incomplete
	if cgPtr.VisitFactsLhs(lhsPtr, Defaults()) {
		t.Fatal("IsValidPtr residual VisitFactsLhs must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsValidPtr residual VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// PtrModified residual soft invent was soft-continue visit invent success.
	// Fair: sticky false via incomplete EffectStm IsWritten residual true (modified).
	cgMod := EmptyCGContext().WithSession(testAmbientSession)
	cgMod.EffectStm = IncompleteEffect()
	cgMod.FM = NewFactMgrSess(testAmbientSession, nil)
	cgMod.FM.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false))}
	if cgMod.VisitFactsLhs(lhsPtr, Defaults()) {
		t.Fatal("PtrModified residual VisitFactsLhs must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("PtrModified residual VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectStm/accum sticky via CheckWriteVar
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.EffectStm = IncompleteEffect()
	if cg2.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("incomplete EffectStm must fail closed VisitFactsLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg3 := EmptyCGContext().WithSession(testAmbientSession)
	inc := IncompleteEffect()
	cg3.EffectAccum = &inc
	if cg3.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("incomplete EffectAccum must fail closed VisitFactsLhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum VisitFactsLhs must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReadPointedNullRejected(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.NullPointerDerefProb = 0
	opts.DeadPointerDerefProb = 0
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if cg.ReadPointed(p, 1, fm.GlobalFacts, opts) {
		t.Fatal("null pointee")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete null reject ReadPointed must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// CheckReadVar residual soft invent was soft-continue later pointees invent success.
	// Fair: sticky false. pointee with incomplete EffectStm stickies CheckReadVar residual.
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", GetIntTypeSess(testAmbientSession), false, false)
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, tgt)}
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm2)
	cg2.EffectStm = IncompleteEffect()
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if cg2.ReadPointed(p, 1, fm2.GlobalFacts, opts) {
		t.Fatal("CheckReadVar residual ReadPointed must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CheckReadVar residual ReadPointed must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAccessDerefVolatile(t *testing.T) {
	// strict rule: volatile after deref clears SE-free
	// IsVolatiles layout: index len-deref-1; for deref=1 need volatiles[0]=true
	q := NewCVQualifiersSess(testAmbientSession, []bool{false, false}, []bool{true, false})
	v := CreateVariableQferSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), q)
	if !v.IsVolatileAfterDerefSess(testAmbientSession, 1) {
		t.Fatal("qfer layout")
	}
	e := EmptyEffect()
	e2 := e.AccessDerefVolatileSess(testAmbientSession, v, 1, true)
	if e2.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("should clear SE-free")
	}
	// non-strict leaves SE-free
	e3 := e.AccessDerefVolatileSess(testAmbientSession, v, 1, false)
	if !e3.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("non-strict")
	}
	// under strictVolatile, Variable always live; sticky IncompleteEffect
	ClearErrorSess(testAmbientSession)
	if EffectComplete(e.AccessDerefVolatileSess(testAmbientSession, nil, 1, true)) {
		t.Fatal("nil Variable strict AccessDerefVolatile must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable strict AccessDerefVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-strict nil subject complete identity
	if !EffectComplete(e.AccessDerefVolatileSess(testAmbientSession, nil, 1, false)) {
		t.Fatal("non-strict nil subject must stay complete identity")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("non-strict nil subject must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckDerefVolatileIncompleteFailClosed(t *testing.T) {
	// incomplete ambient/stm sticky (no invent CheckDerefVolatile / soft re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.StrictVolatileRule = true
	v := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	cg := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	if cg.CheckDerefVolatile(v, 1, opts) {
		t.Fatal("incomplete ambient must fail closed CheckDerefVolatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete ambient CheckDerefVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.EffectStm = IncompleteEffect()
	if cg2.CheckDerefVolatile(v, 1, opts) {
		t.Fatal("incomplete EffectStm must fail closed CheckDerefVolatile")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm CheckDerefVolatile must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckReadWriteVarIncompleteStmFailClosed(t *testing.T) {
	// incomplete EffectStm sticky (no invent CheckRead/WriteVar / soft re-pick)
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectStm = IncompleteEffect()
	if cg.CheckReadVar(v, nil) {
		t.Fatal("incomplete EffectStm must fail closed CheckReadVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm CheckReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cg.CheckWriteVar(v, nil) {
		t.Fatal("incomplete EffectStm must fail closed CheckWriteVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm CheckWriteVar must SetError sticky")
	}
	// incomplete EffectAccum sticky
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	inc := IncompleteEffect()
	cg2.EffectAccum = &inc
	if cg2.CheckReadVar(v, nil) {
		t.Fatal("incomplete EffectAccum must fail closed CheckReadVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum CheckReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cg2.CheckWriteVar(v, nil) {
		t.Fatal("incomplete EffectAccum must fail closed CheckWriteVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum CheckWriteVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCheckReadVarIsInsideUnionFieldResidualSticky(t *testing.T) {
	// IsNonreadableField / IsInsideUnionField Type-nil ancestry residual: soft invent was
	// return CheckReadVar true past hole under empty facts path. Fair: sticky fail closed.
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if cg.CheckReadVar(field, nil) {
		t.Fatal("IsInsideUnionField residual must fail closed CheckReadVar")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsInsideUnionField residual CheckReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil soft invent: IsPointer residual ERROR+false skip dangling then
	// ReadVar/WriteVar return true. Fair: sticky false before classify.
	shell := &Variable{Name: "g_typeless"}
	if cg.CheckReadVar(shell, nil) {
		t.Fatal("Type-nil must fail closed CheckReadVar, not invent read success")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil CheckReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cg.CheckWriteVar(shell, nil) {
		t.Fatal("Type-nil must fail closed CheckWriteVar, not invent write success")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil CheckWriteVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReadIndicesHardIRSticky(t *testing.T) {
	// IsArray without AsArray hard IR sticky
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	broken := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	if cg.ReadIndices(broken, nil) {
		t.Fatal("IsArray without AsArray must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray ReadIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if cg.ReadIndices(nil, nil) {
		t.Fatal("nil var ReadIndices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var ReadIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestReadIndicesConstantOK(t *testing.T) {
	// CGContext.cpp:352–380 — constant index expressions always visit OK
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"1"},
		IndexExprs: []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if !cg.ReadIndices(&item.Variable, nil) {
		t.Fatal("const index")
	}
}

func TestReadIndicesVarRecordsRead(t *testing.T) {
	// Variable index: visit_facts records read of IV
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:    []int{4},
	}
	parent.AsArray = parent
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}},
		Sizes:      []int{4},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if !cg.ReadIndices(&item.Variable, nil) {
		t.Fatal("var index")
	}
	if !eff.IsReadSess(testAmbientSession, iv) {
		t.Fatal("IV should be read via index")
	}
}

func TestReadIndicesArrayFieldWalksParent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}},
		Sizes:    []int{2},
	}
	parent.AsArray = parent
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}},
		Sizes:      []int{2},
		Collective: parent,
		Indices:    []string{"0"},
		IndexExprs: []*Expression{{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	field := &Variable{Name: "g_a[0].f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: &item.Variable}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if !cg.ReadIndices(field, nil) {
		t.Fatal("array field")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete array field ReadIndices must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArrayField residual: IsArray without AsArray parent soft invent was soft-continue
	// complete true skip indices. Fair: sticky false.
	shell := &Variable{Name: "g_b", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	fieldHole := &Variable{Name: "g_b[0].f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: shell}
	if cg.ReadIndices(fieldHole, nil) {
		t.Fatal("IsArrayField residual ReadIndices must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArrayField residual ReadIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitIndicesEffectContext(t *testing.T) {
	// Lhs.cpp:273–284 — IV ok under empty RHS context
	ClearErrorSess(testAmbientSession)
	parent := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
	}
	parent.AsArray = parent
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	item := &ArrayVariable{
		Variable:   Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{3}},
		Sizes:      []int{3},
		Collective: parent,
		Indices:    []string{"g_i"},
		IndexExprs: []*Expression{{Term: TermVariable, Var: iv, ExprType: GetIntTypeSess(testAmbientSession)}},
	}
	item.AsArray = item
	lhs := &Lhs{Var: &item.Variable, Type: GetIntTypeSess(testAmbientSession)}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if !lhs.VisitIndices(&cg, Defaults()) {
		t.Fatal("visit indices")
	}
	// IV written in ambient context → VisitFacts on index should fail CheckReadVar
	cg2 := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, iv)).WithSession(testAmbientSession)
	if lhs.VisitIndices(&cg2, Defaults()) {
		// may still pass if VisitFactsExpressionVariable is lenient — require failure when written partially
		// CheckReadVar rejects IsWrittenPartially
		t.Fatal("want reject when IV written in context")
	}
	// Incomplete ambient (context or EffectStm) must not invent index visit success
	cg3 := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	if lhs.VisitIndices(&cg3, Defaults()) {
		t.Fatal("incomplete EffectContext must fail closed VisitIndices")
	}
	ClearErrorSess(testAmbientSession)
	cg4 := EmptyCGContext().WithSession(testAmbientSession)
	cg4.EffectStm = IncompleteEffect()
	if lhs.VisitIndices(&cg4, Defaults()) {
		t.Fatal("incomplete EffectStm must fail closed VisitIndices")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was complete true (no indices path)
	// fair: sticky false (mirrors ReadIndices hard IR)
	broken := &Lhs{Var: &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}}
	cg5 := EmptyCGContext().WithSession(testAmbientSession)
	if broken.VisitIndices(&cg5, Defaults()) {
		t.Fatal("IsArray without AsArray VisitIndices must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray VisitIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Lhs sticky
	if ((*Lhs)(nil)).VisitIndices(&cg5, Defaults()) {
		t.Fatal("nil Lhs VisitIndices must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Lhs VisitIndices must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExtendCallChainNilHoleFailClosed(t *testing.T) {
	// incomplete call_chain fails closed empty (no invent keep-hole chain)
	from := EmptyCGContext().WithSession(testAmbientSession)
	from.CallChain = []*Block{&Block{StmID: 1}, nil}
	c := EmptyCGContext().WithSession(testAmbientSession)
	c.ExtendCallChain(from)
	if c.CallChain != nil {
		t.Fatal("nil CallChain hole must clear chain, not invent keep-hole")
	}
}

func TestAddVisibleEffectAtNilCallChainHoleFailClosed(t *testing.T) {
	// incomplete call_chain must not invent partial external merge
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	e := EmptyEffect().WriteVarSess(testAmbientSession, g)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.CallChain = []*Block{nil}
	cg.AddVisibleEffectAt(e, &Block{StmID: 1})
	// fail closed: accum unchanged + sticky error (no invent merge past hole)
	if eff.IsWrittenSess(testAmbientSession, g) {
		t.Fatal("nil CallChain hole must not invent external write merge")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CallChain hole must SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestNoteWriteIncompleteAccumFailClosed(t *testing.T) {
	// NoteWrite/NoteRead must not invent silent grow on incomplete accum
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	inc := IncompleteEffect()
	cg.EffectAccum = &inc
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	cg.NoteWrite(g)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("NoteWrite incomplete accum must SetError")
	}
	ClearErrorSess(testAmbientSession)
	inc2 := IncompleteEffect()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg2.EffectAccum = &inc2
	cg2.NoteRead(g)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("NoteRead incomplete accum must SetError")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live; sticky no invent soft-skip write/read past hole
	cg.NoteWrite(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable NoteWrite must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg.NoteRead(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable NoteRead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// CGContext always live on mutators
	(*CGContext)(nil).ClearEffectStm()
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext ClearEffectStm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).ResetEffectAccum(EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext ResetEffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).AddExternalEffect(EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext AddExternalEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).AddVisibleEffectAt(EmptyEffect(), nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext AddVisibleEffectAt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).AddEffect(EmptyEffect(), false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext AddEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).MergeParamContext(EmptyCGContext().WithSession(testAmbientSession), false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext MergeParamContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).ReadVar(g)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext ReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).WriteVar(g)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext WriteVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg.ReadVar(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable CGContext.ReadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg.WriteVar(nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable CGContext.WriteVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).AddIVBound(g, 1)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext AddIVBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg.AddIVBound(nil, 1)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IV AddIVBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	(*CGContext)(nil).RemoveIVBound(g)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CGContext RemoveIVBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddEffectIncompleteFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectStm = IncompleteEffect()
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, false)
	cg.AddEffect(EmptyEffect().WriteVarSess(testAmbientSession, v), false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("AddEffect incomplete stm must SetError")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	cg2.AddEffect(IncompleteEffect(), false)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("AddEffect incomplete e must SetError")
	}
	ClearErrorSess(testAmbientSession)
}
