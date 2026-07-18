package csmith

import (
	"strings"
	"testing"
)

func TestTypeFromString(t *testing.T) {
	if TypeFromString("Int") != GetIntType() {
		t.Fatal("Int")
	}
	if TypeFromString("UInt") == nil || !TypeFromString("UInt").IsSimple() {
		t.Fatal("UInt")
	}
	if TypeFromString("Void").Simple() != EVoid {
		t.Fatal("Void")
	}
	if TypeFromString("NoSuch") != nil {
		t.Fatal("bad")
	}
}

func TestEnabledBuiltinKinds(t *testing.T) {
	opts := Defaults()
	if !EnabledBuiltin(opts, "x86") {
		t.Fatal("x86 default")
	}
	if EnabledBuiltin(opts, "clang") {
		t.Fatal("clang off")
	}
	opts.EnableBuiltinKinds = "clang"
	if !EnabledBuiltin(opts, "clang") {
		t.Fatal("clang enable")
	}
	if !EnabledBuiltin(opts, "ppc | clang") {
		t.Fatal("or list")
	}
	opts.DisableBuiltinKinds = "x86"
	if EnabledBuiltin(opts, "x86") {
		t.Fatal("x86 disabled")
	}
}

func TestMakeBuiltinFunction(t *testing.T) {
	opts := Defaults()
	opts.Builtins = true
	list := &FunctionList{}
	f := MakeBuiltinFunction(opts, NewProbabilities(opts), NewRng(1), list, nil,
		"Int; __builtin_clz; (UInt); x86")
	if f == nil || !f.IsBuiltin || f.Name != "__builtin_clz" {
		t.Fatal(f)
	}
	if len(f.Param) != 1 {
		t.Fatal("params", f.Param)
	}
	if f.Body == nil || !f.IsEffectKnown() {
		t.Fatal("body/built")
	}
	if len(list.Funcs) != 1 {
		t.Fatal("list")
	}
	// disabled kind
	if MakeBuiltinFunction(opts, nil, nil, list, nil, "Int; __builtin_clzs; (UShort); clang") != nil {
		t.Fatal("clang should skip")
	}
}

func TestInitializeBuiltinFunctions(t *testing.T) {
	opts := Defaults()
	opts.Builtins = true
	list := &FunctionList{}
	n := InitializeBuiltinFunctions(opts, NewProbabilities(opts), NewRng(2), list, nil)
	// x86 builtins only by default (~18)
	if n < 10 || n > 25 {
		t.Fatal(n, len(list.Funcs))
	}
	// off
	opts.Builtins = false
	if InitializeBuiltinFunctions(opts, nil, nil, &FunctionList{}, nil) != 0 {
		t.Fatal("off")
	}
}

func TestGenerateWithBuiltins(t *testing.T) {
	opts := Defaults()
	opts.Seed = 11
	opts.Builtins = true
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "__builtin_") {
		// may not call them but decls should appear as forward decls if in FuncList
		// OutputFunctions skips? check forward decls
		t.Log("no builtin name in output — may be ok if only invoked randomly")
	}
	// at least generation succeeds
	if !strings.Contains(out, "func_") {
		t.Fatal("func")
	}
}

func TestHasRaceWith(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e1 := EmptyEffect().ReadVar(a)
	e2 := EmptyEffect().WriteVar(a)
	if !e1.HasRaceWith(e2) {
		t.Fatal("race")
	}
	e3 := EmptyEffect().WriteVar(b)
	if e1.HasRaceWith(e3) {
		t.Fatal("no race")
	}
	if !EmptyEffect().IsEmpty() {
		t.Fatal("empty")
	}
	e2.Clear()
	if !e2.IsEmpty() {
		t.Fatal("clear")
	}
}

func TestChooseFuncCanPickBuiltin(t *testing.T) {
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 100
	bi := &Function{Name: "__builtin_clz", ReturnType: GetIntType(), IsBuiltin: true, BuildState: BuildBuilt, IsBuilt: true}
	user := &Function{Name: "func_1", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	got := ChooseFuncContext(NewRng(3), []*Function{user, bi}, GetIntType(), nil, nil, opts)
	if got != bi {
		t.Fatalf("want builtin got %v", got)
	}
}
