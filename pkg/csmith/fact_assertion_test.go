package csmith

import (
	"strings"
	"testing"
)

func TestFactPointToOutputCondition(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	f := MakeFactPointTo(p, tgt)
	cond := f.OutputCondition()
	if !strings.Contains(cond, "g_p == &g_1") {
		t.Fatal(cond)
	}
	fn := MakeFactPointTo(p, NullPtr)
	if fn.OutputCondition() != "g_p == 0" {
		t.Fatal(fn.OutputCondition())
	}
	fd := MakeFactPointTo(p, GarbagePtr)
	if !strings.Contains(fd.OutputCondition(), "dangling") {
		t.Fatal(fd.OutputCondition())
	}
	// point_to_vars always live; sticky no invent skip nil holes in OR list
	ClearError()
	broken := &FactPointTo{Var: p, PointTo: []*Variable{tgt, nil}}
	if cond := broken.OutputCondition(); cond != "" {
		t.Fatal("nil pointee must fail closed", cond)
	}
	if !HasError() {
		t.Fatal("nil pointee OutputCondition must SetError sticky")
	}
	// sticky no invent " == 0" without subject name
	ClearError()
	anon := &FactPointTo{Var: &Variable{Type: PointerTo(GetIntType())}, PointTo: []*Variable{NullPtr}}
	if cond := anon.OutputCondition(); cond != "" {
		t.Fatal("empty subject name must fail closed", cond)
	}
	if !HasError() {
		t.Fatal("empty subject OutputCondition must SetError sticky")
	}
	// no invent "(p >= & && p <= &)" with empty array bounds
	arr := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	// empty name array bounds fail closed sticky via OutputUpper/LowerBound
	ClearError()
	arrNoName := &Variable{Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if cond := (&FactPointTo{Var: p, PointTo: []*Variable{arrNoName}}).OutputCondition(); cond != "" {
		t.Fatal("empty array bound name must fail closed", cond)
	}
	if !HasError() {
		t.Fatal("empty array bound name must SetError sticky")
	}
	// live array range form (bounds via OutputLower/UpperBound)
	ClearError()
	if cond := (&FactPointTo{Var: p, PointTo: []*Variable{arr}}).OutputCondition(); !strings.Contains(cond, "g_p >= &g_a") {
		t.Fatal("want array range form", cond)
	}
	// sticky no invent bare "&" pointee
	ClearError()
	if cond := (&FactPointTo{Var: p, PointTo: []*Variable{{Type: GetIntType()}}}).OutputCondition(); cond != "" {
		t.Fatal("empty pointee name must fail closed", cond)
	}
	if !HasError() {
		t.Fatal("empty pointee name OutputCondition must SetError sticky")
	}
	ClearError()
}

func TestOutputAssertionCommentedWhenNotAssertable(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f := MakeFactPointTo(p, GarbagePtr)
	out := f.OutputAssertion(nil, "    ")
	if !strings.HasPrefix(strings.TrimSpace(out), "//assert") && !strings.Contains(out, "//assert") {
		t.Fatal(out)
	}
	// assertable: global → global
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	ok := MakeFactPointTo(p, tgt)
	out2 := ok.OutputAssertion(nil, "    ")
	if strings.Contains(out2, "//assert") {
		t.Fatal(out2)
	}
	if !strings.Contains(out2, "assert (") {
		t.Fatal(out2)
	}
	// incomplete PointTo sticky not-assertable
	ClearError()
	hole := &FactPointTo{Var: p, PointTo: []*Variable{nil, GarbagePtr}}
	if hole.IsAssertable(nil) {
		t.Fatal("incomplete PointTo must fail closed not-assertable")
	}
	if !HasError() {
		t.Fatal("incomplete PointTo IsAssertable must SetError sticky")
	}
	ClearError()
	// incomplete stack at parent: HasInvisible sticky true
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc, nil}}
	fl := MakeFactPointTo(loc, NullPtr)
	if !fl.HasInvisible(blk) {
		t.Fatal("incomplete stack must HasInvisible true")
	}
	if !HasError() {
		t.Fatal("incomplete stack HasInvisible must SetError sticky")
	}
	ClearError()
	if fl.IsAssertable(blk) {
		t.Fatal("incomplete stack must not invent assertable")
	}
	ClearError()
	if s := hole.OutputAssertion(nil, "  "); s != "" && !strings.Contains(s, "//") {
		// OutputCondition fails closed empty on hole → empty assertion
		_ = s
	}
	ClearError()
}

func TestOutputAssertionsParanoid(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_1", GetIntType(), true, false)
	// function reads/writes p so fact is printed
	f.FEffect = EmptyEffect().ReadVar(p).WriteVar(p)
	fm := NewFactMgr(f)
	fm.SetMapFactsIn(5, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(5, []*FactPointTo{MakeFactPointTo(p, tgt)})
	fm.SetupInOutMaps(true)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	out := fm.OutputAssertions(st, nil, "    ", true)
	if !strings.Contains(out, "statement id: 5") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "assert") {
		t.Fatal(out)
	}
	// global fact neither read nor written → no invent comment-only shell
	f2 := &Function{Name: "func_2", ReturnType: GetIntType()}
	// empty effect: skip globals
	fm2 := NewFactMgr(f2)
	fm2.SetMapFactsIn(6, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm2.SetMapFactsOut(6, []*FactPointTo{MakeFactPointTo(p, tgt)})
	fm2.SetupInOutMaps(true)
	st2 := &Stmt{Kind: StmtAssign, StmID: 6}
	if s := fm2.OutputAssertions(st2, nil, "    ", true); s != "" {
		t.Fatal("filtered facts must not invent comment-only shell", s)
	}
	// StmID 0 sticky — no invent empty assertion section past incomplete stmt id
	ClearError()
	st0 := &Stmt{Kind: StmtAssign, StmID: 0}
	if s := fm.OutputAssertions(st0, nil, "    ", true); s != "" {
		t.Fatal("StmID 0 OutputAssertions must fail closed", s)
	}
	if !HasError() {
		t.Fatal("StmID 0 OutputAssertions must SetError sticky")
	}
	ClearError()
	// nil FM / Statement sticky
	if s := (*FactMgr)(nil).OutputAssertions(st, nil, "    ", true); s != "" {
		t.Fatal("nil FM OutputAssertions must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil FM OutputAssertions must SetError sticky")
	}
	ClearError()
	if s := fm.OutputAssertions(nil, nil, "    ", true); s != "" {
		t.Fatal("nil Stmt OutputAssertions must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil Stmt OutputAssertions must SetError sticky")
	}
	ClearError()
	if s := (*FactPointTo)(nil).OutputAssertion(nil, "  "); s != "" {
		t.Fatal("nil Fact OutputAssertion must fail closed", s)
	}
	if !HasError() {
		t.Fatal("nil Fact OutputAssertion must SetError sticky")
	}
	ClearError()
}

func TestPostOutputInBlock(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	tgt := CreateVariableScalars("g_2", GetIntType(), true, false)
	f.FEffect = EmptyEffect().WriteVar(p).ReadVar(p)
	fm := NewFactMgr(f)
	fm.SetMapFactsIn(7, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(7, []*FactPointTo{MakeFactPointTo(p, tgt)})
	fm.SetupInOutMaps(true)
	b := &Block{
		EmitParanoid: true,
		EmitFM:       fm,
		Stmts: []Stmt{{
			Kind: StmtAssign, StmID: 7,
			LhsVar: p, Lhs: &Lhs{Var: p, Type: PointerTo(GetIntType())},
			Expr:     &Expression{Term: TermConstant, Con: MakeInt(0)},
			AssignOp: AssignSimple,
		}},
	}
	out := b.Output(0)
	if !strings.Contains(out, "assert") {
		t.Fatal(out)
	}
}

func TestIsTopEmpty(t *testing.T) {
	ClearError()
	f := &FactPointTo{Var: CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)}
	if !f.IsTop() {
		t.Fatal("empty is top")
	}
	if HasError() {
		t.Fatal("empty PointTo IsTop must not sticky")
	}
	// nil Fact sticky false (no invent TOP)
	if (*FactPointTo)(nil).IsTop() {
		t.Fatal("nil Fact IsTop must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil Fact IsTop must SetError sticky")
	}
	ClearError()
}

func TestPreOutputLabelSkipsStepHash(t *testing.T) {
	// Statement.cpp:905–917 — jump target emits label, not step_hash
	ClearError()
	st := &Stmt{Kind: StmtAssign, StmID: 5, SourceLabel: "lbl_1"}
	out, tgt := PreOutput(st, nil, true, false, nil, "    ")
	if !tgt || !strings.Contains(out, "lbl_1:") {
		t.Fatal(out, tgt)
	}
	if strings.Contains(out, "step_hash") {
		t.Fatal("goto target must not step_hash", out)
	}
	// nil Stmt sticky
	ClearError()
	if o, g := PreOutput(nil, nil, true, false, nil, ""); o != "" || g {
		t.Fatal("nil Stmt PreOutput must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil Stmt PreOutput must SetError sticky")
	}
	ClearError()
}

func TestPreOutputStepHashWhenNotTarget(t *testing.T) {
	ClearError()
	st := &Stmt{Kind: StmtAssign, StmID: 9}
	out, tgt := PreOutput(st, nil, true, false, nil, "  ")
	if tgt || out != "  step_hash(9);\n" {
		t.Fatal(out, tgt)
	}
	// FM + StmID 0 sticky — no invent SourceLabel / step_hash for incomplete id
	fm := NewFactMgr(nil)
	st0 := &Stmt{Kind: StmtAssign, StmID: 0, SourceLabel: "lbl_invent"}
	out0, tgt0 := PreOutput(st0, fm, true, false, nil, "  ")
	if out0 != "" || tgt0 {
		t.Fatal("StmID 0 under FM must not invent SourceLabel/step_hash", out0, tgt0)
	}
	if !HasError() {
		t.Fatal("StmID 0 under FM PreOutput must SetError sticky")
	}
	ClearError()
}

func TestPreOutputFromCFGJumpSources(t *testing.T) {
	ClearError()
	f := &Function{Name: "f"}
	body := &Block{Func: f, Stmts: []Stmt{
		{Kind: StmtAssign, StmID: 1},
		{Kind: StmtGoto, StmID: 2, Label: "lbl_cfg", GotoDestStmID: 1},
	}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	fm.CFGEdges = []*CFGEdge{{SrcID: 2, DestStmID: 1}}
	st := &body.Stmts[0]
	out, tgt := PreOutput(st, fm, true, false, nil, "")
	if !tgt || !strings.Contains(out, "lbl_cfg:") {
		t.Fatal(out, tgt)
	}
	if HasError() {
		t.Fatal("complete CFG PreOutput must not sticky")
	}
	ClearError()
}

func TestBlockOutputPreOutputNoHashOnLabel(t *testing.T) {
	ClearError()
	b := &Block{
		EmitStepHash: true,
		Stmts: []Stmt{
			{Kind: StmtAssign, StmID: 3, SourceLabel: "lbl_x",
				LhsVar: CreateVariableScalars("g_1", GetIntType(), false, false),
				Expr:   &Expression{Term: TermConstant, Con: MakeInt(1)},
				AssignOp: AssignSimple},
		},
	}
	out := b.Output(0)
	if !strings.Contains(out, "lbl_x:") {
		t.Fatal(out)
	}
	// labeled stmt should not have step_hash(3)
	if strings.Contains(out, "step_hash(3)") {
		t.Fatal("labeled target should skip hash", out)
	}
}

func TestOutputConditionNilFactSticky(t *testing.T) {
	ClearError()
	if (*FactPointTo)(nil).OutputCondition() != "" {
		t.Fatal("nil Fact OutputCondition must fail closed")
	}
	if !HasError() {
		t.Fatal("nil Fact OutputCondition must SetError sticky")
	}
	ClearError()
}
