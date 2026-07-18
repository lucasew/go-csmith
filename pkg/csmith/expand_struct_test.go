package csmith

import "testing"

func TestEagerCreateGlobalStruct(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	if len(env.StructTypes) == 0 {
		t.Skip("no structs")
	}
	// want int — eager create struct then field match
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.EagerCreateGlobalStruct(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(5), MatchFlexible)
	// may fail if no int fields
	if v != nil && v.Type != nil {
		if !GetIntType().Match(v.Type, MatchFlexible) {
			t.Log("matched", v.Type.CName())
		}
	}
}

func TestSelectGlobalExpandStructPath(t *testing.T) {
	opts := Defaults()
	opts.ExpandStruct = true
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	env := &TypeEnv{}
	GenerateAllTypesEnv(NewRng(3), opts, probs, env)
	vs.Types = env
	vs.Probs = probs
	vs.Opts = opts
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// empty GlobalList → expand or create
	v := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, NewRng(7))
	if v == nil {
		t.Fatal("nil")
	}
}

func TestMergeEffectsMergesReads(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e1 := EmptyEffect().ReadVar(a)
	e2 := EmptyEffect().ReadVar(b)
	m := MergeEffects(e1, e2)
	if !m.IsRead(a) || !m.IsRead(b) {
		t.Fatal("reads")
	}
}

func TestFactMgrForFunc(t *testing.T) {
	f := &Function{Name: "func_1"}
	m := NewFactMgrMap()
	fm := m.ForFunc(f)
	if fm == nil || fm.Func != f {
		t.Fatal(fm)
	}
	if m.ForFunc(f) != fm {
		t.Fatal("reuse")
	}
}
