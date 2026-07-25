package csmith

import (
	"strings"
	"testing"
)

func TestStmtReturnMustReturn(t *testing.T) {
	if !(Stmt{Kind: StmtReturn}).MustReturn() {
		t.Fatal("return must_return")
	}
	if (Stmt{Kind: StmtAssign}).MustReturn() {
		t.Fatal("assign")
	}
}

func TestIfMustReturnBothBranches(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := Stmt{
		Kind: StmtIfElse,
		Then: &Block{Stmts: []Stmt{{Kind: StmtReturn}}},
		Else: &Block{Stmts: []Stmt{{Kind: StmtReturn}}},
	}
	if !st.MustReturn() {
		t.Fatal("both return")
	}
	st.Else = &Block{Stmts: []Stmt{{Kind: StmtAssign}}}
	if st.MustReturn() {
		t.Fatal("only then")
	}
	// incomplete if — nil arm fails closed (no invent must_return soft-skip hole)
	st.Else = nil
	if st.MustReturn() {
		t.Fatal("nil Else must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Else MustReturn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestForArrayOpMustReturnBaseFalse — Statement.h:187 default false; StatementFor /
// StatementArrayOp do not override. A for/array-op whose body returns must NOT
// stop Block::make_random (seed-2 e13830 parent stack depth).
func TestForArrayOpMustReturnBaseFalse(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	body := &Block{Stmts: []Stmt{{Kind: StmtReturn}}}
	for _, kind := range []StatementType{StmtFor, StmtArrayOp} {
		st := Stmt{Kind: kind, Then: body}
		if st.MustReturn() {
			t.Fatalf("%v MustReturn must be false (Statement.h default), body return ignored", kind)
		}
		if st.MustJump() {
			t.Fatalf("%v MustJump must be false (Statement.h default)", kind)
		}
		if HasErrorSess(testAmbientSession) {
			t.Fatalf("%v must not sticky-error on complete body", kind)
		}
	}
	// Parent block continues after for even when for-body last is return.
	parent := &Block{Stmts: []Stmt{
		{Kind: StmtFor, Then: body},
		{Kind: StmtAssign},
	}}
	if parent.MustReturn() {
		t.Fatal("parent ending in assign must not must_return via for body")
	}
	// After only the for (as if Block stopped early on invent for.MustReturn):
	// Block with last=for must not must_return either.
	onlyFor := &Block{Stmts: []Stmt{{Kind: StmtFor, Then: body}}}
	if onlyFor.MustReturn() {
		t.Fatal("block last=for must not must_return (for base false)")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBlockMustReturnLast(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	b := &Block{Stmts: []Stmt{{Kind: StmtAssign}, {Kind: StmtReturn}}}
	if !b.MustReturn() {
		t.Fatal("last return")
	}
	b2 := &Block{Stmts: []Stmt{{Kind: StmtReturn}, {Kind: StmtLabel, SourceLabel: "x"}}}
	// trailing label after return — still last non-label is return
	// wait order is return then label — MustReturn skips labels from end, finds return
	if !b2.MustReturn() {
		t.Fatal("label after return")
	}
}

func TestNeedReturnStmt(t *testing.T) {
	f := &Function{ReturnType: GetIntType()}
	if !f.NeedReturnStmt() {
		t.Fatal("int")
	}
	f.ReturnType = GetSimpleType(EVoid)
	if f.NeedReturnStmt() {
		t.Fatal("void")
	}
	// incomplete without ReturnType — no invent "no return needed"
	if !(&Function{}).NeedReturnStmt() {
		t.Fatal("nil ReturnType must fail closed need-return")
	}
}

func TestGenerateFunctionsHaveReturnWhenNeeded(t *testing.T) {
	// non-void funcs should contain "return" in body
	for seed := uint64(1); seed < 30; seed++ {
		opts := Defaults()
		opts.Seed = seed
		out, err := Generate(opts)
		if err != nil {
			t.Fatal(err)
		}
		// crude: every func_N body that is not void should have return —
		// at least func_1 with non-void almost always
		if strings.Contains(out, "int32_t func_1") || strings.Contains(out, "uint32_t func_1") ||
			strings.Contains(out, "int func_1") || strings.Contains(out, "int8_t func_1") ||
			strings.Contains(out, "int16_t func_1") || strings.Contains(out, "int64_t func_1") ||
			strings.Contains(out, "uint") && strings.Contains(out, "func_1") {
			if !strings.Contains(out, "return ") {
				t.Fatalf("seed %d non-void program without return", seed)
			}
			return
		}
	}
}

func TestHashNoEmptyArrayLoops(t *testing.T) {
	// pointer array should produce empty hash (skipped)
	// IsArray without AsArray is incomplete IR sticky empty (suite hygiene ClearError)
	ClearErrorSess(testAmbientSession)
	v := &Variable{
		Name: "g_p", Type: PointerTo(GetIntType()), IsArray: true, ArraySizes: []int{4},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if v.HashOutputSess(testAmbientSession) != "" {
		t.Fatal("pointer array must not hash")
	}
	// sticky residual from IsArray without AsArray — clear so later hash tests stay complete
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHashGlobalVarsSharedIndices(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	CtrlVarsDoFinalizationSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	// live AsArray required for GetMaxArrayDimension / hashArrayVariable
	ga := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2, 3}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		Sizes:    []int{2, 3},
	}
	ga.AsArray = ga
	gb := &ArrayVariable{
		Variable: Variable{Name: "g_b", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		Sizes:    []int{4},
	}
	gb.AsArray = gb
	vs.GlobalList = []*Variable{&ga.Variable, &gb.Variable}
	// OutputHashFuncDef path: declare ctrl vars then hash
	dimen := GetMaxArrayDimensionSess(testAmbientSession, vs.GlobalList)
	ctrl := GetNewCtrlVarsSess(testAmbientSession, opts)
	decl := OutputArrayCtrlVarsSess(testAmbientSession, ctrl, dimen, "    ")
	out := decl + HashGlobalVariables(vs)
	// one combined "int i, j;" (max dim 2)
	if !strings.Contains(out, "int i, j;") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "g_a[i][j]") || !strings.Contains(out, "g_b[i]") {
		t.Fatal(out)
	}
}
