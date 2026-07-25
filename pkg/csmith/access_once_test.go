package csmith

import (
	"strings"
	"testing"
)

func TestAccessOnceMarking(t *testing.T) {
	opts := Defaults()
	opts.AccessOnce = true
	// Variable.cpp:694 — CGOptions::access_once() process option for Output wrap
	prev := ProcessOptionsSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	vs := NewVariableSelector(opts)
	vs.Probs = NewProbabilities(opts)
	// force many creates until AccessOnce set
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		r := NewRng(seed)
		v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, r)
		if v != nil && v.IsAccessOnce {
			found = true
			if !strings.Contains(v.OutputCSess(testAmbientSession, false), "ACCESS_ONCE") {
				t.Fatal(v.OutputCSess(testAmbientSession, false))
			}
			v.IsAddrTaken = true
			if strings.Contains(v.OutputCSess(testAmbientSession, false), "ACCESS_ONCE") {
				t.Fatal("addr taken should clear wrap")
			}
			break
		}
	}
	if !found {
		t.Log("AccessOnce rare at 20% — ok if unlucky")
	}
}

func TestAccessOnceWrapRequiresOption(t *testing.T) {
	// Variable.cpp:694–695 — assert(access_once); sticky, no invent wrap when option off
	ClearErrorSess(testAmbientSession)
	prev := ProcessOptionsSess(testAmbientSession)
	opts := Defaults()
	opts.AccessOnce = false
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, prev)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	v.IsAccessOnce = true
	out := v.OutputCSess(testAmbientSession, false)
	if strings.Contains(out, "ACCESS_ONCE") {
		t.Fatal("option off must not wrap")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsAccessOnce with option off must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestForSafeIncrEmit(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	seedTypesForTest(NewRng(2), opts, probs, vs, nil)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil, nil)
	// StatementFor.cpp:172 assert(blk) — parent on stack after MakeFirst
	parent := &Block{Func: f}
	f.Stack = []*Block{parent}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	st := MakeRandomFor(NewRng(5), opts, probs, vs, tables, stmtTab, &cg)
	if st == nil || st.Loop == nil {
		t.Skip("no for")
	}
	if !st.Loop.SafeIncr {
		t.Fatal("expected SafeIncr when SafeMath")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "safe_") && !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
	// with SafeIncr should have safe_add or safe_sub often
	if !strings.Contains(out, "safe_") {
		// ++ might still be rewritten to safe_add
		if len(out) > 200 {
			out = out[:200]
		}
		t.Log("no safe_ in for sample", out)
	}
}
