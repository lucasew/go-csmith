package csmith

import (
	"strings"
	"testing"
)

func TestAddExternalEffectWithCallers(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	loc.Name = "l_1"
	blk := &Block{LocalVars: []*Variable{loc}}
	// other has global + local write
	other := EmptyEffect().WriteVarSess(testAmbientSession, g).WriteVarSess(testAmbientSession, loc)
	// without chain: only global
	e1 := EmptyEffect().AddExternalEffectSess(testAmbientSession, other)
	if !e1.IsWrittenSess(testAmbientSession, g) || e1.written[loc] {
		t.Fatal("globals only", e1)
	}
	// with chain containing loc: both
	e2 := EmptyEffect().AddExternalEffectWithCallersSess(testAmbientSession, other, []*Block{blk})
	if !e2.IsWrittenSess(testAmbientSession, g) || !e2.written[loc] {
		t.Fatal("chain includes local", e2)
	}
	// nil effect hole / nil chain hole fails closed sticky IncompleteEffect
	// (no invent leave-base empty-complete / partial merge)
	// IncompleteEffect IsRead/IsWritten are fail-closed true — probe completeness / maps
	base := EmptyEffect().ReadVarSess(testAmbientSession, g)
	hole := EmptyEffect()
	hole.written = map[*Variable]bool{nil: true, g: true}
	got := base.AddExternalEffectWithCallersSess(testAmbientSession, hole, []*Block{blk})
	if EffectComplete(got) || got.IsEmptySess(testAmbientSession) || got.read[g] || got.written[g] {
		t.Fatal("nil effect hole must fail closed IncompleteEffect", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil effect hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	got2 := base.AddExternalEffectWithCallersSess(testAmbientSession, other, []*Block{nil, blk})
	if EffectComplete(got2) || got2.written[g] || got2.written[loc] {
		t.Fatal("nil call_chain hole must fail closed IncompleteEffect", got2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil call_chain hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete LocalVars on chain frame: no invent partial global merge
	holeBlk := &Block{LocalVars: []*Variable{loc, nil}}
	got3 := base.AddExternalEffectWithCallersSess(testAmbientSession, other, []*Block{holeBlk})
	if EffectComplete(got3) || got3.written[g] || got3.written[loc] {
		t.Fatal("incomplete stack on chain must fail closed IncompleteEffect", got3)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete stack chain must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsVarOOSIncompleteStackFailClosed(t *testing.T) {
	// soft invent: Param hole → IsVarVisible false → not found in Blocks → not OOS
	// fair: StackScanComplete false → OOS true sticky
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	p := CreateVariableScalarsSess(testAmbientSession, "p_1", GetIntTypeSess(testAmbientSession), false, false)
	f.Param = []*Variable{p, nil}
	body := &Block{Func: f, LocalVars: nil}
	if f.IsVarVisibleSess(testAmbientSession, p, body) {
		t.Fatal("incomplete Param must not invent visible")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Param IsVarVisible must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !f.IsVarOOSSess(testAmbientSession, p, body) {
		t.Fatal("incomplete stack must fail closed OOS, not invent not-OOS")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete stack IsVarOOS must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Blocks hole residual: soft invent was soft-skip hole then not-OOS / invent match later.
	// Fair: sticky OOS true fail closed (hole before match).
	f2 := &Function{Name: "f2"}
	loc := &Variable{Name: "l_1", Type: GetIntTypeSess(testAmbientSession)}
	okBlk := &Block{Func: f2, LocalVars: []*Variable{loc}}
	f2.Blocks = []*Block{nil, okBlk}
	// nil dest: not visible; scan Blocks hits nil hole before okBlk match
	if !f2.IsVarOOSSess(testAmbientSession, loc, nil) {
		t.Fatal("Blocks hole must fail closed OOS sticky")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Blocks hole IsVarOOS must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAddVisibleEffectUsesChain(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1"}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	if loc == nil {
		t.Fatal("loc")
	}
	loc.Name = "l_1"
	blk := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Stack = []*Block{blk}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	// call chain empty; current block has loc
	other := EmptyEffect().WriteVarSess(testAmbientSession, loc)
	cg.AddVisibleEffect(other)
	if !cg.EffectAccum.written[loc] {
		t.Fatal("visible local via current block")
	}
}

func TestOutputVariableList(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), true, false)
	out := OutputVariableListSess(testAmbientSession, []*Variable{a, b}, "  ", true, sessOpts(testAmbientSession))
	// Variable.cpp:858–860 — vector order (no invent name-sort)
	ia := strings.Index(out, "g_a")
	ib := strings.Index(out, "g_b")
	if ia < 0 || ib < 0 || ib > ia {
		t.Fatal("want vector order g_b then g_a", out)
	}
	// incomplete OutputDef — sticky no invent indent-only blank lines
	ClearErrorSess(testAmbientSession)
	broken := &Variable{Name: "g_x", Type: GetIntTypeSess(testAmbientSession)} // no init
	if s := OutputVariableListSess(testAmbientSession, []*Variable{broken}, "    ", true, sessOpts(testAmbientSession)); s != "" {
		t.Fatal("empty defs must fail closed empty list", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty-def OutputVariableList must SetError sticky")
	}
	// incomplete entry fails whole list (no invent skip holes)
	ClearErrorSess(testAmbientSession)
	if s := OutputVariableListSess(testAmbientSession, []*Variable{a, broken}, "  ", true, sessOpts(testAmbientSession)); s != "" {
		t.Fatal("mixed incomplete must fail closed whole list", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("mixed incomplete OutputVariableList must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := OutputVariableListSess(testAmbientSession, []*Variable{a, nil}, "  ", true, sessOpts(testAmbientSession)); s != "" {
		t.Fatal("nil hole must fail closed whole list", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole OutputVariableList must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was scalar OutputDef path
	// fair: sticky empty whole list
	arrShell := &Variable{Name: "g_arr", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}, Init: MakeIntSess(testAmbientSession, 0)}
	if s := OutputVariableListSess(testAmbientSession, []*Variable{arrShell}, "  ", true, sessOpts(testAmbientSession)); s != "" {
		t.Fatal("IsArray without AsArray must fail closed whole list", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputVariableList must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputGlobalVariables(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	out := OutputGlobalVariablesSess(testAmbientSession, []*Variable{v}, sessOpts(testAmbientSession))
	if !strings.Contains(out, "GLOBAL VARIABLES") || !strings.Contains(out, "g_1") {
		t.Fatal(out)
	}
	decl := OutputGlobalVariablesDeclsSess(testAmbientSession, []*Variable{v}, "extern ", sessOpts(testAmbientSession))
	if !strings.Contains(decl, "extern ") {
		t.Fatal(decl)
	}
	// no invent section-only header when all defs empty
	if s := OutputGlobalVariablesSess(testAmbientSession, []*Variable{{Name: "g_x", Type: GetIntTypeSess(testAmbientSession)}}, sessOpts(testAmbientSession)); s != "" {
		t.Fatal("empty globals must fail closed section", s)
	}
	if s := OutputGlobalVariablesDeclsSess(testAmbientSession, nil, "extern ", sessOpts(testAmbientSession)); s != "" {
		t.Fatal("nil globals must fail closed section", s)
	}
}

func TestMakeRandomIfFunc1UncertainPath(t *testing.T) {
	ReinstallTestProcessSingletons()
	// smoke: func_1 with FM does not panic on if generation
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessWrite, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f)), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	fm := NewFactMgrSess(testAmbientSession, f)
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	// seed may fail; retry
	var st *Stmt
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		cg2.EffectAccum = &eff
		cg2.Types = vs.Types
		st = MakeRandomIf(NewRngSess(testAmbientSession, seed), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts),
			NewStatementThresholdTableSess(testAmbientSession, opts), &cg2)
		if st != nil && st.Kind == StmtIfElse {
			break
		}
	}
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
}
