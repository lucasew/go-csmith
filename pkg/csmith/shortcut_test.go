package csmith

import (
	"testing"
)

func TestSameFacts(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	b := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if !SameFacts(a, b) {
		t.Fatal("same")
	}
	b2 := []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	if SameFacts(a, b2) {
		t.Fatal("diff")
	}
	// nil hole sticky — no invent same-as-skip
	ClearError()
	hole := []*FactPointTo{nil}
	if SameFacts(hole, hole) {
		t.Fatal("nil hole must not be same")
	}
	if !HasError() {
		t.Fatal("SameFacts nil hole must SetError sticky")
	}
	ClearError()
	// incomplete PointTo sticky
	ptHole := []*FactPointTo{{Var: p, PointTo: []*Variable{nil}}}
	if SameFacts(ptHole, ptHole) {
		t.Fatal("nil pointee must not invent SameFacts")
	}
	if !HasError() {
		t.Fatal("SameFacts nil pointee must SetError sticky")
	}
	ClearError()
	if FindFact(ptHole, MakeFactPointTo(p, NullPtr)) >= 0 {
		t.Fatal("FindFact incomplete map must fail closed -1")
	}
	if !HasError() {
		t.Fatal("FindFact incomplete map must SetError sticky")
	}
	ClearError()
	// Equal residual soft invent was soft-continue later match invent found index.
	// Fair: sticky -1. Want with PointTo hole stickies Equal residual false.
	wantHole := &FactPointTo{Var: p, PointTo: []*Variable{nil}}
	complete := []*FactPointTo{MakeFactPointTo(p, NullPtr), MakeFactPointTo(p, GarbagePtr)}
	if FindFact(complete, wantHole) >= 0 {
		t.Fatal("Equal residual FindFact must fail closed -1")
	}
	if !HasError() {
		t.Fatal("equal residual FindFact must SetError sticky")
	}
	ClearError()
	// SameFacts residual via FindFact Equal residual soft invent was same-true.
	// Fair: sticky not-same.
	// Use complete map vs want that causes Equal residual on first scan element...
	// SameFacts(a,b) for each a in FindFact(b): if want in a has PointTo hole, FactsComplete fails first.
	// Equal residual: complete facts with different PointTo holes on map side already FactsComplete false.
	// Equal residual via want complete but map fact with PointTo that Equal can residual - FactsComplete rejects map holes.
	// Residual path for FindFact Equal residual is want incomplete PointTo - already covered above for FindFact.
	// SameFacts with residual from FindFact when want is complete: soft invent covered by FindFact residual.
	// Add SameFacts residual when FindFact residual from incomplete want isn't reachable via SameFacts (FactsComplete on a).
	// Covered by FindFact residual test.
}

func TestSubsetFacts(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// wider set implies narrower
	wide := []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}
	narrow := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// subset_facts(narrow, wide): each narrow must be implied by related in wide
	// wide.Imply(narrow) means wide covers narrow's points
	if !SubsetFacts(narrow, wide) {
		// narrow is subset if wide implies narrow
		// Imply: f.Imply(other) means f covers other
		// SubsetFacts(a,b): for each f1 in a, related f2 in b implies f1
		// so f2.Imply(f1) — wide implies narrow ✓
		if !wide[0].Imply(narrow[0]) {
			t.Fatal("imply")
		}
		// size must match for subset_facts upstream
		t.Log("size mismatch expected fail")
	}
	// same size: both one fact
	if !SubsetFacts(narrow, []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}) {
		// related wide implies null-only? wide implies null-only yes
		// wait SubsetFacts(narrow, wide2) with same len
		w := []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})}
		if !SubsetFacts(narrow, w) {
			t.Fatal("subset")
		}
	}
	// nil fact hole sticky — no invent skip as subset
	ClearError()
	hole := []*FactPointTo{nil}
	if SubsetFacts(hole, hole) {
		t.Fatal("nil hole must not be subset")
	}
	if !HasError() {
		t.Fatal("SubsetFacts nil hole must SetError sticky")
	}
	ClearError()
	// Imply residual: PointTo nil hole soft invent was soft-continue not-subset then invent subset later.
	// Fair: sticky fail closed not-subset with ERROR.
	broken := &FactPointTo{Var: p, PointTo: []*Variable{NullPtr, nil}}
	ok := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if SubsetFacts(ok, []*FactPointTo{broken}) {
		t.Fatal("Imply residual must fail closed not-subset")
	}
	if !HasError() {
		t.Fatal("Imply residual SubsetFacts must SetError sticky")
	}
	ClearError()
}

func TestIsCtrlStmt(t *testing.T) {
	if !IsCtrlStmt(&Stmt{Kind: StmtBreak}) || IsCtrlStmt(&Stmt{Kind: StmtAssign}) {
		t.Fatal("ctrl")
	}
	// Statement always live; sticky no invent not-ctrl soft-skip
	ClearError()
	if IsCtrlStmt(nil) {
		t.Fatal("nil IsCtrlStmt must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsCtrlStmt must SetError sticky")
	}
	ClearError()
}


func TestSameFactVec(t *testing.T) {
	// Fact.cpp:237–246 full FactVec (ePointTo + eUnionWrite).
	ClearError()
	ut := &Type{isUnion: true, StructName: "U_sfv", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_sfv", ut, false, false)
	parent.CreateFieldVars()
	u0 := MakeFactUnion(parent, 0)
	u1 := MakeFactUnion(parent, 1)
	pvar := CreateVariableScalars("g_p_sfv", PointerTo(GetIntType()), false, false)
	pt := []*FactPointTo{MakeFactPointTo(pvar, NullPtr)}
	if !SameFactVec(pt, []*FactUnion{u0}, pt, []*FactUnion{MakeFactUnion(parent, 0)}) {
		t.Fatal("same full vec")
	}
	if SameFactVec(pt, []*FactUnion{u0}, pt, []*FactUnion{u1}) {
		t.Fatal("union mismatch")
	}
	ClearError()
}

func TestShortcutAnalysisInstallsOutUnions(t *testing.T) {
	// Statement.cpp:559 — inputs = map_facts_out[this] full FactVec (eUnionWrite too).
	// Soft invent left live UnionFacts at entry after ShortcutOK.
	ClearError()
	ut := &Type{isUnion: true, StructName: "U_sc", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_sc", ut, false, false)
	parent.CreateFieldVars()
	entryU := MakeFactUnion(parent, 0)
	outU := MakeFactUnion(parent, 1)
	if entryU == nil || outU == nil {
		t.Fatal("facts")
	}
	fm := NewFactMgr(nil)
	pt := []*FactPointTo{}
	fm.SetMapFactsInPair(9, pt, []*FactUnion{entryU})
	fm.SetMapFactsOutPair(9, pt, []*FactUnion{outU})
	fm.SetMapStmEffect(9, EmptyEffect())
	fm.UnionFacts = []*FactUnion{MakeFactUnion(parent, 0)} // entry lattice live
	st := &Stmt{Kind: StmtAssign, StmID: 9}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := append([]*FactPointTo(nil), pt...)
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutOK {
		t.Fatalf("want ShortcutOK err=%v", HasError())
	}
	if len(fm.UnionFacts) != 1 || fm.UnionFacts[0] == nil || fm.UnionFacts[0].LastWrittenFID != 1 {
		t.Fatalf("shortcut must install map_out unions last_write=1, got %#v", fm.UnionFacts)
	}
	// entry snapshot must not be mutated by install (deep clone)
	if entryU.LastWrittenFID != 0 {
		t.Fatal("entry fact object must stay fid 0")
	}
	ClearError()
}

func TestValidateAndUpdateFactsMapInKeepsPreUnions(t *testing.T) {
	// Statement.cpp:600–605 inputs_copy before visit; set_fact_in(pre full FactVec).
	ClearError()
	ut := &Type{isUnion: true, StructName: "U_vin2", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_vin2", ut, false, false)
	parent.CreateFieldVars()
	pre := MakeFactUnion(parent, 0)
	fm := NewFactMgr(nil)
	fm.UnionFacts = []*FactUnion{pre}
	// deep snapshot then in-place mutate live (Join path)
	snap := CloneUnionFactSliceDeep(fm.UnionFacts)
	fm.UnionFacts[0].LastWrittenFID = 1
	if snap[0].LastWrittenFID != 0 {
		t.Fatal("deep clone must isolate lattice from live mutate")
	}
	fm.SetMapFactsInPair(88, []*FactPointTo{}, snap)
	fm.SetMapFactsOutPair(88, []*FactPointTo{}, fm.UnionFacts)
	got := fm.GetMapUnionFactsIn(88)
	if len(got) != 1 || got[0].LastWrittenFID != 0 {
		t.Fatalf("map_in pre fid 0, got %#v", got)
	}
	ClearError()
}

func TestShortcutAnalysisReuse(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 5, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(5, facts)
	fm.SetMapFactsOut(5, facts)
	fm.SetMapStmEffect(5, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// first: no prior in match empty — SameFacts empty empty works
	sc := ShortcutAnalysis(st, &facts, &cg, Defaults())
	if sc != ShortcutOK {
		t.Fatal(sc)
	}
	// ctrl stmt never shortcuts
	st2 := &Stmt{Kind: StmtReturn, StmID: 6}
	fm.SetMapFactsIn(6, facts)
	if ShortcutAnalysis(st2, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("ctrl")
	}
	// Statement + facts + CGContext always live; sticky
	// Nil FM / StmID≤0 stay non-sticky ShortcutNone
	ClearError()
	if ShortcutAnalysis(nil, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("nil stmt must ShortcutNone")
	}
	if !HasError() {
		t.Fatal("nil stmt ShortcutAnalysis must SetError sticky")
	}
	ClearError()
	if ShortcutAnalysis(st, nil, &cg, Defaults()) != ShortcutNone {
		t.Fatal("nil facts must ShortcutNone")
	}
	if !HasError() {
		t.Fatal("nil facts ShortcutAnalysis must SetError sticky")
	}
	ClearError()
	if ShortcutAnalysis(st, &facts, nil, Defaults()) != ShortcutNone {
		t.Fatal("nil cg must ShortcutNone")
	}
	if !HasError() {
		t.Fatal("nil cg ShortcutAnalysis must SetError sticky")
	}
	ClearError()
	cgNoFM := EmptyCGContext()
	if ShortcutAnalysis(st, &facts, &cgNoFM, Defaults()) != ShortcutNone {
		t.Fatal("nil FM must ShortcutNone")
	}
	if HasError() {
		t.Fatal("nil FM ShortcutAnalysis must stay non-sticky soft re-pick")
	}
	ClearError()
	st0 := &Stmt{Kind: StmtAssign, StmID: IncompleteStmID}
	if ShortcutAnalysis(st0, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("StmID 0 must ShortcutNone")
	}
	if HasError() {
		t.Fatal("StmID 0 ShortcutAnalysis must stay non-sticky soft re-pick")
	}
	ClearError()
	// Block shortcut: hard IR sticky; FM/StmID 0 non-sticky
	b := &Block{StmID: 9}
	fm.SetMapFactsIn(9, facts)
	fm.SetMapFactsOut(9, facts)
	fm.SetMapStmEffect(9, EmptyEffect())
	if ShortcutAnalysisBlock(nil, &facts, &cg) != ShortcutNone {
		t.Fatal("nil block must ShortcutNone")
	}
	if !HasError() {
		t.Fatal("nil block ShortcutAnalysisBlock must SetError sticky")
	}
	ClearError()
	if ShortcutAnalysisBlock(b, &facts, &cgNoFM) != ShortcutNone {
		t.Fatal("nil FM block shortcut must ShortcutNone")
	}
	if HasError() {
		t.Fatal("nil FM ShortcutAnalysisBlock must stay non-sticky soft re-pick")
	}
	ClearError()
}

func TestShortcutConflict(t *testing.T) {
	ClearError()
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	st := &Stmt{Kind: StmtAssign, StmID: 3}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(3, facts)
	fm.SetMapFactsOut(3, facts)
	// previous effect wrote g_x
	fm.SetMapStmEffect(3, EmptyEffect().WriteVar(g))
	// ambient context also wrote g_x → InConflict
	cg := WithEffectContext(EmptyEffect().WriteVar(g))
	cg.FM = fm
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutConflict {
		t.Fatal("want conflict")
	}
}

func TestValidateAndUpdateFacts(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 9, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if !ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("validate")
	}
	if !fm.MapVisited[9] {
		t.Fatal("visited")
	}
	// second pass with same facts → shortcut
	if !ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("shortcut")
	}
}

func TestValidateAndUpdateFactsMarksContainedGotos(t *testing.T) {
	// Statement.cpp:580–595 — shortcut reuse marks gotos inside tree visited
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	gotoSt := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	// for-like compound with nested goto
	loop := &Stmt{
		Kind: StmtFor, StmID: 10,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
		Then: &Block{Stmts: []Stmt{gotoSt}},
	}
	fm := NewFactMgr(nil)
	// seed maps so shortcut fires
	fm.SetMapFactsIn(10, nil)
	fm.SetMapFactsOut(10, nil)
	fm.SetMapStmEffect(10, EmptyEffect())
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10, BackLink: true}}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if !ValidateAndUpdateFacts(loop, &facts, &cg, Defaults(), nil) {
		t.Fatal("shortcut validate")
	}
	if !fm.MapVisited[20] {
		t.Fatal("goto inside for should be marked visited on shortcut")
	}
	if !fm.MapVisited[10] {
		t.Fatal("loop itself visited")
	}
}

func TestMarkContainedGotosVisitedCFGHoleNoPartial(t *testing.T) {
	// soft invent: mark first goto then stop on nil CFG edge
	// fair: incomplete CFG sticky — mark none (no invent partial / soft re-pick)
	ClearError()
	gotoSt := Stmt{Kind: StmtGoto, StmID: 20, GotoDestStmID: 10}
	root := &Stmt{Kind: StmtFor, StmID: 10, Then: &Block{Stmts: []Stmt{gotoSt}}}
	fm := NewFactMgr(nil)
	fm.MapVisited = map[int]bool{}
	// hole after a valid edge that would mark goto 20
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 20, DestStmID: 10, BackLink: true},
		nil,
		{SrcID: 30, DestStmID: 10},
	}
	MarkContainedGotosVisited(root, fm)
	if fm.MapVisited[20] {
		t.Fatal("incomplete CFG must not invent partial goto visited mark")
	}
	if !HasError() {
		t.Fatal("incomplete CFG MarkContainedGotosVisited must SetError sticky")
	}
	ClearError()
	// Statement + FactMgr always live; sticky (no invent soft-skip mark past hole)
	MarkContainedGotosVisited(nil, fm)
	if !HasError() {
		t.Fatal("nil root MarkContainedGotosVisited must SetError sticky")
	}
	ClearError()
	MarkContainedGotosVisited(root, nil)
	if !HasError() {
		t.Fatal("nil fm MarkContainedGotosVisited must SetError sticky")
	}
	ClearError()
}

func TestCGContextAddEffect(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.AddEffect(EmptyEffect().WriteVar(v), false)
	if !cg.AccumEffect().IsWritten(v) || !cg.EffectStm.IsWritten(v) {
		t.Fatal("add")
	}
}

func TestContainsStmt(t *testing.T) {
	inner := Stmt{Kind: StmtAssign, StmID: 2}
	// StatementIf always has both arms
	outer := Stmt{Kind: StmtIfElse, StmID: 1, Then: &Block{Stmts: []Stmt{inner}}, Else: &Block{}}
	if !ContainsStmt(&outer, &inner) {
		t.Fatal("contains")
	}
	// Statement always live; sticky no invent not-contained soft-skip
	ClearError()
	if ContainsStmt(nil, &inner) {
		t.Fatal("nil root ContainsStmt must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil root ContainsStmt must SetError sticky")
	}
	ClearError()
	if ContainsStmt(&outer, nil) {
		t.Fatal("nil target ContainsStmt must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil target ContainsStmt must SetError sticky")
	}
	ClearError()
	if FindStmtInTree(nil, 1) != nil {
		t.Fatal("nil FindStmtInTree must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FindStmtInTree must SetError sticky")
	}
	ClearError()
	if FindStmtInTree(&outer, IncompleteStmID) != nil {
		t.Fatal("stmID 0 FindStmtInTree must fail closed")
	}
	if !HasError() {
		t.Fatal("stmID 0 FindStmtInTree must SetError sticky")
	}
	ClearError()
	if BlockContainsStmt(nil, &inner) {
		t.Fatal("nil BlockContainsStmt must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil BlockContainsStmt must SetError sticky")
	}
	ClearError()
}

func TestStmVisitFactsRemoveRVAndAlwaysVisited(t *testing.T) {
	// Statement.cpp:609–626 — remove_rv_facts; map_visited even when visit fails
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	f.RV.Name = "func_1_rv"
	otherRV := CreateVariableScalars("func_2_rv", GetIntType(), false, false)
	otherRV.Name = "func_2_rv"
	// mark as RVs via naming convention used by IsRV
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 42, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(f)
	// inject foreign RV into working facts
	facts := []*FactPointTo{
		MakeFactPointTo(otherRV, GarbagePtr),
		MakeFactPointTo(f.RV, v),
	}
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatal("visit assign should ok")
	}
	if !fm.MapVisited[42] {
		t.Fatal("always visited")
	}
	// foreign RV dropped; own RV kept
	for _, pt := range facts {
		if pt != nil && pt.Var != nil && pt.Var.Name == "func_2_rv" {
			t.Fatal("foreign rv must be removed")
		}
	}
	// validate sets fact_in from pre-visit copy
	pre := []*FactPointTo{MakeFactPointTo(v, GarbagePtr)}
	st2 := &Stmt{
		Kind: StmtAssign, StmID: 43, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(2)}, AssignOp: AssignSimple,
	}
	work := CloneFactSlice(pre)
	if !ValidateAndUpdateFacts(st2, &work, &cg, Defaults(), nil) {
		t.Fatal("validate")
	}
	in := fm.MapFactsIn[43]
	if len(in) != 1 || in[0] == nil || in[0].Var != v {
		t.Fatalf("fact_in should be pre-visit copy: %+v", in)
	}
}

func TestStmVisitFactsMarksVisitedOnFail(t *testing.T) {
	// visit fail (write IV) still marks visited per C++ stm_visit_facts
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 77, LhsVar: iv, Lhs: &Lhs{Var: iv, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	cg.IVBounds = map[*Variable]int{iv: 10}
	facts := []*FactPointTo{}
	FailedStm = nil
	ok := StmVisitFacts(st, &facts, &cg, Defaults())
	if ok {
		t.Fatal("expected visit fail writing IV")
	}
	if !fm.MapVisited[77] {
		t.Fatal("visited must be set even on fail")
	}
	// Statement.cpp:615–617 — failed_stm records non-compound failure
	if FailedStm != st {
		t.Fatal("FailedStm must be the failing assign")
	}
}

func TestContainsUnfixedGotoFindStmtResidualSticky(t *testing.T) {
	// FindStmt residual soft invent was soft-continue skip then invent fixed tree later.
	// Fair: residual sticky restrictive unfixed true.
	ClearError()
	defer ClearError()
	f := &Function{Name: "f"}
	// incomplete if sole Blocks entry — FindStmt residual when classifying edge src
	outer := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtIfElse, StmID: 1, Then: &Block{Stmts: []Stmt{{Kind: StmtGoto, StmID: 20}}}, Else: nil},
		{Kind: StmtAssign, StmID: 10},
	}}
	f.Blocks = []*Block{outer}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	root := &Stmt{Kind: StmtBlock, Then: outer, StmID: 99}
	if !ContainsUnfixedGoto(root, fm) {
		t.Fatal("FindStmt residual must fail closed unfixed, not invent fixed")
	}
	if !HasError() {
		t.Fatal("FindStmt residual ContainsUnfixedGoto must SetError sticky")
	}
	ClearError()
	// residual after containsUnfixedGotoIDs residual soft invent was invent fixed false.
	// Fair: sticky unfixed true. incomplete CFG residual.
	fm2 := NewFactMgr(f)
	fm2.CFGEdges = []*CFGEdge{nil}
	if !ContainsUnfixedGoto(root, fm2) {
		t.Fatal("CFG residual ContainsUnfixedGoto must fail closed unfixed")
	}
	if !HasError() {
		t.Fatal("CFG residual ContainsUnfixedGoto must SetError sticky")
	}
	ClearError()
}

func TestContainsUnfixedGotoImply(t *testing.T) {
	// Statement.cpp:797–800 — dest fact not imply jump_src → unfixed
	f := &Function{Name: "f"}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	// dest in: p→{a}; jump src out: p→{a,b} — dest does not imply src (narrower dest
	// imply wider src? Imply is other ⊆ this, so dest.Imply(src) means src ⊆ dest.
	// C++: !f->imply(*jump_src_f) with f = dest in, jump_src = src out.
	// If dest is {a} and src is {a,b}, {a}.Imply({a,b}) is false (b not in dest).
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 10, SourceLabel: "lbl"},
		{Kind: StmtGoto, StmID: 20, Label: "lbl", GotoDestStmID: 10},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 20, DestStmID: 10}}
	fm.MapVisited = map[int]bool{20: true}
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointToSet(p, []*Variable{a, b})})
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, a)})
	root := &Stmt{Kind: StmtBlock, Then: body, StmID: 1}
	if !ContainsUnfixedGoto(root, fm) {
		t.Fatal("expect unfixed when dest does not imply jump src")
	}
	// equal sets → fixed
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointTo(p, a)})
	if ContainsUnfixedGoto(root, fm) {
		t.Fatal("equal should be fixed")
	}
	// Imply residual: PointTo nil hole soft invent was soft-continue then invent fixed.
	// Fair: sticky unfixed true.
	ClearError()
	broken := &FactPointTo{Var: p, PointTo: []*Variable{a, nil}}
	fm.SetMapFactsOut(20, []*FactPointTo{MakeFactPointTo(p, a)})
	fm.SetMapFactsIn(10, []*FactPointTo{broken})
	if !ContainsUnfixedGoto(root, fm) {
		t.Fatal("Imply residual must fail closed unfixed")
	}
	if !HasError() {
		t.Fatal("Imply residual ContainsUnfixedGoto must SetError sticky")
	}
	ClearError()
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, a)})
	// incomplete srcOut hole: fail closed unfixed (no invent fixed past hole)
	fm.MapFactsOut[20] = []*FactPointTo{MakeFactPointTo(p, a), nil}
	if !ContainsUnfixedGoto(root, fm) {
		t.Fatal("incomplete MapFactsOut must fail closed unfixed")
	}
	// Statement/Block always live; sticky unfixed (no invent all-fixed soft-skip)
	ClearError()
	if !ContainsUnfixedGoto(nil, fm) {
		t.Fatal("nil root ContainsUnfixedGoto must fail closed unfixed")
	}
	if !HasError() {
		t.Fatal("nil root ContainsUnfixedGoto must SetError sticky")
	}
	ClearError()
	if !ContainsUnfixedGotoBlock(nil, fm) {
		t.Fatal("nil Block ContainsUnfixedGotoBlock must fail closed unfixed")
	}
	if !HasError() {
		t.Fatal("nil Block ContainsUnfixedGotoBlock must SetError sticky")
	}
	ClearError()
}

func TestShortcutAnalysisBlockUnfixedGoto(t *testing.T) {
	// block shortcut must not fire with unfixed internal goto
	f := &Function{Name: "f"}
	body := &Block{Func: f, StmID: 50, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 51},
		{Kind: StmtGoto, StmID: 52, Label: "elsewhere", GotoDestStmID: 99},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 52, DestStmID: 99}} // dest outside
	// unvisited goto
	fm.MapVisited = map[int]bool{}
	fm.SetMapFactsIn(50, nil)
	fm.SetMapFactsOut(50, nil)
	fm.SetMapStmEffect(50, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("unfixed goto must block shortcut")
	}
}

func TestStmVisitFactsIncompleteInputFailClosed(t *testing.T) {
	// Fact* always live; incomplete working set sticky (no invent visit success)
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 88, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{MakeFactPointTo(v, GarbagePtr), nil}
	if StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatal("incomplete inputs must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete inputs StmVisitFacts must SetError sticky")
	}
	if fm.MapVisited[88] {
		t.Fatal("must not mark visited when inputs incomplete before visit")
	}
	ClearError()
}

func TestValidateAndUpdateFactsIncompleteInputFailClosed(t *testing.T) {
	// incomplete pre-visit inputs sticky (no invent set_fact_in from cleaned clone)
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 90, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{MakeFactPointTo(v, GarbagePtr), nil}
	if ValidateAndUpdateFacts(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete inputs must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete inputs ValidateAndUpdateFacts must SetError sticky")
	}
	if _, ok := fm.MapFactsIn[90]; ok {
		t.Fatal("must not invent MapFactsIn from incomplete inputs")
	}
	ClearError()
}

func TestShortcutAnalysisMissingOutFailClosed(t *testing.T) {
	// Statement.cpp:559 — inputs = map_facts_out[this]
	// missing out must not invent ShortcutOK while leaving inputs unchanged
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 7, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(7, facts)
	// no MapFactsOut[7]
	fm.SetMapStmEffect(7, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("missing MapFactsOut must fail closed ShortcutNone")
	}
	if fm.MapVisited[7] {
		t.Fatal("must not mark visited on incomplete shortcut")
	}
}

func TestShortcutAnalysisIncompleteOutFailClosed(t *testing.T) {
	// nil fact hole in MapFactsOut — no invent clone-to-nil while ShortcutOK
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 8, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.SetMapFactsIn(8, in)
	// plant hole bypassing SetMapFactsOut (CloneFactSlice strips holes)
	fm.MapFactsOut = map[int][]*FactPointTo{
		8: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	fm.SetMapStmEffect(8, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := CloneFactSlice(in)
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("incomplete MapFactsOut must fail closed ShortcutNone")
	}
	// inputs must not be replaced with invent-cleaned clone
	if !SameFacts(facts, in) {
		t.Fatal("facts must stay pre-shortcut inputs on fail closed")
	}
}

func TestShortcutAnalysisBlockMissingOutFailClosed(t *testing.T) {
	body := &Block{StmID: 60, Stmts: []Stmt{{Kind: StmtAssign, StmID: 61}}}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(60, facts)
	// no MapFactsOut[60]
	fm.SetMapStmEffect(60, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("block missing MapFactsOut must fail closed")
	}
}

func TestShortcutAnalysisBlockIncompleteOutFailClosed(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	body := &Block{StmID: 70, Stmts: []Stmt{{Kind: StmtAssign, StmID: 71}}}
	fm := NewFactMgr(nil)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.SetMapFactsIn(70, in)
	fm.MapFactsOut = map[int][]*FactPointTo{
		70: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	fm.SetMapStmEffect(70, EmptyEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := CloneFactSlice(in)
	if ShortcutAnalysisBlock(body, &facts, &cg) != ShortcutNone {
		t.Fatal("block incomplete MapFactsOut must fail closed")
	}
}

func TestShortcutAnalysisIncompleteEffectFailClosed(t *testing.T) {
	// incomplete map_stm_effect / accum must not invent ShortcutOK
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 9, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	facts := []*FactPointTo{}
	fm.SetMapFactsIn(9, facts)
	fm.SetMapFactsOut(9, facts)
	fm.SetMapStmEffect(9, IncompleteEffect())
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("incomplete map_stm_effect must fail closed ShortcutNone")
	}
	// incomplete parent accum
	fm.SetMapStmEffect(9, EmptyEffect())
	inc := IncompleteEffect()
	cg.EffectAccum = &inc
	if ShortcutAnalysis(st, &facts, &cg, Defaults()) != ShortcutNone {
		t.Fatal("incomplete EffectAccum must fail closed ShortcutNone")
	}
	ClearError()
}

func TestStmVisitFactsIncompleteAccumFailClosed(t *testing.T) {
	// incomplete EffectAccum must not invent StmVisitFacts true while recording map_accum
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 90, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	inc := IncompleteEffect()
	cg.EffectAccum = &inc
	facts := []*FactPointTo{}
	if StmVisitFacts(st, &facts, &cg, Defaults()) {
		t.Fatal("incomplete EffectAccum must fail closed StmVisitFacts")
	}
	// C++ still records map_accum / visited — incomplete marker, not invent pure
	if !fm.MapVisited[90] {
		t.Fatal("must still mark visited")
	}
	if EffectComplete(fm.GetMapAccumEffect(90)) {
		t.Fatal("map_accum must stay incomplete marker")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	// StmID 0 fails closed sticky (no invent soft-skip map_accum/visited)
	st0 := &Stmt{
		Kind: StmtAssign, StmID: IncompleteStmID, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts0 := []*FactPointTo{}
	if StmVisitFacts(st0, &facts0, &cg, Defaults()) {
		t.Fatal("IncompleteStmID must fail closed StmVisitFacts")
	}
	if FactsComplete(facts0) {
		t.Fatal("StmID 0 must wipe facts incomplete")
	}
	if !HasError() {
		t.Fatal("StmID 0 must SetError sticky")
	}
	ClearError()
}
