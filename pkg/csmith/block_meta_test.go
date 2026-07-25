package csmith

import (
	"strings"
	"testing"
)

func TestFromTailToHead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	b := &Block{Looping: true, Stmts: []Stmt{
		{Kind: StmtAssign},
	}}
	if !b.FromTailToHead() {
		t.Fatal("fall through")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete fall-through FromTailToHead must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	b.Stmts = []Stmt{{Kind: StmtReturn}}
	if b.FromTailToHead() {
		t.Fatal("return must_jump")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete must_jump FromTailToHead must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	b.Looping = false
	if b.FromTailToHead() {
		t.Fatal("not looping")
	}
	ClearErrorSess(testAmbientSession)
	// MustJump residual soft invent was soft-continue fall-through invent true.
	// Fair: sticky false. last if incomplete residual MustJump.
	b2 := &Block{Looping: true, Stmts: []Stmt{
		{Kind: StmtIfElse, Then: nil, Else: &Block{}},
	}}
	if b2.FromTailToHead() {
		t.Fatal("MustJump residual FromTailToHead must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MustJump residual FromTailToHead must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestGetLastStmStopsAtReturn(t *testing.T) {
	b := &Block{Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtReturn, StmID: 2},
		{Kind: StmtAssign, StmID: 3},
	}}
	if b.GetLastStm() == nil || b.GetLastStm().StmID != 2 {
		t.Fatal(b.GetLastStm())
	}
}

func TestSetAccumulatedEffect(t *testing.T) {
	fm := NewFactMgr(nil)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	fm.SetMapStmEffect(1, EmptyEffect().WriteVar(v))
	fm.SetMapStmEffect(2, EmptyEffect().ReadVar(v))
	b := &Block{
		StmID: 10,
		Stmts: []Stmt{{StmID: 1}, {StmID: 2}},
	}
	eff := b.SetAccumulatedEffect(fm)
	if !eff.IsWritten(v) || !eff.IsRead(v) {
		t.Fatal("union")
	}
	if !fm.GetMapStmEffect(10).IsWritten(v) {
		t.Fatal("block effect")
	}
	// StmID 0 incomplete sticky — IncompleteEffect (not EmptyEffect invent pure/empty success)
	ClearErrorSess(testAmbientSession)
	b2 := &Block{StmID: 11, Stmts: []Stmt{{StmID: 1}, {StmID: IncompleteStmID}}}
	fm.SetMapStmEffect(11, EmptyEffect().WriteVar(v))
	eff2 := b2.SetAccumulatedEffect(fm)
	if EffectComplete(eff2) || eff2.IsEmpty() || eff2.IsPure() {
		t.Fatal("StmID 0 must fail closed IncompleteEffect, not invent empty/pure", eff2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 SetAccumulatedEffect must SetError sticky")
	}
	// IncompleteEffect: IsWritten is fail-closed true — probe completeness / map shell only
	if EffectComplete(fm.GetMapStmEffect(11)) || fm.GetMapStmEffect(11).written[v] {
		t.Fatal("block map must IncompleteEffect, not invent partial write map entry")
	}
	ClearErrorSess(testAmbientSession)
	// nil block/fm must IncompleteEffect sticky (not invent EmptyEffect pure)
	if EffectComplete(((*Block)(nil)).SetAccumulatedEffect(fm)) {
		t.Fatal("nil block must IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil block SetAccumulatedEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomParentBlock(t *testing.T) {
	outer := &Block{}
	inner := &Block{Parent: outer}
	seen := map[*Block]bool{}
	r := NewRng(1)
	for i := 0; i < 40; i++ {
		seen[inner.RandomParentBlock(r, true)] = true
	}
	// nil (global), outer, inner — three slots (Block.cpp:297–304)
	if len(seen) < 2 {
		t.Fatal(seen)
	}
	// with global_variables: domain size is 1 (nil) + chain length
	rN := NewRng(2)
	d0 := rN.RandDepth()
	_ = inner.RandomParentBlock(rN, true)
	// one U draw with n == 3 for [nil, inner, outer]
	// (cannot read n from depth alone; polarity: without global never returns nil)
	// without global
	seen2 := map[*Block]bool{}
	for i := 0; i < 20; i++ {
		p := inner.RandomParentBlock(NewRng(uint64(i+2)), false)
		if p == nil {
			t.Fatal("nil without global")
		}
		seen2[p] = true
	}
	// with global must be able to return nil
	foundNil := false
	for i := 0; i < 80; i++ {
		if inner.RandomParentBlock(NewRng(uint64(i+1)), true) == nil {
			foundNil = true
			break
		}
	}
	if !foundNil {
		t.Fatal("allowGlobal must include nil global site")
	}
	_ = d0
	// Block.cpp:306 — nil RNG sticky ERROR_GUARD
	ClearErrorSess(testAmbientSession)
	if inner.RandomParentBlock(nil, false) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG RandomParentBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRandomParentBlockDomainWithGlobals(t *testing.T) {
	// Block.cpp:295–308 + StatementArrayOp.cpp:141 — GlobalVariables → n = 1+depth
	ClearErrorSess(testAmbientSession)
	outer := &Block{}
	inner := &Block{Parent: outer}
	// Probe domain size by counting distinct results over many seeds with allowGlobal
	// and comparing draw: first event after NewRng is U with n=3 when chain has 2 blocks.
	// Use raw Rng: flipcoin not used; only RndUpto(3).
	r := NewRng(11)
	// Manually mirror domain: [nil, inner, outer]
	// After one RandomParentBlock(true), depth must advance by 1
	d0 := r.RandDepth()
	_ = inner.RandomParentBlock(r, true)
	if r.RandDepth() != d0+1 {
		t.Fatalf("one upto draw expected: %d → %d", d0, r.RandDepth())
	}
	// without global domain is 2
	r2 := NewRng(11)
	d1 := r2.RandDepth()
	_ = inner.RandomParentBlock(r2, false)
	if r2.RandDepth() != d1+1 {
		t.Fatal("one upto")
	}
	// Same seed, same raw, different n → different v is possible; both consume one draw.
	ClearErrorSess(testAmbientSession)
}

func TestLabelAttrEmit(t *testing.T) {
	ClearAttrGeneratorsSess(testAmbientSession)
	currentSession().LabelAttrGenerator = &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "hot", Prob: 100},
	}}
	b := &Block{
		EmitLabelAttrs: true,
		LabelAttrRng:   NewRng(1),
		Stmts: []Stmt{{
			Kind: StmtAssign, SourceLabel: "lbl_1",
			LhsVar:   CreateVariableScalars("g_1", GetIntType(), false, false),
			AssignOp: AssignSimple,
			Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)},
		}},
	}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_1:") || !strings.Contains(out, "hot") {
		t.Fatal(out)
	}
	ClearAttrGeneratorsSess(testAmbientSession)
}

func TestLoopSelfBackEdgeOnPostCreation(t *testing.T) {
	ReinstallTestProcessSingletons()
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// make a small looping block
	b := MakeRandomBlock(NewRng(3), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, true)
	if b == nil {
		t.Fatal("nil")
	}
	// if fall-through possible, self back edge exists
	if b.FromTailToHead() {
		found := false
		for _, e := range fm.CFGEdges {
			if e != nil && e.BackLink && e.DestBlock == b {
				found = true
			}
		}
		if !found {
			t.Fatal("missing self back edge", fm.CFGEdges)
		}
	}
}

func TestMustBreakOrReturn(t *testing.T) {
	ClearErrorSess(testAmbientSession) // sticky GenError from earlier suite tests must not mask must_return
	// Block.cpp:342–357 — last must_return (break alone is not enough)
	b := &Block{Stmts: []Stmt{{
		Kind: StmtBreak,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}}
	if b.MustBreakOrReturn() {
		t.Fatal("break is not must_return")
	}
	b.Stmts = []Stmt{{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
	}}
	if !b.MustBreakOrReturn() {
		t.Fatal("return must_break_or_return")
	}
}

func TestMustReturnBreakStmsAndBackEdge(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Block.cpp:313–331 — break_stms nonempty → not must_return
	ret := Stmt{Kind: StmtReturn, StmID: 2, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	b := &Block{StmID: 1, Stmts: []Stmt{ret}, BreakStmIDs: []int{9}}
	if b.MustReturn() {
		t.Fatal("break_stms blocks must_return")
	}
	b.BreakStmIDs = nil
	if !b.MustReturn() {
		t.Fatal("return last")
	}
	// continue edge into block escapes — CreateCFGEdge(src, block) stores DestStmID=block.StmID
	// (FactMgr.cpp:597–598 e->dest == Block*; Statement.cpp:453–467 find_edges_in).
	fm := NewFactMgr(nil)
	b.EmitFM = fm
	fm.CFGEdges = []*CFGEdge{{SrcID: 99, DestBlock: b, DestStmID: b.StmID, BackLink: true}}
	if b.MustReturn() {
		t.Fatal("back edge escapes")
	}
	// StatementGoto.cpp:139 — create_cfg_edge(sg, other_stm): DestStmID is the label
	// statement, not the parent block. Parent DestBlock bookkeeping must not escape
	// must_return (seed-79 double return / append_return).
	ClearErrorSess(testAmbientSession)
	fm.CFGEdges = []*CFGEdge{{SrcID: 5, DestBlock: b, DestStmID: 2 /* labeled stmt */, BackLink: true}}
	if !b.MustReturn() {
		t.Fatal("goto-to-label edge must not escape block must_return")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete goto-to-label must_return must not sticky")
	}
	// Block StmID 0 + FM sticky fail closed as escape (no invent "no back edge")
	ClearErrorSess(testAmbientSession)
	b0 := &Block{StmID: IncompleteStmID, Stmts: []Stmt{ret}, EmitFM: fm}
	if b0.MustReturn() {
		t.Fatal("block StmID 0 must fail closed not must_return")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("block StmID 0 MustReturn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// MustJump also requires empty break_stms
	b2 := &Block{Stmts: []Stmt{{
		Kind: StmtBreak, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}}, BreakStmIDs: []int{1}}
	if b2.MustJump() {
		t.Fatal("break_stms nonempty")
	}
	b2.BreakStmIDs = nil
	if !b2.MustJump() {
		t.Fatal("true break must_jump")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete MustJump must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Block always live; sticky no invent not-must-return / not-must-jump soft-skip
	if (*Block)(nil).MustReturn() {
		t.Fatal("nil MustReturn must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MustReturn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Block)(nil).MustJump() {
		t.Fatal("nil MustJump must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MustJump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Block)(nil).MustReturnWithFM(NewFactMgr(nil)) {
		t.Fatal("nil MustReturnWithFM must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil MustReturnWithFM must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBlockOutputBlockIDComment(t *testing.T) {
	// Block.cpp:250–253 — "{ " + /* block id: N */
	ClearErrorSess(testAmbientSession)
	b := &Block{StmID: 42, Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 1,
		LhsVar:   CreateVariableScalars("g_1", GetIntType(), false, false),
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)},
		AssignOp: AssignSimple,
	}}}
	out := b.Output(0)
	if !strings.Contains(out, "/* block id: 42 */") {
		t.Fatal(out)
	}
	// concise skips comment body of OutputCommentLine when we gate EmitConcise
	b.EmitConcise = true
	out2 := b.Output(0)
	if strings.Contains(out2, "block id:") {
		t.Fatal("concise should skip block id", out2)
	}
}

// StatementFor.cpp:422–424 / StatementArrayOp.cpp:230 — body.Output(indent)
// same as header (not indent+1). Unfair indent+1: "for\n        {" vs "for\n    {".
func TestForBodyBlockSameIndentAsHeader(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	body := &Block{StmID: 7, Stmts: []Stmt{{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermVariable, Var: iv, ExprType: GetIntType()},
	}}}
	// ArrayOp header only needs IV + InitN/LimitN/IncrN (numeric), same indent rule
	parent := &Block{StmID: 1, Stmts: []Stmt{{
		Kind: StmtArrayOp, StmID: 2,
		Loop: &LoopControl{IV: iv, InitN: 0, LimitN: 3, IncrN: 1},
		Then: body,
	}}}
	out := parent.Output(0)
	if out == "" || HasErrorSess(testAmbientSession) {
		t.Fatalf("Output empty/err: %q sticky=%v", out, HasErrorSess(testAmbientSession))
	}
	lines := strings.Split(out, "\n")
	var forLine, braceLine string
	for i, l := range lines {
		if strings.Contains(l, "for (") {
			forLine = l
			if i+1 < len(lines) {
				braceLine = lines[i+1]
			}
			break
		}
	}
	if forLine == "" || braceLine == "" {
		t.Fatalf("missing for/brace in:\n%s", out)
	}
	lead := func(s string) int {
		n := 0
		for _, c := range s {
			if c == ' ' {
				n++
			} else {
				break
			}
		}
		return n
	}
	if lead(forLine) != lead(braceLine) {
		t.Fatalf("for indent %d vs body brace %d\nfor:%q\nbrace:%q\nout:\n%s",
			lead(forLine), lead(braceLine), forLine, braceLine, out)
	}
}

// Statement.cpp:239,370–371 — sid starts 0; assign then increment.
func TestAllocStmIDMatchesCppSid(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	currentSession().NextStmID = 0
	if AllocStmID() != 0 {
		t.Fatal("first stm_id must be 0")
	}
	if AllocStmID() != 1 || AllocStmID() != 2 {
		t.Fatal("monotonic sid")
	}
	if StmIDUnset(0) {
		t.Fatal("valid id 0 must not be unset")
	}
	if !StmIDUnset(IncompleteStmID) {
		t.Fatal("IncompleteStmID must be unset")
	}
}

// Seed-2: func_1 body is block id 0; late id 577 (matches instrumented upstream).
func TestSeed2BlockIDsMatchUpstream(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	currentSession().NextStmID = 0
	opts := Defaults()
	opts.Seed = 2
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "{ /* block id: 0 */") {
		t.Fatal("missing block id 0")
	}
	if !strings.Contains(out, "block id: 577") {
		t.Fatal("missing block id 577 (sid desync vs C++)")
	}
	// definition (not forward decl) opens with block id 0
	idx := strings.Index(out, "static uint32_t  func_1(void)\n{ /* block id: 0 */")
	if idx < 0 {
		t.Fatal("func_1 definition must open with block id 0")
	}
}
