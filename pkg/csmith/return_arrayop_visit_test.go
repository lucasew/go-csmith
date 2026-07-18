package csmith

import (
	"strings"
	"testing"
)

func TestReturnOutputDepthProtect(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	st := Stmt{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
	}
	b := &Block{Func: f, Stmts: []Stmt{st}, EmitDepthProtect: true}
	out := b.Output(0)
	// DEPTH-- must appear before return
	iDepth := strings.Index(out, "DEPTH--")
	iRet := strings.Index(out, "return")
	if iDepth < 0 || iRet < 0 || iDepth > iRet {
		t.Fatal(out)
	}
}

func TestArrayInitAggregateTmpEmit(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		},
	}
	av := CreateVariableQfer("g_a", st, NewCVQualifiers([]bool{false}, []bool{false}))
	// fake constant aggregate
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: st, Value: "{1}"}, ExprType: st}
	asg := Stmt{
		Kind:        StmtAssign,
		LhsVar:      av,
		ArrayAccess: "g_a[i]",
		Expr:        rhs,
		AssignOp:    AssignSimple,
	}
	out := (&Block{Stmts: []Stmt{asg}}).Output(0)
	if !strings.Contains(out, "tmp") || !strings.Contains(out, "g_a[i] = tmp") {
		t.Fatal(out)
	}
}

func TestVisitFactsStatementArrayOpInit(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), q)
	av.Sizes = []int{3}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQfer("i", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	st := MakeRandomArrayInit(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st.Kind != StmtArrayOp {
		t.Fatal(st.Kind)
	}
	cgp := cg
	if !VisitFactsStatementArrayOp(&st, &cgp, opts) {
		t.Fatal("visit array op init")
	}
}

func TestMakeRandomReturnNoEagerVisitFacts(t *testing.T) {
	// StatementReturn.cpp:54–72 — make_random does not visit_facts; append_return does
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableQfer("rv", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false}))
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(1))
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	st := MakeRandomReturn(NewRng(3), opts, vs, &cg)
	if st.Kind != StmtReturn || st.Expr == nil {
		t.Fatal(st)
	}
	// no map_visited / map_stm_effect until explicit visit or append_return
	if st.StmID > 0 && fm.MapVisited != nil && fm.MapVisited[st.StmID] {
		t.Fatal("make_random must not mark visited (no eager visit_facts)")
	}
}
