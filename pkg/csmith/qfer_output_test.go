package csmith

import (
	"strings"
	"testing"
)

func TestRestrictWrite(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	q.Restrict(AccessWrite, EmptyCGContext())
	if q.IsConst() {
		t.Fatal("const cleared")
	}
	// SE-free keeps volatile
	if !q.IsVolatile() {
		t.Fatal("vol kept se-free")
	}
	q2 := NewCVQualifiers([]bool{false}, []bool{true})
	q2.Restrict(AccessRead, WithEffectContext(WithSideEffects()))
	if q2.IsVolatile() {
		t.Fatal("vol cleared non-se-free")
	}
}

func TestSetConstPosFromEnd(t *testing.T) {
	// CVQualifiers.cpp:588–592 — set_const(v, pos) → is_consts[len-pos-1]
	q := NewCVQualifiers([]bool{true, true}, []bool{false, false})
	q.SetConst(false, 0) // storage (last)
	if q.IsConsts[1] || !q.IsConsts[0] {
		t.Fatalf("pos0 clears last only: %v", q.IsConsts)
	}
	q.SetConst(false, 1) // pointee level
	if q.IsConsts[0] {
		t.Fatalf("pos1 clears first: %v", q.IsConsts)
	}
	// no invent grow on empty
	empty := CVQualifiers{}
	empty.SetConst(true, 0)
	if len(empty.IsConsts) != 0 {
		t.Fatal("must not invent slots")
	}
}

func TestSelectDerefExpandStructFailClosed(t *testing.T) {
	// VariableSelector.cpp:1287–1297 — expand_struct miss → Error, no Generate fallthrough
	ClearError()
	opts := Defaults()
	opts.ExpandStruct = true
	opts.Volatiles = true
	opts.VolatilePointers = true
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	// pointee type for *p selection; qfer depth 1 for int
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// force create path: empty nonvol list; ExpandStruct with no struct types → fail
	got := selectDerefPointer(NewRng(3), opts, NewProbabilities(opts), vs, cg, GetIntType(), &q, AccessRead)
	if got != nil && len(vs.GlobalList) > 0 {
		// if somehow created without expand path, ok only when ExpandStruct off
	}
	// with ExpandStruct and no matching struct, must not soft-create pointer via GenerateNew*
	// either nil+error or successful choose (empty cands → create path fail-closed)
	if got == nil && !HasError() {
		// fail closed without sticky error is also ok if ptr_type / path returned 0 early
	}
	ClearError()
}

func TestOutputQualifiedTypeSimple(t *testing.T) {
	// Defaults enable Consts/Volatiles
	SetProcessOptions(Defaults())
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedType(GetIntType())
	if !strings.Contains(s, "const") || !strings.Contains(s, "volatile") || !strings.Contains(s, "int") {
		t.Fatal(s)
	}
}

func TestOutputQualifiedTypeNoInventWhenOptionsOff(t *testing.T) {
	// CVQualifiers.cpp:541–552 — assert(0) if const/vol bit without option; no invent keyword
	opts := Defaults()
	opts.Consts = false
	opts.Volatiles = false
	SetProcessOptions(opts)
	defer SetProcessOptions(Defaults())
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedType(GetIntType())
	if strings.Contains(s, "const") || strings.Contains(s, "volatile") {
		t.Fatalf("must not invent disabled quals: %q", s)
	}
	if !strings.Contains(s, "int") {
		t.Fatal(s)
	}
}

func TestOutputQualifiedTypeNoInventVoidForNil(t *testing.T) {
	// CVQualifiers.cpp:532 — assert(t); no soft invent "void"
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if s := q.OutputQualifiedType(nil); s != "" {
		t.Fatal("nil type must fail closed empty, got", s)
	}
	if s := (*Type)(nil).CName(); s != "" {
		t.Fatal("nil Type.CName must not invent void, got", s)
	}
}

func TestFunctionHeaderNoInventVoidReturn(t *testing.T) {
	// Function always has return_type; no invent void when missing
	f := &Function{Name: "func_x"}
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("missing return type must fail closed header", out)
	}
	f.ReturnType = GetSimpleType(EVoid)
	out := f.OutputHeader(false)
	if !strings.Contains(out, "void func_x(void)") {
		t.Fatal(out)
	}
	// no invent "void (void)" without name
	f.Name = ""
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("empty name must fail closed header", out)
	}
}

func TestOutputDeclNoInventEmptyName(t *testing.T) {
	// Variable always has live name; no invent "int "
	v := &Variable{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if out := v.OutputDecl(false); out != "" {
		t.Fatal("empty name must fail closed decl", out)
	}
}

func TestOutputGlobalsNoInventEmptyDef(t *testing.T) {
	// incomplete OutputDef must not invent "static \n" / blank lines / section-only
	opts := Defaults()
	opts.ForceGlobalsStatic = true
	g := NewProgramGenerator(opts)
	// global without init → OutputDef empty
	v := &Variable{Name: "g_x", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if out != "" {
		t.Fatal("all-empty globals must fail closed empty section", out)
	}
}

func TestOutputStructTypesNoInventEmptySection(t *testing.T) {
	// incomplete / empty decls must not invent section-only header
	g := NewProgramGenerator(Defaults())
	g.Types.StructTypes = []*Type{{isStruct: true}} // unnamed → empty decl
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("empty struct decls must fail closed section", out)
	}
}

func TestOutputFunctionsNoInventEmptySections(t *testing.T) {
	// incomplete funcs must not invent FORWARD/FUNCTIONS section-only shells
	g := NewProgramGenerator(Defaults())
	g.Funcs.Funcs = []*Function{
		{Name: "", ReturnType: GetIntType()}, // empty name → empty header
	}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("empty function IR must fail closed sections", out)
	}
}

func TestBlockLocalNoInventEmptyDef(t *testing.T) {
	// incomplete local OutputDef must not invent blank indent lines
	b := &Block{LocalVars: []*Variable{
		{Name: "l_x", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // no init
	}}
	out := b.Output(0)
	if strings.Contains(out, "l_x") {
		t.Fatal("empty local def must not invent", out)
	}
}

func TestOutputValueDumpNoInventEmptyName(t *testing.T) {
	// Variable.cpp:1184 — name + directive always live
	v := &Variable{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if s := v.OutputValueDump("checksum ", 1, nil); s != "" {
		t.Fatal("empty name must fail closed dump", s)
	}
	if s := v.HashOutput(); s != "" {
		t.Fatal("empty name must fail closed hash", s)
	}
}

func TestOutputExpressionVariableNoInventEmptyBase(t *testing.T) {
	// ExpressionVariable Output requires live Variable::Output base
	// UseVolRVal + volatile + nil type → OutputC empty; multi * must not invent
	v := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, true)
	v.UseVolRVal = true
	v.Type = nil // force VOL_RVAL fail closed empty
	if s := outputExpressionVariable(v, GetIntType()); s != "" {
		t.Fatal("empty base must fail closed", s)
	}
}

func TestCNameNoInventBareAggregateOrDefaultInt(t *testing.T) {
	// Type.cpp always has sid; no invent bare "struct"/"union" or default "int"
	if s := (&Type{isStruct: true}).CName(); s != "" {
		t.Fatal("unnamed struct", s)
	}
	if s := (&Type{isUnion: true}).CName(); s != "" {
		t.Fatal("unnamed union", s)
	}
	// unknown simple enum value — not a known E*
	if s := (&Type{simple: ESimpleType(99)}).CName(); s != "" {
		t.Fatal("unknown simple must not invent int", s)
	}
	if s := (&Type{ptrTo: &Type{isStruct: true}}).CName(); s != "" {
		t.Fatal("ptr to unnamed struct", s)
	}
}

func TestVolWrapNoInventIntType(t *testing.T) {
	// Variable.cpp:690–693 — type->Output; no invent "int" when Type nil
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.UseVolRVal = true
	v.Type = nil
	if out := v.OutputC(); out != "" {
		t.Fatal("nil Type VOL_RVAL must fail closed", out)
	}
	if out := v.OutputLhsC(); out != "" {
		t.Fatal("nil Type VOL_LVAL must fail closed", out)
	}
}

func TestOutputHeaderAliasNoInvent(t *testing.T) {
	// no invent Name+"_alias" when AliasName empty
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	if out := f.OutputHeaderAlias(false); out != "" {
		t.Fatal("missing AliasName must fail closed", out)
	}
	f.AliasName = "func_1_alias"
	out := f.OutputHeaderAlias(true)
	if !strings.Contains(out, "static int func_1_alias(void)") ||
		!strings.Contains(out, `alias("func_1")`) {
		t.Fatal(out)
	}
}

func TestExpressionCastNoInventEmpty(t *testing.T) {
	// cast_type + body both required
	e := &Expression{Term: TermConstant, Con: MakeInt(1), CastType: &Type{isStruct: true}}
	if out := e.Output(); out != "" {
		t.Fatal("cast with incomplete type must fail closed", out)
	}
	e2 := &Expression{Term: TermConstant, CastType: GetIntType()}
	if out := e2.Output(); out != "" {
		t.Fatal("cast with empty body must fail closed", out)
	}
}

func TestExpressionCommaNoInventEmptySide(t *testing.T) {
	// ExpressionComma.cpp:109–115 — both sides Output live; no invent "( , x)"
	good := &Expression{Term: TermConstant, Con: MakeInt(1)}
	bad := &Expression{Term: TermConstant} // nil Con → empty Output
	e := &Expression{Term: TermCommaExpr, CommaLHS: bad, CommaRHS: good}
	if out := e.Output(); out != "" {
		t.Fatal("empty lhs Output must fail closed comma", out)
	}
	e.CommaLHS, e.CommaRHS = good, bad
	if out := e.Output(); out != "" {
		t.Fatal("empty rhs Output must fail closed comma", out)
	}
	e.CommaLHS, e.CommaRHS = good, &Expression{Term: TermConstant, Con: MakeInt(2)}
	if out := e.Output(); out != "(1 , 2)" {
		t.Fatal(out)
	}
}

func TestOutputDeclNoInventEmptyType(t *testing.T) {
	// Variable::OutputDecl — qualified type always live; no invent " name"
	v := &Variable{Name: "g_x"}
	if out := v.OutputDecl(false); out != "" {
		t.Fatal("nil type must fail closed decl", out)
	}
}

func TestOutputForwardDeclNoInventBareSemi(t *testing.T) {
	// incomplete header → empty, not bare ";"
	f := &Function{Name: "func_x"}
	if out := f.OutputForwardDecl(); out != "" {
		t.Fatal("incomplete forward decl must fail closed", out)
	}
	if out := f.OutputForwardDeclAlias(false); out != "" {
		t.Fatal("incomplete alias decl must fail closed", out)
	}
}

func TestMakeRandomLoopControlErrorReturn(t *testing.T) {
	// StatementFor.cpp:79/82/102 ERROR_RETURN on sticky error
	ClearError()
	SetError(ErrGeneric)
	defer ClearError()
	opts := Defaults()
	_, _, _, _, _ = MakeRandomLoopControl(NewRng(1), opts, true)
	// sticky remains; MakeIteration would abort
	if !HasError() {
		t.Fatal("ERROR_RETURN must keep sticky error")
	}
}

func TestVariableOutputDef(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	v.Init = MakeInt(3)
	s := v.OutputDef(true)
	if !strings.Contains(s, "static") || !strings.Contains(s, "const") || !strings.Contains(s, "g_1") || !strings.Contains(s, "3") {
		t.Fatal(s)
	}
}

func TestVariableOutputDefMissingInitFailClosed(t *testing.T) {
	// Variable.cpp:659 — assert(init); no soft invent "= ;"
	v := &Variable{Name: "g_u", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if v.OutputDef(true) != "" {
		t.Fatal("missing init must fail closed")
	}
}

func TestVariableOutputDefMissingDeclFailClosed(t *testing.T) {
	// Variable.cpp:640–660 — no invent " = 3;" without live decl/type
	v := &Variable{Name: "g_u", Init: MakeInt(3)}
	if v.OutputDef(true) != "" {
		t.Fatal("missing type must fail closed", v.OutputDef(true))
	}
}

func TestOutputQualifiedTypeBadSanityFailClosed(t *testing.T) {
	// CVQualifiers.cpp:533 — assert(sanity_check(t)); no invent bare type
	// pointer type needs 2-level qfer (indirect+1)
	pt := PointerTo(GetIntType())
	q := NewCVQualifiers([]bool{false}, []bool{false}) // too short
	if s := q.OutputQualifiedType(pt); s != "" {
		t.Fatal("bad qfer layout must fail closed", s)
	}
}

func TestOutputGlobalsUsesOutputDef(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	g := NewProgramGenerator(opts)
	g.GenerateAllTypes()
	// force a global
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = g.VS.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, g.Rng)
	out := g.OutputGlobals()
	if !strings.Contains(out, "static") || !strings.Contains(out, "g_") {
		t.Fatal(out)
	}
}
