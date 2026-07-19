package csmith

import (
	"strings"
	"testing"
)

func TestAddExternalEffectWithCallers(t *testing.T) {
	ClearError()
	g := CreateVariableScalars("g_1", GetIntType(), true, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	// other has global + local write
	other := EmptyEffect().WriteVar(g).WriteVar(loc)
	// without chain: only global
	e1 := EmptyEffect().AddExternalEffect(other)
	if !e1.IsWritten(g) || e1.written[loc] {
		t.Fatal("globals only", e1)
	}
	// with chain containing loc: both
	e2 := EmptyEffect().AddExternalEffectWithCallers(other, []*Block{blk})
	if !e2.IsWritten(g) || !e2.written[loc] {
		t.Fatal("chain includes local", e2)
	}
	// nil effect hole / nil chain hole fails closed sticky IncompleteEffect
	// (no invent leave-base empty-complete / partial merge)
	// IncompleteEffect IsRead/IsWritten are fail-closed true — probe completeness / maps
	base := EmptyEffect().ReadVar(g)
	hole := EmptyEffect()
	hole.written = map[*Variable]bool{nil: true, g: true}
	got := base.AddExternalEffectWithCallers(hole, []*Block{blk})
	if EffectComplete(got) || got.IsEmpty() || got.read[g] || got.written[g] {
		t.Fatal("nil effect hole must fail closed IncompleteEffect", got)
	}
	if !HasError() {
		t.Fatal("nil effect hole must SetError sticky")
	}
	ClearError()
	got2 := base.AddExternalEffectWithCallers(other, []*Block{nil, blk})
	if EffectComplete(got2) || got2.written[g] || got2.written[loc] {
		t.Fatal("nil call_chain hole must fail closed IncompleteEffect", got2)
	}
	if !HasError() {
		t.Fatal("nil call_chain hole must SetError sticky")
	}
	ClearError()
	// incomplete LocalVars on chain frame: no invent partial global merge
	holeBlk := &Block{LocalVars: []*Variable{loc, nil}}
	got3 := base.AddExternalEffectWithCallers(other, []*Block{holeBlk})
	if EffectComplete(got3) || got3.written[g] || got3.written[loc] {
		t.Fatal("incomplete stack on chain must fail closed IncompleteEffect", got3)
	}
	if !HasError() {
		t.Fatal("incomplete stack chain must SetError sticky")
	}
	ClearError()
}

func TestIsVarOOSIncompleteStackFailClosed(t *testing.T) {
	// soft invent: Param hole → IsVarVisible false → not found in Blocks → not OOS
	// fair: StackScanComplete false → OOS true sticky
	ClearError()
	f := &Function{Name: "f"}
	p := CreateVariableScalars("p_1", GetIntType(), false, false)
	f.Param = []*Variable{p, nil}
	body := &Block{Func: f, LocalVars: nil}
	if f.IsVarVisible(p, body) {
		t.Fatal("incomplete Param must not invent visible")
	}
	if !HasError() {
		t.Fatal("incomplete Param IsVarVisible must SetError sticky")
	}
	ClearError()
	if !f.IsVarOOS(p, body) {
		t.Fatal("incomplete stack must fail closed OOS, not invent not-OOS")
	}
	if !HasError() {
		t.Fatal("incomplete stack IsVarOOS must SetError sticky")
	}
	ClearError()
	// Blocks hole residual: soft invent was soft-skip hole then not-OOS / invent match later.
	// Fair: sticky OOS true fail closed (hole before match).
	f2 := &Function{Name: "f2"}
	loc := &Variable{Name: "l_1", Type: GetIntType()}
	okBlk := &Block{Func: f2, LocalVars: []*Variable{loc}}
	f2.Blocks = []*Block{nil, okBlk}
	// nil dest: not visible; scan Blocks hits nil hole before okBlk match
	if !f2.IsVarOOS(loc, nil) {
		t.Fatal("Blocks hole must fail closed OOS sticky")
	}
	if !HasError() {
		t.Fatal("Blocks hole IsVarOOS must SetError sticky")
	}
	ClearError()
}

func TestAddVisibleEffectUsesChain(t *testing.T) {
	ClearError()
	f := &Function{Name: "func_1"}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	if loc == nil {
		t.Fatal("loc")
	}
	loc.Name = "l_1"
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect())
	cg.EffectAccum = &eff
	// call chain empty; current block has loc
	other := EmptyEffect().WriteVar(loc)
	cg.AddVisibleEffect(other)
	if !cg.EffectAccum.written[loc] {
		t.Fatal("visible local via current block")
	}
}

func TestOutputVariableList(t *testing.T) {
	a := CreateVariableScalars("g_b", GetIntType(), true, false)
	b := CreateVariableScalars("g_a", GetIntType(), true, false)
	out := OutputVariableList([]*Variable{a, b}, "  ", true)
	// sorted by name: g_a then g_b
	ia := strings.Index(out, "g_a")
	ib := strings.Index(out, "g_b")
	if ia < 0 || ib < 0 || ia > ib {
		t.Fatal(out)
	}
	// incomplete OutputDef — sticky no invent indent-only blank lines
	ClearError()
	broken := &Variable{Name: "g_x", Type: GetIntType()} // no init
	if s := OutputVariableList([]*Variable{broken}, "    ", true); s != "" {
		t.Fatal("empty defs must fail closed empty list", s)
	}
	if !HasError() {
		t.Fatal("empty-def OutputVariableList must SetError sticky")
	}
	// incomplete entry fails whole list (no invent skip holes)
	ClearError()
	if s := OutputVariableList([]*Variable{a, broken}, "  ", true); s != "" {
		t.Fatal("mixed incomplete must fail closed whole list", s)
	}
	if !HasError() {
		t.Fatal("mixed incomplete OutputVariableList must SetError sticky")
	}
	ClearError()
	if s := OutputVariableList([]*Variable{a, nil}, "  ", true); s != "" {
		t.Fatal("nil hole must fail closed whole list", s)
	}
	if !HasError() {
		t.Fatal("nil hole OutputVariableList must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was scalar OutputDef path
	// fair: sticky empty whole list
	arrShell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}, Init: MakeInt(0)}
	if s := OutputVariableList([]*Variable{arrShell}, "  ", true); s != "" {
		t.Fatal("IsArray without AsArray must fail closed whole list", s)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputVariableList must SetError sticky")
	}
	ClearError()
}

func TestOutputGlobalVariables(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	out := OutputGlobalVariables([]*Variable{v})
	if !strings.Contains(out, "GLOBAL VARIABLES") || !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
	decl := OutputGlobalVariablesDecls([]*Variable{v}, "extern ")
	if !strings.Contains(decl, "extern ") {
		t.Fatal(decl)
	}
	// no invent section-only header when all defs empty
	if s := OutputGlobalVariables([]*Variable{{Name: "g_x", Type: GetIntType()}}); s != "" {
		t.Fatal("empty globals must fail closed section", s)
	}
	if s := OutputGlobalVariablesDecls(nil, "extern "); s != "" {
		t.Fatal("nil globals must fail closed section", s)
	}
}

func TestMakeRandomIfFunc1UncertainPath(t *testing.T) {
	// smoke: func_1 with FM does not panic on if generation
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f)), GetIntType(), nil, NewRng(1))
	fm := NewFactMgr(f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	// seed may fail; retry
	var st *Stmt
	for seed := uint64(1); seed < 40; seed++ {
		ClearError()
		cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
		cg2.EffectAccum = &eff
		cg2.Types = vs.Types
		st = MakeRandomIf(NewRng(seed), opts, probs, vs, NewExprTables(opts),
			NewStatementThresholdTable(opts), &cg2)
		if st != nil && st.Kind == StmtIfElse {
			break
		}
	}
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
}
