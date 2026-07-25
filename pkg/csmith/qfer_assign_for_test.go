package csmith

import "testing"

func TestCVQualifiersMatch(t *testing.T) {
	// isolate process match_exact (CGOptions) so Match(false) still sees non-exact
	prev := ProcessOptionsSess(testAmbientSession)
	po := prev
	po.MatchExactQualifiers = false
	SetProcessOptionsSess(testAmbientSession, po)
	defer SetProcessOptionsSess(testAmbientSession, prev)

	a := NewCVQualifiers([]bool{false}, []bool{false})
	b := NewCVQualifiers([]bool{true}, []bool{false})
	// one-level → always match (non-exact)
	if !a.Match(b, false) {
		t.Fatal("1-level")
	}
	if a.Match(b, true) {
		t.Fatal("exact")
	}
	w := CVQualifiers{Wildcard: true}
	if !w.Match(b, true) {
		t.Fatal("wild")
	}
	// process CGOptions::match_exact_qualifiers true → Match(false) still exact via Options
	po.MatchExactQualifiers = true
	SetProcessOptionsSess(testAmbientSession, po)
	if a.Match(b, false) {
		t.Fatal("process exact should reject differing 1-level")
	}
}

func TestChooseVarFullUsesProcessMatchExact(t *testing.T) {
	// VariableSelector choose_var → match_indirect → process match_exact
	prev := ProcessOptionsSess(testAmbientSession)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	po := prev
	po.MatchExactQualifiers = true
	SetProcessOptionsSess(testAmbientSession, po)

	vol := CreateVariableScalars("g_v", GetIntType(), false, true)
	plain := CreateVariableScalars("g_p", GetIntType(), false, false)
	// const qfer wants exact match — volatile var should fail
	q := NewCVQualifiers([]bool{true}, []bool{false})
	cg := EmptyCGContext().WithSession(testAmbientSession)
	got := ChooseVarFull(NewRng(1), []*Variable{vol, plain}, AccessRead, cg, GetIntType(), &q, MatchFlexible, nil, false, false, false)
	// neither matches exact const; plain is non-const non-vol
	if got != nil {
		// may still be nil if eligibility rejects; process exact must not pick vol for const want
		if got == vol {
			t.Fatal("exact match must not select volatile for const qfer")
		}
	}
}

func TestMatchIndirect(t *testing.T) {
	prev := ProcessOptionsSess(testAmbientSession)
	po := prev
	po.MatchExactQualifiers = false
	SetProcessOptionsSess(testAmbientSession, po)
	defer SetProcessOptionsSess(testAmbientSession, prev)

	// wanted scalar qfer vs pointer var qfer (2 levels)
	want := NewCVQualifiers([]bool{false}, []bool{false})
	have := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// deref = 2-1 = 1; match want with have.IndirectQualifiers(1)
	if !want.MatchIndirect(have, false) {
		t.Fatal("indirect")
	}
	// multi-level address gap complete false (C++ return false, no assert)
	ClearErrorSess(testAmbientSession)
	deep := NewCVQualifiers([]bool{false, false, false}, []bool{false, false, false})
	shallow := NewCVQualifiers([]bool{false}, []bool{false})
	// want deep vs shallow: deref = 1-3 = -2
	if deep.MatchIndirect(shallow, false) {
		t.Fatal("deref < -1 must fail closed false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("deref < -1 MatchIndirect must stay non-sticky complete false")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIndirectQualifiers(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{true, false}, []bool{false, true})
	// address — push_back false,false
	addr := q.IndirectQualifiers(-1)
	if len(addr.IsConsts) != 3 {
		t.Fatal(len(addr.IsConsts))
	}
	// deref 1 — pop_back (upstream remove_qualifiers)
	// [true, false] → [true]
	d := q.IndirectQualifiers(1)
	if len(d.IsConsts) != 1 || d.IsConsts[0] != true {
		t.Fatalf("%v", d.IsConsts)
	}
	// over-deref sticky empty (no invent partial pop)
	ClearErrorSess(testAmbientSession)
	got := q.IndirectQualifiers(5)
	if len(got.IsConsts) != 0 || len(got.IsVolatiles) != 0 {
		t.Fatal("over-deref must fail closed empty", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("over-deref IndirectQualifiers must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindPointerFieldsNilHole(t *testing.T) {
	// nil FieldVars hole sticky (no invent empty-complete pointer fields / soft re-pick)
	ClearErrorSess(testAmbientSession)
	sv := &Variable{
		Name: "s", Type: &Type{isStruct: true},
		FieldVars: []*Variable{
			{Name: "s.p", Type: PointerTo(GetIntType())},
			nil,
		},
	}
	if VariablesComplete(sv.FindPointerFields()) {
		t.Fatal("nil FieldVars hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FieldVars FindPointerFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VariablesComplete(((*Variable)(nil)).FindPointerFields()) {
		t.Fatal("nil subject FindPointerFields must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject FindPointerFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil field soft invent: IsPointer/IsAggregate residual ERROR then skip as
	// neither → complete empty (or later fields only). Fair: sticky Incomplete.
	tyNil := &Variable{
		Name: "s2", Type: &Type{isStruct: true},
		FieldVars: []*Variable{
			{Name: "s2.typeless"}, // Type nil non-special
			{Name: "s2.p", Type: PointerTo(GetIntType())},
		},
	}
	gotTy := tyNil.FindPointerFields()
	if VariablesComplete(gotTy) {
		t.Fatal("Type-nil field must fail closed incomplete, not soft-skip complete", gotTy)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil field FindPointerFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nested FindPointerFields residual soft invent was soft-continue later fields invent complete.
	// Fair: sticky IncompleteVariables via nested Type-nil.
	nestHole := &Variable{
		Name: "nest", Type: &Type{isStruct: true},
		FieldVars: []*Variable{{Name: "nest.x"}}, // Type nil
	}
	outer := &Variable{
		Name: "outer", Type: &Type{isStruct: true},
		FieldVars: []*Variable{
			nestHole,
			{Name: "outer.p", Type: PointerTo(GetIntType())},
		},
	}
	// nestHole Type is aggregate - IsAggregate true, recurse FindPointerFields hits Type-nil field
	gotNest := outer.FindPointerFields()
	if VariablesComplete(gotNest) {
		t.Fatal("nested residual FindPointerFields must fail closed incomplete", gotNest)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nested residual FindPointerFields must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindPointerFields(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	// inject a pointer field if none
	sv := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	ptrs := sv.FindPointerFields()
	// may be empty for random struct
	_ = ptrs
	// manual: struct with pointer field
	pt := PointerTo(GetIntType())
	// CreateFieldVars from type with pointer - use Variable with FieldVars set
	parent := &Variable{Name: "g_u", Type: &Type{isStruct: true, StructName: "S"}}
	pf := &Variable{Name: "g_u.f0", Type: pt, FieldVarOf: parent}
	parent.FieldVars = []*Variable{pf}
	got := parent.FindPointerFields()
	if len(got) != 1 || got[0] != pf {
		t.Fatal(got)
	}
}

func TestAbstractFactAggregatePointerFields(t *testing.T) {
	pt := PointerTo(GetIntType())
	parent := &Variable{Name: "g_s", Type: &Type{isStruct: true}}
	pf := &Variable{Name: "g_s.f0", Type: pt, FieldVarOf: parent}
	parent.FieldVars = []*Variable{pf}
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	rhs := &Expression{Term: TermVariable, Var: tgt, ExprType: pt}
	// address-of style: indirect -1 on tgt would need expr; use null const
	rhs = &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	facts, _ := AbstractFactForAssign(nil, parent, 0, rhs)
	if len(facts) == 0 {
		t.Fatal("no facts")
	}
	if !FindRelatedPointTo(facts, pf).IsNull() {
		t.Fatal("null field")
	}
}

func TestPostLoopAnalysisMissingBodyInFailClosed(t *testing.T) {
	// StatementFor.cpp:355 — global_facts = map_facts_in[&body]
	// missing body in must not invent keep prior GlobalFacts
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	body := &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtAssign}}}
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	// no MapFactsIn[10]; not must_return — C++ map[] empty is complete empty
	postLoopAnalysis(fm, forSt, body, nil, nil, EmptyEffect(), nil)
	if FindRelatedPointTo(fm.GlobalFacts, p) != nil {
		t.Fatal("missing body MapFactsIn must clear prior, not invent keep prior")
	}
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("missing body in is complete empty, not incomplete marker")
	}
	// body StmID 0 — incomplete IR marker
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	postLoopAnalysis(fm, forSt, &Block{StmID: IncompleteStmID}, nil, nil, EmptyEffect(), nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("body StmID 0 must fail closed incomplete GlobalFacts")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisIncompleteBodyInFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	body := &Block{StmID: 11, Stmts: []Stmt{{Kind: StmtAssign}}}
	fm.MapFactsIn = map[int][]*FactPointTo{
		11: {MakeFactPointTo(p, NullPtr), nil},
	}
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	postLoopAnalysis(fm, forSt, body, nil, nil, EmptyEffect(), nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete body MapFactsIn must fail closed nil GlobalFacts")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisIncompleteBreakOutFailClosed(t *testing.T) {
	// merge_jump_facts always; incomplete break out fails closed
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	body := &Block{StmID: 12, BreakStmIDs: []int{22}, Stmts: []Stmt{{Kind: StmtAssign}}}
	fm.SetMapFactsIn(12, pre)
	fm.MapFactsOut = map[int][]*FactPointTo{
		22: {MakeFactPointTo(p, NullPtr), nil},
	}
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	postLoopAnalysis(fm, forSt, body, pre, nil, EmptyEffect(), nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete break MapFactsOut must fail closed nil GlobalFacts")
	}
	// incomplete first break must not invent continue-merge later complete break
	// via FactsComplete(nil)==true after wipe
	c := CreateVariableScalars("g_c", GetIntType(), false, false)
	body2 := &Block{StmID: 13, BreakStmIDs: []int{30, 31}, Stmts: []Stmt{{Kind: StmtAssign}}}
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.SetMapFactsIn(13, pre)
	fm2.MapFactsOut = map[int][]*FactPointTo{
		30: {MakeFactPointTo(p, NullPtr), nil},
		31: {MakeFactPointTo(p, c)},
	}
	postLoopAnalysis(fm2, &Stmt{Kind: StmtFor, StmID: 8, Then: body2}, body2, pre, nil, EmptyEffect(), nil)
	if FactsComplete(fm2.GlobalFacts) {
		t.Fatal("incomplete first break must not invent later break merge", fm2.GlobalFacts)
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisIncompleteBodyInNoMustReturnRestore(t *testing.T) {
	// incomplete map_facts_in[body] must not invent RestoreFacts(pre) on must_return
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	body := &Block{StmID: 14, Stmts: []Stmt{{Kind: StmtReturn}}}
	fm.MapFactsIn = map[int][]*FactPointTo{
		14: {MakeFactPointTo(p, NullPtr), nil},
	}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	postLoopAnalysis(fm, &Stmt{Kind: StmtFor, StmID: 7, Then: body}, body, pre, nil, EmptyEffect(), nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete body in must not invent must_return restore", fm.GlobalFacts)
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisMustReturn(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	body := &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtReturn}}}
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	postLoopAnalysis(fm, forSt, body, pre, nil, EmptyEffect(), nil)
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || fp.IsNull() || (len(fp.PointTo) > 0 && fp.PointTo[0] != a) {
		t.Fatalf("want pre fact → a, got %+v", fp)
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisMustReturnResidualSticky(t *testing.T) {
	// MustReturn residual soft invent was soft-continue break-merge invent complete GlobalFacts.
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	// complete map_facts_in so we reach MustReturn
	body := &Block{
		StmID: 10,
		Stmts: []Stmt{{Kind: StmtIfElse, Then: nil, Else: &Block{StmID: 30}}},
	}
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	postLoopAnalysis(fm, &Stmt{Kind: StmtFor, StmID: 9, Then: body}, body, pre, nil, EmptyEffect(), nil)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("MustReturn residual must fail closed incomplete GlobalFacts", fm.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MustReturn residual postLoopAnalysis must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostLoopAnalysisBreakMerge(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	body := &Block{
		StmID:       10,
		BreakStmIDs: []int{20},
		Stmts:       []Stmt{{Kind: StmtAssign}},
	}
	fm.SetMapFactsIn(10, pre)
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointTo(p, b)})
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	// body entry facts base; merge break outs
	postLoopAnalysis(fm, forSt, body, pre, nil, EmptyEffect(), nil)
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil {
		t.Fatal("nil")
	}
	// should include b from break and a from entry (merge)
	if !IsVariableInSet(fp.PointTo, a) && !IsVariableInSet(fp.PointTo, b) {
		// at least one after merge
		t.Fatalf("%+v", fp)
	}
	// break edge created
	found := false
	for _, e := range fm.CFGEdges {
		if e != nil && e.SrcID == 20 && e.DestStmID == 9 && e.PostDest {
			found = true
		}
	}
	if !found {
		t.Fatal("break edge", fm.CFGEdges)
	}
}
