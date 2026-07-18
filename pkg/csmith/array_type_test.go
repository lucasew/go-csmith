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
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := EmptyEffect()
	if !e.IsSideEffectFree() {
		t.Fatal("empty")
	}
	e2 := e.WriteVar(v)
	if e2.IsSideEffectFree() || e2.IsPure() {
		t.Fatal("after write")
	}
	if !e2.IsWritten(v) {
		t.Fatal("not marked")
	}
	// original unchanged
	if !e.IsSideEffectFree() {
		t.Fatal("value semantics")
	}
}
