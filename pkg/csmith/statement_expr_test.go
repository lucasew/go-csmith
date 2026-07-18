package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomExprStmtUserCall(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	stmtTab := NewStatementThresholdTable(opts)
	r := NewRng(2)
	var list FunctionList
	f := MakeFirst(r, opts, probs, vs, &vs.Sym, tables, stmtTab, &list)
	cg := WithFunc(f, EmptyEffect()).WithFuncList(&list)
	st := MakeRandomExprStmt(NewRng(7), opts, probs, vs, tables, cg)
	if st.Kind != StmtInvoke {
		t.Fatalf("kind %v", st.Kind)
	}
	// may fail if max funcs / no list — with list should often succeed
	if st.Expr != nil && st.Expr.Invoke != nil && !st.Expr.Invoke.Failed {
		out := st.Expr.Output()
		if out == "" || out == "/*bad_call*/" {
			t.Fatal(out)
		}
	}
}

func TestMakeRandomExprStmtEmitSemicolon(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	tables := NewExprTables(opts)
	// force a binary invoke via synthetic expr
	fi := MakeRandomBinaryInvocation(NewRng(3), opts, probs, vs, tables, EmptyCGContext(), GetIntType())
	st := Stmt{Kind: StmtInvoke, Expr: &Expression{Term: TermFunction, Invoke: fi}}
	b := &Block{Stmts: []Stmt{st}}
	out := b.Output(0)
	if !strings.Contains(out, ";") {
		t.Fatal(out)
	}
	if strings.Contains(out, "invoke-stub") {
		t.Fatal("still stub")
	}
}
