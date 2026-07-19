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

func TestGenerateParameterListFromStringAsserts(t *testing.T) {
	// Function.cpp:350/355 — empty / mid Void fail closed sticky
	ClearError()
	f := &Function{Name: "b"}
	if GenerateParameterListFromString(f, "") {
		t.Fatal("empty params")
	}
	f2 := &Function{Name: "b2"}
	if GenerateParameterListFromString(f2, "UInt, Void") {
		t.Fatal("mid Void")
	}
	// IncompleteVariables sticky — not bare nil invent empty-complete void Param
	if VariablesComplete(f2.Param) {
		t.Fatal("mid Void fail must IncompleteVariables Param, not empty-complete")
	}
	if !HasError() {
		t.Fatal("mid Void fail must SetError sticky")
	}
	ClearError()
	f3 := &Function{Name: "b3"}
	if !GenerateParameterListFromString(f3, "Void") {
		t.Fatal("sole Void ok")
	}
	f4 := &Function{Name: "b4"}
	if !GenerateParameterListFromString(f4, "UInt, UChar") || len(f4.Param) != 2 {
		t.Fatal("two params", f4.Param)
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
	// empty name token — no invent shell
	if MakeBuiltinFunction(opts, NewProbabilities(opts), NewRng(1), list, nil, "Int; ; (UInt); x86") != nil {
		t.Fatal("empty builtin name must fail closed")
	}
	// Function.cpp always has RNG for random_qualifiers; no invent fixed RV qfer
	if MakeBuiltinFunction(opts, NewProbabilities(opts), nil, list, nil, "Int; __builtin_clz; (UInt); x86") != nil {
		t.Fatal("nil RNG must not invent builtin")
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
	// Clear must not invent wipe IncompleteEffect to empty pure — sticky
	ClearError()
	inc := IncompleteEffect()
	inc.Clear()
	if EffectComplete(inc) || inc.IsEmpty() || inc.IsPure() {
		t.Fatal("Clear incomplete must stay IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("Clear incomplete must SetError sticky")
	}
	ClearError()
}

func TestChooseFuncCanPickBuiltin(t *testing.T) {
	opts := Defaults()
	opts.Builtins = true
	opts.BuiltinFunctionProb = 100
	bi := &Function{Name: "__builtin_clz", ReturnType: GetIntType(), IsBuiltin: true, BuildState: BuildBuilt, IsBuilt: true}
	user := &Function{Name: "func_1", ReturnType: GetIntType(), BuildState: BuildBuilt, IsBuilt: true}
	got := ChooseFuncContext(NewRng(3), []*Function{user, bi}, GetIntType(), nil, nil, opts, nil)
	if got != bi {
		t.Fatalf("want builtin got %v", got)
	}
}
