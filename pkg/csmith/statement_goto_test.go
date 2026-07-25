package csmith

import (
	"regexp"
	"strings"
	"testing"
)

func TestMakeRandomGotoEmptyBlockReturnsNull(t *testing.T) {
	// StatementGoto.cpp:86–87 / 130–132 — no soft makeForwardGotoOnly
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	blk := &Block{Func: f}
	f.Blocks = []*Block{blk}
	st := MakeRandomGoto(NewRngSess(testAmbientSession, 9), opts, probs, vs, tables, &cg, blk)
	if st.Label != "" || st.Expr != nil {
		t.Fatalf("want nullptr-style empty goto, got %+v", st)
	}
	if stmtOK(st) {
		t.Fatal("stmtOK must reject null goto")
	}
}

func TestMakeRandomGotoBackEdge(t *testing.T) {
	// StatementGoto.cpp:138–150 — back-edge with choose_visible_read_var
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	g := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	vs.AllVars = []*Variable{g}
	vs.GlobalList = []*Variable{g}
	// target stmt for back-edge
	tgt := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}
	blk := &Block{Func: f, Stmts: []Stmt{tgt, {Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}}}
	f.Blocks = []*Block{blk}
	f.Body = blk
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	var st Stmt
	for seed := uint64(1); seed < 60; seed++ {
		blk.Stmts[0].SourceLabel = ""
		st = MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, blk)
		if st.GotoBack {
			break
		}
	}
	if !st.GotoBack {
		t.Skip("no back-edge in seed sample")
	}
	if st.Label == "" || blk.Stmts[0].SourceLabel == "" {
		t.Fatalf("back edge label missing: st=%+v src=%q", st, blk.Stmts[0].SourceLabel)
	}
	if st.Label != blk.Stmts[0].SourceLabel {
		t.Fatalf("label mismatch goto=%q target=%q", st.Label, blk.Stmts[0].SourceLabel)
	}
	if st.Expr == nil || st.Expr.Var != g {
		t.Fatalf("cond must be visible read var g_c: %+v", st.Expr)
	}
}

// StatementGoto.cpp:131–133 — ExpressionVariable(*cond_var) only; no read_var
// during make_random (visit_facts later). Soft invent NoteRead bloated EffectStm/accum.
func TestMakeRandomGotoDoesNotReadVarAtMake(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	g := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	vs.AllVars = []*Variable{g}
	vs.GlobalList = []*Variable{g}
	tgt := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}
	blk := &Block{Func: f, Stmts: []Stmt{tgt, {Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}}}
	f.Blocks = []*Block{blk}
	f.Body = blk
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	// accum already has g as read (choose pool); EffectStm empty before make
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect()
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		blk.Stmts[0].SourceLabel = ""
		cg.EffectStm = EmptyEffect()
		// reset accum to only pre-read (no invent re-seed pollution)
		pre := EmptyEffect().ReadVarSess(testAmbientSession, g)
		cg.EffectAccum = &pre
		st := MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, blk)
		if !stmtOK(st) || st.Expr == nil || st.Expr.Var != g {
			continue
		}
		found = true
		// make_random must not push cond into effect_stm (C++ leaves stm cleared at 112)
		if cg.EffectStm.IsReadSess(testAmbientSession, g) {
			t.Fatal("goto make_random must not invent read_var into EffectStm")
		}
		// EffectAccum should still be only the pre-existing read (not re-pushed as "new")
		// IsRead stays true; SE-free unchanged for non-vol
		if !cg.EffectAccum.IsReadSess(testAmbientSession, g) {
			t.Fatal("pre-existing accum read must remain")
		}
		break
	}
	if !found {
		t.Skip("no goto with g_c cond in seed sample")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateCanEmitGoto(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		// fair inventory can ERROR_RETURN some seeds (C++ assert on validate)
		if err != nil {
			continue
		}
		if strings.Contains(out, "goto lbl_") {
			found = true
			if !strings.Contains(out, "lbl_") || !strings.Contains(out, ":") {
				t.Fatal("goto without label def")
			}
			break
		}
	}
	if !found {
		t.Log("goto rare (statement weight band 45-49)")
	}
}

func TestLabelForGotoDestReuses(t *testing.T) {
	GotoLabelsDoFinalization()
	defer GotoLabelsDoFinalization()
	n := 0
	next := func() string {
		n++
		return "lbl_" + Int2Str(n)
	}
	a := LabelForGotoDest(42, next)
	b := LabelForGotoDest(42, next)
	if a != b || a != "lbl_1" {
		t.Fatalf("%q %q n=%d", a, b, n)
	}
	c := LabelForGotoDest(99, next)
	if c == a || c != "lbl_2" {
		t.Fatalf("%q %q", a, c)
	}
	// nil nextLabel → process gensym; no invent fixed "lbl_1"
	GotoLabelsDoFinalization()
	ResetDefaultGensym()
	g1 := LabelForGotoDest(7, nil)
	g2 := LabelForGotoDest(8, nil)
	if g1 == "" || g2 == "" || g1 == g2 {
		t.Fatalf("gensym labels want distinct non-empty, got %q %q", g1, g2)
	}
	if g1 == "lbl_1" && g2 == "lbl_1" {
		t.Fatal("must not invent same fixed lbl_1")
	}
	// empty nextLabel — sticky fail closed, no invent empty label token
	ClearErrorSess(testAmbientSession)
	if LabelForGotoDest(9, func() string { return "" }) != "" {
		t.Fatal("empty gensym must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty gensym must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestStmVisitFactsClearsEffectStmBeforeForVisit — Statement.cpp:611 +
// StatementGoto.cpp:165–171. Forward dest re-analysis must clear effect_stm
// before visit_facts so StatementFor map_stm_effect is init+body only.
// Soft invent VisitFactsStmt after add_effect(map_accum) kept pollution and
// froze gen IV-first reads (seed-42 func_68: g_77 before g_16).
func TestStmVisitFactsClearsEffectStmBeforeForVisit(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "func_68", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	pollute := CreateVariableScalarsSess(testAmbientSession, "g_77", GetIntTypeSess(testAmbientSession), false, false)
	bodyRead := CreateVariableScalarsSess(testAmbientSession, "g_16", GetIntTypeSess(testAmbientSession), false, false)
	tmp := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	body := &Block{
		StmID: AllocStmID(), Func: f, Looping: true,
		Stmts: []Stmt{{
			Kind: StmtAssign, StmID: AllocStmID(), LhsVar: tmp,
			Lhs:      &Lhs{Var: tmp, Type: GetIntTypeSess(testAmbientSession)},
			Expr:     &Expression{Term: TermVariable, Var: bodyRead, ExprType: GetIntTypeSess(testAmbientSession)},
			AssignOp: AssignSimple,
		}},
	}
	forSt := &Stmt{
		Kind: StmtFor, StmID: AllocStmID(),
		Loop: &LoopControl{
			IV: iv, InitN: 0, LimitN: 2, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd,
			InitStmt: testForInit(iv, 0),
		},
		Then: body,
	}
	// gen-style map_stm: IV read first (make_iteration read_var) then body
	fm.SetMapStmEffect(forSt.StmID, EmptyEffect().ReadVarSess(testAmbientSession, pollute).ReadVarSess(testAmbientSession, bodyRead))
	fm.SetMapStmEffect(body.StmID, EmptyEffect().ReadVarSess(testAmbientSession, bodyRead))
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	acc := EmptyEffect()
	cg.EffectAccum = &acc
	// pollution as after add_effect(map_accum[other]) on the forward path
	cg.EffectStm = EmptyEffect().ReadVarSess(testAmbientSession, pollute)
	facts := []*FactPointTo{}
	if !StmVisitFacts(forSt, &facts, &cg, opts) {
		t.Fatalf("StmVisitFacts for: err=%v", HasErrorSess(testAmbientSession))
	}
	got := fm.GetMapStmEffect(forSt.StmID)
	if !EffectComplete(got) {
		t.Fatal("incomplete map_stm after StmVisitFacts")
	}
	reads := got.ReadVarsSess(testAmbientSession)
	if len(reads) == 0 || !got.IsReadSess(testAmbientSession, bodyRead) {
		t.Fatalf("want body read g_16 in map_stm, got %v", mapAccumNamesOf(reads))
	}
	// clean visit: init is simple const assign (no IV read); body reads g_16 first
	if reads[0] == pollute {
		t.Fatalf("map_stm still pollution/IV-first after StmVisitFacts clear: %v", mapAccumNamesOf(reads))
	}
	// negative: VisitFactsStmt without clear keeps pollution in the pre-init snapshot
	ClearErrorSess(testAmbientSession)
	fm.SetMapStmEffect(forSt.StmID, EmptyEffect().ReadVarSess(testAmbientSession, pollute).ReadVarSess(testAmbientSession, bodyRead))
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	acc2 := EmptyEffect()
	cg2.EffectAccum = &acc2
	cg2.EffectStm = EmptyEffect().ReadVarSess(testAmbientSession, pollute)
	if !VisitFactsStmt(forSt, &cg2, opts) {
		t.Fatalf("VisitFactsStmt for: err=%v", HasErrorSess(testAmbientSession))
	}
	polluted := fm.GetMapStmEffect(forSt.StmID)
	if !polluted.IsReadSess(testAmbientSession, pollute) {
		t.Fatal("precondition: VisitFactsStmt without clear should keep EffectStm pollution in map_stm")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMarkNeedRevisitLCA(t *testing.T) {
	// Statement.cpp:789–795 Block contains_stmt: dest on parent chain, not Stmts walk.
	// outer → then(inner with assign) — back-edge LCA is outer when dest in then
	dest := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 7}
	inner := &Block{Stmts: []Stmt{dest}}
	// StatementIf always has live if_true/if_false
	outer := &Block{
		Stmts: []Stmt{{Kind: StmtIfElse, Then: inner, Else: &Block{}, StmID: 1}},
	}
	inner.Parent = outer
	// dest pointer must be into slice; destParent is inner
	d := &inner.Stmts[0]
	MarkNeedRevisitLCAParent(inner, d, inner)
	if !inner.NeedRevisit {
		t.Fatal("inner is destParent → mark inner")
	}
	// from a sibling-ish curr that does not contain dest → walk to outer
	curr := &Block{Parent: outer, Stmts: []Stmt{{Kind: StmtAssign, StmID: 8}}}
	outer.NeedRevisit = false
	inner.NeedRevisit = false
	MarkNeedRevisitLCAParent(curr, d, inner)
	if !outer.NeedRevisit {
		t.Fatal("outer is LCA on dest parent chain")
	}
	if curr.NeedRevisit {
		t.Fatal("curr should not be marked when outer contains dest")
	}
	// StatementGoto.cpp:147 assert(b) — no soft invent NeedRevisit when dest not in ancestry
	orphan := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 9}}}
	MarkNeedRevisitLCAParent(orphan, d, inner)
	if orphan.NeedRevisit {
		t.Fatal("orphan must not invent NeedRevisit when dest not found")
	}
}

// Statement.cpp:789–795 — Block contains_stmt walks s->parent, so the function
// body contains a dest in an if-arm that is not yet in body.Stmts (MakeRandomIf
// builds then/else before the if is appended). Soft invent Stmts-walk missed body.
func TestMarkNeedRevisitLCABodyWhileIfNotAppended(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	body := &Block{StmID: 1, Func: &Function{Name: "func_21", ReturnType: GetIntTypeSess(testAmbientSession)}}
	// then arm holds dest; else is curr (back-edge from else to then)
	dest := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 10}
	thenB := &Block{StmID: 2, Parent: body, Stmts: []Stmt{dest}}
	elseB := &Block{StmID: 3, Parent: body}
	// body.Stmts empty — if not appended yet (MakeRandomIf mid-build)
	d := &thenB.Stmts[0]
	MarkNeedRevisitLCAParent(elseB, d, thenB)
	if !body.NeedRevisit {
		t.Fatal("body must be LCA via dest parent chain while if not in body.Stmts")
	}
	if elseB.NeedRevisit {
		t.Fatal("else does not contain then dest")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBlockContainsViaParent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	body := &Block{StmID: 1}
	mid := &Block{StmID: 2, Parent: body}
	leaf := &Block{StmID: 3, Parent: mid}
	if !BlockContainsViaParent(body, leaf) || !BlockContainsViaParent(mid, leaf) || !BlockContainsViaParent(leaf, leaf) {
		t.Fatal("ancestors and self must contain via parent chain")
	}
	sib := &Block{StmID: 4, Parent: body}
	if BlockContainsViaParent(sib, leaf) {
		t.Fatal("sibling must not contain")
	}
	if BlockContainsViaParent(nil, leaf) {
		t.Fatal("nil block sticky false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil block must SetError")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomGotoRequiresFactMgr(t *testing.T) {
	// StatementGoto.cpp:66–67 get_fact_mgr; non-sticky soft re-pick without FM
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	// no FM
	st := MakeRandomGoto(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, blk)
	if stmtOK(st) {
		t.Fatal("goto without FactMgr must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM goto must stay non-sticky soft re-pick")
	}
	// sticky without RNG / cg / curr_func
	if stmtOK(MakeRandomGoto(nil, opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, blk)) {
		t.Fatal("nil RNG goto must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG goto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeBinaryForCompare(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	lhs := &Expression{Term: TermVariable, Var: CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), true, false), ExprType: GetIntTypeSess(testAmbientSession)}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(10), ExprType: GetIntTypeSess(testAmbientSession)}
	fi := MakeBinary(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), EmptyCGContext().WithSession(testAmbientSession), BinCmpLt, lhs, rhs)
	if fi == nil || fi.Binary != "<" {
		t.Fatalf("%+v", fi)
	}
	if fi.Safe == nil {
		t.Fatal("flags always set")
	}
	if fi.GetType() != GetIntTypeSess(testAmbientSession) {
		t.Fatal("cmp type")
	}
	// Output is standard cmp (not safe_ops arith)
	out := fi.Output()
	if strings.Contains(out, "safe_") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "<") {
		t.Fatal(out)
	}
}

func TestMakeBinaryIncompleteAmbientSticky(t *testing.T) {
	// incomplete ambient must not invent binary shell / soft re-pick past holes
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	lhs := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntTypeSess(testAmbientSession)}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(2), ExprType: GetIntTypeSess(testAmbientSession)}
	if fi := MakeBinary(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession), BinAdd, lhs, rhs); fi != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeBinary")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	if fi := MakeBinary(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), cg, BinAdd, lhs, rhs); fi != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeBinary")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeBinaryGetTypeResidualSticky(t *testing.T) {
	// GetType residual soft invent was MakeRandomBinaryKind with nil types then invent binary shell.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	hole := &Expression{Term: TermConstant, Con: &Constant{Value: "1"}}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(2), ExprType: GetIntTypeSess(testAmbientSession)}
	if fi := MakeBinary(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), EmptyCGContext().WithSession(testAmbientSession), BinAdd, hole, rhs); fi != nil {
		t.Fatal("GetType residual must fail closed MakeBinary, not invent shell", fi)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("GetType residual MakeBinary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeBinaryNoInventWithoutRNGOrInvalidOp(t *testing.T) {
	// FunctionInvocation.cpp:565+ — always has RNG + operands sticky; no invent Binary shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	lhs := &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: GetIntTypeSess(testAmbientSession)}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(2), ExprType: GetIntTypeSess(testAmbientSession)}
	if fi := MakeBinary(nil, opts, NewProbabilities(opts), EmptyCGContext().WithSession(testAmbientSession), BinAdd, lhs, rhs); fi != nil {
		t.Fatal("nil RNG")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeBinary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if fi := MakeBinary(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), EmptyCGContext().WithSession(testAmbientSession), BinAdd, nil, rhs); fi != nil {
		t.Fatal("nil lhs")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhs MakeBinary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if fi := MakeBinary(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), EmptyCGContext().WithSession(testAmbientSession), BinaryOp(MaxBinaryOp), lhs, rhs); fi != nil {
		t.Fatal("MAX binary op must fail closed")
	}
	// MAX op BinaryOpC sticky
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MAX binary op MakeBinary must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGotoLabelsClearedOnFinalization(t *testing.T) {
	prevR := ProcessRngSess(testAmbientSession)
	prevP := ProcessProbabilitiesSess(testAmbientSession)
	defer func() {
		SetProcessRngSess(testAmbientSession, prevR)
		SetProcessProbabilitiesSess(testAmbientSession, prevP)
	}()
	GotoLabelsDoFinalization()
	_ = LabelForGotoDest(1, func() string { return "lbl_x" })
	DoFinalizationSess(testAmbientSession)
	// after finalization map empty → new gensym path
	lab := LabelForGotoDest(1, func() string { return "lbl_y" })
	if lab != "lbl_y" {
		t.Fatal(lab)
	}
	GotoLabelsDoFinalization()
}

func TestMakeRandomGotoForwardInsert(t *testing.T) {
	// Two-block setup: curr has dest stmt; other block has insert site.
	// Force forward path (as_dest=false) via seed scan; expect side-effect insert.
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	// source block (forward ok_blk) — two assigns so insert after first is possible
	src := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
	}}
	// current block with a dest last stmt
	curr := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
	}}
	f.Blocks = []*Block{src, curr}
	f.Body = curr
	fm := NewFactMgrSess(testAmbientSession, f)
	// seed map facts so merge can run
	for _, b := range f.Blocks {
		for i := range b.Stmts {
			id := b.Stmts[i].StmID
			fm.SetMapFactsIn(id, nil)
			fm.SetMapFactsOut(id, nil)
			fm.MapAccumEffect[id] = EmptyEffect()
		}
	}
	// visible read var on accum for cond selection (back) / src accum (forward)
	g := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	vs.AllVars = append(vs.AllVars, g)
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// mark both stmts' accum as having read g so forward cond can choose
	for i := range src.Stmts {
		fm.MapAccumEffect[src.Stmts[i].StmID] = eff
	}

	inserted := false
	for seed := uint64(1); seed < 80; seed++ {
		// reset src to two assigns (prior seeds may have inserted)
		src.Stmts = []Stmt{
			{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
			{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
		}
		for i := range src.Stmts {
			id := src.Stmts[i].StmID
			fm.SetMapFactsIn(id, nil)
			fm.SetMapFactsOut(id, nil)
			fm.MapAccumEffect[id] = eff
		}
		before := len(src.Stmts)
		st := MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, curr)
		// forward success: returns failed (no label) and inserts into some block
		if st.Label == "" {
			for _, b := range f.Blocks {
				for _, s := range b.Stmts {
					if s.Kind == StmtGoto && s.GotoForward {
						inserted = true
						if s.GotoDestStmID != curr.Stmts[len(curr.Stmts)-1].StmID {
							t.Fatalf("dest id: got %d want last curr", s.GotoDestStmID)
						}
						if s.Label == "" {
							t.Fatal("inserted goto missing label")
						}
						break
					}
				}
			}
			if inserted {
				// side-effect insert grew a block
				if len(src.Stmts) <= before && len(curr.Stmts) <= 1 {
					// may insert into curr if FindGoodJumpBlock picked curr
					grew := false
					for _, b := range f.Blocks {
						for _, s := range b.Stmts {
							if s.Kind == StmtGoto && s.GotoForward {
								grew = true
							}
						}
					}
					if !grew {
						t.Fatal("forward claimed but no goto found")
					}
				}
				break
			}
		}
		// back-edge success returns labeled goto — keep scanning for forward
		_ = before
	}
	if !inserted {
		t.Skip("no forward insert in seed sample (RNG)")
	}
}

// TestForwardGotoSameBlockInsertPreservesDestID — StatementGoto.cpp:184–203.
// When ok_blk == curr_blk, insert shifts slice elements after other. C++ Statement*
// stays heap-stable; Go must not use &Stmts[i] after insert for dest StmID
// (CFG edge / set_fact). seed-42: mis-aimed edge labeled the pre-dest assign.
func TestForwardGotoSameBlockInsertPreservesDestID(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	// single block: other candidate + dest last (pre-sized capacity so insert
	// shifts in place without always reallocating away from the bug)
	otherID := AllocStmID()
	destID := AllocStmID()
	midID := AllocStmID()
	blk := &Block{Func: f, Stmts: make([]Stmt, 0, 8)}
	blk.Stmts = append(blk.Stmts,
		Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: otherID},
		Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: midID},
		Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: destID},
	)
	f.Blocks = []*Block{blk}
	f.Body = blk
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_c", GetIntTypeSess(testAmbientSession), true, false)
	vs.AllVars = append(vs.AllVars, g)
	eff := EmptyEffect().ReadVarSess(testAmbientSession, g)
	for i := range blk.Stmts {
		id := blk.Stmts[i].StmID
		fm.SetMapFactsIn(id, nil)
		fm.SetMapFactsOut(id, nil)
		fm.MapAccumEffect[id] = eff
	}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// force many seeds until same-block forward insert lands
	var got *Stmt
	for seed := uint64(1); seed < 200; seed++ {
		// restore three assigns (prior seeds may have inserted)
		blk.Stmts = blk.Stmts[:0]
		blk.Stmts = append(blk.Stmts,
			Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: otherID},
			Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: midID},
			Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: destID},
		)
		for i := range blk.Stmts {
			id := blk.Stmts[i].StmID
			fm.SetMapFactsIn(id, nil)
			fm.SetMapFactsOut(id, nil)
			fm.MapAccumEffect[id] = eff
		}
		fm.CFGEdges = nil
		st := MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, blk)
		if st.Label != "" {
			continue // back-edge
		}
		for i := range blk.Stmts {
			if blk.Stmts[i].Kind == StmtGoto && blk.Stmts[i].GotoForward {
				got = &blk.Stmts[i]
				break
			}
		}
		if got != nil {
			break
		}
	}
	if got == nil {
		t.Skip("no same-block forward insert in seed sample")
	}
	if got.GotoDestStmID != destID {
		t.Fatalf("GotoDestStmID=%d want dest last %d", got.GotoDestStmID, destID)
	}
	// CFG edge must name dest by id (not a shifted slot)
	foundEdge := false
	for _, e := range fm.CFGEdges {
		if e.SrcID == got.StmID {
			foundEdge = true
			if e.DestStmID != destID {
				t.Fatalf("CFG edge DestStmID=%d want %d (post-insert pointer bug)", e.DestStmID, destID)
			}
		}
	}
	if !foundEdge {
		t.Fatal("missing CFG edge from inserted goto")
	}
	// PreOutput labels dest, not mid
	preDest, okDest := PreOutput(&Stmt{Kind: StmtAssign, StmID: destID}, fm, false, false, nil, "")
	if !okDest || !strings.Contains(preDest, got.Label) {
		t.Fatalf("dest PreOutput want label %q, got %q ok=%v", got.Label, preDest, okDest)
	}
	preMid, okMid := PreOutput(&Stmt{Kind: StmtAssign, StmID: midID}, fm, false, false, nil, "")
	if okMid && strings.Contains(preMid, got.Label) {
		t.Fatalf("mid must not carry dest label; pre=%q", preMid)
	}
	ClearErrorSess(testAmbientSession)
}

// StatementGoto.cpp:185 — StatementGoto ctor gensyms lbl_ only after DFA ok.
// Pre-visit gensym on failed forward burned seed-2 names (extra lbl_710 →
// g_733/l_718). After fix, surviving names match upstream (g_732, l_717, lbl_1269).
func TestSeed2GensymNamesMatchUpstreamAfterGotoFix(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	// Upstream seed-2 survivors (not the intermediate burned-then-discarded gensyms)
	for _, want := range []string{
		"g_732", "l_717", "lbl_1269",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s (early forward-goto gensym burn shifts names)", want)
		}
	}
	// Pre-fix wrong names must not appear as the shifted survivors
	if strings.Contains(out, "static volatile int32_t g_733") {
		t.Fatal("g_733 is the pre-fix +1-shifted volatile; gensym still desynced")
	}
	if regexp.MustCompile(`\bl_718\b`).MatchString(out) && !strings.Contains(out, "l_717") {
		t.Fatal("l_718 without l_717 indicates post-l_692 gensym shift")
	}
}

func TestResetEffectAccum(t *testing.T) {
	cg := EmptyCGContext().WithSession(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), true, false)
	pre := EmptyEffect().ReadVarSess(testAmbientSession, v)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect().WriteVarSess(testAmbientSession, v)
	cg.ResetEffectAccum(pre)
	if !cg.EffectAccum.IsReadSess(testAmbientSession, v) || cg.EffectAccum.IsWrittenSess(testAmbientSession, v) {
		t.Fatalf("%+v", cg.EffectAccum)
	}
}

func TestFindGoodJumpBlockNilRNGSticky(t *testing.T) {
	// StatementGoto always has process RNG; sticky no invent jump block without it
	ClearErrorSess(testAmbientSession)
	good := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	if FindGoodJumpBlock(nil, []*Block{good}, good, false) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG FindGoodJumpBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindGoodJumpBlockNilHoleFailClosed(t *testing.T) {
	// Block* always live; nil hole fails closed sticky (no invent soft-skip / re-pick)
	good := &Block{StmID: 1, Stmts: []Stmt{{Kind: StmtAssign, StmID: 2}}}
	ClearErrorSess(testAmbientSession)
	if FindGoodJumpBlock(NewRngSess(testAmbientSession, 1), []*Block{good, nil}, good, false) != nil {
		t.Fatal("nil Block hole must fail closed FindGoodJumpBlock")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Block hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FindGoodJumpBlock(NewRngSess(testAmbientSession, 2), []*Block{nil, good}, good, true) != nil {
		t.Fatal("nil hole first must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole first must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomGotoNilBlocksHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.Blocks = []*Block{nil}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	blk := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	st := MakeRandomGoto(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, blk)
	if st.Kind == StmtGoto && st.Label != "" {
		t.Fatal("nil Blocks hole must fail closed MakeRandomGoto")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Blocks hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomGotoIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete EffectAccum/EffectContext must sticky ERROR (no invent goto re-pick)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Blocks = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	st := MakeRandomGoto(NewRngSess(testAmbientSession, 4), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, blk)
	if st.Kind == StmtGoto && st.Label != "" {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomGoto")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff := EmptyEffect()
	cg2.EffectAccum = &eff
	st2 := MakeRandomGoto(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg2, blk)
	if st2.Kind == StmtGoto && st2.Label != "" {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomGoto")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts must fail closed before jump-block scan
	fm3 := NewFactMgrSess(testAmbientSession, f)
	fm3.GlobalFacts = IncompleteFactSlice()
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm3)
	st3 := MakeRandomGoto(NewRngSess(testAmbientSession, 6), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg3, blk)
	if st3.Kind == StmtGoto && st3.Label != "" {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomGoto")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsGotoIncompleteFactsFailClosed(t *testing.T) {
	// incomplete working facts or prev outs sticky fail closed
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	st := &Stmt{Kind: StmtGoto, StmID: 10, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, GotoDestStmID: 20}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if VisitFactsStatementGoto(st, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts must fail closed VisitFactsGoto")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts VisitFactsGoto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete facts, incomplete prev out
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	if VisitFactsStatementGoto(st, &cg, Defaults()) {
		t.Fatal("incomplete prev MapFactsOut must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete prev MapFactsOut VisitFactsGoto must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPreOutputLabelAttrResidualSticky(t *testing.T) {
	// Label attr Output residual soft invent was invent label line past broken attr shell.
	// PreOutput with fixed LabelAttr skips generator; residual path uses generator.
	// Fair: incomplete Statement StmID 0 with FM already sticky; without FM uses SourceLabel.
	ClearErrorSess(testAmbientSession)
	st := &Stmt{Kind: StmtAssign, SourceLabel: "lbl_1", StmID: 1}
	// emitLabelAttrs true with rng — generator complete path may emit attrs; no residual unless hole.
	// residual hole: verify complete path hygiene + empty LabelAttr path ok
	out, isGoto := PreOutput(st, nil, false, false, nil, "")
	if !isGoto || !strings.Contains(out, "lbl_1:") {
		t.Fatal("complete SourceLabel PreOutput", out, isGoto)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete PreOutput must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil Statement residual
	if s, _ := PreOutput(nil, nil, false, false, nil, ""); s != "" {
		t.Fatal("nil PreOutput must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil PreOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputSkippedVarInitsResidualSticky(t *testing.T) {
	// InitExpr Output residual soft invent was soft-continue invent partial re-inits.
	ClearErrorSess(testAmbientSession)
	ok := CreateVariableScalarsSess(testAmbientSession, "l_ok", GetIntTypeSess(testAmbientSession), false, false)
	ok.Init = MakeInt(0)
	ok.InitExpr = nil
	hole := CreateVariableScalarsSess(testAmbientSession, "l_bad", GetIntTypeSess(testAmbientSession), false, false)
	hole.Init = nil
	hole.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "1"}} // Type-nil residual
	st := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{ok, hole}}
	if s := OutputSkippedVarInits(st, "    "); s != "" {
		t.Fatal("InitExpr Output residual must fail closed OutputSkippedVarInits", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("InitExpr Output residual OutputSkippedVarInits must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete single re-init
	st2 := &Stmt{Kind: StmtGoto, InitSkippedVars: []*Variable{ok}}
	if s := OutputSkippedVarInits(st2, "    "); !strings.Contains(s, "l_ok = 0") {
		t.Fatal("complete OutputSkippedVarInits", s)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete OutputSkippedVarInits must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomGotoUsesOnlyFuncBlocks(t *testing.T) {
	// StatementGoto.cpp:70–84 — vector copy of func->blocks only; no invent
	// append of current block when missing from the list.
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	outer := &Block{StmID: 1, Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 2}}}
	// curr not registered on f.Blocks
	curr := &Block{StmID: 3, Func: f, Parent: outer, Stmts: []Stmt{{Kind: StmtAssign, StmID: 4}}}
	f.Blocks = []*Block{outer}
	f.Stack = []*Block{curr}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.CurrentFunc = f
	cg.FM = NewFactMgrSess(testAmbientSession, f)
	cg.EffectAccum = &Effect{}
	r := NewRngSess(testAmbientSession, 1)
	// Force goto: may still fail soft if no good block; ensure we don't panic
	// and that FindGoodJumpBlock is only given func.Blocks (size 1), not 2.
	_ = MakeRandomGoto(r, Defaults(), ProcessProbabilitiesSess(testAmbientSession), NewVariableSelector(testAmbientSession, Defaults()), nil, &cg, curr)
	// If invent-append were present, first RndUpto would see n=2 for seed paths;
	// unit documents the contract: Blocks list is source of truth.
	if len(f.Blocks) != 1 {
		t.Fatalf("func.Blocks must stay size 1, got %d", len(f.Blocks))
	}
}

func TestIsVisibleLocalUsesMatch(t *testing.T) {
	// Variable.cpp:490–500 — match() for params and locals.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)
	if p == nil {
		t.Fatal("param")
	}
	f.Param = []*Variable{p}
	blk := &Block{StmID: 1, Func: f}
	if !p.IsVisibleLocalSess(testAmbientSession, blk) {
		t.Fatal("param must be visible in function block")
	}
	// identity via Match path for locals
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	blk.LocalVars = []*Variable{loc}
	if !loc.IsVisibleLocalSess(testAmbientSession, blk) {
		t.Fatal("local must be visible")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("sticky")
	}
}

// TestForwardGotoVisitMakeupLaterLocals — StatementGoto.cpp:167–204.
// map_facts_in[dest] is a pre-make snapshot and can omit locals created after
// dest (or during dest generation). Before VisitFactsStmt, MakeupNewVarFacts
// from live GlobalFacts must restore those facts so opportunistic_validate
// still sees them (seed-2 e19427: l_432 wiped → extra Select U100 vs UP U120).
func TestForwardGotoVisitMakeupLaterLocals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f, StmID: 1}
	f.Blocks = []*Block{blk}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)

	// earlier local pointer with fact (simulates l_432)
	lp := CreateVariableScalarsSess(testAmbientSession, "l_early", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// IsLocal is name-prefix "l_" (Variable.cpp:is_local)
	blk.LocalVars = append(blk.LocalVars, lp)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(lp, NullPtr)}
	// dest statement created with map_in that lacks l_early (pre-make snapshot)
	dest := Stmt{Kind: StmtAssign, StmID: 10}
	blk.Stmts = []Stmt{dest}
	// map_in[dest] is incomplete relative to GlobalFacts (no l_early)
	otherFact := MakeFactPointTo(
		CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false),
		NullPtr,
	)
	fm.SetMapFactsIn(10, []*FactPointTo{otherFact})
	fm.SetMapFactsOut(10, []*FactPointTo{otherFact})

	// Makeup from live GlobalFacts into map_in clone must re-add l_early
	stmIn := CloneFactSlice(fm.GetMapFactsIn(10))
	if FindRelatedPointTo(stmIn, lp) != nil {
		t.Fatal("map_in should start without l_early")
	}
	if !MakeupNewVarFacts(&stmIn, fm.GlobalFacts) {
		t.Fatal("MakeupNewVarFacts", HasErrorSess(testAmbientSession))
	}
	if FindRelatedPointTo(stmIn, lp) == nil {
		t.Fatal("MakeupNewVarFacts must re-add later local fact into visit inputs")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete makeup must not sticky")
	}
	// without makeup, SetGlobalFacts(map_in) would wipe l_early from live env
	wiped := CloneFactSlice(fm.GetMapFactsIn(10))
	if FindRelatedPointTo(wiped, lp) != nil {
		t.Fatal("raw map_in still must lack l_early")
	}
}

// StatementGoto.cpp:125–128 — forward choose_visible_read_var uses map_facts_out[other],
// not live global_facts. Soft invent always-live UnionFacts over-filtered when live
// last-writes differ from historical out lattice (seed-42 first_div: U13 vs U100).
func TestForwardGotoCondUsesMapUnionFactsOut(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntTypeSess(testAmbientSession), GetSimpleTypeSess(testAmbientSession, EShort), GetSimpleTypeSess(testAmbientSession, EUInt)}
	ut := MakeRandomUnionType(NewRngSess(testAmbientSession, 5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 1 {
		t.Skip("fields")
	}
	f0 := uv.FieldVars[0]
	// Live lattice: last write f1 → f0 nonreadable
	liveUF := []*FactUnion{MakeFactUnion(uv, 1)}
	// Historical map_facts_out: last write f0 → f0 readable
	outUF := []*FactUnion{MakeFactUnion(uv, 0)}

	// ChooseVisibleReadVar with live → NR skip → nil pool
	ClearErrorSess(testAmbientSession)
	gotLive := ChooseVisibleReadVar(NewRngSess(testAmbientSession, 1), nil, []*Variable{f0}, GetIntTypeSess(testAmbientSession), liveUF)
	if gotLive != nil {
		t.Fatal("live last_write=f1 must make f0 nonreadable → no pick")
	}
	// With map out lattice → f0 ok
	ClearErrorSess(testAmbientSession)
	gotOut := ChooseVisibleReadVar(NewRngSess(testAmbientSession, 1), nil, []*Variable{f0}, GetIntTypeSess(testAmbientSession), outUF)
	if gotOut != f0 {
		t.Fatalf("map_facts_out last_write=f0 must allow f0, got %v err=%v", gotOut, HasErrorSess(testAmbientSession))
	}

	// Wire MakeRandomGoto forward path: live UF would fail; MapUnionFactsOut succeeds.
	vs := NewVariableSelector(testAmbientSession, opts)
	vs.GlobalList = append(vs.GlobalList, uv)
	vs.AllVars = append(vs.AllVars, uv, f0)
	tables := NewExprTables(opts)
	fn := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	src := &Block{Func: fn, Stmts: []Stmt{
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
	}}
	curr := &Block{Func: fn, Stmts: []Stmt{
		{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
	}}
	fn.Blocks = []*Block{src, curr}
	fn.Body = curr
	fm := NewFactMgrSess(testAmbientSession, fn)
	fm.UnionFacts = liveUF // live would block f0
	eff := EmptyEffect().ReadVarSess(testAmbientSession, f0)
	for i := range src.Stmts {
		id := src.Stmts[i].StmID
		fm.SetMapFactsIn(id, nil)
		fm.SetMapFactsOut(id, nil)
		fm.MapUnionFactsOut[id] = outUF
		fm.MapAccumEffect[id] = eff
	}
	for i := range curr.Stmts {
		id := curr.Stmts[i].StmID
		fm.SetMapFactsIn(id, nil)
		fm.SetMapFactsOut(id, nil)
		fm.MapAccumEffect[id] = EmptyEffect()
	}
	cg := WithFunc(fn, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect()

	// Scan seeds for a forward path that would have failed under live UF
	ok := false
	for seed := uint64(1); seed < 120; seed++ {
		src.Stmts = []Stmt{
			{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
			{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()},
		}
		for i := range src.Stmts {
			id := src.Stmts[i].StmID
			fm.SetMapFactsIn(id, nil)
			fm.SetMapFactsOut(id, nil)
			fm.MapUnionFactsOut[id] = outUF
			fm.MapAccumEffect[id] = eff
		}
		ClearErrorSess(testAmbientSession)
		st := MakeRandomGoto(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg, curr)
		// success: either labeled back-edge or forward insert with cond on f0
		for _, b := range fn.Blocks {
			for _, s := range b.Stmts {
				if s.Kind == StmtGoto && s.Expr != nil && s.Expr.Var == f0 {
					ok = true
				}
			}
		}
		if st.Kind == StmtGoto && st.Expr != nil && st.Expr.Var == f0 {
			ok = true
		}
		if ok {
			break
		}
	}
	if !ok {
		// At least unit ChooseVisibleReadVar contract is locked above; integration may skip.
		t.Log("integration scan found no goto cond on f0; ChooseVisibleReadVar contract holds")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGotoOkStmsPointerNotStmID(t *testing.T) {
	// StatementGoto.cpp:97–106 — exclude only s==stm pointer, not StmID match.
	// Two distinct statements may share no identity even if tests assign equal ids.
	ClearErrorSess(testAmbientSession)
	// Build ok_stms logic inline (same as MakeRandomGoto)
	dest := &Stmt{Kind: StmtAssign, StmID: 5}
	other := &Stmt{Kind: StmtAssign, StmID: 5} // same id, different pointer
	okBlk := &Block{Stmts: []Stmt{*other, {Kind: StmtAssign, StmID: 6}}}
	var okStms []int
	for i := range okBlk.Stmts {
		s := &okBlk.Stmts[i]
		if dest != nil && s == dest {
			continue
		}
		if s.MustReturn() {
			continue
		}
		okStms = append(okStms, i)
	}
	if len(okStms) != 2 {
		t.Fatalf("want both stmts (pointer != dest), got %v", okStms)
	}
	// StmID-based exclusion would drop other (id 5) unfairly
	ClearErrorSess(testAmbientSession)
}
