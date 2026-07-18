package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomArrayControlLe(t *testing.T) {
	r := NewRng(2)
	// force Le by unsigned
	init, limit, incr, testOp, incrOp := MakeRandomArrayControl(r, 10, false, 0)
	if testOp != BinCmpLe {
		t.Fatalf("unsigned want Le got %v", testOp)
	}
	if incrOp != AssignAdd {
		t.Fatal(incrOp)
	}
	if incr < 1 {
		t.Fatal(incr)
	}
	if limit != 10 {
		t.Fatalf("limit %d", limit)
	}
	_ = init
}

func TestMakeIterationUsesMustUseArrays(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(1), opts, nil, "g_1", GetIntType(), MakeInt(0), q)
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{8}
	av.ArraySizes = av.Sizes
	vs.GlobalList = []*Variable{&av.Variable}
	vs.Arrays = []*ArrayVariable{av}
	// add a loop ctrl candidate
	iv := CreateVariableQfer("g_2", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	cg.MustUseArrays = []*ArrayVariable{av}
	lc := MakeIteration(NewRng(5), opts, probs, vs, cg)
	if lc == nil {
		t.Fatal("nil lc")
	}
	// with array control limit should be size-1 (=7) for Le path typically
	if lc.LimitN > 8 {
		t.Fatalf("limit too large %d", lc.LimitN)
	}
}

func TestArrayOpLoopPassesMustUse(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_a", GetIntType(), MakeInt(0), q)
	av.Sizes = []int{5, 5}
	av.ArraySizes = av.Sizes
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQfer("g_iv", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	// force non-init path by calling setup+for
	avs := MakeRandomArrayLoopSetup(NewRng(3), opts, vs, cg)
	cg.MustUseArrays = avs
	st := MakeRandomFor(NewRng(4), opts, probs, vs, tables, stmtTab, cg)
	if st == nil || st.Loop == nil {
		t.Fatal("for")
	}
	out := (&Block{Stmts: []Stmt{*st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
}
