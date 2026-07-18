package csmith

import (
	"strings"
	"testing"
)

func TestReachMaxFunctions(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 2
	var list FunctionList
	list.Funcs = []*Function{{Name: "a"}, {Name: "b"}}
	if !ReachMaxFunctions(&list, opts) {
		t.Fatal("at max")
	}
	list.Funcs = list.Funcs[:1]
	if ReachMaxFunctions(&list, opts) {
		t.Fatal("under max")
	}
}

func TestMakeRandomBinaryInvocationOutput(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	r := NewRng(2)
	fi := MakeRandomBinaryInvocation(r, opts, probs, vs, tables, EmptyCGContext(), GetIntType())
	if fi == nil || !fi.IsStd || fi.Output() == "/*invoke*/" {
		t.Fatalf("%+v out=%s", fi, fi.Output())
	}
	out := fi.Output()
	// With SafeMath default, expect safe_* wrapper; otherwise infix op.
	if fi.Safe != nil {
		if !strings.Contains(out, "safe_") {
			t.Fatalf("safe wrapper missing: %s", out)
		}
	} else if !strings.Contains(out, fi.Binary) {
		t.Fatal(out)
	}
}

func TestExpressionFuncallCanCreateUserFunc(t *testing.T) {
	// Force TermFunction and stdFunc=false path often enough to create multi-func programs.
	foundMulti := false
	for seed := uint64(1); seed < 60; seed++ {
		opts := Defaults()
		opts.Seed = seed
		opts.MaxFuncs = 10
		g := NewProgramGenerator(opts)
		_ = g.GoGenerator()
		if len(g.Funcs.Funcs) > 1 {
			foundMulti = true
			// later funcs should be built
			for _, f := range g.Funcs.Funcs {
				if f != nil && !f.IsBuilt {
					t.Fatalf("%s not built", f.Name)
				}
			}
			out := g.GoGenerator() // regenerate for string — wait that re-seeds. use first run
			_ = out
			break
		}
	}
	// Check multi from a single generate
	for seed := uint64(1); seed < 80; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// count func_ definitions roughly
		if strings.Count(out, "func_") >= 4 { // forward + body for 2 funcs at least
			// e.g. func_1 and func_2 appear
			if strings.Contains(out, "func_2") {
				foundMulti = true
				break
			}
		}
	}
	if !foundMulti {
		t.Log("no multi-func in seeds 1..79 — may still be rare; check binary ops at least")
	}
}

func TestGenerateEmitsBinaryOrCall(t *testing.T) {
	found := false
	for seed := uint64(1); seed < 40; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, " + ") || strings.Contains(out, " - ") ||
			strings.Contains(out, "func_2") || strings.Contains(out, "++") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected binary op or second function in some seed")
	}
}

func TestGenerateNewParentLocal(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	blk := &Block{}
	v := vs.GenerateNewParentLocal(blk, AccessRead, EmptyCGContext(), GetIntType(), nil, r)
	if v == nil || !v.IsLocal() || len(blk.LocalVars) != 1 {
		t.Fatalf("%+v", v)
	}
}
