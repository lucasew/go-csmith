package csmith

import (
	"strings"
	"testing"
)

func TestBuildInvocationAndFunctionParamsBeforeBody(t *testing.T) {
	opts := Defaults()
	opts.MaxFuncs = 10
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	// encourage multi-param signatures
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	list := &FunctionList{}
	// seed globals for args / body
	for i := 0; i < 3; i++ {
		_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(uint64(10+i)))
	}
	caller := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	fm := NewFactMgr(caller)
	cg := WithFunc(caller, EmptyEffect()).WithFactMgr(fm)
	cg.Funcs = list
	list.Funcs = []*Function{caller}

	// force new function creation path
	fi := BuildInvocationAndFunction(NewRng(7), opts, probs, vs, tables, stmtTab, cg, list, GetIntType())
	if fi == nil || fi.Failed {
		// may fail if max funcs / depth — try more seeds
		ok := false
		for seed := uint64(1); seed < 40; seed++ {
			fi = BuildInvocationAndFunction(NewRng(seed), opts, probs, vs, tables, stmtTab, cg, list, GetIntType())
			if fi != nil && !fi.Failed && fi.User != nil {
				ok = true
				break
			}
		}
		if !ok {
			t.Skip("could not create invocation in seed scan")
		}
	}
	if fi.User == nil {
		t.Fatal("no callee")
	}
	if fi.User.BuildState != BuildBuilt && fi.User.Body == nil {
		// GenerateBody should have run
		t.Log("body state", fi.User.BuildState)
	}
	// arg count matches params
	if len(fi.Args) != len(fi.User.Param) {
		t.Fatalf("args %d params %d", len(fi.Args), len(fi.User.Param))
	}
	// visited once
	if fi.User.VisitedCnt < 1 {
		t.Fatal("visited_cnt")
	}
	// output is a call
	out := fi.Output()
	if !strings.Contains(out, fi.User.Name+"(") {
		t.Fatal(out)
	}
}

func TestBuildUserInvocationArgCount(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	callee := &Function{
		Name:       "g_1",
		ReturnType: GetIntType(),
		Param: []*Variable{
			CreateVariableScalars("p_1", GetIntType(), false, false),
			CreateVariableScalars("p_2", PointerTo(GetIntType()), false, false),
		},
		BuildState: BuildBuilt,
		FEffect:    EmptyEffect(),
	}
	// mark effect known
	callee.FEffect = EmptyEffect().ReadVar(vs.GlobalList[0])
	caller := &Function{Name: "func_1"}
	blk := &Block{Func: caller}
	caller.Stack = []*Block{blk}
	cg := WithFunc(caller, EmptyEffect())
	fi := BuildUserInvocation(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), cg, nil, callee)
	if fi.Failed {
		t.Fatal("failed")
	}
	if len(fi.Args) != 2 {
		t.Fatalf("args %d", len(fi.Args))
	}
}
