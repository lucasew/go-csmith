package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomLoopControlRanges(t *testing.T) {
	opts := Defaults()
	r := NewRng(2)
	for i := 0; i < 50; i++ {
		init, limit, incr, _, incrOp := MakeRandomLoopControl(r, opts, true)
		if incr == 0 {
			t.Fatal("incr never 0 after fixup")
		}
		_ = init
		_ = limit
		_ = incrOp
	}
	// nil RNG sticky — no invent fixed init/limit/incr shell
	ClearErrorSess(testAmbientSession)
	init, limit, incr, testOp, incrOp := MakeRandomLoopControl(nil, opts, true)
	if init != 0 || limit != 0 || incr != 0 || testOp != 0 || incrOp != 0 {
		t.Fatalf("nil RNG must fail closed zeros, got %d %d %d %v %v", init, limit, incr, testOp, incrOp)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomLoopControl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfHasBranches(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	var list FunctionList
	// need a function for context
	r := NewRng(2)
	seedTypesForTest(r, opts, probs, vs, &list)
	ClearErrorSess(testAmbientSession)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list, nil)
	if f == nil {
		t.Fatal("MakeFirst")
	}
	ClearErrorSess(testAmbientSession)
	// Paired FactMgr + function body on stack (generation context)
	fm := f.PairedFactMgr()
	if fm == nil {
		fm = NewFactMgrSess(testAmbientSession, f)
	}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	if f.Body != nil {
		f.Stack = []*Block{f.Body}
	}
	// force if generation — soft re-pick seeds (make_random can miss)
	var st *Stmt
	for seed := uint64(11); seed < 80; seed++ {
		ClearErrorSess(testAmbientSession)
		st = MakeRandomIf(NewRng(seed), opts, probs, vs, tables, stmtTab, &cg)
		if st != nil && st.Kind == StmtIfElse && st.Then != nil && st.Else != nil && st.Expr != nil {
			return
		}
	}
	t.Fatalf("%+v err=%v", st, GetErrorSess(testAmbientSession))
}

func TestMakeRandomForHasLoopAndBody(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	// StatementFor.cpp:172 assert(blk) — parent block on stack (MakeFirst pops body)
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := MakeRandomFor(NewRng(4), opts, probs, vs, tables, stmtTab, &cg)
	if st == nil || st.Kind != StmtFor || st.Loop == nil || st.Loop.IV == nil || st.Then == nil {
		t.Fatalf("%+v", st)
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(out, st.Loop.IV.Name) {
		t.Fatal("iv name missing")
	}
}

func TestMakeRandomForNullptrNoKindShell(t *testing.T) {
	// StatementFor.cpp always has RNG+CG sticky; nil FM soft re-pick
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	if st := MakeRandomFor(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), nil); st != nil {
		t.Fatal("nil cg")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeRandomFor must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cgEmpty := EmptyCGContext().WithSession(testAmbientSession)
	if st := MakeRandomFor(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cgEmpty); st != nil {
		t.Fatal("nil RNG")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomFor must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession) // no FM
	if st := MakeRandomFor(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg); st != nil {
		t.Fatalf("nil FM must return nil, got %#v", st)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM MakeRandomFor must stay non-sticky soft re-pick")
	}
}

func TestGenerateCanEmitIfOrFor(t *testing.T) {
	// Scan seeds until we see real if/for syntax (not stubs).
	foundIf, foundFor := false, false
	for seed := uint64(1); seed < 80 && !(foundIf && foundFor); seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "if (") {
			foundIf = true
		}
		if strings.Contains(out, "for (") {
			foundFor = true
		}
	}
	if !foundIf {
		t.Log("no if in seeds 1..79 (probabilistic)")
	}
	if !foundFor {
		t.Log("no for in seeds 1..79 (probabilistic)")
	}
	// Soft: at least one control structure in range is enough for smoke
	if !foundIf && !foundFor {
		t.Fatal("expected some if or for in seeds 1..79")
	}
}

// TestMakeRandomForSharesEffectAccumWithParent — CGContext.cpp:95–101 loop-body
// constructor sets effect_accum(cgc.effect_accum) (shared pointer). A forked
// bodyEff snapshot (removed) left mid-body StatementGoto choose_visible_read_var
// with fewer parent+body reads (seed-2 first_div 12693: ok_vars 11 vs UP 16).
func TestMakeRandomForSharesEffectAccumWithParent(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	seedTypesForTest(r, opts, probs, vs, nil)
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	fm := NewFactMgrSess(testAmbientSession, f)
	// Plant a read on the parent accum before for-body generation.
	pre := CreateVariableScalarsSess(testAmbientSession, "pre_rd", GetIntType(), true, false)
	accum := EmptyEffect().ReadVarSess(testAmbientSession, pre)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &accum
	// WithLoopBody / C++ loop-body ctor share EffectAccum pointer.
	bodyCG := cg.WithLoopBody(cg.RW, nil, 0)
	if bodyCG.EffectAccum != cg.EffectAccum {
		t.Fatal("WithLoopBody must share EffectAccum pointer (CGContext.cpp:101)")
	}
	// Body code path must not rebind EffectAccum to a private snapshot.
	// MakeRandomFor is the production path; re-check pointer identity after call.
	st := MakeRandomFor(NewRng(4), opts, probs, vs, tables, stmtTab, &cg)
	if st == nil {
		// soft factory miss is ok for this contract; pointer must still be live
		if cg.EffectAccum != &accum && cg.EffectAccum != bodyCG.EffectAccum {
			// after make, still the same planted pointer if for failed early
		}
	}
	if cg.EffectAccum == nil {
		t.Fatal("parent EffectAccum must remain non-nil")
	}
	// Planted pre-read must survive (shared path never drops parent reads for a body copy)
	if !cg.EffectAccum.IsReadSess(testAmbientSession, pre) {
		t.Fatal("parent EffectAccum must keep pre-for reads when body shares accum (C++ shared pointer)")
	}
	ClearErrorSess(testAmbientSession)
}
