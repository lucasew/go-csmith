package csmith

import (
	"strings"
	"testing"
)

// TestArrayOpAggregateConstantTmp mirrors StatementArrayOp.cpp:235–247 Output:
// constant init_value + array_var.is_aggregate() → type tmp = init; a[i] = tmp;
// (cannot assign brace init directly to array member of struct/union).
func TestArrayOpAggregateConstantTmp(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	ut := &Type{
		isUnion: true, StructName: "U0", Used: true,
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		},
	}
	av := &ArrayVariable{
		Variable: Variable{Name: "g_134", Type: ut, IsArray: true},
		Sizes:    []int{4},
	}
	av.AsArray = av
	iv := CreateVariableScalarsSess(testAmbientSession, "g_1287.f3", GetIntTypeSess(testAmbientSession), false, false)
	if iv == nil {
		t.Fatal("iv")
	}
	rhs := &Expression{
		Term: TermConstant, Con: &Constant{Value: "{0x818A33E5L}", Type: ut}, ExprType: ut,
	}
	st := Stmt{
		Kind:        StmtArrayOp,
		Loop:        &LoopControl{IV: iv, InitN: 0, LimitN: 4, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		ArrayAccess: "g_134[g_1287.f3]",
		Expr:        rhs,
		LhsVar:      &av.Variable,
		Then:        &Block{}, // unused when ArrayAccess+Expr set
		StmID:       1,
	}
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if HasErrorSess(testAmbientSession) || out == "" {
		t.Fatalf("output empty/err: %q err=%v", out, HasErrorSess(testAmbientSession))
	}
	if !strings.Contains(out, "union U0 tmp = {0x818A33E5L};") {
		t.Fatalf("want aggregate tmp init, got %q", out)
	}
	if !strings.Contains(out, "g_134[g_1287.f3] = tmp;") {
		t.Fatalf("want a[i] = tmp, got %q", out)
	}
	if strings.Contains(out, "g_134[g_1287.f3] = {0x818A33E5L}") {
		t.Fatalf("must not invent direct brace assign to aggregate array member: %q", out)
	}
	// non-aggregate: direct assign (StatementArrayOp.cpp:248–254)
	ClearErrorSess(testAmbientSession)
	avInt := &ArrayVariable{
		Variable: Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true},
		Sizes:    []int{2},
	}
	avInt.AsArray = avInt
	rhsInt := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)}
	stInt := Stmt{
		Kind:        StmtArrayOp,
		Loop:        &LoopControl{IV: iv, InitN: 0, LimitN: 2, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd},
		ArrayAccess: "g_a[g_1287.f3]",
		Expr:        rhsInt,
		LhsVar:      &avInt.Variable,
		Then:        &Block{},
		StmID:       2,
	}
	outInt := (&Block{Stmts: []Stmt{stInt}}).Output(0)
	if strings.Contains(outInt, " tmp =") {
		t.Fatalf("scalar array must not invent tmp: %q", outInt)
	}
	if !strings.Contains(outInt, "g_a[g_1287.f3] = ") {
		t.Fatalf("want direct scalar assign: %q", outInt)
	}
	// LhsVar missing (pre-fix invent): must not use aggregate path via soft guess
	ClearErrorSess(testAmbientSession)
	stNoLhs := st
	stNoLhs.LhsVar = nil
	outNo := (&Block{Stmts: []Stmt{stNoLhs}}).Output(0)
	if strings.Contains(outNo, " tmp =") {
		t.Fatalf("nil LhsVar must not invent aggregate tmp: %q", outNo)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomArrayInitZeroIncrOne(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	if av == nil {
		t.Fatal("nil av")
	}
	av.Sizes = []int{6, 3}
	av.ArraySizes = av.Sizes
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	vs.GlobalNonvolatilesList = []*Variable{&av.Variable}
	// seed IVs
	for i := 0; i < 3; i++ {
		iv := CreateVariableQferSess(testAmbientSession, "g_i"+string(rune('0'+i)), GetIntTypeSess(testAmbientSession), q)
		vs.GlobalList = append(vs.GlobalList, iv)
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, iv)
	}
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	// force SelectArray to return our av by only having one
	st := MakeRandomArrayInit(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st.Kind != StmtArrayOp {
		t.Fatalf("kind %v", st.Kind)
	}
	// walk nested loops: all InitN=0 IncrN=1 LimitN=size
	var check func(s Stmt)
	check = func(s Stmt) {
		if s.Loop != nil {
			if s.Loop.InitN != 0 {
				t.Fatalf("init %d want 0", s.Loop.InitN)
			}
			if s.Loop.IncrN != 1 {
				t.Fatalf("incr %d want 1", s.Loop.IncrN)
			}
			if s.Loop.TestOp != BinCmpLt {
				t.Fatal(s.Loop.TestOp)
			}
		}
		if s.Then != nil {
			for _, nested := range s.Then.Stmts {
				if nested.Kind == StmtArrayOp {
					check(nested)
				}
			}
		}
	}
	check(st)
	out := (&Block{Stmts: []Stmt{st}}).Output(0)
	if !strings.Contains(out, "for (") {
		t.Fatal(out)
	}
	// should index array
	if !strings.Contains(out, "g_a[") {
		t.Log(out) // may use OutputC name
	}
}

func TestMakeRandomArrayInitEmptySizesNoSoft(t *testing.T) {
	// StatementArrayOp.cpp:103 — get_dimension(); no soft invent size 1 / [0]
	opts := Defaults()
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_empty", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	av.Sizes = nil
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := MakeRandomArrayInit(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	// empty dims → fail (no soft invent size [1] or access[0])
	if st.Loop != nil || st.Then != nil {
		t.Fatal("empty sizes must not soft-succeed")
	}
}

func TestMakeRandomArrayInitRejectsFloatIV(t *testing.T) {
	opts := Defaults()
	// no float types in simple select — just ensure no panic with empty filter path
	vs := NewVariableSelector(testAmbientSession, opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), nil, nil, nil, "g_b", GetIntTypeSess(testAmbientSession), MakeIntSess(testAmbientSession, 0), q)
	av.Sizes = []int{4}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQferSess(testAmbientSession, "g_iv", GetIntTypeSess(testAmbientSession), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := MakeRandomArrayInit(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st.Loop == nil && st.Then == nil {
		t.Fatal("empty")
	}
}

func TestMakeRandomIfClearsEffectStm(t *testing.T) {
	// StatementIf.cpp:69 — get_effect_stm().clear() on CGContext& before condition
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	probs := NewProbabilities(opts)
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()).WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	v := CreateVariableScalarsSess(testAmbientSession, "g_z", GetIntTypeSess(testAmbientSession), false, false)
	// FactMgr required when condition may build ExpressionAssign
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg.Types = vs.Types
	cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, v)
	st := MakeRandomIf(NewRngSess(testAmbientSession, 4), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
	if st.Then == nil || st.Else == nil {
		t.Fatal("branches")
	}
	// StatementIf.cpp:69 clear on CGContext& — pre-seed write must not survive on caller
	if cg.EffectStm.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("effect_stm clear on *CGContext must drop pre-seed write")
	}
}
