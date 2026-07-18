package csmith

import (
	"strings"
	"testing"
)

func TestPtrModifiedInRhs(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	lhs := &Lhs{Var: p, Type: GetIntType()} // *p
	cg := EmptyCGContext()
	// RHS wrote the pointer itself
	cg.EffectStm = EmptyEffect().WriteVar(p)
	if !cg.PtrModifiedInRhs(lhs, nil) {
		t.Fatal("ptr written")
	}
	cg.EffectStm = EmptyEffect()
	if cg.PtrModifiedInRhs(lhs, nil) {
		t.Fatal("clean")
	}
}

func TestOutputDefWithAttrs(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	v.Init = MakeInt(0)
	// force attrs with 100% boolean
	ClearAttrGenerators()
	varAttrGenerator = &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "unused", Prob: 100},
	}}
	s := v.OutputDefFull(true, false, true, NewRng(1))
	if !strings.Contains(s, "__attribute__((unused))") {
		t.Fatal(s)
	}
	ClearAttrGenerators()
}

func TestOutputFuncWithAttrs(t *testing.T) {
	f := &Function{Name: "func_1", ReturnType: GetIntType(), Body: &Block{}}
	ClearAttrGenerators()
	funcAttrGenerator = &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "noinline", Prob: 100},
	}}
	d := f.OutputForwardDeclOpts(true, NewRng(1), true)
	if !strings.Contains(d, "static ") || !strings.Contains(d, "noinline") {
		t.Fatal(d)
	}
	ClearAttrGenerators()
}

func TestArrayOutputInitNoPostIncr(t *testing.T) {
	av := &ArrayVariable{
		Variable: Variable{Name: "l_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{3}},
		Sizes:    []int{3},
		Block:    &Block{}, // local
	}
	// force not NoLoopInitializer — local non-const
	av.Init = MakeInt(0)
	out := av.OutputInitOpts("    ", []string{"i"}, false)
	if !strings.Contains(out, "i = i + 1") {
		t.Fatal(out)
	}
	out2 := av.OutputInitOpts("    ", []string{"i"}, true)
	if !strings.Contains(out2, "i++") {
		t.Fatal(out2)
	}
}

func TestGenerateVariableAttributesOption(t *testing.T) {
	opts := Defaults()
	opts.Seed = 9
	opts.VariableAttributes = true
	// force attr always for test by pre-seeding generator after init
	// just ensure Generate doesn't panic
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "func_") {
		t.Fatal("gen")
	}
}
