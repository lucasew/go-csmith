package csmith

import (
	"strings"
	"testing"
)

func TestLocalOutputDef(t *testing.T) {
	ClearError()
	b := &Block{}
	lv := CreateVariableScalars("l_1", GetIntType(), true, false)
	lv.Init = MakeInt(2)
	b.LocalVars = []*Variable{lv}
	out := b.Output(0)
	if !strings.Contains(out, "const") || !strings.Contains(out, "l_1") || !strings.Contains(out, "2") {
		t.Fatal(out)
	}
}

func TestBlockOutputDefResidualSticky(t *testing.T) {
	// OutputDef residual soft invent was soft-continue later locals invent partial block.
	ClearError()
	good := CreateVariableScalars("l_ok", GetIntType(), false, false)
	good.Init = MakeInt(1)
	// incomplete InitExpr residual OutputDef
	bad := CreateVariableScalars("l_bad", GetIntType(), false, false)
	bad.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil
	b := &Block{LocalVars: []*Variable{good, bad}}
	if s := b.Output(0); s != "" {
		t.Fatal("OutputDef residual must fail closed whole Block.Output, not invent later locals", s)
	}
	if !HasError() {
		t.Fatal("OutputDef residual Block.Output must SetError sticky")
	}
	ClearError()
}

func TestBlockOutputInvokeResidualSticky(t *testing.T) {
	// Expr.Output residual soft invent was soft-continue later stmts invent partial block.
	ClearError()
	good := Stmt{
		Kind: StmtInvoke, StmID: 1,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	bad := Stmt{
		Kind: StmtInvoke, StmID: 2,
		Expr: &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}, // Type-nil residual
	}
	b := &Block{Stmts: []Stmt{good, bad}}
	if s := b.Output(0); s != "" {
		t.Fatal("invoke Output residual must fail closed whole Block.Output", s)
	}
	if !HasError() {
		t.Fatal("invoke Output residual Block.Output must SetError sticky")
	}
	ClearError()
}

func TestFunctionParamQualified(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("func_1_rv", GetIntType(), true, false)
	p := CreateVariableScalars("p_1", GetIntType(), true, true)
	f.Param = []*Variable{p}
	decl := f.OutputForwardDecl()
	if !strings.Contains(decl, "const") || !strings.Contains(decl, "p_1") {
		t.Fatal(decl)
	}
	if !strings.Contains(decl, "volatile") {
		t.Fatal("param vol", decl)
	}
	// return type from RV includes const
	if !strings.HasPrefix(strings.TrimSpace(decl), "const") {
		t.Fatal("return", decl)
	}
}
