package csmith

import "testing"

func TestChooseFuncSkipsUnbuilt(t *testing.T) {
	built := &Function{Name: "func_1", ReturnType: GetIntType(), IsBuilt: true}
	unbuilt := &Function{Name: "func_2", ReturnType: GetIntType(), IsBuilt: false}
	funcs := []*Function{built, unbuilt}
	r := NewRng(2)
	for i := 0; i < 20; i++ {
		f := ChooseFunc(r, funcs, GetIntType(), nil)
		if f == unbuilt {
			t.Fatal("must not choose unbuilt")
		}
		if f != built && f != nil {
			t.Fatalf("%v", f)
		}
	}
	// only unbuilt → nil
	if ChooseFunc(NewRng(1), []*Function{unbuilt}, GetIntType(), nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestArrayNoLoopInitializer(t *testing.T) {
	opts := Defaults()
	// global → true
	g := CreateArrayVariable(NewRng(2), opts, nil, "g_1", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if g == nil || !g.NoLoopInitializer() {
		t.Fatal("global must no-loop")
	}
	// local with no multi inits: force empty InitValues
	blk := &Block{}
	loc := CreateArrayVariable(NewRng(3), opts, blk, "l_1", GetIntType(), MakeInt(1), NewCVQualifiers([]bool{false}, []bool{false}))
	if loc == nil {
		t.Fatal("nil local")
	}
	loc.InitValues = nil
	loc.ArrayInits = nil
	if loc.NoLoopInitializer() {
		// local non-const simple without multi inits → false
		t.Fatal("local should allow loop init")
	}
	out := loc.OutputInit("    ", []string{"i"})
	if out == "" || !contains(out, "for") || !contains(out, "for (i = 0") {
		t.Fatal(out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
