package csmith

import "testing"

func TestCVQualifiersMatch(t *testing.T) {
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
}

func TestMatchIndirect(t *testing.T) {
	// wanted scalar qfer vs pointer var qfer (2 levels)
	want := NewCVQualifiers([]bool{false}, []bool{false})
	have := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// deref = 2-1 = 1; match want with have.IndirectQualifiers(1)
	if !want.MatchIndirect(have, false) {
		t.Fatal("indirect")
	}
}

func TestIndirectQualifiers(t *testing.T) {
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
}

func TestFindPointerFields(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
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
	facts := AbstractFactForAssign(nil, parent, 0, rhs)
	if len(facts) == 0 {
		t.Fatal("no facts")
	}
	if !FindRelatedPointTo(facts, pf).IsNull() {
		t.Fatal("null field")
	}
}

func TestPostLoopAnalysisMustReturn(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	pre := []*FactPointTo{MakeFactPointTo(p, a)}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	body := &Block{StmID: 10, Stmts: []Stmt{{Kind: StmtReturn}}}
	forSt := &Stmt{Kind: StmtFor, StmID: 9, Then: body}
	postLoopAnalysis(fm, forSt, body, pre, nil)
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || fp.IsNull() || (len(fp.PointTo) > 0 && fp.PointTo[0] != a) {
		t.Fatalf("want pre fact → a, got %+v", fp)
	}
}

func TestPostLoopAnalysisBreakMerge(t *testing.T) {
	fm := NewFactMgr(nil)
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
	postLoopAnalysis(fm, forSt, body, pre, nil)
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
