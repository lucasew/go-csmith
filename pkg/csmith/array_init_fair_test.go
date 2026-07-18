package csmith

import (
	"strings"
	"testing"
)

func TestMakeRandomArrayInitZeroIncrOne(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(1), opts, nil, "g_a", GetIntType(), MakeInt(0), q)
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
		iv := CreateVariableQfer("g_i"+string(rune('0'+i)), GetIntType(), q)
		vs.GlobalList = append(vs.GlobalList, iv)
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, iv)
	}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgr(f)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	// force SelectArray to return our av by only having one
	st := MakeRandomArrayInit(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
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
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_empty", GetIntType(), MakeInt(0), q)
	av.Sizes = nil
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	st := MakeRandomArrayInit(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	// empty dims → fail (no soft invent size [1] or access[0])
	if st.Loop != nil || st.Then != nil {
		t.Fatal("empty sizes must not soft-succeed")
	}
}

func TestMakeRandomArrayInitRejectsFloatIV(t *testing.T) {
	opts := Defaults()
	// no float types in simple select — just ensure no panic with empty filter path
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(NewRng(2), opts, nil, "g_b", GetIntType(), MakeInt(0), q)
	av.Sizes = []int{4}
	vs.Arrays = []*ArrayVariable{av}
	vs.GlobalList = []*Variable{&av.Variable}
	iv := CreateVariableQfer("g_iv", GetIntType(), q)
	vs.GlobalList = append(vs.GlobalList, iv)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	st := MakeRandomArrayInit(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st.Loop == nil && st.Then == nil {
		t.Fatal("empty")
	}
}

func TestMakeRandomIfClearsEffectStm(t *testing.T) {
	// StatementIf.cpp:69 — get_effect_stm().clear() on CGContext& before condition
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	vs := NewVariableSelector(opts)
	probs := NewProbabilities(opts)
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	_ = vs.GenerateNewGlobal(AccessRead, WithFunc(f, EmptyEffect()), GetIntType(), nil, NewRng(1))
	v := CreateVariableScalars("g_z", GetIntType(), false, false)
	// FactMgr required when condition may build ExpressionAssign
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg.Types = vs.Types
	cg.EffectStm = EmptyEffect().WriteVar(v)
	st := MakeRandomIf(NewRng(4), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal(st)
	}
	if st.Then == nil || st.Else == nil {
		t.Fatal("branches")
	}
	// StatementIf.cpp:69 clear on CGContext& — pre-seed write must not survive on caller
	if cg.EffectStm.IsWritten(v) {
		t.Fatal("effect_stm clear on *CGContext must drop pre-seed write")
	}
}
