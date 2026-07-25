package csmith

import (
	"strings"
	"testing"
)

func TestReturnOutputDepthProtect(t *testing.T) {
	// StatementReturn.cpp:127–129 / Block.cpp:255 — emit gates on CGOptions::depth_protect
	opts := Defaults()
	opts.DepthProtect = true
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	st := Stmt{
		Kind: StmtReturn,
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)},
	}
	// Body flag may be set by set_depth_protect(true); emit still requires CGOptions.
	b := &Block{Func: f, Stmts: []Stmt{st}, EmitDepthProtect: true}
	out := b.Output(0)
	// DEPTH-- must appear before return
	iDepth := strings.Index(out, "DEPTH--")
	iRet := strings.Index(out, "return")
	if iDepth < 0 || iRet < 0 || iDepth > iRet {
		t.Fatal(out)
	}
	// default options must not invent DEPTH++/--
	SetProcessOptionsSess(testAmbientSession, Defaults())
	out2 := b.Output(0)
	if strings.Contains(out2, "DEPTH") {
		t.Fatal("depth_protect off must not invent DEPTH:", out2)
	}
}

func TestArrayInitAggregateTmpEmit(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	av := CreateVariableQferSess(testAmbientSession, "g_a", st, NewCVQualifiers([]bool{false}, []bool{false}))
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
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	av.Sizes = []int{3}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQferSess(testAmbientSession, "i", GetIntTypeSess(testAmbientSession), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := MakeRandomArrayInit(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st.Kind != StmtArrayOp {
		t.Fatal(st.Kind)
	}
	cgp := cg
	if !VisitFactsStatementArrayOp(&st, &cgp, opts) {
		t.Fatal("visit array op init")
	}
}

func TestMakeRandomReturnNoEagerVisitFacts(t *testing.T) {
	ReinstallTestProcessSingletons()
	// StatementReturn.cpp:54–72 — make_random does not visit_facts; append_return does
	opts := Defaults()
	opts.NoReturnDeadPointer = true
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableQferSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false}))
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	st := MakeRandomReturn(NewRngSess(testAmbientSession, 3), opts, vs, &cg)
	if st.Kind != StmtReturn || st.Expr == nil {
		t.Fatal(st)
	}
	// no map_visited / map_stm_effect until explicit visit or append_return
	if st.StmID > 0 && fm.MapVisited != nil && fm.MapVisited[st.StmID] {
		t.Fatal("make_random must not mark visited (no eager visit_facts)")
	}
}
