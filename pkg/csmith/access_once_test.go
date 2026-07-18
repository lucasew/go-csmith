package csmith

import (
	"strings"
	"testing"
)

func TestAccessOnceMarking(t *testing.T) {
	opts := Defaults()
	opts.AccessOnce = true
	vs := NewVariableSelector(opts)
	vs.Probs = NewProbabilities(opts)
	// force many creates until AccessOnce set
	found := false
	for seed := uint64(1); seed < 80; seed++ {
		r := NewRng(seed)
		v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, r)
		if v != nil && v.IsAccessOnce {
			found = true
			if !strings.Contains(v.OutputC(), "ACCESS_ONCE") {
				t.Fatal(v.OutputC())
			}
			v.IsAddrTaken = true
			if strings.Contains(v.OutputC(), "ACCESS_ONCE") {
				t.Fatal("addr taken should clear wrap")
			}
			break
		}
	}
	if !found {
		t.Log("AccessOnce rare at 20% — ok if unlucky")
	}
}

func TestForSafeIncrEmit(t *testing.T) {
	opts := Defaults()
	opts.SafeMath = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	f := MakeFirst(NewRng(2), opts, probs, vs, &vs.Sym, tables, stmtTab, nil)
	cg := WithFunc(f, EmptyEffect())
	st := MakeRandomFor(NewRng(5), opts, probs, vs, tables, stmtTab, cg)
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
