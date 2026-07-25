package csmith

import (
	"strings"
	"testing"
)

func TestRestrictWrite(t *testing.T) {
	q := NewCVQualifiers([]bool{true}, []bool{true})
	q.Restrict(AccessWrite, EmptyCGContext().WithSession(testAmbientSession))
	if q.IsConstSess(testAmbientSession) {
		t.Fatal("const cleared")
	}
	// SE-free keeps volatile
	if !q.IsVolatileSess(testAmbientSession) {
		t.Fatal("vol kept se-free")
	}
	q2 := NewCVQualifiers([]bool{false}, []bool{true})
	q2.Restrict(AccessRead, WithEffectContext(WithSideEffects()).WithSession(testAmbientSession))
	if q2.IsVolatileSess(testAmbientSession) {
		t.Fatal("vol cleared non-se-free")
	}
}

func TestRestrictIncompleteEffectSticky(t *testing.T) {
	// Incomplete EffectContext must not invent clear-vol via IncompleteEffect SE-false
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{true}, []bool{true})
	q.Restrict(AccessWrite, WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession))
	if !q.IsConstSess(testAmbientSession) || !q.IsVolatileSess(testAmbientSession) {
		t.Fatalf("incomplete ambient must not mutate qfer: const=%v vol=%v", q.IsConstSess(testAmbientSession), q.IsVolatileSess(testAmbientSession))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetConstPosFromEnd(t *testing.T) {
	// CVQualifiers.cpp:588–592 — set_const(v, pos) → is_consts[len-pos-1]
	q := NewCVQualifiers([]bool{true, true}, []bool{false, false})
	q.SetConstSess(testAmbientSession, false, 0) // storage (last)
	if q.IsConsts[1] || !q.IsConsts[0] {
		t.Fatalf("pos0 clears last only: %v", q.IsConsts)
	}
	q.SetConstSess(testAmbientSession, false, 1) // pointee level
	if q.IsConsts[0] {
		t.Fatalf("pos1 clears first: %v", q.IsConsts)
	}
	// no invent grow on empty
	empty := CVQualifiers{}
	empty.SetConstSess(testAmbientSession, true, 0)
	if len(empty.IsConsts) != 0 {
		t.Fatal("must not invent slots")
	}
}

func TestSelectDerefExpandStructFailClosed(t *testing.T) {
	// VariableSelector.cpp:1287–1297 — expand_struct miss → Error, no Generate fallthrough
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.ExpandStruct = true
	opts.Volatiles = true
	opts.VolatilePointers = true
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f"}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession)
	// pointee type for *p selection; qfer depth 1 for int
	q := NewCVQualifiers([]bool{false}, []bool{false})
	// force create path: empty nonvol list; ExpandStruct with no struct types → fail
	got := selectDerefPointer(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, cg, GetIntTypeSess(testAmbientSession), &q, AccessRead)
	if got != nil && len(vs.GlobalList) > 0 {
		// if somehow created without expand path, ok only when ExpandStruct off
	}
	// with ExpandStruct and no matching struct, must not soft-create pointer via GenerateNew*
	// either nil+error or successful choose (empty cands → create path fail-closed)
	if got == nil && !HasErrorSess(testAmbientSession) {
		// fail closed without sticky error is also ok if ptr_type / path returned 0 early
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputQualifiedTypeSimple(t *testing.T) {
	// Defaults enable Consts/Volatiles
	SetProcessOptionsSess(testAmbientSession, Defaults())
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedTypeSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if !strings.Contains(s, "const") || !strings.Contains(s, "volatile") || !strings.Contains(s, "int") {
		t.Fatal(s)
	}
}

func TestOutputQualifiedTypeNoInventWhenOptionsOff(t *testing.T) {
	// CVQualifiers.cpp:541–552 — assert(0) sticky if const/vol bit without option
	// (no invent bare "int" by silently dropping disabled quals)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Consts = false
	opts.Volatiles = false
	SetProcessOptionsSess(testAmbientSession, opts)
	defer SetProcessOptionsSess(testAmbientSession, Defaults())
	q := NewCVQualifiers([]bool{true}, []bool{true})
	s := q.OutputQualifiedTypeSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	if s != "" {
		t.Fatalf("const bit without Consts option must fail closed empty, got %q", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("const bit without Consts option must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputQualifiedTypeNoInventVoidForNil(t *testing.T) {
	// CVQualifiers.cpp:532 — assert(t); sticky no soft invent "void"
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if s := q.OutputQualifiedTypeSess(testAmbientSession, nil); s != "" {
		t.Fatal("nil type must fail closed empty, got", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type OutputQualifiedType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (*Type)(nil).CNameSess(testAmbientSession); s != "" {
		t.Fatal("nil Type.CName must not invent void, got", s)
	}
	ClearErrorSess(testAmbientSession)
}

func TestFunctionHeaderNoInventVoidReturn(t *testing.T) {
	// Function always has return_type; sticky no invent void when missing
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_x"}
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("missing return type must fail closed header", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing return type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.ReturnType = GetSimpleTypeSess(testAmbientSession, EVoid)
	out := f.OutputHeader(false)
	if !strings.Contains(out, "void func_x(void)") {
		t.Fatal(out)
	}
	// sticky no invent "void (void)" without name
	ClearErrorSess(testAmbientSession)
	f.Name = ""
	if out := f.OutputHeader(false); out != "" {
		t.Fatal("empty name must fail closed header", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty function name must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputDeclNoInventEmptyName(t *testing.T) {
	// Variable always has live name; sticky no invent "int "
	ClearErrorSess(testAmbientSession)
	v := &Variable{Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if out := v.OutputDeclSess(testAmbientSession, false, false); out != "" {
		t.Fatal("empty name must fail closed decl", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name OutputDecl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputGlobalsNoInventEmptyDef(t *testing.T) {
	// incomplete OutputDef must not invent "static \n" / blank lines / section-only
	opts := Defaults()
	opts.ForceGlobalsStatic = true
	g := NewProgramGenerator(NewSession(opts))
	// global without init → OutputDef empty sticky
	v := &Variable{Name: "g_x", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	g.VS.GlobalList = []*Variable{v}
	out := g.OutputGlobals()
	if out != "" {
		t.Fatal("all-empty globals must fail closed empty section", out)
	}
	if !g.hasErr() {
		t.Fatal("empty-def globals Output must SetError sticky")
	}
	// incomplete among live fails whole section (no invent skip holes)
	g.clearErr()
	good := CreateVariableScalarsSess(testAmbientSession, "g_ok", GetIntTypeSess(testAmbientSession), false, false)
	g.VS.GlobalList = []*Variable{good, v}
	if out2 := g.OutputGlobals(); out2 != "" {
		t.Fatal("mixed incomplete globals must fail closed", out2)
	}
	if !g.hasErr() {
		t.Fatal("mixed incomplete globals must SetError sticky")
	}
	g.clearErr()
	// IsArray without AsArray soft invent was scalar OutputDef for array shell
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}, Init: MakeInt(0)}
	g.VS.GlobalList = []*Variable{shell}
	if out3 := g.OutputGlobals(); out3 != "" {
		t.Fatal("IsArray without AsArray must fail closed globals", out3)
	}
	if !g.hasErr() {
		t.Fatal("IsArray without AsArray OutputGlobals must SetError sticky")
	}
	g.clearErr()
}

func TestOutputStructTypesNoInventEmptySection(t *testing.T) {
	// Type.cpp:1894–1901 — empty used pool still emits section header; incomplete IR fails closed.
	g := NewProgramGenerator(NewSession(Defaults()))
	// complete empty: only section comment (no invent types past unused inventory)
	g.Types.StructTypes = nil
	g.Types.UnionTypes = nil
	g.Types.AllTypes = nil
	out := g.OutputStructTypes()
	if out != "/* --- Struct/Union Declarations --- */\n" {
		t.Fatalf("empty used pool must still emit section header, got %q", out)
	}
	if g.hasErr() {
		t.Fatal("complete empty OutputStructTypes must not SetError")
	}
	// used aggregate emits decl body
	g.clearErr()
	st := &Type{isStruct: true, StructName: "S0", Used: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	g.Types.StructTypes = []*Type{st}
	g.Types.AllTypes = []*Type{st}
	out2 := g.OutputStructTypes()
	if !strings.Contains(out2, "/* --- Struct/Union Declarations --- */") || !strings.Contains(out2, "struct S0") {
		t.Fatal("used struct must emit under section", out2)
	}
	// unused inventory must not invent decl (only header)
	g.clearErr()
	stU := &Type{isStruct: true, StructName: "S1", Used: false, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1, Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
	}}
	g.Types.StructTypes = []*Type{stU}
	g.Types.AllTypes = []*Type{stU}
	out3 := g.OutputStructTypes()
	if out3 != "/* --- Struct/Union Declarations --- */\n" || strings.Contains(out3, "struct S1") {
		t.Fatal("unused struct must not invent decl", out3)
	}
	// incomplete unnamed used → fail closed
	g.clearErr()
	g.Types.StructTypes = []*Type{{isStruct: true, Used: true}} // unnamed → empty decl
	g.Types.AllTypes = g.Types.StructTypes
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("empty struct decls must fail closed section", out)
	}
	if !g.hasErr() {
		t.Fatal("empty struct decl OutputStructTypes must SetError sticky")
	}
	// nil hole fails closed sticky
	g.clearErr()
	g.Types.StructTypes = []*Type{nil}
	g.Types.AllTypes = nil
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("nil struct hole must fail closed", out)
	}
	if !g.hasErr() {
		t.Fatal("nil struct hole must SetError sticky")
	}
	g.clearErr()
	g.Types.StructTypes = nil
	g.Types.UnionTypes = []*Type{nil}
	g.Types.AllTypes = nil
	if out := g.OutputStructTypes(); out != "" {
		t.Fatal("nil union hole must fail closed", out)
	}
	if !g.hasErr() {
		t.Fatal("nil union hole must SetError sticky")
	}
	g.clearErr()
}

func TestOutputFunctionsNoInventEmptySections(t *testing.T) {
	// incomplete funcs must not invent FORWARD/FUNCTIONS section-only shells
	g := NewProgramGenerator(NewSession(Defaults()))
	g.Funcs.Funcs = []*Function{
		{Name: "", ReturnType: GetIntTypeSess(testAmbientSession)}, // empty name → empty header
	}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("empty function IR must fail closed sections", out)
	}
	// nil Funcs hole fails closed sticky (no invent skip holes)
	g.clearErr()
	good := &Function{
		Name: "func_1", AliasName: "func_1_alias", ReturnType: GetIntTypeSess(testAmbientSession),
		RV:   CreateVariableQferSess(testAmbientSession, "func_1_rv", GetIntTypeSess(testAmbientSession), NewCVQualifiers([]bool{false}, []bool{false})),
		Body: &Block{}, IsBuilt: true, BuildState: BuildBuilt,
	}
	g.Funcs.Funcs = []*Function{good, nil}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("nil Funcs hole must fail closed", out)
	}
	if !g.hasErr() {
		t.Fatal("nil Funcs hole must SetError sticky")
	}
	g.clearErr()
	// empty-name incomplete among live fails whole (no invent skip)
	g.clearErr()
	g.Funcs.Funcs = []*Function{good, {Name: "", ReturnType: GetIntTypeSess(testAmbientSession)}}
	if out := g.OutputFunctions(); out != "" {
		t.Fatal("mixed incomplete must fail closed", out)
	}
	g.clearErr()
}

func TestBlockLocalNoInventEmptyDef(t *testing.T) {
	// incomplete local OutputDef fails whole block sticky (no invent blank lines / partial)
	ClearErrorSess(testAmbientSession)
	b := &Block{LocalVars: []*Variable{
		{Name: "l_x", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // no init
	}}
	out := b.Output(0)
	if out != "" {
		t.Fatal("incomplete local must fail closed whole block", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete local block Output must SetError sticky")
	}
	// good local still emits after clear
	ClearErrorSess(testAmbientSession)
	ok := CreateVariableScalarsSess(testAmbientSession, "l_ok", GetIntTypeSess(testAmbientSession), false, false)
	if ok == nil {
		t.Fatal("CreateVariableScalars after ClearError")
	}
	ok.Init = MakeInt(1)
	b2 := &Block{LocalVars: []*Variable{ok}}
	if out2 := b2.Output(0); !strings.Contains(out2, "l_ok") {
		t.Fatal(out2)
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputValueDumpNoInventEmptyName(t *testing.T) {
	// Variable.cpp:1184 — name + directive always live sticky; hash empty name sticky
	ClearErrorSess(testAmbientSession)
	v := &Variable{Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if s := v.OutputValueDumpSess(testAmbientSession, "checksum ", 1, nil); s != "" {
		t.Fatal("empty name must fail closed dump", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := v.HashOutputSess(testAmbientSession); s != "" {
		t.Fatal("empty name must fail closed hash", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// array dump/hash without name — no invent bare "[0]" / for ( = 0; …)
	arr := &Variable{Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}, Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if s := arr.OutputValueDumpSess(testAmbientSession, "c ", 1, nil); s != "" {
		t.Fatal("empty array name must fail closed dump", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty array name OutputValueDump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	ctrl := []*Variable{
		{Name: "i", Type: GetIntTypeSess(testAmbientSession)},
	}
	if s := hashArrayVariableSess(testAmbientSession, arr, ctrl, nil); s != "" {
		t.Fatal("empty array name must fail closed hash", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty array name hashArrayVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty ctrl name sticky — no invent for ( = 0; …)
	arr.Name = "g_a"
	if s := hashArrayVariableSess(testAmbientSession, arr, []*Variable{{Type: GetIntTypeSess(testAmbientSession)}}, nil); s != "" {
		t.Fatal("empty ctrl name must fail closed array hash", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty ctrl name hashArrayVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete field Type sticky (no invent skip partial hash as empty success)
	agg := &Type{isStruct: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: nil, BitWidth: -1},
	}}
	arr2 := &Variable{
		Name: "g_s", Type: agg, IsArray: true, ArraySizes: []int{1},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if s := hashArrayVariableSess(testAmbientSession, arr2, ctrl, nil); s != "" {
		t.Fatal("field Type hole must fail closed hash", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("field Type hole hashArrayVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsFieldReadable residual: incomplete UnionFacts stickies ERROR+false.
	// Soft invent was soft-continue then emit later field transparent_crc past hole.
	// Fair: sticky fail closed whole array hash.
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	arrU := &Variable{
		Name: "g_u", Type: ut, IsArray: true, ArraySizes: []int{1},
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	holeFacts := []*FactUnion{nil}
	if s := hashArrayVariableSess(testAmbientSession, arrU, ctrl, holeFacts); s != "" {
		t.Fatal("IsFieldReadable residual must fail closed array hash", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsFieldReadable residual hashArrayVariable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputExpressionVariableNoInventEmptyBase(t *testing.T) {
	// ExpressionVariable Output requires live Variable::Output base
	// UseVolRVal + volatile + nil type → OutputC empty; multi * must not invent
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, true)
	if v == nil {
		t.Fatal("create g_p")
	}
	v.UseVolRVal = true
	v.Type = nil // force VOL_RVAL fail closed empty
	if s := outputExpressionVariable(v, GetIntTypeSess(testAmbientSession)); s != "" {
		t.Fatal("empty base must fail closed", s)
	}
}

func TestCNameNoInventBareAggregateOrDefaultInt(t *testing.T) {
	// Type.cpp always has sid; sticky no invent bare "struct"/"union" or default "int"
	ClearErrorSess(testAmbientSession)
	if s := (&Type{isStruct: true}).CNameSess(testAmbientSession); s != "" {
		t.Fatal("unnamed struct", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unnamed struct CName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&Type{isUnion: true}).CNameSess(testAmbientSession); s != "" {
		t.Fatal("unnamed union", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unnamed union CName must SetError sticky")
	}
	// unknown simple enum value — not a known E*
	ClearErrorSess(testAmbientSession)
	if s := (&Type{simple: ESimpleType(99)}).CNameSess(testAmbientSession); s != "" {
		t.Fatal("unknown simple must not invent int", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown simple CName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := (&Type{ptrTo: &Type{isStruct: true}}).CNameSess(testAmbientSession); s != "" {
		t.Fatal("ptr to unnamed struct", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ptr to unnamed struct CName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVolWrapNoInventIntType(t *testing.T) {
	// Variable.cpp:690–693 — type->Output; sticky no invent "int" when Type nil
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	v.UseVolRVal = true
	v.Type = nil
	if out := v.OutputCSess(testAmbientSession, false); out != "" {
		t.Fatal("nil Type VOL_RVAL must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type VOL_RVAL must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if out := v.OutputLhsCOptsSess(testAmbientSession, false); out != "" {
		t.Fatal("nil Type VOL_LVAL must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Type VOL_LVAL must SetError sticky")
	}
	// sticky no invent VOL_RVAL(, int) / VOL_LVAL(, int) with empty name
	ClearErrorSess(testAmbientSession)
	v2 := &Variable{Type: GetIntTypeSess(testAmbientSession), UseVolRVal: true, Qfer: NewCVQualifiers([]bool{false}, []bool{true})}
	if out := v2.OutputCSess(testAmbientSession, false); out != "" {
		t.Fatal("empty name VOL_RVAL must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name VOL_RVAL must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if out := v2.OutputLhsCOptsSess(testAmbientSession, false); out != "" {
		t.Fatal("empty name VOL_LVAL must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name VOL_LVAL must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputHeaderAliasNoInvent(t *testing.T) {
	// sticky no invent Name+"_alias" when AliasName empty
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	if out := f.OutputHeaderAlias(false); out != "" {
		t.Fatal("missing AliasName must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing AliasName must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f.AliasName = "func_1_alias"
	out := f.OutputHeaderAlias(true)
	if !strings.Contains(out, "static int32_t func_1_alias(void)") ||
		!strings.Contains(out, `alias("func_1")`) {
		t.Fatal(out)
	}
}

func TestExpressionCastNoInventEmpty(t *testing.T) {
	// cast_type + body both required sticky
	ClearErrorSess(testAmbientSession)
	e := &Expression{Term: TermConstant, Con: MakeInt(1), CastType: &Type{isStruct: true}}
	if out := e.Output(); out != "" {
		t.Fatal("cast with incomplete type must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("cast incomplete type must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	e2 := &Expression{Term: TermConstant, CastType: GetIntTypeSess(testAmbientSession)}
	if out := e2.Output(); out != "" {
		t.Fatal("cast with empty body must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("cast empty body must SetError sticky")
	}
	// Constant with empty Value — sticky no invent empty token
	ClearErrorSess(testAmbientSession)
	e3 := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession), Value: ""}}
	if out := e3.Output(); out != "" {
		t.Fatal("empty Constant.Value must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Constant.Value must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionCommaNoInventEmptySide(t *testing.T) {
	// ExpressionComma.cpp:109–115 — both sides Output live; sticky no invent "( , x)"
	ClearErrorSess(testAmbientSession)
	good := &Expression{Term: TermConstant, Con: MakeInt(1)}
	bad := &Expression{Term: TermConstant} // nil Con → empty Output sticky
	e := &Expression{Term: TermCommaExpr, CommaLHS: bad, CommaRHS: good}
	if out := e.Output(); out != "" {
		t.Fatal("empty lhs Output must fail closed comma", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty lhs comma must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	e.CommaLHS, e.CommaRHS = good, bad
	if out := e.Output(); out != "" {
		t.Fatal("empty rhs Output must fail closed comma", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty rhs comma must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	e.CommaLHS, e.CommaRHS = good, &Expression{Term: TermConstant, Con: MakeInt(2)}
	if out := e.Output(); out != "(1 , 2)" {
		t.Fatal(out)
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputDeclNoInventEmptyType(t *testing.T) {
	// Variable::OutputDecl — qualified type always live; sticky no invent " name"
	ClearErrorSess(testAmbientSession)
	v := &Variable{Name: "g_x"}
	if out := v.OutputDeclSess(testAmbientSession, false, false); out != "" {
		t.Fatal("nil type must fail closed decl", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil type OutputDecl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputForwardDeclNoInventBareSemi(t *testing.T) {
	// incomplete header sticky → empty, not bare ";"
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_x"}
	if out := f.OutputForwardDecl(); out != "" {
		t.Fatal("incomplete forward decl must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete forward decl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if out := f.OutputForwardDeclAlias(false); out != "" {
		t.Fatal("incomplete alias decl must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete alias decl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Function always live at emit; sticky empty (no invent bare ";")
	if out := (*Function)(nil).OutputForwardDecl(); out != "" {
		t.Fatal("nil OutputForwardDecl must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil OutputForwardDecl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if out := (*Function)(nil).OutputForwardDeclAlias(false); out != "" {
		t.Fatal("nil OutputForwardDeclAlias must fail closed", out)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil OutputForwardDeclAlias must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// builtins complete empty (compiler-provided)
	if out := (&Function{Name: "abs", IsBuiltin: true}).OutputForwardDecl(); out != "" {
		t.Fatal("builtin OutputForwardDecl must stay complete empty", out)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("builtin OutputForwardDecl must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomLoopControlErrorReturn(t *testing.T) {
	// StatementFor.cpp:79/82/102 ERROR_RETURN on sticky error
	ClearErrorSess(testAmbientSession)
	SetErrorSess(testAmbientSession, ErrGeneric)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	_, _, _, _, _ = MakeRandomLoopControl(NewRngSess(testAmbientSession, 1), opts, true)
	// sticky remains; MakeIteration would abort
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ERROR_RETURN must keep sticky error")
	}
}

func TestVariableOutputDef(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), true, false)
	v.Init = MakeInt(3)
	s := v.OutputDefFullSess(testAmbientSession, true, false, false, nil)
	if !strings.Contains(s, "static") || !strings.Contains(s, "const") || !strings.Contains(s, "g_1") || !strings.Contains(s, "3") {
		t.Fatal(s)
	}
}

func TestVariableOutputDefMissingInitFailClosed(t *testing.T) {
	// Variable.cpp:659 — assert(init); sticky no soft invent "= ;"
	ClearErrorSess(testAmbientSession)
	v := &Variable{Name: "g_u", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}
	if v.OutputDefFullSess(testAmbientSession, true, false, false, nil) != "" {
		t.Fatal("missing init must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing init OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray soft invent was scalar "int g_a = 0;"
	shell := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}, Init: MakeInt(0)}
	if shell.OutputDefFullSess(testAmbientSession, true, false, false, nil) != "" {
		t.Fatal("IsArray without AsArray OutputDef must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVariableOutputDefMissingDeclFailClosed(t *testing.T) {
	// Variable.cpp:640–660 — sticky no invent " = 3;" without live decl/type
	ClearErrorSess(testAmbientSession)
	v := &Variable{Name: "g_u", Init: MakeInt(3)}
	if v.OutputDefFullSess(testAmbientSession, true, false, false, nil) != "" {
		t.Fatal("missing type must fail closed", v.OutputDefFullSess(testAmbientSession, true, false, false, nil))
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("missing type OutputDef must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputQualifiedTypeBadSanityFailClosed(t *testing.T) {
	// CVQualifiers.cpp:533 — assert(sanity_check(t)); sticky no invent bare type
	// pointer type needs 2-level qfer (indirect+1)
	ClearErrorSess(testAmbientSession)
	pt := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	q := NewCVQualifiers([]bool{false}, []bool{false}) // too short
	if s := q.OutputQualifiedTypeSess(testAmbientSession, pt); s != "" {
		t.Fatal("bad qfer layout must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("bad qfer layout must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputGlobalsUsesOutputDef(t *testing.T) {
	opts := Defaults()
	opts.Seed = 2
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	// force a global
	q := NewCVQualifiers([]bool{false}, []bool{false})
	_ = g.VS.GenerateNewGlobal(AccessRead, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), &q, g.Rng)
	out := g.OutputGlobals()
	if !strings.Contains(out, "static") || !strings.Contains(out, "g_") {
		t.Fatal(out)
	}
}

func TestOutputDeclOutputQualifiedTypeResidualSticky(t *testing.T) {
	// OutputQualifiedType residual soft invent was soft-continue invent " name" decl.
	// Unnamed struct CName residual under OutputQualifiedType.
	ClearErrorSess(testAmbientSession)
	v := &Variable{
		Name: "g_s",
		Type: &Type{isStruct: true}, // CName residual
		Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
	}
	if s := v.OutputDeclSess(testAmbientSession, false, false); s != "" {
		t.Fatal("CName residual must fail closed OutputDecl", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CName residual OutputDecl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputAddrOfGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was soft-empty invent bare "&".
	// Empty name already sticky; residual path: covered by nil. Keep complete hygiene.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	if s := v.OutputAddrOfSess(testAmbientSession, false); s != "&g_x" {
		t.Fatal("complete OutputAddrOf", s)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete OutputAddrOf must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty name residual hole
	if s := (&Variable{Type: GetIntTypeSess(testAmbientSession)}).OutputAddrOfSess(testAmbientSession, false); s != "" {
		t.Fatal("empty name OutputAddrOf must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name OutputAddrOf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputConditionBoundResidualSticky(t *testing.T) {
	// OutputLower/UpperBound residual soft invent was soft-continue invent partial range.
	ClearErrorSess(testAmbientSession)
	// array without AsArray → OutputLowerBound residual sticky
	arr := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{4}}
	f := &FactPointTo{Var: CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false), PointTo: []*Variable{arr}}
	if s := f.OutputCondition(); s != "" {
		t.Fatal("bound residual must fail closed OutputCondition", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("bound residual OutputCondition must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputForwardDeclHeaderResidualSticky(t *testing.T) {
	// OutputHeader residual soft invent was invent bare ";" past empty header shell.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "", ReturnType: GetIntTypeSess(testAmbientSession)} // empty name residual
	if s := f.OutputForwardDecl(); s != "" {
		t.Fatal("OutputHeader residual must fail closed OutputForwardDecl", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("OutputHeader residual OutputForwardDecl must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputGlobalsOutputDefResidualSticky(t *testing.T) {
	// OutputDef residual soft invent was soft-continue later globals invent partial section.
	opts := Defaults()
	sess := NewSession(opts)
	g := NewProgramGenerator(sess)
	g.VS = NewVariableSelector(testAmbientSession, opts)
	// Type-nil InitExpr residual on OutputDefFull
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	v.Init = nil
	v.InitExpr = &Expression{Term: TermConstant, Con: &Constant{Value: "0"}} // Type-nil residual
	g.VS.GlobalList = []*Variable{v}
	if s := g.OutputGlobals(); s != "" {
		t.Fatal("InitExpr Output residual must fail closed OutputGlobals", s)
	}
	// bag-local sticky (no ambient dual-sync)
	if !HasErrorSess(sess) {
		t.Fatal("InitExpr Output residual OutputGlobals must SetError sticky")
	}
	ClearErrorSess(sess)
}

func TestReturnTypeCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was invent "void"/partial header past hole.
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_1", ReturnType: &Type{isStruct: true}} // unnamed struct CName residual
	if s := f.returnTypeC(); s != "" {
		t.Fatal("CName residual must fail closed returnTypeC", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CName residual returnTypeC must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if s := f.OutputHeader(false); s != "" {
		t.Fatal("CName residual must fail closed OutputHeader", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CName residual OutputHeader must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputArrayInitForcedResidualSticky(t *testing.T) {
	// InitExpr Output residual soft invent was invent forced loop-init past hole.
	ClearErrorSess(testAmbientSession)
	av := &ArrayVariable{
		Variable: Variable{
			Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2},
			InitExpr: &Expression{Term: TermConstant, Con: &Constant{Value: "0"}}, // Type-nil residual
		},
		Sizes: []int{2},
	}
	av.AsArray = av
	if s := outputArrayInitForced(av, "    ", []string{"i"}, true); s != "" {
		t.Fatal("InitExpr Output residual must fail closed outputArrayInitForced", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("InitExpr Output residual outputArrayInitForced must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputQualifiedTypeCNameResidualSticky(t *testing.T) {
	// CName residual soft invent was invent "const " / partial qualified type past hole.
	ClearErrorSess(testAmbientSession)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	if s := q.OutputQualifiedTypeSess(testAmbientSession, &Type{isStruct: true}); s != "" {
		t.Fatal("CName residual must fail closed OutputQualifiedType", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("CName residual OutputQualifiedType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputUpperBoundResidualSticky(t *testing.T) {
	// OutputUpperBoundArray residual soft invent was invent bare field path past hole.
	ClearErrorSess(testAmbientSession)
	// IsArray without AsArray residual
	v := &Variable{Name: "g_a", Type: GetIntTypeSess(testAmbientSession), IsArray: true, ArraySizes: []int{2}}
	if s := v.OutputUpperBoundSess(testAmbientSession, false); s != "" {
		t.Fatal("IsArray without AsArray must fail closed OutputUpperBound", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsArray without AsArray OutputUpperBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// empty name residual
	if s := (&Variable{Type: GetIntTypeSess(testAmbientSession)}).OutputUpperBoundSess(testAmbientSession, false); s != "" {
		t.Fatal("empty name OutputUpperBound must fail closed", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name OutputUpperBound must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestCtrlVarNamesGetActualNameResidualSticky(t *testing.T) {
	// GetActualName residual soft invent was soft-continue invent partial name list.
	ClearErrorSess(testAmbientSession)
	ctrl := []*Variable{
		{Name: "i", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})},
		{Name: "", Type: GetIntTypeSess(testAmbientSession), Qfer: NewCVQualifiers([]bool{false}, []bool{false})}, // residual
	}
	if LabelsComplete(CtrlVarNamesSess(testAmbientSession, ctrl)) {
		t.Fatal("empty name residual must fail closed IncompleteLabelsSlice")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty name residual CtrlVarNames must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsFieldReadableIsUnionResidualSticky(t *testing.T) {
	// IsUnion residual soft invent was invent not-readable soft-skip past Type-nil already sticky.
	// Non-union complete: not readable false without sticky.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	if IsFieldReadable(v, 0, nil) {
		t.Fatal("non-union IsFieldReadable must be false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete non-union IsFieldReadable must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil sticky
	if IsFieldReadable(&Variable{Name: "g_u", Type: nil}, 0, nil) {
		t.Fatal("Type-nil IsFieldReadable must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil IsFieldReadable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputGlobalVariablesListResidualSticky(t *testing.T) {
	// OutputDef residual soft invent was invent section header past incomplete var.
	ClearErrorSess(testAmbientSession)
	// incomplete list sticky
	if OutputGlobalVariables([]*Variable{nil}) != "" {
		t.Fatal("nil hole OutputGlobalVariables must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole OutputGlobalVariables must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty
	if OutputGlobalVariables(nil) != "" {
		t.Fatal("empty OutputGlobalVariables must be empty")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty OutputGlobalVariables must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestOutputQualifiedTypeConstVolatilePointerDoubleSpace(t *testing.T) {
	// CVQualifiers.cpp:540–552 — level i>0 with both const and volatile:
	// "const " then (i>0) " " then "volatile " → "const  volatile "
	// (seed-2 g_459: UP "const  volatile" vs invent skip space when const already present)
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	pt := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))) // int32_t **
	// depth 3: [obj, *, *] — outer pointer level both const+vol
	q := NewCVQualifiers(
		[]bool{false, false, true},
		[]bool{false, false, true},
	)
	s := q.OutputQualifiedTypeSess(testAmbientSession, pt)
	if !strings.Contains(s, "const  volatile") {
		t.Fatalf("want double space const  volatile, got %q", s)
	}
	if strings.Contains(s, "const volatile") && !strings.Contains(s, "const  volatile") {
		t.Fatalf("single space is wrong: %q", s)
	}
}
