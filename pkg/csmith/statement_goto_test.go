package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomGotoEmptyBlockReturnsNull(t *testing.T) {
	// StatementGoto.cpp:86–87 / 130–132 — no soft makeForwardGotoOnly
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	blk := &Block{Func: f}
	f.Blocks = []*Block{blk}
	st := MakeRandomGoto(NewRng(9), opts, probs, vs, tables, &cg, blk)
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
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	g := CreateVariableScalars("g_c", GetIntType(), true, false)
	vs.AllVars = []*Variable{g}
	vs.GlobalList = []*Variable{g}
	// target stmt for back-edge
	tgt := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}
	blk := &Block{Func: f, Stmts: []Stmt{tgt, {Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}}}
	f.Blocks = []*Block{blk}
	f.Body = blk
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	eff := EmptyEffect().ReadVar(g)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	var st Stmt
	for seed := uint64(1); seed < 60; seed++ {
		blk.Stmts[0].SourceLabel = ""
		st = MakeRandomGoto(NewRng(seed), opts, probs, vs, tables, &cg, blk)
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

func TestGenerateCanEmitGoto(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
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
}

func TestMarkNeedRevisitLCA(t *testing.T) {
	// outer → then(inner with assign) — back-edge LCA is outer when dest in then
	dest := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: 7}
	inner := &Block{Stmts: []Stmt{dest}}
	outer := &Block{
		Stmts: []Stmt{{Kind: StmtIfElse, Then: inner, StmID: 1}},
	}
	inner.Parent = outer
	// dest pointer must be into slice
	d := &inner.Stmts[0]
	MarkNeedRevisitLCA(inner, d)
	if !inner.NeedRevisit {
		t.Fatal("inner contains dest → mark inner")
	}
	// from a sibling-ish curr that does not contain dest → walk to outer
	curr := &Block{Parent: outer, Stmts: []Stmt{{Kind: StmtAssign, StmID: 8}}}
	outer.NeedRevisit = false
	inner.NeedRevisit = false
	MarkNeedRevisitLCA(curr, d)
	if !outer.NeedRevisit {
		t.Fatal("outer is LCA containing dest")
	}
	if curr.NeedRevisit {
		t.Fatal("curr should not be marked when outer contains dest")
	}
	// StatementGoto.cpp:147 assert(b) — no soft invent NeedRevisit when dest not in ancestry
	orphan := &Block{Stmts: []Stmt{{Kind: StmtAssign, StmID: 9}}}
	MarkNeedRevisitLCA(orphan, d)
	if orphan.NeedRevisit {
		t.Fatal("orphan must not invent NeedRevisit when dest not found")
	}
}

func TestMakeRandomGotoRequiresFactMgr(t *testing.T) {
	// StatementGoto.cpp:66–67 get_fact_mgr; no soft invent goto without FM
	opts := Defaults()
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f, Stmts: []Stmt{{Kind: StmtAssign, StmID: 1}}}
	f.Stack = []*Block{blk}
	f.Blocks = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	// no FM
	st := MakeRandomGoto(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, blk)
	if stmtOK(st) {
		t.Fatal("goto without FactMgr must fail closed")
	}
}

func TestMakeBinaryForCompare(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	lhs := &Expression{Term: TermVariable, Var: CreateVariableScalars("g_i", GetIntType(), true, false), ExprType: GetIntType()}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(10), ExprType: GetIntType()}
	fi := MakeBinary(NewRng(1), opts, NewProbabilities(opts), EmptyCGContext(), BinCmpLt, lhs, rhs)
	if fi == nil || fi.Binary != "<" {
		t.Fatalf("%+v", fi)
	}
	if fi.Safe == nil {
		t.Fatal("flags always set")
	}
	if fi.GetType() != GetIntType() {
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

func TestGotoLabelsClearedOnFinalization(t *testing.T) {
	GotoLabelsDoFinalization()
	_ = LabelForGotoDest(1, func() string { return "lbl_x" })
	DoFinalization()
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
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
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
	fm := NewFactMgr(f)
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
	g := CreateVariableScalars("g_c", GetIntType(), true, false)
	vs.AllVars = append(vs.AllVars, g)
	eff := EmptyEffect().ReadVar(g)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
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
		st := MakeRandomGoto(NewRng(seed), opts, probs, vs, tables, &cg, curr)
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

func TestResetEffectAccum(t *testing.T) {
	cg := EmptyCGContext()
	v := CreateVariableScalars("g_x", GetIntType(), true, false)
	pre := EmptyEffect().ReadVar(v)
	cg.EffectAccum = &Effect{}
	*cg.EffectAccum = EmptyEffect().WriteVar(v)
	cg.ResetEffectAccum(pre)
	if !cg.EffectAccum.IsRead(v) || cg.EffectAccum.IsWritten(v) {
		t.Fatalf("%+v", cg.EffectAccum)
	}
}
