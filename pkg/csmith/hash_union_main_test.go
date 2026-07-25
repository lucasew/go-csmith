package csmith

import (
	"strings"
	"testing"
)

func TestHashGlobalVariablesWithUnionFacts(t *testing.T) {
	ReinstallTestProcessSingletons()
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	vs := NewVariableSelector(Defaults())
	vs.GlobalList = []*Variable{uv}
	// nil facts → all fields
	all := HashGlobalVariables(vs)
	if !strings.Contains(all, "g_u.f0") || !strings.Contains(all, "g_u.f1") {
		t.Fatal(all)
	}
	// only field 0 readable
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := HashGlobalVariablesWithUnionFacts(vs, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal(out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("must skip f1", out)
	}
}

func TestHashGlobalVariablesIncompleteSticky(t *testing.T) {
	// incomplete GlobalList / UnionFacts fail closed sticky (no invent empty hash)
	vs := NewVariableSelector(Defaults())
	g := CreateVariableScalars("g_1", GetIntType(), false, false)
	vs.GlobalList = []*Variable{g, nil}
	ClearErrorSess(testAmbientSession)
	if HashGlobalVariables(vs) != "" {
		t.Fatal("nil GlobalList hole must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil GlobalList hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	vs.GlobalList = []*Variable{g}
	if HashGlobalVariablesWithUnionFacts(vs, IncompleteUnionFactSlice()) != "" {
		t.Fatal("incomplete union facts must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete union facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHashGlobalVariablesHashOutputResidualSticky(t *testing.T) {
	// hashOutput residual soft invent was soft-continue later globals invent partial hash.
	ClearErrorSess(testAmbientSession)
	good := CreateVariableScalars("g_ok", GetIntType(), false, false)
	// IsArray without AsArray stickies hashOutput
	shell := &Variable{Name: "g_arr", Type: GetIntType(), IsArray: true, ArraySizes: []int{2}}
	vs := NewVariableSelector(Defaults())
	vs.GlobalList = []*Variable{good, shell}
	if s := HashGlobalVariables(vs); s != "" {
		t.Fatal("hashOutput residual must fail closed whole HashGlobalVariables, not invent partial", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("hashOutput residual HashGlobalVariables must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual mid-list must not invent hash of later complete globals only
	vs.GlobalList = []*Variable{shell, good}
	if s := HashGlobalVariables(vs); s != "" {
		t.Fatal("early residual must fail closed before later globals", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("early residual HashGlobalVariables must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHashFuncDefReadyIncompleteGlobalList(t *testing.T) {
	// incomplete GlobalList must not invent ready via GetMaxArrayDimension -1 <= 0
	opts := Defaults()
	opts.ComputeHash = true
	opts.StepHashByStmt = true // hashHelpersEnabled requires both
	g := NewProgramGenerator(NewSession(opts))
	// complete empty globals: ready (no arrays)
	g.VS.GlobalList = nil
	if !g.hashFuncDefReady() {
		t.Fatal("complete empty GlobalList must be ready")
	}
	g.VS.GlobalList = []*Variable{CreateVariableScalars("g_1", GetIntType(), false, false), nil}
	if g.hashFuncDefReady() {
		t.Fatal("incomplete GlobalList must not invent hashFuncDefReady")
	}
}

func TestProgramGeneratorHashGlobalsUsesFM(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxFuncs = 2
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	g := NewProgramGenerator(NewSession(opts))
	g.GenerateAllTypes()
	// seed a union global with live init (nil rhs union abstract fails closed sticky)
	ut := &Type{
		isUnion: true, StructName: "U1",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// Constant init field 0 — AbstractFactUnion transfer for constant → MakeFactUnions
	uv.Init = MakeInt(0)
	g.VS.GlobalList = append(g.VS.GlobalList, uv)
	g.GenerateFunctions()
	ClearErrorSess(testAmbientSession) // generation may fail-closed other paths; hash path only needs VS+FM
	// attach union fact on first func FM (or use generator FM after seed)
	if len(g.Funcs.Funcs) > 0 {
		fm := g.FactMgrs.ForFunc(g.Funcs.Funcs[0])
		if fm != nil {
			fm.UnionFacts = []*FactUnion{MakeFactUnion(uv, 0)}
		}
	}
	out := g.hashGlobals()
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("unread field hashed", out)
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeExpressionAssignSetsTypeAndFacts(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	e := MakeExpressionAssign(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType(), nil)
	if e == nil || e.Term != TermAssignment || e.Assign == nil {
		t.Fatal(e)
	}
	if e.ExprType == nil {
		t.Fatal("ExprType")
	}
	// Output is parenthesized assign
	s := e.Output()
	if !strings.Contains(s, "(") || !strings.Contains(s, ")") {
		t.Fatal(s)
	}
}

func TestHashGlobalVariablesNilVSSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if HashGlobalVariables(nil) != "" {
		t.Fatal("nil VS HashGlobalVariables must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil VS HashGlobalVariables must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
