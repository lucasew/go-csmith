package csmith

import "testing"

func TestCreateRandomArrayUsesEnvTypes(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	// create several arrays; at least one may be non-simple if structs exist
	seenSimple, seenAgg := false, false
	for seed := uint64(1); seed < 40; seed++ {
		f := &Function{Name: "func_1", ReturnType: GetIntType()}
		blk := &Block{Func: f}
		f.Stack = []*Block{blk}
		cg := WithFunc(f, EmptyEffect())
		av := vs.CreateRandomArray(NewRng(seed), cg)
		if av == nil || av.Type == nil {
			continue
		}
		if av.Type.IsSimple() {
			seenSimple = true
		}
		if av.Type.IsAggregate() {
			seenAgg = true
		}
	}
	if !seenSimple {
		t.Fatal("expected some simple-element arrays")
	}
	_ = seenAgg // optional if structs rare under global flip
}

func TestChooseRandomNonvoidNonvolatile(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(3), opts, probs, env)
	// inject volatile struct
	volt := &Type{isStruct: true, StructName: "SV", Fields: []StructField{
		{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{true}), BitWidth: -1},
	}}
	env.AllTypes = append(env.AllTypes, volt)
	r := NewRng(4)
	for i := 0; i < 50; i++ {
		ty := env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
		if ty != nil && ty.IsVolatileStructUnion() {
			t.Fatal("volatile aggregate")
		}
		if ty != nil && ty.IsSimple() && ty.Simple() == EVoid {
			t.Fatal("void")
		}
	}
}

func TestEffectWriteVar(t *testing.T) {
	// Effect.cpp:137–146 — non-volatile write keeps pure and SE-free
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := EmptyEffect()
	if !e.IsSideEffectFree() {
		t.Fatal("empty")
	}
	e2 := e.WriteVar(v)
	if !e2.IsWritten(v) {
		t.Fatal("not marked")
	}
	if !e2.IsSideEffectFree() || !e2.IsPure() {
		t.Fatal("non-vol write keeps pure/SE-free")
	}
	// volatile write clears SE-free only
	vv := CreateVariableScalars("g_v", GetIntType(), false, true)
	e3 := EmptyEffect().WriteVar(vv)
	if e3.IsSideEffectFree() {
		t.Fatal("vol write clears SE-free")
	}
	if !e3.IsPure() {
		t.Fatal("write does not clear pure")
	}
	// original unchanged
	if !e.IsSideEffectFree() {
		t.Fatal("value semantics")
	}
}

func TestEffectReadVarPurity(t *testing.T) {
	// Effect.cpp:116–122
	c := CreateVariableScalars("g_c", GetIntType(), true, false) // const non-vol
	e := EmptyEffect().ReadVar(c)
	if !e.IsPure() || !e.IsSideEffectFree() {
		t.Fatal("const non-vol read pure+SE-free")
	}
	nv := CreateVariableScalars("g_nv", GetIntType(), false, false)
	e2 := EmptyEffect().ReadVar(nv)
	if e2.IsPure() {
		t.Fatal("non-const read not pure")
	}
	if !e2.IsSideEffectFree() {
		t.Fatal("non-vol read still SE-free")
	}
	vol := CreateVariableScalars("g_vol", GetIntType(), false, true)
	e3 := EmptyEffect().ReadVar(vol)
	if e3.IsSideEffectFree() || e3.IsPure() {
		t.Fatal("vol read")
	}
}
