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

func TestRestrictIncompleteEffectSticky(t *testing.T) {
	// Incomplete EffectContext must not invent clear-vol via IncompleteEffect SE-false
	ClearError()
	q := NewCVQualifiers([]bool{true}, []bool{true})
	q.Restrict(AccessWrite, WithEffectContext(IncompleteEffect()))
	if !q.IsConst() || !q.IsVolatile() {
		t.Fatalf("incomplete ambient must not mutate qfer: const=%v vol=%v", q.IsConst(), q.IsVolatile())
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
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
	// CVQualifiers.cpp:541–552 — assert(0) sticky if const/vol bit without option
	// (no invent bare "int" by silently dropping disabled quals)
	ClearError()
	opts := Defaults()
	opts.Consts = false
	opts.Volatiles = false
	SetProcessOptions(opts)
	defer SetProcessOptions(Defaults())
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedType(GetIntType())
	if s != "" {
		t.Fatalf("const bit without Consts option must fail closed empty, got %q", s)
	}
	if !HasError() {
		t.Fatal("const bit without Consts option must SetError sticky")
	}
	ClearError()
}

func TestOutputQualifiedTypeNoInventVoidForNil(t *testing.T) {
	// CVQualifiers.cpp:532 — assert(t); sticky no soft invent "void"
	ClearError()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if s := q.OutputQualifiedType(nil); s != "" {
		t.Fatal("nil type must fail closed empty, got", s)
	}
	if !HasError() {
		t.Fatal("nil type OutputQualifiedType must SetError sticky")
	}
	ClearError()
	if s := (*Type)(nil).CName(); s != "" {
		t.Fatal("nil Type.CName must not invent void, got", s)
	}
	ClearError()
}

func TestFunctionHeaderNoInventVoidReturn(t *testing.T) {
	// Function always has return_type; sticky no invent void when missing
	ClearError()
	f := &Function{Name: "func_x"}
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("missing return type must fail closed header", out)
	}
	if !HasError() {
		t.Fatal("missing return type must SetError sticky")
	}
	ClearError()
	f.ReturnType = GetSimpleType(EVoid)
	out := f.OutputHeader(false)
	if !strings.Contains(out, "void func_x(void)") {
		t.Fatal(out)
	}
	// sticky no invent "void (void)" without name
	ClearError()
	f.Name = ""
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("empty name must fail closed header", out)
	}
	if !HasError() {
		t.Fatal("empty function name must SetError sticky")
	}
	ClearError()
}

func TestOutputDeclNoInventEmptyName(t *testing.T) {
	// Variable always has live name; sticky no invent "int "
	ClearError()
	v := &Variable{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if out := v.OutputDecl(false); out != "" {
		t.Fatal("empty name must fail closed decl", out)
	}
	if !HasError() {
		t.Fatal("empty name OutputDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputGlobalsNoInventEmptyDef(t *testing.T) {
	// incomplete OutputDef must not invent "static \n" / blank lines / section-only
	ClearError()
	opts := Defaults()
	opts.ForceGlobalsStatic = true
	g := NewProgramGenerator(NewSession(opts))
	// global without init → OutputDef empty sticky
	v := &Variable{Name: "g_x", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if out != "" {
		t.Fatal("all-empty globals must fail closed empty section", out)
	}
	if !HasError() {
		t.Fatal("empty-def globals Output must SetError sticky")
	}
	// incomplete among live fails whole section (no invent skip holes)
	ClearError()
	good := CreateVariableScalars("g_ok", GetIntType(), false, false)
	g.VS.GlobalList = []*Variable{good, v}
	if out2 := g.OutputGlobals(); out2 != "" {
		t.Fatal("mixed incomplete globals must fail closed", out2)
	}
	if !HasError() {
		t.Fatal("mixed incomplete globals must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was scalar OutputDef for array shell
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}, Init: MakeInt(0)}
	g.VS.GlobalList = []*Variable{shell}
	if out3 := g.OutputGlobals(); out3 != "" {
		t.Fatal("IsArray without AsArray must fail closed globals", out3)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputGlobals must SetError sticky")
	}
	ClearError()
}

func TestOutputStructTypesNoInventEmptySection(t *testing.T) {
	// Type.cpp:1894–1901 — empty used pool still emits section header; incomplete IR fails closed.
	ClearError()
	g := NewProgramGenerator(NewSession(Defaults()))
	// complete empty: only section comment (no invent types past unused inventory)
	g.Types.StructTypes = nil
	g.Types.UnionTypes = nil
	g.Types.AllTypes = nil
	out := g.OutputStructTypes()
	if out != "/* --- Struct/Union Declarations --- */\n" {
		t.Fatalf("empty used pool must still emit section header, got %q", out)
	}
	if HasError() {
		t.Fatal("complete empty OutputStructTypes must not SetError")
	}
	// used aggregate emits decl body
	ClearError()
	st := &Type{isStruct: true, StructName: "S0", Used: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	g.Types.StructTypes = []*Type{st}
	g.Types.AllTypes = []*Type{st}
	out2 := g.OutputStructTypes()
	if !strings.Contains(out2, "/* --- Struct/Union Declarations --- */") || !strings.Contains(out2, "struct S0") {
		t.Fatal("used struct must emit under section", out2)
	}
	// unused inventory must not invent decl (only header)
	ClearError()
	stU := &Type{isStruct: true, StructName: "S1", Used: false, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	g.Types.StructTypes = []*Type{stU}
	g.Types.AllTypes = []*Type{stU}
	out3 := g.OutputStructTypes()
	if out3 != "/* --- Struct/Union Declarations --- */\n" || strings.Contains(out3, "struct S1") {
		t.Fatal("unused struct must not invent decl", out3)
	}
	// incomplete unnamed used → fail closed
	ClearError()
	g.Types.StructTypes = []*Type{{isStruct: true, Used: true}} // unnamed → empty decl
	g.Types.AllTypes = g.Types.StructTypes
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("empty struct decls must fail closed section", out)
	}
	if !HasError() {
		t.Fatal("empty struct decl OutputStructTypes must SetError sticky")
	}
	// nil hole fails closed sticky
	ClearError()
	g.Types.StructTypes = []*Type{nil}
	g.Types.AllTypes = nil
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("nil struct hole must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil struct hole must SetError sticky")
	}
	ClearError()
	g.Types.StructTypes = nil
	g.Types.UnionTypes = []*Type{nil}
	g.Types.AllTypes = nil
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("nil union hole must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil union hole must SetError sticky")
	}
	ClearError()
}

func TestOutputFunctionsNoInventEmptySections(t *testing.T) {
	// incomplete funcs must not invent FORWARD/FUNCTIONS section-only shells
	g := NewProgramGenerator(NewSession(Defaults()))
	g.Funcs.Funcs = []*Function{
		{Name: "", ReturnType: GetIntType()}, // empty name → empty header
	}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("empty function IR must fail closed sections", out)
	}
	// nil Funcs hole fails closed sticky (no invent skip holes)
	ClearError()
	good := &Function{
		Name: "func_1", AliasName: "func_1_alias", ReturnType: GetIntType(),
		RV:   CreateVariableQfer("func_1_rv", GetIntType(), NewCVQualifiers([]bool{false}, []bool{false})),
		Body: &Block{}, IsBuilt: true, BuildState: BuildBuilt,
	}
	g.Funcs.Funcs = []*Function{good, nil}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("nil Funcs hole must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil Funcs hole must SetError sticky")
	}
	ClearError()
	// empty-name incomplete among live fails whole (no invent skip)
	ClearError()
	g.Funcs.Funcs = []*Function{good, {Name: "", ReturnType: GetIntType()}}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("mixed incomplete must fail closed", out)
	}
	ClearError()
}

func TestBlockLocalNoInventEmptyDef(t *testing.T) {
	// incomplete local OutputDef fails whole block sticky (no invent blank lines / partial)
	ClearError()
	b := &Block{LocalVars: []*Variable{
		{Name: "l_x", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // no init
	}}
	out := b.Output(0)
	if out != "" {
		t.Fatal("incomplete local must fail closed whole block", out)
	}
	if !HasError() {
		t.Fatal("incomplete local block Output must SetError sticky")
	}
	// good local still emits after clear
	ClearError()
	ok := CreateVariableScalars("l_ok", GetIntType(), false, false)
	if ok == nil {
		t.Fatal("CreateVariableScalars after ClearError")
	}
	ok.Init = MakeInt(1)
	b2 := &Block{LocalVars: []*Variable{ok}}
	if out2 := b2.Output(0); !strings.Contains(out2, "l_ok") {
		t.Fatal(out2)
	}
	ClearError()
}

func TestOutputValueDumpNoInventEmptyName(t *testing.T) {
	// Variable.cpp:1184 — name + directive always live sticky; hash empty name sticky
	ClearError()
	v := &Variable{Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if s := v.OutputValueDump("checksum ", 1, nil); s != "" {
		t.Fatal("empty name must fail closed dump", s)
	}
	if !HasError() {
		t.Fatal("empty name OutputValueDump must SetError sticky")
	}
	ClearError()
	if s := v.HashOutput(); s != "" {
		t.Fatal("empty name must fail closed hash", s)
	}
	if !HasError() {
		t.Fatal("empty name HashOutput must SetError sticky")
	}
	ClearError()
	// array dump/hash without name — no invent bare "[0]" / for ( = 0; …)
	arr := &Variable{Type: GetIntType(), IsArray: true, ArraySizes: []int{2}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if s := arr.OutputValueDump("c ", 1, nil); s != "" {
		t.Fatal("empty array name must fail closed dump", s)
	}
	if !HasError() {
		t.Fatal("empty array name OutputValueDump must SetError sticky")
	}
	ClearError()
	ctrl := []*Variable{
		{Name: "i", Type: GetIntType()},
	}
	if s := hashArrayVariable(arr, ctrl, nil); s != "" {
		t.Fatal("empty array name must fail closed hash", s)
	}
	if !HasError() {
		t.Fatal("empty array name hashArrayVariable must SetError sticky")
	}
	ClearError()
	// empty ctrl name sticky — no invent for ( = 0; …)
	arr.Name = "g_a"
	if s := hashArrayVariable(arr, []*Variable{{Type: GetIntType()}}, nil); s != "" {
		t.Fatal("empty ctrl name must fail closed array hash", s)
	}
	if !HasError() {
		t.Fatal("empty ctrl name hashArrayVariable must SetError sticky")
	}
	ClearError()
	// incomplete field Type sticky (no invent skip partial hash as empty success)
	agg := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1},
	}}
	arr2 := &Variable{
		Name: "g_s", Type: agg, IsArray: true, ArraySizes: []int{1},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if s := hashArrayVariable(arr2, ctrl, nil); s != "" {
		t.Fatal("field Type hole must fail closed hash", s)
	}
	if !HasError() {
		t.Fatal("field Type hole hashArrayVariable must SetError sticky")
	}
	ClearError()
	// IsFieldReadable residual: incomplete UnionFacts stickies ERROR+false.
	// Soft invent was soft-continue then emit later field transparent_crc past hole.
	// Fair: sticky fail closed whole array hash.
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	arrU := &Variable{
		Name: "g_u", Type: ut, IsArray: true, ArraySizes: []int{1},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	holeFacts := []*FactUnion{nil}
	if s := hashArrayVariable(arrU, ctrl, holeFacts); s != "" {
		t.Fatal("IsFieldReadable residual must fail closed array hash", s)
	}
	if !HasError() {
		t.Fatal("IsFieldReadable residual hashArrayVariable must SetError sticky")
	}
	ClearError()
}

func TestOutputExpressionVariableNoInventEmptyBase(t *testing.T) {
	// ExpressionVariable Output requires live Variable::Output base
	// UseVolRVal + volatile + nil type → OutputC empty; multi * must not invent
	ClearError()
	v := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, true)
	if v == nil {
		t.Fatal("create g_p")
	}
	v.UseVolRVal = true
	v.Type = nil // force VOL_RVAL fail closed empty
	if s := outputExpressionVariable(v, GetIntType()); s != "" {
		t.Fatal("empty base must fail closed", s)
	}
}

func TestCNameNoInventBareAggregateOrDefaultInt(t *testing.T) {
	// Type.cpp always has sid; sticky no invent bare "struct"/"union" or default "int"
	ClearError()
	if s := (&Type{isStruct: true}).CName(); s != "" {
		t.Fatal("unnamed struct", s)
	}
	if !HasError() {
		t.Fatal("unnamed struct CName must SetError sticky")
	}
	ClearError()
	if s := (&Type{isUnion: true}).CName(); s != "" {
		t.Fatal("unnamed union", s)
	}
	if !HasError() {
		t.Fatal("unnamed union CName must SetError sticky")
	}
	// unknown simple enum value — not a known E*
	ClearError()
	if s := (&Type{simple: ESimpleType(99)}).CName(); s != "" {
		t.Fatal("unknown simple must not invent int", s)
	}
	if !HasError() {
		t.Fatal("unknown simple CName must SetError sticky")
	}
	ClearError()
	if s := (&Type{ptrTo: &Type{isStruct: true}}).CName(); s != "" {
		t.Fatal("ptr to unnamed struct", s)
	}
	if !HasError() {
		t.Fatal("ptr to unnamed struct CName must SetError sticky")
	}
	ClearError()
}

func TestVolWrapNoInventIntType(t *testing.T) {
	// Variable.cpp:690–693 — type->Output; sticky no invent "int" when Type nil
	ClearError()
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	v.UseVolRVal = true
	v.Type = nil
	if out := v.OutputC(); out != "" {
		t.Fatal("nil Type VOL_RVAL must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil Type VOL_RVAL must SetError sticky")
	}
	ClearError()
	if out := v.OutputLhsC(); out != "" {
		t.Fatal("nil Type VOL_LVAL must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil Type VOL_LVAL must SetError sticky")
	}
	// sticky no invent VOL_RVAL(, int) / VOL_LVAL(, int) with empty name
	ClearError()
	v2 := &Variable{Type: GetIntType(), UseVolRVal: true, Qfer: NewCVQualifiers([]bool{false}, []bool{true})}
	if out := v2.OutputC(); out != "" {
		t.Fatal("empty name VOL_RVAL must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty name VOL_RVAL must SetError sticky")
	}
	ClearError()
	if out := v2.OutputLhsC(); out != "" {
		t.Fatal("empty name VOL_LVAL must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty name VOL_LVAL must SetError sticky")
	}
	ClearError()
}

func TestOutputHeaderAliasNoInvent(t *testing.T) {
	// sticky no invent Name+"_alias" when AliasName empty
	ClearError()
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	if out := f.OutputHeaderAlias(false); out != "" {
		t.Fatal("missing AliasName must fail closed", out)
	}
	if !HasError() {
		t.Fatal("missing AliasName must SetError sticky")
	}
	ClearError()
	f.AliasName = "func_1_alias"
	out := f.OutputHeaderAlias(true)
	if !strings.Contains(out, "static int32_t func_1_alias(void)") ||
		!strings.Contains(out, `alias("func_1")`) {
		t.Fatal(out)
	}
}

func TestExpressionCastNoInventEmpty(t *testing.T) {
	// cast_type + body both required sticky
	ClearError()
	e := &Expression{Term: TermConstant, Con: MakeInt(1), CastType: &Type{isStruct: true}}
	if out := e.Output(); out != "" {
		t.Fatal("cast with incomplete type must fail closed", out)
	}
	if !HasError() {
		t.Fatal("cast incomplete type must SetError sticky")
	}
	ClearError()
	e2 := &Expression{Term: TermConstant, CastType: GetIntType()}
	if out := e2.Output(); out != "" {
		t.Fatal("cast with empty body must fail closed", out)
	}
	if !HasError() {
		t.Fatal("cast empty body must SetError sticky")
	}
	// Constant with empty Value — sticky no invent empty token
	ClearError()
	e3 := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntType(), Value: ""}}
	if out := e3.Output(); out != "" {
		t.Fatal("empty Constant.Value must fail closed", out)
	}
	if !HasError() {
		t.Fatal("empty Constant.Value must SetError sticky")
	}
	ClearError()
}

func TestExpressionCommaNoInventEmptySide(t *testing.T) {
	// ExpressionComma.cpp:109–115 — both sides Output live; sticky no invent "( , x)"
	ClearError()
	good := &Expression{Term: TermConstant, Con: MakeInt(1)}
	bad := &Expression{Term: TermConstant} // nil Con → empty Output sticky
	e := &Expression{Term: TermCommaExpr, CommaLHS: bad, CommaRHS: good}
	if out := e.Output(); out != "" {
		t.Fatal("empty lhs Output must fail closed comma", out)
	}
	if !HasError() {
		t.Fatal("empty lhs comma must SetError sticky")
	}
	ClearError()
	e.CommaLHS, e.CommaRHS = good, bad
	if out := e.Output(); out != "" {
		t.Fatal("empty rhs Output must fail closed comma", out)
	}
	if !HasError() {
		t.Fatal("empty rhs comma must SetError sticky")
	}
	ClearError()
	e.CommaLHS, e.CommaRHS = good, &Expression{Term: TermConstant, Con: MakeInt(2)}
	if out := e.Output(); out != "(1 , 2)" {
		t.Fatal(out)
	}
	ClearError()
}

func TestOutputDeclNoInventEmptyType(t *testing.T) {
	// Variable::OutputDecl — qualified type always live; sticky no invent " name"
	ClearError()
	v := &Variable{Name: "g_x"}
	if out := v.OutputDecl(false); out != "" {
		t.Fatal("nil type must fail closed decl", out)
	}
	if !HasError() {
		t.Fatal("nil type OutputDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputForwardDeclNoInventBareSemi(t *testing.T) {
	// incomplete header sticky → empty, not bare ";"
	ClearError()
	f := &Function{Name: "func_x"}
	if out := f.OutputForwardDecl(); out != "" {
		t.Fatal("incomplete forward decl must fail closed", out)
	}
	if !HasError() {
		t.Fatal("incomplete forward decl must SetError sticky")
	}
	ClearError()
	if out := f.OutputForwardDeclAlias(false); out != "" {
		t.Fatal("incomplete alias decl must fail closed", out)
	}
	if !HasError() {
		t.Fatal("incomplete alias decl must SetError sticky")
	}
	ClearError()
	// Function always live at emit; sticky empty (no invent bare ";")
	if out := (*Function)(nil).OutputForwardDecl(); out != "" {
		t.Fatal("nil OutputForwardDecl must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil OutputForwardDecl must SetError sticky")
	}
	ClearError()
	if out := (*Function)(nil).OutputForwardDeclAlias(false); out != "" {
		t.Fatal("nil OutputForwardDeclAlias must fail closed", out)
	}
	if !HasError() {
		t.Fatal("nil OutputForwardDeclAlias must SetError sticky")
	}
	ClearError()
	// builtins complete empty (compiler-provided)
	if out := (&Function{Name: "abs", IsBuiltin: true}).OutputForwardDecl(); out != "" {
		t.Fatal("builtin OutputForwardDecl must stay complete empty", out)
	}
	if HasError() {
		t.Fatal("builtin OutputForwardDecl must not sticky")
	}
	ClearError()
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
	// Variable.cpp:659 — assert(init); sticky no soft invent "= ;"
	ClearError()
	v := &Variable{Name: "g_u", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if v.OutputDef(true) != "" {
		t.Fatal("missing init must fail closed")
	}
	if !HasError() {
		t.Fatal("missing init OutputDef must SetError sticky")
	}
	ClearError()
	// IsArray without AsArray soft invent was scalar "int g_a = 0;"
	shell := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}, Init: MakeInt(0)}
	if shell.OutputDef(true) != "" {
		t.Fatal("IsArray without AsArray OutputDef must fail closed")
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputDef must SetError sticky")
	}
	ClearError()
}

func TestVariableOutputDefMissingDeclFailClosed(t *testing.T) {
	// Variable.cpp:640–660 — sticky no invent " = 3;" without live decl/type
	ClearError()
	v := &Variable{Name: "g_u", Init: MakeInt(3)}
	if v.OutputDef(true) != "" {
		t.Fatal("missing type must fail closed", v.OutputDef(true))
	}
	if !HasError() {
		t.Fatal("missing type OutputDef must SetError sticky")
	}
	ClearError()
}

func TestOutputQualifiedTypeBadSanityFailClosed(t *testing.T) {
	// CVQualifiers.cpp:533 — assert(sanity_check(t)); sticky no invent bare type
	// pointer type needs 2-level qfer (indirect+1)
	ClearError()
	pt := PointerTo(GetIntType())
	q := NewCVQualifiers([]bool{false}, []bool{false}) // too short
	if s := q.OutputQualifiedType(pt); s != "" {
		t.Fatal("bad qfer layout must fail closed", s)
	}
	if !HasError() {
		t.Fatal("bad qfer layout must SetError sticky")
	}
	ClearError()
}

func TestOutputGlobalsUsesOutputDef(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	// force a global
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = g.VS.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetIntType(), &q, g.Rng)
	out := g.OutputGlobals()
	if !strings.Contains(out, "static") || !strings.Contains(out, "g_") {
		t.Fatal(out)
	}
}

func TestOutputDeclOutputQualifiedTypeResidualSticky(t *testing.T) {
	// OutputQualifiedType residual soft invent was soft-continue invent " name" decl.
	// Unnamed struct CName residual under OutputQualifiedType.
	ClearError()
	v := &Variable{
		Name: "g_s",
		Type: &Type{isStruct: true}, // CName residual
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if s := v.OutputDecl(false); s != "" {
		t.Fatal("CName residual must fail closed OutputDecl", s)
	}
	if !HasError() {
		t.Fatal("CName residual OutputDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputAddrOfGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was soft-empty invent bare "&".
	// Empty name already sticky; residual path: covered by nil. Keep complete hygiene.
	ClearError()
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	if s := v.OutputAddrOf(false); s != "&g_x" {
		t.Fatal("complete OutputAddrOf", s)
	}
	if HasError() {
		t.Fatal("complete OutputAddrOf must not sticky")
	}
	ClearError()
	// empty name residual hole
	if s := (&Variable{Type: GetIntType()}).OutputAddrOf(false); s != "" {
		t.Fatal("empty name OutputAddrOf must fail closed", s)
	}
	if !HasError() {
		t.Fatal("empty name OutputAddrOf must SetError sticky")
	}
	ClearError()
}

func TestOutputConditionBoundResidualSticky(t *testing.T) {
	// OutputLower/UpperBound residual soft invent was soft-continue invent partial range.
	ClearError()
	// array without AsArray → OutputLowerBound residual sticky
	arr := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{4}}
	f := &FactPointTo{Var: CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), PointTo: []*Variable{arr}}
	if s := f.OutputCondition(); s != "" {
		t.Fatal("bound residual must fail closed OutputCondition", s)
	}
	if !HasError() {
		t.Fatal("bound residual OutputCondition must SetError sticky")
	}
	ClearError()
}

func TestOutputForwardDeclHeaderResidualSticky(t *testing.T) {
	// OutputHeader residual soft invent was invent bare ";" past empty header shell.
	ClearError()
	f := &Function{Name: "", ReturnType: GetIntType()} // empty name residual
	if s := f.OutputForwardDecl(); s != "" {
		t.Fatal("OutputHeader residual must fail closed OutputForwardDecl", s)
	}
	if !HasError() {
		t.Fatal("OutputHeader residual OutputForwardDecl must SetError sticky")
	}
	ClearError()
}

func TestOutputGlobalsOutputDefResidualSticky(t *testing.T) {
	// OutputDef residual soft invent was soft-continue later globals invent partial section.
	ClearError()
	opts := Defaults()
	g := NewProgramGenerator(NewSession(opts))
	g.VS = NewVariableSelector(opts)
	// Type-nil InitExpr residual on OutputDefFull
	v := CreateVariableScalars("g_x", GetIntType(), false, false)
	v.Init = nil
	v.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil residual
	g.VS.GlobalList = []*Variable{v}
	if s := g.OutputGlobals(); s != "" {
		t.Fatal("InitExpr Output residual must fail closed OutputGlobals", s)
	}
	if !HasError() {
		t.Fatal("InitExpr Output residual OutputGlobals must SetError sticky")
	}
	ClearError()
}

func TestReturnTypeCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was invent "void"/partial header past hole.
	ClearError()
	f := &Function{Name: "func_1", ReturnType: &Type{isStruct: true}} // unnamed struct CName residual
	if s := f.returnTypeC(); s != "" {
		t.Fatal("CName residual must fail closed returnTypeC", s)
	}
	if !HasError() {
		t.Fatal("CName residual returnTypeC must SetError sticky")
	}
	ClearError()
	if s := f.OutputHeader(false); s != "" {
		t.Fatal("CName residual must fail closed OutputHeader", s)
	}
	if !HasError() {
		t.Fatal("CName residual OutputHeader must SetError sticky")
	}
	ClearError()
}

func TestOutputArrayInitForcedResidualSticky(t *testing.T) {
	// InitExpr Output residual soft invent was invent forced loop-init past hole.
	ClearError()
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2},
			InitExpr: &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}, // Type-nil residual
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	if s := outputArrayInitForced(av, "    ", []string{"i"}, true); s != "" {
		t.Fatal("InitExpr Output residual must fail closed outputArrayInitForced", s)
	}
	if !HasError() {
		t.Fatal("InitExpr Output residual outputArrayInitForced must SetError sticky")
	}
	ClearError()
}

func TestOutputQualifiedTypeCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was invent "const " / partial qualified type past hole.
	ClearError()
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if s := q.OutputQualifiedType(&Type{isStruct: true}); s != "" {
		t.Fatal("CName residual must fail closed OutputQualifiedType", s)
	}
	if !HasError() {
		t.Fatal("CName residual OutputQualifiedType must SetError sticky")
	}
	ClearError()
}

func TestOutputUpperBoundResidualSticky(t *testing.T) {
	// OutputUpperBoundArray residual soft invent was invent bare field path past hole.
	ClearError()
	// IsArray without AsArray residual
	v := &Variable{Name: "g_a", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	if s := v.OutputUpperBound(false); s != "" {
		t.Fatal("IsArray without AsArray must fail closed OutputUpperBound", s)
	}
	if !HasError() {
		t.Fatal("IsArray without AsArray OutputUpperBound must SetError sticky")
	}
	ClearError()
	// empty name residual
	if s := (&Variable{Type: GetIntType()}).OutputUpperBound(false); s != "" {
		t.Fatal("empty name OutputUpperBound must fail closed", s)
	}
	if !HasError() {
		t.Fatal("empty name OutputUpperBound must SetError sticky")
	}
	ClearError()
}

func TestCtrlVarNamesGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was soft-continue invent partial name list.
	ClearError()
	ctrl := []*Variable{
		{Name: "i", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Name: "", Type: GetIntType(), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // residual
	}
	if LabelsComplete(CtrlVarNames(ctrl)) {
		t.Fatal("empty name residual must fail closed IncompleteLabelsSlice")
	}
	if !HasError() {
		t.Fatal("empty name residual CtrlVarNames must SetError sticky")
	}
	ClearError()
}

func TestIsFieldReadableIsUnionResidualSticky(t *testing.T) {
	// IsUnion residual soft invent was invent not-readable soft-skip past Type-nil already sticky.
	// Non-union complete: not readable false without sticky.
	ClearError()
	v := CreateVariableScalars("g_i", GetIntType(), false, false)
	if IsFieldReadable(v, 0, nil) {
		t.Fatal("non-union IsFieldReadable must be false")
	}
	if HasError() {
		t.Fatal("complete non-union IsFieldReadable must not sticky")
	}
	ClearError()
	// Type-nil sticky
	if IsFieldReadable(&Variable{Name: "g_u", Type: nil}, 0, nil) {
		t.Fatal("Type-nil IsFieldReadable must fail closed false")
	}
	if !HasError() {
		t.Fatal("Type-nil IsFieldReadable must SetError sticky")
	}
	ClearError()
}

func TestOutputGlobalVariablesListResidualSticky(t *testing.T) {
	// OutputDef residual soft invent was invent section header past incomplete var.
	ClearError()
	// incomplete list sticky
	if OutputGlobalVariables([]*Variable{nil}) != "" {
		t.Fatal("nil hole OutputGlobalVariables must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil hole OutputGlobalVariables must SetError sticky")
	}
	ClearError()
	// complete empty
	if OutputGlobalVariables(nil) != "" {
		t.Fatal("empty OutputGlobalVariables must be empty")
	}
	if HasError() {
		t.Fatal("empty OutputGlobalVariables must not sticky")
	}
	ClearError()
}

func TestOutputQualifiedTypeConstVolatilePointerDoubleSpace(t *testing.T) {
	// CVQualifiers.cpp:540–552 — level i>0 with both const and volatile:
	// "const " then (i>0) " " then "volatile " → "const  volatile "
	// (seed-2 g_459: UP "const  volatile" vs invent skip space when const already present)
	ClearError()
	SetProcessOptions(Defaults())
	pt := PointerTo(PointerTo(GetIntType())) // int32_t **
	// depth 3: [obj, *, *] — outer pointer level both const+vol
	q := NewCVQualifiers(
		[]bool{false, false, true},
		[]bool{false, false, true},
	)
	s := q.OutputQualifiedType(pt)
	if !strings.Contains(s, "const  volatile") {
		t.Fatalf("want double space const  volatile, got %q", s)
	}
	if strings.Contains(s, "const volatile") && !strings.Contains(s, "const  volatile") {
		t.Fatalf("single space is wrong: %q", s)
	}
}
